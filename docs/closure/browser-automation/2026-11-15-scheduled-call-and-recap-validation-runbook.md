---
title: Live-tenant scheduled call and Idea 2C Recap validation runbook
description: Browser-verification runbook, human checkpoint protocol, and validator evidence matrix for 029-S
date: 2026-11-16
feature: 029-F
shipment: 029-S
deliberation_id: 018-DL
source_stash: 3EB89E2D
status: ready-with-conditions
---

## Scope and target route classes

This runbook defines the browser-verification procedure and observed validator
evidence for shipment `029-S` (`029-F`, deliberation `018-DL`, source stash
`3EB89E2D`, sequenced `028-S -> 029-S -> 010-S`).

Target route classes (all identifiers and query tokens redacted):

* `https://teams.microsoft.com/meet/<redacted-id>` (modern join link; requires
  `TEAMS_RECAP_URL` for Recap AI Summary extraction)
* `https://teams.microsoft.com/l/meetup-join/<redacted-id>` (classic link with
  embedded `threadId`; supports automatic `deriveRecapURLFromMeetingURL`)
* `https://teams.microsoft.com/l/meetingrecap?threadId=<redacted-id>` (Meeting
  Recap route for `ForceRecap` + `AISummaryOnly` extraction)
* `http://127.0.0.1:<port>/meet/sched-e2e?p=synthetic-placeholder-passcode`,
  `http://127.0.0.1:<port>/meet/recap-e2e?p=synthetic-placeholder-passcode`, and
  `http://127.0.0.1:<port>/meet/sched123?p=synthetic-placeholder-passcode`
  (loopback `httptest.NewServer` synthetic Teams SPAs for Tier 2 E2E)

## Headed vs. headless mode selection

* **Tier 2 synthetic E2E (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`
  and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`)**: uses
  `--headless=new` via `cleanHeadlessAllocatorOptions` for deterministic,
  repeatable automation on Windows and CI `test-windows`.
* **Tier 5 live-tenant `-tags=integration` suites**: uses `--headless=new` once
  `TEAMS_PROFILE_DIR` holds an authenticated session; operators bootstrap a cold
  profile interactively in `--headed` mode (`go-teams auth`) before running the
  headless scheduled validation suite.

## Human checkpoint protocol

Live-tenant validation cannot run unattended in an environment without an
active Microsoft Teams meeting and organizer control. Operators must complete
three explicit human checkpoints when executing Tier 5 tests:

1. **Organizer meeting start and lobby admission**: launch the scheduled test
   meeting as organizer and admit `Transcription Agent` from the lobby within
   `2m`.
2. **Overtime speech stimulus and call-end transition**: sustain live audio or
   caption mutations past the `15s` nominal duration into the `15s` overtime
   window, then end the meeting from the organizer client (or allow `--idle-timeout`
   silence to elapse).
3. **Post-call Meeting Recap AI Summary availability**: allow Teams `5-30m`
   after call end to publish the Meeting Recap AI Summary, or provide an existing
   published `TEAMS_RECAP_URL` in gitignored `.env.local.test`.

## Validator evidence matrix

Tier 2 real-Chromium synthetic E2E probes (`go test -race ./...`) and Tier 5
live-tenant `-tags=integration` probes are tracked separately per
`browser-verification.instructions.md`.

