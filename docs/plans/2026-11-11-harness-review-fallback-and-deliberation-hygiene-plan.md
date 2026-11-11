---
title: Harness review-gate fallback reconciliation and retained deliberation hygiene implementation plan
description: Implementation plan for 019-F / 019-S reconciling the single-agent declared-degradation review fallback against the adversarial-review multi-reviewer halt and documenting post-closure archival of retained_engine_unverified deliberations
source: docs/decisions/2026-11-11-harness-review-fallback-and-deliberation-hygiene-deliberation.md
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
date: 2026-11-11
status: approved
tags:
  - harness
  - adversarial-review
  - p-012
  - p-021
  - shipment-reconcile
  - backlogit
---

## Problem Frame

Following the accepted deliberation in
[2026-11-11-harness-review-fallback-and-deliberation-hygiene-deliberation.md](../decisions/2026-11-11-harness-review-fallback-and-deliberation-hygiene-deliberation.md)
(`008-DL`), stash entries **`680A62E6`** (P-021 C2 deferred scope expansion from
`015-S`, `015-F`, `015.001-T`, PR `#38`, review thread `PRRT_kwDOTStvas62RRVH`)
and **`9A9D8106`** (follow-up from `017-S` closure regarding orphaned
`retained_engine_unverified` deliberation `006-DL`) are planned under covering
feature **`019-F`** and shipment **`019-S`** (chained after `018-S`).

Two contract surfaces are addressed:

