---
title: "Live Teams DOM Selector & Join-Path Hardening for Browser FSM/Caption/Audio Verification"
description: "Decide the covering-feature abstraction and decomposition posture for live-verifying and hardening the VERIFY-LIVE browser selectors against a real authenticated Teams meeting."
topic: "Live end-to-end verification and hardening of the FSM/caption/audio DOM selectors (stash CC0A7034)"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md"
source_stash: "CC0A7034"
tags:
  - "browser-verification"
  - "teams"
  - "selector-hardening"
  - "runtime-verification"
---

## Problem Frame

Stash `CC0A7034` (high priority, task-shaped) asks for live end-to-end
verification of the FSM / caption / audio DOM selectors against a real
authenticated Microsoft Teams meeting. Every browser selector is currently
marked `VERIFY LIVE` in `internal/browser/prejoin.go`, `captions.go`, and
`audio.go`; they were derived from the architecture spec and public references,
never confirmed against live Teams DOM.

A parent Orchestrator session produced verified live evidence that turns this
from an open-ended "go verify" task into a concrete, evidence-grounded hardening
job:

* The managed `agent-browser` Chrome launch failed (DevTools port never
  exposed; circuit breaker open — see
  `docs/memory/2026-09-05/circuit-break-agent-browser-chrome-launch.md`).
  Microsoft Edge was then launched manually with CDP port 9223 against a
  workspace-contained temporary profile and authenticated via the user's
  Microsoft account.
* An instant no-invite meeting ("go-teams live verification") was created and
  joined as organizer; live caption controls are visible. The organizer browser
  process MUST be left running and unaltered.
* The integration harness (`internal/browser/meeting_integration_test.go`) was
  run with `GO_TEAMS_INTEGRATION=1`, `TEAMS_PROFILE_DIR` pointing at a separate
  empty bot profile, and `TEAMS_MEETING_URL` loaded from an out-of-tree private
  file. The first run failed because no `google-chrome` binary was on PATH;
  after prepending the installed Chrome-for-Testing directory to PATH,
  `TestMeetingJoin` launched but failed after ~123s with:
  `browser: pre-join not visible: context deadline exceeded`.

This is the expected RED live harness. The observed root-cause surface:

* Current Teams-generated join links use
  `https://teams.microsoft.com/meet/<id>?p=<token>`, whereas the existing code
  and unit tests primarily model the older `/l/meetup-join/...` links.
* `teams.ValidateMeetingURL` accepts the new URL (scheme + host only), but
  `teams.MutateJoinURL` unconditionally appends `type=meetup-join` and other
  bypass params tuned for the legacy flow — those may not correctly force the
  web pre-join SPA for the `/meet/` flow.
* `browser.chromeMeeting.Join` waits on `joinNowSel`
  (`[data-tid="prejoin-join-button"]`). The wait timed out, so either the
  selector is stale for the current `/meet/` pre-join DOM, or the mutated URL
  landed on a non-pre-join surface (desktop-app interstitial / unsupported-UA
  page), or both.

### Who cares and why

The operator wants autonomous, reproducible proof that the transcription bot can
actually join a live meeting, enable captions, and capture audio — the browser
automation is inert until these `VERIFY LIVE` selectors are confirmed working.

### Constraints

* Stage plans and decomposes only. It MUST NOT modify production/test source,
  run the harness/build, claim the shipment, or open a product PR (P-010).
* The private join link lives only in the operator's out-of-tree session-state
  file and MUST NOT be copied into backlog items, docs, or commits. Downstream
  work references it exclusively through the `TEAMS_MEETING_URL` env var.
* The manually launched Edge organizer session and its CDP port 9223 MUST NOT be
  terminated or altered by downstream work.
* Existing blocked/closure evidence docs (`docs/memory/2026-09-05/...` and
  `docs/closure/browser-automation/...`) are parent-owned; do not edit unless
  Stage policy requires it.

### Success criteria

`TestMeetingJoin`, `TestMeetingEnableCaptions`, and `TestMeetingCaptureAudio`
all pass green against the live authenticated meeting, with the `VERIFY LIVE`
markers resolved and a reproducible runbook captured as closure evidence.

