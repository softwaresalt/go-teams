---
type: memory-compacted
date: 2026-11-10
shipment: 016-S
feature: 016-F
source: docs/archive/memory/2026-11-10/016-S-cli-self-update-command-memory.md
---

# Compacted memory - Shipment 016-S (teams-cli update)

**Shipped**: PR #40, merge `f762dd5e2a4181a1e018821f7d01e88060fedfbf`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship --sha`). Source deliberation `005-DL` was archived by the
explicit source-artifact cleanup step; stash `A08726BC` was already archived.

## Decision and outcome

* `teams-cli update` (`--check`, `--version`, `--force`) downloads the platform
  binary from a GitHub Release, verifies its SHA-256 against the release
  `SHA256SUMS` before touching the installed executable, and swaps it in with a
  `.old` rename sequence that rolls back on failure. Packages: `internal/version`
  (link-time `Version`/`Commit`/`BuildDate`, SemVer compare) and `internal/updater`
  (standard library only); the CLI reaches the updater through `deps.runUpdate`.
* `release.yml` publishes raw executables, so there is no archive extraction code.
* Downgrades and same-version reinstalls are gated by `--force` inside the updater.
* Accepted residual risk: `SHA256SUMS` is unsigned, so a compromised release would
  carry a matching checksum.

## Key reusable insights

* A `defer` that reads a named result sees the value set by `return "", err`, so
  cleanup keyed on a named return path silently does nothing. Use a local variable
  for the path and put the verify step in its own helper.
* Re-validate every redirect target (`CheckRedirect`) when a scheme rule such as
  HTTPS-only matters; a validated first URL does not cover redirects.
* Strip query strings from URLs that appear in errors, including the `*url.Error`
  returned by `http.Client.Do`, and keep the chain intact for `errors.Is`.
* SemVer: build metadata does not affect precedence but is part of the tag name,
  so the release lookup needs the raw tag. Compare numeric pre-release identifiers
  by digit width without integer conversion.
* A `--check` that advertises an update must prove the release is installable
  (platform asset and checksum file present) without downloading it.
* Moving a task or feature to `done` relocates it to `.backlogit/archive/`, so the
  cascade close sees already-archived members; `returned_ids: []` still holds.
* The shipment ship flag is `--sha`, not `--commit`.

## Review outcome

Local review: `READY_WITH_FOLLOWUPS`, `P0=0, P1=0` (single-agent declared
degradation). Copilot raised six threads on PR #40: five fixed in scope under
P-021 C1 (`ba22f60`) and one out of scope (inter-process update lock) captured as
deferred entry `2D0BAA19`. The `copilot-review` gate returned `SATISFIED` and CI
passed before the merge commit.

## Post-merge closure summary

P-015 classification returned `CASCADE`. `backlogit shipment ship` returned
`archived_ids: [016.001-T, 016.002-T, 016.003-T, 016.004-T, 016.005-T, 016-F,
016-S]` and `returned_ids: []`; all tasks kept `parent_id: 016-F`.
