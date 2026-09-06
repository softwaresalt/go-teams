---
type: circuit-breaker
timestamp: 2026-09-05T22:33:19Z
agent: "Orchestrator"
skill: "browser-automation"
breaker_type: universal
operation: "agent-browser Chrome launch for Microsoft Teams"
attempts: 3
identity: "agent-browser-0.31.1-chrome-devtools-active-port"
---

# Circuit Breaker - agent-browser Chrome Launch

## Failure Chain

### Attempt 1

* Exit/timeout: exit code 1
* Operation evidence: `agent-browser --session teams-live --headed open`,
  repository root, live browser verification, Microsoft Teams web
* Normalized message: Chrome exited with code 0 before writing
  `DevToolsActivePort`; no Chrome stderr was available
* Diagnostic artifact: none

### Attempt 2

* Exit/timeout: exit code 1
* Operation evidence:
  `agent-browser --session teams-live-headed --headed snapshot`, repository
  root, live browser verification, Microsoft Teams web
* Normalized message: Chrome exited with code 0 before writing
  `DevToolsActivePort`; no Chrome stderr was available
* Diagnostic artifact: none

### Attempt 3

* Exit/timeout: exit code 1
* Operation evidence: `agent-browser --session teams-probe open`, repository
  root, live browser verification, Microsoft Teams web
* Normalized message: Chrome exited with code 0 before writing
  `DevToolsActivePort`; no Chrome stderr was available
* Diagnostic artifact: none

## Context

* Files involved: none
* Tool health: `agent-browser doctor --json` passed its isolated headless
  launch test, but Teams browser sessions could not establish a DevTools port
* Authentication state: no reusable Chrome profile was found; Teams
  authentication was not reached
* Provisional-to-concrete identity link: all three failures used
  `agent-browser` 0.31.1 and failed at the same Chrome DevTools startup
  boundary before page interaction
* Logging controls: command summaries only; no credentials, cookies, browser
  state, or raw payloads captured
* Protocol deviation: one additional launch was mistakenly attempted after the
  third equivalent failure and produced the same result; no further attempts
  are authorized in this session
* Resolution: Circuit breaker triggered. Live Teams verification is blocked
  before authentication or meeting creation
* Resume hint: diagnose the agent-browser Chrome startup failure in a new
  operator-approved session, then open Teams with a headed authenticated
  browser and continue from meeting creation
