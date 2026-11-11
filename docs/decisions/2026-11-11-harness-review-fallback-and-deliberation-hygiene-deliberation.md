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
       define `active_reviewers` as the count of independent reviewer instances
       that successfully returned valid JSON findings. If `active_reviewers < 2`,
       halt immediately with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) and never
       silently fall back to `single-agent-declared-degradation` mid-run. In
       normal full-pool `multi-agent` mode (`active_reviewers == reviewers`),
       every `HIGH` consensus P0/P1 finding AND every `P0` (`CRITICAL`) finding
       regardless of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`)
       blocks the gate (`BLOCKED`), while `MEDIUM` (both `Majority` and
       `Plurality`) P1 findings require explicit disposition/acknowledgment. If
       `2 <= active_reviewers < reviewers` (partial pool degradation, Finding 5
       & Comment `4543766839`), log
       `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`,
       compute Phase 3 consensus thresholds against `active_reviewers` (so
       unanimous agreement across all `active_reviewers` is classified
       `HIGH / Consensus section`), and in `_ship.agent.md` Step 4.4 block the
       review gate (`BLOCKED`) on **ALL unresolved P0 and P1 (`CRITICAL` and
       `MAJOR`) findings regardless of confidence (`HIGH`, `MEDIUM`
       Majority/Plurality, or `LOW`)** until resolved or dispositioned under
       P-021.
     * When subagent dispatch is unavailable at the P-012 pre-dispatch capability
       gate or restricted by runtime privacy / no-third-party-transmission policy
       (`TOOL_DEGRADED: subagent-dispatch`), `dispatch_mode: single-agent-declared-degradation`
       is an explicitly authorized **non-consensus** degraded review outcome
       (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) that branches
       Phases 2–7, Output, and Quality Criteria to use severity-only scoring
       (`confidence_tier: DEGRADED_SINGLE_AGENT`, `confidence_weight = 1.0`,
       ordered by `severity_score` alone), severity-only action routing
       (`P0/P1 -> safe_auto` when mechanical/in-scope or `gated_auto`/`manual`
       when architectural; `P2 -> gated_auto`; `P3 -> advisory`), a single
       `### Degraded Single-Agent Findings (Non-Consensus)` report section
       instead of mandatory `Consensus / Majority / Plurality / Unique`
       multi-reviewer sections, and an in-session, severity-gated Phase 7
       post-remediation re-review over only the files modified by `safe_auto`
       fixes (without attempting subagent pool redispatch, retaining the hard
       2-cycle recursion cap, evaluating residual `P0`/`P1` severity findings
       under `confidence_tier: DEGRADED_SINGLE_AGENT` instead of `HIGH`-confidence
       consensus findings, and recording any remaining findings after 2 cycles as
       `post_remediation_residual: true` under `DEGRADED_SINGLE_AGENT`), gating
       Ship readiness on `P0=0, P1=0`.
     * In both `multi-agent` and `single-agent-declared-degradation` modes,
       `adversarial-review.agent.md` Phase 6, Phase 7, and Quality Criteria
       explicitly honor `mode: report-only` (Finding 6): when
       `mode: report-only` is set, the agent is **strictly read-only** over
       repository source/config files — Phase 6 writes only the review report
       and remediation plan (without applying `safe_auto` edits directly), and
       Phase 7 records `post_remediation.status: "skipped"` with reason
       `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`.
       Automatic `safe_auto` fix application in Phase 6 and the Phase 7
       post-remediation re-review loop run ONLY when `mode` is NOT
       `report-only` AND `post_remediation_review: true` AND each `safe_auto`
       candidate passes the P-021 C1 same-contract-surface check.
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
     `.backlogit/queue/006-DL.md` with zero live governing/dependency references
     (and at the moment `backlogit archive 006-DL` was executed in Step 1,
     `.backlogit/queue/` had zero occurrences of `006-DL` of any kind).
   * We must decide:
     * Whether to archive `006-DL` (`backlogit archive 006-DL`) now that `017-S`
       is shipped and zero live governing/dependency references remain in
       `.backlogit/queue/` (precisely defining a *live governing/dependency
       reference* vs. an *audit citation* per Finding 8).
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
    pass is a **non-consensus** degraded outcome (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`)
    or that when `dispatch_mode: multi-agent` is active, fewer than two
    independent reviewer instances returning results halts with
    `INSUFFICIENT_REVIEWERS` (`BLOCKED`) rather than degrading mid-run.
  * In `.github/agents/subagents/adversarial-review.agent.md`, Phase 1 step 3
    introduces `TOOL_DEGRADED: subagent-dispatch — declared fallback: single-agent-declared-degradation`,
    but Phases 2–7, Output, and the Quality Criteria bullets (`The output must
    include all three confidence tiers (never drop LOW findings)` and `If fewer
    than 2 reviewer instances return results, halt and report the failure`) do
    not distinguish between:
    1. **`dispatch_mode: multi-agent`** (Finding 5 & Comment `4543766839`:
       define `active_reviewers` as the count of independent reviewer instances
       that successfully returned valid JSON findings; if
       `active_reviewers < 2`, halt immediately with `INSUFFICIENT_REVIEWERS`
       (`BLOCKED`); in normal full-pool mode (`active_reviewers == reviewers`),
       every `HIGH` consensus P0/P1 finding and every `P0` (`CRITICAL`) finding
       regardless of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`)
       blocks the gate (`BLOCKED`) while `MEDIUM` Majority/Plurality P1 findings
       require explicit disposition/acknowledgment; if
       `2 <= active_reviewers < reviewers`, log
       `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`,
       compute Phase 3 consensus thresholds against `active_reviewers` so
       unanimous agreement across all `active_reviewers` is classified
       `HIGH / Consensus section`, and in `_ship.agent.md` Step 4.4 block the
       review gate (`BLOCKED`) on **ALL unresolved P0 and P1 (`CRITICAL` and
       `MAJOR`) findings regardless of confidence (`HIGH`, `MEDIUM`
       Majority/Plurality, or `LOW`)** until resolved or dispositioned under
       P-021), and
    2. **`dispatch_mode: single-agent-declared-degradation`** (entered at Phase 1
       step 3 only when subagent dispatch is unavailable or blocked by runtime
       privacy / no-third-party-transmission policy under P-012 item 5, where
       no multi-reviewer consensus is claimed, `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`
       is recorded, Phase 4 uses severity-only scoring with
       `confidence_tier: DEGRADED_SINGLE_AGENT` and `confidence_weight = 1.0`
       ordered by `severity_score` alone, Phase 5 routes actions by severity
       only (`P0/P1 -> safe_auto` when mechanical/in-scope or
       `gated_auto`/`manual` when architectural; `P2 -> gated_auto`;
       `P3 -> advisory`), Phase 6 / Output emits a single
       `### Degraded Single-Agent Findings (Non-Consensus)` section, and Phase 7
       executes an in-session, severity-gated post-remediation re-review over
       only the files modified by `safe_auto` fixes without subagent pool
       redispatch, retaining the hard 2-cycle recursion cap, evaluating residual
       `P0`/`P1` severity findings under `confidence_tier: DEGRADED_SINGLE_AGENT`,
       and recording any remaining findings after 2 cycles as
       `post_remediation_residual: true` under `DEGRADED_SINGLE_AGENT`), and
    3. **`mode: report-only` enforcement across Phases 6–7 and Quality Criteria**
       (Finding 6: when `mode: report-only` is set — whether in `multi-agent` or
       `single-agent-declared-degradation` mode — the agent is strictly
       read-only over repository source/config files: Phase 6 writes only the
       review report and remediation plan without applying `safe_auto` edits
       directly, and Phase 7 records `post_remediation.status: "skipped"` with
       reason `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`;
       automatic `safe_auto` fix application in Phase 6 and the Phase 7
       post-remediation re-review loop run ONLY when `mode` is NOT `report-only`
       AND `post_remediation_review: true` AND each `safe_auto` candidate passes
       the P-021 C1 same-contract-surface check).
