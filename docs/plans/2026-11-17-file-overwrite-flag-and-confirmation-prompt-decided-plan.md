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

## Decisions, Rationale, and Rejected Alternatives

* **Chosen Option A (Single unified `overwrite` boolean + `(Y)es | (N)o` prompt across `Dir` and `FilePath` modes)**: Protects existing `.md` and `.mp3` files from silent truncation in both modes while permitting unattended automation via `--overwrite` or `overwrite: true` (alias `file_overwrite: true`).
* **Pre-open confirmation and `!mdExisted` / `!out.AudioExisted` rollback protection**: Running `os.Lstat` and `ConfirmOverwrite` before opening files guarantees byte-for-byte preservation on decline (`ErrOverwriteDeclined`), `nil` callback (`os.ErrExist`), or pre-open errors; backing up `mdFile` prior to `Truncate(0)` and deferring `mp3File.Truncate(0)` until after header flush protects pre-existing content on post-open header/flush failures.
* **Encapsulating `promptIn`/`promptOut` on `deps` (`bindPromptStreams`)**: Keeps `config.Config` strictly limited to configuration data (no runtime `io.Reader`/`io.Writer` fields) and preserves outer streams across `--queue` `executeQueueLine` invocations (`!isInsideQueueExecution`).
* **Rejected Option B (Separate `--overwrite-transcript` and `--overwrite-audio` flags)**: Rejected because each capture owns a coupled `.md`/`.mp3` pair and stash `DF20598F` specifies one boolean flag/key.
* **Rejected Option C (`Dir`-only prompt, silent `FilePath` truncation)**: Rejected because silent truncation in `FilePath` mode causes unintended data loss.

## Protected Invariants

1. **Zero data loss on decline, `nil` callback, or pre-open error**: When `mdPath` or `mp3Path` already exists on disk and `Overwrite` is `false`, `setupOutput` must not open, truncate, or remove any file before `ConfirmOverwrite` returns `true, nil` (first-time captures and explicit `Overwrite: true` skip confirmation).
2. **Pre-existing pathname and rollback protection**: `os.Remove` on post-open rollback executes only when `!mdExisted` / `!mp3Existed`, header-flush rollback restores `mdFile` from backup (preserving backup on disk if restore fails), and `Session.Run` 0-byte audio cleanup executes only when `!out.AudioExisted`.
3. **Owner-only permissions (`0o600` / `0o700`)**: `filePerm` and `dirPerm` remain unchanged, and pre-existing overwritten files retain their existing permissions (`O_RDWR` in-place).
4. **Template parity**: `TemplateYAML()` and `config.example.yaml` remain byte-for-byte identical (LF-normalized).
5. **Three-tier schedule precedence & `--queue` stream preservation**: `config.yaml` baseline -> `schedule` flags -> per-meeting `meetings[].overwrite` (`file_overwrite`), with outer `promptIn`/`promptOut` preserved across `--queue`.

## Scope Boundaries, Risks, and Rollback Triggers

* **Out-of-scope boundary**: Zero changes to `go.mod`/`go.sum`, no `docs/CLI_REFERENCE.md` file created (`USER_GUIDE.md` + `ARCHITECTURE.md` hold CLI docs), and existing active stash items remain untouched.
* **Rollback trigger & procedure**: Any capture file-creation regression, unintended truncation on declined overwrite, or CI failure triggers `git revert -m 1 <merge-sha>` on `main`.

