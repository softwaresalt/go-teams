---
title: "Live Teams DOM Selector & Join-Path Hardening — Implementation Plan"
description: "Diagnostic-first plan to live-verify and harden the VERIFY-LIVE browser selectors and the /meet join path so TestMeetingJoin/EnableCaptions/CaptureAudio pass green against a real authenticated Teams meeting."
source: "docs/decisions/2026-09-05-teams-live-selector-path-hardening-deliberation.md"
source_stash: "CC0A7034"
deliberation_id: "001-DL"
date: "2026-09-05"
tags:
  - "browser-verification"
  - "teams"
  - "selector-hardening"
  - "runtime-verification"
---

## Problem Frame

Make the `VERIFY LIVE` browser automation in `internal/browser` actually work
against current Microsoft Teams DOM, using the live authenticated meeting the
parent Orchestrator session established. The integration harness
(`internal/browser/meeting_integration_test.go`, build tag `integration`) drives
three flows through `browser.Meeting`:

* `TestMeetingJoin` → `chromeMeeting.Join` (`meeting_chrome.go`) waits on
  `joinNowSel` (`prejoin.go`). **Observed live failure:** `browser: pre-join not
  visible: context deadline exceeded` after ~123s.
* `TestMeetingEnableCaptions` → `chromeMeeting.EnableCaptions` activates captions
  via `moreActionsSel → languageSpeechSel → turnOnCaptionsSel` and observes
  `captionWrapperSel` (`captions.go`).
* `TestMeetingCaptureAudio` → `chromeMeeting.CaptureAudio` runs `audioCaptureJS`
  (`audio.go`) which depends on the `--auto-select-desktop-capture-source`
  allocator flag (`allocator.go`).

The join URL format shifted: current Teams links are
`https://teams.microsoft.com/meet/<id>?p=<token>`, while `internal/teams/url.go`
`MutateJoinURL` unconditionally appends legacy `type=meetup-join` bypass params.
`ValidateMeetingURL` already accepts the new format (scheme/host only). The
harness applies `MutateJoinURL` to `TEAMS_MEETING_URL` before navigation.

**Root cause — CONFIRMED (2026-09-05 live evidence).** The failure is the
`/dl/launcher/launcher.html` launcher interstitial: on a fresh guest profile the
`/meet/<id>?p=<token>` link redirects to the launcher, whose first control is
`button[data-tid="joinOnWeb"]`, but `chromeMeeting.Join` waits on `joinNowSel`
immediately after `Navigate` and never clicks `joinOnWeb`, so it times out on the
launcher before pre-join renders (the observed `pre-join not visible: context
deadline exceeded`). Identity is confirmed **guest**. The launcher fix (U3) is no
longer hypothetical. What remains genuinely open is narrower and diagnostic-only:
whether `MutateJoinURL`'s legacy params change the `/meet/` landing versus the raw
URL (the U2 input) and which caption-activation variant (guest-direct vs.
full-nested) the bot lands in per run — these are U1's residual scope.

**Hard constraints threaded into every unit:**

* The operator-provided private join link is out-of-tree; work references it only
  via `TEAMS_MEETING_URL`. No artifact/commit may embed the URL.
* The manually launched Edge organizer (CDP port 9223) and its profile MUST NOT
  be terminated or altered. The bot uses the separate empty `TEAMS_PROFILE_DIR`.
* Parent-owned evidence docs (`docs/memory/2026-09-05/circuit-break-...md`,
  `docs/closure/browser-automation/2026-09-05-...md`) are not edited by this work
  except to add net-new closure evidence in unit 6.

## Requirements Trace

| Source requirement (CC0A7034 / deliberation) | Implementation unit(s) |
|---|---|
| Make the live harness launchable on Edge-only / non-PATH hosts (test-infra enabler; 2026-09-05 live evidence) | U7 (DAG root; U1 dep) |
| Launcher root cause CONFIRMED (fresh-guest `/meet/` link redirects to `/dl/launcher/launcher.html`; a `joinOnWeb` click is required before pre-join). Assess the remaining open questions — `/meet/` URL-mutation behavior and caption-activation variants | U1 (assess); U3 implements the confirmed launcher fix |
| Handle the current `/meet/<id>?p=<token>` join-path format correctly | U2 |
| Make the pre-join/lobby FSM pass live → green `TestMeetingJoin` | U3 (dep U1, U2) |
| Make live captions activate/stream → green `TestMeetingEnableCaptions` | U4 (dep U3) |
| Make tab audio capture/encode → green `TestMeetingCaptureAudio` | U5 (dep U3) |
| Resolve the `VERIFY LIVE` markers in prejoin/captions/audio | U3, U4, U5 |
| Reproducible live-harness invocation + closure evidence | U6 (dep U3, U4, U5) |

## Canonical Verification Topology (resolves identity/topology ambiguity)

One and only one verification topology is used, matching deployment:

* The **bot** runs in its **own isolated browser** launched by the harness
  (`chromedp.NewExecAllocator`) using the separate empty `TEAMS_PROFILE_DIR`.
  `TEAMS_PROFILE_DIR` MUST resolve to a git-ignored path inside the workspace
  tree (a precheck asserts this); it is treated as secret material.
* The bot's identity mode is **confirmed guest pre-join** by the 2026-09-05 live
  evidence: on a fresh guest profile the `/meet/<id>?p=<passcode>` link redirects
  to `/dl/launcher/launcher.html`, whose "Join meeting from this browser" control
  is activated to reach the **guest lightweight pre-join** surface, and a manual
  guest join was **admitted successfully** (proving the no-invite two-session
  topology). This confirmed guest mode fixes the *expected pre-join surface* that
  U2's exit gate, U3's oracle, and the U6 runbook all reference — guest and
  full-client expectations must not be silently mixed. If a future run instead
  lands on Microsoft sign-in/MFA (authenticated bot identity), that is an
  explicit operator checkpoint (never automated) and the affected selectors
  switch to their full-client variants recorded in the Live Selector Evidence
  table; the `meeting_integration_test.go` `browserContext` docstring
  ("already authenticated") is stale for the guest topology and is aligned in U6
  scope.
* The **Edge organizer** session (CDP port 9223) is a **separate trust
  boundary**, left running and unaltered. Attaching the bot to CDP 9223 (or any
  RemoteAllocator attach) is explicitly a **future-only** option and is NOT part
  of this scope — attaching would validate the wrong role (organizer, not bot).

## Live Selector Evidence (2026-09-05) — authoritative selector oracle

Concrete DOM findings captured from a live authenticated Teams meeting (private
join URL/token deliberately omitted). This table is the selector oracle U3/U4/U6
implement against; U1's remaining diagnostic value narrows to the URL-mutation
question (U2 input) and confirming the caption-path variant per run.

**Join-path / launcher (root cause of the current red harness):**

* Current join URL shape: `https://teams.microsoft.com/meet/<numeric-id>?p=<passcode>`.
* On a **fresh guest profile** it redirects to `/dl/launcher/launcher.html`; the
  required first control is `button[data-tid="joinOnWeb"]`
  (aria-label "Join meeting from this browser").
* **Defect:** `chromeMeeting.Join` (meeting_chrome.go) calls
  `chromedp.WaitVisible(joinNowSel)` immediately after `Navigate`, so it never
  clicks `joinOnWeb` and times out on the launcher before pre-join renders — this
  is exactly the observed `pre-join not visible: context deadline exceeded`.
  **Fix (U3):** handle the launcher (detect + click `joinOnWeb`) before waiting
  for the pre-join button.

**Guest lightweight pre-join surface (after `joinOnWeb`):**

| Concern | Current code | Live 2026-09-05 | Status | Owner |
|---|---|---|---|---|
| Launcher "join on web" | (none) | `button[data-tid="joinOnWeb"]` | **MISSING — add** | U3 |
| Display-name input | `input[data-tid="prejoin-display-name-input"]` | same | ✅ valid (refresh provenance) | U3 |
| Join-now button | `[data-tid="prejoin-join-button"]` | same | ✅ valid | U3 |
| Mic toggle *selector* | `[data-tid="toggle-mute"]` | `input[type=checkbox][role=switch][data-tid="toggle-mute"][data-cid="toggle-mute-true"]` | ✅ selector valid | U3 |
| Mic toggle *state read* | `muteRequired(ariaChecked)` reads `aria-checked` | switch exposes **no `aria-checked`**; DOM property `checked=true` when mic is ON | **STALE — read the `checked` DOM property ONLY (no `aria-checked`; do NOT use `data-cid`/title as the state source)** | U3 |
| Lobby text | `"Someone will let you in shortly"` | same (worked live) | ✅ valid | U3 |
| Admission/hangup | `meetingToolbarSel = [data-tid="hangup-button"]` | guest hangup is `#hangup-button` (**no data-tid**); organizer full client also has `#hangup-button` (its data-tid is `hangup-main-btn`) | **STALE — use `#hangup-button`** | U3 |

**Caption activation — two variants (must support both, no broad silent fallback):**

| Step | Current code | Guest lightweight | Full/organizer client | Owner |
|---|---|---|---|---|
| More menu | `moreActionsSel = [data-tid="more-button"]` | `#callingButtons-showMoreBtn` (aria "More", no data-tid) | `#callingButtons-showMoreBtn` | U4 |
| Language & speech submenu | `languageSpeechSel = [data-tid="language-speech-button"]` | **absent** (captions shown directly) | `#LanguageSpeechMenuControl-id` | U4 |
| Captions menuitem | `turnOnCaptionsSel = [data-tid="closed-captions-button"]` | `#closed-captions-button` (role menuitem, aria "Captions", title "Show live captions") | `#closed-captions-button` | U4 |
| Caption renderer wrapper | `[data-tid="closed-caption-renderer-wrapper"]` | same | same | U4 |

* Stable caption button id is `#closed-captions-button`; the live **data-tid
  varies** (`closed-captions-button-on` in full client, absent in guest) — bind
  to the id, not the data-tid.
* Activation flow: click More (`#callingButtons-showMoreBtn`); **if**
  `#LanguageSpeechMenuControl-id` is present (full client) click it, **else**
  (guest) the captions menuitem is shown directly; then click
  `#closed-captions-button`; then wait for
  `[data-tid="closed-caption-renderer-wrapper"]`. Use a bounded per-step
  `WaitVisible`, not an unbounded blocking `Click` and not a broad silent
  fallback.
