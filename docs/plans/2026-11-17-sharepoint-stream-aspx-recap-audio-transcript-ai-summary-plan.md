---
title: Direct SharePoint and OneDrive stream.aspx audio stream, transcript tab activation, and Copilot AI summary capture implementation plan
description: Implementation plan for 033-F / 033-S covering stash entry D034C18B
source: docs/decisions/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-deliberation.md
deliberation_id: 022-DL
feature: 033-F
shipment: 033-S
source_stash:
  - D034C18B
date: 2026-11-17
status: approved
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

Following the accepted deliberation in [2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-deliberation.md](../decisions/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-deliberation.md) (`022-DL`), stash entry **`D034C18B`** (`priority: high`, `kind: bug`) is planned under covering feature **`033-F`** and shipment **`033-S`**, sequenced immediately after `032-S` and before `010-S` in the shipment dependency DAG **`032-S -> 033-S -> 010-S`**.

When `go-teams` (or `.env.local.test` during `-tags=integration` verification) is configured with a direct SharePoint or OneDrive video recording URL (`https://<tenant>.sharepoint.com/personal/<user>/_layouts/15/stream.aspx?id=/personal/<user>/Documents/Recordings/<file>.mp4&share=...&referrer=StreamWebApp.Web&referrerScenario=...`), the URL has no `"teams"` or `"meetingrecap"` token, and four coupled root causes prevent end-to-end transcript, AI summary, and MP3 audio capture:

1. **Web Worker pre-buffering, `g_fileInfo.EncKey` key material, and `track=video` (`wsd=96000`) overwriting `track=audio` (`wsd=96256`) in `extractRecapAudioJS` (`internal/browser/recap.go`)**:
   * On `stream.aspx`, `window.g_fileInfo.isDownloadBlocked == true` and `window.g_fileInfo.downloadUrl` returns HTTP `403` / `text/html`.
   * Unlike `streamembed.aspx` embedded inside `teams.microsoft.com/l/meetingrecap` (which stays idle until `'Play video'` is clicked during `CaptureAudio`), the top-level `stream.aspx` player automatically initializes `MediaSource`, fetches and decrypts `part=initsegment`, and pre-buffers ~120 seconds of audio/video segments (`oneDrive.transcode`) during page load (`Join`), BEFORE `CaptureAudio` runs — and performs segment fetches inside a dedicated Web Worker rather than `window.fetch` or `window.XMLHttpRequest`.
   * Consequently, main-window hooks installed when `CaptureAudio` begins never observe `MediaSource.prototype.addSourceBuffer`, the `initsegment` `appendBuffer`, or the Worker's `oneDrive.transcode` requests (`transcodeAudioTemplateUrl = ''`, `transcodeWsd = 0`, `transcodePpd = 0`, `audioCryptoKeyInfo = null`, `mseInitSegment = null`, `activityObserved = false`), causing `captureStreamEmbedAudio()` to abort after `initialActivityTimeoutMs` (`10s`) with `isDownloadBlocked=true`.
   * Furthermore, on SharePoint `oneDrive.transcode`, `track=audio&quality=audcopy` uses `wsd=96256` (`94 * 1024` AAC samples at 16 kHz) whereas `track=video&quality=vcopy` uses `wsd=96000`. In `inspectTranscodeRequest`, `if (Number.isFinite(wsd) && wsd > 0) transcodeWsd = wsd;` and `if (!transcodeAudioTemplateUrl || track === 'audio')` allow a `track=video` request to seed or overwrite `transcodeWsd` and `transcodeAudioTemplateUrl` with `wsd=96000`, which `oneDrive.transcode` rejects for `track=audio` segments (`segmentTime = k * 96256`).
2. **Headless Transcript pane activation on `stream.aspx` (`internal/browser/recap.go`)**:
   * On `stream.aspx`, the right-hand sidebar opens to the **Copilot** pane by default (`button[aria-label="Close Copilot"]`), so `.ms-List` is not mounted until the `Transcript` button (`[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]`) is clicked.
3. **SharePoint `stream.aspx` Copilot AI summary generation and extraction (`internal/browser/recap.go`)**:
   * On `stream.aspx`, instead of Teams Recap `Notes` -> `AINotes` -> `button[aria-label="Copy all"]`, the page provides a Copilot sidebar with a `"Recap the meeting"` prompt button that generates the AI summary inside `.fai-CopilotMessage` (`.fai-CopilotMessage__content`) with `.fai-CopilotMessage__actions button[title="Copy"]` once generation completes (~12.5 seconds, exceeding the existing 20-attempt / `10s` polling budget in `captureAISummaryFromTargets`).
