---
title: Transcript blockquote formatting and cold-profile recap authentication hardening decision
date: 2026-11-10
status: accepted
feature: 012-F
shipment: 012-S
---

## Problem frame

Post-merge testing of the Teams browser capture CLI surfaced three issues that
affect Markdown transcript formatting and Meeting Recap extraction reliability:

1. Speaker utterances were rendered inline (`**Speaker (HH:MM:SS):** text`),
   whereas downstream note workflows require a bold speaker header followed by
   a Markdown blockquote line (`**Speaker** (HH:MM:SS):\n> text`).
2. First-run headed capture with a newly configured `profile_dir` entered
   repeated authentication and re-navigation loops when opening
   `/l/meetingrecap` deep links before the base Teams v2 origin had initialized
   its MSAL and Skype tokens, or while multi-host Entra/MSA sign-in (`login.microsoft.com`,
   `mysignins.microsoft.com`) and OAuth `#code=` redemption were still in flight.
3. Meeting Recap links shared with `subType=RecapSharingLink_RecapChiclet` pass
   a SharePoint `/:v:/p/<user>/<token>` sharing URL in `fileUrl`. Extracting
   `filepath.Base(fileUrl)` treated the opaque sharing token as an `.mp4`
   filename, causing `verifyRecapTargetMatchesMeeting` to reject valid
   `xplatplugins.aspx` transcripts whose `g_fileInfo.name` carried the real
   `.mp4` recording name.

## Requirements and success criteria

* R1: `Writer.WriteEvent` renders each utterance as `**Speaker** (HH:MM:SS):\n> text\n\n` while preserving Markdown/HTML escaping and newline collapsing
* R2: Cold or unauthenticated profile directories bootstrap Meeting Recap sessions at the matching `/v2/` origin (`https://teams.microsoft.com/v2/` or `https://teams.cloud.microsoft/v2/`) before navigating to `/l/meetingrecap`
* R3: `headedAuthTracker` never interrupts stationary user login/MFA pages, OAuth callback code exchange (`code=`, `id_token=`, `interaction_in_progress`), or the Teams splash screen before session hydration
* R4: `findBoundCapturePage` follows single-tab COOP `targetId` swaps across Microsoft login and Teams landing hosts when `ParentID` is empty, while failing closed on concurrent tabs or conflicting meeting IDs
* R5: `ExtractMeetingIdentity` only records `RecordingName` from `fileUrl` when the path is not a SharePoint sharing link (`/:`) and ends with a media extension (`.mp4`, `.mkv`, `.webm`, `.mov`, `.m4a`)
* R6: All unit, race, lint, and compiled-binary acceptance checks pass and a review-gated PR is prepared against `main`

## Options

| Option | Benefit | Limitation |
|---|---|---|
| Bootstrap cold profiles on same-origin `/v2/` + state-tracked post-auth navigation | Eliminates `/l/meetingrecap` cold-profile redirect loops and preserves same-origin `localStorage` | Adds bounded Local Storage inspection and poll-state tracking in `internal/browser` |
| Increase fixed re-navigation timers only | Smaller code diff | Still interrupts multi-step MFA or reloads `/l/meetingrecap` before `/v2/` tokens exist |
| Disable target-identity verification on `fileUrl` entirely | Avoids sharing-link false negatives | Loses `.mp4` filename mismatch protection when `fileUrl` does carry a direct recording path |

## Chosen direction

We update `Writer.WriteEvent` to emit `**%s** (%s):\n> %s\n\n` while keeping
`sanitizeField` unchanged so embedded control characters and line breaks are
collapsed to spaces before formatting.

For browser authentication, `hasTeamsSessionInProfile` checks whether
`<profile_dir>/Default/Local Storage/leveldb` already contains `SKYPE-TOKEN`,
`UserAuthState`, or `msal.2.account.keys`. When absent for a Meeting Recap URL,
`newHeadedAuthTracker` launches the browser at the matching `/v2/` origin
(`teamsV2BootstrapURLFor`) first, waits until OAuth callback parameters clear
and the Teams v2 shell hydrates, and then navigates once to the target recap
URL. `isAuthRedirectURL` is expanded with `login.microsoft.com`,
`account.live.com`, `mysignins.microsoft.com`, and
`account.activedirectory.windowsazure.com` so multi-host login steps are not
misclassified as Teams-to-auth bounces. `findBoundCapturePage` tracks
concurrent page target IDs (`otherPageIDs`) so single-tab COOP `targetId`
swaps rebind safely without switching to unrelated open tabs.

For recap identity verification, `applyQueryIdentity` ignores `fileUrl` paths
containing `/:` SharePoint sharing prefixes or lacking a media file extension,
and `verifyRecapTargetMatchesMeeting` compares `exp.DriveItemID` against
`p.ItemID` only when `p.ItemID` carries a Vroom item prefix (`01...`).

## Boundaries and risks

* No credentials, session cookies, real meeting URLs, or personal participant names in repository artifacts or test fixtures
* Single-tab COOP rebind remains restricted to trusted HTTPS Microsoft/Teams/SharePoint hosts with no conflicting meeting identity and no concurrent tab ambiguity
* Previously merged shipment `011-S` is closed and archived with its merge commit SHA (`d50bab42c8dc6339c8b791f0f17647dce991df82`); separate queued shipment `010-S` remains untouched

## Open questions

None blocking implementation or verification.

## Related artifacts

* [Capture design](../design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md)
* [Implementation plan](../plans/2026-11-10-transcript-blockquote-and-recap-auth-plan.md)
* [Review and closure](../closure/2026-11-10-012-S-review-and-closure.md)
