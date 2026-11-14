---
title: Scheduled and protocol-driven live call capture implementation plan
description: Implementation plan for 022-F / 022-S implementing R1-R15 scheduled live call capture, activity-aware duration/overtime/idle lifecycle, capture modes, and Idea 2C hybrid delayed AI summary enrichment
source: docs/decisions/2026-11-13-scheduled-live-call-capture-deliberation.md
spec: docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md
deliberation_id: 011-DL
feature: 022-F
shipment: 022-S
source_stash:
  - 77FC68CA
requirement_ids:
  - R1
  - R2
  - R3
  - R4
  - R5
  - R6
  - R7
  - R8
  - R9
  - R10
  - R11
  - R12
  - R13
  - R14
  - R15
date: 2026-11-13
status: approved
tags:
  - schedule
  - capture
  - browser
  - cli
  - config
  - audio
  - ai-summary
  - tests
  - docs
---

## Problem Frame

Following the accepted deliberation in
[2026-11-13-scheduled-live-call-capture-deliberation.md](../decisions/2026-11-13-scheduled-live-call-capture-deliberation.md)
(`011-DL`) and the approved product specification
[2026-11-13-scheduled-live-call-capture-requirements.md](../product-specs/2026-11-13-scheduled-live-call-capture-requirements.md)
(`R1`–`R15`), stash entry **`77FC68CA`** (`feature`, `medium`) is planned under
covering feature **`022-F`** and shipment **`022-S`** (chained after `021-S` via
`022-S blocks 021-S`).

This plan resolves the unbounded live-capture hang in `teams-cli schedule`,
enables bounded one-shot `teams-cli join --duration` automation for OS task
schedulers, preserves live calls that run overtime via caption and PCM voice
activity monitoring (`R11`, `R12`), detects in-browser call teardown (`R3`),
enriches `config.MeetingSchedule` and `internal/scheduler` with per-meeting
overrides, fallback title propagation, and one-shot `at:` timestamps (`R4`–`R8`),
bounds lobby waits (`R9`), introduces protocol-driven capture modes
(`ephemeral-live`, `live-with-summary`, `past-recap`, `auto`, `R13`), and
implements the Idea 2C hybrid in-process timer + crash-safe
`transcription_queue.txt` (`--append-ai-summary <transcript.md> --not-before <RFC3339>`)
workflow for delayed post-call AI Summary capture (`R14`, `R15`).

## Requirements Trace