1. **Harness review-gate fallback reconciliation (`680A62E6`)**:
   * In `.github/agents/_ship.agent.md` (Step 4.4) and
     `.github/agents/subagents/adversarial-review.agent.md` (Phase 1 step 3,
     Phases 2–7, Output, and Quality Criteria), the P-012 item 5
     `single-agent-declared-degradation` fallback must be explicitly separated
     from `dispatch_mode: multi-agent` consensus review so that:
     * When `subagent-dispatch` is active (`dispatch_mode: multi-agent`, Finding
       5 & Comment `4543766839`), define `active_reviewers` as the count of
       independent reviewer instances that successfully returned valid JSON
       findings. If `active_reviewers < 2`, halt immediately with
       `INSUFFICIENT_REVIEWERS` (`BLOCKED`) and never silently degrade to
       single-agent review mid-run. In normal full-pool mode
       (`active_reviewers == reviewers`), every `HIGH` consensus P0/P1 finding
       AND every `P0` (`CRITICAL`) finding regardless of confidence (`HIGH`,
       `MEDIUM` Majority/Plurality, or `LOW`) blocks the gate (`BLOCKED`), while
       `MEDIUM` (both `Majority` and `Plurality`) P1 findings require explicit
       disposition/acknowledgment. If `2 <= active_reviewers < reviewers`
       (partial pool degradation), log
       `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`,
       compute Phase 3 consensus thresholds against `active_reviewers` (so
       unanimous agreement across all `active_reviewers` is classified
       `HIGH / Consensus section`), and in `_ship.agent.md` Step 4.4 block the
       review gate (`BLOCKED`) on **ALL unresolved P0 and P1 (`CRITICAL` and
       `MAJOR`) findings regardless of confidence (`HIGH`, `MEDIUM`
       Majority/Plurality, or `LOW`)** until resolved or dispositioned under
       P-021.
     * When `subagent-dispatch` is unavailable at the Phase 1 P-012 pre-dispatch
       capability probe or restricted by runtime privacy /
       no-third-party-transmission policy (`TOOL_DEGRADED: subagent-dispatch —
       declared fallback: single-agent-declared-degradation`),
       `single-agent-declared-degradation` is an explicitly authorized
       **non-consensus** degraded outcome (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`)
       that branches Phases 2–7, Output, and Quality Criteria to use
       severity-only scoring (`confidence_tier: DEGRADED_SINGLE_AGENT`,
       `confidence_weight = 1.0`, ordered by `severity_score` alone),
       severity-only action routing (`P0/P1 -> safe_auto` when
       mechanical/in-scope or `gated_auto`/`manual` when architectural;
       `P2 -> gated_auto`; `P3 -> advisory`), a single
       `### Degraded Single-Agent Findings (Non-Consensus)` report section
       instead of mandatory `Consensus / Majority / Plurality / Unique`
       sections, and an in-session, severity-gated Phase 7 post-remediation
       re-review over only the files modified by `safe_auto` fixes (without
       attempting subagent pool redispatch, retaining the hard 2-cycle recursion
       cap, evaluating residual `P0`/`P1` severity findings under
       `confidence_tier: DEGRADED_SINGLE_AGENT` instead of `HIGH`-confidence
       multi-reviewer consensus findings, and recording any remaining findings
       after 2 cycles as `post_remediation_residual: true` under
       `DEGRADED_SINGLE_AGENT`), gating Ship readiness on `P0=0, P1=0`.
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
2. **Post-closure `retained_engine_unverified` deliberation hygiene (`9A9D8106`,
   Finding 8)**:
   * Orphaned queued deliberation `006-DL` (retained at `017-S` closure because
     `backlogit 1.12.0` is not listed in the external `autoharness` Python
     package's `VERIFIED_CASCADE_ENGINE_MINOR_LINES`) has zero live
     governing/dependency references in `.backlogit/queue/` and its covering
     feature `017-F` and shipment `017-S` are archived (and at the moment
     `backlogit archive 006-DL` was executed in Step 1, `.backlogit/queue/` had
     zero occurrences of `006-DL` of any kind).
   * We precisely define **"live governing/dependency references"**: a *live
     governing/dependency reference* is any unarchived artifact in
     `.backlogit/queue/` (regardless of `queued`, `active`, `blocked`, or `done`
     status) whose `custom_fields.source_deliberation_id`, frontmatter
     `dependencies` / `references`, or unshipped covering-feature/shipment scope
     is governed by the deliberation, or any active stash entry whose
     `deliberation_id` links to it (excluding the cleanup stash entry that
     itself directs archiving the orphaned deliberation) — whereas historical
     audit/disposition prose in a subsequent cleanup deliberation/feature/task
     (such as `008-DL`, `019-F`, `019-S`, and `019.002-T` recording that
     `006-DL` was archived per `9A9D8106`) is an **audit citation, not a live
     governing reference**.
   * Because **Constitution Principle IV (CLI Workspace Containment,
     NON-NEGOTIABLE)** forbids modifying files outside `C:\Source\GitHub\go-teams`
     (such as the external Python installation `C:\Python\Python315\...`),
     `.github/skills/shipment-reconcile/SKILL.md` and a new compound learning
     document `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`
     will document the workspace-local post-closure Stage/operator archival rule
     for `retained_engine_unverified` deliberations with zero live
     governing/dependency references and record the Principle IV boundary.

## Requirements Trace

| Requirement | Stash ID / Provenance | Implementation unit | Verification |
|---|---|---|---|
| R1: Reconcile `.github/agents/_ship.agent.md` Step 4.4 and `.github/agents/subagents/adversarial-review.agent.md` so `dispatch_mode: multi-agent` defines `active_reviewers`, halts with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) when `active_reviewers < 2`, in full-pool mode (`active_reviewers == reviewers`) blocks Ship Step 4.4 on every `HIGH` consensus P0/P1 finding and every `P0` finding regardless of confidence (while `MEDIUM` Majority/Plurality P1 findings require explicit disposition/acknowledgment), and when `2 <= active_reviewers < reviewers` logs `REVIEWER_POOL_DEGRADED`, computes Phase 3 consensus thresholds against `active_reviewers`, and blocks Ship Step 4.4 on ALL unresolved P0 and P1 (`CRITICAL` and `MAJOR`) findings regardless of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`) | `680A62E6` (`015-S`, `015-F`, `015.001-T`, PR `#38`, `PRRT_kwDOTStvas62RRVH`, `008-DL`, Finding 5, Comment `4543766839`) | U1 (`019.001-T`) | Template Integrity & Schema-CLI-Docs verification + `go test -race ./...` |
| R2: Define `dispatch_mode: single-agent-declared-degradation` in `_ship.agent.md` and `adversarial-review.agent.md` (Phases 1–7, Output, and Quality Criteria) as an explicitly authorized non-consensus P-012 item 5 fallback (`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`) with severity-only scoring (`confidence_tier: DEGRADED_SINGLE_AGENT`, `confidence_weight = 1.0`), severity-only action routing, a single `### Degraded Single-Agent Findings (Non-Consensus)` section, and an in-session severity-gated Phase 7 post-remediation re-review over `safe_auto`-modified files, and strictly honor `mode: report-only` in Phases 6–7 (read-only over repo source/config files, `post_remediation.status: "skipped"`, running `safe_auto` edits and Phase 7 re-review ONLY when `mode != report-only` AND `post_remediation_review: true` AND P-021 C1 check passes) | `680A62E6` (`PRRT_kwDOTStvas62RRVH`, PR `#44` threads `PRRT_kwDOTStvas62Ve-8`, `PRRT_kwDOTStvas62V-38`, `008-DL`, Finding 6) | U1 (`019.001-T`) | Template Integrity & Schema-CLI-Docs verification + `go test -race ./...` |
| R3: Document the post-closure Stage/operator archival rule for `retained_engine_unverified` deliberations with zero live governing/dependency references in `.backlogit/queue/` (distinguishing live governing references from historical audit citations) and the Constitution Principle IV boundary for external `autoharness` `VERIFIED_CASCADE_ENGINE_MINOR_LINES` in `.github/skills/shipment-reconcile/SKILL.md` and `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md` | `9A9D8106` (`017-S` closure doc, `006-DL`, `008-DL`, Finding 8) | U2 (`019.002-T`) | Markdown / Template Integrity verification + `backlogit doctor` |

## Implementation Units

### U1 (`019.001-T`) — Reconcile `_ship.agent.md` and `adversarial-review.agent.md` single-agent degraded fallback, `active_reviewers` partial-pool consensus thresholds, `mode: report-only` read-only enforcement, and multi-reviewer `INSUFFICIENT_REVIEWERS` halt

* **Domain:** config (harness agent specifications). **Posture:** characterization-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `.github/agents/subagents/adversarial-review.agent.md`:
     * In **Phase 1: Prepare (step 3)**: Explicitly define the pre-dispatch
       P-012 capability branch:
       - When cross-model / subagent reviewer dispatch is available
         (`TOOL_OK: subagent-dispatch`), set `dispatch_mode: multi-agent`.
         Require at least 2 independent reviewer instances returning structured
         results; if fewer than 2 reviewer instances return results in
         `dispatch_mode: multi-agent`, halt immediately with
         `INSUFFICIENT_REVIEWERS` (`BLOCKED`) — never downgrade a partial or
         failed multi-agent run into `single-agent-declared-degradation` after
         dispatch begins.
       - When subagent reviewer dispatch is unavailable at the Phase 1
         capability probe or restricted by runtime privacy /
         no-third-party-transmission policy (P-012 item 5), log
         `TOOL_DEGRADED: subagent-dispatch — declared fallback: single-agent-declared-degradation`,
         set `dispatch_mode: single-agent-declared-degradation` and
         `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, and execute
         the full multi-persona review rubric in-session without external model
         transmission.
     * In **Phase 2: Parallel Dispatch** and **Phase 3: Aggregate and Classify**
       (Finding 5 & Round-2 Pass):
       - In `dispatch_mode: multi-agent`, replace the conflicting "wait
         indefinitely / partial consensus is not valid" wording with a bounded
         per-reviewer settlement wait, and define `active_reviewers` as the
         count of independent reviewer instances that successfully returned
         valid JSON findings.
       - If `active_reviewers < 2`, halt immediately with
         `INSUFFICIENT_REVIEWERS` (`BLOCKED`).
       - If `2 <= active_reviewers < reviewers` (partial pool degradation), log
         `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`
         and compute Phase 3 consensus thresholds against `active_reviewers` as
         the consensus denominator (so unanimous agreement across all
         `active_reviewers` is classified `HIGH / Consensus section`).
       - In `single-agent-declared-degradation`, findings are collected across
         the in-session multi-persona rubric and assigned
         `confidence_tier: DEGRADED_SINGLE_AGENT`.
     * In **Phase 4: Order and Score (Confidence-Weighted Scoring)** and
       **Phase 5: Route to Action Classes (Action Classification & Remediation
       Routing)**:
       - Map reviewer JSON severities deterministically (`CRITICAL -> P0`,
         `MAJOR -> P1`, `MINOR -> P2`, with no ambiguous split of `MAJOR`).
       - Branch Phase 4 for `single-agent-declared-degradation` to use
         **severity-only scoring**: `confidence_tier: DEGRADED_SINGLE_AGENT`,
         `confidence_weight = 1.0`, ordered by `severity_score`
         (`severity_weight`) alone without claiming `HIGH`/`MEDIUM`/`LOW`
         multi-reviewer consensus confidence.
       - Branch Phase 5 for `single-agent-declared-degradation` to route actions
         by **severity only**:
         * `P0/P1` (`CRITICAL` / `MAJOR`) -> `safe_auto` when
           mechanical/in-scope or `gated_auto`/`manual` when architectural
         * `P2` (`MINOR`) -> `gated_auto` / `advisory`
         instead of `confidence_tier × severity`.
     * In **Phase 6: Produce Output** and **Output** (Finding 6 & Round-2 Pass):
       - Branch for `single-agent-declared-degradation` so the report uses a
         single `### Degraded Single-Agent Findings (Non-Consensus)` section
         (plus the remediation plan) instead of mandatory
         `Consensus / Majority / Plurality / Unique` multi-reviewer sections,
         records `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, and
         sets `confidence: "DEGRADED_SINGLE_AGENT"` on any generated P0/P1
         backlog work item entries.
       - Explicitly honor `mode: report-only` and `authorized_contract_scope`
         (in both `multi-agent` and `single-agent-declared-degradation` modes):
         when `mode: report-only` is set, the agent is **strictly read-only**
         over repository source/config files and Phase 6 writes only the review
         report and remediation plan without applying `safe_auto` edits
         directly. When `mode` is NOT `report-only`, require an explicit
         `authorized_contract_scope` input to evaluate P-021 C1 before applying
         `safe_auto` fixes; when `authorized_contract_scope` is absent or
         ambiguous, fail closed to `mode: report-only` (read-only report +
         remediation plan, skipping automatic file edits and Phase 7).
     * In **Phase 7: Post-Remediation Re-Review (lines 254–274)** (Finding 6 &
       Round-2 Pass):
       - When `mode: report-only` is set (or when `authorized_contract_scope` is
         absent/ambiguous, failing closed to `mode: report-only`), do NOT apply
         fixes or run re-review; record `post_remediation.status: "skipped"`
         with reason `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`.
       - Automatic `safe_auto` fix application in Phase 6 and the Phase 7
         post-remediation re-review loop (multi-agent redispatch in
         `multi-agent` mode, or in-session severity-gated re-review in
         `single-agent-declared-degradation` mode over only the files modified
         by `safe_auto` fixes without subagent pool redispatch, retaining the
         hard 2-cycle recursion cap, evaluating residual `P0`/`P1` severity
         findings under `confidence_tier: DEGRADED_SINGLE_AGENT`, and recording
         any remaining findings after 2 cycles as
         `post_remediation_residual: true` under `DEGRADED_SINGLE_AGENT`) run
         ONLY when `mode` is NOT `report-only` AND
         `post_remediation_review: true` AND `authorized_contract_scope` is
         explicitly provided and each `safe_auto` candidate passes the P-021 C1
         same-contract-surface check.
     * In **Quality Criteria (including line 298)**: Replace any conflicting
       "partial consensus is not valid" wording and qualify the
       minimum-2-reviewers rule, bounded per-reviewer settlement wait,
       `active_reviewers` partial-pool consensus thresholds, deterministic
       `CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2` severity mapping,
       `mode: report-only` / `authorized_contract_scope` fail-closed read-only
       enforcement, and the confidence-tier / consensus-section requirements so
       `dispatch_mode: multi-agent` enforces `HIGH`/`MEDIUM`/`LOW` confidence
       tiers against `active_reviewers`, mandatory consensus sections, and
       `active_reviewers >= 2` (halting with `INSUFFICIENT_REVIEWERS` if
       `active_reviewers < 2`), while
       `dispatch_mode: single-agent-declared-degradation` enforces
       `DEGRADED_SINGLE_AGENT` severity-only scoring, routing,
       `### Degraded Single-Agent Findings (Non-Consensus)` output, and (when
       `mode != report-only`, `post_remediation_review: true`, and
       `authorized_contract_scope` is present) in-session Phase 7
       post-remediation re-review.
  2. In `.github/agents/_ship.agent.md` (Step 4.4, around line 465):
     * Remove the stale `(approved for 012-S–017-S)` shipment-range qualifier on
       the P-012 Item 5 Privacy / Single-Agent Fallback.
     * Map reviewer JSON severities deterministically (`CRITICAL -> P0`,
       `MAJOR -> P1`, `MINOR -> P2`, with no ambiguous split of `MAJOR`) and
       explicitly distinguish the two modes (Finding 5 & Comment `4543766839`):
       - When `subagent-dispatch` is available (`dispatch_mode: multi-agent`),
         `adversarial-review` requires `active_reviewers >= 2` independent
         reviewer instances returning valid JSON findings within a bounded
         per-reviewer settlement wait; if `active_reviewers < 2`, Ship halts
         with `INSUFFICIENT_REVIEWERS` (`BLOCKED`) and never treats a single
         reviewer or failed multi-agent dispatch as consensus. In normal
         full-pool mode (`active_reviewers == reviewers`), every `HIGH`
         consensus P0/P1 finding AND every `P0` (`CRITICAL`) finding regardless
         of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`) blocks
         the gate (`BLOCKED`), while `MEDIUM` (both `Majority` and `Plurality`)
         P1 findings require explicit disposition/acknowledgment. When
         `2 <= active_reviewers < reviewers` (`REVIEWER_POOL_DEGRADED`), Phase 3
         consensus thresholds are computed against `active_reviewers` AND in
         Step 4.4 **ALL unresolved P0 and P1 (`CRITICAL` and `MAJOR`) findings
         regardless of confidence (`HIGH`, `MEDIUM` Majority/Plurality, or
         `LOW`) block the review gate (`BLOCKED`)** until resolved or
         dispositioned under P-021.
       - When `subagent-dispatch` is unavailable at the pre-dispatch capability
         check or restricted by runtime privacy / no-third-party-transmission
         policy (P-012 item 5), Ship logs
         `TOOL_DEGRADED: subagent-dispatch — declared fallback: single-agent-declared-degradation`,
         runs the full multi-persona review rubric in-session, records both
         `dispatch_mode: single-agent-declared-degradation` and
         `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)` in the review
         and closure output (using severity-only scoring, severity-only action
         routing, and a single `### Degraded Single-Agent Findings (Non-Consensus)`
         section without claiming multi-reviewer consensus), and proceeds only
         when `P0=0, P1=0`.
