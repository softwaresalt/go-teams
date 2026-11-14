---
title: Scheduled and protocol-driven live call capture decision
description: Deliberation on implementing R1-R15 scheduled live call capture, activity-aware duration/overtime/idle lifecycle, capture modes, and Idea 2C hybrid delayed AI summary enrichment
topic: Scheduled & Protocol-Driven Live Call Capture (77FC68CA, R1-R15)
depth: deep
decision_status: decided
promoted_to: both
deliberation_id: 011-DL
feature: 022-F
shipment: 022-S
source_stash:
  - 77FC68CA
source_spec: docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md
linked_artifacts:
  - docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md
  - docs/plans/2026-11-13-scheduled-live-call-capture-plan.md
  - .backlogit/queue/011-DL.md
  - .backlogit/queue/022-F.md
  - .backlogit/queue/022-S.md
date: 2026-11-13
status: accepted
tags:
  - schedule
  - capture
  - browser
  - cli
  - config
  - audio
  - ai-summary
---

## Problem Frame

Stash entry **`77FC68CA`** (`feature`, `medium`) and the approved brainstorm
requirements specification
[2026-11-13-scheduled-live-call-capture-requirements.md](../product-specs/2026-11-13-scheduled-live-call-capture-requirements.md)
define 15 requirements (`R1`–`R15`) to make `teams-cli` capable of unattended
scheduled and protocol-driven live call capture:

1. **Activity-aware bounded live capture (`R1`, `R2`, `R3`, `R11`, `R12`)**:
   * Currently, `scheduleJob` in `internal/cli/schedule.go` passes the daemon's
     root `cmd.Context()` into `runCapture`, and `capture.Session.Run` blocks
     until its context is cancelled. A scheduled live call never exits on its
     own, permanently holding the single Chromium `UserDataDir` profile and
     keeping the `busy` atomic gate set to `true` (`schedule: skipped, profile busy`).
   * Conversely, a blind hard cutoff at `--duration` would sever meetings that
     run overtime. `--duration` / `duration` must act as the **nominal baseline
     window**: once `duration` elapses, capture continues into overtime as long
     as live caption DOM mutations/utterances (`R11`) or PCM voice audio energy
     above the noise floor (`R11`) is observed, and shuts down cleanly once
     either (a) in-browser call-end teardown (`R3`) is observed and activity
     ceases, or (b) `duration` has elapsed and neither caption nor voice
     activity has occurred for `--idle-timeout` / `idle_timeout` (default `3m`,
     `R12`).
2. **Per-meeting configuration, fallback meeting title, and one-shot `at` triggers (`R4`–`R9`)**:
   * `config.MeetingSchedule` must support per-meeting `mode`, `duration`,
     `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `category`,
     `capture_audio` / `audio`, and `output_dir` (`R4`), plus one-shot `at:`
     timestamps (RFC 3339 or local `YYYY-MM-DD HH:MM`) alongside recurring
     5-field `cron:` expressions (`R7`, `R8`).
   * `scheduleJob` must forward `e.Name` into `capture.SessionConfig.MeetingTitle`
     so `DefaultFilenameTemplate` (`<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md`)
     and the Markdown header use the configured meeting name when the live DOM
     does not supply a richer title (`R5`).
   * `teams-cli schedule` must bind `--audio` / `--recap-audio`, `--headless`,
     `--headed`, `--duration`, `--idle-timeout`, `--mode`,
     `--ai-summary-delay`, `--no-ai-summary`, and `--lobby-timeout` (`R6`).
   * Pre-join and lobby admission waits must honor a configurable
     `--lobby-timeout` / `lobby_timeout` (`R9`) and release the browser process
     and profile lock cleanly on timeout.
3. **Protocol-driven capture modes, Idea 2C hybrid delayed AI Summary, and batch queue catch-up (`R13`, `R14`, `R15`)**:
   * Support four meeting capture protocols (`--mode` / `capture_mode` /
     `meetings[].mode`, `R13`):
     - `ephemeral-live`: real-time caption + MP3 audio capture (`capture_audio`
       defaults to `true` unless explicitly set to `false`) with zero post-call
       AI summary wait (`ai_summary_delay: 0`).
     - `live-with-summary`: real-time caption + optional audio capture,
       immediate flush of `.md` and `.mp3` and browser close on call end (freeing
       the Chromium profile), crash-safe queue persistence
       (`"<url>" --append-ai-summary "<transcript.md>" --not-before <RFC3339>`
       in `transcription_queue.txt` / `queue_file` when configured), and an
       in-process timer (`--ai-summary-delay`, default `30m`, disableable via
       `0` / `--no-ai-summary`) that re-opens the meeting in Recap mode,
       appends `# AI Summary` / `## Meeting notes` to the existing `.md`
       transcript without overwriting live captions, and marks the queue line
       `::` (`R14`).
     - `past-recap`: finite post-meeting Recap export (transcript + AI Summary +
       optional MP3), including batch `--queue` execution into configured
       `output_dir` (`R15`).
     - `auto` (default): selects `past-recap` for Recap URLs and
       `live-with-summary` (when `ai_summary_delay > 0`) or `ephemeral-live` for
       live join URLs.