* **Still VERIFY LIVE (not covered by this evidence):** the caption-text and
  author inner selectors inside `captionObserverJS` (`closed-caption-text`,
  `author`, `.fui-ChatMessageCompact`) — U4 must still confirm these against a
  live finalized caption.

**Confirmation run (2026-09-05T17:06) — findings reproduced, not merely observed
once:** a second live cycle re-confirmed the table above end-to-end. The existing
`TestMeetingJoin` was rerun (Chrome-for-Testing prepended to PATH so it launched)
and failed again after ~123s at `pre-join not visible: context deadline exceeded`
— a clean reproduction that isolates the red harness to the **launcher gap** (U3),
not browser startup. A manual live continuation then walked the full happy path —
`/meet/<id>` → `/dl/launcher/launcher.html` → `joinOnWeb` → guest pre-join →
existing display-name/join selectors → lobby text → organizer admit → active
meeting — and re-confirmed admission is detectable via `#hangup-button` (no
matching `data-tid="hangup-button"`) and that guest captions open **directly** via
`#closed-captions-button` (no Language & speech submenu) with
`[data-tid="closed-caption-renderer-wrapper"]` present/valid after activation. No
selector value changed; this promotes the table's status from single-observation
to reproduced. **New harness-runnability finding (drives U7):** on this Edge-only
Windows host, chromedp browser auto-discovery aborts with
`exec: "google-chrome": executable file not found in %PATH%`; the live workaround
was prepending the Chrome-for-Testing directory to PATH — U7 replaces that ad-hoc
trick with an explicit, version-controlled harness executable-path option.

## Implementation Units

### U1 — Live pre-join/lobby DOM diagnostic (Ship-pipeline diagnostic task)

* **Domain:** test infrastructure / investigation (a build-tagged diagnostic
  probe plus a redacted findings note; single domain — no production source).
  **Executor:** Ship (this is a Ship-pipeline diagnostic task, NOT a Stage spike
  worktree — it runs the live harness, which Stage must not do under P-010).
* **Posture:** characterization-first.
* **Scope narrowed by the 2026-09-05 live evidence (see Live Selector Evidence
  table):** the pre-join root cause is already established — the harness times out
  on the `/dl/launcher/launcher.html` interstitial because `Join` never clicks
  `joinOnWeb` — and the identity is confirmed **guest**, with a mostly-valid
  pre-join selector inventory captured. Those confirmed fixes flow directly to U3
  (launcher + selectors) and U4 (caption variants) without waiting on a fresh
  diagnostic. U1's **remaining, still-open** diagnostic value is therefore
  narrower: (a) determine whether `MutateJoinURL`'s legacy params change the
  `/meet/` launcher landing versus the raw URL (the U2 input — the manual proof
  used direct navigation, not the mutated URL the harness sends), and (b) confirm
  per-run which caption-activation variant (guest-direct vs. full-nested) the bot
  lands in. If Ship judges the launcher root cause and guest identity already
  conclusive from the recorded evidence, U1 may reduce to the URL-mutation matrix
  plus a confirmation pass rather than the full landing-surface hunt (record the
  reduction).
* **Executable diagnostic seam (required — the existing harness cannot do this):**
  the current `meeting_integration_test.go` always applies `MutateJoinURL`,
  hardcodes `Headless: true`, and passes an empty UA that resolves to
  `DefaultUserAgent`, so it cannot compare configurations. U1 adds a
  **build-tagged (`integration`) diagnostic probe** (e.g.
  `internal/browser/meeting_diagnostic_test.go`) with explicit controls:
  * URL mode — **raw** vs. **`MutateJoinURL`-mutated**;
  * render mode — **headed** vs. **`--headless=new`**;
  * UA policy — **tri-state**: fixed Chrome/125, a newer fixed UA, and a genuine
    **native/omitted** override (not silently defaulted to `DefaultUserAgent`);
  * per-`/meet/` **candidate bypass-param experiments**: a baseline of
    **none added**, then a **single-parameter ablation** that toggles each of the
    five legacy params individually (`msLaunch=false`, `type=meetup-join`,
    `directDl=true`, `enableMobilePage=true`, `suppressPrompt=true`), plus the
    **full legacy set**, so U2's kept-vs-dropped decision is grounded
    per-parameter (which individual param helps/hurts `/meet/`) rather than as one
    all-or-nothing comparison. If run-budget forces trimming, prioritize the
    singleton ablations of the params most likely implicated by U1's landing-surface
    evidence and record which cells were skipped.
  Each matrix cell runs **serially** with a **clean/isolated profile dir** so
  cells cannot contaminate one another; profiles remain secret material.
* **Dedicated opt-in (required — must NOT launch on routine integration runs):**
  the diagnostic matrix is expensive (raw vs. mutated URL × headed vs. headless ×
  tri-state UA × five-param ablation), so it MUST sit behind its **own** opt-in
  *beyond* the `integration` tag — a second build tag (e.g.
  `//go:build integration && diag`) or a dedicated env gate (e.g.
  `GO_TEAMS_DIAGNOSTIC_MATRIX=1`). A routine `go test -tags integration` run
  compiles the probe but does **not** execute the matrix; the matrix runs only
  when the dedicated opt-in is set. Record which mechanism was chosen in the
  findings note.
* **What:** For each cell, record which landing surface renders (guest pre-join
  SPA / Microsoft sign-in / MFA / lobby / "unsupported browser" interstitial) and
  the current values of the pre-join, mute, display-name, admitted-toolbar, and
  lobby selectors. Explicitly answer: (a) is the failure a stale selector, wrong
  landing surface, headless variance, UA interstitial, or bot-side auth?
  **(2026-09-05 evidence answers this: the launcher interstitial is the cause.)**
  (b) is the URL mutation implicated, and which specific params help/hurt
  `/meet/`? **(still open — primary remaining U1 question, U2 input.)** (c) does
  the bot land on guest pre-join, sign-in, or lobby? **(2026-09-05 evidence:
  guest pre-join after `joinOnWeb`; re-confirm per run and flag any sign-in/MFA
  drift as a checkpoint.)**
* **Bot-side auth checkpoint:** if the bot lands on Microsoft login/MFA, record
  `BLOCKED` and raise a headed operator checkpoint — NEVER automate credential
  entry. If the bot waits in the lobby, request organizer admission as a human
  checkpoint.
* **Redaction (NON-NEGOTIABLE):** the diagnostic evidence is a **selector
  inventory + a redacted landing-surface class only** (e.g. `/meet/{id}` web
  pre-join SPA, with the `p` token and full query string stripped). Do NOT
  commit `outerHTML`/HTML dumps, HAR, screenshots of the address bar, CDP
  storage/cookie dumps, or profile paths. Scrub `p=`, full join URLs, cookies,
  `localStorage`, and profile paths from any `t.Log`/chromedp error output before
  it is written anywhere under `docs/`. The build-tagged probe is a diagnostic
  scaffold, not a committed selector oracle — it must not embed live URLs/tokens.
* **Files:** a net-new `integration`-tagged diagnostic test
  (`internal/browser/meeting_diagnostic_test.go`) and a net-new **redacted**
  findings note under `docs/memory/2026-09-05/` (do not overwrite parent
  evidence). No production source edits in U1.
* **Verify (concrete exit artifacts consumed downstream):** a dated note
  containing (1) the live selector inventory, (2) an explicit root-cause verdict,
  (3) a URL-implicated yes/no **plus the per-param kept/dropped decision for
  `/meet/`**, (4) the bot landing-surface class **and the selected identity
  mode** (guest vs. authenticated), (5) the winning config (headed/headless, UA)
  — all redacted. Items (3)/(4) are the explicit inputs U2 and U3 consume.
* **Depends on:** U7 (the live harness must be launchable on the target host —
  no `google-chrome not found` abort — before any live diagnostic run).

### U2 — Join-path URL mutation hardening for the `/meet/` flow

* **Domain:** code (Go). **Posture:** test-first.
* **Gated on U1:** only edit `MutateJoinURL` if U1's verdict marks the URL
  mutation as implicated. If U1 shows the mutated `/meet/` URL reaches the same
  `/dl/launcher/launcher.html` landing as the raw URL (i.e. mutation is not the
  blocker — the launcher handling in U3 is), collapse U2 to a recorded no-op
  (documented) rather than a speculative change.
* **Scope boundary vs. U3:** U2 is **pure URL-string mutation only**. The
  `/meet/<id>?p=<passcode>` → `/dl/launcher/launcher.html` redirect and the
  `joinOnWeb` click are **runtime navigation** handled by U3, not by U2. U2 never
  touches the launcher; it only decides which query params `MutateJoinURL`
  emits for a `/meet/` path.
* **What (when implicated):** In `internal/teams/url.go`, branch on the URL path
  family and suppress the **legacy** bypass params for `/meet/` links. Precise
  mechanism (the real defect is that the fixed `bypassParams` loop
  *unconditionally forces* `type=meetup-join` and four peers — `p` is already
  preserved because it is not in the loop):
  * `/l/meetup-join/...` → keep the current behavior **byte-identical** (still
    set the legacy param set) so all three existing tests stay green.
  * `/meet/...` → do NOT set `type=meetup-join`; apply only the params that U1's
    **per-param experiments** showed help the `/meet/` web pre-join SPA (state the
    exact kept/dropped set from U1). If a `/meet/` input **already carries** a
    legacy param (e.g. `type=meetup-join`), do not clobber or depend on it —
    treat the input as authoritative and document the conflict handling. Preserve
    `u.Path` and the `p` token unchanged.
  * Any other `ValidateMeetingURL`-accepted path → documented fallback
    (default to the conservative no-legacy-force behavior).
  * Do NOT mutate the shared package-level `bypassParams` map; select the param
    set via branch logic or a separate immutable const set (avoids shared-state
    corruption / data race).
