---
title: Rename CLI executable, entrypoint package, release assets, and self-updater contract from teams-cli to go-teams implementation plan
description: Implementation plan for 027-F / 027-S covering stash entry 66488961
source: docs/decisions/2026-11-15-go-teams-binary-and-release-rename-deliberation.md
deliberation_id: 016-DL
feature: 027-F
shipment: 027-S
source_stash:
  - 66488961
date: 2026-11-15
status: approved
tags:
  - cli
  - updater
  - config
  - workflows
  - release
  - docs
---

## Problem Frame

Following the accepted deliberation in [2026-11-15-go-teams-binary-and-release-rename-deliberation.md](../decisions/2026-11-15-go-teams-binary-and-release-rename-deliberation.md) (`016-DL`), stash entry **`66488961`** (`feature`, `high`) is planned under covering feature **`027-F`** and shipment **`027-S`** (sequenced after `026-S`).

The repository `github.com/softwaresalt/go-teams` currently builds its binary from `./cmd/teams-cli` as `teams-cli` / `teams-cli.exe`, names release assets `teams-cli-<goos>-<goarch>[.exe]`, stores per-user configuration and browser profiles under `teams-cli`, and presents `teams-cli` in Cobra `Use` / help / update strings and documentation. This plan renames the executable, entrypoint package, release asset contract, configuration directory, CLI surface, and documentation to `go-teams` / `go-teams.exe` across six width-isolated, 2-hour-rule-compliant tasks (`027.001-T` through `027.006-T`) while preserving backward-compatible stripping of legacy `teams-cli` and `teams-cli.exe` leading tokens in batch queue files (`internal/cli/queue.go`).

### Explicit Out-of-Scope Boundaries

* Active stash entries **`ED249B2F`** and **`3EB89E2D`** remain untouched in the stash.
* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) remains untouched.
* Historical graduated artifacts in `docs/decisions/`, `docs/plans/`, `docs/closure/`, and `docs/design-docs/` from prior shipments are immutable point-in-time records and must not be modified.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Update `internal/updater/updater.go` so `AssetName(goos, goarch)` returns `go-teams-<goos>-<goarch>` (with `.exe` when `goos == "windows"`), `tempPattern` is `".go-teams-update-*"`, `User-Agent` is `"go-teams-updater"`, and `lockDiagnostic` recovery messages reference `go-teams update`; update `internal/version/version.go` package comment; and update `internal/updater/updater_test.go` fixtures and assertions (including `AssetName` for all 6 release targets, `tempPattern`, `User-Agent`, and `lockDiagnostic` `"go-teams update"` text) | `66488961` | U1 (`027.001-T`) | `go test -race ./...` and `go vet ./...` |
| R2: Update `internal/config/config.go` (`appDirName = "go-teams"` and GoDoc comments), `internal/config/template.go` (all 10 `teams-cli` occurrences in `templateYAML`, including lines 35, 38, 42, 45, 47, 54, 127, 136, 151, 156), and `config.example.yaml` (preserving byte-for-byte parity with `TemplateYAML()`), and update `internal/config/config_test.go` and `internal/config/template_test.go` (including `OutputDir`, `ProfileDir`, `DefaultConfigPath()`, and `assert.NotContains(t, tpl, "teams-cli")`) | `66488961` | U2 (`027.002-T`) | `go test -race ./...` and `go vet ./...` |
| R3: Move `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` via `git mv cmd/teams-cli cmd/go-teams` (ensuring `cmd/teams-cli` and any stale `bin/teams-cli.exe` are removed); update `internal/cli/queue.go` (`runQueueFile`) to normalize backslashes (`strings.ReplaceAll(lineArgs[0], `\`, "/")`) before `filepath.Base` and strip leading `go-teams`, `go-teams.exe`, `teams-cli`, and `teams-cli.exe` tokens (and `continue` if `len(lineArgs) == 0` after stripping); and update `internal/cli/queue_test.go` and `internal/cli/binary_automation_test.go` | `66488961` | U3 (`027.003-T`) | `go build ./cmd/...` and `go test -race ./...` |
| R4: Update Cobra `Use`, `Short`, `Long`, `Example`, `printUpdateResult`, and root help `Note:` banner (`installRootHelpHandler`) from `teams-cli` to `go-teams` across `internal/cli/root.go` (including inline comment at line 212), `internal/cli/auth.go`, `internal/cli/join.go`, `internal/cli/schedule.go`, `internal/cli/update.go`, and `internal/cli/generate_config.go` (preserving `redactHelpArg` prefix dispatch untouched), and update `internal/cli/help_test.go`, `internal/cli/update_test.go`, `internal/cli/root_test.go`, and `internal/cli/e2e_test.go` | `66488961` | U4 (`027.004-T`) | `go test -race ./...` and `go vet ./...` |
| R5: Update `.github/workflows/ci.yml` (`go build -o bin/go-teams.exe ./cmd/go-teams` in `test` and `test-windows`), `.github/workflows/release.yml` (`-o "dist/go-teams-${GOOS}-${GOARCH}${EXT}" ./cmd/go-teams`, artifact upload name `go-teams-${{ matrix.goos }}-${{ matrix.goarch }}`, `find . -maxdepth 1 -type f -name 'go-teams-*'`, `SHA256SUMS` 6-line count + verification loop for all six `go-teams-*` assets, and commented `go-teams-mcp` lines at 160, 211, 212), and `.github/workflows/cli-reference-drift.yml` comment | `66488961` | U5 (`027.005-T`) | Workflow YAML check, zero `teams-cli` matches in `.github/workflows/`, and `go test -race ./...` |
| R6: Update `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, and `.github/copilot-instructions.md` to reference `go-teams` / `go-teams.exe`, `cmd/go-teams`, `go-teams-<goos>-<goarch>[.exe]`, `%APPDATA%\go-teams`, `%LOCALAPPDATA%\go-teams\profile`, and `~/.config/go-teams` (while documenting in `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md` that `--queue` strips both `go-teams[.exe]` and legacy `teams-cli[.exe]` tokens and documenting the one-time upgrade path for existing `teams-cli` binaries and profiles) | `66488961` | U6 (`027.006-T`) | `go test -race ./...` (including `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) |

