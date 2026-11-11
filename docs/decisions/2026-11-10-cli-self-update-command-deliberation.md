---
title: CLI self-update command decision
description: Deliberation on adding teams-cli update to self-update the binary to the latest GitHub release with SHA-256 verification
topic: Add update command to teams-cli for self-updating to the latest release (A08726BC)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 005-DL
feature: 016-F
shipment: 016-S
source_stash:
  - A08726BC
linked_artifacts:
  - docs/plans/2026-11-10-cli-self-update-command-plan.md
  - .backlogit/queue/005-DL.md
  - .backlogit/queue/016-F.md
  - .backlogit/queue/016-S.md
date: 2026-11-10
status: accepted
tags:
  - cli
  - update
  - release
  - security
---

## Problem Frame

Stash entry **`A08726BC`** (`feature`, `medium`) requests:

> Add an update command to teams-cli to enable self-updating the tool to the
> latest release version.

Operators running `teams-cli` on Windows workstations currently must manually
download release binaries from GitHub Releases or recompile from source. Meanwhile,
`.github/workflows/release.yml` already builds release binaries with version
ldflags (`-X github.com/softwaresalt/go-teams/internal/version.Version=${VERSION}`,
`Commit`, and `BuildDate`) and publishes platform binaries
(`teams-cli-windows-amd64.exe`, `teams-cli-windows-arm64.exe`) alongside a
`SHA256SUMS` manifest to GitHub Releases (`softwaresalt/go-teams`), but the
`internal/version` package and `teams-cli update` subcommand do not yet exist in
the repository.

## Duplicate Detection Record

An unconditional duplicate scan across `.backlogit/stash.jsonl`,
`.backlogit/archive/stash.jsonl`, `.backlogit/queue/`, and
`.backlogit/archive/` confirmed **CLEAN DUPLICATE SCAN** for `A08726BC`.

## Research Findings

* **Release workflow contract (`.github/workflows/release.yml`)**:
  * Injects `Version`, `Commit`, and `BuildDate` into
    `github.com/softwaresalt/go-teams/internal/version`.
  * Publishes raw executable assets named
    `teams-cli-${goos}-${goarch}.exe` (`windows`/`amd64` and `windows`/`arm64`)
    plus a `SHA256SUMS` file containing `sha256sum` entries for every
    `teams-cli-*` binary asset.
* **CLI reference drift workflow (`.github/workflows/cli-reference-drift.yml`)**:
  * The `drift` job in `cli-reference-drift.yml` is currently commented out and
    no `docs/cli-reference/` directory exists; CLI subcommands are documented in
    `README.md` and verified in `internal/cli/root_test.go`
    (`TestRootHasSubcommands`).
* **Windows running-binary replacement constraint**:
  * On Windows, an executing `.exe` file cannot be unlinked (`os.Remove`) or
    truncated in place, but the filesystem allows renaming the running binary to
    a backup path (`<exe>.old`) within the same directory and then renaming a
    newly downloaded, checksum-verified temporary file into `<exe>`, with
    automatic rollback (`<exe>.old` -> `<exe>`) if the second rename fails.

## Options Evaluated

### Option A: Third-party self-update library (`creativeprojects/go-selfupdate` or `minio/selfupdate`)

Add an external self-update dependency to `go.mod`.

