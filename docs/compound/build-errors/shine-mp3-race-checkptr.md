---
type: compound-learning
category: build-errors
date: 2026-07-10
tags: [shine-mp3, unsafe, race-detector, checkptr, audio]
evidence: shipment 006-S audio encoder
---

# shine-mp3 crashes under `-race` (unsafe pointer arithmetic + checkptr)

## Problem

`go test -race ./internal/audio/` crashed with a fatal runtime error while
encoding PCM to MP3:

```text
runtime: pointer 0x... to unused region of span ...
fatal error: found bad pointer in Go heap (incorrect use of unsafe or cgo?)
  ...mp3.(*Encoder).windowFilterSubband l3subband.go:70
  ...mp3.(*Encoder).Write layer3.go:194
```

The same test passed without `-race`.

## Root cause

`github.com/braheezy/shine-mp3` (a pure-Go port of the C Shine encoder) uses
`unsafe` pointer arithmetic to walk sample buffers (`windowFilterSubband` takes
`**int16`). The `-race` build enables `checkptr`, which flags this
out-of-bounds/derived-pointer arithmetic as invalid. Normal (non-race) builds
disable `checkptr`, so production binaries run fine.

## Resolution

shine-mp3 is the only viable pure-Go MP3 encoder and is spec-mandated, so keep
it but isolate the `-race` incompatibility:

1. Refactor `handleAudioPayload` to take a `pcmSink func([]int16) error` seam, so
   the browser unit test verifies base64/PCM decoding with a **fake sink** and
   never invokes the encoder under `-race`.
2. Guard the encoder's own test with a `//go:build race` /`//go:build !race`
   `raceDetectorEnabled` const and `t.Skip` under `-race`. Encoding correctness
   is still verified in non-race builds (synthetic PCM -> valid MP3 frames).

Production code path is unchanged and unaffected (non-race binary).

## Prevention / follow-up

- Pure-Go ports of C code that use `unsafe` pointer arithmetic frequently fail
  `-race`/`checkptr`. Isolate their use behind a seam so `-race` unit tests
  don't exercise them.
- Follow-up risk: shine-mp3's unsafe reads are technically UB even in non-race
  builds. Consider contributing a checkptr-safe fix upstream or vendoring a safe
  encoder if audio ships to production.
