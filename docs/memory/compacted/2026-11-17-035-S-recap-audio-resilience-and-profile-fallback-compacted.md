---
title: Compacted Memory — Shipment 035-S (Recap Audio Cold SharePoint Transcode Resilience and %APPDATA% Profile Fallback)
description: Consolidated Stage and Ship execution memory for stash 6E17E6EA, deliberation 024-DL, feature 035-F, tasks 035.001-T..035.003-T, and shipment 035-S
date: 2026-11-17
shipment_id: 035-S
feature_id: 035-F
deliberation_id: 024-DL
pull_request: 84
merge_commit: b2538a2f7385b6e443d43771c02b6121da446a79
archived_sources:
  - docs/archive/memory/2026-11-17/stage-035-s-recap-audio-resilience-and-profile-fallback-memory.md
  - docs/archive/memory/2026-11-17/ship-035-s-recap-audio-resilience-and-profile-fallback-memory.md
---

## Key Decisions and Outcomes

* **Stage (`024-DL`, `035-F`, `035-S`)**: Triaged `6E17E6EA` (clean P-021 C5/C6 duplicate scan), passed 6-persona Plan Review (`Architecture Strategist` anchor `gpt-6.1-sol`), harvested into `035-F` + `035.001-T`..`035.003-T`, and sequenced `034-S -> 035-S -> 010-S`.
* **Implementation (`035.001-T`..`035.003-T`)**:
  * In `extractRecapAudioJS` (`internal/browser/recap.go`): marked HTTP `408` retryable alongside `429`/`5xx`, scaled `fetchBytesWithCap` to 5 attempts (`250/500/1000/2000ms`) and `deadlineAt` to `240000ms` (`4m`) when `maxRecapAudioDurationSec > 300` (with optional `recapAudioFetchMaxAttempts` / `recapAudioCaptureDeadlineMs` overrides), increased `runPathA` concurrency to `12` workers, split `btnClicked`/`mediaPlayTriggered` in `triggerPlaybackIfReady()`, and threw `'recap audio Path A capture deadline exceeded'` before Path B MSE fallback when `pathAPromise && !pathAFailed && !pathAResult` (`0dd4998`).
  * In `captureRecapAudioFromTargets` (`internal/browser/recap.go`): increased `recapAudioEvalTimeout` to `10m` as an operation-wide `captureCtx` deadline, added a single attempt-0 retry on Path A / MSE deadline errors while breaking `orderedTargets` on stream-deadline errors, and preserved `ctx.Err()` / `errors.Join(lastErr, captureCtx.Err())` (`0dd4998`, `770fda6`, `96c2fe0`, `619f01e`, `c123099`).
  * In `resolveExpandedProfileDir` (`internal/browser/recap.go`): added canonical cross-root `<APPDATA|UserConfigDir|LOCALAPPDATA>/<go-teams|teams-cli>/profile` fallback guarded by `!isPathInsideRepoRoot` with a `os.UserHomeDir()` `.git` dotfiles carve-out (`0dd4998`, `770fda6`, `96c2fe0`, `619f01e`, `24ffd83`).
  * Added unit tests in `internal/browser/recap_test.go` (`f2d880a`, `770fda6`, `96c2fe0`, `619f01e`, `24ffd83`, `c123099`) and updated `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` (`4fccb86`, `96c2fe0`, `24ffd83`).
* **Verification and Merge**: Passed all local quality gates (`go build`, `go vet`, `go vet -tags=integration`, `GOOS=darwin GOARCH=amd64 go vet`, `gofmt -l .`, `go test -race ./...`), CI checks on PR #84, 6 rounds of multi-agent Adversarial Review (`READY_WITH_FOLLOWUPS` at `c123099` with `0` P0, `0` HIGH, `0` MEDIUM), and `autoharness gate copilot-review` (`SATISFIED`). Merged PR #84 at `b2538a2f7385b6e443d43771c02b6121da446a79` (P-009) and closed via `cascade-close` (`024-DL` archived; `B98262C8` captured under P-021 C2; advisories `920CF700`, `CDCE76CE` stashed).
