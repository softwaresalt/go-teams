---
title: "Scheduled Live Call and Capture Requirements"
description: "Product requirements for scheduling teams-cli to launch, join, capture live captions and MP3 audio, and terminate cleanly"
doc_type: spec
source: "docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md"
date: "2026-11-13"
source_stash_ids:
  - "77FC68CA"
source_research:
  - "docs/ARCHITECTURE.md"
  - "internal/cli/schedule.go"
  - "internal/cli/join.go"
  - "internal/cli/capture.go"
  - "internal/scheduler/schedule.go"
  - "internal/capture/session.go"
  - "internal/config/config.go"
  - "internal/browser/meeting.go"
  - "internal/browser/meeting_chrome.go"
scope: "standard"
handoff_status: "ready_for_plan"
dark_factory_ready: false
requirement_ids:
  - "R1"
  - "R2"
  - "R3"
  - "R4"
  - "R5"
  - "R6"
  - "R7"
  - "R8"
  - "R9"
  - "R10"
  - "R11"
  - "R12"
  - "R13"
  - "R14"
  - "R15"
---

## Problem Frame

### What We Are Solving and Why

Operators want `teams-cli` to join scheduled Microsoft Teams meetings unattended,
enable live captions, optionally record meeting audio to MP3, and flush clean
Markdown and MP3 artifacts to disk without manual intervention at meeting start
or end.

A code scan of `internal/cli/schedule.go`, `internal/scheduler/schedule.go`,
`internal/capture/session.go`, and `internal/config/config.go` reveals that
while a cron-based `teams-cli schedule` subcommand already exists, four
structural gaps currently prevent effective scheduled live capture in practice:

* **Unbounded live capture blocks all subsequent scheduled meetings.**
  `scheduleJob` in `internal/cli/schedule.go` passes the daemon's root
  `cmd.Context()` directly into `runCapture`. In `capture.Session.Run`, live
  caption and audio streams run until their context is cancelled. Because
  neither a per-meeting duration timeout nor in-browser call-end detection
  exists, the first scheduled live call never exits on its own. It holds the
  single Chromium `UserDataDir` profile and keeps the `busy` atomic gate set to
  `true`, causing every later scheduled meeting to be skipped with
  `schedule: skipped, profile busy`.
* **One-shot OS scheduling lacks bounded execution on `join` and `captions`.**
  Operators who prefer Windows Task Scheduler, `cron`, or `systemd` timers over
  a continuously running daemon cannot run `teams-cli join <url> --duration 60m`
  because `join` and `captions` have no `--duration` flag to stop capture and
  exit cleanly when the scheduled window ends.
* **Scheduled meeting entries drop metadata and per-meeting overrides.**
  `config.MeetingSchedule` (`internal/config/config.go`) only defines `name`,
  `url`, and `cron`. In `scheduleJob`, `e.Name` is used only for log messages
  and is never passed into `capture.SessionConfig` for filename generation
  (`<concat-meeting-name>`), nor can an entry specify its own `duration`,
  `category`, or `capture_audio` setting.
* **One-time scheduled meetings require cron workarounds.**
  `scheduler.Entry.Validate` accepts only standard 5-field recurring cron
  expressions (`robfig/cron/v3`). Scheduling a single ad-hoc meeting at a
  specific date and time requires either editing `config.yaml` after the call or
  using an external OS task scheduler.
