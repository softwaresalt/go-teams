---
type: compound-learning
category: workflow-issues
date: 2026-11-11
tags: [backlogit, shipment, deliberation, retained_engine_unverified, cascade-close, principle-iv]
evidence: shipment 017-S closure, deliberation 006-DL, stash entry 9A9D8106, shipment 019-S
---

# Retained `retained_engine_unverified` deliberations need a post-closure archival path

## Problem

After shipment `017-S` closed through the cascade path, its linked deliberation
`006-DL` stayed in `.backlogit/queue/` with `status: queued`. Nothing in the
closure flow ever archived it, and stash entry `9A9D8106` recorded the follow-up.

## Root cause

Shipment closure classifies each linked deliberation with an INV-12 disposition.
With backlogit 1.12.0 installed, the external `autoharness` package does not list
that minor line in `VERIFIED_CASCADE_ENGINE_MINOR_LINES` (it lists only `"1.11"`).
The engine semantics are therefore unverified, and the classifier fails closed to
`retained_engine_unverified`: retain, never archive. Cascade Close Sub-Procedure
step 7 in `shipment-reconcile` correctly refuses to archive that disposition.

The gap was that no later step owned the orphan once closure had finished.

## Why not edit the engine allow-list

`VERIFIED_CASCADE_ENGINE_MINOR_LINES` is defined in the external `autoharness`
Python package. Constitution Principle IV (CLI Workspace Containment) places that
package outside the target repository boundary. Never edit it, or any
site-packages file, from within the workspace. Widening the verified-engine list
is a change for the package maintainers.

## Solution

Stage or the operator archives the orphaned deliberation in a later triage or
hygiene pass. Ship's automated closure step never does this.

Checklist before running `backlogit archive <dl_id>`:

1. The covering shipment is in `.backlogit/archive/` with
   `archived_status: shipped`.
2. The covering feature is in `.backlogit/archive/` with `archived_status: done`.
3. The deliberation still has `status: queued` in `.backlogit/queue/`.
4. The deliberation has zero live governing/dependency references.

### Live references versus audit citations

A **live governing/dependency reference** is either of:

* any unarchived artifact in `.backlogit/queue/`, in any status (`queued`,
  `active`, `blocked`, or `done`), whose `custom_fields.source_deliberation_id`,
  frontmatter `dependencies` or `references`, or unshipped covering-feature or
  shipment scope is governed by the deliberation
* any active stash entry in `.backlogit/stash.jsonl` whose `deliberation_id`
  links to the deliberation, excluding the cleanup stash entry that itself
  directs archiving the orphan

An **audit citation** is historical disposition prose in a later cleanup
deliberation, feature, shipment, or task. It does not block archival. For
example, `008-DL`, `019-F`, `019-S`, and `019.002-T` record that `006-DL` was
archived per `9A9D8106`; they describe what happened and do not govern `006-DL`.

When `backlogit archive 006-DL` ran in Step 1 of the cleanup, `.backlogit/queue/`
had zero occurrences of `006-DL` of any kind, so the archive was safe by either
reading.

## Prevention

* Treat `retained_engine_unverified` as a closure-time retention, not a final state.
* Capture a follow-up stash entry when closure retains a deliberation, as `017-S`
  did with `9A9D8106`.
* Re-run the reference scan against the live queue and stash immediately before
  archiving, not from an earlier snapshot.
* Leave `VERIFIED_CASCADE_ENGINE_MINOR_LINES` alone; record engine-verification
  gaps for the package maintainers instead.

See `.github/skills/shipment-reconcile/SKILL.md` (Cascade Close Sub-Procedure
step 7 and Quality Criteria) for the authoritative rule.
