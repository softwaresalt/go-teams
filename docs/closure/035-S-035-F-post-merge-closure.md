---
title: Recap audio cold SharePoint transcode resilience and %APPDATA% profile fallback operational closure
date: 2026-11-17
feature: 035-F
shipment: 035-S
branch: feat/035-s-recap-audio-cold-sharepoint-transcode-resilience-and-appdata-profile-fallback
status: closed
closure_status: READY
compaction_status: done
merge_commit: b2538a2f7385b6e443d43771c02b6121da446a79
pull_request: 84
close_path: cascade
close_evidence: docs/closure/evidence/035-S-035-F-close-evidence.json
---

## Release scope

Shipment `035-S` (`035-F`, `035.001-T`..`035.003-T`) delivers deliberation
`024-DL` (source stash `6E17E6EA`), hardening cold SharePoint `oneDrive.transcode`
audio extraction and cross-root `%LOCALAPPDATA%` / `%APPDATA%` profile session
discovery across `internal/browser` and `docs/`:

* `035.001-T` (`internal/browser/recap.go`, commits `0dd4998`, `770fda6`,
  `96c2fe0`, `619f01e`, `24ffd83`, `c123099`):
  * In `extractRecapAudioJS`: marked HTTP `408` retryable alongside `429`/`5xx`
    in `fetchBytesOnce`; scaled `fetchBytesWithCap` to `5` attempts
    (`250ms, 500ms, 1000ms, 2000ms` backoff) and `deadlineAt` to `240000ms`
    (`4m`) when `maxRecapAudioDurationSec > 300` (with optional
    `recapAudioFetchMaxAttempts` and `recapAudioCaptureDeadlineMs` overrides);
    increased `runPathA` worker concurrency from `8` to `12`; tracked
    `btnClicked` and `mediaPlayTriggered` separately in
    `triggerPlaybackIfReady()` so late-mounted `<video>` elements still receive
    `m.play()`; and threw `'recap audio Path A capture deadline exceeded'` when
    `pathAPromise && !pathAFailed && !pathAResult` before falling back to partial
    pre-pause `mseInitSegment` / `mseSegments`.
  * In `captureRecapAudioFromTargets`: increased `recapAudioEvalTimeout` from
    `5m` to `10m` enforced as an operation-wide `captureCtx` deadline; allowed a
    single attempt-0 retry on `'Path A capture deadline exceeded'` or
    `'MSE stream incomplete before deadline'`; stopped the inner
    `orderedTargets` loop on stream-deadline errors so same-origin parent
    targets do not repeat capture in the same attempt; and preserved
    `ctx.Err()` and `errors.Join(lastErr, captureCtx.Err())` on cancellation or
    timeout.
  * In `resolveExpandedProfileDir`: added `userConfigProfileBases` (`APPDATA`,
    `os.UserConfigDir()`, `LOCALAPPDATA`), `sameNormalizedPath`, and
    `isPathInsideRepoRoot` (with a `os.UserHomeDir()` `.git` dotfiles carve-out)
    so cold canonical `<base>/<go-teams|teams-cli>/profile` paths fall back to
    sibling per-user config directories with a persisted Teams session while
    leaving warm, custom `--profile-dir`, and repo-local paths untouched.
* `035.002-T` (`internal/browser/recap_test.go`, commits `f2d880a`, `770fda6`,
  `96c2fe0`, `619f01e`, `24ffd83`, `c123099`):
  * Extended `TestResolveExistingProfileDirExpansionAndFallback` and added
    `TestRecapAudioColdTranscodeResilienceAndDeadlineRetry` and
    `TestRecapAudioColdTranscodeResilienceAndAppDataFallback`.
* `035.003-T` (`docs/ARCHITECTURE.md`, `docs/USER_GUIDE.md`, commits `4fccb86`,
  `96c2fe0`, `24ffd83`):
  * Documented `%LOCALAPPDATA%` / `%APPDATA%` (`os.UserConfigDir()`) profile
    fallback and cold-transcode resilience (12 workers, 4m capture deadline,
    10m operation-wide CDP budget, 5-attempt backoff, single attempt-0 retry).