* **Different meeting types need distinct capture protocols (Batch Past Recaps
  vs. Live + Delayed AI Summary vs. Ephemeral Live-Only Calls):**
  1. *Scenario 1 (Batch catch-up on past calls via `transcription_queue.txt`)*:
     The operator queues many completed calls in `transcription_queue.txt` and
     wants `teams-cli` to iterate through each call, export its transcript, AI
     Summary, and optional MP3 into the configured `output_dir`, and mark each
     completed line with `::`.
  2. *Scenario 2 (Live call with delayed AI Summary generation)*: The operator
     captures a live call in real time, but Microsoft Teams does not generate
     the AI Summary (`Meeting notes`) until 5–30 minutes after the call ends.
     Because `capture.Session.Run` currently checks `AISummary()` only at the
     instant live capture stops, the AI Summary is always missed on live calls
     unless `teams-cli` supports a configurable post-call delay (for example,
     default `30m`) to fetch and append the AI Summary after the call finishes.
  3. *Scenario 3 (Ephemeral live call with no post-call recap or recording)*:
     Some meetings (external tenants, unrecorded calls, or restricted policies)
     leave no post-call Recap link, no saved transcript, and no AI Summary once
     the call ends. Everything must be captured in real time (live captions +
     live MP3 audio) with zero post-call AI Summary wait.

### Pressure-Tested Framing

Rather than treating "scheduling" as a single uniform flow, the highest-leverage
framing is **Protocol-Driven Capture Modes + Activity-Aware Lifecycle +
Dual-Trigger Scheduling**:

1. **Classify every capture into an explicit Meeting Capture Protocol
   (`capture_mode` / `--mode`)**:
   * `past-recap` (*Scenario 1 — Post-Meeting Catch-Up*): Finite export of
     transcript, AI Summary, and optional MP3 from completed meetings, iterated
     via `--queue transcription_queue.txt` into the configured `output_dir`.
   * `live-with-summary` (*Scenario 2 — Live Call + Delayed AI Summary*):
     Real-time caption and optional audio capture during the live call, immediate
     flush of `.md` and `.mp3` when the call ends, release of the browser
     profile, and a delayed post-call pass after `ai_summary_delay` (default
     `30m`) that appends `## AI Summary` to the saved Markdown transcript.
   * `ephemeral-live` (*Scenario 3 — Live-Only, No Post-Call Recap*): Real-time
     caption and MP3 audio capture during the live call with no post-call AI
     Summary wait (`ai_summary_delay: 0`), ensuring calls that vanish after
     hangup are captured completely in real time.
   * `auto` (*Default*): Automatically selects `past-recap` for Meeting Recap
     URLs and either `live-with-summary` (when `ai_summary_delay > 0`) or
     `ephemeral-live` for live join URLs.
2. **Make every live capture self-terminating without cutting off meetings that
   run overtime**:
   * Treat `--duration` / `duration` as the **scheduled nominal duration**
     (baseline window), not a blind cutoff.
   * If a meeting runs past its scheduled `duration`, automatically **continue
     capturing in overtime** as long as live caption activity and/or voice
     audio activity is still detected.
   * Shut down cleanly when either (a) the browser detects the Teams end-of-call
     teardown and caption/voice activity ceases, or (b) the nominal `duration`
     has elapsed and neither live caption activity nor voice audio activity has
     occurred for a configurable silence/inactivity grace window
     (`--idle-timeout` / `idle_timeout`).
3. **Support both trigger models on top of that shared capture lifecycle**:
   * **One-shot CLI / Queue trigger (`teams-cli join --duration ...` and
     `teams-cli --queue transcription_queue.txt`)** for Windows Task Scheduler,
     `at`, `cron`, or batch catch-up runs.
   * **Built-in daemon trigger (`teams-cli schedule`)** for recurring or
     pre-listed meetings in `config.yaml`, enriched with per-meeting `mode`,
     `duration`, `idle_timeout`, `ai_summary_delay`, `category`, `audio`,
     meeting-name propagation, and one-time `at` timestamps alongside recurring
     `cron` expressions.
4. **Pair with unattended Entra account selection (`30E3B3A7`) and live DOM
   selector verification (`010-S`)** so that when the browser opens at meeting
   time, neither the Entra account picker nor the `/dl/launcher/launcher.html`
   interstitial blocks admission.

## Requirements

**Bounded and Activity-Aware Live Capture Lifecycle**

* R1. `teams-cli join`, `teams-cli captions`, and the root command MUST accept a
  `--duration` flag (Go duration syntax such as `30m`, `1h`, `1h30m`, plus a
  matching `duration` key in `config.yaml`) that defines the nominal scheduled
  capture window before post-duration inactivity monitoring begins, and flushes
  the Markdown transcript and MP3 file cleanly when the capture concludes.
