---
type: compound-learning
category: workflow-issues
date: 2026-11-14
tags: [wsl, linux, go-test, cross-compile, posix-modes]
evidence: 024-S ship (PR 61)
---

# Running Linux tests when WSL has no Go toolchain

## Problem

The Ship checklist asks for a Linux run of `internal/config` tests. On this
host the WSL Ubuntu distribution had no `go` binary (the Windows
`C:\Program Files\Go\bin` appears on its `PATH` only as `.exe` files), so
`go test` inside WSL failed.

## Resolution

Cross-compile the package's test binary on Windows and execute it in WSL from
the package directory, because tests locate the repository root relative to
the working directory:

```powershell
$env:GOOS='linux'; $env:GOARCH='amd64'; $env:CGO_ENABLED='0'
go test -c -o .\bin\config.linux.test ./internal/config
Remove-Item Env:GOOS,Env:GOARCH,Env:CGO_ENABLED
wsl bash -c "cd /mnt/c/<repo>/internal/config && cp ../../bin/config.linux.test /tmp/config.test && chmod +x /tmp/config.test && /tmp/config.test -test.count=1"
```

Delete the binary afterward (`bin/` is gitignored). Copy it to `/tmp` first so
the executable bit works on the Linux filesystem. This exercises the POSIX
branches (`0o700` and `0o600` mode assertions) that are skipped on Windows.
