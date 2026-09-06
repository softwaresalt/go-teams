---
type: memory-compacted
date: 2026-07-11
shipment: 009-S
feature: 009-F
source: docs/archive/memory/2026-07-11/009-S-mp3-race-safety-memory.md
---

# Compacted memory — Shipment 009-S (shine-mp3 -race safety)

**Shipped**: PR #23, merge `3d59b7d`. Post-merge archival: PR #24, merge
`12f0dc5`. Closure backfill (this compaction + `docs/closure/009-S-009-F-post-merge-closure.md`): 2026-09-05.

## Decision and outcome

shine-mp3's `windowFilterSubband` advances an internal read pointer one
stride past the final sample (index `len+1`) — never dereferenced, but
`-race` checkptr faulted on the pointer arithmetic. shine-mp3 is
`NOASSERTION`-licensed (vendoring/forking rejected — copyright risk).
Fix: `audio.encodeFrames` allocates the interleaved buffer with small extra
**capacity** (`encoderTailSlack = 4`) so the end pointer stays inside the
allocation. Capacity-only change → MP3 output byte-identical. Removed
`raceDetectorEnabled` skip guards + `race.go`/`norace.go`; encoder tests now
run under `-race` unconditionally.

## Key reusable insight

When a third-party library computes a benign end-of-buffer pointer via
`unsafe` that trips `-race` checkptr, and the library cannot be modified,
sizing the input buffer with spare **capacity** (not length) keeps the
pointer in-allocation and satisfies checkptr without changing output. The
re-enabled `-race` test suite becomes the upgrade/regression gate.
(Durable form of this learning lives in
`docs/compound/build-errors/shine-mp3-race-checkptr.md`.)

## Review outcome

Adversarial review: correctness reviewer clean (slack proven sufficient —
need `>= len+2`, have 4); Go reviewer's 2 P2 + 1 P3 doc/durability findings
applied (no P0/P1). 1 Copilot cycle (cross-doc consistency).

## Follow-ups carried forward (unchanged, tracked elsewhere)

- Live Teams DOM/audio end-to-end verification — stash `CC0A7034` / shipment
  `010-S` (queued, out of scope for 009-S).
- CI `paths-ignore` tuning — stash `D3EFD224` (low priority, parked).
- shine-mp3 `unsafe`/`-race` upstream fix — tracked in
  `docs/compound/build-errors/shine-mp3-race-checkptr.md`.

Verbose original archived at
`docs/archive/memory/2026-07-11/009-S-mp3-race-safety-memory.md`.
