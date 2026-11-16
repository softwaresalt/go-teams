---
title: Live integration .env.local.test path expansion, legacy profile fallback, and remote headless Recap fixes implementation plan
description: Implementation plan for 030-F / 030-S covering stash entry 02B5113C
source: docs/decisions/2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-deliberation.md
deliberation_id: 019-DL
feature: 030-F
shipment: 030-S
source_stash:
  - 02B5113C
date: 2026-11-16
status: approved
tags:
  - browser
  - cli
  - testing
  - integration
  - recap
---

## Problem Frame

Following the accepted deliberation in [2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-deliberation.md](../decisions/2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-deliberation.md) (`019-DL`), stash entry **`02B5113C`** (`priority: high`, `kind: bug`) is planned under covering feature **`030-F`** and shipment **`030-S`**, sequenced immediately after `029-S` and before `010-S` in the shipment dependency DAG **`029-S -> 030-S -> 010-S`**.

Running the full test suite (including `go test -tags=integration ./...`) against `.env.local.test` currently fails across four coupled surfaces:

1. **`.env.local.test` path expansion (`%VAR%`, `$VAR`, `~`) and `go-teams` -> `teams-cli` profile/binary fallback**:
   * `resolveRelative` in `internal/browser/integration_helper_test.go` and `internal/cli/binary_automation_test.go` does not expand `%VAR%`, `$VAR`, or leading `~` before evaluating `filepath.IsAbs(val)` and `filepath.IsLocal(cleaned)`.
   * Neither `internal/browser` (`recap.go`, `meeting.go`, `integration_helper_test.go`) nor `internal/cli` (`binary_automation_test.go`) falls back from `go-teams\profile` to an existing `teams-cli\profile` when the `go-teams` profile directory does not yet contain a Teams session on disk (`!hasTeamsSessionInProfile`), or from `teams-cli.exe` to `go-teams.exe` for `GO_TEAMS_BINARY`.
2. **Remote headless `NewLiveMeeting` allocator vs. clean `--headless=new` CDP launcher and `#openTeamsClientInBrowser`**:
   * `NewLiveMeeting` (`internal/browser/meeting.go`) currently sets `cm.headedOpts` only when `!opts.Headless`. When `opts.Headless == true` on a remote authenticated Teams URL with a `UserDataDir`, `chromedp.NewExecAllocator` applies `chromedp.DefaultExecAllocatorOptions` (`--password-store=basic`, `--use-mock-keychain`), breaking Windows DPAPI cookie decryption and skipping `joinHeadedWithDeadline`'s COOP `targetId` rebinding.
   * `joinOnWebSel` in `internal/browser/prejoin.go` omits `#openTeamsClientInBrowser`.
   * `launchCleanHeadedBrowser`'s `cleanup` closure in `internal/browser/recap.go` does not wait for `listCDPTargets(port)` to return an error after `terminateSurvivingCDPBrowserOnPort(port, execPath)`, allowing subsequent tests to hit a locked `UserDataDir`.
3. **SharePoint Meeting Recap `VroomItemId` casing and `isDownloadBlocked` HTML error response**:
   * `extractRecapAudioJS` and `extractRecapTranscriptJS` in `internal/browser/recap.go` check `info.VroomItemID` (uppercase `ID`), whereas live SharePoint `window.g_fileInfo` populates `info.VroomItemId` (lowercase `d`).
   * In `extractRecapAudioJS`, when `info.isDownloadBlocked` is `true`, SharePoint returns HTTP `200 OK` with `Content-Type: text/html`; passing those HTML bytes to `decodeAudioData` fails with an opaque `EncodingError` instead of reporting `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`.
4. **Live-tenant `-tags=integration` test guards for external `%APPDATA%` profiles and `/l/meetingrecap` URLs**:
   * `recapIntegrationURL` and `TestMeetingLiveScheduledOvertimeAndCallEnd` in `internal/browser/scheduled_lifecycle_integration_test.go` and `requireCLIIntegration` in `internal/cli/scheduled_integration_test.go` fail when `TEAMS_PROFILE_DIR` is an external `%APPDATA%` directory outside the git repository root.
   * `newMeeting(t)` / `browserContext(t)` in `internal/browser/meeting_integration_test.go` use `browser.AllocatorOptions` + `browser.NewMeeting` instead of `browser.NewLiveMeeting`, and `TestMeetingCaptureAudio` does not handle `RecapMeeting` (`rm.IsRecap()`) or skip cleanly on `isDownloadBlocked=true`.
   * `TestMeetingLiveScheduledOvertimeAndCallEnd` (`internal/browser/scheduled_lifecycle_integration_test.go`) and `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` (`internal/cli/scheduled_integration_test.go`) do not detect when `TEAMS_MEETING_URL` in `.env.local.test` is a `/l/meetingrecap` URL.

### Explicit Out-of-Scope Boundaries