4. **SharePoint `stream.aspx` recording identity and target classification (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, integration tests, docs)**:
   * On `stream.aspx`, the recording path is carried in the `id` query parameter (`?id=/personal/<user>/Documents/Recordings/<file>.mp4`) rather than `fileUrl`. `applyQueryIdentity`, `checkIdentityAgainst`, `hasSpecificMeetingOrRecapTarget`, `isOnTargetMeetingOrRecapURL`, `recapIntegrationURL`, `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, and `TestLiveTenantDelayedAISummaryQueueReplay` must recognize `stream.aspx` / `streamembed.aspx` and extract `RecordingName` from media-path `id` query parameters.

### Explicit Out-of-Scope Boundaries

* Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, and `4AB8EAEE` remain untouched in `active` stash state.
* No external binaries (`ffmpeg`, `yt-dlp`), CGO, or new `go.mod` dependencies are introduced.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: In `extractRecapAudioJS` and `captureRecapAudioFromTargets` (`internal/browser/recap.go`), seed `transcodeAudioTemplateUrl`, `transcodeWsd`, and `transcodePpd` from `performance.getEntriesByType('resource')` (`scanPerformanceTranscodeEntries`), seed AES-CBC `audioCryptoKeyInfo` from `window.g_fileInfo.EncKey` via `crypto.subtle.importKey('raw', ...)` (`seedEncKeyFromFileInfo`), guard `inspectTranscodeRequest` so `track=video` requests (`wsd=96000`) never overwrite or seed `track=audio` `wsd` (`96256`) or `transcodeAudioTemplateUrl`, and recognize `stream.aspx` and `[role='menuitem'][aria-label='Play']` as a playable stream surface | `D034C18B` | U1 (`033.001-T`), U2 (`033.002-T`) | `go test -race ./internal/browser/... -run "TestSharePointStreamAspx|TestChromeSharePointStreamAspx"` |
| R2: In `extractRecapTranscriptJS`, `copyAISummaryJS`, and `captureAISummaryFromTargets` (`internal/browser/recap.go`), click the SharePoint `stream.aspx` `Transcript` button (`[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]`) once when `.ms-List` is not yet mounted (`cells.length === 0`), and support SharePoint `stream.aspx` Copilot `"Recap the meeting"` AI summary generation and extraction from `.fai-CopilotMessage` (`.fai-CopilotMessage__content` / `.fai-CopilotMessage__actions button[title="Copy"]`) with a 60-attempt (`30s`) polling budget while preserving `TestCopyAISummaryJSStructure` | `D034C18B` | U1 (`033.001-T`), U2 (`033.002-T`) | `go test -race ./internal/browser/... -run "TestCopyAISummaryJSStructure|TestSharePointStreamAspx|TestChromeSharePointStreamAspx"` |
| R3: In `ExtractMeetingIdentity` / `applyQueryIdentity`, `checkIdentityAgainst`, `hasSpecificMeetingOrRecapTarget`, and `isOnTargetMeetingOrRecapURL` (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`), extract `RecordingName` from `stream.aspx` `id` query parameter paths ending in `.mp4`/`.mkv`/`.webm`/`.mov`/`.m4a` (ignoring non-media `id` values and `/:` sharing prefixes) and treat `stream.aspx` and `streamembed.aspx` as specific recap targets with conflict-checked `RecordingName` matching | `D034C18B` | U1 (`033.001-T`), U2 (`033.002-T`) | `go test -race ./internal/browser/... -run "TestExtractMeetingIdentity|TestSharePointStreamAspx"` |
| R4: Update unit, E2E, and live-integration tests (`internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`, `internal/cli/scheduled_integration_test.go`) and docs (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`) so direct SharePoint/OneDrive `stream.aspx` URLs capture transcript, AI summary, and MP3 audio end-to-end | `D034C18B` | U2 (`033.002-T`), U3 (`033.003-T`), U4 (`033.004-T`) | `go test -race ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run` |

## Implementation Units

### U1 (`033.001-T`) — Support SharePoint `stream.aspx` audio transcode seeding, transcript tab activation, Copilot AI summary, and `id` recording identity (`internal/browser`)

* **Domain:** code (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`). **Posture:** test-first (Principle II — Wave-Staged Red-Phase Harness: stage U2 red-phase tests in `logs/staged-harnesses/033.002-T/` and U3 red-phase tests in `logs/staged-harnesses/033.003-T/` during U1 red-phase verification to prove they fail before editing `internal/browser/recap.go` and `internal/browser/meeting_chrome.go`, then activate in U2 and U3). **Size:** `M` | **Complexity:** `medium`.
* **Files (2 files, 4 cohesive functional groups — audio stream seeding, transcript tab activation, Copilot AI summary extraction, and `stream.aspx` recording identity/target matching — kept together in `internal/browser` to avoid intermediate broken states across coupled JS/Go symbols):** `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`.
* **What:**
  1. **In `internal/browser/recap.go` (`extractRecapAudioJS`)**:
     * In `checkIdentityAgainst(raw, baseUrl, state)`:
       * Check both `'fileUrl'` and `'id'` query parameters for media recording paths:
         ```javascript
         for (const paramKey of ['fileUrl', 'id']) {
             for (const value of params.getAll(paramKey)) {
                 try {
                     const filePath = new URL(value, baseUrl).pathname;
                     if (!filePath.includes('/:') && /\.(mp4|mkv|webm|mov|m4a)$/i.test(filePath)) {
                         matchField('RecordingName', decodeURIComponent(filePath.split('/').pop()), true, state);
                     }
                 } catch (_) {
                     if (paramKey === 'fileUrl') throw new Error('recap audio recording identity mismatch');
                 }
             }
         }
         ```
     * In `playBtnSelector`: add `[role="menuitem"][aria-label="Play"]`:
       ```javascript
       const playBtnSelector = 'button[aria-label="Play video"], button[title="Play video"], button[data-automationid="playButton"], button[aria-label="Play"], [role="menuitem"][aria-label="Play"], .vjs-big-play-button, [role="button"][aria-label="Play video"]';
       ```
     * In `hasPlayableStreamSurface(ctx)` and `frameScore(ctx)`: recognize `urlLower.includes('streamembed.aspx') || urlLower.includes('stream.aspx')`.
     * In `inspectTranscodeRequest(rawUrl, headersObj, hookWin)`:
       * Guard `transcodeWsd` and `transcodeAudioTemplateUrl` updates with `track !== 'video'` so `track=video` (`wsd=96000`) never seeds or overwrites `track=audio` (`wsd=96256`):
         ```javascript
         if (track !== 'video' && Number.isFinite(wsd) && wsd > 0) transcodeWsd = wsd;
         if (Number.isFinite(ppd) && ppd > 0) {
             transcodePpd = ppd;
             const ppdSec = ppd >= 1e6 ? (ppd / 1e7) : ppd;
             if (Number.isFinite(ppdSec) && (ppdSec > maxRecapAudioDurationSec || Math.ceil(ppdSec * 44100) * 2 > maxRecapAudioPCMBytes)) {
                 fatalErr = new Error(limitError);
                 if (abortCtrl) abortCtrl.abort();
                 signalWake();
             }
         }
         if (track !== 'video' && (!transcodeAudioTemplateUrl || track === 'audio')) {
             const tmpl = new URL(u.toString());
             tmpl.searchParams.set('track', 'audio');
             tmpl.searchParams.set('quality', 'audcopy');
             tmpl.searchParams.delete('segmentTime');
             tmpl.searchParams.delete('part');
             transcodeAudioTemplateUrl = tmpl.toString();
         }
         ```
     * In `captureStreamEmbedAudio()`:
       * Add `seedEncKeyFromFileInfo()`:
         ```javascript
         async function seedEncKeyFromFileInfo() {
             if (audioCryptoKeyInfo || !info || typeof info.EncKey !== 'string' || !info.EncKey.trim()) return;
             if (!origPlayerSubtle || typeof origPlayerSubtle.importKey !== 'function') return;
             try {
                 const parsed = JSON.parse(info.EncKey);
                 if (!parsed || typeof parsed.Key !== 'string' || typeof parsed.IV !== 'string') return;
                 const decodeB64 = (s) => {
                     if (typeof atob === 'function') {
                         const bin = atob(s);
                         const out = new Uint8Array(bin.length);
                         for (let i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
                         return out;
                     }
                     return new Uint8Array(Buffer.from(s, 'base64'));
                 };
                 const keyBytes = decodeB64(parsed.Key);
                 const ivBytes = decodeB64(parsed.IV);
                 if (keyBytes.byteLength !== 16 || ivBytes.byteLength !== 16) return;
                 const importedKey = await origPlayerSubtle.importKey('raw', keyBytes, { name: 'AES-CBC' }, false, ['decrypt']);
                 if (!audioCryptoKeyInfo) {
                     audioCryptoKeyInfo = { algorithm: 'AES-CBC', key: importedKey, iv: ivBytes, prefixIV: false };
                     activityObserved = true;
                     signalWake();
                 }
             } catch (_) {}
         }
         ```
       * Add `scanPerformanceTranscodeEntries(w)`:
         ```javascript
         function scanPerformanceTranscodeEntries(w) {
             if (!w || !w.performance || typeof w.performance.getEntriesByType !== 'function') return;
             let entries = [];
             try { entries = w.performance.getEntriesByType('resource') || []; } catch (_) { entries = []; }
             for (const entry of entries) {
                 const rawName = String((entry && entry.name) || '');
                 if (!rawName) continue;
                 const lower = rawName.toLowerCase();
                 if (lower.includes('onedrive.transcode') || lower.includes('part=initsegment') || lower.includes('part=mediasegment')) {
                     inspectTranscodeRequest(rawName, null, w);
                 }
             }
         }
         ```
       * Inside `captureStreamEmbedAudio()`'s `try` block, immediately after `installWindowHooks(playerWin); if (playerWin !== window) installWindowHooks(window);`:
         call `await seedEncKeyFromFileInfo();`, `scanPerformanceTranscodeEntries(playerWin);`, and `if (playerWin !== window) scanPerformanceTranscodeEntries(window);`.
       * Inside the `while (Date.now() < deadlineAt)` loop, before `maybeStartPathA()`:
         `if (!transcodeAudioTemplateUrl) { scanPerformanceTranscodeEntries(playerWin); if (playerWin !== window) scanPerformanceTranscodeEntries(window); }`.
  2. **In `internal/browser/recap.go` (`captureRecapAudioFromTargets`, `applyQueryIdentity`, `extractRecapTranscriptJS`, `copyAISummaryJS`, `captureAISummaryFromTargets`)**:
     * In `captureRecapAudioFromTargets`: treat `strings.Contains(lowerURL, "streamembed.aspx") || strings.Contains(lowerURL, "stream.aspx")` as `hasStreamEmbedTarget = true` and rank `t.Type == "iframe"` with `streamembed.aspx` or `stream.aspx` at `0` in `recapTargetRank`.
     * In `applyQueryIdentity`: check both `"fileUrl"` and `"id"` query parameters for media paths ending in `.mp4`, `.mkv`, `.webm`, `.mov`, or `.m4a` (when `!strings.Contains(fu.Path, "/:")`):
       ```go
       for _, key := range []string{"fileUrl", "id"} {
           if v := strings.TrimSpace(vals.Get(key)); v != "" && id.RecordingName == "" {
               if fu, err := url.Parse(v); err == nil && fu.Path != "" && !strings.Contains(fu.Path, "/:") {
                   base := filepath.Base(filepath.FromSlash(fu.Path))
                   ext := strings.ToLower(filepath.Ext(base))
                   if base != "." && base != "/" && (ext == ".mp4" || ext == ".mkv" || ext == ".webm" || ext == ".mov" || ext == ".m4a") {
                       id.RecordingName = strings.ToLower(base)
                   }
               }
           }
       }
       ```
     * In `extractRecapTranscriptJS`: when `cells.length === 0`, click the SharePoint `stream.aspx` `Transcript` button once before returning the empty `entries: []` JSON:
       ```javascript
       if (cells.length === 0) {
           const txBtn = document.querySelector('[role="menuitem"][aria-label="Transcript"], button[aria-label="Transcript"], [role="tab"][aria-label="Transcript"]');
           if (txBtn && typeof txBtn.click === 'function' && !txBtn.dataset.goTeamsTranscriptClicked) {
               txBtn.dataset.goTeamsTranscriptClicked = 'true';
               txBtn.click();
           }
           return JSON.stringify({ title: fileTitle, fileName: fileName, itemId: itemId, driveItemId: driveItemId, pageUrl: pageUrl, startTime: fileStart, timezone: tz, entries: [] });
       }
       ```
     * In `copyAISummaryJS`: when `!copyBtn` (`button[aria-label="Copy all"]` is absent), support SharePoint `stream.aspx` Copilot AI summary extraction and generation (gating Copilot pane/prompt clicks on the absence of Teams Recap tabs/header `!notesTab && !aiNotesTab && !document.querySelector('[data-tid="intelligent-recap-header"]')` so loading Teams `meetingrecap` pages are unaffected) while keeping `TestCopyAISummaryJSStructure`'s `NotContains(copyAISummaryJS, 'aria-label*="Copy"')` invariant intact:
       ```javascript
       const copyBtn = document.querySelector('button[aria-label="Copy all"]');
       if (!copyBtn) {
           const spCopyBtn = document.querySelector('.fai-CopilotMessage__actions button[title="Copy"], .fai-CopilotMessage__actions button[aria-label="Copy"]');
           if (spCopyBtn) {
               const msgEl = spCopyBtn.closest ? spCopyBtn.closest('.fai-CopilotMessage') : null;
               const contentEl = msgEl && msgEl.querySelector ? msgEl.querySelector('.fai-CopilotMessage__content') : null;
               const txt = String((contentEl ? (contentEl.innerText || contentEl.textContent) : (msgEl ? (msgEl.innerText || msgEl.textContent) : '')) || '').trim();
               if (txt) return txt;
           }
           const isTeamsRecapSurface = Boolean(notesTab || aiNotesTab || document.querySelector('[data-tid="intelligent-recap-header"]'));
           if (!isTeamsRecapSurface) {
               const closeCopilotBtn = document.querySelector('button[aria-label="Close Copilot"]');
               if (!closeCopilotBtn) {
                   const copilotBtn = document.querySelector('[role="menuitem"][aria-label="Copilot"], button[aria-label="Copilot"]');
                   if (copilotBtn && typeof copilotBtn.click === 'function' && !copilotBtn.dataset.goTeamsCopilotClicked) {
                       copilotBtn.dataset.goTeamsCopilotClicked = 'true';
                       copilotBtn.click();
                       await new Promise(r => setTimeout(r, 300));
                   }
               }
               const candidates = Array.from(document.querySelectorAll('button, [role="button"], .fui-ChatMyMessage__body, [class*="PromptStarter"], [class*="zeroPrompt"]') || []);
               const recapBtn = candidates.find(el => String((el && (el.innerText || el.textContent)) || '').trim().toLowerCase() === 'recap the meeting');
               if (recapBtn && typeof recapBtn.click === 'function' && !recapBtn.dataset.goTeamsRecapClicked) {
                   recapBtn.dataset.goTeamsRecapClicked = 'true';
                   recapBtn.click();
               }
           }
           return '';
       }
       ```
     * In `captureAISummaryFromTargets`: increase the polling loop from `20` attempts (`10s`) to `60` attempts (`30s` at `500ms` interval, checking `if attempt == 59 { break }` and bounded by `ctx.Err()`).
  3. **In `internal/browser/meeting_chrome.go` (`hasSpecificMeetingOrRecapTarget`, `isOnTargetMeetingOrRecapURL`, `isRecapIframeTarget`)**:
     * In `hasSpecificMeetingOrRecapTarget(meetingURL string) bool`:
       * Include `strings.Contains(lower, "stream.aspx")` and `strings.Contains(lower, "streamembed.aspx")`, and include `id.RecordingName != ""` in the `ExtractMeetingIdentity` return expression.
     * In `isOnTargetMeetingOrRecapURL(meetingURL, pageURL, path, hash string) bool`:
       * Check `exp := ExtractMeetingIdentity(meetingURL)` and `act := ExtractMeetingIdentity(pageURL)`:
         - If `hasMeetingIdentityConflict(exp, act)` is `true`, return `false` (ensuring a `stream.aspx` page with a different `RecordingName` or conflicting identity field never matches).
         - If `hasRecapAudioIdentityMatch(exp, act)` (`hasPositiveMeetingIdentityOverlap(exp, act) || (exp.RecordingName != "" && exp.RecordingName == act.RecordingName)`) is `true`, return `true`.
       * Add `case strings.Contains(lowerMeeting, "stream.aspx"): return strings.Contains(combined, "stream.aspx")` and `case strings.Contains(lowerMeeting, "streamembed.aspx"): return strings.Contains(combined, "streamembed.aspx")` to the `switch`.
     * In `isRecapIframeTarget(t cdpTargetInfo) bool`: include `strings.Contains(t.URL, "stream.aspx")` alongside `xplatplugins.aspx`, `streamembed.aspx`, and `hasHTTPSHostSuffix(t.URL, "sharepoint.com")`.