* **Files:** `internal/teams/url.go`, `internal/teams/url_test.go` (<3 files).
* **Test discipline (secrets):** use **synthetic fixtures only** —
  `https://teams.microsoft.com/meet/example-id?p=synthetic-token`. NEVER copy
  `TEAMS_MEETING_URL` or any U1 DOM string into tests, comments, or commit
  messages.
* **Concrete assertions:** for a `/meet/<id>?p=<token>` input, parse the result
  with `url.Parse` and assert `u.Path` unchanged, `u.Query().Get("p")` equals the
  exact (decoded) token, and `u.Query().Get("type") != "meetup-join"`; add a
  `/meet/<id>` (no `p`) case; add a `/meet/<id>?type=meetup-join` **conflict**
  case asserting the pre-existing param is not clobbered; and assert the legacy
  `/l/meetup-join` branch output is unchanged via a **golden/equality** check
  (full mutated-string equality against the pre-change output), making the
  "byte-identical" claim verifiable rather than only param-presence.
* **Verify:** `go test ./internal/teams/...` green; full gates
  (`go vet ./...`, `golangci-lint run`, `gofmt -l .`) clean.
* **Exit gate (feeds U3):** after U2, a live re-navigation confirms the mutated
  `/meet/` URL lands the **expected pre-join surface for the U1-selected identity
  mode** (guest pre-join SPA, or the authenticated pre-join surface if U1 chose
  authenticated); that post-U2 landing DOM (redacted) is U3's selector oracle.
* **Depends on:** U1 (root-cause verdict, per-param decision, selected identity).

### U3 — Pre-join/lobby FSM: launcher handling + selector/mute-state fixes → green `TestMeetingJoin`

* **Domain:** code (Go). **Posture:** characterization-first for the live-DOM
  selectors + launcher step; **test-first** for the pure `muteRequired` predicate
  rewrite. Both are the browser package (single domain). Selector oracle: the
  Live Selector Evidence (2026-09-05) table.
* **What — three concrete changes grounded in live evidence:**
  1. **Launcher-interstitial handling (root-cause fix).** In `chromeMeeting.Join`
     (`internal/browser/meeting_chrome.go`), between `chromedp.Navigate` and
     `chromedp.WaitVisible(joinNowSel)`, add a step that detects the
     `/dl/launcher/launcher.html` interstitial and clicks the "join on web"
     control before waiting for pre-join. Add a new selector const
     `joinOnWebSel` (Go raw-string value `` `button[data-tid="joinOnWeb"]` ``) in
     `prejoin.go`. Probe
     for it first (bounded `WaitVisible`/`existsJS`, matching the `enforceMute`
     idiom) so a run that lands directly on pre-join (no launcher) still proceeds.
     This resolves the observed `pre-join not visible: context deadline exceeded`.
  2. **Admission/hangup selector.** Change `meetingToolbarSel` from
     `[data-tid="hangup-button"]` to `#hangup-button` (stable across guest
     lightweight and organizer full client; guest has no `data-tid` on hangup).
     `awaitAdmission` already uses `existsJS(meetingToolbarSel)`, so only the const
     changes.
  3. **Mute-state read (currently leaves the bot unmuted).** The mic switch no
     longer exposes `aria-checked`; it is
     `input[type=checkbox][role=switch][data-tid="toggle-mute"]` with DOM property
     `checked=true` when the mic is ON. `enforceMute` currently reads
     `aria-checked` via `chromedp.AttributeValue` and `muteRequired(ariaChecked
     string)` compares to `"true"`, so it never mutes. Change the state source to
     read **only** the boolean `checked` property (e.g. `chromedp.Evaluate` of
     `document.querySelector('[data-tid="toggle-mute"]').checked`); do **not**
     read `aria-checked`, and do **not** use `data-cid`/title as the mic-state
     source. Rewrite `muteRequired` to take that boolean. **Preserve test-first
     coverage:** `muteRequired` stays a pure,
     unit-tested predicate (assert mute-required when ON, not when OFF).
* **Already-valid selectors (do NOT churn — refresh provenance only):**
  `joinNowSel = [data-tid="prejoin-join-button"]`,
  `displayNameInputSel = input[data-tid="prejoin-display-name-input"]`, the mic
  toggle *selector* `micToggleSel = [data-tid="toggle-mute"]`, and
  `lobbyText = "Someone will let you in shortly"` are all confirmed valid by the
  2026-09-05 live evidence; only refresh their `VERIFY LIVE` comment date.
* **Timing knob (made precise):** the observed 123s was the harness
  `context.WithTimeout(2*time.Minute)` budget consumed by `WaitVisible`, not a
  fixed sleep. Two distinct, separately-owned knobs:
  * poll cadence → `admissionPollInterval` in `meeting_chrome.go`;
  * total admission budget (incl. manual lobby-admission latency) →
    `context.WithTimeout` in `meeting_integration_test.go`.
  Change the budget knob ONLY if U1 shows manual lobby admission needs more time;
  if so, `meeting_integration_test.go` is added to this unit's file scope.
* **Headed/UA fallback branch (contingent on U1):** if U1 pins `--headless=new`
  DOM variance or the Chrome/125 UA interstitial as the cause, U3 may switch the
  harness/allocator to headed and/or revise `DefaultUserAgent` (`allocator.go`)
  instead of / in addition to selector edits — bounded to what U1's verdict
  authorizes.
* **Teardown safety (protects the organizer):** any browser teardown MUST target
  only the bot's own launched browser instance/PID via the harness's own
  `context` cancel (`t.Cleanup(cancelBrowser)` already does this); image-wide
  `taskkill`/`Stop-Process -Name`/`pkill` of `chrome`/`msedge` is forbidden — it
  would kill the organizer on CDP 9223.
* **Files:** `internal/browser/prejoin.go` (selector consts: add `joinOnWebSel`,
  change `meetingToolbarSel`, rewrite `muteRequired`) and
  `internal/browser/meeting_chrome.go` (`Join` launcher step; `enforceMute`
  mute-state read) — 2 files, well within the <3 bound. The optional
  `admissionPollInterval`/budget and headed/UA changes below only apply if a live
  cycle shows they are needed; if the budget knob changes
  `meeting_integration_test.go` (test infrastructure), split that into its own
  micro-unit so production edits and test-infra edits are not combined.
* **Identity mode:** confirmed **guest** (2026-09-05 evidence). Selectors and the
  expected admitted-toolbar/lobby surface target the guest lightweight UI; the
  `#hangup-button` admission selector also covers the full client, so an
  authenticated-drift run still detects admission. Flag any sign-in/MFA landing as
  an operator checkpoint rather than silently proceeding.
* **Verify:** `TestMeetingJoin` passes green live (launcher handled → pre-join →
  muted → join → admitted via `#hangup-button`); the `muteRequired` unit test is
  green (mute required when the mic reads ON, not when OFF); `internal/browser`
  unit tests green; full gates (`go vet`, `golangci-lint`, `gofmt -l`, `go test`)
  clean; `VERIFY LIVE` provenance comments refreshed with the 2026-09-05 date.
* **Bounded-cycle stop condition (2-hour rule):** if the join flow is not green
  within a bounded number of live cycles (e.g. 3), stop and file a follow-up task
  rather than exceeding the 2-hour budget.
* **Depends on:** U1 (URL-mutation verdict), U2 (mutated URL that still reaches
  the launcher/guest pre-join).

### U4 — Caption activation/observer selector update → green `TestMeetingEnableCaptions`

* **Domain:** code (Go). **Posture:** characterization-first.
* **What:** Update the caption selectors in `internal/browser/captions.go` to the
  2026-09-05 live values and make `activateCaptions` handle **both** UI variants.
  Concrete selector changes (see Live Selector Evidence table):
  * `moreActionsSel`: `[data-tid="more-button"]` → `#callingButtons-showMoreBtn`
    (guest and full client).
  * `languageSpeechSel`: `[data-tid="language-speech-button"]` →
    `#LanguageSpeechMenuControl-id` (**full client only** — absent in guest).
  * `turnOnCaptionsSel`: `[data-tid="closed-captions-button"]` →
    `#closed-captions-button` (stable id; live `data-tid` varies —
    `closed-captions-button-on` in full client, absent in guest — so bind to the
    **id**, not the data-tid).
  * `captionWrapperSel`: `[data-tid="closed-caption-renderer-wrapper"]` — **valid,
    keep** (prefer interpolating this shared const into `captionObserverJS` rather
    than duplicating it as a raw JS literal — see the single-source-of-truth
    fold-in below).
* **Two-variant activation (no broad silent fallback):** rewrite the fixed
  3-`Click` chain in `activateCaptions` to: click `#callingButtons-showMoreBtn`;
  **if** `#LanguageSpeechMenuControl-id` becomes visible (full client) click it,
  **else** (guest lightweight) skip straight to the captions menuitem; click
  `#closed-captions-button`; then `WaitVisible` `captionWrapperSel`. Branch on an
  explicit bounded presence check for the submenu — do not blind-click a possibly
  absent `languageSpeechSel`, and do not swallow a genuinely stale selector.
* **Still VERIFY LIVE (not covered by the 2026-09-05 evidence):** the in-JS
  literals inside `captionObserverJS` — `closed-caption-text`, `author`, and the
  `.fui-ChatMessageCompact` container class — must still be confirmed against a
  live finalized caption. **Selector-sync checklist (avoid Go-const vs JS-literal
  drift):** keep `captionWrapperSel` and the wrapper literal in sync, and verify
  the three inner literals in the same edit. (A silent miss makes the observer
  IIFE return `false` and `EnableCaptions` blocks with no error.)
* **Prefer shared selector constants over duplicated raw JS literals (fold-in):**
  rather than hand-duplicating `captionWrapperSel` (and any other selector already
  defined as a Go const) as a raw literal inside `captionObserverJS`,
  **interpolate the shared Go const into the JS** (e.g. build the observer script
  with `fmt.Sprintf`/string concatenation from `captionWrapperSel`) so the Go
  const is the single source of truth and const-vs-literal drift becomes
  structurally impossible. Where a name genuinely lives only in JS (the inner
  `closed-caption-text`, `author`, `.fui-ChatMessageCompact`), the selector-sync
  checklist above remains the backstop; verify those in the same edit.
