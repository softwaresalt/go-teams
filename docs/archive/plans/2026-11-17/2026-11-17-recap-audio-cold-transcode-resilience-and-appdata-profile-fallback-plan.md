---
title: Recap audio cold SharePoint transcode resilience and %APPDATA% profile fallback implementation plan
description: Implementation plan for 035-F / 035-S covering stash entry 6E17E6EA
source: docs/decisions/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-deliberation.md
deliberation_id: 024-DL
feature: 035-F
shipment: 035-S
source_stash:
  - 6E17E6EA
date: 2026-11-17
status: approved
tags:
  - audio
  - recap
  - browser
  - sharepoint
  - transcode
  - profile-dir
  - appdata
---

## Problem Frame

Following the accepted deliberation in [2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-deliberation.md](../../../decisions/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-deliberation.md) (`024-DL`), stash entry **`6E17E6EA`** (`priority: high`, `kind: bug`) is planned under covering feature **`035-F`** and shipment **`035-S`**, sequenced immediately after `034-S` and before `010-S` in the shipment dependency DAG **`034-S -> 035-S -> 010-S`**.

When running `go-teams` in `--queue` mode against a cold 60.6-minute (`3637.76s`, `605`-segment) Teams Meeting Recap (`https://teams.microsoft.com/l/meetingrecap?...`), two issues surfaced in `internal/browser/recap.go`:

1. **`ErrAccountRequired` when the authenticated Teams session lives under `%APPDATA%\go-teams\profile` (or `%APPDATA%\teams-cli\profile`) and `--profile-dir` is omitted**:
   * `config.DefaultConfig()` resolves `ProfileDir` to `%LOCALAPPDATA%\go-teams\profile` on Windows.
   * `resolveExpandedProfileDir(expanded string)` in `internal/browser/recap.go` checks `expanded` and `replaceRightmostPathSegment(expanded, "go-teams", "teams-cli")` within the same `%LOCALAPPDATA%` parent, never checking the `%APPDATA%` / `os.UserConfigDir()` sibling `go-teams/profile` and `teams-cli/profile` directories when `%LOCALAPPDATA%\go-teams\profile` has no persisted Teams session on disk.
2. **`Error: recap audio MSE stream incomplete before deadline` on a cold 60.6-minute (`605`-segment) recording**:
   * When `'Play video'` is clicked on `streamembed.aspx`, the player's MSE pipeline appends `mseInitSegment` (`1` init segment) and `1-2` initial `mseSegments` (`~12s` of audio) before `runPathA` starts (~1.5s later) and pauses `<video>`.
   * For `605` audio segments (`wsd=96256`, `ppd=58204160`), `8` Path A workers (`76` rounds per worker) against cold SharePoint `oneDrive.transcode` segments (~1.6s-2.0s per on-demand transcode round-trip or transient HTTP `408`/`429`/`5xx` responses) can exceed the hardcoded `120,000` ms (`2m`) `deadlineAt` or exhaust the 3-attempt (`250ms, 500ms`) retry budget.
   * When `while (Date.now() < deadlineAt)` exits at `deadlineAt` while `pathAPromise` is still in flight (`pathAPromise && !pathAFailed && !pathAResult`), because `mseInitSegment` and those `1-2` initial `mseSegments` (~12s) are non-empty, `if (!initSegment || !orderedSegments.length)` is `false`. Execution falls into Path B's completeness check (`mseContiguousEnd < finalTargetDur - 0.5`: `12s < 3637.26s`) and throws the misleading `'recap audio MSE stream incomplete before deadline'` error — and `captureRecapAudioFromTargets` does not retry because `hasStreamEmbedTarget` is `true`.
   * Additionally, `triggerPlaybackIfReady()` uses a single `playTriggered` boolean set when `btn.click()` runs at `t = 0`, so if `<video>` (`m`) mounts slightly later in a paused state before Path A starts, `m.play()` is never called.

### Explicit Out-of-Scope Boundaries

