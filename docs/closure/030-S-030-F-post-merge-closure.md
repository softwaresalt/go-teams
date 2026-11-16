---
title: Live integration .env.local.test path expansion, legacy profile fallback, and remote headless Recap fixes operational closure
date: 2026-11-16
feature: 030-F
shipment: 030-S
branch: feat/030-s-live-integration-env-local-test-path-expansion-legacy-profile-fallback-and-remote-headless-recap-fixes
status: closed
closure_status: READY_WITH_CONDITIONS
conditions:
  - condition: SharePoint tenant raw audio download policy (isDownloadBlocked=true) deterministically detected and skipped via t.Skip in TestMeetingCaptureAudio while all other Tier 2 E2E and Tier 5 live-tenant -tags=integration tests pass
    satisfied: true
    evidence: docs/closure/030-S-030-F-post-merge-closure.md
compaction_status: done
merge_commit: 29d554b918a55c8b978d3f826c6672d358ef5a8a
pull_request: 74
close_path: cascade
close_evidence: docs/closure/evidence/030-S-030-F-close-evidence.json
---

## Release scope

Shipment `030-S` (`030-F`, `030.001-T`..`030.003-T`) delivers deliberation
`019-DL` (source stash `02B5113C`), resolving `.env.local.test` environment
variable expansion, legacy `teams-cli` fallback, remote headless CDP routing,
and SharePoint Meeting Recap extraction issues across `internal/browser` and
`internal/cli`:

* `030.001-T` (`internal/browser/prejoin.go`, `internal/browser/meeting.go`,
  `internal/browser/recap.go`, commits `1387ae9`, `0104696`, `3fe5457`,
  `b9e1672`):
  * Adds `#openTeamsClientInBrowser` to `joinOnWebSel` in
    `internal/browser/prejoin.go`.
  * Adds single-pass `%VAR%`, `${VAR}`, `$VAR`, and `~` expansion plus
    rightmost non-repo-root `go-teams` -> `teams-cli` profile fallback
    (`expandProfilePath`, `resolveExistingProfileDir`,
    `resolveExpandedProfileDir`) in `internal/browser/recap.go`.
  * Supports SharePoint `VroomItemId` (lowercase `d`) in `extractRecapAudioJS`
    and `extractRecapTranscriptJS`, detects SharePoint `isDownloadBlocked=true`
    across HTTP errors, `text/html` error pages, trailing `blob:` candidates,
    and multi-target loops, and polls `listCDPTargets(port)` up to `2s` on
    Windows teardown to prevent `UserDataDir` lock contention.
  * Resolves `Options.UserDataDir` once at the `NewLiveMeeting` input boundary
    (`m.managedProfile`) and routes remote authenticated headless sessions with
    a non-empty profile through `cm.headedOpts` (`launchCleanHeadedBrowser` with
    `--headless=new`) in `internal/browser/meeting.go`.
* `030.002-T` (`internal/browser/integration_helper_test.go`,
  `internal/browser/meeting_integration_test.go`,
  `internal/browser/scheduled_lifecycle_integration_test.go`, commits
  `b0b67e9`, `3fe5457`, `1add058`, `b9e1672`):
  * Updates `loadTestEnvFromFile` / `resolveRelative` in
    `internal/browser/integration_helper_test.go` for single-pass
    `%VAR%`/`${VAR}`/`$VAR`/`~` expansion (failing closed on unset variables
    and drive-relative paths without echoing values), post-expansion bare
    browser executable resolution, `go-teams` -> `teams-cli` profile fallback,
    and `teams-cli.exe` -> `go-teams.exe` binary fallback.
  * Updates `internal/browser/meeting_integration_test.go` to use
    `browser.NewLiveMeeting`, adds synchronous caption guard in
    `TestMeetingEnableCaptions`, and handles `RecapMeeting` +
    `isDownloadBlocked=true` (`t.Skip`) in `TestMeetingCaptureAudio`.
  * Updates `internal/browser/scheduled_lifecycle_integration_test.go` to allow
    external `%APPDATA%` profile directories (while keeping `git check-ignore`
    enforcement for repo-local profiles and rejecting repository-ancestor
    profiles) and support `RecapMeeting` caption verification in
    `TestMeetingLiveScheduledOvertimeAndCallEnd`.