* **Files (2 files):** `.github/agents/subagents/adversarial-review.agent.md`,
  `.github/agents/_ship.agent.md`.
* **Verify:** Both agent markdown files have valid YAML frontmatter, matching
  `dispatch_mode` and `consensus_status` terminology, and pass all repo quality
  gates (`go test -race ./...`, `backlogit doctor`).
* **Depends on:** none.

### U2 (`019.002-T`) — Document post-closure Stage/operator archival rule for `retained_engine_unverified` deliberations with zero live governing/dependency references and Principle IV boundary

* **Domain:** docs (skill & compound learnings documentation). **Posture:** characterization-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. In `.github/skills/shipment-reconcile/SKILL.md`:
     * In **Cascade Close Sub-Procedure step 7 (`Post-cascade source-artifact
       cleanup (linked deliberations)`)** and **Quality Criteria** (Finding 8):
       Add an explicit note on the post-closure lifecycle of a
       `retained_engine_unverified` deliberation: while Ship never archives
       `retained_engine_unverified` during shipment closure (leaving it at its
       snapshotted location and stashing a follow-up item when no live work
       references it), once the covering shipment (`archived_status: shipped`)
       and covering feature (`archived_status: done`) are in
       `.backlogit/archive/` and the deliberation has `status: queued` with zero
       remaining **live governing/dependency references** in
       `.backlogit/queue/` (and no live stash referrer), Stage (during triage /
       Step 5.6) or the operator retires the orphaned deliberation via
       `backlogit archive <dl_id>`. Precisely define a **live
       governing/dependency reference** as any unarchived artifact in
       `.backlogit/queue/` (regardless of `queued`, `active`, `blocked`, or
       `done` status) whose `custom_fields.source_deliberation_id`, frontmatter
       `dependencies` / `references`, or unshipped covering-feature/shipment
       scope is governed by the deliberation, or any active stash entry whose
       `deliberation_id` links to it (excluding the cleanup stash entry that
       itself directs archiving the orphaned deliberation) — whereas historical
       audit/disposition prose in a subsequent cleanup deliberation/feature/task
       (such as `008-DL`, `019-F`, `019-S`, and `019.002-T` recording that
       `006-DL` was archived per `9A9D8106`) is an **audit citation, not a live
       governing reference** (and note that at the moment
       `backlogit archive 006-DL` was executed in Step 1, `.backlogit/queue/`
       had zero occurrences of `006-DL` of any kind). Also note that
       `VERIFIED_CASCADE_ENGINE_MINOR_LINES` in the external `autoharness`
       Python package is outside target repositories under Constitution
       Principle IV (CLI Workspace Containment) and is never edited from within
       a target repo.
  2. Create `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`:
     * Capture the institutional learning from `017-S` closure and `006-DL`
       (`9A9D8106`): why `backlogit 1.12.0` triggers
       `retained_engine_unverified` when the external `autoharness` package lists
       only `"1.11"` in `VERIFIED_CASCADE_ENGINE_MINOR_LINES`, why Ship fails
       closed to retain at closure step 7, why Constitution Principle IV forbids
       mutating external Python site-packages from the workspace, the exact
       distinction between live governing/dependency references and historical
       audit citations (noting that `.backlogit/queue/` had zero occurrences of
       `006-DL` of any kind when `backlogit archive 006-DL` ran in Step 1), and
       the `backlogit archive <dl_id>` checklist Stage/operators use to archive
       the orphaned deliberation after closure.
