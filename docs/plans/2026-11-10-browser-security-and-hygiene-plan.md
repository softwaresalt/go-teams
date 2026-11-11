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
   and `ChromeRunner.Join` (`internal/browser/runner.go:45-61`) passes its
   `meetingURL` argument directly to `r.run` / `chromedp.Navigate` without
   calling `teams.ValidateMeetingURL` before browser allocation. This leaves
   initial-URL ingress unconstrained instead of enforcing trusted Microsoft
   Teams / SharePoint / Microsoft 365 `https://` hosts on initial navigation
   (and validating `meetingURL` at the `ChromeRunner.Join` entry point before
   allocating a browser) while preserving `http://` and `https://` loopback
   (`127.0.0.1`, `localhost`, `::1`) for local `httptest.Server` e2e and binary
   acceptance fixtures (post-navigation HTTP/JS redirects inside Chromium are
   documented as residual risk).
2. `ChromeRunner.run` (`internal/browser/runner.go`), `chromeMeeting.Join`
   (`internal/browser/meeting_chrome.go`), and `runQueueFile` / `splitQueueArgs`
   (`internal/cli/queue.go`) can surface raw URLs containing `?p=<passcode>`,
   `code=`, or `#` fragments in error messages (including `splitQueueArgs`
   unterminated-quote errors) or structured logs.
