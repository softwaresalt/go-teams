---
title: Round 3 adversarial review for shipment 031-S (HEAD 72ad055)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: 72ad05539346bf9c43a98935726a5358da9cb84d
diff_range: main..HEAD
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 0 MEDIUM Majority, 0 MEDIUM Plurality, 6 LOW Unique)
readiness_verdict: READY_WITH_FOLLOWUPS
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Authorized contract scope (P-021 C1)**:
  * Plan: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`
  * Deliberation: `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`
  * Diff: `main..HEAD` (`72ad05539346bf9c43a98935726a5358da9cb84d`), 7 changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `readiness_verdict`: `READY_WITH_FOLLOWUPS` (0 `P0`, 0 `HIGH`, 0 `MEDIUM`, 6 `LOW` Unique advisory observations: 3 `P1` `advisory` in full-pool mode and 3 `P2` `advisory`)

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 5 (3 MAJOR, 2 MINOR) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 1 (1 MINOR) |

## Verification of Prior R2 Findings (`M1-R2`, `U1-R2`, `U2-R2`, `U3-R2`)

| Prior ID | Summary | Status at `72ad055` | Evidence |
|---|---|---|---|
| `M1-R2` | Unvalidated `info.transcodeUrl` pre-seed bypassing HTTPS/same-origin checks (`recap.go:632`) | **RESOLVED** | `isAllowedSameOriginTranscodeURL` validates `https:` (or explicit loopback `http:`) and `u.origin === winOrigin` when seeding `transcodeAudioTemplateUrl` (`recap.go:632-662`), in `inspectTranscodeRequest` (`recap.go:866`), in `fetchBytesWithCap` (`recap.go:940-942`), and at `runPathA` entry (`recap.go:1043-1045`); covered by `M1-R2-info-transcodeUrl-origin-and-protocol-validation` (`recap_test.go:2200-2271`). |
| `U1-R2` | `maybeStartPathA` fallback resuming `16x` `m.play()` after `cleanupHooks()` on deadline/abort (`recap.go:886`) | **RESOLVED** | `cleaningUp` flag guards `maybeStartPathA().catch` alongside `hooksRestored` (`recap.go:689, 909, 920`), and `finally` sets `cleaningUp = true`, aborts `abortCtrl`, and races `Promise.allSettled([pathAPromise])` with a tracked 250 ms timer before `cleanupHooks()` (`recap.go:1591-1612`); covered by `U1-R2-deadline-aborts-path-a-without-post-cleanup-play` (`recap_test.go:2274-2359`). |
| `U2-R2` | Pre-hook native `video/mp4` `SourceBuffer` without `.mime` leaking into `mseSegments` (`recap.go:1284`) | **RESOLVED** | `sbTrackMap` (`WeakMap`, `recap.go:680`) tracks `'audio'`, `'video'`, or `'unknown'` from `addSourceBuffer` (`recap.go:1288-1293`) and `classifyInitSegmentTrack` (`recap.go:1318-1352`), accepting media segments and `mseMaxBufferedEnd` updates only when `sbTrackMap.get(this) === 'audio'`; covered by `U2-R2-pre-hook-native-video-sourcebuffer-without-mime-ignored` (`recap_test.go:2362-2446`). |
| `U3-R2` | Path B returning truncated audio when `deadlineAt` expires before `targetDur` (`recap.go:1352`) | **RESOLVED** | Throws `'recap audio Path A capture deadline exceeded'` when Path A is still pending at exit (`recap.go:1493-1495`) and `'recap audio MSE stream incomplete before deadline'` when `finalTargetDur > 0 && !(curMedia && curMedia.ended) && mseMaxBufferedEnd < finalTargetDur - 0.5` (`recap.go:1507-1520`), both propagated past `isDownloadBlocked=true` inside `extractRecapAudioJS` (`recap.go:1621-1622`); covered by `U3-R2-path-b-rejects-truncated-stream-at-deadline` (`recap_test.go:2449-2524`). |

## Protected Invariants and Redaction Verification

| Check | Status | Notes |
|---|---|---|
| Invariant 1: Single-binary pure-Go | PASS | Zero external binaries (`ffmpeg`, `yt-dlp`), zero CGO, zero `go.mod` / `go.sum` changes. |
| Invariant 2: Per-frame identity isolation and origin containment | PASS | `M1-R2` resolved; one `LOW` Unique `P2` advisory observation on `meeting_chrome.go:981` (`recapItemID` accepting generic `g_fileInfo.id` / `docId` fallback). |
| Invariant 3: Pre-allocation resource and segment-count caps + error precedence | PASS | `M3`, `M4`, `U2-R2`, `U3-R2` resolved; two `LOW` Unique advisory observations remain (`U2-R3` pre-decode cap when `finalTargetDur == 0`; `U6-R3` Go `errPriority` ordering between Path A/B timeout errors and `isDownloadBlocked=true`). |
| Invariant 4: Backward compatibility (direct-fetch and `isDownloadBlocked=true`) | PASS | Direct-fetch and inert blocked-page contracts intact; one `LOW` Unique `P1` advisory observation on `recap.go:1886` (`lastErr == nil` exiting the 8-attempt discovery loop early if `streamembed.aspx` mounts late). |
| Invariant 5: Clean restoration of browser globals, timers, `AbortController`, and `__goTeamsRecapAudioTransfer` | PASS | `U1-R2` resolved; `cleanupHooks()` and `__goTeamsRecapAudioTransfer` deletion are idempotent. |
| Strict redaction | PASS | Zero live SharePoint URLs, tenant hostnames, UPNs, passcodes, or tokens in any tracked file; all URLs, `tempauth=` values, and UPNs are synthetic `contoso` test fixtures or doc placeholders. |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

None. Zero findings were flagged by a majority of reviewers.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1-R3 (`P1` / `MAJOR`): Target discovery loop breaks immediately when `lastErr == nil` before delayed `streamembed.aspx` mounts

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Protected Invariant 4 — Delayed stream surface compatibility
* **File / Line**: `internal/browser/recap.go:1886` (reported at `recap.go:1894`)
* **Issue**: When the outer recap page returns `''` (`!candidates.length && !streamSurface && !info.isDownloadBlocked`) and `streamembed.aspx` has not yet mounted (`hasStreamEmbedTarget == false`), `lastErr` remains `nil`, so `if lastErr == nil || !strings.Contains(lastErr.Error(), "isDownloadBlocked=true") || hasStreamEmbedTarget || attempt == maxAttempts-1` breaks on the first attempt instead of polling up to `maxAttempts` (`8`).
* **Fix**: Retry while `!hasStreamEmbedTarget` and no definitive identity, resource-limit, or direct-fetch error occurred; do not treat an empty pre-mount result as terminal on early attempts.

### U2-R3 (`P1` / `MAJOR`): Path B calls `decodeAudioData` when `finalTargetDur == 0` and `mseMaxBufferedEnd == 0`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Protected Invariant 3 — Pre-allocation PCM and duration caps
* **File / Line**: `internal/browser/recap.go:1517`
* **Issue**: When `media.duration`, `info.duration`, `info.durationSeconds`, `transcodePpd`, and `mseMaxBufferedEnd` are all `0` or non-finite, `finalTargetDur` evaluates to `0`, allowing `decodeAudioData` to decode an audio stream before duration/PCM bounds are verified against `4200` s / `360` MiB.
* **Fix**: Require a finite bounded duration derived from metadata, `mseMaxBufferedEnd`, or parsed fMP4 `mvhd`/`tkhd`/`mdhd`/`sidx`/`trun` sample timing before calling `decodeAudioData`, or reject streams with unknown duration.

### U3-R3 (`P1` / `MAJOR`): CDP chunked transfer loop treats empty batch as normal completion when `pulled < payload.ChunkCount`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Correctness — Complete Go/CDP chunk transfer
* **File / Line**: `internal/browser/recap.go:1824`
* **Issue**: In the `for pulled := 0; pulled < payload.ChunkCount;` loop, `if len(batch) == 0 { break }` exits normally. If at least one batch was already pulled (`size > 0`) and the page navigates or `window.__goTeamsRecapAudioTransfer` disappears before all `ChunkCount` chunks are pulled, `captureRecapAudioFromTargets` flushes `enc` and returns `nil` with silently truncated MP3 output.
* **Fix**: Return an explicit error whenever `len(batch) == 0` while `pulled < payload.ChunkCount` so partial chunk transfers never report success.

### U4-R3 (`P2` / `MINOR`): Path A launches 16 concurrent workers instead of 8 specified by plan

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Action class**: `advisory`
* **Rule**: P-021 C1 — Path A bounded concurrency contract
* **File / Line**: `internal/browser/recap.go:1119`
* **Issue**: `Math.min(16, totalSegments)` starts up to 16 concurrent `oneDrive.transcode` fetch/decrypt workers whereas the deliberation and plan contract specified bounded concurrency `8`.
* **Fix**: Align worker cap to `Math.min(8, totalSegments)` or document the 16-worker concurrency choice in the plan/deliberation if intentional.

### U5-R3 (`P2` / `MINOR`): `recapItemID` retains fallback `g_fileInfo.id` / `docId` values from `extractRecapTranscriptJS`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Action class**: `advisory`
* **Rule**: Protected Invariant 2 — Verified transcript item identity
* **File / Line**: `internal/browser/meeting_chrome.go:981`
* **Issue**: `extractRecapTranscriptJS` populates `rawPayload.ItemID` using `fi.VroomItemID || fi.VroomItemId || fi.itemId || fi.driveItemId || fi.id || fi.docId`, and `meeting_chrome.go:981` accepts any alphanumeric/dash/underscore string into `m.recapItemID`, which `CaptureAudio` later attaches as `driveItemId`.
* **Fix**: Restrict `m.recapItemID` retention to IDs sourced from `VroomItemID`, `VroomItemId`, `itemId`, or `driveItemId` (excluding generic `id`/`docId`).

### U6-R3 (`P2` / `MINOR`): Go `errPriority` ranks Path A/B deadline and incomplete-stream errors below `isDownloadBlocked=true`

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Action class**: `advisory`
* **Rule**: Protected Invariant 3 — Error precedence (`setLastErr` / `errPriority`)
* **File / Line**: `internal/browser/recap.go:1658`
* **Issue**: `errPriority` assigns priority `0` to `'recap audio MSE stream incomplete before deadline'` and `'recap audio Path A capture deadline exceeded'`, while `'isDownloadBlocked=true'` has priority `1`. A later-iterated inert frame (such as `xplatplugins.aspx`) can overwrite a real stream timeout or incomplete-stream failure with `'isDownloadBlocked=true'`.
* **Fix**: Assign priority `2` (or `>= 2` / distinct tier above `isDownloadBlocked=true`) to `'MSE stream incomplete'` and `'Path A capture deadline exceeded'` in `errPriority`, and add a Go unit test verifying precedence across multi-target evaluations.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Under full-pool multi-agent consensus (`active_reviewers = 3/3`), zero `HIGH` or `MEDIUM` findings remain; all 6 `LOW` Unique findings are `advisory` follow-ups (`mode: report-only` applied no edits).

| Priority | ID | File and Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 3 | U2-R3 | `internal/browser/recap.go:1517` | `LOW` | `P1` (`MAJOR`) | `advisory` | Require bounded duration from metadata or parsed fMP4 boxes before `decodeAudioData` when `finalTargetDur == 0` |
| 3 | U3-R3 | `internal/browser/recap.go:1824` | `LOW` | `P1` (`MAJOR`) | `advisory` | Return an error if CDP chunk pull gets an empty batch with `pulled < payload.ChunkCount` |
| 3 | U1-R3 | `internal/browser/recap.go:1886` | `LOW` | `P1` (`MAJOR`) | `advisory` | Retry target discovery when `lastErr == nil` and `!hasStreamEmbedTarget` so late-mounting `streamembed.aspx` is polled |
| 2 | U5-R3 | `internal/browser/meeting_chrome.go:981` | `LOW` | `P2` (`MINOR`) | `advisory` | Exclude generic `g_fileInfo.id` / `docId` fallbacks when populating `m.recapItemID` |
| 2 | U4-R3 | `internal/browser/recap.go:1119` | `LOW` | `P2` (`MINOR`) | `advisory` | Align Path A worker count (`16`) with plan specification (`8`) or update plan rationale |
| 2 | U6-R3 | `internal/browser/recap.go:1658` | `LOW` | `P2` (`MINOR`) | `advisory` | Rank Path A/B deadline and incomplete-stream errors above `isDownloadBlocked=true` in `errPriority` |

## Bug / Issue Queue Entries

Advisory follow-up backlog templates for operator triage (`mode: report-only`; none created automatically):

```yaml
- type: bug
  title: "Correctness: Fail closed on incomplete CDP recap audio chunk transfer"
  description: "Empty batch in captureRecapAudioFromTargets breaks the chunk-pull loop and flushes a partial MP3 when pulled < payload.ChunkCount."
  file: "internal/browser/recap.go"
  line: 1824
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Return an error when len(batch) == 0 before pulled == payload.ChunkCount."
  linked_review: "docs/closure/2026-11-16-031-s-r3-adversarial-review.md"

- type: bug
  title: "Invariant 4: Continue recap discovery polling when pre-mount outer frame returns empty result"
  description: "Target discovery exits on attempt 0 when lastErr == nil and streamembed.aspx has not mounted yet."
  file: "internal/browser/recap.go"
  line: 1886
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Retry up to maxAttempts while !hasStreamEmbedTarget and no definitive error occurred."
  linked_review: "docs/closure/2026-11-16-031-s-r3-adversarial-review.md"

- type: bug
  title: "Invariant 3: Require bounded duration before Path B decodeAudioData when metadata is absent"
  description: "When finalTargetDur == 0 and mseMaxBufferedEnd == 0, decodeAudioData executes before duration/PCM bounds are known."
  file: "internal/browser/recap.go"
  line: 1517
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Derive bounded duration from fMP4 timing tables or reject unknown-duration streams before decodeAudioData."
  linked_review: "docs/closure/2026-11-16-031-s-r3-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 6
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
