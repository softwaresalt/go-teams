//go:build integration

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
)

const (
	liveCLINominalDuration = 15 * time.Second
	liveCLIIdleTimeout     = 15 * time.Second
)

// requireCLIIntegration validates GO_TEAMS_INTEGRATION and confirms
// TEAMS_PROFILE_DIR is workspace-contained and git-ignored without echoing
// sensitive values.
func requireCLIIntegration(t *testing.T) string {
	t.Helper()
	loadCLITestEnv(t)

	v := strings.ToLower(strings.TrimSpace(os.Getenv("GO_TEAMS_INTEGRATION")))
	if v == "" || v == "0" || v == "false" {
		t.Skip("set GO_TEAMS_INTEGRATION=1 to run CLI live-tenant integration tests")
	}

	profile := strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))
	if profile == "" {
		t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for CLI live-tenant integration tests")
	}

	root, err := os.Getwd()
	require.NoError(t, err, "resolve working directory")
	for {
		if _, statErr := os.Stat(filepath.Join(root, "go.mod")); statErr == nil {
			break
		}
		parent := filepath.Dir(root)
		require.NotEqual(t, root, parent, "repository root with go.mod not found")
		root = parent
	}
	realRoot, err := filepath.EvalSymlinks(root)
	require.NoError(t, err, "resolve repository root")

	absProfile := profile
	if !filepath.IsAbs(absProfile) {
		absProfile = filepath.Join(realRoot, absProfile)
	}
	absProfile = filepath.Clean(absProfile)
	cur := absProfile
	var trailing []string
	for {
		if _, lerr := os.Lstat(cur); lerr == nil {
			break
		}
		parent := filepath.Dir(cur)
		require.NotEqual(t, cur, parent, "TEAMS_PROFILE_DIR ancestor not found")
		trailing = append([]string{filepath.Base(cur)}, trailing...)
		cur = parent
	}
	realCur, err := filepath.EvalSymlinks(cur)
	require.NoError(t, err, "evaluate TEAMS_PROFILE_DIR symlinks")
	realProfile := filepath.Join(append([]string{realCur}, trailing...)...)
	relProfile, err := filepath.Rel(realRoot, realProfile)
	require.True(t, err == nil && filepath.IsLocal(relProfile) && relProfile != ".", "TEAMS_PROFILE_DIR must resolve within repository root")

	ignoreCmd := exec.Command("git", "check-ignore", "-q", "--", filepath.ToSlash(relProfile))
	ignoreCmd.Dir = realRoot
	require.NoError(t, ignoreCmd.Run(), "TEAMS_PROFILE_DIR must be ignored by git")

	return realProfile
}

// TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown exercises `go-teams
// join --mode live-with-summary` against an authorized live Teams meeting,
// verifying nominal duration, overtime continuation, call-end silence shutdown,
// transcript persistence, and deferred queue entry creation.
func TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown(t *testing.T) {
	profile := requireCLIIntegration(t)

	rawURL := strings.TrimSpace(os.Getenv("TEAMS_MEETING_URL"))
	if rawURL == "" {
		t.Skip("set TEAMS_MEETING_URL to run live scheduled join integration test")
	}
	require.NoError(t, teams.ValidateMeetingURL(rawURL), "invalid meeting URL in test environment")

	outDir := t.TempDir()
	queuePath := filepath.Join(t.TempDir(), "transcription_queue.txt")
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfgYAML := fmt.Sprintf(
		"output_dir: %q\nprofile_dir: %q\nbot_name: \"Transcription Agent\"\nqueue_file: %q\n",
		filepath.ToSlash(outDir),
		filepath.ToSlash(profile),
		filepath.ToSlash(queuePath),
	)
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o600))

	args := []string{
		"join",
		rawURL,
		"--config", cfgPath,
		"--mode", config.CaptureModeLiveWithSummary,
		"--duration", liveCLINominalDuration.String(),
		"--idle-timeout", liveCLIIdleTimeout.String(),
		"--ai-summary-delay", "30m",
		"--lobby-timeout", "2m",
		"--category", "LIVE",
		"--output-dir", outDir,
		"--profile-dir", profile,
		"--headless",
	}
	if bPath := strings.TrimSpace(os.Getenv("TEAMS_BROWSER_PATH")); bPath != "" {
		args = append(args, "--browser-path", bPath)
	}
	if acct := strings.TrimSpace(os.Getenv("TEAMS_ACCOUNT")); acct != "" {
		args = append(args, "--account", acct)
	}

	outerCtx, cancelOuter := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelOuter()
	runCtx, cancelRun := context.WithCancel(outerCtx)
	defer cancelRun()

	errCh := make(chan error, 1)
	go func() {
		root := NewRootCommand()
		root.SetArgs(args)
		errCh <- root.ExecuteContext(runCtx)
	}()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var qStr string
	for {
		select {
		case err := <-errCh:
			require.NoError(t, err, "live join command exited before persisting deferred queue entry")
			t.Fatal("live join command exited before persisting deferred queue entry")
		case <-outerCtx.Done():
			cancelRun()
			<-errCh
			t.Fatal("timed out waiting for live join session self-termination and deferred queue persistence")
		case <-ticker.C:
			qBytes, qErr := os.ReadFile(queuePath)
			if qErr != nil || len(bytes.TrimSpace(qBytes)) == 0 {
				continue
			}
			candidate := string(qBytes)
			if !strings.Contains(candidate, "--append-ai-summary") || !strings.Contains(candidate, "--not-before") {
				continue
			}
			if _, lockErr := os.Stat(queuePath + ".lock"); !errors.Is(lockErr, os.ErrNotExist) {
				continue
			}
			qStr = candidate
		}
		if qStr != "" {
			break
		}
	}

	require.NoError(t, outerCtx.Err(), "expected live join session to self-terminate before outer timeout")
	cancelRun()
	require.NoError(t, <-errCh)

	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	var mdPath string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			mdPath = filepath.Join(outDir, e.Name())
			break
		}
	}
	require.NotEmpty(t, mdPath, "expected flushed Markdown transcript in output directory")

	mdInfo, err := os.Stat(mdPath)
	require.NoError(t, err)
	require.NotZero(t, mdInfo.Size(), "expected non-empty Markdown transcript file")
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0o600), mdInfo.Mode().Perm(), "expected owner-only 0600 transcript file permissions")
	}
	mdBytes, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	require.True(t, strings.Contains(string(mdBytes), "## Transcript"), "expected transcript body section in saved Markdown")

	require.True(t, strings.Contains(qStr, "--append-ai-summary"), "expected deferred --append-ai-summary entry in queue file")
	require.True(t, strings.Contains(qStr, filepath.Base(mdPath)), "expected deferred queue entry to reference saved transcript")
}

