---
title: Templated YAML configuration generator subcommand and user documentation restructure decision
description: Deliberation on grouping stash entries D4D4F773 and AB5E856D into feature 024-F and shipment 024-S
topic: Templated YAML Configuration Generator Subcommand and User Documentation Restructure (D4D4F773 + AB5E856D)
depth: standard
decision_status: decided
promoted_to: both
deliberation_id: 013-DL
feature: 024-F
shipment: 024-S
source_stash:
  - D4D4F773
  - AB5E856D
linked_artifacts:
  - docs/plans/2026-11-14-templated-config-generator-and-user-docs-plan.md
  - .backlogit/queue/013-DL.md
  - .backlogit/queue/024-F.md
  - .backlogit/queue/024-S.md
date: 2026-11-14
status: accepted
tags:
  - config
  - cli
  - docs
  - ux
  - onboarding
---

## Problem Frame

This deliberation covers the contextual grouping of two active `medium`-priority
feature stash entries into a single cohesive release unit under covering feature
**`024-F`** and shipment **`024-S`** (sequenced after `023-S`):

1. **`D4D4F773`** (`feature`, `medium`):
   > Add a dedicated CLI subcommand (e.g., `teams-cli generate-config` or
   > similar) to generate a fully templated YAML configuration file with all
   > options commented out and inline explanatory comments on how to set up the
   > configuration file. Requirements: support a target output directory and
   > config filename as command arguments/flags.
2. **`AB5E856D`** (`feature`, `medium`):
   > Restructure and complete repository documentation for non-technical users
   > and legitimate AI agentic workflows: (1) Make the main `README.md` a
   > concise, approachable cover page stating the core purpose of `teams-cli`
   > (not to circumvent meeting security, but to enable legitimate context
   > capture in AI agentic work environments — capturing requirements, action
   > items, team/org-level employee impact for self-assessments, and building a
   > personal/team communications library for AI-assisted synthesis of key
   > outcomes) along with a brief overview of features and capabilities; (2)
   > Extract and expand comprehensive user-facing guides in `docs/` covering all
   > commands, flags, config examples, and step-by-step setup/configuration
   > instructions written for non-technical users, linked clearly from
   > `README.md`.

### Why These Entries Belong Together

* **Shared user-onboarding and configuration-legibility theme**: Non-technical
  users adopting `teams-cli` for legitimate AI-assisted meeting context capture
  need two things immediately: (1) an approachable cover page (`README.md`) and
  step-by-step user guide (`docs/USER_GUIDE.md`) that explain why `teams-cli`
  exists and how to use it without reading Go source code, and (2) a single
  command (`teams-cli generate-config`) that creates a self-documenting
  `config.yaml` with every option explained in plain language.
* **Direct workflow dependency**: The restructured 3-step Quick Start in
  `README.md` and the step-by-step setup walkthrough in `docs/USER_GUIDE.md`
  both begin with `teams-cli generate-config`, and `config.example.yaml` mirrors
  the exact commented YAML template returned by `config.TemplateYAML()`.
  Shipping them together in `024-S` ensures the CLI subcommand, the repository's
  `config.example.yaml`, the cover-page `README.md`, and `docs/USER_GUIDE.md`
  are 100% consistent in a single pull request.

### Explicit Scope Boundary (Excluded Items)

* Queued shipment **`010-S`** (`010-F`, `001-DL`, `010.001-T`..`010.009-T`) and
  stash entry **`3EB89E2D`** (live-tenant validation of scheduled meeting join,
  overtime continuation, in-browser call-end silence shutdown, and delayed AI
  summary Recap enrichment) remain **explicitly out of scope** and untouched
  because they require a live Microsoft Teams tenant and interactive human
  checkpoints while the operator is AFK.

## P-021 C5 and C6 Triage Record

* Neither `D4D4F773` nor `AB5E856D` carries the `DEFERRED SCOPE EXPANSION`
  marker.
