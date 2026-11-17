---
title: Safe pre-existing file overwrite rollback, Windows Lstat identity pinning, and non-buffering CLI prompt streams
description: Lessons from implementing uniform capture overwrite protection, TOCTOU-safe temp backup/restore, and --queue-safe interactive prompts in 034-S
source: docs/compound/runtime-errors/setupoutput-overwrite-backup-rollback-and-prompt-stream-2026-11-17.md
doc_type: learning
problem_type: runtime-error
category: runtime-errors
component: internal/capture, internal/cli
root_cause: Overwriting pre-existing files with O_TRUNC before header flush destroys original contents on rollback, Windows os.Lstat loads file IDs lazily inside os.SameFile, and bufio.NewReader per prompt discards queued lines
resolution_type: code_fix
severity: high
message: "capture: flush transcript header / prompt stream busy after cancelled prompt"
file_path: internal/capture/output.go
citations:
  - docs/closure/034-S-034-F-post-merge-closure.md
  - docs/plans/2026-11-17-file-overwrite-flag-and-confirmation-prompt-decided-plan.md
  - PR #82 (merge commit 913775a22c0e120e8bec967f7585626c34d2685a)
tags:
  - overwrite
  - rollback
  - windows
  - toctou
  - cli-prompt
  - queue
---

## Problem

Three subtle runtime hazards surfaced during adversarial and Copilot review of shipment `034-S`:

1. **Pre-existing content loss on `setupOutput` header-flush failure**: Opening pre-existing `.md`/`.mp3` files with `O_TRUNC` before `WriteHeader`/`Flush` destroyed original bytes even when `rollbackPair` skipped `os.Remove(mdPath)`.
2. **Windows lazy file-ID loading and TOCTOU symlink race**: `os.Lstat` on Windows populates `vol`/`idxhi`/`idxlo` lazily inside `os.SameFile`, so `preInfo` was not pinned before `ConfirmOverwrite`.
3. **Buffered stdin lookahead and mutex release races across `--queue`**: Creating a fresh `bufio.NewReader(in)` per prompt discarded trailing lines (breaking `"y\ny\n"`), while unlocking `promptMu` via `defer` when `ctx.Done()` fired early allowed concurrent `in.Read` calls while the previous reader goroutine was still blocked.

## Root Cause

* `O_TRUNC` truncates immediately at open time before header serialization/flush completes, and discarding restore errors or deleting backup files on failure strands truncated targets.
* `bufio.NewReader` reads up to 4096 bytes ahead from `io.Reader` and drops unread bytes on return.
* `defer promptMu.Unlock()` ties lock lifetime to the caller rather than the goroutine owning `in.Read`.

## Resolution

* **Backup-before-truncate and defer MP3 truncation (`internal/capture/output.go`)**: Verify `os.Lstat` (`!symlink && IsRegular`), pin Windows file IDs immediately via `os.SameFile(info, info)`, open existing files with `O_RDWR` (no `O_TRUNC`) after verifying `f.Stat()` matches `preInfo`, back up `mdFile` to an owner-only temp file before `mdFile.Truncate(0)`, defer `mp3File.Truncate(0)` until after `w.WriteHeader` and `w.Flush` succeed, and preserve the backup file on disk (surfacing its path in the error) if `restoreFromTemp` fails.
* **Non-buffering `readPromptLine` and reader-owned unlock (`internal/cli/root.go`)**: Read one byte at a time through `\n` (draining excess bytes > 4096 through `\n`), bind outer `promptIn`/`promptOut` on `deps` only when `!isInsideQueueExecution(cmd.Context())`, and unlock `d.promptMu` inside the reader goroutine immediately before sending the result.

## Prevention

* Never combine `O_TRUNC` with fallible setup writes on pre-existing user files without a verified backup/restore or atomic staging path.
* Never construct transient `bufio.NewReader` instances around shared `io.Reader` streams in `--queue` loops, and never release a reader mutex in a caller `defer` when the underlying `Read` goroutine may outlive `<-ctx.Done()`.