* **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, and `golangci-lint run`.
* **Depends on:** none.

### U2 (`033.002-T`) — Add unit, Chromium E2E, and browser live-integration tests for SharePoint `stream.aspx` capture (`internal/browser`)

* **Domain:** test (`internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`). **Posture:** test-first (Principle II — activate the Wave-Staged Red-Phase Harness from `logs/staged-harnesses/033.002-T/`). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `033.001-T` (`blocks`).
* **Files (3 files, 3 functions, 3 test scenarios):** `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`.
* **What:**
  1. **In `internal/browser/recap_test.go`**:
     * Add `TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity`:
       * Subtest `"performance-resource-timing-and-enckey-seeding-with-video-wsd-isolation"`: runs `extractRecapAudioJS` via `node -e` on a simulated `https://contoso.sharepoint.com/personal/user_contoso_com/_layouts/15/stream.aspx?id=%2Fpersonal%2Fuser_contoso_com%2FDocuments%2FRecordings%2FRecap.mp4` page with `g_fileInfo = { isDownloadBlocked: true, name: 'Recap.mp4', EncKey: JSON.stringify({ Key: keyB64, IV: ivB64 }) }` where no main-thread `crypto.subtle.decrypt` call occurs and `performance.getEntriesByType('resource')` contains pre-buffered `oneDrive.transcode` entries for both `track=audio&quality=audcopy&wsd=96256&ppd=192512` and `track=video&quality=vcopy&wsd=96000&ppd=192512`. Verifies that `seedEncKeyFromFileInfo` imports `EncKey`, `inspectTranscodeRequest` ignores `track=video`'s `wsd=96000`, Path A fetches and decrypts audio segments at `segmentTime=0` and `segmentTime=96256` (`wsd=96256`), and valid `pcmChunks` are returned.
       * Subtest `"transcript-tab-activation-and-copilot-ai-summary"`: verifies via `node -e` that `extractRecapTranscriptJS` clicks `[role="menuitem"][aria-label="Transcript"]` idempotently (`dataset.goTeamsTranscriptClicked === 'true'`) when `.ms-List` is absent and extracts the mounted `.ms-List` entries on the next poll, and verifies via `node -e` that `copyAISummaryJS` opens Copilot / clicks `"Recap the meeting"` once (`dataset.goTeamsRecapClicked === 'true'`) when `.fai-CopilotMessage` is absent and extracts `.fai-CopilotMessage__content` text once `.fai-CopilotMessage__actions button[title="Copy"]` appears.
       * Subtest `"stream-aspx-id-query-recording-name-and-target-url-matching"`: verifies in Go that `ExtractMeetingIdentity` extracts `RecordingName: "recap-20261116.mp4"` from `https://contoso.sharepoint.com/personal/u/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FRecap-20261116.mp4` while ignoring non-media `?id=222` and `/:v:/` paths, and that `hasSpecificMeetingOrRecapTarget` and `isOnTargetMeetingOrRecapURL` match same-recording `stream.aspx` URLs and reject conflicting `RecordingName` URLs.
  2. **In `internal/browser/meeting_e2e_test.go`**:
     * Add `TestChromeSharePointStreamAspxRecapEndToEnd`: launches headless Chromium via `NewLiveMeeting` against an `httptest.NewServer` serving `/_layouts/15/stream.aspx?id=%2Fpersonal%2Fu%2FDocuments%2FRecordings%2FStreamE2E.mp4`:
       * On page load (before `CaptureAudio`), the page issues `fetch` requests to `/_api/v2.1/drives/d1/items/s1/oneDrive.transcode?part=initsegment&track=audio&quality=audcopy&wsd=96256&ppd=192512` and `track=video&quality=vcopy&wsd=96000&ppd=192512` (populating `performance.getEntriesByType('resource')` without calling `window.crypto.subtle.decrypt`), and sets `window.g_fileInfo = { isDownloadBlocked: true, title: 'Stream.aspx E2E Recap', name: 'StreamE2E.mp4', EncKey: JSON.stringify({ Key: keyB64, IV: ivB64 }) }`.
       * `.ms-List` is not mounted until `[role="menuitem"][aria-label="Transcript"]` is clicked.
       * Clicking `"Recap the meeting"` in the Copilot sidebar renders `.fai-CopilotMessage` containing `<div class="fai-CopilotMessage__content">Copilot summary from stream.aspx.</div>` and `<div class="fai-CopilotMessage__actions"><button title="Copy">Copy</button></div>`.
       * Asserts `m.Join(ctx)` succeeds, `rm.IsRecap()` is `true`, `m.EnableCaptions` delivers `"Alice\x0000:00:05"` -> `"Hello from top-level stream.aspx."`, `m.(AISummaryProvider).AISummary()` equals `"Copilot summary from stream.aspx."`, and `m.CaptureAudio(ctx, &buf)` produces a non-empty valid MP3 (`0xFF`, `(b[1]&0xE0) == 0xE0`) having fetched audio segments with `wsd=96256` (`segmentTime=0` and `segmentTime=96256`) and zero `wsd=96000` audio requests.
  3. **In `internal/browser/scheduled_lifecycle_integration_test.go`**:
     * In `recapIntegrationURL`: update the `TEAMS_MEETING_URL` fallback check to:
       ```go
       if strings.Contains(lower, "meetingrecap") ||
           strings.Contains(lower, "stream.aspx") ||
           strings.Contains(lower, "streamembed.aspx") ||
           strings.Contains(lower, "19:meeting_") ||
           strings.Contains(lower, "19%3ameeting_") {
           raw = mURL
       }
       ```
