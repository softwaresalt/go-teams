---
title: Repository documentation refresh and design graduation review and operational closure
date: 2026-11-11
feature: 020-F
shipment: 020-S
branch: feat/020-s-update-and-complete-go-teams-repository-documentation-and-graduate-013-s-018-s-design-decisions
status: review-complete
closure_status: READY
compaction_status: pending
---

## Release scope

Shipment `020-S` (`020-F`, `020.001-T`, `020.002-T`, `020.003-T`) refreshes the
`go-teams` documentation and graduates design decisions from shipments `013-S`,
`016-S`, `017-S`, and `018-S`. It is planned under deliberation `009-DL` and
consumes stash entry `6227A297`. No Go source changed.

* `020.001-T` (`README.md`, commit `3fc94af`):
  * Documents batch queue mode: `--queue` / `-q`, line syntax, quoting, `::`
    as the only comment marker, no nested `--queue`, isolated per-line
    execution, in-place `::` completion marking through `writeQueueLines`
    (`internal/cli/queue.go:200`, temporary-file rename with an in-place
    `os.WriteFile` fallback), `inheritedQueueFlags` forwarding, and
    `teams.RedactArgs` log redaction.
  * Adds `--category` / `-c`, `--filename-template`, `--name-template`, and
    `--output-folder`, the eight-level output target precedence, and the
    filename template placeholders.
  * Adds the `018-S` update locking reference (`<exe>.lock`, `<exe>.lock.ctl`,
    ordered lock chain, pre-swap wait, identity guards, `--force` behavior) and
    manual recovery steps.
  * Expands `## Configuration` (discovery order, key and alias table, the `output`
    file-or-directory rule) and `## Project layout` (all 10 `internal/` packages).
* `020.002-T` (`docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, commit
  `5c0c60c`): domain map, import-derived dependency direction, runtime flows,
  security invariants, graded packages with measured coverage, five test tiers,
  and live-verification gaps (`010-S`).
* `020.003-T` (`docs/design-docs/`, commit `b9628fe`): new
  `2026-11-11-self-update-and-interprocess-locking.md` graduating `005-DL` and
  `007-DL`; `2026-11-09-teams-browser-caption-and-recap-capture.md` updated for
  `002-DL` (URL allowlist and redaction) and `006-DL` (sandbox posture).
* Review-driven corrections (separate commits, task commits not amended):
  `7132936` (`--force` skips identity guards), `83fd1b4` (queue comment syntax),
  `6f4f223` (`internal/cli` import ownership wording).

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`,
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, `mode: report-only`
posture, permanent P-012 Item 5 contract under `008-DL` / `019-F`). The
multi-persona rubric (Constitution, Go, Correctness, Maintainability, Security,
Concurrency, Scope Boundary, Learnings, Template Integrity, Schema-CLI-Docs
Coupling) ran in-session against `main..HEAD`. No repository content went to an
external reviewer model.