* R2. In `teams-cli schedule`, every scheduled meeting invocation MUST run with
  a nominal `duration` (from the meeting entry's `duration`, falling back to
  top-level `config.duration`, and then to a default nominal window such as
  `1h`) paired with post-duration caption/voice inactivity detection (`R11`,
  `R12`) so a finished call releases the Chromium profile automatically while an
  active call running overtime is never cut off mid-conversation.
* R3. `internal/browser` SHOULD detect in-browser meeting termination / stage
  teardown signals during live capture (such as the Teams "You left the meeting"
  / "Call ended" state or loss of the active call surface after admission) and,
  once no further caption or voice activity arrives, cancel the active capture
  loop gracefully even if the nominal `--duration` has not yet elapsed.

**Per-Meeting Configuration and Metadata Propagation**

* R4. `config.MeetingSchedule` MUST support optional per-meeting fields:
  `duration` (duration string), `category` (string), `capture_audio` / `audio`
  (`*bool` override of top-level `capture_audio`), and `output_dir` (string).
* R5. When `teams-cli schedule` executes a `MeetingSchedule` entry, `e.Name`
  MUST be forwarded into the capture session as the fallback meeting title so
  `DefaultFilenameTemplate` (`<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md`)
  and the Markdown header use the configured meeting name whenever the live DOM
  does not supply a richer title.
* R6. `teams-cli schedule` MUST bind `--audio` / `--recap-audio`, `--headless`,
  and `--headed` CLI flags consistently with `join` and `captions`, applying CLI
  flag overrides over `config.yaml` defaults.

**Flexible Schedule Triggers (Recurring and One-Shot)**

* R7. `config.MeetingSchedule` and `internal/scheduler` SHOULD support one-time
  scheduled timestamps via an `at` field (RFC 3339 or local `YYYY-MM-DD HH:MM`
  timestamp) as an alternative to `cron`, firing once at the target time and
  skipping past timestamps on daemon startup with a structured log message.
* R8. `scheduler.Entry.Validate` MUST require exactly one trigger specification
  (`cron` or `at`), a non-empty `url` passing `teams.ValidateMeetingURL`, and a
  positive `duration` when specified.

**Unattended Readiness and Pre-Join Resilience**

* R9. Scheduled and one-shot live captures MUST honor a configurable lobby/join
  wait window (or retry policy when the organizer has not yet started the
  meeting) and release the browser process and profile lock cleanly if admission
  does not succeed within the window.
* R10. Documentation (`README.md`) MUST provide copy-pasteable examples for both
  operating models: (a) running `teams-cli schedule` with recurring `cron` and
  one-shot `at` entries in `config.yaml`, and (b) registering a one-shot Windows
  Task Scheduler (`Register-ScheduledTask` / `schtasks`) task invoking
  `teams-cli join <url> --duration <d> --audio --headless`.
* R11. **Overtime Continuation via Caption and Voice Activity Monitoring**: When
  a live call reaches its nominal `--duration` / `duration` without an explicit
  call-ended teardown, `teams-cli` MUST continue capturing into overtime as long
  as ongoing live caption activity (DOM caption mutations or finalized
  utterances) or ongoing voice audio activity (non-silent PCM audio frames above
  a voice-activity energy threshold when audio capture is enabled) is observed.
* R12. **Silence and Inactivity End-of-Call Shutdown**: Once either (a) the
  nominal `duration` has elapsed or (b) an end-of-call DOM trigger/teardown is
  observed, `teams-cli` MUST shut down the capture and flush the Markdown and
  MP3 files as soon as neither live caption activity nor voice audio activity is
  detected for a configurable inactivity grace period (`--idle-timeout` /
  `idle_timeout` in `config.yaml` and per-meeting `meetings[].idle_timeout`,
  default `3m`).

**Meeting Capture Protocols, Batch Queue Catch-Up, and Delayed AI Summary**

