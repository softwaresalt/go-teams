---
title: Load test-suite identities, URLs, and secrets from gitignored .env.local.test and add tracked .env.example template
description: Deliberation on stash entry ED249B2F for covering feature 028-F and shipment 028-S
topic: Test-Suite .env.local.test Loader and Tracked .env.example Template (Option A)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 017-DL
feature: 028-F
shipment: 028-S
source_stash:
  - ED249B2F
linked_artifacts:
  - docs/plans/2026-11-15-test-env-local-loader-and-example-template-plan.md
  - .backlogit/queue/017-DL.md
  - .backlogit/queue/028-F.md
  - .backlogit/queue/028-S.md
date: 2026-11-15
status: accepted
tags:
  - testing
  - browser
  - cli
  - config
  - security
  - docs
---

## Problem Frame

Integration and end-to-end tests in `github.com/softwaresalt/go-teams` currently read test-suite identities, URLs, browser profile paths, browser executable paths, and compiled binary paths exclusively from process environment variables via direct `os.Getenv` calls:

1. **Browser integration harness (`internal/browser/launch_integration_test.go`, `internal/browser/meeting_integration_test.go`, `//go:build integration`)**:
   * `requireIntegration(t)` reads `os.Getenv("GO_TEAMS_INTEGRATION")`.
   * `meetingURL(t)` reads `os.Getenv("TEAMS_MEETING_URL")`.
   * `browserContext(t)` reads `os.Getenv("TEAMS_PROFILE_DIR")` and does not yet forward `TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT` (`DefaultAccount`) or `TEAMS_BROWSER_PATH` (`ExecPath`) into `browser.Options` and `browser.MeetingOptions`, nor does the harness provide a `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` helper for Meeting Recap integration tests.
2. **Browser and CLI E2E / compiled-binary helpers (`internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, `internal/cli/e2e_test.go`)**:
   * `TestCompiledBinaryCapturesRecapToExplicitOutput(t)` reads `os.Getenv("GO_TEAMS_BINARY")`.
   * `TestChromeMeetingEndToEnd(t)` and `TestCaptionsCommandEndToEndWritesMarkdownFile(t)` call `browser.ResolveExecPath("")`, which checks `os.Getenv("TEAMS_BROWSER_PATH")` before host auto-discovery.
3. **Repository template and secret-hygiene guardrails (`.gitignore`, `internal/config/gitignore_test.go`, `.github/workflows/ci.yml`)**:
   * `.gitignore` already ignores `.env`, `.env.*`, and `*.env` while re-including `!.env.example`, and `internal/config/gitignore_test.go` (`isLocalEnvPath`) together with `.github/workflows/ci.yml` exempt `.env.example` from the local-env filename ban while scanning its content for live passcodes (`livePasscodeRe` / `syntheticPasscodeMarkers`), `SKYPE-TOKEN` JWTs, and `Bearer` JWTs. However, no tracked `.env.example` template exists at the repository root yet.

Stash entry **`ED249B2F`** (`task`, `medium`) requires loading test-suite identities, URLs, and secrets from a gitignored root `.env.local.test` file in integration and E2E test helpers (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `GO_TEAMS_BINARY`) when those environment variables are not already set in the process environment, and adding a tracked `.env.example` template at the repository root containing synthetic placeholder values only.

### Scope Boundaries

* **In scope**:
  * Tracked `.env.example` template at the repository root with synthetic placeholder values only.
  * Secret-hygiene and template validation tests in `internal/config/gitignore_test.go`.
  * Test-only `.env.local.test` loader and alias/precedence helpers in `internal/browser/integration_helper_test.go`, wired into `internal/browser/launch_integration_test.go`, `internal/browser/meeting_integration_test.go`, and `internal/browser/meeting_e2e_test.go`.
  * Test-only `.env.local.test` loader wired into `internal/cli/binary_automation_test.go` and `internal/cli/e2e_test.go`.
  * Developer and integration-test documentation in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md`.
* **Out of scope**:
  * Production CLI runtime code (`cmd/go-teams`, non-test `*.go` files in `internal/`): the production `go-teams` binary reads YAML configuration (`config.yaml`), CLI flags, and documented runtime env vars (`TEAMS_BROWSER_PATH`, `TEAMS_DISABLE_SANDBOX`); it must never read `.env.local.test`.
  * Active stash entry **`3EB89E2D`** (`low`, live-tenant validation follow-up from `022-S`) remains untouched in `active` stash state for the subsequent Option C Stage pass after `028-S` ships.
  * Existing queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) remains untouched.

## Research Findings

### Prior Learnings (`docs/compound/`)

