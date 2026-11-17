---
title: Windows-only CI quality gates and 3-target release matrix trimming decision
description: Deliberation on stash entry EE2869C2 for covering feature 032-F and shipment 032-S
topic: Windows-Only CI Quality Gates and 3-Target Release Matrix Trimming (032-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 021-DL
feature: 032-F
shipment: 032-S
source_stash:
  - EE2869C2
linked_artifacts:
  - docs/plans/2026-11-16-windows-ci-and-release-target-trimming-plan.md
  - .backlogit/queue/021-DL.md
  - .backlogit/queue/032-F.md
  - .backlogit/queue/032-S.md
date: 2026-11-16
status: accepted
tags:
  - ci
  - release
  - workflows
  - windows
  - darwin
  - tests
  - docs
---

## Problem Frame

In shipment `023-S` (`012-DL`), `.github/workflows/ci.yml` and `.github/workflows/release.yml` were expanded to run Go quality gates on both `ubuntu-latest` (`test`) and `windows-latest` (`test-windows`) and to cross-compile six release binaries (`{linux, darwin, windows} x {amd64, arm64}`).

Running full Go build, vet, and race-detector test suites twice on every push, pull request, and release tag—plus building and publishing `linux-amd64`, `linux-arm64`, and `darwin-arm64` release binaries—consumes unnecessary GitHub Actions budget for targets the operator does not support.

Stash entry **`EE2869C2`** (`priority: high`, `kind: task`) directs:

> Remove all Linux-related CI/release test jobs and release targets (`linux-amd64`, `linux-arm64`) as well as `darwin-arm64` from `.github/workflows/ci.yml` and `.github/workflows/release.yml` (and aligned docs) so CI/release only runs Windows quality gates (including lint) and only builds/publishes `windows-amd64`, `windows-arm64`, and `darwin-amd64` release assets to conserve GitHub Actions budget.

### P-021 C5 and C6 Triage Record

* `EE2869C2` does not carry the `DEFERRED SCOPE EXPANSION` marker.
* **Obligation (A) Unconditional duplicate detection**: Executed across `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`. Outcome: **CLEAN DUPLICATE SCAN** for `EE2869C2`. Archived shipment `023-S` (`012-DL`) introduced the 6-target matrix and dual-OS test jobs; no other active or queued item trims the CI test jobs and release matrix to `windows-amd64`, `windows-arm64`, and `darwin-amd64`.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (no `N/A` deferred-expansion source refs).

### Scope Boundaries

* **In scope**:
  * **`.github/workflows/ci.yml`**: Remove the duplicate Linux `test` (`Build, vet, lint & test` on `ubuntu-latest`) job and the dead commented-out `docs-lint` block. Consolidate `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`) and coverage reporting (`go test -race -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out`) into `test-windows` (`runs-on: windows-latest`) alongside `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, and the non-skipped `TestChromeMeetingEndToEnd` check.
  * **`.github/workflows/release.yml`**: Remove the duplicate Linux `test` (`Quality Gates` on `ubuntu-latest`) job and the dead commented-out `npm-package` / `npm-publish` blocks. Consolidate `golangci/golangci-lint-action` and coverage reporting into `test-windows` (`Quality Gates (Windows)` on `windows-latest`), update `build.needs` to `[test-windows]`, replace the 6-target `strategy.matrix` with an explicit 3-target `matrix.include` (`windows/amd64`, `windows/arm64`, `darwin/amd64`), and update `release` (`Generate SHA256SUMS`) to assert `test "$(wc -l < SHA256SUMS)" -eq 3` and verify only `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`.
  * **Workflow contract tests (`internal/config/workflow_test.go`)**: Add hermetic Go unit tests using `gopkg.in/yaml.v3` to parse `.github/workflows/ci.yml` and `.github/workflows/release.yml` and assert the absence of the Linux `test` job, the presence of `golangci-lint` and `TestChromeMeetingEndToEnd` in `test-windows`, the exact 3-target `release.yml` build matrix (`windows/amd64`, `windows/arm64`, `darwin/amd64` with zero `linux` or `darwin/arm64` entries), the 3-entry `SHA256SUMS` verification loop, and 40-char SHA pinning + `persist-credentials: false` compliance.
  * **Documentation alignment (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`)**: Update all documentation referencing the 6-target release matrix or dual Linux+Windows CI test jobs to reflect the 3 published release targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`) and the consolidated `windows-latest` (`test-windows`) quality gate job.
  * **Shipment sequencing**: Sequence shipment `032-S` (`032-F`, `021-DL`) after `031-S` and before `010-S` (`031-S -> 032-S -> 010-S`).
* **Out of scope**:
  * Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `7883544E`, and `39450846`, which remain untouched in the stash.
  * Production Go source files (`internal/updater/updater.go`, `internal/cli/update.go`, `internal/capture/output_unix_test.go`). `updater.AssetName(goos, goarch)` remains a pure naming function tested by hermetic unit fixtures, `internal/cli/update.go` is intentionally excluded to keep `032-S` strictly scoped to `.github/workflows/`, `internal/config/workflow_test.go`, and `docs/` per `EE2869C2`, and `internal/capture/output_unix_test.go` remains in the repository (type-checked via `GOOS=darwin GOARCH=amd64 go vet ./...` in `test-windows` and runnable via the cross-compile recipe `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` documented in `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`).

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/build-errors/golangci-lint-go-version-mismatch.md`**: `golangci-lint` v1.64.8 must run with `install-mode: goinstall` on `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` so it compiles with the Go 1.26 toolchain installed by `actions/setup-go`. Moving this step into `test-windows` in `ci.yml` and `release.yml` preserves Principle I (`golangci-lint run` gate) on `windows-latest` while eliminating the separate Linux `test` job.
* **`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`**: POSIX file-mode tests (`//go:build unix`) in `internal/capture/output_unix_test.go` and `internal/config` can still be cross-compiled on Windows (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c`) when local WSL verification is desired, even though CI only runs Windows quality gates.
* **`docs/compound/build-errors/shine-mp3-race-checkptr.md`**: `go test -race ./...` exercises the race detector and `checkptr` on `windows-latest` in `test-windows`.

### Runner Billing and Workflow Analysis

* **GitHub Actions billing multipliers**: GitHub Actions bills `ubuntu-latest` at a **1x** minute multiplier ($0.008/min) with ~3s VM startup, whereas `windows-latest` is billed at a **2x** minute multiplier ($0.016/min) with ~15-30s VM startup.
* **Heavy vs. lightweight jobs**:
  * The heavy Linux `test` job in `ci.yml` and `release.yml` downloads the Go module cache, compiles `golangci-lint` from source, builds all packages, and runs `go test -race ./...`. Removing `test` from `ci.yml` and `release.yml` and removing the 3 unwanted cross-compile matrix legs (`linux-amd64`, `linux-arm64`, `darwin-arm64`) from `release.yml` eliminates all Linux target testing and cuts the release build matrix in half (from 6 jobs to 3 jobs).
  * By contrast, the lightweight host-agnostic utility jobs—`changes` in `ci.yml` (checkout + `dorny/paths-filter` + `python3` secret hygiene check, ~10s) and `changelog` (`git-cliff`), `build` (`CGO_ENABLED=0` cross-compilation of the 3 targets), and `release` (`sha256sum` + `softprops/action-gh-release`) in `release.yml`—run POSIX `bash` / `python3` / `sha256sum` pipelines and do not test or target Linux release artifacts. Keeping those lightweight utility jobs on `ubuntu-latest` avoids paying the 2x `windows-latest` billing multiplier on every docs-only push and release cross-compile.

## Options Evaluated

### Option A: Consolidate Go Quality Gates onto `test-windows` (`windows-latest`), Trim Release Matrix to 3 Targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`), and Add Workflow Contract Tests (Recommended)

