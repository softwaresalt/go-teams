---
title: Harness review-gate fallback reconciliation and retained deliberation hygiene decision
description: Deliberation on reconciling the single-agent declared-degradation review fallback against the adversarial-review multi-reviewer consensus halt and establishing post-closure archival hygiene for retained_engine_unverified deliberations
topic: Reconcile harness review fallback (680A62E6) and retained_engine_unverified deliberation archival hygiene (9A9D8106)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 008-DL
feature: 019-F
shipment: 019-S
source_stash:
  - 680A62E6
  - 9A9D8106
source_refs:
  stash_id: 680A62E6
  shipment_id: 015-S
  feature_id: 015-F
  task_id: 015.001-T
  pr_number: 38
  review_thread_id: PRRT_kwDOTStvas62RRVH
  followup_stash_id: 9A9D8106
  followup_closure_doc: docs/closure/2026-11-10-017-S-review-and-closure.md
  orphaned_deliberation_id: 006-DL
linked_artifacts:
  - docs/plans/2026-11-11-harness-review-fallback-and-deliberation-hygiene-plan.md
  - .backlogit/queue/008-DL.md
  - .backlogit/queue/019-F.md
  - .backlogit/queue/019-S.md
date: 2026-11-11
status: accepted
tags:
  - harness
  - adversarial-review
  - p-012
  - p-021
  - shipment-reconcile
  - backlogit
---

## Problem Frame

This deliberation covers the remaining two active stash entries in Option 3
(Track C: Harness Review-Gate and Backlog Closure Hygiene):

1. **Stash entry `680A62E6`** (`kind: task`, `priority: medium`, P-021 C2
   `DEFERRED SCOPE EXPANSION` from shipment `015-S`, feature `015-F`, task
   `015.001-T`, PR `#38`, Copilot review thread `PRRT_kwDOTStvas62RRVH`):
   * In `.github/agents/_ship.agent.md` (Step 4.4, line 465) and
     `.github/agents/subagents/adversarial-review.agent.md` (Phase 1 step 3,
     lines 94–99, and Quality Criteria, line 240), the P-012 item 5 privacy /
     degraded subagent fallback (`TOOL_DEGRADED: subagent-dispatch — declared
     fallback: single-agent-declared-degradation`) was added alongside the
     multi-reviewer consensus protocol without explicitly separating their
     execution paths and invariants:
     * `adversarial-review.agent.md` Phase 3 classifies findings flagged by all
       reviewers as `HIGH / Consensus`, and Quality Criteria states *"If fewer
       than 2 reviewer instances return results, halt and report the failure."*
     * Meanwhile, `_ship.agent.md` Step 4.4 carries a shipment-range annotation
       (`approved for 012-S–017-S`) and permits `single-agent-declared-degradation`
       when `subagent-dispatch` is unavailable, without stating whether a
       single-agent pass counts as multi-reviewer consensus or whether a failed
       multi-agent dispatch (< 2 reviewers returning results when subagent
       dispatch is active) still halts with `INSUFFICIENT_REVIEWERS`.
   * We must reconcile both files so that:
     * When multi-reviewer subagent dispatch is active (`dispatch_mode: multi-agent`),
       fewer than two independent reviewer instances returning results
       unconditionally triggers an `INSUFFICIENT_REVIEWERS` halt (`BLOCKED`) and
       never silently falls back to `single-agent-declared-degradation` mid-run.
     * When subagent dispatch is unavailable at the P-012 pre-dispatch capability
       gate or restricted by runtime privacy / no-third-party-transmission policy
       (`TOOL_DEGRADED: subagent-dispatch`), `dispatch_mode: single-agent-declared-degradation`
       is an explicitly authorized **non-consensus** degraded review outcome
       (`consensus_status: degraded-single-agent-non-consensus`) that skips
       multi-reviewer consensus claims (`HIGH / Consensus`, `Majority`,
       `Plurality`) and gates readiness directly on the in-session multi-persona
       `P0`–`P3` severity rubric (`P0=0, P1=0` required to proceed).
