---
title: Inter-process lock for teams-cli update binary swap decision
description: Deliberation on serializing concurrent teams-cli update swap, rollback, and .old cleanup operations with an executable-keyed inter-process lock
topic: Add inter-process lock around teams-cli update binary swap, rollback, and .old cleanup (2D0BAA19)
depth: standard
decision_status: decided
promoted_to: both
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
linked_artifacts:
  - docs/plans/2026-11-11-update-interprocess-lock-plan.md
  - .backlogit/queue/007-DL.md
  - .backlogit/queue/018-F.md
  - .backlogit/queue/018-S.md
date: 2026-11-11
status: accepted
tags:
  - cli
  - updater
  - concurrency
  - lock
  - windows
---

## Problem Frame

Stash entry **`2D0BAA19`** (`task`, captured with provisional priority `low` and
re-prioritized by Stage to `medium` as a binary-replacement integrity guard)
carries a P-021 C2 `DEFERRED SCOPE EXPANSION` marker from shipment **`016-S`**
(feature **`016-F`**, task **`016.002-T`**, PR **#40**, review thread
**`PRRT_kwDOTStvas62SjnB`**, comment `4541277169` on
`internal/updater/updater.go`):

> Add an inter-process lock (keyed by the executable path) around the
> `teams-cli update` swap, rollback, and `.old` cleanup so two concurrent
> `teams-cli update` processes cannot delete each other's rollback copy.

In `internal/updater/updater.go`, `Updater.Update` downloads and SHA-256-verifies
the release binary into a unique temporary file (`os.CreateTemp(filepath.Dir(exe), ".teams-cli-update-*")`),
and then invokes `Updater.swap(exe, tmp)` to replace the target executable using
the Windows-safe `<exe>.old` rename sequence:

1. `os.Remove(exe + ".old")` removes any stale backup from a prior update.
2. `rename(exe, exe + ".old")` moves the current executable aside.
3. `rename(tmp, exe)` installs the verified temporary binary (rolling back via
   `rename(exe + ".old", exe)` if this second rename fails).
4. `_ = os.Remove(exe + ".old")` removes the backup best-effort after a
   successful swap.

Because `<exe>.old` is a single path shared by all processes targeting `<exe>`,
two concurrent `teams-cli update` processes can interleave inside `swap`:

* After Process A completes step 3 (`rename(tmpA, exe)`), Process B can enter
  `swap` and execute steps 1 and 2 (`rename(exe, exe + ".old")`), placing
  Process B's rollback copy at `<exe>.old`.
* Process A then executes step 4 (`_ = os.Remove(exe + ".old")`), deleting
  Process B's active rollback copy while Process B is still executing step 3.
* If Process B's step 3 (`rename(tmpB, exe)`) fails, Process B's rollback
  (`rename(exe + ".old", exe)`) also fails because `<exe>.old` was deleted by
  Process A, leaving neither `<exe>` nor `<exe>.old` on disk.

An in-memory `sync.Mutex` on `Updater` does not protect separate OS processes
invoking `teams-cli update` concurrently. We need a cross-platform,
standard-library-only inter-process lock keyed by the target executable path that
serializes the entire swap, rollback, and `.old` cleanup lifecycle, handles stale
locks left behind by crashed processes, and leaves zero stray lock or temporary
files after completion.

## P-021 C5 and C6 Triage and Reconciliation Record

Per Stage Step 1 (`Deferred-Expansion Triage Obligations`, P-021 C5/C6), both
separately triggered obligations were executed before deliberation:

* **Obligation (A) Unconditional duplicate detection**: Scanned all active stash
  entries (`.backlogit/stash.jsonl`), archived stash entries
  (`.backlogit/archive/stash.jsonl`), active queue artifacts
  (`.backlogit/queue/`), and archived backlog artifacts (`.backlogit/archive/`).
  Only `2D0BAA19` describes inter-process locking around the `teams-cli update`
  swap, rollback, and `.old` cleanup. Outcome: **CLEAN DUPLICATE SCAN**.
* **Obligation (B) Mandatory late-identifier reconciliation**: Inspected all five
  source-reference fields recorded on `2D0BAA19` against
  [2026-11-10-016-S-review-and-closure.md](../closure/2026-11-10-016-S-review-and-closure.md):
  * `shipment`: `016-S` (concrete at capture)
  * `feature`: `016-F` (concrete at capture)
  * `task`: `016.002-T` (concrete at capture)
  * `PR`: `40` (concrete at capture)
  * `review-thread`: `PRRT_kwDOTStvas62SjnB` (concrete at capture)
  No field was recorded as `N/A`. Outcome: **idempotent no-op (all five source
  identifiers concrete at capture)**.

