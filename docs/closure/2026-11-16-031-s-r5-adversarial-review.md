---
title: Round 5 adversarial review for shipment 031-S (HEAD ed092e5)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: ed092e5b05d170b8a4318acae47a2effc641e825
diff_range: main..HEAD
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 0 MEDIUM Majority, 0 MEDIUM Plurality, 2 LOW Unique)
readiness_verdict: READY_WITH_FOLLOWUPS
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Diff reviewed**: `main..HEAD` (`ed092e5b05d170b8a4318acae47a2effc641e825`), including `685e08b3f43a1ebc0d445eaeabb15c93fdc46972` and `ed092e5b05d170b8a4318acae47a2effc641e825` across all 7 changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Authorized contract scope (P-021 C1)**: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`, `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`, Protected Invariants 1..5 in `## Plan Hardening`, and strict redaction (zero live URLs, hostnames, UPNs, passcodes, or tokens)
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `findings_summary`: `0` `P0` (`CRITICAL`), `2` `P1` (`MAJOR`: `0` HIGH Consensus, `0` MEDIUM Majority/Plurality, `2` LOW Unique `advisory`), `0` `P2` (`MINOR`)
  * `readiness_verdict`: `READY_WITH_FOLLOWUPS`

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 2 (2 MAJOR) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 0 |

## Verification of Round 4 Remediations and Protected Invariants (`ed092e5`)

| Item | Summary | Status at `ed092e5` | Evidence |
|---|---|---|---|
| `U2-R3` | Reject Path B `decodeAudioData` when `finalTargetDur <= 0 && !(mseMaxBufferedEnd > 0)` | **RESOLVED** | Throws `'recap audio MSE stream duration unverified'` (`recap.go:1645-1647`), checks `mseMaxBufferedEnd` against `maxRecapAudioDurationSec`/`maxRecapAudioPCMBytes` (`recap.go:1654-1659`), propagates past `blockedErr` (`recap.go:1753`), and ranks priority `2` in `errPriority` (`recap.go:1795`). |
| `U1-R3` | Delayed `streamembed.aspx` mount polling when `lastErr == nil` | **RESOLVED FOR `xplatplugins.aspx` TOPOLOGIES** (1 `LOW` Unique `P1` advisory follow-up `U2-R5`) | `shouldRetry` retries when `!hasStreamEmbedTarget && attempt < maxAttempts-1 && ((lastErr != nil && isDownloadBlocked) \|\| (lastErr == nil && hasXplatTarget))` (`recap.go:1842-1854, 2060-2063`). `U2-R5` notes that `hasXplatTarget` excludes non-xplat delayed-mount topologies when `lastErr == nil`. |
| `U1-R4` | Abort-aware Path A retry backoff during `cleanupHooks` | **RESOLVED** | `fetchBytesWithCap` registers `onBackoffAbort` on `parentSignal`, clears `tid`, removes it from `activeTimers`, and rejects immediately (`recap.go:1105-1135`). |
| `U2-R4` | Clear `window.__goTeamsRecapAudioTransfer` on early exits before CDP pull loop | **RESOLVED** | `cleanupPendingTransfer()` executes best-effort deletion on identity mismatch/unverified, `payload.Error != ""`, size/duration limit failure, `ChunkCount <= 0`, `newCDPEvalSession` failure, and pull loop exits (`recap.go:1908-1960`). |
| `U5-R3` | Exclude generic `g_fileInfo.id` / `docId` from `m.recapItemID` retention | **RESOLVED** | `extractRecapTranscriptJS` emits dedicated `driveItemId` (`VroomItemID \|\| VroomItemId \|\| itemId \|\| driveItemId`) and `meeting_chrome.go:981` reads `rawPayload.DriveItemID` (`recap.go:2875, 2938`). |
| Invariants 1..5 and Strict Redaction | Pure-Go single binary, identity/origin containment, resource/segment caps, backward compatibility, clean hook restoration, and zero live secret/URL leaks | **PASS** (2 `LOW` Unique `P1` advisory observations: `U1-R5`, `U2-R5`) | Zero `go.mod`/`go.sum` changes, all 5 invariants upheld, and all URLs/UPNs/tokens in tracked diffs use synthetic `contoso` fixtures or doc placeholders. |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

