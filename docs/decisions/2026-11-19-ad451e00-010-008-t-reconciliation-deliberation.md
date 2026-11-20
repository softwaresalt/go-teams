---
title: "Wave 1 P-021 Reconciliation of Stash AD451E00 and Queued Task 010.008-T"
description: "Deliberation reconciling queued task 010.008-T from removed browserContext(t) to newMeeting(t) within shipment 010-S while preserving the deferred bound-target hangup/CDP helper request in AD451E00"
doc_type: "decision"
source: ".backlogit/stash.jsonl#AD451E00"
topic: "Wave 1 Stage reconciliation of AD451E00 and stale queued task 010.008-T before shipment 010-S"
depth: "standard"
decision_status: "decided"
promoted_to: "both"
source_stash:
  - "AD451E00"
linked_artifacts:
  - "docs/plans/2026-11-19-ad451e00-010-008-t-reconciliation-plan.md"
  - ".backlogit/queue/010.008-T.md"
  - ".backlogit/queue/010-S.md"
date: "2026-11-20"
status: "accepted"
tags:
  - "p-021"
  - "browser-verification"
  - "stage-reconciliation"
  - "010-S"
---

## Problem Frame

Stash entry `AD451E00` (`priority: medium`, `kind: task`) carries the literal
`DEFERRED SCOPE EXPANSION` precedence marker from shipment `030-S` (`030-F`,
task `030.002-T`), which forces mandatory `deliberate` routing under `P-021`
Clause C6 before any planning or backlog mutation.

`AD451E00` bundles two distinct post-`030-S` concerns:

