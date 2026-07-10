//go:build integration

package browser_test

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
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

// browserContext allocates a chromedp browser context using the persisted
// profile (TEAMS_PROFILE_DIR) so the bot session is already authenticated. The
// context is cancelled via t.Cleanup, reaping the browser process.
func browserContext(t *testing.T) context.Context {
	t.Helper()
	profile := os.Getenv("TEAMS_PROFILE_DIR")
	if profile == "" {
		t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for live meeting tests")
	}
	opts := browser.AllocatorOptions(browser.Options{
		Headless:    true,
		UserDataDir: profile,
	})
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	t.Cleanup(cancelAlloc)
	bctx, cancelBrowser := chromedp.NewContext(allocCtx)
	t.Cleanup(cancelBrowser)
	return bctx
}

func newMeeting(t *testing.T) (browser.Meeting, context.Context) {
	url := meetingURL(t)
	m := browser.NewMeeting(browser.MeetingOptions{
		MeetingURL: url,
		BotName:    "Transcription Agent",
	})
	return m, browserContext(t)
}

// TestMeetingJoin exercises the pre-join / lobby FSM against a live meeting.
func TestMeetingJoin(t *testing.T) {
	m, bctx := newMeeting(t)
	ctx, cancel := context.WithTimeout(bctx, 2*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx), "pre-join FSM must admit the bot to the meeting")
}

// TestMeetingEnableCaptions verifies captions are activated and streamed to the
// sink against a live meeting. EnableCaptions blocks until ctx is cancelled, so
// it runs in a goroutine and the test waits for the first delivered utterance.
func TestMeetingEnableCaptions(t *testing.T) {
	m, bctx := newMeeting(t)
	ctx, cancel := context.WithTimeout(bctx, 2*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	got := make(chan struct{}, 1)
	go func() {
		_ = m.EnableCaptions(ctx, func(_, _ string) {
			select {
			case got <- struct{}{}:
			default:
			}
		})
	}()

	select {
	case <-got:
		// A finalized caption was delivered.
	case <-ctx.Done():
		t.Fatal("no caption delivered before timeout")
	}
}

// TestMeetingCaptureAudio is a red contract test for the deferred audio capture
// and MP3 encoding. It fails until encoded audio bytes are written.
func TestMeetingCaptureAudio(t *testing.T) {
	m, bctx := newMeeting(t)
	ctx, cancel := context.WithTimeout(bctx, 1*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	var buf bytes.Buffer
	require.NoError(t, m.CaptureAudio(ctx, &buf), "audio capture must stream encoded bytes")
	require.NotZero(t, buf.Len(), "expected captured audio bytes")
}
