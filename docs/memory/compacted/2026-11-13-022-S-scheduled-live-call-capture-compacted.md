---
type: memory-compacted
date: 2026-11-13
shipment: 022-S
feature: 022-F
source:
  - docs/archive/memory/2026-11-13/022-S-scheduled-live-call-capture-memory.md
  - docs/archive/memory/2026-11-13/stage-021-S-022-S-memory.md
---

# Compacted memory - Shipment 022-S (Scheduled and protocol-driven live call capture, R1-R15)

**Shipped**: PR #55, merge `7b6c31d98ab00f1aaea759e87f5c073936a53dbc`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship 022-S --sha 7b6c31d98ab00f1aaea759e87f5c073936a53dbc`),
and deliberation `011-DL` archived via `backlogit archive 011-DL`.

## Decision and outcome

* `internal/config`: Added capture protocol constants (`auto`,
  `ephemeral-live`, `live-with-summary`, `past-recap`), `IsValidCaptureMode`,
  lifecycle duration fields (`duration`, `idle_timeout`, `ai_summary_delay`,
  `lobby_timeout`), `queue_file`, `ExplicitCaptureAudio`, and per-meeting
  `MeetingSchedule` overrides (`at`, `mode`/`capture_mode`, `duration`,
  `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `category`,
  `capture_audio`/`audio`, `output_dir`).
* `internal/scheduler`: Added `ParseAt` (RFC 3339 and local `YYYY-MM-DD HH:MM`),
  `Entry.Validate`, and `BuildWithClock` with a single-fire `oneShotSchedule`
  adapter that skips past `at` timestamps on startup and fires future `at`
  entries once.
* `internal/browser`: Added `callEndedProbeJS()` / `shouldNotifyCallEnded`,
  throttled 1000 ms `interim: true` caption mutation heartbeats in
  `captionObserverJS` + `handleCaptionPayloadWithActivity`, `PCMRMS` /
  `HasVoiceActivity` (`DefaultVoiceRMSThreshold = 300.0`),
  `MeetingOptions.LobbyTimeout` (`ErrLobbyTimeout`), `ForceRecap`
  (`deriveRecapURLFromMeetingURL` + 2-minute `defaultForceRecapTimeout`), and
  `ActivityMonitor` callbacks.
* `internal/capture`: Extended `Session.Run` with activity-aware nominal
  `Duration`, overtime continuation while caption or PCM voice activity persists
  within `IdleTimeout` (default `3m`), early stop on `RecordCallEnded`, fallback
  `MeetingTitle` propagation, and idempotent `AppendAISummaryToFile`
  (`O_WRONLY|O_APPEND`).
* `internal/cli`: Bound `--mode`, `--duration`, `--idle-timeout`,
  `--ai-summary-delay`, `--no-ai-summary`, `--lobby-timeout`,
  `--append-ai-summary`, and `--not-before`; added post-flag
  `validateCaptureLifecycles`; implemented Idea 2C delayed AI Summary enrichment
  with immediate browser teardown and `busy` gate release, inter-process queue
  file locking (`withQueueFileLock` on `<queuePath>.lock`), and
  `splitQueueArgs`-compatible argument quoting (`quoteQueueArg`).
* Documentation: Updated `README.md`, `docs/ARCHITECTURE.md`,
  `config.example.yaml`, `docs/QUALITY_SCORE.md`, and graduated `011-DL` into
  `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`.

## Key reusable insights

* In `chromedp`, the first `chromedp.Run(ctx, ...)` call on a fresh
  `chromedp.NewContext(allocCtx)` binds the browser lifecycle to `ctx`. Never
  pass a short-lived child `context.WithDeadline` / `context.WithCancel` to that
  initial `chromedp.Navigate` call and cancel it before `EnableCaptions` /
  `CaptureAudio`, or `chromedp` will tear down the target immediately (`target closed`
  / `context canceled`).
* `splitQueueArgs` in `internal/cli/queue.go` preserves raw Windows backslashes
  inside double quotes except for `\"`, so formatting Windows paths with Go's
  `%q` (which doubles backslashes and turns a trailing `\` into `\\"`) breaks
  tokenization on trailing backslashes (`C:\Profiles\Alice\`). `quoteQueueArg`
  avoids `%q` and quotes a trailing backslash as `'\\'` (single-quoted `\`)
  immediately adjacent to the double-quoted prefix so `splitQueueArgs`
  round-trips arbitrary Windows paths losslessly.
