---
title: Inter-process lock for teams-cli update binary swap implementation plan
description: Implementation plan for 018-F / 018-S adding an executable-keyed inter-process lock with stale-lock eviction and token-verified release around Updater.swap
source: docs/decisions/2026-11-11-update-interprocess-lock-deliberation.md
deliberation_id: 007-DL
feature: 018-F
shipment: 018-S
source_stash:
  - 2D0BAA19
source_refs:
  stash_id: 2D0BAA19
  shipment_id: 016-S
  feature_id: 016-F
  task_id: 016.002-T
  pr_number: 40
  review_thread_id: PRRT_kwDOTStvas62SjnB
date: 2026-11-11
status: approved
tags:
  - cli
  - updater
  - concurrency
  - lock
  - windows
---

## Problem Frame

Following the accepted deliberation in
[2026-11-11-update-interprocess-lock-deliberation.md](../decisions/2026-11-11-update-interprocess-lock-deliberation.md)
(`007-DL`), deferred scope expansion stash entry **`2D0BAA19`** (originating from
shipment `016-S`, feature `016-F`, task `016.002-T`, PR `#40`, review thread
`PRRT_kwDOTStvas62SjnB`) is planned under covering feature **`018-F`** and
shipment **`018-S`**.

In `internal/updater/updater.go`, `Updater.swap(exe, tmp)` performs four
filesystem mutations on the shared `<exe>` and `<exe>.old` paths:

1. `os.Remove(exe + ".old")` removes any stale backup from a prior update.
2. `rename(exe, exe + ".old")` moves the active binary aside.
3. `rename(tmp, exe)` moves the verified download into place (or rolls back via
   `rename(exe + ".old", exe)` if this step fails).
4. `_ = os.Remove(exe + ".old")` removes the backup best-effort after success.

