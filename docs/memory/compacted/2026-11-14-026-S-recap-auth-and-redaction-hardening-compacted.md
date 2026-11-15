---
title: Compacted session memory for shipment 026-S (026-F)
description: Consolidated Stage and Ship session memory for past-meeting recap reliability, deterministic Entra auth bootstrap, Edge CDP lifecycle, and transcript passcode redaction
date: 2026-11-14
feature: 026-F
shipment: 026-S
deliberation: 015-DL
pull_request: 66
merge_commit: 89059afb9fd9c02f238cdf91525761e9a1783009
archived_sources:
  - docs/archive/memory/2026-11-14/stage-026-S-memory.md
  - docs/archive/memory/2026-11-14/026-S-ship-memory.md
status: compacted
---

## Summary of Shipped Work

Shipment **`026-S`** (Feature **`026-F`**, deliberation **`015-DL`**, PR **#66**, merge commit `89059afb9fd9c02f238cdf91525761e9a1783009`) grouped and resolved seven stash entries (`52519E8D`, `0544DBA7`, `030CD519`, `6B0E5F68`, `1B1877DB`, `54894478`, and `2925E813`):

* **`026.001-T` (`22a5997`, `internal/browser`)**: Separated `AISummaryOnly` from `ForceRecap` on `browser.MeetingOptions` so `--mode past-recap` (`ForceRecap: true, AISummaryOnly: false`) waits for `.ms-List` transcript entries before returning from `Join()`, while `--append-ai-summary` (`AISummaryOnly: true`) returns once the AI Summary is copied; honored `DisableAISummary` during Recap extraction; and hardened `extractRecapAudioJS` for deduplicated non-`blob:` candidate ordering, per-candidate identity verification with fallback across unverified or non-decodable candidates, `redirect: 'follow'` with post-redirect `response.url` identity verification (preserving verified `candidateUrl` when `response.url` is an opaque CDN URL), `credentials: 'omit'` retry on `TypeError`, `AudioContext({sampleRate: 44100})`, and `4200s` / `256 MiB` source / `360 MiB` PCM caps (`16 MiB` DOM CDP message cap).
* **`026.002-T` (`a7c0fb6`, `internal/browser`)**: Enforced `browser.ErrAccountRequired` on `ChromeRunner.Authenticate` and cold-profile `Join` / `joinHeadedWithDeadline` across remote `https://` meeting/Recap hosts (`isRemoteAuthBackedMeetingURL`), kept `joinHeadedWithDeadline` alive when the Windows `msedge.exe` parent stub exits while `listCDPTargets(port)` responds, and replaced unverified raw-PID launcher `taskkill` in `launchCleanHeadedBrowser` with bounded `Browser.close`, `<-procDone`-guarded `cmd.Process.Kill()`, and verified loopback listener cleanup (`terminateSurvivingCDPBrowserOnPort`).
* **`026.003-T` (`7bd513b`, `internal/capture`, `internal/transcript`)**: Preserved `# AI Summary` when `CaptureAudio` fails on a Recap and removed 0-byte `.mp3` artifacts after `out.Close()`; honored `DisableAISummary` in `Session.Run`; passed `headerStart` (`cfg.MeetingStart` when non-zero) to `FormatTranscriptFilename`; and redacted `SourceURL` and fallback `MeetingID` URLs to `scheme://host/path` (stripping userinfo, query parameters, fragments, and raw or percent-encoded SharePoint `/:` sharing paths while preserving leaf-package isolation in `internal/transcript`).
* **`026.004-T` (`531a65a`, `internal/cli`, `docs/`)**: Wired `AISummaryOnly`, `DisableAISummary` (`isAISummaryDisabled`), and `auth` `--account` validation in `internal/cli`, updated `redactHelpArg` doc comments (`2925E813`), and updated `docs/ARCHITECTURE.md` and `docs/USER_GUIDE.md`.

## Key Learnings Retained

* **Windows browser teardown safety**: Never run raw-PID `taskkill /PID cmd.Process.Pid` after a background `cmd.Wait()` goroutine may have already reaped the launcher and released its PID; guard `cmd.Process.Kill()` behind `<-procDone` and verify `127.0.0.1:<port>` CDP liveness (`isCDPPortStillAlive`), loopback `LISTENING` state (`parseNetstatListeningPID`), and Chromium process image (`isChromiumProcessImage`) before terminating surviving child processes.
* **Copilot re-review trigger**: When re-requesting Copilot review after pushing a fix commit, use GraphQL `requestReviews(input: { pullRequestId: $prId, botIds: ["BOT_kgDOCnlnWA"] })` rather than the REST `requested_reviewers` endpoint.
