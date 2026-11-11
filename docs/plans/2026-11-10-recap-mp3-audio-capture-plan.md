---
title: Post-meeting Recap MP3 audio capture alongside transcript plan
description: Implementation plan for 015-F / 015-S covering post-meeting Recap MP3 audio capture via CDP Web Audio extraction and pure-Go shine-mp3 encoding
source: docs/decisions/2026-11-10-recap-mp3-audio-capture-deliberation.md
deliberation_id: 004-DL
feature: 015-F
shipment: 015-S
date: 2026-11-10
status: approved
tags:
  - audio
  - recap
  - browser
  - cli
  - capture
---

## Problem Frame

Following the accepted deliberation in
[2026-11-10-recap-mp3-audio-capture-deliberation.md](../decisions/2026-11-10-recap-mp3-audio-capture-deliberation.md)
(`004-DL`), stash entry **`DF1F461F`** is planned under feature **`015-F`** and
shipment **`015-S`**:

1. **`internal/browser/recap.go` & `internal/browser/meeting_chrome.go`**:
   When `m.IsRecap()` is `true` and `(*chromeMeeting).CaptureAudio(ctx, w)` is
   invoked, `chromeMeeting` must extract post-meeting audio from the bound Recap
   CDP targets (`isTargetInCaptureScope(t, m.capturePageID, ownedFrames)`),
   validate that the owned target/frame's `t.URL` and media source URL / page
   metadata match the expected meeting recording identity via
   `recapExpectedIdentifiers(meetingURL)` (`targetMatchesIdentifiers` /
   `payloadMatchesIdentifiers` matching `parseRecapPayloadForMeeting` when
   `recapExpectedIdentifiers(meetingURL)` is non-empty), enforce bounded
   decoding and transfer resource limits (`maxRecapAudioSourceBytes = 64 MiB`,
   `maxRecapAudioDurationSec = 1800`, `maxRecapAudioPCMBytes = 128 MiB`) before
   allocating unbounded PCM/base64 buffers, decode base64 little-endian mono
   44.1 kHz `int16` PCM chunks via `decodeAudioPayload`, and encode them to MP3
   via `audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` + `Flush()`),
   completing finitely once all Recap audio frames are flushed. In addition,
   `joinHeaded` must store `m.cdpPort = port` so post-`Join` CDP target lookup
   works in both headed and headless modes.
2. **`internal/capture/session.go`, `internal/capture/session_test.go`, & `internal/cli/join.go`**:
   `Session.Run` in `internal/capture/session.go` currently forces
   `captionsOnly = true` whenever `m.(browser.RecapMeeting).IsRecap()` is
   `true`. Removing that override (and updating
   `TestSessionRunRecapModeSkipsAudioEvenWhenCaptionsOnlyFalse` in
   `internal/capture/session_test.go` in the same task so `go test ./...`
   passes at the end of `015.002-T`) allows `s.cfg.CaptionsOnly`
   (`!cfg.CaptureAudio`) to control Recap audio capture: when `--audio` or
   `--recap-audio` (or `capture_audio: true` / `audio: true` in `config.yaml`)
   is set, `setupOutput` creates both the `.md` transcript file and the
   companion `.mp3` file and runs `m.CaptureAudio(runCtx, out.Audio)`
   concurrently with `m.EnableCaptions(runCtx, sink)`; when audio capture is not
   enabled (default), `CaptionsOnly` remains `true` and only the `.md`
   transcript is created.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Extract post-meeting Recap audio from scoped CDP targets validated against `recapExpectedIdentifiers(meetingURL)`, enforce bounded PCM/source resource limits (`maxRecapAudioSourceBytes`, `maxRecapAudioDurationSec`, `maxRecapAudioPCMBytes`), and encode to MP3 via `audio.NewEncoder` in both headed and headless modes | `DF1F461F` | U1 (`015.001-T`), U3 (`015.003-T`) | `go test -race ./internal/browser/...` |
| R2: Enable Recap MP3 capture via `--audio` / `--recap-audio` CLI flags and `capture_audio: true` / `audio: true` config options while preserving transcript-only default and updating `session_test.go` in the same unit | `DF1F461F` | U2 (`015.002-T`) | `go test -race ./internal/capture/... ./internal/cli/...` |
| R3: Document Recap MP3 audio capture in user and architecture documentation | `DF1F461F` | U4 (`015.004-T`) | Documentation review and `backlogit doctor` |

## Implementation Units

### U1 (`015.001-T`) — Add CDP Recap audio extraction and `chromeMeeting.CaptureAudio` recap branch

