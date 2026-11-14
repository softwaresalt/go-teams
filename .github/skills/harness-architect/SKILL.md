---
name: harness-architect
description: "Scaffolds compilable but failing test harnesses for feature and chore tasks"
argument-hint: "feature=001-{SUFFIX_FEATURE} tasks=001.001-{SUFFIX_TASK},001.002-{SUFFIX_TASK}"
input:
  properties:
    feature:
      type: string
      description: "Feature or chore ID to scaffold harnesses for"
    tasks:
      type: string
      description: "Comma-separated task IDs to scaffold (optional, defaults to all ready tasks)"
  required:
    - feature
---

# Harness Architect Skill

Scaffold strict test harnesses for a feature's or chore's ready work items.
The output must compile cleanly, fail for the intended not-yet-implemented
behavior, and leave clear harness commands for downstream build execution.

## Purpose

Use this skill when a release unit needs executable test boundaries before
implementation starts. The skill prepares the red phase and stops there. It
does not implement production logic.

## Agent-Intercom Communication

When the `agent-intercom` capability pack is installed, call `ping` at
session start. If reachable, broadcast at every step. If unreachable,
warn the operator that visibility is degraded and continue locally.

| Event | Level | Message prefix |
|---|---|---|
| Session start | info | `[HARNESS] Starting: feature={input.feature}` |
| Tasks loaded | info | `[HARNESS] Ready tasks: {task_count}` |
| Codebase analyzed | info | `[HARNESS] Context gathered: {module_count} modules` |
| Harness generated | info | `[HARNESS] Generated: {test_file} ({scenario_count} scenarios)` |
| Compilation check | info | `[HARNESS] Compilation: {result}` |
| Red phase check | info | `[HARNESS] Red phase: {result}` |
| Label applied | success | `[HARNESS] harness-ready: {task_id}` |
| Complete | success | `[HARNESS] Complete: {task_count} tasks harnessed` |

## Inputs

* `${input:feature}`: (Required) Feature or chore ID such as `001-F`
* `${input:tasks}`: (Optional) Comma-separated task IDs to scaffold.
  When omitted, use all executable (`queued` or `active`) tasks under the feature.

## Workflow

### Step 1: Claim the target task set

1. Load the feature or chore and its descendant tasks (`queued` or `active`) through backlog
   query or queue operations.
2. If `${input:tasks}` is present, restrict the scope to that explicit
   task set.
3. Exclude tasks whose status is `blocked`, `done`, or `archived`. Do **not** exclude a `queued` or `active` task merely because it has an intra-feature / intra-shipment `blocks` dependency on another task in the same batch: instead, sort the batch in topological dependency order so upstream structural stubs are created first, allowing downstream test harnesses in the same batch to compile against those stubs in the red phase. (Dependencies between tasks in the same feature govern the subsequent `build-feature` implementation order, not up-front harness generation.)
4. Preserve the work-item-to-task mapping so each harness can be traced
   back to the correct backlog item.

### Step 2: Read task intent

1. Read each selected task's title, description, acceptance criteria,
   and file references.
2. Pull in feature-level acceptance criteria when task text depends on
   broader feature behavior.
3. Translate acceptance criteria into named test scenarios before writing
   code.
4. Identify the correct module, test tier, and affected files for each
   harness.

When the `agent-engram` capability pack is installed, prefer indexed
symbol lookup and code-graph tools over broad grep when surveying
existing modules, test patterns, and import paths.

### Step 3: Determine execution posture

For each task, select the appropriate harness strategy:

| Posture | When to use | Harness pattern |
|---|---|---|
| **test-first** | New functionality with clear inputs/outputs | Write failing tests for expected behavior |
| **characterization-first** | Modifying existing behavior | Write tests that capture current behavior then modify |
| **migration-first** | Moving code between modules | Write tests at the destination, verify source behavior |
| **spike** | Exploratory with uncertain approach | Write minimal integration test, implement spike, expand tests |

### Step 4: Generate failing harness skeletons

1. Create test files that express the task intent as compilable tests.
2. Prefer table-driven or parameterized tests when the task describes
   multiple scenarios.
3. Create matching production stubs with error-returning or zero-value bodies (e.g., `return fmt.Errorf("not implemented: ...")`, never `panic()` per Go safety rules)
   so the module compiles while the tests still fail for the intended
   reason.
4. Keep signatures, types, and module names aligned with the current
   codebase.
5. **Docs-only or config-only verification boundary**: When a task modifies only documentation, markdown, or static configuration examples with no production Go code or test symbols to stub, do not invent artificial Go stubs; instead record the task's verification boundary (e.g., `go vet ./...` or tracked-file hygiene test) as its `harness_cmd`.

### File placement rules

Write harness files into the module that matches the work item's scope:

* **Unit harnesses**: colocated with the production code in the
  appropriate internal/ subdirectory
* **Integration harnesses**: in `co-located *_test.go files/integration/` when the
  task spans modules or runtime boundaries
* **Contract harnesses**: in `co-located *_test.go files/contract/` when the task
  defines API, CLI, or schema behavior

Write companion stub files into the production module that the tests
exercise. Do not place scaffolding in unrelated modules.

### Step 5: Verify harness

#### Step 5.1: Compilation check

Run `go vet ./...` including tests. The harness MUST compile.

If compilation fails, fix the harness until it compiles. Do not proceed
with a non-compiling harness.

#### Step 5.2: Red phase check

For code and test tasks, run the task-scoped `go test` command (`harness_cmd`) for each scaffolded task. ALL newly scaffolded harness tests MUST fail with
the expected failure marker (not-implemented error or failing test assertion). For docs-only or config-only tasks with no Go code/test surface, verify the recorded verification boundary (`go vet ./...`) passes and record `Red Phase: N/A (docs/config-only verification boundary)`.

If any code/test harness passes (false positive) or fails with an unexpected error
(compilation vs runtime), fix the harness.

### Step 6: Apply harness-ready label

After both checks pass (P-004 gate satisfied):

1. Update each task with `harness-ready` label using the backlog tool's
   update operation.
2. Add an implementation note with the task-scoped `harness_cmd` (so `build-feature` and intermediate task verification run that task's test target while downstream tasks' red harnesses remain pending, before the unfiltered full-suite `go test ./...` runs on the final task).
3. Record the harness manifest: `Compilation: PASS`,
   `Red Phase: CONFIRMED` (or `Red Phase: N/A (docs/config-only verification boundary)`).

## Completion Criteria

The skill is complete only when the selected tasks have:

* harness files in the correct modules (or a recorded verification boundary for docs/config-only tasks)
* structural stubs with intentional not-implemented behavior for all code/test tasks
* a successful `go vet ./...` result after scaffolding
* all code/test harness tests failing with the expected marker
* clear mapping from backlog task to its task-scoped `harness_cmd`

## Guardrails

* Do not implement production logic — stubs only.
* Do not skip compilation verification.
* Do not apply the harness-ready label until compilation and the red
  phase check (or documented docs/config-only verification boundary) pass.
* Keep test scenarios traceable to acceptance criteria.

## Model Routing

This skill operates at **Tier 2 (Standard)** — test scaffolding is structured but routine.
