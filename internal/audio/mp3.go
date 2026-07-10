// Package audio provides pure-Go MP3 encoding for captured meeting audio.
package audio

import (
	"fmt"
	"io"

	"github.com/braheezy/shine-mp3/pkg/mp3"
)

// Standard parameters for captured meeting audio.
const (
	// SampleRate is the PCM sample rate captured in the browser and encoded.
	SampleRate = 44100
	// Channels is the captured channel count (mono).
	Channels = 1
)

// Encoder streams interleaved int16 PCM samples to an io.Writer as MP3 using a
// pure-Go encoder (no CGO or external tools).
type Encoder struct {
	enc *mp3.Encoder
	w   io.Writer
}

// NewEncoder returns an Encoder that writes MP3 frames to w for PCM sampled at
// sampleRate with the given channel count.
func NewEncoder(w io.Writer, sampleRate, channels int) *Encoder {
	return &Encoder{enc: mp3.NewEncoder(sampleRate, channels), w: w}
}

// WritePCM encodes samples (interleaved int16 PCM) and writes the resulting MP3
// frames to the underlying writer. Empty input is a no-op.
func (e *Encoder) WritePCM(samples []int16) error {
	if len(samples) == 0 {
		return nil
	}
	if err := e.enc.Write(e.w, samples); err != nil {
		return fmt.Errorf("audio: encode mp3: %w", err)
	}
	return nil
}
