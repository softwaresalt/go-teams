//go:build integration

package browser_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/teams"
)

const (
	liveNominalDuration = 15 * time.Second
	liveOvertimeWindow  = 15 * time.Second
)

// recapIntegrationURL resolves the live Meeting Recap URL and validates that
// TEAMS_PROFILE_DIR is workspace-contained and git-ignored without echoing
// sensitive values.
func recapIntegrationURL(t *testing.T) (string, string) {
	t.Helper()
	requireIntegration(t)

	profileDir := strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))
	if profileDir == "" {
		t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for live recap integration tests")
	}

	root, err := os.Getwd()
	require.NoError(t, err, "resolve working directory")
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		require.NotEqual(t, root, parent, "repository root with go.mod not found")
		root = parent
	}
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err, "resolve repository root")

	absProfile := profileDir
	if !filepath.IsAbs(absProfile) {
		absProfile = filepath.Join(realRoot, absProfile)
	}
	absProfile = filepath.Clean(absProfile)
	cur := absProfile
	var trailing []string
	for {
		if _, lerr := os.Lstat(cur); lerr == nil {
			break
		}
		parent := filepath.Dir(cur)
		require.NotEqual(t, cur, parent, "TEAMS_PROFILE_DIR ancestor not found")
		trailing = append([]string{filepath.Base(cur)}, trailing...)
		cur = parent
	}
	realCur, err := filepath.EvalSymlinks(cur)
	require.NoError(t, err, "evaluate TEAMS_PROFILE_DIR symlinks")
	realProfile := filepath.Join(append([]string{realCur}, trailing...)...)
	relRoot, relErr := filepath.Rel(realProfile, realRoot)
	require.False(t, relErr == nil && filepath.IsLocal(relRoot), "TEAMS_PROFILE_DIR must not contain repository root")
	relProfile, err := filepath.Rel(realRoot, realProfile)
	if err == nil && filepath.IsLocal(relProfile) {
		require.NotEqual(t, ".", relProfile, "TEAMS_PROFILE_DIR must not be the repository root")
		ignoreCmd := exec.Command("git", "check-ignore", "-q", "--", filepath.ToSlash(relProfile))
		ignoreCmd.Dir = realRoot
		require.NoError(t, ignoreCmd.Run(), "TEAMS_PROFILE_DIR must be ignored by git")
	}

	raw := strings.TrimSpace(os.Getenv("TEAMS_RECAP_URL"))
	if raw == "" {
		mURL := strings.TrimSpace(os.Getenv("TEAMS_MEETING_URL"))
		lower := strings.ToLower(mURL)
		if strings.Contains(lower, "meetingrecap") || strings.Contains(lower, "19:meeting_") || strings.Contains(lower, "19%3ameeting_") {
			raw = mURL
		} else {
			t.Skip("set TEAMS_RECAP_URL (or a TEAMS_MEETING_URL carrying threadId) to run live Recap AI Summary integration tests")
		}
	}

	mutated, err := teams.MutateJoinURL(raw)
	require.NoError(t, err, "invalid recap URL in test environment")
	return mutated, realProfile
}

