---
type: session-memory
date: 2026-07-11
shipment: 009-S
outcome: shipped
---

# Session memory — Shipment 009-S (shine-mp3 -race safety)

## Shipped (PR #23, merge `3d59b7d`)

Resolved stash `3E55DF0D`. shine-mp3's `windowFilterSubband` advances a read
pointer one stride past the final sample (index `len+1`) — never dereferenced,
but `-race` checkptr faulted on the arithmetic, so encoder tests were skipped
under `-race`.

**Fix (no shine-mp3 modification):** shine-mp3 is `NOASSERTION`-licensed, so
vendoring/forking to patch the `unsafe` was rejected (copyright risk). Instead,
`audio.encodeFrames` allocates the interleaved buffer with a small extra
**capacity** (`encoderTailSlack = 4`) so the end pointer stays inside the
allocation. Capacity-only → MP3 output byte-identical. Removed the
`raceDetectorEnabled` skip guards + `race.go`/`norace.go`; encoder tests now run
under `-race`.

Adversarial review: correctness clean (slack proven sufficient — need `>= len+2`,
have 4); Go review's doc/durability findings applied. 1 Copilot cycle (cross-doc
consistency in the frame-alignment learning). Compound updated.

## Key insight (reusable)

When a third-party lib computes a benign end-of-buffer pointer via `unsafe` that
trips `-race` checkptr, and you cannot modify the lib, sizing the input with
spare **capacity** (not length) keeps the pointer in-allocation and satisfies
checkptr without changing output. Re-enabled `-race` tests are the upgrade gate.

## Dark-mode scope status

- 007-S (capture wiring), 008-S (transcript hardening), 009-S (mp3 -race safety):
  all shipped + closed this run.
- Remaining stash — DEFERRED:
  - `CC0A7034` (high) — live Teams DOM verification; operator-required
    (login/MFA + real meeting). All selectors remain `VERIFY LIVE`.
  - `D3EFD224` (low) — CI `paths-ignore`; no current value (`required_checks=[]`).

## Next session

Operator input needed for `CC0A7034` (live meeting + auth). `D3EFD224` parked.
No autonomously-actionable stash remains.
