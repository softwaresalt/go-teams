package browser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/softwaresalt/go-teams/internal/audio"
	"github.com/softwaresalt/go-teams/internal/teams"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

// admissionPollInterval controls how often pre-join and admission loops
// re-check the DOM.
const (
	admissionPollInterval    = 1 * time.Second
	callEndedPollInterval    = 2 * time.Second
	defaultForceRecapTimeout = 2 * time.Minute
	captionMenuStepWait      = 5 * time.Second
	captionMenuPoll          = 150 * time.Millisecond
)

// chromeMeeting is the chromedp-backed Meeting implementation. Its methods
// operate on a chromedp browser context supplied by the caller so that a single
// browser session spans join, captions, and audio.
type chromeMeeting struct {
	opts                 MeetingOptions
	headedOpts           *Options
	cdpPort              int
	capturePageID        string
	allowSingleTabRebind bool
	otherPageIDs         map[string]bool
	headedMu             sync.Mutex
	headedStop           func()
	attachedCtx          context.Context
	recapEntries         []RecapTranscriptEntry
	recapMeta            MeetingMetadata
	// recapAudioURL is the selected Recap page URL retained when the requested
	// URL carries no recording identity (interactive Recap selection), so audio
	// validation still binds to the recording that supplied the transcript.
	recapAudioURL string
	aiSummary     string

	activityMu        sync.Mutex
	onCaptionActivity func()
	onAudioActivity   func()
	onCallEnded       func()

	pollInterval        time.Duration
	callEndPoll         time.Duration
	probePreJoinState   func(ctx context.Context) (toolbar, lobby, prejoin, launcher bool, err error)
	probeAdmissionState func(ctx context.Context) (toolbar, lobby bool, err error)
}

func newChromeMeeting(opts MeetingOptions) *chromeMeeting {
	return &chromeMeeting{opts: opts}
}

// SetActivityCallbacks implements ActivityMonitor.
func (m *chromeMeeting) SetActivityCallbacks(onCaption, onAudio, onCallEnd func()) {
	m.activityMu.Lock()
	defer m.activityMu.Unlock()
	m.onCaptionActivity = onCaption
	m.onAudioActivity = onAudio
	m.onCallEnded = onCallEnd
}

func (m *chromeMeeting) notifyCaptionActivity() {
	m.activityMu.Lock()
	cb := m.onCaptionActivity
	if cb == nil {
		cb = m.opts.OnCaptionActivity
	}
	m.activityMu.Unlock()
	if cb != nil {
		cb()
	}
}

func (m *chromeMeeting) notifyAudioActivity() {
	m.activityMu.Lock()
	cb := m.onAudioActivity
	if cb == nil {
		cb = m.opts.OnAudioActivity
	}
	m.activityMu.Unlock()
	if cb != nil {
		cb()
	}
}

func (m *chromeMeeting) notifyCallEnded() {
	m.activityMu.Lock()
	cb := m.onCallEnded
	if cb == nil {
		cb = m.opts.OnCallEnded
	}
	m.activityMu.Unlock()
	if cb != nil {
		cb()
	}
}

func (m *chromeMeeting) effectivePollInterval() time.Duration {
	if m.pollInterval > 0 {
		return m.pollInterval
	}
	return admissionPollInterval
}

func (m *chromeMeeting) lobbyDeadline() time.Time {
	if m.opts.LobbyTimeout > 0 {
		return time.Now().Add(m.opts.LobbyTimeout)
	}
	if m.opts.ForceRecap {
		return time.Now().Add(defaultForceRecapTimeout)
	}
	return time.Time{}
}

func (m *chromeMeeting) withDeadlineContext(ctx context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	if deadline.IsZero() {
		return context.WithCancel(ctx)
	}
	return context.WithDeadline(ctx, deadline)
}

func (m *chromeMeeting) isLobbyDeadlineExceeded(parentCtx context.Context, deadline time.Time, err error) bool {
	if deadline.IsZero() || parentCtx.Err() != nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || !time.Now().Before(deadline)
}

func (m *chromeMeeting) defaultAccount() string {
	if acct := strings.TrimSpace(m.opts.DefaultAccount); acct != "" {
		return acct
	}
	if m.headedOpts != nil {
		return strings.TrimSpace(m.headedOpts.DefaultAccount)
	}
	return ""
}

// AISummary implements AISummaryProvider.
func (m *chromeMeeting) AISummary() string {
	return m.aiSummary
}

// MeetingMetadata implements MetadataProvider.
func (m *chromeMeeting) MeetingMetadata() MeetingMetadata {
	return m.recapMeta
}

// retainRecapAudioIdentity records pageURL as the identity source for Recap
// audio validation when the requested meeting URL carries no recording
// identity (for example the default Teams home URL used when the operator
// selects a Recap interactively). A requested URL that has an identity always
// stays authoritative, so recording isolation is never relaxed.
func (m *chromeMeeting) retainRecapAudioIdentity(pageURL string) {
	if ExtractMeetingIdentity(m.opts.MeetingURL) != (MeetingIdentity{}) {
		return
	}
	if ExtractMeetingIdentity(pageURL) == (MeetingIdentity{}) {
		return
	}
	m.recapAudioURL = pageURL
}

// IsRecap implements RecapMeeting.
func (m *chromeMeeting) IsRecap() bool {
	return len(m.recapEntries) > 0
}

func (m *chromeMeeting) stopHeadedBrowser() {
	m.headedMu.Lock()
	stop := m.headedStop
	m.headedStop = nil
	m.headedMu.Unlock()
	if stop != nil {
		stop()
	}
}

func deriveRecapURLFromMeetingURL(meetingURL string) string {
	lower := strings.ToLower(strings.TrimSpace(meetingURL))
	if strings.Contains(lower, "meetingrecap") {
		return ""
	}
	id := ExtractMeetingIdentity(meetingURL)
	if id.ThreadID == "" {
		return ""
	}
	host := "teams.cloud.microsoft"
	if hasHTTPSHostSuffix(meetingURL, "teams.microsoft.com") {
		host = "teams.microsoft.com"
	}
	return fmt.Sprintf("https://%s/l/meetingrecap?threadId=%s", host, url.QueryEscape(id.ThreadID))
}

