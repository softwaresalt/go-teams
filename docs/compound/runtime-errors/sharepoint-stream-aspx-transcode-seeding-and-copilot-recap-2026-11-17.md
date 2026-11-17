---
title: "SharePoint stream.aspx transcode timescale isolation, EncKey seeding, and Copilot AI summary capture"
description: "Capture patterns and invariants for direct SharePoint/OneDrive stream.aspx audio, transcript, and Copilot AI summary extraction."
source: "docs/compound/runtime-errors/sharepoint-stream-aspx-transcode-seeding-and-copilot-recap-2026-11-17.md"
doc_type: "learning"
problem_type: "runtime-error"
category: "runtime-errors"
component: "internal/browser"
root_cause: "SharePoint stream.aspx pre-buffers oneDrive.transcode segments before CaptureAudio runs, uses 16 kHz sample units (wsd=96256) for audio vs 96000 for video, defaults to the Copilot sidebar, and passes decoded server-relative paths in ?id=."
resolution_type: "code_fix"
severity: "high"
message: "recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)"
file_path: "internal/browser/recap.go"
citations:
  - "docs/plans/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-decided-plan.md"
  - "docs/closure/033-S-033-F-post-merge-closure.md"
tags:
  - sharepoint
  - stream-aspx
  - onedrive-transcode
  - webcrypto
  - copilot
---

## Problem

Direct SharePoint and OneDrive `stream.aspx` recording URLs failed capture in four ways:

1. `captureStreamEmbedAudio()` timed out after `10s` with `isDownloadBlocked=true` because `stream.aspx` pre-buffers `oneDrive.transcode` segments before `CaptureAudio` installs main-window hooks.
2. `track=video` requests (`wsd=96000`) overwrote `track=audio` `transcodeWsd` (`96256`), and `ppd` values in 16 kHz sample units (`192512` < `1e6`) tripped `maxRecapAudioDurationSec` when treated as seconds.
3. `.ms-List` stayed unmounted until the `Transcript` button was clicked, and SharePoint Copilot AI summaries rendered asynchronously (`~12.5s`) under `.fai-CopilotMessage` rather than Teams Recap `Copy all`.
4. `ExtractMeetingIdentity` ignored `?id=...mp4` and double-decoded server-relative `id` paths containing `%` or `#`.

## Root Cause

* **Dual `oneDrive.transcode` timescales**: Classic `streamembed.aspx` uses 100-nanosecond ticks (`wsd >= 1e6`, `1e7` ticks/sec), whereas SharePoint `stream.aspx` uses 16 kHz AAC sample units (`wsd=96256` = `94 * 1024` samples for audio; `wsd=96000` for video) where short recordings have `ppd < 1e6` (for example `192512` = `12.032s`).
* **Pre-existing Copilot DOM & sent prompt bubbles**: `.fui-ChatMyMessage__body` matches the user's own sent `"Recap the meeting"` bubble on subsequent polls, and earlier Copilot answers already carry a `Copy` button before the new recap finishes.
* **Already-decoded `?id=` query values**: `url.Values.Get("id")` and `URLSearchParams.getAll("id")` decode percent escapes once; passing server-relative paths through `url.Parse` or `new URL()` corrupts literal `%` and truncates `#`.

## Resolution

1. Seed `audioCryptoKeyInfo` from `g_fileInfo.EncKey` via non-extractable `crypto.subtle.importKey('raw', ..., { name: 'AES-CBC' }, false, ['decrypt'])` without setting `activityObserved` prematurely, and scan `performance.getEntriesByType('resource')` filtered by recording identity and `itemId` while `!transcodeAudioTemplateUrl || !(transcodeWsd > 0) || !(transcodePpd > 0)`.
2. Guard `transcodeWsd`, `transcodePpd`, and `transcodeAudioTemplateUrl` in `inspectTranscodeRequest` with `track !== 'video'`, and compute `transcodeDurationSec(ppdVal, wsdVal)` using `ppdVal / 16000` when `1000 <= wsdVal < 1e6`.
3. Click `Transcript` idempotently when `.ms-List` is empty, gate SharePoint Copilot `"Recap the meeting"` on `!isTeamsRecapSurface && isSharePointStreamSurface` + `document.documentElement.dataset.goTeamsStreamRecapTriggered === 'true'`, exclude `.fui-ChatMyMessage__body`, and poll 60 times (`30s`).
4. Keep `url.Parse` / `new URL` for `fileUrl` (including relative paths with `?web=1`), read server-relative `id` paths directly via `path.Base` without double decoding, and enforce case-insensitive canonical parameter checks in `canonicalStreamTargetMatch`.

## Prevention

* Always check `track !== 'video'` before mutating `transcodeWsd`, `transcodePpd`, or `transcodeAudioTemplateUrl`, and derive `ppd` timescale from `wsd` (`>= 1e6` -> `1e7`, `1000..999999` -> `16000`).
* Never run `url.Parse` or `new URL` on already query-decoded server-relative `id` paths; keep `url.Parse` / `new URL` strictly for URL-shaped parameters like `fileUrl`.
