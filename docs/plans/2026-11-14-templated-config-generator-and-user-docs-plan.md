---
title: Templated YAML configuration generator subcommand and user documentation restructure implementation plan
description: Implementation plan for 024-F / 024-S adding internal/config/template.go, the teams-cli generate-config subcommand, a concise cover-page README.md, and comprehensive non-technical user documentation in docs/USER_GUIDE.md
source: docs/decisions/2026-11-14-templated-config-generator-and-user-docs-deliberation.md
deliberation_id: 013-DL
feature: 024-F
shipment: 024-S
source_stash:
  - D4D4F773
  - AB5E856D
date: 2026-11-14
status: approved
tags:
  - config
  - cli
  - docs
  - ux
  - onboarding
---

## Problem Frame

Following the accepted deliberation in
[2026-11-14-templated-config-generator-and-user-docs-deliberation.md](../decisions/2026-11-14-templated-config-generator-and-user-docs-deliberation.md)
(`013-DL`), stash entries **`D4D4F773`** (`feature`, `medium`) and
**`AB5E856D`** (`feature`, `medium`) are planned under covering feature
**`024-F`** and shipment **`024-S`** (sequenced after `023-S`):

1. **Templated YAML configuration generator (`D4D4F773`)**:
   Users currently must locate and manually copy `config.example.yaml` from the
   source repository, and several configuration keys, aliases, and per-meeting
   schedule overrides in `internal/config/config.go` require cross-referencing
   documentation to configure properly. Adding `TemplateYAML()`,
   `ResolveTemplatePath(outputDir, name string) (string, error)`, and
   `WriteTemplate(path string, force bool) error` in
   `internal/config/template.go` and exposing them via the
   `teams-cli generate-config [output-dir]` subcommand (aliases `init-config`
   and `gen-config`, with `--output-dir`/`-d`, `--name`/`-n`, and `--force`)
   allows any user to generate a complete, self-documenting YAML configuration
   file where every option is commented out and explained in plain language.
2. **Concise cover-page `README.md` and non-technical user guide `docs/USER_GUIDE.md` (`AB5E856D`)**:
   `README.md` currently combines project overview and a 480-line technical
   reference into one file without explaining the core purpose of `teams-cli`
   for legitimate AI agentic workflows or providing a step-by-step onboarding
   walkthrough for non-technical users. Restructuring `README.md` into an
   approachable cover page (prominently stating the legitimate context-capture
   purpose of `teams-cli` for AI agentic work environments, a concise summary of
   capabilities, a 3-step Quick Start starting with
   `teams-cli generate-config`, and clear links to detailed docs) and extracting
   and expanding the full step-by-step setup and complete CLI/configuration
   reference into `docs/USER_GUIDE.md` serves both non-technical users and power
   users without losing any technical detail.

## Requirements Trace

| Requirement | Stash ID | Implementation unit | Verification |
|---|---|---|---|
| R1: Define `ErrConfigExists` and `ErrInvalidConfigName`, `TemplateYAML() string`, `ResolveTemplatePath(outputDir, name string) (string, error)`, and `WriteTemplate(path string, force bool) error` in `internal/config/template.go` with `0o700` directory creation, `0o600` file permissions, exclusive creation when `!force`, atomic temp-file + rename when `force`, and path-traversal validation on `name` | `D4D4F773` | U1 (`024.001-T`) | `go vet ./internal/config/...` and `go test -race ./internal/config/...` |
| R2: Add `newGenerateConfigCommand(d *deps) *cobra.Command` (`Use: "generate-config [output-dir]"`, aliases `["init-config", "gen-config"]`, flags `--output-dir`/`-d`, `--name`/`-n`, `--force`, positional argument handling) in `internal/cli/generate_config.go`, wire into `newRootCommand` in `internal/cli/root.go`, and print the resolved path to `cmd.OutOrStdout()` | `D4D4F773` | U2 (`024.002-T`) | `go vet ./internal/cli/...` and `go test -race ./internal/cli/...` |
| R3: Restructure `README.md` into a concise cover page stating the legitimate AI agentic context-capture purpose of `teams-cli`, create `docs/USER_GUIDE.md` with step-by-step non-technical setup/configuration instructions and 100% of the CLI/flag/protocol/queue/lock/config reference, and update `config.example.yaml`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md` | `AB5E856D`, `D4D4F773` | U3 (`024.003-T`) | `go test -race ./...` including `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |

## Implementation Units

### U1 (`024.001-T`) — Implement YAML configuration template generator and path/write helpers in `internal/config`

