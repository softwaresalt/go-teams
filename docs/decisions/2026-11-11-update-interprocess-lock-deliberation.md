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
serializes the entire swap, rollback, and `.old` cleanup lifecycle, fails closed
without lock-stealing races while diagnosing orphaned or contended locks in
`ErrUpdateLocked`, and leaves zero stray lock or temporary files after normal
completion.

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
    process from unlinking or renaming `lockPath`. Writing a compact JSON
    payload (`pid`, unique per-acquisition `token`, and UTC `created_at`
    timestamp) and closing the file handle immediately allows atomic
    `O_CREATE|O_EXCL` exclusion, provided five concurrency and platform hazards
    are closed without automatic lock-stealing races:
    1. **Control-lock and primary-lock eviction races & OS PID reuse (Findings 1
       & 7, Comment `4543360510`)**: Any automatic lock eviction or stealing of
       `<exe>.lock` or `<exe>.lock.ctl` based on age + PID liveness is
       fundamentally racy without OS-kernel handle locks: two contenders
       inspecting an expired `.lock.ctl` (or `.lock`) can race where Contender A
       replaces `.lock.ctl` and Contender B's delayed rename moves Contender A's
       *new live* `.lock.ctl`. Furthermore, OS PID reuse means an unrelated
       process assigned the same PID prevents age+PID eviction indefinitely.
       Therefore, automatic lock eviction/stealing MUST be eliminated entirely
       (**fail-closed lock acquisition — no automatic eviction of `<exe>.lock`
       or `<exe>.lock.ctl` by contenders**, and `<exe>.lock.ctl` fails closed to
       manual control-lock cleanup so two competing control-lock contenders
       never steal a live `.lock.ctl`). Contenders ONLY acquire `<exe>.lock` via
       `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`, poll
       every `lockPollInterval` (default `50ms`) up to `lockWaitTimeout`
       (default `15s`) respecting `ctx.Done()`, and never unlink, rename, or
       replace an existing `<exe>.lock` or `<exe>.lock.ctl`. When
       `lockWaitTimeout` expires, the contender inspects `<exe>.lock` metadata
       (`pid`, `token`, `created_at`), `lockStaleAfter` (default `2m`), and
       injectable `isProcessAlive(pid)` **for diagnostic classification only** in
       the returned `ErrUpdateLocked` error:
       * *Dead-PID / stale lock* (`now - created_at > lockStaleAfter` and
         `!isProcessAlive(pid)`): `ErrUpdateLocked` reports that `<exe>.lock`
         appears orphaned by dead PID `<pid>` (age `<age>`) and states the
         manual recovery steps.
       * *Live-PID past stale threshold (paused holder OR PID reuse)*
         (`now - created_at > lockStaleAfter` and `isProcessAlive(pid)`):
         `ErrUpdateLocked` reports that `<exe>.lock` is held by live PID `<pid>`
         past `lockStaleAfter` (explicitly noting that if no `teams-cli update`
         process is running, the OS may have reused PID `<pid>`) and states the
         manual recovery steps.
       * *Active / corrupt / unreadable lock*: `ErrUpdateLocked` reports active
         contention or unreadable metadata and states the manual recovery steps.
    2. **Release synchronized with ownership replacement via fail-closed
       `<exe>.lock.ctl` and non-cancelable error-returning `release()` (Findings
       1 & 2, Comment `4543360569`)**: During lock release (`release()`), keep
       the fail-closed `<exe>.lock.ctl`
       (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`,
       **never auto-evicted**) around the authoritative token read +
       `os.Remove(lockPath)` so external/test replacement between an initial
       unlocked read and `<exe>.lock.ctl` acquisition is detected and never
       deletes another owner's `<exe>.lock`. Furthermore, if `release()`
       respected the caller's possibly canceled `ctx` or returned `func()` with
       no error, cancellation during `swap` or a `<exe>.lock.ctl` timeout would
       silently strand `<exe>.lock` while `Update` reported `Updated: true`.
       Therefore, `acquireSwapLock(ctx context.Context, exe string)` returns
       `(release func() error, err error)`, `release()` uses an **independent
       non-cancelable bounded cleanup context**
       `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)`
       (default `2s`, using wall-clock deadline +
       `time.Sleep(ctlLockPollInterval)` — **never** the caller's `ctx`), and
       `swap(ctx, exe, tmp)` executes `relErr := release()` and returns
       `errors.Join(swapErr, relErr)` so any release failure surfaces as a
       non-nil error (`Updated: false`).
    3. **Pre-swap `os.Stat(exe)` / `filepath.EvalSymlinks(exe)` racing with
       in-flight `rename(exe, old)` (Finding 3, Comment `4543360630`)**:
       `Update` calls `u.executablePath()` (`filepath.EvalSymlinks(exe)`) and
       `u.downloadVerified` (`os.Stat(exe)`) *before* `swap` acquires
       `<exe>.lock` (so `<exe>.lock` is not held across the network download).
       If another process is currently inside `swap` between `rename(exe, old)`
       and `rename(tmp, exe)`, `exe` is momentarily absent on disk and
       `os.Stat(exe)` / `EvalSymlinks(exe)` would fail with `fs.ErrNotExist`. A
       helper in `internal/updater/updater.go` used by `executablePath` and
       `downloadVerified` must detect `fs.ErrNotExist` while `<exe>.lock` (or
       `exe + ".old"`) exists on disk and poll every `lockPollInterval` up to
       `lockWaitTimeout` (respecting `ctx.Done()`) waiting for the in-flight
       `swap` to restore `exe` (returning `ErrUpdateLocked` if `lockWaitTimeout`
       expires while `<exe>.lock` remains).
    4. **Windows running-process image lock on `<exe>.old` vs. non-mapped
       concurrent serialization (Finding 4)**: On Windows, `os.Remove(old)` at
       the start of `swap` cannot unlink `<exe>.old` while any Windows process is
       still executing that `.old` binary image. Therefore, both behaviors must
       be specified and verified explicitly:
       * When two concurrent `Updater.Update` / CLI `runUpdate` calls target a
         fixture binary `ExecutablePath` that is not mapped as a running Windows
         process image (or on Unix), `<exe>.lock` serializes them and **both**
         updates succeed in sequence without corrupting `.old`.
       * When `os.Remove(old)` at the start of `swap` fails because `<exe>.old`
         is still locked/mapped by a running Windows process (simulated
         deterministically in unit tests via an open exclusive file handle or
         injectable `removeOld` / file lock on `<exe>.old`), `swap` aborts
         cleanly at `remove stale backup <exe>.old` **before** moving the newly
         installed `exe` aside, leaving the first updater's installed binary
         intact on disk.

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

### Option B: Automatic stale-lock eviction/stealing of `<exe>.lock` and `<exe>.lock.ctl` based on age and `isProcessAlive(pid)`

Allow contenders to automatically rename/unlink an expired `<exe>.lock` or
`<exe>.lock.ctl` when `now - created_at > staleThreshold` and
`!isProcessAlive(pid)`.

* **Pros**: Attempts self-healing after a hard crash without manual lockfile
  removal.
* **Cons**: Two flaws identified in review (Findings 1 & 7, Comment
  `4543360510`): (1) two contenders inspecting an expired `.lock.ctl` (or
  `.lock`) can race where Contender A replaces `.lock.ctl` and Contender B's
  delayed `os.Rename` moves Contender A's *new live* `.lock.ctl`, breaking
  mutual exclusion; and (2) OS PID reuse means an unrelated process assigned the
  same PID causes `isProcessAlive(pid)` to return `true` indefinitely without
  explaining the PID-reuse possibility to the operator.
* **Effort**: medium | **Fit**: low.

### Option C: Fail-closed standard-library atomic `<exe>.lock` (`O_CREATE|O_EXCL`) with zero automatic eviction, diagnostic `ErrUpdateLocked` classification, pre-swap stat wait polling, and non-cancelable error-returning `<exe>.lock.ctl` release (chosen)

In `internal/updater`, wrap the entire `swap` critical section (stale `<exe>.old`
removal, `<exe> -> <exe>.old` rename, `<tmp> -> <exe>` rename, `<exe>.old -> <exe>`
rollback on failure, and post-swap `<exe>.old` cleanup) in a fail-closed,
executable-keyed inter-process lock `<exe>.lock` with release coordinated by
fail-closed `<exe>.lock.ctl`:

1. **Keying, legitimate `.old` preservation vs. renamed running-image
   fail-closed guard, symlink target resolution, immediate `os.Stat` re-probe,
   and start-of-`Update` baseline snapshot + under-lock `installedVersion` /
   running-image identity guard when `!force` (Finding 3, Comments
   `4543360630`, `4543766778`, `4543766919`)**:
   * Do NOT unconditionally strip `.old` in `executablePath()`. A configured
     `u.cfg.ExecutablePath` ending in `.old` or a process legitimately invoked
     as `<name>.old` (`filepath.Base(os.Args[0])` ends in `.old`) MUST preserve
     its exact `.old` path and use `<exe>.old.lock` / `<exe>.old.old` without
     ever redirecting to a sibling `<exe>`. When `u.cfg.ExecutablePath == ""`
     and `os.Executable()` / `filepath.EvalSymlinks` resolves to a path ending
     in ` (deleted)` or ending in `.old` while `filepath.Base(os.Args[0])` does
     NOT end in `.old` (corroborating that another updater renamed the running
     executable image after launch), **fail closed** with
     `ErrConcurrentUpdateModified` (reporting that the running binary was
     already replaced by another update process) unless `--force` is set with an
     explicit path, rather than silently redirecting to a sibling path.
   * Add a helper used by `executablePath()` and `downloadVerified()` that, when
     resolving a symlink or statting `exe` during the transient rename gap,
     resolves the symlink's target path (or parent dir + link target when
     dangling) so marker checks inspect `<target>.lock` / `<target>.old`, and if
     `os.Stat` fails with `fs.ErrNotExist` and neither marker is seen, performs
     one immediate re-probe of `os.Stat(exe)` before returning `fs.ErrNotExist`
     (in case the holder finished `swap` and removed both markers between the
     initial failed `Stat` and the marker check). When `<target>.lock` or
     `<target>.old` exists on disk, poll every `lockPollInterval` up to
     `lockWaitTimeout` (respecting `ctx.Done()`) waiting for the in-flight
     `swap` to restore `exe` (returning `ErrUpdateLocked` if `lockWaitTimeout`
     expires while `<target>.lock` is still present), without holding
     `<exe>.lock` across the network download.
   * Capture the baseline snapshot (`os.Stat` + SHA-256) at the **very start of
     `Update`** (before `lookup` and before any wait); set
     `concurrentSwapObserved = true` ONLY when **`exe` is missing
     (`fs.ErrNotExist`) OR `<exe>.lock` exists on disk** at that initial check
     (so B had to wait for an in-flight swap) and reject `!force` under
     `<exe>.lock` (`ErrConcurrentUpdateModified`) — NEVER set
     `concurrentSwapObserved` merely because a leftover `<exe>.old` exists
     alongside an intact `exe` when `<exe>.lock` is absent, because a leftover
     `<exe>.old` beside an intact `exe` with no `<exe>.lock` is normal
     post-update steady state on Windows (preserving stale-backup cleanup at the
     start of `swap` and `TestUpdateReplacesStaleOldFile`). In addition, under
     `<exe>.lock` inside `swap` when `!force`, verify installed-binary version /
     running-image identity before `rename(exe, old)`: provide an injectable
     `installedVersion func(ctx context.Context, exePath string) (string, error)`
     (or running-executable `os.SameFile` + SHA-256 identity check when
     `u.cfg.ExecutablePath == ""`) that inspects the on-disk `exe` under
     `<exe>.lock` and rejects the swap with `ErrDowngrade` /
     `ErrConcurrentUpdateModified` if the on-disk binary at `exe` is already at
     a version newer than `res.LatestVersion` (or was replaced since B's running
     image launched), verified by
     `TestUpdateRejectsLateSnapshotDowngradeAfterConcurrentUpgradeWithoutForce`
     and
     `TestExecutablePathPreservesLegitimateOldFilenameAndFailsClosedOnRenamedRunningImage`.
2. **Fail-closed atomic acquisition with zero automatic eviction (Findings 1 &
   7, Comment `4543360510`)**: Contenders ONLY acquire `lockPath` via
   `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`, writing
   and flushing JSON metadata
   `{"pid": <pid>, "token": "<128-bit-hex>", "created_at": "<RFC3339Nano>"}` and
   closing the descriptor. While `<exe>.lock` exists (`os.ErrExist`), contenders
   poll every `lockPollInterval` (default `50ms`, overridable on `Updater`) up
   to `lockWaitTimeout` (default `15s`, overridable on `Updater`) respecting
   caller `ctx.Done()`, and **never unlink, rename, or replace an existing
   `<exe>.lock` or `<exe>.lock.ctl`** (`<exe>.lock.ctl` fails closed to manual
   control-lock cleanup and never auto-evicts a `.ctl` file).
3. **Diagnostic classification in `ErrUpdateLocked` on timeout (Findings 1 &
   7)**: When `lockWaitTimeout` expires, the contender inspects `<exe>.lock`
   metadata (`pid`, `token`, `created_at`), `lockStaleAfter` (default `2m`), and
   injectable `isProcessAlive(pid)` **for diagnostic classification only** in
   the returned `ErrUpdateLocked` error:
   * *Dead-PID / stale lock* (`now - created_at > lockStaleAfter` and
     `!isProcessAlive(pid)`): `ErrUpdateLocked` reports that `<exe>.lock`
     appears orphaned by dead PID `<pid>` (age `<age>`) and states the manual
     recovery steps (including restoring `<exe>.old -> <exe>` if `<exe>` is
     missing after a crash between `rename(exe, old)` and `rename(tmp, exe)`).
   * *Live-PID past stale threshold (paused holder OR PID reuse)*
     (`now - created_at > lockStaleAfter` and `isProcessAlive(pid)`):
     `ErrUpdateLocked` reports that `<exe>.lock` is held by live PID `<pid>`
     past `lockStaleAfter` (explicitly noting that if no `teams-cli update`
     process is running, the OS may have reused PID `<pid>`) and states the
     manual recovery steps.
   * *Active / corrupt / unreadable lock*: `ErrUpdateLocked` reports active
     contention or unreadable metadata and states the manual recovery steps.
4. **Error-returning `release()` with independent non-cancelable bounded cleanup
   context and fail-closed `<exe>.lock.ctl` (Findings 1 & 2, Comment
   `4543360569`)**: `acquireSwapLock(ctx context.Context, exe string)` returns
   `(release func() error, err error)`. While acquisition/wait honors the
   caller's `ctx`, `release()` derives an **independent non-cancelable bounded
   cleanup context** via
   `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)`
   (default `2s`, using wall-clock deadline + `time.Sleep(ctlLockPollInterval)`
   default `10ms` — **never** canceled by the caller's `ctx`). `release()`
   acquires fail-closed `<exe>.lock.ctl`
   (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`,
   **never auto-evicted**) around the authoritative token read + comparison +
   `os.Remove(lockPath)` before verifying `ctlToken` and removing
   `<exe>.lock.ctl`, so external/test replacement between an initial unlocked
   read and `<exe>.lock.ctl` acquisition is detected and never deletes another
   owner's `<exe>.lock`. `swap(ctx, exe, tmp)` executes `relErr := release()`
   and returns `errors.Join(swapErr, relErr)`; if `swap` succeeded in replacing
   `exe` (`rename(tmp, exe)` succeeded) and only `release()` returns a cleanup
   error, `Update` sets `res.Updated = true` while still returning the non-nil
   `release()` error surfacing the stranded lock (whereas if `swap` failed
   before replacing `exe`, `res.Updated` remains `false`).
