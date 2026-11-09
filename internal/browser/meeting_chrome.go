package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/cdproto/target"
	"github.com/chromedp/chromedp"

	"github.com/softwaresalt/go-teams/internal/audio"
	"github.com/softwaresalt/go-teams/internal/teams"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

// admissionPollInterval controls how often pre-join and admission loops
// re-check the DOM.
const (
	admissionPollInterval = 1 * time.Second
	captionMenuStepWait   = 5 * time.Second
	captionMenuPoll       = 150 * time.Millisecond
)

// chromeMeeting is the chromedp-backed Meeting implementation. Its methods
// operate on a chromedp browser context supplied by the caller so that a single
// browser session spans join, captions, and audio.
type chromeMeeting struct {
	opts         MeetingOptions
	headedOpts   *Options
	cdpPort      int
	headedMu     sync.Mutex
	headedStop   func()
	attachedCtx  context.Context
	recapEntries []RecapTranscriptEntry
	aiSummary    string
}

func newChromeMeeting(opts MeetingOptions) *chromeMeeting {
	return &chromeMeeting{opts: opts}
}

// AISummary implements AISummaryProvider.
func (m *chromeMeeting) AISummary() string {
	return m.aiSummary
}

func (m *chromeMeeting) stopHeadedBrowser() {
	m.headedMu.Lock()
	stop := m.headedStop
	m.headedStop = nil
	m.headedMu.Unlock()
	if stop != nil {
		stop()
	}
}

// Join implements Meeting. It validates the meeting URL, navigates to it, waits
// for user authentication or launcher interstitial handling as needed, and
// detects either a Teams Meeting Recap transcript (via xplatplugins.aspx /
// .ms-List) or a live call pre-join / active call toolbar.
func (m *chromeMeeting) Join(ctx context.Context) error {
	if m.headedOpts == nil && chromedp.FromContext(ctx) == nil {
		return fmt.Errorf("browser: join requires a chromedp browser context; create one with chromedp.NewContext")
	}
	if err := teams.ValidateMeetingURL(m.opts.MeetingURL); err != nil {
		return fmt.Errorf("browser: invalid meeting url: %w", err)
	}
	if m.headedOpts != nil {
		return m.joinHeaded(ctx)
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(m.opts.MeetingURL)); err != nil {
		return fmt.Errorf("browser: navigate to meeting: %w", err)
	}
	return m.awaitPreJoinOrAdmission(ctx)
}

