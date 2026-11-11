---
title: P-021 deferred scope expansion reconciliation plan for shipment-reconcile deliberation disposition and headed browser sandbox opt-in
description: Implementation plan for 017-F / 017-S reconciling shipment-reconcile linked-deliberation cascade rules with backlogit 1.12 / compute_linked_deliberation_disposition and honoring TEAMS_DISABLE_SANDBOX / Options.DisableSandbox in the direct-exec browser launcher
source: docs/decisions/2026-11-10-reconcile-deliberation-and-headed-sandbox-deliberation.md
deliberation_id: 006-DL
feature: 017-F
shipment: 017-S
date: 2026-11-10
status: approved
tags:
  - p-021
  - shipment-reconcile
  - browser
  - security
---

## Problem Frame

Following the accepted deliberation in
[2026-11-10-reconcile-deliberation-and-headed-sandbox-deliberation.md](../decisions/2026-11-10-reconcile-deliberation-and-headed-sandbox-deliberation.md)
(`006-DL`), two P-021 C2 deferred scope expansion items (`1B03802C` and
`FC763C17`) are planned under covering feature **`017-F`** and shipment
**`017-S`**:

1. **`1B03802C` (`.github/skills/shipment-reconcile/SKILL.md`)**:
   Reconcile Step 0(c) ("Linked-deliberation snapshot extension"), Cascade Close
   Sub-Procedure step 3, and Quality Criteria in
   `.github/skills/shipment-reconcile/SKILL.md` with backlogit 1.12 engine
   behavior and `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`
   (`LinkedDeliberationOutcome.RETAINED_DESCRIPTION_MENTION` and INV-12 outcome
   precedence), so a linked deliberation that is retained by the engine (such as
   one mentioned only in a feature's description or `references` without
   `custom_fields.source_deliberation_id`) remains permitted in `allowed_ids`
   but is NOT placed in `required_ids` (where its engine-expected absence from
   `archived_ids` would falsely trigger `HALT — cascade did not archive required
   artifact {id}`).
2. **`FC763C17` (`internal/browser/recap.go` & `internal/browser/recap_test.go`)**:
   Make the direct-exec browser launcher (`launchCleanHeadedBrowser` /
   `cleanBrowserArgs` / `cleanHeadedBrowserArgs` in `internal/browser/recap.go`)
   honor the same `shouldDisableSandbox(opts)` (`Options.DisableSandbox` and
   `TEAMS_DISABLE_SANDBOX` `"1"` or `"true"`) opt-in that `013.003-T` wired into
   `StealthFlags` and `cleanHeadlessFlags`, while keeping the Chromium sandbox
   enabled by default.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Reconcile `.github/skills/shipment-reconcile/SKILL.md` Step 0(c), Cascade Close Sub-Procedure step 3, and Quality Criteria with backlogit 1.12 and `compute_linked_deliberation_disposition` (`retained_description_mention` / INV-12) | `1B03802C` | U1 (`017.001-T`) | Markdown / skill structure check and consistency verification against `autoharness.gates.shipment_closure` |
| R2: Honor `Options.DisableSandbox` and `TEAMS_DISABLE_SANDBOX` in the direct-exec browser launcher (`launchCleanHeadedBrowser` / `cleanBrowserArgs` / `cleanHeadedBrowserArgs` in `internal/browser/recap.go`) while keeping sandbox enabled by default | `FC763C17` | U2 (`017.002-T`) | `go test -race ./internal/browser/...` |

## Implementation Units

### U1 (`017.001-T`) — Reconcile `shipment-reconcile` linked-deliberation cascade rules with backlogit 1.12 and `compute_linked_deliberation_disposition`