5. **Explicit Windows locked-`<exe>.old` vs. non-mapped concurrent serialization
   contract (Finding 4)**:
   * When two concurrent `Updater.Update` / CLI `runUpdate` calls target a
     fixture binary `ExecutablePath` that is not mapped as a running Windows
     process image (or on Unix), `<exe>.lock` serializes them and **both**
     updates succeed in sequence without corrupting `.old`.
   * When `os.Remove(old)` at the start of `swap` fails because `<exe>.old` is
     still locked/mapped by a running Windows process (simulated
     deterministically in unit tests via an open exclusive file handle or
     injectable `removeOld` / file lock on `<exe>.old`), `swap` aborts cleanly
     at `remove stale backup <exe>.old` **before** moving the newly installed
     `exe` aside, leaving the first updater's installed binary intact on disk.

* **Pros**: Zero external dependencies; completely eliminates stale-eviction and
  control-lock-stealing races; explains both dead-PID orphans and OS PID reuse
  in `ErrUpdateLocked`; waits across transient pre-swap `exe` absence; never
  strands `<exe>.lock` silently on caller context cancellation or control-lock
  contention; accurately models Windows running-image `.old` locking.
* **Cons**: Requires manual operator removal of an orphaned `<exe>.lock` or
  `<exe>.lock.ctl` after a hard process kill (`SIGKILL` / power loss), guided by
  explicit diagnostic instructions in `ErrUpdateLocked`.
