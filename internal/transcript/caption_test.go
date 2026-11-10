package transcript

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDeduplicatorRollingText(t *testing.T) {
	d := NewDeduplicator()
	fragments := []string{
		"The architecture",
		"The architecture requires",
		"The architecture requires a standalone",
		"The architecture requires a standalone binary.",
	}
	var finals []string
	for _, f := range fragments {
		if txt, ok := d.Ingest("Alice", f); ok {
			finals = append(finals, txt)
		}
	}
	assert.Equal(t, []string{"The architecture requires a standalone binary."}, finals)
}

func TestDeduplicatorSuppressesDuplicateFinal(t *testing.T) {
	d := NewDeduplicator()
	_, ok1 := d.Ingest("Bob", "Hello world.")
	assert.True(t, ok1)
	_, ok2 := d.Ingest("Bob", "hello world!") // same normalized key
	assert.False(t, ok2)
}

func TestDeduplicatorPerSpeaker(t *testing.T) {
	d := NewDeduplicator()
	_, ok1 := d.Ingest("Alice", "Same sentence.")
	_, ok2 := d.Ingest("Bob", "Same sentence.")
	assert.True(t, ok1)
	assert.True(t, ok2, "identical text from a different speaker is not a duplicate")
}

func TestDeduplicatorNonFinal(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"no terminal punctuation", "Just a fragment"},
		{"empty", ""},
		{"whitespace only", "   "},
		{"punctuation only", "..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := NewDeduplicator()
			_, ok := d.Ingest("Alice", tc.text)
			assert.False(t, ok)
		})
	}
}

func TestDeduplicatorTerminalVariants(t *testing.T) {
	for _, text := range []string{"A question?", "An exclamation!", "A statement."} {
		d := NewDeduplicator()
		got, ok := d.Ingest("Alice", text)
		assert.True(t, ok)
		assert.Equal(t, text, got)
	}
}

func TestDeduplicatorTrimsWhitespace(t *testing.T) {
	d := NewDeduplicator()
	got, ok := d.Ingest("Alice", "  Trimmed sentence.  ")
	assert.True(t, ok)
	assert.Equal(t, "Trimmed sentence.", got)
}

func TestDeduplicatorIngestSettled(t *testing.T) {
	d := NewDeduplicator()

	// Unpunctuated fragment that settled after speaker paused is emitted.
	got, ok := d.IngestSettled("Alice", "  Thanks everyone for joining  ")
	assert.True(t, ok)
	assert.Equal(t, "Thanks everyone for joining", got)

	// Duplicate settled fragment is suppressed.
	_, ok = d.IngestSettled("Alice", "Thanks everyone for joining")
	assert.False(t, ok)

	// Retroactive punctuation on the same utterance is also suppressed as duplicate.
	_, ok = d.Ingest("Alice", "Thanks everyone for joining.")
	assert.False(t, ok)

	// Empty or punctuation-only settled fragment is ignored.
	_, ok = d.IngestSettled("Alice", "   ")
	assert.False(t, ok)
	_, ok = d.IngestSettled("Alice", "...")
	assert.False(t, ok)
}
