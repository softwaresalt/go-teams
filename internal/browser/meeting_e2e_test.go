package browser

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strconv"
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

// TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd verifies that
// headless Chromium captures and MP3-encodes audio from a download-blocked
// SharePoint Meeting Recap (g_fileInfo.isDownloadBlocked = true) embedded in a
// streamembed.aspx iframe, using real browser WebCrypto AES-CBC decryption
// of oneDrive.transcode audio/mp4 (mp4a.40.2) fMP4 segments and Chromium's
// native AudioContext.decodeAudioData.
func TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd(t *testing.T) {
	LoadTestEnv(t)
	if runtime.GOOS != "windows" {
		t.Skip("live browser e2e test targets Windows consumer hosts only")
	}
	if ResolveExecPath("") == "" {
		t.Skip("no Chromium-based browser executable found on host")
	}

	aesKey := []byte("0123456789abcdef")
	aesIV := []byte("fedcba9876543210")
	initSeg := buildSilentAACAudioInitFMP4()
	const framesPerSeg = 44
	segPlain := make([][]byte, 3)
	segEnc := make([][]byte, 3)
	for k := range segPlain {
		segPlain[k] = buildSilentAACMediaSegmentFMP4(uint32(k+1), uint64(k*framesPerSeg*1024), framesPerSeg)
		segEnc[k] = encryptAESCBC(t, aesKey, aesIV, segPlain[k])
	}
	initEnc := encryptAESCBC(t, aesKey, aesIV, initSeg)

	keyB64 := base64.StdEncoding.EncodeToString(aesKey)
	ivB64 := base64.StdEncoding.EncodeToString(aesIV)

	var audioSegRequests [3]atomic.Int32
	var videoSegRequests atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/l/meetingrecap":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!doctype html>
<html>
<head><title>E2E StreamEmbed Recap | Microsoft Teams</title></head>
<body>
  <div data-tid="intelligent-recap-header">Recap</div>
  <div class="ms-List">
    <div class="ms-List-cell" data-transcript-entry-id="e1">
      <span data-transcript-copy-field="speaker">Alice</span>
      <span data-transcript-copy-field="timestamp">0:05</span>
      <span class="entryText-1">Hello from download-blocked streamembed.</span>
    </div>
  </div>
  <iframe id="streamFrame" src="/_layouts/15/streamembed.aspx?driveItemId=recording-e2e"></iframe>
  <script>
    window.g_fileInfo = {
      VroomItemId: 'recording-e2e',
      title: 'E2E StreamEmbed Recap',
      name: 'E2E.mp4'
    };
  </script>
</body>
</html>`))
		case "/_layouts/15/streamembed.aspx":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(w, `<!doctype html>
<html>
<head><title>StreamEmbed</title></head>
<body>
  <video id="player"></video>
  <button aria-label="Play video" id="playBtn">Play video</button>
  <script>
    window.g_fileInfo = {
      VroomItemId: 'recording-e2e',
      isDownloadBlocked: true,
      title: 'E2E StreamEmbed Recap',
      name: 'E2E.mp4'
    };
    function b64ToU8(b64) {
      const bin = atob(b64);
      const out = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
      return out;
    }
    const keyBytes = b64ToU8(%q);
    const ivBytes = b64ToU8(%q);
    document.getElementById('playBtn').addEventListener('click', async () => {
      try {
        const key = await window.crypto.subtle.importKey('raw', keyBytes, { name: 'AES-CBC' }, false, ['decrypt']);
        const base = location.origin + '/_api/v2.1/drives/d1/items/recording-e2e/oneDrive.transcode';
        const initResp = await fetch(base + '?part=initsegment&track=audio&quality=audcopy&wsd=50000000&ppd=150000000&driveItemId=recording-e2e');
        const initBuf = await initResp.arrayBuffer();
        await window.crypto.subtle.decrypt({ name: 'AES-CBC', iv: ivBytes }, key, initBuf);
        const seg0Resp = await fetch(base + '?part=mediasegment&track=audio&quality=audcopy&segmentTime=0&wsd=50000000&ppd=150000000&driveItemId=recording-e2e');
        const seg0Buf = await seg0Resp.arrayBuffer();
        await window.crypto.subtle.decrypt({ name: 'AES-CBC', iv: ivBytes }, key, seg0Buf);
        setTimeout(() => {
          fetch(base + '?part=mediasegment&track=video&quality=high&segmentTime=50000000&wsd=50000000&ppd=150000000&driveItemId=recording-e2e').catch(() => {});
        }, 20);
      } catch (_) {}
    });
  </script>
</body>
</html>`, keyB64, ivB64)
		case "/_api/v2.1/drives/d1/items/recording-e2e/oneDrive.transcode":
			q := r.URL.Query()
			if q.Get("track") == "video" {
				videoSegRequests.Add(1)
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "video/mp4")
			if q.Get("part") == "initsegment" {
				_, _ = w.Write(initEnc)
				return
			}
			if q.Get("part") == "mediasegment" {
				segTime, err := strconv.Atoi(q.Get("segmentTime"))
				if err == nil {
					idx := segTime / 50000000
					if idx >= 0 && idx < len(segEnc) {
						audioSegRequests[idx].Add(1)
						_, _ = w.Write(segEnc[idx])
						return
					}
				}
			}
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
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
		MeetingOptions{
			MeetingURL:       srv.URL + "/l/meetingrecap?driveItemId=recording-e2e",
			ForceRecap:       true,
			DisableAISummary: true,
			LobbyTimeout:     20 * time.Second,
		},
	)
	require.NoError(t, err)
	defer cleanup()

	ctx, cancel := context.WithTimeout(bctx, 30*time.Second)
	defer cancel()

	require.NoError(t, m.Join(ctx))

	rm, ok := m.(RecapMeeting)
	require.True(t, ok)
	require.True(t, rm.IsRecap(), "expected Join to extract .ms-List recap transcript")

	var transcriptLines [][2]string
	require.NoError(t, m.EnableCaptions(ctx, func(speaker, text string) {
		transcriptLines = append(transcriptLines, [2]string{speaker, text})
	}))
	require.Equal(t, [][2]string{{"Alice\x0000:00:05", "Hello from download-blocked streamembed."}}, transcriptLines)

	var buf bytes.Buffer
	require.NoError(t, m.CaptureAudio(ctx, &buf))

	b := buf.Bytes()
	require.NotEmpty(t, b, "expected non-empty MP3 stream")
	require.True(t, len(b) >= 2 && b[0] == 0xff && (b[1]&0xe0) == 0xe0, "expected valid MPEG MP3 frame sync header")
	for i := range audioSegRequests {
		assert.GreaterOrEqual(t, audioSegRequests[i].Load(), int32(1), "expected audio segment %d to be fetched", i)
	}
	assert.Zero(t, videoSegRequests.Load(), "expected track=video transcode requests to be stripped")
}

func encryptAESCBC(t *testing.T, key, iv, plaintext []byte) []byte {
	t.Helper()
	block, err := aes.NewCipher(key)
	require.NoError(t, err)
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)
	return ciphertext
}

func makeISOBMFFBox(boxType string, payloads ...[]byte) []byte {
	total := 8
	for _, p := range payloads {
		total += len(p)
	}
	var out bytes.Buffer
	out.Grow(total)
	_ = binary.Write(&out, binary.BigEndian, uint32(total))
	out.WriteString(boxType)
	for _, p := range payloads {
		out.Write(p)
	}
	return out.Bytes()
}

func makeFullBox(boxType string, version byte, flags uint32, payloads ...[]byte) []byte {
	hdr := [4]byte{version, byte(flags >> 16), byte(flags >> 8), byte(flags)}
	all := make([][]byte, 0, len(payloads)+1)
	all = append(all, hdr[:])
	all = append(all, payloads...)
	return makeISOBMFFBox(boxType, all...)
}

func buildSilentAACAudioInitFMP4() []byte {
	ftyp := makeISOBMFFBox("ftyp",
		[]byte("isom"),
		[]byte{0x00, 0x00, 0x02, 0x00},
		[]byte("isom"), []byte("iso5"), []byte("mp41"),
	)

	var mvhdBody bytes.Buffer
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(0))     // creation_time
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(0))     // modification_time
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(44100)) // timescale
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(0))     // duration
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(0x00010000))
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint16(0x0100))
	mvhdBody.Write(make([]byte, 10))
	for _, v := range [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000} {
		_ = binary.Write(&mvhdBody, binary.BigEndian, v)
	}
	mvhdBody.Write(make([]byte, 24))
	_ = binary.Write(&mvhdBody, binary.BigEndian, uint32(2)) // next_track_ID
	mvhd := makeFullBox("mvhd", 0, 0, mvhdBody.Bytes())

	var tkhdBody bytes.Buffer
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(1)) // track_ID
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	tkhdBody.Write(make([]byte, 8))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint16(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint16(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint16(0x0100))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint16(0))
	for _, v := range [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000} {
		_ = binary.Write(&tkhdBody, binary.BigEndian, v)
	}
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&tkhdBody, binary.BigEndian, uint32(0))
	tkhd := makeFullBox("tkhd", 0, 7, tkhdBody.Bytes())

	var mdhdBody bytes.Buffer
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint32(44100))
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint32(0))
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint16(0x55c4)) // und
	_ = binary.Write(&mdhdBody, binary.BigEndian, uint16(0))
	mdhd := makeFullBox("mdhd", 0, 0, mdhdBody.Bytes())

	var hdlrBody bytes.Buffer
	_ = binary.Write(&hdlrBody, binary.BigEndian, uint32(0))
	hdlrBody.WriteString("soun")
	hdlrBody.Write(make([]byte, 12))
	hdlrBody.WriteString("SoundHandler\x00")
	hdlr := makeFullBox("hdlr", 0, 0, hdlrBody.Bytes())

	smhd := makeFullBox("smhd", 0, 0, make([]byte, 4))
	urlBox := makeFullBox("url ", 0, 1)
	dref := makeFullBox("dref", 0, 0, []byte{0, 0, 0, 1}, urlBox)
	dinf := makeISOBMFFBox("dinf", dref)

	esds := makeFullBox("esds", 0, 0, []byte{
		0x03, 25, 0x00, 0x01, 0x00,
		0x04, 17, 0x40, 0x15, 0x00, 0x00, 0x00,
		0x00, 0x00, 0xfa, 0x00,
		0x00, 0x00, 0xfa, 0x00,
		0x05, 2, 0x12, 0x08, // AAC-LC 44.1kHz mono
		0x06, 1, 0x02,
	})
	var mp4aBody bytes.Buffer
	mp4aBody.Write(make([]byte, 6))
	_ = binary.Write(&mp4aBody, binary.BigEndian, uint16(1)) // data_reference_index
	mp4aBody.Write(make([]byte, 8))
	_ = binary.Write(&mp4aBody, binary.BigEndian, uint16(1))  // channelcount = 1
	_ = binary.Write(&mp4aBody, binary.BigEndian, uint16(16)) // samplesize = 16
	_ = binary.Write(&mp4aBody, binary.BigEndian, uint32(0))  // pre_defined + reserved
	_ = binary.Write(&mp4aBody, binary.BigEndian, uint32(44100<<16))
	mp4a := makeISOBMFFBox("mp4a", mp4aBody.Bytes(), esds)

	stsd := makeFullBox("stsd", 0, 0, []byte{0, 0, 0, 1}, mp4a)
	stts := makeFullBox("stts", 0, 0, make([]byte, 4))
	stsc := makeFullBox("stsc", 0, 0, make([]byte, 4))
	stsz := makeFullBox("stsz", 0, 0, make([]byte, 8))
	stco := makeFullBox("stco", 0, 0, make([]byte, 4))
	stbl := makeISOBMFFBox("stbl", stsd, stts, stsc, stsz, stco)
	minf := makeISOBMFFBox("minf", smhd, dinf, stbl)
	mdia := makeISOBMFFBox("mdia", mdhd, hdlr, minf)
	trak := makeISOBMFFBox("trak", tkhd, mdia)

	var trexBody bytes.Buffer
	_ = binary.Write(&trexBody, binary.BigEndian, uint32(1))          // track_ID
	_ = binary.Write(&trexBody, binary.BigEndian, uint32(1))          // default_sample_description_index
	_ = binary.Write(&trexBody, binary.BigEndian, uint32(1024))       // default_sample_duration
	_ = binary.Write(&trexBody, binary.BigEndian, uint32(4))          // default_sample_size
	_ = binary.Write(&trexBody, binary.BigEndian, uint32(0x02000000)) // default_sample_flags
	trex := makeFullBox("trex", 0, 0, trexBody.Bytes())
	mvex := makeISOBMFFBox("mvex", trex)

	moov := makeISOBMFFBox("moov", mvhd, trak, mvex)
	return append(ftyp, moov...)
}

func buildSilentAACMediaSegmentFMP4(seqNum uint32, baseMediaDecodeTime uint64, frameCount int) []byte {
	styp := makeISOBMFFBox("styp",
		[]byte("msdh"),
		[]byte{0, 0, 0, 0},
		[]byte("msdh"), []byte("msix"),
	)

	var seqBuf [4]byte
	binary.BigEndian.PutUint32(seqBuf[:], seqNum)
	mfhd := makeFullBox("mfhd", 0, 0, seqBuf[:])

	var trackBuf [4]byte
	binary.BigEndian.PutUint32(trackBuf[:], 1)
	tfhd := makeFullBox("tfhd", 0, 0x020000, trackBuf[:])

	var tfdtBuf [8]byte
	binary.BigEndian.PutUint64(tfdtBuf[:], baseMediaDecodeTime)
	tfdt := makeFullBox("tfdt", 1, 0, tfdtBuf[:])

	buildTrun := func(dataOffset int32) []byte {
		var trunBody bytes.Buffer
		_ = binary.Write(&trunBody, binary.BigEndian, uint32(frameCount))
		_ = binary.Write(&trunBody, binary.BigEndian, dataOffset)
		for range frameCount {
			_ = binary.Write(&trunBody, binary.BigEndian, uint32(1024))
			_ = binary.Write(&trunBody, binary.BigEndian, uint32(4))
		}
		return makeFullBox("trun", 0, 0x000301, trunBody.Bytes())
	}

	placeholderMoof := makeISOBMFFBox("moof", mfhd, makeISOBMFFBox("traf", tfhd, tfdt, buildTrun(0)))
	moof := makeISOBMFFBox("moof", mfhd, makeISOBMFFBox("traf", tfhd, tfdt, buildTrun(int32(len(placeholderMoof)+8))))

	// Minimal valid AAC-LC 44.1 kHz mono silent raw_data_block (ID_SCE + ID_END).
	silentFrame := []byte{0x00, 0x00, 0x00, 0x07}
	mdatPayload := bytes.Repeat(silentFrame, frameCount)
	mdat := makeISOBMFFBox("mdat", mdatPayload)

	out := make([]byte, 0, len(styp)+len(moof)+len(mdat))
	out = append(out, styp...)
	out = append(out, moof...)
	out = append(out, mdat...)
	return out
}
