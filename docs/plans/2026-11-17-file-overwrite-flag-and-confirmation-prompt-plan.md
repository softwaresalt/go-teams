---
title: File overwrite configuration, --overwrite CLI flag, and interactive (Y)es | (N)o confirmation prompt implementation plan
description: Implementation plan for 034-F / 034-S covering stash entry DF20598F
source: docs/decisions/2026-11-17-file-overwrite-flag-and-confirmation-prompt-deliberation.md
deliberation_id: 023-DL
feature: 034-F
shipment: 034-S
source_stash:
  - DF20598F
date: 2026-11-17
status: approved
tags:
  - config
  - cli
  - capture
  - scheduler
  - overwrite
  - prompt
---

## Problem Frame

Following the accepted deliberation in [2026-11-17-file-overwrite-flag-and-confirmation-prompt-deliberation.md](../decisions/2026-11-17-file-overwrite-flag-and-confirmation-prompt-deliberation.md) (`023-DL`), stash entry **`DF20598F`** (`priority: high`, `kind: feature`) is planned under covering feature **`034-F`** and shipment **`034-S`**, sequenced immediately after `033-S` and before `010-S` in the shipment dependency DAG **`033-S -> 034-S -> 010-S`**.

Currently, `setupOutput` in `internal/capture/output.go` handles existing output files inconsistently across directory mode (`OutputConfig.Dir`) and explicit file mode (`OutputConfig.FilePath`):

1. **In `Dir` mode**: `setupOutput` opens `mdPath` and `mp3Path` with `os.O_CREATE | os.O_WRONLY | os.O_EXCL`, failing immediately when a same-day transcript or audio file already exists, with no `overwrite` configuration key, `--overwrite` CLI flag, or interactive `(Y)es | (N)o` confirmation prompt.
2. **In `FilePath` mode**: `setupOutput` opens `mdPath` and `mp3Path` with `os.O_CREATE | os.O_WRONLY | os.O_TRUNC`, silently truncating existing files even when the operator did not request overwriting.
3. **Rollback & 0-byte audio cleanup hazards for pre-existing files**: When `mdPath` or `mp3Path` already existed on disk prior to `setupOutput`, calling `os.Remove(mdPath)` on a subsequent `WriteHeader`, `Flush`, or `mp3Path` open error (or calling `os.Remove(out.AudioPath)` on a 0-byte audio error in `Session.Run`) would delete a pre-existing file rather than only cleaning up newly created files.

### Explicit Out-of-Scope Boundaries

* Active stash entries `9BDFB275`, `560B0C44`, `188D4FBB`, `232C6844`, `AD451E00`, `3F6BA819`, `39450846`, `7883544E`, `59545A24`, `C3460FDC`, `4AB8EAEE`, `E03CB3C3`, `4C40AF2B`, `BA069506`, `196AB9E4`, `69AB7ACC`, and `0DE2E900` remain untouched in `active` stash state.
* `docs/CLI_REFERENCE.md` is not created because `go-teams` maintains its complete command and flag reference inside `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md`.
* No external dependencies (`go.mod` / `go.sum`) are added or modified.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Add `Overwrite bool` (`yaml:"overwrite,omitempty"`, default `false`, plus `file_overwrite` alias) to `config.Config`, add `Overwrite *bool` (`yaml:"overwrite,omitempty"`, plus `file_overwrite` alias) to `config.MeetingSchedule`, and document `# overwrite: false` and `#   Alias: file_overwrite` in `internal/config/template.go` and `config.example.yaml` | `DF20598F` | U1 (`034.001-T`), U4 (`034.004-T`) | `go test -race ./internal/config/...` |
| R2: Define `ErrOverwriteDeclined` in `internal/capture/output.go`, add `Overwrite` and `ConfirmOverwrite` to `OutputConfig` and `SessionConfig`, track `AudioExisted` (`mdExisted` / `mp3Existed`) so pre-existing files are preserved byte-for-byte when declined/unconfirmed and never deleted (`os.Remove`) on post-open rollback or 0-byte audio error, enforce uniform overwrite/prompt/decline policy in `setupOutput` across both `Dir` and `FilePath` modes, check `isShutdown(ctx, ctx.Err())` before `setupOutput` in `Session.Run`, and add `Overwrite *bool` to `scheduler.Entry` (`internal/scheduler/schedule.go`) | `DF20598F` | U2 (`034.002-T`), U4 (`034.004-T`) | `go test -race ./internal/capture/... ./internal/scheduler/...` |
| R3: Bind `--overwrite` on root, `join`, `captions` (`captureFlags`), and `schedule` (`scheduleFlags`), apply in `applyCaptureFlags` and `applyScheduleFlags`, forward in `inheritedQueueFlags`, propagate `Overwrite` in `buildScheduleEntryConfig` and `buildSchedulerEntryFromConfig`, and wire interactive `promptOverwriteApproval` (`"File already exists: %s. Overwrite? (Y)es \| (N)o: "`) via `d.promptIn` / `d.promptOut` on `deps` (bound from `cmd.InOrStdin()` / `cmd.ErrOrStderr()` when `!isInsideQueueExecution(cmd.Context())`) | `DF20598F` | U3 (`034.003-T`), U4 (`034.004-T`) | `go test -race ./internal/cli/...` |
| R4: Add unit tests across `internal/config/config_test.go`, `internal/capture/output_test.go`, and `internal/cli/join_test.go` and update `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md` | `DF20598F` | U4 (`034.004-T`), U5 (`034.005-T`) | `go test -race ./...`, `go vet ./...`, `golangci-lint run` |

## Implementation Units

### U1 (`034.001-T`) — Add `overwrite` (`file_overwrite` alias) to `config.Config`, `MeetingSchedule`, `template.go`, and `config.example.yaml` (`internal/config`)

