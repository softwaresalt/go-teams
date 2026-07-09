package cli

import "github.com/spf13/cobra"

func newAuthCommand(d *deps, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Launch a visible browser to sign in to Teams and cache the session",
		Long: "auth opens a visible (headed) browser at teams.microsoft.com so you " +
			"can complete Microsoft 365 login and MFA manually. On exit, session " +
			"cookies and tokens are persisted to the profile directory for later " +
			"headless use by the join and schedule commands.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			return d.newRunner(cfg).Authenticate(cmd.Context())
		},
	}
}