* Active stash entries `188D4FBB`, `560B0C44`, and `9BDFB275` remain untouched in `active` stash state.
* `.env.local.test` and `.env.example` are not modified.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Update `joinOnWebSel` in `internal/browser/prejoin.go` to include `#openTeamsClientInBrowser`; add `expandProfilePath` and `resolveExistingProfileDir` (`%VAR%`, `$VAR`, `~` expansion + `go-teams` -> `teams-cli` profile fallback), `VroomItemId` support in `extractRecapAudioJS` and `extractRecapTranscriptJS`, `text/html` / `isDownloadBlocked=true` detection in `extractRecapAudioJS`, and 2s `listCDPTargets(port)` teardown polling in `internal/browser/recap.go`; and resolve `opts.UserDataDir` and route remote authenticated headless sessions through `cm.headedOpts` in `NewLiveMeeting` (`internal/browser/meeting.go`) | `02B5113C` | U1 (`030.001-T`) | `go test -race ./internal/browser/...`, `go vet ./...`, `golangci-lint run` |
| R2: Expand `%VAR%`, `$VAR`, and `~` and apply `resolveExistingProfileDir` for `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR` in `internal/browser/integration_helper_test.go` (with unit test in `TestLoadTestEnvPrecedenceAndPaths`); update `newMeeting(t)` / `browserContext(t)` to use `browser.NewLiveMeeting` and handle `RecapMeeting` + `isDownloadBlocked=true` in `TestMeetingCaptureAudio` (`internal/browser/meeting_integration_test.go`); and allow external `%APPDATA%` profile directories (`filepath.IsLocal(relProfile)` conditional git-ignore check) and `RecapMeeting` caption verification in `TestMeetingLiveScheduledOvertimeAndCallEnd` (`internal/browser/scheduled_lifecycle_integration_test.go`) | `02B5113C` | U2 (`030.002-T`) | `go test -race ./internal/browser/...`, `go vet -tags=integration ./internal/browser/...`, `go test -tags=integration -run '^$' ./internal/browser/...` |
| R3: Expand `%VAR%`, `$VAR`, and `~`, apply `go-teams` -> `teams-cli` profile fallback and `teams-cli.exe` -> `go-teams.exe` binary fallback in `internal/cli/binary_automation_test.go` (with unit test in `TestLoadCLITestEnvPrecedenceAndPaths`); allow external `%APPDATA%` profile directories and handle `/l/meetingrecap` URLs via `--mode past-recap` in `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` (`internal/cli/scheduled_integration_test.go`); and add unit tests for `resolveExistingProfileDir`, `extractRecapAudioJS` `isDownloadBlocked=true` + `text/html`, and `VroomItemId` in `internal/browser/recap_test.go` | `02B5113C` | U3 (`030.003-T`) | `go test -race ./...`, `go vet -tags=integration ./...`, `go test -tags=integration ./...` |

## Implementation Units

### U1 (`030.001-T`) — Fix browser profile expansion/fallback, remote headless CDP launcher routing, `#openTeamsClientInBrowser`, and SharePoint `VroomItemId`/`isDownloadBlocked` handling (`internal/browser`)