* **Retained deliberation lifecycle surface (`006-DL`,
  `.github/skills/shipment-reconcile/SKILL.md`, and Constitution Principle IV)**:
  * Verified `.backlogit/queue/` and `.backlogit/archive/`:
    * `017-F` (`.backlogit/archive/017-F.md`, `status: archived`,
      `archived_status: done`) and `017-S` (`.backlogit/archive/017-S.md`,
      `status: archived`, `archived_status: shipped`) are both in
      `.backlogit/archive/`.
    * At the moment `backlogit archive 006-DL` was executed in Step 1, no file
      in `.backlogit/queue/` other than `.backlogit/queue/006-DL.md` itself had
      any occurrence of `006-DL` of any kind.
    * Furthermore, per Finding 8 and Round-2 Pass, we precisely define a **live
      governing/dependency reference** as any unarchived artifact in
      `.backlogit/queue/` (regardless of `queued`, `active`, `blocked`, or
      `done` status) whose `custom_fields.source_deliberation_id`, frontmatter
      `dependencies` / `references`, or unshipped covering-feature/shipment
      scope is governed by the deliberation, or any active stash entry whose
      `deliberation_id` links to it (excluding the cleanup stash entry that
      itself directs archiving the orphaned deliberation). Historical
      audit/disposition prose in a subsequent cleanup deliberation/feature/task
      (such as `008-DL`, `019-F`, `019-S`, and `019.002-T` recording that
      `006-DL` was archived per `9A9D8106`) is an **audit citation, not a live
      governing reference**.
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
       with zero remaining live governing/dependency references in
       `.backlogit/queue/` (and no active stash referrer), Stage or the operator
       archives `<id>-DL` via `backlogit archive <id>-DL`.

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
step 3, Phases 2–7, and Quality Criteria) so the two modes are mutually
exclusive and unambiguous across every downstream phase:

1. **Multi-agent consensus mode (`dispatch_mode: multi-agent`, Finding 5 &
   Round-2 Pass)**: Entered when subagent reviewer dispatch is available at
   Phase 1 capability check. Replace the conflicting "wait indefinitely /
   partial consensus is not valid" wording with a bounded per-reviewer
   settlement wait, map reviewer JSON severities deterministically
   (`CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`, with no ambiguous split of
   `MAJOR`), and define `active_reviewers` as the count of independent reviewer
   instances that successfully returned valid JSON findings:
   * If `active_reviewers < 2`, `adversarial-review` and Ship MUST halt
     immediately with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) — never downgrading a
     failed or partial multi-reviewer dispatch into
     `single-agent-declared-degradation` mid-run, and never counting 1 reviewer
     as consensus.
   * If `active_reviewers == reviewers` (normal full-pool mode), in
     `_ship.agent.md` Step 4.4 every `HIGH` consensus P0/P1 finding AND every
     `P0` (`CRITICAL`) finding regardless of confidence (`HIGH`, `MEDIUM`
     Majority/Plurality, or `LOW`) blocks the gate (`BLOCKED`), while `MEDIUM`
     (both `Majority` and `Plurality`) P1 findings require explicit
     disposition/acknowledgment.
   * If `2 <= active_reviewers < reviewers` (partial pool degradation, Comment
     `4543766839`), log
     `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`
     and compute Phase 3 consensus thresholds against `active_reviewers` as the
     consensus denominator (so unanimous agreement across all
     `active_reviewers` is classified `HIGH / Consensus section`), AND in
     `_ship.agent.md` Step 4.4 because a missing reviewer could have
     corroborated a single-survivor finding, **ALL unresolved P0 and P1
     (`CRITICAL` and `MAJOR`) findings regardless of confidence (`HIGH`,
     `MEDIUM` Majority/Plurality, or `LOW`) block the review gate (`BLOCKED`)**
     until resolved or dispositioned under P-021.
   * Enforces `HIGH`/`MEDIUM`/`LOW` confidence tiers against `active_reviewers`,
     confidence × severity scoring and routing, mandatory
     `Consensus / Majority / Plurality / Unique` sections, and reviewer-pool
     redispatch in Phase 7 (when `mode != report-only` and
     `authorized_contract_scope` is present).
