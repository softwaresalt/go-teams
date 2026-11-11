package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/updater"
)

// updateOptions carries the flags of the update command to deps.runUpdate.
type updateOptions struct {
	// CheckOnly reports update availability without installing.
	CheckOnly bool
	// TargetVersion selects a specific release tag; empty means latest.
	TargetVersion string
	// Force reinstalls the target even when it is current or older.
	Force bool
}

// newUpdateCommand builds the self-update subcommand. It deliberately does not
// load the capture config or touch the browser layer.
func newUpdateCommand(d *deps) *cobra.Command {
	var opts updateOptions
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update teams-cli to the latest release version",
		Long: "Update replaces the running teams-cli executable with a GitHub Release build.\n" +
			"The download is verified against the release's SHA256SUMS before the\n" +
			"executable is replaced; the previous binary is kept as <exe>.old until the\n" +
			"swap succeeds and is restored automatically if it fails.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if d.runUpdate == nil {
				return errors.New("cli: update is not configured")
			}
			res, err := d.runUpdate(cmd.Context(), opts)
			if err != nil {
				return fmt.Errorf("update: %w", err)
			}
			printUpdateResult(cmd.OutOrStdout(), opts, res)
			return nil
		},
	}
	cmd.Flags().BoolVar(&opts.CheckOnly, "check", false, "check for an update without installing it")
	cmd.Flags().StringVar(&opts.TargetVersion, "version", "", "install a specific release version (for example v1.2.0) instead of the latest")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "reinstall even when already on the target version, and allow downgrades")
	return cmd
}

// printUpdateResult writes a human-readable summary of res to w. Write errors
// are intentionally ignored: output is informational and the update outcome is
// already decided.
func printUpdateResult(w io.Writer, opts updateOptions, res updater.Result) {
	switch {
	case opts.CheckOnly && res.UpdateAvailable:
		install := "teams-cli update"
		if opts.TargetVersion != "" {
			install += " --version " + res.TagName
		}
		_, _ = fmt.Fprintf(w, "Update available: %s -> %s. Run `%s` to install it.\n",
			res.CurrentVersion, res.LatestVersion, install)
	case opts.CheckOnly && res.Downgrade:
		_, _ = fmt.Fprintf(w, "Release %s is older than the running version %s. Use `teams-cli update --version %s --force` to downgrade.\n",
			res.LatestVersion, res.CurrentVersion, res.TagName)
	case opts.CheckOnly:
		_, _ = fmt.Fprintf(w, "teams-cli is up to date (%s).\n", res.CurrentVersion)
	case res.Updated:
		_, _ = fmt.Fprintf(w, "Updated teams-cli from %s to %s.\n", res.CurrentVersion, res.LatestVersion)
	default:
		_, _ = fmt.Fprintf(w, "teams-cli is already up to date (%s). Use --force to reinstall.\n", res.CurrentVersion)
	}
}