func (m *chromeMeeting) joinHeaded(ctx context.Context) error {
	port, wsURL, cleanupProc, err := launchCleanHeadedBrowser(*m.headedOpts, m.opts.MeetingURL)
	if err != nil {
		return err
	}
	allocCtx, cancelAlloc := chromedp.NewRemoteAllocator(ctx, wsURL)
	m.headedMu.Lock()
	m.headedStop = func() {
		cancelAlloc()
		cleanupProc()
	}
	m.headedMu.Unlock()

	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	loggedWaiting := false
	clickedLauncher := false
	reNavigatedRecap := false

	for {
		targets, err := listCDPTargets(port)
		if err == nil {
			var mainPage *cdpTargetInfo
			for i := range targets {
				t := &targets[i]
				if (t.Type != "page" && t.Type != "iframe") || t.WebSocketDebuggerURL == "" {
					continue
				}
				if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
					continue
				}
				if t.Type == "page" && mainPage == nil {
					mainPage = t
				}

				// Check if this target (e.g. xplatplugins.aspx iframe or recap page)
				// has a Meeting Recap transcript list populated.
				if raw, evalErr := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, extractRecapTranscriptJS); evalErr == nil && raw != "" {
					if entries, pErr := parseRecapEntriesPayload(raw); pErr == nil && len(entries) > 0 {
						m.recapEntries = entries
						slog.Info("browser: extracted meeting recap transcript", "entries", len(entries))
						if summary := captureAISummaryFromTargets(ctx, port); summary != "" {
							m.aiSummary = summary
							slog.Info("browser: copied AI summary from meeting recap")
						}
						return nil
					}
				}
			}

			if mainPage != nil {
				stateRaw, evalErr := evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, `JSON.stringify({
					host: location.host,
					path: location.pathname,
					toolbar: document.querySelector('#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]') !== null,
					lobby: document.body !== null && document.body.innerText.includes("Someone will let you in shortly"),
					prejoin: document.querySelector('[data-tid="prejoin-join-button"]') !== null,
					launcher: document.querySelector('#openTeamsClientInBrowser, button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]') !== null,
					recapHeader: document.querySelector('[data-tid="intelligent-recap-header"]') !== null,
					tidCount: document.querySelectorAll('[data-tid]').length
				})`)
				if evalErr == nil {
					var st struct {
						Host        string `json:"host"`
						Path        string `json:"path"`
						Toolbar     bool   `json:"toolbar"`
						Lobby       bool   `json:"lobby"`
						Prejoin     bool   `json:"prejoin"`
						Launcher    bool   `json:"launcher"`
						RecapHeader bool   `json:"recapHeader"`
						TidCount    int    `json:"tidCount"`
					}
					_ = json.Unmarshal([]byte(stateRaw), &st)

					if st.Launcher {
						if !clickedLauncher {
							slog.Info("browser: launcher interstitial detected; clicking use web app")
							_, _ = evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, `(() => {
								const btn = document.querySelector('#openTeamsClientInBrowser, button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]');
								if (btn) { btn.click(); return true; }
								return false;
							})()`)
							clickedLauncher = true
						}
					} else {
						clickedLauncher = false
						if strings.Contains(m.opts.MeetingURL, "meetingrecap") &&
							strings.Contains(st.Host, "teams.cloud.microsoft") &&
							!st.RecapHeader &&
							st.TidCount > 20 &&
							!reNavigatedRecap {
							// Wait for Teams to finish routing the launcher deep link; only
							// re-navigate if the recap container still has not appeared.
							reNavigatedRecap = true
						} else if st.Toolbar || st.Lobby || st.Prejoin {
							// Live meeting flow on the open page target.
							tCtx, _ := chromedp.NewContext(allocCtx, chromedp.WithTargetID(target.ID(mainPage.ID)))
							m.attachedCtx = tCtx
							return m.awaitPreJoinOrAdmission(tCtx)
						} else if !loggedWaiting {
							slog.Info("browser: waiting for user authentication and meeting/recap load in browser window")
							loggedWaiting = true
						}
					}
				}
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: pre-join not visible: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *chromeMeeting) awaitPreJoinOrAdmission(ctx context.Context) error {
	ticker := time.NewTicker(admissionPollInterval)
	defer ticker.Stop()
	loggedWaiting := false
	clickedLauncher := false

	for {
		if m.cdpPort > 0 {
			if ok := m.tryExtractRecapViaCDP(ctx, m.cdpPort); ok {
				return nil
			}
		}

		var toolbar, lobby, prejoin, launcher bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(existsJS(meetingToolbarSel), &toolbar),
			chromedp.Evaluate(containsTextJS(lobbyText), &lobby),
			chromedp.Evaluate(existsJS(joinNowSel), &prejoin),
			chromedp.Evaluate(existsJS(joinOnWebSel), &launcher),
		); err != nil {
			return fmt.Errorf("browser: pre-join not visible: %w", err)
		}

		switch classifyPreJoinState(toolbar, lobby, prejoin, launcher) {
		case PreJoinAdmitted:
			slog.Info("browser: admitted to meeting")
			return nil
		case PreJoinInLobby:
			return m.awaitAdmission(ctx)
		case PreJoinReady:
			if err := m.enforceMute(ctx); err != nil {
				return err
			}
			if err := m.injectName(ctx); err != nil {
				return err
			}
			if err := chromedp.Run(ctx, chromedp.Click(joinNowSel, chromedp.ByQuery)); err != nil {
				return fmt.Errorf("browser: click join now: %w", err)
			}
			return m.awaitAdmission(ctx)
		case PreJoinLauncher:
			if !clickedLauncher {
				slog.Info("browser: launcher interstitial detected; clicking join on web")
				var clicked bool
				if err := chromedp.Run(ctx, chromedp.Evaluate(clickJS(joinOnWebSel), &clicked)); err != nil {
					slog.Warn("browser: click join on web failed; retrying", "err", err)
				} else if clicked {
					clickedLauncher = true
				}
			}
		case PreJoinWaitingForUser:
			clickedLauncher = false
			if !loggedWaiting {
				slog.Info("browser: waiting for user authentication and meeting entry in browser window")
				loggedWaiting = true
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: pre-join not visible: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func (m *chromeMeeting) tryExtractRecapViaCDP(ctx context.Context, port int) bool {
	targets, err := listCDPTargets(port)
	if err != nil {
		return false
	}
	for _, t := range targets {
		if (t.Type != "page" && t.Type != "iframe") || t.WebSocketDebuggerURL == "" {
			continue
		}
		if strings.HasPrefix(t.URL, "edge://") || strings.HasPrefix(t.URL, "chrome-extension://") {
			continue
		}
		if raw, evalErr := evalInCDPTarget(ctx, t.WebSocketDebuggerURL, extractRecapTranscriptJS); evalErr == nil && raw != "" {
			if entries, pErr := parseRecapEntriesPayload(raw); pErr == nil && len(entries) > 0 {
				m.recapEntries = entries
				slog.Info("browser: extracted meeting recap transcript", "entries", len(entries))
				if summary := captureAISummaryFromTargets(ctx, port); summary != "" {
					m.aiSummary = summary
					slog.Info("browser: copied AI summary from meeting recap")
				}
				return true
			}
		}
	}
	return false
}

// enforceMute clicks the microphone toggle if it is currently enabled so the
// bot joins muted. It probes for the toggle first (a single, non-blocking
// evaluation) because chromedp query actions retry until the context expires
// when a selector matches nothing.
func (m *chromeMeeting) enforceMute(ctx context.Context) error {
	var present bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(micToggleSel), &present)); err != nil {
		return fmt.Errorf("browser: probe mic toggle: %w", err)
	}
	if !present {
		// The mic toggle may be absent in some pre-join variants; treat as
		// already muted rather than failing the join.
		slog.Warn("browser: mic toggle not found; assuming muted")
		return nil
	}
	var checked bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(propBoolJS(micToggleSel, "checked"), &checked)); err != nil {
		return fmt.Errorf("browser: read mic checked property: %w", err)
	}
	var ariaChecked string
	var ok bool
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(micToggleSel, "aria-checked", &ariaChecked, &ok, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: read mic toggle state: %w", err)
	}
	if micStateRequiresMute(checked, ariaChecked) {
		if err := chromedp.Run(ctx, chromedp.Click(micToggleSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: mute microphone: %w", err)
		}
	}
	return nil
}

// injectName types the configured bot name into the display-name input when it
// is present and empty (unauthenticated guest flow).
func (m *chromeMeeting) injectName(ctx context.Context) error {
	if m.opts.BotName == "" {
		return nil
	}
	var present bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(displayNameInputSel), &present)); err != nil {
		return fmt.Errorf("browser: probe display-name input: %w", err)
	}
	if !present {
		return nil
	}
	var existing string
	if err := chromedp.Run(ctx, chromedp.Evaluate(propStringJS(displayNameInputSel, "value"), &existing)); err != nil {
		return fmt.Errorf("browser: read display-name input: %w", err)
	}
	if existing != "" {
		return nil
	}
	if err := chromedp.Run(ctx,
		chromedp.SendKeys(displayNameInputSel, m.opts.BotName, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: inject display name: %w", err)
	}
	return nil
}