* `030.003-T` (`internal/cli/binary_automation_test.go`,
  `internal/cli/scheduled_integration_test.go`,
  `internal/browser/recap_test.go`, commits `7d2fb4b`, `3fe5457`, `b9e1672`):
  * Updates `internal/cli/binary_automation_test.go` with symmetric single-pass
    env expansion, post-expansion bare browser resolution, Teams-session-backed
    `go-teams` -> `teams-cli` profile fallback, and `teams-cli.exe` ->
    `go-teams.exe` binary fallback.
  * Updates `internal/cli/scheduled_integration_test.go` to permit external
    `%APPDATA%` profiles and validate `/l/meetingrecap` URLs via
    `--mode past-recap` in `TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`.
  * Adds unit tests in `internal/browser/recap_test.go` covering
    `resolveExistingProfileDir`, `NewLiveMeeting` single-pass profile
    expansion (`TestNewLiveMeetingExpandsProfileOncePreservingLiteralVarTokens`),
    `extractRecapAudioJS` `isDownloadBlocked=true` + `text/html`, and
    `VroomItemId`.

## Review

Multi-persona adversarial and `rubber-duck` review of `main..f99cb56` (nine
reviewers: `Architecture Strategist`, `Security Reviewer`,
`Correctness Reviewer`, `Go Reviewer`, `Maintainability Reviewer`,
`Scope Boundary Auditor`, `Constitution Reviewer`,
`Schema-CLI-Docs Coupling Reviewer`, and `rubber-duck`), followed by GitHub
Copilot review of PR #74 (`P-018` `copilot-review` gate `SATISFIED` on HEAD
`b9e1672088dac02c15c85d9755ccb33f8bf83a87`):

| Area | Finding | Disposition |
|---|---|---|
| Pre-PR 9-persona + `rubber-duck` panel | In-scope hardening: profile ancestor containment (`realProfile` vs `realRoot`), drive-relative checks before path cleaning, POSIX `%VAR%`/`~` subtests, and `tildeRoot` test wiring | Fixed in-scope (P-021 C1) in commits `3fe5457` and `1add058` |
| Pre-PR 9-persona + `rubber-duck` panel (out-of-scope) | `meeting_chrome.go` startup/cleanup serialization, exported bound-target hangup helper + queued `010.008-T` update, and `AllocatorOptions` profile expansion parity | Deferred out-of-scope under P-021 C1/C2 to active stash entries `232C6844`, `AD451E00`, and `3F6BA819` |
| Copilot (`PRRT_kwDOTStvas63xV7_`, comment `4582511227`, `internal/browser/meeting.go:115`) | Profile resolved in `NewLiveMeeting` was re-expanded downstream in launcher/allocator paths | Fixed in-scope (P-021 C1) in commit `b9e1672` (`m.managedProfile` + `TestNewLiveMeetingExpandsProfileOncePreservingLiteralVarTokens`); replied and resolved |
| Copilot (`PRRT_kwDOTStvas63xV8G`, comment `4582511276`, `internal/cli/binary_automation_test.go:288`) | Bare browser executable check ran before `$VAR`/`%VAR%` expansion in `binary_automation_test.go` and `integration_helper_test.go` | Fixed in-scope (P-021 C1) in commit `b9e1672`; replied and resolved |

Blocking findings: `P0=0, P1=0`. Outcome: `READY_WITH_FOLLOWUPS`.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #74, merge commit `29d554b918a55c8b978d3f826c6672d358ef5a8a`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-16T06:30:03Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 030-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/030-s-live-integration-env-local-test-path-expansion-legacy-profile-fallback-and-remote-headless-recap-fixes` |
| Close path | `cascade` (`autoharness shipment cascade-close --shipment 030-S --feature 030-F --sha 29d554b918a55c8b978d3f826c6672d358ef5a8a --timeout 600 --caller-wait 1200` returned `exit_code: 0`, `classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`; recovered after initial `--timeout 180` timeout) |
| Close evidence | `docs/closure/evidence/030-S-030-F-close-evidence.json` |
| `archived_ids` | `030-F`, `030-S`, `030.001-T`, `030.002-T`, `030.003-T` (equal to `required_ids`; `unexpected_archived: []`) |
| `returned_ids` | empty |
| `parent_id` preservation | `030.001-T`..`030.003-T` retained `parent_id: 030-F` (`parent_id_preserved: true`) |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup and follow-up stash state

