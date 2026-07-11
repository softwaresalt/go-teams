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

// captureRecorder records what the capture seams were invoked with so join and
// schedule wiring can be asserted without a real browser.
type captureRecorder struct {
	url     string
	called  bool
	runErr  error
	meetErr error
}

type recorderSession struct{ rec *captureRecorder }

func (s recorderSession) Run(context.Context, browser.Meeting) error {
	s.rec.called = true
	return s.rec.runErr
}

// depsWith returns a deps set whose loader always succeeds, whose runner is the
// supplied fake, and whose capture seams route through a throwaway recorder.
func depsWith(runner browser.Runner) *deps {
	return depsWithCapture(runner, &captureRecorder{})
}

// depsWithCapture is depsWith with an inspectable capture recorder.
func depsWithCapture(runner browser.Runner, rec *captureRecorder) *deps {
	return &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
		newRunner:  func(config.Config) browser.Runner { return runner },
		newMeeting: func(ctx context.Context, _ config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			rec.url = url
			return nil, ctx, func() {}, rec.meetErr
		},
		newSession: func(config.Config, string) meetingSession { return recorderSession{rec} },
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
	rec := &captureRecorder{}
	err := runCmd(context.Background(), depsWithCapture(&fakeRunner{}, rec),
		"join", "https://teams.microsoft.com/l/meetup-join/x")
	require.NoError(t, err)
	require.True(t, rec.called)
	assert.Contains(t, rec.url, "msLaunch=false")
	assert.Contains(t, rec.url, "suppressPrompt=true")
}

func TestJoinCommandRejectsInvalidURL(t *testing.T) {
	rec := &captureRecorder{}
	err := runCmd(context.Background(), depsWithCapture(&fakeRunner{}, rec), "join", "not-a-url")
	require.Error(t, err)
	assert.False(t, rec.called)
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