| Area | Finding | Disposition |
|---|---|---|
| Correctness | README queue sample used `#` as a comment marker, but `runQueueFile` honors only `::` | Fixed in-scope (P-021 C1, same documentation contract); commit `83fd1b4` |
| Correctness | README presented the update identity guards without noting `--force` skips them (`swap` runs them only when `!force`) | Fixed in-scope; commit `7132936` |
| Correctness | `ARCHITECTURE.md` said `internal/cli` is the only importer of `version`, but `internal/updater` also imports it | Fixed in-scope; commit `6f4f223` |
| Correctness (pre-push `rubber-duck` 1) | README queue note did not say output flag rank still applies after outer and per-line flags merge, so an outer `--output-file` outranks a per-line `--output` or `--output-dir` | Fixed in-scope in the pre-push fix commit; README now recommends an outer `--output-dir` / `--output-folder` |
| Correctness (pre-push `rubber-duck` 2) | Secret-scan scope and limits were unstated in `ARCHITECTURE.md` and `QUALITY_SCORE.md` | Fixed in-scope: tracked regular text files up to `8 MiB`, binary files skipped, `.env` filenames, Teams passcodes, `SKYPE-TOKEN`, and `Bearer eyJ...` patterns listed |
| Correctness (pre-push `rubber-duck` 3) | This record overstated CI coverage for a docs/backlog-only PR | Fixed in-scope: see Verification evidence |
| Correctness (pre-push `rubber-duck` 4) | README and the capture design doc claimed a profile search next to the executable and in `bin\profile`; verified against source, no such fallback exists (`defaultProfileDir` prefers `%LOCALAPPDATA%`, then `os.UserConfigDir()`; an empty `UserDataDir` passes no user-data directory to Chromium) | Fixed in-scope to match the code, then qualified by launch path after Copilot review (chromedp allocator uses a temporary profile; the direct-exec headed launcher omits `--user-data-dir` so Chromium's default profile applies); the finding text named `browser.resolveProfileDir`, which does not exist in the repository or its history |
| Correctness (pre-push `rubber-duck` 5) | `ARCHITECTURE.md` omitted the scheduler's shared-profile busy gate | Fixed in-scope: the gate is `scheduleJob`'s `atomic.Bool` in `internal/cli/schedule.go`, not in `internal/scheduler` |
| Correctness (pre-push `rubber-duck` 6) | README implied `teams-cli update` works on every OS, but `release.yml` publishes only `windows/amd64` and `windows/arm64` | Fixed in-scope: README `Self-update` notes `ErrAssetNotFound` on other platforms |
| Correctness | First `QUALITY_SCORE.md` draft claimed `internal/capture` tests assert `0o700` / `0o600`; no test asserts mode bits | Fixed before commit: package graded `B`, gap recorded |
| Correctness | First design-doc draft said the direct-exec launcher once hardcoded `--no-sandbox`; `006-DL` records that it never consulted the opt-in | Fixed before commit |
| Correctness | Task text cites `TestEncoderDeterministicGoldenReference` near `mp3_test.go:105`; it is at line 158 | No line number cited in the docs; no action |
| Correctness | Task text names `#` comments for queue files; the code supports only `::` | Docs follow the code; noted for Stage |
| Scope | Only `README.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, two design docs, and backlog, closure, and memory records changed; `git diff --stat main` shows no Go files | No action |
| Security | Examples use synthetic URLs only; secret-scan test passes after each staged change | No action |
| Template Integrity | New docs use `title` frontmatter with no H1, fenced blocks carry languages, tables have consistent cells, no trailing whitespace or double blank lines, files end with one newline | No action |
| Constitution | Principles I and II unaffected (documentation only); no edit outside the workspace; `stash@{0}` untouched | No action |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

### Review record

| Field | Value |
|---|---|
| Reviewed diff identity | `main..6f4f223` (`6f4f223`, task commits `3fc94af`, `5c0c60c`, `b9628fe`), plus the pre-push `rubber-duck` fix commit |
| Reviewed HEAD | `6f4f223` for the original review; the pre-push fix commit and the closure commit that follows add only documentation, backlog, and memory records. The final PR HEAD SHA appears in the PR body `## Local Review Readiness` block. |
| `dispatch_mode` | `single-agent-declared-degradation` |
| `confidence_tier` | `DEGRADED_SINGLE_AGENT` |
| `consensus_status` | `DEGRADED_SINGLE_AGENT (non-consensus)` |
| `post_remediation.status` | `"skipped (mode: report-only)"` |

## Follow-ups

The grading work surfaced improvements that are outside this documentation
shipment. They are recorded in `docs/QUALITY_SCORE.md` and are candidates for
follow-up stash entries at the PR stage (Ship Step 5 item 9):

* Add a POSIX-only test asserting the `0o700` and `0o600` modes created by
  `internal/capture`.
* Raise `internal/config` coverage above 80% with alias and precedence cases.
* Add a Windows CI job so Windows-only tests run outside developer machines.
* `010-S` (live Teams verification) remains queued and untouched.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis | `go vet ./...` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, run after every staged commit | PASS |
| Backlog integrity | `backlogit sync` (116 artifacts) and `backlogit doctor` ("No issues found.") | PASS |
| Coverage snapshot | `go test -cover` per package (host ran out of memory on parallel builds; rerun with `-p 1`): `version` 97.8%, `teams` 93.8%, `scheduler` 93.3%, `transcript` 91.2%, `audio` 87.5%, `updater` 84.5%, `cli` 82.0%, `capture` 81.7%, `browser` 72.9%, `config` 69.7% | PASS (recorded in `QUALITY_SCORE.md`) |
| Full build and `go test -race ./...` | Not applicable: no Go source changed. On this docs/backlog-only PR, GitHub Actions runs `Detect code changes` (which includes the always-run tracked-file secret scan) and `Detect CLI-affecting changes`, and skips `Build, vet, lint & test` because `needs.changes.outputs.code == 'true'` is false | N/A |
| Local commands executed | `go vet ./...`, `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` after each staged commit (including the pre-push fix commit), `go test -cover` per package, `backlogit sync`, and `backlogit doctor`; no full `go test -race ./...` or `golangci-lint` run | PASS |
| Telemetry | `autoharness telemetry begin` returned `disabled`; no telemetry recorded | N/A |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Edit documentation and backlog records only | low | Workspace-contained Markdown; `git stash` stack unchanged | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Documentation accuracy against shipped code | READY | Claims were checked against `internal/cli`, `internal/config`, `internal/updater`, `internal/teams`, and `internal/browser` source |
| Runtime impact | READY | None; no executable behavior changed |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: until the next shipment that changes CLI flags, config keys, or the
`internal/` package set.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Docs match shipped flags and packages | Next post-merge closure documentation check | All 10 packages and listed flags documented | A new flag, key, or package absent from `README.md` or `ARCHITECTURE.md` |

## Rollback

Revert the `020-S` merge commit on `main`. All changes are Markdown and backlog
records, with no data migration.

## Handoff

Related [decision](../decisions/2026-11-11-repo-documentation-refresh-deliberation.md)
and [plan](../plans/2026-11-11-repo-documentation-refresh-plan.md). Compaction
(P-020) runs at post-merge closure; `compaction_status` stays `pending` until
then. Phase B pushes the branch and opens the pull request after the
Orchestrator's `rubber-duck` pass.
