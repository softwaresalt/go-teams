---
title: Rename CLI executable, entrypoint package, release assets, and self-updater contract from teams-cli to go-teams
description: Deliberation on stash entry 66488961 for covering feature 027-F and shipment 027-S
topic: Rename CLI Executable and Release Contract from teams-cli to go-teams (Wave 2 — Option B)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 016-DL
feature: 027-F
shipment: 027-S
source_stash:
  - 66488961
linked_artifacts:
  - docs/plans/2026-11-15-go-teams-binary-and-release-rename-plan.md
  - .backlogit/queue/016-DL.md
  - .backlogit/queue/027-F.md
  - .backlogit/queue/027-S.md
date: 2026-11-15
status: accepted
tags:
  - cli
  - updater
  - config
  - workflows
  - release
  - docs
---

## Problem Frame

Repository `github.com/softwaresalt/go-teams` currently compiles and publishes its command-line binary under the legacy name `teams-cli` (`teams-cli.exe` on Windows) from `./cmd/teams-cli`. That divergence between the repository/module identity (`go-teams`) and the binary/asset identity (`teams-cli`) spans six coupled surfaces:

1. **Entrypoint package path**: `./cmd/teams-cli/main.go` causes `go build ./cmd/...` and `go install github.com/softwaresalt/go-teams/cmd/teams-cli@latest` to emit `teams-cli[.exe]` instead of `go-teams[.exe]`.
2. **CI and release workflows**: `.github/workflows/ci.yml` compiles `bin/teams-cli.exe` from `./cmd/teams-cli`, and `.github/workflows/release.yml` builds, uploads, and verifies `SHA256SUMS` against `teams-cli-<goos>-<goarch>[.exe]` across `{linux, darwin, windows} x {amd64, arm64}`.
3. **Self-updater contract**: `internal/updater/updater.go` computes `AssetName(goos, goarch)` as `teams-cli-<goos>-<goarch>[.exe]`, creates temporary downloads with `tempPattern = ".teams-cli-update-*"`, sends `User-Agent: teams-cli-updater`, and formats lock recovery diagnostics referencing `teams-cli update`.
4. **Configuration defaults and generated template**: `internal/config/config.go` sets `appDirName = "teams-cli"` (resolving `%APPDATA%\teams-cli\config.yaml`, `~/.config/teams-cli/config.yaml`, and `%LOCALAPPDATA%\teams-cli\profile`), and `internal/config/template.go` (`TemplateYAML()`) together with `config.example.yaml` document `teams-cli` paths and commands.
5. **Cobra CLI surface and batch queue parser**: `internal/cli/root.go`, `auth.go`, `join.go`, `schedule.go`, `update.go`, and `generate_config.go` declare `Use: "teams-cli ..."`, help/example text, placeholder note banners, and `update` status output (`Updated teams-cli...`, `teams-cli is up to date...`), while `internal/cli/queue.go` strips only `teams-cli` and `teams-cli.exe` leading tokens from batch queue lines.
6. **Documentation**: `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md` reference `teams-cli`, `cmd/teams-cli`, and `teams-cli-<goos>-<goarch>[.exe]`.

Stash entry **`66488961`** (`feature`, `high`) requires renaming the compiled and released CLI executable to `go-teams` / `go-teams.exe` across all six surfaces while preserving backward compatibility for existing batch queue files that still begin lines with `teams-cli` or `teams-cli.exe`.

### Scope Boundaries

