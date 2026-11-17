package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/capture"
	"github.com/softwaresalt/go-teams/internal/config"
)

func TestPromptOverwriteApproval(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    bool
		wantErr bool
	}{
		{name: "lowercase y", input: "y\n", want: true},
		{name: "uppercase Y", input: "Y\n", want: true},
		{name: "yes with CRLF", input: "yes\r\n", want: true},
		{name: "uppercase YES", input: "YES\n", want: true},
		{name: "(Y)es", input: "(Y)es\n", want: true},
		{name: "lowercase n", input: "n\n", want: false},
		{name: "uppercase N", input: "N\n", want: false},
		{name: "no", input: "no\n", want: false},
		{name: "(N)o", input: "(N)o\n", want: false},
		{name: "empty EOF", input: "", want: false},
		{name: "other", input: "other\n", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := promptOverwriteApproval(context.Background(), strings.NewReader(tc.input), &out, []string{"a.md", "a.mp3"})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, "File already exists: a.md, a.mp3\nOverwrite? (Y)es | (N)o: ", out.String())
		})
	}

	t.Run("cancelled context aborts prompt immediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var out bytes.Buffer
		got, err := promptOverwriteApproval(ctx, strings.NewReader("y\n"), &out, []string{"a.md"})
		assert.ErrorIs(t, err, context.Canceled)
		assert.False(t, got)
	})
}

func TestOverwriteFlagAndConfigPrecedenceAndQueueInheritance(t *testing.T) {
	for _, name := range []string{"root", "join", "captions"} {
		for _, value := range []string{"true", "false"} {
			t.Run(name+"/overwrite="+value, func(t *testing.T) {
				root := newRootCommand(defaultDeps())
				cmd := root
				if name != "root" {
					var err error
					cmd, _, err = root.Find([]string{name})
					require.NoError(t, err)
				}
				require.NotNil(t, cmd.Flags().Lookup("overwrite"))

				var f captureFlags
				testCmd := &cobra.Command{Use: name}
				bindCaptureFlags(testCmd, &f)
				require.NoError(t, testCmd.Flags().Parse([]string{"--overwrite=" + value}))

				cfg := config.Config{Overwrite: value == "false"}
				applyCaptureFlags(testCmd, &cfg, f)
				assert.Equal(t, value == "true", cfg.Overwrite)

				want := "--overwrite"
				if value == "false" {
					want = "--overwrite=false"
				}
				got := inheritedQueueFlags(testCmd, "", f)
				assert.Equal(t, []string{want}, got)
			})
		}

		t.Run(name+"/unchanged overwrite omitted from inheritedQueueFlags", func(t *testing.T) {
			var f captureFlags
			testCmd := &cobra.Command{Use: name}
			bindCaptureFlags(testCmd, &f)
			require.NoError(t, testCmd.Flags().Parse(nil))
			assert.Empty(t, inheritedQueueFlags(testCmd, "", f))
		})
	}

	t.Run("schedule flag and per-meeting Overwrite precedence", func(t *testing.T) {
		root := newRootCommand(defaultDeps())
		schedCmd, _, err := root.Find([]string{"schedule"})
		require.NoError(t, err)
		require.NotNil(t, schedCmd.Flags().Lookup("overwrite"))

		var sf scheduleFlags
		testSched := &cobra.Command{Use: "schedule"}
		bindScheduleFlags(testSched, &sf)
		require.NoError(t, testSched.Flags().Parse([]string{"--overwrite"}))

		base := validConfig()
		applyScheduleFlags(testSched, &base, sf)
		assert.True(t, base.Overwrite)

		overFalse := false
		entry, err := buildSchedulerEntryFromConfig(config.MeetingSchedule{
			Name:      "standup",
			URL:       "https://teams.microsoft.com/meet/1",
			Cron:      "0 9 * * 1-5",
			Overwrite: &overFalse,
		})
		require.NoError(t, err)
		require.NotNil(t, entry.Overwrite)
		assert.False(t, *entry.Overwrite)

		entryCfg := buildScheduleEntryConfig(base, entry)
		assert.False(t, entryCfg.Overwrite, "per-meeting Overwrite=false must override schedule --overwrite")

		inheritEntry, err := buildSchedulerEntryFromConfig(config.MeetingSchedule{
			Name: "weekly",
			URL:  "https://teams.microsoft.com/meet/2",
			Cron: "0 10 * * 1-5",
		})
		require.NoError(t, err)
		assert.Nil(t, inheritEntry.Overwrite)
		assert.True(t, buildScheduleEntryConfig(base, inheritEntry).Overwrite)
	})
}

