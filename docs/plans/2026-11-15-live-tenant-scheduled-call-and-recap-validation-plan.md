---
title: Live-tenant validation of scheduled meeting join, overtime continuation, call-end silence shutdown, and Idea 2C Recap enrichment implementation plan
description: Implementation plan for 029-F / 029-S covering stash entry 3EB89E2D
source: docs/decisions/2026-11-15-live-tenant-scheduled-call-and-recap-validation-deliberation.md
deliberation_id: 018-DL
feature: 029-F
shipment: 029-S
source_stash:
  - 3EB89E2D
date: 2026-11-15
status: approved
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

Following the accepted deliberation in [2026-11-15-live-tenant-scheduled-call-and-recap-validation-deliberation.md](../decisions/2026-11-15-live-tenant-scheduled-call-and-recap-validation-deliberation.md) (`018-DL`), stash entry **`3EB89E2D`** (`task`, `low`, follow-up from `022-S` operational closure in [2026-11-13-022-S-review-and-closure.md](../closure/2026-11-13-022-S-review-and-closure.md)) is planned under covering feature **`029-F`** and shipment **`029-S`**, sequenced immediately after `028-S` in the shipment dependency DAG **`028-S -> 029-S -> 010-S`**.

Shipment `022-S` (`022-F`, `R1`–`R15`) implemented protocol-driven capture modes (`auto`, `ephemeral-live`, `live-with-summary`, `past-recap`), activity-aware nominal `--duration` and overtime continuation via live caption DOM mutations (including throttled 1000 ms `interim: true` heartbeats) and PCM voice energy (`PCMRMS` / `HasVoiceActivity >= 300.0`), in-browser call-end teardown detection (`callEndedProbeJS` / `shouldNotifyCallEnded`) + `--idle-timeout` silence shutdown, `go-teams schedule` recurring `cron` and one-shot `at` triggers with `busy` gate serialization, and the Idea 2C hybrid delayed AI Summary enrichment workflow (`--ai-summary-delay` in-process timer and crash-safe `transcription_queue.txt` `--append-ai-summary <md> --not-before <RFC3339>` fallback). Shipment `028-S` (`028-F`, `017-DL`) subsequently delivered `.env.example` and the fail-closed `.env.local.test` loaders (`browser.LoadTestEnv(t)` in `internal/browser/integration_helper_test.go` and `loadCLITestEnv(t)` in `internal/cli/binary_automation_test.go`).

This plan delivers:

1. Browser-level real-Chromium E2E (`internal/browser/meeting_e2e_test.go`) and `-tags=integration` live-tenant (`internal/browser/scheduled_lifecycle_integration_test.go`) tests exercising scheduled meeting join, overtime continuation activity callbacks (`OnCaptionActivity`, `OnAudioActivity`), in-browser call-end teardown (`OnCallEnded`), and `ForceRecap` / `AISummaryOnly` (`AISummaryProvider.AISummary()`) extraction.
2. CLI-level real-Chromium E2E (`internal/cli/e2e_test.go`) and `-tags=integration` live-tenant (`internal/cli/scheduled_integration_test.go`) tests exercising `go-teams join --mode live-with-summary --duration ... --idle-timeout ... --ai-summary-delay ...` and `go-teams --queue` across scheduled join, overtime continuation, call-end silence shutdown, and Idea 2C delayed AI Summary Recap enrichment (`# AI Summary` appended idempotently and `transcription_queue.txt` entry prefixed with `::`).
3. Updated developer and test-tier documentation in `docs/USER_GUIDE.md` and `docs/QUALITY_SCORE.md`.
4. A structured browser-verification and runtime-verification runbook in `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` and a cross-reference update in `docs/closure/2026-11-13-022-S-review-and-closure.md`.

### Explicit Out-of-Scope Boundaries