// Join implements Meeting. It validates the meeting URL, navigates to it, waits
// for user authentication or launcher interstitial handling as needed, and
// detects either a Teams Meeting Recap transcript (via xplatplugins.aspx /
// .ms-List) or a live call pre-join / active call toolbar.
func (m *chromeMeeting) Join(ctx context.Context) error {
	if m.headedOpts == nil && m.probePreJoinState == nil && chromedp.FromContext(ctx) == nil {
		return fmt.Errorf("browser: join requires a chromedp browser context; create one with chromedp.NewContext")
	}
	if err := teams.ValidateMeetingURL(m.opts.MeetingURL); err != nil {
		return fmt.Errorf("browser: invalid meeting url: %w", err)
	}
	deadline := m.lobbyDeadline()
	if m.headedOpts != nil {
		return m.joinHeadedWithDeadline(ctx, deadline)
	}
	if m.probePreJoinState == nil {
		targetURL := m.opts.MeetingURL
		if m.opts.ForceRecap {
			if recapURL := deriveRecapURLFromMeetingURL(targetURL); recapURL != "" {
				targetURL = recapURL
			}
		}
		if err := chromedp.Run(ctx, chromedp.Navigate(targetURL)); err != nil {
			if m.isLobbyDeadlineExceeded(ctx, deadline, err) {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
			return fmt.Errorf("browser: navigate to meeting: %w", err)
		}
	}
	return m.awaitPreJoinOrAdmissionWithDeadline(ctx, deadline)
}

const teamsV2BootstrapURL = "https://teams.cloud.microsoft/v2/"

func teamsV2BootstrapURLFor(meetingURL string) string {
	if hasHTTPSHostSuffix(meetingURL, "teams.microsoft.com") {
		return "https://teams.microsoft.com/v2/"
	}
	return teamsV2BootstrapURL
}

type headedPageState struct {
	PageURL        string `json:"-"`
	Host           string `json:"host"`
	Path           string `json:"path"`
	Hash           string `json:"hash"`
	ReadyState     string `json:"readyState"`
	Toolbar        bool   `json:"toolbar"`
	Lobby          bool   `json:"lobby"`
	Prejoin        bool   `json:"prejoin"`
	Launcher       bool   `json:"launcher"`
	RecapHeader    bool   `json:"recapHeader"`
	TidCount       int    `json:"tidCount"`
	AuthInFlight   bool   `json:"authInFlight"`
	MsalReady      bool   `json:"msalReady"`
	SkypeReady     bool   `json:"skypeReady"`
	HasRecapIframe bool   `json:"-"`
}

// headedAuthTracker manages post-authentication URL restoration and cold-profile
// bootstrap state so headed capture neither gets stuck in a /l/meetingrecap
// OAuth redirect loop nor interrupts in-progress user sign-in or page loads.
type headedAuthTracker struct {
	meetingURL                string
	needsPostAuthMeetingNav   bool
	appliedPostAuthMeetingURL bool
	bootstrappedTeamsHome     bool
	sawAuthRedirect           bool
	lastWasTeams              bool
	rapidAuthBounces          int
	authStayPolls             int
	teamsStayPolls            int
	settledPolls              int
	reNavigateAttempts        int
}

func newHeadedAuthTracker(meetingURL string, coldProfile bool) (*headedAuthTracker, string) {
	t := &headedAuthTracker{meetingURL: meetingURL}
	if coldProfile && isMicrosoftTeamsRecapURL(meetingURL) {
		t.needsPostAuthMeetingNav = true
		t.bootstrappedTeamsHome = true
		return t, teamsV2BootstrapURLFor(meetingURL)
	}
	return t, meetingURL
}

func (t *headedAuthTracker) onAuthRedirect(_ string) string {
	t.sawAuthRedirect = true
	t.settledPolls = 0
	if t.lastWasTeams {
		t.lastWasTeams = false
		t.authStayPolls = 1
		if t.teamsStayPolls > 0 && t.teamsStayPolls <= 3 {
			t.rapidAuthBounces++
		} else {
			t.rapidAuthBounces = 1
		}
		t.teamsStayPolls = 0
	} else {
		t.authStayPolls++
		if t.authStayPolls > 4 {
			// User is stationary on the login/MFA/KMSI screen; never treat user
			// interaction time as an automatic redirect bounce loop.
			t.rapidAuthBounces = 0
		}
	}
	if t.rapidAuthBounces >= 3 && t.authStayPolls >= 2 && t.reNavigateAttempts < 2 {
		t.rapidAuthBounces = 0
		t.authStayPolls = 0
		t.reNavigateAttempts++
		if isMicrosoftTeamsRecapURL(t.meetingURL) && !t.bootstrappedTeamsHome {
			t.bootstrappedTeamsHome = true
			t.needsPostAuthMeetingNav = true
			t.appliedPostAuthMeetingURL = false
			return teamsV2BootstrapURLFor(t.meetingURL)
		}
		t.appliedPostAuthMeetingURL = true
		return t.meetingURL
	}
	return ""
}

func (t *headedAuthTracker) onNonAuthPage(st headedPageState) string {
	hostLower := strings.ToLower(st.Host)
	if !strings.Contains(hostLower, "teams.cloud.microsoft") &&
		!strings.Contains(hostLower, "teams.microsoft.com") &&
		!strings.Contains(hostLower, "teams.live.com") {
		// External federated SSO/IdP host during login; do not count as Teams.
		t.settledPolls = 0
		return ""
	}
	t.lastWasTeams = true
	t.authStayPolls = 0
	t.teamsStayPolls++
	if t.teamsStayPolls > 3 {
		t.rapidAuthBounces = 0
	}
	if st.Launcher || st.Toolbar || st.Lobby || st.Prejoin || st.RecapHeader || st.HasRecapIframe {
		t.settledPolls = 0
		return ""
	}
	if st.AuthInFlight || (st.ReadyState != "" && st.ReadyState != "complete") || !st.MsalReady {
		t.settledPolls = 0
		return ""
	}
	if !st.SkypeReady && st.TidCount <= 15 {
		t.settledPolls = 0
		return ""
	}

	t.settledPolls++
	onTarget := isOnTargetMeetingOrRecapURL(t.meetingURL, st.PageURL, st.Path, st.Hash)
	if (!onTarget || strings.Contains(strings.ToLower(st.Hash), "launchagent=join_launcher_web")) &&
		!t.appliedPostAuthMeetingURL &&
		(t.needsPostAuthMeetingNav || t.sawAuthRedirect || hasSpecificMeetingOrRecapTarget(t.meetingURL)) {
		threshold := 5
		if st.TidCount > 15 && st.SkypeReady {
			threshold = 3
		}
		if t.settledPolls >= threshold {
			t.needsPostAuthMeetingNav = false
			t.appliedPostAuthMeetingURL = true
			t.settledPolls = 0
			t.reNavigateAttempts++
			return t.meetingURL
		}
		return ""
	}

	if t.settledPolls >= 20 && t.reNavigateAttempts < 2 && hasSpecificMeetingOrRecapTarget(t.meetingURL) {
		t.settledPolls = 0
		t.reNavigateAttempts++
		t.appliedPostAuthMeetingURL = true
		return t.meetingURL
	}
	return ""
}

func hasSpecificMeetingOrRecapTarget(meetingURL string) bool {
	lower := strings.ToLower(strings.TrimSpace(meetingURL))
	if strings.Contains(lower, "meetingrecap") ||
		strings.Contains(lower, "meetup-join") ||
		strings.Contains(lower, "/meet/") {
		return true
	}
	id := ExtractMeetingIdentity(meetingURL)
	return id.ThreadID != "" || id.MeetingToken != "" || id.DriveItemID != "" || id.CallID != "" || id.ICalUID != "" || id.MeetID != ""
}

func isOnTargetMeetingOrRecapURL(meetingURL, pageURL, path, hash string) bool {
	if !hasSpecificMeetingOrRecapTarget(meetingURL) {
		return true
	}
	if hasPositiveMeetingIdentityMatch(meetingURL, pageURL) {
		return true
	}
	lowerMeeting := strings.ToLower(meetingURL)
	combined := strings.ToLower(pageURL + " " + path + " " + hash)
	switch {
	case strings.Contains(lowerMeeting, "meetingrecap"):
		return strings.Contains(combined, "meetingrecap")
	case strings.Contains(lowerMeeting, "meetup-join"):
		return strings.Contains(combined, "meetup-join")
	case strings.Contains(lowerMeeting, "/meet/"):
		return strings.Contains(combined, "/meet/")
	default:
		return strings.TrimRight(strings.ToLower(pageURL), "/") == strings.TrimRight(lowerMeeting, "/")
	}
}

func (m *chromeMeeting) joinHeaded(ctx context.Context) error {
	return m.joinHeadedWithDeadline(ctx, m.lobbyDeadline())
}

func (m *chromeMeeting) joinHeadedWithDeadline(ctx context.Context, deadline time.Time) error {
	coldProfile := !hasTeamsSessionInProfile(m.headedOpts.UserDataDir)
	effectiveMeetingURL := m.opts.MeetingURL
	if m.opts.ForceRecap {
		if recapURL := deriveRecapURLFromMeetingURL(effectiveMeetingURL); recapURL != "" {
			effectiveMeetingURL = recapURL
		}
	}
	authTracker, initialURL := newHeadedAuthTracker(effectiveMeetingURL, coldProfile)
	if initialURL != effectiveMeetingURL {
		slog.Info("browser: new or unauthenticated profile detected; bootstrapping Teams session at /v2/ before opening meeting recap")
	}
	port, wsURL, targetID, procDone, cleanupProc, err := launchCleanHeadedBrowser(*m.headedOpts, initialURL)
	if err != nil {
		return err
	}
	m.cdpPort = port
	m.capturePageID = targetID
	m.allowSingleTabRebind = true
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	m.headedMu.Lock()
	m.headedStop = func() {
		cancelAlloc()
		cleanupProc()
	}
	m.headedMu.Unlock()

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	loggedWaiting := false
	clickedLauncher := false
	consecutiveListFailures := 0
	missingTargetPolls := 0

	for {
		if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
			return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
		}
		targets, err := listCDPTargets(port)
		if err != nil {
			consecutiveListFailures++
			select {
			case <-procDone:
				return fmt.Errorf("browser: browser process exited before meeting admission: %w", err)
			default:
			}
			if consecutiveListFailures >= 5 {
				return fmt.Errorf("browser: DevTools endpoint unreachable before meeting admission: %w", err)
			}
		} else {
			consecutiveListFailures = 0
			mainPage, found := m.findBoundCapturePage(targets)
			if !found {
				missingTargetPolls++
				if missingTargetPolls >= 5 {
					return fmt.Errorf("browser: capture tab %s was closed before meeting admission", m.capturePageID)
				}
			} else if isAuthRedirectURL(mainPage.URL) {
				missingTargetPolls = 0
				if !loggedWaiting {
					slog.Info("browser: waiting for user authentication in browser window")
					loggedWaiting = true
				}
				if acct := m.defaultAccount(); acct != "" {
					if clickedRaw, evalErr := evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, selectEntraAccountJS(acct)); evalErr == nil && strings.TrimSpace(clickedRaw) == "true" {
						slog.Info("browser: selected default Entra account", "account", redactAccountIdentifier(acct))
						authTracker.settledPolls = 0
						authTracker.rapidAuthBounces = 0
					}
				}
				if navURL := authTracker.onAuthRedirect(mainPage.URL); navURL != "" {
					clickedLauncher = false
					slog.Info("browser: detected rapid auth redirect bounce loop; navigating to break loop", "attempt", authTracker.reNavigateAttempts)
					_ = navigateCDPTarget(ctx, mainPage.WebSocketDebuggerURL, navURL)
				}
			} else {
				missingTargetPolls = 0
				ownedFrames := fetchOwnedFrames(ctx, mainPage.WebSocketDebuggerURL)
				hasRecapIframe := false
				for i := range targets {
					t := &targets[i]
					if !isTargetInCaptureScope(*t, m.capturePageID, ownedFrames) {
						continue
					}
					if isRecapIframeTarget(*t) {
						hasRecapIframe = true
					}

					// Check if this target (e.g. xplatplugins.aspx iframe or recap page)
					// has a Meeting Recap transcript list populated for the requested meeting.
					if raw, evalErr := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, extractRecapTranscriptJS); evalErr == nil && raw != "" {
						if entries, meta, pErr := parseRecapPayloadForMeeting(raw, m.opts.MeetingURL, mainPage.URL, t.URL); pErr == nil && len(entries) > 0 {
							m.recapEntries = entries
							if summary := captureAISummaryFromTargets(ctx, port, m.capturePageID); summary != "" {
								m.aiSummary = summary
								slog.Info("browser: copied AI summary from meeting recap")
							}
							enrichRecapMetadataFromPage(ctx, port, m.capturePageID, &meta)
							m.recapMeta = meta
							m.retainRecapAudioIdentity(mainPage.URL)
							slog.Info("browser: extracted meeting recap transcript", "entries", len(entries), "title", meta.Title, "attendees", len(meta.Attendees))
							return nil
						}
					}
				}
				if m.opts.ForceRecap && isOnTargetMeetingOrRecapURL(m.opts.MeetingURL, mainPage.URL, "", "") {
					if summary := captureAISummaryFromTargets(ctx, port, m.capturePageID); summary != "" {
						m.aiSummary = summary
						m.retainRecapAudioIdentity(mainPage.URL)
						slog.Info("browser: copied AI summary from meeting recap")
						return nil
					}
				}

				stateRaw, evalErr := evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, `JSON.stringify({
					host: location.host,
					path: location.pathname,
					hash: location.hash,
					readyState: document.readyState || '',
					toolbar: document.querySelector('#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]') !== null,
					lobby: document.body !== null && document.body.innerText.includes("Someone will let you in shortly"),
					prejoin: document.querySelector('[data-tid="prejoin-join-button"]') !== null,
					launcher: document.querySelector('#openTeamsClientInBrowser, button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]') !== null,
					recapHeader: document.querySelector('[data-tid="intelligent-recap-header"]') !== null,
					tidCount: document.querySelectorAll('[data-tid]').length,
					authInFlight: (() => {
						try {
							const h = (location.hash || '') + '&' + (location.search || '');
							if (/(?:^|[#?&])(code|id_token|access_token|session_state|client_info|error)=/i.test(h)) {
								return true;
							}
							for (const store of [window.sessionStorage, window.localStorage]) {
								if (!store) continue;
								for (let i = 0; i < store.length; i++) {
									const k = store.key(i) || '';
									if (k.endsWith('interaction.status') && store.getItem(k) === 'interaction_in_progress') {
										return true;
									}
								}
							}
							return false;
						} catch (_) {
							return false;
						}
					})(),
					msalReady: (() => {
						try {
							return Boolean(
								localStorage.getItem('msal.2.account.keys') ||
								localStorage.getItem('msal.account.keys') ||
								Object.keys(localStorage).some(k => k.includes('SKYPE-TOKEN') || k.includes('UserAuthState'))
							);
						} catch (_) {
							return false;
						}
					})(),
					skypeReady: (() => {
						try {
							return Object.keys(localStorage).some(k =>
								k.includes('SKYPE-TOKEN') ||
								k.includes('UserAuthState') ||
								k.includes('msal.2.token.keys')
							);
						} catch (_) {
							return false;
						}
					})()
				})`)
				if evalErr == nil {
					var st headedPageState
					_ = json.Unmarshal([]byte(stateRaw), &st)
					st.PageURL = mainPage.URL
					st.HasRecapIframe = hasRecapIframe

					if st.Launcher {
						authTracker.settledPolls = 0
						if !clickedLauncher {
							slog.Info("browser: launcher interstitial detected; clicking use web app")
							_, _ = evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, `(() => {
								const btn = document.querySelector('#openTeamsClientInBrowser, button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]');
								if (btn) { btn.click(); return true; }
								return false;
							})()`)
							clickedLauncher = true
						}
					} else {
						clickedLauncher = false
						if !m.opts.ForceRecap && (st.Toolbar || st.Lobby || st.Prejoin) {
							// Live meeting flow on the open page target; keep allocCtx
							// derived from the session-lifetime ctx so attachedCtx stays
							// alive after Join returns.
							tCtx, _ := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(mainPage.ID)))
							m.attachedCtx = tCtx
							return m.awaitPreJoinOrAdmissionWithDeadline(tCtx, deadline)
						}
						if acct := m.defaultAccount(); acct != "" {
							if clickedRaw, evalErr := evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, selectEntraAccountJS(acct)); evalErr == nil && strings.TrimSpace(clickedRaw) == "true" {
								slog.Info("browser: selected default Entra account", "account", redactAccountIdentifier(acct))
								authTracker.settledPolls = 0
								authTracker.rapidAuthBounces = 0
							}
						}
						if navURL := authTracker.onNonAuthPage(st); navURL != "" {
							slog.Info("browser: Teams session ready; navigating to target meeting URL", "attempt", authTracker.reNavigateAttempts)
							_ = navigateCDPTarget(ctx, mainPage.WebSocketDebuggerURL, navURL)
						} else if !loggedWaiting {
							slog.Info("browser: waiting for user authentication and meeting/recap load in browser window")
							loggedWaiting = true
						}
					}
				}
			}
		}

		select {
		case <-procDone:
			return fmt.Errorf("browser: browser process exited before meeting admission")
		case <-ctx.Done():
			return fmt.Errorf("browser: pre-join not visible: %w", ctx.Err())
		case <-ticker.C:
			if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
		}
	}
}

