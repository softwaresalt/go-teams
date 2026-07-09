package cli

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
)

type fakeRunner struct {
	authCalled bool
	joinURL    string
	joinCalled bool
	authErr    error
	joinErr    error
}

func (f *fakeRunner) Authenticate(context.Context) error {
	f.authCalled = true
	return f.authErr
}

func (f *fakeRunner) Join(_ context.Context, meetingURL string) error {
	f.joinCalled = true
	f.joinURL = meetingURL
	return f.joinErr
}

func validConfig() config.Config {
	return config.Config{OutputDir: "out", ProfileDir: "prof", BotName: "bot"}
}

// depsWith returns a deps set whose loader always succeeds and whose runner is
// the supplied fake.
func depsWith(runner browser.Runner) *deps {
	return &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
		newRunner:  func(config.Config) browser.Runner { return runner },
	}
}

func runCmd(ctx context.Context, d *deps, args ...string) error {
	root := newRootCommand(d)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	if ctx == nil {
		ctx = context.Background()
	}
	return root.ExecuteContext(ctx)
}

func TestAuthCommandInvokesAuthenticate(t *testing.T) {
	fake := &fakeRunner{}
	err := runCmd(context.Background(), depsWith(fake), "auth")
	require.NoError(t, err)
	assert.True(t, fake.authCalled)
}

func TestJoinCommandMutatesURL(t *testing.T) {
	fake := &fakeRunner{}
	err := runCmd(context.Background(), depsWith(fake),
		"join", "https://teams.microsoft.com/l/meetup-join/x")
	require.NoError(t, err)
	require.True(t, fake.joinCalled)
	assert.Contains(t, fake.joinURL, "msLaunch=false")
	assert.Contains(t, fake.joinURL, "suppressPrompt=true")
}

func TestJoinCommandRejectsInvalidURL(t *testing.T) {
	fake := &fakeRunner{}
	err := runCmd(context.Background(), depsWith(fake), "join", "not-a-url")
	require.Error(t, err)
	assert.False(t, fake.joinCalled)
}

func TestJoinCommandRequiresArg(t *testing.T) {
	err := runCmd(context.Background(), depsWith(&fakeRunner{}), "join")
	assert.Error(t, err)
}

func TestCommandPropagatesConfigError(t *testing.T) {
	wantErr := errors.New("boom")
	d := &deps{
		loadConfig: func(string) (config.Config, error) { return config.Config{}, wantErr },
		newRunner:  func(config.Config) browser.Runner { return &fakeRunner{} },
	}
	err := runCmd(context.Background(), d, "auth")
	assert.ErrorIs(t, err, wantErr)
}

func TestScheduleCommandBuildsAndStops(t *testing.T) {
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.Meetings = []config.MeetingSchedule{
				{Name: "standup", URL: "https://teams.microsoft.com/x", Cron: "* * * * *"},
			}
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
	}
	// Cancelled context so the daemon returns immediately after building.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := runCmd(ctx, d, "schedule")
	assert.NoError(t, err)
}

func TestScheduleCommandInvalidCron(t *testing.T) {
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			c := validConfig()
			c.Meetings = []config.MeetingSchedule{
				{Name: "bad", URL: "https://x", Cron: "nonsense"},
			}
			return c, nil
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
	}
	err := runCmd(context.Background(), d, "schedule")
	assert.Error(t, err)
}

func TestRootHasSubcommands(t *testing.T) {
	root := newRootCommand(defaultDeps())
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"auth", "join", "schedule"} {
		assert.Truef(t, names[want], "missing subcommand %q", want)
	}
}
