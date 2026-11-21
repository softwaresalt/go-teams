package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/scheduler"
)

func TestScheduleNoAISummaryAndPerMeetingDelayPrecedence(t *testing.T) {
	t.Run("explicit-no-ai-summary-true", func(t *testing.T) {
		var f scheduleFlags
		cmd := &cobra.Command{Use: "schedule"}
		bindScheduleFlags(cmd, &f)
		require.NoError(t, cmd.Flags().Parse([]string{"--no-ai-summary"}))
		cfg := config.Config{AISummaryDelay: "30m"}

		applyScheduleFlags(cmd, &cfg, f)
		assert.True(t, cfg.NoAISummary)
		assert.Equal(t, "0s", cfg.AISummaryDelay)
	})

	t.Run("explicit-no-ai-summary-false", func(t *testing.T) {
		var f scheduleFlags
		cmd := &cobra.Command{Use: "schedule"}
		bindScheduleFlags(cmd, &f)
		require.NoError(t, cmd.Flags().Parse([]string{"--no-ai-summary=false"}))
		cfg := config.Config{AISummaryDelay: "30m", NoAISummary: true}

		applyScheduleFlags(cmd, &cfg, f)
		assert.False(t, cfg.NoAISummary)
		assert.Equal(t, "30m", cfg.AISummaryDelay)
	})

	t.Run("positive-per-meeting-delay", func(t *testing.T) {
		delay := 5 * time.Minute
		base := config.Config{AISummaryDelay: "0s", NoAISummary: true}

		got := buildScheduleEntryConfig(base, scheduler.Entry{
			Mode:           config.CaptureModePastRecap,
			AISummaryDelay: &delay,
		})
		assert.False(t, got.NoAISummary)
		assert.Equal(t, "5m0s", got.AISummaryDelay)
	})

	for _, tc := range []struct {
		name  string
		delay time.Duration
	}{
		{name: "zero-delay-preserves-inherited-disable", delay: 0},
		{name: "negative-delay-preserves-inherited-disable", delay: -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := config.Config{
				CaptureMode:    config.CaptureModePastRecap,
				NoAISummary:    true,
				AISummaryDelay: "0s",
			}

			got := buildScheduleEntryConfig(base, scheduler.Entry{
				Mode:           config.CaptureModePastRecap,
				AISummaryDelay: &tc.delay,
			})
			assert.True(t, got.NoAISummary)
			assert.Equal(t, tc.delay.String(), got.AISummaryDelay)
		})
	}

	t.Run("schedule-help", func(t *testing.T) {
		var f scheduleFlags
		cmd := &cobra.Command{Use: "schedule"}

		bindScheduleFlags(cmd, &f)
		flag := cmd.Flags().Lookup("no-ai-summary")
		require.NotNil(t, flag)
		assert.Equal(t, "disable AI Summary capture for both Meeting Recaps and post-call enrichment", flag.Usage)
	})
}

