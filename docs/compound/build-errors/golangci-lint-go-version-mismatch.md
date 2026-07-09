---
type: compound-learning
category: build-errors
date: 2026-07-09
tags: [ci, golangci-lint, go-version, github-actions]
evidence: PR #2 CI run 29048242596 (job 86222153818)
---

# golangci-lint CI fails on modules targeting a newer Go than the lint binary

## Problem

The re-enabled CI lint step failed immediately with:

```text
can't load config: the Go language version (go1.24) used to build golangci-lint
is lower than the targeted Go version (1.26)
```

`golangci-lint run` passed locally but failed in GitHub Actions.

## Root cause

golangci-lint refuses to analyze a module whose `go` directive is newer than the
Go version the golangci-lint **binary itself** was compiled with. The official
`v1.64.8` release binary (downloaded by `golangci/golangci-lint-action` in the
default `binary` install mode) is built with go1.24, while `go.mod` declares
`go 1.26` (forced by `chromedp v0.15.1` during `go mod tidy`). The local
golangci-lint happened to be a go1.26-built copy, which masked the issue.

## Resolution

Set `install-mode: goinstall` on `golangci/golangci-lint-action` so CI runs
`go install golangci-lint@<version>` using the runner's `setup-go` toolchain
(Go 1.26), producing a lint binary built with a Go version >= the module target.

```yaml
- name: Lint
  uses: golangci/golangci-lint-action@...
  with:
    version: v1.64.8
    install-mode: goinstall
    args: --timeout=5m
```

Trade-off: `goinstall` compiles golangci-lint on each run (adds ~2-3 min).

## Prevention

- When a dependency bumps the `go.mod` directive, verify the CI lint binary's
  build-Go is >= the module target, or use `goinstall`.
- Do not trust a locally passing `golangci-lint run` if the local binary was
  built with a different Go than CI installs.
- Alternative: migrate to golangci-lint v2 (built with newer Go) — requires the
  v2 config schema and a newer action major version.
