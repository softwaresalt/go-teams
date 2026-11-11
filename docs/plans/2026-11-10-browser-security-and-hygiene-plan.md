---
title: Browser security posture, URL redaction, host allowlist, secret hygiene, and ExecPath configuration plan
description: Implementation plan for 013-F / 013-S covering navigation host allowlist, URL query/passcode redaction, opt-in Chromium sandbox disable, ExecPath CLI/config wiring across all subcommands, and repository secret-hygiene guardrails
source: docs/decisions/2026-11-10-browser-security-and-hygiene-deliberation.md
deliberation_id: 002-DL
feature: 013-F
shipment: 013-S
date: 2026-11-10
status: approved
tags:
  - security
  - browser
  - teams
  - config
  - cli
---

## Problem Frame

Five deferred scope expansion items from shipment `010-S` / PR #27 (`EEA7F685`,
`D2023898`, `A99EEABD`, `3084A83E`, and `87CDB97A`), deliberated in
[2026-11-10-browser-security-and-hygiene-deliberation.md](../decisions/2026-11-10-browser-security-and-hygiene-deliberation.md)
(`002-DL`), require hardening across `internal/teams`, `internal/browser`,
`internal/config`, `internal/cli`, and `.gitignore`:

1. `teams.ValidateMeetingURL` (`internal/teams/url.go`) currently accepts any
   `http://` or `https://` URL with a non-empty host (`http://example.com/x`),
   leaving remote navigation unconstrained instead of enforcing trusted
   Microsoft Teams / SharePoint / Microsoft 365 `https://` hosts while
   preserving `http://` and `https://` loopback (`127.0.0.1`, `localhost`,
   `::1`) for local `httptest.Server` e2e and binary acceptance fixtures.
2. `ChromeRunner.run` (`internal/browser/runner.go`), `chromeMeeting.Join`
   (`internal/browser/meeting_chrome.go`), and `runQueueFile`
   (`internal/cli/queue.go`) can surface raw URLs containing `?p=<passcode>`,
   `code=`, or `#` fragments in error messages or structured logs.
3. `StealthFlags(opts Options)` (`internal/browser/allocator.go`)
   unconditionally sets `no-sandbox: true` and `disable-setuid-sandbox: true`
   instead of defaulting to Chromium sandbox enabled and gating sandbox disable
   behind an explicit opt-in (`Options.DisableSandbox` or
   `TEAMS_DISABLE_SANDBOX`).
4. While `011-S` wired `ResolveExecPath`, `config.Config.BrowserPath`
   (`browser_path`), and `--browser-path` on `root`, `join`, and `captions`,
   the `auth` (`internal/cli/auth.go`) and `schedule`
   (`internal/cli/schedule.go`) subcommands still do not bind `--browser-path`
   (or `--profile-dir`), and unit tests do not yet verify `BrowserPath` /
   `--browser-path` propagation across all five CLI commands into
   `browser.Options.ExecPath`.
5. `.gitignore` covers `.env` and `.env.*` (`!.env.example`) but lacks `*.env`
   wildcard coverage and a deterministic pure-Go guardrail test in
   `internal/config/gitignore_test.go` verifying `.gitignore` secret rules and
   scanning tracked files for accidental live Teams passcode or token strings.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Trusted Microsoft/Teams/SharePoint/Office HTTPS host allowlist + loopback HTTP/HTTPS fixture support | `A99EEABD` | U1 (`013.001-T`) | `go test -race ./internal/teams/...` (`TestValidateMeetingURL`, `TestMutateJoinURLErrors`) and `./internal/cli/...` |
| R2: Redact query strings (`?p=`, `code=`) and fragments from navigation errors and logs | `EEA7F685` | U2 (`013.002-T`) | `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...` (`TestRedactURL`, `TestRunnerJoinErrorRedactsURL`, `TestQueueLogsRedactURL`) |
| R3: Default Chromium sandbox enabled in `StealthFlags`; opt-in via `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX=1` | `D2023898` | U3 (`013.003-T`) | `go test -race ./internal/browser/...` (`TestStealthFlagsDefaultSandboxEnabled`, `TestStealthFlagsOptInDisableSandbox`) |
| R4: Complete `--browser-path` / `browser_path` (`ExecPath`) CLI and config wiring across `auth`, `schedule`, `root`, `join`, `captions` | `87CDB97A` | U4 (`013.004-T`) | `go test -race ./internal/config/... ./internal/cli/...` (`TestSaveLoadRoundTrip`, `TestBrowserPathWiredAcrossAllCommands`) |
| R5: `.gitignore` `*.env` coverage + pure-Go repository secret-hygiene guardrail test | `3084A83E` | U5 (`013.005-T`) | `go test -race ./internal/config/...` (`TestGitignoreCoversLocalEnvAndSecretPatterns`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) |

