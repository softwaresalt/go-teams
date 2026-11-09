// Package cli wires the go-teams Cobra command tree.
package cli

import (
	"context"
	"errors"
	"os"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/capture"
	"github.com/softwaresalt/go-teams/internal/config"
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
}

func defaultDeps() *deps {
	return &deps{
		loadConfig: config.Load,
		newRunner: func(c config.Config) browser.Runner {
			return browser.NewChromeRunner(browser.Options{
				UserDataDir: c.ProfileDir,
				Headless:    c.Headless,
				ExecPath:    c.BrowserPath,
			})
		},
		newMeeting: defaultNewMeeting,
		newSession: func(c config.Config, meetingURL string) meetingSession {
			return capture.NewSession(capture.SessionConfig{
				OutputDir:    c.OutputDir,
				MeetingURL:   meetingURL,
				BotName:      c.BotName,
				CaptionsOnly: !c.CaptureAudio,
			})
		},
	}
}

// defaultNewMeeting allocates a Chromium context for meeting capture and
// returns a chromedp-backed Meeting bound to it. The cleanup func cancels the
// browser and allocator contexts, terminating the spawned Chromium process.
func defaultNewMeeting(ctx context.Context, c config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error) {
	return browser.NewLiveMeeting(ctx,
		browser.Options{
			UserDataDir: c.ProfileDir,
			Headless:    c.Headless,
			ExecPath:    c.BrowserPath,
		},
		browser.MeetingOptions{MeetingURL: meetingURL, BotName: c.BotName})
}

// NewRootCommand builds the root command with all subcommands attached, using
// the production dependency set.
func NewRootCommand() *cobra.Command {
	return newRootCommand(defaultDeps())
}

func newRootCommand(d *deps) *cobra.Command {
	var configPath string
	root := &cobra.Command{
		Use:          "teams-cli",
		Short:        "Autonomous Microsoft Teams live-transcription and audio capture",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&configPath, "config", "",
		"path to the config file (default: user config directory)")
	root.AddCommand(
		newAuthCommand(d, &configPath),
		newJoinCommand(d, &configPath),
		newCaptionsCommand(d, &configPath),
		newScheduleCommand(d, &configPath),
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
		p, err := config.DefaultConfigPath()
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
			return def, nil
		}
		return config.Config{}, err
	}
	return cfg, nil
}
