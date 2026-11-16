---
title: Round 4 adversarial review for shipment 031-S (HEAD 685e08b)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: 685e08b3f43a1ebc0d445eaeabb15c93fdc46972
diff_range: 72ad055..HEAD (and readiness check over main..HEAD)
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (2 HIGH, 0 MEDIUM Majority, 0 MEDIUM Plurality, 3 LOW Unique)
readiness_verdict: BLOCKED
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Primary delta reviewed**: `72ad055..HEAD` (`685e08b3f43a1ebc0d445eaeabb15c93fdc46972` in `internal/browser/recap.go` and `internal/browser/recap_test.go`), addressing the 4 GitHub Copilot PR #76 review threads plus Round 3 items `U3-R3`, `U4-R3`, and `U6-R3`
* **Branch readiness scope**: `main..HEAD` (`685e08b3f43a1ebc0d445eaeabb15c93fdc46972`, 7 changed files across `docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Authorized contract scope (P-021 C1)**: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md` and `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `findings_summary`: `0` P0 (`CRITICAL`), `3` P1 (`MAJOR`: `2` HIGH Consensus + `1` LOW Unique), `2` P2 (`MINOR`: `2` LOW Unique)
  * `72ad055..HEAD` delta alone: `0` P0, `0` HIGH, `0` MEDIUM, `2` LOW Unique (`1` P1 `advisory`, `1` P2 `advisory`)
  * `readiness_verdict`: `BLOCKED` (2 HIGH-confidence `P1` Consensus findings on `main..HEAD`: `U1-R3` and `U2-R3`)

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 3 (3 MAJOR) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 2 (2 MAJOR) |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 4 (2 MAJOR, 2 MINOR) |

## Verification of `685e08b` Remediations (`72ad055..HEAD`)

| Item | Summary | Status at `685e08b` | Evidence |
|---|---|---|---|
| PR #76 Thread 1 | Contiguous MSE buffered coverage and gap rejection regardless of `media.ended` | **RESOLVED** | `mseContiguousEnd`, `mseHasBufferedGap`, `range0End` playback stepping, and `mseHasBufferedGap \|\| mseContiguousEnd < finalTargetDur - 0.5` (`recap.go:684-685, 837-852, 1554-1557, 1610-1621`); tested by `path-b-rejects-gapped-buffered-ranges-even-when-ended` (`recap_test.go:2548-2652`). |
| PR #76 Thread 2 + `U3-R3` | Persistent `cdpEvalSession` for `transferPending` pulls + fail on empty batch before `ChunkCount` | **RESOLVED** | `newCDPEvalSession`, `pullSess.eval` with `splice(0, 64)`, and `incomplete recap audio chunk transfer` error (`recap.go:1902-1936, 3240-3355`); tested by `transfer-pending-empty-batch-mid-transfer-returns-error` (`recap_test.go:2654-2737`). |
| PR #76 Thread 3 | Cross-realm `ArrayBuffer` / view support in `toUint8Array` | **RESOLVED** | Checks `playerWin.ArrayBuffer`, `Object.prototype.toString.call`, `playerWin.ArrayBuffer.isView`, and buffer view duck-typing (`recap.go:722-732`); tested by `path-b-cross-realm-arraybuffer-from-child-iframe` (`recap_test.go:2739-2877`). |
| PR #76 Thread 4 | Bounded `sb.remove(bufStart, cutoff)` eviction only when `cutoff > bufStart + 0.5` | **RESOLVED** | Uses `range0Start` and `cutoff = range0End - 30` gated by `cutoff > bufStart + 0.5` (`recap.go:853-857`); tested by `path-b-sb-remove-not-repeated-after-eviction` (`recap_test.go:2879-2988`). |
| `U4-R3` | Path A worker concurrency aligned to plan (`8`) + per-attempt 15s timeout and 2 retries | **RESOLVED** | `Math.min(8, totalSegments)` (`recap.go:1219`); `fetchBytesOnce` 15s `reqTimer` + `fetchBytesWithCap` 3-attempt loop (`recap.go:957-1113`). One `LOW` Unique `P1` advisory observation (`U1-R4`) remains on retry backoff cancellation. |
| `U6-R3` | `errPriority` ranks MSE incomplete and Path A deadline errors above `isDownloadBlocked=true` | **RESOLVED** | Both strings promoted to priority `2` in `errPriority` (`recap.go:1765-1767`). |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

### U1-R3 (`P1` / `MAJOR`): Target discovery loop breaks immediately when `lastErr == nil` before delayed `streamembed.aspx` mounts

* **Confidence**: `HIGH` (3/3: Anchor Reviewer, Reviewer-A, Reviewer-C)
* **Priority score**: `9` (`confidence_weight=3 × severity_weight=3`)
* **Action class**: `gated_auto`
* **Rule**: Protected Invariant 4 — Backward compatibility and delayed stream discovery
* **File / Line**: `internal/browser/recap.go:2003`
* **Issue**: When the outer recap frame returns an empty result (`!candidates.length && !streamSurface && !info.isDownloadBlocked`) and `streamembed.aspx` has not yet mounted (`hasStreamEmbedTarget == false`), `lastErr` remains `nil`, so `if lastErr == nil || !strings.Contains(lastErr.Error(), "isDownloadBlocked=true") || hasStreamEmbedTarget || attempt == maxAttempts-1` breaks on attempt `0` instead of polling up to `maxAttempts` (`8`).
* **Fix**: Continue polling while `!hasStreamEmbedTarget && attempt < maxAttempts-1` and either `lastErr == nil` or `strings.Contains(lastErr.Error(), "isDownloadBlocked=true")`, breaking on definitive fatal errors or when a `streamembed.aspx` target is present.

### U2-R3 (`P1` / `MAJOR`): Path B calls `decodeAudioData` without a bounded duration when `finalTargetDur == 0` and `mseMaxBufferedEnd == 0`

* **Confidence**: `HIGH` (3/3: Anchor Reviewer, Reviewer-A, Reviewer-C)
* **Priority score**: `9` (`confidence_weight=3 × severity_weight=3`)
* **Action class**: `manual`
* **Rule**: Protected Invariant 3 — Pre-allocation PCM and duration caps
* **File / Line**: `internal/browser/recap.go:1649` (guard at `recap.go:1619-1633`)
* **Issue**: When `media.duration`, `info.duration`, `info.durationSeconds`, `transcodePpd`, and `mseMaxBufferedEnd` are all `0` or non-finite, `finalTargetDur` evaluates to `0`, skipping the pre-decode completeness check and duration cap loop so `decodeAudioData(combined.buffer)` allocates decoded PCM before `4200` s / `360` MiB bounds are verified.
* **Fix**: Derive and validate a finite bounded duration from metadata, `mseMaxBufferedEnd` / `mseContiguousEnd`, or parsed fMP4 timing tables (`mvhd`/`mdhd`/`sidx`/`trun`/`tfdt`) before `decodeAudioData`, or fail closed when stream duration cannot be bounded.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

None. Zero findings were flagged by a majority of reviewers.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1-R4 (`P1` / `MAJOR`): Path A retry backoff timer is not abort-aware and is cleared without settling during `cleanupHooks`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Protected Invariant 5 — Clean restoration and resource cleanup
* **File / Line**: `internal/browser/recap.go:1106`
* **Issue**: In `fetchBytesWithCap`, the `250 * (attempt + 1)` ms retry backoff awaits a Promise resolved only by `setTimeout`. If `parentSignal` aborts (during `runPathA` error cancellation or deadline cleanup), the backoff does not reject early, and `cleanupHooks()` clears `activeTimers` without resolving or rejecting the timer Promise, which can leave Path A workers pending past the `250` ms cleanup settlement window.
* **Fix**: Register an abort listener on `parentSignal` (and reject/resolve on abort) and track or settle pending backoff promises before `cleanupHooks()` clears `activeTimers`.

### U2-R4 (`P2` / `MINOR`): `window.__goTeamsRecapAudioTransfer` is not cleared on early exits before the CDP pull loop

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Action class**: `advisory`
* **Rule**: Protected Invariant 5 — Clean restoration of `window.__goTeamsRecapAudioTransfer`
* **File / Line**: `internal/browser/recap.go:1860`
* **Issue**: After JS populates `window.__goTeamsRecapAudioTransfer` and returns `transferPending: true`, `cleanupTransfer()` is only constructed inside the `if payload.TransferPending` pull block (`recap.go:1906`). Early `continue` or `return` paths between JSON unmarshaling and the pull block (identity mismatch/unverified, `payload.Error != ""`, `SourceBytes`/`DurationSec` limit rejection, `ChunkCount <= 0`, or `newCDPEvalSession` failure) skip deleting `window.__goTeamsRecapAudioTransfer`, leaving up to `360` MiB of base64 chunks pinned on `window` until the next eval or page unload.
* **Fix**: When `payload.TransferPending` is `true`, schedule a best-effort cleanup of `window.__goTeamsRecapAudioTransfer` via `defer` or helper on every early exit path before the pull loop.

### U5-R3 (`P2` / `MINOR`): `extractRecapTranscriptJS` falls back to generic `g_fileInfo.id` / `docId` when populating `m.recapItemID`

* **Confidence**: `LOW` (1/3: Reviewer-C)
* **Priority score**: `2` (`confidence_weight=1 × severity_weight=2`)
* **Action class**: `advisory`
* **Rule**: Protected Invariant 2 — Per-frame identity isolation
* **File / Line**: `internal/browser/recap.go:2814` (consumed at `internal/browser/meeting_chrome.go:981, 1436-1439`)
* **Issue**: `extractRecapTranscriptJS` sets `itemId` using `fi.VroomItemID || fi.VroomItemId || fi.itemId || fi.driveItemId || fi.id || fi.docId`, and `meeting_chrome.go` attaches any alphanumeric/dash/underscore value as `driveItemId` for audio capture when the meeting URL omits `driveItemId`. A generic `fi.id` or `fi.docId` that is not a DriveItem ID can cause false identity mismatches or misattributed identity checks.
* **Fix**: Restrict `itemId` fallbacks in `extractRecapTranscriptJS` (or `meeting_chrome.go`) to `VroomItemID`, `VroomItemId`, `itemId`, and `driveItemId`, excluding generic `id` and `docId`.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Under full-pool multi-agent consensus (`active_reviewers = 3/3`), `2` `HIGH`-confidence `P1` (`MAJOR`) findings (`U1-R3`, `U2-R3`) block `main..HEAD` merge readiness (`mode: report-only` applied no edits).

| Priority | ID | File and Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 9 | U2-R3 | `internal/browser/recap.go:1649` | `HIGH` | `P1` (`MAJOR`) | `manual` | Require bounded duration from metadata, MSE buffered ranges, or fMP4 timing boxes before `decodeAudioData` when `finalTargetDur == 0` |
| 9 | U1-R3 | `internal/browser/recap.go:2003` | `HIGH` | `P1` (`MAJOR`) | `gated_auto` | Continue target discovery polling when `lastErr == nil` and `!hasStreamEmbedTarget` so late-mounting `streamembed.aspx` is polled |
| 3 | U1-R4 | `internal/browser/recap.go:1106` | `LOW` | `P1` (`MAJOR`) | `advisory` | Make `fetchBytesWithCap` retry backoff `parentSignal`-abort-aware and settle before `cleanupHooks` clears `activeTimers` |
| 2 | U2-R4 | `internal/browser/recap.go:1860` | `LOW` | `P2` (`MINOR`) | `advisory` | Delete `window.__goTeamsRecapAudioTransfer` on early exits between payload unmarshal and the CDP pull loop when `TransferPending` is true |
| 2 | U5-R3 | `internal/browser/recap.go:2814` | `LOW` | `P2` (`MINOR`) | `advisory` | Exclude generic `g_fileInfo.id` / `docId` fallbacks when populating `itemId` / `m.recapItemID` |

## Bug / Issue Queue Entries

Backlog work item templates for `P1` findings (`mode: report-only`; none created automatically):

```yaml
- type: bug
  title: "Invariant 3: Require bounded duration before Path B decodeAudioData when metadata is absent (U2-R3)"
  description: "When finalTargetDur == 0 and mseMaxBufferedEnd == 0, decodeAudioData executes before duration/PCM bounds are known."
  file: "internal/browser/recap.go"
  line: 1649
  severity: "MAJOR"
  confidence: "HIGH"
  fix: "Derive bounded duration from fMP4 timing tables or reject unknown-duration streams before decodeAudioData."
  linked_review: "docs/closure/2026-11-16-031-s-r4-adversarial-review.md"

- type: bug
  title: "Invariant 4: Continue recap discovery polling when pre-mount outer frame returns empty result (U1-R3)"
  description: "Target discovery exits on attempt 0 when lastErr == nil and streamembed.aspx has not mounted yet."
  file: "internal/browser/recap.go"
  line: 2003
  severity: "MAJOR"
  confidence: "HIGH"
  fix: "Retry up to maxAttempts while !hasStreamEmbedTarget and no definitive error occurred."
  linked_review: "docs/closure/2026-11-16-031-s-r4-adversarial-review.md"

- type: bug
  title: "Invariant 5: Make Path A retry backoff abort-aware during cleanupHooks (U1-R4)"
  description: "fetchBytesWithCap retry backoff uses setTimeout without parentSignal abort handling, so cleanupHooks clearing activeTimers leaves the Promise unsettled."
  file: "internal/browser/recap.go"
  line: 1106
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Reject or resolve the retry backoff immediately when parentSignal aborts."
  linked_review: "docs/closure/2026-11-16-031-s-r4-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 5
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
