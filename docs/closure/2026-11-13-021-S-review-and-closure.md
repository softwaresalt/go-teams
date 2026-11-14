---
title: Default Microsoft Entra account auto-selection and internal/config test hardening review and operational closure
date: 2026-11-13
feature: 021-F
shipment: 021-S
branch: feat/021-s-default-microsoft-entra-account-auto-selection-and-internal-config-test-coverage-hardening
status: closed
closure_status: READY
compaction_status: done
merge_commit: f1c9dd3807e8ff048e1b93005ec80b32af5ad115
pull_request: 53
---

## Release scope

Shipment `021-S` (`021-F`, `021.001-T`..`021.006-T`) implements automatic
Microsoft Entra account-picker selection across `Runner` (`auth`), live `Join`,
and `Recap` browser flows and raises `internal/config` unit-test statement
coverage from `69.7%` to `83.3%` (`Grade A` in `docs/QUALITY_SCORE.md`). It is
planned under deliberation `010-DL` and consumes stash entries `30E3B3A7` and
`A368D60E`.

* `021.001-T` (`internal/config/config.go`, `internal/config/config_test.go`,
  commits `3a22f09`, `4f03532`):
  * Adds `DefaultAccount string` (`yaml:"default_account,omitempty"`) to
    `config.Config` and resolves `default_account`, `entra_account`, and
    `account` with deterministic precedence (`default_account` > `entra_account`
    > `account`) and `strings.TrimSpace` normalization.
  * Adds `TestLoadDefaultAccountAndAliases`, `TestLoadAliasAndOutputPrecedence`,
    and `TestExpandUserPathAndDefaultProfileDir`, raising `internal/config`
    statement coverage from `69.7%` to `83.3%`.
* `021.002-T` (`internal/browser/allocator.go`, `internal/browser/prejoin.go`,
  `internal/browser/runner.go`, commits `16f5c9a`, `809e55d`, `4f03532`,
  `c200155`):
  * Adds `DefaultAccount string` to `browser.Options`.
  * Adds `RedactAccountIdentifier` / `redactAccountIdentifier` (`a***@contoso.com`
    and `A***`) and `selectEntraAccountJS` in `internal/browser/prejoin.go`.
  * Wires periodic account-picker polling into `ChromeRunner.run`.
* `021.003-T` (`internal/browser/meeting.go`,
  `internal/browser/meeting_chrome.go`, commit `922910c`):
  * Adds `DefaultAccount string` to `browser.MeetingOptions` and propagates
    `Options.DefaultAccount` in `NewLiveMeeting`.
  * Wires `selectEntraAccountJS` into `chromeMeeting.joinHeaded` and
    `chromeMeeting.awaitPreJoinOrAdmission` (`PreJoinWaitingForUser`).
* `021.004-T` (`internal/cli/root.go`, `internal/cli/join.go`,
  `internal/cli/auth.go`, `internal/cli/schedule.go`, `internal/cli/queue.go`,
  `internal/cli/root_test.go`, `internal/cli/queue_test.go`, commits `fd1482c`,
  `4f03532`):
  * Binds `--account` on `root`, `join`, `captions`, `auth`, and `schedule`,
    honoring `cmd.Flags().Changed("account")` so `--account=` clears a
    configured `default_account`.
  * Forwards `--account` (and empty `--account=`) in `inheritedQueueFlags` and
    redacts `--account` values in `sanitizeQueueArgsForLog`.
* `021.005-T` (`internal/browser/prejoin_test.go`,
  `internal/browser/meeting_test.go`, commits `361dbb6`, `809e55d`, `4f03532`,
  `c200155`, `7d91a60`, `7ea71c9`):
  * Unit tests for `redactAccountIdentifier`, `selectEntraAccountJS`, and
    `NewLiveMeeting` `DefaultAccount` propagation, plus headless Chromium
    synthetic-DOM tests in `TestSelectEntraAccountJSDOM`.