* **Domain:** code (`internal/browser/prejoin.go`, `internal/browser/meeting.go`, `internal/browser/recap.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Files (3 files, 4 functions):** `internal/browser/prejoin.go`, `internal/browser/meeting.go`, `internal/browser/recap.go`.
* **What:**
  1. **In `internal/browser/prejoin.go`**:
     * Update `joinOnWebSel` from ``button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]`` to ``#openTeamsClientInBrowser, button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]``.
  2. **In `internal/browser/recap.go`**:
     * Add `var winProfileEnvVarRe = regexp.MustCompile(`%([A-Za-z_][A-Za-z0-9_]*)%`)` and helper `resolveExistingProfileDir(raw string) string`:
       * Trims whitespace; if empty, returns `""`.
       * Expands `%VAR%` via `winProfileEnvVarRe.ReplaceAllStringFunc` (resolving `%USER%` / `%USERPROFILE%` to `home` when `home, err := os.UserHomeDir(); err == nil && home != ""`, and looking up `os.LookupEnv(sub[1])`), then `os.ExpandEnv`, then leading `~` (`~`, `~/`, `~\`) via `os.UserHomeDir()`, normalizing backslashes on non-Windows platforms and cleaning via `filepath.Clean(expanded)`.
       * If `!hasTeamsSessionInProfile(expanded)`: replaces the rightmost `go-teams` path segment in `expanded` with `teams-cli` (`legacy`); if `legacy != expanded && hasTeamsSessionInProfile(legacy)`, returns `legacy`.
       * Otherwise returns `expanded`.
     * In `hasTeamsSessionInProfile`: expand `%VAR%`, `$VAR`, and `~` on `trimmed` before `filepath.Abs` (without calling `resolveExistingProfileDir`, avoiding recursion), and in `launchCleanHeadedBrowser`, resolve `opts.UserDataDir = resolveExistingProfileDir(opts.UserDataDir)` before `filepath.Abs` (and in `captureRecapAudioFromTargets`, if `lastErr` already contains `"isDownloadBlocked=true"`, preserve it rather than overwriting `lastErr` with a later target's generic error).
     * In `extractRecapAudioJS`:
       * Support both `info.VroomItemID` and `info.VroomItemId`:
         - Update `itemId` initialization to `const itemId = String(info.itemId || info.driveItemId || info.VroomItemID || info.VroomItemId || new URL(pageUrl).searchParams.get('driveItemId') || '');`.
         - Add `checkField('DriveItemID', info.VroomItemId);` right after `checkField('DriveItemID', info.VroomItemID);`.
       * Track `let blockedErr = null;` alongside `let lastFetchErr = null;`. Immediately after `fetch(candidateUrl, ...)`:
         - If `!resp || !resp.ok`: when `info.isDownloadBlocked` is truthy, set `blockedErr = new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)'); lastFetchErr = blockedErr; continue;`.
         - Inspect `const ct = String((resp.headers && resp.headers.get && resp.headers.get('content-type')) || '').toLowerCase();` before the `resp.url` identity check (`if (enforceIdentity && !identityMatched)`). If `ct.includes('text/html')` (which SharePoint returns when `info.isDownloadBlocked` is `true`), set `const err = new Error(info.isDownloadBlocked ? 'recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)' : 'recap audio fetch returned unexpected text/html content'); if (info.isDownloadBlocked) blockedErr = err; lastFetchErr = err; continue;` instead of passing HTML bytes to `decodeAudioData`.
         - At the end of the candidate loop, `throw (blockedErr || lastFetchErr || new Error('recap audio fetch failed'));` so a trailing `blob:` candidate on `media.currentSrc` throwing `TypeError: Failed to fetch` never overwrites `blockedErr`.
     * In `extractRecapTranscriptJS`:
       * Support both `fi.VroomItemID` and `fi.VroomItemId`: update `const itemId = String(fi.id || fi.VroomItemID || fi.VroomItemId || fi.docId || '').trim();`.
     * In `launchCleanHeadedBrowser`'s `cleanup` closure:
       * After the `if runtime.GOOS == "windows" { terminateSurvivingCDPBrowserOnPort(port, execPath) }` block, poll up to `2s` (every `100ms`) until `listCDPTargets(port)` returns a non-nil error so the browser process has completely exited and released its `UserDataDir` lock before the next test starts:
         ```go
         deadline := time.Now().Add(2 * time.Second)
         for time.Now().Before(deadline) {
         	if _, probeErr := listCDPTargets(port); probeErr != nil {
         		break
         	}
         	time.Sleep(100 * time.Millisecond)
         }
         ```
  3. **In `internal/browser/meeting.go` (`NewLiveMeeting`)**:
     * Resolve `opts.UserDataDir = resolveExistingProfileDir(opts.UserDataDir)` before setting `cm.userDataDir = strings.TrimSpace(opts.UserDataDir)`.
     * Change `if !opts.Headless {` to `if !opts.Headless || (isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != "") {` so remote authenticated sessions with a user profile directory route through `cm.headedOpts` (`joinHeadedWithDeadline` -> `launchCleanHeadedBrowser` with `--headless=new` via `cleanBrowserArgsForOptions`), and update the doc comment on `NewLiveMeeting` to note that `cm.headedOpts` drives the clean direct-exec CDP launcher for both headed and remote authenticated `--headless=new` sessions.
  4. **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, and `golangci-lint run`.

### U2 (`030.002-T`) — Update `internal/browser` `.env.local.test` loader and `-tags=integration` tests for `%VAR%` expansion, `NewLiveMeeting`, `isDownloadBlocked`, and `/l/meetingrecap` URLs (`internal/browser`)

