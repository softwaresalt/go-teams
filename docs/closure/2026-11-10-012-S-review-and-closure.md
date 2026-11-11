---
title: Transcript blockquote formatting and recap auth hardening review and operational closure
date: 2026-11-10
feature: 012-F
shipment: 012-S
branch: feat/transcript-blockquote-and-recap-auth
status: ready-with-conditions
closure_status: READY
compaction_status: done
merge_commit: aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb
pr: 30
---

## Release scope

Shipment `012-S` (`012-F` and `012.001-T` through `012.003-T`) updates Markdown
transcript utterance formatting to a bold speaker header plus blockquote line
(`**Speaker** (HH:MM:SS):\n> text`), hardens first-run headed authentication on
newly configured profile directories via same-origin `/v2/` bootstrap,
non-interrupting OAuth/MFA state tracking, and single-tab COOP `targetId`
rebinding, and ignores opaque SharePoint `/:v:/p/` sharing tokens in `fileUrl`
during Meeting Recap identity verification. It also completes the post-merge
reconciliation of `011-S` (`d50bab42c8dc6339c8b791f0f17647dce991df82`). The
separate queued shipment `010-S` remains unchanged.

## Review

Code review ran across two passes on `feat/transcript-blockquote-and-recap-auth`:

* Pass 1 surfaced one P2 finding in `internal/browser/meeting_chrome.go`:
  `headedAuthTracker.onNonAuthPage` lowered `stallThreshold` from 20 polls
  (30s) to 5 polls (7.5s) when `reNavigateAttempts == 0`, which could cause a
  warm profile launched directly at a recap URL (`onTarget == true`) to reload
  after 7.5s while the recap page was still loading.
* Remediation: removed the `reNavigateAttempts == 0` override on the on-target
  stall threshold (keeping `settledPolls >= 20` when already on the target URL,
  while off-target and `launchagent=join_launcher_web` landings continue to use
  the 3- or 5-poll post-auth threshold) and added a warm-profile unit test
  assertion in `internal/browser/recap_test.go`.
* Pass 2 re-verified the remediated diff: `READY`, with `P0=0, P1=0, P2=0, P3=0`
  unresolved findings.
* Copilot shadow review on PR #30 raised four in-scope findings (P-021 C1 same
  contract surface), all fixed with regression tests:
  * Omitted `navURL` from bounce-recovery `slog.Info` in `meeting_chrome.go`
  * Recorded candidates seen during multi-tab missing-target polls in
    `findBoundCapturePage` so subsequent single-tab polls still fail closed
  * Restricted `hasRecapIframe` via `isRecapIframeTarget` to `iframe` targets
    matching `xplatplugins.aspx`, `streamembed.aspx`, or `hasHTTPSHostSuffix(..., "sharepoint.com")`
  * Added `sanitizeBlockquoteText` in `writer.go` to escape leading `-`, `+`,
    `=`, and ordered-list markers (`1.`, `1)`) at the start of blockquote captions

## Verification evidence

| Validator | Environment and evidence | Result |
|---|---|---|
| Format | `gofmt -l cmd internal` (0 files listed) | PASS |
| Static analysis | `go vet ./...` | PASS |
| Integration compilation | `go vet -tags=integration ./...` | PASS |
| Lint | `golangci-lint run`, CI-pinned v1.64.8 with `GOTOOLCHAIN=go1.26.0` | PASS |
| Race suite | `go test -race ./...` | PASS |
| Full build | `go build ./...` and `go build -o .\bin\teams-cli.exe .\cmd\teams-cli` | PASS |
| Compiled acceptance | `go test ./internal/cli -run TestCompiledBinaryCapturesRecapToExplicitOutput -count=1` | PASS |
| Operator queue verification | Headed queue run against a `RecapSharingLink_RecapChiclet` URL extracted 468 entries, 13 attendees, and AI summary in 15s | PASS |
| Backlog integrity | `backlogit sync` and `backlogit doctor` | PASS |

