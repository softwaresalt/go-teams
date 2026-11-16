---
title: High-speed audio-only MP3 capture on download-blocked SharePoint Meeting Recaps implementation plan
description: Implementation plan for 031-F / 031-S covering stash entry 9A16B213
source: docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md
deliberation_id: 020-DL
feature: 031-F
shipment: 031-S
source_stash:
  - 9A16B213
date: 2026-11-16
status: approved
tags:
  - audio
  - recap
  - browser
  - sharepoint
  - mp3
---

## Problem Frame

Following the accepted deliberation in [2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md](../decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md) (`020-DL`), stash entry **`9A16B213`** (`priority: high`, `kind: feature`) is planned under covering feature **`031-F`** and shipment **`031-S`**, sequenced immediately after `030-S` and before `010-S` in the shipment dependency DAG **`030-S -> 031-S -> 010-S`**.

When a SharePoint Meeting Recap enforces view-only recording permissions (`window.g_fileInfo.isDownloadBlocked == true` in `streamembed.aspx`), direct media download endpoints (`downloadUrl`, `streamUrl`, `mediaUrl`) fail with HTTP `403` or return `Content-Type: text/html` error pages (`AccessDenied.aspx`), and the `<video>` element's `blob:` URL is backed by MediaSource Extensions (`MSE`) fed from SharePoint's `oneDrive.transcode` endpoint. Currently, `extractRecapAudioJS` in `internal/browser/recap.go` rejects `isDownloadBlocked=true` with `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`, causing `TestMeetingCaptureAudio` in `internal/browser/meeting_integration_test.go` to skip (`t.Skip`).

To capture MP3 audio at `14x-100x` real-time speed on download-blocked SharePoint Meeting Recaps without external tools or FFmpeg, `go-teams` must:

1. **Reach `streamembed.aspx` across all SharePoint iframe topologies (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`)**:
   * Compute transitive `ParentID` iframe ownership (`expandOwnedFrames`) in `captureRecapAudioFromTargets` and prioritize `streamembed.aspx` targets before `xplatplugins.aspx` and outer `page` targets.
   * Traverse accessible same-origin child `<iframe>` windows (`contentWindow` / `contentDocument`) inside `extractRecapAudioJS` so `streamembed.aspx` is reached even when it is embedded as a same-origin child `<iframe>` inside `xplatplugins.aspx`.
   * Retain the verified transcript `ItemID` (`VroomItemId`) on `chromeMeeting` during `extractRecapFromTargets` so `captureRecapAudioFromTargets` always has the expected `DriveItemID` even when the caller's Recap URL carried only `threadId`.
2. **Collect and decrypt only the `audio/mp4` (`mp4a.40.2`) fMP4 stream at `14x-100x` real-time speed while stripping `video/mp4` (`internal/browser/recap.go`)**:
   * Install cleanup-safe hooks (`try ... finally` restoring all patched browser globals) on `window` and accessible same-origin child frame windows BEFORE clicking `'Play video'` or calling `video.play()` (with `video.muted = true; video.volume = 0;`).
   * **Path A (WebCrypto AES-CBC + parallel `oneDrive.transcode` segment fetch/decrypt at `50x-100x` real-time)**: Hook `window.crypto.subtle.decrypt`, `window.fetch`, and `XMLHttpRequest.prototype.open`/`send`/`setRequestHeader`. Capture the authenticated `oneDrive.transcode` audio URL template (`part=initsegment` + `part=mediasegment` with `track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd`, or rewrite a `track=video` transcode URL to `track=audio&quality=audcopy`) and the AES-CBC `CryptoKey` and IV (supporting both fixed session IV and 16-byte prefix IV). Once captured and verified to decrypt into valid ISO BMFF fMP4 boxes (`ftyp`/`moov` with `'mp4a'`/`'soun'` for `initsegment` and `styp`/`sidx`/`moof` for `mediasegment`), pause `<video>`, drop `track=video` requests, and fetch + decrypt all audio segments `k = 0 .. Math.ceil(ppd / wsd) - 1` (`segmentTime = k * wsd`) with bounded concurrency (`8`), enforcing `maxRecapAudioDurationSec` and `maxRecapAudioSourceBytes` pre-allocation limits.
   * **Path B (MSE `MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hook at `14x-40x` real-time)**: Hook `MediaSource.prototype.addSourceBuffer` and `SourceBuffer.prototype.appendBuffer`. Strip out the `video/mp4` track (returning a lightweight `EventTarget`-compatible no-op `SourceBuffer` shim for `video/mp4` and/or dropping `video/mp4` media appends so `MediaSource` never stalls on video decoding or downloads video segments). Collect only the `audio/mp4` (`mp4a.40.2`) `initsegment` (`ftyp`/`moov`) and deduplicated `mediasegment` (`styp`/`sidx`/`moof`) buffers keyed and sorted chronologically by ISO BMFF `moof -> traf -> tfdt` `baseMediaDecodeTime`. Drive playback with `video.muted = true`, `video.playbackRate = 16.0`, and `video.currentTime` stepping to the end of `audioSB.buffered` (evicting buffered ranges `> 30s` behind `bufEnd` to prevent `QuotaExceededError`) until all segments up to `duration` / `ppd` are collected.
3. **Decode to 44.1 kHz mono PCM and encode to MP3 (`internal/browser/recap.go`)**:
   * Reassemble `[initSegment, ...orderedMediaSegments]` into a contiguous `audio/mp4` fMP4 stream (`<= maxRecapAudioSourceBytes`), decode via `AudioContext({ sampleRate: 44100 }).decodeAudioData` (with per-segment zeroed-`tfdt` fallback if single-buffer decode fails on multi-segment streams) into 44.1 kHz mono `Int16Array` PCM chunks (`<= maxRecapAudioPCMBytes` and `<= maxRecapAudioDurationSec`), and encode to MP3 in Go via `audio.NewEncoder(w, audio.SampleRate)`.

### Explicit Out-of-Scope Boundaries

* Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, and `3F6BA819` remain untouched in `active` stash state.
* No external tools (`ffmpeg`, `yt-dlp`), CGO, or new `go.mod` dependencies are added.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Support high-speed (`14x-100x` real-time) audio-only MP3 capture on download-blocked SharePoint Meeting Recaps (`g_fileInfo.isDownloadBlocked=true` in `streamembed.aspx`) by triggering muted in-page playback (`'Play video'`), stripping `video/mp4`, collecting/decrypting `audio/mp4` (`mp4a.40.2`) `part=initsegment` + `part=mediasegment` (`track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd` via `oneDrive.transcode`) via WebCrypto AES-CBC (`window.crypto.subtle.decrypt`) parallel fetch/decrypt and MSE `MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hooks with `16x` `playbackRate` and `currentTime` stepping, decoding via `AudioContext.decodeAudioData` to 44.1 kHz mono PCM, and encoding to MP3 while supporting same-origin child `<iframe>` and transitive `ParentID` OOPIF topologies | `9A16B213` | U1 (`031.001-T`) | `go test -race ./internal/browser/...`, `go vet ./...`, `golangci-lint run` |
| R2: Add deterministic `node -e` unit tests for Path A (WebCrypto AES-CBC + parallel `oneDrive.transcode` fetch/decrypt), Path B (MSE `addSourceBuffer` + `appendBuffer` with `video/mp4` stripping, `16x` `playbackRate`, `currentTime` stepping, and `tfdt` deduplication/ordering), same-origin child `<iframe>` traversal, transitive `ParentID` iframe ownership, and resource-limit enforcement in `internal/browser/recap_test.go`; add real headless Chromium E2E test `TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd` in `internal/browser/meeting_e2e_test.go`; and update `TestMeetingCaptureAudio` in `internal/browser/meeting_integration_test.go` to require non-empty valid MP3 capture without skipping on `isDownloadBlocked=true` | `9A16B213` | U2 (`031.002-T`) | `go test -race ./internal/browser/...`, `go test -tags=integration ./internal/browser/...` |
| R3: Document download-blocked `streamembed.aspx` high-speed audio-only MP3 capture in `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` | `9A16B213` | U3 (`031.003-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, `backlogit doctor` |

## Implementation Units

### U1 (`031.001-T`) — Add high-speed `streamembed.aspx` audio-only fMP4 capture and AES-CBC/MSE interception (`internal/browser`)

