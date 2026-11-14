---
type: memory-compacted
date: 2026-11-13
shipment: 021-S
feature: 021-F
source:
  - docs/archive/memory/2026-11-13/021-S-default-entra-account-selection-memory.md
  - docs/memory/2026-11-13/ship-021-S-memory.md
---

# Compacted memory - Shipment 021-S (Default Microsoft Entra account auto-selection and internal/config test hardening)

**Shipped**: PR #53, merge `f1c9dd3807e8ff048e1b93005ec80b32af5ad115`. Closed and
archived via the P-015 verified fully-covered-root cascade
(`backlogit shipment ship 021-S --sha f1c9dd3807e8ff048e1b93005ec80b32af5ad115`),
and deliberation `010-DL` archived via `backlogit archive 010-DL`.

## Decision and outcome

* `internal/config`: Added `Config.DefaultAccount` (`default_account` with YAML
  aliases `entra_account` and `account`, precedence `default_account` >
  `entra_account` > `account`) and table-driven alias/precedence/path tests,
  raising `internal/config` statement coverage from `69.7%` to `83.3%`
  (`Grade A` in `docs/QUALITY_SCORE.md`).
* `internal/browser`: Added `DefaultAccount` on `Options` and `MeetingOptions`,
  `RedactAccountIdentifier` (`a***@contoso.com` / `A***`), and
  `selectEntraAccountJS` (JSON-encoded target, trusted Microsoft login origin
  check, visible picker container check, password and MFA challenge exclusion,
  ancestor-aware `isVisible` check, `#otherTile` exclusion, exact
  case-insensitive matching with `matches.length !== 1` fail-closed semantics,
  and 3000 ms click cooldown), wired into `ChromeRunner.run`,
  `chromeMeeting.joinHeaded`, and `chromeMeeting.awaitPreJoinOrAdmission`.
* `internal/cli`: Bound `--account` across `root`, `join`, `captions`, `auth`,
  and `schedule`, honored `cmd.Flags().Changed("account")` so `--account=`
  clears `default_account`, forwarded `--account` in `inheritedQueueFlags`, and
  masked `--account` / `--account=` in `sanitizeQueueArgsForLog`.
* Documentation: Updated `README.md`, `docs/ARCHITECTURE.md`,
  `docs/QUALITY_SCORE.md`, and graduated `010-DL` into
  `docs/design-docs/2026-11-09-teams-browser-caption-and-recap-capture.md`.

## Key reusable insights

* Cross-platform path assertions in `internal/config/config_test.go` must use
  `filepath.FromSlash("C:/...")` rather than `filepath.Clean("C:\\...")` because
  `expandUserPath` normalizes backslashes to `/` on non-Windows (`ubuntu-latest`)
  hosts.
* Headless Chromium synthetic-DOM tests (`TestSelectEntraAccountJSDOM`) running
  on `ubuntu-latest` containers must prefer `/usr/bin/google-chrome` over
  `/opt/microsoft/msedge`, opt in to `DisableSandbox: runtime.GOOS != "windows"`,
  and wait/retry `os.RemoveAll(profileDir)` after `cancelAlloc()` so background
  Chromium profile flushing does not race `t.TempDir()` cleanup.
* Amending a task commit after running `backlogit move <task> --status done`
  leaves `gate_evidence.head_sha` pointing at the pre-amend SHA; re-running the
  `active -> done` transition on the merged ancestor refreshes `gate_evidence`
  so `backlogit shipment ship` succeeds.
