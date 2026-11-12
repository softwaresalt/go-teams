---
title: Shipment 018-S execution checkpoint
date: 2026-11-11
status: in-progress
shipment: 018-S
---

## State

* Branch: `feat/018-s-add-inter-process-lock-around-teams-cli-update-binary-swap-rollback-and-old-cleanup`
* HEAD: `e46aa949371c7b414359c84f2ca2ed15634c6af3`
* Shipment, feature, and both tasks are active after the shipment claim
* Intake reconciliation passed; baseline `go vet ./...` passed
* No implementation, harness, review, commit, or push was performed

## Blocker

The harness architect excludes tasks with unfinished dependencies, so
`018.002-T` is not ready while `018.001-T` remains active. Ship requires a
harness for every task before implementation starts. These requirements
prevent progressing without a compliant resolution.

## Next step

Request a workflow or planning resolution before continuing. Preserve the
active shipment and uncommitted backlog changes; do not claim new work.

## Resume 2026-11-11

* Operator selected and confirmed `checkpoint-20261111-230513.json`. Restored
  and resolved it after resuming; `ENGRAM_DEGRADED` fallback used.
* Shipment `018-S` remains active on the branch above. The lifecycle topology
  gate passed. No task is completed and no new commit or push was made.
* Harness delegation produced uncommitted stubs and partial tests in
  `internal/updater/updater.go`, `internal/updater/updater_test.go`, and
  `internal/cli/update_test.go`. `go vet ./...` passed. The requested
  `go test ./internal/updater/... ./internal/cli/...` command failed as
  expected for missing lock implementation, but the delegate reported missing
  acceptance scenarios; a subset of new tests also passed. Step 2 is not
  complete and neither task is labeled harness-ready.
* Next: complete the full named harness coverage, including real paused-update
  CLI contention, locked Windows backup, control-lock contention, release
  cancellation, executable identity and provenance, and transient stat
  re-probe; rerun compilation and red-phase checks before applying labels.
  Preserve the operator's separate `.github/` and `docs/scratch/` changes.

## Resume attempt 2026-11-11, 18:45 PST

* Selected checkpoint `checkpoint-20261112-023654.json` was restored and
  resolved after confirming the active shipment, branch and topology.
  `ENGRAM_DEGRADED` fallback was used. HEAD remains
  `e46aa949371c7b414359c84f2ca2ed15634c6af3`.
* `go vet ./...` passed. Existing harness tests fail, but coverage is incomplete:
  the Go Engineer reported missing real paused CLI update, no-force race,
  concurrent lock and control-lock contenders, cancellation during swap,
  symlink/identity and stat re-probe coverage, and a real locked-old CLI case.
  Some existing tests pass prematurely or stop at the lock stub. No source was
  changed during this attempt; neither task was labeled or completed.
* Blocked at Step 2. Next: complete the deterministic red-phase harnesses for
  both tasks and verify every required scenario before applying labels or
  beginning implementation. Preserve existing unrelated dirty files.

## Resume attempt 2026-11-11, 18:53 PST

* Selected `checkpoint-20261112-025055.json` was validated and restored.
  `ENGRAM_DEGRADED` fallback and the lifecycle topology gate passed. The
  checkpoint remains active because the resumed harness gate is incomplete.
* The red-phase delegate changed `internal/updater/updater.go` and
  `internal/updater/updater_test.go`; `go vet ./...` passed. Harness completion
  failed: several new executable-path cases pass prematurely, and other tests
  fail at the lock stub rather than demonstrating their intended behavior.
* Missing or insufficient coverage includes real paused-update CLI contention,
  dangling-symlink marker/reprobe, actual late-downgrade interleaving, and
  held-lock stale-backup and control-lock contender scenarios. The requested
  red-phase edit boundary excludes the CLI test file needed for its missing
  scenario. No harness-ready labels, task completions, commits, or pushes were
  made in this attempt. Preserve the unrelated `.github/` and scratch changes.
* Next: obtain authorization to edit the CLI harness during the red phase;
  complete deterministic acceptance scenarios and confirm intended red failures
  before labeling tasks or beginning implementation.

## Resume 2026-11-11: green phase blocked