type fakeCLIOverwriteMeeting struct{}

func (fakeCLIOverwriteMeeting) Join(context.Context) error { return nil }
func (fakeCLIOverwriteMeeting) EnableCaptions(_ context.Context, sink browser.CaptionSink) error {
	sink("Presenter", "Fresh transcript line.")
	return nil
}
func (fakeCLIOverwriteMeeting) CaptureAudio(context.Context, io.Writer) error { return nil }
func (fakeCLIOverwriteMeeting) IsRecap() bool                                 { return true }

func TestCLIJoinInteractiveOverwritePromptAndDecline(t *testing.T) {
	url := "https://teams.microsoft.com/meet/999888777"

	setupDeps := func(t *testing.T) *deps {
		t.Helper()
		d := defaultDeps()
		d.loadConfig = func(string) (config.Config, error) {
			c := validConfig()
			c.CaptureAudio = false
			return c, nil
		}
		d.newMeeting = func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return fakeCLIOverwriteMeeting{}, ctx, func() {}, nil
		}
		return d
	}

	t.Run("direct join decline preserves existing file and returns ErrOverwriteDeclined", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "transcript.md")
		require.NoError(t, os.WriteFile(target, []byte("original content"), 0o600))

		d := setupDeps(t)
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader("n\n"))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", url, target})

		err := root.ExecuteContext(context.Background())
		require.ErrorIs(t, err, capture.ErrOverwriteDeclined)
		assert.Contains(t, errBuf.String(), "Overwrite? (Y)es | (N)o: ")

		data, readErr := os.ReadFile(target)
		require.NoError(t, readErr)
		assert.Equal(t, "original content", string(data))
	})

	t.Run("direct join EOF declines without blocking", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "transcript.md")
		require.NoError(t, os.WriteFile(target, []byte("original content"), 0o600))

		d := setupDeps(t)
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader(""))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", url, target})

		err := root.ExecuteContext(context.Background())
		require.ErrorIs(t, err, capture.ErrOverwriteDeclined)
		data, _ := os.ReadFile(target)
		assert.Equal(t, "original content", string(data))
	})

	t.Run("direct join y approves overwrite", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "transcript.md")
		require.NoError(t, os.WriteFile(target, []byte("original content"), 0o600))

		d := setupDeps(t)
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader("y\n"))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", url, target})

		require.NoError(t, root.ExecuteContext(context.Background()))
		assert.Contains(t, errBuf.String(), "Overwrite? (Y)es | (N)o: ")
		data, _ := os.ReadFile(target)
		assert.Contains(t, string(data), "Fresh transcript line.")
	})

	t.Run("direct join --overwrite bypasses prompt", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "transcript.md")
		require.NoError(t, os.WriteFile(target, []byte("original content"), 0o600))

		d := setupDeps(t)
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader("n\n"))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", url, target, "--overwrite"})

		require.NoError(t, root.ExecuteContext(context.Background()))
		assert.NotContains(t, errBuf.String(), "Overwrite?")
		data, _ := os.ReadFile(target)
		assert.Contains(t, string(data), "Fresh transcript line.")
	})

	t.Run("--queue preserves outer prompt streams across executeQueueLine", func(t *testing.T) {
		dir := t.TempDir()
		target1 := filepath.Join(dir, "queued1.md")
		target2 := filepath.Join(dir, "queued2.md")
		require.NoError(t, os.WriteFile(target1, []byte("queued original 1"), 0o600))
		require.NoError(t, os.WriteFile(target2, []byte("queued original 2"), 0o600))

		queuePath := filepath.Join(dir, "queue.txt")
		queueBody := url + " -o " + quoteQueueArg(target1) + "\n" +
			url + " -o " + quoteQueueArg(target2) + "\n"
		require.NoError(t, os.WriteFile(queuePath, []byte(queueBody), 0o600))

		d := setupDeps(t)
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader("y\ny\n"))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", "--queue", queuePath})

		require.NoError(t, root.ExecuteContext(context.Background()))
		assert.Equal(t, 2, strings.Count(errBuf.String(), "Overwrite? (Y)es | (N)o: "))
		data1, _ := os.ReadFile(target1)
		data2, _ := os.ReadFile(target2)
		assert.Contains(t, string(data1), "Fresh transcript line.")
		assert.Contains(t, string(data2), "Fresh transcript line.")
	})
}