* **Files (2 files):** `.github/skills/shipment-reconcile/SKILL.md`,
  `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`.
* **Verify:** Both markdown files conform to `.github/instructions/markdown.instructions.md`
  and `.github/instructions/prompt-builder.instructions.md`, and
  `backlogit doctor` passes with zero issues.
* **Depends on:** U1 (`019.001-T`).

## Dependency Graph

```text
U1 (019.001-T: _ship.agent.md & adversarial-review.agent.md review fallback reconciliation)
  └──> U2 (019.002-T: shipment-reconcile/SKILL.md & compound learning doc for retained_engine_unverified)
```

* Shipment sequencing: `019-S` is chained after `018-S` via
  `backlogit dep add 019-S 018-S --type blocks` so `_Ship` executes `018-S`
  first and `019-S` unlocks automatically once `018-S` ships.

## Decisions and Rationale

* **Explicit `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)` marker and
  Phases 2–7 branching**: Distinguishes an authorized P-012 item 5 single-agent
  degraded review from a multi-reviewer consensus report across every phase of
  `adversarial-review.agent.md` (Phases 2–3 classification, Phase 4
  severity-only scoring with `confidence_tier: DEGRADED_SINGLE_AGENT` and
  `confidence_weight = 1.0`, Phase 5 severity-only action routing, Phase 6
  `### Degraded Single-Agent Findings (Non-Consensus)` output section, Phase 7
  in-session severity-gated post-remediation re-review over `safe_auto`-modified
  files retaining the 2-cycle cap under `DEGRADED_SINGLE_AGENT` without subagent
  pool redispatch, and Quality Criteria), ensuring single-agent passes never
  emit `HIGH`/`MEDIUM`/`LOW` consensus labels while keeping the
  `INSUFFICIENT_REVIEWERS` halt strict whenever `dispatch_mode: multi-agent` is
  used.