* **Domain:** test (`internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `030.001-T` (`blocks`).
* **Files (3 files, 4 functions, 3 test scenarios):** `internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`.
* **What:**
  1. **In `internal/browser/integration_helper_test.go`**:
     * In `loadTestEnvFromFile`'s `resolveRelative`:
       * Preserve bare browser executable names (`if isBrowserPath && !strings.HasPrefix(val, ".") && !strings.HasPrefix(val, "~") && !strings.ContainsAny(val, `/\%$`) { return val }`) and check drive-relative paths (`hasDrive && !filepath.IsAbs(val)`) on the raw value before path cleaning.
       * Expand `%VAR%` (`%([A-Za-z_][A-Za-z0-9_]*)%`), `$VAR` (`os.ExpandEnv`), and leading `~` (`if home, err := os.UserHomeDir(); err == nil && home != ""`), failing closed (`t.Fatalf` naming only `keyName`, never echoing `val`) if a referenced `%VAR%` or `$VAR` is unset/empty, and normalizing backslashes on non-Windows platforms when an expanded variable produces an absolute path.
       * For `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR` (including when set in the process environment), apply `resolveExistingProfileDir(resolved)` so `%APPDATA%\go-teams\profile` expands and falls back to `%APPDATA%\teams-cli\profile` when `%APPDATA%\go-teams\profile` does not yet have a Teams session on disk (without rewriting `repoRoot` on repo-relative paths). For `GO_TEAMS_BINARY`, if the resolved path ends with `teams-cli.exe` and does not exist while `go-teams.exe` exists in the same directory, fall back to `go-teams.exe`.
     * In `TestLoadTestEnvPrecedenceAndPaths`:
       * Add unit test assertions verifying `%VAR%` expansion and `go-teams` -> `teams-cli` fallback on `TEAMS_PROFILE_DIR`.
  2. **In `internal/browser/meeting_integration_test.go`**:
     * Update imports (removing unused `"github.com/chromedp/chromedp"` and adding `"strings"`), and update `newMeeting(t)` / `browserContext(t)` to use `browser.NewLiveMeeting(context.Background(), browser.Options{Headless: true, UserDataDir: profile, ExecPath: os.Getenv("TEAMS_BROWSER_PATH"), DefaultAccount: os.Getenv("TEAMS_ACCOUNT")}, browser.MeetingOptions{MeetingURL: url, BotName: "Transcription Agent", DefaultAccount: os.Getenv("TEAMS_ACCOUNT"), LobbyTimeout: 2 * time.Minute})` with `t.Cleanup(cleanup)`.
     * In `TestMeetingEnableCaptions`: inside `case err := <-errCh:`, check `select { case <-got: default: t.Fatal("EnableCaptions returned before delivering a caption") }` after `require.NoError(t, err)` so synchronous Recap caption delivery does not race with `errCh`.
     * In `TestMeetingCaptureAudio`:
       * After `require.NoError(t, m.Join(ctx))`, check `if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap()`:
         - When `rm.IsRecap()` is `true`: wait for `err := <-errCh` (or `<-ctx.Done()`); if `err != nil && strings.Contains(err.Error(), "isDownloadBlocked=true")`, call `t.Skip("recap recording has SharePoint download blocked (isDownloadBlocked=true); skipping raw audio download assertion")`; otherwise require `require.NoError(t, err)` and `require.NotZero(t, buf.Len(), "expected captured MP3 bytes")`, and return.
  3. **In `internal/browser/scheduled_lifecycle_integration_test.go`**:
     * In `recapIntegrationURL` and `TestMeetingLiveScheduledOvertimeAndCallEnd`:
       * Verify `realProfile` is not an ancestor of `realRoot` (`relRoot, relErr := filepath.Rel(realProfile, realRoot); require.False(t, relErr == nil && filepath.IsLocal(relRoot), "TEAMS_PROFILE_DIR must not contain repository root")`), and only enforce `relProfile != "."` and `git check-ignore -q -- <relProfile>` when `err == nil && filepath.IsLocal(relProfile)` (allowing external `%APPDATA%` profile directories outside `realRoot`).
     * In `TestMeetingLiveScheduledOvertimeAndCallEnd`:
       * After `require.NoError(t, m.Join(ctx))` and `streamCtx, cancelStream := context.WithCancel(ctx); defer cancelStream()`, check `if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap()`:
         - When `rm.IsRecap()` is `true` (because `TEAMS_MEETING_URL` in `.env.local.test` is a `/l/meetingrecap` URL), run `err := m.EnableCaptions(streamCtx, func(_, _ string) { recordActivity() })`, assert `require.NoError(t, err)` and `require.NotZero(t, lastActivityUnixNano.Load(), "expected delivered recap transcript utterances")`, and `return` cleanly without waiting for a live-call `#hangup-button`.
  4. **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/browser/...`.

### U3 (`030.003-T`) — Update `internal/cli` `.env.local.test` loader, CLI `-tags=integration` tests, and `internal/browser/recap_test.go` unit tests (`internal/cli`, `internal/browser`)

* **Domain:** test (`internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`, `internal/browser/recap_test.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `030.001-T` (`blocks`), `030.002-T` (`blocks`).
* **Files (3 files, 4 functions, 3 test scenarios):** `internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`, `internal/browser/recap_test.go`.
* **What:**
  1. **In `internal/cli/binary_automation_test.go`**:
     * In `loadCLITestEnvFromFile`'s `resolveRelative`:
       * Preserve bare browser executable names (`if isBrowserPath && !strings.HasPrefix(val, ".") && !strings.HasPrefix(val, "~") && !strings.ContainsAny(val, `/\%$`) { return val }`) and check drive-relative paths (`hasDrive && !filepath.IsAbs(val)`) on the raw value before path cleaning.
       * Expand `%VAR%` (`%([A-Za-z_][A-Za-z0-9_]*)%`), `$VAR` (`os.ExpandEnv`), and leading `~` (`if home, err := os.UserHomeDir(); err == nil && home != ""`), failing closed (`t.Fatalf` naming only `keyName`, never echoing `val`) if a referenced `%VAR%` or `$VAR` is unset/empty, and normalizing backslashes on non-Windows platforms when an expanded variable produces an absolute path.
       * For `TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR` (including when set in the process environment), apply the rightmost-segment `go-teams` -> `teams-cli` profile fallback when the `go-teams` profile directory does not exist or lacks `Default/Local Storage/leveldb` and the corresponding `teams-cli` profile directory exists on disk.
       * For `GO_TEAMS_BINARY`, if the resolved path ends with `teams-cli.exe` (or `teams-cli`) and does not exist on disk (`os.Stat` error) while `go-teams.exe` (or `go-teams`) exists in the same directory, fall back to `go-teams.exe` (or `go-teams`).
     * In `TestLoadCLITestEnvPrecedenceAndPaths`:
       * Add a unit test assertion covering `%VAR%` expansion on `TEAMS_PROFILE_DIR` (with `go-teams` -> `teams-cli` fallback) and `teams-cli.exe` -> `go-teams.exe` fallback on `GO_TEAMS_BINARY`.
  2. **In `internal/cli/scheduled_integration_test.go`**:
     * In `requireCLIIntegration`:
       * Verify `realProfile` is not an ancestor of `realRoot` (`relRoot, relErr := filepath.Rel(realProfile, realRoot); require.False(t, relErr == nil && filepath.IsLocal(relRoot), "TEAMS_PROFILE_DIR must not contain repository root")`), and only enforce `relProfile != "."` and `git check-ignore -q -- <relProfile>` when `err == nil && filepath.IsLocal(relProfile)` (allowing external `%APPDATA%` profile directories).
     * In `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`:
       * If `strings.Contains(strings.ToLower(rawURL), "meetingrecap")`, execute `NewRootCommand()` with `join`, `rawURL`, `--config`, `cfgPath`, `--mode`, `config.CaptureModePastRecap`, `--lobby-timeout`, `"2m"`, `--category`, `"LIVE"`, `--output-dir`, `outDir`, `--profile-dir`, `profile`, `--headless` (plus `--browser-path` and `--account` when set in env), require `ExecuteContext` to succeed within a `3m` timeout context (`require.NoError(t, ctx.Err())`), and assert via boolean `require.True(t, strings.Contains(...), "static message")` (never `require.Contains`, avoiding raw transcript PII dumps on failure) that the written `.md` file in `outDir` is non-empty and contains `## Transcript`, `"\n> "`, and `# AI Summary`, then `return`.
  3. **In `internal/browser/recap_test.go`**:
     * Add unit tests covering:
       * `resolveExistingProfileDir` (`%VAR%` expansion + rightmost-segment `go-teams` -> `teams-cli` fallback when `teams-cli\profile` contains a Teams session in `Default/Local Storage/leveldb` and `go-teams\profile` does not).
       * `extractRecapAudioJS` and `extractRecapTranscriptJS` executed via `node -e` (matching `TestRecapAudioJSValidatesVroomItemID` in `recap_retry_test.go`) verifying `isDownloadBlocked=true` + `Content-Type: text/html` error reporting (even when a trailing `blob:` candidate is present on `media.currentSrc`, with 0 calls to `decodeAudioData`) and `VroomItemId` lowercase-`d` recognition in both scripts.
  4. **Verify:** Run `go test -race ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -tags=integration ./...`.

## Dependency Graph

```text
030.001-T (U1: internal/browser/prejoin.go + meeting.go + recap.go)
    |
    +---> 030.002-T (U2: internal/browser/integration_helper_test.go + meeting_integration_test.go + scheduled_lifecycle_integration_test.go)
              |
              +---> 030.003-T (U3: internal/cli/binary_automation_test.go + scheduled_integration_test.go + internal/browser/recap_test.go)
```

* **Shipment sequencing DAG**:
  * Remove `010-S -> 029-S` direct edge (`backlogit dep remove 010-S 029-S`).
  * Add `030-S -> 029-S` (`backlogit dep add 030-S 029-S --type blocks` — `029-S` is `shipped`, placing `030-S` in `ready_set`).
  * Add `010-S -> 030-S` (`backlogit dep add 010-S 030-S --type blocks` — sequencing `010-S` after `030-S`).

## Decisions and Rationale

1. **Self-contained `expandProfilePath` and `resolveExistingProfileDir` in `internal/browser/recap.go`**: Because `internal/browser` does not import `internal/config` (preserving the `docs/ARCHITECTURE.md` dependency rule that `internal/cli` is the sole importer of `internal/config`), implementing `%VAR%`, `$VAR`, and `~` expansion plus `go-teams` -> `teams-cli` session fallback directly in `internal/browser/recap.go` keeps the package dependency graph clean while serving both `NewLiveMeeting` / `launchCleanHeadedBrowser` and `internal/browser` tests.
2. **Routing remote authenticated headless sessions through `cm.headedOpts` (`launchCleanHeadedBrowser` with `--headless=new`)**: `chromedp.DefaultExecAllocatorOptions` injects `--password-store=basic` and `--use-mock-keychain`, which prevent Chromium on Windows from decrypting DPAPI-protected cookies in an existing user profile, and chromedp's static target binding fails when Microsoft login COOP headers swap `targetId`. Routing `NewLiveMeeting` through `cm.headedOpts` whenever `!opts.Headless || (isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != "")` uses `launchCleanHeadedBrowser` (`--headless=new` via `cleanBrowserArgsForOptions`) while leaving local `httptest.NewServer` URLs and empty-profile unit tests on the fast allocator path.
3. **Graceful handling of SharePoint `isDownloadBlocked=true` and `/l/meetingrecap` URLs in `-tags=integration` tests**: Checking `Content-Type: text/html` and `info.isDownloadBlocked` in `extractRecapAudioJS` surfaces a deterministic error message (`recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`) so `TestMeetingCaptureAudio` can call `t.Skip(...)` when tenant policy blocks raw `.mp4` downloads, while transcript and AI Summary integration assertions continue to run and pass.

## Risks and Caveats

* **3-Threat Security Model**:
  1. *Most likely threat — Unset or empty `%VAR%`/`$VAR` in `.env.local.test` collapsing to a root-anchored path or creating literal `%VAR%` directories inside `repoRoot`*: Mitigated by checking `isBrowserPath` and `hasDrive` on the raw value before cleaning, and failing closed in `resolveRelative` whenever a referenced `%VAR%` or `$VAR` is unset or expands to empty/whitespace.
  2. *Highest impact threat — Leftmost `strings.Replace(..., "go-teams", "teams-cli", 1)` rewriting the `go-teams` repository root segment into a sibling `..\teams-cli` path*: Mitigated by replacing only the rightmost `go-teams` path segment (never rewriting `repoRoot`) and verifying in `recapIntegrationURL`, `TestMeetingLiveScheduledOvertimeAndCallEnd`, and `requireCLIIntegration` that `realProfile` is not an ancestor of `realRoot`.
  3. *Most subtle threat — SharePoint `isDownloadBlocked=true` masked by `resp.url` identity mismatch or trailing `blob:` candidate, or `require.Contains` dumping live meeting transcripts on failure*: Mitigated by checking `Content-Type: text/html` and `!resp.ok && info.isDownloadBlocked` before the `resp.url` identity gate, tracking `blockedErr` separately from `lastFetchErr` across the candidate loop, and using boolean `require.True(t, strings.Contains(...), "static message")` on live `.md` files.
* **Teardown race on `UserDataDir` between sequential tests**: Waiting up to `2s` (polling `listCDPTargets(port)` every `100ms` until error) at the end of `launchCleanHeadedBrowser`'s `cleanup` closure guarantees that the browser process has released the profile lock before the next test launches Chromium against the same `TEAMS_PROFILE_DIR`.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, no `unsafe`, zero lint findings, all errors (including `os.UserHomeDir()`) checked |
| II. Test-First Development | Compliant — unit tests in `integration_helper_test.go`, `binary_automation_test.go`, and `recap_test.go` plus `-tags=integration` suites |
| III. Workspace Isolation | Compliant — relative paths in `.env.local.test` remain strictly contained within `repoRoot` (`filepath.IsLocal`), while repo-local profile directories remain verified via `git check-ignore -q` |
| IV. CLI Workspace Containment | Compliant (Governance note: external `%APPDATA%` profile directories explicitly provided by the operator in `.env.local.test` are existing user-profile directories matched via `DefaultConfig()` in `internal/config/config.go`; integration test outputs write solely to `t.TempDir()`, and `realProfile` is verified not to be an ancestor of `realRoot`) |
| V. Structured Observability | Compliant — `launchCleanHeadedBrowser` logs PID, CDP port, target ID, and headless state via `log/slog` |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); 3 atomic tasks (`<= 3` files and `<= 4` functions each) |
| VII. Destructive Command Approval | Compliant — no destructive commands used |
| VIII. Explicit Safety Modes | Compliant — `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory (`PA-030-1`..`PA-030-4`) recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded test outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No exported Go signatures or CLI flags are changed (`joinOnWebSel` is an unexported selector constant; `resolveExistingProfileDir` and `expandProfilePath` are unexported package helpers).
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Touches browser profile path expansion, authenticated session detection (`hasTeamsSessionInProfile`), remote headless CDP launching, and SharePoint `isDownloadBlocked` error handling.
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Non-destructive read-only fallback from `go-teams` to `teams-cli` profile directory when `go-teams` has no session on disk.
* **External integration, operator checkpoint, or external dependency**: **Present** — Exercises live Microsoft Teams and SharePoint Meeting Recap integration tests (`-tags=integration`) against `.env.local.test`.
* **High runtime, rollout, or rollback risk**: **Present** — Modifies `NewLiveMeeting` in `internal/browser/meeting.go` and `launchCleanHeadedBrowser` in `internal/browser/recap.go`.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * Production browser pre-join selector, profile resolution, Recap JS extraction, and remote headless `NewLiveMeeting` routing (`internal/browser/prejoin.go`, `internal/browser/meeting.go`, `internal/browser/recap.go`)
  * Browser test env loader, integration tests, and unit tests (`internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`, `internal/browser/recap_test.go`)
  * CLI test env loader and integration tests (`internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`)
* **Runtime verification**:
  * Run `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...`.
  * Run `go test -tags=integration ./...` against `.env.local.test` to verify all unit, E2E, and live-tenant integration tests pass.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, and releasability evidence contract recorded in `docs/closure/` during Ship operational closure.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan modifies `NewLiveMeeting` and `launchCleanHeadedBrowser` in `internal/browser`, adds `%VAR%`/`$VAR`/`~` path expansion and legacy `go-teams` -> `teams-cli` profile fallback in both production browser profile resolution and `.env.local.test` test loaders, updates SharePoint Recap audio/transcript JS extraction (`VroomItemId` and `isDownloadBlocked`), and updates `-tags=integration` test suites.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/browser-verification.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **Architecture Import Boundary (`docs/ARCHITECTURE.md`)**: `internal/browser` MUST NOT import `internal/config`. Profile path expansion (`expandProfilePath`) and `go-teams` -> `teams-cli` fallback (`resolveExistingProfileDir`) in `internal/browser/recap.go` MUST use standard library packages only (`os`, `path/filepath`, `regexp`, `strings`).
2. **Non-Regression of Local `httptest.NewServer` E2E and Unit Tests**: In `NewLiveMeeting` (`internal/browser/meeting.go`), routing `opts.Headless == true` through `cm.headedOpts` MUST be gated on `isRemoteAuthBackedMeetingURL(mopts.MeetingURL) && strings.TrimSpace(opts.UserDataDir) != ""`. Local `http://127.0.0.1:...` `httptest.NewServer` URLs (`TestChromeMeetingEndToEnd`, `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`, `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) and empty-`UserDataDir` unit tests (`TestNewLiveMeeting`, `TestNewLiveMeetingPropagatesDefaultAccount`) MUST remain on the fast `cleanHeadlessAllocatorOptions` path.
3. **Relative Path Traversal Rejection in `.env.local.test`**: In both `internal/browser/integration_helper_test.go` and `internal/cli/binary_automation_test.go`, after expanding `%VAR%`, `$VAR`, and leading `~`, any non-absolute path (other than a bare browser executable name when `isBrowserPath == true`) MUST still pass `cleaned != "." && filepath.IsLocal(cleaned)` before `filepath.Join(repoRoot, cleaned)` so `../` traversal and drive-relative paths (`C:..\...`) continue to fail closed without echoing the secret path value.
4. **Repository-Local Profile Git-Ignore Enforcement Preserved**: In `scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go`, whenever `realProfile` resolves inside `realRoot` (`err == nil && filepath.IsLocal(relProfile)`), `relProfile != "."` and `git check-ignore -q -- <relProfile>` MUST still be enforced; only external profile paths outside `realRoot` (such as `%APPDATA%\go-teams\profile` or `%APPDATA%\teams-cli\profile`) skip `git check-ignore` because they are outside the git repository tree.
5. **Clean CDP Port and Profile Lock Release on Teardown**: In `launchCleanHeadedBrowser`'s `cleanup` closure (`internal/browser/recap.go`), after `terminateSurvivingCDPBrowserOnPort(port, execPath)`, `cleanup` MUST poll `listCDPTargets(port)` every `100ms` for up to `2s` until it returns an error so back-to-back tests sharing `TEAMS_PROFILE_DIR` never encounter a locked profile or surviving CDP listener.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-030-1` | Update `joinOnWebSel`, add `expandProfilePath` + `resolveExistingProfileDir`, support `VroomItemId` and `isDownloadBlocked` `text/html` detection in Recap JS, poll CDP port teardown up to `2s`, and route remote authenticated headless `NewLiveMeeting` sessions through `cm.headedOpts` | `internal/browser/prejoin.go`, `internal/browser/meeting.go`, `internal/browser/recap.go` | Production browser session, launcher, and Recap extraction fix | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-030-2` | Expand `%VAR%`/`$VAR`/`~` and apply `resolveExistingProfileDir` in `internal/browser/integration_helper_test.go`, update `meeting_integration_test.go` to use `NewLiveMeeting` and handle `RecapMeeting` + `isDownloadBlocked=true`, and update `scheduled_lifecycle_integration_test.go` for external `%APPDATA%` profiles and `RecapMeeting` | `internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go` | Browser test loader and `-tags=integration` test updates | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-030-3` | Expand `%VAR%`/`$VAR`/`~` and apply `go-teams` -> `teams-cli` profile/binary fallbacks in `internal/cli/binary_automation_test.go`, update `scheduled_integration_test.go` for external `%APPDATA%` profiles and `/l/meetingrecap` URLs, and add unit tests in `internal/browser/recap_test.go` | `internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`, `internal/browser/recap_test.go` | CLI test loader, `-tags=integration` test updates, and browser unit tests | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 commit |
| `PA-030-4` | Execute `-tags=integration` live-tenant browser verification (`go test -tags=integration ./...`) against `.env.local.test` using the clean `--headless=new` CDP launcher and external `%APPDATA%` profile | `internal/browser` and `internal/cli` `-tags=integration` suites | External call (live-tenant headless browser verification against Microsoft Teams / SharePoint Recap) | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | Cancel test context; temporary outputs cleaned via `t.TempDir()` |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages (`internal/audio`, `internal/browser`, `internal/capture`, `internal/cli`, `internal/config`, `internal/scheduler`, `internal/teams`, `internal/transcript`).
  * [ ] `go test -tags=integration ./...` passes against `.env.local.test`.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `unit_and_e2e_test_pass_rate` (`go test -race ./...` == 100%), (2) `integration_test_pass_rate` (`go test -tags=integration ./...` == 100% with zero profile lock collisions), (3) `recap_audio_blocked_detection` (deterministic `isDownloadBlocked=true` error message when SharePoint returns `text/html` on blocked media download), and (4) `tracked_secret_leak_count` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == 0`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main`.
  * **Dashboard / query**: `gh run list --workflow=ci.yml` (`test`, `test-windows`, `Secret hygiene check (tracked files)`) and local `go test -tags=integration -v ./...`.
  * **Baseline**: 100% pass rate on `go test -race ./...` and `go test -tags=integration ./...`, 0 linter issues, 0 secret-hygiene findings.
  * **Alert threshold**: `>= 1` test failure, profile lock timeout, or CI check failure.
