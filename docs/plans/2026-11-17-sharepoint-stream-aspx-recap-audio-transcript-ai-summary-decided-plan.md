---
title: Decided Plan — Direct SharePoint/OneDrive stream.aspx Audio, Transcript, and Copilot AI Summary Capture (033-S)
description: Consolidated execution plan for feature 033-F / shipment 033-S from deliberation 022-DL
date: 2026-11-17
status: decided
feature: 033-F
shipment: 033-S
deliberation_id: 022-DL
source_stash:
  - D034C18B
archived_source_plan: docs/archive/plans/2026-11-17/2026-11-17-sharepoint-stream-aspx-recap-audio-transcript-ai-summary-plan.md
---

## Decided Architecture & Implementation Units

* **U1 (`033.001-T` — `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`)**:
  * Recognize `stream.aspx` and `[role="menuitem"][aria-label="Play"]` as a playable stream surface; isolate `transcodeWsd`, `transcodePpd`, and `transcodeAudioTemplateUrl` from `track=video` (`wsd=96000`); seed `g_fileInfo.EncKey` via WebCrypto `AES-CBC` (`seedEncKeyFromFileInfo`) and `PerformanceResourceTiming` (`scanPerformanceTranscodeEntries`, filtered by identity and `itemId`); normalize 16 kHz sample-unit `ppd` (`transcodeDurationSec`).
  * Activate the SharePoint `Transcript` button idempotently when `.ms-List` is empty; extract SharePoint Copilot `.fai-CopilotMessage` summaries after `"Recap the meeting"` (`goTeamsStreamRecapTriggered === 'true'`, 60-attempt / `30s` polling, preserving `TestCopyAISummaryJSStructure`).
  * Extract `RecordingName` from both `fileUrl` and server-relative `id` media paths (preserving `%` and `#`) and enforce conflict-checked `hasRecapAudioIdentityMatch` / `canonicalStreamTargetMatch` on `stream.aspx` and `streamembed.aspx`.
* **U2 (`033.002-T` — `internal/browser/recap_test.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`)**:
  * Added `TestSharePointStreamAspxAudioTranscriptAISummaryAndIdentity`, headless Chromium `TestChromeSharePointStreamAspxRecapEndToEnd`, and `stream.aspx` / `streamembed.aspx` recognition in `recapIntegrationURL`.
* **U3 (`033.003-T` — `internal/cli/scheduled_integration_test.go`)**:
  * Extended `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` (with `--audio` and `.mp3` MPEG sync assertion) and `TestLiveTenantDelayedAISummaryQueueReplay` for `stream.aspx` / `streamembed.aspx`.
* **U4 (`033.004-T` — `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`)**:
  * Documented direct SharePoint/OneDrive `stream.aspx` capture across architecture and user guide.

## Protected Invariants

1. Zero external media binaries (`ffmpeg`, `yt-dlp`) or CGO (`go.mod` unchanged); chunked CDP PCM pulls preserved.
2. Same-origin `https:` validation on all `PerformanceResourceTiming` URLs and non-extractable in-memory `CryptoKey` import for `g_fileInfo.EncKey`.
3. Strict `track !== 'video'` isolation for `transcodeWsd`, `transcodePpd`, and `transcodeAudioTemplateUrl`.
4. Conflict-checked `RecordingName` basename matching and canonical URL isolation (with full normalized path/origin isolation deferred under `E03CB3C3`).
5. `TestCopyAISummaryJSStructure` invariant preserved (`aria-label*="Copy"` never introduced).