* R13. **Meeting Capture Protocol Classification (`capture_mode` / `--mode`)**:
  `teams-cli` MUST support a configurable meeting capture mode (`--mode` CLI
  flag, top-level `capture_mode` in `config.yaml`, and per-meeting
  `meetings[].mode`) with four values:
  * `ephemeral-live`: Real-time caption and MP3 audio capture (`capture_audio`
    defaults to `true` unless explicitly set to `false` via flag or config) with
    no post-call AI Summary wait (`ai_summary_delay: 0`), designed for calls
    that leave no post-meeting Recap or recording.
  * `live-with-summary`: Real-time caption and optional audio capture during the
    live call, followed by a delayed post-call Recap pass after
    `ai_summary_delay` (default `30m`) that fetches the generated AI Summary and
    appends `## AI Summary` to the already-flushed Markdown transcript.
  * `past-recap`: Finite post-meeting Recap extraction (transcript + immediate
    AI Summary + optional MP3) with no live-call wait.
  * `auto` (default): Automatically selects `past-recap` for Recap URLs/views
    and `live-with-summary` or `ephemeral-live` for live join URLs based on
    whether delayed AI Summary capture is enabled.
* R14. **Delayed Post-Call AI Summary Capture via Hybrid Timer + Queue Fallback
  (`--ai-summary-delay` / `ai_summary_delay`, Idea 2C)**: For `live-with-summary`
  captures, `teams-cli` MUST:
  1. Flush the live `.md` transcript and `.mp3` audio file and close the browser
     immediately when the live call ends, releasing the Chromium `UserDataDir`
     profile lock during the delay window.
  2. Record a crash-safe pending enrichment entry (`<url> --append-ai-summary
     <transcript.md> --not-before <RFC3339-timestamp>`) in the configured queue
     file (`transcription_queue.txt` / `queue_file` when configured) so the
     deferred summary pass survives process exit or machine reboot.
  3. Run an in-process timer for `ai_summary_delay` (default `30m`, configurable
     via `--ai-summary-delay`, top-level `ai_summary_delay`, or per-meeting
     `meetings[].ai_summary_delay`, and disableable via `0` /
     `--no-ai-summary`) that re-opens the meeting in Recap mode, extracts and
     appends the `## AI Summary` section to the existing `.md` transcript file
     without overwriting the live captions, and marks the corresponding queue
     file line with `::` upon completion. If the process exited before the timer
     fired, a subsequent `teams-cli --queue` or `teams-cli schedule` pass
     executes any due `--append-ai-summary` queue lines whose `--not-before`
     timestamp has passed.
* R15. **Batch Queue Catch-Up into Configured Output Directory (`--queue` /
  `transcription_queue.txt`)**: When `teams-cli --queue <file>` (or `captions` /
  `join --queue <file>`) processes a queue file of past call links, each entry
  MUST resolve its output directory from `config.yaml` (`output_dir` /
  `output_folder` / `default_path`) or outer CLI flags when no per-line output
  path is given, support per-line `--mode`, `--category`, and `--audio` flags
  (as well as `--append-ai-summary <transcript.md>` for deferred Scenario 2
  summary enrichment), and mark each succeeded line with `::` in place.

## Success Criteria

* A scheduled live meeting in `teams-cli schedule` with `duration: 30m` that
  ends on time (or early via call teardown) shuts down automatically once
  caption and voice activity cease, releases the `busy` profile gate, and allows
  the next scheduled meeting in `config.yaml` to run without manual
  intervention.
* A scheduled `30m` meeting that runs `25m` into overtime (`55m` total) with
  continuous conversation (captions and/or voice audio) is **not** cut off at
  the `30m` mark; it continues capturing until both caption and voice activity
  go silent for `idle_timeout` (or the call tears down) and then exits cleanly.
* Running `teams-cli join "<meeting-url>" --duration 45m --audio --headless`
  from the CLI or Windows Task Scheduler exits with code `0` after the call
  concludes (at `45m` + `idle_timeout` silence, or earlier on call-end teardown)
  and produces valid `0o600` `.md` and `.mp3` files named with the configured
  `<category>` and `<concat-meeting-name>`.
