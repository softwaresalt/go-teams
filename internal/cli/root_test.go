package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

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

// browserPathProbe collects the config each seam receives so a test can assert
// what reached the browser layer for every command.
type browserPathProbe struct {
	cfgs chan config.Config
}

func newBrowserPathProbe() *browserPathProbe {
	return &browserPathProbe{cfgs: make(chan config.Config, 8)}
}

func (p *browserPathProbe) deps(loaded config.Config) *deps {
	return &deps{
		loadConfig: func(string) (config.Config, error) { return loaded, nil },
		newRunner: func(c config.Config) browser.Runner {
			p.cfgs <- c
			return &fakeRunner{}
		},
		newMeeting: func(ctx context.Context, c config.Config, _ string) (browser.Meeting, context.Context, func(), error) {
			p.cfgs <- c
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return recorderSession{&captureRecorder{}} },
	}
}

func TestBrowserPathWiredAcrossAllCommands(t *testing.T) {
	const (
		meetingURL = "https://teams.microsoft.com/meet/123"
		cfgBrowser = "/from/config/browser"
		cfgProfile = "/from/config/profile"
		flagBrower = "/from/flag/browser"
		flagProf   = "/from/flag/profile"
	)
	commands := []struct {
		name string
		args []string
	}{
		{"root", []string{meetingURL}},
		{"join", []string{"join", meetingURL}},
		{"captions", []string{"captions", meetingURL}},
		{"auth", []string{"auth"}},
		{"schedule", []string{"schedule"}},
	}
	scenarios := []struct {
		name        string
		extra       []string
		wantBrowser string
		wantProfile string
	}{
		{"config only", nil, cfgBrowser, cfgProfile},
		{"flags override config", []string{"--browser-path", flagBrower, "--profile-dir", flagProf}, flagBrower, flagProf},
	}
	for _, cmdCase := range commands {
		for _, sc := range scenarios {
			t.Run(cmdCase.name+"/"+sc.name, func(t *testing.T) {
				loaded := validConfig()
				loaded.BrowserPath = cfgBrowser
				loaded.ProfileDir = cfgProfile
				loaded.Meetings = []config.MeetingSchedule{{Name: "m", URL: meetingURL, Cron: "@every 1s"}}
				probe := newBrowserPathProbe()
				args := append(append([]string{}, cmdCase.args...), sc.extra...)

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- runCmd(ctx, probe.deps(loaded), args...) }()

				var got config.Config
				finished := false
				select {
				case got = <-probe.cfgs:
				case err := <-done:
					finished = true
					require.NoError(t, err)
					// Completion and the buffered config can both be ready.
					select {
					case got = <-probe.cfgs:
					default:
						t.Fatal("command finished without reaching the browser layer")
					}
				case <-time.After(10 * time.Second):
					t.Fatal("timed out waiting for the browser layer")
				}
				cancel()
				if !finished {
					select {
					case <-done:
					case <-time.After(10 * time.Second):
						t.Fatal("command did not stop after cancel")
					}
				}

				assert.Equal(t, sc.wantBrowser, got.BrowserPath)
				assert.Equal(t, sc.wantProfile, got.ProfileDir)
				opts := browserOptions(got)
				assert.Equal(t, sc.wantBrowser, opts.ExecPath)
				assert.Equal(t, sc.wantProfile, opts.UserDataDir)
			})
		}
	}
}

func TestRootHasSubcommands(t *testing.T) {
	root := newRootCommand(defaultDeps())
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"auth", "join", "captions", "schedule"} {
		assert.Truef(t, names[want], "missing subcommand %q", want)
	}
}

func TestJoinFallsBackToDefaultHeadedConfigWhenConfigFileMissing(t *testing.T) {
	var gotCfg config.Config
	rec := &captureRecorder{}
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{}, os.ErrNotExist
		},
		newRunner: func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, cfg config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			gotCfg = cfg
			rec.url = url
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return recorderSession{rec} },
	}
	err := runCmd(context.Background(), d, "join", "https://teams.microsoft.com/meet/123?p=abc", "--output-dir", "custom-out")
	require.NoError(t, err)
	assert.True(t, rec.called)
	assert.False(t, gotCfg.Headless, "zero-config join should default to a visible (headed) browser window for user auth")
	assert.Equal(t, "custom-out", gotCfg.OutputDir)
}

func TestCaptionsCommandDefaultsToTeamsHomeWhenNoURLGiven(t *testing.T) {
	rec := &captureRecorder{}
	err := runCmd(context.Background(), depsWithCapture(&fakeRunner{}, rec), "captions")
	require.NoError(t, err)
	assert.True(t, rec.called)
	assert.Contains(t, rec.url, "teams.microsoft.com")
}

func TestCLIOutputFileFlagAndPositionalArg(t *testing.T) {
	var gotCfg config.Config
	rec := &captureRecorder{}
	d := &deps{
		loadConfig: func(string) (config.Config, error) { return validConfig(), nil },
		newRunner:  func(config.Config) browser.Runner { return &fakeRunner{} },
		newMeeting: func(ctx context.Context, cfg config.Config, url string) (browser.Meeting, context.Context, func(), error) {
			gotCfg = cfg
			rec.url = url
			return nil, ctx, func() {}, nil
		},
		newSession: func(config.Config, string) meetingSession { return recorderSession{rec} },
	}

	// 1. Positional <url> <output.md> on captions command
	err := runCmd(context.Background(), d, "captions", "https://teams.cloud.microsoft/l/meetingrecap?threadId=1", `C:\out\recap.md`)
	require.NoError(t, err)
	assert.Equal(t, `C:\out\recap.md`, gotCfg.OutputFile)

	// 2. -o / --output with a .md file path on join command
	gotCfg = config.Config{}
	err = runCmd(context.Background(), d, "join", "https://teams.microsoft.com/meet/123", "-o", `C:\out\call.md`)
	require.NoError(t, err)
	assert.Equal(t, `C:\out\call.md`, gotCfg.OutputFile)

	// 3. Direct root command invocation: teams-cli <url> <output.md>
	gotCfg = config.Config{}
	err = runCmd(context.Background(), d, "https://teams.cloud.microsoft/l/meetingrecap?threadId=2", `C:\out\direct.md`)
	require.NoError(t, err)
	assert.Equal(t, `C:\out\direct.md`, gotCfg.OutputFile)
}
