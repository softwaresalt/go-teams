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

func TestHandleAudioPayloadDecodesPCM(t *testing.T) {
	want := []int16{0, 100, -100, 32767, -32768, 12345}
	var got []int16
	sink := func(samples []int16) error {
		got = append(got, samples...)
		return nil
	}
	require.NoError(t, handleAudioPayload(encodePCM(want), sink))
	assert.Equal(t, want, got)
}

func TestHandleAudioPayloadInvalidBase64(t *testing.T) {
	called := false
	err := handleAudioPayload("!!! not base64 !!!", func([]int16) error { called = true; return nil })
	assert.Error(t, err)
	assert.False(t, called)
}

func TestHandleAudioPayloadOddLength(t *testing.T) {
	raw := base64.StdEncoding.EncodeToString([]byte{1, 2, 3})
	err := handleAudioPayload(raw, func([]int16) error { return nil })
	assert.Error(t, err)
}

func TestAudioCaptureJSStructure(t *testing.T) {
	assert.Contains(t, audioCaptureJS, "getDisplayMedia")
	assert.Contains(t, audioCaptureJS, "createScriptProcessor")
	assert.Contains(t, audioCaptureJS, "sendAudioToGo")
	assert.Contains(t, audioCaptureJS, "getVideoTracks")
}