4. **Operator documentation (`R10`)**:
   * Update `README.md`, `docs/ARCHITECTURE.md`, and `config.example.yaml` with
     `teams-cli schedule` (`cron` + `at`), capture modes, delayed AI summary,
     and Windows Task Scheduler (`Register-ScheduledTask` / `schtasks`) recipes.

## P-021 C5 and C6 Triage Record

* `77FC68CA` does not carry the `DEFERRED SCOPE EXPANSION` marker.
* **Obligation (A) Unconditional duplicate detection**: Executed across
  `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`.
  Outcome: **CLEAN DUPLICATE SCAN** for `77FC68CA`.

## Resolution of Deferred Brainstorm Items (`D1`, `D2`, `D3`)

### D1: DOM selectors and logic for in-browser call-end detection (`R3`)

We evaluate two approaches for detecting when a live Teams call has ended
inside `internal/browser`:

* **Option D1-A (Single text match only)**: Check only for `"Call ended"` in
  `document.body.innerText`. Rejected because participant chat or captions can
  contain those words, causing false-positive early termination.
* **Option D1-B (Composite post-admission toolbar-loss + explicit call-end selector probe — chosen)**:
  In `internal/browser/prejoin.go` and `internal/browser/meeting_chrome.go`,
  evaluate `callEndedJS` only *after* `Join` has confirmed admission (the active
  call toolbar `#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]`
  was present). Report call-ended when either:
  1. An explicit Teams v2 post-call / ended-call container selector is present
     (`[data-tid="call-ended-screen"]`, `[data-tid="post-call-screen"]`,
     `[data-tid="rejoin-call-button"]`, `[data-tid="call-quality-rating"]`) and
     the active hangup button (`#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]`)
     is absent, OR
  2. The active hangup button is absent for 3 consecutive 2-second probes
     (`6s` total) while not on a Meeting Recap view.
  Once `OnCallEnded` triggers in `capture.Session`, the session transitions to
  post-teardown silence verification and cancels `runCtx` once caption and
  voice activity remain quiet.

### D2: PCM voice-activity threshold and interim caption mutation heartbeats (`R11`, `R12`)

* **PCM Voice Activity Detection (`internal/browser/audio.go`)**:
  - `decodeAudioPayload` already decodes each browser audio chunk into mono
    16-bit signed PCM samples (`[]int16` in `[-32768, 32767]`).
  - We add `PCMRMS(samples []int16) float64` (root-mean-square amplitude) and
    `HasVoiceActivity(samples []int16, threshold float64) bool` with
    `DefaultVoiceRMSThreshold = 300.0` (~`-40.8 dBFS`). Silence and browser
    WebRTC comfort noise stay well below `50` RMS, whereas normal conversational
    speech sits between `800` and `8000` RMS. Whenever a chunk's RMS meets or
    exceeds `DefaultVoiceRMSThreshold`, `CaptureAudio` invokes the session's
    `onAudioActivity` callback.