* **Robustness (fold-in):** wrap each `activateCaptions` menu step in a **bounded
  per-step `chromedp.WaitVisible` with a short timeout** (rather than the current
  blocking `Click` that retries until the full 2-minute ctx expiry, or a single
  instantaneous `existsJS` probe that could false-negative a valid-but-slow menu
  reveal). Menu items render only *after* the prior menu `Click`, so a short
  bounded wait fails a genuinely stale selector fast with an identifiable name
  while tolerating normal render/animation latency and enabling the guest-vs-full
  branch decision.
* **Live precondition:** `TestMeetingEnableCaptions` waits for a *finalized*
  utterance (text ending `.!?`); a speaker must be audibly talking during the
  run. Record this as an operator/fixture precondition (U6 runbook).
* **Bounded-cycle stop condition (2-hour rule):** if the caption flow is not green
  within a bounded number of live cycles (e.g. 3), stop and file a follow-up task
  rather than exceeding the 2-hour budget.
* **Files:** `internal/browser/captions.go` (selector consts + `captionObserverJS`
  literals) and `internal/browser/meeting_chrome.go` (`activateCaptions`
  two-variant branch) — 2 files, within the <3 bound.
* **Verify:** `TestMeetingEnableCaptions` passes green (a finalized caption
  delivered to the sink); unit tests green; full gates clean.
* **Depends on:** U3.

### U5 — Audio capture path verification → green `TestMeetingCaptureAudio`

* **Domain:** code (Go). **Posture:** characterization-first.
* **Prior art (from `docs/compound/`):** the MP3 encoder is already hardened —
  `shine-mp3-frame-alignment.md` (frame-buffer non-aligned input) and
  `shine-mp3-race-checkptr.md` (capacity-slack for `-race` checkptr). U5 verifies
  the live capture path and MUST NOT re-break frame alignment or the `-race`
  behavior when touching capture JS.
* **What:** Confirm the `getDisplayMedia` auto-approval path in
  `internal/browser/audio.go` works with
  `--auto-select-desktop-capture-source="Microsoft Teams"` (`allocator.go`);
  adjust the capture-source match string or capture JS only if live evidence
  requires it. Refresh the `VERIFY LIVE` provenance comment.
* **Live evidence (2026-09-05):** the capture **prerequisites are present live** —
  `navigator.mediaDevices.getDisplayMedia` and `AudioContext` both resolve in the
  guest meeting tab — so the API surface used by `audioCaptureJS` is **not** the
  open question. What remains **unverified** is end-to-end audio **streaming**:
  non-zero PCM forwarded over the `sendAudioToGo` binding and non-zero MP3 bytes
  written. U5's exit therefore explicitly separates "prereqs present" (already
  confirmed — not a pass by itself) from "stream verified" (the green
  `TestMeetingCaptureAudio` with a **stimulus-verified** capture — not merely
  non-zero MP3 bytes — and a clean cancel). Do not downgrade prereq presence into
  a pass.
* **Fake-media distinction (critical).** `allocator.go` sets
  `use-fake-device-for-media-stream` and `use-fake-ui-for-media-stream`, so Chrome
  can supply a **synthetic** audio device — a naive non-zero-MP3-bytes check can
  pass on fake media **without ever capturing real meeting/tab audio**. The
  verification MUST distinguish fake-media capture from real meeting/tab audio:
  assert the captured signal matches a **known live stimulus** (a deterministic
  tone or spoken phrase injected into the meeting/tab) via a signal check
  (expected dominant frequency / energy in the known band, or non-silence
  correlated to the stimulus window), or exercise the capture assertion against
  the real tab source rather than the fake device. Non-zero MP3 bytes alone are
  NOT proof of real audio capture.
* **Live precondition:** meeting audio must be present during capture (a speaker
  or shared audio) for non-zero MP3 bytes. Record as a U6 precondition.
* **Bounded-cycle stop condition (2-hour rule):** if the capture path is not green
  within a bounded number of live cycles (e.g. 3), stop and file a follow-up task
  rather than exceeding the 2-hour budget.
* **Files:** `internal/browser/audio.go` (+ `allocator.go` only if the
  capture-source match string must change) (<3 files).
* **Verify:** `TestMeetingCaptureAudio` passes green with a **stimulus-verified**
  assertion — the captured audio must contain a **known live stimulus** (a
  deterministic tone or spoken phrase played into the meeting/tab), checked by a
  signal test (expected dominant frequency / energy in the known band, or
  non-silence correlated to the stimulus window), **not merely non-zero MP3
  bytes**, and must be distinguishable from Chrome fake-media output; clean return
  after cancel; run with `-race` where feasible; unit tests green; full gates
  clean.
* **Depends on:** U3.

### U6 — Live-harness reproducibility runbook + closure evidence

* **Domain:** docs. **Posture:** documentation / operational closure.
* **What:** Write a reproducible runbook for the live integration harness:
  required env (`GO_TEAMS_INTEGRATION=1`; `TEAMS_PROFILE_DIR` = separate
  **git-ignored, workspace-contained** bot profile — **guest (confirmed
  2026-09-05); an authenticated-identity landing is an operator checkpoint**;
  `TEAMS_MEETING_URL` sourced from the out-of-tree private file — **never
  committed**), the **`TEAMS_BROWSER_PATH` primary harness setup** (point the
  harness at the browser executable via U7; prepending Chrome-for-Testing to PATH
  is only a historical fallback for the first-run `google-chrome` abort), the **join-path launcher note** (a `/meet/`
  link on a fresh guest profile redirects to `/dl/launcher/launcher.html` and the
  harness must click `joinOnWeb` before pre-join), the organizer/lobby-admission
  and bot-side login/MFA human checkpoints, the live speech/audio stimulus
  preconditions (U4/U5), and the browser-verification closure evidence (redacted
  route, headed/headless mode, verdicts) for the three now-green flows.
* **Confirmed selector inventory (record in the runbook, redacted):** capture the
  2026-09-05 Live Selector Evidence table so future selector drift is diffable —
  guest launcher `button[data-tid="joinOnWeb"]`; pre-join display-name/join/mic
  toggle; mute state via the `checked` property (no `aria-checked`); admission via
  `#hangup-button`; captions via `#callingButtons-showMoreBtn` → (full only)
  `#LanguageSpeechMenuControl-id` → `#closed-captions-button` →
  `[data-tid="closed-caption-renderer-wrapper"]`. Note the guest-direct vs.
  full-nested caption variants. **Do not** include any live URL/token.
* **Docstring alignment (in scope):** update the stale `meeting_integration_test.go`
  `browserContext` docstring that says the profile is "already authenticated" to
  match the **confirmed guest** topology (a docs/comment-only edit). This is a
  comment-only change, preserving U6's docs domain.
* **Secrets discipline (NON-NEGOTIABLE):** examples reference `TEAMS_MEETING_URL`
  only; record a **redacted route class** (`https://teams.microsoft.com/meet/{id}`,
  `p` stripped) — the closure template's `URL:` field takes the redacted class,
  never the live link. Explicitly BAN committing screenshots/HAR/traces/console
  logs that include the address bar, `p=` token, cookies, or profile paths.
  State that organizer and bot user-data dirs are **secret material** (out of
  git, never copied into `docs/` or test artifacts); do NOT attach to CDP 9223.
* **Live-validation serialization:** because U4 and U5 both launch a browser
  against the same `TEAMS_PROFILE_DIR`/meeting, their **live** validation runs
  MUST be serialized (or use distinct profile dirs) to avoid profile-lock /
  nondeterministic admission — even though their code changes are independent.
* **Workflow learnings (from `docs/compound/`):** the closure/PR checklist heeds
  `amend-breaks-shipment-gate.md` (no `git commit --amend`/rebase after items are
  marked done) and `copilot-rereview-timing.md` (don't merge on a false-clean
  thread count; wait for Copilot review at current HEAD).
* **Compound-refresh follow-up:** after U6, promote BOTH distinct root causes —
  the agent-browser managed-launch DevToolsActivePort failure and the chromedp
  harness `google-chrome` PATH-discovery failure (remedied by `TEAMS_BROWSER_PATH`)
  — into `docs/compound/` as separate learnings so future compound
  searches surface them (today they live only in `docs/memory/`).
* **Files:** `docs/closure/browser-automation/` (net-new runbook + closure entry;
  do not overwrite the parent blocked-closure record) + a comment-only docstring
  fix in `internal/browser/meeting_integration_test.go` (<3 files; docs/comment
  domain only — no executable code change).
* **Verify:** runbook reproduces the green run from a clean shell without
  embedding the private URL; closure entry records the three passing flows and
  any residual blocked/partial flow as a follow-up condition.
* **Depends on:** U3, U4, U5.

### U7 — Integration-harness browser executable-path option (test-infra enabler; runs first)

* **Domain:** test infrastructure only. **Posture:** test-first-safe (pure
  superset — an unset var yields byte-identical current behavior). **Runs first —
  prerequisite of U1** (and therefore of every live-harness unit).
* **Added post-gate (2026-09-05)** from the harness-runnability finding in the
  Live Selector Evidence section, and independently scope-audited (Scope Boundary
  Auditor verdict **ADVISORY / lean PASS**); see the post-gate note in
  `## Plan Review`. It bypassed the original attempt-1..3 plan-review gate, so it
  is recorded here explicitly rather than as an undocumented queue mutation.
* **Problem:** on an Edge-only Windows host, chromedp browser auto-discovery
  aborts with `exec: "google-chrome": executable file not found in %PATH%`; the
  only way the live harness ran was an ad-hoc PATH prepend of the
  Chrome-for-Testing directory. U6 documents this as a precondition, but a manual
  PATH trick is un-versioned and easy to get wrong.
* **What (narrowest form — test-only, NO production change):** in
  `internal/browser/meeting_integration_test.go`'s `browserContext(t)` helper,
  read a new env var (e.g. `TEAMS_BROWSER_PATH`); when set, append
  `chromedp.ExecPath(path)` to the option slice returned by
  `browser.AllocatorOptions(...)` before `chromedp.NewExecAllocator`; when unset,
  fall back to the current auto-discovery (PATH) behavior. Pure superset — no
  behavior change when the var is absent; `chromedp.ExecPath` is appended after
  `DefaultExecAllocatorOptions`, so it correctly overrides auto-discovery.
