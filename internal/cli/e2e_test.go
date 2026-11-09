package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
)

// TestCaptionsCommandEndToEndWritesMarkdownFile exercises the full CLI MVP:
// running `teams-cli captions <url>` opens the browser, waits for user
// authentication, navigates into the Teams call UI, enables live captions,
// reads caption mutations from the DOM, and saves them to a local Markdown
// transcript file.
func TestCaptionsCommandEndToEndWritesMarkdownFile(t *testing.T) {
	if browser.ResolveExecPath("") == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Microsoft Teams</title></head>
<body>
  <div id="root"><h1>Sign in to Microsoft Teams</h1></div>
  <script>
    const root = document.getElementById('root');
    // Simulate user completing authentication and entering a Teams call.
    setTimeout(() => {
      root.innerHTML = '' +
        '<button id="hangup-button">Leave</button>' +
        '<button id="callingButtons-showMoreBtn">More</button>' +
        '<div id="menu"></div>' +
        '<div id="captions"></div>';
      document.getElementById('callingButtons-showMoreBtn').addEventListener('click', () => {
        document.getElementById('menu').innerHTML =
          '<button id="closed-captions-button">Show live captions</button>';
        document.getElementById('closed-captions-button').addEventListener('click', () => {
          const c = document.getElementById('captions');
          c.innerHTML = '<div data-tid="closed-caption-renderer-wrapper">' +
            '<div class="fui-ChatMessageCompact">' +
            '  <span data-tid="author">Presenter</span>' +
            '  <span data-tid="closed-caption-text">Live captions are now streaming to Markdown.</span>' +
            '</div>' +
            '</div>';
        });
      });
    }, 250);
  </script>
</body>
</html>`))
	}))
	defer srv.Close()

	outDir := t.TempDir()
	profileDir := t.TempDir()
	t.Cleanup(func() {
		for range 10 {
			if os.RemoveAll(profileDir) == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		root := NewRootCommand()
		root.SetArgs([]string{
			"captions",
			srv.URL + "/meet/mvp123?p=testpass",
			"--output-dir", outDir,
			"--profile-dir", profileDir,
			"--headless",
		})
		errCh <- root.ExecuteContext(ctx)
	}()

	// Poll outDir until the flushed Markdown file contains the captured caption,
	// proving real-time disk persistence while the call is still active.
	var mdContent string
	require.Eventually(t, func() bool {
		entries, err := os.ReadDir(outDir)
		if err != nil || len(entries) == 0 {
			return false
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(outDir, e.Name()))
			if err != nil {
				continue
			}
			if strings.Contains(string(b), "Live captions are now streaming to Markdown.") {
				mdContent = string(b)
				return true
			}
		}
		return false
	}, 20*time.Second, 150*time.Millisecond, "expected Markdown transcript file to contain live caption")

	cancel()
	require.NoError(t, <-errCh)

	assert.Contains(t, mdContent, `meeting_id: "meet-mvp123"`)
	assert.Contains(t, mdContent, "# Transcript")
	assert.Contains(t, mdContent, "**Presenter (")
	assert.Contains(t, mdContent, "Live captions are now streaming to Markdown.")
}