* **Interim Caption Mutation Heartbeats (`internal/browser/captions.go`)**:
  - Currently, `captionObserverJS` emits `sendCaptionToGo` only when a block
    ends with `[.!?]` or after a 1500ms settle timer. During rapid unpunctuated
    speech, the settle timer keeps resetting.
  - We extend `captionObserverJS` to emit a throttled interim activity payload
    (`{"speaker": speaker, "text": text, "interim": true}`) at most once per
    `1000ms` while non-empty caption text is mutating, and update
    `handleCaptionPayloadWithActivity` so `onCaptionActivity` fires on any
    valid non-empty payload (including `interim: true`), while only settled /
    punctuated payloads are passed to `dedup` and written to the Markdown
    transcript.

### D3: Leaf-package boundary preservation & width-isolated task decomposition

* **Preserving `docs/ARCHITECTURE.md` leaf-package invariant**:
  - `internal/config` and `internal/scheduler` are leaf packages and MUST NOT
    import `internal/teams`, `internal/browser`, or `internal/capture`.
  - In `internal/scheduler`, `Entry.Validate()` validates URL structure
    (`http`/`https` scheme and non-empty host, plus an optional injectable
    `ValidateURL func(string) error`), while `internal/cli/schedule.go` passes
    `teams.ValidateMeetingURL`.
* **Width-isolated 11-task decomposition under `022-F` / `022-S`**:
  1. `022.001-T` (`code`, 2 files: `internal/config/config.go`,
     `internal/config/config_test.go`): Config model, aliases, per-meeting
     fields, and validation (`R1`, `R2`, `R4`, `R7`, `R9`, `R12`, `R13`, `R14`,
     `R15`).
  2. `022.002-T` (`code`, 2 files: `internal/scheduler/schedule.go`,
     `internal/scheduler/schedule_test.go`): Cron + one-shot `at` scheduler
     with `Stop()` lifecycle and validation (`R7`, `R8`).
  3. `022.003-T` (`code`, 3 files: `internal/browser/prejoin.go`,
     `internal/browser/captions.go`, `internal/browser/audio.go`): Call-end DOM
     probe JS, interim caption activity heartbeats, and PCM RMS voice-activity
     detection (`R3`, `R11`, `R12`).
  4. `022.004-T` (`code`, 2 files: `internal/browser/meeting.go`,
     `internal/browser/meeting_chrome.go`): `LobbyTimeout` enforcement,
     `SetActivityCallbacks` / `MeetingOptions` callbacks, and live call-end
     polling in `chromeMeeting` (`R3`, `R9`, `R11`, `R12`).
  5. `022.005-T` (`tests`, 3 files: `internal/browser/audio_test.go`,
     `internal/browser/captions_test.go`, `internal/browser/prejoin_test.go`):
     Browser unit tests for `PCMRMS`/`HasVoiceActivity`, interim caption
     heartbeats, `callEndedJS`, and `LobbyTimeout`.
  6. `022.006-T` (`code`, 2 files: `internal/capture/session.go`,
     `internal/capture/output.go`): Activity-aware nominal duration, overtime
     continuation, idle-timeout shutdown, fallback `MeetingTitle`, and
     `AppendAISummaryToFile` (`R1`, `R2`, `R3`, `R5`, `R11`, `R12`, `R13`,
     `R14`).
  7. `022.007-T` (`tests`, 1 file: `internal/capture/session_test.go`):
     Deterministic unit tests for nominal duration, overtime continuation,
     call-end early exit, `MeetingTitle` fallback, and `AppendAISummaryToFile`.
  8. `022.008-T` (`code`, 3 files: `internal/cli/root.go`,
     `internal/cli/join.go`, `internal/cli/capture.go`): CLI flags (`--mode`,
     `--duration`, `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`,
     `--lobby-timeout`, `--append-ai-summary`, `--not-before`), mode resolution,
     and Idea 2C hybrid delayed AI Summary timer + queue fallback (`R1`, `R9`,
     `R12`, `R13`, `R14`).
  9. `022.009-T` (`code`, 2 files: `internal/cli/queue.go`,
     `internal/cli/schedule.go`): Batch queue `--append-ai-summary` /
     `--not-before` execution and `schedule` command flag binding, per-meeting
     overrides, and profile `busy`-gate release during `ai_summary_delay` (`R2`,
     `R4`–`R8`, `R14`, `R15`).
  10. `022.010-T` (`tests`, 3 files: `internal/cli/capture_test.go`,
      `internal/cli/queue_test.go`, `internal/cli/schedule_test.go`): CLI unit
      and integration tests for capture modes, Idea 2C timer + queue fallback,
      `--not-before` gating, and `schedule` overrides.
  11. `022.011-T` (`docs`, 3 files: `README.md`, `docs/ARCHITECTURE.md`,
      `config.example.yaml`): Operator documentation and Windows Task Scheduler
      recipes (`R10`).