* Production `.go` source files under `cmd/` and `internal/` are not modified; all code changes in `029-S` are confined to `*_test.go` test suites and `docs/*.md` documentation/closure artifacts.
* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and its target file `internal/browser/meeting_integration_test.go` are not modified by `029-S` (`029-S` uses dedicated files `internal/browser/scheduled_lifecycle_integration_test.go` and `internal/cli/scheduled_integration_test.go` so there is zero file overlap with `010-S`).
* Active stash entry **`9BDFB275`** (`DEFERRED SCOPE EXPANSION` captured during `028-S` closure PR #71 regarding external `autoharness` package) remains untouched in `active` stash state per P-017 and P-021.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Validate browser-level scheduled meeting join, overtime activity callbacks (`OnCaptionActivity` including 1000 ms `interim: true` heartbeats and `OnAudioActivity` via `HasVoiceActivity`), in-browser call-end teardown (`OnCallEnded`), and post-call Recap AI Summary extraction (`ForceRecap: true, AISummaryOnly: true`) in both real-Chromium E2E (`internal/browser/meeting_e2e_test.go`) and `-tags=integration` live-tenant (`internal/browser/scheduled_lifecycle_integration_test.go`) test suites using `browser.LoadTestEnv(t)` | `3EB89E2D` | U1 (`029.001-T`) | `go test -race ./internal/browser/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/browser/...` |
| R2: Validate CLI-level scheduled meeting join (`go-teams join --mode live-with-summary --duration ... --idle-timeout ... --ai-summary-delay ...`), overtime continuation past `--duration`, in-browser call-end silence shutdown, and Idea 2C delayed AI Summary Recap enrichment (`transcription_queue.txt` `--append-ai-summary --not-before` persistence, Recap extraction, `# AI Summary` append, and `::` completion) in both real-Chromium E2E (`internal/cli/e2e_test.go`) and `-tags=integration` live-tenant (`internal/cli/scheduled_integration_test.go`) test suites using `loadCLITestEnv(t)` | `3EB89E2D` | U2 (`029.002-T`) | `go test -race ./internal/cli/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/cli/...` |
| R3: Update the `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` table row in `docs/USER_GUIDE.md` and document the live-tenant scheduled call and Idea 2C Recap validation runbook, `.env.local.test` prerequisite variables, human checkpoints, and URL shape rules (`/l/meetup-join/` vs `/meet/` + `TEAMS_RECAP_URL`) in `docs/USER_GUIDE.md` and `docs/QUALITY_SCORE.md` | `3EB89E2D` | U3 (`029.003-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `engram verify` |
| R4: Create the structured browser-verification runbook and validator evidence matrix in `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md` and cross-reference `029-S` in `docs/closure/2026-11-13-022-S-review-and-closure.md` | `3EB89E2D` | U4 (`029.004-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `engram verify` |

## Implementation Units

### U1 (`029.001-T`) — Add browser-level scheduled call lifecycle and Recap AI Summary E2E and live-tenant integration tests (`internal/browser`)

* **Domain:** test (`internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Files (2 files, 4 functions, 3 test scenarios):** `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`.
* **What:**
  1. **In `internal/browser/meeting_e2e_test.go` (`package browser`, no build tag — 1 function, 1 sequential two-phase test scenario running in `go test -race ./...` and CI `test-windows`)**:
     * Add `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd(t *testing.T)`:
       * Call `LoadTestEnv(t)` (`028-S`), skip if `runtime.GOOS != "windows"` or `ResolveExecPath("") == ""`.
       * **Phase 1 (Overtime caption activity and in-browser call-end teardown)**:
         * Serve a simulated Teams SPA over `httptest.NewServer` (`/meet/sched-e2e?p=synthetic-placeholder-passcode`):
           - Renders launcher (`button[data-tid="joinOnWeb"]`) -> pre-join (`input[data-tid="toggle-mute"]`, `input[data-tid="prejoin-display-name-input"]`, `button[data-tid="prejoin-join-button"]`) -> lobby (`Someone will let you in shortly`) -> active call (`#hangup-button`, `#callingButtons-showMoreBtn`).
           - When captions are enabled (`#callingButtons-showMoreBtn` -> `#LanguageSpeechMenuControl-id` -> `#closed-captions-button`), injects `[data-tid="closed-caption-renderer-wrapper"]`, emits an unpunctuated rolling caption mutation at `100ms` (triggering an `interim: true` activity heartbeat via `captionObserverJS`), emits a punctuated final caption at `700ms` (`"Overtime discussion still active."`), and at `1100ms` replaces the call toolbar with `<div data-tid="call-ended-screen">You left the meeting</div>` (removing `#hangup-button`).
         * Allocate `liveProfileDir := t.TempDir()` (with a bounded 10-attempt `os.RemoveAll` retry in `t.Cleanup`) and call `NewLiveMeeting` with `Options{UserDataDir: liveProfileDir, Headless: true}` and `MeetingOptions{MeetingURL: srv.URL + "/meet/sched-e2e?p=synthetic-placeholder-passcode", BotName: "Sched Bot", LobbyTimeout: 20 * time.Second, OnCaptionActivity: ..., OnCallEnded: ...}` (protecting callback counters/channels with `sync/atomic` and `sync.Once`, and setting `cm, ok := m.(*chromeMeeting); require.True(t, ok); cm.callEndPoll = 250 * time.Millisecond` synchronously before calling `Join` / `EnableCaptions`).
         * Assert `Join(ctx)` succeeds, run `EnableCaptions` in a goroutine, wait until both the finalized caption `"Overtime discussion still active."` (`OnCaptionActivity` count `>= 2`) and `OnCallEnded` have fired, then call `cancel()` on the child context and drain `<-errCh` (`require.NoError(t, <-errCh)`) before `cleanup()` closes the Phase 1 browser.
       * **Phase 2 (`ForceRecap` + `AISummaryOnly` extraction)**:
         * Serve a simulated Teams Meeting Recap page over `httptest.NewServer` (`/meet/recap-e2e?p=synthetic-placeholder-passcode`) containing `<div data-tid="intelligent-recap-header">Recap</div>`, `<button role="tab" data-tid="Notes" aria-selected="false">Notes</button>`, `<button role="tab" data-tid="AINotes" aria-selected="false">AI notes</button>`, and `<button aria-label="Copy all">Copy all</button>` whose click handler calls `navigator.clipboard.writeText("Meeting notes:\nFollow-up actions confirmed.")`.
         * Allocate a fresh `recapProfileDir := t.TempDir()` (with a bounded 10-attempt `os.RemoveAll` retry in `t.Cleanup`) and call `NewLiveMeeting` with `Options{UserDataDir: recapProfileDir, Headless: true}` and `MeetingOptions{MeetingURL: recapSrv.URL + "/meet/recap-e2e?p=synthetic-placeholder-passcode", ForceRecap: true, AISummaryOnly: true, LobbyTimeout: 20 * time.Second}` (`require.NoError(t, err)` and `defer recapCleanup()`).
         * Assert `Join(recapCtx)` succeeds without entering live pre-join, check `sp, ok := recapMeeting.(AISummaryProvider); require.True(t, ok)`, and assert `sp.AISummary()` equals `"Meeting notes:\nFollow-up actions confirmed."`.
  2. **Create `internal/browser/scheduled_lifecycle_integration_test.go` (`//go:build integration`, `package browser_test`, 3 functions, 2 test scenarios)**:
     * Define package-level constants `const liveNominalDuration = 15 * time.Second` and `const liveOvertimeWindow = 15 * time.Second`.
     * Implement helper `recapIntegrationURL(t *testing.T) (recapURL string, profileDir string)`:
       * Calls `requireIntegration(t)` (which invokes `browser.LoadTestEnv(t)`).
       * Reads `profileDir = strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))` (skipping if `profileDir == ""`), and verifies that `profileDir` resolves within the canonicalized repository root (`filepath.EvalSymlinks` + `filepath.IsLocal`) and is ignored by Git (`git check-ignore -q -- <relPath>` with `cmd.Dir = repoRoot`), failing closed without echoing `profileDir` on error.
       * Checks `raw := strings.TrimSpace(os.Getenv("TEAMS_RECAP_URL"))`. If `raw == ""`, checks whether `mURL := strings.TrimSpace(os.Getenv("TEAMS_MEETING_URL"))` contains `"meetingrecap"` or a `threadId` (`"19:meeting_"` / `"19%3ameeting_"` case-insensitively); if so, uses `mURL`, otherwise calls `t.Skip("set TEAMS_RECAP_URL (or a TEAMS_MEETING_URL carrying threadId) to run live Recap AI Summary integration tests")`.
       * Normalizes and validates `raw` via `mutated, err := teams.MutateJoinURL(raw)` (`require.NoError(t, err, "invalid recap URL in test environment")` without echoing `raw`).
     * Implement `TestMeetingLiveScheduledOvertimeAndCallEnd(t *testing.T)`:
       * Calls `requireIntegration(t)`, reads `url := meetingURL(t)` and `profile := strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))` (skipping if `profile == ""` and verifying `profile` is workspace-contained and git-ignored via the same check in `recapIntegrationURL`).
       * Allocates `m, bctx, cleanup, err := browser.NewLiveMeeting(...)` (`require.NoError(t, err)` and `defer cleanup()`) with `Headless: true`, `UserDataDir: profile`, `ExecPath: os.Getenv("TEAMS_BROWSER_PATH")`, `DefaultAccount: os.Getenv("TEAMS_ACCOUNT")`, and `MeetingOptions{MeetingURL: url, BotName: "Transcription Agent", DefaultAccount: os.Getenv("TEAMS_ACCOUNT"), LobbyTimeout: 2 * time.Minute, OnCaptionActivity: ..., OnAudioActivity: ..., OnCallEnded: ...}` (recording activity timestamps with `sync/atomic`).
       * Runs `require.NoError(t, m.Join(ctx))`, starts `m.EnableCaptions` and `m.CaptureAudio` concurrently under a cancellable child context `streamCtx, cancelStream := context.WithCancel(ctx)`, waits across `liveNominalDuration + liveOvertimeWindow` (`30s`), asserts that caption or voice activity callbacks fired past `liveNominalDuration` (overtime continuation), clicks `#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]` in-browser via `chromedp.Evaluate` if `OnCallEnded` has not yet fired from an organizer hangup, asserts `OnCallEnded` fires within `15s`, and then calls `cancelStream()` and drains both goroutine error channels before `defer cleanup()` runs.
     * Implement `TestMeetingLiveRecapAISummaryEnrichment(t *testing.T)`:
       * Calls `rURL, profile := recapIntegrationURL(t)`.
       * Allocates `m, bctx, cleanup, err := browser.NewLiveMeeting(...)` (`require.NoError(t, err)` and `defer cleanup()`) with `Options{UserDataDir: profile, Headless: true, ExecPath: os.Getenv("TEAMS_BROWSER_PATH"), DefaultAccount: os.Getenv("TEAMS_ACCOUNT")}` and `MeetingOptions{MeetingURL: rURL, ForceRecap: true, AISummaryOnly: true, LobbyTimeout: 2 * time.Minute, DefaultAccount: os.Getenv("TEAMS_ACCOUNT")}`, calls `require.NoError(t, m.Join(ctx))`, checks `sp, ok := m.(browser.AISummaryProvider); require.True(t, ok)`, and asserts `strings.TrimSpace(sp.AISummary()) != ""`.
  3. **Verify:** Run `go test -race ./internal/browser/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/browser/...`.

### U2 (`029.002-T`) — Add CLI end-to-end and live-tenant integration tests for scheduled join, overtime, call-end silence shutdown, and Idea 2C delayed AI Summary enrichment (`internal/cli`)

* **Domain:** test (`internal/cli/e2e_test.go`, `internal/cli/scheduled_integration_test.go`). **Posture:** test-first (Principle II). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `029.001-T` (`blocks`).
* **Files (2 files, 4 functions, 3 test scenarios):** `internal/cli/e2e_test.go`, `internal/cli/scheduled_integration_test.go`.
* **What:**
  1. **In `internal/cli/e2e_test.go` (`package cli`, no build tag — 1 function, 1 test scenario running in `go test -race ./...` and CI `test-windows`)**:
     * Add `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd(t *testing.T)`:
       * Call `loadCLITestEnv(t)` (`028-S`), skip if `runtime.GOOS != "windows"` or `browser.ResolveExecPath("") == ""`.
       * Start an `httptest.NewServer` that ignores non-meeting paths (`if !strings.HasPrefix(r.URL.Path, "/meet/") { w.WriteHeader(http.StatusNoContent); return }`) before incrementing an `atomic.Int32` visit counter:
         - **Visit 1 (Live scheduled call)**: Renders the Teams call SPA (launcher `joinOnWeb` -> pre-join mute + bot name + `prejoin-join-button` -> lobby -> admitted `#hangup-button` + `#callingButtons-showMoreBtn`). When captions are turned on (`#callingButtons-showMoreBtn` -> `#closed-captions-button`), emits Utterance 1 at `150ms` (`"Initial scheduled utterance."`), emits an unpunctuated rolling caption mutation at `750ms` (refreshing `lastActivity` via `interim: true` so `now.Sub(lastActivity)` stays `< 1500ms`), emits Utterance 2 into overtime at `1350ms` (`"Overtime scheduled utterance after nominal duration."` — past `--duration 1000ms`), and at `1550ms` removes `#hangup-button` and renders `<div data-tid="call-ended-screen">You left the meeting</div>` (detected by `pollCallEnded` at `2000ms`).
         - **Visit 2+ (Delayed Idea 2C Recap AI Summary pass after `--ai-summary-delay`)**: Renders the Teams Meeting Recap page (`<div data-tid="intelligent-recap-header">Recap</div>`, `<button role="tab" data-tid="Notes" aria-selected="false">Notes</button>`, `<button role="tab" data-tid="AINotes" aria-selected="false">AI notes</button>`, and `<button aria-label="Copy all">Copy all</button>` whose click listener calls `navigator.clipboard.writeText("Summary notes:\nOvertime decision recorded.")`).
       * Write a temporary `config.yaml` in `t.TempDir()` using `filepath.ToSlash(queuePath)` (`queue_file: <slashQueuePath>`) so Windows `\t` sequences are never interpreted as YAML tab escapes, and register a bounded 10-attempt `os.RemoveAll(profileDir)` retry in `t.Cleanup`.
       * Execute `NewRootCommand()` under a `45s` outer timeout context `ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)` with args:
         `join`, `srv.URL + "/meet/sched123?p=synthetic-placeholder-passcode"`, `--config`, `cfgPath`, `--mode`, `live-with-summary`, `--duration`, `1000ms`, `--idle-timeout`, `1500ms`, `--ai-summary-delay`, `250ms`, `--category`, `SCHED`, `--output-dir`, `outDir`, `--profile-dir`, `profileDir`, `--headless`.
       * Assert:
         - `ExecuteContext(ctx)` returns `nil` on its own and `require.NoError(t, ctx.Err())` holds (proving self-termination without outer context timeout).
         - The written `.md` file in `outDir` contains both `"Initial scheduled utterance."` and the overtime caption `"Overtime scheduled utterance after nominal duration."`, plus the appended `# AI Summary` section containing `"Summary notes:\nOvertime decision recorded."`.
         - The `transcription_queue.txt` file exists, contains `--append-ai-summary` and `--not-before`, and has its line prefixed with `::` after the delayed AI Summary enrichment completes.
  2. **Create `internal/cli/scheduled_integration_test.go` (`//go:build integration`, `package cli`, 3 functions, 2 test scenarios)**:
     * Define package-level constants `const liveCLINominalDuration = 15 * time.Second` and `const liveCLIIdleTimeout = 15 * time.Second`.
     * Implement `requireCLIIntegration(t *testing.T) string`: calls `loadCLITestEnv(t)`, skips when `strings.ToLower(strings.TrimSpace(os.Getenv("GO_TEAMS_INTEGRATION")))` is `""`, `"0"`, or `"false"`, reads `profile := strings.TrimSpace(os.Getenv("TEAMS_PROFILE_DIR"))` (skipping if `profile == ""`), verifies `profile` resolves inside the canonicalized repository root (`filepath.EvalSymlinks` + `filepath.IsLocal`) and is ignored by Git (`git check-ignore -q -- <relPath>` with `cmd.Dir = repoRoot`) without echoing `profile` on error, and returns `profile`.
     * Implement `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown(t *testing.T)`:
       * Calls `profile := requireCLIIntegration(t)`, reads `rawURL := strings.TrimSpace(os.Getenv("TEAMS_MEETING_URL"))` (skipping if `rawURL == ""`), and validates `rawURL` via `teams.ValidateMeetingURL(rawURL)` (`require.NoError(t, err, "invalid meeting URL in test environment")` without echoing `rawURL`).
       * Configures `outDir := t.TempDir()` and `queuePath := filepath.Join(t.TempDir(), "transcription_queue.txt")`, writes a hermetic temporary `config.yaml` with `queue_file: <filepath.ToSlash(queuePath)>`, and executes `NewRootCommand()` in a goroutine under an outer context `outerCtx, cancelOuter := context.WithTimeout(context.Background(), 5*time.Minute)` and a child context `runCtx, cancelRun := context.WithCancel(outerCtx)` with `join`, `rawURL`, `--config`, `cfgPath`, `--mode`, `live-with-summary`, `--duration`, `liveCLINominalDuration.String()`, `--idle-timeout`, `liveCLIIdleTimeout.String()`, `--ai-summary-delay`, `30m`, `--lobby-timeout`, `2m`, `--category`, `LIVE`, `--output-dir`, `outDir`, `--profile-dir`, `profile` (plus `--browser-path` and `--account` when set in env), `--headless`.
       * Polls until `queuePath` is non-empty, contains `"--append-ai-summary"` and `"--not-before"` (checked via boolean `strings.Contains` predicates so raw queue lines are never echoed on failure), AND `queuePath + ".lock"` does not exist (`errors.Is(statErr, os.ErrNotExist)`), verifies `require.NoError(t, outerCtx.Err())` (proving `Session.Run` self-terminated via the duration/overtime/call-end watchdog rather than outer timeout), and then calls `cancelRun()` to unblock the 30-minute `--ai-summary-delay` timer and waits for `<-errCh` (`require.NoError(t, <-errCh)`).
       * Asserts that a non-empty `.md` transcript file was flushed in `outDir` (`0o600` permissions on POSIX) and that `queuePath` holds the pending `--append-ai-summary` entry referencing that `.md` file.
     * Implement `TestLiveTenantDelayedAISummaryQueueReplay(t *testing.T)`:
       * Calls `profile := requireCLIIntegration(t)`, resolves `rawRecapURL` from `os.Getenv("TEAMS_RECAP_URL")` (or `os.Getenv("TEAMS_MEETING_URL")` when it contains `"meetingrecap"` or a `threadId` `"19:meeting_"` / `"19%3ameeting_"`), skipping if unset, and validates/normalizes via `recapURL, err := teams.MutateJoinURL(rawRecapURL)` (`require.NoError(t, err, "invalid recap URL in test environment")` without echoing `rawRecapURL`).
       * Writes a hermetic temporary `config.yaml` in `t.TempDir()` (`cfgPath`), creates a temporary `.md` transcript file (`"# Meeting Transcript\n\n> **Alice:** Live capture.\n"`, mode `0o600`), and writes a `transcription_queue.txt` file containing a single due line via `appendDeferredAISummaryQueueLineWithConfig(queuePath, recapURL, mdPath, time.Now().Add(-1*time.Minute), config.Config{ConfigPath: cfgPath, ProfileDir: profile, BrowserPath: strings.TrimSpace(os.Getenv("TEAMS_BROWSER_PATH")), DefaultAccount: strings.TrimSpace(os.Getenv("TEAMS_ACCOUNT")), LobbyTimeout: "2m", Headless: true})`.
       * Executes `NewRootCommand()` with `--config`, `cfgPath`, `--queue`, `queuePath` under a `3m` timeout context (`require.NoError(t, err)` and `require.NoError(t, ctx.Err())`).
       * Asserts (using boolean `strings.Contains` / `strings.HasPrefix` predicates that never echo raw queue lines) that `mdPath` preserves `"> **Alice:** Live capture."`, now contains `"# AI Summary"`, and that the line in `queuePath` is prefixed with `"::"`.
  3. **Verify:** Run `go test -race ./internal/cli/...`, `go vet ./...`, `go vet -tags=integration ./...`, and `go test -tags=integration -run '^$' ./internal/cli/...`.

### U3 (`029.003-T`) — Document the live-tenant scheduled call and Idea 2C Recap validation runbook in `docs/USER_GUIDE.md` and `docs/QUALITY_SCORE.md`

* **Domain:** docs (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`). **Posture:** characterization-first. **Size:** `XS` | **Complexity:** `low`.
* **Depends on:** `029.001-T`, `029.002-T` (`blocks`).
* **Files (2 files):** `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`.
* **What:**
  1. **In `docs/USER_GUIDE.md` (`### Integration tests`)**:
     * Update the `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` row in the environment-variable table to remove `(no live recap test consumes it yet)` and state that it is consumed by `TestMeetingLiveRecapAISummaryEnrichment` (`internal/browser`) and `TestLiveTenantDelayedAISummaryQueueReplay` (`internal/cli`).
     * Add `#### Live-tenant scheduled call and Recap AI Summary validation` documenting:
       * Prerequisite setup in `.env.local.test` (`GO_TEAMS_INTEGRATION="1"`, `TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`, `TEAMS_PROFILE_DIR` inside gitignored `./bin/`, `TEAMS_BROWSER_PATH`, `TEAMS_ACCOUNT`).
       * Exact commands for running the browser-level (`TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`) and CLI-level (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`) live-tenant validation tests.
       * Operator human checkpoints: (1) organizer meeting start and lobby admission, (2) active speech/captions across the 15-second nominal duration boundary into overtime followed by call hangup or silence for `--idle-timeout`, and (3) post-call Recap AI Summary generation window (`5–30m` after call end, or immediate replay against an existing `TEAMS_RECAP_URL`).
       * URL shape distinction for Idea 2C Recap enrichment (`/l/meetup-join/19%3ameeting_...` automatic `deriveRecapURLFromMeetingURL` derivation vs `/meet/<numeric-id>?p=<passcode>` paired with `TEAMS_RECAP_URL`).
  2. **In `docs/QUALITY_SCORE.md` (`## Grades by package`, `### Tier 2`, `### Tier 5`, `## Live-verification gaps`, and `## Improvement roadmap`)**:
     * Update the `internal/browser` rationale in `## Grades by package` and item 1 of `## Improvement roadmap` to reference `010-S` and `029-S` together.
     * Update `### Tier 2` to mention `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` (`internal/browser/meeting_e2e_test.go`) and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` (`internal/cli/e2e_test.go`).
     * Update `### Tier 5` and `## Live-verification gaps` to reference `029-S` (`scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go`) alongside `010-S`.
  3. **Verify:** Run `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `engram --workspace "C:\Source\GitHub\go-teams" verify` on modified markdown files.

### U4 (`029.004-T`) — Record live-tenant scheduled call validation runbook and `022-S` closure cross-reference in `docs/closure/`

* **Domain:** docs (`docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`, `docs/closure/2026-11-13-022-S-review-and-closure.md`). **Posture:** characterization-first. **Size:** `XS` | **Complexity:** `low`.
* **Depends on:** `029.003-T` (`blocks`).
* **Files (2 files):** `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`, `docs/closure/2026-11-13-022-S-review-and-closure.md`.
* **What:**
  1. **Create `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`**:
     * Record the browser-verification and runtime-verification runbook for `029-S` (`029-F`, `018-DL`, source stash `3EB89E2D`, sequenced after `010-S`):
       * Redacted target route classes (`https://teams.microsoft.com/meet/<redacted-id>`, `https://teams.microsoft.com/l/meetingrecap?threadId=<redacted-id>`) with all query tokens (`?p=`, `driveId=`, `driveItemId=`, `fileUrl=`, `context=`) and path identifiers replaced by `<redacted-id>`.
       * Headed vs. headless mode selection rationale (`--headless=new` via `cleanHeadlessAllocatorOptions` for automated scheduled capture and Recap extraction; `--headed` for initial cold-profile interactive MFA bootstrap).
       * Human checkpoint protocol (organizer lobby admission, overtime speech/caption stimulus + call-end silence transition, post-call Recap availability).
       * Validator evidence matrix with explicit `browser-verification.instructions.md` columns (`probe_id`, `route`, `environment`, `adapter`, `mode`, `server_reachability`, `expected_confirmation`, `observed_result`, `blocks_release`, `verdict`) distinguishing Tier 2 real-Chromium synthetic E2E verdicts (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) from Tier 5 live-tenant `-tags=integration` verdicts (`TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`, `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay`), explicitly requiring observed execution evidence before any Tier 5 flow is marked `PASS` (`BLOCKED` / `READY_WITH_CONDITIONS` when live tenant credentials/meeting are absent).
  2. **In `docs/closure/2026-11-13-022-S-review-and-closure.md`**:
     * In `## Follow-ups` and `## Verification evidence`, annotate (preserving the original closure record intact) that stash `3EB89E2D` has been staged and implemented under shipment `029-S` (`029-F`, deliberation `018-DL`, sequenced after `028-S -> 010-S -> 029-S`) with cross-links to `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`.
  3. **Verify:** Run `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` and `engram --workspace "C:\Source\GitHub\go-teams" verify` on modified markdown files.

## Dependency Graph

```text
029.001-T (U1: internal/browser/meeting_e2e_test.go + scheduled_lifecycle_integration_test.go)
    |
    +---> 029.002-T (U2: internal/cli/e2e_test.go + scheduled_integration_test.go)
              |
              +---> 029.003-T (U3: docs/USER_GUIDE.md + docs/QUALITY_SCORE.md)
                        |
                        +---> 029.004-T (U4: docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md + docs/closure/2026-11-13-022-S-review-and-closure.md)
```

* **Shipment sequencing DAG**:
  * `010-S` depends on `028-S` (`backlogit dep add 010-S 028-S --type blocks` — `028-S` is `shipped`, placing `010-S` in `ready_set`).
  * `029-S` depends on `010-S` (`backlogit dep add 029-S 010-S --type blocks` — sequencing `029-S` immediately after `010-S`).

## Decisions and Rationale

1. **Dual-tier verification (Real-Chromium E2E in `go test -race ./...` + `-tags=integration` live-tenant suites)**: Adding `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` (`internal/browser/meeting_e2e_test.go`) and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` (`internal/cli/e2e_test.go`) guarantees that every `go test -race ./...` run on Windows and in GitHub Actions (`test-windows`) exercises the full `022-S` scheduled call join, overtime continuation, call-end silence shutdown, and Idea 2C delayed AI Summary Recap enrichment pipeline in a real headless Chromium browser, while `scheduled_lifecycle_integration_test.go` and `scheduled_integration_test.go` provide the `-tags=integration` live-tenant harnesses powered by `028-S`'s `.env.local.test`.
2. **Zero file overlap with `010-S`**: Placing the new `-tags=integration` tests in `internal/browser/scheduled_lifecycle_integration_test.go` and `internal/cli/scheduled_integration_test.go` avoids touching `internal/browser/meeting_integration_test.go` (which is modified by `010-S`), eliminating merge conflicts across the `010-S -> 029-S` shipment chain.
3. **Handling both `/l/meetup-join/` and `/meet/` URL shapes for Recap AI Summary enrichment**: Because `/meet/<numeric-id>?p=<passcode>` links do not carry a `threadId` in the URL query string, `TestMeetingLiveRecapAISummaryEnrichment` and `TestLiveTenantDelayedAISummaryQueueReplay` check `TEAMS_RECAP_URL` first and fall back to `TEAMS_MEETING_URL` only when `TEAMS_MEETING_URL` contains `"meetingrecap"` or a `threadId` (`"19:meeting_"` / `"19%3ameeting_"`).

## Risks and Caveats

* **Secret leakage in test failures or closure artifacts**: All new integration tests validate URLs via `teams.ValidateMeetingURL` without echoing raw URLs, passcodes, or account UPNs in error messages, and all tracked documentation/closure files use synthetic or `<redacted-id>` placeholders verified by `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Windows temporary `UserDataDir` cleanup after Chromium exit**: Both `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` register bounded retry cleanup loops (`for range 10 { if os.RemoveAll(profileDir) == nil { break }; time.Sleep(100 * time.Millisecond) }`) matching `TestChromeMeetingEndToEnd` and `TestCaptionsCommandEndToEndWritesMarkdownFile`.
* **Honest live-tenant verdict reporting (`browser-verification.instructions.md`)**: In unattended or headless CI environments where a live Microsoft Teams organizer session and live meeting stimulus are not active, `-tags=integration` tests skip cleanly when `GO_TEAMS_INTEGRATION` is unset, and the closure runbook records `BLOCKED` / `READY_WITH_CONDITIONS` for unexecuted live-tenant human checkpoints rather than claiming unverified live passes.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, no `unsafe`, zero lint findings, all errors checked |
| II. Test-First Development | Compliant — U1 and U2 add executable real-Chromium E2E tests and `-tags=integration` test suites |
| III. Workspace Isolation | Compliant — all test outputs and profiles use `t.TempDir()` or workspace-contained gitignored paths verified by `LoadTestEnv` / `loadCLITestEnv` |
| IV. CLI Workspace Containment | Compliant — no files created or modified outside the repository root |
| V. Structured Observability | Compliant — validator evidence and releasability signals recorded in `docs/closure/` |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); every task has `<= 2` files, `<= 4` functions, and `<= 3` test scenarios |
| VII. Destructive Command Approval | Compliant — no destructive commands used |
| VIII. Explicit Safety Modes | Compliant — `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded test outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Absent** — No production `.go` files or CLI flags are modified.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present** — Exercises live-tenant authentication/profile state (`TEAMS_PROFILE_DIR`, `TEAMS_ACCOUNT`) and passcode-bearing URLs (`TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`) loaded from `.env.local.test`.
* **Migration, backfill, destructive data/config action, or irreversible step**: **Absent** — Additive test suites and documentation only.
* **External integration, operator checkpoint, or external dependency**: **Present** — Live Microsoft Teams scheduled meeting admission, live speech/caption stimulus across overtime, call-end teardown, and post-call Recap AI Summary availability involve explicit operator human checkpoints under `browser-verification.instructions.md`.
* **High runtime, rollout, or rollback risk**: **Absent** — Confined to `*_test.go` files and documentation.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * Browser E2E and `-tags=integration` test suite (`internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go`)
  * CLI E2E and `-tags=integration` test suite (`internal/cli/e2e_test.go`, `internal/cli/scheduled_integration_test.go`)
  * Developer and quality-grade documentation (`docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`)
  * Browser-verification runbook and `022-S` closure cross-reference (`docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`, `docs/closure/2026-11-13-022-S-review-and-closure.md`)
* **Runtime verification**:
  * Run `go test -race ./...` (executing both new real-Chromium E2E tests `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`, plus `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
  * Run `go vet ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -run '^$' ./internal/browser/... ./internal/cli/...`, `golangci-lint run`, and `gofmt -l .`.
  * When `.env.local.test` is populated with live-tenant credentials and `GO_TEAMS_INTEGRATION=1` (after `010-S` completes), run `go test -tags=integration -v -run "TestMeetingLiveScheduledOvertimeAndCallEnd|TestMeetingLiveRecapAISummaryEnrichment" ./internal/browser/...` and `go test -tags=integration -v -run "TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown|TestLiveTenantDelayedAISummaryQueueReplay" ./internal/cli/...` and record the observed verdicts in `docs/closure/`.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, and releasability evidence contract recorded in `docs/closure/029-S-029-F-post-merge-closure.md` and `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan exercises live-tenant credentials, authenticated browser profiles, and passcode-bearing URLs from `.env.local.test`, and coordinates browser-verification human checkpoints across scheduled meeting admission, overtime speech/caption continuation, in-browser call-end silence shutdown, and post-call Meeting Recap AI Summary enrichment.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/browser-verification.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`

### Protected Invariants

1. **Zero Secret / Passcode Leakage in Test Output, Queue Assertions, and Committed Artifacts**: Neither `scheduled_lifecycle_integration_test.go` nor `scheduled_integration_test.go` may format raw `TEAMS_MEETING_URL`, `TEAMS_RECAP_URL`, `?p=` passcodes, `TEAMS_ACCOUNT` UPNs, or `TEAMS_PROFILE_DIR` paths into `t.Fatal`/`t.Log`/`require` messages, nor may they pass raw `transcription_queue.txt` (`queuePath`) contents to value-dumping `testify` helpers (`require.Contains`, `assert.Contains`, `assert.Equal`). Queue file assertions MUST use boolean predicates (`require.True(t, strings.Contains(qContent, "--append-ai-summary"), "static message")`) or `sanitizeQueueArgsForLog`. Any URL logged in diagnostics MUST pass through `teams.RedactURL`, and any account identifier MUST pass through `browser.RedactAccountIdentifier`. In all tracked documentation and `docs/closure/` artifacts, both query parameters (`?p=`, `driveId=`, `driveItemId=`, `fileUrl=`, `context=`, `threadId=`) and path-embedded meeting identifiers (`/meet/<id>` and `/l/meetup-join/19%3ameeting_...`) MUST be replaced with `<redacted-id>` or synthetic placeholders and pass `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
2. **Conflict-Free Sequencing with `010-S`**: `029-S` MUST NOT modify `internal/browser/meeting_integration_test.go` or any other file in `010-S`'s task manifest, and `029-S` MUST be wired with `backlogit dep add 029-S 010-S --type blocks` (with `010-S` wired via `backlogit dep add 010-S 028-S --type blocks`) so `010-S` executes first and establishes the verified live selector/join/audio baseline before `029-S` runs.
3. **Honest Browser-Verification Evidence (`browser-verification.instructions.md`)**: No live-tenant `-tags=integration` flow may be reported as `PASS` unless live execution against an authorized Microsoft Teams tenant actually occurred and produced observed evidence. When live-tenant human checkpoints (organizer lobby admission, live speech stimulus, post-call Recap generation) are not available in an unattended environment, record `BLOCKED` or `READY_WITH_CONDITIONS` with the exact missing prerequisite, while keeping Tier 2 synthetic real-Chromium E2E verdicts (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) separately reported as `PASS`.
4. **Workspace-Contained Gitignored `TEAMS_PROFILE_DIR`, Shared Profile Serialization, and Clean Chromium Teardown**: Live-tenant integration tests MUST verify that `TEAMS_PROFILE_DIR` resolves within the canonicalized repository root (`filepath.EvalSymlinks` + `filepath.IsLocal`) and is ignored by Git (`git check-ignore -q -- <relPath>` with `cmd.Dir = repoRoot`) before launching Chromium, MUST run sequentially (never `t.Parallel()` on a shared `UserDataDir`), MUST pass a hermetic temporary `config.yaml` (using `filepath.ToSlash(queuePath)`) so ambient host configs are never loaded, and `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown` MUST wait until `queuePath` is written and `queuePath + ".lock"` is removed (with `outerCtx.Err() == nil`) before cancelling `runCtx` to abort the 30-minute `--ai-summary-delay` timer.
5. **Untouched Deferred Scope Expansion (`9BDFB275`)**: Stash entry `9BDFB275` (`DEFERRED SCOPE EXPANSION` captured during `028-S` closure PR #71) MUST remain untouched in `active` stash state per P-017 and P-021.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-029-1` | Add browser-level real-Chromium E2E test (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`) and `-tags=integration` live-tenant test suite (`scheduled_lifecycle_integration_test.go`) | `internal/browser/meeting_e2e_test.go`, `internal/browser/scheduled_lifecycle_integration_test.go` | Browser E2E and live-tenant integration test harness | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-029-2` | Add CLI-level real-Chromium E2E test (`TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`) and `-tags=integration` live-tenant test suite (`scheduled_integration_test.go`) | `internal/cli/e2e_test.go`, `internal/cli/scheduled_integration_test.go` | CLI E2E and live-tenant integration test harness | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-029-3` | Document live-tenant scheduled call and Idea 2C Recap validation runbook, human checkpoints, and closure cross-references | `docs/USER_GUIDE.md`, `docs/QUALITY_SCORE.md`, `docs/closure/browser-automation/2026-11-15-scheduled-call-and-recap-validation-runbook.md`, `docs/closure/2026-11-13-022-S-review-and-closure.md` | Documentation and operational closure runbook | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 and U4 commits |
| `PA-029-4` | Execute operator-supervised `-tags=integration` live-tenant browser verification against an active Microsoft Teams scheduled call and post-call Recap | `TestMeetingLiveScheduledOvertimeAndCallEnd`, `TestMeetingLiveRecapAISummaryEnrichment`, `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`, `TestLiveTenantDelayedAISummaryQueueReplay` | External call (live-tenant browser automation with human checkpoints) | `high` | Operator human checkpoint required (lobby admission, overtime speech stimulus, post-call Recap window) | `planned` | Cancel test context and remove temporary output/queue artifacts |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` (`internal/browser`) and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` (`internal/cli`) pass without skipping on Windows in `go test -race ./...`.
  * [ ] `go vet ./...`, `go vet -tags=integration ./...`, `go test -tags=integration -run '^$' ./internal/browser/... ./internal/cli/...`, `golangci-lint run`, `gofmt -l .`, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` pass with zero findings.
  * [ ] `engram verify` passes on all modified and created markdown files under `docs/`.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `windows_e2e_pass_rate` (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`), (2) `e2e_runtime_seconds` (`< 45s` per E2E test), (3) `integration_compile_status` (`go vet -tags=integration ./...` and `go test -tags=integration -run '^$' ./...`), and (4) `tracked_secret_leak_count` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks == 0`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main` (and during operator-supervised live-tenant scheduled meeting and Recap validation runs).
  * **Dashboard / query**: `gh run list --workflow=ci.yml` (`test`, `test-windows`, `Secret hygiene check (tracked files)`) and local `go test -tags=integration -v ./internal/browser/... ./internal/cli/...`.
  * **Baseline**: 100% pass rate on `CI` (`changes`, `test`, `test-windows`) including non-skipped Windows Chromium E2E tests, and 0 secret-hygiene findings.
  * **Alert threshold**: `>= 1` failure or timeout in `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd`, `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`, or secret-hygiene scanning.
* **Rollback trigger & procedure**:
  * **Trigger**: Any CI flake or failure in the new Chromium E2E tests on `windows-latest`, any compilation failure under `-tags=integration`, or any secret-hygiene violation.
  * **Procedure**: Revert the `029-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, real-Chromium E2E tests, PR CI checks (`changes`, `test`, `test-windows`), and live-tenant `-tags=integration` checks pass and are recorded in `docs/closure/`.
  * `READY_WITH_CONDITIONS`: All local quality gates, real-Chromium E2E tests (`TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` and `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd`), and PR CI checks pass, while operator-supervised live-tenant human checkpoints (`scheduled-meeting-join`, overtime speech stimulus, and 30-minute post-call Recap availability) are documented in the runbook and awaiting an active tenant meeting window.
  * `BLOCKED`: Any pre-deploy audit check, E2E test, `-tags=integration` compile check, or CI job fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona adversarial review panel (`Architecture Strategist`, `Security Lens Reviewer`, `Scope Boundary Auditor`, `Constitution Reviewer`, `Go Reviewer`, `Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`), remediate all surfaced P0/P1/P2/P3 findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all surfaced P1, P2, and P3 findings remediated in-place in `docs/plans/2026-11-15-live-tenant-scheduled-call-and-recap-validation-plan.md` and `docs/decisions/2026-11-15-live-tenant-scheduled-call-and-recap-validation-deliberation.md` prior to harvest; zero P0 findings).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, `strict-safety` `ProposedAction` / `ActionRisk` table including `PA-029-4` for operator-supervised live-tenant browser verification, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1, 3 P2, 1 P3 | Remediated — `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0, 1 P1, 2 P2, 1 P3 | Remediated — `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0, 1 P1, 3 P2 | Remediated — `PASS` |
| `Schema-CLI-Docs Coupling Reviewer` | `subagent` (`multi-agent`) | 0 P0, 2 P1, 1 P2, 1 P3 | Remediated — `PASS` |
| `rubber-duck` (`Correctness Reviewer`) | `subagent` (`multi-agent`) | 0 P0, 3 P1, 2 P2 | Remediated — `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Architecture Strategist`, `Go Reviewer`, `rubber-duck`] Fix E2E overtime caption timing gap, filter `/favicon.ico` in `httptest.NewServer`, and verify `outerCtx.Err() == nil` in U2**:
   * **Finding**: In `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` (`internal/cli/e2e_test.go`), emitting Utterance 1 at `150ms` and Utterance 2 at `1800ms` left a `1650ms` silence gap that exceeded `--idle-timeout 1200ms` once `--duration 1200ms` elapsed, causing `capture.Session.Run`'s 250ms watchdog to shut down before the overtime caption arrived; additionally, Chromium's `GET /favicon.ico` would increment an unfiltered visit counter, and `Session.Run` returns `nil` on caller context expiry.
   * **Remediation Applied**: Updated U2 and `018-DL` to use `--duration 1000ms --idle-timeout 1500ms --ai-summary-delay 250ms` with Utterance 1 at `150ms`, an unpunctuated rolling caption mutation at `750ms` (refreshing `lastActivity` via `interim: true`), Utterance 2 in overtime at `1350ms` (keeping every inter-event interval `< 1500ms`), and `[data-tid="call-ended-screen"]` teardown at `1550ms` (detected by `pollCallEnded` at `2000ms`); filtered non-meeting paths (`!strings.HasPrefix(r.URL.Path, "/meet/")`) before incrementing the visit counter; and required asserting `require.NoError(t, outerCtx.Err())` in both `TestJoinLiveWithSummaryOvertimeCallEndAndDelayedRecapEndToEnd` and `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`.
2. **[P1/P2 — All 7 Reviewers] Remove unrequested `TEAMS_TEST_*` duration env vars and `parse*Duration` helpers to align with `028-S` `.env.local.test` allowlist and strict `< 5` functions / `< 4` test scenarios per task**:
   * **Finding**: `TEAMS_TEST_NOMINAL_DURATION`, `TEAMS_TEST_OVERTIME_WINDOW`, and `TEAMS_TEST_IDLE_TIMEOUT` are not in `028-S`'s 13-key `.env.local.test` allowlist (`LoadTestEnv` / `loadCLITestEnv`), and adding `parseIntegrationDuration` / `parseCLIIntegrationDuration` plus two subtests in `meeting_e2e_test.go` pushed U1 and U2 to `5` functions (`>= 5`).
   * **Remediation Applied**: Removed the unrequested `TEAMS_TEST_*` duration override env vars and `parse*Duration` helpers from R3, U1, U2, U3, and `018-DL` in favor of fixed `15 * time.Second` test constants, and structured `TestChromeScheduledLifecycleAndRecapAISummaryEndToEnd` as 1 sequential two-phase test scenario, bringing both U1 and U2 to `2` files, `4` functions (`< 5`), and `3` test scenarios (`< 4`).
3. **[P1 — `Security Lens Reviewer`, `Architecture Strategist`, `rubber-duck`] Zero secret echo on `queuePath` assertions, `TEAMS_PROFILE_DIR` workspace/gitignore validation, Windows YAML path escaping, and hermetic `--config` in `TestLiveTenantDelayedAISummaryQueueReplay`**:
   * **Finding**: Passing raw `transcription_queue.txt` (`queuePath`) contents to `testify` `require.Contains` / `assert.Contains` dumps raw URLs (`?p=`), `--account`, and `--profile-dir` on failure; `TestLiveTenantDelayedAISummaryQueueReplay` omitted `teams.MutateJoinURL(rawRecapURL)` and a hermetic `--config` file (falling back to `%APPDATA%\go-teams\config.yaml` and `%LOCALAPPDATA%\go-teams\profile`); Windows `\t` in `queue_file` YAML strings can be parsed as tab escapes; and `TEAMS_PROFILE_DIR` needed a workspace-containment + `git check-ignore -q` precheck.
   * **Remediation Applied**: Updated U1, U2, and Protected Invariants #1 and #4 to forbid passing raw `queuePath` contents to value-dumping `testify` helpers (using boolean `strings.Contains` / `strings.HasPrefix` predicates instead), validate `TEAMS_PROFILE_DIR` via `filepath.EvalSymlinks` + `filepath.IsLocal` + `git check-ignore -q`, validate `rawRecapURL` via `teams.MutateJoinURL`, pass a hermetic temporary `config.yaml` (using `filepath.ToSlash(queuePath)`) in both live-tenant CLI tests, and wait for `queuePath + ".lock"` removal before cancelling `runCtx`.
4. **[P1/P2/P3 — `Schema-CLI-Docs Coupling Reviewer`, `Go Reviewer`, `Constitution Reviewer`] Update `docs/USER_GUIDE.md` `TEAMS_RECAP_URL` table row, `docs/QUALITY_SCORE.md` rationale/roadmap, checked type assertions, goroutine barriers, `PA-029-4`, and `browser-verification` columns**:
   * **Remediation Applied**: Updated U3 (`029.003-T`) to revise the `TEAMS_RECAP_URL` / `TEAMS_TEST_RECAP_URL` table row in `docs/USER_GUIDE.md` and the `internal/browser` rationale and roadmap in `docs/QUALITY_SCORE.md`; updated U1 (`029.001-T`) with checked two-value type assertions (`cm, ok := m.(*chromeMeeting)` and `sp, ok := m.(AISummaryProvider)`), explicit child-context `cancel()` + goroutine error draining before `defer cleanup()`, and in-browser `#hangup-button` click fallback after `liveNominalDuration + liveOvertimeWindow`; and updated U4 (`029.004-T`) and `## Plan Hardening` with the full `browser-verification.instructions.md` validator evidence columns, `PA-029-4`, and explicit SLIs.
