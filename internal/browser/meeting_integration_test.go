//go:build integration

package browser_test

import (
	"bytes"
	"context"
	"os"
	"strings"
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

func newMeeting(t *testing.T) (browser.Meeting, context.Context) {
	t.Helper()
	url := meetingURL(t)
	profile := os.Getenv("TEAMS_PROFILE_DIR")
	if profile == "" {
		t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for live meeting tests")
	}
	m, bctx, cleanup, err := browser.NewLiveMeeting(
		context.Background(),
		browser.Options{
			Headless:       true,
			UserDataDir:    profile,
			ExecPath:       os.Getenv("TEAMS_BROWSER_PATH"),
			DefaultAccount: os.Getenv("TEAMS_ACCOUNT"),
		},
		browser.MeetingOptions{
			MeetingURL:     url,
			BotName:        "Transcription Agent",
			DefaultAccount: os.Getenv("TEAMS_ACCOUNT"),
			LobbyTimeout:   2 * time.Minute,
		},
	)
	require.NoError(t, err)
	t.Cleanup(cleanup)
	return m, bctx
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
	errCh := make(chan error, 1)
	go func() {
		errCh <- m.EnableCaptions(ctx, func(_, _ string) {
			select {
			case got <- struct{}{}:
			default:
			}
		})
	}()

	select {
	case <-got:
		// A finalized caption was delivered.
	case err := <-errCh:
		require.NoError(t, err, "EnableCaptions exited before delivering a caption")
		select {
		case <-got:
		default:
			t.Fatal("EnableCaptions returned before delivering a caption")
		}
	case <-ctx.Done():
		t.Fatal("no caption delivered before timeout")
	}
}

// TestMeetingCaptureAudio verifies tab audio is captured and MP3-encoded to the
// writer against a live meeting. CaptureAudio blocks until ctx is cancelled, so
// it runs in a goroutine; the test lets capture run briefly, cancels, waits for
// the teardown barrier, then asserts bytes were written.
func TestMeetingCaptureAudio(t *testing.T) {
	m, bctx := newMeeting(t)
	ctx, cancel := context.WithTimeout(bctx, 1*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	var buf bytes.Buffer
	errCh := make(chan error, 1)
	go func() { errCh <- m.CaptureAudio(ctx, &buf) }()

	if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() {
		select {
		case err := <-errCh:
			if err != nil && strings.Contains(err.Error(), "isDownloadBlocked=true") {
				t.Skip("recap recording has SharePoint download blocked (isDownloadBlocked=true); skipping raw audio download assertion")
			}
			require.NoError(t, err, "CaptureAudio failed for RecapMeeting")
			require.NotZero(t, buf.Len(), "expected captured MP3 bytes")
			return
		case <-ctx.Done():
			t.Fatal("CaptureAudio did not complete before timeout for RecapMeeting")
		}
	}

	select {
	case err := <-errCh:
		require.NoError(t, err, "CaptureAudio exited before capturing audio")
		t.Fatal("CaptureAudio returned before capturing audio")
	case <-time.After(10 * time.Second):
	}

	cancel()
	select {
	case err := <-errCh:
		require.NoError(t, err, "CaptureAudio should return cleanly after cancel")
	case <-time.After(30 * time.Second):
		t.Fatal("CaptureAudio did not return within 30s after cancel")
	}
	require.NotZero(t, buf.Len(), "expected captured MP3 bytes")
}
