package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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
			// Real Chrome /json/list descriptors do not include parentId;
			// iframe ownership is resolved via Page.getFrameTree on target-tab.
			targets := []cdpTargetInfo{
				{
					ID:                   "other-tab",
					Type:                 "page",
					URL:                  "https://teams.cloud.microsoft/l/meetingrecap?other=1",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/other-summary",
				},
				{
					ID:                   "other-iframe",
					Type:                 "iframe",
					URL:                  "https://example.invalid/xplatplugins.aspx",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/other-transcript",
				},
				{
					ID:                   "target-tab",
					Type:                 "page",
					URL:                  "https://teams.cloud.microsoft/l/meetingrecap?target=1",
					WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/target-summary",
				},
				{
					ID:                   "target-iframe",
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
		reqBytes, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.Unmarshal(reqBytes, &req)
		if req.Method == "Page.getFrameTree" {
			frameTreeResp := map[string]any{
				"id": req.ID,
				"result": map[string]any{
					"frameTree": map[string]any{
						"frame": map[string]any{
							"id":  "target-tab",
							"url": "https://teams.cloud.microsoft/l/meetingrecap?target=1",
						},
						"childFrames": []map[string]any{
							{
								"frame": map[string]any{
									"id":  "target-iframe",
									"url": "https://example.invalid/xplatplugins.aspx",
								},
							},
						},
					},
				},
			}
			b, _ := json.Marshal(frameTreeResp)
			_ = wsutil.WriteServerText(conn, b)
			return
		}
		var value string
		switch r.URL.Path {
		case "/other-transcript":
			value = `{"entries":[{"speaker":"Wrong","timestamp":"PT1S","text":"Wrong transcript"}]}`
		case "/transcript":
			value = `{"entries":[{"speaker":"Presenter","timestamp":"PT1S","text":"Target transcript"}]}`
		case "/other-summary":
			value = "WRONG MEETING SUMMARY"
		case "/target-summary":
			value = "TARGET MEETING SUMMARY"
		}
		payload, _ := json.Marshal(map[string]any{
			"id": req.ID, "result": map[string]any{"result": map[string]string{"value": value}},
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
	require.Len(t, m.recapEntries, 1)
	require.Equal(t, "Target transcript", m.recapEntries[0].Text)
	require.Equal(t, "TARGET MEETING SUMMARY", m.AISummary(),
		"AI summary must be captured strictly from the owning meeting tab, never another open tab")
}

func TestNavigateCDPTargetSendsPageNavigate(t *testing.T) {
	var gotMethod, gotURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		reqBytes, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				URL string `json:"url"`
			} `json:"params"`
		}
		_ = json.Unmarshal(reqBytes, &req)
		gotMethod = req.Method
		gotURL = req.Params.URL
		resp, _ := json.Marshal(map[string]any{
			"id":     req.ID,
			"result": map[string]any{"frameId": "frame-1"},
		})
		_ = wsutil.WriteServerText(conn, resp)
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	wantURL := "https://teams.cloud.microsoft/l/meetingrecap?threadId=123"
	require.NoError(t, navigateCDPTarget(context.Background(), wsURL, wantURL))
	require.Equal(t, "Page.navigate", gotMethod)
	require.Equal(t, wantURL, gotURL)
}

const harnessRecapURL = "https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a"

func TestMain(m *testing.M) {
	if os.Getenv("GO_TEAMS_RECAP_HEADED_FIXTURE") == "1" {
		runHeadedRecapFixture()
		return
	}
	os.Exit(m.Run())
}

func runHeadedRecapFixture() {
	var port string
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--remote-debugging-port=") {
			port = strings.TrimPrefix(arg, "--remote-debugging-port=")
		}
	}
	if port == "" {
		os.Exit(2)
	}
	base := "127.0.0.1:" + port
	server := &http.Server{Addr: base, ReadHeaderTimeout: time.Second}
	mux := http.NewServeMux()
	server.Handler = mux
	mux.HandleFunc("/json/version", func(w http.ResponseWriter, _ *http.Request) {
		if err := json.NewEncoder(w).Encode(map[string]string{"webSocketDebuggerUrl": "ws://" + base + "/browser"}); err != nil {
			os.Exit(2)
		}
	})
	mux.HandleFunc("/json/list", func(w http.ResponseWriter, _ *http.Request) {
		targets := []cdpTargetInfo{{ID: "owner", Type: "page", URL: harnessRecapURL, WebSocketDebuggerURL: "ws://" + base + "/owner"}}
		if err := json.NewEncoder(w).Encode(targets); err != nil {
			os.Exit(2)
		}
	})
	mux.HandleFunc("/owner", func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
		raw, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if json.Unmarshal(raw, &req) != nil {
			return
		}
		var result interface{}
		if req.Method == "Page.getFrameTree" {
			result = map[string]interface{}{"frameTree": map[string]interface{}{"frame": map[string]string{"id": "owner", "url": harnessRecapURL}}}
		} else {
			payload := `{"pageUrl":"` + harnessRecapURL + `","entries":[{"speaker":"Presenter","timestamp":"PT1S","text":"` + port + `"}]}`
			result = map[string]interface{}{"result": map[string]string{"value": payload}}
		}
		response, err := json.Marshal(map[string]interface{}{"id": req.ID, "result": result})
		if err == nil {
			_ = wsutil.WriteServerText(conn, response)
		}
	})
	if err := server.ListenAndServe(); err != nil {
		os.Exit(2)
	}
}