* **Description**:
  1. **U1 (`032.001-T` — `config` domain: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `Size: S`, `Complexity: low`, 2 files)**:
     * In `.github/workflows/ci.yml`: remove the `test` (`Build, vet, lint & test` on `ubuntu-latest`) job and the commented-out `docs-lint` block; update `test-windows` (`name: Build, vet, lint & test (Windows)`, `runs-on: windows-latest`) to include the SHA-pinned `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`) step, `go test -race -coverprofile=coverage.out ./...`, and `go tool cover -func=coverage.out`.
     * In `.github/workflows/release.yml`: remove the `test` (`Quality Gates` on `ubuntu-latest`) job and the commented-out `npm-package` and `npm-publish` blocks; update `test-windows` (`Quality Gates (Windows)`, `runs-on: windows-latest`) to include `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`) and `-coverprofile=coverage.out`; update `build.needs` to `[test-windows]`; replace `strategy.matrix` with an explicit `include:` list of the 3 supported targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`); and update `release` (`Generate SHA256SUMS`) to assert `test "$(wc -l < SHA256SUMS)" -eq 3` and verify only `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`.
  2. **U2 (`032.002-T` — `tests` domain: `internal/config/workflow_test.go`, `Size: S`, `Complexity: low`, depends on `032.001-T`, 1 file)**:
     * Add `internal/config/workflow_test.go` parsing `.github/workflows/ci.yml` and `.github/workflows/release.yml` via `gopkg.in/yaml.v3` to assert:
       * Neither `ci.yml` nor `release.yml` defines a duplicate `test` job; both define `test-windows` on `windows-latest` with `golangci/golangci-lint-action` and `TestChromeMeetingEndToEnd`.
       * `release.yml` `build` depends on `test-windows` and defines `strategy.matrix.include` with exactly the 3 supported targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`), with zero `linux` targets and zero `darwin/arm64` target.
       * `release.yml` `release` (`Generate SHA256SUMS`) enforces a 3-line `SHA256SUMS` manifest covering only `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`.
       * Every `uses:` reference in `ci.yml` and `release.yml` is pinned to a 40-char commit SHA and every `actions/checkout` step sets `persist-credentials: false`.
  3. **U3 (`032.003-T` — `docs` domain: `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`, `Size: S`, `Complexity: low`, depends on `032.001-T`, 3 files)**:
     * Align all three documentation files with the 3-target release matrix (`windows/amd64`, `windows/arm64`, `darwin/amd64`) and the consolidated `windows-latest` (`test-windows`) quality gate job.
