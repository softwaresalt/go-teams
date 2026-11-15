---
title: Past-meeting recap reliability, deterministic Entra auth bootstrap, Edge CDP lifecycle, and transcript passcode redaction decision
description: Deliberation on grouping stash entries 52519E8D, 0544DBA7, 030CD519, 6B0E5F68, 1B1877DB, 54894478, and 2925E813 into feature 026-F and shipment 026-S
topic: Recap Capture Reliability, Auth/Edge CDP Lifecycle, and Transcript Redaction Hardening (Option A)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 015-DL
feature: 026-F
shipment: 026-S
source_stash:
  - 52519E8D
  - 0544DBA7
  - 030CD519
  - 6B0E5F68
  - 1B1877DB
  - 54894478
  - 2925E813
linked_artifacts:
  - docs/plans/2026-11-14-recap-auth-and-redaction-hardening-plan.md
  - .backlogit/queue/015-DL.md
  - .backlogit/queue/026-F.md
  - .backlogit/queue/026-S.md
date: 2026-11-14
status: accepted
tags:
  - browser
  - recap
  - auth
  - capture
  - transcript
  - security
  - cli
---

## Problem Frame

This deliberation covers the Option A grouping of seven active stash entries into a single cohesive release unit under covering feature **`026-F`** and shipment **`026-S`** (sequenced after `025-S`):

1. **`52519E8D`** (`bug`, `critical`):
   * Setting `--mode past-recap` (or `capture_mode: past-recap` in `config.yaml`) sets `ForceRecap: true` on `browser.MeetingOptions`. In `internal/browser/meeting_chrome.go`, `tryExtractRecapViaCDP` and `joinHeadedWithDeadline` short-circuit `Join()` as soon as `captureAISummaryFromTargets` finds the AI Summary on the outer page—before the cross-origin SharePoint `xplatplugins.aspx` transcript iframe (`.ms-List`) finishes loading. `Join()` returns `nil` with `len(m.recapEntries) == 0`, so `m.IsRecap()` returns `false`, and `Session.Run` writes an empty transcript header (`Total Attendees: 0`, fallback meeting-ID title, zero utterances) and enters the live-call `EnableCaptions` loop where it hangs waiting for `#callingButtons-showMoreBtn`.
2. **`0544DBA7`** (`bug`, `high`):
   * Running Meeting Recap capture with `--recap-audio` / `--audio` against a real SharePoint Stream recording fails with `Uncaught (in promise) TypeError: Failed to fetch` in `extractRecapAudioJS` (`internal/browser/recap.go`), which calls `fetch(mediaUrl, {credentials: 'include', redirect: 'error'})` against SharePoint media/blob URLs that redirect or use MediaSource Extensions (MSE), and enforces a 30-minute `maxRecapAudioDurationSec = 1800` cap that rejects 1-hour meetings. When `CaptureAudio` fails, `Session.Run` (`internal/capture/session.go`) leaves an empty 0-byte `.mp3` file on disk and aborts before `# AI Summary` is appended to the `.md` transcript.
3. **`030CD519`** (`bug`, `medium`):
   * Passing `--no-ai-summary` (or `ai_summary_delay: 0s`) is ignored when capturing a Meeting Recap: `tryExtractRecapViaCDP`, `joinHeadedWithDeadline`, and `Session.Run` unconditionally extract and append `# AI Summary` to the output Markdown file even when AI summary capture is disabled.
4. **`6B0E5F68`** (`bug`, `medium`):
   * In `setupOutput` (`internal/capture/output.go`), `FormatTranscriptFilename(cfg.FilenameTemplate, cfg, now, id)` is called with the current capture time `now` instead of `headerStart` (`cfg.MeetingStart` when non-zero). Exporting a previous day's Meeting Recap writes the meeting's actual date in `**Meeting Start:**` but expands `<YYYYMMDD>` and `<YYYY-MM-DD>` in the filename to today's capture date.
