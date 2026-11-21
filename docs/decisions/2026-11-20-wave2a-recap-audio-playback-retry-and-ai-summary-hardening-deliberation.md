---
title: "Wave 2A SharePoint Recap Audio Playback, Retry, Identity Decoding, and Past-Recap AI Summary Hardening"
description: "Deliberation reconciling Wave 2A stash entries 47D1E1C0, 196AB9E4, 69AB7ACC, 0DE2E900, 39450846, 7883544E, and BA069506 against post-035-S HEAD (ea651c1)"
doc_type: "decision"
topic: "Wave 2A SharePoint streamembed.aspx/stream.aspx audio resilience, URL identity decoding, Copilot freshness, and past-recap ai_summary_delay=0s semantics"
depth: "standard"
decision_status: "decided"
promoted_to: "both"
deliberation_id: "027-DL"
feature: "036-F"
shipment: "036-S"
date: "2026-11-20"
status: "accepted"
linked_artifacts:
  - "docs/plans/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan.md"
  - ".backlogit/queue/027-DL.md"
tags:
  - "recap-audio"
  - "sharepoint-stream"
  - "ai-summary"
  - "url-identity"
  - "wave-2a"
---

## Problem Frame

Under `DARK_MODE_ACTIVE` (`P-017`, `merge_approval_pre_authorized: true`,
`admin_fallback_pre_authorized: false`), Wave 2A consolidates seven active stash
entries targeting SharePoint Meeting Recap / `stream.aspx` audio extraction,
recording identity normalization, Copilot recap selection, CLI AI Summary flag
semantics, and user/architecture documentation:

* **`47D1E1C0` (`high`, `bug`)**:
  1. Early `CaptureAudio` invocations on `streamembed.aspx` / `stream.aspx`
     click the `'Play video'` button only once at `t=0` (`btnClicked`), so a
     player whose SharePoint token or React/VideoJS handler finishes mounting
     after `t=0` never starts playback and aborts after the 10-second
     `initialActivityTimeoutMs` window; additionally, direct non-`blob:`
     `candidateUrl` fetches still fire even when `info.isDownloadBlocked &&
     streamSurface` is true.
  2. In `captureRecapAudioFromTargets` (`internal/browser/recap.go`), when a
     mounted `streamembed.aspx` or `stream.aspx` target (`hasStreamEmbedTarget ==
     true`) returns `isDownloadBlocked=true` on attempt `0`, `shouldRetry` refuses
     to retry, and the inner `orderedTargets` loop continues evaluating
     `xplatplugins.aspx` and outer `/v2/` targets in the same attempt.
  3. In `isAISummaryDisabled` (`internal/cli/capture.go`), `ai_summary_delay:
     "0s"` in `config.yaml` (or `--ai-summary-delay 0s`) disables AI Summary
     extraction during `past-recap` (and `auto` Recap) exports because
     `--no-ai-summary` previously signaled disablement solely by setting
     `Config.AISummaryDelay = "0s"`.
* **`196AB9E4` (`low`, `bug` — `033-S` `F-1b`)**: Ensure SharePoint Copilot recap
  selection in `copyAISummaryJS` (`internal/browser/recap.go`) never reads a
  stale pre-existing `.fai-CopilotMessage`, user prompt bubble, or uncontained
  button while `'Recap the meeting'` is still generating.
* **`69AB7ACC` (`low`, `bug` — `033-S` `F-4b`)**: Normalize percent-decoding in
  `applyQueryIdentity` and `checkIdentityAgainst` (`internal/browser/recap.go`)
  to a single pass across both `?id=` and server-relative `fileUrl` paths
  containing literal `%` sequences.
* **`0DE2E900` (`low`, `bug` — `033-S` advisory)**: Verify case-insensitive
  query key normalization (`id`, `UniqueId`, `driveId`) in
  `canonicalStreamTargetMatch` (`internal/browser/meeting_chrome.go`) and ensure
  thorough negative and positive unit test coverage.
* **`39450846` (`low`, `task` — `031-S` R3 `U1-R3`..`U6-R3`)**: Reconcile all six
  `031-S` Round 3 items against post-`033-S` / post-`035-S` `HEAD` (`ea651c1`)
  and close any remaining code or test gap without regressing `035-S`.
* **`7883544E` (`low`, `task` — `031-S` R5 `U1-R5`, `U2-R5`)**: Reserve
  per-worker in-flight streaming bytes against `maxRecapAudioSourceBytes` during
  concurrent Path A downloads (`U1-R5`) and poll delayed `streamembed.aspx`
  mounts when `lastErr == nil` even without an `xplatplugins.aspx` target
  (`U2-R5`, reconciled with `47D1E1C0`).
