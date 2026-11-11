---
title: Deterministic MP3 golden regression test and CI path-filter trigger hygiene decision
date: 2026-11-10
status: accepted
deliberation_id: 003-DL
feature: 014-F
shipment: 014-S
source_stash:
  - FE245033
  - D3EFD224
---

## Problem frame

Two remaining maintenance and verification items in the stash address test
determinism and CI status-check reporting:

1. **`FE245033`** (`low`, `task`, `DEFERRED SCOPE EXPANSION` from PR #28 /
   `009-S`): Shipment `009-S` resolved the `shine-mp3` `-race` `checkptr` crash
   by allocating `stereo := make([]int16, len(mono)*2, len(mono)*2+encoderTailSlack)`
   in `internal/audio/mp3.go`. While `009-S`'s post-merge closure record
   (`docs/closure/009-S-009-F-post-merge-closure.md`) proved at the source level
   that growing slice capacity (`cap`) without changing slice length (`len`)
   does not alter the bytes `shine-mp3` encodes,    `internal/audio/mp3_test.go` currently checks only frame counts and
   non-empty buffer emission — it does not verify deterministic byte-for-byte
   reproducibility across encoder runs or compare the encoded MP3 output
   against a fixed golden reference (pinned total byte length, SHA-256 digest,
   and leading/trailing frame bytes; note that a live zero-slack `cap == len`
   buffer MUST NOT be passed to `shine-mp3` under `-race` because that triggers
   the exact `checkptr` fault fixed in `009-S`).
2. **`D3EFD224`** (`low`, `task`): `.github/workflows/ci.yml` currently
   configures `paths-ignore` on both `push` and `pull_request` triggers for
   `.backlogit/**`, `docs/**`, `.autoharness/**`, and `**/*.md`. Because the
   workflow is skipped entirely on documentation- or backlog-only pull requests,
   the lightweight `Detect code changes` (`changes`) job never runs or reports a
   status check on those PRs, even though the `Build, vet, lint & test` (`test`)
   job is already self-gated via `if: needs.changes.outputs.code == 'true'` and
   the `dorny/paths-filter` denylist.

## P-021 C5 and C6 triage and reconciliation record

Because `FE245033` carries the `DEFERRED SCOPE EXPANSION` marker, Step 1.2
precedence routes the group through Step 2 deliberation (`003-DL`) regardless of
the entry's `requires deliberation: NO` note (P-021 C6). Both separately
triggered Step 1 obligations were executed:

* **(A) Unconditional duplicate detection**: Executed across `.backlogit/stash.jsonl`,
  `.backlogit/queue/`, and `.backlogit/archive/`. Outcome: **CLEAN DUPLICATE SCAN**
  for `FE245033` (and `D3EFD224`) — no duplicate stash or backlog items exist.
* **(B) Late-identifier reconciliation**: All five source-reference fields on
  `FE245033` were already populated with concrete identifiers at capture time
  (`PR number: 28`, `review-thread ID: PRRT_kwDOTStvas6fpj0Y`,
  `task ID: 009.001-T`, `feature ID: 009-F`, `shipment ID: 009-S`, closure
  artifact `docs/closure/009-S-009-F-post-merge-closure.md`). Reconciliation is
  an idempotent no-op with all concrete source references preserved.

## Requirements and success criteria

* **R1 (Deterministic MP3 byte-for-byte reproducibility and fixed golden reference regression test — `FE245033`)**:
  As established in `009-S` (`docs/closure/009-S-009-F-post-merge-closure.md`
  and `docs/compound/build-errors/shine-mp3-race-checkptr.md`), allocating a
  zero-slack (`cap == len`) stereo buffer in `encodeFrames` triggers a Go
  `-race` `checkptr` fault inside `shine-mp3` (`windowFilterSubband`). Therefore,
  a unit test that runs under `go test -race ./...` MUST NOT call `shine-mp3`
  with a zero-slack (`cap == len`) buffer. Instead, add a deterministic unit
  test in `internal/audio/mp3_test.go` (`TestEncoderDeterministicGoldenReference`)
  that verifies deterministic, byte-for-byte reproducibility against a **fixed
  golden reference** without constructing a zero-slack buffer:
  1. Encoding a deterministic synthesized mono PCM input
     (`sineMono(4 * monoSamplesPerFrame)`, as well as non-frame-aligned chunk
     sizes flushed via `Flush()`) across two independent `NewEncoder` runs
     produces byte-for-byte identical MP3 output (`bytes.Equal`).
  2. The encoded MP3 output for the canonical deterministic synthesized PCM
     input matches a **fixed golden reference**: pinned expected total byte
     length, pinned SHA-256 hex digest (`sha256.Sum256`), and pinned
     leading/trailing MP3 frame bytes, catching any future `shine-mp3` or
     `Encoder` change that alters output bytes while remaining 100% safe under
     `go test -race ./...`.
* **R2 (CI `pull_request` trigger hygiene — `D3EFD224`)**:
  Remove `paths-ignore` from the `pull_request:` trigger in
  `.github/workflows/ci.yml` (retaining `paths-ignore` on `push:` to `main` if
  desired, or relying on the `changes` job) so `Detect code changes` (`changes`)
  always runs and reports on every pull request while `Build, vet, lint & test`
  remains skipped on docs/backlog-only PRs via
  `if: needs.changes.outputs.code == 'true'`. Also include `!.autoharness/**` in
  the `dorny/paths-filter` `code` denylist so `.autoharness/**` matches the
  previous `paths-ignore` exclusion list.

## Options

| Option | Benefit | Limitation |
|---|---|---|
| Option A: Commit a binary `.mp3` golden file in `internal/audio/testdata/` or invoke `shine-mp3` directly with a zero-slack (`cap == len`) buffer | Direct file or live buffer comparison | Binary blob pollutes git history; calling `shine-mp3` with a zero-slack (`cap == len`) buffer triggers a fatal `checkptr` crash under `go test -race ./...` (`009-S`) |
| Option B (chosen): Assert deterministic byte-for-byte reproducibility across independent `NewEncoder` runs AND against a fixed golden reference (pinned byte length, SHA-256 hex digest, and leading/trailing frame bytes) in `internal/audio/mp3_test.go` without constructing a zero-slack buffer, plus remove `pull_request.paths-ignore` in `.github/workflows/ci.yml` | Locks in deterministic byte-for-byte MP3 output under `go test -race ./...` without violating `checkptr` or committing binary blobs; keeps CI `changes` check reporting on all PRs | None |

## Chosen direction

We select **Option B** and synthesize covering feature **`014-F`**
(*Deterministic MP3 golden regression test and CI path-filter trigger hygiene*)
packaged in shipment **`014-S`** (chained after `013-S` via
`backlogit dep add 014-S 013-S --type blocks`) with two width-isolated tasks:

1. **`014.001-T` (U1, code/test domain)**: Add
   `TestEncoderDeterministicGoldenReference` in
   `internal/audio/mp3_test.go` (`XS` / `low`).
2. **`014.002-T` (U2, CI config domain)**: Remove `paths-ignore` from the
   `pull_request` trigger in `.github/workflows/ci.yml` and align the
   `dorny/paths-filter` denylist (`XS` / `trivial`).

## Open questions

None blocking planning or harvest.

## Related artifacts

* [Implementation plan](../plans/2026-11-10-audio-golden-test-and-ci-hygiene-plan.md)
* [009-S post-merge closure](../closure/009-S-009-F-post-merge-closure.md)
* [shine-mp3 -race checkptr learning](../compound/build-errors/shine-mp3-race-checkptr.md)
