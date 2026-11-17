---
title: Compacted Session Memory — Shipment 033-S (SharePoint stream.aspx Audio, Transcript, and Copilot AI Summary Capture)
description: Consolidated Stage and Ship session memory for feature 033-F / shipment 033-S (stash D034C18B, PR #80)
date: 2026-11-17
shipment: 033-S
feature: 033-F
deliberation: 022-DL
pull_request: 80
merge_commit: dc11b50a6c315c8afabe18f90d2d2978d69ba384
archived_sources:
  - docs/archive/memory/2026-11-17/stage-033-s-sharepoint-stream-aspx-capture-memory.md
  - docs/archive/memory/2026-11-17/ship-033-s-sharepoint-stream-aspx-capture-memory.md
---

## Summary of Shipped Scope

* **Deliberation & Feature**: `022-DL` / `033-F` / `033-S` (from stash `D034C18B`) added end-to-end support for direct SharePoint/OneDrive `stream.aspx` recording URLs across `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, unit/E2E/live-integration tests, `docs/ARCHITECTURE.md`, and `docs/USER_GUIDE.md`.
* **PR & Merge**: PR #80 merged via merge commit `dc11b50a6c315c8afabe18f90d2d2978d69ba384` (P-009) after passing all GitHub Actions checks, 4 rounds of `multi-agent` Adversarial Review (`READY` at `e25599aac70cdf3b1e68284c2cb7e60a07c30d4b`), and 3 GitHub Copilot review rounds (`P-018` `SATISFIED`).

## Key Decisions and Technical Learnings

* **Audio segment seeding & timescale isolation**: `seedEncKeyFromFileInfo()` imports `g_fileInfo.EncKey` via WebCrypto `AES-CBC` without setting `activityObserved`; `scanPerformanceTranscodeEntries(w)` scans `performance.getEntriesByType('resource')` (identity/item-filtered) until `transcodeAudioTemplateUrl`, `transcodeWsd`, and `transcodePpd` are populated; `track=video` (`wsd=96000`) is blocked from setting or overwriting `transcodeWsd`, `transcodePpd`, or `transcodeAudioTemplateUrl`; `transcodeDurationSec` divides `ppd` by `16000` when `1000 <= wsd < 1e6`.
* **Transcript & Copilot AI summary**: `extractRecapTranscriptJS` clicks `Transcript` once when `.ms-List` is absent; `copyAISummaryJS` is gated on `!isTeamsRecapSurface && isSharePointStreamSurface`, excludes `.fui-ChatMyMessage__body`, marks pre-existing messages, requires `goTeamsStreamRecapTriggered === 'true'`, and polls 60 times (`30s`).
* **Recording identity & canonical URL checks**: `applyQueryIdentity` and `checkIdentityAgainst` extract `RecordingName` from both `fileUrl` (URL-parsed) and decoded server-relative `id` paths (preserving `%` and `#`); `canonicalStreamTargetMatch` compares `id`, `uniqueid`, and `driveid` case-insensitively on opaque-`id` / `driveId`-only SharePoint URLs.

## Verification & Closure

* `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `gofmt -l .`, `go test -race ./...`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, and live `-tags=integration` suites (`internal/browser` 6/6, `internal/cli` 2/2) all passed.
* `autoharness shipment cascade-close` archived `033-F`, `033-S`, `033.001-T`..`033.004-T` (`022-DL` archived prior to cascade) with `postcondition_verdict: pass`.
* Follow-up stash entries retained for Stage triage: `E03CB3C3`, `4C40AF2B` (P-021 C2 deferrals), `BA069506` (docs wording qualification), and `196AB9E4`, `69AB7ACC`, `0DE2E900` (remediated in PR #80).
