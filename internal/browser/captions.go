package browser

import (
	"encoding/json"
	"fmt"

	"github.com/softwaresalt/go-teams/internal/transcript"
)

// captionBinding is the name of the CDP runtime binding the injected observer
// calls to forward caption fragments to Go.
const captionBinding = "sendCaptionToGo"

// Teams live-caption DOM selectors.
//
// VERIFY LIVE: these selectors are derived from the architecture spec and
// public references and MUST be confirmed against a live meeting via the
// integration harness (TestMeetingEnableCaptions).
const (
	moreActionsSel    = `[data-tid="more-button"]`
	languageSpeechSel = `[data-tid="language-speech-button"]`
	turnOnCaptionsSel = `[data-tid="closed-captions-button"]`
	captionWrapperSel = `[data-tid="closed-caption-renderer-wrapper"]`
)

// captionObserverJS instantiates a MutationObserver on the Teams caption
// renderer. On each mutation it forwards caption blocks whose text ends with
// terminal punctuation (candidate finalized utterances) to Go via the
// sendCaptionToGo binding as JSON {speaker, text}. Authoritative deduplication
// is performed on the Go side. The IIFE returns true when the observer was
// attached (the caption wrapper was present).
const captionObserverJS = `(() => {
  const wrapper = document.querySelector('[data-tid="closed-caption-renderer-wrapper"]');
  if (!wrapper) { return false; }
  const observer = new MutationObserver(() => {
    const blocks = wrapper.querySelectorAll('[data-tid="closed-caption-text"]');
    blocks.forEach((el) => {
      const text = (el.textContent || '').trim();
      if (!/[.!?]$/.test(text)) { return; }
      const container = el.closest('.fui-ChatMessageCompact') || el.parentElement;
      const authorEl = container ? container.querySelector('[data-tid="author"]') : null;
      const speaker = authorEl ? (authorEl.textContent || '').trim() : '';
      window.sendCaptionToGo(JSON.stringify({ speaker: speaker, text: text }));
    });
  });
  observer.observe(wrapper, { childList: true, subtree: true, characterData: true });
  return true;
})()`

// captionPayload is the JSON structure emitted by the in-browser observer.
type captionPayload struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
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
	if text, ok := dedup.Ingest(p.Speaker, p.Text); ok {
		sink(p.Speaker, text)
	}
	return nil
}