* **Verify:** Run `go test -race -v ./internal/browser -run "TestSharePointStreamAspx|TestChromeSharePointStreamAspx|TestCopyAISummaryJSStructure"`, `go vet ./...`, `go vet -tags=integration ./...`, and `golangci-lint run`.

### U3 (`033.003-T`) — Update CLI live-tenant integration tests for SharePoint `stream.aspx` transcript, AI summary, and MP3 verification (`internal/cli`)

* **Domain:** test (`internal/cli/scheduled_integration_test.go`). **Posture:** test-first (Principle II — activate the Wave-Staged Red-Phase Harness from `logs/staged-harnesses/033.003-T/`). **Size:** `XS` | **Complexity:** `low`.
* **Depends on:** `033.001-T` (`blocks`), `033.002-T` (`blocks`).
* **Files (1 file, 2 test functions):** `internal/cli/scheduled_integration_test.go`.
* **What:**
  1. **In `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` (`internal/cli/scheduled_integration_test.go`)**:
     * Extend the existing dual-mode Recap URL check (`if strings.Contains(strings.ToLower(rawURL), "meetingrecap")`, which allows `.env.local.test` configured with a finite Recap URL to exercise `past-recap` while a live call URL exercises the scheduled overtime/call-end branch) to also recognize `"stream.aspx"` and `"streamembed.aspx"`:
       ```go
       lowerURL := strings.ToLower(rawURL)
       if strings.Contains(lowerURL, "meetingrecap") || strings.Contains(lowerURL, "stream.aspx") || strings.Contains(lowerURL, "streamembed.aspx") {
       ```
     * Add `"--audio"` to `recapArgs` so the CLI Recap integration branch captures MP3 audio alongside the Markdown transcript and AI Summary.
     * In addition to verifying the `.md` file (`## Transcript`, `"\n> "`, and `# AI Summary`), verify that a non-empty `.mp3` file was written to `outDir` with a valid MPEG frame sync header (`len(mp3Bytes) >= 2 && mp3Bytes[0] == 0xff && (mp3Bytes[1]&0xe0) == 0xe0`).
  2. **In `TestLiveTenantDelayedAISummaryQueueReplay` (`internal/cli/scheduled_integration_test.go`)**:
     * Update the `TEAMS_MEETING_URL` fallback check to include `strings.Contains(lower, "stream.aspx") || strings.Contains(lower, "streamembed.aspx")` alongside `meetingrecap`, `19:meeting_`, and `19%3ameeting_`.
