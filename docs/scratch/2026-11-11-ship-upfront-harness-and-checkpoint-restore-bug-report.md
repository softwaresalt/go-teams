---
title: "Bug Report: Ship Up-Front Harness Generation Deadlock on Intra-Feature Dependent Tasks and Checkpoint Prune-on-Restore External-Memory Fail-Closed Contradiction"
date: 2026-11-11
status: open
severity: high
affected_surfaces:
  - ".github/agents/_ship.agent.md (Step 2: Harness Generation; Crash-Resumption / Startup Recovery Protocol)"
  - ".github/agents/_stage.agent.md (Crash-Resumption / Startup Recovery Protocol)"
  - ".github/skills/harness-architect/SKILL.md (Inputs; Step 1: Claim the ready task set)"
  - ".github/instructions/backlogit.instructions.md (Checkpoint-Recovery / Prune-on-Restore Protocol)"
---

## Summary

Two interrelated workflow contradictions in the Ship agent, the `harness-architect` skill, and the backlog overlay's Checkpoint-Recovery / Prune-on-Restore protocol cause deterministic pipeline halts whenever a staged shipment contains tasks with intra-feature `blocks` dependencies:

1. **Bug 1 (Primary Deadlock) — Up-Front Step 2 Harness Generation vs. Intra-Feature Task Dependencies (`_ship.agent.md` Step 2 + `harness-architect/SKILL.md` Step 1)**:
   * `_ship.agent.md` Step 2 runs **once, up front** (not in a per-task loop) and requires **every** task in the target feature to carry the `harness-ready` label before Step 3 / Step 4 implementation begins, halting if any task lacks `harness-ready`.
   * Simultaneously, `harness-architect/SKILL.md` Step 1 Item 3 instructs the agent to *"Exclude blocked, done, or otherwise non-ready work items."*
   * Whenever Stage decomposes a feature into two or more tasks connected by a `blocks` dependency edge (`Task 2 --depends_on--> Task 1`), `Task 2` has an unfinished dependency at Step 2 time. `harness-architect` therefore excludes `Task 2` as "non-ready", leaving `Task 2` without the `harness-ready` label, which immediately forces `_ship.agent.md` Step 2 Item 4 to halt before a single task can be implemented.
   * Compounding this, `_ship.agent.md` Step 2 Items 1 and 4 filter for `queued` tasks even though claiming the shipment in Step 0.5 (`shipment claim`) already transitioned all manifest tasks from `queued` to `active`.

2. **Bug 2 (Secondary Recovery Deadlock) — Checkpoint Prune-on-Restore Fail-Closed Requirement on External Memory Daemon Reachability (`backlogit.instructions.md` + `_ship.agent.md` + `_stage.agent.md`)**:
   * When Bug 1 halts `_Ship`, `_Ship` writes a recovery checkpoint to `.backlogit/checkpoints/` (`status: active`).
   * On the next invocation, the Crash-Resumption / Prune-on-Restore protocol requires reading state from the external `agent-engram` daemon if that capability pack is listed in workspace configuration, and mandates **FAIL CLOSED to OPERATOR HANDOFF (no prune, no resume)** if the `engram` daemon is unreachable or times out at restore time.
   * This is self-contradictory: the checkpoint `state_dump` is stored entirely in `.backlogit/checkpoints/*.json` (and `docs/memory/`), normal execution treats an unreachable `engram` daemon as a non-blocking degradation (`TOOL_DEGRADED: agent-engram`), and the very same protocol section states that workspaces without `agent-engram` skip pruning as a *"supported, non-degraded no-op"*.

---

## Detailed Root-Cause Analysis

### Bug 1: Up-Front Harness Generation Deadlock on Dependent Tasks

#### Observed Failure Sequence

