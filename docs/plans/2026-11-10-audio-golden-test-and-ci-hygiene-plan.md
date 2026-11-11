---
title: Deterministic MP3 golden regression test and CI path-filter trigger hygiene plan
description: Implementation plan for 014-F / 014-S covering a deterministic fixed-reference MP3 byte-equality test in internal/audio/mp3_test.go and pull_request trigger path-filter hygiene in .github/workflows/ci.yml
source: docs/decisions/2026-11-10-audio-golden-test-and-ci-hygiene-deliberation.md
deliberation_id: 003-DL
feature: 014-F
shipment: 014-S
date: 2026-11-10
status: approved
tags:
  - audio
  - testing
  - ci
  - workflows
---

## Problem Frame

Following the deliberation in
[2026-11-10-audio-golden-test-and-ci-hygiene-deliberation.md](../decisions/2026-11-10-audio-golden-test-and-ci-hygiene-deliberation.md)
(`003-DL`), two maintenance items (`FE245033` and `D3EFD224`) are planned under
feature **`014-F`** and shipment **`014-S`**:

1. **`FE245033`**: `internal/audio/mp3.go` (`encodeFrames`) allocates
   `stereo := make([]int16, len(mono)*2, len(mono)*2+encoderTailSlack)`
   (`encoderTailSlack = 4`) so `shine-mp3` v0.1.0's end-of-buffer pointer stays
   within the allocation under `go test -race`. Adding a deterministic
   byte-for-byte and fixed SHA-256 reference test in
   `internal/audio/mp3_test.go` turns the source-level byte-identity argument
   recorded in `docs/closure/009-S-009-F-post-merge-closure.md` into an
   automated test guarantee.
2. **`D3EFD224`**: `.github/workflows/ci.yml` currently specifies `paths-ignore`
   on the `pull_request` trigger, preventing the `Detect code changes`
   (`changes`) job from running and reporting a status check on
   documentation- or backlog-only PRs. Removing `paths-ignore` from
   `pull_request:` allows `changes` to run and report on every PR while `Build,
   vet, lint & test` remains gated by `if: needs.changes.outputs.code == 'true'`.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Deterministic byte-for-byte and fixed SHA-256 reference regression test for `internal/audio` MP3 output | `FE245033` | U1 (`014.001-T`) | `go test -race ./internal/audio/...` (`TestEncoderOutputByteIdenticalToZeroSlackAndGoldenHash`) |
| R2: Remove `paths-ignore` from `pull_request` trigger in `.github/workflows/ci.yml` while keeping `test` job self-gated via `needs.changes.outputs.code == 'true'` | `D3EFD224` | U2 (`014.002-T`) | YAML structural verification and GitHub Actions `Detect code changes` check on PR |

## Implementation Units

### U1 (`014.001-T`) — Add deterministic fixed-reference MP3 byte-equality regression test in `internal/audio/mp3_test.go`

* **Domain:** tests (Go). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:** Add `TestEncoderOutputByteIdenticalToZeroSlackAndGoldenHash` in
  `internal/audio/mp3_test.go` that:
  1. Generates a deterministic mono PCM input via `sineMono(4 * monoSamplesPerFrame)`
     and encodes it via `NewEncoder(&buf, SampleRate)`.
  2. Encodes the identical interleaved dual-mono samples when the spare tail
     capacity (`stereo[len(stereo):cap(stereo)]`) is filled with non-zero
     sentinel values (`0x7FFF`, `-0x8000`) and asserts `bytes.Equal` with the
     standard `Encoder` output, proving that `encoderTailSlack` capacity bytes
     never influence the encoded MP3 stream.
  3. Asserts the encoded MP3 byte length and `sha256.Sum256` hex digest against
     a pinned constant reference so any future `shine-mp3` or `Encoder` change
     that alters output bytes is caught deterministically.
* **Files (1 file):** `internal/audio/mp3_test.go`.
* **Verify:** `go test -race ./internal/audio/...` passes.
* **Depends on:** none.

### U2 (`014.002-T`) — Remove `paths-ignore` from `pull_request` trigger in `.github/workflows/ci.yml`

* **Domain:** config (CI workflow). **Posture:** migration-first / config hygiene. **Size:** `XS` | **Complexity:** `trivial`.
* **What:**
  1. In `.github/workflows/ci.yml`, replace the `pull_request: paths-ignore: [...]`
     block with an unconditional `pull_request:` trigger so the lightweight
     `Detect code changes` (`changes`) job runs and reports on every PR.
  2. Add `'!.autoharness/**'` to the `dorny/paths-filter` `code` filter block in
     the `changes` job so `.autoharness/**`-only PR diffs are excluded from
     triggering `Build, vet, lint & test`, matching the `push` trigger's
     `paths-ignore` list.
  3. Preserve all SHA-pinned action references (`actions/checkout@11bd7190...`,
     `dorny/paths-filter@d1c1ffe0...`), `persist-credentials: false`, and
     least-privilege `permissions` per `ci-security.instructions.md`.