Without an inter-process lock keyed by `exe`, two concurrent `teams-cli update`
processes can interleave inside `swap`: Process A's post-swap `os.Remove(old)`
(or Process B's pre-swap `os.Remove(old)`) can delete the peer's active
`<exe>.old` rollback copy while the peer's `rename(tmp, exe)` is in flight. If
that peer's final rename fails, rollback also fails, leaving neither `<exe>` nor
`<exe>.old`.

## Requirements Trace

| Requirement | Stash ID / Provenance | Implementation unit | Verification |
|---|---|---|---|
| R1: Serialize the entire `Updater.swap` lifecycle (stale `.old` removal, `exe -> .old` rename, `tmp -> exe` rename, rollback `.old -> exe`, and post-swap `.old` cleanup) with an executable-keyed inter-process lock `<exe>.lock` (`O_CREATE\|O_EXCL`) using only the Go standard library | `2D0BAA19` (`016-S`, `016-F`, `016.002-T`, PR `#40`, `PRRT_kwDOTStvas62SjnB`) | U1 (`018.001-T`) | `go test -race ./internal/updater/...` |
| R2: Support bounded context-aware wait polling (`ErrUpdateLocked` on timeout), dead-holder liveness proof before stale eviction (`isProcessAlive`, failing closed when holder PID is alive, `<= 0`, or metadata is unreadable/corrupt), and crash-safe, wait-bounded `<exe>.lock.ctl`-synchronized stale-lock replacement and owner-token-verified release | `2D0BAA19` (`007-DL`, PR `#44` threads `PRRT_kwDOTStvas62Ve9W`, `PRRT_kwDOTStvas62Ve95`, `PRRT_kwDOTStvas62Ve-c`, `PRRT_kwDOTStvas62V-4v`, `PRRT_kwDOTStvas62V-5N`) | U1 (`018.001-T`) | `go test -race ./internal/updater/...` |
| R3: Verify CLI propagation of `updater.ErrUpdateLocked` and end-to-end lock contention handling in `internal/cli` tests | `2D0BAA19` (`007-DL`) | U2 (`018.002-T`) | `go test -race ./internal/cli/...` |

## Implementation Units

### U1 (`018.001-T`) — Add inter-process `<exe>.lock` coordination and stale-lock recovery around `Updater.swap`

* **Domain:** code (Go `internal/updater`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/updater/updater.go`:
     * Export sentinel error `ErrUpdateLocked = errors.New("updater: another update is in progress")`.
     * Add constants `lockSuffix = ".lock"`, `lockCtlSuffix = ".ctl"`,
       `defaultLockWaitTimeout = 30 * time.Second`,
       `defaultLockPollInterval = 10 * time.Millisecond`,
       `defaultLockStaleAfter = 2 * time.Minute`,
       `defaultCtlLockWaitTimeout = 2 * time.Second`,
       `defaultCtlLockPollInterval = 10 * time.Millisecond`, and
       `defaultCtlLockStaleAfter = 5 * time.Second`.
     * Add test-overridable fields on `Updater` initialized in `New`:
       `lockWaitTimeout time.Duration`, `lockPollInterval time.Duration`,
       `lockStaleAfter time.Duration`, `ctlLockWaitTimeout time.Duration`,
       `ctlLockPollInterval time.Duration`, `ctlLockStaleAfter time.Duration`,
       `now func() time.Time` (defaulting to `time.Now`),
       `isProcessAlive func(pid int) bool` (defaulting to an OS
       process-existence check that fails closed to `alive = true` whenever
       `pid <= 0` or liveness cannot be disproved), and injectable test hooks
       for deterministic stale-eviction and release interleaving (such as a hook
       between initial unlocked token inspection and `<exe>.lock.ctl`
       acquisition in `release()`).
     * In `executablePath()`, canonicalize a non-empty `u.cfg.ExecutablePath` via
       `filepath.Clean` and `filepath.Abs` (plus `filepath.EvalSymlinks` when the
       resolved path exists) so two callers targeting the same executable via
       different relative or symlinked paths contend on the same `<exe>.lock`.
     * Implement `lockSwap(ctx context.Context, exe string) (func(), error)`
       around `lockPath := exe + lockSuffix` and `ctlPath := lockPath + lockCtlSuffix`:
       - Generates a unique per-acquisition hex token (`crypto/rand`, 16 bytes)
         and writes/flushes JSON metadata
         `{"pid": os.Getpid(), "token": token, "created_at": now.UTC()}` into the
         file created via `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`,
         closing the file handle before returning.
       - **Dead-holder proof before stale eviction (Thread 1)**: Age
         (`now - created_at > u.lockStaleAfter`) is necessary but NEVER
         sufficient on its own to evict `<exe>.lock`. Before any stale eviction,
         the contender MUST verify that the recorded holder `PID` is no longer a
         live process via `u.isProcessAlive(meta.PID)` (using OS
         process-existence checks and failing closed to `alive = true` whenever
         `pid <= 0`, metadata is unreadable/corrupt, or liveness cannot be
         disproved). If the recorded PID is still alive
         (`u.isProcessAlive(meta.PID) == true`), the contender MUST NOT evict
         `<exe>.lock` regardless of age and instead continues waiting until
         `u.lockWaitTimeout` expires and fails closed with `ErrUpdateLocked`.
       - **Crash-safe exclusive control lock `<exe>.lock.ctl` with bounded
         acquisition/release and dead-owner recovery (Thread 2, Round-2 Thread
         2 `PRRT_kwDOTStvas62V-4v`)**: Both stale-lock eviction/replacement and
         lock release MUST acquire a short-lived exclusive control lock
         `<exe>.lock.ctl` (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`):
         * **Control-lock owner metadata**: `<exe>.lock.ctl` writes and flushes
           the same JSON owner metadata (`pid`, 128-bit hex `token`,
           RFC3339Nano `created_at`) on creation (`0o600`).
         * **Bounded acquisition & release**: Both stale-recovery and
           lock-release acquire `<exe>.lock.ctl` using a bounded wait budget
           (`ctlLockWaitTimeout` default `2s`, `ctlLockPollInterval` default
           `10ms`, injectable on `Updater` for deterministic tests) while
           respecting `ctx.Done()`. Releasing `<exe>.lock.ctl` verifies its own
           `ctlToken` before `os.Remove(ctlPath)`.
         * **Dead-owner `<exe>.lock.ctl` recovery**: If `<exe>.lock.ctl` exists
           with `now - ctlMeta.CreatedAt > u.ctlLockStaleAfter` (default `5s`,
           injectable on `Updater`) AND `ctlMeta.PID > 0` AND
           `!u.isProcessAlive(ctlMeta.PID)` (provably dead control-lock owner),
           a contender atomically renames `<exe>.lock.ctl` to
           `<exe>.lock.ctl.stale.<token>` (where `<token>` is the contender's
           freshly generated 128-bit hex token), removes the `.stale` file, and
           retries `os.O_CREATE|os.O_EXCL` acquisition within
           `u.ctlLockWaitTimeout`.
         * **Fail-closed when `<exe>.lock.ctl` is live or unreadable/corrupt**:
           If `<exe>.lock.ctl` metadata is unreadable/corrupt or
           `u.isProcessAlive(ctlMeta.PID)` is `true`, automatic control-lock
           eviction is refused. When `u.ctlLockWaitTimeout` expires, stale
           recovery fails closed with `ErrUpdateLocked` (naming `<exe>.lock` and
           `<exe>.lock.ctl` for manual operator cleanup after confirming all
           `teams-cli update` processes have exited), and primary-lock release
           aborts without unlinking `<exe>.lock` outside `<exe>.lock.ctl`
           (preserving mutual exclusion).
         * **Critical section inside `<exe>.lock.ctl`**: During stale recovery,
           after acquiring `<exe>.lock.ctl`, the contender MUST re-read
           `<exe>.lock` inside the `<exe>.lock.ctl` critical section, re-verify
           `age > u.lockStaleAfter` AND `!u.isProcessAlive(meta.PID)`,
           remove/replace `<exe>.lock`, and create its own fresh `<exe>.lock`
           (with its new token, PID, and fresh `created_at` flushed to disk)
           **before** verifying its `ctlToken` and releasing `<exe>.lock.ctl`.
           Thus, if two contenders race to evict the same expired lock, the
           second contender to acquire `<exe>.lock.ctl` observes the first
           contender's fresh, live-PID lock inside `<exe>.lock.ctl` and aborts
           eviction.
       - While the lock is held by a live or non-stale owner, polls every
         `u.lockPollInterval` until `u.lockWaitTimeout` elapses (or `ctx.Done()`
         fires); if the timeout expires while contended, returns a wrapped
         `ErrUpdateLocked` (`fmt.Errorf("%w: %s", ErrUpdateLocked, lockPath)`).
       - **Release synchronized with ownership replacement via `<exe>.lock.ctl`
         (Thread 3)**: Returns an idempotent `release()` closure that (after an
         optional initial unlocked token inspection and injectable test hook)
         acquires `<exe>.lock.ctl` within `u.ctlLockWaitTimeout` (aborting
         without unlinking `<exe>.lock` outside `<exe>.lock.ctl` if
         `u.ctlLockWaitTimeout` expires) and performs the authoritative token
         read + comparison + `os.Remove(lockPath)` atomically inside the
         `<exe>.lock.ctl` critical section before verifying `ctlToken` and
         removing `<exe>.lock.ctl`.
     * Update `swap(ctx context.Context, exe, tmp string) error` (and its call
       site in `Update`) to acquire `release, err := u.lockSwap(ctx, exe)` at
       entry and `defer release()` so stale `.old` removal, `exe -> .old` rename,
       `tmp -> exe` rename, rollback `.old -> exe`, and post-swap `.old` cleanup
       all run inside the `<exe>.lock` critical section.
  2. In `internal/updater/updater_test.go`, add deterministic unit and
     regression tests:
     * `TestUpdateConcurrentSwapsDoNotDeleteRollbackCopy`: two concurrent
       `Update` executions targeting the same `exePath` where the first pauses
       inside `rename` until the second reaches `swap` and the second's
       `tmp -> exe` rename fails; proves the second update waits for the first's
       `.old` cleanup and lock release before moving `exe` to `.old`, so the
       second update's rollback copy is never deleted by the first update and is
       restored cleanly, with no `.lock`, `.lock.ctl`, `.old`, or temp files
       left behind.
     * `TestUpdateFailsWhenLockHeld`: pre-creates an active non-stale
       `<exe>.lock`, runs `Update` with a short `lockWaitTimeout`, and verifies
       `errors.Is(err, ErrUpdateLocked)`, `exe` is untouched, the holder's
       `<exe>.lock` remains intact, and no `.teams-cli-update-*` temp file
       remains.
     * `TestUpdateEvictsStaleLock`: verifies that when `<exe>.lock` has
       `created_at` older than `lockStaleAfter` AND its recorded `PID` is
       verified dead (`isProcessAlive(meta.PID) == false`), `Update` acquires
       `<exe>.lock.ctl`, re-verifies staleness + dead PID, replaces `<exe>.lock`
       with its fresh lock before releasing `<exe>.lock.ctl`, updates `exe`, and
       leaves no `.lock` or `.lock.ctl` files behind; also verifies that
       unreadable/corrupt metadata or `pid <= 0` fails closed to `alive = true`
       and returns `ErrUpdateLocked`.
     * `TestUpdatePausedLiveHolderNotEvictedPastStaleThreshold` (**paused-live-holder
       regression test**, Thread 1): a live holder pauses inside `swap` past
       `lockStaleAfter` (`isProcessAlive(meta.PID) == true`), and a second
       updater refuses to evict `<exe>.lock` despite its age and fails closed
       with `ErrUpdateLocked` without entering `swap` or touching `<exe>` /
       `<exe>.old`.
     * `TestUpdateTwoConcurrentStaleEvictorsSerialize` (**deterministic
       two-evictor regression test**, Thread 2): two contenders concurrently
       attempt stale eviction on the same expired dead-PID `<exe>.lock`;
       proves that `<exe>.lock.ctl` serializes recovery so only the first
       contender evicts and creates its fresh live-PID `<exe>.lock` inside
       `<exe>.lock.ctl` while the second contender re-reads `<exe>.lock` inside
       `<exe>.lock.ctl`, observes the first contender's fresh live-PID lock,
       aborts eviction, and waits/serializes rather than entering `swap`
       concurrently.
     * `TestSwapLockReleaseInterleavedWithReplacementDoesNotDeleteNewOwnerLock`
       (**deterministic release-interleaved-with-replacement regression test**,
       Thread 3): replaces `<exe>.lock` with a new owner's token after the
       original holder begins `release()` (between initial unlocked token read
       and unlink via the injectable release hook) and proves that the
       authoritative token read + comparison inside `<exe>.lock.ctl` detects the
       replacement and does NOT unlink the new owner's `<exe>.lock`.
     * `TestUpdateRecoverFromCrashedControlLockAndStalePrimaryLock`
       (**crash-during-control-section regression test**, Round-2 Thread 2
       `PRRT_kwDOTStvas62V-4v`): covers both (a) automatic recovery when both
       `<exe>.lock.ctl` (`age > ctlLockStaleAfter`, `ctlMeta.PID > 0`,
       `!isProcessAlive(ctlMeta.PID)`) and `<exe>.lock`
       (`age > lockStaleAfter`, `meta.PID > 0`, `!isProcessAlive(meta.PID)`)
       were left behind by dead PIDs (atomically renaming `<exe>.lock.ctl` to
       `<exe>.lock.ctl.stale.<token>`, removing `.stale`, acquiring
       `<exe>.lock.ctl`, replacing `<exe>.lock`, completing the update, and
       leaving zero `.lock`, `.lock.ctl`, or `.stale` files), AND (b)
       fail-closed refusal (`errors.Is(err, ErrUpdateLocked)` naming both
       `<exe>.lock` and `<exe>.lock.ctl`) when `<exe>.lock.ctl` is held by a
       live PID (`isProcessAlive(ctlMeta.PID) == true`) or contains
       unreadable/corrupt metadata.
* **Files (2 files):** `internal/updater/updater.go`,
  `internal/updater/updater_test.go`.
* **Verify:** `go test -race ./internal/updater/...` passes.
* **Depends on:** none.

### U2 (`018.002-T`) — Add CLI unit and end-to-end lock contention tests for `teams-cli update`

* **Domain:** tests (Go `internal/cli` tests). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. In `internal/cli/update_test.go`:
     * Add `updater.ErrUpdateLocked` to `TestUpdateCommandPropagatesErrors` so
       CLI error wrapping (`errors.Is(err, updater.ErrUpdateLocked)`) is verified.
     * Add `TestUpdateEndToEndConcurrentUpdatesSerialize` executing two
       concurrent `teams-cli update` runs against the same executable path and
       verifying both succeed in sequence without corrupting the binary or
       leaving `.lock`, `.old`, or `.teams-cli-update-*` files behind.
* **Files (1 file):** `internal/cli/update_test.go`.
* **Verify:** `go test -race ./internal/cli/...` passes.
* **Depends on:** U1 (`018.001-T`).

## Dependency Graph

```text
U1 (018.001-T: internal/updater inter-process lock & unit tests)
  └──> U2 (018.002-T: internal/cli ErrUpdateLocked & end-to-end concurrency tests)
```

* Shipment `018-S` has no upstream shipment blocker (`015-S`, `016-S`, and
  `017-S` are already shipped and archived; `010-S` is independent and excluded).

## Decisions and Rationale

* **Lock scope around `swap` rather than network download**: Each `Update` call
  already streams its download to a private `os.CreateTemp` file and verifies
  its SHA-256 checksum before calling `swap`. Acquiring `<exe>.lock` at the
  entry to `swap` keeps lock hold times under a millisecond during normal
  operation, avoids holding a filesystem lock across slow network transfers, and
  protects the entire shared-path critical section (stale `.old` removal,
  `exe -> .old`, `tmp -> exe`, rollback `.old -> exe`, and post-swap `.old`
  cleanup).
* **Bounded wait polling before `ErrUpdateLocked`**: Because `swap` is a brief
  local rename sequence, polling at `10ms` intervals up to `30s` allows two
  legitimate concurrent `teams-cli update` commands to serialize transparently
  while still failing closed with `ErrUpdateLocked` if a peer is wedged.
* **Dead-holder proof (`isProcessAlive`) and `<exe>.lock.ctl`-synchronized
  recovery and release**: Age (`now - created_at > lockStaleAfter`) is necessary
  but never sufficient to evict `<exe>.lock` because a live updater suspended
  inside `swap` could resume later. Verifying `!isProcessAlive(meta.PID)`
  (failing closed to `alive = true` on `pid <= 0`, unreadable/corrupt metadata,
  or indeterminate liveness) prevents evicting a paused live holder. Acquiring
  the short-lived exclusive control lock `<exe>.lock.ctl` around both stale
  eviction/replacement (re-reading `<exe>.lock`, re-verifying age and dead PID,
  and writing/flushing the new owner's fresh `<exe>.lock` before releasing
  `<exe>.lock.ctl`) and lock release (authoritative token read + comparison +
  `os.Remove(lockPath)` inside `<exe>.lock.ctl`) eliminates both the two-evictor
  race and the token-read-to-unlink release race.

## Risks and Caveats

* **Clock skew across processes on the same host**: All `teams-cli update`
  processes targeting the same local executable run on the same host clock, and
  falling back to `os.Stat(lockPath).ModTime()` for unreadable/empty lockfiles
  uses the same local filesystem clock.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only (`os`, `crypto/rand`, `encoding/json`, `time`, `path/filepath`), explicit sentinel `ErrUpdateLocked`, zero lint/vet findings |
| II: Test first | Deterministic unit and concurrency tests in `internal/updater/updater_test.go` and `internal/cli/update_test.go` under `-race` |
| III, IV: Secrets & containment | Lockfile contains only PID, random hex token, and UTC timestamp (`0o600` permissions) in the executable directory and is removed on exit |
| VI: Single responsibility | Two width-isolated tasks (`U1`: `code` in `internal/updater`, 2 files; `U2`: `tests` in `internal/cli`, 1 file) |
| VII, VIII: Strict safety | Filesystem lock and binary swap synchronization classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `007-DL`, plan, feature `018-F`, tasks `018.001-T`–`018.002-T`, and shipment `018-S` recorded with full P-021 C6 provenance |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: exports sentinel error
  `updater.ErrUpdateLocked` and adds cross-process lock coordination on
  `<exe>.lock`.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  protects executable self-replacement integrity and rollback availability under
  concurrent invocations.
* Migration, backfill, destructive data/config action, or irreversible step —
  **present (executable swap & stale lock eviction)**: coordinates `.old` backup
  deletion, binary replacement, and stale `.lock` eviction on disk.
* External integration, operator checkpoint, or external dependency — **absent**:
  uses local filesystem operations only (zero external dependencies).
* High runtime, rollout, or rollback risk — **moderate**: lock bugs could leave
  a stale `<exe>.lock` blocking updates or delete an active lock; mitigated by
  token-verified release and `lockStaleAfter` eviction.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 modifies the executable replacement critical
section in `internal/updater` and introduces cross-process lockfile creation,
stale-lock eviction, and unlock semantics.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `docs/decisions/2026-11-10-cli-self-update-command-deliberation.md` (`005-DL`)
  * `docs/closure/2026-11-10-016-S-review-and-closure.md` (`016-S`)
  * `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`

### Protected Invariants

1. **Serialized `.old` lifecycle**: Every read, rename, rollback, or removal of
   `<exe>.old` and `<exe>` inside `Updater.swap` MUST occur while holding a
   valid `<exe>.lock` token.
2. **Dead-holder proof before stale eviction**: Age (`now - created_at > lockStaleAfter`)
   is necessary but NEVER sufficient on its own to evict `<exe>.lock`. A
   contender MUST verify `!isProcessAlive(meta.PID)` (failing closed to
   `alive = true` whenever `pid <= 0`, metadata is unreadable/corrupt, or
   liveness cannot be disproved) before evicting `<exe>.lock`; if the recorded
   PID is still alive, the contender MUST NOT evict `<exe>.lock` and MUST fail
   closed with `ErrUpdateLocked` once `lockWaitTimeout` expires.
3. **`<exe>.lock.ctl`-serialized stale replacement, crash-safe control-lock
   recovery, and owner-verified unlock**: Both stale-lock replacement and
   `release()` MUST acquire the short-lived exclusive control lock
   `<exe>.lock.ctl` (`O_CREATE|O_EXCL`, writing/flushing `{pid, token, created_at}`
   metadata on creation) within bounded `ctlLockWaitTimeout` (`2s`, polling at
   `ctlLockPollInterval` = `10ms`). An expired `<exe>.lock.ctl`
   (`now - ctlMeta.CreatedAt > ctlLockStaleAfter`, default `5s`) may be
   recovered ONLY when `ctlMeta.PID > 0` AND `!isProcessAlive(ctlMeta.PID)` via
   atomic rename to `<exe>.lock.ctl.stale.<token>` + remove + retry; if
   `<exe>.lock.ctl` is live or unreadable/corrupt, automatic control-lock
   eviction is refused, stale recovery fails closed with `ErrUpdateLocked`
   (naming `<exe>.lock` and `<exe>.lock.ctl`), and primary-lock release aborts
   without unlinking `<exe>.lock` outside `<exe>.lock.ctl`. Stale recovery MUST
   re-read `<exe>.lock`, re-verify `age > lockStaleAfter` AND
   `!isProcessAlive(meta.PID)`, and write/flush the new owner's fresh
   `<exe>.lock` inside `<exe>.lock.ctl` before verifying `ctlToken` and
   releasing `<exe>.lock.ctl`. `release()` MUST perform the authoritative token
   read + comparison + `os.Remove(lockPath)` atomically inside `<exe>.lock.ctl`
   before verifying `ctlToken` and removing `<exe>.lock.ctl`.
4. **Zero stray artifacts after completion**: Whether `Update` succeeds, rolls
   back on rename failure, times out waiting for `<exe>.lock`, or evicts a stale
   dead-PID lock, no `.teams-cli-update-*` temp file or `.lock.ctl` control file
   may remain in `filepath.Dir(exe)`, and `<exe>.lock` must be removed when the
   active holder exits `swap`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Create and release `<exe>.lock` (synchronized via `<exe>.lock.ctl`) around `Updater.swap` in `internal/updater` | medium | Uses atomic `os.OpenFile(..., os.O_CREATE\|os.O_EXCL\|os.O_WRONLY, 0o600)` in `filepath.Dir(exe)` and `<exe>.lock.ctl`-synchronized authoritative token-verified removal in `defer release()` | Concurrent `teams-cli update` swaps serialize without interleaving on `<exe>.old` or unlinking a replacement owner's lock |
| Evict stale `<exe>.lock` older than `lockStaleAfter` when holder PID is dead in `internal/updater` | medium | Requires `!isProcessAlive(meta.PID)` (failing closed to `alive = true` on `pid <= 0` or unreadable metadata) and re-verifies + replaces `<exe>.lock` atomically inside `<exe>.lock.ctl` | Crashed update lockfiles self-heal automatically while paused live holders and competing evictors never corrupt `<exe>.lock` |

### Review-Gate Capability Risks

* In this CLI environment, reviewer subagent dispatch is degraded under P-012
  (`TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`).
  The `## Plan Review` section below records literal `dispatch_mode: single-agent-declared-degradation`
  and `decision: PASS` markers and covers all six triggered personas inline.

## Runtime Verification and Closure

* **Verification**:
  * `go test -race ./internal/updater/...` and `go test -race ./internal/cli/...`
  * `go test -race ./...`
  * `go vet ./...` and `go vet -tags=integration ./...`
  * CI-pinned `golangci-lint run` (`v1.64.8` with `GOTOOLCHAIN=go1.26.0`)
  * `gofmt -l cmd internal`
* **Operational closure**:
  * **Monitoring**: Verify no stray `teams-cli.exe.lock` or
    `teams-cli.exe.lock.ctl` files remain in the executable directory after
    `teams-cli update`.
  * **Rollback and manual operator recovery (Round-2 Thread 3,
    `PRRT_kwDOTStvas62V-5N`)**:
    - **Limits of automatic eviction**: Waiting `lockStaleAfter` (2 minutes)
      only auto-recovers a lock when metadata is valid and the recorded holder
      `PID` (plus any expired `<exe>.lock.ctl` holder `PID` past
      `ctlLockStaleAfter`) is provably dead (`!isProcessAlive(pid)`). Automatic
      eviction cannot recover a paused/hung live holder, unreadable/corrupt lock
      metadata, or a live/corrupt `<exe>.lock.ctl`.
    - **Required preconditions before manual lockfile cleanup**: Never delete
      `<exe>.lock` or `<exe>.lock.ctl` while a live or suspended
      `teams-cli update` process exists, because a resumed holder would race in
      `swap` and corrupt the shared `<exe>.old` rollback copy. Before manually
      removing `<exe>.lock` or `<exe>.lock.ctl`, the operator MUST: (1) prevent
      new `teams-cli update` invocations (e.g., disable/pause scheduled runs),
      and (2) confirm that all existing `teams-cli update` processes have
      completely terminated. If a code regression is suspected, revert the
      `018-S` merge commit on `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants and `strict-safety`
  `ProposedAction` / `ActionRisk` table.

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Both units satisfy the 2-hour rule (`U1`: 2 files, `U2`: 1 file), width isolation (`code` vs. `tests`), test-first posture, and zero third-party dependency additions. | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Uses `os.OpenFile` with `O_CREATE\|O_EXCL` for `<exe>.lock` and `<exe>.lock.ctl`, `isProcessAlive` dead-holder proof, `crypto/rand` token generation, context-aware polling, explicit `ErrUpdateLocked` sentinel error, and race-tested concurrency seams. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly scoped to `2D0BAA19` (inter-process lock around `teams-cli update` swap, rollback, and `.old` cleanup); does not touch `010-S`, `680A62E6`, or `9A9D8106`. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Aligns with Go 1.26 / `golangci-lint` v1.64.8 conventions and `016-S` closure architecture. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Keeps all locking encapsulated in `internal/updater` with `ErrUpdateLocked` surfaced cleanly through `internal/cli`. | PASS |
| Security Lens Reviewer (binary update & lock surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Lockfile and control lock created with `0o600` in the executable directory, contain no secrets, require dead-holder PID proof before eviction, and synchronize replacement and token-verified release under `<exe>.lock.ctl`. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `018-F` and shipment `018-S`.
