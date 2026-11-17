---
title: Post-remediation adversarial review for shipment 031-S (HEAD 459d1b3)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: 459d1b3057bcf882b77a0704b82ed1eebd56553c
diff_range: main..HEAD
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 1 MEDIUM Majority, 0 MEDIUM Plurality, 3 LOW Unique)
readiness_verdict: BLOCKED
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Authorized contract scope (P-021 C1)**:
  * Plan: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`
  * Deliberation: `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`
  * Diff: `main..HEAD` (`459d1b3057bcf882b77a0704b82ed1eebd56553c`), 7 changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Dispatch & Pool Status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `readiness_verdict`: `BLOCKED` (0 `P0`, 4 `P1` including 1 `MEDIUM` Majority finding violating Protected Invariant 2 and 3 `LOW` Unique findings, 0 `P2`)

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 2 (2 MAJOR) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 3 (2 MAJOR, 1 MINOR) |

## Verification of Prior Findings (`M1`..`M4`, `U1`..`U4`)

| Prior ID | Summary | Status at `459d1b3` | Notes |
|---|---|---|---|
| `M1` | Child frame ancestor identity inheritance (`recap.go:133`) | **RESOLVED** | `validateFrameContext(win, doc)` computes `baseIdentityMatched: state.matched` solely from own URL and own `g_fileInfo`; `captureStreamEmbedAudio` also verifies `transcodeAudioTemplateUrl` when `!baseIdentityMatched` |
| `M2` | `oneDrive.transcode` `http:` origin, `x-ms-*` wildcard, unkeyed header map (`recap.go:820`) | **PARTIALLY RESOLVED** | `inspectTranscodeRequest` now enforces `https:` (or loopback `http:`), exact `u.origin === winOrigin`, `allowlistedHeaderNames`, and `capturedAuthHeadersByOrigin[u.origin]`; however, `info.transcodeUrl` at `recap.go:632` pre-seeds `transcodeAudioTemplateUrl` without protocol/origin validation (see `M1-R2`) |
| `M3` | Missing Path B 2400-segment cap & `runPathA` segment-count violation masked by fallback (`recap.go:717,982`) | **RESOLVED** | `maxRecapAudioSegments = 2400` enforced in `recordMSESegment` (`recap.go:710`) and `runPathA` throws `limitError` (`recap.go:975`) without fallback masking |
| `M4` | Path B `decodeAudioData` without re-checking updated duration/buffered end (`recap.go:1377`) | **RESOLVED** | `curMedia.duration`, `info.duration`, `info.durationSeconds`, and `mseMaxBufferedEnd` are checked against `maxRecapAudioDurationSec` and `maxRecapAudioPCMBytes` prior to `combined` allocation and `decodeAudioData` (`recap.go:1436-1443`) |
| `U1` | Uncancelled Path A workers mutating `streamSourceBytes` on Path B fallback (`recap.go:1013`) | **PARTIALLY RESOLVED** | `runPathA` uses `pathAAbortCtrl` and awaits `Promise.allSettled(workers)` before resetting `streamSourceBytes`; however, on outer 120s deadline expiry, `cleanupHooks()` aborts `abortCtrl` and `maybeStartPathA().catch` runs afterward and calls `m.play()` at 16x (see `U1-R2`) |
| `U2` | Path B exits and returns truncated audio after 3-second append gap (`recap.go:1322`) | **PARTIALLY RESOLVED** | 3-second append-gap early break is skipped when `targetDur > 0`, but hitting the 120s `deadlineAt` without reaching `targetDur - 0.5` or `m.ended` still exits normally and returns truncated audio (see `U3-R2`) |
| `U3` | Inert `isDownloadBlocked=true` page returning `''` (`recap.go:301`) | **RESOLVED** | `!candidates.length && !streamSurface && info.isDownloadBlocked` throws `recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)` (`recap.go:306-311`) |
| `U4` | Direct-fetch path returning unsanitized `selectedUrl` (`recap.go:437`) | **RESOLVED** | Direct-fetch return applies `mediaUrl: sanitizeMediaUrl(selectedUrl)` (`recap.go:444`) |

## Protected Invariants & Redaction Verification

| Check | Status | Notes |
|---|---|---|
| Invariant 1: Single-binary pure-Go | PASS | Zero external binaries (`ffmpeg`, `yt-dlp`), zero CGO, zero `go.mod`/`go.sum` changes |
| Invariant 2: Per-frame identity isolation & origin containment | PARTIAL FAIL | `M1` and `U4` fixed, `inspectTranscodeRequest` hardened; 1 `MEDIUM` Majority `P1` (`M1-R2` at `recap.go:632`: unvalidated `info.transcodeUrl` pre-seed) |
| Invariant 3: Pre-allocation resource & segment-count caps + error precedence | PASS | `M3` and `M4` resolved (`maxRecapAudioSegments = 2400`, pre-`decodeAudioData` duration/PCM caps, `limitError` precedence, `pathAAbortCtrl` worker settlement) |
| Invariant 4: Backward compatibility (direct-fetch & `isDownloadBlocked=true`) | PASS | `U3` resolved; direct-fetch and inert blocked-page contracts intact |
| Invariant 5: Clean restoration of browser globals, timers, `AbortController`, and `__goTeamsRecapAudioTransfer` | PARTIAL FAIL | `cleanupHooks()` is idempotent, but 1 `LOW` Unique `P1` (`U1-R2` at `recap.go:886`) can call `m.play()` at `16x` after `cleanupHooks()` when `abortCtrl` aborts a pending `pathAPromise` |
| Strict redaction | PASS | Zero live SharePoint URLs, tenant hostnames, UPNs, passcodes, or tokens in any tracked file (all `sharepoint.com`, `tempauth=`, and UPN strings are synthetic `contoso` test fixtures or documentation examples) |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

### M1-R2 (`P1` / `MAJOR`): `info.transcodeUrl` pre-seeds `transcodeAudioTemplateUrl` without HTTPS/loopback or same-origin validation

* **Confidence**: `MEDIUM` (2/3: Anchor Reviewer as `MAJOR`, Reviewer-C as `MINOR`; conservative severity `MAJOR`)
* **Priority score**: `6` (`confidence_weight=2 × severity_weight=3`)
* **Rule**: Protected Invariant 2 — HTTPS origin containment (`M2` incomplete closure)
* **File / Line**: `internal/browser/recap.go:632` (and `recap.go:845`, `recap.go:903`, `recap.go:960`)
* **Issue**: `transcodeAudioTemplateUrl = info.transcodeUrl ? String(info.transcodeUrl) : ''` initializes `transcodeAudioTemplateUrl` without checking `allowedProto` (`https:` or explicit loopback `http:`) or `u.origin === winOrigin`. In `inspectTranscodeRequest`, `if (!transcodeAudioTemplateUrl || track === 'audio')` preserves this pre-seeded URL whenever `track === 'video'` supplies `wsd`/`ppd`. `runPathA` then calls `fetchBytesWithCap` with `credentials: 'include'` on the unvalidated `info.transcodeUrl`, permitting cross-origin or non-HTTPS credentialed fetches.
* **Fix**: Validate `info.transcodeUrl` with the same `allowedProto` and exact `playerWin` origin check as `inspectTranscodeRequest` (discarding it if invalid), and re-verify protocol and origin at the start of `runPathA` and inside `fetchBytesWithCap`.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1-R2 (`P1` / `MAJOR`): `maybeStartPathA` fallback handler can resume 16x playback after `cleanupHooks()` on deadline/abort

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Rule**: Protected Invariant 5 — Clean idempotent restoration of media elements & worker settlement
* **File / Line**: `internal/browser/recap.go:886` (and `recap.go:1310-1405`)
* **Issue**: When the collection loop exits at the 120s `deadlineAt` while `pathAPromise` is still pending, `finally { cleanupHooks(); }` sets `hooksRestored = true`, pauses media elements, and calls `abortCtrl.abort()`. `runPathA` then rejects with `'transcode fetch aborted'`, and the `.catch` handler in `maybeStartPathA` executes after cleanup, mutating `streamSourceBytes` and calling `m.muted = true; m.playbackRate = 16; m.play()` on the page's media element after restoration.
* **Fix**: Guard the `maybeStartPathA` `.catch` handler with `if (hooksRestored) return null;` and abort/await `Promise.allSettled([pathAPromise])` before or as part of `cleanupHooks()` in `finally`; throw an explicit timeout error if `deadlineAt` is reached with Path A incomplete and no Path B output.

### U2-R2 (`P1` / `MAJOR`): Pre-hook native `video/mp4` `SourceBuffer` lacks `.mime` and is recorded as audio

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Rule**: Correctness — Path B / MSE `video/mp4` track stripping
* **File / Line**: `internal/browser/recap.go:1284` (`SourceBuffer.prototype.appendBuffer` hook)
* **Issue**: Native `SourceBuffer` instances created before `installWindowHooks` do not expose a `.mime` property and do not carry `__isRecapVideoShim` or `__isRecapAudioSB`. `appendBuffer` assigns such a buffer to `audioSBRef` and passes its media segments into `recordMSESegment(u8)`, allowing pre-existing `video/mp4` media segments (`moof`/`styp`/`sidx`) to enter `mseSegments` and overwrite audio segments sharing the same `tfdt`.
* **Fix**: Track per-`SourceBuffer` classification in a `WeakMap` (`'audio' | 'video' | 'unknown'`) populated from `addSourceBuffer(mimeType)` and from `isFMP4InitSegment(u8)` (`classifyInitSegmentTrack`), and ignore media segments from `'video'` or unverified `'unknown'` buffers.

### U3-R2 (`P1` / `MAJOR`): Path B returns truncated audio when `deadlineAt` expires before `targetDur`

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Rule**: Correctness — Path B completion & truncation prevention (`U2` incomplete closure)
* **File / Line**: `internal/browser/recap.go:1352` (`captureStreamEmbedAudio` loop exit & assembly)
* **Issue**: Although the 3-second append-gap early break is now skipped when `targetDur > 0`, a permanent stall or slow stream where `!m.ended` and `mseMaxBufferedEnd < targetDur - 0.5` runs until `Date.now() >= deadlineAt` (120s) and exits the `while` loop normally. Subsequent code only checks that `initSegment` and `orderedSegments.length` are non-empty, decoding and returning the truncated prefix as a valid MP3.
* **Fix**: Track `targetDur` outside the loop and, when `!pathAResult && targetDur > 0`, require `(curMedia && curMedia.ended) || mseMaxBufferedEnd >= targetDur - 0.5` before decoding; otherwise throw a stall/timeout error instead of returning truncated audio.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Action classes are recorded for eligibility only (`mode: report-only` applied no edits).

| Priority | ID | File & Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 6 | M1-R2 | `internal/browser/recap.go:632` | `MEDIUM` | `P1` (`MAJOR`) | `gated_auto` | Validate `info.transcodeUrl` with `allowedProto` (`https:` / loopback) and exact `playerWin` origin, and re-verify before Path A fetches |
| 3 | U1-R2 | `internal/browser/recap.go:886` | `LOW` | `P1` (`MAJOR`) | `advisory` (needs explicit disposition) | Guard `maybeStartPathA().catch` with `hooksRestored` and settle `pathAPromise` before/during `cleanupHooks()` |
| 3 | U2-R2 | `internal/browser/recap.go:1284` | `LOW` | `P1` (`MAJOR`) | `advisory` (needs explicit disposition) | Track per-`SourceBuffer` track classification in a `WeakMap` and collect media only from verified audio buffers |
| 3 | U3-R2 | `internal/browser/recap.go:1352` | `LOW` | `P1` (`MAJOR`) | `advisory` (needs explicit disposition) | Throw a timeout/stall error if Path B exits at `deadlineAt` with `targetDur > 0`, `!m.ended`, and `mseMaxBufferedEnd < targetDur - 0.5` |

## Bug / Issue Queue Entries

Ready for backlog creation for each `P0` and `P1` finding (not created automatically under `mode: report-only`):

```yaml
- type: bug
  title: "Invariant 2: Validate seeded info.transcodeUrl protocol and origin before Path A fetch"
  description: "transcodeAudioTemplateUrl is pre-seeded from info.transcodeUrl without https/loopback or exact playerWin origin validation, allowing video-only transcode observations to trigger credentialed Path A fetches to unvalidated URLs."
  file: "internal/browser/recap.go"
  line: 632
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Validate info.transcodeUrl with the same allowedProto and same-origin checks as inspectTranscodeRequest and re-verify origin/protocol in runPathA and fetchBytesWithCap."
  linked_review: "docs/closure/2026-11-16-031-s-post-remediation-adversarial-review.md"

