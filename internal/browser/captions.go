package browser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/softwaresalt/go-teams/internal/transcript"
)

// captionBinding is the name of the CDP runtime binding the injected observer
// calls to forward caption fragments to Go.
const captionBinding = "sendCaptionToGo"

// captionQueueSize bounds the buffer between chromedp's event goroutine and the
// caption worker, decoupling a slow sink from CDP event processing.
const captionQueueSize = 256

// Teams live-caption DOM selectors.
//
// Confirmed against live Microsoft Teams web DOM (2026-09-05), supporting both
// the guest lightweight client (direct #closed-captions-button in the More
// menu) and the authenticated full v2 client (#LanguageSpeechMenuControl-id ->
// #closed-captions-button) while preserving legacy data-tid selectors.
const (
	moreActionsSel           = `#callingButtons-showMoreBtn, [data-tid="more-button"]`
	languageSpeechSel        = `#LanguageSpeechMenuControl-id, [data-tid="language-speech-button"]`
	turnOnCaptionsSel        = `#closed-captions-button, [data-tid="closed-captions-button"], [data-tid="closed-captions-button-on"]`
	spokenLanguageConfirmSel = `[data-tid="spoken-language-confirm-button"], [data-tid="confirm-spoken-language"]`
	captionWrapperSel        = `[data-tid="closed-caption-renderer-wrapper"], [data-tid="closed-captions-renderer"]`
)

// captionObserverJS instantiates a MutationObserver on the Teams caption
// renderer. On attachment and on each mutation it forwards caption blocks whose
// text ends with terminal punctuation immediately, and arms a per-element
// settle timer (1500ms) so utterances that finish without trailing punctuation
// are still delivered as settled finals via sendCaptionToGo. Authoritative
// deduplication is performed on the Go side. The IIFE returns true when the
// observer was attached (the caption wrapper was present).
const captionObserverJS = `(() => {
  const initialWrapper = document.querySelector('[data-tid="closed-caption-renderer-wrapper"], [data-tid="closed-captions-renderer"]');
  if (!initialWrapper) { return false; }
  const timers = new WeakMap();
  const pendingText = new WeakMap();
  const lastSent = new WeakMap();
  let lastInterimSentAt = 0;
  const findSpeaker = (el, root) => {
    let cur = el.closest('.fui-ChatMessageCompact') || el.parentElement;
    while (cur && cur !== root && cur !== document.body) {
      const authorEl = cur.querySelector('[data-tid="author"], [data-tid="closed-caption-speaker-name"], .fui-ChatMessageCompact__author');
      if (authorEl && (authorEl.textContent || '').trim()) {
        return (authorEl.textContent || '').trim();
      }
      cur = cur.parentElement;
    }
    return '';
  };
  const inspectBlock = (el, root) => {
    const text = (el.textContent || '').trim();
    if (!text) { return; }
    const speaker = findSpeaker(el, root);
    const stateKey = speaker + '\x00' + text;
    if (lastSent.get(el) === stateKey) { return; }
    if (/[.!?]$/.test(text)) {
      const prevTimer = timers.get(el);
      if (prevTimer) { clearTimeout(prevTimer); timers.delete(el); }
      pendingText.delete(el);
      lastSent.set(el, stateKey);
      window.sendCaptionToGo(JSON.stringify({ speaker: speaker, text: text }));
      return;
    }
    if (pendingText.get(el) === stateKey && timers.has(el)) { return; }
    const now = Date.now();
    if (now - lastInterimSentAt >= 1000) {
      lastInterimSentAt = now;
      window.sendCaptionToGo(JSON.stringify({ speaker: speaker, text: text, interim: true }));
    }
    const prevTimer = timers.get(el);
    if (prevTimer) { clearTimeout(prevTimer); }
    pendingText.set(el, stateKey);
    const timer = setTimeout(() => {
      timers.delete(el);
      pendingText.delete(el);
      const settledText = (el.textContent || '').trim();
      if (!settledText) { return; }
      const settledSpeaker = findSpeaker(el, root) || speaker;
      const settledKey = settledSpeaker + '\x00' + settledText;
      if (lastSent.get(el) === settledKey) { return; }
      lastSent.set(el, settledKey);
      window.sendCaptionToGo(JSON.stringify({ speaker: settledSpeaker, text: settledText, final: true }));
    }, 1500);
    timers.set(el, timer);
  };
  const scan = () => {
    const wrapper = document.querySelector('[data-tid="closed-caption-renderer-wrapper"], [data-tid="closed-captions-renderer"]') || initialWrapper;
    const blocks = wrapper.querySelectorAll('[data-tid="closed-caption-text"], .fui-ChatMessageCompact__body');
    blocks.forEach((el) => inspectBlock(el, wrapper));
  };
  const observer = new MutationObserver(scan);
  observer.observe(document.body || initialWrapper, { childList: true, subtree: true, characterData: true });
  scan();
  return true;
})()`

// captionPayload is the JSON structure emitted by the in-browser observer.
type captionPayload struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
	Final   bool   `json:"final,omitempty"`
	Interim bool   `json:"interim,omitempty"`
}

// handleCaptionPayloadWithActivity parses a raw binding payload, invokes
// onActivity when non-empty caption text arrives, and passes non-interim
// fragments through Go-side rolling deduplication to sink.
func handleCaptionPayloadWithActivity(raw string, dedup *transcript.Deduplicator, sink CaptionSink, onActivity func()) error {
	var p captionPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("browser: parse caption payload: %w", err)
	}
	if strings.TrimSpace(p.Text) != "" && onActivity != nil {
		onActivity()
	}
	if p.Interim {
		return nil
	}
	var (
		text string
		ok   bool
	)
	if p.Final {
		text, ok = dedup.IngestSettled(p.Speaker, p.Text)
	} else {
		text, ok = dedup.Ingest(p.Speaker, p.Text)
	}
	if ok {
		sink(p.Speaker, text)
	}
	return nil
}

// handleCaptionPayload parses a raw binding payload, applies Go-side rolling
// deduplication, and invokes sink for newly finalized utterances. It returns an
// error only when the payload cannot be parsed; suppressed duplicates and
// non-final fragments are not errors.
func handleCaptionPayload(raw string, dedup *transcript.Deduplicator, sink CaptionSink) error {
	return handleCaptionPayloadWithActivity(raw, dedup, sink, nil)
}