* **Pros**: Pre-built GitHub API client.
* **Cons**: Pulls a large dependency tree into `go.mod` for an operation well
  covered by Go's standard library (`net/http`, `crypto/sha256`, `os.Rename`);
  violates `go.instructions.md` (*"Prefer the standard library over third-party
  packages when the standard library solution is adequate"*).
* **Effort**: medium | **Fit**: low.

### Option B: Standard-library `internal/version` + `internal/updater` with mandatory `SHA256SUMS` verification and Windows-safe `.old` rename swap (chosen)

Implement `internal/version` (matching `.github/workflows/release.yml` ldflags
and providing semver normalization and comparison), `internal/updater` using Go's
standard library (`net/http`, `crypto/sha256`, `encoding/hex`, `encoding/json`,
`os`, `path/filepath`, `runtime`), and the `teams-cli update` Cobra subcommand
(`--check`, `--version`, `--force`) in `internal/cli`.

* **Pros**: Zero new third-party dependencies in `go.mod`; aligns with the exact
  asset names (`teams-cli-<goos>-<goarch>[.exe]`) and `SHA256SUMS` format in
  `.github/workflows/release.yml`; enforces HTTPS on non-loopback endpoints and
  mandatory SHA-256 verification before any binary swap; handles Windows
  in-place updates via `.old` rename with rollback on failure; 100% unit-testable
  offline via `httptest.NewServer`.
* **Cons**: None.
* **Effort**: medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (Third-party library) | Option B (Stdlib `internal/version` + `internal/updater`) |
|---|---|---|
| Dependency footprint | Adds external dependencies to `go.mod` | Zero external dependencies (stdlib only) |
| Alignment with `release.yml` | Requires custom adapter for raw `.exe` + `SHA256SUMS` | Native match for `teams-cli-<os>-<arch>.exe` + `SHA256SUMS` |
| Supply-chain & integrity safety | Library-dependent | Enforces HTTPS + mandatory `SHA256SUMS` verification before swap |
| Windows `.exe` self-replacement | Varies | Explicit `.old` rename + rollback on failure |

## Decision

We select **Option B** and decompose the work into feature **`016-F`**
(*Add `teams-cli update` self-update command with SHA-256 verification*) and
shipment **`016-S`** (chained after `015-S` via
`backlogit dep add 016-S 015-S --type blocks`) with five width-isolated tasks:

1. **`016.001-T` (U1, `code` domain — `internal/version`)**: Create
   `internal/version/version.go` and `internal/version/version_test.go` with
   `Version`, `Commit`, `BuildDate`, and semver comparison helpers (`XS` / `low`).
2. **`016.002-T` (U2, `code` domain — `internal/updater`)**: Create
   `internal/updater/updater.go` and `internal/updater/updater_test.go`
   implementing GitHub Releases discovery, HTTPS ingress enforcement,
   `SHA256SUMS` digest verification, and Windows-safe `.old` atomic binary
   replacement with rollback (`S` / `medium`).
3. **`016.003-T` (U3, `code` domain — `internal/cli`)**: Create
   `internal/cli/update.go` (`teams-cli update` with `--check`, `--version`,
   `--force`) and wire it plus `root.Version` in `internal/cli/root.go`
   (`XS` / `low`).
4. **`016.004-T` (U4, `tests` domain — `internal/cli`)**: Add CLI unit tests in
   `internal/cli/update_test.go` and update `TestRootHasSubcommands` in
   `internal/cli/root_test.go` (`XS` / `low`).
5. **`016.005-T` (U5, `docs` domain)**: Document `teams-cli update` in
   `README.md` (`XS` / `trivial`).

## Rejected Alternatives

* **Option A (Third-party self-update library)**: Rejected to keep `go.mod`
  free of unnecessary dependencies per `go.instructions.md`.
* **Replacing the binary without verifying `SHA256SUMS`**: Rejected as a
  supply-chain integrity risk; `release.yml` always publishes `SHA256SUMS`, so
  the updater must fail closed when `SHA256SUMS` or the asset entry is missing
  or mismatches.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Supply-chain / MITM integrity risk**: Mitigated by requiring `https://` for
  all non-loopback API and asset download URLs, bounding download sizes via
  `io.LimitReader`, and verifying the SHA-256 digest against `SHA256SUMS` in a
  temporary file before touching the installed executable.
* **Windows open-executable lock**: Mitigated by renaming the active executable
  to `<exe>.old` before renaming the verified temp binary to `<exe>`, with
  automatic rollback from `<exe>.old` if the final rename fails.