* **`BA069506` (`low`, `task` — `033-S` docs advisory)**: Qualify
  `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md` to state that
  `PerformanceResourceTiming` inspects window-visible resource entries (with
  dedicated Worker-only CDP instrumentation tracked under `4C40AF2B`), alongside
  all documentation updates required for Wave 2A behavior.

### Explicit Scope Boundaries

* **In scope**: The 7 Wave 2A stash entries listed above (`47D1E1C0`,
  `196AB9E4`, `69AB7ACC`, `0DE2E900`, `39450846`, `7883544E`, `BA069506`),
  packaged into covering feature `036-F` and `critical`-priority shipment
  `036-S`.
* **Strictly out of scope**:
  * Queued Wave 1 shipment `010-S` (`010-F`, `010.001-T`..`010.009-T`, `001-DL`,
    `025-DL`, `026-DL`) — never modified or depended upon by `036-S`.
  * Wave 2B `DEFERRED SCOPE EXPANSION` entries (`E03CB3C3` full path+origin
    `MeetingIdentity`, `4C40AF2B` dedicated Web Worker CDP `Network`
    interception, `B98262C8` Path B cross-attempt MSE segment stitching / player
    reload) — left `active` and untouched in `.backlogit/stash.jsonl`.
  * All other active stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`,
    `232C6844`, `AD451E00`, `3F6BA819`, `59545A24`, `C3460FDC`, `4AB8EAEE`,
    `EF14C4D1`, `326E22EB`, `4A8F9C31`, `C7BB2E7E`, `75AD89E9`, `920CF700`,
    `CDCE76CE`) — left `active` and untouched.
  * Pre-existing foreign dirty/untracked paths in the working tree (`.github/*`,
    `start.ps1`, `docs/memory/2026-11-18/*`, `.gsd/carry-forward.yaml`) under
    `P-011`.

### P-021 C5/C6 Triage Audit

* **Precedence check**: None of the 7 Wave 2A entries carry the `DEFERRED SCOPE
  EXPANSION` marker (all three Recap `DEFERRED SCOPE EXPANSION` entries belong
  to Wave 2B).
* **Duplicate scan (`P-021 C5/C6 (A)`)**: `CLEAN DUPLICATE SCAN` across all 26
  active stash entries, `.backlogit/queue/`, and `.backlogit/archive/stash.jsonl`.
* **Late-identifier reconciliation (`P-021 C5/C6 (B)`)**: No `N/A` source-ref
  fields exist on any of the 7 Wave 2A entries (`39450846` and `7883544E` map to
  `031-S` / PR `#76`; `196AB9E4`, `69AB7ACC`, `0DE2E900`, and `BA069506` map to
  `033-S` / PR `#80`; `47D1E1C0` is a direct post-`035-S` bug intake).

### Crystallized Intent Block (`GRILL_ME_INTENT_CRYSTALLIZED`, `dark_factory_assumed`)

| ID | Question | Decision | Source | Rationale |
|---|---|---|---|---|
| `D1` | How should `triggerPlaybackIfReady()` re-click `'Play video'` without interfering with active Path A, already-running Path B streams, or replacement `<video>` elements? | Replace one-shot `btnClicked` with `lastBtnClickAt` (`0` initially), enforce `m.muted = true; m.volume = 0; m.playbackRate = 16` whenever `m && !pathAActive`, track `triggeredMediaEl` reference (instead of sticky boolean `mediaPlayTriggered`) so replacement `<video>` elements receive `.play()` once, and click `btn` whenever `!activityObserved && !pathAActive && (lastBtnClickAt === 0 || Date.now() - lastBtnClickAt >= 1000)`. | `repo-fact` / `47D1E1C0` | Once `activityObserved` or `pathAActive` becomes `true`, button re-clicking stops immediately so active playback is never toggled back to pause; replacement `<video>` elements stay muted and play-triggered once, while 1000 ms retries wake players that finish initializing after `t=0`. |
| `D2` | How should `extractRecapAudioJS` skip non-blob `candidateUrl` fetches when `info.isDownloadBlocked && streamSurface` without weakening identity or duration guards? | Run `checkIdentityAgainst` and pre-fetch `duration` limit checks first, then for non-`blob:` candidates under `info.isDownloadBlocked && streamSurface`, record `blockedErr` (`isDownloadBlocked=true`) and `continue` without invoking `fetchFn`. | `repo-fact` | Preserves Protected Invariants 2 (identity conflict/unverified checks) and 3 (duration limit rejection) while eliminating guaranteed-403/HTML network round-trips before `captureStreamEmbedAudio()` runs; non-stream pages (`!streamSurface`) continue probing direct candidates as tested in `TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked`. |
| `D3` | How do `47D1E1C0` (part 2), `39450846` (`U1-R3`, `U6-R3`), and `7883544E` (`U2-R5`) reconcile in `captureRecapAudioFromTargets`? | (a) Define path-based `isStreamTarget(rawURL)` (`url.Parse`, `!isAuthRedirectURL`, `u.Path` matching `streamembed.aspx`/`stream.aspx`), rank stream `page` targets ahead of non-stream targets, and compute `hasStreamEmbedTarget` strictly over in-scope, non-auth-redirect, non-identity-conflicting targets; (b) on retryable stream errors (`isStreamDeadlineMsg` or `isDownloadBlocked=true`) from an `isStreamTarget`, set `skipNonStreamTargets = true` so `xplatplugins.aspx` and `/v2/` are skipped while any remaining non-conflicting stream targets still run on `identity mismatch`/`unverified`/`-32000`; (c) record priority-`0` `fmt.Errorf("browser: recap audio frame URL does not match requested recording")` in `lastErr` when skipping conflicting owned URLs (so an evaluated valid stream target's priority-`1` `isDownloadBlocked=true` still outranks a skipped sibling frame URL, while single-frame URL mismatches still set `lastErr != nil` and fail fast on attempt `0`) while tracking per-attempt `sawRetryableStreamTargetErr` so sibling mismatches never suppress attempt-0 stream retry, and poll up to `maxAttempts` (`8`) when `!hasStreamEmbedTarget && (lastErr == nil || isDownloadBlockedErr)` without `hasXplatTarget`; (d) keep `errPriority` ranking stream-deadline/MSE errors (`2`) above `isDownloadBlocked=true` (`1`). | `repo-fact` | Satisfies all three stash entries simultaneously with zero contradictions: inert stream surfaces retry at most once (2 total evals), query-string `stream.aspx` or SSO redirects never suppress delayed-mount polling, parent targets are never needlessly evaluated in the same attempt, and `recap_retry_test.go` exercises full 8-attempt exhaustion cleanly. |
| `D4` | How should `U1-R5` (`7883544E`) bound concurrent Path A streaming memory across all 12 `035-S` workers while honoring `< 5 functions changed` per task? | Isolate `U1-R5` into its own 2-file task (`U2`), keep `runPathA` at `12` workers (`035-S` supersedes `U4-R3`'s older 8-worker figure), and track `inFlightSourceBytes` alongside committed `streamSourceBytes` via a shared cap helper in `captureStreamEmbedAudio()`. Set `fatalErr` and `abortCtrl.abort()` synchronously before `await reader.cancel()`, commit exact `total` on completion, and release uncommitted `workerReserved` in `finally` on error, retry, or abort. | `repo-fact` | Guarantees aggregate committed + in-flight buffered bytes across all 12 concurrent workers never exceed `256 MiB` (`maxRecapAudioSourceBytes`) with zero microtask yield windows or retry reservation leaks, while keeping inner JS function edits strictly `< 5` per task. |
| `D5` | How should `69AB7ACC` (`F-4b`) and `0DE2E900` normalize percent-decoding and query key matching across Go `applyQueryIdentity` and JS `checkIdentityAgainst`? | Match query keys case-insensitively (`threadId`, `driveItemId`, `driveId`, `callId`, `iCalUid`, `fileUrl`, `id`) in both Go and JS, trim values before regex checks, and treat any scheme-less, non-protocol-relative path (no `scheme:` prefix and not starting with `//` or `:`) identically for BOTH `id` and `fileUrl`: strip any `?` query suffix (`?web=1`), reject trailing `/` in Go to match JS `$` anchoring, and skip `url.Parse` / `new URL` + `decodeURIComponent` so literal `%` and `#` survive the single query-parameter unescape pass. Keep `url.Parse` / `new URL` + `safeDecodeURIComponent` strictly for scheme-bearing (`http://` / `https://`) and protocol-relative (`//...`) URLs in both Go and JS. | `repo-fact` | Handles leading-slash (`/Recordings/...`), relative (`Recordings/...`), scheme-bearing (`https://...`), and protocol-relative (`//...`) values with identical single-pass `%` decoding across Go and JS while ensuring uppercase `?ID=` / `?DriveItemId=` URLs enforce recording identity identically to `canonicalStreamTargetMatch`. |
| `D6` | How should `isAISummaryDisabled` and `effectiveAISummaryDelay` (`47D1E1C0` part 3) distinguish `--no-ai-summary` from `ai_summary_delay: "0s"` across all four capture modes and `schedule`? | Follow an expand-then-contract task sequence (`U5 -> U6 -> U7 -> U8`): add runtime-only `NoAISummary bool` (`yaml:"-"`) to `config.Config` (`U5`); migrate producers `applyCaptureFlags`/`inheritedQueueFlags` (`U6`) and `applyScheduleFlags`/`buildScheduleEntryConfig` (`U7`, clearing `entryCfg.NoAISummary = false` when `*e.AISummaryDelay > 0` and aligning `schedule` flag help); then contract consumers `isAISummaryDisabled` (true ONLY when `AppendAISummary == ""` and [`c.NoAISummary` or `CaptureMode == ephemeral-live`]) and `effectiveAISummaryDelay` (return `0` immediately when `isRecap || cfg.NoAISummary`) in `capture.go` (`U8`). | `repo-fact` / `47D1E1C0` | Cleanly separates disabling delayed post-call AI Summary queuing and fetching on live calls while preserving immediate export on already-available Meeting Recaps (`ai_summary_delay: "0s"`, where `runCaptureWithConfig` returns at `delay <= 0` on live calls before queuing or fetching post-call enrichment) from suppressing AI Summary extraction altogether (`--no-ai-summary` / `ephemeral-live`), keeping every intermediate commit behaviorally green across all CLI commands and tests. |

* **Runnable Objective Acceptance Checks (`AC1..AC6`)**:
  * `AC1`: `go test -race ./internal/browser/...` passes new Node/Go unit tests proving periodic `1000ms` `'Play video'` re-clicking (with replacement `<video>` mute/play safety) while `!activityObserved && !pathAActive`, zero direct non-blob `fetch` calls when `info.isDownloadBlocked && streamSurface`, and aggregate `streamSourceBytes + inFlightSourceBytes <= maxRecapAudioSourceBytes` enforcement (with synchronous `fatalErr`/`abortCtrl.abort()` before `reader.cancel()`) across concurrent Path A workers with clean release on retry.
  * `AC2`: `go test -race ./internal/browser/...` passes new CDP fixture tests in `recap_retry_test.go` proving attempt-0 retry on `streamembed.aspx`/`stream.aspx` `isDownloadBlocked=true` (stopping after attempt 1 on persistent block), `skipNonStreamTargets` skipping `xplatplugins.aspx`/`/v2/` while preserving sibling stream target evaluation on identity mismatch, path-based `isStreamTarget`/`hasStreamEmbedTarget` delayed-mount discovery without `xplatplugins.aspx` (including fast 8-attempt exhaustion in `TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable`), and `errPriority` preservation of stream-deadline errors over `isDownloadBlocked=true`.
  * `AC3`: `go test -race ./internal/browser/...` passes single-pass scheme-less/server-relative, scheme-bearing, and protocol-relative (`//...`) `fileUrl` and `id` percent-decoding and case-insensitive query-key parity tests (`100% Sync.mp4`, `foo%20bar.mp4`, `Recordings/Sync.mp4?web=1`, `//example.invalid/Recordings/Sync.mp4`, `?ID=...`), `copyAISummaryJS` fresh `.fai-CopilotMessage` container guard tests (excluding `.fui-ChatMyMessage` and `.ms-List`), and `canonicalStreamTargetMatch` / `recapItemID` characterization tests.
  * `AC4`: `go test -race ./internal/config/... ./internal/cli/...` passes at every intermediate commit (`U5 -> U6 -> U7 -> U8`), proving `ai_summary_delay: "0s"` in `config.yaml` (and `--ai-summary-delay 0s`) keeps `isAISummaryDisabled == false` in `past-recap`, `auto`, and `live-with-summary` while `effectiveAISummaryDelay` returns `0` for both `ai_summary_delay: "0s"` and `NoAISummary: true` (so `runCaptureWithConfig` returns at `delay <= 0` without queuing or fetching delayed post-call enrichment on live calls while preserving immediate `# AI Summary` export on already-available Meeting Recaps), and `--no-ai-summary` / `ephemeral-live` set `isAISummaryDisabled == true`.
  * `AC5`: `markdownlint --disable MD013 "--" docs/ARCHITECTURE.md docs/USER_GUIDE.md`, `gsd gate line-endings --path docs/ARCHITECTURE.md --json`, and `gsd gate line-endings --path docs/USER_GUIDE.md --json` exit `0` with durable `PerformanceResourceTiming` window-visibility qualification and Wave 2A audio/CLI/schedule documentation updates present.
  * `AC6`: `gsd harness resolve --workspace . --shipment 036-S --json` returns `HARNESS_READY` (`exit_code: 0`) and `gsd gate pipeline-topology --mode agent --shipment 036-S --phase pre_claim --json` exits `0`.

## Research Findings

### Post-`035-S` Codebase Reconciliation Matrix (`HEAD` `ea651c1`)

| Stash / Sub-Item | `HEAD` (`ea651c1`) Status | Evidence in Codebase | Wave 2A Action |
|---|---|---|---|
| `47D1E1C0` (1a): Periodic `'Play video'` re-click | **OPEN** | `recap.go:1658-1692` clicks `btn` once (`btnClicked = true`) | Track `lastBtnClickAt` + `triggeredMediaEl` and re-click every `>= 1000ms` while `!activityObserved && !pathAActive` (`U1`) |
| `47D1E1C0` (1b): Skip non-blob direct fetch on `isDownloadBlocked && streamSurface` | **OPEN** | `recap.go:357-436` calls `fetchFn` on `nonBlobCandidates` before `captureStreamEmbedAudio()` | Record `blockedErr` and `continue` for non-`blob:` URLs when `info.isDownloadBlocked && streamSurface` after identity & duration checks (`U1`) |
| `47D1E1C0` (2): Attempt-0 `isDownloadBlocked=true` retry + skip parent targets on stream failure | **OPEN** | `recap.go:2054-2065,2104-2110` breaks only on `isStreamDeadlineMsg`; `recap.go:2249-2254` gates `isDownloadBlocked=true` on `!hasStreamEmbedTarget` | Add path-based `isStreamTarget`, `skipNonStreamTargets` on retryable stream errors, and `sawRetryableStreamTargetErr` attempt-0 retry (`U3`) |
| `47D1E1C0` (3): `past-recap` `ai_summary_delay: '0s'` semantics | **OPEN** | `capture.go:73-107` treats `AISummaryDelay == "0s"` as disabled; `join.go:220` / `schedule.go:88` conflate `--no-ai-summary` with `AISummaryDelay = "0s"` | Expand `Config.NoAISummary` (`U5`), migrate `join`/`schedule` producers (`U6`–`U7`), and contract `isAISummaryDisabled` + `effectiveAISummaryDelay` (`U8`) |
| `196AB9E4` (`033-S` `F-1b`): `copyAISummaryJS` stale `.fai-CopilotMessage` | **PARTIALLY RESOLVED (`83e39f1`/`c173196`) — HARDEN CONTAINER GUARD** | `recap.go:2531-2553` tags `goTeamsPreexisting = 'true'` on click, but `freshCopyBtns` passes if `msg` is `null` or inside `.fui-ChatMyMessage` | Require `Boolean(msg)`, exclude `.fui-ChatMyMessage` / `[class*="ChatMyMessage"]`, and exclude `.ms-List` on `recapBtn` (`U4`) |
| `69AB7ACC` (`033-S` `F-4b`): Single-pass `%` decoding on `?id=` and `fileUrl` | **OPEN FOR RELATIVE / SERVER-RELATIVE `fileUrl`** | `recap.go:90-124` and `recap.go:2342-2381` treat only `id` as once-decoded and look up keys case-sensitively | Match query keys case-insensitively and apply single-pass `?`-stripping (rejecting trailing `/`) to all scheme-less non-`//` paths for `id` and `fileUrl` (`U4`) |
| `0DE2E900` (`033-S` advisory): Case-insensitive `canonicalStreamTargetMatch` + negative tests | **CODE RESOLVED (`e25599a`) — EXPAND CHARACTERIZATION TESTS** | `meeting_chrome.go:440-492` folds `id`, `uniqueid`, `driveid` keys case-insensitively; `recap_test.go:4563-4565` covers 3 `isOnTargetMeetingOrRecapURL` cases | Add direct characterization tests (`ID`, `uniqueId`, `DRIVEID`, ambiguous duplicate keys, missing target key) in `recap_test.go` (`U4`) |
| `39450846` `U1-R3`: Discovery retry when outer page returns `""` | **PARTIALLY RESOLVED (`ed092e5`) — OPEN AS `U2-R5`** | `recap.go:2253` still gates `lastErr == nil` on `hasXplatTarget` | Remove `hasXplatTarget` gate, filter `hasStreamEmbedTarget` properly, and test 8-attempt exhaustion in `recap_retry_test.go` (`U3`) |
| `39450846` `U2-R3`: Reject Path B `decodeAudioData` when duration & `mseMaxBufferedEnd` are `0` | **ALREADY RESOLVED (`ed092e5`)** | `recap.go:1797-1800` throws `'recap audio MSE stream duration unverified'`; tested at `recap_test.go:3078,3570` | Preserve unchanged and record verification |
| `39450846` `U3-R3`: Fail on empty CDP chunk batch before `ChunkCount` | **ALREADY RESOLVED (`685e08b`)** | `recap.go:2178-2181` returns `incomplete recap audio chunk transfer`; tested at `recap_test.go:2816` | Preserve unchanged and record verification |
| `39450846` `U4-R3`: Align Path A worker concurrency (`8` vs `16`) | **SUPERSEDED BY `035-S` (`12` WORKERS)** | `035-S` (`024-DL`, `recap.go:1303`, `recap_test.go:4999`, `ARCHITECTURE.md:208`) deliberately scaled `runPathA` to `Math.min(12, totalSegments)` | Preserve `12` workers (`035-S` contract) and pair with `U1-R5` in-flight byte reservation (`U2`) |
| `39450846` `U5-R3`: Restrict `m.recapItemID` to `VroomItemID`/`VroomItemId`/`itemId`/`driveItemId` | **ALREADY RESOLVED (`ed092e5`)** | `meeting_chrome.go:1080` reads `rawPayload.DriveItemID`; tested at `recap_test.go:4049` | Preserve unchanged and record characterization verification (`U4`) |
| `39450846` `U6-R3`: Rank stream-incomplete/deadline errors above `isDownloadBlocked=true` in `errPriority` | **CODE RESOLVED — ADD CROSS-ATTEMPT RETRY TESTS** | `recap.go:1942-1955` ranks `"MSE stream"` and `"Path A capture deadline exceeded"` at `2` > `"isDownloadBlocked=true"` at `1` | Add Go unit tests in `recap_retry_test.go` verifying `errPriority` across attempt-0 -> attempt-1 `47D1E1C0` retry transitions (`U3`) |
| `7883544E` `U1-R5`: Reserve in-flight bytes per Path A worker against `maxRecapAudioSourceBytes` | **OPEN** | `recap.go:1003-1135` commits `streamSourceBytes += total` only after full segment body read completes | Add `inFlightSourceBytes` reservation per chunk / `Content-Length` with synchronous `fatalErr`/`abortCtrl.abort()` and `finally` release (`U2`) |
| `7883544E` `U2-R5`: Poll delayed `streamembed.aspx` mounts without `hasXplatTarget` | **OPEN** | `recap.go:2253` requires `(lastErr == nil && hasXplatTarget)` | Poll whenever `!hasStreamEmbedTarget && (lastErr == nil || isDownloadBlockedErr)` up to `maxAttempts` (`U3`) |
| `BA069506`: Qualify `PerformanceResourceTiming` window scope + Wave 2A docs | **OPEN** | `ARCHITECTURE.md:201` & `USER_GUIDE.md:331` do not note window-visible scope or decoupled `ai_summary_delay: 0s` | Update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` durably under `P-002` docs-only route (`U9`) |

### Prior Institutional Learnings Consulted (`docs/compound/`)

* `docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md`:
  Preserve `track !== 'video'` isolation (`wsd=96256` audio vs `wsd=96000`
  video), `g_fileInfo.EncKey` non-extractable WebCrypto import without setting
  `activityObserved`, SharePoint Copilot surface gating (`!isTeamsRecapSurface &&
  isSharePointStreamSurface`), and single-pass server-relative path handling.
* `docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`:
  Preserve 12-worker Path A concurrency, HTTP `408`/`429`/`5xx` 5-attempt
  backoff, `240000ms` production deadline, in-flight `pathAPromise` deadline
  precedence over partial MSE segments, `10m` `recapAudioEvalTimeout`, and Node
  test timer invariants (never clamping `ms >= 10000` `reqTimer` delays).
* `docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`:
  Preserve the `4 MiB` inline-vs-chunked threshold, persistent `cdpEvalSession`
  WebSocket reuse, empty-batch failure before `ChunkCount`, and
  `cleanupPendingTransfer` on all exit paths.
* `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`:
  Keep whole-token `[redacted]` CLI argument hygiene intact and never introduce
  live tenant URLs, UPNs, or tokens into tests or docs.

## Options Evaluated

### Option A: Cohesive Single Shipment (`036-S`, `priority: critical`) Decomposed into Nine 2-File Atomic Tasks (`036.001-T`..`036.009-T`)

Package all 7 Wave 2A stash entries under one covering feature (`036-F`) and one
`critical`-priority shipment (`036-S`, sequenced via `blocks` after shipped
`035-S` and independent of Wave 1's `high`-priority `010-S`), decomposed into 9
strictly 2-file, `< 5`-function, single-domain tasks:

* **Browser track (`036.001-T` -> `036.002-T` -> `036.003-T` -> `036.004-T`)**:
  four focused `internal/browser` tasks (`2` files each):
  (1) `extractRecapAudioJS` periodic `1000ms` Play re-clicking + replacement
  `<video>` mute/play safety + blocked non-blob skip (`recap.go` +
  `recap_test.go`);
  (2) concurrent Path A `inFlightSourceBytes` reservation across the 12 workers
  (`recap.go` + `recap_test.go`);
  (3) Go CDP `captureRecapAudioFromTargets` path-based `isStreamTarget`,
  `skipNonStreamTargets`, `sawRetryableStreamTargetErr` attempt-0 retry, and
  `xplatplugins.aspx`-independent delayed-mount discovery (`recap.go` +
  `recap_retry_test.go`); and
  (4) case-insensitive single-pass scheme-less/server-relative `%` identity
  decoding, `copyAISummaryJS` fresh-container guard, and
  `canonicalStreamTargetMatch`/`recapItemID` characterization coverage
  (`recap.go` + `recap_test.go`).
* **Config & CLI track — Expand -> Migrate Producers -> Contract Consumers
  (`036.005-T` -> `036.006-T` -> `036.007-T` -> `036.008-T`)**: four focused `XS`
  2-file tasks (`config.go`+`config_test.go`, `join.go`+`join_test.go`,
  `schedule.go`+`schedule_test.go`, and finally `capture.go`+`capture_test.go`)
  cleanly separating `Config.NoAISummary` from `AISummaryDelay == "0s"` across
  every CLI surface while keeping every intermediate commit behaviorally green.
* **Docs track (`036.009-T`, depends on `036.004-T` and `036.008-T`)**: one
  `P-002` verification-gated docs-only task (`docs/ARCHITECTURE.md` +
  `docs/USER_GUIDE.md`, `2` files, `harness-surface:none`).

* **Pros**: Every task touches strictly `2` files (`< 3` files), `1–4`
  functions (`< 5` functions, including inner JS functions), and `2–3` test
  scenarios (`< 4` scenarios) within a single Go package or `docs/` surface;
  expand-then-contract ordering keeps every intermediate commit 100% green;
  `P-004` `harness-architect` + `leaf-diff` always have the colocated `*_test.go`
  file in `--allowed-paths`; `priority: critical` orders `036-S` ahead of
  `010-S` in `backlogit_get_queue` without touching `010-S`.
* **Cons**: Slightly higher task count (9 small tasks) than bundling multiple
  concerns into larger tasks.
* **Effort estimate**: Medium (`4 × S` + `4 × XS` + `1 × S`).
* **Fit**: High — 100% compliant with Constitution Task Granularity, Width
  Isolation, `P-002`, `P-004`, and `P-021`.

### Option B: Monolithic 3-Task Bundle (One Browser Task, One Multi-Package Config+CLI Task, One Docs Task)

Combine all `internal/browser` changes into a single task (`recap.go`,
`meeting_chrome.go`, `recap_test.go`), all `internal/config` + `internal/cli`
changes into a single 6-file task (`config.go`, `capture.go`, `join.go`,
`schedule.go`, `capture_test.go`, `join_test.go`), and docs into a third task.

* **Pros**: Fewer task transitions during Ship.
* **Cons**: Violates the Constitution's NON-NEGOTIABLE 2-Hour Rule (`< 3` files,
  `< 5` functions, `< 4` test scenarios per task) on both code tasks, mixes
  `internal/config` and `internal/cli` packages in a single unit, and increases
  `harness-architect` RED-stub blast radius.
* **Effort estimate**: Medium, with high review/gate rejection risk.
* **Fit**: Rejected (violates 2-Hour Rule).

### Option C: Infer `--no-ai-summary` from `CaptureMode != past-recap && AISummaryDelay == "0s"` Without Adding `Config.NoAISummary`

Modify `isAISummaryDisabled` to return `false` only when `CaptureMode ==
"past-recap"`, leaving `Config` unchanged.

* **Pros**: Minimal diff in `internal/cli/capture.go`.
* **Cons**: Breaks `--no-ai-summary` when combined with `--mode past-recap`
  (`go-teams join <recap-url> --mode past-recap --no-ai-summary` would still
  extract the AI Summary, violating `47D1E1C0`!), and still wrongly disables
  Recap AI Summaries in default `capture_mode: "auto"` when `ai_summary_delay:
  "0s"` is set in `config.yaml`.
* **Effort estimate**: Low, but functionally broken.
* **Fit**: Rejected (fails `47D1E1C0` contract).

## Trade-off Comparison

| Criterion | Option A (9 Two-File Tasks + `Config.NoAISummary`) | Option B (3 Bundled Tasks) | Option C (`CaptureMode`-Only Heuristic) |
|---|---|---|---|
| 2-Hour Rule (`< 3` files, `< 5` funcs) | PASS (`2` files and `<= 4` funcs per task across all 9 tasks) | FAIL (6 files in CLI/config task; >5 funcs in browser task) | PASS |
| Width & Package Isolation | PASS (1 package/domain per task) | FAIL (`config` + `cli` mixed) | PASS |
| `47D1E1C0` Flag/Config Semantics | PASS (exact across `past-recap`, `auto`, `live-with-summary`, `ephemeral-live`, `--queue`, `schedule`) | PASS | FAIL (`--mode past-recap --no-ai-summary` and `auto` + `0s` broken) |
| `P-004` / `P-002` Gate Compatibility | PASS (colocated `*_test.go` in every code budget; full `P-002` docs record) | Moderate risk | PASS |

## Decision

Adopt **Option A**: create deliberation `027-DL`, generate and harden
`docs/plans/2026-11-20-wave2a-recap-audio-playback-retry-and-ai-summary-hardening-plan.md`,
pass multi-persona adversarial `plan-review` + `rubber-duck`, harvest covering
feature `036-F` and nine 2-file tasks (`036.001-T`..`036.009-T`), assemble
`critical`-priority shipment `036-S` sequenced via `blocks` on `035-S` (leaving
`010-S` and Wave 2B untouched), and archive the 7 consumed Wave 2A stash entries
with full provenance.

## Rejected Alternatives

* **Option B** was set aside because a 6-file cross-package `config`+`cli` task
  and a multi-concern browser task violate the workspace Constitution's
  non-negotiable `< 3` files / `< 5` functions / `< 4` test scenarios rule.
* **Option C** was set aside because `AISummaryDelay: "0s"` alone cannot
  distinguish `ai_summary_delay: "0s"` in `config.yaml` from an explicit
  `--no-ai-summary` CLI flag under `--mode past-recap` or `--mode auto`.
* **Reverting `runPathA` concurrency from `12` to `8` for `U4-R3`** was set
  aside because `035-S` (`024-DL`) intentionally raised concurrency to `12` for
  cold 60+ minute SharePoint recordings and live-verified it; pairing `12`
  workers with `U1-R5`'s `inFlightSourceBytes` reservation enforces the exact
  `256 MiB` memory cap across all 12 workers.

## Unresolved Questions

None blocking Wave 2A execution. Wave 2B (`E03CB3C3`, `4C40AF2B`, `B98262C8`)
remains active in `.backlogit/stash.jsonl` for separate deliberation after
`036-S` ships.

## Risks and Mitigations

* **Risk 1 — Periodic `'Play video'` re-clicking toggling pause on active
  playback**: Mitigated by gating button re-clicks strictly on `!activityObserved
  && !pathAActive` and spacing clicks by `>= 1000ms` (`Date.now() -
  lastBtnClickAt >= 1000`). The instant any transcode request, WebCrypto
  decryption, or MSE buffer append sets `activityObserved = true` (or Path A
  starts), button clicking ceases.
* **Risk 2 — In-flight byte reservation leaks on retryable HTTP `408`/`429`/`5xx`
  or timeout errors**: Mitigated by scoping `workerReserved` inside each
  `fetchBytesOnce` call and releasing any uncommitted `workerReserved` from
  `inFlightSourceBytes` inside `fetchBytesOnce`'s `finally` block (plus resetting
  `inFlightSourceBytes = 0` if Path A fails over to Path B).
* **Risk 3 — `U2-R5` (`lastErr == nil` discovery polling) slowing failure on
  mismatched owned frames**: Mitigated by recording priority-`0`
  `setLastErr(fmt.Errorf("browser: recap audio frame URL does not match requested recording"))`
  when an in-scope target URL conflicts with `expected` in
  `captureRecapAudioFromTargets`, so single-frame URL mismatches set `lastErr != nil`
  and fail fast on attempt `0` without entering delayed-mount polling while never
  masking an evaluated valid stream target's priority-`1` `isDownloadBlocked=true`
  error, and exposing `var recapAudioRetryDelay
  = 1 * time.Second` (matching `recapAudioEvalTimeout`) for fast deterministic
  exhaustion unit tests.
* **Risk 4 — Schedule per-meeting `ai_summary_delay` vs global `--no-ai-summary`
  precedence**: Mitigated in `buildScheduleEntryConfig` by clearing
  `entryCfg.NoAISummary = false` when a per-meeting entry explicitly configures a
  positive `AISummaryDelay` (`*e.AISummaryDelay > 0`), preserving the documented
  3-tier `schedule` override contract.
