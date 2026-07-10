package browser

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewMeetingCaptureStubs documents that the caption and audio capabilities
// still report ErrNotImplemented. Join is exercised by the integration harness
// because it drives a real browser.
func TestNewMeetingCaptureStubs(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	ctx := context.Background()

	assert.ErrorIs(t, m.EnableCaptions(ctx, func(string, string) {}), ErrNotImplemented)
	assert.ErrorIs(t, m.CaptureAudio(ctx, io.Discard), ErrNotImplemented)
}
