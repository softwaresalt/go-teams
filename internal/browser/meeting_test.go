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
