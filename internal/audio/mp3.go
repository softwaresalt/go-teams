// Package audio provides pure-Go MP3 encoding for captured meeting audio.
package audio

import (
	"fmt"
	"io"

	"github.com/braheezy/shine-mp3/pkg/mp3"
)

// SampleRate is the PCM sample rate captured in the browser and encoded.
const SampleRate = 44100

// encodeChannels is the channel count passed to shine-mp3. shine-mp3 v0.1.0's
// Write advances its input cursor by 2*samplesPerPass (a stereo-interleaved
// assumption), so mono input MUST be encoded as dual-mono interleaved stereo;
// otherwise the encoder drops half of the samples.
const encodeChannels = 2

// Encoder streams mono int16 PCM samples to an io.Writer as MP3 using a pure-Go
// encoder (no CGO or external tools). Mono input is encoded as dual-mono stereo
// to work around shine-mp3's stereo-only stride.
type Encoder struct {
	enc *mp3.Encoder
	w   io.Writer
}

// NewEncoder returns an Encoder that writes MP3 frames to w for mono PCM sampled
// at sampleRate.
func NewEncoder(w io.Writer, sampleRate int) *Encoder {
	return &Encoder{enc: mp3.NewEncoder(sampleRate, encodeChannels), w: w}
}

// WritePCM encodes mono int16 PCM samples and writes the resulting MP3 frames to
// the underlying writer. The samples are interleaved as dual-mono stereo before
// encoding. Empty input is a no-op.
func (e *Encoder) WritePCM(mono []int16) error {
	if len(mono) == 0 {
		return nil
	}
	stereo := make([]int16, len(mono)*2)
	for i, s := range mono {
		stereo[2*i] = s
		stereo[2*i+1] = s
	}
	if err := e.enc.Write(e.w, stereo); err != nil {
		return fmt.Errorf("audio: encode mp3: %w", err)
	}
	return nil
}