## Implementation Units

### U1 (`013.001-T`) — Enforce trusted Teams/Microsoft HTTPS host allowlist and loopback fixture boundary in `ValidateMeetingURL`

* **Domain:** code (Go). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:** Update `ValidateMeetingURL` in `internal/teams/url.go` so that:
  1. Loopback hosts (`localhost`, `127.0.0.1`, `::1`, retrieved via
     `u.Hostname()`) are permitted over both `http` and `https` (preserving
     `httptest.NewServer` fixtures in `internal/cli/e2e_test.go` and
     `internal/cli/binary_automation_test.go`).
  2. Non-loopback hosts MUST use `https` (rejecting plain `http://` on remote
     hosts) and their lowercase `u.Hostname()` MUST match or be a dot-prefixed
     subdomain of an allowed Microsoft 365 / Teams / SharePoint host suffix:
     `teams.microsoft.com`, `teams.live.com`, `teams.cloud.microsoft`,
     `sharepoint.com`, `microsoft.com`, `office.com`, `sfx.ms`, `aka.ms`.
  3. Any other remote host (such as `http://example.com/x`,
     `https://example.com/x`, or `https://evil-teams.microsoft.com.attacker.example/x`)
     is rejected with an actionable error.
* **Files (2 files):** `internal/teams/url.go`, `internal/teams/url_test.go`.
* **Verify:** Update `TestValidateMeetingURL` in `internal/teams/url_test.go`
  to cover trusted HTTPS hosts (`teams.microsoft.com`, `teams.cloud.microsoft`,
  `teams.live.com`, `contoso.sharepoint.com`), loopback HTTP/HTTPS
  (`http://127.0.0.1:54321/meet/x`, `http://localhost:8080/l/meetingrecap`,
  `http://[::1]:9000/meet/x`), and rejected cases (`http://teams.microsoft.com/x`,
  `https://example.com/x`, `https://teams.microsoft.com.evil.example/x`); run
  `go test -race ./internal/teams/... ./internal/cli/...`.
* **Depends on:** none.

### U2 (`013.002-T`) — Add `teams.RedactURL` and scrub userinfo, query, and fragment tokens from navigation errors and CLI queue logs

* **Domain:** code (Go). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Add `RedactURL(raw string) string` (and `RedactArgs(args []string) string`)
     in `internal/teams/url.go`:
     * For a parseable HTTP/HTTPS URL with a non-empty host (`u.Opaque == ""`):
       clear `u.User = nil`, `u.RawQuery = ""`, `u.ForceQuery = false`,
       `u.Fragment = ""`, and `u.Opaque = ""` (keeping only normalized
       `scheme://host/path`).
     * For the malformed-URL / fallback path (when `url.Parse` fails, scheme is
       non-HTTP(S), `u.Opaque != ""`, or `u.Host == ""`): do NOT simply slice up
       to `?` or `#` (which would leave `https://user:secret@bad%zz/path` or
       `https://?p=secret` leaking userinfo or tokens). Instead, if the fallback
       cannot safely extract a clean `scheme://host/path` free of userinfo
       (`@`), query (`?`), and fragment (`#`), return a constant placeholder
       `"[invalid-url]"` (or strip any `userinfo@` before the host in addition
       to stripping `?query` and `#fragment`, and return `"[invalid-url]"` if
       any `@`, `?`, `#`, or control character remains).
     * Furthermore, error messages inside `ValidateMeetingURL` must never echo
       the unredacted raw URL.
  2. In `internal/browser/runner.go` (`ChromeRunner.run`), wrap `chromedp.Navigate`
     errors using `teams.RedactURL(target)` instead of raw `target`.
  3. In `internal/cli/queue.go` (`runQueueFile`), log `teams.RedactArgs(lineArgs)`
     instead of raw `strings.Join(lineArgs, " ")` so batch queue runs never emit
     userinfo, `?p=` passcodes, or meeting query/fragment strings into
     `slog.Info`.