* **Domain:** docs (skill specification). **Posture:** specification reconciliation. **Size:** `XS` | **Complexity:** `low`.
* **What:** In `.github/skills/shipment-reconcile/SKILL.md`:
  1. **Step 0(c) ("Linked-deliberation snapshot extension")**:
     * Distinguish the link kinds collected for each validated linked
       deliberation of a qualifying feature member:
       `custom_fields.source_deliberation_id` (`source_deliberation_id` link
       kind) vs. description/references matches (`\b(?:DL\d+|[0-9]+(?:\.[0-9]+)*-DL)\b`).
     * Record each validated linked deliberation's pre-close INV-12 disposition
       per `compute_linked_deliberation_disposition` outcome precedence:
       `retained_read_error -> retained_ambiguous -> already-archived ->
       retained_engine_unverified -> retained_live_status ->
       retained_shared_reference -> retained_description_mention -> archive`
       (where a deliberation linked only via description/references without
       `custom_fields.source_deliberation_id` resolves to
       `retained_description_mention`, because backlogit 1.12's `ShipShipment`
       retains it rather than archiving it during the cascade).
  2. **Cascade Close Sub-Procedure preamble, step 3, and step 5**:
     * Keep `allowed_ids` = manifest tasks + qualifying feature members +
       every validated linked deliberation ID of each qualifying feature member
       + the shipment record.
     * Define `required_ids` = the shipment record and every qualifying feature
       member (both unconditionally) + every manifest task item not truly
       `status: archived` in the Step 0(b) pre-close declared-status snapshot +
       every validated linked deliberation of a qualifying feature member whose
       Step 0(c) pre-close INV-12 disposition is `archive` (`PLANNED_ARCHIVE` —
       i.e., linked via `custom_fields.source_deliberation_id`, not
       `already-archived` (`status: archived`), and not any `retained_*`
       disposition such as `retained_description_mention`,
       `retained_live_status`, `retained_shared_reference`,
       `retained_engine_unverified`, `retained_ambiguous`, or
       `retained_read_error`).
     * Clarify that a validated linked deliberation with disposition
       `retained_description_mention` (or any other `retained_*` /
       `already-archived` outcome) is retained by backlogit 1.12 during
       `backlogit shipment ship` and is therefore exempt from `required_ids`
       (while remaining in `allowed_ids`); when a `retained_description_mention`
       deliberation has no remaining live referrers outside the closed shipment
       scope, Ship's post-cascade source-artifact cleanup step may archive it
       explicitly via `backlogit archive <dl_id>`.
  3. **Quality Criteria**: Update the Cascade Close Sub-Procedure bullet in
     `## Quality Criteria` so its `required_ids` definition matches Step 0(c)
     and Cascade Close Sub-Procedure step 3.
* **Files (1 file):** `.github/skills/shipment-reconcile/SKILL.md`.
* **Verify:** Inspect diff against `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`,
  `prompt-builder.instructions.md`, and `markdown.instructions.md`.
* **Depends on:** none.

### U2 (`017.002-T`) — Honor `TEAMS_DISABLE_SANDBOX` and `Options.DisableSandbox` in the direct-exec browser launcher

