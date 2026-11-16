---
title: Load test-suite identities, URLs, and secrets from gitignored .env.local.test and add tracked .env.example template implementation plan
description: Implementation plan for 028-F / 028-S covering stash entry ED249B2F
source: docs/decisions/2026-11-15-test-env-local-loader-and-example-template-deliberation.md
deliberation_id: 017-DL
feature: 028-F
shipment: 028-S
source_stash:
  - ED249B2F
date: 2026-11-15
status: approved
tags:
  - testing
  - browser
  - cli
  - config
  - security
  - docs
---

## Problem Frame

Following the accepted deliberation in [2026-11-15-test-env-local-loader-and-example-template-deliberation.md](../decisions/2026-11-15-test-env-local-loader-and-example-template-deliberation.md) (`017-DL`), stash entry **`ED249B2F`** (`task`, `medium`) is planned under covering feature **`028-F`** and shipment **`028-S`** (sequenced after `027-S`).

Integration and E2E test helpers in `internal/browser` (`launch_integration_test.go`, `meeting_integration_test.go`, `meeting_e2e_test.go`) and `internal/cli` (`binary_automation_test.go`, `e2e_test.go`) currently require all test-suite identities, URLs, profile paths, browser paths, and compiled binary paths to be exported in the process environment before `go test` runs, and the repository does not yet include a tracked `.env.example` template at the repository root. This plan delivers:

1. A tracked root `.env.example` template containing synthetic placeholder values only (`TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, plus documented `# GO_TEAMS_INTEGRATION` and `# TEAMS_DISABLE_SANDBOX`), validated by `internal/config/gitignore_test.go` and `.github/workflows/ci.yml`.
2. A test-only `.env.local.test` loader and alias/precedence helper in `internal/browser/integration_helper_test.go` (`package browser`), wired into `internal/browser/launch_integration_test.go` and `internal/browser/meeting_integration_test.go`.
3. Test-only `.env.local.test` loading wired into `internal/browser/meeting_e2e_test.go` (`TestChromeMeetingEndToEnd`), `internal/cli/binary_automation_test.go` (`TestCompiledBinaryCapturesRecapToExplicitOutput`), and `internal/cli/e2e_test.go` (`TestCaptionsCommandEndToEndWritesMarkdownFile`).
4. Updated developer and test-tier documentation in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md`.

### Explicit Out-of-Scope Boundaries

* Production CLI code (`cmd/go-teams`, non-test `*.go` files under `internal/`) is never modified; `.env.local.test` loading is strictly confined to `*_test.go` test helpers.
* Active stash entry **`3EB89E2D`** (`low`, live-tenant validation follow-up from `022-S`) remains in `active` stash state and is deferred to the Option C Stage pass after `028-S` ships.
* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) remains untouched.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Add tracked `.env.example` at the repository root with synthetic placeholder values only (`TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, plus documented `# GO_TEAMS_INTEGRATION` and `# TEAMS_DISABLE_SANDBOX`) that pass `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and CI's secret-hygiene check, and add regression tests in `internal/config/gitignore_test.go` | `ED249B2F` | U1 (`028.001-T`) | `go test -race ./...` and `go vet ./...` |
| R2: Implement fail-closed root `.env.local.test` discovery, regular-file/symlink/size/git-untracked validation, `.env` parsing, allowlisted key + alias-group process-environment precedence, and repo-root relative path anchoring in `internal/browser/integration_helper_test.go` (`LoadTestEnv`), and wire into `internal/browser/launch_integration_test.go` and `internal/browser/meeting_integration_test.go` | `ED249B2F` | U2 (`028.002-T`) | `go test -race ./...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/browser/...` |
| R3: Wire `.env.local.test` loading in `internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, and `internal/cli/e2e_test.go`, with unit tests in `internal/cli/binary_automation_test.go` verifying `.env.local.test` loading, process-environment precedence, alias resolution, unedited `.env.example` placeholder filtering, and repo-root relative `GO_TEAMS_BINARY` resolution | `ED249B2F` | U3 (`028.003-T`) | `go test -race ./...` and `go vet ./...` |
| R4: Document `.env.example` and `.env.local.test` workflow, supported variables and aliases, and process-environment precedence in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md` | `ED249B2F` | U4 (`028.004-T`) | `go test -race ./...` (including `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) |

## Implementation Units

### U1 (`028.001-T`) — Add tracked `.env.example` template with synthetic placeholders and secret-hygiene regression tests in `internal/config/gitignore_test.go`