1. **Queued task `010.008-T` stale symbol target (`010-S` reconciliation)**:
   * `010.008-T` (Plan U8 in
     `docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md`, split
     from `010.006-T` during PR #27 review) is a comment-only `XS` task inside
     queued shipment `010-S` (`covering_feature: 010-F`).
   * Its queued specification instructs Ship to update the `browserContext(t)`
     docstring in `internal/browser/meeting_integration_test.go` so the helper
     no longer claims `TEAMS_PROFILE_DIR` is unconditionally `"already
     authenticated"` and instead documents the U1-confirmed guest pre-join
     topology (with any sign-in or MFA landing treated as an operator
     checkpoint).
   * During `030.002-T` (commit `b0b67e9`, merged to `main` via PR #74 commit
     `29d554b918a55c8b978d3f826c6672d358ef5a8a`), `browserContext(t)` and its
     stale 3-line docstring were folded directly into `newMeeting(t)` in
     `internal/browser/meeting_integration_test.go` (`HEAD` lines `32-57` of
     `147`), which now invokes `browser.NewLiveMeeting(...)` directly and
     currently carries no GoDoc or helper comment above line `32` (while line
     `37` retains `t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile for
     live meeting tests")`).
   * Leaving `010.008-T` pointed at removed `browserContext(t)` gives Ship a
     non-existent target symbol when executing `010-S`.
2. **Bound-target hangup / CDP test-control helper request (new API surface)**:
   * `AD451E00` also proposes exposing a bound-target hangup or CDP control
     helper on `browser.Meeting` (or `chromeMeeting`) for
     `TestMeetingLiveScheduledOvertimeAndCallEnd` in
     `internal/browser/scheduled_lifecycle_integration_test.go` (`HEAD` lines
     `106-264` of `298`).
   * When `browser.NewLiveMeeting` (`internal/browser/meeting.go:120-125`)
     routes a remote authenticated session with a non-empty `UserDataDir`
     through `cm.headedOpts` (`launchCleanHeadedBrowser` with `--headless=new`),
     the returned `bctx` is a plain `context.WithCancel` context while the bound
     page target lives in unexported `m.attachedCtx` inside
     `internal/browser/meeting_chrome.go`. Consequently, the best-effort
     `chromedp.Run(streamCtx, ...)` `#hangup-button` evaluation in
     `scheduled_lifecycle_integration_test.go` (`HEAD` lines `237-242`) cannot
     drive `m.attachedCtx` from external `package browser_test` without a new
     exported or test-seam contract.
   * `scheduled_lifecycle_integration_test.go` was introduced in `029-S` and
     updated in `030-S`; it is completely outside `010-F` / `010-S` (`TestMeetingJoin`,
     `TestMeetingEnableCaptions`, and `TestMeetingCaptureAudio` in
     `internal/browser/meeting_integration_test.go`).

### P-021 C5 and C6 Triage Record

* **Precedence classification (`P-021` C6)**: `AD451E00` begins with the literal
  `DEFERRED SCOPE EXPANSION` token and `requires deliberation: YES (P-021 C6)`,
  routing unconditionally to `deliberate` prior to planning.
* **Obligation (A) Unconditional duplicate detection**: Scanned all 26 active
  entries in `.backlogit/stash.jsonl` and all 251 indexed queue and archive
  records in `.backlogit/`. Peer `030-S` deferred entries `232C6844`
  (`joinHeadedWithDeadline` / `stopHeadedBrowser` serialization) and `3F6BA819`
  (`AllocatorOptions` profile expansion parity) cover disjoint contract
  surfaces; no other active stash entry or backlog item duplicates either half
  of `AD451E00`. Outcome: **CLEAN DUPLICATE SCAN** (zero duplicates found).
* **Obligation (B) Mandatory late-identifier reconciliation**: `AD451E00` was
  captured with `task ID 030.002-T; feature ID 030-F; shipment ID 030-S; PR
  number N/A; review-thread ID N/A`. Querying Ship's authoritative closure
  record `docs/closure/030-S-030-F-post-merge-closure.md` (and compacted memory
  `docs/memory/compacted/2026-11-16-030-S-live-integration-fixes-compacted.md`):
  * **Originating PR number reconciled**: **PR #74** (merge commit
    `29d554b918a55c8b978d3f826c6672d358ef5a8a`), recovered from
    `docs/closure/030-S-030-F-post-merge-closure.md` (`pull_request: 74`).
  * **Review-thread ID outcome**: **no late identifier found (`N/A` terminal)**.
    `docs/closure/030-S-030-F-post-merge-closure.md` confirms `AD451E00` was
    raised by the pre-PR 9-persona + `rubber-duck` local review panel before PR
    #74 opened (PR #74's two Copilot threads `PRRT_kwDOTStvas63xV7_` and
    `PRRT_kwDOTStvas63xV8G` addressed separate in-scope expansion fixes). Under
    `P-021` C3/C6, `review-thread ID: N/A` stands as the truthful terminal
    record.

### Crystallized Intent Block (`grill-me`, `mode: dark_factory_assumed`)

* **Intent Summary**:
  * **Outcome**: Reconcile queued task `010.008-T` in place within `010-S` so
    its comment-only target in `internal/browser/meeting_integration_test.go`
    points at `newMeeting(t)` (the direct `030.002-T` replacement for removed
    `browserContext(t)`) with valid `P-004` / `SHIP-8` metadata, while keeping
    `010-S`'s scope narrow and preserving the separate bound-target hangup/CDP
    helper request in active stash `AD451E00` without mutating dirty
    `.backlogit/stash.jsonl`.
  * **Boundaries**:
    * *In scope*: Stage deliberation (`025-DL` + `docs/decisions/`), Stage
      reconciliation plan + hardening + review manifest (`docs/plans/` +
      `docs/reviews/`), in-place reconciliation of `010.008-T` (description,
      acceptance criteria, implementation notes, `harness-surface:none` label,
      `size: XS`, `complexity: trivial`), semantic link `025-DL -> 010.008-T`
      (`informs`), traceability comment on `010.008-T`, and Stage continuity
      memory (`docs/memory/2026-11-19/`).
    * *Out of scope*: Implementing any production or test Go code (`P-010`),
      adding a hangup/CDP helper to `browser.Meeting` or `chromeMeeting`,
      creating a new shipment for the helper, modifying sibling tasks
      `010.001-T`..`010.007-T` or `010.009-T` outside `harvest_ids` without
      operator confirmation, editing or archiving `.backlogit/stash.jsonl`
      (which holds pre-existing uncommitted Wave 2+ entry `47D1E1C0`), or
      performing git branch/commit/push/PR operations during this Stage-only
      handoff.
  * **Constraints**: Authorized test fixtures and operator-controlled Teams
    sessions only; zero live URLs, passcodes (`p=`), account UPNs, or profile
    paths in any artifact; `P-010` Stage boundary; `P-011` protection of
    pre-existing dirty foreign paths and `.backlogit/stash.jsonl`; no assumed
    destructive or merge approval.
  * **Context**: `DARK_MODE_ACTIVE` Wave 1 Stage-only delegation ahead of queued
    shipment `010-S` (`031-S` and `035-S` blocking edges already satisfied).

