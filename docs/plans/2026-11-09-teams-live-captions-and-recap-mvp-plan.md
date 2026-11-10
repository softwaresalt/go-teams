---
title: Teams live captions and recap MVP implementation plan
date: 2026-11-09
status: implemented
feature: 011-F
shipment: 011-S
---

## Problem frame

The existing browser MVP needs a complete input/output CLI contract and
late-rendering recap summary retries. We consolidate its implementation and
release evidence on the operator-authorized `feat/mvp-live-captions` branch.
This plan records earlier implementation retrospectively; only new regression
work has red/green evidence from this session.

Source: [accepted decision](../decisions/2026-11-09-teams-live-captions-and-recap-mvp-deliberation.md).

## Requirements trace

| Requirement | Implementation units | Evidence |
|---|---|---|
| R1, R2: URL and output arguments | U5a, U5b | CLI argument matrix and output tests |
| R3: Headless profile reuse | U1, U2, U5a | Launch tests and compiled fixture |
| R4: Settled live captions | U3a, U3b | FSM, observer, payload-pump and synthetic live tests |
| R5: Recap transcript and summary | U4a, U4b | Recap parsing and writer tests |
| R6: Late target discovery | U4c | Synthetic CDP regression, observed red then green |
| R7: Durable lifecycle artifacts | U6 | Linked documents and backlog records |
| R8: Compiled verification and PR | U7 | Executable harness, gates and current-HEAD PR readiness |

## Prior learnings

The repository's `golangci-lint-go-version-mismatch` learning requires
matching the CI lint tool version and building it with a sufficiently new Go
toolchain. CI pins golangci-lint v1.64.8 with `goinstall`; the globally
installed v2 cannot read the checked-in v1 configuration. Use a
workspace-local CI-pinned binary rather than changing CI as part of this MVP.
The host Go 1.27.3 export format also requires selecting the module's
Go 1.26.0 toolchain for compatible lint analysis.

## Implementation units

Each sub-unit is bounded to one domain and approximately two hours. The five
backlog tasks group these small milestones; no new parallel worktree is used.

| Unit | Backlog | Surface and files | Posture and verification |
|---|---|---|---|
| U1 | 011.001-T | URL normalization and browser executable discovery | Retrospective characterization; URL/discovery tests |
| U2 | 011.002-T | `browser/recap.go` clean launch flags | Retrospective characterization; headed/headless argument tests |
| U3a | 011.003-T | `browser/meeting_chrome.go` admission and captions | Existing implementation; FSM and binding tests |
| U3b | 011.003-T | Browser observer payload | Existing implementation; settle/dedup and local live-page harness |
| U4a | 011.004-T | `browser/recap.go` transcript and summary extraction | Existing implementation; parsing tests |
| U4b | 011.004-T | `transcript/writer.go`, `capture/session.go` | Existing implementation; offset and summary-divider tests |
| U4c | 011.004-T | `browser/recap.go`, `browser/meeting_chrome.go` | Test-first late-target failure, then bounded rediscovery |
| U5a | 011.005-T | `cli/root.go`, `cli/join.go` | Existing edits plus regression-first directory precedence repair |
| U5b | 011.005-T | `config/config.go`, `capture/output.go`, session wiring | Existing explicit-file edits; output creation/replacement tests |
| U6 | 011-F | README, decision, design, plan, closure and memory | Documentation-only; links, frontmatter and privacy checks |
| U7 | 011-S | Build, compiled fixture, review and PR | Verify first; never claim an unexecuted live-tenant check |

## Dependency graph

U1 and U2 are independent prerequisites for live and recap browser capture.
U3 depends on U1 and U2. U4 depends on U2. U5 completes output routing;
the executable acceptance gate depends on U3, U4, and U5. Explicit task
prerequisites are recorded through backlogit dependency operations.
Documentation records the resulting contracts and evidence, not a second
task tracker.

## Decisions and rationale

* Keep standard-library filesystem and HTTP support; add no production dependency
* Keep clean CDP capture launch distinct from legacy authentication allocation
* Refresh targets rather than retrying only stale websocket addresses
* Treat a missing summary as optional so transcript export remains useful
* Keep explicit directory semantics independent of filename extension
* Document explicit-path replacement rather than imply transactional overwrite protection
* Preserve historical commits and use one implementation branch

## Constitution check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Go implementation, checked errors, vet/lint/build gates |
| II: Test first | New retry and directory-precedence regressions observed red before fixes; historical provenance not fabricated |
| III, IV: Isolation and containment | Agent artifacts remain in this workspace; synthetic fixture inputs only |
| V: Observability | Backlog history, gate results, closure and PR readiness |
| VI: Single responsibility | No dependency expansion; bounded package changes |
| VII, VIII: Approval and safety | No deletion, history rewrite, merge, or real credential operation |
| IX, X: Persistence and context | Git-friendly documentation and backlogit continuity |
| XI: Merge history | PR only; future merge must use a merge commit and fresh approval |

## Plan hardening signals

* Public contract change: present; `-o` now accepts files and directories, with explicit aliases
* Security/auth-sensitive behavior: present; existing browser profiles contain credentials
* Destructive action: present in product behavior; explicit destinations replace prior files after admission
* External dependency/operator checkpoint: present; Teams authentication, admission and recap availability
* Runtime/rollback risk: present; DOM selectors, private React properties and missing summaries

Requires plan hardening: yes

Hardening controls are recorded below and in the closure record. This is a
retrospective consolidation, not a claim that an independent pre-implementation
planning gate occurred for historical commits.

## Risk controls and proposed actions

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Local source, tests and docs on existing branch | moderate | Operator-authorized scope; no new branch | applied |
| Run synthetic browser fixtures | moderate | New isolated fixture profiles; loopback server only | applied |
| Reuse real authenticated profile | high | Not executed; operator-assisted checkpoint | planned |
| Replace explicit output path during capture | high | Documented CLI behavior; verify only fresh synthetic destinations | applied |
| Push branch and open PR | moderate | Explicit operator request; no merge authority | planned |

## Runtime verification and closure

U1-U5 change runtime behavior. Verify `gofmt`, both vet modes, CI-compatible
lint, the full race suite, and `go build ./...`. Build `bin\teams-cli.exe`
and run its opt-in synthetic recap harness across positional, short-flag,
long-flag, file, and directory forms from another working directory.

U6 requires valid frontmatter, working cross-links, and privacy review.
U7 requires ignored binaries/profiles/logs, commit traceability, a pushed
branch, and a PR readiness block matching the final HEAD.

Real Teams auth/MFA, lobby admission, full transcript completeness, and audio
fidelity remain explicitly unverified. The
[closure record](../closure/2026-11-09-011-S-review-and-closure.md)
defines monitoring, owner, observation duration, rollback triggers, and
post-merge work. The shipment is not shipped or archived before merge.