* All other pre-existing active stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, `4AB8EAEE`, `E03CB3C3`, `4C40AF2B`, `BA069506`, `196AB9E4`, `69AB7ACC`, `0DE2E900`) remain untouched in `active` stash state.
* No changes to `config.DefaultConfig()` or external dependencies (`go.mod` / `go.sum`).

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: In `extractRecapAudioJS` (`internal/browser/recap.go`), scale `deadlineAt` to `240000` ms (`4m`) when `maxRecapAudioDurationSec > 300` (keeping `120000` ms for short unit-test caps), increase `runPathA` worker concurrency from `8` to `12`, mark HTTP `408` as retryable in `fetchBytesOnce` alongside `429`/`5xx`, scale `fetchBytesWithCap` to `5` attempts (`250ms, 500ms, 1000ms, 2000ms` backoff) when `maxRecapAudioDurationSec > 300`, track `btnClicked` and `mediaPlayTriggered` separately in `triggerPlaybackIfReady()`, and throw `'recap audio Path A capture deadline exceeded'` before Path B MSE fallback when `pathAPromise && !pathAFailed && !pathAResult` at `deadlineAt` | `6E17E6EA` | U1 (`035.001-T`), U2 (`035.002-T`) | `go test -race ./internal/browser/...` |
| R2: In `captureRecapAudioFromTargets` and `recapAudioEvalTimeout` (`internal/browser/recap.go`), increase `recapAudioEvalTimeout` from `5 * time.Minute` to `10 * time.Minute` and retry once in `shouldRetry` when `attempt == 0 && attempt < maxAttempts-1` fails with `'Path A capture deadline exceeded'` or `'MSE stream incomplete before deadline'` | `6E17E6EA` | U1 (`035.001-T`), U2 (`035.002-T`) | `go test -race ./internal/browser/...` |
| R3: In `resolveExpandedProfileDir` (`internal/browser/recap.go`), when neither `expanded` nor `legacy` has a persisted Teams session on disk (`!hasTeamsSessionInExpandedProfile`) and `expanded` is a standard `<LOCALAPPDATA\|APPDATA\|UserConfigDir>/<go-teams\|teams-cli>/profile` path outside a repository root (`!isPathInsideRepoRoot`), fall back to `APPDATA` / `os.UserConfigDir()` / `LOCALAPPDATA` sibling `go-teams/profile` and `teams-cli/profile` directories containing a persisted Teams session | `6E17E6EA` | U1 (`035.001-T`), U2 (`035.002-T`) | `go test -race ./internal/browser/...` |
| R4: Add unit tests in `internal/browser/recap_test.go` and update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` | `6E17E6EA` | U2 (`035.002-T`), U3 (`035.003-T`) | `go test -race ./...`, `go vet ./...`, `golangci-lint run` |

## Implementation Units

### U1 (`035.001-T`) — Harden `extractRecapAudioJS` cold-transcode resilience, `captureRecapAudioFromTargets` retry & timeout, and `resolveExpandedProfileDir` `%APPDATA%` fallback (`internal/browser/recap.go`)

* **Domain:** code (`internal/browser/recap.go`). **Posture:** test-first (Constitution Principle II Wave-Staged Red-Phase Harness exception for width-isolated tasks: stage U2's `internal/browser/recap_test.go` red-phase tests in `logs/staged-harnesses/035.002-T/` before editing `internal/browser/recap.go`, then activate in U2). **Size:** `S` | **Complexity:** `medium`.
* **Files (1 file):** `internal/browser/recap.go`.
* **What:**
  1. **In `extractRecapAudioJS` (`internal/browser/recap.go`)**:
     * **HTTP 408 retryability in `fetchBytesOnce`**:
       Update the non-OK HTTP status check in `fetchBytesOnce` from `if (!resp || st === 429 || st >= 500) err.retryable = true;` to:
       ```javascript
       if (!resp || st === 408 || st === 429 || st >= 500) err.retryable = true;
       ```
     * **5-attempt exponential backoff in `fetchBytesWithCap`**:
       Support an optional `recapAudioFetchMaxAttempts` override and otherwise scale `maxAttempts` to `5` when `maxRecapAudioDurationSec > 300` (keeping `3` for short unit-test caps) with backoff `250ms, 500ms, 1000ms, 2000ms`:
       ```javascript
       async function fetchBytesWithCap(urlStr, origFetchFn, hookWin, signal) {
           const parentSignal = signal || (abortCtrl ? abortCtrl.signal : undefined);
           const maxAttempts = (typeof recapAudioFetchMaxAttempts === 'number' && recapAudioFetchMaxAttempts > 0)
               ? recapAudioFetchMaxAttempts
               : ((typeof maxRecapAudioDurationSec === 'number' && maxRecapAudioDurationSec > 300) ? 5 : 3);
           let lastAttemptErr = null;
           for (let attempt = 0; attempt < maxAttempts; attempt++) {
               if (fatalErr) throw fatalErr;
               if (parentSignal && parentSignal.aborted) {
                   throw new Error('transcode fetch aborted');
               }
               try {
                   return await fetchBytesOnce(urlStr, origFetchFn, hookWin, signal);
               } catch (err) {
                   lastAttemptErr = err;
                   if (fatalErr || !err || !err.retryable || attempt === maxAttempts - 1 || (parentSignal && parentSignal.aborted)) {
                       throw err;
                   }
                   const backoffMs = attempt < 2 ? 250 * (attempt + 1) : 500 * Math.pow(2, attempt - 1);
                   await new Promise((resolve, reject) => {
                       ...
                       tid = setTimeout(() => {
                           ...
                           resolve();
                       }, backoffMs);
                       activeTimers.add(tid);
                   });
               }
           }
           throw lastAttemptErr;
       }
       ```
     * **12-worker concurrency in `runPathA`**:
       Change worker pool size from `Math.min(8, totalSegments)` to `Math.min(12, totalSegments)`:
       ```javascript
       for (let i = 0; i < Math.min(12, totalSegments); i++) workers.push(worker());
       ```
     * **Separate `btnClicked` and `mediaPlayTriggered` in `triggerPlaybackIfReady()` and 4-minute production `deadlineAt`**:
       Support an optional `recapAudioCaptureDeadlineMs` override and otherwise use `240000` ms (`4m`) when `maxRecapAudioDurationSec > 300` (`120000` ms for short unit-test caps):
       ```javascript
       let btnClicked = false;
       let mediaPlayTriggered = false;
       function triggerPlaybackIfReady() {
           if (pathAActive || (btnClicked && mediaPlayTriggered)) return;
           const btn = playerDoc.querySelector ? playerDoc.querySelector(playBtnSelector) : null;
           const m = playerDoc.querySelector ? playerDoc.querySelector('audio, video') : null;
           if (m && !mediaPlayTriggered) {
               try {
                   m.muted = true;
                   m.volume = 0;
                   if (m.playbackRate !== 16) m.playbackRate = 16;
               } catch (_) {}
           }
           if (!btnClicked && btn && typeof btn.click === 'function') {
               btnClicked = true;
               btn.click();
           }
           if (!mediaPlayTriggered && !pathAActive && m && typeof m.play === 'function') {
               mediaPlayTriggered = true;
               try { Promise.resolve(m.play()).catch(() => {}); } catch (_) {}
           }
       }
       triggerPlaybackIfReady();

       const startedAt = Date.now();
       const isProductionCap = typeof maxRecapAudioDurationSec === 'number' && maxRecapAudioDurationSec > 300;
       const captureDeadlineMs = (typeof recapAudioCaptureDeadlineMs === 'number' && recapAudioCaptureDeadlineMs > 0)
           ? recapAudioCaptureDeadlineMs
           : (isProductionCap ? 240000 : 120000);
       const deadlineAt = startedAt + captureDeadlineMs;
       const initialActivityTimeoutMs = 10000;
       let pathAResult = null;

       while (Date.now() < deadlineAt) {
           if (!btnClicked || !mediaPlayTriggered) triggerPlaybackIfReady();
       ```
     * **In-flight `pathAPromise` deadline check before Path B MSE fallback**:
       Immediately after `if (fatalErr) throw fatalErr;` following the `while (Date.now() < deadlineAt)` loop (before falling back to `mseInitSegment` / `mseSegments`), add:
       ```javascript
       if (pathAPromise && !pathAFailed && !pathAResult) {
           throw new Error('recap audio Path A capture deadline exceeded');
       }
       ```
       Because `runPathA` paused `<video>` when Path A started, any `mseInitSegment` / `mseSegments` buffered in the 1.5s before `runPathA` started cannot form a complete Path B stream if `pathAPromise` was still in flight when `deadlineAt` expired.
  2. **In `captureRecapAudioFromTargets` and `recapAudioEvalTimeout` (`internal/browser/recap.go`)**:
     * Increase `recapAudioEvalTimeout` from `5 * time.Minute` to `10 * time.Minute` and enforce it as an **operation-wide context deadline** at the start of `captureRecapAudioFromTargets`:
       ```go
       var recapAudioEvalTimeout = 10 * time.Minute
       ```
       Inside `captureRecapAudioFromTargets(ctx context.Context, ...)`:
       ```go
       captureCtx, cancelCapture := context.WithTimeout(ctx, recapAudioEvalTimeout)
       defer cancelCapture()
       ```
       Use `captureCtx` for target frame discovery (`fetchOwnedFrames`), per-target evaluation (`evalInCDPTargetWithin(captureCtx, target.WebSocketDebuggerURL, expr, recapAudioEvalTimeout)`), chunked transfer (`newCDPEvalSession(captureCtx, ...)`, `pullSess.eval(captureCtx, ...)`), and the retry wait (`<-captureCtx.Done()`), while returning `ctx.Err()` when the caller's parent `ctx` was cancelled and `captureCtx.Err()` when the operation-wide `recapAudioEvalTimeout` expires. This guarantees that multiple eligible targets and the attempt-0 retry cannot multiply total queue blocking time beyond `recapAudioEvalTimeout` (`10m`).
     * In `captureRecapAudioFromTargets`, allow a single retry (`attempt == 0 && attempt < maxAttempts-1`) when `lastErr` is a stream capture deadline error (`"Path A capture deadline exceeded"` or `"MSE stream incomplete before deadline"`), even when `hasStreamEmbedTarget` is `true`:
       ```go
       isRetryableStreamDeadline := attempt == 0 && lastErr != nil &&
           (strings.Contains(lastErr.Error(), "Path A capture deadline exceeded") ||
               strings.Contains(lastErr.Error(), "MSE stream incomplete before deadline"))
       shouldRetry := attempt < maxAttempts-1 &&
           (isRetryableStreamDeadline ||
               (!hasStreamEmbedTarget &&
                   ((lastErr != nil && strings.Contains(lastErr.Error(), "isDownloadBlocked=true")) ||
                       (lastErr == nil && hasXplatTarget))))
       ```
  3. **In `resolveExpandedProfileDir` (`internal/browser/recap.go`)**:
     * Add `isPathInsideRepoRoot(p string) bool` walking parent directories and checking `isRepoRootDir(dir)`.
     * Add helper `userConfigProfileBases() []string` returning non-empty deduplicated cleaned paths from `os.Getenv("APPDATA")`, `os.UserConfigDir()` (checking `if cfgDir, err := os.UserConfigDir(); err == nil`), and `os.Getenv("LOCALAPPDATA")`.
     * In `resolveExpandedProfileDir(expanded string) string`:
       When neither `expanded` nor `legacy` (`replaceRightmostPathSegment(expanded, "go-teams", "teams-cli")`) has a persisted Teams session on disk (`!hasTeamsSessionInExpandedProfile`), and `!isPathInsideRepoRoot(expanded)`:
       Check whether `filepath.Clean(expanded)` matches `filepath.Clean(filepath.Join(base, app, "profile"))` (`strings.EqualFold`) for any `base` in `userConfigProfileBases()` and `app` in `{"go-teams", "teams-cli"}`. When it does, iterate across `userConfigProfileBases()` (skipping any `base` where `isPathInsideRepoRoot(base)` is `true`) and check `filepath.Join(base, "go-teams", "profile")` followed by `filepath.Join(base, "teams-cli", "profile")`. If `hasTeamsSessionInExpandedProfile(cand)` is `true`, return `cand`. Any profile directory that already contains a persisted session on disk (`expanded` or same-parent `legacy`), or any custom `--profile-dir` outside the canonical `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` paths, remains authoritative and is never redirected.
* **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./internal/browser/...`, and `golangci-lint run ./internal/browser/...`.
* **Depends on:** none.

### U2 (`035.002-T`) — Add unit tests for cold-transcode resilience, deadline retry, and `%APPDATA%` profile fallback (`internal/browser/recap_test.go`)

* **Domain:** test (`internal/browser/recap_test.go`). **Posture:** test-first (activate the Wave-Staged Red-Phase Harness from `logs/staged-harnesses/035.002-T/`). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `035.001-T` (`blocks`).
* **Files (1 file):** `internal/browser/recap_test.go`.
* **What:**
  1. **In `TestResolveExistingProfileDirExpansionAndFallback` (`internal/browser/recap_test.go`)**:
     * Add subtests/assertions verifying:
       * When `LOCALAPPDATA` (`%LOCALAPPDATA%\go-teams\profile` and `%LOCALAPPDATA%\teams-cli\profile`) has no persisted Teams session, `resolveExistingProfileDir` falls back to `%APPDATA%\go-teams\profile` (or `%APPDATA%\teams-cli\profile` when `%APPDATA%\go-teams\profile` is cold) when a persisted Teams session (`SKYPE-TOKEN` / `UserAuthState` / `msal.2.account.keys`) exists there.
       * When `%LOCALAPPDATA%\go-teams\profile` itself has a persisted Teams session, it takes precedence over `%APPDATA%\go-teams\profile`.
       * Arbitrary non-`LOCALAPPDATA`/`APPDATA` paths (such as `filepath.Join(t.TempDir(), "go-teams", "profile")`) and paths inside a repository root (`go.mod` / `.git`) never fall back to `%APPDATA%`.
  2. **Add `TestRecapAudioColdTranscodeResilienceAndDeadlineRetry` in `internal/browser/recap_test.go`**:
     * **Subtest 1 (`path-a-in-flight-at-deadline-throws-path-a-deadline-even-with-mse-segments`)**:
       Simulate `streamembed.aspx` where `playBtn.click()` appends `mseInitSegment` and 1 initial `mseSegment` (`tfdt=0`, `duration=60`, `buffered end=6`) before Path A starts (`maxRecapAudioDurationSec = 4200 > 300`), and `fetch` for `segmentTime=0` stalls until `fakeNow += 245000` (`> 240000ms` production deadline; verify `fakeNow += 125000` does NOT trigger the deadline when `maxRecapAudioDurationSec = 4200`). Verify `extractRecapAudioJS` throws `'recap audio Path A capture deadline exceeded'` rather than `'recap audio MSE stream incomplete before deadline'`.
     * **Subtest 2 (`path-a-12-workers-5-attempts-408-retry-and-late-video-mount-play`)**:
       Simulate `maxRecapAudioDurationSec = 4200` with `15` segments (`wsd=96256`, `ppd=1443840`), where:
       * At `t = 0`, `playBtn` is mounted in `document.querySelector` while `<video>` (`videoEl`) is `null`; `playBtn.click()` mounts `videoEl` (`paused: true`) without starting playback until `triggerPlaybackIfReady()` runs on the next loop tick and calls `videoEl.play()` (which triggers the initial transcode `initsegment` fetch).
       * `segmentTime=0` fails 4 times with HTTP `408`, `429`, `502`, and `503` (exercising attempts `0..3` with `250ms, 500ms, 1000ms, 2000ms` backoff) and succeeds on attempt `4` (the 5th attempt).
       * Track maximum concurrent in-flight `mediasegment` fetches and verify it reaches `12` workers and decodes all `15` segments in order.
     * **Subtest 3 (`capture-from-targets-retries-once-on-attempt-0-stream-deadline-error`)**:
       Verify `recapAudioEvalTimeout == 10 * time.Minute`, and test `captureRecapAudioFromTargets` against a loopback CDP server with multiple owned targets (`owner` page, `xplatplugins.aspx` iframe, and `streamembed.aspx` iframe with `hasStreamEmbedTarget == true`) where attempt `0` returns a JS exception `'recap audio Path A capture deadline exceeded'` (or `'recap audio MSE stream incomplete before deadline'`) and attempt `1` returns valid PCM chunks after warming the cache; verify `captureRecapAudioFromTargets` succeeds on attempt `1` and writes valid MP3 frames, and verify that if both attempt `0` and attempt `1` fail with `'recap audio Path A capture deadline exceeded'`, it stops after `2` stream-target evaluations (`attempt == 0` retry only) rather than running all `8` attempts.
* **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, and `golangci-lint run`.

### U3 (`035.003-T`) — Document cold-transcode audio resilience and `%APPDATA%` profile fallback in `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` (`docs/`)

* **Domain:** docs (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **Depends on:** `035.001-T` (`blocks`).
* **Files (2 files):** `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`.
* **What:**
  1. **In `docs/ARCHITECTURE.md`**:
     * In `### Finite Meeting Recap export` (item 3), note that Path A fetches and decrypts audio segments with 12-worker concurrency, a 4-minute capture deadline (`10m` operation-wide CDP evaluation budget), 5-attempt exponential retry backoff (`250ms-2000ms` across HTTP `408`/`429`/`5xx`), and an automatic single-retry recovery pass when a cold recording exceeds the capture deadline on attempt 0.
     * In `### Live join and captions`, note that profile directory resolution (`NewLiveMeeting` / `resolveExistingProfileDir`) checks both `%LOCALAPPDATA%\go-teams\profile` (plus legacy `teams-cli\profile`) and `%APPDATA%\go-teams\profile` (`os.UserConfigDir()`, plus legacy `teams-cli\profile`) when the default per-user profile path has no persisted Teams session on disk.
  2. **In `docs/USER_GUIDE.md`**:
     * In `## Headless automation` (where `config.DefaultConfig()` and `%LOCALAPPDATA%\go-teams\profile` are described), document that when `%LOCALAPPDATA%\go-teams\profile` has no persisted Teams session on disk, `go-teams` automatically falls back to an existing authenticated profile under `%APPDATA%\go-teams\profile` or `%APPDATA%\teams-cli\profile` (`os.UserConfigDir()`).
* **Verify:** Run `go test -race ./internal/config/...` and `backlogit doctor`.

## Dependency Graph

```text
035.001-T (U1: internal/browser/recap.go)
    |
    +---> 035.002-T (U2: internal/browser/recap_test.go)
    |
    +---> 035.003-T (U3: docs/ARCHITECTURE.md + docs/USER_GUIDE.md)
```

* **Shipment sequencing DAG**:
  * Remove `010-S -> 034-S` direct edge (`backlogit dep remove 010-S 034-S`).
  * Add `035-S -> 034-S` (`backlogit dep add 035-S 034-S --type blocks` — `034-S` is `shipped`, placing `035-S` in `ready_set`).
  * Add `010-S -> 035-S` (`backlogit dep add 010-S 035-S --type blocks` — sequencing `010-S` after `035-S`).

## Decisions and Rationale

1. **Gate 4-minute `deadlineAt` and 5-attempt retry schedule on `maxRecapAudioDurationSec > 300`**:
   * **Rationale**: Production sets `maxRecapAudioDurationSec = 4200` (`70m`), whereas existing deterministic unit tests in `internal/browser/recap_test.go` set `maxRecapAudioDurationSec = 120` (or `2`, `10`) and advance `fakeNow += 125000` or expect `3` attempts (`250ms, 500ms`). Scaling `deadlineAt` to `240000ms` and `maxAttempts` to `5` (`250ms, 500ms, 1000ms, 2000ms`) when `maxRecapAudioDurationSec > 300` gives hour-long cold SharePoint recordings a 4-minute window and 3.75s of per-segment retry tolerance while preserving every existing unit test unchanged.
2. **Check `pathAPromise && !pathAFailed && !pathAResult` before falling back to `mseInitSegment` / `mseSegments`**:
   * **Rationale**: When `'Play video'` is clicked on `streamembed.aspx`, the browser's MSE pipeline appends `mseInitSegment` and 1-2 initial `mseSegments` (~12s) before `runPathA` starts and pauses `<video>`. If `deadlineAt` expires while `pathAPromise` is still in flight, those 1-2 initial MSE segments are not a Path B fallback stream; checking `pathAPromise && !pathAFailed && !pathAResult` immediately after the loop reports the true failure (`'recap audio Path A capture deadline exceeded'`) and enables `captureRecapAudioFromTargets` to retry once with a warm transcode cache.
3. **Restrict `%APPDATA%` / `os.UserConfigDir()` fallback in `resolveExpandedProfileDir` to standard per-user profile paths outside repository roots**:
   * **Rationale**: Checking `userConfigProfileBases()` only when `expanded` matches `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` and `!isPathInsideRepoRoot(expanded)` allows `--queue` and CLI runs using `DefaultConfig()` (`%LOCALAPPDATA%\go-teams\profile`) to discover existing authenticated sessions in `%APPDATA%\go-teams\profile` or `%APPDATA%\teams-cli\profile` while ensuring unit tests using `t.TempDir()` or repo-local profiles never pick up a developer's host profile.

## Risks and Caveats

* **Lightweight 3-Threat Model**:
  1. **Most likely threat**: Existing unit tests in `internal/browser/recap_test.go` (`TestNewLiveMeetingRemoteHeadlessUsesCleanLauncherAndJoinOnWebSel`, `TestResolveExistingProfileDirExpansionAndFallback`, `U1-R2`, `U3-R2`, `U1-R4`) failing if `resolveExpandedProfileDir` matches arbitrary `t.TempDir()/go-teams/profile` directories on a host with a live `%APPDATA%\go-teams\profile`, or if `deadlineAt` / `maxAttempts` changes affect `maxRecapAudioDurationSec = 120` tests. Mitigated by requiring `expanded` to match a standard `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` path outside any repo root, and gating `240000ms` / `5` attempts on `maxRecapAudioDurationSec > 300`.
  2. **Highest-impact threat**: Calling `m.play()` from `triggerPlaybackIfReady()` after `runPathA` has already started (`pathAActive === true`) and paused `<video>`, which would resume competing video/MSE network fetches during Path A. Mitigated by guarding `triggerPlaybackIfReady()` with `if (pathAActive || (btnClicked && mediaPlayTriggered)) return;` and `if (!mediaPlayTriggered && !pathAActive && m && typeof m.play === 'function')`.
  3. **Most subtle threat**: `captureRecapAudioFromTargets` retrying all `8` attempts (`8 * 4m = 32m`) when a recording is permanently broken. Mitigated by restricting `isRetryableStreamDeadline` to `attempt == 0` (at most 1 retry = 2 total attempts) and bounding total CDP evaluation by `recapAudioEvalTimeout = 10 * time.Minute` and `ctx.Done()`.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, zero `unsafe`, all errors wrapped with context |
| II. Test-First Development | Compliant — Wave-Staged Red-Phase Harness staged in `logs/staged-harnesses/035.002-T/` before editing `internal/browser/recap.go` in U1 and activated in U2 |
| III. Workspace Isolation | Compliant — all profile fallback tests use `t.TempDir()` and `t.Setenv("LOCALAPPDATA", ...)` / `t.Setenv("APPDATA", ...)` |
| IV. CLI Workspace Containment | Compliant — `isPathInsideRepoRoot` prevents profile resolution from escaping or rewriting repository-local paths |
| V. Structured Observability | Compliant — `slog.Debug` / `slog.Info` elapsed and duration telemetry preserved; mandatory post-merge context compaction (P-020) enforced at closure |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); 3 width-isolated tasks (`1-2` files each) |
| VII. Destructive Command Approval | Compliant — read-only profile session discovery and in-memory audio stream capture |
| VIII. Explicit Safety Modes | Compliant — `Careful mode` + `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory (`PA-035-1`..`PA-035-3`) recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded command outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No exported Go type signatures, CLI flags, or YAML config keys change.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — `resolveExpandedProfileDir` discovers persisted Teams authentication sessions across `%LOCALAPPDATA%` and `%APPDATA%` (`os.UserConfigDir()`), and `extractRecapAudioJS` replays allowlisted SharePoint transcode requests across 12 workers and 5 retry attempts.
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Read-only profile directory fallback.
* **External integration, operator checkpoint, or external dependency**: **Present** — Interacts with SharePoint `oneDrive.transcode` and `streamembed.aspx` / `stream.aspx` over CDP.
* **High runtime, rollout, or rollback risk**: **Present** — Modifies in-page audio capture deadline (`240s`), CDP evaluation timeout (`10m`), and retry behavior in `captureRecapAudioFromTargets`.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * In-page audio stream extraction, CDP evaluation timeout/retry, and profile directory fallback (`internal/browser/recap.go`)
  * Unit tests (`internal/browser/recap_test.go`) and documentation (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`)
