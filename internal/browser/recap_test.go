package browser

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
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
	t.Setenv("TEAMS_DISABLE_SANDBOX", "")
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
	t.Setenv("TEAMS_DISABLE_SANDBOX", "")
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

func TestCleanBrowserArgsHonorsDisableSandboxOptIn(t *testing.T) {
	const (
		profile = `/tmp/profile`
		target  = "https://teams.cloud.microsoft"
	)
	sandboxFlags := []string{"--no-sandbox", "--disable-setuid-sandbox"}

	tests := []struct {
		name         string
		env          string
		opts         Options
		wantNoSbx    bool
		wantHeadless bool
	}{
		{name: "default keeps sandbox", env: "", opts: Options{}, wantNoSbx: false},
		{name: "env 0 keeps sandbox", env: "0", opts: Options{}, wantNoSbx: false},
		{name: "option opts out of sandbox", env: "", opts: Options{DisableSandbox: true}, wantNoSbx: true},
		{name: "env 1 opts out of sandbox", env: "1", opts: Options{}, wantNoSbx: true},
		{name: "env true opts out of sandbox", env: "true", opts: Options{}, wantNoSbx: true},
		{name: "env TRUE opts out of sandbox", env: "TRUE", opts: Options{}, wantNoSbx: true},
		{name: "headless option and env", env: "1", opts: Options{Headless: true}, wantNoSbx: true, wantHeadless: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", tc.env)
			args := cleanBrowserArgsForOptions(9222, profile, tc.opts, target)

			require.NotEmpty(t, args)
			assert.Equal(t, target, args[len(args)-1], "initialURL must remain the trailing argument")
			assert.Contains(t, args, "--remote-debugging-port=9222")
			assert.Equal(t, tc.wantHeadless, containsArg(args, "--headless=new"))
			for _, a := range args {
				assert.NotContains(t, a, "AutomationControlled")
			}
			for _, flag := range sandboxFlags {
				if !tc.wantNoSbx {
					assert.NotContains(t, args, flag, "sandbox must stay enabled by default")
					continue
				}
				idx := indexOfArg(args, flag)
				require.GreaterOrEqual(t, idx, 0, "%s must be emitted on opt-in", flag)
				assert.Less(t, idx, len(args)-1, "%s must precede initialURL", flag)
			}
		})
	}
}

func TestCleanBrowserArgsWrappersHonorSandboxEnvOptIn(t *testing.T) {
	t.Setenv("TEAMS_DISABLE_SANDBOX", "1")
	headed := cleanHeadedBrowserArgs(9222, `/tmp/profile`, "https://teams.cloud.microsoft")
	assert.Contains(t, headed, "--no-sandbox")
	assert.Contains(t, headed, "--disable-setuid-sandbox")
	assert.False(t, containsArg(headed, "--headless=new"))
	assert.Equal(t, "https://teams.cloud.microsoft", headed[len(headed)-1])

	headless := cleanBrowserArgs(9222, `/tmp/profile`, true, "https://teams.cloud.microsoft")
	assert.Contains(t, headless, "--no-sandbox")
	assert.Contains(t, headless, "--disable-setuid-sandbox")
	assert.True(t, containsArg(headless, "--headless=new"))

	t.Setenv("TEAMS_DISABLE_SANDBOX", "")
	assert.NotContains(t, cleanHeadedBrowserArgs(9222, `/tmp/profile`, "about:blank"), "--no-sandbox")
	assert.NotContains(t, cleanBrowserArgs(9222, `/tmp/profile`, true, "about:blank"), "--no-sandbox")
}

func containsArg(args []string, want string) bool { return indexOfArg(args, want) >= 0 }

func indexOfArg(args []string, want string) int {
	for i, a := range args {
		if a == want {
			return i
		}
	}
	return -1
}

func TestCleanHeadlessAllocatorOptionsDoesNotDisableSandbox(t *testing.T) {
	t.Setenv("TEAMS_DISABLE_SANDBOX", "")
	flags := cleanHeadlessFlags(Options{})
	for _, name := range []string{"no-sandbox", "disable-setuid-sandbox"} {
		f, ok := findFlag(flags, name)
		require.Truef(t, ok, "%s must be emitted explicitly so chromedp's root default is overridden", name)
		assert.Equal(t, false, f.Value, "cleanHeadlessAllocatorOptions must not disable %s by default", name)
	}
	for _, f := range flags {
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

func TestExtractMeetingIdentityAndVerifyRecapTargetMatchesMeeting(t *testing.T) {
	submittedURL := "https://teams.cloud.microsoft/l/meetingrecap?driveId=b%21syntheticDriveId123&driveItemId=01SYNTHETICITEMID456&fileUrl=https%3A%2F%2Fcontoso-my.sharepoint.com%2Fpersonal%2Fpresenter_contoso_com%2FDocuments%2FRecordings%2FARCHITECTURE+SYNC+OPTIONS-20261109_105939-Meeting+Recording.mp4%3Fweb%3D1&iCalUid=040000008200E00074C5B7101A82E00800000000SYNTHETICICALUID&threadId=19%3Ameeting_U1lOVEhFVElDX01FRVRJTkdfVE9LRU4%40thread.v2&callId=00000000-1111-2222-3333-444444444444"

	id := ExtractMeetingIdentity(submittedURL)
	assert.Equal(t, "19:meeting_u1lovehfveldx01frvrjtkdfve9lru4@thread.v2", id.ThreadID)
	assert.Equal(t, "meeting_u1lovehfveldx01frvrjtkdfve9lru4", id.MeetingToken)
	assert.Equal(t, "01syntheticitemid456", id.DriveItemID)
	assert.Equal(t, "00000000-1111-2222-3333-444444444444", id.CallID)
	assert.Equal(t, "architecture sync options-20261109_105939-meeting recording.mp4", id.RecordingName)

	// Rebind check: a restored tab for a different meeting must be rejected,
	// whereas a tab carrying the submitted meeting's threadId/driveItemId is accepted.
	m := &chromeMeeting{
		opts:                 MeetingOptions{MeetingURL: submittedURL},
		capturePageID:        "original-tab",
		allowSingleTabRebind: true,
	}
	otherMeetingTab := []cdpTargetInfo{
		{
			ID:                   "restored-other-meeting",
			Type:                 "page",
			URL:                  "https://teams.cloud.microsoft/_#/l/meetingrecap?threadId=19%3Ameeting_OTHERMEETING%40thread.v2&driveItemId=01OTHERITEM",
			WebSocketDebuggerURL: "ws://127.0.0.1/other",
		},
	}
	_, ok := m.findBoundCapturePage(otherMeetingTab)
	assert.False(t, ok, "restored tab for a different meeting ID must be rejected")

	sameMeetingTab := []cdpTargetInfo{
		{
			ID:                   "post-auth-same-meeting",
			Type:                 "page",
			URL:                  "https://teams.cloud.microsoft/_#/l/meetingrecap?threadId=19%3Ameeting_U1lOVEhFVElDX01FRVRJTkdfVE9LRU4%40thread.v2&driveItemId=01SYNTHETICITEMID456",
			WebSocketDebuggerURL: "ws://127.0.0.1/same",
		},
	}
	rebound, ok := m.findBoundCapturePage(sameMeetingTab)
	require.True(t, ok, "tab carrying the exact submitted meeting IDs must be matched")
	assert.Equal(t, "post-auth-same-meeting", rebound.ID)

	// Extraction-time check: payload from a mismatched recording file or driveItemId must be rejected.
	mismatchedPayload := `{
		"title": "Some Other Meeting",
		"fileName": "Other Meeting-20261109_090000-Meeting Recording.mp4",
		"itemId": "01DIFFERENTITEMID",
		"entries": [{"speaker": "Alice", "timestamp": "PT10S", "text": "Wrong meeting", "type": "Text"}]
	}`
	_, _, err := parseRecapPayloadForMeeting(mismatchedPayload, submittedURL, sameMeetingTab[0].URL, "https://contoso-my.sharepoint.com/xplatplugins.aspx")
	require.Error(t, err, "extraction must reject payload whose recording identity does not match submitted URL")

	matchingPayload := `{
		"title": "ARCHITECTURE SYNC OPTIONS",
		"fileName": "ARCHITECTURE SYNC OPTIONS-20261109_105939-Meeting Recording.mp4",
		"itemId": "01SYNTHETICITEMID456",
		"entries": [{"speaker": "Presenter A", "timestamp": "PT19S", "text": "Good morning.", "type": "Text"}]
	}`
	entries, meta, err := parseRecapPayloadForMeeting(matchingPayload, submittedURL, sameMeetingTab[0].URL, "https://contoso-my.sharepoint.com/xplatplugins.aspx")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "ARCHITECTURE SYNC OPTIONS", meta.Title)

	// SharePoint /:v:/p/ sharing links in fileUrl carry an opaque sharing token, not an .mp4 filename;
	// verifyRecapTargetMatchesMeeting must not reject the real .mp4 filename in g_fileInfo.name.
	sharingLinkURL := "https://teams.microsoft.com/l/meetingrecap?driveId=b%21syntheticDriveId&driveItemId=0153RCEHUPKMC42MOY75E2K7UM6XPTXKBA&fileUrl=https%3A%2F%2Fcontoso-my.sharepoint.com%2F%3Av%3A%2Fp%2Fpresenter%2FIQCPUwXNMdj_SaV-jPXfO6ggAU7_E5VW7OxYAZmKSEjRQo8&threadId=19%3Ameeting_ZWM5NTU3NjctYWE3Yi00ZTQwLTg4ODEtNTZmZTg0NmEyODA3%40thread.v2&subType=RecapSharingLink_RecapChiclet&recapType=RecordingAndTranscript"
	sharingID := ExtractMeetingIdentity(sharingLinkURL)
	assert.Empty(t, sharingID.RecordingName, "opaque SharePoint /:v:/p/ sharing token must not be treated as a recording filename")
	sharingPayload := `{
		"title": "All the Vibes: Hack & Furious",
		"fileName": "All the Vibes Hack _ Furious-20261110_175804UTC-Meeting Recording.mp4",
		"entries": [{"speaker": "Presenter A", "timestamp": "PT5S", "text": "Welcome everyone.", "type": "Text"}]
	}`
	sharingEntries, sharingMeta, err := parseRecapPayloadForMeeting(sharingPayload, sharingLinkURL, "https://teams.microsoft.com/v2/", "https://contoso-my.sharepoint.com/personal/presenter/_layouts/15/xplatplugins.aspx?uniqueId=cd05538f")
	require.NoError(t, err)
	require.Len(t, sharingEntries, 1)
	assert.Equal(t, "All the Vibes: Hack & Furious", sharingMeta.Title)
}

func TestIsAuthRedirectURLIncludesMicrosoftLoginHosts(t *testing.T) {
	for _, u := range []string{
		"https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
		"https://login.microsoft.com/consumers/fido/get",
		"https://account.live.com/proofs/Manage",
		"https://mysignins.microsoft.com/security-info",
		"https://account.activedirectory.windowsazure.com/RelyingPartyStrongAuthentication",
		"https://aadcdn.msftauth.net/shared/1.0/content/js/ConvergedLogin.js",
	} {
		assert.Truef(t, isAuthRedirectURL(u), "expected auth redirect URL: %s", u)
	}
}

func TestHasTeamsSessionInProfile(t *testing.T) {
	assert.False(t, hasTeamsSessionInProfile(""), "empty path must return false")
	dir := t.TempDir()
	assert.False(t, hasTeamsSessionInProfile(dir), "new directory without Default/Local Storage/leveldb must return false")

	leveldbDir := filepath.Join(dir, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(leveldbDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(leveldbDir, "000001.log"), []byte("https://login.microsoftonline.com some-other-key"), 0o600))
	assert.False(t, hasTeamsSessionInProfile(dir), "profile with only login page storage and no Teams tokens must return false")

	require.NoError(t, os.WriteFile(filepath.Join(leveldbDir, "000002.ldb"), []byte("_https://teams.cloud.microsoft\x00\x01SKYPE-TOKEN-value"), 0o600))
	assert.True(t, hasTeamsSessionInProfile(dir), "profile containing SKYPE-TOKEN in leveldb must return true")
}

func TestHeadedAuthTrackerColdProfileBootstrapsTeamsV2AndDoesNotLoopDuringLogin(t *testing.T) {
	recapURL := "https://teams.cloud.microsoft/l/meetingrecap?driveId=b%21123&driveItemId=01ABC&threadId=19%3Ameeting_XYZ%40thread.v2"

	tracker, initialURL := newHeadedAuthTracker(recapURL, true)
	assert.Equal(t, teamsV2BootstrapURL, initialURL, "cold profile for meetingrecap URL must bootstrap at /v2/ first")

	_, msInitialURL := newHeadedAuthTracker("https://teams.microsoft.com/l/meetingrecap?threadId=19%3Ameeting_XYZ%40thread.v2", true)
	assert.Equal(t, "https://teams.microsoft.com/v2/", msInitialURL, "cold profile for teams.microsoft.com recap URL must bootstrap on the same teams.microsoft.com/v2/ origin")

	// Simulate multi-host interactive login (login.microsoftonline.com -> login.microsoft.com -> login.microsoftonline.com)
	// and extended user MFA wait (15 polls): must never yank the browser away from the login flow.
	assert.Empty(t, tracker.onAuthRedirect("https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
	assert.Empty(t, tracker.onAuthRedirect("https://login.microsoft.com/consumers/fido/get"))
	for range 15 {
		assert.Empty(t, tracker.onAuthRedirect("https://login.microsoftonline.com/common/SAS/ProcessAuth"))
	}

	// Simulate redirect back to teams.cloud.microsoft/v2/#code=... while OAuth code exchange is in flight.
	for range 5 {
		nav := tracker.onNonAuthPage(headedPageState{
			PageURL:      "https://teams.cloud.microsoft/v2/#code=0.AXIA...",
			Host:         "teams.cloud.microsoft",
			Path:         "/v2/",
			Hash:         "#code=0.AXIA...",
			ReadyState:   "complete",
			AuthInFlight: true,
			MsalReady:    true,
		})
		assert.Empty(t, nav, "must never navigate while OAuth callback code exchange is in flight")
	}

	// Simulate Teams splash screen where msal.2.account.keys exists but neither SKYPE-TOKEN nor the app shell (>15 data-tid) is ready.
	for range 5 {
		nav := tracker.onNonAuthPage(headedPageState{
			PageURL:      "https://teams.cloud.microsoft/v2/",
			Host:         "teams.cloud.microsoft",
			Path:         "/v2/",
			ReadyState:   "complete",
			AuthInFlight: false,
			MsalReady:    true,
			SkypeReady:   false,
			TidCount:     3,
		})
		assert.Empty(t, nav, "must not navigate while Teams splash screen is still acquiring tokens")
	}

	// Once Teams v2 shell and Skype token are ready on /v2/, wait 3 polls to settle, then navigate ONCE to recapURL.
	settledV2 := headedPageState{
		PageURL:    "https://teams.cloud.microsoft/v2/",
		Host:       "teams.cloud.microsoft",
		Path:       "/v2/",
		ReadyState: "complete",
		MsalReady:  true,
		SkypeReady: true,
		TidCount:   42,
	}
	assert.Empty(t, tracker.onNonAuthPage(settledV2))
	assert.Empty(t, tracker.onNonAuthPage(settledV2))
	assert.Equal(t, recapURL, tracker.onNonAuthPage(settledV2), "third settled poll on /v2/ must navigate to the target recap URL")

	// While the recap page is loading (even if it takes 15 polls / 22.5s before RecapHeader or xplatplugins.aspx mounts),
	// it must NOT re-navigate in a 3s/9s loop.
	loadingRecap := headedPageState{
		PageURL:    recapURL,
		Host:       "teams.cloud.microsoft",
		Path:       "/l/meetingrecap",
		ReadyState: "complete",
		MsalReady:  true,
		SkypeReady: true,
		TidCount:   30,
	}
	for range 15 {
		assert.Empty(t, tracker.onNonAuthPage(loadingRecap), "must not interrupt recap page while it is loading")
	}

	// Warm profile launched directly at recapURL (reNavigateAttempts == 0) must also allow at least
	// 15 polls (22.5s) of on-target recap loading without interrupting it.
	warmTracker, warmInitialURL := newHeadedAuthTracker(recapURL, false)
	assert.Equal(t, recapURL, warmInitialURL)
	for range 15 {
		assert.Empty(t, warmTracker.onNonAuthPage(loadingRecap), "warm profile must not reload recapURL mid-load")
	}
}

func TestHeadedAuthTrackerBreaksRapidRecapAuthBounceLoopViaTeamsV2(t *testing.T) {
	recapURL := "https://teams.cloud.microsoft/l/meetingrecap?driveId=b%21123&driveItemId=01ABC&threadId=19%3Ameeting_XYZ%40thread.v2"

	// Warm-disk profile launches directly at recapURL, but session is unauthenticated and /l/meetingrecap bounces rapidly.
	tracker, initialURL := newHeadedAuthTracker(recapURL, false)
	assert.Equal(t, recapURL, initialURL)

	unsettledRecap := headedPageState{
		PageURL:    recapURL,
		Host:       "teams.cloud.microsoft",
		Path:       "/l/meetingrecap",
		ReadyState: "complete",
	}
	// Bounce 1: Teams -> Auth
	assert.Empty(t, tracker.onNonAuthPage(unsettledRecap))
	assert.Empty(t, tracker.onAuthRedirect("https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
	// Bounce 2: Teams -> Auth
	assert.Empty(t, tracker.onNonAuthPage(unsettledRecap))
	assert.Empty(t, tracker.onAuthRedirect("https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
	// Bounce 3: Teams -> Auth (rapid bounce loop detected on second poll of 3rd bounce)
	assert.Empty(t, tracker.onNonAuthPage(unsettledRecap))
	assert.Empty(t, tracker.onAuthRedirect("https://login.microsoftonline.com/common/oauth2/v2.0/authorize"))
	gotNav := tracker.onAuthRedirect("https://login.microsoftonline.com/common/oauth2/v2.0/authorize")
	assert.Equal(t, teamsV2BootstrapURL, gotNav, "rapid /l/meetingrecap auth bounce loop must escape to /v2/ bootstrap URL, not re-apply the looping recap URL")
}

func TestFindBoundCapturePageRebindsSingleTabAcrossAuthAndTeamsHostsWithoutSwitchingToConcurrentTab(t *testing.T) {
	recapURL := "https://teams.cloud.microsoft/l/meetingrecap?driveId=b%21123&driveItemId=01ABC&threadId=19%3Ameeting_XYZ%40thread.v2"
	m := &chromeMeeting{
		opts:                 MeetingOptions{MeetingURL: recapURL},
		capturePageID:        "tab-1",
		allowSingleTabRebind: true,
	}

	// COOP targetId swap onto login.microsoftonline.com (ParentID is empty on page targets in /json/list).
	rebound, ok := m.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-2-auth", Type: "page", URL: "https://login.microsoftonline.com/common/oauth2/v2.0/authorize", WebSocketDebuggerURL: "ws://127.0.0.1/2"},
	})
	require.True(t, ok, "must follow single-tab COOP swap onto login.microsoftonline.com")
	assert.Equal(t, "tab-2-auth", rebound.ID)

	// COOP targetId swap onto login.microsoft.com.
	rebound, ok = m.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-3-mslogin", Type: "page", URL: "https://login.microsoft.com/consumers/fido/get", WebSocketDebuggerURL: "ws://127.0.0.1/3"},
	})
	require.True(t, ok, "must follow single-tab COOP swap onto login.microsoft.com")
	assert.Equal(t, "tab-3-mslogin", rebound.ID)

	// COOP targetId swap back onto generic teams.cloud.microsoft/v2/ landing page (no threadId in URL yet).
	rebound, ok = m.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-4-teams-home", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/4"},
	})
	require.True(t, ok, "must follow single-tab COOP swap onto teams.cloud.microsoft/v2/ post-auth landing page")
	assert.Equal(t, "tab-4-teams-home", rebound.ID)

	// Now suppose a second concurrent tab ("concurrent-other-tab") is opened while tab-4-teams-home is still present.
	rebound, ok = m.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-4-teams-home", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/4"},
		{ID: "concurrent-other-tab", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/other"},
	})
	require.True(t, ok)
	assert.Equal(t, "tab-4-teams-home", rebound.ID)

	// If tab-4-teams-home is then closed, leaving only concurrent-other-tab, findBoundCapturePage must fail closed.
	_, ok = m.findBoundCapturePage([]cdpTargetInfo{
		{ID: "concurrent-other-tab", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/other"},
	})
	assert.False(t, ok, "must fail closed when bound tab closes and remaining tab was a known concurrent tab")

	// Ambiguous missing-target poll with two candidates (B and C) followed by only C must also fail closed.
	m2 := &chromeMeeting{
		opts:                 MeetingOptions{MeetingURL: recapURL},
		capturePageID:        "tab-a",
		allowSingleTabRebind: true,
	}
	_, ok = m2.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-b", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/b"},
		{ID: "tab-c", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/c"},
	})
	assert.False(t, ok)
	_, ok = m2.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-c", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/c"},
	})
	assert.False(t, ok, "candidate seen in ambiguous multi-tab missing-target poll must not be rebound later")

	// Concurrent tab initially at edge://newtab/ alongside capture tab A must be remembered so it cannot be rebound later.
	m3 := &chromeMeeting{
		opts:                 MeetingOptions{MeetingURL: recapURL},
		capturePageID:        "tab-a",
		allowSingleTabRebind: true,
	}
	_, ok = m3.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-a", Type: "page", URL: recapURL, WebSocketDebuggerURL: "ws://127.0.0.1/a"},
		{ID: "tab-edge-newtab", Type: "page", URL: "edge://newtab/", WebSocketDebuggerURL: "ws://127.0.0.1/newtab"},
	})
	require.True(t, ok)
	_, ok = m3.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-edge-newtab", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/newtab"},
	})
	assert.False(t, ok, "concurrent edge:// tab that later navigates to Teams /v2/ must not be rebound")

	// Candidate with a nonempty ParentID pointing to a different page must be rejected by the identity-free fallback.
	m4 := &chromeMeeting{
		opts:                 MeetingOptions{MeetingURL: recapURL},
		capturePageID:        "tab-a",
		allowSingleTabRebind: true,
	}
	_, ok = m4.findBoundCapturePage([]cdpTargetInfo{
		{ID: "tab-child-of-b", ParentID: "tab-b", Type: "page", URL: "https://teams.cloud.microsoft/v2/", WebSocketDebuggerURL: "ws://127.0.0.1/child"},
	})
	assert.False(t, ok, "candidate whose nonempty ParentID points to a different page must be rejected")

	// Outer page URL containing sharepoint.com in its fileUrl query must not be classified as a recap iframe.
	assert.False(t, isRecapIframeTarget(cdpTargetInfo{
		ID:   "main-page",
		Type: "page",
		URL:  "https://teams.microsoft.com/l/meetingrecap?fileUrl=https%3A%2F%2Fcontoso-my.sharepoint.com%2Fvideo.mp4",
	}))
	assert.True(t, isRecapIframeTarget(cdpTargetInfo{
		ID:   "sp-iframe",
		Type: "iframe",
		URL:  "https://contoso-my.sharepoint.com/personal/user/_layouts/15/xplatplugins.aspx",
	}))
}

