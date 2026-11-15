---
type: compound-learning
category: security-issues
date: 2026-11-14
tags: [redaction, cli, help, credentials, copilot-review]
evidence: 025-S ship (PR 64, thread PRRT_kwDOTStvas63lNfV)
---

# Redact free-form CLI arguments from the original text, not after truncation

## Problem

The root help `Note:` banner echoes a rejected positional argument such as
`[command]`. `teams.RedactArgs` only recognizes scheme-bearing URLs (`http:`,
`https:`, `://`), so a schemeless meeting URL with a passcode or userinfo was
echoed unchanged. A first fix that cut the redacted text at `?`/`#` and after the
last `@` still leaked secret prefixes: a password may itself contain those
delimiters (`//user:secret?pw@host/...` became `//user:secret`), and an embedded
`https:` token inside a schemeless password made `RedactArgs` keep the text before
it. Pre-PR review (rubber-duck and eight personas) missed this; Copilot review of
the PR found it.

## Resolution

* Dispatch on the original argument's prefix. An input that starts
  (case-insensitively, after trimming) with `http://` or `https://` goes through
  `teams.RedactArgs`, which strips userinfo, query, and fragment. This is prefix
  dispatch, not URL validation: a malformed URL with that prefix still reaches
  `RedactArgs`, and host or path text it retains can still contain secrets, so
  callers must not assume the output has a validated safe shape.
* Any other input containing `@`, `?`, `#`, `://`, or an embedded `http:` or
  `https:` token is replaced wholesale with `[redacted]`.
* Pin both delimiter-containing and scheme-token-containing passwords with
  regression tests that assert the secret text is absent from the output.

## Prevention

Never trim or partially redact text that may contain a secret; either the input
has a known safe shape (then use the structured redactor) or suppress it. Treat
every delimiter as attacker- or user-controlled when the secret itself can contain
it.
