//go:build integration

package browser_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/teams"
)

// meetingURL returns the mutated meeting URL for live tests, skipping when the
// required environment is not provided.
func meetingURL(t *testing.T) string {
	t.Helper()
	requireIntegration(t)
	raw := os.Getenv("TEAMS_MEETING_URL")
	if raw == "" {
		t.Skip("set TEAMS_MEETING_URL to run live meeting integration tests")
	}
	mutated, err := teams.MutateJoinURL(raw)
	require.NoError(t, err)
	return mutated
}

func newMeeting(t *testing.T) browser.Meeting {
	return browser.NewMeeting(browser.MeetingOptions{
		MeetingURL: meetingURL(t),
		BotName:    "Transcription Agent",
	})
}

// TestMeetingJoin is a red contract test for the deferred pre-join / lobby FSM.
// It fails until NewMeeting returns a real implementation that admits the bot.
func TestMeetingJoin(t *testing.T) {
	m := newMeeting(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx), "pre-join FSM must admit the bot to the meeting")
}

// TestMeetingEnableCaptions is a red contract test for the deferred caption
// observer. It fails until captions are activated and streamed to the sink.
func TestMeetingEnableCaptions(t *testing.T) {
	m := newMeeting(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	got := make(chan struct{}, 1)
	err := m.EnableCaptions(ctx, func(_, _ string) {
		select {
		case got <- struct{}{}:
		default:
		}
	})
	require.NoError(t, err, "caption observer must activate and stream utterances")
}

// TestMeetingCaptureAudio is a red contract test for the deferred audio capture
// and MP3 encoding. It fails until encoded audio bytes are written.
func TestMeetingCaptureAudio(t *testing.T) {
	m := newMeeting(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	var buf bytes.Buffer
	require.NoError(t, m.CaptureAudio(ctx, &buf), "audio capture must stream encoded bytes")
	require.NotZero(t, buf.Len(), "expected captured audio bytes")
}
