---
title: 013-S browser security session memory
date: 2026-11-11
shipment: 013-S
feature: 013-F
branch: feat/013-s-browser-security-secret-hygiene-and-execpath-configuration
status: pr-pending
---

## Completed

* `013.001-T`: URL host allowlist, loopback boundary, userinfo rejection, and
  pre-allocation validation in `ChromeRunner.Join` (commit `7c3dc23`)
* `013.002-T`: `RedactURL`, `RedactArgs`, queue log and error redaction
  (commit `1493941`)
* `013.003-T`: explicit sandbox opt-in and default-off flags (commit `eb102c9`)
* `013.004-T`: `--browser-path` and `--profile-dir` on `auth` and `schedule`
  (commit `812cba8`)
* `013.005-T`: `*.env` ignore rule and Git-index secret-hygiene test
  (see branch history)

## Files modified

`internal/teams/url.go`, `internal/browser/{allocator,meeting,runner}.go`,
`internal/cli/{auth,schedule,queue,root}.go`, matching `*_test.go` files,
`internal/config/gitignore_test.go`, `.gitignore`, `README.md`.

## Decisions and rationale

* Allowlist includes `sfx.ms` and `aka.ms` per the plan in addition to the
  Teams, SharePoint, Microsoft, and Office suffixes.
* `RedactURL` returns `[invalid-url]` instead of a partial string whenever a
  clean `scheme://host/path` cannot be guaranteed.
* The sandbox opt-in is exposed through `Options` and `TEAMS_DISABLE_SANDBOX`
  only, to keep the surface minimal.
* Effective Chromium arguments are tested by intercepting chromedp's command
  with `ModifyCmdFunc`; no browser is launched.
* Local lint uses the CI-pinned v1.64.8 binary in `bin/lint-v1` with
  `GOTOOLCHAIN=go1.26.0` because the PATH binary is v2 with an incompatible
  config schema.
* Moving a task to `done` in backlogit archives it immediately, so manifest
  tasks live in `.backlogit/archive/` before shipment close.

## Failed approaches

* A `RedactURL` test input of `https://user:123?p=tok@host:bad/x` was invalid:
  the URL authority ends at `?`, so the output `https://user:123` is correct.
* `backlogit hooks poll --consumer ship` is not a supported CLI form in
  backlogit 1.12.0 (`unknown flag: --consumer`); hook polling was skipped.

## Open questions and next steps

* Open the feature PR, run Copilot review, merge with a merge commit.
* Post-merge: close `013-S` through the P-015 cascade path, archive `002-DL`,
  finalize `compaction_status`, run `compact-context`, and open the closure PR.
* Tenants on vanity or sovereign hosts may need allowlist additions.
