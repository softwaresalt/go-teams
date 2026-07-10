package audio

import (
	"bytes"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mpeg1Layer3Bitrates maps the 4-bit bitrate index of an MPEG-1 Layer III frame
// header to kbps (index 0 = free, 15 = reserved).
var mpeg1Layer3Bitrates = []int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}

// countMP3Frames counts MPEG-1 Layer III frames at 44.1 kHz by parsing frame
// headers and skipping each frame by its computed length. This detects the
// "half the audio dropped" failure mode that a mere sync-word search cannot.
func countMP3Frames(data []byte) int {
	const sampleRate = 44100
	count := 0
	for i := 0; i+4 <= len(data); {
		if data[i] != 0xFF || data[i+1]&0xE0 != 0xE0 {
			i++
			continue
		}
		brIndex := (data[i+2] >> 4) & 0x0F
		padding := int((data[i+2] >> 1) & 0x01)
		if brIndex == 0 || brIndex == 15 {
			i++
			continue
		}
		bitrate := mpeg1Layer3Bitrates[brIndex] * 1000
		frameLen := 144*bitrate/sampleRate + padding
		if frameLen <= 0 {
			i++
			continue
		}
		count++
		i += frameLen
	}
	return count
}

func sineMono(n int) []int16 {
	s := make([]int16, n)
	for i := range s {
		s[i] = int16(0.3 * math.MaxInt16 * math.Sin(2*math.Pi*440*float64(i)/float64(SampleRate)))
	}
	return s
}

func TestEncoderProducesMP3Frames(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("shine-mp3 uses unsafe pointer arithmetic incompatible with -race checkptr; " +
			"encoding is verified in non-race builds")
	}
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	require.NoError(t, enc.WritePCM(sineMono(SampleRate)))
	require.Positive(t, buf.Len(), "expected encoded MP3 output")
}

// TestEncoderEncodesAllSamples is the regression test for shine-mp3's stereo
// stride dropping half of mono input: the encoded frame count must reflect the
// full sample count (~samples/1152), not half of it.
func TestEncoderEncodesAllSamples(t *testing.T) {
	if raceDetectorEnabled {
		t.Skip("shine-mp3 is incompatible with -race checkptr")
	}
	const frames = 8
	const samplesPerFrame = 1152
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	require.NoError(t, enc.WritePCM(sineMono(frames*samplesPerFrame)))

	got := countMP3Frames(buf.Bytes())
	// Allow a small tolerance for encoder framing, but reject the halved case
	// (which would yield ~4 frames for 8 frames of input).
	assert.GreaterOrEqualf(t, got, frames-1,
		"expected ~%d MP3 frames for %d mono samples; got %d (half would be ~%d)",
		frames, frames*samplesPerFrame, got, frames/2)
}

func TestEncoderEmptyInput(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	assert.NoError(t, enc.WritePCM(nil))
	assert.Zero(t, buf.Len())
}
