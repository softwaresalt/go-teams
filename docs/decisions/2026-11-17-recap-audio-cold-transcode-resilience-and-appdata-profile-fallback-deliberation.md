---
title: Recap audio cold SharePoint transcode resilience and %APPDATA% profile fallback
description: Deliberation on stash entry 6E17E6EA for covering feature 035-F and shipment 035-S
topic: Recap Audio Cold SharePoint Transcode Resilience and %APPDATA% Profile Fallback (035-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 024-DL
feature: 035-F
shipment: 035-S
source_stash:
  - 6E17E6EA
linked_artifacts:
  - docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-decided-plan.md
  - docs/archive/plans/2026-11-17/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-plan.md
  - .backlogit/archive/024-DL.md
  - .backlogit/archive/035-F.md
  - .backlogit/archive/035-S.md
date: 2026-11-17
status: accepted
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

When running `go-teams` in `--queue` mode against a 60.6-minute (`3637.76s`, `605`-segment) Microsoft Teams Meeting Recap (`https://teams.microsoft.com/l/meetingrecap?...`), two issues surfaced and were captured in stash entry **`6E17E6EA`** (`priority: high`, `kind: bug`):

1. **`cli: queue line 1`: `browser: initial authentication requires --account or default_account in config.yaml` (`ErrAccountRequired`)**:
   * When a queue entry or CLI invocation uses `config.DefaultConfig()` without an explicit `--profile-dir` flag or `profile_dir` in `config.yaml`, `defaultProfileDir` resolves `ProfileDir` to `%LOCALAPPDATA%\go-teams\profile` (`C:\Users\<user>\AppData\Local\go-teams\profile`).
   * Operators who authenticated with `TEAMS_PROFILE_DIR="%APPDATA%\go-teams\profile"` (or legacy `%APPDATA%\teams-cli\profile` under `os.UserConfigDir()` = `AppData\Roaming`) have their persisted Teams session under `%APPDATA%\go-teams\profile` (or `%APPDATA%\teams-cli\profile`), while `%LOCALAPPDATA%\go-teams\profile` has no session on disk.
   * Currently, `resolveExpandedProfileDir(expanded string)` in `internal/browser/recap.go` checks only `expanded` and `replaceRightmostPathSegment(expanded, "go-teams", "teams-cli")` within the same `%LOCALAPPDATA%` parent, never checking the `%APPDATA%` / `os.UserConfigDir()` sibling `go-teams/profile` and `teams-cli/profile` directories when `expanded` is a standard per-user `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` path outside a git repository root.

2. **`cli: queue line 3`: `Error: recap audio MSE stream incomplete before deadline` on a cold 60.6-minute (`605`-segment) recording**:
   * When `'Play video'` is clicked on `streamembed.aspx`, the player's MSE pipeline appends `mseInitSegment` (`1` init segment) and `1-2` initial `mseSegments` (`~12s` of audio) before `runPathA` starts (~1.5s later) and pauses `<video>`.
   * For a 60.6-minute (`3637.76s`) recording with `wsd=96256` and `ppd=58204160`, `runPathA` must fetch and decrypt `605` audio segments. With `8` workers (`76` sequential rounds per worker), when the recording's `track=audio&quality=audcopy` segments are cold on SharePoint's `oneDrive.transcode` service (~1.6s-2.0s per on-demand transcode round-trip, or hitting transient HTTP `408`/`429`/`5xx` throttling), `76` rounds exceed the hardcoded `120,000` ms (`2m`) `deadlineAt` (or a single segment fails after only 2 short retries of `250ms` + `500ms` = `750ms`, causing `runPathA` to abort and fall back to Path B with insufficient time remaining).
   * When `while (Date.now() < deadlineAt)` exits at `120s` while `pathAPromise` is still in flight (`pathAPromise && !pathAFailed && !pathAResult`), because `mseInitSegment` and those `1-2` initial `mseSegments` (~12s) are non-empty, `if (!initSegment || !orderedSegments.length)` is `false`. Instead of reporting `'recap audio Path A capture deadline exceeded'`, execution falls into Path B's completeness check (`mseContiguousEnd < finalTargetDur - 0.5`: `12s < 3637.26s`) and throws the misleading `'recap audio MSE stream incomplete before deadline'` error — and `captureRecapAudioFromTargets` does not retry because `hasStreamEmbedTarget` is `true`.
   * Additionally, in `triggerPlaybackIfReady()`, clicking `btn` sets `playTriggered = true` at `t = 0` when `<video>` (`m`) has not yet mounted in `streamembed.aspx`; 1s later when `<video>` mounts in a paused state (`m.paused === true`), `triggerPlaybackIfReady()` is never called again because `playTriggered` is already `true`.

### P-021 C5 and C6 Triage Record

* `6E17E6EA` does not carry the `DEFERRED SCOPE EXPANSION` marker; it is a high-priority bug entry (`kind: bug`, `priority: high`).
* **Obligation (A) Unconditional duplicate detection**: Executed across `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`. Outcome: **CLEAN DUPLICATE SCAN** for `6E17E6EA`. Archived shipments `031-S` (`9A16B213`, `020-DL`) and `033-S` (`D034C18B`, `022-DL`) implemented initial `streamembed.aspx` and `stream.aspx` audio capture, and active stash entries `39450846` and `7883544E` track unrelated minor R3/R5 review follow-ups from `031-S`. No other active or archived item addresses cold SharePoint transcode deadline scaling, 12-worker Path A concurrency, 5-attempt HTTP 408/429/5xx retry backoff, separate `btnClicked`/`mediaPlayTriggered` tracking, in-flight `pathAPromise` deadline precedence over partial MSE segments, `captureRecapAudioFromTargets` attempt-0 stream deadline retry, or `%APPDATA%` / `os.UserConfigDir()` fallback in `resolveExpandedProfileDir`.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (no `N/A` deferred-expansion source refs).

### Scope Boundaries

* **In scope**:
  * **U1 (`035.001-T` — `code` domain: `internal/browser/recap.go`, `Size: S`, `Complexity: medium`, 1 file)**:
    * In `extractRecapAudioJS`:
      1. Scale `deadlineAt` in `captureStreamEmbedAudio`: use `const isProductionCap = typeof maxRecapAudioDurationSec === 'number' && maxRecapAudioDurationSec > 300;` and `const captureDeadlineMs = (typeof recapAudioCaptureDeadlineMs === 'number' && recapAudioCaptureDeadlineMs > 0) ? recapAudioCaptureDeadlineMs : (isProductionCap ? 240000 : 120000); const deadlineAt = startedAt + captureDeadlineMs;` (`4m` in production when `maxRecapAudioDurationSec = 4200`, keeping `120000ms` for short-cap unit tests where `maxRecapAudioDurationSec <= 300`, while also supporting explicit `recapAudioCaptureDeadlineMs` overrides).
      2. Increase `runPathA` worker concurrency from `8` to `12` (`Math.min(12, totalSegments)`).
      3. In `fetchBytesOnce`, mark HTTP `408` as retryable alongside `429` and `>= 500` (`!resp || st === 408 || st === 429 || st >= 500`); in `fetchBytesWithCap`, use `const maxAttempts = (typeof recapAudioFetchMaxAttempts === 'number' && recapAudioFetchMaxAttempts > 0) ? recapAudioFetchMaxAttempts : ((typeof maxRecapAudioDurationSec === 'number' && maxRecapAudioDurationSec > 300) ? 5 : 3);` with backoff `attempt < 2 ? 250 * (attempt + 1) : 500 * Math.pow(2, attempt - 1)` (`250ms, 500ms, 1000ms, 2000ms` in production; `250ms, 500ms` in short-cap unit tests).
      4. In `triggerPlaybackIfReady()`, track `let btnClicked = false; let mediaPlayTriggered = false;` separately so `btn.click()` fires once when `btn` is present and `m.play()` fires once when `<video>` (`m`) is present while `!pathAActive`, and call `if (!btnClicked || !mediaPlayTriggered) triggerPlaybackIfReady();` in the `while (Date.now() < deadlineAt)` loop.
      5. Immediately after the `while (Date.now() < deadlineAt)` loop (and after `if (fatalErr) throw fatalErr;`), check `if (pathAPromise && !pathAFailed && !pathAResult) { throw new Error('recap audio Path A capture deadline exceeded'); }` BEFORE falling back to `mseInitSegment` / `mseSegments`.
    * In `captureRecapAudioFromTargets` and `recapAudioEvalTimeout`:
      1. Increase `recapAudioEvalTimeout` from `5 * time.Minute` to `10 * time.Minute` and apply it as an operation-wide context timeout (`captureCtx, cancelCapture := context.WithTimeout(ctx, recapAudioEvalTimeout); defer cancelCapture()`) across target discovery, per-target evaluation, attempt-0 retry wait, and chunked PCM transfer.
      2. In `captureRecapAudioFromTargets`, allow a retry on `attempt == 0 && attempt < maxAttempts-1` when `lastErr != nil && (strings.Contains(lastErr.Error(), "Path A capture deadline exceeded") || strings.Contains(lastErr.Error(), "MSE stream incomplete before deadline"))` so a cold recording whose first attempt warmed the SharePoint transcode cache succeeds on the retry.
    * In `resolveExpandedProfileDir`:
      1. Add helper `isPathInsideRepoRoot(p string) bool` walking parent directories via `isRepoRootDir`.
      2. When neither `expanded` nor `legacy` (`replaceRightmostPathSegment(expanded, "go-teams", "teams-cli")`) has a persisted Teams session on disk (`!hasTeamsSessionInExpandedProfile`), and `!isPathInsideRepoRoot(expanded)` and `expanded` matches a canonical per-user profile path under `LOCALAPPDATA`, `APPDATA`, or `os.UserConfigDir()` (`<base>/go-teams/profile` or `<base>/teams-cli/profile`), check the sibling `<base>/go-teams/profile` and `<base>/teams-cli/profile` directories across `APPDATA`, `os.UserConfigDir()`, and `LOCALAPPDATA` (checking `err == nil` on `os.UserConfigDir()` and excluding any path inside a repo root) and return the first one containing a persisted Teams session. Any profile path that already contains a session on disk, or any custom `--profile-dir` outside the canonical `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` locations, remains authoritative and is never redirected.
  * **U2 (`035.002-T` — `test` domain: `internal/browser/recap_test.go`, `Size: S`, `Complexity: low`, depends on `035.001-T`, 1 file)**:
    * Add unit tests in `internal/browser/recap_test.go` covering:
      * `resolveExistingProfileDir` / `resolveExpandedProfileDir` falling back from `%LOCALAPPDATA%\go-teams\profile` (when empty/missing) to `%APPDATA%\go-teams\profile` and `%APPDATA%\teams-cli\profile` (`os.UserConfigDir()`), preferring `%LOCALAPPDATA%` when it has a session, and never escaping a repository root or hijacking unrelated `t.TempDir()` paths.
      * `extractRecapAudioJS` throwing `'recap audio Path A capture deadline exceeded'` (not `'recap audio MSE stream incomplete before deadline'`) when `pathAPromise` is still in flight at `deadlineAt` even after initial `mseInitSegment` and `mseSegments` were appended before Path A paused `<video>`.
      * `extractRecapAudioJS` scaling `deadlineAt` to `240000ms` (`4m`) and `fetchBytesWithCap` to `5` attempts (`250ms, 500ms, 1000ms, 2000ms` backoff, including HTTP `408`) when `maxRecapAudioDurationSec > 300`, and running up to `12` concurrent Path A workers.
      * `triggerPlaybackIfReady()` clicking `btn` at `t = 0` when `<video>` is not yet mounted and then calling `m.play()` once `<video>` mounts on a later poll while `!pathAActive`.
      * `captureRecapAudioFromTargets` retrying once when `attempt == 0` fails with `'recap audio Path A capture deadline exceeded'` or `'recap audio MSE stream incomplete before deadline'` even when `hasStreamEmbedTarget` is `true`, and `recapAudioEvalTimeout == 10 * time.Minute`.
  * **U3 (`035.003-T` — `docs` domain: `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `Size: XS`, `Complexity: trivial`, depends on `035.001-T`, 2 files)**:
    * Update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` to document cold SharePoint transcode resilience (12-worker Path A concurrency, 4-minute capture deadline, 5-attempt exponential backoff, and single-retry recovery) and automatic `%LOCALAPPDATA%` <-> `%APPDATA%` (`os.UserConfigDir()`) profile session discovery fallback.
* **Out of scope**:
  * All other pre-existing active stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, `4AB8EAEE`, `E03CB3C3`, `4C40AF2B`, `BA069506`, `196AB9E4`, `69AB7ACC`, `0DE2E900`), which remain untouched in `active` stash state.
  * Changes to `config.DefaultConfig()` or external dependencies (`go.mod` / `go.sum`).

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md`**: SharePoint `stream.aspx` and `streamembed.aspx` audio streams use `track=audio&quality=audcopy` (`wsd=96256`, `16000` Hz sample units). All `transcodeWsd`, `transcodePpd`, and `transcodeAudioTemplateUrl` state must remain isolated from `track=video`, and `seedEncKeyFromFileInfo()` must not set `activityObserved` prematurely.
* **`docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`**: Hour-long recordings produce >4 MiB of base64 PCM chunks (`transferPending: true`) pulled over a persistent `cdpEvalSession`, and `recapAudioEvalTimeout` must exceed the worst-case in-page transcode fetch + WebAudio decode window (`240s` capture deadline + multi-hundred-segment WebAudio decode). Increasing `recapAudioEvalTimeout` from `5m` to `10m` prevents premature CDP WebSocket deadline cancellation during long cold-recording captures and retries.

## Options Evaluated

### Option A: In-Page Cold-Transcode Resilience + Single Attempt-0 Stream Deadline Retry + `%APPDATA%` / `os.UserConfigDir()` Profile Session Fallback in `resolveExpandedProfileDir` (Recommended)

* **Description**:
  * **U1 (`035.001-T` — `code` domain: `internal/browser/recap.go`, `Size: S`, `Complexity: medium`, 1 file)**: Implement all 5 `extractRecapAudioJS` improvements, the `recapAudioEvalTimeout = 10 * time.Minute` + `captureRecapAudioFromTargets` attempt-0 retry on stream deadline errors, and the `%APPDATA%` / `os.UserConfigDir()` profile session fallback in `resolveExpandedProfileDir`.
  * **U2 (`035.002-T` — `test` domain: `internal/browser/recap_test.go`, `Size: S`, `Complexity: low`, depends on `035.001-T`, 1 file)**: Add unit tests for all new behaviors while keeping existing `U1-R2`, `U3-R2`, and `U1-R4` fast-clock unit tests green.
  * **U3 (`035.003-T` — `docs` domain: `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `Size: XS`, `Complexity: trivial`, depends on `035.001-T`, 2 files)**: Update architecture and user guide documentation.