## Implementation Units

### U1 (`027.001-T`) — Rename self-updater release asset contract (`AssetName`, `tempPattern`, diagnostics) and version package doc to `go-teams`

* **Domain:** code (`internal/updater`, `internal/version`). **Posture:** test-first (co-located `updater_test.go` updated first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Files:** `internal/updater/updater.go`, `internal/version/version.go`, `internal/updater/updater_test.go`.
* **What:**
  1. **Red phase (`internal/updater/updater_test.go`)**: Update unit tests first so they fail against the legacy `teams-cli` asset names and strings:
     * In `TestAssetName`, assert all six published platform targets: `AssetName("windows", "amd64") == "go-teams-windows-amd64.exe"`, `AssetName("windows", "arm64") == "go-teams-windows-arm64.exe"`, `AssetName("linux", "amd64") == "go-teams-linux-amd64"`, `AssetName("linux", "arm64") == "go-teams-linux-arm64"`, `AssetName("darwin", "amd64") == "go-teams-darwin-amd64"`, and `AssetName("darwin", "arm64") == "go-teams-darwin-arm64"`, and assert `tempPattern == ".go-teams-update-*"`.
     * In `fixture.handle` (or `TestCheckReportsUpdateAvailable`), assert `assert.Equal(t, "go-teams-updater", r.Header.Get("User-Agent"))`.
     * In `TestUpdatePausedOrStaleHolderNeverEvictedAndClassifiedInError`, assert `assert.Contains(t, err.Error(), "go-teams update")` and `assert.NotContains(t, err.Error(), "teams-cli")` for both `dead` and `live` subtests (covering both `updater.go:162` and `updater.go:172`).
     * In `TestParseChecksums`, use `./go-teams-windows-amd64.exe` and `*go-teams-windows-arm64.exe`.
     * In `newFixture` and standalone test helpers across `internal/updater/updater_test.go`, change `filepath.Join(..., "teams-cli")` to `filepath.Join(..., "go-teams")`, update `f.dirEntries()` assertions from `"teams-cli"` / `"teams-cli.lock"` to `"go-teams"` / `"go-teams.lock"`, update `TestSwapRejectsTargetsCollidingWithLockMarkerNames` to `{"go-teams.lock", "go-teams.lock.ctl", "go-teams.lock.old", "go-teams.LOCK"}`, and update `TestUpdateWithRealInstalledVersionProbe` to print `"go-teams version"`.
  2. **Green phase (`internal/updater/updater.go`, `internal/version/version.go`)**:
     * In `internal/updater/updater.go`:
       * Update `AssetName(goos, goarch string) string` to build `name := "go-teams-" + goos + "-" + goarch` (appending `".exe"` when `goos == "windows"`).
       * Update `tempPattern = ".go-teams-update-*"`.
       * Update `req.Header.Set("User-Agent", "go-teams-updater")` in `open`.
       * Update both `lockDiagnostic` recovery messages (`recovery` at line 162 and the live-PID error format at line 172) from `"teams-cli update"` to `"go-teams update"`.
       * Update package, `DefaultRepo`, and `Updater` GoDoc comments from `teams-cli` to `go-teams`.
     * In `internal/version/version.go`:
       * Update the package GoDoc comment to `// Package version exposes build metadata for go-teams and semantic version helpers used by the self-update command.`
  3. **Verify:** Run `go test -race ./...` and `go vet ./...`.

### U2 (`027.002-T`) — Rename default per-user config/profile directory and generated YAML template (`TemplateYAML`, `config.example.yaml`) to `go-teams`

* **Domain:** code (`internal/config` + tracked `config.example.yaml` coupled via `TestExampleConfigMatchesTemplateYAML`). **Posture:** test-first (co-located `config_test.go` and `template_test.go` updated first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `027.001-T` (`blocks`).
* **Files:** `internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`, `internal/config/config_test.go`, `internal/config/template_test.go`.
* **What:**
  1. **Red phase (`internal/config/config_test.go`, `internal/config/template_test.go`)**:
     * In `internal/config/config_test.go`: update `TestDefaultConfig` to assert `assert.Contains(t, cfg.OutputDir, filepath.Join("go-teams", "transcripts"))` and `assert.Contains(t, cfg.ProfileDir, filepath.Join("go-teams", "profile"))`; update `TestDefaultConfigPath` to assert `assert.Contains(t, path, filepath.Join("go-teams", "config.yaml"))`; and update `TestLoadExpandsWindowsUserVariables` to use `"%USERPROFILE%\\AppData\\Local\\go-teams\\profile"` and `filepath.Join(home, "AppData", "Local", "go-teams", "profile")`.
     * In `internal/config/template_test.go`: update `TestTemplateYAMLContentAndLoad` to assert `assert.Equal(t, expandUserPath(`%LOCALAPPDATA%\go-teams\profile`), got.ProfileDir)`, `assert.Contains(t, tpl, "# go-teams configuration file")`, and `assert.NotContains(t, tpl, "teams-cli")`; and update the failure message in `TestExampleConfigMatchesTemplateYAML` to reference `go-teams generate-config -d . -n config.example.yaml --force`.
  2. **Green phase (`internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`)**:
     * In `internal/config/config.go`: change `appDirName = "teams-cli"` to `appDirName = "go-teams"`, and update the GoDoc comments on `DefaultConfig` and `defaultProfileDir` to reference `%LOCALAPPDATA%\go-teams` and `%APPDATA%\go-teams`.
     * In `internal/config/template.go`: update all 10 `teams-cli` occurrences in `templateYAML` (`# go-teams configuration file`, `generated and go-teams falls back`, `Where go-teams looks for this file`, `config.yaml next to the go-teams executable`, `%APPDATA%\go-teams\config.yaml`, `per-user go-teams\transcripts folder`, `"go-teams schedule"`, `# profile_dir: "%LOCALAPPDATA%\\go-teams\\profile"`, `go-teams auth`, and `# Scheduled meetings for "go-teams schedule"`).
     * In `config.example.yaml`: synchronize all 10 occurrences with `internal/config/template.go` (`TemplateYAML()`) so `TestExampleConfigMatchesTemplateYAML` passes with LF line endings.
  3. **Verify:** Run `go test -race ./...` and `go vet ./...`.

### U3 (`027.003-T`) — Move entrypoint to `cmd/go-teams` and support cross-platform `go-teams` and legacy `teams-cli` leading tokens in `queue.go`

* **Domain:** code (`cmd/go-teams`, `internal/cli/queue.go`). **Posture:** test-first (co-located `queue_test.go` and `binary_automation_test.go` updated first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `027.001-T`, `027.002-T` (`blocks`).
* **Files:** `cmd/go-teams/main.go` (moved from `cmd/teams-cli/main.go` via `git mv`), `internal/cli/queue.go`, `internal/cli/queue_test.go`, `internal/cli/binary_automation_test.go`.
* **What:**
  1. **Red phase (`internal/cli/queue_test.go`, `internal/cli/binary_automation_test.go`)**:
     * In `internal/cli/queue_test.go`: extend `TestQueueFileProcessesActiveLinesAndCommentsOutCompletedEntries` (or add a dedicated test) verifying that leading `go-teams`, `go-teams.exe`, uppercase `GO-TEAMS.EXE`, Windows path-qualified `C:\Tools\go-teams.exe`, POSIX path-qualified `/usr/local/bin/go-teams`, and legacy `teams-cli` and `teams-cli.exe` tokens are all stripped identically on both Windows and Linux, and that a queue line containing only a bare binary token (`go-teams` or `teams-cli.exe` with no trailing arguments) is skipped cleanly without invoking capture.
     * In `internal/cli/binary_automation_test.go`: update the skip message to `"set GO_TEAMS_BINARY to a freshly built go-teams executable"`.
  2. **Green phase (`cmd/go-teams/main.go`, `internal/cli/queue.go`)**:
     * Move `cmd/teams-cli` to `cmd/go-teams` using `git mv cmd/teams-cli cmd/go-teams` (do not use unapproved destructive shell deletion commands), update the package GoDoc comment in `cmd/go-teams/main.go` to `// Command go-teams is the command-line entrypoint for autonomous Microsoft Teams live-transcription and audio capture.`, confirm `cmd/teams-cli` no longer exists, and remove any stale untracked `bin/teams-cli.exe` build artifact.
     * In `internal/cli/queue.go` (`runQueueFile`): normalize backslashes before `filepath.Base` so Windows paths work on Linux/macOS CI (`firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))`), strip `lineArgs[0]` when `firstBase` is `"go-teams"`, `"go-teams.exe"`, `"teams-cli"`, or `"teams-cli.exe"`, and immediately guard `if len(lineArgs) == 0 { continue }`.
  3. **Verify:** Run `go build ./cmd/...`, confirm `cmd/teams-cli` does not exist, and run `go test -race ./...`.

### U4 (`027.004-T`) — Update Cobra `Use`, `Short`, `Long`, `Example`, `printUpdateResult`, and root help `Note:` banner across `internal/cli`

* **Domain:** code (`internal/cli`). **Posture:** test-first (co-located `help_test.go`, `update_test.go`, `root_test.go`, and `e2e_test.go` updated first per Principle II). **Size:** `M` | **Complexity:** `low`.
* **Depends on:** `027.003-T` (`blocks`).
* **Files:** `internal/cli/root.go`, `internal/cli/auth.go`, `internal/cli/join.go`, `internal/cli/schedule.go`, `internal/cli/update.go`, `internal/cli/generate_config.go`, `internal/cli/help_test.go`, `internal/cli/update_test.go`, `internal/cli/root_test.go`, `internal/cli/e2e_test.go`.
* **What:**
  1. **Red phase (`internal/cli/help_test.go`, `internal/cli/update_test.go`, `internal/cli/root_test.go`, `internal/cli/e2e_test.go`)**:
     * In `internal/cli/help_test.go`: update `helpNoteTail` to list `go-teams join --help, go-teams captions --help, go-teams schedule --help, go-teams auth --help, go-teams generate-config --help, or go-teams update --help`; update `TestAllSubcommandsProvideComprehensiveHelpAndExamples` expected phrases for `root` (`"go-teams join --help"`, `"%APPDATA%\\go-teams\\config.yaml"`, `"~/.config/go-teams/config.yaml"`), `auth` (`"%LOCALAPPDATA%\\go-teams\\profile"`), and `update` (`"go-teams-<goos>-<goarch>"`); and in the subcommand loop assert `assert.NotContains(t, out, "teams-cli")` and `assert.Contains(t, cmd.Example, "go-teams")`, plus assert `root.Use == "go-teams [meeting-url] [output-markdown-path]"`.
     * In `internal/cli/update_test.go`: update assertions in `TestUpdateCommandAppliesUpdate` (`"Updated go-teams from 1.0.0 to 1.2.0"`), `TestUpdateCommandAlreadyUpToDate` (`"go-teams is already up to date (1.2.0)"`), `TestUpdateCommandCheckHintKeepsRequestedVersion` (``"`go-teams update --version v2.0.0-rc.1`"`` and ``"`go-teams update`"``), `TestUpdateCommandCheckReportsAvailability` (`"go-teams is up to date (1.2.0)"`, `assert.NotContains(t, out, "Updated go-teams")`, plus a `"downgrade"` case asserting ``"Use `go-teams update --version v1.1.0 --force` to downgrade."``), `TestUpdateCommandForwardsVersionAndForce`, `TestUpdateEndToEndReplacesBinary`, `TestUpdateEndToEndDowngradeRequiresForce`, `TestUpdateEndToEndConcurrentUpdatesSerialize`, and `newExe` / `newVersionedExe` helper binary names from `teams-cli` to `go-teams`.
     * In `internal/cli/root_test.go` and `internal/cli/e2e_test.go`: update comments referencing `teams-cli` to `go-teams`.
  2. **Green phase (`internal/cli/root.go`, `auth.go`, `join.go`, `schedule.go`, `update.go`, `generate_config.go`)**:
     * In `internal/cli/root.go`: update `deps.runUpdate` comment, `root.Use` (`"go-teams [meeting-url] [output-markdown-path]"`), `root.Long` (`go-teams`, `%APPDATA%\go-teams\config.yaml`, `~/.config/go-teams/config.yaml`), `root.Example` (`go-teams ...`), `installRootHelpHandler` GoDoc comment, inline comment at line 212 (`// "go-teams --help join" renders the subcommand's own help.`), and `Note:` banner (`go-teams <subcommand> --help`). Preserve `redactHelpArg` and `redactedHelpArg` untouched.
     * In `internal/cli/auth.go`: update `Long` (`%LOCALAPPDATA%\go-teams\profile`) and `Example` (`go-teams auth ...`).
     * In `internal/cli/join.go`: update `configAndUsageHelp` (`Run "go-teams generate-config"...`) and `Example` strings on `newJoinCommand` (`go-teams join ...`) and `newCaptionsCommand` (`go-teams captions ...`).
     * In `internal/cli/schedule.go`: update `Example` (`go-teams schedule ...` and `%LOCALAPPDATA%\go-teams\profile`).
     * In `internal/cli/update.go`: update `Short` (`"Update go-teams to the latest release version"`), `Long` (`go-teams`, `go-teams-<goos>-<goarch>`, `go-teams update`), `Example` (`go-teams update ...`), and all 5 branches of `printUpdateResult` (`"go-teams update"`, `"Use `go-teams update --version %s --force` to downgrade.\n"`, `"go-teams is up to date (%s).\n"`, `"Updated go-teams from %s to %s.\n"`, `"go-teams is already up to date (%s). Use --force to reinstall.\n"`).
     * In `internal/cli/generate_config.go`: update `Example` (`go-teams generate-config ...`, `%LOCALAPPDATA%\go-teams`, `go-teams init-config ...`).
  3. **Verify:** Run `go test -race ./...` and `go vet ./...`.

### U5 (`027.005-T`) — Update CI and Release GitHub Actions workflows to build and publish `go-teams`

* **Domain:** config (`.github/workflows/`). **Posture:** migration-first. **Size:** `XS` | **Complexity:** `low`.
* **Depends on:** `027.001-T`, `027.003-T` (`blocks`).
* **Files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/cli-reference-drift.yml`.
* **What:**
  1. In `.github/workflows/ci.yml`:
     * In job `test` (`Build Windows executable`) and job `test-windows` (`Build Windows executable`), change `go build -o bin/teams-cli.exe ./cmd/teams-cli` to `go build -o bin/go-teams.exe ./cmd/go-teams`.
     * Preserve all SHA-pinned actions, `persist-credentials: false`, `permissions: contents: read`, and `install-mode: goinstall` on `golangci-lint-action`.
  2. In `.github/workflows/release.yml`:
     * In job `build` (`Build release executable`), update the comment and build command to `-o "dist/go-teams-${GOOS}-${GOARCH}${EXT}" ./cmd/go-teams`, and in `Upload artifacts` change `name: teams-cli-${{ matrix.goos }}-${{ matrix.goarch }}` to `name: go-teams-${{ matrix.goos }}-${{ matrix.goarch }}`.
     * In the commented-out `npm-package` and `npm-publish` blocks (lines 160, 211, 212), replace `teams-cli-mcp` with `go-teams-mcp` so zero `teams-cli` strings remain anywhere in `.github/workflows/`.
     * In job `release` (`Generate SHA256SUMS`), change `find . -maxdepth 1 -type f -name 'teams-cli-*'` to `find . -maxdepth 1 -type f -name 'go-teams-*'`, verify `test "$(wc -l < SHA256SUMS)" -eq 6`, and update the `for expected in` verification list to `go-teams-linux-amd64`, `go-teams-linux-arm64`, `go-teams-darwin-amd64`, `go-teams-darwin-arm64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`.
  3. In `.github/workflows/cli-reference-drift.yml`:
     * Update the comment referencing `teams-cli` to `go-teams`.
  4. **Verify:** Confirm all six asset names in `.github/workflows/release.yml` match `updater.AssetName(goos, goarch)`, verify zero `teams-cli` matches remain in `.github/workflows/`, and run `go test -race ./...`.

### U6 (`027.006-T`) — Update `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, and `.github/copilot-instructions.md` for `go-teams`

* **Domain:** docs (`README.md`, `docs/`, `.github/copilot-instructions.md`). **Posture:** characterization-first. **Size:** `M` | **Complexity:** `low`.
* **Depends on:** `027.002-T`, `027.003-T`, `027.004-T`, `027.005-T` (`blocks`).
* **Files:** `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`, `.github/copilot-instructions.md`.
* **What:**
  1. In `README.md`:
     * Replace `teams-cli` / `teams-cli.exe` / `./cmd/teams-cli` references with `go-teams` / `go-teams.exe` / `./cmd/go-teams`, including section headings (`## Why go-teams exists`), quick-start build and invocation commands (`go build -o .\bin\go-teams.exe .\cmd\go-teams`, `.\bin\go-teams.exe ...`), the Commands table, help examples, self-update section, configuration section, and the cross-file anchor link `[Built-in daemon scheduling](docs/USER_GUIDE.md#built-in-daemon-scheduling-go-teams-schedule)`.
  2. In `docs/USER_GUIDE.md`:
     * Update frontmatter (`title: go-teams user guide`, `description: ... for go-teams`), table of contents (`[Why go-teams exists](#why-go-teams-exists)` at line 11 and `## Why go-teams exists` at line 30), build commands (`go build -o .\bin\go-teams.exe .\cmd\go-teams`, `go run ./cmd/go-teams ...`), CLI invocations (`go-teams` / `.\bin\go-teams.exe`, including `generate-config -d "%LOCALAPPDATA%\go-teams"` at line 186 and queue example `go-teams.exe captions ...` at line 394), headings `### Built-in daemon scheduling (`go-teams schedule`)` (line 482) and `### Windows Task Scheduler one-shot recipes (`go-teams join --duration`)` (line 564/571), internal anchor link `[Built-in daemon scheduling (go-teams schedule)](#built-in-daemon-scheduling-go-teams-schedule)` (line 797/826), per-user config and profile paths (`%APPDATA%\go-teams`, `%LOCALAPPDATA%\go-teams\profile`, `go-teams/profile` at line 617, `/home/user/.config/go-teams/transcripts` and `/home/user/.config/go-teams/profile` at lines 752/767 and 756/771), release asset names (`go-teams-<goos>-<goarch>[.exe]`), backup/lock examples (`go-teams.exe.old`), Task Scheduler examples (`C:\Tools\go-teams\go-teams.exe`), project layout (`cmd/go-teams/`), and compiled-binary test instructions (`.\bin\go-teams.exe`).
     * In the `Batch queue mode` section, document that a leading `go-teams` or `go-teams.exe` token (as well as a legacy `teams-cli` or `teams-cli.exe` token) is stripped so existing queue files continue to work unchanged.
     * In the `Self-update` and `Configuration` sections, include a concise upgrade note for existing `teams-cli` operators: (a) perform a one-time binary download or `go install github.com/softwaresalt/go-teams/cmd/go-teams@latest` when upgrading across the `teams-cli-*` -> `go-teams-*` release asset rename, and (b) rename/move `%APPDATA%\teams-cli` -> `%APPDATA%\go-teams` and `%LOCALAPPDATA%\teams-cli\profile` -> `%LOCALAPPDATA%\go-teams\profile` (`~/.config/teams-cli` -> `~/.config/go-teams`), or re-run `go-teams auth` and securely delete the orphaned `teams-cli\profile` directory so cached Microsoft 365 tokens are not left behind.
  3. In `docs/ARCHITECTURE.md`:
     * Update frontmatter description (`for go-teams`), `## System overview` (`go-teams` executable), `## Domain map` and `## Dependency direction` (`cmd/go-teams`), `## Runtime flows` (`go-teams auth`, `go-teams --queue`, `go-teams schedule`, `go-teams update`, `go-teams generate-config`), and note that `runQueueFile` strips an optional leading `go-teams` / `go-teams.exe` (or legacy `teams-cli` / `teams-cli.exe`) token.
  4. In `docs/QUALITY_SCORE.md` and `.github/copilot-instructions.md`:
     * Update the `cmd/teams-cli` row in `docs/QUALITY_SCORE.md` (`## Grades by package`) to `cmd/go-teams` and update the Tier 2 `TestCompiledBinaryCapturesRecapToExplicitOutput` prose to `go-teams`.
     * Update `└── teams-cli/` in `.github/copilot-instructions.md` (`## Project Structure`, line 26) to `└── go-teams/`.
  5. **Verify:** Confirm via grep that the only remaining `teams-cli` occurrences in `README.md`, `docs/{USER_GUIDE,ARCHITECTURE,QUALITY_SCORE}.md`, and `.github/copilot-instructions.md` are the intentional legacy `--queue` compatibility and upgrade notes, run `go test -race ./...` (verifying `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `TestExampleConfigMatchesTemplateYAML`), and run `engram --workspace "C:\Source\GitHub\go-teams" verify` on modified markdown files.

## Dependency Graph

```text
027.001-T (U1: internal/updater + internal/version)
    |
    +---> 027.002-T (U2: internal/config + config.example.yaml)
    |         |
    +---------+---> 027.003-T (U3: cmd/go-teams + internal/cli/queue.go)
                        |
                        +---> 027.004-T (U4: internal/cli Cobra Use/Short/Long/Example/printUpdateResult + help_test.go/update_test.go)
                        |         |
                        +---> 027.005-T (U5: .github/workflows/{ci,release,cli-reference-drift}.yml)
                                  |
                                  +---> 027.006-T (U6: README.md + docs/{USER_GUIDE,ARCHITECTURE,QUALITY_SCORE}.md + .github/copilot-instructions.md)
```

* Shipment sequencing: `027-S` depends on `026-S` (`backlogit dep add 027-S 026-S --type blocks` — `026-S` is already shipped).

## Decisions and Rationale

1. **Canonical entrypoint move (`cmd/teams-cli` -> `cmd/go-teams` via `git mv`)**: Moving `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` ensures `go build ./cmd/go-teams` and `go install github.com/softwaresalt/go-teams/cmd/go-teams@latest` produce `go-teams[.exe]` without requiring `-o`, while preserving the single-entrypoint invariant in `docs/ARCHITECTURE.md`.
2. **Cross-platform backward-compatible queue token stripping (`internal/cli/queue.go`)**: Normalizing backslashes before `filepath.Base` (`firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))`) and supporting both `go-teams` / `go-teams.exe` and legacy `teams-cli` / `teams-cli.exe` ensures existing `transcription_queue.txt` files (including Windows path-qualified invocations run on POSIX CI or Linux hosts) continue to work without manual editing.
3. **Six-unit decomposition with zero file overlap**: Splitting `internal/updater` (U1), `internal/config` (U2), `cmd/go-teams` + `queue.go` (U3), `internal/cli` Cobra command/update strings (U4), `.github/workflows/` (U5), and `docs/` (U6) eliminates file overlap between U3 and U4, keeps every task within a single skill domain, and guarantees that `go test -race ./...` passes across the entire repository after every single task commit.

## Risks and Caveats

* **Cross-reference drift between `README.md` and `docs/USER_GUIDE.md` heading anchors**: Changing `## Why teams-cli exists`, `### Built-in daemon scheduling (teams-cli schedule)`, and `### Windows Task Scheduler one-shot recipes (teams-cli join --duration)` in `docs/USER_GUIDE.md` changes their GitHub-flavored Markdown anchor slugs to `#why-go-teams-exists`, `#built-in-daemon-scheduling-go-teams-schedule`, and `#windows-task-scheduler-one-shot-recipes-go-teams-join---duration`. Both the internal TOC/body links in `docs/USER_GUIDE.md` and the cross-file links in `README.md` must be updated in lockstep in U6 (`027.006-T`).
* **Updater lock and backup suffixes**: Only `AssetName`, `tempPattern` (`".go-teams-update-*"`), `User-Agent` (`"go-teams-updater"`), and diagnostic messages change in `internal/updater/updater.go`. The lock and backup suffixes (`.old`, `.lock`, `.lock.ctl`) are appended to the resolved runtime executable path (`exe + lockSuffix`) and remain unchanged.
* **Cross-rename self-update cutover (`teams-cli update` -> `go-teams`)**: Pre-`027-S` `teams-cli` binaries compile `teams-cli-<goos>-<goarch>[.exe]` into `updater.AssetName`, so running `teams-cli update` against a release that publishes `go-teams-<goos>-<goarch>[.exe]` fails closed with `ErrAssetNotFound` (and `go-teams update --version <pre-027-tag> --force` cannot downgrade to a pre-rename release tag). U6 (`027.006-T`) documents that upgrading across the rename boundary requires a one-time binary download or `go install github.com/softwaresalt/go-teams/cmd/go-teams@latest`.
* **Per-user configuration and credential-bearing browser profile cutover (`appDirName = "go-teams"`)**: Changing `appDirName` from `"teams-cli"` to `"go-teams"` shifts the default per-user config directory (`%APPDATA%\go-teams` / `~/.config/go-teams`) and default browser profile directory (`%LOCALAPPDATA%\go-teams\profile` / `~/.config/go-teams/profile`). Because `DefaultConfig()` must stay hermetic and never probe host directories, U6 (`027.006-T`) documents that upgrading operators should rename/move `%APPDATA%\teams-cli` -> `%APPDATA%\go-teams` and `%LOCALAPPDATA%\teams-cli\profile` -> `%LOCALAPPDATA%\go-teams\profile` (or re-run `go-teams auth` and securely delete the orphaned `teams-cli\profile` folder so cached M365 session cookies/tokens are not left on disk).

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, no `unsafe`, zero lint findings, all errors checked |
| II. Test-First Development | Compliant — U1, U2, U3, and U4 specify failing co-located `*_test.go` assertions before production edits |
| III. Workspace Isolation | Compliant — all test paths use `t.TempDir()` and repository-relative `findRepoRoot(t)` |
| IV. CLI Workspace Containment | Compliant — no file operations outside workspace root |
| V. Structured Observability | Compliant — `slog` structured logging and `release-observability` monitoring/rollback contract preserved |
| VI. Single Responsibility | Compliant — zero new third-party dependencies (`go.mod` unchanged); package boundaries preserved |
| VII. Destructive Command Approval | Compliant — moving `cmd/teams-cli` to `cmd/go-teams` uses tracked `git mv cmd/teams-cli cmd/go-teams`; unapproved destructive shell deletion commands are forbidden |
| VIII. Explicit Safety Modes | Compliant — `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory recorded below with `ActionRisk: high` on release/auth-profile actions |
| IX. Git-Friendly Persistence | Compliant — `config.example.yaml`, markdown docs, and backlog artifacts remain LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — bounded reads and targeted symbol/test verification used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Present** — Renames the compiled CLI binary (`go-teams` / `go-teams.exe`), entrypoint package (`./cmd/go-teams`), GitHub Release asset naming contract (`go-teams-<goos>-<goarch>[.exe]`), and default per-user configuration/profile directory (`go-teams`).
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Touches `.github/workflows/ci.yml`, `.github/workflows/release.yml` (`SHA256SUMS` verification loop), `internal/updater/updater.go`, `internal/config/config.go` (`defaultProfileDir` session persistence path), and `internal/cli/root.go` (`installRootHelpHandler` banner next to `redactHelpArg`).
* **Migration, backfill, destructive data/config action, or irreversible step**: **Present** — Moves `cmd/teams-cli/main.go` to `cmd/go-teams/main.go`, shifts `appDirName` to `"go-teams"`, and updates batch queue leading-token stripping (`internal/cli/queue.go`).
* **External integration, operator checkpoint, or external dependency**: **Absent** — No new third-party dependencies or external service integrations.
* **High runtime, rollout, or rollback risk**: **Present** — `updater.AssetName` and `.github/workflows/release.yml` must match exactly so `go-teams update` can verify and install future releases.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed runtime surfaces**:
  * CLI entrypoint and Cobra command tree (`./cmd/go-teams`, `internal/cli`)
  * Self-updater asset lookup and temporary download file naming (`internal/updater`)
  * Default configuration and browser profile directory resolution (`internal/config`)
  * CI and Release build/checksum workflows (`.github/workflows/ci.yml`, `.github/workflows/release.yml`)
* **Runtime verification**:
  * Build `.\bin\go-teams.exe` from `./cmd/go-teams` and verify `cmd/teams-cli` and `bin/teams-cli.exe` do not exist.
  * Verify `.\bin\go-teams.exe --version`, `.\bin\go-teams.exe --help`, `.\bin\go-teams.exe [command] --help` (confirming the `Note:` banner and credential redaction), and `.\bin\go-teams.exe <subcommand> --help` for all six subcommands (`join`, `captions`, `schedule`, `auth`, `update`, `generate-config`).
  * Run `TestCompiledBinaryCapturesRecapToExplicitOutput` with `GO_TEAMS_BINARY` pointing to `.\bin\go-teams.exe`.
  * Run the full race-enabled test suite (`go test -race ./...`), static analysis (`go vet ./...`, `go vet -tags=integration ./...`), linter (`golangci-lint run`), and formatting check (`gofmt -l .`).
* **Operational closure**:
  * Pre-deploy audit checklist, post-deploy observation window (7 days / next tagged release `v*.*.*`), rollback trigger, and releasability evidence contract (`READY`) recorded in the closure artifact `docs/closure/2026-11-15-027-S-review-and-closure.md`.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan modifies the CLI executable identity, the GitHub Release asset naming contract consumed by `internal/updater`, the default browser profile directory (`defaultProfileDir`) used for Microsoft 365 session persistence, the CI/Release workflow files, and the root CLI help banner adjacent to security-sensitive argument redaction.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `.github/instructions/ci-security.instructions.md`
  * `.github/instructions/workflows.instructions.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`
  * `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **Updater <-> Release Asset Parity**: `updater.AssetName(goos, goarch)` (`go-teams-<goos>-<goarch>[.exe]`) MUST match the `-o "dist/go-teams-${GOOS}-${GOARCH}${EXT}"` build output, the `find . -maxdepth 1 -type f -name 'go-teams-*'` filter, the 6-line count check (`test "$(wc -l < SHA256SUMS)" -eq 6`), and all six expected entries in the `SHA256SUMS` verification loop of `.github/workflows/release.yml`.
2. **Root Help Argument Redaction (`redactHelpArg`)**: Updating the `Note:` banner in `installRootHelpHandler` (`internal/cli/root.go`) from `teams-cli` to `go-teams` MUST NOT alter `redactHelpArg` or `redactedHelpArg`; all schemeless and scheme-bearing secret-redaction test cases in `TestRootHelpPlaceholderAndUnknownCommandGuidance` (`internal/cli/help_test.go`) must continue to pass.
3. **Cross-Platform Batch Queue Backward Compatibility (`runQueueFile`)**: `internal/cli/queue.go` MUST normalize backslashes before calling `filepath.Base` (`firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))`), strip leading `go-teams`, `go-teams.exe`, `teams-cli`, and `teams-cli.exe` tokens on both Windows and POSIX hosts, and skip any line whose remaining argument slice is empty (`if len(lineArgs) == 0 { continue }`).
4. **Template <-> `config.example.yaml` Parity**: `config.example.yaml` MUST equal `config.TemplateYAML()` after CRLF/LF normalization (`TestExampleConfigMatchesTemplateYAML`), contain zero `"teams-cli"` substrings (`assert.NotContains(t, tpl, "teams-cli")`), and contain zero active lines or live credentials (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
5. **CI Security Invariants**: `.github/workflows/ci.yml` and `.github/workflows/release.yml` MUST retain full commit SHA action pinning, `persist-credentials: false`, least-privilege `permissions: contents: read` defaults, and `install-mode: goinstall` on `golangci-lint-action`.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-027-1` | Rename updater release asset contract (`AssetName` -> `go-teams-<goos>-<goarch>[.exe]`, `tempPattern` -> `.go-teams-update-*`, `User-Agent`, lock recovery strings) and align `.github/workflows/release.yml` + `ci.yml` | `internal/updater/updater.go`, `.github/workflows/release.yml`, `.github/workflows/ci.yml` | Release & self-update contract | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 and U5 commits |
| `PA-027-2` | Rename `appDirName` in `internal/config/config.go` (`defaultProfileDir` session persistence path and `DefaultConfigPath`) to `"go-teams"` and synchronize `internal/config/template.go` and `config.example.yaml` | `internal/config/config.go`, `internal/config/template.go`, `config.example.yaml` | Auth profile persistence path & template | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-027-3` | Move `cmd/teams-cli` to `cmd/go-teams` via `git mv cmd/teams-cli cmd/go-teams` (no unapproved destructive shell deletion commands), update Cobra `Use`/help/update strings to `go-teams`, and support cross-platform `go-teams[.exe]` and legacy `teams-cli[.exe]` leading tokens in `internal/cli/queue.go` | `cmd/go-teams/main.go`, `internal/cli/*.go` | CLI entrypoint & command contract | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 and U4 commits |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build -o .\bin\go-teams.exe .\cmd\go-teams` succeeds and `cmd/teams-cli` and `bin/teams-cli.exe` no longer exist.
  * [ ] `GO_TEAMS_BINARY` compiled-binary test `TestCompiledBinaryCapturesRecapToExplicitOutput` passes against `.\bin\go-teams.exe`.
  * [ ] `TestExampleConfigMatchesTemplateYAML` and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` pass in `internal/config`.
  * [ ] Cross-rename upgrade notes (`teams-cli update` one-time binary download / `go install` cutover and `%APPDATA%\go-teams` + `%LOCALAPPDATA%\go-teams\profile` directory migration) are documented in `docs/USER_GUIDE.md`.
  * [ ] `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...` pass with zero findings.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main` and across the first tagged GitHub Release (`v*.*.*`).
  * **Dashboard / query**: `gh run list --workflow=ci.yml`, `gh run list --workflow=release.yml`, `gh release view --json assets`, and local `.\bin\go-teams.exe update --check`.
  * **Baseline**: 100% pass rate on `CI` (`changes`, `test`, `test-windows`) and `Release` (`test`, `test-windows`, `build`, `release`) workflows; 6/6 `go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]` assets listed in `SHA256SUMS`.
  * **Alert threshold**: `>= 1` workflow job failure on `ci.yml` or `release.yml`, or any missing/mismatched `go-teams-*` entry in `SHA256SUMS`.
* **Rollback trigger & procedure**:
  * **Trigger**: Any `CI` build failure on `bin/go-teams.exe`, any `Release` workflow failure in `Generate SHA256SUMS` (`missing an entry for go-teams-*`), or any regression in `go-teams --queue` or `go-teams --help`.
  * **Procedure**: Revert the `027-S` merge commit on `main` (`git revert -m 1 <merge-sha>`) and re-tag if a release tag was affected.
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All pre-deploy audit checks pass, `CI` checks on the `027-S` PR are green, and the manual observation plan + rollback trigger are recorded in `docs/closure/2026-11-15-027-S-review-and-closure.md`.
  * `READY_WITH_CONDITIONS`: Local gates and PR CI checks pass, pending the next tagged release (`v*.*.*`) to exercise the tag-triggered `Release` workflow in production.
  * `BLOCKED`: Any pre-deploy audit check, test, or CI job fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona adversarial review panel (`Architecture Strategist`, `Security Lens Reviewer`, `Scope Boundary Auditor`, `Constitution Reviewer`, `Go Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`), remediate all surfaced P0/P1/P2/P3 findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings remediated in-place in `docs/plans/2026-11-15-go-teams-binary-and-release-rename-plan.md` and `docs/decisions/2026-11-15-go-teams-binary-and-release-rename-deliberation.md` prior to harvest).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, `strict-safety` `ProposedAction` / `ActionRisk` table, and `release-observability` monitoring/rollback/releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` | `subagent` (`multi-agent`) | 1 P1, 1 P2, 1 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 1 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2, 1 P3 | Remediated — `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 4 P2, 2 P3 | Remediated — `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 1 P1, 1 P2, 2 P3 | Remediated — `PASS` |
| `Schema-CLI-Docs Coupling Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 3 P2, 3 P3 | Remediated — `PASS` |
| `rubber-duck` (`Correctness Reviewer`) | `subagent` (`multi-agent`) | 3 P1, 3 P2 | Remediated — `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — Consensus HIGH: `Architecture Strategist`, `Security Lens Reviewer`, `Go Reviewer`, `rubber-duck`] Cross-platform backslash normalization in `internal/cli/queue.go` (`runQueueFile`)**:
   * **Finding**: On Linux/macOS (`ubuntu-latest` CI), `filepath.Base("C:\\Tools\\go-teams.exe")` does not split on `\` and returns `"C:\\Tools\\go-teams.exe"`, failing to strip Windows path-qualified leading binary tokens in queue files.
   * **Remediation Applied**: Updated R3, U3 (`027.003-T`), Protected Invariant 3, and `016-DL` to normalize backslashes before `filepath.Base`: `firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))`, followed by `if len(lineArgs) == 0 { continue }`.
2. **[P1 — `rubber-duck`, `Schema-CLI-Docs Coupling Reviewer`, `Architecture Strategist`, `Security Lens Reviewer`, `Constitution Reviewer`] Residual `teams-cli` strings in `.github/workflows/release.yml` and `.github/copilot-instructions.md`**:
   * **Finding**: `.github/workflows/release.yml` lines 160, 211, 212 contained commented-out `teams-cli-mcp` references, and `.github/copilot-instructions.md` line 26 listed `└── teams-cli/`.
   * **Remediation Applied**: Updated R5/U5 (`027.005-T`) to replace `teams-cli-mcp` with `go-teams-mcp` in `release.yml`, assert `test "$(wc -l < SHA256SUMS)" -eq 6`, and verify zero `teams-cli` matches in `.github/workflows/`; updated R6/U6 (`027.006-T`) to update `.github/copilot-instructions.md` line 26 to `└── go-teams/`.
3. **[P1 — `rubber-duck`, `Constitution Reviewer`, `Schema-CLI-Docs Coupling Reviewer`] Full-repo intermediate test verification and zero file overlap between U3 and U4**:
   * **Finding**: `Requirements Trace` and `Verify:` steps in U1–U4 specified package-scoped `go test` rather than `go test -race ./...`, and `internal/cli/update.go` was split across both U3 and U4.
   * **Remediation Applied**: Consolidated all `internal/cli/update.go` and `internal/cli/update_test.go` changes into U4 (`027.004-T`, sized `M`), keeping U3 (`027.003-T`, sized `S`) focused on `cmd/go-teams/main.go` (via `git mv cmd/teams-cli cmd/go-teams`) and `internal/cli/queue.go`, and updated all `Verify:` steps across U1–U6 to run `go test -race ./...` (plus `go build ./cmd/...` in U3).
4. **[P2 — `Constitution Reviewer`, `Go Reviewer`, `Security Lens Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, `rubber-duck`] Red-phase test coverage across U1, U2, U3, and U4**:
   * **Finding**: Red-phase test specifications needed explicit assertions for `tempPattern`, `User-Agent: "go-teams-updater"`, both `lockDiagnostic` `"go-teams update"` branches, all 6 `AssetName` targets, `cfg.OutputDir` / `DefaultConfigPath()`, `assert.NotContains(t, tpl, "teams-cli")`, all 5 `printUpdateResult` branches (including `CheckOnly && Downgrade`), and `assert.NotContains(t, out, "teams-cli")` across all subcommands in `help_test.go`.
   * **Remediation Applied**: Added all listed assertions to the Red phases of U1, U2, U3, and U4, and included all 10 `templateYAML` lines (`internal/config/template.go:35,38,42,45,47,54,127,136,151,156`) and `internal/cli/root.go:212`.
