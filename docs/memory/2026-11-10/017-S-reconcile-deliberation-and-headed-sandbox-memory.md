---
type: memory
date: 2026-11-10
shipment: 017-S
feature: 017-F
branch: feat/017-s-p-021-deferred-scope-expansion-reconciliation-shipment-reconcile-deliberation-disposition-and-headed-browser-sandbox-opt-in
status: feature-pr-pending
---

# Session memory - Shipment 017-S

## Items completed

* `017.001-T` committed as `095f7d8` and moved to done: `shipment-reconcile`
  Step 0(c), Cascade Close steps 3, 5, 6, and a new step 7, Quality Criteria, and
  `_ship.agent.md` Post-Merge Closure step 7 reconciled with backlogit 1.12 INV-12
  dispositions.
* `017.002-T` committed as `2e2a2f5` and moved to done: `cleanBrowserArgsForOptions`
  in `internal/browser/recap.go` honors `Options.DisableSandbox` and
  `TEAMS_DISABLE_SANDBOX`; tests in `internal/browser/recap_test.go`.
* `017-F` moved to done.

## Blocked

none

## Branch state

Feature branch `feat/017-s-p-021-...-sandbox-opt-in` cut from `main` at
`a602fb7`. Shipment `017-S` claimed and verified active (pipeline-topology
`post_claim` passed). Hook events through seq 267 acknowledged; none required
action.

## Decisions

* Disposition source of truth is
  `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`;
  the skill refers to it rather than re-deriving rules, and fails closed to
  `retained_engine_unverified` when it cannot evaluate.
* Retained deliberations are excluded from `allowed_ids` as well as `required_ids`
  so an engine that archives one trips both the unexpected-artifact check and the
  preservation check.
* The post-cascade cleanup became a new Cascade Close step 7 because step 4 is the
  `parent_id` check; the plan's "Step 4 cleanup" wording maps to it.
* `cleanBrowserArgs` and `cleanHeadedBrowserArgs` keep their signatures and read
  the environment opt-in through `Options{}`.
* Telemetry is disabled in this workspace (`autoharness telemetry begin` returned
  `disabled`), so no close payload was recorded.

## Errors and resolutions

* gofmt flagged struct field alignment in the new test table; fixed with
  `gofmt -w`.
* Stale Step 0(c) sentence claiming a live linked deliberation is always archived
  by the cascade was reworded during review.

## Next steps

Write closure doc, run backlog sync and doctor, lifecycle and pre-review gates,
push, open the feature PR with the Local Review Readiness block, satisfy CI and
the Copilot gate, merge with a merge commit, then run post-merge closure on
`post-merge/017-reconcile-deliberation-and-headed-sandbox` (P-015 cascade ship,
`backlogit archive 006-DL`, P-020 compaction).
