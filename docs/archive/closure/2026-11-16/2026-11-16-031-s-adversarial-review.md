---
title: Adversarial review for shipment 031-S (high-speed audio-only MP3 capture on download-blocked SharePoint Meeting Recaps)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: abc78db71881d62fa9db659701ad4a2dd8e56374
diff_range: main..HEAD
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 4 MEDIUM Majority, 0 MEDIUM Plurality, 4 LOW Unique)
readiness_verdict: BLOCKED
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Authorized contract scope (P-021 C1)**:
  * Plan: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`
  * Deliberation: `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`
  * Diff: `main..HEAD` (`abc78db71881d62fa9db659701ad4a2dd8e56374`), 7 changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Dispatch & Pool Status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `readiness_verdict`: `BLOCKED` (0 `P0`, 6 `P1` including 4 `MEDIUM` Majority findings violating Protected Invariants 2 and 3, 2 `P2`)

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 6 (5 MAJOR, 1 MINOR) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 6 (4 MAJOR, 2 MINOR) |

## Protected Invariants & Redaction Verification

| Check | Status | Notes |
|---|---|---|
| Invariant 1: Single-binary pure-Go | PASS | Zero external binaries (`ffmpeg`, `yt-dlp`), zero CGO, zero `go.mod`/`go.sum` changes |
| Invariant 2: Per-frame identity isolation & origin containment | FAIL | Two `MEDIUM` Majority `P1` findings (`recap.go:133` ancestor identity inheritance; `recap.go:820` `http:` origin & unkeyed/wildcard header capture) plus 1 `LOW` Unique `P2` (`recap.go:437` unsanitized direct-fetch `mediaUrl`) |
| Invariant 3: Pre-allocation resource & segment-count caps + error precedence | FAIL | Two `MEDIUM` Majority `P1` findings (`recap.go:717,982` missing Path B 2400-segment cap & `runPathA` segment-count error masked by fallback; `recap.go:1377` missing Path B pre-`decodeAudioData` duration/PCM cap) plus 1 `LOW` Unique `P1` (`recap.go:1013` uncancelled Path A workers mutating `streamSourceBytes`) |
| Invariant 4: Backward compatibility (direct-fetch & `isDownloadBlocked=true`) | PARTIAL | Direct-fetch and existing `recap_test.go` probes remain intact; 1 `LOW` Unique `P2` (`recap.go:301` returns `''` instead of `isDownloadBlocked=true` on inert blocked page with no candidates and no stream surface) |
| Invariant 5: Clean restoration of browser globals, timers, `AbortController`, and `__goTeamsRecapAudioTransfer` | PASS | Idempotent `cleanupHooks` in `finally`, `AbortController` abort, `<video>` pause, and `delete window.__goTeamsRecapAudioTransfer` in Go and JS |
| Strict redaction | PASS | Zero live SharePoint URLs, tenant hostnames, UPNs, passcodes, or tokens in any tracked file |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

### M1 (`P1` / `MAJOR`): Child frame inherits ancestor recording identity without own-frame verification

* **Confidence**: `MEDIUM` (2/3: Anchor Reviewer, Reviewer-C)
* **Priority score**: `6` (`confidence_weight=2 × severity_weight=3`)
* **Rule**: Protected Invariant 2 — Per-frame identity isolation (`recapExpectedIdentity`)
* **File / Line**: `internal/browser/recap.go:133` (and `walk` at `recap.go:180`)
* **Issue**: `validateFrameContext(win, doc, ancestorMatched, ancestorItemId)` sets `baseIdentityMatched: state.matched || Boolean(ancestorMatched)` and falls back to `ancestorItemId`. A child `<iframe>` with no identity on its own URL or `g_fileInfo` (e.g., an unpopulated or unrelated player frame) inherits its parent's verification and `itemId`, allowing it to win `frameScore` and be hooked/captured without positive verification of its own `g_fileInfo` or URL.
* **Fix**: Do not inherit `ancestorMatched` or `ancestorItemId` into `baseIdentityMatched` or `itemId`. Require each selected player frame to positively match `recapExpectedIdentity` via its own URL, its own `g_fileInfo`, or the `oneDrive.transcode` URL (`tState.matched`), otherwise fail closed with `'recap audio recording identity unverified'`.

### M2 (`P1` / `MAJOR`): `oneDrive.transcode` header capture/replay permits `http:`, `x-ms-*` wildcard, and unkeyed cross-origin state

* **Confidence**: `MEDIUM` (2/3: Anchor Reviewer, Reviewer-C)
* **Priority score**: `6` (`confidence_weight=2 × severity_weight=3`)
* **Rule**: Protected Invariant 2 — HTTPS origin containment and explicit header allowlist
* **File / Line**: `internal/browser/recap.go:820` (`inspectTranscodeRequest`, `fetchBytesWithCap`)
* **Issue**: `inspectTranscodeRequest` treats `u.protocol === 'http:'` as valid alongside `'https:'` for arbitrary hosts, captures any header matching `kl.startsWith('x-ms-')` rather than an explicit allowlist, and stores headers in a single shared `capturedAuthHeaders` map rather than keying by origin (despite hooking both `playerWin` and `window`). Additionally, if `info.transcodeUrl` pre-populates `transcodeAudioTemplateUrl`, `transcodeOrigin` remains `''` unless a `track=audio` request is later seen.
* **Fix**: Restrict `inspectTranscodeRequest` and header replay to `u.protocol === 'https:'` (plus explicit loopback `http://127.0.0.1` / `http://localhost` / `http://[::1]` only if local E2E fixtures use plain HTTP, or migrate the E2E fixture to TLS), replace the `x-ms-` prefix wildcard with an explicit header allowlist, and store captured headers in a map keyed by `u.origin` so headers are replayed solely to the exact origin that issued them.

