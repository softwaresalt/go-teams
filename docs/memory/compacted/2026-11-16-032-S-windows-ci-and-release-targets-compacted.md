---
title: Compacted memory for shipment 032-S Windows-only CI quality gates and 3-target release matrix trimming
date: 2026-11-16
feature: 032-F
shipment: 032-S
deliberation: 021-DL
source_stash: EE2869C2
pull_request: 78
merge_commit: 80bf806112a0c887c5216a4be39bba888ca1f859
closure_status: READY
status: compacted
---

## Decisions and Scope

* **Windows-only CI quality gates and 3-target release matrix (`032-F`, `021-DL`, stash `EE2869C2`)**:
  * Consolidated Go build, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint` (`v1.64.8`, `install-mode: goinstall`, `--timeout=5m`), non-skipped `TestChromeMeetingEndToEnd`, and `go test -race "-coverprofile=coverage.out" ./...` (plus `go tool cover "-func=coverage.out"` in `ci.yml`) onto `test-windows` (`windows-latest`) in `.github/workflows/ci.yml` and `.github/workflows/release.yml`, removing the duplicate Linux `test` jobs and dead commented-out blocks (`docs-lint`, `npm-package`, `npm-publish`).
  * Retained lightweight host-agnostic utility jobs (`changes` in `ci.yml`, and `changelog`, `build`, `release` in `release.yml`) on 1x-billed `ubuntu-latest`, trimmed `release.yml` `build` to `matrix.include` (`windows/amd64`, `windows/arm64`, `darwin/amd64`), and updated `Generate SHA256SUMS` to assert `test "$(wc -l < SHA256SUMS)" -eq 3`.
  * Added hermetic YAML workflow contract tests in `internal/config/workflow_test.go` and aligned `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`.
* **Post-merge closure (`PR #78`, merge `80bf806112a0c887c5216a4be39bba888ca1f859`)**: Archived `021-DL` first and executed `autoharness shipment cascade-close` (`classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`). Deferred out-of-scope items captured under P-021 C2: `4AB8EAEE` (Copilot thread `PRRT_kwDOTStvas64Pwye` — native POSIX/macOS permission-test gate evaluation), `59545A24` (`actions/upload-artifact` `retention-days` in `release.yml`), and `C3460FDC` (pre-existing NUL-byte / `8 MiB` `MAX_BYTES` secret-scan heuristic in `ci.yml` `jobs.changes` / `gitignore_test.go`).

## Delivered Artifacts

* `032.001-T` (`058e4ed`, `009c005`): `.github/workflows/ci.yml`, `.github/workflows/release.yml`.
* `032.002-T` (`911af2b`, `009c005`): `internal/config/workflow_test.go`.
* `032.003-T` (`f88ce82`, `009c005`): `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`.
* Post-merge closure & compound learning: `docs/closure/032-S-032-F-post-merge-closure.md`, `docs/closure/evidence/032-S-032-F-close-evidence.json`, `docs/compound/workflow-issues/pwsh-dotted-go-flag-tokenization.md`.

## Key Learnings

* On `windows-latest` (`pwsh`), unquoted `-coverprofile=coverage.out` and `-func=coverage.out` split at `.` into two arguments; double-quoting `"-coverprofile=coverage.out"` and `"-func=coverage.out"` works cleanly on both `pwsh` and `bash`.
* Local `golangci-lint` (`v2.13.2` built with Go `1.27.0`) cannot decode Go `1.27.4` stdlib export data locally, making CI's `install-mode: goinstall` (`golangci-lint v1.64.8`) on `test-windows` the authoritative lint gate.

## Archived Source Memories

* `docs/archive/memory/2026-11-16/stage-032-s-windows-ci-and-release-targets-memory.md`
* `docs/archive/memory/2026-11-16/ship-032-s-windows-ci-and-release-targets-memory.md`
