package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

func TestQueueFileProcessesActiveLinesAndCommentsOutCompletedEntries(t *testing.T) {
	dir := t.TempDir()
	cfgA := filepath.Join(dir, "wsp.yaml")
	cfgB := filepath.Join(dir, "acme.yaml")
	queuePath := filepath.Join(dir, "transcript-queue.txt")

	initialQueue := strings.Join([]string{
		":: --config " + cfgA + ` -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=already-done"`,
		"",
		`--config "` + cfgA + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=first"`,
		`go-teams.exe --config "` + cfgB + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=second" -c ACME`,
	}, "\r\n") + "\r\n"
	require.NoError(t, os.WriteFile(queuePath, []byte(initialQueue), 0o600))

	var loadedConfigs []string
	var capturedURLs []string
	var capturedCategories []string

	d := &deps{
		loadConfig: func(p string) (config.Config, error) {
			loadedConfigs = append(loadedConfigs, p)
			c := validConfig()
			c.Category = "WSP"
			c.Headless = true
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, c config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			capturedURLs = append(capturedURLs, url)
			capturedCategories = append(capturedCategories, c.Category)
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession {
			return recorderSession{&captureRecorder{}}
		},
	}

	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath))

	assert.Equal(t, []string{cfgA, cfgB}, loadedConfigs)
	assert.Equal(t, []string{
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=first",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=second",
	}, capturedURLs)
	assert.Equal(t, []string{"WSP", "ACME"}, capturedCategories)

	updatedBytes, err := os.ReadFile(queuePath)
	require.NoError(t, err)
	updated := string(updatedBytes)
	assert.Contains(t, updated, "\r\n", "queue file should preserve CRLF line endings")

	expectedQueue := strings.Join([]string{
		":: --config " + cfgA + ` -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=already-done"`,
		"",
		`::--config "` + cfgA + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=first"`,
		`::go-teams.exe --config "` + cfgB + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=second" -c ACME`,
	}, "\r\n") + "\r\n"
	assert.Equal(t, expectedQueue, updated)

	// Re-running the same queue file must be a no-op since all active lines now start with "::".
	capturedURLs = nil
	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath))
	assert.Empty(t, capturedURLs)
}

func TestQueueFileStripsCrossPlatformGoTeamsAndLegacyTeamsCLITokensAndSkipsBareTokens(t *testing.T) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "transcript-queue.txt")

	initialQueue := strings.Join([]string{
		`go-teams`,
		`teams-cli.exe`,
		`go-teams -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=one"`,
		`GO-TEAMS.EXE -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=two"`,
		`"C:\Tools\go-teams.exe" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=three"`,
		`"/usr/local/bin/go-teams" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=four"`,
		`teams-cli -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=five"`,
		`".\bin\teams-cli.exe" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=six"`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(queuePath, []byte(initialQueue), 0o600))

	var capturedURLs []string
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.Headless = true
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, _ config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			capturedURLs = append(capturedURLs, url)
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession {
			return recorderSession{&captureRecorder{}}
		},
	}

	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath))
	assert.Equal(t, []string{
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=one",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=two",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=three",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=four",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=five",
		"https://teams.cloud.microsoft/l/meetingrecap?threadId=six",
	}, capturedURLs)
}

func TestQueueFileLeavesFailedLineUncommentedAndContinuesBatch(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "wsp.yaml")
	queuePath := filepath.Join(dir, "transcript-queue.txt")

	initialQueue := strings.Join([]string{
		`--config "` + cfgPath + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=fail"`,
		`--config "` + cfgPath + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=succeed"`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(queuePath, []byte(initialQueue), 0o600))

	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.Headless = true
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, _ config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			if strings.Contains(url, "threadId=fail") {
				return nil, ctx, func() {}, errors.New("simulated failure")
			}
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession {
			return recorderSession{&captureRecorder{}}
		},
	}

	err := runCmd(context.Background(), d, "--queue", queuePath)
	require.Error(t, err)

	updatedBytes, readErr := os.ReadFile(queuePath)
	require.NoError(t, readErr)
	lines := strings.Split(strings.TrimSpace(string(updatedBytes)), "\n")
	require.Len(t, lines, 2)
	assert.False(t, strings.HasPrefix(lines[0], "::"), "failed line must remain uncommented for retry")
	assert.True(t, strings.HasPrefix(lines[1], "::"), "succeeded line must be prefixed with ::")
}

func TestQueueFileDoesNotCommentOutCancelledEntryAndRejectsNestedQueueAndForwardsAudioFalse(t *testing.T) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "transcript-queue.txt")
	initialQueue := strings.Join([]string{
		`-q` + queuePath,
		`-u "https://teams.cloud.microsoft/l/meetingrecap?threadId=cancel"`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(queuePath, []byte(initialQueue), 0o600))

	ctx, cancel := context.WithCancel(context.Background())
	var gotAudio bool
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.CaptureAudio = true
			c.Headless = true
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(mctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			gotAudio = c.CaptureAudio
			// Simulate cancellation during Join where Session.Run returns nil.
			cancel()
			return nil, mctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession {
			return recorderSession{&captureRecorder{}}
		},
	}

	err := runCmd(ctx, d, "--queue", queuePath, "--audio=false")
	require.Error(t, err)
	assert.False(t, gotAudio, "explicit --audio=false must be forwarded to queue entries")

	updatedBytes, readErr := os.ReadFile(queuePath)
	require.NoError(t, readErr)
	for _, line := range strings.Split(strings.TrimSpace(string(updatedBytes)), "\n") {
		assert.False(t, strings.HasPrefix(line, "::"), "neither rejected nested -q nor cancelled entry may be commented out: %q", line)
	}
}