* **Verify:** Run `go vet -tags=integration ./...`, `go test -race ./internal/cli/...`, and (when `.env.local.test` is present in Ship verification) `go test -tags=integration -v ./internal/cli/...`.

### U4 (`033.004-T`) — Document direct SharePoint/OneDrive `stream.aspx` capture in architecture and user guide (`docs/`)

* **Domain:** docs (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **Depends on:** `033.001-T` (`blocks`).
* **Files (2 files):** `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`.
* **What:**
  1. **In `docs/ARCHITECTURE.md` (`### Finite Meeting Recap export`)**:
     * Following progressive-disclosure conventions (`.github/instructions/architecture-doc.instructions.md`), concisely update steps 1, 2, and 3 to describe direct SharePoint/OneDrive `stream.aspx` recording URLs alongside Teams `meetingrecap` URLs:
       * Step 1: On SharePoint `stream.aspx` pages where the sidebar opens to Copilot by default, transcript extraction activates the `Transcript` tab once so `.ms-List` mounts, and verifies recording identity including `RecordingName` extracted from `stream.aspx?id=...mp4`.
       * Step 2: AI Summary extraction supports both the Teams Recap `Copy all` clipboard flow and the SharePoint `stream.aspx` Copilot `"Recap the meeting"` flow (`.fai-CopilotMessage`), polled up to 60 times at 500 ms intervals (`30s`).
       * Step 3: When `stream.aspx` pre-buffers `oneDrive.transcode` segments before `CaptureAudio` runs, `extractRecapAudioJS` seeds the audio transcode template (`track=audio` `wsd` isolated from `track=video`) from `PerformanceResourceTiming` and imports `g_fileInfo.EncKey` via WebCrypto AES-CBC to fetch and decrypt all audio segments in parallel.
  2. **In `docs/USER_GUIDE.md` (`## Compiled executable and explicit output file`)**:
     * Update the Meeting Recap paragraph to note that direct SharePoint and OneDrive `stream.aspx` recording URLs (`https://<tenant>.sharepoint.com/personal/<user>/_layouts/15/stream.aspx?id=/personal/<user>/Documents/Recordings/<file>.mp4`) are also supported end-to-end: `go-teams` activates the SharePoint `Transcript` pane, triggers and extracts the SharePoint Copilot `"Recap the meeting"` AI Summary, and captures high-speed MP3 audio.
* **Verify:** Run `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `backlogit doctor`.

## Dependency Graph

```text
033.001-T (U1: internal/browser/recap.go + meeting_chrome.go)
    |
    +---> 033.002-T (U2: internal/browser/recap_test.go + meeting_e2e_test.go + scheduled_lifecycle_integration_test.go)
    |         |
    |         +---> 033.003-T (U3: internal/cli/scheduled_integration_test.go)
    |
    +---> 033.004-T (U4: docs/ARCHITECTURE.md + docs/USER_GUIDE.md)
```

* **Shipment sequencing DAG**:
  * Remove `010-S -> 032-S` direct edge (`backlogit dep remove 010-S 032-S`).
  * Add `033-S -> 032-S` (`backlogit dep add 033-S 032-S --type blocks` — `032-S` is `shipped`, placing `033-S` in `ready_set`).
  * Add `010-S -> 033-S` (`backlogit dep add 010-S 033-S --type blocks` — sequencing `010-S` after `033-S`).

## Decisions and Rationale

1. **Seeding `PerformanceResourceTiming` + `g_fileInfo.EncKey` for Web Worker pre-buffered streams**:
   * **Rationale**: On top-level SharePoint `stream.aspx`, the player starts pre-buffering during `Join` inside a dedicated Web Worker before `CaptureAudio` runs. Because Worker `fetch` calls bypass `window.fetch` hooks, scanning `w.performance.getEntriesByType('resource')` (`scanPerformanceTranscodeEntries`) and importing `window.g_fileInfo.EncKey` (`seedEncKeyFromFileInfo`) via `crypto.subtle.importKey('raw', ...)` allows Path A to immediately discover the authenticated `oneDrive.transcode` URL template, `wsd`, `ppd`, and AES-CBC key/IV without reloading the page.
2. **Guarding `transcodeWsd` and `transcodeAudioTemplateUrl` against `track === 'video'`**:
   * **Rationale**: SharePoint `oneDrive.transcode` uses `wsd=96256` for `track=audio&quality=audcopy` (`94 * 1024` samples at 16 kHz) and `wsd=96000` for `track=video&quality=vcopy`. Guarding `transcodeWsd` and `transcodeAudioTemplateUrl` updates with `track !== 'video'` guarantees that video segment requests never corrupt audio segment `segmentTime = k * wsd` calculations.
3. **Idempotent DOM activation for `stream.aspx` Transcript and Copilot `"Recap the meeting"`**:
   * **Rationale**: Tagging clicked buttons with `dataset.goTeamsTranscriptClicked = 'true'` and `dataset.goTeamsRecapClicked = 'true'` ensures `extractRecapTranscriptJS` and `copyAISummaryJS` trigger pane/prompt activation at most once across repeated 500 ms / 1500 ms CDP polls, while extending `captureAISummaryFromTargets` to 60 attempts (`30s`) accommodates SharePoint Copilot's ~12.5-second summary generation latency.
4. **Extracting `RecordingName` from media-path `id` query parameters**:
   * **Rationale**: Direct SharePoint `stream.aspx` links encode the server-relative file path in `?id=/personal/<user>/Documents/Recordings/<file>.mp4`. Restricting `id`-based extraction to paths without `/:` whose basename ends in `.mp4`, `.mkv`, `.webm`, `.mov`, or `.m4a` enables strict recording-identity verification without false positives on numeric or opaque `id` query parameters.

## Risks and Caveats

* **Lightweight 3-Threat Model**:
  1. **Most likely threat**: Cross-track `wsd` contamination where a `track=video` (`wsd=96000`) entry in `performance.getEntriesByType('resource')` overwrites `transcodeWsd` (`96256`), causing `oneDrive.transcode` to reject `track=audio` segment requests. Mitigated by guarding both `transcodeWsd` and `transcodeAudioTemplateUrl` in `inspectTranscodeRequest` with `if (track !== 'video')`, verified by dedicated unit and E2E tests.
  2. **Highest-impact threat**: Cross-recording contamination or unverified audio/transcript capture when navigating to or rebinding a SharePoint `stream.aspx` tab. Mitigated by extracting `RecordingName` from `stream.aspx?id=...mp4` in both Go (`applyQueryIdentity`) and JS (`checkIdentityAgainst`) and enforcing `hasMeetingIdentityConflict` in `isOnTargetMeetingOrRecapURL`, `verifyRecapTargetMatchesMeeting`, and `captureRecapAudioFromTargets`.
  3. **Most subtle threat**: Regressing `TestCopyAISummaryJSStructure` (which asserts `NotContains(copyAISummaryJS, 'aria-label*="Copy"')` so generic copy buttons are never clicked) or re-triggering `"Recap the meeting"` on every 500 ms poll in `captureAISummaryFromTargets`. Mitigated by using exact selectors `.fai-CopilotMessage__actions button[title="Copy"], .fai-CopilotMessage__actions button[aria-label="Copy"]`, reading `.fai-CopilotMessage__content` directly once the Copy button renders, and guarding the `"Recap the meeting"` click with `dataset.goTeamsRecapClicked = 'true'`.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, zero `unsafe`, zero lint findings, all errors checked |
| II. Test-First Development | Compliant — Wave-Staged Red-Phase Harness for U1/U2, `node -e` unit tests (`recap_test.go`), real Chromium E2E test (`meeting_e2e_test.go`), and live `-tags=integration` tests (`scheduled_lifecycle_integration_test.go`, `scheduled_integration_test.go`) |
| III. Workspace Isolation | Compliant — all temporary test outputs use `t.TempDir()` or repo-local `logs/` |
| IV. CLI Workspace Containment | Compliant — zero writes outside the repository or configured output directory |
| V. Structured Observability | Compliant — structured `log/slog` logging with zero unredacted URLs, tokens, or `EncKey` values; mandatory post-merge context compaction (P-020) enforced at closure |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); 4 width-isolated tasks (`<= 3` files each) |
| VII. Destructive Command Approval | Compliant — no destructive commands used |
| VIII. Explicit Safety Modes | Compliant — `Careful mode` + `Freeze-scope mode` (scoped to `internal/browser`, `internal/cli`, and `docs/`) + `Strict-safety` active; `ProposedAction` inventory (`PA-033-1`..`PA-033-4`) recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded command outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No exported Go type signatures, CLI flags, or config schema keys are changed (`ExtractMeetingIdentity` signature is unchanged; its documented behavior is extended to also inspect media-path `id` query parameters).
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Imports AES-CBC key material from `window.g_fileInfo.EncKey` via WebCrypto `crypto.subtle.importKey`, inspects `PerformanceResourceTiming` URLs under same-origin `https:` validation (`isAllowedSameOriginTranscodeURL`), and extends recording-identity matching (`RecordingName` from `stream.aspx?id=...`).
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Purely additive in-memory browser extraction and test/documentation updates.
* **External integration, operator checkpoint, or external dependency**: **Present** — Exercises live SharePoint `stream.aspx` integration tests (`-tags=integration`) against `.env.local.test`.
* **High runtime, rollout, or rollback risk**: **Present** — Modifies `extractRecapAudioJS`, `extractRecapTranscriptJS`, `copyAISummaryJS`, and `captureAISummaryFromTargets` in `internal/browser/recap.go` and target URL matching in `internal/browser/meeting_chrome.go`.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * Production Recap audio, transcript, AI summary, and recording-identity extraction (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`)
  * Browser unit, E2E, and `-tags=integration` test suites (`internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`)
  * CLI `-tags=integration` test suite (`internal/cli/scheduled_integration_test.go`)
  * Architecture and user guide documentation (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`)
* **Runtime verification**:
  * Run `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...`.
  * Run `go test -tags=integration -v ./internal/browser/...` and `go test -tags=integration -v ./internal/cli/...` against `.env.local.test` during Ship verification to confirm all live-tenant integration tests pass with zero skips and capture transcript, Copilot AI summary, and valid MP3 audio on SharePoint `stream.aspx`.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, releasability evidence contract, and mandatory post-merge context compaction (`compact-context` skill, P-020) recorded in `docs/closure/` during Ship operational closure.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan modifies security-sensitive recording-identity matching (`ExtractMeetingIdentity`, `checkIdentityAgainst`, `isOnTargetMeetingOrRecapURL`), WebCrypto AES-CBC key seeding (`window.g_fileInfo.EncKey`), `PerformanceResourceTiming` transcode URL discovery, and DOM/Copilot extraction in `internal/browser/recap.go` and `internal/browser/meeting_chrome.go`.
* **Safety Mode Posture (Principle VIII)**: `Careful mode` + `Freeze-scope mode` (strictly scoped to `internal/browser/`, `internal/cli/scheduled_integration_test.go`, and `docs/`) + `Strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/browser-verification.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md`
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md`

### Protected Invariants

1. **Single-Binary Pure-Go Architecture & Chunked CDP Transfer (`docs/ARCHITECTURE.md`)**: Zero external media tools (`ffmpeg`, `yt-dlp`), zero CGO, and zero changes to `go.mod` / `go.sum`. Multi-megabyte PCM payloads (`totalPCMBytes > 4 MiB`) continue to stage on `window.__goTeamsRecapAudioTransfer` and pull in 64-chunk batches over a single persistent WebSocket session (`newCDPEvalSession`), and all PCM samples flow through `internal/audio.NewEncoder`.
2. **Same-Origin `https:` Validation & Secret Redaction on `PerformanceResourceTiming` and `EncKey`**: Every URL discovered from `performance.getEntriesByType('resource')` passes through `inspectTranscodeRequest` -> `isAllowedSameOriginTranscodeURL(u.toString(), hookWin)`, which enforces same-origin `https:` (or loopback `http:` for local tests). `window.g_fileInfo.EncKey` is imported exclusively into an non-extractable (`extractable: false`) in-memory `CryptoKey` inside the browser page and is never serialized, logged, or returned over CDP.
3. **Track-Specific `wsd` Isolation (`track !== 'video'`)**: `inspectTranscodeRequest` MUST NEVER allow a `track=video` request (`wsd=96000`) to seed or overwrite `transcodeWsd` (`96256` for `track=audio`) or `transcodeAudioTemplateUrl`.
4. **Conflict-Checked Recording Identity Isolation (`recapExpectedIdentity`)**: `applyQueryIdentity` and `checkIdentityAgainst` extract `RecordingName` from `fileUrl` and `id` only when the path contains no `/:` sharing prefix and ends in `.mp4`, `.mkv`, `.webm`, `.mov`, or `.m4a`. `isOnTargetMeetingOrRecapURL` checks `hasMeetingIdentityConflict(exp, act)` before returning `true`, preventing cross-recording contamination.
5. **Preservation of `TestCopyAISummaryJSStructure` and Existing `meetingrecap` / `streamembed.aspx` Flows**: `copyAISummaryJS` MUST retain `data-tid="Notes"`, `data-tid="AINotes"`, `aria-label="Copy all"`, and `navigator.clipboard`, and MUST NOT contain `aria-label*="Copy"`.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-033-1` | Seed `PerformanceResourceTiming` transcode URLs and `g_fileInfo.EncKey` AES-CBC keys, isolate `track !== 'video'` `wsd`, click `stream.aspx` `Transcript` tab when `.ms-List` is unmounted, extract SharePoint Copilot `.fai-CopilotMessage` AI summary with 60-attempt polling, and support `stream.aspx?id=...mp4` `RecordingName` identity matching | `internal/browser/recap.go`, `internal/browser/meeting_chrome.go` | Production browser Recap audio, transcript, AI summary, and identity enhancement | `high` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U1 commit |
| `PA-033-2` | Add `node -e` unit tests (`TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity`), headless Chromium E2E test (`TestChromeSharePointStreamAspxRecapEndToEnd`), and `stream.aspx` URL recognition in `recapIntegrationURL` | `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go` | Browser unit, E2E, and `-tags=integration` test updates | `moderate` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U2 commit |
| `PA-033-3` | Update CLI live-tenant integration tests (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`) to recognize `stream.aspx` / `streamembed.aspx` and verify `.mp3` + `.md` + `# AI Summary` capture | `internal/cli/scheduled_integration_test.go` | CLI `-tags=integration` test update | `low` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U3 commit |
| `PA-033-4` | Document direct SharePoint/OneDrive `stream.aspx` transcript, Copilot AI summary, and MP3 audio capture in architecture and user guide | `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md` | Documentation update | `low` | Pre-authorized under `Autopilot Mode` | `planned` | `git revert` of U4 commit |
| `PA-033-5` | Execute live-tenant `-tags=integration` browser and CLI test suites against `.env.local.test` during Ship verification to confirm end-to-end SharePoint `stream.aspx` transcript, Copilot AI summary, and MP3 audio capture | `internal/browser`, `internal/cli` (`-tags=integration`) | External live-tenant integration verification | `moderate` | Pre-authorized under `Autopilot Mode` | `planned` | Ephemeral `t.TempDir()` outputs only; no persistent external mutation |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages, including `TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity`, `TestChromeSharePointStreamAspxRecapEndToEnd`, and `TestCopyAISummaryJSStructure`.
  * [ ] `go test -tags=integration -v ./internal/browser/...` and `go test -tags=integration -v ./internal/cli/...` pass against `.env.local.test` with zero skips and zero credential leaks (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `recap_stream_aspx_audio_capture_success_rate` (`TestChromeSharePointStreamAspxRecapEndToEnd` and live `TestMeetingCaptureAudio` / `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` == 100% with valid MPEG frame sync `0xFF`, `(b[1]&0xE0) == 0xE0`), (2) `recap_stream_aspx_transcript_and_copilot_summary_pass_rate` (100% non-empty transcript + `# AI Summary` extraction), and (3) `tracked_secret_leak_count` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == 0`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main`.
  * **Dashboard / query**: `gh run list --workflow=ci.yml` and local `-tags=integration` suite execution.
  * **Baseline**: 100% pass rate on `changes` and `test-windows` in CI and 100% pass rate across all `-tags=integration` tests in `internal/browser` and `internal/cli`.
  * **Alert threshold**: `>= 1` unit, E2E, or live integration test failure or unexpected `isDownloadBlocked=true` / empty summary error on `stream.aspx`.
* **Rollback trigger & procedure**:
  * **Trigger**: Any regression in `streamembed.aspx`, `stream.aspx`, or `meetingrecap` transcript, AI summary, or MP3 audio capture, or any CI failure on `main`.
  * **Procedure**: Revert the `033-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, unit tests, Chromium E2E tests, live-tenant `-tags=integration` suites (`internal/browser` and `internal/cli`), and GitHub Actions PR checks (`changes`, `test-windows`) pass and are recorded in `docs/closure/` (followed by mandatory post-merge context compaction under P-020).
  * `READY_WITH_CONDITIONS`: All local quality gates, unit/E2E tests, and PR CI checks pass, with any environment-specific condition documented and verified.
  * `BLOCKED`: Any pre-deploy audit check, unit/E2E test, live integration test, or PR CI check fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist` on anchor route `gpt-6.1-sol`, and `Security Lens Reviewer`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `Autopilot Mode`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings from the 6-persona review panel were remediated in-place in `docs/plans/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-plan.md` and `docs/decisions/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-deliberation.md` prior to harvest; zero P0 findings).
