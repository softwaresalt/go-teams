// Package browser constructs chromedp allocator options and drives a headless
// Chromium instance against Microsoft Teams.
package browser

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/chromedp/chromedp"
)

// DefaultUserAgent is a fixed Windows Chromium User-Agent string. Teams serves
// DOM structure keyed to the User-Agent, so a stable value keeps CSS selectors
// consistent regardless of the host operating system.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36"

// Options configures the allocator flag set.
type Options struct {
	UserDataDir string
	Headless    bool
	UserAgent   string
	ExecPath    string
}

// Flag is a single Chromium command-line flag with its value. A bool value of
// true renders as a valueless flag (e.g. --no-sandbox); a string value renders
// as --name=value; a bool value of false disables a flag enabled by default.
type Flag struct {
	Name  string
	Value any
}

// StealthFlags returns the ordered set of Chromium flags that stabilize the
// headless environment, grant synthetic media permissions, and mask automation
// from Teams' bot-detection telemetry. The trailing headless flag reflects
// opts.Headless: "new" when headless, or an explicit disable when headed.
func StealthFlags(opts Options) []Flag {
	flags := []Flag{
		{Name: "no-sandbox", Value: true},
		{Name: "disable-setuid-sandbox", Value: true},
		{Name: "use-fake-ui-for-media-stream", Value: true},
		{Name: "use-fake-device-for-media-stream", Value: true},
		{Name: "disable-blink-features", Value: "AutomationControlled"},
		{Name: "auto-select-desktop-capture-source", Value: "Microsoft Teams"},
		{Name: "mute-audio", Value: false},
	}
	if opts.Headless {
		flags = append(flags, Flag{Name: "headless", Value: "new"})
	} else {
		flags = append(flags, Flag{Name: "headless", Value: false})
	}
	return flags
}

// ResolveExecPath returns the Chromium-based browser executable path to launch.
// It checks, in order: explicit, the TEAMS_BROWSER_PATH environment variable,
// PATH lookup for Chrome/Chromium/Edge/Brave, and standard OS install paths
// (including Microsoft Edge on Windows where chromedp's default lookup only
// checks for Google Chrome).
func ResolveExecPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if env := os.Getenv("TEAMS_BROWSER_PATH"); env != "" {
		return env
	}
	pathNames := []string{
		"google-chrome",
		"google-chrome-stable",
		"chromium",
		"chromium-browser",
		"chrome",
		"msedge",
		"microsoft-edge",
		"microsoft-edge-stable",
		"brave",
		"brave-browser",
	}
	for _, name := range pathNames {
		if p, err := exec.LookPath(name); err == nil && p != "" {
			return p
		}
	}
	for _, candidate := range candidateBrowserPaths() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func candidateBrowserPaths() []string {
	var paths []string
	for _, root := range []string{
		os.Getenv("ProgramFiles"),
		os.Getenv("ProgramFiles(x86)"),
		os.Getenv("LOCALAPPDATA"),
	} {
		if root == "" {
			continue
		}
		paths = append(paths,
			filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(root, "BraveSoftware", "Brave-Browser", "Application", "brave.exe"),
			filepath.Join(root, "Chromium", "Application", "chrome.exe"),
		)
	}
	paths = append(paths,
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/usr/bin/microsoft-edge",
		"/usr/bin/microsoft-edge-stable",
		"/usr/bin/brave-browser",
	)
	return paths
}

// AllocatorOptions builds the chromedp ExecAllocator option slice from opts,
// applying the stealth flags, the persistent user-data directory, a resolved
// browser executable path, and a stable User-Agent.
func AllocatorOptions(opts Options) []chromedp.ExecAllocatorOption {
	out := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	for _, f := range StealthFlags(opts) {
		out = append(out, chromedp.Flag(f.Name, f.Value))
	}
	if opts.UserDataDir != "" {
		dir := opts.UserDataDir
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
		out = append(out, chromedp.UserDataDir(dir))
	}
	if execPath := ResolveExecPath(opts.ExecPath); execPath != "" {
		out = append(out, chromedp.ExecPath(execPath))
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	out = append(out, chromedp.UserAgent(ua))
	return out
}
