---
title: "Wave 1 Reconciliation Plan for Stash AD451E00 and Queued Task 010.008-T"
description: "Implementation plan to reconcile queued task 010.008-T onto newMeeting(t) in internal/browser/meeting_integration_test.go within shipment 010-S while preserving the deferred AD451E00 hangup/CDP helper request"
doc_type: "plan"
source: "docs/decisions/2026-11-19-ad451e00-010-008-t-reconciliation-deliberation.md"
plan_id: "2026-11-19-ad451e00-010-008-t-reconciliation"
revision: 2
deliberation_id: "025-DL"
feature: "010-F"
shipment: "010-S"
source_stash:
  - "AD451E00"
date: "2026-11-20"
status: "draft"
tags:
  - "p-021"
  - "browser-verification"
  - "stage-reconciliation"
  - "010-S"
review_manifest: docs/reviews/2026-11-19-ad451e00-010-008-t-reconciliation-plan-review.md
source_history:
  - "review attempt 1: FAIL (plan revision 1) -\u003e docs/reviews/2026-11-19-ad451e00-010-008-t-reconciliation-plan-review.md"
  - "review attempt 2: PASS (plan revision 2) -\u003e docs/reviews/2026-11-19-ad451e00-010-008-t-reconciliation-plan-review.md"
---

## Problem Frame

Following the accepted Wave 1 deliberation in
[2026-11-19-ad451e00-010-008-t-reconciliation-deliberation.md](../decisions/2026-11-19-ad451e00-010-008-t-reconciliation-deliberation.md)
(`025-DL`), this plan governs the same-contract-surface reconciliation of queued
task `010.008-T` (Plan U8 under covering feature `010-F` and queued shipment
`010-S`) before `010-S` executes, while separating and preserving the unrelated
bound-target hangup/CDP helper request in active stash entry `AD451E00`.

In `030.002-T` (commit `b0b67e9`, merged to `main` in PR #74 commit
`29d554b918a55c8b978d3f826c6672d358ef5a8a`), `func browserContext(t *testing.T)
context.Context` and its stale `"already authenticated"` docstring in
`internal/browser/meeting_integration_test.go` were folded directly into `func
newMeeting(t *testing.T) (browser.Meeting, context.Context)` (`HEAD` lines
`32-57` of `147`), which delegates to `browser.NewLiveMeeting(...)` and
currently has no GoDoc or helper comment above line `32` (while line `37`
retains `t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for live
meeting tests")`). Queued task `010.008-T` still instructs Ship to edit the
removed `browserContext(t)` docstring and lacks a `P-004` `harness-surface:*`
label.

### Explicit Scope Boundaries

* **In scope**:
  * Reconciling existing queued task `010.008-T` (inside existing covering
    feature `010-F` and existing queued shipment `010-S`, `harvest_ids =
    ["010.008-T"]`) via separate mutually exclusive `backlogit_update_item`
    calls so its comment-only specification targets `newMeeting(t)` in
    `internal/browser/meeting_integration_test.go`, cites this plan alongside
    `2026-09-05` Plan U8, and declares `harness-surface:none`, `size: XS`
    (`ah-stage-sizing-v1`), and `complexity: trivial`.
  * Creating deliberation `025-DL` via `backlogit_create_item`
    (`artifact_type: "deliberation"`, avoiding `backlogit_deliberate` so dirty
    `.backlogit/stash.jsonl` stays byte-identical), semantic link `025-DL ->
    010.008-T` (`informs`), and traceability comment on `010.008-T`.
* **Out of scope**:
  * Adding any bound-target hangup or CDP control helper on `browser.Meeting` or
    `chromeMeeting` for `TestMeetingLiveScheduledOvertimeAndCallEnd` in
    `internal/browser/scheduled_lifecycle_integration_test.go` (`HEAD` lines
    `106-264`, unattached `streamCtx` hangup `chromedp.Evaluate` block at lines
    `237-242`; preserved in active stash `AD451E00` for a future dedicated
    deliberation and shipment).
  * Creating any new feature, task, subtask, or shipment.
  * Mutating `.backlogit/stash.jsonl` (already dirty at session start with
    pre-existing uncommitted entry `47D1E1C0`), mutating sibling tasks
    `010.001-T`..`010.007-T` or `010.009-T` outside `harvest_ids` without
    explicit operator confirmation, or modifying any Go source, test, or
    configuration file during Stage.