| probe_id | route | environment | adapter | mode | server_reachability | expected_confirmation | observed_result | blocks_release | verdict |
|---|---|---|---|---|---|---|---|---|---|
| `browser-scheduled-e2e` (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`) | `http://127.0.0.1:<port>/meet/sched-e2e?p=synthetic-placeholder-passcode` and `/meet/recap-e2e?p=synthetic-placeholder-passcode` | Windows 11 local host (`go test -race ./internal/browser/...`) | `chromedp` (`NewLiveMeeting`) | `headless` | Loopback `httptest.NewServer` reachable (`200 OK`) | `OnCaptionActivity >= 2` (interim + final), `OnCallEnded` DOM teardown, `AISummaryProvider.AISummary()` non-empty | Passed in `8.21s`: interim and final caption mutations observed, `call-ended-screen` triggered `OnCallEnded`, `AISummary()` matched expected notes | `true` | `PASS` |
| `cli-scheduled-recap-e2e` (`TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) | `http://127.0.0.1:<port>/meet/sched123?p=synthetic-placeholder-passcode` | Windows 11 local host (`go test -race ./internal/cli/...`) | `chromedp` via `NewRootCommand` (`join --mode live-with-summary`) | `headless` | Loopback `httptest.NewServer` reachable (`200 OK`) | Self-termination (`ExecuteContext(ctx) == nil`, `ctx.Err() == nil`), overtime caption persisted, `# AI Summary` appended, `::` queue line marked done | Passed in `10.63s`: nominal + overtime captions written, call-end watchdog stopped capture, delayed Recap enriched `.md`, queue entry prefixed with `::` | `true` | `PASS` |
| `live-integration-compile` (`go vet -tags=integration ./...` and `go test -tags=integration -run '^$' ./...`) | N/A (compile gate) | Windows 11 local host and Ubuntu CI | Go compiler + `go vet` | `headless` (compile-only) | N/A | Zero vet or compile errors across `scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go` | Both packages compiled and passed `go vet -tags=integration ./...` with zero diagnostics | `true` | `PASS` |
| `live-browser-scheduled-overtime` (`TestMeetingLiveScheduledOvertimeAndCallEnd`) | `https://teams.microsoft.com/meet/<redacted-id>` | Live Microsoft Teams tenant (`.env.local.test`) | `chromedp` (`NewLiveMeeting`) | `headless` | Blocked in AFK dark run: no active live Teams meeting or organizer admission stimulus | Callback activity recorded past `15s` nominal duration and `OnCallEnded` observed | `BLOCKED`: operator human checkpoints (organizer lobby admission and live overtime speech/silence stimulus) unavailable while AFK | `false` | `BLOCKED` |
| `live-browser-recap-ai-summary` (`TestMeetingLiveRecapAISummaryEnrichment`) | `https://teams.microsoft.com/l/meetingrecap?threadId=<redacted-id>` | Live Microsoft Teams tenant (`.env.local.test`) | `chromedp` (`ForceRecap: true, AISummaryOnly: true`) | `headless` | Blocked in AFK dark run: requires live published Meeting Recap with AI Summary | Non-empty `AISummaryProvider.AISummary()` from live Recap | `BLOCKED`: operator human checkpoint (published post-call Meeting Recap with AI Summary in `.env.local.test`) unavailable while AFK | `false` | `BLOCKED` |
| `live-cli-scheduled-shutdown` (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`) | `https://teams.microsoft.com/meet/<redacted-id>` | Live Microsoft Teams tenant (`.env.local.test`) | `chromedp` via `NewRootCommand` (`join --mode live-with-summary`) | `headless` | Blocked in AFK dark run: no active live Teams meeting | Self-termination, `.md` transcript (`0o600` POSIX), and unlocked pending `--append-ai-summary` line in `transcription_queue.txt` | `BLOCKED`: operator human checkpoints (active live scheduled call, overtime stimulus, and call hangup) unavailable while AFK | `false` | `BLOCKED` |
| `live-cli-recap-queue-replay` (`TestLiveTenantDelayedAISummaryQueueReplay`) | `https://teams.microsoft.com/l/meetingrecap?threadId=<redacted-id>` | Live Microsoft Teams tenant (`.env.local.test`) | `chromedp` via `NewRootCommand` (`--queue`) | `headless` | Blocked in AFK dark run: requires live published Meeting Recap with AI Summary | Existing `.md` preserved, `# AI Summary` appended, queue line prefixed with `::` | `BLOCKED`: operator human checkpoint (published post-call Meeting Recap with AI Summary) unavailable while AFK | `false` | `BLOCKED` |

## Resumption for operator-assisted live verification

When an authorized operator is present with `.env.local.test` populated
(`GO_TEAMS_INTEGRATION=1`, `TEAMS_PROFILE_DIR` inside gitignored `./bin/`,
`TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_ACCOUNT`, and
`TEAMS_BROWSER_PATH`), execute the four Tier 5 commands documented in
[`docs/USER_GUIDE.md`](../../USER_GUIDE.md#live-tenant-scheduled-call-and-recap-ai-summary-validation)
and update the four `BLOCKED` rows above with observed live timestamps and
verdicts.
