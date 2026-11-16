package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/browser"
)

// TestCaptionsCommandEndToEndWritesMarkdownFile exercises the full CLI MVP:
// running `go-teams captions <url>` opens the browser, waits for user
// authentication, navigates into the Teams call UI, enables live captions,
// reads caption mutations from the DOM, and saves them to a local Markdown
// transcript file.
func TestCaptionsCommandEndToEndWritesMarkdownFile(t *testing.T) {
	loadCLITestEnv(t)
	if runtime.GOOS != "windows" {
		t.Skip("live browser e2e test targets Windows consumer hosts only")
	}
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

	assert.Contains(t, mdContent, "# Meeting Attendees")
	assert.Contains(t, mdContent, "**Meeting:** meet-mvp123")
	assert.Contains(t, mdContent, "## Transcript")
	assert.Contains(t, mdContent, "**Presenter** (")
	assert.Contains(t, mdContent, "\n> Live captions are now streaming to Markdown.")
}

// TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd exercises
// `go-teams join --mode live-with-summary` across two headless Chromium sessions:
// scheduled join -> overtime caption continuation -> call-end silence shutdown ->
// transcription_queue.txt persistence -> delayed Recap AI Summary append and
// `::` queue completion.
func TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd(t *testing.T) {
	loadCLITestEnv(t)
	for _, k := range []string{
		"TEAMS_ACCOUNT",
		"TEAMS_TEST_ACCOUNT",
		"TEAMS_MEETING_URL",
		"TEAMS_TEST_MEETING_URL",
		"TEAMS_RECAP_URL",
		"TEAMS_TEST_RECAP_URL",
		"TEAMS_PROFILE_DIR",
		"TEAMS_TEST_PROFILE_DIR",
		"GO_TEAMS_BINARY",
	} {
		t.Setenv(k, "")
	}
	if runtime.GOOS != "windows" {
		t.Skip("live browser e2e test targets Windows consumer hosts only")
	}
	if browser.ResolveExecPath("") == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	var visits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/meet/") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		visit := visits.Add(1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if visit == 1 {
			// httptest handler; write failure surfaces as test timeout.
			_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Microsoft Teams</title></head>
<body>
  <div id="app">
    <button data-tid="joinOnWeb">Continue on this browser</button>
  </div>
  <script>
    const app = document.getElementById('app');
    document.querySelector('[data-tid="joinOnWeb"]').addEventListener('click', showPreJoin);

    function showPreJoin() {
      app.innerHTML = '' +
        '<input type="checkbox" role="switch" data-tid="toggle-mute" checked />' +
        '<input data-tid="prejoin-display-name-input" value="" />' +
        '<button data-tid="prejoin-join-button">Join now</button>';
      document.querySelector('[data-tid="prejoin-join-button"]').addEventListener('click', showLobby);
    }

    function showLobby() {
      app.innerHTML = '<div>Someone will let you in shortly</div>';
      setTimeout(showCall, 150);
    }

    function showCall() {
      app.innerHTML = '' +
        '<div id="toolbar">' +
        '  <button id="hangup-button">Leave</button>' +
        '  <button id="callingButtons-showMoreBtn">More</button>' +
        '</div>' +
        '<div id="menu"></div>' +
        '<div id="stage"></div>';
      document.getElementById('callingButtons-showMoreBtn').addEventListener('click', () => {
        document.getElementById('menu').innerHTML =
          '<button id="closed-captions-button">Turn on live captions</button>';
        document.getElementById('closed-captions-button').addEventListener('click', startCaptions);
      });
    }

    function startCaptions() {
      const stage = document.getElementById('stage');
      stage.innerHTML = '<div data-tid="closed-caption-renderer-wrapper" id="cc"></div>';
      const cc = document.getElementById('cc');

      function emitWhenObserverReady() {
        if (typeof window.sendCaptionToGo !== 'function') {
          setTimeout(emitWhenObserverReady, 50);
          return;
        }
        setTimeout(() => {
          cc.innerHTML = '' +
            '<div class="fui-ChatMessageCompact" id="msg1">' +
            '  <span data-tid="author">Alice</span>' +
            '  <span data-tid="closed-caption-text" id="t1">Initial scheduled utterance.</span>' +
            '</div>';
        }, 150);

        setTimeout(() => {
          const div = document.createElement('div');
          div.className = 'fui-ChatMessageCompact';
          div.innerHTML = '' +
            '<span data-tid="author">Alice</span>' +
            '  <span data-tid="closed-caption-text" id="t2">Overtime scheduled utterance</span>';
          cc.appendChild(div);
        }, 1000);

        setTimeout(() => {
          const t2 = document.getElementById('t2');
          if (t2) {
            t2.textContent = 'Overtime scheduled utterance after nominal duration.';
          }
        }, 1900);

        setTimeout(() => {
          app.innerHTML = '<div data-tid="call-ended-screen">You left the meeting</div>';
        }, 2100);
      }

      emitWhenObserverReady();
    }
  </script>
</body>
</html>`))
			return
		}

		// httptest handler; write failure surfaces as test timeout.
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Meeting Recap</title></head>
<body>
  <div data-tid="intelligent-recap-header">Recap</div>
  <button role="tab" data-tid="Notes" aria-selected="false">Notes</button>
  <button role="tab" data-tid="AINotes" aria-selected="false">AI notes</button>
  <button aria-label="Copy all">Copy all</button>
  <script>
    document.querySelector('[data-tid="Notes"]').addEventListener('click', (e) => {
      e.currentTarget.setAttribute('aria-selected', 'true');
    });
    document.querySelector('[data-tid="AINotes"]').addEventListener('click', (e) => {
      e.currentTarget.setAttribute('aria-selected', 'true');
    });
    document.querySelector('button[aria-label="Copy all"]').addEventListener('click', () => {
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText("Summary notes:\nOvertime decision recorded.");
      }
    });
  </script>
</body>
</html>`))
	}))
	defer srv.Close()

	workDir := t.TempDir()
	outDir := filepath.Join(workDir, "out")
	require.NoError(t, os.MkdirAll(outDir, 0o700))
	profileDir := filepath.Join(workDir, "profile")
	queuePath := filepath.Join(workDir, "transcription_queue.txt")
	cfgPath := filepath.Join(workDir, "config.yaml")
	cfgYAML := fmt.Sprintf(
		"output_dir: %q\nprofile_dir: %q\nbot_name: \"Sched Bot\"\nqueue_file: %q\n",
		filepath.ToSlash(outDir),
		filepath.ToSlash(profileDir),
		filepath.ToSlash(queuePath),
	)
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o600))

	t.Cleanup(func() {
		for range 10 {
			if os.RemoveAll(profileDir) == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	root := NewRootCommand()
	root.SetArgs([]string{
		"join",
		srv.URL + "/meet/sched123?p=synthetic-placeholder-passcode",
		"--config", cfgPath,
		"--mode", "live-with-summary",
		"--duration", "1500ms",
		"--idle-timeout", "2500ms",
		"--ai-summary-delay", "250ms",
		"--category", "SCHED",
		"--output-dir", outDir,
		"--profile-dir", profileDir,
		"--headless",
	})

	require.NoError(t, root.ExecuteContext(ctx))
	require.NoError(t, ctx.Err())

	entries, err := os.ReadDir(outDir)
	require.NoError(t, err)
	var mdPath string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			mdPath = filepath.Join(outDir, e.Name())
			break
		}
	}
	require.NotEmpty(t, mdPath, "expected generated Markdown transcript in output directory")

	mdBytes, err := os.ReadFile(mdPath)
	require.NoError(t, err)
	mdContent := string(mdBytes)
	assert.Contains(t, mdContent, "Initial scheduled utterance.")
	assert.Contains(t, mdContent, "Overtime scheduled utterance after nominal duration.")
	assert.Contains(t, mdContent, "# AI Summary")
	assert.Contains(t, mdContent, "* **Summary notes:** Overtime decision recorded.")

	qBytes, err := os.ReadFile(queuePath)
	require.NoError(t, err)
	qContent := string(qBytes)
	assert.Contains(t, qContent, "--append-ai-summary")
	assert.Contains(t, qContent, "--not-before")
	var completedLine bool
	for _, line := range strings.Split(strings.ReplaceAll(qContent, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "::") && strings.Contains(line, "--append-ai-summary") {
			completedLine = true
			break
		}
	}
	assert.True(t, completedLine, "expected queue entry to be prefixed with :: after delayed AI Summary enrichment")

	// Phase 2: Early in-browser call-end must stop capture well before a long
	// nominal --duration (60s) under a 15s outer context timeout.
	earlySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/meet/") {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// httptest handler; write failure surfaces as test timeout.
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Microsoft Teams</title></head>
<body>
  <div id="app">
    <button data-tid="joinOnWeb">Continue on this browser</button>
  </div>
  <script>
    const app = document.getElementById('app');
    document.querySelector('[data-tid="joinOnWeb"]').addEventListener('click', showPreJoin);

    function showPreJoin() {
      app.innerHTML = '' +
        '<input type="checkbox" role="switch" data-tid="toggle-mute" checked />' +
        '<input data-tid="prejoin-display-name-input" value="" />' +
        '<button data-tid="prejoin-join-button">Join now</button>';
      document.querySelector('[data-tid="prejoin-join-button"]').addEventListener('click', showLobby);
    }

    function showLobby() {
      app.innerHTML = '<div>Someone will let you in shortly</div>';
      setTimeout(showCall, 100);
    }

    function showCall() {
      app.innerHTML = '' +
        '<div id="toolbar">' +
        '  <button id="hangup-button">Leave</button>' +
        '  <button id="callingButtons-showMoreBtn">More</button>' +
        '</div>' +
        '<div id="menu"></div>' +
        '<div id="stage"></div>';
      document.getElementById('callingButtons-showMoreBtn').addEventListener('click', () => {
        document.getElementById('menu').innerHTML =
          '<button id="closed-captions-button">Turn on live captions</button>';
        document.getElementById('closed-captions-button').addEventListener('click', startCaptions);
      });
    }

    function startCaptions() {
      const stage = document.getElementById('stage');
      stage.innerHTML = '<div data-tid="closed-caption-renderer-wrapper" id="cc"></div>';
      const cc = document.getElementById('cc');

      function emitWhenObserverReady() {
        if (typeof window.sendCaptionToGo !== 'function') {
          setTimeout(emitWhenObserverReady, 50);
          return;
        }
        setTimeout(() => {
          cc.innerHTML = '' +
            '<div class="fui-ChatMessageCompact" id="msg1">' +
            '  <span data-tid="author">Bob</span>' +
            '  <span data-tid="closed-caption-text">Early call-end utterance.</span>' +
            '</div>';
        }, 150);
        setTimeout(() => {
          app.innerHTML = '<div data-tid="call-ended-screen">You left the meeting</div>';
        }, 350);
      }

      emitWhenObserverReady();
    }
  </script>
</body>
</html>`))
	}))
	defer earlySrv.Close()

	earlyDir := t.TempDir()
	earlyOutDir := filepath.Join(earlyDir, "out")
	require.NoError(t, os.MkdirAll(earlyOutDir, 0o700))
	earlyProfileDir := filepath.Join(earlyDir, "profile")
	earlyCfgPath := filepath.Join(earlyDir, "config.yaml")
	earlyCfgYAML := fmt.Sprintf(
		"output_dir: %q\nprofile_dir: %q\nbot_name: \"Sched Bot\"\n",
		filepath.ToSlash(earlyOutDir),
		filepath.ToSlash(earlyProfileDir),
	)
	require.NoError(t, os.WriteFile(earlyCfgPath, []byte(earlyCfgYAML), 0o600))
	t.Cleanup(func() {
		for range 10 {
			if os.RemoveAll(earlyProfileDir) == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	})

	earlyCtx, earlyCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer earlyCancel()

	earlyRoot := NewRootCommand()
	earlyRoot.SetArgs([]string{
		"join",
		earlySrv.URL + "/meet/earlyend123?p=synthetic-placeholder-passcode",
		"--config", earlyCfgPath,
		"--mode", "live-with-summary",
		"--duration", "60s",
		"--idle-timeout", "800ms",
		"--no-ai-summary",
		"--output-dir", earlyOutDir,
		"--profile-dir", earlyProfileDir,
		"--headless",
	})
	require.NoError(t, earlyRoot.ExecuteContext(earlyCtx))
	require.NoError(t, earlyCtx.Err())

	earlyEntries, err := os.ReadDir(earlyOutDir)
	require.NoError(t, err)
	var earlyMDPath string
	for _, e := range earlyEntries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			earlyMDPath = filepath.Join(earlyOutDir, e.Name())
			break
		}
	}
	require.NotEmpty(t, earlyMDPath, "expected early call-end Markdown transcript")
	earlyMDBytes, err := os.ReadFile(earlyMDPath)
	require.NoError(t, err)
	assert.Contains(t, string(earlyMDBytes), "Early call-end utterance.")
}