## Requirements Trace

| Requirement (`GRILL_ME_INTENT_CRYSTALLIZED`) | Source Ref | Target Backlog Unit | Verification (`saw_red` -> `green_after_red`) |
|---|---|---|---|
| `D1` / `AC1`: Reconcile `010.008-T` from removed `browserContext(t)` to `newMeeting(t)` in `internal/browser/meeting_integration_test.go` (comment-only GoDoc `// newMeeting ...`) | `AD451E00`, `025-DL` | `U1` (`010.008-T` in `010-F` / `010-S`) | Stage: `backlogit get 010.008-T` confirms `newMeeting(t)` target + `harness-surface:none` + `XS` / `trivial`. Ship (`010-S`): comment-only diff above `newMeeting(t)` + `AC4` compile/vet/lint/fmt/redaction gates |
| `D2` / `D3` / `AC3`: Create `025-DL` via `backlogit_create_item` and preserve `AD451E00` without mutating dirty `.backlogit/stash.jsonl` | `AD451E00`, `P-011`, `P-021` | `025-DL` + untouched `AD451E00` | Pre/post SHA-256 of `.backlogit/stash.jsonl` equals `C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A` |
| `D4`: Declare `harness-surface:none` on `010.008-T` as a non-docs test-only task (no route label) | `P-002`, `P-004` | `U1` (`010.008-T`) | `backlogit get 010.008-T` shows `labels: ["harness-surface:none"]` and explicit non-docs inline-verification posture |
| `D5`: Respect `harvest_ids` repair boundary and surface `010-S` sibling blockers (`DECLARATION_MISSING` + `010.007-T` post-`035-S` drift) | Stage Step 5.3 / 5.5 | Stage report (`010-S` preflight) | `gsd harness resolve --workspace . --shipment 010-S --json` run read-only; both `DECLARATION_MISSING` and `010.007-T` drift surfaced for operator confirmation |

## Implementation Units

### U1 (`010.008-T`, reconciled in place) — Align `newMeeting` live-harness docstring in `internal/browser/meeting_integration_test.go` to confirmed guest and authenticated profile modes

* **Domain:** test file comment-only (`internal/browser/meeting_integration_test.go`).
* **Posture:** documentation comment on a non-docs **`harness-surface:none`**
  test-only task (NOT a `P-002` `docs-only` route candidate because
  `internal/browser/meeting_integration_test.go` is outside `docs/`; under
  `_ship.agent.md` Step 4.0/4.2, Ship skips T2/T3 `harness-architect`, applies
  neither `harness-ready` nor `harness-verification-gated`, and runs the
  recorded verification commands inline with no expected-RED stub roster under
  `P-004` Rule 4).
* **Size:** `XS` (`size_source: agent`, `size_ruleset_version: ah-stage-sizing-v1`).
* **Complexity:** `trivial` (applied in an isolated `complexity: "trivial"`
  `backlogit_update_item` call per `.gsd/backlog-registry.yaml`, and preserved
  as `Size: XS | Complexity: trivial` prose in `implementation-notes` if the
  `task` WIT omits `custom_fields.complexity`).