* All unit tests (`go test ./...`), race detector checks (`go test -race ./...`),
  `go vet ./...`, and `golangci-lint run` pass with zero findings.

## Scope Boundaries

### In Scope

* Adding `--duration` / `duration` and `--idle-timeout` / `idle_timeout` support
  to `config`, `cli` (`join`, `captions`, root, `schedule`), and
  `capture.Session`.
* Adding an activity tracker in `internal/capture` / `internal/browser` that
  monitors both live caption activity and PCM voice-energy activity (when audio
  capture is enabled) to extend capture past `duration` during active overtime
  and trigger clean shutdown once activity ceases.
* Enriching `config.MeetingSchedule` and `internal/scheduler` with per-meeting
  `duration`, `idle_timeout`, `category`, `capture_audio`, `output_dir`,
  fallback meeting-title propagation (`e.Name`), and one-shot `at` timestamp
  triggers alongside `cron`.
* Adding in-browser live call-end detection in `internal/browser` so a meeting
  that finishes early exits before the nominal duration elapses.
* Binding `--audio`, `--headless`, and `--headed` flags on `teams-cli schedule`.
* Documenting both `teams-cli schedule` and Windows Task Scheduler +
  `teams-cli join --duration` workflows.

### Out of Scope

* **Multi-profile concurrent meeting capture**: Running two simultaneous
  Chromium instances against cloned or separate `UserDataDir` profiles remains
  out of scope; single-profile serialization with bounded duration and clean
  profile release solves sequential scheduling first.
* **Microsoft Graph / Outlook Calendar API integration**: Automatically polling
  a user's Outlook calendar via OAuth/Graph API to discover meetings is out of
  scope; meeting URLs and schedules are supplied via `config.yaml`, CLI flags,
  or `--queue`.
* **Default Entra Account Auto-Selection (`30E3B3A7`)**: Tracked as its own
  dedicated feature stash entry (`30E3B3A7`) so it can ship as an independent,
  width-isolated release unit (or immediately precede scheduled capture).
* **Live DOM selector verification (`010-S`)**: Already staged and tracked in
  queued shipment `010-S`.

## Key Decisions

* **Support both daemon scheduling and OS-scheduled one-shot `join` via a shared
  activity-aware lifecycle**: Adding `--duration`, `--idle-timeout`, voice/caption
  activity monitoring, and DOM call-end detection to the core capture path fixes
  the infinite-hang bug in `teams-cli schedule` while simultaneously enabling
  zero-daemon Windows Task Scheduler automation via `teams-cli join --duration`.
* **Treat `--duration` as a nominal baseline window with activity-extended
  overtime (`R11`, `R12`)**: Meetings frequently run past their scheduled
  calendar slot. Instead of severing the recording at `duration`, `teams-cli`
  holds the call open for at least `duration` (unless explicit DOM call teardown
  + silence occurs earlier) and then keeps recording into overtime until both
  live caption activity and voice audio activity have been silent for
  `idle_timeout`.
* **Protocol-driven meeting classification (`mode: ephemeral-live |
  live-with-summary | past-recap | auto`, `R13`)**: Confirmed by operator.
  Categorizing meetings by capture protocol lets `teams-cli` distinguish
  ephemeral live calls (which have no post-call recap or AI summary and must
  record captions + MP3 in real time), recorded live calls (which need a delayed
  post-call AI summary fetch), and past recaps (finite batch export via
  `transcription_queue.txt`).
* **Idea 2C — Hybrid In-Process Timer + `transcription_queue.txt` Fallback for
  Delayed AI Summary (`R14`)**: Confirmed by operator. When a `live-with-summary`
  call ends, `teams-cli` immediately flushes the live `.md` and `.mp3`, closes
  Chromium to free the profile, writes a crash-safe `--append-ai-summary
  <path.md> --not-before <timestamp>` entry to `transcription_queue.txt` (when a
  queue file is configured), and starts a `30m` in-process timer to re-open the
  Recap view, append `## AI Summary`, and mark the queue line `::` done.