// findBoundCapturePage locates the page target bound to m.capturePageID. During
// cross-origin OAuth redirects (login.microsoftonline.com <-> teams.cloud.microsoft),
// Chromium COOP can swap the targetId of the single tab in a dedicated browser
// instance; when allowSingleTabRebind is enabled and exactly one Microsoft/auth
// page tab exists in the browser, m.capturePageID follows that single tab. If
// the bound tab disappears when multiple tabs or non-Microsoft tabs exist, it
// fails closed.
func (m *chromeMeeting) findBoundCapturePage(targets []cdpTargetInfo) (*cdpTargetInfo, bool) {
	if m.capturePageID == "" {
		return nil, false
	}
	var (
		candidates      []*cdpTargetInfo
		allOtherPageIDs []string
		boundTarget     *cdpTargetInfo
	)
	for i := range targets {
		t := &targets[i]
		if t.Type != "page" || strings.HasPrefix(t.URL, "chrome-extension://") {
			continue
		}
		if t.ID == m.capturePageID && t.WebSocketDebuggerURL != "" {
			boundTarget = t
			continue
		}
		if t.ID != "" {
			allOtherPageIDs = append(allOtherPageIDs, t.ID)
		}
		if t.WebSocketDebuggerURL == "" || strings.HasPrefix(t.URL, "edge://") {
			continue
		}
		candidates = append(candidates, t)
	}
	recordOtherIDs := func() {
		if len(allOtherPageIDs) == 0 {
			return
		}
		if m.otherPageIDs == nil {
			m.otherPageIDs = make(map[string]bool)
		}
		for _, id := range allOtherPageIDs {
			m.otherPageIDs[id] = true
		}
	}
	if boundTarget != nil {
		recordOtherIDs()
		return boundTarget, true
	}
	if len(allOtherPageIDs) > 1 || len(candidates) != 1 {
		recordOtherIDs()
		return nil, false
	}
	if m.allowSingleTabRebind &&
		!m.otherPageIDs[candidates[0].ID] &&
		isTrustedSingleTabRebindURL(candidates[0].URL) &&
		(candidates[0].ParentID == m.capturePageID ||
			hasPositiveMeetingIdentityMatch(m.opts.MeetingURL, candidates[0].URL) ||
			(candidates[0].ParentID == "" && m.opts.MeetingURL != "" && !hasMeetingIdentityConflict(ExtractMeetingIdentity(m.opts.MeetingURL), ExtractMeetingIdentity(candidates[0].URL)))) {
		m.capturePageID = candidates[0].ID
		return candidates[0], true
	}
	return nil, false
}

