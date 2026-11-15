package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/browser"
)

func newAuthCommand(d *deps, configPath *string) *cobra.Command {
	var flags browserFlags
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Launch a visible browser to sign in to Teams and cache the session",
		Long: "auth opens a visible (headed) browser at teams.microsoft.com so you " +
			"can complete Microsoft 365 login and MFA manually. On exit, session " +
			"cookies and tokens are persisted to the profile directory for later " +
			"headless use by the join, captions, and schedule commands.\n\n" +
			"Profile directory:\n" +
			"  --profile-dir (or profile_dir in config.yaml) selects the browser user-data directory.\n" +
			"  It defaults to %LOCALAPPDATA%\\go-teams\\profile on Windows when LOCALAPPDATA is set.\n" +
			"  Reuse the same directory for every command that should share the signed-in session.\n\n" +
			"Account selection (required):\n" +
			"  --account (or default_account in config.yaml) is required for initial authentication.\n" +
			"  Supply the Microsoft Entra account (UPN/email or display name) so the Entra account\n" +
			"  tile picker selects that account deterministically.\n\n" +
			"Profile locking:\n" +
			"  A Chromium profile needs exclusive access. Do not run two browsers against the same\n" +
			"  profile directory at once; close the auth window before starting a headless capture.\n\n" +
			"Other browser flags: --browser-path selects the Chrome, Edge, or Chromium executable\n" +
			"(also read from TEAMS_BROWSER_PATH).",
		Example: "  go-teams auth --account alice@contoso.com\n" +
			"  go-teams auth --account alice@contoso.com --profile-dir \"%LOCALAPPDATA%\\go-teams\\profile\"\n" +
			"  go-teams auth --account alice@contoso.com --browser-path \"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe\"",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			applyBrowserFlags(cmd, &cfg, flags)
			if strings.TrimSpace(cfg.DefaultAccount) == "" {
				return fmt.Errorf("cli: %w", browser.ErrAccountRequired)
			}
			return d.newRunner(cfg).Authenticate(cmd.Context())
		},
	}
	bindBrowserFlags(cmd, &flags)
	return cmd
}