| ID | Question | Decision | Source | Rationale |
|---|---|---|---|---|
| `D1` | Was `010.008-T` rendered obsolete by `b0b67e9`, or is `newMeeting(t)` its direct same-contract-surface successor? | Reconcile `010.008-T` onto `newMeeting(t)` (same-contract-surface target correction). | `repo-fact` | Commit `b0b67e9` (`030.002-T`) folded `browserContext(t)` into `newMeeting(t)` (`HEAD` `internal/browser/meeting_integration_test.go:32-57`) and left `newMeeting(t)` with zero docstring while retaining `t.Skip("set TEAMS_PROFILE_DIR to an authenticated profile...")` at line `37`; the U8 identity-mode documentation gap remains open on `newMeeting(t)` in the exact same file and comment-only slice. |
| `D2` | Should the bound-target hangup/CDP test-control helper in `AD451E00` enter `010-S` or spawn a new shipment in Wave 1? | `[ASSUMED]` Keep the helper out of `010-S` and spawn no new shipment in Wave 1; preserve it in active `AD451E00`. | `assumed` | `TestMeetingLiveScheduledOvertimeAndCallEnd` (`scheduled_lifecycle_integration_test.go:106-264`) is unrelated to `010-S`, adding a method to `browser.Meeting`/`chromeMeeting` expands the public/package API surface (`P-021` C1), and Wave 1 authorizes `010-S` reconciliation only. |
| `D3` | How should `025-DL` and `AD451E00` be recorded given `.backlogit/stash.jsonl` already has pre-existing uncommitted foreign line `47D1E1C0`? | `[ASSUMED]` Create `025-DL` via `backlogit_create_item` (`artifact_type: "deliberation"`) instead of `backlogit_deliberate`, and skip `stash_archive` / `stash_edit` on `AD451E00` so `.backlogit/stash.jsonl` remains byte-identical. | `assumed` | Both `backlogit_deliberate` (which stamps `"deliberation_id":"025-DL"` onto `.backlogit/stash.jsonl`) and `stash_edit`/`stash_archive` rewrite `.backlogit/stash.jsonl`, entangling Wave 1 staging artifacts with pre-existing foreign uncommitted entry `47D1E1C0` (`P-011`) and colliding with `AD451E00`'s future hangup-helper deliberation; `backlogit_create_item` creates `.backlogit/queue/025-DL.md` cleanly while keeping `.backlogit/stash.jsonl` byte-identical. |
| `D4` | Which `P-002` / `P-004` execution path and `harness-surface` label govern reconciled `010.008-T`? | Declare `harness-surface:none` on `010.008-T` as a non-docs test-only task (no route label). | `repo-fact` | `internal/browser/meeting_integration_test.go` is an integration `*_test.go` file outside `docs/` (zero production code -> `harness-surface:none`); under `_ship.agent.md` Step 4.0/4.2, a non-docs `harness-surface:none` task skips T2/T3 (`harness-architect`), receives neither `harness-ready` nor `harness-verification-gated`, and executes its recorded verification commands inline. |
| `D5` | May Wave 1A repair missing `harness-surface:*` labels or post-`030-S` drift on sibling tasks `010.001-T`..`010.007-T` and `010.009-T`? | Do not mutate sibling tasks outside initial `harvest_ids = ["010.008-T"]` in Wave 1A; surface both `DECLARATION_MISSING` and `010.007-T` post-`030-S`/`035-S` drift for operator confirmation before end-of-Wave-1 Ship execution of `010-S` *(historical Wave 1A snapshot — subsequently authorized by the operator and resolved in Wave 1B via `026-DL` on `2026-11-20` UTC)*. | `repo-fact` | Stage Step 5.3 and Step 5.5 forbid repairing `DECLARATION_MISSING` labels outside `harvest_ids` without operator confirmation, and Wave 1A forbade broadening scope beyond `AD451E00` and `010.008-T`. |

