package browser

import (
	"context"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findFlag(flags []Flag, name string) (Flag, bool) {
	for _, f := range flags {
		if f.Name == name {
			return f, true
		}
	}
	return Flag{}, false
}

func TestStealthFlagsPresent(t *testing.T) {
	t.Setenv("TEAMS_DISABLE_SANDBOX", "")
	flags := StealthFlags(Options{Headless: true})
	want := map[string]any{
		"no-sandbox":                         false,
		"disable-setuid-sandbox":             false,
		"use-fake-ui-for-media-stream":       true,
		"use-fake-device-for-media-stream":   true,
		"disable-blink-features":             "AutomationControlled",
		"auto-select-desktop-capture-source": "Microsoft Teams",
		"mute-audio":                         false,
		"headless":                           "new",
	}
	for name, value := range want {
		f, ok := findFlag(flags, name)
		require.Truef(t, ok, "flag %q missing", name)
		assert.Equalf(t, value, f.Value, "flag %q value", name)
	}
}

func TestShouldDisableSandbox(t *testing.T) {
	tests := []struct {
		name string
		opts Options
		env  string
		want bool
	}{
		{"default off", Options{}, "", false},
		{"option on", Options{DisableSandbox: true}, "", true},
		{"env 1", Options{}, "1", true},
		{"env true", Options{}, "true", true},
		{"env TRUE", Options{}, "TRUE", true},
		{"env 0", Options{}, "0", false},
		{"env false", Options{}, "false", false},
		{"env junk", Options{}, "yes please", false},
		{"option on overrides env off", Options{DisableSandbox: true}, "0", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", tc.env)
			assert.Equal(t, tc.want, shouldDisableSandbox(tc.opts))
		})
	}
}

func TestSandboxFlagsDefaultOffAndOptIn(t *testing.T) {
	builders := map[string]func(Options) []Flag{
		"StealthFlags":       StealthFlags,
		"cleanHeadlessFlags": cleanHeadlessFlags,
	}
	for name, build := range builders {
		t.Run(name+"/default", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "")
			for _, flag := range []string{"no-sandbox", "disable-setuid-sandbox"} {
				f, ok := findFlag(build(Options{Headless: true}), flag)
				require.Truef(t, ok, "%s must be emitted explicitly so root defaults are overridden", flag)
				assert.Equal(t, false, f.Value)
			}
		})
		t.Run(name+"/option", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "")
			for _, flag := range []string{"no-sandbox", "disable-setuid-sandbox"} {
				f, ok := findFlag(build(Options{Headless: true, DisableSandbox: true}), flag)
				require.Truef(t, ok, "%s missing", flag)
				assert.Equal(t, true, f.Value)
			}
		})
		t.Run(name+"/env", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "1")
			for _, flag := range []string{"no-sandbox", "disable-setuid-sandbox"} {
				f, ok := findFlag(build(Options{Headless: true}), flag)
				require.Truef(t, ok, "%s missing", flag)
				assert.Equal(t, true, f.Value)
			}
		})
	}
}

// captureLaunchArgs applies base then allocator options, intercepts the browser
// command before it starts (substituting the test binary so nothing launches),
// and returns the Chromium arguments chromedp would have used.
func captureLaunchArgs(t *testing.T, base, allocOpts []chromedp.ExecAllocatorOption) []string {
	t.Helper()
	var got []string
	capture := chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) {
		got = append([]string(nil), cmd.Args...)
		cmd.Args = []string{cmd.Path, "-test.run=^$"}
	})
	all := append([]chromedp.ExecAllocatorOption{}, base...)
	all = append(all, allocOpts...)
	all = append(all, capture, chromedp.WSURLReadTimeout(5*time.Second))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, all...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	// The substituted process exits without a DevTools endpoint, so Run is
	// expected to fail; only the captured arguments matter here.
	_ = chromedp.Run(browserCtx)
	require.NotEmpty(t, got, "browser command was not intercepted")
	return got
}

func TestEffectiveLaunchArgsSandboxOptIn(t *testing.T) {
	self, err := os.Executable()
	require.NoError(t, err)
	// Simulates chromedp's root/container behavior, where NoSandbox is preset
	// ahead of the allocator options.
	rootDefault := []chromedp.ExecAllocatorOption{chromedp.NoSandbox}

	builders := map[string]func(Options) []chromedp.ExecAllocatorOption{
		"AllocatorOptions":              AllocatorOptions,
		"cleanHeadlessAllocatorOptions": cleanHeadlessAllocatorOptions,
	}
	for name, build := range builders {
		t.Run(name+"/default-off", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "")
			opts := Options{Headless: true, ExecPath: self, UserDataDir: t.TempDir()}
			for _, base := range [][]chromedp.ExecAllocatorOption{nil, rootDefault} {
				args := captureLaunchArgs(t, base, build(opts))
				assert.NotContains(t, args, "--no-sandbox")
				assert.NotContains(t, args, "--disable-setuid-sandbox")
			}
		})
		t.Run(name+"/opt-in", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "")
			opts := Options{Headless: true, ExecPath: self, UserDataDir: t.TempDir(), DisableSandbox: true}
			args := captureLaunchArgs(t, nil, build(opts))
			assert.Contains(t, args, "--no-sandbox")
			assert.Contains(t, args, "--disable-setuid-sandbox")
		})
		t.Run(name+"/env-opt-in", func(t *testing.T) {
			t.Setenv("TEAMS_DISABLE_SANDBOX", "true")
			opts := Options{Headless: true, ExecPath: self, UserDataDir: t.TempDir()}
			args := captureLaunchArgs(t, nil, build(opts))
			assert.Contains(t, args, "--no-sandbox")
			assert.Contains(t, args, "--disable-setuid-sandbox")
		})
	}
}

func TestStealthFlagsHeadedDisablesHeadless(t *testing.T) {
	flags := StealthFlags(Options{Headless: false})
	f, ok := findFlag(flags, "headless")
	require.True(t, ok)
	assert.Equal(t, false, f.Value)
}

func TestAllocatorOptionsExtendsDefaults(t *testing.T) {
	opts := AllocatorOptions(Options{UserDataDir: "/tmp/profile", Headless: true})
	assert.Greater(t, len(opts), len(chromedp.DefaultExecAllocatorOptions),
		"allocator options should extend the chromedp defaults")
}

func TestAllocatorOptionsDefaultUserAgent(t *testing.T) {
	// Exercises the default-User-Agent branch without launching a browser.
	assert.NotPanics(t, func() {
		_ = AllocatorOptions(Options{Headless: false})
	})
	assert.Contains(t, DefaultUserAgent, "Windows NT 10.0")
}

func TestResolveExecPathPrecedence(t *testing.T) {
	t.Setenv("TEAMS_BROWSER_PATH", "/env/browser")
	assert.Equal(t, "/explicit/browser", ResolveExecPath("/explicit/browser"),
		"explicit ExecPath must take precedence over TEAMS_BROWSER_PATH")
	assert.Equal(t, "/env/browser", ResolveExecPath(""),
		"TEAMS_BROWSER_PATH must be used when explicit ExecPath is empty")
}

func TestAllocatorOptionsAcceptsExecPath(t *testing.T) {
	opts := AllocatorOptions(Options{
		Headless: true,
		ExecPath: "/custom/chromium",
	})
	assert.Greater(t, len(opts), len(chromedp.DefaultExecAllocatorOptions))
}