### M3 (`P1` / `MAJOR`): Missing 2400-segment cap in Path B and `runPathA` segment-count violation masked by fallback

* **Confidence**: `MEDIUM` (2/3: Anchor Reviewer as `MAJOR` at line 717, Reviewer-C as `MINOR` at line 982; conservative severity `MAJOR`)
* **Priority score**: `6` (`confidence_weight=2 × severity_weight=3`)
* **Rule**: Protected Invariant 3 — Segment-count cap (`maxRecapAudioSegments = 2400`) and error precedence
* **File / Line**: `internal/browser/recap.go:717, 982` (`recordMSESegment`, `runPathA`)
* **Issue**: `recordMSESegment` in Path B does not enforce `maxRecapAudioSegments = 2400` before inserting into `mseSegments`. Meanwhile, `runPathA` throws `'transcode segment count out of bounds'` when `totalSegments < 1 || totalSegments > 2400`; because that message does not contain `limitError`, `maybeStartPathA().catch(...)` treats it as a recoverable error and falls back to uncapped Path B (or turns it into `isDownloadBlocked=true` if Path B produces 0 segments).
* **Fix**: Define `const maxRecapAudioSegments = 2400`, enforce `mseSegments.size >= maxRecapAudioSegments` in `recordMSESegment` (setting `fatalErr = new Error(limitError)` and aborting), and throw `limitError` (or include it) when `totalSegments > maxRecapAudioSegments` in `runPathA` so resource-limit rejections are never masked by Path B fallback or `isDownloadBlocked=true`.

### M4 (`P1` / `MAJOR`): Path B calls `decodeAudioData` without re-checking updated duration or buffered end against caps

* **Confidence**: `MEDIUM` (2/3: Anchor Reviewer, Reviewer-C)
* **Priority score**: `6` (`confidence_weight=2 × severity_weight=3`)
* **Rule**: Protected Invariant 3 — Pre-allocation PCM and duration caps before `decodeAudioData`
* **File / Line**: `internal/browser/recap.go:1377` (`captureStreamEmbedAudio`)
* **Issue**: Duration checks run only at the entry of `captureStreamEmbedAudio`, when MSE `<video>.duration` is normally `NaN`. Before allocating `combined` and calling `audioContext.decodeAudioData(combined.buffer)` in Path B, neither updated `media.duration` nor `mseMaxBufferedEnd` is checked against `maxRecapAudioDurationSec` and `maxRecapAudioPCMBytes`. An unstarted/long MSE stream under the 256 MiB compressed source cap can trigger multi-GB float32 decoding before `decodeAudioBufferToPCMChunks` runs.
* **Fix**: Before allocating `combined` and calling `decodeAudioData`, evaluate `[media && media.duration, info.duration, info.durationSeconds, mseMaxBufferedEnd]` against `maxRecapAudioDurationSec` and `Math.ceil(d * 44100) * 2 > maxRecapAudioPCMBytes`, throwing `limitError` before decoding.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1 (`P1` / `MAJOR`): Path A workers continue fetching and mutating `streamSourceBytes` after non-fatal fallback to Path B

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Rule**: Protected Invariant 3 & Path A -> Path B fallback correctness
* **File / Line**: `internal/browser/recap.go:1013` (`runPathA` `worker()`, `maybeStartPathA`)
* **Issue**: When one Path A worker rejects with a non-fatal error (e.g., per-segment rotating key failure or HTTP error), `maybeStartPathA().catch(...)` resets `streamSourceBytes = mseBytesTotal()` and resumes Path B, but `abortCtrl` is not aborted on non-fatal fallback. Up to 15 sibling workers keep running `while (nextIdx < totalSegments)` and calling `fetchBytesWithCap`, mutating the shared `streamSourceBytes` counter concurrently with Path B and potentially tripping a spurious `limitError`.
* **Fix**: Create a Path A-specific `AbortController` (linked to the outer `abortCtrl`) and abort it immediately when any Path A worker fails or Path A falls back; wait for workers to settle before resetting `streamSourceBytes = mseBytesTotal()`.

