package browser

import (
	"context"
	"io"
	"testing"
	"time"

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

func TestTargetContextPropagatesCallerCancellationWithAttachedCtx(t *testing.T) {
	allocCtx, cancelAlloc := context.WithCancel(context.Background())
	defer cancelAlloc()

	cm := newChromeMeeting(MeetingOptions{MeetingURL: "https://teams.microsoft.com/x"})
	cm.attachedCtx = allocCtx

	callerCtx, cancelCaller := context.WithCancel(context.Background())
	targetCtx, cancelTarget := cm.targetContext(callerCtx)
	defer cancelTarget()

	cancelCaller()
	select {
	case <-targetCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("targetCtx was not cancelled after callerCtx cancellation")
	}
	assert.ErrorIs(t, targetCtx.Err(), context.Canceled,
		"cancelling callerCtx must cancel targetCtx even when m.attachedCtx is still active")
	assert.NoError(t, allocCtx.Err(),
		"cancelling callerCtx must not cancel the underlying attachedCtx")
}
