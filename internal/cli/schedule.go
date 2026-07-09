package cli

import (
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/scheduler"
	"github.com/softwaresalt/go-teams/internal/teams"
)

// shutdownTimeout bounds how long the schedule daemon waits for in-flight joins
// (and their browser processes) to be reaped during graceful shutdown.
const shutdownTimeout = 15 * time.Second

func newScheduleCommand(d *deps, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "schedule",
		Short: "Run as a daemon, joining configured meetings on their cron schedules",
		Long: "schedule runs as a long-lived daemon and joins each configured " +
			"meeting when its cron trigger fires. A join currently holds the " +
			"browser (and its single profile) open until the daemon is stopped, " +
			"so overlapping triggers are skipped while a join is active; " +
			"per-meeting isolation and meeting-end detection arrive with live " +
			"capture support.",
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

			runner := d.newRunner(cfg)
			cronScheduler, err := scheduler.Build(entries, func(e scheduler.Entry) {
				mutated, err := teams.MutateJoinURL(e.URL)
				if err != nil {
					slog.Error("schedule: invalid meeting url", "name", e.Name, "err", err)
					return
				}
				if err := runner.Join(cmd.Context(), mutated); err != nil {
					slog.Error("schedule: join failed", "name", e.Name, "err", err)
				}
			}, cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
			if err != nil {
				return err
			}

			cronScheduler.Start()
			slog.Info("schedule: daemon started", "meetings", len(entries))
			<-cmd.Context().Done()

			// Stop dispatching and wait for in-flight joins to finish reaping
			// their browser processes, bounded by shutdownTimeout, to avoid
			// orphaned Chromium processes on shutdown.
			slog.Info("schedule: shutting down")
			stopCtx := cronScheduler.Stop()
			select {
			case <-stopCtx.Done():
			case <-time.After(shutdownTimeout):
				slog.Warn("schedule: shutdown timed out waiting for active joins")
			}
			return nil
		},
	}
}
