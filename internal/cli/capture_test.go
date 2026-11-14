package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

// fakeSession is a test double for the capture session seam.
type fakeSession struct {
	runErr         error
	gotCtx         context.Context
	gotMeeting     browser.Meeting
	ran            chan struct{} // signalled once when Run is entered
	block          chan struct{} // if non-nil, Run blocks until it is closed
	once           sync.Once
	transcriptPath string
	wasRecap       bool
}

func (f *fakeSession) Run(ctx context.Context, m browser.Meeting) error {
	f.gotCtx = ctx
	f.gotMeeting = m
	if f.ran != nil {
		f.once.Do(func() { close(f.ran) })
	}
	if f.block != nil {
		<-f.block
	}
	return f.runErr
}

func (f *fakeSession) LastTranscriptPath() string { return f.transcriptPath }
func (f *fakeSession) WasRecap() bool             { return f.wasRecap }

type fakeSummaryMeeting struct {
	summary string
}

func (f *fakeSummaryMeeting) Join(context.Context) error                                { return nil }
func (f *fakeSummaryMeeting) EnableCaptions(context.Context, browser.CaptionSink) error { return nil }
func (f *fakeSummaryMeeting) CaptureAudio(context.Context, io.Writer) error             { return nil }
func (f *fakeSummaryMeeting) AISummary() string                                         { return f.summary }

func TestRunCaptureCleanupOnSuccess(t *testing.T) {
	var cleanupCalls int32
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	if err := runCapture(context.Background(), d, config.Config{}, "url"); err != nil {
		t.Fatalf("runCapture: %v", err)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup called %d times, want 1", got)
	}
}

func TestRunCaptureCleanupOnError(t *testing.T) {
	var cleanupCalls int32
	boom := errors.New("session boom")
	sess := &fakeSession{runErr: boom}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	err := runCapture(context.Background(), d, config.Config{}, "url")
	if !errors.Is(err, boom) {
		t.Fatalf("runCapture error = %v, want %v", err, boom)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup called %d times on error, want 1", got)
	}
}

func TestRunCaptureNewMeetingErrorNoCleanup(t *testing.T) {
	var cleanupCalls int32
	boom := errors.New("alloc boom")
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, ctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, boom
		},
		newSession: func(config.Config, string) meetingSession {
			t.Fatal("newSession must not be called when newMeeting fails")
			return nil
		},
	}
	err := runCapture(context.Background(), d, config.Config{}, "url")
	if !errors.Is(err, boom) {
		t.Fatalf("runCapture error = %v, want %v", err, boom)
	}
	if got := atomic.LoadInt32(&cleanupCalls); got != 0 {
		t.Fatalf("cleanup must not run on newMeeting failure, got %d", got)
	}
}

func TestRunCaptureCleanupOnCancellation(t *testing.T) {
	var cleanupCalls int32
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // caller cancellation (e.g. Ctrl+C) must still tear the browser down
	sess := &fakeSession{runErr: context.Canceled}
	d := &deps{
		newMeeting: func(mctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return nil, mctx, func() { atomic.AddInt32(&cleanupCalls, 1) }, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	_ = runCapture(ctx, d, config.Config{}, "url")
	if got := atomic.LoadInt32(&cleanupCalls); got != 1 {
		t.Fatalf("cleanup must run on cancellation, got %d", got)
	}
}

func TestRunCaptureLiveWithSummaryWritesQueueFallbackAndAppendsAfterDelay(t *testing.T) {
	cfg := config.Config{CaptureMode: config.CaptureModeLiveWithSummary}
	assert.Equal(t, 30*time.Minute, effectiveAISummaryDelay(cfg, false))
	assert.Equal(t, time.Duration(0), effectiveAISummaryDelay(cfg, true))
	assert.Equal(t, time.Duration(0), effectiveAISummaryDelay(config.Config{CaptureMode: config.CaptureModeEphemeralLive}, false))

	dir := t.TempDir()
	mdPath := filepath.Join(dir, "live-call.md")
	qPath := filepath.Join(dir, "transcription_queue.txt")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Meeting Transcript\n\n> **Alice:** Live utterance.\n"), 0o600))

	var (
		cleanupCalls int32
		meetingModes []string
	)
	sess := &fakeSession{
		transcriptPath: mdPath,
		wasRecap:       false,
	}
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.QueueFile = qPath
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			meetingModes = append(meetingModes, c.CaptureMode)
			return &fakeSummaryMeeting{summary: "Summary notes:\nKey decision reached."}, ctx, func() {
				atomic.AddInt32(&cleanupCalls, 1)
			}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}

	require.NoError(t, runCmd(context.Background(), d,
		"join", "https://teams.microsoft.com/meet/123",
		"--mode", "live-with-summary",
		"--duration", "45m",
		"--idle-timeout", "2m",
		"--lobby-timeout", "3m",
		"--account", "alice@contoso.com",
		"--ai-summary-delay", "20ms",
	))

	assert.Equal(t, int32(2), atomic.LoadInt32(&cleanupCalls), "both live session and delayed recap session must clean up their browser")
	assert.Equal(t, []string{"live-with-summary", "past-recap"}, meetingModes)

	mdBytes, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	assert.Contains(t, string(mdBytes), "Live utterance.")
	assert.Contains(t, string(mdBytes), "# AI Summary")
	assert.Contains(t, string(mdBytes), "Key decision reached.")

	qBytes, err := os.ReadFile(qPath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(string(qBytes)), "::"), "completed deferred queue line must be marked ::, got:\n%s", string(qBytes))
	assert.Contains(t, string(qBytes), "--profile-dir")
	assert.Contains(t, string(qBytes), "--account")

	// Also verify --mode ephemeral-live defaults CaptureAudio=true and AISummaryDelay=0s,
	// including when loaded from YAML config, and rejects negative CLI durations.
	var gotEphemeral config.Config
	dEph := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.CaptureMode = config.CaptureModeEphemeralLive
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			gotEphemeral = c
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return &fakeSession{} },
	}
	require.NoError(t, runCmd(context.Background(), dEph, "join", "https://teams.microsoft.com/meet/123"))
	assert.Equal(t, config.CaptureModeEphemeralLive, gotEphemeral.CaptureMode)
	assert.True(t, gotEphemeral.CaptureAudio)
	assert.Equal(t, "0s", gotEphemeral.AISummaryDelay)

	err = runCmd(context.Background(), dEph, "join", "https://teams.microsoft.com/meet/123", "--duration=-1m")
	require.ErrorIs(t, err, config.ErrInvalidDuration)
}