5. **[P2 — `Architecture Strategist`, `Security Lens Reviewer`, `Constitution Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, `rubber-duck`] Strict-safety `ActionRisk: high`, release-observability fields, `docs/USER_GUIDE.md` line checklist, and operator upgrade notes**:
   * **Finding**: `PA-027-1` and `PA-027-2` warranted `ActionRisk: high`; `release-observability` required `Dashboard/query`, `Baseline`, `Alert threshold`, `manual observation requirement`, and `Releasability Evidence Contract`; and U6 needed explicit coverage of `docs/USER_GUIDE.md` lines `11, 30, 186, 394, 482, 564/571, 617, 752/767, 756/771, 797/826` plus upgrade notes for cross-rename `update` and `%LOCALAPPDATA%\teams-cli\profile` migration/cleanup.
   * **Remediation Applied**: Elevated `PA-027-1` and `PA-027-2` to `ActionRisk: high`, added all required `release-observability` fields, and expanded U6 (`027.006-T`, sized `M`) and `Risks and Caveats`.
6. **[P3 — `Scope Boundary Auditor`, `Constitution Reviewer`] Deliberation Option A task count and `## Constitution Check` Principles I–XI table**:
   * **Remediation Applied**: Updated Option A in `016-DL` to `6 atomic tasks` and expanded `## Constitution Check` in the plan to map all 11 constitutional principles (I–XI).
