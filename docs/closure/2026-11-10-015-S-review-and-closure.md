---
title: Post-meeting Recap MP3 audio capture alongside transcript review and operational closure
date: 2026-11-10
feature: 015-F
shipment: 015-S
branch: feat/post-meeting-recap-mp3-audio-capture-alongside-transcript
status: ready
closure_status: READY
compaction_status: done
merge_commit: 6d66531b932aba4d6f20ed18e0dd644dc8a53a39
pr: 38
---

## Release scope

Shipment `015-S` (`015-F`, `015.001-T`, `015.002-T`, `015.003-T`, and
`015.004-T`) adds opt-in MP3 audio capture for Teams Meeting Recap URLs, written
as a same-basename `.mp3` companion next to the Markdown transcript.

* `015.001-T` (`internal/browser/recap.go`, `internal/browser/meeting_chrome.go`):
  `captureRecapAudioFromTargets` evaluates an extraction expression over CDP in
  targets that pass `isTargetInCaptureScope` for the bound owner page. The
  expression validates the recording identity (drive item, recording name,
  meeting token, thread, call, and iCal identifiers) before it fetches or decodes
  anything, decodes the recording with Web Audio, and returns 44.1 kHz mono PCM
  chunks that `internal/audio.NewEncoder` encodes to MP3. `joinHeaded` now
  persists the launched CDP port in `m.cdpPort`, and `chromeMeeting.CaptureAudio`
  routes Recap meetings to the new path. `evalInCDPTarget` reads WebSocket
  frames through `readBoundedCDPMessage`, which enforces a transport cap before
  it allocates a frame buffer.
* `015.002-T` (`internal/capture/session.go`, `internal/cli/join.go`): removes the
  Recap override that forced `captionsOnly = true`, so `SessionConfig.CaptionsOnly`
  governs both live and Recap sessions (transcript-only remains the default). Adds
  `--recap-audio`, updates the `--audio` help, applies `--recap-audio` to
  `cfg.CaptureAudio`, and forwards `--audio` and `--recap-audio` (and their `=false`
  forms) through `inheritedQueueFlags` for `--queue` runs. The obsolete
  `TestSessionRunRecapModeSkipsAudioEvenWhenCaptionsOnlyFalse` was replaced by
  `TestSessionRunRecapModeHonorsCaptionsOnlyFlag` (Rule B, authorized scope), and
  `TestRecapAudioFlagAndQueueForwarding` was added to `internal/cli/root_test.go`.
* `015.003-T` (`internal/browser/recap_retry_test.go`): consolidates the Wave-1
  browser harness (scoped capture and MP3 encoding, owned-frame identity mismatch,
  oversized-recording caps, no-audio error, headed CDP port persistence, JS identity
  and duration-before-fetch probes, and oversized/fragmented CDP frame rejection).
* `015.004-T` (`README.md`, `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`):
  documents `--audio`, `--recap-audio`, `capture_audio: true`, and `audio: true`
  for Recap MP3 capture, the identity check, and the layered size limits.