* **Obligation (A) Unconditional duplicate detection**: Executed across
  `.backlogit/stash.jsonl`, `.backlogit/queue/`, and `.backlogit/archive/`.
  Outcome: **CLEAN DUPLICATE SCAN** for `D4D4F773` and `AB5E856D`.
* **Obligation (B) Late-identifier reconciliation**: Not triggered (neither
  entry has `N/A` deferred-expansion source refs).

## Research Findings

### Prior Learnings (`docs/compound/`)

* **`docs/compound/workflow-issues/amend-breaks-shipment-gate.md`**:
  Never amend or rebase commits after marking backlogit items done; push any
  post-completion review fixes as new commits.
* **`docs/compound/workflow-issues/topology-gate-closure-branch-naming.md`**:
  Use the shipment branch during active implementation and
  `post-merge/{feature_slug}` for post-merge closure; when testing `//go:build unix`
  packages on Windows via WSL, copy only `cmd/`, `internal/`, `go.mod`, and
  `go.sum`.
* **`docs/compound/workflow-issues/copilot-rereview-timing.md`**:
  When pushing fixes to a PR with `dismiss_stale_reviews_on_push`, poll until a
  Copilot review matches the new `HEAD` SHA before trusting a zero unresolved
  thread count.
* **`internal/config/gitignore_test.go` (`TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`)**:
  Scans all git-tracked files for live Teams meeting passcodes (`?p=` or `&p=`
  without a synthetic marker), `SKYPE-TOKEN` JWTs, and `Bearer eyJ...` JWTs.
  All example URLs in `TemplateYAML()`, `config.example.yaml`, `README.md`, and
  `docs/USER_GUIDE.md` must use synthetic URLs (`https://teams.microsoft.com/meet/synthetic-...`
  or `https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic`).

### Codebase Investigation

* **`internal/config/config.go`**:
  * `Config` and `MeetingSchedule` support primary YAML keys and aliases parsed
    in `config.Load`:
    - `output_dir` (aliases: `output_folder`, `default_path`; `output` sets
      `output_file` when ending in `.md`/`.markdown` and `output_dir` otherwise)
    - `output_file`
    - `filename_template` (alias: `name_template`), default
      `"<category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md"`
    - `category`
    - `meeting_url` (alias: `url`)
    - `profile_dir`, default `%LOCALAPPDATA%\teams-cli\profile` on Windows or
      `<UserConfigDir>/teams-cli/profile`
    - `browser_path`
    - `default_account` (aliases: `entra_account`, `account`)
    - `bot_name`, default `"Transcription Agent"`
    - `headless` (alias: `headed`)
    - `capture_audio` (alias: `audio`)
    - `capture_mode` (alias: `mode`: `auto`, `ephemeral-live`,
      `live-with-summary`, `past-recap`)
    - `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`
    - `queue_file`
    - `meetings` (`name`, `url`, `cron` or `at`, `mode` / `capture_mode`,
      `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`,
      `category`, `capture_audio` / `audio`, `output_dir`)
  * When `config.Load` reads a file where every key is commented out, it begins
    with `DefaultConfig()` (`OutputDir`, `ProfileDir`, `BotName`, and
    `FilenameTemplate` pre-populated) and sets `Headless = false` when neither
    `headless` nor `headed` is uncommented, which passes `cfg.Validate()` out of
    the box.
* **`internal/cli/root.go` and `internal/cli/update.go`**:
  * `newRootCommand(d *deps)` attaches subcommands via `root.AddCommand(...)`.
  * Subcommands that do not require an existing config file or browser session
    (such as `newUpdateCommand(d)`) take `d *deps` directly without invoking
    `loadConfigFrom`.

## Options Evaluated

### Option A: Root Flag (`--init-config`) Dumping `yaml.Marshal(DefaultConfig())` + Monolithic `README.md`

* Add a boolean `--init-config` flag to the root command that calls
  `config.Save("config.yaml", defaultCfg)`.
* Expand `README.md` in place with additional prose for non-technical users.

