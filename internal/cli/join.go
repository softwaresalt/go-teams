package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
)

const defaultTeamsCaptureURL = "https://teams.microsoft.com/v2/"

type captureFlags struct {
	meetingURL  string
	output      string
	outputFile  string
	outputDir   string
	profileDir  string
	botName     string
	browserPath string
	headless    bool
	headed      bool
	audio       bool
}

func bindCaptureFlags(cmd *cobra.Command, f *captureFlags) {
	cmd.Flags().StringVarP(&f.meetingURL, "url", "u", "", "Teams meeting or recap URL to capture")
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "output Markdown file path (.md) or directory")
	cmd.Flags().StringVar(&f.outputFile, "output-file", "", "explicit output Markdown file path (.md)")
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "directory where the Markdown transcript is saved")
	cmd.Flags().StringVar(&f.profileDir, "profile-dir", "", "browser user-data directory for session persistence")
	cmd.Flags().StringVar(&f.botName, "bot-name", "", "display name injected on guest pre-join")
	cmd.Flags().StringVar(&f.browserPath, "browser-path", "", "path to Chrome/Edge/Chromium executable")
	cmd.Flags().BoolVar(&f.headless, "headless", false, "run browser in headless mode")
	cmd.Flags().BoolVar(&f.headed, "headed", false, "open a visible browser window for interactive authentication")
	cmd.Flags().BoolVar(&f.audio, "audio", false, "also capture tab audio to an MP3 file")
}

func applyOutputTarget(cfg *config.Config, target string) {
	if target == "" {
		return
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		cfg.OutputDir = target
		cfg.OutputFile = ""
		return
	}
	ext := strings.ToLower(filepath.Ext(target))
	if ext == ".md" || ext == ".markdown" {
		cfg.OutputFile = target
		return
	}
	cfg.OutputDir = target
	cfg.OutputFile = ""
}

func applyCaptureFlags(cmd *cobra.Command, cfg *config.Config, f captureFlags) {
	if f.outputDir != "" {
		cfg.OutputDir = f.outputDir
		cfg.OutputFile = ""
	}
	if f.output != "" {
		applyOutputTarget(cfg, f.output)
	}
	if f.outputFile != "" {
		cfg.OutputFile = f.outputFile
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
		Use:   "join <meeting-url> [output-markdown-path]",
		Short: "Join a Teams meeting and capture its live captions to a Markdown file",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rawURL := flags.meetingURL
			if len(args) >= 1 {
				rawURL = args[0]
			}
			if rawURL == "" {
				return fmt.Errorf("cli: join requires a <meeting-url> argument or --url flag")
			}
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			applyCaptureFlags(cmd, &cfg, flags)
			if len(args) >= 2 {
				applyOutputTarget(&cfg, args[1])
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

func newCaptionsCommand(d *deps, configPath *string) *cobra.Command {
	var flags captureFlags
	cmd := &cobra.Command{
		Use:   "captions [meeting-url] [output-markdown-path]",
		Short: "Capture Teams live captions or Meeting Recap transcript + AI Summary to Markdown",
		Long: "captions opens a browser (headed by default, or headless with --headless), " +
			"navigates to the provided Teams meeting or Meeting Recap URL, and writes " +
			"the captured transcript and AI Summary to the specified Markdown file or output directory.",
		Args: cobra.MaximumNArgs(2),
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
			if flags.meetingURL != "" {
				rawURL = flags.meetingURL
			}
			if len(args) >= 1 {
				rawURL = args[0]
			}
			if len(args) >= 2 {
				applyOutputTarget(&cfg, args[1])
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