* `021.006-T` (`README.md`, `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`,
  commit `e622da9`):
  * Documents `default_account` (`entra_account`, `account`), `--account`,
    batch queue forwarding, and UPN log redaction, and upgrades
    `internal/config` from Grade `B` (69.7%) to Grade `A` (83.3%).

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`,
`dispatch_mode: single-agent-declared-degradation`,
`consensus_status: DEGRADED_SINGLE_AGENT (non-consensus)`, `mode: report-only`
posture, permanent P-012 Item 5 contract under `008-DL` / `019-F`), followed by
GitHub Copilot shadow review on PR #53 (`P-018` `copilot-review` gate
`SATISFIED`).

| Area | Finding | Disposition |
|---|---|---|
| Correctness / Security (local review) | `selectEntraAccountJS` checked `document.querySelector('#tilesHolder') === null` before honoring visible password inputs, allowing clicks if `#tilesHolder` remained in the DOM during password entry | Fixed in-scope (P-021 C1); commit `809e55d` |
| Security (Copilot thread `PRRT_kwDOTStvas63ZSmv`) | Document-wide tile selectors could match generic buttons outside a Microsoft authentication origin | Fixed in-scope (P-021 C1): `selectEntraAccountJS` requires a trusted Microsoft login origin (`authHosts`) and a visible picker surface (`#tilesHolder`, `acctTile`, `accountTile`, `otherTile`); commits `4f03532`, `c200155` |
| Correctness / Security (Copilot thread `PRRT_kwDOTStvas63ZSm3`) | Challenge guard checked password inputs only and did not exclude active MFA one-time-code controls | Fixed in-scope (P-021 C1): added MFA challenge selectors (`input[name="otc"]`, `input[autocomplete="one-time-code"]`, `#idTxtBx_SAOTCC_OTC`, `#idDiv_SAOTCS_Proofs`, `#idRichContext_DisplaySign`, `[data-testid="displaySign"]`) and regression test; commit `4f03532` |
| Correctness (Copilot thread `PRRT_kwDOTStvas63ZSnG`) | Computed-style check on tile alone did not reject tiles beneath a `display:none` ancestor | Fixed in-scope (P-021 C1): added `isVisible(node)` checking nonzero bounding box and ancestor styles/attributes, plus hidden-ancestor regression test; commit `4f03532` |
| Security (Copilot thread `PRRT_kwDOTStvas63ZSnl`) | `sanitizeQueueArgsForLog` in `internal/cli/queue.go` redacted URLs but did not mask `--account` or `--account=` values in queue logs | Fixed in-scope (P-021 C1): masked both forms via `browser.RedactAccountIdentifier` and added `TestSanitizeQueueArgsForLogRedactsAccountFlag`; commit `4f03532` |
| Correctness (Copilot thread `PRRT_kwDOTStvas63ZSn8`) | Explicit empty `--account=` did not clear a configured `default_account` or survive `inheritedQueueFlags` | Fixed in-scope (P-021 C1): checked `cmd.Flags().Changed("account")` in `applyBrowserFlags`, `applyCaptureFlags`, and `inheritedQueueFlags` with regression tests; commit `4f03532` |
| Security (Copilot thread `PRRT_kwDOTStvas63ZaUf`) | `.windows.net` suffix in `isTrustedAuthOrigin` was broader than `login.windows.net` | Fixed in-scope (P-021 C1): restricted `authHosts` to exact Microsoft login hosts and their dot-delimited subdomains; commit `c200155` |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

### Review record

