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

// monoSamplesPerFrame is the MPEG-1 Layer III samples-per-channel per frame
// (1152). shine-mp3 advances its interleaved input by 2*1152 values per pass and
// unsafely reads a full pass from the chunk pointer even for the final, short
// chunk. Feeding a buffer whose length is not a whole number of frames therefore
// reads past the slice (out of bounds). WritePCM buffers partial frames so only
// whole frames are ever handed to the encoder.
const monoSamplesPerFrame = 1152

// Encoder streams mono int16 PCM samples to an io.Writer as MP3 using a pure-Go
// encoder (no CGO or external tools). Mono input is encoded as dual-mono stereo
// to work around shine-mp3's stereo-only stride, and is buffered into whole MP3
// frames so the encoder never reads past its input.
type Encoder struct {
	enc *mp3.Encoder
	w   io.Writer
	buf []int16 // mono samples not yet forming a complete frame
}

// NewEncoder returns an Encoder that writes MP3 frames to w for mono PCM sampled
// at sampleRate.
func NewEncoder(w io.Writer, sampleRate int) *Encoder {
	return &Encoder{enc: mp3.NewEncoder(sampleRate, encodeChannels), w: w}
}

// WritePCM buffers mono int16 PCM samples and encodes as many complete MP3
// frames as are available, writing the resulting frames to the underlying
// writer. Samples that do not fill a whole frame are retained for the next call;
// call Flush to emit them at end of stream. Empty input is a no-op.
func (e *Encoder) WritePCM(mono []int16) error {
	if len(mono) == 0 {
		return nil
	}
	if e.w == nil {
		return fmt.Errorf("audio: encoder has no writer")
	}
	e.buf = append(e.buf, mono...)
	completeFrames := len(e.buf) / monoSamplesPerFrame
	if completeFrames == 0 {
		return nil
	}
	n := completeFrames * monoSamplesPerFrame
	if err := e.encodeFrames(e.buf[:n]); err != nil {
		return err
	}
	// Retain the sub-frame remainder for the next call.
	rem := copy(e.buf, e.buf[n:])
	e.buf = e.buf[:rem]
	return nil
}

// Flush pads any buffered sub-frame remainder with silence to a whole frame and
// encodes it, so no captured audio is lost at end of stream. It is a no-op when
// nothing is buffered.
func (e *Encoder) Flush() error {
	if len(e.buf) == 0 {
		return nil
	}
	if e.w == nil {
		return fmt.Errorf("audio: encoder has no writer")
	}
	if pad := monoSamplesPerFrame - (len(e.buf) % monoSamplesPerFrame); pad != monoSamplesPerFrame {
		e.buf = append(e.buf, make([]int16, pad)...)
	}
	if err := e.encodeFrames(e.buf); err != nil {
		return err
	}
	e.buf = e.buf[:0]
	return nil
}

// encodeFrames encodes a whole number of mono frames as dual-mono interleaved
// stereo. len(mono) MUST be a non-zero multiple of monoSamplesPerFrame so the
// encoder never reads past the interleaved buffer.
func (e *Encoder) encodeFrames(mono []int16) error {
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
