---
title: Compacted memory for shipment 029-S live-tenant scheduled call and Recap validation
date: 2026-11-16
feature: 029-F
shipment: 029-S
deliberation: 018-DL
source_stash: 3EB89E2D
pull_request: 72
merge_commit: eb40cc71247d4aecd6cd41205e240399fede583e
closure_status: READY_WITH_CONDITIONS
status: compacted
---

## Decisions and Scope

* **Dual-tier verification (`029-F`, `018-DL`)**: Implemented Tier 2 headless real-Chromium E2E tests (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` in `internal/browser/meeting_e2e_test.go` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` in `internal/cli/e2e_test.go`) alongside Tier 5 `-tags=integration` live-tenant test suites (`internal/browser/scheduled_lifecycle_integration_test.go` and `internal/cli/scheduled_integration_test.go`) powered by `028-S`'s `.env.local.test` loaders (`browser.LoadTestEnv` and `loadCLITestEnv`).
* **Zero overlap with `010-S`**: Placed new browser integration tests in `internal/browser/scheduled_lifecycle_integration_test.go` so `internal/browser/meeting_integration_test.go` remained untouched (`028-S -> 029-S -> 010-S`).
* **Zero production `.go` changes**: Confined all changes to `*_test.go`, `docs/`, and `.backlogit/`.
* **Post-merge closure (`PR #72`, merge `eb40cc71247d4aecd6cd41205e240399fede583e`)**: Closed via `cascade` (`docs/closure/029-S-029-F-post-merge-closure.md`) with status `READY_WITH_CONDITIONS` (Tier 2 E2E and Tier 5 compile gates `PASS`; Tier 5 live-tenant human checkpoints `BLOCKED` while AFK in `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`). Deferred stash `560B0C44` captured under P-021 C2 for PR #72 Copilot thread `PRRT_kwDOTStvas63xJru`; pre-existing stash `9BDFB275` left untouched.

## Delivered Artifacts

* `029.001-T` (`9b63726`, hardened in `85b83c7`): Browser E2E (two-phase interim/final caption activity, `OnCallEnded`, `ForceRecap` + `AISummaryOnly`) and live-tenant browser integration tests (`TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`).
* `029.002-T` (`5d7f58c`, hardened in `85b83c7` and `4ee45d4`): CLI E2E (Phase 1 overtime + delayed Recap `# AI Summary` enrichment + `::` queue completion; Phase 2 early call-end shutdown under `--duration 60s --idle-timeout 800ms`) and live-tenant CLI integration tests (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`).
* `029.003-T` (`156f489`): Updated `TEAMS_RECAP_URL` table row and live-tenant validation runbook in `docs/USER_GUIDE.md`; updated `docs/QUALITY_SCORE.md`.
* `029.004-T` (`998209d`, updated in `85b83c7`): Added `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` separating Tier 2 (`PASS`) and Tier 5 (`BLOCKED` while AFK) verdicts, and cross-referenced `029-S` in `docs/closure/2026-11-13-022-S-review-and-closure.md`.

## Key Learnings

* Hermetic loopback `httptest.NewServer` E2E tests that call `LoadTestEnv(t)` or `loadCLITestEnv(t)` must explicitly clear ambient `TEAMS_*` and `GO_TEAMS_BINARY` variables via `t.Setenv` so local developer `.env.local.test` values never divert synthetic runs.
* Gating simulated SPA caption mutations on `typeof window.sendCaptionToGo === 'function'` removes startup timing races between `activateCaptions` CDP roundtrips and `captionObserverJS` attachment.
* `AppendAISummaryToFile` formats `Summary notes:\n...` headings into bulleted Markdown (`* **Summary notes:** ...`).
* Archiving `018-DL` before running `autoharness shipment cascade-close` resolves `linked_deliberation_disposition` to `already-archived` cleanly in `029-S-029-F-close-evidence.json`.

## Archived Source Memories

* `docs/archive/memory/2026-11-15/stage-029-s-live-scheduled-validation-memory.md`
* `docs/archive/memory/2026-11-16/ship-029-s-live-scheduled-validation-memory.md`
