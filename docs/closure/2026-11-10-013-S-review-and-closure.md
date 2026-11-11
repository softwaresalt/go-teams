---
title: Browser security, secret hygiene, and ExecPath configuration review and operational closure
date: 2026-11-10
feature: 013-F
shipment: 013-S
branch: feat/013-s-browser-security-secret-hygiene-and-execpath-configuration
status: ready
closure_status: READY
compaction_status: pending
---

## Release scope

Shipment `013-S` (`013-F` and `013.001-T` through `013.005-T`) hardens the
browser security posture of `teams-cli` and closes secret-hygiene gaps:

* `013.001-T`: `teams.ValidateMeetingURL` enforces an initial-URL allowlist
  (HTTPS on `teams.microsoft.com`, `teams.live.com`, `teams.cloud.microsoft`,
  `sharepoint.com`, `microsoft.com`, `office.com`, `sfx.ms`, `aka.ms`, and
  their dot-delimited subdomains), permits `http` or `https` only on loopback
  fixtures (`localhost`, `127.0.0.1`, `::1`), and rejects embedded userinfo.
  `ChromeRunner.Join` validates before any browser is allocated.
* `013.002-T`: `teams.RedactURL` and `teams.RedactArgs` reduce URLs to
  `scheme://host/path` (or `[invalid-url]` when a clean form cannot be
  guaranteed). Navigation errors, queue entry logs, and the `splitQueueArgs`
  unterminated-quote error no longer expose passcodes, userinfo, or fragments.
* `013.003-T`: `browser.Options.DisableSandbox` and `TEAMS_DISABLE_SANDBOX`
  (`1` or `true`) make the Chromium sandbox opt-out explicit. Both
  `StealthFlags` and `cleanHeadlessFlags` emit `no-sandbox` and
  `disable-setuid-sandbox` as `false` by default, overriding chromedp's
  root-user default.
* `013.004-T`: `auth` and `schedule` accept `--browser-path` and
  `--profile-dir`; a shared `browserOptions` mapping feeds both the runner and
  the meeting launcher so `browser_path` reaches `browser.Options.ExecPath` on
  every command.
* `013.005-T`: `.gitignore` gains `*.env`, and a guardrail test scans files
  enumerated from the Git index (`git ls-files -z`, then a pure-Go DIRC v2/v3/v4
  parser, failing closed when neither is readable) for tracked local env files,
  live Teams passcode URLs, `SKYPE-TOKEN` credential values, and bearer JWTs.

Source stash entries `EEA7F685`, `D2023898`, `A99EEABD`, `3084A83E`, and
`87CDB97A` fed this scope. Stash entry `DF1F461F` and shipments `010-S` and
`014-S` are out of scope and untouched.

## Review

Single-agent declared degradation (`TOOL_DEGRADED: subagent-dispatch`): the
multi-persona review rubric ran in-session against `git diff main...HEAD`
covering correctness, security, maintainability, Go conventions, and
schema/docs coupling.

| Area | Finding | Disposition |
|---|---|---|
| Correctness | `ValidateMeetingURL` is shared by `MutateJoinURL`, `NewMeeting`, and `ChromeRunner.Join`; existing e2e fixtures use `httptest` loopback and still pass | No action |
| Security | `RedactURL` fallback strips through the last `@` before the first `/`, then rejects non-numeric ports, so userinfo cannot survive a malformed parse | Covered by `TestRedactURLMalformedNeverLeaksUserinfo` |
| Security | A URL such as `https://user:123?x@host` parses to host `user`, port `123` (RFC 3986 authority ends at `?`); the redacted output reflects that parse and carries no query data | Test input removed; behavior is correct |
| Security | Post-navigation redirects inside Chromium are not intercepted | Documented residual risk (federated sign-in flows would break) |
| Maintainability | Sandbox opt-in is exposed through `Options` and an environment variable only; no config key or CLI flag | Deliberate, minimal surface per plan |
| Test quality | Sandbox effective-argument tests intercept the chromedp command with `ModifyCmdFunc` and a simulated root `NoSandbox` preset, so the false-flag override is exercised | No action |

Blocking findings: `P0=0, P1=0, P2=0, P3=0`. Outcome: `READY`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Format | `gofmt -l cmd internal` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` | PASS |
| Full build | `go build ./...` and `go build -o .\bin\teams-cli.exe .\cmd\teams-cli` | PASS |
| Secret-hygiene guardrail | `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`; parser cross-checked against `git ls-files` | PASS |
| CLI reference drift | `cli-reference-drift.yml` drift job is disabled in this repository; no generated reference to update | Not applicable |
| Backlog integrity | `backlogit sync` and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Reject non-allowlisted and credential-bearing meeting URLs at `ValidateMeetingURL` and `ChromeRunner.Join` | high | Initial-URL check only; loopback fixtures preserved; no in-browser interception | applied |
| Make `--no-sandbox` and `--disable-setuid-sandbox` default to `false` in both allocator flag sets | high | Operator opt-in via `DisableSandbox` or `TEAMS_DISABLE_SANDBOX` restores prior behavior for containers | applied |
| Redact URLs in navigation errors, queue logs, and queue parse errors | moderate | Fail-safe placeholder `[invalid-url]` rather than partial output | applied |
| Add `--browser-path` and `--profile-dir` to `auth` and `schedule` | low | Flags only override config when supplied | applied |
| Add `*.env` to `.gitignore` and a Git-index secret scan test | low | Test reads tracked files only, never echoes matched secrets, fails closed on missing index | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Unit and compiled-CLI contract | READY | All unit and e2e tests pass under the race detector |
| URL ingress allowlist | READY_WITH_CONDITIONS | Tenants using vanity or sovereign-cloud hosts outside the allowlist must be added in a follow-up; redirect interception remains a documented residual risk |
| Sandbox default | READY_WITH_CONDITIONS | Root or container runs that relied on chromedp's implicit `--no-sandbox` must now set `TEAMS_DISABLE_SANDBOX=1` |
| Secret hygiene | READY | Guardrail runs in `go test`; unconditional CI enforcement on docs-only PRs is owned by `014.002-T` in `014-S` |

## Monitoring and observation window

Owner: operator deploying the executable.
Duration: the first headed `auth` run and one headless `join` or `--queue` run
after deployment.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Valid meeting URL acceptance | CLI exit status and `slog` output | Teams, SharePoint, and loopback URLs launch normally | Any valid Teams URL rejected with `is not a trusted Microsoft Teams host` |
| Log redaction | `slog` output and error text | URLs appear as `scheme://host/path` only | Any passcode, userinfo, or fragment visible in logs |
| Browser launch under sandbox | Launch result in container or root environments | Chromium starts with the sandbox enabled | Launch failure that names the sandbox, unless `TEAMS_DISABLE_SANDBOX` is set |

## Rollback

Trigger rollback on valid meeting URLs being rejected or on Chromium failing to
launch in a supported environment. Revert the merge commit on `main` and
rebuild `teams-cli.exe` from the prior release commit. As a no-revert
mitigation, set `TEAMS_DISABLE_SANDBOX=1` for sandbox launch failures. No data
migration or profile changes are involved.

## Handoff

Related [decision](../decisions/2026-11-10-browser-security-and-hygiene-deliberation.md)
and [plan](../plans/2026-11-10-browser-security-and-hygiene-plan.md).
