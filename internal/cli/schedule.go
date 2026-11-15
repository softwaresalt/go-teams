package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/scheduler"
	"github.com/softwaresalt/go-teams/internal/teams"
)

// shutdownTimeout bounds how long the schedule daemon waits for in-flight
// captures (and their browser processes) to be reaped during graceful shutdown.
const (
	shutdownTimeout            = 15 * time.Second
	defaultScheduleDuration    = 1 * time.Hour
	defaultScheduleIdleTimeout = 3 * time.Minute
)

type scheduleFlags struct {
	browserFlags
	headless       bool
	headed         bool
	audio          bool
	mode           string
	duration       time.Duration
	idleTimeout    time.Duration
	aiSummaryDelay time.Duration
	noAISummary    bool
	lobbyTimeout   time.Duration
}

func bindScheduleFlags(cmd *cobra.Command, f *scheduleFlags) {
	bindBrowserFlags(cmd, &f.browserFlags)
	cmd.Flags().BoolVar(&f.headless, "headless", false, "run browser in headless mode")
	cmd.Flags().BoolVar(&f.headed, "headed", false, "open a visible browser window for interactive authentication")
	cmd.Flags().BoolVar(&f.audio, "audio", false, "also capture meeting or recap audio to an MP3 file")
	cmd.Flags().BoolVar(&f.audio, "recap-audio", false, "also capture Meeting Recap audio to an MP3 file")
	cmd.Flags().StringVar(&f.mode, "mode", "", "default meeting capture protocol: auto, ephemeral-live, live-with-summary, or past-recap")
	cmd.Flags().DurationVar(&f.duration, "duration", 0, "default nominal live capture duration (default 1h)")
	cmd.Flags().DurationVar(&f.idleTimeout, "idle-timeout", 0, "default post-duration inactivity grace period (default 3m)")
	cmd.Flags().DurationVar(&f.aiSummaryDelay, "ai-summary-delay", 0, "default delay after a live call ends before fetching the AI Summary (default 30m)")
	cmd.Flags().BoolVar(&f.noAISummary, "no-ai-summary", false, "disable delayed post-call AI Summary capture")
	cmd.Flags().DurationVar(&f.lobbyTimeout, "lobby-timeout", 0, "default maximum time to wait in pre-join or lobby before aborting")
}

func applyScheduleFlags(cmd *cobra.Command, cfg *config.Config, f scheduleFlags) {
	applyBrowserFlags(cmd, cfg, f.browserFlags)
	if cmd != nil && cmd.Flags().Changed("headless") {
		cfg.Headless = f.headless
	}
	if f.headed {
		cfg.Headless = false
	}
	audioChanged := cmd != nil && (cmd.Flags().Changed("audio") || cmd.Flags().Changed("recap-audio"))
	if audioChanged {
		cfg.CaptureAudio = f.audio
		cfg.ExplicitCaptureAudio = true
	}
	if (cmd != nil && cmd.Flags().Changed("mode")) || strings.TrimSpace(f.mode) != "" {
		cfg.CaptureMode = strings.ToLower(strings.TrimSpace(f.mode))
	}
	if (cmd != nil && cmd.Flags().Changed("duration")) || f.duration != 0 {
		cfg.Duration = f.duration.String()
	}
	if (cmd != nil && cmd.Flags().Changed("idle-timeout")) || f.idleTimeout != 0 {
		cfg.IdleTimeout = f.idleTimeout.String()
	}
	if (cmd != nil && cmd.Flags().Changed("ai-summary-delay")) || f.aiSummaryDelay != 0 {
		cfg.AISummaryDelay = f.aiSummaryDelay.String()
	}
	if f.noAISummary {
		cfg.AISummaryDelay = "0s"
	}
	if (cmd != nil && cmd.Flags().Changed("lobby-timeout")) || f.lobbyTimeout != 0 {
		cfg.LobbyTimeout = f.lobbyTimeout.String()
	}
	if strings.EqualFold(strings.TrimSpace(cfg.CaptureMode), config.CaptureModeEphemeralLive) {
		cfg.AISummaryDelay = "0s"
		if !audioChanged && !cfg.ExplicitCaptureAudio {
			cfg.CaptureAudio = true
		}
	}
}

