---
title: High-speed audio-only MP3 capture on download-blocked SharePoint Meeting Recaps
description: Deliberation on stash entry 9A16B213 for covering feature 031-F and shipment 031-S
topic: High-Speed Audio-Only MP3 Capture on Download-Blocked SharePoint Meeting Recaps (031-S)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 020-DL
feature: 031-F
shipment: 031-S
source_stash:
  - 9A16B213
linked_artifacts:
  - docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md
  - .backlogit/queue/020-DL.md
date: 2026-11-16
status: accepted
tags:
  - audio
  - recap
  - browser
  - sharepoint
  - mp3
---

## Problem Frame

In enterprise Microsoft 365 tenants, SharePoint Meeting Recaps frequently set `window.g_fileInfo.isDownloadBlocked = true` inside `streamembed.aspx` (view-only recording policy). Under this policy:

* Direct raw media download URLs (`g_fileInfo.downloadUrl`, `g_fileInfo.streamUrl`, `g_fileInfo.mediaUrl`) either return HTTP `403 Forbidden` or redirect to a `Content-Type: text/html` error page (`AccessDenied.aspx`).
* The `<video>` element in `streamembed.aspx` uses a MediaSource Extensions (`blob:`) URL fed by fragmented MP4 (`fMP4`) segments from SharePoint's `oneDrive.transcode` endpoint rather than a downloadable monolithic `.mp4` file.
* In shipment `030-S` (`019-DL`), `extractRecapAudioJS` added deterministic detection for `isDownloadBlocked=true` so `TestMeetingCaptureAudio` could skip cleanly (`t.Skip`) rather than passing HTML error bytes to `AudioContext.decodeAudioData`.

Stash entry **`9A16B213`** (`priority: high`, `kind: feature`) requests full support for high-speed (`14x-100x` real-time) audio-only MP3 capture on download-blocked SharePoint Meeting Recaps (`g_fileInfo.isDownloadBlocked=true` in `streamembed.aspx`):

1. Trigger muted in-page playback (`'Play video'`) inside `streamembed.aspx` (whether `streamembed.aspx` is a top-level/OOPIF CDP target or a same-origin child `<iframe>` inside `xplatplugins.aspx`).
2. Strip out the `video/mp4` track so the browser never wastes bandwidth or CPU downloading and decoding H.264 video frames.
3. Collect and decrypt only the `audio/mp4` (`mp4a.40.2`) fMP4 stream (`part=initsegment` + `part=mediasegment` with `track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd` via `oneDrive.transcode`) at `14x-100x` real-time speed by:
   * **Path A (WebCrypto AES-CBC + parallel `oneDrive.transcode` segment fetch/decrypt at `50x-100x` real-time)**: Intercepting `window.crypto.subtle.decrypt` (AES-CBC) and `fetch` / `XMLHttpRequest` to capture the authenticated `oneDrive.transcode` audio URL template (`track=audio&quality=audcopy`, `wsd`, `ppd`) and AES-CBC `CryptoKey` / IV, pausing `<video>` / dropping `track=video` requests, and fetching + decrypting all audio segments (`segmentTime = k * wsd` up to `ppd`) in bounded parallel batches.
   * **Path B (MSE `MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hook at `14x-40x` real-time)**: Hooking `MediaSource.prototype.addSourceBuffer` and `SourceBuffer.prototype.appendBuffer`, stripping `video/mp4` via a lightweight `EventTarget`-compatible no-op `SourceBuffer` shim (or dropping `video/mp4` media appends), collecting deduplicated `audio/mp4` (`mp4a.40.2`) `initsegment` (`ftyp`/`moov`) and `mediasegment` (`styp`/`sidx`/`moof`) buffers ordered chronologically by ISO BMFF `moof -> traf -> tfdt` `baseMediaDecodeTime`, muting `<video>`, setting `playbackRate = 16.0`, and stepping `video.currentTime` across buffered audio ranges (with periodic buffer eviction to avoid `QuotaExceededError`).
4. Decode the reassembled `audio/mp4` fMP4 stream via `AudioContext.decodeAudioData` to 44.1 kHz mono `int16` PCM chunks and encode to MP3 in Go via `internal/audio.NewEncoder`.

### Duplicate Detection Record

An unconditional duplicate scan across `.backlogit/stash.jsonl`, `.backlogit/archive/stash.jsonl`, `.backlogit/queue/`, and `.backlogit/archive/` confirmed **CLEAN DUPLICATE SCAN** for `9A16B213`. Archived shipment `015-S` (`DF1F461F`) implemented direct-fetch Recap audio extraction, and archived shipment `030-S` (`02B5113C`) added `isDownloadBlocked=true` rejection and `t.Skip` in `TestMeetingCaptureAudio`. No active or archived item implements high-speed `streamembed.aspx` MSE / `oneDrive.transcode` / WebCrypto AES-CBC audio-only capture.

### Scope Boundaries

* **In scope**:
  * Production high-speed `streamembed.aspx` audio-only fMP4 extraction (`Path A` WebCrypto AES-CBC parallel `oneDrive.transcode` fetch/decrypt + `Path B` MSE `addSourceBuffer`/`appendBuffer` hook with `video/mp4` stripping, `16x` `playbackRate`, and `currentTime` stepping), same-origin child `<iframe>` traversal, transitive `ParentID` iframe scope resolution, and verified transcript `ItemID` (`VroomItemId`) retention in `internal/browser/recap.go` and `internal/browser/meeting_chrome.go` (`031.001-T`).
  * Deterministic `node -e` unit tests in `internal/browser/recap_test.go`, real headless Chromium E2E test in `internal/browser/meeting_e2e_test.go`, and live-tenant `-tags=integration` verification in `internal/browser/meeting_integration_test.go` (`TestMeetingCaptureAudio` asserting MP3 capture without skipping on `isDownloadBlocked=true`) (`031.002-T`).
  * Architecture and user guide documentation updates in `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` (`031.003-T`).
  * Sequencing shipment `031-S` (`031-F`, `020-DL`) immediately after `030-S` and before `010-S` in the shipment DAG.
* **Out of scope**:
  * Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, and `3F6BA819`, which remain untouched in `active` stash state.
  * Any external binaries (`ffmpeg`, `yt-dlp`), CGO dependencies, or new entries in `go.mod`.

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/build-errors/shine-mp3-frame-alignment.md` & `docs/compound/build-errors/shine-mp3-race-checkptr.md`**: All decoded 44.1 kHz mono `int16` PCM chunks must flow through `audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` + `Flush()`), which buffers 1152-sample frames and pads `encoderTailSlack = 4` capacity on the stereo slice so `go test -race` checkptr never faults.
* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: Never log or echo unredacted `oneDrive.transcode` URLs, query tokens, meeting URLs, or account identifiers.

