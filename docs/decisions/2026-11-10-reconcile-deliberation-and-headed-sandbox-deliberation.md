---
title: P-021 deferred scope expansion reconciliation for shipment-reconcile deliberation disposition and headed browser sandbox opt-in
description: Deliberation on reconciling shipment-reconcile linked-deliberation cascade rules with backlogit 1.12 / compute_linked_deliberation_disposition and honoring TEAMS_DISABLE_SANDBOX / Options.DisableSandbox in the direct-exec browser launcher
topic: Reconcile shipment-reconcile linked-deliberation disposition (1B03802C) and direct-exec browser launcher sandbox opt-in (FC763C17)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 006-DL
feature: 017-F
shipment: 017-S
source_stash:
  - 1B03802C
  - FC763C17
linked_artifacts:
  - docs/plans/2026-11-10-reconcile-deliberation-and-headed-sandbox-plan.md
  - .backlogit/queue/006-DL.md
  - .backlogit/queue/017-F.md
  - .backlogit/queue/017-S.md
date: 2026-11-10
status: accepted
tags:
  - p-021
  - shipment-reconcile
  - browser
  - security
---

## Problem Frame

Two P-021 C2 `DEFERRED SCOPE EXPANSION` entries from recent shipments require
mandatory Step 2 deliberation under P-021 C6 before planning and harvest:

1. **`1B03802C`** (`task`, `medium`, surfaced on PR #36 during `014-S`
   post-merge closure, review-thread `PRRT_kwDOTStvas62B91t`):
   In `.github/skills/shipment-reconcile/SKILL.md`, Step 0(c) ("Linked-deliberation
   snapshot extension") and Cascade Close Sub-Procedure step 3 currently treat
   every deliberation ID matched in a qualifying feature's
   `custom_fields.source_deliberation_id`, description text, or `references` as
   both an `allowed_ids` member AND (whenever not already `status: archived`
   pre-close) a `required_ids` member that `backlogit shipment ship` must return
   in `archived_ids`. However, in backlogit 1.12.0 and
   `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`,
   a deliberation linked only via description/references mention without
   `custom_fields.source_deliberation_id` is classified as
   `retained_description_mention` (`LinkedDeliberationOutcome.RETAINED_DESCRIPTION_MENTION`)
   under INV-12 outcome precedence (`retained_read_error -> retained_ambiguous
   -> already-archived -> retained_engine_unverified -> retained_live_status ->
   retained_shared_reference -> retained_description_mention -> archive`), and
   the backlogit 1.12 engine retains it rather than archiving it during
   `ShipShipment` (as observed with `002-DL` in `013-S` and `003-DL` in
   `014-S`). As written, `SKILL.md` would falsely fail the
   missing-required-artifact check (`required_ids - archived_ids`) on any
   description-mentioned deliberation.
2. **`FC763C17`** (`task`, `medium`, surfaced on PR #33 during `013-S`,
   review-thread `PRRT_kwDOTStvas62BKaF`, task `013.003-T`):
   In `013.003-T` (`013-S`), `Options.DisableSandbox` and
   `TEAMS_DISABLE_SANDBOX` (`"1"` or `"true"`) were wired into the `chromedp`
   allocator flag emitters (`StealthFlags` in `internal/browser/allocator.go`
   and `cleanHeadlessFlags` in `internal/browser/meeting.go`), keeping the
   Chromium sandbox enabled by default. However, the direct-exec browser
   launcher in `internal/browser/recap.go` (`launchCleanHeadedBrowser` /
   `cleanBrowserArgs` / `cleanHeadedBrowserArgs`), which launches headed
   `join --headed` and `captions` sessions via `exec.Command`, does not consult
   `shouldDisableSandbox(opts)`. As a result, operators running `join --headed`
   or `captions` in a root/container environment with Xvfb/VNC cannot opt out of
   the Chromium sandbox even when setting `TEAMS_DISABLE_SANDBOX=1` or
   `Options.DisableSandbox: true`.

## P-021 C5 and C6 Triage and Reconciliation Record

Both separately triggered Step 1 obligations were executed for `1B03802C` and
`FC763C17`:

* **(A) Unconditional duplicate detection**: Executed across
  `.backlogit/stash.jsonl`, `.backlogit/archive/stash.jsonl`,
  `.backlogit/queue/`, and `.backlogit/archive/`.
  * **`1B03802C`**: **CLEAN DUPLICATE SCAN** — no duplicate stash or backlog
    item exists.
  * **`FC763C17`**: **CLEAN DUPLICATE SCAN** — no duplicate stash or backlog
    item exists (`013.003-T` in `013-S` is archived and covered only
    `StealthFlags` and `cleanHeadlessFlags`).
* **(B) Late-identifier reconciliation**:
  * **`1B03802C`**: Source refs at capture were `PR number: 36`,
    `review-thread ID: PRRT_kwDOTStvas62B91t`,
    `task ID: N/A (post-merge closure step, no single task)`,
    `feature ID: 014-F`, and `shipment ID: 014-S`. Because `task ID` was `N/A`,
    Stage inspected the Ship-owned closure record
    `docs/closure/2026-11-10-014-S-review-and-closure.md`, which confirms the
    finding arose on the post-merge shipment closure PR (#36) rather than any
    individual task (`014.001-T` or `014.002-T`). Outcome: **no late task
    identifier found (`task ID: N/A` stands as the truthful terminal record for
    a post-merge closure finding; `PR: 36`,
    `review-thread: PRRT_kwDOTStvas62B91t`, `feature: 014-F`, and
    `shipment: 014-S` are concrete and carried forward)**.
  * **`FC763C17`**: All five source-reference fields were already concrete at
    capture (`task ID: 013.003-T`, `feature ID: 013-F`, `shipment ID: 013-S`,
    `PR number: 33`, `review-thread ID: PRRT_kwDOTStvas62BKaF`, corroborated by
    `docs/closure/2026-11-10-013-S-review-and-closure.md`). Outcome:
    **idempotent no-op (all source refs concrete at capture)**.

## Options Evaluated

### Issue 1 (`1B03802C`): `shipment-reconcile` linked-deliberation `allowed_ids` / `required_ids` rule

* **Option 1A (Force `SKILL.md` prose to override the engine)**: Keep
  description-mentioned deliberations in `required_ids` and treat backlogit
  1.12's retention of `retained_description_mention` deliberations as a hard
  `HALT` failure.
  * *Limitation*: Falsely halts every valid cascade closure where a feature
    mentions its deliberation in prose/references without setting
    `custom_fields.source_deliberation_id`, contradicting both backlogit 1.12
    and `autoharness.gates.shipment_closure.compute_linked_deliberation_disposition`.
* **Option 1B (Chosen — Align `SKILL.md` with `compute_linked_deliberation_disposition` and backlogit 1.12 INV-12 semantics with a pre/post retained-deliberation preservation check)**:
  Update `.github/skills/shipment-reconcile/SKILL.md` (Step 0(c), Cascade Close
  Sub-Procedure step 3, post-cascade source-artifact cleanup, and Quality
  Criteria) so that:
  1. Step 0(c) records each validated linked deliberation's link kind
     (`custom_fields.source_deliberation_id` vs description/references mention)
     and evaluates its pre-close INV-12 disposition via
     `compute_linked_deliberation_disposition` (`retained_read_error ->
     retained_ambiguous -> already-archived -> retained_engine_unverified ->
     retained_live_status -> retained_shared_reference ->
     retained_description_mention -> archive`).
  2. In Cascade Close Sub-Procedure step 3, only deliberations classified as
     `expected_cascade` (`archive` / `PLANNED_ARCHIVE` — linked via
     `custom_fields.source_deliberation_id` and not matching any
     `already-archived` or `retained_*` rule) belong in BOTH `allowed_ids` AND
     `required_ids` for the `backlogit_ship_shipment` cascade call itself (while
     `already-archived` deliberations are excluded from `required_ids` and
     tolerated in `allowed_ids`). Deliberations classified into any of the
     retained dispositions (`retained_description_mention`,
     `retained_shared_reference`, `retained_live_status`, as well as
     `retained_engine_unverified`, `retained_ambiguous`, `retained_read_error`)
     are EXCLUDED from `required_ids` AND EXCLUDED from `allowed_ids` for the
     cascade call, AND Step 3 runs an explicit **pre/post retained-deliberation
     preservation check**: if any retained deliberation (especially
     `retained_shared_reference` or `retained_live_status`, which must remain
     live in `.backlogit/queue/` for other unshipped work) appears in
     `archived_ids` or is missing from `.backlogit/queue/` immediately after the
     cascade call, the skill MUST halt with `unexpected-cascade`
     (`HALT — cascade archived retained deliberation {id}, revert required`) and
     run the `git-revert-on-cascade` recovery procedure.
  3. In post-cascade source-artifact cleanup, ONLY a
     `retained_description_mention` deliberation whose sole referencing feature
     just shipped and has no remaining live references may be archived
     explicitly by the agent (`backlogit archive <dl_id>`), whereas
     `retained_shared_reference` and `retained_live_status` deliberations MUST
     remain in `.backlogit/queue/` untouched.

### Issue 2 (`FC763C17`): Direct-exec headed browser launcher sandbox opt-in

* **Option 2A (Never allow sandbox opt-out in `launchCleanHeadedBrowser`)**:
  Refuse `TEAMS_DISABLE_SANDBOX` / `Options.DisableSandbox` on the direct-exec
  launcher in `internal/browser/recap.go`.
  * *Limitation*: Leaves an inconsistent contract within `internal/browser`
    where headless `NewLiveMeeting` and `ChromeRunner` honor
    `Options.DisableSandbox` / `TEAMS_DISABLE_SANDBOX`, but headed
    `NewLiveMeeting` (`launchCleanHeadedBrowser`) fails to launch Chromium in
    containerized/root Xvfb environments even when explicitly opted in.
* **Option 2B (Chosen — Honor `shouldDisableSandbox(opts)` in `internal/browser/recap.go` while keeping sandbox enabled by default)**:
  Keep the Chromium sandbox enabled by default (`--no-sandbox` and
  `--disable-setuid-sandbox` omitted when `shouldDisableSandbox(opts)` is
  `false`), and emit `--no-sandbox` and `--disable-setuid-sandbox` (before the
  trailing `initialURL` argument) in the direct-exec launcher (`cleanBrowserArgs`
  / `cleanHeadedBrowserArgs` / `launchCleanHeadedBrowser`) **only** when
  `shouldDisableSandbox(opts)` is `true` (`opts.DisableSandbox == true` or
  `TEAMS_DISABLE_SANDBOX` is `"1"` or `"true"`).

## Trade-off Comparison

| Criterion | Rejected Options (1A + 2A) | Chosen Options (1B + 2B) |
|---|---|---|
| Verifier vs. engine agreement (`1B03802C`) | `SKILL.md` contradicts backlogit 1.12 & `compute_linked_deliberation_disposition` | `SKILL.md`, `compute_linked_deliberation_disposition`, and backlogit 1.12 agree on INV-12 |
| Default browser sandbox security (`FC763C17`) | Enabled by default | Enabled by default (unchanged unless opted in) |
| Root/container headed launch (`FC763C17`) | Broken even when `TEAMS_DISABLE_SANDBOX=1` | Consistent opt-in across all `internal/browser` launch paths |

## Decision

We select **Option 1B** and **Option 2B**, synthesizing covering feature
**`017-F`** (*P-021 deferred scope expansion reconciliation: shipment-reconcile
deliberation disposition and headed browser sandbox opt-in*) and shipment
**`017-S`** (chained after `016-S` via
`backlogit dep add 017-S 016-S --type blocks`) with two strictly width-isolated
tasks:

1. **`017.001-T` (U1, `docs` / skill domain — `.github/skills/shipment-reconcile/SKILL.md`)**:
   Reconcile Step 0(c), Cascade Close Sub-Procedure step 3, and Quality Criteria
   in `.github/skills/shipment-reconcile/SKILL.md` with backlogit 1.12 and
   `compute_linked_deliberation_disposition` (`retained_description_mention`
   and INV-12 retention rules) (`XS` / `low`).
2. **`017.002-T` (U2, `code` domain — `internal/browser/recap.go` & `internal/browser/recap_test.go`)**:
   Make the direct-exec browser launcher (`launchCleanHeadedBrowser` /
   `cleanBrowserArgs` / `cleanHeadedBrowserArgs`) honor
   `shouldDisableSandbox(opts)` (`Options.DisableSandbox` and
   `TEAMS_DISABLE_SANDBOX`) while preserving sandbox-enabled defaults, with unit
   tests in `internal/browser/recap_test.go` (`XS` / `low`).

## Rejected Alternatives

* **Option 1A**: Rejected because it leaves a permanent contradiction between
  the skill prose and the verified `autoharness` / `backlogit` 1.12 engine.
* **Option 2A**: Rejected because `Options.DisableSandbox` and
  `TEAMS_DISABLE_SANDBOX` are documented in `README.md` and `allocator.go` as
  the workspace's container opt-in mechanism.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Merge ordering on `internal/browser/recap.go`**: Shipment `015-S`
  (`015.001-T`) adds `extractRecapAudioJS` and `captureRecapAudioFromTargets`
  to `internal/browser/recap.go`. Chaining `017-S -> 016-S -> 015-S -> 014-S`
  guarantees `015-S` merges to `main` before `017-S` is claimed, and `017.002-T`
  is scoped specifically to `cleanBrowserArgs`, `cleanHeadedBrowserArgs`, and
  `launchCleanHeadedBrowser` in `internal/browser/recap.go`.