func TestJoinHeadedPersistsLaunchedCDPPortForRecap(t *testing.T) {
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("find fixture executable: %v", err)
	}
	t.Setenv("GO_TEAMS_RECAP_HEADED_FIXTURE", "1")
	m := &chromeMeeting{
		opts:       MeetingOptions{MeetingURL: harnessRecapURL},
		headedOpts: &Options{ExecPath: self},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	defer m.stopHeadedBrowser()
	if err := m.joinHeaded(ctx); err != nil {
		t.Fatalf("join synthetic recap: %v", err)
	}
	if len(m.recapEntries) == 0 {
		t.Fatal("joined recap has no transcript entries")
	}
	port, err := strconv.Atoi(m.recapEntries[0].Text)
	if err != nil {
		t.Fatalf("fixture returned invalid port: %v", err)
	}
	if m.cdpPort != port {
		t.Errorf("headed join stored CDP port %d, want launched port %d", m.cdpPort, port)
	}
}

// recapAudioFixture is a loopback CDP target listing and frame-tree fixture.
// Audio evaluation responses use a proposed JSON payload contract for the
// downstream implementation; no real media or authenticated service is used.
type recapAudioFixture struct {
	port      int
	lists     atomic.Int32
	unowned   atomic.Int32
	evaluated atomic.Int32
	frameEval atomic.Int32
	// meetingURL overrides the requested meeting URL when non-empty.
	meetingURL string
	// recapAudioURL is the retained selected-Recap URL, as set during Join.
	recapAudioURL string
	// evalDelay delays Runtime.evaluate responses on the owned frame.
	evalDelay atomic.Int64
}

func newRecapAudioFixture(t *testing.T, frameURL, payload string) *recapAudioFixture {
	t.Helper()
	f := &recapAudioFixture{}
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			f.lists.Add(1)
			wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
			targets := []cdpTargetInfo{
				{ID: "other", Type: "page", URL: harnessRecapURL, WebSocketDebuggerURL: wsBase + "/other"},
				{ID: "owner", Type: "page", URL: harnessRecapURL, WebSocketDebuggerURL: wsBase + "/owner"},
				{ID: "frame", ParentID: "owner", Type: "iframe", URL: frameURL, WebSocketDebuggerURL: wsBase + "/frame"},
			}
			if err := json.NewEncoder(w).Encode(targets); err != nil {
				t.Errorf("encode targets: %v", err)
			}
			return
		}
		if r.URL.Path == "/other" {
			f.unowned.Add(1)
		}
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			t.Errorf("upgrade CDP: %v", err)
			return
		}
		defer func() {
			if closeErr := conn.Close(); closeErr != nil {
				t.Errorf("close CDP: %v", closeErr)
			}
		}()
		raw, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			t.Errorf("read CDP: %v", err)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal(raw, &req); err != nil {
			t.Errorf("decode CDP request: %v", err)
			return
		}
		var result map[string]interface{}
		if req.Method == "Page.getFrameTree" {
			result = map[string]interface{}{"frameTree": map[string]interface{}{
				"frame":       map[string]string{"id": "owner", "url": harnessRecapURL},
				"childFrames": []map[string]interface{}{{"frame": map[string]string{"id": "frame", "url": frameURL}}},
			}}
		} else {
			f.evaluated.Add(1)
			value := ""
			if r.URL.Path == "/frame" {
				f.frameEval.Add(1)
				value = payload
				time.Sleep(time.Duration(f.evalDelay.Load()))
			}
			result = map[string]interface{}{"result": map[string]string{"value": value}}
		}
		response, err := json.Marshal(map[string]interface{}{"id": req.ID, "result": result})
		if err != nil {
			t.Errorf("encode CDP response: %v", err)
			return
		}
		if err := wsutil.WriteServerText(conn, response); err != nil {
			t.Errorf("write CDP response: %v", err)
		}
	}))
	serverURL = srv.URL
	t.Cleanup(srv.Close)
	_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("fixture address: %v", err)
	}
	f.port, err = strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("fixture port: %v", err)
	}
	return f
}