* **Effort**: low-medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (`gofrs/flock`) | Option B (Automatic age+PID eviction) | Option C (Fail-closed `O_CREATE\|O_EXCL` + diagnostic `ErrUpdateLocked` + non-cancelable error-returning release) |
|---|---|---|---|
| Dependency footprint | Adds third-party dependencies to `go.mod` | Zero external dependencies | Zero external dependencies (stdlib only) |
| Eviction / lock-stealing race safety | OS handle release | Vulnerable to two-contender `.lock.ctl` / `.lock` rename races and OS PID reuse | Eliminates automatic eviction entirely; contenders never unlink or rename existing locks |
| Pre-swap `os.Stat` / `EvalSymlinks` safety & release error reporting | Requires separate handling | Fails on transient `exe` absence; silent `func()` release can strand locks | Pre-swap wait helper bridges transient `exe -> .old` gap; `release() error` uses `context.WithoutCancel(ctx)` and `errors.Join(swapErr, relErr)` |

## Decision

We select **Option C** and synthesize covering feature **`018-F`**
(*Add inter-process lock around `teams-cli update` binary swap, rollback, and
`.old` cleanup*) and shipment **`018-S`** with two width-isolated tasks:

1. **`018.001-T` (U1, `code` domain — `internal/updater`)**: Implement
   fail-closed `<exe>.lock` and `<exe>.lock.ctl` inter-process locking
   (`ErrUpdateLocked`, `acquireSwapLock(ctx, exe) (func() error, error)` with
   zero automatic eviction of `<exe>.lock` or `<exe>.lock.ctl`, diagnostic
   `ErrUpdateLocked` classification for dead-PID vs. live-PID/PID-reuse vs.
   active/corrupt locks, pre-swap `os.Stat`/`EvalSymlinks` wait polling across
   transient `exe` absence without holding `<exe>.lock` across the network
   download, non-cancelable bounded `release()` via
   `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)` with
   `errors.Join(swapErr, relErr)`, and Windows locked-`<exe>.old` clean abort)
   around `Updater.swap` in `internal/updater/updater.go` and add deterministic
   unit and regression tests in `internal/updater/updater_test.go` (`S` /
   `medium`).