1. **Stage** harvests a feature (`018-F`) into two width-isolated tasks:
   * `018.001-T` (`domain: code`): adds new exported error sentinels and locking logic in `internal/updater`.
   * `018.002-T` (`domain: tests`, `dependencies: [018.001-T]`): adds CLI-level lock contention tests in `internal/cli` that reference the new sentinel introduced by `018.001-T`.
   * Stage wires `018.002-T -> 018.001-T` (`blocks`) as required by `_stage.agent.md` Step 5 (*"create explicit dependency edges between tasks that must run in sequence instead of encoding that ordering only in prose"*).
2. **Ship** claims the shipment (`018-S`), which transitions `018-S`, `018-F`, `018.001-T`, and `018.002-T` to `status: active`.
3. **Ship Step 2 (`Harness Generation`)** states:
   > *"Ensure every task in the target feature or chore has a passing test harness before any implementation begins. This step runs once, up front — not in a loop. ... 1. List all tasks for the target feature or chore that are in `queued` status. ... 4. After scaffolding completes, confirm every queued task now carries the `harness-ready` label. If any task still lacks it, halt and report the gap rather than proceeding with a partial set."*
4. **`harness-architect/SKILL.md` Step 1 (`Claim the ready task set`)** states:
   > *"1. Load the feature or chore and its ready descendants through backlog query or queue operations. 2. If `${input:tasks}` is present, restrict the scope to that explicit task set. 3. Exclude blocked, done, or otherwise non-ready work items."*
5. Because `018.001-T` is not yet `done`, `018.002-T` has an unmet `blocks` predecessor. A strict model following `harness-architect` Step 1 Item 3 excludes `018.002-T` as "non-ready", then immediately halts at `_ship.agent.md` Step 2 Item 4 because `018.002-T` lacks `harness-ready`.

#### Why Excluding Intra-Feature Dependent Tasks in Step 2 Is Wrong

