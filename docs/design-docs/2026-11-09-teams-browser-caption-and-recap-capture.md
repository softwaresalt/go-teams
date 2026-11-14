---
title: Teams browser caption and recap capture
date: 2026-11-13
status: implemented
feature: 011-F, 013-F, 015-F, 017-F, 021-F
shipment: 011-S, 013-S, 015-S, 017-S, 021-S
deliberation: 002-DL, 006-DL, 010-DL
---

## Architecture and dependency direction

`cmd/teams-cli` enters `internal/cli`, which resolves configuration and passes
browser and output settings into `internal/browser` and `internal/capture`.
Capture coordinates the `browser.Meeting` interface and writes through
`internal/transcript`; browser code does not own output files. URL mutation
belongs to `internal/teams`. Configuration persistence remains in
`internal/config`.

## Browser launch and authentication

The capture path discovers Chrome, Edge, or Brave, or uses `--browser-path`
and `TEAMS_BROWSER_PATH`. It launches a browser with a loopback CDP port,
optional user-data directory, and `--headless=new` only when requested.
Clean capture launch omits `AutomationControlled` and keeps the Chromium
sandbox enabled (see [Browser sandbox posture](#browser-sandbox-posture)).
This does not redefine the separate legacy authentication allocator.

Headed mode supports operator sign-in and meeting admission. When a profile
directory lacks an initialized Teams Local Storage session, headed Meeting
Recap capture bootstraps authentication at the matching `/v2/` origin
(`https://teams.microsoft.com/v2/` or `https://teams.cloud.microsoft/v2/`)
before navigating to the recap URL, and tracks OAuth callback completion and
single-tab COOP `targetId` swaps across Microsoft identity hosts without
rebinding to concurrent tabs. Headless mode requires an already usable profile
and authorized access. `config.DefaultConfig()` sets `ProfileDir` to
`%LOCALAPPDATA%\teams-cli\profile` on Windows when `LOCALAPPDATA` is set, and
otherwise to `teams-cli/profile` under `os.UserConfigDir()` (for example
`~/.config/teams-cli/profile` on Linux). The browser package performs no
fallback search: when `Options.UserDataDir` is empty, as with a direct library
caller that bypasses `DefaultConfig`, behavior depends on the launch path. The
chromedp allocator paths pass no user-data directory, so chromedp uses a
temporary profile and the session is not persisted. The direct-exec headed
launcher omits `--user-data-dir`, so Chromium uses its own default profile,
which may retain credentials. Automation should supply
`--profile-dir` explicitly. Browser profiles contain credentials and must
never be committed or shared.

## Meeting URL validation and redaction

Decision `002-DL` (shipment `013-S`) puts a trust boundary in front of browser
launch and keeps secrets out of every diagnostic.

**Ingress allowlist.** `teams.ValidateMeetingURL` is the single source of truth
for whether an initial meeting URL is acceptable. It runs before any browser is
allocated: `ChromeRunner.Join`, the live meeting path in `internal/browser`, and
`teams.MutateJoinURL` all call it, so a validation failure never starts a
process. The rules are:

* the URL must parse, use `http` or `https`, and carry a host
* embedded userinfo (`user:password@host`) is rejected
* loopback hosts (`localhost`, `127.0.0.1`, `::1`) may use `http` or `https` so
  synthetic fixtures keep working
* every other host must use `https` and equal, or be a dot-delimited subdomain
  of, a trusted Microsoft suffix: `teams.microsoft.com`, `teams.live.com`,
  `teams.cloud.microsoft`, `sharepoint.com`, `microsoft.com`, `office.com`,
  `sfx.ms`, or `aka.ms`

Validation error messages name the scheme or host but never echo the raw URL.
The check covers the initial URL only. Redirects that the browser follows after
navigation are not intercepted; this is a documented residual risk, accepted
because the meeting flow legitimately passes through Microsoft identity hosts.

**Redaction.** `teams.RedactURL` reduces a URL to `scheme://host/path`,
removing userinfo, the query string (which carries the `?p=` passcode and OAuth
parameters), and the fragment. For input that cannot be parsed, it strips query
and fragment text before splitting the path and returns a placeholder rather
than risk leaking userinfo. `teams.RedactArgs` applies the same rule to every
URL-shaped token in an argument list. Navigation errors, browser logs, and the
batch queue logs and parse errors use these helpers; an unterminated quote in a
queue line reports the problem without printing the line.

**Rationale.** Meeting URLs are bearer links, and a transcription bot that
navigates to attacker-supplied hosts could expose the signed-in profile. A
host allowlist at the one ingress, plus redaction at every egress, is cheaper
and easier to audit than scattered checks.

## Browser sandbox posture

Decisions `002-DL` and `006-DL` (shipments `013-S` and `017-S`) keep the
Chromium sandbox enabled by default on every launch path and give containers an
explicit opt-out.

* `013-S` introduced `Options.DisableSandbox` and the `TEAMS_DISABLE_SANDBOX`
  environment variable for the `chromedp` allocator paths (`AllocatorOptions`,
  `StealthFlags`, and the headless meeting flags).
* `017-S` extended the same opt-in to the direct-exec launcher. Before that,
  `join --headed` and `captions` sessions started through `exec.Command` never
  consulted the opt-in, so operators in root or container environments could
  not disable the sandbox even with the variable set.
* All paths now share one predicate, `shouldDisableSandbox`. It is true only
  when `Options.DisableSandbox` is set or `TEAMS_DISABLE_SANDBOX` is `1` or
  `true` (case-insensitive, trimmed).
* `cleanBrowserArgsForOptions` builds the direct-exec arguments. When the
  predicate is false, it emits neither `--no-sandbox` nor
  `--disable-setuid-sandbox`. When it is true, it appends both flags before the
  trailing `initialURL`, so the URL stays the last argument.
* No flag, config key, or heuristic turns the opt-out on automatically; an
  operator must choose it.

**Rationale.** One shared predicate keeps the allocator and direct-exec paths
from drifting apart, and a default-on sandbox means a workstation never loses a
security boundary without someone asking for it.

## Microsoft Entra account-picker auto-selection

Decision `010-DL` (shipment `021-S`) adds opt-in automatic selection of a
configured Microsoft Entra or Microsoft account tile when a browser profile
contains multiple cached sessions and redirects through the "Pick an account"
interstitial (`#tilesHolder`).

* **Configuration and CLI surface.** `Config.DefaultAccount` reads
  `default_account` with YAML aliases `entra_account` and `account` in
  deterministic precedence (`default_account` > `entra_account` > `account`),
  trimmed with `strings.TrimSpace`. The `--account` CLI flag overrides the
  configured value across the root command, `join`, `captions`, `auth`, and
  `schedule`, and is forwarded by `inheritedQueueFlags` in batch queue runs. An
  explicit empty `--account=` clears a configured `default_account` (`cmd.Flags().Changed("account")`).
* **In-browser DOM selection (`selectEntraAccountJS`).** When `DefaultAccount`
  is non-empty, `ChromeRunner.run`, `chromeMeeting.joinHeaded`, and
  `chromeMeeting.awaitPreJoinOrAdmission` (`PreJoinWaitingForUser`) evaluate
  `selectEntraAccountJS(account)`:
  * The normalized lowercase target string is serialized with `json.Marshal` so
    arbitrary characters in `config.yaml` or `--account` cannot break JavaScript
    syntax or inject script statements.
  * Execution requires a trusted Microsoft authentication origin (`https:` on
    `login.microsoftonline.com`, `login.microsoft.com`, `login.live.com`,
    `account.live.com`, `login.windows.net`, `login.microsoftonline.us`,
    `login.partner.microsoftonline.cn`, or their dot-delimited subdomains; plus
    `about:` and loopback hosts for synthetic tests) and a positively
    identified visible picker surface (`#tilesHolder`, `div[data-bind*="acctTile"]`,
    `[data-testid="accountTile"]`, or `otherTile`).
  * It returns `false` immediately if any visible password input
    (`input[type="password"]`) or MFA challenge control (`input[name="otc"]`,
    `input[autocomplete="one-time-code"]`, `#idTxtBx_SAOTCC_OTC`,
    `#idDiv_SAOTCS_Proofs`, `#idRichContext_DisplaySign`,
    `[data-testid="displaySign"]`) is active.
  * Candidate tiles (`#tilesHolder [data-test-id]`,
    `#tilesHolder div.table[role="button"]`,
    `div.table[role="button"][data-test-id]`, `div[data-bind*="acctTile"]`,
    `[data-testid="accountTile"]`,
    `#tilesHolder div[role="listitem"] button[data-test-id]`) must have a
    nonzero rendered bounding box and no hidden/collapsed ancestor
    (`isVisible`), and `#otherTile` ("Use another account") is always excluded.
  * Matching is case-insensitive and exact (never arbitrary substring
    containment): a tile matches only when its `data-test-id` attribute or a
    trimmed text line equals the configured UPN/email or display name. The
    script collects all matching visible tiles first and fails closed
    (`matches.length !== 1`) when zero or multiple tiles match, and enforces a
    3000 ms per-tile click cooldown (`dataset.teamsCliAutoSelected`).
* **Identifier redaction (`RedactAccountIdentifier`).** Whenever a tile
  selection is logged or a queue line containing `--account` / `--account=` is
  logged via `sanitizeQueueArgsForLog`, `RedactAccountIdentifier` masks the
  local part or display name (`a***@contoso.com` or `A***`) so raw account
  identifiers never appear in logs.

## Live caption path

The join flow handles browser launcher pages, pre-join controls, lobby
admission, and active-call detection. Caption activation handles the direct
caption menu and the nested language-and-speech menu.

A JavaScript `MutationObserver` watches Teams caption rows. A settle timer
avoids exporting every incremental rewrite of the same utterance.
CDP binding events enter a bounded Go payload pump, followed by transcript
deduplication and a caption sink. The pump drains accepted events on shutdown;
queue overflow is logged and can drop fragments. Capture flushes each written
caption so output is visible during a live session.

## Meeting Recap path

CDP target discovery inspects both pages and out-of-process iframes, including
`xplatplugins.aspx`. Transcript extraction first seeks the `.ms-List` React
item collection to avoid limiting export to currently rendered virtualized
rows. A DOM fallback reads transcript cells, speaker names, timestamps, and
text. The fallback can be incomplete when virtualization hides entries.

ISO duration or colon-separated recording offsets become `HH:MM:SS` labels,
and `Writer.WriteEvent` renders each utterance with a bold speaker header and
blockquote line (`**Speaker** (HH:MM:SS):\n> text`). Empty transcript events
are omitted. SharePoint `/:v:/p/` sharing links in `fileUrl` are ignored when
extracting recording filenames so opaque sharing tokens do not reject valid
`.mp4` names in `g_fileInfo.name`. Recap captions are finite, so this path
exits after writing the extracted entries instead of waiting for cancellation.

Recap audio is opt-in (`--audio`, `--recap-audio`, `capture_audio: true`, or
`audio: true`); `capture.Session` honors `CaptionsOnly` for Recap sessions, so
the default remains a transcript-only `.md`. When enabled, `CaptureAudio` runs
concurrently with transcript extraction. It evaluates an extraction expression
over CDP only in targets owned by the bound Recap page (the CDP port and page
ID are persisted at join, including headed launches) and rejects any target
whose recording identity (drive item, recording name, meeting token, or
thread) conflicts with, or cannot be positively matched to, the requested
Recap. The expression fetches the recording, decodes it with Web Audio, and
returns 44.1 kHz mono PCM chunks that `internal/audio.NewEncoder` encodes to
MP3. Size is bounded in layers: a known-duration check before fetch, a source
byte cap before decode, a decoded PCM cap before allocation, and a CDP
WebSocket frame/message cap before buffering. Browser-internal decode expansion
of unknown-duration streams within the source cap is an accepted residual risk.

## Teams AI summary

The page script selects Notes and AI notes when present, then invokes Copy all.
It intercepts `navigator.clipboard.writeText`, `navigator.clipboard.write`,
and copy-event plain text. This captures Teams-provided text; it does not
request a separately generated AI response.

Because the transcript iframe may load first, summary capture re-queries
`listCDPTargets(port)` on each of at most 20 attempts, separated by 500 ms.
Cancellation stops retry waiting. HTTP enumeration and individual CDP
evaluations retain their existing I/O timeouts, so this is an attempt bound,
not a strict ten-second wall-clock deadline.

If no nonempty summary is captured, the transcript still succeeds. When
available, `WriteAISummary` appends a horizontal divider and summary section.
Tenant policy, localization, and UI changes can prevent summary extraction.

## CLI and output contract

| Input | Meaning |
|---|---|
| `teams-cli "<url>" "<output.md>"` | Direct capture |
| `teams-cli captions "<url>" "<output.md>"` | Live or recap capture |
| `teams-cli join "<url>" "<output.md>"` | Explicit join/capture entrypoint |
| `-u/--url` | Input URL when positional URL is absent |
| `-q/--queue` | Batch queue file; one capture per line (see the README) |
| `-o/--output` | File for `.md`/`.markdown`, otherwise directory |
| `--output-file` | Explicit transcript path |
| `--output-dir` | Directory regardless of extension |
| `--output-folder` | Default directory for generated names; lowest flag precedence |
| `-c/--category` | Category for the transcript header and filename |
| `--filename-template`, `--name-template` | Filename pattern for directory output |
| `--account` | Default Microsoft Entra account (UPN/email or display name) to auto-select on the account picker interstitial |
| `--headless` | Headless clean-browser launch |
| `--audio` | Also capture meeting or Recap audio to a same-basename `.mp3` |
| `--recap-audio` | Also capture Meeting Recap audio to a same-basename `.mp3` |

Positional inputs override flags. Output settings resolve, from highest to
lowest precedence, as the positional output argument, `--output-file`,
`--output`, `--output-dir`, `--output-folder`, then the config keys
`output_file`, `output_folder`, and `output_dir`. Existing directory targets
remain directories. Explicit output selection overrides configured
`output_file`. Parent directories are created as needed. The
[README](../../README.md#output-target-precedence) documents the full
reference.

Explicit file destinations are truncated after admission, including their
same-basename MP3 companion when audio is enabled. Reusing these paths is
destructive to prior results; unique per-run paths are recommended. Generated
names use exclusive creation. Failure rollback is best-effort, not a
transactional backup of replaced files.

## Validation and known limits

Unit checks cover URL/output argument routing, configuration precedence,
recap offsets, launch flags, transcript rendering, and delayed target discovery.
A synthetic browser harness exercises live captions; an opt-in compiled
executable harness verifies all output forms against a local recap fixture
from a different working directory.

These fixtures establish the executable contract, not production Teams DOM
compatibility, transcript completeness, authorization, or audio fidelity.
Live qualification and rollback ownership are defined in the
[closure record](../closure/2026-11-09-011-S-review-and-closure.md).