func TestResolveExistingProfileDirExpansionAndFallback(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TEST_APPDATA_DIR", base)

	legacyProfile := filepath.Join(base, "teams-cli", "profile")
	leveldbDir := filepath.Join(legacyProfile, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(leveldbDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(leveldbDir, "000001.ldb"), []byte("SKYPE-TOKEN"), 0o600))

	raw := `%TEST_APPDATA_DIR%\go-teams\profile`
	got := resolveExistingProfileDir(raw)
	assert.Equal(t, legacyProfile, got, "must expand %VAR% and fall back to teams-cli profile when go-teams lacks a session")

	// When go-teams already has a Teams session, it must be preferred over teams-cli.
	modernProfile := filepath.Join(base, "go-teams", "profile")
	modernLeveldb := filepath.Join(modernProfile, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(modernLeveldb, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(modernLeveldb, "000001.ldb"), []byte("SKYPE-TOKEN"), 0o600))
	assert.Equal(t, modernProfile, resolveExistingProfileDir(raw))

	// Rightmost go-teams segment only: ancestor go-teams dir must not be rewritten if rightmost segment matches.
	nestedRoot := filepath.Join(base, "go-teams", "nested")
	nestedLegacy := filepath.Join(nestedRoot, "teams-cli", "profile")
	nestedLegacyLDB := filepath.Join(nestedLegacy, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(nestedLegacyLDB, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(nestedLegacyLDB, "000001.ldb"), []byte("SKYPE-TOKEN"), 0o600))
	nestedRaw := filepath.Join(nestedRoot, "go-teams", "profile")
	assert.Equal(t, nestedLegacy, resolveExistingProfileDir(nestedRaw))

	// Repository root go-teams segment (containing go.mod) must never be rewritten to sibling teams-cli.
	repoBase := t.TempDir()
	fakeRepoRoot := filepath.Join(repoBase, "go-teams")
	require.NoError(t, os.MkdirAll(filepath.Join(fakeRepoRoot, "bin"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(fakeRepoRoot, "go.mod"), []byte("module example\n"), 0o600))
	siblingLegacy := filepath.Join(repoBase, "teams-cli", "bin", "profile")
	siblingLDB := filepath.Join(siblingLegacy, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(siblingLDB, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(siblingLDB, "000001.ldb"), []byte("SKYPE-TOKEN"), 0o600))
	repoLocalProfile := filepath.Join(fakeRepoRoot, "bin", "profile")
	assert.Equal(t, repoLocalProfile, resolveExistingProfileDir(repoLocalProfile))
}

func TestNewLiveMeetingExpandsProfileOncePreservingLiteralVarTokens(t *testing.T) {
	base := t.TempDir()
	literalDir := filepath.Join(base, "$CACHE")
	legacyProfile := filepath.Join(literalDir, "teams-cli", "profile")
	leveldbDir := filepath.Join(legacyProfile, "Default", "Local Storage", "leveldb")
	require.NoError(t, os.MkdirAll(leveldbDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(leveldbDir, "000001.ldb"), []byte("SKYPE-TOKEN"), 0o600))

	t.Setenv("PROFILE_BASE", literalDir)
	t.Setenv("CACHE", "wrong-cache-dir")

	m, _, cleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true, UserDataDir: "%PROFILE_BASE%/go-teams/profile"},
		MeetingOptions{MeetingURL: "https://teams.cloud.microsoft/l/meetingrecap?threadId=19%3Ameeting_x%40thread.v2", BotName: "Agent"})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}
	cm, ok := m.(*chromeMeeting)
	require.True(t, ok)
	require.NotNil(t, cm.headedOpts)
	assert.Equal(t, legacyProfile, cm.userDataDir)
	assert.Equal(t, legacyProfile, cm.headedOpts.UserDataDir)
	assert.True(t, cm.managedProfile)
	assert.True(t, hasTeamsSessionInExpandedProfile(cm.headedOpts.UserDataDir))
}

func TestNewLiveMeetingRemoteHeadlessUsesCleanLauncherAndJoinOnWebSel(t *testing.T) {
	assert.Contains(t, joinOnWebSel, "#openTeamsClientInBrowser")

	base := t.TempDir()
	profile := filepath.Join(base, "go-teams", "profile")
	m, _, cleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true, UserDataDir: profile},
		MeetingOptions{MeetingURL: "https://teams.cloud.microsoft/l/meetingrecap?threadId=19%3Ameeting_x%40thread.v2", BotName: "Agent"})
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}
	cm, ok := m.(*chromeMeeting)
	require.True(t, ok)
	require.NotNil(t, cm.headedOpts, "remote headless session with UserDataDir must route through cm.headedOpts")
	assert.True(t, cm.headedOpts.Headless)

	// Local loopback httptest URLs must remain on the fast cleanHeadlessAllocatorOptions path.
	localM, _, localCleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true, UserDataDir: profile},
		MeetingOptions{MeetingURL: "https://127.0.0.1:8443/meet/123", BotName: "Agent"})
	require.NoError(t, err)
	if localCleanup != nil {
		defer localCleanup()
	}
	localCM, ok := localM.(*chromeMeeting)
	require.True(t, ok)
	assert.Nil(t, localCM.headedOpts)
}

func TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. extractRecapAudioJS handles VroomItemId (lowercase d) and reports isDownloadBlocked=true
	// when SharePoint returns text/html (even across redirect URLs and trailing blob: candidates)
	// without calling decodeAudioData.
	audioProbeJS := `
const fs = require('fs');
const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 8;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
const media = {
  currentSrc: 'blob:https://contoso.sharepoint.com/1234',
  src: 'https://contoso.sharepoint.com/download?driveItemId=recording-a',
  getAttribute: () => null
};
globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap' };
globalThis.window = globalThis;
globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
let decodes = 0;
globalThis.fetch = async (url) => {
  if (String(url).startsWith('blob:')) {
    throw new TypeError('Failed to fetch');
  }
  return {
    ok: true,
    status: 200,
    url: 'https://contoso.sharepoint.com/_layouts/15/AccessDenied.aspx',
    headers: { get: (k) => k.toLowerCase() === 'content-type' ? 'text/html; charset=utf-8' : null },
    body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }
  };
};
globalThis.AudioContext = class {
  async decodeAudioData() { decodes++; throw new Error('decodeAudioData must not be called on text/html'); }
  async close() {}
};
(async () => {
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('isDownloadBlocked=true')) {
    throw new Error('expected isDownloadBlocked=true error, got: ' + String(failure));
  }
  if (decodes !== 0) {
    throw new Error('decodeAudioData was called ' + decodes + ' times');
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`
	cmd := exec.CommandContext(ctx, "node", "-e", audioProbeJS)
	cmd.Stdin = strings.NewReader(extractRecapAudioJS)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("extractRecapAudioJS VroomItemId/isDownloadBlocked probe failed: %v: %s", err, out)
	}

	// 2. extractRecapAudioJS reports isDownloadBlocked=true on HTTP 403 (!resp.ok) and fetch CORS TypeError.
	for _, tc := range []struct {
		name      string
		fetchBody string
	}{
		{
			name:      "http-403",
			fetchBody: `return { ok: false, status: 403, headers: { get: () => null } };`,
		},
		{
			name:      "cors-reject",
			fetchBody: `throw new TypeError('Failed to fetch');`,
		},
	} {
		probe := `
const fs = require('fs');
const maxRecapAudioSourceBytes = 8, maxRecapAudioDurationSec = 2, maxRecapAudioPCMBytes = 8;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
const media = { src: 'https://contoso.sharepoint.com/download?driveItemId=recording-a', getAttribute: () => null };
globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap' };
globalThis.window = globalThis;
globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
globalThis.document = { querySelector: selector => selector === 'audio, video' ? media : null };
globalThis.fetch = async () => { ` + tc.fetchBody + ` };
(async () => {
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('isDownloadBlocked=true')) {
    throw new Error('expected isDownloadBlocked=true error, got: ' + String(failure));
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`
		c := exec.CommandContext(ctx, "node", "-e", probe)
		c.Stdin = strings.NewReader(extractRecapAudioJS)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("extractRecapAudioJS %s probe failed: %v: %s", tc.name, err, out)
		}
	}

	// 3. extractRecapTranscriptJS captures VroomItemId (lowercase d).
	transcriptProbeJS := `
const fs = require('fs');
globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap' };
globalThis.window = globalThis;
globalThis.g_fileInfo = { title: 'Sync', name: 'Sync.mp4', VroomItemId: '01LOWERCASEITEMID' };
globalThis.document = { querySelector: () => null, querySelectorAll: () => [] };
const raw = eval(fs.readFileSync(0, 'utf8'));
const parsed = JSON.parse(raw);
if (parsed.itemId !== '01LOWERCASEITEMID') {
  throw new Error('expected itemId 01LOWERCASEITEMID, got: ' + parsed.itemId);
}
`
	cmd2 := exec.CommandContext(ctx, "node", "-e", transcriptProbeJS)
	cmd2.Stdin = strings.NewReader(extractRecapTranscriptJS)
	if out, err := cmd2.CombinedOutput(); err != nil {
		t.Fatalf("extractRecapTranscriptJS VroomItemId probe failed: %v: %s", err, out)
	}

	// 4. Inert isDownloadBlocked=true page with no candidates and no stream surface throws isDownloadBlocked=true (U3).
	inertBlockedProbeJS := `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1024, maxRecapAudioDurationSec = 60, maxRecapAudioPCMBytes = 10240;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
globalThis.location = { href: 'https://contoso.sharepoint.com/_layouts/15/xplatplugins.aspx?driveItemId=recording-a' };
globalThis.window = globalThis;
globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
globalThis.document = { querySelector: () => null, querySelectorAll: () => [] };
(async () => {
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('isDownloadBlocked=true')) {
    throw new Error('expected isDownloadBlocked=true error on inert blocked page, got: ' + String(failure));
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`
	cmd3 := exec.CommandContext(ctx, "node", "-e", inertBlockedProbeJS)
	cmd3.Stdin = strings.NewReader(extractRecapAudioJS)
	if out, err := cmd3.CombinedOutput(); err != nil {
		t.Fatalf("extractRecapAudioJS inert isDownloadBlocked probe failed: %v: %s", err, out)
	}

	// 5. Direct-fetch success path strips secret query tokens from mediaUrl (U4).
	directFetchSanitizeProbeJS := `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1024, maxRecapAudioDurationSec = 60, maxRecapAudioPCMBytes = 10240;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
const media = {
  src: 'https://contoso.sharepoint.com/download?driveItemId=recording-a&tempauth=secret-tok&sig=s1',
  getAttribute: () => null
};
globalThis.location = { href: 'https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a' };
globalThis.window = globalThis;
globalThis.g_fileInfo = { VroomItemId: 'recording-a' };
globalThis.document = { querySelector: sel => sel === 'audio, video' ? media : null, querySelectorAll: () => [] };
globalThis.fetch = async (url) => {
  const bytes = new Uint8Array([1, 2, 3, 4]);
  let sent = false;
  return {
    ok: true,
    status: 200,
    url: String(url),
    headers: { get: (k) => k.toLowerCase() === 'content-type' ? 'audio/mp4' : null },
    body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) }
  };
};
globalThis.AudioContext = class {
  async decodeAudioData() {
    return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
  }
  async close() {}
};
(async () => {
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (/tempauth|sig=/.test(String(parsed.mediaUrl || ''))) {
    throw new Error('direct-fetch mediaUrl leaked secret query parameter: ' + parsed.mediaUrl);
  }
  if (!String(parsed.mediaUrl || '').includes('driveItemId=recording-a')) {
    throw new Error('expected driveItemId preserved in mediaUrl, got: ' + parsed.mediaUrl);
  }
})().catch(e => { console.error(e.message); process.exitCode = 1; });
`
	cmd4 := exec.CommandContext(ctx, "node", "-e", directFetchSanitizeProbeJS)
	cmd4.Stdin = strings.NewReader(extractRecapAudioJS)
	if out, err := cmd4.CombinedOutput(); err != nil {
		t.Fatalf("extractRecapAudioJS direct-fetch sanitizeMediaUrl probe failed: %v: %s", err, out)
	}
}

