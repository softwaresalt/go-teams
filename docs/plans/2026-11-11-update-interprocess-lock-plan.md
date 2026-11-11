---
title: Inter-process lock for teams-cli update binary swap implementation plan
description: Implementation plan for 018-F / 018-S adding a fail-closed executable-keyed inter-process lock with diagnostic timeout classification and non-cancelable token-verified release around Updater.swap
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
| R1: Serialize the entire `Updater.swap` lifecycle (stale `.old` removal, `exe -> .old` rename, `tmp -> exe` rename, rollback `.old -> exe`, and post-swap `.old` cleanup) and pre-swap `os.Stat(exe)` / `filepath.EvalSymlinks(exe)` lookups with a fail-closed executable-keyed inter-process lock `<exe>.lock` (`O_CREATE\|O_EXCL`) using only the Go standard library | `2D0BAA19` (`016-S`, `016-F`, `016.002-T`, PR `#40`, `PRRT_kwDOTStvas62SjnB`; PR `#44` comment `4543360630`) | U1 (`018.001-T`) | `go test -race ./internal/updater/...` |
| R2: Eliminate automatic lock eviction/stealing of `<exe>.lock` and `<exe>.lock.ctl` (fail-closed lock acquisition and manual control-lock cleanup), classify timeout errors in `ErrUpdateLocked` via `lockStaleAfter` and injectable `isProcessAlive(pid)` (dead-PID orphaned vs. live-PID paused/OS-PID-reused vs. active/corrupt), and synchronize token-verified release under fail-closed `<exe>.lock.ctl` using `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)` with `errors.Join(swapErr, relErr)` | `2D0BAA19` (`007-DL`, PR `#44` threads `PRRT_kwDOTStvas62Ve9W`, `PRRT_kwDOTStvas62Ve95`, `PRRT_kwDOTStvas62Ve-c`, `PRRT_kwDOTStvas62V-4v`, `PRRT_kwDOTStvas62V-5N`, comments `4543360510`, `4543360569`) | U1 (`018.001-T`) | `go test -race ./internal/updater/...` |
| R3: Verify CLI propagation of `updater.ErrUpdateLocked`, concurrent update serialization on a non-mapped fixture binary (including pre-swap `executablePath` / `os.Stat` wait during `rename(exe, old)`), and clean abort when `<exe>.old` remains locked by a running Windows process in `internal/cli` tests | `2D0BAA19` (`007-DL`, PR `#44` comment `4543360630`) | U2 (`018.002-T`) | `go test -race ./internal/cli/...` |

## Implementation Units

### U1 (`018.001-T`) — Add fail-closed inter-process `<exe>.lock` coordination, pre-swap stat wait polling, diagnostic `ErrUpdateLocked` classification, and non-cancelable error-returning release around `Updater.swap`