2. **`018.002-T` (U2, `tests` domain — `internal/cli`, depends on `018.001-T`)**:
   Extend `internal/cli/update_test.go` to verify CLI propagation of
   `updater.ErrUpdateLocked`, end-to-end concurrent update serialization on a
   non-mapped fixture binary (pausing Updater A inside `swap` after
   `rename(exe, old)` while Updater B runs pre-swap `executablePath` /
   `os.Stat(exe)`), and clean abort when `<exe>.old` is locked (`XS` / `low`).
3. **Explicit P-012 Item 5 bootstrap authorization (`018-S` before `019-S`,
   Comment `4543907745`)**: Because the operator explicitly ordered Option 2
   (`018-S`) before Option 3 (`019-S`), `007-DL`, `018-F`, `018-S`, and the
   `018-S` plan record explicit P-012 Item 5 bootstrap authorization for
   reviewing `018-S` (and `019-S` prior to `019.001-T` landing) under the
   `single-agent-declared-degradation` review gate (`P0=0, P1=0` severity-gated
   in-session review in `mode: report-only`) when subagent dispatch is
   unavailable.

## Rejected Alternatives

* **Option A (`gofrs/flock` or external syscall locking package)**: Rejected to
  preserve the zero-external-dependency contract of `internal/updater`.
