---
title: Post-meeting Recap MP3 audio capture decision
description: Deliberation on capturing post-meeting Recap audio to MP3 alongside the Markdown transcript via CLI flag or config option
topic: Post-meeting Recap MP3 audio capture (DF1F461F)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 004-DL
feature: 015-F
shipment: 015-S
source_stash:
  - DF1F461F
linked_artifacts:
  - docs/plans/2026-11-10-recap-mp3-audio-capture-plan.md
  - .backlogit/queue/004-DL.md
  - .backlogit/queue/015-F.md
  - .backlogit/queue/015-S.md
date: 2026-11-10
status: accepted
tags:
  - audio
  - recap
  - browser
  - cli
---

## Problem Frame

Stash entry **`DF1F461F`** (`feature`, `medium`) requests:

> Add feature to capture the MP3 audio from a meeting that has already happened
> along with the transcript as a flag or configuration driven option in the
> transcript retrieval/capture workflow.

Currently, `teams-cli` supports `--audio` on the root, `captions`, and `join`
commands and `capture_audio: true` (alias `audio: true`) in `config.yaml`, and
`internal/capture/output.go` already pairs a `.mp3` destination file alongside
the `.md` transcript file whenever `OutputConfig.CaptionsOnly` is `false`.
However, `internal/capture/session.go` (`Session.Run`) unconditionally forces
`captionsOnly = true` whenever `m.(browser.RecapMeeting).IsRecap()` is `true`,
and `(*chromeMeeting).CaptureAudio` in `internal/browser/meeting_chrome.go`
only implements live-meeting `getDisplayMedia` streaming (`audioCaptureJS`),
which blocks until context cancellation rather than extracting finite
post-meeting recording audio.

Operators retrieving a post-meeting Recap want the option to opt in via a CLI
flag (`--audio` or `--recap-audio`) or configuration setting
(`capture_audio: true`, `audio: true`, or `recap_audio: true`) and receive a
companion `.mp3` recording alongside the `.md` transcript and AI summary,
while keeping transcript-only output as the default when audio capture is not
enabled.

## Duplicate Detection Record

An unconditional duplicate scan across `.backlogit/stash.jsonl`,
`.backlogit/archive/stash.jsonl`, `.backlogit/queue/`, and
`.backlogit/archive/` confirmed **CLEAN DUPLICATE SCAN** for `DF1F461F`.
Historical archived entry `36EA24F8` (`006-S` / `007-S`) covered live-call
`getDisplayMedia` tab audio only; no active or archived item covers
post-meeting Recap MP3 audio extraction.

## Research Findings