func isRecapIframeTarget(t cdpTargetInfo) bool {
	if t.Type != "iframe" {
		return false
	}
	return strings.Contains(t.URL, "xplatplugins.aspx") ||
		strings.Contains(t.URL, "streamembed.aspx") ||
		hasHTTPSHostSuffix(t.URL, "sharepoint.com")
}

func isTrustedSingleTabRebindURL(rawURL string) bool {
	if isAuthRedirectURL(rawURL) {
		return true
	}
	return hasHTTPSHostSuffix(rawURL,
		"teams.cloud.microsoft",
		"teams.microsoft.com",
		"teams.live.com",
		"sharepoint.com",
	)
}

func (m *chromeMeeting) awaitPreJoinOrAdmissionWithDeadline(ctx context.Context, deadline time.Time) error {
	ticker := time.NewTicker(m.effectivePollInterval())
	defer ticker.Stop()
	loggedWaiting := false
	clickedLauncher := false
	reNavigateAttempts := 0
	coldRecapPolls := 0
	recapNavAttempted := false

	for {
		if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
			return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
		}
		if m.cdpPort > 0 {
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			ok := m.tryExtractRecapViaCDP(pollCtx, m.cdpPort)
			cancel()
			if ok {
				return nil
			}
		}

		var (
			toolbar, lobby, prejoin, launcher bool
			err                               error
		)
		if m.probePreJoinState != nil {
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			toolbar, lobby, prejoin, launcher, err = m.probePreJoinState(pollCtx)
			cancel()
		} else {
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			err = chromedp.Run(pollCtx,
				chromedp.Evaluate(existsJS(meetingToolbarSel), &toolbar),
				chromedp.Evaluate(containsTextJS(lobbyText), &lobby),
				chromedp.Evaluate(existsJS(joinNowSel), &prejoin),
				chromedp.Evaluate(existsJS(joinOnWebSel), &launcher),
			)
			cancel()
		}
		if err != nil {
			if m.isLobbyDeadlineExceeded(ctx, deadline, err) {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
			return fmt.Errorf("browser: pre-join not visible: %w", err)
		}

		state := classifyPreJoinState(toolbar, lobby, prejoin, launcher)
		if m.opts.ForceRecap && (state == PreJoinAdmitted || state == PreJoinInLobby || state == PreJoinReady) {
			// In ForceRecap mode, never enter the live pre-join click flow or
			// treat a live call toolbar as Recap completion. If the meeting URL
			// opened live pre-join and carries a threadId, navigate to its
			// meetingrecap route once so Recap extraction can proceed.
			if !recapNavAttempted && m.probePreJoinState == nil {
				recapNavAttempted = true
				if recapURL := deriveRecapURLFromMeetingURL(m.opts.MeetingURL); recapURL != "" {
					navCtx, cancelNav := m.withDeadlineContext(ctx, deadline)
					_ = chromedp.Run(navCtx, chromedp.Navigate(recapURL))
					cancelNav()
				}
			}
			state = PreJoinWaitingForUser
		}

		switch state {
		case PreJoinAdmitted:
			slog.Info("browser: admitted to meeting")
			return nil
		case PreJoinInLobby:
			return m.awaitAdmissionWithDeadline(ctx, deadline)
		case PreJoinReady:
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			err := m.enforceMute(pollCtx)
			if err == nil {
				err = m.injectName(pollCtx)
			}
			if err == nil {
				if clickErr := chromedp.Run(pollCtx, chromedp.Click(joinNowSel, chromedp.ByQuery)); clickErr != nil {
					err = fmt.Errorf("browser: click join now: %w", clickErr)
				}
			}
			cancel()
			if err != nil {
				if m.isLobbyDeadlineExceeded(ctx, deadline, err) {
					return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
				}
				return err
			}
			return m.awaitAdmissionWithDeadline(ctx, deadline)
		case PreJoinLauncher:
			coldRecapPolls = 0
			if !clickedLauncher {
				slog.Info("browser: launcher interstitial detected; clicking join on web")
				var clicked bool
				pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
				err := chromedp.Run(pollCtx, chromedp.Evaluate(clickJS(joinOnWebSel), &clicked))
				cancel()
				if err != nil {
					if m.isLobbyDeadlineExceeded(ctx, deadline, err) {
						return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
					}
					slog.Warn("browser: click join on web failed; retrying", "err", err)
				} else if clicked {
					clickedLauncher = true
				}
			}
		case PreJoinWaitingForUser:
			clickedLauncher = false
			if m.probePreJoinState == nil {
				if acct := m.defaultAccount(); acct != "" {
					var clickedAccount bool
					pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
					err := chromedp.Run(pollCtx, chromedp.Evaluate(selectEntraAccountJS(acct), &clickedAccount))
					cancel()
					if err == nil && clickedAccount {
						slog.Info("browser: selected default Entra account", "account", redactAccountIdentifier(acct))
						coldRecapPolls = 0
					}
				}
				if reNavigateAttempts < 1 && hasSpecificMeetingOrRecapTarget(m.opts.MeetingURL) {
					var stalledOffTargetAfterAuth bool
					pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
					_ = chromedp.Run(pollCtx, chromedp.Evaluate(`(() => {
						try {
							if (!location.host.includes('teams.cloud.microsoft') && !location.host.includes('teams.microsoft.com')) return false;
							if (document.readyState && document.readyState !== 'complete') return false;
							const h = (location.hash || '') + '&' + (location.search || '');
							if (/(?:^|[#?&])(code|id_token|access_token|session_state|client_info|error)=/i.test(h)) return false;
							const route = (location.pathname + ' ' + location.hash).toLowerCase();
							if (route.includes('meetingrecap') || route.includes('meetup-join') || route.includes('/meet/')) return false;
							if (document.querySelector('[data-tid="intelligent-recap-header"]')) return false;
							if (document.querySelector('iframe[src*="xplatplugins.aspx"], iframe[src*="streamembed.aspx"], iframe[src*="sharepoint.com"]')) return false;
							const msal = Boolean(
								localStorage.getItem('msal.2.account.keys') ||
								localStorage.getItem('msal.account.keys') ||
								Object.keys(localStorage).some(k => k.includes('SKYPE-TOKEN') || k.includes('UserAuthState'))
							);
							return msal && document.querySelectorAll('[data-tid]').length > 15;
						} catch (_) {
							return false;
						}
					})()`, &stalledOffTargetAfterAuth))
					cancel()
					if stalledOffTargetAfterAuth {
						coldRecapPolls++
						if coldRecapPolls >= 5 {
							reNavigateAttempts++
							coldRecapPolls = 0
							slog.Info("browser: re-applying original meeting URL after authentication", "attempt", reNavigateAttempts)
							navCtx, cancelNav := m.withDeadlineContext(ctx, deadline)
							_ = chromedp.Run(navCtx, chromedp.Navigate(m.opts.MeetingURL))
							cancelNav()
						}
					} else {
						coldRecapPolls = 0
					}
				}
			}
			if !loggedWaiting {
				slog.Info("browser: waiting for user authentication and meeting entry in browser window")
				loggedWaiting = true
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: pre-join not visible: %w", ctx.Err())
		case <-ticker.C:
			if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
		}
	}
}

func (m *chromeMeeting) tryExtractRecapViaCDP(ctx context.Context, port int) bool {
	if m.capturePageID == "" {
		if c := chromedp.FromContext(ctx); c != nil && c.Target != nil && c.Target.TargetID != "" {
			m.capturePageID = string(c.Target.TargetID)
		}
	}
	targets, err := listCDPTargets(port)
	if err != nil {
		return false
	}
	var (
		ownedFrames ownedFrameSet
		mainPageURL string
	)
	if m.capturePageID != "" {
		mainPage, ok := m.findBoundCapturePage(targets)
		if !ok {
			return false
		}
		mainPageURL = mainPage.URL
		ownedFrames = fetchOwnedFrames(ctx, mainPage.WebSocketDebuggerURL)
	}
	for _, t := range targets {
		if !isTargetInCaptureScope(t, m.capturePageID, ownedFrames) {
			continue
		}
		if raw, evalErr := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, extractRecapTranscriptJS); evalErr == nil && raw != "" {
			if entries, meta, pErr := parseRecapPayloadForMeeting(raw, m.opts.MeetingURL, mainPageURL, t.URL); pErr == nil && len(entries) > 0 {
				m.recapEntries = entries
				if summary := captureAISummaryFromTargets(ctx, port, m.capturePageID); summary != "" {
					m.aiSummary = summary
					slog.Info("browser: copied AI summary from meeting recap")
				}
				enrichRecapMetadataFromPage(ctx, port, m.capturePageID, &meta)
				m.recapMeta = meta
				m.retainRecapAudioIdentity(mainPageURL)
				slog.Info("browser: extracted meeting recap transcript", "entries", len(entries), "title", meta.Title, "attendees", len(meta.Attendees))
				return true
			}
		}
	}
	if m.opts.ForceRecap && isOnTargetMeetingOrRecapURL(m.opts.MeetingURL, mainPageURL, "", "") {
		if summary := captureAISummaryFromTargets(ctx, port, m.capturePageID); summary != "" {
			m.aiSummary = summary
			m.retainRecapAudioIdentity(mainPageURL)
			slog.Info("browser: copied AI summary from meeting recap")
			return true
		}
	}
	return false
}