* **Pros**: Minimal code surface.
* **Cons**:
  * `yaml.Marshal` emits uncommented YAML without inline explanations, omits
    `omitempty` fields (`category`, `default_account`, `capture_mode`,
    `duration`, `idle_timeout`, `ai_summary_delay`, `lobby_timeout`,
    `queue_file`, `output_file`, `browser_path`, `meetings` examples, and
    aliases), and leaves all values active rather than commented out.
  * Does not satisfy `D4D4F773`'s requirement for a dedicated CLI subcommand
    supporting target output directory and config filename arguments/flags with
    all options commented out and explained inline.
  * Keeping 500+ lines of deep CLI/flag/lock/protocol reference inside
    `README.md` violates `AB5E856D`'s requirement for a concise cover-page
    `README.md` paired with extracted, expanded guides in `docs/`.
* **Effort**: low | **Fit**: low.

### Option B: Dedicated `teams-cli generate-config` Subcommand + `internal/config/template.go` + Cover-Page `README.md` and `docs/USER_GUIDE.md` (chosen)

1. **`D4D4F773` — Template Generator in `internal/config/template.go` (`024.001-T`, domain: `config`)**:
   * Define exported sentinel errors:
     - `ErrConfigExists = errors.New("config: configuration file already exists")`
     - `ErrInvalidConfigName = errors.New("config: invalid configuration filename")`
   * Provide `TemplateYAML() string` returning a complete, human-friendly YAML
     configuration template where every configuration option (`output_dir` /
     `output_folder` / `default_path` / `output_file` / `output`,
     `filename_template` / `name_template`, `category`, `meeting_url` / `url`,
     `profile_dir`, `browser_path`, `default_account` / `entra_account` /
     `account`, `bot_name`, `headless` / `headed`, `capture_audio` / `audio`,
     `capture_mode` / `mode`, `duration`, `idle_timeout`, `ai_summary_delay`,
     `lobby_timeout`, `queue_file`, and `meetings` with both recurring `cron`
     and one-shot `at` entries) is documented with plain-language inline
     comments and commented-out example values (`# key: value`) so a regular or
     non-technical user can read, uncomment, and customize any setting.
   * Provide `ResolveTemplatePath(outputDir, name string) (string, error)`:
     - Default `outputDir` to `"."` when empty/whitespace; expand user path via
       `expandUserPath`.
     - Default `name` to `"config.yaml"` when empty (`name == ""`); if `name` is
       non-empty but whitespace-only once trimmed, return `ErrInvalidConfigName`.
     - Validate trimmed `name`: reject `"."`, `".."`, any `..` traversal
       segment, any path separator (`/` or `\`), any volume prefix
       (`filepath.VolumeName(trimmed) != ""`), or bare extension-only names
       (`".yaml"`, `".yml"`) with `ErrInvalidConfigName`.
     - If `name` does not end with `.yaml` or `.yml` (case-insensitive), append
       `.yaml`.
     - Return `filepath.Join(resolvedOutputDir, normalizedName), nil`.
   * Provide `WriteTemplate(path string, force bool) error`:
     - Reject empty/whitespace `path` with `ErrInvalidConfigName`.
     - Create parent directories with `0o700` permissions (`os.MkdirAll(dir, 0o700)`).
     - When `!force`: create the file exclusively with
       `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)`, returning
       an error wrapping `ErrConfigExists` if the destination already exists.
     - When `force`: write to a sibling temporary file in `dir`
       (`os.CreateTemp(dir, ".config-template-*.yaml.tmp")`), `Chmod(0o600)`,
       `Write`, `Sync`, `Close`, and atomically `os.Rename` over `path` (cleaning
       up the temp file on any error).
2. **`D4D4F773` — `generate-config` Subcommand in `internal/cli/generate_config.go` (`024.002-T`, domain: `cli`)**:
   * Implement `newGenerateConfigCommand(d *deps) *cobra.Command` and wire it
     into `newRootCommand` in `internal/cli/root.go`:
     - `Use: "generate-config [output-dir]"`
     - `Aliases: []string{"init-config", "gen-config"}`
     - `Args: cobra.MaximumNArgs(1)`
     - Flags:
       * `--output-dir`, `-d`: target directory for the generated config file
         (default `"."`).
       * `--name`, `-n`: config filename to generate inside the target directory
         (default `"config.yaml"`).
       * `--force`: overwrite an existing configuration file at the resolved
         destination path.
     - Positional argument handling:
       * When a single positional argument is passed and ends in `.yaml` or
         `.yml` (case-insensitive), neither `--output-dir` nor `--name` was
         explicitly changed, and the positional argument does not exist on disk
         as a directory, split `filepath.Dir(arg)` as `outputDir` and
         `filepath.Base(arg)` as `name`.
       * Otherwise, when a single positional argument is passed and
         `--output-dir` was not explicitly changed, use the positional argument
         as `outputDir`.
     - Resolve path with `config.ResolveTemplatePath(outputDir, name)`, write
       template with `config.WriteTemplate(resolvedPath, force)`, and print the
       resolved path (`fmt.Fprintln(cmd.OutOrStdout(), resolvedPath)`) to
       `cmd.OutOrStdout()`.
3. **`AB5E856D` — Cover-Page `README.md` + `docs/USER_GUIDE.md` + Repo Doc Alignment (`024.003-T`, domain: `docs`)**:
   * Restructure `README.md` into a concise, welcoming cover page:
     - Prominently state the core purpose: `teams-cli` is built **not to
       circumvent meeting security**, but to enable legitimate context capture
       for authorized users in AI agentic work environments — capturing
       requirements, action items, and team/organizational employee impact for
       self-assessments, and building a personal or team communications library
       so AI assistants can synthesize meeting language into actionable outcomes.
     - Summarize core capabilities (Live Captions, Meeting Recap & AI Summary
       Export, Companion MP3 Audio, Scheduled & Protocol-Driven Capture, Batch
       Queues, Templated YAML Config Generation via `generate-config`, and
       Verified Self-Update).
     - Provide a 3-step Quick Start (`teams-cli generate-config`,
       `teams-cli auth`, `teams-cli captions`) and clear links to
       `docs/USER_GUIDE.md`, `config.example.yaml`, `docs/ARCHITECTURE.md`, and
       `docs/QUALITY_SCORE.md`.
   * Create `docs/USER_GUIDE.md` (with YAML frontmatter `title:` per
     `markdown.instructions.md`):
     - Written in an approachable, step-by-step style for non-technical users
       while preserving 100% of the complete CLI command, flag,
       output-precedence, filename-template, batch-queue (`--queue`),
       scheduled/protocol capture (`ephemeral-live`, `live-with-summary`,
       `past-recap`, `auto`, activity-aware overtime/idle shutdown, `schedule`
       daemon, Windows Task Scheduler recipes), browser security posture,
       headless automation, self-update locking/recovery, and YAML configuration
       key/alias reference tables previously in `README.md`, plus full
       instructions and examples for `teams-cli generate-config`
       (`--output-dir` / `-d`, `--name` / `-n`, `--force`, positional args, and
       aliases `init-config` / `gen-config`).
   * Update `config.example.yaml` to match `config.TemplateYAML()`, and update
     `docs/ARCHITECTURE.md` and `docs/QUALITY_SCORE.md` to reflect
     `generate-config`, `internal/config/template.go`, and `docs/USER_GUIDE.md`.

* **Pros**: Fulfils every requirement of `D4D4F773` and `AB5E856D`, enforces
  strict path-traversal and `0o700`/`0o600` filesystem safety on config
  generation, keeps `README.md` approachable while preserving 100% of technical
  reference depth in `docs/USER_GUIDE.md`, and keeps `config.example.yaml`
  aligned with `TemplateYAML()`.
* **Cons**: Requires coordinating 5 documentation/example files in `024.003-T`.
* **Effort**: medium | **Fit**: high.

## Trade-off Comparison

| Criterion | Option A (`--init-config` `yaml.Marshal` + monolithic `README.md`) | Option B (`generate-config` subcommand + `TemplateYAML()` + cover-page `README.md` & `docs/USER_GUIDE.md`) |
|---|---|---|
| All options commented out with inline plain-language comments (`D4D4F773`) | No (`yaml.Marshal` strips comments and omits `omitempty` keys/aliases) | Yes (`TemplateYAML()` documents every option, alias, placeholder, and schedule mode with `#` comments) |
| Target directory & filename flags + path-traversal safety (`D4D4F773`) | Partial | Yes (`--output-dir`/`-d`, `--name`/`-n`, `--force`, positional arg support, `ErrInvalidConfigName`, `ErrConfigExists`) |
| Concise cover-page `README.md` stating legitimate AI agentic purpose (`AB5E856D`) | No (remains a 500+ line reference manual) | Yes (clear cover page + 3-step Quick Start + links to `docs/USER_GUIDE.md`) |
| 100% preservation of CLI, flag, protocol, queue, and lock reference (`AB5E856D`) | In `README.md` only | Yes, expanded for non-technical users in `docs/USER_GUIDE.md` |

