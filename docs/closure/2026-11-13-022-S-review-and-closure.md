---
title: Scheduled and protocol-driven live call capture review and operational closure
date: 2026-11-13
feature: 022-F
shipment: 022-S
branch: feat/022-s-scheduled-and-protocol-driven-live-call-capture-r1-r15
status: closed
closure_status: READY_WITH_CONDITIONS
compaction_status: done
merge_commit: 7b6c31d98ab00f1aaea759e87f5c073936a53dbc
pull_request: 55
---

## Release scope

Shipment `022-S` (`022-F`, `022.001-T`..`022.011-T`) implements scheduled and
protocol-driven live call capture (`R1`–`R15`, deliberation `011-DL`, source
stash `77FC68CA`), resolving unbounded live-capture hangs in `teams-cli schedule`,
enabling one-shot OS scheduler automation via `teams-cli join --duration`,
extending active calls into overtime while caption or PCM voice activity
persists, detecting in-browser call-end teardown, supporting recurring `cron`
and one-shot `at` triggers with per-meeting overrides and fallback
`MeetingTitle` propagation, and implementing the Idea 2C hybrid in-process
timer + crash-safe `transcription_queue.txt` (`--append-ai-summary <md>
--not-before <RFC3339>`) workflow for delayed post-call AI Summary enrichment.

* `022.001-T` (`internal/config/config.go`, `internal/config/config_test.go`,
  commits `4e07c78`, `fa8b0a9`, `f1521f3`):
  * Adds capture mode constants (`CaptureModeAuto`, `CaptureModeEphemeralLive`,
    `CaptureModeLiveWithSummary`, `CaptureModePastRecap`), `IsValidCaptureMode`,
    and sentinel errors (`ErrInvalidCaptureMode`, `ErrInvalidDuration`,
    `ErrMeetingScheduleConflict`).
  * Extends `Config` with `CaptureMode` (`mode` alias), `Duration`,
    `IdleTimeout`, `AISummaryDelay`, `LobbyTimeout`, `QueueFile`, and transient
    runtime fields (`ConfigPath`, `MeetingTitle`, `AppendAISummary`,
    `NotBefore`, `ExplicitCaptureAudio`), and extends `MeetingSchedule` with
    `At`, `Mode` (`capture_mode` alias), `Duration`, `IdleTimeout`,
    `AISummaryDelay`, `LobbyTimeout`, `Category`, `CaptureAudio`/`Audio`, and
    `OutputDir`.
* `022.002-T` (`internal/scheduler/schedule.go`,
  `internal/scheduler/schedule_test.go`, commit `52eaa69`):
  * Adds `ParseAt` (RFC 3339 and local `YYYY-MM-DD HH:MM`), `Entry.Validate`,
    and `BuildWithClock` with a single-fire `oneShotSchedule` adapter that
    skips past `at` timestamps on startup and fires future `at` entries once
    without repeating.
* `022.003-T` (`internal/browser/prejoin.go`, `internal/browser/captions.go`,
  `internal/browser/audio.go`, commit `26e70e5`):
  * Adds `callEndedScreenSel`, `callEndedProbeJS()`, and
    `shouldNotifyCallEnded`, throttled 1000 ms `interim: true` caption mutation
    heartbeats in `captionObserverJS` + `handleCaptionPayloadWithActivity`, and
    `PCMRMS` / `HasVoiceActivity` (`DefaultVoiceRMSThreshold = 300.0`).
* `022.004-T` (`internal/browser/meeting.go`,
  `internal/browser/meeting_chrome.go`, commits `d037846`, `f1521f3`):
  * Adds `ErrLobbyTimeout`, `ActivityMonitor`, and `MeetingOptions` fields
    (`LobbyTimeout`, `ForceRecap`, `VoiceRMSThreshold`, `OnCaptionActivity`,
    `OnAudioActivity`, `OnCallEnded`), `deriveRecapURLFromMeetingURL`, bounded
    2-minute `defaultForceRecapTimeout`, and 2-second background call-end
    polling in `EnableCaptions`.
* `022.005-T` (`internal/browser/audio_test.go`,
  `internal/browser/captions_test.go`, `internal/browser/prejoin_test.go`,
  commit `96a8a53`):
  * Unit tests for `PCMRMS` / `HasVoiceActivity`, `interim: true` caption
    heartbeats, `callEndedProbeJS` / `shouldNotifyCallEnded`, `ForceRecap`, and
    `LobbyTimeout` (`ErrLobbyTimeout`).