### U2 (`P1` / `MAJOR`): Path B exits and returns truncated audio after any 3-second append gap

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Rule**: Correctness — Path B completion & truncation prevention
* **File / Line**: `internal/browser/recap.go:1322` (`captureStreamEmbedAudio` MSE polling loop)
* **Issue**: Line 1322 breaks out of the MSE wait loop whenever `mseLastAppendAt > 0 && Date.now() - mseLastAppendAt >= 3000`, even if `targetDur` is known and `mseMaxBufferedEnd < targetDur - 0.5` and `!m.ended`. A 3-second network stall causes Path B to decode and return an incomplete prefix as a valid MP3.
* **Fix**: When `targetDur > 0`, do not treat a 3-second append pause as completion while `mseMaxBufferedEnd < targetDur - 0.5` and `!m.ended`; only break on `m.ended`, reached target duration, or throw a stall/timeout error at deadline.

### U3 (`P2` / `MINOR`): Inert `isDownloadBlocked=true` page with no candidates and no stream surface returns `''`

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Rule**: Protected Invariant 4 — Backward compatibility (`isDownloadBlocked=true` error contract)
* **File / Line**: `internal/browser/recap.go:301` (`extractRecapAudioJS`)
* **Issue**: When `!candidates.length && !streamSurface`, line 301 returns `''` even when `info.isDownloadBlocked` is `true`, causing the Go caller to report `'browser: recap audio unavailable for requested recording'` instead of `'recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)'`.
* **Fix**: When `!candidates.length && !streamSurface && info.isDownloadBlocked`, throw `new Error('recap audio download is blocked by SharePoint permissions (isDownloadBlocked=true)')` (note `captureRecapAudioFromTargets` ranks `streamembed.aspx` first and continues across targets on error), and add a unit test covering an inert blocked page with no candidates or stream surface.

### U4 (`P2` / `MINOR`): Direct-fetch success path returns unsanitized `selectedUrl` in `mediaUrl`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Rule**: Protected Invariant 2 — Secret query-token stripping
* **File / Line**: `internal/browser/recap.go:437` (`extractRecapAudioJS`)
* **Issue**: `captureStreamEmbedAudio` applies `sanitizeMediaUrl(selectedMediaUrl)` before returning the CDP payload, whereas the direct-fetch path at line 437 still returns raw `selectedUrl`.
* **Fix**: Return `mediaUrl: sanitizeMediaUrl(selectedUrl)` on line 437.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Action classes are recorded for eligibility only (`mode: report-only` applied no edits).

