---
type: session-memory
date: 2026-09-03
scope: agent-harness
outcome: installed
branch: chore/autoharness-install-2026-09-03
commits: [016b329, f1cd6e0]
---

# Session memory — autoharness 1.5.0 merge install

## What was done

Merge-installed the agent harness at **autoharness 1.5.0** over the prior
**1.4.11** install. Merge mode (not fresh) was chosen because
`.autoharness/harness-manifest.yaml` already existed; existing artifacts were
backed up to `.autoharness/backups/2026-09-03/` (gitignored) before overwrite.

Resolved `autoharness_home` = `C:\Python\Python314\Lib\site-packages\autoharness\data`
via the `autoharness home` CLI. Work was done on a feature branch because the
session started on `main` (the default branch) — install output is never
committed directly to the default branch.

### Baseline to final (deterministic verification)

| Metric | Before | After |
|---|---|---|
| Strict schema blockers | 25 | 0 |
| Blockers | 3 | 0 |
| Unresolved placeholders | 71 | 1 (upstream template prose) |
| Failing targeted checks | 18 | 0 of 75 |
| Migration proposals | 4 | 0 |
| Checksums unchanged | 86 of 97 | 103 of 103 |

Go module remains healthy: `go build ./...`, `go vet ./...`, `gofmt -l .` clean,
and all 8 test packages pass.

### Artifacts

* Legacy `.stage.agent.md` / `.ship.agent.md` standardized to canonical
  `_stage.agent.md` / `_ship.agent.md` (filename, `name:`, `id:`, cross-references,
  manifest paths).
* New: `capability-pack-enforcement`, `escalation-protocol`, `copilot-code-review`,
  `ci-security`, `workflows` instructions; `brainstorm` skill;
  `template-integrity-reviewer` and `schema-cli-docs-coupling-reviewer` personas;
  pre-push, pre-commit-topology, and CI-topology hook scripts.
* Removed: `ping-loop.prompt.md` (no longer shipped by the 1.5.0 agent-intercom pack).
* Refreshed: 88 existing artifacts re-rendered from 1.5.0 templates.
* Contracts: `workspace-profile` and `harness-manifest` schema violations repaired;
  `config.yaml` bumped 1.0.0 to 1.1.0 (adds `sidecars`).

## Key decisions and rationale

1. **Merge install, not fresh.** An existing manifest was present, so existing
   artifacts were preserved/refreshed rather than clobbered.

2. **The hand-written CI workflow was NOT overwritten.** `.github/workflows/ci.yml`
   is operator-maintained with real Go jobs. The autoharness `ci.yml` template was
   deliberately not generated over it; the profile records this.

3. **Staged renders are not sufficient on their own.** `verify-workspace` performs
   literal variable substitution only — it cannot do capability-pack overlay
   weaving, Engram-block preservation, or shell-specific gate composition. Blindly
   copying staging **regressed** `copilot-instructions.md` (dropped the
   `<!-- engram:start -->` block, left `{{PROJECT_STRUCTURE}}` unresolved inside a
   code fence where the scanner does not look). Fix: expand
   `manifest.variables_used` to 89 entries so renders are complete, then re-apply
   overlay edits via `.autoharness/install-overlays.py`.

4. **`manifest.variables_used` is the authoritative variable source.**
   `verify_workspace._derive_template_variables` seeds from it first and only then
   applies `setdefault` derivations. `config.overrides` is **not** consulted for
   variable resolution (only for `HARNESS_OVERRIDES_YAML`).

5. **Two different checksum algorithms coexist.** The general `checksum_scan`
   hashes **raw bytes** (`verify_workspace.py:4930`), while the
   `capability_pack_enforcement` check hashes **EOL-normalized** content
   (`:4316`). Both are satisfied only by writing
   `capability-pack-enforcement.instructions.md` with **LF** endings.

6. **VS Code is not used in this workspace, so the installer's VS Code step is
   not applicable.** The environment is the **Copilot CLI** (`C:\Tools\copilot.exe`,
   resolved from PATH) launched with this workspace as CWD; `start.ps1` redirects
   `COPILOT_HOME` to the workspace-local `.copilot/`, so agent state, the session
   store, and installed plugins are project-scoped. The autoharness elective agents
   and skills are discovered through the workspace-local plugin at
   `.copilot/installed-plugins/autoharness/` (v1.5.0, matching the CLI) — not
   through any editor settings file. The profile records `vscode.detected: false`
   and `ai_environments.detected: ["copilot-cli"]`; the manifest records
   `vscode_settings.applied: false` with `skipped_because: not applicable`.
   Constitution Principle IV (CLI Workspace Containment) is therefore not even
   engaged — there is no out-of-workspace target to write.

7. **Agentic config surfaces retargeted to the real environment.**
   `AGENTIC_CONFIG_GLOB` and `SECURITY_CONFIG_RULES` previously named `.vscode/**`,
   `.claude/**`, and `.cursor/**`. They now name what this workspace actually runs:
   `.github/**`, `.copilot/config.json`, `.copilot/settings.json`,
   `.copilot/installed-plugins/**`, `.mcp.json`, and `AGENTS.md`, so the
   security-audit skill scans live surfaces instead of dead ones.

8. **Stage escalation route made distinct.** Stage's role route falls back to
   `tier3` (claude-opus-4.8/anthropic/xhigh) and its escalation route did too,
   which fires the P-013.6 same-route no-op guard (`ESCALATION_DEGRADED`).
   Declared `model_routing.stage.escalation` = claude-opus-5/anthropic/max.

