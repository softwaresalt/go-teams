---
type: memory-compacted
date: 2026-07-10
shipment: 008-S
feature: 008-F
source: docs/archive/memory/2026-07-10/008-S-and-darkmode-completion-memory.md
---

# Compacted memory - Shipment 008-S (transcript hardening) + dark-mode RUN 2 completion

**Shipped**: PR #21, merge `e4f0c9d`.

## Outcome

Hardened the Markdown transcript against participant-controlled injection:
`internal/transcript.sanitizeField` (applied to speaker + caption text in
`WriteEvent`) collapses control chars and Unicode line/paragraph separators
to spaces, backslash-escapes structural runes (`` \ ` * _ [ ] < > # | ~ ``),
and defuses bare URL/email autolinks with a zero-width space.

## Review outcome

3 pre-PR adversarial reviewers + 1 Copilot cycle (email autolinks, tilde,
U+2028/9 tests, doc accuracy, restored the `44B29911` CI signal) -- all
resolved.

## Dark-mode RUN 2 final state

This run shipped both 007-S (capture wiring) and 008-S (transcript
hardening), fully reviewed (adversarial + Copilot) and closed. Remaining
stash, all deferred with rationale: `CC0A7034` (live Teams DOM
verification -- operator-required, interactive M365 login/MFA + real
meeting, cannot run autonomously); `3E55DF0D` (replace/vendor shine-mp3 --
high-blast-radius dependency decision, recommend operator input; later
resolved in-place by the 009-S buffer-slack fix rather than a
replacement); `D3EFD224` (CI `paths-ignore` -- no value while
`required_status_checks=[]`).

## Key reusable insight

`gh pr edit` (PR-body-only edits) updates the readiness/follow-ups record
without a commit or a new Copilot review -- use it to satisfy
readiness-enumeration comments without a churn cycle.

Verbose original archived at
`docs/archive/memory/2026-07-10/008-S-and-darkmode-completion-memory.md`.
