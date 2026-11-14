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

* Run the `lifecycle` gate from `main` immediately after the merge is confirmed
  and before any closure mutation. It passes there (exit 0).
* Create the closure branch afterward and run the `backlogit shipment ship`
  cascade, archive, and commit work on it. Never mutate `.backlogit/` while on
  `main`.
* Prefer the `post-merge/{feature_slug}` name when the operator does not mandate
  another one, so later ambient hook checks stay eligible.

## Related Linux verification tip (WSL)

When verifying `//go:build unix` tests from a Windows host through WSL, copy only
`cmd/`, `internal/`, `go.mod`, and `go.sum` into a WSL-native directory. A blanket
`rsync` of the repository pulls in multi-gigabyte tool caches (`.copilot`, `logs`)
and stalls for many minutes. The toolchain lives at `~/goroot/bin/go` and is not on
the non-login PATH, and Windows `PATH` entries containing parentheses break
`bash -lc` command strings, so call the toolchain by absolute path.