Source stash entry `DF1F461F` fed this scope under deliberation `004-DL`.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`): the multi-persona review
rubric (Constitution, Go, Correctness, Maintainability, Security, Concurrency,
Scope Boundary, Learnings) ran in-session against each task diff, without
transmitting repository content to an external reviewer model.

| Area | Finding | Disposition |
|---|---|---|
| Correctness | The extraction expression was prefixed with top-level `const` declarations. In a reused CDP execution context a second `Runtime.evaluate` would fail with a lexical redeclaration error | Fixed in-scope (P-021 C1: completes the authorized change); the constants and the expression are now wrapped in a function scope |
| Security | Recording identity is checked in the page before fetch or decode and again in Go on the returned `pageUrl`, `mediaUrl`, `itemId`, and `fileName`; a conflicting or unverifiable identity fails closed and the target is never paired with the requested transcript | No action |
| Security | `fetch` uses `redirect: 'error'`, so a media URL cannot redirect to an unvalidated location | No action |
| Security | Resource limits apply in layers: a known-duration check before fetch, a source byte cap before decode, a decoded PCM cap before allocation, and a CDP frame/message cap (`maxRecapAudioPCMBytes*2+65536`) before buffering, plus Go-side validation of `SourceBytes`, `DurationSec`, and decoded PCM size before MP3 encoding | No action |
| Concurrency | `CaptureAudio` runs concurrently with `EnableCaptions`; the Wave-2 test coordinates both so a serialized implementation would deadlock until the context deadline | No action |
| Correctness | `readBoundedCDPMessage` rejects reserved bits, fragmented control frames, a data frame inside a fragmented message, and unknown opcodes, and it answers pings and treats close as EOF | No action |
| Maintainability | The extraction expression is a single JavaScript string literal in Go, tested through Node-evaluated probes | Advisory P3; see Follow-ups |
| Scope | `015.003-T` and `015.004-T` touch only their declared files; no dependency or schema change | No action |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62RRUq`): the in-page identity check ignored `g_fileInfo.VroomItemID`, so a matching page URL could mask conflicting metadata, and a VroomItemID-only player was rejected | Fixed in-scope (P-021 C1: completes the authorized identity validation); `VroomItemID` is validated as a drive-item ID and returned in `itemId`, while generic `id`/`docId` values are still not treated as drive-item IDs. Node probes cover both cases |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62RRVc`): the 10-second DOM-probe deadline also bounded whole-recording download, decode, and serialization | Fixed in-scope (P-021 C1); audio extraction now uses `recapAudioEvalTimeout` (5 minutes) and the caller's context closes the connection on cancellation. A delayed-response test covers it |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62RRV0`): the page accepted a recording filename from `fileUrl` or a media path, but Go rejected it | Fixed in-scope (P-021 C1); `recapAudioIdentity` and `hasRecapAudioIdentityMatch` apply the same filename extraction and conflict checks to page and media URLs |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62RRWK`): `--recap-audio` took precedence over a per-entry `--audio=false` in queue runs | Fixed in-scope (P-021 C1); the two flags share one value so the last argument wins, and a mixed-alias queue regression test covers it |
| Correctness | Copilot round 2 (thread `PRRT_kwDOTStvas62RjE5`): with `captions --audio` and no URL, the requested URL stays the default Teams home URL after the operator selects a Recap, so its identity is empty and audio extraction always failed | Fixed in-scope (P-021 C1: completes the authorized audio path); `retainRecapAudioIdentity` keeps the selected Recap page URL when the requested URL has no recording identity, and a requested identity always stays authoritative. Tests cover retention, a matching selection, and a different recording |
| Scope | Copilot round 1 (thread `PRRT_kwDOTStvas62RRVH`): the in-session review fallback in `_ship.agent.md` and `adversarial-review.agent.md` bypasses the insufficient-reviewer halt | Out of scope (P-021 C1: harness review-gate policy, a different contract surface); captured as deferred entry `680A62E6`, no code change made |

Blocking findings: `P0=0, P1=0, P2=0, P3=1`. Outcome: `READY_WITH_FOLLOWUPS`.

## Follow-ups

* Deferred scope expansion `680A62E6` (P-021 C2, provisional priority medium,
  requires deliberation): reconcile the harness review fallback so the in-session
  single-agent review is an explicitly authorized degraded outcome that does not
  count as consensus, or preserve the adversarial-review insufficient-reviewer halt.
  Captured from PR #38 Copilot thread `PRRT_kwDOTStvas62RRVH`. Stage owns triage.
* Accepted residual risk (approved layered safety contract): Chromium's
  `decodeAudioData` does not expose a pre-decode PCM size for unknown-duration
  streams, so decode expansion inside the 64 MiB compressed-source cap can use
  browser tab memory before the post-decode check. The risk is isolated to the
  browser tab process.
* Residual-risk note (advisory P3): the Node-based probe tests run `node` and
  therefore need it on the path (the GitHub-hosted CI runner provides it). Keep
  the probes in step with
  `extractRecapAudioJS` when its limits or identity rules change. No stash entry
  is created because the probes are the planned verification shape of
  `015.003-T`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Red then green | `TestSessionRunRecapModeHonorsCaptionsOnlyFlag` failed (`CaptureAudio called = false, want true`) before the `session.go` change and passed after it | PASS |
| Format | `gofmt -l .` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` (all packages) at the end of every task | PASS |
| Full build | `go build ./...` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` | PASS |

Live Teams Recap qualification (real SharePoint recording, tenant policy, and
audio fidelity) is not covered by the synthetic fixtures and remains an operator
check (see Monitoring).

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Add CDP Recap audio extraction that fetches the Recap recording with page credentials | medium | Opt-in only (`--audio`, `--recap-audio`, `capture_audio`); identity check fails closed; `redirect: 'error'`; layered size caps; no credential value is logged or returned | applied |
| Remove the Recap `captionsOnly` override in `capture.Session` | low | Default (`CaptionsOnly: true`) still writes only the `.md`; covered by a two-case test | applied |
| Add `--recap-audio` and queue forwarding | low | Additive flag; `--audio` behavior unchanged | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Recap MP3 capture | READY_WITH_CONDITIONS | Run one live capture of an authorized Recap with audio enabled and confirm the `.mp3` plays and matches the Recap recording |
| Transcript-only default | READY | Without an audio option, Recap output is unchanged |
| Resource limits | READY | A recording over 64 MiB, 1800 s, or 128 MiB of PCM is rejected with `recap audio exceeds maximum supported size/duration` |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the first three authorized Recap captures run with audio enabled.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Recap `.mp3` is written and plays | Output directory and a local player | One playable `.mp3` per audio-enabled Recap | Missing, empty, or wrong-recording file |
| Identity rejections | CLI error output | None for a matching Recap URL | `recording identity mismatch` or `unverified` on a correct Recap URL |
| Resource-limit rejections | CLI error output | None for normal meetings | Repeated `exceeds maximum supported size/duration` on recordings under the caps |

## Rollback

Trigger rollback on a wrong-recording `.mp3`, a hang or memory exhaustion in the
browser tab during Recap audio capture, or a regression in transcript-only Recap
output. Revert the `015-S` merge commit on `main`. Operators can avoid the
feature without a revert by omitting `--audio`, `--recap-audio`, and
`capture_audio`. No data migration is involved.

## Post-merge closure

PR #38 merged to `main` at `6d66531b932aba4d6f20ed18e0dd644dc8a53a39`
(`2026-11-11T17:11:53Z`) with a merge commit (P-009). Dark Factory Mode (P-017)
post-merge closure ran on `post-merge/015-recap-mp3-audio-capture`:

* Copilot shadow review ran two rounds on PR #38. Round 1 raised five threads:
  four were fixed in scope under P-021 C1 in `ce471b1` (VroomItemID identity,
  audio evaluation timeout, recording-filename identity, and `--recap-audio` /
  `--audio` precedence), and one (harness review-gate policy) was out of scope,
  captured as deferred entry `680A62E6`, answered with a reply citing that ID, and
  resolved with no code change. Round 2 raised one thread, fixed in scope in
  `1d88d42` (retain the selected Recap identity for interactive selection). All six
  threads were replied to and resolved. `autoharness gate copilot-review` returned
  `SATISFIED` for the merged HEAD `1d88d42` and CI passed.
* Verified `MERGE_CONFIRMED` via `gh pr view 38` (`state: MERGED`) and
  `git merge-base --is-ancestor 6d66531b932aba4d6f20ed18e0dd644dc8a53a39
  origin/main`.
* `autoharness gate pipeline-topology --phase lifecycle --shipment 015-S` passed
  on the closure branch.
* Pre-close reconciliation: the four manifest tasks and `015-F` were already
  archived with declared `status: done` (backlogit relocates a record on
  `move --status done`), no orphan declared `shipment_id: 015-S`, and the shipment
  record was `active`.
* P-015 fully-covered-root verification: `015-F` is a root feature whose only
  descendants, `015.001-T` to `015.004-T`, are all `015-S` manifest members, so the
  classification was `CASCADE`. `backlogit shipment ship 015-S --sha
  6d66531b932aba4d6f20ed18e0dd644dc8a53a39` returned `archived_ids: [015.001-T,
  015.002-T, 015.003-T, 015.004-T, 015-F, 015-S]` and `returned_ids: []`. All four
  tasks kept `parent_id: 015-F`, `015-S` carries `archived_status: shipped`, and
  no archive file showed as a working-tree deletion (P-007).
* Source artifact cleanup: deliberation `004-DL` (named only in the `015-F`
  description and `source_deliberation_id`) was retained by the engine, so Ship
  archived it with `backlogit archive 004-DL`. Stash entry `DF1F461F` was already
  archived by Stage.
* Mandatory P-020 `compact-context` (`target: all`) ran: the session memory was
  compacted to
  `docs/memory/compacted/2026-11-10-015-S-recap-mp3-audio-capture-compacted.md`
  and the verbose original archived under `docs/archive/memory/2026-11-10/`.
  Recorded as `compaction_status: done` above (initialized `pending` in the
  feature PR and finalized here).

## Handoff

Related [decision](../decisions/2026-11-10-recap-mp3-audio-capture-deliberation.md)
and [plan](../plans/2026-11-10-recap-mp3-audio-capture-plan.md).