func TestRecapJSStreamEmbedHighSpeedAudioCaptureWhenDownloadBlocked(t *testing.T) {
	const mp4HelpersJS = `
function u32(v) {
  return [(v >>> 24) & 0xff, (v >>> 16) & 0xff, (v >>> 8) & 0xff, v & 0xff];
}
function u64(v) {
  const hi = Math.floor(v / 0x100000000);
  const lo = v >>> 0;
  return [...u32(hi), ...u32(lo)];
}
function str4(s) {
  return [s.charCodeAt(0), s.charCodeAt(1), s.charCodeAt(2), s.charCodeAt(3)];
}
function box(type, ...parts) {
  const len = 8 + parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(len);
  out.set(u32(len), 0);
  out.set(str4(type), 4);
  let off = 8;
  for (const p of parts) {
    out.set(p, off);
    off += p.length;
  }
  return out;
}
function buildAudioInitSegment() {
  const ftyp = box('ftyp', new Uint8Array([...str4('iso5'), ...u32(1), ...str4('mp41')]));
  const hdlr = box('hdlr', new Uint8Array([0,0,0,0, 0,0,0,0, ...str4('soun'), ...new Array(12).fill(0), 0]));
  const mp4a = box('mp4a', new Uint8Array([...new Array(6).fill(0), 0,1, ...new Array(8).fill(0), 0,1, 0,16, 0,0, 0,0, 0xac,0x44,0,0]));
  const stsd = box('stsd', new Uint8Array([0,0,0,0, ...u32(1)]), mp4a);
  const stbl = box('stbl', stsd);
  const minf = box('minf', stbl);
  const mdia = box('mdia', hdlr, minf);
  const trak = box('trak', mdia);
  const moov = box('moov', trak);
  const out = new Uint8Array(ftyp.length + moov.length);
  out.set(ftyp, 0);
  out.set(moov, ftyp.length);
  return out;
}
function buildVideoInitSegment() {
  const ftyp = box('ftyp', new Uint8Array([...str4('iso5'), ...u32(1), ...str4('mp41')]));
  const hdlr = box('hdlr', new Uint8Array([0,0,0,0, 0,0,0,0, ...str4('vide'), ...new Array(12).fill(0), 0]));
  const avc1 = box('avc1', new Uint8Array(24));
  const stsd = box('stsd', new Uint8Array([0,0,0,0, ...u32(1)]), avc1);
  const stbl = box('stbl', stsd);
  const minf = box('minf', stbl);
  const mdia = box('mdia', hdlr, minf);
  const trak = box('trak', mdia);
  const moov = box('moov', trak);
  const out = new Uint8Array(ftyp.length + moov.length);
  out.set(ftyp, 0);
  out.set(moov, ftyp.length);
  return out;
}
function buildMediaSegment(tfdtTime, tagByte) {
  const styp = box('styp', new Uint8Array([...str4('msdh'), ...u32(0), ...str4('msdh')]));
  const mfhd = box('mfhd', new Uint8Array([0,0,0,0, ...u32(1)]));
  const tfhd = box('tfhd', new Uint8Array([0,2,0,0, ...u32(1)]));
  const tfdt = box('tfdt', new Uint8Array([1,0,0,0, ...u64(tfdtTime)]));
  const traf = box('traf', tfhd, tfdt);
  const moof = box('moof', mfhd, traf);
  const mdat = box('mdat', new Uint8Array([0xde, 0xad, 0xbe, 0xef, tagByte, tagByte, tagByte, tagByte]));
  const out = new Uint8Array(styp.length + moof.length + mdat.length);
  out.set(styp, 0);
  out.set(moof, styp.length);
  out.set(mdat, styp.length + moof.length);
  return out;
}
function collectTFDTs(buf) {
  const u8 = new Uint8Array(buf);
  const out = [];
  function walk(start, end) {
    let pos = start;
    while (pos + 8 <= end) {
      const size = ((u8[pos] << 24) | (u8[pos+1] << 16) | (u8[pos+2] << 8) | u8[pos+3]) >>> 0;
      if (size < 8 || pos + size > end) break;
      const type = String.fromCharCode(u8[pos+4], u8[pos+5], u8[pos+6], u8[pos+7]);
      if (type === 'moof' || type === 'traf') {
        walk(pos + 8, pos + size);
      } else if (type === 'tfdt' && pos + 16 <= pos + size) {
        const ver = u8[pos+8];
        if (ver === 1 && pos + 20 <= pos + size) {
          const hi = ((u8[pos+12] << 24) | (u8[pos+13] << 16) | (u8[pos+14] << 8) | u8[pos+15]) >>> 0;
          const lo = ((u8[pos+16] << 24) | (u8[pos+17] << 16) | (u8[pos+18] << 8) | u8[pos+19]) >>> 0;
          out.push(hi * 0x100000000 + lo);
        } else {
          const val = ((u8[pos+12] << 24) | (u8[pos+13] << 16) | (u8[pos+14] << 8) | u8[pos+15]) >>> 0;
          out.push(val);
        }
      }
      pos += size;
    }
  }
  walk(0, u8.length);
  return out;
}
`

	t.Run("path-a-webcrypto-aes-cbc-parallel-onedrive-transcode", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };

(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array([1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16]);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);

  const initPlain = buildAudioInitSegment();
  const seg0Plain = buildMediaSegment(0, 0x11);
  const seg1Plain = buildMediaSegment(220500, 0x22);
  const seg2Plain = buildMediaSegment(441000, 0x33);

  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, initPlain));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, seg0Plain));
  const encSeg1 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, seg1Plain));
  const encSeg2 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, seg2Plain));

  const fetchedAudioTimes = [];
  const rawVideoRequests = [];
  let pathAStarted = false;

  const baseTranscode = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  const initURL = baseTranscode + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=150000000&driveItemId=recording-a&tempauth=secret-query-token';
  const seg0URL = baseTranscode + '?part=mediasegment&track=audio&quality=audcopy&segmentTime=0&wsd=50000000&ppd=150000000&driveItemId=recording-a&tempauth=secret-query-token';
  const videoSegURL = baseTranscode + '?part=mediasegment&track=video&quality=high&segmentTime=50000000&wsd=50000000&ppd=150000000&driveItemId=recording-a&tempauth=secret-query-token';

  function makeResp(url, bytes) {
    let sent = false;
    return {
      ok: true,
      status: 200,
      url,
      headers: { get: (k) => k.toLowerCase() === 'content-type' ? 'video/mp4' : (k.toLowerCase() === 'content-length' ? String(bytes.byteLength) : null) },
      body: {
        getReader: () => ({
          read: async () => {
            if (sent) return { done: true };
            sent = true;
            return { done: false, value: bytes };
          },
          cancel: async () => {}
        })
      },
      arrayBuffer: async () => bytes.slice().buffer
    };
  }

  globalThis.fetch = async (url) => {
    const u = new URL(String(url));
    if (!u.pathname.includes('oneDrive.transcode')) {
      throw new TypeError('Failed to fetch');
    }
    const part = u.searchParams.get('part');
    const track = u.searchParams.get('track');
    const segTime = u.searchParams.get('segmentTime');
    if (track === 'video') {
      rawVideoRequests.push(String(url));
      return makeResp(String(url), encSeg0);
    }
    if (part === 'initsegment') {
      return makeResp(String(url), encInit);
    }
    if (part === 'mediasegment') {
      fetchedAudioTimes.push(segTime);
      if (segTime === '0') return makeResp(String(url), encSeg0);
      if (segTime === '50000000') { pathAStarted = true; return makeResp(String(url), encSeg1); }
      if (segTime === '100000000') { pathAStarted = true; return makeResp(String(url), encSeg2); }
    }
    throw new Error('unexpected transcode request: ' + String(url));
  };

  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    muted: false,
    volume: 1,
    playbackRate: 1,
    paused: true,
    duration: 15,
    readyState: 4,
    play: async () => { videoEl.paused = false; },
    pause: () => { videoEl.paused = true; },
    addEventListener: () => {},
    removeEventListener: () => {},
    getAttribute: () => null,
    querySelector: () => null
  };

  const playBtn = {
    click: () => {
      (async () => {
        const rInit = await globalThis.fetch(initURL);
        const abInit = await rInit.arrayBuffer();
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, abInit);
        const rSeg0 = await globalThis.fetch(seg0URL);
        const abSeg0 = await rSeg0.arrayBuffer();
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, abSeg0);
        setTimeout(() => {
          globalThis.fetch(videoSegURL).catch(() => {});
        }, 15);
      })().catch(() => {});
    }
  };

  globalThis.document = {
    querySelector: (sel) => {
      if (sel.includes('Play video')) return playBtn;
      if (sel === 'audio, video' || sel === 'video') return videoEl;
      return null;
    },
    querySelectorAll: (sel) => sel === 'iframe' ? [] : []
  };

  let decodedTFDTs = [];
  globalThis.AudioContext = class {
    async decodeAudioData(ab) {
      decodedTFDTs = collectTFDTs(ab);
      return {
        sampleRate: 44100,
        length: 4410,
        duration: 0.1,
        numberOfChannels: 1,
        getChannelData: () => new Float32Array(4410).fill(0.25)
      };
    }
    async close() {}
  };

  const rawResult = await eval(fs.readFileSync(0, 'utf8'));
  if (!rawResult) {
    throw new Error('expected JSON payload from extractRecapAudioJS, got empty string');
  }
  const parsed = JSON.parse(rawResult);
  if (parsed.itemId !== 'recording-a') {
    throw new Error('unexpected itemId: ' + parsed.itemId);
  }
  if (String(parsed.mediaUrl || '').includes('tempauth')) {
    throw new Error('mediaUrl leaked secret query parameter tempauth: ' + parsed.mediaUrl);
  }
  for (const want of ['0', '50000000', '100000000']) {
    if (!fetchedAudioTimes.includes(want)) {
      throw new Error('missing audio segmentTime ' + want + ', got: ' + JSON.stringify(fetchedAudioTimes));
    }
  }
  if (pathAStarted && rawVideoRequests.length > 0) {
    throw new Error('expected track=video requests to be stripped, got: ' + JSON.stringify(rawVideoRequests));
  }
  if (JSON.stringify(decodedTFDTs) !== JSON.stringify([0, 220500, 441000])) {
    throw new Error('expected decoded fMP4 tfdt timestamps [0,220500,441000], got: ' + JSON.stringify(decodedTFDTs));
  }
  if (!Array.isArray(parsed.pcmChunks) || parsed.pcmChunks.length === 0) {
    throw new Error('expected non-empty pcmChunks');
  }
  const pcmBuf = Buffer.from(parsed.pcmChunks[0], 'base64');
  if (pcmBuf.byteLength !== 8820 || Math.abs(pcmBuf.readInt16LE(0) - 8191) > 2) {
    throw new Error('unexpected 44.1kHz mono PCM bytes: len=' + pcmBuf.byteLength + ' sample0=' + pcmBuf.readInt16LE(0));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
		cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
		cmd.Stdin = strings.NewReader(extractRecapAudioJS)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("path-a probe failed: %v\n%s", err, out)
		}
	})

	t.Run("path-b-mse-sourcebuffer-hook-16x-currenttime-stepping-and-child-iframe", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };

