package cli

import (
	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/teams"
)

func newJoinCommand(d *deps, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "join <meeting-url>",
		Short: "Join a Teams meeting immediately and capture its transcript and audio",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			mutated, err := teams.MutateJoinURL(args[0])
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
}