* **Domain:** code (Go `internal/browser` + colocated unit test). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. In `internal/browser/recap.go` (incorporating `015.001-T`'s additions
     cleanly since `017-S` executes after `015-S` and `016-S` merge):
     * Add `cleanBrowserArgsForOptions(port int, profileDir string, opts Options, initialURL string) []string`
       (or update `cleanBrowserArgs` / `cleanHeadedBrowserArgs` while keeping
       their existing signatures working and delegating to
       `cleanBrowserArgsForOptions`) so that:
       * By default (when `shouldDisableSandbox(opts)` is `false`), `--no-sandbox`
         and `--disable-setuid-sandbox` are NOT emitted, preserving the
         sandbox-enabled default.
       * When `shouldDisableSandbox(opts)` is `true` (`opts.DisableSandbox == true`
         or `TEAMS_DISABLE_SANDBOX` is `"1"` or `"true"`), `"--no-sandbox"` and
         `"--disable-setuid-sandbox"` are appended to `args` before the
         trailing `initialURL` argument (and `--disable-blink-features=AutomationControlled`
         is still never emitted).
     * In `launchCleanHeadedBrowser(opts Options, initialURL string)`, build
       `args` using `cleanBrowserArgsForOptions(port, absProfile, opts, targetURL)`
       so both `opts.DisableSandbox` and `TEAMS_DISABLE_SANDBOX` are honored on
       direct-exec headed and headless browser launches.
  2. In `internal/browser/recap_test.go`:
     * In `TestCleanHeadedBrowserFlags` and `TestCleanHeadlessBrowserFlags`,
       call `t.Setenv("TEAMS_DISABLE_SANDBOX", "")` for the default assertions
       and add subtests / test cases verifying that when
       `opts.DisableSandbox = true` or `TEAMS_DISABLE_SANDBOX=1` / `"true"`,
       `cleanBrowserArgsForOptions` / `cleanHeadedBrowserArgs` includes
       `"--no-sandbox"` and `"--disable-setuid-sandbox"` before `initialURL`,
       omits `AutomationControlled`, and keeps `initialURL` as the final
       argument.
* **Files (2 files):** `internal/browser/recap.go`,
  `internal/browser/recap_test.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** none (within `017-S`; shipment `017-S` depends on `016-S` -> `015-S`).

## Dependency Graph

```text
U1 (017.001-T: .github/skills/shipment-reconcile/SKILL.md — docs/skill domain)
U2 (017.002-T: internal/browser/recap.go & recap_test.go — Go code/test domain)
```

* U1 and U2 touch disjoint files and domains (`docs` vs `code`).
* Shipment `017-S` is chained after `016-S` via
  `backlogit dep add 017-S 016-S --type blocks` (which in turn is chained after
  `015-S`, ensuring `015.001-T`'s edits to `internal/browser/recap.go` are
  already merged to `main` before `017-S` begins).

## Decisions and Rationale

* **Strict per-task width isolation**: `017.001-T` touches only
  `.github/skills/shipment-reconcile/SKILL.md` (`docs` domain), and `017.002-T`
  touches only `internal/browser/recap.go` and `internal/browser/recap_test.go`
  (`code` domain).
* **Preserve sandbox-enabled default in `recap.go`**: The direct-exec launcher
  omits `--no-sandbox` and `--disable-setuid-sandbox` unless the operator
  explicitly opts in via `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX`,
  matching the security posture established in `013.003-T`.

## Risks and Caveats

* **Cross-shipment touch on `internal/browser/recap.go`**: Both `015.001-T`
  (`015-S`) and `017.002-T` (`017-S`) modify `internal/browser/recap.go`.
  Enforcing `017-S blocks 016-S` and `016-S blocks 015-S` serializes execution
  so `017-S` branches from `main` after `015-S` has merged.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Clean helper signature in `recap.go`, zero lint/vet findings |
| II: Test first | Unit tests in `recap_test.go` verify both default sandbox-enabled flags and explicit opt-in under `-race` |
| III, IV: Secrets & containment | Browser sandbox remains enabled by default; opt-out requires explicit `DisableSandbox` or `TEAMS_DISABLE_SANDBOX` |
| VI: Single responsibility | Two `XS` tasks strictly isolated by domain (`docs` vs `code`) |
| VII, VIII: Strict safety | Browser sandbox opt-in and skill gate contract update classified in `Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `006-DL`, plan, feature `017-F`, tasks `017.001-T`–`017.002-T`, and shipment `017-S` (`blocks 016-S`) recorded |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: updates the
  `shipment-reconcile` skill's cascade-close verification contract (`017.001-T`).
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  touches Chromium sandbox launch flags in `internal/browser/recap.go`
  (`017.002-T`).
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**.
* External integration, operator checkpoint, or external dependency — **absent**.
* High runtime, rollout, or rollback risk — **absent**.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 modifies a shipment-closure verification gate
skill and U2 modifies browser sandbox launch flags.

