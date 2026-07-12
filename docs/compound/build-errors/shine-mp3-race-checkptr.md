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

`github.com/braheezy/shine-mp3` (a pure-Go port of the C Shine encoder) walks
its interleaved input with `unsafe` pointer arithmetic (`windowFilterSubband`
advances a `*int16` by the channel stride). After the final frame it computes an
**end-of-buffer pointer one/two `int16` past the last sample** — it never
dereferences it, but under `-race` the `checkptr` instrumentation faults on the
arithmetic itself (`checkptr: pointer arithmetic result points to invalid
allocation`). Normal (non-race) builds disable `checkptr`, so production binaries
run fine. The pointer is only past the *slice length*, not intentionally reading
foreign memory.

## Resolution (009-S — supersedes the earlier `-race` skip workaround)

The fix needs **no shine-mp3 modification** (its license is `NOASSERTION`, so
vendoring/forking is off the table): give the interleaved buffer handed to
`enc.Write` a small extra **capacity** so shine-mp3's benign end-of-buffer
pointer stays inside the allocation.

```go
// internal/audio/mp3.go — encodeFrames
stereo := make([]int16, len(mono)*2, len(mono)*2+encoderTailSlack) // slack cap
```

`encoderTailSlack` (4 int16) is capacity only — never part of the encoded length
— so the MP3 output is byte-identical, but `checkptr` no longer faults. With this
in place the encoder tests run **under `-race`** (the `raceDetectorEnabled` /
`//go:build race` skip guard and `race.go`/`norace.go` were removed).

Verified: `go test -race ./internal/audio/` passes; the earlier probe that
crashed inside `windowFilterSubband` now succeeds with the slack.

### Earlier workaround (now removed)

Previously the encoder tests were skipped under `-race` via a `raceDetectorEnabled`
build-tag const, with encoding verified only in non-race builds. That hid the
`-race` coverage; the capacity-slack fix restores it.

## Prevention / follow-up

- Pure-Go ports of C code often compute end-of-buffer pointers with `unsafe`.
  When you cannot modify the dependency, sizing the input buffer with spare
  **capacity** (not length) can keep such pointers inside the allocation and
  satisfy `checkptr`, without changing output.
- Frame-align input to shine-mp3 (whole 1152-sample frames) to avoid a genuine
  over-read — see `shine-mp3-frame-alignment.md`.
