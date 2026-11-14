---
type: memory-compacted
date: 2026-11-14
shipment: 023-S
feature: 023-F
source:
  - docs/archive/memory/2026-11-14/stage-023-S-memory.md
  - docs/archive/memory/2026-11-14/023-S-ship-memory.md
---

# Compacted memory - Shipment 023-S (Cross-platform CI, POSIX permission verification, multi-OS release parity)

**Shipped**: PR #58, merge `121271ac016d2f1d1257534e423cc8f859ff27be`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship 023-S --sha 121271ac016d2f1d1257534e423cc8f859ff27be`);
deliberation `012-DL` archived via `backlogit archive 012-DL`.

## Decision and outcome

* `internal/capture/output_unix_test.go` (`//go:build unix`): umask-cleared tests
  pin `0o700` directories and `0o600` files, the intermediate directory, a
  pre-existing `0o750` directory, and `AppendAISummaryToFile` preserving `0o600`
  and `0o640`. A constants test pins `dirPerm`/`filePerm`.
* `ci.yml`: new `test-windows` job on `windows-latest` with a non-skipping
  `TestChromeMeetingEndToEnd` check that also requires a `--- PASS` line.
* `release.yml`: build gated on Linux and Windows quality jobs; six CGO-free
  targets (`{linux, darwin, windows} x {amd64, arm64}`) matching
  `updater.AssetName`; `SHA256SUMS` must list all six (dot-escaped match).
* Docs: README, `ARCHITECTURE.md`, `QUALITY_SCORE.md` (`internal/capture` is now
  Grade A), and the self-update design doc record coverage, the
  `ErrAssetNotFound` behavior for unpublished platforms, and the cross-compiled
  only targets (`darwin/*`, `linux/arm64`, `windows/arm64`).

## Process notes

* Eight-reviewer pre-PR review (0 P0, 0 P1); all in-scope P2/P3 items landed in
  one new commit `c2cdfb8` rather than amending task commits `ca5ba6d`,
  `e4109f8`, `8f95435`.
* Copilot reviewed the pushed HEAD automatically (ruleset `review_on_push`): no
  comments. `autoharness gate copilot-review` returned `SATISFIED`.
* Both CI jobs passed on run `43629074778`; the Windows E2E logged
  `--- PASS: TestChromeMeetingEndToEnd`.
* The topology gate rejected the `chore/close-023-S` branch name
  (`BRANCH_MISMATCH`); run the `lifecycle` gate from `main` first. See
  `docs/compound/workflow-issues/topology-gate-closure-branch-naming.md`.
* The cascade engine retained `012-DL`; Ship archived it explicitly after
  confirming zero live references.

## Open items

* The first tagged release exercises the expanded release matrix for real.
* `3EB89E2D` and queued shipment `010-S` (live-tenant verification) are
  untouched.

Full closure record: `docs/closure/2026-11-14-023-S-review-and-closure.md`.