* **Labels:** `harness-surface:none`.
* **File Budget (1 file):** `internal/browser/meeting_integration_test.go`.
* **What (for Ship execution inside `010-S` after U1 / `010.001-T`):**
  1. Add a concise GoDoc comment starting with `// newMeeting ` using complete
     sentences immediately above `func newMeeting(t *testing.T)
     (browser.Meeting, context.Context)` (`HEAD` line `32` of
     `internal/browser/meeting_integration_test.go`, the direct
     `browser.NewLiveMeeting` replacement into which `030.002-T` commit
     `b0b67e9` folded removed `browserContext(t)`).
  2. State accurately in that GoDoc comment (using only exported `package
     browser` contracts visible from external `package browser_test`, never
     citing unexported internal helpers):
     * `newMeeting` loads unset or empty `TEAMS_*` (and `TEAMS_TEST_*`) test
       variables from gitignored `.env.local.test` via `meetingURL` ->
       `requireIntegration` (`browser.LoadTestEnv`), requires a non-empty
       `TEAMS_PROFILE_DIR` (matching line `37`'s `t.Skip` guard), allocates a
       headless (`--headless=new`) `browser.Meeting` and context via
       `browser.NewLiveMeeting` (reading `TEAMS_BROWSER_PATH` and
       `TEAMS_ACCOUNT`), and registers `t.Cleanup(cleanup)` to reap the browser
       session.
     * Pre-bootstrapped authenticated `TEAMS_PROFILE_DIR` sessions (`030-S` /
       `035-S` live-tenant and Recap runs) reuse an out-of-band headed
       operator-authenticated profile (`TEAMS_ACCOUNT` is optional when the
       profile already holds a session, and when set automates Entra
       account-picker tile selection only — never password or MFA credential
       entry; because `Headless: true` has no visible browser UI, any
       interactive password or MFA redirect cannot complete in-band and blocks
       until `LobbyTimeout`).
     * Isolated repo-local `TEAMS_PROFILE_DIR` bot profiles (`010-F` / U1
       confirmed guest topology) are kept isolated from ambient host `%APPDATA%`
       and `%LOCALAPPDATA%` profile fallback by `browser.NewLiveMeeting`
       (provided `.env.local.test` does not inject a live `TEAMS_ACCOUNT` /
       `TEAMS_TEST_ACCOUNT`), while on current `HEAD` `Meeting.Join` requires
       `TEAMS_ACCOUNT` on cold profiles for remote `https://` URLs
       (`browser.ErrAccountRequired`).
  3. Enforce strict comment-only change discipline: modify `//` comment lines
     immediately above `func newMeeting(t *testing.T)` only — zero executable
     statement, `t.Skip` message, selector, import, or logic edits.
  4. Enforce strict redaction (`Principle III`): include only symbolic
     environment-variable and template filenames (`TEAMS_PROFILE_DIR`,
     `TEAMS_BROWSER_PATH`, `TEAMS_ACCOUNT`, `TEAMS_MEETING_URL`, `TEAMS_TEST_*`,
     `.env.local.test`, `%APPDATA%`, `%LOCALAPPDATA%`); never embed live URLs,
     `p=` tokens, account/UPN values, or host filesystem profile paths.
* **Tests and Verification (`saw_red` -> `green_after_red`):**
  * Pre-change (`saw_red`): `func newMeeting(t *testing.T)` (`line 32` in
    `internal/browser/meeting_integration_test.go`) has no `// newMeeting ...`
    GoDoc comment explaining authenticated vs. isolated guest profile semantics.
  * Post-change (`green_after_red`):
    * `git --no-pager diff -- internal/browser/meeting_integration_test.go` shows
      added `//` comment lines starting with `// newMeeting ` above `func
      newMeeting(t *testing.T)` only (zero non-comment diffs, and zero
      `https?://` URLs, `@` UPNs, `p=` tokens, or host user-directory path
      prefixes in added lines).
    * `gofmt -l internal/browser/meeting_integration_test.go` produces empty
      output.
    * `go vet ./...`, `go vet "-tags=integration" ./internal/browser/...`,
      `golangci-lint run`, `golangci-lint run "--build-tags=integration"
      ./internal/browser/...`, `go test ./...`, and `go test "-tags=integration"
      "-run=^$" ./internal/browser/...` all exit `0`.
* **Depends on:** `010.001-T` (`blocks`, preserved unchanged from existing
  `010.008-T` DAG edge).

## Dependency Graph

```text
010.007-T (U7) ──▶ 010.001-T (U1) ──┬──▶ 010.002-T (U2) ──▶ 010.003-T (U3) ──┬──▶ 010.004-T (U4) ──┐
                                    │                                        └──▶ 010.005-T (U5) ──┴──▶ 010.006-T (U6) ──▶ 010.009-T (U9)
                                    └──▶ 010.008-T (U8: newMeeting docstring, reconciled in place)
```

* `010-S` remains gated behind completed predecessors `031-S` and `035-S` (both
  `shipped`).
* No new task or shipment edges are added; `010.008-T` retains its existing
  `blocks` dependency on `010.001-T` and parent `010-F`.

## Decisions and Rationale

* **Reconcile `010.008-T` in place rather than archiving or creating a replacement
  task**: `010.008-T` is already positioned in `010-S`'s manifest (`items[8]`)
  with dependency `010.001-T`. Updating `010.008-T` in place preserves manifest
  continuity, avoids unapproved destructive archival, and gives Ship an accurate
  target (`newMeeting(t)`) in the same file.
* **Create `025-DL` via `backlogit_create_item` and preserve `AD451E00`'s
  bound-target hangup/CDP helper request without mutating
  `.backlogit/stash.jsonl`**: `TestMeetingLiveScheduledOvertimeAndCallEnd` in
  `scheduled_lifecycle_integration_test.go` (`HEAD` lines `106-264`) is outside
  `010-S`, and exposing a bound-target hangup/CDP helper changes
  `internal/browser` API contracts (`P-021` C1). Because `backlogit_deliberate`
  rewrites `.backlogit/stash.jsonl` to stamp `"deliberation_id":"025-DL"`,
  creating `025-DL` via `backlogit_create_item` (`artifact_type: "deliberation"`)
  and linking `025-DL -> 010.008-T` (`informs`, without modifying `010-F`'s
  `source_deliberation_id` `001-DL`) keeps `.backlogit/stash.jsonl`
  byte-identical (protecting pre-existing foreign uncommitted entry `47D1E1C0`
  under `P-011`) and leaves `AD451E00` clean for its future hangup-helper
  deliberation. Under `INV-12`, `025-DL` stays queued until both `010-S` ships
  and `AD451E00` has zero live references.
* **Declare `harness-surface:none` on `010.008-T` as a non-docs test-only task
  (no route label)**: `internal/browser/meeting_integration_test.go` is a Go
  test file outside `docs/`; declaring `harness-surface:none` without
  `docs-route` avoids a `P-002` `docs-only-check` path failure, and under
  `_ship.agent.md` Step 4.0/4.2 Ship skips `harness-architect` (applying neither
  `harness-ready` nor `harness-verification-gated`) and executes `010.008-T`'s
  verification commands inline.

## Risks and Caveats

* **[Historical Wave 1A snapshot — RESOLVED in Wave 1B via `026-DL` on
  `2026-11-20` UTC] Sibling tasks in `010-S` lacked `harness-surface:*` labels
  (`Blocker B1`) and `010.007-T` carried post-`030-S`/`035-S` drift (`Blocker B2`)**:
  1. In Wave 1A, `010.001-T`..`010.007-T` and `010.009-T` lay outside initial
     `harvest_ids = ["010.008-T"]`, so Stage Step 5.3 / 5.5 forbade repairing
     their `DECLARATION_MISSING` labels without explicit operator confirmation.
  2. DAG-root `010.007-T` also targeted removed `browserContext(t)`, forbade
     already-shipped `browser.Options.ExecPath` (`allocator.go:25`,
     `meeting_integration_test.go:44`), and rejected external `%APPDATA%` /
     `%LOCALAPPDATA%` `TEAMS_PROFILE_DIR` paths. Following Wave 1A, the operator
     explicitly authorized Wave 1B (`026-DL`), resolving both blockers ahead of
     end-of-Wave-1 Ship execution of `010-S` (Wave 2A is the separate Recap
     stash track).
* **Dirty worktree paths & UTC chronology during staging publication**:
  `origin/main` (`a7015b9`) tracks `.autoharness/*` (not `.gsd/*`), and dozens
  of unrelated `.github/`, `.autoharness/`, `.gsd/`, `scripts/`, and
  `.backlogit/stash.jsonl` paths are dirty on `main`. Canonical 18-path
  literal-pathspec staging publication excludes all foreign dirty files,
  `.backlogit/stash.jsonl`, and local tool-managed event/checkpoint files
  (`.backlogit/hooks_queue.jsonl`, `.backlogit/checkpoints/*`). Authored
  `2026-11-19-*` filename slugs reflect local `2026-11-19T17:30:11-08:00` PST
  session start (`2026-11-20` UTC) and are retained to preserve frozen
  `plan_id` / `review_manifest` bindings.

## Constitution Check

* **Principle I (Safety-First Go)**: `010.008-T` is comment-only (`// newMeeting
  ...` GoDoc) in `internal/browser/meeting_integration_test.go` and verifies `go
  vet ./...`, `go vet "-tags=integration" ./internal/browser/...`, `golangci-lint
  run`, `golangci-lint run "--build-tags=integration" ./internal/browser/...`,
  and `gofmt -l`.
* **Principle II (Test-First Development & `P-002`/`P-004`)**: `010.008-T`
  touches only an integration `*_test.go` comment (zero production Go files),
  qualifying for `harness-surface:none` under `P-004` Rule 4 (`no roster and no
  RED obligation`), verified by tagged compile and diff-only-comments checks.
* **Principle III & IV (Workspace Isolation, Security Boundaries, Containment)**:
  All Stage artifacts stay inside `docs/` and `.backlogit/queue/`; zero secrets,
  live URLs, tokens, or profile paths appear; planned test usage targets
  authorized operator-controlled Teams fixtures/sessions only (no auth bypass,
  intrusion, or restriction evasion).
* **Principle V (Structured Observability)**: Deliberation (`025-DL`), review
  manifest (`docs/reviews/`), task comment, and Stage memory record full audit
  provenance.
* **Principle VI (Single Responsibility & Width Isolation)**: `010.008-T` stays
  1 file (`< 3`), 0 functions changed (`< 5`), single comment-only domain, `XS`
  / `trivial` (`<= 2h`).
* **Principle VII & VIII (Destructive Command Approval & Safety Modes)**: Zero
  destructive commands or archival mutations executed; `Freeze-scope mode` +
  `Careful mode` applied.
* **Principle IX & X (Git-Friendly Persistence & Context Efficiency)**: Clean
  LF Markdown + YAML frontmatter; query-first `backlogit` and `engram` lookups
  used throughout.
* **Principle XI (`P-009` Merge-Commit-Only)**: Any downstream staging PR merge
  must use `--merge` two-parent merge commits after operator authorization.

## Plan Hardening Signals (REQUIRED)

* **Public API, schema, or contract change**: **Absent** — `010.008-T` is a
  comment-only update on an unexported test helper (`newMeeting`), and the
  `browser.Meeting` hangup helper in `AD451E00` is explicitly deferred out of
  scope.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present
  (sensitive-data redaction boundary)** — `newMeeting(t)` configures
  `TEAMS_PROFILE_DIR`, `TEAMS_ACCOUNT`, and `TEAMS_MEETING_URL` for live
  Microsoft Teams sessions, requiring strict non-disclosure of live URLs,
  passcodes, UPNs, and filesystem profile paths, and explicit prohibition of
  automated credential entry.
* **Migration, backfill, destructive data/config action, or irreversible step**:
  **Absent** — in-place metadata/text reconciliation of a queued task with zero
  archival or deletion.
* **External integration, operator checkpoint, or external dependency**:
  **Present** — documents the human operator checkpoint boundary for Microsoft
  Entra sign-in/MFA vs. isolated guest pre-join in live integration tests, and
  coordinates Stage -> Orchestrator publication across a dirty worktree.
* **High runtime, rollout, or rollback risk**: **Absent** — comment-only test
  helper change with trivial `git revert` rollback.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed runtime surfaces**: None (`010.008-T` modifies only Go comments in
  `internal/browser/meeting_integration_test.go`, gated behind `//go:build
  integration`).
* **Runtime verification**:
  * Stage verification: `backlogit get 010.008-T`, `backlogit get 025-DL`,
    `backlogit get 010-S`, `gsd harness resolve --workspace . --shipment 010-S
    --json`, `gsd gate line-endings`, and `.backlogit/stash.jsonl` SHA-256
    equality check.
  * Ship verification (when `010-S` executes `010.008-T`): `git diff` confirms
    comment-only lines starting with `// newMeeting ` above `newMeeting(t)` with
    zero secret/URL/UPN/path patterns, `gofmt -l`, `go vet ./...`, `go vet
    "-tags=integration" ./internal/browser/...`, `golangci-lint run`,
    `golangci-lint run "--build-tags=integration" ./internal/browser/...`, `go
    test ./...`, and `go test "-tags=integration" "-run=^$"
    ./internal/browser/...`.
* **Operational closure**: Covered by `010-S`'s closure artifact (`010.006-T` +
  Ship Step 6 post-merge closure and `P-020` `compact-context`), explicitly
  citing active stash `AD451E00` for the remaining deferred bound-target
  hangup/CDP helper scope and retaining `025-DL` in `.backlogit/queue/` until
  both `010-S` and `AD451E00` have zero live references.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: Although `010.008-T` is an `XS` comment-only
  test update, it documents the live Microsoft Teams authentication/guest
  boundary (`TEAMS_PROFILE_DIR`, `TEAMS_ACCOUNT`, `TEAMS_MEETING_URL`) and
  executes Stage reconciliation inside a worktree containing pre-existing
  uncommitted `.backlogit/stash.jsonl` and harness changes.