(async () => {
  const audioInit = buildAudioInitSegment();
  const videoInit = buildVideoInitSegment();
  const segA = buildMediaSegment(0, 0x01);
  const segB = buildMediaSegment(220500, 0x02);
  const segBDup = buildMediaSegment(220500, 0x02);
  const segC = buildMediaSegment(441000, 0x03);
  const videoMedia = buildMediaSegment(0, 0x99);

  let nativeVideoAppends = 0;
  let observedMuted = false;
  const observedPlaybackRates = [];
  const observedCurrentTimes = [];

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this._end = 0;
      this.buffered = {
        get length() { return this._sb._end > 0 ? 1 : 0; },
        start: () => 0,
        end: () => this._end,
        _sb: null
      };
      this.buffered._sb = this;
    }
    appendBuffer(data) {
      if (this.mime.includes('video')) {
        nativeVideoAppends++;
      }
      this.updating = true;
      setTimeout(() => {
        this.updating = false;
        this.dispatchEvent(new Event('updateend'));
      }, 2);
    }
    remove(start, end) {
      this.updating = true;
      setTimeout(() => {
        this.updating = false;
        this.dispatchEvent(new Event('updateend'));
      }, 2);
    }
    abort() {}
  }

  class FakeMediaSource extends EventTarget {
    constructor() {
      super();
      this.readyState = 'open';
      this.sourceBuffers = [];
    }
    addSourceBuffer(mime) {
      const sb = new FakeSourceBuffer(mime);
      this.sourceBuffers.push(sb);
      return sb;
    }
    removeSourceBuffer(sb) {
      const idx = this.sourceBuffers.indexOf(sb);
      if (idx < 0) throw new Error('NotFoundError');
      this.sourceBuffers.splice(idx, 1);
    }
    endOfStream() { this.readyState = 'ended'; }
  }

  let _muted = false;
  let _volume = 1;
  let _rate = 1;
  let _time = 0;
  let segCAppended = false;
  let audioSB = null;

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/mse-1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/mse-1',
    duration: 15,
    paused: true,
    ended: false,
    readyState: 4,
    get muted() { return _muted; },
    set muted(v) { _muted = Boolean(v); if (_muted) observedMuted = true; },
    get volume() { return _volume; },
    set volume(v) { _volume = Number(v); },
    get playbackRate() { return _rate; },
    set playbackRate(v) { _rate = Number(v); observedPlaybackRates.push(_rate); },
    get currentTime() { return _time; },
    set currentTime(v) {
      _time = Number(v);
      observedCurrentTimes.push(_time);
      if (_time >= 4.5 && !segCAppended && audioSB) {
        segCAppended = true;
        audioSB._end = 15;
        audioSB.appendBuffer(segC);
      }
      if (_time >= 14.5) {
        videoEl.ended = true;
      }
    },
    play: async () => { videoEl.paused = false; },
    pause: () => { videoEl.paused = true; },
    addEventListener: () => {},
    removeEventListener: () => {},
    getAttribute: () => null,
    querySelector: () => null
  };

  const childPlayBtn = {
    click: () => {
      const ms = new childWin.MediaSource();
      const videoSB = ms.addSourceBuffer('video/mp4; codecs="avc1.640028"');
      audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      videoSB.appendBuffer(videoInit);
      videoSB.appendBuffer(videoMedia);
      audioSB._end = 10;
      audioSB.appendBuffer(audioInit);
      // Append out-of-order (segB before segA where tfdt === 0) plus a duplicate segBDup.
      audioSB.appendBuffer(segB);
      audioSB.appendBuffer(segA);
      audioSB.appendBuffer(segBDup);
    }
  };

  let decodedBytes = null;
  class FakeAudioContext {
    async decodeAudioData(ab) {
      decodedBytes = new Uint8Array(ab);
      return {
        sampleRate: 44100,
        length: 4410,
        duration: 0.1,
        numberOfChannels: 1,
        getChannelData: () => new Float32Array(4410).fill(0.5)
      };
    }
    async close() {}
  }

  const childDoc = {
    querySelector: (sel) => {
      if (sel.includes('Play video')) return childPlayBtn;
      if (sel === 'audio, video' || sel === 'video') return videoEl;
      return null;
    },
    querySelectorAll: () => []
  };

  const childWin = {
    location: { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' },
    g_fileInfo: { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 15 },
    document: childDoc,
    MediaSource: FakeMediaSource,
    SourceBuffer: FakeSourceBuffer,
    AudioContext: FakeAudioContext
  };
  childWin.window = childWin;

  const iframeEl = {
    src: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a',
    contentWindow: childWin,
    contentDocument: childDoc
  };

  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/xplatplugins.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.AudioContext = FakeAudioContext;
  globalThis.document = {
    querySelector: () => null,
    querySelectorAll: (sel) => sel.includes('iframe') ? [iframeEl] : []
  };

  const rawResult = await eval(fs.readFileSync(0, 'utf8'));
  if (!rawResult) {
    throw new Error('expected JSON payload from child streamembed iframe');
  }
  const parsed = JSON.parse(rawResult);
  if (nativeVideoAppends !== 0) {
    throw new Error('expected video/mp4 SourceBuffer appends to be stripped, got: ' + nativeVideoAppends);
  }
  if (!observedMuted) {
    throw new Error('expected <video> to be muted during Path B capture');
  }
  if (!observedPlaybackRates.includes(16)) {
    throw new Error('expected playbackRate=16, got: ' + JSON.stringify(observedPlaybackRates));
  }
  if (observedCurrentTimes.length === 0 || Math.max(...observedCurrentTimes) < 4.5) {
    throw new Error('expected currentTime stepping across buffered ranges, got: ' + JSON.stringify(observedCurrentTimes));
  }
  if (!decodedBytes) {
    throw new Error('expected AudioContext.decodeAudioData to be invoked');
  }
  if (Buffer.from(decodedBytes).includes(Buffer.from('avc1'))) {
    throw new Error('video/mp4 avc1 data leaked into decoded audio buffer');
  }
  const tfdts = collectTFDTs(decodedBytes.buffer);
  if (JSON.stringify(tfdts) !== JSON.stringify([0, 220500, 441000])) {
    throw new Error('expected deduplicated sorted tfdt [0,220500,441000] (preserving tfdt===0), got: ' + JSON.stringify(tfdts));
  }
  if (!Array.isArray(parsed.pcmChunks) || parsed.pcmChunks.length === 0) {
    throw new Error('expected non-empty pcmChunks');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
		cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
		cmd.Stdin = strings.NewReader(extractRecapAudioJS)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("path-b probe failed: %v\n%s", err, out)
		}
	})

	t.Run("transitive-parentid-scope-and-stream-resource-limits", func(t *testing.T) {
		ownerPage := cdpTargetInfo{
			ID:                   "page-owner",
			Type:                 "page",
			URL:                  "https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a",
			WebSocketDebuggerURL: "ws://127.0.0.1/owner",
		}
		otherPage := cdpTargetInfo{
			ID:                   "page-other",
			Type:                 "page",
			URL:                  "https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-b",
			WebSocketDebuggerURL: "ws://127.0.0.1/other",
		}
		xplatOOPIF := cdpTargetInfo{
			ID:                   "xplat-oopif",
			ParentID:             "page-owner",
			Type:                 "iframe",
			URL:                  "https://contoso-my.sharepoint.com/personal/user/_layouts/15/xplatplugins.aspx",
			WebSocketDebuggerURL: "ws://127.0.0.1/xplat",
		}
		streamEmbedOOPIF := cdpTargetInfo{
			ID:                   "streamembed-oopif",
			ParentID:             "xplat-oopif",
			Type:                 "iframe",
			URL:                  "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx",
			WebSocketDebuggerURL: "ws://127.0.0.1/streamembed",
		}
		unownedXplatOOPIF := cdpTargetInfo{
			ID:                   "other-xplat-oopif",
			ParentID:             "page-other",
			Type:                 "iframe",
			URL:                  "https://contoso-my.sharepoint.com/personal/user/_layouts/15/xplatplugins.aspx",
			WebSocketDebuggerURL: "ws://127.0.0.1/oxplat",
		}
		unownedStreamEmbedOOPIF := cdpTargetInfo{
			ID:                   "other-streamembed-oopif",
			ParentID:             "other-xplat-oopif",
			Type:                 "iframe",
			URL:                  "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx",
			WebSocketDebuggerURL: "ws://127.0.0.1/ostreamembed",
		}
		targets := []cdpTargetInfo{
			ownerPage,
			otherPage,
			xplatOOPIF,
			streamEmbedOOPIF,
			unownedXplatOOPIF,
			unownedStreamEmbedOOPIF,
		}
		expanded := expandOwnedFrames(targets, "page-owner", ownedFrameSet{ids: make(map[string]bool)})
		assert.True(t, isTargetInCaptureScope(ownerPage, "page-owner", expanded))
		assert.True(t, isTargetInCaptureScope(xplatOOPIF, "page-owner", expanded))
		assert.True(t, isTargetInCaptureScope(streamEmbedOOPIF, "page-owner", expanded),
			"nested streamembed.aspx OOPIF whose ParentID is an owned xplatplugins.aspx OOPIF must be included")
		assert.False(t, isTargetInCaptureScope(otherPage, "page-owner", expanded))
		assert.False(t, isTargetInCaptureScope(unownedXplatOOPIF, "page-owner", expanded))
		assert.False(t, isTargetInCaptureScope(unownedStreamEmbedOOPIF, "page-owner", expanded))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, tc := range []struct {
			name       string
			maxBytes   int
			maxDur     int
			ppd        string
			msePayload bool
		}{
			{name: "transcode-duration-cap", maxBytes: 1048576, maxDur: 10, ppd: "200000000"},
			{name: "transcode-source-bytes-cap", maxBytes: 64, maxDur: 120, ppd: "100000000"},
			{name: "mse-source-bytes-cap", maxBytes: 64, maxDur: 120, ppd: "100000000", msePayload: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = ` + strconv.Itoa(tc.maxBytes) + `, maxRecapAudioDurationSec = ` + strconv.Itoa(tc.maxDur) + `, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };

(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(7);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const initPlain = buildAudioInitSegment();
  const segPlain = buildMediaSegment(0, 0x44);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, initPlain));
  const encSeg = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, segPlain));

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this.buffered = { length: 0, start: () => 0, end: () => 0 }; }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  let decodes = 0;
  globalThis.AudioContext = class {
    async decodeAudioData() { decodes++; throw new Error('decodeAudioData must not be called on oversized stream'); }
    async close() {}
  };
  globalThis.crypto = { subtle };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };

  const baseTranscode = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  const initURL = baseTranscode + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=` + tc.ppd + `&driveItemId=recording-a';
  const seg0URL = baseTranscode + '?part=mediasegment&track=audio&quality=audcopy&segmentTime=0&wsd=50000000&ppd=` + tc.ppd + `&driveItemId=recording-a';

  globalThis.fetch = async (url) => {
    const s = String(url);
    if (!s.includes('oneDrive.transcode')) throw new TypeError('Failed to fetch');
    const bytes = s.includes('initsegment') ? encInit : encSeg;
    let sent = false;
    return {
      ok: true,
      status: 200,
      url: s,
      headers: { get: (k) => k.toLowerCase() === 'content-type' ? 'video/mp4' : (k.toLowerCase() === 'content-length' ? String(bytes.byteLength) : null) },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  };

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    muted: false, volume: 1, playbackRate: 1, paused: true,
    play: async () => { videoEl.paused = false; },
    pause: () => { videoEl.paused = true; },
    addEventListener: () => {}, removeEventListener: () => {},
    getAttribute: () => null
  };
  const playBtn = {
    click: () => {
      if (` + strconv.FormatBool(tc.msePayload) + `) {
        const ms = new FakeMediaSource();
        const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
        sb.appendBuffer(initPlain);
        sb.appendBuffer(segPlain);
        return;
      }
      (async () => {
        const r = await globalThis.fetch(initURL);
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
        await globalThis.fetch(seg0URL);
      })().catch(() => {});
    }
  };
  globalThis.document = {
    querySelector: (sel) => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio exceeds maximum supported size/duration')) {
    throw new Error('expected resource limit error (not masked by isDownloadBlocked=true), got: ' + String(failure));
  }
  if (decodes !== 0) {
    throw new Error('decodeAudioData was called ' + decodes + ' times on oversized stream');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
				cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
				cmd.Stdin = strings.NewReader(extractRecapAudioJS)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("resource limit probe %s failed: %v\n%s", tc.name, err, out)
				}
			})
		}
	})

	t.Run("transfer-pending-chunked-pull", func(t *testing.T) {
		var evalCalls atomic.Int32
		var cleanupSeen atomic.Bool
		var serverURL string
		pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/json/list" {
				wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
				require.NoError(t, json.NewEncoder(w).Encode([]cdpTargetInfo{
					{ID: "owner", Type: "page", URL: harnessRecapURL, WebSocketDebuggerURL: wsBase + "/owner"},
					{ID: "frame", ParentID: "owner", Type: "iframe", URL: "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a", WebSocketDebuggerURL: wsBase + "/frame"},
				}))
				return
			}
			conn, _, _, err := ws.UpgradeHTTP(r, w)
			if err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			defer func() { _ = conn.Close() }()
			raw, _, err := wsutil.ReadClientData(conn)
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
			require.NoError(t, json.Unmarshal(raw, &req))
			var result map[string]any
			switch {
			case req.Method == "Page.getFrameTree":
				result = map[string]any{"frameTree": map[string]any{
					"frame": map[string]string{"id": "owner", "url": harnessRecapURL},
				}}
			case r.URL.Path == "/frame":
				n := evalCalls.Add(1)
				var value string
				switch n {
				case 1:
					value = `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-a","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":2}`
				case 2:
					batch, _ := json.Marshal([]string{pcm, pcm})
					value = string(batch)
				default:
					if strings.Contains(req.Params.Expression, "delete window.__goTeamsRecapAudioTransfer") {
						cleanupSeen.Store(true)
					}
					value = ""
				}
				result = map[string]any{"result": map[string]string{"value": value}}
			default:
				result = map[string]any{"result": map[string]string{"value": ""}}
			}
			resp, err := json.Marshal(map[string]any{"id": req.ID, "result": result})
			require.NoError(t, err)
			require.NoError(t, wsutil.WriteServerText(conn, resp))
		}))
		defer srv.Close()
		serverURL = srv.URL

		_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
		require.NoError(t, err)
		port, err := strconv.Atoi(portText)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var out bytes.Buffer
		require.NoError(t, captureRecapAudioFromTargets(ctx, port, "owner", harnessRecapURL, &out))
		got := out.Bytes()
		require.True(t, len(got) >= 2 && got[0] == 0xff && (got[1]&0xe0) == 0xe0, "expected valid MP3 frame sync header")
		assert.True(t, cleanupSeen.Load(), "expected transfer buffer cleanup evaluation")
	})

	t.Run("adversarial-review-invariants-m1-to-m4-and-u1-u2", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// M1: Child iframe with no own-frame identity must not inherit parent's verification or itemId.
		t.Run("M1-no-ancestor-identity-inheritance", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const segA = buildMediaSegment(0, 0x01);
  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this.buffered = { length: 1, start: () => 0, end: () => 1 }; }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/mse-unverified',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/mse-unverified',
    duration: 1, ended: true, muted: true, volume: 0, playbackRate: 16, currentTime: 0,
    play: async () => {}, pause: () => {}, getAttribute: () => null, querySelector: () => null
  };
  const childPlayBtn = {
    click: () => {
      const ms = new childWin.MediaSource();
      const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      sb.appendBuffer(audioInit);
      sb.appendBuffer(segA);
    }
  };
  const childDoc = {
    querySelector: (sel) => sel.includes('Play video') ? childPlayBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };
  const childWin = {
    location: { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx' },
    g_fileInfo: { isDownloadBlocked: true },
    document: childDoc,
    MediaSource: FakeMediaSource,
    SourceBuffer: FakeSourceBuffer,
    AudioContext: class { async decodeAudioData() { throw new Error('must not decode unverified frame'); } async close() {} }
  };
  childWin.window = childWin;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/xplatplugins.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.document = {
    querySelector: () => null,
    querySelectorAll: (sel) => sel.includes('iframe') ? [{ contentWindow: childWin, contentDocument: childDoc }] : []
  };
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('identity unverified')) {
    throw new Error('expected identity unverified error, got: ' + String(failure));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("M1 probe failed: %v\n%s", err, out)
			}
		})

		// M2: Explicit header allowlist (no x-ms-* wildcard), https origin containment, and per-origin header map.
		t.Run("M2-https-origin-and-explicit-header-allowlist", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(3);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));
  const encSeg1 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(220500, 0x22)));
  const replayedHeaders = [];
  let httpTranscodeInspected = false;
  const httpsBase = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  const httpBase = 'http://evil.example/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  function makeResp(url, bytes) {
    let sent = false;
    return {
      ok: true, status: 200, url,
      headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  }
  globalThis.fetch = async (url, init) => {
    const s = String(url);
    if (s.includes('evil.example')) {
      return makeResp(s, encSeg0);
    }
    if (s.includes('segmentTime=50000000') && init && init.headers) {
      replayedHeaders.push(Object.keys(init.headers).sort());
    }
    if (s.includes('initsegment')) return makeResp(s, encInit);
    if (s.includes('segmentTime=0')) return makeResp(s, encSeg0);
    if (s.includes('segmentTime=50000000')) return makeResp(s, encSeg1);
    throw new Error('unexpected fetch: ' + s);
  };
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    muted: false, volume: 1, playbackRate: 1, paused: true,
    play: async () => {}, pause: () => {}, getAttribute: () => null
  };
  const playBtn = {
    click: () => {
      (async () => {
        // Non-loopback http: request even on a fake http window must not populate template/headers.
        await globalThis.fetch(httpBase + '?part=mediasegment&track=audio&wsd=50000000&ppd=100000000', { headers: { 'authorization': 'Bearer leak' } }).catch(() => {});
        const r = await globalThis.fetch(httpsBase + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=100000000&driveItemId=recording-a', {
          headers: {
            'authorization': 'Bearer ok',
            'x-ms-client-request-id': 'req-1',
            'x-ms-evil-secret': 'do-not-capture'
          }
        });
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
        const r0 = await globalThis.fetch(httpsBase + '?part=mediasegment&track=audio&quality=audcopy&segmentTime=0&wsd=50000000&ppd=100000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r0.arrayBuffer());
      })().catch(() => { httpTranscodeInspected = true; });
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!replayedHeaders.length) throw new Error('expected Path A to replay captured allowlisted headers');
  for (const keys of replayedHeaders) {
    if (keys.includes('x-ms-evil-secret')) throw new Error('wildcard x-ms-* header leaked: ' + JSON.stringify(keys));
    if (!keys.includes('authorization') || !keys.includes('x-ms-client-request-id')) {
      throw new Error('missing allowlisted headers: ' + JSON.stringify(keys));
    }
  }
  if (!parsed.pcmChunks || !parsed.pcmChunks.length || httpTranscodeInspected) {
    throw new Error('unexpected result');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("M2 probe failed: %v\n%s", err, out)
			}

			// Verify plain http://non-loopback window cannot capture headers or template and Path A does not fire.
			httpProbeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  globalThis.location = { href: 'http://evil.example/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'http://evil.example' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 1 };
  let seg1Called = false;
  globalThis.fetch = async (url) => {
    if (String(url).includes('segmentTime=50000000')) seg1Called = true;
    return { ok: true, status: 200, url: String(url), headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }, arrayBuffer: async () => new ArrayBuffer(0) };
  };
  class FakeSB extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.buffered = { length: 1, start: () => 0, end: () => 1 }; }
    appendBuffer() {} remove() {} abort() {}
  }
  globalThis.MediaSource = class extends EventTarget {
    addSourceBuffer(mime) { return new FakeSB(mime); }
    removeSourceBuffer() {}
  };
  globalThis.SourceBuffer = FakeSB;
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  const videoEl = { src: 'blob:http://evil.example/1', currentSrc: 'blob:http://evil.example/1', duration: 1, ended: true, play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      globalThis.fetch('http://evil.example/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode?part=mediasegment&track=audio&wsd=50000000&ppd=100000000', {
        headers: { authorization: 'Bearer bad' }
      }).catch(() => {});
      const ms = new globalThis.MediaSource();
      const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      sb.appendBuffer(audioInit);
      sb.appendBuffer(seg0);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (seg1Called) throw new Error('non-loopback http: origin must not trigger Path A transcode fetch');
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected Path B fallback PCM');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmdHTTP := exec.CommandContext(ctx, "node", "-e", httpProbeJS)
			cmdHTTP.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmdHTTP.CombinedOutput(); err != nil {
				t.Fatalf("M2 non-loopback http probe failed: %v\n%s", err, out)
			}
		})

		// M3: Path B 2400-segment cap and Path A segment-count overflow preserve limitError.
		t.Run("M3-2400-segment-cap-and-limit-error-precedence", func(t *testing.T) {
			// Path B: 2401 segments must hit limitError without calling decodeAudioData.
			pathBProbe := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 104857600, maxRecapAudioDurationSec = 7200, maxRecapAudioPCMBytes = 720000000;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg = buildMediaSegment(0, 0x01);
  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this.buffered = { length: 0, start: () => 0, end: () => 0 }; }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }
  let decodes = 0;
  globalThis.AudioContext = class { async decodeAudioData() { decodes++; throw new Error('must not decode'); } async close() {} };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      sb.appendBuffer(audioInit);
      for (let i = 0; i <= 2400; i++) {
        // Build unique tfdt per segment quickly by patching tfdt low u32 in seg copy.
        const copy = new Uint8Array(seg);
        const tfdtOff = copy.length - 16 - 4; // mdat is 16 bytes; tfdt ends right before mdat, last 4 bytes of tfdt are low u32
        copy[tfdtOff] = (i >>> 24) & 0xff;
        copy[tfdtOff + 1] = (i >>> 16) & 0xff;
        copy[tfdtOff + 2] = (i >>> 8) & 0xff;
        copy[tfdtOff + 3] = i & 0xff;
        sb.appendBuffer(copy);
      }
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio exceeds maximum supported size/duration')) {
    throw new Error('expected limitError on >2400 Path B segments, got: ' + String(failure));
  }
  if (decodes !== 0) throw new Error('decodeAudioData called');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmdB := exec.CommandContext(ctx, "node", "-e", pathBProbe)
			cmdB.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmdB.CombinedOutput(); err != nil {
				t.Fatalf("M3 Path B 2400-segment cap probe failed: %v\n%s", err, out)
			}

			// Path A: totalSegments > 2400 must reject with limitError (not fall back / mask with isDownloadBlocked=true).
			pathAProbe := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 104857600, maxRecapAudioDurationSec = 7200, maxRecapAudioPCMBytes = 720000000;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(5);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  globalThis.fetch = async (url) => ({
    ok: true, status: 200, url: String(url), headers: { get: () => null },
    body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) },
    arrayBuffer: async () => encInit.slice().buffer
  });
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      (async () => {
        // wsd=1000000 (0.1s), ppd=2500000000 (250s -> 2500 segments > 2400, within duration/PCM caps).
        const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=1000000&ppd=2500000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
      })().catch(() => {});
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio exceeds maximum supported size/duration')) {
    throw new Error('expected limitError on Path A >2400 segments, got: ' + String(failure));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmdA := exec.CommandContext(ctx, "node", "-e", pathAProbe)
			cmdA.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmdA.CombinedOutput(); err != nil {
				t.Fatalf("M3 Path A 2400-segment cap probe failed: %v\n%s", err, out)
			}
		})

		// M4: Path B re-checks updated media.duration / mseMaxBufferedEnd before decodeAudioData.
		t.Run("M4-pre-decode-duration-and-pcm-cap", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 10, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const segA = buildMediaSegment(0, 0x01);
  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this._end = 0; this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null }; this.buffered._sb = this; }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }
  let decodes = 0;
  globalThis.AudioContext = class { async decodeAudioData() { decodes++; throw new Error('must not decode oversized buffered duration'); } async close() {} };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  // duration starts as NaN at entry, then becomes 25 (> maxRecapAudioDurationSec=10) when MSE appends.
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', duration: NaN, ended: false, play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      sb._end = 25;
      videoEl.duration = 25;
      videoEl.ended = true;
      sb.appendBuffer(audioInit);
      sb.appendBuffer(segA);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio exceeds maximum supported size/duration')) {
    throw new Error('expected limitError before decodeAudioData, got: ' + String(failure));
  }
  if (decodes !== 0) throw new Error('decodeAudioData called ' + decodes + ' times');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("M4 probe failed: %v\n%s", err, out)
			}
		})

		// U1: Path A non-fatal failure aborts and settles sibling workers before Path B resumes.
		t.Run("U1-path-a-fallback-aborts-sibling-workers-before-path-b", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(4);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  const seg1 = buildMediaSegment(220500, 0x02);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, audioInit));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, seg0));

  let siblingAbortedBeforeResolve = false;
  let siblingSettledAfterPathB = false;
  let pathBResumed = false;
  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this._end = 0; this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null }; this.buffered._sb = this; }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  let audioSB = null;
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 10, ended: false, paused: true, muted: false, volume: 1, playbackRate: 1, currentTime: 0,
    play: async () => {
      if (videoEl.paused === false && audioSB && !pathBResumed) {
        pathBResumed = true;
        setTimeout(() => {
          audioSB._end = 10;
          videoEl.ended = true;
          audioSB.appendBuffer(seg1);
        }, 20);
      }
      videoEl.paused = false;
    },
    pause: () => {},
    getAttribute: () => null
  };

  globalThis.fetch = async (url, init) => {
    const s = String(url);
    if (s.includes('initsegment')) {
      return { ok: true, status: 200, url: s, headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }, arrayBuffer: async () => encInit.slice().buffer };
    }
    if (s.includes('segmentTime=0')) {
      return { ok: true, status: 200, url: s, headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }, arrayBuffer: async () => encSeg0.slice().buffer };
    }
    if (s.includes('segmentTime=50000000')) {
      // Worker 1 fails quickly with 500 (non-fatal fallback trigger).
      return { ok: false, status: 500, url: s, headers: { get: () => null } };
    }
    if (s.includes('segmentTime=100000000')) {
      // Sibling worker 2 hangs until aborted; if not aborted before Path B resumes, mark failure!
      return new Promise((resolve, reject) => {
        const sig = init && init.signal;
        if (sig && sig.aborted) {
          siblingAbortedBeforeResolve = true;
          reject(new Error('aborted'));
          return;
        }
        if (sig) {
          sig.addEventListener('abort', () => {
            if (pathBResumed) siblingSettledAfterPathB = true;
            siblingAbortedBeforeResolve = true;
            reject(new Error('aborted'));
          }, { once: true });
        }
        setTimeout(() => {
          if (pathBResumed) siblingSettledAfterPathB = true;
          resolve({ ok: true, status: 200, url: s, headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) } });
        }, 80);
      });
    }
    throw new Error('unexpected fetch: ' + s);
  };
  globalThis.crypto = { subtle };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 10 };
  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
      (async () => {
        const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=150000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
      })().catch(() => {});
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected Path B fallback PCM');
  if (!siblingAbortedBeforeResolve) throw new Error('expected sibling Path A worker to be aborted on fallback');
  if (siblingSettledAfterPathB) throw new Error('sibling Path A worker settled after Path B resumed');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U1 probe failed: %v\n%s", err, out)
			}
		})

		// U2: Path B does not exit early on a 3-second append gap when targetDur > 0 and buffer is incomplete.
		t.Run("U2-path-b-does-not-truncate-on-3s-gap-when-duration-known", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const segA = buildMediaSegment(0, 0x01);
  const segB = buildMediaSegment(220500, 0x02);
  let fakeNow = 1000000;
  let polls = 0;
  Date.now = () => fakeNow;

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this._end = 0; this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null }; this.buffered._sb = this; }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }
  let audioSB = null;
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 20, ended: false, paused: true, muted: false, volume: 1, playbackRate: 1, currentTime: 0,
    play: async () => {}, pause: () => {}, getAttribute: () => null
  };
  let decodedTFDTs = [];
  globalThis.AudioContext = class {
    async decodeAudioData(ab) {
      decodedTFDTs = collectTFDTs(ab);
      return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
    }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 20 };

  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    return origSetTimeout(() => {
      polls++;
      if (polls === 2) {
        // Simulate a 4-second network stall while mseMaxBufferedEnd (5) < targetDur - 0.5 (19.5).
        fakeNow += 4000;
      } else if (polls === 5 && audioSB) {
        fakeNow += 200;
        audioSB._end = 20;
        videoEl.ended = true;
        audioSB.appendBuffer(segB);
      }
      fn(...args);
    }, Math.min(ms || 0, 5));
  };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB._end = 5;
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(segA);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (JSON.stringify(decodedTFDTs) !== JSON.stringify([0, 220500])) {
    throw new Error('Path B truncated audio on 3s stall; decoded tfdt=' + JSON.stringify(decodedTFDTs));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U2 probe failed: %v\n%s", err, out)
			}
		})

		// M1-R2: Unvalidated info.transcodeUrl (non-HTTPS or cross-origin) must be discarded and never fetched by Path A.
		t.Run("M1-R2-info-transcodeUrl-origin-and-protocol-validation", func(t *testing.T) {
			for _, badTranscodeURL := range []string{
				"https://evil.example/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode?driveItemId=recording-a",
				"http://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode?driveItemId=recording-a",
			} {
				probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(6);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));

  const evilFetched = [];
  const sameOriginBase = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  function makeResp(url, bytes) {
    let sent = false;
    return {
      ok: true, status: 200, url, headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  }
  globalThis.fetch = async (url) => {
    const s = String(url);
    if (s.includes('evil.example') || s.startsWith('http://')) {
      evilFetched.push(s);
      return makeResp(s, encSeg0);
    }
    if (s.includes('part=initsegment')) return makeResp(s, encInit);
    if (s.includes('part=mediasegment')) return makeResp(s, encSeg0);
    throw new Error('unexpected fetch: ' + s);
  };
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = {
    VroomItemId: 'recording-a',
    isDownloadBlocked: true,
    transcodeUrl: ` + strconv.Quote(badTranscodeURL) + `
  };
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      (async () => {
        // Same-origin audio transcode request supplies wsd/ppd, and WebCrypto decrypts a pre-fetched/cached init buffer.
        // If unvalidated info.transcodeUrl was preserved, Path A will fetch evil.example/http://!
        await globalThis.fetch(sameOriginBase + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=50000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, encInit.slice().buffer);
      })().catch(() => {});
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (evilFetched.length > 0) {
    throw new Error('Path A fetched disallowed info.transcodeUrl: ' + JSON.stringify(evilFetched));
  }
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
				cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
				cmd.Stdin = strings.NewReader(extractRecapAudioJS)
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("M1-R2 probe (%s) failed: %v\n%s", badTranscodeURL, err, out)
				}
			}
		})

		// U1-R2: Deadline expiry aborts pending Path A and settles workers before cleanup without post-cleanup m.play().
		t.Run("U1-R2-deadline-aborts-path-a-without-post-cleanup-play", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(2);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));

  let fakeNow = 1000000;
  Date.now = () => fakeNow;
  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    return origSetTimeout(() => fn(...args), Math.min(ms || 0, 5));
  };

  let workerAborted = false;
  let pauseCalled = false;
  let playCalledAfterPause = false;
  let deadlineTriggered = false;
  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  const origFetch = async (url, init) => {
    const s = String(url);
    if (s.includes('initsegment')) {
      return { ok: true, status: 200, url: s, headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }, arrayBuffer: async () => encInit.slice().buffer };
    }
    if (s.includes('segmentTime=0') && !deadlineTriggered) {
      deadlineTriggered = true;
      origSetTimeout(() => { fakeNow += 125000; }, 10);
    }
    return new Promise((resolve, reject) => {
      const sig = init && init.signal;
      if (sig && sig.aborted) {
        workerAborted = true;
        reject(new Error('aborted'));
        return;
      }
      if (sig) {
        sig.addEventListener('abort', () => {
          workerAborted = true;
          reject(new Error('aborted'));
        }, { once: true });
      }
    });
  };
  globalThis.fetch = origFetch;
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    paused: true, muted: false, volume: 1, playbackRate: 1,
    play: async () => {
      if (pauseCalled) playCalledAfterPause = true;
      videoEl.paused = false;
    },
    pause: () => {
      pauseCalled = true;
      videoEl.paused = true;
      videoEl.playbackRate = 1;
    },
    getAttribute: () => null
  };
  const playBtn = {
    click: () => {
      (async () => {
        const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=100000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
      })().catch(() => {});
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  await new Promise(r => origSetTimeout(r, 25));
  if (!failure || !String(failure.message || failure).includes('deadline')) {
    throw new Error('expected deadline failure, got: ' + String(failure));
  }
  if (!workerAborted) throw new Error('expected Path A worker to be aborted on deadline');
  if (!pauseCalled || !videoEl.paused || videoEl.playbackRate !== 1) throw new Error('expected media element to be paused/restored by cleanupHooks');
  if (playCalledAfterPause) throw new Error('maybeStartPathA catch handler called m.play() after cleanupHooks');
  if (globalThis.fetch !== origFetch) throw new Error('expected fetch hook to be restored');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U1-R2 probe failed: %v\n%s", err, out)
			}
		})

		// U2-R2: Pre-hook native video/mp4 SourceBuffer without .mime must not leak into mseSegments or audioSBRef.
		t.Run("U2-R2-pre-hook-native-video-sourcebuffer-without-mime-ignored", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const videoInit = buildVideoInitSegment();
  const audioSeg0 = buildMediaSegment(0, 0x11);
  const videoMediaNoInit = buildMediaSegment(0, 0x99);
  const videoMediaAfterInit = buildMediaSegment(220500, 0x88);

  class NativeNoMimeSourceBuffer extends EventTarget {
    constructor() {
      super();
      // Intentionally NO .mime property (matches real browser SourceBuffer).
      this.updating = false;
      this._end = 0;
      this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null };
      this.buffered._sb = this;
    }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class NativeMediaSource extends EventTarget {
    addSourceBuffer() { return new NativeNoMimeSourceBuffer(); }
    removeSourceBuffer() {}
  }

  // Created BEFORE installWindowHooks runs.
  const preHookVideoSBNoInit = new NativeNoMimeSourceBuffer();
  const preHookVideoSBWithInit = new NativeNoMimeSourceBuffer();

  let decodedBytes = null;
  globalThis.AudioContext = class {
    async decodeAudioData(ab) {
      decodedBytes = new Uint8Array(ab);
      return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
    }
    async close() {}
  };
  globalThis.MediaSource = NativeMediaSource;
  globalThis.SourceBuffer = NativeNoMimeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 10 };

  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', duration: 10, ended: false, play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      // Pre-hook video buffer without init (init was pre-hook) overwrites tfdt=0 if unguarded!
      preHookVideoSBNoInit._end = 99;
      preHookVideoSBNoInit.appendBuffer(videoMediaNoInit);

      // Pre-hook video buffer with post-hook videoInit.
      preHookVideoSBWithInit._end = 99;
      preHookVideoSBWithInit.appendBuffer(videoInit);
      preHookVideoSBWithInit.appendBuffer(videoMediaAfterInit);

      // Actual audio SourceBuffer.
      const ms = new NativeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB._end = 10;
      videoEl.ended = true;
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(audioSeg0);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (!decodedBytes) throw new Error('expected decodeAudioData call');
  const buf = Buffer.from(decodedBytes);
  if (buf.includes(Buffer.from('avc1')) || buf.includes(Buffer.from([0x99, 0x99, 0x99, 0x99])) || buf.includes(Buffer.from([0x88, 0x88, 0x88, 0x88]))) {
    throw new Error('pre-hook video SourceBuffer media leaked into audio mseSegments');
  }
  const tfdts = collectTFDTs(decodedBytes.buffer);
  if (JSON.stringify(tfdts) !== JSON.stringify([0])) {
    throw new Error('expected only audio tfdt [0], got: ' + JSON.stringify(tfdts));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U2-R2 probe failed: %v\n%s", err, out)
			}
		})

		// U3-R2: Path B rejects truncated audio when deadlineAt expires with mseMaxBufferedEnd < targetDur - 0.5 and !m.ended.
		t.Run("U3-R2-path-b-rejects-truncated-stream-at-deadline", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const segA = buildMediaSegment(0, 0x01);
  let fakeNow = 1000000;
  let polls = 0;
  Date.now = () => fakeNow;

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) { super(); this.mime = mime; this.updating = false; this._end = 0; this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null }; this.buffered._sb = this; }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 20, ended: false, paused: true, muted: false, volume: 1, playbackRate: 1, currentTime: 0,
    play: async () => {}, pause: () => {}, getAttribute: () => null
  };
  let decodes = 0;
  globalThis.AudioContext = class {
    async decodeAudioData() { decodes++; throw new Error('must not decode truncated Path B stream'); }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 20 };

  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    return origSetTimeout(() => {
      polls++;
      if (polls === 3) {
        // Jump past 120s deadline while buffered end is only 5s (< 20 - 0.5) and !ended.
        fakeNow += 125000;
      }
      fn(...args);
    }, Math.min(ms || 0, 5));
  };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB._end = 5;
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(segA);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio MSE stream incomplete before deadline')) {
    throw new Error('expected incomplete MSE stream error, got: ' + String(failure));
  }
  if (decodes !== 0) throw new Error('decodeAudioData called ' + decodes + ' times on truncated stream');
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U3-R2 probe failed: %v\n%s", err, out)
			}
		})
	})

	t.Run("copilot-pr76-review-fixes", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		t.Run("path-b-rejects-gapped-buffered-ranges-even-when-ended", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  const seg2 = buildMediaSegment(441000, 0x02);
  let fakeNow = 1000000;
  let polls = 0;
  Date.now = () => fakeNow;

  const ranges = [[0, 5], [10, 15]];
  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this.buffered = {
        get length() { return ranges.length; },
        start: (i) => ranges[i][0],
        end: (i) => ranges[i][1]
      };
    }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  let _time = 0;
  const observedCurrentTimes = [];
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 15,
    ended: true,
    paused: true,
    muted: false,
    volume: 1,
    playbackRate: 1,
    get currentTime() { return _time; },
    set currentTime(v) { _time = Number(v); observedCurrentTimes.push(_time); },
    play: async () => {},
    pause: () => {},
    getAttribute: () => null
  };

  let decodes = 0;
  globalThis.AudioContext = class {
    async decodeAudioData() { decodes++; throw new Error('must not decode gapped MSE stream'); }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 15 };

  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    return origSetTimeout(() => {
      polls++;
      if (polls === 3) {
        fakeNow += 125000;
      }
      fn(...args);
    }, Math.min(ms || 0, 5));
  };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
      audioSB.appendBuffer(seg2);
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio MSE stream incomplete before deadline')) {
    throw new Error('expected incomplete MSE stream error on gapped ranges, got: ' + String(failure));
  }
  if (decodes !== 0) {
    throw new Error('decodeAudioData called ' + decodes + ' times on gapped stream');
  }
  if (observedCurrentTimes.some(t => t > 5)) {
    throw new Error('currentTime jumped across unbuffered gap: ' + JSON.stringify(observedCurrentTimes));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("gapped buffered ranges probe failed: %v\n%s", err, out)
			}
		})

		t.Run("transfer-pending-empty-batch-mid-transfer-returns-error", func(t *testing.T) {
			var evalCalls atomic.Int32
			var cleanupSeen atomic.Bool
			var serverURL string
			pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/json/list" {
					wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
					require.NoError(t, json.NewEncoder(w).Encode([]cdpTargetInfo{
						{ID: "owner", Type: "page", URL: harnessRecapURL, WebSocketDebuggerURL: wsBase + "/owner"},
						{ID: "frame", ParentID: "owner", Type: "iframe", URL: "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a", WebSocketDebuggerURL: wsBase + "/frame"},
					}))
					return
				}
				conn, _, _, err := ws.UpgradeHTTP(r, w)
				if err != nil {
					t.Errorf("upgrade: %v", err)
					return
				}
				defer func() { _ = conn.Close() }()
				raw, _, err := wsutil.ReadClientData(conn)
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
				require.NoError(t, json.Unmarshal(raw, &req))
				var result map[string]any
				switch {
				case req.Method == "Page.getFrameTree":
					result = map[string]any{"frameTree": map[string]any{
						"frame": map[string]string{"id": "owner", "url": harnessRecapURL},
					}}
				case r.URL.Path == "/frame":
					if len(req.Params.Expression) < 200 && strings.Contains(req.Params.Expression, "delete window.__goTeamsRecapAudioTransfer") {
						cleanupSeen.Store(true)
						result = map[string]any{"result": map[string]string{"value": ""}}
						break
					}
					n := evalCalls.Add(1)
					var value string
					switch n {
					case 1:
						value = `{"pageUrl":"` + harnessRecapURL + `","itemId":"recording-a","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":3}`
					case 2:
						batch, _ := json.Marshal([]string{pcm})
						value = string(batch)
					default:
						value = "[]"
					}
					result = map[string]any{"result": map[string]string{"value": value}}
				default:
					result = map[string]any{"result": map[string]string{"value": ""}}
				}
				resp, err := json.Marshal(map[string]any{"id": req.ID, "result": result})
				require.NoError(t, err)
				require.NoError(t, wsutil.WriteServerText(conn, resp))
			}))
			defer srv.Close()
			serverURL = srv.URL

			_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
			require.NoError(t, err)
			port, err := strconv.Atoi(portText)
			require.NoError(t, err)

			tCtx, tCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer tCancel()
			var out bytes.Buffer
			err = captureRecapAudioFromTargets(tCtx, port, "owner", harnessRecapURL, &out)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "incomplete recap audio chunk transfer (1 of 3 chunks)")
			assert.True(t, cleanupSeen.Load(), "expected transfer buffer cleanup evaluation on incomplete transfer")
		})

		t.Run("path-b-cross-realm-arraybuffer-from-child-iframe", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const vm = require('vm');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const iframeRealm = vm.createContext({});
  const thirdRealm = vm.createContext({});
  function toCrossRealmAB(u8, realm) {
    const ab = vm.runInNewContext('new ArrayBuffer(' + u8.byteLength + ')', realm);
    new Uint8Array(ab).set(u8);
    return ab;
  }
  function toCrossRealmView(u8, realm) {
    const ab = toCrossRealmAB(u8, realm);
    return vm.runInNewContext('(ab) => new Uint8Array(ab)', realm)(ab);
  }

  const audioInitAB = toCrossRealmAB(buildAudioInitSegment(), iframeRealm);
  const segAAB = toCrossRealmAB(buildMediaSegment(0, 0x01), thirdRealm);
  const segBView = toCrossRealmView(buildMediaSegment(220500, 0x02), thirdRealm);
  if (audioInitAB instanceof ArrayBuffer) {
    throw new Error('test setup error: expected cross-realm ArrayBuffer not instanceof host ArrayBuffer');
  }

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this._end = 0;
      this.buffered = {
        get length() { return this._sb._end > 0 ? 1 : 0; },
        start: () => 0,
        end: () => this._end,
        _sb: null
      };
      this.buffered._sb = this;
    }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/mse-cross-realm',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/mse-cross-realm',
    duration: 10,
    ended: false,
    paused: true,
    muted: true,
    volume: 0,
    playbackRate: 16,
    currentTime: 0,
    play: async () => {},
    pause: () => {},
    getAttribute: () => null,
    querySelector: () => null
  };

  let decodedBytes = null;
  class FakeAudioContext {
    async decodeAudioData(ab) {
      decodedBytes = new Uint8Array(ab);
      return {
        sampleRate: 44100,
        length: 4410,
        duration: 0.1,
        numberOfChannels: 1,
        getChannelData: () => new Float32Array(4410).fill(0.5)
      };
    }
    async close() {}
  }

  const childPlayBtn = {
    click: () => {
      const ms = new childWin.MediaSource();
      const sb = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      sb._end = 10;
      videoEl.ended = true;
      sb.appendBuffer(audioInitAB);
      sb.appendBuffer(segAAB);
      sb.appendBuffer(segBView);
    }
  };
  const childDoc = {
    querySelector: sel => sel.includes('Play video') ? childPlayBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };
  const childWin = {
    ArrayBuffer: vm.runInNewContext('ArrayBuffer', iframeRealm),
    location: { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' },
    g_fileInfo: { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 10 },
    document: childDoc,
    MediaSource: FakeMediaSource,
    SourceBuffer: FakeSourceBuffer,
    AudioContext: FakeAudioContext
  };
  childWin.window = childWin;

  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/xplatplugins.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.AudioContext = FakeAudioContext;
  globalThis.document = {
    querySelector: () => null,
    querySelectorAll: sel => sel.includes('iframe') ? [{
      src: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a',
      contentWindow: childWin,
      contentDocument: childDoc
    }] : []
  };

  const rawResult = await eval(fs.readFileSync(0, 'utf8'));
  if (!rawResult) throw new Error('expected JSON payload from cross-realm iframe');
  const parsed = JSON.parse(rawResult);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected non-empty pcmChunks');
  if (!decodedBytes) throw new Error('expected decodeAudioData call');
  const tfdts = collectTFDTs(decodedBytes.buffer);
  if (JSON.stringify(tfdts) !== JSON.stringify([0, 220500])) {
    throw new Error('expected cross-realm tfdt [0,220500], got: ' + JSON.stringify(tfdts));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("cross-realm ArrayBuffer probe failed: %v\n%s", err, out)
			}
		})

		t.Run("path-b-sb-remove-not-repeated-after-eviction", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  const seg1 = buildMediaSegment(220500, 0x02);
  const removeCalls = [];

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this._start = 0;
      this._end = 0;
      const self = this;
      this.buffered = {
        get length() { return self._end > self._start ? 1 : 0; },
        start: () => self._start,
        end: () => self._end
      };
    }
    appendBuffer() {}
    remove(start, end) {
      removeCalls.push([start, end]);
      if (removeCalls.length > 5) {
        throw new Error('sb.remove invoked repeatedly: ' + JSON.stringify(removeCalls));
      }
      this.updating = true;
      this._start = end;
      setTimeout(() => {
        this.updating = false;
        this.dispatchEvent(new Event('updateend'));
      }, 2);
    }
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  let audioSB = null;
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 40,
    ended: false,
    paused: true,
    muted: false,
    volume: 1,
    playbackRate: 1,
    currentTime: 0,
    play: async () => {},
    pause: () => {},
    getAttribute: () => null
  };

  globalThis.AudioContext = class {
    async decodeAudioData() {
      return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
    }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 40 };

  let removeCallsBeforeAdvance = -1;
  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB._end = 35;
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
      // Wait across multiple polling ticks and updateend events while _end stays 35 (cutoff = 5).
      setTimeout(() => {
        removeCallsBeforeAdvance = removeCalls.length;
        audioSB._end = 40;
        videoEl.ended = true;
        audioSB.appendBuffer(seg1);
      }, 90);
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  const rawResult = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(rawResult);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (removeCallsBeforeAdvance !== 1) {
    throw new Error('expected exactly 1 sb.remove call while buffered end=35, got ' + removeCallsBeforeAdvance + ' calls=' + JSON.stringify(removeCalls));
  }
  if (JSON.stringify(removeCalls) !== JSON.stringify([[0, 5], [5, 10]])) {
    throw new Error('expected bounded sb.remove calls [[0,5],[5,10]], got: ' + JSON.stringify(removeCalls));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("sb.remove bounded eviction probe failed: %v\n%s", err, out)
			}
		})
	})

	t.Run("round4-review-fixes-js", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// U2-R3: Path B rejects unknown/zero duration before allocating combined or calling decodeAudioData.
		t.Run("U2-R3-path-b-rejects-unknown-duration-before-decode", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this.buffered = { length: 0, start: () => 0, end: () => 0 };
    }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 0,
    ended: true,
    paused: true,
    muted: false,
    volume: 1,
    playbackRate: 1,
    currentTime: 0,
    play: async () => {},
    pause: () => {},
    getAttribute: () => null
  };

  let decodes = 0;
  globalThis.AudioContext = class {
    async decodeAudioData() { decodes++; throw new Error('must not decode stream with unverified duration'); }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio MSE stream duration unverified')) {
    throw new Error('expected unverified MSE stream duration error, got: ' + String(failure));
  }
  if (decodes !== 0) {
    throw new Error('decodeAudioData called ' + decodes + ' times when duration was unverified');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U2-R3 probe failed: %v\n%s", err, out)
			}
		})

		// U1-R4: fetchBytesWithCap retry backoff aborts immediately on parentSignal abort.
		t.Run("U1-R4-path-a-retry-backoff-aborts-immediately", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(7);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, audioInit));

  let trackedBackoffTid = null;
  let backoffScheduled = false;
  let backoffCleared = false;
  let backoffFired = false;
  const origSetTimeout = globalThis.setTimeout;
  const origClearTimeout = globalThis.clearTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    if (ms === 250 && trackedBackoffTid === null) {
      backoffScheduled = true;
      const tid = origSetTimeout(() => {
        backoffFired = true;
        fn(...args);
      }, ms, ...args);
      trackedBackoffTid = tid;
      return tid;
    }
    return origSetTimeout(fn, ms, ...args);
  };
  globalThis.clearTimeout = (tid) => {
    if (tid && tid === trackedBackoffTid && !backoffFired) {
      backoffCleared = true;
    }
    return origClearTimeout(tid);
  };

  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this._end = 0;
      this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null };
      this.buffered._sb = this;
    }
    appendBuffer() {} remove() {} abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  let audioSB = null;
  let pathBResumed = false;
  let pathAAbortObservedBeforeBackoffTimer = false;
  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: 1,
    ended: false,
    paused: true,
    muted: false,
    volume: 1,
    playbackRate: 1,
    currentTime: 0,
    play: async () => {
      if (videoEl.paused === false && audioSB && !pathBResumed) {
        pathBResumed = true;
        pathAAbortObservedBeforeBackoffTimer = backoffCleared;
        audioSB._end = 1;
        videoEl.ended = true;
        audioSB.appendBuffer(seg0);
      }
      videoEl.paused = false;
    },
    pause: () => {},
    getAttribute: () => null
  };

  globalThis.fetch = async (url) => {
    const s = String(url);
    if (s.includes('initsegment')) {
      return { ok: true, status: 200, url: s, headers: { get: () => null }, body: { getReader: () => ({ read: async () => ({ done: true }), cancel: async () => {} }) }, arrayBuffer: async () => encInit.slice().buffer };
    }
    if (s.includes('segmentTime=0')) {
      // Retryable 500 puts worker 0 into 250ms retry backoff.
      return { ok: false, status: 500, url: s, headers: { get: () => null } };
    }
    if (s.includes('segmentTime=50000000')) {
      // Wait 20ms so worker 0 is inside the 250ms backoff, then fail with non-retryable 403.
      await new Promise(r => origSetTimeout(r, 20));
      return { ok: false, status: 403, url: s, headers: { get: () => null } };
    }
    throw new Error('unexpected fetch: ' + s);
  };
  globalThis.crypto = { subtle };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 1 };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
      (async () => {
        const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=100000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
      })().catch(() => {});
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  const started = Date.now();
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const elapsedMs = Date.now() - started;
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected Path B fallback PCM');
  if (!backoffScheduled) throw new Error('test setup error: expected 250ms retry backoff to be scheduled');
  if (!backoffCleared || !pathAAbortObservedBeforeBackoffTimer) {
    throw new Error('retry backoff timer was not cleared immediately on parentSignal abort (backoffCleared=' + backoffCleared + ', beforeResume=' + pathAAbortObservedBeforeBackoffTimer + ', elapsedMs=' + elapsedMs + ')');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("U1-R4 probe failed: %v\n%s", err, out)
			}
		})
	})

	t.Run("copilot-pr76-round5-fixes", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// 1. PRRT_kwDOTStvas64LgPU: normalizeHeaders preserves [['Authorization', 'Bearer ...']] tuple arrays.
		t.Run("normalize-headers-preserves-tuple-arrays", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(8);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));
  const encSeg1 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(220500, 0x22)));

  const replayedAuthHeaders = [];
  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  function makeResp(url, bytes) {
    let sent = false;
    return {
      ok: true, status: 200, url, headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  }
  globalThis.fetch = async (url, init) => {
    const s = String(url);
    if (s.includes('segmentTime=50000000')) {
      replayedAuthHeaders.push((init && init.headers && init.headers.authorization) || '');
    }
    if (s.includes('initsegment')) return makeResp(s, encInit);
    if (s.includes('segmentTime=0')) return makeResp(s, encSeg0);
    if (s.includes('segmentTime=50000000')) return makeResp(s, encSeg1);
    throw new Error('unexpected fetch: ' + s);
  };
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      (async () => {
        const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=100000000&driveItemId=recording-a', {
          headers: [['Authorization', 'Bearer tuple-secret-token']]
        });
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
        const r0 = await globalThis.fetch(base + '?part=mediasegment&track=audio&quality=audcopy&segmentTime=0&wsd=50000000&ppd=100000000&driveItemId=recording-a');
        await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r0.arrayBuffer());
      })().catch(() => {});
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };
  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (replayedAuthHeaders.length === 0 || replayedAuthHeaders[0] !== 'Bearer tuple-secret-token') {
    throw new Error('expected tuple header Authorization to be preserved, got: ' + JSON.stringify(replayedAuthHeaders));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("normalizeHeaders tuple array probe failed: %v\n%s", err, out)
			}
		})

		// 2. PRRT_kwDOTStvas64LgP4: XHR suppressed video segment fires property handlers once when dispatchEvent exists.
		t.Run("xhr-suppressed-video-fires-handlers-once-with-dispatchevent", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(9);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));

  let nativeXHRSends = 0;
  let readyStateCalls = 0;
  let loadCalls = 0;
  let loadendCalls = 0;
  let loadListenerCalls = 0;

  class FakeXMLHttpRequest extends EventTarget {
    open(method, url) { this._url = url; }
    setRequestHeader() {}
    send() { nativeXHRSends++; }
    dispatchEvent(ev) {
      const ok = super.dispatchEvent(ev);
      const handler = this['on' + ev.type];
      if (typeof handler === 'function') handler.call(this, ev);
      return ok;
    }
  }
  globalThis.XMLHttpRequest = FakeXMLHttpRequest;

  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  globalThis.fetch = async (url) => {
    const s = String(url);
    if (!s.includes('oneDrive.transcode')) throw new TypeError('Failed to fetch');
    const bytes = s.includes('initsegment') ? encInit : encSeg0;
    let sent = false;
    return {
      ok: true, status: 200, url: s, headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  };
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = {
    VroomItemId: 'recording-a',
    isDownloadBlocked: true,
    transcodeUrl: base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=50000000&driveItemId=recording-a'
  };
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };
  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      const xhr = new globalThis.XMLHttpRequest();
      xhr.onreadystatechange = () => { readyStateCalls++; };
      xhr.onload = () => { loadCalls++; };
      xhr.onloadend = () => { loadendCalls++; };
      xhr.addEventListener('load', () => { loadListenerCalls++; });
      xhr.open('GET', base + '?part=mediasegment&track=video&quality=high&segmentTime=0&wsd=50000000&ppd=50000000&driveItemId=recording-a');
      xhr.send();
      setTimeout(() => {
        (async () => {
          const r = await globalThis.fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=50000000&driveItemId=recording-a');
          await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
        })().catch(() => {});
      }, 20);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (nativeXHRSends !== 0) throw new Error('expected video XHR send to be suppressed, got nativeXHRSends=' + nativeXHRSends);
  if (readyStateCalls !== 1 || loadCalls !== 1 || loadendCalls !== 1 || loadListenerCalls !== 1) {
    throw new Error('expected each XHR event handler to fire once, got readyState=' + readyStateCalls + ' load=' + loadCalls + ' loadend=' + loadendCalls + ' listener=' + loadListenerCalls);
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("XHR suppressed video handler count probe failed: %v\n%s", err, out)
			}
		})

		// 3. PRRT_kwDOTStvas64LgQe: Pre-hook native SourceBuffer without .mime suppresses both video init & media and emits updateend.
		t.Run("pre-hook-native-video-sb-suppresses-init-and-media-with-updateend", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const videoInit = buildVideoInitSegment();
  const audioSeg0 = buildMediaSegment(0, 0x11);
  const videoMedia = buildMediaSegment(220500, 0x88);

  let preHookOrigAppendCalls = 0;
  let preHookUpdateEndEvents = 0;
  let preHookUpdatingMutated = false;

  class NativeNoMimeSourceBuffer extends EventTarget {
    constructor() {
      super();
      let _updating = false;
      Object.defineProperty(this, 'updating', {
        get: () => _updating,
        set: () => { preHookUpdatingMutated = true; }
      });
      this._end = 0;
      this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null };
      this.buffered._sb = this;
    }
    appendBuffer() {
      if (this === preHookVideoSB) preHookOrigAppendCalls++;
    }
    remove() {}
    abort() {}
  }
  class NativeMediaSource extends EventTarget {
    addSourceBuffer() { return new NativeNoMimeSourceBuffer(); }
    removeSourceBuffer() {}
  }

  const preHookVideoSB = new NativeNoMimeSourceBuffer();
  preHookVideoSB.addEventListener('updateend', () => { preHookUpdateEndEvents++; });

  globalThis.AudioContext = class {
    async decodeAudioData() {
      return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
    }
    async close() {}
  };
  globalThis.MediaSource = NativeMediaSource;
  globalThis.SourceBuffer = NativeNoMimeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 10 };

  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', duration: 10, ended: false, play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      preHookVideoSB.appendBuffer(videoInit);
      preHookVideoSB.appendBuffer(videoMedia);
      const ms = new NativeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      setTimeout(() => {
        audioSB._end = 10;
        videoEl.ended = true;
        audioSB.appendBuffer(audioInit);
        audioSB.appendBuffer(audioSeg0);
      }, 20);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');
  if (preHookOrigAppendCalls !== 0) {
    throw new Error('expected native origAppend not to be called for video init/media, got calls=' + preHookOrigAppendCalls);
  }
  if (preHookUpdateEndEvents !== 2) {
    throw new Error('expected updateend emitted for both video init and media (2), got ' + preHookUpdateEndEvents);
  }
  if (preHookUpdatingMutated) {
    throw new Error('sb.updating must not be mutated on native SourceBuffer instances');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("pre-hook native video SB suppression probe failed: %v\n%s", err, out)
			}
		})

		// 4. PRRT_kwDOTStvas64LgOi: Path B rejects unknown-duration stream when !media.ended even if mseMaxBufferedEnd > 0 and 3s elapse.
		t.Run("path-b-rejects-unknown-duration-when-not-ended-after-3s", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const audioInit = buildAudioInitSegment();
  const seg0 = buildMediaSegment(0, 0x01);
  let fakeNow = 1000000;
  let polls = 0;
  Date.now = () => fakeNow;

  class FakeSourceBuffer extends EventTarget {
    constructor(mime) {
      super();
      this.mime = mime;
      this.updating = false;
      this._end = 0;
      this.buffered = { get length() { return this._sb._end > 0 ? 1 : 0; }, start: () => 0, end: () => this._end, _sb: null };
      this.buffered._sb = this;
    }
    appendBuffer() {}
    remove() {}
    abort() {}
  }
  class FakeMediaSource extends EventTarget {
    addSourceBuffer(mime) { return new FakeSourceBuffer(mime); }
    removeSourceBuffer() {}
  }

  const videoEl = {
    src: 'blob:https://contoso-my.sharepoint.com/1',
    currentSrc: 'blob:https://contoso-my.sharepoint.com/1',
    duration: NaN,
    ended: false,
    paused: true,
    muted: false,
    volume: 1,
    playbackRate: 1,
    currentTime: 0,
    play: async () => {},
    pause: () => {},
    getAttribute: () => null
  };

  let decodes = 0;
  globalThis.AudioContext = class {
    async decodeAudioData() { decodes++; throw new Error('must not decode unknown-duration stream with !media.ended'); }
    async close() {}
  };
  globalThis.MediaSource = FakeMediaSource;
  globalThis.SourceBuffer = FakeSourceBuffer;
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true };

  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => {
    return origSetTimeout(() => {
      polls++;
      if (polls === 2) {
        // 4 seconds elapse while duration is missing and !videoEl.ended, with mseMaxBufferedEnd = 5 > 0.
        fakeNow += 4000;
      } else if (polls === 5) {
        // Expire deadline while still !videoEl.ended.
        fakeNow += 125000;
      }
      fn(...args);
    }, Math.min(ms || 0, 5));
  };

  const playBtn = {
    click: () => {
      const ms = new FakeMediaSource();
      const audioSB = ms.addSourceBuffer('audio/mp4; codecs="mp4a.40.2"');
      audioSB._end = 5;
      audioSB.appendBuffer(audioInit);
      audioSB.appendBuffer(seg0);
    }
  };
  globalThis.document = {
    querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null),
    querySelectorAll: () => []
  };

  let failure;
  try { await eval(fs.readFileSync(0, 'utf8')); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio MSE stream duration unverified')) {
    throw new Error('expected unverified MSE stream duration error, got: ' + String(failure));
  }
  if (decodes !== 0) {
    throw new Error('decodeAudioData called ' + decodes + ' times');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("Path B unknown-duration !ended probe failed: %v\n%s", err, out)
			}
		})

		// 5. PRRT_kwDOTStvas64MQNJ: Reused XHR restores overridden properties on open() and cleanupHooks() restores all overridden XHR instances.
		t.Run("xhr-restores-overridden-props-on-reuse-and-cleanup", func(t *testing.T) {
			probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { DriveItemID: 'recording-a' };
(async () => {
  const subtle = webcrypto.subtle;
  const iv = new Uint8Array(16).fill(10);
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, false, ['encrypt', 'decrypt']);
  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));

  let nativeXHRSends = 0;

  class FakeXMLHttpRequest extends EventTarget {
    constructor() {
      super();
      this._readyState = 0;
      this._status = 0;
      this._response = null;
      this._responseText = '';
    }
    get readyState() { return this._readyState; }
    get status() { return this._status; }
    get response() { return this._response; }
    get responseText() { return this._responseText; }
    open(method, url) {
      this._url = url;
      this._readyState = 1;
      this._status = 0;
      this._response = null;
      this._responseText = '';
    }
    setRequestHeader() {}
    send() {
      nativeXHRSends++;
      this._readyState = 4;
      this._status = 200;
      this._response = encInit.slice().buffer;
      this._responseText = 'audio-ok';
      this.dispatchEvent(new Event('load'));
    }
    dispatchEvent(ev) {
      const ok = super.dispatchEvent(ev);
      const handler = this['on' + ev.type];
      if (typeof handler === 'function') handler.call(this, ev);
      return ok;
    }
  }
  globalThis.XMLHttpRequest = FakeXMLHttpRequest;

  const base = 'https://contoso-my.sharepoint.com/_api/v2.1/drives/d1/items/recording-a/oneDrive.transcode';
  const videoUrl = base + '?part=mediasegment&track=video&quality=high&segmentTime=0&wsd=50000000&ppd=50000000&driveItemId=recording-a';
  const audioUrl = base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=50000000&driveItemId=recording-a';

  globalThis.fetch = async (url) => {
    const s = String(url);
    if (!s.includes('oneDrive.transcode')) throw new TypeError('Failed to fetch');
    const bytes = s.includes('initsegment') ? encInit : encSeg0;
    let sent = false;
    return {
      ok: true, status: 200, url: s, headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  };
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a', origin: 'https://contoso-my.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = {
    VroomItemId: 'recording-a',
    isDownloadBlocked: true,
    transcodeUrl: audioUrl
  };
  globalThis.AudioContext = class {
    async decodeAudioData() { return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) }; }
    async close() {}
  };

  let videoPhaseObserved = null;
  let openPhaseObserved = null;
  let audioPhaseObserved = null;
  let unreusedVideoXhr = null;
  let ownPropVideoXhr = null;

  const videoEl = { src: 'blob:https://contoso-my.sharepoint.com/1', currentSrc: 'blob:https://contoso-my.sharepoint.com/1', play: async () => {}, pause: () => {}, getAttribute: () => null };
  const playBtn = {
    click: () => {
      const reusedXhr = new globalThis.XMLHttpRequest();
      reusedXhr.onload = () => {
        if (reusedXhr.status === 204) {
          videoPhaseObserved = {
            readyState: reusedXhr.readyState,
            status: reusedXhr.status,
            responseByteLength: reusedXhr.response instanceof ArrayBuffer ? reusedXhr.response.byteLength : -1,
            responseText: reusedXhr.responseText
          };
          reusedXhr.open('GET', audioUrl);
          openPhaseObserved = {
            readyState: reusedXhr.readyState,
            status: reusedXhr.status,
            response: reusedXhr.response,
            responseText: reusedXhr.responseText,
            hasOwnStatus: Object.prototype.hasOwnProperty.call(reusedXhr, 'status'),
            hasOwnReadyState: Object.prototype.hasOwnProperty.call(reusedXhr, 'readyState'),
            hasOwnResponse: Object.prototype.hasOwnProperty.call(reusedXhr, 'response'),
            hasOwnResponseText: Object.prototype.hasOwnProperty.call(reusedXhr, 'responseText')
          };
          reusedXhr.send();
          return;
        }
        audioPhaseObserved = {
          readyState: reusedXhr.readyState,
          status: reusedXhr.status,
          responseByteLength: reusedXhr.response instanceof ArrayBuffer ? reusedXhr.response.byteLength : -1,
          responseText: reusedXhr.responseText,
          hasOwnStatus: Object.prototype.hasOwnProperty.call(reusedXhr, 'status')
        };
      };
      reusedXhr.open('GET', videoUrl);
      reusedXhr.send();

      // Second XHR stays suppressed without reuse so cleanupHooks() must restore it.
      unreusedVideoXhr = new globalThis.XMLHttpRequest();
      unreusedVideoXhr.open('GET', videoUrl);
      unreusedVideoXhr.send();

      // Third XHR has an existing own 'status' property before suppression so cleanupHooks() must restore its descriptor.
      ownPropVideoXhr = new globalThis.XMLHttpRequest();
      Object.defineProperty(ownPropVideoXhr, 'status', { value: 999, writable: true, enumerable: true, configurable: true });
      ownPropVideoXhr.open('GET', videoUrl);
      ownPropVideoXhr.send();

      setTimeout(() => {
        (async () => {
          const r = await globalThis.fetch(audioUrl);
          await globalThis.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, await r.arrayBuffer());
        })().catch(() => {});
      }, 40);
    }
  };
  globalThis.document = { querySelector: sel => sel.includes('Play video') ? playBtn : ((sel === 'audio, video' || sel === 'video') ? videoEl : null), querySelectorAll: () => [] };

  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || !parsed.pcmChunks.length) throw new Error('expected valid PCM');

  if (!videoPhaseObserved || videoPhaseObserved.status !== 204 || videoPhaseObserved.readyState !== 4 || videoPhaseObserved.responseByteLength !== 0 || videoPhaseObserved.responseText !== '') {
    throw new Error('unexpected suppressed video XHR state: ' + JSON.stringify(videoPhaseObserved));
  }
  if (!openPhaseObserved || openPhaseObserved.readyState !== 1 || openPhaseObserved.status !== 0 || openPhaseObserved.response !== null || openPhaseObserved.responseText !== '' ||
      openPhaseObserved.hasOwnStatus || openPhaseObserved.hasOwnReadyState || openPhaseObserved.hasOwnResponse || openPhaseObserved.hasOwnResponseText) {
    throw new Error('expected XHR properties restored on open() for reused instance, got: ' + JSON.stringify(openPhaseObserved));
  }
  if (nativeXHRSends !== 1) {
    throw new Error('expected 1 native XHR send for reused audio request, got ' + nativeXHRSends);
  }
  if (!audioPhaseObserved || audioPhaseObserved.readyState !== 4 || audioPhaseObserved.status !== 200 ||
      audioPhaseObserved.responseByteLength !== encInit.byteLength || audioPhaseObserved.responseText !== 'audio-ok' || audioPhaseObserved.hasOwnStatus) {
    throw new Error('expected clean audio XHR response after reuse, got: ' + JSON.stringify(audioPhaseObserved));
  }
  if (!unreusedVideoXhr || unreusedVideoXhr.readyState !== 1 || unreusedVideoXhr.status !== 0 || unreusedVideoXhr.response !== null || unreusedVideoXhr.responseText !== '' ||
      Object.prototype.hasOwnProperty.call(unreusedVideoXhr, 'status') || Object.prototype.hasOwnProperty.call(unreusedVideoXhr, 'readyState') ||
      Object.prototype.hasOwnProperty.call(unreusedVideoXhr, 'response') || Object.prototype.hasOwnProperty.call(unreusedVideoXhr, 'responseText')) {
    throw new Error('expected cleanupHooks() to restore unreused suppressed XHR properties, got status=' + (unreusedVideoXhr && unreusedVideoXhr.status) + ' hasOwn=' + Object.prototype.hasOwnProperty.call(unreusedVideoXhr, 'status'));
  }
  if (!ownPropVideoXhr || !Object.prototype.hasOwnProperty.call(ownPropVideoXhr, 'status') || ownPropVideoXhr.status !== 999 ||
      Object.prototype.hasOwnProperty.call(ownPropVideoXhr, 'readyState') || ownPropVideoXhr.readyState !== 1) {
    throw new Error('expected cleanupHooks() to restore pre-existing own descriptor on XHR, got status=' + (ownPropVideoXhr && ownPropVideoXhr.status));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapAudioJS)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("XHR reused/cleanup prop restoration probe failed: %v\n%s", err, out)
			}
		})
	})
}

