---
title: Windows-only CI quality gates and 3-target release matrix trimming implementation plan
description: Implementation plan for 032-F / 032-S covering stash entry EE2869C2
source: docs/decisions/2026-11-16-windows-ci-and-release-target-trimming-deliberation.md
deliberation_id: 021-DL
feature: 032-F
shipment: 032-S
source_stash:
  - EE2869C2
date: 2026-11-16
status: approved
tags:
  - ci
  - release
  - workflows
  - windows
  - darwin
  - tests
  - docs
---

## Problem Frame

Following the accepted deliberation in [2026-11-16-windows-ci-and-release-target-trimming-deliberation.md](../decisions/2026-11-16-windows-ci-and-release-target-trimming-deliberation.md) (`021-DL`), stash entry **`EE2869C2`** (`priority: high`, `kind: task`) is planned under covering feature **`032-F`** and shipment **`032-S`**, sequenced immediately after `031-S` and before `010-S` in the shipment dependency DAG **`031-S -> 032-S -> 010-S`**.

Currently:

1. **Duplicate Linux Go quality-gate jobs (`jobs.test` on `ubuntu-latest`)**:
   * `.github/workflows/ci.yml` defines both `test` (`Build, vet, lint & test` on `ubuntu-latest`) and `test-windows` (`Build, vet & test (Windows)` on `windows-latest`), plus a commented-out `docs-lint` job.
   * `.github/workflows/release.yml` defines both `test` (`Quality Gates` on `ubuntu-latest`) and `test-windows` (`Quality Gates (Windows)` on `windows-latest`), plus commented-out `npm-package` and `npm-publish` blocks.
   * Because `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`) and `-coverprofile=coverage.out` live in the Linux `test` job, removing `test` requires moving `golangci-lint` and coverage reporting into `test-windows` (`runs-on: windows-latest`) in both workflows and updating `jobs.build.needs` in `release.yml` from `[test, test-windows]` to `[test-windows]`.
2. **Unneeded release build targets (`linux-amd64`, `linux-arm64`, `darwin-arm64`)**:
   * `.github/workflows/release.yml` `jobs.build` uses `strategy.matrix` with `goos: [linux, darwin, windows]` and `goarch: [amd64, arm64]` (6 targets), and `jobs.release` (`Generate SHA256SUMS`) asserts `test "$(wc -l < SHA256SUMS)" -eq 6` across `go-teams-linux-amd64`, `go-teams-linux-arm64`, `go-teams-darwin-amd64`, `go-teams-darwin-arm64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe`.
   * Per `EE2869C2`, only 3 release targets are supported and published: `windows/amd64` (`go-teams-windows-amd64.exe`), `windows/arm64` (`go-teams-windows-arm64.exe`), and `darwin/amd64` (`go-teams-darwin-amd64`).
3. **Documentation and contract enforcement**:
   * `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` still describe 6 release targets (`{linux, darwin, windows} x {amd64, arm64}`) and dual `ubuntu-latest` + `windows-latest` CI test jobs.
   * A hermetic Go test suite (`internal/config/workflow_test.go`) is needed to parse `.github/workflows/ci.yml` and `.github/workflows/release.yml` with `gopkg.in/yaml.v3` and enforce these workflow invariants automatically on every `go test ./...` run.

### Explicit Out-of-Scope Boundaries

* Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `7883544E`, and `39450846` remain untouched in `active` stash state.
* Production Go source files (`internal/updater/updater.go`, `internal/cli/update.go`, `internal/capture/output_unix_test.go`) are unchanged. `updater.AssetName(goos, goarch)` remains a pure naming function tested by hermetic unit fixtures, `internal/cli/update.go` is kept out of scope to confine `032-S` strictly to `.github/workflows/`, `internal/config/workflow_test.go`, and `docs/` per `EE2869C2`, and `internal/capture/output_unix_test.go` remains in the repository (type-checked via `GOOS=darwin GOARCH=amd64 go vet ./...` in `test-windows` and runnable via the cross-compile recipe `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` from `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`).
* Lightweight host-agnostic utility jobs (`changes` in `ci.yml`, and `changelog`, cross-compile `build`, and `release` in `release.yml`) remain on `ubuntu-latest` (1x GitHub Actions billing multiplier) to avoid the 2x `windows-latest` billing multiplier on non-test utility steps.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Remove the Linux `test` (`Build, vet, lint & test` on `ubuntu-latest`) job and commented-out `docs-lint` block from `.github/workflows/ci.yml`; consolidate `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`), `GOOS=darwin GOARCH=amd64 go vet ./...`, `go test -race -coverprofile=coverage.out ./...`, and `go tool cover -func=coverage.out` into `test-windows` (`name: Build, vet, lint & test (Windows)`, `runs-on: windows-latest`) alongside `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, and `TestChromeMeetingEndToEnd` non-skip verification | `EE2869C2` | U1 (`032.001-T`), U2 (`032.002-T`) | `go test -race ./internal/config/... -run TestCIWorkflow` |
| R2: Remove the Linux `test` (`Quality Gates` on `ubuntu-latest`) job and commented-out `npm-package` / `npm-publish` blocks from `.github/workflows/release.yml`; consolidate `go build -o bin/go-teams.exe ./cmd/go-teams`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`), and `-coverprofile=coverage.out` into `test-windows` (`Quality Gates (Windows)`, `runs-on: windows-latest`); update `jobs.build.needs` to `[test-windows]`; replace the 6-target `strategy.matrix` with an explicit 3-target `matrix.include` (`windows/amd64`, `windows/arm64`, `darwin/amd64`); and update `jobs.release` (`Generate SHA256SUMS`) to assert `test "$(wc -l < SHA256SUMS)" -eq 3` and verify only `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe` | `EE2869C2` | U1 (`032.001-T`), U2 (`032.002-T`) | `go test -race ./internal/config/... -run TestReleaseWorkflow` |
| R3: Add hermetic workflow contract tests in `internal/config/workflow_test.go` parsing `.github/workflows/*.yml` via `gopkg.in/yaml.v3` to enforce R1, R2, and `.github/instructions/ci-security.instructions.md` (40-char SHA pinning, `persist-credentials: false`, least-privilege `permissions`, unconditional secret hygiene in `changes`, and `SHA256SUMS` step ordering) | `EE2869C2` | U2 (`032.002-T`) | `go test -race ./internal/config/...`, `go vet ./...`, `golangci-lint version`, `golangci-lint run` |
| R4: Update `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` so all documentation accurately reflects the 3 supported release targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`) and the consolidated `windows-latest` (`test-windows`) quality gate job | `EE2869C2` | U3 (`032.003-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, `backlogit doctor` |

## Implementation Units

### U1 (`032.001-T`) — Consolidate CI and release quality gates onto `test-windows` and trim `release.yml` to `windows-amd64`, `windows-arm64`, and `darwin-amd64`

