package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// canonicalTemplateKeys lists the top-level keys whose "# key: value" template
// lines are canonical, uncommentable examples. Alias and alternative keys are
// documented only as prose so uncommenting every canonical line never produces
// conflicting overrides.
var canonicalTemplateKeys = []string{
	"output_dir", "filename_template", "category", "meeting_url", "profile_dir",
	"browser_path", "default_account", "bot_name", "headless", "capture_audio",
	"capture_mode", "duration", "idle_timeout", "ai_summary_delay",
	"lobby_timeout", "queue_file",
}

// documentedTemplateKeys lists every configuration key and alias the template
// must explain.
var documentedTemplateKeys = []string{
	"output_dir", "output_folder", "default_path", "output_file", "output",
	"filename_template", "name_template", "category", "meeting_url", "url",
	"profile_dir", "browser_path", "default_account", "entra_account", "account",
	"bot_name", "headless", "headed", "capture_audio", "audio", "capture_mode",
	"mode", "duration", "idle_timeout", "ai_summary_delay", "lobby_timeout",
	"queue_file", "meetings", "cron", "at",
}

var canonicalLineRe = regexp.MustCompile(`^# (` + strings.Join(canonicalTemplateKeys, "|") + `):`)

// uncommentCanonical activates the canonical "# key:" lines and the whole
// "# meetings:" example block (through the next blank line) of tpl.
func uncommentCanonical(tpl string) string {
	var out []string
	inMeetings := false
	for _, line := range strings.Split(strings.ReplaceAll(tpl, "\r\n", "\n"), "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			inMeetings = false
			out = append(out, line)
		case line == "# meetings:":
			inMeetings = true
			out = append(out, strings.TrimPrefix(line, "# "))
		case inMeetings:
			out = append(out, strings.TrimPrefix(line, "# "))
		case canonicalLineRe.MatchString(line):
			out = append(out, strings.TrimPrefix(line, "# "))
		default:
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestTemplateYAMLContentAndLoad(t *testing.T) {
	tpl := TemplateYAML()
	require.NotEmpty(t, tpl)

	t.Run("documents every key and alias", func(t *testing.T) {
		for _, key := range documentedTemplateKeys {
			assert.Containsf(t, tpl, key, "template must document %q", key)
		}
		for _, placeholder := range []string{
			"<category>", "<concat-meeting-name>", "<meeting-name>", "<meeting-id>",
			"<YYYYMMDD>", "<YYYY-MM-DD>", "<HHMMSS>", "<timestamp>",
		} {
			assert.Containsf(t, tpl, placeholder, "template must explain placeholder %s", placeholder)
		}
		for _, mode := range []string{"auto", "ephemeral-live", "live-with-summary", "past-recap"} {
			assert.Containsf(t, tpl, mode, "template must explain capture mode %s", mode)
		}
	})

	t.Run("documents every Config and MeetingSchedule yaml key", func(t *testing.T) {
		for _, typ := range []reflect.Type{reflect.TypeOf(Config{}), reflect.TypeOf(MeetingSchedule{})} {
			for i := range typ.NumField() {
				tag := typ.Field(i).Tag.Get("yaml")
				key, _, _ := strings.Cut(tag, ",")
				if key == "" || key == "-" {
					continue
				}
				assert.Containsf(t, tpl, key, "template must document %s yaml key %q", typ.Name(), key)
			}
		}
	})

	t.Run("has no active lines and no credential-shaped content", func(t *testing.T) {
		for i, line := range strings.Split(strings.ReplaceAll(tpl, "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			assert.Truef(t, trimmed == "" || strings.HasPrefix(trimmed, "#"), "line %d must be blank or a comment: %q", i+1, line)
		}
		assert.Empty(t, scanTrackedContent("TemplateYAML", []byte(tpl)))
	})

	t.Run("raw template loads with defaults", func(t *testing.T) {
		want, err := DefaultConfig()
		require.NoError(t, err)
		got, err := Load(writeTestConfig(t, tpl))
		require.NoError(t, err)
		assert.Equal(t, want.OutputDir, got.OutputDir)
		assert.Equal(t, want.ProfileDir, got.ProfileDir)
		assert.Equal(t, want.BotName, got.BotName)
		assert.Equal(t, want.FilenameTemplate, got.FilenameTemplate)
		assert.False(t, got.Headless, "all-commented config opens a visible browser")
		assert.Empty(t, got.Meetings)
	})

	t.Run("uncommented canonical examples load and validate", func(t *testing.T) {
		active := uncommentCanonical(tpl)
		got, err := Load(writeTestConfig(t, active))
		require.NoError(t, err)
		require.NoError(t, got.Validate())

		assert.Equal(t, expandUserPath(`C:\Transcripts\Team`), got.OutputDir)
		assert.Equal(t, DefaultFilenameTemplate, got.FilenameTemplate)
		assert.Equal(t, "TEAM", got.Category)
		assert.Equal(t, "https://teams.microsoft.com/meet/synthetic-meeting", got.MeetingURL)
		assert.Equal(t, expandUserPath(`%LOCALAPPDATA%\teams-cli\profile`), got.ProfileDir)
		assert.Equal(t, expandUserPath(`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`), got.BrowserPath)
		assert.Equal(t, "alice@contoso.com", got.DefaultAccount)
		assert.Equal(t, "Transcription Agent", got.BotName)
		assert.True(t, got.Headless)
		assert.False(t, got.CaptureAudio)
		assert.Equal(t, CaptureModeAuto, got.CaptureMode)
		assert.Equal(t, "1h", got.Duration)
		assert.Equal(t, "3m", got.IdleTimeout)
		assert.Equal(t, "30m", got.AISummaryDelay)
		assert.Equal(t, "5m", got.LobbyTimeout)
		assert.Equal(t, expandUserPath(`C:\Transcripts\Team\transcription_queue.txt`), got.QueueFile)

		require.Len(t, got.Meetings, 2)
		recurring, oneShot := got.Meetings[0], got.Meetings[1]
		assert.Equal(t, "Daily Standup", recurring.Name)
		assert.Equal(t, "https://teams.microsoft.com/meet/synthetic-standup", recurring.URL)
		assert.Equal(t, "0 9 * * 1-5", recurring.Cron)
		assert.Empty(t, recurring.At)
		assert.Equal(t, CaptureModeLiveWithSummary, recurring.Mode)
		assert.Equal(t, "30m", recurring.Duration)
		assert.Equal(t, "TEAM", recurring.Category)
		require.NotNil(t, recurring.CaptureAudio)
		assert.False(t, *recurring.CaptureAudio)

		assert.Equal(t, "Architecture Review", oneShot.Name)
		assert.Equal(t, "2026-11-15 14:00", oneShot.At)
		assert.Empty(t, oneShot.Cron)
		assert.Equal(t, CaptureModeEphemeralLive, oneShot.Mode)
		assert.Equal(t, "ARCH", oneShot.Category)
		require.NotNil(t, oneShot.CaptureAudio)
		assert.True(t, *oneShot.CaptureAudio)
		assert.Equal(t, expandUserPath(`C:\Transcripts\Architecture`), oneShot.OutputDir)
	})
}

func TestResolveTemplatePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	localAppData := t.TempDir()
	t.Setenv("LOCALAPPDATA", localAppData)
	dir := t.TempDir()

	valid := []struct {
		name      string
		outputDir string
		fileName  string
		want      string
	}{
		{"defaults", "", "", filepath.Join(".", "config.yaml")},
		{"whitespace output dir defaults to cwd", "   ", "", filepath.Join(".", "config.yaml")},
		{"custom dir", dir, "", filepath.Join(dir, "config.yaml")},
		{"whitespace padded dir", "  " + dir + "  ", "", filepath.Join(dir, "config.yaml")},
		{"windows env var dir", `%LOCALAPPDATA%\cfg`, "", filepath.Join(localAppData, "cfg", "config.yaml")},
		{"tilde subdir", "~/cfg", "", filepath.Join(home, "cfg", "config.yaml")},
		{"bare tilde", "~", "", filepath.Join(home, "config.yaml")},
		{"yaml name", dir, "team.yaml", filepath.Join(dir, "team.yaml")},
		{"yml name", dir, "team.yml", filepath.Join(dir, "team.yml")},
		{"upper YAML name keeps extension", dir, "TEAM.YAML", filepath.Join(dir, "TEAM.YAML")},
		{"upper YML name keeps extension", dir, "TEAM.YML", filepath.Join(dir, "TEAM.YML")},
		{"extensionless name appends yaml", dir, "my-team", filepath.Join(dir, "my-team.yaml")},
		{"other extension appends yaml", dir, "my.team", filepath.Join(dir, "my.team.yaml")},
		{"padded name is trimmed", dir, "  team  ", filepath.Join(dir, "team.yaml")},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveTemplatePath(tc.outputDir, tc.fileName)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	invalid := []struct {
		name     string
		fileName string
	}{
		{"whitespace only", "   "},
		{"dot", "."},
		{"dot dot", ".."},
		{"parent traversal", "../evil.yaml"},
		{"embedded dot dot", "a..b.yaml"},
		{"forward slash", "sub/config.yaml"},
		{"backslash", `sub\config.yaml`},
		{"drive relative", "C:config.yaml"},
		{"drive absolute", `C:\config.yaml`},
		{"alternate data stream", "config.yaml:extra"},
		{"bare yaml", ".yaml"},
		{"bare yml", ".yml"},
		{"bare upper yaml", ".YAML"},
		{"trailing dot", "config."},
		{"reserved NUL", "NUL"},
		{"reserved con with extension", "con.yaml"},
		{"reserved COM1 yml", "COM1.yml"},
		{"reserved lowercase lpt9", "lpt9"},
	}
	for _, tc := range invalid {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			got, err := ResolveTemplatePath(dir, tc.fileName)
			require.ErrorIs(t, err, ErrInvalidConfigName)
			assert.Empty(t, got)
		})
	}
}

func listTempArtifacts(t *testing.T, dir string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".config-template-*.yaml.tmp"))
	require.NoError(t, err)
	return matches
}