* **Domain:** code (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`). **Posture:** test-first (Principle II — Wave-Staged Red-Phase Harness: stage U2 red-phase tests in `logs/staged-harnesses/031.002-T/` during U1 red-phase verification and activate in U2). **Size:** `M` | **Complexity:** `medium`.
* **Files (2 files, 4 functions):** `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`.
* **What:**
  1. **In `internal/browser/recap.go` (`extractRecapAudioJS`)**:
     * Discover and validate accessible player windows (`collectPlayerWindows`): traverse `window` and accessible same-origin child/grandchild `<iframe>` elements (`iframe.contentWindow` / `iframe.contentDocument`) up to depth `<= 3` using a visited-`Window` `Set`, skipping any frame whose `location.href` is an auth redirect (`isAuthRedirectURL`). Run `checkIdentity(win.location.href)` and `checkField` against each individual frame's own `g_fileInfo` (`itemId`, `driveItemId`, `VroomItemID`, `VroomItemId`, `fileName`, `name`) BEFORE merging its metadata or hooking its globals — failing closed immediately with `'recap audio recording identity mismatch'` if any frame conflicts with `recapExpectedIdentity`, and binding a per-player capture context (`playerWin`, `playerDoc`, `info`) before evaluation so unrelated child frames cannot contaminate identity or segment state.
     * Extract a shared helper `decodeAudioBufferToPCMChunks(buffer, state)` that tracks cumulative `state.totalDurationSec <= maxRecapAudioDurationSec` and `state.totalPCMBytes <= maxRecapAudioPCMBytes` across single-buffer or multi-segment fallback decodes, downmixes and resamples to 44.1 kHz mono `Int16Array` in 16384-sample chunks, and base64-encodes `pcmChunks`.
     * Propagate any `limitError` or `'recap audio recording identity mismatch'` thrown during direct candidate inspection immediately. When direct candidates are empty or fail without a fatal limit/identity error and a playable stream surface exists (`typeof playerWin.MediaSource !== 'undefined'` with a play button / `<video>` / `oneDrive.transcode`, or `typeof media.play === 'function'`, or `info.transcodeUrl`), invoke `captureStreamEmbedAudio()` (and immediately rethrow any `limitError` or identity error thrown by `captureStreamEmbedAudio()` so `blockedErr` never masks resource-limit or identity rejections):
       * **Pre-checks & lifecycle deadline**:
         - Verify recording identity (`if (enforceIdentity && !baseIdentityMatched)` require a verified identity match on the discovered `oneDrive.transcode` URL before fetching segments; fail closed with `'recap audio recording identity unverified'` or `'recap audio recording identity mismatch'`).
         - Check known duration metadata (`media.duration`, `info.duration`, `info.durationSeconds`) against `maxRecapAudioDurationSec` and `maxRecapAudioPCMBytes` BEFORE starting playback or fetching segments.
         - Create an in-page `AbortController` and deadline timer (plus a short initial-activity timeout when no `oneDrive.transcode` request or `SourceBuffer` creation occurs after triggering playback so inert `blob:` `<video>` unit-test mocks fail fast with `isDownloadBlocked=true`).
       * **fMP4 box helpers**:
         - `readFourCC(u8, off)`, `isFMP4InitSegment(u8)` (`ftyp` or `moov` at offset 4), `isFMP4MediaSegment(u8)` (`styp`, `sidx`, `moof`, or `emsg` at offset 4), `classifyInitSegmentTrack(u8)` (scans `initsegment` for `'mp4a'`/`'soun'` vs `'avc1'`/`'hev1'`/`'hvc1'`/`'vp09'`/`'av01'`/`'vide'`), `parseFMP4DecodeTime(u8)` (walks `moof -> traf -> tfdt` version 0/1 `baseMediaDecodeTime` or `sidx` `earliest_presentation_time`, breaking safely when `boxSize < 8` for `boxSize !== 1`), and `zeroFMP4DecodeTime(u8)` (zeros `tfdt.baseMediaDecodeTime` / `sidx.earliest_presentation_time` in a copy for per-segment decode fallback).
       * **Hook installation (`installWindowHooks(win)`) on the verified player window (with guaranteed idempotent restoration of hooks, timers, listeners, and `AbortController.abort()` in `finally`)**:
         - Hook `win.crypto.subtle.decrypt(algorithm, key, data)` (when available): invoke original `decrypt`, clone `algorithm.iv`, inspect the decrypted `ArrayBuffer` for `isFMP4InitSegment` (`classifyInitSegmentTrack(u8) === 'audio'` -> record `cryptoInitSegment` and lock `audioCryptoKeyInfo = { algorithm, key, iv, prefixIV }`) and `isFMP4MediaSegment` (record fallback `cryptoKeyInfo` only if `audioCryptoKeyInfo` is not yet set; never mix unattributed `crypto.subtle.decrypt` media buffers into the segment map so `video/mp4` segments are never mixed with audio).
         - Hook `win.fetch` and `win.XMLHttpRequest.prototype.open`/`send`/`setRequestHeader`:
           - Track `XMLHttpRequest` headers per XHR instance via a `WeakMap`. Capture only allowlisted headers (`accept`, `authorization`, `x-ms-*`, `prefer`) from requests validated as same-origin `https:` `oneDrive.transcode` URLs, and replay captured headers only to that exact `https:` origin.
           - Inspect request URLs for `oneDrive.transcode` or `part=initsegment` / `part=mediasegment`. Capture the audio transcode URL template (`track=audio` / `quality=audcopy`, or rewrite `track=video` to `track=audio&quality=audcopy`), `wsd`, and `ppd`.
           - Strip `video/mp4` network requests: when `part=mediasegment` and `track=video` while `pathAActive` is true, short-circuit/abort the video segment request.
         - Hook `win.MediaSource.prototype.addSourceBuffer(mimeType)`, `win.MediaSource.prototype.removeSourceBuffer(sb)`, and `win.SourceBuffer.prototype.appendBuffer(data)`:
           - For `video/mp4`: return a lightweight `EventTarget`-compatible no-op `SourceBuffer` shim (`__isRecapVideoShim: true`, `updating`, `buffered` reporting `[0, maxBufferedEnd]`, `mode: 'segments'`, `timestampOffset: 0`, `appendWindowStart: 0`, `appendWindowEnd: Infinity`, `appendBuffer` and `remove` firing async `updatestart`/`update`/`updateend`, `abort`, `changeType`) and ignore `__isRecapVideoShim` in `MediaSource.prototype.removeSourceBuffer` so `NotFoundError` is never thrown and `<video>` never stalls waiting for video frames. If a native `video/mp4` `SourceBuffer` was created before hooks ran, drop `video/mp4` media segments in `appendBuffer`.
           - For `audio/mp4` (`mp4a.40.2`): collect `mseInitSegment` (`ftyp`/`moov`) and deduplicated `mseSegments` (`Map` keyed by `dts !== null && dts !== undefined && Number.isFinite(dts) ? 'dts:' + dts : 'seq:' + seq++` so `dts === 0` is preserved), checking shared cumulative `streamSourceBytes + u8.byteLength <= maxRecapAudioSourceBytes` BEFORE storing each segment. On `updateend`, mute `<video>`, set `playbackRate = 16.0`, step `video.currentTime` to `bufEnd - 0.05`, and evict buffered audio `> 30s` behind `bufEnd` when `!sb.updating`.
       * **Playback trigger**:
         - Click `'Play video'` button (`button[aria-label="Play video"], button[title="Play video"], button[data-automationid="playButton"], button[aria-label="Play"], .vjs-big-play-button, [role="button"][aria-label="Play video"]`), mute `<video>` / `<audio>` (`muted = true; volume = 0; playbackRate = 16.0`), and call `play()`.
       * **Path A execution (parallel `oneDrive.transcode` fetch + WebCrypto AES-CBC decrypt at `50x-100x` speed)**:
         - Once `transcodeAudioTemplateUrl` (with `wsd > 0` and `ppd > 0`) is discovered and either plaintext fMP4 or `audioCryptoKeyInfo`/`cryptoKeyInfo` is available:
           - Validate identity on `transcodeAudioTemplateUrl` (`checkIdentity`), validate duration derived from `ppd` (`ppd >= 1e6 ? ppd / 1e7 : ppd <= maxRecapAudioDurationSec`), and validate bounded segment count `totalSegments = Math.ceil(ppd / wsd)` (`1 <= totalSegments <= 2400`, falling back to Path B if out of range).
           - Set `pathAActive = true` and pause `<video>` so the player stops making competing requests.
           - Fetch (using `AbortController.signal` and streaming byte-cap reader enforcing shared `streamSourceBytes <= maxRecapAudioSourceBytes`) and decrypt `part=initsegment&track=audio&quality=audcopy` (if not already in `cryptoInitSegment` / `mseInitSegment`), verifying `isFMP4InitSegment(u8) && classifyInitSegmentTrack(u8) !== 'video'`.
           - Fetch and decrypt all audio `part=mediasegment&track=audio&quality=audcopy&segmentTime=<k*wsd>` segments for `k = 0 .. totalSegments - 1` in parallel batches (`concurrency = 8`), enforcing `streamSourceBytes <= maxRecapAudioSourceBytes` before buffering/decrypting each segment and verifying `isFMP4MediaSegment`.
           - If any Path A fetch or decryption fails with a non-limit/non-identity error (e.g., per-segment rotating keys), set `pathAActive = false`, reset `streamSourceBytes` to the MSE buffer total, and resume `<video>.play()` with `muted = true; playbackRate = 16.0` so Path B seamlessly finishes collection without hanging.
       * **Path B execution (MSE `appendBuffer` + `16x` `playbackRate` + `currentTime` stepping at `14x-40x` speed)**:
         - When Path A is not used or falls back, drive `<video>` with `muted = true`, `playbackRate = 16.0`, and `currentTime` stepping until `mseInitSegment` and all `mseSegments` up to target duration (`media.duration` / `info.duration` / `ppd`) or `ended` are collected.
       * **Concatenation, Web Audio decode, and URL sanitization**:
         - Concatenate `[initSegment, ...orderedMediaSegments]` (`<= maxRecapAudioSourceBytes`), decode via `AudioContext({ sampleRate: 44100 }).decodeAudioData` (falling back to per-segment `[initSegment + zeroFMP4DecodeTime(seg)]` decoding with cumulative `totalDurationSec` and `totalPCMBytes` enforcement in `decodeAudioBufferToPCMChunks` if single-buffer decode fails on multi-segment input), sanitize `selectedMediaUrl` by stripping secret query tokens (`tempauth`, `access_token`, `sig`, `se`, `sp`, `sv`) while preserving identity parameters (`driveItemId`, `id`, `uniqueId`), never interpolate raw URLs into thrown `Error` messages, and return `{ pageUrl, itemId, fileName, mediaUrl: sanitizedMediaUrl, sourceBytes, durationSec, pcmChunks }`.
  2. **In `internal/browser/recap.go` (`captureRecapAudioFromTargets`, `expandOwnedFrames`, and `extractRecapTranscriptJS`)**:
     * Add `expandOwnedFrames(targets []cdpTargetInfo, capturePageID string, base ownedFrameSet) ownedFrameSet` that allocates a fresh `ids` map and iterates to a fixed point over `targets` (`if t.ParentID != "" && (t.ParentID == capturePageID || out.ids[t.ParentID]) { out.ids[t.ID] = true }`), and update `isTargetInCaptureScope` to check `if t.ParentID != "" { return t.ParentID == capturePageID || ownedFrames.ids[t.ParentID] || ownedFrames.contains(t) }`.
     * In `captureRecapAudioFromTargets`: check `ctx.Err()` at the start of each target iteration, skip `isAuthRedirectURL(target.URL)`, prioritize `streamembed.aspx` targets before `xplatplugins.aspx` and `page` targets, and update `setLastErr` so resource-limit (`exceeds maximum supported size/duration`) and recording-identity (`identity mismatch` / `identity unverified`) errors always take precedence over `isDownloadBlocked=true`.
     * In `extractRecapTranscriptJS`: prioritize `fi.VroomItemID || fi.VroomItemId || fi.itemId || fi.driveItemId || fi.id || fi.docId` when populating `itemId` so `rawRecapPayload.ItemID` carries the canonical `VroomItemId` DriveItem token.
  3. **In `internal/browser/meeting_chrome.go` (`chromeMeeting`)**:
     * Add `recapItemID string` on `chromeMeeting`. Update `extractRecapFromTargets` to record the verified `rawRecapPayload.ItemID` into `m.recapItemID` (when matching `^[A-Za-z0-9_-]+$`), and in `CaptureAudio`, if `ExtractMeetingIdentity(meetingURL).DriveItemID == ""` and `m.recapItemID != ""`, attach `driveItemId` via `net/url` (`u, err := url.Parse(meetingURL); q := u.Query(); q.Set("driveItemId", m.recapItemID); u.RawQuery = q.Encode()`) before calling `captureRecapAudioFromTargets`.
  4. **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, and `golangci-lint run`.

### U2 (`031.002-T`) — Add unit, Chromium E2E, and live integration tests for download-blocked `streamembed.aspx` MP3 capture (`internal/browser`)

* **Domain:** test (`internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `031.001-T` (`blocks`).
* **Files (3 files, 3 functions, 3 test scenarios):** `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`.
* **What:**
  1. **In `internal/browser/recap_test.go`**:
     * Add `TestRecapJSStreamEmbedHighSpeedAudioCaptureWhenDownloadBlocked`:
       * Subtest `"path-a-webcrypto-aes-cbc-parallel-onedrive-transcode"`: executes `extractRecapAudioJS` via `node -e` with `g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true }`, a `'Play video'` button (`button[aria-label="Play video"]`), real Node WebCrypto AES-CBC (`crypto.subtle.encrypt` / `crypto.subtle.decrypt`), and a mock `oneDrive.transcode` endpoint (`part=initsegment` + `part=mediasegment` with `track=audio&quality=audcopy`, `wsd=50000000`, `ppd=150000000`). Asserts that clicking `'Play video'` triggers initial segment decryption, intercepts the AES-CBC key/IV and `oneDrive.transcode` template URL, strips `track=video` requests, fetches and decrypts all 3 audio segments (`segmentTime = 0, 50000000, 100000000`), decodes via `AudioContext.decodeAudioData` to 44.1 kHz mono PCM, and returns valid `pcmChunks`.
       * Subtest `"path-b-mse-sourcebuffer-hook-16x-currenttime-stepping-and-child-iframe"`: executes `extractRecapAudioJS` via `node -e` against a parent `xplatplugins.aspx` window containing a same-origin child `<iframe src="https://contoso-my.sharepoint.com/_layouts/15/streamembed.aspx">` with `g_fileInfo = { VroomItemId: 'recording-a', isDownloadBlocked: true, duration: 15 }`, `MediaSource` + `SourceBuffer` appending `video/mp4` (`avc1`) and `audio/mp4; codecs="mp4a.40.2"` segments with `moof -> traf -> tfdt` timestamps (including an out-of-order and duplicate segment). Asserts that `video/mp4` is stripped, `<video>` is muted with `playbackRate == 16` and `currentTime` stepped across buffered ranges, duplicate `tfdt` segments are deduplicated and sorted chronologically, and 44.1 kHz mono `pcmChunks` are returned.
       * Subtest `"transitive-parentid-scope-and-stream-resource-limits"`: verifies in Go that `expandOwnedFrames` + `isTargetInCaptureScope` includes a nested `streamembed.aspx` OOPIF whose `ParentID` is an owned `xplatplugins.aspx` OOPIF (while excluding unowned tabs/iframes), and verifies via `node -e` that oversized `oneDrive.transcode` / MSE streams exceeding `maxRecapAudioSourceBytes` or `maxRecapAudioDurationSec` fail closed before `decodeAudioData` with `recap audio exceeds maximum supported size/duration`.
  2. **In `internal/browser/meeting_e2e_test.go`**:
     * Add `TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd`: launches headless Chromium via `NewLiveMeeting` against an `httptest.NewServer` serving a simulated Teams Recap page embedding a `streamembed.aspx` iframe (`g_fileInfo.isDownloadBlocked = true`) that uses real browser WebCrypto `crypto.subtle` AES-CBC to encrypt/decrypt `oneDrive.transcode` audio segments (`part=initsegment` + `part=mediasegment`, `track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd`) and synthesizes a real Web Audio WAV/fMP4 stream when `'Play video'` is clicked. Verifies that `m.Join(ctx)` extracts the transcript and `m.CaptureAudio(ctx, &buf)` completes in seconds and writes a non-empty valid MP3 stream (`buf.Len() > 0`, MPEG sync `0xFF`, `(b[1]&0xE0) == 0xE0`).
  3. **In `internal/browser/meeting_integration_test.go`**:
     * In `TestMeetingCaptureAudio`: increase the test timeout to `3 * time.Minute` (`ctx, cancel := context.WithTimeout(bctx, 3*time.Minute)`), remove the `if err != nil && strings.Contains(err.Error(), "isDownloadBlocked=true") { t.Skip(...) }` branch in the `rm.IsRecap()` block, and require `require.NoError(t, err, "CaptureAudio failed for RecapMeeting")`, `require.NotZero(t, buf.Len(), "expected captured MP3 bytes")`, and valid MPEG frame sync header (`b := buf.Bytes(); require.True(t, len(b) >= 2 && b[0] == 0xff && (b[1]&0xe0) == 0xe0, "expected valid MP3 frame sync header")`).
  4. **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration ./internal/browser/...`.

### U3 (`031.003-T`) — Document download-blocked `streamembed.aspx` high-speed audio-only MP3 capture (`docs/`)

* **Domain:** docs (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **Depends on:** `031.001-T` (`blocks`).
* **Files (2 files):** `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`.
* **What:**
  1. **In `docs/ARCHITECTURE.md` (`### Finite Meeting Recap export`)**:
     * Update step 3 to describe both standard direct media decoding and high-speed (`14x-100x` real-time) audio-only capture on download-blocked SharePoint Meeting Recaps (`g_fileInfo.isDownloadBlocked=true` in `streamembed.aspx`) via muted `'Play video'` playback trigger, `video/mp4` track stripping, WebCrypto AES-CBC (`window.crypto.subtle.decrypt`) parallel `oneDrive.transcode` audio segment fetch/decrypt (`part=initsegment` + `part=mediasegment`, `track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd`), and MSE `MediaSource.prototype.addSourceBuffer` / `SourceBuffer.prototype.appendBuffer` hooks with `16x` `playbackRate` and `currentTime` stepping, followed by `AudioContext.decodeAudioData` to 44.1 kHz mono PCM and `internal/audio.Encoder` MP3 encoding.
  2. **In `docs/USER_GUIDE.md` (`## Compiled executable and explicit output file`)**:
     * Update the Meeting Recap audio paragraph to explain that `--audio`, `--recap-audio`, and `capture_audio: true` also capture MP3 audio from view-only (`isDownloadBlocked=true`) SharePoint Meeting Recaps at high speed by extracting and decoding only the in-page audio stream while stripping video, subject to recording-identity verification and size/duration caps.
  3. **Verify:** Run `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `backlogit doctor`.

## Dependency Graph

```text
031.001-T (U1: internal/browser/recap.go + meeting_chrome.go)
    |
    +---> 031.002-T (U2: internal/browser/recap_test.go + meeting_e2e_test.go + meeting_integration_test.go)
    |
    +---> 031.003-T (U3: docs/ARCHITECTURE.md + docs/USER_GUIDE.md)
```

* **Shipment sequencing DAG**:
  * Remove `010-S -> 030-S` direct edge (`backlogit dep remove 010-S 030-S`).
  * Add `031-S -> 030-S` (`backlogit dep add 031-S 030-S --type blocks` — `030-S` is `shipped`, placing `031-S` in `ready_set`).
  * Add `010-S -> 031-S` (`backlogit dep add 010-S 031-S --type blocks` — sequencing `010-S` after `031-S`).

## Decisions and Rationale

1. **Dual-path in-browser collection (`Path A` WebCrypto AES-CBC parallel `oneDrive.transcode` fetch/decrypt + `Path B` MSE `SourceBuffer` hook with `16x` `playbackRate` and `currentTime` stepping)**:
   * Intercepting `window.crypto.subtle.decrypt` and `oneDrive.transcode` URLs (`Path A`) allows parallel HTTP/2 fetching and decryption of all 5-second `audio/mp4` (`mp4a.40.2`, `track=audio&quality=audcopy`) segments (`segmentTime = k * wsd` up to `ppd`), achieving `50x-100x` real-time speed without waiting for the player's sequential buffer loop.
   * Simultaneously installing the MSE `MediaSource.prototype.addSourceBuffer` + `SourceBuffer.prototype.appendBuffer` hook (`Path B`) with a lightweight `EventTarget` no-op `SourceBuffer` shim for `video/mp4`, `16x` `playbackRate`, and `currentTime` stepping (`14x-40x` real-time speed) guarantees that even if a SharePoint player version rotates keys per segment or uses an alternative manifest URL, all decrypted `audio/mp4` segments are still captured in chronological `tfdt` order without downloading or decoding `video/mp4`.
2. **Same-origin child `<iframe>` traversal + transitive `ParentID` OOPIF scope**:
   * Because `xplatplugins.aspx` and `streamembed.aspx` share the `https://<tenant>-my.sharepoint.com` origin, Chromium often renders `streamembed.aspx` as a same-origin child `<iframe>` inside `xplatplugins.aspx` rather than a separate top-level entry in `/json/list` (or as a nested OOPIF whose `ParentID` is `xplatplugins.aspx`). Supporting both same-origin `iframe.contentWindow` traversal in `extractRecapAudioJS` and transitive `ParentID` expansion in `expandOwnedFrames` handles every SharePoint iframe hierarchy cleanly.
3. **Preserving fail-closed identity and resource limits**:
   * Every `oneDrive.transcode` or MSE stream is gated on `recapExpectedIdentity` (`VroomItemId` / `DriveItemID` / `RecordingName`), `maxRecapAudioDurationSec` (`4200s`), `maxRecapAudioSourceBytes` (`256 MiB`), and `maxRecapAudioPCMBytes` (`360 MiB`) prior to `decodeAudioData` and PCM allocation.

## Risks and Caveats

* **3-Threat Security Model**:
  1. *Most likely threat — Unbounded memory allocation in the browser renderer or Go process when collecting fMP4 segments on a multi-hour recording*: Mitigated by checking `ppd` / duration against `maxRecapAudioDurationSec` (`4200s`) before segment collection, enforcing `sourceBytes <= maxRecapAudioSourceBytes` (`256 MiB`) on every segment in both Path A and Path B before buffering or decrypting, and enforcing `maxRecapAudioPCMBytes` (`360 MiB`) before allocating `Int16Array` PCM buffers.
  2. *Highest impact threat — Cross-tab or mismatched-recording audio leakage from an unowned tab or iframe*: Mitigated by restricting transitive `ParentID` expansion strictly to descendants of `ownerID` (`expandOwnedFrames`), verifying `hasMeetingIdentityConflict` and `hasRecapAudioIdentityMatch` on both the JS side (`checkIdentity`) and Go side (`captureRecapAudioFromTargets`), and retaining the verified transcript `ItemID` (`VroomItemId`) on `chromeMeeting`.
  3. *Most subtle threat — Leaving monkey-patched `window.fetch`, `XMLHttpRequest`, `crypto.subtle.decrypt`, `MediaSource.prototype.addSourceBuffer`, or `SourceBuffer.prototype.appendBuffer` active or leaving `<video>` unpaused after `extractRecapAudioJS` finishes*: Mitigated by wrapping `captureStreamEmbedAudio` in a `try ... finally` block that restores all original prototypes/functions across all hooked windows and pauses any active `<video>` / `<audio>` elements before returning.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, zero `unsafe`, zero lint findings, all errors checked |
| II. Test-First Development | Compliant — unit tests (`recap_test.go`), real Chromium E2E test (`meeting_e2e_test.go`), and live `-tags=integration` test (`meeting_integration_test.go`) |
| III. Workspace Isolation | Compliant — all temporary test outputs use `t.TempDir()` or repo-local `logs/` |
| IV. CLI Workspace Containment | Compliant — zero writes outside the repository or configured output directory |
| V. Structured Observability | Compliant — structured `log/slog` logging with zero unredacted URLs or tokens; mandatory post-merge context compaction (P-020) enforced at closure |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); 3 width-isolated tasks (`<= 3` files each) |
| VII. Destructive Command Approval | Compliant — no destructive commands used |
| VIII. Explicit Safety Modes | Compliant — `Careful mode` + `Freeze-scope mode` (scoped to `internal/browser` and `docs/`) + `Strict-safety` active; `ProposedAction` inventory (`PA-031-1`..`PA-031-4`) recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded command outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No exported Go signatures, CLI flags, or config schema keys are changed (`extractRecapAudioJS`, `expandOwnedFrames`, and `recapItemID` are unexported internal implementation details of `internal/browser`).
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Intercepts in-page WebCrypto `window.crypto.subtle.decrypt` and authenticated `oneDrive.transcode` / MSE streams inside the authorized `streamembed.aspx` iframe and validates recording identity (`VroomItemId` / `DriveItemID`).
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Purely additive in-memory browser stream extraction.
* **External integration, operator checkpoint, or external dependency**: **Present** — Exercises live Microsoft Teams and SharePoint `streamembed.aspx` (`isDownloadBlocked=true`) integration tests (`-tags=integration`) against `.env.local.test`.
* **High runtime, rollout, or rollback risk**: **Present** — Modifies `extractRecapAudioJS` and `captureRecapAudioFromTargets` in `internal/browser/recap.go`.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * Production Recap audio extraction JS and CDP target scope resolution (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`)
  * Browser unit, E2E, and `-tags=integration` test suites (`internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`)
  * Architecture and user guide documentation (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`)