## Strict-safety action record

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Read `<profile_dir>/Default/Local Storage/leveldb` (`<= 16 MiB` per file) in `hasTeamsSessionInProfile` | high | Read-only local byte-substring check; never logs or exports token values | applied |
| Expand `isAuthRedirectURL` and pause CDP re-navigation during multi-host login and `#code=` redemption | high | Restricted to `hasHTTPSHostSuffix` allowlist of known identity providers | applied |
| Rebind single capture tab across COOP `targetId` swaps in `findBoundCapturePage` | high | Tracks concurrent page IDs (`otherPageIDs`) and rejects conflicting `MeetingIdentity` or untrusted hosts | applied |
| Ignore SharePoint `/:v:/p/` sharing tokens when extracting `RecordingName` from `fileUrl` | moderate | Preserves `ThreadID`, `DriveItemID`, `DriveID`, `CallID`, `ICalUID`, and `.mp4` filename conflict checks | applied |
| Reconcile and close merged shipment `011-S` (`d50bab42c8dc6339c8b791f0f17647dce991df82`) | low | Verified all `011-S` items are in `.backlogit/archive/` and `010-S` remains intact in `.backlogit/queue/` | applied |

## Releasability evidence

| Evidence | Status | Condition |
|---|---|---|
| Synthetic CLI and browser contract | READY | All unit, e2e, and six compiled CLI forms pass |
| Local quality gates | READY | Build, vet, integration vet, CI-pinned lint, format, and race suite pass |
| SharePoint sharing-link recap extraction | READY | Verified by unit regression and live queue capture |
| Cold-profile headed sign-in/MFA | READY_WITH_CONDITIONS | Requires interactive operator sign-in/MFA on first use of a new `profile_dir` (standing operational note; not a release blocker) |
| Shipment lifecycle | READY | Merged to `main` (PR #30, `aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb`) and `012-S` closed via post-merge reconciliation |

## Monitoring and observation window

Owner: operator deploying the executable.
Duration: one cold-profile headed sign-in and one batch `--queue` recap run
during the first 30 minutes after deployment.

| SLI | Observation source | Baseline | Alert threshold |
|---|---|---|---|
| Utterance formatting | Exported `.md` transcript | Every utterance renders `**Speaker** (HH:MM:SS):` followed by `> text` | Any inline `**Speaker (HH:MM:SS):**` utterance or unescaped newline |
| Cold-profile bootstrap | CLI `slog` output | Single `/v2/` bootstrap followed by single navigation to target recap URL | Repeated auth redirect loop or >2 re-navigation attempts |
| Sharing-link recap extraction | CLI `slog` output and `.md` file | `browser: extracted meeting recap transcript` within 30s of recap load | Hang on `RecapSharingLink_RecapChiclet` URLs when `xplatplugins.aspx` is present |

## Rollback

Trigger rollback on any authentication loop regression, rejected valid recap
transcript, or malformed Markdown utterance output. Revert the merge commit on
`main` and rebuild `teams-cli.exe` from the prior release commit. Existing
browser profiles and exported transcripts require no migration or deletion.

## Post-merge closure

PR #30 merged to `main` at `aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb`
(`2026-11-10T22:59:41Z`). Dark Factory Mode (P-017) post-merge closure ran on
`post-merge/012-transcript-blockquote-and-recap-auth`:

* Verified `MERGE_CONFIRMED` via `gh pr view 30` (`state: MERGED`) and
  `git merge-base --is-ancestor aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb
  origin/main`.
* `autoharness gate pipeline-topology --phase lifecycle --shipment 012-S`
  passed (`BRANCH_POST_MERGE_CLOSURE_ELIGIBLE`, `WORKTREE_TOPOLOGY_OK`).
* P-015 fully-covered-root verification: `012-F` is a root feature (no
  `parent_id`) whose only descendants, `012.001-T` through `012.003-T`, are
  all explicit `012-S` manifest members, so
  `classify_shipment_close_path` selected the cascade close path.
* `backlogit shipment ship 012-S --sha
  aed3bbbc8a08c7f0d9fdc752a9b658eb704f6cfb` archived exactly the required
  set (`archived_ids: [012.001-T, 012.002-T, 012.003-T, 012-F, 012-S]`,
  `returned_ids: []`), with every task's `parent_id` preserved and the
  separate, unchanged `010-S` shipment and its nine tasks verified intact
  in `.backlogit/queue/` throughout.
* `backlogit sync` and `backlogit doctor` ran clean post-closure.
* Mandatory P-020 `compact-context` (`target: all`) ran; recorded as
  `compaction_status: done` above.

## Handoff

Related [decision](../decisions/2026-11-10-transcript-blockquote-and-recap-auth-deliberation.md),
[plan](../plans/2026-11-10-transcript-blockquote-and-recap-auth-plan.md), and
[design](../design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md).
