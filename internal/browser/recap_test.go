package browser

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRecapTimestamp(t *testing.T) {
	base := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"iso seconds only", "PT0M19S", "00:00:19"},
		{"iso minutes and seconds", "PT15M29S", "00:15:29"},
		{"iso hours minutes seconds", "PT1H2M3S", "01:02:03"},
		{"colon mm:ss", "3:07", "00:03:07"},
		{"colon hh:mm:ss", "01:12:45", "01:12:45"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseRecapTimestamp(tc.raw, base)
			require.True(t, ok)
			assert.Equal(t, tc.want, got.Format("15:04:05"))
		})
	}
}

func TestParseRecapItemsJSON(t *testing.T) {
	raw := `{
		"entries": [
			{"speaker": "Presenter A", "timestamp": "PT0M3S", "text": "", "type": "StartEvent"},
			{"speaker": "Presenter A", "timestamp": "PT0M19S", "text": "Good morning.", "type": "Text"},
			{"speaker": "Presenter B", "timestamp": "PT0M20S", "text": "Hello there.", "type": "Text"}
		]
	}`
	entries, err := parseRecapEntriesPayload(raw)
	require.NoError(t, err)
	require.Len(t, entries, 2, "StartEvent with empty text should be filtered out")
	assert.Equal(t, "Presenter A", entries[0].Speaker)
	assert.Equal(t, "00:00:19", entries[0].Offset)
	assert.Equal(t, "Good morning.", entries[0].Text)
	assert.Equal(t, "Presenter B", entries[1].Speaker)
	assert.Equal(t, "00:00:20", entries[1].Offset)
	assert.Equal(t, "Hello there.", entries[1].Text)
}

func TestCleanHeadedBrowserFlags(t *testing.T) {
	args := cleanHeadedBrowserArgs(9222, `/tmp/profile`, "https://teams.cloud.microsoft")
	for _, a := range args {
		assert.NotContains(t, a, "AutomationControlled",
			"headed browser must not pass --disable-blink-features=AutomationControlled")
		assert.NotContains(t, a, "no-sandbox",
			"headed browser must not disable the browser sandbox")
		assert.NotContains(t, a, "headless",
			"headed browser must not include --headless")
	}
	assert.Contains(t, args, "--remote-debugging-port=9222")
	assert.Contains(t, args, "--use-fake-ui-for-media-stream")
	assert.Contains(t, args, "--auto-select-desktop-capture-source=Microsoft Teams")
}

func TestCleanHeadlessBrowserFlags(t *testing.T) {
	args := cleanBrowserArgs(9222, `/tmp/profile`, true, "https://teams.cloud.microsoft")
	for _, a := range args {
		assert.NotContains(t, a, "AutomationControlled",
			"headless browser must not pass --disable-blink-features=AutomationControlled")
		assert.NotContains(t, a, "no-sandbox",
			"headless browser must not disable the browser sandbox")
	}
	assert.Contains(t, args, "--remote-debugging-port=9222")
	assert.Contains(t, args, "--headless=new")
}

func TestCleanHeadlessAllocatorOptionsDoesNotDisableSandbox(t *testing.T) {
	flags := cleanHeadlessFlags()
	for _, f := range flags {
		assert.NotEqual(t, "no-sandbox", f.Name, "cleanHeadlessAllocatorOptions must not disable the browser sandbox")
		assert.NotEqual(t, "disable-setuid-sandbox", f.Name, "cleanHeadlessAllocatorOptions must not disable setuid sandbox")
		assert.NotEqual(t, "disable-blink-features", f.Name, "cleanHeadlessAllocatorOptions must not pass AutomationControlled")
	}
	headlessFlag, ok := findFlag(flags, "headless")
	require.True(t, ok)
	assert.Equal(t, "new", headlessFlag.Value)

	captureFlag, ok := findFlag(flags, "auto-select-desktop-capture-source")
	require.True(t, ok)
	assert.Equal(t, "Microsoft Teams", captureFlag.Value)

	opts := cleanHeadlessAllocatorOptions(Options{UserDataDir: t.TempDir(), ExecPath: "/custom/edge.exe"})
	assert.Greater(t, len(opts), len(flags))
}

