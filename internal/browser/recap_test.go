package browser

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
}