| Artifact | Disposition | Result |
|---|---|---|
| Stash `02B5113C` | Already archived by Stage at harvest; absent from active stash | skipped |
| Deliberation `019-DL` | Archived via `backlogit archive 019-DL` prior to `cascade-close`; resolved to `already-archived` in `pre_close.linked_deliberation_disposition` (`docs/closure/evidence/030-S-030-F-close-evidence.json`) | archived (`.backlogit/archive/019-DL.md`) |
| Stash `232C6844` | Captured per P-021 C2 for out-of-scope `meeting_chrome.go` startup/cleanup serialization | retained active for Stage triage |
| Stash `AD451E00` | Captured per P-021 C2 for out-of-scope `browser.Meeting` bound-target hangup helper and `010.008-T` alignment | retained active for Stage triage |
| Stash `3F6BA819` | Captured per P-021 C2 for out-of-scope `AllocatorOptions` profile expansion parity | retained active for Stage triage |
| Stash `9BDFB275`, `560B0C44`, `188D4FBB` | Pre-existing active stash entries (`028-S`, `029-S`, and `version` enhancement) | preserved untouched in `.backlogit/stash.jsonl` |

### Knowledge graduation, compaction (P-020), and compound refresh

Graduated runtime and test-harness documentation in `docs/USER_GUIDE.md`
(`.env.local.test` `%VAR%`/`$VAR`/`~` expansion, external `%APPDATA%` profile
containment, `go-teams` -> `teams-cli` profile fallback, `teams-cli.exe` ->
`go-teams.exe` binary fallback, and `/l/meetingrecap` integration handling) and
`docs/ARCHITECTURE.md` (remote authenticated `--headless=new` direct-exec CDP
launcher routing, `#openTeamsClientInBrowser`, `VroomItemId`, and
`isDownloadBlocked=true` rejection). `compact-context` consolidated
`stage-030-s-live-integration-fixes-memory.md` and
`ship-030-s-live-integration-fixes-memory.md` into
`docs/memory/compacted/2026-11-16-030-S-live-integration-fixes-compacted.md`
and archived the verbose originals to `docs/archive/memory/2026-11-16/`.
`compound-refresh` verified existing entries in `docs/compound/` remain valid
(`keep`). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Build, vet, lint, fmt | `go build ./...`, `go build -o .\bin\go-teams.exe .\cmd\go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `golangci-lint` v1.64.8, `gofmt -l cmd internal` (0 files) | PASS |
| Unit & E2E suite | `go test -v -race -p 1 ./...` across all packages | PASS |
| CI `ubuntu-latest` | `Detect code changes` (includes secret hygiene check) and `Build, vet, lint & test`, GitHub Actions run `43816475634` (PR #74, HEAD `b9e1672`) | PASS |
| CI `windows-latest` | `Build, vet & test (Windows)`, GitHub Actions run `43816475634` (PR #74, HEAD `b9e1672`) | PASS |
| CLI Reference Drift Check | GitHub Actions run `43816475637` (PR #74, HEAD `b9e1672`) | PASS |
| Live `-tags=integration` (`TestChromeRunnerLaunch`) | `internal/browser/launch_integration_test.go` (`1.77s`, headless browser smoke launch) | PASS |
| Live `-tags=integration` (`TestMeetingJoin`) | `internal/browser/meeting_integration_test.go` (`19.62s`, `#openTeamsClientInBrowser`, AI summary + 393 transcript entries) | PASS |
| Live `-tags=integration` (`TestMeetingEnableCaptions`) | `internal/browser/meeting_integration_test.go` (`25.38s`, 393 transcript entries delivered) | PASS |
| Live `-tags=integration` (`TestMeetingCaptureAudio`) | `internal/browser/meeting_integration_test.go` (`22.78s`, deterministic `isDownloadBlocked=true` detection -> `t.Skip`) | SKIP (SharePoint policy) |
| Live `-tags=integration` (`TestMeetingLiveScheduledOvertimeAndCallEnd`) | `internal/browser/scheduled_lifecycle_integration_test.go` (`21.65s`, external `%APPDATA%` profile + Recap transcript delivery) | PASS |
| Live `-tags=integration` (`TestMeetingLiveRecapAISummaryEnrichment`) | `internal/browser/scheduled_lifecycle_integration_test.go` (`20.81s`, `ForceRecap` + AI summary copied) | PASS |
| Live `-tags=integration` (`TestLiveTenantScheduledJoinOvertimeAndCallEndShutdown`) | `internal/cli/scheduled_integration_test.go` (`20.01s`, `--mode past-recap`, 393 transcript entries + `# AI Summary` written) | PASS |
| Live `-tags=integration` (`TestLiveTenantDelayedAISummaryQueueReplay`) | `internal/cli/scheduled_integration_test.go` (`22.90s`, delayed AI summary appended + `::` queue completion) | PASS |
| Backlog & closure gate | `autoharness gate closure-evidence`, `backlogit sync`, and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| `PA-030-1`: Update `joinOnWebSel`, add single-pass profile expansion and `go-teams` -> `teams-cli` fallback, support `VroomItemId` and `isDownloadBlocked=true` detection, poll CDP teardown, and route remote authenticated headless `NewLiveMeeting` sessions through `cm.headedOpts` | high | Pre-authorized under `DARK_MODE_ACTIVE`; `TestNewLiveMeetingExpandsProfileOncePreservingLiteralVarTokens` and `node -e` Recap JS tests verified | applied |
| `PA-030-2`: Update `internal/browser` `.env.local.test` loader and `-tags=integration` tests for `%VAR%`/`$VAR`/`~` expansion, `NewLiveMeeting`, external `%APPDATA%` profiles, and `RecapMeeting` + `isDownloadBlocked=true` handling | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; verified on `ubuntu-latest`, `windows-latest`, and live `-tags=integration` suite | applied |
| `PA-030-3`: Update `internal/cli` `.env.local.test` loader, `scheduled_integration_test.go`, and `recap_test.go` unit tests | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; verified on `ubuntu-latest`, `windows-latest`, and live `-tags=integration` suite | applied |
| `PA-030-4`: Execute `-tags=integration` live-tenant verification against `.env.local.test` using the clean `--headless=new` CDP launcher and external `%APPDATA%` profile | high | Pre-authorized under `DARK_MODE_ACTIVE`; 7 `PASS`, 1 deterministic `SKIP` (`TestMeetingCaptureAudio`, SharePoint `isDownloadBlocked=true`) | applied |

