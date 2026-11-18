---
title: Cold SharePoint transcode Path A deadline precedence and %APPDATA% profile fallback
description: Distinguishing in-flight Path A deadline expiration from pre-pause MSE buffers, scaling cold transcode concurrency/retries, and resolving %APPDATA% profiles
source: docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md
doc_type: learning
problem_type: runtime-errors
category: runtime-errors
component: internal/browser
root_cause: In-flight Path A deadline expiry fell through to partial pre-pause MSE buffers (~12s) and triggered an incomplete MSE error, while resolveExpandedProfileDir ignored %APPDATA% sibling sessions when %LOCALAPPDATA% was cold
resolution_type: code_fix
severity: high
message: "recap audio MSE stream incomplete before deadline"
file_path: internal/browser/recap.go
citations:
  - docs/closure/035-S-035-F-post-merge-closure.md
  - docs/decisions/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-deliberation.md
  - docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-decided-plan.md
tags:
  - audio
  - recap
  - sharepoint
  - onedrive-transcode
  - appdata
  - profile-dir
---

## Problem

Running `go-teams` against a cold 60.6-minute (`605`-segment) SharePoint Meeting Recap failed in two places:

1. `Error: recap audio MSE stream incomplete before deadline` when `runPathA` was still fetching uncached `oneDrive.transcode` segments at `deadlineAt`.
2. `ErrAccountRequired` when the authenticated Teams profile lived under `%APPDATA%\go-teams\profile` (or `%APPDATA%\teams-cli\profile`) instead of `config.DefaultConfig()`'s `%LOCALAPPDATA%\go-teams\profile`.

## Root Cause

* Before `runPathA` pauses `<video>`, `streamembed.aspx` appends `mseInitSegment` and 1–2 initial `mseSegments` (~12s). When the 2-minute `deadlineAt` expired with `pathAPromise` still running, `initSegment` and `orderedSegments` were non-empty, so execution fell through into Path B completeness verification (`12s < 3637s`) and threw `'recap audio MSE stream incomplete before deadline'`. Because `hasStreamEmbedTarget` was true, `captureRecapAudioFromTargets` did not retry, and `triggerPlaybackIfReady()` gated both `btn.click()` and `m.play()` on a single `playTriggered` boolean.
* `resolveExpandedProfileDir` checked only the expanded path and its same-parent `teams-cli` sibling, never inspecting `%APPDATA%` / `os.UserConfigDir()` when `%LOCALAPPDATA%` had no persisted Teams session.

## Resolution

* In `extractRecapAudioJS` (`internal/browser/recap.go`): marked HTTP `408` retryable alongside `429`/`5xx`, scaled `fetchBytesWithCap` to 5 attempts (`250ms/500ms/1000ms/2000ms`) and `deadlineAt` to `240000ms` (`4m`) when `maxRecapAudioDurationSec > 300`, increased `runPathA` concurrency from `8` to `12` workers, split `btnClicked` and `mediaPlayTriggered` in `triggerPlaybackIfReady()`, and threw `'recap audio Path A capture deadline exceeded'` when `pathAPromise && !pathAFailed && !pathAResult` before falling back to `mseInitSegment` / `mseSegments`.
* In `captureRecapAudioFromTargets`: increased `recapAudioEvalTimeout` to `10 * time.Minute` enforced as an operation-wide `captureCtx` deadline, added a single attempt-0 retry on `'Path A capture deadline exceeded'` or `'MSE stream incomplete before deadline'`, broke out of `orderedTargets` immediately on those errors so parent frames do not duplicate work in the same attempt, and preserved `ctx.Err()` / `errors.Join(lastErr, captureCtx.Err())` on cancellation or timeout.
* In `resolveExpandedProfileDir`: added canonical `<APPDATA|UserConfigDir|LOCALAPPDATA>/<go-teams|teams-cli>/profile` cross-root fallback gated by `!isPathInsideRepoRoot` (with a home-directory `.git` dotfiles carve-out) so arbitrary `t.TempDir()`, custom `--profile-dir`, and lexical repo-local paths are not redirected (symlink/junction resolution across ancestors is tracked as follow-up `CDCE76CE` / `U1`).

## Prevention

* In Node.js `eval` unit tests that clamp `setTimeout` (`Math.min(ms, 5)`), never clamp `ms >= 10000` (such as `fetchBytesOnce`'s 15s `reqTimer`), and attach a `beforeExit` completion sentinel so unhandled dangling promises cannot exit `0` silently.
* Bound multi-target CDP retry loops with an operation-wide `context.WithTimeout` and check `ctx.Err()` before falling back to retained `lastErr`.
