package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.OutputDir)
	assert.NotEmpty(t, cfg.ProfileDir)
	assert.Equal(t, "Transcription Agent", cfg.BotName)
	assert.True(t, cfg.Headless)
	assert.NoError(t, cfg.Validate())
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	require.NoError(t, err)
	assert.Equal(t, "config.yaml", filepath.Base(path))
}

func TestConfigValidate(t *testing.T) {
	base := func() Config {
		return Config{OutputDir: "out", ProfileDir: "prof", BotName: "bot"}
	}
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr error
	}{
		{"valid", func(*Config) {}, nil},
		{"empty output dir", func(c *Config) { c.OutputDir = "" }, ErrEmptyOutputDir},
		{"empty profile dir", func(c *Config) { c.ProfileDir = "" }, ErrEmptyProfileDir},
		{"empty bot name", func(c *Config) { c.BotName = "" }, ErrEmptyBotName},
		{"meeting missing url", func(c *Config) {
			c.Meetings = []MeetingSchedule{{Cron: "* * * * *"}}
		}, ErrMeetingMissingURL},
		{"meeting missing cron", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x"}}
		}, ErrMeetingMissingCron},
		{"valid meeting", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *"}}
		}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	want := Config{
		OutputDir:        filepath.Join(dir, "out"),
		FilenameTemplate: DefaultFilenameTemplate,
		ProfileDir:       filepath.Join(dir, "prof"),
		BotName:          "Bot",
		Headless:         false,
		Meetings: []MeetingSchedule{
			{Name: "standup", URL: "https://teams.microsoft.com/l/meetup-join/x", Cron: "0 9 * * 1-5"},
		},
	}
	require.NoError(t, Save(path, want))

	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := Config{OutputDir: "out", ProfileDir: "prof", BotName: "bot"}
	require.NoError(t, Save(path, cfg))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".config-", "temp file should not remain")
	}
}

func TestSaveRejectsInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := Save(path, Config{})
	assert.ErrorIs(t, err, ErrEmptyOutputDir)
	_, statErr := os.Stat(path)
	assert.True(t, errors.Is(statErr, os.ErrNotExist), "no file should be written")
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nope.yaml"))
	assert.Error(t, err)
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("::not yaml::\n\t- broken"), 0o600))
	_, err := Load(path)
	assert.Error(t, err)
}

func TestLoadAppliesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	// Only override bot_name; other fields must fall back to defaults.
	require.NoError(t, os.WriteFile(path, []byte("bot_name: Custom\n"), 0o600))
	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "Custom", got.BotName)
	assert.NotEmpty(t, got.OutputDir)
	assert.NotEmpty(t, got.ProfileDir)
	assert.Equal(t, DefaultFilenameTemplate, got.FilenameTemplate)
}

func TestLoadCustomerYAMLWithTemplateAndAliases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wsp.yaml")
	yamlContent := `
category: WSP
default_path: C:\Transcripts\WSP
name_template: "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"
browser_path: C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe
profile_dir: C:\Transcripts\profile
url: https://teams.cloud.microsoft/l/meetingrecap?threadId=1
headless: true
`
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))
	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, "WSP", got.Category)
	assert.Equal(t, `C:\Transcripts\WSP`, got.OutputDir)
	assert.Equal(t, "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md", got.FilenameTemplate)
	assert.Equal(t, `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`, got.BrowserPath)
	assert.Equal(t, `C:\Transcripts\profile`, got.ProfileDir)
	assert.Equal(t, "https://teams.cloud.microsoft/l/meetingrecap?threadId=1", got.MeetingURL)
	assert.True(t, got.Headless)
}
