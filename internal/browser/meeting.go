package browser

import (
	"context"
	"errors"
	"fmt"
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

// NewMeeting returns a Meeting for opts. The returned implementation is a
// scaffold whose methods return ErrNotImplemented; the live-capture release
// units provide the real chromedp-backed behavior. The signature is stable so
// the integration harness and callers can be written against it now.
func NewMeeting(opts MeetingOptions) Meeting {
	return &stubMeeting{opts: opts}
}

// stubMeeting is the not-yet-implemented Meeting scaffold.
type stubMeeting struct {
	opts MeetingOptions
}

func (s *stubMeeting) Join(context.Context) error {
	return fmt.Errorf("join meeting %q: %w", s.opts.MeetingURL, ErrNotImplemented)
}

func (s *stubMeeting) EnableCaptions(context.Context, CaptionSink) error {
	return fmt.Errorf("enable captions: %w", ErrNotImplemented)
}

func (s *stubMeeting) CaptureAudio(context.Context, io.Writer) error {
	return fmt.Errorf("capture audio: %w", ErrNotImplemented)
}
