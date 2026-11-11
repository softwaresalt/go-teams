package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
		`teams-cli.exe --config "` + cfgB + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=second" -c ACME`,
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
		`::teams-cli.exe --config "` + cfgB + `" -u "https://teams.cloud.microsoft/l/meetingrecap?threadId=second" -c ACME`,
	}, "\r\n") + "\r\n"
	assert.Equal(t, expectedQueue, updated)

	// Re-running the same queue file must be a no-op since all active lines now start with "::".
	capturedURLs = nil
	require.NoError(t, runCmd(context.Background(), d, "--queue", queuePath))
	assert.Empty(t, capturedURLs)
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