* **Pros**:
  * Cuts 605-segment Path A rounds from 76 (`8` workers) to 51 (`12` workers) while doubling the production capture budget (`120s` -> `240s`) and surviving transient SharePoint `408`/`429`/`5xx` transcode throttling (`5` attempts up to `2000ms` backoff).
  * Fixes the misleading Path B `'recap audio MSE stream incomplete before deadline'` error when `pathAPromise` is still in flight at `deadlineAt` after initial pre-Path-A MSE segments were buffered.
  * Allows a cold recording whose first attempt warmed SharePoint's server-side transcode cache to succeed automatically on attempt 1 (`captureRecapAudioFromTargets`).
  * Automatically discovers existing authenticated Teams sessions under `%APPDATA%\go-teams\profile` or `%APPDATA%\teams-cli\profile` when `%LOCALAPPDATA%\go-teams\profile` is cold, without breaking unit tests that pass temporary `t.TempDir()` profile paths or repository-local profiles.
* **Cons**:
  * A permanently broken stream that times out at `deadlineAt` on both attempt 0 and attempt 1 can take up to `2 * 4m = 8m` before failing (bounded by `ctx.Done()` and `recapAudioEvalTimeout = 10m`).
* **Effort estimate**: Low-Medium (3 atomic, width-isolated tasks: `S`, `S`, `XS`).
* **Fit**: 100% alignment with stash `6E17E6EA` and the root cause analysis.

