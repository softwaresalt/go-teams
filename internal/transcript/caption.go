// Package transcript models Teams live-caption events and renders them to a
// Markdown transcript.
package transcript

import (
	"regexp"
	"strings"
	"time"
)

// CaptionEvent is a single finalized caption utterance.
type CaptionEvent struct {
	Timestamp time.Time
	Speaker   string
	Text      string
}

var (
	terminalPunct = regexp.MustCompile(`[.!?]$`)
	nonAlphaNum   = regexp.MustCompile(`[^\p{L}\p{N}\s]+`)
	whitespace    = regexp.MustCompile(`\s+`)
)

// Deduplicator implements the rolling-text heuristic used to collapse the
// stream of partial caption mutations emitted by the Teams speech-to-text
// engine into finalized utterances. A fragment is treated as final only when
// it ends with terminal punctuation, and duplicate finals for the same speaker
// are suppressed.
type Deduplicator struct {
	lastFinal map[string]string
}

// NewDeduplicator returns a ready-to-use Deduplicator.
func NewDeduplicator() *Deduplicator {
	return &Deduplicator{lastFinal: make(map[string]string)}
}

// Ingest processes one raw caption fragment for speaker. It returns the
// finalized, trimmed text and true when the fragment completes a new utterance;
// otherwise it returns an empty string and false (partial fragment, empty
// content, or an exact duplicate of the speaker's previous utterance).
func (d *Deduplicator) Ingest(speaker, text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	norm := normalize(trimmed)
	if norm == "" {
		return "", false
	}
	if !terminalPunct.MatchString(trimmed) {
		return "", false
	}
	if d.lastFinal[speaker] == norm {
		return "", false
	}
	d.lastFinal[speaker] = norm
	return trimmed, true
}

// IngestSettled processes a caption fragment that has settled in the DOM (no
// further mutations observed within the settle window) without requiring
// trailing sentence punctuation. It shares the same per-speaker normalized
// deduplication key as Ingest so a settled utterance and a later punctuated
// revision of the same words are never emitted twice.
func (d *Deduplicator) IngestSettled(speaker, text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	norm := normalize(trimmed)
	if norm == "" {
		return "", false
	}
	if d.lastFinal[speaker] == norm {
		return "", false
	}
	d.lastFinal[speaker] = norm
	return trimmed, true
}

// normalize reduces a fragment to a comparison key: lowercased, with
// punctuation removed and whitespace collapsed.
func normalize(s string) string {
	s = strings.ToLower(s)
	s = nonAlphaNum.ReplaceAllString(s, " ")
	s = whitespace.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
