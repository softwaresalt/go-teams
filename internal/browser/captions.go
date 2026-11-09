package browser

import (
	"encoding/json"
	"fmt"

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
  const lastSent = new WeakMap();
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
    if (!text || lastSent.get(el) === text) { return; }
    const speaker = findSpeaker(el, root);
    const prevTimer = timers.get(el);
    if (prevTimer) { clearTimeout(prevTimer); }
    if (/[.!?]$/.test(text)) {
      lastSent.set(el, text);
      window.sendCaptionToGo(JSON.stringify({ speaker: speaker, text: text }));
      return;
    }
    const timer = setTimeout(() => {
      const settledText = (el.textContent || '').trim();
      if (!settledText || lastSent.get(el) === settledText) { return; }
      const settledSpeaker = findSpeaker(el, root) || speaker;
      lastSent.set(el, settledText);
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
}

// handleCaptionPayload parses a raw binding payload, applies Go-side rolling
// deduplication, and invokes sink for newly finalized utterances. It returns an
// error only when the payload cannot be parsed; suppressed duplicates and
// non-final fragments are not errors.
func handleCaptionPayload(raw string, dedup *transcript.Deduplicator, sink CaptionSink) error {
	var p captionPayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return fmt.Errorf("browser: parse caption payload: %w", err)
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
