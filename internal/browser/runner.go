package browser

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/softwaresalt/go-teams/internal/teams"
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
	if strings.TrimSpace(r.options.DefaultAccount) == "" {
		return ErrAccountRequired
	}
	opts := r.options
	opts.Headless = false
	return r.run(ctx, opts, teamsHomeURL)
}

// Join implements Runner. It forces headless mode and navigates to meetingURL.
// meetingURL is validated before any browser is allocated, so a disallowed
// initial URL never launches Chromium.
func (r *ChromeRunner) Join(ctx context.Context, meetingURL string) error {
	if err := teams.ValidateMeetingURL(meetingURL); err != nil {
		return fmt.Errorf("browser: reject meeting url: %w", err)
	}
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
		return navigateError(target, err)
	}
	account := strings.TrimSpace(opts.DefaultAccount)
	if account == "" {
		<-browserCtx.Done()
		return nil
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	js := selectEntraAccountJS(account)
	redacted := redactAccountIdentifier(account)
	for {
		select {
		case <-browserCtx.Done():
			return nil
		case <-ticker.C:
			var clicked bool
			if err := chromedp.Run(browserCtx, chromedp.Evaluate(js, &clicked)); err == nil && clicked {
				slog.Info("browser: selected default Entra account", "account", redacted)
			}
		}
	}
}

// navigateError wraps a navigation failure, replacing target with its
// RedactURL form so passcodes, credentials, and tokens never reach logs.
func navigateError(target string, err error) error {
	return fmt.Errorf("browser: navigate to %s: %w", teams.RedactURL(target), err)
}