* **Capability Status (P-012):** `TOOL_OK: reviewer-subagent-dispatch`, `TOOL_OK: model-specific-review-routing` (`Architecture Strategist` dispatched on anchor route `gpt-6.1-sol` with `reasoning_effort: high`).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-033-1`..`PA-033-5`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` (anchor route `gpt-6.1-sol`) | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2, 4 P3 | Remediated — `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Learnings Researcher` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 (5 prior learnings verified aligned) | `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Architecture Strategist`] Pre-buffered `PerformanceResourceTiming` + `g_fileInfo.EncKey` discovery contract and E2E fixture fidelity**:
   * **Finding**: Ensure U1 and U2 explicitly test pre-`CaptureAudio` `oneDrive.transcode` resource timing discovery (`w.performance.getEntriesByType('resource')`) combined with `window.g_fileInfo.EncKey` WebCrypto import when no `window.crypto.subtle.decrypt` call occurs during `CaptureAudio`.
   * **Remediation**: Verified and specified in U1 (`scanPerformanceTranscodeEntries` + `seedEncKeyFromFileInfo`) and U2 (`TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity` and `TestChromeSharePointStreamAspxRecapEndToEnd`) that `oneDrive.transcode` requests occur before `CaptureAudio` (populating `performance.getEntriesByType('resource')` with both `track=audio` `wsd=96256` and `track=video` `wsd=96000`) and `EncKey` is imported via `crypto.subtle.importKey` without any main-thread `crypto.subtle.decrypt` invocation.