// TestMeetingLiveScheduledOvertimeAndCallEnd exercises live-tenant meeting
// join, overtime caption or voice activity continuation past
// liveNominalDuration, and in-browser call-end teardown detection.
func TestMeetingLiveScheduledOvertimeAndCallEnd(t *testing.T) {
	requireIntegration(t)
	url := meetingURL(t)

	profile := strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))
	if profile == "" {
		t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for live meeting integration tests")
	}

	root, err := os.Getwd()
	require.NoError(t, err, "resolve working directory")
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		require.NotEqual(t, root, parent, "repository root with go.mod not found")
		root = parent
	}
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err, "resolve repository root")

	absProfile := profile
	if !filepath.IsAbs(absProfile) {
		absProfile = filepath.Join(realRoot, absProfile)
	}
	absProfile = filepath.Clean(absProfile)
	cur := absProfile
	var trailing []string
	for {
		if _, lerr := os.Lstat(cur); lerr == nil {
			break
		}
		parent := filepath.Dir(cur)
		require.NotEqual(t, cur, parent, "TEAMS_PROFILE_DIR ancestor not found")
		trailing = append([]string{filepath.Base(cur)}, trailing...)
		cur = parent
	}
	realCur, err := filepath.EvalSymlinks(cur)
	require.NoError(t, err, "evaluate TEAMS_PROFILE_DIR symlinks")
	realProfile := filepath.Join(append([]string{realCur}, trailing...)...)
	relRoot, relErr := filepath.Rel(realProfile, realRoot)
	require.False(t, relErr == nil && filepath.IsLocal(relRoot), "TEAMS_PROFILE_DIR must not contain repository root")
	relProfile, err := filepath.Rel(realRoot, realProfile)
	if err == nil && filepath.IsLocal(relProfile) {
		require.NotEqual(t, ".", relProfile, "TEAMS_PROFILE_DIR must not be the repository root")
		ignoreCmd := exec.Command("git", "check-ignore", "-q", "--", filepath.ToSlash(relProfile))
		ignoreCmd.Dir = realRoot
		require.NoError(t, ignoreCmd.Run(), "TEAMS_PROFILE_DIR must be ignored by git")
	}

	var (
		lastActivityUnixNano atomic.Int64
		callEnded            atomic.Bool
		callEndedCh          = make(chan struct{})
	)
	recordActivity := func() {
		lastActivityUnixNano.Store(time.Now().UnixNano())
	}

	m, bctx, cleanup, err := browser.NewLiveMeeting(
		context.Background(),
		browser.Options{
			Headless:       true,
			UserDataDir:    realProfile,
			ExecPath:       os.Getenv("TEAMS_BROWSER_PATH"),
			DefaultAccount: os.Getenv("TEAMS_ACCOUNT"),
		},
		browser.MeetingOptions{
			MeetingURL:        url,
			BotName:           "Transcription Agent",
			DefaultAccount:    os.Getenv("TEAMS_ACCOUNT"),
			LobbyTimeout:      2 * time.Minute,
			OnCaptionActivity: recordActivity,
			OnAudioActivity:   recordActivity,
			OnCallEnded: func() {
				if callEnded.CompareAndSwap(false, true) {
					close(callEndedCh)
				}
			},
		},
	)
	require.NoError(t, err)
	defer cleanup()

	ctx, cancel := context.WithTimeout(bctx, 3*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))
	nominalDeadline := time.Now().Add(liveNominalDuration)

	streamCtx, cancelStream := context.WithCancel(ctx)
	defer cancelStream()
	if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() {
		err := m.EnableCaptions(streamCtx, func(_, _ string) {
			recordActivity()
		})
		require.NoError(t, err, "EnableCaptions on RecapMeeting must deliver transcript utterances")
		require.NotZero(t, lastActivityUnixNano.Load(), "expected delivered recap transcript utterances")
		return
	}
	captionErrCh := make(chan error, 1)
	audioErrCh := make(chan error, 1)
	var audioBuf bytes.Buffer

	go func() {
		captionErrCh <- m.EnableCaptions(streamCtx, func(_, _ string) {
			recordActivity()
		})
	}()
	go func() {
		audioErrCh <- m.CaptureAudio(streamCtx, &audioBuf)
	}()

	select {
	case err := <-captionErrCh:
		cancelStream()
		<-audioErrCh
		require.NoError(t, err, "EnableCaptions exited early during live overtime test")
		t.Fatal("EnableCaptions returned before overtime window completed")
	case err := <-audioErrCh:
		cancelStream()
		<-captionErrCh
		require.NoError(t, err, "CaptureAudio exited early during live overtime test")
		t.Fatal("CaptureAudio returned before overtime window completed")
	case <-time.After(liveNominalDuration + liveOvertimeWindow):
	}

	select {
	case <-callEndedCh:
	default:
		// Best-effort in-browser hangup click; callEndedCh timeout below surfaces any failure to terminate.
		_ = chromedp.Run(streamCtx, chromedp.Evaluate(`(() => {
			const btn = document.querySelector('#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]');
			if (btn) { btn.click(); return true; }
			return false;
		})()`, nil))
	}

	var callEndedTimedOut bool
	select {
	case <-callEndedCh:
	case <-time.After(15 * time.Second):
		callEndedTimedOut = true
	}

	cancelStream()
	capErr := <-captionErrCh
	audErr := <-audioErrCh
	require.NoError(t, capErr)
	require.NoError(t, audErr)
	require.False(t, callEndedTimedOut, "timed out waiting for OnCallEnded callback after call termination")
	require.GreaterOrEqual(
		t,
		lastActivityUnixNano.Load(),
		nominalDeadline.UnixNano(),
		"expected live caption or voice activity past nominal duration during overtime window",
	)
}

// TestMeetingLiveRecapAISummaryEnrichment verifies ForceRecap and AISummaryOnly
// extraction against an authorized live Microsoft Teams Meeting Recap.
func TestMeetingLiveRecapAISummaryEnrichment(t *testing.T) {
	rURL, profile := recapIntegrationURL(t)

	m, bctx, cleanup, err := browser.NewLiveMeeting(
		context.Background(),
		browser.Options{
			UserDataDir:    profile,
			Headless:       true,
			ExecPath:       os.Getenv("TEAMS_BROWSER_PATH"),
			DefaultAccount: os.Getenv("TEAMS_ACCOUNT"),
		},
		browser.MeetingOptions{
			MeetingURL:     rURL,
			ForceRecap:     true,
			AISummaryOnly:  true,
			LobbyTimeout:   2 * time.Minute,
			DefaultAccount: os.Getenv("TEAMS_ACCOUNT"),
		},
	)
	require.NoError(t, err)
	defer cleanup()

	ctx, cancel := context.WithTimeout(bctx, 3*time.Minute)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	sp, ok := m.(browser.AISummaryProvider)
	require.True(t, ok)
	require.NotEmpty(t, strings.TrimSpace(sp.AISummary()), "expected non-empty AI summary from live Recap")
}
