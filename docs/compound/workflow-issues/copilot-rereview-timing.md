---
type: compound-learning
category: workflow-issues
date: 2026-07-10
tags: [copilot-review, ruleset, merge-gate, dismiss-stale-reviews]
evidence: PRs 11/12/14 merged past re-review comments
---

# Merging past Copilot re-review comments (false-clean thread count)

## Problem

Several PRs (11, 12, 14) were merged while a Copilot re-review still had
unaddressed comments. The merge gate checked "0 unresolved review threads" right
after pushing a fix and saw zero — then merged. Copilot then posted a new review
with additional comments *after* the merge, which were never addressed.

## Root cause

The repository ruleset has `dismiss_stale_reviews_on_push: true` and
`copilot_code_review.review_on_push: true`. When a fix is pushed:

1. The previous Copilot review is dismissed.
2. A **new** Copilot review is queued and runs asynchronously (2-5 minutes).

Polling `reviewThreads` immediately after the push (or after a fixed short wait)
returns **zero unresolved threads** — not because the code is clean, but because
the new review has not posted its threads yet. Merging on that false-clean
signal skips the re-review's findings.

## Resolution / correct gate

After pushing any fix, do NOT trust an empty unresolved-thread list until a
**fresh** Copilot review exists for the current HEAD:

1. Record the pushed HEAD SHA.
2. Poll `pullRequest.reviews` until there is a Copilot review whose
   `commit.oid == HEAD` (or `submittedAt` is after the push time). Use the
   back-off cadence (2-5 min, up to ~15 min).
3. Only then evaluate unresolved threads; address each; re-verify.
4. Merge only when the latest Copilot review is at the current HEAD AND there are
   zero unresolved threads.

Admin fallback bypasses required checks/reviews but MUST NOT be used to merge
before the current-HEAD re-review has actually landed.

## Prevention

- Treat "0 unresolved threads" as meaningful only when a Copilot review exists at
  the current HEAD.
- When `dismiss_stale_reviews_on_push` is on, every fix push starts a new review
  cycle; budget for it before merging.