2. **Repository root discovery and path containment (`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`)**: `go test` runs each package with the package directory (`internal/browser`, `internal/cli`, `internal/config`) as the current working directory. Locating root files (`.gitignore`, `config.example.yaml`, `.env.example`, `.env.local.test`) requires walking upward from `os.Getwd()` to the directory containing `go.mod`, and relative file paths written in root `.env.local.test` (such as `GO_TEAMS_BINARY="./bin/go-teams.exe"` or `TEAMS_PROFILE_DIR="./bin/teams-profile"`) must be anchored to that repository root rather than the subpackage working directory.
* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: Credential-bearing and passcode-bearing inputs must never be echoed in error messages or test logs. Any syntax or validation error when parsing `.env.local.test` must report only the file name, 1-based line number or allowlisted key name, and static structural error reason, never the raw line, candidate key text, or secret value.

### Codebase Coupling and Security Analysis

1. **Architecture and Package Dependency Invariant (`docs/ARCHITECTURE.md`)**:
   * `docs/ARCHITECTURE.md` enforces that the module has one entrypoint (`cmd/go-teams`) and ten packages under `internal/`, and that `internal/cli` is the only package importing `internal/config`.
   * Placing `.env.local.test` loading strictly inside `*_test.go` files (`internal/browser/integration_helper_test.go` in `package browser` and `internal/cli/binary_automation_test.go` in `package cli`) keeps 100% of test-env parsing out of production binaries and preserves the 10-package domain map and import direction without modification.
   * Because `internal/browser/integration_helper_test.go` uses `package browser` without a `//go:build integration` tag, its exported `LoadTestEnv(t testing.TB)` helper is compiled and unit-tested during normal `go test -race ./...` while also being directly accessible to `package browser` (`meeting_e2e_test.go`) and `package browser_test` (`launch_integration_test.go`, `meeting_integration_test.go` under `-tags=integration`). Because `LoadTestEnv(t)` synchronizes both primary and alias environment variables (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`) into the test process environment via `t.Setenv`, callers read standard `os.Getenv("<PRIMARY_KEY>")` after `LoadTestEnv(t)` without needing per-variable wrapper functions (which also avoids Go's `Test[A-Z]*` test-entrypoint signature check in `*_test.go` files).
2. **Process-Environment Precedence Across Alias Pairs**:
   * Stash `ED249B2F` specifies alias pairs (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`) alongside `GO_TEAMS_BINARY`, `GO_TEAMS_INTEGRATION`, and `TEAMS_DISABLE_SANDBOX`.
   * Precedence must be evaluated across the entire alias pair in the process environment before falling back to `.env.local.test`: if either `TEAMS_ACCOUNT` or `TEAMS_TEST_ACCOUNT` is already non-empty (`strings.TrimSpace(os.Getenv(k)) != ""`) in the process environment, that process-environment value wins for the pair and `.env.local.test` never overrides it. Only when neither key in the pair is set in the process environment does the helper read the pre-filtered `.env.local.test` map (preferring the primary key `TEAMS_ACCOUNT` over the alias `TEAMS_TEST_ACCOUNT` when both appear in `.env.local.test`).
3. **Unedited `.env.example` Copy Protection (`isTemplatePlaceholder`) and Gitignored Profile Path**:
   * Developers commonly initialize `.env.local.test` via `cp .env.example .env.local.test` and edit only a subset of variables (for example, setting `TEAMS_MEETING_URL` and `TEAMS_PROFILE_DIR` while leaving `GO_TEAMS_BINARY` and `TEAMS_BROWSER_PATH` untouched).
   * Pre-filtering unedited `.env.example` sentinel placeholder values (`strings.Contains(strings.ToLower(val), "synthetic-placeholder") || strings.EqualFold(val, "synthetic-user@example.com")`) out of the parsed `.env.local.test` map before alias-pair resolution (while keeping the low-level `parseDotEnv` parser pure and never filtering process environment variables) ensures `cp .env.example .env.local.test` is safe out of the box and never falls back from an unedited `values[primary]` to an unedited `values[alias]`.
   * In `.env.example`, the synthetic profile directory placeholders use `./bin/teams-profile-synthetic-placeholder` inside the already-gitignored `bin/` directory so that removing `-synthetic-placeholder` in `.env.local.test` never creates an un-ignored Chromium `Cookies` database in the repository root.
4. **Windows Path Backslash Preservation in `.env.local.test`**:
   * On Windows, operators frequently write double-quoted paths with single backslashes in `.env.local.test` (such as `TEAMS_BROWSER_PATH="C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"` or `GO_TEAMS_BINARY=".\bin\go-teams.exe"`).
   * If double-quoted value parsing converted `\t` or `\b` into control characters, paths containing `\teams` or `\bin` would be corrupted. In double-quoted `.env.local.test` values, only `\"` and `\\` are unescaped; all other `\x` sequences are preserved literally.

