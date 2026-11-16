---
title: Round 6 adversarial review for shipment 031-S (HEAD ace434f)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: ace434f88f718ba56bfc3505ed8ea30ad52f497c
diff_range: main..HEAD (delta ed092e5..HEAD)
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 0 MEDIUM Majority, 0 MEDIUM Plurality, 1 LOW Unique on ed092e5..HEAD; 2 prior LOW Unique advisory carried over from R5)
readiness_verdict: READY_WITH_FOLLOWUPS
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Primary delta reviewed**: `ed092e5..HEAD` (`ace434f88f718ba56bfc3505ed8ea30ad52f497c`), modifying only `internal/browser/meeting_e2e_test.go` (`1` insertion, `3` deletions)
* **Full shipment scope confirmed**: `main..HEAD` across all `7` changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`); production code and docs remain byte-identical to `ed092e5` reviewed in Round 5 (`docs/closure/2026-11-16-031-s-r5-adversarial-review.md`)
* **Authorized contract scope (P-021 C1)**: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`, `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`, Protected Invariants 1..5 in `## Plan Hardening`, and strict redaction (zero live URLs, hostnames, UPNs, passcodes, or tokens)
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `findings_summary` (`ed092e5..HEAD`): `0` `P0` (`CRITICAL`), `1` `P1` (`MAJOR`: `0` HIGH Consensus, `0` MEDIUM Majority/Plurality, `1` LOW Unique `advisory`), `0` `P2` (`MINOR`)
  * `findings_summary` (`main..HEAD` combined): `0` `P0` (`CRITICAL`), `3` `P1` (`MAJOR`: `0` HIGH Consensus, `0` MEDIUM Majority/Plurality, `3` LOW Unique `advisory` — `U1-R6` on `meeting_e2e_test.go:568` plus carried-over `U1-R5` and `U2-R5` stashed as `7883544E`), `0` `P2` (`MINOR`)
  * `readiness_verdict`: `READY_WITH_FOLLOWUPS`

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 0 |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 1 (1 MAJOR) |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 0 |

## Verification of `ed092e5..HEAD` (`ace434f`) and Shipment Readiness

| Surface | Summary | Status at `ace434f` | Evidence |
|---|---|---|---|
| `internal/browser/meeting_e2e_test.go` (`ed092e5..HEAD`) | Await `track=video` strip probe after `initBuf` decryption and before `seg0` / Path A completion instead of firing inside `setTimeout(..., 20)` | **VERIFIED** (`0` HIGH/MEDIUM findings; `1` LOW Unique `advisory` `U1-R6`) | `initBuf` transcode fetch sets `transcodeAudioTemplateUrl`, `transcodeWsd`, and `transcodePpd` (`recap.go:903-921`), and `initBuf` `subtle.decrypt` populates `audioCryptoKeyInfo` and invokes `maybeStartPathA()` (`recap.go:925-930`). Awaiting the `track=video` probe immediately afterward ensures `pathAStarted` is already `true` and prevents `cleanupHooks()` from restoring `window.fetch` before a `20ms` timer fires on Windows CI. |
| Production code & docs (`recap.go`, `meeting_chrome.go`, `docs/`) | Byte-identical to `ed092e5` | **PASS** (carries Round 5 `LOW` Unique `advisory` items `U1-R5`, `U2-R5` stashed as `7883544E`) | Verified by `git diff ed092e5..HEAD` touching only `internal/browser/meeting_e2e_test.go`. |
| Invariants 1..5 and Strict Redaction | Pure-Go single binary, identity/origin containment, resource/segment caps, backward compatibility, clean hook restoration, and zero live secret/URL leaks | **PASS** | Zero `go.mod`/`go.sum` changes, all 5 invariants upheld, and all fixture URLs remain local `httptest` / synthetic `contoso` fixtures. |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

None. Zero findings were flagged by a strict majority of reviewers.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### U1-R6 (`P1` / `MAJOR`): E2E test awaits `track=video` strip probe before `seg0` instead of on a 20ms timer