* `022.006-T` (`internal/capture/output.go`, `internal/capture/session.go`,
  commit `28e78b5`):
  * Adds `AppendAISummaryToFile` (`O_WRONLY|O_APPEND`, idempotent) and extends
    `capture.Session.Run` with activity-aware nominal `Duration`, overtime
    continuation while caption or voice activity persists within `IdleTimeout`
    (default `3m`), early shutdown on `RecordCallEnded`, and fallback
    `MeetingTitle` propagation.
* `022.007-T` (`internal/capture/session_test.go`, commit `5982fb4`):
  * Deterministic unit tests for nominal duration + idle shutdown, overtime
    continuation, call-end early stop, fallback `MeetingTitle` vs. DOM title
    precedence, and idempotent `AppendAISummaryToFile`.
* `022.008-T` & `022.009-T` (`internal/cli/root.go`, `internal/cli/join.go`,
  `internal/cli/capture.go`, `internal/cli/queue.go`,
  `internal/cli/schedule.go`, commits `4831b6a`, `fa8b0a9`, `f1521f3`,
  `faded41`):
  * Binds `--mode`, `--duration`, `--idle-timeout`, `--ai-summary-delay`,
    `--no-ai-summary`, `--lobby-timeout`, `--append-ai-summary`, and
    `--not-before` across capture and schedule commands; validates effective
    CLI lifecycle overrides (`validateCaptureLifecycles`); implements the Idea
    2C hybrid delayed AI Summary workflow with immediate browser/profile
    release, inter-process queue locking (`withQueueFileLock`), and
    tokenizer-compatible argument quoting (`quoteQueueArg`).
* `022.010-T` (`internal/cli/capture_test.go`, `internal/cli/queue_test.go`,
  `internal/cli/schedule_test.go`, commits `ca9874d`, `f1521f3`, `faded41`):
  * Unit and integration tests for capture protocols, Idea 2C timer + queue
    fallback, `--not-before` gating, Windows trailing-backslash quoting, and
    `schedule` per-meeting overrides and `busy` gate release.
