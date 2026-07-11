package cli

import (
	"context"
	"log/slog"
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
const shutdownTimeout = 15 * time.Second

// scheduleJob builds the cron job that captures a single meeting. A shared,
// non-blocking busy gate serializes distinct entries on the single Chromium
// profile: because a capture holds the profile until ctx ends, a blocking mutex
// would deadlock the daemon's recurring roster, so an overlapping distinct entry
// is skipped (and logged) rather than launched concurrently.
func scheduleJob(ctx context.Context, d *deps, cfg config.Config, busy *atomic.Bool) scheduler.JobFunc {
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
		defer busy.Store(false)
		if err := runCapture(ctx, d, cfg, mutated); err != nil {
			slog.Error("schedule: capture failed", "name", e.Name, "err", err)
		}
	}
}

func newScheduleCommand(d *deps, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "schedule",
		Short: "Run as a daemon, capturing configured meetings on their cron schedules",
		Long: "schedule runs as a long-lived daemon and captures each configured " +
			"meeting (transcript and audio) when its cron trigger fires. All " +
			"meetings share a single browser profile, so a capture holds that " +
			"profile until the meeting's context ends; while one capture is " +
			"active, overlapping triggers for other meetings are skipped and " +
			"logged rather than launched concurrently. Finite meeting-end " +
			"detection and per-meeting profile isolation (for true concurrent " +
			"capture) are not yet implemented.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			entries := make([]scheduler.Entry, 0, len(cfg.Meetings))
			for _, m := range cfg.Meetings {
				entries = append(entries, scheduler.Entry{Name: m.Name, URL: m.URL, Cron: m.Cron})
			}

			var busy atomic.Bool
			cronScheduler, err := scheduler.Build(entries,
				scheduleJob(cmd.Context(), d, cfg, &busy),
				cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
			if err != nil {
				return err
			}

			cronScheduler.Start()
			slog.Info("schedule: daemon started", "meetings", len(entries))
			<-cmd.Context().Done()

			// Stop dispatching and wait for in-flight captures to finish reaping
			// their browser processes, bounded by shutdownTimeout, to avoid
			// orphaned Chromium processes on shutdown.
			slog.Info("schedule: shutting down")
			stopCtx := cronScheduler.Stop()
			select {
			case <-stopCtx.Done():
			case <-time.After(shutdownTimeout):
				slog.Warn("schedule: shutdown timed out waiting for active captures")
			}
			return nil
		},
	}
}
