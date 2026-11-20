---
title: "Wave 1B Operator-Authorized Reconciliation of 010-S Sibling Labels and Stale Specifications"
description: "Deliberation reconciling the eight missing harness-surface labels and post-011-S..035-S stale requirements across 010.001-T..010.007-T and 010.009-T in queued shipment 010-S without broadening scope"
doc_type: "decision"
source: ".backlogit/queue/010-S.md"
topic: "Operator-authorized Stage reconciliation of 010.001-T..010.007-T and 010.009-T labels and stale contracts in shipment 010-S"
depth: "standard"
decision_status: "decided"
promoted_to: "both"
deliberation_id: "026-DL"
feature: "010-F"
shipment: "010-S"
linked_artifacts:
  - "docs/plans/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-plan.md"
  - "docs/decisions/2026-11-19-ad451e00-010-008-t-reconciliation-deliberation.md"
  - ".backlogit/queue/010-S.md"
date: "2026-11-20"
status: "accepted"
tags:
  - "stage-reconciliation"
  - "010-S"
  - "p-002"
  - "p-004"
  - "browser-verification"
---

## Problem Frame

Following the initial Wave 1 Stage reconciliation of `AD451E00` and `010.008-T`
(`025-DL`,
`docs/decisions/2026-11-19-ad451e00-010-008-t-reconciliation-deliberation.md`),
the operator issued an explicit Stage Step 5.3 / Step 5.5 authorization:

> "I Authorize Stage to repair the eight missing harness-surface labels in 010-S
> and reconcile its stale requirements without expanding scope."

Queued shipment `010-S` (`covering_feature: 010-F`, 9 task members
`010.001-T`..`010.009-T`, `dependencies: ["031-S", "035-S"]` both `shipped`) was
originally staged on `2026-09-05` (PR `#27`). Between `011-S` (`494e656`) and
`035-S` (`a7015b9`), twenty-five shipments landed on `main` ahead of `010-S`.
Two structural blockers remained across the eight sibling tasks (`010.001-T`..`010.007-T`
and `010.009-T`):

1. **Missing `P-004` `harness-surface:*` labels (`DECLARATION_MISSING`)**: All
   eight sibling tasks have `labels: null`, causing `gsd harness resolve
   --workspace . --shipment 010-S --json` to exit `2` (`state: "UNRESOLVED"`,
   `reason_code: "DECLARATION_MISSING"`).
