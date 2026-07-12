---
type: compound-learning
domain: build-errors
tags: [audio, shine-mp3, unsafe, out-of-bounds, mp3, buffering]
created: 2026-07-10
shipment: 007-S
status: active
---

# shine-mp3 over-reads non-frame-aligned input

## Problem

`github.com/braheezy/shine-mp3` v0.1.0's `(*Encoder).Write(out, data []int16)`
iterates the interleaved input in strides of `2 * samplesPerPass` (2304 values =
1152 stereo frames) but hands `&chunk[0]` (a raw pointer) to
`encodeBufferInterleaved`, which **unsafely reads a full 2304-value stride from
that pointer regardless of how many values actually remain**. See
`layer3.go:181-201` → `encodeBufferInterleaved` → `windowFilterSubband`.

Consequently, if `len(data)` is **not a whole multiple of 2304**, the final
iteration reads past the slice's backing array. The go-teams audio path fed
4096-mono-sample browser callbacks → 8192 interleaved values → `8192 / 2304 =
3.55`, so every callback over-read 1024 values. In production (non-race) builds
this silently corrupts the last frame or (rarely) faults; under `-race` it is a
fatal `checkptr: pointer arithmetic result points to invalid allocation`.

## Fix

Buffer PCM in the `audio.Encoder` and only ever hand shine-mp3 a **whole number
of frames** (multiples of 1152 mono / 2304 interleaved). Retain the sub-frame
remainder across `WritePCM` calls; add a `Flush` that pads the final partial
frame with silence and encodes it. Call `Flush` at end of stream (the browser
`CaptureAudio` teardown does this after the worker stops).

```go
const monoSamplesPerFrame = 1152
// WritePCM: append to e.buf, encode floor(len/1152)*1152, keep remainder.
// Flush: pad remainder up to a full frame with zeros, encode, reset.
```

## Critical distinction (do not conflate)

Two **separate** shine-mp3 issues exist:

1. **Over-read from non-frame-aligned input** (this learning) — a *caller* bug,
   fully fixable by frame-buffering. Fixed in `internal/audio/mp3.go`.
2. **End-of-buffer pointer in `windowFilterSubband`** (`l3subband.go:70`) — after
   the final frame it advances a read pointer one stride past the last sample
   (index `len+1`), a pointer it never dereferences, which tripped `-race`
   checkptr **even for valid frame-aligned input**. **RESOLVED in 009-S** by
   giving the interleaved buffer extra *capacity* (`encoderTailSlack`) in
   `audio.encodeFrames`, so the pointer stays inside the allocation. shine-mp3 is
   `NOASSERTION`-licensed, so it was NOT forked/patched. Encoder tests now run
   under `-race` (the `raceDetectorEnabled` skip and `race.go`/`norace.go` were
   removed). See `shine-mp3-race-checkptr.md`.

Historical note: before the 009-S capacity-slack fix, running the encoder under
`-race` crashed inside issue #2 regardless of frame alignment, so the over-read
fix (issue #1) had to be verified by confirming a frame-aligned length reached
shine-mp3 (e.g. 9216 = 4×2304) rather than by expecting `-race` to pass. With the
slack fix in place, `go test -race ./internal/audio/` now passes and is the
regression gate for both issues.