// enforceMute clicks the microphone toggle if it is currently enabled so the
// bot joins muted. It probes for the toggle first (a single, non-blocking
// evaluation) because chromedp query actions retry until the context expires
// when a selector matches nothing.
func (m *chromeMeeting) enforceMute(ctx context.Context) error {
	var present bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(micToggleSel), &present)); err != nil {
		return fmt.Errorf("browser: probe mic toggle: %w", err)
	}
	if !present {
		// The mic toggle may be absent in some pre-join variants; treat as
		// already muted rather than failing the join.
		slog.Warn("browser: mic toggle not found; assuming muted")
		return nil
	}
	var checked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(propBoolJS(micToggleSel, "checked"), &checked)); err != nil {
		return fmt.Errorf("browser: read mic checked property: %w", err)
	}
	var ariaChecked string
	var ok bool
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(micToggleSel, "aria-checked", &ariaChecked, &ok, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: read mic toggle state: %w", err)
	}
	if micStateRequiresMute(checked, ariaChecked) {
		if err := chromedp.Run(ctx, chromedp.Click(micToggleSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: mute microphone: %w", err)
		}
	}
	return nil
}

// injectName types the configured bot name into the display-name input when it
// is present and empty (unauthenticated guest flow).
func (m *chromeMeeting) injectName(ctx context.Context) error {
	if m.opts.BotName == "" {
		return nil
	}
	var present bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(displayNameInputSel), &present)); err != nil {
		return fmt.Errorf("browser: probe display-name input: %w", err)
	}
	if !present {
		return nil
	}
	var existing string
	if err := chromedp.Run(ctx, chromedp.Evaluate(propStringJS(displayNameInputSel, "value"), &existing)); err != nil {
		return fmt.Errorf("browser: read display-name input: %w", err)
	}
	if existing != "" {
		return nil
	}
	if err := chromedp.Run(ctx,
		chromedp.SendKeys(displayNameInputSel, m.opts.BotName, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: inject display name: %w", err)
	}
	return nil
}

