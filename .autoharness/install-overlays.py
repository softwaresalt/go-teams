"""Install staged renders and re-apply the hand-authored overlay edits.

verify-workspace performs literal variable substitution only; it cannot do the
capability-pack overlay weaving or the shell-specific gate composition that the
install-harness skill mandates. This script copies the staged renders and then
re-applies every hand-authored delta so the two stay reconcilable.

Idempotent: each edit checks for its post-state before applying.
"""

from pathlib import Path
import sys

STAGE = Path(".autoharness/staging")
WS = Path(".")
failures: list[str] = []


def install_staged() -> int:
    copied = 0
    for f in sorted(STAGE.rglob("*")):
        if not f.is_file() or f.name.startswith("verify-workspace-report"):
            continue
        rel = f.relative_to(STAGE)
        if rel.as_posix() == ".env.local":  # never overwrite developer secrets
            continue
        dst = WS / rel
        dst.parent.mkdir(parents=True, exist_ok=True)
        data = f.read_bytes()
        if dst.suffix == ".sh":
            data = data.replace(b"\r\n", b"\n")
        dst.write_bytes(data)
        copied += 1
    return copied


def replace(rel: str, old: str, new: str, label: str, binary_nl: str | None = None) -> None:
    p = WS / rel
    raw = p.read_bytes().decode("utf-8")
    if binary_nl is None:
        # Match the file's dominant line ending so multi-line anchors resolve.
        binary_nl = "\r\n" if raw.count("\r\n") > raw.count("\n") - raw.count("\r\n") else "\n"
    o = old.replace("\n", binary_nl)
    n = new.replace("\n", binary_nl)
    if n in raw:
        print(f"  = already applied: {label}")
        return
    if o not in raw:
        failures.append(f"anchor not found: {label} ({rel})")
        print(f"  ! ANCHOR NOT FOUND: {label}")
        return
    p.write_bytes(raw.replace(o, n, 1).encode("utf-8"))
    print(f"  + applied: {label}")


CLOSURE_SECTION = """### Step 4: Source artifact cleanup (backlogit)

**Skip condition**: run this step ONLY when `mode` is `post-merge`. On a
`pre-merge` or `post-deploy` invocation, skip it entirely. Archiving a source
artifact before the PR merges would violate P-021 C5, which permits retirement
of the source stash entry only after a successful merge, and the merge commit
SHA required below does not exist yet at pre-merge time.

Retire the intake artifacts that produced this release unit so the backlog does
not accumulate orphaned sources.

1. Read the shipped work item's `custom_fields` for `source_stash_id` and
   `source_deliberation_id`. Use `backlogit_query_sql` (for example
   `SELECT id, json_extract(custom_fields, '$.source_stash_id') AS stash_id,
   json_extract(custom_fields, '$.source_deliberation_id') AS deliberation_id
   FROM items WHERE id = '<shipped_item_id>'`) rather than scanning markdown.
2. When `source_stash_id` is present and the stash entry is still `active`,
   archive it with `backlogit_stash_archive`. Prefer archival over
   `backlogit_stash_remove` — archiving preserves traceability, removal is
   destructive and deprecated.
3. When `source_deliberation_id` is present and the deliberation is fully
   superseded by shipped work, archive it with `backlogit_archive_item` and pass
   the merge commit SHA so the decision remains traceable to the release.
4. Never delete a source artifact. If either ID is missing, unreadable, or points
   at an item that no longer exists, record the discrepancy as a closure
   follow-up and continue — cleanup is never a release blocker.
5. Record which source artifacts were archived in the closure artifact so the
   stash-to-shipment chain stays auditable.

### Step 5: Feed Back into the Harness"""


