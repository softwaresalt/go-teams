---
title: Transcript blockquote formatting and cold-profile recap authentication hardening plan
date: 2026-11-10
status: implemented
feature: 012-F
shipment: 012-S
---

## Problem frame

Following the merge of `011-S` (PR #29), live operator usage with a newly
configured user profile and a batch transcript queue exposed three issues:

1. Speaker lines in exported Markdown transcripts needed to separate the bold
   speaker name from the timestamp and place the utterance in a blockquote
   (`**Speaker** (HH:MM:SS):\n> text`).
2. First-run headed authentication on a new profile directory looped between
   Teams and Microsoft login hosts because `joinHeaded` launched directly into
   `/l/meetingrecap` before `/v2/` initialized MSAL/Skype tokens, missed
   `login.microsoft.com` in `isAuthRedirectURL`, and re-navigated before OAuth
   `#code=` redemption and splash-screen token acquisition finished.
3. `RecapSharingLink_RecapChiclet` queue URLs carry a SharePoint `/:v:/p/`
   sharing link in `fileUrl`, whose opaque token was stored as `RecordingName`
   and rejected the real `.mp4` filename in `g_fileInfo.name`.

Source: [accepted decision](../decisions/2026-11-10-transcript-blockquote-and-recap-auth-deliberation.md).

## Requirements trace

| Requirement | Implementation units | Evidence |
|---|---|---|
| R1: Speaker header and blockquote format | U1 (`012.001-T`) | `TestWriterEvent`, `TestWriteEventCollapsesLineBreaks`, `TestCaptionsCommandEndToEndWritesMarkdownFile`, `TestCompiledBinaryCapturesRecapToExplicitOutput` |
| R2: Same-origin `/v2/` cold-profile bootstrap | U2 (`012.002-T`) | `TestHasTeamsSessionInProfile`, `TestHeadedAuthTrackerColdProfileBootstrapsTeamsV2AndDoesNotLoopDuringLogin` |
| R3: Non-interrupting auth/MFA and OAuth callback tracker | U2 (`012.002-T`) | `TestIsAuthRedirectURLIncludesMicrosoftLoginHosts`, `TestHeadedAuthTrackerBreaksRapidRecapAuthBounceLoopViaTeamsV2` |
| R4: Single-tab COOP `targetId` rebind with concurrent-tab guard | U2 (`012.002-T`) | `TestFindBoundCapturePageRebindsSingleTabAcrossAuthAndTeamsHostsWithoutSwitchingToConcurrentTab`, `TestVerifyCapturePagePresentFailsClosedWhenBoundTabDisappears` |
| R5: SharePoint `/:v:/p/` sharing-link `fileUrl` handling | U3 (`012.003-T`) | `TestExtractMeetingIdentityAndVerifyRecapTargetMatchesMeeting` |
| R6: Quality gates, review, and PR readiness | `012-S` | Vet, integration vet, CI-pinned lint, race tests, compiled binary acceptance, and PR readiness block |

## Implementation units

Each unit is scoped within the 2-hour rule and verified with unit and end-to-end tests.

| Unit | Backlog | Size / Complexity | Surface and files | Verification |
|---|---|---|---|---|
| U1 | `012.001-T` | `XS` / `low` | `internal/transcript/writer.go`, `internal/transcript/writer_test.go`, `internal/cli/e2e_test.go`, `internal/cli/binary_automation_test.go` | Observed red then green on `go test ./internal/transcript` and `./internal/cli` |
| U2 | `012.002-T` | `M` / `medium` | `internal/browser/meeting_chrome.go`, `internal/browser/recap.go`, `internal/browser/recap_test.go` | Observed red then green on `TestHasTeamsSessionInProfile`, `TestHeadedAuthTracker*`, and `TestFindBoundCapturePage*` |
| U3 | `012.003-T` | `S` / `low` | `internal/browser/recap.go`, `internal/browser/recap_test.go` | Observed red then green on `TestExtractMeetingIdentityAndVerifyRecapTargetMatchesMeeting` |

## Dependency graph

* `012.001-T` (U1) and `012.002-T` (U2) are independent implementation units under `012-F`.
* `012.003-T` (U3) depends on `012.002-T` (U2) as both refine `internal/browser/recap.go` and `internal/browser/recap_test.go`.
* Shipment `012-S` groups `012-F`, `012.001-T`, `012.002-T`, and `012.003-T` on branch `feat/transcript-blockquote-and-recap-auth`.

## Constitution check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only; explicit error checks; zero `golangci-lint` and `go vet` findings |
| II: Test first | Unit tests in `writer_test.go` and `recap_test.go` observed red before production edits and green after |
| III, IV: Isolation and containment | All repository writes confined to `C:\Source\GitHub\go-teams`; no external files modified |
| V: Observability | Structured `slog` messages for `/v2/` bootstrap, post-auth navigation, and bounce recovery |
| VI: Single responsibility | No new external dependencies added |
| VII, VIII: Approval and safety | Auth/session changes classified under `strict-safety` below; no destructive git or filesystem operations |
| IX, X: Persistence and context | Markdown + YAML frontmatter artifacts and `backlogit` state synchronized |
| XI: Merge history | Dedicated feature branch PR targeting `main`; merge commit required |

## Plan Hardening

* Security/auth-sensitive behavior: present; `hasTeamsSessionInProfile`, `isAuthRedirectURL`, `headedAuthTracker`, and `findBoundCapturePage` govern interactive authentication and browser target binding.
* Public output format change: present; `Writer.WriteEvent` changes speaker utterance rendering in Markdown transcripts.
* Rollback and containment: self-contained changes in `internal/transcript` and `internal/browser`; reverting the PR commit restores previous behavior without schema or profile migration.

Requires plan hardening: yes

### Risk controls and proposed actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | ActionResult |
|---|---|---|---|
| Read `<profile_dir>/Default/Local Storage/leveldb` files (`<= 16 MiB`) in `hasTeamsSessionInProfile` to detect existing Teams tokens | high | Read-only local byte-substring check (`SKYPE-TOKEN`, `UserAuthState`, `msal.2.account.keys`); never logs or exports token values | applied |
| Expand `isAuthRedirectURL` and pause CDP re-navigation during multi-host login and `#code=` redemption | high | Restricted to `hasHTTPSHostSuffix` allowlist of known identity providers; never weakens HTTPS host validation | applied |
| Rebind single capture tab across COOP `targetId` swaps in `findBoundCapturePage` | high | Tracks concurrent page IDs (`otherPageIDs`) and rejects any candidate with a conflicting `MeetingIdentity` or untrusted host | applied |
| Ignore SharePoint `/:v:/p/` sharing tokens when extracting `RecordingName` from `fileUrl` | moderate | Preserves `ThreadID`, `DriveItemID`, `DriveID`, `CallID`, `ICalUID`, and `.mp4` filename conflict checks | applied |
| Reconcile and close merged shipment `011-S` (`d50bab42c8dc6339c8b791f0f17647dce991df82`) | low | Verified all `011-S` items are in `.backlogit/archive/` and `010-S` remains intact in `.backlogit/queue/` | applied |

## Plan review gate

* Architecture & scope review: PASS (`P0=0, P1=0`). Changes remain within `internal/transcript`, `internal/browser`, and `internal/cli` tests with no new dependencies or cross-package coupling.
* Security lens review: PASS (`P0=0, P1=0`). Local Storage inspection is bounded (`<= 16 MiB`), read-only, and never emits token contents; COOP tab rebind fails closed when concurrent tabs or conflicting meeting IDs exist.

## Runtime verification and closure

Verify `gofmt -l cmd internal`, `go vet ./...`, `go vet -tags=integration ./...`,
CI-pinned `golangci-lint run`, `go test -race ./...`, `go build ./...`, and
compiled-binary acceptance (`TestCompiledBinaryCapturesRecapToExplicitOutput`).
Record validator and releasability evidence in
[2026-11-10-012-S-review-and-closure.md](../closure/2026-11-10-012-S-review-and-closure.md).
