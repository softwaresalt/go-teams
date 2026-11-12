---
type: memory-compacted
date: 2026-11-11
shipment: 019-S
feature: 019-F
source: docs/archive/memory/2026-11-11/019-S-harness-review-and-deliberation-hygiene-memory.md
---

# Compacted memory - Shipment 019-S (harness review gate and retained deliberation hygiene)

**Shipped**: PR #47, merge `c4dece0851ca75146cec817385b4a9f2b637449d`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship --sha`). Deliberation `008-DL` was retained.

## Decision and outcome

* `_ship.agent.md` Step 4.4 dropped the stale `(approved for 012-S-017-S)` range
  ceiling. P-012 Item 5 single-agent fallback is a permanent contract governed by
  the declared `TOOL_DEGRADED: subagent-dispatch` state. Reviewer severities map
  `CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`.
* `adversarial-review.agent.md` gained `mode` and `authorized_contract_scope`
  inputs, a Dispatch Modes section, a bounded per-reviewer settlement wait,
  `active_reviewers` thresholds, `INSUFFICIENT_REVIEWERS`, `REVIEWER_POOL_DEGRADED`
  blocking on all unresolved P0 and P1 findings, severity-only scoring and routing
  for `DEGRADED_SINGLE_AGENT`, and strict read-only behavior under
  `mode: report-only`. Child reviewers always receive `mode: report-only`.
* `shipment-reconcile` documents that Stage or the operator MAY archive a
  `retained_engine_unverified` deliberation after closure when the covering
  shipment and feature are archived, it is `queued`, and it has zero live
  governing or dependency references. The compound learning at
  `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`
  holds the detail.

## Key reusable insights

* `output_mode: consensus-only` is only a display filter for `LOW`-confidence
  `MINOR` observations. It must never hide a finding the gate evaluates.
* A re-review phase needs the same gate-blocking rule as the original review
  phases, or a re-review silently drops `MEDIUM` and `LOW` blocking findings.
* Every severity and confidence cell in an action-routing table needs an explicit
  row. A missing `LOW + MAJOR` row left the degraded-pool route undefined.
* Child reviewer skills default to interactive, fix-applying mode. Pass
  `mode: report-only` explicitly at every dispatch point, including re-dispatch.
* The Orchestrator `rubber-duck` review caught four documentation gaps before the
  push. A review finding may cite an input name that does not exist; map it to the
  real input and record the discrepancy.
* Compute each deliberation disposition at closure time. Here `008-DL` was
  `retained_engine_unverified` (its `source_deliberation_id` link on `019-F`), so
  the automated cascade left it queued.

## Review outcome

Local review: `READY`, `P0=0, P1=0` (single-agent declared degradation under the
P-012 Item 5 bootstrap authorization, `DEGRADED_SINGLE_AGENT (non-consensus)`).
Copilot raised one in-scope thread (child reviewers need `mode: report-only`),
fixed in `b09cb54` (P-021 C1), replied to, and resolved. The `copilot-review`
gate returned `SATISFIED`. No CI checks are configured for the branch. The merge
used a merge commit.

## Post-merge closure summary

`backlogit shipment ship` returned `archived_ids: [019.001-T, 019.002-T, 019-F,
019-S]` and `returned_ids: []`; both tasks kept `parent_id: 019-F`. `008-DL` stayed
in `.backlogit/queue/` as `queued`, unmoved. Stash entries `680A62E6` and
`9A9D8106` were already archived by Stage. Follow-up: Stage or the operator may
archive `008-DL` under the post-closure rule once it has zero live governing
references.
