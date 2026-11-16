---
title: Live integration .env.local.test path expansion, legacy profile fallback, and remote headless Recap fixes
description: Deliberation on stash entry 02B5113C for covering feature 030-F and shipment 030-S
topic: Live Integration .env.local.test Path Expansion, Legacy Profile Fallback, and Remote Headless Recap Fixes (030-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 019-DL
feature: 030-F
shipment: 030-S
source_stash:
  - 02B5113C
linked_artifacts:
  - docs/plans/2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-plan.md
  - .backlogit/queue/019-DL.md
date: 2026-11-16
status: accepted
tags:
  - browser
  - cli
  - testing
  - integration
  - recap
---

## Problem Frame

Executing the full test suite (including `go test -tags=integration ./...`) against a developer's real `.env.local.test` fails across four coupled surfaces identified in stash entry `02B5113C` (`priority: high`, `kind: bug`):

1. **`.env.local.test` path expansion (`%VAR%`, `$VAR`, `~`) and `go-teams` -> `teams-cli` profile fallback**:
   * Developers configure `TEAMS_PROFILE_DIR="%APPDATA%\go-teams\profile"` and `GO_TEAMS_BINARY=".\bin\teams-cli.exe"` (or `.\bin\go-teams.exe`) in `.env.local.test`.
   * Currently, `resolveRelative` in `internal/browser/integration_helper_test.go` and `internal/cli/binary_automation_test.go` does not expand `%VAR%`, `$VAR`, or leading `~` before evaluating `filepath.IsAbs(val)` and `filepath.IsLocal(cleaned)`. Consequently, `%APPDATA%\go-teams\profile` is treated as a relative path under the repository root (`<repoRoot>\%APPDATA%\go-teams\profile`) instead of the user's Windows roaming profile directory.
   * Furthermore, after the `teams-cli` -> `go-teams` rename (`027-S`), an operator's existing authenticated Microsoft Teams browser session often resides in `%APPDATA%\teams-cli\profile` (or `%LOCALAPPDATA%\teams-cli\profile`) while `.env.local.test` or `Options.UserDataDir` points to `...\go-teams\profile`, which does not yet contain a Teams session on disk (`!hasTeamsSessionInProfile`). Similarly, `GO_TEAMS_BINARY` in `.env.local.test` may reference `...\teams-cli.exe` when `go-teams.exe` is the built binary on disk.
2. **Remote headless `NewLiveMeeting` allocator vs. clean `--headless=new` CDP launcher and `#openTeamsClientInBrowser`**:
   * In `internal/browser/meeting.go`, `NewLiveMeeting` currently populates `cm.headedOpts` only when `!opts.Headless`. When `opts.Headless == true`, it uses `chromedp.NewExecAllocator(ctx, cleanHeadlessAllocatorOptions(opts)...)`, which starts from `chromedp.DefaultExecAllocatorOptions` (including `--password-store=basic` and `--use-mock-keychain`).
   * On Windows, `--password-store=basic` and `--use-mock-keychain` break DPAPI cookie decryption for an existing user profile, and the `chromedp.NewExecAllocator` path does not run `joinHeadedWithDeadline`'s single-tab COOP `targetId` rebinding across Microsoft login redirects (`login.microsoftonline.com` <-> `teams.microsoft.com`).
   * Additionally, `joinOnWebSel` in `internal/browser/prejoin.go` only includes `button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]` and omits `#openTeamsClientInBrowser` (used on `/dl/launcher/launcher.html`), and `launchCleanHeadedBrowser`'s `cleanup` closure in `internal/browser/recap.go` returns immediately after `terminateSurvivingCDPBrowserOnPort(port, execPath)` without polling `listCDPTargets(port)` until the CDP port is unreachable, risking `UserDataDir` lock contention across back-to-back integration tests.
3. **SharePoint Meeting Recap `VroomItemId` casing and `isDownloadBlocked` HTML error response**:
   * In `internal/browser/recap.go`, `extractRecapAudioJS` and `extractRecapTranscriptJS` check `info.VroomItemID` (uppercase `ID`), whereas live SharePoint `window.g_fileInfo` populates `info.VroomItemId` (lowercase `d`).
   * When SharePoint blocks raw media downloads (`info.isDownloadBlocked == true`), `fetch(candidateUrl, ...)` returns HTTP `200 OK` with `Content-Type: text/html` (an HTML error/redirect page) instead of audio bytes. Passing those HTML bytes into `audioContext.decodeAudioData` produces an opaque `EncodingError` instead of reporting `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`.
4. **Live-tenant `-tags=integration` test guards for external `%APPDATA%` profiles and `/l/meetingrecap` URLs**:
   * In `internal/browser/scheduled_lifecycle_integration_test.go` (`recapIntegrationURL`, `TestMeetingLiveScheduledOvertimeAndCallEnd`) and `internal/cli/scheduled_integration_test.go` (`requireCLIIntegration`), the profile containment guard requires `err == nil && filepath.IsLocal(relProfile) && relProfile != "."` and runs `git check-ignore -q -- <relProfile>`, which fails when `TEAMS_PROFILE_DIR` resolves to an external `%APPDATA%` directory outside the git repository root.
   * In `internal/browser/meeting_integration_test.go`, `browserContext(t)` and `newMeeting(t)` use `browser.AllocatorOptions` + `browser.NewMeeting` instead of `browser.NewLiveMeeting`, and `TestMeetingCaptureAudio` does not handle `RecapMeeting` (`rm.IsRecap()`) or skip cleanly when SharePoint reports `isDownloadBlocked=true`.
   * In `internal/browser/scheduled_lifecycle_integration_test.go` (`TestMeetingLiveScheduledOvertimeAndCallEnd`) and `internal/cli/scheduled_integration_test.go` (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`), when `TEAMS_MEETING_URL` in `.env.local.test` is a `/l/meetingrecap` URL rather than an active live call, the tests wait for a live call `#hangup-button` or `--mode live-with-summary` timer instead of validating Recap transcript/summary delivery.

### Scope Boundaries

* **In scope**:
  * `internal/browser/prejoin.go`, `internal/browser/meeting.go`, and `internal/browser/recap.go` (`030.001-T`).
  * `internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, and `internal/browser/scheduled_lifecycle_integration_test.go` (`030.002-T`).
  * `internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`, and `internal/browser/recap_test.go` (`030.003-T`).
  * Sequencing shipment `030-S` (`030-F`, `019-DL`) immediately after `029-S` and before `010-S` in the shipment DAG.
* **Out of scope**:
  * Active stash entries `188D4FBB`, `560B0C44`, and `9BDFB275`, which remain untouched in `active` stash state.
  * Modifying `.env.local.test` or `.env.example`.

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: Never echo raw meeting URLs, Recap URLs, passcodes, or unredacted account identifiers in error messages or test failure output.
* **`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`**: `go test` executes each package with that package's directory as `os.Getwd()`; relative paths in `.env.local.test` must be resolved against `repoRoot` after environment variable expansion.

### Codebase Coupling and Architectural Analysis

1. **Preserving the `internal/browser` Package Dependency Boundary**:
   * Per `docs/ARCHITECTURE.md`, `internal/browser` does not import `internal/config` (`internal/cli` is the sole importer of `internal/config`).
   * Implementing a self-contained `expandProfilePath(raw string) string` and `resolveExistingProfileDir(raw string) string` in `internal/browser/recap.go` (using `regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)`, `os.ExpandEnv`, `os.UserHomeDir`, and `hasTeamsSessionInProfile`) avoids any new package import or circular dependency while making profile path expansion and `go-teams` -> `teams-cli` session fallback available to both production `internal/browser` (`NewLiveMeeting`, `hasTeamsSessionInProfile`, `launchCleanHeadedBrowser`, `cleanHeadlessAllocatorOptions`) and `package browser` tests (`integration_helper_test.go`, `recap_test.go`).
2. **Routing Remote Authenticated Headless Sessions Through `cm.headedOpts`**:
   * In `internal/browser/meeting.go`, changing `if !opts.Headless {` to `if !opts.Headless || (isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != "") {` routes remote `https://` meeting/recap URLs with a non-empty `UserDataDir` through `joinHeadedWithDeadline` -> `launchCleanHeadedBrowser(*m.headedOpts, initialURL)`.
   * Because `launchCleanHeadedBrowser` calls `cleanBrowserArgsForOptions(port, absProfile, opts, targetURL)`, which appends `--headless=new` when `opts.Headless == true` without `chromedp.DefaultExecAllocatorOptions`, the spawned headless Chromium process preserves Windows DPAPI cookie decryption, supports COOP `targetId` rebinding via `findBoundCapturePage`, and clicks `#openTeamsClientInBrowser` on launcher interstitials.
   * Meanwhile, loopback `httptest.NewServer` URLs (`http://127.0.0.1:...`) and empty-`UserDataDir` unit tests (`TestNewLiveMeeting`, `TestNewLiveMeetingPropagatesDefaultAccount`) return `false` for `isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != ""`, preserving fast unit and local E2E test behavior.

## Options Evaluated

### Option A: 3-Task Width-Isolated Decomposition Across `internal/browser` Production, `internal/browser` Integration/Test Helpers, and `internal/cli` Integration/Test Helpers + Unit Tests (Recommended)

* **Description**:
  * **U1 (`030.001-T` — `internal/browser` production fixes, `Size: S`, `Complexity: low`, 3 files)**: Update `internal/browser/prejoin.go` (`joinOnWebSel`), `internal/browser/recap.go` (`expandProfilePath`, `resolveExistingProfileDir`, `VroomItemId` support in `extractRecapAudioJS` and `extractRecapTranscriptJS`, `text/html` + `isDownloadBlocked` check in `extractRecapAudioJS`, and 2s `listCDPTargets(port)` teardown poll in `launchCleanHeadedBrowser`), and `internal/browser/meeting.go` (`resolveExistingProfileDir` and remote authenticated `cm.headedOpts` routing in `NewLiveMeeting`).
  * **U2 (`030.002-T` — `internal/browser` test & integration suite fixes, `Size: S`, `Complexity: low`, depends on `030.001-T`, 3 files)**: Update `internal/browser/integration_helper_test.go` (`%VAR%`/`$VAR`/`~` expansion and `resolveExistingProfileDir` in `resolveRelative`, plus unit test in `TestLoadTestEnvPrecedenceAndPaths`), `internal/browser/meeting_integration_test.go` (`NewLiveMeeting` in `newMeeting(t)` / `browserContext(t)` and `RecapMeeting` + `isDownloadBlocked=true` handling in `TestMeetingCaptureAudio`), and `internal/browser/scheduled_lifecycle_integration_test.go` (conditional `filepath.IsLocal(relProfile)` git-ignore check for external `%APPDATA%` profiles and `RecapMeeting` handling in `TestMeetingLiveScheduledOvertimeAndCallEnd`).
  * **U3 (`030.003-T` — `internal/cli` test & integration suite fixes + recap unit tests, `Size: S`, `Complexity: low`, depends on `030.001-T` and `030.002-T`, 3 files)**: Update `internal/cli/binary_automation_test.go` (`%VAR%`/`$VAR`/`~` expansion, `go-teams` -> `teams-cli` profile fallback, `teams-cli.exe` -> `go-teams.exe` binary fallback, and unit test in `TestLoadCLITestEnvPrecedenceAndPaths`), `internal/cli/scheduled_integration_test.go` (conditional `filepath.IsLocal(relProfile)` git-ignore check and `/l/meetingrecap` `--mode past-recap` execution path in `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`), and `internal/browser/recap_test.go` (unit tests for `resolveExistingProfileDir`, `extractRecapAudioJS` `isDownloadBlocked=true` + `text/html`, and `VroomItemId`).
* **Pros**:
  * Strictly respects the 2-Hour Rule and `<= 3` files per task constraint.
  * Isolates production `internal/browser` changes (`030.001-T`) from test/integration updates (`030.002-T` and `030.003-T`).
  * Enables the entire unit, E2E, and `-tags=integration` test suite to pass against `.env.local.test`.
* **Cons**:
  * Requires sequential dependency wiring (`030.001-T` -> `030.002-T` -> `030.003-T`).
* **Effort estimate**: Low (3 atomic `Size: S` tasks).
* **Fit**: 100% alignment with stash `02B5113C` and architectural constraints.

### Option B: Work Around Profile and Recap Differences Solely via Manual Environment Variable Exports

* **Description**: Require operators to manually expand `%APPDATA%` before running `go test`, copy `%APPDATA%\teams-cli\profile` by hand, and skip `-tags=integration` tests when `TEAMS_MEETING_URL` is a `/l/meetingrecap` link.
* **Pros**: Fewer code changes.
* **Cons**: Leaves `NewLiveMeeting` broken for remote headless authenticated sessions on Windows (due to `chromedp.DefaultExecAllocatorOptions` breaking DPAPI cookies), fails to extract `VroomItemId` or report `isDownloadBlocked=true` in SharePoint Meeting Recaps, and fails the goal of running `go test -tags=integration ./...` directly against `.env.local.test`.

## Trade-off Comparison

| Criterion | Option A (3-Task Production + Integration Fix) | Option B (Manual Env Workarounds Only) |
|---|---|---|
| Fixes remote headless DPAPI cookie & COOP redirect bugs | Yes (`launchCleanHeadedBrowser` with `--headless=new`) | No |
| Supports `%VAR%`, `$VAR`, `~` and `teams-cli` fallback in `.env.local.test` | Yes | No |
| Handles SharePoint `VroomItemId` & `isDownloadBlocked=true` | Yes | No |
| Passes `go test -tags=integration ./...` against `.env.local.test` | Yes | No |
| Task granularity | 3 tasks (`<= 3` files each, `Size: S`) | N/A |

## Decision

We select **Option A**: decompose `030-F` into three atomic, width-isolated tasks (`030.001-T`, `030.002-T`, `030.003-T`, `<= 3` files each) packaged into shipment `030-S`, sequenced immediately after `029-S` and before `010-S`.

## Rejected Alternatives

* **Option B** was rejected because remote headless `NewLiveMeeting` sessions against authenticated Microsoft Teams URLs require `launchCleanHeadedBrowser` (`--headless=new` without `--password-store=basic` / `--use-mock-keychain`), `#openTeamsClientInBrowser` selector support, and SharePoint `VroomItemId` / `isDownloadBlocked` handling in production `internal/browser` code.

## Unresolved Questions

* None.

## Risks and Mitigations

* **Risk — Existing unit tests calling `NewLiveMeeting` without a real browser**: `TestNewLiveMeeting` and `TestNewLiveMeetingPropagatesDefaultAccount` in `internal/browser/meeting_test.go` pass `Options{Headless: true}` with an empty `UserDataDir`.
  * **Mitigation**: Gate the remote headless `cm.headedOpts` path on `isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != ""`, and note that `cm.headedOpts` only launches Chromium when `m.Join(ctx)` is invoked, never inside `NewLiveMeeting`.
* **Risk — Ephemeral CDP port / `UserDataDir` lock contention across sequential integration tests**: On Windows, Chromium child processes may briefly hold the `UserDataDir` lock after `taskkill`.
  * **Mitigation**: In `launchCleanHeadedBrowser`'s `cleanup` closure, after `terminateSurvivingCDPBrowserOnPort(port, execPath)`, poll `listCDPTargets(port)` every `100ms` for up to `2s` until it returns an error, ensuring the browser process has exited and released the profile lock before `cleanup()` returns.
* **Risk — Path traversal, unset `%VAR%`/`$VAR`, and repository-root `go-teams` segment replacement**:
  * **Mitigation**: Check `isBrowserPath` and `hasDrive` on the raw value before cleaning; fail closed in `resolveRelative` if a referenced `%VAR%` or `$VAR` is unset/empty; replace only the rightmost `go-teams` path segment (never rewriting `repoRoot`); and verify `realProfile` is not an ancestor of `realRoot` when allowing external `%APPDATA%` profile directories.
* **Risk — SharePoint `isDownloadBlocked=true` masked by `resp.url` identity check or trailing `blob:` candidate**:
  * **Mitigation**: In `extractRecapAudioJS`, inspect `Content-Type: text/html` and `!resp.ok && info.isDownloadBlocked` before the `resp.url` identity check, record `blockedErr`, and throw `blockedErr || lastFetchErr` at the end of the candidate loop (and preserve `isDownloadBlocked=true` errors across targets in `captureRecapAudioFromTargets`).