func TestScheduleJobRunsCapture(t *testing.T) {
	var meetingCalls int32
	var gotURL string
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			atomic.AddInt32(&meetingCalls, 1)
			gotURL = url
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)
	job(scheduler.Entry{Name: "m1", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc"})

	if got := atomic.LoadInt32(&meetingCalls); got != 1 {
		t.Fatalf("expected one capture, got %d", got)
	}
	if gotURL == "" {
		t.Fatal("meeting url was not passed to capture")
	}
	if busy.Load() {
		t.Fatal("busy flag should be released after the job returns")
	}
}

func TestScheduleJobBusySkipsOverlappingEntry(t *testing.T) {
	var meetingCalls int32
	ran := make(chan struct{})
	block := make(chan struct{})
	sess := &fakeSession{ran: ran, block: block}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			atomic.AddInt32(&meetingCalls, 1)
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)

	go job(scheduler.Entry{Name: "m1", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_one"})
	<-ran // first capture is active and holds the shared profile

	// A distinct entry firing while a capture is active must be skipped, not
	// launched concurrently against the shared profile.
	job(scheduler.Entry{Name: "m2", URL: "https://teams.microsoft.com/l/meetup-join/19%3ameeting_two"})
	if got := atomic.LoadInt32(&meetingCalls); got != 1 {
		t.Fatalf("overlapping distinct entry should be skipped; launched %d times, want 1", got)
	}

	close(block) // let the first capture finish

	deadline := time.After(2 * time.Second)
	for busy.Load() {
		select {
		case <-deadline:
			t.Fatal("busy flag not released after the active capture finished")
		default:
			time.Sleep(time.Millisecond)
		}
	}
}

func TestScheduleJobInvalidURLNotBusy(t *testing.T) {
	sess := &fakeSession{}
	d := &deps{
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			t.Fatal("newMeeting must not be called for an invalid url")
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return sess },
	}
	var busy atomic.Bool
	job := scheduleJob(context.Background(), d, config.Config{}, &busy)
	job(scheduler.Entry{Name: "bad", URL: "ftp://example.com"})
	if busy.Load() {
		t.Fatal("busy must not be held after an invalid url is rejected")
	}
}

func TestScheduleMeetingOverridesAndBusyGateReleaseBeforeSummaryDelay(t *testing.T) {
	root := newRootCommand(defaultDeps())
	sched, _, err := root.Find([]string{"schedule"})
	require.NoError(t, err)
	for _, flag := range []string{"duration", "idle-timeout", "mode", "ai-summary-delay", "no-ai-summary", "lobby-timeout", "audio", "headless", "headed"} {
		require.NotNilf(t, sched.Flags().Lookup(flag), "schedule command missing --%s flag", flag)
	}

	// Verify buildScheduleEntryConfig applies defaults (1h duration, 3m idle timeout) and per-entry overrides.
	base := validConfig()
	defCfg := buildScheduleEntryConfig(base, scheduler.Entry{Name: "Daily Standup", URL: "https://teams.microsoft.com/meet/1"})
	assert.Equal(t, "Daily Standup", defCfg.MeetingTitle)
	assert.Equal(t, "1h0m0s", defCfg.Duration)
	assert.Equal(t, "3m0s", defCfg.IdleTimeout)

	audioTrue := true
	delay5m := 5 * time.Minute
	overCfg := buildScheduleEntryConfig(base, scheduler.Entry{
		Name:           "Exec Sync",
		URL:            "https://teams.microsoft.com/meet/2",
		Mode:           config.CaptureModeLiveWithSummary,
		Duration:       45 * time.Minute,
		IdleTimeout:    2 * time.Minute,
		AISummaryDelay: &delay5m,
		LobbyTimeout:   4 * time.Minute,
		Category:       "EXEC",
		CaptureAudio:   &audioTrue,
		OutputDir:      "custom-dir",
	})
	assert.Equal(t, "Exec Sync", overCfg.MeetingTitle)
	assert.Equal(t, config.CaptureModeLiveWithSummary, overCfg.CaptureMode)
	assert.Equal(t, "45m0s", overCfg.Duration)
	assert.Equal(t, "2m0s", overCfg.IdleTimeout)
	assert.Equal(t, "5m0s", overCfg.AISummaryDelay)
	assert.Equal(t, "4m0s", overCfg.LobbyTimeout)
	assert.Equal(t, "EXEC", overCfg.Category)
	assert.True(t, overCfg.CaptureAudio)
	assert.Equal(t, "custom-dir", overCfg.OutputDir)

	// Verify busy gate releases immediately after live capture finishes (before ai_summary_delay)
	// so a second scheduled meeting can run during the delay window.
	dir := t.TempDir()
	base.QueueFile = filepath.Join(dir, "transcription_queue.txt")
	md1 := filepath.Join(dir, "m1.md")
	require.NoError(t, os.WriteFile(md1, []byte("# Meeting Transcript\n\n> **Alice:** First meeting.\n"), 0o600))

	var (
		mu       sync.Mutex
		captured []string
	)
	d := &deps{
		newMeeting: func(ctx context.Context, c config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			mu.Lock()
			captured = append(captured, c.MeetingTitle+":"+c.CaptureMode)
			mu.Unlock()
			return &fakeSummaryMeeting{summary: "Summary:\nM1 summary."}, ctx, func() {}, nil
		},
		newSession: func(c config.Config, _ string) meetingSession {
			if c.MeetingTitle == "m1" {
				return &fakeSession{transcriptPath: md1, wasRecap: false}
			}
			return &fakeSession{wasRecap: false}
		},
	}

	var (
		busy      atomic.Bool
		summaryWG sync.WaitGroup
	)
	job := scheduleJobWithSummaryWG(context.Background(), d, base, &busy, &summaryWG)

	delay80ms := 80 * time.Millisecond
	job(scheduler.Entry{
		Name:           "m1",
		URL:            "https://teams.microsoft.com/meet/one",
		Mode:           config.CaptureModeLiveWithSummary,
		AISummaryDelay: &delay80ms,
	})
	// Immediately after job(m1) returns, busy must be false even while the 80ms AI summary timer is pending!
	assert.False(t, busy.Load(), "busy gate must be released before ai_summary_delay elapses")

	// A second meeting m2 triggered during m1's delay window must succeed rather than being skipped.
	job(scheduler.Entry{
		Name: "m2",
		URL:  "https://teams.microsoft.com/meet/two",
		Mode: config.CaptureModeEphemeralLive,
	})

	summaryWG.Wait()

	mu.Lock()
	gotCaptured := append([]string(nil), captured...)
	mu.Unlock()
	assert.Equal(t, []string{
		"m1:live-with-summary",
		"m2:ephemeral-live",
		"m1:past-recap",
	}, gotCaptured)
}
