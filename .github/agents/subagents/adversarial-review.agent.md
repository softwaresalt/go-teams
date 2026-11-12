---
name: Adversarial Review
description: "Multi-model parallel review using independent reviewer agents with different models, assembled into a consensus report with confidence-weighted findings and remediation queue. Supports a first-class anchor reviewer route, alternate model providers (e.g., Gemini), and a post-remediation re-review phase."
maturity: stable
tools: read, agent, search, edit
max_subagent_tier: 1
reasoning_effort: "xhigh"
model_provider: "anthropic"
model_family: "claude-opus-4.8"
alt_review_provider: ""
alt_review_family: ""
anchor_review_provider: "openai"
anchor_review_family: "gpt-5.6-sol"
anchor_review_reasoning_effort: "high"
subagent_depth: 2
---

# Adversarial Review

Run multiple independent reviewer agents in parallel, each using a different model,
and assemble their findings into a consensus report. Agreement across models signals
high-confidence findings; unique findings from a single model are preserved as
low-confidence observations worth human attention. The result is a structured
remediation queue with findings ordered by confidence × severity.

After auto-fixes are applied (only when `mode` is not `report-only` and an
`authorized_contract_scope` is provided), a post-remediation re-review phase
re-dispatches the same reviewer pool over the fixed files to verify no new issues
were introduced (or re-runs the in-session rubric in
`single-agent-declared-degradation` mode).
Recursion is capped at 2 cycles to prevent infinite loops.

## Why Adversarial

Different models have different blind spots. A finding that appears in all reviewer
outputs is almost certainly real. A finding that appears in only one model's output
may be a false positive — or a subtle issue that only one model caught. The protocol
preserves both signals with appropriate confidence labels, rather than losing unique
findings or trusting any single model too much.

Anchor Reviewer support (`openai` / `gpt-5.6-sol`)
adds a first-class reviewer route, defaulting to OpenAI GPT-5.6 Sol where supported,
without replacing Tier 1/2/3 diversity. If the anchor route cannot be dispatched,
record a declared fallback and continue only when the remaining reviewer pool still
satisfies the consensus minimum.

Alternate model provider support (`` / ``)
allows reviewer slots to be assigned to Gemini or other providers outside the standard
tier routing set, ensuring reviewer diversity is not limited to a single provider's
model family.

## When to Use

* Pre-merge review for high-risk changes (architecture, security, data integrity)
* Periodic sweep of a module that has accumulated significant churn
* Validating a set of automated fixes before applying them
* Any review where a single-model review felt insufficient or inconclusive

## Inputs

* `files`: (Required) Files or diff to review. Can be a list of paths, a git diff
  ref (e.g., `HEAD~1`), or a PR number.
* `reviewers`: (Optional) Total number of parallel reviewer instances, including
  the Anchor Reviewer when the anchor route is dispatchable. Default: 4 with a
  dispatchable anchor route (Anchor + Tier 1 + Tier 2 + Tier 3), otherwise 3
  (Tier 1 + Tier 2 + Tier 3). Minimum: 2 (a single reviewer provides no
  consensus signal). Maximum: 5.
* `models`: (Optional) Model tiers/routes to use for each reviewer instance.
  Specify a list matching the `reviewers` count, including `anchor_review` when
  assigning an Anchor Reviewer slot. When unset, use the count-specific mapping
  in Phase 1 so every accepted reviewer count has a deterministic slot map.
* `alt_provider`: (Optional) Alternate model provider name (e.g., `google`).
  Overrides `` for this invocation. When set, one reviewer
  slot is assigned to the alternate provider.
* `alt_family`: (Optional) Alternate model family (e.g., `gemini-2.5-flash`).
  Overrides `` for this invocation. Paired with `alt_provider`.
* `anchor_provider`: (Optional) Anchor reviewer provider. Overrides
  `openai` for this invocation.
* `anchor_family`: (Optional) Anchor reviewer model family. Overrides
  `gpt-5.6-sol` for this invocation.
* `anchor_reasoning_effort`: (Optional) Anchor reviewer reasoning effort.
  Overrides `high`; empty means use the model
  default.
* `ruleset`: (Optional) Path to a ruleset file. Defaults to
  `.github/copilot-review-instructions.md` if present, otherwise uses the
  built-in harness review ruleset.
