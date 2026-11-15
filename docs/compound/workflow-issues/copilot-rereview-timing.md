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
2. If GitHub did not automatically re-queue `copilot-pull-request-reviewer` on
   the follow-up push, re-request the bot review via GraphQL `requestReviews`
   using `botIds: ["BOT_kgDOCnlnWA"]` (the REST `requested_reviewers` endpoint
   ignores bot logins and returns `requested_reviewers: []` or HTTP 422).
3. Poll `pullRequest.reviews` (or run `autoharness gate copilot-review <pr> --repo softwaresalt/go-teams --max-wait <seconds> --json`)
   until there is a Copilot review whose `commit.oid` equals the pushed HEAD
   SHA. Do **not** rely on `submittedAt`: a review that started for the previous
   SHA can finish (submit) after the push, so a later timestamp does not prove
   the review covered the new HEAD. The `commit.oid` match is the only reliable
   signal. Use the back-off cadence (2-5 min, up to ~15 min).
4. Only then evaluate unresolved threads; address each; re-verify.
5. Merge only when the latest Copilot review is at the current HEAD AND there are
   zero unresolved threads.

Admin fallback bypasses required checks/reviews but MUST NOT be used to merge
before the current-HEAD re-review has actually landed.

## Prevention

- Treat "0 unresolved threads" as meaningful only when a Copilot review exists at
  the current HEAD.
- When `dismiss_stale_reviews_on_push` is on, every fix push starts a new review
  cycle; budget for it before merging.

## Scope (relationship to the advisory-by-default policy)

This learning **complements** — it does not override —
`github-pr-automation.instructions.md`, under which shadow review is advisory by
default and a timed-out or unavailable shadow review is non-blocking. The rule
here applies specifically when an agent has **chosen to address** shadow-review
comments before merge: in that case, do not declare the comments fully addressed
(or merge on that basis) until a Copilot review exists at the current HEAD.
It does not make shadow review a mandatory merge gate, and the standard
review-fix cycle limit still applies — remaining advisory comments at the cycle
limit are surfaced as follow-ups rather than blocking the merge.
