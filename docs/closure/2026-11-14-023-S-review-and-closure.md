---
title: Cross-platform CI, POSIX permission verification, and multi-OS release asset parity review and operational closure
date: 2026-11-14
feature: 023-F
shipment: 023-S
branch: feat/023-s-cross-platform-ci-posix-permission-verification-and-multi-os-release-asset-parity
status: closed
closure_status: READY
compaction_status: done
merge_commit: 121271ac016d2f1d1257534e423cc8f859ff27be
pull_request: 58
---

## Release scope

Shipment `023-S` (`023-F`, `023.001-T`..`023.003-T`) closes the three hermetic
`020-S` follow-ups (deliberation `012-DL`, source stash `70ABBB2C`, `D3A9CBD0`,
and `EC6D1CAF`): POSIX permission verification, native Windows CI, and
multi-OS release asset parity.

* `023.001-T` (`internal/capture/output_unix_test.go`, commit `ca5ba6d`, refined
  in `c2cdfb8`):
  * Adds a `//go:build unix` suite that clears the umask with `syscall.Umask(0)`
    and asserts the literal `0o700` directory and `0o600` file modes created by
    `setupOutput` (`Dir` and `FilePath` modes, including the intermediate
    `nested` directory), preservation of a pre-existing `0o750` directory, and
    `AppendAISummaryToFile` mode preservation for both `0o600` and a
    pre-chmodded `0o640` file with `# AI Summary` content verified.
  * `TestPermissionConstantsAreOwnerOnly` pins `dirPerm == 0o700` and
    `filePerm == 0o600` so a loosened constant fails immediately.
* `023.002-T` (`.github/workflows/ci.yml`, `.github/workflows/release.yml`,
  commit `e4109f8`, refined in `c2cdfb8`):
  * Adds the `test-windows` job on `windows-latest` (build, vet,
    `go vet -tags=integration`, `go test -race ./...`) with SHA-pinned actions
    and a `Verify Windows browser E2E does not skip` step that now fails on a
    skip or when no `--- PASS: TestChromeMeetingEndToEnd` line appears.
  * `release.yml` gates the build on the Linux and Windows quality jobs,
    cross-compiles CGO-free binaries for `{linux, darwin, windows} x
    {amd64, arm64}` named by `updater.AssetName`, and verifies all six entries in
    `SHA256SUMS` with a dot-escaped match.
* `023.003-T` (`README.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`,
  `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`, commit
  `8f95435`, refined in `c2cdfb8`):
  * Documents the POSIX mode tests, Windows CI, and six-target releases, states
    that unsupported platforms return `ErrAssetNotFound`, upgrades
    `internal/capture` to Grade A, and records the cross-compiled-only targets as
    a live-verification gap.

## Review

Multi-persona, cross-model adversarial review (`dispatch_mode: multi-agent`) of
`main..1783b6b90c1399478c716baf9b49bcdf68ea2e5b`, followed by GitHub Copilot
shadow review on PR #58 (`P-018` `copilot-review` gate `SATISFIED`).

| Reviewer | Model route | Result |
|---|---|---|
| `rubber-duck` | `gpt-6.1-sol` | PASS (`0 P0, 0 P1`) |
| `Architecture Strategist` (anchor) | `gpt-6.1-sol` | PASS (`0 P0, 0 P1`) |
| `Go Reviewer`, `Security Reviewer`, `Correctness Reviewer`, `Constitution Reviewer`, `Scope Boundary Auditor`, `Schema-CLI-Docs Coupling Reviewer` | harness default | PASS (`0 P0, 0 P1`) |

| Area | Finding | Disposition |
|---|---|---|
| Tests (`rubber-duck`, Correctness, Go) | POSIX assertions compared against `dirPerm`/`filePerm` rather than literals; no intermediate-directory or looser-mode coverage | Fixed in-scope (P-021 C1): literal `0o700`/`0o600` assertions, constants test, `nested` directory assertion, `0o640` preservation test; commit `c2cdfb8` |
| CI (Correctness, Scope Boundary, Constitution) | `no tests to run` could pass the Windows E2E step silently; unescaped `.` in the `SHA256SUMS` match | Fixed in-scope (P-021 C1): require `--- PASS: TestChromeMeetingEndToEnd`; dot-escaped pattern; commit `c2cdfb8` |
| Docs (Schema-CLI-Docs Coupling, Constitution) | Missing `ErrAssetNotFound` note for unpublished platforms; cross-compiled-only targets undocumented | Fixed in-scope (P-021 C1): README, `QUALITY_SCORE.md`, and design doc updated; commit `c2cdfb8` |
| Hygiene (Scope Boundary) | Stale `.backlogit/queue/.*.lock` files | No action required: `**/.*.lock` is gitignored and no lock file is tracked |
| Copilot (review `5906554180`) | `Approval recommended`, no open issues, no inline threads | Nothing to resolve |

Blocking findings: `P0=0, P1=0`. Outcome: `READY`.

### Review record

