---
title: Browser security posture, URL redaction, host allowlist, secret hygiene, and ExecPath configuration decision
date: 2026-11-10
status: accepted
deliberation_id: 002-DL
feature: 013-F
shipment: 013-S
source_stash:
  - EEA7F685
  - D2023898
  - A99EEABD
  - 3084A83E
  - 87CDB97A
---

## Problem frame

During the staging and review of shipment `010-S` (PR #27, plan
`docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md`), five
pre-existing production security, secret-hygiene, and deployment-portability
gaps were captured as `DEFERRED SCOPE EXPANSION` stash entries under P-021 C2:

1. **`EEA7F685`** (`high`, `task`): Browser navigation errors in
   `internal/browser/runner.go` (`fmt.Errorf("browser: navigate to %s: %w", target, err)`)
   and queue execution logs in `internal/cli/queue.go` can emit raw meeting URLs
   containing query strings (`?p=<passcode>`, `code=`, `id_token=`,
   `access_token=`) or OAuth URL fragments into terminal or log output.
2. **`D2023898`** (`medium`, `task`): `StealthFlags(opts Options)` in
   `internal/browser/allocator.go` unconditionally sets `no-sandbox: true` and
   `disable-setuid-sandbox: true`. While `NewLiveMeeting` (`011-S`) uses clean
   flags with the Chromium sandbox enabled, `ChromeRunner` (`teams-cli auth` and
   `Runner.Join`) and `AllocatorOptions` still disable the Chromium sandbox by
   default.
3. **`A99EEABD`** (`medium`, `feature`): `teams.ValidateMeetingURL` in
   `internal/teams/url.go` accepts any `http://` or `https://` URL with a
   non-empty host, allowing arbitrary remote HTTP(S) navigation instead of
   constraining remote targets to trusted Microsoft Teams, SharePoint, and
   Microsoft 365 HTTPS domains (plus local loopback HTTP/HTTPS fixtures used by
   end-to-end and compiled-binary tests).
4. **`3084A83E`** (`medium`, `task`): Although `.gitignore` ignores `.env` and
   `.env.*` (while keeping `!.env.example`), it does not yet include `*.env`
   wildcard coverage, and the repository lacks an automated, deterministic
   pure-Go test guardrail that verifies `.gitignore` secret-file coverage and
   scans tracked repository files for accidental live Teams passcode or token
   markers (`?p=` on real meeting URLs, raw `SKYPE-TOKEN` JWTs) without
   embedding any secret material.
5. **`87CDB97A`** (`medium`, `feature`): Production browser executable-path
   (`ExecPath`) portability. While `011-S` added `ResolveExecPath` in
   `internal/browser/allocator.go`, `BrowserPath` (`browser_path`) in
   `internal/config/config.go`, and `--browser-path` on `captureFlags` (`root`,
   `join`, and `captions`), the `auth` (`internal/cli/auth.go`) and `schedule`
   (`internal/cli/schedule.go`) subcommands still do not bind `--browser-path`
   (or `--profile-dir`) flags, and `internal/cli/root_test.go` and
   `internal/config/config_test.go` lack end-to-end unit coverage verifying that
   `browser_path` YAML round-tripping and `--browser-path` CLI overrides flow
   into `browser.Options.ExecPath` across every CLI command.

## P-021 C5 and C6 triage and reconciliation record

All five stash entries carry the `DEFERRED SCOPE EXPANSION` marker, which forces
routing through Step 2 deliberation (`002-DL`) prior to implementation planning
(P-021 C6). Both separately triggered Step 1 obligations were executed:

* **(A) Unconditional duplicate detection**: Executed across all active stash
  entries (`.backlogit/stash.jsonl`), active queue items (`.backlogit/queue/`),
  and archived items (`.backlogit/archive/`). Outcome: **CLEAN DUPLICATE SCAN**
  for all five entries (`EEA7F685`, `D2023898`, `A99EEABD`, `3084A83E`,
  `87CDB97A`) — no duplicate stash entries exist. Partial implementation overlap
  from `011-S` on `87CDB97A` was identified in codebase inspection and used to
  narrow `87CDB97A`'s remaining implementation scope without duplicating work.
* **(B) Late-identifier reconciliation**: Executed for every entry carrying any
  `N/A` source reference against `docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md`
  (`Residual Review Items — R1..R5` and `Capture disposition`) and
  `.backlogit/queue/010-S.md`:

