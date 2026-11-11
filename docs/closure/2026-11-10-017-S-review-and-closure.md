---
title: shipment-reconcile deliberation disposition and headed browser sandbox opt-in review and operational closure
date: 2026-11-10
feature: 017-F
shipment: 017-S
branch: feat/017-s-p-021-deferred-scope-expansion-reconciliation-shipment-reconcile-deliberation-disposition-and-headed-browser-sandbox-opt-in
status: ready
closure_status: READY
compaction_status: done
merge_commit: 03ba97336d9f16b992d4675fae287ed95730b878
pr: 42
---

## Release scope

Shipment `017-S` (`017-F`, `017.001-T`, `017.002-T`) resolves two P-021 C2
deferred scope expansion entries (`1B03802C` and `FC763C17`) planned under
deliberation `006-DL`.

* `017.001-T` (`.github/skills/shipment-reconcile/SKILL.md`,
  `.github/agents/_ship.agent.md`): the cascade close now follows the backlogit
  1.12 INV-12 linked-deliberation dispositions.
  * Step 0(c) records each validated linked deliberation's link kinds
    (`source_deliberation_id`, `description`, `references`) and its pre-close
    disposition, using the precedence in
    `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`:
    `retained_read_error`, `retained_ambiguous`, `already-archived`,
    `retained_engine_unverified`, `retained_live_status`,
    `retained_shared_reference`, `retained_description_mention`, `archive`.
    A deliberation linked only through description or references text resolves to
    `retained_description_mention`. An undeterminable disposition fails closed to
    retain.
  * Cascade Close step 3: only `archive` (`expected_cascade`) deliberations enter
    `allowed_ids` and `required_ids`; `already-archived` ones are tolerated in
    `allowed_ids`; every retained disposition is excluded from both sets. A new
    pre/post retained-deliberation preservation check halts with
    `HALT — cascade archived retained deliberation {id}, revert required` and runs
    the Safe-Close step 6 `git-revert-on-cascade` recovery.
  * Cascade Close steps 5 and 6 report and gate on the retained set, and a new
    step 7 performs post-cascade cleanup: it archives only a
    `retained_description_mention` deliberation with
    `status: queued` and zero remaining live references, and never archives any
    other retained disposition (an `archive` deliberation is required in
    `archived_ids` by step 3 and halts closure if absent). The Quality Criteria
    bullets match.
  * `_ship.agent.md` Post-Merge Closure step 7 replaces the unconditional
    deliberation archive with the same disposition-aware rules.
* `017.002-T` (`internal/browser/recap.go`, `internal/browser/recap_test.go`): the
  direct-exec browser launcher now honors the same sandbox opt-in as the allocator
  flags. `cleanBrowserArgsForOptions(port, profileDir, opts, initialURL)` appends
  `--no-sandbox` and `--disable-setuid-sandbox` before the trailing `initialURL`
  only when `shouldDisableSandbox(opts)` is true (`Options.DisableSandbox`, or
  `TEAMS_DISABLE_SANDBOX` set to `1` or `true`, case-insensitive). The sandbox
  stays enabled by default. `cleanBrowserArgs` and `cleanHeadedBrowserArgs` keep
  their signatures and route through it, and `launchCleanHeadedBrowser` passes its
  full `Options`.

