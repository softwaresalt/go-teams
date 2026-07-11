package browser

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestMeetingInputValidation covers the input guards on the Meeting methods that
// do not require a live browser: nil sink/writer and a non-chromedp context are
// all rejected.
func TestMeetingInputValidation(t *testing.T) {
	m := NewMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	bg := context.Background()

	assert.Error(t, m.EnableCaptions(bg, nil), "nil caption sink")
	assert.Error(t, m.EnableCaptions(bg, func(string, string) {}), "non-chromedp context")
	assert.Error(t, m.CaptureAudio(bg, nil), "nil audio writer")
	assert.Error(t, m.CaptureAudio(bg, io.Discard), "non-chromedp context")
}

// TestNewLiveMeeting verifies the live-meeting constructor wires a Meeting,
// context, and cleanup func without launching a browser (Chromium starts only
// on the first chromedp.Run). Cleanup must be safe to call regardless.
func TestNewLiveMeeting(t *testing.T) {
	m, ctx, cleanup, err := NewLiveMeeting(context.Background(),
		Options{Headless: true},
		MeetingOptions{MeetingURL: "https://teams.microsoft.com/x", BotName: "Agent"})
	assert.NoError(t, err)
	assert.NotNil(t, m)
	assert.NotNil(t, ctx)
	if assert.NotNil(t, cleanup) {
		cleanup()
	}
}