func buildScheduleEntryConfig(base config.Config, e scheduler.Entry) config.Config {
	entryCfg := base
	if strings.TrimSpace(e.Name) != "" {
		entryCfg.MeetingTitle = strings.TrimSpace(e.Name)
	}
	if strings.TrimSpace(e.Category) != "" {
		entryCfg.Category = strings.TrimSpace(e.Category)
	}
	if strings.TrimSpace(e.OutputDir) != "" {
		entryCfg.OutputDir = strings.TrimSpace(e.OutputDir)
	}
	if e.CaptureAudio != nil {
		entryCfg.CaptureAudio = *e.CaptureAudio
		entryCfg.ExplicitCaptureAudio = true
	}
	if strings.TrimSpace(e.Mode) != "" {
		entryCfg.CaptureMode = strings.ToLower(strings.TrimSpace(e.Mode))
	}
	if e.Duration > 0 {
		entryCfg.Duration = e.Duration.String()
	} else if strings.TrimSpace(entryCfg.Duration) == "" {
		entryCfg.Duration = defaultScheduleDuration.String()
	}
	if e.IdleTimeout > 0 {
		entryCfg.IdleTimeout = e.IdleTimeout.String()
	} else if strings.TrimSpace(entryCfg.IdleTimeout) == "" {
		entryCfg.IdleTimeout = defaultScheduleIdleTimeout.String()
	}
	if e.AISummaryDelay != nil {
		entryCfg.AISummaryDelay = (*e.AISummaryDelay).String()
	}
	if e.LobbyTimeout > 0 {
		entryCfg.LobbyTimeout = e.LobbyTimeout.String()
	}
	if strings.EqualFold(strings.TrimSpace(entryCfg.CaptureMode), config.CaptureModeEphemeralLive) {
		entryCfg.AISummaryDelay = "0s"
		if e.CaptureAudio == nil && !entryCfg.ExplicitCaptureAudio {
			entryCfg.CaptureAudio = true
		}
	}
	return entryCfg
}

func buildSchedulerEntryFromConfig(m config.MeetingSchedule) (scheduler.Entry, error) {
	dur, err := parseOptionalDuration(m.Duration)
	if err != nil {
		return scheduler.Entry{}, fmt.Errorf("schedule: meeting %q duration: %w", m.Name, err)
	}
	idle, err := parseOptionalDuration(m.IdleTimeout)
	if err != nil {
		return scheduler.Entry{}, fmt.Errorf("schedule: meeting %q idle_timeout: %w", m.Name, err)
	}
	lobby, err := parseOptionalDuration(m.LobbyTimeout)
	if err != nil {
		return scheduler.Entry{}, fmt.Errorf("schedule: meeting %q lobby_timeout: %w", m.Name, err)
	}
	var aiDelay *time.Duration
	if strings.TrimSpace(m.AISummaryDelay) != "" {
		d, dErr := time.ParseDuration(strings.TrimSpace(m.AISummaryDelay))
		if dErr != nil {
			return scheduler.Entry{}, fmt.Errorf("schedule: meeting %q ai_summary_delay: %w", m.Name, dErr)
		}
		aiDelay = &d
	}
	captureAudio := m.CaptureAudio
	if captureAudio == nil && m.Audio != nil {
		val := *m.Audio
		captureAudio = &val
	}
	return scheduler.Entry{
		Name:           m.Name,
		URL:            m.URL,
		Cron:           m.Cron,
		At:             m.At,
		Mode:           m.Mode,
		Duration:       dur,
		IdleTimeout:    idle,
		AISummaryDelay: aiDelay,
		LobbyTimeout:   lobby,
		Category:       m.Category,
		CaptureAudio:   captureAudio,
		OutputDir:      m.OutputDir,
		ValidateURL:    teams.ValidateMeetingURL,
	}, nil
}

