---
title: Past-meeting recap reliability, deterministic Entra auth bootstrap, Edge CDP lifecycle, and transcript passcode redaction operational closure
date: 2026-11-14
feature: 026-F
shipment: 026-S
branch: feat/026-s-past-meeting-recap-reliability-deterministic-entra-auth-bootstrap-edge-cdp-lifecycle-and-transcript-passcode-redaction
status: closed
closure_status: READY
compaction_status: done
merge_commit: 89059afb9fd9c02f238cdf91525761e9a1783009
pull_request: 66
close_path: cascade
close_evidence: docs/closure/evidence/026-S-026-F-close-evidence.json
---

## Release scope

Shipment `026-S` (`026-F`, `026.001-T`..`026.004-T`) delivers deliberation
`015-DL` (source stash `52519E8D`, `0544DBA7`, `030CD519`, `6B0E5F68`,
`1B1877DB`, `54894478`, and `2925E813`):

* `026.001-T` (`internal/browser`, commit `22a5997`):
  * Separates `AISummaryOnly` from `ForceRecap` on `browser.MeetingOptions` so
    `--mode past-recap` (`ForceRecap: true, AISummaryOnly: false`) waits for
    `.ms-List` transcript entries before returning from `Join()`, while
    `--append-ai-summary` (`AISummaryOnly: true`) returns once the AI Summary is
    copied.
  * Honors `DisableAISummary` during Meeting Recap extraction.
  * Hardens `extractRecapAudioJS` for deduplicated non-`blob:` candidate
    ordering, per-candidate identity verification with fallback across
    unverified or non-decodable candidates, `redirect: 'follow'` with
    post-redirect `response.url` identity verification, `credentials: 'omit'`
    retry on `TypeError`, `AudioContext({sampleRate: 44100})`, and `4200s` /
    `256 MiB` source / `360 MiB` PCM caps (`16 MiB` DOM CDP message cap).
* `026.002-T` (`internal/browser`, commit `a7c0fb6`):
  * Requires non-empty `DefaultAccount` (`browser.ErrAccountRequired`) on
    `ChromeRunner.Authenticate` and cold-profile `Join` /
    `joinHeadedWithDeadline`.
  * Keeps `joinHeadedWithDeadline` alive when the Windows `msedge.exe` parent
    stub exits while `listCDPTargets(port)` responds, and cleans up surviving
    child browser listeners via bounded `Browser.close` and verified loopback
    PID termination.
* `026.003-T` (`internal/capture`, `internal/transcript`, commit `7bd513b`):
  * Preserves `# AI Summary` when `CaptureAudio` fails on a Recap and removes
    0-byte `.mp3` artifacts after `out.Close()`.
  * Honors `DisableAISummary` in `Session.Run`.
  * Uses `headerStart` (`cfg.MeetingStart` when non-zero) for
    `FormatTranscriptFilename` date/time placeholders.
  * Redacts `SourceURL` and fallback `MeetingID` URLs to `scheme://host/path`
    (stripping userinfo, query parameters, fragments, and SharePoint `/:`
    sharing paths while preserving leaf-package isolation in
    `internal/transcript`).
* `026.004-T` (`internal/cli`, `docs/`, commit `531a65a`):
  * Wires `AISummaryOnly`, `DisableAISummary`, and `auth` `--account`
    validation in `internal/cli`, updates `redactHelpArg` doc comments
    (`2925E813`), and updates `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md`.

## Review

Multi-persona adversarial and `rubber-duck` review of `main..HEAD` (nine
reviewers: `Architecture Strategist`, `Security Reviewer`,
`Correctness Reviewer`, `Go Reviewer`, `Concurrency Reviewer`,
`Maintainability Reviewer`, `Scope Boundary Auditor`, `Constitution Reviewer`,
and `rubber-duck`) followed by GitHub Copilot review of PR #66 (P-018
`copilot-review` gate `SATISFIED` on HEAD
`7c8aa267c95504fd198902ee4305ecd763f2fbda`):