* Selected checkpoint `checkpoint-20261112-025055.json` was restored,
  resumed and resolved. `ENGRAM_DEGRADED`; active shipment `018-S` remains
  on its feature branch at `e46aa949371c7b414359c84f2ca2ed15634c6af3`.
* Harnesses were completed, compilation and expected red failures verified,
  and both `018.001-T` and `018.002-T` labeled `harness-ready`. Both tasks
  remain active. Production and test changes remain uncommitted in
  `internal/updater/updater.go`, `internal/updater/updater_test.go`, and
  `internal/cli/update_test.go`.
* After remediation, `go build ./...`, `go vet ./...`,
  `go vet -tags=integration ./...`, `gofmt -l cmd internal`,
  `go test -race ./...`, and the requested config secret regression passed.
  The pinned linter also passed with `GOTOOLCHAIN=go1.26.0`; using the
  default Go 1.27.4 failed due to incompatible export data.
* The report-only review used `single-agent-declared-degradation` under the
  no-external-transmission restriction and initially returned BLOCKED.
  Remediation added control-lock verification and a real Windows exclusive
  backup-handle test. Its conservative identity fix, however, makes every
  ordinary non-force self-update fail on Windows and macOS. This is a
  blocking normal-usage regression; macOS was not tested. No post-remediation
  readiness approval, task completion, commit, push, or PR was performed.
* Next: establish a safe platform-specific running-image identity approach
  that preserves normal non-force self-updates; add regression tests, rerun
  gates, and complete local review before marking tasks done. Preserve the
  unrelated dirty `.github/` and scratch files.

## Resume 2026-11-11: review blocked on launch identity

* Restored and resolved `checkpoint-20261112-033753.json` with the
  `ENGRAM_DEGRADED` fallback. Shipment `018-S` and both tasks remain active;
  branch HEAD remains `e46aa949371c7b414359c84f2ca2ed15634c6af3`.
* Delegated the requested non-force self-update fix and regression tests;
  local review also found and fixed rejection of real `dev` builds. Both
  fixes were tested red before implementation. Changes remain uncommitted in
  `internal/updater/updater.go`, `internal/updater/updater_test.go`, and
  `internal/cli/update_test.go`.
* After the fixes, `go build ./...`, `go vet ./...`,
  `go vet -tags=integration ./...`, `gofmt -l cmd internal`, the pinned
  golangci-lint v1.64.8 with Go 1.26.0, and `go test -race ./...` passed.
  macOS and Linux runtime tests were not performed in this Windows session.
* Local report-only review is **BLOCKED**: `os.Executable()` may still return
  the original pathname after its running image is replaced. On Windows and
  macOS, the new under-lock pathname snapshot can compare the replacement to
  itself; a same-version replacement passes the version probe. This does not
  establish the task's required launch-image identity. Linux has an additional
  `/proc/self/exe` check. A read-only investigation found no proven stdlib-only
  fix that also preserves normal non-force updates; the scenario was not
  independently reproduced on both platforms.
* Next: obtain an executable design satisfying both the launch-identity
  requirement and the cross-platform availability requirement, implement
  and test it, rerun all gates and review. No task completion, new commit,
  push, or PR was performed. Preserve unrelated `.github/` and scratch edits.

## Resume 2026-11-11: initialization snapshot review blocked

* Restored and resolved `checkpoint-20261112-035633.json` with the
  `ENGRAM_DEGRADED` fallback; lifecycle topology passed. Shipment `018-S`,
  feature `018-F`, and tasks `018.001-T` and `018.002-T` remain active.
  Branch HEAD is `e46aa949371c7b414359c84f2ca2ed15634c6af3`.
* The Go Engineer implemented an initialization-time snapshot and regression
  tests for a synthetic path replaced after construction. Changes remain
  uncommitted in the two updater files and the existing CLI test file.
* `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`,
  `gofmt -l cmd internal`, pinned golangci-lint v1.64.8 with
  `GOTOOLCHAIN=go1.26.0`, and `go test -race ./...` passed on Windows.
  Linux and macOS runtime validation was not performed.
* Targeted report-only review is **BLOCKED** (at least one P1;
  `dispatch_mode: single-agent-declared-degradation`). On Windows and macOS
  the package initializer snapshots a pathname, not the actual launched image:
  the process can be paused after the OS maps image A but before package
  initialization, while another updater replaces the pathname with B. The
  initializer then captures B and the same-version check accepts B as the
  running image. The synthetic construction test does not cover this window.
  The full multi-persona review remains incomplete; no readiness claim is made.
