package cli

import (
	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
)

const defaultTeamsCaptureURL = "https://teams.microsoft.com/v2/"

type captureFlags struct {
	outputDir   string
	profileDir  string
	botName     string
	browserPath string
	headless    bool
	headed      bool
	audio       bool
}

func bindCaptureFlags(cmd *cobra.Command, f *captureFlags) {
	cmd.Flags().StringVarP(&f.outputDir, "output-dir", "o", "", "directory where the Markdown transcript is saved")
	cmd.Flags().StringVar(&f.profileDir, "profile-dir", "", "browser user-data directory for session persistence")
	cmd.Flags().StringVar(&f.botName, "bot-name", "", "display name injected on guest pre-join")
	cmd.Flags().StringVar(&f.browserPath, "browser-path", "", "path to Chrome/Edge/Chromium executable")
	cmd.Flags().BoolVar(&f.headless, "headless", false, "run browser in headless mode")
	cmd.Flags().BoolVar(&f.headed, "headed", false, "open a visible browser window for interactive authentication")
	cmd.Flags().BoolVar(&f.audio, "audio", false, "also capture tab audio to an MP3 file")
}

func applyCaptureFlags(cmd *cobra.Command, cfg *config.Config, f captureFlags) {
	if f.outputDir != "" {
		cfg.OutputDir = f.outputDir
	}
	if f.profileDir != "" {
		cfg.ProfileDir = f.profileDir
	}
	if f.botName != "" {
		cfg.BotName = f.botName
	}
	if f.browserPath != "" {
		cfg.BrowserPath = f.browserPath
	}
	if cmd.Flags().Changed("headless") {
		cfg.Headless = f.headless
	}
	if f.headed {
		cfg.Headless = false
	}
	if cmd.Flags().Changed("audio") {
		cfg.CaptureAudio = f.audio
	}
}

func newJoinCommand(d *deps, configPath *string) *cobra.Command {
	var flags captureFlags
	cmd := &cobra.Command{
		Use:   "join <meeting-url>",
		Short: "Join a Teams meeting and capture its live captions to a Markdown file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			applyCaptureFlags(cmd, &cfg, flags)
			mutated, err := teams.MutateJoinURL(args[0])
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	bindCaptureFlags(cmd, &flags)
	return cmd
}

func newCaptionsCommand(d *deps, configPath *string) *cobra.Command {
	var flags captureFlags
	cmd := &cobra.Command{
		Use:   "captions [meeting-url]",
		Short: "Open a browser window, wait for user authentication and call entry, and save live captions to Markdown",
		Long: "captions opens a visible browser window (either at the provided Teams " +
			"meeting URL or at teams.microsoft.com/v2/ when omitted), waits for you " +
			"to sign in and enter a Teams call, navigates the call menu to turn on " +
			"live captions, and streams finalized captions into a local Markdown file.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			// captions defaults to a visible browser window for interactive auth
			// unless --headless was explicitly passed.
			if !cmd.Flags().Changed("headless") {
				cfg.Headless = false
			}
			applyCaptureFlags(cmd, &cfg, flags)
			rawURL := defaultTeamsCaptureURL
			if len(args) == 1 {
				rawURL = args[0]
			}
			mutated, err := teams.MutateJoinURL(rawURL)
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	bindCaptureFlags(cmd, &flags)
	return cmd
}
