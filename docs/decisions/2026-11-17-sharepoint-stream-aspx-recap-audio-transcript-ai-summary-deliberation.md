---
title: Direct SharePoint and OneDrive stream.aspx audio stream, transcript tab activation, and Copilot AI summary capture
description: Deliberation on stash entry D034C18B for covering feature 033-F and shipment 033-S
topic: Direct SharePoint/OneDrive stream.aspx Audio Stream, Transcript Tab Activation, and Copilot AI Summary Capture (033-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 022-DL
feature: 033-F
shipment: 033-S
source_stash:
  - D034C18B
linked_artifacts:
  - docs/plans/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-plan.md
  - .backlogit/queue/022-DL.md
  - .backlogit/queue/033-F.md
  - .backlogit/queue/033-S.md
date: 2026-11-17
status: accepted
tags:
  - audio
  - recap
  - browser
  - sharepoint
  - onedrive
  - stream-aspx
  - copilot
  - transcript
---

## Problem Frame

In shipment `031-S` (`020-DL`), `go-teams` added high-speed (`14x-100x` real-time) audio-only MP3 capture on download-blocked SharePoint Meeting Recaps (`window.g_fileInfo.isDownloadBlocked == true`) when `streamembed.aspx` is embedded inside a Teams Meeting Recap URL (`https://teams.microsoft.com/l/meetingrecap?...` or `https://teams.cloud.microsoft/l/meetingrecap?...`).

However, operators also configure and invoke `go-teams` directly against top-level SharePoint or OneDrive recording URLs (`https://<tenant>.sharepoint.com/personal/<user>/_layouts/15/stream.aspx?id=/personal/<user>/Documents/Recordings/<file>.mp4&share=...&referrer=StreamWebApp.Web&referrerScenario=...`), where the URL contains neither `"teams"` nor `"meetingrecap"`. Live verification against an authenticated SharePoint `stream.aspx` recording surfaced four coupled root causes recorded in stash entry **`D034C18B`** (`priority: high`, `kind: bug`):

1. **Audio stream capture fails after 10 seconds with `Error: recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)` (`internal/browser/recap.go`)**:
   * On `stream.aspx`, `window.g_fileInfo.isDownloadBlocked == true` and `window.g_fileInfo.downloadUrl` returns HTTP `403` / `text/html`.
   * Unlike `streamembed.aspx` embedded inside `teams.microsoft.com/l/meetingrecap` (which stays idle until `'Play video'` is clicked during `CaptureAudio`), the top-level `stream.aspx` player automatically initializes `MediaSource`, fetches and decrypts `part=initsegment`, and pre-buffers ~120 seconds of audio/video segments (`oneDrive.transcode`) during page load (`Join`), BEFORE `CaptureAudio` runs — and performs segment network fetches inside a Web Worker rather than `window.fetch` or `window.XMLHttpRequest`.
   * Consequently, hooks installed when `CaptureAudio` begins never observe `MediaSource.prototype.addSourceBuffer`, the `initsegment` `appendBuffer`, or the Worker's `oneDrive.transcode` requests (`transcodeAudioTemplateUrl = ''`, `transcodeWsd = 0`, `transcodePpd = 0`, `audioCryptoKeyInfo = null`, `mseInitSegment = null`, `activityObserved = false`), causing `captureStreamEmbedAudio()` to abort after `initialActivityTimeoutMs` (`10s`) with `isDownloadBlocked=true`.
   * Additionally, on SharePoint `oneDrive.transcode`, `track=audio&quality=audcopy` uses `wsd=96256` (`94 * 1024` AAC samples at 16 kHz) whereas `track=video&quality=vcopy` uses `wsd=96000`. In `inspectTranscodeRequest`, `if (Number.isFinite(wsd) && wsd > 0) transcodeWsd = wsd;` and `if (!transcodeAudioTemplateUrl || track === 'audio')` allow a `track=video` request to seed or overwrite `transcodeWsd` and `transcodeAudioTemplateUrl` with `wsd=96000`, which `oneDrive.transcode` rejects for `track=audio` segments (`segmentTime = k * 96256`).
2. **Headless Transcript pane activation on `stream.aspx` (`internal/browser/recap.go`)**:
   * On `stream.aspx`, the right-hand sidebar opens to the **Copilot** pane by default (`button[aria-label="Close Copilot"]`), so `.ms-List` is not mounted in the DOM until the `Transcript` button (`[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]`) is clicked.
3. **SharePoint `stream.aspx` Copilot AI summary generation and extraction (`internal/browser/recap.go`)**:
   * On `stream.aspx`, instead of Teams Recap `Notes` -> `AINotes` -> `button[aria-label="Copy all"]`, the page provides a Copilot sidebar with a `"Recap the meeting"` prompt button that generates the AI summary inside `.fai-CopilotMessage` (`.fai-CopilotMessage__content`) with `.fai-CopilotMessage__actions button[title="Copy"]` once generation completes (~12.5 seconds).
   * The existing 20-attempt (`10s`) polling budget in `captureAISummaryFromTargets` is shorter than the ~12.5-second Copilot generation window on `stream.aspx`.
4. **SharePoint `stream.aspx` recording identity and target classification (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, integration tests, docs)**:
   * On `stream.aspx`, the recording path is passed in the `id` query parameter (`?id=/personal/<user>/Documents/Recordings/<file>.mp4`) rather than `fileUrl`, and `hasSpecificMeetingOrRecapTarget`, `isOnTargetMeetingOrRecapURL`, `recapIntegrationURL`, `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, and `TestLiveTenantDelayedAISummaryQueueReplay` check only for `meetingrecap` / `threadId` rather than `stream.aspx` and `streamembed.aspx`.

### P-021 C5 and C6 Triage Record

* `D034C18B` does not carry the `DEFERRED SCOPE EXPANSION` marker; it is a high-priority bug entry (`kind: bug`, `priority: high`).
* **Obligation (A) Unconditional duplicate detection**: Executed across `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`. Outcome: **CLEAN DUPLICATE SCAN** for `D034C18B`. Archived shipment `031-S` (`9A16B213`, `020-DL`) implemented high-speed `streamembed.aspx` audio capture inside Teams `meetingrecap` URLs, and active stash entries `39450846` and `7883544E` track unrelated minor R3/R5 review follow-ups from `031-S`. No other active or archived item addresses top-level SharePoint/OneDrive `stream.aspx` `PerformanceResourceTiming` + `g_fileInfo.EncKey` seeding, `track !== 'video'` `wsd` isolation, `stream.aspx` Transcript tab activation, SharePoint Copilot `.fai-CopilotMessage` AI summary capture, or `id`-parameter `RecordingName` identity matching.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (no `N/A` deferred-expansion source refs).

### Scope Boundaries

* **In scope**:
  * **U1 (`033.001-T` — `code` domain: `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, `Size: M`, `Complexity: medium`, 2 files, test-first with Wave-Staged Red-Phase Harnesses staged in `logs/staged-harnesses/033.002-T/` and `logs/staged-harnesses/033.003-T/`)**:
    * In `extractRecapAudioJS` (`internal/browser/recap.go`): recognize `stream.aspx` in `hasPlayableStreamSurface`, `frameScore`, and `captureRecapAudioFromTargets` (`recapTargetRank` / `hasStreamEmbedTarget`), and include `[role="menuitem"][aria-label="Play"]` in `playBtnSelector`; guard `inspectTranscodeRequest` so `track === 'video'` never seeds or overwrites `transcodeWsd` or `transcodeAudioTemplateUrl`; add `seedEncKeyFromFileInfo()` to import `window.g_fileInfo.EncKey` (`Key` and `IV` base64 JSON) via `crypto.subtle.importKey('raw', keyBytes, { name: 'AES-CBC' }, false, ['decrypt'])` into `audioCryptoKeyInfo`; and add `scanPerformanceTranscodeEntries(w)` using `w.performance.getEntriesByType('resource')` both at the start of `captureStreamEmbedAudio()` and inside the polling loop while `!transcodeAudioTemplateUrl`.
    * In `extractRecapTranscriptJS` (`internal/browser/recap.go`): when `cells.length === 0`, click `[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]` once (`!txBtn.dataset.goTeamsTranscriptClicked`) so `.ms-List` mounts on `stream.aspx`.
    * In `copyAISummaryJS` and `captureAISummaryFromTargets` (`internal/browser/recap.go`): preserve all existing `data-tid="Notes"`, `data-tid="AINotes"`, `aria-label="Copy all"`, and `navigator.clipboard` code (preserving `TestCopyAISummaryJSStructure`'s `NotContains(copyAISummaryJS, 'aria-label*="Copy"')` invariant); when `button[aria-label="Copy all"]` is absent, extract completed SharePoint Copilot summary text from `.fai-CopilotMessage` (`.fai-CopilotMessage__actions button[title="Copy"], .fai-CopilotMessage__actions button[aria-label="Copy"]` -> `.fai-CopilotMessage__content`), or when Teams Recap tabs/headers (`Notes`, `AINotes`, `[data-tid="intelligent-recap-header"]`) are absent, open the SharePoint Copilot pane (`[role="menuitem"][aria-label="Copilot"], button[aria-label="Copilot"]` when `!document.querySelector('button[aria-label="Close Copilot"]')`) and click the `"Recap the meeting"` prompt button once (`!recapBtn.dataset.goTeamsRecapClicked`); and increase `captureAISummaryFromTargets` polling from `20` attempts (`10s`) to `60` attempts (`30s` at `500ms` interval, bounded by `ctx.Err()`).
    * In `ExtractMeetingIdentity` / `applyQueryIdentity`, `checkIdentityAgainst`, `hasSpecificMeetingOrRecapTarget`, and `isOnTargetMeetingOrRecapURL` (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`): extract `RecordingName` from both `fileUrl` and `id` query parameters when the value is a path ending in `.mp4`, `.mkv`, `.webm`, `.mov`, or `.m4a` (ignoring non-media `id` values such as `?id=222` or `/personal/.../:v:/...`), treat `stream.aspx`, `streamembed.aspx`, and `id.RecordingName != ""` as specific recap targets, and enforce conflict-checked `hasRecapAudioIdentityMatch` / `RecordingName` matching in `isOnTargetMeetingOrRecapURL`.
  * **U2 (`033.002-T` — `test` domain: `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`, `Size: S`, `Complexity: low`, depends on `033.001-T`, 3 files)**:
    * Add unit tests in `internal/browser/recap_test.go` covering `stream.aspx` `PerformanceResourceTiming` + `g_fileInfo.EncKey` seeding, `track=video` (`wsd=96000`) isolation from `track=audio` (`wsd=96256`), `stream.aspx` Transcript tab activation, SharePoint Copilot `.fai-CopilotMessage` AI summary generation/extraction, and `stream.aspx?id=...mp4` `RecordingName` identity matching and conflict rejection.
    * Add a real headless Chromium E2E test in `internal/browser/meeting_e2e_test.go` simulating a top-level SharePoint `stream.aspx?id=/personal/.../Recap.mp4` page (where Web Worker pre-buffering populates `performance.getEntriesByType('resource')` and `g_fileInfo.EncKey` before `CaptureAudio`, `.ms-List` mounts only after clicking the `Transcript` menuitem, and Copilot `"Recap the meeting"` renders `.fai-CopilotMessage` asynchronously).
    * Update `recapIntegrationURL` in `internal/browser/scheduled_lifecycle_integration_test.go` to accept `stream.aspx` and `streamembed.aspx` URLs alongside `meetingrecap` and `19:meeting_`.
  * **U3 (`033.003-T` — `test` domain: `internal/cli/scheduled_integration_test.go`, `Size: XS`, `Complexity: low`, depends on `033.001-T` and `033.002-T`, 1 file)**:
    * Update `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` and `TestLiveTenantDelayedAISummaryQueueReplay` in `internal/cli/scheduled_integration_test.go` to recognize `stream.aspx` and `streamembed.aspx` alongside `meetingrecap`, pass `--audio` in `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`'s Recap branch, and assert that a non-empty valid `.mp3` file with an MPEG frame sync header (`0xFF`, `(b[1]&0xE0) == 0xE0`) is written alongside the `.md` transcript + `# AI Summary`.
  * **U4 (`033.004-T` — `docs` domain: `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `Size: XS`, `Complexity: trivial`, depends on `033.001-T`, 2 files)**:
    * Update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` to document direct SharePoint/OneDrive `stream.aspx` URL support for transcript tab activation, Copilot `"Recap the meeting"` AI summary extraction, and `PerformanceResourceTiming` + `g_fileInfo.EncKey` high-speed MP3 audio capture.
* **Out of scope**:
  * All other active stash entries (`9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, `4AB8EAEE`), which remain untouched in `active` stash state.
  * External media tools (`ffmpeg`, `yt-dlp`), CGO, or new `go.mod` dependencies.

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`**: Multi-megabyte PCM payloads (`totalPCMBytes > 4 MiB`, such as a 62.5-minute `stream.aspx` recording producing 2,527 base64 chunks) must be staged on `window.__goTeamsRecapAudioTransfer` and pulled in 64-chunk batches over a single persistent WebSocket session (`newCDPEvalSession`) to prevent `Runtime.evaluate` serialization stalls and Windows loopback ephemeral port exhaustion (`WSAEADDRINUSE`).
* **`docs/compound/build-errors/shine-mp3-frame-alignment.md` & `docs/compound/build-errors/shine-mp3-race-checkptr.md`**: All decoded 44.1 kHz mono `int16` PCM chunks must pass through `audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` + `Flush()`), which preserves 1152-sample frame alignment and `encoderTailSlack = 4` stereo slice capacity under `go test -race`.

### Codebase and Live `stream.aspx` Root Cause Analysis

1. **Web Worker Pre-Buffering and `g_fileInfo.EncKey` on `stream.aspx` (`internal/browser/recap.go`)**:
   * On top-level `stream.aspx`, SharePoint's video player starts playing/buffering during `m.Join(ctx)` (while `go-teams` extracts the transcript and polls for the AI Summary) and performs `oneDrive.transcode` segment fetches inside a dedicated Web Worker rather than calling `window.fetch` or `window.XMLHttpRequest` on the main thread.
   * By the time `m.CaptureAudio(ctx, w)` evaluates `extractRecapAudioJS`, the browser has already buffered ~120 seconds of audio/video and `window.fetch` / `window.XMLHttpRequest` / `window.crypto.subtle.decrypt` on the main window see zero calls.
   * However, Chromium records every Worker and main-thread `oneDrive.transcode` request in the window's `performance.getEntriesByType('resource')` buffer, and `window.g_fileInfo.EncKey` holds the recording's AES-CBC key material as a JSON string (`{"Key":"<base64-16B>","IV":"<base64-16B>"}`).
   * Seeding `audioCryptoKeyInfo` from `window.g_fileInfo.EncKey` via `origPlayerSubtle.importKey('raw', keyBytes, { name: 'AES-CBC' }, false, ['decrypt'])` and scanning `w.performance.getEntriesByType('resource')` via `scanPerformanceTranscodeEntries(w)` immediately populates `transcodeAudioTemplateUrl`, `transcodeWsd`, `transcodePpd`, and `audioCryptoKeyInfo` so Path A runs even when all initial transcode requests occurred in a Web Worker prior to `CaptureAudio`.
2. **Audio vs. Video `wsd` Isolation (`internal/browser/recap.go`)**:
   * SharePoint's `oneDrive.transcode` endpoint uses different window segment durations (`wsd`) per track: `track=audio&quality=audcopy` uses `wsd=96256` (`94 * 1024` AAC samples at 16 kHz = `6.016s`), whereas `track=video&quality=vcopy` uses `wsd=96000` (`6.000s`).
   * Currently, `inspectTranscodeRequest` in `internal/browser/recap.go` unconditionally overwrites `transcodeWsd = wsd` (`if (Number.isFinite(wsd) && wsd > 0) transcodeWsd = wsd;`) and seeds `transcodeAudioTemplateUrl` from `track=video` (`if (!transcodeAudioTemplateUrl || track === 'audio')`). When a `track=video` entry is inspected after (or instead of) `track=audio`, `transcodeWsd` and the template URL's `wsd` query parameter become `96000`, causing `oneDrive.transcode` to fail on `track=audio` segment requests. Guarding both assignments with `if (track !== 'video')` eliminates this cross-track contamination.
3. **Transcript Tab Activation and Copilot AI Summary on `stream.aspx` (`internal/browser/recap.go`)**:
   * On `stream.aspx`, the right-hand sidebar defaults to the Copilot pane (`button[aria-label="Close Copilot"]`), leaving `.ms-List` unmounted until `[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]` is clicked. Clicking that button idempotently (`txBtn.dataset.goTeamsTranscriptClicked = 'true'`) when `cells.length === 0` mounts `.ms-List` for the next poll cycle.
   * For AI Summary capture on `stream.aspx`, the Copilot sidebar provides a `"Recap the meeting"` prompt button that generates the summary into `.fai-CopilotMessage` (`.fai-CopilotMessage__content`) with `.fai-CopilotMessage__actions button[title="Copy"]` after ~12.5 seconds. Supporting this flow in `copyAISummaryJS` (while keeping `TestCopyAISummaryJSStructure`'s `NotContains(copyAISummaryJS, 'aria-label*="Copy"')` invariant intact) and extending `captureAISummaryFromTargets` from 20 attempts (`10s`) to 60 attempts (`30s` at `500ms` interval) enables full headless AI Summary capture on `stream.aspx`.
4. **Recording Identity from `stream.aspx?id=...` (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`)**:
   * Direct `stream.aspx` URLs encode the server-relative recording path in the `id` query parameter (`?id=/personal/<user>/Documents/Recordings/<file>.mp4`). Extracting `RecordingName` from both `fileUrl` and `id` when the path ends in `.mp4`, `.mkv`, `.webm`, `.mov`, or `.m4a` (and contains no `/:` sharing prefix) allows `ExtractMeetingIdentity`, `checkIdentityAgainst`, `hasSpecificMeetingOrRecapTarget`, and `isOnTargetMeetingOrRecapURL` to verify recording identity and prevent cross-recording contamination on `stream.aspx`.

## Options Evaluated

### Option A: In-Page `PerformanceResourceTiming` + `g_fileInfo.EncKey` Seeding, `track !== 'video'` `wsd` Guard, Idempotent `stream.aspx` Transcript Tab Activation, Copilot `.fai-CopilotMessage` AI Summary Extraction, and `id`-Parameter `RecordingName` Identity Matching (Recommended)

* **Description**:
  * **U1 (`033.001-T` — `code` domain: `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, `Size: M`, `Complexity: medium`, 2 files)**: Implement all four verified fixes in `internal/browser/recap.go` and `internal/browser/meeting_chrome.go`.
  * **U2 (`033.002-T` — `test` domain: `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`, `Size: S`, `Complexity: low`, depends on `033.001-T`, 3 files)**: Add deterministic unit tests in `recap_test.go`, a real headless Chromium E2E test in `meeting_e2e_test.go`, and `stream.aspx` URL recognition in `scheduled_lifecycle_integration_test.go`.
  * **U3 (`033.003-T` — `test` domain: `internal/cli/scheduled_integration_test.go`, `Size: XS`, `Complexity: low`, depends on `033.001-T` and `033.002-T`, 1 file)**: Update CLI live-tenant integration tests (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` and `TestLiveTenantDelayedAISummaryQueueReplay`) to recognize `stream.aspx` / `streamembed.aspx` and assert valid MP3 audio + Markdown transcript + `# AI Summary` capture.
  * **U4 (`033.004-T` — `docs` domain: `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `Size: XS`, `Complexity: trivial`, depends on `033.001-T`, 2 files)**: Update architecture and user guide documentation.
* **Pros**:
  * Live-verified against a 62.5-minute (`3753.98s`, 624 segments, `34,851,221` source bytes) SharePoint `stream.aspx` recording, capturing all 325+ transcript entries, the Copilot AI summary, and valid MP3 audio in ~41 seconds (`transferPending: true`, `chunkCount: 2527`).
  * Works when `oneDrive.transcode` requests are issued by a Web Worker before `CaptureAudio` starts.
  * Preserves all existing `teams.microsoft.com/l/meetingrecap` and `streamembed.aspx` behavior and unit/E2E tests.
  * Zero external dependencies (`go.mod` unchanged).
* **Cons**:
  * Increases the maximum `captureAISummaryFromTargets` polling window from 10 seconds to 30 seconds when no AI summary is present on the page (bounded by `ctx.Err()`).
* **Effort estimate**: Medium (4 atomic, width-isolated tasks: `M`, `S`, `XS`, `XS`).
* **Fit**: 100% alignment with stash `D034C18B` and live-tenant verification requirements.

### Option B: Rewrite Direct `stream.aspx` URLs to `teams.microsoft.com/l/meetingrecap`

* **Description**: Attempt to synthesize a `teams.microsoft.com/l/meetingrecap` URL from a SharePoint `stream.aspx` URL before navigating.
* **Pros**: Would reuse the Teams Recap shell if a `threadId` were available.
* **Cons**: Direct SharePoint/OneDrive `stream.aspx?id=/personal/<user>/Documents/Recordings/<file>.mp4` sharing links do not contain a Teams `threadId` or `DriveItemID` in the URL, and standalone OneDrive/SharePoint recordings may not be associated with a Teams meeting chat at all.

### Option C: Force a Full Page Reload Inside `CaptureAudio` to Re-Trigger Player Initialization

* **Description**: Inject hooks via `Page.addScriptToEvaluateOnNewDocument` and reload `stream.aspx` during `CaptureAudio`.
* **Pros**: Hooks run before page load.
* **Cons**: A full SharePoint `stream.aspx` reload adds 10-20 seconds of latency, still fails to intercept `oneDrive.transcode` requests made inside a dedicated Web Worker (`window.fetch` is not shared with Worker global scope), and still fails unless `track=video` (`wsd=96000`) is isolated from `track=audio` (`wsd=96256`).

## Trade-off Comparison

| Criterion | Option A (`PerformanceResourceTiming` + `g_fileInfo.EncKey` + `track !== 'video'` + Transcript/Copilot DOM) | Option B (Rewrite `stream.aspx` to `meetingrecap`) | Option C (Reload Page in `CaptureAudio`) |
|---|---|---|---|
| Works on standalone SharePoint `stream.aspx?id=...mp4` URLs without `threadId` | Yes | No (lacks `threadId`) | Partial |
| Captures `oneDrive.transcode` URLs fetched inside a Web Worker before `CaptureAudio` | Yes (`performance.getEntriesByType('resource')` + `g_fileInfo.EncKey`) | N/A | No (Worker has separate `fetch` global) |
| Prevents `track=video` (`wsd=96000`) from corrupting `track=audio` (`wsd=96256`) | Yes (`track !== 'video'` guard) | No | No |
| Activates `stream.aspx` Transcript tab and Copilot `"Recap the meeting"` summary | Yes | N/A | No |
| Extra page reload latency | Zero | N/A | +10-20s per capture |

## Decision

We select **Option A**: promote stash entry **`D034C18B`** to covering feature **`033-F`** and decompose it into four width-isolated tasks (`033.001-T`..`033.004-T`) assembled into shipment **`033-S`**, sequenced in the ready set before `010-S` (`032-S -> 033-S -> 010-S`).

## Rejected Alternatives

* **Option B** was rejected because direct SharePoint/OneDrive `stream.aspx` URLs carry only the server-relative file path (`?id=/personal/.../<file>.mp4`) without a Teams `threadId`, making a `meetingrecap` URL rewrite impossible.
* **Option C** was rejected because `stream.aspx` fetches transcode segments inside a Web Worker (whose `fetch` is not intercepted by main-window `window.fetch` hooks) and reloading the page wastes 10-20 seconds while risking SharePoint throttling.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Risk — `track=video` (`wsd=96000`) overwriting `track=audio` (`wsd=96256`) when scanning `PerformanceResourceTiming` or live requests**:
  * **Mitigation**: In `inspectTranscodeRequest`, guard both `transcodeWsd = wsd` and `transcodeAudioTemplateUrl = tmpl.toString()` with `if (track !== 'video')`. `transcodePpd` (total recording duration in 100-nanosecond ticks) is identical across audio and video tracks and remains updated on any valid transcode entry.
* **Risk — Malformed or missing `window.g_fileInfo.EncKey` on unencrypted or non-standard streams**:
  * **Mitigation**: Wrap `seedEncKeyFromFileInfo()` in a defensive `try ... catch` block that checks `typeof info.EncKey === 'string'`, parses JSON, validates `keyBytes.byteLength === 16 && ivBytes.byteLength === 16`, and calls `origPlayerSubtle.importKey('raw', keyBytes, { name: 'AES-CBC' }, false, ['decrypt'])` only when `!audioCryptoKeyInfo`. If `EncKey` is absent or invalid, Path A falls back to the `crypto.subtle.decrypt` hook or Path B MSE capture without throwing.
* **Risk — Repeatedly clicking the `Transcript` tab button or `"Recap the meeting"` Copilot prompt across polling iterations**:
  * **Mitigation**: Mark clicked elements with `dataset.goTeamsTranscriptClicked = 'true'` and `dataset.goTeamsRecapClicked = 'true'` so each button is clicked at most once per page load, and in `extractRecapTranscriptJS` click the `Transcript` button only when `cells.length === 0`.
* **Risk — Regressing `TestCopyAISummaryJSStructure` (`NotContains(copyAISummaryJS, 'aria-label*="Copy"')`)**:
  * **Mitigation**: Use exact attribute selectors `.fai-CopilotMessage__actions button[title="Copy"], .fai-CopilotMessage__actions button[aria-label="Copy"]` and never use substring selector `aria-label*="Copy"`.
* **Risk — False-positive `RecordingName` extraction from non-media `id` query parameters (e.g., `?id=222` or SharePoint sharing prefix `/:v:/`)**:
  * **Mitigation**: Only extract `RecordingName` from `fileUrl` or `id` when the parsed path does not contain `/:` and its basename has a lowercase extension in `{.mp4, .mkv, .webm, .mov, .m4a}`.