5. **`1B1877DB`** (`bug`, `high`):
   * (1) Initial authentication (`auth` subcommand and cold-profile bootstrap when `hasTeamsSessionInProfile` is `false`) must require `--account` (or `default_account` in `config.yaml`) and fail fast with an actionable error when no account is provided, ensuring deterministic Microsoft Entra account selection from the auth picker window.
   * (2) On Windows, when `--account` is provided and `launchCleanHeadedBrowser` spawns Microsoft Edge (`msedge.exe`), the initial parent stub process can exit immediately after spawning a child browser process listening on `--remote-debugging-port`. `joinHeadedWithDeadline` must not abort on `procDone` while `listCDPTargets(port)` is still alive, and `cleanupProc` must terminate the child browser process listening on `port`.
6. **`54894478`** (`bug`, `medium`):
   * The `**Source:**` link written to the Markdown transcript header (`internal/capture/output.go` and `internal/transcript/writer.go`) embeds `cfg.MeetingURL` without stripping sensitive query parameters (such as `?p=<passcode>` on `/meet/` URLs or SharePoint `driveId`/`sitePath` tokens on Meeting Recap URLs), leaking passcodes and tokens into saved `.md` transcripts.
7. **`2925E813`** (`task`, `low`, `DEFERRED SCOPE EXPANSION` from `025-S`):
   * Correct the `redactHelpArg` doc comment in `internal/cli/root.go`, which says `"well-formed http:// or https:// scheme"` although dispatch is by prefix only (malformed URLs still reach `teams.RedactArgs` and retained host/path text may carry secrets). Source refs: task `025.001-T`, feature `025-F`, shipment `025-S`, PR `#65`, review thread `PRRT_kwDOTStvas63lUy4`.

### Why These Entries Belong Together

* All seven entries harden the end-to-end Meeting Recap, browser authentication/CDP lifecycle, and transcript output/redaction contract across `internal/browser`, `internal/capture`, `internal/transcript`, and `internal/cli`.
* `52519E8D`, `030CD519`, and `1B1877DB` all modify `joinHeadedWithDeadline` and `tryExtractRecapViaCDP` in `internal/browser/meeting_chrome.go`; `0544DBA7` and `1B1877DB` both modify `internal/browser/recap.go`; `6B0E5F68` and `54894478` both modify `internal/capture/output.go`; and `52519E8D`, `030CD519`, `1B1877DB`, and `2925E813` all touch `internal/cli`. Grouping them into `026-F` / `026-S` with explicit task dependencies eliminates cross-shipment merge conflicts.

### Explicit Scope Boundary (Excluded Items)

* Stash **`66488961`** (Option B — `go-teams` binary and release rename) is reserved for Wave 2 (`027-S`) to preserve artifact-class isolation between runtime bug fixes and cross-cutting binary/workflow/documentation renames.
* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and stash entries **`ED249B2F`** and **`3EB89E2D`** are out of scope for Option A.

## P-021 C5 and C6 Triage Record for `2925E813`

* **Precedence classification (`2925E813`)**: Carries the literal `DEFERRED SCOPE EXPANSION` prefix from `025-S` closure PR `#65` and is routed through this deliberation artifact (`015-DL`) per P-021 C6 before implementation planning.
* **Source references carried forward**:
  * Task ID: `025.001-T`
  * Feature ID: `025-F`
  * Shipment ID: `025-S`
  * Pull request: `#65`
  * Review thread ID: `PRRT_kwDOTStvas63lUy4`
* **Obligation (A) — Unconditional duplicate detection**:
  * **Outcome: CLEAN DUPLICATE SCAN.** Queried all active, harvested, and archived stash entries (`stash_entries`) and all queue/archived backlog items (`items`) for `redactHelpArg` and `PRRT_kwDOTStvas63lUy4`. Only the single entry `2925E813` exists; zero duplicates found.
* **Obligation (B) — Late-identifier reconciliation**:
  * **Outcome: All five source-reference identifiers were populated at capture time** (`025.001-T`, `025-F`, `025-S`, PR `#65`, thread `PRRT_kwDOTStvas63lUy4`) and verified against `docs/closure/2026-11-14-025-S-review-and-closure.md`. No `N/A` fields exist; reconciliation is an idempotent no-op.

## Options Considered

### Option 1 — Separate shipments per bug vs unified 4-task capability shipment (`026-S`)