func TestQueueNoAISummarySkipsInheritedAppendWithoutCommentingLine(t *testing.T) {
	queuePath := filepath.Join(t.TempDir(), "transcript-queue.txt")
	line := `-u "https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic" --append-ai-summary "transcript.md"`
	require.NoError(t, os.WriteFile(queuePath, []byte(line+"\n"), 0o600))

	d := &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
	}
	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath, "--no-ai-summary"))

	updated, err := os.ReadFile(queuePath)
	require.NoError(t, err)
	assert.Equal(t, line+"\n", string(updated))
}

func TestSplitQueueArgsUnterminatedQuoteDoesNotLeakLine(t *testing.T) {
	line := `join --url "https://teams.microsoft.com/meet/123?p=secret-passcode`

	_, err := splitQueueArgs(line)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-passcode")
	assert.NotContains(t, err.Error(), "teams.microsoft.com")
}

func TestQueueFileUnterminatedQuoteDoesNotLeakPasscode(t *testing.T) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "transcript-queue.txt")
	line := `join --url "https://teams.microsoft.com/meet/123?p=secret-passcode`
	require.NoError(t, os.WriteFile(queuePath, []byte(line+"\n"), 0o600))

	d := &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
		newRunner:  func(config.Config) browser.Runner { return &fakeRunner{} },
	}

	err := runCmd(context.Background(), d, "--queue", queuePath)

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-passcode")
	assert.NotContains(t, err.Error(), "https://teams.microsoft.com/meet/123")
}

func TestSanitizeQueueArgsForLogRedactsURLSecrets(t *testing.T) {
	got := sanitizeQueueArgsForLog([]string{"-u", "https://user:pass@teams.microsoft.com/meet/1?p=secret-passcode#frag", "-c", "ACME"})

	assert.Equal(t, "-u https://teams.microsoft.com/meet/1 -c ACME", got)
	for _, s := range []string{"secret-passcode", "pass", "user", "frag"} {
		assert.NotContains(t, got, s)
	}
}

func TestSanitizeQueueArgsForLogRedactsAccountFlag(t *testing.T) {
	gotSeparate := sanitizeQueueArgsForLog([]string{"-u", "https://teams.microsoft.com/meet/1", "--account", "alice@contoso.com"})
	assert.Equal(t, "-u https://teams.microsoft.com/meet/1 --account a***@contoso.com", gotSeparate)
	assert.NotContains(t, gotSeparate, "alice@contoso.com")

	gotEquals := sanitizeQueueArgsForLog([]string{"-u", "https://teams.microsoft.com/meet/1", "--account=Bob Smith"})
	assert.Equal(t, "-u https://teams.microsoft.com/meet/1 --account=B***", gotEquals)
	assert.NotContains(t, gotEquals, "Bob Smith")
}

func TestQueueLineAISummaryOverrideDetection(t *testing.T) {
	assert.True(t, queueLineEnablesAISummary([]string{"--append-ai-summary", "call.md", "--no-ai-summary=false"}))
	assert.False(t, queueLineEnablesAISummary([]string{"--append-ai-summary", "call.md"}))
	assert.False(t, queueLineEnablesAISummary([]string{"--append-ai-summary", "call.md", "--no-ai-summary=true"}))
	assert.True(t, queueLineEnablesAISummary([]string{"--no-ai-summary=true", "--no-ai-summary=false"}))
	assert.False(t, queueLineEnablesAISummary([]string{"--no-ai-summary=false", "--no-ai-summary=true"}))
}

func TestQueueFileHonorsPerEntryAudioAliasOverrides(t *testing.T) {
	dir := t.TempDir()
	queuePath := filepath.Join(dir, "transcript-queue.txt")
	const base = "https://teams.cloud.microsoft/l/meetingrecap?threadId="
	queue := strings.Join([]string{
		`-u "` + base + `inherit"`,
		`-u "` + base + `audio-off" --audio=false`,
		`-u "` + base + `recap-off" --recap-audio=false`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(queuePath, []byte(queue), 0o600))

	got := map[string]bool{}
	d := &deps{
		loadConfig: func(string) (config.Config, error) { c := validConfig(); c.Headless = true; return c, nil },
		newRunner:  func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, c config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			got[strings.TrimPrefix(url, base)] = c.CaptureAudio
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return recorderSession{&captureRecorder{}} },
	}
	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath, "--recap-audio"))
	assert.Equal(t, map[string]bool{"inherit": true, "audio-off": false, "recap-off": false}, got)
}

