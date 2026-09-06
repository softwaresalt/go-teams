---
type: memory-compacted
date: 2026-07-09
shipment: 001-S
feature: 001-F
source: docs/archive/memory/2026-07-09/cli-foundation-memory.md
---

# Compacted memory - Shipment 001-S (go-teams CLI foundation)

**Shipped**: PR #2, dark-factory Stage -> Ship pipeline. Foundation release
for the standalone `teams-cli` Go CLI.

## Outcome

Established module `github.com/softwaresalt/go-teams` (binary
`cmd/teams-cli`; `go 1.26`, forced by `chromedp v0.15.1`/
`go-json-experiment`). Built `internal/config` (atomic YAML load/save,
temp+fsync+rename), `internal/teams` (`MutateJoinURL`), `internal/transcript`
(rolling dedup + buffered Markdown writer), `internal/scheduler` (cron
`Entry`/`Build`), `internal/browser` (`StealthFlags`/`AllocatorOptions`/
`ChromeRunner`), `internal/cli` + `cmd/teams-cli` (Cobra root). CI re-enabled
with `install-mode: goinstall`.

## Review outcome

No P0/P1. Fixed: schedule shutdown browser reaping + overlap skip (P2),
config fsync (P3). Documented/accepted: single shared browser profile (P2,
deferred), caption trailing-punctuation heuristic (P3).

## Key reusable insight

The official `golangci-lint` release binary is built with an older Go
toolchain and rejects a newer module `go` directive (here, `go 1.26`).
Use `install-mode: goinstall` in CI so the linter compiles with the
module's own toolchain instead of a mismatched prebuilt binary.

## Follow-ups carried forward (delivered in later shipments)

Pre-join/lobby FSM, caption `MutationObserver` + `getDisplayMedia` audio
capture, JS<->Go exfiltration bridge, and pure-Go MP3 encoding were all
deferred at this shipment and delivered by 004-S/005-S/006-S.

Verbose original archived at
`docs/archive/memory/2026-07-09/cli-foundation-memory.md`.
