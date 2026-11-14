// Package cli wires the go-teams Cobra command tree.
package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/capture"
	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
	"github.com/softwaresalt/go-teams/internal/updater"
	"github.com/softwaresalt/go-teams/internal/version"
)

// deps holds injectable dependencies so commands can be unit-tested with fakes
// in place of real config loading and browser automation.
type deps struct {
	loadConfig func(path string) (config.Config, error)
	newRunner  func(config.Config) browser.Runner
	// newMeeting builds a browser.Meeting bound to a fresh Chromium context
	// derived from ctx, plus a cleanup func that tears the browser down. The
	// returned context is the one that drives the meeting's methods.
	newMeeting func(ctx context.Context, cfg config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error)
	// newSession builds the capture session that records a joined meeting.
	newSession func(cfg config.Config, meetingURL string) meetingSession
	// runUpdate checks for or installs a teams-cli release.
	runUpdate func(ctx context.Context, opts updateOptions) (updater.Result, error)
}

func defaultDeps() *deps {
	return &deps{
		loadConfig: config.Load,
		runUpdate:  defaultRunUpdate,
		newRunner: func(c config.Config) browser.Runner {
			return browser.NewChromeRunner(browserOptions(c))
		},
		newMeeting: defaultNewMeeting,
		newSession: func(c config.Config, meetingURL string) meetingSession {
			dur, _ := parseOptionalDuration(c.Duration)
			idle, _ := parseOptionalDuration(c.IdleTimeout)
			return capture.NewSession(capture.SessionConfig{
				OutputDir:        c.OutputDir,
				OutputFile:       c.OutputFile,
				FilenameTemplate: c.FilenameTemplate,
				MeetingURL:       meetingURL,
				MeetingTitle:     c.MeetingTitle,
				Category:         c.Category,
				BotName:          c.BotName,
				CaptionsOnly:     !c.CaptureAudio,
				Mode:             c.CaptureMode,
				Duration:         dur,
				IdleTimeout:      idle,
			})
		},
	}
}

// defaultRunUpdate runs the production updater against GitHub Releases.
func defaultRunUpdate(ctx context.Context, opts updateOptions) (updater.Result, error) {
	u := updater.New(updater.Config{CurrentVersion: version.Version})
	if opts.CheckOnly {
		return u.Check(ctx, opts.TargetVersion)
	}
	return u.Update(ctx, opts.TargetVersion, opts.Force)
}

// browserOptions maps the effective config to the browser allocator options,
// so every command launches the same configured Chromium executable and profile.
func browserOptions(c config.Config) browser.Options {
	return browser.Options{
		UserDataDir:    c.ProfileDir,
		Headless:       c.Headless,
		ExecPath:       c.BrowserPath,
		DefaultAccount: c.DefaultAccount,
	}
}

// browserFlags holds the browser selection flags shared by commands that do
// not bind the full capture flag set (auth and schedule).
type browserFlags struct {
	browserPath string
	profileDir  string
	account     string
}

// bindBrowserFlags registers --browser-path, --profile-dir, and --account on cmd.
func bindBrowserFlags(cmd *cobra.Command, f *browserFlags) {
	cmd.Flags().StringVar(&f.browserPath, "browser-path", "", "path to Chrome/Edge/Chromium executable")
	cmd.Flags().StringVar(&f.profileDir, "profile-dir", "", "browser user-data directory for session persistence")
	cmd.Flags().StringVar(&f.account, "account", "", "default Microsoft Entra account (UPN/email or display name) to auto-select")
}

// applyBrowserFlags overrides cfg with any browser flag the operator supplied.
func applyBrowserFlags(cmd *cobra.Command, cfg *config.Config, f browserFlags) {
	if f.browserPath != "" {
		cfg.BrowserPath = f.browserPath
	}
	if f.profileDir != "" {
		cfg.ProfileDir = f.profileDir
	}
	if (cmd != nil && cmd.Flags().Changed("account")) || strings.TrimSpace(f.account) != "" {
		cfg.DefaultAccount = strings.TrimSpace(f.account)
	}
}