* **Files (1 file):** `.github/workflows/ci.yml`.
* **Verify:** Validate YAML syntax and confirm `changes` and `test` jobs pass on
  the shipment PR.
* **Depends on:** none.

## Dependency Graph

```text
U1 (014.001-T) (independent — tests domain)
U2 (014.002-T) (independent — CI config domain)
```

* `014.001-T` and `014.002-T` touch disjoint domains and files.
* Shipment `014-S` is sequenced after `013-S` via
  `backlogit dep add 014-S 013-S --type blocks`.

## Decisions and Rationale

* **Width isolation between U1 and U2:** Keeping the Go audio test (`014.001-T`)
  and the GitHub Actions workflow update (`014.002-T`) in separate tasks
  preserves single-domain width isolation (`tests` vs `config`).
* **Poisoned tail-capacity + SHA-256 golden assertion:** Testing both poisoned
  spare capacity (`stereo[:len]` with non-zero bytes in `stereo[len:cap]`) and a
  pinned SHA-256 digest directly proves that `encoderTailSlack` cannot affect
  encoded output and locks in deterministic byte-for-byte behavior under
  `go test -race`.

## Risks and Caveats

* **Cross-platform floating-point determinism in `sineMono`:** `sineMono` uses
  `math.Sin` in Go's standard library (pure Go implementation on `amd64`) and
  truncates to `int16`; the resulting `[]int16` slice and `shine-mp3` integer
  subband filterbank are deterministic across Windows and Linux `amd64`.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library `crypto/sha256` and `bytes` only; zero `golangci-lint` and `go vet` findings |
| II: Test first | `014.001-T` adds deterministic byte-equality and golden-digest assertions in `internal/audio/mp3_test.go` |
| III, IV: Secrets & containment | No secrets or external paths; CI workflow preserves `persist-credentials: false` and `contents: read` |
| VI: Single responsibility | Two `XS` single-file tasks isolated by domain (`tests` vs `config`) |
| VII, VIII: Strict safety | CI workflow trigger change classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `003-DL`, plan, feature `014-F`, tasks `014.001-T`–`014.002-T`, and shipment `014-S` (`blocks 013-S`) recorded |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **absent**.
* Security, auth, permission, or compliance-sensitive behavior — **present
  (minor CI workflow surface)**: U2 edits `.github/workflows/ci.yml` governed by
  `ci-security.instructions.md`.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**.
* External integration, operator checkpoint, or external dependency — **absent**.
* High runtime, rollout, or rollback risk — **absent**.

Requires plan hardening: yes

## Plan Hardening

Hardening is included because U2 modifies `.github/workflows/ci.yml`.

* **Instructions and learnings consulted:**
  `.github/instructions/ci-security.instructions.md`,
  `.github/instructions/workflows.instructions.md`,
  `docs/compound/build-errors/shine-mp3-race-checkptr.md`,
  `docs/compound/build-errors/shine-mp3-frame-alignment.md`, and
  `docs/closure/009-S-009-F-post-merge-closure.md`.

### Protected Invariants

1. All GitHub Actions in `.github/workflows/ci.yml` remain pinned to full commit
   SHAs with `persist-credentials: false` and `permissions: contents: read`.
2. `encoderTailSlack = 4` and `monoSamplesPerFrame = 1152` in
   `internal/audio/mp3.go` remain unchanged, and `go test -race ./internal/audio/...`
   must pass without `checkptr` faults.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Remove `paths-ignore` from `pull_request` trigger in `.github/workflows/ci.yml` and add `'!.autoharness/**'` to `dorny/paths-filter` | low | Preserves SHA pinning, `persist-credentials: false`, and `if: needs.changes.outputs.code == 'true'` on the `test` job | `Detect code changes` runs and reports on all PRs while `Build, vet, lint & test` still skips docs/backlog-only PRs |

## Runtime Verification and Closure

* **Verification:** `go test -race ./internal/audio/...`, `go vet ./...`,
  CI-pinned `golangci-lint run`, `gofmt -l cmd internal`, and PR CI `changes` +
  `test` job verification.
* **Rollback:** Revert the `014-S` merge commit on `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012):** `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).
* **Constitution Reviewer (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Go Reviewer (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Scope Boundary Auditor (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Both units are single-file `XS` tasks strictly bounded to `FE245033` and
  `D3EFD224`.
* **Learnings Researcher (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Grounded in `shine-mp3-race-checkptr.md` and `009-S-009-F-post-merge-closure.md`.
* **Architecture Strategist (cross-model rubric):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
* **Security Lens Reviewer (CI workflow surface):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Complies with `ci-security.instructions.md` (full-SHA action pinning,
  `persist-credentials: false`, least-privilege `permissions`).

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
`014-F` and shipment `014-S`.