| Requirement | Description | Implementation unit(s) | Verification |
|---|---|---|---|
| R1 | `--duration` flag on `join`, `captions`, and root + `duration` in `config.yaml` defining the nominal capture window and clean `.md`/`.mp3` flush | U1 (`022.001-T`), U6 (`022.006-T`), U8 (`022.008-T`) | `go test -race ./internal/config/... ./internal/capture/... ./internal/cli/...` |
| R2 | `teams-cli schedule` runs every scheduled meeting with a nominal `duration` (`meetings[].duration` -> `config.duration` -> default `1h`) + post-duration inactivity detection (`R11`, `R12`) | U1 (`022.001-T`), U6 (`022.006-T`), U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/cli/...` |
| R3 (`D1`) | In-browser live call-end / stage teardown detection in `internal/browser` canceling active capture once caption/voice activity ceases | U3 (`022.003-T`), U4 (`022.004-T`), U5 (`022.005-T`), U6 (`022.006-T`), U7 (`022.007-T`) | `go test -race ./internal/browser/... ./internal/capture/...` |
| R4 | Per-meeting `config.MeetingSchedule` fields: `mode`, `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `category`, `capture_audio`/`audio`, `output_dir` | U1 (`022.001-T`), U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/config/... ./internal/cli/...` |
| R5 | Forward `MeetingSchedule.Name` into `capture.SessionConfig.MeetingTitle` as fallback meeting title for `<concat-meeting-name>` and Markdown header | U6 (`022.006-T`), U7 (`022.007-T`), U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/capture/... ./internal/cli/...` |
| R6 | Bind `--audio` / `--recap-audio`, `--headless`, `--headed`, `--duration`, `--idle-timeout`, `--mode`, `--ai-summary-delay`, `--no-ai-summary`, `--lobby-timeout` on `teams-cli schedule` | U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/cli/...` |
| R7 | Support one-shot `at:` timestamps (RFC 3339 or local `YYYY-MM-DD HH:MM`) in `config.MeetingSchedule` and `internal/scheduler`, skipping past timestamps on startup with structured log | U1 (`022.001-T`), U2 (`022.002-T`), U9 (`022.009-T`) | `go test -race ./internal/scheduler/... ./internal/cli/...` |
| R8 | `scheduler.Entry.Validate` requires exactly one trigger (`cron` or `at`), a valid URL (paired with `teams.ValidateMeetingURL` in `internal/cli`), and non-negative `duration` | U2 (`022.002-T`), U9 (`022.009-T`) | `go test -race ./internal/scheduler/... ./internal/cli/...` |
| R9 | Configurable pre-join/lobby wait timeout (`--lobby-timeout` / `lobby_timeout` / `meetings[].lobby_timeout`) releasing browser and profile lock cleanly on timeout | U1 (`022.001-T`), U4 (`022.004-T`), U5 (`022.005-T`), U8 (`022.008-T`) | `go test -race ./internal/browser/... ./internal/cli/...` |
| R10 | Document `teams-cli schedule` (`cron` + `at`), capture modes, delayed AI summary, and Windows Task Scheduler (`Register-ScheduledTask` / `schtasks`) recipes in `README.md`, `docs/ARCHITECTURE.md`, and `config.example.yaml` | U11 (`022.011-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| R11 (`D2`) | Overtime continuation past nominal `duration` while live caption DOM mutations (including throttled 1000ms interim heartbeats) or PCM voice audio energy (`PCMRMS >= 300.0`) persists | U3 (`022.003-T`), U4 (`022.004-T`), U5 (`022.005-T`), U6 (`022.006-T`), U7 (`022.007-T`) | `go test -race ./internal/browser/... ./internal/capture/...` |
| R12 | Clean shutdown and `.md`/`.mp3` flush once `duration` (or call-end teardown) has elapsed and neither caption nor voice activity is detected for `idle_timeout` (default `3m`) | U1 (`022.001-T`), U6 (`022.006-T`), U7 (`022.007-T`), U8 (`022.008-T`) | `go test -race ./internal/capture/... ./internal/cli/...` |
| R13 | Capture mode classification (`--mode` / `capture_mode` / `meetings[].mode`: `ephemeral-live`, `live-with-summary`, `past-recap`, `auto`) | U1 (`022.001-T`), U6 (`022.006-T`), U8 (`022.008-T`), U10 (`022.010-T`) | `go test -race ./internal/config/... ./internal/cli/...` |
| R14 | Idea 2C hybrid delayed post-call AI Summary capture (`--ai-summary-delay`, default `30m` in-process timer after browser close + crash-safe `transcription_queue.txt` `--append-ai-summary <transcript.md> --not-before <RFC3339>` fallback that appends AI Summary to the existing `.md` file and marks the queue line `::`) | U6 (`022.006-T`), U7 (`022.007-T`), U8 (`022.008-T`), U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/capture/... ./internal/cli/...` |
| R15 | Batch `--queue` catch-up resolving `output_dir` from `config.yaml` or outer CLI flags, supporting per-line `--mode`, `--category`, `--audio`, `--append-ai-summary`, and `--not-before`, and marking succeeded lines `::` in place | U8 (`022.008-T`), U9 (`022.009-T`), U10 (`022.010-T`) | `go test -race ./internal/cli/...` |

## Implementation Units

### U1 (`022.001-T`) — Extend `internal/config` with capture modes, duration/idle/lobby/AI-summary-delay settings, `queue_file`, per-meeting overrides, and one-shot `at` triggers (`R1`, `R2`, `R4`, `R7`, `R9`, `R12`–`R15`)

* **Domain:** code (Go `internal/config`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/config/config.go`:
     * Export capture mode constants:
       `CaptureModeAuto = "auto"`, `CaptureModeEphemeralLive = "ephemeral-live"`,
       `CaptureModeLiveWithSummary = "live-with-summary"`,
       `CaptureModePastRecap = "past-recap"`, and helper
       `IsValidCaptureMode(mode string) bool`.
     * Export sentinel errors:
       `ErrInvalidCaptureMode = errors.New("config: invalid capture mode")`,
       `ErrInvalidDuration = errors.New("config: invalid duration")`,
       `ErrMeetingScheduleConflict = errors.New("config: meeting must specify either cron or at, not both")`.
     * Extend `Config` with:
       - `CaptureMode string \`yaml:"capture_mode,omitempty"\`` (with YAML alias
         `mode` in `Load`, defaulting to `CaptureModeAuto` when unset)
       - `Duration string \`yaml:"duration,omitempty"\``
       - `IdleTimeout string \`yaml:"idle_timeout,omitempty"\``
       - `AISummaryDelay string \`yaml:"ai_summary_delay,omitempty"\``
       - `LobbyTimeout string \`yaml:"lobby_timeout,omitempty"\``
       - `QueueFile string \`yaml:"queue_file,omitempty"\`` (expanded via
         `expandUserPath` when non-empty)
     * Extend `MeetingSchedule` with:
       - `At string \`yaml:"at,omitempty"\``
       - `Mode string \`yaml:"mode,omitempty"\`` (also accepting `capture_mode`
         alias)
       - `Duration string \`yaml:"duration,omitempty"\``
       - `IdleTimeout string \`yaml:"idle_timeout,omitempty"\``
       - `AISummaryDelay string \`yaml:"ai_summary_delay,omitempty"\``
       - `LobbyTimeout string \`yaml:"lobby_timeout,omitempty"\``
       - `Category string \`yaml:"category,omitempty"\``
       - `CaptureAudio *bool \`yaml:"capture_audio,omitempty"\``
       - `Audio *bool \`yaml:"audio,omitempty"\`` (normalized into `CaptureAudio`
         in `Load` when `CaptureAudio == nil && Audio != nil`, clearing `Audio`
         after normalization so `Save` emits canonical `capture_audio`)
       - `OutputDir string \`yaml:"output_dir,omitempty"\`` (expanded via
         `expandUserPath` in `Load`)
     * In `Config.Validate()`:
       - Validate top-level and per-meeting `CaptureMode` / `Mode` (when
         non-empty) via `IsValidCaptureMode` (`ErrInvalidCaptureMode`).
       - Validate top-level and per-meeting `Duration`, `IdleTimeout`, and
         `LobbyTimeout` (when non-empty) parse via `time.ParseDuration` with
         value `> 0` (`ErrInvalidDuration`).
       - Validate top-level and per-meeting `AISummaryDelay` (when non-empty)
         parses via `time.ParseDuration` with value `>= 0`
         (`ErrInvalidDuration`).
       - For each `MeetingSchedule`: require non-empty `URL`
         (`ErrMeetingMissingURL`); require at least one of `Cron` or `At`
         (`ErrMeetingMissingCron`); reject setting both `Cron` and `At`
         (`ErrMeetingScheduleConflict`).
  2. In `internal/config/config_test.go`:
     * Add unit tests covering all new top-level and per-meeting fields,
       aliases (`mode` -> `CaptureMode`, `meetings[].audio` -> `CaptureAudio`),
       path expansion on `QueueFile` and `meetings[].output_dir`, and validation
       errors (`ErrInvalidCaptureMode`, `ErrInvalidDuration`,
       `ErrMeetingMissingCron`, `ErrMeetingScheduleConflict`).
* **Files (2 files):** `internal/config/config.go`,
  `internal/config/config_test.go`.
* **Verify:** `go test -race -cover ./internal/config/...` passes (>80% coverage).
* **Depends on:** none (sequences after `021-S`).

### U2 (`022.002-T`) — Add one-shot `at` timestamp triggers, per-entry overrides, and trigger/URL/duration validation in `internal/scheduler` (`R7`, `R8`)