* **Scope boundary (do NOT widen — audited):** the production runtime
  (`internal/browser/runner.go`, `meeting.go`) shares the identical allocator
  call and is *also* broken on Edge-only hosts, but adding an `ExecPath` field to
  the production `browser.Options` struct is **out of 010-F scope** (that is
  runtime deployment portability — a different feature) and is filed as a
  **separate follow-up stash entry**. U7 touches test infrastructure ONLY and is
  NOT folded into U3/U4/U5 (production-Go domain), preserving width isolation.
* **Secrets:** `TEAMS_BROWSER_PATH` can embed a user home path — treat it like
  `TEAMS_PROFILE_DIR` (Protected Invariant 5): never echo or commit its value
  into redacted evidence under `docs/`.
* **Files:** `internal/browser/meeting_integration_test.go` (1 file, ~5 lines).
* **Verify:** with `TEAMS_BROWSER_PATH` pointing at the Chrome-for-Testing binary
  and the CfT dir NOT on PATH, the live harness launches the browser (no
  `google-chrome not found` abort); with the var unset, behavior is byte-identical
  to today. Because this is `integration`-tagged, the default `go test ./...` gate
  is unaffected; full gates (`go vet`, `golangci-lint`, `gofmt -l`) stay clean.
  Because the helper only builds under the `integration` tag, a plain
  build / `go vet ./...` will **not** catch a compile break in it — U7 MUST be
  compile-verified **under the tag**: e.g. `go vet -tags integration
  ./internal/browser/...` and a tagged no-run compile such as `go test -tags
  integration -run '^$' ./internal/browser/...` (forces the tagged file to
  compile without a live meeting).
* **Depends on:** none (root enabler).

## Dependency Graph

```text
U7 ──▶ U1 ──▶ U2 ──▶ U3 ──┬──▶ U4 ──┐
                          └──▶ U5 ──┴──▶ U6
```

* U7 (test-infra enabler) has no upstream; it makes the live harness launchable
  on an Edge-only host via an explicit `TEAMS_BROWSER_PATH` option and is the
  prerequisite of U1 (added post-gate; see `## Plan Review`).
* U1 (diagnostic) depends on U7; it de-risks U3/U4/U5.
* U2 depends on U1 (URL findings).
* U3 depends on U1 + U2.
* U4 and U5 both depend on U3 (join must work first); their **code** is
  parallel, but their **live validation** is serialized (shared profile/meeting —
  see U6) or uses distinct profile dirs.
* U6 depends on U3 + U4 + U5 (captures final verified state).
* Acyclic.

## Decisions and Rationale

* **Diagnostic-first (U1) before selector edits** — the launcher root cause is
  already CONFIRMED from 2026-09-05 live evidence (a fresh-guest `/meet/` link
  redirects to `/dl/launcher/launcher.html`, requiring a `joinOnWeb` click before
  pre-join), so U1 does NOT re-diagnose it and U3 implements that confirmed
  launcher fix. U1 instead assesses the remaining open questions — `/meet/`
  URL-mutation behavior and caption-activation variants (guest-direct vs
  full-nested) — so U3/U4/U5 are apply-and-verify.
* **URL hardening (U2) is its own unit** — the `/meet/` vs `/l/meetup-join`
  divergence is a distinct code/domain concern with unit-test coverage,
  separable from DOM selector work.
* **One unit per runtime surface (U3/U4/U5)** — preserves width isolation and a
  single green-test exit each; captions/audio depend on a working join.
* **Feature (not chore) as the covering release unit** — backlogit has no `chore`
  type configured; `feature` is the top-level type.
* **Runbook as a docs-only unit (U6)** — keeps closure documentation single-
  domain and satisfies browser-verification closure-handoff.
* **Harness executable-path option (U7) is test-only; production portability
  deferred** — the Edge-only `google-chrome not found` abort is fixed in the
  integration harness (U7) via an explicit `TEAMS_BROWSER_PATH` /
  `chromedp.ExecPath` option, version-controlling the fix instead of relying on a
  manual PATH prepend. The identical production-runtime gap (`runner.go` /
  `meeting.go`) is **out of 010-F's** "selector/path hardening + verification"
  scope (it is runtime deployment portability, a different feature) and is filed
  as a separate follow-up stash entry — per the Scope Boundary Auditor's
  keep-narrow verdict.

## Risks and Caveats

* **Teams DOM drift** — selectors confirmed today may drift; mitigated by dated
  provenance comments and the U6 runbook.
* **Human checkpoints** — the *organizer* session (Edge on CDP 9223) already
  handled its own OAuth/MFA and is out of scope for the bot. The *bot* profile
  has its own explicit human checkpoints: lobby admission by the organizer, and —
  only if U1 selects an authenticated identity — bot-side login/MFA. Bot
  credential entry is **never automated**; it is an explicit U1/U6 operator
  checkpoint.
* **Secret leakage** — the private join link must never enter an artifact/commit;
  all units reference `TEAMS_MEETING_URL` only.
* **Organizer session** — must not terminate/alter Edge on CDP 9223; the bot uses
  the separate empty profile and launches its own browser.
* **Live-run flakiness/timeouts** — the 2-minute join / 1-minute audio harness
  budgets may need tuning (U3), a bounded change only if U1 shows it is needed.
* **Environment prerequisite** — the harness is pointed at the browser executable
  via the `TEAMS_BROWSER_PATH` option (**U7**) as the PRIMARY setup; prepending
  Chrome-for-Testing to PATH is only a historical fallback. Without either, the
  harness aborts before any DOM work with `google-chrome not found` (captured
  in U6). **U7** provides the explicit `TEAMS_BROWSER_PATH` harness option so an
  Edge-only host can run without the PATH trick; the identical **production-runtime**
  gap (`runner.go`/`meeting.go` share the same allocator call) is out of 010-F
  scope and is filed as a separate follow-up (deferred stash 87CDB97A).

## Constitution Check

Principle-by-principle mapping of U1–U7 against the workspace constitution and
workflow policies:

* **Test-first / red-green (Principle II, P-002/P-004):** U2 is test-first
  (synthetic-fixture unit tests). U3/U4/U5 are characterization-first with the
  `integration`-tagged live test as the oracle. **Justified deviation:** the
  live integration tests are behind the `integration` build tag and are not run
  by default `go test ./...`, so the red phase is confirmed by a recorded live
  `-tags integration` run (the observed `pre-join not visible` failure), not the
  default gate. The simpler alternative (pure unit oracle for DOM selectors) was
  rejected because selector correctness is only observable against live Teams
  DOM. Each code unit still runs the full default gate for regressions. **U7** is
  a **test-only pure superset** (an unset `TEAMS_BROWSER_PATH` yields
  byte-identical current behavior), so it needs no red phase of its own; it is
  compile-verified under the `integration` tag (Principle I gate).
* **Quality gates (Principle I):** every code unit's Verify names
  `go vet ./...`, `golangci-lint run`, `gofmt -l .`, and `go test`.
* **CLI/workspace containment (Principle IV):** `TEAMS_PROFILE_DIR` is asserted
  to resolve to a git-ignored path inside the workspace (U1 precheck, U6
  runbook).
* **Decomposition integrity (P-003):** source → deliberation `001-DL` → this plan
  → feature → tasks; acyclic DAG (U7 is the root enabler; U1–U6 depend
  transitively on it); every unit has a verifiable exit and ≥1 acceptance
  criterion.
* **Hardening (P-006):** `Requires plan hardening: yes`; `## Plan Hardening`
  present with strict-safety ProposedAction/ActionRisk classification.
* **Role boundary (P-010):** U1 is explicitly a Ship-pipeline diagnostic (not a
  Stage spike). Stage produces only planning/decomposition artifacts; Ship runs
  the harness and edits source. U7 is likewise a **Ship-executed test-infra
  enabler** (not a Stage spike); Stage only planned and sequenced it.
* **Dependency hygiene / width isolation (Principle VI):** no new dependencies;
  one skill domain per unit. **Width-isolation note:** U1 is test-infra +
  redacted note (single diagnostic domain); U3 keeps production selector edits
  separate from any test-infra budget-knob change via the micro-split option; U6
  is docs/comment-only (runbook + a comment-only docstring fix); **U7 is
  test-infra only** — deliberately NOT folded into the production-Go units
  U3/U4/U5, preserving width isolation.
* **Simplicity / YAGNI (scope value — not a numbered constitution principle):** no speculative abstraction — U2 changes
  only the `/meet/` branch and leaves the legacy path byte-identical; U1's
  diagnostic probe is build-tagged scaffolding (behind a dedicated opt-in) that is
  not shipped as production behavior; U7 is the **narrowest test-only form** — the
  wider production `Options.ExecPath` portability change is explicitly **deferred
  to a separate feature** (stash `87CDB97A`), not built here; CDP-attach is
  explicitly deferred rather than built.
* **Structured Observability / evidence (Principle V):** every unit emits a verifiable
  artifact — U1 a dated redacted findings note, U2–U5 green named tests, U6 a
  reproducible runbook + closure entry recording the three passing flows and any
  residual blocked flow.
* **Bounded effort / 2-hour rule (Development Workflow > Task Granularity):** U3/U4/U5 each carry an explicit
  bounded-cycle stop condition; oversize discoveries are filed as follow-up tasks
  rather than expanding a unit past budget.
* **Safety (Principle VII, strict-safety):** destructive/broad process kills
  forbidden; live-auth run classified `high`; approval/checkpoint path recorded.
* **Secrets (Principle III):** private URL, tokens, cookies, and profiles are
  secret material; redaction and synthetic-fixture rules bind U1/U2/U6, and
  `TEAMS_BROWSER_PATH` (U7) is bound by Protected Invariant 5 — its value is never
  echoed or committed into redacted `docs/` evidence.
* **Artifacts (Principle IX):** markdown + frontmatter; parent-owned evidence
  protected from overwrite.

## Plan Hardening Signals (REQUIRED)

* Public API / schema / contract change — **absent**. `MutateJoinURL` /
  `ValidateMeetingURL` signatures are unchanged; only URL-mutation behavior and
  internal selector constants change.