| Priority | ID | File & Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 6 | M1 | `internal/browser/recap.go:133` | `MEDIUM` | `P1` (`MAJOR`) | `gated_auto` | Stop inheriting `ancestorMatched`/`ancestorItemId` in `validateFrameContext`; require own-frame or transcode-URL identity match |
| 6 | M2 | `internal/browser/recap.go:820` | `MEDIUM` | `P1` (`MAJOR`) | `gated_auto` | Enforce `https:` origin containment (or explicit loopback test allowance), exact header allowlist (no `x-ms-` wildcard), and per-origin header map |
| 6 | M3 | `internal/browser/recap.go:717,982` | `MEDIUM` | `P1` (`MAJOR`) | `gated_auto` | Enforce `maxRecapAudioSegments = 2400` in Path B `recordMSESegment` and throw `limitError` on Path A segment-count overflow |
| 6 | M4 | `internal/browser/recap.go:1377` | `MEDIUM` | `P1` (`MAJOR`) | `gated_auto` | Re-check `media.duration` and `mseMaxBufferedEnd` against `maxRecapAudioDurationSec` / `maxRecapAudioPCMBytes` before `decodeAudioData` |
| 3 | U1 | `internal/browser/recap.go:1013` | `LOW` | `P1` (`MAJOR`) | `advisory` (needs explicit disposition) | Abort Path A workers with a dedicated `AbortController` and settle before resetting `streamSourceBytes` on Path A -> Path B fallback |
| 3 | U2 | `internal/browser/recap.go:1322` | `LOW` | `P1` (`MAJOR`) | `advisory` (needs explicit disposition) | Do not treat a 3-second MSE append gap as completion when `targetDur > 0` and `mseMaxBufferedEnd < targetDur - 0.5` |
| 2 | U3 | `internal/browser/recap.go:301` | `LOW` | `P2` (`MINOR`) | `advisory` | Throw `isDownloadBlocked=true` error when `!candidates.length && !streamSurface && info.isDownloadBlocked` |
| 2 | U4 | `internal/browser/recap.go:437` | `LOW` | `P2` (`MINOR`) | `advisory` | Apply `sanitizeMediaUrl(selectedUrl)` on the direct-fetch return path |

## Bug / Issue Queue Entries

Ready for backlog creation for each `P0` and `P1` finding (not created automatically under `mode: report-only`):

```yaml
- type: bug
  title: "Invariant 2: Prevent child iframe from inheriting ancestor recording identity without own-frame match"
  description: "validateFrameContext sets baseIdentityMatched = state.matched || Boolean(ancestorMatched) and falls back to ancestorItemId, allowing an unverified child frame to pass identity checks."
  file: "internal/browser/recap.go"
  line: 133
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Remove ancestor identity/itemId inheritance in validateFrameContext; require positive own-frame or transcode-URL identity verification."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"

- type: bug
  title: "Invariant 2: Enforce https origin containment, exact header allowlist, and per-origin auth header map"
  description: "inspectTranscodeRequest accepts arbitrary http: origins, uses an x-ms- prefix wildcard, and stores capturedAuthHeaders in a single unkeyed map across hooked windows."
  file: "internal/browser/recap.go"
  line: 820
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Require https: (or explicit loopback allowance for local test fixtures), enumerate explicit header names, and key capturedAuthHeaders by origin."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"

- type: bug
  title: "Invariant 3: Enforce maxRecapAudioSegments=2400 in Path B and preserve limitError on Path A segment overflow"
  description: "recordMSESegment lacks a 2400-segment cap, and runPathA throws 'transcode segment count out of bounds' which falls back to uncapped Path B or masks the limit error with isDownloadBlocked=true."
  file: "internal/browser/recap.go"
  line: 717
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Declare maxRecapAudioSegments = 2400, enforce it before mseSegments.set in recordMSESegment, and throw limitError on out-of-bounds segment counts in runPathA."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"

- type: bug
  title: "Invariant 3: Enforce duration and PCM caps before decodeAudioData in Path B"
  description: "Path B only checks duration at entry when MSE media.duration is NaN, allowing long streams under 256 MiB source bytes to enter decodeAudioData before PCM/duration bounds are checked."
  file: "internal/browser/recap.go"
  line: 1377
  severity: "MAJOR"
  confidence: "MEDIUM"
  fix: "Re-check media.duration, info.duration, info.durationSeconds, and mseMaxBufferedEnd against maxRecapAudioDurationSec and maxRecapAudioPCMBytes prior to decodeAudioData."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"

- type: bug
  title: "Invariant 3: Abort and settle Path A workers before resetting streamSourceBytes on Path B fallback"
  description: "Non-fatal Path A worker errors trigger fallback and reset streamSourceBytes without aborting sibling Path A workers, causing concurrent streamSourceBytes races."
  file: "internal/browser/recap.go"
  line: 1013
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Use a dedicated Path A AbortController aborted on fallback and await worker settlement before resetting streamSourceBytes."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"

- type: bug
  title: "Correctness: Prevent premature MP3 truncation on 3-second MSE append gaps in Path B"
  description: "Path B exits the collection loop on any 3000 ms gap since mseLastAppendAt even when targetDur > 0 and mseMaxBufferedEnd < targetDur - 0.5 and !m.ended."
  file: "internal/browser/recap.go"
  line: 1322
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Require target duration reached or m.ended when targetDur > 0 rather than breaking early on a 3-second stall."
  linked_review: "docs/closure/2026-11-16-031-s-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 8
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
