---
title: Rename CLI executable, entrypoint package, release assets, and self-updater contract from teams-cli to go-teams operational closure
date: 2026-11-15
feature: 027-F
shipment: 027-S
branch: feat/027-s-rename-cli-executable-entrypoint-package-release-assets-and-self-updater-contract-from-teams-cli-to-go-teams
status: closed
closure_status: READY_WITH_CONDITIONS
conditions:
  - condition: "All pre-deploy audit checks, 6-target AssetName and release.yml SHA256SUMS static verification, compiled-binary harness checks, and PR #68 CI checks pass; next tagged release (v*.*.*) observation window and disposable self-update verification are recorded and owned by repository maintainer"
    satisfied: true
    evidence: "PR #68 CI run 43695968083 (ubuntu-latest + windows-latest PASS), TestAssetName (6/6 targets PASS), release.yml 6-entry SHA256SUMS verification loop, and Releasability evidence monitoring/post-release validation plan below"
compaction_status: done
merge_commit: 8aff656d93b76f11f4e4094fd6e6ce724f784dbf
pull_request: 68
close_path: cascade
close_evidence: docs/closure/evidence/027-S-027-F-close-evidence.json
---

## Release scope

Shipment `027-S` (`027-F`, `027.001-T`..`027.006-T`) delivers deliberation
`016-DL` (source stash `66488961`):

* `027.001-T` (`internal/updater`, `internal/version`, commit `02e1acb`):
  * Updates `updater.AssetName(goos, goarch)` to return `go-teams-<goos>-<goarch>`
    (with `.exe` when `goos == "windows"`).
  * Updates `tempPattern` to `".go-teams-update-*"`, `User-Agent` to
    `"go-teams-updater"`, and both `lockDiagnostic` recovery messages to
    reference `go-teams update`.
  * Updates package GoDoc comments in `internal/updater/updater.go` and
    `internal/version/version.go`, and updates `internal/updater/updater_test.go`.
* `027.002-T` (`internal/config`, `config.example.yaml`, commit `54a2fa4`):
  * Updates `appDirName = "go-teams"` and GoDoc comments in
    `internal/config/config.go` (`%APPDATA%\go-teams`,
    `%LOCALAPPDATA%\go-teams\profile`, `~/.config/go-teams`).
  * Updates all 10 `templateYAML` occurrences in `internal/config/template.go`
    and synchronizes `config.example.yaml` (verified by
    `TestExampleConfigMatchesTemplateYAML`).
* `027.003-T` (`cmd/go-teams`, `internal/cli/queue.go`, commit `9ce37d1`):
  * Moves `cmd/teams-cli/main.go` to `cmd/go-teams/main.go` via `git mv` and
    updates the package GoDoc comment.
  * Updates `runQueueFile` in `internal/cli/queue.go` to normalize backslashes
    before `filepath.Base` and strip both `go-teams[.exe]` and legacy
    `teams-cli[.exe]` leading tokens cross-platform (skipping bare-binary lines).
* `027.004-T` (`internal/cli`, commit `f0f7453`):
  * Updates Cobra `Use`, `Short`, `Long`, `Example`, `printUpdateResult`, and
    `installRootHelpHandler` `Note:` banner across
    `internal/cli/{root,auth,join,schedule,update,generate_config}.go` and
    co-located tests while preserving `redactHelpArg` untouched.
* `027.005-T` (`.github/workflows/`, commit `0bcaebd`):
  * Updates `.github/workflows/ci.yml` (`go build -o bin/go-teams.exe ./cmd/go-teams`
    in `test` and `test-windows`), `.github/workflows/release.yml`
    (`dist/go-teams-${GOOS}-${GOARCH}${EXT}`, artifact upload names, and
    6-entry `SHA256SUMS` verification loop for `go-teams-*`), and
    `.github/workflows/cli-reference-drift.yml`.
* `027.006-T` (`README.md`, `docs/`, `.github/copilot-instructions.md`, commit `527a7ff`):
  * Updates `README.md`, `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`,
    `docs/QUALITY_SCORE.md`, and `.github/copilot-instructions.md` for
    `go-teams`, including heading anchors, legacy queue token compatibility
    notes, and operator upgrade guidance.

## Review

Multi-persona adversarial and `rubber-duck` review of `main..HEAD` (nine
reviewers: `Architecture Strategist`, `Security Reviewer`,
`Correctness Reviewer`, `Go Reviewer`, `Maintainability Reviewer`,
`Scope Boundary Auditor`, `Constitution Reviewer`,
`Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`) followed by GitHub
Copilot review of PR #68 (P-018 `copilot-review` gate `SATISFIED` on HEAD
`8dd682580ea7df510279522e10f5a523cc6478f2`):