// awaitAdmission polls the DOM for the meeting toolbar (admitted) or the lobby
// prompt (waiting) until admitted or ctx is cancelled.
func (m *chromeMeeting) awaitAdmission(ctx context.Context) error {
	ticker := time.NewTicker(admissionPollInterval)
	defer ticker.Stop()
	loggedLobby := false
	for {
		var toolbar, lobby bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(existsJS(meetingToolbarSel), &toolbar),
			chromedp.Evaluate(containsTextJS(lobbyText), &lobby),
		); err != nil {
			return fmt.Errorf("browser: poll admission: %w", err)
		}
		switch classifyAdmission(toolbar, lobby) {
		case AdmissionAdmitted:
			slog.Info("browser: admitted to meeting")
			return nil
		case AdmissionInLobby:
			if !loggedLobby {
				slog.Info("browser: waiting in lobby for admission")
				loggedLobby = true
			}
		case AdmissionUnknown:
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: await admission: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// payloadPump decouples a chromedp binding listener from a single processing
// worker goroutine. The listener enqueues raw payloads via offer; the worker
// applies process to each in arrival order. stop provides a full producer
// barrier: it rejects new payloads, waits for in-flight offers to finish, then
// drains every buffered payload to the worker before returning — so no payload
// is lost when the meeting is cancelled (e.g. on Ctrl+C).
type payloadPump struct {
	payloads chan string
	mu       sync.Mutex
	closing  bool
	prodWG   sync.WaitGroup
	workWG   sync.WaitGroup
}

// newPayloadPump starts a worker goroutine that applies process to each payload
// in arrival order until the pump is stopped.
func newPayloadPump(size int, process func(string)) *payloadPump {
	p := &payloadPump{payloads: make(chan string, size)}
	p.workWG.Add(1)
	go func() {
		defer p.workWG.Done()
		for raw := range p.payloads {
			process(raw)
		}
	}()
	return p
}

// offer enqueues raw for processing. It returns false when the pump is stopping
// or the queue is full, in which case the payload is dropped as backpressure.
func (p *payloadPump) offer(raw string) bool {
	p.mu.Lock()
	if p.closing {
		p.mu.Unlock()
		return false
	}
	p.prodWG.Add(1)
	p.mu.Unlock()
	defer p.prodWG.Done()
	select {
	case p.payloads <- raw:
		return true
	default:
		return false
	}
}

// stop performs the producer barrier: it rejects further offers, waits for
// in-flight offers to complete, closes the queue, and waits for the worker to
// drain the remaining payloads and exit.
func (p *payloadPump) stop() {
	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
	p.prodWG.Wait()
	close(p.payloads)
	p.workWG.Wait()
}

// targetContext returns a context that carries the active chromedp target
// (using m.attachedCtx when Join attached to a headed CDP target) while
// propagating cancellation from callerCtx so Session.Run's runCtx can cancel
// EnableCaptions and CaptureAudio on write/flush/worker errors without closing
// the underlying browser target.
func (m *chromeMeeting) targetContext(callerCtx context.Context) (context.Context, context.CancelFunc) {
	if m.attachedCtx == nil {
		return context.WithCancel(callerCtx)
	}
	targetCtx, cancel := context.WithCancel(m.attachedCtx)
	stop := context.AfterFunc(callerCtx, cancel)
	return targetCtx, func() {
		stop()
		cancel()
	}
}

// EnableCaptions implements Meeting. It activates Teams live captions, registers
// the sendCaptionToGo CDP binding, injects the caption MutationObserver, and
// routes forwarded fragments through Go-side deduplication to sink. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) EnableCaptions(ctx context.Context, sink CaptionSink) error {
	if sink == nil {
		return fmt.Errorf("browser: caption sink must not be nil")
	}
	if len(m.recapEntries) > 0 {
		for _, entry := range m.recapEntries {
			speaker := entry.Speaker
			if entry.Offset != "" {
				speaker = speaker + RecapOffsetSeparator + entry.Offset
			}
			sink(speaker, entry.Text)
		}
		return nil
	}
	targetCtx, cancelTarget := m.targetContext(ctx)
	defer cancelTarget()
	if chromedp.FromContext(targetCtx) == nil {
		return fmt.Errorf("browser: enable captions requires a chromedp browser context")
	}
	if err := m.activateCaptions(targetCtx); err != nil {
		return err
	}
	if err := chromedp.Run(targetCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(captionBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add caption binding: %w", err)
	}

	// Decouple the caller's sink from chromedp's single event goroutine via a
	// payloadPump: the listener only enqueues, and a worker performs dedup and
	// sink delivery. pump.stop() is a producer barrier that drains all buffered
	// fragments before EnableCaptions returns, so the transcript tail is not
	// lost on shutdown and no sink call happens after return.
	workerCtx, cancel := context.WithCancel(targetCtx)
	dedup := transcript.NewDeduplicator()
	pump := newPayloadPump(captionQueueSize, func(raw string) {
		if err := handleCaptionPayload(raw, dedup, sink); err != nil {
			slog.Warn("browser: discarding malformed caption payload", "err", err)
		}
	})
	defer func() {
		cancel()
		pump.stop()
	}()

	chromedp.ListenTarget(workerCtx, func(ev interface{}) {
		bc, ok := ev.(*cdpruntime.EventBindingCalled)
		if !ok || bc.Name != captionBinding {
			return
		}
		if workerCtx.Err() != nil {
			return
		}
		if !pump.offer(bc.Payload) {
			slog.Warn("browser: caption queue full; dropping fragment")
		}
	})

	var attached bool
	if err := chromedp.Run(targetCtx, chromedp.Evaluate(captionObserverJS, &attached)); err != nil {
		return fmt.Errorf("browser: inject caption observer: %w", err)
	}
	if !attached {
		slog.Warn("browser: caption renderer not yet present; observer not attached")
	}

	<-targetCtx.Done()
	return nil
}

// activateCaptions clicks through the meeting menu to turn on live captions:
// "More actions" -> (optional "Language and speech" submenu in full client) ->
// "Turn on live captions", then waits for the caption renderer wrapper to
// appear. If captions are already active in the DOM, it returns immediately.
func (m *chromeMeeting) activateCaptions(ctx context.Context) error {
	var alreadyActive bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		slog.Info("browser: live captions already active")
		return nil
	}

	if err := waitAnyPresent(ctx, captionWrapperSel, moreActionsSel); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		return nil
	}

	if err := chromedp.Run(ctx, chromedp.Click(moreActionsSel, chromedp.ByQuery)); err != nil {
		return fmt.Errorf("browser: activate captions (click more): %w", err)
	}

	menuCtx, cancelMenu := context.WithTimeout(ctx, captionMenuStepWait)
	err := waitAnyPresent(menuCtx, captionWrapperSel, turnOnCaptionsSel, languageSpeechSel)
	cancelMenu()
	if err != nil {
		return fmt.Errorf("browser: activate captions (menu): %w", err)
	}

	var captionsBtn, langSubmenu bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive),
		chromedp.Evaluate(existsJS(turnOnCaptionsSel), &captionsBtn),
		chromedp.Evaluate(existsJS(languageSpeechSel), &langSubmenu),
	); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if alreadyActive {
		return nil
	}

	if !captionsBtn && langSubmenu {
		if err := chromedp.Run(ctx, chromedp.Click(languageSpeechSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: activate captions (language submenu): %w", err)
		}
		subCtx, cancelSub := context.WithTimeout(ctx, captionMenuStepWait)
		err := waitAnyPresent(subCtx, captionWrapperSel, turnOnCaptionsSel)
		cancelSub()
		if err != nil {
			return fmt.Errorf("browser: activate captions (wait captions button): %w", err)
		}
	}

	if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(captionWrapperSel), &alreadyActive)); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	if !alreadyActive {
		if err := chromedp.Run(ctx, chromedp.Click(turnOnCaptionsSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: activate captions (click turn on captions): %w", err)
		}
	}

	return m.awaitCaptionWrapper(ctx)
}