2. **Stale pre-`011-S`..`035-S` task requirements and file budgets**:
   * **`010.007-T` (`U7`, DAG root)**: Still targets removed `browserContext(t)`
     (folded into `newMeeting(t)` in `030.002-T` commit `b0b67e9`), forbids
     `browser.Options.ExecPath` (shipped in `011-S` / `013-S` at
     `internal/browser/allocator.go:25` with `ResolveExecPath` at lines `83-118`,
     wired at `meeting_integration_test.go:44` in `8983f5f` / `b0b67e9`), and
     mandates rejecting any `TEAMS_PROFILE_DIR` outside the git workspace — which
     would regress external `%APPDATA%` / `%LOCALAPPDATA%` profile support
     delivered in `030-S` and `035-S` (`scheduled_lifecycle_integration_test.go:41-81`).
     Meanwhile, `meeting_integration_test.go:32-57` still lacks `030-S`-compatible
     fail-closed `TEAMS_PROFILE_DIR` containment validation (rejecting repo-root
     and repo-ancestor paths and requiring `git check-ignore -q -- <relProfile>`
     on repo-local paths).
   * **`010.002-T` (`U2`), `010.003-T` (`U3`), and `010.004-T` (`U4`) production
     code overlaps**:
     * In `internal/teams/url.go:101-123`, `MutateJoinURL` already implements
       the exact `/l/meetup-join` vs. `/meet/` `q.Del("type")` branch (`494e656`),
       and `internal/teams/url_test.go:47-96` covers the four `/meet/` cases,
       leaving only the legacy `/l/meetup-join` full-string golden equality
       assertion and non-`/meet/` fallback coverage in `url_test.go`.
     * In `internal/browser/prejoin.go:10-23,150-155,169-173` and
       `internal/browser/meeting_chrome.go:701-712,915-950,1098-1126`, launcher
       handling (`joinOnWebSel` including `030.001-T`'s
       `#openTeamsClientInBrowser`), `#hangup-button` +
       `[data-tid="hangup-main-btn"]` (`meetingToolbarSel`), boolean `checked`
       property reads (`propBoolJS` + `micStateRequiresMute`), and `2026-09-05`
       provenance comments already shipped (`0` `VERIFY LIVE` markers remain, and
       `TestMeetingJoin` passed live in `030-S`). Un-reconciled `010.003-T`
       would strip `#openTeamsClientInBrowser`, `[data-tid="hangup-main-btn"]`,
       and `aria-checked` fallback.
     * In `internal/browser/captions.go:19-103` and
       `internal/browser/meeting_chrome.go:1388-1457` (`captionMenuStepWait` at
       line `31`), the `2026-09-05` caption
       selectors (retaining legacy fallback selectors), bounded `5s` two-variant
       `activateCaptions`, and `captionObserverJS` inner literals already shipped
       (`0` `VERIFY LIVE` markers remain). What genuinely remains unimplemented
       for `010.004-T` is strictly in test files: updating
       `TestMeetingEnableCaptions` (`meeting_integration_test.go:71-102`, which
       currently ignores `func(_, _ string)`) to fail on empty speaker or text,
       adding a `captionWrapperSel` Go-const vs. JS-literal sync test in
       `captions_test.go`, and adding guest-direct `#closed-captions-button` E2E
       coverage in `meeting_e2e_test.go`.
   * **`010.005-T` (`U5`, genuinely unimplemented production + test work)**:
     `internal/browser/audio.go:53-55` still carries `// VERIFY LIVE`,
     `internal/browser/meeting_chrome.go:1510-1608` (`CaptureAudio`, live PCM
     `newPayloadPump` callback at lines `1547-1565`) lacks the
     instance-scoped exported nil-by-default test-only PCM-before-encode observer
     seam, and `TestMeetingCaptureAudio` (`meeting_integration_test.go:108-147`)
     lacks the live-call stimulus/Goertzel assertion (and must preserve the
     `030-S`/`031-S` `RecapMeeting` branch at lines `119-130`, plus include
     non-build-tagged `internal/browser/audio_test.go` in its file budget so
     `P-004` `go test ./...` discovers its expected-RED roster test).
   * **`010.006-T` (`U6`) and `010.009-T` (`U9`) `P-002` docs-only route
     qualification**: Both prose documentation tasks currently cite directory
     prefixes (`docs/closure/browser-automation/` and `docs/compound/`) rather
     than explicit normalized `.md` file paths, lack `P-002` durable
     verification records (`Executable Gates` with `markdownlint` separate from
     `Manual Gates`), and `010.006-T` cites workspace-only `TEAMS_PROFILE_DIR`
     wording and five PR `#27` stash IDs (`87CDB97A`, `D2023898`, `EEA7F685`,
     `A99EEABD`, `3084A83E`) that were subsequently delivered and archived in
     `013-S` (`002-DL`) on `2026-11-11`.

### Crystallized Intent Block (`grill-me`, `mode: clarity_skip` / `dark_factory_assumed`)

