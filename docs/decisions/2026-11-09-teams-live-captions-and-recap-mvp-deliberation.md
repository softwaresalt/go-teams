---
title: Teams live captions and recap MVP decision
date: 2026-11-09
status: accepted
feature: 011-F
shipment: 011-S
---

## Problem frame

We need a Windows executable that accepts a shared Teams URL and an explicit
Markdown destination without requiring a particular caller working directory.
The same capture surface must handle live captions and accessible Meeting
Recap transcripts, with an available Teams AI summary appended.

The operator explicitly authorized all planning, backlog, documentation, code,
verification, and PR preparation on `feat/mvp-live-captions`. Existing commits
are recorded retrospectively; this decision does not invent prior test evidence.

## Requirements and success criteria

* R1: Root, `captions`, and `join` accept URL and output positional arguments
* R2: `-u/--url`, `-o/--output`, `--output-file`, and `--output-dir` support automation
* R3: `--headless` reuses an authenticated profile without interactive prompts
* R4: Live captions are settled, deduplicated, and flushed to Markdown
* R5: Meeting Recap entries preserve recording offsets; available AI summaries follow `---`
* R6: Late outer-page rendering does not prevent bounded summary discovery
* R7: Backlog, design, plan, review, closure, and continuity artifacts accompany the PR
* R8: The compiled executable is verified against synthetic content, then pushed with a current-HEAD local readiness record

## Options

| Option | Benefit | Limitation |
|---|---|---|
| Teams APIs | Structured service contracts | Requires a separate permission and tenant-consent design |
| Browser DOM and CDP | Uses content available to the signed-in user | Selectors and private React properties can change |
| Manual export | Minimal automation | Does not satisfy unattended CLI capture |

## Chosen direction

We retain the existing Go, Cobra, and chromedp architecture. Browser capture
uses a clean CDP launch rather than sandbox-disabling or automation-masking
flags. Recap extraction combines a virtualized-list data path with a rendered
DOM fallback. AI summaries use the page's Copy all behavior, not a new AI
service or generated summary.

The CLI classifies `.md` and `.markdown` output targets as files unless they
already identify directories. `--output-dir` always means a directory.
Explicit destinations replace previous content; generated filenames reject
collisions. Output is created only after meeting admission or recap discovery.

Summary discovery refreshes CDP targets on each of at most 20 attempts, with
500 ms between attempts and cancellation-aware waiting. A missing summary
does not invalidate a successfully extracted transcript.

## Boundaries and risks

* No tenant login, MFA, permission bypass, or guarantee of recap availability
* No real meeting links, tenant identifiers, drive identifiers, or participant identities in fixtures or artifacts
* No audio-fidelity qualification in the synthetic caption/recap checks
* No migration, merge, or closure of the separate `010-S` shipment
* No competing branches or worktrees for this release unit
* No simultaneous browser processes sharing one authenticated profile

## Open questions

No implementation-blocking question remains. Real-tenant authentication,
selector compatibility, virtualized transcript completeness, and summary
availability require an authorized operator-assisted verification window.

## Related artifacts

* [Capture design](../design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md)
* [Implementation plan](../plans/2026-11-09-teams-live-captions-and-recap-mvp-plan.md)
* [Review and closure](../closure/2026-11-09-011-S-review-and-closure.md)