## Options Evaluated

### Option A: Blind hard context timeout at `--duration` + in-session browser wait for AI summary

Wrap `runCapture` in `context.WithTimeout(ctx, duration)` and keep Chromium open
for 30 minutes after the call ends to poll for the AI Summary.

* **Pros**: Trivial implementation.
* **Cons**: Violates `R11` (cuts off live meetings that run overtime), violates
  `R14` (holds the single Chromium `UserDataDir` profile lock during the 30m AI
  summary wait, blocking any back-to-back scheduled meeting at `:30` or `:00`),
  and loses the AI summary if the CLI process exits during the 30m wait.
* **Effort**: low | **Fit**: low.

### Option B: Activity-extended nominal duration + Idea 2C hybrid timer & crash-safe queue fallback (chosen)

Implement the full protocol-driven lifecycle (`R1`–`R15`) decomposed into 11
width-isolated tasks under `022-F` / `022-S`, chained after `021-S` (`022-S blocks 021-S`).

* **Pros**: Never cuts off an active overtime call (`R11`), shuts down cleanly
  on call-end or post-duration silence (`R3`, `R12`), releases the Chromium
  profile immediately when the live call ends (`R14`), persists a crash-safe
  `--append-ai-summary <md> --not-before <RFC3339>` queue entry (`R14`, `R15`),
  and supports both daemon (`teams-cli schedule` with `cron` + `at`) and OS
  scheduler (`teams-cli join --duration`) workflows.
* **Cons**: Spans 6 packages; mitigated by strict 11-task width isolation (`<= 3`
  files per task) and sequencing `022-S` after `021-S` so shared files
  (`internal/config/config.go`, `internal/browser/meeting.go`,
  `internal/cli/root.go`, `internal/cli/join.go`, `README.md`,
  `docs/ARCHITECTURE.md`) merge cleanly without conflicts.
* **Effort**: medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (Hard cutoff + hold browser open for AI summary) | Option B (Activity-aware overtime + Idea 2C hybrid timer & queue fallback) |
|---|---|---|
| Overtime meeting preservation (`R11`, `R12`) | Fails (cuts off active speakers at `duration`) | Full (extends while caption or PCM voice activity persists; exits after `idle_timeout` silence) |
| Back-to-back scheduled meetings (`R2`, `R14`) | Fails (profile locked during 30m AI summary wait) | Full (browser closed & profile `busy` gate released immediately after live call) |
| Crash resilience during 30m AI summary wait (`R14`) | None (lost on exit) | Full (`--append-ai-summary <md> --not-before <RFC3339>` written to queue file and marked `::` on completion) |
| Leaf-package architecture compliance | Partial | Full (`internal/config` and `internal/scheduler` remain pure leaf packages) |

## Decision

We select **Option B** and decompose the work into covering feature **`022-F`**
and shipment **`022-S`** (chained after `021-S` via
`backlogit dep add 022-S 021-S --type blocks`) with 11 width-isolated tasks
(`022.001-T` through `022.011-T`).

## Unresolved Questions

None blocking planning or harvest.