* `output_mode`: (Optional) `consensus-only` or `full` (default — return
  consensus + majority + unique with confidence labels). `consensus-only` is a
  display filter for `LOW`-confidence `MINOR` (`P2`) advisory observations only.
  It never filters out or suppresses `P0` (`CRITICAL`) findings, `P1` (`MAJOR`)
  findings when `REVIEWER_POOL_DEGRADED` is active, `DEGRADED_SINGLE_AGENT`
  findings, or any confidence section the gate evaluates (for example `MEDIUM`
  `P1` findings that need disposition). The gate always evaluates the unfiltered
  finding set.
* `post_remediation_review`: (Optional, default `true`) Whether to run the
  post-remediation re-review phase (Phase 7) after auto-fixes are applied. Set
  to `false` to disable re-review and exit after the remediation plan.
* `mode`: (Optional) `report-only` makes this agent strictly read-only over
  repository source and config files: it writes only the review report and
  remediation plan, never applies `safe_auto` edits, and skips Phase 7. Applies
  in both `multi-agent` and `single-agent-declared-degradation` dispatch modes.
* `authorized_contract_scope`: (Required whenever `mode` is not `report-only`)
  The exact change scope the caller authorized, used to evaluate P-021 C1
  (same-contract-surface) before any `safe_auto` fix is applied. When absent or
  ambiguous, fail closed to `mode: report-only`.

## Dispatch Modes

* `dispatch_mode: multi-agent` (default): used when subagent dispatch is
  available (`TOOL_OK: subagent-dispatch`). Produces confidence-tiered consensus
  findings (HIGH / MEDIUM / LOW) computed against `active_reviewers`.
* `dispatch_mode: single-agent-declared-degradation`: entered only at the
  Phase 1 pre-dispatch capability probe when `TOOL_DEGRADED: subagent-dispatch`
  is declared under P-012 item 5. A non-consensus degraded outcome recorded as
  `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)` with
  `confidence_tier: DEGRADED_SINGLE_AGENT`. It never claims HIGH, MEDIUM, or LOW
  multi-reviewer consensus.

Reviewer JSON severities map deterministically to priority levels:
`CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`. `MAJOR` is never split.

## Output

In `dispatch_mode: single-agent-declared-degradation`, the output is a single
`### Degraded Single-Agent Findings (Non-Consensus)` section plus the remediation
plan and backlog entries (items 5 and 6 below), with
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)` recorded in the report.
Items 1 through 4 apply to `dispatch_mode: multi-agent`:

1. **Consensus findings** (confidence: HIGH) — Flagged by all `active_reviewers` agents.
   These require mandatory remediation before merge.
2. **Majority findings** (confidence: MEDIUM) — Flagged by more than half of agents.
   Require explicit acknowledgment (fix or defer with rationale).
3. **Plurality findings** (confidence: MEDIUM) — Flagged by more than one reviewer
   but not by a strict majority, such as 2 of 4 reviewers. These are not consensus,
   but they are stronger than unique observations and require explicit
   acknowledgment.
4. **Unique findings** (confidence: LOW) — Flagged by exactly one agent.
   Preserved as observations; human judgment required on whether to act.
5. **Remediation plan** — Ordered action list combining all findings, sorted by
   `confidence × severity`, with estimated action class (`safe_auto`, `gated_auto`,
   `manual`, `advisory`).
6. **Bug/issue queue entries** — For each P0 and P1 finding, a structured work item
   ready to create in the backlog using `backlogit add --type {{artifact_type}} --title {{title}}`.

Output file at `docs/closure/{YYYY-MM-DD}-{slug}-adversarial-review.md`.

## Required Protocol

### Phase 1: Prepare

1. Resolve the file list or diff to review.
2. Load the ruleset from the specified path or the default.
3. Determine the reviewer count and model route assignment. The `reviewers`
   value is the total number of slots; when the anchor route is dispatchable, the
   Anchor Reviewer consumes one slot. Use these mappings unless `models` provides
   an explicit list with the same count:

   | Reviewer count | Anchor dispatchable mapping | Anchor unavailable mapping |
   |---|---|---|
   | 2 | Anchor Reviewer + Reviewer-C (Tier 3) | Reviewer-B (Tier 2) + Reviewer-C (Tier 3) |
   | 3 | Anchor Reviewer + Reviewer-A (Tier 1) + Reviewer-C (Tier 3) | Reviewer-A (Tier 1) + Reviewer-B (Tier 2) + Reviewer-C (Tier 3) |
   | 4 (default with anchor) | Anchor Reviewer + Reviewer-A (Tier 1) + Reviewer-B (Tier 2) + Reviewer-C (Tier 3) | Reviewer-A (Tier 1) + Reviewer-B (Tier 2) + Reviewer-C (Tier 3) + Reviewer-D (Tier 1/Tier 2 variant) |
   | 5 | Anchor Reviewer + Reviewer-A (Tier 1) + Reviewer-B (Tier 2) + Reviewer-C (Tier 3) + Reviewer-D (Tier 1/Tier 2 variant) | Reviewer-A (Tier 1) + Reviewer-B (Tier 2) + Reviewer-C (Tier 3) + Reviewer-D (Tier 1/Tier 2 variant) + Reviewer-E (Tier 2/Tier 3 variant) |

   Record a declared fallback for the Anchor Reviewer whenever the anchor route is
   configured but unavailable. Select the dispatch mode at this pre-dispatch
   capability probe:

   * When subagent reviewer dispatch is available (`TOOL_OK: subagent-dispatch`),
     set `dispatch_mode: multi-agent`. At least 2 independent reviewer instances
     must return valid structured results; see Phases 2 and 3 for the
     `INSUFFICIENT_REVIEWERS` halt. Never downgrade a partial or failed
     multi-agent run into `single-agent-declared-degradation` after dispatch
     begins.
   * When cross-model / multi-agent reviewer dispatch is unavailable at this probe
     or restricted by runtime privacy / no-third-party-transmission policy
     (P-012 item 5), log `TOOL_DEGRADED: subagent-dispatch — declared fallback:
     single-agent-declared-degradation`, set
     `dispatch_mode: single-agent-declared-degradation` and
     `consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, and run the full
     multi-persona review rubric in-session without external model transmission.