* **Domain:** code (Go `internal/updater`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/updater/updater.go`:
     * Export sentinel error `ErrUpdateLocked = errors.New("updater: another update is in progress")`.
     * Add constants `lockSuffix = ".lock"`, `lockCtlSuffix = ".ctl"`,
       `defaultLockWaitTimeout = 15 * time.Second`,
       `defaultLockPollInterval = 50 * time.Millisecond`,
       `defaultLockStaleAfter = 2 * time.Minute`,
       `defaultCtlLockWaitTimeout = 2 * time.Second`, and
       `defaultCtlLockPollInterval = 10 * time.Millisecond`.
     * Add test-overridable fields on `Updater` initialized in `New`:
       `lockWaitTimeout time.Duration`, `lockPollInterval time.Duration`,
       `lockStaleAfter time.Duration`, `ctlLockWaitTimeout time.Duration`,
       `ctlLockPollInterval time.Duration`,
       `now func() time.Time` (defaulting to `time.Now`),
       `isProcessAlive func(pid int) bool` (defaulting to an OS
       process-existence check that fails closed to `alive = true` whenever
       `pid <= 0` or liveness cannot be disproved), `removeOld func(string) error`
       (defaulting to `os.Remove`), and injectable test hooks for deterministic
       pre-swap and release interleaving (including a hook between initial
       unlocked token inspection and `<exe>.lock.ctl` acquisition in
       `release()`).
     * **Executable path safety (preserving legitimate `.old` filenames vs.
       failing closed on renamed running image), symlink target marker
       resolution, immediate `os.Stat` re-probe, and start-of-`Update` baseline
       snapshot + under-lock `installedVersion` / running-image identity guard
       when `!force` (Finding 3, Comments `4543360630`, `4543766778`,
       `4543766919`)**:
       - Do NOT unconditionally strip `.old` in `executablePath()`. A configured
         `u.cfg.ExecutablePath` ending in `.old` or a process legitimately
         invoked as `<name>.old` (`filepath.Base(os.Args[0])` ends in `.old`)
         MUST preserve its exact `.old` path and use `<exe>.old.lock` /
         `<exe>.old.old` without ever redirecting to a sibling `<exe>`. When
         `u.cfg.ExecutablePath == ""` and `os.Executable()` /
         `filepath.EvalSymlinks` resolves to a path ending in ` (deleted)` or
         ending in `.old` while `filepath.Base(os.Args[0])` does NOT end in
         `.old` (corroborating that another updater renamed the running
         executable image after launch), **fail closed** with
         `ErrConcurrentUpdateModified` (reporting that the running binary was
         already replaced by another update process) unless `--force` is set
         with an explicit path, rather than silently redirecting to a sibling
         path.
       - Add a helper in `internal/updater/updater.go` used by
         `executablePath()` and `downloadVerified()` that, when resolving a
         symlink or statting `exe` during the transient rename gap, resolves the
         symlink's target path (or parent dir + link target when dangling) so
         marker checks inspect `<target>.lock` / `<target>.old`, and if
         `os.Stat` fails with `fs.ErrNotExist` and neither marker is seen,
         performs one immediate re-probe of `os.Stat(exe)` before returning
         `fs.ErrNotExist` (in case the holder finished `swap` and removed both
         markers between the initial failed `Stat` and the marker check). When
         `<target>.lock` or `<target>.old` exists on disk, poll every
         `lockPollInterval` up to `lockWaitTimeout` (respecting `ctx.Done()`)
         waiting for the in-flight `swap` to restore `exe` (and if
         `lockWaitTimeout` expires while `<target>.lock` is still present,
         returns `ErrUpdateLocked`), without holding `<exe>.lock` across the
         network download. Also canonicalize a non-empty `u.cfg.ExecutablePath`
         via `filepath.Clean` and `filepath.Abs` (plus `filepath.EvalSymlinks`
         via the wait helper when the target exists).
       - Capture the baseline snapshot (`os.Stat` + SHA-256) at the **very start
         of `Update`** (before `lookup` and before any wait); set
         `concurrentSwapObserved = true` ONLY when **`exe` is missing
         (`fs.ErrNotExist`) OR `<exe>.lock` exists on disk** at that initial
         check (so B had to wait for an in-flight swap) and reject `!force`
         under `<exe>.lock` (`ErrConcurrentUpdateModified`) — NEVER set
         `concurrentSwapObserved` merely because a leftover `<exe>.old` exists
         alongside an intact `exe` when `<exe>.lock` is absent, because a
         leftover `<exe>.old` beside an intact `exe` with no `<exe>.lock` is
         normal post-update steady state on Windows (preserving stale-backup
         cleanup at the start of `swap` and `TestUpdateReplacesStaleOldFile`).
         In addition, under `<exe>.lock` inside `swap` when `!force`, verify
         installed-binary version / running-image identity before
         `rename(exe, old)`: provide an injectable
         `installedVersion func(ctx context.Context, exePath string) (string, error)`
         (or running-executable `os.SameFile` + SHA-256 identity check when
         `u.cfg.ExecutablePath == ""`) that inspects the on-disk `exe` under
         `<exe>.lock` and rejects the swap with `ErrDowngrade` /
         `ErrConcurrentUpdateModified` if the on-disk binary at `exe` is already
         at a version newer than `res.LatestVersion` (or was replaced since B's
         running image launched).
     * Implement `acquireSwapLock(ctx context.Context, exe string) (release func() error, err error)`
       around `lockPath := exe + lockSuffix` and `ctlPath := lockPath + lockCtlSuffix`:
       - **Fail-closed lock acquisition — zero automatic eviction of `<exe>.lock`
         or `<exe>.lock.ctl` by contenders (Findings 1 & 7, Comment
         `4543360510`)**: Contenders ONLY acquire `<exe>.lock` via
         `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`,
         writing and flushing JSON metadata
         `{"pid": os.Getpid(), "token": token, "created_at": now.UTC()}` (`token`
         generated via `crypto/rand`, 16 bytes hex) and closing the file handle.
         While `<exe>.lock` exists (`os.ErrExist`), contenders poll every
         `u.lockPollInterval` (default `50ms`) up to `u.lockWaitTimeout`
         (default `15s`) respecting caller `ctx.Done()`, and **never unlink,
         rename, or replace an existing `<exe>.lock` or `<exe>.lock.ctl`**
         (`<exe>.lock.ctl` fails closed to manual control-lock cleanup and never
         auto-evicts a `.ctl` file, so two competing control-lock contenders
         never steal a live `.lock.ctl`).
       - **Diagnostic classification in `ErrUpdateLocked` on timeout (Findings 1
         & 7)**: When `u.lockWaitTimeout` expires, the contender inspects
         `<exe>.lock` metadata (`pid`, `token`, `created_at`),
         `u.lockStaleAfter` (default `2m`), and `u.isProcessAlive(pid)` **for
         diagnostic classification only** in the returned `ErrUpdateLocked`
         error:
         1. *Dead-PID / stale lock* (`now - created_at > u.lockStaleAfter` and
            `!u.isProcessAlive(pid)`): `ErrUpdateLocked` reports that
            `<exe>.lock` appears orphaned by dead PID `<pid>` (age `<age>`) and
            states the manual recovery steps.
         2. *Live-PID past stale threshold (paused holder OR PID reuse)*
            (`now - created_at > u.lockStaleAfter` and `u.isProcessAlive(pid)`):
            `ErrUpdateLocked` reports that `<exe>.lock` is held by live PID
            `<pid>` past `lockStaleAfter` (explicitly noting that if no
            `teams-cli update` process is running, the OS may have reused PID
            `<pid>`) and states the manual recovery steps.
         3. *Active / corrupt / unreadable lock*: `ErrUpdateLocked` reports
            active contention or unreadable metadata and states the manual
            recovery steps.
       - **Non-cancelable error-returning `release()` under fail-closed
         `<exe>.lock.ctl` (Findings 1 & 2, Comment `4543360569`)**: `release()`
         returns `error` and derives an **independent non-cancelable bounded
         cleanup context** via
         `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)`
         (default `2s`, using wall-clock deadline +
         `time.Sleep(u.ctlLockPollInterval)` default `10ms` — **never** the
         caller's `ctx`). After an optional initial unlocked token inspection
         and injectable test hook, `release()` acquires fail-closed
         `<exe>.lock.ctl`
         (`os.OpenFile(lockPath+".ctl", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`,
         writing/flushing its own `{pid, token, created_at}` metadata and
         **never auto-evicting** an existing `.lock.ctl`) around the
         authoritative token read + comparison + `os.Remove(lockPath)` before
         verifying `ctlToken` and removing `<exe>.lock.ctl`, so external/test
         replacement between the initial unlocked read and `<exe>.lock.ctl`
         acquisition is detected and never deletes another owner's `<exe>.lock`.
     * **Surface release errors, preserve `res.Updated` accuracy, and handle
       Windows locked `<exe>.old` in `swap(ctx context.Context, exe, tmp string) error`
       (Findings 2 & 4, Round-2 Pass)**:
       - Acquire `release, err := u.acquireSwapLock(ctx, exe)` at entry to
         `swap`.
       - Execute `relErr := release()` and return `errors.Join(swapErr, relErr)`;
         if `swap` succeeds in replacing `exe` (`rename(tmp, exe)` succeeded)
         and only `release()` returns a cleanup error, `Update` sets
         `res.Updated = true` while still returning the non-nil `release()`
         error surfacing the stranded lock (whereas if `swap` failed before
         replacing `exe`, `res.Updated` remains `false`).
       - At step 1 of `swap` (`remove stale backup <exe>.old`), if removing an
         existing `<exe>.old` fails because `<exe>.old` is still locked/mapped
         by a running Windows process, abort `swap` cleanly before moving the
         newly installed `exe` aside, leaving the first updater's installed
         binary intact on disk.
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
     * `TestUpdatePausedOrStaleHolderNeverEvictedAndClassifiedInError`
       (**fail-closed non-eviction and diagnostic classification test**,
       Findings 1 & 7): proves that neither a live-PID nor a dead-PID expired
       `<exe>.lock` is ever auto-evicted by contenders (so paused holders and
       concurrent contenders never race to steal `<exe>.lock` or
       `<exe>.lock.ctl`), while `ErrUpdateLocked` accurately classifies
       live-PID/PID-reuse vs. dead-PID orphaned vs. active/corrupt metadata and
       includes manual recovery steps.
     * `TestUpdateTwoConcurrentContendersSerializeWithoutEvictionRace`
       (**concurrent contender serialization test**, Finding 1): proves two
       contenders waiting on a held `<exe>.lock` serialize cleanly via
       `os.O_CREATE|os.O_EXCL` once the holder releases, with zero eviction
       races.
     * `TestUpdateTwoConcurrentControlLockContendersNeverStealLiveControlLock`
       (**deterministic two-control-contender regression test**, Finding 1 /
       Comment `4543360510`): proves that because `<exe>.lock.ctl` fails closed
       to manual control-lock cleanup and is never auto-evicted, two competing
       control-lock contenders never steal or overwrite a live `<exe>.lock.ctl`.
     * `TestSwapLockReleaseInterleavedWithReplacementDoesNotDeleteNewOwnerLock`
       (**deterministic release-interleaved-with-replacement regression test**,
       Finding 1): proves `release()` under fail-closed `<exe>.lock.ctl` detects
       external/test replacement between the initial unlocked token read and
       `<exe>.lock.ctl` acquisition and preserves the replacement lock.
     * `TestUpdateReleaseSucceedsWhenCallerContextCanceled` and
       `TestUpdateReleaseSucceedsWhenCallerContextCanceledDuringSwap`
       (**non-cancelable release cleanup regression tests**, Finding 2 / Comment
       `4543360569`): proves that when the caller's `ctx` is canceled after lock
       acquisition / during `swap`, `release()` still acquires `<exe>.lock.ctl`
       and unlinks `<exe>.lock` using its independent
       `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)`
       cleanup context rather than stranding `<exe>.lock`.
     * `TestUpdateReleaseFailureSurfacedWhenControlLockHeld` (**surfaced release
       failure regression test**, Finding 2): proves that when `<exe>.lock.ctl`
       remains held past `ctlLockWaitTimeout` during `release()` after
       `rename(tmp, exe)` succeeded, `swap` and `Update` return a non-nil error
       via `errors.Join(swapErr, relErr)` while setting `res.Updated = true`
       rather than silently dropping the release error.
     * `TestUpdatePreSwapStatWaitsWhileConcurrentSwapMovesExecutableAside`
       (**pre-swap stat/EvalSymlinks wait regression test**, Finding 3 / Comment
       `4543360630`): pauses Updater A inside `swap` after `rename(exe, old)`
       (while `exe` is transiently absent on disk and `<exe>.lock` and
       `<exe>.old` exist) and runs Updater B's pre-swap `executablePath` /
       `downloadVerified` `os.Stat(exe)` lookup during that gap (without holding
       `<exe>.lock` across the network download), proving Updater B waits for
       Updater A to finish `swap` and then succeeds; also tests dangling-symlink
       target marker resolution (`<target>.lock` / `<target>.old`) and one
       immediate `os.Stat` re-probe when neither marker is seen.
     * `TestUpdateRejectsLateSnapshotDowngradeAfterConcurrentUpgradeWithoutForce`
       (**late-snapshot downgrade regression test**, Comment `4543766778`):
       Updater A installs `v3` before Updater B takes its pre-download snapshot
       while B's `CurrentVersion` is `v1` and target is `v2` without `--force`;
       proves B rejects the swap under `<exe>.lock` (`ErrDowngrade` /
       `ErrConcurrentUpdateModified`) via start-of-`Update` baseline snapshot /
       under-lock `installedVersion` / running-image identity verification and
       leaves `v3` intact on disk.
     * `TestExecutablePathPreservesLegitimateOldFilenameAndFailsClosedOnRenamedRunningImage`
       (**`.old` filename preservation & renamed running-image fail-closed test**,
       Comment `4543766919`): proves that a legitimate `<name>.old` binary
       (`u.cfg.ExecutablePath` ending in `.old` or `filepath.Base(os.Args[0])`
       ending in `.old`) preserves its `.old` path and uses `<exe>.old.lock` /
       `<exe>.old.old`, while `u.cfg.ExecutablePath == ""` with
       `os.Executable()` resolving to `.old` (when `filepath.Base(os.Args[0])`
       does not end in `.old`) or ` (deleted)` fails closed with
       `ErrConcurrentUpdateModified`.
     * Deterministic tests for both Finding 4 cases: (1) two concurrent
       `Updater.Update` calls against a non-mapped fixture binary serialize via
       `<exe>.lock` and both succeed in sequence; and (2) when `<exe>.old`
       removal at the start of `swap` fails because `<exe>.old` is locked by a
       running Windows process (simulated deterministically via an open
       exclusive file handle or `removeOld` hook), `swap` aborts cleanly at
       `remove stale backup <exe>.old` before moving the newly installed `exe`
       aside, leaving the first updater's installed binary intact on disk.
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
     * Add `TestUpdateEndToEndConcurrentUpdatesSerialize` covering both Finding
       3 and Finding 4 behaviors explicitly (Comment `4543360630`):
       1. When two concurrent `teams-cli update` (`runUpdate` /
          `Updater.Update`) invocations target a fixture binary `ExecutablePath`
          that is not mapped as a running Windows process image (or on Unix),
          including pausing Updater A inside `swap` after `rename(exe, old)` and
          running Updater B's pre-swap `executablePath` / `os.Stat(exe)` lookup
          during that gap (without holding `<exe>.lock` across the network
          download), `<exe>.lock` serializes them and **both** updates succeed
          in sequence without corrupting the binary or leaving `.lock`,
          `.lock.ctl`, `.old`, or `.teams-cli-update-*` files behind.
       2. When `os.Remove(old)` at the start of the second `swap` fails because
          `<exe>.old` is still locked/mapped by a running Windows process
          (simulated deterministically via an open exclusive file handle or
          locked `<exe>.old` fixture), `swap` aborts cleanly at
          `remove stale backup <exe>.old` **before** moving the newly installed
          `exe` aside, leaving the first updater's installed binary intact on
          disk.
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

* **Lock scope around `swap` + pre-swap `os.Stat`/`EvalSymlinks` wait helper
  rather than holding `<exe>.lock` across network download (Finding 3, Comment
  `4543360630`)**: Each `Update` call already streams its download to a private
  `os.CreateTemp` file and verifies its SHA-256 checksum before calling `swap`.
  Acquiring `<exe>.lock` at the entry to `swap` keeps lock hold times under a
  millisecond during normal operation and avoids holding a filesystem lock
  across slow network transfers, while the pre-swap wait helper in
  `executablePath()` and `downloadVerified()` polls across the momentary
  `fs.ErrNotExist` window if a concurrent `swap` has moved `exe` to `<exe>.old`
  while `<exe>.lock` or `<exe>.old` is present.
* **Fail-closed lock acquisition with zero automatic eviction and diagnostic
  `ErrUpdateLocked` classification (Findings 1 & 7, Comment `4543360510`)**:
  Eliminating automatic eviction of `<exe>.lock` and `<exe>.lock.ctl`
  completely removes two-contender rename/steal races on both lockfiles and
  avoids silent misbehavior under OS PID reuse, while diagnostic metadata
  inspection on timeout (`lockStaleAfter = 2m` + `isProcessAlive(pid)`) tells
  the operator whether the lock is orphaned by a dead PID, held past the stale
  threshold by a live or OS-reused PID, or actively contended/corrupt.
* **Independent non-cancelable `release()` context and joined release errors
  (Finding 2, Comment `4543360569`)**: Using
  `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)` inside
  `release()` ensures caller cancellation during `swap` does not abort lock
  cleanup, and returning `errors.Join(swapErr, relErr)` guarantees `Update`
  never reports `Updated: true` if `<exe>.lock` could not be released.

## Risks and Caveats

* **Clock skew across processes on the same host**: All `teams-cli update`
  processes targeting the same local executable run on the same host clock, and
  diagnostic age calculations on `lockPath` use the same local clock.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only (`os`, `context`, `crypto/rand`, `encoding/json`, `errors`, `time`, `path/filepath`), explicit sentinel `ErrUpdateLocked`, zero lint/vet findings |
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
  **present (executable swap & lockfile lifecycle)**: coordinates `.old` backup
  deletion, binary replacement, and fail-closed `.lock` / `.lock.ctl` release on
  disk.
* External integration, operator checkpoint, or external dependency — **absent**:
  uses local filesystem operations only (zero external dependencies).
* High runtime, rollout, or rollback risk — **moderate**: lock bugs could leave
  a stale `<exe>.lock` blocking updates or delete an active lock; mitigated by
  fail-closed acquisition (no auto-eviction), non-cancelable token-verified
  release, and `errors.Join(swapErr, relErr)`.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 modifies the executable replacement critical
section in `internal/updater` and introduces cross-process lockfile creation,
diagnostic timeout classification, and fail-closed unlock semantics.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `docs/decisions/2026-11-10-cli-self-update-command-deliberation.md` (`005-DL`)
  * `docs/closure/2026-11-10-016-S-review-and-closure.md` (`016-S`)
  * `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`

### Protected Invariants

1. **Serialized `.old` lifecycle and pre-swap transient-absence wait**: Every
   rename, rollback, or removal of `<exe>.old` and `<exe>` inside `Updater.swap`
   MUST occur while holding a valid `<exe>.lock` token, and pre-swap
   `os.Stat(exe)` / `filepath.EvalSymlinks(exe)` calls in `executablePath()` and
   `downloadVerified()` MUST poll up to `lockWaitTimeout` whenever `exe` returns
   `fs.ErrNotExist` while `<exe>.lock` or `<exe>.old` exists on disk.
2. **Fail-closed lock acquisition — zero automatic eviction of `<exe>.lock` or
   `<exe>.lock.ctl`**: Contenders MUST ONLY acquire `<exe>.lock` and
   `<exe>.lock.ctl` via `os.OpenFile(..., os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`
   and MUST NEVER unlink, rename, or replace an existing `<exe>.lock` or
   `<exe>.lock.ctl` (`<exe>.lock.ctl` fails closed to manual control-lock
   cleanup). When `lockWaitTimeout` expires, `<exe>.lock` metadata,
   `lockStaleAfter`, and `isProcessAlive(pid)` are inspected strictly for
   diagnostic classification in `ErrUpdateLocked` (dead-PID orphaned vs.
   live-PID paused/OS-PID-reused vs. active/corrupt metadata with manual
   recovery steps).
3. **Non-cancelable error-returning `release()` synchronized under fail-closed
   `<exe>.lock.ctl`**: `acquireSwapLock(ctx, exe)` MUST return
   `(release func() error, err error)`. `release()` MUST use an independent
   non-cancelable bounded cleanup context
   `context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)`
   (default `2s`, polling at `ctlLockPollInterval` = `10ms` — never the caller's
   `ctx`) to acquire fail-closed `<exe>.lock.ctl` and perform the authoritative
   token read + comparison + `os.Remove(lockPath)` atomically inside
   `<exe>.lock.ctl` before verifying `ctlToken` and removing `<exe>.lock.ctl`.
   `swap(ctx, exe, tmp)` MUST return `errors.Join(swapErr, relErr)` so any
   release failure is surfaced to the caller (setting `res.Updated = true` when
   `rename(tmp, exe)` succeeded on disk and only `release()` errored).
4. **Clean abort on Windows locked `<exe>.old` and zero stray artifacts on
   normal completion**: If `os.Remove(old)` at the start of `swap` fails because
   `<exe>.old` is still locked/mapped by a running Windows process, `swap` MUST
   abort before moving the newly installed `exe` aside. On normal completion or
   lock contention timeout, no `.teams-cli-update-*` temp file or `.lock.ctl`
   control file may remain in `filepath.Dir(exe)`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Create and release `<exe>.lock` (synchronized via fail-closed `<exe>.lock.ctl`) around `Updater.swap` in `internal/updater` | medium | Uses atomic `os.OpenFile(..., os.O_CREATE\|os.O_EXCL\|os.O_WRONLY, 0o600)` with zero automatic eviction, non-cancelable `context.WithoutCancel(ctx)` cleanup timeout, and `errors.Join(swapErr, relErr)` | Concurrent `teams-cli update` swaps serialize without eviction races, silent stranded locks, or deleting a replacement owner's lock |
| Diagnose expired or contended `<exe>.lock` on `lockWaitTimeout` via `lockStaleAfter` and `isProcessAlive(pid)` | low | Read-only inspection of `<exe>.lock` metadata on timeout to classify dead-PID orphaned vs. live-PID/PID-reuse vs. active/corrupt lock in `ErrUpdateLocked` | Operator receives actionable diagnostic classification and safe manual cleanup steps without automatic lock-stealing hazards |

### Review-Gate Capability Risks

* In this CLI environment, reviewer subagent dispatch is degraded under P-012
  (`TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`).
  The `## Plan Review` section below records literal `dispatch_mode: single-agent-declared-degradation`
  and `decision: PASS` markers and covers all six triggered personas inline.