3. `StealthFlags(opts Options)` (`internal/browser/allocator.go`)
   unconditionally sets `no-sandbox: true` and `disable-setuid-sandbox: true`,
   and omitting `no-sandbox` in `StealthFlags` or `cleanHeadlessFlags`
   (`internal/browser/meeting.go`) still allows `chromedp` v0.15.1's
   `DefaultExecAllocatorOptions` (`chromedp.NoSandbox`) to add `--no-sandbox`
   automatically when running as root. Both flag builders must explicitly emit
   `Flag{Name: "no-sandbox", Value: false}` and
   `Flag{Name: "disable-setuid-sandbox", Value: false}` unless sandbox disable
   is explicitly opted in (`Options.DisableSandbox` or
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
   scanning git-tracked files (strictly from the authoritative Git index, with
   an always-run CI check in `014.002-T` covering docs/backlog-only PRs) for
   accidental live Teams passcode or token strings.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Initial-URL ingress allowlist for trusted Microsoft/Teams/SharePoint/Office HTTPS hosts + loopback HTTP/HTTPS fixture support in `teams.ValidateMeetingURL` AND pre-allocation entry-point validation in `ChromeRunner.Join` (post-navigation redirects documented as residual risk) | `A99EEABD` | U1 (`013.001-T`) | `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...` (`TestValidateMeetingURL`, `TestMutateJoinURLErrors`, `TestChromeRunnerJoinRejectsDisallowedURLBeforeBrowserAllocation`) |
| R2: Redact userinfo, query strings (`?p=`, `code=`), and fragments from navigation errors, `splitQueueArgs` parse errors, and queue logs | `EEA7F685` | U2 (`013.002-T`) | `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...` (`TestRedactURL`, `TestRunnerJoinErrorRedactsURL`, `TestQueueLogsAndParseErrorsRedactURL`) |
| R3: Explicitly set `no-sandbox: false` and `disable-setuid-sandbox: false` when opt-in is off in both `StealthFlags` (`AllocatorOptions`) and `cleanHeadlessFlags` (`cleanHeadlessAllocatorOptions`); opt-in via `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX=1` | `D2023898` | U3 (`013.003-T`) | `go test -race ./internal/browser/...` (`TestStealthFlagsDefaultSandboxEnabled`, `TestStealthFlagsOptInDisableSandbox`, `TestEffectiveAllocatorLaunchArgsSandboxOptInAndRootOverride`) |
| R4: Complete `--browser-path` / `browser_path` (`ExecPath`) CLI and config wiring across `auth`, `schedule`, `root`, `join`, `captions` | `87CDB97A` | U4 (`013.004-T`) | `go test -race ./internal/config/... ./internal/cli/...` (`TestSaveLoadRoundTrip`, `TestBrowserPathWiredAcrossAllCommands`) |
| R5: `.gitignore` `*.env` coverage + authoritative Git-index secret-hygiene guardrail test in `internal/config/gitignore_test.go` detecting actual credential values bound to `SKYPE-TOKEN` (ignoring bare storage-key references) with negative/positive scanner test cases (paired with always-run CI secret check in `014.002-T` for docs-only PRs) | `3084A83E` | U5 (`013.005-T`) + `014.002-T` | `go test -race ./internal/config/...` (`TestGitignoreCoversLocalEnvAndSecretPatterns`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`) + CI `changes` secret-hygiene step |

## Implementation Units

### U1 (`013.001-T`) — Enforce initial-URL ingress allowlist in `ValidateMeetingURL` and `ChromeRunner.Join` before browser allocation

* **Domain:** code (Go). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:** Update `ValidateMeetingURL` in `internal/teams/url.go` and
  `ChromeRunner.Join` in `internal/browser/runner.go` to enforce initial-URL
  ingress validation before browser allocation and navigation:
  1. Loopback hosts (`localhost`, `127.0.0.1`, `::1`, retrieved via
     `u.Hostname()`) are permitted over both `http` and `https` (preserving
     `httptest.NewServer` fixtures in `internal/cli/e2e_test.go` and
     `internal/cli/binary_automation_test.go`).
  2. Non-loopback initial URLs MUST use `https` (rejecting plain `http://` on
     remote hosts) and their lowercase `u.Hostname()` MUST match or be a
     dot-prefixed subdomain of an allowed Microsoft 365 / Teams / SharePoint
     host suffix: `teams.microsoft.com`, `teams.live.com`,
     `teams.cloud.microsoft`, `sharepoint.com`, `microsoft.com`, `office.com`,
     `sfx.ms`, `aka.ms`.
  3. Any other remote initial host (such as `http://example.com/x`,
     `https://example.com/x`, or `https://evil-teams.microsoft.com.attacker.example/x`)
     is rejected with an actionable error (that does not echo raw query/userinfo
     secrets) before navigation begins.
  4. **Validate at `ChromeRunner.Join` before browser allocation:** Update
     `ChromeRunner.Join(ctx context.Context, meetingURL string) error` in
     `internal/browser/runner.go` to call `teams.ValidateMeetingURL(meetingURL)`
     BEFORE calling `r.run(ctx, opts, meetingURL)` (before
     `chromedp.NewExecAllocator` / `chromedp.NewContext`), returning a wrapped
     validation error immediately on a disallowed URL without launching or
     allocating a browser. (This closes the direct `ChromeRunner.Join` entry-point
     bypass in `internal/browser/runner.go:45-61`, which is separate from the
     documented post-navigation redirect residual risk.)
  5. **Scope & residual risk:** `ValidateMeetingURL` and `ChromeRunner.Join`
     govern **initial-URL ingress validation** prior to browser allocation and
     `chromedp.Navigate`. Post-navigation HTTP 3xx or client-side JavaScript
     redirects inside Chromium (for example, an allowed loopback endpoint or
     shortener redirecting to an untrusted remote host) are explicitly recorded
     as a documented residual risk: enforcing in-browser CDP request
     interception across Microsoft 365 federated OAuth / ADFS / Okta / Duo IdP
     sign-in redirects would break enterprise tenant authentication flows and is
     outside the `ValidateMeetingURL` contract.
* **Files (2 production files + 2 colocated unit test files):**
  `internal/teams/url.go`, `internal/teams/url_test.go`,
  `internal/browser/runner.go`, `internal/browser/runner_test.go`.
* **Verify:**
  * Update `TestValidateMeetingURL` in `internal/teams/url_test.go` to cover
    trusted HTTPS hosts (`teams.microsoft.com`, `teams.cloud.microsoft`,
    `teams.live.com`, `contoso.sharepoint.com`), loopback HTTP/HTTPS
    (`http://127.0.0.1:54321/meet/x`, `http://localhost:8080/l/meetingrecap`,
    `http://[::1]:9000/meet/x`), and rejected cases
    (`http://teams.microsoft.com/x`, `https://example.com/x`,
    `https://teams.microsoft.com.evil.example/x`).
  * Add a direct runner regression test in `internal/browser/runner_test.go`
    (`TestChromeRunnerJoinRejectsDisallowedURLBeforeBrowserAllocation`) asserting
    that `NewChromeRunner(Options{ExecPath: "/nonexistent/chrome-binary"}).Join(ctx, "https://evil.example.com/meet/123?p=secret")`
    returns a validation error immediately before browser allocation/launch and
    does not leak `secret` in the error string.
  * Run `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...`.
* **Depends on:** none.

### U2 (`013.002-T`) — Add `teams.RedactURL` and scrub userinfo, query, and fragment tokens from navigation errors, `splitQueueArgs` parse errors, and CLI queue logs

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
  2. In `internal/browser/runner.go` (preserving the `teams.ValidateMeetingURL(meetingURL)`
     pre-allocation check in `ChromeRunner.Join` from U1 / `013.001-T`), wrap
     `chromedp.Navigate` errors in `ChromeRunner.run` using
     `teams.RedactURL(target)` instead of raw `target`.
  3. In `internal/cli/queue.go`:
     * In `splitQueueArgs(line string)`: remove the raw `line` from the
       unterminated-quote parse error (`return nil, fmt.Errorf("unterminated quote in queue line")`
       instead of `fmt.Errorf("unterminated quote in %q", line)`) so an
       unmatched quote on a queue line containing a passcode-bearing URL never
       exposes the raw URL or `?p=` passcode when `runQueueFile` propagates the
       error before argument tokenization/logging.
     * In `runQueueFile`: log `teams.RedactArgs(lineArgs)` instead of raw
       `strings.Join(lineArgs, " ")` so batch queue runs never emit userinfo,
       `?p=` passcodes, or meeting query/fragment strings into `slog.Info`.
* **Files (3 core files + tests):** `internal/teams/url.go`,
  `internal/teams/url_test.go`, `internal/browser/runner.go`,
  `internal/browser/runner_test.go`, `internal/cli/queue.go`,
  `internal/cli/queue_test.go`.
* **Verify:** Unit tests in `internal/teams/url_test.go`,
  `internal/browser/runner_test.go`, and `internal/cli/queue_test.go` asserting
  that:
  * `?p=secret-passcode`, `code=oauth`, and `#access_token=xyz` are never
    present in `RedactURL`, `RedactArgs`, `ValidateMeetingURL` errors,
    `ChromeRunner.Join` pre-allocation validation and navigation errors, or
    queue logs.
  * `RedactURL` is explicitly tested with embedded userinfo credentials on both
    a valid URL (`https://user:pass@teams.microsoft.com/meet/123?p=tok#frag`)
    and a malformed URL
    (`https://user:pass@teams.microsoft.com:badport/meet/123?p=tok`), asserting
    that neither `user`, `pass`, `tok`, nor `frag` ever appears in the output.
  * An unmatched-quote regression test in `internal/cli/queue_test.go` passes a
    queue line with an unmatched quote and a `?p=secret-passcode` URL (e.g.
    `join --url "https://teams.microsoft.com/meet/123?p=secret-passcode`) to
    both `splitQueueArgs` and `runQueueFile`, asserting that the returned error
    contains neither `secret-passcode` nor the raw URL.
* **Depends on:** U1 (`013.001-T`), since both touch `internal/teams/url.go`,
  `internal/teams/url_test.go`, `internal/browser/runner.go`, and
  `internal/browser/runner_test.go`.

### U3 (`013.003-T`) — Explicitly set `no-sandbox: false` when opt-in is off in `AllocatorOptions` and `cleanHeadlessAllocatorOptions`

* **Domain:** code (Go). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Add `DisableSandbox bool` to `browser.Options` in
     `internal/browser/allocator.go`.
  2. Add a helper `shouldDisableSandbox(opts Options) bool` that returns `true`
     only when `opts.DisableSandbox` is `true` OR when the environment variable
     `TEAMS_DISABLE_SANDBOX` is `"1"` or `"true"` (case-insensitive).
  3. Update both `StealthFlags(opts Options)` (`internal/browser/allocator.go`,
     used by `AllocatorOptions`) and `cleanHeadlessFlags(opts Options)`
     (`internal/browser/meeting.go`, used by `cleanHeadlessAllocatorOptions`)
     so that:
     * When `shouldDisableSandbox(opts)` is `false` (default), both explicitly
       emit `Flag{Name: "no-sandbox", Value: false}` and
       `Flag{Name: "disable-setuid-sandbox", Value: false}` — which translate
       to `chromedp.Flag("no-sandbox", false)` and
       `chromedp.Flag("disable-setuid-sandbox", false)`, explicitly overriding
       `chromedp.NoSandbox` in `chromedp.DefaultExecAllocatorOptions` even when
       `chromedp` v0.15.1 runs as root (`os.Geteuid() == 0`).
     * When `shouldDisableSandbox(opts)` is `true`, both emit
       `Flag{Name: "no-sandbox", Value: true}` and
       `Flag{Name: "disable-setuid-sandbox", Value: true}`.
* **Files (3 files in `internal/browser`):** `internal/browser/allocator.go`,
  `internal/browser/meeting.go`, `internal/browser/allocator_test.go`.
* **Verify:** Update `internal/browser/allocator_test.go` to:
  * Verify `StealthFlags` and `cleanHeadlessFlags` emit `no-sandbox: false` and
    `disable-setuid-sandbox: false` by default and `Value: true` when
    `Options{DisableSandbox: true}` or `TEAMS_DISABLE_SANDBOX=1` is set.
  * Verify effective launch arguments for both `AllocatorOptions(opts)` and
    `cleanHeadlessAllocatorOptions(opts)` — including simulated root launches
    where `chromedp.NoSandbox` / `chromedp.Flag("no-sandbox", true)` is preset
    before applying the allocator options — confirming `--no-sandbox` and
    `--disable-setuid-sandbox` are absent from the effective Chromium command
    line when opt-in is off and present when opt-in is on.
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

### U5 (`013.005-T`) — Verify `.gitignore` local-env coverage and add authoritative Git-index repository secret-hygiene guardrail test

* **Domain:** config / test guardrail. **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. Add `*.env` to the local environment files block in `.gitignore` (alongside
     `.env`, `.env.*`, and `!.env.example`).
  2. Add `internal/config/gitignore_test.go` containing two deterministic,
     pure-Go guardrail tests:
     * `TestGitignoreCoversLocalEnvAndSecretPatterns`: reads the repository root
       `.gitignore` and verifies required ignore entries (`.env`, `.env.*`,
       `*.env`, `!.env.example`, `.mcp.json`, `bin/`, `logs/`, `transcripts/`).
     * `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`: enumerates tracked
       repository files **strictly from the authoritative Git index** — first
       attempting `git ls-files -z` (searching `PATH` and standard Windows Git
       install locations), and if `git` is unavailable, parsing the repository's
       `.git/index` binary file (`DIRC` v2/v3/v4 entry paths) directly in pure
       Go. If neither `git ls-files -z` nor `.git/index` is readable, the test
       MUST fail closed with `t.Fatalf` (prerequisite failure — never `t.Skip`
       and **never fall back to a filesystem/workspace walk**, which would scan
       legitimate ignored local files such as `.env` or `local.env` and report
       false-positive tracked leaks). Verifies that no tracked `.env` file
       (other than `.env.example`) exists in the Git index and no tracked file
       contains:
       - a non-synthetic live Teams `/meet/<id>?p=<token>` passcode URL
         (`[?&]p=[A-Za-z0-9_-]{8,}` on a Teams meeting URL outside allowlisted
         synthetic documentation/test placeholders),
       - an **actual credential value associated with `SKYPE-TOKEN`** (matching
         `SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}` — explicitly NOT matching
         bare `"SKYPE-TOKEN"` storage-key identifier references in
         `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, or
         markdown documentation), or
       - a raw `Bearer\s+eyJ[A-Za-z0-9_-]{10,}` JWT credential value,
       without embedding any real secret in the test source.
     * `internal/config/gitignore_test.go` MUST also include unit tests for the
       scanner helper covering:
       - a **negative test case** confirming that marker-only source and
         documentation strings (such as
         `bytes.Contains(data, []byte("SKYPE-TOKEN"))`, storage-key references
         in `recap.go` / `meeting_chrome.go`, or prose mentions of
         `SKYPE-TOKEN`) pass the scanner without false positives, and
       - a **positive test case** confirming that a synthetic credential
         assignment (e.g., constructed at runtime in `gitignore_test.go` by
         joining `"SKYPE-TOKEN="` and
         `"\"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.syntheticPayload\""`) and a
         live-shaped passcode URL (e.g., constructed at runtime for
         `https://teams.microsoft.com/meet/123456789` + `?p=AbCdEfGh12345678`)
         are detected and rejected by the scanner helper.
  3. **CI enforcement on docs/backlog-only PRs (cross-reference `014.002-T`):**
     Because `014.002-T` (`014-S`) owns `.github/workflows/ci.yml` to preserve
     width isolation, `014.002-T` adds an unconditional tracked-file
     secret-hygiene check step in the always-run `changes` job of
     `.github/workflows/ci.yml` (independent of `needs.changes.outputs.code == 'true'`)
     that scans `git ls-files -z` on every PR and push — including
     docs/backlog-only PRs where the `test` job is skipped — using the same
     credential-value rule (`SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}`,
     `Bearer\s+eyJ[A-Za-z0-9_-]{10,}`, and non-synthetic live Teams `?p=`
     passcode URLs, while allowing bare `SKYPE-TOKEN` storage-key references in
     source and documentation).
* **Files (2 files):** `.gitignore`, `internal/config/gitignore_test.go`.
* **Verify:** `go test -race ./internal/config/...` passes (including negative
  marker-only and positive synthetic-credential scanner unit cases, `.git/index`
  parser coverage, and prerequisite-failure path).
* **Depends on:** U4 (`013.004-T`) to serialize test additions in
  `internal/config/`.

## Dependency Graph

```text
U1 (013.001-T) ──▶ U2 (013.002-T)
U3 (013.003-T) (independent)
U4 (013.004-T) ──▶ U5 (013.005-T)
```

* `013.002-T` depends on `013.001-T` (shared `internal/teams/url.go`,
  `internal/teams/url_test.go`, `internal/browser/runner.go`, and
  `internal/browser/runner_test.go` surface).
* `013.003-T` (`internal/browser/allocator.go`) is independent.
* `013.005-T` depends on `013.004-T` (both touch `internal/config/*_test.go`).
* Shipment `013-S` depends on shipped predecessor `012-S` (`013-S blocks 012-S`
  in backlogit's dependency convention where `backlogit dep add 013-S 012-S --type blocks`
  records `012-S` as the upstream dependency of `013-S`).

## Decisions and Rationale

* **Initial-URL ingress allowlist in `ValidateMeetingURL` and `ChromeRunner.Join` before browser allocation (with post-navigation redirects documented as residual risk)**:
  `TestCaptionsCommandEndToEndWritesMarkdownFile` and
  `TestCompiledBinaryCapturesRecapToExplicitOutput` start a local
  `httptest.NewServer` (`http://127.0.0.1:<port>`). Allowing loopback hosts
  (`127.0.0.1`, `localhost`, `::1`) over `http`/`https` while requiring `https`
  and an explicit Microsoft/Teams/SharePoint/Office domain suffix for all
  non-loopback initial URLs — and enforcing `teams.ValidateMeetingURL(meetingURL)`
  in `ChromeRunner.Join` before `r.run` allocates a browser — enforces
  initial-URL ingress validation across both `chromeMeeting.Join` and
  `ChromeRunner.Join` without breaking local test fixtures. Post-navigation
  HTTP/JS redirects inside Chromium are recorded as a documented residual risk
  because blocking third-party redirect hops in CDP would break enterprise
  federated IdP (ADFS, Okta, Duo, Ping) authentication flows.
* **Explicit `no-sandbox: false` when opt-in is off in both `StealthFlags` and `cleanHeadlessFlags`**:
  In `chromedp` v0.15.1, `DefaultExecAllocatorOptions` includes
  `chromedp.NoSandbox`, which automatically adds `--no-sandbox` when the
  process runs as root (`os.Geteuid() == 0`) unless explicitly overridden by a
  subsequent `chromedp.Flag("no-sandbox", false)`. Emitting
  `Flag{Name: "no-sandbox", Value: false}` and
  `Flag{Name: "disable-setuid-sandbox", Value: false}` in both
  `StealthFlags(opts)` (`AllocatorOptions`) and `cleanHeadlessFlags(opts)`
  (`cleanHeadlessAllocatorOptions` in `internal/browser/meeting.go`) whenever
  `!shouldDisableSandbox(opts)` guarantees that `--no-sandbox` is never active
  unless explicitly opted in via `Options.DisableSandbox` or
  `TEAMS_DISABLE_SANDBOX=1`.
* **Authoritative Git-index secret-hygiene guardrail test + credential-value matching (`SKYPE-TOKEN`) + always-run CI check (`014.002-T`)**:
  Implementing the guardrail in `internal/config/gitignore_test.go` using the
  authoritative Git index (`git ls-files -z` with a pure-Go `.git/index` binary
  parser fallback and fail-closed `t.Fatalf` if neither is readable — never a
  workspace walk) prevents false positives on legitimate ignored local `.env`
  files. Matching an actual credential value associated with `SKYPE-TOKEN`
  (`SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}` and
  `Bearer\s+eyJ[A-Za-z0-9_-]{10,}`) rather than the bare `"SKYPE-TOKEN"`
  substring prevents false positives on legitimate storage-key references in
  `internal/browser/recap.go`, `internal/browser/meeting_chrome.go`, and
  markdown documentation. Pairing it with an unconditional `git ls-files`
  secret-hygiene step in the always-run `changes` job of
  `.github/workflows/ci.yml` (`014.002-T`) ensures docs/backlog-only PRs are
  also scanned in CI even when the Go `test` job is skipped.

## Risks and Caveats

* **Post-navigation redirects (documented residual risk)**: `ValidateMeetingURL`
  validates the initial target URL before browser allocation / `chromedp.Navigate`
  (in `ChromeRunner.Join` and inside `MutateJoinURL`). If an allowed initial
  endpoint (such as a local loopback server or an `aka.ms` link) issues an HTTP
  3xx or JavaScript redirect to an untrusted remote host after initial
  navigation, Chromium will follow it. Intercepting post-navigation redirects at
  the CDP network layer is intentionally out of scope for `ValidateMeetingURL`
  to avoid breaking federated enterprise SSO/IdP login redirects, and is
  recorded as a residual risk.
* **Existing unit tests using `http://example.com/x`**: `TestValidateMeetingURL`
  in `internal/teams/url_test.go` and `TestScheduleCommandInvalidCron` in
  `internal/cli/root_test.go` (which only validates cron parsing before any job
  runs) were inspected. `TestValidateMeetingURL` is updated in U1 to assert that
  `http://example.com/x` is now rejected while `http://127.0.0.1:8080/meet/x`
  and `https://teams.microsoft.com/l/meetup-join/x` succeed.
* **Existing unit test `TestStealthFlagsPresent`**: Updated in U3 so the default
  case asserts `no-sandbox: false` and `disable-setuid-sandbox: false` are
  present and effective allocator launch args exclude `--no-sandbox` (including
  when `NoSandbox` is preset as in root launches), while a dedicated opt-in
  test asserts `Value: true` and `--no-sandbox` presence when
  `DisableSandbox: true` or `TEAMS_DISABLE_SANDBOX=1`.

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
2. **Trusted remote HTTPS initial-URL ingress enforcement (including `ChromeRunner.Join` pre-allocation validation):**
   Non-loopback initial URLs must use `https://` and match or be a dot-prefixed
   subdomain of the trusted Microsoft Teams / SharePoint / Microsoft 365 host
   allowlist, and `ChromeRunner.Join` in `internal/browser/runner.go` must call
   `teams.ValidateMeetingURL(meetingURL)` before allocating a browser
   (with post-navigation redirects documented as residual risk).
3. **Zero secret leakage in errors, logs, or tests:** Navigation errors,
   `splitQueueArgs` unmatched-quote parse errors, and queue logs must never
   include userinfo credentials (`user:pass@`), raw query strings (`?p=`,
   `code=`), or URL fragments (`#access_token=`), including on malformed URLs
   where `url.Parse` fails (returning `"[invalid-url]"` whenever a clean
   `scheme://host/path` free of `@`, `?`, `#`, and control characters cannot be
   extracted) and on queue lines that fail quote tokenization before argument
   logging, and guardrail tests must use synthetic tokens only.
4. **Sandbox enabled by default (including root launches):** Both
   `StealthFlags(opts)` (`AllocatorOptions`) and `cleanHeadlessFlags(opts)`
   (`cleanHeadlessAllocatorOptions` in `internal/browser/meeting.go`) must
   explicitly emit `no-sandbox: false` and `disable-setuid-sandbox: false` when
   opt-in is off so `chromedp.NoSandbox` in `DefaultExecAllocatorOptions` is
   overridden even when running as root.
5. **Authoritative Git-index secret scanning:** Tracked-file secret scanning in
   `internal/config/gitignore_test.go` must read tracked paths strictly from
   `git ls-files -z` or pure-Go `.git/index` parsing (failing closed with
   `t.Fatalf` if neither is available, and never falling back to a workspace
   directory walk that could inspect ignored `.env` files), while CI's
   always-run `changes` job (`014.002-T`) scans tracked files on every PR
   including docs/backlog-only PRs.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Restrict `teams.ValidateMeetingURL` initial-URL ingress to trusted HTTPS Microsoft/Teams/SharePoint/Office hosts plus HTTP/HTTPS loopback (`127.0.0.1`, `localhost`, `::1`) and enforce it in `ChromeRunner.Join` before browser allocation, recording post-navigation redirects as residual risk | high | Pure-Go initial-URL host/scheme check in `internal/teams/url.go` and pre-allocation guard in `internal/browser/runner.go`; verified against unit (`runner_test.go`, `url_test.go`), e2e, and compiled-binary loopback tests | Rejects arbitrary untrusted remote initial URLs and plain remote HTTP before browser allocation/navigation while preserving Teams/SharePoint URLs and local loopback test fixtures; post-navigation redirects documented as residual risk |
| Redact userinfo, query strings, and fragments via `teams.RedactURL` / `teams.RedactArgs` in `runner.go` and `queue.go`, and remove raw line from `splitQueueArgs` unmatched-quote error | moderate | Preserves `MutateJoinURL` output handed to `chromedp.Navigate`; redacts error strings, `splitQueueArgs` parse errors, and log attributes | Passcodes (`?p=`), userinfo, and OAuth parameters never appear in returned navigation errors, `splitQueueArgs` errors, or queue logs |
| Explicitly emit `no-sandbox: false` and `disable-setuid-sandbox: false` in `StealthFlags` (`AllocatorOptions`) and `cleanHeadlessFlags` (`cleanHeadlessAllocatorOptions`) unless `Options.DisableSandbox` or `TEAMS_DISABLE_SANDBOX=1` is set | high | Overrides `chromedp.NoSandbox` in `DefaultExecAllocatorOptions` even on root launches; container/CI environments needing `--no-sandbox` can set `TEAMS_DISABLE_SANDBOX=1` | Production Chromium instances run with OS sandboxing enabled by default across both `AllocatorOptions` and `cleanHeadlessAllocatorOptions` |
| Wire `--browser-path` and `--profile-dir` onto `auth` and `schedule` subcommands | low | Additive Cobra flags matching `root`, `join`, and `captions` | All five CLI entry points honor `--browser-path` and `browser_path` |
| Add `*.env` to `.gitignore` and add authoritative Git-index secret-hygiene guardrail tests in `internal/config/gitignore_test.go` (paired with always-run CI check in `014.002-T`) | low | Strictly inspects Git index (`git ls-files -z` or `.git/index` binary parser; fails closed with `t.Fatalf`, no workspace walk fallback); uses synthetic patterns only | Prevents accidental tracking of `.env` files or live meeting passcode URLs without false positives on local ignored `.env` files, and scans docs-only PRs in CI via `014.002-T` |

## Runtime Verification and Closure

* **Verification per unit:**
  * U1 (`013.001-T`): `go test -race ./internal/teams/... ./internal/browser/... ./internal/cli/...`
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
  **PASS** (`0 P0, 0 P1, 0 P2, 0 P3`). Initial-URL host allowlist uses
  dot-boundary suffix matching (`host == d || strings.HasSuffix(host, "."+d)`)
  and is enforced in both `ValidateMeetingURL` and `ChromeRunner.Join` before
  browser allocation so lookalike domains (`teams.microsoft.com.evil.example`)
  are rejected on initial navigation while post-navigation HTTP/JS redirects are
  explicitly documented as residual risk; plain `http://` is restricted strictly
  to loopback (`127.0.0.1`, `localhost`, `::1`); `RedactURL` strips userinfo,
  query strings, and fragments and `splitQueueArgs` omits the raw line from
  unmatched-quote errors; `StealthFlags` and `cleanHeadlessFlags` explicitly
  set `no-sandbox: false` and `disable-setuid-sandbox: false` by default
  (overriding `chromedp.NoSandbox` even as root); tracked-file secret scanning
  uses the authoritative Git index (`git ls-files -z` or `.git/index` parser,
  failing closed with `t.Fatalf` and no workspace walk), matches actual
  credential values associated with `SKYPE-TOKEN`
  (`SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}` and
  `Bearer\s+eyJ[A-Za-z0-9_-]{10,}`) rather than bare `"SKYPE-TOKEN"`
  storage-key references, and is paired with an always-run CI check in
  `014.002-T`.

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 P2, 0 P3`). Ready for harvest into
`013-F` and shipment `013-S`.