* Security / auth / permission / compliance-sensitive behavior — **present
  (sensitive-data handling).** Live Microsoft auth is in play and a private join
  link and authenticated browser profiles are involved; the secret-handling and
  no-leak constraints are release-critical even though no auth *code* changes.
* Migration / backfill / destructive / irreversible step — **absent.** No data or
  config migration; selector edits are reversible.
* External integration / operator checkpoint / external dependency — **present.**
  Depends on live Microsoft Teams DOM, a live meeting, a manually launched Edge
  organizer (CDP 9223), and operator lobby-admission/OAuth checkpoints.
* High runtime / rollout / rollback risk — **present.** Iterative live-browser
  verification with real timeouts and an external, drift-prone DOM surface.

**Requires plan hardening: yes**

## Runtime Verification and Closure

* **Changed runtime surfaces:** browser UI automation (pre-join FSM, caption
  activation/stream, tab-audio capture) and the join-URL mutation feeding it.
* **Runtime verification (per unit):**
  * U2 — unit tests over `/meet/` URL mutation (deterministic, headless-safe).
  * U3 — live `TestMeetingJoin` green (bot admitted to the meeting).
  * U4 — live `TestMeetingEnableCaptions` green (finalized caption delivered).
  * U5 — live `TestMeetingCaptureAudio` green: the captured MP3 contains the
    KNOWN live audio stimulus (signal correlated to the stimulus window),
    distinguishing real meeting/tab audio from Chrome fake-media, with a clean
    cancel. Non-zero MP3 bytes alone are NOT a sufficient pass.
* **Operational closure:** U6 runbook + browser-verification closure entry
  recording routes, headed/headless mode, human checkpoints, and per-flow
  verdicts. Blocked/partial flows (if any remain) become explicit follow-up
  monitoring conditions rather than silent gaps.

## Plan Hardening

**Hardening required:** yes. Triggers: sensitive-data handling (live Microsoft
auth, private join link, authenticated browser profiles), external integration
(live Teams DOM + live meeting + manually launched Edge organizer on CDP 9223),
and high runtime/rollback risk (iterative live-browser verification against a
drift-prone external surface).

**Consulted:** `browser-verification.instructions.md` (server-availability,
route-selection, headed/headless, human-checkpoint, validator-evidence, and
closure-handoff protocols); `strict-safety.instructions.md`;
`circuit-breaker.instructions.md` (parent circuit-break record for the failed
managed Chrome launch); `docs/compound/` — NOT empty (5 entries): the two
`shine-mp3-*` learnings govern U5 (encoder already hardened via frame-buffer +
capacity-slack — do not re-break), and `amend-breaks-shipment-gate.md` /
`copilot-rereview-timing.md` govern U6 closure/PR discipline.

### Protected Invariants

1. The private join link never enters a backlog item, doc body, log, or commit;
   downstream references only `TEAMS_MEETING_URL`, sourced from the out-of-tree
   session-state file.
2. The Edge organizer session (CDP port 9223) and its profile are never
   terminated or altered; the bot uses the separate empty `TEAMS_PROFILE_DIR`
   and launches its own browser. Teardown targets ONLY the bot's own launched
   browser PID (harness `context` cancel); no image-wide `taskkill`/
   `Stop-Process -Name`/`pkill` of `chrome`/`msedge`.
3. Parent-owned evidence docs (`docs/memory/2026-09-05/circuit-break-...md`,
   `docs/closure/browser-automation/2026-09-05-...md`) are not overwritten; only
   net-new closure/diagnostic notes are added.
4. Public function signatures of `ValidateMeetingURL` / `MutateJoinURL` are
   preserved; legacy `/l/meetup-join` behavior is not regressed.
5. Organizer and bot browser user-data dirs are secret material: git-ignored,
   workspace-contained, never copied into `docs/` or test artifacts. Diagnostic
   and closure evidence carry only a redacted route class + selector inventory —
   never the `p` token, full join URL, cookies, `localStorage`, or profile paths.

### Risky Actions (ProposedAction / ActionRisk / ActionResult)

| ProposedAction | ActionRisk | Approval | Expected ActionResult |
|---|---|---|---|
| Run the live integration harness against the authenticated meeting (U1/U3/U4/U5) | **High** — live Microsoft auth, sensitive-data surface, external drift-prone DOM, real timeouts | Operator checkpoints: bot-side login/MFA (BLOCKED, never automate creds) and lobby admission; prefer approval before first live run | Test outcome + redacted validator evidence recorded; organizer session untouched; no secret written |
| Change `MutateJoinURL` behavior for `/meet/` links (U2) | Medium — could regress the legacy meetup-join path | Covered by test-first; no operator approval needed if legacy tests stay green; edit gated on U1 verdict | `/meet/` mutation correct; legacy branch byte-identical; synthetic fixtures only |
| Edit `VERIFY LIVE` selector constants (U3/U4/U5) | Low-Medium — reversible constant edits; wrong values re-break live flows | None beyond the gating live test | Each gating live test green; provenance comment refreshed |
| Switch to headed / revise UA (U3, conditional) | Low-Medium — changes bot runtime posture | Only when U1 pins headless/UA as root cause | Headed/UA change justified by U1 evidence |
| Adjust join budget/poll cadence (U3, conditional) | Low — bounded numeric change | None | Timeout/cadence change justified by U1 evidence |

### Verification Depth (environment prechecks + blocked-path handling)

* **Prechecks before any live run:** confirm `TEAMS_BROWSER_PATH` is set to the
  browser executable as the primary harness setup (prepending Chrome-for-Testing
  to PATH is only a historical fallback for the first-run `google-chrome` abort);
  confirm the Edge
  organizer/meeting is live and caption controls are visible; confirm
  `TEAMS_PROFILE_DIR` points to the separate empty bot profile and
  `TEAMS_MEETING_URL` resolves from the out-of-tree file.
* **Target scenarios:** join admitted; finalized caption delivered; captured MP3
  contains the KNOWN live audio stimulus (signal correlated to the stimulus
  window, real meeting/tab audio distinguished from Chrome fake-media) with a
  clean cancel — non-zero MP3 bytes alone are NOT a sufficient pass.
* **Blocked-path handling:** if server/meeting availability cannot be
  established, record `BLOCKED` validator evidence naming the exact missing
  prerequisite rather than downgrading to a false pass; if the bot waits in
  lobby, raise the operator admission checkpoint and record its result.

### Operational Closure, Monitoring, Rollback

* **Monitoring signals:** per-flow test verdict, time-to-admit, caption-delivery
  latency, captured-byte count — recorded in the U6 closure entry.
* **Rollback trigger / procedure:** any selector/URL edit that leaves a
  previously green live flow red is reverted to the prior constant/behavior
  (edits are pure constant/behavior changes, `git revert`-safe); no data or
  config state to unwind.
* **Owner / validation window:** the Ship executor owns the live run; validation
  window is the single live session while the organizer meeting is up — the
  runbook (U6) makes re-validation reproducible after the session ends.
* **External-dependency fallback:** if live Teams DOM cannot be reached in a
  future re-run, the flow is reported blocked (not silently skipped), consistent
  with the parent circuit-breaker discipline.

### Review-Gate Capability Risks (carried into plan-review)

* Plan review MUST emit literal `dispatch_mode:` and `decision:` markers so
  `harvest` can fail closed if reviewer dispatch was skipped.
* If reviewer subagent dispatch or model-specific routing is unavailable, plan
  review MUST declare the degradation (`single-agent-declared-degradation` or
  `same-model-declared-degradation`) and still cover every selected persona,
  including the Security Lens Reviewer (triggered by sensitive-data/secret
  handling) — not skip it.

### Unresolved Operator Decisions (do not block harvest; resolved during execution)

* **Bot identity mode** — **confirmed guest** by the 2026-09-05 live evidence
  (manual guest join + admission succeeded). An authenticated-identity landing
  (Microsoft sign-in/MFA) remains an explicit operator checkpoint, never
  automated, and switches the affected selectors to their full-client variants.
* Whether the bot profile requires organizer lobby admission (human checkpoint) —
  confirmed yes in the manual run; re-confirm per live run.
* Whether bot-side login/MFA is required — only on an authenticated-identity
  drift; an explicit operator checkpoint, never automated.

Attaching to the existing organizer CDP endpoint (9223) is **explicitly out of
scope** and is NOT an in-run decision: the bot always launches its own isolated
browser. Any future CDP-attach exploration is a separate follow-up, not part of
this shipment.

## Plan Review

dispatch_mode: multi-agent
decision: FAIL

**Attempt 1.** Reviewed by 6 personas via reviewer subagent dispatch. Always-on:
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings
Researcher. Cross-model (different providers for genuine diversity):
Architecture Strategist (gpt-5.6-sol), Security Lens Reviewer (grok-4.6) —
triggered by sensitive-data/secret handling. Agent-Native Parity Reviewer not
triggered (no MCP/agent-facing surface). Plan hardening was required and is
present; strict-safety risky-action classification is present.

**Persona coverage:** Constitution — subagent (ADVISORY); Go — subagent
(ADVISORY); Scope Boundary — subagent (ADVISORY); Learnings — subagent
(ADVISORY); Architecture Strategist — cross-model subagent (FAIL); Security Lens
— cross-model subagent (FAIL).

**Gate: FAIL** — multiple P1 findings (and two persona FAILs) force a revision
cycle before harvest.

### P1 findings (must resolve)

1. **Secret-leak vector in U1 DOM capture and U6 route recording** (Security Lens
   ×4, Constitution) — for this product the "live pre-join DOM" and recorded
   "route" literally contain the private `/meet/<id>?p=<token>` link, cookies,
   and profile paths. U1/U6 lacked a redaction rule. Fix: capture selector
   inventory + redacted route class only; ban HTML/HAR/screenshot/cookie/
   localStorage dumps; scrub query strings/storage/logs before any write.
2. **Tests could freeze the live `p=` token into `url_test.go`** (Security Lens)
   — mandate synthetic fixtures (`meet/example-id?p=synthetic-token`); forbid
   copying `TEAMS_MEETING_URL`/U1 DOM strings into tests, comments, commits.