func (f *recapAudioFixture) capture(t *testing.T) ([]byte, error) {
	t.Helper()
	meetingURL := harnessRecapURL
	if f.meetingURL != "" {
		meetingURL = f.meetingURL
	}
	m := &chromeMeeting{
		opts:    MeetingOptions{MeetingURL: meetingURL},
		cdpPort: f.port, capturePageID: "owner", recapAudioURL: f.recapAudioURL,
		recapEntries: []RecapTranscriptEntry{{Text: "Synthetic transcript"}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var out bytes.Buffer
	err := m.CaptureAudio(ctx, &out)
	if f.lists.Load() == 0 {
		t.Errorf("recap capture did not inspect the CDP target list")
	}
	if f.unowned.Load() != 0 {
		t.Errorf("recap capture accessed an unrelated tab")
	}
	return out.Bytes(), err
}

func TestCaptureRecapAudioScopedToOwningMeetingPageAndEncodesMP3(t *testing.T) {
	pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
	payload := `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-a","pcmChunks":["` + pcm + `"]}`
	f := newRecapAudioFixture(t, harnessRecapURL, payload)
	got, err := f.capture(t)
	if err != nil {
		t.Fatalf("capture matching recap audio: %v", err)
	}
	if f.evaluated.Load() == 0 {
		t.Error("matching target was not evaluated")
	}
	if len(got) < 2 || got[0] != 0xff || got[1]&0xe0 != 0xe0 {
		t.Errorf("expected MPEG frame sync, got %d bytes", len(got))
	}
}

func TestCaptureRecapAudioRejectsOwnedFrameWithMismatchedRecordingIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, frameURL, payload string
	}{
		{"frame URL", "https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-b", `{"itemId":"recording-b","pcmChunks":["AAAA"]}`},
		{"media metadata", harnessRecapURL, `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-b","mediaUrl":"https://example.invalid/?driveItemId=recording-b","pcmChunks":["AAAA"]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newRecapAudioFixture(t, tc.frameURL, tc.payload)
			got, err := f.capture(t)
			if err == nil || len(got) != 0 {
				t.Errorf("mismatched recording must be rejected, err=%v bytes=%d", err, len(got))
			}
			if tc.name == "frame URL" && f.frameEval.Load() != 0 {
				t.Error("mismatched frame URL must not be evaluated")
			}
			if tc.name == "media metadata" && f.frameEval.Load() == 0 {
				t.Error("matching frame must be evaluated to detect mismatched media metadata")
			}
		})
	}
}

func TestCaptureRecapAudioRejectsOversizedRecordingExceedingResourceCap(t *testing.T) {
	for _, capName := range []string{"source bytes", "duration", "PCM bytes"} {
		t.Run(capName, func(t *testing.T) {
			payload := `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-a","error":"recap audio exceeds maximum supported size/duration","limit":"` + capName + `"}`
			f := newRecapAudioFixture(t, harnessRecapURL, payload)
			got, err := f.capture(t)
			if err == nil || !strings.Contains(err.Error(), "recap audio exceeds maximum supported size/duration") {
				t.Errorf("expected resource-limit error, got %v", err)
			}
			if f.frameEval.Load() == 0 {
				t.Error("resource-limit response was not read from the matching frame")
			}
			if len(got) != 0 {
				t.Errorf("oversized recording wrote %d bytes", len(got))
			}
		})
	}

	t.Run("Go pre-allocation checks", func(t *testing.T) {
		oldSource, oldDuration, oldPCM := maxRecapAudioSourceBytes, maxRecapAudioDurationSec, maxRecapAudioPCMBytes
		t.Cleanup(func() {
			maxRecapAudioSourceBytes, maxRecapAudioDurationSec, maxRecapAudioPCMBytes = oldSource, oldDuration, oldPCM
		})
		maxRecapAudioSourceBytes, maxRecapAudioDurationSec, maxRecapAudioPCMBytes = 8, 2, 4
		for _, tc := range []struct {
			name        string
			sourceBytes int
			duration    int
			pcm         string
			oversized   bool
		}{
			{"source above", 9, 1, "!!!!", true},
			{"duration above", 1, 3, "!!!!", true},
			{"PCM above", 1, 1, "!!!!!!!!", true},
			{"all at limit", 8, 2, base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 0}), false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				payload, err := json.Marshal(map[string]interface{}{
					"pageUrl": harnessRecapURL, "itemId": "recording-a",
					"sourceBytes": tc.sourceBytes, "durationSec": tc.duration,
					"pcmChunks": []string{tc.pcm},
				})
				if err != nil {
					t.Fatal(err)
				}
				f := newRecapAudioFixture(t, harnessRecapURL, string(payload))
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				var out bytes.Buffer
				err = captureRecapAudioFromTargets(ctx, f.port, "owner", harnessRecapURL, &out)
				if f.frameEval.Load() == 0 {
					t.Error("matching frame was not evaluated")
				}
				if tc.oversized {
					if err == nil || !strings.Contains(err.Error(), "recap audio exceeds maximum supported size/duration") {
						t.Errorf("expected resource-limit error, got %v", err)
					}
					if out.Len() != 0 {
						t.Errorf("oversized recording wrote %d bytes", out.Len())
					}
				} else if err != nil {
					t.Errorf("recording exactly at the limits was rejected: %v", err)
				}
			})
		}
	})

	t.Run("JavaScript pre-allocation checks", func(t *testing.T) {
		for _, scenario := range []string{"content-length", "streamed bytes", "duration", "PCM bytes", "at limits"} {
			t.Run(scenario, func(t *testing.T) {
				if strings.TrimSpace(extractRecapAudioJS) == "" {
					t.Fatal("recap audio extraction JavaScript is not implemented")
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "node", "-e", recapAudioLimitProbeJS, scenario)
				cmd.Stdin = strings.NewReader(extractRecapAudioJS)
				output, err := cmd.CombinedOutput()
				if err != nil {
					t.Fatalf("JavaScript %s limit probe: %v: %s", scenario, err, output)
				}
			})
		}
	})
}

