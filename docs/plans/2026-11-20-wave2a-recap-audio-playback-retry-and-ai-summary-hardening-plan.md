---
title: "Wave 2A Implementation Plan — SharePoint Recap Audio Playback, Retry, Identity Decoding, and Past-Recap AI Summary Hardening"
description: "Implementation plan and hardening specification for Wave 2A shipment 036-S (feature 036-F, deliberation 027-DL)"
doc_type: "plan"
source: "docs/decisions/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-deliberation.md"
plan_id: "2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening"
revision: 2
deliberation_id: "027-DL"
feature: "036-F"
shipment: "036-S"
date: "2026-11-20"
status: "draft"
tags:
  - "recap-audio"
  - "sharepoint-stream"
  - "ai-summary"
  - "url-identity"
  - "wave-2a"
review_manifest: docs/reviews/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan-review.md
source_history:
  - "review attempt 1: FAIL (plan revision 1) -\u003e docs/reviews/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan-review.md"
  - "review attempt 2: PASS (plan revision 2) -\u003e docs/reviews/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan-review.md"
---

## Problem Frame

Accepted deliberation
[2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-deliberation.md](../decisions/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-deliberation.md)
(`027-DL`) consolidates seven Wave 2A stash entries (`47D1E1C0`, `196AB9E4`,
`69AB7ACC`, `0DE2E900`, `39450846`, `7883544E`, `BA069506`) against post-`035-S`
`HEAD` (`ea651c1`):

