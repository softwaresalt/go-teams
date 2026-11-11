---
type: memory-compacted
date: 2026-11-10
shipment: 013-S
feature: 013-F
source: docs/archive/memory/2026-11-10/013-S-browser-security-memory.md
---

# Compacted memory - Shipment 013-S (browser security, secret hygiene, ExecPath configuration)

**Shipped**: PR #33, merge `dbf1e2293adb0a3fbc300b9329a44b3a9ccff5b1`. Closed
and archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship`). Source deliberation `002-DL` was archived by the
explicit source-artifact cleanup step.

## Decision and outcome

* Meeting URLs are validated at ingress: HTTPS on Teams, SharePoint, and
  Microsoft 365 host suffixes (plus `sfx.ms` and `aka.ms`), `http` or `https`
  only on loopback, no embedded userinfo. `ChromeRunner.Join` validates before
  any browser is allocated. Redirects inside Chromium stay a documented
  residual risk because intercepting them would break federated sign-in.
* `teams.RedactURL` and `teams.RedactArgs` reduce URLs to `scheme://host/path`
  and return `[invalid-url]` whenever a clean form cannot be guaranteed.
* The Chromium sandbox opt-out is explicit (`Options.DisableSandbox`,
  `TEAMS_DISABLE_SANDBOX`); both chromedp flag emitters write `false` by
  default, overriding chromedp's root-user default.
* `--browser-path` and `--profile-dir` reach `auth` and `schedule` through a
  shared `browserOptions` mapping.
* `*.env` is ignored, and a guardrail test scans files enumerated from the Git
  index and fails closed when neither `git ls-files` nor `.git/index` is usable.

## Key reusable insights

* Redaction of malformed URLs must follow the authority rule (it ends at the
  first `/`, `?`, or `#`): strip query and fragment before separating host from
  path, or a secret containing `/` becomes a logged path.
* Argument redaction must start at the earliest scheme-like token, whether an
  outer `msteams://` URL or a malformed `https:/` form, not at the first
  `http(s)` match.
* A pure-Go Git index parser must reject mandatory extensions (lowercase
  signatures such as the split-index `link`) instead of returning a partial
  path inventory.
* Effective Chromium arguments can be tested without launching a browser by
  intercepting the chromedp command with `ModifyCmdFunc`.
* `backlogit move --status done` archives the task immediately, so manifest
  tasks sit in `.backlogit/archive/` before the shipment closes.

## Review outcome

Local review: `READY`, `P0=0, P1=0`. Copilot shadow review on PR #33 ran two
rounds. Round 1 raised four threads: three fixed (query/fragment split order,
earliest scheme token, flaky command-wiring test) and one deferred under
P-021 C1 to stash entry `FC763C17` (headed direct-exec launcher sandbox
opt-in). Round 2 raised two threads, both fixed (mandatory index extensions,
outer non-HTTP scheme redaction). The `copilot-review` gate returned
`SATISFIED` and CI passed before the merge commit.

## Post-merge closure summary

* Merge confirmed (`gh pr view 33`, `git merge-base --is-ancestor`).
* Cascade close archived `013.001-T` through `013.005-T`, `013-F`, and
  `013-S` with `returned_ids` empty and every task `parent_id` preserved.
  `002-DL` was not reported by the engine and was archived explicitly.
* Shipments `010-S` and `014-S` and their tasks were verified byte-identical.

## Follow-ups carried forward

* `FC763C17`: deliberate whether the direct-exec headed launcher should honor
  the sandbox opt-out.
* Tenants on vanity or sovereign hosts may need allowlist additions.
* `014.002-T` (in `014-S`) owns unconditional CI enforcement of the
  tracked-file secret scan on docs-only PRs.

Verbose original archived at
`docs/archive/memory/2026-11-10/013-S-browser-security-memory.md`.