* **Keep `30E3B3A7` (Entra Default Account Selection) decoupled in the backlog
  but sequenced as a companion capability**: `30E3B3A7` modifies auth/account-picker
  DOM handling, whereas `77FC68CA` modifies scheduler/capture lifecycle and
  per-meeting config. Keeping them as separate features respects the 2-hour rule
  and width isolation while letting them work together at runtime.

## Assumptions

* **A1 (Default nominal duration and idle timeout)**: When a `meetings:` entry
  in `config.yaml` omits `duration` and top-level `duration` is also unset,
  `teams-cli schedule` uses a default nominal duration of `1h` (`60m`) and a
  default post-duration `idle_timeout` of `3m` so an unconfigured entry stays
  alive through normal meeting conversation and overtime, then exits `3m` after
  voice and captions go silent.
* **A2 (Voice activity signal source)**: When `--audio` / `capture_audio` is
  enabled, voice activity is measured from the captured PCM audio stream (RMS /
  peak sample amplitude above a low noise-floor threshold) alongside live
  caption events; in captions-only mode (`!capture_audio`), activity is measured
  from in-browser caption DOM mutations and finalized caption events.
* **A3 (One-shot `at` timestamp format)**: One-shot `at:` entries in
  `config.yaml` accept RFC 3339 (`2026-11-15T14:00:00-08:00`) and local
  `2006-01-02 15:04` (interpreted in `time.Local`), matching standard Go time
  parsing without new third-party dependencies.

## Outstanding Questions

### Resolve Before Planning

None. All blocking product questions have been resolved into explicit
requirements (`R1`–`R12`) and documented assumptions (`A1`–`A3`) above.

### Deferred to Planning

* **D1 (DOM selectors for in-browser call-end detection in `R3`)**: Which exact
  Teams v2 DOM indicators (for example disappearance of the hangup button
  `#hangup-button` / `[data-tid="call-end"]` after admission, or appearance of
  the post-call rating/rejoin banner) provide the lowest false-positive signal
  that a live call has ended? To be finalized in `deliberate` / `impl-plan` and
  verified alongside `010-S`.
* **D2 (PCM voice-activity threshold and caption interim mutation heartbeats for
  `R11`/`R12`)**: Choosing the default PCM RMS threshold so background codec
  comfort noise does not masquerade as speech, and feeding interim DOM caption
  mutations (not only finalized punctuation-terminated utterances) into the
  activity heartbeat so long sentences keep the timer alive. To be specified in
  `impl-plan`.
* **D3 (Task decomposition width)**: Splitting the implementation into
  width-isolated tasks under the 2-hour rule across `internal/config`,
  `internal/scheduler`, `internal/capture`, `internal/browser`, `internal/cli`,
  and `README.md`.

## Document Review

* **Review status**: Completed self-review pass on `2026-11-13` against
  `brainstorm/SKILL.md` Phase 5 quality criteria and `docs/ARCHITECTURE.md`
  package-boundary invariants.
* **Findings verified**:
  * Leaf-package isolation is preserved (`internal/config` and
    `internal/scheduler` do not import `internal/browser` or `internal/capture`;
    `internal/cli` translates config into `scheduler.Entry` and
    `capture.SessionConfig`).
  * All 10 requirements have stable IDs (`R1`–`R10`).
  * `Resolve Before Planning` is empty.

## Handoff

* **Handoff status**: `ready_for_plan`
* **Resolved handoff path**: `ask` (ready for operator selection: `deliberate` /
  `plan`, `queue`, or `both`)
* **Event**: `BRAINSTORM_HANDOFF_READY` (`artifact`:
  `docs/product-specs/2026-11-13-scheduled-live-call-capture-requirements.md`,
  `unresolved_blocking_questions`: `0`, `handoff_target`: `deliberate / impl-plan`)