func (m *chromeMeeting) awaitCaptionWrapper(ctx context.Context) error {
	ticker := time.NewTicker(captionMenuPoll)
	defer ticker.Stop()
	for {
		var wrapper, langConfirm bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(existsJS(captionWrapperSel), &wrapper),
			chromedp.Evaluate(existsJS(spokenLanguageConfirmSel), &langConfirm),
		); err != nil {
			return fmt.Errorf("browser: activate captions: %w", err)
		}
		if wrapper {
			slog.Info("browser: live captions active")
			return nil
		}
		if langConfirm {
			_ = chromedp.Run(ctx, chromedp.Click(spokenLanguageConfirmSel, chromedp.ByQuery))
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("browser: activate captions: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func waitAnyPresent(ctx context.Context, selectors ...string) error {
	ticker := time.NewTicker(captionMenuPoll)
	defer ticker.Stop()
	for {
		for _, sel := range selectors {
			var ok bool
			if err := chromedp.Run(ctx, chromedp.Evaluate(existsJS(sel), &ok)); err != nil {
				return err
			}
			if ok {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// CaptureAudio implements Meeting. It registers the sendAudioToGo CDP binding,
// starts in-browser getDisplayMedia PCM capture, and encodes forwarded chunks to
// MP3 on w via a worker goroutine decoupled from the CDP event loop. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) CaptureAudio(ctx context.Context, w io.Writer) (err error) {
	if w == nil {
		return fmt.Errorf("browser: audio writer must not be nil")
	}
	targetCtx, cancelTarget := m.targetContext(ctx)
	defer cancelTarget()
	if chromedp.FromContext(targetCtx) == nil {
		return fmt.Errorf("browser: capture audio requires a chromedp browser context")
	}
	if err := chromedp.Run(targetCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(audioBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add audio binding: %w", err)
	}

	// A payloadPump decouples the CDP event loop from MP3 encoding and provides a
	// producer barrier on stop, so buffered audio chunks are drained (not lost)
	// on shutdown. workerErr is written only by the pump's worker goroutine and
	// is read after pump.stop() joins it, so the access is race-free.
	workerCtx, cancel := context.WithCancel(targetCtx)
	var workerErr error
	enc := audio.NewEncoder(w, audio.SampleRate)
	pump := newPayloadPump(audioQueueSize, func(raw string) {
		if workerErr != nil {
			return // already failed fatally; skip remaining drained chunks
		}
		samples, decodeErr := decodeAudioPayload(raw)
		if decodeErr != nil {
			slog.Warn("browser: discarding malformed audio chunk", "err", decodeErr)
			return
		}
		if writeErr := enc.WritePCM(samples); writeErr != nil {
			// A writer/encoder failure is fatal: record it and stop capture so
			// CaptureAudio can surface it to the caller.
			workerErr = writeErr
			cancel()
		}
	})
	defer func() {
		cancel()
		pump.stop()
		// Emit any buffered sub-frame remainder now that the worker has stopped.
		if workerErr == nil {
			if flushErr := enc.Flush(); flushErr != nil {
				workerErr = flushErr
			}
		}
		if err == nil {
			err = workerErr
		}
	}()

	chromedp.ListenTarget(workerCtx, func(ev interface{}) {
		bc, ok := ev.(*cdpruntime.EventBindingCalled)
		if !ok || bc.Name != audioBinding {
			return
		}
		if workerCtx.Err() != nil {
			return
		}
		if !pump.offer(bc.Payload) {
			slog.Warn("browser: audio queue full; dropping chunk")
		}
	})

	var started bool
	awaitPromise := func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}
	if err := chromedp.Run(targetCtx, chromedp.Evaluate(audioCaptureJS, &started, awaitPromise)); err != nil {
		return fmt.Errorf("browser: start audio capture: %w", err)
	}
	if !started {
		return fmt.Errorf("browser: audio capture did not start (no audio track)")
	}

	// Block until the caller's ctx is cancelled or a fatal worker error cancels
	// workerCtx; the deferred barrier drains and reaps the worker.
	<-workerCtx.Done()
	return nil
}