* **Option 1A (Rejected)**: Stage 4 separate shipments across Recap Join, Recap Audio, Auth/Edge CDP, and Transcript Redaction. Rejected because 5 of the 7 items modify the exact same functions in `internal/browser/meeting_chrome.go`, `internal/browser/recap.go`, `internal/capture/session.go`, and `internal/capture/output.go`, causing avoidable PR rebase churn.
* **Option 1B (Chosen)**: Stage a single covering feature (`026-F`) and shipment (`026-S`) decomposed into 4 atomic, single-domain tasks (`026.001-T` through `026.004-T`), each touching at most 3 production files within the 2-hour rule.

### Option 2 — `**Source:**` URL redaction policy (`54894478`)

* **Option 2A (Rejected — Parameter denylist)**: Strip only named parameters (`p`, `driveId`, `sitePath`). Rejected per `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`: denylists miss new query tokens, OAuth parameters, or hash-fragment tokens (`#...?...`).
* **Option 2B (Chosen — Strict `scheme://host/path` allowlist preserving leaf-package isolation)**: In `internal/capture/output.go` (adapter layer), redact `cfg.MeetingURL` via `teams.RedactURL` (and also redact `rawURL` in `sanitizeMeetingID` before slugifying any fallback URL so query parameters never leak into filenames or `MeetingID`). In `internal/transcript/writer.go` (`sanitizeSourceURL`), enforce the identical `scheme://host/path` allowlist using only standard library `net/url` (preserving `docs/ARCHITECTURE.md`'s leaf-package invariant that `internal/transcript` never imports another `internal/` package), stripping all userinfo, query parameters, and fragments, and rejecting SharePoint path-embedded sharing tokens (`/:`).

## Decision Outcome

Adopt **Option 1B** and **Option 2B**, decomposing `026-F` / `026-S` into:

1. **`026.001-T` (U1 — `internal/browser`)**: Define `AISummaryOnly`, `DisableAISummary`, and `ErrAccountRequired` in `internal/browser/meeting.go`; separate `AISummaryOnly` from `ForceRecap` (`52519E8D`) so `AISummaryOnly` skips `.ms-List` early return and waits for the AI Summary while `ForceRecap` waits for `.ms-List` transcript entries; honor `DisableAISummary` during Recap CDP extraction (`030CD519`); and harden `extractRecapAudioJS` for deduplicated non-`blob:` URL preference, per-candidate `identityMatched` reset, `redirect: 'follow'` with post-redirect `response.url` identity verification, `AudioContext({sampleRate: 44100})`, and 70-minute (`4200s`) / 256 MiB source / 360 MiB PCM caps that stay safely below V8's ~512 MiB string limit (`0544DBA7`).
2. **`026.002-T` (U2 — `internal/browser`)**: Enforce `ErrAccountRequired` on `ChromeRunner.Authenticate` and cold-profile `Join` / `joinHeadedWithDeadline` (`1B1877DB` part 1), keep `joinHeadedWithDeadline` alive when `procDone` closes while `listCDPTargets(port)` still responds, and terminate any surviving child browser process listening on `127.0.0.1:<port>` during `cleanupProc` after verifying CDP liveness and browser image name (`1B1877DB` part 2).
3. **`026.003-T` (U3 — `internal/capture` & `internal/transcript`)**: Preserve `# AI Summary` output when `CaptureAudio` fails and remove 0-byte `.mp3` files on audio error after `out.Close()` (`0544DBA7`), honor `DisableAISummary` in `Session.Run` (`030CD519`), pass `headerStart` (`cfg.MeetingStart` when non-zero) to `FormatTranscriptFilename` and update `OutputConfig` GoDoc (`6B0E5F68`), and redact `SourceURL` and `sanitizeMeetingID` fallback URLs to `scheme://host/path` (`54894478`).
4. **`026.004-T` (U4 — `internal/cli` & `docs/`)**: Wire `AISummaryOnly` and `DisableAISummary` in `defaultNewMeeting`, `defaultDeps().newSession`, and `runAppendAISummary` (`52519E8D`, `030CD519`), enforce `--account` / `default_account` in `newAuthCommand` (`1B1877DB`), update the `redactHelpArg` doc comment in `internal/cli/root.go` (`2925E813`), and update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md`.