func TestCopyAISummaryJSStructure(t *testing.T) {
	assert.Contains(t, copyAISummaryJS, `data-tid="Notes"`)
	assert.Contains(t, copyAISummaryJS, `data-tid="AINotes"`)
	assert.Contains(t, copyAISummaryJS, `aria-label="Copy all"`)
	assert.NotContains(t, copyAISummaryJS, `aria-label*="Copy"`,
		"summary lookup must be restricted to the AI summary Copy all control")
	assert.Contains(t, copyAISummaryJS, `navigator.clipboard`)
}

func TestIsTargetInCaptureScope(t *testing.T) {
	sharedIframeURL := "https://sharepoint/xplatplugins.aspx"
	pageA := cdpTargetInfo{ID: "page-a", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/a"}
	pageB := cdpTargetInfo{ID: "page-b", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/b"}
	iframeA := cdpTargetInfo{ID: "iframe-a", Type: "iframe", URL: sharedIframeURL, WebSocketDebuggerURL: "ws://127.0.0.1/ia"}
	iframeB := cdpTargetInfo{ID: "iframe-b", Type: "iframe", URL: sharedIframeURL, WebSocketDebuggerURL: "ws://127.0.0.1/ib"}

	frames := ownedFrameSet{
		ids: map[string]bool{"iframe-a": true},
	}

	assert.True(t, isTargetInCaptureScope(pageA, "page-a", frames))
	assert.False(t, isTargetInCaptureScope(pageB, "page-a", frames), "unrelated page tab must be excluded")
	assert.True(t, isTargetInCaptureScope(iframeA, "page-a", frames), "child iframe in Page.getFrameTree must be included")
	assert.False(t, isTargetInCaptureScope(iframeB, "page-a", frames), "distinct iframe ID sharing the same URL must be excluded")
}

func TestVerifyCapturePagePresentFailsClosedWhenBoundTabDisappears(t *testing.T) {
	m := &chromeMeeting{capturePageID: "created-tab", allowSingleTabRebind: true}
	targets := []cdpTargetInfo{
		{ID: "restored-other-tab", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/other"},
	}
	_, ok := m.findBoundCapturePage(targets)
	assert.False(t, ok, "must fail closed when bound capturePageID disappears rather than switching to an unrelated tab")
	assert.Equal(t, "created-tab", m.capturePageID, "capturePageID must not be cleared or reassigned without verified ParentID")

	targetsWithParent := []cdpTargetInfo{
		{ID: "coop-rebound-tab", ParentID: "created-tab", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/rebound"},
	}
	rebound, ok := m.findBoundCapturePage(targetsWithParent)
	require.True(t, ok, "must allow rebind when candidate explicitly reports ParentID == capturePageID")
	assert.Equal(t, "coop-rebound-tab", rebound.ID)
}

func TestIsTrustedSingleTabRebindURLRejectsSpoofedHosts(t *testing.T) {
	assert.True(t, isTrustedSingleTabRebindURL("https://teams.cloud.microsoft/v2/"))
	assert.True(t, isTrustedSingleTabRebindURL("https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
	assert.True(t, isTrustedSingleTabRebindURL("https://microsoft-my.sharepoint.com/personal/user"))
	assert.False(t, isTrustedSingleTabRebindURL("http://teams.cloud.microsoft/v2/"), "non-HTTPS scheme must be rejected")
	assert.False(t, isTrustedSingleTabRebindURL("https://attacker.example/?next=teams.cloud.microsoft"), "query substring must be rejected")
	assert.False(t, isTrustedSingleTabRebindURL("https://teams.cloud.microsoft.attacker.example/"), "spoofed subdomain prefix must be rejected")
	assert.False(t, isTrustedSingleTabRebindURL("https://evilsharepoint.com/"), "non-dot suffix must be rejected")
}

func TestParseRecapStartTime(t *testing.T) {
	ts, ok := parseRecapStartTime("Monday, November 9, 2026 8:00\u202fAM")
	require.True(t, ok)
	assert.Equal(t, "2026-11-09 08:00", ts.Format("2006-01-02 15:04"))

	tsTZ, ok := parseRecapStartTimeInLocation("2026-11-09T15:59:38Z", "America/Los_Angeles")
	require.True(t, ok)
	assert.Equal(t, "2026-11-09 08:00", tsTZ.Format("2006-01-02 15:04"))
}
