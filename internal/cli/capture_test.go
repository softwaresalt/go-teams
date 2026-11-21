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

func TestRunCmdLiveWithSummaryWritesQueueFallbackAndAppendsAfterDelay(t *testing.T) {
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
}

func TestRunCmdEphemeralLiveDefaultsAudioAndZeroDelay(t *testing.T) {
	var gotEphemeral config.Config
	d := &deps{
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
	require.NoError(t, runCmd(context.Background(), d, "join", "https://teams.microsoft.com/meet/123"))
	assert.Equal(t, config.CaptureModeEphemeralLive, gotEphemeral.CaptureMode)
	assert.True(t, gotEphemeral.CaptureAudio)
	assert.Equal(t, "0s", gotEphemeral.AISummaryDelay)
}

func TestBuildMeetingOptionsAISummaryFlags(t *testing.T) {
	const meetingURL = "https://teams.cloud.microsoft/l/meetingrecap?threadId=1"

	t.Run("zero-delay-does-not-disable-recap", func(t *testing.T) {
		opts := buildMeetingOptions(config.Config{
			CaptureMode:    config.CaptureModePastRecap,
			AISummaryDelay: "0s",
		}, meetingURL)
		assert.True(t, opts.ForceRecap)
		assert.False(t, opts.AISummaryOnly)
		assert.False(t, opts.DisableAISummary)
	})

	t.Run("no-ai-summary-disables-recap", func(t *testing.T) {
		opts := buildMeetingOptions(config.Config{
			CaptureMode:    config.CaptureModePastRecap,
			AISummaryDelay: "0s",
			NoAISummary:    true,
		}, meetingURL)
		assert.True(t, opts.ForceRecap)
		assert.False(t, opts.AISummaryOnly)
		assert.True(t, opts.DisableAISummary)
	})

	t.Run("append-always-enables-recap-summary", func(t *testing.T) {
		opts := buildMeetingOptions(config.Config{
			CaptureMode:     config.CaptureModePastRecap,
			AppendAISummary: "C:\\out\\call.md",
			AISummaryDelay:  "0s",
			NoAISummary:     true,
		}, meetingURL)
		assert.True(t, opts.ForceRecap)
		assert.True(t, opts.AISummaryOnly)
		assert.False(t, opts.DisableAISummary)
	})
}

func TestIsAISummaryDisabledAcrossModes(t *testing.T) {
	const meetingURL = "https://teams.cloud.microsoft/l/meetingrecap?threadId=1"

	tests := []struct {
		name     string
		cfg      config.Config
		disabled bool
	}{
		{"past-recap-zero-delay", config.Config{CaptureMode: config.CaptureModePastRecap, AISummaryDelay: "0s"}, false},
		{"auto-zero-delay", config.Config{CaptureMode: config.CaptureModeAuto, AISummaryDelay: "0s"}, false},
		{"live-with-summary-zero-delay", config.Config{CaptureMode: config.CaptureModeLiveWithSummary, AISummaryDelay: "0s"}, false},
		{"ephemeral-live", config.Config{CaptureMode: config.CaptureModeEphemeralLive}, true},
		{"explicit-no-ai", config.Config{CaptureMode: config.CaptureModeAuto, NoAISummary: true}, true},
		{"append-overrides-no-ai", config.Config{CaptureMode: config.CaptureModeEphemeralLive, AppendAISummary: "call.md", NoAISummary: true}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.disabled, isAISummaryDisabled(tc.cfg))
			assert.Equal(t, tc.disabled, buildMeetingOptions(tc.cfg, meetingURL).DisableAISummary)
		})
	}
}

func TestEffectiveAISummaryDelayAcrossModes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		isRecap bool
		want    time.Duration
	}{
		{"live-with-summary-default", config.Config{CaptureMode: config.CaptureModeLiveWithSummary}, false, 30 * time.Minute},
		{"recap", config.Config{CaptureMode: config.CaptureModePastRecap}, true, 0},
		{"no-ai-empty-delay", config.Config{CaptureMode: config.CaptureModeAuto, NoAISummary: true}, false, 0},
		{"no-ai-positive-delay", config.Config{CaptureMode: config.CaptureModeLiveWithSummary, AISummaryDelay: "15m", NoAISummary: true}, false, 0},
		{"ephemeral-live", config.Config{CaptureMode: config.CaptureModeEphemeralLive}, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, effectiveAISummaryDelay(tc.cfg, tc.isRecap))
		})
	}
}

func TestRunAppendAISummaryResetsNoAISummary(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "existing.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Meeting Transcript\n\n> **Alice:** Hi.\n"), 0o600))
	var gotAppendCfg config.Config
	d := &deps{
		newMeeting: func(ctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			gotAppendCfg = c
			return &fakeSummaryMeeting{summary: "Meeting notes:\nAppended item."}, ctx, func() {}, nil
		},
	}
	err := runAppendAISummary(context.Background(), d, config.Config{
		AISummaryDelay: "15m",
		NoAISummary:    true,
	}, "https://teams.cloud.microsoft/l/meetingrecap?threadId=1", mdPath)
	require.NoError(t, err)
	assert.Equal(t, mdPath, gotAppendCfg.AppendAISummary)
	assert.Empty(t, gotAppendCfg.AISummaryDelay)
	assert.False(t, gotAppendCfg.NoAISummary)
	assert.True(t, buildMeetingOptions(gotAppendCfg, "https://teams.cloud.microsoft/l/meetingrecap?threadId=1").AISummaryOnly)
}

