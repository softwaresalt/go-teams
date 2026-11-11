---
title: teams-cli update self-update command with SHA-256 verification review and operational closure
date: 2026-11-10
feature: 016-F
shipment: 016-S
branch: feat/016-s-add-teams-cli-update-self-update-command-with-sha-256-verification
status: pending-merge
closure_status: PENDING
compaction_status: pending
merge_commit: pending
pr: pending
---

## Release scope

Shipment `016-S` (`016-F`, `016.001-T` to `016.005-T`) adds a `teams-cli update`
subcommand that discovers a GitHub Release of `softwaresalt/go-teams`, verifies
the platform binary against the release `SHA256SUMS`, and replaces the running
executable with a rollback-safe `.old` rename sequence.

* `016.001-T` (`internal/version`): `Version`, `Commit`, and `BuildDate` are the
  link-time targets already set by `.github/workflows/release.yml`.
  `Normalize`, `Compare`, and `IsNewer` implement SemVer 2.0.0 precedence
  (pre-release ordering, build metadata ignored, `dev` upgradable to any valid
  release) and return `ErrInvalidVersion` for malformed input.
* `016.002-T` (`internal/updater`): standard-library only. `Check` and `Update`
  query `/releases/latest` or `/releases/tags/v<version>`, require HTTPS for every
  API, asset, and redirect URL (loopback hosts exempt for `httptest`), cap
  metadata (4 MiB), `SHA256SUMS` (1 MiB), and binary (256 MiB) sizes, stream the
  binary to a temp file beside the executable while hashing, and fail closed
  (`ErrChecksumMismatch`, `ErrChecksumMissing`, `ErrAssetNotFound`) before the
  executable is touched. The swap removes a stale `<exe>.old`, renames the current
  binary to `<exe>.old`, moves the verified file into place, restores `<exe>.old`
  if that final move fails, and removes `<exe>.old` best-effort afterwards. A
  release older than the running version is rejected with `ErrDowngrade` unless
  `force` is set.
* `016.003-T` (`internal/cli/update.go`, `internal/cli/root.go`): `update`
  command with `--check`, `--version`, and `--force`, routed through the new
  `deps.runUpdate` seam (production implementation in `defaultRunUpdate`), and
  `root.Version = version.Version` so `teams-cli --version` works.
* `016.004-T` (`internal/cli/update_test.go`, `internal/cli/root_test.go`): unit
  tests for flag forwarding, output, and error propagation, plus end-to-end tests
  that run the real updater against an `httptest.Server` (binary replacement,
  checksum mismatch rejection, downgrade gating, `--force` reinstall).
* `016.005-T` (`README.md`): `## Commands` entry, `### Self-update` section, and
  project layout entries for the two new packages.

Source stash entry `A08726BC` fed this scope under deliberation `005-DL`.

### Scope decisions

* The handoff mentioned archive extraction with zip-slip and tar-slip guards.
  `release.yml` publishes raw executables (`teams-cli-<goos>-<goarch>.exe`) and
  `SHA256SUMS`, with no archive format, so no extractor exists. Adding one would
  be unreachable security-sensitive code outside the plan's contract surface.
* The plan and task records name the version API `Normalize`, `Compare`, and
  `IsNewer` (matching the release link flags), and `deps` is passed by pointer in
  this package. Those names were implemented as written in the approved plan.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`): the multi-persona review
rubric (Constitution, Go, Correctness, Maintainability, Security, Concurrency,
Scope Boundary, Learnings) ran in-session against each task diff, without
transmitting repository content to an external reviewer model.

| Area | Finding | Disposition |
|---|---|---|
| Correctness | In `downloadVerified` an early draft cleaned the temp file in a `defer` that read a named result which `return "", err` had already cleared, so a checksum-mismatch download left `.teams-cli-update-*` behind. The `TestUpdateRejectsChecksumMismatch` and `TestUpdateBinarySizeCap` assertions on directory contents caught it | Fixed in-scope (P-021 C1) by splitting out `streamVerified` and removing the temp file from a local path variable |
| Security | Verify-before-swap: the executable is only renamed after the SHA-256 digest matches the `SHA256SUMS` entry; a missing `SHA256SUMS` asset, a missing platform entry, or a mismatch aborts with the original binary intact | No action |
| Security | `validateURL` requires `https` for non-loopback hosts and is re-applied to every redirect through `CheckRedirect`, so an https release cannot redirect a download to plain http; the lookalike host `127.0.0.1.evil.com` is rejected. Query strings are redacted from error text | No action |
| Security | Downloads are bounded by `io.LimitReader` limits on metadata, checksums, and binary; no credentials or tokens are sent | No action |
| Security | `SHA256SUMS` is served from the same release as the binary and is not signed, so it detects corruption and asset-level tampering but not a compromised release. This matches the approved deliberation scope | Accepted residual risk; see Follow-ups |
| Correctness | Rollback: when the final rename fails the previous binary is restored from `<exe>.old`; if the rollback itself fails, the error names the `.old` path so the operator can recover manually. Tests cover both | No action |
| Correctness | `Update` with `--force` on the current version reinstalls, a downgrade needs `--force`, and `--check` never writes. Pre-release tags order before the release per SemVer | No action |
| Concurrency | No goroutines or shared mutable state; the `rename` and size-cap fields are test seams set before use | No action |
| Maintainability | `deps.runUpdate` keeps CLI tests hermetic, and the updater has one exported constructor with defaults applied in `New` | No action |
| Scope | Changes stay inside the declared files plus two README layout lines (same file as `016.005-T`); no `.github/` or `.autoharness/` file and no `go.mod` change | No action |
| Security | Copilot round 1 (thread `PRRT_kwDOTStvas62Sjmk`): a failed download could return a `*url.Error` whose URL carries signed-URL query tokens | Fixed in-scope (P-021 C1: completes the authorized URL redaction); `open` redacts the query on the returned `*url.Error` while keeping the error chain inspectable. `TestDownloadErrorsRedactSignedURLQuery` covers it |
| Scope | Copilot round 1 (thread `PRRT_kwDOTStvas62SjnB`): two concurrent `teams-cli update` processes can delete each other's `.old` rollback copy and need an inter-process lock | Out of scope (P-021 C1: cross-process locking is a different contract surface from the authorized single-process rename-with-rollback swap, and the plan defines none); captured as deferred entry `2D0BAA19`, no code change made |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62Sjnt`): the `--check` install hint dropped an explicit `--version`, so following it installed a different release | Fixed in-scope (P-021 C1: completes the authorized command output); the hint now includes `--version <tag>` when a target was requested. Covered by `TestUpdateCommandCheckHintKeepsRequestedVersion` |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62SjoE`): `Check` reported an update even when the release lacked the platform binary or `SHA256SUMS` | Fixed in-scope (P-021 C1: the approved plan says `Check` locates both assets); `Check` now returns `ErrAssetNotFound` when an update is available but not installable, without downloading either asset. A current install still needs no assets |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62Sjop`): `Normalize` dropped build metadata, so `--version v1.2.3+build.2` looked up `v1.2.3` | Fixed in-scope (P-021 C1: completes the authorized tag lookup); the exact requested tag is URL-escaped for the lookup, precedence still ignores build metadata, and build metadata is validated against the identifier alphabet |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62Sjpb`): all-digit pre-release identifiers beyond `uint64` were treated as non-numeric and misordered, which could bypass the downgrade guard | Fixed in-scope (P-021 C1: completes the authorized SemVer comparison); numeric identifiers are compared by width then digits without integer conversion |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY_WITH_FOLLOWUPS` (one
accepted residual-risk note, recorded below).