## Options Evaluated

### Option A: Test-Only `.env.local.test` Helpers in `internal/browser` and `internal/cli` + Tracked `.env.example` (Recommended)

* **Description**:
  * Add tracked `.env.example` at the repository root with synthetic placeholder values passing `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and CI secret-hygiene scanning, and add unit tests in `internal/config/gitignore_test.go`.
  * Add `internal/browser/integration_helper_test.go` (`package browser`, no build tag) implementing fail-closed `.env.local.test` workspace/symlink/size/git-untracked checks, `.env` parsing, allowlisted key + alias-group precedence (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, `GO_TEAMS_INTEGRATION`, `TEAMS_DISABLE_SANDBOX`), repo-root relative path anchoring, and unit tests; wire `LoadTestEnv(t)` into `launch_integration_test.go`, `meeting_integration_test.go`, and `meeting_e2e_test.go`.
  * Wire `.env.local.test` loading into `internal/cli/binary_automation_test.go` and `internal/cli/e2e_test.go` with unit tests in `internal/cli/binary_automation_test.go`.
  * Update `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md`.
* **Pros**:
  * Zero production code or binary footprint added; `.env.local.test` parsing exists exclusively in `*_test.go` files.
  * Preserves the 10-package domain map and downward dependency graph in `docs/ARCHITECTURE.md` (`internal/cli` remains the only importer of `internal/config`).
  * Unit tests for the `.env.local.test` loader run in standard `go test -race ./...` on both `ubuntu-latest` and `windows-latest` CI without requiring `-tags=integration`.
* **Cons**:
  * Small parser helper is kept self-contained in `internal/browser/integration_helper_test.go` and `internal/cli/binary_automation_test.go` so neither package needs a new cross-package test dependency.
* **Effort estimate**: Low–Medium (4 atomic tasks, ~0.5–1 hour each).
* **Fit**: Satisfies all requirements in stash `ED249B2F` and prepares the test harness for Option C (`3EB89E2D`).

### Option B: Export a `.env.local.test` Loader from Production `internal/config` or `internal/browser`

* **Description**: Place the `.env.local.test` loader in a non-test `.go` file in `internal/config` (and import `internal/config` from `internal/browser` tests) or in `internal/browser` (and call it from `internal/cli` tests).
* **Pros**: Single function definition across packages.
* **Cons**: Importing `internal/config` from `internal/browser` violates the explicit architecture invariant in `docs/ARCHITECTURE.md` ("`internal/cli` is the only package that imports `config`"). Exporting a test-only `.env.local.test` loader from production `internal/browser` ships dead test-harness code in the production `go-teams` binary, violating `AGENTS.md` ("No dead code").
* **Effort estimate**: Low.
* **Fit**: Poor — violates architectural or no-dead-code invariants.

### Option C: Create an Eleventh Package `internal/testenv`

* **Description**: Add a new package `internal/testenv` imported only by `*_test.go` files in `internal/browser` and `internal/cli`.
* **Pros**: Shared test-only package without shipping code in the production binary.
* **Cons**: Breaks the "one entrypoint and ten packages under `internal/`" domain map across `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `.github/copilot-instructions.md` for a small test helper needed in only two packages.
* **Effort estimate**: Medium.
* **Fit**: Inferior to Option A — introduces unnecessary package sprawl.

## Trade-off Comparison

| Criterion | Option A: Test-Only Helpers in `browser` & `cli` + `.env.example` | Option B: Exported Loader in `internal/config` or `internal/browser` | Option C: New `internal/testenv` Package |
|---|---|---|---|
| Production binary purity | Preserved (100% in `*_test.go`) | Violated (ships in production package) | Preserved |
| `docs/ARCHITECTURE.md` 10-package & import rules | Preserved | Violated (`browser` -> `config` or dead export) | Requires 11-package architecture change |
| Executed in standard `go test -race ./...` | Yes (`integration_helper_test.go` + `binary_automation_test.go` + `gitignore_test.go`) | Yes | Yes |
| Stash `ED249B2F` compliance | Full | Full | Full |

## Decision

Adopt **Option A** under covering feature **`028-F`** and shipment **`028-S`** (sequenced after `027-S`), decomposed into 4 width-isolated tasks:

