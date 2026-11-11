package audio

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	require.NoError(t, enc.WritePCM(sineMono(SampleRate)))
	require.Positive(t, buf.Len(), "expected encoded MP3 output")
}

// TestEncoderEncodesAllSamples is the regression test for shine-mp3's stereo
// stride dropping half of mono input: the encoded frame count must reflect the
// full sample count (~samples/1152), not half of it.
func TestEncoderEncodesAllSamples(t *testing.T) {
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

// TestEncoderHandlesNonFrameAlignedInput is the regression for the shine-mp3
// out-of-bounds read: browser callbacks deliver 4096-sample chunks (not a
// multiple of 1152), which previously over-read the encoder's interleaved buffer
// on the final pass of every callback. Buffering whole frames must prevent this.
func TestEncoderHandlesNonFrameAlignedInput(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	for i := 0; i < 4; i++ { // four 4096-sample browser callbacks
		require.NoError(t, enc.WritePCM(sineMono(4096)))
	}
	require.NoError(t, enc.Flush())
	require.Positive(t, buf.Len())
}

// TestEncoderFlushEmitsRemainder verifies the buffered sub-frame tail is encoded
// on Flush rather than dropped.
func TestEncoderFlushEmitsRemainder(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	require.NoError(t, enc.WritePCM(sineMono(500))) // fewer than one frame
	assert.Zero(t, buf.Len(), "sub-frame input should be buffered, not encoded")
	require.NoError(t, enc.Flush())
	assert.Positive(t, buf.Len(), "Flush should emit the padded remainder frame")
}

// Golden reference for TestEncoderDeterministicGoldenReference. These values pin
// the exact MP3 byte stream produced by Encoder (shine-mp3 v0.1.0, 44.1 kHz,
// dual-mono) for goldenPCM. Any change to the encoder, its framing, or the
// shine-mp3 dependency that alters output bytes must be reviewed deliberately
// and the constants regenerated.
const (
	goldenMP3Len    = 1252
	goldenMP3SHA256 = "8f48238a04612c14851b93c7cd0e56f29d91609c7090f1af269d8e23d25288b6"
	goldenMP3Prefix = "fffb920400022353155601ef79c054e2"
	goldenMP3Suffix = "5bcca62a967bffa327db2f992f1348bb"
	goldenEdgeBytes = 16
)

// goldenPCM synthesizes a deterministic, non-trivial mono signal of n samples
// using integer arithmetic only (a triangle wave plus linear-congruential
// noise), so the bytes are identical on every platform and architecture
// regardless of floating-point behavior.
func goldenPCM(n int) []int16 {
	s := make([]int16, n)
	var lcg uint32 = 0x1badf00d
	for i := range s {
		lcg = lcg*1664525 + 1013904223
		phase := i % 100
		tri := phase
		if phase >= 50 {
			tri = 100 - phase
		}
		// Triangle spans [-8000, +8000]; noise adds +/-511 of fine detail.
		s[i] = int16((tri-25)*320 + int(lcg>>22) - 512)
	}
	return s
}

// encodeGolden encodes pcm through a fresh Encoder using uneven chunk sizes (to
// exercise the sub-frame buffering path) and returns the resulting MP3 bytes.
func encodeGolden(t *testing.T, pcm []int16) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	const chunk = 1000
	for off := 0; off < len(pcm); off += chunk {
		end := min(off+chunk, len(pcm))
		require.NoError(t, enc.WritePCM(pcm[off:end]))
	}
	require.NoError(t, enc.Flush())
	return buf.Bytes()
}

// TestEncoderDeterministicGoldenReference locks in deterministic MP3 output: two
// independent encoder runs over the same PCM must match byte for byte, and the
// output must match a pinned golden reference (length, SHA-256, leading and
// trailing bytes). It never builds a zero-slack (cap == len) stereo buffer, so
// it stays checkptr-safe under go test -race (see encoderTailSlack).
func TestEncoderDeterministicGoldenReference(t *testing.T) {
	pcm := goldenPCM(3 * monoSamplesPerFrame)

	out1 := encodeGolden(t, pcm)
	out2 := encodeGolden(t, pcm)
	require.NotEmpty(t, out1)
	require.Equal(t, out1, out2, "two independent encoder runs must be byte-identical")

	sum := sha256.Sum256(out1)
	gotPrefix := hex.EncodeToString(out1[:goldenEdgeBytes])
	gotSuffix := hex.EncodeToString(out1[len(out1)-goldenEdgeBytes:])
	assert.Equal(t, goldenMP3Len, len(out1), "total encoded byte length")
	assert.Equal(t, goldenMP3SHA256, hex.EncodeToString(sum[:]), "SHA-256 of encoded output")
	assert.Equal(t, goldenMP3Prefix, gotPrefix, "leading %d bytes", goldenEdgeBytes)
	assert.Equal(t, goldenMP3Suffix, gotSuffix, "trailing %d bytes", goldenEdgeBytes)
}

func TestEncoderEmptyInput(t *testing.T) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, SampleRate)
	assert.NoError(t, enc.WritePCM(nil))
	assert.Zero(t, buf.Len())
}

func TestEncoderNilWriter(t *testing.T) {
	enc := NewEncoder(nil, SampleRate)
	// Empty input is a no-op even with a nil writer.
	assert.NoError(t, enc.WritePCM(nil))
	// Non-empty input with a nil writer returns an error rather than panicking.
	assert.Error(t, enc.WritePCM([]int16{1, 2, 3, 4}))
}
