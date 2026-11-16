---
title: Live-tenant validation of scheduled meeting join, overtime continuation, call-end silence shutdown, and Idea 2C Recap enrichment
description: Deliberation on stash entry 3EB89E2D for covering feature 029-F and shipment 029-S
topic: Live-Tenant Scheduled Call Capture and Idea 2C Delayed AI Summary Validation (Option C)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 018-DL
feature: 029-F
shipment: 029-S
source_stash:
  - 3EB89E2D
linked_artifacts:
  - docs/plans/2026-11-15-live-tenant-scheduled-call-and-recap-validation-plan.md
  - docs/closure/2026-11-13-022-S-review-and-closure.md
date: 2026-11-15
status: accepted
tags:
  - testing
  - browser
  - cli
  - capture
  - schedule
  - ai-summary
  - docs
---

## Problem Frame

Shipment `022-S` (`022-F`, `R1`–`R15`, PR #55, merge commit `7b6c31d98ab00f1aaea759e87f5c073936a53dbc`, closed in [2026-11-13-022-S-review-and-closure.md](../closure/2026-11-13-022-S-review-and-closure.md)) implemented protocol-driven and scheduled live call capture across `internal/config`, `internal/scheduler`, `internal/browser`, `internal/capture`, and `internal/cli`:

1. **Bounded and activity-aware live capture (`R1`–`R3`, `R9`, `R11`, `R12`)**:
   * `--duration` / `duration` defines the nominal scheduled capture window (`defaultScheduleDuration = 1h` in `go-teams schedule`).
   * While a live call is active past `duration`, `capture.Session.Run` (`internal/capture/session.go`) extends the session into **overtime** as long as `RecordCaptionActivity()` (driven by `captionObserverJS` finalized utterances and throttled 1000 ms `interim: true` DOM mutation heartbeats in `internal/browser/captions.go`) or `RecordAudioActivity()` (driven by `HasVoiceActivity(samples, threshold)` with `DefaultVoiceRMSThreshold = 300.0` in `internal/browser/audio.go`) continues within `IdleTimeout` (`--idle-timeout` / `idle_timeout`, default `3m`).
   * In-browser call-end polling (`pollCallEnded` every `2s` in `internal/browser/meeting_chrome.go` using `callEndedProbeJS()` and `shouldNotifyCallEnded` in `internal/browser/prejoin.go`) detects disappearance of `#hangup-button` and appearance of `[data-tid="call-ended-screen"]`, `[data-tid="post-call-screen"]`, `[data-tid="rejoin-call-button"]`, `[data-tid="call-quality-rating"]`, or `"You left the meeting"` / `"Call ended"`, invoking `RecordCallEnded()` so capture stops after `IdleTimeout` silence even if nominal `duration` has not elapsed.
2. **Protocol-driven capture modes and Idea 2C delayed AI Summary enrichment (`R13`–`R15`)**:
   * `--mode live-with-summary` flushes the live `.md` transcript and `.mp3` recording immediately when the live call ends, closes Chromium to release the shared `UserDataDir` profile (`busy` atomic gate in `internal/cli/schedule.go`), writes a crash-safe `<url> --append-ai-summary <transcript.md> --not-before <RFC3339>` line into `transcription_queue.txt` under inter-process lock `withQueueFileLock` (`internal/cli/queue.go`), and runs an in-process timer (`--ai-summary-delay`, default `30m`) or deferred `--queue` replay (`runAppendAISummary` in `internal/cli/capture.go`) that opens the Meeting Recap with `ForceRecap: true, AISummaryOnly: true`, extracts the AI Summary via `copyAISummaryJS` (`internal/browser/recap.go`), appends `# AI Summary` idempotently via `capture.AppendAISummaryToFile`, and prefixes the completed queue line with `::`.

At `022-S` operational closure, `closure_status` was recorded as `READY_WITH_CONDITIONS`, and stash entry **`3EB89E2D`** (`task`, `low`) was created to track live-tenant and end-to-end validation of scheduled meeting join, overtime continuation, in-browser call-end silence shutdown, and Idea 2C delayed AI Summary Recap enrichment.

With Shipment **`028-S`** (`028-F`, `017-DL`, merge commit `40e20ef1077f3cfd5b083f1d07879d307ae4aa84`) now shipped on `main` — providing `.env.example` and the fail-closed `.env.local.test` loaders (`browser.LoadTestEnv(t)` in `internal/browser/integration_helper_test.go` and `loadCLITestEnv(t)` in `internal/cli/binary_automation_test.go`) — this Stage session deliberates and plans **`3EB89E2D`** into covering feature **`029-F`** and shipment **`029-S`**, sequenced immediately after `028-S` in the shipment dependency chain `028-S -> 029-S -> 010-S`.

### Scope Boundaries

* **In scope**:
  * Real-Chromium end-to-end tests in `internal/browser/meeting_e2e_test.go` (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`) and `internal/cli/e2e_test.go` (`TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) that run in standard `go test -race ./...` (and CI `test-windows`) against a multi-phase `httptest.NewServer` Teams SPA to verify scheduled join, overtime continuation past `--duration` via interim/final caption mutations, in-browser call-end teardown + `--idle-timeout` silence shutdown, and Idea 2C delayed AI Summary Recap enrichment (`transcription_queue.txt` persistence, `--append-ai-summary` Recap extraction, `# AI Summary` append, and `::` queue completion).
  * Dedicated `-tags=integration` live-tenant test suites in `internal/browser/scheduled_lifecycle_integration_test.go` (`TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`) and `internal/cli/scheduled_integration_test.go` (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`) powered by `028-S`'s `.env.local.test` loaders (`browser.LoadTestEnv(t)` and `loadCLITestEnv(t)`).
  * Developer and operator runbook documentation in `docs/USER_GUIDE.md` and `docs/QUALITY_SCORE.md`, plus a structured browser-verification runbook and `022-S` condition cross-reference in `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` and `docs/closure/2026-11-13-022-S-review-and-closure.md`.
  * Wiring the shipment DAG edges `029-S -> 028-S` (`blocks`) and `010-S -> 029-S` (`blocks`) so `029-S` enters `ready_set` immediately and `010-S` is its sequenced downstream dependent.
* **Out of scope**:
  * Production `.go` source files under `cmd/` and `internal/` (unless a test-only seam is required; as verified below, all needed hooks and options are already exported in `internal/browser`, `internal/capture`, and `internal/cli`).
  * Existing queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and its target file `internal/browser/meeting_integration_test.go`: `029-S` places its `-tags=integration` browser tests in a separate file `internal/browser/scheduled_lifecycle_integration_test.go` to guarantee zero file-level overlap or merge conflicts with `010-S`.
  * Stash entry **`9BDFB275`** (`DEFERRED SCOPE EXPANSION` captured during `028-S` post-merge closure PR #71 regarding external `autoharness shipment cascade-close` verifier set construction): left completely untouched in `active` stash state per P-017 and P-021 ("When a deferred-expansion entry is captured during a dark run, the capture does NOT interrupt the run: the entry is captured (P-021 C2) and left for the next Stage triage cycle; it is never triaged or planned inside the dark run itself.").

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`**: Live-tenant meeting URLs (`TEAMS_MEETING_URL`), Meeting Recap URLs (`TEAMS_RECAP_URL`), passcodes (`?p=`), and account identifiers (`TEAMS_ACCOUNT`) must never be echoed in test failure messages, logs, or committed closure/runbook artifacts. Any URL validation or assertion failure in `scheduled_lifecycle_integration_test.go` or `scheduled_integration_test.go` must redact URLs via `teams.RedactURL` and accounts via `browser.RedactAccountIdentifier` (or omit raw values entirely).
* **`docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`**: `go test` executes each package with that package's directory (`internal/browser`, `internal/cli`) as `os.Getwd()`. Both `browser.LoadTestEnv(t)` and `loadCLITestEnv(t)` (shipped in `028-S`) already walk upward from `os.Getwd()` to `go.mod` and anchor relative paths (`TEAMS_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `GO_TEAMS_BINARY`) to the repository root.

### Codebase Coupling and Technical Analysis

1. **Zero-Conflict File Separation from `010-S`**:
   * Shipment `010-S` modifies `internal/browser/meeting_integration_test.go` (`TestMeetingJoin`, `TestMeetingEnableCaptions`, `TestMeetingCaptureAudio`).
   * Placing `029-S`'s browser-level `-tags=integration` tests in a new co-located file `internal/browser/scheduled_lifecycle_integration_test.go` (`//go:build integration`, `package browser_test`) reuses `requireIntegration(t)`, `meetingURL(t)`, and `browserContext(t)` from `launch_integration_test.go` and `meeting_integration_test.go` without touching any file in `010-S`'s manifest.
2. **Loopback `httptest.NewServer` URL Behavior with `deriveRecapURLFromMeetingURL` (`internal/browser/meeting_chrome.go`)**:
   * In `internal/browser/meeting_chrome.go`, `deriveRecapURLFromMeetingURL(meetingURL)` checks `id := ExtractMeetingIdentity(meetingURL)`. When `id.ThreadID != ""` (for example, a URL containing `threadId=19%3ameeting_...` or `/l/meetup-join/19%3ameeting_...`), `deriveRecapURLFromMeetingURL` rewrites the host to `https://teams.cloud.microsoft/l/meetingrecap?threadId=...` (or `https://teams.microsoft.com/l/meetingrecap?threadId=...`).
   * By contrast, when `meetingURL` uses the `/meet/<id>?p=<passcode>` path without a `threadId` parameter (such as `srv.URL + "/meet/sched123?p=synthetic-placeholder-passcode"` on a loopback `httptest.NewServer`), `id.ThreadID` is empty (`id.MeetID == "sched123"`), `deriveRecapURLFromMeetingURL` returns `""`, and `Join` with `ForceRecap: true, AISummaryOnly: true` navigates directly to `srv.URL + "/meet/sched123?p=synthetic-placeholder-passcode"`, where `isOnTargetMeetingOrRecapURL` matches `MeetID == "sched123"` and `captureAISummaryFromTargets` runs `copyAISummaryJS` against the local test server!
3. **Live-Tenant URL Shapes for Idea 2C Delayed AI Summary (`TEAMS_MEETING_URL` vs `TEAMS_RECAP_URL`)**:
   * On a live Microsoft Teams tenant, a classic `/l/meetup-join/19%3ameeting_<id>@thread.v2/0?...` join URL contains `ThreadID`, enabling `deriveRecapURLFromMeetingURL` to derive `https://teams.microsoft.com/l/meetingrecap?threadId=...` automatically during the in-process `--ai-summary-delay` timer or `--append-ai-summary` queue replay.
   * A modern `/meet/<numeric-id>?p=<passcode>` join link does not embed `ThreadID` in the initial URL string; against a live tenant, opening `/meet/<numeric-id>` in `ForceRecap` mode lands on the launcher/pre-join screen rather than `/l/meetingrecap`. Therefore, for `/meet/<numeric-id>` meetings, post-call Recap AI summary validation uses `TEAMS_RECAP_URL` (alias `TEAMS_TEST_RECAP_URL` in `.env.local.test`) or a `/l/meetup-join/` URL carrying `threadId`. Both `TestMeetingLiveRecapAISummaryEnrichment` (`internal/browser`) and `TestLiveTenantDelayedAISummaryQueueReplay` (`internal/cli`) prefer `TEAMS_RECAP_URL` when set and fall back to `TEAMS_MEETING_URL` when `TEAMS_MEETING_URL` contains a `meetingrecap` path or `threadId` (`19:meeting_` / `19%3ameeting_`), skipping cleanly with an actionable message if neither is provided.
4. **Fast, Deterministic Real-Chromium E2E Timing (`internal/browser/meeting_e2e_test.go`, `internal/cli/e2e_test.go`)**:
   * In `internal/browser/meeting_chrome.go`, `pollCallEnded` uses `callEndedPollInterval = 2 * time.Second` (unless `m.callEndPoll` is set on `chromeMeeting`), and `capture.Session.Run` uses `DefaultPollInterval = 250 * time.Millisecond`.
   * When `copyAISummaryJS` evaluates on a Recap page where `<button aria-label="Copy all">` is already rendered in the initial HTML and its `click` listener writes to `navigator.clipboard.writeText` (and dispatches/populates `copy` event `clipboardData`), `captureAISummaryFromTargets` succeeds on `attempt == 0` in ~500 ms without waiting for retry sleeps.
   * Consequently, a full two-session Chromium CLI E2E test (`join --mode live-with-summary --duration 1000ms --idle-timeout 1500ms --ai-summary-delay 250ms`, with caption activity at `150ms`, `750ms`, and `1350ms` so every inter-event interval is `< 1500ms`, followed by `[data-tid="call-ended-screen"]` teardown at `1550ms` and `pollCallEnded` detection at `2000ms`) completes end-to-end in ~6–8 seconds on Windows and exercises every real production code path from CLI flag parsing through Chromium DOM `MutationObserver`, overtime continuation, `callEndedProbeJS` teardown, `transcription_queue.txt` locking, second-browser Recap `copyAISummaryJS`, `AppendAISummaryToFile`, and `::` queue completion!

## Options Evaluated

### Option A: Dual-Tier Real-Chromium E2E + Dedicated `-tags=integration` Live-Tenant Suites + Reproducible Runbook (Recommended)

* **Description**:
  * **U1 (`029.001-T` — `internal/browser` tests)**: Add `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` in `internal/browser/meeting_e2e_test.go` (1 sequential two-phase test scenario exercising real Chromium interim/final caption activity callbacks, `OnCallEnded` DOM teardown detection, and `ForceRecap` + `AISummaryOnly` extraction in standard `go test -race ./...`), and create `internal/browser/scheduled_lifecycle_integration_test.go` (`//go:build integration`, `package browser_test`, 3 functions: `recapIntegrationURL`, `TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`) using `browser.LoadTestEnv(t)` (`028-S`).
  * **U2 (`029.002-T` — `internal/cli` tests)**: Add `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` in `internal/cli/e2e_test.go` (exercising the full `go-teams join --mode live-with-summary --duration 1000ms --idle-timeout 1500ms --ai-summary-delay 250ms` pipeline across two real headless Chromium sessions in standard `go test -race ./...`), and create `internal/cli/scheduled_integration_test.go` (`//go:build integration`, `package cli`, 3 functions: `requireCLIIntegration`, `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`) using `loadCLITestEnv(t)` (`028-S`).
  * **U3 (`029.003-T` — `docs/USER_GUIDE.md` & `docs/QUALITY_SCORE.md`)**: Update the `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` table row in `docs/USER_GUIDE.md`, document the live-tenant scheduled call and Idea 2C Recap validation workflow, human checkpoints, and URL shape behavior (`/l/meetup-join/` vs `/meet/` + `TEAMS_RECAP_URL`), and update `docs/QUALITY_SCORE.md` (`## Grades by package`, `### Tier 2`, `### Tier 5`, `## Live-verification gaps`, and `## Improvement roadmap`).
  * **U4 (`029.004-T` — `docs/closure/` runbook & `022-S` closure cross-reference)**: Create `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` and annotate `docs/closure/2026-11-13-022-S-review-and-closure.md` (`3EB89E2D`).
* **Pros**:
  * Provides both automated CI-enforced real-Chromium E2E verification (`test-windows` in GitHub Actions) AND operator-invocable `-tags=integration` live-tenant validation suites powered by `028-S`'s `.env.local.test`.
  * Zero file overlap with `010-S` (`internal/browser/meeting_integration_test.go` is untouched).
  * 100% test-and-docs scope; zero production binary changes required.
* **Cons**:
  * Adds two small `//go:build integration` test files (`scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go`).
* **Effort estimate**: Low–Medium (4 atomic tasks, ~0.5–1 hour each).
* **Fit**: Best fit for `3EB89E2D`, `022-S` closure conditions, `028-S` `.env.local.test` integration, and `010-S` sequencing.

### Option B: Append All Scheduled Lifecycle Tests into `internal/browser/meeting_integration_test.go` Only

* **Description**: Add live-tenant scheduled tests directly to `internal/browser/meeting_integration_test.go` without CLI `-tags=integration` tests or real-Chromium E2E tests in `meeting_e2e_test.go` / `e2e_test.go`.
* **Pros**: Fewer files.
* **Cons**: Directly conflicts with `010-S`'s planned modifications to `internal/browser/meeting_integration_test.go`, omits CLI-level Idea 2C `transcription_queue.txt` and `join --mode live-with-summary` validation, and provides no non-skipped real-Chromium E2E test in standard `go test -race ./...`.
* **Effort estimate**: Low.
* **Fit**: Poor — creates avoidable merge conflicts with `010-S` and leaves CLI Idea 2C live validation incomplete.

### Option C: Documentation-Only Runbook Without Executable Test Harnesses

* **Description**: Document manual CLI commands in `docs/USER_GUIDE.md` and `docs/closure/` without adding `*_test.go` E2E or `-tags=integration` tests.
* **Pros**: Minimal diff.
* **Cons**: Violates Constitution Principle II (Test-First Development) and fails to leverage `028-S`'s `LoadTestEnv(t)` / `loadCLITestEnv(t)` test-suite loaders.
* **Effort estimate**: Low.
* **Fit**: Unacceptable.

## Trade-off Comparison

| Criterion | Option A: Dual-Tier E2E + Dedicated `-tags=integration` Suites + Runbook | Option B: Edit `meeting_integration_test.go` Only | Option C: Docs-Only Checklist |
|---|---|---|---|
| Conflict isolation from `010-S` | Complete (zero shared files) | Poor (collides on `meeting_integration_test.go`) | Complete |
| Leverages `028-S` `.env.local.test` loaders | Yes (`LoadTestEnv` & `loadCLITestEnv`) | Partial (`browser` only) | No |
| Real-Chromium E2E coverage in `go test -race ./...` | Yes (`meeting_e2e_test.go` & `e2e_test.go`) | No | No |
| Covers all 4 pillars of `3EB89E2D` | Yes (join, overtime, call-end silence, Idea 2C Recap) | Partial | Manual only |

## Decision

Adopt **Option A** under covering feature **`029-F`** and shipment **`029-S`**, sequenced after `010-S` in the shipment dependency DAG **`028-S -> 010-S -> 029-S`**, decomposed into 4 width-isolated tasks:

1. **`029.001-T` (U1 — `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`)**: Add `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` in `internal/browser/meeting_e2e_test.go` and create `internal/browser/scheduled_lifecycle_integration_test.go` (`//go:build integration`, `package browser_test`) with `TestMeetingLiveScheduledOvertimeAndCallEnd` and `TestMeetingLiveRecapAISummaryEnrichment`.
2. **`029.002-T` (U2 — `internal/cli/e2e_test.go`, `internal/cli/scheduled_integration_test.go`)**: Add `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` in `internal/cli/e2e_test.go` and create `internal/cli/scheduled_integration_test.go` (`//go:build integration`, `package cli`) with `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` and `TestLiveTenantDelayedAISummaryQueueReplay`.
3. **`029.003-T` (U3 — `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`)**: Update the `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` table row in `docs/USER_GUIDE.md` and document the `029-S` live-tenant scheduled call and Idea 2C Recap validation runbook, human checkpoints, and URL shape rules in `docs/USER_GUIDE.md` and `docs/QUALITY_SCORE.md`.
4. **`029.004-T` (U4 — `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`, `docs/closure/2026-11-13-022-S-review-and-closure.md`)**: Record the structured browser-verification runbook and validator evidence matrix in `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` and cross-reference `029-S` in `docs/closure/2026-11-13-022-S-review-and-closure.md`.

## Rejected Alternatives

* **Option B (Editing `internal/browser/meeting_integration_test.go` only)**: Rejected because it collides with `010-S` on `meeting_integration_test.go`, omits CLI-level Idea 2C queue/timer integration tests, and provides no real-Chromium E2E coverage in standard `go test -race ./...`.
* **Option C (Docs-only manual runbook)**: Rejected because it provides no automated test harness consuming `028-S`'s `.env.local.test` loaders.

## Unresolved Questions

None. All interfaces, callbacks, CLI flags, and `.env.local.test` helpers are already shipped on `main` (`022-S` and `028-S`).

## Risks and Mitigations

* **Risk 1 — Secret or passcode leakage in live-tenant test logs, `transcription_queue.txt` assertions, or closure docs**:
  * *Mitigation*: Integration test assertions in `scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go` never format raw `TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_ACCOUNT`, or `TEAMS_PROFILE_DIR` values into error strings and never pass raw `transcription_queue.txt` contents to value-dumping `testify` helpers (`require.Contains`, `assert.Contains`, `assert.Equal`), using boolean predicate checks (`require.True(t, strings.Contains(...), "static message")`) or `sanitizeQueueArgsForLog` instead. `TEAMS_PROFILE_DIR` is verified to resolve within the repository root and to be ignored by Git (`git check-ignore -q`) before browser launch. All tracked docs and closure files replace path/query identifiers with `<redacted-id>` / synthetic placeholders and pass `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Risk 2 — Live `/meet/<numeric-id>` URLs lacking `threadId` for automatic `deriveRecapURLFromMeetingURL` derivation**:
  * *Mitigation*: `TestMeetingLiveRecapAISummaryEnrichment` and `TestLiveTenantDelayedAISummaryQueueReplay` check `TEAMS_RECAP_URL` first and only fall back to `TEAMS_MEETING_URL` when `TEAMS_MEETING_URL` is itself a `meetingrecap` URL or carries a `threadId` (`19:meeting_` / `19%3ameeting_`); otherwise they `t.Skip` with a clear message instructing the operator to set `TEAMS_RECAP_URL` in `.env.local.test`.
* **Risk 3 — Flaky timing or ambient host-config leakage in real-Chromium E2E and CLI integration tests**:
  * *Mitigation*: In `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`, use `--duration 1000ms --idle-timeout 1500ms --ai-summary-delay 250ms` with caption mutations at `150ms`, `750ms`, and `1350ms` (keeping every inter-event gap `< 1500ms`), filter non-meeting paths (`/favicon.ico`) before incrementing the visit counter, render `<button aria-label="Copy all">` immediately on the simulated Recap page so `copyAISummaryJS` succeeds on `attempt == 0`, pass a hermetic temporary `config.yaml` (using `filepath.ToSlash(queuePath)`) in both E2E and live-tenant queue replay tests, and clean up temporary Chromium `UserDataDir` directories with bounded retry loops on Windows.