## Releasability evidence

* **Invariants preserved**: Zero unredacted URLs, passcodes, or account identifiers in logs or tracked files; `TEAMS_PROFILE_DIR` rejects repository ancestors and enforces `git check-ignore` on repo-local profiles; `NewLiveMeeting` expands `UserDataDir` once at the input boundary; active stash entries `188D4FBB`, `560B0C44`, `9BDFB275`, `232C6844`, `AD451E00`, and `3F6BA819` preserved in `.backlogit/stash.jsonl`.
* **Rollout and rollback**: Code, test, and closure artifacts on `main`. Rollback via `git revert -m 1 29d554b918a55c8b978d3f826c6672d358ef5a8a`.
* **Monitoring and validation window (7 days, owner: `softwaresalt`)**:
  * **Healthy signals**: 100% pass rate on `CI` (`Detect code changes`, `Build, vet, lint & test`, `Build, vet & test (Windows)`) and `go test -race ./...`, plus clean live `-tags=integration` runs against `.env.local.test` with zero `UserDataDir` lock collisions.
  * **Failure signals and rollback triggers**: Any regression in `go test -race ./...`, profile lock collision across sequential integration tests, or false-negative `isDownloadBlocked=true` decoding failure.
* **Readiness verdict**: `READY_WITH_CONDITIONS` (all local quality gates, unit/E2E suites, multi-persona and Copilot reviews, `ubuntu-latest` + `windows-latest` CI jobs, and 7 of 8 live-tenant `-tags=integration` tests passed; `TestMeetingCaptureAudio` deterministically verified SharePoint `isDownloadBlocked=true` permission enforcement via `t.Skip`).

## Dark Mode Execution Summary (P-017)

* **Resolved route**: `gemini-4-hydrogen` / `google` / `high` (`long_context` context tier)
* **Gates passed**:
  * `pipeline-topology` (`pre_claim`, `post_claim`, `lifecycle` feature + `lifecycle` closure) — `exit_code: 0`
  * Local Review Readiness (`P-014`) — `LOCAL_REVIEW_READY` for HEAD `b9e1672088dac02c15c85d9755ccb33f8bf83a87` (`READY_WITH_FOLLOWUPS`, `0 P0, 0 P1`)
  * Copilot Review Completion Gate (`P-018`) — `SATISFIED` (`exit_code: 0`, both PR #74 review threads replied and resolved)
  * Merge Strategy (`P-009`) — merge commit `29d554b918a55c8b978d3f826c6672d358ef5a8a` (`DARK_MODE_MERGE_AUTHORIZED`, normal `gh pr merge 74 --merge` succeeded without admin fallback)
  * Context Compaction (`P-020`) — `compaction_status: done`