func TestRunCmdSuppressesDeferredAISummaryTimerAndQueue(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		appendSummary bool
		wantMode      string
		wantDisabled  bool
		wantNoAI      bool
	}{
		{"past-recap-zero-delay", []string{"--mode", "past-recap", "--ai-summary-delay", "0s"}, false, config.CaptureModePastRecap, false, false},
		{"auto-zero-delay", []string{"--mode", "auto", "--ai-summary-delay", "0s"}, false, config.CaptureModeAuto, false, false},
		{"live-with-summary-zero-delay", []string{"--mode", "live-with-summary", "--ai-summary-delay", "0s"}, false, config.CaptureModeLiveWithSummary, false, false},
		{"ephemeral-live", []string{"--mode", "ephemeral-live"}, false, config.CaptureModeEphemeralLive, true, false},
		{"explicit-no-ai", []string{"--mode", "live-with-summary", "--ai-summary-delay", "15m", "--no-ai-summary"}, false, config.CaptureModeLiveWithSummary, true, true},
		{"append-resets-no-ai", []string{"--mode", "live-with-summary", "--no-ai-summary"}, true, config.CaptureModePastRecap, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			mdPath := filepath.Join(dir, "existing.md")
			qPath := filepath.Join(dir, "transcription_queue.txt")
			require.NoError(t, os.WriteFile(mdPath, []byte("# Meeting Transcript\n"), 0o600))

			var (
				gotCfg   config.Config
				gotOpts  browser.MeetingOptions
				meetings int
			)
			d := &deps{
				loadConfig: func(string) (config.Config, error) {
					cfg := validConfig()
					cfg.QueueFile = qPath
					return cfg, nil
				},
				newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
				newMeeting: func(ctx context.Context, cfg config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error) {
					meetings++
					gotCfg = cfg
					gotOpts = buildMeetingOptions(cfg, meetingURL)
					return &fakeSummaryMeeting{summary: "Summary notes:\nEnd-to-end item."}, ctx, func() {}, nil
				},
				newSession: func(config.Config, string) meetingSession {
					return &fakeSession{transcriptPath: mdPath}
				},
			}

			args := append([]string{"join", "https://teams.microsoft.com/meet/123"}, tc.args...)
			if tc.appendSummary {
				args = append(args, "--append-ai-summary", mdPath)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- runCmd(ctx, d, args...)
			}()
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(time.Second):
				cancel()
				select {
				case err := <-done:
					t.Fatalf("runCmd waited on a deferred AI summary timer; cancellation returned %v", err)
				case <-time.After(time.Second):
					t.Fatal("runCmd did not return after deferred AI summary cancellation")
				}
			}

			require.Equal(t, 1, meetings)
			assert.Equal(t, tc.wantMode, gotCfg.CaptureMode)
			assert.Equal(t, tc.wantDisabled, gotOpts.DisableAISummary)
			assert.Equal(t, tc.wantNoAI, gotCfg.NoAISummary)
			_, err := os.Stat(qPath)
			require.ErrorIs(t, err, os.ErrNotExist, "suppressed AI summary must not enqueue fallback work")
		})
	}
}

func TestRunCmdRejectsInvalidAISummaryDelay(t *testing.T) {
	tests := []struct {
		name         string
		loadedDelay  string
		args         []string
		wantSentinel bool
	}{
		{"loaded-malformed", "not-a-duration", nil, true},
		{"loaded-negative", "-1s", nil, true},
		{"cli-malformed", "", []string{"--ai-summary-delay", "not-a-duration"}, false},
		{"cli-negative", "", []string{"--ai-summary-delay=-1s"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := &deps{
				loadConfig: func(string) (config.Config, error) {
					cfg := validConfig()
					cfg.AISummaryDelay = tc.loadedDelay
					return cfg, nil
				},
				newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
				newMeeting: func(context.Context, config.Config, string) (browser.Meeting, context.Context, func(), error) {
					t.Fatal("invalid delay must fail before meeting creation")
					return nil, context.Background(), func() {}, nil
				},
				newSession: func(config.Config, string) meetingSession { return &fakeSession{} },
			}

			args := append([]string{"join", "https://teams.microsoft.com/meet/123"}, tc.args...)
			err := runCmd(context.Background(), d, args...)
			require.Error(t, err)
			assert.Contains(t, strings.ReplaceAll(err.Error(), "_", "-"), "ai-summary-delay")
			if tc.wantSentinel {
				require.ErrorIs(t, err, config.ErrInvalidDuration)
			}
		})
	}
}