* **Domain Glossary**:
  * **Confirmed guest topology (`010-F` / U1)**: Live-harness verification mode
    where an isolated repo-local bot `TEAMS_PROFILE_DIR` (kept separate from
    host `%APPDATA%` / `%LOCALAPPDATA%` fallback by `browser.NewLiveMeeting`,
    provided `.env.local.test` does not inject `TEAMS_ACCOUNT` /
    `TEAMS_TEST_ACCOUNT` via `browser.LoadTestEnv`) exercises guest pre-join; on
    current `HEAD`, `chromeMeeting.Join` requires `TEAMS_ACCOUNT` on cold
    profiles for remote `https://` URLs (`browser.ErrAccountRequired`).
  * **Out-of-band authenticated profile / operator checkpoint**: Because
    `newMeeting(t)` passes `Headless: true` (`--headless=new`, no visible UI),
    authenticated headless runs require an out-of-band headed
    operator-bootstrapped `TEAMS_PROFILE_DIR` (`TEAMS_ACCOUNT` optional when a
    session exists, and when set automates Entra account-picker tile selection
    only — never password or MFA credential entry; any interactive password/MFA
    redirect during headless execution cannot complete in-band and blocks until
    `LobbyTimeout`).
  * **Same-contract-surface correction**: Updating a stale symbol reference in a
    queued task to its direct refactored replacement in the same file, domain,
    and acceptance boundary without adding behavior or files.
* **Runnable Objective Acceptance Checks (`AC1..AC4`)**:
  * `AC1` (*Reconciled task target & metadata*):
    `backlogit get 010.008-T` — `saw_red` before reconciliation: `010.008-T`
    references removed `browserContext(t)` and has `labels: null`;
    `green_after_red`: `010.008-T` targets `newMeeting(t)` (`// newMeeting ...`)
    in `internal/browser/meeting_integration_test.go`, cites the Wave 1 plan,
    carries `labels: ["harness-surface:none"]`, and sets `size: "XS"`
    (`size_source: "agent"`, `size_ruleset_version: "ah-stage-sizing-v1"`) plus
    `complexity: "trivial"` (structured if persisted by the `task` WIT, and
    preserved as `Size: XS | Complexity: trivial` prose per the `harvest`
    capability gate).
  * `AC2` (*Review manifest admission*):
    `gsd op review record-result --in-place ... --json` — `saw_red` before Pass
    2: manifest is `FAIL` at `plan_revision: 1`; `green_after_red`: manifest
    records `verdict: "PASS"` (`p0_open: 0`, `p1_open: 0`) at `plan_revision: 2`.
  * `AC3` (*Dirty `.backlogit/stash.jsonl` byte invariance*):
    `(Get-FileHash .backlogit/stash.jsonl -Algorithm SHA256).Hash` — `saw_red`
    would be any digest drift from pre-session baseline
    `C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`;
    `green_after_red`: post-session SHA-256 equals baseline byte-for-byte with
    `AD451E00` and pre-existing uncommitted `47D1E1C0` untouched.
  * `AC4` (*Downstream Ship verification contract on `010.008-T`*):
    When Ship executes `010.008-T` in `010-S`, `saw_red` is absence of a
    `// newMeeting ...` GoDoc comment above `func newMeeting(t *testing.T)` in
    `internal/browser/meeting_integration_test.go` (`line 32`);
    `green_after_red` is a comment-only diff above `newMeeting(t)` containing
    zero URLs, `@` UPNs, `p=` tokens, or local profile paths, with `gofmt -l
    internal/browser/meeting_integration_test.go` empty, `go vet ./...` exit 0,
    `go vet "-tags=integration" ./internal/browser/...` exit 0, `golangci-lint
    run "--build-tags=integration" ./internal/browser/...` exit 0, and `go test
    "-tags=integration" "-run=^$" ./internal/browser/...` exit 0.