* **Option B (Automatic stale-lock eviction/stealing of `<exe>.lock` or
  `<exe>.lock.ctl`)**: Rejected because two contenders racing to evict an
  expired lock or control lock can overwrite/move a newly created live lock, and
  OS PID reuse prevents age+PID eviction from being deterministic.
* **Holding `<exe>.lock` across the network download**: Rejected because holding
  a filesystem lock during a multi-minute HTTP download increases contention,
  whereas each download already streams to a private `os.CreateTemp` file and a
  lightweight pre-swap `os.Stat`/`EvalSymlinks` wait-poll bridges the sub-millisecond
  window when `exe` is moved to `<exe>.old` under `<exe>.lock`.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Contender race to steal `<exe>.lock` or `<exe>.lock.ctl` & OS PID reuse
  (Findings 1 & 7, Comment `4543360510`)**: Mitigated by eliminating automatic
  eviction of `<exe>.lock` and `<exe>.lock.ctl` entirely (`<exe>.lock.ctl` fails
  closed to manual control-lock cleanup), classifying expired locks
  diagnostically in `ErrUpdateLocked` (dead-PID orphaned vs. live-PID paused or
  OS PID reuse vs. active/corrupt metadata with manual recovery steps), and
  verifying with `TestUpdatePausedOrStaleHolderNeverEvictedAndClassifiedInError`,
  `TestUpdateTwoConcurrentContendersSerializeWithoutEvictionRace`, and
  `TestUpdateTwoConcurrentControlLockContendersNeverStealLiveControlLock`.