* **Explicit P-012 Item 5 bootstrap authorization for `018-S` and `019-S`
  (Comment `4543907745`)**: Because the operator explicitly ordered Option 2
  (`018-S`) before Option 3 (`019-S`), this plan (along with `007-DL`, `018-F`,
  `018-S`, `008-DL`, `019-F`, `019-S`, and the `019-S` plan) records explicit
  P-012 Item 5 bootstrap authorization for `_Ship` Step 4.4 to review `018-S`
  (and `019-S` prior to `019.001-T` landing) under the
  `single-agent-declared-degradation` review gate (`P0=0, P1=0` severity-gated
  in-session review in `mode: report-only` with
  `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) when subagent
  dispatch is unavailable, while `019.001-T` replaces the stale
  `(approved for 012-S–017-S)` shipment-range ceiling in `_ship.agent.md` with
  the permanent P-012 Item 5 + `008-DL` / `019-F` contract.

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
  * **Rollback and manual operator recovery (Findings 1 & 7, Round-2 Thread 3
    `PRRT_kwDOTStvas62V-5N`)**:
    - **No automatic eviction by contenders**: Contenders never automatically
      evict `<exe>.lock` or `<exe>.lock.ctl`. Instead, when `lockWaitTimeout`
      expires, `ErrUpdateLocked` classifies whether `<exe>.lock` appears
      orphaned by a dead PID (`now - created_at > lockStaleAfter` and
      `!isProcessAlive(pid)`), held past `lockStaleAfter` by a live PID (noting
      that if no `teams-cli update` process is running, the OS may have reused
      that PID), or actively contended / unreadable.
    - **Required preconditions before manual lockfile cleanup**: Never delete
      `<exe>.lock` or `<exe>.lock.ctl` while a live or suspended
      `teams-cli update` process exists, because a resumed holder would race in
      `swap` and corrupt the shared `<exe>.old` rollback copy. Before manually
      removing `<exe>.lock` or `<exe>.lock.ctl`, the operator MUST: (1) prevent
      new `teams-cli update` invocations (e.g., disable/pause scheduled runs),
      (2) confirm that all existing `teams-cli update` processes have
      completely terminated, and (3) if a crash between `rename(exe, old)` and
      `rename(tmp, exe)` left `<exe>` missing and `<exe>.old` present, restore
      `<exe>.old -> <exe>` before removing `<exe>.lock`. If a code regression is
      suspected, revert the `018-S` merge commit on `main`.

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
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Uses fail-closed `os.OpenFile` with `O_CREATE\|O_EXCL` for `<exe>.lock` and `<exe>.lock.ctl`, diagnostic `isProcessAlive` timeout classification, `crypto/rand` token generation, `context.WithoutCancel(ctx)` release cleanup, `errors.Join(swapErr, relErr)`, and race-tested concurrency seams. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly scoped to `2D0BAA19` (inter-process lock around `teams-cli update` swap, rollback, and `.old` cleanup); does not touch `010-S`, `680A62E6`, or `9A9D8106`. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Aligns with Go 1.26 / `golangci-lint` v1.64.8 conventions and `016-S` closure architecture. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Keeps all locking encapsulated in `internal/updater` with `ErrUpdateLocked` surfaced cleanly through `internal/cli`. | PASS |
| Security Lens Reviewer (binary update & lock surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Lockfile and control lock created with `0o600` in the executable directory, contain no secrets, eliminate automatic lock-stealing races, and synchronize token-verified release under fail-closed `<exe>.lock.ctl`. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `018-F` and shipment `018-S`.