4. Apply anchor and alternate model provider assignments:
   * Read `openai`, `gpt-5.6-sol`, and
     `high` (or `anchor_provider`,
     `anchor_family`, and `anchor_reasoning_effort` input overrides). If provider
     and family are non-empty and dispatchable, launch the Anchor Reviewer as a
     separate slot, pass the reasoning effort when non-empty, and identify it
     separately in the report.
   * Read `` and `` (or `alt_provider`
     / `alt_family` input overrides).
   * If both alternate values are non-empty: replace one non-anchor reviewer slot
     with the alternate provider. Replace Reviewer-B (Tier 2 slot) by default to
     maximize diversity while preserving Anchor, Tier 1, and Tier 3 coverage.
   * Log the model route assignment table (see below), including declared fallback
     if the Anchor Reviewer or alternate provider is unavailable.
5. Confirm with the operator if the review is interactive mode.

#### Model Route Assignment Table

| Reviewer | Default Route | Default Model | With Alternate Provider |
|---|---|---|---|
| Anchor Reviewer | Anchor review route | `gpt-5.6-sol` via `openai` with `high` when non-empty | unchanged; if unavailable, declared fallback |
| Reviewer-A | Tier 1 (fast/cheap) | `claude-haiku-4.5` | unchanged |
| Reviewer-B | Tier 2 (standard) | `claude-sonnet-5` | `` via `` |
| Reviewer-C | Tier 3 (frontier) | `claude-opus-4.8` | unchanged |
| Reviewer-D | Tier 1 or Tier 2 variant | different from A/B/C | unchanged |
| Reviewer-E (5 reviewers without anchor) | Tier 2 or Tier 3 variant | different from A/B/C/D | unchanged |

When `openai` or `gpt-5.6-sol` is empty or the
route cannot be dispatched, all standard reviewer slots still use tier routing and
the report records the Anchor Reviewer declared fallback. When
`` is non-empty, Reviewer-B is routed to the alternate
provider. This ensures reviewer diversity is not limited to a single provider's
model family while keeping the Anchor Reviewer separate.

### Phase 2: Parallel Dispatch