// The probe supplies a synthetic media element and instruments the actual
// extraction expression; decoding and PCM allocation fail if reached too early.
const recapAudioLimitProbeJS = `
const fs = require('fs');
const scenario = process.argv[1];
const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 176400;
const media = { src: 'https://example.invalid/recording-a', currentSrc: 'https://example.invalid/recording-a',
  getAttribute: () => 'https://example.invalid/recording-a', querySelectorAll: () => [] };
globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a' };
globalThis.document = { location, URL: location.href, title: 'recording-a',
  querySelector: () => media, querySelectorAll: () => [media] };
let reads = 0, decodes = 0, allocations = 0;
const NativeInt16Array = Int16Array;
globalThis.Int16Array = new Proxy(NativeInt16Array, { construct(target, args) {
  allocations++;
  if (scenario !== 'at limits') throw new Error('PCM allocation reached before rejection');
  return Reflect.construct(target, args);
}});
globalThis.fetch = async () => ({ ok: true, status: 200,
  headers: { get: () => scenario === 'content-length' ? '9' : null },
  body: { getReader: () => ({ read: async () => {
    reads++;
    if (reads > 1) return { done: true };
    return { done: false, value: new Uint8Array(scenario === 'streamed bytes' ? 9 : 8) };
  }, cancel: async () => {} }) },
  arrayBuffer: async () => new ArrayBuffer(1) });
globalThis.AudioContext = class {
  async decodeAudioData() {
    decodes++;
    if (scenario === 'content-length' || scenario === 'streamed bytes') throw new Error('decode reached before source rejection');
    return { duration: scenario === 'duration' ? 3 : 2,
      length: scenario === 'PCM bytes' ? 88201 : 88200, sampleRate: 44100, numberOfChannels: 1,
      getChannelData: () => new Float32Array(88200) };
  }
  async close() {}
};
globalThis.window = globalThis;
globalThis.btoa = s => Buffer.from(s, 'binary').toString('base64');
(async () => {
  let result, failure;
  try { result = await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  const text = failure ? String(failure.message || failure) : JSON.stringify(result);
  if (scenario === 'at limits') {
    if (failure || !result || (typeof result === 'object' && result.error)) throw new Error('at-limit input rejected: ' + text);
    if (!reads || !decodes) throw new Error('at-limit input did not reach reading and decoding');
  } else {
    if (!text || !text.includes('recap audio exceeds maximum supported size/duration')) throw new Error('missing limit error: ' + text);
    if (scenario === 'content-length' && (reads || decodes)) throw new Error('content-length checked after reading or decoding');
    if (scenario === 'streamed bytes' && (!reads || decodes)) throw new Error('stream bytes not checked before decoding');
    if ((scenario === 'duration' || scenario === 'PCM bytes') && (!decodes || allocations)) throw new Error('decoded limit not checked before PCM allocation');
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`

func TestRecapAudioJSRequiresIdentityBeforeFetchAndDecode(t *testing.T) {
	for _, scenario := range []string{"unverified", "mismatched media", "matching"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "node", "-e", recapAudioIdentityProbeJS, scenario)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("JavaScript identity probe: %v: %s", err, output)
			}
		})
	}
}

// Evaluate the production expression with synthetic media and observable I/O.
const recapAudioIdentityProbeJS = `
const fs = require('fs');
const scenario = process.argv[1];
const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 8;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
const pageUrl = scenario === 'unverified'
  ? 'https://teams.cloud.microsoft/l/meetingrecap'
  : 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a';
const mediaUrl = scenario === 'mismatched media'
  ? 'https://example.invalid/audio?driveItemId=recording-b'
  : 'https://example.invalid/audio';
const media = { currentSrc: mediaUrl, src: mediaUrl, getAttribute: () => null };
globalThis.location = { href: pageUrl };
globalThis.window = globalThis;
globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
globalThis.btoa = s => Buffer.from(s, 'binary').toString('base64');
let fetches = 0, decodes = 0;
globalThis.fetch = async () => {
  fetches++;
  let read = false;
  return { ok: true, headers: { get: () => null }, body: { getReader: () => ({
    read: async () => read ? { done: true } : (read = true, { done: false, value: new Uint8Array(1) }),
    cancel: async () => {}
  }) } };
};
globalThis.AudioContext = class {
  async decodeAudioData() {
    decodes++;
    return { duration: 1, length: 1, sampleRate: 44100, numberOfChannels: 1,
      getChannelData: () => new Float32Array(1) };
  }
  async close() {}
};
(async () => {
  let result, failure;
  try { result = await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (scenario === 'matching') {
    if (failure || !result || fetches !== 1 || decodes !== 1)
      throw new Error('matching identity did not fetch and decode: ' + String(failure || result));
  } else {
    if (fetches || decodes)
      throw new Error('unverified or mismatched identity fetched or decoded: fetches=' + fetches + ', decodes=' + decodes);
    if ((failure && !String(failure.message || failure).includes('identity')) || (!failure && result !== ''))
      throw new Error('identity rejection missing: ' + String(failure || result));
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`

func TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable(t *testing.T) {
	f := newRecapAudioFixture(t, harnessRecapURL, "")
	got, err := f.capture(t)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "recap") || !strings.Contains(strings.ToLower(err.Error()), "audio") {
		t.Errorf("expected descriptive recap audio error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("no audio source wrote %d bytes", len(got))
	}
}

