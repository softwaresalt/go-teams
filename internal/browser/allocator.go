// Package browser constructs chromedp allocator options and drives a headless
// Chromium instance against Microsoft Teams.
package browser

import "github.com/chromedp/chromedp"

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

// AllocatorOptions builds the chromedp ExecAllocator option slice from opts,
// applying the stealth flags, the persistent user-data directory, and a stable
// User-Agent.
func AllocatorOptions(opts Options) []chromedp.ExecAllocatorOption {
	out := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	for _, f := range StealthFlags(opts) {
		out = append(out, chromedp.Flag(f.Name, f.Value))
	}
	if opts.UserDataDir != "" {
		out = append(out, chromedp.UserDataDir(opts.UserDataDir))
	}
	ua := opts.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	out = append(out, chromedp.UserAgent(ua))
	return out
}