// awaitAdmissionWithDeadline polls the DOM for the meeting toolbar (admitted)
// or the lobby prompt (waiting) until admitted or ctx/deadline expires.
func (m *chromeMeeting) awaitAdmissionWithDeadline(ctx context.Context, deadline time.Time) error {
	ticker := time.NewTicker(m.effectivePollInterval())
	defer ticker.Stop()
	loggedLobby := false
	for {
		if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
			return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
		}
		var (
			toolbar, lobby bool
			err            error
		)
		if m.probeAdmissionState != nil {
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			toolbar, lobby, err = m.probeAdmissionState(pollCtx)
			cancel()
		} else {
			pollCtx, cancel := m.withDeadlineContext(ctx, deadline)
			err = chromedp.Run(pollCtx,
				chromedp.Evaluate(existsJS(meetingToolbarSel), &toolbar),
				chromedp.Evaluate(containsTextJS(lobbyText), &lobby),
			)
			cancel()
		}
		if err != nil {
			if m.isLobbyDeadlineExceeded(ctx, deadline, err) {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
			return fmt.Errorf("browser: poll admission: %w", err)
		}
		switch classifyAdmission(toolbar, lobby) {
		case AdmissionAdmitted:
			slog.Info("browser: admitted to meeting")
			return nil
		case AdmissionInLobby:
			if !loggedLobby {
				slog.Info("browser: waiting in lobby for admission")
				loggedLobby = true
			}
		case AdmissionUnknown:
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: await admission: %w", ctx.Err())
		case <-ticker.C:
			if !deadline.IsZero() && !time.Now().Before(deadline) && ctx.Err() == nil {
				return fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)
			}
		}
	}
}

