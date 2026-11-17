---
title: Runtime Verification — Shipment 031-S (031-F)
description: Structured runtime verification evidence for CLI, browser, and live-tenant Recap audio capture surfaces
date: 2026-11-16
shipment_id: 031-S
feature_id: 031-F
verdict: PASS_WITH_FOLLOW_UP
---

## Validator Contract Mapping

* **Validator manifest**: `.autoharness/workspace-profile.yaml` (`runtime_validation.validator_manifest`)
* **Surfaces exercised**: `cli` (`command` adapter), `browser` (`chromedp` headless E2E and live-tenant `-tags=integration` probes), and `background-job` (`schedule --help` command probe)
* **Strict-safety actions**: `PA-031-1`, `PA-031-2`, `PA-031-3`, `PA-031-4` — all `applied` and verified within workspace isolation and redaction invariants

## Environment Prechecks

* Host toolchain: Go 1.27.4 (`windows/amd64`), Node.js runtime available for `extractRecapAudioJS` / `extractRecapTranscriptJS` harness probes
* Browser executable: Microsoft Edge / Chromium resolved via `ResolveExecPath("")`
* Live integration environment: `.env.local.test` present with authenticated Teams profile and view-only (`isDownloadBlocked=true`) SharePoint Meeting Recap target

## Probe Results

| Probe ID | Surface / Adapter | Command / Target | Expected Signal | Observed Result | Verdict |
|---|---|---|---|---|---|
| `cli-help` | `cli` / `command` | `go run ./cmd/go-teams --help` | Usage lists `auth`, `captions`, `join`, `schedule`, `generate-config`, `update` | Rendered full CLI usage; exit code `0` | `PASS` |
| `cli-build` | `cli` / `command` | `go build -o .\bin\go-teams.exe .\cmd\go-teams` | Exit code `0` | Binary built cleanly; exit code `0` | `PASS` |
| `scheduler-dry-run` | `background-job` / `command` | `go run ./cmd/go-teams schedule --help` | Renders schedule help without launching a browser; exit code `0` | Rendered full schedule usage; exit code `0` | `PASS` |
| `browser-e2e` | `browser` / `chromedp` | `go test -race ./internal/browser/... -run TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd` | Headless Chromium decodes AES-CBC `oneDrive.transcode` fMP4 and writes valid MP3 | Passed (`buf.Len() > 0`, valid MPEG sync header, 0 `track=video` requests) | `PASS` |
| `live-recap-audio` | `browser` / `-tags=integration` | `go test -tags=integration -v -timeout 10m ./internal/browser/...` | All 6 browser integration tests pass including MP3 capture on `isDownloadBlocked=true` | All 6 passed (`TestMeetingCaptureAudio`: `3910.4s` audio, `33,286,790` source bytes, `344,897,280` PCM bytes in `1m41.044s`, 0 skips) | `PASS` |
| `secret-hygiene` | `cli` / `command` | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | Zero leaked passcodes, tokens, or live URLs in tracked files | Passed; exit code `0` | `PASS` |

## Manual Checkpoint & Follow-Up Summary

* `teams-auth-mfa`: Satisfied for headless verification via pre-authenticated `UserDataDir` profile in `.env.local.test` (interactive MFA flow unchanged).
* `scheduled-meeting-join`: Live scheduled overtime/recap tests passed; long-running daemon observation remains covered by the post-deploy validation window.
* Residual follow-ups: 2 `LOW` Unique advisory observations from Round 5 adversarial review captured in active stash `7883544E` (prior `U1-R3`..`U6-R3` in `39450846` were remediated in `685e08b`/`ed092e5`).

## Verification Verdict

`PASS_WITH_FOLLOW_UP` — All automated CLI, E2E, secret-hygiene, and live-tenant integration probes passed; 2 `LOW` advisory observations are tracked in stash `7883544E` for future Stage triage.