1. **In-page SharePoint stream playback, candidate skip, and Path A memory cap
   (`internal/browser/recap.go`, `extractRecapAudioJS`)**:
   * `triggerPlaybackIfReady()` (`recap.go:1658-1692`) clicks the
     `'Play video'` button only once at `t=0` (`btnClicked = true`) and tracks a
     sticky `mediaPlayTriggered` boolean, failing to start playback when
     `CaptureAudio` begins before `streamembed.aspx` / `stream.aspx` finishes
     mounting its SharePoint player/token or replaces its `<video>` element
     (`47D1E1C0` 1a).
   * The direct-fetch loop (`recap.go:357-436`) still issues non-`blob:` HTTP
     fetches when `info.isDownloadBlocked && streamSurface` is true, wasting
     network round-trips on guaranteed-403/HTML responses (`47D1E1C0` 1b).
   * `fetchBytesOnce` (`recap.go:1003-1135`) increments `streamSourceBytes` only
     after an entire segment completes, allowing up to 12 concurrent Path A
     workers (`035-S` at `recap.go:1303`, superseding `U4-R3`'s 8-worker figure)
     to buffer chunks beyond `maxRecapAudioSourceBytes` (`256 MiB`)
     simultaneously (`7883544E` `U1-R5`). `U2-R3` (`recap.go:1797-1800`) is
     already resolved on `HEAD` and must be preserved.
2. **CDP target retry, non-stream target skip, delayed-mount discovery, and
   error precedence (`internal/browser/recap.go`,
   `captureRecapAudioFromTargets`)**:
   * When a mounted `streamembed.aspx` or `stream.aspx` target
     (`hasStreamEmbedTarget == true`, `recap.go:2010-2022`) fails on attempt `0`
     with `isDownloadBlocked=true`, `shouldRetry` (`recap.go:2249-2254`) refuses
     to retry, and the inner `orderedTargets` loop (`recap.go:2038-2247`)
     continues evaluating `xplatplugins.aspx` and outer `/v2/` targets in the
     same attempt (`47D1E1C0` 2).
   * `hasStreamEmbedTarget` currently uses raw URL substring matching before
     filtering out `isAuthRedirectURL` (`recap.go:2046`) or identity-conflicting
     targets (`recap.go:2049`), and when `!hasStreamEmbedTarget` and `lastErr ==
     nil` (outer page returns `""`), `shouldRetry` gates discovery retries on
     `hasXplatTarget`, failing to poll direct delayed `streamembed.aspx` mounts
     without `xplatplugins.aspx` (`39450846` `U1-R3`, `7883544E` `U2-R5`).
   * `U3-R3` (`recap.go:2178-2181`) and `U6-R3` (`errPriority`,
     `recap.go:1942-1955`) are implemented on `HEAD`; `U6-R3` requires explicit
     cross-attempt retry unit tests alongside `47D1E1C0` (2).
3. **Single-pass scheme-less/server-relative `%` identity decoding, Copilot
   fresh-message container guard, and `canonicalStreamTargetMatch` /
   `recapItemID` characterization coverage (`internal/browser/recap.go`,
   `internal/browser/recap_test.go`)**:
   * `applyQueryIdentity` (`recap.go:2342-2381`) and `checkIdentityAgainst`
     (`recap.go:90-124`) perform case-sensitive query key lookups and apply
     single-pass handling only to `id`, leaving scheme-less / server-relative
     `fileUrl` paths (`/Recordings/foo%20bar.mp4`,
     `Recordings/100% Sync.mp4?web=1`) double-unescaping literal `%` sequences in
     JS and failing `url.Parse` on `% ` in Go (`69AB7ACC` / `F-4b`).
   * `copyAISummaryJS` (`recap.go:2499-2556`) filters `freshCopyBtns` via
     `!(msg && msg.dataset && msg.dataset.goTeamsPreexisting === 'true')`, which
     evaluates to `true` if `msg` is `null` or inside a user prompt bubble
     (`.fui-ChatMyMessage` / `[class*="ChatMyMessage"]`) (`196AB9E4` / `F-1b`).
   * `canonicalStreamTargetMatch` (`meeting_chrome.go:440-492`) and
     `extractRecapFromTargets` (`meeting_chrome.go:1080`) already satisfy
     `0DE2E900` (`e25599a`) and `U5-R3` (`ed092e5`) in production code;
     `0DE2E900` requires direct characterization tests for case-variant
     `id`/`ID`, `uniqueId`/`UniqueId`, `driveId`/`DRIVEID`, ambiguous duplicate
     keys, and missing target parameters in `recap_test.go`.
4. **Decoupling `ai_summary_delay: "0s"` from `--no-ai-summary` across
   `internal/config` and `internal/cli` (`47D1E1C0` 3)**:
   * `isAISummaryDisabled` (`internal/cli/capture.go:73-86`) treats
     `AISummaryDelay == "0s"` as disabling AI Summary in `past-recap` and `auto`
     Recap exports because `Config` lacks a dedicated runtime `NoAISummary`
     boolean, and `effectiveAISummaryDelay` (`capture.go:88-107`) relies on
     callers zeroing `AISummaryDelay` rather than checking `NoAISummary`
     directly.
5. **Documentation qualification and Wave 2A alignment (`BA069506`)**:
   * `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` must durably qualify that
     `PerformanceResourceTiming` inspects window-visible resource entries (rather
     than Worker-isolated timelines requiring dedicated Worker CDP
     instrumentation) and document the Wave 2A audio/retry/cap, `--queue`,
     `schedule` precedence, and `ai_summary_delay: "0s"` vs `--no-ai-summary`
     semantics.

### Explicit Scope Boundaries

* **In scope**: Covering feature `036-F`, shipment `036-S` (`priority:
  critical`), deliberation `027-DL`, and tasks `036.001-T`..`036.009-T` covering
  the 7 Wave 2A stash entries (`47D1E1C0`, `196AB9E4`, `69AB7ACC`, `0DE2E900`,
  `39450846`, `7883544E`, `BA069506`).
* **Out of scope**:
  * Queued Wave 1 shipment `010-S` (`010-F`, `010.001-T`..`010.009-T`, `001-DL`,
    `025-DL`, `026-DL`).
  * Wave 2B stash entries (`E03CB3C3`, `4C40AF2B`, `B98262C8`) and all other
    non-Wave-2A stash entries.
  * Pre-existing foreign dirty/untracked paths (`.github/*`, `start.ps1`,
    `docs/memory/2026-11-18/*`, `.gsd/carry-forward.yaml`) under `P-011`.

## Constitution Check

| Principle | Compliance | Notes |
|---|---|---|
| I. Safety-First Go | PASS | Pure Go 1.26; zero `unsafe`; explicit error handling; non-parallel `t.Cleanup` restoration on `recapAudioRetryDelay` under `-race`; `golangci-lint run`, `go vet ./...`, and `gofmt -l .` enforced on every Go unit. |
| II. Test-First Development (`P-002`/`P-004`) | PASS | Every Go code task (`U1`..`U8`) declares `harness-surface:harness-architect` with its colocated `*_test.go` file in budget for expected-RED -> GREEN verification; `U9` includes complete `P-002` Rule 2–4 verification tables (`harness-surface:none`, `harness-verification-gated` applied only by `harness-architect`). |
| III. Workspace Isolation & Security Boundaries | PASS | Same-origin HTTPS transcode containment, case-insensitive single-pass recording identity checks, and zero live URL/UPN/token leaks (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) preserved. |
| IV. CLI Workspace Containment | PASS | All edits stay strictly inside `internal/{browser,config,cli}` and `docs/`. |
| V. Structured Observability | PASS | Existing `slog` structured diagnostics, `%w`/`errors.Join` wrapping, manual-observation closure checklist, Documentation Consistency Audit outcome, and `P-020` post-merge compaction enforced. |
| VI. Single Responsibility | PASS | Zero new external dependencies (`go.mod` and `go.sum` untouched). |
| VII. Destructive Command Approval | PASS | Zero destructive file, git, or config mutations; Stage stash archival uses canonical non-destructive `backlogit` archival. |
| VIII. Explicit Safety Modes | PASS | Freeze-scope (`P-021`) and Careful mode (`## Plan Hardening` with `ProposedAction` inventory including `Approval Required` and conditional `PA-036-7` checkpoint) active. |
| IX. Git-Friendly Persistence | PASS | LF line endings enforced across all Markdown and backlog artifacts; durable `docs/` prose avoids embedding transient stash hashes. |
| X. Agent Context Efficiency | PASS | Query-driven `backlogit` operations, exact `HEAD` line anchors, and targeted 2-file task budgets. |
| XI. Merge Commit History Preservation (`P-009`) | PASS | Downstream staging and feature PRs merge exclusively via `--merge` two-parent commits. |
| Task Granularity (2-Hour Rule & Width Isolation) | PASS | Every implementation unit (`U1`..`U9`) touches strictly `2` files (`< 3` files), `1–4` functions (`< 5` functions, counting inner JS functions), and `2–3` test scenarios (`< 4` scenarios) within a single domain/package. |

## Requirements Trace

| Stash / Decision Ref | Requirement | Implementation Unit(s) | Verification |
|---|---|---|---|
| `47D1E1C0` (1a, 1b), `D1`, `D2`, `AC1` | Periodically re-click `'Play video'` every `1000ms` while `!activityObserved && !pathAActive` (preserving mute/16x and replacement `<video>` play safety) and skip direct non-blob `candidateUrl` fetches when `info.isDownloadBlocked && streamSurface` after identity & duration checks | `U1` (`036.001-T`) | `go test -race "-run=TestWave2ARecapAudioEarlyMountPlayAndBlockedSkip|TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked" ./internal/browser/...` |
| `7883544E` (`U1-R5`), `39450846` (`U2-R3`, `U4-R3`), `D4`, `AC1` | Reserve per-worker `inFlightSourceBytes` against `maxRecapAudioSourceBytes` across all 12 `runPathA` workers (`035-S`) with synchronous `fatalErr`/`abortCtrl.abort()` before `reader.cancel()` and `finally` release on retry/abort; preserve `U2-R3` zero-duration rejection | `U2` (`036.002-T`) | `go test -race "-run=TestWave2APathAInFlightByteReservation|TestRecapAudioColdTranscodeResilienceAndDeadlineRetry|TestRecapJSStreamEmbedHighSpeedAudioCaptureWhenDownloadBlocked" ./internal/browser/...` |
| `47D1E1C0` (2), `7883544E` (`U2-R5`), `39450846` (`U1-R3`, `U3-R3`, `U6-R3`), `D3`, `AC2` | Path-based `isStreamTarget`/`hasStreamEmbedTarget`, `skipNonStreamTargets` on retryable stream errors, `sawRetryableStreamTargetErr` attempt-0 retry, `!hasStreamEmbedTarget && lastErr == nil` delayed-mount polling without `hasXplatTarget`, fast 8-attempt exhaustion in `recap_retry_test.go`, and `errPriority` + `U3-R3` verification | `U3` (`036.003-T`) | `go test -race "-run=TestWave2ACaptureRecapAudioRetryBreakAndDiscovery|TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable|TestRecapRound4ReviewFixes|TestCaptureRecapAudio" ./internal/browser/...` |
| `69AB7ACC` (`F-4b`), `196AB9E4` (`F-1b`), `0DE2E900`, `39450846` (`U5-R3`), `D5`, `AC3` | Case-insensitive query key matching + single-pass scheme-less/server-relative `id` and `fileUrl` `%` decoding in Go & JS, `Boolean(msg)` + non-`.fui-ChatMyMessage`/non-`.ms-List` guards in `copyAISummaryJS`, and `canonicalStreamTargetMatch` + `U5-R3` characterization coverage | `U4` (`036.004-T`) | `go test -race "-run=TestWave2AIdentitySinglePassAndCopilotGuard|TestWave2ACanonicalStreamTargetMatchCharacterization|TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity" ./internal/browser/...` |
| `47D1E1C0` (3), `D6`, `AC4` | Expand `config.Config` with runtime `NoAISummary bool` (`yaml:"-"`) without YAML persistence | `U5` (`036.005-T`) | `go test -race "-run=TestConfigNoAISummaryRuntimeOnly" ./internal/config/...` |
| `47D1E1C0` (3), `D6`, `AC4` | Migrate producers `applyCaptureFlags` and `inheritedQueueFlags` (`--no-ai-summary` / `--no-ai-summary=false`) and update `captureTimingHelp` / flag description in `join.go` | `U6` (`036.006-T`) | `go test -race "-run=TestCaptureFlagsAndQueueForwardNoAISummary|TestAllSubcommandsProvideComprehensiveHelpAndExamples" ./internal/cli/...` |
| `47D1E1C0` (3), `D6`, `AC4` | Migrate producers `applyScheduleFlags` and `buildScheduleEntryConfig` (`NoAISummary = false` on `*e.AISummaryDelay > 0`) and align `bindScheduleFlags` `--no-ai-summary` description in `schedule.go` | `U7` (`036.007-T`) | `go test -race "-run=TestScheduleNoAISummaryAndPerMeetingDelayPrecedence|TestScheduleMeetingOverridesAndBusyGateReleaseBeforeSummaryDelay" ./internal/cli/...` |
| `47D1E1C0` (3), `D6`, `AC4` | Contract consumers `isAISummaryDisabled`, `effectiveAISummaryDelay` (`if isRecap \|\| cfg.NoAISummary { return 0 }`), and `runAppendAISummary` across `past-recap`, `auto`, `live-with-summary`, and `ephemeral-live` | `U8` (`036.008-T`) | `go test -race "-run=TestDefaultNewMeetingAndNewSessionWireAISummaryFlags|TestIsAISummaryDisabledAndEffectiveDelayAcrossModes" ./internal/cli/...` |
| `BA069506`, `47D1E1C0`, `7883544E`, `AC5` | Durably qualify `PerformanceResourceTiming` window-visible scope and document Wave 2A audio/retry/cap, `--queue`, `schedule` precedence, and `ai_summary_delay: 0s` vs `--no-ai-summary` behavior in `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` | `U9` (`036.009-T`) | `markdownlint --disable MD013 "--" docs/ARCHITECTURE.md docs/USER_GUIDE.md`, `gsd gate line-endings --path docs/ARCHITECTURE.md --json`, and `gsd gate line-endings --path docs/USER_GUIDE.md --json` |

## Implementation Units

### U1 (`036.001-T`, Browser Track Root) — Harden `extractRecapAudioJS` early-mount Play re-clicking and skip blocked non-blob candidate fetches

* **Domain**: code (`internal/browser`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/browser/recap.go`
  * `internal/browser/recap_test.go`
* **Functions Changed (`<= 4` total, `< 5`)**: `extractRecapAudioJS` outer `candidates` loop (`recap.go:357-373`), `captureStreamEmbedAudio` playback state + `triggerPlaybackIfReady` (`recap.go:1658-1692`), and `TestWave2ARecapAudioEarlyMountPlayAndBlockedSkip` in `recap_test.go`.
* **Changes Needed**:
  1. **Skip direct non-blob `candidateUrl` fetches when `info.isDownloadBlocked && streamSurface`** (`recap.go:357-373`):
     * Inside `for (const candidateUrl of candidates)`, keep `checkIdentityAgainst(candidateUrl, pageUrl, candState)`, the `enforceIdentity` check, and the pre-fetch `duration` limit loop (`media.duration`, `info.duration`, `info.durationSeconds`) intact first so Protected Invariants 2 and 3 still fire before skipping.
     * Immediately before `if (!fetchFn)`, add:
       `if (info.isDownloadBlocked && streamSurface && !/^blob:/i.test(candidateUrl)) { blockedErr = new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)'); lastFetchErr = blockedErr; continue; }`
       so non-`blob:` direct URLs (`info.downloadUrl`, `info.mediaUrl`, `media.src`, etc.) never trigger wasted HTTP 403/HTML fetches on `streamembed.aspx`/`stream.aspx` while `blob:` candidates and `!streamSurface` pages preserve existing behavior.
  2. **Periodic `1000ms` `'Play video'` button re-clicking + replacement `<video>` mute/play safety** (`recap.go:1658-1692`):
     * Replace `let btnClicked = false; let mediaPlayTriggered = false;` in `captureStreamEmbedAudio()` with `let lastBtnClickAt = 0; let triggeredMediaEl = null;`.
     * In `triggerPlaybackIfReady()`: return early if `pathAActive`; whenever `m && !pathAActive`, always enforce `m.muted = true; m.volume = 0; if (m.playbackRate !== 16) m.playbackRate = 16;` (even on replacement `<video>` elements); click `btn` whenever `!activityObserved && btn && typeof btn.click === 'function' && (lastBtnClickAt === 0 || Date.now() - lastBtnClickAt >= 1000)`, recording `lastBtnClickAt = Math.max(1, Date.now())`; and trigger `m.play()` once per `<video>` element (`if (m && m !== triggeredMediaEl && !pathAActive && typeof m.play === 'function') { triggeredMediaEl = m; try { Promise.resolve(m.play()).catch(() => {}); } catch (_) {} }`).
     * In the `while (Date.now() < deadlineAt)` loop (`recap.go:1691`), invoke `if (!pathAActive && (!activityObserved || !triggeredMediaEl || (playerDoc.querySelector && playerDoc.querySelector('audio, video') !== triggeredMediaEl))) triggerPlaybackIfReady();`.
  3. **Unit Tests (`internal/browser/recap_test.go`, 2 scenarios)**:
     * Add `TestWave2ARecapAudioEarlyMountPlayAndBlockedSkip`:
       (a) delayed player initialization + `<video>` element replacement where clicks at `t=0` and `t=1000ms` occur before token readiness, a replacement `<video>` mounts muted at `16x` and receives `.play()` once, and the click at `t=2000ms` starts playback and halts further button clicks once `activityObserved` becomes `true`;
       (b) `info.isDownloadBlocked && streamSurface` with non-blob `info.downloadUrl`/`media.src` never invokes `fetch` on those non-blob candidates while still rejecting conflicting candidate identities.
     * **Node.js test harness invariants (mandatory)**: Never clamp `setTimeout` delays with `ms >= 10000` (`fetchBytesOnce`'s 15s `reqTimer`), coordinate mocked `Date.now()` monotonically with `1000ms` `lastBtnClickAt` intervals, and attach a `process.on('beforeExit')` completion sentinel.
* **Dependencies**: none.

### U2 (`036.002-T`) — Reserve per-worker `inFlightSourceBytes` against `maxRecapAudioSourceBytes` in Path A

* **Domain**: code (`internal/browser`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: S` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/browser/recap.go`
  * `internal/browser/recap_test.go`
* **Functions Changed (`4` total, `< 5`)**: `canReserveSourceBytes` / `recordMSESegment` (`recap.go:653,773`), `maybeStartPathA` (`recap.go:970`), `fetchBytesOnce` (`recap.go:1003-1135`), and `TestWave2APathAInFlightByteReservation` in `recap_test.go` (preserving 12-worker `runPathA` at `recap.go:1303` per `035-S` / `U4-R3` and `U2-R3` at `recap.go:1797-1800`).
* **Changes Needed**:
  1. **Shared pre-allocation cap helper (`recap.go:653,773,986`)**:
     * Declare `let inFlightSourceBytes = 0;` alongside `let streamSourceBytes = 0;` in `captureStreamEmbedAudio()` with helper `function exceedsSourceCap(extraBytes) { return streamSourceBytes + inFlightSourceBytes + extraBytes > maxRecapAudioSourceBytes; }`.
     * Use `exceedsSourceCap(u8.byteLength)` in `recordMSESegment` (`recap.go:778,805`), and reset `inFlightSourceBytes = 0;` alongside `streamSourceBytes = mseBytesTotal();` in `maybeStartPathA().catch` (`recap.go:986`).
  2. **Per-worker `inFlightSourceBytes` reservation and synchronous abort in `fetchBytesOnce` (`recap.go:1003-1135`)**:
     * Track `let workerReserved = 0;` inside `fetchBytesOnce` with unconditional `finally` cleanup `if (workerReserved > 0) { inFlightSourceBytes = Math.max(0, inFlightSourceBytes - workerReserved); workerReserved = 0; }`.
     * When `Content-Length` `cl` is finite and `> 0`, if `exceedsSourceCap(cl)`, set `fatalErr = new Error(limitError); if (abortCtrl) abortCtrl.abort(); throw fatalErr;`, otherwise reserve `inFlightSourceBytes += cl; workerReserved += cl;`.
     * On `!resp.body || !resp.body.getReader`, compute `const extra = Math.max(0, u8.byteLength - workerReserved);`; if `exceedsSourceCap(extra)`, set `fatalErr = new Error(limitError); if (abortCtrl) abortCtrl.abort(); throw fatalErr;`, otherwise release `workerReserved` (`inFlightSourceBytes = Math.max(0, inFlightSourceBytes - workerReserved); workerReserved = 0;`) and commit `streamSourceBytes += u8.byteLength`.
     * On streaming `resp.body.getReader()`, before pushing each chunk `v`, compute `const extra = Math.max(0, (total + v.byteLength) - workerReserved);`; if `extra > 0`:
       * If `exceedsSourceCap(extra)`: **set `fatalErr = new Error(limitError)` and call `if (abortCtrl) abortCtrl.abort()` synchronously FIRST**, and only then `try { await reader.cancel(); } catch (_) {}` before `throw fatalErr` (so sibling Path A workers see `fatalErr` and `abortCtrl.signal.aborted` immediately with zero microtask yield window).
       * Otherwise reserve `inFlightSourceBytes += extra; workerReserved += extra;`.
     * When `part.done` is reached, release `workerReserved` (`inFlightSourceBytes = Math.max(0, inFlightSourceBytes - workerReserved); workerReserved = 0;`), verify `!exceedsSourceCap(total)`, and commit `streamSourceBytes += total;`.
  3. **Unit Tests (`internal/browser/recap_test.go`, 2 scenarios)**:
     * Add `TestWave2APathAInFlightByteReservation`:
       (a) concurrent Path A streaming workers without `Content-Length` fail closed with `limitError` synchronously as soon as aggregate `streamSourceBytes + inFlightSourceBytes` exceeds `maxRecapAudioSourceBytes` before any worker completes;
       (b) retryable HTTP `408`/`503` and mid-stream error attempts release `workerReserved` cleanly across 5-attempt retries (covering both `Content-Length > actual` and `Content-Length` absent) without false cap exhaustion.
     * **Node.js test harness invariants (mandatory)**: Never clamp `setTimeout` delays with `ms >= 10000` (`fetchBytesOnce`'s 15s `reqTimer`), and attach a `process.on('beforeExit')` completion sentinel.
* **Dependencies**: `blocks` on `U1` (`036.001-T`).

### U3 (`036.003-T`) — Harden `captureRecapAudioFromTargets` path-based stream detection, attempt-0 retry, non-stream target skip, and delayed-mount polling

* **Domain**: code (`internal/browser`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: S` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/browser/recap.go`
  * `internal/browser/recap_retry_test.go`
* **Functions Changed (`4` total, `< 5`)**: `isStreamTarget` + `captureRecapAudioFromTargets` (`recap.go:1927-2265`), `TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable` (`recap_retry_test.go:788`), and `TestWave2ACaptureRecapAudioRetryBreakAndDiscovery` (`recap_retry_test.go`).
* **Changes Needed**:
  1. **Define package-level `var recapAudioRetryDelay = 1 * time.Second`** alongside `recapAudioEvalTimeout` (`recap.go:3637`) and use `time.NewTimer(recapAudioRetryDelay)` in `captureRecapAudioFromTargets` (`recap.go:2257`), keeping the existing `select` branches (`<-ctx.Done()` and `<-captureCtx.Done()`) with `timer.Stop()` and `errors.Join(lastErr, captureCtx.Err())` intact.
  2. **Path-based `isStreamTarget`, filtered `hasStreamEmbedTarget`, and `recapTargetRank`** (`recap.go:2010-2040`):
     * Define `isStreamTarget(rawURL string) bool` by parsing `u, err := url.Parse(strings.TrimSpace(rawURL))`, returning `false` when `err != nil || isAuthRedirectURL(rawURL)`, and checking `lowerPath := strings.ToLower(u.Path)` for `strings.Contains(lowerPath, "streamembed.aspx") || strings.Contains(lowerPath, "stream.aspx")` (ignoring query/fragment strings so `?fileUrl=.../stream.aspx` or SSO `?redirect_uri=.../streamembed.aspx` never misclassify as mounted stream surfaces).
     * Compute `hasStreamEmbedTarget` strictly over targets `t` satisfying `isTargetInCaptureScope(t, ownerID, frames) && !isAuthRedirectURL(t.URL) && !hasMeetingIdentityConflict(expected, ExtractMeetingIdentity(t.URL)) && isStreamTarget(t.URL)` (removing unused `hasXplatTarget`).
     * Update `recapTargetRank` so `t.Type == "iframe" && isStreamTarget(t.URL)` is rank `0`, `t.Type == "page" && isStreamTarget(t.URL)` (direct SharePoint `stream.aspx` page) is rank `1`, `t.Type == "iframe" && strings.Contains(strings.ToLower(t.URL), "xplatplugins.aspx")` is rank `2`, other `iframe` is rank `3`, and non-stream `page` is rank `4`.
  3. **Selective `skipNonStreamTargets` and `sawRetryableStreamTargetErr` retry reconciliation (`47D1E1C0` 2, `U1-R3`, `U2-R5`, `U6-R3`)** (`recap.go:2038-2256`):
     * Track per-attempt `skipNonStreamTargets := false` and `sawRetryableStreamTargetErr := false`.
     * Inside `for _, target := range orderedTargets`:
       * Check `ctx.Err()`, `captureCtx.Err()`, `if isAuthRedirectURL(target.URL) { continue }`, `if !isTargetInCaptureScope(target, ownerID, frames) { continue }`, and `if hasMeetingIdentityConflict(expected, ExtractMeetingIdentity(target.URL)) { setLastErr(fmt.Errorf("browser: recap audio recording identity mismatch")); continue }`.
       * Guard non-stream targets via `if skipNonStreamTargets && !isStreamTarget(target.URL) { continue }` so `xplatplugins.aspx` and outer `/v2/` targets are never evaluated in the same attempt after a stream failure, while any remaining non-conflicting `streamembed.aspx`/`stream.aspx` target still runs if an earlier sibling threw `identity mismatch`, `identity unverified`, or `-32000`.
       * Helper `onRetryableStreamFailure(targetURL, msg string)`: when `isStreamDeadlineMsg(msg) || (isStreamTarget(targetURL) && strings.Contains(msg, "isDownloadBlocked=true"))`, set `skipNonStreamTargets = true`, and when `attempt == 0`, set `sawRetryableStreamTargetErr = true`. Call `onRetryableStreamFailure` on both `evalErr != nil` (`recap.go:2056`) and `payload.Error != ""` (`recap.go:2104`) instead of an unconditional `break`.
     * Set `isDownloadBlockedErr := lastErr != nil && strings.Contains(lastErr.Error(), "isDownloadBlocked=true")` and `shouldRetry := attempt < maxAttempts-1 && (sawRetryableStreamTargetErr || (!hasStreamEmbedTarget && (lastErr == nil || isDownloadBlockedErr)))`.
  4. **Unit Tests (`internal/browser/recap_retry_test.go`, 3 scenarios, non-parallel with `t.Cleanup`)**:
     * In existing `TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable` (`recap_retry_test.go:788`), override `recapAudioRetryDelay = 5 * time.Millisecond` (restoring via `t.Cleanup`, without `t.Parallel()`) and assert `strings.Contains(err.Error(), "unavailable for requested recording")` so full 8-attempt exhaustion completes in ~35 ms without hitting `f.capture`'s 2s context timeout.
     * Add `TestWave2ACaptureRecapAudioRetryBreakAndDiscovery` (without `t.Parallel()`, restoring `recapAudioRetryDelay` via `t.Cleanup`) covering:
       (a) mounted `streamembed.aspx` / `stream.aspx` returning `isDownloadBlocked=true` on attempt `0` skips `xplatplugins.aspx` and `/v2/` (`parentEvals == 0`), retries on attempt `1` (even when a skipped conflicting sibling iframe set `identity mismatch` on attempt `0`), stops after attempt `1` (`2` evals) when `isDownloadBlocked=true` persists, and still evaluates a second valid stream target if a first bare stream iframe returned `identity unverified`;
       (b) direct delayed `streamembed.aspx` mount without `xplatplugins.aspx` (including when outer `owner.URL` has `?fileUrl=.../stream.aspx` or an SSO login iframe has `?redirect_uri=.../streamembed.aspx`) polls across attempts until `streamembed.aspx` mounts (`U1-R3` / `U2-R5`);
       (c) `errPriority` (`U6-R3`) preserves `Path A capture deadline exceeded` (`priority 2`) when attempt `1` returns `isDownloadBlocked=true` (`priority 1`), and upgrades `lastErr` from attempt-0 `isDownloadBlocked=true` to attempt-1 `Path A capture deadline exceeded` without entering attempt `2`.
* **Dependencies**: `blocks` on `U2` (`036.002-T`).

### U4 (`036.004-T`) — Normalize case-insensitive single-pass `fileUrl`/`id` percent-decoding, guard `copyAISummaryJS` containers, and add `canonicalStreamTargetMatch` characterization tests

* **Domain**: code (`internal/browser`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect` for `recap.go` changes; characterization assertions separated from expected-RED roster).
* **Size / Complexity / Label**: `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/browser/recap.go`
  * `internal/browser/recap_test.go`
* **Functions Changed (`4` total, `< 5`)**: `checkIdentityAgainst` (`recap.go:90-124`), `applyQueryIdentity` (`recap.go:2342-2381`), `copyAISummaryJS` (`recap.go:2499-2556`), and `TestWave2AIdentitySinglePassAndCopilotGuard` (+ characterization `TestWave2ACanonicalStreamTargetMatchCharacterization`) in `recap_test.go`.
* **Changes Needed**:
  1. **Case-insensitive query keys and single-pass scheme-less/server-relative `fileUrl` & `id` decoding (`69AB7ACC` / `F-4b`)**:
     * Define scheme-less non-protocol-relative path predicate: a trimmed value `v` that does not start with `//` or `:` and has no `^[A-Za-z][A-Za-z0-9+.-]*:` scheme prefix (covering both `/personal/.../100% Sync.mp4?web=1` and `Recordings/foo%20bar.mp4?web=1`).
     * In Go `applyQueryIdentity` (`recap.go:2342-2381`): look up `threadId`, `driveItemId`, `driveId`, `callId`, `iCalUid`, `fileUrl`, and `id` case-insensitively across `vals` (`strings.EqualFold`); for `"fileUrl"` and `"id"`, when `v` matches the scheme-less non-`//` predicate, strip any `?` query suffix (`if qIdx := strings.IndexByte(v, '?'); qIdx >= 0 { p = v[:qIdx] } else { p = v }`) without calling `url.Parse`, reject trailing `/` (`!strings.HasSuffix(p, "/")`), and extract `path.Base(p)` preserving literal `%` and `#`. Keep `url.Parse(v)` strictly for scheme-bearing URLs (`https://...`).
     * In JS `checkIdentityAgainst` (`recap.go:90-124`): iterate `params.entries()` matching keys case-insensitively (`keyLower === ...`); hoist `const rawVal = String(value || '').trim(); if (!rawVal) continue;` *before* the `paramKey === 'id'` media-extension regex check; when `rawVal` has no `^[a-z][a-z0-9+.-]*:` scheme prefix and does not start with `//` or `:`, set `alreadyDecoded = true` and strip any `?` query suffix (`const qIdx = rawVal.indexOf('?'); filePath = qIdx >= 0 ? rawVal.slice(0, qIdx) : rawVal;`), reserving `new URL(rawVal, baseUrl).pathname` + `safeDecodeURIComponent` strictly for scheme-bearing / protocol-relative URLs (still throwing `'recap audio recording identity mismatch'` when `paramKey === 'fileUrl'` fails to parse).
  2. **Tighten `copyAISummaryJS` Copilot container and prompt exclusions (`196AB9E4` / `F-1b`)** (`recap.go:2531-2556`):
     * When locating `recapBtn` (`recap.go:2533`), exclude elements inside transcript containers (`!(el && el.closest && el.closest('.ms-List, [data-tid*="transcript" i]'))`).
     * In `freshCopyBtns` (`recap.go:2546-2549`), require `Boolean(msg) && !(msg.dataset && msg.dataset.goTeamsPreexisting === 'true') && !(btn && btn.closest && btn.closest('.fui-ChatMyMessage, .fui-ChatMyMessage__body, [class*="ChatMyMessage"]'))`.
  3. **Unit & Characterization Tests (`internal/browser/recap_test.go`, 3 scenarios)**:
     * **Expected-RED roster (`TestWave2AIdentitySinglePassAndCopilotGuard`, targeting `recap.go` only)**:
       (a) Go `ExtractMeetingIdentity` and JS `checkIdentityAgainst` parity on case-variant (`?ID=`, `?FILEURL=`, `?DriveItemId=`), leading-slash (`/Recordings/foo%20bar.mp4`), and relative (`Recordings/100% Sync.mp4?web=1`) paths containing literal `%20`, literal `% `, literal `#`, trailing whitespace, and trailing `/` rejection, plus `https://` `fileUrl` single-layer decoding;
       (b) `copyAISummaryJS` ignoring uncontained Copy buttons (`closest('.fai-CopilotMessage') === null`), `.fui-ChatMyMessage` user bubbles, `.ms-List` transcript utterance buttons, and pre-existing `.fai-CopilotMessage` items until a new post-click `.fai-CopilotMessage` renders (leaving `setTimeout` `ms >= 10000` unclamped and attaching `process.on('beforeExit')`).
     * **Green characterization coverage (`TestWave2ACanonicalStreamTargetMatchCharacterization`, outside the expected-RED roster so `meeting_chrome.go` is never stubbed)**:
       (c) direct table-driven assertions on `canonicalStreamTargetMatch` (`0DE2E900`) covering uppercase `ID`, lowercase `uniqueId`, mixed-case `DriveId`/`DRIVEID`, ambiguous duplicate case-variant keys (`?uniqueId=a&UniqueId=b`), and missing target parameters, alongside `U5-R3` `recapItemID` verification.
* **Dependencies**: `blocks` on `U3` (`036.003-T`).

### U5 (`036.005-T`, Config/CLI Expand Root) — Add runtime `NoAISummary` field to `config.Config` without YAML persistence

* **Domain**: code (`internal/config`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: XS` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/config/config.go`
  * `internal/config/config_test.go`
* **Functions / Types Changed (`2` total, `< 5`)**: `Config` struct (`internal/config/config.go:113`) and `TestConfigNoAISummaryRuntimeOnly` (`internal/config/config_test.go`).
* **Changes Needed**:
  1. Add `// NoAISummary records an explicit --no-ai-summary CLI flag at runtime without YAML serialization.` and `NoAISummary bool` (`yaml:"-"`) on `config.Config` (`internal/config/config.go`, immediately after `ExplicitCaptureAudio bool` `yaml:"-"`).
  2. Add `TestConfigNoAISummaryRuntimeOnly` in `internal/config/config_test.go` verifying that `config.Load` on YAML configurations setting `ai_summary_delay: "0s"` across `capture_mode: "past-recap"`, `"auto"`, `"live-with-summary"`, and `"ephemeral-live"` leaves `cfg.NoAISummary == false`, and that `config.Save` / `yaml.Marshal` never persists `NoAISummary: true` into YAML on disk.
* **Dependencies**: none.

### U6 (`036.006-T`, Migrate Capture/Queue Producers) — Wire `--no-ai-summary` to `Config.NoAISummary` in `join.go`, queue forwarding, and help text

* **Domain**: code (`internal/cli`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: XS` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/cli/join.go`
  * `internal/cli/join_test.go`
* **Functions Changed (`4` total, `< 5`)**: `captureTimingHelp` / `bindCaptureFlags` (`join.go:37,132`), `applyCaptureFlags` (`join.go:220-222`), `inheritedQueueFlags` (`join.go:324-326`), and `TestCaptureFlagsAndQueueForwardNoAISummary` (`join_test.go`).
* **Changes Needed**:
  1. In `applyCaptureFlags` (`internal/cli/join.go:220-222`):
     * If `cmd != nil && cmd.Flags().Changed("no-ai-summary")`, set `cfg.NoAISummary = f.noAISummary` (and `if f.noAISummary { cfg.AISummaryDelay = "0s" }`); else if `f.noAISummary`, set `cfg.NoAISummary = true` and `cfg.AISummaryDelay = "0s"` (retaining `AISummaryDelay = "0s"` so intermediate builds prior to `U8` remain 100% compatible).
  2. In `inheritedQueueFlags` (`internal/cli/join.go:324-326`):
     * When `cmd != nil && cmd.Flags().Changed("no-ai-summary")`, append `"--no-ai-summary"` if `f.noAISummary` and `"--no-ai-summary=false"` otherwise so per-line queue overrides propagate deterministically.
  3. Update `captureTimingHelp` (`join.go:37`) and `bindCaptureFlags` (`join.go:132`) to state that `--no-ai-summary` disables AI Summary capture for both Meeting Recaps and delayed post-call enrichment (removing `(same as --ai-summary-delay=0s)` while preserving `"--no-ai-summary"` for `help_test.go`).
  4. Add producer wiring unit test `TestCaptureFlagsAndQueueForwardNoAISummary` in `internal/cli/join_test.go` asserting `Config.NoAISummary` population across `--no-ai-summary`, `--no-ai-summary=false`, `--ai-summary-delay 0s` (`NoAISummary == false`), and `inheritedQueueFlags` / `--queue` per-line overrides.
* **Dependencies**: `blocks` on `U5` (`036.005-T`).

### U7 (`036.007-T`, Migrate Schedule Producer) — Wire `--no-ai-summary`, per-meeting `ai_summary_delay` precedence, and flag help in `schedule.go`

* **Domain**: code (`internal/cli`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: XS` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/cli/schedule.go`
  * `internal/cli/schedule_test.go`
* **Functions Changed (`4` total, `< 5`)**: `bindScheduleFlags` (`schedule.go:54`), `applyScheduleFlags` (`schedule.go:88-90`), `buildScheduleEntryConfig` (`schedule.go:128-130`), and `TestScheduleNoAISummaryAndPerMeetingDelayPrecedence` (`schedule_test.go`).
* **Changes Needed**:
  1. Update `bindScheduleFlags` (`internal/cli/schedule.go:54`) `--no-ai-summary` description to `"disable AI Summary capture for both Meeting Recaps and post-call enrichment"` (matching `join.go:132`).
  2. In `applyScheduleFlags` (`internal/cli/schedule.go:88-90`): if `cmd != nil && cmd.Flags().Changed("no-ai-summary")`, set `cfg.NoAISummary = f.noAISummary` (and `if f.noAISummary { cfg.AISummaryDelay = "0s" }`); else if `f.noAISummary`, set `cfg.NoAISummary = true` and `cfg.AISummaryDelay = "0s"`.
  3. In `buildScheduleEntryConfig` (`internal/cli/schedule.go:128-130`): when `e.AISummaryDelay != nil`, set `entryCfg.AISummaryDelay = (*e.AISummaryDelay).String()`, and when `*e.AISummaryDelay > 0`, set `entryCfg.NoAISummary = false` so a positive per-meeting `ai_summary_delay` overrides a global `schedule --no-ai-summary` flag while `ai_summary_delay: 0s` without `--no-ai-summary` keeps `NoAISummary == false`.
  4. Add `TestScheduleNoAISummaryAndPerMeetingDelayPrecedence` in `internal/cli/schedule_test.go` verifying `NoAISummary` state across global `schedule --no-ai-summary`, positive per-meeting `ai_summary_delay` override (`NoAISummary == false`), and per-meeting `mode: past-recap` + `ai_summary_delay: 0s` without `--no-ai-summary` (`NoAISummary == false`).
* **Dependencies**: `blocks` on `U6` (`036.006-T`).

### U8 (`036.008-T`, Contract CLI Consumers) — Decouple `isAISummaryDisabled` from `ai_summary_delay: "0s"` and enforce `NoAISummary` in `effectiveAISummaryDelay`

* **Domain**: code (`internal/cli`).
* **Execution Posture**: `test-first` (`harness-surface:harness-architect`).
* **Size / Complexity / Label**: `Size: XS` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:harness-architect"]`.
* **Files Affected (2)**:
  * `internal/cli/capture.go`
  * `internal/cli/capture_test.go`
* **Functions Changed (`4` total, `< 5`)**: `isAISummaryDisabled` (`capture.go:73-86`), `effectiveAISummaryDelay` (`capture.go:88-107`), `runAppendAISummary` (`capture.go:112-120`), and `TestDefaultNewMeetingAndNewSessionWireAISummaryFlags` / `TestIsAISummaryDisabledAndEffectiveDelayAcrossModes` (`capture_test.go`).
* **Changes Needed**:
  1. In `isAISummaryDisabled(c config.Config) bool` (`internal/cli/capture.go:73-86`):
     * Return `false` when `strings.TrimSpace(c.AppendAISummary) != ""`.
     * Return `true` when `c.NoAISummary` is `true` OR `strings.EqualFold(strings.TrimSpace(c.CaptureMode), config.CaptureModeEphemeralLive)`.
     * Return `false` otherwise (removing the `c.AISummaryDelay` `d == 0` check so `ai_summary_delay: "0s"` in `config.yaml` or `--ai-summary-delay 0s` keeps `isAISummaryDisabled == false` for `past-recap`, `auto`, and `live-with-summary` — preserving immediate `# AI Summary` export on already-available Meeting Recaps while `runCaptureWithConfig` still returns at `delay <= 0` before queuing or fetching delayed post-call enrichment on live calls).
  2. In `effectiveAISummaryDelay(cfg config.Config, isRecap bool) time.Duration` (`internal/cli/capture.go:88-107`):
     * Return `0` immediately when `isRecap || cfg.NoAISummary` so `Config.NoAISummary` unconditionally suppresses post-live-call timers and deferred queue entries even if `AISummaryDelay` is `""` or positive.
  3. In `runAppendAISummary` (`internal/cli/capture.go:112-120`), clear `recapCfg.NoAISummary = false` alongside `recapCfg.AISummaryDelay = ""`.
  4. Update `TestDefaultNewMeetingAndNewSessionWireAISummaryFlags` and add `TestIsAISummaryDisabledAndEffectiveDelayAcrossModes` in `internal/cli/capture_test.go` verifying both direct `Config` combinations (including `NoAISummary: true` with `AISummaryDelay: ""` and `"15m"`) and end-to-end `runCmd` CLI invocations across `past-recap`, `auto`, `live-with-summary`, `ephemeral-live`, and `--append-ai-summary`.
* **Dependencies**: `blocks` on `U7` (`036.007-T`).

### U9 (`036.009-T`) — Qualify `PerformanceResourceTiming` window scope and document Wave 2A Recap/CLI/Schedule semantics in `ARCHITECTURE.md` and `USER_GUIDE.md`

* **Domain**: docs (`docs/`).
* **Execution Posture**: `P-002` verification-gated docs-only route (`harness-surface:none` assigned by Stage; `harness-architect` applies `harness-verification-gated` at T2; Ship runs `gsd harness docs-only-check --phase baseline` immediately after claim before any edits and `--phase completion` after `EG-1`..`EG-3` and `MG-1` pass before commit).
* **Size / Complexity / Label**: `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:none"]`.
* **Files Affected (2 — Owned Paths)**:
  * `docs/ARCHITECTURE.md`
  * `docs/USER_GUIDE.md`
* **Changes Needed**:
  1. **Durable `PerformanceResourceTiming` Window-Scope Qualification (`BA069506`)**:
     * In `docs/ARCHITECTURE.md` (`Finite Meeting Recap export` item 3) and `docs/USER_GUIDE.md` (`Compiled executable and explicit output file` Recap audio paragraph), qualify durably (without embedding transient stash hashes in product docs) that `window.performance.getEntriesByType('resource')` inspects **window-visible** `PerformanceResourceTiming` resource entries and does not observe `oneDrive.transcode` requests initiated solely inside a dedicated Web Worker whose entries remain isolated to Worker-local performance timelines without dedicated Worker CDP `Network` / `Target.setAutoAttach` instrumentation.
  2. **Wave 2A Audio Playback, Retry, Identity & Memory-Cap Alignment**:
     * In `docs/ARCHITECTURE.md` (`Finite Meeting Recap export` items 1–3), document periodic `1000ms` `'Play video'` re-clicking (with replacement `<video>` mute/play tracking) prior to initial stream activity, skipping direct non-blob candidate fetches on `isDownloadBlocked=true` stream surfaces, per-worker `inFlightSourceBytes` reservation against the `256 MiB` source cap across the 12 Path A workers, case-insensitive single-pass percent-decoding on scheme-less/server-relative `?id=` and `fileUrl` paths, attempt-0 retry on `streamembed.aspx`/`stream.aspx` `isDownloadBlocked=true` with non-stream parent target skipping, and `xplatplugins.aspx`-independent delayed-mount discovery polling.
  3. **Wave 2A `--no-ai-summary`, `--queue`, `ai_summary_delay: "0s"`, and `schedule` Precedence Alignment**:
     * In `docs/ARCHITECTURE.md` (`Finite Meeting Recap export` item 2 and `Batch queue` item 2 `inheritedQueueFlags` list), note that Recap AI Summary extraction runs unless disabled via `--no-ai-summary` (`Config.NoAISummary`) or `ephemeral-live` mode (never suppressed by `ai_summary_delay: "0s"`), distinguish that on a live call (`live-with-summary` or `auto` landing on a live call) `ai_summary_delay: "0s"` (`delay <= 0` in `runCaptureWithConfig`) disables delayed post-call AI Summary queuing and fetching (returning before `appendDeferredAISummaryQueueLineWithConfig` and delayed post-call fetch, NOT performing immediate post-call enrichment) whereas on an already-available Meeting Recap (`past-recap` or `auto` landing on an `isRecap` surface) it preserves immediate `# AI Summary` export unless `--no-ai-summary` (`cfg.NoAISummary`) is explicitly set, and include `--no-ai-summary` in `Batch queue`'s forwarded flags list.
     * In `docs/USER_GUIDE.md` (`--ai-summary-delay` / `--no-ai-summary` flag bullets, `past-recap` and `live-with-summary` / `auto` protocol bullets, `Schedule configuration and flag precedence`, and `ai_summary_delay` configuration table row), clarify that `--no-ai-summary` and `ephemeral-live` disable AI Summary capture across both Meeting Recaps and post-call enrichment, whereas `ai_summary_delay: "0s"` (`delay <= 0` in `runCaptureWithConfig`) disables delayed post-call AI Summary queuing and fetching on live calls (`live-with-summary` or `auto` landing on a live call — returning before queuing or fetching the post-call summary rather than performing immediate post-call enrichment) while preserving immediate `# AI Summary` export for an already-available Meeting Recap (`past-recap` or `auto` landing on an `isRecap` surface) unless `--no-ai-summary` (`cfg.NoAISummary`) is explicitly set, and qualify that a global `schedule --no-ai-summary` flag remains active on scheduled entries (including `mode: past-recap` and `ai_summary_delay: 0s`) unless overridden by a positive per-meeting `ai_summary_delay`.
  4. **`P-002` Verification Record Tables**:

     **Executable Gates**:

     | Gate ID | Literal Command | Working Directory | Exact Pass Criteria | Result |
     |---|---|---|---|---|
     | `EG-1` | `markdownlint --disable MD013 "--" docs/ARCHITECTURE.md docs/USER_GUIDE.md` | `.` | Exit code `0` with zero violations | `pending` |
     | `EG-2` | `gsd gate line-endings --path docs/ARCHITECTURE.md --json` | `.` | Exit code `0` with `crlf_count == 0`, `bom_count == 0`, `missing_eof_newline_count == 0` | `pending` |
     | `EG-3` | `gsd gate line-endings --path docs/USER_GUIDE.md --json` | `.` | Exit code `0` with `crlf_count == 0`, `bom_count == 0`, `missing_eof_newline_count == 0` | `pending` |

     **Manual Gates**:

     | Gate ID | Source Paths | Target Paths | Exact Pass/Fail Criteria | Required Evidence |
     |---|---|---|---|---|
     | `MG-1` | `docs/plans/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan.md` | `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md` | PASS iff both target files durably qualify window-visible `PerformanceResourceTiming` vs dedicated Worker-isolated timelines (without transient stash hashes), document Wave 2A audio/retry/in-flight-cap/single-pass-decoding behavior, explicitly distinguish disabling delayed post-call AI Summary queuing and fetching on live calls (`delay <= 0` in `runCaptureWithConfig` under `ai_summary_delay: "0s"`, without immediate post-call enrichment) from preserving immediate `# AI Summary` export for an already-available Meeting Recap (`past-recap` or `auto` landing on an `isRecap` surface) unless `--no-ai-summary` (`cfg.NoAISummary`) is set alongside `--queue` forwarding and `schedule --no-ai-summary` precedence, and contain zero live URLs, `?p=` passcodes, Entra UPNs, or concrete host profile paths; FAIL otherwise | Task completion comment summarizing verified sections and redaction scan |

* **Dependencies**: `blocks` on `U4` (`036.004-T`) and `U8` (`036.008-T`).

## Dependency Graph

```text
Browser Track:     U1 (036.001-T) -> U2 (036.002-T) -> U3 (036.003-T) -> U4 (036.004-T) ---+
                                                                                            |--> U9 (036.009-T, docs)
Config/CLI Track:  U5 (036.005-T) -> U6 (036.006-T) -> U7 (036.007-T) -> U8 (036.008-T) ---+
```

* Shipment DAG: `036-S` (`priority: critical`) depends via `blocks` on shipped
  `035-S` (and has zero dependency edges to or from `010-S`).
* Zero cycles; sequential edges within `internal/browser` (`U1 -> U2 -> U3 -> U4`)
  and expand-then-contract edges across `internal/config` and `internal/cli`
  (`U5 -> U6 -> U7 -> U8`) guarantee zero intra-package file contention and
  100% build/test greenness after every single unit.

## Decisions and Rationale

1. **Four `< 5`-function 2-file browser slices (`U1 -> U2 -> U3 -> U4`)**:
   Splitting `U1` (playback re-click + blocked skip) from `U2`
   (`inFlightSourceBytes` reservation) keeps inner JavaScript function edits
   strictly `< 5` per task; `U3` pairs `recap.go` with `recap_retry_test.go` so
   `TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable` exercises fast
   8-attempt exhaustion alongside the new CDP retry/discovery tests; and `U4`
   separates `recap.go` expected-RED tests from `meeting_chrome.go`
   characterization tests.
2. **Expand-then-contract `Config.NoAISummary` migration (`U5 -> U6 -> U7 -> U8`)**:
   Adding `Config.NoAISummary` (`U5`) and migrating CLI producers `join.go`
   (`U6`) and `schedule.go` (`U7`) *before* contracting `isAISummaryDisabled` and
   `effectiveAISummaryDelay` (`U8`) guarantees that `--no-ai-summary` never loses
   enforcement at any intermediate commit.
3. **Preserving `035-S`'s 12-worker `runPathA` concurrency (`U4-R3`
   reconciliation)**: `031-S` R3 flagged 16 vs 8 workers, and `035-S` (`024-DL`)
   subsequently standardized and live-verified 12 workers for cold 60+ minute
   SharePoint recordings. Keeping 12 workers and adding `U2`'s synchronous
   `inFlightSourceBytes` cap satisfies both cold-transcode throughput and strict
   `256 MiB` memory enforcement.

## Risks and Caveats

* **Node.js `setTimeout` test harness pitfall**: Per
  `cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`,
  every Node.js `extractRecapAudioJS` / `copyAISummaryJS` test in `U1`, `U2`, and
  `U4` overriding `globalThis.setTimeout` MUST leave `ms >= 10000`
  (`fetchBytesOnce`'s 15s `reqTimer`) unclamped and register a
  `process.on('beforeExit')` completion assertion.
* **SharePoint filename character set (`#` and `%` legal, `?` illegal)**: When
  normalizing scheme-less/server-relative `fileUrl` / `id` paths in `U4`, strip
  `?` query suffixes (`?web=1`) only and reject trailing `/` — never strip `#`,
  because `#` in an already query-decoded `?id=%2F...%2FQ4%20%231.mp4` value is
  a literal filename character (`Q4 #1.mp4`).
* **Inert `streamembed.aspx` / `stream.aspx` retry bound & `-race` safety**:
  Under `U3`, an attempt-0 `isDownloadBlocked=true` on a mounted stream target
  retries **at most once** (2 total evaluations), and tests overriding
  `recapAudioRetryDelay` MUST NOT call `t.Parallel()` and MUST restore via
  `t.Cleanup`.

## Plan Hardening Signals

* **public API, schema, or contract change**: **Present** — adds exported runtime
  field `Config.NoAISummary` (`yaml:"-"`) on `config.Config` and refines CLI
  `ai_summary_delay: "0s"` vs `--no-ai-summary` behavior in `past-recap` and
  `auto` modes.
* **security, auth, permission, or compliance-sensitive behavior**: **Present**
  — modifies recording identity URL decoding (`applyQueryIdentity`,
  `checkIdentityAgainst`), SharePoint `isDownloadBlocked=true` stream retry
  handling, and pre-allocation `256 MiB` memory-cap accounting
  (`inFlightSourceBytes`).
* **migration, backfill, destructive data/config action, or irreversible step**:
  **Absent** — zero config migrations (`yaml:"-"`), zero `go.mod` changes, and
  zero destructive operations.
* **external integration, operator checkpoint, or external dependency**:
  **Present** — touches SharePoint `streamembed.aspx` / `stream.aspx` CDP browser
  integration and Copilot DOM extraction (`teams-auth-mfa` live-tenant
  integration surface).
* **high runtime, rollout, or rollback risk**: **Present** — retry loop bounds
  and in-flight byte accounting in `internal/browser/recap.go` sit directly on
  the 60+ minute Recap audio capture hot path.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed Runtime Surfaces**:
  * `cli` (`internal/config`, `internal/cli`): `go-teams` root, `join`,
    `captions`, `--queue`, and `schedule` AI Summary flag/config wiring.
  * `browser` (`internal/browser`): `extractRecapAudioJS`, `copyAISummaryJS`,
    `ExtractMeetingIdentity`, and `captureRecapAudioFromTargets`.
* **Required Pre-Merge Verification Probes (`Ship`)**:
  1. `gofmt -l .` (0 files), `go vet ./...`, `go vet "-tags=integration" ./...`,
     PowerShell cross-OS vet (`$env:GOOS='darwin'; $env:GOARCH='amd64'; go vet ./...; Remove-Item Env:GOOS,Env:GOARCH`),
     `golangci-lint run`, and `go build -o bin/go-teams.exe ./cmd/go-teams` (run
     after each unit and before PR creation).
  2. `go test -race -count=1 ./...` (including all Node.js `extractRecapAudioJS`
     / `copyAISummaryJS` probes, CDP loopback fixtures, CLI flag/queue/schedule
     suites, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
  3. CLI runtime probes from `.gsd/workspace-profile.yaml`:
     `go run ./cmd/go-teams --help` and `go run ./cmd/go-teams schedule --help`
     (exit `0`).
  4. `markdownlint --disable MD013 "--" docs/ARCHITECTURE.md docs/USER_GUIDE.md`,
     `gsd gate line-endings --path docs/ARCHITECTURE.md --json`, and
     `gsd gate line-endings --path docs/USER_GUIDE.md --json` (exit `0`,
     `crlf_count == 0`).
  5. Live `-tags=integration` checkpoint (`teams-auth-mfa`, `PA-036-7`): if
     `.env.local.test` and an operator-authenticated profile are present and
     authorized, run fresh `go test -count=1 "-tags=integration"` Recap
     verification; when running unattended under `DARK_MODE_ACTIVE` without an
     interactive MFA/live tenant session, record truthful `BLOCKED`
     manual-checkpoint evidence per `.gsd/workspace-profile.yaml`
     (`teams-auth-mfa` fallback) while requiring 100% pass on all unit, Node.js,
     headless Chromium E2E (`TestChromeSharePointStreamAspxRecapEndToEnd`), and
     CLI probes.
* **Operational Closure Artifact (`docs/closure/036-S-036-F-post-merge-closure.md`)**:
  * **Owner**: `softwaresalt`
  * **Validation window**: 7 days / one full scheduled-meeting and Recap export
    cycle after merge (**explicitly flagged as a manual observation requirement**
    per `release-observability.instructions.md` because `go-teams` is a local CLI
    without hosted telemetry; inspect via `gh run list --workflow=ci.yml`,
    scheduler stdout/stderr logs, and output transcript `.md`/`.mp3` files
    against the `100%` pass / `0` unexpected exit pre-change baseline).
  * **Healthy signals**: 100% pass on Windows CI (`test-windows`), secret-hygiene
    gate (`Detect code changes`), `go test -race ./...`, and Recap exports with
    `ai_summary_delay: "0s"` producing both transcript and `# AI Summary` +
    valid MPEG sync `.mp3`.
  * **Failure signals & rollback trigger**: Any regression in `go test -race
    ./...`, memory growth above `256 MiB` source / `360 MiB` PCM caps, infinite
    CDP retry loops, missing `# AI Summary` in `past-recap` mode without
    `--no-ai-summary`, or `--no-ai-summary` failing to suppress `# AI Summary`.
  * **Rollback procedure**: Stop any running `go-teams schedule` daemon, revert
    the two-parent merge commit (`git revert -m 1 <merge_sha>`), rebuild
    `bin/go-teams.exe`, and re-verify `go test -race ./...` and
    `go run ./cmd/go-teams --help`.
  * **Mandatory Post-Merge Closure Gates**:
    1. Record the Shipment Documentation Consistency Audit outcome
       (`AUDIT_CLEAN` / `AUDIT_FINDINGS_RESOLVED` / `AUDIT_FOLLOWUPS`) per
       `shipment-documentation-audit.instructions.md`.
    2. Refresh `docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md`
       during `compound-refresh` to reflect `U4`'s single-pass scheme-less/server-relative
       `fileUrl` decoding (`69AB7ACC`).
    3. Execute mandatory `P-020` post-merge context compaction (`compact-context`
       skill) over session memories and decided plan artifacts.

## Plan Hardening

### Why Hardening Was Required

Four hardening signals are present: runtime `Config.NoAISummary` addition and
CLI `ai_summary_delay: "0s"` contract refinement, recording identity URL
decoding (`applyQueryIdentity` / `checkIdentityAgainst`), pre-allocation `256
MiB` concurrent streaming byte-cap enforcement (`inFlightSourceBytes`), and CDP
retry/target-skip control flow on SharePoint `streamembed.aspx` / `stream.aspx`
targets.

### Reinforced Instructions and Compound Learnings Consulted

* `.github/instructions/constitution.instructions.md` (Principles I–XI, 2-Hour
  Rule, Width Isolation, `P-020` compaction)
* `.github/instructions/strict-safety.instructions.md` (`ProposedAction` /
  `ActionRisk` / `ActionResult` contract)
* `.github/instructions/go.instructions.md` &
  `.github/instructions/markdown.instructions.md`
* `docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md`
* `docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`
* `docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`
* `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`

### Protected Invariants (1–6)

1. **Invariant 1 — Pure-Go Single Binary**: Zero external media binaries
   (`ffmpeg`, `yt-dlp`), zero CGO, and zero `go.mod` / `go.sum` modifications.
2. **Invariant 2 — Per-Frame Identity Isolation, Same-Origin HTTPS Containment,
   and Case-Insensitive Single-Pass Decoding**: `isAllowedSameOriginTranscodeURL`,
   `track !== 'video'` audio `wsd`/`ppd` isolation,
   `VroomItemID`/`VroomItemId`/`itemId`/`driveItemId`-only `m.recapItemID`
   retention (`U5-R3`), and conflict-checked `MeetingIdentity` matching remain
   enforced before any media decode or PCM transfer; query keys match
   case-insensitively and scheme-less/server-relative `id` and `fileUrl` paths
   decode percent-escapes in a single pass without corrupting literal `%` or `#`.
3. **Invariant 3 — Pre-Allocation Resource, In-Flight Byte, and Duration Caps +
   Error Precedence**: Combined committed + in-flight source bytes
   (`streamSourceBytes + inFlightSourceBytes`) across all 12 Path A workers and
   MSE buffers MUST NOT exceed `maxRecapAudioSourceBytes` (`256 MiB`), with
   synchronous `fatalErr`/`abortCtrl.abort()` before `await reader.cancel()` and
   `workerReserved` released in `fetchBytesOnce`'s `finally` on every
   error/retry/abort path; Path B zero-duration streams remain rejected before
   `decodeAudioData` (`U2-R3`); empty CDP transfer batches before `ChunkCount`
   fail closed (`U3-R3`); and `errPriority` ranks resource-cap (`3`) >
   identity/MSE/Path-A-deadline (`2`) > `isDownloadBlocked=true` (`1`) (`U6-R3`).
4. **Invariant 4 — Bounded Retry & Playback Trigger Discipline**: `'Play video'`
   re-clicking (`>= 1000ms` interval) stops immediately once `activityObserved`
   or `pathAActive` is `true`; direct non-blob fetches are skipped only when
   `info.isDownloadBlocked && streamSurface` (preserving `!streamSurface` direct
   probes); path-based `isStreamTarget` ignores query/fragment strings and SSO
   redirects; mounted stream targets retry `attempt == 0` `isDownloadBlocked=true`
   or stream-deadline errors **at most once** (2 total attempts) and skip
   non-stream parent targets in the same attempt while preserving sibling stream
   target evaluation on identity errors; and `!hasStreamEmbedTarget`
   delayed-mount polling (`maxAttempts = 8`) skips immediately when an owned
   target recorded an `identity mismatch`.
5. **Invariant 5 — Clean Browser Global, Timer, Hook, and Transfer Restoration**:
   `cleanupHooks()` restores `fetch`, `XMLHttpRequest`, `MediaSource`,
   `SourceBuffer`, `crypto.subtle.decrypt`, `activeTimers`, `AbortController`,
   and paused media state on every exit path, and `cleanupPendingTransfer()`
   deletes `window.__goTeamsRecapAudioTransfer` on all normal and early-exit
   branches.
6. **Invariant 6 — Explicit AI Summary Disablement & Strict Redaction**:
   `isAISummaryDisabled` returns `true` ONLY when `AppendAISummary == ""` and
   (`Config.NoAISummary` is `true` OR `CaptureMode == "ephemeral-live"`), and
   `effectiveAISummaryDelay` returns `0` whenever `isRecap || cfg.NoAISummary`;
   zero live tenant URLs, UPNs, passcodes, or tokens may appear in any tracked
   source, test, backlog, or doc file.

### Lightweight 3-Threat Model

* **T1 (Unbounded concurrent Path A memory growth)**: Up to 12 concurrent
  `fetchBytesOnce` workers streaming segments without `Content-Length` could
  buffer >`256 MiB` before committing `streamSourceBytes` -> Mitigated by
  per-chunk `inFlightSourceBytes` reservation checked prior to `chunks.push(v)`,
  synchronous `fatalErr`/`abortCtrl.abort()` before `await reader.cancel()`, and
  unconditional `finally` release on non-committed exits (`U2`).
* **T2 (Cross-recording spoofing via double-decoded `fileUrl`, case-variant
  query keys, or stale Copilot DOM)**: Double percent-decoding
  `/Recordings/foo%2520bar.mp4` into `foo bar.mp4`, skipping identity on
  `?ID=...`, or reading a pre-existing/uncontained Copy button in
  `copyAISummaryJS` could misbind recording identity or return stale summary
  text -> Mitigated by case-insensitive single-pass scheme-less/server-relative
  path extraction in both Go and JS and `Boolean(msg)` + non-`.fui-ChatMyMessage`
  + non-`.ms-List` + `goTeamsPreexisting` filtering (`U4`).
* **T3 (Runaway CDP target evaluation / retry amplification)**: Evaluating
  `xplatplugins.aspx` and `/v2/` after `streamembed.aspx` fails in the same
  attempt or retrying mounted inert pages 8 times multiplies 10s–240s timeouts ->
  Mitigated by `skipNonStreamTargets` on retryable stream errors and restricting
  `hasStreamEmbedTarget == true` retries strictly to `attempt == 0`
  (`sawRetryableStreamTargetErr`) within the operation-wide `10m` `captureCtx`
  (`U3`).

### Strict-Safety `ProposedAction` Inventory

| Action ID | ProposedAction Summary | Targets | Change Kind | ActionRisk | Approval Required | Approval Path | ActionResult | Rollback / Containment |
|---|---|---|---|---|---|---|---|---|
| `PA-036-1` | Harden `extractRecapAudioJS` periodic 1000ms Play re-click + replacement `<video>` safety + blocked non-blob skip (`U1`) and 12-worker `inFlightSourceBytes` reservation (`U2`) | `internal/browser/recap.go`, `internal/browser/recap_test.go` | local code + unit test edit | `high` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope) | `planned` | `git revert` task/merge commit; guarded by Node.js unit suites and Protected Invariants 2–5 |
| `PA-036-2` | Harden `captureRecapAudioFromTargets` path-based `isStreamTarget`, `skipNonStreamTargets`, `sawRetryableStreamTargetErr` attempt-0 retry, and delayed-mount discovery (`U3`) | `internal/browser/recap.go`, `internal/browser/recap_retry_test.go` | local code + unit test edit | `high` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope) | `planned` | `git revert` task/merge commit; bounded by `attempt == 0` cap and `10m` `captureCtx` |
| `PA-036-3` | Normalize case-insensitive single-pass `fileUrl`/`id` `%` decoding, tighten `copyAISummaryJS` container guards, and add `canonicalStreamTargetMatch` characterization tests (`U4`) | `internal/browser/recap.go`, `internal/browser/recap_test.go` | local code + unit test edit | `moderate` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope) | `planned` | `git revert` task/merge commit; verified by Go + Node.js identity parity tests |
| `PA-036-4` | Expand-then-contract runtime `Config.NoAISummary` (`yaml:"-"`) across `config`, `join`/`--queue`, `schedule`, and `capture` (`U5`–`U8`) | `internal/config/config.go`, `internal/config/config_test.go`, `internal/cli/join.go`, `internal/cli/join_test.go`, `internal/cli/schedule.go`, `internal/cli/schedule_test.go`, `internal/cli/capture.go`, `internal/cli/capture_test.go` | local contract/CLI + unit test edit | `moderate` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope) | `planned` | `git revert` task/merge commit; zero on-disk YAML schema changes (`yaml:"-"`) |
| `PA-036-5` | Update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` under `P-002` verification-gated docs-only route (`U9`) | `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md` | documentation edit | `low` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope) | `planned` | `git revert` task/merge commit; gated by `markdownlint`, `gsd gate line-endings`, and `gsd harness docs-only-check` |
| `PA-036-6` | Harvest `036-F` + `036.001-T`..`036.009-T`, assemble `critical` shipment `036-S` (`blocks` on `035-S`), and archive the 7 consumed Wave 2A stash entries via non-destructive `backlogit` archival | `.backlogit/queue/*`, `.backlogit/stash.jsonl`, `.backlogit/archive/stash.jsonl` | backlog staging | `low` | `false` | Pre-authorized under `DARK_MODE_ACTIVE` (`P-017` Wave 2A scope; non-destructive `backlogit` stash archival) | `planned` | Reversible from `.backlogit/archive/stash.jsonl` and uncommitted branch state prior to Step 1.5c staging PR |
| `PA-036-7` | Conditional `-tags=integration` live-tenant Recap verification checkpoint (`teams-auth-mfa`) | `.env.local.test` / live SharePoint & Teams Recap | external call | `moderate` | `true` (unless pre-authenticated `.env.local.test` is authorized) | Operator live-session authorization; transitions to truthful `ActionResult: blocked` manual-checkpoint record under unattended `DARK_MODE_ACTIVE` when unavailable | `planned` | Read-only Recap export probe; zero production state mutation |

<!-- plan-review-attempt: 1 -->

### Review-Gate Capability Risks and Marker Contract (`P-012`)

* Before `harvest`, `plan-review` MUST execute across all always-on personas
  (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`,
  `Learnings Researcher`), all triggered cross-model personas
  (`Architecture Strategist` routed via anchor `gpt-6.1-sol` when dispatchable,
  plus `Security Lens Reviewer` for URL identity / permission / resource-cap
  boundaries; `Agent-Native Parity Reviewer` is not triggered because `go-teams`
  exposes no MCP server or agent tool surface per `.gsd/workspace-profile.yaml`
  `agent_native.detected: false`), **and** a `rubber-duck` external/second-opinion
  adversarial review pass per `gsd-dark-mode` Step 2.1 (declaring
  `TOOL_DEGRADED: rubber-duck — declared fallback: same-roster adversarial
  review` with an independent `Correctness Reviewer` / `Evidence Consistency
  Reviewer` challenger if an external `rubber-duck` CLI/tool is not installed).
* The review MUST be recorded via `gsd op review record-result --in-place`
  writing `docs/reviews/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan-review.md`
  and appending `## Plan Review` with literal `dispatch_mode:` and `decision:`
  markers (`MaxPlanReviewPasses = 2`).
* **Unresolved operator decisions blocking execution**: None.

## Plan Review

Review pass 2: **PASS** (plan revision 2). See `docs/reviews/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan-review.md`.