## Review

Six rounds of `multi-agent` Adversarial Review (`gpt-6.1-sol` anchor +
`claude-haiku-5.5` + `gemini-4-argon`, culminating in `READY_WITH_FOLLOWUPS` at
HEAD `c123099888c88cabd65d0d23f059517ef4b4cda7`, `0` P0, `0` HIGH Consensus,
`0` MEDIUM Majority/Plurality) and two GitHub Copilot review rounds on PR #84
(`P-018` `copilot-review` gate `SATISFIED` on HEAD
`c123099888c88cabd65d0d23f059517ef4b4cda7`):

| Area | Finding | Disposition |
|---|---|---|
| Adversarial Review Rounds 1–5 | `770fda6`..`24ffd83` `U1`/`U2`/`U3`/`M1` items (caller cancellation on attempt 1, `captureCtx` `DeadlineExceeded` preservation, unused `isPathInsideRepoRoot` removal, full-ancestor repo check with home dotfiles carve-out, JS override tests, doc clarifications) | Remediated in-scope (P-021 C1) in `96c2fe0`, `619f01e`, `24ffd83`, and `c123099` |
| Copilot PR #84 Round 1 (`PRRT_kwDOTStvas64wnOB`, `PRRT_kwDOTStvas64wnOR`) | Caller cancel loss on `captureCtx.Done()` select branch; full candidate `isPathInsideRepoRoot(cand)` check | Fixed in `770fda6`..`24ffd83`; replied and resolved |
| Copilot PR #84 Round 2 (`PRRT_kwDOTStvas64x3cF`) | Break `orderedTargets` loop on stream-deadline error so same-origin parent does not duplicate capture inside same attempt | Fixed in `c123099`; replied and resolved |
| Copilot PR #84 Round 2 (`PRRT_kwDOTStvas64x3cS`) | Reinitializing player or persisting Path B MSE state across CDP attempts for Path B-only streams | Captured under P-021 C2 as `B98262C8` (`DEFERRED SCOPE EXPANSION`); replied citing `B98262C8` and resolved |
| Adversarial Review Round 1 & Round 6 (`LOW` advisories) | `920CF700` (R1 `U1`–`U3`, addressed in `96c2fe0`/`24ffd83`), `CDCE76CE` (R5/R6 `U1`–`U3`, including `U1` classified `MAJOR`/`P1` `LOW`-confidence `1/3` `IN_SCOPE` for Protected Invariant 4 symlink/junction ancestor resolution in `isPathInsideRepoRoot`, plus `U2`/`U3` `P2` advisories) | Stashed for Stage triage; zero `P0`, `HIGH` Consensus, or `MEDIUM` Majority P1 findings (`READY_WITH_FOLLOWUPS` at `c123099`; `LOW`-confidence findings are `advisory` in full-pool mode) |

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #84, merge commit `b2538a2f7385b6e443d43771c02b6121da446a79`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-18T05:01:35Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 035-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/035-s-recap-audio-cold-sharepoint-transcode-resilience-and-appdata-profile-fallback` |
| Close path | `cascade` (`autoharness shipment cascade-close --shipment 035-S --feature 035-F --sha b2538a2f7385b6e443d43771c02b6121da446a79 --message ... --author Ship --timeout 600 --caller-wait 1200` returned `exit_code: 0`, `classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`) |
| Close evidence | `docs/closure/evidence/035-S-035-F-close-evidence.json` |
| `archived_ids` | `035-F`, `035-S`, `035.001-T`, `035.002-T`, `035.003-T` (`unexpected_archived: []`) |
| `returned_ids` | empty |
| `parent_id` preservation | `035.001-T`..`035.003-T` retained `parent_id: 035-F` (`parent_id_preserved: true`) |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup and follow-up stash state

