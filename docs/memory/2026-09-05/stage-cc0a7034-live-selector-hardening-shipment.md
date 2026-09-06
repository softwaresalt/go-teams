---
title: "Stage session — CC0A7034 → shipment 010-S (live Teams selector/path hardening)"
date: 2026-09-05
agent: stage
status: complete
stash_processed: [CC0A7034]
stash_deferred: [D3EFD224, 87CDB97A, D2023898, EEA7F685, A99EEABD, 3084A83E]
feature: 010-F
tasks: [010.007-T, 010.001-T, 010.002-T, 010.003-T, 010.004-T, 010.005-T, 010.006-T, 010.008-T, 010.009-T]
shipment: 010-S
deliberation: 001-DL
plan: docs/plans/2026-09-05-teams-live-selector-path-hardening-plan.md
decision: docs/decisions/2026-09-05-teams-live-selector-path-hardening-deliberation.md
---

# Stage session: CC0A7034 → queued shipment 010-S

## Outcome

Staged the single operator-targeted stash entry **CC0A7034** (high, task-shaped,
no `DEFERRED SCOPE EXPANSION` marker) into a reviewed backlog hierarchy and a
**queued shipment `010-S`**, ready for Ship to claim. Stage did not modify
production/test source, run any build/harness, claim the shipment, or open a
product PR (P-010 boundary held).

## Pipeline trace

- **Step 0.0 tool gate:** backlogit CLI v1.10.1 (CLI-fallback mode; MCP tools not
  exposed as callable functions). Registry `features.sizing:true`,
  `features.shipments:true`.
- **Step 0.1 index sync:** OK (45 artifacts at start).
- **Step 1 triage:** CC0A7034 (target, high, task-shaped) + D3EFD224 (low, CI
  config — OUT OF SCOPE, deferred). No deferred-scope-expansion marker → normal route.
- **Step 1.5 grouping:** single-entry fallback (operator targeted CC0A7034) →
  synthesized covering feature.
- **Step 1.8 learnings:** `docs/compound/` has 5 entries (NOT empty — initial
  shallow check was wrong, corrected in deliberation + plan + 001-DL notes).
  shine-mp3-frame-alignment.md / shine-mp3-race-checkptr.md govern U5;
  amend-breaks-shipment-gate.md / copilot-rereview-timing.md govern U6.
- **Step 2 deliberation:** Option B (diagnostic-first per-surface decomposition).
  Decision doc + backlogit deliberation `001-DL` linked to CC0A7034.
- **Step 3 planning + hardening:** plan written; `Requires plan hardening: yes` →
  `## Plan Hardening` appended (strict-safety; live-auth run classified High).
