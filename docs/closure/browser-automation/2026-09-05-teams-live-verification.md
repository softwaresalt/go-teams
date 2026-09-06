---
title: Teams Live Verification
date: 2026-09-05
status: blocked
mode: interact
auth: state
---

## Summary

* URL: `https://teams.microsoft.com/v2/`
* Browser adapter: `agent-browser` 0.31.1
* Browser mode: headed first, then headless diagnostic fallback
* Authentication strategy: reuse an existing local browser session if
  available
* Verdict: blocked before authentication

The browser automation runtime is installed, and its isolated doctor check
passes. Live Teams verification could not start because Chrome repeatedly
exited before exposing its DevTools port. No meeting was created, no invitation
was sent, and no Microsoft credentials or browser state were captured.

## Scope and Later Status (annotation added 2026-09-06 — historical record preserved)

**This BLOCKED verdict is scoped to the managed `agent-browser` Chrome adapter
launch attempt only.** The block was the `agent-browser` 0.31.1 *managed* Chrome
process repeatedly exiting before exposing its DevTools port (`DevToolsActivePort`
never written) — a browser-**launch** failure of the managed adapter, NOT a
Teams/selector defect and NOT an integration-test result. The Phase Log, Failures,
and Resumption sections below are the original, unchanged record of that attempt.

**Later partial success on a different launch path (same day, separate
operator-approved session):** Microsoft **Edge** was launched manually with a
workspace-contained temporary profile over **native CDP (port 9223)**; an instant
no-invite meeting was created and joined as **organizer**, and a **guest** session
was manually **admitted** — proving the no-invite two-session topology works end
to end via the Edge / native-CDP path. This does **not** revise the
managed-adapter block above; it records that a different, manually driven launch
path succeeded where the managed adapter did not.

**Integration tests are NOT claimed to pass.** Once the harness could launch the
browser (Chrome-for-Testing via an explicit executable path / PATH), the existing
`TestMeetingJoin` still failed **RED** after ~123s at
`pre-join not visible: context deadline exceeded` — the
`/dl/launcher/launcher.html` → `joinOnWeb` launcher gap, distinct from the
managed-adapter `DevToolsActivePort` launch failure (the two failure signatures
must not be conflated). Greening the three live flows (join / captions / audio) is
the scope of the queued shipment **`010-S`** (feature `010-F`, plan
`docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md`); the **final
browser-verification closure will be recorded when `010-S` ships** and is not
asserted here.

**Redaction:** the live meeting URL, passcode/token, account identity, and
calendar/profile details are deliberately excluded from this record (only the
generic `teams.microsoft.com` route class appears).

## Phase Log

| Phase | Result | Detail |
|---|---|---|
| Environment check | Pass | `agent-browser` 0.31.1 is installed; its doctor launch check passed |
| Authentication setup | Blocked | No reusable Chrome profile was found, and the Teams browser could not launch |
| Navigate and snapshot | Blocked | Chrome exited before writing `DevToolsActivePort` |
| Interact | Skipped | No controllable Teams page was available |
| Session management | Skipped | No authenticated state was established |
| Visual verification | Skipped | No page snapshot was available |

## Snapshots

No screenshots or DOM snapshots were produced because the browser failed
before navigation became controllable.

## Human Checkpoints

No Microsoft OAuth or MFA checkpoint was reached.

## Failures

| Operation | Target | Failure |
|---|---|---|
| Headed Chrome launch | Microsoft Teams web | Chrome exited before writing `DevToolsActivePort` |
| Headed session snapshot | Microsoft Teams web | Chrome exited before writing `DevToolsActivePort` |
| Headless diagnostic launch | Microsoft Teams web | Chrome exited before writing `DevToolsActivePort` |

The retry circuit is open. Detailed bounded evidence is recorded in
`docs/memory/2026-09-05/circuit-break-agent-browser-chrome-launch.md`.

## Resumption

Diagnose the local `agent-browser` Chrome startup failure in a new
operator-approved session. After Chrome remains attached, open Teams in headed
mode, complete any Microsoft OAuth or MFA checkpoint, start an instant meeting,
and run the existing join, caption, and audio integration tests against its
join URL.