| Artifact | Disposition | Result |
|---|---|---|
| Stash `6E17E6EA` | Already archived by Stage at harvest; absent from active stash | skipped |
| Deliberation `024-DL` | Archived via `backlogit archive 024-DL` prior to `cascade-close`; resolved to `already-archived` in `pre_close.linked_deliberation_disposition` | archived (`.backlogit/archive/024-DL.md`) |
| Stash `B98262C8` | Captured under P-021 C2 for Path B-only MSE state persistence / player reload across CDP retry attempts (PR #84 `PRRT_kwDOTStvas64x3cS`); untouched per Ship Role Boundary | retained active for Stage triage |
| Stash `920CF700`, `CDCE76CE` | Adversarial review `LOW` Unique follow-up advisories (`920CF700` items addressed in `96c2fe0`/`24ffd83`; `CDCE76CE` tracks R6 `U1` `MAJOR`/`P1` [`LOW`-confidence `1/3` `IN_SCOPE` advisory for `isPathInsideRepoRoot` symlink/junction resolution], plus `U2` `os.UserHomeDir()` doc note and `U3` hermetic `4b` isolation — non-blocking `advisory` under full-pool `READY_WITH_FOLLOWUPS`) | retained active for Stage triage |

### Knowledge graduation, compaction (P-020), and compound refresh

Graduated documentation in `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md`
(`035.003-T`), and added compound learning
`docs/compound/runtime-errors/cold-sharepoint-transcode-path-a-deadline-and-appdata-fallback-2026-11-17.md`.
`compact-context` consolidated `stage-035-s-recap-audio-resilience-and-profile-fallback-memory.md`
and `ship-035-s-recap-audio-resilience-and-profile-fallback-memory.md` into
`docs/memory/compacted/2026-11-17-035-S-recap-audio-resilience-and-profile-fallback-compacted.md`
(moving both originals to `docs/archive/memory/2026-11-17/`), and consolidated
the plan into
`docs/plans/2026-11-17-recap-audio-cold-transcode-resilience-and-appdata-profile-fallback-decided-plan.md`
with the verbose original archived under `docs/archive/plans/2026-11-17/`.
`compound-refresh` verified existing `docs/compound/` entries remain accurate
(`keep`). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Build, vet, fmt | `go build -o bin/go-teams.exe ./cmd/go-teams`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `gofmt -l .` (0 files) | PASS |
| Unit & E2E suite | `go test -race ./...` across all packages including `TestResolveExistingProfileDirExpansionAndFallback`, `TestRecapAudioColdTranscodeResilienceAndDeadlineRetry`, `TestRecapAudioColdTranscodeResilienceAndAppDataFallback`, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` | PASS |
| CI `windows-latest` & `ubuntu-latest` | `Detect code changes`, `Detect CLI-affecting changes`, and `Build, vet, lint & test (Windows)` on PR #84 (HEAD `c123099`) | PASS |
| Runtime verification probes | `go run ./cmd/go-teams --help`, `go run ./cmd/go-teams schedule --help`; `teams-auth-mfa` (`required_for_release: true` in `.autoharness/workspace-profile.yaml`) marked `BLOCKED` (missing prerequisite: live operator interactive Teams login/MFA completion outside automated CI); `scheduled-meeting-join` (`required_for_release: false`) deferred to post-deploy observation window | PASS (CLI probes); BLOCKED (`teams-auth-mfa`) |
| Backlog & closure gate | `autoharness gate closure-evidence`, `backlogit sync`, and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| `PA-035-1`: Scale production `deadlineAt` to `240s`, increase `runPathA` to `12` workers, add HTTP `408` + 5-attempt backoff (`250/500/1000/2000ms`), split `btnClicked`/`mediaPlayTriggered`, prioritize in-flight Path A deadline error, set `recapAudioEvalTimeout=10m` with single attempt-0 retry, and add `%APPDATA%` / `os.UserConfigDir()` fallback in `internal/browser/recap.go` | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; verified across 6 adversarial review rounds, 2 Copilot rounds, and `go test -race` | applied |
| `PA-035-2`: Add unit tests in `internal/browser/recap_test.go` | low | Pre-authorized under `DARK_MODE_ACTIVE`; all tests pass with `-race` | applied |
| `PA-035-3`: Update `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md` | low | Pre-authorized under `DARK_MODE_ACTIVE`; zero secret leaks verified | applied |

## Releasability evidence

* **Invariants preserved**: Same-origin HTTPS transcode containment and recording identity verification enforced; `track === 'video'` never seeds audio template/timing fields; in-flight `pathAPromise` deadline precedence prevents 1–2 pre-pause MSE buffers from misreporting `'MSE stream incomplete before deadline'`; `triggerPlaybackIfReady()` guards against `m.play()` while `pathAActive === true`; warm, custom `--profile-dir`, and repo-local profiles are never redirected (`%APPDATA%` / `os.UserConfigDir()` fallback is a scoped cross-root exception to the workspace-local `UserDataDir` invariant in `.autoharness/workspace-profile.yaml` for per-user default profiles outside repo roots); zero new dependencies (`go.mod` unchanged).
* **Rollout and rollback**: Code, tests, docs, and closure artifacts on `main`. Rollback trigger: any test regression, non-zero exit from CLI root or scheduler daemon, unintended profile redirection, or CI check failure; procedure: `git revert -m 1 b2538a2f7385b6e443d43771c02b6121da446a79` (and stop scheduler daemon if crash-looping).
* **Monitoring and validation window (7 days, owner: `softwaresalt`)**:
  * **Healthy signals**: 100% pass rate on `CI` (`Detect code changes`, `Detect CLI-affecting changes`, `Build, vet, lint & test (Windows)`) and `go test -race ./...`, including `TestResolveExistingProfileDirExpansionAndFallback`, `TestRecapAudioColdTranscodeResilienceAndDeadlineRetry`, `TestRecapAudioColdTranscodeResilienceAndAppDataFallback`, and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`; CLI `--help` / `schedule --help` exit 0 (scheduler daemon liveness and cron fire metrics in `.autoharness/workspace-profile.yaml` are unaffected by Recap-only `recap.go` changes and monitored across one full scheduled-meeting cycle).
  * **Failure signals and rollback triggers**: Non-zero exit from CLI root or scheduler daemon, scheduler crash-loops or consecutive cron fires failing to produce transcripts, any test regression, or CI check failure.
* **Readiness verdict**: `READY` (gate-required `closure_status: READY` under `autoharness gate closure-evidence`, which accepts `READY` or `READY_WITH_CONDITIONS` only when every condition has `satisfied: true`). Gate-vs-profile disposition: all automated release gates, local quality gates, unit/E2E suites, adversarial review culminating in P-014 `READY_WITH_FOLLOWUPS` at `c123099` (`0` P0, `0` HIGH/MEDIUM P1, `1` LOW Unique `P1` `advisory` `U1` tracked under `CDCE76CE`, plus `920CF700` and P-021 C2 `B98262C8` stashed for Stage triage), 2 Copilot review rounds, GitHub Actions CI checks, and cascade-close postconditions passed; live interactive `teams-auth-mfa` (`required_for_release: true` in `.autoharness/workspace-profile.yaml`) is recorded `BLOCKED` in automated CI without an interactive operator (`035-S` does not modify interactive MFA authentication, only existing-session discovery and cold transcode audio capture).

## Dark Factory Execution Summary

* **Resolved route**: `gemini-4-hydrogen` / `google` / `high` (`long_context` context tier)
* **Gates passed**:
  * `pipeline-topology` (`pre_claim`, `post_claim`, `lifecycle` feature + `lifecycle` closure) — `exit_code: 0`
  * Local Review Readiness (`P-014`) — `READY_WITH_FOLLOWUPS` at HEAD `c123099888c88cabd65d0d23f059517ef4b4cda7` (`0` P0, `0` HIGH/MEDIUM P1, follow-ups `920CF700`, `CDCE76CE`, `B98262C8` stashed)
  * Copilot Review Completion Gate (`P-018`) — `SATISFIED` (`exit_code: 0`, all PR #84 review threads replied and resolved)
  * Merge Strategy (`P-009`) — merge commit `b2538a2f7385b6e443d43771c02b6121da446a79`
  * Context Compaction (`P-020`) — `compaction_status: done`
