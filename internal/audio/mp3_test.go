package audio

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hasMP3Sync reports whether b contains an MP3 frame sync word (0xFF followed
// by a byte whose top three bits are set).
func hasMP3Sync(b []byte) bool {
	for i := 0; i+1 < len(b); i++ {
		if b[i] == 0xFF && b[i+1]&0xE0 == 0xE0 {
			return true
		}
	}
	return false
}

func TestEncoderProducesMP3Frames(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("shine-mp3 uses unsafe pointer arithmetic incompatible with -race checkptr; " +
			"encoding is verified in non-race builds")
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate, Channels)

	// One second of a 440 Hz sine wave.
	samples := make([]int16, SampleRate)
	for i := range samples {
		samples[i] = int16(0.3 * math.MaxInt16 * math.Sin(2*math.Pi*440*float64(i)/float64(SampleRate)))
	}
	require.NoError(t, enc.WritePCM(samples))

	out := buf.Bytes()
	require.Positive(t, len(out), "expected encoded MP3 output")
	assert.True(t, hasMP3Sync(out), "output should contain an MP3 frame sync word")
}

func TestEncoderEmptyInput(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate, Channels)
	assert.NoError(t, enc.WritePCM(nil))
	assert.Zero(t, buf.Len())
}