* **Domain:** code (`internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`). **Posture:** test-first (Constitution Principle II Wave-Staged Red-Phase Harness exception for width-isolated tasks: stage U4's `internal/config/config_test.go` red-phase tests in `logs/staged-harnesses/034.004-T/` before editing `internal/config/config.go`, then activate in U4; existing `TestTemplateYAMLContentAndLoad` and `TestExampleConfigMatchesTemplateYAML` in `internal/config/template_test.go` also verify template parity immediately). **Size:** `S` | **Complexity:** `low`.
* **Files (3 files):** `internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`.
* **What:**
  1. **In `internal/config/config.go`**:
     * Add `Overwrite *bool` (`yaml:"overwrite,omitempty"`) to `MeetingSchedule`:
       ```go
       CaptureAudio   *bool  `yaml:"capture_audio,omitempty"`
       Audio          *bool  `yaml:"audio,omitempty"`
       Overwrite      *bool  `yaml:"overwrite,omitempty"`
       OutputDir      string `yaml:"output_dir,omitempty"`
       ```
     * Add `Overwrite bool` (`yaml:"overwrite,omitempty"`) to `Config` (keeping `Config` strictly limited to configuration data without runtime I/O streams):
       ```go
       Headless             bool              `yaml:"headless"`
       CaptureAudio         bool              `yaml:"capture_audio,omitempty"`
       Overwrite            bool              `yaml:"overwrite,omitempty"`
       ```
     * In `Load(path string)`:
       * Extend `aliases` struct with `Overwrite *bool` (`yaml:"overwrite"`), `FileOverwrite *bool` (`yaml:"file_overwrite"`), and inside `aliases.Meetings` add `Overwrite *bool` (`yaml:"overwrite"`) and `FileOverwrite *bool` (`yaml:"file_overwrite"`).
       * Resolve top-level `Overwrite` (`overwrite` wins over `file_overwrite`):
         ```go
         if aliases.Overwrite != nil {
             cfg.Overwrite = *aliases.Overwrite
         } else if aliases.FileOverwrite != nil {
             cfg.Overwrite = *aliases.FileOverwrite
         }
         ```
       * Inside `for i := range cfg.Meetings`, resolve per-meeting `Overwrite` (`overwrite` wins over `file_overwrite`):
         ```go
         if i < len(aliases.Meetings) {
             if aliases.Meetings[i].Overwrite != nil {
                 val := *aliases.Meetings[i].Overwrite
                 cfg.Meetings[i].Overwrite = &val
             } else if aliases.Meetings[i].FileOverwrite != nil {
                 val := *aliases.Meetings[i].FileOverwrite
                 cfg.Meetings[i].Overwrite = &val
             }
         }
         ```
  2. **In `internal/config/template.go` (`templateYAML`) and `config.example.yaml`**:
     * Under `# Where transcripts are saved` (immediately after `# category: "TEAM"`):
       ```yaml
       # Replace existing transcript (.md) and audio (.mp3) files without prompting
       # when true; when false (the default), go-teams prompts "(Y)es | (N)o" before
       # overwriting an existing file.
       #   Alias: file_overwrite
       # overwrite: false
       ```
     * Under `# Scheduled meetings for "go-teams schedule"`:
       * Update the comment to list `overwrite (alias: file_overwrite)` among the optional per-meeting settings:
         ```yaml
         # Optional per-meeting settings: mode (alias: capture_mode), duration,
         # idle_timeout, ai_summary_delay, lobby_timeout, category, capture_audio
         # (alias: audio), overwrite (alias: file_overwrite), and output_dir. They
         # override the global settings above.
         ```
       * Add `#     overwrite: false` under `#   - name: "Daily Standup"` in the `# meetings:` example block.
     * Ensure `config.example.yaml` and `templateYAML` in `internal/config/template.go` are byte-for-byte identical (LF-normalized).
* **Verify:** Run `go test -race ./internal/config/...`, `go vet ./internal/config/...`, and `golangci-lint run ./internal/config/...`.
* **Depends on:** none.

### U2 (`034.002-T`) — Enforce uniform overwrite policy, `ConfirmOverwrite` prompt callback, and pre-existing file preservation in `setupOutput`, `Session.Run`, and `scheduler.Entry` (`internal/capture`, `internal/scheduler`)

* **Domain:** code (`internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go`). **Posture:** test-first (Constitution Principle II Wave-Staged Red-Phase Harness exception for width-isolated tasks: stage U4's `internal/capture/output_test.go` red-phase tests in `logs/staged-harnesses/034.004-T/` before editing `internal/capture/output.go`). **Size:** `M` | **Complexity:** `medium`.
* **Depends on:** `034.001-T` (`blocks`).
* **Files (3 files):** `internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go`.
* **What:**
  1. **In `internal/capture/output.go`**:
     * Define sentinel error:
       ```go
       // ErrOverwriteDeclined is returned when one or more target output files
       // already exist, Overwrite is false, and the operator declines the
       // overwrite confirmation prompt.
       var ErrOverwriteDeclined = errors.New("capture: file overwrite declined")
       ```
     * Add `Overwrite bool` and `ConfirmOverwrite func(paths []string) (bool, error)` to `OutputConfig`.
     * Add `AudioExisted bool` to `Output`.
     * In `setupOutput(cfg OutputConfig)`:
       * Resolve `mdPath` and `mp3Path` and create parent directories (`os.MkdirAll(..., dirPerm)`) for both `FilePath != ""` and `Dir` modes (without hardcoding `O_TRUNC` for `FilePath`).
       * Keep the `!cfg.CaptionsOnly` `strings.EqualFold(mdPath, mp3Path)` and `os.SameFile` collision checks before any existence/prompt or `os.OpenFile` call.
       * Stat `mdPath` and (when `!cfg.CaptionsOnly`) `mp3Path` before opening any file:
         * If `statErr == nil && info.IsDir()`: return `fmt.Errorf("capture: output path is a directory: %s", p)` (catching directory targets for both `mdPath` and `mp3Path` before opening either file).
         * If `statErr != nil && !errors.Is(statErr, os.ErrNotExist)`: return `fmt.Errorf("capture: stat %s: %w", p, statErr)`.
         * Record `mdExisted` and `mp3Existed`, and append existing paths in deterministic order (`mdPath`, then `mp3Path` when `!cfg.CaptionsOnly`) to `existingPaths`.
       * When `len(existingPaths) > 0 && !cfg.Overwrite`:
         * If `cfg.ConfirmOverwrite == nil`: return `fmt.Errorf("capture: target file already exists (use --overwrite or overwrite: true): %s: %w", existingPaths[0], os.ErrExist)` without opening, truncating, or deleting any file.
         * Call `approved, err := cfg.ConfirmOverwrite(existingPaths)`.
         * If `err != nil`: return `fmt.Errorf("capture: confirm overwrite: %w", err)` without modifying any file.
         * If `!approved`: return `fmt.Errorf("%w: %s", ErrOverwriteDeclined, strings.Join(existingPaths, ", "))` without modifying any file.
       * Open `mdPath` with `os.O_CREATE | os.O_WRONLY | os.O_TRUNC` when `mdExisted`, else `os.O_CREATE | os.O_WRONLY | os.O_EXCL`.
       * If `w.WriteHeader(...)`, `w.Flush()`, or `os.OpenFile(mp3Path, ...)` fails after opening `mdPath`, close `mdFile` and only call `_ = os.Remove(mdPath)` when `!mdExisted` (never deleting a pre-existing `mdPath` pathname).
       * When `!cfg.CaptionsOnly`, open `mp3Path` with `os.O_CREATE | os.O_WRONLY | os.O_TRUNC` when `mp3Existed`, else `os.O_CREATE | os.O_WRONLY | os.O_EXCL`, and populate `AudioExisted: mp3Existed` on the returned `&Output{...}`.
  2. **In `internal/capture/session.go`**:
     * Add `Overwrite bool` and `ConfirmOverwrite func(paths []string) (bool, error)` to `SessionConfig`.
     * In `Session.Run`, check `if isShutdown(ctx, ctx.Err()) { return nil }` before invoking `s.setupOutput(outCfg)` so a context cancelled as `Join` finishes never prompts or opens output files, and pass `Overwrite: s.cfg.Overwrite` and `ConfirmOverwrite: s.cfg.ConfirmOverwrite` into `outCfg`.
     * In `Session.Run`'s deferred cleanup, guard the 0-byte audio removal with `!out.AudioExisted`:
       ```go
       if hadAudioErr && out.AudioPath != "" && !out.AudioExisted {
           if info, statErr := os.Stat(out.AudioPath); statErr == nil && info.Size() == 0 {
               _ = os.Remove(out.AudioPath) // Best-effort removal of empty 0-byte audio artifact after failed audio capture.
           }
       }
       ```
  3. **In `internal/scheduler/schedule.go`**:
     * Add `Overwrite *bool` to `scheduler.Entry`.
* **Verify:** Run `go build ./...`, `go vet ./...`, `go test -race ./internal/scheduler/...`, and `go test -race ./internal/capture/... -skip TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` (under the Constitution Principle II Wave-Staged Red-Phase Harness exception for width-isolated tasks, `internal/capture/output_test.go` is updated in U4 `034.004-T`).

### U3 (`034.003-T`) — Wire `--overwrite` CLI flag, queue inheritance, schedule overrides, and interactive `(Y)es | (N)o` prompt (`internal/cli`)

* **Domain:** code (`internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`). **Posture:** test-first (Constitution Principle II Wave-Staged Red-Phase Harness exception for width-isolated tasks: stage U4's `internal/cli/join_test.go` red-phase tests in `logs/staged-harnesses/034.004-T/` before editing `internal/cli/`). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `034.001-T` (`blocks`), `034.002-T` (`blocks`).
* **Files (3 files):** `internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`.
* **What:**
  1. **In `internal/cli/root.go`**:
     * Import `"bufio"` and `"io"`.
     * Add `promptIn io.Reader` and `promptOut io.Writer` fields to `deps` and a helper `bindPromptStreams(cmd *cobra.Command, d *deps)`:
       ```go
       func bindPromptStreams(cmd *cobra.Command, d *deps) {
           if cmd == nil || d == nil || isInsideQueueExecution(cmd.Context()) {
               return
           }
           d.promptIn = cmd.InOrStdin()
           d.promptOut = cmd.ErrOrStderr()
       }
       ```
       Calling `bindPromptStreams(cmd, d)` in `root.RunE`, `join.RunE`, `captions.RunE`, and `schedule.RunE` before `runQueueFile` or `runCapture` keeps `config.Config` free of runtime I/O fields AND ensures that when `executeQueueLine` (`internal/cli/queue.go`) executes a queued line with `isInsideQueueExecution(ctx) == true` and `SetErr(io.Discard)`, `d.promptIn` and `d.promptOut` retain the outer command's `InOrStdin()` and `ErrOrStderr()` streams.
     * Implement `promptOverwriteApproval(in io.Reader, out io.Writer, paths []string) (bool, error)`:
       ```go
       func promptOverwriteApproval(in io.Reader, out io.Writer, paths []string) (bool, error) {
           if in == nil {
               in = os.Stdin
           }
           if out == nil {
               out = os.Stderr
           }
           if _, err := fmt.Fprintf(out, "File already exists: %s. Overwrite? (Y)es | (N)o: ", strings.Join(paths, ", ")); err != nil {
               return false, err
           }
           line, err := bufio.NewReader(in).ReadString('\n')
           if err != nil && !errors.Is(err, io.EOF) {
               return false, err
           }
           switch strings.ToLower(strings.TrimSpace(line)) {
           case "y", "yes", "(y)es":
               return true, nil
           default:
               return false, nil
           }
       }
       ```
       When `in` is at `io.EOF` (such as unattended daemon or batch execution with closed stdin `/dev/null` or `NUL`), `ReadString('\n')` returns `io.EOF` immediately without blocking, returning `false, nil` (`ErrOverwriteDeclined`) and releasing `Session.Run` and the scheduler's `busy` gate immediately.
     * In `defaultDeps()`:
       initialize `d := &deps{...}` and wire `d.newSession` to pass `Overwrite: c.Overwrite` and `ConfirmOverwrite`:
       ```go
       Overwrite: c.Overwrite,
       ConfirmOverwrite: func(paths []string) (bool, error) {
           return promptOverwriteApproval(d.promptIn, d.promptOut, paths)
       },
       ```
  2. **In `internal/cli/join.go`**:
     * Add `overwrite bool` to `captureFlags`.
     * In `bindCaptureFlags(cmd *cobra.Command, f *captureFlags)`:
       ```go
       cmd.Flags().BoolVar(&f.overwrite, "overwrite", false, "overwrite existing transcript and audio files without prompting")
       ```
     * In `applyCaptureFlags(cmd *cobra.Command, cfg *config.Config, f captureFlags)`:
       ```go
       if cmd != nil && cmd.Flags().Changed("overwrite") {
           cfg.Overwrite = f.overwrite
       } else if f.overwrite {
           cfg.Overwrite = true
       }
       ```
     * In `inheritedQueueFlags(cmd *cobra.Command, configPath string, f captureFlags)`:
       ```go
       if cmd != nil && cmd.Flags().Changed("overwrite") {
           if f.overwrite {
               out = append(out, "--overwrite")
           } else {
               out = append(out, "--overwrite=false")
           }
       }
       ```
     * Call `bindPromptStreams(cmd, d)` at the start of `newJoinCommand`'s `RunE` and `newCaptionsCommand`'s `RunE` (as well as `newRootCommand`'s `RunE` in `root.go`).
  3. **In `internal/cli/schedule.go`**:
     * Add `overwrite bool` to `scheduleFlags`.
     * In `bindScheduleFlags(cmd *cobra.Command, f *scheduleFlags)`:
       ```go
       cmd.Flags().BoolVar(&f.overwrite, "overwrite", false, "overwrite existing transcript and audio files without prompting")
       ```
     * In `applyScheduleFlags(cmd *cobra.Command, cfg *config.Config, f scheduleFlags)`:
       ```go
       if cmd != nil && cmd.Flags().Changed("overwrite") {
           cfg.Overwrite = f.overwrite
       } else if f.overwrite {
           cfg.Overwrite = true
       }
       ```
     * In `buildScheduleEntryConfig(base config.Config, e scheduler.Entry) config.Config`:
       ```go
       if e.Overwrite != nil {
           entryCfg.Overwrite = *e.Overwrite
       }
       ```
     * In `buildSchedulerEntryFromConfig(m config.MeetingSchedule) (scheduler.Entry, error)`:
       populate `Overwrite: m.Overwrite` on the returned `scheduler.Entry`.
     * Call `bindPromptStreams(cmd, d)` at the start of `newScheduleCommand`'s `RunE`.
* **Verify:** Run `go test -race ./internal/cli/...`, `go vet ./internal/cli/...`, and `golangci-lint run ./internal/cli/...`.

### U4 (`034.004-T`) — Add unit tests for `overwrite` config/aliases, `setupOutput`/`Session.Run` overwrite & rollback safety, and CLI `--overwrite` & interactive prompt (`internal/config`, `internal/capture`, `internal/cli`)

* **Domain:** test (`internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go`). **Posture:** test-first (activate the Wave-Staged Red-Phase Harnesses from `logs/staged-harnesses/034.004-T/`). **Size:** `S` | **Complexity:** `low`.
* **Depends on:** `034.001-T` (`blocks`), `034.002-T` (`blocks`), `034.003-T` (`blocks`).
* **Files (3 files):** `internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go`.
* **What:**
  1. **In `internal/config/config_test.go`**:
     * Add `TestLoadOverwriteAndAliasesAndMeetings` verifying:
       * `DefaultConfig()` defaults `Overwrite` to `false`.
       * Top-level `overwrite: true`, `overwrite: false`, `file_overwrite: true`, and `overwrite` precedence over `file_overwrite` when both are present.
       * Per-meeting `meetings[].overwrite` (`true` and `false`), `meetings[].file_overwrite` alias, and `meetings[].overwrite` precedence over `meetings[].file_overwrite`.
       * `Save` and `Load` round-trip preserving `Overwrite` and `Meetings[].Overwrite`.
  2. **In `internal/capture/output_test.go`**:
     * Update `TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` to verify that a second `setupOutput` call on an existing explicit `FilePath` refuses to overwrite (`errors.Is(err, os.ErrExist)`) when `Overwrite: false` and `ConfirmOverwrite == nil` (leaving the first file's content untouched), and overwrites when `Overwrite: true`.
     * Add `TestSetupOutputOverwriteAndConfirmPrompt` covering:
       * `Overwrite: true` in both `Dir` and `FilePath` modes overwrites existing `.md` and `.mp3` files without invoking `ConfirmOverwrite`.
       * `Overwrite: false` with `ConfirmOverwrite` returning `true, nil` passes `existingPaths` in deterministic order (`[mdPath]`, `[mp3Path]`, or `[mdPath, mp3Path]`) and overwrites the existing files.
       * `Overwrite: false` with `ConfirmOverwrite` returning `false, nil` returns `ErrOverwriteDeclined` (`errors.Is(err, ErrOverwriteDeclined)`) and preserves existing `.md` and `.mp3` contents byte-for-byte.
       * `Overwrite: false` with `ConfirmOverwrite` returning an error wraps and returns that error without modifying existing files.
       * Pre-open directory check and post-open rollback preservation: when `mp3Path` is a directory, `setupOutput` fails during the pre-open `os.Stat` check leaving a pre-existing `mdPath` byte-for-byte untouched; when `mdPath` pre-existed (`mdExisted == true`) and `Overwrite: true`, if a post-open failure occurs, `setupOutput` does NOT delete (`os.Remove`) `mdPath`; when `mdPath` did not pre-exist (`mdExisted == false`) and a post-open failure occurs, `setupOutput` removes the newly created `mdPath`.
       * `Session.Run` 0-byte audio preservation: when `mp3Path` pre-existed as a 0-byte file (`AudioExisted == true`, `Overwrite: true`) and `CaptureAudio` fails, `Session.Run`'s deferred cleanup preserves `mp3Path` on disk; when `mp3Path` was newly created (`AudioExisted == false`) and `CaptureAudio` fails leaving 0 bytes, `Session.Run` removes the 0-byte `mp3Path`.
  3. **In `internal/cli/join_test.go`**:
     * Create `internal/cli/join_test.go` with:
       * `TestPromptOverwriteApproval`: table-driven tests for `"y\n"`, `"Y\n"`, `"yes\r\n"`, `"YES\n"`, `"(Y)es\n"`, `"n\n"`, `"N\n"`, `"no\n"`, `"(N)o\n"`, `""` (`io.EOF`), `"other\n"`, and prompt output `"File already exists: a.md, a.mp3. Overwrite? (Y)es | (N)o: "`.
       * `TestOverwriteFlagAndConfigPrecedenceAndQueueInheritance`: verifies `--overwrite` and `--overwrite=false` across root, `join`, `captions`, and `schedule`, `inheritedQueueFlags` forwarding (`--overwrite` and `--overwrite=false` when changed, omitted when unchanged), and `buildSchedulerEntryFromConfig` / `buildScheduleEntryConfig` per-meeting `Overwrite` override precedence.
       * `TestCLIJoinInteractiveOverwritePromptAndDecline`: exercises `newRootCommand(d)` with `defaultDeps().newSession`, `root.SetIn(...)`, and `root.SetErr(...)` against an existing target `.md` file (including both direct `join` and `--queue` execution), verifying that `"n\n"` (or `io.EOF`) returns an error wrapping `capture.ErrOverwriteDeclined` while preserving the original file content, and `"y\n"` (or `--overwrite`) overwrites the file with the new capture transcript.
* **Verify:** Run `go test -race ./...`, `go vet ./...`, and `golangci-lint run`.

### U5 (`034.005-T`) — Document `overwrite` / `file_overwrite`, `--overwrite`, and the interactive `(Y)es | (N)o` prompt in `docs/USER_GUIDE.md` and `docs/ARCHITECTURE.md` (`docs/`)

* **Domain:** docs (`docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`). **Posture:** documentation update. **Size:** `XS` | **Complexity:** `trivial`.
* **Depends on:** `034.001-T` (`blocks`), `034.002-T` (`blocks`), `034.003-T` (`blocks`).
* **Files (2 files):** `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`.
* **What:**
  1. **In `docs/USER_GUIDE.md`**:
     * Add `--overwrite` to the flag lists under `## Compiled executable and explicit output file` (for root, `captions`, `join`, and `schedule`) and `## Batch queue mode` (`inheritedQueueFlags`).
     * Update the output file collision explanation under `## Compiled executable and explicit output file` to explain that both directory mode and explicit file mode protect existing `.md` and `.mp3` files by default (`overwrite: false`): when a target file already exists, `go-teams` prompts `File already exists: <path>. Overwrite? (Y)es | (N)o: ` on standard error and aborts with `capture: file overwrite declined` without touching the existing file when declined, or replaces existing files without prompting when `--overwrite` or `overwrite: true` (alias `file_overwrite: true`) is set.
     * Add `overwrite` (alias `file_overwrite`) and `meetings[].overwrite` (alias `file_overwrite`) to the configuration reference table and schedule precedence section under `## Scheduled and protocol-driven capture` and `## Configuration`.
  2. **In `docs/ARCHITECTURE.md`**:
     * Concisely update the `internal/capture` entry in `## Domain map` and `### Live join and captions` to note uniform overwrite protection (`Overwrite`, `ConfirmOverwrite`, `ErrOverwriteDeclined`, and preservation of pre-existing files on decline or rollback).
* **Verify:** Run `go test -race ./internal/config/...` and `backlogit doctor`.

## Dependency Graph

```text
034.001-T (U1: internal/config/config.go + template.go + config.example.yaml)
    |
    +---> 034.002-T (U2: internal/capture/output.go + session.go + internal/scheduler/schedule.go)
              |
              +---> 034.003-T (U3: internal/cli/root.go + join.go + schedule.go)
                        |
                        +---> 034.004-T (U4: internal/config/config_test.go + internal/capture/output_test.go + internal/cli/join_test.go)
                        |
                        +---> 034.005-T (U5: docs/USER_GUIDE.md + docs/ARCHITECTURE.md)
```

* **Shipment sequencing DAG**:
  * Remove `010-S -> 033-S` direct edge (`backlogit dep remove 010-S 033-S`).
  * Add `034-S -> 033-S` (`backlogit dep add 034-S 033-S --type blocks` — `033-S` is `shipped`, placing `034-S` in `ready_set`).
  * Add `010-S -> 034-S` (`backlogit dep add 010-S 034-S --type blocks` — sequencing `010-S` after `034-S`).

## Decisions and Rationale

1. **Uniform overwrite policy across `Dir` and `FilePath` modes**:
   * **Rationale**: Previously, `FilePath` mode unconditionally truncated existing files while `Dir` mode unconditionally failed with `O_EXCL`. Checking `os.Stat` for `mdPath` and (when `!cfg.CaptionsOnly`) `mp3Path` before opening any file and gating overwrites on `cfg.Overwrite || cfg.ConfirmOverwrite(existingPaths)` protects existing files in both modes while allowing unattended automation via `--overwrite` or `overwrite: true`.
2. **Pre-open confirmation and `!mdExisted` / `!out.AudioExisted` rollback guards**:
   * **Rationale**: Running `ConfirmOverwrite(existingPaths)` before calling `os.OpenFile` guarantees that declining the prompt (`ErrOverwriteDeclined`), encountering a prompt I/O error, or failing a pre-open `os.Stat` / directory check never truncates or touches any pre-existing `.md` or `.mp3` file (preserving them byte-for-byte). Recording `mdExisted` and `mp3Existed` (`Output.AudioExisted`) ensures that post-open partial-failure rollback in `setupOutput` and 0-byte audio cleanup in `Session.Run` only remove (`os.Remove`) files that were newly created during the current run.
3. **Carrying `promptIn` and `promptOut` on `deps` (`bindPromptStreams` when `!isInsideQueueExecution(cmd.Context())`)**:
   * **Rationale**: Binding `d.promptIn = cmd.InOrStdin()` and `d.promptOut = cmd.ErrOrStderr()` on `deps` in `internal/cli/root.go` (guarded by `!isInsideQueueExecution(cmd.Context())`) keeps `config.Config` strictly limited to configuration data, preserves the `deps.newSession` signature (`func(cfg config.Config, meetingURL string) meetingSession`), and ensures `--queue` execution (`executeQueueLine`, which sets `io.Discard` on inner commands) retains the outer command's prompt streams.

## Risks and Caveats

* **Lightweight 3-Threat Model**:
  1. **Most likely threat**: `TestTemplateYAMLContentAndLoad` or `TestExampleConfigMatchesTemplateYAML` in `internal/config/template_test.go` failing due to drift between `internal/config/template.go` and `config.example.yaml` or an undocumented YAML key on `Config` / `MeetingSchedule`. Mitigated by keeping `templateYAML` and `config.example.yaml` identical and documenting `# overwrite: false` and `#   Alias: file_overwrite` in both top-level and `# meetings:` template sections.
  2. **Highest-impact threat**: Destroying a pre-existing `.md` transcript or `.mp3` audio file when the operator declines the prompt, when `ConfirmOverwrite` is `nil`, or when a subsequent error triggers rollback in `setupOutput` or `Session.Run`. Mitigated by evaluating `Overwrite` / `ConfirmOverwrite` and directory `os.Stat` checks before opening any file and guarding every post-open `os.Remove` rollback path with `!mdExisted` and `!out.AudioExisted`.
  3. **Most subtle threat**: Losing outer `SetIn`/`SetErr` streams during `--queue` execution (`executeQueueLine` sets `SetErr(io.Discard)`) or updating `setupOutput` in `U2` so `FilePath` mode no longer overwrites when `Overwrite: false && ConfirmOverwrite == nil` (which changes the expectation of existing test `TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` in `internal/capture/output_test.go`). Mitigated by binding `d.promptIn` / `d.promptOut` only when `!isInsideQueueExecution(cmd.Context())` in `U3` and updating `TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` in `U4` (`034.004-T`).

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | Compliant — pure Go 1.26, zero `unsafe`, sentinel error `ErrOverwriteDeclined`, all errors checked |
| II. Test-First Development | Compliant — Wave-Staged Red-Phase Harnesses staged in `logs/staged-harnesses/034.004-T/` for U1-U3 and activated in U4 across `internal/config`, `internal/capture`, and `internal/cli` |
| III. Workspace Isolation | Compliant — all test files use `t.TempDir()` |
| IV. CLI Workspace Containment | Compliant — zero writes outside the repository or configured output directory |
| V. Structured Observability | Compliant — structured error wrapping (`ErrOverwriteDeclined`, `os.ErrExist`) with zero unredacted URLs in prompts; mandatory post-merge context compaction (P-020) enforced at closure |
| VI. Single Responsibility | Compliant — zero new dependencies (`go.mod` unchanged); 5 width-isolated tasks (`<= 3` files each) |
| VII. Destructive Command Approval | Compliant — protects existing user transcript and audio files from silent truncation by requiring `--overwrite` / `overwrite: true` or interactive `(Y)es \| (N)o` approval |
| VIII. Explicit Safety Modes | Compliant — `Careful mode` + `Freeze-scope mode` + `Strict-safety` active; `ProposedAction` inventory (`PA-034-1`..`PA-034-5`) recorded in `## Plan Hardening` |
| IX. Git-Friendly Persistence | Compliant — markdown, YAML template, and backlog artifacts are LF-normalized and human-readable |
| X. Agent Context Efficiency | Compliant — targeted reads and bounded command outputs used throughout |
| XI. Merge Commit History Preservation | Compliant — PR merges via `--merge` only (P-009) and rollback uses `git revert -m 1 <merge-sha>` |

## Plan Hardening Signals

* **Public API, schema, or contract change**: **Present** — Adds `Overwrite` (`yaml:"overwrite,omitempty"`, alias `file_overwrite`) to `config.Config` and `config.MeetingSchedule`, `--overwrite` CLI flag to root/`join`/`captions`/`schedule`, `ErrOverwriteDeclined`, `Overwrite`, `ConfirmOverwrite`, and `AudioExisted` to `internal/capture`, and `Overwrite *bool` to `scheduler.Entry`.
* **Security, auth, permission, or compliance-sensitive behavior**: **Absent** — Local filesystem overwrite confirmation only; `filePerm = 0o600` and `dirPerm = 0o700` are preserved unchanged.
* **Migration, backfill, destructive data/config action, or irreversible step**: **Present** — Prevents accidental destructive truncation of existing `.md` and `.mp3` files when `Overwrite` is `false` and preserves pre-existing files on rollback.
* **External integration, operator checkpoint, or external dependency**: **Absent** — Interactive CLI stdin/stderr prompt only; no new external dependencies.
* **High runtime, rollout, or rollback risk**: **Present** — Changes default `FilePath` overwrite behavior in `setupOutput` from unconditional `O_TRUNC` to uniform `Overwrite` / `ConfirmOverwrite` gating.

`Requires plan hardening: yes`

## Runtime Verification and Closure

* **Changed surfaces**:
  * YAML configuration model and template (`internal/config/config.go`, `internal/config/template.go`, `config.example.yaml`)
  * Capture output creation, session lifecycle, and scheduler entry model (`internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go`)
  * CLI flags, queue flag inheritance, schedule entry wiring, and interactive `(Y)es | (N)o` prompt (`internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go`)
  * Unit tests (`internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go`) and documentation (`docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md`)
* **Runtime verification**:
  * Run `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, `gofmt -l .`, and `go test -race ./...`.
* **Operational closure**:
  * Pre-deploy audit checklist, monitoring plan, rollback trigger, releasability evidence contract, and mandatory post-merge context compaction (`compact-context` skill, P-020) recorded in `docs/closure/` during Ship operational closure.

## Plan Hardening

### Hardening Need and Consulted Context

* **Why hardening is required**: This plan introduces a new YAML configuration key (`overwrite` / `file_overwrite`), a new CLI flag (`--overwrite`), a new sentinel error (`capture.ErrOverwriteDeclined`), and a uniform file-overwrite and rollback-preservation policy in `setupOutput` and `Session.Run`.
* **Safety Mode Posture (Principle VIII)**: `Careful mode` + `Freeze-scope mode` (strictly scoped to the 14 files declared across `034.001-T`..`034.005-T`) + `Strict-safety`.
* **Instructions and compound learnings consulted**:
  * `.github/instructions/constitution.instructions.md`
  * `.github/instructions/strict-safety.instructions.md`
  * `.github/instructions/release-observability.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `docs/compound/workflow-issues/linux-test-run-without-wsl-go.md`
  * `docs/compound/security-issues/redact-freeform-cli-args-before-echo.md`

### Protected Invariants

1. **Zero Data Loss for Pre-Existing Files When Declined, Unconfirmed, or Pre-Open Rejected**: When `mdPath` or `mp3Path` already exists on disk and `cfg.Overwrite` is `false`, `setupOutput` MUST NOT open, truncate, or remove any file before `cfg.ConfirmOverwrite(existingPaths)` returns `true, nil`. When `cfg.ConfirmOverwrite == nil`, `ConfirmOverwrite` returns an error, `ConfirmOverwrite` returns `false, nil` (`ErrOverwriteDeclined`), or a pre-open `os.Stat` / directory check fails, all pre-existing files on disk MUST remain byte-for-byte untouched.
2. **Pre-Existing Pathname Preservation on Post-Open Rollback and 0-Byte Audio Error**: In `setupOutput`, if `WriteHeader`, `Flush`, or opening `mp3Path` fails after `mdPath` was opened, `_ = os.Remove(mdPath)` MUST ONLY execute when `!mdExisted` (never deleting a pre-existing `mdPath` pathname). In `Session.Run`'s deferred cleanup, removing a 0-byte `out.AudioPath` on `hadAudioErr` MUST ONLY execute when `!out.AudioExisted`.
3. **Owner-Only File and Directory Permissions (`0o600` / `0o700`)**: `filePerm = 0o600` and `dirPerm = 0o700` in `internal/capture/output.go` and `templateFileMode = 0o600` / `templateDirMode = 0o700` in `internal/config/template.go` MUST remain unchanged.
4. **Template and `config.example.yaml` Parity**: `internal/config/template.go` (`TemplateYAML()`) and `config.example.yaml` MUST remain byte-for-byte identical (after CRLF normalization), every YAML tag on `Config` and `MeetingSchedule` MUST be documented in `TemplateYAML()`, and `uncommentCanonical(TemplateYAML())` MUST load and validate cleanly (`TestTemplateYAMLContentAndLoad` and `TestExampleConfigMatchesTemplateYAML`).
5. **Three-Tier Schedule Precedence & Queue Stream Preservation**: In `go-teams schedule`, top-level `config.yaml` (`overwrite` / `file_overwrite`) establishes the baseline, global `schedule --overwrite` (`--overwrite=false`) overrides the baseline (`applyScheduleFlags`), and per-meeting `meetings[].overwrite` (alias `file_overwrite`) overrides the flag-adjusted base for that meeting (`buildSchedulerEntryFromConfig` -> `buildScheduleEntryConfig`). In `--queue` batch mode, `bindPromptStreams` preserves the outer command's `InOrStdin()` and `ErrOrStderr()` on `deps` across `executeQueueLine` invocations while keeping `config.Config` free of runtime I/O fields.

### Strict-Safety `ProposedAction` Inventory

| Action ID | Summary | Targets | Change Kind | ActionRisk | Approval Required | ActionResult | Rollback |
|---|---|---|---|---|---|---|---|
| `PA-034-1` | Add `Overwrite` (`yaml:"overwrite,omitempty"`, alias `file_overwrite`) to `config.Config`, add `Overwrite *bool` (plus `file_overwrite` alias) to `config.MeetingSchedule`, and document `# overwrite: false` in `internal/config/template.go` and `config.example.yaml` | `internal/config/config.go`, `internal/config/template.go`, `config.example.yaml` | Config schema and template extension | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U1 commit |
| `PA-034-2` | Add `ErrOverwriteDeclined`, `Overwrite`, `ConfirmOverwrite`, and `AudioExisted` to `internal/capture`, enforce uniform overwrite/prompt/decline and `!mdExisted`/`!out.AudioExisted` rollback guards in `setupOutput` and `Session.Run`, and add `Overwrite *bool` to `scheduler.Entry` | `internal/capture/output.go`, `internal/capture/session.go`, `internal/scheduler/schedule.go` | Capture file-safety and scheduler contract update | `high` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U2 commit |
| `PA-034-3` | Bind `--overwrite` on root, `join`, `captions`, and `schedule`, forward `--overwrite` via `inheritedQueueFlags`, propagate per-meeting `Overwrite` in `schedule.go`, and wire `promptOverwriteApproval` (`(Y)es \| (N)o`) via `d.promptIn` / `d.promptOut` on `deps` (`cmd.InOrStdin()` / `cmd.ErrOrStderr()`) | `internal/cli/root.go`, `internal/cli/join.go`, `internal/cli/schedule.go` | CLI flag and interactive prompt wiring | `moderate` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U3 commit |
| `PA-034-4` | Add unit tests for config/alias/meeting overwrite parsing, `setupOutput`/`Session.Run` overwrite/prompt/decline/rollback preservation, and CLI `--overwrite` + interactive `(Y)es \| (N)o` prompt handling | `internal/config/config_test.go`, `internal/capture/output_test.go`, `internal/cli/join_test.go` | Unit test suite expansion | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U4 commit |
| `PA-034-5` | Document `overwrite` / `file_overwrite`, `--overwrite`, and the interactive `(Y)es \| (N)o` confirmation prompt in user guide and architecture docs | `docs/USER_GUIDE.md`, `docs/ARCHITECTURE.md` | Documentation update | `low` | Pre-authorized under `DARK_MODE_ACTIVE` | `planned` | `git revert` of U5 commit |

### Release Observability, Verification, and Rollback Detail

* **Pre-deploy audit checklist**:
  * [ ] `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`, `GOOS=darwin GOARCH=amd64 go vet ./...`, `golangci-lint run`, and `gofmt -l .` pass with zero findings.
  * [ ] `go test -race ./...` passes across all packages, including `TestTemplateYAMLContentAndLoad`, `TestExampleConfigMatchesTemplateYAML`, `TestLoadOverwriteAndAliasesAndMeetings`, `TestSetupOutputOverwriteAndConfirmPrompt`, `TestPromptOverwriteApproval`, `TestOverwriteFlagAndConfigPrecedenceAndQueueInheritance`, and `TestCLIJoinInteractiveOverwritePromptAndDecline`.
  * [ ] `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` passes with zero findings.
* **Monitoring plan (manual observation requirement — no hosted telemetry service)**:
  * **SLIs / key metrics**: (1) `overwrite_protection_and_prompt_test_pass_rate` (100% pass rate across `internal/config`, `internal/capture`, and `internal/cli` unit and E2E tests), (2) `pre_existing_file_preservation_on_decline_or_rollback` (100% byte-for-byte preservation when `Overwrite: false` is declined/unconfirmed or pre-open rejected, and 100% non-deletion of pre-existing pathnames on post-open rollback or 0-byte audio error), and (3) `template_and_example_config_parity` (`TestExampleConfigMatchesTemplateYAML == PASS`).
  * **Owner**: Repository maintainer (`softwaresalt`) / Ship agent.
  * **Validation window**: 7 days following merge to `main`.
  * **Dashboard / query**: `gh run list --workflow=ci.yml` and local `go test -race ./...` execution.
  * **Baseline**: 100% pass rate on `changes` and `test-windows` in CI.
  * **Alert threshold**: `>= 1` test failure, unintended truncation of a pre-existing file when `Overwrite: false` is declined, or CI check failure.
* **Rollback trigger & procedure**:
  * **Trigger**: Any regression in capture output creation, CLI flag parsing, batch queue execution, or CI quality gates on `main`.
  * **Procedure**: Revert the `034-S` merge commit on `main` (`git revert -m 1 <merge-sha>`).
* **Releasability Evidence Contract (for `operational-closure`)**:
  * `READY`: All local quality gates, unit tests, E2E tests, and GitHub Actions PR checks (`changes`, `test-windows`) pass and are recorded in `docs/closure/` (followed by mandatory post-merge context compaction under P-020).
  * `READY_WITH_CONDITIONS`: All local quality gates, unit tests, and PR CI checks pass, with any environment-specific condition documented and verified.
  * `BLOCKED`: Any pre-deploy audit check, unit test, or PR CI check fails.

### Review-Gate Capability and Marker Requirements (P-012)

* Plan review (`plan-review`) MUST dispatch the multi-persona review panel (`Constitution Reviewer`, `Go Reviewer`, `Scope Boundary Auditor`, `Learnings Researcher`, `Architecture Strategist` on anchor route `gpt-6.1-sol`, and `Security Lens Reviewer`), remediate any surfaced findings in the plan and deliberation documents before harvest, and append a `## Plan Review` section carrying literal `dispatch_mode:` and `decision:` lines.
* Unresolved operator decisions blocking execution: **None** (pre-authorized under `DARK_MODE_ACTIVE`).

## Plan Review

```text
dispatch_mode: multi-agent
decision: PASS
```

* **Gate Decision:** `PASS` (all 5 findings from `Architecture Strategist` on anchor route `gpt-6.1-sol` were remediated in-place in `docs/plans/2026-11-17-file-overwrite-flag-and-confirmation-prompt-plan.md` and `docs/decisions/2026-11-17-file-overwrite-flag-and-confirmation-prompt-deliberation.md` prior to harvest; zero P0 findings).
* **Capability Status (P-012):** `TOOL_OK: reviewer-subagent-dispatch`, `TOOL_OK: model-specific-review-routing` (`Architecture Strategist` dispatched on anchor route `gpt-6.1-sol` with `reasoning_effort: high`).
* **Plan Hardening Requirement:** Required (`Requires plan hardening: yes`) and satisfied (`## Plan Hardening` present with Protected Invariants, 3-threat model, `strict-safety` `ProposedAction` / `ActionRisk` table `PA-034-1`..`PA-034-5`, and `release-observability` SLIs, monitoring, rollback, and releasability contract).

### Persona Coverage Table

| Persona | Dispatch Mode | Findings Surfaced | Status After Remediation |
|---|---|---|---|
| `Architecture Strategist` (anchor route `gpt-6.1-sol`) | `subagent` (`multi-agent`) | 0 P0, 2 P1, 3 P2, 0 P3 | Remediated — `PASS` |
| `Security Lens Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Constitution Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Go Reviewer` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Scope Boundary Auditor` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 | `PASS` |
| `Learnings Researcher` | `subagent` (`multi-agent`) | 0 P0/P1/P2/P3 (3 prior learnings verified aligned) | `PASS` |

### Consolidated Findings and Remediations Applied

1. **[P1 — `Architecture Strategist`] Preserve outer prompt streams across `--queue` execution (`executeQueueLine`) (`U3`, `U4`)**:
   * **Finding**: `executeQueueLine` (`internal/cli/queue.go`) constructs a subcommand with `root.SetOut(io.Discard)` and `root.SetErr(io.Discard)`. If prompt streams were read unconditionally from the inner command, `--queue` runs would discard prompt output on stderr and ignore outer `SetIn` / `SetErr` streams.
   * **Remediation**: Store `promptIn` and `promptOut` on `deps` (`internal/cli/root.go`) and bind them via `bindPromptStreams(cmd, d)` only when `!isInsideQueueExecution(cmd.Context())`. Outer command streams are preserved across queued invocations, and U4 (`TestCLIJoinInteractiveOverwritePromptAndDecline`) tests both direct and `--queue` execution.
2. **[P1 — `Architecture Strategist`] Pre-setup context-cancellation check in `Session.Run` and immediate `io.EOF` decline on closed stdin (`U2`, `U3`)**:
   * **Finding**: Ensure a context cancelled as `Join` finishes never enters `setupOutput` / `ConfirmOverwrite`, and document how unattended/daemon runs with closed stdin (`io.EOF`) behave.
   * **Remediation**: Added `if isShutdown(ctx, ctx.Err()) { return nil }` before `s.setupOutput(outCfg)` in `Session.Run` (`U2`) and documented that `promptOverwriteApproval` returns `false, nil` (`ErrOverwriteDeclined`) immediately on `io.EOF` without blocking (`U3`).
3. **[P2 — `Architecture Strategist`] Keep `config.Config` pure configuration data without runtime `io.Reader` / `io.Writer` fields (`U1`, `U3`)**:
   * **Finding**: Carrying `PromptIn` and `PromptOut` on `config.Config` coupled the `internal/config` data model to CLI runtime I/O streams.
   * **Remediation**: Removed `PromptIn` and `PromptOut` from `config.Config` (`U1`) and kept `promptIn` and `promptOut` encapsulated on `deps` inside `internal/cli/root.go` (`U3`).
4. **[P2 — `Architecture Strategist`] Clarify pre-open byte-for-byte preservation vs. post-open non-deletion rollback contract (`U2`, `U4`, `Protected Invariants`)**:
   * **Finding**: Distinguish pre-open rejection/decline (where pre-existing files remain byte-for-byte untouched because neither file has been opened) from post-open rollback after an approved overwrite (where `!mdExisted` and `!out.AudioExisted` prevent `os.Remove` from deleting pre-existing pathnames).
   * **Remediation**: Updated `U2`, `U4`, `Decisions and Rationale`, `Protected Invariants`, and `SLIs` (and the deliberation artifact) to state both guarantees explicitly and verify that pre-open `os.Stat` directory checks run on both `mdPath` and `mp3Path` before opening either file.
5. **[P2 — `Architecture Strategist`] Document Constitution Principle II Wave-Staged Red-Phase Harness rationale for width-isolated tasks (`U1`..`U4`)**:
   * **Finding**: Explain why `U1`..`U3` (`code` domain) and `U4` (`test` domain) are width-isolated into separate tasks with `TestSetupOutputExplicitFilePathCreatesAndOverwritesMarkdownFile` updated in `U4`.
   * **Remediation**: Explicitly cited the `harvest` skill's Width Isolation rule (`code` OR `test` per task) and Constitution Principle II's Wave-Staged Red-Phase Harness exception in `U1`..`U4`.

