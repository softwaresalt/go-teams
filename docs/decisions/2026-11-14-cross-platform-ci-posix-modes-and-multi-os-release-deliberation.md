---
title: Cross-platform CI, POSIX permission verification, and multi-OS release asset parity decision
description: Deliberation on grouping 020-S closure follow-ups 70ABBB2C, D3A9CBD0, and EC6D1CAF into feature 023-F and shipment 023-S
topic: Cross-Platform CI, POSIX Permission Verification, and Multi-OS Release Asset Parity (70ABBB2C + D3A9CBD0 + EC6D1CAF)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 012-DL
feature: 023-F
shipment: 023-S
source_stash:
  - 70ABBB2C
  - D3A9CBD0
  - EC6D1CAF
linked_artifacts:
  - docs/plans/2026-11-14-cross-platform-ci-posix-modes-and-multi-os-release-plan.md
  - .backlogit/queue/012-DL.md
  - .backlogit/queue/023-F.md
  - .backlogit/queue/023-S.md
date: 2026-11-14
status: accepted
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

This deliberation covers the contextual grouping of three active stash entries
originating from the `020-S` post-merge closure audit
([2026-11-11-020-S-review-and-closure.md](../closure/2026-11-11-020-S-review-and-closure.md))
into a single cohesive release unit under covering feature **`023-F`** and
shipment **`023-S`**:

1. **`70ABBB2C`** (`task`, `low`, follow-up from `020-S` closure):
   > Follow-up from 020-S closure
   > (docs/closure/2026-11-11-020-S-review-and-closure.md): add a POSIX-only
   > test asserting the 0o700 directory and 0o600 file modes created by
   > internal/capture; QUALITY_SCORE.md grades the package B until it exists.
2. **`D3A9CBD0`** (`task`, `low`, follow-up from `020-S` closure):
   > Follow-up from 020-S closure
   > (docs/closure/2026-11-11-020-S-review-and-closure.md): add a Windows CI
   > job so Windows-only tests run outside developer machines; ci.yml currently
   > builds the Windows executable on ubuntu-latest only.
3. **`EC6D1CAF`** (`task`, `low`, follow-up from `020-S` closure):
   > Follow-up from 020-S closure (README Self-update note): release.yml builds
   > only windows/amd64 and windows/arm64 assets, so teams-cli update returns
   > ErrAssetNotFound on Linux and macOS; decide whether to publish non-Windows
   > release assets or document Windows-only support.

### Why These Entries Belong Together

* **Shared origin and cross-platform verification theme**: All three entries
  were stashed together during the `020-S` documentation and quality-score audit
  as the remaining hermetic cross-platform verification and release-parity gaps
  that do not require a live Microsoft Teams tenant.
* **Complementary OS test and release coverage**:
  * `70ABBB2C` adds a `//go:build unix` test (`internal/capture/output_unix_test.go`)
    that executes on the `ubuntu-latest` CI runner to verify `0o700` directory
    and `0o600` file permissions under `syscall.Umask(0)`.
  * `D3A9CBD0` adds a `windows-latest` CI runner job (`test-windows`) to
    `.github/workflows/ci.yml` so Windows-specific tests (`TestChromeMeetingEndToEnd`
    in `internal/browser` and Windows `.old` rename window tests in
    `internal/updater`) execute on every code change.
  * `EC6D1CAF` expands `.github/workflows/release.yml` to build and publish
    CGO-free (`CGO_ENABLED=0`) release binaries for `linux`, `darwin`, and
    `windows` across `amd64` and `arm64`, matching `updater.AssetName(goos, goarch)`
    so `teams-cli update` works across all three operating systems.
* **Single documentation pass (`023.003-T`)**: All three items converge on the
  same four documentation files (`README.md`, `docs/QUALITY_SCORE.md`,
  `docs/ARCHITECTURE.md`, and
  `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`),
  avoiding three separate documentation-update PRs.

### Explicit Scope Boundary (Excluded Items)

* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and
  stash entry **`3EB89E2D`** (live-tenant validation of scheduled meeting join,
  overtime continuation, and delayed AI summary Recap enrichment) are
  **explicitly out of scope** and left untouched because they require a live
  Microsoft Teams tenant, an authenticated browser profile, and interactive
  human checkpoints while the operator is AFK.

## P-021 C5 and C6 Triage Record

* None of `70ABBB2C`, `D3A9CBD0`, or `EC6D1CAF` carries the
  `DEFERRED SCOPE EXPANSION` marker.