* **Safety Mode Posture (Principle VIII)**: `Freeze-scope mode` (mutations
  confined strictly to Wave 1 Stage artifacts for `AD451E00` / `025-DL` and
  `010.008-T`) + `Careful mode` + `strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/backlogit.instructions.md`
  * `.github/policies/workflow-policies.md` (`P-002`, `P-003`, `P-004`, `P-006`,
    `P-010`, `P-011`, `P-012`, `P-017`, `P-021`)
  * `docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`
  * `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`
  * `docs/compound/workflow-issues/pwsh-dotted-go-flag-tokenization.md`

### Protected Invariants

1. **Zero Secret or Identifier Leakage (`Principle III`)**: Neither Stage
   artifacts nor Ship's `newMeeting(t)` comment may contain any live meeting
   URL, `?p=` passcode, Entra UPN/email, cookie/storage state, or local profile
   path; only environment-variable names may be referenced.
2. **Authorized Operator / Fixture Scope Only**: Documentation and test guidance
   target operator-controlled authorized Teams sessions and synthetic loopback
   fixtures exclusively; credential entry is never automated and no auth/policy
   bypass is introduced.
3. **Dirty `.backlogit/stash.jsonl` and Foreign Worktree Invariance (`P-011`)**:
   `.backlogit/stash.jsonl` (SHA-256
   `C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`,
   containing both `AD451E00` and uncommitted `47D1E1C0`) and all pre-existing
   dirty/untracked harness/config files MUST remain byte-for-byte untouched.