### Option B: Rely Solely on Path B 16x MSE Playback for Long Cold Recordings

* **Description**: Disable Path A on long recordings and rely on `<video>.playbackRate = 16` MSE buffer capture.
* **Pros**: Simpler in-page code.
* **Cons**: At 16x playback rate, a 60.6-minute recording requires at least `3637.76 / 16 = 227.4s` of continuous gap-free audio/video playback even when warm, and stalls or drops ranges on cold SharePoint transcode segments.

### Option C: Change `config.DefaultConfig()` `defaultProfileDir` from `%LOCALAPPDATA%` to `%APPDATA%`

* **Description**: Change `defaultProfileDir` in `internal/config/config.go` to return `%APPDATA%\go-teams\profile` on Windows.
* **Pros**: Matches `OutputDir` parent (`%APPDATA%\go-teams`).
* **Cons**: Breaks existing installations that already authenticated under the default `%LOCALAPPDATA%\go-teams\profile` directory and violates Chromium's standard practice of storing browser user-data directories under `AppData\Local` rather than roaming `AppData\Roaming`.

## Trade-off Comparison

| Criterion | Option A (Recommended) | Option B (Path B only) | Option C (Change DefaultConfig) |
|---|---|---|---|
| Cold 60-min Recap audio reliability | High (12 workers, 4m budget, 5 retries, + 1 warm-cache retry) | Fails (>227s + video bandwidth overhead) | N/A (does not address audio) |
| Backward compatibility with existing profiles | Preserves `%LOCALAPPDATA%` preference; falls back to `%APPDATA%` only when cold | N/A | Breaks existing `%LOCALAPPDATA%` profiles |
| Unit test speed & compatibility | Preserves `120s` fake-clock & `3`-attempt caps when `maxRecapAudioDurationSec <= 300` | Breaks Path A tests | Breaks `internal/config` tests |

