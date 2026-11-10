package browser

import (
	"testing"

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
	flags := StealthFlags(Options{Headless: true})
	want := map[string]any{
		"no-sandbox":                         true,
		"disable-setuid-sandbox":             true,
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
