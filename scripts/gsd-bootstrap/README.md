---
title: "GSD checkout bootstrap"
---

From the repository root, provision the pinned GSD 1.6.0 executable and
`gsd-assets-v1.6.0.zip` in `.bin/` from the trusted GSD distribution. Then run:

```text
go run ./scripts/gsd-bootstrap
```

The command uses `.gsd/checkout-seed.json` and the checked-out artifacts to
create `.gsd/workspace-profile.yaml` and `.gsd/harness-manifest.yaml`. Both files
are ignored because they record the checkout's absolute path. The manifest
records the actual local bundle, config, profile, and artifact digests.

The bootstrap does not install tools, register clients, run launchers, or
read `.env.local` or `.gsd/carry-forward.yaml`. It leaves an existing pair
unchanged. If only one file exists, it fails without changing it; restore its
matching file from a trusted backup before proceeding.

If you install the optional GSD pre-push hooks, provision Node.js and
`markdownlint-cli` 0.49.1 before using the hooks on Markdown changes. The
topology script is not wired into the GitHub Actions workflow.

Use the workspace-local GSD executable to check the resulting installation:

```powershell
.\.bin\gsd.exe verify-workspace --workspace . --gsd-home .\.bin --json
.\.bin\gsd.exe harness resolve --workspace . --shipment 010-S --json
```

On Unix, use `./.bin/gsd` and `./.bin` instead. The verifier may write ignored
reports under `.gsd/staging/`. Bootstrapping neither claims a shipment nor
makes a pull request merge-ready.