func TestAppendDeferredAISummaryQueueHelpers(t *testing.T) {
	dir := t.TempDir()
	qPath := filepath.Join(dir, "transcription_queue.txt")
	mdPath := filepath.Join(dir, "call.md")
	cfg := config.Config{
		ProfileDir:     `C:\Profiles\Alice\`,
		DefaultAccount: "alice@contoso.com",
		Headless:       true,
	}
	require.NoError(t, appendDeferredAISummaryQueueLineWithConfig(qPath, "https://teams.microsoft.com/meet/123", mdPath, time.Now().UTC(), cfg))

	rawBytes, err := os.ReadFile(qPath)
	require.NoError(t, err)
	parsedArgs, err := splitQueueArgs(strings.TrimSpace(string(rawBytes)))
	require.NoError(t, err, "trailing backslash in Windows profile path must parse cleanly via splitQueueArgs")
	assert.Equal(t, `C:\Profiles\Alice\`, extractQueueFlagValue(parsedArgs, "--profile-dir"))
	assert.Equal(t, mdPath, extractQueueFlagValue(parsedArgs, "--append-ai-summary"))

	require.NoError(t, markDeferredAISummaryQueueLineDone(qPath, mdPath))
	b, err := os.ReadFile(qPath)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(string(b)), "::"))
}

func TestQueueSkipsFutureNotBeforeAndExecutesDueAppendAISummary(t *testing.T) {
	dir := t.TempDir()
	qPath := filepath.Join(dir, "transcription_queue.txt")
	futureMD := filepath.Join(dir, "future.md")
	dueMD := filepath.Join(dir, "due.md")
	require.NoError(t, os.WriteFile(futureMD, []byte("# Meeting Transcript\n\n> **Alice:** Future.\n"), 0o600))
	require.NoError(t, os.WriteFile(dueMD, []byte("# Meeting Transcript\n\n> **Bob:** Due now.\n"), 0o600))

	futureTS := time.Now().Add(1 * time.Hour).UTC().Format(time.RFC3339)
	pastTS := time.Now().Add(-1 * time.Minute).UTC().Format(time.RFC3339)

	require.NoError(t, appendDeferredAISummaryQueueLine(qPath, "https://teams.microsoft.com/meet/future", futureMD, time.Now().Add(1*time.Hour)))
	require.NoError(t, appendDeferredAISummaryQueueLine(qPath, "https://teams.microsoft.com/meet/due", dueMD, time.Now().Add(-1*time.Minute)))
	_ = futureTS
	_ = pastTS

	d := &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
		newRunner:  func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, _ config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			return &fakeSummaryMeeting{summary: "Summary:\nAppended from queue."}, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession {
			t.Fatal("newSession must not be called for --append-ai-summary queue lines")
			return nil
		},
	}

	require.NoError(t, runCmd(context.Background(), d, "--queue", qPath))

	qBytes, err := os.ReadFile(qPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(qBytes)), "\n")
	require.Len(t, lines, 2)
	assert.False(t, strings.HasPrefix(lines[0], "::"), "future --not-before line must remain uncommented")
	assert.True(t, strings.HasPrefix(lines[1], "::"), "due --not-before line must be marked ::")

	dueBytes, err := os.ReadFile(dueMD)
	require.NoError(t, err)
	assert.Contains(t, string(dueBytes), "Due now.")
	assert.Contains(t, string(dueBytes), "Appended from queue.")
}

func TestQueueBatchCatchUpUsesConfiguredOutputDirAndPerLineFlags(t *testing.T) {
	dir := t.TempDir()
	qPath := filepath.Join(dir, "transcription_queue.txt")
	queueContent := strings.Join([]string{
		`"https://teams.cloud.microsoft/l/meetingrecap?threadId=one" --mode past-recap --category ENG --audio`,
		`"https://teams.microsoft.com/meet/two" --mode ephemeral-live --category PROD`,
	}, "\n") + "\n"
	require.NoError(t, os.WriteFile(qPath, []byte(queueContent), 0o600))

	var gotCfgs []config.Config
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.OutputDir = filepath.Join(dir, "configured-out")
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			gotCfgs = append(gotCfgs, c)
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return &fakeSession{} },
	}

	require.NoError(t, runCmd(context.Background(), d, "--queue", qPath))
	require.Len(t, gotCfgs, 2)
	assert.Equal(t, filepath.Join(dir, "configured-out"), gotCfgs[0].OutputDir)
	assert.Equal(t, config.CaptureModePastRecap, gotCfgs[0].CaptureMode)
	assert.Equal(t, "ENG", gotCfgs[0].Category)
	assert.True(t, gotCfgs[0].CaptureAudio)

	assert.Equal(t, filepath.Join(dir, "configured-out"), gotCfgs[1].OutputDir)
	assert.Equal(t, config.CaptureModeEphemeralLive, gotCfgs[1].CaptureMode)
	assert.Equal(t, "PROD", gotCfgs[1].Category)
	assert.True(t, gotCfgs[1].CaptureAudio)
}
