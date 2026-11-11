---
type: memory-compacted
date: 2026-11-10
shipment: 014-S
feature: 014-F
source: docs/archive/memory/2026-11-10/014-S-audio-golden-and-ci-hygiene-memory.md
---

# Compacted memory - Shipment 014-S (MP3 golden regression test and CI trigger hygiene)

**Shipped**: PR #35, merge `d245d870b2bfd16beb4da0c581755dad779bfcb5`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship`). Source deliberation `003-DL` was archived by the
explicit source-artifact cleanup step because the engine retains deliberations
that are only mentioned in a feature description.

## Decision and outcome

* `TestEncoderDeterministicGoldenReference` pins the Encoder output for three
  frames of integer-synthesized PCM: 1252 bytes, SHA-256
  `8f48238a04612c14851b93c7cd0e56f29d91609c7090f1af269d8e23d25288b6`, and the
  leading and trailing 16 bytes. Two independent encoder runs must also match
  byte for byte. The test never builds a zero-slack stereo buffer, so it is
  `checkptr`-safe under `-race`.
* `.github/workflows/ci.yml` no longer uses `paths-ignore`, so `Detect code
  changes` reports on every push and pull request. An unconditional
  `Secret hygiene check (tracked files)` step scans `git ls-files -z`. `test`
  stays gated on the `code` filter, and `.autoharness/**` is excluded from it.

## Key reusable insights

* Synthesize golden PCM with integer arithmetic (triangle plus LCG noise), not
  `math.Sin`, so pinned bytes cannot drift with floating-point fusion on other
  architectures.
* `Encoder` exposes `WritePCM` and `Flush` (no `Close`); drive uneven chunk
  sizes through it to cover the sub-frame buffering path.
* A CI secret scan written in Python must mirror the Go guardrail exactly:
  identical synthetic markers (`token`, not `tok`), case-insensitive `p=`, and
  `lstat` skipping of non-regular and oversized files.
* Avoid nested lazy quantifiers in a scan that runs on every PR: extract URL
  candidates once, then check the earliest Teams marker and `p=` values
  separately. A backtracking pattern let one crafted 8 MiB file stall the job.
* Test an extracted workflow `run:` block locally by parsing the YAML and
  running the script against throwaway git repositories, including pathological
  inputs.
* Commit scopes are limited to `cli, browser, auth, transcript, audio,
  scheduler, config`, so workflow changes use `config`.

## Review outcome

Local review: `READY_WITH_FOLLOWUPS`, `P0=0, P1=0`, with one advisory P3
residual-risk note (the CI rules duplicate the Go guardrail rules; keep them in
sync). Copilot shadow review on PR #35 ran three rounds, each raising one thread
on the new CI step, all fixed in scope under P-021 C1 and resolved: marker
alignment (`dc6eaec`), linear-time passcode matching (`cdfc26f`), and
case-insensitive `p=` plus regular-file guard (`af949ad`). The `copilot-review`
gate returned `SATISFIED` and CI passed before the merge commit.

## Post-merge closure summary

P-015 classification returned `CASCADE` (`014-F` is a root whose only
descendants are the two manifest tasks; engine 1.12 `VERIFIED`).
`backlogit shipment ship` returned `archived_ids: [014.001-T, 014.002-T, 014-F,
014-S]` and `returned_ids: []`; both tasks kept `parent_id: 014-F`. Shipment
`010-S` and stash entries `DF1F461F` and `FC763C17` were verified untouched.
