---
title: Windows-only CI quality gates and 3-target release matrix trimming operational closure
date: 2026-11-16
feature: 032-F
shipment: 032-S
branch: feat/032-s-windows-only-ci-quality-gates-and-3-target-release-matrix-trimming
status: closed
closure_status: READY
compaction_status: done
merge_commit: 80bf806112a0c887c5216a4be39bba888ca1f859
pull_request: 78
close_path: cascade
close_evidence: docs/closure/evidence/032-S-032-F-close-evidence.json
---

## Release scope

Shipment `032-S` (`032-F`, `032.001-T`..`032.003-T`) delivers deliberation
`021-DL` (source stash `EE2869C2`), consolidating CI and release Go quality
gates onto `test-windows` (`windows-latest`) and trimming the published release
matrix to `windows/amd64`, `windows/arm64`, and `darwin/amd64`:

* `032.001-T` (`.github/workflows/ci.yml`, `.github/workflows/release.yml`,
  commits `058e4ed`, `009c005`):
  * Removed duplicate Linux Go quality-gate `test` jobs (`ubuntu-latest`) and
    dead commented-out blocks (`docs-lint`, `npm-package`, `npm-publish`) from
    both workflows while keeping lightweight host-agnostic utility jobs
    (`changes`, `changelog`, `build`, `release`) on `1x`-billed
    `ubuntu-latest`.
  * Consolidated `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`,
    `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`,
    `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837`
    (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`),
    non-skipped `TestChromeMeetingEndToEnd`, and
    `go test -race "-coverprofile=coverage.out" ./...` (plus
    `go tool cover "-func=coverage.out"` in `ci.yml`) into `test-windows`
    (`runs-on: windows-latest`).
  * Updated `jobs.build.needs` in `.github/workflows/release.yml` to
    `[test-windows]`, replaced the 6-target matrix with an explicit 3-target
    `matrix.include` (`windows/amd64`, `windows/arm64`, `darwin/amd64`), and
    updated `Generate SHA256SUMS` to enforce `test "$(wc -l < SHA256SUMS)" -eq 3`
    across `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and
    `go-teams-windows-arm64.exe`.
* `032.002-T` (`internal/config/workflow_test.go`, commits `911af2b`, `009c005`):
  * Added hermetic YAML contract tests (`TestCIWorkflowRunsWindowsQualityGatesOnly`,
    `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly`, and
    `TestWorkflowsPinThirdPartyActionsAndDisableCredentials`) using
    `gopkg.in/yaml.v3` (with custom `workflowNeeds` Unmarshaler for scalar and
    sequence `needs:`), verifying step ordering (`setup-go` before
    `golangci-lint-action`), unconditional `If == ""` and `!ContinueOnError` on
    `changes` secret scanning and all `test-windows` quality-gate steps, the
    3-target matrix and `SHA256SUMS` checks, least-privilege `permissions`,
    40-char SHA pinning, and `persist-credentials: false`.
* `032.003-T` (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`,
  `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`,
  commits `f88ce82`, `009c005`):
  * Updated user guide, quality score, and self-update design documentation to
    reflect the 3 published release targets (`windows/amd64`, `windows/arm64`,
    `darwin/amd64`), consolidated `test-windows` CI quality gates, and the
    `//go:build unix` CI lint/execution boundary (`GOOS=darwin GOARCH=amd64 go vet ./...`
    in CI and `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` locally).

## Review

Two rounds of `multi-agent` Adversarial Review (`openai/gpt-6.1-sol` anchor +
`claude-haiku-5.5` + `gemini-4-argon`, `3/3` full pool) plus 1 GitHub Copilot
shadow-review round on PR #78 (`P-018` `copilot-review` gate `SATISFIED` on
HEAD `009c0056bc3b9ff2e4f0cb47e36c5eae78effa85`):