func TestWriteTemplate(t *testing.T) {
	t.Run("creates nested directories with owner-only modes", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "a", "b", "config.yaml")
		require.NoError(t, WriteTemplate(path, false))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, TemplateYAML(), string(data))
		_, err = Load(path)
		require.NoError(t, err)

		if runtime.GOOS != "windows" {
			fileInfo, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
			for _, d := range []string{filepath.Join(root, "a"), filepath.Join(root, "a", "b")} {
				dirInfo, err := os.Stat(d)
				require.NoError(t, err)
				assert.Equalf(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "dir %s", d)
			}
		}
	})

	t.Run("refuses to overwrite without force", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))

		err := WriteTemplate(path, false)
		require.ErrorIs(t, err, ErrConfigExists)
		assert.Contains(t, err.Error(), path)

		data, readErr := os.ReadFile(path)
		require.NoError(t, readErr)
		assert.Equal(t, "original", string(data))
	})

	t.Run("force replaces existing file atomically", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.WriteFile(path, []byte("original"), 0o644))

		require.NoError(t, WriteTemplate(path, true))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, TemplateYAML(), string(data))
		assert.Empty(t, listTempArtifacts(t, dir))
		if runtime.GOOS != "windows" {
			info, err := os.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
		}
	})

	t.Run("force creates a missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "new", "config.yaml")
		require.NoError(t, WriteTemplate(path, true))
		_, err := os.Stat(path)
		require.NoError(t, err)
	})

	t.Run("relative path in working directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)

		require.NoError(t, WriteTemplate("config.yaml", false))
		require.ErrorIs(t, WriteTemplate("config.yaml", false), ErrConfigExists)
		require.NoError(t, WriteTemplate("config.yaml", true))
		assert.Empty(t, listTempArtifacts(t, dir))
	})

	t.Run("force failure cleans up temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "config.yaml")
		require.NoError(t, os.MkdirAll(filepath.Join(path, "keep"), 0o700))

		require.Error(t, WriteTemplate(path, true))
		assert.Empty(t, listTempArtifacts(t, dir))
	})

	t.Run("rejects empty path", func(t *testing.T) {
		require.ErrorIs(t, WriteTemplate("", false), ErrInvalidConfigName)
		require.ErrorIs(t, WriteTemplate("   ", true), ErrInvalidConfigName)
	})
}

func TestExpandPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("EXPAND_PATH_TEST", "expanded")

	assert.Equal(t, expandUserPath("~/cfg"), ExpandPath("  ~/cfg  "))
	assert.Equal(t, "expanded", ExpandPath("$EXPAND_PATH_TEST"))
	assert.Empty(t, ExpandPath("   "))
}

func TestExampleConfigMatchesTemplateYAML(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(findRepoRoot(t), "config.example.yaml"))
	require.NoError(t, err)
	normalize := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }
	assert.Equal(t, normalize(TemplateYAML()), normalize(string(data)),
		"config.example.yaml must equal TemplateYAML(); regenerate with: teams-cli generate-config -d . -n config.example.yaml --force")
}
