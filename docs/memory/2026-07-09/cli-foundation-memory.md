---
type: session-memory
date: 2026-07-09
agent: orchestrator (dark-factory pipeline)
shipment: 001-S
feature: 001-F
pr: 2
branch: feat/cli-foundation
---

# go-teams CLI foundation — session memory

## Outcome

Shipped the foundation release unit for the `teams-cli` standalone Go CLI as
PR #2, driven end-to-end through a dark-factory Stage → Ship pipeline. All local
gates and CI are green. Awaiting operator merge approval
(`merge_approval_pre_authorized=false`).

## Work items completed

- Feature `001-F` (done) with tasks `001.001-T`..`001.007-T` (all done).
- Shipment `001-S` (active — remains active until PR #2 merges and closure runs).

## Files created / changed

- `go.mod`, `go.sum`, `.golangci.yml`
- `internal/config/` — Config model + atomic YAML Load/Save (temp+fsync+rename)
- `internal/teams/` — `MutateJoinURL` bypass-param builder
- `internal/transcript/` — rolling-text `Deduplicator` + buffered Markdown `Writer`
- `internal/scheduler/` — cron `Entry` + `Build` (accepts cron options)
- `internal/browser/` — `StealthFlags`/`AllocatorOptions` + `ChromeRunner`
- `internal/cli/` + `cmd/teams-cli/` — Cobra root + auth/join/schedule, main
- `.github/workflows/ci.yml` — re-enabled test job; `install-mode: goinstall`
- `README.md`

## Key decisions

- Module path `github.com/softwaresalt/go-teams`; binary `cmd/teams-cli`.
- Go directive is `go 1.26` (forced by `chromedp v0.15.1` / `go-json-experiment`
  after `go mod tidy`), stricter than the AGENTS.md "1.23+" baseline.
- Browser automation is verified via interface seams (`browser.Runner`) + unit
  tests on pure helpers; live meeting behavior is deferred (needs a real Teams
  meeting) and would be `integration`-tagged.
- Commits split per scope, each buildable in isolation, merge-commit-only.

## Review findings (code-review persona)

No P0/P1. Fixed: schedule shutdown browser reaping + overlap skip (P2), config
fsync (P3). Documented/accepted: single shared browser profile (P2, deferred),
caption trailing-punctuation heuristic (P3).

## Failed approach

- Initial CI used the official `golangci-lint v1.64.8` binary (built with
  go1.24); it rejects a go 1.26 module. Fixed by `install-mode: goinstall` so
  CI compiles golangci-lint with its own Go 1.26 toolchain.

## Open items / next steps

1. Operator: disable squash + rebase merging in repo settings (P-009), then
   approve and merge PR #2 with a **merge commit**.
2. Post-merge closure for shipment `001-S` (shipment-reconcile, graduate design
   notes).
3. Follow-on release units (deferred, need live Teams): pre-join/lobby FSM,
   caption MutationObserver + `getDisplayMedia` audio payloads, JS↔Go
   exfiltration bridge, pure-Go MP3 encoding.
