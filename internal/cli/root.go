// Package cli wires the go-teams Cobra command tree.
package cli

import (
	"context"

	"github.com/chromedp/chromedp"
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
			})
		},
		newMeeting: defaultNewMeeting,
		newSession: func(c config.Config, meetingURL string) meetingSession {
			return capture.NewSession(capture.SessionConfig{
				OutputDir:  c.OutputDir,
				MeetingURL: meetingURL,
				BotName:    c.BotName,
			})
		},
	}
}

// defaultNewMeeting allocates a headless Chromium context for automated capture
// and returns a chromedp-backed Meeting bound to it. The cleanup func cancels
// the browser and allocator contexts, terminating the spawned Chromium process.
func defaultNewMeeting(ctx context.Context, c config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error) {
	opts := browser.Options{UserDataDir: c.ProfileDir, Headless: true}
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, browser.AllocatorOptions(opts)...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	cleanup := func() {
		cancelBrowser()
		cancelAlloc()
	}
	m := browser.NewMeeting(browser.MeetingOptions{MeetingURL: meetingURL, BotName: c.BotName})
	return m, browserCtx, cleanup, nil
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
		newScheduleCommand(d, &configPath),
	)
	return root
}

// loadConfigFrom resolves the effective config path (falling back to the
// default location) and loads it via the injected loader.
func loadConfigFrom(d *deps, configPath string) (config.Config, error) {
	path := configPath
	if path == "" {
		p, err := config.DefaultConfigPath()
		if err != nil {
			return config.Config{}, err
		}
		path = p
	}
	return d.loadConfig(path)
}