// defaultNewMeeting allocates a Chromium context for meeting capture and
// returns a chromedp-backed Meeting bound to it. The cleanup func cancels the
// browser and allocator contexts, terminating the spawned Chromium process.
func defaultNewMeeting(ctx context.Context, c config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error) {
	lobbyTimeout, _ := parseOptionalDuration(c.LobbyTimeout)
	return browser.NewLiveMeeting(ctx,
		browserOptions(c),
		browser.MeetingOptions{
			MeetingURL:     meetingURL,
			BotName:        c.BotName,
			DefaultAccount: c.DefaultAccount,
			LobbyTimeout:   lobbyTimeout,
			ForceRecap:     strings.EqualFold(strings.TrimSpace(c.CaptureMode), config.CaptureModePastRecap),
		})
}

// NewRootCommand builds the root command with all subcommands attached, using
// the production dependency set.
func NewRootCommand() *cobra.Command {
	return newRootCommand(defaultDeps())
}

func newRootCommand(d *deps) *cobra.Command {
	var configPath string
	var flags captureFlags
	root := &cobra.Command{
		Use:   "teams-cli [meeting-url] [output-markdown-path]",
		Short: "Autonomous Microsoft Teams live-transcription and meeting recap capture",
		Long: "teams-cli captures live captions and Meeting Recap transcripts (including " +
			"the AI Summary) from Microsoft Teams into local Markdown files.\n\n" +
			"Examples:\n" +
			"  teams-cli.exe \"https://teams.cloud.microsoft/l/meetingrecap?...\" \"C:\\out\\transcript.md\" --headless\n" +
			"  teams-cli.exe --queue transcript-queue.txt\n" +
			"  teams-cli.exe captions \"https://teams.cloud.microsoft/l/meetingrecap?...\" -o \"C:\\out\\transcript.md\" --headless",
		SilenceUsage: true,
		Version:      version.Version,
		Args:         cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.queueFile != "" {
				if isInsideQueueExecution(cmd.Context()) {
					return errors.New("cli: nested --queue flag is not allowed")
				}
				return runQueueFile(cmd.Context(), d, flags.queueFile, inheritedQueueFlags(cmd, configPath, flags))
			}
			cfg, err := loadConfigFrom(d, configPath)
			if err != nil {
				return err
			}
			applyCaptureFlags(cmd, &cfg, flags)
			if len(args) >= 2 {
				applyOutputTarget(&cfg, args[1])
			}
			rawURL := cfg.MeetingURL
			if flags.meetingURL != "" {
				rawURL = flags.meetingURL
			}
			if len(args) >= 1 {
				rawURL = args[0]
			}
			if rawURL == "" {
				return cmd.Help()
			}
			mutated, err := teams.MutateJoinURL(rawURL)
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "f", "",
		"path to the YAML config file (default: ./config.yaml or user config directory)")
	bindCaptureFlags(root, &flags)
	root.AddCommand(
		newAuthCommand(d, &configPath),
		newJoinCommand(d, &configPath),
		newCaptionsCommand(d, &configPath),
		newScheduleCommand(d, &configPath),
		newUpdateCommand(d),
	)
	return root
}

// loadConfigFrom resolves the effective config path (falling back to the
// default location) and loads it via the injected loader. When no --config path
// was explicitly specified and the default config file does not exist on disk,
// it falls back to DefaultConfig with Headless=false so the CLI opens a visible
// browser window for interactive user authentication out of the box.
func loadConfigFrom(d *deps, configPath string) (config.Config, error) {
	path := configPath
	if path == "" {
		p, err := resolveDefaultConfigPath()
		if err != nil {
			return config.Config{}, err
		}
		path = p
	}
	cfg, err := d.loadConfig(path)
	if err != nil {
		if configPath == "" && errors.Is(err, os.ErrNotExist) {
			def, defErr := config.DefaultConfig()
			if defErr != nil {
				return config.Config{}, defErr
			}
			def.Headless = false
			def.OutputDir = "transcripts"
			return def, nil
		}
		return config.Config{}, err
	}
	return cfg, nil
}

func resolveDefaultConfigPath() (string, error) {
	candidates := []string{"config.yaml"}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "config.yaml"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return config.DefaultConfigPath()
}