// scheduleJob builds the cron job that captures a single meeting. A shared,
// non-blocking busy gate serializes distinct entries on the single Chromium
// profile: because a capture holds the profile until the live call ends, a
// blocking mutex would deadlock the daemon's recurring roster, so an
// overlapping distinct entry is skipped (and logged) rather than launched
// concurrently.
func scheduleJob(ctx context.Context, d *deps, cfg config.Config, busy *atomic.Bool) scheduler.JobFunc {
	return scheduleJobWithSummaryWG(ctx, d, cfg, busy, nil)
}

func scheduleJobWithSummaryWG(ctx context.Context, d *deps, cfg config.Config, busy *atomic.Bool, summaryWG *sync.WaitGroup) scheduler.JobFunc {
	return func(e scheduler.Entry) {
		mutated, err := teams.MutateJoinURL(e.URL)
		if err != nil {
			slog.Error("schedule: invalid meeting url", "name", e.Name, "err", err)
			return
		}
		if !busy.CompareAndSwap(false, true) {
			slog.Warn("schedule: skipped, profile busy", "name", e.Name)
			return
		}
		var releaseOnce sync.Once
		releaseBusy := func() {
			releaseOnce.Do(func() {
				busy.Store(false)
			})
		}
		defer releaseBusy()

		entryCfg := buildScheduleEntryConfig(cfg, e)
		hooks := captureHooks{
			onLiveCallFinished: releaseBusy,
			acquireProfile: func() (func(), bool) {
				if !busy.CompareAndSwap(false, true) {
					return nil, false
				}
				return func() { busy.Store(false) }, true
			},
			scheduleAsyncSummary: func(delay time.Duration, fn func()) {
				if summaryWG != nil {
					summaryWG.Add(1)
				}
				go func() {
					if summaryWG != nil {
						defer summaryWG.Done()
					}
					timer := time.NewTimer(delay)
					defer timer.Stop()
					select {
					case <-ctx.Done():
						return
					case <-timer.C:
						fn()
					}
				}()
			},
		}
		if err := runCaptureWithHooks(ctx, d, entryCfg, mutated, hooks); err != nil {
			slog.Error("schedule: capture failed", "name", e.Name, "err", err)
		}
	}
}

func runDeferredAppendAISummarySweep(ctx context.Context, d *deps, cfg config.Config, configPath string, busy *atomic.Bool) {
	queuePath := resolveDeferredQueuePath(cfg)
	if queuePath == "" {
		return
	}
	if _, err := os.Stat(queuePath); err != nil {
		return
	}
	if !busy.CompareAndSwap(false, true) {
		return
	}
	defer busy.Store(false)
	var inherited []string
	if strings.TrimSpace(configPath) != "" {
		inherited = append(inherited, "--config", configPath)
	}
	if err := runQueueFile(ctx, d, queuePath, inherited); err != nil {
		slog.Warn("schedule: startup deferred queue sweep completed with errors", "queue", queuePath, "err", err)
	}
}