* **Domain:** code (`internal/config`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. **Red phase (`internal/config/template_test.go`)**: Create table-driven unit
     tests in `package config` first and observe failure before implementing
     `internal/config/template.go`:
     * `TestTemplateYAMLContentAndLoad`:
       - Verify `TemplateYAML()` is non-empty, contains plain-language
         explanatory comments and references for every documented configuration
         key and alias (`output_dir`, `output_folder`, `default_path`,
         `output_file`, `output`, `filename_template`, `name_template`,
         `category`, `meeting_url`, `url`, `profile_dir`, `browser_path`,
         `default_account`, `entra_account`, `account`, `bot_name`, `headless`,
         `headed`, `capture_audio`, `audio`, `capture_mode`, `mode`, `duration`,
         `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `queue_file`,
         `meetings`, `cron`, `at`), has no active (uncommented) YAML lines by
         default, and passes `assert.Empty(t, scanTrackedContent("TemplateYAML", []byte(TemplateYAML())))`.
       - Verify that writing raw `TemplateYAML()` to disk and calling
         `config.Load` succeeds and returns the expected effective defaults
         (`DefaultConfig()` values for `OutputDir`, `ProfileDir`, `BotName`, and
         `FilenameTemplate`, with `Headless == false` because neither `headless`
         nor `headed` is uncommented).
       - Verify that uncommenting the canonical `# <key>:` and `# meetings:`
         example lines in `TemplateYAML()` and calling `config.Load` succeeds,
         populates the exact expected `Config` and `Meetings` fields (both
         recurring `cron` and one-shot `at` entries), and passes `Validate()`.
     * `TestResolveTemplatePath`:
       - Table-test default `""` arguments (`"config.yaml"`), custom
         `outputDir` (including whitespace-padded paths, `%LOCALAPPDATA%`,
         `~/...`, and bare `~`), custom `name` with `.yaml`, `.yml`, `.YAML`,
         `.YML`, and extensionless names (auto-appending `.yaml` without adding
         a duplicate extension to uppercase `.YAML`/`.YML`), and rejection
         (`errors.Is(err, ErrInvalidConfigName)`) of whitespace-only names,
         `"."`, `".."`, `"../evil.yaml"`, `"sub/config.yaml"`,
         `'sub\config.yaml'`, volume-prefixed names, and bare `".yaml"` /
         `".yml"`.
     * `TestWriteTemplate`:
       - Test creation of nested parent directories, refusal to overwrite an
         existing file when `force == false` (`errors.Is(err, ErrConfigExists)`
         with original file bytes preserved), replacement when `force == true`
         (including relative path `"config.yaml"` in the working directory and
         cleanup of `.config-template-*.yaml.tmp` files on error or success),
         and `0o700` directory / `0o600` file mode verification when
         `runtime.GOOS != "windows"`.
  2. **Green phase (`internal/config/template.go`)**: Create
     `internal/config/template.go` in `package config`:
     * Define exported sentinel errors with GoDoc comments:
       - `ErrConfigExists = errors.New("config: configuration file already exists")`
       - `ErrInvalidConfigName = errors.New("config: invalid configuration filename")`
     * Implement `TemplateYAML() string`:
       - Returns a complete, human-friendly YAML configuration template where
         every configuration option supported by `Config`, `MeetingSchedule`,
         and `config.Load` is documented with plain-language inline comments and
         commented-out example values:
         * `output_dir` (and aliases `output_folder`, `default_path`),
           `output_file`, and `output`
         * `filename_template` (and alias `name_template`) with explanations of
           all supported placeholders (`<category>`, `<concat-meeting-name>`,
           `<meeting-name>`, `<meeting-id>`, `<YYYYMMDD>`, `<YYYY-MM-DD>`,
           `<HHMMSS>`, `<timestamp>`)
         * `category`
         * `meeting_url` (and alias `url`)
         * `profile_dir`
         * `browser_path`
         * `default_account` (and aliases `entra_account`, `account`)
         * `bot_name`
         * `headless` (and alias `headed`)
         * `capture_audio` (and alias `audio`)
         * `capture_mode` (and alias `mode`: `auto`, `ephemeral-live`,
           `live-with-summary`, `past-recap`)
         * `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`
         * `queue_file`
         * `meetings` with both a recurring `cron` example and a one-shot `at`
           example (showing `name`, `url`, `cron` / `at`, `mode` /
           `capture_mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
           `lobby_timeout`, `category`, `capture_audio` / `audio`, `output_dir`)
       - Distinguish canonical uncommentable YAML lines (`# <key>: ...` and the
         two `#   - name: ...` entries under `# meetings:`) from explanatory and
         alias-reference comment lines (which use `#   Alias: ...` or prose `# `
         comments so uncommenting the canonical example block never activates
         conflicting overrides like both `headless: true` and `headed: true` or
         both `output_dir` and `output_file`).
     * Implement `ResolveTemplatePath(outputDir, name string) (string, error)`:
       - Default `outputDir` to `"."` when `strings.TrimSpace(outputDir) == ""`;
         otherwise expand `strings.TrimSpace(outputDir)` via `expandUserPath`
         (enhancing `expandUserPath` or `ResolveTemplatePath` so a bare `"~"`
         also expands to `os.UserHomeDir()` when non-empty).
       - When `name == ""` (unset), default `name` to `"config.yaml"`.
       - Trim whitespace (`trimmed := strings.TrimSpace(name)`). Reject with
         `ErrInvalidConfigName` when:
         * `trimmed == ""` (whitespace-only `name` passed explicitly)
         * `trimmed == "."` or `trimmed == ".."`
         * `strings.Contains(trimmed, "..")`
         * `strings.ContainsAny(trimmed, `/\`)`
         * `filepath.VolumeName(trimmed) != ""`
         * `strings.TrimSpace(strings.TrimSuffix(trimmed, filepath.Ext(trimmed))) == ""`
           (e.g., bare `".yaml"` or `".yml"` with no filename stem)
       - If `!strings.EqualFold(filepath.Ext(trimmed), ".yaml") && !strings.EqualFold(filepath.Ext(trimmed), ".yml")`,
         append `".yaml"`.
       - Return `filepath.Join(resolvedDir, trimmed), nil`.
     * Implement `WriteTemplate(path string, force bool) error`:
       - If `strings.TrimSpace(path) == ""`, return `ErrInvalidConfigName`.
       - Resolve `dir := filepath.Dir(path)` and create missing parent
         directories with `0o700` permissions (`os.MkdirAll(dir, 0o700)`).
       - Content is `[]byte(TemplateYAML())`.
       - When `!force`: open `path` with
         `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)`. If
         `errors.Is(err, os.ErrExist)`, return
         `fmt.Errorf("config: write template %s: %w", path, ErrConfigExists)`.
         Write `content`, `Sync()`, and `Close()` (with commented best-effort
         cleanup `_ = f.Close()` and `_ = os.Remove(path)` if write, sync, or
         close fails).
       - When `force`: create a sibling temporary file in `dir` via
         `os.CreateTemp(dir, ".config-template-*.yaml.tmp")`, check
         `tmp.Chmod(0o600)` explicitly, write `content`, `Sync()`, `Close()`,
         and replace `path` via `os.Rename(tmpName, path)` (with commented
         best-effort cleanup `_ = tmp.Close()` and `_ = os.Remove(tmpName)` on
         any error).
* **Files (2 files):** `internal/config/template.go`,
  `internal/config/template_test.go`.
* **Verify:** `go vet ./internal/config/...`,
  `go test -race ./internal/config/...`, and `gofmt -l internal/config`.
* **Depends on:** none.

### U2 (`024.002-T`) — Implement `teams-cli generate-config` subcommand in `internal/cli`

* **Domain:** code (`internal/cli`). **Posture:** test-first. **Size:** `S` | **Complexity:** `low`.
* **What:**
  1. **Red phase (`internal/cli/generate_config_test.go`)**: Create table-driven
     unit tests in `package cli` first and observe failure before implementing
     `internal/cli/generate_config.go`:
     * `TestGenerateConfigCommandFlagsAndPositionalArgs`:
       - Default invocation with `-d <tempDir>` writes `config.yaml`, prints the
         resolved path to `cmd.OutOrStdout()`, and `config.Load` succeeds on the
         generated file.
       - `--output-dir` / `-d` and `--name` / `-n` (including extensionless
         `--name my-team` auto-appending `.yaml` and uppercase `.YAML`/`.YML`).
       - Positional `[output-dir]` argument, positional `.yaml`/`.yml` file path
         argument (`filepath.Join(dir, "custom.yaml")`) splitting directory and
         filename when neither `--output-dir` nor `--name` was explicitly
         passed, positional `.yaml`-named directory that already exists on disk
         (including via env var expansion) treated as `outputDir`, and explicit
         `--output-dir` taking precedence over a positional argument.
       - Aliases `init-config` and `gen-config`, plus verification that root
         persistent `--config` / `-f` is ignored (does not invoke `d.loadConfig`
         or modify the `--config` path).
     * `TestGenerateConfigCommandCollisionAndForce`:
       - Refuses to overwrite an existing file without `--force`
         (`errors.Is(err, config.ErrConfigExists)`) and replaces it when
         `--force` is set.
     * `TestGenerateConfigCommandRejectsInvalidName`:
       - Rejects path traversal in `--name ../escape.yaml` and whitespace-only
         `--name "   "` (`errors.Is(err, config.ErrInvalidConfigName)`).
  2. **Green phase (`internal/cli/generate_config.go` and `internal/cli/root.go`)**:
     * Implement `newGenerateConfigCommand(d *deps) *cobra.Command` in
       `internal/cli/generate_config.go`:
       - `Use: "generate-config [output-dir]"`
       - `Aliases: []string{"init-config", "gen-config"}`
       - `Short: "Generate a commented YAML configuration file template"`
       - `Long`: Explain that `generate-config` writes a fully commented YAML
         configuration file with all options commented out and inline
         setup instructions, defaulting to `./config.yaml`. Note that root
         `--config` / `-f` is not read by `generate-config`; destination is
         controlled exclusively by `--output-dir` (`-d`), `--name` (`-n`), and
         `[output-dir]`.
       - `Args: cobra.MaximumNArgs(1)`
       - Flags:
         * `--output-dir`, `-d`: string, default `"."`, target directory for the
           generated configuration file
         * `--name`, `-n`: string, default `"config.yaml"`, configuration
           filename to generate inside the target directory
         * `--force`: bool, default `false`, overwrite an existing configuration
           file at the resolved destination path
       - Positional argument handling in `RunE`:
         * When `len(args) == 1`:
           - Trim whitespace: `arg := strings.TrimSpace(args[0])`.
           - Resolve any user/env path prefix on `arg` before checking
             `os.Stat` (e.g., via `config.ResolveTemplatePath(arg, "probe.yaml")`
             `filepath.Dir` or equivalent expansion) so an existing directory
             ending in `.yaml`/`.yml` referenced via `%LOCALAPPDATA%` or `~` is
             detected accurately.
           - Let `ext := strings.ToLower(filepath.Ext(arg))`.
           - If `(ext == ".yaml" || ext == ".yml") && !cmd.Flags().Changed("output-dir") && !cmd.Flags().Changed("name")`:
             if the expanded `arg` is not an existing directory on disk, set
             `outputDir = filepath.Dir(arg)` and `name = filepath.Base(arg)`;
             otherwise set `outputDir = arg`.
           - Else if `!cmd.Flags().Changed("output-dir")`:
             set `outputDir = arg`.
       - Call `destPath, err := config.ResolveTemplatePath(outputDir, name)` and
         `config.WriteTemplate(destPath, force)`.
       - Write `destPath` followed by a newline to `cmd.OutOrStdout()` with an
         explanatory comment on the ignored write return:
         `_, _ = fmt.Fprintln(cmd.OutOrStdout(), destPath) // informational stdout output after template is persisted`.
     * Register `newGenerateConfigCommand(d)` in `root.AddCommand(...)` in
       `internal/cli/root.go`.
* **Files (3 files):** `internal/cli/generate_config.go`,
  `internal/cli/root.go`, `internal/cli/generate_config_test.go`.
* **Verify:** `go vet ./internal/cli/...`, `go test -race ./internal/cli/...`,
  and `gofmt -l internal/cli`.
* **Depends on:** U1 (`024.001-T`).

### U3 (`024.003-T`) — Restructure `README.md` as cover page, create `docs/USER_GUIDE.md`, and update `config.example.yaml`, `docs/ARCHITECTURE.md`, and `docs/QUALITY_SCORE.md`

* **Domain:** docs (Markdown & commented YAML example). **Posture:** migration-first. **Size:** `M` | **Complexity:** `low`.
* **What:**
  1. Restructure `README.md` into a concise, welcoming cover page:
     * State the core purpose prominently: `teams-cli` is built **not to
       circumvent meeting security**, but to enable legitimate context capture
       for authorized users in AI agentic work environments — capturing
       requirements, action items, and team/organizational employee impact for
       self-assessments, and building a personal/team communications library so
       AI assistants can synthesize meeting language into actionable outcomes.
     * Provide a concise summary of capabilities:
       - Live Captions Capture
       - Meeting Recap & AI Summary Export
       - Companion MP3 Audio Recording
       - Scheduled & Protocol-Driven Capture (`auto`, `ephemeral-live`,
         `live-with-summary`, `past-recap`)
       - Batch Queue Processing (`--queue`)
       - Templated YAML Configuration Generation (`teams-cli generate-config`)
       - Verified Cross-Platform Self-Update (`teams-cli update`)
     * Provide a 3-step Quick Start:
       1. `teams-cli generate-config` (create a commented `config.yaml`)
       2. `teams-cli auth` (sign in interactively once to cache your browser
          session)
       3. `teams-cli captions "<meeting-or-recap-url>"` (capture live captions
          or export a Meeting Recap)
     * Provide a concise command overview table and prominent links to:
       - [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md) for the complete non-technical
         step-by-step setup guide, command/flag reference, and YAML
         configuration manual
       - [`config.example.yaml`](config.example.yaml) for the commented YAML
         configuration template
       - [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) and
         [`docs/QUALITY_SCORE.md`](docs/QUALITY_SCORE.md) for architecture and
         quality grades
       - Preserve concise signpost subsections (`### Output target precedence`,
         `### Self-update`, `#### Manual lock and backup recovery`, and
         `## Configuration`) that summarize the core rule in 2–4 lines and link
         directly to [`docs/USER_GUIDE.md`](docs/USER_GUIDE.md) so existing
         inbound links from `docs/design-docs/` (`README.md#output-target-precedence`,
         `README.md#self-update`, `README.md#manual-lock-and-backup-recovery`)
         remain valid without broken anchors
       - Brief development commands (`go build`, `go test`, `go vet`,
         `golangci-lint`)
  2. Create `docs/USER_GUIDE.md` (with YAML frontmatter `title:` per
     `markdown.instructions.md` — no H1 heading because `title:` is in
     frontmatter):
     * Written in an approachable, step-by-step style for non-technical users,
       while preserving 100% of the technical depth previously in `README.md`:
       - Why `teams-cli` exists for legitimate AI agentic workflows (capturing
         requirements, action items, self-assessment impact evidence, and
         team/org communication libraries within authorized meetings)
       - Step-by-step Getting Started walkthrough for non-technical users
         (installing or building `teams-cli`, generating `config.yaml` with
         `teams-cli generate-config`, signing in with `teams-cli auth`, and
         running your first live caption or Meeting Recap capture)
       - Complete `teams-cli generate-config` guide (aliases `init-config` and
         `gen-config`, `--output-dir` / `-d`, `--name` / `-n`, `--force`,
         positional directory or `.yaml`/`.yml` file arguments, how to
         uncomment settings, and note that a newly generated all-commented
         `./config.yaml` uses the per-user default transcript directory and
         opens a visible browser window until `output_dir` or `headless: true`
         is uncommented)
       - Complete CLI command and flag reference (`teams-cli`, `captions`,
         `join`, `auth`, `schedule`, `generate-config`, `update`, `--version`)
       - Output target precedence (all 8 levels from positional `arg[1]` down to
         config `output_dir`)
       - Filename templates and all 8 placeholders (`<category>`,
         `<concat-meeting-name>`, `<meeting-name>`, `<meeting-id>`,
         `<YYYYMMDD>`, `<YYYY-MM-DD>`, `<HHMMSS>`, `<timestamp>`)
       - Batch queue mode (`--queue` / `-q`, `::` comment syntax, quoting rules,
         `inheritedQueueFlags`, deferred `--append-ai-summary --not-before`
         entries, and URL redaction)
       - Scheduled and protocol-driven capture (`auto`, `ephemeral-live`,
         `live-with-summary`, `past-recap`, activity-aware nominal `--duration`,
         overtime, and `--idle-timeout` shutdown, `teams-cli schedule` daemon
         with `cron` and `at`, and Windows Task Scheduler PowerShell and
         `schtasks.exe` recipes)
       - Browser security posture and headless automation (`%LOCALAPPDATA%\teams-cli\profile`,
         `~/.config/teams-cli/profile`, `--profile-dir`, `--account` /
         `default_account`)
       - Self-update (`teams-cli update`, `--check`, `--version`, `--force`,
         6-platform `SHA256SUMS` verification, `<exe>.lock` / `<exe>.lock.ctl`
         concurrency protocol, `ErrUpdateLocked`, `ErrConcurrentUpdateModified`,
         `ErrDowngrade`, and step-by-step manual lock and `.old` backup
         recovery)
       - Complete YAML configuration discovery order and key/alias reference
         table (`output_dir`, `output_folder`, `default_path`, `output`,
         `output_file`, `filename_template`, `name_template`, `meeting_url`,
         `url`, `category`, `profile_dir`, `browser_path`, `default_account`,
         `entra_account`, `account`, `bot_name`, `headless`, `headed`,
         `capture_audio`, `audio`, `capture_mode`, `mode`, `duration`,
         `idle_timeout`, `ai_summary_delay`, `lobby_timeout`, `queue_file`,
         `meetings`), noting that every scheduled meeting requires `url` and
         either `cron` or `at` (not both)
  3. Update `config.example.yaml` so its content is byte-identical (after
     LF/CRLF normalization) to `config.TemplateYAML()`.
  4. Update `docs/ARCHITECTURE.md`:
     * Link to `docs/USER_GUIDE.md` alongside `README.md` in `## System overview`.
     * In `## Domain map`, include `generate-config` in `internal/cli` and
       `TemplateYAML`, `ResolveTemplatePath`, `WriteTemplate` in
       `internal/config`.
     * In `## Security and isolation invariants` (`Output permissions`), note
       that `config.WriteTemplate` also creates parent directories with `0o700`
       and generated configuration files with `0o600` (using `O_EXCL` unless
       `--force` is set) and rejects filename path traversal via
       `ErrInvalidConfigName`.
  5. Update `docs/QUALITY_SCORE.md`:
     * Update `internal/cli` and `internal/config` rationales in
       `## Grades by package` to include `generate-config` and
       `TemplateYAML`/`ResolveTemplatePath`/`WriteTemplate` coverage, and note
       `docs/USER_GUIDE.md` in the repository documentation note.
* **Files (5 files, single `docs` domain pass — ~45m human-equivalent effort, no Go code changes):**
  `README.md`, `docs/USER_GUIDE.md`, `config.example.yaml`,
  `docs/ARCHITECTURE.md`, `docs/QUALITY_SCORE.md`.
* **Verify:** Verify `config.example.yaml` matches `config.TemplateYAML()`
  (normalized for LF/CRLF), stage all modified/created files with `git add`, and
  run `go test -race ./...` (including
  `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`).
* **Depends on:** U2 (`024.002-T`).

## Dependency Graph

```text
U1 (024.001-T: internal/config/template.go + template_test.go)
  └──> U2 (024.002-T: internal/cli/generate_config.go + root.go + generate_config_test.go)
         └──> U3 (024.003-T: README.md, docs/USER_GUIDE.md, config.example.yaml, ARCHITECTURE.md, QUALITY_SCORE.md)
```

## Decisions and Rationale

* **Dedicated `internal/config/template.go` with `TemplateYAML()`, `ResolveTemplatePath()`, and `WriteTemplate()`**:
  Keeping the YAML template text, filename traversal validation, and atomic /
  exclusive file writing in `internal/config` preserves the leaf-package boundary
  documented in `docs/ARCHITECTURE.md` (`internal/cli` wires flags and delegates
  config persistence to `internal/config`).
* **All options commented out in `TemplateYAML()`**:
  Because `config.Load` initializes `cfg` from `DefaultConfig()` before
  unmarshaling YAML, a file whose lines are all `#` comments is already a valid
  configuration file that passes `config.Load`, while allowing users to
  selectively uncomment only the settings they wish to customize.
* **Strict `name` validation in `ResolveTemplatePath`**:
  Rejecting `/` and `\`, `..`, volume prefixes, `.` and `..`, bare `.yaml`/`.yml`
  without a stem, and whitespace-only names with `ErrInvalidConfigName` ensures
  `--name` can only ever select a filename inside `outputDir` and can never
  escape `outputDir` via path traversal.
* **Concise cover-page `README.md` + comprehensive `docs/USER_GUIDE.md`**:
  Gives non-technical users an immediate, approachable entry point and clear
  statement of `teams-cli`'s legitimate AI agentic workflow purpose while
  preserving 100% of the deep technical reference in `docs/USER_GUIDE.md`.

## Risks and Caveats

* **Accidental overwrite of an existing `config.yaml`**:
  `WriteTemplate(path, false)` uses `os.O_CREATE|os.O_WRONLY|os.O_EXCL` (`0o600`)
  and returns `ErrConfigExists` if the target file already exists; overwriting
  requires explicit `--force` (`force == true`).
* **Markdown and writing-style lint compliance (`markdown.instructions.md`, `writing-style.instructions.md`)**:
  `docs/USER_GUIDE.md` includes YAML frontmatter with `title:` and therefore
  must start at `##` (no `#` H1 heading), use `*` for unordered lists, include
  language identifiers on all fenced code blocks, and avoid bare URLs.

## Constitution Check

| Principle | Disposition |
|---|---|
| I: Safety-first Go | Standard library only (`errors`, `fmt`, `os`, `path/filepath`, `strings`); exported symbols have GoDoc comments; clean `go vet`, `gofmt`, and `golangci-lint` |
| II: Test first | `internal/config/template_test.go` (`024.001-T`) and `internal/cli/generate_config_test.go` (`024.002-T`) cover template parsing, uncommented parsing, flag/arg combinations, `ErrConfigExists`, and `ErrInvalidConfigName` |
| III, IV: Secrets & containment | `ResolveTemplatePath` rejects path traversal (`ErrInvalidConfigName`); `WriteTemplate` uses `0o700` dir and `0o600` file permissions; all example URLs are synthetic and verified by `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` |
| V: Structured observability | `generate-config` prints the resolved destination path to `cmd.OutOrStdout()` and returns wrapped sentinel errors on failure |
| VI: Single responsibility | Three width-isolated tasks (`024.001-T` in `config`, `024.002-T` in `cli`, `024.003-T` in `docs`) |
| VII, VIII: Strict safety | Filesystem creation/overwrite and CLI contract additions classified in `## Plan Hardening` below under `Freeze-scope mode` (leaving `010-S` and `3EB89E2D` untouched) and `Careful mode` (governing `--force` overwrite and `README.md` reference extraction) |
| IX, X, XI: Persistence & merge history | Deliberation `013-DL`, plan, feature `024-F`, tasks `024.001-T`–`024.003-T`, and shipment `024-S` recorded with full stash traceability (`D4D4F773`, `AB5E856D`); PR merge must use a merge commit (`gh pr merge --merge`, `P-009` / Principle XI — no squash or rebase merge) |

## Plan Hardening Signals (REQUIRED)

* Public API, schema, or contract change — **present**: adds exported functions
  and sentinel errors in `internal/config` (`TemplateYAML`,
  `ResolveTemplatePath`, `WriteTemplate`, `ErrConfigExists`,
  `ErrInvalidConfigName`) and a new CLI subcommand
  (`teams-cli generate-config`, aliases `init-config`, `gen-config`).
* Security, auth, permission, or compliance-sensitive behavior — **present
  (`ActionRisk: low`)**: validates user-supplied `--name` against path traversal
  and writes files to disk with `0o700` directory and `0o600` file permissions
  (`O_EXCL` when `!force`, atomic temp-file + rename when `force`).
* Migration, backfill, destructive data/config action, or irreversible step —
  **present (`ActionRisk: low`)**: `--force` overwrites an existing config file
  atomically when explicitly requested by the user; restructures `README.md` by
  extracting reference sections into `docs/USER_GUIDE.md`.
* External integration, operator checkpoint, or external dependency — **absent**:
  100% local CLI and documentation changes with zero new dependencies.
* High runtime, rollout, or rollback risk — **low**: existing capture, auth,
  schedule, queue, and update commands are unchanged.

Requires plan hardening: yes

## Plan Hardening

Hardening is applied because U1 and U2 add a filesystem-writing CLI subcommand
with user-supplied path/filename inputs and an optional `--force` overwrite mode,
and U3 restructures `README.md` into `README.md` + `docs/USER_GUIDE.md`.

* **Instructions and learnings consulted**:
  * `.github/instructions/go.instructions.md`
  * `.github/instructions/markdown.instructions.md`
  * `.github/instructions/writing-style.instructions.md`
  * `.github/instructions/architecture-doc.instructions.md`
  * `docs/compound/workflow-issues/amend-breaks-shipment-gate.md`
  * `docs/compound/workflow-issues/topology-gate-closure-branch-naming.md`
  * `docs/compound/workflow-issues/copilot-rereview-timing.md`

### Protected Invariants

1. **Path Traversal Rejection on `name` (`ErrInvalidConfigName`)**:
   `ResolveTemplatePath(outputDir, name)` MUST reject any `name` that is
   whitespace-only when non-empty, `"."`, `".."`, contains `".."`, contains `/`
   or `\`, carries a volume name (`filepath.VolumeName`), or has an empty stem
   before `.yaml`/`.yml`.
2. **Fail-Closed Exclusive Creation Without `--force` (`ErrConfigExists`)**:
   When `force == false`, `WriteTemplate` MUST open the target file with
   `os.O_CREATE|os.O_WRONLY|os.O_EXCL` and `0o600` permissions so an existing
   configuration file is never truncated or overwritten, returning an error
   wrapping `ErrConfigExists`.
3. **Owner-Only Permissions (`0o700` / `0o600`) and Safe `--force` Replacement**:
   `WriteTemplate` MUST create missing parent directories with `0o700` and
   write files with `0o600`. When `force == true`, it MUST write to a sibling
   temporary file in the target directory, check `tmp.Chmod(0o600)`, `Sync()`,
   `Close()`, and replace the target path via `os.Rename(tmpName, path)`
   (matching `config.Save` in `internal/config/config.go`), removing the temp
   file on any error.
4. **100% Technical Reference and Inbound Anchor Preservation Across `README.md` and `docs/USER_GUIDE.md`**:
   Every CLI command, flag, output-precedence rule, filename-template
   placeholder, batch-queue rule, capture protocol, schedule/Task Scheduler
   recipe, browser security invariant, self-update locking/recovery procedure,
   and YAML configuration key/alias previously documented in `README.md` MUST
   be preserved in `docs/USER_GUIDE.md`, and `README.md` MUST retain concise
   signpost subsections for `#output-target-precedence`, `#self-update`,
   `#manual-lock-and-backup-recovery`, and `#configuration` so inbound links from
   `docs/design-docs/` remain valid.

### Risky Actions (`strict-safety`)

| ProposedAction | ActionRisk | Approval and containment | Expected ActionResult |
|---|---|---|---|
| Write generated YAML configuration file to disk in `config.WriteTemplate` (`O_EXCL` by default, atomic rename when `--force` is set) | low | Enforces `ErrInvalidConfigName` on `name`, `0o700` directory mode, `0o600` file mode, and `O_EXCL` (`ErrConfigExists`) unless `--force` is explicitly passed; unit tests run in `t.TempDir()` | Safe, deterministic config template generation without unintended overwrites or path traversal |
| Extract detailed CLI and configuration reference from `README.md` into `docs/USER_GUIDE.md` and make `README.md` a concise cover page | low | Preserves 100% of existing technical documentation in `docs/USER_GUIDE.md` while linking prominently from `README.md` and `docs/ARCHITECTURE.md` | Non-technical users get an approachable cover page and step-by-step guide with zero loss of reference detail |

### Review-Gate Capability Risks

* In Step 4 (`plan-review`), Stage will invoke multi-persona reviewer subagents
  (including cross-model `Architecture Strategist` on `gpt-6.1-sol` and
  `rubber-duck` adversarial critique) and append literal `dispatch_mode:` and
  `decision:` markers to `## Plan Review` before harvest.

## Runtime Verification and Closure

* **Runtime surface impact**: Adds the `teams-cli generate-config` subcommand
  (aliases `init-config`, `gen-config`) and `internal/config` template helpers;
  existing subcommands (`captions`, `join`, `auth`, `schedule`, `update`) are
  untouched.
* **Verification**:
  * `go vet ./...` and `go vet -tags=integration ./...`
  * `golangci-lint run`
  * `gofmt -l .`
  * `go test -race ./...` (including `internal/config` and `internal/cli` unit
    tests and `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`)
* **Operational closure**:
  * **Healthy signal**: All unit tests in `internal/config/template_test.go` and
    `internal/cli/generate_config_test.go`, `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`,
    and GitHub Actions CI (`ubuntu-latest` and `windows-latest`) pass on the PR.
  * **Failure signal**: Non-zero exit in `go test -race ./...`, `golangci-lint`,
    or CI jobs.
  * **Rollback trigger**: Regression in CLI command tree or config loading.
  * **Rollback procedure**: Revert the `024-S` merge commit on `main`.
  * **Owner**: Repository maintainer / operator.
  * **Validation window**: Immediate on PR CI run (`ubuntu-latest` +
    `windows-latest`), followed by post-merge closure (`post-merge/*` branch,
    `P-020` `compact-context` evaluation, and knowledge graduation).

## Plan Review

dispatch_mode: multi-agent
decision: PASS

* **Capability declaration (P-012)**:
  `TOOL_OK: reviewer-subagent-dispatch` — Multi-agent persona review executed across
  7 specialized reviewer subagents, including cross-model anchor review
  (`Architecture Strategist` on `gpt-6.1-sol`) and rubber-duck adversarial review
  (`Correctness Reviewer` on `gpt-6.1-sol`).
* **Plan hardening check (P-006)**: `Requires plan hardening: yes` declared and
  satisfied by `## Plan Hardening` with Protected Invariants, `Freeze-scope mode` /
  `Careful mode`, and `strict-safety` `ProposedAction` / `ActionRisk` table.

### Persona Coverage

| Persona | Mode / Model | Initial Findings | Post-Remediation Verdict |
|---|---|---|---|
| Constitution Reviewer (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 4 P2, 3 P3` — Recommended sequencing `*_test.go` Red phase before production code in U1/U2, checking `tmp.Chmod(0o600)` and commenting intentional `_ =` discards per Principle I, aligning `013-DL` and U3 on `config.example.yaml` parity verification, and naming Principle VIII safety modes, Principle XI (`P-009`), and `P-020` post-merge compaction. | PASS (all incorporated into U1, U2, U3, Constitution Check, and Plan Hardening) |
| Go Reviewer (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Confirmed idiomatic sentinel errors (`ErrConfigExists`, `ErrInvalidConfigName`), `errors.Is` wrapping, `path/filepath` traversal prevention, `O_EXCL` vs temp-file replacement, and `newGenerateConfigCommand(d *deps)` Cobra wiring. | PASS |
| Scope Boundary Auditor (always-on) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Confirmed strict 1:1 traceability to `D4D4F773` and `AB5E856D` and complete exclusion of `010-S` and `3EB89E2D`. | PASS |
| Learnings Researcher (always-on) | Subagent (`claude-haiku-5.5`) | `0 P0, 0 P1, 0 P2, 0 P3` — Surfaced `amend-breaks-shipment-gate.md`, `topology-gate-closure-branch-naming.md`, `copilot-rereview-timing.md`, and `golangci-lint-go-version-mismatch.md`, all incorporated into plan context. | PASS |
| Architecture Strategist (anchor cross-model) | Subagent (`gpt-6.1-sol`, `high`) | `0 P0, 0 P1, 3 P2, 0 P3` — Recommended (1) deterministic positional argument contract and test coverage when combined with `--output-dir` / `--name` or existing `.yaml`-named directories, (2) byte-identical parity check (normalized for LF/CRLF) between `config.example.yaml` and `TemplateYAML()` in U3, and (3) preserving `#output-target-precedence`, `#self-update`, and `#manual-lock-and-backup-recovery` signpost anchors in `README.md` so inbound links from `docs/design-docs/` do not break. | PASS (all 3 incorporated into U1, U2, and U3) |
| Security Lens Reviewer (filesystem & secret hygiene) | Subagent (`claude-sonnet-5.5`) | `0 P0, 0 P1, 0 P2, 3 P3` — Verified path-traversal rejection in `ResolveTemplatePath`, `0o700`/`0o600` modes in `WriteTemplate`, and recommended checking `tmp.Chmod(0o600)`, asserting POSIX mode bits when `runtime.GOOS != "windows"`, and calling `scanTrackedContent("TemplateYAML", []byte(TemplateYAML()))` in `template_test.go`. | PASS (all incorporated into U1) |
| Correctness Reviewer (rubber-duck adversarial) & Schema-CLI-Docs Coupling Reviewer | Subagents (`gpt-6.1-sol` & `claude-sonnet-5.5`) | `0 P0, 0 P1, 5 P2, 3 P3` — Identified (1) separating canonical uncommentable `# <key>:` lines from alias comment notes so uncommenting never activates conflicting overrides, (2) asserting `Headless == false` when loading raw all-commented `TemplateYAML()`, (3) expanding bare `~` and expanding positional args before `os.Stat` directory detection, (4) qualifying `os.Rename` replacement semantics on Windows, (5) explicitly documenting and testing that root `--config`/`-f` is ignored by `generate-config`, and (6) including all 11 YAML aliases in `template_test.go` and clarifying `cron` or `at` meeting validation in `docs/USER_GUIDE.md`. | PASS (all incorporated into U1, U2, and U3) |

### Findings and Dispositions

* **[P2 — Rubber-Duck Correctness Reviewer, resolved in U1]**: `TemplateYAML()`
  separates canonical uncommentable `# <key>:` / `# meetings:` lines from
  indented `#   Alias: ...` explanatory lines so uncommenting the canonical
  block never triggers conflicting overrides (`output_file` vs `output_dir` or
  `headed` vs `headless`), and `TestTemplateYAMLContentAndLoad` asserts both raw
  loading (`Headless == false`) and uncommented loading with exact field values.
* **[P2 — Rubber-Duck Correctness & Architecture Strategist, resolved in U1/U2]**:
  `ResolveTemplatePath` / `expandUserPath` expands bare `"~"`, `generate-config`
  expands positional arguments before checking `os.Stat` for existing
  `.yaml`-named directories, and `TestGenerateConfigCommandFlagsAndPositionalArgs`
  tests positional/flag interactions and confirms root `--config`/`-f` is
  ignored.
* **[P2 — Architecture Strategist, resolved in U3]**: `README.md` retains
  concise signpost subsections (`### Output target precedence`,
  `### Self-update`, `#### Manual lock and backup recovery`, and
  `## Configuration`) linking to `docs/USER_GUIDE.md` so inbound links from
  `docs/design-docs/` remain valid, and U3 verifies `config.example.yaml`
  matches `config.TemplateYAML()`.
* **[P2/P3 — Constitution, Security Lens, & Coupling Reviewers, resolved in U1/U2/U3]**:
  Sequenced `*_test.go` Red phase before Green phase in U1/U2, checked
  `tmp.Chmod(0o600)` and added comments on intentional `_ =` discards, added
  `scanTrackedContent` and non-Windows `0o700`/`0o600` mode assertions in
  `template_test.go`, included all 11 YAML aliases in the `template_test.go`
  assertion list, and documented `cron` or `at` validation in
  `docs/USER_GUIDE.md`.

**Gate verdict:** `PASS` (`0 P0, 0 P1, 0 unresolved P2`). Ready for harvest into
feature `024-F` and shipment `024-S`.

