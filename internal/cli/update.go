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
			"swap succeeds and is restored automatically if it fails.\n\n" +
			"Release asset: teams-cli-<goos>-<goarch> (with an .exe suffix on Windows) for linux,\n" +
			"darwin, and windows on amd64 and arm64. A missing SHA256SUMS file, a missing entry for\n" +
			"your platform, or a digest mismatch aborts the update and leaves the current binary untouched.\n\n" +
			"Flags:\n" +
			"  --check            Report whether a newer release exists without installing it.\n" +
			"  --version <tag>    Install a specific release (for example v1.2.0) instead of the latest.\n" +
			"  --force            Reinstall the target even when current, and allow a downgrade.\n\n" +
			"Locking: two updates cannot swap the same executable at once. Each update takes <exe>.lock\n" +
			"(JSON owner metadata: pid, token, created_at) plus a short-lived <exe>.lock.ctl control lock,\n" +
			"waiting up to 15 seconds before failing with a locked error. The lock fails closed:\n" +
			"teams-cli never evicts another process's lock on its own.\n\n" +
			"Manual lock recovery: if an update was killed and left <exe>.lock behind, confirm that the\n" +
			"recorded pid is no longer running, delete <exe>.lock (and <exe>.lock.ctl if present) by hand,\n" +
			"and rerun the update. Executable names ending in .lock or .lock.ctl are rejected.\n\n" +
			"The executable's directory must be writable by the current user; an install under a\n" +
			"privileged folder such as C:\\Program Files needs an elevated shell. Local builds report\n" +
			"the version dev and treat any release as an upgrade.",
		Example: "  teams-cli update --check\n" +
			"  teams-cli update\n" +
			"  teams-cli update --version v1.2.0\n" +
			"  teams-cli update --version v1.1.0 --force",
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
