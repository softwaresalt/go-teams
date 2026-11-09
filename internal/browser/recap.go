package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
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
	const copyBtn = document.querySelector('button[aria-label="Copy all"], button[aria-label*="Copy" i]');
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

func captureAISummaryFromTargets(ctx context.Context, targets []cdpTargetInfo) string {
	for _, t := range targets {
		if t.Type != "page" || t.WebSocketDebuggerURL == "" {
			continue
		}
		if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
			continue
		}
		for attempt := 0; attempt < 6; attempt++ {
			if raw, err := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, copyAISummaryJS); err == nil && strings.TrimSpace(raw) != "" {
				return strings.TrimSpace(raw)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	return ""
}

type cdpTargetInfo struct {
	ID                   string `json:"id"`
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

func launchCleanHeadedBrowser(opts Options, initialURL string) (int, string, func(), error) {
	execPath := ResolveExecPath(opts.ExecPath)
	if execPath == "" {
		return 0, "", nil, fmt.Errorf("browser: no Chromium-based browser executable found")
	}
	var absProfile string
	if opts.UserDataDir != "" {
		abs, err := filepath.Abs(opts.UserDataDir)
		if err != nil {
			return 0, "", nil, fmt.Errorf("browser: resolve profile dir: %w", err)
		}
		if err := os.MkdirAll(abs, 0o700); err != nil {
			return 0, "", nil, fmt.Errorf("browser: create profile dir: %w", err)
		}
		absProfile = abs
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, "", nil, fmt.Errorf("browser: allocate debugging port: %w", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	args := cleanBrowserArgs(port, absProfile, opts.Headless, initialURL)
	cmd := exec.Command(execPath, args...)
	if err := cmd.Start(); err != nil {
		return 0, "", nil, fmt.Errorf("browser: start browser: %w", err)
	}
	cleanup := func() {
		if cmd.Process != nil {
			if runtime.GOOS == "windows" {
				_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
			}
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}

	wsURL, err := waitForCDPBrowserWS(port, 20*time.Second)
	if err != nil {
		cleanup()
		return 0, "", nil, err
	}
	slog.Info("browser: launched clean browser", "pid", cmd.Process.Pid, "cdp_port", port, "headless", opts.Headless)
	return port, wsURL, cleanup, nil
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
			ID     int `json:"id"`
			Result struct {
				Result struct {
					Value any `json:"value"`
				} `json:"result"`
				ExceptionDetails any `json:"exceptionDetails"`
			} `json:"result"`
		}
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == 1 {
			if resp.Result.ExceptionDetails != nil {
				eb, _ := json.Marshal(resp.Result.ExceptionDetails)
				return "", fmt.Errorf("browser: js exception: %s", string(eb))
			}
			if s, ok := resp.Result.Result.Value.(string); ok {
				return s, nil
			}
			vb, _ := json.Marshal(resp.Result.Result.Value)
			return string(vb), nil
		}
	}
}
