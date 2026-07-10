package browser

import (
	"encoding/base64"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodePCM(samples []int16) string {
	buf := make([]byte, len(samples)*2)
	for i, s := range samples {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(s))
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func TestDecodeAudioPayload(t *testing.T) {
	want := []int16{0, 100, -100, 32767, -32768, 12345}
	got, err := decodeAudioPayload(encodePCM(want))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestDecodeAudioPayloadInvalidBase64(t *testing.T) {
	_, err := decodeAudioPayload("!!! not base64 !!!")
	assert.Error(t, err)
}

func TestDecodeAudioPayloadOddLength(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	_, err := decodeAudioPayload(raw)
	assert.Error(t, err)
}

func TestAudioCaptureJSStructure(t *testing.T) {
	assert.Contains(t, audioCaptureJS, "getDisplayMedia")
	assert.Contains(t, audioCaptureJS, "createScriptProcessor")
	assert.Contains(t, audioCaptureJS, "sendAudioToGo")
	assert.Contains(t, audioCaptureJS, "getVideoTracks")
}
