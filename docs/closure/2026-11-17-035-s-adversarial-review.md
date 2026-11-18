---
title: Adversarial review for shipment 035-S (HEAD c123099)
date: 2026-11-17
feature: 035-F
shipment: 035-S
head_sha: c123099888c88cabd65d0d23f059517ef4b4cda7
diff_range: main..c123099888c88cabd65d0d23f059517ef4b4cda7
mode: report-only
dispatch_mode: multi-agent
reviewers_requested: 3
active_reviewers: 3
consensus_status: CONSENSUS_ASSEMBLED (0 HIGH Consensus, 0 MEDIUM Majority, 0 MEDIUM Plurality, 1 LOW Unique)
readiness_verdict: READY_WITH_FOLLOWUPS
---

## Invocation Summary

* **Mode**: `report-only` (strictly read-only over repository source, test, and config files; `safe_auto` edits not applied; Phase 7 skipped).
* **Branch**: `feat/035-s-recap-audio-cold-sharepoint-transcode-resilience-and-appdata-profile-fallback`.
* **Diff reviewed**: `main..c123099888c88cabd65d0d23f059517ef4b4cda7` across all 4 changed files (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, `internal/browser/recap.go`, `internal/browser/recap_test.go`), including commit `c123099` (`fix(browser): break target loop on stream-deadline error so outer retry owns single retry`).
* **Authorized contract scope (P-021 C1)**: `docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-plan.md` (shipment `035-S` / feature `035-F` / tasks `035.001-T`..`035.003-T`).
* **Deferred scope expansion noted**: Path B-only MSE-state persistence / player reload across CDP retry attempts is already captured as P-021 C2 deferred scope expansion `B98262C8`.
* **Ruleset**: `.github/instructions/copilot-code-review.instructions.md`, `.github/instructions/constitution.instructions.md`, `.github/instructions/go.instructions.md`, `.github/instructions/architecture-doc.instructions.md`, `.github/policies/workflow-policies.md` (P-021 C1), and the plan's Protected Invariants 1..5 and R1..R4 (`.github/copilot-review-instructions.md` absent).
* **Verification baseline observed on HEAD `c123099`**: `go vet ./...` (exit 0), `gofmt -l internal/browser/recap.go internal/browser/recap_test.go` (clean, exit 0), and `go test -race ./internal/browser/...` (`59.824s`, exit 0) passed.
* **Commit `c123099` delta verified**: `isStreamDeadlineMsg` is shared across target evaluation errors (`evalErr`), payload extraction errors (`payload.Error`), and `isRetryableStreamDeadline`, breaking out of the inner `orderedTargets` loop on a stream-deadline error so same-origin parent targets are not evaluated and the outer `attempt == 0` retry owns the single retry; covered by `assert.Zero(t, parentEvals.Load())` in `internal/browser/recap_test.go`.
* **Dispatch and pool status**:
  * `dispatch_mode`: `multi-agent`
  * `active_reviewers`: `3` / `3`
  * `consensus_status`: `CONSENSUS_ASSEMBLED` (full pool settled cleanly; no pool degradation)
  * `findings_summary`: `0` `P0` (`CRITICAL`), `1` `P1` (`MAJOR`: `1` LOW Unique `advisory` in full-pool mode), `0` `P2` (`MINOR`), `0` `P3`
  * `readiness_verdict`: `READY_WITH_FOLLOWUPS` (zero `P0`, zero `HIGH` Consensus, and zero `MEDIUM` Majority/Plurality findings, so the full-pool gate is not `BLOCKED`; `1` `LOW` Unique `P1` `advisory` finding `U1` remains as an advisory follow-up with a backlog entry generated).

### Model Route Assignment Table

| Reviewer | Route | Model Used | Reasoning Effort | Settlement | Findings Returned |
| --- | --- | --- | --- | --- | --- |
| Anchor Reviewer | Anchor review route (`openai`) | `gpt-6.1-sol` | `high` | Settled (valid JSON) | 1 (`1 MAJOR`) |
| Reviewer-A | Tier 1 (`anthropic`) | `claude-haiku-5.5` | default | Settled (valid JSON) | 0 |
| Reviewer-C | Tier 3 (`google`) | `gemini-4-argon` | default | Settled (valid JSON) | 0 |