// TestLiveTenantDelayedAISummaryQueueReplay verifies that `go-teams --queue`
// replays a due `--append-ai-summary` queue entry against an authorized live
// Teams Meeting Recap, appends `# AI Summary`, and comments the queue line.
func TestLiveTenantDelayedAISummaryQueueReplay(t *testing.T) {
	profile := requireCLIIntegration(t)

	rawRecapURL := strings.TrimSpace(os.Getenv("TEAMS_RECAP_URL"))
	if rawRecapURL == "" {
		mURL := strings.TrimSpace(os.Getenv("TEAMS_MEETING_URL"))
		lower := strings.ToLower(mURL)
		if strings.Contains(lower, "meetingrecap") || strings.Contains(lower, "19:meeting_") || strings.Contains(lower, "19%3ameeting_") {
			rawRecapURL = mURL
		} else {
			t.Skip("set TEAMS_RECAP_URL (or a TEAMS_MEETING_URL carrying threadId) to run live delayed AI summary queue replay test")
		}
	}
	recapURL, err := teams.MutateJoinURL(rawRecapURL)
	require.NoError(t, err, "invalid recap URL in test environment")

	workDir := t.TempDir()
	outDir := filepath.Join(workDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o700))
	cfgPath := filepath.Join(workDir, "config.yaml")
	queuePath := filepath.Join(workDir, "transcription_queue.txt")
	cfgYAML := fmt.Sprintf(
		"output_dir: %q\nprofile_dir: %q\nbot_name: \"Transcription Agent\"\nqueue_file: %q\n",
		filepath.ToSlash(outDir),
		filepath.ToSlash(profile),
		filepath.ToSlash(queuePath),
	)
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o600))

	mdPath := filepath.Join(outDir, "live-replay.md")
	require.NoError(t, os.WriteFile(mdPath, []byte("# Meeting Transcript\n\n> **Alice:** Live capture.\n"), 0o600))

	err = appendDeferredAISummaryQueueLineWithConfig(
		queuePath,
		recapURL,
		mdPath,
		time.Now().Add(-1*time.Minute),
		config.Config{
			ConfigPath:     cfgPath,
			ProfileDir:     profile,
			BrowserPath:    strings.TrimSpace(os.Getenv("TEAMS_BROWSER_PATH")),
			DefaultAccount: strings.TrimSpace(os.Getenv("TEAMS_ACCOUNT")),
			LobbyTimeout:   "2m",
			Headless:       true,
		},
	)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	root := NewRootCommand()
	root.SetArgs([]string{
		"--config", cfgPath,
		"--queue", queuePath,
	})
	require.NoError(t, root.ExecuteContext(ctx))
	require.NoError(t, ctx.Err())

	mdBytes, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	mdContent := string(mdBytes)
	require.True(t, strings.Contains(mdContent, "> **Alice:** Live capture."), "expected existing live transcript content to be preserved")
	require.True(t, strings.Contains(mdContent, "# AI Summary"), "expected appended # AI Summary section")

	qBytes, err := os.ReadFile(queuePath)
	require.NoError(t, err)
	qContent := string(qBytes)
	var markedDone bool
	for _, line := range strings.Split(strings.ReplaceAll(qContent, "\r\n", "\n"), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "::") && strings.Contains(trimmed, "--append-ai-summary") {
			markedDone = true
			break
		}
	}
	require.True(t, markedDone, "expected completed queue line to be prefixed with ::")
}