2. **Stash entry `9A9D8106`** (`kind: task`, `priority: low`, follow-up from
   `017-S` closure in
   [2026-11-10-017-S-review-and-closure.md](../closure/2026-11-10-017-S-review-and-closure.md)):
   * During `017-S` post-merge closure, `006-DL` was classified as
     `retained_engine_unverified` because `backlogit 1.12.0` is not listed in the
     external `autoharness` Python package's
     `VERIFIED_CASCADE_ENGINE_MINOR_LINES` (which lists only `"1.11"`).
   * Per the reconciled `shipment-reconcile` Cascade Close Sub-Procedure step 7
     and `_ship.agent.md` Post-Merge Closure step 7, Ship never automatically
     archives a `retained_engine_unverified` deliberation during closure (failing
     closed to retain).
   * After `017-S` and `017-F` were archived (`017-S` `archived_status: shipped`,
     `017-F` `archived_status: done`), `006-DL` remained queued in
     `.backlogit/queue/006-DL.md` with zero live references.
   * We must decide:
     * Whether to archive `006-DL` (`backlogit archive 006-DL`) now that `017-S`
       is shipped and zero live references remain in `.backlogit/queue/`.
     * How to handle the `VERIFIED_CASCADE_ENGINE_MINOR_LINES` question while
       strictly honoring **Constitution Principle IV (CLI Workspace Containment,
       NON-NEGOTIABLE)**, which forbids modifying any file outside
       `C:\Source\GitHub\go-teams` (such as the external Python installation at
       `C:\Python\Python315\...`).

## P-021 C5 and C6 Triage and Reconciliation Record

Per Stage Step 1 (`Deferred-Expansion Triage Obligations`, P-021 C5/C6), both
separately triggered obligations were executed for `680A62E6` before
deliberation:

* **Precedence classification (P-021 C6)**: Stash entry `680A62E6` carries the
  literal `DEFERRED SCOPE EXPANSION` prefix, forcing the Step 2 `deliberate`
  route prior to planning or harvest. Stage retains priority `medium` for the
  covering feature `019-F` and shipment `019-S`.
* **Obligation (A) Unconditional duplicate detection**: Scanned all active stash
  entries (`.backlogit/stash.jsonl`), archived stash entries
  (`.backlogit/archive/stash.jsonl`), active queue items (`.backlogit/queue/`),
  and archived backlog items (`.backlogit/archive/`). Only `680A62E6` describes
  reconciling the `_ship.agent.md` and `adversarial-review.agent.md`
  `single-agent-declared-degradation` fallback against the multi-reviewer
  `INSUFFICIENT_REVIEWERS` halt. Outcome: **CLEAN DUPLICATE SCAN**.
* **Obligation (B) Mandatory late-identifier reconciliation**: Verified all five
  source-reference fields recorded on `680A62E6` against
  [2026-11-10-015-S-review-and-closure.md](../closure/2026-11-10-015-S-review-and-closure.md):
  * `shipment`: `015-S` (concrete at capture)
  * `feature`: `015-F` (concrete at capture)
  * `task`: `015.001-T` (concrete at capture)
  * `PR`: `38` (concrete at capture)
  * `review-thread`: `PRRT_kwDOTStvas62RRVH` (concrete at capture)
  No field was recorded as `N/A`. Outcome: **idempotent no-op (all five source
  identifiers concrete at capture)**.

## Research Findings

* **Review-gate fallback surface (`.github/agents/_ship.agent.md` and
  `.github/agents/subagents/adversarial-review.agent.md`)**:
  * In `.github/agents/_ship.agent.md` line 465, the P-012 item 5 fallback is
    currently labeled `approved for 012-S–017-S` and describes running the
    multi-persona rubric in-session with `dispatch_mode: single-agent-declared-degradation`,
    proceeding when `P0=0, P1=0`. It does not explicitly state that a single-agent
    pass is a **non-consensus** degraded outcome (`consensus_status: degraded-single-agent-non-consensus`)
    or that when `dispatch_mode: multi-agent` is active, fewer than two
    independent reviewer instances returning results halts with
    `INSUFFICIENT_REVIEWERS` (`BLOCKED`) rather than degrading mid-run.
  * In `.github/agents/subagents/adversarial-review.agent.md`, Phase 1 step 3
    introduces `TOOL_DEGRADED: subagent-dispatch — declared fallback: single-agent-declared-degradation`,
    but Phases 2–3 and the Quality Criteria bullet (`If fewer than 2 reviewer
    instances return results, halt and report the failure`) do not distinguish
    between:
    1. **`dispatch_mode: multi-agent`** (where `< 2` returning independent
       reviewer subagents is a hard `INSUFFICIENT_REVIEWERS` failure that halts
       Phase 3 consensus aggregation), and
    2. **`dispatch_mode: single-agent-declared-degradation`** (entered at Phase 1
       step 3 only when subagent dispatch is unavailable or blocked by runtime
       privacy / no-third-party-transmission policy under P-012 item 5, where
       no multi-reviewer consensus is claimed, `consensus_status: degraded-single-agent-non-consensus`
       is recorded, and findings are classified by `P0`–`P3` severity only).