* The purpose of `harness-architect` Step 4 is to create **compilable structural stubs** (stub types, functions, and error sentinels returning not-implemented errors) alongside the failing test skeletons.
* When `harness-architect` processes a feature's tasks in **topological dependency order**, scaffolding `Task 1`'s structural stubs first defines the exact exported symbols that `Task 2`'s test harness needs in order to compile (`go vet ./...` passes) and fail in the red phase (`go test ./...` fails on the stub).
* Intra-feature `blocks` dependencies between tasks in the same shipment are meant to govern the **Step 3 / Step 4 `build-feature` implementation order** (implementing `Task 1` to green before implementing `Task 2` to green) — **not** to exclude downstream tasks in the same feature from up-front Step 2 harness scaffolding.
* Additionally, `_ship.agent.md` Step 2 Items 1 and 4 must include both `queued` and `active` tasks (matching `_ship.agent.md` Step 3 Item 1's shipment-manifest executable-set derivation), because claiming a shipment transitions its member tasks from `queued` to `active` before Step 2 runs.

---

### Bug 2: Contradictory `engram`-Reachability Requirement During Checkpoint Prune-on-Restore

#### Observed Failure Sequence

1. After halting on Bug 1, `_Ship` writes a structured checkpoint to `.backlogit/checkpoints/checkpoint-*.json`.
2. When `_Ship` is re-invoked and operator-confirmed to restore that checkpoint, `.github/instructions/backlogit.instructions.md` (**Checkpoint-Recovery / Prune-on-Restore Protocol**, Item 4) and `_ship.agent.md` / `_stage.agent.md` state:
   > *"When the `agent-engram` capability pack IS installed/active, and the bound engram substrate is unreachable when attempting to read state for pruning, FAIL CLOSED to OPERATOR HANDOFF — NO prune and NO resume."*
3. When the local `engram` daemon experiences an IPC readiness timeout, `_Ship` refuses to restore the `backlogit` checkpoint and halts again — leaving the checkpoint unresolved in `.backlogit/checkpoints/` and wedging the pipeline.

#### Why This Requirement Is Contradictory

* `backlogit` checkpoints store their entire `state_dump` JSON in `.backlogit/checkpoints/` (supplemented by markdown checkpoints in `docs/memory/`). `engram` is not the storage substrate for `backlogit` checkpoints.
* During normal startup and execution (Step 0.0 Tool Availability Gate), an unreachable `engram` daemon degrades gracefully (`TOOL_DEGRADED: agent-engram`) to file-based search without halting the pipeline.
* Right above Item 4 in `backlogit.instructions.md`, the Applicability note states that when `agent-engram` is not installed, skipping `engram` during restore (`restore -> resume`) is a *"supported, non-degraded no-op"*.
* Requiring `engram` to be reachable at restore time makes crash recovery fail closed on an optional sidecar that the interrupted session did not even require during normal execution.

---

## Required Source Remediations (for the Harness Template Repository)

### Remediation 1: Update `_ship.agent.md` (Template: `ship.agent.md.tmpl`) — Step 2

In `### Step 2: Harness Generation (P-002 / P-004)`:

1. **Include both `queued` and `active` tasks**: When assembling the target task set for Step 2, include all feature/manifest tasks in `queued` or `active` status (aligning Step 2 with Step 3 Item 1's executable-set derivation after `shipment claim`).
2. **Clarify the role of intra-feature task dependencies**: State explicitly that intra-feature / intra-shipment `blocks` dependencies between tasks in the target batch order **topological stub/harness scaffolding in Step 2** (upstream structural stubs first, downstream test harnesses second) and **Step 3 / Step 4 `build-feature` execution**, and MUST NOT exclude downstream tasks in the same feature/shipment from up-front Step 2 harness generation.
3. **Handle docs-only / config-only tasks**: Note that for docs-only or config-only tasks that have no code/test surface to stub, Step 2 records the verification command and applies `harness-ready` so mixed code+docs features do not stall at Step 2 Item 4.

### Remediation 2: Update `harness-architect/SKILL.md` (Template: `harness-architect/SKILL.md.tmpl`) — Step 1

In `### Step 1: Claim the target task set`:

1. Load descendant tasks in `queued` or `active` status (or the explicit `${input:tasks}` set).
2. Replace *"3. Exclude blocked, done, or otherwise non-ready work items"* with:
   > *"3. Exclude tasks whose status is `blocked`, `done`, or `archived`. Do **not** exclude a `queued` or `active` task merely because it has an intra-feature / intra-shipment `blocks` dependency on another task in the same batch: instead, sort the batch in topological dependency order so upstream structural stubs are created first, allowing downstream test harnesses in the same batch to compile against those stubs in the red phase. (Dependencies between tasks in the same feature govern the subsequent `build-feature` implementation order, not up-front harness generation.)"*

### Remediation 3: Update `backlogit.instructions.md`, `_ship.agent.md`, and `_stage.agent.md` — Checkpoint Prune-on-Restore

In `.github/instructions/backlogit.instructions.md` (`## Checkpoint-Recovery / Prune-on-Restore Protocol`, Applicability and Item 4) and the `OWNER-EXCLUSIVE, OPERATOR-CONFIRMED RESTORE` Step 3 bullet in `_ship.agent.md` and `_stage.agent.md`:

1. Treat `backlogit`'s `state_dump` (and `docs/memory/`) as the authoritative checkpoint substrate, and `agent-engram` as an optional context source when reachable.
2. When `agent-engram` is installed in configuration but unreachable at restore time, log `ENGRAM_DEGRADED` (or `TOOL_DEGRADED: agent-engram`), perform the bounded read-select-summarize step directly over the restored `backlogit` `state_dump` and `docs/memory/` checkpoint (preserving the Prune allowlist: active cursor, unresolved-checkpoint pointer, and gate verdicts), and proceed to resume.
3. Reserve the fail-closed operator handoff strictly for when **`backlogit` itself** (the checkpoint substrate) is unreachable or the checkpoint `state_dump` is unreadable/malformed.
