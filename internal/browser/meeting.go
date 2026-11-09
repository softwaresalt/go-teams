package browser

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"strconv"

	"github.com/chromedp/chromedp"
)

// CaptionSink receives finalized caption utterances (speaker and text) as the
// in-meeting observer emits them. Implementations should return promptly; a
// slow sink causes fragments to be dropped under backpressure rather than
// blocking the browser event loop.
type CaptionSink func(speaker, text string)

// MeetingOptions configures a live meeting capture session.
type MeetingOptions struct {
	// MeetingURL is the mutated Teams meeting URL to join.
	MeetingURL string
	// BotName is the display name injected on the pre-join screen.
	BotName string
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
		cancelBrowser()
		cancelAlloc()
	}
	return cm, browserCtx, cleanup, nil
}

func cleanHeadlessAllocatorOptions(opts Options) []chromedp.ExecAllocatorOption {
	out := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	out = append(out, chromedp.Flag("headless", "new"))
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
