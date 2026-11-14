---
title: Ship session memory for 022-S (Scheduled and protocol-driven live call capture)
date: 2026-11-13
agent: ship
status: complete
shipment: 022-S
feature: 022-F
deliberation: 011-DL
pull_request: 55
merge_commit: 7b6c31d98ab00f1aaea759e87f5c073936a53dbc
---

## Session summary

Executed a complete Dark Factory Mode (`DARK_MODE_ACTIVE`) Ship session for
shipment **`022-S`** (*Scheduled and protocol-driven live call capture (`R1`–`R15`)*,
covering feature **`022-F`**, deliberation **`011-DL`**, source stash
**`77FC68CA`**).

* **Tasks completed (11/11)**:
  * `022.001-T` (`4e07c78`, `fa8b0a9`, `f1521f3`): `internal/config` capture
    modes, lifecycle durations, `queue_file`, per-meeting overrides, and
    validation.
  * `022.002-T` (`52eaa69`): `internal/scheduler` `ParseAt`, `Entry.Validate`,
    and `BuildWithClock` with `oneShotSchedule`.
  * `022.003-T` (`26e70e5`): `internal/browser` `callEndedProbeJS` /
    `shouldNotifyCallEnded`, throttled 1000 ms `interim: true` caption mutation
    heartbeats, and `PCMRMS` / `HasVoiceActivity`.
  * `022.004-T` (`d037846`, `f1521f3`): `internal/browser` `LobbyTimeout`
    (`ErrLobbyTimeout`), `ForceRecap` (`deriveRecapURLFromMeetingURL`),
    `ActivityMonitor`, and live call-end polling.
  * `022.005-T` (`96a8a53`): `internal/browser` unit tests for PCM RMS voice
    activity, interim captions, call-end probes, `ForceRecap`, and lobby
    timeout.
  * `022.006-T` (`28e78b5`): `internal/capture` activity-aware nominal
    `Duration`, overtime continuation, `IdleTimeout` / `RecordCallEnded`
    shutdown, fallback `MeetingTitle`, and `AppendAISummaryToFile`.
  * `022.007-T` (`5982fb4`): `internal/capture` deterministic unit tests.
  * `022.008-T` (`4831b6a`, `f1521f3`, `faded41`): `internal/cli` capture
    protocol and lifecycle flags, inter-process queue locking
    (`withQueueFileLock`), and `quoteQueueArg` quoting.
  * `022.009-T` (`fa8b0a9`, `f1521f3`): `internal/cli` Idea 2C delayed AI
    Summary workflow, `validateCaptureLifecycles`, and `schedule` per-meeting
    overrides and `busy` gate release.
  * `022.010-T` (`ca9874d`, `f1521f3`, `faded41`): `internal/cli` unit and
    integration tests.
  * `022.011-T` (`59f99aa`): `README.md`, `docs/ARCHITECTURE.md`, and
    `config.example.yaml` documentation and Windows Task Scheduler recipes.
* **PR #55**: Merged to `main` via merge commit
  `7b6c31d98ab00f1aaea759e87f5c073936a53dbc` after local review (`READY`),
  addressing and resolving 7 Copilot review threads across 2 rounds (`P-018`
  `copilot-review` gate `SATISFIED`), and passing all GitHub Actions CI checks.
* **Post-merge closure**: Archived `022-S` (`shipped`), `022-F` (`done`),
  `022.001-T`..`022.011-T` (`done`), and `011-DL` (`archived`), wrote
  `docs/closure/2026-11-13-022-S-review-and-closure.md`, graduated `011-DL`
  into `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`,
  and compacted session memory (P-020).
