---
type: session-memory
date: 2026-07-10
shipment: 007-S
outcome: shipped
---

# Session memory — Shipment 007-S (wire Meeting capture into the CLI)

## Completed

- **007-S shipped** (PR #19, merge commit `4a17552`). Wired the `browser.Meeting`
  capture pipeline into the `join` and `schedule` CLI commands.
- New package **`internal/capture`**: `Session.Run` (join → concurrent
  captions+audio → teardown; output created only after join; cancellation
  normalized to graceful; first-error propagation) and `setupOutput`
  (atomic transcript+MP3 pair, `O_EXCL`, ms-timestamp names, owner-only perms,
  `filepath.Abs`, sanitized `[a-z0-9-]+` meeting id).
- **`internal/cli`**: `newMeeting`/`newSession` seams + `runCapture` (mandatory
  browser cleanup on success/failure/cancellation); `schedule` non-blocking
  busy-skip profile gate; `browser.NewLiveMeeting` centralizes chromedp lifecycle.
- **`internal/browser`**: replaced best-effort drain with a **`payloadPump`**
  producer barrier (mutex-guarded `closing` + `prodWG` + `close(payloads)`) so no
  caption/audio payload is lost on Ctrl+C.
- **`internal/audio`**: frame-buffering fix for the shine-mp3 over-read (see
  `docs/compound/build-errors/shine-mp3-frame-alignment.md`).

## Adversarial review (before PR) + 6 Copilot cycles

- 5 parallel model-diverse reviewers pre-PR (concurrency clean; security proved
  path-traversal safe; found P1 filename collision → `O_EXCL`+ms; arch P2 →
  `NewLiveMeeting`). Record: `docs/closure/2026-07-10-007-S-review.md`.
- Copilot review found and I fixed: tail-loss drain → producer barrier;
  shine-mp3 over-read → frame buffering; header-flush atomicity; perms/paths
  doc accuracy; cancellation cleanup test; markdown-injection residual (stashed
  `FFF4CA31`). Doc-HEAD staleness recurred → fixed structurally (stop pinning a
  commit-stale SHA; point to PR readiness block).

## Key gotchas (reusable)

- **Copilot re-review gate**: only merge when the latest Copilot review's
  `commit.oid == current HEAD` AND 0 unresolved threads. PR-body edits (`gh pr
  edit`) do NOT trigger a new review or change HEAD — use them to fix the
  readiness/follow-ups record without a churn cycle.
- **Self-referential doc SHA**: a closure artifact committed in-branch can never
  pin its own resulting commit; don't chase the SHA — reference the PR readiness
  block (the merge-gate source of truth) instead.
- **Marking backlog items `done` archives them** (moves to `.backlogit/archive/`).
  `backlogit shipment ship` still works post-merge when no `git commit --amend`
  happened after the done-marking commit.

## Remaining stash (dark-mode scope)

- `44B29911` (high) — CI `paths-ignore` fix so backlog/docs PRs report the
  required check (next).
- `3E55DF0D` (med) — shine-mp3 inherent `-race`/unsafe (replace/vendor encoder).
- `FFF4CA31` (med) — transcript Markdown escaping.
- `CC0A7034` (high) — **live DOM verification: DEFERRED, requires operator**
  (interactive login/MFA + real meeting; cannot run autonomously).

## Next

Continue dark-mode pipeline: Stage+Ship `44B29911` (CI fix), then assess
`3E55DF0D`/`FFF4CA31`; defer `CC0A7034` to operator.
