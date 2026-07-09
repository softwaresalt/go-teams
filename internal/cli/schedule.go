package cli

import (
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/scheduler"
	"github.com/softwaresalt/go-teams/internal/teams"
)

func newScheduleCommand(d *deps, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "schedule",
		Short: "Run as a daemon, joining configured meetings on their cron schedules",
		Args:  cobra.NoArgs,
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
			})
			if err != nil {
				return err
			}

			cronScheduler.Start()
			defer cronScheduler.Stop()
			slog.Info("schedule: daemon started", "meetings", len(entries))
			<-cmd.Context().Done()
			return nil
		},
	}
}