4. **No Unconfirmed Out-of-Scope Backlog Mutation**: Sibling tasks
   `010.001-T`..`010.007-T` and `010.009-T` outside `harvest_ids =
   ["010.008-T"]` MUST NOT be edited without explicit operator confirmation.
5. **Comment-Only Execution Boundary on `010.008-T`**: When Ship implements
   `010.008-T`, `git diff -- internal/browser/meeting_integration_test.go` MUST
   contain only `//` comment lines above `func newMeeting(t *testing.T)`.

### Lightweight 3-Threat Model (`Security Lens`)

1. **Most likely threat**: Accidental embedding of an operator Entra UPN or
   local filesystem profile path in task/comment prose where `gitignore_test.go`
   regexes only scan tracked files for `?p=` passcodes and JWTs — mitigated by
   explicit added-line secret/UPN/URL/path inspection in both Stage and `U1`.
2. **Highest-impact threat**: Unintended bleed of an operator's `.env.local.test`
   `TEAMS_ACCOUNT` / `TEAMS_TEST_ACCOUNT` into a nominally isolated guest test
   run via `requireIntegration(t)` -> `browser.LoadTestEnv(t)` — mitigated by
   documenting that `LoadTestEnv` behavior explicitly in `U1`'s `newMeeting(t)`
   GoDoc specification.