In `dispatch_mode: single-agent-declared-degradation`, skip subagent dispatch: run
the full multi-persona rubric (Constitution, Go, Correctness, Maintainability,
Security, Concurrency, Scope Boundary, Learnings) in-session over the same file
list and ruleset, produce findings in the JSON shape below, and continue to
Phase 3. The remainder of this phase describes `dispatch_mode: multi-agent`.

Launch all reviewer agents **simultaneously** as parallel subagents. Each receives:

* The same file list or diff
* The same ruleset
* Its assigned model tier or Anchor Reviewer route instruction (prepend to the reviewer's system prompt:
  "You are operating as the Anchor Reviewer or Tier N reviewer. Use concise, precise findings only.")
* Instruction to return **structured JSON findings only** — no prose summaries

Each reviewer produces a JSON array of findings:

```json
[
  {
    "severity": "CRITICAL|MAJOR|MINOR",
    "rule": "Rule number and name",
    "file": "path/to/file",
    "line": 42,
    "issue": "Precise description of what is wrong",
    "fix": "What the correct value or behavior should be"
  }
]
```

Wait for each reviewer agent to settle (return valid JSON, error, or exceed a
bounded per-reviewer settlement timeout) before proceeding to Phase 3; do not wait
indefinitely on a single reviewer. Define `active_reviewers` as the count of
independent reviewer instances that successfully returned valid JSON findings.

* If `active_reviewers < 2`, halt immediately with `INSUFFICIENT_REVIEWERS`
  (`BLOCKED`). Never count a single reviewer as consensus and never downgrade to
  `single-agent-declared-degradation` mid-run.
* If `2 <= active_reviewers < reviewers` (partial pool degradation), log
  `REVIEWER_POOL_DEGRADED: {active_reviewers}/{reviewers} reviewers returned` and
  continue, computing Phase 3 thresholds against `active_reviewers`.

### Phase 3: Aggregate and Classify

Collect all finding arrays. For each unique finding (keyed by `file` + `line` + `rule`):

In `dispatch_mode: single-agent-declared-degradation`, there is no multi-reviewer
agreement to count: assign `confidence_tier: DEGRADED_SINGLE_AGENT` to every
finding and skip the tier and section assignment below.

1. Count how many reviewers flagged it (using fuzzy match on `file` + `line` ± 2 + same `rule`).
2. Assign confidence tier and report section so every agreement count from 1 to
   `active_reviewers` is classified (`active_reviewers` equals `reviewers` when the
   full pool returned, so unanimous agreement across all `active_reviewers` is
   HIGH / Consensus even under `REVIEWER_POOL_DEGRADED`):
   * **HIGH / Consensus section**: Flagged by all `active_reviewers` agents.
   * **MEDIUM / Majority section**: Flagged by a strict majority (> active_reviewers / 2).
   * **MEDIUM / Plurality section**: Flagged by more than one reviewer but not a
     strict majority (for example, 2 of 4 reviewers).
   * **LOW / Unique section**: Flagged by exactly one reviewer.
3. For severity conflicts between reviewers, take the most conservative (highest severity).
4. Map severities deterministically: `CRITICAL -> P0`, `MAJOR -> P1`,
   `MINOR -> P2`.

### Phase 4: Order and Score

Compute a priority score for each finding:

```text
priority = confidence_weight × severity_weight
confidence_weight: HIGH=3, MEDIUM=2, LOW=1
severity_weight: CRITICAL(P0)=4, MAJOR(P1)=3, MINOR(P2)=2
```

In `dispatch_mode: single-agent-declared-degradation`, use severity-only scoring:
`confidence_tier: DEGRADED_SINGLE_AGENT`, `confidence_weight = 1.0`, so the score
is ordered by `severity_weight` alone. Never claim HIGH, MEDIUM, or LOW
multi-reviewer consensus confidence.

Sort all findings descending by priority score. Within the same score, order by
file path for deterministic output.

### Phase 5: Route to Action Classes

In `dispatch_mode: multi-agent`, route by `confidence_tier × severity`:

| Finding | Action Class |
|---|---|
| HIGH confidence + CRITICAL severity | `safe_auto` (if deterministic fix exists) or `manual` |
| HIGH confidence + MAJOR severity | `gated_auto` or `manual` |
| MEDIUM confidence + CRITICAL/MAJOR (majority or plurality) | `gated_auto` — confirm before applying |
| LOW confidence + CRITICAL | `gated_auto` — unusual enough to flag despite single source |
| LOW confidence + MAJOR (P1) | `gated_auto` when `REVIEWER_POOL_DEGRADED` is active (gate-blocking); `advisory` in full-pool mode (`active_reviewers == reviewers`) |
| Any + MINOR | `advisory` |