2. **Declared single-agent degraded mode (`dispatch_mode: single-agent-declared-degradation`)**:
   Entered ONLY at Phase 1 step 3 when subagent reviewer dispatch is unavailable
   at the P-012 capability probe or restricted by runtime privacy /
   no-third-party-transmission policy (`TOOL_DEGRADED: subagent-dispatch —
   declared fallback: single-agent-declared-degradation`). It is an explicitly
   authorized **non-consensus** degraded outcome:
   * **Phases 2–3**: Branches single-agent degraded mode from multi-reviewer
     consensus mode, running the full multi-persona rubric in-session without
     claiming `HIGH`/`MEDIUM`/`LOW` consensus tiers.
   * **Phase 4 (Confidence-Weighted Scoring)**: Maps reviewer severities
     deterministically (`CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`) and
     branches to use **severity-only scoring**
     (`confidence_tier: DEGRADED_SINGLE_AGENT`, `confidence_weight = 1.0`,
     ordered by `severity_score` alone without claiming `HIGH`/`MEDIUM`/`LOW`
     multi-reviewer consensus confidence).
   * **Phase 5 (Action Classification & Remediation Routing)**: Branches to
     route actions by **severity only** (`P0/P1 [CRITICAL/MAJOR] -> safe_auto`
     when mechanical/in-scope or `gated_auto`/`manual` when architectural;
     `P2 [MINOR] -> gated_auto/advisory`) instead of
     `confidence_tier × severity`.
   * **Phase 6 & Output Format**: Branches so the report uses a single
     `### Degraded Single-Agent Findings (Non-Consensus)` section instead of
     mandatory `Consensus / Majority / Plurality / Unique` multi-reviewer
     sections, and records `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`
     (and `confidence: "DEGRADED_SINGLE_AGENT"` in work item entries).
   * **Phase 6, Phase 7 (Post-Remediation Re-Review, lines 254–274),
     `mode: report-only` enforcement, and `authorized_contract_scope`
     fail-closed (Finding 6 & Round-2 Pass)**: Explicitly honors
     `mode: report-only` and requires `authorized_contract_scope` in both
     `multi-agent` and `single-agent-declared-degradation` modes:
     - When `mode: report-only` is set, the agent is **strictly read-only** over
       repository source/config files: Phase 6 writes only the review report and
       remediation plan (without applying `safe_auto` edits directly), and Phase
       7 records `post_remediation.status: "skipped"` with reason
       `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`.
     - When `mode` is NOT `report-only`, require an explicit
       `authorized_contract_scope` input to evaluate P-021 C1 before applying
       `safe_auto` fixes; when `authorized_contract_scope` is absent or
       ambiguous, fail closed to `mode: report-only` (read-only report +
       remediation plan, skipping automatic file edits and Phase 7).
     - Automatic `safe_auto` fix application in Phase 6 and the Phase 7
       post-remediation re-review loop (multi-agent redispatch in `multi-agent`
       mode, or in-session severity-gated re-review in
       `single-agent-declared-degradation` mode over only the files modified by
       `safe_auto` fixes without subagent pool redispatch, retaining the hard
       2-cycle recursion cap, evaluating residual `P0`/`P1` severity findings
       under `confidence_tier: DEGRADED_SINGLE_AGENT`, and recording any
       remaining findings after 2 cycles as `post_remediation_residual: true`
       under `DEGRADED_SINGLE_AGENT`) run ONLY when `mode` is NOT `report-only`
       AND `post_remediation_review: true` AND `authorized_contract_scope` is
       explicitly provided and each `safe_auto` candidate passes the P-021 C1
       same-contract-surface check.
   * **Quality Criteria**: Qualifies the minimum-2-reviewers rule,
     `active_reviewers` partial-pool consensus thresholds, `mode: report-only`
     read-only enforcement, and the confidence-tier / consensus-section
     requirements so multi-reviewer mode enforces `HIGH`/`MEDIUM`/`LOW`
     confidence tiers against `active_reviewers` and `active_reviewers >= 2`,
     while `single-agent-declared-degradation` enforces `DEGRADED_SINGLE_AGENT`
     severity-only scoring, routing, output, and (when `mode != report-only` and
     `post_remediation_review: true`) in-session Phase 7 post-remediation
     re-review.
   * Gates Ship Step 4.4 readiness on `P0=0, P1=0` (any `P0`/`P1` blocks with
     `BLOCKED`; `P2`/`P3` require follow-up or acknowledgment) and removes the
     stale `(approved for 012-S–017-S)` range cap in `_ship.agent.md`.

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