Source stash entries `1B03802C` and `FC763C17` fed this scope under deliberation
`006-DL`. Excluded from this shipment and untouched: shipment `010-S` and stash
entries `680A62E6` and `2D0BAA19`.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`): the multi-persona review
rubric (Constitution, Go, Correctness, Maintainability, Security, Concurrency,
Scope Boundary, Learnings, Template Integrity, Schema-CLI-Docs Coupling) ran
in-session against each task diff, without transmitting repository content to an
external reviewer model.

| Area | Finding | Disposition |
|---|---|---|
| Schema-CLI-Docs Coupling | The skill's Step 0(c) paragraph still said a qualifying feature's live linked deliberation "archives it during the same cascade invocation", which contradicts the 1.12 retention behavior | Fixed in-scope (P-021 C1: completes the authorized Step 0(c) reconciliation); reworded to "cascade-archive candidate, subject to the retention dispositions" |
| Template Integrity | New cross-references (`Cascade Close step 7`, `Safe-Close Mode step 6`, `git-revert-on-cascade`) resolve to existing or newly added headings and steps; no placeholder or frontmatter change | No action |
| Correctness | Retained deliberations are excluded from `allowed_ids`, so an engine that archives one trips the unexpected-artifact check as well as the preservation check; the checks stay separate conditions | No action |
| Correctness | `already-archived` deliberations remain tolerated in `allowed_ids` and absent from `required_ids`, preserving the 147-F / 027-DL behavior | No action |
| Security | Sandbox stays on by default; the opt-in is the existing `shouldDisableSandbox` contract, not a new env parse. `AutomationControlled` remains absent | No action |
| Correctness | Test default cases set `TEAMS_DISABLE_SANDBOX` to empty so an ambient value cannot flip them | No action |
| Scope | Changes stay inside the four authorized files plus `017-S` backlog, closure, and memory docs; no `010-S` or stash entry touched | No action |
| Correctness | Copilot round 2 (thread `PRRT_kwDOTStvas62TdA7`): the closure doc predicted `006-DL` would resolve to `retained_description_mention`, but `017-F` sets `source_deliberation_id: 006-DL`, so that link kind is present and the prediction was wrong | Fixed in-scope (P-021 C1: corrects this shipment's own closure record); the Releasability row no longer predicts a disposition and states the description-only branch has no live fixture here |
| Maintainability | One new unexported constructor; `cleanBrowserArgs` and `cleanHeadedBrowserArgs` are thin wrappers, so there is one place that builds the arguments | No action |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62TTB_`): the Ship Step 7 deliberation cleanup only enumerated `source_deliberation_id`, so a description-only deliberation (the `retained_description_mention` case) was never considered, notably on the safe-close path | Fixed in-scope (P-021 C1: completes the authorized Step 7 reconciliation); Step 7 now enumerates the Step 0(c) snapshot set or the same validated source/description/references discovery |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62TTCh`): the preservation check required retained deliberations to sit in `queue/`, rejecting a valid pre-close `done` record in `archive/` | Fixed in-scope (P-021 C1: same sentence of the authorized check); the check and report now use the snapshotted location and declared status, with Step 7 wording aligned |
| Correctness | Copilot round 1 (thread `PRRT_kwDOTStvas62TTDQ`): the `archive`-disposition fallback in Cascade Close step 7 was unreachable because step 3 requires it in `archived_ids` and step 6 blocks step 7 otherwise | Fixed in-scope (P-021 C1: removes an unreachable branch of the authorized step); strict halt preserved, step 7 now handles only `retained_description_mention`; `_ship.agent.md` keeps the `archive` archive call only for the safe-close path |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

## Follow-ups

* Stash entry `9A9D8106` (low): deliberation `006-DL` stays queued in
  `.backlogit/queue/` with no live references (see Post-merge closure). Decide
  whether to archive it with `backlogit archive 006-DL`, and whether the
  autoharness plugin's `VERIFIED_CASCADE_ENGINE_MINOR_LINES` (currently only
  `1.11`) should add `1.12`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Red then green | `TestCleanBrowserArgsHonorsDisableSandboxOptIn` failed against a compiling stub that never emitted the sandbox flags and passed after implementation | PASS |
| Format | `gofmt -l .` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` (all packages) | PASS |
| Build | `go build ./...` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` ("No issues found.") | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Change how the browser launcher decides to pass `--no-sandbox` | medium | Default behavior unchanged (sandbox on); opt-in only through the documented option or `TEAMS_DISABLE_SANDBOX` | applied |
| Change which linked deliberations the cascade close may archive | medium | Fails closed to retain; preservation check halts and reverts on any retained deliberation archived | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Browser launcher default arguments | READY | Same argument list as before when no opt-in is set |
| Browser launcher sandbox opt-in | READY | Operator sets `TEAMS_DISABLE_SANDBOX=1` or `Options.DisableSandbox` in a container that cannot run the sandbox |
| Cascade-close specification | READY_WITH_CONDITIONS | First live exercise is this shipment's own closure. `017-F` carries `source_deliberation_id: 006-DL`, so `006-DL` is not a description-only link; its disposition is whatever Step 0(c) computes at closure (planned `archive` unless a live external reference or live status retains it) and is recorded in the post-merge section. The `retained_description_mention` cleanup branch (step 7) has no live fixture in this shipment and is validated by specification review only |

## Monitoring and observation window

Owner: operator merging to `main`.
Duration: the next headed `teams-cli` recap launch and the next shipment closure.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Headed browser starts with sandbox on | Browser process command line | No `--no-sandbox` | `--no-sandbox` present without the opt-in |
| Retained deliberations survive a cascade | `.backlogit/queue/` after `backlogit shipment ship` | Retained records still present | An unexpected-cascade halt |

## Rollback

Revert the `017-S` merge commit on `main`. The code change only affects argument
construction; the specification change only affects Ship and `shipment-reconcile`
prose. No data migration is involved.

## Post-merge closure

PR #42 merged to `main` at `03ba97336d9f16b992d4675fae287ed95730b878`
(`2026-11-11T18:24:51Z`) with a merge commit (P-009). Dark Factory Mode (P-017)
post-merge closure ran on `post-merge/017-reconcile-deliberation-and-headed-sandbox`:

* Copilot shadow review raised four threads over two rounds on PR #42, all fixed in
  scope under P-021 C1 (three in `decaf3f` for the deliberation rules, one in
  `49815f4` for this document's disposition prediction), replied to, and resolved.
  No thread was out of scope, so no deferred scope expansion entry was captured.
  `autoharness gate copilot-review` returned `SATISFIED` for the merged HEAD
  `49815f4510a30bbd817141e3b4ee37330682c639`, CI passed, and the merge used a merge
  commit with no admin fallback.
* `MERGE_CONFIRMED`: `gh pr view 42` returned `state: MERGED`, and
  `git merge-base --is-ancestor 03ba97336d9f16b992d4675fae287ed95730b878
  origin/main` exited 0.
* `autoharness gate pipeline-topology --phase lifecycle --shipment 017-S` passed on
  the closure branch.
* Pre-close reconciliation: `017-F`, `017.001-T`, and `017.002-T` were already
  archived with `status: done`, no orphan declared `shipment_id: 017-S`, and the
  shipment record was `active`.
* P-015 fully-covered-root verification: `017-F` is a root feature whose only
  descendants, `017.001-T` and `017.002-T`, are both `017-S` manifest members, so the
  classification was `CASCADE`. Step 0(c) linked-deliberation disposition, computed
  with `compute_linked_deliberation_disposition` (engine `1.12.0` is not in
  `VERIFIED_CASCADE_ENGINE_MINOR_LINES`), classified `006-DL` (link kinds
  `source_deliberation_id` and `description`, status `queued`, no live referrer) as
  `retained_engine_unverified`. It was therefore excluded from `allowed_ids` and
  `required_ids`. `backlogit shipment ship 017-S --sha
  03ba97336d9f16b992d4675fae287ed95730b878` returned `archived_ids: [017.001-T,
  017.002-T, 017-F, 017-S]` and `returned_ids: []`. Both set differences were empty,
  both tasks kept `parent_id: 017-F`, `017-S` carries `archived_status: shipped`,
  `006-DL` was still in `.backlogit/queue/` with `status: queued` (retained-deliberation
  preservation check passed), and no archive file showed as a working-tree deletion
  (P-007).
* Source artifact cleanup: stash entries `1B03802C` and `FC763C17` were already
  archived by Stage (skipped). Deliberation `006-DL` was **retained, not archived**:
  its disposition is `retained_engine_unverified`, which the reconciled contract never
  archives. The handoff predicted `retained_description_mention`, but `017-F` sets
  `source_deliberation_id: 006-DL`, so that link kind is present, and the engine is
  unverified in any case. Follow-up stash entry `9A9D8106` records the decision.
  Shipment `010-S` and stash entries `680A62E6` and `2D0BAA19` were not touched.
* Mandatory P-020 `compact-context` (`target: all`) ran: the session memory was
  compacted to
  `docs/memory/compacted/2026-11-10-017-S-reconcile-deliberation-and-headed-sandbox-compacted.md`
  and the verbose original archived under `docs/archive/memory/2026-11-10/`.
  Recorded as `compaction_status: done` above.

## Handoff

Related [decision](../decisions/2026-11-10-reconcile-deliberation-and-headed-sandbox-deliberation.md)
and [plan](../plans/2026-11-10-reconcile-deliberation-and-headed-sandbox-plan.md).