| ID | Question | Decision | Source | Rationale |
|---|---|---|---|---|
| `D1` | How should already-shipped production logic in `010.002-T`, `010.003-T`, and `010.004-T` be reconciled? | Reconcile to evidence-backed test verification + missing unit/E2E/integration assertions without reimplementing or regressing shipped production code (`url.go`, `prejoin.go`, `captions.go`, `meeting_chrome.go`). | `operator` / `repo-fact` | Operator mandated preferring evidence-backed verification over reimplementing already-landed behavior; keeping already-implemented production files out of `010.002-T`..`010.004-T`'s file budgets prevents `P-004` `harness-architect` from forcing artificial production-breaking stubs while delivering every unverified test assertion. |
| `D2` | Which `harness-surface:*` labels apply across all 9 task members of `010-S` after reconciling file budgets? | `010.005-T` -> `harness-surface:harness-architect`; `010.001-T`, `010.002-T`, `010.003-T`, `010.004-T`, `010.006-T`, `010.007-T`, `010.008-T`, `010.009-T` -> `harness-surface:none` (per `_ship.agent.md` lines `442-448`, qualifying docs-only tasks `010.006-T` and `010.009-T` still route through `harness-architect`'s Step 2a label-only path at T2 to receive `harness-verification-gated` with no RED stub/evidence, while non-docs test/diagnostic tasks `010.001-T`..`010.004-T`, `010.007-T`, and `010.008-T` skip T2/T3 with no route label). | `repo-fact` | Only `010.005-T` adds/modifies production Go code (`internal/browser/audio.go`, `internal/browser/meeting_chrome.go`). Every other task's reconciled file budget touches strictly `*_test.go` files and/or `docs/` Markdown files (`P-004` Rule 4). |
| `D3` | How must `010.007-T` reconcile `TEAMS_BROWSER_PATH` and `TEAMS_PROFILE_DIR` against `011-S`..`035-S`? | Target `newMeeting(t)` in `internal/browser/meeting_integration_test.go` (1 test file): verify shipped `ExecPath: os.Getenv("TEAMS_BROWSER_PATH")` wiring without touching `allocator.go`, implement `030-S`/`035-S`-compatible profile validation (reject repo-root and repo-ancestor; require `git check-ignore -q -- <relProfile>` when `filepath.IsLocal(relProfile)`; allow external `%APPDATA%`/`%LOCALAPPDATA%` profiles), and add tagged unit tests. | `operator` / `repo-fact` | Preserves U7's fail-closed repo-local profile safety and `TEAMS_BROWSER_PATH` contract while honoring `Options.ExecPath` (`011-S`/`013-S`) and external profile support (`030-S`/`035-S`). |
| `D4` | How must `010.005-T` satisfy `P-004`'s default `go test ./...` RED discovery when `meeting_integration_test.go` is behind `//go:build integration`? | Include `internal/browser/audio_test.go` alongside `internal/browser/audio.go`, `internal/browser/meeting_chrome.go`, and `internal/browser/meeting_integration_test.go` so `harness-architect` places the expected-RED PCM-tap/Goertzel unit roster test in non-tagged `audio_test.go` while `meeting_integration_test.go` exercises the live/Recap integration assertions. | `repo-fact` | `P-004` runs `go test ./...` with no `-tags=integration` flag; without `audio_test.go`, `harness-architect` would hit `Zero-discovery` or `leaf-diff` out-of-surface failure. |
| `D5` | How are live-meeting human checkpoints handled across `010.001-T`, `010.003-T`..`010.006-T` without weakening acceptance criteria? | Preserve live-verification requirements when a live Teams meeting and organizer checkpoint are reachable via `.env.local.test`, and explicitly incorporate the `2026-09-05` Plan Hardening `Blocked-path handling` contract (when AFK/unreachable, record truthful `BLOCKED` live-meeting validator evidence while requiring 100% pass on all unit, tagged-compile, and real-Chromium loopback `httptest` E2E gates; never claim a false green live pass). | `repo-fact` | Matches `docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md` (`Blocked-path handling`), `browser-verification.instructions.md`, and `029-S`/`030-S` runbook precedent. |

* **Runnable Objective Acceptance Checks (`AC1..AC5`)**:
  * `AC1` (*Native resolver green*): `gsd harness resolve --workspace . --shipment 010-S --json` — `saw_red` before Wave 1B: `exit_code: 2`, `state: "UNRESOLVED"`, `reason_code: "DECLARATION_MISSING"`; `green_after_red`: `exit_code: 0`, `state: "HARNESS_READY"`, `declarations` has all 9 tasks (`010.005-T` -> `harness-architect`, remaining 8 -> `none`), `surfaces: ["harness-architect"]`.
  * `AC2` (*Size-composition gate*): `backlogit_get_shipment("010-S")` — `size_composition.unsized == 0` (`int`, not `bool`) and `histogram == {"M": 4, "S": 4, "XS": 1}` (`PASS` under `[SC-B1]` + `[SC-B5]`).
  * `AC3` (*Preserved membership, statuses, dependencies & prior Wave 1A contracts*): `010-S` stays `queued` and `.backlogit/queue/010-S.md` stays untouched with identical 10-item manifest order and DAG edges; `010.008-T` and `025-DL` remain unchanged; committed `HEAD:.backlogit/stash.jsonl` stays unchanged at SHA-256 `2085925DD4258468AA892F2BEA9AE426B6B10DBEFD298701D5F86D6F14B2D57A` while the dirty local working-tree `.backlogit/stash.jsonl` retains exact SHA-256 `C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`.
  * `AC4` (*`P-002` docs-route readiness on `010.006-T` and `010.009-T`*): Both tasks declare explicit regular `.md` paths under `docs/`, `harness-surface:none`, `docs-route candidate` prose, and valid separate `Executable Gates` (`markdownlint --disable MD013 "--" <owned_paths>` — self-contained without relying on untracked `.markdownlint.json`) and 4-field `Manual Gates` tables.
  * `AC5` (*Review manifest admission*): `gsd op review record-result --in-place` records `verdict: "PASS"` (`p0_open: 0`, `p1_open: 0`) on the Wave 1B plan before any `.backlogit/` mutation executes.

## Research Findings

* **Verified committed `HEAD` (`a7015b94be13e99124196be263ef5dda9f52c6a5`) facts**:
  * `internal/browser/allocator.go:25,83-118`: `Options.ExecPath` and
    `ResolveExecPath` shipped in `011-S`/`013-S`.
  * `.backlogit/archive/stash.jsonl`: All five original PR `#27` deferred stash
    entries cited by `010.006-T` (`87CDB97A`, `D2023898`, `EEA7F685`,
    `A99EEABD`, `3084A83E`) were delivered and archived on `2026-11-11` (`013-S`
    / `002-DL`), while committed `HEAD:.backlogit/stash.jsonl` has SHA-256
    `2085925DD4258468AA892F2BEA9AE426B6B10DBEFD298701D5F86D6F14B2D57A`.
* **Verified `origin/main` (`a7015b94be13e99124196be263ef5dda9f52c6a5`) vs.
  working-tree harness context**:
  * `origin/main` tracks `.autoharness/*` (`backlog-registry.yaml`,
    `config.yaml` at `schema_version: "1.3.0"`, `harness-manifest.yaml`,
    `install-overlays.py`, `workspace-profile.yaml`) and does **not** track
    `.gsd/*`.
  * The local working tree carries the operator's uncommitted `.autoharness/`
    -> `.gsd/` harness upgrade (`?? .gsd/harness-manifest.yaml` matching
    working-tree `.github/skills/harness-architect/SKILL.md` SHA-256
    `d82c15ab9d907f575fd6e9fcaa522ca56675b9cbbc2afbf78bda699e5b4f0558`).
    `HARNESS_READY` (`exit_code: 0`) was verified against this **local
    unpublished `.gsd` rollout**, **not** published `.autoharness/*`; Ship
    execution of `010-S` therefore remains gated on a separately published
    compatible harness (neither Stage nor Ship applies legacy route labels as a
    workaround or publishes the harness upgrade in this wave).
  * `markdownlint` (`0.49.1`) exits `0` with `markdownlint --disable MD013 "--"
    <path>` without depending on untracked `.markdownlint.json`.
  * `gsd gate pre-review --base main --json` exits `0` with 9 medium `ART-02`
    findings across `010.001-T`..`010.009-T`: verified as a detector mismatch
    because `ART-02` scans for a literal `## Acceptance Criteria` Markdown H2,
    whereas all 9 tasks populate `backlogit` v1.13's canonical
    `<!-- BEGIN:acceptance-criteria -->` sections (`558`–`1745` non-whitespace
    chars each; no redundant H2 headings added).

## Options Evaluated

### Option A: Evidence-Backed Verification Reconciliation + Exact File-Budget `harness-surface:*` Assignment (Recommended)

Reconcile `010.001-T`..`010.007-T` and `010.009-T` in place (`backlogit_update_item`)
so already-shipped production changes (`url.go`, `prejoin.go`, `captions.go`,
`meeting_chrome.go` join/caption paths, `allocator.go` `ExecPath`) are verified
and regression-locked by targeted unit/E2E/integration tests rather than
re-implemented or regressed, `010.007-T` enforces `030-S`/`035-S`-compatible
`TEAMS_PROFILE_DIR` containment in `meeting_integration_test.go`, `010.005-T`
implements the genuinely unshipped production PCM-before-encode observer tap +
stimulus verification (`harness-surface:harness-architect` with `audio_test.go`
in budget), and `010.006-T` / `010.009-T` carry explicit `P-002` docs-route
owned paths and verification records (`harness-surface:none`).

* **Pros**: Zero re-implementation or regression of `011-S`..`035-S`; zero
  artificial `P-004` stubbing of shipped production code; `gsd harness resolve`
  reaches `HARNESS_READY` (`exit_code: 0`); preserves all 10 `010-S` manifest
  items, statuses, sizes, and dependency edges.
* **Cons**: Requires careful alignment of all 8 task descriptions and acceptance
  criteria.
* **Effort**: medium. **Fit**: strong.

### Option B: Label `010.002-T`..`010.004-T` as `harness-surface:harness-architect` Without Reconciling Production Overlap

Assign `harness-surface:harness-architect` to `010.002-T`, `010.003-T`, and
`010.004-T` while leaving `url.go`, `prejoin.go`, `captions.go`, and
`meeting_chrome.go` as required production edits.

* **Pros**: Minimal edits to original September task prose.
* **Cons**: Fails during end-of-Wave-1 Ship execution of `010-S` because `P-004`
  forbids passing roster tests and requires a failing production RED-phase stub
  in `go test ./...`, forcing Ship to sabotage or reimplement already-shipped
  `MutateJoinURL`, `enforceMute`, and `activateCaptions` code, and leaves
  `010.003-T`/`010.007-T` instructing Ship to regress `#openTeamsClientInBrowser`
  and external `%APPDATA%` profiles.
* **Effort**: low now, high failure risk in Ship. **Fit**: poor.

### Option C: Archive or Remove Overlapped Tasks (`010.002-T`, `010.003-T`, `010.007-T`) from `010-S`

Retire overlapped tasks from `010-S` instead of reconciling them to verification
and remaining test gaps.

* **Pros**: Fewer tasks in `010-S`.
* **Cons**: Violates operator constraints ("do not ... move statuses, change
  membership", no destructive approval), drops genuinely unimplemented safety
  and regression checks (`newMeeting` profile containment in `010.007-T`,
  legacy golden equality in `010.002-T`, guest-direct caption E2E and empty
  speaker/text failure in `010.004-T`), and breaks `010-S` DAG edges.
* **Effort**: high. **Fit**: rejected.

## Trade-off Comparison

| Criterion | Option A (Evidence-backed verification + accurate surfaces) | Option B (Raw `harness-architect` on already-shipped code) | Option C (Archive/remove tasks) |
|---|---|---|---|
| Preserves `011-S`..`035-S` shipped behavior | Yes (zero regressions) | No (conflicts with `030-S`/`035-S` & `P-004`) | Drops remaining test/containment gaps |
| `gsd harness resolve` & end-of-Wave-1 `010-S` `P-002`/`P-004` compatibility | `HARNESS_READY` (`exit_code: 0`) & clean T1-T3 execution | Fails at T2/T3 (`P-004` RED stub conflict) | Violates no-membership-change rule |
| Preserves `010-S` membership, statuses, & DAG | Yes (all 10 items & edges intact) | Yes | No |

## Decision

Adopt **Option A**:

1. Create deliberation `026-DL` via `backlogit_create_item` (`artifact_type:
   "deliberation"`, preserving `025-DL` and `010.008-T` from Wave 1A and keeping
   `.backlogit/stash.jsonl` byte-identical).
2. Reconcile `010.001-T`..`010.007-T` and `010.009-T` in place via registered
   `backlogit_update_item` calls (separate description/section updates, dedicated
   `labels` updates, and `ah-stage-sizing-v1` size provenance updates where
   applicable while preserving existing sizes and dependency edges), linking
   `026-DL` to each reconciled task (`informs`) and recording concise Stage
   traceability comments.
3. Verify `gsd harness resolve --workspace . --shipment 010-S --json` returns
   `state: "HARNESS_READY"` (`exit_code: 0`) and `010-S`'s `size_composition`
   gate passes (`unsized == 0`).

## Rejected Alternatives

* **Option B** — rejected because labeling already-shipped production paths as
  `harness-surface:harness-architect` forces destructive `P-004` stubbing of
  working `main` code and leaves `030-S`/`035-S` regressions in task specs.
* **Option C** — rejected because changing `010-S` membership or archiving tasks
  is forbidden by the delegation contract and would drop genuinely unimplemented
  containment and test-verification deliverables.

## Unresolved Questions

* None blocking Stage completion or canonical 18-path staging publication (which
  excludes local `.backlogit/hooks_queue.jsonl` and disposable checkpoints).
  During end-of-Wave-1 Ship execution of `010-S` (Wave 2A is the separate Recap
  stash track), live-tenant `-tags=integration` runs remain subject to
  `.env.local.test` and organizer/meeting availability per `D5` (`Blocked-path
  handling`).
* **UTC Chronology vs. Filename Slug Note**: Session execution occurred on
  `2026-11-20` UTC (`025-DL` `2026-11-20T02:29:45Z`, `026-DL`
  `2026-11-20T03:48:42Z`); the `2026-11-19-*` filename slugs reflect local PST
  session start (`2026-11-19T17:30:11-08:00` = `2026-11-20T01:30:11Z` UTC) and
  are retained unchanged to preserve frozen `plan_id` / `review_manifest`
  bindings.

## Risks and Mitigations

* **`P-002` docs-only baseline check (`OWNED_PATH_DIRTY_AT_BASELINE`)** —
  Mitigated by assigning net-new, currently non-existent `docs/` target paths to
  `010.006-T` and `010.009-T` and forbidding edits to existing parent closure
  records.
* **`P-004` `go test ./...` discovery for `010.005-T`** — Mitigated by including
  non-tagged `internal/browser/audio_test.go` in `010.005-T`'s file budget
  alongside `audio.go`, `meeting_chrome.go`, and `meeting_integration_test.go`.
* **Dirty worktree & `.backlogit/stash.jsonl` protection (`P-011`)** — Mitigated
  by performing zero stash mutations and verifying `.backlogit/stash.jsonl`
  SHA-256 invariance (`C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`).
