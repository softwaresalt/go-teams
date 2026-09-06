---
type: memory-compacted
date: 2026-07-10
shipment: 007-S
feature: 007-F
source: docs/archive/memory/2026-07-10/007-S-capture-wiring-memory.md
---

# Compacted memory - Shipment 007-S (wire Meeting capture into the CLI)

**Shipped**: PR #19, merge commit `4a17552`. Wired the `browser.Meeting`
capture pipeline into the `join` and `schedule` CLI commands.

## Outcome

New package `internal/capture`: `Session.Run` (join -> concurrent
captions+audio -> teardown; output created only after join; cancellation
normalized to graceful; first-error propagation) and `setupOutput` (atomic
transcript+MP3 pair, `O_EXCL`, ms-timestamp names, owner-only perms,
`filepath.Abs`, sanitized meeting id). `internal/cli`: `newMeeting`/
`newSession` seams + `runCapture` (mandatory browser cleanup);
`browser.NewLiveMeeting` centralizes chromedp lifecycle. `internal/browser`:
replaced best-effort drain with a `payloadPump` producer barrier so no
caption/audio payload is lost on Ctrl+C. `internal/audio`: frame-buffering
fix for the shine-mp3 over-read.

## Review outcome

5 parallel model-diverse pre-PR reviewers (concurrency clean; path-traversal
proven safe; P1 filename collision -> `O_EXCL`+ms; arch P2 ->
`NewLiveMeeting`). 6 Copilot cycles: tail-loss drain -> producer barrier;
shine-mp3 over-read -> frame buffering; header-flush atomicity;
perms/paths doc accuracy; cancellation cleanup test; markdown-injection
residual (stashed, delivered by 008-S). Doc-HEAD staleness recurred ->
fixed structurally (stop pinning a commit-stale SHA in readiness docs).

## Key reusable insight

Only merge when the latest Copilot review's `commit.oid == current HEAD`
AND 0 unresolved threads. `gh pr edit` (PR-body-only edits) does NOT
trigger a new review or change HEAD -- use it to correct the readiness/
follow-ups record without a churn cycle. A closure artifact committed
in-branch can never pin its own resulting commit SHA -- reference the PR
readiness block instead. Marking backlog items `done` archives them
(moves to `.backlogit/archive/`); `backlogit shipment ship` still works
post-merge as long as no `git commit --amend` happened after the
done-marking commit.

## Follow-ups carried forward (delivered/tracked elsewhere)

`FFF4CA31` (transcript Markdown escaping) delivered by 008-S. `44B29911`
(CI `paths-ignore`) and `3E55DF0D` (shine-mp3 `-race`/unsafe) tracked
separately. `CC0A7034` (live DOM verification) deferred to `010-S`,
requires operator (interactive login/MFA + real meeting).

Verbose original archived at
`docs/archive/memory/2026-07-10/007-S-capture-wiring-memory.md`.