* **Retained deliberation lifecycle surface (`006-DL`,
  `.github/skills/shipment-reconcile/SKILL.md`, and Constitution Principle IV)**:
  * Verified `.backlogit/queue/` and `.backlogit/archive/`:
    * `017-F` (`.backlogit/archive/017-F.md`, `status: archived`,
      `archived_status: done`) and `017-S` (`.backlogit/archive/017-S.md`,
      `status: archived`, `archived_status: shipped`) are both in
      `.backlogit/archive/`.
    * No file in `.backlogit/queue/` other than `.backlogit/queue/006-DL.md`
      itself references `006-DL`.
    * In `.backlogit/stash.jsonl`, only `9A9D8106` (the follow-up stash entry
      being consumed in this session) mentions `006-DL`.
  * Why `006-DL` was retained at `017-S` closure: `autoharness.gates.shipment_closure`
    lives in the external Python package (`C:\Python\Python315\...`) outside the
    repository root `C:\Source\GitHub\go-teams`. Under **Constitution Principle IV
    (CLI Workspace Containment, NON-NEGOTIABLE)**, agents operating in this
    workspace MUST NOT modify any file outside `C:\Source\GitHub\go-teams`.
  * Therefore, `VERIFIED_CASCADE_ENGINE_MINOR_LINES` in the external Python
    package is an upstream `autoharness` concern outside the `go-teams`
    workspace boundary. Within `go-teams`, the durable fix is:
    1. Archive the orphaned `006-DL` artifact during this Stage session via
       `backlogit archive 006-DL` under Stage's backlog hygiene authority.
    2. Document the post-closure Stage/operator archival rule in
       `.github/skills/shipment-reconcile/SKILL.md` (Cascade Close Sub-Procedure
       step 7 and Quality Criteria) and in a compound learning note
       `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`:
       during shipment closure, Ship never auto-archives
       `retained_engine_unverified`; once the covering shipment and feature are
       archived in `.backlogit/archive/` and the deliberation has `status: queued`
       with zero remaining live references in `.backlogit/queue/` (and no active
       stash referrer), Stage or the operator archives `<id>-DL` via
       `backlogit archive <id>-DL`.

## Options Evaluated

### Issue 1 (`680A62E6`): Reconciling `single-agent-declared-degradation` and the `adversarial-review` `INSUFFICIENT_REVIEWERS` halt

#### Option 1A: Remove `single-agent-declared-degradation` and unconditionally halt when `< 2` subagent reviewers run

* **Pros**: Simplifies `adversarial-review.agent.md` to a single multi-agent
  execution path.
* **Cons**: Breaks P-012 item 5 degraded-mode support in CLI environments where
  external cross-model subagent dispatch is unavailable or prohibited by
  no-third-party-transmission policy, blocking every local review gate in Dark
  Factory Mode.
* **Effort**: low | **Fit**: low.

#### Option 1B (Chosen): Explicitly partition `dispatch_mode: multi-agent` (preserving the `< 2` reviewers `INSUFFICIENT_REVIEWERS` halt) from `dispatch_mode: single-agent-declared-degradation` (authorized non-consensus P-012 fallback)

Update `.github/agents/_ship.agent.md` (Step 4.4) and
`.github/agents/subagents/adversarial-review.agent.md` (Inputs, Output, Phase 1
step 3, Phase 2, Phase 3, and Quality Criteria) so the two modes are mutually
exclusive and unambiguous:

1. **Multi-agent consensus mode (`dispatch_mode: multi-agent`)**: Entered when
   subagent reviewer dispatch is available at Phase 1 capability check. Requires
   at least 2 independent reviewer instances returning structured results. If
   fewer than 2 reviewer instances return results, `adversarial-review` and Ship
   MUST halt immediately with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) — never
   downgrading a failed or partial multi-reviewer dispatch into
   `single-agent-declared-degradation` mid-run, and never counting 1 reviewer as
   consensus.
