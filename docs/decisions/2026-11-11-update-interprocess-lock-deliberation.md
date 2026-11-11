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
    atomic `O_CREATE|O_EXCL` exclusion while permitting safe rename-based
    stale-lock eviction and owner-token-verified unlock across both Windows and
    Linux/macOS.

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

### Option C: Standard-library atomic `<exe>.lock` (`O_CREATE|O_EXCL`) with owner token metadata, context-aware serialization polling, stale-lock eviction, and token-verified release (chosen)

In `internal/updater`, wrap the entire `swap` critical section (stale `<exe>.old`
removal, `<exe> -> <exe>.old` rename, `<tmp> -> <exe>` rename, `<exe>.old -> <exe>`
rollback on failure, and post-swap `<exe>.old` cleanup) in an executable-keyed
inter-process lock `<exe>.lock`:

1. **Keying**: Canonicalize `exe` in `executablePath()` (`filepath.Clean` +
   best-effort `filepath.Abs` / `filepath.EvalSymlinks`) and derive
   `lockPath := exe + ".lock"` (`lockSuffix = ".lock"`).
2. **Atomic acquisition with owner metadata**: Acquire `lockPath` using
   `os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)`, write JSON
   metadata `{"pid": <pid>, "token": "<128-bit-hex>", "created_at": "<RFC3339Nano>"}`
   into the file, and close the descriptor before returning the release closure.
3. **Serialization via bounded context-aware polling**: When `OpenFile` returns
   `fs.ErrExist` and the existing lock is not yet stale, poll at
   `lockPollInterval` (default `10ms`, overridable on `Updater` for tests) up to
   `lockWaitTimeout` (default `30s`, overridable on `Updater` for tests) while
   honoring `ctx.Done()`. Because `swap` performs only local filesystem renames,
   concurrent `teams-cli update` processes serialize cleanly; if the lock remains
   held past `lockWaitTimeout`, `swap` fails closed with sentinel error
   `ErrUpdateLocked` (`"updater: another update is in progress"`) without
   touching `<exe>` or `<exe>.old`.
4. **Crash-safe stale-lock detection and atomic eviction**: Inspect the existing
   lockfile's age using its parsed `created_at` timestamp (or `os.Stat(lockPath).ModTime()`
   when the file is empty or unreadable after an interrupted write, treating an
   unreadable file as active while `now.Sub(info.ModTime()) < lockStaleAfter` so
   an in-flight writer is never mistaken for a crash). When the lock's age is at
   or beyond `lockStaleAfter` (default `2m`, overridable on `Updater` for tests),
   evict it by renaming `lockPath` to a unique per-process path
   `<exe>.lock.stale.<token>` and removing that file before retrying acquisition.
5. **Owner-verified release**: The `defer release()` closure re-reads `lockPath`
   and unlinks it only when the stored `token` matches the holder's token,
   guaranteeing a delayed holder can never delete a lock acquired by a later
   process after stale-lock eviction.

* **Pros**: Zero external dependencies; serializes concurrent `teams-cli update`
  processes around the entire `.old` swap, rollback, and cleanup lifecycle;
  self-heals after process crashes via `lockStaleAfter`; prevents a late holder
  from deleting a newer owner's lock via token verification; 100% deterministic
  unit testing via injectable timing seams on `Updater`.
* **Cons**: Slightly more implementation detail in `internal/updater/updater.go`
  (~110 lines), well within a single `code` task.
* **Effort**: low-medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (`gofrs/flock`) | Option B (Bare `O_CREATE\|O_EXCL`) | Option C (Atomic `<exe>.lock` + poll + stale eviction + token release) |
|---|---|---|---|
| Dependency footprint | Adds third-party dependencies to `go.mod` | Zero external dependencies | Zero external dependencies (stdlib only) |
| Concurrent update serialization | Supported | Fails immediately without waiting | Serializes concurrent swaps via bounded context-aware polling; returns `ErrUpdateLocked` on timeout |
| Crash recovery (stale lockfile) | OS handle release | None (manual deletion required) | Automatic stale-lock eviction after `lockStaleAfter` using rename-then-remove |
| Late-holder safety | OS handle release | Vulnerable to deleting a newer lock | Token-verified release unlinks `<exe>.lock` only if owner token still matches |

## Decision

We select **Option C** and synthesize covering feature **`018-F`**
(*Add inter-process lock around `teams-cli update` binary swap, rollback, and
`.old` cleanup*) and shipment **`018-S`** with two width-isolated tasks:

1. **`018.001-T` (U1, `code` domain — `internal/updater`)**: Implement
   `<exe>.lock` inter-process locking (`ErrUpdateLocked`, `lockSwap`, bounded
   context-aware wait polling, stale-lock eviction, and token-verified release)
   around `Updater.swap` in `internal/updater/updater.go` and add deterministic
   concurrency, contention-timeout, stale-lock eviction, and token-ownership unit
   tests in `internal/updater/updater_test.go` (`S` / `medium`).
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

* **Race between lockfile creation and metadata write**: Mitigated by treating an
  empty or partially written `<exe>.lock` as active whenever its filesystem
  `ModTime` age is less than `lockStaleAfter`, so a polling peer waits for the
  writer to finish rather than misclassifying the new lockfile as corrupt/stale.
* **Delayed process releasing a lock after stale eviction**: Mitigated by reading
  back `<exe>.lock` in `release()` and removing the file only when its JSON
  `token` matches the holder's unique random token.
* **Directory cleanliness after contention or failure**: Mitigated by unlinking
  both `<exe>.lock` (via token-verified `release()`) and any `.stale.` eviction
  file, while `Updater.Update`'s existing `defer os.Remove(tmp)` cleans up the
  downloaded temporary binary if `swap` returns `ErrUpdateLocked`.