- **Step 4 plan review (3 attempts):**
  - Attempt 1 — 6 personas, 4 ADVISORY + 2 FAIL → **FAIL**.
  - Attempt 2 (re-entry 1) — Constitution PASS, Go PASS, Security Lens ADVISORY,
    Architecture FAIL (2 P1s) → **FAIL**.
  - Attempt 3 (re-entry 2, final) — Architecture **ADVISORY** (both P1s + the P2
    resolved/folded), Security Lens **PASS** → **gate ADVISORY**, no P0/P1 open.
    Operator directive to autonomously produce the shipment = authorization to
    proceed past ADVISORY (recorded in the plan's `## Plan Review`).
- **Step 5 harvest:** feature `010-F` + 6 tasks (U1–U6), each with acceptance
  criteria, structured `size`, prose `complexity` (see degradation note), and
  dependency edges.
- **Step 5.5 shipment assembly:** `010-S` (queued), covering feature first, 6
  tasks parent-first in dependency order. Verified: 7 items, size histogram
  M:2/S:4, 0 unsized, 0 skipped.
- **Step 5.6 archive:** CC0A7034 text annotated with promotion target then
  archived. D3EFD224 left active (deferred).

## Backlog shape

Feature **010-F** — "Live Teams DOM selector and join-path hardening for browser
FSM/caption/audio verification"

| Task | Unit | Domain | Size | Complexity | Depends on |
|---|---|---|---|---|---|
| 010.007-T | U7 integration-harness browser exec-path enabler (test-only) | test-infra | XS | trivial | — (DAG root) |
| 010.001-T | U1 live-DOM diagnostic (build-tagged probe, Ship-executed) | test-infra/spike | M | medium | U7 |
| 010.002-T | U2 `/meet/` URL mutation hardening | code (test-first) | S | low | U1 |
| 010.003-T | U3 pre-join/lobby selectors → green TestMeetingJoin | code (char-first) | M | medium | U1, U2 |
| 010.004-T | U4 caption selectors → green TestMeetingEnableCaptions | code (char-first) | S | medium | U3 |
| 010.005-T | U5 audio path → green TestMeetingCaptureAudio | code (char-first) | M | medium | U3 |
| 010.006-T | U6 runbook + closure evidence (docs/closure only) | docs | S | low | U3, U4, U5 |
| 010.008-T | U8 browserContext docstring → confirmed guest (comment-only; split from U6) | docs/comment | XS | trivial | U1 |
| 010.009-T | U9 promote both root-cause learnings to docs/compound (split from U6) | docs | S | low | U6 |

DAG: U7 → U1 → U2 → U3 → (U4 ∥ U5) → U6 → U9; U1 → U8. Shipment `010-S` = **10 items**
(feature `010-F` + 9 tasks); task-size histogram **M:3 / S:4 / XS:2**, 0 unsized.
Estimated effort ≈ 9 tasks × ≤2h.

## Degradation note (capability gate)

This workspace's backlogit task type defines a structured `size` field but **not**
a structured `complexity` field (`backlogit update --complexity` → validation
error "artifact type task does not define a complexity field"). Per the Stage
structured-emission capability gate, both axes were still assigned: `size`
written structurally; `complexity` preserved as clearly-labeled prose in each
task's `implementation-notes` section. Ship should read complexity from the
task's implementation-notes, not from a structured field.

## Hard constraints threaded into every artifact

- Private join link stays ONLY in the out-of-tree session-state file; downstream
  references `TEAMS_MEETING_URL` only. Redaction rules bind U1/U2/U6 (selector
  inventory + redacted route class only; no HTML/HAR/screenshots/cookies/
  localStorage/profile paths).
- Do NOT terminate/alter the manually-launched Edge organizer (CDP 9223);
  teardown targets only the bot's own launched PID (no image-wide kill).
- Bot uses a separate git-ignored, workspace-contained profile; the live harness
  is pointed at the browser executable via `TEAMS_BROWSER_PATH` (primary setup),
  with prepending Chrome-for-Testing to PATH only a historical fallback for the
  first-run `google-chrome` abort.
- Parent evidence docs (circuit-break + blocked-closure) must not be overwritten.
- Bot-side login/MFA is an explicit, never-automated U1/U6 operator checkpoint.

## Handoff

- **Shipment ID (handoff token to Ship): `010-S`** (status: queued).
- Ship claims `010-S`, executes **U7 first** (`010.007-T` — the harness
  exec-path enabler and DAG root that makes the live harness launchable on the
  Edge-only host), **then U1** (`010.001-T` — live diagnostic feeds U2/U3), then
  the remaining DAG. U1 is a Ship-pipeline diagnostic, not a Stage spike; U7 is a
  Ship-executed test-infra enabler. Dependency edge: `010.001-T` → `010.007-T`.

## Addendum — post-gate live selector evidence folded in (2026-09-05)

After harvest + shipment assembly, the operator supplied concrete live DOM
evidence from a real authenticated meeting. Folded into the plan (new **Live
Selector Evidence (2026-09-05)** section as the selector oracle) and into the
queued backlog tasks in place. Treated as **de-risking sharpening, not scope
expansion** → ADVISORY gate verdict stands, no fourth review cycle (recorded in
the plan's Plan Review as a post-gate fold-in).

Key confirmed facts and where they landed:

- **Root cause confirmed:** `/meet/<id>?p=<passcode>` on a fresh guest profile
  redirects to `/dl/launcher/launcher.html`; `chromeMeeting.Join` waits for
  `[data-tid="prejoin-join-button"]` immediately after `Navigate` and never
  clicks `button[data-tid="joinOnWeb"]`, so it times out (`pre-join not visible`).
  → **U3 (010.003-T)** gained launcher handling as the primary fix.
- **Identity confirmed guest** (manual guest join + admission succeeded; no-invite
  two-session topology proven). Collapsed the guest-vs-authenticated branch across
  the plan/topology/U1/U6.
- **Selector deltas → U3 (010.003-T):** `meetingToolbarSel` `[data-tid="hangup-button"]`
  → `#hangup-button`; mic state read changed from `aria-checked` (no longer
  present) to the `checked` DOM property (bot was being left unmuted); already-valid
  (no churn): `prejoin-join-button`, `prejoin-display-name-input`, `toggle-mute`,
  lobby text.
- **Caption deltas → U4 (010.004-T):** `more-button`→`#callingButtons-showMoreBtn`;
  `language-speech-button`→`#LanguageSpeechMenuControl-id` (full client only);
  `closed-captions-button`→`#closed-captions-button` (bind to id; live data-tid
  varies); wrapper `[data-tid="closed-caption-renderer-wrapper"]` still valid;
  two-variant activation (guest-direct vs full-nested, no broad silent fallback);
  inner JS literals `closed-caption-text`/`author`/`.fui-ChatMessageCompact` still
  VERIFY LIVE. U4 file scope now includes `meeting_chrome.go` (activateCaptions).
- **U6 (010.006-T):** runbook records the confirmed redacted selector inventory +
  launcher note; docstring aligned to guest.
- **U1/U2 annotated:** U1 narrowed to the still-open URL-mutation question +
  caption-variant confirmation; U2 clarified as pure URL-string mutation (launcher
  handling is U3's, not U2's).

**Defect found and fixed during this addendum:** the create-time
`add --type task --section acceptance-criteria=...` **silently dropped** the
acceptance-criteria section for U1/U2/U5/U6 (only U3/U4, updated via
`update --section`, had it). Re-applied acceptance criteria to all four via
`backlogit update --section acceptance-criteria` — all six tasks now carry ≥1
acceptance criterion (P-003 restored). Lesson for future harvests on this
backlogit build: set `acceptance-criteria` with a follow-up `update --section`
call, not at `add` time, and verify the section persisted.

Sizes/DAG unchanged after the fold-in; shipment `010-S` re-verified: 7 items,
histogram M:2/S:4, 0 unsized, 0 skipped.

## Addendum 2 — U7 harness exec-path enabler + production follow-up (2026-09-05T17:06)

A fourth round of live evidence arrived. Sorted into three buckets:

**(a) Confirmations (no re-gate)** — a second live cycle reproduced the launcher
gap (`TestMeetingJoin` red again at ~123s `pre-join not visible`) and walked the
full happy path (`/meet/` → launcher → `joinOnWeb` → guest pre-join → join → lobby
→ admit), re-confirming `#hangup-button` admission and guest-direct
`#closed-captions-button`. Recorded as a "Confirmation run" note in the plan's
Live Selector Evidence section; status promoted from single-observation to
reproduced.

**(b) Audio sharpening (no re-gate)** — live evidence confirmed the capture
**prerequisites** (`getDisplayMedia` + `AudioContext`) are present; streaming
still unverified. U5 (`010.005-T`) plan text + acceptance criteria updated to
separate "prereqs present" (confirmed, NOT a pass) from verified end-to-end
streaming, where the pass REQUIRES a KNOWN live-stimulus correlation (a
deterministic tone / spoken phrase asserted in the captured signal, distinguishing
real meeting/tab audio from Chrome fake-media); non-zero MP3 bytes alone are
INSUFFICIENT. Within U5's existing boundary.

**(c) NEW SCOPE — U7 (independently scope-audited, then added)** — chromedp
auto-discovery aborts `exec: "google-chrome": executable file not found in %PATH%`
on the Edge-only Windows host; live workaround was an ad-hoc PATH prepend. Because
this ADDS a unit (not sharpening), it was routed through an independent **Scope
Boundary Auditor** pass → **ADVISORY (lean PASS)**. Actions taken, all auditor
advisories applied:
- Added **U7** to the plan (units, Dependency Graph sequenced ahead of U1, Risks,
  Decisions, and a dedicated post-gate `## Plan Review` note recording the
  ADVISORY scope-audit).
- Harvested task **`010.007-T`** (size XS, complexity trivial-as-prose, AC +
  notes present), dependency edge **`010.001-T` (U1) → `010.007-T` (U7)**, added
  to shipment **`010-S`**. Shipment now **8 items** (`010-F` + 7 tasks),
  histogram **M:2 / S:4 / XS:1**, 0 unsized, 0 skipped.
- U7 kept **test-only** (`internal/browser/meeting_integration_test.go` only, ~5
  lines, `TEAMS_BROWSER_PATH` → `chromedp.ExecPath`, pure superset). The
  **production-runtime** portability gap (`runner.go`/`meeting.go` share the same
  broken allocator call — confirmed by the auditor) was **split out as a separate
  follow-up stash entry `87CDB97A`** (kind: feature, medium) rather than bundled
  into 010-F.
- Secrets: `TEAMS_BROWSER_PATH` bound by Protected Invariant 5 (never
  echoed/committed into redacted `docs/` evidence).

**Repeat of the create-time section-drop quirk:** `--section
implementation-notes=...` AND `--section acceptance-criteria=...` are BOTH
silently dropped at `add` time — only a follow-up `update --section` persists
them. Confirmed and fixed for `010.007-T`; all 7 tasks re-verified to carry
size + acceptance-criteria + implementation-notes.

Gate verdict for 010-F stands **ADVISORY**. Shipment `010-S` remains **queued**
and unclaimed. Deferred/active stash: `D3EFD224` (CI config, prior) and new
`87CDB97A` — the explicit **residual-risk record (P-021 C3)** for the unresolved
production-runtime browser executable-path portability gap, deferred out of
010-F scope and not implemented in this shipment.

## Blocking conditions / open items

- None blocking harvest or shipment. Residual advisory (folded into U1): the
  `/meet/` param question is now handled by **bounded adaptive screening** (start
  with the smallest decisive raw-vs-mutated comparison, expand only along an
  implicated dimension, hard cap ≤8 launches / ≤3 live cycles) rather than a full
  five-param Cartesian ablation.
- Live execution risk (owned by Ship): the harness is currently RED
  (`pre-join not visible`); greening depends on live Teams DOM + operator
  checkpoints (lobby admission, possible bot-side login/MFA).
- Deferred: D3EFD224 (CI paths-ignore; latent, non-blocking while
  required_status_checks=[]).

## Post-gate review-fix cycle (2026-09-06) — PR #27 Copilot review: capture-first + task-contract corrections

Scope: Stage-owned planning/backlog/doc artifacts ONLY (no Go/source, no build,
no commit/push, no PR reply/thread-resolve, no shipment claim). Applied the
actionable PR #27 Copilot review items.

**A. P-021 capture-first — four new active DEFERRED SCOPE EXPANSION stash entries**
(each `requires deliberation: YES`; refs PR #27 + review threads
`PRRT_kwDOTStvas6foPdV` / `PRRT_kwDOTStvas6foPd-` + feature `010-F` + shipment
`010-S` + plan; task N/A — pre-existing production/repo surfaces):
- **R2 `D2023898`** (task, medium) — make the Chromium sandbox-disable flag
  opt-in, not unconditional (`internal/browser/allocator.go`).
- **R3 `EEA7F685`** (task, high — secret-adjacent) — redact query/passcode from
  browser navigation errors/logging (`runner.go`/`meeting.go`).
- **R4 `A99EEABD`** (feature, medium) — deliberate a Teams-host allowlist vs.
  intentional documented arbitrary-HTTP(S) support.
- **R5 `3084A83E`** (task, medium) — confirm local env files untracked + add a
  repository-approved secret-scanning guardrail (no secret values/paths recorded).

Plan `## Residual Review Items` R2–R5 dispositions and shipment `010-S` residual
section now cite the exact IDs. `87CDB97A` stays scoped **only** to R1
(production-runtime ExecPath portability), not widened. (`chore` is not a
supported stash kind in this workspace, so R2/R3/R5 were recorded as `task` with a
chore-class note; R4 as `feature`.)

**B. Same-surface task-contract corrections:**
- **U1 / `010.001-T`** — replaced the 84-cell Cartesian matrix with **bounded
  adaptive screening**: hard cap ≤8 launches / ≤3 live cycles; start with the
  single smallest decisive raw-vs-mutated comparison; expand ONLY along an
  implicated dimension; acceptance requires bounded outputs (launched cells +
  expand/stop decisions), NOT all 84 combinations. Env-gate mechanism preserved.
- **U4 / `010.004-T`** — the inner caption selectors (`closed-caption-text`/text,
  `author`/speaker, `.fui-ChatMessageCompact`/block) must be **confirmed live and
  MUST NOT remain VERIFY LIVE at completion**; `TestMeetingEnableCaptions` FAILS
  if the required speaker/text/block extraction is empty or stale.
- **U5 / `010.005-T`** — replaced the infeasible MP3 spectral assertion (the test
  receives **encoded MP3**; repo has **no decoder**) with a feasible U5-owned
  seam: a **PCM-before-encode observer tap** (nil-by-default test hook; single-
  frequency Goertzel/energy on raw PCM) or a **browser-side AnalyserNode**; still
  distinguishes real tab audio from Chrome fake-media and rejects non-zero MP3
  bytes alone. File scope <3 (`audio.go` + `meeting_integration_test.go`,
  `+allocator.go` conditional); split the analyser variant to a follow-up if it
  exceeds 2h.
- **U2 / `010.002-T`** — narrowed the secrecy criterion: only the **live
  `TEAMS_MEETING_URL` value / `p=` query token** and **raw DOM/storage/profile
  payloads** are forbidden; the env-var **identifier** `TEAMS_MEETING_URL` and
  **redacted selector strings** are allowed.

**C. Sizes / histogram / trace:** no task size changed — U1 stays **M** (bounded
screening now fits the 2h budget), U2/U4/U5 stay **S**, U7 **XS**. Shipment
`010-S` unchanged: **8 items**, histogram **M:2 / S:4 / XS:1**, DAG
`U7→U1→U2→U3→(U4∥U5)→U6`, status **queued/unclaimed**. Requirements Trace rows
unchanged (within-unit refinements only). Secret-scan of all touched artifacts:
CLEAN (no live URL / `p=` token / account / calendar / cookie / profile path).

## Post-gate review-fix cycle 2 (2026-09-06) — PR #27 current-HEAD Copilot review: U5 scope, U6 split, U2 policy

Scope: Stage-owned planning/backlog/doc artifacts ONLY (no Go/source, no build,
no commit/push, no PR reply/thread-resolve, no shipment claim). Applied the
current-HEAD PR #27 Copilot review items.

- **U5 (`010.005-T`) file scope + size S -> M.** Added
  `internal/browser/meeting_chrome.go` to U5 file scope: decoded PCM `samples`
  reach `enc.WritePCM` there (encoder built at meeting_chrome.go:305, WritePCM at
  :315), so the PCM-before-encode observer tap is wired at that point. Because
  `meeting_integration_test.go` is `package browser_test` (external), the seam MUST
  be an EXPORTED / accessible nil-by-default hook (or the AnalyserNode decision is
  transported over the `sendAudioToGo` binding). 3 core files (`meeting_chrome.go`
  + `audio.go` + external `browser_test`) exceed the S `<3 files` guidance -> size
  raised to **M** via `backlogit update --size M`.
- **U6 (`010.006-T`) width split -> U8 + U9.** U6 exceeded width. It now owns ONLY
  the runbook + closure evidence under `docs/closure/browser-automation/`. Created:
  - **U8 `010.008-T`** (XS, trivial; dep U1) — comment-only `browserContext`
    docstring aligned to the confirmed guest topology in
    `internal/browser/meeting_integration_test.go`.
  - **U9 `010.009-T`** (S, low; dep U6) — promote BOTH distinct root-cause
    learnings (agent-browser managed-launch DevToolsActivePort failure; chromedp
    harness `google-chrome` PATH-discovery failure + `TEAMS_BROWSER_PATH` remedy)
    into `docs/compound/` as SEPARATE learnings (never conflated), cross-referencing
    (not overwriting) the parent blocked-closure + circuit-break records.
  Both added to shipment `010-S` in dependency order (appended after U6; U8's dep
  U1 and U9's dep U6 both precede). New DAG: `U7 → U1 → U2 → U3 → (U4 ∥ U5) → U6 →
  U9`, with `U1 → U8` branch.
- **U2 (`010.002-T`) deterministic conflict policy.** Replaced the
  "do-not-clobber pre-existing `type`" ambiguity with ONE deterministic rule for
  `/meet/` inputs: STRIP the `type` query key ENTIRELY (including an incoming
  `type=meetup-join`; stripped key not preserved), while PRESERVING `u.Path`, the
  `p` token, all unrelated query keys, and the URL fragment. Legacy
  `/l/meetup-join/` stays byte-for-byte compatible. Acceptance assertions/fixtures
  aligned: conflict case asserts `type` ABSENT; new case asserts unrelated keys +
  fragment preserved.
- **Shipment `010-S` wording.** "Live verification" now describes the organizer as
  a **manually launched Microsoft Edge connected through native CDP** and keeps the
  **managed agent-browser Chrome launch identified as the BLOCKED path**.

**Sizes / histogram / trace.** U5 S -> M; U8 XS; U9 S. New shipment size histogram
**M:3 / S:4 / XS:2** (9 tasks), **10 items** total (feature + 9 tasks), status
**queued / unclaimed**. Requirements Trace gained U8 + U9 rows; DAG, Constitution
Check width-isolation note, and Runtime Verification updated. PR body (current HEAD
`d5a6d77`) NOT altered. Secret-scan of all touched artifacts: CLEAN (no live URL /
`p=` token / account / calendar / cookie / profile path).

## Final staging review remediation and safe-stop handoff

The final current-HEAD Copilot pass found six consistency issues. Stage artifacts
were corrected without source changes or shipment claim:

* U2 always implements the deterministic `/meet/` `type`-strip contract; U1
  controls only the evidence-based keep/drop choice for other legacy parameters
* U4 owns the integration-test assertion that rejects empty or stale caption
  extraction; its scope is three coupled files and size M
* U6 closure must cite all five deferred residual entries: `87CDB97A`,
  `D2023898`, `EEA7F685`, `A99EEABD`, and `3084A83E`
* U7 owns fail-closed `TEAMS_PROFILE_DIR` workspace-containment and git-ignore
  validation in the integration-test helper; size S
* The canonical topology correctly assigns the `browserContext` docstring to U8
* The PR body already reflects nine tasks and ten shipment items

Final shipment size histogram: **M:4 / S:4 / XS:1**. The DAG is unchanged:
`U7 -> U1 -> U2 -> U3 -> (U4 || U5) -> U6 -> U9`, with `U1 -> U8`.
Shipment `010-S` remains queued and unclaimed. The requested safe stop is after
staging PR #27 merges and local `main` is clean; Ship implementation must not
start in this session.