* **Files (3 core files + tests):** `internal/teams/url.go`,
  `internal/teams/url_test.go`, `internal/browser/runner.go`,
  `internal/cli/queue.go` (and `internal/cli/queue_test.go`).
* **Verify:** Unit tests in `internal/teams/url_test.go` and
  `internal/cli/queue_test.go` asserting that:
  * `?p=secret-passcode`, `code=oauth`, and `#access_token=xyz` are never
    present in `RedactURL`, `RedactArgs`, `ValidateMeetingURL` errors,
    `ChromeRunner.Join` navigation errors, or queue logs.
  * `RedactURL` is explicitly tested with embedded userinfo credentials on both
    a valid URL (`https://user:pass@teams.microsoft.com/meet/123?p=tok#frag`)
    and a malformed URL
    (`https://user:pass@teams.microsoft.com:badport/meet/123?p=tok`), asserting
    that neither `user`, `pass`, `tok`, nor `frag` ever appears in the output.
* **Depends on:** U1 (`013.001-T`), since both touch `internal/teams/url.go` and
  `internal/teams/url_test.go`.

### U3 (`013.003-T`) — Make Chromium `--no-sandbox` opt-in in `browser.StealthFlags`

* **Domain:** code (Go). **Posture:** test-first. **Size:** `XS` | **Complexity:** `low`.
* **What:**
  1. Add `DisableSandbox bool` to `browser.Options` in
     `internal/browser/allocator.go`.
  2. Add a helper `shouldDisableSandbox(opts Options) bool` that returns `true`
     only when `opts.DisableSandbox` is `true` OR when the environment variable
     `TEAMS_DISABLE_SANDBOX` is `"1"` or `"true"` (case-insensitive).
  3. Update `StealthFlags(opts Options)` so `no-sandbox: true` and
     `disable-setuid-sandbox: true` are included ONLY when
     `shouldDisableSandbox(opts)` is `true`. By default (`Options{}` with
     `TEAMS_DISABLE_SANDBOX` unset), the Chromium sandbox remains enabled.
* **Files (2 files):** `internal/browser/allocator.go`,
  `internal/browser/allocator_test.go`.
* **Verify:** Update `TestStealthFlagsPresent` in
  `internal/browser/allocator_test.go` to verify default `StealthFlags` omits
  `no-sandbox` and `disable-setuid-sandbox`, and add
  `TestStealthFlagsOptInDisableSandbox` verifying both `Options{DisableSandbox: true}`
  and `TEAMS_DISABLE_SANDBOX=1` include those two flags.
* **Depends on:** none.

### U4 (`013.004-T`) — Complete `--browser-path` and `BrowserPath` (`ExecPath`) CLI/config wiring across all subcommands

* **Domain:** code (Go). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Update `newAuthCommand` (`internal/cli/auth.go`) and `newScheduleCommand`
     (`internal/cli/schedule.go`) to accept `--browser-path` and `--profile-dir`
     flags and apply them to `cfg.BrowserPath` and `cfg.ProfileDir` before
     invoking `d.newRunner(cfg)` or `scheduleJob`.
  2. Update `TestSaveLoadRoundTrip` in `internal/config/config_test.go` to
     include `BrowserPath` in the round-trip assertion, and add a test in
     `internal/cli/root_test.go` (`TestBrowserPathWiredAcrossAllCommands`)
     verifying that both `config.yaml`'s `browser_path` and the `--browser-path`
     CLI flag propagate into `cfg.BrowserPath` and `browser.Options.ExecPath`
     across `root`, `join`, `captions`, `auth`, and `schedule`.
* **Files (4 files):** `internal/cli/auth.go`, `internal/cli/schedule.go`,
  `internal/cli/root_test.go`, `internal/config/config_test.go`.
* **Verify:** `go test -race ./internal/config/... ./internal/cli/...` passes.
* **Depends on:** none.

### U5 (`013.005-T`) — Verify `.gitignore` local-env coverage and add pure-Go repository secret-hygiene guardrail test

