package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	helpNoteMarker = "is not a subcommand or a valid Teams meeting URL"
	helpNoteTail   = ". Replace [command] with one of the subcommands below " +
		"(for example: teams-cli join --help, teams-cli captions --help, " +
		"teams-cli schedule --help, teams-cli auth --help, " +
		"teams-cli generate-config --help, or teams-cli update --help).\n\n"
)

var (
	captureProtocols = []string{"auto", "ephemeral-live", "live-with-summary", "past-recap"}

	outputPrecedenceLevels = []string{
		"1. positional arg[1]",
		"2. --output-file",
		"3. --output\n",
		"4. --output-dir",
		"5. --output-folder",
		"6. config output_file",
		"7. config output_folder",
		"8. config output_dir",
	}

	filenamePlaceholders = []string{
		"<category>", "<concat-meeting-name>", "<meeting-name>", "<meeting-id>",
		"<YYYYMMDD>", "<YYYY-MM-DD>", "<HHMMSS>", "<timestamp>",
	}
)

// runHelp executes a fresh root command with args and returns everything the
// command wrote to its standard output.
func runHelp(t *testing.T, args ...string) string {
	t.Helper()
	root := newRootCommand(depsWith(&fakeRunner{}))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(args)
	require.NoError(t, root.ExecuteContext(context.Background()), "args: %v", args)
	return out.String()
}

