package browser

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// ErrLobbyTimeout is returned when pre-join or lobby admission exceeds
// MeetingOptions.LobbyTimeout.
var ErrLobbyTimeout = errors.New("browser: lobby admission timed out")

// CaptionSink receives finalized caption utterances (speaker and text) as the
// in-meeting observer emits them. Implementations should return promptly; a
// slow sink causes fragments to be dropped under backpressure rather than
// blocking the browser event loop.
type CaptionSink func(speaker, text string)

// ActivityMonitor is an optional interface implemented by Meeting instances
// that report live caption activity, PCM voice audio activity, and in-browser
// call-end teardown signals.
type ActivityMonitor interface {
	SetActivityCallbacks(onCaption, onAudio, onCallEnd func())
}

// MeetingOptions configures a live meeting capture session.
type MeetingOptions struct {
	// MeetingURL is the mutated Teams meeting URL to join.
	MeetingURL string
	// BotName is the display name injected on the pre-join screen.
	BotName string
	// DefaultAccount is the optional Microsoft Entra account (UPN/email or
	// display name) to auto-select on the Entra account picker interstitial.
	DefaultAccount string
	// LobbyTimeout bounds how long Join waits in pre-join or the meeting lobby
	// before aborting with ErrLobbyTimeout. Zero means no lobby timeout.
	LobbyTimeout time.Duration
	// ForceRecap forces Join to use the Meeting Recap discovery and extraction
	// path and never enter the live pre-join click flow.
	ForceRecap bool
	// VoiceRMSThreshold overrides DefaultVoiceRMSThreshold when > 0.
	VoiceRMSThreshold float64
	// OnCaptionActivity is invoked when non-empty live caption DOM activity is
	// observed.
	OnCaptionActivity func()
	// OnAudioActivity is invoked when a captured PCM audio chunk exceeds the
	// voice-activity RMS threshold.
	OnAudioActivity func()
	// OnCallEnded is invoked when in-browser call-end teardown is detected.
	OnCallEnded func()
}

// Meeting drives in-meeting capture against a live Teams meeting: the pre-join
// finite state machine, lobby admission, caption activation and observation,
// and tab-audio capture. Implementations require a real browser and meeting and
// are therefore exercised by the integration test harness (build tag
// "integration").
type Meeting interface {
	// Join runs the pre-join FSM (enforce mute, inject the bot name, click
	// "Join now") and resolves the lobby, returning once admitted or when ctx
	// is cancelled.
	Join(ctx context.Context) error
	// EnableCaptions turns on live captions and starts the DOM observer,
	// delivering finalized, deduplicated utterances to sink until ctx is
	// cancelled.
	EnableCaptions(ctx context.Context, sink CaptionSink) error
	// CaptureAudio starts tab-audio capture and streams encoded MP3 bytes to w
	// until ctx is cancelled.
	CaptureAudio(ctx context.Context, w io.Writer) error
}

// NewMeeting returns a chromedp-backed Meeting for opts. Join, EnableCaptions,
// and CaptureAudio are all implemented; the methods operate on a chromedp
// browser context supplied by the caller.
func NewMeeting(opts MeetingOptions) Meeting {
	return newChromeMeeting(opts)
}

// NewLiveMeeting allocates a Chromium context for meeting capture and returns a
// chromedp-backed Meeting bound to it, the context that drives the meeting's
// methods, and a cleanup func that cancels the browser and allocator contexts
// (terminating the spawned Chromium process). Both headed and headless modes
// launch via a clean local CDP debugging port without stealth or
// sandbox-disabling flags so both Live Calls and Meeting Recaps work in
// headless mode once the profile is authenticated. The caller MUST invoke
// cleanup when the meeting is done, on success, failure, and cancellation.
func NewLiveMeeting(ctx context.Context, opts Options, mopts MeetingOptions) (Meeting, context.Context, func(), error) {
	if strings.TrimSpace(mopts.DefaultAccount) == "" && strings.TrimSpace(opts.DefaultAccount) != "" {
		mopts.DefaultAccount = strings.TrimSpace(opts.DefaultAccount)
	}
	cm := newChromeMeeting(mopts)
	if !opts.Headless {
		optsCopy := opts
		cm.headedOpts = &optsCopy
		browserCtx, cancelBrowser := context.WithCancel(ctx)
		cleanup := func() {
			cancelBrowser()
			cm.stopHeadedBrowser()
		}
		return cm, browserCtx, cleanup, nil
	}
	allocOpts := cleanHeadlessAllocatorOptions(opts)
	if ln, err := net.Listen("tcp", "127.0.0.1:0"); err == nil {
		port := ln.Addr().(*net.TCPAddr).Port
		_ = ln.Close()
		cm.cdpPort = port
		allocOpts = append(allocOpts, chromedp.Flag("remote-debugging-port", strconv.Itoa(port)))
	}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	cleanup := func() {
		launched := false
		if c := chromedp.FromContext(browserCtx); c != nil && c.Browser != nil {
			launched = true
		}
		cancelBrowser()
		cancelAlloc()
		if launched {
			time.Sleep(200 * time.Millisecond)
		}
	}
	return cm, browserCtx, cleanup, nil
}

// cleanHeadlessFlags returns the minimal flags for the clean headless
// allocator. The sandbox flags are emitted explicitly (false unless the
// operator opted in) so chromedp's root-user no-sandbox default is overridden.
func cleanHeadlessFlags(opts Options) []Flag {
	noSandbox := shouldDisableSandbox(opts)
	return []Flag{
		{Name: "no-sandbox", Value: noSandbox},
		{Name: "disable-setuid-sandbox", Value: noSandbox},
		{Name: "headless", Value: "new"},
		{Name: "use-fake-ui-for-media-stream", Value: true},
		{Name: "auto-select-desktop-capture-source", Value: "Microsoft Teams"},
		{Name: "mute-audio", Value: false},
	}
}

func cleanHeadlessAllocatorOptions(opts Options) []chromedp.ExecAllocatorOption {
	out := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	for _, f := range cleanHeadlessFlags(opts) {
		out = append(out, chromedp.Flag(f.Name, f.Value))
	}
	if opts.UserDataDir != "" {
		dir := opts.UserDataDir
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		out = append(out, chromedp.UserDataDir(dir))
	}
	if execPath := ResolveExecPath(opts.ExecPath); execPath != "" {
		out = append(out, chromedp.ExecPath(execPath))
	}
	return out
}
