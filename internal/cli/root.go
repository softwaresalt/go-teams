// Package cli wires the go-teams Cobra command tree.
package cli

import (
	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

// deps holds injectable dependencies so commands can be unit-tested with fakes
// in place of real config loading and browser automation.
type deps struct {
	loadConfig func(path string) (config.Config, error)
	newRunner  func(config.Config) browser.Runner
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
	}
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