* **Domain:** config / test guardrail. **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Add `*.env` to the local environment files block in `.gitignore` (alongside
     `.env`, `.env.*`, and `!.env.example`).
  2. Add `internal/config/gitignore_test.go` containing two deterministic,
     pure-Go guardrail tests:
     * `TestGitignoreCoversLocalEnvAndSecretPatterns`: reads the repository root
       `.gitignore` and verifies required ignore entries (`.env`, `.env.*`,
       `*.env`, `!.env.example`, `.mcp.json`, `bin/`, `logs/`, `transcripts/`).
     * `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`: walks repository
       files (or `git ls-files` when available, falling back to a workspace walk
       excluding `.git/`, `bin/`, `logs/`, `transcripts/`, `.copilot/`,
       `.engram/`, `.graphtor/`) and verifies that no tracked `.env` file (other
       than `.env.example`) exists and no tracked file contains a non-synthetic
       live Teams `/meet/<id>?p=<token>` passcode URL or raw `eyJ...` JWT bound
       to `SKYPE-TOKEN`, without embedding any real secret in the test source.
* **Files (2 files):** `.gitignore`, `internal/config/gitignore_test.go`.
* **Verify:** `go test -race ./internal/config/...` passes.
* **Depends on:** U4 (`013.004-T`) to serialize test additions in
  `internal/config/`.

## Dependency Graph

```text
U1 (013.001-T) ──▶ U2 (013.002-T)
U3 (013.003-T) (independent)
U4 (013.004-T) ──▶ U5 (013.005-T)
```

* `013.002-T` depends on `013.001-T` (shared `internal/teams/url.go` and
  `internal/teams/url_test.go` surface).
* `013.003-T` (`internal/browser/allocator.go`) is independent.
* `013.005-T` depends on `013.004-T` (both touch `internal/config/*_test.go`).
* Shipment `013-S` depends on shipped predecessor `012-S` (`013-S blocks 012-S`
  in backlogit's dependency convention where `backlogit dep add 013-S 012-S --type blocks`
  records `012-S` as the upstream dependency of `013-S`).

## Decisions and Rationale

* **Allow loopback over HTTP/HTTPS alongside trusted HTTPS Microsoft domains**:
  `TestCaptionsCommandEndToEndWritesMarkdownFile` and
  `TestCompiledBinaryCapturesRecapToExplicitOutput` start a local
  `httptest.NewServer` (`http://127.0.0.1:<port>`). Allowing loopback hosts
  (`127.0.0.1`, `localhost`, `::1`) over `http`/`https` while requiring `https`
  and an explicit Microsoft/Teams/SharePoint/Office domain suffix for all
  non-loopback hosts closes the arbitrary remote host SSRF/phishing vector
  without breaking local test fixtures.
* **Opt-in sandbox disable in `StealthFlags`**: `NewLiveMeeting` already runs
  with the Chromium sandbox enabled; changing `StealthFlags` to omit
  `no-sandbox` and `disable-setuid-sandbox` unless `Options.DisableSandbox` or
  `TEAMS_DISABLE_SANDBOX=1` is set unifies the default security posture across
  `ChromeRunner` (`auth`, `Runner.Join`) and `AllocatorOptions`.
* **Pure-Go secret-hygiene guardrail test**: Implementing the guardrail as a
  standard `go test` in `internal/config/gitignore_test.go` ensures it runs
  automatically in every local and CI `go test -race ./...` execution without
  requiring external third-party CI actions or unpinned containers.

## Risks and Caveats

* **Existing unit tests using `http://example.com/x`**: `TestValidateMeetingURL`
  in `internal/teams/url_test.go` and `TestScheduleCommandInvalidCron` in
  `internal/cli/root_test.go` (which only validates cron parsing before any job
  runs) were inspected. `TestValidateMeetingURL` is updated in U1 to assert that
  `http://example.com/x` is now rejected while `http://127.0.0.1:8080/meet/x`
  and `https://teams.microsoft.com/l/meetup-join/x` succeed.
* **Existing unit test `TestStealthFlagsPresent`**: Updated in U3 so the default
  case asserts `no-sandbox` and `disable-setuid-sandbox` are absent and a
  dedicated opt-in test asserts their presence when `DisableSandbox: true` or
  `TEAMS_DISABLE_SANDBOX=1`.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only; explicit error wrapping; zero `golangci-lint` and `go vet` findings |
