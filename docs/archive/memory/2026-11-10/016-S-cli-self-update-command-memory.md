---
type: memory
date: 2026-11-10
shipment: 016-S
feature: 016-F
branch: feat/016-s-add-teams-cli-update-self-update-command-with-sha-256-verification
status: pending-merge
---

# Session memory - Shipment 016-S (teams-cli update)

## Completed

* `016.001-T` `internal/version`: commit `07f759d`
* `016.002-T` `internal/updater`: commit `5245955`
* `016.003-T` `update` command and `root.Version`: commit `7c21484`
* `016.004-T` CLI unit and end-to-end tests: commit `7f7557f`
* `016.005-T` README documentation: commit `143a94a`
* `016-F` moved to `done`; `016-S` is `active` until post-merge closure

## Decisions

* Followed the approved plan and task API (`Normalize`, `Compare`, `IsNewer`,
  `deps` by pointer) rather than the differently named API in the handoff, because
  the plan matches `release.yml` and the existing package conventions.
* No archive extraction: `release.yml` publishes raw executables, so tar/zip
  guards would be unreachable code.
* Downgrades are rejected inside `updater.Update` (not only the CLI) so every
  caller gets the guard; `--force` allows downgrade and same-version reinstall.
* Wave-staged harness: each task's tests were written first against compiling
  error-returning stubs, verified failing, then implemented; the CLI tests were
  written during `016.003-T` and committed under `016.004-T`.

## Errors and resolutions

* Temp-file leak on checksum mismatch: a `defer` read a named return that
  `return "", err` had already cleared. Fixed by moving the stream/verify step
  into `streamVerified` and cleaning up from a local variable.
* `Perm()&0o100` is always zero on Windows; the executable-bit assertion is
  skipped there.
* Moving a task or feature to `done` relocates it to `.backlogit/archive/`, so
  the closure sees already-archived members.

## Next steps

1. Run `autoharness gate pipeline-topology --phase lifecycle` and
   `autoharness gate pre-review --base main`.
2. Push, open the feature PR with a Local Review Readiness block, wait for CI and
   `copilot-review`, then merge with a merge commit.
3. Post-merge: closure branch, `backlogit shipment ship 016-S` (P-015 cascade),
   archive `005-DL`, finalize the closure record, P-020 compaction, closure PR.
