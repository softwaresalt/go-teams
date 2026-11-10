package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
)

// RecapOffsetSeparator separates the speaker display name from an HH:MM:SS
// recording offset when delivering Meeting Recap transcript entries to a
// CaptionSink.
const RecapOffsetSeparator = "\x00"

var (
	isoDurationRe = regexp.MustCompile(`^PT(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)
	colonTimeRe   = regexp.MustCompile(`^(?:(\d+):)?(\d{1,2}):(\d{2})$`)
)

// RecapTranscriptEntry is a single utterance extracted from a Teams Meeting
// Recap transcript panel.
type RecapTranscriptEntry struct {
	Speaker string
	Offset  string
	Text    string
}

// AISummaryProvider is an optional interface implemented by Meeting values that
// capture the Teams Meeting Recap AI Summary alongside the transcript.
type AISummaryProvider interface {
	AISummary() string
}

// RecapMeeting is an optional interface implemented by Meeting values that
// report whether the joined session is a Meeting Recap export rather than a
// live call.
type RecapMeeting interface {
	IsRecap() bool
}

// copyAISummaryJS navigates to the Notes -> AI summary tab on the Teams Meeting
// Recap page, hooks clipboard writes (navigator.clipboard and document copy
// events), clicks the "Copy all" button on the AI summary, and returns the
// copied plain-text summary.
const copyAISummaryJS = `(async () => {
	const notesTab = document.querySelector('button[role="tab"][data-tid="Notes"]');
	if (notesTab && notesTab.getAttribute('aria-selected') !== 'true') {
		notesTab.click();
		await new Promise(r => setTimeout(r, 300));
	}
	const aiNotesTab = document.querySelector('button[role="tab"][data-tid="AINotes"]');
	if (aiNotesTab && aiNotesTab.getAttribute('aria-selected') !== 'true') {
		aiNotesTab.click();
		await new Promise(r => setTimeout(r, 300));
	}
	const copyBtn = document.querySelector('button[aria-label="Copy all"]');
	if (!copyBtn) {
		return '';
	}
	let copiedText = '';
	const copyListener = (e) => {
		if (e.clipboardData) {
			const origSetData = e.clipboardData.setData.bind(e.clipboardData);
			e.clipboardData.setData = (type, val) => {
				if (type === 'text/plain') copiedText = String(val || '');
				return origSetData(type, val);
			};
		}
	};
	document.addEventListener('copy', copyListener, true);
	if (navigator.clipboard) {
		const origWriteText = navigator.clipboard.writeText ? navigator.clipboard.writeText.bind(navigator.clipboard) : null;
		navigator.clipboard.writeText = async (txt) => {
			copiedText = String(txt || '');
			if (origWriteText) {
				try { await origWriteText(txt); } catch (_) {}
			}
		};
		const origWrite = navigator.clipboard.write ? navigator.clipboard.write.bind(navigator.clipboard) : null;
		navigator.clipboard.write = async (items) => {
			try {
				for (const item of items) {
					if (item.types && item.types.includes('text/plain')) {
						const blob = await item.getType('text/plain');
						copiedText = await blob.text();
					}
				}
			} catch (_) {}
			if (origWrite) {
				try { await origWrite(items); } catch (_) {}
			}
		};
	}
	copyBtn.click();
	await new Promise(r => setTimeout(r, 500));
	document.removeEventListener('copy', copyListener, true);
	return copiedText;
})()`

// ownedFrameSet holds the verified frame IDs belonging to a specific page
// target's frame tree (obtained via Page.getFrameTree).
type ownedFrameSet struct {
	ids map[string]bool
}

func (s ownedFrameSet) contains(t cdpTargetInfo) bool {
	return t.ID != "" && s.ids[t.ID]
}

// isTargetInCaptureScope reports whether t belongs to the active capture page
// (either the page tab itself or one of its child iframes in ownedFrames) so
// unrelated open or restored browser tabs are never scraped or attached.
func isTargetInCaptureScope(t cdpTargetInfo, capturePageID string, ownedFrames ownedFrameSet) bool {
	if (t.Type != "page" && t.Type != "iframe") || t.WebSocketDebuggerURL == "" {
		return false
	}
	if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
		return false
	}
	if capturePageID == "" {
		return true
	}
	if t.Type == "page" {
		return t.ID == capturePageID
	}
	// Real Chromium/Edge /json/list populates parentId on out-of-process
	// iframes (OOPIFs such as cross-origin SharePoint xplatplugins.aspx), while
	// Page.getFrameTree only lists same-process local frames. Check parentId
	// first when present, and fall back to Page.getFrameTree frame IDs.
	if t.ParentID != "" {
		return t.ParentID == capturePageID
	}
	return ownedFrames.contains(t)
}

type cdpFrameTreeNode struct {
	Frame struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	} `json:"frame"`
	ChildFrames []cdpFrameTreeNode `json:"childFrames"`
}

func collectFrameTree(node cdpFrameTreeNode, out *ownedFrameSet) {
	if node.Frame.ID != "" {
		out.ids[node.Frame.ID] = true
	}
	for _, child := range node.ChildFrames {
		collectFrameTree(child, out)
	}
}

func fetchOwnedFrames(ctx context.Context, pageWSURL string) ownedFrameSet {
	out := ownedFrameSet{
		ids: make(map[string]bool),
	}
	if pageWSURL == "" {
		return out
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, pageWSURL)
	if err != nil {
		return out
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Page.getFrameTree",
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return out
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return out
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return out
		}
		var resp struct {
			ID     int `json:"id"`
			Result struct {
				FrameTree cdpFrameTreeNode `json:"frameTree"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			collectFrameTree(resp.Result.FrameTree, &out)
			return out
		}
	}
}

