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
