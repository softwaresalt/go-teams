---
title: Compacted memory for shipment 031-S download-blocked SharePoint Recap high-speed audio capture
date: 2026-11-16
feature: 031-F
shipment: 031-S
deliberation: 020-DL
source_stash: 9A16B213
pull_request: 76
merge_commit: 300afcef5958747f708649d1daefc05e648ff484
closure_status: READY
status: compacted
---

## Decisions and Scope

* **High-speed (`14x-100x` real-time) audio-only MP3 capture on download-blocked SharePoint Meeting Recaps (`031-F`, `020-DL`)**: Implemented dual-path stream capture inside `extractRecapAudioJS` (`internal/browser/recap.go`) when `g_fileInfo.isDownloadBlocked=true` in `streamembed.aspx`:
  * **Path A (`50x-100x` real-time)**: Hooks `window.crypto.subtle.decrypt`, `fetch`, and `XMLHttpRequest` (with `WeakMap` per-instance header tracking, explicit allowlist keyed by origin, HTTPS/loopback containment, and descriptor restoration on `open()` and cleanup), strips `track=video` media segments, fetches and decrypts all `audio/mp4` (`mp4a.40.2`) `oneDrive.transcode` segments (`part=initsegment` + `part=mediasegment`, `track=audio&quality=audcopy`, `segmentTime = k * wsd` up to `ppd`, `maxRecapAudioSegments = 2400`) across 8 bounded parallel workers with 15s per-attempt timeout and 2 retries, and aborts cleanly on Path B fallback.
  * **Path B (`14x-40x` real-time)**: Hooks `MediaSource.prototype.addSourceBuffer` / `removeSourceBuffer` and `SourceBuffer.prototype.appendBuffer` with `WeakMap` track classification, an `EventTarget` no-op `video/mp4` shim, `tfdt`-ordered segment deduplication (preserving `dts === 0`), muted `16x` `playbackRate`, `range0End`-guided `currentTime` stepping, bounded `sb.remove` eviction, and fail-closed contiguous/duration completeness checks.
  * **Cross-frame topology, chunked CDP pull, and `DriveItemID` retention**: Added per-frame validated same-origin `<iframe>` traversal (`collectPlayerWindows` / `validateFrameContext`), fixed-point transitive `ParentID` OOPIF scope (`expandOwnedFrames`), canonical `DriveItemID` retention on `chromeMeeting` (`m.recapItemID`), `transferPending` chunked pull over persistent `cdpEvalSession` (`window.__goTeamsRecapAudioTransfer`) for `> 4 MiB` PCM buffers, and `xplatplugins.aspx` delayed `streamembed.aspx` mount polling.
* **Post-merge closure (`PR #76`, merge `300afcef5958747f708649d1daefc05e648ff484`)**: Archived `020-DL` first and executed `autoharness shipment cascade-close --timeout 600 --caller-wait 1200` (`postcondition_verdict: pass`). All 6 live browser `-tags=integration` tests including `TestMeetingCaptureAudio` (`3,874.30s` audio captured and MP3-encoded in `1m43.463s` with 0 skips), `go test -race ./...`, and all 4 GitHub Actions CI checks passed. Residual `LOW` advisory items `U1-R5` and `U2-R5` are tracked in stash `7883544E`, while `U1-R6` and `U1-R8` are recorded in archived closure reviews (`39450846` from R3 was remediated in `685e08b`/`ed092e5` and remains in stash for Stage disposition).

## Delivered Artifacts

* `031.001-T` (`5397550`, `99933b5`, `459d1b3`, `72ad055`, `685e08b`, `ed092e5`, `5f3ec41`): `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`.
* `031.002-T` (`6a0c1ce`, `459d1b3`, `72ad055`, `685e08b`, `ed092e5`, `ace434f`, `5f3ec41`): `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`.
* `031.003-T` (`abc78db`): `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`.
* Post-merge closure & compound learning: `docs/closure/031-S-031-F-post-merge-closure.md`, `docs/closure/evidence/031-S-031-F-close-evidence.json`, `docs/compound/build-errors/cdp-chunked-transfer-for-large-audio-pcm.md`.

## Key Learnings

* Returning >400 MB base64 JSON in a single CDP `Runtime.evaluate` response stalls Chromium's DevTools transport for >2m45s; staging `> 4 MiB` PCM chunk arrays on `window.__goTeamsRecapAudioTransfer` and pulling 64-chunk batches over one persistent `cdpEvalSession` WebSocket avoids both DevTools stalls and Windows `WSAEADDRINUSE` ephemeral port exhaustion.
* `TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd` must await its `track=video` strip probe before Path A finishes so slow Windows CI runners do not fire a delayed `setTimeout` after `cleanupHooks()` restores `window.fetch`.

## Archived Source Memories

* `docs/archive/memory/2026-11-16/stage-031-s-download-blocked-recap-audio-memory.md`
* `docs/archive/memory/2026-11-16/ship-031-s-download-blocked-recap-audio-memory.md`
