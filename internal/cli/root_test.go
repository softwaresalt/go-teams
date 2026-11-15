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
	"github.com/spf13/cobra"
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
	return config.Config{
		OutputDir:      "out",
		ProfileDir:     "prof",
		BotName:        "bot",
		DefaultAccount: "user@example.com",
	}
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

func TestAuthCommandRequiresAccount(t *testing.T) {
	fake := &fakeRunner{}
	d := &deps{
		loadConfig: func(string) (config.Config, error) {
			return config.Config{OutputDir: "out", ProfileDir: "prof", BotName: "bot"}, nil
		},
		newRunner: func(config.Config) browser.Runner { return fake },
	}
	err := runCmd(context.Background(), d, "auth")
	require.ErrorIs(t, err, browser.ErrAccountRequired)
	assert.False(t, fake.authCalled, "Authenticate must not be called when account is missing")

	require.NoError(t, runCmd(context.Background(), d, "auth", "--account", "alice@contoso.com"))
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
	for _, want := range []string{"auth", "join", "captions", "schedule", "update"} {
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

func TestRecapAudioFlagAndQueueForwarding(t *testing.T) {
	for _, name := range []string{"root", "captions", "join"} {
		for _, flag := range []string{"audio", "recap-audio"} {
			for _, value := range []string{"true", "false"} {
				t.Run(name+"/"+flag+"="+value, func(t *testing.T) {
					root := newRootCommand(defaultDeps())
					cmd := root
					if name != "root" {
						var err error
						cmd, _, err = root.Find([]string{name})
						if err != nil {
							t.Fatalf("find command: %v", err)
						}
					}
					if cmd.Flags().Lookup(flag) == nil {
						t.Fatalf("%s does not register --%s", name, flag)
					}
					// Bind directly so the same flag state can be checked in both
					// applyCaptureFlags and inheritedQueueFlags.
					var f captureFlags
					testCmd := &cobra.Command{Use: name}
					bindCaptureFlags(testCmd, &f)
					if err := testCmd.Flags().Parse([]string{"--" + flag + "=" + value}); err != nil {
						t.Fatalf("parse flag: %v", err)
					}
					cfg := config.Config{CaptureAudio: value == "false"}
					applyCaptureFlags(testCmd, &cfg, f)
					if cfg.CaptureAudio != (value == "true") {
						t.Errorf("CaptureAudio = %v for --%s=%s", cfg.CaptureAudio, flag, value)
					}
					want := "--" + flag
					if value == "false" {
						want += "=false"
					}
					got := inheritedQueueFlags(testCmd, "", f)
					if len(got) != 1 || got[0] != want {
						t.Errorf("queue flags = %v, want [%s]", got, want)
					}
				})
			}
		}
	}
}

func TestDefaultAccountFlagAndQueueForwarding(t *testing.T) {
	const (
		meetingURL = "https://teams.microsoft.com/meet/123"
		cfgAcct    = "config-user@contoso.com"
		flagAcct   = "flag-user@contoso.com"
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
	for _, cmdCase := range commands {
		t.Run(cmdCase.name+"/flag overrides config", func(t *testing.T) {
			loaded := validConfig()
			loaded.DefaultAccount = cfgAcct
			loaded.Meetings = []config.MeetingSchedule{{Name: "m", URL: meetingURL, Cron: "@every 1s"}}
			probe := newBrowserPathProbe()
			args := append(append([]string{}, cmdCase.args...), "--account", "  "+flagAcct+"  ")

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

			assert.Equal(t, flagAcct, got.DefaultAccount)
			opts := browserOptions(got)
			assert.Equal(t, flagAcct, opts.DefaultAccount)
		})

		t.Run(cmdCase.name+"/empty flag clears config default_account", func(t *testing.T) {
			loaded := validConfig()
			loaded.DefaultAccount = cfgAcct
			loaded.Meetings = []config.MeetingSchedule{{Name: "m", URL: meetingURL, Cron: "@every 1s"}}
			probe := newBrowserPathProbe()
			args := append(append([]string{}, cmdCase.args...), "--account=")

			if cmdCase.name == "auth" {
				err := runCmd(context.Background(), probe.deps(loaded), args...)
				require.ErrorIs(t, err, browser.ErrAccountRequired)
				return
			}

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

			assert.Empty(t, got.DefaultAccount)
			opts := browserOptions(got)
			assert.Empty(t, opts.DefaultAccount)
		})
	}

	t.Run("inheritedQueueFlags forwards --account and preserves empty --account=", func(t *testing.T) {
		var f captureFlags
		testCmd := &cobra.Command{Use: "root"}
		bindCaptureFlags(testCmd, &f)
		require.NoError(t, testCmd.Flags().Parse([]string{"--account", "queue-user@contoso.com"}))
		got := inheritedQueueFlags(testCmd, "", f)
		assert.Equal(t, []string{"--account", "queue-user@contoso.com"}, got)

		var emptyFlags captureFlags
		emptyCmd := &cobra.Command{Use: "root"}
		bindCaptureFlags(emptyCmd, &emptyFlags)
		require.NoError(t, emptyCmd.Flags().Parse([]string{"--account="}))
		assert.Equal(t, []string{"--account="}, inheritedQueueFlags(emptyCmd, "", emptyFlags))
	})
}