// payloadPump decouples a chromedp binding listener from a single processing
// worker goroutine. The listener enqueues raw payloads via offer; the worker
// applies process to each in arrival order. stop provides a full producer
// barrier: it rejects new payloads, waits for in-flight offers to finish, then
// drains every buffered payload to the worker before returning — so no payload
// is lost when the meeting is cancelled (e.g. on Ctrl+C).
type payloadPump struct {
	payloads chan string
	mu       sync.Mutex
	closing  bool
	prodWG   sync.WaitGroup
	workWG   sync.WaitGroup
}

// newPayloadPump starts a worker goroutine that applies process to each payload
// in arrival order until the pump is stopped.
func newPayloadPump(size int, process func(string)) *payloadPump {
	p := &payloadPump{payloads: make(chan string, size)}
	p.workWG.Add(1)
	go func() {
		defer p.workWG.Done()
		for raw := range p.payloads {
			process(raw)
		}
	}()
	return p
}

// offer enqueues raw for processing. It returns false when the pump is stopping
// or the queue is full, in which case the payload is dropped as backpressure.
func (p *payloadPump) offer(raw string) bool {
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return false
	}
	p.prodWG.Add(1)
	p.mu.Unlock()
	defer p.prodWG.Done()
	select {
	case p.payloads <- raw:
		return true
	default:
		return false
	}
}

// stop performs the producer barrier: it rejects further offers, waits for
// in-flight offers to complete, closes the queue, and waits for the worker to
// drain the remaining payloads and exit.
func (p *payloadPump) stop() {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
	p.prodWG.Wait()
	close(p.payloads)
	p.workWG.Wait()
}

// targetContext returns a context that carries the active chromedp target
// (using m.attachedCtx when Join attached to a headed CDP target) while
// propagating cancellation from callerCtx so Session.Run's runCtx can cancel
// EnableCaptions and CaptureAudio on write/flush/worker errors without closing
// the underlying browser target.
func (m *chromeMeeting) targetContext(callerCtx context.Context) (context.Context, context.CancelFunc) {
	if m.attachedCtx == nil {
		return context.WithCancel(callerCtx)
	}
	targetCtx, cancel := context.WithCancel(m.attachedCtx)
	stop := context.AfterFunc(callerCtx, cancel)
	return targetCtx, func() {
		stop()
		cancel()
	}
}

// EnableCaptions implements Meeting. It activates Teams live captions, registers
// the sendCaptionToGo CDP binding, injects the caption MutationObserver, and
// routes forwarded fragments through Go-side deduplication to sink. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) EnableCaptions(ctx context.Context, sink CaptionSink) error {
	if sink == nil {
		return fmt.Errorf("browser: caption sink must not be nil")
	}
	if len(m.recapEntries) > 0 {
		for _, entry := range m.recapEntries {
			speaker := entry.Speaker
			if entry.Offset != "" {
				speaker = speaker + RecapOffsetSeparator + entry.Offset
			}
			sink(speaker, entry.Text)
		}
		return nil
	}
	targetCtx, cancelTarget := m.targetContext(ctx)
	defer cancelTarget()
	if chromedp.FromContext(targetCtx) == nil {
		return fmt.Errorf("browser: enable captions requires a chromedp browser context")
	}
	if err := m.activateCaptions(targetCtx); err != nil {
		return err
	}
	if err := chromedp.Run(targetCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(captionBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add caption binding: %w", err)
	}

	// Decouple the caller's sink from chromedp's single event goroutine via a
	// payloadPump: the listener only enqueues, and a worker performs dedup and
	// sink delivery. pump.stop() is a producer barrier that drains all buffered
	// fragments before EnableCaptions returns, so the transcript tail is not
	// lost on shutdown and no sink call happens after return.
	workerCtx, cancel := context.WithCancel(targetCtx)
	dedup := transcript.NewDeduplicator()
	pump := newPayloadPump(captionQueueSize, func(raw string) {
		if err := handleCaptionPayloadWithActivity(raw, dedup, sink, m.notifyCaptionActivity); err != nil {
			slog.Warn("browser: discarding malformed caption payload", "err", err)
		}
	})
	defer func() {
		cancel()
		pump.stop()
	}()

	chromedp.ListenTarget(workerCtx, func(ev interface{}) {
		bc, ok := ev.(*cdpruntime.EventBindingCalled)
		if !ok || bc.Name != captionBinding {
			return
		}
		if workerCtx.Err() != nil {
			return
		}
		if !pump.offer(bc.Payload) {
			slog.Warn("browser: caption queue full; dropping fragment")
		}
	})

	var attached bool
	if err := chromedp.Run(targetCtx, chromedp.Evaluate(captionObserverJS, &attached)); err != nil {
		return fmt.Errorf("browser: inject caption observer: %w", err)
	}
	if !attached {
		slog.Warn("browser: caption renderer not yet present; observer not attached")
	}

	go m.pollCallEnded(workerCtx)

	<-targetCtx.Done()
	return nil
}

func (m *chromeMeeting) pollCallEnded(ctx context.Context) {
	interval := m.callEndPoll
	if interval <= 0 {
		interval = callEndedPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	missingToolbarPolls := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		var raw string
		if err := chromedp.Run(ctx, chromedp.Evaluate(callEndedProbeJS(), &raw)); err != nil {
			continue
		}
		if shouldNotifyCallEnded(raw, &missingToolbarPolls) {
			slog.Info("browser: detected in-browser call-end signal")
			m.notifyCallEnded()
			return
		}
	}
}

