---
title: Ship session memory for 015-S Recap MP3 audio capture
date: 2026-11-10
shipment: 015-S
feature: 015-F
agent: ship
status: in-progress
---

## Completed

* `015.001-T`: CDP Recap audio extraction, identity validation, layered resource
  limits, and the `CaptureAudio` Recap branch in `internal/browser/recap.go` and
  `internal/browser/meeting_chrome.go`.
* `015.002-T`: Recap override removed from `capture.Session`; `--recap-audio` and
  queue forwarding added in `internal/cli/join.go`; Wave-2 harnesses merged into
  `internal/capture/session_test.go` and `internal/cli/root_test.go`.
* `015.003-T`: browser harness consolidated into
  `internal/browser/recap_retry_test.go`.
* `015.004-T`: README and capture design doc updated.

## Decisions and rationale

* The review gate ran as a single-agent declared degradation because external
  reviewer routes could have transmitted repository code to third parties.
* The extraction expression's `const` preamble is wrapped in a function scope so a
  repeated `Runtime.evaluate` in the same CDP context cannot fail on redeclaration.
* A task moved to `done` (and the feature moved to `done`) relocates its record
  to `.backlogit/archive/`; the P-015 cascade close then has to cope with
  already-archived members.
* `compaction_status` starts `pending` in the closure artifact and is set to `done`
  only after P-020 compaction runs post-merge.

## Residual risk

Browser decode expansion of unknown-duration streams inside the 64 MiB source cap
is accepted by the approved layered safety contract.
