---
type: design-decision
id: 0001
date: 2026-07-09
status: accepted
shipment: 001-S
---

# 0001 — go-teams CLI foundation: module, testing seams, and scope split

## Context

The product spec (`docs/product-specs/Go-TeamsTranscriptCLI-Architecture.md`)
defines a standalone Go CLI that drives a headless browser to capture Teams
transcripts. The foundation release scaffolds the module and the verifiable
core. Much of the spec (live chromedp automation) cannot be unit-verified
without a real Teams meeting.

## Decisions

### 1. Module target is Go 1.26

`go mod tidy` resolved the `go` directive to `1.26` because `chromedp v0.15.1`
(and its `go-json-experiment/json` dependency) require it. This is stricter than
the AGENTS.md "1.23+" baseline. Accepted rather than pinning older chromedp,
because the toolchain is already 1.26 and downgrading the primary automation
dependency for a version-baseline preference is not worth the loss of CDP
coverage. CI reads the version from `go.mod`.

### 2. Browser automation is verified through interface seams

`internal/browser` exposes a `Runner` interface (`Authenticate`, `Join`).
Commands depend on the interface and are unit-tested with a fake, so command
wiring (config load, URL mutation, error propagation) is fully covered without a
browser. The chromedp-backed `ChromeRunner` is the production implementation;
its live behavior is exercised only against a real browser and will be
`integration`-tagged.

### 3. Pure logic is separated from browser-dependent behavior

URL mutation, caption deduplication, Markdown rendering, config persistence, and
cron construction are pure and fully unit-tested. The live pre-join FSM, caption/
audio JavaScript payloads, exfiltration bridge, and MP3 encoding are deferred to
follow-on release units because they require a live meeting to verify — shipping
them unverified would violate the test-first and verify-before-claim principles.

## Consequences

- The foundation is fully green (build/vet/lint/race-test) and safe to extend.
- Follow-on releases add live behavior behind the existing seams without
  reworking the command layer.
- CI must compile golangci-lint via `goinstall` so the lint binary's build-Go is
  not older than the module's `go 1.26` target (see
  `docs/compound/build-errors/golangci-lint-go-version-mismatch.md`).