## Decision

We select **Option A** and promote directly to both implementation plan (`docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-decided-plan.md`; verbose original archived at `docs/archive/plans/2026-11-17/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-plan.md`) and backlog queue (`024-DL`, covering feature `035-F`, tasks `035.001-T`..`035.003-T`, and shipment `035-S`, archived post-merge under `.backlogit/archive/`).

## Rejected Alternatives

* **Option B** is rejected because Path B 16x MSE playback fetches both audio and video tracks and cannot complete a 60.6-minute cold recording reliably.
* **Option C** is rejected because changing `defaultProfileDir` in `DefaultConfig()` would break operators whose active Teams session lives in `%LOCALAPPDATA%\go-teams\profile`.

## Unresolved Questions

* None.

## Risks and Mitigations

* **Risk 1 (Unit test interference from host `%APPDATA%` profile)**: On developer machines where `%APPDATA%\go-teams\profile` contains a live Teams session, an overly broad fallback in `resolveExpandedProfileDir` could redirect a unit test's `filepath.Join(t.TempDir(), "go-teams", "profile")` to the developer's real `%APPDATA%\go-teams\profile`.
  * **Mitigation**: Restrict the cross-root (`LOCALAPPDATA` <-> `APPDATA` / `os.UserConfigDir()`) fallback in `resolveExpandedProfileDir` to paths whose cleaned parent matches `<LOCALAPPDATA|APPDATA|UserConfigDir>/<go-teams|teams-cli>/profile` and where neither `expanded` nor the candidate path is inside a repository root (`!isPathInsideRepoRoot`). Arbitrary `t.TempDir()` paths (under `AppData\Local\Temp\...`) never trigger cross-root fallback.
* **Risk 2 (Existing unit tests expecting `120000ms` deadline and `3` fetch attempts)**: Existing `recap_test.go` subtests (`U1`, `U1-R2`, `U3-R2`, `U1-R4`) set `maxRecapAudioDurationSec = 120` and advance `fakeNow += 125000`.
  * **Mitigation**: Gate the `240000ms` (`4m`) deadline and `5`-attempt retry schedule on `maxRecapAudioDurationSec > 300` (production sets `maxRecapAudioDurationSec = 4200`), keeping `120000ms` and `3` attempts (`250ms, 500ms`) when `maxRecapAudioDurationSec <= 300`.
