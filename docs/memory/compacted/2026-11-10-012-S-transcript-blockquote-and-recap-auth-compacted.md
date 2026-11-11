---
type: memory-compacted
date: 2026-11-10
shipment: 012-S
feature: 012-F
source: docs/archive/memory/2026-11-10/transcript-blockquote-and-recap-auth-memory.md
---

# Compacted memory - Shipment 012-S (transcript blockquote and recap auth hardening)

**Shipped**: PR #30, merge `aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb`. Closed
and archived via P-015 verified fully-covered-root cascade
(`backlogit shipment ship`). Also completed post-merge reconciliation of the
previously merged `011-S` (`d50bab42c8dc6339c8b791f0f17647dce991df82`).

## Decision and outcome

Formatted Markdown transcript utterances with a bold speaker header plus
blockquote line (`**Speaker** (HH:MM:SS):\n> text`). Hardened cold-profile
headed authentication: bootstrap on same-origin `/v2/` first when no Teams
Local Storage tokens are present, never interrupt multi-host login/MFA or
OAuth `#code=` exchanges, and rebind the single capture tab safely across
COOP `targetId` swaps. Ignored opaque SharePoint `/:v:/p/` sharing tokens in
`fileUrl` so `RecapSharingLink_RecapChiclet` URLs no longer reject the real
`.mp4` recording filename during Meeting Recap identity verification.

## Key reusable insight

A warm profile already on the target recap URL must keep the full on-target
stall threshold (20 polls / 30s); only the off-target and
`launchagent=join_launcher_web` landing paths warrant the shorter post-auth
threshold. Conflating the two by keying off `reNavigateAttempts == 0` alone
(ignoring `onTarget`) causes a premature reload while a warm, already-loading
recap page is still rendering.

## Review outcome

Local review: two passes, `READY` with `P0=0, P1=0, P2=0, P3=0` after
remediating one P2 (on-target stall threshold regression). Copilot shadow
review on PR #30 raised four in-scope findings (P-021 C1 same contract
surface), all fixed with regression tests: omitted `navURL` in
bounce-recovery logging, missing multi-candidate `otherPageIDs` tracking on
missing-target polls, `hasRecapIframe` not restricted to `iframe` targets on
known recap hostnames, and unescaped leading list/thematic markers in
blockquote captions (`sanitizeBlockquoteText`).

## Post-merge closure summary

* Merge confirmed (`gh pr view 30`, `git merge-base --is-ancestor`).
* `012-S` cascade-closed: `archived_ids` matched the expected set exactly
  (`012.001-T, 012.002-T, 012.003-T, 012-F, 012-S`), `returned_ids` empty,
  every task `parent_id` preserved. The unrelated, separately queued
  `010-S` and its nine tasks were verified untouched throughout.
* `docs/closure/2026-11-10-012-S-review-and-closure.md` and
  `docs/closure/2026-11-09-011-S-review-and-closure.md` updated with
  `closure_status: READY` / `compaction_status: done` so both pass the
  `pipeline-topology` closure-completeness predicate.
* `backlogit sync` and `backlogit doctor` ran clean post-closure.

## Follow-ups carried forward

None identified at closure time.

Verbose original archived at
`docs/archive/memory/2026-11-10/transcript-blockquote-and-recap-auth-memory.md`.
