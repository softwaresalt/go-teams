package browser

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
)

// audioBinding is the CDP runtime binding the injected capture script calls to
// forward PCM chunks to Go.
const audioBinding = "sendAudioToGo"

// audioQueueSize bounds the buffer between chromedp's event goroutine and the
// audio worker.
const audioQueueSize = 256

// audioCaptureJS captures the meeting tab's audio via getDisplayMedia, discards
// the video track, and forwards raw int16 PCM (mono, 44.1 kHz) as base64 chunks
// to Go via the sendAudioToGo binding. The async IIFE resolves to true when
// capture started.
//
// VERIFY LIVE: getDisplayMedia auto-approval depends on the
// --auto-select-desktop-capture-source allocator flag; confirm against a live
// meeting via the integration harness (TestMeetingCaptureAudio).
const audioCaptureJS = `(async () => {
  const stream = await navigator.mediaDevices.getDisplayMedia({ audio: true, video: true });
  stream.getVideoTracks().forEach((t) => t.stop());
  const track = stream.getAudioTracks()[0];
  if (!track) { return false; }
  const ctx = new AudioContext({ sampleRate: 44100 });
  const source = ctx.createMediaStreamSource(new MediaStream([track]));
  const processor = ctx.createScriptProcessor(4096, 1, 1);
  source.connect(processor);
  processor.connect(ctx.destination);
  processor.onaudioprocess = (event) => {
    const input = event.inputBuffer.getChannelData(0);
    const pcm = new Int16Array(input.length);
    for (let i = 0; i < input.length; i++) {
      const s = Math.max(-1, Math.min(1, input[i]));
      pcm[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
    }
    const bytes = new Uint8Array(pcm.buffer);
    let binary = '';
    const chunk = 0x8000;
    for (let i = 0; i < bytes.length; i += chunk) {
      binary += String.fromCharCode.apply(null, bytes.subarray(i, i + chunk));
    }
    window.sendAudioToGo(btoa(binary));
  };
  return true;
})()`

// decodeAudioPayload decodes a base64 little-endian int16 PCM chunk into mono
// samples. It returns an error only for a malformed payload (bad base64 or odd
// byte length).
func decodeAudioPayload(raw string) ([]int16, error) {
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("browser: decode audio payload: %w", err)
	}
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("browser: audio payload has odd byte length %d", len(data))
	}
	samples := make([]int16, len(data)/2)
	for i := range samples {
		samples[i] = int16(binary.LittleEndian.Uint16(data[i*2:]))
	}
	return samples, nil
}