| Area | Finding | Disposition |
|---|---|---|
| Capture / Transcript (`rubber-duck` P1) | `sanitizeMeetingID` matched `meeting_` tokens across unredacted query/fragment parameters and missed percent-encoded `/%3A` SharePoint sharing paths | Fixed in-scope (P-021 C1): `hasSharePointSharingPath` multi-pass unescape check and redacted-path / `threadId`-only token extraction; commit `1504fa8` |
| Browser (`rubber-duck` P2, Maintainability P3) | `awaitPreJoinOrAdmissionWithDeadline` re-navigated to raw `MeetingURL` instead of derived `meetingrecap` URL after auth; `captureAISummaryFromTargets` lacked `isAuthRedirectURL` guard; duplicated Recap extraction loop | Fixed in-scope (P-021 C1): extracted `extractRecapFromTargets`, bounded with `withDeadlineContext`, added `isAuthRedirectURL` guard; commit `1504fa8` |
| Docs / GoDoc (Architecture P3, Maintainability P2/P3, Constitution P3) | `docs/ARCHITECTURE.md` dependency diagram omitted `internal/teams` under `internal/capture`; `docs/USER_GUIDE.md` cold-profile examples omitted `--account`; `_ =` discards needed Principle I comments | Fixed in-scope (P-021 C1); commit `1504fa8` |
| Copilot (`PRRT_kwDOTStvas63m8H9`, `internal/capture/output.go`) | Raw-token fallback in `sanitizeMeetingID` could match `meeting_` inside a rejected SharePoint sharing path | Fixed in-scope (P-021 C1): up-front `hasSharePointSharingPath(rawURL)` guard + test; commit `ef30a77`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63m8IF`, `internal/browser/meeting_chrome.go`) | Headless cold-profile check skipped SharePoint Recap URLs and empty `UserDataDir` on `NewLiveMeeting` | Fixed in-scope (P-021 C1): `managedProfile` + `isRemoteAuthBackedMeetingURL` check + test; commit `ef30a77`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63m8IO`, `internal/browser/recap.go`) | Unverified early media URL candidate aborted `extractRecapAudioJS` before trying later verified candidates | Fixed in-scope (P-021 C1): skip unverified candidate without fetching and continue to later candidates; commit `ef30a77`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63m_jf`, `internal/browser/recap.go`) | Unconditional `taskkill /PID cmd.Process.Pid` in `cleanup` risked PID reuse after `cmd.Wait()` reaped an exited launcher | Fixed in-scope (P-021 C1): removed raw-PID `taskkill`, guarded `cmd.Process.Kill()` with `<-procDone`, and used verified listener cleanup (`terminateSurvivingCDPBrowserOnPort`); commit `4c97830`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63nCJ5`, `internal/browser/recap.go`) | Replacing verified `candidateUrl` with opaque CDN `resp.url` lost identity evidence for Go-side re-verification | Fixed in-scope (P-021 C1): preserve verified `candidateUrl` when `resp.url` is opaque while still running `checkIdentity(resp.url)`; commit `7c8aa26`; replied and resolved |
| Copilot (`PRRT_kwDOTStvas63nCKI`, `internal/browser/recap.go`) | Candidate loop exited before `decodeAudioData`, so a non-audio manifest/HTML response on an earlier candidate prevented fallback | Fixed in-scope (P-021 C1): moved bounded stream read and `decodeAudioData` inside candidate loop; commit `7c8aa26`; replied and resolved |

Blocking findings: `P0=0, P1=0`. Outcome: `READY`.

## Post-merge closure

| Field | Value |
|---|---|
| Feature PR | #66, merge commit `89059afb9fd9c02f238cdf91525761e9a1783009`, merge commit strategy (P-009) |
| Merge confirmation | `state: MERGED` (`2026-11-15T06:59:30Z`); `git merge-base --is-ancestor` against `origin/main` returned 0 |
| Topology gate | `autoharness gate pipeline-topology --mode agent --shipment 026-S --phase lifecycle --json` passed (exit 0, `BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`) on `post-merge/026-s-past-meeting-recap-reliability-deterministic-entra-auth-bootstrap-edge-cdp-lifecycle-and-transcript-passcode-redaction` |
| Close path | `cascade` (`autoharness shipment cascade-close --shipment 026-S --feature 026-F --sha 89059afb9fd9c02f238cdf91525761e9a1783009` returned `exit_code: 0`, `classifier_verdict: CASCADE`, `engine_verdict: VERIFIED`, `postcondition_verdict: pass`) |
| Close evidence | `docs/closure/evidence/026-S-026-F-close-evidence.json` |
| `archived_ids` | `026-F`, `026-S`, `026.001-T`, `026.002-T`, `026.003-T`, `026.004-T` (equal to `required_ids`; `unexpected_archived: []`) |
| `returned_ids` | empty |
| `parent_id` preservation | `026.001-T`..`026.004-T` retained `parent_id: 026-F` (`parent_id_preserved: true`) |
| Archive integrity (P-007) | No archive file reported as deleted |

### Source artifact cleanup

| Artifact | Disposition | Result |
|---|---|---|
| Stashes `52519E8D`, `0544DBA7`, `030CD519`, `6B0E5F68`, `1B1877DB`, `54894478`, `2925E813` | Already archived by Stage at harvest; absent from active stash | skipped |
| Deliberation `015-DL` | `retained_description_mention`; status `queued` with zero live references in `.backlogit/queue/`; archived via `backlogit archive 015-DL` | archived |

### Compaction (P-020)