| Field | Value |
|---|---|
| Reviewed diff identity | `main..7ea71c99927246c0416cf6ef6cc5be88c3642666` (PR #53) |
| Reviewed HEAD | `7ea71c99927246c0416cf6ef6cc5be88c3642666` |
| `dispatch_mode` | `single-agent-declared-degradation` |
| `confidence_tier` | `DEGRADED_SINGLE_AGENT` |
| `consensus_status` | `DEGRADED_SINGLE_AGENT (non-consensus)` |
| P-018 `copilot-review` gate | `SATISFIED` (`exit_code: 0`, `unresolved_thread_ids: []`) |

## Follow-ups

No new follow-up stash items were generated during `021-S`. Previously stashed
low-priority items (`70ABBB2C`, `D3A9CBD0`, `EC6D1CAF`) and queued live
verification shipment `010-S` remain untouched, and `022-S` is ready to run
next.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #53, merge commit `f1c9dd3807e8ff048e1b93005ec80b32af5ad115`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED`; `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Close path | P-015 verified fully-covered-root cascade (`021-F` is a root feature, and all six child tasks `021.001-T`..`021.006-T` are manifest members) |
| Command | `backlogit shipment ship 021-S --sha f1c9dd3807e8ff048e1b93005ec80b32af5ad115` |
| `archived_ids` | `021.001-T`, `021.002-T`, `021.003-T`, `021.004-T`, `021.005-T`, `021.006-T`, `021-F`, `021-S` |
| `returned_ids` | empty |
| `parent_id` preservation | `021.001-T`..`021.006-T` all retained `parent_id: 021-F` |
| Final records | `021-S`: `archived_status: shipped`; `021-F` and `021.001-T`..`021.006-T`: `archived_status: done` |
| Archive integrity (P-007) | No archive file reported as deleted |
| Topology gate | `lifecycle` passed before the close (`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stash `30E3B3A7` | Already archived by Stage at harvest; verified absent from active `backlogit stash list` | skipped |
| Stash `A368D60E` | Already archived by Stage at harvest; verified absent from active `backlogit stash list` | skipped |
| Deliberation `010-DL` | Zero remaining live references in `.backlogit/queue/`; archived via `backlogit archive 010-DL` | archived |

### Compaction (P-020)

`compact-context` ran at closure. It wrote
`docs/memory/compacted/2026-11-13-021-S-default-entra-account-selection-compacted.md`
and archived the verbose Ship memory record to
`docs/archive/memory/2026-11-13/021-S-default-entra-account-selection-memory.md`
while preserving `docs/memory/2026-11-13/stage-021-S-022-S-memory.md` (which
covers still-queued shipment `022-S`). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| CLI probe (`cli-help`) | `go run ./cmd/teams-cli --help` — lists `auth`, `captions`, `join`, `schedule`, `update` and `--account` flag; exit 0 | PASS |
| CLI build (`cli-build`) | `go build ./...` — exit 0 | PASS |
| Background-job probe (`scheduler-dry-run`) | `go run ./cmd/teams-cli schedule --help` — renders usage and `--account` flag without launching a browser; exit 0 | PASS |
| Static analysis | `go vet ./...` and `go vet -tags=integration ./...` | PASS |
| Secret hygiene | `go test -race ./internal/config/... -run TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| Full test suite & coverage | `go test -race -p 1 -cover ./...` (local Windows + GitHub Actions `ubuntu-latest` run `43564415661`): `version` 97.8%, `teams` 93.8%, `scheduler` 93.3%, `transcript` 91.2%, `audio` 87.5%, `updater` 84.5%, `config` 83.3%, `cli` 82.4%, `capture` 81.7%, `browser` 72.3% | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` ("No issues found.") | PASS |
| Manual checkpoint (`teams-auth-mfa`) | Live Microsoft Entra multi-account picker auto-selection and MFA non-interference against a real tenant have not been executed in this unattended session (`010-S` covers guest pre-join and treats login/MFA landings as blocked rather than testing `default_account` picker selection). Synthetic Chromium DOM tests in `TestSelectEntraAccountJSDOM` verified tile selection, ambiguity fail-closed, `#otherTile` exclusion, password/MFA non-interference, and hidden-ancestor rejection | PENDING (operator live-tenant verification; no dedicated follow-up task) |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Auto-select matching Microsoft Entra account picker tile across `Runner`, `Join`, and `Recap` when `DefaultAccount` is non-empty, and log only `RedactAccountIdentifier(account)` | high | Opt-in via `default_account` / `--account`; target string JSON-encoded via `json.Marshal`; trusted Microsoft login host + visible picker surface required; `#otherTile` and password/MFA screens excluded; 3000 ms per-tile click cooldown; raw UPN redacted in browser and queue logs | applied and verified |
| Add `default_account` (`entra_account` / `account` aliases) to `internal/config` and expand unit tests in `internal/config/config_test.go` | low | Pure additive YAML field and hermetic `t.TempDir()` unit tests | applied (`83.3%` statement coverage, Grade `A`) |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Healthy signal | READY | `go build ./...` and `go test -race ./...` pass locally and in CI |
| Failure signal | READY_WITH_CONDITIONS | Non-zero exit from CLI root or `schedule` daemon, or context deadline expiration (`browser: pre-join not visible: context deadline exceeded`) when a session stalls on `PreJoinWaitingForUser`. `selectEntraAccountJS` returning `false` is not logged directly because `false` also occurs normally when no picker is present or when password/MFA challenges are active; a stuck or ambiguous picker is therefore surfaced only by the absence of `browser: selected default Entra account` followed by a bounded session timeout |
| Monitoring plan | READY_WITH_CONDITIONS | Monitoring gap: callers log only successful clicks (`slog.Info("browser: selected default Entra account", "account", redacted)`), not unmatched/ambiguous picker polls. Operators must verify `browser: selected default Entra account` in logs during an initial supervised `auth` / `captions` run and enforce a bounded session/context timeout on unattended `schedule` runs to detect stalled authentication |
| Rollback trigger | READY | Unexpected account tile selection or authentication stall in interactive/headless login flows |
| Rollback procedure | READY | Omit `default_account` / pass `--account=`, or revert merge commit `f1c9dd3807e8ff048e1b93005ec80b32af5ad115` on `main` |
| Owner | READY | Operator supervising scheduled/interactive Teams capture |
| Validation window | READY_WITH_CONDITIONS | First operator-supervised multi-account `auth` / `captions` session and one full scheduled-meeting cycle |

## Handoff

Related [decision](../decisions/2026-11-13-default-entra-account-selection-deliberation.md)
(`010-DL`), [plan](../plans/2026-11-13-default-entra-account-selection-plan.md),
and graduated [capture design doc](../design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md).
Compaction (P-020) completed at post-merge closure; see the compacted
[memory summary](../memory/compacted/2026-11-13-021-S-default-entra-account-selection-compacted.md).
