---
title: Ship Session Memory — Shipment 033-S (SharePoint stream.aspx Audio, Transcript, and Copilot AI Summary Capture)
description: Ship execution continuity record for shipment 033-S (033-F, deliberation 022-DL, stash D034C18B)
date: 2026-11-17
agent: ship
session_id: ship-2026-11-17-033-s
shipment: 033-S
feature: 033-F
deliberation: 022-DL
pull_request: 80
merge_commit: dc11b50a6c315c8afabe18f90d2d2978d69ba384
---

## Execution Summary

* **Branch**: `feat/033-s-direct-sharepoint-onedrive-stream-aspx-audio-stream-transcript-tab-activation-and-copilot-ai-summary-capture` -> PR #80 (merged via `--merge` commit `dc11b50a6c315c8afabe18f90d2d2978d69ba384`).
* **Tasks Completed**: `033.001-T` (`012dc4c`), `033.002-T` (`57994d6`), `033.003-T` (`db8d69f`), `033.004-T` (`ac2c6dd`), plus adversarial/Copilot remediation commits (`d9bb23c`, `718c1fa`, `83e39f1`, `e25599a`).
* **Key Technical Discoveries**:
  * SharePoint `oneDrive.transcode` audio on `stream.aspx` uses 16 kHz sample timescale (`wsd=96256`, `ppd=192512` or `60063744`), requiring `transcodeDurationSec(ppdVal, wsdVal)` to divide by `16000` when `1000 <= wsdVal < 1e6` instead of treating `< 1e6` `ppd` as raw seconds.
  * `track=video` must be excluded from `transcodeWsd`, `transcodePpd`, and `transcodeAudioTemplateUrl` updates (`M1-R2` test updated to send `track=audio`).
  * `seedEncKeyFromFileInfo` must not set `activityObserved = true` so inert pages still fail fast in 10s.
  * `copyAISummaryJS` must exclude `.fui-ChatMyMessage__body` and strictly require `document.documentElement.dataset.goTeamsStreamRecapTriggered === 'true'` before reading `.fai-CopilotMessage__actions` `Copy` buttons.
  * Server-relative `id` query values (already decoded by `url.Values` / `URLSearchParams`) must not be re-parsed with `url.Parse` / `new URL`, whereas `fileUrl` (including relative `/Recordings/Sync.mp4?web=1`) must still use `url.Parse` / `new URL`.
* **Verification**:
  * `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `gofmt -l .`, `go test -race ./...`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`, GitHub Actions `CI` (`Build, vet, lint & test (Windows)`, `Detect code changes`, `Detect CLI-affecting changes`), and `autoharness gate copilot-review 80` (`SATISFIED`).
  * Live `-tags=integration` against `.env.local.test`: all 6 browser integration tests (`TestMeetingCaptureAudio` capturing 3753.98s / 331 MB PCM -> valid MP3) and both CLI live integration tests (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`) passed with 0 skips.
* **Follow-up Stash Captures**:
  * P-021 C2 deferred scope expansion: `E03CB3C3` (full normalized path+origin `MeetingIdentity`), `4C40AF2B` (CDP `Network`/Worker target instrumentation).
  * Residual advisories: `196AB9E4`, `69AB7ACC`, `0DE2E900` (remediated in `83e39f1`/`c173196`/`e25599a`), `BA069506` (docs wording qualification).
