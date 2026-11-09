package browser

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"
)

func TestRecapSummaryRefreshesTargetsAfterTranscriptLoads(t *testing.T) {
	var lists atomic.Int32
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []cdpTargetInfo{{
				Type: "iframe", URL: "https://example.invalid/xplatplugins.aspx",
				WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/transcript",
			}}
			if lists.Add(1) >= 3 {
				targets = append(targets, cdpTargetInfo{
					Type: "page", URL: "https://teams.cloud.microsoft/l/meetingrecap",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/summary",
				})
			}
			require.NoError(t, json.NewEncoder(w).Encode(targets))
			return
		}
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }() // Test connection cleanup.
		if _, _, err := wsutil.ReadClientData(conn); err != nil {
			t.Errorf("read evaluation: %v", err)
			return
		}
		value := `{"entries":[{"speaker":"Presenter","timestamp":"PT1S","text":"Synthetic recap"}]}`
		if r.URL.Path == "/summary" {
			value = "  Synthetic AI summary  "
		}
		payload, err := json.Marshal(map[string]any{
			"id": 1, "result": map[string]any{"result": map[string]string{"value": value}},
		})
		if err != nil {
			t.Errorf("marshal evaluation: %v", err)
			return
		}
		if err := wsutil.WriteServerText(conn, payload); err != nil {
			t.Errorf("write evaluation: %v", err)
		}
	}))
	defer srv.Close()
	serverURL = srv.URL
	_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	m := &chromeMeeting{}
	require.True(t, m.tryExtractRecapViaCDP(ctx, port))
	require.Equal(t, "Synthetic AI summary", m.AISummary())
	require.GreaterOrEqual(t, lists.Load(), int32(3))
}

func TestEvalInCDPTargetRejectsProtocolErrorAndNilValue(t *testing.T) {
	var call atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if _, _, err := wsutil.ReadClientData(conn); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		n := call.Add(1)
		var resp map[string]any
		if n == 1 {
			resp = map[string]any{
				"id":    1,
				"error": map[string]any{"code": -32000, "message": "Execution context was destroyed"},
			}
		} else {
			resp = map[string]any{
				"id":     1,
				"result": map[string]any{"result": map[string]any{"type": "undefined"}},
			}
		}
		b, _ := json.Marshal(resp)
		_ = wsutil.WriteServerText(conn, b)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	_, err := evalInCDPTarget(context.Background(), wsURL, "1+1")
	require.Error(t, err, "top-level CDP error must return an error")

	val, err := evalInCDPTarget(context.Background(), wsURL, "undefined")
	require.NoError(t, err)
	require.Equal(t, "", val, "nil/undefined CDP value must return empty string, not \"null\"")
}

func TestRecapSummaryScopedToOwningMeetingPageOnly(t *testing.T) {
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			targets := []cdpTargetInfo{
				{
					ID:                   "other-tab",
					Type:                 "page",
					URL:                  "https://teams.cloud.microsoft/l/meetingrecap?other=1",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/other-summary",
				},
				{
					ID:                   "target-tab",
					Type:                 "page",
					URL:                  "https://teams.cloud.microsoft/l/meetingrecap?target=1",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/target-summary",
				},
				{
					ID:                   "target-iframe",
					ParentID:             "target-tab",
					Type:                 "iframe",
					URL:                  "https://example.invalid/xplatplugins.aspx",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/transcript",
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(targets))
			return
		}
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if _, _, err := wsutil.ReadClientData(conn); err != nil {
			t.Errorf("read: %v", err)
			return
		}
		var value string
		switch r.URL.Path {
		case "/transcript":
			value = `{"entries":[{"speaker":"Presenter","timestamp":"PT1S","text":"Target transcript"}]}`
		case "/other-summary":
			value = "WRONG MEETING SUMMARY"
		case "/target-summary":
			value = "TARGET MEETING SUMMARY"
		}
		payload, _ := json.Marshal(map[string]any{
			"id": 1, "result": map[string]any{"result": map[string]string{"value": value}},
		})
		_ = wsutil.WriteServerText(conn, payload)
	}))
	defer srv.Close()
	serverURL = srv.URL
	_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	m := &chromeMeeting{capturePageID: "target-tab"}
	require.True(t, m.tryExtractRecapViaCDP(context.Background(), port))
	require.Equal(t, "TARGET MEETING SUMMARY", m.AISummary(),
		"AI summary must be captured strictly from the owning meeting tab, never another open tab")
}