- type: bug
  title: "Invariant 5: Prevent maybeStartPathA catch handler from resuming playback after cleanupHooks"
  description: "When captureStreamEmbedAudio exits at deadlineAt while pathAPromise is pending, cleanupHooks aborts abortCtrl and maybeStartPathA().catch runs post-cleanup, calling m.play() at 16x."
  file: "internal/browser/recap.go"
  line: 886
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Return early in maybeStartPathA().catch when hooksRestored is true, and abort plus await Promise.allSettled([pathAPromise]) in finally before restoring/pausing media."
  linked_review: "docs/closure/2026-11-16-031-s-post-remediation-adversarial-review.md"

- type: bug
  title: "Correctness: Ignore media segments from pre-hook native video SourceBuffers"
  description: "Native SourceBuffer instances created before installWindowHooks lack .mime and __isRecapVideoShim, causing their video media segments to enter mseSegments."
  file: "internal/browser/recap.go"
  line: 1284
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Maintain a WeakMap mapping SourceBuffer instances to audio/video/unknown from addSourceBuffer and initsegment classification, and drop video/unknown media segments."
  linked_review: "docs/closure/2026-11-16-031-s-post-remediation-adversarial-review.md"

- type: bug
  title: "Correctness: Reject truncated Path B output when deadlineAt expires before targetDur"
  description: "If Path B stalls when targetDur > 0 and !m.ended, the loop exits at deadlineAt and decodes a partial prefix instead of failing."
  file: "internal/browser/recap.go"
  line: 1352
  severity: "MAJOR"
  confidence: "LOW"
  fix: "When !pathAResult && targetDur > 0, require m.ended or mseMaxBufferedEnd >= targetDur - 0.5 after the loop; otherwise throw a timeout/stall error."
  linked_review: "docs/closure/2026-11-16-031-s-post-remediation-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 4
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