* **Confidence**: `LOW` (1/3: Reviewer-A)
* **Priority score**: `3` (`confidence_weight=1 × severity_weight=3`)
* **Action class**: `advisory` (full-pool mode: `active_reviewers == reviewers`; non-blocking)
* **Rule**: Test Correctness — Concurrent Request Timing
* **File / Line**: `internal/browser/meeting_e2e_test.go:568` (reported by Reviewer-A at line `572`)
* **Issue**: Moving the `track=video` probe from `setTimeout(..., 20)` to an awaited `fetch` before `seg0` changes the timing of the mock video request from a delayed asynchronous callback to a deterministic pre-`seg0` request, reducing overlap with later `seg0` handling.
* **Fix / Disposition**: Non-blocking `advisory` note only. Code inspection of `internal/browser/recap.go:903-930` confirms `transcodeAudioTemplateUrl`, `transcodeWsd`, `transcodePpd`, and `maybeStartPathA()` are already active after `initBuf` decryption, so awaiting the probe at line `568` exercises `isVideoSegment` stripping while Path A is active and eliminates the Windows CI race against `cleanupHooks()`. If a concurrent post-`seg0` video strip test is desired later, add a dedicated deterministic unit case rather than an unawaited `setTimeout`.

### Carried-Over Round 5 Unique Findings (`main..ed092e5`, unchanged in `ace434f`)

* **`U1-R5` (`P1` / `MAJOR`, `LOW` confidence, `advisory`, stashed as `7883544E`)**: `internal/browser/recap.go:1053` — Track committed-plus-in-flight bytes across concurrent Path A workers so combined buffered chunks cannot exceed `maxRecapAudioSourceBytes`.
* **`U2-R5` (`P1` / `MAJOR`, `LOW` confidence, `advisory`, stashed as `7883544E`)**: `internal/browser/recap.go:2060` — Remove `hasXplatTarget` gate on `lastErr == nil` retries so delayed non-xplat `streamembed.aspx` mounts are polled across `maxAttempts`.

## Remediation Plan

Ordered descending by `priority = confidence_weight × severity_weight`, then by file path. Under full-pool multi-agent consensus (`active_reviewers = 3/3`), zero `HIGH`-confidence or `MEDIUM`-confidence findings exist across `main..HEAD`, and all `LOW`-confidence `P1` observations carry action class `advisory` (`mode: report-only` applied no edits).

| Priority | ID | File and Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 3 | U1-R6 | `internal/browser/meeting_e2e_test.go:568` | `LOW` | `P1` (`MAJOR`) | `advisory` | Note timing trade-off of awaiting `track=video` strip probe before `seg0` vs concurrent post-`seg0` test coverage (non-blocking; fixes Windows CI `cleanupHooks()` race) |
| 3 | U1-R5 | `internal/browser/recap.go:1053` | `LOW` | `P1` (`MAJOR`) | `advisory` | Carried over from R5 (stashed `7883544E`): track committed-plus-in-flight bytes across concurrent Path A workers |
| 3 | U2-R5 | `internal/browser/recap.go:2060` | `LOW` | `P1` (`MAJOR`) | `advisory` | Carried over from R5 (stashed `7883544E`): remove `hasXplatTarget` gate on `lastErr == nil` retries |

## Bug / Issue Queue Entries

Backlog work item templates for `P1` findings (`mode: report-only`; `U1-R5` and `U2-R5` already tracked under stash `7883544E`; none created automatically):

```yaml
- type: bug
  title: "Test Correctness: Document or supplement pre-seg0 awaited track=video probe in E2E test (U1-R6)"
  description: "TestChromeRecapDownloadBlockedStreamEmbedAudioEndToEnd awaits the track=video strip probe immediately after initBuf decrypt rather than concurrently with seg0, avoiding the Windows CI cleanupHooks() race while exercising Path A video stripping."
  file: "internal/browser/meeting_e2e_test.go"
  line: 568
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Keep deterministic pre-cleanup await in the E2E test; add a synchronized unit case if concurrent post-seg0 video stripping needs separate coverage."
  linked_review: "docs/closure/2026-11-16-031-s-r6-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 3
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