func newScheduleCommand(d *deps, configPath *string) *cobra.Command {
	var flags scheduleFlags
	cmd := &cobra.Command{
		Use:   "schedule",
		Short: "Run as a daemon, capturing configured meetings on their cron or one-shot schedules",
		Long: "schedule runs as a long-lived daemon and captures each configured " +
			"meeting (transcript and audio) when its cron or one-shot at trigger " +
			"fires. All meetings share a single browser profile, serialized by an " +
			"atomic busy gate that releases automatically when each live call and " +
			"browser session finish.\n\n" +
			"Meetings come from the meetings: list in config.yaml. Each entry requires a url and either a " +
			"5-field cron expression or a one-shot at timestamp (RFC3339 or local YYYY-MM-DD HH:MM); name is " +
			"an optional human-readable title fallback. " +
			"Past at timestamps are logged and skipped on startup.\n\n" +
			"Busy gate: while a capture holds the shared profile, an overlapping distinct entry is skipped " +
			"and logged rather than launched. The gate is released as soon as the live call ends, " +
			"before the delayed --ai-summary-delay AI Summary fetch, so the next meeting is not blocked.\n\n" +
			"Startup sweep: when queue_file is configured, the daemon scans it at startup and executes " +
			"the pending entries it finds, including deferred --append-ai-summary entries; entries whose " +
			"--not-before timestamp is still in the future are skipped and stay queued.\n\n" +
			"Precedence for each scheduled meeting (3 tiers, lowest to highest):\n" +
			"  1. Top-level settings in config.yaml form the baseline.\n" +
			"  2. Global schedule flags (--mode, --duration, --idle-timeout, --ai-summary-delay,\n" +
			"     --no-ai-summary, --lobby-timeout, --audio / --recap-audio, --headless, --headed,\n" +
			"     --browser-path, --profile-dir, --account) are applied to that base configuration\n" +
			"     first (applyScheduleFlags). If the base mode is then ephemeral-live, the base\n" +
			"     ai_summary_delay is forced to 0s and capture_audio defaults to true unless audio\n" +
			"     was set explicitly.\n" +
			"  3. When a meeting fires, its per-meeting settings (mode / capture_mode, duration,\n" +
			"     idle_timeout, ai_summary_delay, lobby_timeout, category, capture_audio / audio,\n" +
			"     output_dir) are applied on top of the flag-adjusted base\n" +
			"     (buildScheduleEntryConfig), followed by the same ephemeral-live normalization on\n" +
			"     the entry: when its effective mode is ephemeral-live, ai_summary_delay becomes 0s\n" +
			"     and capture_audio defaults to true unless audio was explicitly false (an entry\n" +
			"     capture_audio: true overrides an explicit false on the base).\n" +
			"So per-meeting settings in config.yaml override global schedule flags for that meeting, while\n" +
			"daemon-wide browser flags (--browser-path, --profile-dir, --account, --headless, --headed)\n" +
			"apply to every meeting. Because base normalization already ran, changing an entry's mode away\n" +
			"from an ephemeral-live base (for example to live-with-summary) does not restore an earlier\n" +
			"ai_summary_delay or capture_audio value. To override the normalized base, set\n" +
			"explicit per-meeting ai_summary_delay and capture_audio values on that entry.\n\n" +
			"Stop the daemon with Ctrl+C; in-flight captures get up to 15 seconds to release their browsers.",
		Example: "  teams-cli schedule --config config.yaml --headless\n" +
			"  teams-cli schedule --mode live-with-summary --duration 30m --ai-summary-delay 30m --profile-dir \"%LOCALAPPDATA%\\teams-cli\\profile\"\n" +
			"  teams-cli schedule --mode ephemeral-live --audio=false --headless",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			applyScheduleFlags(cmd, &cfg, flags)
			if err := validateCaptureLifecycles(cfg); err != nil {
				return err
			}
			entries := make([]scheduler.Entry, 0, len(cfg.Meetings))
			for _, m := range cfg.Meetings {
				entry, bErr := buildSchedulerEntryFromConfig(m)
				if bErr != nil {
					return bErr
				}
				entries = append(entries, entry)
			}

			var (
				busy      atomic.Bool
				summaryWG sync.WaitGroup
			)
			cronScheduler, err := scheduler.Build(entries,
				scheduleJobWithSummaryWG(cmd.Context(), d, cfg, &busy, &summaryWG),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
			if err != nil {
				return err
			}

			if resolveDeferredQueuePath(cfg) != "" {
				summaryWG.Add(1)
				go func() {
					defer summaryWG.Done()
					runDeferredAppendAISummarySweep(cmd.Context(), d, cfg, *configPath, &busy)
				}()
			}

			cronScheduler.Start()
			slog.Info("schedule: daemon started", "meetings", len(entries))
			<-cmd.Context().Done()

			// Stop dispatching and wait for in-flight captures to finish reaping
			// their browser processes, bounded by shutdownTimeout, to avoid
			// orphaned Chromium processes on shutdown.
			slog.Info("schedule: shutting down")
			stopCtx := cronScheduler.Stop()
			allDone := make(chan struct{})
			go func() {
				<-stopCtx.Done()
				summaryWG.Wait()
				close(allDone)
			}()
			select {
			case <-allDone:
			case <-time.After(shutdownTimeout):
				slog.Warn("schedule: shutdown timed out waiting for active captures")
			}
			return nil
		},
	}
	bindScheduleFlags(cmd, &flags)
	return cmd
}
