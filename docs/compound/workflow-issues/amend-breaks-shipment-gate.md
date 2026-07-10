---
type: compound-learning
category: workflow-issues
date: 2026-07-10
tags: [backlogit, shipment, git-amend, gate]
evidence: shipment 004-S close attempt
---

# `git commit --amend` after marking backlog items done breaks `shipment ship`

## Problem

`backlogit shipment ship 004-S` refused with:

```text
shipment refused: member 004.001-T gate evidence is stale (recorded at a
divergent head): gate blocked: 004.001-T remains active
```

even though `004.001-T` was `done` and the code was correctly merged to `main`.

## Root cause

The task was marked `done` on the feature branch, then the feature commit was
`git commit --amend`ed (to fold in a small review fix) before pushing. Amending
rewrote the commit SHA, orphaning the git head that the shipment's completion
("gate evidence") was recorded against. The `shipment ship` reconcile gate
checks that each member's completion head is consistent with the current/merged
head; the orphaned head reads as "divergent → stale", so it treats the member
as not-completed and refuses the cascade close.

The `done → active` transition is also disallowed, so the evidence cannot be
refreshed by re-cycling the status.

## Resolution

Close the shipment via the sanctioned per-artifact path instead of the cascade
`shipment ship`:

```text
backlogit archive 004-F          # feature (done) -> archived
backlogit move 004-S --status done
backlogit archive 004-S          # shipment -> archived
```

The merge SHA is recorded in the closure doc and PR, so traceability is
preserved. This mirrors the shipment-reconcile skill's single-artifact
safe-close.

## Prevention

- **Do not `git commit --amend` (or rebase) a feature branch after marking its
  backlog items `done`.** Add review fixes as new commits instead.
- If an amend already happened, close the shipment with per-artifact
  `backlogit archive` rather than the cascade `shipment ship`.
