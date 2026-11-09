package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/transcript"
)

func TestHandleCaptionPayload(t *testing.T) {
	dedup := transcript.NewDeduplicator()
	var got [][2]string
	sink := func(speaker, text string) { got = append(got, [2]string{speaker, text}) }

	// New finalized utterance -> delivered.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Alice","text":"Hello world."}`, dedup, sink))
	require.Len(t, got, 1)
	assert.Equal(t, [2]string{"Alice", "Hello world."}, got[0])

	// Exact duplicate -> suppressed.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Alice","text":"Hello world."}`, dedup, sink))
	assert.Len(t, got, 1)

	// Non-final fragment -> suppressed.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Bob","text":"a partial fragment"}`, dedup, sink))
	assert.Len(t, got, 1)

	// Different speaker, new final -> delivered.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Bob","text":"A new sentence."}`, dedup, sink))
	assert.Len(t, got, 2)

	// Settled unpunctuated fragment (final:true) -> delivered.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Bob","text":"Sounds good to me","final":true}`, dedup, sink))
	require.Len(t, got, 3)
	assert.Equal(t, [2]string{"Bob", "Sounds good to me"}, got[2])

	// Duplicate settled fragment after punctuation added -> suppressed.
	require.NoError(t, handleCaptionPayload(`{"speaker":"Bob","text":"Sounds good to me."}`, dedup, sink))
	assert.Len(t, got, 3)
}

func TestHandleCaptionPayloadInvalidJSON(t *testing.T) {
	dedup := transcript.NewDeduplicator()
	called := false
	err := handleCaptionPayload("not json", dedup, func(string, string) { called = true })
	assert.Error(t, err)
	assert.False(t, called)
}

func TestCaptionObserverJSStructure(t *testing.T) {
	assert.Contains(t, captionObserverJS, "MutationObserver")
	assert.Contains(t, captionObserverJS, "closed-caption-renderer-wrapper")
	assert.Contains(t, captionObserverJS, "closed-caption-text")
	assert.Contains(t, captionObserverJS, "sendCaptionToGo")
	assert.Contains(t, captionObserverJS, "characterData")
}