| Area | Finding | Disposition |
|---|---|---|
| Adversarial Review Round 1 (`f88ce82`) | Anchor `P1` (PowerShell splitting unquoted `-coverprofile=coverage.out` / `-func=coverage.out` at `.` on `windows-latest`), Reviewer-A `P1`/`P2` (`//go:build unix` lint boundary note, `If == ""` / `!ContinueOnError` assertions, stale `test` comment, `hasBrowserE2E` skip/exit checks) | Fixed in-scope (P-021 C1) in `009c005`; `actions/upload-artifact` `retention-days` deferred out-of-scope under P-021 C2 as `59545A24` |
| Adversarial Review Round 2 (`009c005`) | `0` P0, `0` HIGH/MEDIUM consensus/majority findings; `LOW` Unique `U1-R2` / `U2-R2` (pre-existing `jobs.changes` NUL-byte / `8 MiB` `MAX_BYTES` secret-scan heuristic) and `LOW` `P2` advisories `U3-R2`, `U4-R2` (`READY_WITH_FOLLOWUPS`) | `U1-R2` and `U2-R2` deferred out-of-scope under P-021 C2 as `C3460FDC`; `U3-R2` and `U4-R2` noted as residual `LOW` advisories |
| Copilot PR #78 Round 1 (`PRRT_kwDOTStvas64Pwye`, comment `4594638631`) | Keep a small native POSIX permission-test gate in CI and `release.yml` for `internal/capture/output_unix_test.go` and `internal/config/template_test.go` | Classified out-of-scope per P-021 C1 (`EE2869C2` / `021-DL` remove all non-Windows Go test jobs); captured under P-021 C2 as `4AB8EAEE`, replied citing `4AB8EAEE`, and resolved via GraphQL |

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #78, merge commit `80bf806112a0c887c5216a4be39bba888ca1f859`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-17T02:49:24Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 032-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/032-s-windows-only-ci-quality-gates-and-3-target-release-matrix-trimming` |
| Close path | `cascade` (`autoharness shipment cascade-close --shipment 032-S --feature 032-F --sha 80bf806112a0c887c5216a4be39bba888ca1f859 --timeout 600 --caller-wait 1200` returned `exit_code: 0`, `classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`) |
| Close evidence | `docs/closure/evidence/032-S-032-F-close-evidence.json` |
| `archived_ids` | `032-F`, `032-S`, `032.001-T`, `032.002-T`, `032.003-T` (equal to `required_ids`; `unexpected_archived: []`) |
| `returned_ids` | empty |
| `parent_id` preservation | `032.001-T`..`032.003-T` retained `parent_id: 032-F` (`parent_id_preserved: true`) |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup and follow-up stash state

| Artifact | Disposition | Result |
|---|---|---|
| Stash `EE2869C2` | Already archived by Stage at harvest; absent from active stash | skipped |
| Deliberation `021-DL` | Archived via `backlogit archive 021-DL` prior to `cascade-close`; resolved to `already-archived` in `pre_close.linked_deliberation_disposition` (`docs/closure/evidence/032-S-032-F-close-evidence.json`) | archived (`.backlogit/archive/021-DL.md`) |
| Stash `4AB8EAEE` | Captured from PR #78 Copilot thread `PRRT_kwDOTStvas64Pwye` (evaluate targeted POSIX/macOS permission-test gate in `ci.yml`/`release.yml`, P-021 C2) | retained active for Stage triage |
| Stash `59545A24` | Captured during Round 1 (`actions/upload-artifact` `retention-days` in `release.yml`, P-021 C2) | retained active for Stage triage |
| Stash `C3460FDC` | Captured during Round 2 (`U1-R2`/`U2-R2` pre-existing `jobs.changes` secret-scan heuristics, P-021 C2) | retained active for Stage triage |
| Pre-existing active stash entries | `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E` | preserved untouched in `.backlogit/stash.jsonl` |

### Knowledge graduation, compaction (P-020), and compound refresh