* **Prior learnings consulted (`docs/compound/`)**:
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md` (`007-S`):
    `internal/audio.Encoder` buffers mono `int16` PCM in 1152-sample frames via
    `WritePCM` and pads sub-frame remainders on `Flush()` so `shine-mp3` never
    reads past the input slice.
  * `docs/compound/build-errors/shine-mp3-race-checkptr.md` (`009-S`):
    `encodeFrames` allocates `encoderTailSlack = 4` extra capacity on the
    interleaved stereo slice so `go test -race ./...` passes without `checkptr`
    faults. Routing Recap PCM through `audio.NewEncoder(w, audio.SampleRate)`
    inherits both guarantees without external binaries or CGO.
* **Codebase investigation**:
  * `internal/browser/recap.go` and `internal/browser/meeting_chrome.go`:
    `chromeMeeting` extracts Recap transcripts and AI summaries over CDP from
    targets passing `isTargetInCaptureScope(t, m.capturePageID, ownedFrames)`.
    In `joinHeaded`, the allocated DevTools `port` is currently a local
    variable (`m.cdpPort` is only assigned in headless `NewLiveMeeting`);
    storing `m.cdpPort = port` in `joinHeaded` makes post-`Join` CDP target
    operations work identically in headed and headless modes.
  * `internal/capture/session.go`: removing the hardcoded
    `if rm, ok := m.(browser.RecapMeeting); ok && rm.IsRecap() { captionsOnly = true }`
    override allows `s.cfg.CaptionsOnly` (`!cfg.CaptureAudio`) to govern Recap
    sessions. Because `EnableCaptions` returns immediately after emitting
    `m.recapEntries`, `CaptureAudio` in Recap mode must also complete finitely
    once the recording audio has been extracted, encoded via
    `audio.NewEncoder`, and flushed to `w`.
  * `internal/cli/join.go` and `internal/config/config.go`: `--audio` and
    `capture_audio` / `audio` are already wired into `cfg.CaptureAudio`. Adding
    `--recap-audio` on the CLI (and forwarding it in `inheritedQueueFlags`)
    gives operators an explicit recap flag alias in addition to `--audio` and
    `capture_audio: true`.

## Options Evaluated

### Option A: External `ffmpeg` or `yt-dlp` subprocess download

Invoke an external `ffmpeg` or `yt-dlp` binary from Go to download the
SharePoint stream and transcode it to MP3.

* **Pros**: Offloads container demuxing to an external tool.
* **Cons**: Violates the repository's core single-binary, zero-FFmpeg
  architecture (`README.md`: *"It compiles to a single static binary and drives
  the host browser directly over the Chrome DevTools Protocol via chromedp, so
  it needs no external drivers or FFmpeg"*); requires exporting SharePoint
  authentication cookies outside the browser process.
* **Effort**: medium | **Fit**: poor.

### Option B: In-browser CDP Web Audio extraction + pure-Go `audio.NewEncoder` MP3 encoding (chosen)

When `m.IsRecap()` is `true` and `CaptureAudio(ctx, w)` is called (because
`--audio` / `--recap-audio` or `capture_audio: true` / `audio: true` was
enabled), extract the post-meeting audio from the bound Recap CDP targets
(`isTargetInCaptureScope`) using in-browser Web Audio decoding
(`extractRecapAudioJS`: reading pre-supplied `data-recap-audio-pcm` /
`g_fileInfo.audioPcmBase64` or fetching the `<video>`/`<audio>`/`g_fileInfo`
media source within the authenticated frame context, decoding and resampling to
44.1 kHz mono `int16` PCM via `AudioContext` / `OfflineAudioContext`, and
returning base64-encoded little-endian `int16` PCM chunks), then decode via
`decodeAudioPayload` and encode to MP3 in Go using
`audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` + `Flush()`).

* **Pros**: Preserves the single-binary pure-Go architecture (zero FFmpeg/CGO);
  keeps SharePoint cookies inside the browser tab/iframe; enforces existing tab
  isolation (`isTargetInCaptureScope`); reuses the battle-tested,
  `checkptr`-safe `internal/audio.Encoder`; supports fast, deterministic
  unit testing over loopback CDP fixtures.
* **Cons**: Requires the Recap target to expose a decodable media source or PCM
  payload within the scoped page/iframe targets.
* **Effort**: medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (External `ffmpeg`) | Option B (CDP Web Audio + `audio.NewEncoder`) |
|---|---|---|
| Single-binary / zero-FFmpeg | Violates architecture | Preserves pure-Go single binary |
| Cookie / token containment | Exports cookies to child CLI | Stays inside authenticated browser target |
| `-race` & `checkptr` safety | External process | Reuses verified `audio.NewEncoder` (`encoderTailSlack = 4`) |
| Deterministic unit testability | Requires external binary stub | Testable with `httptest` CDP WebSocket fixtures |

## Decision

We select **Option B** and decompose the work into feature **`015-F`**
(*Post-meeting Recap MP3 audio capture alongside transcript*) and shipment
**`015-S`** (chained after `014-S` via
`backlogit dep add 015-S 014-S --type blocks`) with four width-isolated tasks:

1. **`015.001-T` (U1, `code` domain — `internal/browser`)**: Add
   `extractRecapAudioJS` and `captureRecapAudioFromTargets` in
   `internal/browser/recap.go`, record `m.cdpPort = port` in `joinHeaded`, and
   branch `(*chromeMeeting).CaptureAudio` in
   `internal/browser/meeting_chrome.go` on `m.IsRecap()` (`S` / `medium`).
2. **`015.002-T` (U2, `code` domain — `internal/capture` & `internal/cli`)**:
   Remove the `IsRecap()` override of `captionsOnly` in
   `internal/capture/session.go` and bind/forward `--recap-audio` alongside
   `--audio` in `internal/cli/join.go` (`XS` / `low`).
3. **`015.003-T` (U3, `tests` domain — `internal/browser` & `internal/capture`)**:
   Add deterministic CDP recap audio extraction and MP3 encoding tests in
   `internal/browser/recap_retry_test.go` and update session recap audio tests
   in `internal/capture/session_test.go` (`S` / `low`).
4. **`015.004-T` (U4, `docs` domain)**: Document `--audio` / `--recap-audio`
   and `capture_audio: true` for Meeting Recap MP3 capture in `README.md` and
   `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`
   (`XS` / `trivial`).

## Rejected Alternatives

* **Option A (External `ffmpeg` / `yt-dlp`)**: Rejected because it breaks the
  standalone static binary design and requires external media tooling on
  consumer Windows hosts.
* **Always capturing MP3 audio on every Recap run by default**: Rejected because
  the stash request specifies a flag or configuration-driven option, and
  existing callers expect transcript-only output unless audio capture is enabled.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Unrelated browser tab leakage**: Mitigated by restricting Recap audio
  target discovery to `isTargetInCaptureScope(t, ownerPageID, ownedFrames)` on
  the bound `capturePageID`.
* **`shine-mp3` frame alignment and `-race` `checkptr` safety**: Mitigated by
  streaming all decoded `[]int16` mono PCM chunks exclusively through
  `audio.NewEncoder(w, audio.SampleRate)` (`WritePCM` + `Flush()`).
* **Headed vs. headless parity**: Mitigated by persisting `m.cdpPort = port` in
  `joinHeaded` so post-`Join` `CaptureAudio` has the active DevTools port in
  both launch modes.
