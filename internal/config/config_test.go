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
	assert.Contains(t, cfg.OutputDir, filepath.Join("go-teams", "transcripts"))
	assert.NotContains(t, cfg.ProfileDir, filepath.Join("bin", "profile"), "default profile dir should live under the user profile, not workspace bin/profile")
	assert.Contains(t, cfg.ProfileDir, filepath.Join("go-teams", "profile"))
	assert.Equal(t, "Transcription Agent", cfg.BotName)
	assert.True(t, cfg.Headless)
	assert.NoError(t, cfg.Validate())
}

func TestDefaultConfigPath(t *testing.T) {
	path, err := DefaultConfigPath()
	require.NoError(t, err)
	assert.Equal(t, "config.yaml", filepath.Base(path))
	assert.Contains(t, path, filepath.Join("go-teams", "config.yaml"))
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
		BrowserPath:      filepath.Join(dir, "browser", "chrome"),
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
output_folder: C:\Transcripts\WSP
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
	assert.Equal(t, filepath.FromSlash("C:/Transcripts/WSP"), got.OutputDir)
	assert.Equal(t, "<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md", got.FilenameTemplate)
	assert.Equal(t, filepath.FromSlash("C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe"), got.BrowserPath)
	assert.Equal(t, filepath.FromSlash("C:/Transcripts/profile"), got.ProfileDir)
	assert.Equal(t, "https://teams.cloud.microsoft/l/meetingrecap?threadId=1", got.MeetingURL)
	assert.True(t, got.Headless)
}

func TestLoadExpandsWindowsUserVariables(t *testing.T) {
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	dir := t.TempDir()
	path := filepath.Join(dir, "user-vars.yaml")
	yamlContent := `
output_folder: "%USER%\\Documents\\Transcripts"
profile_dir: "%USERPROFILE%\\AppData\\Local\\go-teams\\profile"
`
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))
	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "Documents", "Transcripts"), got.OutputDir)
	assert.Equal(t, filepath.Join(home, "AppData", "Local", "go-teams", "profile"), got.ProfileDir)
}

func TestLoadDefaultAccountAndAliases(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "default_account primary key trimmed",
			yaml: "default_account: \"  alice@contoso.com  \"\n",
			want: "alice@contoso.com",
		},
		{
			name: "entra_account alias trimmed",
			yaml: "entra_account: \"  bob@contoso.com\\t\"\n",
			want: "bob@contoso.com",
		},
		{
			name: "account alias trimmed",
			yaml: "account: \"  carol@contoso.com \"\n",
			want: "carol@contoso.com",
		},
		{
			name: "default_account wins over entra_account and account",
			yaml: "default_account: primary@contoso.com\nentra_account: secondary@contoso.com\naccount: tertiary@contoso.com\n",
			want: "primary@contoso.com",
		},
		{
			name: "entra_account wins over account when default_account empty",
			yaml: "default_account: \"   \"\nentra_account: secondary@contoso.com\naccount: tertiary@contoso.com\n",
			want: "secondary@contoso.com",
		},
		{
			name: "omitted leaves DefaultAccount empty",
			yaml: "bot_name: Bot\n",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0o600))
			got, err := Load(path)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.DefaultAccount)
		})
	}

	t.Run("Save and Load round-trip preserves DefaultAccount", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "roundtrip.yaml")
		want := Config{
			OutputDir:        filepath.Join(dir, "out"),
			FilenameTemplate: DefaultFilenameTemplate,
			ProfileDir:       filepath.Join(dir, "prof"),
			BotName:          "Bot",
			DefaultAccount:   "alice@contoso.com",
		}
		require.NoError(t, Save(path, want))
		got, err := Load(path)
		require.NoError(t, err)
		assert.Equal(t, "alice@contoso.com", got.DefaultAccount)
	})
}