| Stash ID | Originating PR | Review-thread IDs | Task / Feature / Shipment | Duplicate scan (A) | Late-identifier reconciliation (B) |
|---|---|---|---|---|---|
| `EEA7F685` | PR #27 | `PRRT_kwDOTStvas6foPdV`, `PRRT_kwDOTStvas6foPd-` | `task: N/A`, `010-F`, `010-S` | Clean duplicate scan | `task ID: N/A` — no late identifier found (pre-existing production-source condition R3, not tied to a single `010-F` task) |
| `D2023898` | PR #27 | `PRRT_kwDOTStvas6foPdV`, `PRRT_kwDOTStvas6foPd-` | `task: N/A`, `010-F`, `010-S` | Clean duplicate scan | `task ID: N/A` — no late identifier found (pre-existing production-source condition R2, not tied to a single `010-F` task) |
| `A99EEABD` | PR #27 | `PRRT_kwDOTStvas6foPdV`, `PRRT_kwDOTStvas6foPd-` | `task: N/A`, `010-F`, `010-S` | Clean duplicate scan | `task ID: N/A` — no late identifier found (pre-existing production-source condition R4, not tied to a single `010-F` task) |
| `3084A83E` | PR #27 | `PRRT_kwDOTStvas6foPdV`, `PRRT_kwDOTStvas6foPd-` | `task: N/A`, `010-F`, `010-S` | Clean duplicate scan | `task ID: N/A` — no late identifier found (pre-existing repository-hygiene condition R5, not tied to a single `010-F` task) |
| `87CDB97A` | Reconciled: PR #27 (was `N/A`) | `N/A` (referenced alongside `PRRT_kwDOTStvas6foPdV`, `PRRT_kwDOTStvas6foPd-`) | `010.007-T`, `010-F`, `010-S` | Clean duplicate scan | Reconciled `PR number: 27` from `docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md` (R1 / Capture disposition); `review-thread ID: N/A` — no late identifier found (originated in local Scope Boundary Auditor pass on `010.007-T`) |

## Requirements and success criteria

* **R1 (Host allowlist and loopback fixture boundary — `A99EEABD`)**:
  `teams.ValidateMeetingURL` must reject arbitrary untrusted remote hosts and
  plain `http://` on non-loopback hosts. It must allow `https://` URLs whose
  hostname matches or is a dot-subdomain of trusted Microsoft Teams / Microsoft
  365 domains (`teams.microsoft.com`, `teams.live.com`, `teams.cloud.microsoft`,
  `sharepoint.com`, `microsoft.com`, `office.com`) and must allow `http://` or
  `https://` on loopback hosts (`127.0.0.1`, `localhost`, `::1`) so
  `TestCaptionsCommandEndToEndWritesMarkdownFile` (`internal/cli/e2e_test.go`)
  and `TestCompiledBinaryCapturesRecapToExplicitOutput`
  (`internal/cli/binary_automation_test.go`) continue to pass against local
  `httptest.Server` fixtures.
* **R2 (URL query and fragment redaction — `EEA7F685`)**: Provide a
  deterministic `teams.RedactURL(raw string) string` helper that strips query
  parameters (`?p=`, `code=`, etc.), userinfo, and URL fragments (returning
  `scheme://host/path` or `[redacted-url]` on unparseable strings containing
  query/fragment separators) and apply it to navigation error wrapping in
  `internal/browser/runner.go` and `internal/browser/meeting_chrome.go` as well
  as queue argument logging in `internal/cli/queue.go`.
* **R3 (Opt-in Chromium sandbox disable — `D2023898`)**: Add
  `DisableSandbox bool` to `browser.Options` (and honor `TEAMS_DISABLE_SANDBOX=1`
  or `true` when `Options.DisableSandbox` is false) so `StealthFlags(opts Options)`
  in `internal/browser/allocator.go` omits `no-sandbox` and
  `disable-setuid-sandbox` by default (sandbox enabled in production) and emits
  them only when explicitly opted in.