## Follow-ups

* Deferred scope expansion `2D0BAA19` (P-021 C2, provisional priority low,
  requires deliberation): add an inter-process lock around the update swap,
  rollback, and `.old` cleanup so concurrent `teams-cli update` processes cannot
  remove each other's rollback copy. Captured from PR #40 Copilot thread
  `PRRT_kwDOTStvas62SjnB`. Stage owns triage.
* Accepted residual risk: `SHA256SUMS` is not signed (no signature or provenance
  attestation is published by `release.yml`). A compromised release would carry a
  matching checksum file. Adding signature verification is a separate contract
  change to the release workflow and requires deliberation if wanted.
* Residual-risk note: live qualification against a published release is not
  covered by the synthetic `httptest` fixtures and remains an operator check (see
  Monitoring). No release exists yet that carries `SHA256SUMS`, which the
  `release.yml` flow produces on the next `v*.*.*` tag.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Red then green | `internal/version`, `internal/updater`, and `internal/cli` update tests failed against compiling error-returning stubs before implementation and passed after it | PASS |
| Format | `gofmt -l cmd internal` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` (all packages) at the end of every task | PASS |
| Binary smoke | Built with `-ldflags "-X .../internal/version.Version=9.9.9"`; `teams-cli --version` printed `teams-cli version 9.9.9` and `teams-cli update --help` listed `--check`, `--force`, and `--version` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Download release metadata, `SHA256SUMS`, and the binary from GitHub Releases | high | HTTPS-only for non-loopback hosts including redirects, size caps, SHA-256 verified before any change to the installed binary | applied |
| Replace the installed executable through the `.old` rename sequence | high | Operator-invoked only (`teams-cli update`), current binary kept until the swap succeeds, automatic rollback on failure, `--check` is read-only | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| `teams-cli update` on a published release | READY_WITH_CONDITIONS | After the next tagged release, run `teams-cli update --check` and `teams-cli update` once on a Windows build and confirm `--version` reports the new tag |
| `--check` and `--version` (read-only) | READY | Never modifies the executable |
| Failure containment | READY | A checksum mismatch, missing checksum, or insecure URL leaves the installed binary untouched |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the first two updates run from a published release.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Update applies and the new version is reported | `teams-cli --version` after `teams-cli update` | New tag reported | Old version still reported, or a missing executable |
| Checksum verification | CLI error output | None for a published release | `checksum mismatch` or `no checksum entry` on a release built by `release.yml` |
| Leftover files | Executable directory | At most `teams-cli.exe.old` held by Windows until the next update | Stray `.teams-cli-update-*` files |

## Rollback

Trigger rollback on a failed or corrupt in-place update, or on a regression in the
command tree. If an update left the executable missing, rename `<exe>.old` back to
the executable name. Otherwise reinstall a known-good build, run
`teams-cli update --version <prior-tag> --force`, or revert the `016-S` merge
commit on `main`. No data migration is involved.

## Post-merge closure

Pending merge of the feature PR. Post-merge closure (P-015 cascade close, source
artifact cleanup, and P-020 compaction) is recorded here by the closure PR.

## Handoff

Related [decision](../decisions/2026-11-10-cli-self-update-command-deliberation.md)
and [plan](../plans/2026-11-10-cli-self-update-command-plan.md).
