---
title: Update inter-process lock review and operational closure
date: 2026-11-11
feature: 018-F
shipment: 018-S
branch: feat/018-s-add-inter-process-lock-around-teams-cli-update-binary-swap-rollback-and-old-cleanup
status: ready
closure_status: READY
compaction_status: done
merge_commit: 3be466618936f5bf7d2bbc2412d719264c4259db
pr: 45
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
    cannot move each other's rollback copy. The `.old` suffix is case-folded on
    Windows and macOS, and targets whose names end in `.lock` or `.lock.ctl` are
    rejected before any mutation so a marker can never overwrite an executable.
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
| Concurrency | Copilot round 2 (thread `PRRT_kwDOTStvas62ed97`): an executable named `foo.lock` takes marker `foo.lock.lock`, while a concurrent `foo` update creates `foo.lock`, which the first updater's `rename(tmp, exe)` could overwrite | Fixed in-scope (P-021 C1: protects the authorized lock's marker namespace); `acquireSwapLockChain` rejects any chain path ending in `.lock` or `.lock.ctl` (case-folded) with `errReservedMarkerName` before any mutation, covered by `TestSwapRejectsTargetsCollidingWithLockMarkerNames` |
| Concurrency | Copilot round 2 (thread `PRRT_kwDOTStvas62ed-D`): `.old` suffix case folding was Windows-only, so on case-insensitive macOS a `foo.OLD` target skipped the `foo` lock and could move the base rollback copy | Fixed in-scope (P-021 C1: completes the authorized backup-overlap protection); folding now applies on Windows and macOS, covered by `TestUpdateOverlappingBaseHeldBlocksCaseVariantOldInstallation` (skipped where the filesystem is case-sensitive; exercised locally on Windows only, not on macOS) |

Accepted boundary (not a defect): on platforms without `/proc/self/exe`, a
sub-millisecond replacement of the executable with an identical version before
the updater captures its launch snapshot cannot be distinguished using only the
standard library. This is recorded in `018-F` as an accepted stdlib-only boundary.

Reviewed code HEAD: `06ef5010c1241bed642d7dd87a59e0ef25a09291` (the final PR HEAD
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
| Race suite | `go test -race -count=1 ./...` (all packages), re-run after the Copilot round 1 and 2 fixes | PASS |
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

## Post-merge closure

PR #45 merged to `main` at `3be466618936f5bf7d2bbc2412d719264c4259db`
(`2026-11-12T05:11:56Z`) with a merge commit (P-009). Dark Factory Mode (P-017)
post-merge closure ran on `post-merge/018-update-interprocess-lock`:

* Copilot shadow review raised four threads over two rounds on PR #45, all fixed in
  scope under P-021 C1 (`feb47dd` and `06ef501`), replied to, and resolved. No thread
  was out of scope, so no deferred scope expansion entry was captured.
  `autoharness gate copilot-review` returned `SATISFIED` for the merged HEAD
  `9562ebaeedd59d2dfa13e549d4ac29df55dde870` and again at the last-mile re-check, CI
  passed, and the merge used a merge commit with no admin fallback.
* `MERGE_CONFIRMED`: `gh pr view 45` returned `state: MERGED`, and
  `git merge-base --is-ancestor 3be466618936f5bf7d2bbc2412d719264c4259db
  origin/main` exited 0.
* `autoharness gate pipeline-topology --phase lifecycle --shipment 018-S` passed on
  the closure branch (`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`).
* Pre-close snapshot: `018-F`, `018.001-T`, and `018.002-T` already resided in
  `.backlogit/archive/` with declared `status: done`, so all three were required in
  `archived_ids`. Both tasks declared `parent_id: 018-F`; the shipment record was
  `active`; no orphan declared `shipment_id: 018-S`.
* P-015 fully-covered-root verification: `classify_shipment_close_path` returned
  `CASCADE` (`018-F` is a root whose only descendants are the two manifest tasks), and
  `assess_cascade_engine_semantics` for backlogit `1.12.0` (commit `718e710`, CLI) is
  `VERIFIED` because `1.12` is now in `VERIFIED_CASCADE_ENGINE_MINOR_LINES`.
  `backlogit shipment ship 018-S --sha 3be466618936f5bf7d2bbc2412d719264c4259db`
  returned `archived_ids: [018.001-T, 018.002-T, 018-F, 018-S]` and
  `returned_ids: []`. `archived_ids - allowed_ids` and `required_ids - archived_ids`
  are both empty, both tasks kept `parent_id: 018-F`, `018-S` carries
  `archived_status: shipped`, and no archive file showed as a working-tree deletion
  (P-007).
* Linked-deliberation disposition (`compute_linked_deliberation_disposition`):
  `007-DL` (link kind `description` only, declared status `queued`) is
  `retained_shared_reference` with live referrer `019-S`. It was excluded from
  `allowed_ids` and `required_ids`, and the retained-deliberation preservation check
  passed: `007-DL` is still in `.backlogit/queue/` with `status: queued`. It was
  **not archived**; Cascade Close step 7 never archives a shared-reference
  deliberation. The handoff offered archiving it only once it had zero live
  governing references, which does not hold while `019-S` references it. `019-S`
  stays responsible for it.
* Source artifact cleanup: stash entry `2D0BAA19` was already archived by Stage
  (skipped; present in `.backlogit/archive/stash.jsonl`). `018-F` carries no
  `custom_fields.source_stash_id`. Stash entries `680A62E6` and `9A9D8106` and
  shipment `019-S` were not touched.
* Mandatory P-020 `compact-context` (`target: all`) ran: the session memory was
  compacted to
  `docs/memory/compacted/2026-11-11-018-S-update-interprocess-lock-compacted.md`
  and the verbose originals (`018-S-update-interprocess-lock-memory.md` and
  `018-S-stage-update-interprocess-lock-memory.md`) were archived under
  `docs/archive/memory/2026-11-11/`. Recorded as `compaction_status: done` above.

## Handoff

Related [decision](../decisions/2026-11-11-update-interprocess-lock-deliberation.md)
and [plan](../plans/2026-11-11-update-interprocess-lock-plan.md).