None. Zero findings were flagged by a strict majority of reviewers.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1-R5 (`P1` / `MAJOR`): Path A concurrent workers buffer segments before committing `streamSourceBytes`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Protected Invariant 3 — Pre-Allocation Resource and Segment-Count Caps
* **File / Line**: `internal/browser/recap.go:1053`
* **Issue**: In `fetchBytesOnce`, each of the up to 8 concurrent Path A workers checks `streamSourceBytes + total + v.byteLength > maxRecapAudioSourceBytes` while streaming chunks, but updates `streamSourceBytes += total` only after the full response completes (`recap.go:1068`). Multiple concurrent workers can therefore each buffer up to `maxRecapAudioSourceBytes` (`256` MiB) simultaneously before any worker commits its total.
* **Fix**: Reserve bytes in a shared committed-plus-in-flight counter as each chunk arrives, and release un-committed in-flight reservations on failure or retry.

### U2-R5 (`P1` / `MAJOR`): Discovery retry loop gates `lastErr == nil` on `hasXplatTarget`

* **Confidence**: `LOW` (1/3: Anchor Reviewer)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`)
* **Rule**: Protected Invariant 4 — Backward Compatibility and Delayed Stream Discovery
* **File / Line**: `internal/browser/recap.go:2060`
* **Issue**: `shouldRetry` requires `hasXplatTarget` when `lastErr == nil` (`(lastErr == nil && hasXplatTarget)`). If `streamembed.aspx` mounts directly (or without an `xplatplugins.aspx` target present on the initial poll) and the outer page returns an empty result (`lastErr == nil`), the loop breaks after attempt `0` instead of polling up to `maxAttempts`.
* **Fix**: Retry while `!hasStreamEmbedTarget && attempt < maxAttempts-1` whenever `lastErr == nil` or `strings.Contains(lastErr.Error(), "isDownloadBlocked=true")`, without requiring `hasXplatTarget`.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Under full-pool multi-agent consensus (`active_reviewers = 3/3`), zero `HIGH`-confidence or `MEDIUM`-confidence findings remain, and both `LOW`-confidence `P1` observations (`U1-R5`, `U2-R5`) carry action class `advisory` (`mode: report-only` applied no edits).

| Priority | ID | File and Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 3 | U1-R5 | `internal/browser/recap.go:1053` | `LOW` | `P1` (`MAJOR`) | `advisory` | Track committed-plus-in-flight bytes across concurrent Path A workers so combined buffered chunks cannot exceed `maxRecapAudioSourceBytes` |
| 3 | U2-R5 | `internal/browser/recap.go:2060` | `LOW` | `P1` (`MAJOR`) | `advisory` | Remove `hasXplatTarget` gate on `lastErr == nil` retries so delayed non-xplat `streamembed.aspx` mounts are polled across `maxAttempts` |

## Bug / Issue Queue Entries

Backlog work item templates for `P1` findings (`mode: report-only`; none created automatically):

```yaml
- type: bug
  title: "Invariant 3: Reserve in-flight bytes across concurrent Path A workers before completion (U1-R5)"
  description: "fetchBytesOnce updates streamSourceBytes only after a response finishes, allowing up to 8 concurrent Path A workers to buffer multiple segments simultaneously before the shared 256 MiB cap commits."
  file: "internal/browser/recap.go"
  line: 1053
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Track a shared committed-plus-in-flight byte budget incremented per chunk and released on error/retry."
  linked_review: "docs/closure/2026-11-16-031-s-r5-adversarial-review.md"

- type: bug
  title: "Invariant 4: Poll delayed streamembed.aspx mounts when lastErr == nil without requiring hasXplatTarget (U2-R5)"
  description: "shouldRetry gates lastErr == nil polling on hasXplatTarget, skipping retries on direct delayed-mount topologies."
  file: "internal/browser/recap.go"
  line: 2060
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Allow retries when !hasStreamEmbedTarget && attempt < maxAttempts-1 for lastErr == nil regardless of hasXplatTarget."
  linked_review: "docs/closure/2026-11-16-031-s-r5-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 2
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