* **Pros**:
  * Directly fulfills `EE2869C2` by removing the duplicate Linux `test` quality-gate jobs and the `linux-amd64`, `linux-arm64`, and `darwin-arm64` release targets and dead commented blocks.
  * Preserves all required quality gates (`go vet`, `go vet -tags=integration`, `golangci-lint`, `TestChromeMeetingEndToEnd`, and `go test -race -coverprofile=coverage.out ./...`) on `windows-latest`.
  * Maximizes GitHub Actions budget savings by keeping lightweight non-Go utility steps (`changes`, `changelog`, cross-compile `build`, `release`) on `1x`-billed `ubuntu-latest` rather than `2x`-billed `windows-latest`.
  * Enforces the workflow invariants via hermetic Go unit tests in `internal/config/workflow_test.go`.
* **Cons**:
  * `//go:build unix` unit tests (`internal/capture/output_unix_test.go`) no longer run automatically in GitHub Actions CI, though they remain type-checkable and runnable locally via WSL (`GOOS=linux`).
* **Effort estimate**: Low (3 atomic `S`-sized tasks across `config`, `tests`, and `docs`).
* **Fit**: High — exact match with `EE2869C2` and operator intent.

### Option B: Also Migrate Utility Jobs (`changes`, `changelog`, `build`, `release`, `cli-reference-drift.yml`) from `ubuntu-latest` to `windows-latest`

* **Description**: In addition to Option A, switch `runs-on: ubuntu-latest` to `runs-on: windows-latest` on `changes` (`ci.yml` and `cli-reference-drift.yml`), `changelog`, `build`, and `release` (`release.yml`), rewriting the `python3` heredoc and `bash` `sha256sum` / `git-cliff` steps for Windows runners.
* **Pros**: Zero `ubuntu-latest` strings anywhere in `.github/workflows/`.
* **Cons**:
  * **Increases** GitHub Actions budget usage on `changes`, `changelog`, `build` (3 matrix legs), and `release` because `windows-latest` carries a **2x billing multiplier** ($0.016/min vs. $0.008/min) and 2-3x slower VM boot time, directly contradicting the operator's goal to conserve GitHub Actions budget.
  * Adds unnecessary fragility by rewriting working `python3` heredoc and `find -print0 | sort -z | xargs -0 -r sha256sum` scripts for Windows shell environments.