func createCDPPageTarget(ctx context.Context, browserWSURL, initialURL string) (string, error) {
	if initialURL == "" {
		initialURL = "about:blank"
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, browserWSURL)
	if err != nil {
		return "", fmt.Errorf("browser: dial browser websocket: %w", err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Target.createTarget",
		"params": map[string]any{
			"url": initialURL,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return "", err
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return "", err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result struct {
				TargetID string `json:"targetId"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return "", fmt.Errorf("browser: Target.createTarget error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			if resp.Result.TargetID == "" {
				return "", fmt.Errorf("browser: Target.createTarget returned empty targetId")
			}
			return resp.Result.TargetID, nil
		}
	}
}

func captureAISummaryFromTargets(ctx context.Context, port int, ownerPageID string) string {
	for attempt := 0; attempt < 20; attempt++ {
		if ctx.Err() != nil {
			return ""
		}
		// The outer recap page and its Copy all button can appear after the
		// transcript iframe, so discover targets again on every attempt.
		targets, err := listCDPTargets(port)
		if err == nil {
			for _, t := range targets {
				if t.Type != "page" || t.WebSocketDebuggerURL == "" {
					continue
				}
				if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
					continue
				}
				if ownerPageID != "" && t.ID != ownerPageID {
					continue
				}
				if raw, err := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, copyAISummaryJS); err == nil && strings.TrimSpace(raw) != "" {
					return strings.TrimSpace(raw)
				}
			}
		}
		if attempt == 19 {
			break
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ""
		case <-timer.C:
		}
	}
	return ""
}

type cdpTargetInfo struct {
	ID                   string `json:"id"`
	ParentID             string `json:"parentId"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// cleanBrowserArgs returns the minimal, untampered command-line flags used to
// launch a browser instance (headed or --headless=new) without stealth/sandbox
// flags such as --disable-blink-features=AutomationControlled or --no-sandbox.
func cleanBrowserArgs(port int, profileDir string, headless bool, initialURL string) []string {
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-first-run",
		"--no-default-browser-check",
	}
	if headless {
		args = append(args, "--headless=new")
	}
	if profileDir != "" {
		args = append(args, fmt.Sprintf("--user-data-dir=%s", profileDir))
	}
	if initialURL != "" {
		args = append(args, initialURL)
	}
	return args
}

// hasHTTPSHostSuffix parses rawURL and reports whether it is an https:// URL
// whose hostname equals one of domains or is a dot-prefixed subdomain of one
// of domains.
func hasHTTPSHostSuffix(rawURL string, domains ...string) bool {
	u, err := url.Parse(rawURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return false
	}
	for _, d := range domains {
		d = strings.ToLower(d)
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

// isAuthRedirectURL reports whether rawURL is on a Microsoft / identity
// provider login host where CDP Runtime.evaluate probes should be paused until
// interactive sign-in redirects back to Teams.
func isAuthRedirectURL(rawURL string) bool {
	return hasHTTPSHostSuffix(rawURL,
		"login.microsoftonline.com",
		"login.live.com",
		"login.windows.net",
		"autologon.microsoftazuread-sso.com",
		"device.login.microsoftonline.com",
		"aadcdn.msftauth.net",
		"msauth.net",
		"duosecurity.com",
		"okta.com",
	)
}

func cleanHeadedBrowserArgs(port int, profileDir, initialURL string) []string {
	return cleanBrowserArgs(port, profileDir, false, initialURL)
}

// ParseRecapTimestamp parses an ISO-8601 duration (e.g. "PT15M29S", "PT1H2M3S")
// or colon-separated offset (e.g. "3:07", "01:12:45") into a time.Time anchored
// at midnight on base's date so Format("15:04:05") renders the exact HH:MM:SS
// recording offset.
func ParseRecapTimestamp(raw string, base time.Time) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	var hours, minutes, seconds int
	if m := isoDurationRe.FindStringSubmatch(trimmed); m != nil {
		if m[1] == "" && m[2] == "" && m[3] == "" {
			return time.Time{}, false
		}
		if m[1] != "" {
			hours, _ = strconv.Atoi(m[1])
		}
		if m[2] != "" {
			minutes, _ = strconv.Atoi(m[2])
		}
		if m[3] != "" {
			seconds, _ = strconv.Atoi(m[3])
		}
	} else if m := colonTimeRe.FindStringSubmatch(trimmed); m != nil {
		if m[1] != "" {
			hours, _ = strconv.Atoi(m[1])
		}
		minutes, _ = strconv.Atoi(m[2])
		seconds, _ = strconv.Atoi(m[3])
	} else {
		return time.Time{}, false
	}
	midnight := time.Date(base.Year(), base.Month(), base.Day(), 0, 0, 0, 0, base.Location())
	d := time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
	return midnight.Add(d), true
}

const extractRecapTranscriptJS = `(() => {
	const listEl = document.querySelector('.ms-List');
	if (listEl) {
		const fiberKey = Object.keys(listEl).find(k => k.startsWith('__reactFiber$') || k.startsWith('__reactInternalInstance$'));
		let cur = fiberKey ? listEl[fiberKey] : null;
		for (let depth = 0; cur && depth < 25; depth++, cur = cur.return) {
			const p = cur.memoizedProps || cur.pendingProps;
			if (p && Array.isArray(p.items) && p.items.length > 0) {
				const entries = p.items.map(it => ({
					speaker: String(it.speakerDisplayName || it.speaker || '').trim(),
					timestamp: String(it.timestamp || it.startTime || '').trim(),
					text: String(it.text || '').trim(),
					type: String(it.type || 'Text')
				}));
				return JSON.stringify({ entries });
			}
		}
	}
	const cells = Array.from(document.querySelectorAll('.ms-List-cell, [data-transcript-entry-id]'));
	if (cells.length === 0) {
		return JSON.stringify({ entries: [] });
	}
	const entries = [];
	const seen = new Set();
	for (const cell of cells) {
		const idEl = cell.hasAttribute('data-transcript-entry-id') ? cell : cell.querySelector('[data-transcript-entry-id]');
		const id = idEl ? idEl.getAttribute('data-transcript-entry-id') : '';
		if (id && seen.has(id)) continue;
		if (id) seen.add(id);
		const spkEl = cell.querySelector('[data-transcript-copy-field="speaker"], [class*="itemDisplayName-"]');
		const tsEl = cell.querySelector('[data-transcript-copy-field="timestamp"], [id^="Header-timestamp-"]');
		const txtEl = cell.querySelector('[class*="entryText-"]');
		if (!txtEl) continue;
		const text = (txtEl.textContent || '').trim();
		if (!text) continue;
		entries.push({
			speaker: spkEl ? (spkEl.textContent || '').trim() : '',
			timestamp: tsEl ? (tsEl.textContent || '').trim() : '',
			text: text,
			type: 'Text'
		});
	}
	return JSON.stringify({ entries });
})()`

type rawRecapPayload struct {
	Entries []struct {
		Speaker   string `json:"speaker"`
		Timestamp string `json:"timestamp"`
		Text      string `json:"text"`
		Type      string `json:"type"`
	} `json:"entries"`
}

func parseRecapEntriesPayload(raw string) ([]RecapTranscriptEntry, error) {
	var p rawRecapPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil, fmt.Errorf("browser: parse recap transcript payload: %w", err)
	}
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]RecapTranscriptEntry, 0, len(p.Entries))
	for _, item := range p.Entries {
		text := strings.TrimSpace(item.Text)
		if text == "" {
			continue
		}
		speaker := strings.TrimSpace(item.Speaker)
		if speaker == "" {
			speaker = "Speaker"
		}
		var offset string
		if ts, ok := ParseRecapTimestamp(item.Timestamp, base); ok {
			offset = ts.Format("15:04:05")
		}
		out = append(out, RecapTranscriptEntry{
			Speaker: speaker,
			Offset:  offset,
			Text:    text,
		})
	}
	return out, nil
}

func launchCleanHeadedBrowser(opts Options, initialURL string) (int, string, string, <-chan struct{}, func(), error) {
	execPath := ResolveExecPath(opts.ExecPath)
	if execPath == "" {
		return 0, "", "", nil, nil, fmt.Errorf("browser: no Chromium-based browser executable found")
	}
	var absProfile string
	if opts.UserDataDir != "" {
		abs, err := filepath.Abs(opts.UserDataDir)
		if err != nil {
			return 0, "", "", nil, nil, fmt.Errorf("browser: resolve profile dir: %w", err)
		}
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return 0, "", "", nil, nil, fmt.Errorf("browser: create profile dir: %w", err)
		}
		absProfile = abs
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", "", nil, nil, fmt.Errorf("browser: allocate debugging port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Launch directly with initialURL so the browser opens a single tab (no
	// extra about:blank tab). Once DevTools is ready, bind capturePageID to
	// that single page target (or create one if none exists yet).
	targetURL := initialURL
	if targetURL == "" {
		targetURL = "about:blank"
	}
	args := cleanBrowserArgs(port, absProfile, opts.Headless, targetURL)
	cmd := exec.Command(execPath, args...)
	if err := cmd.Start(); err != nil {
		return 0, "", "", nil, nil, fmt.Errorf("browser: start browser: %w", err)
	}
	procDone := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(procDone)
	}()
	cleanup := func() {
		if cmd.Process != nil {
			if runtime.GOOS == "windows" {
				_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
			}
			_ = cmd.Process.Kill()
			<-procDone
		}
	}

	wsURL, err := waitForCDPBrowserWS(port, 20*time.Second)
	if err != nil {
		cleanup()
		return 0, "", "", nil, nil, err
	}
	targetID := waitForInitialPageTargetID(port, 5*time.Second)
	if targetID == "" {
		targetID, err = createCDPPageTarget(context.Background(), wsURL, targetURL)
		if err != nil {
			cleanup()
			return 0, "", "", nil, nil, err
		}
	}
	slog.Info("browser: launched clean browser", "pid", cmd.Process.Pid, "cdp_port", port, "target_id", targetID, "headless", opts.Headless)
	return port, wsURL, targetID, procDone, cleanup, nil
}

func waitForInitialPageTargetID(port int, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		targets, err := listCDPTargets(port)
		if err == nil {
			var candidates []cdpTargetInfo
			for _, t := range targets {
				if t.Type != "page" || t.WebSocketDebuggerURL == "" {
					continue
				}
				if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
					continue
				}
				candidates = append(candidates, t)
			}
			if len(candidates) == 1 {
				return candidates[0].ID
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return ""
}

func waitForCDPBrowserWS(port int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	versionURL := fmt.Sprintf("http://127.0.0.1:%d/json/version", port)
	client := &http.Client{Timeout: 2 * time.Second}
	for time.Now().Before(deadline) {
		resp, err := client.Get(versionURL)
		if err == nil {
			var v struct {
				WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&v)
			_ = resp.Body.Close()
			if v.WebSocketDebuggerURL != "" {
				return v.WebSocketDebuggerURL, nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", fmt.Errorf("browser: DevTools endpoint on port %d did not become ready", port)
}

func listCDPTargets(port int) ([]cdpTargetInfo, error) {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	var targets []cdpTargetInfo
	if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func evalInCDPTarget(ctx context.Context, wsURL, expr string) (string, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, wsURL)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Runtime.evaluate",
		"params": map[string]any{
			"expression":    expr,
			"returnByValue": true,
			"awaitPromise":  true,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return "", err
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return "", err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
			Result struct {
				Result struct {
					Type  string `json:"type"`
					Value any    `json:"value"`
				} `json:"result"`
				ExceptionDetails any `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return "", fmt.Errorf("browser: cdp error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			if resp.Result.ExceptionDetails != nil {
				eb, _ := json.Marshal(resp.Result.ExceptionDetails)
				return "", fmt.Errorf("browser: js exception: %s", string(eb))
			}
			if resp.Result.Result.Value == nil {
				return "", nil
			}
			if s, ok := resp.Result.Result.Value.(string); ok {
				return s, nil
			}
			vb, _ := json.Marshal(resp.Result.Result.Value)
			return string(vb), nil
		}
	}
}

// navigateCDPTarget sends a top-level CDP Page.navigate command to wsURL,
// which is the programmatic equivalent of pasting targetURL into the browser's
// address bar. Unlike Runtime.evaluate("location.href = ..."), Page.navigate
// succeeds even when the tab is mid-redirect in an authentication loop.
func navigateCDPTarget(ctx context.Context, wsURL, targetURL string) error {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, _, _, err := ws.Dial(dialCtx, wsURL)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))

	req := map[string]any{
		"id":     1,
		"method": "Page.navigate",
		"params": map[string]any{
			"url": targetURL,
		},
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	if err := wsutil.WriteClientText(conn, payload); err != nil {
		return err
	}
	for {
		msg, _, err := wsutil.ReadServerData(conn)
		if err != nil {
			return err
		}
		var resp struct {
			ID    int `json:"id"`
			Error *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Error != nil {
				return fmt.Errorf("browser: Page.navigate error %d: %s", resp.Error.Code, resp.Error.Message)
			}
			return nil
		}
	}
}