3. **Bot-side OAuth/MFA is unmodeled; organizer auth is the wrong trust
   boundary** (Security Lens) — the empty bot profile may land on Microsoft
   login/MFA, not pre-join. Add an explicit bot-side checkpoint (guest pre-join
   vs login/MFA vs lobby); never automate credential entry; never copy profile
   state into artifacts.
4. **U2 acceptance criteria not Go-verifiable / mechanism mislabeled** (Go ×3) —
   `MutateJoinURL` already preserves `p`; the real defect is unconditionally
   forcing `type=meetup-join`. Reframe U2 as branch-on-path-prefix that
   suppresses legacy params for `/meet/`, keeps the legacy branch byte-identical,
   and pins concrete assertions (`url.Parse` + `Get("p")`, `type != meetup-join`,
   path unchanged).
5. **Join-timing fix points at the wrong file** (Go) — the 123s is the harness
   `context.WithTimeout` budget + `WaitVisible`, not a fixed sleep in
   `meeting_chrome.go`. Clarify the knob (poll `admissionPollInterval` vs total
   budget in the integration test) and add the harness file to U3 scope if the
   budget changes.
6. **U1 cannot isolate root cause with a single-config harness** (Architecture) —
   add a diagnostic probe comparing raw vs mutated URL, headed vs headless, and
   fixed vs native UA, with redacted evidence.
7. **U1→U3 evidence invalidated by U2** (Architecture) — U1 inventories the
   pre-U2 route; U2 changes it. Make post-U2 live landing verification a U2 exit
   gate; that post-hardening DOM is U3's selector oracle.
8. **Browser identity/topology contradictory** (Architecture) — pick one
   canonical verification topology (isolated bot browser; guest-or-authenticated
   bot identity); keep organizer CDP 9223 separate; attach-to-CDP is future-only.
9. **Missing mandatory Constitution Check section** (Constitution) — add a
   principle-by-principle mapping and document the live-integration-test
   deviation as justified.
10. **`docs/compound/` falsely claimed empty** (Learnings) — it holds 5 entries
    incl. `shine-mp3-frame-alignment.md` / `shine-mp3-race-checkptr.md` (govern
    U5 MP3 bytes/`-race`) and two workflow-issues (govern U6). Correct the
    provenance and cite them.

### P2 findings (fold in)

* Caption selector duplication: `captionWrapperSel` Go const vs four in-JS
  literals (`closed-caption-renderer-wrapper`, `closed-caption-text`, `author`,
  `.fui-ChatMessageCompact`) — U4 must sync all as one checklist (Go, Architecture).
* `activateCaptions` uses blocking `chromedp.Click` retry — add probe-first
  (`existsJS`) pattern so a stale selector fails fast (Go).
* `TEAMS_PROFILE_DIR` workspace containment unspecified (Constitution) — assert
  it resolves inside the workspace (git-ignored) or justify.
* Organizer non-termination vs harness teardown broad process kills
  (Constitution) — teardown MUST target only the bot's own launched PID; no
  image-wide `taskkill`/`pkill`.
* Live-auth harness run should be `ActionRisk: high`, not Medium (Constitution).
* U3 needs a headed-mode/UA fallback branch if U1 pins headless/UA as cause
  (Learnings).
* Captions/audio green tests require live speech/audio stimulus — add an
  operator/fixture precondition (Go, Architecture).
* U4/U5 parallel live validation shares one profile/meeting — serialize live
  validation or provision distinct profiles (Architecture).
* U2 route-family contract underspecified — explicit `/meet` and
  `/l/meetup-join` policies plus fallback for other accepted paths (Architecture).
* Profiles are secret stores — organizer and bot user-data dirs out of git,
  never copied into `docs/`/tests (Security Lens).

### P3 findings (advisory)

* Add full quality gates (`go vet`, `golangci-lint`, `gofmt -l`, `go test`) to
  each code unit's Verify.
* Mark U1 explicitly as a Ship-pipeline diagnostic task, not a Stage spike
  worktree, to avoid a P-010 misroute.
* Add a bounded live-cycle stop/checkpoint to U3 to preserve the 2-hour rule.
* Do not mutate the shared `bypassParams` map when branching; select the param
  set via branch logic or a separate const set.
* U2 test matrix: add `/meet/<id>` without `p` and confirm legacy branch still
  sets `type=meetup-join`.
* After U6, compound-refresh BOTH distinct root causes — the agent-browser
  managed-launch DevToolsActivePort failure and the chromedp harness
  `google-chrome` PATH-discovery failure (remedied by `TEAMS_BROWSER_PATH`) —
  into `docs/compound/`.

**Disposition:** revise the plan (cycle 1) to resolve all P1 findings and fold
in the P2/P3 items, then re-run the review gate.

---

dispatch_mode: multi-agent
decision: FAIL

**Attempt 2 (re-entry cycle 1).** Re-dispatched after the cycle-1 revision. Personas:
Constitution Reviewer (subagent), Go Reviewer (subagent), Security Lens Reviewer
(cross-model, grok-4.6; triggered by sensitive-data/secret handling),
Architecture Strategist (cross-model, gpt-5.6-sol).

**Persona coverage / verdicts:** Constitution — **PASS**; Go — **PASS**; Security
Lens — **ADVISORY**; Architecture Strategist — **FAIL**.

**Gate: FAIL** — Architecture's two still-open P1 findings force a second revision
cycle (final allowed re-entry).

### P1 findings (must resolve — Architecture)

1. **U1 lacks an executable diagnostic seam.** The existing harness
   `meeting_integration_test.go` ALWAYS applies `MutateJoinURL`, hardcodes
   `Headless: true`, and passes an empty UA that resolves to `DefaultUserAgent`,
   so U1's "raw vs mutated / headed vs headless / fixed vs native UA" comparison
   cannot actually run. Fix: specify a build-tagged (`integration`) diagnostic
   probe (or env-driven harness controls) with a tri-state UA that can truly OMIT
   the override, serial execution, and clean profile state per matrix cell.
2. **Bot-identity inconsistency.** Canonical Topology permits an authenticated bot
   after U1, but U2 assumes a guest pre-join SPA, U6 says "empty bot profile," and
   the `meeting_integration_test.go` docstring says the profile is "already
   authenticated." Fix: make U1's selected identity mode an explicit downstream
   input; define U2's exit gate as the expected pre-join surface for that
   identity; align U6 terminology; bring the integration-test docstring into
   scope.

### P2 findings (fold in — Architecture)

* `/meet` param policy is evidence-defined but U1 only compares full-legacy-set
  vs no-added-set, so it cannot tell which individual params help; a pre-existing
  legacy param on a `/meet/` input is unspecified. Fix: per-param-set experiments
  in U1 + a conflict-case assertion in U2.

### Advisory fold-ins (Security Lens ADVISORY, Go, Constitution)

* Security Lens P3: stale "Risks and Caveats" sentence still implied organizer
  OAuth/MFA "already handled" covers the bot — rewrite so bot-side login/MFA is
  an explicit U1/U6 checkpoint; ensure "Unresolved Operator Decisions" does not
  read as authorizing an in-run CDP-9223 attach.
* Go P2: probe-first fix for `activateCaptions` should use a bounded per-step
  `WaitVisible` (short timeout), not an instantaneous `existsJS` probe (menu
  items render only AFTER the prior Click; an instant probe false-negatives).
* Go P3: make the legacy `/l/meetup-join` "byte-identical" claim verifiable via a
  golden/equality assertion.
* Constitution P3: Constitution Check omitted Principles V/VIII/XI mapping; add
  U4/U5 bounded-cycle stop (symmetry with U3); note U3 conditional test-file
  scope brushes width-isolation.

**Disposition:** revise the plan (cycle 2 — final re-entry) to resolve
Architecture's two P1s and P2 plus the advisory fold-ins, then re-run the review
gate (attempt 3, final).

<!-- plan-review-attempt: 2 -->

---

dispatch_mode: multi-agent
decision: ADVISORY

**Attempt 3 (re-entry cycle 2 — final).** Re-dispatched after the cycle-2
revision to the two personas that carried the still-open findings: Architecture
Strategist (cross-model, gpt-5.6-sol; the attempt-2 blocker) and Security Lens
Reviewer (cross-model, grok-4.6; sensitive-data trigger). Constitution and Go
had already returned PASS on attempt 2 with no re-opened scope.

**Persona coverage / verdicts:** Architecture Strategist — **ADVISORY**
(both attempt-2 P1s verified RESOLVED; one residual P2); Security Lens —
**PASS** (residual ADVISORY item verified RESOLVED; no new findings).

**Gate: ADVISORY** — no P0/P1 findings remain open, so the FAIL condition is
cleared. One residual P2 (advisory) is recorded below and was folded into U1.

### Verification of prior findings

* **P1-1 (executable diagnostic seam):** RESOLVED — U1 now specifies an
  `integration`-tagged diagnostic probe with raw/mutated URL, headed/headless,
  tri-state UA (fixed/newer/native-omitted), serial execution, and isolated
  clean profiles per matrix cell.
* **P1-2 (bot-identity consistency):** RESOLVED — Canonical Topology makes U1's
  selected identity mode an explicit downstream input; U2's exit gate is defined
  as the expected pre-join surface for that identity; U6 aligns the runbook
  wording and brings the `meeting_integration_test.go` docstring into scope.
* **Security residual (bot-side MFA / CDP-attach wording):** RESOLVED — Risks and
  Caveats separates organizer OAuth/MFA (out of bot scope) from bot-side
  login/MFA (explicit, never-automated U1/U6 checkpoint); Unresolved Operator
  Decisions states CDP-9223 attach is explicitly out of scope and not an in-run
  decision.

### Residual P2 (advisory — folded in, non-blocking)

* Architecture: U1's param experiment should isolate each individual legacy
  bypass param, not just `suppressPrompt`. **Folded in** — U1 now specifies a
  single-parameter ablation across all five legacy params plus a
  none/full-set baseline, with a documented trim-and-record fallback under
  run-budget pressure.

### Cycle-2 changes confirmed introducing no new secret-leak vector (Security Lens)