func concat(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

func TestAllSubcommandsProvideComprehensiveHelpAndExamples(t *testing.T) {
	captureHelp := concat(captureProtocols, outputPrecedenceLevels, filenamePlaceholders, []string{"--queue"})

	tests := []struct {
		name    string
		sub     string
		phrases []string
	}{
		{
			name: "root",
			sub:  "",
			phrases: concat(captureProtocols, []string{
				"./config.yaml", "config.yaml next to the executable", "teams-cli join --help",
			}),
		},
		{
			name: "join",
			sub:  "join",
			phrases: concat(captureHelp, []string{
				"<meeting-url>", "--url", "meeting_url",
				"--duration", "--idle-timeout", "--lobby-timeout",
				"--ai-summary-delay", "--no-ai-summary", "--append-ai-summary", "--not-before",
				"::",
			}),
		},
		{
			name: "captions",
			sub:  "captions",
			phrases: concat(captureHelp, []string{
				"[meeting-url]", defaultTeamsCaptureURL, "::",
			}),
		},
		{
			name: "auth",
			sub:  "auth",
			phrases: []string{
				"--profile-dir", `%LOCALAPPDATA%\teams-cli\profile`, "MFA",
				"--account", "default_account", "exclusive access",
			},
		},
		{
			name: "schedule",
			sub:  "schedule",
			phrases: []string{
				"5-field cron", "RFC3339", "YYYY-MM-DD HH:MM", "busy",
				"--ai-summary-delay", "queue_file", "--append-ai-summary",
				"applyScheduleFlags", "buildScheduleEntryConfig",
				"per-meeting settings in config.yaml override global schedule flags",
				"ephemeral-live", "explicit per-meeting ai_summary_delay and capture_audio",
			},
		},
		{
			name: "update",
			sub:  "update",
			phrases: []string{
				"teams-cli-<goos>-<goarch>", "SHA256SUMS", "<exe>.old",
				"<exe>.lock", "<exe>.lock.ctl", "--check", "--version", "--force",
				"Manual lock recovery",
			},
		},
		{
			name: "generate-config",
			sub:  "generate-config",
			phrases: []string{
				"init-config", "gen-config", "--output-dir", "-d", "--name", "-n", "--force",
				".yaml", ".yml", "--config", "-f",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var invocations [][]string
			if tc.sub == "" {
				invocations = [][]string{{"--help"}}
			} else {
				invocations = [][]string{{tc.sub, "--help"}, {"help", tc.sub}}
			}

			root := newRootCommand(depsWith(&fakeRunner{}))
			cmd := root
			if tc.sub != "" {
				found, _, err := root.Find([]string{tc.sub})
				require.NoError(t, err)
				cmd = found
			}
			require.Greater(t, len(cmd.Long), len(cmd.Short)+200, "Long must be richer than Short")
			require.NotEmpty(t, cmd.Example, "Example must be populated")
			assert.NotContains(t, cmd.Long, "Examples:", "Long must not carry its own Examples label")
			assert.False(t, strings.HasPrefix(strings.TrimSpace(cmd.Example), "Examples:"),
				"Example must not repeat the Examples: label")
			for _, line := range strings.Split(cmd.Example, "\n") {
				if line != "" {
					assert.True(t, strings.HasPrefix(line, "  "), "example line %q must be 2-space indented", line)
				}
			}

			for _, args := range invocations {
				out := runHelp(t, args...)
				assert.Greater(t, len(out), len(cmd.Short)+400, "args %v: help must exceed Short summary", args)
				assert.Equal(t, 1, strings.Count(out, "Examples:"), "args %v: exactly one Examples: header", args)
				assert.NotContains(t, out, "Examples:\nExamples:")
				assert.NotContains(t, out, helpNoteMarker, "args %v: no placeholder banner", args)
				for _, phrase := range tc.phrases {
					assert.Contains(t, out, phrase, "args %v", args)
				}
			}
		})
	}
}

func TestRootHelpPlaceholderAndUnknownCommandGuidance(t *testing.T) {
	banner := func(arg string) string {
		return `Note: "` + arg + `" ` + helpNoteMarker + helpNoteTail
	}

	t.Run("rejected arguments print the banner before root help", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
			arg  string
		}{
			{"placeholder with help flag", []string{"[command]", "--help"}, "[command]"},
			{"angle placeholder", []string{"<command>", "--help"}, "<command>"},
			{"unknown subcommand", []string{"unknown-subcmd", "--help"}, "unknown-subcmd"},
			{"help flag first", []string{"--help", "[command]"}, "[command]"},
			{"config flag and value skipped", []string{"-f", "config.yaml", "[command]", "--help"}, "[command]"},
			{"help subcommand placeholder", []string{"help", "[command]"}, "[command]"},
			{"help subcommand unknown", []string{"help", "unknown-subcmd"}, "unknown-subcmd"},
			{"help config flag value skipped", []string{"help", "-f", "config.yaml", "[command]"}, "[command]"},
			{"help long config flag value skipped", []string{"help", "--config", "config.yaml", "[command]"}, "[command]"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out := runHelp(t, tc.args...)
				require.True(t, strings.HasPrefix(out, banner(tc.arg)),
					"args %v: output must start with the banner, got:\n%.400s", tc.args, out)
				rest := strings.TrimPrefix(out, banner(tc.arg))
				assert.Contains(t, rest, "Usage:")
				assert.Contains(t, rest, "ephemeral-live", "root help follows the banner")
				assert.Equal(t, 1, strings.Count(out, helpNoteMarker), "banner printed exactly once")
			})
		}
	})

	t.Run("help subcommand placeholder lists the version flag like --help", func(t *testing.T) {
		for _, args := range [][]string{{"--help"}, {"help", "[command]"}} {
			out := runHelp(t, args...)
			assert.Contains(t, out, "--version", "args %v", args)
		}
	})

	t.Run("leading help flag renders the named subcommand help", func(t *testing.T) {
		for _, args := range [][]string{{"--help", "join"}, {"-f", "config.yaml", "--help", "join"}} {
			out := runHelp(t, args...)
			assert.NotContains(t, out, helpNoteMarker, "args %v", args)
			assert.Contains(t, out, "--not-before", "args %v: join help expected", args)
			assert.Contains(t, out, "Examples:", "args %v", args)
		}
		out := runHelp(t, "--help", "init-config")
		assert.NotContains(t, out, helpNoteMarker)
		assert.Contains(t, out, "--force")
	})

	t.Run("schemeless URL credentials are never echoed", func(t *testing.T) {
		for _, raw := range []string{
			"teams.microsoft.com/meet/123?p=secretpass",
			"user:secret@teams.microsoft.com/meet/123",
			"//user:secret@teams.microsoft.com/meet/123#secretpass",
			"//user:secret?pw@teams.microsoft.com/meet/123",
			"user:secret-https:pw@teams.microsoft.com/meet/123",
			"secret-http:pw",
			"teams.microsoft.com/meet/123?p=secretpass#secretfrag",
		} {
			out := runHelp(t, raw, "--help")
			assert.Contains(t, out, helpNoteMarker, "raw %q", raw)
			assert.Contains(t, out, `Note: "`+redactedHelpArg+`" `, "raw %q", raw)
			assert.NotContains(t, out, "secret", "raw %q", raw)
			assert.NotContains(t, out, "?p=", "raw %q", raw)
		}
	})

	t.Run("benign rejected arguments are still echoed", func(t *testing.T) {
		assert.Equal(t, "[command]", redactHelpArg("[command]"))
		assert.Equal(t, "unknown-subcmd", redactHelpArg("unknown-subcmd"))
		assert.Equal(t, "http://teams.microsoft.com/meet/123",
			redactHelpArg("http://user:secret@teams.microsoft.com/meet/123?p=secretpass"))
	})

	t.Run("rejected URL is redacted", func(t *testing.T) {
		out := runHelp(t, "http://user:secret@teams.microsoft.com/meet/123?p=secretpass", "--help")
		require.True(t, strings.HasPrefix(out, banner("http://teams.microsoft.com/meet/123")),
			"banner must use the redacted URL, got:\n%.400s", out)
		assert.NotContains(t, out, "user:secret")
		assert.NotContains(t, out, "secretpass")
	})

	t.Run("no banner without a rejected positional argument", func(t *testing.T) {
		tests := []struct {
			name string
			args []string
		}{
			{"help flag only", []string{"--help"}},
			{"config flag value is not positional", []string{"-f", "config.yaml", "--help"}},
			{"valid Teams meeting URL", []string{"https://teams.microsoft.com/meet/synthetic", "--help"}},
			{"subcommand positional", []string{"generate-config", "./custom-dir", "--help"}},
			{"help subcommand without topic", []string{"help"}},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				out := runHelp(t, tc.args...)
				assert.NotContains(t, out, helpNoteMarker)
				assert.NotEmpty(t, out)
			})
		}
	})
}
