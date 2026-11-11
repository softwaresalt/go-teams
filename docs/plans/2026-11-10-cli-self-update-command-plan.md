---
title: CLI self-update command implementation plan
description: Implementation plan for 016-F / 016-S adding teams-cli update with GitHub Releases discovery, SHA256SUMS verification, and Windows-safe atomic binary swap
source: docs/decisions/2026-11-10-cli-self-update-command-deliberation.md
deliberation_id: 005-DL
feature: 016-F
shipment: 016-S
date: 2026-11-10
status: approved
tags:
  - cli
  - update
  - release
  - version
  - security
---

## Problem Frame

Following the accepted deliberation in
[2026-11-10-cli-self-update-command-deliberation.md](../decisions/2026-11-10-cli-self-update-command-deliberation.md)
(`005-DL`), stash entry **`A08726BC`** is planned under feature **`016-F`** and
shipment **`016-S`**:

1. **`.github/workflows/release.yml` contract**: The existing release workflow
   builds `teams-cli-${goos}-${goarch}.exe` with `-ldflags` setting
   `Version`, `Commit`, and `BuildDate` in
   `github.com/softwaresalt/go-teams/internal/version` and publishes those
   executables alongside `SHA256SUMS` to GitHub Releases.
2. **Self-update workflow**: Operators need `teams-cli update` (with `--check`,
   `--version`, and `--force`) to query the GitHub Releases API, verify the
   platform asset's SHA-256 checksum against `SHA256SUMS`, and atomically
   replace the current executable (using a `.old` rename dance so running
   Windows `.exe` binaries can self-update cleanly and roll back if the swap
   fails).

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Provide `internal/version` build metadata (`Version`, `Commit`, `BuildDate`) matching `release.yml` ldflags and semver comparison helpers | `A08726BC` | U1 (`016.001-T`) | `go test -race ./internal/version/...` |
| R2: Query GitHub Releases, enforce HTTPS on non-loopback endpoints, verify binary SHA-256 against `SHA256SUMS`, and perform atomic `.old` rename binary replacement with rollback | `A08726BC` | U2 (`016.002-T`) | `go test -race ./internal/updater/...` |
| R3: Expose `teams-cli update` (`--check`, `--version`, `--force`) and `root.Version` in `internal/cli` with injectable test seams | `A08726BC` | U3 (`016.003-T`), U4 (`016.004-T`) | `go test -race ./internal/cli/...` |
| R4: Document `teams-cli update` in `README.md` | `A08726BC` | U5 (`016.005-T`) | Documentation check and secret-hygiene guardrail |

## Implementation Units

### U1 (`016.001-T`) — Create `internal/version` metadata and semver comparison package

* **Domain:** code (Go `internal/version`). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. Create `internal/version/version.go` defining:
     * `var ( Version = "dev"; Commit = "unknown"; BuildDate = "unknown" )`
       matching `.github/workflows/release.yml` `-ldflags`.
     * `Normalize(v string) (string, error)` stripping optional leading `v`/`V`
       and validating `MAJOR.MINOR.PATCH` (with optional pre-release suffix).
     * `Compare(a, b string) (int, error)` comparing two semantic versions
       (`-1`, `0`, `+1`, with pre-release versions ordered before the
       corresponding normal release per SemVer 2.0.0) and `IsNewer(current, candidate string) (bool, error)`
       (treating `"dev"` as upgradable to any valid release version).
  2. Create `internal/version/version_test.go` with table-driven unit tests
     covering `Normalize`, `Compare`, and `IsNewer` (including `"dev"`, `v`-prefixed
     tags, pre-releases, and malformed version strings).
* **Files (2 files):** `internal/version/version.go`,
  `internal/version/version_test.go`.
* **Verify:** `go test -race ./internal/version/...` passes.
* **Depends on:** none.

### U2 (`016.002-T`) — Create `internal/updater` with GitHub Releases lookup, `SHA256SUMS` verification, and atomic `.old` swap