#### Option 2B (Chosen): Archive `006-DL` in Stage via `backlogit archive 006-DL` and document the workspace-local post-closure Stage/operator archival rule for `retained_engine_unverified` deliberations with zero live governing/dependency references

1. **Immediate Stage hygiene action (Finding 8)**: Archive
   `.backlogit/queue/006-DL.md` via `backlogit archive 006-DL` in this Stage
   session after verifying that `017-F` and `017-S` are archived and `006-DL`
   has zero live governing/dependency references in `.backlogit/queue/` (at the
   moment `backlogit archive 006-DL` was executed in Step 1, `.backlogit/queue/`
   had zero occurrences of `006-DL` of any kind).
2. **Workspace-local skill and compound documentation (`019.002-T`, Finding 8 &
   Round-2 Pass)**:
   * Precisely define **"live governing/dependency references"**:
     - A *live governing/dependency reference* is any unarchived artifact in
       `.backlogit/queue/` (regardless of `queued`, `active`, `blocked`, or
       `done` status) whose `custom_fields.source_deliberation_id`, frontmatter
       `dependencies` / `references`, or unshipped covering-feature/shipment
       scope is governed by the deliberation, or any active stash entry whose
       `deliberation_id` links to it (excluding the cleanup stash entry that
       itself directs archiving the orphaned deliberation).
     - Historical audit/disposition prose in a subsequent cleanup
       deliberation/feature/task (such as `008-DL`, `019-F`, `019-S`, and
       `019.002-T` recording that `006-DL` was archived per `9A9D8106`) is an
       **audit citation, not a live governing reference**, and at the moment
       `backlogit archive 006-DL` was executed in Step 1, `.backlogit/queue/`
       had zero occurrences of `006-DL` of any kind.
   * **In `.github/skills/shipment-reconcile/SKILL.md`** (Cascade Close Sub-Procedure
     step 7 and Quality Criteria), clarify that while Ship never archives
     `retained_engine_unverified` during closure (failing closed to retain and
     stashing a follow-up when zero live governing/dependency references remain),
     once the covering shipment and feature are archived in `.backlogit/archive/`
     and the deliberation has `status: queued` with zero remaining live
     governing/dependency references in `.backlogit/queue/` and
     `.backlogit/stash.jsonl`, Stage (at triage/Step 5.6) or the operator
     archives the orphaned deliberation via `backlogit archive <dl_id>`.
   * In `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`,
     record the institutional learning from `017-S` / `006-DL`, including the
     exact definition of live governing/dependency references vs. historical
     audit citations, why `VERIFIED_CASCADE_ENGINE_MINOR_LINES` in the external
     `autoharness` Python installation is outside the `go-teams` workspace
     boundary under Constitution Principle IV, and how Stage/operators cleanly
     retire zero-governing-reference `retained_engine_unverified` deliberations.

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
| Multi-reviewer consensus integrity (`PRRT_kwDOTStvas62RRVH`) | Halts all degraded CLI reviews | Preserves `INSUFFICIENT_REVIEWERS` halt for multi-agent mode and explicitly labels single-agent fallback as non-consensus (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) |
| Backlog queue hygiene (`006-DL`) | Leaves `006-DL` queued in `.backlogit/queue/` | Archives `006-DL` via `backlogit archive 006-DL` and documents the post-closure Stage/operator archival rule |

