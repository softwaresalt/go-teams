---
title: Cross-platform CI, POSIX permission verification, and multi-OS release asset parity implementation plan
description: Implementation plan for 023-F / 023-S adding POSIX 0o700/0o600 permission verification tests in internal/capture, a windows-latest CI runner job in ci.yml, multi-OS release assets in release.yml, and updated documentation
source: docs/decisions/2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-deliberation.md
deliberation_id: 012-DL
feature: 023-F
shipment: 023-S
source_stash:
  - 70ABBB2C
  - D3A9CBD0
  - EC6D1CAF
date: 2026-11-14
status: approved
tags:
  - capture
  - ci
  - release
  - updater
  - posix
  - windows
  - tests
  - docs
---

## Problem Frame

Following the accepted deliberation in
[2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-deliberation.md](../decisions/2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-deliberation.md)
(`012-DL`), stash entries **`70ABBB2C`** (`task`, `low`), **`D3A9CBD0`** (`task`,
`low`), and **`EC6D1CAF`** (`task`, `low`) are planned under covering feature
**`023-F`** and shipment **`023-S`**:

1. **POSIX mode verification for `internal/capture` (`70ABBB2C`)**:
   `internal/capture/output.go` defines `dirPerm = 0o700` and `filePerm = 0o600`
   so newly created output directories and transcript/MP3 files are owner-only,
   while pre-existing directories keep their existing permissions. However, no
   unit test currently asserts the resulting POSIX mode bits on disk, leaving
   `internal/capture` at `Grade B` in `docs/QUALITY_SCORE.md`. Adding a
   `//go:build unix` unit test file `internal/capture/output_unix_test.go` that
   runs under `syscall.Umask(0)` (restoring the previous umask via `defer`)
   closes this verification gap and upgrades `internal/capture` to `Grade A`.
2. **Windows CI runner job in `.github/workflows/ci.yml` (`D3A9CBD0`)**:
   `.github/workflows/ci.yml` currently runs `test` (`Build, vet, lint & test`)
   only on `ubuntu-latest`. As a result, Windows-only tests — including
   `TestChromeMeetingEndToEnd` in `internal/browser/meeting_e2e_test.go` and
   Windows `.old` executable rename window tests in `internal/updater` — only
   execute on developer workstations. Adding a `test-windows` job
   (`runs-on: windows-latest`, `needs: changes`,
   `if: needs.changes.outputs.code == 'true'`) runs `go build`, `go vet ./...`,
   `go vet -tags=integration ./...`, and `go test -race ./...` natively on
   Windows in CI (and a matching `test-windows` gate in `release.yml` gates
   tagged releases on native Windows verification).
