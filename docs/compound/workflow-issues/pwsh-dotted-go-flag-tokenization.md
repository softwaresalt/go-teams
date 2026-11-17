---
type: compound-learning
category: workflow-issues
date: 2026-11-16
tags: [ci, github-actions, windows, pwsh, powershell, go-test, coverage]
evidence: "PR #78 (032-S) Anchor Review Round 1 + CI run 44005115860 on windows-latest"
---

# PowerShell splits unquoted dotted Go flags on windows-latest runners

## Problem

Moving `go test -race -coverprofile=coverage.out ./...` and `go tool cover -func=coverage.out` from `ubuntu-latest` (`bash`) to `windows-latest` (whose default `run:` shell is `pwsh`) causes PowerShell's argument-mode parser to split unquoted `-flag=name.ext` tokens at `.`, passing `-coverprofile=coverage` and `.out` (or `-func=coverage` and `.out`) as separate arguments. `go test` then treats `.out` as a package pattern and `go tool cover` fails with `too many arguments`.

## Root cause

In PowerShell argument mode, an unquoted token beginning with `-` (`-coverprofile=coverage.out`) is parsed as a parameter name/value up to the dot (`.`), splitting `.out` into a subsequent argument.

## Resolution

Quote the full flag-and-value token in double quotes (or set `shell: bash`):

```yaml
- name: Test
  run: go test -race "-coverprofile=coverage.out" ./...
- name: Coverage report
  run: go tool cover "-func=coverage.out"
```

Quoting the complete argument works identically under `pwsh` and `bash`.

## Prevention

* Quote any `-flag=value` argument containing `.` or punctuation when running Go commands in `windows-latest` GitHub Actions steps.
* Enforce the quoted command form in `internal/config/workflow_test.go`.