* **Instructions and learnings consulted**:
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/prompt-builder.instructions.md`
  * `.github/instructions/go.instructions.md`
  * `docs/closure/2026-11-10-013-S-review-and-closure.md`
  * `docs/closure/2026-11-10-014-S-review-and-closure.md`

### Protected Invariants

1. **Unconditional `required_ids` membership for the shipment record, qualifying
   features, and non-archived manifest tasks**: U1's exemption from
   `required_ids` applies **only** to validated linked deliberations whose
   pre-close INV-12 disposition is `already-archived` or `retained_*`
   (including `retained_description_mention`); it never weakens the
   unconditional requirement on the shipment record, qualifying feature
   members, or non-archived manifest tasks, nor does it relax the
   `archived_ids - allowed_ids` unexpected-artifact check.
2. **Sandbox enabled by default**: When `Options.DisableSandbox` is `false` and
   `TEAMS_DISABLE_SANDBOX` is unset/empty, `cleanBrowserArgs`,
   `cleanHeadedBrowserArgs`, and `cleanBrowserArgsForOptions` MUST NOT emit
   `--no-sandbox` or `--disable-setuid-sandbox`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Reconcile linked-deliberation `required_ids` rule in `.github/skills/shipment-reconcile/SKILL.md` with `compute_linked_deliberation_disposition` (`retained_description_mention`) | moderate | Preserves `allowed_ids` check and unconditional `required_ids` for shipment, qualifying features, and manifest tasks; exempts only INV-12 retained/already-archived linked deliberations | Written `shipment-reconcile` verifier and backlogit 1.12 / `autoharness` engine agree on description-linked deliberations |
| Emit `--no-sandbox` and `--disable-setuid-sandbox` in `internal/browser/recap.go` when `shouldDisableSandbox(opts)` is `true` | high | Off by default; requires explicit `Options.DisableSandbox: true` or `TEAMS_DISABLE_SANDBOX=1`/`true`; verified by unit tests in `recap_test.go` | `join --headed` and `captions` honor explicit sandbox opt-out in root/container environments while keeping the sandbox enabled by default |

## Runtime Verification and Closure

* **Verification**:
  * `go test -race ./internal/browser/...` and `go test -race ./...`
  * `go vet ./...` and `go vet -tags=integration ./...`
  * CI-pinned `golangci-lint run` (`v1.64.8` with `GOTOOLCHAIN=go1.26.0`)
  * `gofmt -l cmd internal`
  * Exercise the updated `.github/skills/shipment-reconcile/SKILL.md` cascade
    verification during `017-S` post-merge closure.
* **Operational closure**:
  * Monitor `017-S` cascade closure verification against `006-DL` and confirm
    zero mismatch between `SKILL.md` and `backlogit shipment ship`.
  * **Rollback**: Revert the `017-S` merge commit on `main`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Both units are `XS` tasks (`1` file and `2` files) with strict width isolation (`docs` vs `code`). | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — `cleanBrowserArgsForOptions` reuses `shouldDisableSandbox(opts)` in package `browser` and keeps `initialURL` as the final positional argument. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly bounded to `1B03802C` and `FC763C17`; accounts for `015-S`'s prior edits to `internal/browser/recap.go`. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Incorporates closure findings from `013-S` (`PRRT_kwDOTStvas62BKaF`) and `014-S` (`PRRT_kwDOTStvas62B91t`). | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Aligns `.github/skills/shipment-reconcile/SKILL.md` with `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition` and unifies sandbox opt-in across all `internal/browser` launch paths. | PASS |
| Security Lens Reviewer (browser sandbox & gate contract surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Preserves default-enabled Chromium sandbox and fail-closed `allowed_ids` / `required_ids` checks. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `017-F` and shipment `017-S`.
