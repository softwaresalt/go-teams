---
type: memory-compacted
date: 2026-11-10
shipment: 017-S
feature: 017-F
source: docs/archive/memory/2026-11-10/017-S-reconcile-deliberation-and-headed-sandbox-memory.md
---

# Compacted memory - Shipment 017-S (deliberation disposition and headed sandbox opt-in)

**Shipped**: PR #42, merge `03ba97336d9f16b992d4675fae287ed95730b878`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship --sha`). Deliberation `006-DL` was retained.

## Decision and outcome

* `shipment-reconcile` now classifies each linked deliberation with the backlogit
  1.12 INV-12 disposition (`compute_linked_deliberation_disposition`). Only `archive`
  (`expected_cascade`) deliberations enter `allowed_ids` and `required_ids`;
  `already-archived` is tolerated; every retained disposition is excluded from both,
  guarded by a pre/post preservation check on the snapshotted location and status
  that halts with `unexpected-cascade` and runs `git-revert-on-cascade`. Cascade
  Close step 7 archives only a queued `retained_description_mention` deliberation
  with zero live references. Ship Post-Merge Closure step 7 enumerates source and
  description/references links and applies the same rules.
* The direct-exec browser launcher builds its arguments through
  `cleanBrowserArgsForOptions`; `--no-sandbox` and `--disable-setuid-sandbox` appear
  before the URL only on `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX=1|true`.

## Key reusable insights

* The autoharness gate treats an engine minor line outside
  `VERIFIED_CASCADE_ENGINE_MINOR_LINES` (only `1.11`) as UNVERIFIED, so under backlogit
  1.12.0 every non-archived linked deliberation resolves to `retained_engine_unverified`
  and is never archived by the closure. Compute the disposition instead of predicting it.
* A feature's `source_deliberation_id` is a link kind; `retained_description_mention`
  needs that link to be absent.
* A cleanup branch behind a strict halt can be unreachable. Copilot caught an
  `archive` fallback in step 7 that step 3 and step 6 made impossible.
* Preservation checks should compare the snapshotted location and status, not assume
  `queue/`; a `done` record can live in `archive/`.
* Moving tasks and the feature to `done` relocates them to `.backlogit/archive/`, and
  the cascade still reports them in `archived_ids`.

## Review outcome

Local review: `READY`, `P0=0, P1=0` (single-agent declared degradation). Copilot raised
four in-scope threads over two rounds, all fixed. The `copilot-review` gate returned
`SATISFIED`, CI passed, and the merge used a merge commit.

## Post-merge closure summary

`backlogit shipment ship` returned `archived_ids: [017.001-T, 017.002-T, 017-F,
017-S]` and `returned_ids: []`; both tasks kept `parent_id: 017-F`. `006-DL` stayed
queued; follow-up stash entry `9A9D8106` covers it.