2. **[P1 — `Architecture Strategist`] Conflict-checked `RecordingName` matching semantics across `fileUrl` and `stream.aspx?id=...mp4`**:
   * **Finding**: Ensure `isOnTargetMeetingOrRecapURL` enforces `hasMeetingIdentityConflict(exp, act)` before accepting a `RecordingName` match (`hasRecapAudioIdentityMatch(exp, act)`), and cover conflicting `RecordingName` values as well as non-media `id` values (`?id=222`, `/:v:/`) in U2.
   * **Remediation**: Explicitly specified in U1 (`isOnTargetMeetingOrRecapURL` checks `if hasMeetingIdentityConflict(exp, act) { return false }` followed by `if hasRecapAudioIdentityMatch(exp, act) { return true }`) and in U2 (`stream-aspx-id-query-recording-name-and-target-url-matching` subtest).
3. **[P2 — `Architecture Strategist`] Positive surface gating before clicking SharePoint Copilot controls in `copyAISummaryJS`**:
   * **Finding**: Gate the SharePoint Copilot pane opener and `"Recap the meeting"` prompt click on the absence of Teams Recap controls (`!notesTab && !aiNotesTab && !document.querySelector('[data-tid="intelligent-recap-header"]')`) so loading Teams `meetingrecap` pages never trigger unintended clicks.
   * **Remediation**: Added `const isTeamsRecapSurface = Boolean(notesTab || aiNotesTab || document.querySelector('[data-tid="intelligent-recap-header"]')); if (!isTeamsRecapSurface) { ... }` in U1's `copyAISummaryJS` specification.
