---
title: Default Microsoft Entra account auto-selection and config test hardening decision
description: Deliberation on adding default_account / --account Entra account-picker auto-selection and raising internal/config test coverage above 80%
topic: Default Microsoft Entra Account Auto-Selection & Config Test Hardening (30E3B3A7 + A368D60E)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 010-DL
feature: 021-F
shipment: 021-S
source_stash:
  - 30E3B3A7
  - A368D60E
linked_artifacts:
  - docs/plans/2026-11-13-default-entra-account-selection-plan.md
  - .backlogit/queue/010-DL.md
  - .backlogit/queue/021-F.md
  - .backlogit/queue/021-S.md
date: 2026-11-13
status: accepted
tags:
  - config
  - browser
  - cli
  - entra
  - auth
  - tests
---

## Problem Frame

This deliberation covers the contextual grouping of two active stash entries for
shipment **`021-S`** under covering feature **`021-F`**:

1. **`30E3B3A7`** (`feature`, `medium`):
   > Add a configuration file option to set a default Entra account to be
   > selected from the account picker list when the Entra authentication pop-up
   > window appears in the browser, and have teams-cli automatically select
   > that account from the list.
2. **`A368D60E`** (`task`, `low`, follow-up from `020-S` closure
   `docs/closure/2026-11-11-020-S-review-and-closure.md`):
   > Follow-up from 020-S closure
   > (docs/closure/2026-11-11-020-S-review-and-closure.md): raise
   > internal/config test coverage above 80 percent (currently 69.7 percent)
   > with alias and output-precedence cases.

### Why These Entries Belong Together

* **Shared package surface (`internal/config` and documentation)**: `30E3B3A7`
  adds `DefaultAccount` (`default_account` with YAML aliases `entra_account`
  and `account`) to `internal/config/config.go` and `internal/config/config_test.go`,
  which is the exact file pair targeted by `A368D60E` to raise `internal/config`
  coverage above 80% and lift its grade in `docs/QUALITY_SCORE.md` from `B` to
  `A`.
* **Operational prerequisite for unattended scheduled capture (`022-S`)**:
  When a persisted Chromium profile holds multiple Microsoft 365 / Entra ID
  sessions or an SSO redirect lands on `login.microsoftonline.com` /
  `login.live.com`, Microsoft Entra presents the "Pick an account" interstitial
  (`#tilesHolder` / `[data-test-id="<upn>"]` / `.table[role="button"]` /
  `div[data-bind*="acctTile"]`). Without automatic account tile selection, both
  interactive `auth` / `captions` and unattended `join` / `schedule` runs stall
  waiting for a manual click even when valid session tokens already exist in
  the profile.

## Post-Closure Deliberation Hygiene Record (`009-DL`)

Per the post-closure Stage archival rule in
`.github/skills/shipment-reconcile/SKILL.md` and
`docs/compound/workflow-issues/retained-engine-unverified-deliberation-cleanup.md`:

* Covering shipment `020-S` is in `.backlogit/archive/020-S.md` with
  `archived_status: shipped` (commit `480e7162851583fe684598e50d2c0b2bc0a45f7c`),
  and covering feature `020-F` is in `.backlogit/archive/020-F.md` with
  `archived_status: done`.
* `009-DL` had `status: queued` in `.backlogit/queue/` with zero remaining live
  governing or dependency references in `.backlogit/queue/` or
  `.backlogit/stash.jsonl`.
* Stage archived `009-DL` via `backlogit archive 009-DL` during Step 0 hygiene.

## P-021 C5 and C6 Triage Record

* Neither `30E3B3A7` nor `A368D60E` carries the `DEFERRED SCOPE EXPANSION`
  marker.
* **Obligation (A) Unconditional duplicate detection**: Executed across
  `.backlogit/stash.jsonl`, `.backlogit/archive/`, and `.backlogit/queue/`.
  Outcome: **CLEAN DUPLICATE SCAN** for both `30E3B3A7` and `A368D60E`.

## Options Evaluated

### Option A: URL `login_hint` query parameter injection only

Append `login_hint=<default_account>` to the initial Microsoft Teams meeting or
home URL without inspecting or clicking the in-browser Entra account picker
DOM.

* **Pros**: Minimal code change in `internal/teams` or `internal/browser`.
* **Cons**: Microsoft Teams `/l/meetup-join/...`, `/meet/...`, and
  `/l/meetingrecap?...` links redirect through Teams bootstrap and SharePoint
  endpoints that frequently strip custom `login_hint` parameters before
  reaching `login.microsoftonline.com`, and multi-account browser profiles still
  render the `#tilesHolder` "Pick an account" interstitial during token refresh.
  Also directly fails the operator requirement in `30E3B3A7` to select the
  account from the account picker list when the Entra authentication window
  appears.
* **Effort**: low | **Fit**: low.

### Option B: DOM-based Entra account picker auto-selection across `Runner`, `Join`, and `Recap` + config alias & coverage hardening (chosen)

1. **`internal/config` (`021.001-T`)**:
   * Add `DefaultAccount string \`yaml:"default_account,omitempty"\`` to
     `Config` in `internal/config/config.go`.
   * Support YAML aliases `entra_account` and `account` in `Load` with
     deterministic precedence: `default_account` > `entra_account` > `account`,
     trimmed with `strings.TrimSpace`.
   * Expand `internal/config/config_test.go` (`A368D60E`) with table-driven
     tests for `default_account` / `entra_account` / `account`, `output` vs.
     `output_file` vs. `output_folder` vs. `default_path` precedence, `headless`
     vs. `headed` pointer combinations, `capture_audio` vs. `audio`,
     `filename_template` vs. `name_template`, `meeting_url` vs. `url`,
     `expandUserPath` (`~`, `%USER%`, `%USERPROFILE%`, custom `%ENV%`, unknown
     `%VAR%`), and `defaultProfileDir` when `LOCALAPPDATA` is empty, raising
     `internal/config` statement coverage above 80%.