1. **`028.001-T` (U1 — `.env.example` & `internal/config/gitignore_test.go`)**: Create the tracked `.env.example` template at the repository root with synthetic placeholder values only (`TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, plus documented `# GO_TEAMS_INTEGRATION` and `# TEAMS_DISABLE_SANDBOX`), and add unit tests in `internal/config/gitignore_test.go` verifying `isLocalEnvPath(".env.local.test") == true`, zero secret-scanner findings on `.env.example`, and synthetic-only placeholder values matching the exact `isTemplatePlaceholder` contract for all active keys.
2. **`028.002-T` (U2 — `internal/browser/integration_helper_test.go`, `launch_integration_test.go`, `meeting_integration_test.go`)**: Create `internal/browser/integration_helper_test.go` (`package browser`) with `.env.local.test` discovery, fail-closed regular-file/size/git-untracked checks (`git ls-files -z -- <relPath>` with `.git/index` fallback), `.env` parsing (preserving Windows single backslashes in double-quoted paths and unquoted URL `#` fragments), allowlisted key + alias-group process-env precedence (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, `GO_TEAMS_INTEGRATION`, `TEAMS_DISABLE_SANDBOX`), repo-root relative path resolution, and unit tests; wire `browser.LoadTestEnv(t)` into `internal/browser/launch_integration_test.go` and `internal/browser/meeting_integration_test.go` (forwarding `ExecPath` and `DefaultAccount` in `browser.Options` and `browser.MeetingOptions`).
3. **`028.003-T` (U3 — `internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, `internal/cli/e2e_test.go`)**: Wire `LoadTestEnv(t)` in `internal/browser/meeting_e2e_test.go` (`TestChromeMeetingEndToEnd`) and implement and wire `loadCLITestEnv(t)` in `internal/cli/binary_automation_test.go` (`TestCompiledBinaryCapturesRecapToExplicitOutput`) and `internal/cli/e2e_test.go` (`TestCaptionsCommandEndToEndWritesMarkdownFile`), with unit tests in `internal/cli/binary_automation_test.go` covering `.env.local.test` loading, process-environment precedence, alias resolution, repo-root relative `GO_TEAMS_BINARY` resolution, and `.env.example` placeholder filtering.
4. **`028.004-T` (U4 — `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `README.md`)**: Update `docs/USER_GUIDE.md` (`### Integration tests`), `docs/QUALITY_SCORE.md` (`### Tier 2` and `### Tier 5`), and `README.md` (`## Development`) to document `.env.example`, gitignored `.env.local.test`, supported test variables and aliases, and process-environment precedence.

## Rejected Alternatives

* **Option B (Exporting `.env.local.test` loader from `internal/config` or `internal/browser`)**: Rejected because it either breaks the `docs/ARCHITECTURE.md` dependency rule (`internal/cli` is the sole importer of `internal/config`) or ships test-only code in the production `go-teams` binary.
* **Option C (Adding `internal/testenv` package)**: Rejected because it alters the 10-package architecture invariant in `docs/ARCHITECTURE.md` for a test-only helper used by two packages.

## Unresolved Questions

None. All affected test files, secret-hygiene scanner rules, and environment variable aliases have been verified against the codebase.

## Risks and Mitigations

* **Risk 1 — Accidental tracking or secret leakage from `.env.local.test` or `.env.example`**:
  * *Mitigation*: `.gitignore` already contains `.env.*` (ignoring `.env.local.test`) followed by `!.env.example`, and `.env.example` places its sample profile path under `./bin/teams-profile-synthetic-placeholder` inside the gitignored `bin/` directory. `LoadTestEnv` fails closed if `.env.local.test` is a symlink, exceeds `64 KiB`, or appears in the Git index (`git ls-files -z -- <relPath>` with `cmd.Dir = repoRoot` and `.git/index` fallback when `git` is unavailable), and never echoes raw line contents, candidate key names, or secret values in error messages. Every URL in `.env.example` uses a `?p=synthetic-placeholder-passcode` token matching `syntheticPasscodeMarkers` in `internal/config/gitignore_test.go` and `.github/workflows/ci.yml`.
* **Risk 2 — Process environment overridden by `.env.local.test` across alias pairs**:
  * *Mitigation*: Evaluate process-environment presence across the entire alias pair (`primary` and `alias`) before consulting `.env.local.test`, and pin all four precedence combinations with table-driven unit tests in `internal/browser/integration_helper_test.go` and `internal/cli/binary_automation_test.go`.
* **Risk 3 — Unedited `cp .env.example .env.local.test` breaking E2E or compiled-binary tests**:
  * *Mitigation*: `applyTestEnvMap` skips unedited `.env.example` sentinel placeholder values (`isTemplatePlaceholder`), so copying `.env.example` to `.env.local.test` without editing `TEAMS_BROWSER_PATH` or `GO_TEAMS_BINARY` preserves browser auto-discovery in `TestChromeMeetingEndToEnd` and clean `t.Skip` behavior in `TestCompiledBinaryCapturesRecapToExplicitOutput`.