* **Archiving `006-DL` in Stage Step 5.6 + documenting the rule in `019.002-T`**:
  Stage's Role Boundary explicitly authorizes archiving backlog items on `main`
  during staging (`Create, update, archive backlog items, stash entries,
  shipment manifests`), while modifying `.github/skills/` and `docs/compound/`
  belongs to a Ship-executed task (`019.002-T`).

## Risks and Caveats

* **Cross-file wording drift between `_ship.agent.md` and `adversarial-review.agent.md`**:
  Mitigated by keeping both files in a single `config`-domain task (`019.001-T`)
  with identical `dispatch_mode`, `consensus_status`, and `INSUFFICIENT_REVIEWERS`
  identifiers.

## Constitution Check

| Principle | Disposition |
|---|---|
| I, II: Safety-first Go & Test-first | No Go production source files modified; existing Go test suite (`go test -race ./...`) verified clean |
| III, IV: Workspace isolation & CLI containment | Strictly enforces Principle IV by refusing any edit to external `C:\Python\Python315\...` packages and keeping all changes inside `C:\Source\GitHub\go-teams` |
| V: Structured observability | Adds explicit `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, `confidence_tier: DEGRADED_SINGLE_AGENT`, and `INSUFFICIENT_REVIEWERS` markers |
| VI: Single responsibility | Two width-isolated tasks (`019.001-T` in `config` domain, 2 files; `019.002-T` in `docs` domain, 2 files) |
| VII, VIII: Strict safety | Harness review-gate policy and backlog archival actions classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `008-DL`, plan, feature `019-F`, tasks `019.001-T`–`019.002-T`, and shipment `019-S` recorded in git-friendly markdown |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: refines the harness
  review-gate contract in `.github/agents/_ship.agent.md` and
  `.github/agents/subagents/adversarial-review.agent.md` (`dispatch_mode`,
  `consensus_status`, and `INSUFFICIENT_REVIEWERS` halt) and clarifies
  post-closure deliberation hygiene in `.github/skills/shipment-reconcile/SKILL.md`.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  governs P-012 item 5 privacy / no-third-party-transmission review fallback and
  Constitution Principle IV workspace containment.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: non-destructive backlog archival (`backlogit archive 006-DL`) and
  markdown specification updates only.
* External integration, operator checkpoint, or external dependency — **absent**:
  no external service or dependency changes.
* High runtime, rollout, or rollback risk — **low**: affects agent specification
  and skill documentation files only, trivially reversible via `git revert`.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 updates the harness review-gate contract in
`_ship.agent.md` and `adversarial-review.agent.md` and U2 updates the
linked-deliberation post-closure lifecycle in `shipment-reconcile/SKILL.md`.

* **Instructions and learnings consulted**:
  * `.github/instructions/constitution.instructions.md` (Principle IV CLI
    Workspace Containment)
  * `.github/instructions/prompt-builder.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `docs/closure/2026-11-10-015-S-review-and-closure.md` (`PRRT_kwDOTStvas62RRVH`)
  * `docs/closure/2026-11-10-017-S-review-and-closure.md` (`9A9D8106` / `006-DL`)

