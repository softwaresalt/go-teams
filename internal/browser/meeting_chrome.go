package browser

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"github.com/softwaresalt/go-teams/internal/audio"
	"github.com/softwaresalt/go-teams/internal/teams"
	"github.com/softwaresalt/go-teams/internal/transcript"
)

// admissionPollInterval controls how often awaitAdmission re-checks the DOM.
const admissionPollInterval = 2 * time.Second

// chromeMeeting is the chromedp-backed Meeting implementation. Its methods
// operate on a chromedp browser context supplied by the caller so that a single
// browser session spans join, captions, and audio.
type chromeMeeting struct {
	opts MeetingOptions
}

func newChromeMeeting(opts MeetingOptions) *chromeMeeting {
	return &chromeMeeting{opts: opts}
}

// Join implements Meeting. ctx MUST be a chromedp browser context. It validates
// the meeting URL, navigates to it, runs the pre-join sequence (enforce mute,
// inject the bot name, click "Join now"), then polls until admitted or ctx is
// cancelled. Navigation failures are reported separately from pre-join
// readiness failures.
func (m *chromeMeeting) Join(ctx context.Context) error {
	if chromedp.FromContext(ctx) == nil {
		return fmt.Errorf("browser: join requires a chromedp browser context; create one with chromedp.NewContext")
	}
	if err := teams.ValidateMeetingURL(m.opts.MeetingURL); err != nil {
		return fmt.Errorf("browser: invalid meeting url: %w", err)
	}
	if err := chromedp.Run(ctx, chromedp.Navigate(m.opts.MeetingURL)); err != nil {
		return fmt.Errorf("browser: navigate to meeting: %w", err)
	}
	if err := chromedp.Run(ctx, chromedp.WaitVisible(joinNowSel, chromedp.ByQuery)); err != nil {
		return fmt.Errorf("browser: pre-join not visible: %w", err)
	}
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
	var ariaChecked string
	var ok bool
	if err := chromedp.Run(ctx,
		chromedp.AttributeValue(micToggleSel, "aria-checked", &ariaChecked, &ok, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: read mic toggle state: %w", err)
	}
	if ok && muteRequired(ariaChecked) {
		if err := chromedp.Run(ctx, chromedp.Click(micToggleSel, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("browser: mute microphone: %w", err)
		}
	}
	return nil
}

// injectName types the configured bot name into the display-name input when it
// is present (unauthenticated guest flow).
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

// EnableCaptions implements Meeting. It activates Teams live captions, registers
// the sendCaptionToGo CDP binding, injects the caption MutationObserver, and
// routes forwarded fragments through Go-side deduplication to sink. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) EnableCaptions(ctx context.Context, sink CaptionSink) error {
	if sink == nil {
		return fmt.Errorf("browser: caption sink must not be nil")
	}
	if chromedp.FromContext(ctx) == nil {
		return fmt.Errorf("browser: enable captions requires a chromedp browser context")
	}
	if err := m.activateCaptions(ctx); err != nil {
		return err
	}
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(captionBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add caption binding: %w", err)
	}

	// Decouple the caller's sink from chromedp's single event goroutine via a
	// payloadPump: the listener only enqueues, and a worker performs dedup and
	// sink delivery. pump.stop() is a producer barrier that drains all buffered
	// fragments before EnableCaptions returns, so the transcript tail is not
	// lost on shutdown and no sink call happens after return.
	workerCtx, cancel := context.WithCancel(ctx)
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
	if err := chromedp.Run(ctx, chromedp.Evaluate(captionObserverJS, &attached)); err != nil {
		return fmt.Errorf("browser: inject caption observer: %w", err)
	}
	if !attached {
		slog.Warn("browser: caption renderer not yet present; observer not attached")
	}

	<-ctx.Done()
	return nil
}

// activateCaptions clicks through the meeting menu to turn on live captions:
// "More actions" -> "Language and speech" -> "Turn on live captions".
func (m *chromeMeeting) activateCaptions(ctx context.Context) error {
	if err := chromedp.Run(ctx,
		chromedp.Click(moreActionsSel, chromedp.ByQuery),
		chromedp.Click(languageSpeechSel, chromedp.ByQuery),
		chromedp.Click(turnOnCaptionsSel, chromedp.ByQuery),
		chromedp.WaitVisible(captionWrapperSel, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("browser: activate captions: %w", err)
	}
	return nil
}

// CaptureAudio implements Meeting. It registers the sendAudioToGo CDP binding,
// starts in-browser getDisplayMedia PCM capture, and encodes forwarded chunks to
// MP3 on w via a worker goroutine decoupled from the CDP event loop. It blocks
// until ctx is cancelled.
func (m *chromeMeeting) CaptureAudio(ctx context.Context, w io.Writer) (err error) {
	if w == nil {
		return fmt.Errorf("browser: audio writer must not be nil")
	}
	if chromedp.FromContext(ctx) == nil {
		return fmt.Errorf("browser: capture audio requires a chromedp browser context")
	}
	if err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return cdpruntime.AddBinding(audioBinding).Do(ctx)
	})); err != nil {
		return fmt.Errorf("browser: add audio binding: %w", err)
	}

	// A payloadPump decouples the CDP event loop from MP3 encoding and provides a
	// producer barrier on stop, so buffered audio chunks are drained (not lost)
	// on shutdown. workerErr is written only by the pump's worker goroutine and
	// is read after pump.stop() joins it, so the access is race-free.
	workerCtx, cancel := context.WithCancel(ctx)
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
	if err := chromedp.Run(ctx, chromedp.Evaluate(audioCaptureJS, &started, awaitPromise)); err != nil {
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