* `022.011-T` (`README.md`, `docs/ARCHITECTURE.md`, `config.example.yaml`,
  commit `59f99aa`):
  * Documents scheduled capture (`cron` + `at`), capture protocols, delayed AI
    summary enrichment, and Windows Task Scheduler (`Register-ScheduledTask` /
    `schtasks`) recipes.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`,
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, `mode: report-only`
posture, permanent P-012 Item 5 contract under `008-DL` / `019-F`), followed by
GitHub Copilot shadow review on PR #55 (`P-018` `copilot-review` gate
`SATISFIED`).

| Area | Finding | Disposition |
|---|---|---|
| Correctness (Copilot thread `PRRT_kwDOTStvas63aa1d`) | `ForceRecap` on a live `/meet/` or `/l/meetup-join/` URL without `lobby_timeout` could wait indefinitely on pre-join | Fixed in-scope (P-021 C1): added `deriveRecapURLFromMeetingURL` and bounded 2-minute default timeout (`defaultRecapEnrichmentTimeout` / `defaultForceRecapTimeout`); commit `f1521f3` |
| Correctness (Copilot thread `PRRT_kwDOTStvas63aa1q`) | Negative CLI duration flags (such as `--duration=-1m`) bypassed `Config.Validate` after CLI flag application | Fixed in-scope (P-021 C1): added `validateCaptureLifecycles` in `runCaptureWithHooks` and `newScheduleCommand`; commit `f1521f3` |
| Reliability (Copilot thread `PRRT_kwDOTStvas63aa1w`) | `resolveDeferredQueuePath` returned empty when `queue_file` was unset and `transcription_queue.txt` did not yet exist | Fixed in-scope (P-021 C1): returns `defaultQueueFileName` (`transcription_queue.txt`) so `appendDeferredAISummaryQueueLineWithConfig` creates it on demand; commit `f1521f3` |
| Correctness (Copilot thread `PRRT_kwDOTStvas63aa13`) | `capture_mode: ephemeral-live` in YAML did not default `CaptureAudio = true` unless `--mode` was passed on the CLI | Fixed in-scope (P-021 C1): tracked `ExplicitCaptureAudio` in `config.Load` and CLI flag handlers so `ephemeral-live` enables audio by default while honoring explicit opt-outs; commit `f1521f3` |
| Reliability (Copilot thread `PRRT_kwDOTStvas63aa2E`) | Deferred `--append-ai-summary` queue lines omitted `--config`, `--profile-dir`, `--browser-path`, `--account`, `--lobby-timeout`, and `--headless` | Fixed in-scope (P-021 C1): persisted replay flags in `appendDeferredAISummaryQueueLineWithConfig`; commit `f1521f3` |
| Concurrency (Copilot thread `PRRT_kwDOTStvas63anIF`) | Process-local `queueFileMu` alone did not serialize concurrent CLI processes updating the same queue file | Fixed in-scope (P-021 C1): added inter-process lock file acquisition (`withQueueFileLock` on `<queuePath>.lock`) across all queue read-modify-write helpers; commit `faded41` |
| Correctness (Copilot thread `PRRT_kwDOTStvas63anIV`) | Go `%q` escaping doubled backslashes and produced an unterminated quote in `splitQueueArgs` for Windows paths ending in `\` | Fixed in-scope (P-021 C1): added `quoteQueueArg` with regression coverage for `C:\Profiles\Alice\`; commit `faded41` |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

### Review record

| Field | Value |
|---|---|
| Reviewed diff identity | `main..faded416dc38f3a27461fa19bda0e2f0583db7ef` (PR #55) |
| Reviewed HEAD | `faded416dc38f3a27461fa19bda0e2f0583db7ef` |
| `dispatch_mode` | `single-agent-declared-degradation` |
| `confidence_tier` | `DEGRADED_SINGLE_AGENT` |
| `consensus_status` | `DEGRADED_SINGLE_AGENT (non-consensus)` |
| P-018 `copilot-review` gate | `SATISFIED` (`exit_code: 0`, `unresolved_thread_ids: []`) |

## Follow-ups

* Stashed follow-up **`3EB89E2D`** (`task`, `low`): Live-tenant validation of
  scheduled meeting join, overtime continuation (`PCMRMS` voice activity and
  interim caption heartbeats), in-browser call-end silence shutdown, and Idea 2C
  delayed AI summary Recap enrichment (`--ai-summary-delay` timer and
  `--append-ai-summary --not-before` queue replay) against an authorized
  Microsoft Teams tenant.
* Previously stashed low-priority items (`70ABBB2C`, `D3A9CBD0`, `EC6D1CAF`) and
  queued live verification shipment `010-S` remain untouched.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #55, merge commit `7b6c31d98ab00f1aaea759e87f5c073936a53dbc`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED`; `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Close path | P-015 verified fully-covered-root cascade (`022-F` is a root feature, and all 11 child tasks `022.001-T`..`022.011-T` are manifest members) |
| Command | `backlogit shipment ship 022-S --sha 7b6c31d98ab00f1aaea759e87f5c073936a53dbc` |
| `archived_ids` | `022.001-T`..`022.011-T`, `022-F`, `022-S` |
| `returned_ids` | empty |
| `parent_id` preservation | `022.001-T`..`022.011-T` all retained `parent_id: 022-F` |
| Final records | `022-S`: `archived_status: shipped`; `022-F` and `022.001-T`..`022.011-T`: `archived_status: done` |
| Archive integrity (P-007) | No archive file reported as deleted |
| Topology gate | `lifecycle` passed before the close (`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stash `77FC68CA` | Already archived by Stage at harvest; verified absent from active `backlogit stash list` | skipped |
| Deliberation `011-DL` | Zero remaining live references in `.backlogit/queue/`; archived via `backlogit archive 011-DL` | archived |

### Compaction (P-020)