func TestRecapAudioCDPRejectsOversizedFrameHeaderBeforeBody(t *testing.T) {
	oldPCM := maxRecapAudioPCMBytes
	maxRecapAudioPCMBytes = 8
	t.Cleanup(func() { maxRecapAudioPCMBytes = oldPCM })

	headerSent := make(chan error, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			headerSent <- err
			return
		}
		defer conn.Close()
		if _, _, err = wsutil.ReadClientData(conn); err != nil {
			headerSent <- err
			return
		}
		// Advertise 32 MiB, but never transmit a payload.
		header := make([]byte, 10)
		header[0], header[1] = 0x81, 127
		binary.BigEndian.PutUint64(header[2:], 32*1024*1024)
		_, err = conn.Write(header)
		headerSent <- err
		if err == nil {
			<-release
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result := make(chan error, 1)
	done := make(chan struct{})
	t.Cleanup(func() { <-done })
	go func() {
		defer close(done)
		_, err := evalInCDPTarget(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), "1")
		result <- err
	}()
	select {
	case err := <-headerSent:
		if err != nil {
			t.Fatalf("send oversized frame header: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("fixture did not send the frame header")
	}
	select {
	case err := <-result:
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "size") {
			t.Fatalf("expected frame size rejection before reading body, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("oversized frame header was not rejected promptly; client waited for a body")
	}
}

func TestRecapAudioCDPRejectsOversizedFragmentedMessage(t *testing.T) {
	oldPCM := maxRecapAudioPCMBytes
	maxRecapAudioPCMBytes = 8
	t.Cleanup(func() { maxRecapAudioPCMBytes = oldPCM })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, _, err := ws.UpgradeHTTP(r, w)
		if err != nil {
			return
		}
		defer conn.Close()
		if _, _, err = wsutil.ReadClientData(conn); err != nil {
			return
		}
		if err = conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
			return
		}
		part := bytes.Repeat([]byte("a"), 1024*1024)
		for i := 0; i < 16; i++ {
			op := ws.OpContinuation
			if i == 0 {
				op = ws.OpText
			}
			if err = ws.WriteFrame(conn, ws.Frame{Header: ws.Header{Fin: i == 15, OpCode: op, Length: int64(len(part))}, Payload: part}); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := evalInCDPTarget(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), "1")
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "size") {
		t.Fatalf("expected accumulated CDP message size rejection, got %v", err)
	}
}

func TestRecapAudioJSRejectsKnownOversizedDurationBeforeFetch(t *testing.T) {
	for _, scenario := range []string{"media", "duration string", "durationSeconds string"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "node", "-e", recapAudioDurationBeforeFetchProbeJS, scenario)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("pre-fetch duration probe: %v: %s", err, output)
			}
		})
	}
}

// A compressed-byte cap cannot bound decodeAudioData's decoded allocation.
// This probe covers known media duration only; unknown or dishonest duration
// metadata needs a separately bounded decoder to prove a pre-decode PCM cap.
const recapAudioDurationBeforeFetchProbeJS = `
	const fs = require('fs');
	const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 8;
	const recapExpectedIdentity = { DriveItemID: 'recording-a' };
	const media = { duration: 3, currentSrc: 'https://example.invalid/audio',
	  src: 'https://example.invalid/audio', getAttribute: () => null };
	globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a' };
	globalThis.window = globalThis;
	if (process.argv[1] !== 'media') {
	  media.duration = NaN;
	  globalThis.g_fileInfo = process.argv[1] === 'duration string' ? { duration: '3' } : { durationSeconds: '3' };
	}
	globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
	let fetches = 0, decodes = 0, result, failure;
	globalThis.fetch = async () => { fetches++; throw new Error('fetch reached before duration rejection'); };
	globalThis.AudioContext = class { async decodeAudioData() { decodes++; throw new Error('decode reached'); } };
	(async () => {
	  try { result = await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
	  const text = failure ? String(failure.message || failure) : JSON.stringify(result);
	  if (!text || !text.includes('recap audio exceeds maximum supported size/duration'))
	    throw new Error('missing duration-limit rejection: ' + text);
	  if (fetches || decodes) throw new Error('duration checked too late: fetches=' + fetches + ', decodes=' + decodes);
	})().catch(e => { console.error(e.message); process.exitCode = 1; });
	`

func TestCaptureRecapAudioAllowsEvaluationLongerThanDOMProbeTimeout(t *testing.T) {
	prevDOM, prevAudio := cdpEvalTimeout, recapAudioEvalTimeout
	cdpEvalTimeout, recapAudioEvalTimeout = 100*time.Millisecond, 3*time.Second
	t.Cleanup(func() { cdpEvalTimeout, recapAudioEvalTimeout = prevDOM, prevAudio })
	pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
	payload := `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-a","pcmChunks":["` + pcm + `"]}`
	f := newRecapAudioFixture(t, harnessRecapURL, payload)
	f.evalDelay.Store(int64(400 * time.Millisecond))
	got, err := f.capture(t)
	if err != nil || len(got) == 0 {
		t.Fatalf("slow audio evaluation must use the audio budget, err=%v bytes=%d", err, len(got))
	}
}