* **In scope**: `cmd/go-teams/main.go` (replacing `cmd/teams-cli/main.go`), `internal/updater`, `internal/config`, `internal/cli`, `internal/version`, `config.example.yaml`, `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md`.
* **Out of scope**:
  * Active stash entries `ED249B2F` (`.env.local.test` test helper loader) and `3EB89E2D` (`022-S` live-tenant follow-up) remain untouched in the stash.
  * Queued shipment `010-S` (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) remains untouched.
  * Historical graduated records in `docs/decisions/`, `docs/plans/`, `docs/closure/`, and `docs/design-docs/` from prior shipments are point-in-time records and are not rewritten.

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: Root CLI help `Note:` banner in `installRootHelpHandler` (`internal/cli/root.go`) is security-sensitive and pinned by regression tests in `internal/cli/help_test.go`. Updating `helpNoteTail` and the `Note:` banner from `teams-cli <subcommand> --help` to `go-teams <subcommand> --help` must preserve `redactHelpArg` prefix dispatch (`http://` / `https://` to `teams.RedactArgs`, credential-delimiter inputs to `[redacted]`) without alteration.
* **`docs/compound/build-errors/golangci-lint-go-version-mismatch.md`**: When editing `.github/workflows/ci.yml` and `.github/workflows/release.yml`, preserve `install-mode: goinstall` on `golangci/golangci-lint-action` steps so Go 1.26 builds pass in CI.
* **`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`**: Tests in `internal/config/template_test.go` and `internal/config/gitignore_test.go` locate the repository root via `findRepoRoot(t)` relative to the package working directory; `TestExampleConfigMatchesTemplateYAML` enforces byte-for-byte parity (after CRLF/LF normalization) between `config.TemplateYAML()` and `config.example.yaml`.

### Codebase Coupling Analysis

1. **Updater and Release Workflow Coupling (`internal/updater/updater.go` <-> `.github/workflows/release.yml`)**:
   * `updater.AssetName(goos, goarch)` returns `<binary>-<goos>-<goarch>` (with `.exe` when `goos == "windows"`).
   * `.github/workflows/release.yml` builds `-o "dist/go-teams-${GOOS}-${GOARCH}${EXT}" ./cmd/go-teams`, uploads artifact `go-teams-${{ matrix.goos }}-${{ matrix.goarch }}`, runs `find . -maxdepth 1 -type f -name 'go-teams-*' -print0 | sort -z | xargs -0 -r sha256sum > SHA256SUMS`, and verifies that all six `go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]` entries exist in `SHA256SUMS`.
   * Any mismatch between `updater.AssetName` and `release.yml` breaks `go-teams update` with `ErrAssetNotFound` or `ErrChecksumMissing`.
2. **Queue File Backward Compatibility (`internal/cli/queue.go`)**:
   * `runQueueFile` currently inspects `firstBase := strings.ToLower(filepath.Base(lineArgs[0]))` and strips `"teams-cli"` or `"teams-cli.exe"`.
   * Operators upgrading from `teams-cli` may have existing `transcription_queue.txt` files containing lines prefixed with `teams-cli` or `teams-cli.exe` as well as new lines prefixed with `go-teams` or `go-teams.exe` (or full paths such as `C:\Tools\go-teams.exe` or `.\bin\teams-cli.exe`). Normalizing backslashes before `filepath.Base` (`firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))`) and supporting both pairs (`go-teams`, `go-teams.exe`, `teams-cli`, `teams-cli.exe`) preserves 100% backward compatibility across Windows, Linux, and macOS.
   * If a queue line consists solely of the binary name with no trailing arguments, `lineArgs` becomes empty after stripping; guarding `if len(lineArgs) == 0 { continue }` after token stripping prevents executing a no-arg root command on an empty remainder.
3. **Configuration Directory and Template Parity (`internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`)**:
   * `internal/config/config.go` defines `const appDirName = "teams-cli"`, which drives `DefaultConfig()`, `DefaultConfigPath()`, and `defaultProfileDir()`. Updating `appDirName = "go-teams"` aligns the runtime defaults with the renamed executable (`%APPDATA%\go-teams\config.yaml`, `~/.config/go-teams/config.yaml`, and `%LOCALAPPDATA%\go-teams\profile`) and matches the updated comments and canonical `# profile_dir: "%LOCALAPPDATA%\\go-teams\\profile"` example in `internal/config/template.go` and `config.example.yaml`. Existing operators upgrading from `teams-cli` should rename/move their `%APPDATA%\teams-cli` and `%LOCALAPPDATA%\teams-cli\profile` (`~/.config/teams-cli`) directories to `go-teams` (or re-run `go-teams auth` and securely delete the orphaned `teams-cli\profile` folder).
