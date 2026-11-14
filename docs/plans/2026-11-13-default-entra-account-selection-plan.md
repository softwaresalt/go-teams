---
title: Default Microsoft Entra account auto-selection and config test hardening implementation plan
description: Implementation plan for 021-F / 021-S adding default_account / --account Entra account-picker auto-selection across Runner, Join, and Recap flows and raising internal/config test coverage above 80%
source: docs/decisions/2026-11-13-default-entra-account-selection-deliberation.md
deliberation_id: 010-DL
feature: 021-F
shipment: 021-S
source_stash:
  - 30E3B3A7
  - A368D60E
date: 2026-11-13
status: approved
tags:
  - config
  - browser
  - cli
  - entra
  - auth
  - tests
  - docs
---

## Problem Frame

Following the accepted deliberation in
[2026-11-13-default-entra-account-selection-deliberation.md](../decisions/2026-11-13-default-entra-account-selection-deliberation.md)
(`010-DL`), stash entries **`30E3B3A7`** (`feature`, `medium`) and **`A368D60E`**
(`task`, `low`) are planned under covering feature **`021-F`** and shipment
**`021-S`**:

1. **Unattended Microsoft Entra account picker auto-selection (`30E3B3A7`)**:
   When a Chromium profile contains multiple cached Microsoft accounts or an
   SSO flow redirects through `login.microsoftonline.com` / `login.live.com`,
   Microsoft Entra displays the "Pick an account" interstitial (`#tilesHolder`
   with account tiles carrying `data-test-id="<upn>"`, `.table[role="button"]`,
   or `div[data-bind*="acctTile"]`). Operators need a `default_account`
   configuration field (with YAML aliases `entra_account` and `account`) and a
   `--account` CLI flag across `root`, `join`, `captions`, `auth`, and
   `schedule` (plus `inheritedQueueFlags` forwarding) so `teams-cli`
   automatically selects the matching account tile across `Runner` (`auth`),
   live `Join`, and `Recap` flows while redacting any logged account identifier
   under `strict-safety` (`ActionRisk: high`).
2. **`internal/config` unit-test coverage hardening (`A368D60E`)**:
   `internal/config` currently stands at 69.7% statement coverage (`Grade B` in
   `docs/QUALITY_SCORE.md`). Adding table-driven tests for alias precedence
   (`default_account` / `entra_account` / `account`, `output` vs. `output_file`
   vs. `output_folder` vs. `default_path`, `headless` vs. `headed`,
   `capture_audio` vs. `audio`, `filename_template` vs. `name_template`,
   `meeting_url` vs. `url`), `expandUserPath`, and `defaultProfileDir` raises
   coverage above 80% and lifts `internal/config` to `Grade A`.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Add `Config.DefaultAccount string` (`yaml:"default_account,omitempty"`) with YAML aliases `entra_account` and `account` (`default_account` > `entra_account` > `account`), trimmed on load | `30E3B3A7` | U1 (`021.001-T`) | `go test -race -cover ./internal/config/...` |
| R2: Raise `internal/config` unit-test coverage above 80% with alias and output-precedence table tests plus `default_account` / `entra_account` / `account` tests | `A368D60E`, `30E3B3A7` | U1 (`021.001-T`) | `go test -race -cover ./internal/config/...` (>80%) |
| R3: Add `Options.DefaultAccount`, `selectEntraAccountJS(account)`, `redactAccountIdentifier(account)`, and automatic Entra account-picker selection in `ChromeRunner.run` (`auth` / `Runner.Join`) | `30E3B3A7` | U2 (`021.002-T`), U5 (`021.005-T`) | `go test -race ./internal/browser/...` |
| R4: Add `MeetingOptions.DefaultAccount` (populated from `Options.DefaultAccount` in `NewLiveMeeting` when unset) and automatic Entra account-picker selection in `chromeMeeting.joinHeaded` and `chromeMeeting.awaitPreJoinOrAdmission` (covering headed and headless live `Join` and `Recap` flows) | `30E3B3A7` | U3 (`021.003-T`), U5 (`021.005-T`) | `go test -race ./internal/browser/...` |
| R5: Bind `--account` CLI flag on `root`, `join`, `captions`, `auth`, and `schedule`, forward `--account` in `inheritedQueueFlags`, and pass `cfg.DefaultAccount` into `browserOptions` and `defaultNewMeeting` | `30E3B3A7` | U4 (`021.004-T`) | `go test -race ./internal/cli/...` |
| R6: Update `README.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md` for `default_account` / `--account` and `internal/config` Grade `A` (>80% coverage) | `30E3B3A7`, `A368D60E` | U6 (`021.006-T`) | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |

## Implementation Units

### U1 (`021.001-T`) — Add `default_account` (`entra_account` / `account` aliases) to `internal/config` and raise package test coverage above 80%

* **Domain:** code (Go `internal/config`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/config/config.go`:
     * Add `DefaultAccount string \`yaml:"default_account,omitempty"\`` to
       `Config`.
     * In `Load(path string)`, extend the `aliases` struct with
       `EntraAccount string \`yaml:"entra_account"\`` and
       `Account string \`yaml:"account"\``.
     * Resolve `cfg.DefaultAccount = strings.TrimSpace(cfg.DefaultAccount)`;
       when `cfg.DefaultAccount == ""`, fall back to
       `strings.TrimSpace(aliases.EntraAccount)` and then
       `strings.TrimSpace(aliases.Account)`.
  2. In `internal/config/config_test.go`:
     * Add table-driven test `TestLoadDefaultAccountAndAliases` covering
       `default_account`, `entra_account`, `account`, precedence
       (`default_account` > `entra_account` > `account`), whitespace trimming,
       and `Save`/`Load` round-trip.
     * Add table-driven test `TestLoadAliasAndOutputPrecedence` covering:
       - `output` ending in `.md` or `.markdown` setting `OutputFile` and
         overriding `output_file` in the same file;
       - `output` not ending in `.md`/`.markdown` setting `OutputDir` and
         overriding `output_folder` and `default_path`;
       - `output_folder` overriding `default_path` and `output_dir`;
       - `headed: true` forcing `Headless = false` even when `headless: true` is
         present; `headless: false` setting `Headless = false`; omitting both
         `headless` and `headed` in `Load` defaulting `Headless = false`;
       - `audio: true` and `audio: false` overriding `capture_audio`;
       - `name_template` used when `filename_template` is omitted, while an
         explicit custom `filename_template` wins over `name_template`, and an
         explicit empty `filename_template: ""` restores `DefaultFilenameTemplate`;
       - `url` alias used when `meeting_url` is omitted, while explicit
         `meeting_url` wins over `url`.
     * Add unit tests for `expandUserPath` (`""`, `~/transcripts`, custom
       `%CUSTOM_TEAMS_DIR%` via `t.Setenv`, unknown `%UNSET_TEAMS_VAR%` left
       intact) and `defaultProfileDir` when `LOCALAPPDATA` is empty
       (`t.Setenv("LOCALAPPDATA", "")`), raising `internal/config` coverage
       above 80%.
* **Files (2 files):** `internal/config/config.go`,
  `internal/config/config_test.go`.
* **Verify:** `go test -race -cover ./internal/config/...` passes with coverage
  `> 80.0%`.
* **Depends on:** none.

### U2 (`021.002-T`) — Add `Options.DefaultAccount`, `selectEntraAccountJS`, `redactAccountIdentifier`, and `ChromeRunner` Entra account auto-selection

* **Domain:** code (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/browser/allocator.go`:
     * Add `DefaultAccount string` to `browser.Options`.
  2. In `internal/browser/prejoin.go`:
     * Add `redactAccountIdentifier(account string) string` that masks
       configured UPNs/emails and display names before logging (for example,
       `alice@contoso.com` -> `a***@contoso.com`, or first rune + `***` when no
       `@` domain is present, and `""` when blank) so raw account identifiers
       are never emitted to logs.
     * Add `selectEntraAccountJS(account string) string`:
       - Trims `account`; if empty, returns `"false"`.
       - Encodes the lowercase trimmed target string with `json.Marshal` so
         special characters or quotes in config never break JS syntax or allow
         injection.
       - Evaluates in-page: returns `false` immediately if a visible password
         input (`input[type="password"]:not([aria-hidden="true"])` with non-zero
         bounding box) is focused/active on a password-entry step without
         account tiles.
       - Queries Microsoft Entra ID (`login.microsoftonline.com`) and Microsoft
         account (`login.live.com`) picker tile selectors:
         `#tilesHolder [data-test-id], div.table[role="button"][data-test-id], div.table[role="button"], div[data-bind*="acctTile"], [data-testid="accountTile"], div[role="listitem"] button[data-test-id]`.
       - Skips "Use another account" tiles (`id === "otherTile"`,
         `data-test-id === "otherTile"`, or `data-bind` containing `"otherTile"`).
       - Matches a tile case-insensitively using **exact, unambiguous matching**
         (never arbitrary substring containment, so `alice@contoso.com` never
         matches `bob.alice@contoso.com` and `"John"` never matches
         `"Johnathan"`):
         (a) `el.getAttribute("data-test-id")` (trimmed, lowercase) equals the
         target account exactly, OR
         (b) any trimmed line or child text node (`[data-test-id]`,
         `.table-cell`, `small`, `div`) equals the target account exactly.
       - **Fail-closed on ambiguity**: Collect all matching candidate tiles on
         the page first; if `matches.length !== 1` (either `0` matches or `> 1`
         ambiguous tiles match the configured identifier), do not click anything
         and return `false`.
       - Marks the single matched tile with a transient DOM dataset guard
         (`el.dataset.teamsCliAutoSelected = String(Date.now())`, skipping if
         clicked within the last `3000ms`) before invoking `el.click()`, and
         returns `true`.
  3. In `internal/browser/runner.go`:
     * In `ChromeRunner.run(ctx, opts, target)`, when
       `strings.TrimSpace(opts.DefaultAccount) != ""`, run a periodic poll loop
       (every `1 * time.Second` until `browserCtx.Done()`) that evaluates
       `selectEntraAccountJS(opts.DefaultAccount)` on `browserCtx` and, when it
       returns `true`, logs
       `slog.Info("browser: selected default Entra account", "account", redactAccountIdentifier(opts.DefaultAccount))`.
* **Files (3 files):** `internal/browser/allocator.go`,
  `internal/browser/prejoin.go`, `internal/browser/runner.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** none.

### U3 (`021.003-T`) — Wire `DefaultAccount` Entra account auto-selection into `chromeMeeting` live `Join` and `Recap` flows

* **Domain:** code (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `medium`.
* **What:**
  1. In `internal/browser/meeting.go`:
     * Add `DefaultAccount string` to `browser.MeetingOptions`.
     * In `NewLiveMeeting(ctx, opts, mopts)`, if
       `strings.TrimSpace(mopts.DefaultAccount) == ""` and
       `strings.TrimSpace(opts.DefaultAccount) != ""`, copy
       `mopts.DefaultAccount = strings.TrimSpace(opts.DefaultAccount)`.
  2. In `internal/browser/meeting_chrome.go`:
     * Add helper `m.defaultAccount() string` returning the trimmed
       `m.opts.DefaultAccount` (falling back to `m.headedOpts.DefaultAccount`
       when `m.headedOpts != nil`).
     * In `joinHeaded(ctx)` (which drives both headed live meeting joins and
       headed Meeting Recap exports): when `isAuthRedirectURL(mainPage.URL)` is
       true (and also when `mainPage.URL` is on an account picker page) and
       `m.defaultAccount() != ""`, evaluate
       `selectEntraAccountJS(m.defaultAccount())` via
       `evalInCDPTarget(ctx, mainPage.WebSocketDebuggerURL, selectEntraAccountJS(m.defaultAccount()))`.
       When it returns `"true"`, log
       `slog.Info("browser: selected default Entra account", "account", redactAccountIdentifier(m.defaultAccount()))`
       and reset `authTracker.settledPolls = 0` and `authTracker.rapidAuthBounces = 0`
       so selecting an account tile is never mistaken for an OAuth bounce loop.
     * In `awaitPreJoinOrAdmission(ctx)` (which drives headless live `Join` and
       headless `Recap` flows as well as headed post-attach pre-join): in the
       `PreJoinWaitingForUser` case, when `m.defaultAccount() != ""`, evaluate
       `selectEntraAccountJS(m.defaultAccount())` via `chromedp.Run` and, when
       `clicked` is `true`, log with `redactAccountIdentifier(m.defaultAccount())`
       and reset `coldRecapPolls = 0`.
* **Files (2 files):** `internal/browser/meeting.go`,
  `internal/browser/meeting_chrome.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** U2 (`021.002-T`).

### U4 (`021.004-T`) — Bind `--account` CLI flag across `root`, `join`, `captions`, `auth`, `schedule`, and `inheritedQueueFlags`

* **Domain:** code (Go `internal/cli`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/cli/root.go`:
     * In `browserOptions(c config.Config) browser.Options`, set
       `DefaultAccount: c.DefaultAccount`.
     * In `defaultNewMeeting(ctx, c, meetingURL)`, set
       `DefaultAccount: c.DefaultAccount` on `browser.MeetingOptions`.
     * Add `account string` to `browserFlags`, bind `--account` in
       `bindBrowserFlags` (`"default Microsoft Entra account (UPN/email or display name) to auto-select"`),
       and apply `if strings.TrimSpace(f.account) != "" { cfg.DefaultAccount = strings.TrimSpace(f.account) }`
       in `applyBrowserFlags` (automatically covering both `auth` and
       `schedule`).
  2. In `internal/cli/join.go`:
     * Add `account string` to `captureFlags`, bind `--account` in
       `bindCaptureFlags` (covering `root`, `join`, and `captions`), apply it in
       `applyCaptureFlags`, and forward `"--account", f.account` when non-empty
       in `inheritedQueueFlags`.
  3. In `internal/cli/root_test.go`:
     * Add unit tests verifying `--account` flag override over `config.yaml`
       `default_account` on `root`, `join`, `captions`, `auth`, and `schedule`,
       `browserOptions(cfg).DefaultAccount` propagation, and `--account`
       forwarding in `inheritedQueueFlags`.
* **Files (3 files):** `internal/cli/root.go`, `internal/cli/join.go`,
  `internal/cli/root_test.go`.
* **Verify:** `go test -race ./internal/cli/...` passes.
* **Depends on:** U1 (`021.001-T`), U2 (`021.002-T`), U3 (`021.003-T`).

### U5 (`021.005-T`) — Add unit and synthetic-DOM tests for Entra account-picker selection and identifier redaction in `internal/browser`

* **Domain:** tests (Go `internal/browser`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. In `internal/browser/prejoin_test.go`:
     * Add table-driven tests for `redactAccountIdentifier` (standard UPN
       `alice@contoso.com` -> `a***@contoso.com`, single-char local part
       `a@contoso.com` -> `a***@contoso.com`, display name `Alice Smith` ->
       `A***`, empty/whitespace -> `""`).
     * Add tests for `selectEntraAccountJS` verifying safe JSON escaping of
       quotes/backslashes, `"false"` on empty account, and expected selector and
       cooldown logic.
  2. In `internal/browser/meeting_test.go`:
     * Add tests verifying `NewLiveMeeting` propagates `Options.DefaultAccount`
       into `chromeMeeting` when `MeetingOptions.DefaultAccount` is empty, and
       synthetic-DOM / chromedp evaluation tests proving `selectEntraAccountJS`
       clicks the matching `[data-test-id="user@contoso.com"]` or display-name
       `.table[role="button"]` tile case-insensitively, ignores `#otherTile`
       ("Use another account"), does not re-click within the 3000ms cooldown
       window, and returns `false` when no matching account tile exists.
* **Files (2 files):** `internal/browser/prejoin_test.go`,
  `internal/browser/meeting_test.go`.
* **Verify:** `go test -race ./internal/browser/...` passes.
* **Depends on:** U2 (`021.002-T`), U3 (`021.003-T`).

### U6 (`021.006-T`) — Update `README.md`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md` for `default_account` / `--account` and `internal/config` Grade `A`

* **Domain:** docs (Markdown). **Posture:** migration-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. In `README.md`:
     * Document `--account` under `Flags for the root command, captions, and join`
       (and note that `auth` and `schedule` also accept `--account` alongside
       `--browser-path` and `--profile-dir`), add `--account` to the
       `inheritedQueueFlags` list in `### Batch queue mode`, and add
       `default_account` (with aliases `entra_account` and `account`) to the
       `## Configuration` YAML example and key/alias table.
  2. In `docs/ARCHITECTURE.md`:
     * Update `### Interactive auth`, `### Live join and captions`,
       `### Batch queue` (`inheritedQueueFlags`), and `## Security and isolation invariants`
       to document automatic Entra account-picker selection when
       `default_account` / `--account` is configured and UPN redaction in logs.
  3. In `docs/QUALITY_SCORE.md`:
     * Update `internal/config` in `## Grades by package` from Grade `B` (69.7%)
       to Grade `A` (with the new `>80%` measured coverage and alias/precedence
       table test rationale) and mark item 2 in `## Improvement roadmap`
       completed.
* **Files (3 files):** `README.md`, `docs/ARCHITECTURE.md`,
  `docs/QUALITY_SCORE.md`.
* **Verify:** Stage changes with `git add` and run
  `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
* **Depends on:** U1 (`021.001-T`), U4 (`021.004-T`).

## Dependency Graph

```text
U1 (021.001-T: internal/config default_account & >80% coverage tests) ──┐
U2 (021.002-T: browser Options, selectEntraAccountJS, Runner) ──────────┼──> U4 (021.004-T: CLI --account flag & queue forwarding) ──> U6 (021.006-T: README, ARCHITECTURE, QUALITY_SCORE)
  └──> U3 (021.003-T: chromeMeeting headed/headless Join & Recap) ──────┤
         └──> U5 (021.005-T: browser unit & synthetic-DOM tests)        │
U1 ─────────────────────────────────────────────────────────────────────┘
```

## Decisions and Rationale

* **JSON-encoded target account in `selectEntraAccountJS`**: Using
  `json.Marshal(strings.ToLower(strings.TrimSpace(account)))` guarantees that
  any string in `config.yaml` or `--account` is embedded into JavaScript as a
  safe literal without quotes or script-injection hazards.
* **Per-element 3-second click cooldown (`dataset.teamsCliAutoSelected`)**:
  Prevents repeated `.click()` calls on the same account tile while Microsoft
  Entra's form submission or redirect is in flight.
* **Redacted account logging (`redactAccountIdentifier`)**: Satisfies
  Constitution Principle III and `strict-safety` (`ActionRisk: high` for
  authentication surfaces) by masking UPNs/emails (`a***@contoso.com`) before
  any `slog.Info` call.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library + existing `chromedp` only; zero new dependencies; clean `go vet` and `golangci-lint` |
| II: Test first | Raises `internal/config` coverage above 80% (`U1`) and adds unit + synthetic-DOM tests across `internal/browser` (`U5`) and `internal/cli` (`U4`) |
| III, IV: Secrets & containment | Account identifiers redacted in logs via `redactAccountIdentifier`; synthetic domains (`@contoso.com` / `@example.com`) used in all tests and docs |
| VI: Single responsibility | Six width-isolated tasks (`<= 3` files each, single domain `code`, `tests`, or `docs`) |
| VII, VIII: Strict safety | Auth-adjacent DOM interaction classified with `ActionRisk: high` in `## Plan Hardening` below |
| IX, X, XI: Persistence & merge history | Deliberation `010-DL`, plan, feature `021-F`, tasks `021.001-T`–`021.006-T`, and shipment `021-S` recorded with full stash traceability (`30E3B3A7`, `A368D60E`) |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: adds `default_account`
  (`entra_account`, `account` aliases) to `config.yaml`, `--account` CLI flag
  to `root`, `join`, `captions`, `auth`, and `schedule`, and `DefaultAccount`
  to `browser.Options` and `browser.MeetingOptions`.
* Security, auth, permission, or compliance-sensitive behavior — **present
  (`ActionRisk: high`)**: interacts with the Microsoft Entra authentication
  account-picker DOM (`login.microsoftonline.com` / `login.live.com`) and
  handles user account identifiers (UPN/email).
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: additive configuration field and optional CLI flag; empty
  `DefaultAccount` preserves existing behavior identically.
* External integration, operator checkpoint, or external dependency — **absent**:
  zero new third-party dependencies.
* High runtime, rollout, or rollback risk — **low**: gated on non-empty
  `DefaultAccount` and guarded by a 3-second per-tile click cooldown.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U2 and U3 evaluate JavaScript against Microsoft
Entra authentication pages (`login.microsoftonline.com` / `login.live.com`) to
select an account tile and log the selection event.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `docs/closure/2026-11-11-020-S-review-and-closure.md` (`A368D60E` provenance)

### Protected Invariants

1. **No-op when `DefaultAccount` is empty**: When `DefaultAccount` is unset or
   whitespace-only, `selectEntraAccountJS` is never evaluated and browser
   authentication behavior is 100% unchanged.
2. **Safe JS serialization of `DefaultAccount`**: `selectEntraAccountJS` MUST
   serialize the normalized target string with `json.Marshal` rather than raw
   string interpolation so arbitrary characters in `config.yaml` or `--account`
   cannot break JS syntax or inject script statements.
3. **Password / MFA / "Use another account" non-interference**:
   `selectEntraAccountJS` MUST ignore `#otherTile` / `[data-bind*="otherTile"]`
   and MUST NOT click anything when no matching account tile is present or when
   a tile was already clicked within the 3-second cooldown window.
4. **Redacted account identifiers in logs (`strict-safety`)**: Raw `DefaultAccount`
   strings MUST NEVER be written to `slog` or error messages; all log statements
   recording account selection MUST pass the value through
   `redactAccountIdentifier`.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Auto-select matching Microsoft Entra account picker tile (`[data-test-id="<upn>"]`, `.table[role="button"]`, `div[data-bind*="acctTile"]`) across `Runner`, `Join`, and `Recap` when `DefaultAccount` is non-empty, and log only `redactAccountIdentifier(account)` | high | Opt-in via `default_account` / `--account`; target string JSON-encoded via `json.Marshal`; `#otherTile` excluded; 3000ms per-tile click cooldown; raw UPN redacted in logs | Browser automatically selects the configured Entra account tile without stalling on multi-account profiles or leaking raw UPNs to logs |
| Add `default_account` (`entra_account` / `account` aliases) to `internal/config` and expand unit tests in `internal/config/config_test.go` | low | Pure additive YAML field and hermetic `t.TempDir()` unit tests | `internal/config` statement coverage exceeds 80% (`Grade A`) |

## Runtime Verification and Closure

* **Verification**:
  * `go test -race -cover ./internal/config/...` (verify statement coverage > 80%)
  * `go test -race ./internal/browser/...` and `go test -race ./internal/cli/...`
  * `go test -race ./...`
  * `go vet ./...` and `go vet -tags=integration ./...`
  * `golangci-lint run`
  * `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`
* **Rollback**: Revert the `021-S` merge commit on `main` or omit
  `default_account` / `--account`.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (inline rubric evaluation across all six selected personas).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants and `strict-safety`
  `ProposedAction` / `ActionRisk: high` table.

| Persona | Mode | Findings | Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — All 6 units satisfy the 2-hour rule (`<= 3` files per task), width isolation (`code`, `tests`, `docs`), test-first posture, and Principle III secret/PII redaction. | PASS |
| Go Reviewer (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Idiomatic Go 1.26, context-bounded polling in `Runner` and `chromeMeeting`, table-driven `testify` unit tests in `internal/config`, `internal/browser`, and `internal/cli`. | PASS |
| Scope Boundary Auditor (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Strictly covers `30E3B3A7` and `A368D60E`; does not touch `010-S` or deferred stash entries `70ABBB2C`, `D3A9CBD0`, `EC6D1CAF`. | PASS |
| Learnings Researcher (always-on) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Incorporates `020-S` closure learnings on `internal/config` alias interplay (`output` vs `output_file` vs `output_folder` vs `default_path`) and tracked-file secret scan hygiene. | PASS |
| Architecture Strategist (cross-model rubric) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — Preserves leaf-package isolation (`internal/config` imports no internal packages; `internal/cli` maps `cfg.DefaultAccount` into `browser.Options` and `browser.MeetingOptions`). | PASS |
| Security Lens Reviewer (auth & DOM surface) | Inline persona pass | `0 P0, 0 P1, 0 P2, 0 P3` — `selectEntraAccountJS` uses `json.Marshal` to prevent DOM/JS injection, excludes `#otherTile`, enforces a 3-second click cooldown, and redacts UPNs via `redactAccountIdentifier`. | PASS |

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
feature `021-F` and shipment `021-S`.
