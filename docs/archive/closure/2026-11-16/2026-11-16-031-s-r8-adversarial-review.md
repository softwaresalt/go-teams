---
title: Round 8 adversarial review for shipment 031-S (HEAD 5f3ec41)
date: 2026-11-16
feature: 031-F
shipment: 031-S
head_sha: 5f3ec4160481a21e58d2e945bd0357c599c7d574
diff_range: main..HEAD (delta e27ff89..HEAD)
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH, 0 MEDIUM Majority, 0 MEDIUM Plurality, 1 LOW Unique P2 on e27ff89..HEAD; 3 carried-over LOW Unique P1 advisory on main..e27ff89)
readiness_verdict: READY_WITH_FOLLOWUPS
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped)
* **Primary delta reviewed**: `e27ff89..HEAD` (`5f3ec4160481a21e58d2e945bd0357c599c7d574`), modifying `internal/browser/recap.go` and `internal/browser/recap_test.go`
* **Full shipment scope confirmed**: `main..HEAD` across all `7` changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/meeting_chrome.go`, `internal/browser/meeting_e2e_test.go`, `internal/browser/meeting_integration_test.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go`)
* **Authorized contract scope (P-021 C1)**: `docs/plans/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-plan.md`, `docs/decisions/2026-11-16-download-blocked-sharepoint-recap-high-speed-audio-deliberation.md`, Protected Invariants 1..5 in `## Plan Hardening`, and strict redaction (zero live URLs, hostnames, UPNs, passcodes, or tokens)
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `findings_summary` (`e27ff89..HEAD`): `0` `P0` (`CRITICAL`), `0` `P1` (`MAJOR`), `1` `P2` (`MINOR`, `LOW` Unique `advisory` — `U1-R8`)
  * `findings_summary` (`main..HEAD` combined): `0` `P0` (`CRITICAL`), `3` `P1` (`MAJOR`: `0` HIGH Consensus, `0` MEDIUM Majority/Plurality, `3` carried-over `LOW` Unique `advisory` from R5/R6 — `U1-R5`, `U2-R5` stashed as `7883544E`, and `U1-R6`), `1` `P2` (`MINOR`, `LOW` Unique `advisory` — `U1-R8`)
  * `readiness_verdict`: `READY_WITH_FOLLOWUPS`

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
|---|---|---|---|---|---|
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-5.6-sol` | `high` | Settled (valid JSON) | 1 (`MINOR` / `P2`) |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | runtime default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (frontier) | `claude-sonnet-5.5` (highest available Anthropic model in tool enum for `claude-opus-4.8`) | runtime default | Settled (valid JSON) | 0 |

## Verification of `e27ff89..HEAD` (`5f3ec41`) and Shipment Readiness

| Surface | Summary | Status at `5f3ec41` | Evidence |
|---|---|---|---|
| XHR property descriptor tracking and restoration (`internal/browser/recap.go:1368-1446`) | Saves pre-existing own descriptors via `overriddenXhrs = new Map()` before overriding `readyState`, `status`, `response`, and `responseText`, and restores them via `restoreXHRProps(xhr)` on `proto.open` and during `cleanupHooks()` (`PRRT_kwDOTStvas64MQNJ`) | **VERIFIED** (`0` P0/P1; `1` `LOW` `P2` advisory `U1-R8`) | Restores both prototype-backed XHR instances (via `delete`) and pre-existing own descriptors (via `Object.defineProperty`), resets `xhrMeta = new WeakMap()`, and clears `overriddenXhrs`. |
| XHR reuse and cleanup regression test (`internal/browser/recap_test.go:3587-3769`) | Subtest `xhr-restores-overridden-props-on-reuse-and-cleanup` covers reused XHR (`video` -> `open()` -> `audio`), unreused suppressed XHR restored by `cleanupHooks()`, and pre-existing own `status` descriptor restoration | **VERIFIED** (`0` findings) | Synthetic `contoso-my.sharepoint.com` fixture only; strict redaction maintained. |
| Prior `main..e27ff89` surfaces (`docs/`, `meeting_chrome.go`, `meeting_e2e_test.go`, `meeting_integration_test.go`, remaining `recap.go` / `recap_test.go`) | Unchanged since Round 7 (`e27ff89`) | **PASS** (carries R5/R6 `LOW` Unique `advisory` items `U1-R5`, `U2-R5`, `U1-R6`) | Verified by `git diff e27ff89..HEAD` touching only `internal/browser/recap.go` and `internal/browser/recap_test.go`. |
| Invariants 1..5 and Strict Redaction | Pure-Go single binary, identity/origin containment, resource/segment caps, backward compatibility, clean hook restoration, and zero live secret/URL leaks | **PASS** | Zero `go.mod`/`go.sum` changes, all 5 invariants upheld, and zero live URLs or credentials in committed diffs. |

## Consensus Findings (HIGH Confidence — 3/3 Reviewers)

None. Zero findings were flagged unanimously by all 3 reviewers.

## Majority Findings (MEDIUM Confidence — 2/3 Reviewers)

None. Zero findings were flagged by a strict majority of reviewers.

## Plurality Findings (MEDIUM Confidence — >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence — 1/3 Reviewers)

### New on `e27ff89..HEAD` (`5f3ec41`)

* **`U1-R8` (`P2` / `MINOR`, `LOW` confidence, `advisory`, Anchor Reviewer `gpt-5.6-sol`)**: `internal/browser/recap.go:1386` — Reopening a suppressed XHR via `proto.open` restores its overridden properties (`restoreXHRProps(this)`) but does not cancel or invalidate a still-pending `setTimeout(..., 0)` synthetic completion timer scheduled by an earlier `send()`. If a caller calls `xhr.open(...)` synchronously before that zero-delay macrotask fires, the pending timer can still emit synthetic `readystatechange`/`load`/`loadend` events against the reopened instance. Suggested follow-up: track per-XHR pending timer ID or generation token so `proto.open` cancels/invalidates any pending synthetic timer.

### Carried-Over Prior Unique Findings (`main..e27ff89`, unchanged in `5f3ec41`)

* **`U1-R5` (`P1` / `MAJOR`, `LOW` confidence, `advisory`, stashed as `7883544E`)**: `internal/browser/recap.go:1053` — Track committed-plus-in-flight bytes across concurrent Path A workers so combined buffered chunks cannot exceed `maxRecapAudioSourceBytes`.
* **`U2-R5` (`P1` / `MAJOR`, `LOW` confidence, `advisory`, stashed as `7883544E`)**: `internal/browser/recap.go:2060` — Remove `hasXplatTarget` gate on `lastErr == nil` retries so delayed non-xplat `streamembed.aspx` mounts are polled across `maxAttempts`.
* **`U1-R6` (`P1` / `MAJOR`, `LOW` confidence, `advisory`)**: `internal/browser/meeting_e2e_test.go:568` — Non-blocking advisory note on pre-`seg0` awaited `track=video` strip probe vs separate concurrent post-`seg0` unit coverage.

## Remediation Plan

Under full-pool multi-agent consensus (`active_reviewers = 3/3`), zero `P0` (`CRITICAL`) findings and zero `HIGH`-confidence or `MEDIUM`-confidence findings exist across `main..HEAD`. The single new `LOW`-confidence `P2` (`MINOR`) observation (`U1-R8`) and the 3 carried-over `LOW`-confidence `P1` observations remain non-blocking `advisory` items (`mode: report-only` applied no edits).

| Priority | ID | File and Line | Confidence | Severity | Action Class | Summary |
|---|---|---|---|---|---|---|
| 3 | U1-R6 | `internal/browser/meeting_e2e_test.go:568` | `LOW` | `P1` (`MAJOR`) | `advisory` | Carried over from R6: note timing trade-off of awaiting `track=video` strip probe before `seg0` |
| 3 | U1-R5 | `internal/browser/recap.go:1053` | `LOW` | `P1` (`MAJOR`) | `advisory` | Carried over from R5 (stashed `7883544E`): track committed-plus-in-flight bytes across concurrent Path A workers |
| 3 | U2-R5 | `internal/browser/recap.go:2060` | `LOW` | `P1` (`MAJOR`) | `advisory` | Carried over from R5 (stashed `7883544E`): remove `hasXplatTarget` gate on `lastErr == nil` retries |
| 2 | U1-R8 | `internal/browser/recap.go:1386` | `LOW` | `P2` (`MINOR`) | `advisory` | Cancel or invalidate pending synthetic XHR completion timer when `proto.open` reuses a suppressed XHR synchronously before the timer fires |

## Bug / Issue Queue Entries

None newly generated (`0` P0/P1 findings on `e27ff89..HEAD`; `U1-R8` is `P2` `MINOR` advisory; carried-over `U1-R5` and `U2-R5` are already tracked under stash `7883544E`, and `U1-R6` was recorded in `docs/closure/2026-11-16-031-s-r6-adversarial-review.md`).

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 4
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