3. **Most subtle threat**: Future implementation of `AD451E00`'s deferred
   hangup/CDP helper exposing raw `m.attachedCtx` or arbitrary CDP script
   evaluation on exported `browser.Meeting` — mitigated by recording in `025-DL`
   that any future helper must stay a narrow hangup/teardown test seam.

### Strict-Safety `ProposedAction` Inventory

| ProposedAction | Targets | Change Kind | ActionRisk | Approval | ActionResult | Rollback |
|---|---|---|---|---|---|---|
| `PA-W1-1`: Create deliberation `025-DL` via `backlogit_create_item` (`artifact_type: "deliberation"`) for `AD451E00` without mutating `.backlogit/stash.jsonl` | `.backlogit/queue/025-DL.md` (published; local `.backlogit/hooks_queue.jsonl` side-effect event journal excluded from canonical 18-path publication) | Backlog deliberation creation | `low` | Authorized within Wave 1 Stage handoff | `applied` | Delete untracked `025-DL.md`, run `git checkout -- .backlogit/hooks_queue.jsonl`, and run `backlogit sync` (only if explicitly approved) |
| `PA-W1-2`: Reconcile `010.008-T` in place via four mutually exclusive `backlogit_update_item` calls (`title`+`description`, `labels: "harness-surface:none"`, `size: "XS"`+`size_source: "agent"`+`size_ruleset_version: "ah-stage-sizing-v1"`, and `complexity: "trivial"` with prose fallback), link `025-DL -> 010.008-T` (`informs`), and append traceability comment | `.backlogit/queue/010.008-T.md` (published; local `.backlogit/hooks_queue.jsonl` side-effect event journal excluded from canonical 18-path publication) | Backlog task metadata & text reconciliation | `low` | Authorized within Wave 1 Stage handoff | `applied` | Run `git checkout -- .backlogit/queue/010.008-T.md .backlogit/hooks_queue.jsonl` and `backlogit sync` (only if explicitly approved) |
| `PA-W1-3` (Downstream end-of-Wave-1 Ship execution in `010-S`): Add comment-only `// newMeeting ...` GoDoc docstring in `internal/browser/meeting_integration_test.go` | `internal/browser/meeting_integration_test.go` | Test-helper GoDoc comment update | `low` | Deferred to end-of-Wave-1 Ship execution of `010-S` after staging publication (`B1` and `B2` resolved in Wave 1B via `026-DL`) | `planned` | `git revert` of `010.008-T` commit on `010-S` branch |