Graduated documentation updates in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`,
and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`
(`032.003-T`), and added compound learning
`docs/compound/workflow-issues/pwsh-dotted-go-flag-tokenization.md`.
`compact-context` consolidated `stage-032-s-windows-ci-and-release-targets-memory.md`
and `ship-032-s-windows-ci-and-release-targets-memory.md` into
`docs/memory/compacted/2026-11-16-032-S-windows-ci-and-release-targets-compacted.md`
and archived the source memory files to `docs/archive/memory/2026-11-16/`.
`compound-refresh` confirmed `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`
and `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md` remain
accurate (`keep`). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Build, vet, fmt | `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `gofmt -l cmd internal` (0 files) | PASS |
| Unit & E2E suite | `go test -race ./...` across all packages including `TestCIWorkflowRunsWindowsQualityGatesOnly`, `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly`, `TestWorkflowsPinThirdPartyActionsAndDisableCredentials`, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| CI `windows-latest` | `Build, vet, lint & test (Windows)` (runs `golangci-lint v1.64.8` via `goinstall`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `TestChromeMeetingEndToEnd`, `go test -race "-coverprofile=coverage.out" ./...`, and `go tool cover "-func=coverage.out"`), GitHub Actions run `44005115860` (`3m22s`, PR #78, HEAD `009c005`) | PASS |
| CI `ubuntu-latest` utility checks | `Detect code changes` (includes secret hygiene scan, `9s`) and `Detect CLI-affecting changes` (`4s`), PR #78, HEAD `009c005` | PASS |
| Runtime verification probes | `go run ./cmd/go-teams --help`, `go run ./cmd/go-teams schedule --help` | PASS |
| Backlog & closure gate | `autoharness gate closure-evidence`, `backlogit sync`, and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| `PA-032-1`: Remove Linux `test` jobs and dead commented-out blocks from `.github/workflows/ci.yml` and `.github/workflows/release.yml`, consolidate `golangci-lint`, `GOOS=darwin GOARCH=amd64 go vet ./...`, and coverage onto `test-windows`, and trim `release.yml` `build` and `SHA256SUMS` to `windows/amd64`, `windows/arm64`, and `darwin/amd64` | moderate | Pre-authorized under Autopilot Mode; verified by `workflow_test.go` and PR #78 CI `windows-latest` run `44005115860` | applied |
| `PA-032-2`: Add hermetic YAML workflow contract tests in `internal/config/workflow_test.go` | low | Pre-authorized under Autopilot Mode; verified in `go test -race ./...` and CI | applied |
| `PA-032-3`: Align `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` | low | Pre-authorized under Autopilot Mode; zero secret leaks verified | applied |

## Releasability evidence

* **Invariants preserved**: All external actions pinned to 40-char SHAs with `persist-credentials: false`; top-level `permissions: contents: read` preserved (`contents: write` scoped exclusively to `release.yml` `jobs.release`); unconditional tracked-file secret hygiene scan preserved in `ci.yml` `jobs.changes`; complete `test-windows` quality gate enforced in both workflows; exact 3-target `release.yml` build matrix and `SHA256SUMS` (`wc -l == 3`) verification enforced; zero production Go code or unrelated stash entries mutated
* **Rollout and rollback**: Workflows, contract tests, docs, compound learning, and closure artifacts on `main`; rollback via `git revert -m 1 80bf806112a0c887c5216a4be39bba888ca1f859`
* **Monitoring and validation window (7 days, owner: `softwaresalt`)**:
  * **Healthy signals**: 100% pass rate on `CI` (`Detect code changes`, `Build, vet, lint & test (Windows)`) and `go test -race ./...`, zero linter issues, zero secret-hygiene findings, and clean 3-asset `SHA256SUMS` release publication on tag push
  * **Failure signals and rollback triggers**: Any `>= 1` workflow job failure in `test-windows` (`ci.yml` or `release.yml`) or `build` / `release` (`SHA256SUMS` check)
* **Readiness verdict**: `READY` (all local quality gates, unit/E2E suites, 2 rounds of `multi-agent` Adversarial Review, 1 Copilot shadow-review round, PR #78 CI checks, and `cascade-close` postconditions passed)

## Autopilot Execution Summary

* **Resolved route**: `gemini-4-hydrogen` / `google` / `high`
* **Gates passed**:
  * `pipeline-topology` (`pre_claim`, `post_claim`, `lifecycle` feature + `lifecycle` closure) -- `exit_code: 0`
  * Local Review Readiness (`P-014`) -- `LOCAL_REVIEW_READY` for HEAD `009c0056bc3b9ff2e4f0cb47e36c5eae78effa85` (`READY_WITH_FOLLOWUPS`, `0` P0, `0` P1 consensus/majority)
  * Copilot Review Completion Gate (`P-018`) -- `SATISFIED` (`exit_code: 0`, thread `PRRT_kwDOTStvas64Pwye` deferred under P-021 C2 as `4AB8EAEE`, replied, and resolved)
  * Merge Strategy (`P-009`) -- merge commit `80bf806112a0c887c5216a4be39bba888ca1f859` (normal `gh pr merge 78 --merge` succeeded without admin fallback)
  * Context Compaction (`P-020`) -- `compaction_status: done`