4. **Cobra Help and Update Output Assertions (`internal/cli/help_test.go`, `internal/cli/update_test.go`)**:
   * `internal/cli/help_test.go` asserts specific substrings in `root`, `auth`, and `update` help output (`"go-teams join --help"`, `"%LOCALAPPDATA%\\go-teams\\profile"`, `"go-teams-<goos>-<goarch>"`) and checks `helpNoteTail` on the root placeholder banner.
   * `internal/cli/update.go` (`printUpdateResult`) prints human-readable status lines (`"Update available: ... Run `go-teams update...`"`, `"Use `go-teams update --version %s --force` to downgrade."`, `"go-teams is up to date (%s)."`, `"Updated go-teams from %s to %s."`, `"go-teams is already up to date (%s). Use --force to reinstall."`), which are pinned by `internal/cli/update_test.go`.

## Options Evaluated

### Option A: Complete Canonical Rename to `go-teams` with Legacy Queue Token Compatibility (Recommended)

* **Description**:
  * Move `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` and remove `cmd/teams-cli`.
  * Update `internal/updater` (`AssetName` -> `go-teams-<goos>-<goarch>[.exe]`, `tempPattern` -> `".go-teams-update-*"`, `User-Agent` -> `"go-teams-updater"`, lock diagnostics -> `go-teams update`).
  * Update `internal/config` (`appDirName = "go-teams"`, `template.go`) and `config.example.yaml`.
  * Update `internal/cli` (`Use: "go-teams ..."`, help/examples, placeholder banner, `update` messages) while accepting both `go-teams` / `go-teams.exe` and legacy `teams-cli` / `teams-cli.exe` leading tokens in `internal/cli/queue.go`.
  * Update `.github/workflows/ci.yml` and `.github/workflows/release.yml` to build `./cmd/go-teams` and emit `bin/go-teams.exe` and `dist/go-teams-<goos>-<goarch>[.exe]`.
  * Update `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md`.
* **Pros**:
  * Complete alignment between module name (`github.com/softwaresalt/go-teams`), entrypoint package (`./cmd/go-teams`), compiled binary (`go-teams` / `go-teams.exe`), release assets (`go-teams-<goos>-<goarch>[.exe]`), config directory (`go-teams`), and documentation.
  * Preserves the single-entrypoint architecture documented in `docs/ARCHITECTURE.md` and avoids dead/duplicate `main` packages.
  * Preserves backward compatibility for existing batch queue files (`--queue`) containing `teams-cli` or `teams-cli.exe` prefixes.
* **Cons**:
  * Touching 6 surfaces requires sequenced, domain-isolated tasks so every intermediate commit compiles and passes `go test ./...`.
* **Effort estimate**: Medium (6 atomic tasks, ~0.5–1.5 hours each).
* **Fit**: Matches every requirement in stash `66488961`.

### Option B: Keep `cmd/teams-cli` Entrypoint Path and Rename Only `-o` Build Outputs

* **Description**: Leave `cmd/teams-cli/main.go` in place and change only `-o bin/go-teams.exe` and `-o dist/go-teams-...` in CI/release workflows.
* **Pros**: Fewer file moves in `cmd/`.
* **Cons**: Running `go build ./cmd/...` or `go install github.com/softwaresalt/go-teams/cmd/teams-cli@latest` still produces a `teams-cli` binary, leaving the package path out of sync with the executable name and contradicting stash `66488961` (`cmd/ package path or build output` and `docs/ARCHITECTURE.md` / `docs/QUALITY_SCORE.md` updates).
* **Effort estimate**: Low.
* **Fit**: Poor — leaves permanent naming drift between `cmd/` and the binary.