2. **`internal/browser` (`021.002-T`, `021.003-T`, `021.005-T`)**:
   * Add `DefaultAccount string` to `browser.Options` (`allocator.go`) and
     `browser.MeetingOptions` (`meeting.go`), propagating `opts.DefaultAccount`
     into `mopts.DefaultAccount` in `NewLiveMeeting` when `mopts.DefaultAccount`
     is empty.
   * Add `selectEntraAccountJS(account string) string` and
     `redactAccountIdentifier(account string) string` in
     `internal/browser/prejoin.go`:
     - JSON-encode the trimmed account string via `json.Marshal` so arbitrary
       config input can never break JavaScript syntax or inject code.
     - Match only when an Entra / Microsoft account picker surface is visible
       (`#tilesHolder`, `[data-test-id]`, `div.table[role="button"]`,
       `div[data-bind*="acctTile"]`, `[data-testid="accountTile"]`) and no
       visible password input (`input[type="password"]`) or MFA challenge
       input is active.
     - Exclude the "Use another account" tile (`#otherTile`,
       `[data-bind*="otherTile"]`, `[data-test-id="otherTile"]`).
     - Match a candidate tile case-insensitively using **exact, unambiguous
       matching** (never arbitrary substring containment, so
       `alice@contoso.com` never matches `bob.alice@contoso.com` and `"John"`
       never matches `"Johnathan"`): when its trimmed `data-test-id` attribute
       equals the configured account exactly, or when any trimmed tile text line
       or nested cell equals the configured UPN/email or display text exactly.
       Collect all matching candidate tiles first and fail closed (`return false`
       without clicking) when `matches.length !== 1` (`0` or `> 1` matches).
     - Redact any logged account identifier via `redactAccountIdentifier`
       (e.g. `a***@contoso.com` or `j***`) so raw UPNs/emails never appear in
       logs (`strict-safety` `ActionRisk: high` protection).
   * Wire `selectEntraAccountJS` into:
     - `ChromeRunner.run` (`internal/browser/runner.go`) for `teams-cli auth`
       and `Runner.Join`.
     - `chromeMeeting.joinHeaded` (`internal/browser/meeting_chrome.go`) when
       `isAuthRedirectURL(mainPage.URL)` is true during headed live calls and
       headed Meeting Recap flows.
     - `chromeMeeting.awaitPreJoinOrAdmission` (`internal/browser/meeting_chrome.go`)
       in the `PreJoinWaitingForUser` branch during headless live calls and
       headless Meeting Recap flows.
3. **`internal/cli` (`021.004-T`)**:
   * Add `--account` flag to `browserFlags` (`bindBrowserFlags` /
     `applyBrowserFlags` in `internal/cli/root.go`, covering `auth` and
     `schedule`) and `captureFlags` (`bindCaptureFlags` / `applyCaptureFlags` /
     `inheritedQueueFlags` in `internal/cli/join.go`, covering `root`, `join`,
     `captions`, and batch `--queue` forwarding).
   * Populate `DefaultAccount: c.DefaultAccount` in `browserOptions(c)` and
     `defaultNewMeeting`.
4. **Documentation (`021.006-T`)**:
   * Update `README.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md`
     (upgrading `internal/config` grade from `B` to `A` with >80% coverage).

* **Pros**: Works across all three browser flows (`Runner`/`auth`, `Join`, and
  `Recap`, both headed and headless), safely serializes account strings into JS,
  redacts UPNs in logs, and completes `A368D60E` in the same release unit.
* **Cons**: Requires synthetic DOM unit tests in `internal/browser` to verify
  tile matching and non-interference with password/MFA screens.
* **Effort**: medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (`login_hint` URL param only) | Option B (DOM picker auto-select + config test hardening) |
|---|---|---|
| Handles Entra `#tilesHolder` account picker interstitial | No (dropped on Teams/SharePoint redirects) | Yes (detects and clicks matching tile across `auth`, `join`, and `recap`) |
| Multi-account persisted Chromium profile support | Partial | Full (matches UPN `data-test-id` or display name case-insensitively) |
| Security & log hygiene (`strict-safety`) | Exposes UPN in URL query if unredacted | JSON-encoded DOM lookup + `redactAccountIdentifier` in logs |
| Resolves `A368D60E` (`internal/config` > 80% coverage) | No | Yes (table tests in `021.001-T` + `QUALITY_SCORE.md` grade `A`) |

## Decision

We select **Option B** and decompose the work into covering feature **`021-F`**
and shipment **`021-S`** across six width-isolated tasks (`021.001-T` through
`021.006-T`), each touching `<= 3` files within a single domain (`code`,
`tests`, or `docs`).

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Infinite click loop if Entra transition is slow after clicking the account tile**:
  Mitigated by tracking a per-navigation / cooldown guard (`accountPickerClicked`
  or poll cooldown) so `selectEntraAccountJS` does not repeatedly click the
  tile on every poll while `login.microsoftonline.com` is submitting the form,
  and resetting `authTracker.settledPolls = 0` so `headedAuthTracker` does not
  treat the account-picker redirect as a rapid bounce loop.
* **PII / UPN leakage in logs (`strict-safety` `ActionRisk: high`)**:
  Mitigated by `redactAccountIdentifier(account)` which masks the local part or
  display name (`a***@tenant.com`) before passing to `slog.Info` / `slog.Warn`.
