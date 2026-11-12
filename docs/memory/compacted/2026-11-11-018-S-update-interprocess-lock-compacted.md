---
type: memory-compacted
date: 2026-11-11
shipment: 018-S
feature: 018-F
source: docs/archive/memory/2026-11-11/018-S-update-interprocess-lock-memory.md
---

# Compacted memory - Shipment 018-S (teams-cli update inter-process lock)

**Shipped**: PR #45, merge `3be466618936f5bf7d2bbc2412d719264c4259db`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship --sha`). Deliberation `007-DL` was retained.

## Decision and outcome

* `Updater.swap` runs under a fail-closed lock chain: `<exe>.lock` (`O_EXCL`, owner
  JSON with pid, token, `created_at`) plus a `<exe>.lock.ctl` control lock for the
  token-verified release. Contenders poll and never evict a lock. A timeout returns
  `ErrUpdateLocked` with a diagnostic that separates a dead-PID orphan, a live PID
  past `lockStaleAfter`, and an active or unreadable lock, with manual recovery steps.
* `acquireSwapLockChain` takes the shortest prefix first, so `foo` and `foo.old`
  updates serialize. The `.old` suffix is case-folded on Windows and macOS. Targets
  ending in `.lock` or `.lock.ctl` are rejected before any mutation.
* Non-force swaps verify the baseline snapshot, running image, and installed version
  under the lock. The launch-image snapshot is a `sync.OnceValue` captured on the
  first `updater.New`, not at package initialization. The `--version` probe has a 30s
  deadline and a 2s `WaitDelay`.
* `internal/cli/update_test.go` covers contention and diagnostics through the command.

## Key reusable insights

* Hashing a whole executable in a package-level `var` taxes every command that
  imports the package. Capture identity lazily on the one path that needs it.
* Anything held under a lock that calls an external process needs its own deadline
  and `WaitDelay`, or a stall holds the lock until manual interruption.
* A 200ms wall-clock timeout in a cancellation test flaked under full-suite load.
  Cancel on an observed condition (both lock markers exist) instead.
* An overlap regression needs both holder directions: base paused with `.old`
  waiting, and `.old` paused with base waiting. The paused base swap removes the
  stale `.old` and moves the executable there, so assertions must match that state.
* Lock-marker names share the namespace with executables. Reject reserved suffixes
  rather than coordinating around them.
* The installed autoharness gate now lists backlogit `1.12` as a verified engine
  line, so `007-DL` classified as `retained_shared_reference` (live referrer
  `019-S`) rather than `retained_engine_unverified`. Compute the disposition each
  time instead of reusing a prior shipment's result.

## Review outcome

Local review: `READY`, `P0=0, P1=0` (single-agent declared degradation under the
P-012 Item 5 bootstrap authorization). Copilot raised four in-scope threads over two
rounds, all fixed (P-021 C1), replied to, and resolved. The `copilot-review` gate
returned `SATISFIED`, CI passed, and the merge used a merge commit.

## Post-merge closure summary

`backlogit shipment ship` returned `archived_ids: [018.001-T, 018.002-T, 018-F,
018-S]` and `returned_ids: []`; both tasks kept `parent_id: 018-F`. `007-DL` stayed
queued because `019-S` still references it. Stash `2D0BAA19` was already archived by
Stage. Linux and macOS runtime behavior was not exercised locally.