### Option C: Maintain Dual `cmd/go-teams` and `cmd/teams-cli` Entrypoints

* **Description**: Create `cmd/go-teams/main.go` while keeping `cmd/teams-cli/main.go` as a second `main` package.
* **Pros**: `go run ./cmd/teams-cli` continues to work temporarily.
* **Cons**: Violates the single-entrypoint invariant in `docs/ARCHITECTURE.md` ("The module contains one entrypoint and ten packages under `internal/`") and `AGENTS.md` ("No dead code: Placeholder modules replaced or removed before completion").
* **Effort estimate**: Medium.
* **Fit**: Poor — introduces unnecessary maintenance duplication.

## Trade-off Comparison

| Criterion | Option A: Canonical `go-teams` Rename + Queue Compat | Option B: `-o` Output Rename Only | Option C: Dual `cmd/` Entrypoints |
|---|---|---|---|
| `go install` / `go build` binary name | `go-teams` / `go-teams.exe` | `teams-cli` / `teams-cli.exe` (broken without `-o`) | Both (ambiguous `./cmd/...`) |
| Single-entrypoint architecture (`docs/ARCHITECTURE.md`) | Preserved (`cmd/go-teams`) | Preserved (`cmd/teams-cli` mismatch) | Violated (two entrypoints) |
| Batch `--queue` backward compatibility | Preserved (`go-teams[.exe]` + `teams-cli[.exe]`) | Preserved | Preserved |
| Self-updater & `release.yml` asset parity | `go-teams-<goos>-<goarch>[.exe]` | `go-teams-<goos>-<goarch>[.exe]` | `go-teams-<goos>-<goarch>[.exe]` |
| Stash `66488961` compliance | Full | Partial | Partial |

## Decision

Adopt **Option A** under covering feature **`027-F`** and shipment **`027-S`** (sequenced after `026-S`), decomposed into 6 width-isolated tasks:

1. **`027.001-T` (U1 — `internal/updater` & `internal/version`)**: Update `internal/updater/updater.go` (`AssetName` returning `go-teams-<goos>-<goarch>[.exe]`, `tempPattern = ".go-teams-update-*"`, `User-Agent: "go-teams-updater"`, lock recovery messages) and `internal/updater/updater_test.go`, plus the package doc comment in `internal/version/version.go`.
2. **`027.002-T` (U2 — `internal/config` & `config.example.yaml`)**: Update `internal/config/config.go` (`appDirName = "go-teams"`), `internal/config/template.go` (`templateYAML`), `config.example.yaml`, `internal/config/config_test.go`, and `internal/config/template_test.go`.
3. **`027.003-T` (U3 — `cmd/go-teams` & `internal/cli/queue.go`)**: Move `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` via `git mv cmd/teams-cli cmd/go-teams` (removing `cmd/teams-cli` and any stale `bin/teams-cli.exe`); update `internal/cli/queue.go` to normalize backslashes (`strings.ReplaceAll(lineArgs[0], `\`, "/")`) before `filepath.Base` and strip leading `go-teams`, `go-teams.exe`, `teams-cli`, and `teams-cli.exe` tokens (and skip lines that become empty after stripping); and update `internal/cli/queue_test.go` and `internal/cli/binary_automation_test.go`.
4. **`027.004-T` (U4 — `internal/cli` Cobra `Use`, `Short`, `Long`, `Example`, `printUpdateResult`, and root help `Note:` banner)**: Update `Use: "go-teams ..."`, `Short`, `Long`, `Example`, `printUpdateResult`, and `installRootHelpHandler` `Note:` banner across `internal/cli/root.go`, `auth.go`, `join.go`, `schedule.go`, `update.go`, and `generate_config.go`, and update `internal/cli/help_test.go`, `internal/cli/update_test.go`, `internal/cli/root_test.go`, and `internal/cli/e2e_test.go`.
5. **`027.005-T` (U5 — `.github/workflows/`)**: Update `.github/workflows/ci.yml` (`go build -o bin/go-teams.exe ./cmd/go-teams` in `test` and `test-windows`), `.github/workflows/release.yml` (`go build ... -o "dist/go-teams-${GOOS}-${GOARCH}${EXT}" ./cmd/go-teams`, upload artifact `go-teams-${{ matrix.goos }}-${{ matrix.goarch }}`, `find . -maxdepth 1 -type f -name 'go-teams-*'`, `SHA256SUMS` 6-entry count and verification loop for the six `go-teams-*` assets, and commented `go-teams-mcp` references), and `.github/workflows/cli-reference-drift.yml` comment.
6. **`027.006-T` (U6 — `README.md`, `docs/`, & `.github/copilot-instructions.md`)**: Update `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, and `.github/copilot-instructions.md` to reference `go-teams`, `go-teams.exe`, `./cmd/go-teams`, `go-teams-<goos>-<goarch>[.exe]`, and `%APPDATA%\go-teams` / `%LOCALAPPDATA%\go-teams\profile`, including operator upgrade guidance for migrating existing `teams-cli` config/profile folders and performing a one-time binary download across the asset rename boundary.

## Rejected Alternatives

* **Option B (`-o` build output rename only)**: Rejected because leaving `cmd/teams-cli` in place causes `go build ./cmd/...` and `go install` to produce `teams-cli[.exe]`, violating the requirement to rename the compiled CLI executable across the project.
* **Option C (Dual `cmd/go-teams` and `cmd/teams-cli` packages)**: Rejected because maintaining two identical `main` packages violates the single-entrypoint domain map in `docs/ARCHITECTURE.md` and the no-dead-code rule in `AGENTS.md`.

## Unresolved Questions

None. All affected files, test assertions, and backward-compatibility seams have been inspected and mapped to implementation units.

## Risks and Mitigations

* **Risk 1 — Cross-package test breakage between `internal/updater` (`AssetName`) and `internal/cli` (`update_test.go`, `help_test.go`)**:
  * *Mitigation*: `internal/cli/update_test.go` calls `updater.AssetName("linux", "amd64")` dynamically rather than hardcoding `"teams-cli-linux-amd64"`, so U1 (`internal/updater` + `internal/version`) passes `go test -race ./...` independently before U3 and U4 update `internal/cli`.
* **Risk 2 — `TestExampleConfigMatchesTemplateYAML` failure if `config.example.yaml` drifts from `internal/config/template.go`**:
  * *Mitigation*: Update `internal/config/template.go`, `config.example.yaml`, and `internal/config/template_test.go` together in U2 (`027.002-T`) so `TestExampleConfigMatchesTemplateYAML` passes on the U2 commit.
* **Risk 3 — `release.yml` `SHA256SUMS` asset name mismatch with `updater.AssetName`**:
  * *Mitigation*: Verify in U5 (`027.005-T`) that the six expected asset names in `.github/workflows/release.yml` (`go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]`) match `updater.AssetName(goos, goarch)` character-for-character and assert an exact 6-entry count on `SHA256SUMS`.
* **Risk 4 — Cross-rename self-update (`teams-cli update`) and legacy per-user profile/config directories**:
  * *Mitigation*: Pre-`027-S` `teams-cli` binaries look up `teams-cli-<goos>-<goarch>[.exe]` and fail closed with `ErrAssetNotFound` against `go-teams-*` releases, and changing `appDirName` to `"go-teams"` shifts the default `%APPDATA%\go-teams` and `%LOCALAPPDATA%\go-teams\profile` (`~/.config/go-teams`) paths without mutating host filesystems during `DefaultConfig()`. Document the one-time binary download / `go install` upgrade step and the profile/config directory rename (or secure deletion of orphaned `teams-cli\profile` credential folders) in `docs/USER_GUIDE.md` and `README.md` under U6 (`027.006-T`).