func TestCaptureRecapAudioAcceptsRecordingNameFromMediaPath(t *testing.T) {
	pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
	const meetingURL = "https://teams.cloud.microsoft/l/meetingrecap?fileUrl=https%3A%2F%2Fexample.invalid%2Frec%2FMeeting-A.mp4"
	for _, tc := range []struct {
		name, mediaURL string
		wantErr        bool
	}{
		{"matching media path", "https://example.invalid/media/meeting-a.mp4?sig=1", false},
		{"different media path", "https://example.invalid/media/meeting-b.mp4", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"pageUrl":"https://teams.cloud.microsoft/l/meetingrecap","mediaUrl":"` + tc.mediaURL + `","pcmChunks":["` + pcm + `"]}`
			f := newRecapAudioFixture(t, harnessRecapURL, payload)
			f.meetingURL = meetingURL
			got, err := f.capture(t)
			if tc.wantErr != (err != nil) || tc.wantErr == (len(got) != 0) {
				t.Errorf("err=%v bytes=%d, wantErr=%v", err, len(got), tc.wantErr)
			}
		})
	}
}

func TestRecapAudioIdentityRecordingNameFromPath(t *testing.T) {
	if got := recapAudioIdentity("https://example.invalid/a/Meeting-A.MP4?x=1").RecordingName; got != "meeting-a.mp4" {
		t.Errorf("direct media path recording name = %q", got)
	}
	if got := recapAudioIdentity("https://example.invalid/:v:/p/opaque-token").RecordingName; got != "" {
		t.Errorf("sharing link must not yield a recording name, got %q", got)
	}
	exp := ExtractMeetingIdentity("https://teams.cloud.microsoft/l/meetingrecap?fileUrl=https%3A%2F%2Fexample.invalid%2Fmeeting-a.mp4")
	if !hasRecapAudioIdentityMatch(exp, recapAudioIdentity("https://example.invalid/meeting-a.mp4")) {
		t.Error("matching recording filename must verify identity")
	}
	if hasRecapAudioIdentityMatch(exp, recapAudioIdentity("https://example.invalid/meeting-b.mp4")) {
		t.Error("different recording filename must not verify identity")
	}
}

func TestRecapAudioJSValidatesVroomItemID(t *testing.T) {
	for _, scenario := range []string{"mismatched vroom", "vroom only"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "node", "-e", recapAudioVroomProbeJS, scenario)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("JavaScript VroomItemID probe: %v: %s", err, output)
			}
		})
	}
}

// Evaluate the production expression with a VroomItemID-only recording identity.
const recapAudioVroomProbeJS = `
const fs = require('fs');
const scenario = process.argv[1];
const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 8;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
const vroomOnly = scenario === 'vroom only';
const pageUrl = vroomOnly
  ? 'https://teams.cloud.microsoft/l/meetingrecap'
  : 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a';
const media = { currentSrc: 'https://example.invalid/audio', src: 'https://example.invalid/audio', getAttribute: () => null };
globalThis.location = { href: pageUrl };
globalThis.window = globalThis;
globalThis.g_fileInfo = { VroomItemID: vroomOnly ? 'recording-a' : 'recording-b' };
globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
globalThis.btoa = s => Buffer.from(s, 'binary').toString('base64');
let fetches = 0;
globalThis.fetch = async () => {
  fetches++;
  let read = false;
  return { ok: true, headers: { get: () => null }, body: { getReader: () => ({
    read: async () => read ? { done: true } : (read = true, { done: false, value: new Uint8Array(1) }),
    cancel: async () => {}
  }) } };
};
globalThis.AudioContext = class {
  async decodeAudioData() {
    return { duration: 1, length: 1, sampleRate: 44100, numberOfChannels: 1, getChannelData: () => new Float32Array(1) };
  }
  async close() {}
};
(async () => {
  let result, failure;
  try { result = await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (vroomOnly) {
    if (failure || !result || fetches !== 1) throw new Error('VroomItemID match did not fetch: ' + String(failure || result));
  } else if (fetches || !failure || !String(failure.message || failure).includes('identity')) {
    throw new Error('conflicting VroomItemID was not rejected before fetch: fetches=' + fetches + ' ' + String(failure || result));
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`

func TestCaptureRecapAudioUsesRetainedIdentityForInteractiveSelection(t *testing.T) {
	pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
	for _, tc := range []struct {
		name, itemID string
		wantErr      bool
	}{
		{"selected recording", "recording-a", false},
		{"different recording", "recording-b", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"pageUrl":"` + harnessRecapURL + `","itemId":"` + tc.itemID + `","pcmChunks":["` + pcm + `"]}`
			f := newRecapAudioFixture(t, harnessRecapURL, payload)
			f.meetingURL = "https://teams.microsoft.com/v2/"
			f.recapAudioURL = harnessRecapURL
			got, err := f.capture(t)
			if tc.wantErr != (err != nil) || tc.wantErr == (len(got) != 0) {
				t.Errorf("err=%v bytes=%d, wantErr=%v", err, len(got), tc.wantErr)
			}
		})
	}
}

func TestRetainRecapAudioIdentity(t *testing.T) {
	m := &chromeMeeting{opts: MeetingOptions{MeetingURL: "https://teams.microsoft.com/v2/"}}
	m.retainRecapAudioIdentity("https://teams.cloud.microsoft/l/meetingrecap")
	if m.recapAudioURL != "" {
		t.Errorf("page without identity must not be retained, got %q", m.recapAudioURL)
	}
	m.retainRecapAudioIdentity(harnessRecapURL)
	if m.recapAudioURL != harnessRecapURL {
		t.Errorf("selected recap identity not retained, got %q", m.recapAudioURL)
	}
	specific := &chromeMeeting{opts: MeetingOptions{MeetingURL: harnessRecapURL}}
	specific.retainRecapAudioIdentity("https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-b")
	if specific.recapAudioURL != "" {
		t.Errorf("requested identity must stay authoritative, got %q", specific.recapAudioURL)
	}
}

func TestTryExtractRecapViaCDPWaitsForTranscriptUnlessAISummaryOnly(t *testing.T) {
	var transcriptReady atomic.Bool
	var summaryCalls atomic.Int32
	var serverURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json/list" {
			wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
			targets := []cdpTargetInfo{
				{
					ID:                   "owner",
					Type:                 "page",
					URL:                  harnessRecapURL,
					WebSocketDebuggerURL: wsBase + "/owner",
				},
				{
					ID:                   "frame",
					ParentID:             "owner",
					Type:                 "iframe",
					URL:                  "https://example.sharepoint.com/xplatplugins.aspx",
					WebSocketDebuggerURL: wsBase + "/frame",
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
		reqBytes, _, err := wsutil.ReadClientData(conn)
		if err != nil {
			t.Errorf("read: %v", err)
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
			Params struct {
				Expression string `json:"expression"`
			} `json:"params"`
		}
		_ = json.Unmarshal(reqBytes, &req)
		if req.Method == "Page.getFrameTree" {
			resp := map[string]any{
				"id": req.ID,
				"result": map[string]any{
					"frameTree": map[string]any{
						"frame": map[string]any{"id": "owner", "url": harnessRecapURL},
					},
				},
			}
			b, _ := json.Marshal(resp)
			_ = wsutil.WriteServerText(conn, b)
			return
		}
		value := ""
		switch {
		case strings.Contains(req.Params.Expression, "Copy all"):
			summaryCalls.Add(1)
			value = "Outer page AI summary ready early"
		case strings.Contains(req.Params.Expression, ".ms-List"):
			if r.URL.Path == "/frame" && transcriptReady.Load() {
				value = `{"pageUrl":"` + harnessRecapURL + `","entries":[{"speaker":"Alice","timestamp":"PT2S","text":"Actual recap utterance"}]}`
			} else {
				value = `{"pageUrl":"` + harnessRecapURL + `","entries":[]}`
			}
		}
		resp, _ := json.Marshal(map[string]any{
			"id":     req.ID,
			"result": map[string]any{"result": map[string]string{"value": value}},
		})
		_ = wsutil.WriteServerText(conn, resp)
	}))
	defer srv.Close()
	serverURL = srv.URL
	_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
	require.NoError(t, err)
	port, err := strconv.Atoi(portText)
	require.NoError(t, err)

	ctx := context.Background()

	// 1. ForceRecap: true, AISummaryOnly: false (i.e. --mode past-recap) must NOT
	// return early when only the AI summary is present and .ms-List is still empty.
	recapMeeting := &chromeMeeting{
		opts: MeetingOptions{
			MeetingURL: harnessRecapURL,
			ForceRecap: true,
		},
		capturePageID: "owner",
	}
	require.False(t, recapMeeting.tryExtractRecapViaCDP(ctx, port),
		"ForceRecap without AISummaryOnly must wait for .ms-List transcript entries")
	require.Empty(t, recapMeeting.recapEntries)

	// Once .ms-List populates, ForceRecap extracts both transcript and AI summary.
	transcriptReady.Store(true)
	require.True(t, recapMeeting.tryExtractRecapViaCDP(ctx, port))
	require.Len(t, recapMeeting.recapEntries, 1)
	require.Equal(t, "Actual recap utterance", recapMeeting.recapEntries[0].Text)
	require.Equal(t, "Outer page AI summary ready early", recapMeeting.AISummary())

	// 2. DisableAISummary: true must skip captureAISummaryFromTargets during Recap extraction.
	summaryCalls.Store(0)
	noSummaryMeeting := &chromeMeeting{
		opts: MeetingOptions{
			MeetingURL:       harnessRecapURL,
			ForceRecap:       true,
			DisableAISummary: true,
		},
		capturePageID: "owner",
	}
	require.True(t, noSummaryMeeting.tryExtractRecapViaCDP(ctx, port))
	require.Len(t, noSummaryMeeting.recapEntries, 1)
	require.Empty(t, noSummaryMeeting.AISummary(), "DisableAISummary must suppress AI summary extraction")
	require.Equal(t, int32(0), summaryCalls.Load(), "DisableAISummary must not evaluate copyAISummaryJS")

	// 3. AISummaryOnly: true (e.g. --append-ai-summary) skips .ms-List early return
	// and succeeds as soon as the AI summary is copied even when .ms-List is empty.
	transcriptReady.Store(false)
	summaryOnlyMeeting := &chromeMeeting{
		opts: MeetingOptions{
			MeetingURL:    harnessRecapURL,
			ForceRecap:    true,
			AISummaryOnly: true,
		},
		capturePageID: "owner",
	}
	require.True(t, summaryOnlyMeeting.tryExtractRecapViaCDP(ctx, port))
	require.Empty(t, summaryOnlyMeeting.recapEntries, "AISummaryOnly must skip .ms-List transcript extraction")
	require.Equal(t, "Outer page AI summary ready early", summaryOnlyMeeting.AISummary())
}

func TestRecapAudioJSHandlesBlobFallbackRedirectsCredentialsRetryAndOneHour48kHz(t *testing.T) {
	require.Equal(t, 4200, maxRecapAudioDurationSec, "default Recap audio duration cap should support 1-hour+ meetings (4200s)")
	require.Equal(t, 256*1024*1024, maxRecapAudioSourceBytes, "default Recap audio source cap should be 256 MiB")
	require.Equal(t, 360*1024*1024, maxRecapAudioPCMBytes, "default Recap audio PCM cap should be 360 MiB")

	for _, scenario := range []string{
		"prefers-non-blob-and-retries-omit",
		"resets-identity-per-candidate",
		"rejects-redirect-identity-mismatch",
		"accepts-one-hour-48khz-resampled",
	} {
		t.Run(scenario, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "node", "-e", recapAudioSharePointHardeningProbeJS, scenario)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("JavaScript SharePoint audio hardening probe (%s): %v: %s", scenario, err, output)
			}
		})
	}
}

