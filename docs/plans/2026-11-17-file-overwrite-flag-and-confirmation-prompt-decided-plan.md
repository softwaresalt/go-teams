---
title: Decided Plan — File overwrite configuration, --overwrite CLI flag, and interactive (Y)es | (N)o confirmation prompt (034-S)
description: Consolidated decisions, invariants, and scope for 034-F / 034-S (archived verbose plan preserved under docs/archive/plans/2026-11-17/)
date: 2026-11-17
deliberation_id: 023-DL
feature: 034-F
shipment: 034-S
source_stash:
  - DF20598F
status: decided
archived_plan: docs/archive/plans/2026-11-17/2026-11-17-file-overwrite-flag-and-confirmation-prompt-plan.md
---

## Decided Architecture

1. **Config and template (`034.001-T`)**: `Overwrite bool` (`yaml:"overwrite,omitempty"`, default `false`) on `config.Config`, `Overwrite *bool` + `FileOverwrite *bool` on `config.MeetingSchedule`, `overwrite` precedence over `file_overwrite` in `config.Load`, and byte-for-byte parity between `internal/config/template.go` and `config.example.yaml`.
2. **Uniform capture overwrite policy and rollback (`034.002-T`)**: Enforce uniform overwrite gating across both `Dir` and `FilePath` modes in `setupOutput` via `os.Lstat` (rejecting symlinks, directories, and non-regular files), `ConfirmOverwrite` / `ErrOverwriteDeclined`, handle-identity verification before `Truncate(0)`, temp-file backup/restore for `mdFile` across `WriteHeader`/`Flush` rollback (preserving backups on restore failure), deferred `mp3File.Truncate(0)`, `Session.Run` context cancellation and `!out.AudioExisted` 0-byte audio cleanup guard, and `Overwrite *bool` on `scheduler.Entry`.
3. **CLI flag, queue inheritance, and prompt (`034.003-T`)**: Bind `--overwrite` on root, `join`, `captions`, and `schedule`, forward `--overwrite` / `--overwrite=false` in `inheritedQueueFlags`, apply 3-tier `schedule` precedence, and wire `promptOverwriteApproval` (`File already exists: <paths>\nOverwrite? (Y)es | (N)o: `) using `bindPromptStreams` (`!isInsideQueueExecution`), non-buffering `readPromptLine`, `promptMu`, and a 15-second `schedule` prompt timeout.
4. **Tests and docs (`034.004-T`, `034.005-T`)**: Unit tests across `internal/config`, `internal/capture`, and `internal/cli`, plus documentation updates in `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md`.