### Protected Invariants

1. **Multi-agent `INSUFFICIENT_REVIEWERS` halt and `active_reviewers`
   partial-pool consensus thresholds (Finding 5 & Comment `4543766839`)**:
   Whenever `dispatch_mode: multi-agent` is active, define `active_reviewers`
   as the count of independent reviewer instances returning valid JSON findings.
   If `active_reviewers < 2`, the review gate MUST halt with
   `INSUFFICIENT_REVIEWERS` (`BLOCKED`) and MUST NOT fall back to
   `single-agent-declared-degradation` mid-run. In normal full-pool mode
   (`active_reviewers == reviewers`), every `HIGH` consensus P0/P1 finding AND
   every `P0` (`CRITICAL`) finding regardless of confidence (`HIGH`, `MEDIUM`
   Majority/Plurality, or `LOW`) blocks the gate (`BLOCKED`), while `MEDIUM`
   (both `Majority` and `Plurality`) P1 findings require explicit
   disposition/acknowledgment. If `2 <= active_reviewers < reviewers`, the
   review workflow MUST log
   `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned`,
   compute Phase 3 consensus thresholds against `active_reviewers` (so
   unanimous agreement across all `active_reviewers` is classified
   `HIGH / Consensus`), and in `_ship.agent.md` Step 4.4 block (`BLOCKED`) on
   **ALL unresolved P0 and P1 (`CRITICAL` and `MAJOR`) findings regardless of
   confidence (`HIGH`, `MEDIUM` Majority/Plurality, or `LOW`)** until resolved
   or dispositioned under P-021.