## Research Findings

* **Codebase surface (`internal/updater/updater.go` and `internal/cli/update.go`)**:
  * `Updater.Update(ctx context.Context, targetVersion string, force bool)`
    resolves `exe` via `u.executablePath()`, downloads and verifies the binary
    into `tmp` via `u.downloadVerified(ctx, binaryURL, exe, want)` (which uses
    `os.CreateTemp(filepath.Dir(exe), ".teams-cli-update-*")`, so concurrent
    downloads never collide on temporary filenames), registers
    `defer func() { _ = os.Remove(tmp) }()`, and calls `u.swap(exe, tmp)`.
  * `u.executablePath()` resolves `os.Executable()` + `filepath.EvalSymlinks`
    when `u.cfg.ExecutablePath == ""`, or returns `u.cfg.ExecutablePath` when
    configured explicitly. Canonicalizing `u.cfg.ExecutablePath` via
    `filepath.Abs` and `filepath.EvalSymlinks` (when the target exists) ensures
    callers passing relative paths or symlinks to the same binary derive the same
    canonical path.
* **Prior learnings and constraints (`docs/compound/`, `go.instructions.md`, `005-DL`)**:
  * `internal/updater` has a strict standard-library-only contract established in
    deliberation `005-DL` and task `016.002-T` (`go.mod` must not gain third-party
    locking packages).
  * On Windows (NTFS) and POSIX filesystems, `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`
    is an atomic kernel-enforced create-if-not-exists primitive: if `lockPath`
    already exists, `OpenFile` fails with an error matching `errors.Is(err, fs.ErrExist)`.
  * On Windows, keeping an open file handle to `lockPath` prevents another
    process from unlinking or renaming `lockPath` during stale-lock recovery.
    Writing a compact JSON payload (`pid`, unique per-acquisition `token`, and
    UTC `created_at` timestamp) and closing the file handle immediately allows
    atomic `O_CREATE|O_EXCL` exclusion, provided three additional concurrency
    hazards are closed:
    1. **Paused live holder past `lockStaleAfter` (Thread 1)**: Age
       (`now - created_at > lockStaleAfter`) alone never proves that the lock
       holder crashed; a live updater suspended inside `swap` beyond two minutes
       can resume after an age-only eviction and corrupt `<exe>` or `<exe>.old`.
       Before any stale eviction, the contender must prove the recorded `PID` is
       no longer a live process via an injectable `isProcessAlive func(pid int) bool`
       probe (using OS process-existence checks and failing closed to
       `alive = true` whenever `pid <= 0`, metadata is unreadable/corrupt, or
       liveness cannot be disproved). If `isProcessAlive(meta.PID) == true`, the
       contender must never evict `<exe>.lock` regardless of age and instead
       continue polling until `lockWaitTimeout` expires and fail closed with
       `ErrUpdateLocked`.
    2. **Two contenders racing to evict the same expired lock (Thread 2)**: An
       atomic rename alone does not make the check-and-replace sequence atomic.
       If two contenders inspect the same expired dead-PID lock, Contender B's
       delayed eviction could overwrite Contender A's newly acquired lock.
       Serializing stale-lock eviction/replacement with a short-lived exclusive
       control lock `<exe>.lock.ctl` (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`),
       re-reading `<exe>.lock` and re-verifying `age > lockStaleAfter` AND
       `!isProcessAlive(meta.PID)` inside `<exe>.lock.ctl`, and writing/flushing
       the new owner's fresh `<exe>.lock` before releasing `<exe>.lock.ctl`
       ensures the second contender observes the first contender's fresh,
       live-PID lock and aborts eviction.
    3. **Release racing with ownership replacement between token read and unlink
       (Thread 3)**: If a holder reads its matching token without synchronization
       and pauses while stale recovery replaces `<exe>.lock`, the resuming
       holder's `os.Remove(lockPath)` would delete the new owner's lock.
       Acquiring `<exe>.lock.ctl` during release and performing the authoritative
       token read + comparison + `os.Remove(lockPath)` atomically inside the
       `<exe>.lock.ctl` critical section prevents a releasing holder from ever
       unlinking a replacement lock.
    4. **Crash while holding `<exe>.lock.ctl` and unbounded control-lock wait
       (Round-2 Thread 2, `PRRT_kwDOTStvas62V-4v`)**: If a process is killed while
       holding `<exe>.lock.ctl`, a bare `O_CREATE|O_EXCL` control file without
       owner metadata, wait bounds, or stale recovery would block subsequent
       stale-lock recovery even when the primary holder is provably dead, and
       primary-lock release also requires `<exe>.lock.ctl`. To make
       `<exe>.lock.ctl` crash-safe and wait-bounded:
       * `<exe>.lock.ctl` writes and flushes the same JSON owner metadata
         (`pid`, 128-bit hex `token`, RFC3339Nano `created_at`) on creation
         (`0o600`).
       * Both stale-recovery and lock-release acquire `<exe>.lock.ctl` using a
         bounded wait budget (`ctlLockWaitTimeout` default `2s`,
         `ctlLockPollInterval` default `10ms`, injectable on `Updater` for
         deterministic tests) while respecting `ctx.Done()`.
       * If `<exe>.lock.ctl` exists with
         `now - ctlMeta.CreatedAt > ctlLockStaleAfter` (default `5s`, injectable
         on `Updater`) AND `ctlMeta.PID > 0` AND `!isProcessAlive(ctlMeta.PID)`
         (provably dead control-lock owner), a contender atomically renames
         `<exe>.lock.ctl` to `<exe>.lock.ctl.stale.<token>` (where `<token>` is
         the contender's freshly generated 128-bit hex token), removes the
         `.stale` file, and retries `os.O_CREATE|os.O_EXCL` acquisition within
         `ctlLockWaitTimeout`.
       * If `<exe>.lock.ctl` metadata is unreadable/corrupt or
         `isProcessAlive(ctlMeta.PID)` is `true`, automatic control-lock
         eviction is refused. When `ctlLockWaitTimeout` expires, stale recovery
         fails closed with `ErrUpdateLocked` (naming `<exe>.lock` and
         `<exe>.lock.ctl` for manual operator cleanup after confirming all
         `teams-cli update` processes have exited), and primary-lock release
         aborts without unlinking `<exe>.lock` outside `<exe>.lock.ctl`
         (preserving mutual exclusion). Releasing `<exe>.lock.ctl` verifies its
         own `ctlToken` before `os.Remove`.

## Options Evaluated

### Option A: Third-party OS file-lock dependency (`gofrs/flock`)

Add an external file-locking library using `LockFileEx` on Windows and `flock` on
Unix.

* **Pros**: Automatic kernel release on process termination.
* **Cons**: Violates the standard-library-only architecture of `internal/updater`
  (`005-DL` / `016.002-T`) and adds external `golang.org/x/sys` / `gofrs/flock`
  dependencies to `go.mod` when standard-library `O_CREATE|O_EXCL` lockfiles
  suffice.
* **Effort**: medium | **Fit**: low.

### Option B: Bare `O_CREATE|O_EXCL` lockfile (`<exe>.lock`) with immediate fail-fast and no stale-lock eviction

Create `<exe>.lock` via `os.OpenFile(..., os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`
at the start of `swap`, failing immediately if `<exe>.lock` exists, and remove it
on exit.

* **Pros**: Minimal code using only the Go standard library.
* **Cons**: Two flaws: (1) if a `teams-cli update` process is killed (`SIGKILL`,
  power loss, or crash) during the swap, `<exe>.lock` remains on disk forever and
  permanently blocks all future updates until an operator manually deletes the
  hidden lockfile; (2) failing immediately on contention rather than waiting
  briefly with context-aware polling causes concurrent `teams-cli update` calls
  to fail even though `swap` is a sub-millisecond local rename sequence that can
  be serialized cleanly.
* **Effort**: low | **Fit**: low.

### Option C: Standard-library atomic `<exe>.lock` (`O_CREATE|O_EXCL`) with owner PID/token metadata, dead-holder liveness proof (`isProcessAlive`), `<exe>.lock.ctl`-synchronized stale recovery and release, and context-aware polling (chosen)

In `internal/updater`, wrap the entire `swap` critical section (stale `<exe>.old`
removal, `<exe> -> <exe>.old` rename, `<tmp> -> <exe>` rename, `<exe>.old -> <exe>`
rollback on failure, and post-swap `<exe>.old` cleanup) in an executable-keyed
inter-process lock `<exe>.lock` coordinated by a short-lived exclusive control
lock `<exe>.lock.ctl`:

1. **Keying**: Canonicalize `exe` in `executablePath()` (`filepath.Clean` +
   best-effort `filepath.Abs` / `filepath.EvalSymlinks`) and derive
   `lockPath := exe + ".lock"` (`lockSuffix = ".lock"`) and control lock
   `ctlPath := lockPath + ".ctl"` (`lockCtlSuffix = ".ctl"`).
2. **Atomic acquisition with owner metadata**: Acquire `lockPath` using
   `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`, write and
   flush JSON metadata `{"pid": <pid>, "token": "<128-bit-hex>", "created_at": "<RFC3339Nano>"}`
   into the file, and close the descriptor before returning the release closure.
3. **Serialization via bounded context-aware polling**: When `OpenFile` returns
   `fs.ErrExist` and the existing lock is not eligible for eviction, poll at
   `lockPollInterval` (default `10ms`, overridable on `Updater` for tests) up to
   `lockWaitTimeout` (default `30s`, overridable on `Updater` for tests) while
   honoring `ctx.Done()`. Because `swap` performs only local filesystem renames,
   concurrent `teams-cli update` processes serialize cleanly; if the lock remains
   held past `lockWaitTimeout`, `swap` fails closed with sentinel error
   `ErrUpdateLocked` (`"updater: another update is in progress"`) without
   touching `<exe>` or `<exe>.old`.
4. **Dead-holder proof before stale eviction and crash-safe, wait-bounded
   `<exe>.lock.ctl`-serialized recovery**: Age (`now - created_at > lockStaleAfter`,
   default `2m`, overridable on `Updater` for tests) is necessary but NEVER
   sufficient on its own to evict `<exe>.lock`. Before any stale eviction, the
   contender MUST verify that the recorded holder `PID` is no longer a live
   process via an injectable `isProcessAlive func(pid int) bool` probe on
   `Updater` (using OS process-existence checks and failing closed to
   `alive = true` whenever `pid <= 0`, metadata is unreadable/corrupt, or
   liveness cannot be disproved). If `isProcessAlive(meta.PID) == true`, the
   contender MUST NOT evict `<exe>.lock` regardless of age and instead continues
   waiting until `lockWaitTimeout` expires and fails closed with
   `ErrUpdateLocked`. When a lockfile satisfies both `age > lockStaleAfter` AND
   `!isProcessAlive(meta.PID)`, the contender acquires the short-lived exclusive
   control lock `<exe>.lock.ctl` (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`),
   which writes and flushes its own JSON owner metadata (`pid`, 128-bit hex
   `token`, RFC3339Nano `created_at`) on creation and uses a bounded wait budget
   (`ctlLockWaitTimeout` default `2s`, `ctlLockPollInterval` default `10ms`,
   injectable on `Updater` while respecting `ctx.Done()`). If `<exe>.lock.ctl`
   already exists with `now - ctlMeta.CreatedAt > ctlLockStaleAfter` (default
   `5s`, injectable on `Updater`) AND `ctlMeta.PID > 0` AND
   `!isProcessAlive(ctlMeta.PID)` (provably dead control-lock owner), the
   contender atomically renames `<exe>.lock.ctl` to
   `<exe>.lock.ctl.stale.<token>` (where `<token>` is the contender's freshly
   generated 128-bit hex token), removes the `.stale` file, and retries
   `os.O_CREATE|os.O_EXCL` acquisition within `ctlLockWaitTimeout`. If
   `<exe>.lock.ctl` metadata is unreadable/corrupt or
   `isProcessAlive(ctlMeta.PID)` is `true`, automatic control-lock eviction is
   refused and, when `ctlLockWaitTimeout` expires, stale recovery fails closed
   with `ErrUpdateLocked` (naming `<exe>.lock` and `<exe>.lock.ctl` for manual
   operator cleanup after confirming all `teams-cli update` processes have
   exited). Once `<exe>.lock.ctl` is acquired, the contender re-reads
   `<exe>.lock` inside the `<exe>.lock.ctl` critical section, re-verifies
   `age > lockStaleAfter` AND `!isProcessAlive(meta.PID)`, removes/replaces
   `<exe>.lock`, and creates its own fresh `<exe>.lock` (with its new token,
   PID, and fresh `created_at` flushed to disk) **before** verifying its own
   `ctlToken` and releasing `<exe>.lock.ctl`. Thus, if two contenders race to
   evict the same expired dead-PID lock, the second contender to acquire
   `<exe>.lock.ctl` observes the first contender's fresh, live-PID lock inside
   `<exe>.lock.ctl` and aborts eviction.
5. **Release synchronized with ownership replacement via `<exe>.lock.ctl`**:
   When the lock holder releases `<exe>.lock`, it acquires `<exe>.lock.ctl`
   under the bounded `ctlLockWaitTimeout` (`2s`) budget (if `ctlLockWaitTimeout`
   expires without acquiring `<exe>.lock.ctl`, primary-lock release aborts
   without unlinking `<exe>.lock` outside `<exe>.lock.ctl`, preserving mutual
   exclusion) and performs the authoritative token read + comparison +
   `os.Remove(lockPath)` atomically inside the `<exe>.lock.ctl` critical section
   before verifying its `ctlToken` and removing `<exe>.lock.ctl`. Supporting an
   injectable test hook between an initial unlocked token inspection and
   `<exe>.lock.ctl` acquisition (or inside release) allows deterministic testing
   where `<exe>.lock` is replaced with a new owner's token after the holder
   begins release (between initial token read and unlink), proving the
   authoritative check under `<exe>.lock.ctl` detects the replacement and never
   unlinks the new owner's `<exe>.lock`.

* **Pros**: Zero external dependencies; serializes concurrent `teams-cli update`
  processes around the entire `.old` swap, rollback, and cleanup lifecycle;
  never evicts a paused live holder; serializes competing stale-lock evictors
  and releasing holders via `<exe>.lock.ctl`; 100% deterministic unit testing via
  injectable timing, liveness (`isProcessAlive`), and release/eviction hooks on
  `Updater`.
* **Cons**: Slightly more implementation detail in `internal/updater/updater.go`
  (~160 lines), well within a single `code` task.
* **Effort**: low-medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (`gofrs/flock`) | Option B (Bare `O_CREATE\|O_EXCL`) | Option C (Atomic `<exe>.lock` + `isProcessAlive` + `<exe>.lock.ctl` recovery/release) |
|---|---|---|---|
| Dependency footprint | Adds third-party dependencies to `go.mod` | Zero external dependencies | Zero external dependencies (stdlib only) |
| Concurrent update serialization | Supported | Fails immediately without waiting | Serializes concurrent swaps via bounded context-aware polling; returns `ErrUpdateLocked` on timeout |
| Crash recovery & paused-live-holder safety | OS handle release | None (manual deletion required) | Evicts after `lockStaleAfter` ONLY when `!isProcessAlive(meta.PID)` verified inside `<exe>.lock.ctl`; fails closed if holder PID is still alive or unreadable |
| Multi-evictor & release-vs-replacement safety | OS handle release | Vulnerable to deleting a newer lock | `<exe>.lock.ctl` serializes stale replacement and authoritative token-verified `os.Remove(lockPath)` |

## Decision

We select **Option C** and synthesize covering feature **`018-F`**
(*Add inter-process lock around `teams-cli update` binary swap, rollback, and
`.old` cleanup*) and shipment **`018-S`** with two width-isolated tasks:

1. **`018.001-T` (U1, `code` domain — `internal/updater`)**: Implement
   `<exe>.lock` and crash-safe, wait-bounded `<exe>.lock.ctl` inter-process
   locking (`ErrUpdateLocked`, `lockSwap`, bounded context-aware wait polling,
   `isProcessAlive` dead-holder proof before stale eviction on both `<exe>.lock`
   and `<exe>.lock.ctl`, `<exe>.lock.ctl`-serialized stale replacement and
   token-verified release) around `Updater.swap` in
   `internal/updater/updater.go` and add deterministic concurrency,
   contention-timeout, dead-PID stale eviction, **paused-live-holder**,
   **two-evictor**, **release-interleaved-with-replacement**, and
   **crashed-control-lock (`TestUpdateRecoverFromCrashedControlLockAndStalePrimaryLock`)**
   regression unit tests in `internal/updater/updater_test.go` (`S` / `medium`).
2. **`018.002-T` (U2, `tests` domain — `internal/cli`, depends on `018.001-T`)**:
   Extend `internal/cli/update_test.go` to verify CLI propagation of
   `updater.ErrUpdateLocked` and end-to-end lock contention behavior (`XS` /
   `low`).

## Rejected Alternatives

* **Option A (`gofrs/flock` or external syscall locking package)**: Rejected to
  preserve the zero-external-dependency contract of `internal/updater`.
* **Option B (Bare `O_CREATE|O_EXCL` without polling or stale-lock eviction)**:
  Rejected because an interrupted update would permanently wedge future updates
  and concurrent updates would fail instead of serializing across the brief swap
  window.
* **Holding `<exe>.lock` across the network download**: Rejected because holding
  a filesystem lock during a multi-minute HTTP download increases contention and
  causes slow downloads to collide with stale-lock eviction thresholds, whereas
  each download already streams to a private `os.CreateTemp` file and only the
  local filesystem `swap` (stale `.old` removal, rename to `.old`, rename of temp
  to `exe`, rollback, and `.old` cleanup) touches shared paths.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Suspended live holder exceeding `lockStaleAfter` (Thread 1)**: Mitigated by
  requiring dead-holder proof via `isProcessAlive(meta.PID) == false` before any
  stale eviction, failing closed to `alive = true` whenever `pid <= 0`, metadata
  is unreadable/corrupt, or process liveness cannot be disproved, and verified by
  a paused-live-holder regression test.
* **Competing contenders racing to evict the same expired lock (Thread 2)**:
  Mitigated by acquiring `<exe>.lock.ctl` (`O_CREATE|O_EXCL`), re-reading
  `<exe>.lock`, re-verifying `age > lockStaleAfter` AND `!isProcessAlive(meta.PID)`,
  and creating/flushing the new owner's fresh `<exe>.lock` inside the
  `<exe>.lock.ctl` critical section before releasing `<exe>.lock.ctl`, verified
  by a deterministic two-evictor regression test.
* **Releasing holder unlinking a replacement lock between token read and
  `os.Remove` (Thread 3)**: Mitigated by acquiring `<exe>.lock.ctl` in
  `release()` (bounded by `ctlLockWaitTimeout`, aborting without unlinking
  `<exe>.lock` outside `<exe>.lock.ctl` if `ctlLockWaitTimeout` expires) and
  performing the authoritative token read + comparison + `os.Remove(lockPath)`
  atomically inside the `<exe>.lock.ctl` critical section before verifying
  `ctlToken` and removing `<exe>.lock.ctl`, verified by a deterministic
  release-interleaved-with-replacement regression test using an injectable hook
  between initial unlocked token inspection and `<exe>.lock.ctl` acquisition.
* **Crashed control-lock holder leaving `<exe>.lock.ctl` behind (Round-2 Thread
  2, `PRRT_kwDOTStvas62V-4v`)**: Mitigated by writing/flushing `{pid, token, created_at}`
  metadata into `<exe>.lock.ctl`, bounding `<exe>.lock.ctl` acquisition/release
  by `ctlLockWaitTimeout` (`2s`) and `ctlLockPollInterval` (`10ms`), recovering
  an expired `<exe>.lock.ctl` (`now - ctlMeta.CreatedAt > ctlLockStaleAfter`,
  default `5s`) when `ctlMeta.PID > 0` AND `!isProcessAlive(ctlMeta.PID)` via
  atomic rename to `<exe>.lock.ctl.stale.<token>` + remove + retry, failing
  closed with `ErrUpdateLocked` (naming `<exe>.lock` and `<exe>.lock.ctl` for
  manual operator cleanup after confirming all `teams-cli update` processes have
  exited) when `<exe>.lock.ctl` is live or unreadable/corrupt, and verifying
  both paths in `TestUpdateRecoverFromCrashedControlLockAndStalePrimaryLock`.
* **Limits of automatic eviction and safe manual operator cleanup (Round-2
  Thread 3, `PRRT_kwDOTStvas62V-5N`)**: Waiting `lockStaleAfter` (2 minutes)
  only auto-recovers a lock when metadata is valid and the recorded holder `PID`
  (plus any expired `<exe>.lock.ctl` holder `PID`) is provably dead
  (`!isProcessAlive(pid)`). Automatic eviction cannot recover a paused/hung live
  holder, unreadable/corrupt lock metadata, or a live/corrupt `<exe>.lock.ctl`.
  Before manually removing `<exe>.lock` or `<exe>.lock.ctl`, an operator MUST:
  (1) prevent new `teams-cli update` invocations (e.g., disable/pause scheduled
  runs), and (2) confirm that all existing `teams-cli update` processes have
  completely terminated — never deleting `<exe>.lock` or `<exe>.lock.ctl` while
  a live or suspended updater process exists, because a resumed holder would
  race in `swap` and corrupt the `.old` rollback copy.
* **Directory cleanliness after contention or failure**: Mitigated by unlinking
  both `<exe>.lock` and `<exe>.lock.ctl` on completion, while `Updater.Update`'s
  existing `defer os.Remove(tmp)` cleans up the downloaded temporary binary if
  `swap` returns `ErrUpdateLocked`.