4. **[P2 — `Architecture Strategist`] Dual-mode `.env.local.test` behavior in `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` (`U3`)**:
   * **Finding**: Document how `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` handles `.env.local.test` configured with a `stream.aspx` URL vs. a live call URL.
   * **Remediation**: Documented in U3 that `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` extends its existing `meetingrecap` dual-mode branch to recognize `stream.aspx` and `streamembed.aspx` (and adds `--audio` + `.mp3` MPEG sync verification), while preserving the scheduled live-call overtime and call-end branch when `TEAMS_MEETING_URL` is a live call URL.
5. **[P3 — `Architecture Strategist` & `Constitution Reviewer`] Progressive disclosure in `docs/ARCHITECTURE.md` (`U4`), Wave-Staged Red-Phase Harnesses for both `033.002-T` and `033.003-T`, U1 4-group cohesion rationale, and `PA-033-5` `ProposedAction` entry**:
   * **Remediation**: Updated U4 to keep `docs/ARCHITECTURE.md` concise per `.github/instructions/architecture-doc.instructions.md`; updated U1, U2, U3, and the deliberation to stage red-phase harnesses in both `logs/staged-harnesses/033.002-T/` and `logs/staged-harnesses/033.003-T/`; documented the 4 cohesive functional groups in U1; and added `PA-033-5` for live-tenant `-tags=integration` verification in `## Plan Hardening`.