* **R4 (Complete `ExecPath` / `--browser-path` CLI and config wiring — `87CDB97A`)**:
  Bind `--browser-path` (and `--profile-dir`) on `auth` (`internal/cli/auth.go`)
  and `schedule` (`internal/cli/schedule.go`) so every CLI command supports
  explicit browser executable overrides, preserve `BrowserPath` in
  `config.SaveLoadRoundTrip` tests, and add unit tests in
  `internal/cli/root_test.go` verifying that `config.Config.BrowserPath` and
  `--browser-path` flow into `browser.Options.ExecPath` across `root`, `join`,
  `captions`, `auth`, and `schedule`.
* **R5 (Repository secret-hygiene and `.gitignore` guardrail — `3084A83E`)**:
  Ensure `.gitignore` covers `.env`, `.env.*`, and `*.env` (while keeping
  `!.env.example` tracked) and add a deterministic, pure-Go guardrail test in
  `internal/config/gitignore_test.go` that verifies `.gitignore` local-env
  patterns and scans tracked files in the repository for accidental live Teams
  meeting passcode URLs or raw token artifacts without embedding any secret.

## Options

| Option | Benefit | Limitation |
|---|---|---|
| Option A: Separate single-entry shipments for each stash item | Smallest possible per-PR diff | Splits tightly coupled `internal/teams` and `internal/browser` trust-boundary changes across five sequential PRs |
| Option B (chosen): Cohesive browser security, secret-hygiene, and `ExecPath` feature (`013-F` / `013-S`) with 5 width-isolated tasks | Aligns URL validation, URL redaction, allocator sandbox posture, CLI `ExecPath` plumbing, and repository secret-hygiene guardrails in one coherent release unit | Requires careful dependency ordering where tasks touch `internal/teams` and `internal/browser` |
| Option C: Document arbitrary HTTP(S) navigation and unconditional `--no-sandbox` as intentional | Zero code change in `internal/teams/url.go` and `internal/browser/allocator.go` | Leaves SSRF/untrusted-host navigation open and runs `ChromeRunner` without Chromium sandbox protection by default |

## Chosen direction

We select **Option B** and synthesize covering feature **`013-F`**
(*Browser security posture, URL redaction, host allowlist, secret hygiene, and
ExecPath configuration*) packaged in shipment **`013-S`** (with explicit
blocking predecessor `012-S`). The work is decomposed into five atomic tasks,
each well under the 2-hour rule:

1. **`013.001-T` (U1)**: Enforce trusted Microsoft Teams / SharePoint /
   Microsoft 365 HTTPS host allowlist and loopback HTTP/HTTPS fixture support in
   `teams.ValidateMeetingURL` (`internal/teams/url.go`,
   `internal/teams/url_test.go`).
2. **`013.002-T` (U2)**: Add `teams.RedactURL` in `internal/teams/url.go` and
   redact URLs in navigation errors and queue logs (`internal/browser/runner.go`,
   `internal/browser/meeting_chrome.go`, `internal/cli/queue.go`, plus unit
   tests).
3. **`013.003-T` (U3)**: Make `--no-sandbox` and `--disable-setuid-sandbox`
   opt-in in `browser.StealthFlags` via `Options.DisableSandbox` and
   `TEAMS_DISABLE_SANDBOX` (`internal/browser/allocator.go`,
   `internal/browser/allocator_test.go`).
4. **`013.004-T` (U4)**: Complete `--browser-path` and `BrowserPath` (`ExecPath`)
   wiring across `auth` and `schedule` CLI commands and add config/CLI unit
   tests (`internal/cli/auth.go`, `internal/cli/schedule.go`,
   `internal/cli/root_test.go`, `internal/config/config_test.go`).
5. **`013.005-T` (U5)**: Add `*.env` to `.gitignore` and add a pure-Go
   `.gitignore` and tracked-file secret-hygiene guardrail test
   (`.gitignore`, `internal/config/gitignore_test.go`).

## Open questions

None blocking planning or harvest. Loopback (`127.0.0.1`, `localhost`, `::1`)
is explicitly allowed over `http://` and `https://` in `ValidateMeetingURL` so
existing local `httptest.Server` e2e and binary acceptance tests require no
weakening of production remote host rules.

## Related artifacts

* [Implementation plan](../plans/2026-11-10-browser-security-and-hygiene-plan.md)
* [010-S selector and join-path plan](../plans/2026-09-05-teams-live-selector-path-hardening-plan.md)
* [012-S review and closure](../closure/2026-11-10-012-S-review-and-closure.md)