| Field | Value |
|---|---|
| Reviewed diff identity | `main..c2cdfb8058ef10fd9855a972a148c9861409eb36` (PR #58) |
| Reviewed HEAD | `c2cdfb8058ef10fd9855a972a148c9861409eb36` |
| `dispatch_mode` | `multi-agent` |
| P-018 `copilot-review` gate | `SATISFIED` (exit 0, no unresolved threads); re-run at the last mile with the same result |

## Follow-ups

* No new follow-ups were created.
* Residual limit (tracked in `docs/QUALITY_SCORE.md` under `023-S`):
  `darwin/{amd64,arm64}`, `linux/arm64`, and `windows/arm64` release binaries are
  cross-compiled on `ubuntu-latest` and never run a native test suite.
* Stash `3EB89E2D` and queued live-verification shipment `010-S` remain
  untouched.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #58, merge commit `121271ac016d2f1d1257534e423cc8f859ff27be`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-14T18:58:09Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Close path | P-015 verified fully-covered-root cascade (`023-F` is a root feature and all 3 child tasks `023.001-T`..`023.003-T` are manifest members) |
| Command | `backlogit shipment ship 023-S --sha 121271ac016d2f1d1257534e423cc8f859ff27be` |
| `archived_ids` | `023.001-T`, `023.002-T`, `023.003-T`, `023-F`, `023-S` |
| `returned_ids` | empty |
| `parent_id` preservation | `023.001-T`..`023.003-T` all retained `parent_id: 023-F` |
| Final records | `023-S`: `archived_status: shipped`; `023-F` and `023.001-T`..`023.003-T`: `archived_status: done` |
| Archive integrity (P-007) | No archive file reported as deleted |
| Topology gate | `lifecycle` passed on `main` before the close (see `docs/compound/workflow-issues/topology-gate-closure-branch-naming.md` for the `chore/close-*` branch-name rejection) |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stashes `70ABBB2C`, `D3A9CBD0`, `EC6D1CAF` | Already archived by Stage at harvest; verified absent from the active stash | skipped |
| Deliberation `012-DL` | Retained by the cascade engine; zero live references in `.backlogit/queue/`, status `queued`; archived via `backlogit archive 012-DL` | archived |

### Compaction (P-020)

`compact-context` ran at closure. It wrote
`docs/memory/compacted/2026-11-14-023-S-cross-platform-ci-and-release-parity-compacted.md`
and moved the verbose session memory to
`docs/archive/memory/2026-11-14/stage-023-S-memory.md` and
`docs/archive/memory/2026-11-14/023-S-ship-memory.md`. `compound-refresh` kept all
six existing learnings and added
`docs/compound/workflow-issues/topology-gate-closure-branch-naming.md`.
`compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis | `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `gofmt -l .` (clean) | PASS |
| Linux POSIX tests | WSL Ubuntu 26.04, Go 1.26.0: `go test ./internal/capture -run "Unix\|PermissionConstants" -v` (6 tests) and full `./internal/capture` package | PASS |
| Full test suite | `go test -race ./...` on Windows | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| CI `ubuntu-latest` | `Build, vet, lint & test`, GitHub Actions run `43629074778` (PR #58, HEAD `c2cdfb8`) | PASS |
| CI `windows-latest` | `Build, vet & test (Windows)`, same run; `Verify Windows browser E2E does not skip` logged `--- PASS: TestChromeMeetingEndToEnd (13.31s)` | PASS |
| Backlog integrity | `backlogit sync` (146 artifacts indexed) and `backlogit doctor` ("No issues found.") | PASS |
| Release workflow publication | The six-target matrix and `SHA256SUMS` verification run only on a version tag; not exercised by PR CI | PENDING (first tagged release; see Releasability) |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Add a `windows-latest` CI job and gate the release build on it | low | Third-party actions stay SHA-pinned with `persist-credentials: false` and read-only permissions; the job only builds and tests | applied and verified (run `43629074778`) |
| Expand the release matrix to six CGO-free targets and require six `SHA256SUMS` entries before publishing | medium | Build jobs run only after the quality gates pass; publication fails closed when any entry is missing; the matrix matches `updater.AssetName` | applied; publication path verified at the next tag |
| Cascade-close `023-S` with `backlogit shipment ship` | medium | Classified as a P-015 fully-covered-root close; `returned_ids`, `parent_id`, and the archive-deletion guard verified afterward | applied and verified |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Healthy signal | READY | `go build ./...` and `go test -race ./...` pass locally; both CI jobs green on PR #58 |
| Failure signal | READY | A failing `test-windows` job, a missing `SHA256SUMS` entry (`ERROR: SHA256SUMS is missing an entry for ...`), or a skipped Windows E2E step |
| Monitoring plan | READY | Watch the `release` workflow on the next version tag for six `teams-cli-*` assets plus `SHA256SUMS`; confirm `teams-cli update --check` resolves the matching asset per platform |
| Rollback trigger | READY | A release fails to publish or ships an unusable binary for a supported platform |
| Rollback procedure | READY | Revert merge commit `121271ac016d2f1d1257534e423cc8f859ff27be` on `main`; no runtime code changed |
| Owner | READY | Repository maintainer cutting the next release |
| Validation window | READY | The next tagged release (the first run of the expanded matrix) |

## Dark Mode Execution Summary (P-017)

* **Gates passed**:
  * `pipeline-topology` (`pre_claim`, `post_claim`, `lifecycle`) - exit 0 (the closure-phase gate was run from `main`)
  * Local Review Readiness (`P-014`) - `LOCAL_REVIEW_READY` for HEAD `c2cdfb8058ef10fd9855a972a148c9861409eb36` (`READY`, `0 P0, 0 P1`)
  * Copilot Review Completion Gate (`P-018`) - `SATISFIED` (exit 0), re-run at the last mile
  * Merge Strategy (`P-009`) - merge commit `121271ac016d2f1d1257534e423cc8f859ff27be`; the repository allows only merge commits; `gh pr merge 58 --merge` succeeded without admin fallback (`DARK_MODE_MERGE_AUTHORIZED`)
  * Context Compaction (`P-020`) - `compaction_status: done`

## Handoff

Related [decision](../decisions/2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-deliberation.md)
(`012-DL`), [plan](../plans/2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-plan.md),
and the [quality score](../QUALITY_SCORE.md). Compaction (P-020) completed at
post-merge closure; see the compacted
[memory summary](../memory/compacted/2026-11-14-023-S-cross-platform-ci-and-release-parity-compacted.md).
