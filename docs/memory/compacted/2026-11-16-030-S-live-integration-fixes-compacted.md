---
title: Compacted memory for shipment 030-S live integration env expansion and remote headless Recap fixes
date: 2026-11-16
feature: 030-F
shipment: 030-S
deliberation: 019-DL
source_stash: 02B5113C
pull_request: 74
merge_commit: 29d554b918a55c8b978d3f826c6672d358ef5a8a
closure_status: READY_WITH_CONDITIONS
status: compacted
---

## Decisions and Scope

* **Single-pass path expansion and legacy `teams-cli` fallback (`030-F`, `019-DL`)**: Expanded `%VAR%`, `${VAR}`, `$VAR`, and `~` once at the input boundary in `internal/browser/recap.go` (`expandProfilePath`, `resolveExistingProfileDir`, `resolveExpandedProfileDir`), `internal/browser/integration_helper_test.go`, and `internal/cli/binary_automation_test.go`, falling back from the rightmost non-repo-root `go-teams` segment to `teams-cli` when no persisted Teams session exists in `go-teams` and from `teams-cli.exe` to `go-teams.exe` for `GO_TEAMS_BINARY`.
* **Remote authenticated `--headless=new` launcher routing**: Routed remote authenticated `NewLiveMeeting` sessions with a non-empty `UserDataDir` through `cm.headedOpts` (`launchCleanHeadedBrowser` with `--headless=new`) while propagating `m.managedProfile` downstream so Windows DPAPI cookies and cross-origin Entra/Teams COOP `targetId` rebinding work identically to headed mode without re-expanding literal `$VAR` path tokens.
* **SharePoint Recap `VroomItemId` and `isDownloadBlocked=true` handling**: Added lowercase `VroomItemId` support in `extractRecapAudioJS` and `extractRecapTranscriptJS`, detected `isDownloadBlocked=true` across HTTP errors, `text/html` error pages, trailing `blob:` candidates, and multi-target loops, added `#openTeamsClientInBrowser` to `joinOnWebSel`, and added a `2s` `listCDPTargets(port)` teardown poll in `launchCleanHeadedBrowser`.
* **Post-merge closure (`PR #74`, merge `29d554b918a55c8b978d3f826c6672d358ef5a8a`)**: Recovered from the initial `--timeout 180` timeout by restoring clean `HEAD`, archiving `019-DL` first, and running `autoharness shipment cascade-close --timeout 600 --caller-wait 1200` (`postcondition_verdict: pass`). Closed with `READY_WITH_CONDITIONS` (`7 PASS`, `1 SKIP` in `TestMeetingCaptureAudio` due to SharePoint `isDownloadBlocked=true`). Deferred follow-ups `232C6844`, `AD451E00`, and `3F6BA819` remain in active stash alongside `9BDFB275`, `560B0C44`, and `188D4FBB`.

## Delivered Artifacts

* `030.001-T` (`1387ae9`, `0104696`, `3fe5457`, `b9e1672`): Production browser fixes across `internal/browser/prejoin.go`, `internal/browser/meeting.go`, and `internal/browser/recap.go`.
* `030.002-T` (`b0b67e9`, `3fe5457`, `1add058`, `b9e1672`): `internal/browser/integration_helper_test.go`, `internal/browser/meeting_integration_test.go`, and `internal/browser/scheduled_lifecycle_integration_test.go`.
* `030.003-T` (`7d2fb4b`, `3fe5457`, `b9e1672`): `internal/cli/binary_automation_test.go`, `internal/cli/scheduled_integration_test.go`, and `internal/browser/recap_test.go`.
* Post-merge closure & knowledge graduation: `docs/closure/030-S-030-F-post-merge-closure.md`, `docs/closure/evidence/030-S-030-F-close-evidence.json`, `docs/USER_GUIDE.md`, and `docs/ARCHITECTURE.md`.

## Key Learnings

* `NewLiveMeeting` must expand `Options.UserDataDir` once at the boundary and carry the resolved path (`m.managedProfile`) downstream so literal `$VAR` or `%VAR%` substrings inside pre-expanded paths are never re-expanded.
* Bare browser executable detection (`msedge`, `chrome`) must execute after `%VAR%`/`$VAR`/`~` expansion so `TEAMS_BROWSER_PATH=$BROWSER` resolves via `PATH` rather than anchoring to `repoRoot`.
* With ~195 artifacts in `.backlogit/`, `backlogit shipment ship` takes ~185s; `autoharness shipment cascade-close` needs `--timeout 600 --caller-wait 1200` (since `--caller-wait` must be at least `--timeout + 600s` supervision margin).

## Archived Source Memories

* `docs/archive/memory/2026-11-16/stage-030-s-live-integration-fixes-memory.md`
* `docs/archive/memory/2026-11-16/ship-030-s-live-integration-fixes-memory.md`