* **Caller context cancellation or control-lock contention stranding
  `<exe>.lock` silently on release (Finding 2, Comment `4543360569`)**:
  Mitigated by returning `(release func() error, err error)` from
  `acquireSwapLock`, using
  `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)` inside
  `release()`, joining `errors.Join(swapErr, relErr)` in `swap`, and verifying
  with `TestUpdateReleaseSucceedsWhenCallerContextCanceled`,
  `TestUpdateReleaseSucceedsWhenCallerContextCanceledDuringSwap`, and
  `TestUpdateReleaseFailureSurfacedWhenControlLockHeld`.
* **Releasing holder unlinking a replacement lock between token read and
  `os.Remove` (Finding 1 / Thread 3)**: Mitigated by acquiring fail-closed
  `<exe>.lock.ctl` in `release()` around the authoritative token read +
  comparison + `os.Remove(lockPath)`, verified by
  `TestSwapLockReleaseInterleavedWithReplacementDoesNotDeleteNewOwnerLock`.
* **Pre-swap `os.Stat(exe)` / `EvalSymlinks(exe)` failing with `fs.ErrNotExist`
  during concurrent `swap` (Finding 3, Comment `4543360630`)**: Mitigated by
  polling every `lockPollInterval` up to `lockWaitTimeout` when `os.Stat(exe)`
  or `EvalSymlinks(exe)` returns `fs.ErrNotExist` while `<exe>.lock` or
  `<exe>.old` exists on disk, verified by
  `TestUpdatePreSwapStatWaitsWhileConcurrentSwapMovesExecutableAside`.
* **Windows running process holding `<exe>.old` mapped during a second update
  (Finding 4)**: Mitigated by aborting cleanly at `remove stale backup <exe>.old`
  at the start of `swap` before moving the newly installed `exe` aside, leaving
  the first updater's installed binary intact on disk, and testing both
  non-mapped concurrent serialization and locked-`<exe>.old` clean abort in
  `internal/updater` and `internal/cli`.
* **Safe manual operator cleanup**: Before manually removing `<exe>.lock` or
  `<exe>.lock.ctl`, an operator MUST: (1) prevent new `teams-cli update`
  invocations (e.g., disable/pause scheduled runs), and (2) confirm that all
  existing `teams-cli update` processes have completely terminated — never
  deleting `<exe>.lock` or `<exe>.lock.ctl` while a live or suspended updater
  process exists. If a crash between `rename(exe, old)` and `rename(tmp, exe)`
  left `<exe>` missing and `<exe>.old` present, the operator restores
  `<exe>.old -> <exe>` after confirming all `teams-cli update` processes have
  exited and before removing `<exe>.lock`.
* **Directory cleanliness after contention or failure**: Mitigated by unlinking
  both `<exe>.lock` and `<exe>.lock.ctl` on normal completion, while
  `Updater.Update`'s existing `defer os.Remove(tmp)` cleans up the downloaded
  temporary binary if `swap` returns `ErrUpdateLocked`.