`compact-context` ran at closure. It wrote
`docs/memory/compacted/2026-11-13-022-S-scheduled-live-call-capture-compacted.md`
and archived the verbose session memory records to
`docs/archive/memory/2026-11-13/022-S-scheduled-live-call-capture-memory.md`
and `docs/archive/memory/2026-11-13/stage-021-S-022-S-memory.md`.
`compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| CLI probe (`cli-help`) | `go run ./cmd/teams-cli --help` — lists `auth`, `captions`, `join`, `schedule`, `update` and new lifecycle/protocol flags; exit 0 | PASS |
| CLI build (`cli-build`) | `go build ./...` — exit 0 | PASS |
| Background-job probe (`scheduler-dry-run`) | `go run ./cmd/teams-cli schedule --help` — renders usage and `--mode`, `--duration`, `--idle-timeout`, `--ai-summary-delay`, `--lobby-timeout` flags without launching a browser; exit 0 | PASS |
| Static analysis & lint | `go vet ./...`, `go vet -tags=integration ./...`, and `golangci-lint run` (GitHub Actions `ubuntu-latest` run `43571245461`) | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Full test suite & coverage | `go test -race -cover ./...` (local Windows + GitHub Actions `ubuntu-latest` run `43571245461`): `version` 97.8%, `teams` 93.8%, `scheduler` 93.5%, `transcript` 91.2%, `audio` 87.5%, `config` 86.9%, `capture` 84.8%, `updater` 84.5%, `cli` 80.0%, `browser` 71.6% | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` ("No issues found.") | PASS |
| Manual checkpoint (`scheduled-meeting-join` & `teams-auth-mfa`) | Live Microsoft Teams scheduled meeting join, overtime continuation, in-browser call-end silence shutdown, and post-call Recap AI summary enrichment against a real tenant require an active scheduled meeting and authenticated profile. Hermetic unit and synthetic Chromium tests verify the watchdog, RMS voice detector, interim caption heartbeats, one-shot `at` triggers, and Idea 2C timer + queue fallback | PENDING (operator live-tenant observation window; tracked by dedicated follow-up stash `3EB89E2D` alongside `010-S`) |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Append delayed AI Summary to existing `.md` transcript via `capture.AppendAISummaryToFile` (`os.O_WRONLY\|os.O_APPEND`, `0o600`) and update `transcription_queue.txt` under inter-process lock `withQueueFileLock` | medium | Verifies file exists via `os.ReadFile` before opening with `O_APPEND`; never truncates existing live captions; checks idempotency marker (`# AI Summary`); serializes queue file read-modify-write via `<queuePath>.lock` | applied and verified |
| Manage Chromium process lifecycle and shared `UserDataDir` profile `busy` gate across nominal `duration`, overtime, and `ai_summary_delay` timer in `internal/capture` and `internal/cli` | medium | Inner closure `defer cleanup()` closes Chromium immediately after `Session.Run`; `onLiveCallFinished` releases `busy` gate before `ai_summary_delay` sleep and re-acquires it only for the bounded Recap fetch | applied and verified |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Healthy signal | READY | `go build ./...` and `go test -race ./...` pass locally and in CI |
| Failure signal | READY | Non-zero exit from CLI root or `schedule` daemon, `ErrLobbyTimeout` log when pre-join/lobby exceeds `lobby_timeout`, or `cli: delayed AI summary fetch did not succeed; entry remains in queue file` warning when Recap is not yet ready |
| Monitoring plan | READY | Monitor `schedule: daemon started`, `capture: nominal duration and idle timeout elapsed; stopping capture`, `browser: detected in-browser call-end signal`, `cli: appended delayed AI summary to transcript`, and uncommented entries in `transcription_queue.txt` |
| Rollback trigger | READY | Scheduler daemon crash-loops, premature call termination during active speech, or corrupted queue lines |
| Rollback procedure | READY | Stop the `schedule` daemon (or disable the OS scheduled task) and revert merge commit `7b6c31d98ab00f1aaea759e87f5c073936a53dbc` on `main` |
| Owner | READY | Operator supervising scheduled/unattended Teams capture |
| Validation window | READY_WITH_CONDITIONS | One full scheduled-meeting cycle (including overtime and 30-minute delayed AI summary enrichment) after release |

## Dark Mode Execution Summary (P-017)

* **Routing status**: `ROUTING_DEGRADED: Ship invocation on gpt-6.1-terra/openai refused git push to origin — falling back to gemini-4-argon/google (high, long_context)`
* **Gates passed**:
  * `pipeline-topology` (`pre_claim`, `post_claim`, `lifecycle`) — `exit_code: 0`
  * Local Review Readiness (`P-014`) — `LOCAL_REVIEW_READY` for HEAD `faded416dc38f3a27461fa19bda0e2f0583db7ef` (`READY`, `0 P0, 0 P1`)
  * Copilot Review Completion Gate (`P-018`) — `SATISFIED` (`exit_code: 0`, all 7 review threads across 2 review rounds fixed in-scope, replied, and resolved)
  * Merge Strategy (`P-009`) — merge commit `7b6c31d98ab00f1aaea759e87f5c073936a53dbc` (`DARK_MODE_MERGE_AUTHORIZED`, normal `gh pr merge 55 --merge` succeeded without admin fallback)
  * Context Compaction (`P-020`) — `compaction_status: done`

## Handoff

Related [decision](../decisions/2026-11-13-scheduled-live-call-capture-deliberation.md)
(`011-DL`), [product spec](../product-specs/2026-11-13-scheduled-live-call-capture-requirements.md)
(`R1`–`R15`), [plan](../plans/2026-11-13-scheduled-live-call-capture-plan.md),
and graduated [capture design doc](../design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md).
Compaction (P-020) completed at post-merge closure; see the compacted
[memory summary](../memory/compacted/2026-11-13-022-S-scheduled-live-call-capture-compacted.md).