* **Runtime verification**:
  * Run `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...`.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, releasability evidence contract, and mandatory post-merge context compaction (`compact-context` skill, P-020) recorded in `docs/closure/` during Ship operational closure.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan modifies authenticated browser profile discovery (`resolveExpandedProfileDir`) and SharePoint `oneDrive.transcode` audio capture concurrency, retry backoff, capture deadline, and CDP evaluation timeout in `internal/browser/recap.go`.
* **Safety Mode Posture (Principle VIII)**: `Careful mode` + `Freeze-scope mode` (strictly scoped to the 4 files declared across `035.001-T`..`035.003-T`: `internal/browser/recap.go`, `internal/browser/recap_test.go`, `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`) + `Strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md`
  * `docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`

### Protected Invariants

1. **Same-Origin HTTPS Transcode Containment and Recording Identity Verification**: All `oneDrive.transcode` fetches in `runPathA` and `fetchBytesOnce` MUST continue to enforce `isAllowedSameOriginTranscodeURL`, `allowlistedHeaderNames`, and `checkIdentityAgainst` / `hasRecapAudioIdentityMatch`, and `track === 'video'` requests MUST NEVER seed or overwrite `transcodeWsd`, `transcodePpd`, or `transcodeAudioTemplateUrl`.
2. **In-Flight Path A Deadline Precedence Over Partial Pre-Pause MSE Buffers**: When `while (Date.now() < deadlineAt)` exits while `pathAPromise` is still in flight (`pathAPromise && !pathAFailed && !pathAResult`), `captureStreamEmbedAudio` MUST throw `'recap audio Path A capture deadline exceeded'` before inspecting `mseInitSegment` / `mseSegments`, and `cleanupHooks()` MUST abort `abortCtrl` (which aborts `pathAAbortCtrl` and clears any pending `fetchBytesWithCap` retry timers) without calling `m.play()` after cleanup.
3. **No Competing `<video>.play()` While Path A Is Active**: `triggerPlaybackIfReady()` MUST check `if (pathAActive || (btnClicked && mediaPlayTriggered)) return;` and `if (!mediaPlayTriggered && !pathAActive && m && typeof m.play === 'function')` so `<video>` stays paused once `runPathA` sets `pathAActive = true`.
4. **Scoped Cross-Root Profile Discovery (`%LOCALAPPDATA%` <-> `%APPDATA%` / `os.UserConfigDir()`)**: `resolveExpandedProfileDir` MUST prefer `expanded` and same-parent `legacy` (`teams-cli`) first, MUST NEVER rewrite or escape any path inside a git repository root (`isPathInsideRepoRoot`), and MUST only fall back across `LOCALAPPDATA`, `APPDATA`, and `os.UserConfigDir()` when `expanded` itself is a standard `<base>/<go-teams|teams-cli>/profile` path under one of those per-user roots (never redirecting arbitrary `t.TempDir()` paths).
5. **Bounded Attempt-0 Stream Deadline Retry**: `captureRecapAudioFromTargets` MUST retry `'Path A capture deadline exceeded'` or `'MSE stream incomplete before deadline'` at most once (`attempt == 0 && attempt < maxAttempts-1`) while respecting `ctx.Err()` cancellation and `cleanupPendingTransfer()`.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-035-1` | Scale production `deadlineAt` to `240s` (`maxRecapAudioDurationSec > 300`), increase `runPathA` workers to `12`, add HTTP `408` + 5-attempt backoff (`250/500/1000/2000ms`) in `fetchBytesOnce`/`fetchBytesWithCap`, split `btnClicked`/`mediaPlayTriggered` in `triggerPlaybackIfReady()`, prioritize in-flight `pathAPromise` deadline error before Path B MSE fallback, increase `recapAudioEvalTimeout` to `10m` with attempt-0 retry in `captureRecapAudioFromTargets`, and add `%APPDATA%` / `os.UserConfigDir()` fallback in `resolveExpandedProfileDir` | `internal/browser/recap.go` | Browser audio capture & profile resolution hardening | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-035-2` | Add unit tests for `%APPDATA%` / `os.UserConfigDir()` profile fallback, in-flight Path A deadline precedence over partial MSE buffers, 12-worker / 5-attempt / HTTP 408 backoff, late `<video>` mount playback trigger, and `captureRecapAudioFromTargets` attempt-0 stream deadline retry | `internal/browser/recap_test.go` | Unit test suite expansion | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-035-3` | Document cold SharePoint transcode audio resilience and `%LOCALAPPDATA%` <-> `%APPDATA%` (`os.UserConfigDir()`) profile fallback in architecture and user guide docs | `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md` | Documentation update | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 commit |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages, including `TestResolveExistingProfileDirExpansionAndFallback`, `TestRecapJSStreamEmbedHighSpeedAudioCaptureWhenDownloadBlocked`, `TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity`, and `TestRecapAudioColdTranscodeResilienceAndDeadlineRetry`.
  * [ ] `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` passes with zero findings.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `recap_audio_cold_transcode_and_retry_test_pass_rate` (100% pass rate across all `internal/browser` audio capture and retry unit/E2E tests), (2) `profile_dir_appdata_fallback_test_pass_rate` (100% pass rate in `TestResolveExistingProfileDirExpansionAndFallback`), and (3) `zero_credential_or_url_leak_rate` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == PASS`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main`.
  * **Dashboard / query**: `gh run list --workflow=ci.yml` and local `go test -race ./...` execution.
  * **Baseline**: 100% pass rate on `changes` and `test-windows` in CI.
  * **Alert threshold**: `>= 1` test failure or CI check failure.
* **Rollback trigger & procedure**:
  * **Trigger**: Any regression in Recap audio extraction, browser profile resolution, or CI quality gates on `main`.
  * **Procedure**: Revert the `035-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, unit tests, E2E tests, and GitHub Actions PR checks (`changes`, `test-windows`) pass and are recorded in `docs/closure/` (followed by mandatory post-merge context compaction under P-020).
  * `READY_WITH_CONDITIONS`: All local quality gates, unit tests, and PR CI checks pass, with any environment-specific condition documented and verified.
  * `BLOCKED`: Any pre-deploy audit check, unit test, or PR CI check fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist` on anchor route `gpt-6.1-sol`, and `Security Lens Reviewer`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all 4 findings from `Architecture Strategist` on anchor route `gpt-6.1-sol` and the `os.UserConfigDir()` error-check note from `Go Reviewer` were remediated in-place in `docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-plan.md` and `docs/decisions/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-deliberation.md` prior to harvest; zero P0 findings).
* **Capability Status (P-012):** `TOOL_OK: reviewer-subagent-dispatch`, `TOOL_OK: model-specific-review-routing` (`Architecture Strategist` dispatched on anchor route `gpt-6.1-sol` with `reasoning_effort: high`).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-035-1`..`PA-035-3`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` (anchor route `gpt-6.1-sol`) | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 0 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 (explicit `err == nil` check on `os.UserConfigDir()` noted and incorporated) | `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Learnings Researcher` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 (3 prior learnings verified aligned) | `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Architecture Strategist`] Enforce `recapAudioEvalTimeout` (`10m`) as an operation-wide deadline across targets, retry, and chunked transfer (`U1`, `U2`, `U3`)**:
   * **Finding**: `captureRecapAudioFromTargets` evaluates multiple targets sequentially in each attempt, so passing a fresh `recapAudioEvalTimeout` per target evaluation without an outer deadline could multiply queue blocking time when multiple targets are evaluated across attempt 0 and attempt 1.
   * **Remediation**: Derive `captureCtx, cancelCapture := context.WithTimeout(ctx, recapAudioEvalTimeout); defer cancelCapture()` at the start of `captureRecapAudioFromTargets` (`U1`) and use `captureCtx` across target discovery, per-target evaluation, the attempt-0 retry wait, and chunked PCM transfer; added multi-target (`owner` + `xplatplugins.aspx` + `streamembed.aspx`) coverage to `U2` Subtest 3 and updated `U3` documentation.
2. **[P1 — `Architecture Strategist`] Clarify authoritative profile precedence vs. cold canonical per-user cross-root fallback (`U1`, `U2`, `U3`)**:
   * **Finding**: Ensure any profile directory that already contains a persisted Teams session (`expanded` or same-parent `legacy`) or any custom `--profile-dir` outside the canonical `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` paths remains authoritative and is never redirected.
   * **Remediation**: Explicitly documented in `U1`, `U2`, `Protected Invariant 4`, and `024-DL` that `resolveExpandedProfileDir` returns `expanded` (or same-parent `legacy`) immediately whenever a session exists or when `expanded` is a custom/repo-local path, and only checks sibling `userConfigProfileBases()` (`APPDATA`, `os.UserConfigDir()` with `err == nil`, `LOCALAPPDATA`) when `expanded` is a cold canonical per-user `<base>/<go-teams|teams-cli>/profile` path.
3. **[P2 — `Architecture Strategist`] Support explicit `recapAudioCaptureDeadlineMs` and `recapAudioFetchMaxAttempts` overrides alongside `isProductionCap` (`U1`, `U2`)**:
   * **Finding**: Decouple explicit deadline/retry overrides from `maxRecapAudioDurationSec` while preserving compatibility with the 20+ existing `node -e` probes in `recap_test.go` and `recap_retry_test.go` that only inject `maxRecapAudioDurationSec`.
   * **Remediation**: Updated `U1` and `024-DL` so `extractRecapAudioJS` checks optional `recapAudioCaptureDeadlineMs` and `recapAudioFetchMaxAttempts` bindings first before defaulting to `isProductionCap ? 240000 : 120000` and `isProductionCap ? 5 : 3`.
4. **[P2 — `Architecture Strategist`] Scope `U3` documentation of profile fallback to `NewLiveMeeting` / `resolveExistingProfileDir` flows (`U3`)**:
   * **Finding**: `go-teams auth` (`ChromeRunner.Authenticate`) always requires `--account` / `default_account` to sign in and create a session, whereas `NewLiveMeeting` / `resolveExistingProfileDir` (used by `join`, `captions`, `--queue`, and `schedule`) resolves existing authenticated sessions across `%LOCALAPPDATA%` and `%APPDATA%`.
   * **Remediation**: Scoped `U3`'s `docs/ARCHITECTURE.md` update to `### Live join and captions` and `### Finite Meeting Recap export` alongside `docs/USER_GUIDE.md` `## Headless automation`.