// activateCaptions clicks through the meeting menu to turn on live captions:
// "More actions" -> (optional "Language and speech" submenu in full client) ->
// "Turn on live captions", then waits for the caption renderer wrapper to
// appear. If captions are already active in the DOM, it returns immediately.
func (m *chromeMeeting) activateCaptions(ctx context.Context) error {
	var alreadyActive bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		slog.Info("browser: live captions already active")
		return nil
	}

	if err := waitAnyPresent(ctx, captionWrapperSel, moreActionsSel); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		return nil
	}

	if err := chromedp.Run(ctx, chromedp.Click(moreActionsSel, chromedp.ByQuery)); err != nil {
		return fmt.Errorf("browser: activate captions (click more): %w", err)
	}

	menuCtx, cancelMenu := context.WithTimeout(ctx, captionMenuStepWait)
	err := waitAnyPresent(menuCtx, captionWrapperSel, turnOnCaptionsSel, languageSpeechSel)
	cancelMenu()
	if err != nil {
		return fmt.Errorf("browser: activate captions (menu): %w", err)
	}

	var captionsBtn, langSubmenu bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive),
		chromedp.Evaluate(existsJS(turnOnCaptionsSel), &captionsBtn),
		chromedp.Evaluate(existsJS(languageSpeechSel), &langSubmenu),
	); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		return nil
	}

	if !captionsBtn && langSubmenu {
		if err := chromedp.Run(ctx, chromedp.Click(languageSpeechSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: activate captions (language submenu): %w", err)
		}
		subCtx, cancelSub := context.WithTimeout(ctx, captionMenuStepWait)
		err := waitAnyPresent(subCtx, captionWrapperSel, turnOnCaptionsSel)
		cancelSub()
		if err != nil {
			return fmt.Errorf("browser: activate captions (wait captions button): %w", err)
		}
	}

	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if !alreadyActive {
		if err := chromedp.Run(ctx, chromedp.Click(turnOnCaptionsSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: activate captions (click turn on captions): %w", err)
		}
	}

	return m.awaitCaptionWrapper(ctx)
}

func (m *chromeMeeting) awaitCaptionWrapper(ctx context.Context) error {
	ticker := time.NewTicker(captionMenuPoll)
	defer ticker.Stop()
	for {
		var wrapper, langConfirm bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(existsJS(captionWrapperSel), &wrapper),
			chromedp.Evaluate(existsJS(spokenLanguageConfirmSel), &langConfirm),
		); err != nil {
			return fmt.Errorf("browser: activate captions: %w", err)
		}
		if wrapper {
			slog.Info("browser: live captions active")
			return nil
		}
		if langConfirm {
			_ = chromedp.Run(ctx, chromedp.Click(spokenLanguageConfirmSel, chromedp.ByQuery))
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: activate captions: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitAnyPresent(ctx context.Context, selectors ...string) error {
	ticker := time.NewTicker(captionMenuPoll)
	defer ticker.Stop()
	for {
		for _, sel := range selectors {
			var ok bool
			if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(sel), &ok)); err != nil {
				return err
			}
			if ok {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// CaptureAudio implements Meeting. It registers the sendAudioToGo CDP binding,
// starts in-browser getDisplayMedia PCM capture, and encodes forwarded chunks to
// MP3 on w via a worker goroutine decoupled from the CDP event loop. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) CaptureAudio(ctx context.Context, w io.Writer) (err error) {
	if w == nil {
		return fmt.Errorf("browser: audio writer must not be nil")
	}
	if m.IsRecap() {
		meetingURL := m.opts.MeetingURL
		if m.recapAudioURL != "" {
			meetingURL = m.recapAudioURL
		}
		return captureRecapAudioFromTargets(ctx, m.cdpPort, m.capturePageID, meetingURL, w)
	}
	targetCtx, cancelTarget := m.targetContext(ctx)
	defer cancelTarget()
	if chromedp.FromContext(targetCtx) == nil {
		return fmt.Errorf("browser: capture audio requires a chromedp browser context")
	}
	if err := chromedp.Run(targetCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(audioBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add audio binding: %w", err)
	}

	// A payloadPump decouples the CDP event loop from MP3 encoding and provides a
	// producer barrier on stop, so buffered audio chunks are drained (not lost)
	// on shutdown. workerErr is written only by the pump's worker goroutine and
	// is read after pump.stop() joins it, so the access is race-free.
	workerCtx, cancel := context.WithCancel(targetCtx)
	var workerErr error
	enc := audio.NewEncoder(w, audio.SampleRate)
	pump := newPayloadPump(audioQueueSize, func(raw string) {
		if workerErr != nil {
			return // already failed fatally; skip remaining drained chunks
		}
		samples, decodeErr := decodeAudioPayload(raw)
		if decodeErr != nil {
			slog.Warn("browser: discarding malformed audio chunk", "err", decodeErr)
			return
		}
		if HasVoiceActivity(samples, m.opts.VoiceRMSThreshold) {
			m.notifyAudioActivity()
		}
		if writeErr := enc.WritePCM(samples); writeErr != nil {
			// A writer/encoder failure is fatal: record it and stop capture so
			// CaptureAudio can surface it to the caller.
			workerErr = writeErr
			cancel()
		}
	})
	defer func() {
		cancel()
		pump.stop()
		// Emit any buffered sub-frame remainder now that the worker has stopped.
		if workerErr == nil {
			if flushErr := enc.Flush(); flushErr != nil {
				workerErr = flushErr
			}
		}
		if err == nil {
			err = workerErr
		}
	}()

	chromedp.ListenTarget(workerCtx, func(ev interface{}) {
		bc, ok := ev.(*cdpruntime.EventBindingCalled)
		if !ok || bc.Name != audioBinding {
			return
		}
		if workerCtx.Err() != nil {
			return
		}
		if !pump.offer(bc.Payload) {
			slog.Warn("browser: audio queue full; dropping chunk")
		}
	})

	var started bool
	awaitPromise := func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}
	if err := chromedp.Run(targetCtx, chromedp.Evaluate(audioCaptureJS, &started, awaitPromise)); err != nil {
		return fmt.Errorf("browser: start audio capture: %w", err)
	}
	if !started {
		return fmt.Errorf("browser: audio capture did not start (no audio track)")
	}

	// Block until the caller's ctx is cancelled or a fatal worker error cancels
	// workerCtx; the deferred barrier drains and reaps the worker.
	<-workerCtx.Done()
	return nil
}