* **Obligation (A) Unconditional duplicate detection**: Executed across
  `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`.
  Outcome: **CLEAN DUPLICATE SCAN** for `70ABBB2C`, `D3A9CBD0`, and `EC6D1CAF`.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (none of the
  three entries has `N/A` deferred-expansion source refs).

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/build-errors/golangci-lint-go-version-mismatch.md`**:
  `golangci-lint` v1.64.8 in `.github/workflows/ci.yml` uses
  `install-mode: goinstall` so it compiles from source with the `setup-go` Go
  1.26 toolchain, adding ~2–3 minutes of build time. Keeping `golangci-lint` on
  the `ubuntu-latest` `test` job while running `go build`, `go vet ./...`,
  `go vet -tags=integration ./...`, and `go test -race ./...` on
  `windows-latest` avoids doubling linter compilation time in CI while still
  running `go vet` and `go test -race` natively on Windows.
* **`docs/compound/build-errors/shine-mp3-race-checkptr.md`**:
  `internal/audio` allocates `encoderTailSlack` capacity slack so `shine-mp3`
  is `checkptr`-safe under `go test -race ./...` on all platforms, including
  `windows-latest`.
* **`docs/compound/workflow-issues/amend-breaks-shipment-gate.md`**:
  Never amend or rebase commits after marking backlogit items done.

### Codebase Investigation

* **`internal/capture/output.go`**:
  * Defines `dirPerm = 0o700` and `filePerm = 0o600`.
  * `setupOutput(cfg OutputConfig)` creates missing directories with
    `os.MkdirAll(..., dirPerm)` (both when `cfg.FilePath` is set and when
    `cfg.Dir` is used), creates the Markdown transcript file with
    `os.OpenFile(mdPath, openFlg, filePerm)`, and (when `!cfg.CaptionsOnly`)
    creates the MP3 audio file with `os.OpenFile(mp3Path, openFlg, filePerm)`.
  * When the output directory already exists, `os.MkdirAll` leaves the
    pre-existing directory's permissions intact.
  * `AppendAISummaryToFile(transcriptPath, summary string)` opens the existing
    transcript file with `os.OpenFile(absPath, os.O_WRONLY|os.O_APPEND, filePerm)`
    without loosening its permissions.
* **`.github/workflows/ci.yml`**:
  * `changes` job runs `dorny/paths-filter` and the unconditional tracked-file
    secret hygiene check, outputting `code: ${{ steps.filter.outputs.code }}`.
  * `test` job (`runs-on: ubuntu-latest`, `needs: changes`,
    `if: needs.changes.outputs.code == 'true'`) uses SHA-pinned
    `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2` with
    `persist-credentials: false` and
    `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0` with
    `go-version-file: go.mod`.
* **`internal/updater/updater.go` and `.github/workflows/release.yml`**:
  * `updater.AssetName(goos, goarch)` returns
    `"teams-cli-" + goos + "-" + goarch` with `".exe"` appended only when
    `goos == "windows"`.
  * `updater` already contains Linux- and macOS-specific logic
    (`/proc/self/exe` snapshot on Linux in `captureProcessSnapshot`,
    `caseInsensitiveSuffixes()` for `windows` and `darwin`), and `go-teams` has
    zero CGO dependencies.
  * `.github/workflows/release.yml` `build` job currently sets
    `goos: [windows]`, `goarch: [amd64, arm64]` and hardcodes `.exe` in the `-o`
    path (`-o dist/teams-cli-${{ matrix.goos }}-${{ matrix.goarch }}.exe`).
    The downstream `release` job computes `SHA256SUMS` via
    `find . -maxdepth 1 -type f -name 'teams-cli-*' -print0 | sort -z | xargs -0 -r sha256sum > SHA256SUMS`,
    which already matches both `teams-cli-<goos>-<goarch>` and
    `teams-cli-windows-<goarch>.exe` without modification.

## Options Evaluated

### Option A: Minimalist / Windows-Only Release Posture + Matrixed Lint in CI

* Add a basic POSIX mode test in `internal/capture/output_unix_test.go` without
  resetting `syscall.Umask(0)`.
* Convert the existing `test` job in `.github/workflows/ci.yml` into a
  2-OS matrix (`ubuntu-latest`, `windows-latest`) that runs `golangci-lint`
  (`install-mode: goinstall`) on both OSes.
* Keep `.github/workflows/release.yml` Windows-only (`goos: [windows]`) and
  document `teams-cli update` as unsupported on Linux and macOS.

* **Pros**: No change to `release.yml` build matrix.
* **Cons**:
  * Without `syscall.Umask(0)`, a runner with a restrictive umask (`0o077`)
    could pass the mode check even if `dirPerm` / `filePerm` were accidentally
    widened to `0o777` / `0o666` in code.
  * Running `golangci-lint` (`install-mode: goinstall`) on `windows-latest`
    re-compiles the linter from source on Windows (~3–4 extra minutes) and
    renames the existing `Build, vet, lint & test` status check.
  * Leaves `teams-cli update` broken (`ErrAssetNotFound`) on Linux and macOS
    despite `go-teams` being a pure-Go binary with cross-platform `updater`
    support already implemented and tested.
* **Effort**: low | **Fit**: low.

### Option B: `syscall.Umask(0)` POSIX Mode Suite + Dedicated `test-windows` CI Job + 6-Target Multi-OS Release Matrix (chosen)

1. **`70ABBB2C` — POSIX Permission Verification (`023.001-T`, domain: tests)**:
   * Create `internal/capture/output_unix_test.go` gated by `//go:build unix`.
   * Use a helper that sets `oldMask := syscall.Umask(0)` and restores
     `defer syscall.Umask(oldMask)` (running sequentially without `t.Parallel()`
     because `umask` is process-global) so mode assertions test the exact
     `dirPerm` (`0o700`) and `filePerm` (`0o600`) constants passed by
     `internal/capture` unaffected by the host runner's ambient umask.
   * Verify:
     1. `setupOutput` creating a new nested output directory (`Dir`) and new
        `.md` + `.mp3` files produces `dirInfo.Mode().Perm() == 0o700` and
        `fileInfo.Mode().Perm() == 0o600` for both files.
     2. `setupOutput` with an explicit `FilePath` in a new parent directory
        creates the parent directory with `0o700` and the `.md` and `.mp3` files
        with `0o600`.
     3. `setupOutput` into a pre-existing directory with custom permissions
        (for example `0o750`) preserves the directory's `0o750` mode without
        clobbering it, while still creating the `.md` and `.mp3` files inside it
        with `0o600`.
     4. `AppendAISummaryToFile` appends the `# AI Summary` section to an
        existing `0o600` transcript file and preserves `0o600` permissions.
