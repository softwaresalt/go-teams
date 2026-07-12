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
// unsafely DEREFERENCES a full pass from the chunk pointer even for the final,
// short chunk. Feeding a buffer whose length is not a whole number of frames
// therefore reads real samples past the slice (a genuine out-of-bounds read).
// WritePCM buffers partial frames so only whole frames are ever handed to the
// encoder, keeping every dereference in bounds.
const monoSamplesPerFrame = 1152

// encoderTailSlack is spare CAPACITY (in int16 values) appended to the
// interleaved buffer handed to shine-mp3. Distinct from monoSamplesPerFrame
// (which prevents out-of-bounds dereferences): shine-mp3 v0.1.0's
// windowFilterSubband also ADVANCES a read pointer one stride past the final
// sample without dereferencing it. Because the right channel starts at offset 1
// and the stride is 2, that end pointer reaches index len+1 — two elements past
// the last valid index. The slack keeps that pointer inside the allocation so
// the -race checkptr instrumentation does not fault; 4 (>= the required 2)
// leaves margin. It is capacity only, never part of the encoded length, so the
// MP3 output is byte-identical. The re-enabled `go test -race ./internal/audio/`
// is the upgrade gate: if a future shine-mp3 version moves the pointer further,
// that test faults and this constant must be revisited.
const encoderTailSlack = 4

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
	// Whole-frame input (monoSamplesPerFrame) keeps every shine-mp3 dereference
	// in bounds; encoderTailSlack additionally keeps its benign end-of-buffer
	// pointer (advanced but never dereferenced) inside the allocation so -race
	// checkptr does not fault. The slack is capacity only, so output is unchanged.
	stereo := make([]int16, len(mono)*2, len(mono)*2+encoderTailSlack)
	for i, s := range mono {
		stereo[2*i] = s
		stereo[2*i+1] = s
	}
	if err := e.enc.Write(e.w, stereo); err != nil {
		return fmt.Errorf("audio: encode mp3: %w", err)
	}
	return nil
}