In `dispatch_mode: single-agent-declared-degradation`, route by severity only:

| Finding | Action Class |
|---|---|
| P0 (CRITICAL) | `safe_auto` when the fix is deterministic and in scope, otherwise `manual` |
| P1 (MAJOR) | `safe_auto` when the fix is mechanical and in scope, otherwise `gated_auto` or `manual` |
| P2 (MINOR) | `gated_auto` or `advisory` |

An action class of `safe_auto` records eligibility only. Edits are applied under
the `mode: report-only` and `authorized_contract_scope` rules in Phase 6.

### Phase 6: Produce Output

In `dispatch_mode: multi-agent`, assemble the output report with all five
finding/remediation sections (consensus, majority, plurality, unique, remediation
plan). In `dispatch_mode: single-agent-declared-degradation`, emit a single
`### Degraded Single-Agent Findings (Non-Consensus)` section plus the remediation
plan instead of the Consensus / Majority / Plurality / Unique sections, and record
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`. For each P0 and P1
finding, produce a backlog work item entry:

```yaml
type: bug
title: "{rule}: {brief description}"
description: "{issue}"
file: "{file}"
line: {line}
severity: "{severity}"
confidence: "{HIGH|MEDIUM|LOW|DEGRADED_SINGLE_AGENT}"
fix: "{fix}"
linked_review: "{output_file_path}"
```

Set `confidence: "DEGRADED_SINGLE_AGENT"` for entries generated in
`dispatch_mode: single-agent-declared-degradation`.

Write the output report to `docs/closure/{YYYY-MM-DD}-{slug}-adversarial-review.md`.
If in interactive mode, present the consensus findings and remediation plan to the
operator for confirmation before creating any backlog items.

**Read-only enforcement (both dispatch modes)**: When `mode: report-only` is set,
this agent is strictly read-only over repository source and config files. Phase 6
writes only the review report and remediation plan and never applies `safe_auto`
edits directly; Phase 7 is skipped. When `mode` is not `report-only`, an explicit
`authorized_contract_scope` input is required to evaluate P-021 C1 before any
`safe_auto` fix is applied. When `authorized_contract_scope` is absent or
ambiguous, fail closed to `mode: report-only` (read-only report and remediation
plan, no automatic file edits, no Phase 7).

Apply `safe_auto` fixes from the remediation plan and proceed to Phase 7 ONLY when
`mode` is not `report-only` AND `post_remediation_review` is `true` AND
`authorized_contract_scope` is explicitly provided AND each `safe_auto` candidate
passes the P-021 C1 same-contract-surface check.

### Phase 7: Post-Remediation Re-Review

When `mode: report-only` is set, or `authorized_contract_scope` is absent or
ambiguous (fail closed to `mode: report-only`), do not apply fixes or run
re-review. Record `post_remediation.status: "skipped"` with reason
`"mode: report-only (caller owns P-021 C1 scope classification, fix application, and re-review)"`
and finish.

Otherwise, after `safe_auto` fixes are applied in Phase 6, verify no new issues
were introduced. This phase prevents a fix in one location from inadvertently
breaking a related invariant.

* `dispatch_mode: multi-agent`: re-dispatch the same reviewer pool over the fixed
  files.
* `dispatch_mode: single-agent-declared-degradation`: run an in-session
  severity-gated re-review over only the files modified by `safe_auto` fixes,
  without subagent pool redispatch. Evaluate residual P0/P1 findings under
  `confidence_tier: DEGRADED_SINGLE_AGENT`, and record any findings remaining
  after the 2-cycle cap as `post_remediation_residual: true` under
  `DEGRADED_SINGLE_AGENT`.

**Recursion cap**: Maximum 2 re-review cycles (Phase 7 executes at most twice per
invocation). Track the cycle count. When the cap is reached, note any remaining
findings in the output report and halt the re-review loop — do not continue.

**Re-review protocol**:

1. Identify files modified by `safe_auto` fixes in the previous cycle.
2. Re-review those files only (not the full original scope): re-dispatch all
   reviewer agents in `multi-agent` mode, or re-run the in-session rubric in
   `single-agent-declared-degradation` mode.
3. Aggregate and classify new findings per Phases 3–5.
4. If new gate-blocking findings are introduced: in `multi-agent` mode, any new
   `HIGH`-confidence `P0`/`P1` (`CRITICAL`/`MAJOR`) finding, any new `P0`
   (`CRITICAL`) finding regardless of confidence (`HIGH`/`MEDIUM`/`LOW`), or,
   when `REVIEWER_POOL_DEGRADED` is active, any new `P0`/`P1`
   (`CRITICAL`/`MAJOR`) finding regardless of confidence
   (`HIGH`/`MEDIUM`/`LOW`); in `single-agent-declared-degradation`, any new
   P0/P1 finding. Add them to the remediation plan,
   apply any new `safe_auto` entries, and increment the cycle counter.
5. If the cycle counter reaches 2 and findings remain: record them as
   `post_remediation_residual` in the output report. Do not apply further fixes.
6. If no new findings: mark the post-remediation phase clean and finish.

**Cycle tracking** in the output report:

```yaml
post_remediation:
  cycles_run: {0|1|2}
  cap_reached: {true|false}
  residual_findings: {count}
  status: "clean|residual_capped|skipped"