2. **`D3A9CBD0` & `EC6D1CAF` — Windows CI Job & Multi-OS Release Matrix (`023.002-T`, domain: config/workflows)**:
   * In `.github/workflows/ci.yml`, add job `test-windows`:
     - `name: Build, vet & test (Windows)`
     - `needs: changes`
     - `if: needs.changes.outputs.code == 'true'`
     - `runs-on: windows-latest`
     - Steps: SHA-pinned `actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2`
       with `persist-credentials: false`, SHA-pinned
       `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0`
       with `go-version-file: go.mod`, `go mod download`,
       `go build -o bin/teams-cli.exe ./cmd/teams-cli`, `go vet ./...`,
       `go vet -tags=integration ./...`, and `go test -race ./...`.
   * In `.github/workflows/release.yml`, update `test-windows`, `build`, and `release`:
     - Add job `test-windows` (`name: Quality Gates (Windows)`, `runs-on: windows-latest`) running `actions/checkout` (`persist-credentials: false`), `actions/setup-go` (`go-version-file: go.mod`), `go mod download`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -race ./...`, and update `build` to `needs: [test, test-windows]` so tag pushes gate release binary builds on both Linux and native Windows verification without duplicating `golangci-lint`.
     - Rename `build` job `name: Build release binaries`
     - Set `strategy.fail-fast: false` and expand `strategy.matrix` to
       `goos: [linux, darwin, windows]` and `goarch: [amd64, arm64]`.
     - In the build step (`name: Build release executable`, `shell: bash`), set
       `CGO_ENABLED: "0"`, `GOOS: ${{ matrix.goos }}`, and
       `GOARCH: ${{ matrix.goarch }}`, compute `EXT=""` (`if [ "${GOOS}" = "windows" ]; then EXT=".exe"; fi`),
       and build `-o "dist/teams-cli-${GOOS}-${GOARCH}${EXT}" ./cmd/teams-cli`
       so every artifact name matches `updater.AssetName(goos, goarch)` and is
       hashed into `SHA256SUMS` by the downstream `release` job.
     - In the `release` job's `Generate SHA256SUMS` step, verify that all six
       expected `updater.AssetName(goos, goarch)` entries
       (`teams-cli-linux-amd64`, `teams-cli-linux-arm64`,
       `teams-cli-darwin-amd64`, `teams-cli-darwin-arm64`,
       `teams-cli-windows-amd64.exe`, `teams-cli-windows-arm64.exe`) are present
       in `SHA256SUMS` (matching the optional `./` or `*` prefix stripped by
       `updater.parseChecksums`) before publishing the GitHub Release.
3. **Documentation & Quality Score Upgrade (`023.003-T`, domain: docs)**:
   * Update `docs/QUALITY_SCORE.md`:
     - Upgrade `internal/capture` from Grade `B` to Grade `A` (citing
       `internal/capture/output_unix_test.go` `0o700` / `0o600` mode assertions
       under `syscall.Umask(0)`).
     - Update `### Tier 1: unit and concurrency suite` and
       `### Tier 2: synthetic browser and compiled-binary end to end` to reflect
       that CI runs on both `ubuntu-latest` and `windows-latest` (while noting
       that `darwin` release binaries remain cross-compiled on `ubuntu-latest`
       and `TestCompiledBinaryCapturesRecapToExplicitOutput` remains gated on
       `GO_TEAMS_BINARY`).
     - Remove the two closed rows from `## Live-verification gaps` (Windows CI
       gap and `internal/capture` POSIX mode gap) and mark items 3 and 4 in
       `## Improvement roadmap` as completed in `023-S`.
   * Update `README.md`:
     - In `### Self-update`, replace the Windows-only `ErrAssetNotFound` caveat
       with documentation that `.github/workflows/release.yml` publishes
       CGO-free (`CGO_ENABLED=0`) release binaries for `linux`, `darwin`, and
       `windows` (`amd64` and `arm64`) matching `teams-cli-<goos>-<goarch>[.exe]`
       alongside `SHA256SUMS`.
   * Update `docs/ARCHITECTURE.md`:
     - In `## Security and isolation invariants` (`Output permissions`), note
       that the `0o700` directory and `0o600` file creation modes are verified
       by POSIX unit tests under `syscall.Umask(0)`.
   * Update `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`:
     - In `## Release discovery and transport trust` and
       `## Consequences and known limits`, note that `release.yml` publishes
       `linux`, `darwin`, and `windows` (`amd64` and `arm64`) assets and that
       the Windows rename window is now exercised in CI on `windows-latest`.