## Decision

We select **Option B** and decompose the work into covering feature **`024-F`**
and shipment **`024-S`** (sequenced after `023-S`) across three width-isolated
tasks:

1. **`024.001-T`** (`config` domain, size `S`, complexity `low`):
   Implement `TemplateYAML()`, `ResolveTemplatePath()`, `WriteTemplate()`,
   `ErrConfigExists`, and `ErrInvalidConfigName` in `internal/config/template.go`
   with unit tests in `internal/config/template_test.go`.
2. **`024.002-T`** (`cli` domain, size `S`, complexity `low`, depends on `024.001-T`):
   Implement `newGenerateConfigCommand` (`generate-config`, aliases
   `init-config`, `gen-config`) in `internal/cli/generate_config.go`, wire into
   `internal/cli/root.go`, and add unit tests in
   `internal/cli/generate_config_test.go`.
3. **`024.003-T`** (`docs` domain, size `M`, complexity `low`, depends on `024.002-T`):
   Restructure `README.md` as the cover page, create `docs/USER_GUIDE.md`, and
   update `config.example.yaml`, `docs/ARCHITECTURE.md`, and
   `docs/QUALITY_SCORE.md`.

## Rejected Alternatives

* **Option A** was rejected because `yaml.Marshal(DefaultConfig())` cannot emit
  commented-out options or inline explanatory prose, and keeping all detailed
  CLI/configuration reference material inside `README.md` fails to provide the
  concise cover page and dedicated user guide requested in `AB5E856D`.

## Unresolved Questions

None blocking planning or harvest.

## Risks and Mitigations

* **Path traversal or unintended overwrite via `--name` or `--force`**:
  Mitigated by rejecting path separators (`/` and `\`), `..` segments, volume
  prefixes, and empty/whitespace-only names in `ResolveTemplatePath` with
  `ErrInvalidConfigName`, and using `O_CREATE|O_WRONLY|O_EXCL` (`0o600`) when
  `!force` so existing files are never overwritten unless `--force` is passed.
* **Drift between `TemplateYAML()` and `config.Load` / `config.example.yaml`**:
  Mitigated by unit tests in `internal/config/template_test.go` that verify:
  (1) the raw `TemplateYAML()` parses cleanly via `config.Load` (populating
  `DefaultConfig()` fields with `Headless == false`), and (2) an uncommented
  copy of the canonical template settings parses and validates cleanly via
  `config.Load` with exact expected field values, while U3 (`024.003-T`) updates
  `config.example.yaml` to match `TemplateYAML()` (verified after LF/CRLF line
  ending normalization).
* **Secret-hygiene false positives in documentation examples**:
  Mitigated by using exclusively synthetic URLs without live passcodes and
  verifying with `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks`.