* **Rollback trigger & procedure**:
  * **Trigger**: Any regression in `go test -race ./...`, `go test -tags=integration ./...`, or GitHub Actions CI (`test`, `test-windows`).
  * **Procedure**: Revert the `030-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, unit tests, real-Chromium E2E tests, `-tags=integration` tests against `.env.local.test`, and PR CI checks (`changes`, `test`, `test-windows`) pass and are recorded in `docs/closure/`.
  * `READY_WITH_CONDITIONS`: All local quality gates, unit/E2E tests, and PR CI checks pass, with any tenant-blocked raw audio download (`isDownloadBlocked=true`) documented and verified via `t.Skip` in `TestMeetingCaptureAudio`.
  * `BLOCKED`: Any pre-deploy audit check, unit/E2E/integration test, or CI job fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Architecture Strategist`, `Security Lens Reviewer`, `Scope Boundary Auditor`, `Constitution Reviewer`, `Go Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings from the 7-persona review panel remediated in-place in `docs/plans/2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-plan.md` and `docs/decisions/2026-11-16-live-integration-env-expansion-and-recap-headless-fixes-deliberation.md` prior to harvest; zero P0 findings).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-030-1`..`PA-030-4`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` | `subagent` (`multi-agent`) | 0 P0/P1/P2, 2 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 2 P3 | Remediated — `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Schema-CLI-Docs Coupling Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `rubber-duck` (`Correctness Reviewer`) | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2 | Remediated — `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Security Lens Reviewer`, `Constitution Reviewer`, `rubber-duck`, `Schema-CLI-Docs Coupling Reviewer`] Rightmost-segment `go-teams` -> `teams-cli` fallback and fail-closed `%VAR%`/`$VAR` expansion in `resolveRelative`**:
   * **Finding**: Using `strings.Replace(expanded, "go-teams", "teams-cli", 1)` on the leftmost match would rewrite the `go-teams` repository root directory (`C:\Source\GitHub\go-teams\...` -> `C:\Source\GitHub\teams-cli\...`) on repo-relative paths; calling `filepath.Clean` before `resolveRelative`'s `isBrowserPath` and `hasDrive` checks would strip `./` from `./msedge.exe` and collapse `C:../..` on Linux; and an unset/empty `%VAR%` or `$VAR` could collapse to a root-anchored path (`/go-teams/profile`).
   * **Remediation Applied**: Updated U1, U2, and U3 so `resolveExistingProfileDir` replaces only the rightmost `go-teams` path segment (never rewriting `repoRoot`), `resolveRelative` checks `isBrowserPath` and `hasDrive` on the raw value before cleaning and fails closed when a referenced `%VAR%` or `$VAR` is unset/empty, both `integration_helper_test.go` and `binary_automation_test.go` apply session-aware profile fallback and `GO_TEAMS_BINARY` `teams-cli.exe` -> `go-teams.exe` fallback, and `recapIntegrationURL`, `TestMeetingLiveScheduledOvertimeAndCallEnd`, and `requireCLIIntegration` verify `realProfile` is not an ancestor of `realRoot`.
