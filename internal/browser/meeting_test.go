package browser

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestNewMeetingScaffoldNotImplemented documents the current scaffold state:
// the Meeting returned by NewMeeting reports ErrNotImplemented until the
// live-capture release units replace it. The integration harness carries the
// real (currently failing) behavioral contract.
func TestNewMeetingScaffoldNotImplemented(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	ctx := context.Background()

	assert.ErrorIs(t, m.Join(ctx), ErrNotImplemented)
	assert.ErrorIs(t, m.EnableCaptions(ctx, func(string, string) {}), ErrNotImplemented)
	assert.ErrorIs(t, m.CaptureAudio(ctx, io.Discard), ErrNotImplemented)
}
