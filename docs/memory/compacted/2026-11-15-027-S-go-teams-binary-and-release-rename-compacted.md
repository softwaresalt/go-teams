---
title: Compacted session memory for shipment 027-S (027-F)
description: Consolidated Stage and Ship session memory for renaming the CLI executable, entrypoint package, release assets, and self-updater contract from teams-cli to go-teams
date: 2026-11-15
feature: 027-F
shipment: 027-S
deliberation: 016-DL
pull_request: 68
merge_commit: 8aff656d93b76f11f4e4094fd6e6ce724f784dbf
archived_sources:
  - docs/archive/memory/2026-11-15/stage-027-S-memory.md
  - docs/archive/memory/2026-11-15/027-S-ship-memory.md
status: compacted
---

## Summary of Shipped Work

Shipment **`027-S`** (Feature **`027-F`**, deliberation **`016-DL`**, PR **#68**, merge commit `8aff656d93b76f11f4e4094fd6e6ce724f784dbf`) resolved stash entry **`66488961`** across six width-isolated tasks (`027.001-T`..`027.006-T`):

* **`027.001-T` (`02e1acb`, `internal/updater`, `internal/version`)**: Renamed `updater.AssetName(goos, goarch)` to `go-teams-<goos>-<goarch>[.exe]`, `tempPattern` to `".go-teams-update-*"`, `User-Agent` to `"go-teams-updater"`, and `lockDiagnostic` recovery messages to `go-teams update`; updated package GoDoc comments in `internal/updater/updater.go` and `internal/version/version.go` and unit tests in `internal/updater/updater_test.go`.
* **`027.002-T` (`54a2fa4`, `internal/config`, `config.example.yaml`)**: Renamed `appDirName = "go-teams"` in `internal/config/config.go` (`%APPDATA%\go-teams`, `%LOCALAPPDATA%\go-teams\profile`, `~/.config/go-teams`), updated all 10 `templateYAML` occurrences in `internal/config/template.go` and `config.example.yaml`, and updated `internal/config/config_test.go` and `internal/config/template_test.go`.
* **`027.003-T` (`9ce37d1`, `cmd/go-teams`, `internal/cli/queue.go`)**: Moved `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` via `git mv`, updated `runQueueFile` in `internal/cli/queue.go` to normalize backslashes before `filepath.Base` and strip both `go-teams[.exe]` and legacy `teams-cli[.exe]` leading tokens cross-platform (skipping empty remainders), and updated `internal/cli/queue_test.go` and `internal/cli/binary_automation_test.go`.
* **`027.004-T` (`f0f7453`, `internal/cli`)**: Updated Cobra `Use`, `Short`, `Long`, `Example`, `printUpdateResult`, and `installRootHelpHandler` `Note:` banner across `internal/cli/{root,auth,join,schedule,update,generate_config}.go` and `internal/cli/{help_test,update_test,root_test,e2e_test}.go` while keeping `redactHelpArg` untouched.
* **`027.005-T` (`0bcaebd`, `.github/workflows/`)**: Updated `.github/workflows/ci.yml`, `.github/workflows/release.yml` (6-asset `SHA256SUMS` count and verification loop for `go-teams-*`), and `.github/workflows/cli-reference-drift.yml`.
* **`027.006-T` (`527a7ff` + `8dd6825`, `README.md`, `docs/`, `.github/copilot-instructions.md`, `.autoharness/workspace-profile.yaml`)**: Updated documentation, runtime validator probes (`./cmd/go-teams`), GFM heading anchors, legacy queue token compatibility notes, and operator upgrade guidance.

## Key Learnings Retained

* **Cross-platform backslash normalization for queue tokens**: On Linux/macOS (`ubuntu-latest` CI), `filepath.Base` does not split on `\`; normalizing backslashes via `strings.ReplaceAll(lineArgs[0], `\`, "/")` before `filepath.Base` ensures Windows path-qualified binary tokens (`C:\Tools\go-teams.exe`, `.\bin\teams-cli.exe`) are stripped identically on all platforms.
* **Workspace profile runtime probe coupling**: When renaming a `cmd/` entrypoint package, update `.autoharness/workspace-profile.yaml` (`structure.source_layout` and `runtime_validation.validator_manifest` probe commands) alongside `.github/copilot-instructions.md` and `docs/ARCHITECTURE.md`.