2. **[P1/P2 — `rubber-duck`, `Security Lens Reviewer`] Preserve `isDownloadBlocked=true` across `resp.url` redirects, trailing `blob:` candidates in `extractRecapAudioJS`, and multi-target evaluation in `captureRecapAudioFromTargets`**:
   * **Finding**: In `extractRecapAudioJS`, SharePoint's `isDownloadBlocked` response may redirect `fetch` to an HTML error URL lacking `driveItemId` (failing the `resp.url` identity check before `ct.includes('text/html')` is reached) or be overwritten by a subsequent `blob:` candidate on `media.currentSrc` throwing `TypeError: Failed to fetch`.
   * **Remediation Applied**: Updated U1 so `extractRecapAudioJS` tracks `blockedErr` separately, checks `!resp.ok && info.isDownloadBlocked` and `ct.includes('text/html')` before the `resp.url` identity check, and throws `blockedErr || lastFetchErr` at the end of the loop; updated `captureRecapAudioFromTargets` to preserve `isDownloadBlocked=true` errors across targets; and specified `node -e` unit test execution in `internal/browser/recap_test.go` (U3).
3. **[P2/P3 — `rubber-duck`, `Constitution Reviewer`, `Architecture Strategist`, `Security Lens Reviewer`] Clean imports and channel race guard in `meeting_integration_test.go`, boolean assertions on live `.md` transcripts, `PA-030-4`, and `os.UserHomeDir()` error check**:
   * **Remediation Applied**: Updated U2 to remove unused `"github.com/chromedp/chromedp"` and add `"strings"` in `meeting_integration_test.go`, guard `case err := <-errCh:` with `select { case <-got: default: ... }` in `TestMeetingEnableCaptions`, and declare `streamCtx` before the `rm.IsRecap()` check in `TestMeetingLiveScheduledOvertimeAndCallEnd`; updated U3 to require boolean `require.True(t, strings.Contains(...), "static message")` on live `.md` content; checked `os.UserHomeDir()` error return in `resolveExistingProfileDir`; and added `PA-030-4` and the 3-threat model.