* Next: establish a verified platform-specific running-image identity or
  fail-closed design that meets the task contract, then rerun the gates and
  complete review. No task completion, new commit, push, or PR was performed.
  Preserve the unrelated `.github/` and scratch edits.

## Resume 2026-11-11: local gates passed, delivery blocked

* Selected and read `checkpoint-20261112-042445.json`; its Ship ownership,
  active shipment, branch, and lifecycle topology were verified.
  `ENGRAM_DEGRADED` recovery used the checkpoint and this memory.
* Passed `go build ./...`, `go vet ./...`,
  `go vet -tags=integration ./...`, `gofmt -l cmd internal`, pinned
  golangci-lint v1.64.8 with Go 1.26.0, `go test -race ./...`, and the
  tracked-secrets regression. The full local review is not complete.
* External delivery is not available in this environment. The checkpoint
  remains active for handoff; no tasks were completed, no commits or PRs were
  created, and no files were deleted. Preserve the unrelated dirty files.

## Resume 2026-11-11: local review blocked on overlapping paths

* Selected `checkpoint-20261112-042445.json` was read; `ENGRAM_DEGRADED`
  recovery and the lifecycle topology gate passed. The checkpoint remains
  active because the resumed work has not completed.
* Local review is BLOCKED: an updater targeting a legitimate `foo.old` and
  one targeting `foo` use different locks, though `foo.old` is also `foo`'s
  rollback path. The sibling-lock check is only before download, so a `foo`
  update can start after it; a forced `foo.old` swap can then move the
  `foo` rollback copy while `foo` is installing or rolling back. The
  sibling-lock test also fails on mismatched launch provenance before it
  exercises the sibling-lock check.
* No fresh complete multi-persona review, task completion, commit, push or PR
  was performed. Next: resolve the overlapping-path coordination design and
  regression coverage, then rerun the gates and review. Preserve unrelated
  `.github/` and scratch changes.

## Resume 2026-11-11: overlap regression coverage incomplete

* Restored and resolved the operator-selected
  `checkpoint-20261112-042445.json` using the `ENGRAM_DEGRADED` path.
  Shipment `018-S` remains active on its feature branch; HEAD remains
  `e46aa949371c7b414359c84f2ca2ed15634c6af3`.
* The Go Engineer added ordered prefix-chain lock acquisition and tests in
  the two updater files. The overlap test pauses the `.old` installation
  and blocks the base installation; it does not exercise the requested
  inverse interleaving with the base installation paused inside `swap`.
* `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`,
  `gofmt -l cmd internal`, the pinned linter using the cached Go 1.26
  toolchain and local module cache, and `go test -race ./...` passed.
  No complete local review or readiness verdict was produced.
* Next: add and verify the requested base-holder overlap regression, then
  rerun the gates and complete review. Tasks remain active; no commits,
  push, or PR were made. Preserve unrelated working-tree changes.

## Implementation complete 2026-11-11

* Added `TestUpdateOverlappingBaseHeldBlocksOldInstallation`: the base swap
  pauses inside `swap` while an `.old` contender waits on the lock chain, then
  both complete in order. The base paused swap removes the stale `.old` and
  moves the executable to `.old`, so the contender assertions check the
  `.old` path still holds the base backup and no `.old.old` appears.
* `TestSwapOldChainReleasesAcquiredLocksOnCancellation` failed once in the
  full race suite because its 200ms timeout fired before the second lock was
  taken. It now cancels only after both lock markers exist; 20 consecutive
  `-race` runs passed.
* All gates passed: build, vet, integration vet, gofmt, pinned lint, the
  full race suite, and the tracked-secrets regression.
* Committed `018.001-T` as `290b649` and `018.002-T` as `009c23f`; both
  tasks and `018-F` are `done`. In-session review is READY (P0=0, P1=0) and
  is recorded in `docs/closure/2026-11-11-018-S-review-and-closure.md`.
* Next: commit the closure and backlog records, push, open the PR, and run
  the Copilot and CI gates. Keep `.github/` and `docs/scratch/` edits out of
  every `018-S` commit; stash them before checkout in post-merge closure.