const recapAudioSharePointHardeningProbeJS = `
const fs = require('fs');
const scenario = process.argv[1];
const maxRecapAudioSourceBytes = 256 * 1024 * 1024;
const maxRecapAudioDurationSec = 4200;
const maxRecapAudioPCMBytes = 360 * 1024 * 1024;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };

const isResetScenario = scenario === 'resets-identity-per-candidate';
const pageUrl = isResetScenario
  ? 'https://teams.cloud.microsoft/l/meetingrecap'
  : 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a';

const media = {
  currentSrc: 'blob:https://tenant-my.sharepoint.com/550e8400-e29b-41d4-a716-446655440000',
  src: 'blob:https://tenant-my.sharepoint.com/550e8400-e29b-41d4-a716-446655440000',
  getAttribute: (name) => {
    if (name !== 'data-audio-src') return null;
    if (isResetScenario) return 'https://tenant-my.sharepoint.com/stream?driveItemId=recording-a';
    return null;
  },
  querySelector: () => null,
};
globalThis.g_fileInfo = isResetScenario
  ? { streamUrl: 'https://tenant-my.sharepoint.com/unverified-fallback' }
  : { streamUrl: 'https://tenant-my.sharepoint.com/streamContent?driveItemId=recording-a' };
globalThis.location = { href: pageUrl };
globalThis.window = globalThis;
globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
globalThis.btoa = s => Buffer.from(s, 'binary').toString('base64');

const fetchCalls = [];
let audioContextOpts = null;
globalThis.fetch = async (url, opts) => {
  fetchCalls.push({ url, opts });
  if (isResetScenario) {
    throw new TypeError('Failed to fetch');
  }
  if (String(url).startsWith('blob:')) {
    throw new Error('blob URL should not be fetched before non-blob streamUrl');
  }
  if (opts && opts.credentials === 'include') {
    throw new TypeError('Failed to fetch');
  }
  const redirectedUrl = scenario === 'rejects-redirect-identity-mismatch'
    ? 'https://tenant-my.sharepoint.com/redirected?driveItemId=recording-b'
    : 'https://tenant-my.sharepoint.com/redirected?driveItemId=recording-a';
  let read = false;
  return {
    ok: true,
    status: 200,
    url: redirectedUrl,
    headers: { get: () => '4' },
    body: {
      getReader: () => ({
        read: async () => read ? { done: true } : (read = true, { done: false, value: new Uint8Array(4) }),
        cancel: async () => {},
      }),
    },
  };
};

globalThis.AudioContext = class {
  constructor(opts) {
    audioContextOpts = opts || null;
  }
  async decodeAudioData() {
    if (scenario === 'accepts-one-hour-48khz-resampled') {
      // Simulate metadata of a 1-hour (3600s) 48 kHz buffer while keeping
      // getChannelData small via a Proxy so the test finishes in milliseconds.
      const proxyChannel = new Proxy(new Float32Array(1), {
        get(target, prop) {
          if (typeof prop === 'string' && !Number.isNaN(Number(prop))) return 0;
          return target[prop];
        }
      });
      return {
        duration: 3600,
        length: 48000,
        sampleRate: 48000,
        numberOfChannels: 1,
        getChannelData: () => proxyChannel,
      };
    }
    return {
      duration: 1,
      length: 1,
      sampleRate: 44100,
      numberOfChannels: 1,
      getChannelData: () => new Float32Array(1),
    };
  }
  async close() {}
};

(async () => {
  let result, failure;
  try { result = await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (scenario === 'prefers-non-blob-and-retries-omit') {
    if (failure) throw new Error('unexpected failure: ' + String(failure.message || failure));
    const parsed = JSON.parse(result);
    if (fetchCalls.length !== 2) throw new Error('expected 2 fetch calls (include then omit), got ' + fetchCalls.length);
    if (fetchCalls[0].opts.redirect !== 'follow' || fetchCalls[1].opts.redirect !== 'follow') {
      throw new Error('expected redirect: follow on fetch calls');
    }
    if (fetchCalls[0].opts.credentials !== 'include' || fetchCalls[1].opts.credentials !== 'omit') {
      throw new Error('expected credentials include then omit');
    }
    if (parsed.mediaUrl !== 'https://tenant-my.sharepoint.com/redirected?driveItemId=recording-a') {
      throw new Error('expected post-redirect mediaUrl, got ' + parsed.mediaUrl);
    }
    if (!audioContextOpts || audioContextOpts.sampleRate !== 44100) {
      throw new Error('expected AudioContext({sampleRate: 44100})');
    }
  } else if (scenario === 'resets-identity-per-candidate') {
    if (!failure || !String(failure.message || failure).includes('identity unverified')) {
      throw new Error('expected second unverified candidate to fail identity check after reset, got: ' + String(failure || result));
    }
  } else if (scenario === 'rejects-redirect-identity-mismatch') {
    if (!failure || !String(failure.message || failure).includes('identity mismatch')) {
      throw new Error('expected post-redirect identity mismatch error, got: ' + String(failure || result));
    }
  } else if (scenario === 'accepts-one-hour-48khz-resampled') {
    if (failure) throw new Error('1-hour 48kHz recording rejected: ' + String(failure.message || failure));
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`