2. **Non-consensus single-agent degradation and `mode: report-only` read-only
   enforcement across Phases 2–7, Output, and Quality Criteria (Finding 6)**:
   Whenever `dispatch_mode: single-agent-declared-degradation` is entered at
   Phase 1 pre-dispatch capability probe under P-012 item 5, the review
   workflow MUST record `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`,
   use severity-only scoring in Phase 4 (`confidence_tier: DEGRADED_SINGLE_AGENT`,
   `confidence_weight = 1.0`, ordered by `severity_score` alone), route actions
   by severity only in Phase 5 (`P0/P1 -> safe_auto` when mechanical/in-scope or
   `gated_auto`/`manual` when architectural; `P2 -> gated_auto`;
   `P3 -> advisory`), emit a single
   `### Degraded Single-Agent Findings (Non-Consensus)` section in Phase 6 /
   Output instead of `Consensus / Majority / Plurality / Unique` sections, and
   block on any `P0` or `P1` finding (`P0=0, P1=0` required to proceed). In
   both `multi-agent` and `single-agent-declared-degradation` modes, when
   `mode: report-only` is set, the agent MUST remain strictly read-only over
   repository source/config files (writing only the report/remediation plan in
   Phase 6 and recording `post_remediation.status: "skipped"` with reason
   `"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`
   in Phase 7); automatic `safe_auto` fix application and Phase 7 re-review run
   ONLY when `mode` is NOT `report-only` AND `post_remediation_review: true`
   AND each `safe_auto` candidate passes the P-021 C1 same-contract-surface
   check.