* **Pros**: Closes all three `020-S` follow-up gaps completely, lifts
  `internal/capture` to Grade `A`, runs Windows-specific browser and updater
  tests in CI on every code change without duplicating slow linter builds, and
  enables `teams-cli update` across Linux, macOS, and Windows (`amd64`/`arm64`).
* **Cons**: Cross-compiling 6 release binaries in `release.yml` adds 4 fast
  pure-Go matrix legs on tag pushes (~30s each on `ubuntu-latest`).
* **Effort**: low-medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (No `Umask(0)`, matrixed lint, Windows-only release) | Option B (`Umask(0)` POSIX tests, dedicated `test-windows` job, 6-target release matrix) |
|---|---|---|
| Proves exact `0o700` / `0o600` mode bits regardless of runner umask | No (ambient umask can mask overly permissive constants) | Yes (`syscall.Umask(0)` + `defer syscall.Umask(oldMask)`) |
| Exercises Windows-only tests in CI (`TestChromeMeetingEndToEnd`, `updater` `.old` window) | Yes, but wastes ~3–4m re-building `golangci-lint` from source on Windows | Yes, fast dedicated `test-windows` job (`build`, `vet`, `test -race`) |
| `teams-cli update` support across Linux, macOS, and Windows (`amd64` & `arm64`) | No (`ErrAssetNotFound` on Linux and macOS) | Yes (all 6 `teams-cli-<goos>-<goarch>[.exe]` binaries + `SHA256SUMS`) |
| Lifts `internal/capture` to Grade `A` in `docs/QUALITY_SCORE.md` | Partial | Yes (full Grade `A` criteria met) |

## Decision

We select **Option B** and decompose the work into covering feature **`023-F`**
and shipment **`023-S`** across three width-isolated tasks (`023.001-T`,
`023.002-T`, and `023.003-T`), each scoped within a single skill domain
(`tests`, `config`/workflows, and `docs`).

## Rejected Alternatives

* **Option A** was rejected because relying on ambient runner umask weakens the
  `0o700` / `0o600` permission test, compiling `golangci-lint` from source a
  second time on `windows-latest` adds unnecessary CI latency, and keeping
  `release.yml` Windows-only leaves `teams-cli update` broken on Linux and macOS
  when cross-compiling pure-Go binaries on `ubuntu-latest` is trivial.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **`syscall.Umask` is process-global on Unix**:
  Mitigated by omitting `t.Parallel()` in `internal/capture/output_unix_test.go`
  and restoring the previous mask immediately via `defer syscall.Umask(oldMask)`
  inside a narrow helper around `setupOutput` / `AppendAISummaryToFile`.
* **GitHub Actions security compliance (`.github/instructions/ci-security.instructions.md`)**:
  Mitigated by pinning `actions/checkout` and `actions/setup-go` in
  `test-windows` to the exact immutable full-length commit SHAs already audited
  in `.github/workflows/ci.yml` and setting `persist-credentials: false` on
  `actions/checkout`.
* **Release asset naming parity with `updater.AssetName(goos, goarch)`**:
  Mitigated by computing `EXT=".exe"` only when `GOOS == "windows"` and empty
  otherwise, matching `internal/updater/updater.go:572-578` (`AssetName`) and
  the `find . -maxdepth 1 -type f -name 'teams-cli-*'` glob in the `release`
  job's `Generate SHA256SUMS` step.