func TestLoadAliasAndOutputPrecedence(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		check func(t *testing.T, got Config)
	}{
		{
			name: "output ending in .md sets OutputFile and overrides output_file",
			yaml: "output_file: C:\\old\\ignored.md\noutput: C:\\new\\chosen.md\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, filepath.FromSlash("C:/new/chosen.md"), got.OutputFile)
			},
		},
		{
			name: "output ending in .markdown sets OutputFile",
			yaml: "output: C:\\new\\chosen.markdown\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, filepath.FromSlash("C:/new/chosen.markdown"), got.OutputFile)
			},
		},
		{
			name: "output directory overrides output_folder and default_path",
			yaml: "default_path: C:\\one\noutput_folder: C:\\two\noutput: C:\\three\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, filepath.FromSlash("C:/three"), got.OutputDir)
			},
		},
		{
			name: "output_folder overrides default_path and output_dir",
			yaml: "output_dir: C:\\one\ndefault_path: C:\\two\noutput_folder: C:\\three\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, filepath.FromSlash("C:/three"), got.OutputDir)
			},
		},
		{
			name: "default_path overrides output_dir when output_folder omitted",
			yaml: "output_dir: C:\\one\ndefault_path: C:\\two\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, filepath.FromSlash("C:/two"), got.OutputDir)
			},
		},
		{
			name: "headed true forces Headless false even when headless true is set",
			yaml: "headless: true\nheaded: true\n",
			check: func(t *testing.T, got Config) {
				assert.False(t, got.Headless)
			},
		},
		{
			name: "headless false sets Headless false",
			yaml: "headless: false\n",
			check: func(t *testing.T, got Config) {
				assert.False(t, got.Headless)
			},
		},
		{
			name: "omitting both headless and headed defaults Headless to false in Load",
			yaml: "bot_name: Bot\n",
			check: func(t *testing.T, got Config) {
				assert.False(t, got.Headless)
			},
		},
		{
			name: "audio true overrides capture_audio false",
			yaml: "capture_audio: false\naudio: true\n",
			check: func(t *testing.T, got Config) {
				assert.True(t, got.CaptureAudio)
			},
		},
		{
			name: "audio false overrides capture_audio true",
			yaml: "capture_audio: true\naudio: false\n",
			check: func(t *testing.T, got Config) {
				assert.False(t, got.CaptureAudio)
			},
		},
		{
			name: "explicit filename_template wins over name_template",
			yaml: "filename_template: \"custom-<YYYYMMDD>.md\"\nname_template: \"ignored-<YYYYMMDD>.md\"\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, "custom-<YYYYMMDD>.md", got.FilenameTemplate)
			},
		},
		{
			name: "explicit empty filename_template restores DefaultFilenameTemplate",
			yaml: "filename_template: \"\"\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, DefaultFilenameTemplate, got.FilenameTemplate)
			},
		},
		{
			name: "explicit meeting_url wins over url alias",
			yaml: "meeting_url: https://teams.microsoft.com/meet/primary\nurl: https://teams.microsoft.com/meet/secondary\n",
			check: func(t *testing.T, got Config) {
				assert.Equal(t, "https://teams.microsoft.com/meet/primary", got.MeetingURL)
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0o600))
			got, err := Load(path)
			require.NoError(t, err)
			tc.check(t, got)
		})
	}
}

func TestExpandUserPathAndDefaultProfileDir(t *testing.T) {
	assert.Equal(t, "", expandUserPath(""))

	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "transcripts"), expandUserPath("~/transcripts"))

	t.Setenv("CUSTOM_TEAMS_DIR", `C:\CustomTeams`)
	assert.Equal(t, filepath.FromSlash("C:/CustomTeams/out"), expandUserPath(`%CUSTOM_TEAMS_DIR%\out`))

	assert.Contains(t, expandUserPath(`%UNSET_TEAMS_VAR_XYZ%\out`), "%UNSET_TEAMS_VAR_XYZ%")

	t.Setenv("LOCALAPPDATA", "")
	assert.Equal(t, filepath.Join(`C:\AppDir`, "profile"), defaultProfileDir(`C:\AppDir`))
}