func TestRecapRound4ReviewFixes(t *testing.T) {
	const testRecapURL = "https://teams.cloud.microsoft/l/meetingrecap?driveItemId=recording-a"

	// U1-R3: Retry when xplatplugins.aspx returns "" (lastErr == nil) before streamembed.aspx mounts.
	t.Run("U1-R3-retries-when-xplat-returns-empty-before-streamembed-mounts", func(t *testing.T) {
		var listCalls atomic.Int32
		var serverURL string
		pcm := base64.StdEncoding.EncodeToString(make([]byte, 4608))
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/json/list" {
				n := listCalls.Add(1)
				wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
				targets := []cdpTargetInfo{
					{ID: "owner", Type: "page", URL: testRecapURL, WebSocketDebuggerURL: wsBase + "/owner"},
					{ID: "xplat", ParentID: "owner", Type: "iframe", URL: "https://contoso-my.sharepoint.com/personal/u/_layouts/15/xplatplugins.aspx?driveItemId=recording-a", WebSocketDebuggerURL: wsBase + "/xplat"},
				}
				if n >= 2 {
					targets = append(targets, cdpTargetInfo{
						ID:                   "streamembed",
						ParentID:             "xplat",
						Type:                 "iframe",
						URL:                  "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a",
						WebSocketDebuggerURL: wsBase + "/streamembed",
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
			defer func() { _ = conn.Close() }()
			raw, _, err := wsutil.ReadClientData(conn)
			if err != nil {
				t.Errorf("read: %v", err)
				return
			}
			var req struct {
				ID     int    `json:"id"`
				Method string `json:"method"`
			}
			require.NoError(t, json.Unmarshal(raw, &req))
			var result map[string]any
			switch {
			case req.Method == "Page.getFrameTree":
				result = map[string]any{"frameTree": map[string]any{
					"frame": map[string]string{"id": "owner", "url": testRecapURL},
					"childFrames": []any{
						map[string]any{"frame": map[string]string{"id": "xplat", "url": "https://contoso-my.sharepoint.com/personal/u/_layouts/15/xplatplugins.aspx?driveItemId=recording-a"}},
						map[string]any{"frame": map[string]string{"id": "streamembed", "url": "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a"}},
					},
				}}
			case r.URL.Path == "/streamembed":
				payload, _ := json.Marshal(map[string]any{
					"pageUrl":     testRecapURL,
					"itemId":      "recording-a",
					"sourceBytes": 1024,
					"durationSec": 1.0,
					"pcmChunks":   []string{pcm},
				})
				result = map[string]any{"result": map[string]string{"value": string(payload)}}
			default:
				// Simulate xplatplugins.aspx before streamembed.aspx mounts returning empty string (lastErr == nil).
				result = map[string]any{"result": map[string]string{"value": ""}}
			}
			resp, err := json.Marshal(map[string]any{"id": req.ID, "result": result})
			require.NoError(t, err)
			require.NoError(t, wsutil.WriteServerText(conn, resp))
		}))
		defer srv.Close()
		serverURL = srv.URL

		_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
		require.NoError(t, err)
		port, err := strconv.Atoi(portText)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var out bytes.Buffer
		require.NoError(t, captureRecapAudioFromTargets(ctx, port, "owner", testRecapURL, &out))
		assert.GreaterOrEqual(t, listCalls.Load(), int32(2), "expected retry after empty xplatplugins.aspx eval")
		got := out.Bytes()
		require.True(t, len(got) >= 2 && got[0] == 0xff && (got[1]&0xe0) == 0xe0, "expected valid MP3 frame sync header")
	})

	// U2-R4: __goTeamsRecapAudioTransfer is cleaned up on early exits before the CDP pull loop.
	t.Run("U2-R4-cleanup-pending-transfer-on-early-exits", func(t *testing.T) {
		cases := []struct {
			name        string
			payloadJSON string
			wantErrSub  string
		}{
			{
				name:        "identity-mismatch",
				payloadJSON: `{"pageUrl":"` + testRecapURL + `","itemId":"other-recording","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":2}`,
				wantErrSub:  "identity mismatch",
			},
			{
				name:        "identity-unverified",
				payloadJSON: `{"pageUrl":"https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx","itemId":"","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":2}`,
				wantErrSub:  "identity unverified",
			},
			{
				name:        "payload-error",
				payloadJSON: `{"pageUrl":"` + testRecapURL + `","itemId":"recording-a","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":2,"error":"synthetic failure"}`,
				wantErrSub:  "synthetic failure",
			},
			{
				name:        "size-cap-exceeded",
				payloadJSON: `{"pageUrl":"` + testRecapURL + `","itemId":"recording-a","sourceBytes":` + strconv.Itoa(maxRecapAudioSourceBytes+1) + `,"durationSec":1.0,"transferPending":true,"chunkCount":2}`,
				wantErrSub:  "exceeds maximum supported size/duration",
			},
			{
				name:        "chunk-count-zero",
				payloadJSON: `{"pageUrl":"` + testRecapURL + `","itemId":"recording-a","sourceBytes":1024,"durationSec":1.0,"transferPending":true,"chunkCount":0}`,
				wantErrSub:  "recap audio unavailable",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				var cleanupSeen atomic.Bool
				var serverURL string
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/json/list" {
						wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
						require.NoError(t, json.NewEncoder(w).Encode([]cdpTargetInfo{
							{ID: "owner", Type: "page", URL: testRecapURL, WebSocketDebuggerURL: wsBase + "/owner"},
							{ID: "frame", ParentID: "owner", Type: "iframe", URL: "https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx?driveItemId=recording-a", WebSocketDebuggerURL: wsBase + "/frame"},
						}))
						return
					}
					conn, _, _, err := ws.UpgradeHTTP(r, w)
					if err != nil {
						t.Errorf("upgrade: %v", err)
						return
					}
					defer func() { _ = conn.Close() }()
					raw, _, err := wsutil.ReadClientData(conn)
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
					require.NoError(t, json.Unmarshal(raw, &req))
					var result map[string]any
					switch {
					case req.Method == "Page.getFrameTree":
						result = map[string]any{"frameTree": map[string]any{
							"frame": map[string]string{"id": "owner", "url": testRecapURL},
						}}
					case r.URL.Path == "/frame":
						if len(req.Params.Expression) < 200 && strings.Contains(req.Params.Expression, "delete window.__goTeamsRecapAudioTransfer") {
							cleanupSeen.Store(true)
							result = map[string]any{"result": map[string]string{"value": ""}}
						} else {
							result = map[string]any{"result": map[string]string{"value": tc.payloadJSON}}
						}
					default:
						result = map[string]any{"result": map[string]string{"value": ""}}
					}
					resp, err := json.Marshal(map[string]any{"id": req.ID, "result": result})
					require.NoError(t, err)
					require.NoError(t, wsutil.WriteServerText(conn, resp))
				}))
				defer srv.Close()
				serverURL = srv.URL

				_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
				require.NoError(t, err)
				port, err := strconv.Atoi(portText)
				require.NoError(t, err)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				var out bytes.Buffer
				err = captureRecapAudioFromTargets(ctx, port, "owner", testRecapURL, &out)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErrSub)
				assert.True(t, cleanupSeen.Load(), "expected window.__goTeamsRecapAudioTransfer cleanup on %s", tc.name)
			})
		}
	})

	// U5-R3: fi.id / fi.docId alone must not set m.recapItemID, whereas VroomItemId does.
	t.Run("U5-R3-recap-item-id-ignores-generic-id-and-docId", func(t *testing.T) {
		runExtractTranscriptJS := func(t *testing.T, fileInfoJS string) string {
			t.Helper()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			probeJS := `
const fs = require('fs');
globalThis.window = globalThis;
globalThis.location = { href: 'https://contoso-my.sharepoint.com/_layouts/15/xplatplugins.aspx' };
globalThis.g_fileInfo = ` + fileInfoJS + `;
const listEl = {
  __reactFiber$test: {
    memoizedProps: {
      items: [
        { speakerDisplayName: 'Alice', timestamp: 'PT0M5S', text: 'Hello world', type: 'Text' }
      ]
    }
  }
};
globalThis.document = {
  querySelector: sel => sel === '.ms-List' ? listEl : null,
  querySelectorAll: () => []
};
process.stdout.write(eval(fs.readFileSync(0, 'utf8')));
`
			cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
			cmd.Stdin = strings.NewReader(extractRecapTranscriptJS)
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "extractRecapTranscriptJS failed: %s", out)
			return string(out)
		}

		extractFromTarget := func(t *testing.T, transcriptJSON string) *chromeMeeting {
			t.Helper()
			var serverURL string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/json/list" {
					wsBase := "ws" + strings.TrimPrefix(serverURL, "http")
					require.NoError(t, json.NewEncoder(w).Encode([]cdpTargetInfo{
						{ID: "owner", Type: "page", URL: "https://teams.cloud.microsoft/l/meetingrecap?threadId=19%3Ameeting_abc%40thread.v2", WebSocketDebuggerURL: wsBase + "/owner"},
					}))
					return
				}
				conn, _, _, err := ws.UpgradeHTTP(r, w)
				if err != nil {
					t.Errorf("upgrade: %v", err)
					return
				}
				defer func() { _ = conn.Close() }()
				raw, _, err := wsutil.ReadClientData(conn)
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
				require.NoError(t, json.Unmarshal(raw, &req))
				var result map[string]any
				if strings.Contains(req.Params.Expression, "g_fileInfo") {
					result = map[string]any{"result": map[string]string{"value": transcriptJSON}}
				} else {
					result = map[string]any{"result": map[string]string{"value": ""}}
				}
				resp, err := json.Marshal(map[string]any{"id": req.ID, "result": result})
				require.NoError(t, err)
				require.NoError(t, wsutil.WriteServerText(conn, resp))
			}))
			defer srv.Close()
			serverURL = srv.URL

			_, portText, err := net.SplitHostPort(srv.Listener.Addr().String())
			require.NoError(t, err)
			port, err := strconv.Atoi(portText)
			require.NoError(t, err)

			m := newChromeMeeting(MeetingOptions{
				MeetingURL:       "https://teams.cloud.microsoft/l/meetingrecap?threadId=19%3Ameeting_abc%40thread.v2",
				DisableAISummary: true,
			})
			m.capturePageID = "owner"
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			targets := []cdpTargetInfo{
				{ID: "owner", Type: "page", URL: "https://teams.cloud.microsoft/l/meetingrecap?threadId=19%3Ameeting_abc%40thread.v2", WebSocketDebuggerURL: "ws" + strings.TrimPrefix(serverURL, "http") + "/owner"},
			}
			ok := m.extractRecapFromTargets(ctx, port, targets[0].URL, targets, ownedFrameSet{})
			require.True(t, ok)
			return m
		}

		genericJSON := runExtractTranscriptJS(t, `{ title: 'Sync', name: 'Sync.mp4', id: 'generic-id-123', docId: 'doc-456' }`)
		var genericParsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(genericJSON), &genericParsed))
		assert.Equal(t, "generic-id-123", genericParsed["itemId"], "itemId must stay preserved for existing verifyRecapTargetMatchesMeeting callers")
		assert.Equal(t, "", genericParsed["driveItemId"], "driveItemId must be empty when only fi.id/fi.docId are present")
		mGeneric := extractFromTarget(t, genericJSON)
		assert.Empty(t, mGeneric.recapItemID, "fi.id/fi.docId alone must not set m.recapItemID")

		vroomJSON := runExtractTranscriptJS(t, `{ title: 'Sync', name: 'Sync.mp4', VroomItemId: '01VALIDDRIVEITEMID', id: 'generic-id-123' }`)
		var vroomParsed map[string]any
		require.NoError(t, json.Unmarshal([]byte(vroomJSON), &vroomParsed))
		assert.Equal(t, "01VALIDDRIVEITEMID", vroomParsed["driveItemId"])
		mVroom := extractFromTarget(t, vroomJSON)
		assert.Equal(t, "01VALIDDRIVEITEMID", mVroom.recapItemID, "VroomItemId must populate m.recapItemID")
	})
}

func TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required to execute JS probe tests")
	}

	const mp4HelpersJS = `
function u32(v) {
  return [(v >>> 24) & 0xff, (v >>> 16) & 0xff, (v >>> 8) & 0xff, v & 0xff];
}
function u64(v) {
  const hi = Math.floor(v / 0x100000000);
  const lo = v >>> 0;
  return [...u32(hi), ...u32(lo)];
}
function str4(s) {
  return [s.charCodeAt(0), s.charCodeAt(1), s.charCodeAt(2), s.charCodeAt(3)];
}
function box(type, ...parts) {
  const len = 8 + parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(len);
  out.set(u32(len), 0);
  out.set(str4(type), 4);
  let off = 8;
  for (const p of parts) {
    out.set(p, off);
    off += p.length;
  }
  return out;
}
function buildAudioInitSegment() {
  const ftyp = box('ftyp', new Uint8Array([...str4('iso5'), ...u32(1), ...str4('mp41')]));
  const hdlr = box('hdlr', new Uint8Array([0,0,0,0, 0,0,0,0, ...str4('soun'), ...new Array(12).fill(0), 0]));
  const mp4a = box('mp4a', new Uint8Array([...new Array(6).fill(0), 0,1, ...new Array(8).fill(0), 0,1, 0,16, 0,0, 0,0, 0xac,0x44,0,0]));
  const stsd = box('stsd', new Uint8Array([0,0,0,0, ...u32(1)]), mp4a);
  const stbl = box('stbl', stsd);
  const minf = box('minf', stbl);
  const mdia = box('mdia', hdlr, minf);
  const trak = box('trak', mdia);
  const moov = box('moov', trak);
  const out = new Uint8Array(ftyp.length + moov.length);
  out.set(ftyp, 0);
  out.set(moov, ftyp.length);
  return out;
}
function buildMediaSegment(tfdtTime, tagByte) {
  const styp = box('styp', new Uint8Array([...str4('msdh'), ...u32(0), ...str4('msdh')]));
  const mfhd = box('mfhd', new Uint8Array([0,0,0,0, ...u32(1)]));
  const tfhd = box('tfhd', new Uint8Array([0,2,0,0, ...u32(1)]));
  const tfdt = box('tfdt', new Uint8Array([1,0,0,0, ...u64(tfdtTime)]));
  const traf = box('traf', tfhd, tfdt);
  const moof = box('moof', mfhd, traf);
  const mdat = box('mdat', new Uint8Array([0xde, 0xad, 0xbe, 0xef, tagByte, tagByte, tagByte, tagByte]));
  const out = new Uint8Array(styp.length + moof.length + mdat.length);
  out.set(styp, 0);
  out.set(moof, styp.length);
  out.set(mdat, styp.length + moof.length);
  return out;
}
`

	t.Run("performance-resource-timing-and-enckey-seeding-with-video-wsd-isolation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		probeJS := mp4HelpersJS + `
const fs = require('fs');
const { webcrypto } = require('crypto');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { RecordingName: 'recap.mp4' };
(async () => {
  const subtle = webcrypto.subtle;
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 256 }, true, ['encrypt', 'decrypt']);
  const rawKeyBytes = new Uint8Array(await subtle.exportKey('raw', key));
  const iv = new Uint8Array(16).fill(7);
  const keyB64 = Buffer.from(rawKeyBytes).toString('base64');
  const ivB64 = Buffer.from(iv).toString('base64');

  const encInit = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildAudioInitSegment()));
  const encSeg0 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(0, 0x11)));
  const encSeg1 = new Uint8Array(await subtle.encrypt({ name: 'AES-CBC', iv }, key, buildMediaSegment(96256, 0x22)));

  const base = 'https://contoso.sharepoint.com/_api/v2.1/drives/d1/items/s1/oneDrive.transcode';
  const fetchedAudioTimes = [];
  const fetchedWsds = [];
  let fetchedVideoCount = 0;

  globalThis.fetch = async (url) => {
    const u = new URL(String(url));
    const track = u.searchParams.get('track');
    const wsd = u.searchParams.get('wsd');
    const part = u.searchParams.get('part');
    if (track === 'video') {
      fetchedVideoCount++;
      throw new Error('Path A must not fetch track=video');
    }
    fetchedWsds.push(wsd);
    if (wsd !== '96256') {
      throw new Error('expected audio wsd=96256, got ' + wsd);
    }
    let bytes = encInit;
    if (part === 'mediasegment') {
      const segTime = u.searchParams.get('segmentTime');
      fetchedAudioTimes.push(segTime);
      bytes = segTime === '96256' ? encSeg1 : encSeg0;
    }
    let sent = false;
    return {
      ok: true,
      status: 200,
      url: String(url),
      headers: { get: () => null },
      body: { getReader: () => ({ read: async () => sent ? { done: true } : (sent = true, { done: false, value: bytes }), cancel: async () => {} }) },
      arrayBuffer: async () => bytes.slice().buffer
    };
  };

  globalThis.crypto = { subtle };
  globalThis.location = {
    href: 'https://contoso.sharepoint.com/personal/user_contoso_com/_layouts/15/stream.aspx?id=%2Fpersonal%2Fuser_contoso_com%2FDocuments%2FRecordings%2FRecap.mp4',
    origin: 'https://contoso.sharepoint.com'
  };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = {
    isDownloadBlocked: true,
    itemId: 's1',
    name: 'Recap.mp4',
    transcodeUrl: base + '?track=audio&quality=audcopy',
    EncKey: JSON.stringify({ Key: keyB64, IV: ivB64 })
  };
  let perfPolls = 0;
  globalThis.performance = {
    getEntriesByType: (type) => {
      if (type !== 'resource') return [];
      perfPolls++;
      const entries = [
        { name: 'https://contoso.sharepoint.com/_api/v2.1/drives/d1/items/stale-other/oneDrive.transcode?part=initsegment&track=audio&quality=audcopy&wsd=50000&ppd=100000' },
        { name: base + '?part=initsegment&track=video&quality=vcopy&wsd=96000&ppd=120320000' }
      ];
      if (perfPolls >= 2) {
        entries.push(
          { name: base + '?part=initsegment&track=audio&quality=audcopy&wsd=96256&ppd=192512' },
          { name: base + '?part=mediasegment&track=video&quality=vcopy&segmentTime=0&wsd=96000&ppd=120320000' },
          { name: base + '?part=mediasegment&track=video&quality=vcopy&segmentTime=96000&wsd=96000&ppd=120320000' }
        );
      }
      return entries;
    }
  };
  globalThis.AudioContext = class {
    async decodeAudioData() {
      return { sampleRate: 44100, length: 441, duration: 0.01, numberOfChannels: 1, getChannelData: () => new Float32Array(441) };
    }
    async close() {}
  };
  globalThis.document = { querySelector: () => null, querySelectorAll: () => [] };

  const raw = await eval(fs.readFileSync(0, 'utf8'));
  const parsed = JSON.parse(raw);
  if (!parsed.pcmChunks || parsed.pcmChunks.length === 0) {
    throw new Error('expected non-empty pcmChunks');
  }
  if (fetchedVideoCount !== 0) {
    throw new Error('unexpected video fetch count: ' + fetchedVideoCount);
  }
  if (JSON.stringify(fetchedAudioTimes.sort()) !== JSON.stringify(['0', '96256'])) {
    throw new Error('unexpected audio segmentTimes: ' + JSON.stringify(fetchedAudioTimes));
  }
  if (fetchedWsds.some(w => w !== '96256')) {
    throw new Error('unexpected wsd in audio fetches: ' + JSON.stringify(fetchedWsds));
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
		cmd := exec.CommandContext(ctx, "node", "-e", probeJS)
		cmd.Stdin = strings.NewReader(extractRecapAudioJS)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("SharePoint stream.aspx PerformanceResourceTiming/EncKey probe failed: %v\n%s", err, out)
		}
	})

	t.Run("transcript-tab-activation-and-copilot-ai-summary", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		transcriptProbeJS := `
const fs = require('fs');
const txJS = fs.readFileSync(0, 'utf8');
globalThis.window = globalThis;
globalThis.location = { href: 'https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FSync.mp4' };
globalThis.g_fileInfo = { title: 'Stream Sync', name: 'Sync.mp4' };

let listMounted = false;
let txClickCount = 0;
const txBtn = {
  dataset: {},
  click: () => {
    txClickCount++;
    listMounted = true;
  }
};
const listEl = {
  __reactFiber$test: {
    memoizedProps: {
      items: [
        { speakerDisplayName: 'Alice', timestamp: 'PT0M5S', text: 'Activated transcript.', type: 'Text' }
      ]
    }
  }
};
globalThis.document = {
  querySelector: (sel) => {
    if (sel === '.ms-List') return listMounted ? listEl : null;
    if (sel.includes('aria-label="Transcript"')) return txBtn;
    return null;
  },
  querySelectorAll: () => []
};

const first = JSON.parse(eval(txJS));
if (!Array.isArray(first.entries) || first.entries.length !== 0) {
  throw new Error('expected empty entries on first poll before Transcript tab mounts');
}
if (txClickCount !== 1 || txBtn.dataset.goTeamsTranscriptClicked !== 'true') {
  throw new Error('expected Transcript button clicked once with dataset marker');
}
const second = JSON.parse(eval(txJS));
if (!Array.isArray(second.entries) || second.entries.length !== 1 || second.entries[0].text !== 'Activated transcript.') {
  throw new Error('expected mounted transcript entries on second poll, got: ' + JSON.stringify(second));
}
if (txClickCount !== 1) {
  throw new Error('expected Transcript button not to be clicked twice');
}
`
		cmd := exec.CommandContext(ctx, "node", "-e", transcriptProbeJS)
		cmd.Stdin = strings.NewReader(extractRecapTranscriptJS)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("SharePoint stream.aspx transcript tab activation probe failed: %v\n%s", err, out)
		}

		copilotProbeJS := `
const fs = require('fs');
const copyJS = fs.readFileSync(0, 'utf8');
(async () => {
  globalThis.window = globalThis;
  globalThis.location = { href: 'https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FSync.mp4' };
  let copilotOpen = false;
  let copilotClicks = 0;
  let recapClicks = 0;
  let summaryReady = false;

  const copilotBtn = {
    dataset: {},
    click: () => {
      copilotClicks++;
      copilotOpen = true;
    }
  };
  const closeCopilotBtn = { dataset: {} };
  const recapBtn = {
    dataset: {},
    innerText: 'Recap the meeting',
    click: () => {
      recapClicks++;
      summaryReady = true;
    }
  };
  const preExistingMsg = {
    dataset: {},
    querySelector: (sel) => sel === '.fai-CopilotMessage__content' ? { innerText: 'Stale earlier Copilot reply.' } : null
  };
  const preExistingCopyBtn = {
    closest: (sel) => sel === '.fai-CopilotMessage' ? preExistingMsg : null
  };
  const sentPromptBubble = {
    dataset: {},
    innerText: 'Recap the meeting',
    click: () => { throw new Error('must not re-click sent user prompt bubble'); }
  };
  const msgEl = {
    dataset: {},
    querySelector: (sel) => sel === '.fai-CopilotMessage__content' ? { innerText: 'Generated Copilot meeting summary.' } : null
  };
  const spCopyBtn = {
    closest: (sel) => sel === '.fai-CopilotMessage' ? msgEl : null
  };

  globalThis.document = {
    documentElement: { dataset: {} },
    querySelector: (sel) => {
      if (sel === 'button[aria-label="Close Copilot"]') return copilotOpen ? closeCopilotBtn : null;
      if (sel.includes('aria-label="Copilot"')) return copilotBtn;
      if (sel.includes('.fai-CopilotMessage__actions')) return summaryReady ? spCopyBtn : preExistingCopyBtn;
      return null;
    },
    querySelectorAll: (sel) => {
      if (sel === '.fai-CopilotMessage') return summaryReady ? [preExistingMsg, msgEl] : [preExistingMsg];
      if (sel.includes('PromptStarter')) return copilotOpen ? (summaryReady ? [sentPromptBubble] : [recapBtn]) : [];
      if (sel.includes('.fai-CopilotMessage__actions')) return summaryReady ? [preExistingCopyBtn, spCopyBtn] : [preExistingCopyBtn];
      return [];
    },
    addEventListener: () => {},
    removeEventListener: () => {}
  };
  globalThis.navigator = {};

  const poll1 = await eval(copyJS);
  if (poll1 !== '') {
    throw new Error('expected empty summary on initial Copilot trigger poll, got: ' + JSON.stringify(poll1));
  }
  if (copilotClicks !== 1 || copilotBtn.dataset.goTeamsCopilotClicked !== 'true') {
    throw new Error('expected Copilot button clicked once, got ' + copilotClicks);
  }
  if (recapClicks !== 1 || recapBtn.dataset.goTeamsRecapClicked !== 'true') {
    throw new Error('expected Recap the meeting button clicked once, got ' + recapClicks);
  }

  // While summaryReady is false, a poll must not return the stale pre-existing Copilot message.
  summaryReady = false;
  const pendingPoll = await eval(copyJS);
  if (pendingPoll !== '') {
    throw new Error('expected empty summary while new recap is still generating, got: ' + JSON.stringify(pendingPoll));
  }

  summaryReady = true;
  const poll2 = await eval(copyJS);
  if (poll2 !== 'Generated Copilot meeting summary.') {
    throw new Error('expected extracted Copilot summary on poll 2, got: ' + JSON.stringify(poll2));
  }
  if (copilotClicks !== 1 || recapClicks !== 1) {
    throw new Error('expected Copilot/Recap buttons not to be clicked repeatedly');
  }

  // Ensure Teams Recap surfaces do not click SharePoint Copilot controls while Copy all is still loading.
  let teamsGuardViolated = false;
  const teamsNotesTab = { getAttribute: () => 'true', click: () => {} };
  const guardCopilotBtn = { dataset: {}, click: () => { teamsGuardViolated = true; } };
  const guardRecapBtn = { dataset: {}, innerText: 'Recap the meeting', click: () => { teamsGuardViolated = true; } };
  globalThis.document = {
    querySelector: (sel) => {
      if (sel === 'button[role="tab"][data-tid="Notes"]') return teamsNotesTab;
      if (sel === '[data-tid="intelligent-recap-header"]') return {};
      if (sel.includes('aria-label="Copilot"')) return guardCopilotBtn;
      return null;
    },
    querySelectorAll: (sel) => {
      if (sel.includes('.fai-CopilotMessage__actions')) return [];
      return [guardRecapBtn];
    },
    addEventListener: () => {},
    removeEventListener: () => {}
  };
  const teamsPoll = await eval(copyJS);
  if (teamsPoll !== '' || teamsGuardViolated) {
    throw new Error('Teams Recap surface must not click SharePoint Copilot controls');
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
		cmd2 := exec.CommandContext(ctx, "node", "-e", copilotProbeJS)
		cmd2.Stdin = strings.NewReader(copyAISummaryJS)
		if out, err := cmd2.CombinedOutput(); err != nil {
			t.Fatalf("SharePoint stream.aspx Copilot AI summary probe failed: %v\n%s", err, out)
		}
	})

	t.Run("stream-aspx-id-query-recording-name-and-target-url-matching", func(t *testing.T) {
		streamURL := "https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FRecap-20261116.mp4&referrer=StreamWebApp.Web"
		id := ExtractMeetingIdentity(streamURL)
		assert.Equal(t, "recap-20261116.mp4", id.RecordingName)

		nonMediaID := ExtractMeetingIdentity("https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=222")
		assert.Empty(t, nonMediaID.RecordingName, "numeric id query parameter must not be treated as a recording filename")

		sharingID := ExtractMeetingIdentity("https://contoso.sharepoint.com/:v:/p/u/EXabc?id=%2F%3Av%3A%2Fp%2Fu%2FEXabc.mp4")
		assert.Empty(t, sharingID.RecordingName, "sharing /: path in id query parameter must be ignored")

		assert.True(t, hasSpecificMeetingOrRecapTarget(streamURL))
		assert.True(t, isOnTargetMeetingOrRecapURL(streamURL, streamURL+"&ga=1", "/personal/u/_layouts/15/stream.aspx", ""))
		assert.False(t, isOnTargetMeetingOrRecapURL(streamURL, "https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=222", "/personal/u/_layouts/15/stream.aspx", ""), "stream.aspx page lacking recording identity must not match a recording-scoped streamURL")
		otherStreamURL := "https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FOther-20261116.mp4"
		assert.False(t, isOnTargetMeetingOrRecapURL(streamURL, otherStreamURL, "/personal/u/_layouts/15/stream.aspx", ""), "conflicting RecordingName must be rejected")

		// Verify % and # characters in query-encoded server-relative ?id= paths are preserved without double decoding.
		pctURL := "https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2F100%25%20Sync.mp4"
		assert.Equal(t, "100% sync.mp4", ExtractMeetingIdentity(pctURL).RecordingName)
		hashURL := "https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FQ4%20%231.mp4"
		assert.Equal(t, "q4 #1.mp4", ExtractMeetingIdentity(hashURL).RecordingName)

		// F-2 & %/# preservation in JS checkIdentityAgainst + no-activity fast-fail when only EncKey is present.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		mismatchJS := `