## Decision

We adopt **Option 1B + Option 2B**:

1. Archive orphaned deliberation **`006-DL`** during Step 5.6 of this Stage
   session via `backlogit archive 006-DL` (verified zero live
   governing/dependency references in `.backlogit/queue/`, zero occurrences of
   `006-DL` of any kind in `.backlogit/queue/` at the moment `backlogit archive 006-DL`
   ran in Step 1, and covering feature `017-F` and shipment `017-S` already
   archived).
2. Synthesize covering feature **`019-F`** (*Reconcile harness review-gate
   single-agent fallback and post-closure retained deliberation hygiene*) and
   shipment **`019-S`** (chained after `018-S` via
   `backlogit dep add 019-S 018-S --type blocks`) with two width-isolated tasks:
   * **`019.001-T` (U1, `config` domain — harness agent definitions, 2 files)**:
     Reconcile `.github/agents/_ship.agent.md` and
     `.github/agents/subagents/adversarial-review.agent.md` (Phases 1–7, Output,
     and Quality Criteria) so (1) `dispatch_mode: multi-agent` defines
     `active_reviewers`, halts with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) when
     `active_reviewers < 2`, in full-pool mode (`active_reviewers == reviewers`)
     blocks Ship Step 4.4 on every `HIGH` consensus P0/P1 finding and every `P0`
     finding regardless of confidence (while `MEDIUM` Majority/Plurality P1
     findings require explicit disposition/acknowledgment), and when
     `2 <= active_reviewers < reviewers` logs `REVIEWER_POOL_DEGRADED`, computes
     Phase 3 consensus thresholds against `active_reviewers`, and blocks Ship
     Step 4.4 on **ALL unresolved P0 and P1 (`CRITICAL` and `MAJOR`) findings
     regardless of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`)**
     until resolved or dispositioned under P-021; (2) `single-agent-declared-degradation`
     is an explicitly authorized non-consensus degraded outcome
     (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) with
     severity-only scoring (`confidence_tier: DEGRADED_SINGLE_AGENT`,
     `confidence_weight = 1.0`), severity-only action routing, a single
     `### Degraded Single-Agent Findings (Non-Consensus)` output section, and an
     in-session, severity-gated Phase 7 post-remediation re-review over
     `safe_auto`-modified files; and (3) `mode: report-only` is strictly
     read-only in Phases 6–7 (`post_remediation.status: "skipped"` with reason
     `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`),
     running automatic `safe_auto` edits and Phase 7 re-review ONLY when
     `mode != report-only` AND `post_remediation_review: true` AND each
     `safe_auto` candidate passes the P-021 C1 same-contract-surface check
     (`S` / `low`).
   * **`019.002-T` (U2, `docs` domain — skill & compound documentation, 2 files)**:
     Document the post-closure Stage/operator archival rule for
     `retained_engine_unverified` deliberations with zero live
     governing/dependency references (distinguishing live governing references
     from historical audit citations) and the Principle IV external
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
  vs. `TOOL_DEGRADED: subagent-dispatch` -> `dispatch_mode: single-agent-declared-degradation`),
  forbidding any transition from `multi-agent` to
  `single-agent-declared-degradation` after Phase 2 dispatch begins, and
  computing Phase 3 consensus thresholds against `active_reviewers` with
  `REVIEWER_POOL_DEGRADED` blocking rules when `2 <= active_reviewers < reviewers`.
* **Risk of archiving a `retained_engine_unverified` deliberation that still has
  live governing/dependency references**: Mitigated in `019.002-T` by requiring
  verification of zero live governing/dependency references
  (`custom_fields.source_deliberation_id`, frontmatter `dependencies` /
  `references`, or unshipped covering-feature/shipment scope, as distinct from
  historical audit citations) across `.backlogit/queue/` and
  `.backlogit/stash.jsonl` (plus confirming the covering shipment and feature
  are archived in `.backlogit/archive/`) before Stage or the operator runs
  `backlogit archive <dl_id>`.
