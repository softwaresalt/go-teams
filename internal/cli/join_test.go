package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	t.Run("oversized line drains through newline without leaking trailing bytes", func(t *testing.T) {
		longLine := strings.Repeat("x", 5000) + "\ny\n"
		reader := strings.NewReader(longLine)
		var out1, out2 bytes.Buffer

		got1, err1 := promptOverwriteApproval(context.Background(), reader, &out1, []string{"a.md"})
		require.NoError(t, err1)
		assert.False(t, got1)

		got2, err2 := promptOverwriteApproval(context.Background(), reader, &out2, []string{"b.md"})
		require.NoError(t, err2)
		assert.True(t, got2)
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

func TestCaptureFlagsAndQueueForwardNoAISummary(t *testing.T) {
	t.Run("apply-flag-true", func(t *testing.T) {
		var f captureFlags
		cmd := &cobra.Command{Use: "join"}
		bindCaptureFlags(cmd, &f)
		require.NoError(t, cmd.Flags().Parse([]string{"--no-ai-summary"}))

		cfg := config.Config{AISummaryDelay: "30m"}
		applyCaptureFlags(cmd, &cfg, f)
		assert.True(t, cfg.NoAISummary)
		assert.Equal(t, "0s", cfg.AISummaryDelay)
	})

	t.Run("apply-flag-false", func(t *testing.T) {
		var f captureFlags
		cmd := &cobra.Command{Use: "join"}
		bindCaptureFlags(cmd, &f)
		require.NoError(t, cmd.Flags().Parse([]string{"--no-ai-summary=false"}))

		cfg := config.Config{AISummaryDelay: "30m", NoAISummary: true}
		applyCaptureFlags(cmd, &cfg, f)
		assert.False(t, cfg.NoAISummary)
		assert.Equal(t, "30m", cfg.AISummaryDelay)
	})

	t.Run("apply-delay-zero", func(t *testing.T) {
		var f captureFlags
		cmd := &cobra.Command{Use: "join"}
		bindCaptureFlags(cmd, &f)
		require.NoError(t, cmd.Flags().Parse([]string{"--ai-summary-delay=0s"}))

		cfg := config.Config{AISummaryDelay: "30m"}
		applyCaptureFlags(cmd, &cfg, f)
		assert.False(t, cfg.NoAISummary)
		assert.Equal(t, "0s", cfg.AISummaryDelay)
	})

	t.Run("config-delay-zero-is-not-explicit-disable-characterization", func(t *testing.T) {
		var f captureFlags
		cmd := &cobra.Command{Use: "join"}
		bindCaptureFlags(cmd, &f)
		cfg := config.Config{AISummaryDelay: "0s"}

		applyCaptureFlags(cmd, &cfg, f)
		assert.False(t, cfg.NoAISummary)
		assert.Equal(t, "0s", cfg.AISummaryDelay)
	})

	t.Run("nil-command-preserves-config-and-does-not-forward-flags", func(t *testing.T) {
		cfg := config.Config{AISummaryDelay: "30m", NoAISummary: true}

		assert.NotPanics(t, func() {
			applyCaptureFlags(nil, &cfg, captureFlags{})
		})
		assert.True(t, cfg.NoAISummary)
		assert.Equal(t, "30m", cfg.AISummaryDelay)
		assert.Empty(t, inheritedQueueFlags(nil, "", captureFlags{}))
	})

	for _, tc := range []struct {
		name         string
		flag         string
		perLineFlag  string
		want         string
		wantResolved bool
	}{
		{
			name:         "queue-flag-true",
			flag:         "--no-ai-summary",
			perLineFlag:  "--no-ai-summary=false",
			want:         "--no-ai-summary",
			wantResolved: false,
		},
		{
			name:         "queue-flag-false",
			flag:         "--no-ai-summary=false",
			perLineFlag:  "--no-ai-summary",
			want:         "--no-ai-summary=false",
			wantResolved: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f captureFlags
			cmd := &cobra.Command{Use: "join"}
			bindCaptureFlags(cmd, &f)
			require.NoError(t, cmd.Flags().Parse([]string{tc.flag}))

			inherited := inheritedQueueFlags(cmd, "", f)
			assert.Equal(t, []string{tc.want}, inherited)

			var lineFlags captureFlags
			lineCmd := &cobra.Command{Use: "join"}
			bindCaptureFlags(lineCmd, &lineFlags)
			require.NoError(t, lineCmd.Flags().Parse(append(inherited, tc.perLineFlag)))
			cfg := config.Config{}
			applyCaptureFlags(lineCmd, &cfg, lineFlags)
			assert.Equal(t, tc.wantResolved, cfg.NoAISummary)
		})
	}

	t.Run("shared-help", func(t *testing.T) {
		help := captureHelpBody()
		assert.Contains(t, help,
			"--no-ai-summary    Disable AI Summary capture for both Meeting Recaps and delayed post-call enrichment.",
		)
	})

	t.Run("flag-help", func(t *testing.T) {
		var f captureFlags
		cmd := &cobra.Command{Use: "join"}
		bindCaptureFlags(cmd, &f)
		usage := cmd.Flags().Lookup("no-ai-summary").Usage
		assert.Equal(t,
			"disable AI Summary capture for both Meeting Recaps and delayed post-call enrichment",
			usage,
		)
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

	t.Run("--queue does not mark line complete when browserCtx cancels during ConfirmOverwrite", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "cancelled.md")
		require.NoError(t, os.WriteFile(target, []byte("keep original"), 0o600))

		queuePath := filepath.Join(dir, "queue.txt")
		queueLine := url + " -o " + quoteQueueArg(target)
		require.NoError(t, os.WriteFile(queuePath, []byte(queueLine+"\n"), 0o600))

		d := setupDeps(t)
		d.newMeeting = func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			bCtx, bCancel := context.WithCancel(ctx)
			bCancel()
			return fakeCLIOverwriteMeeting{}, bCtx, func() {}, nil
		}
		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(strings.NewReader("y\n"))
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", "--queue", queuePath})

		err := root.ExecuteContext(context.Background())
		require.ErrorIs(t, err, context.Canceled)

		data, _ := os.ReadFile(target)
		assert.Equal(t, "keep original", string(data))
		qData, _ := os.ReadFile(queuePath)
		assert.NotContains(t, string(qData), "::"+queueLine, "cancelled queue line must remain uncommented")
	})

	t.Run("stuck cancelled prompt keeps promptMu held against concurrent queue prompt", func(t *testing.T) {
		dir := t.TempDir()
		target1 := filepath.Join(dir, "one.md")
		target2 := filepath.Join(dir, "two.md")
		require.NoError(t, os.WriteFile(target1, []byte("keep 1"), 0o600))
		require.NoError(t, os.WriteFile(target2, []byte("keep 2"), 0o600))

		pr, pw := io.Pipe()
		t.Cleanup(func() {
			_ = pw.Close()
			_ = pr.Close()
		})

		d := setupDeps(t)
		var callCount int
		d.newMeeting = func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			callCount++
			if callCount == 1 {
				bCtx, bCancel := context.WithTimeout(ctx, 25*time.Millisecond)
				return fakeCLIOverwriteMeeting{}, bCtx, bCancel, nil
			}
			return fakeCLIOverwriteMeeting{}, ctx, func() {}, nil
		}

		queuePath := filepath.Join(dir, "queue.txt")
		queueBody := url + " -o " + quoteQueueArg(target1) + "\n" +
			url + " -o " + quoteQueueArg(target2) + "\n"
		require.NoError(t, os.WriteFile(queuePath, []byte(queueBody), 0o600))

		root := newRootCommand(d)
		var errBuf bytes.Buffer
		root.SetIn(pr)
		root.SetOut(io.Discard)
		root.SetErr(&errBuf)
		root.SetArgs([]string{"join", "--queue", queuePath})

		err := root.ExecuteContext(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "prompt stream busy after cancelled prompt")

		data1, _ := os.ReadFile(target1)
		data2, _ := os.ReadFile(target2)
		assert.Equal(t, "keep 1", string(data1))
		assert.Equal(t, "keep 2", string(data2))
	})
}