3. **Fail-closed Ship Step 7 retention and precise definition of live
   governing/dependency references (Finding 8)**: During shipment closure, Ship
   MUST continue to retain `retained_engine_unverified` deliberations at
   `shipment-reconcile` Cascade Close step 7 and `_ship.agent.md` Post-Merge
   Closure step 7; post-closure archival via `backlogit archive <dl_id>` is
   performed only by Stage or the operator after verifying the covering
   shipment/feature are in `.backlogit/archive/` and zero **live
   governing/dependency references** (items whose
   `custom_fields.source_deliberation_id`, frontmatter `dependencies` /
   `references`, or unshipped covering-feature/shipment scope depends on the
   deliberation, as distinct from historical audit citations in cleanup
   artifacts) remain in `.backlogit/queue/` and `.backlogit/stash.jsonl`.
4. **Principle IV CLI Workspace Containment**: No agent task may read-for-write
   or mutate any file outside `C:\Source\GitHub\go-teams` (including the
   external Python `autoharness` installation).

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Reconcile `single-agent-declared-degradation`, `active_reviewers` partial-pool consensus thresholds, and `mode: report-only` enforcement in `_ship.agent.md` and `adversarial-review.agent.md` | low | Preserves strict `P0=0, P1=0` gate in degraded mode, strict `active_reviewers >= 2` consensus minimum and `REVIEWER_POOL_DEGRADED` blocking in multi-agent mode, and read-only `mode: report-only` enforcement | Review gate cannot conflate single-agent fallback with multi-reviewer consensus, weaken consensus on partial reviewer return, or mutate files in `report-only` mode |
| Archive orphaned deliberation `006-DL` via `backlogit archive 006-DL` and document post-closure archival rule in `shipment-reconcile/SKILL.md` | low | Verified zero live governing/dependency references in `.backlogit/queue/` (and zero occurrences of `006-DL` of any kind at Step 1 archival) and archived status of `017-F` / `017-S` prior to non-destructive archival to `.backlogit/archive/006-DL.md` | `.backlogit/queue/` is free of orphaned `006-DL` while preserving full archive history |

### Review-Gate Capability Risks

* In this CLI environment, reviewer subagent dispatch is degraded under P-012
  (`TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`).
  The `## Plan Review` section below records literal `dispatch_mode: single-agent-declared-degradation`
  and `decision: PASS` markers and covers all six triggered personas inline.

## Runtime Verification and Closure

* **Verification**:
  * Inspect YAML frontmatter and cross-references in
    `.github/agents/_ship.agent.md`,
    `.github/agents/subagents/adversarial-review.agent.md`,
    `.github/skills/shipment-reconcile/SKILL.md`, and
    `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`.
  * Run `backlogit sync` and `backlogit doctor`.
  * Run `go test -race ./...` and `go vet ./...` to confirm repository integrity.
* **Operational closure**:
  * **Monitoring**: Verify subsequent Ship review sections record both
    `dispatch_mode: single-agent-declared-degradation` and
    `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)` when subagent
    dispatch is unavailable, and verify `.backlogit/queue/` has no orphaned
    deliberations after closure + Stage triage.
  * **Rollback**: Revert the `019-S` merge commit on `main` if any agent prompt
    or skill instruction regression is observed.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants and `strict-safety`
  `ProposedAction` / `ActionRisk` table.

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly enforces Principle IV (CLI Workspace Containment) by forbidding edits to external Python site-packages, and satisfies the 2-hour rule and width isolation (`U1`: `config` domain, 2 files; `U2`: `docs` domain, 2 files). | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — No Go code changes required; existing Go test suite and backlog doctor checks included in verification. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly scoped to stash entries `680A62E6` and `9A9D8106`; does not touch `018-S` or excluded shipment `010-S`. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Captures the `017-S` / `006-DL` `retained_engine_unverified` lifecycle in `docs/compound/workflow-issues/` for future sessions. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Cleanly partitions `dispatch_mode: multi-agent` consensus aggregation from `dispatch_mode: single-agent-declared-degradation` severity gating across `_ship.agent.md` and `adversarial-review.agent.md`. | PASS |
| Agent-Native Parity Reviewer (agent/skill workflow surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Preserves fail-closed `retained_engine_unverified` retention during Ship closure while defining deterministic Stage/operator post-closure archival once zero live governing/dependency references remain (distinguishing live governing references from historical audit citations). | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `019-F` and shipment `019-S`.