2. **Declared single-agent degraded mode (`dispatch_mode: single-agent-declared-degradation`)**:
   Entered ONLY at Phase 1 step 3 when subagent reviewer dispatch is unavailable
   at the P-012 capability probe or restricted by runtime privacy /
   no-third-party-transmission policy (`TOOL_DEGRADED: subagent-dispatch —
   declared fallback: single-agent-declared-degradation`). It is an explicitly
   authorized **non-consensus** degraded outcome:
   * Records `dispatch_mode: single-agent-declared-degradation` and
     `consensus_status: degraded-single-agent-non-consensus`.
   * Does NOT claim multi-reviewer consensus (`HIGH / Consensus`, `Majority`, or
     `Plurality` labels are not emitted for a single-agent pass; findings are
     classified by `P0`–`P3` / `CRITICAL`–`MINOR` severity across the in-session
     multi-persona rubric).
   * Gates Ship Step 4.4 readiness on `P0=0, P1=0` (any `P0`/`P1` blocks with
     `BLOCKED`; `P2`/`P3` require follow-up or acknowledgment).
   * Removes the stale `(approved for 012-S–017-S)` range cap in
     `_ship.agent.md` so the P-012 item 5 fallback contract applies cleanly to
     all shipments.

* **Pros**: Directly resolves PR #38 thread `PRRT_kwDOTStvas62RRVH`; preserves
  the `INSUFFICIENT_REVIEWERS` halt when multi-agent dispatch is active; prevents
  single-agent degraded reviews from masquerading as multi-model consensus;
  maintains P-012 item 5 compatibility.
* **Cons**: Requires coordinated wording updates in two agent specification
  files.
* **Effort**: low | **Fit**: high.

### Issue 2 (`9A9D8106`): Orphaned `006-DL` and `retained_engine_unverified` post-closure disposition

#### Option 2A: Modify the external `autoharness` Python package to add `"1.12"` to `VERIFIED_CASCADE_ENGINE_MINOR_LINES`

* **Pros**: Would classify future 1.12 deliberations as `archive` during Step 0(c).
* **Cons**: **Violates Constitution Principle IV (CLI Workspace Containment,
  NON-NEGOTIABLE)** because the installed `autoharness` Python package resides
  outside `C:\Source\GitHub\go-teams` (`C:\Python\Python315\...`). Also does not
  clean up `006-DL`, which is already retained in `.backlogit/queue/`.
* **Effort**: low | **Fit**: disqualified (Principle IV violation).

#### Option 2B (Chosen): Archive `006-DL` in Stage via `backlogit archive 006-DL` and document the workspace-local post-closure Stage/operator archival rule for zero-reference `retained_engine_unverified` deliberations

1. **Immediate Stage hygiene action**: Archive `.backlogit/queue/006-DL.md` via
   `backlogit archive 006-DL` in Step 5.6 of this Stage session, after verifying
   that `017-F` and `017-S` are archived and `006-DL` has zero live references in
   `.backlogit/queue/`.
2. **Workspace-local skill and compound documentation (`019.002-T`)**:
   * In `.github/skills/shipment-reconcile/SKILL.md` (Cascade Close Sub-Procedure
     step 7 and Quality Criteria), clarify that while Ship never archives
     `retained_engine_unverified` during closure (failing closed to retain and
     stashing a follow-up when zero live references remain), once the covering
     shipment and feature are archived in `.backlogit/archive/` and the
     deliberation has `status: queued` with zero remaining live references in
     `.backlogit/queue/` and `.backlogit/stash.jsonl`, Stage (at triage/Step 5.6)
     or the operator archives the orphaned deliberation via
     `backlogit archive <dl_id>`.
   * In `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`,
     record the institutional learning from `017-S` / `006-DL`, including why
     `VERIFIED_CASCADE_ENGINE_MINOR_LINES` in the external `autoharness` Python
     installation is outside the `go-teams` workspace boundary under Constitution
     Principle IV and how Stage/operators cleanly retire zero-reference
     `retained_engine_unverified` deliberations.

* **Pros**: Strictly complies with Constitution Principle IV; immediately retires
  orphaned `006-DL` from `.backlogit/queue/`; makes the Ship -> Stage follow-up
  lifecycle for `retained_engine_unverified` deliberations explicit and durable.
