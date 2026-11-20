---
title: "Wave 1B Implementation Plan — 010-S Sibling Labels and Stale Specification Reconciliation"
description: "Operator-authorized Stage plan repairing the eight missing harness-surface labels and reconciling post-011-S..035-S stale specifications across 010.001-T..010.007-T and 010.009-T in queued shipment 010-S"
doc_type: "plan"
source: "docs/decisions/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-deliberation.md"
plan_id: "2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation"
revision: 2
deliberation_id: "026-DL"
feature: "010-F"
shipment: "010-S"
date: "2026-11-20"
status: "draft"
tags:
  - "stage-reconciliation"
  - "010-S"
  - "p-002"
  - "p-004"
  - "browser-verification"
review_manifest: docs/reviews/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-plan-review.md
source_history:
  - "review attempt 1: FAIL (plan revision 1) -\u003e docs/reviews/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-plan-review.md"
  - "review attempt 2: PASS (plan revision 2) -\u003e docs/reviews/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-plan-review.md"
---

## Problem Frame

Following explicit operator authorization ("I Authorize Stage to repair the
eight missing harness-surface labels in 010-S and reconcile its stale
requirements without expanding scope") and accepted deliberation
[2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-deliberation.md](../decisions/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-deliberation.md)
(`026-DL`), this plan specifies the exact in-place Stage reconciliation for the
eight remaining task members of queued shipment `010-S` (`010.007-T`,
`010.001-T`, `010.002-T`, `010.003-T`, `010.004-T`, `010.005-T`, `010.006-T`,
and `010.009-T`) while preserving `010.008-T` and `025-DL` from Wave 1A and
leaving active stash `AD451E00` and dirty `.backlogit/stash.jsonl` untouched.

### Explicit Scope Boundaries

* **In scope**:
  * Creating deliberation `026-DL` via `backlogit_create_item`
    (`artifact_type: "deliberation"`).
  * Repairing `harness-surface:*` labels and reconciling stale `2026-09-05`
    descriptions, file budgets, verification records, and acceptance criteria on
    `010.007-T`, `010.001-T`, `010.002-T`, `010.003-T`, `010.004-T`,
    `010.005-T`, `010.006-T`, and `010.009-T` via registered
    `backlogit_update_item` (replacing the full `description` payload so zero
    stale untagged preamble survives above `<!-- BEGIN:description -->`),
    `backlogit_add_link` (`informs`), and `backlogit_append_comment` calls.
* **Out of scope**:
  * Modifying `010-S.md`, `010.008-T`, `025-DL`, or the frozen Wave 1A
    plan/review artifacts.
  * Modifying `.backlogit/stash.jsonl` (`AD451E00` or uncommitted `47D1E1C0`),
    changing any item status, claiming `010-S`, changing `010-S` membership or
    DAG edges, creating a new shipment, editing any Go source/test/config file,
    or running git branch/stage/commit/push/PR operations.

## Requirements Trace

| Requirement | Decision Ref | Reconciled Backlog Target(s) | Verification |
|---|---|---|---|
| Repair all 8 missing `harness-surface:*` labels from verified post-`035-S` file budgets so `010-S` resolves cleanly | `D1`, `D2`, `AC1` | `010.001-T`..`010.007-T`, `010.009-T` | `gsd harness resolve --workspace . --shipment 010-S --json` returns `state: "HARNESS_READY"`, `exit_code: 0` |
| Reconcile `010.007-T` onto `newMeeting(t)` + shipped `Options.ExecPath` + `030-S`/`035-S` external-profile-compatible `TEAMS_PROFILE_DIR` containment | `D3` | `U7` (`010.007-T`) | `backlogit get 010.007-T` confirms `meeting_integration_test.go` test-only budget, `harness-surface:none`, repo-root/ancestor + repo-local `git check-ignore` rules |
| Reconcile `010.001-T`..`010.004-T` to evidence-backed verification + remaining test assertions without regressing `011-S`..`035-S` production code | `D1`, `D5` | `U1`..`U4` (`010.001-T`..`010.004-T`) | `backlogit get` confirms non-production file budgets (`harness-surface:none`), preserved multi-selector constants, and explicit test gates |
| Reconcile `010.005-T` with non-tagged `audio_test.go` in file budget + `RecapMeeting` preservation for `P-004` `harness-architect` execution | `D2`, `D4`, `D5` | `U5` (`010.005-T`) | `backlogit get 010.005-T` confirms `harness-surface:harness-architect` and 4-file budget (`audio.go`, `meeting_chrome.go`, `audio_test.go`, `meeting_integration_test.go`) |
| Qualify `010.006-T` and `010.009-T` for `P-002` docs-only route with explicit `.md` paths, `markdownlint` executable gates, and archived PR `#27` stash status | `D2`, `D5`, `AC4` | `U6` (`010.006-T`), `U9` (`010.009-T`) | `backlogit get` confirms explicit normalized `docs/.../*.md` owned paths, `harness-surface:none`, and separate `Executable Gates` / `Manual Gates` |

## Implementation Units

### U7 (`010.007-T`, DAG root, reconciled in place) — Verify `TEAMS_BROWSER_PATH` wiring and add `TEAMS_PROFILE_DIR` containment validation in `meeting_integration_test.go`

* **Domain:** test infrastructure (`internal/browser/meeting_integration_test.go` only).
* **Posture:** non-docs `harness-surface:none` test-only task (skips T2/T3
  `harness-architect`, no route label, verified inline per `_ship.agent.md` Step
  2 / Step 4.2 on `HEAD` and Step 4.0 / Step 4.2 in the pending harness
  upgrade).
* **Size / Complexity / Label:** `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:none"]`.
* **File Budget (1 file):** `internal/browser/meeting_integration_test.go`.
* **Shipped vs. Remaining Reconciliation:**
  * *Already shipped on `HEAD` (verify, do NOT reimplement or revert)*:
    `browser.Options.ExecPath` and `ResolveExecPath` (`internal/browser/allocator.go:25,83-118`,
    `011-S`/`013-S`), `.env.local.test` `%VAR%`/`$VAR`/`~` + bare browser
    resolution (`internal/browser/integration_helper_test.go`, `028-S`/`030-S`),
    and `newMeeting(t)` replacing removed `browserContext(t)` and passing
    `ExecPath: os.Getenv("TEAMS_BROWSER_PATH")` into `browser.NewLiveMeeting`
    (`internal/browser/meeting_integration_test.go:32-57`, `028-S`/`030-S`).
  * *Remaining deliverable in `internal/browser/meeting_integration_test.go`*:
    1. Add a fail-closed `TEAMS_PROFILE_DIR` containment validator helper
       (matching `scheduled_lifecycle_integration_test.go:41-81` from `030-S` /
       `035-S`, while sanitizing `os.Lstat` and `filepath.EvalSymlinks` errors to
       symbolic messages so raw `*fs.PathError` / `*os.LinkError` paths are
       never echoed) and invoke it in `newMeeting(t)` (after `strings.TrimSpace`
       and line `37`'s `t.Skip` guard, passing `realProfile` to `UserDataDir`)
       while **preserving `func newMeeting(t *testing.T)`'s signature, line `37`
       `t.Skip` message, and absence of a `// newMeeting ...` GoDoc comment** (so
       preserved `010.008-T`'s `saw_red` precondition stays intact): resolve
       `repoRoot` (`go.mod` walk + `filepath.EvalSymlinks`) and `realProfile`
       (resolving nearest existing ancestor symlinks + trailing non-existent
       segments); reject when `realProfile` equals `realRoot` (`.`) or is an
       ancestor containing `realRoot`; when `realProfile` is inside `realRoot`
       (`filepath.IsLocal(relProfile)`), require `git check-ignore -q --
       <slashRelProfile>` to succeed; and permit valid external profile
       directories outside `realRoot` (preserving out-of-band operator `%APPDATA%`
       / `%LOCALAPPDATA%` support from `030-S`/`035-S`, while CLI-agent browser
       runs keep isolated profiles inside git-ignored workspace paths).
    2. Add `integration`-tagged unit tests (`TestValidateLiveMeetingProfileDir`)
       in `meeting_integration_test.go` parameterizing `repoRoot` onto a
       dedicated `t.TempDir()` git fixture (so a sibling `t.TempDir()` stays
       deterministically external even when `TEMP` is redirected under
       `.proof-scratch/`), covering rejection of repo-root, repo-ancestor,
       unignored repo-local, and broken-symlink (non-echoing) profile paths,
       acceptance of a git-ignored repo-local profile and an external profile,
       and an option-builder / `browser.ResolveExecPath` check confirming
       non-empty `TEAMS_BROWSER_PATH` propagation without launching a live
       meeting.
  * *Inline Verification Commands*: `gofmt -l internal/browser/meeting_integration_test.go`
    (empty), `go vet ./...`, `go vet "-tags=integration" ./internal/browser/...`,
    `golangci-lint run`, `golangci-lint run "--build-tags=integration"
    ./internal/browser/...`, `go test -race ./...`, and `go test -race
    "-tags=integration" "-run=TestValidateLiveMeetingProfileDir"
    ./internal/browser/...` all exit `0`.
* **Dependencies:** none (DAG root).

### U1 (`010.001-T`, reconciled in place) — Add `GO_TEAMS_DIAGNOSTIC_MATRIX` build-tagged probe and record `/meet/` + caption-variant findings

* **Domain:** diagnostic test infrastructure + redacted memory note.
* **Posture:** non-docs `harness-surface:none` diagnostic task (skips T2/T3
  `harness-architect`, no route label, verified inline).
* **Size / Complexity / Label:** `Size: M` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:none"]`.
* **File Budget (2 concrete files):** `internal/browser/meeting_diagnostic_test.go`,
  `docs/memory/2026-11-20/u1-live-diagnostic-findings.md`.
* **Shipped vs. Remaining Reconciliation:**
  * *Already shipped on `HEAD`*: `/meet/` `q.Del("type")` in
    `internal/teams/url.go`, `#openTeamsClientInBrowser` + `joinOnWeb` in
    `internal/browser/prejoin.go`, bounded two-variant `activateCaptions` in
    `internal/browser/captions.go` / `meeting_chrome.go`, and `030-S` live-tenant
    join/caption proof.
  * *Remaining deliverable*:
    1. Create `internal/browser/meeting_diagnostic_test.go` in external `package
       browser_test` carrying **only** `//go:build integration` (so routine
       `-tags=integration` always compiles it and accesses `010.007-T`'s
       profile-containment helper) and guarded at runtime by
       `GO_TEAMS_DIAGNOSTIC_MATRIX=1` (`t.Skip` when unset/false), plus a fast
       tagged unit test verifying the skip gate and bounded cell-plan cap
       (`<= 8` launches / `<= 3` cycles). For cold isolated guest per-cell
       probes, drive Chromium directly via `browser.AllocatorOptions` +
       `chromedp.NewExecAllocator` + `browser.NewMeeting` (so cold guest cells do
       not hit `NewLiveMeeting`'s `managedProfile` cold-profile
       `browser.ErrAccountRequired` guard without modifying production code,
       while `NewLiveMeeting` cells require an authenticated profile or
       `TEAMS_ACCOUNT`).
    2. Enforce strict redaction in both `meeting_diagnostic_test.go` and
       `docs/memory/2026-11-20/u1-live-diagnostic-findings.md`: strip any
       captured URL via `teams.RedactURL` and persist only normalized route
       classes (`/meet/*` vs. `/l/meetup-join/*` — never raw meeting IDs, `?p=`
       tokens, Entra UPNs, HTML/HAR/screenshots, cookies, `localStorage`, or
       profile paths), and record any unexpected Microsoft login/MFA landing as
       `BLOCKED` with an operator checkpoint (never automating credentials).
       Per `2026-09-05` Plan Hardening `Blocked-path handling`, execute live
       screening when `.env.local.test` provides a live meeting and organizer
       checkpoint, or record truthful `BLOCKED` live-checkpoint evidence (never
       a false live pass) when AFK without an active meeting.
  * *Inline Verification Commands*: `gofmt -l internal/browser/meeting_diagnostic_test.go`
    (empty), `go vet ./...`, `go vet "-tags=integration" ./internal/browser/...`,
    `golangci-lint run`, `golangci-lint run "--build-tags=integration"
    ./internal/browser/...`, `go test -race ./...`, `go test -race
    "-tags=integration" "-run=TestDiagnosticMatrix" ./internal/browser/...`,
    `markdownlint --disable MD013 "--"
    docs/memory/2026-11-20/u1-live-diagnostic-findings.md`, and `gsd gate
    line-endings --path docs/memory/2026-11-20/u1-live-diagnostic-findings.md
    --json` all exit `0`.
* **Depends on:** `010.007-T` (`blocks`).

### U2 (`010.002-T`, reconciled in place) — Verify shipped `/meet/` URL mutation policy and add legacy golden equality + fallback tests

* **Domain:** unit test verification (`internal/teams/url_test.go` only).
* **Posture:** non-docs `harness-surface:none` test-only verification task
  (evidence-backed verification of shipped `internal/teams/url.go`; skips T2/T3
  `harness-architect`, verified inline).
* **Size / Complexity / Label:** `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:none"]`.
* **File Budget (1 file):** `internal/teams/url_test.go`.
* **Shipped vs. Remaining Reconciliation:**
  * *Already shipped on `HEAD` (verify, do NOT reimplement)*: `MutateJoinURL` in
    `internal/teams/url.go:101-123` (`isLegacyMeetupJoinPath` preserving
    `bypassParams` without mutating the package-level map, and `q.Del("type")`
    on `/meet/` and other non-legacy paths) plus the 4 `/meet/` table cases in
    `internal/teams/url_test.go:47-96`.
  * *Remaining deliverable in `internal/teams/url_test.go`*: Add (1) a full
    mutated-string golden equality assertion on `/l/meetup-join` output proving
    byte-identical legacy output, (2) table cases for non-`/meet/` non-legacy
    accepted paths (such as `/l/meetingrecap?threadId=synthetic&type=meetup-join`
    stripping `type` while preserving `threadId` and fragment), and (3) an
    immutability check confirming `MutateJoinURL` never mutates `bypassParams`,
    using synthetic tokens only.
  * *Inline Verification Commands*: `gofmt -l internal/teams/url_test.go`
    (empty), `go vet ./...`, `golangci-lint run`, and `go test -race
    ./internal/teams/...` exit `0`.
* **Depends on:** `010.001-T` (`blocks`).

### U3 (`010.003-T`, reconciled in place) — Verify shipped launcher, `#hangup-button`, and boolean `checked` mute selectors with unit and Chromium E2E tests

* **Domain:** browser test verification (`internal/browser/prejoin_test.go`,
  `internal/browser/meeting_e2e_test.go`).
* **Posture:** non-docs `harness-surface:none` test-only verification task
  (skips T2/T3 `harness-architect`, verified inline).
* **Size / Complexity / Label:** `Size: M` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:none"]`.
* **File Budget (2 files):** `internal/browser/prejoin_test.go`,
  `internal/browser/meeting_e2e_test.go`.
* **Shipped vs. Remaining Reconciliation:**
  * *Already shipped on `HEAD` (preserve intact — MUST NOT strip fallbacks)*:
    `joinOnWebSel` (`#openTeamsClientInBrowser, button[data-tid="joinOnWeb"],
    [data-tid="joinOnWeb"]`), `meetingToolbarSel` (`#hangup-button,
    [data-tid="hangup-button"], [data-tid="hangup-main-btn"]`),
    `propBoolJS(micToggleSel, "checked")` + `micStateRequiresMute(checked,
    ariaChecked)`, launcher click handling in `joinHeadedWithDeadline` and
    `awaitPreJoinOrAdmissionWithDeadline`, and `2026-09-05` provenance comments
    in `prejoin.go` (`0` `VERIFY LIVE` markers remain).
  * *Remaining deliverable*: Add targeted unit tests in `prejoin_test.go`
    (pinning `joinOnWebSel`, `meetingToolbarSel`, `propBoolJS`, and absence of
    `VERIFY LIVE` markers in `prejoin.go`) and real-Chromium loopback `httptest`
    E2E coverage in `meeting_e2e_test.go` verifying `#openTeamsClientInBrowser`
    launcher click, guest `#hangup-button` admission (with no `data-tid`), and
    boolean `checked` mute-state handling when `aria-checked` is absent (both
    `checked=true` -> muted and `checked=false` -> left unclicked); also execute
    `TestMeetingJoin` under `-tags=integration` when a live meeting is configured
    (or record `BLOCKED` checkpoint status if AFK).
  * *Inline Verification Commands*: `gofmt -l internal/browser/prejoin_test.go
    internal/browser/meeting_e2e_test.go` (empty), `go vet ./...`,
    `golangci-lint run`, and `go test -race ./internal/browser/...` exit `0`.
* **Depends on:** `010.001-T`, `010.002-T` (`blocks`).

### U4 (`010.004-T`, reconciled in place) — Enforce non-empty speaker/text in `TestMeetingEnableCaptions` and add guest-direct caption E2E + selector-sync tests

* **Domain:** browser test verification (`internal/browser/captions_test.go`,
  `internal/browser/meeting_e2e_test.go`,
  `internal/browser/meeting_integration_test.go`).
* **Posture:** non-docs `harness-surface:none` test-only task (skips T2/T3
  `harness-architect`, verified inline).
* **Size / Complexity / Label:** `Size: M` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:none"]`.
* **File Budget (3 files):** `internal/browser/captions_test.go`,
  `internal/browser/meeting_e2e_test.go`,
  `internal/browser/meeting_integration_test.go`.
* **Shipped vs. Remaining Reconciliation:**
  * *Already shipped on `HEAD` (preserve intact — MUST NOT strip fallbacks)*:
    `moreActionsSel`, `languageSpeechSel`, `turnOnCaptionsSel`,
    `captionWrapperSel`, inner `captionObserverJS` literals, `2026-09-05`
    provenance comments (`0` `VERIFY LIVE` markers remain in `captions.go`), and
    bounded `5s` two-variant `activateCaptions` in `meeting_chrome.go`.
  * *Remaining deliverable*:
    1. Update `TestMeetingEnableCaptions` in
       `internal/browser/meeting_integration_test.go` (`lines 71-102`) so its
       `CaptionSink` callback validates non-empty `strings.TrimSpace(speaker)`
       AND non-empty `strings.TrimSpace(text)` (failing if only empty/stale
       captions arrive rather than discarding `func(_, _ string)`).
    2. Add a selector-sync unit test in `internal/browser/captions_test.go`
       asserting `captionObserverJS` embeds `captionWrapperSel` verbatim and
       `captions.go` contains zero `VERIFY LIVE` comments.
    3. Add a real-Chromium loopback `httptest` E2E test in
       `internal/browser/meeting_e2e_test.go` exercising the **guest-direct
       `#closed-captions-button`** menu variant (where
       `#LanguageSpeechMenuControl-id` is absent) and verifying finalized
       non-empty speaker + text delivery.
    4. Execute opt-in targeted live `TestMeetingEnableCaptions`
       (`GO_TEAMS_INTEGRATION=1` `go test -json "-tags=integration"
       "-run=^TestMeetingEnableCaptions$" ./internal/browser/...`) when an
       authorized live Teams meeting, lobby admission, and live speech stimulus
       setup are available: because `go test` exits `0` on `t.Skip`, require a
       test-level event for `"Test":"TestMeetingEnableCaptions"` with
       `"Action":"pass"` (not `"Action":"skip"` and not package aggregate exit
       `0` alone); record `"Action":"skip"` or unavailable prerequisites as
       explicit redacted `BLOCKED` validator evidence naming the exact missing
       prerequisite, while any executed `"Action":"fail"` remains a failure, and
       keep synthetic E2E and tagged no-run compile verification distinct.
  * *Inline Verification Commands*: Always-on synthetic + tagged-compile gates
    (`gofmt -l internal/browser/captions_test.go
    internal/browser/meeting_e2e_test.go
    internal/browser/meeting_integration_test.go` empty, `go vet ./...`, `go vet
    "-tags=integration" ./internal/browser/...`, `golangci-lint run`,
    `golangci-lint run "--build-tags=integration" ./internal/browser/...`, `go
    test -race ./internal/browser/...`, and `go test "-tags=integration"
    "-run=^$" ./internal/browser/...`) all exit `0`; plus opt-in live `go test
    -json "-tags=integration" "-run=^TestMeetingEnableCaptions$"
    ./internal/browser/...` requiring named test `"Action":"pass"` when live
    meeting/speech setup is available (`"Action":"skip"` / unavailable
    prerequisites recorded as redacted `BLOCKED`; executed `"Action":"fail"`
    stays failure).
* **Depends on:** `010.003-T` (`blocks`).

### U5 (`010.005-T`, reconciled in place) — Add PCM-before-encode observer tap, Goertzel stimulus check, and conditionally resolve `audio.go` `VERIFY LIVE`

* **Domain:** production Go + unit/integration tests (`internal/browser/`).
* **Posture:** `test-first` / `characterization-first` on the `P-002`/`P-004`
  **code route with `harness-surface:harness-architect`** (`harness-architect`
  scaffolds pure-Go, non-skipping RED-phase unit roster tests in non-tagged
  `audio_test.go` against production stubs in `audio.go` / `meeting_chrome.go` so
  canonical `go test ./...` observes RED before implementation).
* **Size / Complexity / Label:** `Size: M` (`ah-stage-sizing-v1`) | `Complexity: medium` | `labels: ["harness-surface:harness-architect"]`.
* **File Budget (4 tightly coupled files in `internal/browser`):**
  `internal/browser/audio.go`, `internal/browser/meeting_chrome.go`,
  `internal/browser/audio_test.go`,
  `internal/browser/meeting_integration_test.go`.
* **Shipped vs. Remaining Reconciliation:**
  * *Genuinely unimplemented on `HEAD`*:
    1. Declare an exported instance-scoped optional interface in
       `internal/browser/audio.go` (for example `type AudioPCMObserver interface
       { SetAudioPCMObserver(func(samples []int16)) }`, mirroring
       `ActivityMonitor` / `RecapMeeting` / `AISummaryProvider` with zero
       package-global mutable state and zero edits to `internal/browser/meeting.go`)
       and implement it on `*chromeMeeting` in
       `internal/browser/meeting_chrome.go` synchronized via `m.activityMu`
       (`sync.Mutex`), wired inside `chromeMeeting.CaptureAudio`
       (`internal/browser/meeting_chrome.go:1510-1608`, live PCM `newPayloadPump`
       callback at lines `1547-1565`) so that when the observer callback is
       non-nil it receives `slices.Clone(samples)` synchronously before
       `enc.WritePCM(samples)` without mutating encoder buffers or perturbing
       `shine-mp3` `1152`-sample frame alignment / `encoderTailSlack` `-race`
       `checkptr` safety.
    2. Add a deterministic single-frequency Goertzel / tone-discrimination helper
       in `internal/browser/audio.go`, backed by pure-Go (never `t.Skip`ped)
       unit tests in `internal/browser/audio_test.go` (distinguishing known
       stimulus tone PCM from silence, noise, and uncorrelated synthetic
       frequencies, plus testing thread-safe `SetAudioPCMObserver` delivery with
       `slices.Clone` isolation). Remove/resolve the `// VERIFY LIVE` warning in
       `internal/browser/audio.go:53-55` **ONLY AFTER** `GO_TEAMS_INTEGRATION=1`
       `go test -json "-tags=integration" "-run=^TestMeetingCaptureAudio$"
       ./internal/browser/...` executes the non-`RecapMeeting` live-call path
       (never a `RecapMeeting` early return or `t.Skip`) and emits test-level
       `"Test":"TestMeetingCaptureAudio"` `"Action":"pass"` confirming actual
       `getDisplayMedia` auto-approval / `--auto-select-desktop-capture-source`
       capture + Goertzel stimulus detection; if `"Action":"skip"` occurs or
       live prerequisites are unavailable, retain the unresolved `// VERIFY LIVE`
       note in `internal/browser/audio.go` and record explicit redacted `BLOCKED`
       validator evidence naming the exact missing prerequisite, while any
       executed `"Action":"fail"` remains a failure.
    3. Wire the `AudioPCMObserver` type assertion + stimulus check into the
       non-`RecapMeeting` live-call path of `TestMeetingCaptureAudio` in
       `internal/browser/meeting_integration_test.go` (`lines 108-147`) **while
       preserving the `030-S`/`031-S` `RecapMeeting` branch (`lines 119-130`)
       intact**, using `-json` test-level `"Action":"pass"` on the non-`RecapMeeting`
       stimulus path as the live oracle (`t.Skip` / unavailable prerequisites ->
       redacted `BLOCKED`; executed `"Action":"fail"` -> failure).
  * *Verification Commands*: Always-on synthetic + tagged-compile gates (`gofmt
    -l internal/browser/audio.go internal/browser/meeting_chrome.go
    internal/browser/audio_test.go internal/browser/meeting_integration_test.go`
    empty, `go vet ./...`, `go vet "-tags=integration" ./internal/browser/...`,
    `golangci-lint run`, `golangci-lint run "--build-tags=integration"
    ./internal/browser/...`, `go test -race ./internal/browser/...
    ./internal/audio/...`, `go test -race ./...`, and `go test
    "-tags=integration" "-run=^$" ./internal/browser/...`) all exit `0`
    independently of live availability; plus opt-in live `go test -json
    "-tags=integration" "-run=^TestMeetingCaptureAudio$" ./internal/browser/...`
    requiring named test `"Action":"pass"` on the non-`RecapMeeting` stimulus
    path when live setup is available (`"Action":"skip"` / unavailable
    prerequisites recorded as redacted `BLOCKED` with `audio.go` `VERIFY LIVE`
    retained; executed `"Action":"fail"` stays failure).
* **Depends on:** `010.003-T` (`blocks`).

### U6 (`010.006-T`, reconciled in place) — Write live-harness reproducibility runbook and closure evidence (`P-002` docs-only route)

* **Domain:** documentation (`docs/closure/browser-automation/`).
* **Posture:** `P-002` **docs-route candidate** (`harness-surface:none`;
  `harness-architect` applies `harness-verification-gated` at T2).
* **Size / Complexity / Label:** `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:none"]`.
* **Explicit Owned Path (1 regular prose `.md` file under `docs/`):**
  `docs/closure/browser-automation/010-s-live-selector-path-verification-runbook.md`.
* **Reconciled Content & `P-002` Verification Record:**
  * Cross-references the September parent blocked verification record and `029-S`
    scheduled-validation runbook without modifying any existing file, and
    documents `.env.local.test` (`browser.LoadTestEnv`), `TEAMS_BROWSER_PATH` +
    `ResolveExecPath`, `030-S`/`035-S`-compatible `TEAMS_PROFILE_DIR` rules
    (isolated git-ignored repo-local guest profiles + external `%APPDATA%` /
    `%LOCALAPPDATA%` authenticated profiles; repo-root and repo-ancestor
    rejected; plus the `NewLiveMeeting` / `Meeting.Join` cold-profile
    `browser.ErrAccountRequired` precondition on remote `https://` URLs matching
    `010.008-T`), redacted `2026-09-05` + `030-S` selector inventory, the two
    distinct browser launch failure signatures, PowerShell-quoted Go flags,
    per-flow observed Tier 2 E2E and Tier 5 live verdicts (`pass` / `blocked` /
    `partial`, never claiming live green without live evidence), and accurate
    `P-021` residual-risk status (noting PR `#27` items `87CDB97A`, `D2023898`,
    `EEA7F685`, `A99EEABD`, and `3084A83E` were delivered/archived in `013-S` /
    `002-DL`, while `AD451E00` remains active for its deferred bound-target
    hangup/CDP helper scope).
  * Embeds explicit `P-002` **Executable Gates** (`markdownlint --disable MD013
    "--"
    docs/closure/browser-automation/010-s-live-selector-path-verification-runbook.md`
    and `gsd gate line-endings --path
    docs/closure/browser-automation/010-s-live-selector-path-verification-runbook.md
    --json`, cwd `.`, exit `0` / `0` findings, `result: pending`) separate from
    4-field **Manual Gates** (`source_paths`, `target_paths`, exact `pass_fail`
    criteria, and `required_evidence`, without mixing non-owned filenames into
    the `Owned Paths` block).
* **Depends on:** `010.003-T`, `010.004-T`, `010.005-T` (`blocks`).

### U9 (`010.009-T`, reconciled in place) — Promote both live-harness root-cause learnings to `docs/compound/` (`P-002` docs-only route)

* **Domain:** documentation (`docs/compound/`).
* **Posture:** `P-002` **docs-route candidate** (`harness-surface:none`;
  `harness-architect` applies `harness-verification-gated` at T2).
* **Size / Complexity / Label:** `Size: S` (`ah-stage-sizing-v1`) | `Complexity: low` | `labels: ["harness-surface:none"]`.
* **Explicit Owned Paths (2 regular prose `.md` files under `docs/`):**
  1. `docs/compound/runtime-errors/agent-browser-managed-launch-devtoolsactiveport.md`
  2. `docs/compound/build-errors/chromedp-harness-google-chrome-path-discovery.md`
* **Reconciled Content & `P-002` Verification Record:**
  * Keeps the two root-cause signatures strictly separate (`agent-browser`
    `DevToolsActivePort` managed-adapter startup exit vs. `chromedp`
    `google-chrome` `PATH` auto-discovery failure resolved by `TEAMS_BROWSER_PATH`
    and `Options.ExecPath` / `ResolveExecPath`), uses YAML frontmatter `title:`
    with `##` body headings (never combining `title:` with `# ` H1), and
    cross-references (without modifying) the parent `2026-09-05` blocked closure
    record, the `U6` runbook, and the `2026-09-05` circuit-break memory note with
    zero live URLs, tokens, UPNs, or profile paths.
  * Embeds explicit `P-002` **Executable Gates** (`markdownlint --disable MD013
    "--"
    docs/compound/runtime-errors/agent-browser-managed-launch-devtoolsactiveport.md
    docs/compound/build-errors/chromedp-harness-google-chrome-path-discovery.md`,
    `gsd gate line-endings --path
    docs/compound/runtime-errors/agent-browser-managed-launch-devtoolsactiveport.md
    --json`, and `gsd gate line-endings --path
    docs/compound/build-errors/chromedp-harness-google-chrome-path-discovery.md
    --json`, cwd `.`, exit `0` / `0` findings, `result: pending`) separate from
    4-field **Manual Gates** (`source_paths`, `target_paths`, exact `pass_fail`
    criteria, and `required_evidence`).
* **Depends on:** `010.006-T` (`blocks`).

## Dependency Graph

```text
010.007-T (U7, none) ──▶ 010.001-T (U1, none) ──┬──▶ 010.002-T (U2, none) ──▶ 010.003-T (U3, none) ──┬──▶ 010.004-T (U4, none) ───────────────┐
                                                │                                                    └──▶ 010.005-T (U5, harness-architect) ──┴──▶ 010.006-T (U6, docs/none) ──▶ 010.009-T (U9, docs/none)
                                                └──▶ 010.008-T (U8, none — preserved from Wave 1A)
```

* All existing `blocks` dependency edges, `parent_id: 010-F` links, task sizes
  (`4 M, 4 S, 1 XS`), `010-S` manifest order, and `031-S` / `035-S` shipment
  dependencies remain 100% unchanged.

## Decisions and Rationale

* **Evidence-backed verification for `010.002-T`..`010.004-T` (`harness-surface:none`)**:
  Production code for `U2`, `U3`, and `U4` already shipped and hardened across
  `011-S`..`035-S`. Scoping `010.002-T`..`010.004-T` to their remaining
  unimplemented test/assertion files avoids artificial `P-004` production stubs
  and prevents stripping `#openTeamsClientInBrowser` or legacy selector
  fallbacks.
* **`harness-surface:harness-architect` on `010.005-T` with `audio_test.go` in
  budget**: `010.005-T` genuinely introduces new production Go code in
  `audio.go` and `meeting_chrome.go`; including non-tagged `audio_test.go`
  alongside `meeting_integration_test.go` satisfies `P-004`'s canonical `go test
  ./...` RED discovery and `leaf-diff` surface bounds.
* **Explicit `P-002` docs-route vs. non-docs `harness-surface:none` distinction
  (`_ship.agent.md` lines `442-448`)**: Naming concrete net-new `.md` paths and
  separate `Executable Gates` (`markdownlint`) + `Manual Gates` tables allows
  qualifying docs-only `harness-surface:none` tasks (`010.006-T`, `010.009-T`)
  to route through `harness-architect`'s Step 2a label-only path at T2 to
  receive `harness-verification-gated` (with no stub, test, or RED evidence) and
  pass `gsd harness docs-only-check` during end-of-Wave-1 Ship execution of
  `010-S` (Wave 2A is the separate Recap stash track), while non-docs
  `harness-surface:none` test/diagnostic tasks (`010.001-T`..`010.004-T`,
  `010.007-T`, `010.008-T`) skip `harness-architect` T2/T3 completely and carry
  no route label.

## Risks and Caveats

* **Shared file `internal/browser/meeting_integration_test.go` across sequenced
  tasks (`010.007-T` -> `010.008-T` / `010.004-T` / `010.005-T`)**: Tasks run
  sequentially inside a single `010-S` branch under Ship's Step 4 loop (`010.007-T`
  first, then `010.001-T`, then `010.008-T` and `010.002-T` -> `010.003-T` ->
  `010.004-T` / `010.005-T`). Each task owns a disjoint function/comment slice in
  `meeting_integration_test.go` (`validateLiveMeetingProfileDir` + `newMeeting`
  check in `010.007-T`, `// newMeeting ...` GoDoc in `010.008-T`,
  `TestMeetingEnableCaptions` in `010.004-T`, `TestMeetingCaptureAudio` in
  `010.005-T`).
* **Local `.gsd` resolver caveat, `origin/main` `.autoharness/*` state &
  `.backlogit/stash.jsonl` (`P-011`)**: `origin/main` (`a7015b9`) tracks
  `.autoharness/*` (`schema_version: "1.3.0"`), not `.gsd/*`. `HARNESS_READY`
  (`exit_code: 0`) was verified against the **local unpublished `.gsd` rollout**
  (`?? .gsd/harness-manifest.yaml` and modified
  `.github/skills/harness-architect/SKILL.md`), **not** published
  `.autoharness/*`; therefore downstream Ship execution of `010-S` remains gated
  on a separately published compatible harness (neither Stage nor Ship applies
  legacy route labels as a workaround or publishes the harness upgrade here).
  Stage touches zero source/config files and zero stash entries; canonical
  18-path literal-pathspec staging publication excludes all foreign dirty files,
  `.backlogit/stash.jsonl`, and local tool-managed event/checkpoint journals
  (`.backlogit/hooks_queue.jsonl`, `.backlogit/checkpoints/*`).
* **`gsd gate pre-review` `ART-02` detector mismatch & UTC slug note**:
  `gsd gate pre-review --base main --json` exits `0` with 9 medium `ART-02`
  findings on `010.001-T`..`010.009-T` because `ART-02` scans for a literal `##
  Acceptance Criteria` Markdown H2 whereas all 9 tasks populate `backlogit`
  v1.13's canonical `<!-- BEGIN:acceptance-criteria -->` sections (`558`–`1745`
  chars; no redundant H2 headings added). Session execution occurred on
  `2026-11-20` UTC (`025-DL` `2026-11-20T02:29:45Z`, `026-DL`
  `2026-11-20T03:48:42Z`); `2026-11-19-*` filename slugs reflect local PST
  session start and are retained to preserve frozen `plan_id` /
  `review_manifest` bindings.

## Constitution Check

* **Principle I (Safety-First Go)**: All Go tasks enforce `gofmt -l`, `go vet
  ./...`, `go vet "-tags=integration"`, `golangci-lint run` (plus
  `"--build-tags=integration"` where tagged files change), and `go test -race`.
* **Principle II (`P-002` / `P-004` Test-First Development)**: `010.005-T` uses
  full `harness-architect` RED-to-GREEN TDD via `audio_test.go`; `010.006-T` and
  `010.009-T` satisfy `P-002` docs-only verification gating; `010.007-T` and
  `010.001-T`..`010.004-T` touch strictly non-production test/diagnostic files
  with concrete inline verification commands.
* **Principle III & IV (Security Boundaries & CLI Containment)**: All paths stay
  inside repo root; `010.007-T` enforces repo-root/repo-ancestor rejection and
  repo-local `git check-ignore` on `TEAMS_PROFILE_DIR`; zero live URLs, `?p=`
  tokens, UPNs, or profile paths appear anywhere; authorized operator/fixture
  usage only.
* **Principle V, VI, VII, VIII, IX, X, XI**: Full audit trail (`026-DL`, review
  manifest, task comments, checkpoint); zero new dependencies; **Task Granularity
  conflict-resolution note**: because operator constraints freeze `010-S`'s
  10-item membership (rejecting splitting into additional tasks), `U1` retains
  its inherited diagnostic `*_test.go` + `docs/memory/` note pair (`<= 8`
  launches, `M`), `U4` spans 3 tightly coupled test files (`captions_test.go`,
  `meeting_e2e_test.go`, `meeting_integration_test.go`, `M`), and `U5` spans 4
  files (`audio.go`, `meeting_chrome.go`, `audio_test.go`,
  `meeting_integration_test.go`, `M`) because `P-004` canonical `go test ./...`
  requires non-tagged `audio_test.go` alongside `//go:build integration`
  `meeting_integration_test.go`; zero destructive actions; `Freeze-scope` +
  `Careful` + `strict-safety` active; LF Markdown + YAML frontmatter; `P-009`
  `--merge` preserved.

## Plan Hardening Signals (REQUIRED)

* **Public API, schema, or contract change**: **Present (narrow test-only hook in
  `010.005-T`)** — `010.005-T` exposes an optional nil-by-default test-only PCM
  observer seam in `package browser` so external `package browser_test` can
  verify live stimulus PCM before MP3 encoding without altering normal runtime
  behavior.
* **Security, auth, permission, or compliance-sensitive behavior**: **Present**
  — `010.007-T` specifies fail-closed `TEAMS_PROFILE_DIR` containment, and all
  `010-S` tasks govern secret/URL/UPN/profile redaction across live Teams runs.
* **Migration, backfill, destructive data/config action, or irreversible step**:
  **Absent** — non-destructive in-place metadata and specification updates on
  queued tasks.
* **External integration, operator checkpoint, or external dependency**:
  **Present** — models live Microsoft Teams human checkpoints (`D5`
  `Blocked-path handling`) and `P-011` dirty-worktree staging boundaries.
* **High runtime, rollout, or rollback risk**: **Absent** — Stage-only backlog
  updates; downstream `010.005-T` PCM observer is nil-by-default and guarded by
  `shine-mp3` `-race` tests.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed runtime surfaces (Stage)**: None (backlog and `docs/` planning
  artifacts only; downstream `010.005-T` touches `internal/browser/audio.go` and
  `meeting_chrome.go`).
* **Stage verification**:
  * `gsd harness resolve --workspace . --shipment 010-S --json` exits `0`
    (`state: "HARNESS_READY"`).
  * `backlogit_get_shipment("010-S")` confirms `size_composition.unsized == 0`
    and unchanged 10-item manifest.
  * `backlogit_doctor`, `backlogit_docs_lint`, `markdownlint --disable MD013
    "--"` over new `docs/` files, `gsd gate line-endings`, zero residual untagged
    preamble above `<!-- BEGIN:description -->` on all 8 reconciled task files,
    and `.backlogit/stash.jsonl` SHA-256 invariance all pass.
* **Operational closure**: `010.006-T` + `010.009-T` + Ship Step 6 post-merge
  closure and `P-020` `compact-context` when `010-S` ships (`026-DL` retained
  per `INV-12` until `010-S` archives, independent of `025-DL` / `AD451E00`).

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: Reconciles 8 tasks in a live-browser shipment
  (`010-S`) touching `TEAMS_PROFILE_DIR` containment (`010.007-T`), an exported
  test-only PCM observer seam (`010.005-T`), and `P-002` docs-only gating
  (`010.006-T`, `010.009-T`) inside a dirty worktree.
* **Safety Mode Posture (Principle VIII)**: `Freeze-scope mode` + `Careful mode`
  + `strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`,
    `.github/instructions/strict-safety.instructions.md`,
    `.github/instructions/browser-verification.instructions.md`,
    `.github/policies/workflow-policies.md` (`P-002`, `P-003`, `P-004`, `P-006`,
    `P-010`, `P-011`, `P-012`, `P-021`)
  * `docs/compound/build-errors/shine-mp3-frame-alignment.md` &
    `docs/compound/build-errors/shine-mp3-race-checkptr.md` (govern `010.005-T`)
  * `docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`
    (governs `010.007-T` external `%APPDATA%` vs. repo-local profile isolation)
  * `docs/compound/workflow-issues/pwsh-dotted-go-flag-tokenization.md`,
    `amend-breaks-shipment-gate.md`, `copilot-rereview-timing.md`, and
    `retained-engine-unverified-deliberation-cleanup.md`

### Protected Invariants

1. **Zero Secret, UPN, Passcode, or Host-Path Leakage (`Principle III`)**: No
   live URL, `?p=` token, Entra UPN, cookie/storage dump, or concrete host
   profile path may appear in any task, diagnostic note, runbook, compound
   learning, test, or commit.
2. **External `%APPDATA%`/`%LOCALAPPDATA%` Compatibility + Repo-Local Fail-Closed
   Ignore Check (`030-S`/`035-S`)**: `010.007-T` rejects repo-root and
   repo-ancestor `TEAMS_PROFILE_DIR` paths and enforces `git check-ignore -q` on
   repo-local paths while permitting external operator profiles.
3. **Preservation of Shipped Multi-Selector & Recap Contracts (`011-S`..`035-S`)**:
   `010.002-T`..`010.005-T` MUST NOT strip `#openTeamsClientInBrowser`,
   `[data-tid="hangup-main-btn"]`, `micStateRequiresMute` fallback, legacy
   caption `data-tid` selectors, or `TestMeetingCaptureAudio`'s `RecapMeeting`
   branch.
4. **Instance-Scoped Cloning PCM Observer Seam (`010.005-T`)**: The exported
   `AudioPCMObserver` optional interface on `*chromeMeeting` MUST synchronize
   observer registration/reads via `sync.Mutex` and pass `slices.Clone(samples)`
   whenever non-nil, never altering `enc.WritePCM` framing or `shine-mp3`
   `encoderTailSlack` `-race` `checkptr` safety.
5. **Wave 1A Contract & Dirty `.backlogit/stash.jsonl` Invariance (`P-011`)**:
   `010-S.md`, `010.008-T`, `025-DL`, `AD451E00`, committed `HEAD:.backlogit/stash.jsonl`
   (`SHA-256: 2085925DD4258468AA892F2BEA9AE426B6B10DBEFD298701D5F86D6F14B2D57A`),
   and dirty local `.backlogit/stash.jsonl` (`SHA-256:
   C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`) remain
   untouched.

### Lightweight 3-Threat Model (`Security Lens`)

1. **Most likely threat**: `010.007-T` logging a raw `TEAMS_PROFILE_DIR` value
   (including via unsanitized `*fs.PathError` / `*os.LinkError` from
   `os.Lstat` / `filepath.EvalSymlinks`) in an assertion failure message when
   rejecting an unignored, broken-symlink, or ancestor path — mitigated by
   requiring `validateLiveMeetingProfileDir` to emit symbolic error messages
   without formatting raw `err.Error()` path strings.
2. **Highest-impact threat**: `010.001-T` diagnostic probe capturing raw DOM /
   `location.href` containing `?p=` passcodes, meeting IDs, or MSAL tokens into
   `docs/memory/` — mitigated by stripping via `teams.RedactURL` and persisting
   only normalized route classes (`/meet/*` vs. `/l/meetup-join/*`) + boolean
   selector presence.
3. **Most subtle threat**: `010.005-T` PCM observer hook mutating or retaining a
   shared `samples` backing array concurrently with `enc.WritePCM` under `-race`
   — mitigated by Protected Invariant 4 (`sync.Mutex`-guarded instance field +
   `slices.Clone(samples)` before invoking a non-nil observer, verified under
   `go test -race`).

### Strict-Safety `ProposedAction` Inventory

| ProposedAction | Targets | Change Kind | ActionRisk | Approval | ActionResult | Rollback |
|---|---|---|---|---|---|---|
| `PA-W1B-1`: Create deliberation `026-DL` via `backlogit_create_item` (`artifact_type: "deliberation"`) without touching `.backlogit/stash.jsonl` | `.backlogit/queue/026-DL.md` (published; local `.backlogit/hooks_queue.jsonl` side-effect event journal excluded from canonical 18-path publication) | Backlog deliberation creation | `low` | Explicitly authorized by operator follow-up | `applied` | Delete untracked `026-DL.md`, run `git checkout -- .backlogit/hooks_queue.jsonl`, and run `backlogit sync` (only if explicitly approved) |
| `PA-W1B-2`: Reconcile `010.007-T`, `010.001-T`..`010.006-T`, and `010.009-T` full `description` bodies (`untagged_preamble == empty`), assign their 8 `harness-surface:*` labels (`010.005-T` -> `harness-surface:harness-architect`; remaining 7 -> `harness-surface:none`), refresh `size_ruleset_version: ah-stage-sizing-v1` in separate mutually exclusive calls, link `026-DL` (`informs`), and append traceability comments on those 8 tasks (`010-S.md` untouched) | `.backlogit/queue/010.001-T.md`..`010.007-T.md`, `.backlogit/queue/010.009-T.md` (published; local `.backlogit/hooks_queue.jsonl` side-effect event journal excluded from canonical 18-path publication) | Backlog task label repair & stale-spec reconciliation | `low` | Explicitly authorized by operator follow-up | `applied` | Run `git checkout -- .backlogit/queue/010.001-T.md ... .backlogit/queue/010.007-T.md .backlogit/queue/010.009-T.md .backlogit/hooks_queue.jsonl` and `backlogit sync` (only if explicitly approved) |
| `PA-W1B-3` (Downstream end-of-Wave-1 Ship execution of `010-S`): Execute reconciled `010-S` tasks `010.007-T`, `010.001-T`..`010.006-T`, `010.008-T`, `010.009-T` on a dedicated shipment branch after staging publication | `internal/teams/url_test.go`, `internal/browser/*_test.go`, `internal/browser/audio.go`, `internal/browser/meeting_chrome.go`, `docs/memory/2026-11-20/u1-live-diagnostic-findings.md`, `docs/closure/browser-automation/010-s-live-selector-path-verification-runbook.md`, `docs/compound/runtime-errors/agent-browser-managed-launch-devtoolsactiveport.md`, `docs/compound/build-errors/chromedp-harness-google-chrome-path-discovery.md` | Ship test, audio-tap, and docs execution | `moderate` | Deferred to end-of-Wave-1 Ship execution of `010-S` after staging publication | `planned` | `git revert` on `010-S` shipment branch |

### Verification Depth and Review-Gate Requirements (`P-012`)

* **Pre-mutation review gate**: Dispatch the 7-persona review roster
  (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`,
  `Learnings Researcher`, `Architecture Strategist` on anchor `gpt-6.1-sol`,
  `Security Lens Reviewer`, and `Evidence Consistency Reviewer` as declared
  `rubber-duck` fallback over a fresh scratch-index candidate git tree SHA) and
  record Pass 1 (`FAIL`, `revision: 1`) and targeted delta Pass 2 (`PASS`,
  `revision: 2`, `p0_open: 0`, `p1_open: 0`) via `gsd op review record-result
  --in-place` before executing `PA-W1B-1` or `PA-W1B-2`.
* **Post-mutation Stage gates**: Confirm `gsd harness resolve --workspace .
  --shipment 010-S --json` exits `0` (`state: "HARNESS_READY"`), `010-S`
  `size_composition.unsized == 0`, zero residual untagged preamble exists on
  `010.001-T`..`010.007-T` and `010.009-T`, `backlogit_doctor` reports `0`
  findings, committed `HEAD:.backlogit/stash.jsonl` stays at SHA-256
  `2085925DD4258468AA892F2BEA9AE426B6B10DBEFD298701D5F86D6F14B2D57A`, and
  dirty working-tree `.backlogit/stash.jsonl` retains SHA-256
  `C3C07FE23BFE70BEC368B6500534C468DD75EEA183CE50C3450F8A1925BCF82A`.

## Plan Review

Review pass 2: **PASS** (plan revision 2). See `docs/reviews/2026-11-19-010-s-sibling-labels-and-stale-spec-reconciliation-plan-review.md`.
