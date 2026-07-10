//go:build integration

package browser_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
)

// requireIntegration skips unless GO_TEAMS_INTEGRATION is set, so the harness
// never launches a real browser in unattended CI.
func requireIntegration(t *testing.T) {
	t.Helper()
	if os.Getenv("GO_TEAMS_INTEGRATION") == "" {
		t.Skip("set GO_TEAMS_INTEGRATION=1 to run browser integration tests")
	}
}

// TestChromeRunnerLaunch verifies that AllocatorOptions produce a working
// chromedp browser that can navigate and read the DOM. Requires a
// Chromium-based browser on the host. This validates the existing browser
// allocator layer end-to-end.
func TestChromeRunnerLaunch(t *testing.T) {
	requireIntegration(t)

	opts := browser.AllocatorOptions(browser.Options{Headless: true})
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	runCtx, cancelTimeout := context.WithTimeout(browserCtx, 30*time.Second)
	defer cancelTimeout()

	var title string
	err := chromedp.Run(runCtx,
		chromedp.Navigate("https://example.com"),
		chromedp.Title(&title),
	)
	require.NoError(t, err)
	require.Contains(t, title, "Example")
}