## Defects found by adversarial review, and fixed

* **High — the `fmt` quality gate was inert.** `gofmt -l .` only *lists* files; it
  exits 0 regardless, so every consumer checking exit status passed unconditionally.
  `QUALITY_GATE_3` is now `test -z "$(gofmt -l .)"` and both pre-push hooks carry a
  shell-correct failing form (POSIX `test -z "$(...)"`; PowerShell output check).
  Both directions were proven with a stubbed `gofmt` (clean to exit 0, dirty to exit 1).
* **Root cause behind it — `core.autocrlf=true` with no `.gitattributes`.** Committed
  blobs are LF and gofmt-clean, but the Windows working tree was CRLF, so `gofmt -l .`
  flagged all 40 Go files. A hard gate would have blocked every local push. Added
  `.gitattributes` (`*.go text eol=lf`) and renormalized the working tree — **zero blob
  changes**, working-tree-only fix.
* **Medium — closure cleanup fired pre-merge.** The backlogit "Source artifact cleanup"
  overlay had no `mode` guard, so a `pre-merge` invocation would archive the source
  stash/deliberation before merge, violating P-021 C5 (and the merge SHA it references
  does not exist yet). Now explicitly gated to `mode == post-merge`.
* **Medium — security patterns targeted a nonexistent package.** `internal/auth/` does
  not exist; the auth/session surface is `internal/cli/auth.go`, `internal/teams/`, and
  `internal/browser/` (where `UserDataDir` lives). Retargeted, so the Security Reviewer
  persona actually auto-selects on auth changes.
* **Medium — harness falsely claimed gofmt was a CI gate.** No workflow runs gofmt and
  `.golangci.yml` does not enable it. `CI_NOTES` and `HARNESS_ENFORCED_SUMMARY` corrected
  so code review is not told to de-prioritize an unenforced concern.
* **Medium — invalid runtime probe.** `teams-cli schedule` takes no flags
  (`cobra.NoArgs`); the `--dry-run` probe would always fail. Replaced with `--help`.
* **Low — broken Markdown code spans.** `SECURITY_SCAN_PATTERNS` and
  `SECURITY_OWASP_PATTERNS` contained backticks but are rendered *inside* code spans
  by their templates. Backticks stripped.
* **Low — `backlog_tool.mcp_registered` comment contradicted `.mcp.json`.** Corrected.

## Failed / rejected approaches

* **Reverse-engineering old variable values by full-file regex** against the 1.5.0
  templates recovered only 3 of 34 (templates changed too much). Anchor-based
  extraction (literal context before/after each placeholder) recovered 12 and was
  the workable technique.
* **A single quote-free format command for both shells** does not exist. `test -z
  "$(...)"` breaks the `.ps1` render; `... | grep -q ^ && exit 1` breaks PowerShell.
  `FORMAT_CHECK_COMMAND` is therefore kept quote-free (`gofmt -l .`) so renders stay
  valid, and each hook gets its shell-correct form post-render.
* **Editing upstream template prose to silence the last placeholder/portability
  warnings** was rejected — they originate in autoharness 1.5.0 templates and
  diverging would create permanent tune drift for no functional gain.

## Gotcha for the next session

`.autoharness/install-overlays.py` **must be re-run after any `verify-workspace`
re-render or tune**, otherwise the overlay weaving, hook gate composition, LF
normalization, and Engram block are silently lost. It is idempotent and
newline-aware (matches each file's dominant line ending). After running it,
refresh manifest checksums (raw-byte SHA-256) and re-verify.

## Confirmed operating environment

| Component | Value |
|---|---|
| AI environment | GitHub Copilot CLI (`C:\Tools\copilot.exe`, from PATH), CWD = workspace |
| Copilot state | workspace-local `.copilot/` (`COPILOT_HOME` set by `start.ps1`) |
| autoharness plugin | `.copilot/installed-plugins/autoharness/` v1.5.0 |
| autoharness CLI | `C:\Python\Python314\Scripts\autoharness.exe` v1.5.0 |
| autoharness home | `C:\Python\Python314\Lib\site-packages\autoharness\data` |
| Python | 3.14.3 (`C:\Python\Python314\python.exe`) |
| VS Code | not used |
| Python 3.12 | not used (a stale autoharness copy remains installed; harmless) |

## Open items / next steps

1. **Open a PR** from `chore/autoharness-install-2026-09-03`; merge commit only (P-009).
2. **Opt-in hooks are installed but not activated** — `scripts/pre-push-quality-gates.*`
   and `scripts/pre-commit-pipeline-topology.*` are written but never wired into
   `.git/hooks` automatically (P-019). Chain both pre-commit hooks from one
   dispatcher if markdownlint is added later.
3. **Optional cleanup** — a leftover autoharness package remains under Python 3.12
   (`C:\Python\Python312\python.exe -m pip uninstall autoharness`) and a stale,
   gitignored `.vscode/mcp.json` remains on disk. Neither is referenced by the
   harness or the CLI; removing them is cosmetic.
4. **2 residual warnings + 1 unresolved placeholder are upstream** autoharness 1.5.0
   template text (`~/.autoharness/` fallback documentation flagged by the portability
   scanner; a literal `{{VARIABLE}}` in `_ship.agent.md` prose). Not install defects.