### Codebase and SharePoint `streamembed.aspx` Analysis

1. **Iframe Topology (`xplatplugins.aspx` and `streamembed.aspx`)**:
   * In Teams Meeting Recaps, the outer `teams.microsoft.com/l/meetingrecap` page embeds SharePoint's `xplatplugins.aspx` as a cross-origin Out-Of-Process Iframe (OOPIF), and `streamembed.aspx` is either the iframe target itself, a child OOPIF whose `ParentID` is `xplatplugins.aspx`'s target ID, or a same-origin `<iframe src*="streamembed.aspx">` inside `xplatplugins.aspx` (since both `xplatplugins.aspx` and `streamembed.aspx` share the `https://<tenant>-my.sharepoint.com` origin).
   * Updating `isTargetInCaptureScope` / `expandOwnedFrames` in `internal/browser/recap.go` to compute the transitive closure of `ParentID` under `ownerID` AND traversing accessible same-origin child `<iframe>` windows (`contentWindow` / `contentDocument`) inside `extractRecapAudioJS` guarantees that `streamembed.aspx` is reached in all three topologies while still excluding unrelated browser tabs.
2. **Dual-Path High-Speed Audio Collection (`Path A` + `Path B`)**:
   * **Path A (`oneDrive.transcode` + WebCrypto `crypto.subtle.decrypt`, `50x-100x` speed)**: When `'Play video'` is clicked, `streamembed.aspx` fetches `part=initsegment` and `part=mediasegment` (`track=audio&quality=audcopy`, `segmentTime=0`, `wsd`, `ppd`) from `oneDrive.transcode` and decrypts them with `window.crypto.subtle.decrypt({ name: 'AES-CBC', iv }, key, ciphertext)`. By hooking `crypto.subtle.decrypt`, `fetch`, and `XMLHttpRequest` before triggering playback, `extractRecapAudioJS` captures the audio `oneDrive.transcode` URL template and AES-CBC `CryptoKey`/`iv` (supporting both fixed session IV and 16-byte prefix IV), pauses `<video>` / drops `track=video` requests, and fetches + decrypts all audio segments (`segmentTime = k * wsd` for `0 <= k * wsd < ppd`) with bounded concurrency (`8`).
   * **Path B (MSE `MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hook, `14x-40x` speed)**: Concurrently, hooking `MediaSource.prototype.addSourceBuffer` and `SourceBuffer.prototype.appendBuffer` intercepts the already-decrypted `audio/mp4` (`mp4a.40.2`) buffers appended by the player, strips out `video/mp4` (via a lightweight `EventTarget` mock `SourceBuffer` for `video/mp4` and/or dropping video media appends so `<video>` never stalls waiting for video frames), deduplicates and orders audio media segments by their ISO BMFF `moof -> traf -> tfdt` `baseMediaDecodeTime`, mutes `<video>`, sets `playbackRate = 16.0`, and steps `video.currentTime` to the end of `audioSB.buffered` (evicting buffered ranges `> 30s` behind `bufEnd`) until all segments up to `duration` / `ppd` are collected.
3. **Preserving Existing Fail-Closed Unit Tests and Guardrails**:
   * In minimal unit test mocks (such as `TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked` in `internal/browser/recap_test.go`) where `MediaSource` is undefined and no `'Play video'` button, `media.play()`, or `oneDrive.transcode` stream is present, `extractRecapAudioJS` immediately throws `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`.
   * All pre-allocation resource limits (`maxRecapAudioSourceBytes = 256 MiB`, `maxRecapAudioDurationSec = 4200`, `maxRecapAudioPCMBytes = 360 MiB`) and recording-identity checks (`recapExpectedIdentity`) are enforced before `decodeAudioData` and PCM buffer allocation.

## Options Evaluated

### Option A: Dual-Path In-Browser WebCrypto AES-CBC Parallel `oneDrive.transcode` Fetch/Decrypt + MSE `SourceBuffer` Hook with `video/mp4` Stripping, `16x` Playback Rate, and `currentTime` Stepping (Recommended)

* **Description**:
  * **U1 (`031.001-T` — `code` domain: `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, `Size: M`, `Complexity: medium`, 2 files)**: Implement `captureStreamEmbedAudio` inside `extractRecapAudioJS` combining Path A (WebCrypto AES-CBC `window.crypto.subtle.decrypt` + `oneDrive.transcode` parallel audio segment fetch/decrypt) and Path B (`MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hook with `video/mp4` track stripping, `tfdt`-ordered segment deduplication, muted `16x` `playbackRate`, and `currentTime` stepping), same-origin child `<iframe>` traversal, transitive `ParentID` iframe scope resolution, and verified transcript `ItemID` retention on `chromeMeeting`.
  * **U2 (`031.002-T` — `test` domain: `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `Size: S`, `Complexity: low`, depends on `031.001-T`, 3 files)**: Add `node -e` unit tests for Path A, Path B, same-origin child `<iframe>`, transitive `ParentID`, and resource-limit rejection in `recap_test.go`; add real headless Chromium E2E test `TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd` in `meeting_e2e_test.go`; and update `TestMeetingCaptureAudio` in `meeting_integration_test.go` to require MP3 capture without skipping on `isDownloadBlocked=true`.
  * **U3 (`031.003-T` — `docs` domain: `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `Size: XS`, `Complexity: trivial`, depends on `031.001-T`, 2 files)**: Document high-speed audio-only MP3 capture on download-blocked SharePoint Meeting Recaps.
* **Pros**:
  * Achieves `14x-100x` real-time capture speed (fetching and decoding a 30-to-60-minute meeting's audio in ~3–15 seconds).
  * Strips `video/mp4` completely, saving 90%+ network bandwidth and CPU.
  * Dual-path design (Path A parallel `oneDrive.transcode` + `crypto.subtle.decrypt` backed by Path B MSE `appendBuffer` + `16x`/`currentTime` stepping) handles both standard AES-CBC session keys and player-managed per-segment key rotation.
  * Preserves the pure-Go single-binary architecture with zero external dependencies.
* **Cons**:
  * Requires careful ISO BMFF box inspection (`ftyp`/`moov`/`moof`/`traf`/`tfdt`) and hook restoration in `extractRecapAudioJS`.
* **Effort estimate**: Medium (3 atomic tasks: `M`, `S`, `XS`).
* **Fit**: 100% alignment with stash `9A16B213` and repository architecture.

### Option B: Real-Time `1x` `getDisplayMedia` Tab-Audio Recording During Full Recap Playback

* **Description**: Trigger `'Play video'` and record tab audio in real time at `1x` speed via `navigator.mediaDevices.getDisplayMedia` for the entire duration of the recording.
* **Pros**: Simpler JS hook.
* **Cons**: Violates the `14x-100x` real-time speed requirement in `9A16B213` (a 60-minute meeting would take 60 minutes to capture and time out `TestMeetingCaptureAudio`), downloads the full video stream, and suffers from pitch/drop distortion if `playbackRate > 1x` is used with `getDisplayMedia`.

### Option C: External `ffmpeg` / `yt-dlp` Subprocess Pipeline

* **Description**: Export browser cookies and invoke an external `ffmpeg` or `yt-dlp` binary to download and decrypt the stream.
* **Pros**: Uses external C demuxer.
* **Cons**: Violates the repository's core single-binary, zero-FFmpeg architecture (`docs/ARCHITECTURE.md`), fails when `oneDrive.transcode` segments are encrypted via in-page WebCrypto `crypto.subtle.decrypt`, and exports sensitive authentication cookies outside the browser process.

## Trade-off Comparison

| Criterion | Option A (Dual-Path WebCrypto + MSE `audio/mp4` fMP4 Capture) | Option B (`1x` `getDisplayMedia` Tab Audio) | Option C (External `ffmpeg` / `yt-dlp`) |
|---|---|---|---|
| Capture speed | `14x-100x` real-time (~3–15s per meeting) | `1x` real-time (30–60 min per meeting) | N/A (cannot decrypt in-page WebCrypto) |
| Strips `video/mp4` track | Yes (`audio/mp4` `mp4a.40.2` only) | No (downloads & decodes full video) | No |
| Single-binary / zero-FFmpeg | Yes (pure Go + Chromium Web Audio) | Yes | No (violates architecture) |
| Passes `TestMeetingCaptureAudio` on `isDownloadBlocked=true` | Yes | Times out | Fails |

## Decision

We select **Option A**: decompose covering feature **`031-F`** into three atomic, width-isolated tasks (`031.001-T`, `031.002-T`, `031.003-T`) packaged into shipment **`031-S`**, sequenced immediately before `010-S`.

## Rejected Alternatives

* **Option B** was rejected because `1x` real-time playback takes up to an hour for a 1-hour recording, downloads unnecessary video segments, and fails the `14x-100x` speed requirement.
* **Option C** was rejected because it violates the zero-FFmpeg single-binary architecture and cannot intercept in-page WebCrypto AES-CBC decryption.

## Unresolved Questions

* None.

## Risks and Mitigations

* **Risk — Same-origin vs. out-of-process `streamembed.aspx` iframe placement and cross-frame identity**: Depending on how Teams/SharePoint loads the Recap page, `streamembed.aspx` may be a top-level/OOPIF target, a nested OOPIF under `xplatplugins.aspx`, or a same-origin child `<iframe>` inside `xplatplugins.aspx`.
  * **Mitigation**: Compute fixed-point transitive `ParentID` iframe ownership in `expandOwnedFrames` / `isTargetInCaptureScope` (`t.ParentID == capturePageID || ownedFrames.ids[t.ParentID] || ownedFrames.contains(t)`), prioritize `streamembed.aspx` targets, and traverse accessible same-origin child `<iframe>` windows (`contentWindow`, depth `<= 3`, skipping `isAuthRedirectURL` frames) inside `extractRecapAudioJS`, validating each frame's own `g_fileInfo` identity before binding a per-player capture context.
* **Risk — MSE `SourceBuffer` quota exhaustion (`QuotaExceededError`), `tfdt === 0` deduplication, or mid-stream Path A fallback**:
  * **Mitigation**: Key collected MSE `audio/mp4` media segments by their ISO BMFF `moof -> traf -> tfdt` `baseMediaDecodeTime` (checking `dts !== null && Number.isFinite(dts)` so `dts === 0` is preserved, breaking box walks on `boxSize < 8`), hook `MediaSource.prototype.removeSourceBuffer` to ignore the `video/mp4` no-op shim, evict buffered ranges `> 30s` behind `bufEnd`, and if Path A fails mid-stream, clear the Path A active flag and resume `<video>.play()` at `playbackRate = 16.0` so Path B finishes collection seamlessly.
* **Risk — Unbounded memory growth or excessive segment count on oversized streams**:
  * **Mitigation**: Enforce `maxRecapAudioDurationSec` (`4200s`), `maxRecapAudioSourceBytes` (`256 MiB`), `maxRecapAudioPCMBytes` (`360 MiB`), and `maxRecapAudioSegments = 2400` (`1 <= Math.ceil(ppd / wsd) <= 2400`) across a shared cumulative byte counter (including cumulative duration/PCM tracking in the per-segment decode fallback), and ensure `extractRecapAudioJS` and `setLastErr` prioritize resource-limit and identity errors over `isDownloadBlocked=true`.
* **Risk — Regressing existing `TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked` unit tests or leaking `oneDrive.transcode` query tokens**:
  * **Mitigation**: Gate stream capture on a verified playable stream surface with a short initial-activity timeout, enforce an in-page `AbortController` deadline with idempotent `finally` cleanup, strip non-identity query tokens (`tempauth`, `access_token`, signatures) from `mediaUrl`, and never interpolate raw URLs into JS error messages.