| II: Test first | Each task (U1–U5) adds or updates colocated `_test.go` unit tests observed red before production edits and green after |
| III: Secrets & credentials | Redacts URLs in error/log paths, enforces `.gitignore` `.env` coverage, and adds a pure-Go secret-hygiene test using synthetic fixtures only |
| IV: Containment | All writes confined to workspace root; loopback-only HTTP exception strictly bounded to `127.0.0.1`, `localhost`, and `::1` |
| V: Observability | Queue `slog.Info` logs redacted arguments via `teams.RedactArgs` |
| VI: Single responsibility | Five atomic, width-isolated tasks with no new external dependencies |
| VII, VIII: Strict safety | Risky security/trust-boundary actions classified in the `ProposedAction` table below |
| IX, X: Persistence & context | Deliberation `002-DL`, plan, backlog items `013-F`/`013.001-T`..`013.005-T`, and shipment `013-S` synchronized |
| XI: Merge history | Dedicated feature branch PR targeting `main` during Ship execution |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: `teams.ValidateMeetingURL`
  tightens accepted URL hosts/schemes, `teams.RedactURL` is added,
  `browser.Options.DisableSandbox` is added, and `auth`/`schedule` gain
  `--browser-path` and `--profile-dir` CLI flags.
* Security, auth, permission, or compliance-sensitive behavior — **present**:
  governs the browser navigation trust boundary, Chromium process sandboxing,
  passcode/token redaction in logs/errors, and repository secret hygiene.
* Migration, backfill, destructive data/config action, or irreversible step —
  **absent**: additive/tightening changes with clean revert path.
* External integration, operator checkpoint, or external dependency — **absent**:
  all five units are deterministic and verified by unit and local loopback tests.
* High runtime, rollout, or rollback risk — **absent**: low blast radius, pure
  Go + `.gitignore` changes.

Requires plan hardening: yes

## Plan Hardening

Hardening is required because `013-F` modifies the browser navigation trust
boundary (`ValidateMeetingURL`), Chromium sandbox launch flags (`StealthFlags`),
and URL token redaction across `internal/teams`, `internal/browser`, and
`internal/cli`.

* **Instructions and learnings consulted:** `.github/instructions/go.instructions.md`,
  `.github/instructions/strict-safety.instructions.md`,
  `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`,
  `docs/compound/workflow-issues/amend-breaks-shipment-gate.md`, and
  `docs/closure/2026-11-10-012-S-review-and-closure.md`.

### Protected Invariants

1. **Loopback test-fixture compatibility:** `127.0.0.1`, `localhost`, and `::1`
   must remain valid in `ValidateMeetingURL` over `http://` and `https://` so
   `TestCaptionsCommandEndToEndWritesMarkdownFile` and
   `TestCompiledBinaryCapturesRecapToExplicitOutput` continue to run offline.
2. **Trusted remote HTTPS enforcement:** Non-loopback URLs must use `https://`
   and match or be a dot-prefixed subdomain of the trusted Microsoft Teams /
   SharePoint / Microsoft 365 host allowlist.
3. **Zero secret leakage in errors, logs, or tests:** Navigation errors and
   queue logs must never include userinfo credentials (`user:pass@`), raw query
   strings (`?p=`, `code=`), or URL fragments (`#access_token=`), including on
   malformed URLs where `url.Parse` fails (returning `"[invalid-url]"` whenever
   a clean `scheme://host/path` free of `@`, `?`, `#`, and control characters
   cannot be extracted), and guardrail tests must use synthetic tokens only.
4. **Sandbox enabled by default:** Neither `NewLiveMeeting` nor default
   `StealthFlags(Options{})` may pass `--no-sandbox` or
   `--disable-setuid-sandbox` unless explicitly opted in.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Restrict `teams.ValidateMeetingURL` to trusted HTTPS Microsoft/Teams/SharePoint/Office hosts plus HTTP/HTTPS loopback (`127.0.0.1`, `localhost`, `::1`) | high | Pure-Go host/scheme check in `internal/teams/url.go`; verified against unit, e2e, and compiled-binary loopback tests | Rejects arbitrary untrusted remote hosts and plain remote HTTP while preserving all Teams/SharePoint URLs and local loopback test fixtures |