`compact-context` ran at closure, producing
`docs/memory/compacted/2026-11-14-026-S-recap-auth-and-redaction-hardening-compacted.md`
and archiving the verbose session memory files to
`docs/archive/memory/2026-11-14/stage-026-S-memory.md` and
`docs/archive/memory/2026-11-14/026-S-ship-memory.md`. `compound-refresh`
reviewed all 10 entries in `docs/compound/` (retaining all 10 and updating
`docs/compound/workflow-issues/copilot-rereview-timing.md` with the GraphQL
`requestReviews` bot-ID trigger). `compaction_status` is `done`.

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Static analysis | `go build -o bin/teams-cli.exe ./cmd/teams-cli`, `go vet ./...`, `go vet -tags=integration ./...`, `gofmt -l .` (clean) | PASS |
| Full test suite | `go test -race ./...` on Windows (including `TestChromeMeetingEndToEnd` and Node.js WebAudio/SharePoint probes) | PASS |
| CI `ubuntu-latest` | `Build, vet, lint & test`, GitHub Actions run `43687510912` (PR #66, HEAD `7c8aa26`) | PASS |
| CI `windows-latest` | `Build, vet & test (Windows)`, same run | PASS |
| Backlog & closure gate | `backlogit sync` (166 artifacts indexed), `backlogit doctor` ("No issues found."), and `autoharness gate closure-evidence` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| `PA-026-1`: Require `--account` / `default_account` on initial `auth` and cold-profile bootstrap; keep headed browser loop alive across Windows `msedge.exe` stub exit and clean up surviving child listener on `127.0.0.1:<port>` | high | Pre-authorized under `DARK_MODE_ACTIVE`; raw-PID launcher `taskkill` removed and child termination gated on `isCDPPortStillAlive`, loopback `LISTENING` PID > 4, and `isChromiumProcessImage` | applied and verified |
| `PA-026-2`: Separate `AISummaryOnly` from `ForceRecap`, honor `DisableAISummary`, and harden Recap audio extraction + 0-byte `.mp3` cleanup | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; covered by unit and Node.js probes | applied and verified |
| `PA-026-3`: Redact `**Source:**` header links and fallback `MeetingID` URLs to `scheme://host/path` and use `MeetingStart` for transcript filenames | moderate | Pre-authorized under `DARK_MODE_ACTIVE`; covered by unit tests | applied and verified |

## Releasability evidence

* **Invariants preserved**: Leaf-package isolation (`internal/transcript` imports stdlib only), single-binary CGO-free build, URL/passcode redaction (`scheme://host/path`), Recap recording identity isolation, and owner-only (`0o600`/`0o700`) output permissions.
* **Rollout and rollback**: Single-binary CLI on `main`; rollback via `git revert -m 1 89059afb9fd9c02f238cdf91525761e9a1783009` (retaining the `main`-branch parent to revert the feature merge cleanly).
* **Monitoring and validation window (7 days, owner: repository maintainer)**:
  * **Healthy signals**:
    1. *Tenant Recap capture (`transcripts/*.md`, `slog` output)*: `--mode past-recap` logs `browser: extracted meeting recap transcript` with `entries > 0`, populates `**Meeting Start:**` and `<YYYYMMDD>` filename dates from the meeting's actual start time, redacts `**Source:**` to `scheme://host/path` with zero query/fragment tokens, and omits `# AI Summary` when `--no-ai-summary` is passed; `--recap-audio` writes a non-empty `.mp3` (up to 70 minutes) or removes any 0-byte `.mp3` if audio extraction fails while still preserving `# AI Summary` in the `.md` file.
    2. *Cold-profile authentication (`slog` output)*: `teams-cli auth` and cold-profile `join` / `captions` without `--account` / `default_account` fail fast with `browser: initial authentication requires --account or default_account in config.yaml`, and with `--account` log `browser: selected default Entra account` (`account=a***@domain`).
    3. *Windows Edge CDP lifecycle (`netstat -ano -p tcp`)*: Headed Edge launches survive parent stub exit (`procDone`) while `127.0.0.1:<cdp_port>` remains active, and after session exit `netstat -ano -p tcp` shows zero lingering `LISTENING` sockets on the ephemeral `<cdp_port>`.
  * **Failure signals and rollback triggers**:
    1. `--mode past-recap` returning 0 transcript utterances when `.ms-List` is populated in the Recap tab, or leaving a 0-byte `.mp3` on disk after an audio error.
    2. Any query parameter (`?p=`, `driveId=`, `sitePath=`, `threadId=`), fragment, userinfo, or SharePoint `/:` sharing path appearing in `**Source:**` or a fallback `MeetingID` / filename.
    3. Orphaned `msedge.exe` / `chrome.exe` listener remaining on `127.0.0.1:<cdp_port>` after `cleanup` completes, or premature `browser process exited before meeting admission` while the child browser window is still open.
    * *Action on trigger*: Halt release distribution and execute `git revert -m 1 89059afb9fd9c02f238cdf91525761e9a1783009`.
* **Readiness verdict**: `READY` (all local unit, race, Node.js WebAudio, and Windows headless E2E checks passed; live Microsoft 365 tenant integration harness execution is tracked separately under queued shipment `010-S`).