* **Domain:** code (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/browser/recap.go`, add configurable resource-limit package
     variables (`maxRecapAudioSourceBytes = 64 * 1024 * 1024` / 64 MiB
     compressed media blob cap, `maxRecapAudioDurationSec = 1800`, and
     `maxRecapAudioPCMBytes = 128 * 1024 * 1024` decoded PCM cap),
     `extractRecapAudioJS`, and
     `captureRecapAudioFromTargets(ctx context.Context, cdpPort int, boundTargetID string, meetingURL string, w io.Writer) error`:
     * Queries `listCDPTargets(cdpPort)`, resolves the owning page target and
       its `ownedFrames` via `fetchOwnedFrames`, and filters targets with
       `isTargetInCaptureScope(t, boundTargetID, ownedFrames)` so unrelated open
       browser tabs are never accessed.
     * Validates expected recording identity before extracting audio: computes
       `expectedIDs := recapExpectedIdentifiers(meetingURL)` (matching the
       transcript path's `targetMatchesIdentifiers` /
       `payloadMatchesIdentifiers` and `parseRecapPayloadForMeeting`
       discipline). When `expectedIDs` is non-empty, skips any owned
       target/frame whose `t.URL` conflicts with `meetingURL` or whose
       extracted page metadata / media source URL (`pageUrl`, `fileName`,
       `itemId`, `mediaUrl`) fails to match `expectedIDs`, ensuring an owned
       frame displaying or navigating to a different meeting recording is never
       paired with the requested meeting's transcript.
     * Enforces bounded decoding and transfer in `extractRecapAudioJS` and
       `captureRecapAudioFromTargets`:
       1. Checks for pre-supplied base64 little-endian `int16` mono PCM on
          `[data-recap-audio-pcm]` (`getAttribute('data-recap-audio-pcm')`) or
          `window.g_fileInfo.audioPcmBase64`, validating decoded byte length
          against `maxRecapAudioPCMBytes` before allocating unbounded buffers.
       2. Otherwise inspects `<audio>` and `<video>` elements (`currentSrc`,
          `src`, `querySelector('source')?.src`, `getAttribute('data-audio-src')`)
          and `window.g_fileInfo` (`audioUrl`, `mediaUrl`, `downloadUrl`,
          `streamUrl`), fetches the audio resource within the authenticated
          target origin (`fetch(mediaURL, { credentials: 'include' })`), checks
          `Content-Length` and the streamed reader byte count against
          `maxRecapAudioSourceBytes` BEFORE calling `decodeAudioData`, checks
          `audioBuffer.duration` (`<= maxRecapAudioDurationSec`) and
          `audioBuffer.length * 2` (`<= maxRecapAudioPCMBytes`) BEFORE
          allocating the `Int16Array` / base64 chunks, resamples and downmixes
          to 44100 Hz mono in bounded windows, and returns a structured error
          (`recap audio exceeds maximum supported size/duration`) whenever any
          cap is exceeded.
     * Decodes each base64 PCM chunk in Go using `decodeAudioPayload` (enforcing
       cumulative decoded bytes `<= maxRecapAudioPCMBytes`) and streams the
       `[]int16` samples through
       `enc := audio.NewEncoder(w, audio.SampleRate)` (`enc.WritePCM(samples)`
       followed by `enc.Flush()`). Returns an actionable error if a resource
       cap is exceeded or no scoped target with matching recording identity
       yields audio data.
  2. In `internal/browser/meeting_chrome.go`:
     * Set `m.cdpPort = port` in `joinHeaded` after `launchCleanHeadedBrowser`
       succeeds so `m.cdpPort` is populated in both headed and headless Recap
       sessions.
     * In `(*chromeMeeting).CaptureAudio(ctx context.Context, w io.Writer)`,
       after checking `w != nil`, branch on `if m.IsRecap()` to invoke
       `m.captureRecapAudio(ctx, w)` (calling
       `captureRecapAudioFromTargets(ctx, m.cdpPort, m.capturePageID, m.meetingURL, w)`)
       and return once the MP3 stream is flushed.
* **Files (2 files):** `internal/browser/recap.go`,
  `internal/browser/meeting_chrome.go`.
* **Verify:** `go build ./...` and `go vet ./...` pass; unit tests in U3
  exercise `captureRecapAudioFromTargets` and `chromeMeeting.CaptureAudio`.
* **Depends on:** none.

### U2 (`015.002-T`) — Wire Recap audio option in `capture.Session` and `internal/cli`

* **Domain:** code (Go `internal/capture` and `internal/cli`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/capture/session.go` (`Session.Run`), remove the lines:
     ```go
     if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() {
         captionsOnly = true
     }
     ```
     so `captionsOnly := s.cfg.CaptionsOnly` governs both live calls and
     Meeting Recaps (default `CaptionsOnly: true` when `--audio` /
     `--recap-audio` / `capture_audio` is not set; `CaptionsOnly: false` when
     audio capture is enabled).
  2. In `internal/capture/session_test.go` (updated in this behavior-changing
     unit so `go test ./...` passes at the end of `015.002-T`):
     * Replace `TestSessionRunRecapModeSkipsAudioEvenWhenCaptionsOnlyFalse` with
       `TestSessionRunRecapModeHonorsCaptionsOnlyFlag`:
       * Subtest `"captions only true skips audio in recap mode"`: with
         `CaptionsOnly: true` and `isRecap: true`, asserts `CaptureAudio` is not
         called and only the `.md` file is written.
       * Subtest `"captions only false captures recap audio alongside transcript"`:
         with `CaptionsOnly: false` and `isRecap: true` (`fakeMeeting.CaptureAudio`
         writing synthetic MP3 bytes and returning `nil` when `f.isRecap` is
         `true`), asserts both `.md` (with transcript + AI Summary) and `.mp3`
         files are created in `OutputDir` and `calls()` includes `"join"`,
         `"captions"`, and `"audio"`.
  3. In `internal/cli/join.go`:
     * Add `recapAudio bool` to `captureFlags`.
     * Bind `--recap-audio` (`"also capture Meeting Recap audio to an MP3 file"`)
       and update `--audio` description (`"also capture meeting or recap audio to an MP3 file"`)
       in `bindCaptureFlags`.
     * In `applyCaptureFlags`, if `cmd.Flags().Changed("recap-audio")`, set
       `cfg.CaptureAudio = f.recapAudio` (and if `f.recapAudio` or `f.audio` is
       true when changed, enable `cfg.CaptureAudio`).
     * In `inheritedQueueFlags`, forward `--recap-audio` / `--recap-audio=false`
       when `cmd.Flags().Changed("recap-audio")`.
* **Files (3 files):** `internal/capture/session.go`,
  `internal/capture/session_test.go`, `internal/cli/join.go`.
* **Verify:** `go test -race ./internal/capture/... ./internal/cli/...` and
  `go test -race ./...` pass completely.
* **Depends on:** U1 (`015.001-T`).

### U3 (`015.003-T`) — Add deterministic browser CDP unit tests for Recap MP3 audio capture, identity validation, and resource limits

* **Domain:** tests (Go unit tests in `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/browser/recap_retry_test.go`, add:
     * `TestCaptureRecapAudioScopedToOwningMeetingPageAndEncodesMP3`: sets up a
       loopback CDP HTTP/WebSocket fixture with an unowned tab (`other-tab`) and
       the bound `target-tab` + `target-iframe` (`Page.getFrameTree`) matching
       `m.meetingURL`, serves deterministic synthesized mono `int16` PCM
       (base64-encoded), calls `m.CaptureAudio(ctx, &buf)` on a recap
       `chromeMeeting` (`len(m.recapEntries) > 0`), and asserts that only the
       scoped target is used, `CaptureAudio` returns `nil` finitely, and
       `buf.Bytes()` is a non-empty valid MP3 stream starting with the MPEG
       frame sync header (`0xFF` and `(b[1]&0xE0) == 0xE0`).
     * `TestCaptureRecapAudioRejectsOwnedFrameWithMismatchedRecordingIdentity`:
       sets up an owned frame (`parentId == m.capturePageID`) whose URL / media
       source metadata belongs to a different meeting recording (mismatching
       `recapExpectedIdentifiers(m.meetingURL)`), and asserts that
       `m.CaptureAudio(ctx, &buf)` rejects the mismatched owned frame and never
       pairs another meeting's audio with the requested meeting.
     * `TestCaptureRecapAudioRejectsOversizedRecordingExceedingResourceCap`:
       verifies that when a Recap recording exceeds the configured resource limit
       (`maxRecapAudioSourceBytes` / `maxRecapAudioDurationSec` /
       `maxRecapAudioPCMBytes`), `m.CaptureAudio(ctx, &buf)` rejects it before
       unbounded PCM/base64 allocation and returns a clear error containing
       `recap audio exceeds maximum supported size/duration`.
     * `TestCaptureRecapAudioReturnsErrorWhenNoAudioAvailable`: verifies
       `m.CaptureAudio(ctx, &buf)` on a recap `chromeMeeting` returns a
       descriptive error when no scoped target provides recap audio.
* **Files (1 file):** `internal/browser/recap_retry_test.go`.
* **Verify:** `go test -race ./internal/browser/...` passes with 0 failures.
* **Depends on:** U1 (`015.001-T`), U2 (`015.002-T`).

### U4 (`015.004-T`) — Document Recap MP3 audio capture in `README.md` and capture design doc

* **Domain:** docs (Markdown). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **What:**
  1. In `README.md`, update the `--audio` flag description and add
     `--recap-audio` / `capture_audio: true` notes under `### Compiled executable and explicit output file`
     and `### Headless automation` explaining that passing `--audio` (or
     `--recap-audio` / `capture_audio: true`) during Meeting Recap capture
     writes a companion `.mp3` file alongside the `.md` transcript.
  2. In `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`,
     update `## Meeting Recap path` and `## CLI and output contract` to document
     optional post-meeting Recap MP3 audio extraction over CDP and encoding via
     `internal/audio.NewEncoder`.
* **Files (2 files):** `README.md`,
  `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`.
* **Verify:** Markdown linting / formatting rules (`markdown.instructions.md`)
  and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` pass.
* **Depends on:** U2 (`015.002-T`).

## Dependency Graph

```text
U1 (015.001-T: browser recap audio extraction, identity check & resource caps)
  └──> U2 (015.002-T: capture.Session, session_test.go & CLI flag wiring)
         ├──> U3 (015.003-T: browser CDP recap audio, identity & resource-cap tests)
         └──> U4 (015.004-T: README & design doc update)
```

* Shipment `015-S` is chained after `014-S` via
  `backlogit dep add 015-S 014-S --type blocks`.

## Decisions and Rationale

* **Pure-Go MP3 encoding via `internal/audio.NewEncoder`**: Reuses the existing
  `shine-mp3` wrapper with 1152-sample frame buffering and `encoderTailSlack = 4`
  so zero external binaries (`ffmpeg`) are required and `go test -race ./...`
  remains `checkptr`-safe.
* **Opt-in only (`CaptionsOnly: !cfg.CaptureAudio`)**: Default Recap invocations
  without `--audio`, `--recap-audio`, or `capture_audio: true` continue to write
  only the `.md` transcript and AI summary, preserving backward compatibility
  with all existing CLI and compiled-binary tests.

## Risks and Caveats

* **Cross-origin SharePoint media URLs**: Because `extractRecapAudioJS` runs
  inside each scoped CDP target (including the `xplatplugins.aspx` /
  `streamembed.aspx` SharePoint iframe), `fetch` executes in the iframe's own
  authenticated origin. If a tenant policy blocks raw media fetch or no audio
  track exists, `CaptureAudio` surfaces a clear error.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Explicit error propagation, bounded CDP timeouts, bounded PCM/media resource caps, zero `golangci-lint` / `go vet` findings |
| II: Test first | Deterministic unit tests in `recap_retry_test.go` and `session_test.go` run under `go test -race ./...` |
| III, IV: Secrets & containment | Scoped strictly to `isTargetInCaptureScope` + `recapExpectedIdentifiers(meetingURL)` identity check; no URLs with passcodes or tokens logged |
| VI: Single responsibility | Four tasks (`<= 3` files each) strictly isolated by domain (`code`, `code`, `tests`, `docs`) |
| VII, VIII: Strict safety | Browser CDP evaluation and file creation classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `004-DL`, plan, feature `015-F`, tasks `015.001-T`–`015.004-T`, and shipment `015-S` (`blocks 014-S`) recorded |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: extends CLI flags
  (`--recap-audio` and `--audio` behavior on Recap URLs) and `capture.Session`
  Recap audio contract.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  evaluates CDP JavaScript inside authenticated Teams/SharePoint Recap targets.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**.
* External integration, operator checkpoint, or external dependency — **absent**
  (uses existing CDP WebSocket and `internal/audio` packages).
* High runtime, rollout, or rollback risk — **absent** (opt-in behind `--audio`
  / `--recap-audio` / `capture_audio: true`).

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 and U2 extend the CLI/capture contract for
Meeting Recap URLs and execute CDP evaluations in authenticated browser targets.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md`
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md`
  * `docs/closure/2026-11-09-011-S-review-and-closure.md`
  * `docs/closure/2026-11-10-012-S-review-and-closure.md`

### Protected Invariants

1. **Default Recap behavior unchanged**: When neither `--audio`,
   `--recap-audio`, `capture_audio: true`, nor `audio: true` is active,
   `SessionConfig.CaptionsOnly` is `true`, no `.mp3` file is created, and
   `CaptureAudio` is not invoked.
2. **Strict tab/frame scope isolation and recording-identity validation**:
   `captureRecapAudioFromTargets` MUST filter CDP targets through
   `isTargetInCaptureScope(t, boundTargetID, ownedFrames)` AND validate expected
   recording identity via `recapExpectedIdentifiers(meetingURL)`
   (`targetMatchesIdentifiers` / `payloadMatchesIdentifiers`) before extracting
   audio, skipping owned frames whose URL or media source mismatches the
   requested meeting.
3. **Bounded memory and resource limits**: `extractRecapAudioJS` and
   `captureRecapAudioFromTargets` MUST enforce `maxRecapAudioSourceBytes` (`64 MiB`),
   `maxRecapAudioDurationSec` (`1800`), and `maxRecapAudioPCMBytes` (`128 MiB`)
   before allocating unbounded PCM/base64 buffers, rejecting oversized
   recordings with `recap audio exceeds maximum supported size/duration`.
4. **`shine-mp3` frame and `checkptr` safety**: All PCM samples are encoded
   exclusively through `audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` +
   `Flush()`).

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Evaluate `extractRecapAudioJS` over CDP on scoped Recap targets in `internal/browser/recap.go` | moderate | Restricted to targets passing `isTargetInCaptureScope(t, boundTargetID, ownedFrames)` and `recapExpectedIdentifiers(meetingURL)` identity check, bounded by `maxRecapAudioSourceBytes` / `maxRecapAudioDurationSec` / `maxRecapAudioPCMBytes` and per-call context timeouts; never logs media URLs or tokens | Extracts mono 44.1 kHz `int16` PCM from the matching Recap recording and encodes valid MP3 frames via `audio.NewEncoder` |
| Enable `.mp3` file creation and `CaptureAudio` during Recap sessions when `CaptionsOnly` is `false` (`internal/capture/session.go`, `internal/cli/join.go`) | low | Opt-in only via `--audio`, `--recap-audio`, or `capture_audio: true`; uses `0o600` file permissions and existing path-collision guards in `setupOutput` | Writes companion `.mp3` alongside `.md` transcript when opted in; leaves default transcript-only Recap workflow untouched |

## Runtime Verification and Closure

* **Verification**:
  * `go test -race ./...` (including new unit tests in `internal/browser` and
    `internal/capture`)
  * `go vet ./...` and `go vet -tags=integration ./...`
  * CI-pinned `golangci-lint run` (`v1.64.8` with `GOTOOLCHAIN=go1.26.0`)
  * `gofmt -l cmd internal`
  * Compiled binary acceptance check (`TestCompiledBinaryCapturesRecapToExplicitOutput`)
* **Operational closure**:
  * Monitor `.mp3` file creation and non-zero frame output when `--audio` /
    `--recap-audio` is supplied on a Meeting Recap URL.
  * **Rollback**: Pass `--audio=false` (or omit `--audio` / `--recap-audio`) as
    an immediate no-revert mitigation, or revert the `015-S` merge commit on
    `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — All 4 units obey the 2-hour rule (`<= 3` files each), width isolation, test-first verification, and pure-Go stdlib/existing-dependency constraints. | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Storing `m.cdpPort = port` in `joinHeaded` closes the headed vs. headless CDP port gap; `session_test.go` is updated in U2 (`015.002-T`) so `go test ./...` passes at the end of U2; `fakeMeeting.CaptureAudio` in `session_test.go` returns finitely when `f.isRecap` is true so concurrent `wg.Wait()` in `Session.Run` does not deadlock. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly bounded to `DF1F461F`; no unrelated refactoring or dependency additions. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Incorporates `shine-mp3-frame-alignment.md` and `shine-mp3-race-checkptr.md` by routing all PCM encoding through `audio.NewEncoder`. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Preserves package boundaries (`internal/browser` extracts and encodes to `io.Writer`; `internal/capture` manages `.md`/`.mp3` files; `internal/cli` binds flags). | PASS |
| Security Lens Reviewer (CDP & media surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Enforces `isTargetInCaptureScope` tab/frame isolation, `recapExpectedIdentifiers(meetingURL)` recording-identity validation, bounded media/PCM resource caps, and `0o600` output file permissions. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `015-F` and shipment `015-S`.