* **Domain:** code (Go `internal/updater`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. Create `internal/updater/updater.go` using only the Go standard library:
     * `Config` / `Updater` with configurable `HTTPClient *http.Client`,
       `APIBaseURL string` (default `"https://api.github.com"`), `Repo string`
       (default `"softwaresalt/go-teams"`), `CurrentVersion string`,
       `GOOS string` (default `runtime.GOOS`), `GOARCH string`
       (default `runtime.GOARCH`), and `ExecutablePath string` (default
       resolved via `os.Executable()` + `filepath.EvalSymlinks`).
     * `AssetName(goos, goarch string) string` returning
       `teams-cli-<goos>-<goarch>.exe` when `goos == "windows"` and
       `teams-cli-<goos>-<goarch>` otherwise.
     * URL validation helper requiring `https://` for all API and asset download
       URLs except loopback hosts (`127.0.0.1`, `localhost`, `::1`) used by
       `httptest.NewServer`.
     * `Check(ctx context.Context, targetVersion string) (Result, error)`
       fetching `/repos/<repo>/releases/latest` (or
       `/repos/<repo>/releases/tags/v<normalized>` when `targetVersion != ""`),
       locating the platform binary asset and `SHA256SUMS` asset, and reporting
       `CurrentVersion`, `LatestVersion`, and `UpdateAvailable`.
     * `Update(ctx context.Context, targetVersion string, force bool) (Result, error)`
       downloading `SHA256SUMS` (bounded by `io.LimitReader`), parsing the
       expected SHA-256 hex digest for `AssetName`, streaming the binary asset
       to a temporary file in `filepath.Dir(ExecutablePath)` while computing
       `sha256.New()`, failing closed (`ErrChecksumMismatch`, removing temp
       file) if the digest does not match, and performing the Windows-safe
       atomic replacement (`os.Remove(oldPath)` -> `os.Rename(exePath, oldPath)`
       -> `os.Rename(tmpPath, exePath)` with rollback `os.Rename(oldPath, exePath)`
       if the second rename fails, followed by best-effort `os.Remove(oldPath)`).
  2. Create `internal/updater/updater_test.go` with `httptest.NewServer` tests
     covering: (a) `Check` and `Update` when already up to date vs. newer
     version available, (b) verified SHA-256 download and `.old` atomic binary
     replacement on a temporary executable file, (c) checksum mismatch rejection
     leaving the original executable untouched and no temp files behind, and
     (d) non-HTTPS non-loopback download URL rejection.
* **Files (2 files):** `internal/updater/updater.go`,
  `internal/updater/updater_test.go`.
* **Verify:** `go test -race ./internal/updater/...` passes.
* **Depends on:** U1 (`016.001-T`).

### U3 (`016.003-T`) — Wire `teams-cli update` subcommand and `root.Version` in `internal/cli`

* **Domain:** code (Go `internal/cli`). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. Create `internal/cli/update.go` implementing `newUpdateCommand(d *deps) *cobra.Command`:
     * `Use: "update"`, `Short: "Update teams-cli to the latest release version"`.
     * Flags: `--check` (`bool`, check for updates without installing),
       `--version` (`string`, update to a specific release version tag), and
       `--force` (`bool`, reinstall even if already on the target version).
     * Calls `d.runUpdate(cmd.Context(), updateOptions{...})` and writes a
       clear human-readable summary to `cmd.OutOrStdout()`.
  2. In `internal/cli/root.go`:
     * Add `runUpdate func(ctx context.Context, opts updateOptions) (updater.Result, error)`
       to `deps` and wire the default implementation in `defaultDeps()` using
       `updater.New(updater.Config{CurrentVersion: version.Version})`.
     * Set `root.Version = version.Version` and register `newUpdateCommand(d)`
       in `root.AddCommand(...)`.
* **Files (2 files):** `internal/cli/update.go`, `internal/cli/root.go`.
* **Verify:** `go build ./...` and `go vet ./...` pass.
* **Depends on:** U1 (`016.001-T`), U2 (`016.002-T`).

### U4 (`016.004-T`) — Add CLI unit tests for `teams-cli update`

* **Domain:** tests (Go `internal/cli` tests). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. Create `internal/cli/update_test.go` testing:
     * `teams-cli update` when an update is applied (`Updated: true`),
     * `teams-cli update --check` reporting available vs. up-to-date status
       without mutating the binary,
     * `teams-cli update --version v1.2.0 --force` forwarding flags to
       `deps.runUpdate` and surfacing errors cleanly.
  2. In `internal/cli/root_test.go`, update `TestRootHasSubcommands` to include
     `"update"` alongside `"auth"`, `"join"`, `"captions"`, and `"schedule"`.
* **Files (2 files):** `internal/cli/update_test.go`,
  `internal/cli/root_test.go`.
* **Verify:** `go test -race ./internal/cli/...` passes.
* **Depends on:** U3 (`016.003-T`).

### U5 (`016.005-T`) — Document `teams-cli update` in `README.md`

* **Domain:** docs (Markdown). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **What:** Update `README.md` (`## Commands` list and a new `### Self-update`
  subsection) documenting `teams-cli update`, `teams-cli update --check`,
  `teams-cli update --version <tag>`, and `teams-cli update --force`.
* **Files (1 file):** `README.md`.
* **Verify:** Markdown conventions (`markdown.instructions.md`) and
  `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` pass.
* **Depends on:** U3 (`016.003-T`).

## Dependency Graph

```text
U1 (016.001-T: internal/version)
  └──> U2 (016.002-T: internal/updater)
         └──> U3 (016.003-T: internal/cli update command)
                ├──> U4 (016.004-T: internal/cli unit tests)
                └──> U5 (016.005-T: README.md documentation)
```

* Shipment `016-S` is chained after `015-S` via
  `backlogit dep add 016-S 015-S --type blocks`.

## Decisions and Rationale

* **Zero new third-party dependencies**: Uses Go standard library packages
  (`net/http`, `crypto/sha256`, `encoding/hex`, `encoding/json`, `os`,
  `path/filepath`) to keep `go.mod` minimal and auditable.
* **Mandatory `SHA256SUMS` verification**: Never replaces the executable with an
  unverified download; fails closed if `SHA256SUMS` is missing or the digest
  does not match.
* **Windows `.old` rename dance with rollback**: Avoids `EACCES` / sharing
  violations when updating a running `.exe` on Windows and guarantees the prior
  binary is restored if moving the new binary into place fails.

## Risks and Caveats

* **File system permissions on installation directory**: If `teams-cli.exe` is
  installed in a privileged directory (e.g. `C:\Program Files`) and run by a
  non-elevated user, `os.CreateTemp` in the executable directory will return a
  permission error before any download or rename occurs.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Pure Go standard library, explicit error wrapping, bounded `io.LimitReader`, zero lint/vet findings |
| II: Test first | Unit tests in `internal/version`, `internal/updater`, and `internal/cli` run offline via `httptest.NewServer` under `-race` |
| III, IV: Secrets & containment | HTTPS enforced on non-loopback URLs; temp files created in executable dir and cleaned up on failure |
| VI: Single responsibility | Five tasks (`<= 2` files each) isolated by domain (`code`, `code`, `code`, `tests`, `docs`) |
| VII, VIII: Strict safety | Network download and executable replacement classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `005-DL`, plan, feature `016-F`, tasks `016.001-T`–`016.005-T`, and shipment `016-S` (`blocks 015-S`) recorded |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: adds top-level CLI
  subcommand `teams-cli update` and `--version`.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  downloads and replaces the local executable binary from GitHub Releases.
* Migration, backfill, destructive data/config action, or irreversible step —
  **present (executable replacement)**: overwrites the installed binary on disk.
* External integration, operator checkpoint, or external dependency — **present**:
  integrates with GitHub Releases HTTPS API and asset downloads.
* High runtime, rollout, or rollback risk — **moderate**: mitigated by `.old`
  backup and rollback on rename failure.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because `teams-cli update` downloads an executable over the
network and replaces the local binary on disk.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/ci-security.instructions.md`
  * `.github/workflows/release.yml`

### Protected Invariants

1. **HTTPS-only non-loopback transport**: `internal/updater` rejects any API or
   asset download URL whose scheme is not `https` unless the host is a loopback
   address (`127.0.0.1`, `localhost`, `::1`).
2. **Verify-before-swap integrity**: The downloaded binary's SHA-256 digest MUST
   match the entry in `SHA256SUMS` before `ExecutablePath` is renamed or
   modified. On any mismatch or I/O error, the temporary file is removed and the
   existing binary remains untouched.
3. **Rollback on swap failure**: If renaming the verified temp file to
   `ExecutablePath` fails after `ExecutablePath` was renamed to
   `ExecutablePath + ".old"`, the updater immediately renames
   `ExecutablePath + ".old"` back to `ExecutablePath`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Download release metadata, `SHA256SUMS`, and binary asset from GitHub Releases in `internal/updater` | high | Enforces HTTPS on non-loopback hosts, bounds read sizes via `io.LimitReader`, and verifies SHA-256 checksum before touching `ExecutablePath` | Verified release binary staged in temporary file next to executable |
| Replace installed `teams-cli` executable via `.old` rename dance in `internal/updater` | high | Renames active binary to `<exe>.old` first, moves verified temp file into `<exe>`, and rolls back from `<exe>.old` if the move fails | Installed binary atomically updated to target release version |

## Runtime Verification and Closure

* **Verification**:
  * `go test -race ./...` (covering `internal/version`, `internal/updater`, and
    `internal/cli`)
  * `go vet ./...` and `go vet -tags=integration ./...`
  * CI-pinned `golangci-lint run` (`v1.64.8` with `GOTOOLCHAIN=go1.26.0`)
  * `gofmt -l cmd internal`
  * Compiled binary smoke check: `teams-cli.exe --version` and
    `teams-cli.exe update --help`
* **Operational closure**:
  * Monitor `teams-cli update --check` and `teams-cli update` against loopback
    and published GitHub Releases.
  * **Rollback**: Restore `<exe>.old` if present, run
    `teams-cli update --version <prior-tag> --force`, or revert the `016-S`
    merge commit on `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — All 5 units obey the 2-hour rule (`<= 2` files each), width isolation, test-first posture, and zero third-party dependency additions. | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Clean package separation (`internal/version`, `internal/updater`, `internal/cli`), context-aware HTTP requests, and `io.LimitReader` bounds. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly bounded to `A08726BC`; matches existing `.github/workflows/release.yml` asset and `SHA256SUMS` layout without modifying CI workflows. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Compatible with Go 1.26 and `golangci-lint` v1.64.8. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — `internal/updater` depends only on `internal/version` and stdlib; `internal/cli` injects `runUpdate` on `deps` for fast, hermetic CLI tests. | PASS |
| Security Lens Reviewer (binary update & network surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Enforces HTTPS on non-loopback hosts, mandatory `SHA256SUMS` digest verification before swap, bounded downloads, and rollback-safe `.old` rename. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `016-F` and shipment `016-S`.