### Verification Depth and Operational Closure

* **Environment prechecks (Stage)**: Verify `backlogit_docs_lint` passes with 0
  violations on both new `docs/` files, `gsd gate line-endings` passes on all
  new Stage files, and `.backlogit/stash.jsonl` retains its exact baseline
  SHA-256 (`C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`).
* **Blocked-path handling**: After updating `010.008-T`, run read-only `gsd
  harness resolve --workspace . --shipment 010-S --json`; because sibling tasks
  `010.001-T`..`010.007-T` and `010.009-T` are outside `harvest_ids`, report
  both their `DECLARATION_MISSING` status (`Blocker B1`) and `010.007-T`'s
  post-`030-S`/`035-S` specification drift (`Blocker B2`) to the
  Orchestrator/operator rather than bypassing the `harvest_ids` guardrail.
* **Rollback trigger & owner**: Repository owner (`softwaresalt`) / Orchestrator;
  any unintended touch outside Wave 1 Stage paths halts publication immediately.

### Review-Gate Capability Risks (`P-012`)

* Plan review (`plan-review`) dispatches the multi-persona reviewer roster
  (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`,
  `Learnings Researcher`, `Architecture Strategist` on anchor route
  `gpt-6.1-sol`, and `Security Lens Reviewer`) plus `Evidence Consistency
  Reviewer` as the independent second-opinion adversarial/`rubber-duck` pass
  (declaring `TOOL_DEGRADED: rubber-duck — declared fallback: same-roster
  adversarial review` under `P-012` because no standalone `rubber-duck` CLI/MCP
  binary is installed on this host, and supplying a temporary scratch-index git
  tree SHA on Pass 2 so `Evidence Consistency Reviewer`'s branch-tree-only
  constraint reads the exact candidate tree without touching `.git/index`).
* Record Pass 1 (`FAIL`, `revision: 1`) and targeted delta Pass 2 (`PASS`,
  `revision: 2`, `MaxPlanReviewPasses = 2`) atomically via `gsd op review
  record-result --in-place` before performing `PA-W1-1` or `PA-W1-2`.
* Unresolved operator decisions blocking Wave 1 Stage reconciliation: **None**
  *(historical Wave 1A sibling `010-S` blockers `B1` and `B2` were subsequently
  resolved in Wave 1B via `026-DL` ahead of end-of-Wave-1 Ship execution of
  `010-S`)*.

## Plan Review

Review pass 2: **PASS** (plan revision 2). See `docs/reviews/2026-11-19-ad451e00-010-008-t-reconciliation-plan-review.md`.