* **Effort estimate**: Medium.
* **Fit**: Low — counterproductive for GitHub Actions budget conservation.

## Trade-off Comparison

| Criterion | Option A (Windows-only quality gates + 3 release targets; keep utility jobs on 1x runner) | Option B (Move all utility jobs to 2x `windows-latest` runner as well) |
|---|---|---|
| Removes duplicate Linux Go `test` jobs (`ci.yml`, `release.yml`) | Yes | Yes |
| Removes `linux-amd64`, `linux-arm64`, and `darwin-arm64` release targets | Yes (only `windows/amd64`, `windows/arm64`, `darwin/amd64` built & published) | Yes |
| GitHub Actions budget savings | Maximum (eliminates Linux Go test runs and 3 matrix legs while keeping utility jobs at 1x rate) | Lower (doubles per-minute billing rate on `changes`, `changelog`, `build`, and `release`) |
| Preserves `golangci-lint` and `TestChromeMeetingEndToEnd` on `windows-latest` | Yes | Yes |
| Risk to secret-hygiene `python3` and `SHA256SUMS` `sha256sum` scripts | None (unchanged) | Moderate (requires shell migration) |

## Decision

We select **Option A**: decompose covering feature **`032-F`** into three atomic, width-isolated tasks (`032.001-T`, `032.002-T`, `032.003-T`) packaged into shipment **`032-S`** and sequenced as `031-S -> 032-S -> 010-S`.

## Rejected Alternatives

* **Option B** was rejected because `windows-latest` has a 2x GitHub Actions billing multiplier and slower startup than `ubuntu-latest`; migrating lightweight host-agnostic utility jobs (`changes`, `changelog`, cross-compile `build`, `release`) to `windows-latest` would waste GitHub Actions budget rather than conserving it.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Risk — Losing `golangci-lint` enforcement or `setup-go` toolchain ordering when removing the Linux `test` job**:
  * **Mitigation**: Move the exact SHA-pinned `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`) step into `test-windows` after `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b` (`go-version-file: go.mod`) in both `.github/workflows/ci.yml` and `.github/workflows/release.yml`, and assert both presence and step ordering in `internal/config/workflow_test.go`. While compiling `golangci-lint` via `goinstall` on `windows-latest` incurs ~2-3 minutes at the 2x Windows billing rate, eliminating the entire separate `ubuntu-latest` `test` job (module download, build, vet, and full `-race` test suite) and 3 release matrix legs still saves net GitHub Actions budget.
* **Risk — Losing compile/vet coverage of `//go:build unix` / `darwin`-tagged code shipped in `go-teams-darwin-amd64`**:
  * **Mitigation**: Include a fast `Vet (darwin/amd64 release target)` step (`GOOS=darwin GOARCH=amd64 go vet ./...`) inside `test-windows` in both `ci.yml` and `release.yml` so `unix`/`darwin`-tagged files are always type-checked and vetted before merge and release, and document the `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` cross-compile recipe from `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`.
* **Risk — Dangling `needs: [test, test-windows]` reference in `.github/workflows/release.yml` or status-check context changes**:
  * **Mitigation**: Update `jobs.build.needs` in `.github/workflows/release.yml` to `[test-windows]` when removing `jobs.test`, verify `jobs.build.needs` in `internal/config/workflow_test.go`, and verify required status-check contexts (`Detect code changes` and `Build, vet, lint & test (Windows)`) during PR CI validation.
* **Risk — `SHA256SUMS` line-count or expected-asset mismatch in `.github/workflows/release.yml`**:
  * **Mitigation**: Update `Generate SHA256SUMS` in `.github/workflows/release.yml` from `test "$(wc -l < SHA256SUMS)" -eq 6` to `-eq 3` and list only `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`, verified by `internal/config/workflow_test.go` (note: `SHA256SUMS` enforces manifest completeness and digest integrity against accidental corruption or missing assets; cryptographic build-provenance attestation remains a separate future consideration).