## Consensus Findings (HIGH Confidence -- 3/3 Reviewers)

None. Zero findings were flagged by all 3 reviewers.

## Majority Findings (MEDIUM Confidence -- 2/3 Reviewers)

None. Zero findings were flagged by 2 or more reviewers.

## Plurality Findings (MEDIUM Confidence -- >1 Reviewer, Not Strict Majority)

Not applicable with `active_reviewers = 3` (any count > 1 is already a strict majority).

## Unique Findings (LOW Confidence -- 1/3 Reviewers)

### U1 (`P1` / `MAJOR`): `isPathInsideRepoRoot` walks lexical ancestors without resolving symlinks or Windows junctions

* **Confidence**: `LOW` (1/3: Anchor Reviewer `MAJOR`).
* **Priority score**: `3` (`confidence_weight=1 x severity_weight=3`).
* **Action class**: `advisory` (`LOW` confidence + `MAJOR` in full-pool mode `active_reviewers == reviewers`).
* **P-021 C1 scope**: `IN_SCOPE` (same contract surface in U1 / `035.001-T` / R3 / Protected Invariant 4).
* **Rule**: Protected Invariant 4 -- Scoped Cross-Root Profile Discovery.
* **File / Line**: `internal/browser/recap.go:3030` (`isPathInsideRepoRoot` at `3025-3047`).
* **Issue**: `isPathInsideRepoRoot` walks lexical ancestors via `filepath.Clean(p)` and `filepath.Dir(cur)` without resolving symlinks or Windows junctions. If `APPDATA/go-teams/profile` links to a profile beneath a repository, that repository's ancestor markers (`go.mod` or `.git`) are missed, allowing cross-root fallback to select a repository-backed credential store despite the required exclusion.
* **Fix**: Check resolved filesystem ancestors (resolving the nearest existing ancestor via `filepath.EvalSymlinks` when the leaf profile does not yet exist) as well as lexical ancestors before permitting fallback, and cover linked source and candidate profiles with regression tests.

## Remediation Plan

Ordered descending by `priority = confidence_weight x severity_weight`, then deterministically by file path and line (`mode: report-only`; no edits applied):

| Priority | ID | File and Line | Confidence | Severity | Action Class | P-021 C1 Scope | Summary |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 3 | U1 | `internal/browser/recap.go:3030` | `LOW` | `P1` (`MAJOR`) | `advisory` | `IN_SCOPE` | Check resolved filesystem ancestors (nearest existing ancestor via `filepath.EvalSymlinks`) as well as lexical ancestors in `isPathInsideRepoRoot` |

## Bug / Issue Queue Entries

Ready for backlog creation via `backlogit add --type bug --title "..."` for `P1` finding `U1`:

```yaml
- type: bug
  title: "Protected Invariant 4: resolve symlinks/junctions in isPathInsideRepoRoot before ancestor walk"
  description: "isPathInsideRepoRoot walks lexical ancestors without resolving symlinks or Windows junctions, allowing a linked APPDATA/go-teams/profile pointing beneath a git repository to bypass ancestor marker checks and be selected as a fallback."
  file: "internal/browser/recap.go"
  line: 3030
  severity: "MAJOR"
  confidence: "LOW"
  fix: "Check resolved filesystem ancestors (resolving the nearest existing ancestor via filepath.EvalSymlinks when the leaf profile does not yet exist) as well as lexical ancestors before permitting fallback, and cover linked source and candidate profiles with regression tests."
  linked_review: "docs/closure/2026-11-17-035-s-adversarial-review.md"
```

## Post-Remediation Status

```yaml
post_remediation:
  cycles_run: 0
  cap_reached: false
  residual_findings: 1
  status: "skipped"
  reason: "mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"
```
