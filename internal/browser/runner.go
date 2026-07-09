package browser

import (
	"context"
	"fmt"

	"github.com/chromedp/chromedp"
)

const teamsHomeURL = "https://teams.microsoft.com"

// Runner drives a Chromium browser through the Teams authentication and
// meeting-join flows.
type Runner interface {
	// Authenticate launches a visible browser for interactive Microsoft 365
	// login and blocks until ctx is cancelled, persisting session state to the
	// configured user-data directory.
	Authenticate(ctx context.Context) error
	// Join launches a headless browser, navigates to meetingURL, and blocks
	// until ctx is cancelled.
	Join(ctx context.Context, meetingURL string) error
}

// ChromeRunner is the production Runner backed by chromedp. It launches the
// host's Chromium-based browser and drives it over the DevTools protocol.
type ChromeRunner struct {
	options Options
}

// NewChromeRunner returns a ChromeRunner using opts as the allocator base
// configuration. The Headless field is overridden per flow.
func NewChromeRunner(opts Options) *ChromeRunner {
	return &ChromeRunner{options: opts}
}

// Authenticate implements Runner. It forces a headed browser so the user can
// complete the Microsoft 365 login and MFA challenges manually.
func (r *ChromeRunner) Authenticate(ctx context.Context) error {
	opts := r.options
	opts.Headless = false
	return r.run(ctx, opts, teamsHomeURL)
}

// Join implements Runner. It forces headless mode and navigates to meetingURL.
func (r *ChromeRunner) Join(ctx context.Context, meetingURL string) error {
	opts := r.options
	opts.Headless = true
	return r.run(ctx, opts, meetingURL)
}

// run allocates a chromedp browser, navigates to target, and blocks until ctx
// is cancelled or the browser exits. The deferred cancel funcs guarantee the
// spawned Chromium process is terminated, preventing zombie processes.
func (r *ChromeRunner) run(ctx context.Context, opts Options, target string) error {
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, AllocatorOptions(opts)...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	if err := chromedp.Run(browserCtx, chromedp.Navigate(target)); err != nil {
		return fmt.Errorf("browser: navigate to %s: %w", target, err)
	}
	<-browserCtx.Done()
	return nil
}