* U1 build-tagged probe redaction is NON-NEGOTIABLE (selector inventory +
  redacted landing class only; no live URLs/tokens/HTML/HAR/cookies/profile
  paths). U2 conflict-case uses synthetic fixtures only. U6 docstring fix is
  comment-only; secrets discipline still bans CDP-9223 attach and committing
  screenshots/HAR/cookies/profile paths.

**Disposition:** Gate ADVISORY with all P0/P1 cleared and the residual P2 folded
in. Per the operator directive to autonomously produce a queue-ready shipment,
the explicit authorization to proceed past ADVISORY is recorded here; proceed to
harvest (Step 5) and shipment assembly (Step 5.5).

<!-- plan-review-attempt: 3 -->

---

### Post-gate live-evidence incorporation (2026-09-05, after harvest/shipment)

decision: ADVISORY (unchanged — no new review cycle required)

After the gate closed and shipment `010-S` was assembled, the operator supplied
concrete live DOM evidence from a real authenticated meeting. It was folded into
the plan and the queued backlog tasks. This is a **de-risking sharpening, not a
scope expansion**, so it does not re-open the review gate:

* The evidence **confirms** (does not add to) the existing U1/U3/U4 scope: it
  pins the pre-join root cause to the `/dl/launcher/launcher.html` `joinOnWeb`
  interstitial (already within U3's "pre-join FSM" scope), confirms **guest**
  identity (collapsing the previously open guest-vs-authenticated branch to the
  confirmed case), and replaces hypothesized selectors with measured ones.
* Net effect on risk: **lower** — uncertainty and live-cycle count drop because
  exact selectors and the failure cause are now known.
* New authoritative artifact: the **Live Selector Evidence (2026-09-05)** section
  (selector oracle). U1 narrowed to the still-open URL-mutation question + caption
  variant; U3 gained the launcher step + `#hangup-button` + `checked`-property
  mute fix; U4 gained the exact caption selectors + guest/full two-variant
  activation; U6 records the confirmed inventory. No new unit, no changed DAG, no
  new secret-exposure surface (selectors only; private URL/token still excluded).
* Backlog tasks `010.003-T` (U3) and `010.004-T` (U4) descriptions/acceptance
  criteria were updated in place to match; `010.001-T` (U1) and `010.002-T` (U2)
  annotated with the narrowed scope. Sizes unchanged (still within the 2-hour
  rule: U3 = 2 files / ~3 functions / 2 test scenarios).

Because the change only removes uncertainty and adds a concrete selector oracle
already inside the reviewed unit boundaries, the ADVISORY gate verdict stands and
no fourth review cycle is required.

<!-- plan-review-attempt: 3 (post-gate evidence fold-in, no re-gate) -->

---

### Post-gate scope addition — U7 harness executable-path enabler (2026-09-05T17:06)

decision: ADVISORY (unchanged gate; new unit independently scope-audited)

A further live cycle on an Edge-only Windows host surfaced a **harness-runnability
gap** (not a selector gap): chromedp browser auto-discovery aborts with
`exec: "google-chrome": executable file not found in %PATH%`, and the only way the
live harness ran was an ad-hoc PATH prepend of the Chrome-for-Testing directory.
The operator asked whether the scoped work should add an explicit
harness-executable-path option, "kept narrowly tied to making the live harness
runnable."

Unlike the prior post-gate fold-in (pure sharpening within existing unit
boundaries), this **adds a new unit (U7)**, so it was NOT treated as silent
sharpening. It was routed through an **independent Scope Boundary Auditor** pass
before touching the queued shipment:

* **Verdict: ADVISORY (lean PASS)** on adding U7 as scoped. Rationale: U7 is in
  the same domain as U1 (test-infra) and serves the feature's own "verification"
  half — it removes an un-versioned PATH trick otherwise required to run the very
  harness U1/U3/U4/U5 depend on. It is a single-file (~5-line) pure-superset
  change (unset var → byte-identical behavior), passing width-isolation and the
  2-hour rule. No new attack surface (the var is consumed only under the
  `integration` build tag + `GO_TEAMS_INTEGRATION=1` by an operator who can
  already run arbitrary `go test` code; passed as an exec-path arg, not
  shell-interpolated).
* **Must-fix advisories, all applied:** (1) keep U7 **test-only** — the wider
  production `Options.ExecPath` form is rejected for 010-S; (2) the auditor
  confirmed the production runtime (`runner.go:56`, `meeting.go:55`) shares the
  identical broken allocator call, so the deployed bot is *also* affected — that
  **production Edge-only-host portability gap is filed as a separate follow-up
  stash entry** under its own future feature, not bundled into 010-F; (3) U7 is
  recorded in this plan's units, Dependency Graph (sequenced ahead of U1), Risks,
  and Decisions rather than as an undocumented queue mutation; (4)
  `TEAMS_BROWSER_PATH` is bound by Protected Invariant 5 (never echoed/committed
  into redacted `docs/` evidence).

**Also folded in this pass (pure sharpening, no re-gate):** a confirmation run
reproduced the launcher/selector findings end-to-end (see the Confirmation-run
note in Live Selector Evidence); and live audio evidence confirmed the capture
**prerequisites** (`getDisplayMedia` + `AudioContext`) are present, sharpening
U5's exit to separate "prereqs present" (not a pass) from verified end-to-end
streaming.

**Disposition:** the ADVISORY gate verdict stands. U7 was added as a bounded,
independently-audited test-infra enabler with all auditor advisories applied;
harvested as task `010.007-T` and added to shipment `010-S` sequenced ahead of
U1 (dependency edge `010.001-T` → `010.007-T`). The production-runtime
portability gap is deferred to a new stash entry.

<!-- plan-review-attempt: 3 (post-gate U7 scope addition, scope-audited ADVISORY, no full re-gate) -->

---

### Residual Review Items (non-blocking P2/P3 — intentionally unresolved)

decision: ADVISORY (unchanged). These are recorded as explicit residual-risk
notes per P-021 so their non-resolution is a **documented, deliberate** decision,
not an oversight. None blocks shipment `010-S`; each is either out of the
`010-F` selector/path-hardening + verification contract surface or a pre-existing
condition not introduced by this work. **No implementation is performed here.**

* **R1 — Production Edge-only / non-PATH host browser-path portability (deferred,
  captured).** `ChromeRunner.run` (`internal/browser/runner.go`) and
  `NewLiveMeeting` (`internal/browser/meeting.go`) build the chromedp allocator
  with no exec-path option, so the deployed bot aborts on hosts without a
  Chrome/Chromium binary on `PATH` — the runtime twin of the harness gap U7
  fixes for tests only. **Disposition:** out of `010-F` scope (runtime deployment
  portability is a distinct feature); captured as a full P-021 C2 DEFERRED SCOPE
  EXPANSION in stash `87CDB97A` (`requires deliberation: YES`). Non-blocking: U7
  unblocks live verification without it. Signature note: the production abort is
  the same chromedp PATH-discovery signature (`exec: "google-chrome"… not found in
  %PATH%`), **distinct** from the managed agent-browser `DevToolsActivePort`
  launch failure — the two must not be conflated.
* **R2 — `--no-sandbox` allocator flag (pre-existing).** The allocator disables
  the Chrome sandbox. **Disposition:** pre-existing hardening concern, not
  introduced or widened by this feature and outside the selector/path-hardening
  contract surface; left intentionally unresolved. Not captured as a new deferred
  entry (not directly caused by this change; revisit under a dedicated
  browser-hardening effort).
* **R3 — Runner/error URL logging (pre-existing; secret-adjacent).** Existing
  logging in the runner/error paths could surface a join URL/token if invoked with
  a live URL. **Disposition:** out of scope for `010-F` (no logging code changes
  here); this plan's redaction rules already forbid committing any live URL/token
  into artifacts, and U1/U6 scrub `t.Log`/chromedp error output. Flagged as a
  residual **secret-hygiene** risk for a future logging-redaction pass; not
  implemented in this shipment.
* **R4 — Navigation host allowlist (pre-existing).** No allowlist constrains the
  URLs the automation will navigate. **Disposition:** pre-existing; out of the
  selector/path-hardening contract surface; intentionally unresolved. Not captured
  as a new deferred entry.
* **R5 — `.env.local` handling (pre-existing).** Reviewer-noted local-env file
  handling. **Disposition:** pre-existing and out of scope; not acted on and not
  captured as a deferred entry (not directly relevant to the selector/path work
  under P-021 C1).

Rationale for not opening new deferred entries beyond `87CDB97A`: under P-021 C1
only findings on the **authorized contract surface** are in scope, and only
**directly relevant** unaddressed findings warrant a new deferred capture. R2–R5
are pre-existing conditions outside this feature's surface; R1 is directly
relevant and is the one item captured (`87CDB97A`).

### Post-gate local-review remediation (2026-09-06)

decision: ADVISORY (unchanged — a doc/backlog/plan-artifact remediation pass; no
source edits, no re-gate). A local review of branch `chore/stage-010-S` surfaced
precision/consistency findings against the planning and backlog artifacts (not
against source). All corrections were applied to **artifacts only** (plan,
backlog items, shipment manifest, memory/closure records, stash) with **no
production/test source change, no build, no shipment claim, and no PR action**
(P-010 boundary preserved). Notable changes: Problem Frame marks the launcher
root cause **confirmed**; Requirements Trace and Constitution Check now cover
**U1–U7** with corrected principle citations; the malformed `joinOnWebSel` inline
code is fixed; U1 is narrowed to the still-open URL-mutation/caption-variant
diagnostics **behind a dedicated opt-in** so routine `-tags integration` runs do
not launch the matrix; U7 requires **tagged compile verification**; U4 prefers a
**single shared selector const interpolated into the JS** over duplicated
literals; U5 acceptance now requires a **stimulus-verified** capture that
distinguishes Chrome fake-media from real meeting/tab audio; and the mic-state
read is constrained to the boolean **`checked` property only** (no `aria-checked`,
no `data-cid`/title as state). Redaction rules unchanged and re-verified: no live
URL, `p=` token, account/UPN, calendar, cookie, or profile/session-state path
appears in any artifact. The ADVISORY gate verdict stands.

<!-- plan-review-attempt: 3 (local-review remediation of artifacts only, no source change, no re-gate) -->