```

## Subagent Depth

Maximum 2 hops. This agent dispatches review skill instances (hop 1), which may invoke
review persona subagents (hop 2). The consensus-assembly phase and post-remediation
re-review loop run in this agent — no further delegation.

## Quality Criteria

* In `dispatch_mode: multi-agent`, every dispatched reviewer must settle (valid
  JSON, error, or bounded per-reviewer timeout) before Phase 3 begins. Wait is
  bounded, never indefinite. `active_reviewers` counts reviewers that returned
  valid JSON; consensus thresholds are computed against `active_reviewers`
* In `dispatch_mode: multi-agent`, `active_reviewers >= 2` is required. When
  `active_reviewers < 2`, halt with `INSUFFICIENT_REVIEWERS` (`BLOCKED`); when
  `2 <= active_reviewers < reviewers`, log `REVIEWER_POOL_DEGRADED` and continue
* Reviewer severities map `CRITICAL -> P0`, `MAJOR -> P1`, `MINOR -> P2`
* In `dispatch_mode: multi-agent`, the output must include all three confidence
  tiers (never drop LOW findings) and the Consensus / Majority / Plurality /
  Unique sections
* In `dispatch_mode: single-agent-declared-degradation`, scoring and routing are
  severity-only under `confidence_tier: DEGRADED_SINGLE_AGENT`, the output uses the
  single `### Degraded Single-Agent Findings (Non-Consensus)` section, and no
  multi-reviewer consensus is claimed
* Every P0 finding, regardless of confidence, must appear in the remediation plan
* The output file must be written even if all findings are advisory
* If fewer than 2 reviewer instances return results in `dispatch_mode: multi-agent`,
  halt and report the failure
* When `mode: report-only` is set (in either dispatch mode), or when
  `authorized_contract_scope` is absent or ambiguous, the agent is read-only over
  repository source and config files, applies no `safe_auto` edits, and records
  `post_remediation.status: "skipped"`
* Post-remediation re-review runs only when `mode` is not `report-only`,
  `post_remediation_review` is `true`, `authorized_contract_scope` is provided,
  and `safe_auto` fixes passing the P-021 C1 check were applied; it is skipped
  when no fixes were made. In `single-agent-declared-degradation` it runs
  in-session without subagent redispatch
* The recursion cap of 2 cycles is enforced — the agent MUST NOT recurse more than
  twice regardless of remaining findings
* When `openai` is set, the Anchor Reviewer must use that
  provider's model family unless the provider is unavailable; if unavailable, log
  the declared fallback separately and continue only when consensus minimums hold
* When `` is set, at least one non-anchor reviewer must use the alternate
  provider; failure to route when the provider and family are both configured and
  reachable is a configuration error; if the provider is unreachable at runtime,
  fall back to the Tier 2 standard model, log the fallback, and continue

Generated by autoharness | Template: adversarial-review.agent.md.tmpl