### Out of scope

* The latent CI paths-ignore config issue (stash `D3EFD224`) — separate entry.
* Any refactor of the payload-pump / dedup / encoder internals that are already
  unit-tested and not implicated by the live failure.
* Net-new capture features beyond making the existing three flows verifiably
  green.

## Research Findings

* `internal/teams/url.go`: `ValidateMeetingURL` is host/scheme-only (already
  accepts `/meet/` links). `MutateJoinURL` applies a fixed `bypassParams` map
  (`msLaunch=false`, `type=meetup-join`, `directDl=true`,
  `enableMobilePage=true`, `suppressPrompt=true`). `type=meetup-join` is
  semantically legacy for a `/meet/` link.
* `internal/browser/prejoin.go`: pre-join / lobby / toolbar selectors and lobby
  text — all `VERIFY LIVE`. The join FSM (`meeting_chrome.go`) hard-waits on
  `joinNowSel` before muting/injecting name/clicking join.
* `internal/browser/captions.go`: caption menu-activation selectors
  (`more-button` → `language-speech-button` → `closed-captions-button`) and the
  `MutationObserver` keyed to `closed-caption-renderer-wrapper` /
  `closed-caption-text` / `author` — all `VERIFY LIVE`.
* `internal/browser/audio.go`: `getDisplayMedia` capture path depends on the
  `--auto-select-desktop-capture-source="Microsoft Teams"` allocator flag (see
  `allocator.go`) — `VERIFY LIVE`.