| Area | Finding | Disposition |
|---|---|---|
| Pre-PR 9-persona adversarial + `rubber-duck` panel | Reviewed `main..a94f0ff` across all six units (`027.001-T`..`027.006-T`) | 0 findings (`P0=0, P1=0, P2=0, P3=0`) |
| Copilot (`PRRT_kwDOTStvas63n2mF`, `cmd/go-teams/main.go`) | `.autoharness/workspace-profile.yaml` runtime validator probes (`cli-help`, `scheduler-dry-run`) and `structure.source_layout` still referenced `./cmd/teams-cli` | Fixed in-scope (P-021 C1): updated `.autoharness/workspace-profile.yaml` to `./cmd/go-teams` and `go-teams`; commit `8dd6825`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63n2mL`, `docs/USER_GUIDE.md`) | Directory migration note should remind operators to update explicit `config.yaml` / CLI flag paths and pending `transcription_queue.txt` entries and verify before deleting legacy folders | Fixed in-scope (P-021 C1): expanded migration note in `docs/USER_GUIDE.md`; commit `8dd6825`; replied and resolved |

Blocking findings: `P0=0, P1=0`. Outcome: `READY`.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #68, merge commit `8aff656d93b76f11f4e4094fd6e6ce724f784dbf`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-15T08:50:36Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 027-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/027-s-rename-cli-executable-entrypoint-package-release-assets-and-self-updater-contract-from-teams-cli-to-go-teams` |
| Close path | `cascade` (`autoharness shipment cascade-close --shipment 027-S --feature 027-F --sha 8aff656d93b76f11f4e4094fd6e6ce724f784dbf` returned `exit_code: 0`, `classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`) |
| Close evidence | `docs/closure/evidence/027-S-027-F-close-evidence.json` |
| `archived_ids` | `027-F`, `027-S`, `027.001-T`, `027.002-T`, `027.003-T`, `027.004-T`, `027.005-T`, `027.006-T` (equal to `required_ids`; `unexpected_archived: []`) |
| `returned_ids` | empty |
| `parent_id` preservation | `027.001-T`..`027.006-T` retained `parent_id: 027-F` (`parent_id_preserved: true`) |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stash `66488961` | Already archived by Stage at harvest; absent from active stash | skipped |
| Deliberation `016-DL` | `already-archived` in `pre_close.linked_deliberation_disposition` (`docs/closure/evidence/027-S-027-F-close-evidence.json`): `backlogit archive 016-DL` (hook event 501) executed immediately before the mutating `cascade-close` snapshot after the initial `cascade-close` invocation exited with usage help (ordering deviation from post-cascade Step 6.7); post-cascade cleanup was therefore skipped while preserving the original hook and close-evidence records | skipped (`already-archived`) |

### Compaction (P-020)

