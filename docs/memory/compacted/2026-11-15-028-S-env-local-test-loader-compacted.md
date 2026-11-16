---
title: Compacted Memory - Shipment 028-S (.env.local.test Loader and .env.example Template)
description: Consolidated Stage and Ship session summary for shipment 028-S (feature 028-F, deliberation 017-DL, source stash ED249B2F)
date: 2026-11-15
shipment: 028-S
feature: 028-F
deliberation: 017-DL
source_stash:
  - ED249B2F
archived_sources:
  - docs/archive/memory/2026-11-15/stage-028-s-env-local-test-memory.md
  - docs/archive/memory/2026-11-15/ship-028-s-env-local-test-memory.md
---

## Outcome

Shipment `028-S` (`028-F`, deliberation `017-DL`, source stash `ED249B2F`) merged to `main` in PR #70 (`40e20ef1077f3cfd5b083f1d07879d307ae4aa84`) with all 4 tasks (`028.001-T`..`028.004-T`) complete, 4/4 CI checks green on `ubuntu-latest` and `windows-latest`, and `autoharness gate copilot-review` `SATISFIED`.

## Decisions & Key Implementation Facts

1. **Test-only loaders preserving 10-package architecture (`017-DL` Option A)**: `.env.local.test` loading lives solely in `internal/browser/integration_helper_test.go` (`LoadTestEnv`) and `internal/cli/binary_automation_test.go` (`loadCLITestEnv`) - zero production `.go` changes and zero new packages.
2. **Tracked `.env.example` & secret hygiene**: Root `.env.example` supplies 11 synthetic placeholder variables (`TEAMS_PROFILE_DIR` / `TEAMS_TEST_PROFILE_DIR` under gitignored `./bin/`) plus commented `# GO_TEAMS_INTEGRATION` and `# TEAMS_DISABLE_SANDBOX`, verified by `internal/config/gitignore_test.go` (`TestIsLocalEnvPath` and `TestEnvExampleTemplateContainsOnlySyntheticPlaceholders`).
3. **Fail-closed loader invariants**: Regular non-symlink file check, symmetrical `filepath.EvalSymlinks` containment, `64 KiB` bounded open-handle read, fail-closed `git ls-files -z` / `.git/index` untracked check (case-insensitive on Windows), allowlisted alias-pair precedence (`os.Getenv` always wins even when `.env.local.test` is absent), Windows backslash preservation in double-quoted paths, separator normalization with `..` / drive-relative path rejection, `isTemplatePlaceholder` pre-filtering, and zero-echo parse/path diagnostics.
4. **Learnings & catches during execution**:
   * `TestTrackedFilesContainNoLivePasscodeOrTokenLeaks` enforces `syntheticPasscodeMarkers` on test URL fixtures too (`?p=realpass` failed; changed to `?p=samplepass`).
   * Linux CI catches Windows-only `filepath.IsAbs` assumptions and `t.Setenv("PATH", "")` leaks across subtests (`6e88530`).
   * Operator's local gitignored `.env.local.test` (`2026-11-14`) had `GO_TEAMS_BINARY` pointing to pre-`027-S` `.\bin\teams-cli.exe`; overriding `GO_TEAMS_BINARY` in process env passed all 6 `TestCompiledBinaryCapturesRecapToExplicitOutput` subtests.
5. **Untouched deferred work**: Stash `3EB89E2D` remains active for Option C live-tenant validation; shipment `010-S` remains queued.
