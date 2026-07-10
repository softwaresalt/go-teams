package browser

import (
	"context"
	"errors"
	"io"
)

// ErrNotImplemented marks capability stubs that are scaffolded with a defined
// contract but not yet implemented. The live-capture release units replace the
// stub returned by NewMeeting.
var ErrNotImplemented = errors.New("browser: not implemented")

// CaptionSink receives finalized caption utterances (speaker and text) as the
// in-meeting observer emits them.
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

// NewMeeting returns a chromedp-backed Meeting for opts. The pre-join / lobby
// FSM (Join) is implemented; EnableCaptions and CaptureAudio still return
// ErrNotImplemented until their release units are built. Meeting methods
// operate on a chromedp browser context supplied by the caller.
func NewMeeting(opts MeetingOptions) Meeting {
	return newChromeMeeting(opts)
}