| Redact query strings and fragments via `teams.RedactURL` / `teams.RedactArgs` in `runner.go` and `queue.go` | moderate | Preserves `MutateJoinURL` output handed to `chromedp.Navigate`; redacts only error strings and log attributes | Passcodes (`?p=`) and OAuth parameters never appear in returned navigation errors or queue logs |
| Change `browser.StealthFlags` default to omit `--no-sandbox` and `--disable-setuid-sandbox` unless `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX=1` is set | high | Aligns `StealthFlags` with `NewLiveMeeting`; container/CI environments needing `--no-sandbox` can set `TEAMS_DISABLE_SANDBOX=1` | Production Chromium instances run with OS sandboxing enabled by default |
| Wire `--browser-path` and `--profile-dir` onto `auth` and `schedule` subcommands | low | Additive Cobra flags matching `root`, `join`, and `captions` | All five CLI entry points honor `--browser-path` and `browser_path` |
| Add `*.env` to `.gitignore` and add pure-Go secret-hygiene guardrail tests in `internal/config/gitignore_test.go` | low | Read-only workspace inspection in tests; uses only synthetic patterns | Prevents accidental tracking of `.env` files or live meeting passcode URLs |

## Runtime Verification and Closure

* **Verification per unit:**
  * U1 (`013.001-T`): `go test -race ./internal/teams/... ./internal/cli/...`
  * U2 (`013.002-T`): `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...`
  * U3 (`013.003-T`): `go test -race ./internal/browser/...`
  * U4 (`013.004-T`): `go test -race ./internal/config/... ./internal/cli/...`
  * U5 (`013.005-T`): `go test -race ./internal/config/...`
* **Full shipment gates:** `gofmt -l cmd internal`, `go vet ./...`,
  `go vet -tags=integration ./...`, CI-pinned `golangci-lint run` (v1.64.8 with
  `GOTOOLCHAIN=go1.26.0`), `go test -race ./...`, `go build ./...`, and
  compiled-binary acceptance (`TestCompiledBinaryCapturesRecapToExplicitOutput`).
* **Rollback procedure:** Revert the `013-S` merge commit on `main` and rebuild
  `teams-cli.exe`; all changes are self-contained with no profile or config
  schema migration required.

## Plan Review

dispatch_mode: single-agent-declared-degradation
decision: PASS

* **Capability declaration (P-012):** `TOOL_DEGRADED: reviewer-subagent-dispatch — declared fallback: single-agent persona pass`
  (plan artifact fits a single direct read; inline rubric pass executed across
  all six selected personas).
* **Constitution Reviewer (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Principles I–XI mapped and satisfied; test-first posture enforced on U1–U5;
  no secrets or external paths introduced.
* **Go Reviewer (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`). Idiomatic
  `net/url` host parsing via `u.Hostname()`, explicit error wrapping, no new
  dependencies, and clean test-table patterns.
* **Scope Boundary Auditor (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Every unit maps 1-to-1 to the five selected stash entries (`A99EEABD`,
  `EEA7F685`, `D2023898`, `87CDB97A`, `3084A83E`), stays within `<= 4` files and
  `< 2h` effort, and respects width isolation.
* **Learnings Researcher (always-on):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Incorporates `golangci-lint-go-version-mismatch.md` and
  `amend-breaks-shipment-gate.md` guidance.
* **Architecture Strategist (cross-model rubric):** **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`).
  Placing `ValidateMeetingURL` and `RedactURL` in `internal/teams/url.go` keeps
  URL validation and sanitization in a single foundational package imported by
  `internal/browser` and `internal/cli` without circular dependencies.
* **Security Lens Reviewer (triggered by trust boundary, sandbox posture, and secret hygiene):**
  **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`). Host allowlist uses dot-boundary suffix
  matching (`host == d || strings.HasSuffix(host, "."+d)`) so lookalike domains
  (`teams.microsoft.com.evil.example`) are rejected; plain `http://` is
  restricted strictly to loopback (`127.0.0.1`, `localhost`, `::1`); `RedactURL`
  strips userinfo, query strings, and fragments; `StealthFlags` enables the
  Chromium sandbox by default.

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
`013-F` and shipment `013-S`.