const fs = require('fs');
const { webcrypto } = require('crypto');
const audioJS = fs.readFileSync(0, 'utf8');
const maxRecapAudioSourceBytes = 1048576, maxRecapAudioDurationSec = 120, maxRecapAudioPCMBytes = 10485760;
const recapExpectedIdentity = { RecordingName: '100% sync.mp4' };
(async () => {
  globalThis.location = { href: 'https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FOther-20261116.mp4', origin: 'https://contoso.sharepoint.com' };
  globalThis.window = globalThis;
  globalThis.g_fileInfo = { isDownloadBlocked: true };
  globalThis.document = { querySelector: () => null, querySelectorAll: () => [] };
  let failure;
  try { await eval(audioJS); } catch (e) { failure = e; }
  if (!failure || !String(failure.message || failure).includes('recap audio recording identity mismatch')) {
    throw new Error('expected ?id= identity mismatch error, got: ' + String(failure));
  }

  // Matching 100% Sync.mp4 with EncKey only and no transcode/MSE activity must exit via 10s initialActivityTimeoutMs (fast fake clock), not wait 120s.
  const subtle = webcrypto.subtle;
  const key = await subtle.generateKey({ name: 'AES-CBC', length: 128 }, true, ['decrypt']);
  const rawKeyBytes = new Uint8Array(await subtle.exportKey('raw', key));
  const iv = new Uint8Array(16).fill(3);
  let fakeNow = 1000;
  Date.now = () => fakeNow;
  const origSetTimeout = globalThis.setTimeout;
  globalThis.setTimeout = (fn, ms, ...args) => origSetTimeout(() => { fakeNow += 4000; fn(...args); }, Math.min(ms || 0, 5));
  globalThis.crypto = { subtle };
  globalThis.location = { href: 'https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2F100%25%20Sync.mp4', origin: 'https://contoso.sharepoint.com' };
  globalThis.g_fileInfo = {
    isDownloadBlocked: true,
    EncKey: JSON.stringify({ Key: Buffer.from(rawKeyBytes).toString('base64'), IV: Buffer.from(iv).toString('base64') })
  };
  let encOnlyErr;
  try { await eval(audioJS); } catch (e) { encOnlyErr = e; }
  if (!encOnlyErr || !String(encOnlyErr.message || encOnlyErr).includes('isDownloadBlocked=true')) {
    throw new Error('expected fast-fail isDownloadBlocked error when only EncKey is present, got: ' + String(encOnlyErr));
  }
  if (fakeNow >= 120000) {
    throw new Error('expected 10s initialActivityTimeoutMs fast-fail rather than 120s deadline wait, fakeNow=' + fakeNow);
  }
})().catch(e => { console.error(e && e.stack ? e.stack : String(e)); process.exitCode = 1; });
`
		cmdMismatch := exec.CommandContext(ctx, "node", "-e", mismatchJS)
		cmdMismatch.Stdin = strings.NewReader(extractRecapAudioJS)
		if out, err := cmdMismatch.CombinedOutput(); err != nil {
			t.Fatalf("SharePoint ?id= identity mismatch / EncKey fast-fail probe failed: %v\n%s", err, out)
		}
	})
}
