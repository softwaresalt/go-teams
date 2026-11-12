---
title: Harness review-gate single-agent fallback and retained deliberation hygiene review and operational closure
date: 2026-11-11
feature: 019-F
shipment: 019-S
branch: feat/019-s-reconcile-harness-review-gate-single-agent-fallback-and-post-closure-retained-deliberation-hygiene
status: ready
closure_status: READY
compaction_status: pending
---

## Release scope

Shipment `019-S` (`019-F`, `019.001-T`, `019.002-T`) reconciles the harness review
gate with the single-agent degraded fallback and documents the post-closure
archival path for `retained_engine_unverified` deliberations. It is planned under
deliberation `008-DL` and consumes stash entries `680A62E6` and `9A9D8106`.

* `019.001-T` (`.github/agents/_ship.agent.md`,
  `.github/agents/subagents/adversarial-review.agent.md`):
  * `_ship.agent.md` Step 4.4 drops the stale `(approved for 012-S-017-S)` range
    ceiling. P-012 Item 5 is now a permanent contract governed by the declared
    `TOOL_DEGRADED: subagent-dispatch` state (`008-DL` / `019-F`). Reviewer
    severities map `CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`.
  * Multi-agent gate rules: `INSUFFICIENT_REVIEWERS` (`BLOCKED`) when
    `active_reviewers < 2`; full pool blocks on every `HIGH` consensus P0/P1 and
    every P0 regardless of confidence, with `MEDIUM` P1 findings needing explicit
    disposition; `REVIEWER_POOL_DEGRADED` (`2 <= active_reviewers < reviewers`)
    blocks on all unresolved P0 and P1 findings regardless of confidence.
  * Single-agent gate rules: `dispatch_mode: single-agent-declared-degradation`
    is a bounded non-consensus outcome
    (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) where `READY`
    requires `P0=0, P1=0`.
  * `adversarial-review.agent.md` gains `mode` and `authorized_contract_scope`
    inputs, a Dispatch Modes section, a bounded per-reviewer settlement wait,
    `active_reviewers` thresholds, severity-only scoring and routing for the
    degraded mode, the `### Degraded Single-Agent Findings (Non-Consensus)` output,
    strict `mode: report-only` read-only enforcement (fail closed when
    `authorized_contract_scope` is absent or ambiguous), a branched Phase 7, and
    qualified Quality Criteria.
* `019.002-T` (`.github/skills/shipment-reconcile/SKILL.md`,
  `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`):
  documents that Stage or the operator MAY archive a `retained_engine_unverified`
  deliberation with `backlogit archive <dl_id>` after closure, when the covering
  shipment and feature are archived, the deliberation is `queued`, and it has zero
  live governing/dependency references. Historical audit citations (`008-DL`,
  `019-F`, `019-S`, `019.002-T` for `006-DL` / `9A9D8106`) do not count. Records
  that `VERIFIED_CASCADE_ENGINE_MINOR_LINES` is outside the workspace under
  Constitution Principle IV.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`,
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`): the multi-persona
rubric (Constitution, Go, Correctness, Maintainability, Security, Concurrency,
Scope Boundary, Learnings, Template Integrity, Schema-CLI-Docs Coupling) ran
in-session in report-only posture against the shipment diff
(`main..HEAD`). This shipment is the bootstrap case authorized across `007-DL`,
`018-F`, `018-S`, `008-DL`, `019-F`, and `019-S`. No repository content went to an
external reviewer model.

| Area | Finding | Disposition |
|---|---|---|
| Template Integrity | Both agent files and the skill keep valid frontmatter; no new headings break the heading hierarchy; the new `## Dispatch Modes` section sits between `## Inputs` and `## Output`; the compound learning follows the existing `compound-learning` frontmatter shape | No action |
| Schema-CLI-Docs Coupling | `_ship.agent.md` Step 4.4 and `adversarial-review.agent.md` use the same terms (`dispatch_mode`, `consensus_status`, `confidence_tier`, `INSUFFICIENT_REVIEWERS`, `REVIEWER_POOL_DEGRADED`, `active_reviewers`) and the same severity mapping | No action |
| Correctness | The adversarial-review introduction still described re-review as unconditional multi-agent redispatch, conflicting with the new report-only rule | Fixed in-scope (P-021 C1: completes the authorized reconciliation of the same file); commit `ba88485` |
| Correctness | Ship invokes the agent with `mode: report-only`, so Phase 6 and Phase 7 stay read-only and Ship owns scope classification and fixes | No action |
| Scope | Changes stay inside the four authorized files plus the new compound learning and `019-S` backlog, closure, and memory docs; no Go source changed; stash `stash@{0}` untouched | No action |
| Security | No secrets, credentials, or external transmission introduced; the degraded mode explicitly forbids external model transmission | No action |
| Constitution | Principle IV preserved: no edit to the external `autoharness` package; Principle I-II unaffected (documentation only) | No action |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

## Follow-ups

None captured. No P-021 deferred scope expansion entry was needed.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis | `go vet ./...` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` ("No issues found.") | PASS |
| Whitespace | `git diff --check` (no whitespace errors) | PASS |
| Full build | Not applicable: documentation, agent specification, and backlog changes only, no Go source changed | N/A |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Tighten the review gate to fail closed on partial reviewer pools and on missing `authorized_contract_scope` | low | Specification text only; stricter than before; Ship retains scope classification | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Review-gate specification | READY | Applies at the next review-gate invocation after merge |
| Post-closure archival rule | READY | Stage or the operator applies it manually; no automation added |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the next shipment review gate and the next shipment closure.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Review records show `dispatch_mode` and `consensus_status` | Closure artifacts | Recorded on every review | A degraded review without `DEGRADED_SINGLE_AGENT` |
| Retained deliberations get a follow-up | Stash after closure | Follow-up captured | An orphaned `retained_engine_unverified` deliberation with no follow-up |

## Rollback

Revert the `019-S` merge commit on `main`. All changes are prose specifications and
documentation, with no data migration.

## Handoff

Related [decision](../decisions/2026-11-11-harness-review-fallback-and-deliberation-hygiene-deliberation.md)
and [plan](../plans/2026-11-11-harness-review-fallback-and-deliberation-hygiene-plan.md).
Local commits are unpushed pending Orchestrator `rubber-duck` review.