* `internal/browser/allocator.go`: pins `DefaultUserAgent` at Chrome/125 and
  `--headless=new`; the harness `browserContext` launches its OWN browser via
  `chromedp.NewExecAllocator` (chromedp's default binary discovery), which is
  why a missing `google-chrome` binary aborted the first run.
* `docs/compound/` learnings library holds 5 entries. Two — `shine-mp3-frame-
  alignment.md` and `shine-mp3-race-checkptr.md` — are directly relevant to the
  audio-capture verification (the MP3 encoder is already hardened; do not
  re-break frame alignment or `-race` behavior). Two workflow-issues entries
  (`amend-breaks-shipment-gate.md`, `copilot-rereview-timing.md`) govern closure/
  PR discipline. The DevToolsActivePort circuit-break learning lives in
  `docs/memory/`, not yet promoted into `docs/compound/`.
* Backlog state is a clean slate: empty queue, no shipments, no existing
  covering feature for this work.

## Options Evaluated

### Option A: Single "iterate until green" task (mirror the stash text literally)

One task that runs the harness and iterates on all selectors and timing across
URL, pre-join, captions, and audio until all three tests pass.

* **Pros**: Matches the stash wording; minimal backlog ceremony.
* **Cons**: Violates the 2-hour rule (multi-hour live iteration); conflates four
  skill domains (URL code, pre-join code, caption code, audio code) plus docs;
  high, undecomposed complexity with no de-risking of the live-DOM unknown;
  one big blast radius; hard for Ship to checkpoint.
* **Effort**: high. **Fit**: poor.

### Option B: Diagnostic-first, per-surface decomposition (chosen)

A live-DOM diagnostic task first characterizes the current `/meet/` pre-join DOM
and pins the root cause (stale selector vs. wrong landing surface). That
de-risks the high-complexity selector work. Then width-isolated hardening tasks
follow — join-path URL mutation, pre-join/lobby FSM selectors, caption
activation/observer, audio capture path — each gated on its own live integration
test, closed out by a reproducibility runbook.

* **Pros**: Every task fits the 2-hour rule and a single skill domain; the
  diagnostic front-loads and collapses the live-DOM uncertainty so downstream
  tasks are apply-and-verify; clean dependency DAG; each task has a concrete
  green-test exit; matches browser-verification closure-handoff discipline.
* **Cons**: More backlog items; the diagnostic and the pre-join fix are tightly
  coupled and could feel redundant if the root cause is trivial.
* **Effort**: medium. **Fit**: strong.

### Option C: Two-phase "verify pass, then fix pass"

A pure verification pass to enumerate what fails, then a separate fix pass.

* **Pros**: Clean conceptual split.
* **Cons**: Artificial here — you cannot verify live selectors without a live
  browser, and once the live DOM is in hand you fix immediately; the split
  doubles live-session setup cost for no de-risking benefit beyond what the
  Option B diagnostic already provides.
* **Effort**: medium-high. **Fit**: weak.

## Trade-off Comparison

| Criterion | Option A | Option B | Option C |
|---|---|---|---|
| 2-hour-rule compliance | Fails | Passes | Passes |
| Width isolation (single domain/task) | Fails | Passes | Partial |
| De-risks live-DOM unknown | No | Yes (diagnostic first) | Partial |
| Dependency clarity | N/A (monolith) | Clean DAG | Linear, coarse |
| Ship checkpointing | Poor | Strong | Moderate |
| Backlog ceremony | Lowest | Moderate | Higher |

## Decision

Adopt **Option B**. The group of live-verification actions forms one coherent
covering feature: **"Live Teams DOM selector & join-path hardening for browser
FSM/caption/audio verification."** The abstraction is correct at the feature
level (backlogit has no `chore` type configured; `feature` is the top-level
release unit). Decompose into a diagnostic-first DAG:

1. Live pre-join/lobby DOM diagnostic (spike/characterization posture) — pins
   root cause and captures selectors; de-risks the high-complexity work.
2. Join-path URL mutation hardening for the `/meet/<id>?p=<token>` flow
   (`internal/teams/url.go`, test-first).
3. Pre-join/lobby FSM selector update → green `TestMeetingJoin`
   (`internal/browser/prejoin.go`).
4. Caption activation/observer selector update → green
   `TestMeetingEnableCaptions` (`internal/browser/captions.go`).
5. Audio capture path verification → green `TestMeetingCaptureAudio`
   (`internal/browser/audio.go`).
6. Live-harness reproducibility runbook + closure evidence (docs).

This scope confirms the covering feature, keeps every task inside the 2-hour and
width-isolation constraints, and threads the private-URL and organizer-session
constraints into every downstream item.

## Rejected Alternatives

* **Option A** — rejected for violating the 2-hour rule and conflating four
  domains with an undecomposed high-complexity live unknown.
* **Option C** — rejected because verify and fix are physically inseparable for
  live selectors; the split adds live-session cost without added de-risking.

## Unresolved Questions

* Is the `pre-join not visible` failure a stale `joinNowSel`, a wrong landing
  surface from the legacy `type=meetup-join` mutation, an unsupported-UA
  interstitial (Chrome/125 UA), or a `--headless=new` DOM difference? The
  diagnostic task (unit 1) resolves this before selector edits.
* Does the empty bot profile need lobby admission by the organizer for
  `TestMeetingJoin` to reach the admitted state, and should that operator
  admission be an explicit human checkpoint in the harness runbook?
* Should the harness optionally attach to an existing authenticated CDP endpoint
  (RemoteAllocator) instead of always launching its own browser? Flagged for the
  diagnostic/runbook tasks; not pre-committed to keep scope narrow.

## Risks and Mitigations

* **External live dependency (Teams DOM drift)** — selectors verified today can
  drift. Mitigation: keep `VERIFY LIVE`-style provenance comments and capture a
  dated runbook so re-verification is cheap.
* **Operator/human checkpoints (OAuth/MFA, lobby admission)** — not fully
  automatable. Mitigation: model them as explicit browser-verification human
  checkpoints in the plan and runbook.
* **Secret leakage of the private join link** — Mitigation: downstream work
  references only `TEAMS_MEETING_URL`; no artifact embeds the URL.
* **Disturbing the organizer session** — Mitigation: tasks explicitly forbid
  terminating/altering the Edge organizer (CDP 9223) and use the separate empty
  bot profile.
* **High runtime/rollback risk of live iteration** — Mitigation: plan hardening
  is required; each task has a bounded live scope and a single green-test exit.