3. **Multi-OS release assets in `.github/workflows/release.yml` & documentation (`EC6D1CAF`)**:
   `internal/updater/updater.go` `AssetName(goos, goarch)` resolves
   `teams-cli-<goos>-<goarch>` (with `.exe` only when `goos == "windows"`), and
   `go-teams` is 100% pure Go with zero CGO dependencies. Expanding the `build`
   job matrix in `.github/workflows/release.yml` from `goos: [windows]` to
   `goos: [linux, darwin, windows]` × `goarch: [amd64, arm64]` with
   `CGO_ENABLED: "0"` and conditional `.exe` suffixing publishes all six
   platform binaries and their `SHA256SUMS` digests so `teams-cli update`
   succeeds on Linux, macOS, and Windows (`amd64` and `arm64`).

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Add `//go:build unix` test suite in `internal/capture/output_unix_test.go` asserting `0o700` directory and `0o600` file permissions created by `setupOutput` (`Dir` and `FilePath` modes), pre-existing directory permission preservation (`0o750`), and `AppendAISummaryToFile` `0o600` permission preservation under `syscall.Umask(0)` | `70ABBB2C` | U1 (`023.001-T`) | `go vet ./internal/capture/...` and `go test -race ./internal/capture/...` (and `ubuntu-latest` CI run) |
| R2: Add `test-windows` job (`runs-on: windows-latest`, `needs: changes`, `if: needs.changes.outputs.code == 'true'`) to `.github/workflows/ci.yml` with SHA-pinned `actions/checkout` (`persist-credentials: false`) and `actions/setup-go` (`go-version-file: go.mod`) running `go build -o bin/teams-cli.exe ./cmd/teams-cli`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -race ./...` | `D3A9CBD0` | U2 (`023.002-T`) | GitHub Actions `ci.yml` `Build, vet & test (Windows)` job green |
| R3: Expand `.github/workflows/release.yml` with a `test-windows` quality gate (`needs: [test, test-windows]` on `build`), expand the `build` job (`name: Build release binaries`) strategy matrix to `goos: [linux, darwin, windows]` × `goarch: [amd64, arm64]` (`fail-fast: false`) with `CGO_ENABLED: "0"` and conditional `.exe` suffix only when `GOOS == "windows"` matching `updater.AssetName(goos, goarch)`, and assert all 6 expected entries exist in `SHA256SUMS` before release publication | `EC6D1CAF`, `D3A9CBD0` | U2 (`023.002-T`) | Workflow YAML validation, `SHA256SUMS` 6-target manifest check, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| R4: Update `README.md`, `docs/QUALITY_SCORE.md`, `docs/ARCHITECTURE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` to reflect `internal/capture` Grade `A`, the `windows-latest` CI job, and multi-OS `teams-cli update` release assets | `70ABBB2C`, `D3A9CBD0`, `EC6D1CAF` | U3 (`023.003-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |

## Implementation Units

### U1 (`023.001-T`) — Add POSIX-only `0o700`/`0o600` permission verification tests in `internal/capture/output_unix_test.go`

