---
title: Decided Plan — Recap audio cold SharePoint transcode resilience and %APPDATA% profile fallback (035-S)
description: Consolidated decisions, invariants, and scope for 035-F / 035-S (archived verbose plan preserved under docs/archive/plans/2026-11-17/)
date: 2026-11-17
deliberation_id: 024-DL
feature: 035-F
shipment: 035-S
source_stash:
  - 6E17E6EA
status: decided
archived_plan: docs/archive/plans/2026-11-17/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-plan.md
---

## Decided Architecture

1. **In-page audio capture hardening (`035.001-T`, `internal/browser/recap.go`)**:
   * In `extractRecapAudioJS`: mark HTTP `408` retryable alongside `429`/`5xx` in `fetchBytesOnce`; scale `fetchBytesWithCap` to `5` attempts (`250ms, 500ms, 1000ms, 2000ms`) when `maxRecapAudioDurationSec > 300` (or explicit `recapAudioFetchMaxAttempts` override); increase `runPathA` worker concurrency from `8` to `12`; track `btnClicked` and `mediaPlayTriggered` separately in `triggerPlaybackIfReady()`; scale `deadlineAt` to `240000ms` (`4m`) when `maxRecapAudioDurationSec > 300` (or explicit `recapAudioCaptureDeadlineMs` override); and throw `'recap audio Path A capture deadline exceeded'` before inspecting `mseInitSegment` / `mseSegments` when `pathAPromise && !pathAFailed && !pathAResult`.
2. **CDP evaluation timeout and bounded retry (`035.001-T`, `internal/browser/recap.go`)**:
   * Increase `recapAudioEvalTimeout` to `10 * time.Minute` enforced as an operation-wide `captureCtx` deadline across target discovery, evaluation, chunked transfer, and retry wait.
   * Allow a single `attempt == 0` retry on `'Path A capture deadline exceeded'` or `'MSE stream incomplete before deadline'` (breaking out of `orderedTargets` on stream-deadline errors so parent frames do not repeat capture within the same attempt, and preserving `ctx.Err()` / `errors.Join(lastErr, captureCtx.Err())`).
3. **Scoped `%APPDATA%` / `os.UserConfigDir()` profile fallback (`035.001-T`, `internal/browser/recap.go`)**:
   * In `resolveExpandedProfileDir`: when a canonical `<APPDATA|UserConfigDir|LOCALAPPDATA>/<go-teams|teams-cli>/profile` path outside a repository root has no persisted Teams session on disk (nor its same-parent `teams-cli` sibling), fall back across `userConfigProfileBases()` (`APPDATA`, `os.UserConfigDir()`, `LOCALAPPDATA`) to the first non-repo directory with a persisted session.
4. **Tests and documentation (`035.002-T`, `035.003-T`)**:
   * Add unit tests in `internal/browser/recap_test.go` and update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md`.

## Decisions, Rationale, and Rejected Alternatives

* **Gate 4-minute `deadlineAt` and 5-attempt retries on `maxRecapAudioDurationSec > 300` (plus explicit overrides)**: Gives production 60.6-minute (`3637.76s`, `605`-segment) cold SharePoint captures (within the 70-minute `maxRecapAudioDurationSec = 4200` cap) a 4-minute deadline and 3.75s of per-segment backoff while preserving deterministic short-cap unit tests (`maxRecapAudioDurationSec = 120`) unchanged.
* **Prioritize in-flight `pathAPromise` deadline error over pre-pause MSE segments**: Prevents 1–2 pre-pause MSE buffers (~12s) from masquerading as a truncated Path B stream (`'recap audio MSE stream incomplete before deadline'`).
* **Restrict cross-root profile fallback to canonical per-user config paths outside repo roots**: Enables `--queue` / `DefaultConfig()` runs to discover `%APPDATA%\go-teams\profile` sessions while keeping `t.TempDir()`, custom `--profile-dir`, and repo-local paths hermetic.

## Protected Invariants

1. **Same-origin HTTPS transcode containment and recording identity verification**: `isAllowedSameOriginTranscodeURL`, `allowlistedHeaderNames`, and `checkIdentityAgainst` remain enforced; `track === 'video'` never seeds audio template/timing fields.
2. **In-flight Path A deadline precedence over partial pre-pause MSE buffers**: `pathAPromise && !pathAFailed && !pathAResult` throws `'recap audio Path A capture deadline exceeded'` before Path B MSE inspection.
3. **No competing `<video>.play()` while Path A is active**: `triggerPlaybackIfReady()` checks `pathAActive`.
4. **Scoped cross-root profile discovery**: Warm `expanded` / same-parent `legacy`, custom `--profile-dir` paths, and lexical paths inside repo roots are not redirected (with an `os.UserHomeDir()` `.git` dotfiles carve-out; symlink/junction ancestor resolution is tracked as follow-up `CDCE76CE`).
5. **Bounded attempt-0 stream deadline retry**: At most 1 retry (`attempt == 0`) bounded by `recapAudioEvalTimeout = 10m` and `ctx.Done()`.

## Scope Boundaries, Risks, and Rollback Triggers

* **Out-of-scope boundary**: Zero changes to `go.mod`/`go.sum` or `config.DefaultConfig()`; Path B-only MSE state persistence / player reload across CDP retry attempts deferred under P-021 C2 (`B98262C8`).
* **Rollback trigger & procedure**: Any regression in Recap audio extraction, profile resolution, or CI quality gates triggers `git revert -m 1 <merge-sha>` on `main`.
