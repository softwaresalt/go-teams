// Package config defines the go-teams CLI configuration model and its
// on-disk YAML persistence.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	appDirName = "go-teams"
	// DefaultFilenameTemplate is the default transcript filename pattern used
	// when only an output directory is specified.
	DefaultFilenameTemplate = "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"

	// CaptureModeAuto selects past-recap for Recap URLs and live-with-summary
	// (or ephemeral-live when delayed AI summary is disabled) for live URLs.
	CaptureModeAuto = "auto"
	// CaptureModeEphemeralLive captures live captions and audio in real time
	// with no post-call AI Summary wait.
	CaptureModeEphemeralLive = "ephemeral-live"
	// CaptureModeLiveWithSummary captures live captions and optional audio and
	// fetches the post-call AI Summary after ai_summary_delay.
	CaptureModeLiveWithSummary = "live-with-summary"
	// CaptureModePastRecap exports a finite post-meeting Recap transcript, AI
	// Summary, and optional audio.
	CaptureModePastRecap = "past-recap"
)

// IsValidCaptureMode reports whether mode is a supported capture protocol.
func IsValidCaptureMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case CaptureModeAuto, CaptureModeEphemeralLive, CaptureModeLiveWithSummary, CaptureModePastRecap:
		return true
	default:
		return false
	}
}

var winEnvVarRe = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)

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
	// ErrMeetingMissingCron indicates a scheduled meeting has no cron or at
	// trigger.
	ErrMeetingMissingCron = errors.New("config: meeting cron must not be empty")
	// ErrMeetingScheduleConflict indicates a scheduled meeting specifies both
	// cron and at triggers.
	ErrMeetingScheduleConflict = errors.New("config: meeting must specify either cron or at, not both")
	// ErrInvalidCaptureMode indicates an unsupported capture_mode / mode value.
	ErrInvalidCaptureMode = errors.New("config: invalid capture mode")
	// ErrInvalidDuration indicates an invalid duration string.
	ErrInvalidDuration = errors.New("config: invalid duration")
)

// MeetingSchedule maps a Teams meeting URL to a cron expression or one-shot
// timestamp consumed by the scheduler daemon, along with optional per-meeting
// capture overrides.
type MeetingSchedule struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	Cron           string `yaml:"cron,omitempty"`
	At             string `yaml:"at,omitempty"`
	Mode           string `yaml:"mode,omitempty"`
	Duration       string `yaml:"duration,omitempty"`
	IdleTimeout    string `yaml:"idle_timeout,omitempty"`
	AISummaryDelay string `yaml:"ai_summary_delay,omitempty"`
	LobbyTimeout   string `yaml:"lobby_timeout,omitempty"`
	Category       string `yaml:"category,omitempty"`
	CaptureAudio   *bool  `yaml:"capture_audio,omitempty"`
	Audio          *bool  `yaml:"audio,omitempty"`
	Overwrite      *bool  `yaml:"overwrite,omitempty"`
	FileOverwrite  *bool  `yaml:"file_overwrite,omitempty"`
	OutputDir      string `yaml:"output_dir,omitempty"`
}

// Config is the top-level go-teams CLI configuration.
type Config struct {
	OutputDir            string            `yaml:"output_dir"`
	OutputFile           string            `yaml:"output_file,omitempty"`
	FilenameTemplate     string            `yaml:"filename_template,omitempty"`
	MeetingURL           string            `yaml:"meeting_url,omitempty"`
	ProfileDir           string            `yaml:"profile_dir"`
	BotName              string            `yaml:"bot_name"`
	Category             string            `yaml:"category,omitempty"`
	Headless             bool              `yaml:"headless"`
	CaptureAudio         bool              `yaml:"capture_audio,omitempty"`
	Overwrite            bool              `yaml:"overwrite,omitempty"`
	BrowserPath          string            `yaml:"browser_path,omitempty"`
	DefaultAccount       string            `yaml:"default_account,omitempty"`
	CaptureMode          string            `yaml:"capture_mode,omitempty"`
	Duration             string            `yaml:"duration,omitempty"`
	IdleTimeout          string            `yaml:"idle_timeout,omitempty"`
	AISummaryDelay       string            `yaml:"ai_summary_delay,omitempty"`
	LobbyTimeout         string            `yaml:"lobby_timeout,omitempty"`
	QueueFile            string            `yaml:"queue_file,omitempty"`
	ConfigPath           string            `yaml:"-"`
	MeetingTitle         string            `yaml:"-"`
	AppendAISummary      string            `yaml:"-"`
	NotBefore            string            `yaml:"-"`
	ExplicitCaptureAudio bool              `yaml:"-"`
	Meetings             []MeetingSchedule `yaml:"meetings"`
}