* **Domain:** config (`.github/workflows/ci.yml`, `.github/workflows/release.yml`). **Posture:** test-first (Principle II — Wave-Staged Red-Phase Harness: stage U2's `workflow_test.go` in `logs/staged-harnesses/032.002-T/` during U1 red-phase verification to prove the tests fail against the unmodified workflows before editing `.github/workflows/ci.yml` and `.github/workflows/release.yml`, then activate `internal/config/workflow_test.go` in U2). **Size:** `S` | **Complexity:** `low`.
* **Files (2 files):** `.github/workflows/ci.yml`, `.github/workflows/release.yml`.
* **What:**
  1. **In `.github/workflows/ci.yml`**:
     * Keep `changes` (`Detect code changes`) unchanged (`runs-on: ubuntu-latest`, unconditional `Secret hygiene check (tracked files)` step).
     * Remove the `test` (`Build, vet, lint & test` on `ubuntu-latest`) job completely.
     * Update `test-windows`:
       * Update the comment above `test-windows` to explain that `test-windows` is the single consolidated Go quality-gate job in CI (running build, vet including `darwin/amd64` cross-target vet, `golangci-lint` via `install-mode: goinstall`, the Windows browser E2E check, and the race-enabled test suite with coverage report on `windows-latest`).
       * Set `name: Build, vet, lint & test (Windows)`.
       * Keep `needs: changes`, `if: needs.changes.outputs.code == 'true'`, and `runs-on: windows-latest`.
       * Steps in `test-windows`:
         1. `uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2` with `persist-credentials: false`
         2. `uses: actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0` with `go-version-file: go.mod`
         3. `name: Install dependencies` -> `run: go mod download`
         4. `name: Build Windows executable` -> `run: go build -o bin/go-teams.exe ./cmd/go-teams`
         5. `name: Vet` -> `run: go vet ./...`
         6. `name: Vet (integration harness)` -> `run: go vet -tags=integration ./...`
         7. `name: Vet (darwin/amd64 release target)` -> `env: { GOOS: darwin, GOARCH: amd64 }`, `run: go vet ./...` (ensures `//go:build unix`/`darwin`-tagged files shipped in `go-teams-darwin-amd64` are always type-checked and vetted in CI)
         8. `name: Lint` -> `uses: golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837 # v6.5.0` with `version: v1.64.8`, `install-mode: goinstall`, and `args: --timeout=5m`
         9. `name: Verify Windows browser E2E does not skip` (`shell: pwsh`, running `go test -v ./internal/browser -run "^TestChromeMeetingEndToEnd$"` and failing on non-zero exit code or `--- SKIP:` / missing `--- PASS: TestChromeMeetingEndToEnd`)
         10. `name: Test` -> `run: go test -race -coverprofile=coverage.out ./...`
         11. `name: Coverage report` -> `run: go tool cover -func=coverage.out`
     * Remove the commented-out `docs-lint` block at the bottom of `.github/workflows/ci.yml`.
  2. **In `.github/workflows/release.yml`**:
     * Remove the `test` (`Quality Gates` on `ubuntu-latest`) job completely.
     * Update `test-windows` (`name: Quality Gates (Windows)`, `runs-on: windows-latest`):
       * Steps:
         1. `uses: actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683 # v4.2.2` with `persist-credentials: false`
         2. `uses: actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b # v5.4.0` with `go-version-file: go.mod`
         3. `name: Install dependencies` -> `run: go mod download`
         4. `name: Build Windows executable` -> `run: go build -o bin/go-teams.exe ./cmd/go-teams`
         5. `name: Vet` -> `run: go vet ./...`
         6. `name: Vet (integration harness)` -> `run: go vet -tags=integration ./...`
         7. `name: Vet (darwin/amd64 release target)` -> `env: { GOOS: darwin, GOARCH: amd64 }`, `run: go vet ./...`
         8. `name: Lint` -> `uses: golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837 # v6.5.0` with `version: v1.64.8`, `install-mode: goinstall`, and `args: --timeout=5m`
         9. `name: Verify Windows browser E2E does not skip` (`shell: pwsh`, running `go test -v ./internal/browser -run "^TestChromeMeetingEndToEnd$"`)
         10. `name: Test` -> `run: go test -race -coverprofile=coverage.out ./...`
     * In `build` (`name: Build release binaries`):
       * Change `needs: [test, test-windows]` to `needs: [test-windows]`.
       * Replace `strategy.matrix` (`goos: [linux, darwin, windows]`, `goarch: [amd64, arm64]`) with an explicit `include:` list of the 3 supported targets only:
         ```yaml
         strategy:
           fail-fast: false
           matrix:
             include:
               - goos: windows
                 goarch: amd64
               - goos: windows
                 goarch: arm64
               - goos: darwin
                 goarch: amd64
         ```
     * Remove the commented-out `npm-package` and `npm-publish` blocks completely.
     * In `release` (`name: Publish GitHub Release`, `Generate SHA256SUMS` step):
       * Change `test "$(wc -l < SHA256SUMS)" -eq 6` to `test "$(wc -l < SHA256SUMS)" -eq 3`.
       * Update the `for expected in ...` loop to list only the 3 published assets:
         ```bash
         for expected in \
           go-teams-darwin-amd64 \
           go-teams-windows-amd64.exe \
           go-teams-windows-arm64.exe; do
         ```
* **Verify:** Run `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Depends on:** none.

### U2 (`032.002-T`) — Add CI and release workflow contract tests in `internal/config/workflow_test.go`

* **Domain:** tests (`internal/config/workflow_test.go`). **Posture:** test-first (activate the Wave-Staged Red-Phase Harness from `logs/staged-harnesses/032.002-T/`). **Size:** `S` | **Complexity:** `low`.
* **Files (1 file, 3 test functions):** `internal/config/workflow_test.go`.
* **What:**
  1. Create `internal/config/workflow_test.go` in `package config`, reusing `findRepoRoot(t)` from `internal/config/gitignore_test.go`, `filepath.Join`, and `gopkg.in/yaml.v3`.
  2. Define typed structs (`workflowFile`, `workflowJob`, `workflowNeeds`, `workflowStrategy`, `workflowMatrix`, `workflowMatrixTarget`, `workflowStep`) to unmarshal `.github/workflows/*.yml`:
     * Define `type workflowNeeds []string` with a custom `UnmarshalYAML(value *yaml.Node) error` method handling both `yaml.ScalarNode` (for example `needs: changes` in `ci.yml`) and `yaml.SequenceNode` (for example `needs: [test-windows]` and `needs: [changelog, build]` in `release.yml`).
     * Define `Permissions map[string]string` on both `workflowFile` and `workflowJob`, `If string` and `ContinueOnError bool` (`yaml:"continue-on-error"`) on `workflowJob` and `workflowStep`, and `With map[string]string` and `Env map[string]string` on `workflowStep` so YAML boolean/integer scalars (`persist-credentials: false`, `fetch-depth: 0`) unmarshal deterministically as strings.
  3. Add 3 unit tests:
     * `TestCIWorkflowRunsWindowsQualityGatesOnly(t *testing.T)`:
       * Reads `filepath.Join(findRepoRoot(t), ".github", "workflows", "ci.yml")`, asserts it unmarshals cleanly, defines `jobs["changes"]` and `jobs["test-windows"]`, and does NOT define `jobs["test"]` or `jobs["docs-lint"]` (nor contain commented-out `docs-lint:` lines).
       * Asserts `jobs["changes"].If == ""` and `!jobs["changes"].ContinueOnError`, and that `jobs["changes"]` contains step `Secret hygiene check (tracked files)` with `If == ""`, `!ContinueOnError`, and `Run` containing `git ls-files`.
       * Asserts `jobs["test-windows"].RunsOn == "windows-latest"`, `Needs` equals `workflowNeeds{"changes"}`, and `If == "needs.changes.outputs.code == 'true'"`.
       * Asserts `jobs["test-windows"].Steps` includes:
         - `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b` (`with["go-version-file"] == "go.mod"`) at a step index strictly before `golangci/golangci-lint-action`
         - `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, and `GOOS=darwin` / `GOARCH=amd64` `go vet ./...`
         - `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` with `version: "v1.64.8"` and `install-mode: "goinstall"`
         - `TestChromeMeetingEndToEnd` non-skip verification (`--- PASS: TestChromeMeetingEndToEnd`)
         - `go test -race -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out`
     * `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly(t *testing.T)`:
       * Reads `filepath.Join(findRepoRoot(t), ".github", "workflows", "release.yml")`, asserts it unmarshals cleanly, defines `jobs["test-windows"]`, `jobs["changelog"]`, `jobs["build"]`, and `jobs["release"]`, and does NOT define `jobs["test"]`, `jobs["npm-package"]`, or `jobs["npm-publish"]` (nor contain commented-out `npm-package:` / `npm-publish:` blocks).
       * Asserts `jobs["test-windows"].RunsOn == "windows-latest"` and its steps include `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b` (`with["go-version-file"] == "go.mod"`) strictly before `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837` (`install-mode: "goinstall"`), `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin` / `GOARCH=amd64` `go vet ./...`, `TestChromeMeetingEndToEnd` non-skip verification, and `go test -race -coverprofile=coverage.out ./...`.
       * Asserts `jobs["build"].Needs` equals `workflowNeeds{"test-windows"}` (no dependency on removed `test`).
       * Asserts `jobs["build"].Strategy.Matrix.GOOS` is empty, `jobs["build"].Strategy.Matrix.GOArch` is empty, and `jobs["build"].Strategy.Matrix.Include` equals the exact 3-element slice:
         1. `{GOOS: "windows", GOArch: "amd64"}`
         2. `{GOOS: "windows", GOArch: "arm64"}`
         3. `{GOOS: "darwin", GOArch: "amd64"}`
         (explicitly verifying zero `linux` targets and zero `darwin`/`arm64` target).
       * Asserts `jobs["release"].Needs` equals `workflowNeeds{"changelog", "build"}`, and in `jobs["release"].Steps`, `Generate SHA256SUMS` precedes `Publish release`, neither step sets `If` or `ContinueOnError`, and `Generate SHA256SUMS` contains `test "$(wc -l < SHA256SUMS)" -eq 3` and `exit 1` (without `|| true`), checks every asset derived from `jobs["build"].Strategy.Matrix.Include` (`go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, `go-teams-windows-arm64.exe`), and does NOT contain `go-teams-linux-amd64`, `go-teams-linux-arm64`, `go-teams-darwin-arm64`, or `-eq 6`.
     * `TestWorkflowsPinThirdPartyActionsAndDisableCredentials(t *testing.T)`:
       * Enumerates all workflow files matching `filepath.Join(findRepoRoot(t), ".github", "workflows", "*.yml")` (`ci.yml`, `release.yml`, `cli-reference-drift.yml`).
       * Asserts each workflow's top-level `Permissions` equals `map[string]string{"contents": "read"}`, and that only `jobs["release"]` in `release.yml` grants `"contents": "write"` (no other job in any workflow grants a `write` permission).
       * Iterates over all jobs and steps in each workflow file, allowing local `./` reusable references per `.github/instructions/ci-security.instructions.md` while asserting every external `uses:` reference matches `^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*@[0-9a-f]{40}$` and every `actions/checkout@` step sets `with["persist-credentials"] == "false"`.
* **Verify:** Run `go test -race -v ./internal/config/...`, `go vet ./...`, `golangci-lint version` (confirming built with Go >= 1.26), `golangci-lint run`, and `gofmt -l .`.
* **Depends on:** `032.001-T`.

### U3 (`032.003-T`) — Align documentation with Windows-only CI quality gates and 3 release targets

* **Domain:** docs (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`). **Posture:** migration-first. **Size:** `S` | **Complexity:** `low`.
* **Files (3 files):** `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`.
* **What:**
  1. **In `docs/USER_GUIDE.md` (`## Self-update`)**:
     * Update the first bullet under `## Self-update` (currently stating that `release.yml` builds and publishes binaries for `linux`, `darwin`, and `windows` on both `amd64` and `arm64` after the Linux and Windows quality gates pass, and that `SHA256SUMS` lists six platform entries) to state that `.github/workflows/release.yml` builds and publishes CGO-free (`CGO_ENABLED=0`) binaries for `windows/amd64` (`go-teams-windows-amd64.exe`), `windows/arm64` (`go-teams-windows-arm64.exe`), and `darwin/amd64` (`go-teams-darwin-amd64`) together with `SHA256SUMS` after the Windows quality gate (`test-windows`) passes, that publication fails if `SHA256SUMS` lacks any of those three platform entries, and that platforms outside those three published targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`) return `ErrAssetNotFound`.
  2. **In `docs/QUALITY_SCORE.md`**:
     * Update frontmatter `date: 2026-11-16`.
     * In `## Grades by package` (`internal/capture`), update the rationale to note that `internal/capture/output_unix_test.go` asserts `0o700` and `0o600` creation and preservation modes under `syscall.Umask(0)` (type-checked in CI via `GOOS=darwin GOARCH=amd64 go vet ./...` and runnable locally via the `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` cross-compile recipe), while CI consolidates Go test execution onto `windows-latest` (`test-windows`).
     * In `### Tier 1: unit and concurrency suite`, update the prose to state that `.github/workflows/ci.yml` and `.github/workflows/release.yml` run the consolidated quality-gate suite on `windows-latest` (`test-windows`), covering `go build`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run` (`install-mode: goinstall`), the non-skipped `TestChromeMeetingEndToEnd` browser E2E check, and `go test -race -coverprofile=coverage.out ./...`, while `//go:build unix` mode tests in `internal/capture/output_unix_test.go` remain available via `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c ./internal/capture`.
     * In `## Live-verification gaps`, update the cross-compiled release binary row to state that the `darwin/amd64` and `windows/arm64` release binaries are cross-compiled on `ubuntu-latest` at release time and never execute a native test suite (only `windows/amd64` on `windows-latest` runs the native test suite in CI, with `GOOS=darwin GOARCH=amd64 go vet ./...` type-checking `unix`/`darwin` files), and that `//go:build unix` permission tests in `internal/capture/output_unix_test.go` do not execute at runtime in CI after `032-S` (tracking `023-S`, `032-S`).
  3. **In `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`**:
     * In `## Release discovery and transport trust`, update the first bullet to state that the updater selects `go-teams-<goos>-<goarch>[.exe]` and that `.github/workflows/release.yml` builds CGO-free binaries for `windows/amd64`, `windows/arm64`, and `darwin/amd64` (cross-compiled on `ubuntu-latest` after `test-windows` passes) and verifies that `SHA256SUMS` lists all three published assets (ensuring manifest completeness and digest integrity) before publishing.
     * In `## Consequences and known limits`, update the final bullet to state that `.github/workflows/release.yml` publishes `windows/amd64`, `windows/arm64`, and `darwin/amd64` release binaries (`linux/*` and `darwin/arm64` are omitted in `032-S` to conserve GitHub Actions budget) and that `windows/amd64` (`windows-latest`) executes the native quality-gate and test suite in CI.
* **Verify:** Run `go test -race ./internal/config/...`, `go vet ./...`, and `backlogit doctor`.
* **Depends on:** `032.001-T`.

## Dependency Graph

```text
032.001-T (config: .github/workflows/ci.yml, .github/workflows/release.yml)
  ├──> 032.002-T (tests: internal/config/workflow_test.go)
  └──> 032.003-T (docs: docs/USER_GUIDE.md, docs/QUALITY_SCORE.md, docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md)
```

Shipment-level dependency order: **`031-S -> 032-S -> 010-S`** (`031-S` is already `shipped`; `032-S` depends on `031-S` and blocks `010-S`).

## Decisions and Rationale

1. **Consolidating `golangci-lint` and coverage into `test-windows` (`windows-latest`)**:
   * **Rationale**: Previously, `golangci-lint` (`install-mode: goinstall`) and `-coverprofile=coverage.out` ran only in the `ubuntu-latest` `test` job. Moving both steps into `test-windows` in `.github/workflows/ci.yml` and `.github/workflows/release.yml` ensures Principle I (`golangci-lint run`) and test coverage reporting remain enforced on every code push, PR, and release tag after the Linux `test` job is removed.
2. **Using an explicit `matrix.include` in `.github/workflows/release.yml`**:
   * **Rationale**: Because the 3 supported targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`) are an asymmetric subset of `{windows, darwin} x {amd64, arm64}`, an explicit 3-entry `matrix.include` list is clearer and less error-prone than a Cartesian product with `exclude:`.
3. **Keeping lightweight non-test utility jobs (`changes`, `changelog`, `build`, `release`) on `ubuntu-latest`**:
   * **Rationale**: GitHub Actions bills `windows-latest` at 2x the per-minute rate of `ubuntu-latest`. Retaining `ubuntu-latest` for the 10-second `changes` path-filter/secret-hygiene job and the tag-only `changelog`, `CGO_ENABLED=0` cross-compile `build` (now 3 legs instead of 6), and `release` jobs maximizes GitHub Actions budget conservation while eliminating all Linux Go test jobs and Linux/`darwin-arm64` release targets.
4. **Placing workflow contract tests in `internal/config/workflow_test.go`**:
   * **Rationale**: `internal/config` is a leaf package that already imports `gopkg.in/yaml.v3` and already houses repository-hygiene tests (`internal/config/gitignore_test.go` with `findRepoRoot(t)`). Co-locating `workflow_test.go` in `internal/config` requires zero new dependencies and runs automatically in `go test -race ./...`.

## Risks and Caveats

* **Risk — Branch protection / ruleset status-check context names**:
  * **Mitigation**: GitHub status-check contexts use the job `name:` (`Detect code changes` and `Build, vet, lint & test (Windows)`). Verify during PR CI validation (`gh pr checks`) that all required status checks resolve and pass cleanly.
* **Risk — `golangci-lint` compilation time on `windows-latest` (2x billing multiplier)**:
  * **Mitigation**: Per `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`, `install-mode: goinstall` adds ~2-3 minutes per run (roughly 4-6 billed minutes on `windows-latest` at 2x), which is still a net GitHub Actions budget reduction compared to running a separate `ubuntu-latest` job that downloads modules, builds, vets, and executes the entire `go test -race ./...` suite a second time, plus 3 removed release build matrix legs. `golangci/golangci-lint-action` sets `--timeout=5m` and benefits from `actions/setup-go`'s built-in Go module and build cache.

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Present** — changes the published GitHub Release asset matrix contract from 6 targets to 3 targets (`windows/amd64`, `windows/arm64`, `darwin/amd64`) and consolidates CI/release quality gates onto `test-windows`.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — modifies `.github/workflows/ci.yml` and `.github/workflows/release.yml`, which are governed by `.github/instructions/ci-security.instructions.md` (immutable 40-char SHA pinning, explicit least-privilege `permissions`, `persist-credentials: false`, and `SHA256SUMS` release verification).
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — workflow and documentation configuration changes only, reversible via `git revert`.
* **External integration, operator checkpoint, or external dependency**: **Absent** — no live tenant checkpoint required; verified via hermetic Go unit tests and GitHub Actions PR checks.
* **High runtime, rollout, or rollback risk**: **Absent** — zero production Go runtime changes.

Requires plan hardening: yes

## Runtime Verification and Closure

* **U1 (`032.001-T` — `.github/workflows/ci.yml`, `.github/workflows/release.yml`)**:
  * **Runtime surface**: GitHub Actions CI and Release pipelines.
  * **Verification**: Verified locally by `internal/config/workflow_test.go` (`go test -race ./internal/config/...`) and in PR CI by GitHub Actions executing `changes` (`Detect code changes`) and `test-windows` (`Build, vet, lint & test (Windows)`) on the pull request.
* **U2 (`032.002-T` — `internal/config/workflow_test.go`)**:
  * **Runtime surface**: Hermetic unit test suite (`internal/config`).
  * **Verification**: `go test -race -v ./internal/config/...`, `go vet ./...`, `golangci-lint version` (Go >= 1.26), `golangci-lint run`, and `gofmt -l .`.
* **U3 (`032.003-T` — `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md`)**:
  * **Runtime surface**: Operator and developer documentation.
  * **Verification**: `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` in `internal/config/gitignore_test.go` and `backlogit doctor`.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan modifies `.github/workflows/ci.yml` and `.github/workflows/release.yml` (governed by `.github/instructions/ci-security.instructions.md` and `.github/instructions/workflows.instructions.md`) and changes the published release asset matrix and `SHA256SUMS` verification contract.
* **Safety Mode Posture (Principle VI)**: `careful` + `freeze-scope` (strictly bounded to `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `internal/config/workflow_test.go`, and the 3 documentation files) + `strict-safety`.
* **Lightweight 3-Threat Model**:
  1. **Most likely threat**: Future workflow edits accidentally weaken CI security invariants (broadening `permissions`, adding `if:` or `continue-on-error` to the secret-hygiene step, or introducing unpinned actions). Mitigated by `TestWorkflowsPinThirdPartyActionsAndDisableCredentials` and `TestCIWorkflowRunsWindowsQualityGatesOnly` in `internal/config/workflow_test.go`.
  2. **Highest-impact threat**: `release.yml` publishes an incomplete or mismatched `SHA256SUMS` manifest or skips quality gates before building release binaries. Mitigated by `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly` asserting `build.needs == ["test-windows"]`, `release.needs == ["changelog", "build"]`, and `Generate SHA256SUMS` step ordering and fail-closed `-eq 3` verification.
  3. **Most subtle threat**: Removing the Linux `test` job drops compile/vet coverage of `//go:build unix` / `darwin`-tagged files that still ship in `go-teams-darwin-amd64`, or breaks `actions/setup-go` ordering before `golangci-lint` (`install-mode: goinstall`). Mitigated by adding `GOOS=darwin GOARCH=amd64 go vet ./...` in `test-windows` and asserting `setup-go` step ordering before `golangci-lint-action` in `internal/config/workflow_test.go`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/ci-security.instructions.md`
  * `.github/instructions/workflows.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `.github/instructions/writing-style.instructions.md`
  * `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **CI Security Pinning and Least-Privilege Credentials (`.github/instructions/ci-security.instructions.md`)**: Every external GitHub Action in `.github/workflows/*.yml` MUST remain pinned to an immutable 40-character commit SHA (`actions/checkout@11bd71901bbe5b1630ceea73d27597364c9af683`, `dorny/paths-filter@d1c1ffe0248fe513906c8e24db8ea791d46f8590`, `actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b`, `golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837`, `taiki-e/install-action@e03236526ace47fa2e04bebcfc6da471ebd4690c`, `actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02`, `actions/download-artifact@95815c38cf2ff2164869cbab79da8d1f422bc89e`, `softprops/action-gh-release@c95fe1489396fe8a9eb87c0abf8aa5b2ef267fda`), top-level `permissions: contents: read` MUST be preserved across all workflows (with `contents: write` scoped exclusively to `jobs.release` in `release.yml`), and every `actions/checkout` step MUST set `persist-credentials: false`.
2. **Unconditional Tracked-File Secret Hygiene Scan (`ci.yml` `jobs.changes`)**: The `changes` job and its `Secret hygiene check (tracked files)` step in `.github/workflows/ci.yml` MUST remain unconditional (`If == ""`, `!ContinueOnError`) on every push and pull request (including docs/backlog-only changes).
3. **Complete Windows Quality Gate Enforcement (`test-windows`)**: In both `.github/workflows/ci.yml` and `.github/workflows/release.yml`, `test-windows` (`runs-on: windows-latest`) MUST run `actions/setup-go` (`go-version-file: go.mod`) strictly before `golangci/golangci-lint-action` (`version: v1.64.8`, `install-mode: goinstall`, `args: --timeout=5m`), plus `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, the non-skipped `TestChromeMeetingEndToEnd` check, and `go test -race -coverprofile=coverage.out ./...`.
4. **Exact 3-Target Release Matrix and `SHA256SUMS` Integrity (`release.yml`)**: `.github/workflows/release.yml` `jobs.build` (`needs: [test-windows]`) MUST build only `{windows/amd64, windows/arm64, darwin/amd64}`, and `jobs.release` (`needs: [changelog, build]`, `Generate SHA256SUMS` preceding `Publish release` with no `if:` or `continue-on-error:`) MUST verify `test "$(wc -l < SHA256SUMS)" -eq 3` and require exact digest lines for `go-teams-darwin-amd64`, `go-teams-windows-amd64.exe`, and `go-teams-windows-arm64.exe` before `Publish release` executes.
5. **Zero Unscoped Production Code or Stash Mutation**: No production `.go` files are modified in `032-S`, and all other stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `7883544E`, `39450846`) remain untouched.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-032-1` | Remove duplicate Linux `test` jobs and dead commented-out blocks from `.github/workflows/ci.yml` and `.github/workflows/release.yml`, consolidate `golangci-lint`, `GOOS=darwin GOARCH=amd64 go vet ./...`, and coverage into `test-windows`, and trim `release.yml` `build` matrix and `SHA256SUMS` verification to `windows-amd64`, `windows-arm64`, and `darwin-amd64` | `.github/workflows/ci.yml`, `.github/workflows/release.yml` | CI/CD workflow configuration update | `moderate` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U1 commit |
| `PA-032-2` | Add hermetic YAML workflow contract tests asserting CI/release job topology, 3-target release matrix, `SHA256SUMS` verification, least-privilege `permissions`, and SHA-pinning / `persist-credentials: false` compliance | `internal/config/workflow_test.go` | Unit test addition | `low` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U2 commit |
| `PA-032-3` | Align user guide, quality score, and self-update design documentation with Windows-only CI quality gates and the 3 published release targets | `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/design-docs/2026-11-11-self-update-and-interprocess-locking.md` | Documentation update | `low` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U3 commit |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint version` (Go >= 1.26), `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages, including `TestCIWorkflowRunsWindowsQualityGatesOnly`, `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly`, and `TestWorkflowsPinThirdPartyActionsAndDisableCredentials` in `internal/config/workflow_test.go`.
  * [ ] GitHub Actions PR checks (`Detect code changes` and `Build, vet, lint & test (Windows)`) pass on the pull request for `032-S` (authoritative CI lint and test verification).
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `ci_windows_quality_gate_pass_rate` (`test-windows` in `ci.yml` == 100%, including `golangci-lint`, `GOOS=darwin GOARCH=amd64 go vet ./...`, and `TestChromeMeetingEndToEnd`), (2) `workflow_contract_test_pass_rate` (`go test -race ./internal/config/...` == 100%), and (3) `tracked_secret_leak_count` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == 0`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main` (and next `v*.*.*` release tag push).
  * **Dashboard / query**: `gh run list --workflow=ci.yml` and `gh run list --workflow=release.yml`.
  * **Baseline**: 100% pass rate on `changes` and `test-windows`, 0 linter issues, 0 secret-hygiene findings.
  * **Alert threshold**: `>= 1` workflow job failure on `main` or tag push.
* **Rollback trigger & procedure**:
  * **Trigger**: Any failure in `test-windows` (`ci.yml` or `release.yml`) or `build` / `release` (`SHA256SUMS` verification) caused by the workflow consolidation.
  * **Procedure**: Revert the `032-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, `internal/config/workflow_test.go` contract tests, full `go test -race ./...` suite, and GitHub Actions PR checks (`changes`, `test-windows`) pass and are recorded in `docs/closure/` (followed by mandatory post-merge context compaction under P-020).
  * `READY_WITH_CONDITIONS`: All local quality gates and PR CI checks pass, with any environment-specific condition documented and verified.
  * `BLOCKED`: Any pre-deploy audit check, unit test, or PR CI check fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, and `Security Lens Reviewer`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `Autopilot Mode`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings from the 6-persona review panel were remediated in-place in `docs/plans/2026-11-16-windows-ci-and-release-target-trimming-plan.md` and `docs/decisions/2026-11-16-windows-ci-and-release-target-trimming-deliberation.md` prior to harvest; zero P0 findings).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-032-1`..`PA-032-3`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` (anchor route `gpt-6.1-sol`) | `subagent` (`multi-agent`) | 0 P0/P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 6 P2, 3 P3 | Remediated — `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0, 1 P1, 1 P2, 1 P3 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Learnings Researcher` | `subagent` (`multi-agent`) | 0 P0, 5 P1/P2 learning-alignment items | Remediated — `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Go Reviewer`] Custom `yaml.Unmarshaler` (`workflowNeeds`) for scalar vs. sequence `needs:`, `With map[string]string`, and complete `release.yml` step assertions**:
   * **Finding**: In GitHub Actions YAML, `needs:` can be a scalar (`needs: changes` in `ci.yml`) or a sequence (`needs: [test-windows]` in `release.yml`), which fails `yaml.Unmarshal` into a plain `[]string`; `With` needs `map[string]string` so YAML boolean `false` unmarshals to `"false"`; and `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly` should also assert the build and vet steps in `release.yml`.
   * **Remediation Applied**: Updated U2 to define `type workflowNeeds []string` implementing `UnmarshalYAML(value *yaml.Node) error` for both `yaml.ScalarNode` and `yaml.SequenceNode`, typed `With` and `Env` as `map[string]string`, and added `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, and `go vet -tags=integration ./...` assertions to `TestReleaseWorkflowBuildsThreeSupportedTargetsOnly`.
2. **[P1 — `Learnings Researcher`, `Security Lens Reviewer`] `setup-go` step ordering before `golangci-lint-action`, `GOOS=darwin GOARCH=amd64 go vet ./...` in `test-windows`, and `GOOS=linux go test -c` cross-compile recipe**:
   * **Finding**: `golangci-lint` (`install-mode: goinstall`) requires `actions/setup-go` (`go-version-file: go.mod`) to run before the lint step; removing the Linux `test` job would otherwise leave `//go:build unix`/`darwin`-tagged files (shipped in `go-teams-darwin-amd64`) unvetted in CI; and `docs/QUALITY_SCORE.md` should cite the `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` recipe from `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`.
   * **Remediation Applied**: Added `Vet (darwin/amd64 release target)` (`GOOS=darwin GOARCH=amd64 go vet ./...`) to `test-windows` in both `ci.yml` and `release.yml` (U1), added `setup-go`-before-`golangci-lint-action` step-order assertions in U2, and updated U3 and Risks/Caveats to cite the `GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c` recipe and the `windows-latest` 2x billing trade-off.
3. **[P2 — `Security Lens Reviewer`, `Architecture Strategist`] Enforcing least-privilege `permissions`, unconditional `changes` secret-hygiene step, `SHA256SUMS` step ordering, and all `.github/workflows/*.yml` files in U2**:
   * **Remediation Applied**: Updated U2 so `TestWorkflowsPinThirdPartyActionsAndDisableCredentials` enumerates all `.github/workflows/*.yml` files (`ci.yml`, `release.yml`, `cli-reference-drift.yml`), allows local `./` reusable references while requiring 40-char SHA pins on all external `uses:` references, verifies top-level `permissions: contents: read` and that only `release.yml` `jobs.release` grants `contents: write`, verifies `changes` and `Secret hygiene check (tracked files)` have no `if:` or `continue-on-error:`, and verifies `Generate SHA256SUMS` precedes `Publish release` and checks the exact 3 assets derived from `build.strategy.matrix.include`.
