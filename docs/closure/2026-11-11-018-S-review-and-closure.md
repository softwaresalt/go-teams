---
title: Update inter-process lock review and operational closure
date: 2026-11-11
feature: 018-F
shipment: 018-S
branch: feat/018-s-add-inter-process-lock-around-teams-cli-update-binary-swap-rollback-and-old-cleanup
status: ready
closure_status: READY
compaction_status: pending
merge_commit: pending
pr: pending
---

## Release scope

Shipment `018-S` (`018-F`, `018.001-T`, `018.002-T`) resolves stash entry
`2D0BAA19` (originating from shipment `016-S`, PR #40 and PR #44 review threads)
planned under deliberation `007-DL`.

* `018.001-T` (`internal/updater/updater.go`, `internal/updater/updater_test.go`):
  `Updater.swap` now runs under a fail-closed, executable-keyed inter-process lock.
  * `<exe>.lock` is created with `O_CREATE|O_EXCL`, mode `0o600`, and holds JSON
    owner metadata (`pid`, 128-bit hex token, `created_at`). Contenders poll
    (`lockPollInterval` 50ms, `lockWaitTimeout` 15s) and never evict or steal a lock.
  * Release is token-verified under a separate `<exe>.lock.ctl` control lock with an
    independent non-cancelable cleanup context (`ctlLockWaitTimeout` 2s). A release
    failure is joined into the swap error, and `Updated` stays true when the new
    binary was installed and only the release failed.
  * A timeout returns `ErrUpdateLocked` with a diagnostic that distinguishes a
    dead-PID orphan, a live PID past `lockStaleAfter` (paused holder or PID reuse),
    and an active or unreadable lock, each with manual recovery steps.
  * Overlapping installations are coordinated through `acquireSwapLockChain`: a swap
    on `foo.old` first takes the locks of every shorter prefix (`foo`, then
    `foo.old`) in a fixed order, so a `foo` update and a forced `foo.old` update
    cannot move each other's rollback copy.
  * With `force` false, `swap` verifies under the lock that the executable and the
    running image still match the baseline snapshot, and that the installed version
    is neither a downgrade nor different from the running version. It aborts with
    `ErrConcurrentUpdateModified` or `ErrDowngrade` before moving the executable
    aside. Legitimate `<name>.old` installations are preserved through verified
    launch-path provenance, and transient pre-swap absence gets one immediate
    re-probe.
* `018.002-T` (`internal/cli/update_test.go`): command-level tests cover lock
  contention surfaced through `update`, with a real paused update, actionable lock
  diagnostics, and the locked Windows backup abort.

No new module dependency was added; the lock uses only the standard library.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`), authorized for `018-S` by the
recorded P-012 Item 5 bootstrap authorization on `018-F`. The multi-persona rubric
(Constitution, Go, Correctness, Maintainability, Security, Concurrency, Scope
Boundary, Learnings) ran in-session against the implementation diff in
`mode: report-only`, without transmitting repository content to an external model.
The Orchestrator's earlier `rubber-duck` pass over the same diff had its findings
folded in before this review.

| Area | Finding | Disposition |
|---|---|---|
| Concurrency | Overlapping base and `.old` installations took different locks, so a forced `foo.old` swap could move the `foo` rollback copy mid-install | Fixed in-scope (P-021 C1: completes the authorized lock); ordered prefix-chain acquisition, with release in reverse order |
| Concurrency | The overlap regression covered only the `.old` holder blocking the base installation | Fixed in-scope; `TestUpdateOverlappingBaseHeldBlocksOldInstallation` now pauses the base swap inside `swap` while an `.old` contender waits, in addition to the existing inverse test |
| Concurrency | `TestSwapOldChainReleasesAcquiredLocksOnCancellation` relied on a 200ms wall-clock timeout and failed once under full-suite load | Fixed in-scope; the test cancels only after both prerequisite lock markers exist, and passed 20 consecutive `-race` runs |
| Correctness | A lock released while a replacement lock appeared could delete the new owner's lock | Covered: release re-reads the owner token under the control lock and refuses a changed owner |
| Correctness | A release failure after a successful install must not report the update as failed | Covered: `Updated` stays true and the release error is joined |
| Correctness | Non-force self-update must not reject normal Windows and macOS updates or real `dev` builds | Covered by regression tests added during harness remediation |
| Security | Lock files use `0o600`, owner metadata holds no secret, and no automatic lock eviction exists | No action |
| Go | Errors wrapped with `%w` and classified through sentinel errors; contexts passed first; no goroutine in production code lacks cancellation | No action |
| Maintainability | Lock helpers stay unexported and local to `internal/updater`; injection seams are unexported fields | No action |
| Scope | Changes stay in the three authorized Go files plus `018-S` backlog, plan, decision, closure, and memory records; the operator's `.github/` and `docs/scratch/` edits are excluded | No action |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62eVwU`): the launch-image snapshot hashed the whole executable at package initialization, so every CLI invocation (capture, auth, `--help`, `--version`, the version subprocess) paid for a full binary read | Fixed in-scope (P-021 C1: completes the authorized launch-identity guard without changing its contract); the snapshot is now a `sync.OnceValue` captured on the first `New` that needs it, which only the update command path reaches, before any wait or network work. The identity reference point moves from package initialization to first updater construction |
| Concurrency | Copilot round 1 (thread `PRRT_kwDOTStvas62eVwm`): the installed-version probe ran under the swap lock with no deadline, so a stalled `--version` would hold the lock until manual interruption | Fixed in-scope (P-021 C1: bounds the authorized lock's hold time); the probe has a 30s deadline and `Cmd.WaitDelay` of 2s, and `TestReadInstalledVersionTimesOutStalledProbe` covers it |

Accepted boundary (not a defect): on platforms without `/proc/self/exe`, a
sub-millisecond replacement of the executable with an identical version before
the updater captures its launch snapshot cannot be distinguished using only the
standard library. This is recorded in `018-F` as an accepted stdlib-only boundary.

Reviewed code HEAD: `feb47dde772120d6fdeb5e44a1ddd4c9d350392f` (the final PR HEAD
adds only documentation records on top of it).
Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

## Follow-ups

None captured. No finding was classified out of scope under P-021 C1, so no
deferred scope expansion entry was created.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Build | `go build ./...` | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Format | `gofmt -l cmd internal` (0 files listed) | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race -count=1 ./...` (all packages), re-run after the Copilot round 1 fixes | PASS |
| Flake check | `go test -race -count=20` on the cancellation test | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Platform coverage | Windows only; Linux and macOS runtime behavior was not exercised locally and relies on CI | NOTED |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Add lock files next to the installed executable during update | medium | Fail-closed creation, mode `0o600`, token-verified release, no automatic eviction, manual recovery documented in the diagnostic | applied |
| Gate non-force swaps on baseline and version identity | medium | Aborts before the executable is moved aside; `force` bypasses the identity guards but keeps the lock | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Single-process update behavior | READY | Same swap, rollback, and cleanup sequence as before when no contention exists |
| Concurrent update serialization | READY | Contenders wait up to 15s, then fail with `ErrUpdateLocked` and recovery steps |
| Orphaned lock after a crash | READY_WITH_CONDITIONS | A crashed updater leaves `<exe>.lock`; the operator follows the diagnostic's manual recovery (never auto-evicted by design) |
| Linux and macOS runtime | READY_WITH_CONDITIONS | Validated through CI only; no local runtime test |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the next two `teams-cli update` runs on each supported platform.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Update completes without leftover lock | Files beside the installed executable | No `.lock` or `.lock.ctl` after exit | A lock file remains with no running update |
| Contention reported clearly | `update` command output | Not applicable | An `ErrUpdateLocked` with no recovery steps |

## Rollback

Revert the `018-S` merge commit on `main`. The change only affects the update
command's swap path. No data migration is involved. An operator who finds a stale
`<exe>.lock` follows the recovery steps in the error message; if the executable is
missing, restore `<exe>.old` to `<exe>` first.

## Handoff

Related [decision](../decisions/2026-11-11-update-interprocess-lock-deliberation.md)
and [plan](../plans/2026-11-11-update-interprocess-lock-plan.md).
