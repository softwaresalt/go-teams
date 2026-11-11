---
type: memory-compacted
date: 2026-11-10
shipment: 015-S
feature: 015-F
source: docs/archive/memory/2026-11-10/015-S-recap-mp3-audio-capture-memory.md
---

# Compacted memory - Shipment 015-S (Recap MP3 audio capture)

**Shipped**: PR #38, merge `6d66531b932aba4d6f20ed18e0dd644dc8a53a39`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship`). Source deliberation `004-DL` was archived by the
explicit source-artifact cleanup step because the engine retains deliberations
that are only mentioned in a feature description.

## Decision and outcome

* Meeting Recap audio is opt-in (`--audio`, `--recap-audio`, `capture_audio: true`,
  `audio: true`). `capture.Session` no longer forces `captionsOnly` for Recaps, so
  the default is still a transcript-only `.md` and the opt-in adds a same-basename
  `.mp3`.
* `captureRecapAudioFromTargets` evaluates an extraction expression over CDP in
  targets owned by the bound Recap page, validates the recording identity in the
  page before fetching and again in Go, decodes with Web Audio, and encodes MP3
  through `internal/audio.NewEncoder`. Size limits apply in layers (known duration
  before fetch, source bytes before decode, PCM size before allocation, and a CDP
  frame/message cap before buffering). Decode expansion of unknown-duration
  streams inside the 64 MiB source cap is an accepted residual risk.

## Key reusable insights

* Do not prefix a CDP `Runtime.evaluate` expression with top-level `const`
  declarations: a second evaluation in the same context fails on redeclaration.
  Wrap the constants and the expression in a function scope.
* `evalInCDPTarget` DOM probes need a short deadline, but whole-recording
  extraction needs its own budget and must close the socket when the caller's
  context is cancelled.
* Validate recording identity with the same extraction rules on both sides of the
  page/Go boundary (drive-item ID including `VroomItemID`, recording filename from
  `fileUrl` or a media path), and retain the selected Recap's identity during an
  interactive join, otherwise the check can never succeed.
* Two flag spellings for one option must share one value so the last argument wins;
  setting the config field from each flag in turn lets one alias override a
  per-entry queue opt-out.
* A task or feature moved to `done` relocates to `.backlogit/archive/`, so the
  cascade close sees already-archived members; the classifier and
  `returned_ids: []` check still hold.
* PowerShell: write reply bodies to a BOM-less file and pass `--field body=@file`;
  index an array inside a native-command argument as `$($a[$i])` or loop variables.

## Review outcome

Local review: `READY_WITH_FOLLOWUPS`, `P0=0, P1=0` (single-agent declared
degradation). Copilot raised six threads over two rounds on PR #38: five fixed in
scope under P-021 C1 (`ce471b1`, `1d88d42`) and one out of scope (harness review
fallback policy) captured as deferred entry `680A62E6`. The `copilot-review` gate
returned `SATISFIED` and CI passed before the merge commit.

## Post-merge closure summary

P-015 classification returned `CASCADE`. `backlogit shipment ship` returned
`archived_ids: [015.001-T, 015.002-T, 015.003-T, 015.004-T, 015-F, 015-S]` and
`returned_ids: []`; all tasks kept `parent_id: 015-F`. Stash `DF1F461F` was already
archived by Stage.
