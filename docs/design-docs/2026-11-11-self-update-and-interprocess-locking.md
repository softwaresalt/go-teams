---
title: Self-update and inter-process locking
date: 2026-11-11
status: implemented
feature: 016-F, 018-F
shipment: 016-S, 018-S
deliberation: 005-DL, 007-DL
---

## Purpose and scope

`teams-cli update` replaces the running executable with a published GitHub
Release build. This document records the durable decisions behind that command:
how versions are compared, how a release is trusted, how the binary is swapped
on Windows, and how concurrent `update` processes are kept from corrupting each
other's work. It graduates the decisions from deliberations
[005-DL](../decisions/2026-11-10-cli-self-update-command-deliberation.md) (the
update command, shipment `016-S`) and
[007-DL](../decisions/2026-11-11-update-interprocess-lock-deliberation.md) (the
inter-process lock, shipment `018-S`). It omits the review history; the
deliberations keep that record.

The code lives in `internal/version`, `internal/updater`, and
`internal/cli/update.go`. For operator-facing steps and flags, see the
[README](../../README.md#self-update). For package placement, see
[ARCHITECTURE.md](../ARCHITECTURE.md).

## Decision summary

| Area | Decision |
|---|---|
| Implementation | Standard library only; no third-party self-update or file-lock dependency |
| Version model | SemVer 2.0.0, with `dev` meaning an upgradable local build |
| Transport | `https` required, with loopback `http` allowed for tests; every redirect target is re-validated |
| Integrity | `SHA256SUMS` is mandatory and verified on a temporary file before any rename |
| Swap | Windows-safe `<exe>.old` rename sequence with automatic rollback |
| Concurrency | Fail-closed `<exe>.lock` created with exclusive create (`O_CREATE` plus `O_EXCL`); no automatic eviction |
| Identity | Under the lock, the updater proves it is replacing the binary it expects |

## Version model (`internal/version`)

* `Version`, `Commit`, and `BuildDate` are package variables that
  `.github/workflows/release.yml` sets through link flags. A local build keeps
  `Version = "dev"`.
* `Normalize` strips a leading `v` or `V`, validates `MAJOR.MINOR.PATCH` with an
  optional pre-release, and drops `+build` metadata because build metadata has
  no precedence.
* `Compare` orders versions by SemVer 2.0.0 precedence, including pre-release
  identifier rules. `IsNewer` treats `dev` as older than every valid release, so
  a local build always sees an upgrade.
* Invalid version strings return `ErrInvalidVersion`; the updater never guesses.

**Rationale.** The update decision must be reproducible and independent of tag
formatting. Hand-rolled comparison is small enough to test exhaustively and
avoids a dependency for roughly a hundred lines of logic.

## Release discovery and transport trust

* The updater queries the GitHub Releases API for `softwaresalt/go-teams`
  (`DefaultAPIBaseURL`) and selects the asset named
  `teams-cli-<goos>-<goarch>[.exe]`, matching what the release workflow
  publishes: `.github/workflows/release.yml` builds CGO-free binaries for
  `linux`, `darwin`, and `windows` on `amd64` and `arm64`, all cross-compiled
  on `ubuntu-latest` (including `darwin` and `linux/arm64`), and verifies that
  `SHA256SUMS` lists all six assets before publishing.
* Every request URL, and every redirect target, must be `https`, or `http` on a
  loopback host so `httptest` servers work. The HTTP client's `CheckRedirect`
  re-runs this validation on each hop and stops after 10 redirects.
* Errors and logs drop the query string from URLs, because signed asset URLs can
  carry tokens in it.
* Downloads are bounded: 4 MiB for release metadata, 1 MiB for `SHA256SUMS`,
  and 256 MiB for the binary.

**Rationale.** The binary is about to replace code the operator will run, so
transport rules are strict and errors must not leak credentials.

## Integrity: mandatory `SHA256SUMS`

The updater downloads `SHA256SUMS` and the platform binary, hashes the binary
while writing it to a temporary file in the executable's directory, and compares
the digest with the manifest entry. A missing manifest, a missing entry for the
platform, an oversized body, or a digest mismatch aborts the update with
`ErrAssetNotFound`, `ErrChecksumMissing`, or `ErrChecksumMismatch` before the
installed executable is touched. The temporary file lives beside the target so
the final rename stays on one filesystem, and a unique name keeps concurrent
downloads from colliding.

**Rejected:** installing without checksum verification. `release.yml` always
publishes `SHA256SUMS`, so a missing manifest signals a problem rather than an
optional feature.

## Windows-safe swap and rollback

A running Windows `.exe` can be renamed but not overwritten or deleted. The swap
therefore works in four steps:

1. Remove a stale `<exe>.old` from an earlier update.
2. Rename `<exe>` to `<exe>.old`.
3. Rename the verified temporary file to `<exe>`; if this fails, rename
   `<exe>.old` back to `<exe>`.
4. Remove `<exe>.old` best-effort. Windows may still hold the running image
   open, so a retained `.old` is cleaned up at the start of the next update.

If step 1 cannot remove the stale backup (for example because a running process
still maps it), the swap stops before step 2, which leaves the installed binary
intact.

## Why a lock: concurrent updaters

`<exe>.old` is one shared path per executable. Without cross-process exclusion,
Process A can finish its swap and delete `<exe>.old` while Process B has just
moved the executable there as its rollback copy. If B's install then fails,
neither `<exe>` nor `<exe>.old` remains. An in-memory mutex cannot help because
the updaters are separate operating system processes.

## Lock protocol

### `<exe>.lock`

* **Creation.** `os.OpenFile(path, O_CREATE|O_EXCL|O_WRONLY, 0o600)` is the only
  acquisition primitive. It is atomic on NTFS and POSIX filesystems.
* **Owner metadata.** The file holds compact JSON: `pid`, a random 128-bit
  `token` unique to this acquisition, and a UTC `created_at`.
* **Waiting.** A contender polls every 50 ms and gives up after 15 seconds,
  honoring context cancellation.
* **No eviction.** A contender never renames, deletes, or replaces another
  process's `<exe>.lock`. When the wait expires, it reads the metadata and
  reports an `ErrUpdateLocked` diagnostic only. A lock older than 2 minutes is
  classified as an orphan of a dead PID, or as held by a live PID that may be
  stale or reused; an active, corrupt, or unreadable lock is reported as
  contention. Each message states the manual recovery steps.
* **Dependency.** The package keeps the standard-library-only contract from
  `005-DL`; `flock`-style libraries were rejected.

**Rationale for no eviction.** Age plus PID liveness cannot prove a holder is
dead. Two contenders can both judge a lock expired, and the slower one can then
remove the faster one's new live lock. The operating system can also reuse a
PID so that an unrelated process makes a dead holder look alive. Without kernel
handle locks, the only safe behavior is to fail closed and ask a human to
recover.

### `<exe>.lock.ctl` and release

Releasing the lock must not delete a lock that another owner has since replaced.
Release therefore:

1. Takes a second marker, `<exe>.lock.ctl`, with the same `O_CREATE|O_EXCL`
   creation and the same no-eviction rule.
2. Rereads `<exe>.lock` while holding the control lock and removes it only if
   the stored `token` matches the releaser's own token.
3. Runs with a bounded cleanup context that ignores cancellation of the update
   context (`context.WithoutCancel`, 2 second wait, 10 ms poll), so Ctrl+C or a
   timeout during the swap cannot strand the lock.
4. Returns an error. `swap` joins a release failure with any swap error, so an
   update whose lock could not be released never reports success silently. A
   control lock that remains after a failed release requires manual cleanup.

### Ordered prefix lock chain

When the target path ends in `.old`, other processes may be locking the
sibling path with the suffix removed. `acquireSwapLockChain` collects the
target and each path produced by stripping trailing `.old` suffixes, then
acquires the locks shortest prefix first, and releases them in reverse. Every
contender uses the same order, so overlapping updaters cannot deadlock. The
`.old` suffix is matched case-insensitively on Windows and macOS, where the
filesystem is commonly case-insensitive.

### Reserved marker names

An executable whose name ends in `.lock` or `.lock.ctl` could have its own
bytes overwritten by another installation's marker. The chain rejects such
names with `errReservedMarkerName` before any filesystem change. The check
compares case-folded names on every platform, which is conservative on
case-sensitive volumes.

## Observing an in-flight swap

The updater resolves the executable path and statistics the file before it
takes the lock, so that the lock is not held across the network download. If
another process is between `rename(exe, old)` and `rename(tmp, exe)`, the
executable is briefly missing. `statExecutable` handles that window:

* If `os.Stat(exe)` reports not-exist, resolve the final symlink target with
  `targetPath` (which also handles dangling links) and check for
  `<target>.lock` and `<target>.old`.
* If either marker exists, poll every 50 ms for up to 15 seconds, honoring
  cancellation, waiting for the holder to restore the executable. On timeout,
  report the lock diagnostic if the lock remains, or a missing-executable error
  if only the backup remains.
* If neither marker exists, probe once more before returning not-exist, in case
  the holder finished between the first stat and the marker check.

## Identity guards

Serialization alone does not stop a stale updater from overwriting a newer
installation. Unless `--force` is set, the swap verifies identity under the lock
before it moves anything:

| Check | Failure |
|---|---|
| A baseline snapshot of the executable (file identity and SHA-256), taken at the start of `Update`, still matches the file under the lock | `ErrConcurrentUpdateModified` |
| The baseline was not taken while a swap by another process was in flight | `ErrConcurrentUpdateModified` |
| The running image matches the target. On Linux the check reads `/proc/self/exe`; elsewhere it uses the launch-time snapshot and `os.Executable` | `ErrConcurrentUpdateModified` |
| The installed binary's version, read by running `<exe> --version`, is not newer than the requested release | `ErrDowngrade` |
| The installed version equals the running version, unless the running build is `dev` | `ErrConcurrentUpdateModified` |
| The executable is unchanged after the version probe | `ErrConcurrentUpdateModified` |

Further rules:

* `ErrConcurrentUpdateModified` wraps `ErrUpdateLocked`, so callers can test for
  either with `errors.Is`.
* A path ending in ` (deleted)` fails closed. A running image whose name ends in
  `.old` is accepted only when `os.Args[0]` provably names that file and no
  sibling `<trimmed>.lock` exists; any ambiguity fails closed rather than
  silently redirecting to a different executable.
* A leftover `<exe>.old` beside an intact executable with no lock is the normal
  state after a Windows update and does not trigger a failure.
* `--force` skips these identity checks (but still takes the lock), because
  it exists to reinstall or downgrade deliberately.

**Accepted boundary.** On platforms without a procfs equivalent, a file replaced
with an identical version string before the process started is not detectable
with the standard library. This is a documented limit, not a defect.

## Hardening from the `016-S` pull request review

Review of the first update implementation (pull request #45) added four rules
that remain part of the contract:

* **Lazy process snapshot.** The running-image snapshot hashes the whole
  executable, so `captureProcessSnapshot` (`sync.OnceValue`) runs on the first
  `updater.New` call that needs it, not at package initialization. Unrelated
  CLI invocations such as `captions` pay nothing.
* **Bounded version probe.** The installed-binary probe (`<exe> --version`) runs
  while the lock is held, so it has a 30 second timeout and a 2 second
  `WaitDelay` that bounds output-pipe cleanup after the process is killed. A
  hung binary cannot hold the lock indefinitely.
* **Reserved marker rejection.** Executable names ending in `.lock` or
  `.lock.ctl` are rejected, as described above.
* **Case-folded `.old` suffix.** The `.old` suffix match in the lock chain is
  case-folded on Windows and macOS.

## Operator recovery contract

Lock errors carry the same instruction: confirm that all `teams-cli update`
processes have exited; if the executable is missing and its `.old` backup
exists, restore `.old` to the executable; then remove `<exe>.lock` and any
`<exe>.lock.ctl`. The error text also names the PID and the lock's age, and
warns that a live PID might be an unrelated process after PID reuse. The
[README](../../README.md#manual-lock-and-backup-recovery) repeats these steps
for operators.

## Consequences and known limits

* A crash between acquisition and release leaves a lock that needs manual
  recovery. This is deliberate; automatic recovery would reintroduce the
  eviction race.
* Locking protects cooperating `teams-cli` processes only. It does not stop an
  unrelated program from modifying the executable.
* The wait budget (15 seconds) can surface `ErrUpdateLocked` on a slow swap;
  retrying the command is safe.
* Behavior on the Windows rename window is verified on `windows-latest` in CI
  (the `test-windows` job, `023-S`) as well as on Windows development hosts (see
  [QUALITY_SCORE.md](../QUALITY_SCORE.md#live-verification-gaps)).
* The `darwin/{amd64,arm64}`, `linux/arm64`, and `windows/arm64` release
  binaries are cross-compiled on `ubuntu-latest` at release time. Only
  `linux/amd64` (`ubuntu-latest`) and `windows/amd64` (`windows-latest`) execute
  native test suites in CI.
