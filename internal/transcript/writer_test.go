package transcript

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriterHeader(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	start := time.Date(2026, 7, 9, 13, 5, 30, 0, time.UTC)
	require.NoError(t, w.WriteHeader(Metadata{
		MeetingID: "19:meeting_abc",
		StartTime: start,
		BotName:   "Transcription Agent",
	}))
	require.NoError(t, w.Flush())

	out := buf.String()
	assert.Contains(t, out, `meeting_id: "19:meeting_abc"`)
	assert.Contains(t, out, "date: 2026-07-09")
	assert.Contains(t, out, `start_time: "13:05:30"`)
	assert.Contains(t, out, `bot_name: "Transcription Agent"`)
	assert.Contains(t, out, "# Transcript")
}

func TestWriterEvent(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	ev := CaptionEvent{
		Timestamp: time.Date(2026, 7, 9, 13, 6, 5, 0, time.UTC),
		Speaker:   "Alice",
		Text:      "The architecture requires a standalone binary.",
	}
	require.NoError(t, w.WriteEvent(ev))
	require.NoError(t, w.Flush())

	assert.Equal(t,
		"**Alice (13:06:05):** The architecture requires a standalone binary.\n\n",
		buf.String())
}

func TestWriterBuffersUntilFlush(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)
	require.NoError(t, w.WriteHeader(Metadata{MeetingID: "x", StartTime: time.Now(), BotName: "b"}))
	assert.Zero(t, buf.Len(), "small writes stay buffered until Flush")
	require.NoError(t, w.Flush())
	assert.Positive(t, buf.Len())
}