* **Domain:** test (`.env.example`, `internal/config/gitignore_test.go`). **Posture:** test-first (co-located `gitignore_test.go` assertions written first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Files:** `.env.example`, `internal/config/gitignore_test.go`.
* **What:**
  1. **Red phase (`internal/config/gitignore_test.go`)**:
     * In `TestIsLocalEnvPath`, add `assert.True(t, isLocalEnvPath(".env.local.test"))` and `assert.True(t, isLocalEnvPath("sub/.env.local.test"))` alongside the existing `.env`, `.env.local`, and `.env.example` assertions.
     * Add `TestEnvExampleTemplateContainsOnlySyntheticPlaceholders(t *testing.T)`:
       * Read `filepath.Join(findRepoRoot(t), ".env.example")`, require no error and non-empty content, and assert `.env.example` is present in `trackedFiles(root)`.
       * Assert `assert.Empty(t, scanTrackedContent(".env.example", data))` (zero live passcode, `SKYPE-TOKEN`, or `Bearer` findings).
       * Parse every non-empty, non-comment line in `.env.example`: assert that every active line is a valid `KEY=VALUE` assignment belonging to the exact expected 11-key template set (`TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`), that all 11 keys are present with non-empty unquoted values, and that every value satisfies the exact `isTemplatePlaceholder` contract (`strings.Contains(strings.ToLower(val), "synthetic-placeholder") || strings.EqualFold(val, "synthetic-user@example.com")`) without echoing secret values on failure.
       * Assert that `GO_TEAMS_INTEGRATION` and `TEAMS_DISABLE_SANDBOX` are documented in comments (and not enabled as active `1`/`true` assignments) so copying `.env.example` to `.env.local.test` never enables live browser tests or disables the Chromium sandbox implicitly.
  2. **Green phase (`.env.example`)**:
     * Create `.env.example` at the repository root (LF line endings) with header instructions explaining how to copy `.env.example` to `.env.local.test`, that `.env.local.test` is gitignored via `.env.*`, and that process environment variables always take precedence over `.env.local.test`.
     * Populate synthetic placeholder assignments for all eleven keys (placing the profile placeholder under the gitignored `./bin/` directory so removing `-synthetic-placeholder` never creates an un-ignored Chromium `Cookies` database in the repository root):
       * `TEAMS_ACCOUNT="synthetic-user@example.com"`
       * `TEAMS_TEST_ACCOUNT="synthetic-user@example.com"`
       * `TEAMS_MEETING_URL="https://teams.microsoft.com/meet/000000000000?p=synthetic-placeholder-passcode"`
       * `TEAMS_TEST_MEETING_URL="https://teams.microsoft.com/meet/000000000000?p=synthetic-placeholder-passcode"`
       * `TEAMS_RECAP_URL="https://teams.microsoft.com/l/meetingrecap?driveId=b!synthetic-placeholder&p=synthetic-placeholder-passcode"`
       * `TEAMS_TEST_RECAP_URL="https://teams.microsoft.com/l/meetingrecap?driveId=b!synthetic-placeholder&p=synthetic-placeholder-passcode"`
       * `TEAMS_PROFILE_DIR="./bin/teams-profile-synthetic-placeholder"`
       * `TEAMS_TEST_PROFILE_DIR="./bin/teams-profile-synthetic-placeholder"`
       * `TEAMS_BROWSER_PATH="./synthetic-placeholder-browser"`
       * `TEAMS_TEST_BROWSER_PATH="./synthetic-placeholder-browser"`
       * `GO_TEAMS_BINARY="./bin/go-teams-synthetic-placeholder"`
     * Include commented `# GO_TEAMS_INTEGRATION="1"` and `# TEAMS_DISABLE_SANDBOX="0"` notes.
  3. **Verify:** Stage `.env.example` in the Git index and run `go test -race ./...` and `go vet ./...`.

### U2 (`028.002-T`) — Implement `.env.local.test` loader (`LoadTestEnv`) in `internal/browser/integration_helper_test.go` and wire `launch_integration_test.go` and `meeting_integration_test.go`

* **Domain:** test (`internal/browser/*_test.go`). **Posture:** test-first (unit tests in `integration_helper_test.go` written first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `028.001-T` (`blocks`).
* **Files:** `internal/browser/integration_helper_test.go`, `internal/browser/launch_integration_test.go`, `internal/browser/meeting_integration_test.go`.
* **What:**
  1. **Create `internal/browser/integration_helper_test.go` (`package browser`, no build tag, `<= 4` helper functions and `<= 3` table-driven `Test*` scenarios)**:
     * Implement `parseDotEnv(data []byte) (map[string]string, error)`:
       * Strip optional UTF-8 BOM (`\xef\xbb\xbf`), normalize `\r\n` to `\n`, skip blank lines and `#` comment lines, and strip optional `export ` prefixes.
       * Require `KEY=VALUE` format with a valid ASCII identifier key (`[A-Za-z_][A-Za-z0-9_]*`). On syntax error (missing `=`, invalid key, or unclosed quote), return an error citing only the 1-based line number and static reason (`fmt.Errorf("line %d: <reason>", lineNum)`), never echoing the raw line, candidate key text, or secret value.
       * Support unquoted values (stripping inline `#` comments only when `#` is preceded by space or tab so URL `#` fragments are preserved), single-quoted `'...'` values (verbatim), and double-quoted `"..."` values (unescaping `\"` and `\\` while preserving Windows path backslashes such as `\t` or `\b` literally).
     * Implement `loadTestEnvFromFile(t testing.TB, envPath, repoRoot string)` and `LoadTestEnv(t testing.TB)` (with GoDoc comment):
       * Track per-`testing.TB` execution in a `sync.Map` (cleaned up via `t.Cleanup`) so repeated `LoadTestEnv(t)` calls within the same test (or after `loadTestEnvFromFile` in unit tests) are fast idempotent no-ops.
       * `LoadTestEnv(t)` calls `t.Helper()`, locates `repoRoot` by walking upward from `os.Getwd()` for `go.mod`, and delegates to `loadTestEnvFromFile(t, filepath.Join(repoRoot, ".env.local.test"), repoRoot)`.
       * `loadTestEnvFromFile` checks `os.Lstat(envPath)`: returns cleanly when `errors.Is(err, os.ErrNotExist)`; fails via `t.Fatalf` when `info.Mode()&os.ModeSymlink != 0` or `!info.Mode().IsRegular()`; resolves `filepath.EvalSymlinks` on **both** `repoRoot` and `envPath` and verifies `filepath.Rel(realRoot, realEnv)` satisfies `filepath.IsLocal(rel)`; opens `envPath`, verifies `f.Stat().Size() <= 64<<10` (`64 KiB`), and reads via `io.LimitReader(f, (64<<10)+1)`.
       * When a `.git` directory/file exists at `repoRoot`, verify via `git ls-files -z -- <relPath>` (with `cmd.Dir = repoRoot`, or `.git/index` inspection if the `git` binary is unavailable) that `.env.local.test` is NOT tracked in the Git index — failing closed via `t.Fatalf` if `.env.local.test` is tracked or if `.git` exists and index verification fails.
       * Pre-filter unedited `.env.example` sentinel placeholder values (`strings.Contains(strings.ToLower(val), "synthetic-placeholder") || strings.EqualFold(val, "synthetic-user@example.com")`) out of the parsed `.env.local.test` map before resolving keys.
       * Apply only the allowlisted test keys (`GO_TEAMS_INTEGRATION`, `TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, `TEAMS_DISABLE_SANDBOX`) using `t.Setenv`:
         * For each alias pair (`TEAMS_ACCOUNT` <-> `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` <-> `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` <-> `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` <-> `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` <-> `TEAMS_TEST_BROWSER_PATH`), check the process environment (`strings.TrimSpace(os.Getenv(k)) != ""`) across both keys first: if either `primary` or `alias` is non-empty in the process environment, synchronize the unset partner via `t.Setenv` and never overwrite either from `.env.local.test`. Only when both are unset in the process environment, pick `values[primary]` (if non-empty) else `values[alias]` and set both via `t.Setenv`.
         * For path-valued keys (`TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`) loaded from `.env.local.test`, when the value is a relative path (not `filepath.IsAbs(p)`, not starting with `/` or `\`, and without a Windows `[A-Za-z]:` volume prefix; for `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`, only when starting with `.` or containing `/` or `\` so bare executable names like `msedge` stay on `PATH`), normalize `/` and `\` to `filepath.Separator`, reject `"."` or any path failing `filepath.IsLocal(cleaned)` (without echoing the path value), and anchor to `filepath.Join(repoRoot, cleaned)`.
     * Write 3 focused table-driven unit tests in `internal/browser/integration_helper_test.go` (clearing all 13 allowlisted keys via `t.Setenv(k, "")` at the start of each subtest):
       1. `TestParseDotEnvSyntaxAndRedaction`: covers unquoted/single-quoted/double-quoted values, Windows backslash preservation (`\t`, `\b`), URL `#fragment` vs inline `# comment`, UTF-8 BOM + CRLF + `export ` prefix, and line-numbered syntax errors that never echo raw lines, candidate keys, or secret values.
       2. `TestLoadTestEnvPrecedenceAndPaths`: covers missing `.env.local.test` no-op, loading all allowlisted keys (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, `GO_TEAMS_INTEGRATION`, `TEAMS_DISABLE_SANDBOX`), process-env precedence when primary or alias is pre-set in process env vs `.env.local.test`, primary-over-alias precedence within `.env.local.test`, per-`testing.TB` idempotency, and repo-root relative path anchoring.
       3. `TestLoadTestEnvSecurityAndTemplateGuardrails`: covers loading the repository's real `.env.example` (asserting all allowlisted keys remain empty), rejecting symlinks, oversized files (`> 64 KiB`), `..` path traversal, and Git-tracked `.env.local.test` files.
  2. **Wire `internal/browser/launch_integration_test.go` and `internal/browser/meeting_integration_test.go` (`//go:build integration`, `package browser_test`)**:
     * In `internal/browser/launch_integration_test.go`: in `requireIntegration(t)`, call `browser.LoadTestEnv(t)` first and skip when `strings.ToLower(strings.TrimSpace(os.Getenv("GO_TEAMS_INTEGRATION")))` is `""`, `"0"`, or `"false"`; in `TestChromeRunnerLaunch(t)`, pass `ExecPath: os.Getenv("TEAMS_BROWSER_PATH")` in `browser.Options`.
     * In `internal/browser/meeting_integration_test.go`:
       * In `meetingURL(t)`: call `requireIntegration(t)` (which invokes `browser.LoadTestEnv(t)` and synchronizes `TEAMS_MEETING_URL` <- `TEAMS_TEST_MEETING_URL`) and read `raw := os.Getenv("TEAMS_MEETING_URL")`.
       * In `browserContext(t)`: call `requireIntegration(t)`, read `profile := os.Getenv("TEAMS_PROFILE_DIR")` (synchronized with `TEAMS_TEST_PROFILE_DIR`), and pass `ExecPath: os.Getenv("TEAMS_BROWSER_PATH")` and `DefaultAccount: os.Getenv("TEAMS_ACCOUNT")` in `browser.Options`.
       * In `newMeeting(t)`: add `t.Helper()` at entry and pass `DefaultAccount: os.Getenv("TEAMS_ACCOUNT")` (synchronized with `TEAMS_TEST_ACCOUNT`) in `browser.MeetingOptions`.
  3. **Verify:** Run `go test -race ./...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/browser/...`.

### U3 (`028.003-T`) — Wire `.env.local.test` loading in `internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, and `internal/cli/e2e_test.go`

* **Domain:** test (`internal/browser/meeting_e2e_test.go`, `internal/cli/*_test.go`). **Posture:** test-first (unit tests in `binary_automation_test.go` written first per Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `028.002-T` (`blocks`).
* **Files:** `internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, `internal/cli/e2e_test.go`.
* **What:**
  1. **In `internal/browser/meeting_e2e_test.go`**:
     * Call `LoadTestEnv(t)` at the start of `TestChromeMeetingEndToEnd(t)` before `if ResolveExecPath("") == ""` so `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH` from `.env.local.test` is honored when no browser is in the default system path, while keeping the test's `httptest.NewServer` URL and `t.TempDir()` profile 100% hermetic.
  2. **In `internal/cli/binary_automation_test.go` and `internal/cli/e2e_test.go`**:
     * Implement `loadCLITestEnv(t testing.TB)` and `loadCLITestEnvFromFile(t testing.TB, envPath, repoRoot string)` in `internal/cli/binary_automation_test.go` (`<= 4` helper functions) enforcing the identical fail-closed `.env.local.test` rules (symmetrical `filepath.EvalSymlinks` on `repoRoot` and `envPath`, open-handle `f.Stat()` + `io.LimitReader(f, (64<<10)+1)`, `git ls-files -z -- <relPath>` untracked check with `.git/index` fallback, pre-filtered `isTemplatePlaceholder`, allowlisted alias-pair precedence, and repo-root relative path anchoring with `filepath.IsLocal`).
     * Call `loadCLITestEnv(t)` at the start of `TestCompiledBinaryCapturesRecapToExplicitOutput(t)` (`internal/cli/binary_automation_test.go`) before reading `os.Getenv("GO_TEAMS_BINARY")`, and at the start of `TestCaptionsCommandEndToEndWritesMarkdownFile(t)` (`internal/cli/e2e_test.go`) before calling `browser.ResolveExecPath("")`.
     * Add 3 focused unit tests in `internal/cli/binary_automation_test.go` (clearing all allowlisted keys via `t.Setenv(k, "")` at the start of each subtest):
       1. `TestLoadCLITestEnvPrecedenceAndPaths`: verifies `.env.local.test` fallback loading, process-environment precedence across primary and alias keys, Windows backslash preservation, and repo-root relative `GO_TEAMS_BINARY` resolution.
       2. `TestLoadCLITestEnvSecurityAndTemplateGuardrails`: verifies loading the repository's real `.env.example` leaves all allowlisted variables unset, and rejects symlinks, oversized files, `..` path traversal, tracked `.env.local.test` files, and malformed lines without echoing secrets.
  3. **Verify:** Run `go test -race ./...` and `go vet ./...`.

### U4 (`028.004-T`) — Document `.env.local.test` and `.env.example` in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md`

* **Domain:** docs (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `README.md`). **Posture:** characterization-first. **Size:** `XS` | **Complexity:** `low`.
* **Depends on:** `028.001-T`, `028.002-T`, `028.003-T` (`blocks`).
* **Files:** `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `README.md`.
* **What:**
  1. **In `docs/USER_GUIDE.md` (`### Integration tests`)**:
     * Document that integration and E2E test helpers automatically load root `.env.local.test` (when present) for variables not already set in the process environment, and that `.env.example` provides a tracked template with synthetic placeholders (`cp .env.example .env.local.test`).
     * Expand the environment variable table under `### Integration tests` to list `GO_TEAMS_INTEGRATION`, `TEAMS_ACCOUNT` (alias `TEAMS_TEST_ACCOUNT`), `TEAMS_MEETING_URL` (alias `TEAMS_TEST_MEETING_URL`), `TEAMS_RECAP_URL` (alias `TEAMS_TEST_RECAP_URL`), `TEAMS_PROFILE_DIR` (alias `TEAMS_TEST_PROFILE_DIR`), `TEAMS_BROWSER_PATH` (alias `TEAMS_TEST_BROWSER_PATH`), `GO_TEAMS_BINARY`, and `TEAMS_DISABLE_SANDBOX`, noting that process environment variables always take precedence over `.env.local.test` and that `.env.local.test` is gitignored and must never be committed.
  2. **In `docs/QUALITY_SCORE.md` (`### Tier 2` and `### Tier 5`)**:
     * Note in Tier 2 (`TestCompiledBinaryCapturesRecapToExplicitOutput` and `TestChromeMeetingEndToEnd`) and Tier 5 (`integration`-tagged live harness) that test helpers fall back to gitignored root `.env.local.test` (seeded from `.env.example`) when environment variables are not already set in the process environment.
  3. **In `README.md` (`## Documentation` and `## Development`)**:
     * Reference `.env.example` / `.env.local.test` alongside the integration-test pointer to `docs/USER_GUIDE.md#development`.
  4. **Verify:** Run `go test -race ./...` (verifying `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) and `engram --workspace "C:\Source\GitHub\go-teams" verify` on modified markdown files.

## Dependency Graph

```text
028.001-T (U1: .env.example + internal/config/gitignore_test.go)
    |
    +---> 028.002-T (U2: internal/browser/integration_helper_test.go + launch_integration_test.go + meeting_integration_test.go)
              |
              +---> 028.003-T (U3: internal/browser/meeting_e2e_test.go + internal/cli/binary_automation_test.go + internal/cli/e2e_test.go)
                        |
                        +---> 028.004-T (U4: docs/USER_GUIDE.md + docs/QUALITY_SCORE.md + README.md)
```

* Shipment sequencing: `028-S` depends on `027-S` (`backlogit dep add 028-S 027-S --type blocks` — `027-S` is already shipped).

## Decisions and Rationale

1. **Test-only (`*_test.go`) `.env.local.test` helpers preserving the 10-package architecture**: Placing the loader in `internal/browser/integration_helper_test.go` (`package browser`, no build tag) and `internal/cli/binary_automation_test.go` (`package cli`) keeps all `.env.local.test` code out of the production `go-teams` binary, preserves the 10-package domain map and import direction in `docs/ARCHITECTURE.md`, and ensures the loader's unit tests execute on every `go test -race ./...` run in CI.
2. **Process-environment precedence across alias pairs**: Evaluating both keys of an alias pair (`TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL` / `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH` / `TEAMS_TEST_BROWSER_PATH`) in the process environment before consulting `.env.local.test` guarantees that setting either the primary or alias variable in the shell always overrides `.env.local.test`.
3. **Unedited `.env.example` placeholder filtering (`isTemplatePlaceholder`)**: Ignoring `.env.example` sentinel placeholder values when applying `.env.local.test` allows developers to run `cp .env.example .env.local.test` and edit only the subset of variables they need without breaking browser auto-discovery in `TestChromeMeetingEndToEnd` or skip behavior in `TestCompiledBinaryCapturesRecapToExplicitOutput`.

## Risks and Caveats

* **Secret leakage via tracked `.env.local.test` or error messages**: If an operator force-adds `.env.local.test` (`git add -f .env.local.test`), `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` (`isLocalEnvPath`), `.github/workflows/ci.yml`, and `LoadTestEnv`'s `git ls-files -z -- <relPath>` check (with `.git/index` fallback) all fail closed. Parse and validation errors in `parseDotEnv` / `loadTestEnvFromFile` report only the file path, 1-based line number or allowlisted key name, and static structural reason, never the raw line text, candidate key text, or secret value.
* **Windows path backslashes in double-quoted `.env.local.test` values**: Unescaping only `\"` and `\\` in double-quoted `.env` values prevents `\t` or `\b` in Windows paths (such as `C:\Users\alice\AppData\Local\go-teams\profile` or `.\bin\go-teams.exe`) from being corrupted into control characters.
* **Subpackage working directory during `go test`**: Because `go test` runs with the package directory (`internal/browser` or `internal/cli`) as `os.Getwd()`, relative path values in root `.env.local.test` (`./bin/go-teams.exe`, `.\bin\go-teams.exe`, `./bin/teams-profile`) are resolved against `repoRoot` (`findRepoRoot`) and checked with `filepath.IsLocal` against `..` traversal escaping `repoRoot`.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, no `unsafe`, zero lint findings, all errors checked |
| II. Test-First Development | Compliant — U1, U2, and U3 specify failing unit tests in `gitignore_test.go`, `integration_helper_test.go`, and `binary_automation_test.go` before implementation |
| III. Workspace Isolation | Compliant — `.env.local.test` is verified to be a regular file inside `repoRoot` (symmetrical `EvalSymlinks` + `filepath.IsLocal`), bounded to `64 KiB`, gitignored, and relative paths are blocked from traversing above `repoRoot` |
| IV. CLI Workspace Containment | Compliant — all test file operations use `t.TempDir()` and read-only repository-root lookup |
| V. Structured Observability | Compliant — parse/validation failures report structured file+line diagnostics without echoing secret values |
| VI. Single Responsibility | Compliant — zero new third-party dependencies (`go.mod` unchanged); 10-package layout preserved; every task has `<= 3` files, `< 5` functions, and `<= 3` test scenarios |
| VII. Destructive Command Approval | Compliant — no destructive commands used |
| VIII. Explicit Safety Modes | Compliant — `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — `.env.example`, markdown docs, and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and unit tests used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No production CLI flags, config keys, or exported package APIs are changed.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Introduces `.env.local.test` loading for test-suite credentials, meeting/recap URLs (with passcodes), and authenticated browser profile directories, plus a tracked `.env.example` file scanned by CI secret-hygiene gates.
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Additive test-helper and template files only.
* **External integration, operator checkpoint, or external dependency**: **Absent** — No new third-party dependencies.
* **High runtime, rollout, or rollback risk**: **Absent** — Confined to `*_test.go` helpers, `.env.example`, and documentation.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * Tracked test-environment template (`.env.example`) and secret-hygiene test suite (`internal/config/gitignore_test.go`)
  * Browser integration and E2E test helpers (`internal/browser/integration_helper_test.go`, `launch_integration_test.go`, `meeting_integration_test.go`, `meeting_e2e_test.go`)
  * CLI E2E and compiled-binary test helpers (`internal/cli/binary_automation_test.go`, `internal/cli/e2e_test.go`)
  * Developer documentation (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `README.md`)
* **Runtime verification**:
  * Run `go test -race ./...` (including `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, `TestEnvExampleTemplateContainsOnlySyntheticPlaceholders`, `internal/browser` `.env.local.test` loader unit tests, `TestChromeMeetingEndToEnd`, and `internal/cli` `.env.local.test` loader unit tests).
  * Build `.\bin\go-teams.exe` (`go build -o .\bin\go-teams.exe .\cmd\go-teams`) and run `TestCompiledBinaryCapturesRecapToExplicitOutput` with `GO_TEAMS_BINARY` supplied via `.env.local.test` or process env.
  * Run `go vet ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -run '^$' ./internal/browser/...`, `golangci-lint run`, and `gofmt -l .`.
* **Operational closure**:
  * Pre-deploy audit checklist, post-merge observation window, rollback trigger, and releasability evidence contract (`READY`) recorded in `docs/closure/2026-11-15-028-S-review-and-closure.md`.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan touches secret-hygiene guardrails (`internal/config/gitignore_test.go`, `.env.example`) and introduces `.env.local.test` loading for sensitive test-suite inputs (`TEAMS_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_PROFILE_DIR`).
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **Process Environment Precedence (Non-Overwriting Invariant)**: `LoadTestEnv` and `loadCLITestEnv` MUST NEVER overwrite an environment variable (or alias pair) that is already non-empty (`strings.TrimSpace(os.Getenv(k)) != ""`) in the process environment. If either `primary` (e.g., `TEAMS_ACCOUNT`) or `alias` (e.g., `TEAMS_TEST_ACCOUNT`) is set in the process environment, that process-environment value governs and `.env.local.test` is ignored for that pair.
2. **Workspace Containment, Regular-File, and Untracked Enforcement for `.env.local.test`**: `.env.local.test` MUST only be read from the resolved repository root (`findRepoRoot`), MUST be verified via `os.Lstat`, symmetrical `filepath.EvalSymlinks` on both `repoRoot` and `envPath` (`filepath.IsLocal(rel)`), and open-handle `f.Stat()` + `io.LimitReader(f, (64<<10)+1)` to be a regular non-symlink file inside `repoRoot` of size `<= 64 KiB`, and MUST fail closed if listed as tracked in the Git index (`git ls-files -z -- <relPath>` with `cmd.Dir = repoRoot` and `.git/index` fallback when `git` is unavailable, or if `.git` exists and index inspection fails). Relative path values loaded from `.env.local.test` MUST NOT be `"."` or traverse above `repoRoot` (`!filepath.IsLocal(cleaned)`).
3. **Zero Secret Echo in Diagnostics and Tracked Files**: Parse or validation errors on `.env.local.test` MUST report only the file path, 1-based line number or allowlisted key name, and static structural error reason, NEVER the raw line, candidate key text, or secret value. Tracked `.env.example` MUST place its sample profile path under the gitignored `./bin/` directory (`./bin/teams-profile-synthetic-placeholder`), MUST contain only synthetic placeholder values matching `isTemplatePlaceholder` (`strings.Contains(strings.ToLower(val), "synthetic-placeholder") || strings.EqualFold(val, "synthetic-user@example.com")`), and MUST pass `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and CI's `Secret hygiene check (tracked files)` step with zero findings.
4. **Allowlisted Test Keys & Template Placeholder Pre-Filtering**: `.env.local.test` loading MUST only set the allowlisted test environment variables (`GO_TEAMS_INTEGRATION`, `TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`, `TEAMS_DISABLE_SANDBOX`) — never arbitrary process variables like `PATH`, `HOME`, or `LD_PRELOAD` — and MUST pre-filter unedited `.env.example` sentinel placeholder values (`isTemplatePlaceholder`) out of the parsed map before alias-pair resolution so `cp .env.example .env.local.test` never breaks browser auto-discovery or test skip conditions.
5. **Production Binary Purity**: Zero production `.go` files are modified; `.env.local.test` loading is compiled only during `go test`.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-028-1` | Add tracked `.env.example` template with synthetic-only placeholders and enforce synthetic-only validation in `internal/config/gitignore_test.go` | `.env.example`, `internal/config/gitignore_test.go` | Tracked test template & secret-hygiene guardrail | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-028-2` | Implement fail-closed `.env.local.test` loader, allowlisted alias/precedence resolution, and repo-root path containment in `internal/browser` and `internal/cli` test helpers | `internal/browser/integration_helper_test.go`, `internal/browser/launch_integration_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/cli/binary_automation_test.go`, `internal/cli/e2e_test.go` | Test-harness secret/env loader | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 and U3 commits |
| `PA-028-3` | Document `.env.example` and `.env.local.test` setup and precedence in `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, and `README.md` | `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `README.md` | Documentation | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U4 commit |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `.env.example` is tracked in Git and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` + `TestEnvExampleTemplateContainsOnlySyntheticPlaceholders` pass in `internal/config`.
  * [ ] `go vet ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -run '^$' ./internal/browser/...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...` pass with zero findings.
  * [ ] `TestCompiledBinaryCapturesRecapToExplicitOutput` passes against `.\bin\go-teams.exe`.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main` (and immediately during the Option C `3EB89E2D` live-tenant validation pass).
  * **Dashboard / query**: `gh run list --workflow=ci.yml` (`Secret hygiene check (tracked files)`, `test`, `test-windows`) and local `go test -race ./...`.
  * **Baseline**: 100% pass rate on `CI` (`changes`, `test`, `test-windows`) with 0 secret-hygiene findings across all tracked files.
  * **Alert threshold**: `>= 1` failure in `Secret hygiene check (tracked files)`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, or `go test -race ./...`.
* **Rollback trigger & procedure**:
  * **Trigger**: Any CI secret-hygiene failure on `.env.example`, any unexpected test failure when `.env.local.test` is absent or copied from `.env.example`, or any regression in `TestChromeMeetingEndToEnd` / `TestCompiledBinaryCapturesRecapToExplicitOutput`.
  * **Procedure**: Revert the `028-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All pre-deploy audit checks pass, PR CI checks (`changes`, `test`, `test-windows`) are green, and the monitoring plan and rollback trigger are recorded in `docs/closure/2026-11-15-028-S-review-and-closure.md`.
  * `READY_WITH_CONDITIONS`: Local gates and PR CI checks pass, pending live-tenant execution in Option C (`3EB89E2D`).
  * `BLOCKED`: Any pre-deploy audit check, secret-hygiene scan, or CI job fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona adversarial review panel (`Architecture Strategist`, `Security Lens Reviewer`, `Scope Boundary Auditor`, `Constitution Reviewer`, `Go Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`), remediate all surfaced P0/P1/P2/P3 findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P0, P1, P2, and P3 findings remediated in-place in `docs/plans/2026-11-15-test-env-local-loader-and-example-template-plan.md` and `docs/decisions/2026-11-15-test-env-local-loader-and-example-template-deliberation.md` prior to harvest).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, `strict-safety` `ProposedAction` / `ActionRisk` table, and `release-observability` monitoring/rollback/releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` | `subagent` (`multi-agent`) | 0 P0/P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1, 3 P2, 1 P3 | Remediated — `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 2 P2, 3 P3 | Remediated — `PASS` |
| `Schema-CLI-Docs Coupling Reviewer` | `subagent` (`multi-agent`) | 1 P0, 0 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `rubber-duck` (`Correctness Reviewer`) | `subagent` (`multi-agent`) | 0 P0/P1, 3 P2, 2 P3 | Remediated — `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P0 — `Schema-CLI-Docs Coupling Reviewer`, `Scope Boundary Auditor`, `Architecture Strategist`, `rubber-duck`] Remove `Test[A-Z]*` wrapper functions in `internal/browser/integration_helper_test.go`**:
   * **Finding**: Exporting `TestAccount`, `TestMeetingURL`, `TestRecapURL`, `TestProfileDir`, and `TestBrowserPath` in `internal/browser/integration_helper_test.go` collides with Go's `cmd/go` and `go vet` `Test[A-Z]*` test-entrypoint signature check (`func TestXxx(t *testing.T)`), and adds 5 redundant wrapper functions when `LoadTestEnv(t)` already synchronizes both primary and alias environment variables via `t.Setenv`.
   * **Remediation Applied**: Removed the 5 wrapper functions so `internal/browser/integration_helper_test.go` exports only `LoadTestEnv(t testing.TB)` (`<= 4` helper functions total, satisfying the 2-Hour Rule), and updated `launch_integration_test.go` and `meeting_integration_test.go` to call `browser.LoadTestEnv(t)` and read the synchronized `os.Getenv("<PRIMARY_KEY>")` values directly.
2. **[P1 — `Security Lens Reviewer`] Gitignored `./bin/teams-profile-synthetic-placeholder` path in `.env.example` and fail-closed `git ls-files -z -- <relPath>` check**:
   * **Finding**: `.gitignore` ignores `bin/`, not `.teams-profile`, so `./.teams-profile-synthetic-placeholder` in `.env.example` risked an operator creating a live Chromium `Cookies` SQLite database in an un-ignored root directory; additionally, `git ls-files --error-unmatch` conflates exit code `1` (untracked) with exit code `128` (`git` error).
   * **Remediation Applied**: Updated `.env.example` in U1 (and prose examples) to `./bin/teams-profile-synthetic-placeholder` / `./bin/teams-profile` inside the gitignored `bin/` directory, and updated U2, U3, and Protected Invariant #2 to verify untracked status via `git ls-files -z -- <relPath>` with `cmd.Dir = repoRoot` (and `.git/index` fallback when `git` is unavailable), failing closed if `.env.local.test` is tracked or if `.git` exists and index inspection fails.
3. **[P2 — `Security Lens Reviewer`, `Go Reviewer`, `rubber-duck`] Symmetrical `filepath.EvalSymlinks`, open-handle bounded read, pre-filtered `isTemplatePlaceholder`, and zero-echo error diagnostics**:
   * **Finding**: Calling `EvalSymlinks(envPath)` without `EvalSymlinks(repoRoot)` breaks on Windows 8.3 short-name `t.TempDir()` paths; `isTemplatePlaceholder` must pre-filter the parsed map before alias-pair fallback; and `parseDotEnv` / path-validation errors must never echo candidate key text or path values.
   * **Remediation Applied**: Updated U2, U3, and Protected Invariants #2–#4 to evaluate `filepath.EvalSymlinks` on both `repoRoot` and `envPath`, read `.env.local.test` via open-handle `f.Stat()` + `io.LimitReader(f, (64<<10)+1)`, pre-filter `isTemplatePlaceholder` before alias resolution, normalize `/` and `\` before `filepath.IsLocal` relative-path containment checks, and forbid echoing candidate keys or path values on error.
4. **[P2/P3 — `Schema-CLI-Docs Coupling Reviewer`, `Architecture Strategist`, `Scope Boundary Auditor`, `Go Reviewer`, `rubber-duck`] Cross-surface key inventory, `.env.example` contract test, per-`testing.TB` idempotency, and `<= 3` test scenarios per task**:
   * **Remediation Applied**: Aligned `.env.example` (U1), U2, U3, and `docs/USER_GUIDE.md` (U4) across all 11 active template keys (`TEAMS_ACCOUNT`, `TEAMS_TEST_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_TEST_RECAP_URL`, `TEAMS_PROFILE_DIR`, `TEAMS_TEST_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_TEST_BROWSER_PATH`, `GO_TEAMS_BINARY`) plus `GO_TEAMS_INTEGRATION` and `TEAMS_DISABLE_SANDBOX`; required U1's `TestEnvExampleTemplateContainsOnlySyntheticPlaceholders` to assert the exact `isTemplatePlaceholder` contract and required U2/U3 unit tests to load the real `.env.example` and verify zero variables are set; added `sync.Map` per-`testing.TB` idempotency, `t.Setenv(k, "")` test baselining, `GO_TEAMS_INTEGRATION` `"0"`/`"false"` skip handling in `requireIntegration`, `t.Helper()` in `newMeeting`, and consolidated U2 and U3 unit tests into `<= 3` table-driven test scenarios.