def main() -> int:
    print(f"staged renders installed: {install_staged()}")
    print("re-applying overlay edits:")

    # 1. P-013.5/H7 propagation: tokens must sit within the checker's 600-char window.
    replace(
        ".github/agents/_orchestrator.agent.md",
        "* **Propagate to inherited skills (P-013.5/H7)**: skills invoked by Stage/Ship at\n"
        "  depth 2 (see the Subagent Depth table below) do not carry an independent\n"
        "  `model_family`/`model_provider` binding of their own — they execute inside the\n"
        "  depth-1 agent's own invocation, so the depth-1 override declared above already\n"
        "  covers them. There is nothing separate to \"resolve\" or \"reload\" at the skill\n"
        "  layer. If a runtime ever exposes an independent per-skill model binding that\n"
        "  could diverge from its parent invocation's override, that divergence is itself\n"
        "  a `ROUTING_DEGRADED` condition and must be declared, never silently absorbed.",
        "* **Propagate to inherited skills (P-013.5/H7)**: depth-2 skills invoked by Stage/Ship\n"
        "  have no independent model binding of their own — they execute inside the depth-1\n"
        "  agent's own invocation, so the override declared above already covers them. Nothing\n"
        "  is separately resolved or reloaded at the skill layer. If a runtime ever exposed an\n"
        "  independent per-skill binding diverging from its parent override, that divergence is\n"
        "  itself a `ROUTING_DEGRADED` condition and must be declared, never silently absorbed.",
        "orchestrator P-013.5/H7 propagation window",
    )

    # 2. P-001 release-closure exact phrase.
    replace(
        ".github/agents/_ship.agent.md",
        "Treat the shipment as still active for P-001 purposes, and do not allow another top-level release unit to begin yet.",
        "Treat the shipment as still active for P-001 purposes: another top-level release unit may not begin yet.",
        "ship P-001 release-closure phrase",
    )

    # 3. Split so the prohibition scopes to the retry clause (no coordinating conjunction).
    replace(
        ".github/agents/_stage.agent.md",
        "replaces that fallback or authorizes another execution attempt.",
        "replaces that fallback. It never authorizes another execution attempt.",
        "stage escalation retry prohibition",
    )

    # 4. backlogit overlay: source artifact cleanup, gated to post-merge.
    replace(
        ".github/skills/operational-closure/SKILL.md",
        "### Step 4: Feed Back into the Harness",
        CLOSURE_SECTION,
        "closure source-artifact cleanup (post-merge gated)",
    )
    replace(
        ".github/skills/operational-closure/SKILL.md",
        "* Ownership and validation windows are recorded\n",
        "* Ownership and validation windows are recorded\n"
        "* Source stash and deliberation artifacts are archived (never deleted) after the release unit ships\n",
        "closure quality-criteria bullet",
    )

    # 5. Pre-push gate composition: only local_gating.pre_push_gates, in order,
    #    with a format command that actually fails (gofmt -l always exits 0).
    replace(
        "scripts/pre-push-quality-gates.sh",
        'run_gate "Lint" "golangci-lint" "golangci-lint run"\n'
        'run_gate "Format" "gofmt" "gofmt -l ."\n'
        'run_gate "Typecheck" "" ""\n'
        'run_gate "Test" "go" "go test ./..."\n'
        'run_gate "Build" "go" "go vet ./..."\n',
        'run_gate "Format" "gofmt" "test -z \\"\\$(gofmt -l .)\\" || { gofmt -l .; false; }"\n'
        'run_gate "Lint" "golangci-lint" "golangci-lint run"\n'
        'run_gate "Test" "go" "go test ./..."\n'
        'run_gate "Build" "go" "go vet ./..."\n',
        "pre-push sh gate composition",
        binary_nl="\n",
    )
    replace(
        "scripts/pre-push-quality-gates.ps1",
        'Invoke-Gate "Lint" "golangci-lint" "golangci-lint run"\n'
        'Invoke-Gate "Format" "gofmt" "gofmt -l ."\n'
        'Invoke-Gate "Typecheck" "" ""\n'
        'Invoke-Gate "Test" "go" "go test ./..."\n'
        'Invoke-Gate "Build" "go" "go vet ./..."\n',
        "Invoke-Gate \"Format\" \"gofmt\" "
        "'$unformatted = gofmt -l .; if ($unformatted) { "
        "Write-Host ($unformatted -join [Environment]::NewLine); "
        "$global:LASTEXITCODE = 1 } else { $global:LASTEXITCODE = 0 }'\n"
        'Invoke-Gate "Lint" "golangci-lint" "golangci-lint run"\n'
        'Invoke-Gate "Test" "go" "go test ./..."\n'
        'Invoke-Gate "Build" "go" "go vet ./..."\n',
        "pre-push ps1 gate composition",
        binary_nl="\r\n",
    )

    # 6. The capability-pack enforcement coordinator is checked with an
    #    EOL-normalized (LF) hash, while the general checksum scan hashes raw
    #    bytes. Writing it as LF makes both hashes agree.
    enf = WS / ".github/instructions/capability-pack-enforcement.instructions.md"
    if enf.exists():
        raw = enf.read_bytes()
        if b"\r\n" in raw:
            enf.write_bytes(raw.replace(b"\r\n", b"\n"))
            print("  + applied: enforcement instruction normalized to LF")
        else:
            print("  = already applied: enforcement instruction LF")

    # 7. Preserve the Engram-injected block (tool-managed, not template-derived).
    ci = WS / ".github/copilot-instructions.md"
    block_path = WS / ".autoharness/backups/2026-09-03/engram-block.md"
    t = ci.read_text(encoding="utf-8")
    if "<!-- engram:start -->" in t:
        print("  = already applied: engram block")
    elif not block_path.exists():
        # Backups are gitignored, so a fresh clone has no copy to restore. The
        # Engram daemon re-injects its own block, so this is a warning, not a failure.
        print("  ~ skipped: engram block (no saved copy; Engram will re-inject)")
    else:
        block = block_path.read_text(encoding="utf-8")
        ci.write_text(t.rstrip("\n") + "\n\n" + block.rstrip("\n") + "\n", encoding="utf-8")
        print("  + applied: engram block")

    if failures:
        print("\nFAILURES:")
        for f in failures:
            print("  -", f)
        return 1
    print("\nall overlay edits applied")
    return 0


sys.exit(main())