`compact-context` ran at closure, producing
`docs/memory/compacted/2026-11-15-027-S-go-teams-binary-and-release-rename-compacted.md`
and archiving the verbose session memory files to
`docs/archive/memory/2026-11-15/stage-027-S-memory.md` and
`docs/archive/memory/2026-11-15/027-S-ship-memory.md`. `compound-refresh`
reviewed all 10 entries in `docs/compound/` and classified all 10 as `keep`
(zero stale `teams-cli` references). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis & runtime probes | `go build -o bin/go-teams.exe ./cmd/go-teams`, `go build ./...`, `go run ./cmd/go-teams --help`, `go run ./cmd/go-teams schedule --help`, `go vet ./...`, `go vet -tags=integration ./...`, `gofmt -l .` (clean) | PASS |
| Full test suite & compiled binary harness | `go test -race ./...` and `TestCompiledBinaryCapturesRecapToExplicitOutput` (`GO_TEAMS_BINARY=.\bin\go-teams.exe`) on Windows | PASS |
| CI `ubuntu-latest` | `Build, vet, lint & test`, GitHub Actions run `43695968083` (PR #68, HEAD `8dd6825`) | PASS |
| CI `windows-latest` | `Build, vet & test (Windows)`, same run | PASS |
| Backlog & closure gate | `backlogit sync` (174 artifacts indexed), `backlogit doctor` ("No issues found."), and `autoharness gate closure-evidence` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| `PA-027-1`: Rename updater release asset contract (`AssetName` -> `go-teams-<goos>-<goarch>[.exe]`, `tempPattern` -> `.go-teams-update-*`, `User-Agent: "go-teams-updater"`, lock recovery messages) and align `.github/workflows/release.yml` + `ci.yml` | high | Pre-authorized under `DARK_MODE_ACTIVE`; verified by unit tests and 6-target `SHA256SUMS` check | applied and verified |
| `PA-027-2`: Rename `appDirName` in `internal/config/config.go` (`defaultProfileDir` and `DefaultConfigPath`) to `"go-teams"` and synchronize `internal/config/template.go` and `config.example.yaml` | high | Pre-authorized under `DARK_MODE_ACTIVE`; verified by `TestDefaultConfig`, `TestDefaultConfigPath`, and `TestExampleConfigMatchesTemplateYAML` | applied and verified |
| `PA-027-3`: Move `cmd/teams-cli` to `cmd/go-teams` via `git mv`, update Cobra `Use`/help/update strings to `go-teams`, and support cross-platform `go-teams[.exe]` and legacy `teams-cli[.exe]` leading tokens in `internal/cli/queue.go` | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; verified by unit tests and `TestCompiledBinaryCapturesRecapToExplicitOutput` | applied and verified |

## Releasability evidence

* **Invariants preserved**: Updater <-> `release.yml` 6-asset parity (`go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]`), root help argument redaction (`redactHelpArg`), cross-platform batch queue backward compatibility (`go-teams[.exe]` + legacy `teams-cli[.exe]`), `TemplateYAML()` <-> `config.example.yaml` byte-for-byte parity, and CI security invariants.
* **Rollout and rollback**: Single-binary CLI on `main`. Rollback covers three coordinated surfaces:
  1. *Repository branch (`main`)*: Execute `git revert -m 1 8aff656d93b76f11f4e4094fd6e6ce724f784dbf` (retaining the `main`-branch parent to revert the feature merge cleanly).
  2. *Published GitHub Release / tag (`v*.*.*`, if already cut)*: Retract or mark the affected GitHub Release as a broken pre-release (`gh release edit <tag> --prerelease --notes "Retracted: use previous release or next patch tag"` or `gh release delete <tag> --cleanup-tag --yes`) and cut a new patch tag `v*.*.*` from the reverted `main` commit so `release.yml` republishes working release assets and `SHA256SUMS`.
  3. *Operator binary, configuration, and profile recovery*: Reinstall the previous release binary (via direct download from the prior GitHub Release or `go install`). If an operator already renamed `%APPDATA%\teams-cli` -> `%APPDATA%\go-teams` and `%LOCALAPPDATA%\teams-cli\profile` -> `%LOCALAPPDATA%\go-teams\profile` (`~/.config/teams-cli` -> `~/.config/go-teams`), either pass `--config "%APPDATA%\go-teams\config.yaml"` (or `~/.config/go-teams/config.yaml`) and `--profile-dir "%LOCALAPPDATA%\go-teams\profile"` (or `~/.config/go-teams/profile`) or move the directories back to `teams-cli` only after updating any explicit `profile_dir`, `output_dir`, and `queue_file` paths in `config.yaml`, scheduled task arguments, and pending `--append-ai-summary` lines in `transcription_queue.txt` so cached session state and queued work are preserved.
* **Monitoring and validation window (7 days / next tagged release `v*.*.*`, owner: repository maintainer)**:
  * **Healthy signals**:
    1. *CI & Release builds (`ci.yml`, `release.yml`)*: `go build -o bin/go-teams.exe ./cmd/go-teams` passes on `ubuntu-latest` (`test`) and `windows-latest` (`test-windows`), and the next tagged release (`v*.*.*`) publishes all 6 `go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]` binaries plus a 6-entry `SHA256SUMS` file.
    2. *Self-update & disposable installation probe*: `go-teams --version` and `go-teams --help` report `go-teams` usage; `go-teams update --check --version <tagged-release>` from an older build (or `dev` build) resolves `go-teams-<goos>-<goarch>[.exe]`; and a disposable-directory installation probe (copying `go-teams[.exe]` into a temporary directory and running `go-teams update --version <tagged-release> --force`, plus `gh release download <tagged-release> --pattern "SHA256SUMS" --pattern "go-teams-*"` with `sha256sum -c SHA256SUMS`) verifies end-to-end download, `SHA256SUMS` digest validation, and binary replacement.
    3. *Batch queue compatibility*: `go-teams --queue` strips both `go-teams[.exe]` and legacy `teams-cli[.exe]` leading tokens on Windows and POSIX hosts.
  * **Failure signals and rollback triggers**:
    1. Any `CI` build failure on `bin/go-teams.exe` or `Release` workflow failure in `Generate SHA256SUMS` (`missing an entry for go-teams-*`).
    2. Any failure in the post-release `gh release download` + `sha256sum -c SHA256SUMS` check or disposable `go-teams update --version <tagged-release> --force` installation probe (`ErrAssetNotFound`, `ErrChecksumMissing`, or `ErrChecksumMismatch`).
    3. Any regression in `go-teams --queue`, `go-teams --help`, or `go-teams generate-config`.
    * *Action on trigger*: Halt release distribution, execute `git revert -m 1 8aff656d93b76f11f4e4094fd6e6ce724f784dbf` on `main`, retract any affected GitHub Release tag (`v*.*.*`) and cut a corrective patch release, and apply the operator binary/configuration/profile recovery steps above while preserving migrated profile state and pending `transcription_queue.txt` entries.
* **Pending post-release validation (maintainer-owned)**:
  * On the next tagged release (`v*.*.*`), verify that `.github/workflows/release.yml` publishes all six `go-teams-{linux,darwin,windows}-{amd64,arm64}[.exe]` assets and `SHA256SUMS`, verify all six digests against `SHA256SUMS`, and run the disposable `go-teams update --version <tag> --force` installation probe.
* **Readiness verdict**: `READY_WITH_CONDITIONS` (all local gates, compiled-binary harness checks, and PR #68 CI checks on `ubuntu-latest` and `windows-latest` passed; tag-triggered `.github/workflows/release.yml` asset publication and disposable self-update verification are pending the next `v*.*.*` release tag).
