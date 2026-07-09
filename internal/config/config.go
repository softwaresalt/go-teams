// Package config defines the go-teams CLI configuration model and its
// on-disk YAML persistence.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const appDirName = "teams-cli"

// Sentinel validation errors returned by Config.Validate.
var (
	// ErrEmptyOutputDir indicates that OutputDir was not set.
	ErrEmptyOutputDir = errors.New("config: output_dir must not be empty")
	// ErrEmptyProfileDir indicates that ProfileDir was not set.
	ErrEmptyProfileDir = errors.New("config: profile_dir must not be empty")
	// ErrEmptyBotName indicates that BotName was not set.
	ErrEmptyBotName = errors.New("config: bot_name must not be empty")
	// ErrMeetingMissingURL indicates a scheduled meeting has no URL.
	ErrMeetingMissingURL = errors.New("config: meeting url must not be empty")
	// ErrMeetingMissingCron indicates a scheduled meeting has no cron
	// expression.
	ErrMeetingMissingCron = errors.New("config: meeting cron must not be empty")
)

// MeetingSchedule maps a Teams meeting URL to a cron expression consumed by
// the scheduler daemon.
type MeetingSchedule struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	Cron string `yaml:"cron"`
}

// Config is the top-level go-teams CLI configuration.
type Config struct {
	OutputDir  string            `yaml:"output_dir"`
	ProfileDir string            `yaml:"profile_dir"`
	BotName    string            `yaml:"bot_name"`
	Headless   bool              `yaml:"headless"`
	Meetings   []MeetingSchedule `yaml:"meetings"`
}

// DefaultConfig returns a Config populated with platform-appropriate default
// paths resolved via os.UserConfigDir.
func DefaultConfig() (Config, error) {
	appDir, err := appConfigDir()
	if err != nil {
		return Config{}, err
	}
	return Config{
		OutputDir:  filepath.Join(appDir, "transcripts"),
		ProfileDir: filepath.Join(appDir, "profile"),
		BotName:    "Transcription Agent",
		Headless:   true,
	}, nil
}

// DefaultConfigPath returns the default location of the config file.
func DefaultConfigPath() (string, error) {
	appDir, err := appConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(appDir, "config.yaml"), nil
}

func appConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve user config dir: %w", err)
	}
	return filepath.Join(base, appDirName), nil
}

// Validate reports whether the configuration is internally consistent.
func (c Config) Validate() error {
	switch {
	case c.OutputDir == "":
		return ErrEmptyOutputDir
	case c.ProfileDir == "":
		return ErrEmptyProfileDir
	case c.BotName == "":
		return ErrEmptyBotName
	}
	for i, m := range c.Meetings {
		if m.URL == "" {
			return fmt.Errorf("config: meeting %d: %w", i, ErrMeetingMissingURL)
		}
		if m.Cron == "" {
			return fmt.Errorf("config: meeting %d: %w", i, ErrMeetingMissingCron)
		}
	}
	return nil
}

// Load reads and parses the YAML configuration at path. Fields absent from the
// file retain the values from DefaultConfig. The parsed configuration is
// validated before being returned.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("config: read %s: %w", path, err)
	}
	cfg, err := DefaultConfig()
	if err != nil {
		return Config{}, err
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

// Save writes cfg to path as YAML, creating parent directories as needed. The
// write is atomic: content is written to a temporary file in the same
// directory and renamed into place.
func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("config: create dir %s: %w", dir, err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("config: create temp file: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: rename temp file: %w", err)
	}
	return nil
}