## Research Findings

* **Codebase verification (`HEAD` `a7015b94be13e99124196be263ef5dda9f52c6a5`,
  `internal/browser/meeting_integration_test.go`, `147` lines)**:
  * Lines `32-57` define `func newMeeting(t *testing.T) (browser.Meeting,
    context.Context)` with no preceding comment, calling `meetingURL(t)` ->
    `requireIntegration(t)` (`browser.LoadTestEnv(t)` auto-loading unset/empty
    `TEAMS_*` and `TEAMS_TEST_*` variables from gitignored `.env.local.test`),
    checking `TEAMS_PROFILE_DIR` (`line 37`), invoking `browser.NewLiveMeeting`
    (`lines 39-53`, `Headless: true`, `UserDataDir: profile`, `ExecPath:
    os.Getenv("TEAMS_BROWSER_PATH")` at `line 44`, `DefaultAccount:
    os.Getenv("TEAMS_ACCOUNT")`), and registering `t.Cleanup(cleanup)`.
  * Commit `b0b67e9` (`030.002-T`) deleted `func browserContext(t *testing.T)
    context.Context` and its stale `"already authenticated"` comment when
    migrating from `chromedp.NewExecAllocator` + `browser.NewMeeting` to
    `browser.NewLiveMeeting`. Under `NewLiveMeeting`, remote `https://` URLs
    with a non-empty `UserDataDir` route through `cm.headedOpts`, where
    `chromeMeeting.Join` fails closed with `browser.ErrAccountRequired` on a
    cold profile when `DefaultAccount` (`TEAMS_ACCOUNT`) is empty.
* **Compound learnings (`Learnings Researcher`, `confidence: high`)**:
  * `docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`:
    `resolveExpandedProfileDir` (invoked by `NewLiveMeeting` via
    `resolveExistingProfileDir`) isolates repo-local, custom, and `t.TempDir()`
    profile directories (`!isPathInsideRepoRoot`) from host `%APPDATA%` /
    `%LOCALAPPDATA%` fallback so guest-mode `TEAMS_PROFILE_DIR` profiles never
    pick up ambient user credentials, while canonical per-user profiles reuse
    persisted authenticated sessions.
  * `docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`:
    Deliberations linked to queued tasks/shipments (`010.008-T`, `010-S`) or
    active stash entries (`AD451E00`) remain live governing references in
    `.backlogit/queue/` until the governed work ships.
  * `docs/compound/workflow-issues/pwsh-dotted-go-flag-tokenization.md`:
    Windows PowerShell commands must quote `-flag=value` tokens containing
    punctuation or regexes (for example `"-run=^$"` and `"-tags=integration"`).