// DefaultConfig returns a Config populated with platform-appropriate default
// paths resolved in the user's Windows profile (%LOCALAPPDATA%\go-teams and
// %APPDATA%\go-teams).
func DefaultConfig() (Config, error) {
	appDir, err := appConfigDir()
	if err != nil {
		return Config{}, err
	}
	return Config{
		OutputDir:        filepath.Join(appDir, "transcripts"),
		FilenameTemplate: DefaultFilenameTemplate,
		ProfileDir:       defaultProfileDir(appDir),
		BotName:          "Transcription Agent",
		Headless:         true,
	}, nil
}

// defaultProfileDir returns the standard per-user browser profile location on
// Windows (%LOCALAPPDATA%\go-teams\profile, matching Edge/Chrome User Data
// placement under AppData\Local), falling back to appDir\profile when
// LOCALAPPDATA is unset.
func defaultProfileDir(appDir string) string {
	if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
		return filepath.Join(localAppData, appDirName, "profile")
	}
	return filepath.Join(appDir, "profile")
}

func expandUserPath(raw string) string {
	if raw == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	expanded := winEnvVarRe.ReplaceAllStringFunc(raw, func(m string) string {
		sub := winEnvVarRe.FindStringSubmatch(m)
		if len(sub) == 2 {
			if strings.EqualFold(sub[1], "USER") || strings.EqualFold(sub[1], "USERPROFILE") {
				if home != "" {
					return home
				}
			}
			if val, ok := os.LookupEnv(sub[1]); ok {
				return val
			}
		}
		return m
	})
	expanded = os.ExpandEnv(expanded)
	if expanded == "~" && home != "" {
		expanded = home
	}
	if strings.HasPrefix(expanded, "~"+string(filepath.Separator)) || strings.HasPrefix(expanded, "~/") {
		if home != "" {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	if filepath.Separator != '\\' {
		expanded = strings.ReplaceAll(expanded, `\`, string(filepath.Separator))
	}
	return filepath.Clean(expanded)
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

func validatePositiveDuration(raw, field string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil || d <= 0 {
		return fmt.Errorf("config: %s %q: %w", field, raw, ErrInvalidDuration)
	}
	return nil
}

func validateNonNegativeDuration(raw, field string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil || d < 0 {
		return fmt.Errorf("config: %s %q: %w", field, raw, ErrInvalidDuration)
	}
	return nil
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
	if strings.TrimSpace(c.CaptureMode) != "" && !IsValidCaptureMode(c.CaptureMode) {
		return fmt.Errorf("config: capture_mode %q: %w", c.CaptureMode, ErrInvalidCaptureMode)
	}
	if err := validatePositiveDuration(c.Duration, "duration"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.IdleTimeout, "idle_timeout"); err != nil {
		return err
	}
	if err := validateNonNegativeDuration(c.AISummaryDelay, "ai_summary_delay"); err != nil {
		return err
	}
	if err := validatePositiveDuration(c.LobbyTimeout, "lobby_timeout"); err != nil {
		return err
	}
	for i, m := range c.Meetings {
		if strings.TrimSpace(m.URL) == "" {
			return fmt.Errorf("config: meeting %d: %w", i, ErrMeetingMissingURL)
		}
		hasCron := strings.TrimSpace(m.Cron) != ""
		hasAt := strings.TrimSpace(m.At) != ""
		if !hasCron && !hasAt {
			return fmt.Errorf("config: meeting %d: %w", i, ErrMeetingMissingCron)
		}
		if hasCron && hasAt {
			return fmt.Errorf("config: meeting %d: %w", i, ErrMeetingScheduleConflict)
		}
		if strings.TrimSpace(m.Mode) != "" && !IsValidCaptureMode(m.Mode) {
			return fmt.Errorf("config: meeting %d mode %q: %w", i, m.Mode, ErrInvalidCaptureMode)
		}
		if err := validatePositiveDuration(m.Duration, fmt.Sprintf("meeting %d duration", i)); err != nil {
			return err
		}
		if err := validatePositiveDuration(m.IdleTimeout, fmt.Sprintf("meeting %d idle_timeout", i)); err != nil {
			return err
		}
		if err := validateNonNegativeDuration(m.AISummaryDelay, fmt.Sprintf("meeting %d ai_summary_delay", i)); err != nil {
			return err
		}
		if err := validatePositiveDuration(m.LobbyTimeout, fmt.Sprintf("meeting %d lobby_timeout", i)); err != nil {
			return err
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
	var aliases struct {
		OutputFolder  string `yaml:"output_folder"`
		DefaultPath   string `yaml:"default_path"`
		Output        string `yaml:"output"`
		NameTemplate  string `yaml:"name_template"`
		URL           string `yaml:"url"`
		EntraAccount  string `yaml:"entra_account"`
		Account       string `yaml:"account"`
		Mode          string `yaml:"mode"`
		Headless      *bool  `yaml:"headless"`
		Headed        *bool  `yaml:"headed"`
		CaptureAudio  *bool  `yaml:"capture_audio"`
		Audio         *bool  `yaml:"audio"`
		Overwrite     *bool  `yaml:"overwrite"`
		FileOverwrite *bool  `yaml:"file_overwrite"`
		Meetings      []struct {
			CaptureMode   string `yaml:"capture_mode"`
			Overwrite     *bool  `yaml:"overwrite"`
			FileOverwrite *bool  `yaml:"file_overwrite"`
		} `yaml:"meetings"`
	}
	if err := yaml.Unmarshal(data, &aliases); err == nil {
		if aliases.DefaultPath != "" {
			cfg.OutputDir = aliases.DefaultPath
		}
		if aliases.OutputFolder != "" {
			cfg.OutputDir = aliases.OutputFolder
		}
		if aliases.Output != "" {
			ext := filepath.Ext(aliases.Output)
			if ext == ".md" || ext == ".markdown" {
				cfg.OutputFile = aliases.Output
			} else {
				cfg.OutputDir = aliases.Output
			}
		}
		if cfg.FilenameTemplate == DefaultFilenameTemplate && aliases.NameTemplate != "" {
			cfg.FilenameTemplate = aliases.NameTemplate
		}
		if cfg.MeetingURL == "" && aliases.URL != "" {
			cfg.MeetingURL = aliases.URL
		}
		cfg.DefaultAccount = strings.TrimSpace(cfg.DefaultAccount)
		if cfg.DefaultAccount == "" {
			cfg.DefaultAccount = strings.TrimSpace(aliases.EntraAccount)
		}
		if cfg.DefaultAccount == "" {
			cfg.DefaultAccount = strings.TrimSpace(aliases.Account)
		}
		cfg.CaptureMode = strings.ToLower(strings.TrimSpace(cfg.CaptureMode))
		if cfg.CaptureMode == "" && strings.TrimSpace(aliases.Mode) != "" {
			cfg.CaptureMode = strings.ToLower(strings.TrimSpace(aliases.Mode))
		}
		switch {
		case aliases.Headed != nil && *aliases.Headed:
			cfg.Headless = false
		case aliases.Headless != nil:
			cfg.Headless = *aliases.Headless
		default:
			cfg.Headless = false
		}
		if aliases.CaptureAudio != nil || aliases.Audio != nil {
			cfg.ExplicitCaptureAudio = true
		}
		if aliases.Audio != nil {
			cfg.CaptureAudio = *aliases.Audio
		}
		if aliases.Overwrite != nil {
			cfg.Overwrite = *aliases.Overwrite
		} else if aliases.FileOverwrite != nil {
			cfg.Overwrite = *aliases.FileOverwrite
		}
		if strings.EqualFold(cfg.CaptureMode, CaptureModeEphemeralLive) {
			if strings.TrimSpace(cfg.AISummaryDelay) == "" {
				cfg.AISummaryDelay = "0s"
			}
			if !cfg.ExplicitCaptureAudio {
				cfg.CaptureAudio = true
			}
		}
		for i := range cfg.Meetings {
			cfg.Meetings[i].Mode = strings.ToLower(strings.TrimSpace(cfg.Meetings[i].Mode))
			if cfg.Meetings[i].Mode == "" && i < len(aliases.Meetings) && strings.TrimSpace(aliases.Meetings[i].CaptureMode) != "" {
				cfg.Meetings[i].Mode = strings.ToLower(strings.TrimSpace(aliases.Meetings[i].CaptureMode))
			}
			if cfg.Meetings[i].CaptureAudio == nil && cfg.Meetings[i].Audio != nil {
				val := *cfg.Meetings[i].Audio
				cfg.Meetings[i].CaptureAudio = &val
			}
			cfg.Meetings[i].Audio = nil
			if i < len(aliases.Meetings) {
				if aliases.Meetings[i].Overwrite != nil {
					val := *aliases.Meetings[i].Overwrite
					cfg.Meetings[i].Overwrite = &val
				} else if aliases.Meetings[i].FileOverwrite != nil {
					val := *aliases.Meetings[i].FileOverwrite
					cfg.Meetings[i].Overwrite = &val
				}
			}
			cfg.Meetings[i].FileOverwrite = nil
			if cfg.Meetings[i].OutputDir != "" {
				cfg.Meetings[i].OutputDir = expandUserPath(cfg.Meetings[i].OutputDir)
			}
		}
	}
	if cfg.FilenameTemplate == "" {
		cfg.FilenameTemplate = DefaultFilenameTemplate
	}
	cfg.OutputDir = expandUserPath(cfg.OutputDir)
	cfg.OutputFile = expandUserPath(cfg.OutputFile)
	cfg.ProfileDir = expandUserPath(cfg.ProfileDir)
	cfg.BrowserPath = expandUserPath(cfg.BrowserPath)
	cfg.QueueFile = expandUserPath(cfg.QueueFile)
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
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("config: sync temp file: %w", err)
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