func TestCaptureModeValidationAndAliases(t *testing.T) {
	for _, m := range []string{
		CaptureModeAuto,
		CaptureModeEphemeralLive,
		CaptureModeLiveWithSummary,
		CaptureModePastRecap,
	} {
		assert.True(t, IsValidCaptureMode(m), "expected %q to be valid", m)
	}
	assert.False(t, IsValidCaptureMode("bogus-mode"))

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yamlContent := `
mode: live-with-summary
duration: 45m
idle_timeout: 3m
ai_summary_delay: 30m
lobby_timeout: 5m
queue_file: "~/queue.txt"
meetings:
  - name: "One-shot"
    url: "https://teams.microsoft.com/meet/123"
    at: "2026-11-15T14:00:00Z"
    capture_mode: ephemeral-live
    duration: 30m
    idle_timeout: 2m
    ai_summary_delay: 0s
    lobby_timeout: 4m
    category: "ENG"
    audio: true
    output_dir: "~/eng-out"
`
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0o600))
	got, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t, CaptureModeLiveWithSummary, got.CaptureMode)
	assert.Equal(t, "45m", got.Duration)
	assert.Equal(t, "3m", got.IdleTimeout)
	assert.Equal(t, "30m", got.AISummaryDelay)
	assert.Equal(t, "5m", got.LobbyTimeout)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, "queue.txt"), got.QueueFile)
	require.Len(t, got.Meetings, 1)
	assert.Equal(t, "2026-11-15T14:00:00Z", got.Meetings[0].At)
	assert.Equal(t, CaptureModeEphemeralLive, got.Meetings[0].Mode)
	assert.Equal(t, "30m", got.Meetings[0].Duration)
	assert.Equal(t, "2m", got.Meetings[0].IdleTimeout)
	assert.Equal(t, "0s", got.Meetings[0].AISummaryDelay)
	assert.Equal(t, "4m", got.Meetings[0].LobbyTimeout)
	assert.Equal(t, "ENG", got.Meetings[0].Category)
	require.NotNil(t, got.Meetings[0].CaptureAudio)
	assert.True(t, *got.Meetings[0].CaptureAudio)
	assert.Nil(t, got.Meetings[0].Audio)
	assert.Equal(t, filepath.Join(home, "eng-out"), got.Meetings[0].OutputDir)

	base := func() Config {
		return Config{OutputDir: "out", ProfileDir: "prof", BotName: "bot"}
	}
	errCases := []struct {
		name    string
		mutate  func(*Config)
		wantErr error
	}{
		{"invalid top-level capture_mode", func(c *Config) { c.CaptureMode = "invalid" }, ErrInvalidCaptureMode},
		{"invalid top-level duration", func(c *Config) { c.Duration = "0s" }, ErrInvalidDuration},
		{"invalid top-level idle_timeout", func(c *Config) { c.IdleTimeout = "-1m" }, ErrInvalidDuration},
		{"invalid top-level ai_summary_delay", func(c *Config) { c.AISummaryDelay = "-5m" }, ErrInvalidDuration},
		{"invalid top-level lobby_timeout", func(c *Config) { c.LobbyTimeout = "not-a-duration" }, ErrInvalidDuration},
		{"meeting both cron and at", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", At: "2026-11-15T14:00:00Z"}}
		}, ErrMeetingScheduleConflict},
		{"meeting invalid mode", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", Mode: "bad"}}
		}, ErrInvalidCaptureMode},
		{"meeting invalid duration", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", Duration: "0s"}}
		}, ErrInvalidDuration},
		{"meeting invalid idle_timeout", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", IdleTimeout: "0s"}}
		}, ErrInvalidDuration},
		{"meeting invalid ai_summary_delay", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", AISummaryDelay: "-1s"}}
		}, ErrInvalidDuration},
		{"meeting invalid lobby_timeout", func(c *Config) {
			c.Meetings = []MeetingSchedule{{URL: "https://x", Cron: "* * * * *", LobbyTimeout: "bad"}}
		}, ErrInvalidDuration},
	}
	for _, tc := range errCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base()
			tc.mutate(&cfg)
			assert.ErrorIs(t, cfg.Validate(), tc.wantErr)
		})
	}
}