* **Shipment `010-S` resolver and post-`030-S`/`035-S` sibling state**:
  * Read-only `gsd harness resolve --workspace . --shipment 010-S --json`
    currently exits `2` (`state: "UNRESOLVED"`, `reason_code:
    "DECLARATION_MISSING"`) because `010.001-T`..`010.009-T` were harvested on
    `2026-09-05` before `P-004` `harness-surface:*` task labels existed.
  * Sibling DAG-root task `010.007-T` (outside Wave 1's `010.008-T` delegation)
    also still references removed `browserContext(t)`, forbids
    `browser.Options.ExecPath` (already shipped at `internal/browser/allocator.go:25`
    and wired at `meeting_integration_test.go:44`), and requires rejecting any
    `TEAMS_PROFILE_DIR` outside the git workspace (conflicting with external
    `%APPDATA%` / `%LOCALAPPDATA%` profile support shipped in `030-S` and
    `035-S`); at the Wave 1A snapshot, both sibling blockers were elevated for
    explicit operator reconciliation before end-of-Wave-1 Ship execution of
    `010-S` *(subsequently resolved in Wave 1B via `026-DL` on `2026-11-20`
    UTC; Wave 2A is the separate Recap stash track)*.

## Options Evaluated

### Option A: In-Place Same-Contract-Surface Reconciliation of `010.008-T` + Create `025-DL` via `backlogit_create_item` + Preserve `AD451E00` Untouched (Recommended)

Update `010.008-T` in place (`backlogit_update_item`) so its title, description,
acceptance criteria, and implementation notes target `newMeeting(t)` in
`internal/browser/meeting_integration_test.go` (comment-only GoDoc `// newMeeting
...` documenting both pre-bootstrapped authenticated `TEAMS_PROFILE_DIR` usage
and isolated repo-local guest / cold-profile `ErrAccountRequired` behavior under
`browser.NewLiveMeeting`), assign `labels: "harness-surface:none"`, `size: "XS"`
(`ah-stage-sizing-v1`), and `complexity: "trivial"`, create deliberation
`025-DL` via `backlogit_create_item` (`artifact_type: "deliberation"`) linked
`025-DL -> 010.008-T` (`informs`), and leave `.backlogit/stash.jsonl`
(`AD451E00` + pre-existing uncommitted `47D1E1C0`) byte-identical so the
deferred bound-target hangup/CDP helper request remains preserved for a future
dedicated feature/shipment.

* **Pros**: Exact same 1-file comment-only contract surface as original U8;
  closes the undocumented `newMeeting(t)` gap left by `b0b67e9` while remaining
  consistent with `NewLiveMeeting`, `ErrAccountRequired`, and line `37`'s
  `t.Skip`; zero expansion of `010-S`; zero destructive operations; strict byte
  invariance on dirty `.backlogit/stash.jsonl` (`P-011` safe, avoiding
  `backlogit_deliberate`'s stash rewrite); satisfies `P-004` and `SHIP-8` on
  `010.008-T`.
* **Cons**: *(Historical Wave 1A snapshot — resolved in Wave 1B via `026-DL`)*:
  Pre-existing sibling tasks `010.001-T`..`010.007-T` and `010.009-T` required
  separate operator confirmation (`DECLARATION_MISSING` + `010.007-T`
  post-`035-S` alignment) before `010-S` cleared `gsd harness resolve` for
  end-of-Wave-1 Ship execution.
* **Effort**: low. **Fit**: strong.

### Option B: Retire / Archive `010.008-T` as Obsolete

Treat removal of `browserContext(t)` in `b0b67e9` as rendering `010.008-T`
obsolete, archiving `010.008-T` and removing it from `010-S`.

* **Pros**: Eliminates one `XS` task from `010-S`.
* **Cons**: Factually inaccurate because `newMeeting(t)` replaced
  `browserContext(t)` in the same file with zero docstring while retaining the
  `"set TEAMS_PROFILE_DIR to an authenticated profile"` skip message; requires
  destructive archival (`strict-safety` approval not pre-authorized in Wave 1);
  and `.gsd/backlog-registry.yaml` exposes no `remove_from_shipment` operation
  on a `queued` shipment (`return_blocked` is for active shipments).
* **Effort**: medium. **Fit**: poor.

### Option C: Bundle the Bound-Target Hangup / CDP Helper into `010-S` or a New Wave 1 Shipment

Plan and harvest a new production/test task exposing a bound-target hangup/CDP
helper on `browser.Meeting` or `chromeMeeting` for
`TestMeetingLiveScheduledOvertimeAndCallEnd` alongside `010.008-T`.

* **Pros**: Resolves both halves of `AD451E00` in one pass.
* **Cons**: Directly violates the Wave 1 authorization boundary ("Stage
  reconciliation before `010-S`, not insertion of an unrelated new API contract
  into the old shipment; don't implement or broaden `010-S`"); widens public or
  package API contracts (`internal/browser/meeting.go` / `meeting_chrome.go`);
  and is unnecessary to reconcile `010.008-T` or run `010-S`.
* **Effort**: medium-high. **Fit**: rejected (out of scope).

## Trade-off Comparison

| Criterion | Option A (Reconcile `010.008-T` onto `newMeeting`) | Option B (Archive `010.008-T` as obsolete) | Option C (Bundle hangup/CDP helper now) |
|---|---|---|---|
| Accuracy against `HEAD` (`meeting_integration_test.go`) | High (`newMeeting` is the undocumented replacement) | Low (leaves `newMeeting` identity contract undocumented) | High, but mixes unrelated surfaces |
| Wave 1 scope & `P-021` C1 compliance | Passes (same 1-file comment-only surface) | Fails (`strict-safety` destructive archive unapproved) | Fails (broadens API / shipment scope) |
| Dirty `.backlogit/stash.jsonl` safety (`P-011`) | Byte-identical preservation | Neutral, but destructive on queue/shipment | Risks rewriting dirty `stash.jsonl` |
| Backlogit registry compatibility | Supported (`create_task` deliberation, `update_task`, `add_link`, `append_comment`) | Unsupported (`queued` shipment member removal absent) | Requires new feature/shipment |

## Decision

Adopt **Option A**:

1. Create deliberation `025-DL` via `backlogit_create_item`
   (`artifact_type: "deliberation"`, recording `AD451E00`, reconciled `PR #74`,
   and terminal `review-thread ID: N/A` in its sections and references), which
   writes `.backlogit/queue/025-DL.md` without modifying `.backlogit/stash.jsonl`
   or pre-stamping `deliberation_id` on `AD451E00`.
2. Reconcile `010.008-T` in place via separate mutually exclusive
   `backlogit_update_item` calls onto `newMeeting(t)` in
   `internal/browser/meeting_integration_test.go` (1 file, comment-only GoDoc
   `// newMeeting ...`, non-docs `harness-surface:none` with no route label,
   `size: XS` under `ah-stage-sizing-v1`, `complexity: trivial`), linking
   `025-DL -> 010.008-T` (`informs`, never overwriting `010-F`'s
   `source_deliberation_id` `001-DL`) and recording a concise traceability
   comment.
3. Preserve `AD451E00` active and byte-unchanged in `.backlogit/stash.jsonl` so
   its remaining bound-target hangup/CDP test-control helper request stays
   queued for a future dedicated feature/shipment without touching pre-existing
   foreign uncommitted stash entry `47D1E1C0`; retain `025-DL` in
   `.backlogit/queue/` (`INV-12` independent task-linked deliberation) until both
   `010-S` ships and `AD451E00`'s remaining helper scope has zero live
   references.
4. Do not mutate sibling tasks `010.001-T`..`010.007-T` or `010.009-T` outside
   initial `harvest_ids` in Wave 1A; report both their `DECLARATION_MISSING`
   `harness-surface:*` assignments and `010.007-T`'s post-`030-S`/`035-S`
   specification drift for explicit operator confirmation prior to end-of-Wave-1
   Ship execution of `010-S` *(resolved in Wave 1B via `026-DL` on `2026-11-20`
   UTC; Wave 2A is the separate Recap stash track)*.

## Rejected Alternatives

* **Option B** — rejected because `browserContext(t)` was inlined into
  `newMeeting(t)` rather than eliminated, leaving `newMeeting(t)` without a
  docstring, and because archiving/removing a queued shipment member requires
  destructive authorization and an unregistered manifest-removal operation.
* **Option C** — rejected because exposing a bound-target hangup/CDP helper on
  `browser.Meeting` or `chromeMeeting` for `scheduled_lifecycle_integration_test.go`
  introduces a new API contract outside `010-S` and exceeds Wave 1's explicit
  reconciliation mandate.

## Unresolved Questions

* **[Historical Wave 1A snapshot — RESOLVED in Wave 1B via `026-DL` on
  `2026-11-20` UTC] Operator confirmation for legacy `010-S` sibling task
  `harness-surface:*` labels and `010.007-T` post-`035-S` drift**: At Wave 1A
  completion, sibling tasks `010.001-T`..`010.007-T` and `010.009-T` predated
  `P-004` `harness-surface:*` labels (`labels: null`), and DAG-root `010.007-T`
  still cited removed `browserContext(t)`, forbade `Options.ExecPath`, and
  rejected external `%APPDATA%` `TEAMS_PROFILE_DIR` paths. The operator
  subsequently authorized Wave 1B (`026-DL`), resolving both blockers ahead of
  end-of-Wave-1 Ship execution of `010-S`.
* **UTC Chronology vs. Filename Slug Note**: This session executed on
  `2026-11-20` UTC (`025-DL` created `2026-11-20T02:29:45Z`), starting at
  `2026-11-19T17:30:11-08:00` local PST (`2026-11-20T01:30:11Z` UTC); the
  `2026-11-19-*` filename slugs are retained unchanged to preserve frozen
  `plan_id` / `review_manifest` bindings.
* **Future scheduling of `AD451E00`'s remaining helper scope**: When scheduled,
  a separate deliberation/feature MUST constrain any test helper to a narrow
  hangup/teardown seam (for example an internal `package browser` test helper or
  narrow optional interface) strictly for authorized integration-test call-end
  teardown in `scheduled_lifecycle_integration_test.go`, never exposing raw
  `m.attachedCtx` or arbitrary CDP JavaScript evaluation on exported
  `browser.Meeting`. Once foreign `47D1E1C0` is committed in its own wave, Stage
  or the operator may also run `backlogit stash edit AD451E00` to annotate
  reconciled `PR #74` and the `025-DL` partial discharge on the stash entry text.

## Risks and Mitigations

* **Dirty `.backlogit/stash.jsonl` contamination (`P-011`)** — Mitigated by
  creating `025-DL` via `backlogit_create_item` instead of `backlogit_deliberate`
  (and avoiding `stash_archive`, `stash_edit`, and `stash` mutations), verified
  by exact pre/post SHA-256 invariance on `.backlogit/stash.jsonl`.
* **Route-label confusion on `010.008-T` (`P-002` / `_ship.agent.md` Step
  4.0/4.2)** — Mitigated by explicitly specifying in `010.008-T` that
  `internal/browser/meeting_integration_test.go` is a non-docs
  `harness-surface:none` test-only task that skips `harness-architect` (carrying
  neither `harness-ready` nor `harness-verification-gated`) and executes its
  recorded verification commands inline.
* **Credential, UPN, or URL leakage in test comments (`Principle III`)** —
  Mitigated by prohibiting live URLs, `p=` tokens, account/UPN values, and
  profile paths in `newMeeting(t)` comments (referencing env-var / config file
  identifiers `TEAMS_PROFILE_DIR`, `TEAMS_BROWSER_PATH`, `TEAMS_ACCOUNT`,
  `TEAMS_MEETING_URL`, `TEAMS_TEST_*`, and `.env.local.test` only) and verifying
  added comment lines against secret patterns.