* **Runtime verification**:
  * Run `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...`.
  * Run `go test -tags=integration -v ./internal/browser -run TestMeetingCaptureAudio` and full `go test -tags=integration ./...` against `.env.local.test` to verify that all 8 live-tenant integration tests (including `TestMeetingCaptureAudio` on a download-blocked SharePoint Meeting Recap) pass with zero skips and produce valid MP3 audio.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, releasability evidence contract, and mandatory post-merge context compaction (`compact-context` skill, P-020) recorded in `docs/closure/` during Ship operational closure.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan extends `extractRecapAudioJS` and `captureRecapAudioFromTargets` in `internal/browser/recap.go` to intercept WebCrypto AES-CBC (`window.crypto.subtle.decrypt`), `oneDrive.transcode` segment requests, and MSE `MediaSource` / `SourceBuffer` buffers across `streamembed.aspx` iframes, and updates `TestMeetingCaptureAudio` (`-tags=integration`) to require live MP3 capture on `isDownloadBlocked=true` SharePoint Meeting Recaps.
* **Safety Mode Posture (Principle VIII)**: `Careful mode` + `Freeze-scope mode` (strictly scoped to `internal/browser/` and `docs/`) + `Strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/browser-verification.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md`
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`
  * `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`

### Protected Invariants

1. **Single-Binary Pure-Go Architecture (`docs/ARCHITECTURE.md`)**: Zero external media tools (`ffmpeg`, `yt-dlp`), zero CGO, and zero changes to `go.mod` / `go.sum`. All AAC (`mp4a.40.2`) fMP4 demuxing and decoding happens inside Chromium via `AudioContext.decodeAudioData`, and all MP3 encoding happens in pure Go via `internal/audio.NewEncoder`.
2. **Per-Frame Recording Identity Isolation & Origin Containment (`recapExpectedIdentity`)**: `collectPlayerWindows` validates `checkIdentity(win.location.href)` and `checkField` on each frame's own `g_fileInfo` (up to depth `<= 3`, skipping `isAuthRedirectURL`) before binding a per-player capture context or installing hooks. Neither Path A nor Path B may extract audio from an unowned tab or from a frame whose `VroomItemId` / `DriveItemID` / `RecordingName` conflicts with or fails to match `recapExpectedIdentity`. `XMLHttpRequest` headers are tracked per instance via `WeakMap` and replayed only to the same `https:` origin using an explicit allowlist, and non-identity query tokens (`tempauth`, `access_token`, signatures) are stripped from `mediaUrl`.
3. **Pre-Allocation Resource & Segment-Count Caps and Error Precedence**: Path A and Path B enforce `maxRecapAudioDurationSec` (`4200s`), `maxRecapAudioSourceBytes` (`256 MiB`, shared cumulative counter across streamed segment reads), `maxRecapAudioPCMBytes` (`360 MiB`, including cumulative tracking across per-segment decode fallback), and `maxRecapAudioSegments = 2400` (`1 <= Math.ceil(ppd / wsd) <= 2400`) BEFORE allocating combined fMP4 buffers, calling `decodeAudioData`, or allocating `Int16Array` PCM chunks. Both `extractRecapAudioJS` and `setLastErr` in `captureRecapAudioFromTargets` preserve resource-limit and identity errors over `isDownloadBlocked=true`.
4. **Backward Compatibility with Existing Direct-Fetch and `isDownloadBlocked=true` Unit Tests**: When direct non-blob/blob candidate fetch succeeds, `extractRecapAudioJS` returns immediately without invoking stream capture. When `isDownloadBlocked=true` and no playable MSE / `oneDrive.transcode` / `'Play video'` stream surface is present (as in `TestRecapJSVroomItemIdLowercaseAndIsDownloadBlocked`) or when stream capture produces zero segments within the initial-activity timeout, `extractRecapAudioJS` MUST throw `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)`.
5. **Clean Restoration of Browser Globals and Media Elements**: `captureStreamEmbedAudio` enforces an in-page `AbortController` deadline and restores all hooked methods (`window.fetch`, `XMLHttpRequest.prototype.open`/`send`/`setRequestHeader`, `crypto.subtle.decrypt`, `MediaSource.prototype.addSourceBuffer`/`removeSourceBuffer`, `SourceBuffer.prototype.appendBuffer`), clears timers/listeners, and pauses any `<video>` / `<audio>` element in an idempotent `finally` block.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-031-1` | Implement `captureStreamEmbedAudio` (Path A WebCrypto AES-CBC + parallel `oneDrive.transcode` fetch/decrypt and Path B MSE `addSourceBuffer`/`appendBuffer` hook with `video/mp4` stripping, `16x` `playbackRate`, and `currentTime` stepping), per-frame validated same-origin child `<iframe>` traversal, fixed-point transitive `ParentID` iframe scope (`expandOwnedFrames`), and `net/url`-encoded `recapItemID` retention | `internal/browser/recap.go`, `internal/browser/meeting_chrome.go` | Production browser Recap audio extraction enhancement | `high` | Pre-authorized under `Autopilot Mode` / `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-031-2` | Add `node -e` unit tests (`TestRecapJSStreamEmbedHighSpeedAudioCaptureWhenDownloadBlocked`), headless Chromium E2E test (`TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd`), and update `TestMeetingCaptureAudio` (`-tags=integration`) to assert MP3 capture on `isDownloadBlocked=true` | `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go` | Browser unit, E2E, and `-tags=integration` test updates | `moderate` | Pre-authorized under `Autopilot Mode` / `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-031-3` | Update architecture and user guide documentation for download-blocked SharePoint Recap high-speed audio-only MP3 capture | `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md` | Documentation update | `low` | Pre-authorized under `Autopilot Mode` / `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 commit |
| `PA-031-4` | Execute `-tags=integration` live-tenant browser verification (`go test -tags=integration ./...`) against `.env.local.test` to verify high-speed MP3 capture on a live download-blocked SharePoint Meeting Recap | `internal/browser` and `internal/cli` `-tags=integration` suites | External call (live-tenant headless browser verification against Microsoft Teams / SharePoint Recap) | `high` | Pre-authorized under `Autopilot Mode` / `DARK_MODE_ACTIVE` | `planned` | Cancel test context; temporary outputs cleaned via `t.TempDir()` |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages (`internal/audio`, `internal/browser`, `internal/capture`, `internal/cli`, `internal/config`, `internal/scheduler`, `internal/teams`, `internal/transcript`, `internal/updater`, `internal/version`).
  * [ ] `go test -tags=integration ./...` passes against `.env.local.test` with `TestMeetingCaptureAudio` capturing non-empty valid MP3 bytes (`PASS`, 0 skips).
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `unit_and_e2e_test_pass_rate` (`go test -race ./...` == 100%), (2) `integration_audio_capture_pass_rate` (`TestMeetingCaptureAudio` passes with `buf.Len() > 0` and valid MPEG frame sync header on download-blocked SharePoint Recap at `>= 14x` real-time speed), and (3) `tracked_secret_leak_count` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == 0`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main`.
  * **Dashboard / query**: `gh run list --workflow=ci.yml` (`test`, `test-windows`, `Secret hygiene check (tracked files)`) and local `go test -tags=integration -v ./...`.
  * **Baseline**: 100% pass rate on `go test -race ./...` and all 8 `-tags=integration` tests, 0 linter issues, 0 secret-hygiene findings.
  * **Alert threshold**: `>= 1` test failure, timeout, or CI check failure.
* **Rollback trigger & procedure**:
  * **Trigger**: Any regression in `go test -race ./...`, `go test -tags=integration ./...`, or GitHub Actions CI (`test`, `test-windows`).
  * **Procedure**: Revert the `031-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, unit tests, real-Chromium E2E tests, all 8 `-tags=integration` tests (including `TestMeetingCaptureAudio` producing valid MP3 bytes on a download-blocked Recap), and PR CI checks (`changes`, `test`, `test-windows`) pass and are recorded in `docs/closure/`.
  * `READY_WITH_CONDITIONS`: All local quality gates, unit/E2E tests, and PR CI checks pass, with any environment-specific condition documented and verified.
  * `BLOCKED`: Any pre-deploy audit check, unit/E2E/integration test, or CI job fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist`, `Security Lens Reviewer`, and `Correctness Reviewer`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `Autopilot Mode` / `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings from the 7-persona review panel remediated in-place in `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md` and `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md` prior to harvest; zero P0 findings).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-031-1`..`PA-031-4`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` (anchor route `gpt-6.1-sol`) | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 3 P2 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1, 1 P2, 3 P3 | Remediated — `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2 | Remediated — `PASS` |
| `Correctness Reviewer` | `subagent` (`multi-agent`) | 0 P0, 3 P1, 3 P2, 1 P3 | Remediated — `PASS` |
| `Learnings Researcher` | `subagent` (`multi-agent`) | 0 P0/P1 (`ALIGNED`, confidence `high`) | `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Architecture Strategist`, `Security Lens Reviewer`, `Go Reviewer`] Per-player-window identity verification, depth-bounded traversal, and in-page `AbortController` deadline cleanup**:
   * **Finding**: Merging `g_fileInfo` across all child iframes before checking identity could allow an unverified or conflicting child frame to contaminate capture state, and an interrupted CDP evaluation could leave hooks or in-flight fetches active.
   * **Remediation Applied**: Updated U1 and Protected Invariants 2 and 5 so `collectPlayerWindows` validates `checkIdentity(win.location.href)` and `checkField` on each frame's own `g_fileInfo` (depth `<= 3`, skipping `isAuthRedirectURL`) before binding a per-player capture context, and `captureStreamEmbedAudio` enforces an in-page `AbortController` deadline (plus short initial-activity timeout) with idempotent `finally` cleanup.
2. **[P1 — `Security Lens Reviewer`, `Correctness Reviewer`] Bounded segment count (`maxRecapAudioSegments = 2400`), shared cumulative byte counter, and cumulative per-segment decode limits**:
   * **Finding**: Path A needed a cap on `Math.ceil(ppd / wsd)` in case `wsd` is tiny/malformed, Path A and Path B needed a shared cumulative `streamSourceBytes` budget, and `decodeAudioBufferToPCMChunks` needed cumulative duration/PCM byte tracking across per-segment fallback decodes.
   * **Remediation Applied**: Updated U1 and Protected Invariant 3 to enforce `1 <= Math.ceil(ppd / wsd) <= 2400`, shared `streamSourceBytes <= maxRecapAudioSourceBytes` with streaming byte-capped reads, and cumulative `state.totalDurationSec <= maxRecapAudioDurationSec` and `state.totalPCMBytes <= maxRecapAudioPCMBytes`.
3. **[P1 — `Go Reviewer`, `Correctness Reviewer`] Error precedence (`limitError` / identity over `isDownloadBlocked=true`), Path A -> Path B `<video>.play()` resumption, fixed-point `expandOwnedFrames`, and audio-only `cryptoKeyInfo` attribution**:
   * **Finding**: `extractRecapAudioJS` and `setLastErr` in `captureRecapAudioFromTargets` needed to preserve resource-limit and identity errors over `isDownloadBlocked=true`; Path A needed to attribute `audioCryptoKeyInfo` from the verified `audio/mp4` `initsegment` and resume `<video>.play()` at `16x` if falling back to Path B; and `isTargetInCaptureScope` needed to check `t.ParentID == capturePageID || ownedFrames.ids[t.ParentID] || ownedFrames.contains(t)` with fixed-point `expandOwnedFrames`.
   * **Remediation Applied**: Updated U1.1 and U1.2 with all four safeguards.
4. **[P1/P2 — `Go Reviewer`, `Security Lens Reviewer`, `Architecture Strategist`] Canonical `VroomItemId` priority in `extractRecapTranscriptJS`, `net/url` encoding for `recapItemID`, same-origin `WeakMap` header binding, and `mediaUrl` token redaction**:
   * **Remediation Applied**: Updated U1.1, U1.2, and U1.3 to prioritize `fi.VroomItemID || fi.VroomItemId || fi.itemId || fi.driveItemId` in `extractRecapTranscriptJS`, attach `driveItemId` via `net/url` in `CaptureAudio`, bind XHR headers via `WeakMap` with a strict same-origin `https:` allowlist, and strip secret query parameters (`tempauth`, `access_token`, signatures) from `mediaUrl`.
5. **[P2/P3 — `Correctness Reviewer`, `Constitution Reviewer`] `dts === 0` truthiness guard, `boxSize < 8` loop guard, `removeSourceBuffer` shim handling, Wave-Staged Red-Phase Harness (Principle II), and P-020 / Principle VIII posture**:
   * **Remediation Applied**: Updated U1.1 to check `dts !== null && dts !== undefined && Number.isFinite(dts)`, break box walks on `boxSize < 8`, hook `MediaSource.prototype.removeSourceBuffer` to ignore `__isRecapVideoShim`, and documented Wave-Staged Red-Phase Harness execution, `Careful mode` + `Freeze-scope mode` + `Strict-safety`, and P-020 context compaction.

