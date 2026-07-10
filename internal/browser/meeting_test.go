package browser

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCaptureAudioStub documents that audio capture still reports
// ErrNotImplemented pending its release unit.
func TestCaptureAudioStub(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	assert.ErrorIs(t, m.CaptureAudio(context.Background(), io.Discard), ErrNotImplemented)
}

// TestEnableCaptionsValidation covers the input guards that do not require a
// live browser: a nil sink and a non-chromedp context are both rejected.
func TestEnableCaptionsValidation(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})

	assert.Error(t, m.EnableCaptions(context.Background(), nil))
	assert.Error(t, m.EnableCaptions(context.Background(), func(string, string) {}))
}
