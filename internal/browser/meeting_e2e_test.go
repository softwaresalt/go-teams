package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestChromeMeetingEndToEnd verifies the full browser automation flow against a
// realistic simulated Teams SPA served over httptest:
//  1. User authentication screen (waits until user completes login)
//  2. /dl/launcher/launcher.html interstitial (clicks button[data-tid="joinOnWeb"])
//  3. Pre-join screen (toggles checked mic switch off, injects bot name, clicks Join)
//  4. Lobby waiting room -> admitted active call (#hangup-button)
//  5. Caption menu navigation (#callingButtons-showMoreBtn -> #LanguageSpeechMenuControl-id -> #closed-captions-button)
//  6. Live caption DOM MutationObserver (rolling text + multi-turn same-speaker + settled unpunctuated caption)
func TestChromeMeetingEndToEnd(t *testing.T) {
	LoadTestEnv(t)
	if runtime.GOOS != "windows" {
		t.Skip("live browser e2e test targets Windows consumer hosts only")
	}
	if ResolveExecPath("") == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>Microsoft Teams</title></head>
<body>
  <div id="app">
    <h1>Sign in to your Microsoft account</h1>
    <p id="status">Waiting for user authentication...</p>
  </div>
  <script>
    const app = document.getElementById('app');

    // Step 1 -> Step 2: Simulate user completing login after 300ms, redirecting to launcher.
    setTimeout(() => {
      app.innerHTML = '<button data-tid="joinOnWeb">Continue on this browser</button>';
      document.querySelector('[data-tid="joinOnWeb"]').addEventListener('click', showPreJoin);
    }, 300);

    function showPreJoin() {
      app.innerHTML = '' +
        '<input type="checkbox" role="switch" data-tid="toggle-mute" checked />' +
        '<input data-tid="prejoin-display-name-input" value="" />' +
        '<button data-tid="prejoin-join-button">Join now</button>';
      document.querySelector('[data-tid="prejoin-join-button"]').addEventListener('click', () => {
        const mic = document.querySelector('[data-tid="toggle-mute"]');
        const nameInput = document.querySelector('[data-tid="prejoin-display-name-input"]');
        window.__joinedMuted = !mic.checked;
        window.__joinedName = nameInput.value;
        showLobby();
      });
    }

    function showLobby() {
      app.innerHTML = '<div>Someone will let you in shortly</div>';
      setTimeout(showCall, 300);
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
          '<button id="LanguageSpeechMenuControl-id">Language and speech</button>';
        document.getElementById('LanguageSpeechMenuControl-id').addEventListener('click', () => {
          document.getElementById('menu').innerHTML =
            '<button id="closed-captions-button">Turn on live captions</button>';
          document.getElementById('closed-captions-button').addEventListener('click', startCaptions);
        });
      });
    }

    function startCaptions() {
      const stage = document.getElementById('stage');
      stage.innerHTML = '<div data-tid="closed-caption-renderer-wrapper" id="cc"></div>';
      const cc = document.getElementById('cc');

      // Utterance 1: Alice rolling text ending with punctuation.
      setTimeout(() => {
        cc.innerHTML = '' +
          '<div class="fui-ChatMessageCompact" id="msg1">' +
          '  <span data-tid="author">Alice</span>' +
          '  <span data-tid="closed-caption-text" id="t1">Welcome to the</span>' +
          '</div>';
      }, 100);
      setTimeout(() => {
        document.getElementById('t1').textContent = 'Welcome to the live caption MVP.';
      }, 250);

      // Utterance 2: Second turn from Alice in a second caption block (must not re-emit msg1).
      setTimeout(() => {
        const div = document.createElement('div');
        div.className = 'fui-ChatMessageCompact';
        div.innerHTML = '' +
          '<span data-tid="author">Alice</span>' +
          '<span data-tid="closed-caption-text" id="t2">Second sentence from Alice.</span>';
        cc.appendChild(div);
      }, 450);

      // Utterance 3: Bob speaks without trailing punctuation and pauses (settled timer).
      setTimeout(() => {
        const div = document.createElement('div');
        div.className = 'fui-ChatMessageCompact';
        div.innerHTML = '' +
          '<span data-tid="author">Bob</span>' +
          '<span data-tid="closed-caption-text" id="t3">Sounds good to me</span>';
        cc.appendChild(div);
      }, 650);
    }
  </script>
