---
type: compound-learning
category: workflow-issues
date: 2026-11-14
tags: [pipeline-topology, post-merge-closure, branch-naming, wsl, go-test]
evidence: 023-S post-merge closure (PR 58 merge 121271a)
---

# Topology gate rejects `chore/close-*` closure branches

## Problem

`autoharness gate pipeline-topology --mode agent --shipment <S> --phase lifecycle`
returned `status: blocked`, `token: BRANCH_MISMATCH` (exit 1) when run from a
closure branch named `chore/close-023-S`. The gate treats only the shipment's own
branch and `post-merge/*` branches as ownership-eligible.

## Resolution

* Create an eligible `post-merge/{feature_slug}` branch from `main` after the
  merge is confirmed, and run the `lifecycle` gate there immediately before the
  `backlogit shipment ship` cascade. Halt if it fails, as the Ship agent requires.
  Do all closure mutations and commits on that branch, never on `main`.
* If an operator mandates a different branch name (as with `chore/close-023-S`),
  the gate cannot give a verdict on that branch. Treat running it on `main`
  immediately before the close as a degraded substitute, record the deviation in
  the closure record, and do not describe the close as gate-verified on the closure
  branch. 023-S closed this way; the deviation is recorded in its closure record.

## Related Linux verification tip (WSL)

When verifying `//go:build unix` tests from a Windows host through WSL, copy only
`cmd/`, `internal/`, `go.mod`, and `go.sum` into a WSL-native directory. A blanket
`rsync` of the repository pulls in multi-gigabyte tool caches (`.copilot`, `logs`)
and stalls for many minutes. The toolchain lives at `~/goroot/bin/go` and is not on
the non-login PATH, and Windows `PATH` entries containing parentheses break
`bash -lc` command strings, so call the toolchain by absolute path.