* **Cons**: Until upstream `autoharness` adds `"1.12"` in a future release,
  Ship continues to retain `source_deliberation_id`-linked deliberations at step
  7 and hand off their archival to Stage/operator follow-up.
* **Effort**: low | **Fit**: high.

## Trade-off Comparison

| Criterion | Option 1A + 2A | Option 1B + 2B (Chosen) |
|---|---|---|
| Constitution Principle IV compliance | Violates Principle IV by mutating external Python site-packages | 100% workspace-contained inside `C:\Source\GitHub\go-teams` |
| Multi-reviewer consensus integrity (`PRRT_kwDOTStvas62RRVH`) | Halts all degraded CLI reviews | Preserves `INSUFFICIENT_REVIEWERS` halt for multi-agent mode and explicitly labels single-agent fallback as non-consensus (`degraded-single-agent-non-consensus`) |
| Backlog queue hygiene (`006-DL`) | Leaves `006-DL` queued in `.backlogit/queue/` | Archives `006-DL` via `backlogit archive 006-DL` and documents the post-closure Stage/operator archival rule |

## Decision

We adopt **Option 1B + Option 2B**:

1. Archive orphaned deliberation **`006-DL`** during Step 5.6 of this Stage
   session via `backlogit archive 006-DL` (verified zero live references in
   `.backlogit/queue/`, covering feature `017-F` and shipment `017-S` already
   archived).
2. Synthesize covering feature **`019-F`** (*Reconcile harness review-gate
   single-agent fallback and post-closure retained deliberation hygiene*) and
   shipment **`019-S`** (chained after `018-S` via
   `backlogit dep add 019-S 018-S --type blocks`) with two width-isolated tasks:
   * **`019.001-T` (U1, `config` domain — harness agent definitions, 2 files)**:
     Reconcile `.github/agents/_ship.agent.md` and
     `.github/agents/subagents/adversarial-review.agent.md` so
     `single-agent-declared-degradation` is an explicitly authorized
     non-consensus degraded outcome (`consensus_status: degraded-single-agent-non-consensus`)
     entered only when `subagent-dispatch` is unavailable at the P-012
     capability check, while active `multi-agent` dispatch preserves the
     `< 2` independent reviewers `INSUFFICIENT_REVIEWERS` halt (`S` / `low`).
   * **`019.002-T` (U2, `docs` domain — skill & compound documentation, 2 files)**:
     Document the post-closure Stage/operator archival rule for zero-reference
     `retained_engine_unverified` deliberations and the Principle IV external
     `autoharness` package boundary in
     `.github/skills/shipment-reconcile/SKILL.md` and
     `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`
     (`XS` / `low`).

## Rejected Alternatives

* **Option 1A (Removing `single-agent-declared-degradation`)**: Rejected because
  P-012 item 5 requires a declared in-session fallback when runtime privacy or
  platform constraints prevent external reviewer subagent dispatch.
* **Option 1C (Treating 1 in-session reviewer as `HIGH / Consensus`)**: Rejected
  because a single reviewer provides zero cross-model agreement signal; labeling
  it `HIGH / Consensus` misrepresents a degraded single-agent pass as
  multi-reviewer consensus.
* **Option 2A (Editing external Python `autoharness` site-packages)**: Rejected
  as a non-negotiable violation of Constitution Principle IV (CLI Workspace
  Containment).

## Unresolved Questions

None blocking planning, harvest, or shipment assembly.

## Risks and Mitigations

* **Risk of conflating `multi-agent` partial failure with `single-agent-declared-degradation`**:
  Mitigated in `019.001-T` by requiring the mode selection at Phase 1 pre-dispatch
  capability check (`TOOL_OK: subagent-dispatch` -> `dispatch_mode: multi-agent`
  vs. `TOOL_DEGRADED: subagent-dispatch` -> `dispatch_mode: single-agent-declared-degradation`)
  and forbidding any transition from `multi-agent` to
  `single-agent-declared-degradation` after Phase 2 dispatch begins.
* **Risk of archiving a `retained_engine_unverified` deliberation that still has
  live references**: Mitigated in `019.002-T` by requiring the same zero-live-reference
  verification across `.backlogit/queue/` and `.backlogit/stash.jsonl` (plus
  confirming the covering shipment and feature are archived in
  `.backlogit/archive/`) before Stage or the operator runs
  `backlogit archive <dl_id>`.