* **Domain:** tests (Go `internal/capture`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Create `internal/capture/output_unix_test.go` with `//go:build unix` build
     constraint and `package capture`.
  2. Implement a test helper `withZeroUmask(t *testing.T, fn func())` that
     calls `oldMask := syscall.Umask(0)` and `defer syscall.Umask(oldMask)`
     before invoking `fn()`. Do NOT call `t.Parallel()` in this file because
     `syscall.Umask` mutates process-wide state.
  3. Add unit tests covering:
     * `TestSetupOutputUnixPermissionsDirMode`: Under `withZeroUmask`, call
       `setupOutput` with a non-existent nested `Dir` (`filepath.Join(t.TempDir(), "nested", "captures")`)
       and `CaptionsOnly: false`. Close `out`, then `os.Stat` the created `Dir`,
       `out.TranscriptPath`, and `out.AudioPath`, and assert:
       - `dirInfo.Mode().Perm() == 0o700` (`dirPerm`)
       - `mdInfo.Mode().Perm() == 0o600` (`filePerm`)
       - `mp3Info.Mode().Perm() == 0o600` (`filePerm`)
     * `TestSetupOutputUnixPermissionsExplicitFilePath`: Under `withZeroUmask`,
       call `setupOutput` with a non-existent parent directory in `FilePath`
       (`filepath.Join(t.TempDir(), "explicit-parent", "call.md")`) and
       `CaptionsOnly: false`. Close `out`, then `os.Stat` the created parent
       directory, `out.TranscriptPath`, and `out.AudioPath`, asserting `0o700`
       on the parent directory and `0o600` on both `.md` and `.mp3` files.
     * `TestSetupOutputUnixPreservesPreExistingDirPermissions`: Under
       `withZeroUmask`, pre-create a directory with `os.Mkdir(preExistingDir, 0o750)`,
       run `setupOutput` into `Dir: preExistingDir` (`CaptionsOnly: true`),
       close `out`, and assert `dirInfo.Mode().Perm() == 0o750` (not clobbered
       to `0o700`) while `mdInfo.Mode().Perm() == 0o600`.
     * `TestAppendAISummaryToFileUnixPreservesFilePermissions`: Under
       `withZeroUmask`, create a transcript via `setupOutput` (`CaptionsOnly: true`),
       close `out`, call `AppendAISummaryToFile(out.TranscriptPath, "Key decisions made.")`,
       and assert `mdInfo.Mode().Perm() == 0o600` after the append.
* **Files (1 file):** `internal/capture/output_unix_test.go`.
* **Verify:** Run PowerShell cross-target type-checking locally on Windows
  (`$env:GOOS='linux'; go vet ./internal/capture/...; go test -c ./internal/capture -o NUL; Remove-Item Env:GOOS`)
  so the `//go:build unix` test file is compiled and type-checked before push,
  plus `go vet ./...` and `go test -race ./internal/capture/...` locally, and
  verify all four tests execute and pass on `ubuntu-latest` in GitHub Actions CI.
* **Depends on:** none.

### U2 (`023.002-T`) — Add `windows-latest` CI test job to `.github/workflows/ci.yml` and expand `.github/workflows/release.yml` with `test-windows` gate and `linux`, `darwin`, `windows` (`amd64`, `arm64`) release matrix

* **Domain:** config (GitHub Actions workflows). **Posture:** migration-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `.github/workflows/ci.yml`:
     * Retain `changes` and `test` (`ubuntu-latest`) unchanged.
     * Add job `test-windows` immediately after `test`:
       - `name: Build, vet & test (Windows)`
       - `needs: changes`
       - `if: needs.changes.outputs.code == 'true'`
       - `runs-on: windows-latest`
       - Steps:
         1. `uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`
            with `persist-credentials: false`
         2. `uses: actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0`
            with `go-version-file: go.mod`
         3. `name: Install dependencies` -> `run: go mod download`
         4. `name: Build Windows executable` -> `run: go build -o bin/teams-cli.exe ./cmd/teams-cli`
         5. `name: Vet` -> `run: go vet ./...`
         6. `name: Vet (integration harness)` -> `run: go vet -tags=integration ./...`
         7. `name: Verify Windows browser E2E does not skip` -> run
            `go test -v ./internal/browser -run "^TestChromeMeetingEndToEnd$"` and fail if the output contains `--- SKIP:` (confirming `ResolveExecPath("")` locates a pre-installed Chromium/Edge browser on `windows-latest` and `TestChromeMeetingEndToEnd` executes rather than skipping)
         8. `name: Test` -> `run: go test -race ./...`
  2. In `.github/workflows/release.yml`:
     * Add job `test-windows` (`name: Quality Gates (Windows)`,
       `runs-on: windows-latest`) after `test` (`Quality Gates`), running
       SHA-pinned `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`
       with `persist-credentials: false`, SHA-pinned
       `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0`
       with `go-version-file: go.mod`, `go mod download`, `go vet ./...`,
       `go vet -tags=integration ./...`, the non-skipped
       `TestChromeMeetingEndToEnd` preflight check, and `go test -race ./...` so
       tagged releases are also gated on native Windows verification without
       duplicating `golangci-lint` compilation.
     * In job `build`:
       - Change `name: Build Windows binaries` to `name: Build release binaries`
         and update `needs: test` to `needs: [test, test-windows]`.
       - Set `strategy.fail-fast: false` and update `strategy.matrix` to:
         ```yaml
         strategy:
           fail-fast: false
           matrix:
             goos: [linux, darwin, windows]
             goarch: [amd64, arm64]
         ```
       - In the build step, change `name: Build Windows executable` to
         `name: Build release executable`, add `CGO_ENABLED: "0"` to `env:`
         alongside `GOOS: ${{ matrix.goos }}` and `GOARCH: ${{ matrix.goarch }}`,
         and set `EXT=""` (`if [ "${GOOS}" = "windows" ]; then EXT=".exe"; fi`)
         so `go build` writes to `-o "dist/teams-cli-${GOOS}-${GOARCH}${EXT}" ./cmd/teams-cli`.
       - Keep `Upload artifacts` (`name: teams-cli-${{ matrix.goos }}-${{ matrix.goarch }}`,
         `path: dist/`, `if-no-files-found: error`) intact.
     * In job `release`, extend the `Generate SHA256SUMS` step to verify that
       all six expected `updater.AssetName(goos, goarch)` entries
       (`teams-cli-linux-amd64`, `teams-cli-linux-arm64`,
       `teams-cli-darwin-amd64`, `teams-cli-darwin-arm64`,
       `teams-cli-windows-amd64.exe`, `teams-cli-windows-arm64.exe`) are present
       in `SHA256SUMS` (matching the optional `./` and `*` prefixes stripped by
       `updater.parseChecksums` in `internal/updater/updater.go:1267-1283`, e.g.
       `grep -Eq "^[0-9a-f]{64}[[:space:]]+\*?(\./)?${expected}$" SHA256SUMS`)
       before `Publish release` runs.
* **Files (2 files):** `.github/workflows/ci.yml`,
  `.github/workflows/release.yml`.
* **Verify:** Verify all third-party actions remain pinned to full 40-char commit
  SHAs with `persist-credentials: false` on checkout, and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Depends on:** none.

### U3 (`023.003-T`) — Update `README.md`, `docs/QUALITY_SCORE.md`, `docs/ARCHITECTURE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`

* **Domain:** docs (Markdown). **Posture:** migration-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `docs/QUALITY_SCORE.md`:
     * In `## Grades by package`, upgrade `internal/capture` from Grade `B` to
       Grade `A` and update its Rationale to state that
       `internal/capture/output_unix_test.go` asserts the `0o700` (`dirPerm`)
       and `0o600` (`filePerm`) creation modes under `syscall.Umask(0)` as well
       as pre-existing directory mode preservation and `AppendAISummaryToFile`
       mode preservation.
     * In `## Grades by package` (`internal/updater`), note that the Windows
       rename window is exercised on `windows-latest` in CI as well as Windows
       development hosts.
     * In `### Tier 1: unit and concurrency suite` and
       `### Tier 2: synthetic browser and compiled-binary end to end`, note that
       CI runs `go test -race ./...` on both `ubuntu-latest` and
       `windows-latest` (`test-windows`), keep the Windows + Chromium-based
       browser prerequisite (`ResolveExecPath("")`) explicit for
       `TestChromeMeetingEndToEnd`, and state that `test-windows` enforces a
       non-skipped pass of `TestChromeMeetingEndToEnd` alongside the Windows
       updater tests in CI, while noting that
       `TestCompiledBinaryCapturesRecapToExplicitOutput` remains gated on
       `GO_TEAMS_BINARY`.
     * In `## Live-verification gaps`, remove the two closed rows (`CI runs on ubuntu-latest only...`
       and `No test asserts the 0o700 directory and 0o600 file modes...`) only
       after verifying the non-skipped `TestChromeMeetingEndToEnd` preflight step
       is wired in `test-windows`, leaving only the three real-tenant `010-S`
       rows.
     * In `## Improvement roadmap`, mark items 3 and 4 as completed in `023-S`
       with strikethrough (`~~...~~ Completed in 023-S (...)`), matching item 2's
       format.
  2. In `README.md`:
     * In `### Self-update`, update the first bullet to state that
       `internal/updater` resolves release assets named
       `teams-cli-<goos>-<goarch>` (with `.exe` on Windows) and
       `.github/workflows/release.yml` builds and publishes CGO-free
       (`CGO_ENABLED=0`) release binaries for `linux`, `darwin`, and `windows`
       (`amd64` and `arm64`, cross-compiled on `ubuntu-latest` after Linux and
       Windows quality gates pass) alongside `SHA256SUMS`.
  3. In `docs/ARCHITECTURE.md`:
     * In `## Security and isolation invariants` (`Output permissions`), note
       that the `0o700` directory and `0o600` file modes are enforced by
       `internal/capture` and verified on POSIX platforms under
       `syscall.Umask(0)` (`internal/capture/output_unix_test.go`).
  4. In `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`:
     * In `## Release discovery and transport trust`, state that
       `.github/workflows/release.yml` publishes `teams-cli-<goos>-<goarch>[.exe]`
       for `linux`, `darwin`, and `windows` (`amd64` and `arm64`, with `darwin`
       and `linux/arm64` cross-compiled on `ubuntu-latest`).
     * In `## Consequences and known limits`, update the final bullet to note
       that the Windows rename window is verified on `windows-latest` in CI as
       well as Windows development hosts (`023-S`).
* **Files (4 files, single docs domain — all Markdown documentation):**
  `README.md`, `docs/QUALITY_SCORE.md`, `docs/ARCHITECTURE.md`,
  `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`.
* **Verify:** Stage changes with `git add` and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Depends on:** U1 (`023.001-T`), U2 (`023.002-T`).

## Dependency Graph

```text
U1 (023.001-T: internal/capture/output_unix_test.go POSIX 0o700/0o600 tests) ──┐
                                                                               ├──> U3 (023.003-T: README, QUALITY_SCORE, ARCHITECTURE, self-update design doc)
U2 (023.002-T: ci.yml test-windows job & release.yml 6-target matrix) ─────────┘
```

## Decisions and Rationale

* **`//go:build unix` with `syscall.Umask(0)` restoration in `output_unix_test.go`**:
  On Unix, the kernel applies `mode & ^umask` to `mkdir` and `open(..., O_CREAT, mode)`
  syscalls. Temporarily setting `syscall.Umask(0)` and restoring the prior mask
  via `defer` proves that `internal/capture` passes `0o700` and `0o600` directly
  rather than relying on a restrictive process umask.
* **Separate `test-windows` job in `.github/workflows/ci.yml` and `.github/workflows/release.yml` instead of matrixing `golangci-lint`**:
  `golangci-lint` in `test` (`ubuntu-latest`) compiles from source via
  `install-mode: goinstall` (`docs/compound/build-errors/golangci-lint-go-version-mismatch.md`).
  Running `go build`, `go vet ./...`, `go vet -tags=integration ./...`, and
  `go test -race ./...` in `test-windows` exercises all Windows-specific runtime
  and browser tests without paying the source-compiled linter cost twice.
* **`CGO_ENABLED: "0"` and conditional `.exe` suffix in `release.yml`**:
  Produces CGO-free release binaries across `linux`, `darwin`, and `windows`
  (`amd64` and `arm64`) whose filenames (`teams-cli-<goos>-<goarch>[.exe]`)
  match `updater.AssetName(goos, goarch)` in
  `internal/updater/updater.go:572-578`.

## Risks and Caveats

* **Process-wide `syscall.Umask` in `internal/capture` tests**:
  If a test in `output_unix_test.go` ran in parallel with another test creating
  files, `syscall.Umask(0)` could affect sibling tests. Omitting `t.Parallel()`
  in `output_unix_test.go` (and matching the existing sequential test posture in
  `internal/capture/output_test.go` and `session_test.go`) eliminates any race on
  the process umask.
* **Windows CI runner execution time**:
  `TestChromeMeetingEndToEnd` launches headless Chrome/Edge against a local
  `httptest.Server` (~3–5 seconds). Full `go test -race ./...` on
  `windows-latest` completes well within GitHub Actions job timeouts.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only (`os`, `path/filepath`, `syscall`, `testing`); zero new dependencies; clean `go vet` and `golangci-lint` |
| II: Test first | Adds deterministic POSIX permission unit tests (`023.001-T`) and enables automated Windows CI test execution (`023.002-T`) |
| III, IV: Secrets & containment | All tests use `t.TempDir()`; CI workflows pin actions to full commit SHAs with `persist-credentials: false`; `permissions: contents: read` preserved |
| V: Structured observability | CI reports separate `Build, vet, lint & test` (`ubuntu-latest`) and `Build, vet & test (Windows)` (`windows-latest`) status checks |
| VI: Single responsibility | Three width-isolated tasks (`023.001-T` in `tests`, `023.002-T` in `config`/workflows, `023.003-T` in `docs`) |
| VII, VIII: Strict safety | CI/CD workflow and release matrix modifications classified in `## Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `012-DL`, plan, feature `023-F`, tasks `023.001-T`–`023.003-T`, and shipment `023-S` recorded with full stash traceability (`70ABBB2C`, `D3A9CBD0`, `EC6D1CAF`) |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: expands published GitHub
  Release binary contract from `windows/{amd64,arm64}` to
  `{linux,darwin,windows}/{amd64,arm64}` matching `updater.AssetName(goos, goarch)`.
* Security, auth, permission, or compliance-sensitive behavior — **present
  (`ActionRisk: medium`)**: modifies `.github/workflows/ci.yml` and
  `.github/workflows/release.yml` (governed by
  `.github/instructions/ci-security.instructions.md`) and tests `0o700`/`0o600`
  filesystem permission invariants using `syscall.Umask(0)`.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: additive CI job, additive release matrix targets, and hermetic unit
  tests in `t.TempDir()`.
* External integration, operator checkpoint, or external dependency — **absent**:
  runs on standard GitHub-hosted `ubuntu-latest` and `windows-latest` runners
  with zero new dependencies.
* High runtime, rollout, or rollback risk — **low**: no production Go source
  files are modified; changes are confined to a `//go:build unix` test file,
  two workflow `.yml` files, and four documentation files.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 mutates process-wide `syscall.Umask` during
POSIX permission verification tests and U2 modifies `.github/workflows/ci.yml`
and `.github/workflows/release.yml` (governed by `ci-security.instructions.md`
and `workflows.instructions.md`).

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/ci-security.instructions.md`
  * `.github/instructions/workflows.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `.github/instructions/writing-style.instructions.md`
  * `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md`
  * `docs/compound/workflow-issues/amend-breaks-shipment-gate.md`

### Protected Invariants

1. **CI Action SHA Pinning and Least-Privilege Token Permissions (`ci-security.instructions.md`)**:
   Every action in `test-windows` (`.github/workflows/ci.yml`) and `build`
   (`.github/workflows/release.yml`) MUST be pinned to a full 40-character
   commit SHA (`actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`,
   `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0`,
   `actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02 # v4.6.2`).
   Every `actions/checkout` step MUST set `persist-credentials: false`, and
   top-level workflow `permissions: contents: read` MUST remain unchanged.
2. **Exact Release Asset Naming Parity with `updater.AssetName(goos, goarch)`**:
   In `.github/workflows/release.yml`, the built binary filename MUST be
   `teams-cli-${GOOS}-${GOARCH}.exe` when `GOOS == "windows"` and
   `teams-cli-${GOOS}-${GOARCH}` (no extension) when `GOOS` is `linux` or
   `darwin`. The `release` job's `Generate SHA256SUMS` step
   (`find . -maxdepth 1 -type f -name 'teams-cli-*' -print0 | sort -z | xargs -0 -r sha256sum > SHA256SUMS`)
   MUST include all six platform binaries so `updater.Update` can verify every
   platform binary against `SHA256SUMS`.
3. **Deterministic `syscall.Umask(0)` Restoration in `output_unix_test.go`**:
   `withZeroUmask` MUST restore the original process umask via
   `defer syscall.Umask(oldMask)` even if an assertion fails (`t.Fatalf`), and
   tests in `internal/capture/output_unix_test.go` MUST NOT use `t.Parallel()`.
4. **No Production Go Code Mutation**:
   `internal/capture/output.go` and `internal/updater/updater.go` already
   implement the required `0o700`/`0o600` modes and `AssetName(goos, goarch)`
   contract; this shipment adds verification, CI/release workflow parity, and
   documentation only.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Temporarily set `syscall.Umask(0)` in `internal/capture/output_unix_test.go` (`//go:build unix`) to verify `0o700` directory and `0o600` file permissions | low | Scoped to sequential unit tests in `t.TempDir()`; previous umask is restored unconditionally via `defer syscall.Umask(oldMask)` | `output_unix_test.go` proves `0o700` and `0o600` creation modes and pre-existing directory mode preservation on Unix CI runners |
| Add `test-windows` job (`runs-on: windows-latest`) to `.github/workflows/ci.yml` and expand `.github/workflows/release.yml` build matrix to `{linux,darwin,windows} x {amd64,arm64}` | medium | All actions SHA-pinned; `persist-credentials: false` on checkout; `permissions: contents: read` preserved; `CGO_ENABLED: "0"`; `fail-fast: false` | Windows-only tests execute in CI on every code PR/push, and tagged releases publish verified binaries for Linux, macOS, and Windows |

### Review-Gate Capability Risks

* In Step 4 (`plan-review`), Stage will invoke reviewer persona subagents (and
  `Adversarial Review` / `rubber-duck` critique) and append literal
  `dispatch_mode:` and `decision:` markers to `## Plan Review` before harvest.

## Runtime Verification and Closure

* **Runtime surface impact**: No CLI flags or runtime Go packages are modified.
  The CI pipeline surface (`.github/workflows/ci.yml`) and release pipeline
  surface (`.github/workflows/release.yml`) are updated.
* **Verification**:
  * `go vet ./...` and `go vet -tags=integration ./...`
  * `golangci-lint run`
  * `gofmt -l .`
  * `go test -race ./...` (local Windows + GitHub Actions `ubuntu-latest` and
    `windows-latest` jobs on the feature PR)
  * `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
* **Operational closure**:
  * **Healthy signal**: Both `Build, vet, lint & test` (`ubuntu-latest`, running
    `output_unix_test.go`) and `Build, vet & test (Windows)` (`windows-latest`,
    verifying a non-skipped `TestChromeMeetingEndToEnd` pass with a detected
    Chromium/Edge browser via `ResolveExecPath("")` plus Windows `updater`
    tests) pass on the PR in GitHub Actions before the Windows CI gap is
    recorded as closed.
  * **Failure signal**: Non-zero exit in `test` or `test-windows` CI jobs, or
    missing platform binary in `release.yml` `build` matrix.
  * **Rollback trigger**: Flaky or failing `test-windows` runner job in CI or
    broken release artifact upload on tag push.
  * **Rollback procedure**: Revert the `023-S` merge commit on `main`.
  * **Owner**: Repository maintainer / operator.
  * **Validation window**: Immediate on PR #56 CI run (`ubuntu-latest` +
    `windows-latest`) and next tagged release (`v*.*.*`).

## Plan Review

dispatch_mode: multi-agent
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_OK: reviewer-subagent-dispatch` — Multi-agent persona review executed across
  7 specialized reviewer subagents, including cross-model anchor review
  (`Architecture Strategist` on `gpt-6.1-sol`) and rubber-duck adversarial review
  (`Correctness Reviewer` on `gpt-6.1-sol`).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants and `strict-safety`
  `ProposedAction` / `ActionRisk` table.

### Persona Coverage

| Persona | Mode / Model | Initial Findings | Post-Remediation Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 2 P3` — Suggested local cross-OS `GOOS=linux` type-checking of `output_unix_test.go` on Windows and noted U3 touches 4 docs files within a single `docs` domain (~20m effort). | PASS |
| Go Reviewer (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Confirmed `//go:build unix`, `syscall.Umask(0)` restoration via `defer`, sequential execution without `t.Parallel()`, and `updater.AssetName` parity. | PASS |
| Scope Boundary Auditor (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Confirmed strict 1:1 traceability to `70ABBB2C`, `D3A9CBD0`, and `EC6D1CAF`, and complete exclusion of `010-S` and `3EB89E2D`. | PASS |
| Learnings Researcher (always-on) | Subagent (`claude-haiku-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Surfaced `golangci-lint-go-version-mismatch.md`, `shine-mp3-race-checkptr.md`, and `amend-breaks-shipment-gate.md`, all incorporated into plan design. | PASS |
| Architecture Strategist (anchor cross-model) | Subagent (`gpt-6.1-sol`, `high`) | `0 P0, 0 P1, 2 P2, 1 P3` — Recommended (1) asserting all 6 `updater.AssetName` entries in `release.yml` `Generate SHA256SUMS`, (2) gating `release.yml` `build` on a `test-windows` job (`needs: [test, test-windows]`), and (3) distinguishing cross-compiled `darwin` release binaries from native `ubuntu-latest`/`windows-latest` CI runs. | PASS (all 3 incorporated into U2 and U3) |
| Security Lens Reviewer (CI/CD & permissions) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Verified full 40-char commit SHA pinning, `persist-credentials: false`, `permissions: contents: read`, and `SHA256SUMS` integrity. | PASS |
| Correctness Reviewer (rubber-duck adversarial) & Schema-CLI-Docs Coupling Reviewer | Subagents (`gpt-6.1-sol` & `claude-sonnet-5.5`) | `0 P0, 0 P1, 1 P2 (out-of-scope production code note), 5 P3` — Refined U1 local verification command to PowerShell `$env:GOOS='linux'` syntax, matched optional `./` and `*` prefixes in `Generate SHA256SUMS` verification (`grep -Eq "^[0-9a-f]{64}[[:space:]]+\*?(\./)?${expected}$"`), updated `AssetName` line citation to `internal/updater/updater.go:572-578`, and clarified `"CGO-free release binaries"`. | PASS (all in-scope refinements applied) |

### Findings and Dispositions

* **[P2 — Architecture Strategist, resolved in U2]**: `release.yml` `Generate SHA256SUMS`
  now asserts that all 6 expected `updater.AssetName(goos, goarch)` entries are
  present in `SHA256SUMS` before `Publish release` runs.
* **[P2 — Architecture Strategist, resolved in U2]**: `release.yml` now includes
  a `test-windows` job (`Quality Gates (Windows)`) and sets
  `needs: [test, test-windows]` on `build` so tagged releases gate binary
  publication on both Linux and native Windows verification.
* **[P2 — Rubber-Duck Correctness Reviewer, out-of-scope note]**: Noted that in
  `internal/capture/output.go:219`, explicit `FilePath` overwrite truncates the
  existing `.md` before opening `.mp3`. Because `70ABBB2C` is strictly scoped to
  adding a POSIX mode-bit verification test (`output_unix_test.go`) without
  mutating production code in `internal/capture/output.go` (and hard-link aliasing
  is already guarded by `TestSetupOutputRejectsHardLinkedTranscriptAndAudioAlias`),
  no production code mutation is added to `023-S`.
* **[P3 — Constitution / Rubber-Duck / Coupling Reviewers, resolved in U1/U2/U3]**:
  Updated U1 local verification to PowerShell syntax (`$env:GOOS='linux'`),
  matched `./` prefix in `SHA256SUMS` regex check, updated `AssetName` line
  reference to `572-578`, and used `"CGO-free release binaries"` wording.

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 unresolved P2`). Ready for harvest into
feature `023-F` and shipment `023-S`.