</body>
</html>`))
	}))
	defer srv.Close()

	profileDir := t.TempDir()
	t.Cleanup(func() {
		for range 10 {
			if os.RemoveAll(profileDir) == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	m, bctx, cleanup, err := NewLiveMeeting(
		context.Background(),
		Options{UserDataDir: profileDir, Headless: true},
		MeetingOptions{MeetingURL: srv.URL, BotName: "MVP Bot"},
	)
	require.NoError(t, err)
	defer cleanup()

	ctx, cancel := context.WithTimeout(bctx, 30*time.Second)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	var (
		mu       sync.Mutex
		captured [][2]string
		doneCh   = make(chan struct{})
	)
	errCh := make(chan error, 1)
	go func() {
		errCh <- m.EnableCaptions(ctx, func(speaker, text string) {
			mu.Lock()
			captured = append(captured, [2]string{speaker, text})
			if len(captured) == 3 {
				select {
				case <-doneCh:
				default:
					close(doneCh)
				}
			}
			mu.Unlock()
		})
	}()

	select {
	case <-doneCh:
		cancel()
	case err := <-errCh:
		require.NoError(t, err)
		t.Fatal("EnableCaptions returned before delivering all 3 captions")
	case <-ctx.Done():
		mu.Lock()
		gotSoFar := append([][2]string(nil), captured...)
		mu.Unlock()
		t.Fatalf("timed out waiting for captions; got %v", gotSoFar)
	}

	require.NoError(t, <-errCh)

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, [][2]string{
		{"Alice", "Welcome to the live caption MVP."},
		{"Alice", "Second sentence from Alice."},
		{"Bob", "Sounds good to me"},
	}, captured)
}

// TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd verifies scheduled
// live call overtime caption activity, in-browser call-end teardown detection,
// and ForceRecap + AISummaryOnly extraction against simulated Teams SPAs in
// real headless Chromium.
func TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd(t *testing.T) {
	LoadTestEnv(t)
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
	if ResolveExecPath("") == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	// Phase 1: Overtime caption activity (interim + final) and call-end teardown.
	func() {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
      setTimeout(showCall, 200);
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
          '<button id="LanguageSpeechMenuControl-id">Language and speech</button>';
        document.getElementById('LanguageSpeechMenuControl-id').addEventListener('click', () => {
          document.getElementById('menu').innerHTML =
            '<button id="closed-captions-button">Turn on live captions</button>';
          document.getElementById('closed-captions-button').addEventListener('click', startCaptions);
        });
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
            '  <span data-tid="closed-caption-text" id="t1">Overtime discussion still</span>' +
            '</div>';
        }, 100);

        setTimeout(() => {
          const el = document.getElementById('t1');
          if (el) {
            el.textContent = 'Overtime discussion still active.';
          }
        }, 700);

        setTimeout(() => {
          app.innerHTML = '<div data-tid="call-ended-screen">You left the meeting</div>';
        }, 1100);
      }

      emitWhenObserverReady();
    }
  </script>
</body>
</html>`))
		}))
		defer srv.Close()

		liveProfileDir := t.TempDir()
		t.Cleanup(func() {
			for range 10 {
				if os.RemoveAll(liveProfileDir) == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		})

		var (
			captionActivityCount atomic.Int32
			finalCaptionOnce     sync.Once
			finalCaptionCh       = make(chan struct{})
			callEndedOnce        sync.Once
			callEndedCh          = make(chan struct{})
			mu                   sync.Mutex
			captured             [][2]string
		)

		m, bctx, cleanup, err := NewLiveMeeting(
			context.Background(),
			Options{UserDataDir: liveProfileDir, Headless: true},
			MeetingOptions{
				MeetingURL:   srv.URL + "/meet/sched-e2e?p=synthetic-placeholder-passcode",
				BotName:      "Sched Bot",
				LobbyTimeout: 20 * time.Second,
				OnCaptionActivity: func() {
					captionActivityCount.Add(1)
				},
				OnCallEnded: func() {
					callEndedOnce.Do(func() { close(callEndedCh) })
				},
			},
		)
		require.NoError(t, err)
		defer cleanup()

		cm, ok := m.(*chromeMeeting)
		require.True(t, ok)
		cm.callEndPoll = 250 * time.Millisecond

		ctx, cancel := context.WithTimeout(bctx, 30*time.Second)
		defer cancel()

		require.NoError(t, m.Join(ctx))

		errCh := make(chan error, 1)
		go func() {
			errCh <- m.EnableCaptions(ctx, func(speaker, text string) {
				mu.Lock()
				captured = append(captured, [2]string{speaker, text})
				mu.Unlock()
				if text == "Overtime discussion still active." {
					finalCaptionOnce.Do(func() { close(finalCaptionCh) })
				}
			})
		}()

		select {
		case <-finalCaptionCh:
		case err := <-errCh:
			require.NoError(t, err)
			t.Fatal("EnableCaptions returned before finalized caption arrived")
		case <-ctx.Done():
			t.Fatalf("timed out waiting for finalized caption: %v", ctx.Err())
		}

		select {
		case <-callEndedCh:
		case err := <-errCh:
			require.NoError(t, err)
			t.Fatal("EnableCaptions returned before OnCallEnded fired")
		case <-ctx.Done():
			t.Fatalf("timed out waiting for OnCallEnded: %v", ctx.Err())
		}

		cancel()
		require.NoError(t, <-errCh)

		assert.GreaterOrEqual(t, captionActivityCount.Load(), int32(2))
		mu.Lock()
		assert.Equal(t, [][2]string{{"Alice", "Overtime discussion still active."}}, captured)
		mu.Unlock()
	}()

	// Phase 2: ForceRecap + AISummaryOnly extraction.
	func() {
		recapSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasPrefix(r.URL.Path, "/meet/") {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			// httptest handler; write failure surfaces as test timeout.
			_, _ = w.Write([]byte(`<!doctype html>
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
        navigator.clipboard.writeText("Meeting notes:\nFollow-up actions confirmed.");
      }
    });
  </script>
</body>
</html>`))
		}))
		defer recapSrv.Close()

		recapProfileDir := t.TempDir()
		t.Cleanup(func() {
			for range 10 {
				if os.RemoveAll(recapProfileDir) == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		})

		recapMeeting, recapBctx, recapCleanup, err := NewLiveMeeting(
			context.Background(),
			Options{UserDataDir: recapProfileDir, Headless: true},
			MeetingOptions{
				MeetingURL:    recapSrv.URL + "/meet/recap-e2e?p=synthetic-placeholder-passcode",
				ForceRecap:    true,
				AISummaryOnly: true,
				LobbyTimeout:  20 * time.Second,
			},
		)
		require.NoError(t, err)
		defer recapCleanup()

		recapCtx, recapCancel := context.WithTimeout(recapBctx, 30*time.Second)
		defer recapCancel()

		require.NoError(t, recapMeeting.Join(recapCtx))

		sp, ok := recapMeeting.(AISummaryProvider)
		require.True(t, ok)
		assert.Equal(t, "Meeting notes:\nFollow-up actions confirmed.", sp.AISummary())
	}()
}