* **Domain:** code (Go `internal/scheduler`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/scheduler/schedule.go`:
     * Keep `internal/scheduler` a pure leaf package (importing only the Go
       standard library and `github.com/robfig/cron/v3`).
     * Extend `Entry` with:
       - `At string` (RFC 3339 `2006-01-02T15:04:05Z07:00` or local
         `2006-01-02 15:04`)
       - `Mode string`
       - `Duration time.Duration`
       - `IdleTimeout time.Duration`
       - `AISummaryDelay *time.Duration`
       - `LobbyTimeout time.Duration`
       - `Category string`
       - `CaptureAudio *bool`
       - `OutputDir string`
       - `ValidateURL func(string) error` (optional URL validator hook; when
         nil, `Entry.Validate` checks that `e.URL` parses as a valid `http` or
         `https` URL with a non-empty host)
     * Add `ParseAt(raw string, loc *time.Location) (time.Time, error)`
       supporting `time.RFC3339` and `"2006-01-02 15:04"` in `loc` (defaulting
       to `time.Local`).
     * Update `Entry.Validate()` (`R8`) to enforce:
       1. Non-empty `e.URL` passing `e.ValidateURL` (or default `http`/`https`
          URL parse check).
       2. Exactly one trigger: `(strings.TrimSpace(e.Cron) != "") != (strings.TrimSpace(e.At) != "")`.
          When `e.Cron != ""`, validate with `cron.ParseStandard(e.Cron)`; when
          `e.At != ""`, validate with `ParseAt(e.At, time.Local)`.
       3. Non-negative durations (`e.Duration >= 0`, `e.IdleTimeout >= 0`,
          `e.LobbyTimeout >= 0`).
     * Update `Build(entries []Entry, job JobFunc, opts ...cron.Option)` (and
       add `BuildWithClock(entries []Entry, job JobFunc, now func() time.Time, opts ...cron.Option)`)
       so that in addition to registering recurring `e.Cron` entries on `*cron.Cron`,
       one-shot `e.At` entries are scheduled via a `cron.Schedule` adapter
       (`oneShotSchedule{at: targetTime, fired: atomic.Bool}`):
       - If `targetTime.After(now())` is false at `Build` time, log
         `slog.Info("scheduler: skipping past one-shot meeting", "name", e.Name, "at", e.At)`
         and do not register the past entry (returning the `*cron.Cron` instance
         cleanly even when all entries are past one-shot meetings).
       - If `targetTime.After(now())` is true, register the entry with
         `c.Schedule(&oneShotSchedule{at: targetTime}, cron.FuncJob(func() { job(e) }))`
         where `Next(t time.Time) time.Time` returns `targetTime` while
         `!t.After(targetTime)` and `time.Time{}` (zero time, which `robfig/cron`
         treats as never firing again) once `t.After(targetTime)`. This reuses
         `*cron.Cron`'s exact `Start()`, `Stop()`, `SkipIfStillRunning`, and
         in-flight job WaitGroup lifecycle without changing `Build`'s return type!
  2. In `internal/scheduler/schedule_test.go`:
     * Add unit tests covering `ParseAt` (RFC 3339 and local `YYYY-MM-DD HH:MM`),
       `Entry.Validate` (missing URL, invalid URL via `ValidateURL`, both `cron`
       and `at` set, neither set, invalid `at`, negative `Duration`), past
       one-shot `at` entries skipped without firing, and future one-shot `at`
       entries firing exactly once and never repeating.
* **Files (2 files):** `internal/scheduler/schedule.go`,
  `internal/scheduler/schedule_test.go`.
* **Verify:** `go test -race -cover ./internal/scheduler/...` passes.
* **Depends on:** none.

### U3 (`022.003-T`) — Add in-browser call-end DOM probe (`R3`), interim caption mutation heartbeats (`R11`), and PCM RMS voice activity detection (`R11`) in `internal/browser`

* **Domain:** code (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/browser/prejoin.go` (`R3`, `D1`):
     * Add `callEndedScreenSel = \`[data-tid="call-ended-screen"], [data-tid="post-call-screen"], [data-tid="rejoin-call-button"], [data-tid="call-quality-rating"]\``.
     * Add `callEndedProbeJS() string` that evaluates in the active meeting tab
       and returns a JSON/boolean status reporting whether the active hangup
       toolbar (`meetingToolbarSel`) is absent AND either an explicit post-call
       selector (`callEndedScreenSel`) or call-ended banner text
       (`"You left the meeting"`, `"Call ended"`) is present, or whether the
       hangup toolbar alone has disappeared.
  2. In `internal/browser/captions.go` (`R11`, `D2`):
     * Extend `captionPayload` with `Interim bool \`json:"interim,omitempty"\``.
     * Update `captionObserverJS` so that when `inspectBlock` sees a non-empty
       caption mutation that does not yet end with `[.!?]`, in addition to
       arming the 1500ms settle timer it emits a throttled interim activity
       payload `window.sendCaptionToGo(JSON.stringify({ speaker: speaker, text: text, interim: true }))`
       at most once every `1000ms` while caption text is changing.
     * Add `handleCaptionPayloadWithActivity(raw string, dedup *transcript.Deduplicator, sink CaptionSink, onActivity func()) error`
       (and keep `handleCaptionPayload` calling it with `nil`): whenever a valid
       `captionPayload` has non-empty trimmed `Text`, invoke `onActivity()`
       (if non-nil); if `p.Interim` is true, return `nil` immediately without
       passing the un-settled fragment to `dedup` or `sink`.
  3. In `internal/browser/audio.go` (`R11`, `D2`):
     * Export `DefaultVoiceRMSThreshold = 300.0`.
     * Add `PCMRMS(samples []int16) float64` (computing
       `math.Sqrt(sumSquares / float64(len(samples)))`, returning `0` on empty
       slice) and `HasVoiceActivity(samples []int16, threshold float64) bool`
       (using `DefaultVoiceRMSThreshold` when `threshold <= 0`).
* **Files (3 files):** `internal/browser/prejoin.go`,
  `internal/browser/captions.go`, `internal/browser/audio.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** none.

### U4 (`022.004-T`) — Wire `LobbyTimeout` (`R9`), `ForceRecap` (`R14`), activity callbacks (`R11`, `R12`), and live call-end polling (`R3`) in `internal/browser`

* **Domain:** code (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/browser/meeting.go`:
     * Export sentinel error
       `ErrLobbyTimeout = errors.New("browser: lobby admission timed out")`.
     * Extend `MeetingOptions` with:
       - `LobbyTimeout time.Duration`
       - `ForceRecap bool` (`R13`, `R14`: forces `chromeMeeting.Join` to use
         the Meeting Recap discovery/extraction path with
         `ExtractMeetingIdentity` verification and never enter the live
         pre-join click flow)
       - `VoiceRMSThreshold float64`
       - `OnCaptionActivity func()`
       - `OnAudioActivity func()`
       - `OnCallEnded func()`
     * Export optional interface `ActivityMonitor`:
       ```go
       type ActivityMonitor interface {
           SetActivityCallbacks(onCaption, onAudio, onCallEnd func())
       }
       ```
  2. In `internal/browser/meeting_chrome.go`:
     * Implement `SetActivityCallbacks(onCaption, onAudio, onCallEnd func())`
       on `*chromeMeeting` (synchronized under a mutex `activityMu` and falling
       back to `m.opts.OnCaptionActivity`, `m.opts.OnAudioActivity`,
       `m.opts.OnCallEnded`).
     * In `Join(ctx)`:
       - Keep the session-lifetime `ctx` as the parent of
         `chromedp.NewRemoteAllocator` / `chromedp.NewContext` when constructing
         `m.attachedCtx` in `joinHeaded` so `m.attachedCtx` remains alive and
         uncancelled after `Join(ctx)` returns for subsequent `EnableCaptions`
         and `CaptureAudio` calls!
       - Enforce `m.opts.LobbyTimeout` (when `> 0`) via an explicit admission
         deadline (`deadline := time.Now().Add(m.opts.LobbyTimeout)`) and
         bounded per-poll contexts inside the pre-join/admission wait loops
         (`joinHeaded`, `awaitPreJoinOrAdmission`, `awaitAdmission`), returning
         `fmt.Errorf("%w after %s", ErrLobbyTimeout, m.opts.LobbyTimeout)` if
         the deadline expires while `ctx.Err() == nil` (`R9`).
       - When `m.opts.ForceRecap` is `true`, treat the session as Recap-only:
         verify recording identity via `ExtractMeetingIdentity` /
         `retainRecapAudioIdentity`, extract the Recap transcript and
         `AISummary()` without ever clicking live pre-join `Join now`, and
         return an error if no Recap view or AI Summary is available (`R14`).
     * In `EnableCaptions`:
       - Pass `m.notifyCaptionActivity` to `handleCaptionPayloadWithActivity`.
       - Launch a background goroutine tied to `workerCtx` that polls the live
         meeting DOM every `2 * time.Second` via `callEndedProbeJS()`. When an
         explicit call-ended screen is detected or the active hangup toolbar is
         absent for 3 consecutive polls (`6s`), invoke `m.notifyCallEnded()` once
         and log `slog.Info("browser: detected in-browser call-end signal")`
         (`R3`).
     * In `CaptureAudio`:
       - Inside the `audioQueueSize` `payloadPump` worker, after
         `decodeAudioPayload(raw)` succeeds, check
         `HasVoiceActivity(samples, m.opts.VoiceRMSThreshold)` and, when true,
         invoke `m.notifyAudioActivity()` (`R11`).
* **Files (2 files):** `internal/browser/meeting.go`,
  `internal/browser/meeting_chrome.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** U3 (`022.003-T`).

### U5 (`022.005-T`) — Add unit tests for PCM RMS voice activity, interim caption heartbeats, call-end probes, `ForceRecap`, and lobby timeout in `internal/browser`

* **Domain:** tests (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/browser/audio_test.go`:
     * Add `TestPCMRMSAndHasVoiceActivity` covering empty samples (`0`), silence
       (`0`), low comfort noise (`±40` int16 < `300.0` threshold -> `false`),
       and speech signal (`±2000` int16 >= `300.0` threshold -> `true`).
  2. In `internal/browser/captions_test.go`:
     * Add `TestHandleCaptionPayloadWithActivityFiresOnInterimWithoutEmittingToSink`
       proving `interim: true` payloads trigger `onActivity()` without calling
       `sink` or polluting deduplication state, while settled/punctuated
       payloads trigger both `onActivity()` and `sink`.
  3. In `internal/browser/prejoin_test.go`:
     * Add unit tests for `callEndedProbeJS()`, `SetActivityCallbacks` on
       `chromeMeeting`, `MeetingOptions.ForceRecap` bypassing live pre-join,
       and `Join` returning `ErrLobbyTimeout` when `LobbyTimeout` expires before
       admission while keeping `m.attachedCtx` derived from the session-lifetime
       `ctx` uncancelled after successful `Join` return.
* **Files (3 files):** `internal/browser/audio_test.go`,
  `internal/browser/captions_test.go`, `internal/browser/prejoin_test.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** U3 (`022.003-T`), U4 (`022.004-T`).

### U6 (`022.006-T`) — Implement activity-aware nominal duration, overtime continuation, idle-timeout/call-end shutdown, fallback `MeetingTitle`, and `AppendAISummaryToFile` in `internal/capture` (`R1`–`R3`, `R5`, `R11`–`R14`)

* **Domain:** code (Go `internal/capture`). **Posture:** test-first. **Size:** `M` | **Complexity:** `medium`.
* **What:**
  1. In `internal/capture/output.go`:
     * Export `AppendAISummaryToFile(transcriptPath, summary string) error`
       (`R14`):
       - Trims `summary`; if empty, returns `errors.New("capture: ai summary is empty")`.
       - Resolves `absPath, err := filepath.Abs(transcriptPath)` and verifies the
         existing transcript file via `os.ReadFile(absPath)`.
       - If the existing file already contains `"\n# AI Summary\n"` or
         `"\n## AI Summary\n"`, returns `nil` (idempotent no-op).
       - Opens `absPath` with `os.OpenFile(absPath, os.O_WRONLY|os.O_APPEND, filePerm)`,
         writes the formatted summary via `transcript.NewWriter(f).WriteAISummary(summary)`,
         flushes, syncs, and closes the file without modifying any preceding
         live transcript content.
  2. In `internal/capture/session.go`:
     * Export `DefaultIdleTimeout = 3 * time.Minute` and
       `DefaultPollInterval = 250 * time.Millisecond`.
     * Extend `SessionConfig` with:
       - `MeetingTitle string` (`R5` fallback meeting title when live DOM does
         not supply a richer title)
       - `Mode string` (`R13`: `auto`, `ephemeral-live`, `live-with-summary`,
         `past-recap`)
       - `Duration time.Duration` (`R1`, `R2` nominal capture window)
       - `IdleTimeout time.Duration` (`R12` post-duration / post-teardown
         inactivity window; defaults to `DefaultIdleTimeout` when `Duration > 0`
         and `IdleTimeout <= 0`)
       - `PollInterval time.Duration` (optional watchdog check interval for
         deterministic fast tests; defaults to `DefaultPollInterval`)
     * Add `LastTranscriptPath() string` and `WasRecap() bool` on `*Session` so
       `internal/cli` can inspect the resolved `.md` output path and whether the
       session ran as a finite Recap export or a live call.
     * Add `RecordCaptionActivity()`, `RecordAudioActivity()`, and
       `RecordCallEnded()` on `*Session` (and automatically register them via
       `am.SetActivityCallbacks(s.RecordCaptionActivity, s.RecordAudioActivity, s.RecordCallEnded)`
       when `m` implements `browser.ActivityMonitor`).
     * In `Session.Run`:
       - Set `outCfg.MeetingTitle = s.cfg.MeetingTitle` before checking
         `browser.MetadataProvider`; if `MetadataProvider` returns a non-empty
         `meta.Title`, it overrides the fallback (`R5`).
       - Also update `s.RecordCaptionActivity()` inside `sink` whenever a live
         caption event arrives.
       - For live calls (`!isRecap`): if `s.cfg.Duration > 0`,
         `s.cfg.IdleTimeout > 0`, or `RecordCallEnded` is triggered, run an
         activity-aware watchdog goroutine tied to `runCtx`:
         1. Initialize `startTime := s.now()` and `lastActivity := startTime`.
         2. Before `Duration` elapses (or when `Duration == 0`): if `callEnded`
            has been signaled (`R3`), cancel `runCtx` as soon as
            `s.now().Sub(maxTime(callEndedAt, lastActivity)) >= effectiveIdleTimeout`
            (where `effectiveIdleTimeout` defaults to `DefaultIdleTimeout` =
            `3 * time.Minute` per `R12` when `s.cfg.IdleTimeout <= 0`, or uses
            `s.cfg.IdleTimeout` when explicitly configured).
         3. Once `s.cfg.Duration > 0` and `s.now().Sub(startTime) >= s.cfg.Duration`
            (`R1`, `R2`, `R11`, `R12`):
            - If `s.now().Sub(lastActivity) < effectiveIdleTimeout` (caption or
              voice activity occurred within the idle window), remain in
              overtime (`R11`) and continue polling!
            - Once `s.now().Sub(lastActivity) >= effectiveIdleTimeout` (neither
              live caption activity nor voice audio activity has occurred for
              `effectiveIdleTimeout`), log
              `slog.Info("capture: nominal duration and idle timeout elapsed; stopping capture")`
              and call `cancel()` on `runCtx` so `EnableCaptions` and
              `CaptureAudio` exit cleanly and flush `.md` and `.mp3` (`R12`).
       - When `runCtx` was cancelled by the watchdog (`watchdogStopped == true`),
         treat `isShutdown(runCtx, e)` as graceful completion so `Session.Run`
         flushes the transcript and MP3 and returns `nil`.
       - When `s.cfg.Mode == "ephemeral-live"` and `!isRecap`, skip synchronous
         post-call `AISummary()` extraction (`R13`).
* **Files (2 files):** `internal/capture/output.go`,
  `internal/capture/session.go`.
* **Verify:** `go test -race ./internal/capture/...` passes.
* **Depends on:** U4 (`022.004-T`).

### U7 (`022.007-T`) — Add deterministic unit tests for nominal duration, overtime continuation, call-end shutdown, fallback `MeetingTitle`, and `AppendAISummaryToFile` in `internal/capture`

* **Domain:** tests (Go `internal/capture`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/capture/session_test.go`:
     * Add `TestSessionRunStopsAfterNominalDurationAndIdleTimeout`: verifies a
       live session with `Duration: 40ms, IdleTimeout: 20ms` and no activity
       exits cleanly with `nil` error and flushes the `.md` and `.mp3` files.
     * Add `TestSessionRunExtendsOvertimeWhileCaptionOrAudioActivityPersists`:
       verifies a live session with `Duration: 40ms, IdleTimeout: 50ms` that
       receives periodic `RecordCaptionActivity()` and `RecordAudioActivity()`
       heartbeats past `120ms` stays alive in overtime and only shuts down once
       heartbeats stop for `50ms` (`R11`, `R12`).
     * Add `TestSessionRunStopsEarlyOnCallEndedSignal`: verifies a live session
       with a long `Duration: 10s` exits promptly after `RecordCallEnded()` once
       activity is silent (`R3`).
     * Add `TestSessionRunUsesConfiguredMeetingTitleFallbackWhenDOMTitleEmpty`:
       verifies `SessionConfig.MeetingTitle` populates `<concat-meeting-name>`
       and the Markdown header when `MetadataProvider` is absent or empty, while
       a non-empty `MetadataProvider` title takes precedence (`R5`).
     * Add `TestAppendAISummaryToFileAppendsWithoutOverwritingAndIsIdempotent`:
       verifies `AppendAISummaryToFile` appends the formatted AI Summary section
       to an existing live transcript `.md` file while preserving all caption
       blockquotes and is idempotent on a second call (`R14`).
* **Files (1 file):** `internal/capture/session_test.go`.
* **Verify:** `go test -race -cover ./internal/capture/...` passes.
* **Depends on:** U6 (`022.006-T`).

### U8 (`022.008-T`) — Bind capture protocol and lifecycle CLI flags (`join.go`) and implement serialized deferred queue helpers and `--not-before` handling (`queue.go`) (`R1`, `R9`, `R12`–`R15`)

* **Domain:** code (Go `internal/cli`). **Posture:** test-first. **Size:** `M` | **Complexity:** `medium`.
* **What:**
  1. In `internal/cli/join.go`:
     * Extend `captureFlags` and `bindCaptureFlags` with:
       - `mode string` (`--mode`, `"meeting capture protocol: auto, ephemeral-live, live-with-summary, or past-recap"`)
       - `duration time.Duration` (`--duration`, `"nominal live capture duration (e.g. 30m, 1h) before post-duration idle monitoring"`)
       - `idleTimeout time.Duration` (`--idle-timeout`, `"inactivity grace period after duration or call-end before stopping capture (default 3m)"`)
       - `aiSummaryDelay time.Duration` (`--ai-summary-delay`, `"delay after a live call ends before fetching and appending the AI Summary (default 30m)"`)
       - `noAISummary bool` (`--no-ai-summary`, `"disable delayed post-call AI Summary capture"`)
       - `lobbyTimeout time.Duration` (`--lobby-timeout`, `"maximum time to wait in pre-join or lobby before aborting"`)
       - `appendAISummary string` (`--append-ai-summary`, `"existing Markdown transcript file to append the Meeting Recap AI Summary into"`)
       - `notBefore string` (`--not-before`, `"RFC3339 timestamp before which a queued --append-ai-summary entry is skipped"`)
     * Update `applyCaptureFlags` to apply these flags onto `config.Config`
       (including `ephemeral-live` defaulting `cfg.CaptureAudio = true` unless
       `--audio` / `--recap-audio` or config `capture_audio` / `audio` was
       explicitly set to `false`, and setting `cfg.AISummaryDelay = "0s"` when
       `--no-ai-summary` or `mode == "ephemeral-live"`).
     * Update `inheritedQueueFlags` to forward `--mode`, `--duration`,
       `--idle-timeout`, `--ai-summary-delay`, `--no-ai-summary`, and
       `--lobby-timeout` when changed, while never forwarding `--url`,
       `--queue`, `--append-ai-summary`, or `--not-before`.
  2. In `internal/cli/queue.go` (`R14`, `R15`):
     * Define sentinel error `errQueueEntryNotYetDue = errors.New("cli: queue entry not yet due")`.
     * When `executeQueueLine` (or `runQueueFile` inspecting `--not-before`)
       sees a `--not-before <RFC3339>` timestamp that is still in the future
       (`now.Before(notBefore)`), return `errQueueEntryNotYetDue`.
     * In `runQueueFile`: when `errors.Is(err, errQueueEntryNotYetDue)`, log
       `slog.Info("cli: skipping deferred queue entry before not-before timestamp", "line", i+1)`
       and leave the line uncommented (do NOT prepend `::` and do NOT append to
       `errs`) so a later `--queue` or `schedule` run executes it once due!
     * Export package helpers `appendDeferredAISummaryQueueLine(queuePath, meetingURL, transcriptPath string, notBefore time.Time) error`
       and `markDeferredAISummaryQueueLineDone(queuePath, transcriptPath string) error`
       protected by a package-level `queueFileMu sync.Mutex` that always
       re-reads the current on-disk queue file before mutating and writing via
       `writeQueueLines`, and also use that same re-read-under-mutex helper when
       `runQueueFile` marks a completed line as `::` so concurrent or nested
       appends are never overwritten!
* **Files (2 files):** `internal/cli/join.go`, `internal/cli/queue.go`.
* **Verify:** `go test -race ./internal/cli/...` passes.
* **Depends on:** U1 (`022.001-T`), U4 (`022.004-T`), U6 (`022.006-T`).

### U9 (`022.009-T`) — Wire `root` / `capture` Idea 2C delayed AI Summary execution and enrich `teams-cli schedule` with per-meeting overrides, flags, and profile-gate release (`R2`, `R4`–`R8`, `R14`, `R15`)

* **Domain:** code (Go `internal/cli`). **Posture:** test-first. **Size:** `M` | **Complexity:** `medium`.
* **What:**
  1. In `internal/cli/root.go`:
     * Pass `LobbyTimeout` and `ForceRecap: c.CaptureMode == config.CaptureModePastRecap`
       in `defaultNewMeeting`, and pass `Mode`, `Duration`, `IdleTimeout`, and
       `MeetingTitle` in `defaultDeps().newSession`.
     * When `--append-ai-summary` is passed on `root`, `join`, or `captions`,
       route execution to `runAppendAISummary(cmd.Context(), d, cfg, mutated, flags.appendAISummary)`
       (checking `--not-before` first when set and returning `errQueueEntryNotYetDue`
       when still in the future).
  2. In `internal/cli/capture.go` (`R13`, `R14`):
     * Implement `resolveCaptureDurations(cfg config.Config)` and
       `effectiveAISummaryDelay(cfg config.Config, isRecap bool) time.Duration`
       (returning `0` for `past-recap`, `ephemeral-live`, or when
       `cfg.AISummaryDelay == "0"` / `"0s"`, and defaulting to `30 * time.Minute`
       for `live-with-summary` when `cfg.AISummaryDelay == ""`).
     * Implement `runAppendAISummary(ctx context.Context, d *deps, cfg config.Config, meetingURL, transcriptPath string) error`:
       sets `cfg.CaptureMode = config.CaptureModePastRecap` (so
       `defaultNewMeeting` sets `MeetingOptions.ForceRecap = true` and
       `chromeMeeting.Join` uses the Recap discovery/extraction path and
       verifies recording identity via `ExtractMeetingIdentity` rather than
       entering the live pre-join FSM), allocates a meeting via
       `d.newMeeting(ctx, cfg, meetingURL)` (`defer cleanup()`), calls
       `meeting.Join(browserCtx)`, extracts `AISummary()` via
       `browser.AISummaryProvider`, and calls
       `capture.AppendAISummaryToFile(transcriptPath, summary)` (returning an
       error if no summary is available yet so queue retries remain active).
     * Implement the Idea 2C hybrid flow in `runCaptureWithHooks`:
       1. Run `session.Run(browserCtx, meeting)` inside an inner function whose
          `defer cleanup()` closes Chromium immediately when the live call ends,
          releasing the `UserDataDir` profile before any post-call delay begins!
       2. Release any caller-supplied `onLiveCallFinished func()` hook
          immediately after `cleanup()` so `scheduleJob` frees its `busy`
          profile gate before waiting out `ai_summary_delay`!
       3. When the completed session was a live call (`!isRecap`), produced a
          non-empty `transcriptPath`, and `delay := effectiveAISummaryDelay(cfg, isRecap); delay > 0`:
          - Resolve the queue file path (`cfg.QueueFile`, or `"transcription_queue.txt"`
            if that file exists in the working directory) and write a crash-safe
            pending line:
            `"<meetingURL>" --append-ai-summary "<transcriptPath>" --not-before <RFC3339>`
            using `appendDeferredAISummaryQueueLine` from `022.008-T`.
          - When `scheduleAsyncSummary func(delay time.Duration, fn func())` is
            provided by `scheduleJob`, detach the `delay` timer into a
            daemon-managed goroutine (tracked by a daemon `sync.WaitGroup`) so
            `cron.SkipIfStillRunning` never blocks recurring triggers of the
            same schedule entry during the 30-minute wait! In one-shot `join`
            mode, wait on `time.NewTimer(delay)` respecting `ctx.Done()`,
            acquire the optional `acquireProfile func() (func(), bool)` hook,
            run `runAppendAISummary(ctx, d, cfg, meetingURL, transcriptPath)`,
            and on success mark the corresponding queue file line with `::` via
            `markDeferredAISummaryQueueLineDone` from `022.008-T`!
  3. In `internal/cli/schedule.go` (`R2`, `R4`–`R8`, `R14`):
     * Export `defaultScheduleDuration = 1 * time.Hour` and
       `defaultScheduleIdleTimeout = 3 * time.Minute` (`R2`, `A1`).
     * Bind `--audio`, `--recap-audio`, `--headless`, `--headed`, `--duration`,
       `--idle-timeout`, `--mode`, `--ai-summary-delay`, `--no-ai-summary`, and
       `--lobby-timeout` on `newScheduleCommand` (`R6`), applying CLI flag
       overrides over `config.yaml` defaults.
     * Map each `config.MeetingSchedule` into `scheduler.Entry` with `Name`,
       `URL`, `Cron`, `At`, `Mode`, `Duration`, `IdleTimeout`, `AISummaryDelay`,
       `LobbyTimeout`, `Category`, `CaptureAudio`, `OutputDir`, and
       `ValidateURL: teams.ValidateMeetingURL` (`R4`, `R7`, `R8`).
     * In `scheduleJob` and `newScheduleCommand`:
       - Build a per-invocation `entryCfg := cfg` applying `e.Category`,
         `e.OutputDir`, `e.CaptureAudio`, `e.Mode`, `e.Duration` (falling back
         to `cfg.Duration` and then to `defaultScheduleDuration` = `1h`),
         `e.IdleTimeout` (falling back to `cfg.IdleTimeout` and then to
         `defaultScheduleIdleTimeout` = `3m`), `e.AISummaryDelay`,
         `e.LobbyTimeout`, and `e.Name` as fallback `MeetingTitle` (`R2`, `R4`,
         `R5`).
       - Detach post-call `ai_summary_delay` timers from the cron job's blocking
         execution via a daemon-level `sync.WaitGroup` (cancelled/awaited during
         `shutdownTimeout`) and release `busy.Store(false)` as soon as the live
         call capture and browser teardown finish so neither `busy` nor
         `cron.SkipIfStillRunning` blocks subsequent triggers during the
         30-minute summary wait (`R14`).
       - On `teams-cli schedule` startup (when a `queue_file` or default
         `transcription_queue.txt` is present), run a non-blocking catch-up sweep
         for any due `--append-ai-summary` queue entries whose `--not-before`
         timestamp has already passed so daemon restarts recover pending
         summaries (`R14`, `R15`).
* **Files (3 files):** `internal/cli/root.go`, `internal/cli/capture.go`,
  `internal/cli/schedule.go`.
* **Verify:** `go test -race ./internal/cli/...` passes.
* **Depends on:** U1 (`022.001-T`), U2 (`022.002-T`), U8 (`022.008-T`).

### U10 (`022.010-T`) — Add CLI unit and integration tests for capture protocols, Idea 2C delayed AI Summary timer + queue fallback, and `schedule` overrides (`R1`–`R9`, `R12`–`R15`)

* **Domain:** tests (Go `internal/cli`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/cli/capture_test.go`:
     * Add unit tests for `--duration`, `--idle-timeout`, `--mode`
       (`ephemeral-live` defaulting `CaptureAudio = true` and `AISummaryDelay = 0`,
       `live-with-summary`, `past-recap`, `auto`), `--ai-summary-delay`,
       `--no-ai-summary`, and `--lobby-timeout`.
     * Add `TestRunCaptureLiveWithSummaryWritesQueueFallbackAndAppendsAfterDelay`
       verifying that when a `live-with-summary` call finishes:
       (a) Chromium `cleanup()` runs immediately before the delay timer expires;
       (b) a `--append-ai-summary "<transcript.md>" --not-before <RFC3339>` line
       is written to the configured `queue_file`;
       (c) once the short test `ai_summary_delay` (`20ms`) fires,
       `runAppendAISummary` opens the meeting in Recap mode, appends `# AI Summary`
       to the existing `.md` transcript without overwriting the live captions,
       and comments out the queue line with `::` (`R14`).
  2. In `internal/cli/queue_test.go`:
     * Add `TestQueueSkipsFutureNotBeforeAndExecutesDueAppendAISummary` proving
       a `--append-ai-summary <md> --not-before <future>` line is skipped
       without error and left uncommented, whereas a line whose `--not-before`
       timestamp has passed appends the AI summary to `<md>` and is marked `::`
       (`R14`, `R15`).
     * Add `TestQueueBatchCatchUpUsesConfiguredOutputDirAndPerLineFlags`
       verifying per-line `--mode`, `--category`, and `--audio` resolve into the
       configured `output_dir` (`R15`).
  3. In `internal/cli/schedule_test.go`:
     * Add tests verifying `schedule` flag bindings (`--audio`, `--headless`,
       `--headed`, `--duration`, `--idle-timeout`, `--mode`,
       `--ai-summary-delay`, `--lobby-timeout`), per-meeting `MeetingSchedule`
       overrides (`Name` -> `MeetingTitle`, default `1h` duration and `3m` idle
       timeout, `Category`, `CaptureAudio`, `OutputDir`, one-shot `At`), and
       `busy` profile-gate release before `ai_summary_delay` so a second
       scheduled meeting runs during the delay window (`R2`, `R4`–`R8`, `R14`).
* **Files (3 files):** `internal/cli/capture_test.go`,
  `internal/cli/queue_test.go`, `internal/cli/schedule_test.go`.
* **Verify:** `go test -race -cover ./internal/cli/...` passes.
* **Depends on:** U8 (`022.008-T`), U9 (`022.009-T`).

### U11 (`022.011-T`) — Document scheduled capture (`cron` + `at`), capture modes, delayed AI Summary, and Windows Task Scheduler recipes in `README.md`, `docs/ARCHITECTURE.md`, and `config.example.yaml` (`R10`)

* **Domain:** docs (Markdown & example YAML). **Posture:** migration-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `README.md` (`R10`):
     * Document `--mode`, `--duration`, `--idle-timeout`, `--ai-summary-delay`,
       `--no-ai-summary`, `--lobby-timeout`, `--append-ai-summary`, and
       `--not-before`.
     * Add a dedicated `### Scheduled and protocol-driven capture` section with:
       - Explanation of the four capture protocols (`ephemeral-live`,
         `live-with-summary`, `past-recap`, `auto`), activity-aware nominal
         `--duration` + overtime continuation + `--idle-timeout` shutdown, and
         Idea 2C hybrid delayed AI Summary capture (`--ai-summary-delay` +
         `transcription_queue.txt` `--append-ai-summary --not-before` fallback).
       - Copy-pasteable `config.yaml` and CLI examples for `teams-cli schedule`
         using both recurring `cron:` and one-shot `at:` meeting entries with
         per-meeting `mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
         `category`, and `capture_audio`.
       - Copy-pasteable Windows Task Scheduler (`Register-ScheduledTask` and
         `schtasks /Create /SC ONCE`) recipes invoking
         `teams-cli.exe join "https://teams.microsoft.com/meet/synthetic" --duration 45m --audio --headless`.
     * Update `## Configuration` key/alias table and `### Batch queue mode`
       (`inheritedQueueFlags` and `--append-ai-summary` / `--not-before`).
  2. In `docs/ARCHITECTURE.md`:
     * Update `### Live join and captions`, `### Batch queue`, and
       `### Cron schedule` to describe activity-aware duration/overtime/idle
       shutdown, in-browser call-end detection, capture protocols, one-shot `at`
       triggers, and Idea 2C delayed AI summary enrichment.
  3. In `config.example.yaml`:
     * Add commented examples for `default_account`, `capture_mode`, `duration`,
       `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `queue_file`, and
       both `cron:` and `at:` `meetings:` entries.
* **Files (3 files):** `README.md`, `docs/ARCHITECTURE.md`,
  `config.example.yaml`.
* **Verify:** Stage changes with `git add` and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Depends on:** U8 (`022.008-T`), U9 (`022.009-T`).

## Dependency Graph

```text
U1 (022.001-T: internal/config R1/R4/R7/R9/R12-R15) ──────────────────────────┐
U2 (022.002-T: internal/scheduler cron + one-shot at R7/R8) ──────────────────┼──> U9 (022.009-T: CLI queue & schedule) ──┬──> U10 (022.010-T: CLI tests)
U3 (022.003-T: browser callEndedJS, interim captions, PCM RMS) ───────────────┤                                           └──> U11 (022.011-T: README, ARCHITECTURE, config.example.yaml)
  └──> U4 (022.004-T: browser LobbyTimeout, ActivityMonitor, call-end poll) ──┼──> U8 (022.008-T: CLI root/join/capture) ─┘
         ├──> U5 (022.005-T: browser unit tests)                              │
         └──> U6 (022.006-T: capture Session lifecycle & AppendAISummary) ────┘
                └──> U7 (022.007-T: capture unit tests)
```

* Shipment sequencing: `022-S` depends on `021-S` (`backlogit dep add 022-S 021-S --type blocks`).

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library + existing `robfig/cron/v3`, `chromedp`, and `cobra`; zero new external dependencies; clean `go vet` and `golangci-lint` |
| II: Test first | Dedicated unit and concurrency tests across `internal/config`, `internal/scheduler`, `internal/browser`, `internal/capture`, and `internal/cli` under `-race` |
| III, IV: Secrets & containment | Queue file updates use `writeQueueLines` (`0o600`), transcript appends preserve `0o600` permissions, and queue logs redact URLs via `teams.RedactArgs` |
| VI: Single responsibility | 11 width-isolated tasks, each touching `<= 3` files in a single domain (`code`, `tests`, or `docs`) |
| VII, VIII: Strict safety | In-place transcript append, queue persistence, and browser lifecycle management hardened below |
| IX, X, XI: Persistence & merge history | Deliberation `011-DL`, spec `R1`–`R15`, plan, feature `022-F`, tasks `022.001-T`–`022.011-T`, and shipment `022-S` recorded with full traceability (`77FC68CA`) |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: adds new CLI flags
  (`--mode`, `--duration`, `--idle-timeout`, `--ai-summary-delay`,
  `--no-ai-summary`, `--lobby-timeout`, `--append-ai-summary`, `--not-before`),
  top-level and per-meeting `config.yaml` fields, `scheduler.Entry.At`, and
  `capture.AppendAISummaryToFile`.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  appends AI Summaries to existing `0o600` transcript files and writes deferred
  enrichment entries to `transcription_queue.txt` (`0o600`).
* Migration, backfill, destructive data/config action, or irreversible step —
  **present (in-place file mutation)**: `AppendAISummaryToFile` mutates an
  existing `.md` transcript file in place (`O_WRONLY|O_APPEND`) rather than
  truncating it, and `writeQueueLines` updates `transcription_queue.txt`.
* External integration, operator checkpoint, or external dependency — **absent**:
  zero new third-party dependencies.
* High runtime, rollout, or rollback risk — **moderate**: timer, goroutine, and
  profile `busy`-gate coordination across live capture and delayed AI summary
  enrichment; mitigated by immediate browser teardown after live capture and
  crash-safe queue fallback.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because `022-F` introduces concurrent activity-watchdog
goroutines in `internal/capture`, delayed post-call timers and profile-lock
handoffs in `internal/cli`, and in-place append mutations on existing Markdown
transcripts (`AppendAISummaryToFile`).

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md`
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md`
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md`

### Protected Invariants

1. **Leaf-package isolation (`docs/ARCHITECTURE.md`)**: `internal/config` and
   `internal/scheduler` MUST NOT import `internal/teams`, `internal/browser`,
   or `internal/capture`. `internal/cli` wires `teams.ValidateMeetingURL` into
   `scheduler.Entry.ValidateURL`.
2. **Non-destructive transcript append (`R14`)**: `AppendAISummaryToFile` MUST
   open existing transcripts with `os.O_WRONLY|os.O_APPEND` (never `os.O_TRUNC`),
   MUST fail if the target file does not exist on disk, and MUST be idempotent
   if `# AI Summary` / `## AI Summary` is already present in the file.
3. **Immediate browser & profile release after live call (`R2`, `R14`)**: When a
   live call finishes (`Session.Run` returns), `cleanup()` MUST terminate the
   Chromium process and `scheduleJob` MUST release the `busy` atomic gate
   *before* waiting on the `ai_summary_delay` timer, so back-to-back scheduled
   meetings are never blocked during the 30-minute AI summary delay window.
4. **Overtime preservation (`R11`, `R12`)**: Reaching nominal `Duration` MUST
   NEVER sever a call while caption mutations or PCM voice activity
   (`PCMRMS >= DefaultVoiceRMSThreshold`) have occurred within `IdleTimeout`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Append delayed AI Summary to existing `.md` transcript via `capture.AppendAISummaryToFile` (`os.O_WRONLY\|os.O_APPEND`, `0o600`) and update `transcription_queue.txt` via `writeQueueLines` | medium | Verifies file exists via `os.ReadFile` before opening with `O_APPEND`; never truncates existing live captions; checks idempotency marker (`# AI Summary`) | Existing live transcript retains all recorded captions and gains the `# AI Summary` section at EOF |
| Manage Chromium process lifecycle and shared `UserDataDir` profile `busy` gate across nominal `duration`, overtime, and `ai_summary_delay` timer in `internal/capture` and `internal/cli` | medium | Deferred `cleanup()` closes Chromium immediately after `Session.Run`; `busy` gate is released before the `ai_summary_delay` sleep and re-acquired only for the brief Recap fetch | Scheduled meetings release the browser profile automatically after each call and never deadlock subsequent scheduled runs |

## Runtime Verification and Closure

* **Verification**:
  * `go test -race -cover ./internal/config/... ./internal/scheduler/... ./internal/browser/... ./internal/capture/... ./internal/cli/...`
  * `go test -race ./...`
  * `go vet ./...` and `go vet -tags=integration ./...`
  * `golangci-lint run`
  * `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
* **Rollback**: Revert the `022-S` merge commit on `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants and `strict-safety`
  `ProposedAction` / `ActionRisk` table.

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — All 11 units satisfy the 2-hour rule (`<= 3` files per task), width isolation (`code`, `tests`, `docs`), test-first posture, and zero new external dependencies. | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Context-cancelled watchdog goroutines, race-free mutex/atomic state in `chromeMeeting` and `capture.Session`, `oneShotSchedule` integrating cleanly with `robfig/cron/v3` `Stop()`. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Traces 1:1 to `77FC68CA` and `R1`–`R15` (`D1`–`D3`) in `docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md`; leaves `010-S` and excluded stash entries untouched. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Preserves `shine-mp3` frame-alignment and `checkptr` invariants in `CaptureAudio` while computing `PCMRMS` on decoded `[]int16` slices. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly preserves leaf-package isolation (`internal/config` and `internal/scheduler` import no internal packages; `internal/cli` bridges `config`, `scheduler`, `browser`, and `capture`). | PASS |
| Concurrency & Security Reviewer (timers, queues & file mutation) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — `AppendAISummaryToFile` uses `O_WRONLY\|O_APPEND` with existence and idempotency checks, `writeQueueLines` uses atomic temp-file rename + `0o600` fallback, and `busy` gate releases before `ai_summary_delay`. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `022-F` and shipment `022-S`.
