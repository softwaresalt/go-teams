package browser

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Teams launcher, pre-join, and in-meeting DOM selectors.
//
// Confirmed against live Microsoft Teams web DOM (2026-09-05), supporting both
// the guest lightweight client and the authenticated full v2 client while
// preserving legacy data-tid selectors.
const (
	joinOnWebSel        = `button[data-tid="joinOnWeb"], [data-tid="joinOnWeb"]`
	micToggleSel        = `[data-tid="toggle-mute"]`
	displayNameInputSel = `input[data-tid="prejoin-display-name-input"]`
	joinNowSel          = `[data-tid="prejoin-join-button"]`
	meetingToolbarSel   = `#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]`
	callEndedScreenSel  = `[data-tid="call-ended-screen"], [data-tid="post-call-screen"], [data-tid="rejoin-call-button"], [data-tid="call-quality-rating"]`
	lobbyText           = "Someone will let you in shortly"
)

type callEndedProbeResult struct {
	ToolbarPresent  bool `json:"toolbarPresent"`
	CallEndedScreen bool `json:"callEndedScreen"`
	RecapView       bool `json:"recapView"`
}

// shouldNotifyCallEnded evaluates a callEndedProbeJS JSON payload and updates
// missingToolbarPolls, returning true when an explicit call-ended screen is
// visible without the active toolbar or when the toolbar has been absent for 3
// consecutive polls outside a Meeting Recap view.
func shouldNotifyCallEnded(raw string, missingToolbarPolls *int) bool {
	var res callEndedProbeResult
	if err := json.Unmarshal([]byte(raw), &res); err != nil || res.RecapView {
		if missingToolbarPolls != nil {
			*missingToolbarPolls = 0
		}
		return false
	}
	if res.ToolbarPresent {
		if missingToolbarPolls != nil {
			*missingToolbarPolls = 0
		}
		return false
	}
	if res.CallEndedScreen {
		return true
	}
	if missingToolbarPolls != nil {
		*missingToolbarPolls++
		return *missingToolbarPolls >= 3
	}
	return false
}

// callEndedProbeJS returns a JavaScript expression that evaluates whether the
// live call toolbar has disappeared or an explicit post-call / call-ended
// surface is visible.
func callEndedProbeJS() string {
	return `JSON.stringify((() => {
	const toolbar = document.querySelector('#hangup-button, [data-tid="hangup-button"], [data-tid="hangup-main-btn"]') !== null;
	const endedSel = document.querySelector('[data-tid="call-ended-screen"], [data-tid="post-call-screen"], [data-tid="rejoin-call-button"], [data-tid="call-quality-rating"]') !== null;
	const bodyText = (document.body && document.body.innerText) || '';
	const endedBanner = !toolbar && (bodyText.includes('You left the meeting') || bodyText.includes('Call ended'));
	const recap = document.querySelector('[data-tid="intelligent-recap-header"]') !== null;
	return {
		toolbarPresent: toolbar,
		callEndedScreen: !toolbar && (endedSel || endedBanner),
		recapView: recap
	};
})())`
}

// PreJoinState classifies the browser page state while waiting for user
// authentication, launcher bypass, pre-join configuration, or direct call
// entry.
type PreJoinState int

const (
	// PreJoinWaitingForUser means the browser is on a login/MFA screen or
	// Teams navigation surface and has not yet reached launcher, pre-join,
	// lobby, or an active call.
	PreJoinWaitingForUser PreJoinState = iota
	// PreJoinLauncher means the /dl/launcher/launcher.html interstitial is
	// visible and "Continue on this browser" (joinOnWeb) should be clicked.
	PreJoinLauncher
	// PreJoinReady means the pre-join screen is visible and ready for mute,
	// name injection, and clicking "Join now".
	PreJoinReady
	// PreJoinInLobby means the session is already waiting in the meeting lobby.
	PreJoinInLobby
	// PreJoinAdmitted means the active call toolbar is already present.
	PreJoinAdmitted
)

// classifyPreJoinState maps DOM presence probes to the highest-precedence
// PreJoinState.
func classifyPreJoinState(toolbarPresent, lobbyPresent, prejoinPresent, launcherPresent bool) PreJoinState {
	switch {
	case toolbarPresent:
		return PreJoinAdmitted
	case lobbyPresent:
		return PreJoinInLobby
	case prejoinPresent:
		return PreJoinReady
	case launcherPresent:
		return PreJoinLauncher
	default:
		return PreJoinWaitingForUser
	}
}

// AdmissionState classifies the meeting entry state after clicking "Join now".
type AdmissionState int

const (
	// AdmissionUnknown means neither the meeting toolbar nor the lobby prompt
	// has been detected yet.
	AdmissionUnknown AdmissionState = iota
	// AdmissionAdmitted means the meeting control toolbar is present.
	AdmissionAdmitted
	// AdmissionInLobby means the lobby waiting prompt is present.
	AdmissionInLobby
)

// classifyAdmission maps the presence of the meeting toolbar and lobby prompt
// to an AdmissionState. The toolbar takes precedence: if both are present, the
// bot is considered admitted.
func classifyAdmission(toolbarPresent, lobbyPresent bool) AdmissionState {
	switch {
	case toolbarPresent:
		return AdmissionAdmitted
	case lobbyPresent:
		return AdmissionInLobby
	default:
		return AdmissionUnknown
	}
}

// muteRequired reports whether the microphone toggle must be clicked to enter
// the meeting muted, given the toggle's aria-checked attribute value. A value
// of "true" means the mic is currently enabled and must be toggled off.
func muteRequired(ariaChecked string) bool {
	return ariaChecked == "true"
}

// micStateRequiresMute reports whether the microphone toggle must be clicked
// to enter the meeting muted, checking both the boolean DOM checked property
// (Teams v2 switch input) and the legacy aria-checked attribute.
func micStateRequiresMute(checked bool, ariaChecked string) bool {
	return checked || muteRequired(ariaChecked)
}

// existsJS returns a JavaScript expression that evaluates to true when selector
// matches an element in the document.
func existsJS(selector string) string {
	return fmt.Sprintf(`document.querySelector(%q) !== null`, selector)
}

// clickJS returns a JavaScript expression that invokes .click() on the first
// element matching selector and returns true when matched.
func clickJS(selector string) string {
	return fmt.Sprintf(`(() => { const el = document.querySelector(%q); if (el) { el.click(); return true; } return false; })()`, selector)
}

// propBoolJS returns a JavaScript expression that safely reads a boolean
// property on the first element matching selector.
func propBoolJS(selector, prop string) string {
	return fmt.Sprintf(`(() => { const el = document.querySelector(%q); return Boolean(el && el[%q]); })()`, selector, prop)
}

// propStringJS returns a JavaScript expression that safely reads a string
// property on the first element matching selector.
func propStringJS(selector, prop string) string {
	return fmt.Sprintf(`(() => { const el = document.querySelector(%q); return el && typeof el[%q] === "string" ? el[%q] : ""; })()`, selector, prop, prop)
}

// containsTextJS returns a JavaScript expression that evaluates to true when the
// document body text contains substr.
func containsTextJS(substr string) string {
	return fmt.Sprintf(`document.body !== null && document.body.innerText.includes(%q)`, substr)
}

// RedactAccountIdentifier masks a configured Microsoft Entra UPN/email or
// display name before logging so raw account identifiers never appear in logs.
func RedactAccountIdentifier(account string) string {
	return redactAccountIdentifier(account)
}

// redactAccountIdentifier masks a configured Microsoft Entra UPN/email or
// display name before logging so raw account identifiers never appear in logs.
func redactAccountIdentifier(account string) string {
	trimmed := strings.TrimSpace(account)
	if trimmed == "" {
		return ""
	}
	if at := strings.IndexByte(trimmed, '@'); at > 0 {
		local := trimmed[:at]
		domain := trimmed[at:]
		r, _ := utf8.DecodeRuneInString(local)
		return string(r) + "***" + domain
	}
	r, _ := utf8.DecodeRuneInString(trimmed)
	return string(r) + "***"
}

// selectEntraAccountJS returns a JavaScript expression that detects a Microsoft
// Entra / Microsoft account picker surface and clicks the single tile matching
// account case-insensitively. It fails closed when zero or multiple tiles match,
// skips #otherTile ("Use another account"), avoids password-entry and MFA
// screens, and enforces a 3000ms per-tile click cooldown.
func selectEntraAccountJS(account string) string {
	target := strings.ToLower(strings.TrimSpace(account))
	if target == "" {
		return "false"
	}
	encoded, err := json.Marshal(target)
	if err != nil {
		return "false"
	}
	return fmt.Sprintf(`(() => {
	try {
		const target = %s;
		if (!target || target === "othertile" || target === "use another account") {
			return false;
		}
		const proto = (location.protocol || '').toLowerCase();
		const host = (location.hostname || '').toLowerCase();
		const isLoopbackOrBlank = proto === 'about:' || host === 'localhost' || host === '127.0.0.1' || host === '::1' || host === '[::1]';
		const authHosts = [
			'login.microsoftonline.com',
			'login.microsoft.com',
			'login.live.com',
			'account.live.com',
			'login.windows.net',
			'login.microsoftonline.us',
			'login.partner.microsoftonline.cn'
		];
		const isTrustedAuthOrigin = proto === 'https:' && authHosts.some(h => host === h || host.endsWith('.' + h));
		if (!isLoopbackOrBlank && !isTrustedAuthOrigin) {
			return false;
		}
		const isVisible = (node) => {
			if (!node || typeof node.getBoundingClientRect !== 'function') {
				return false;
			}
			const rect = node.getBoundingClientRect();
			if (!rect || rect.width <= 0 || rect.height <= 0) {
				return false;
			}
			let cur = node;
			while (cur && cur.nodeType === 1) {
				if (cur.hidden || cur.getAttribute('aria-hidden') === 'true') {
					return false;
				}
				const st = window.getComputedStyle(cur);
				if (st && (st.display === 'none' || st.visibility === 'hidden' || st.opacity === '0')) {
					return false;
				}
				cur = cur.parentElement;
			}
			return true;
		};
		const pickerSurfaceSelectors = '#tilesHolder, div[data-bind*="acctTile"], [data-testid="accountTile"], #otherTile, [data-test-id="otherTile"], [data-bind*="otherTile"]';
		const pickerSurfaces = Array.from(document.querySelectorAll(pickerSurfaceSelectors));
		if (!pickerSurfaces.some(isVisible)) {
			return false;
		}
		const challengeSelector = 'input[type="password"]:not([aria-hidden="true"]), input[name="otc"]:not([aria-hidden="true"]), input[autocomplete="one-time-code"]:not([aria-hidden="true"]), #idTxtBx_SAOTCC_OTC, #idDiv_SAOTCS_Proofs, #idRichContext_DisplaySign, [data-testid="displaySign"]';
		const challengeInputs = Array.from(document.querySelectorAll(challengeSelector));
		if (challengeInputs.some(isVisible)) {
			return false;
		}
		const selector = '#tilesHolder [data-test-id], #tilesHolder div.table[role="button"], div.table[role="button"][data-test-id], div[data-bind*="acctTile"], [data-testid="accountTile"], #tilesHolder div[role="listitem"] button[data-test-id]';
		const rawNodes = Array.from(document.querySelectorAll(selector));
		if (rawNodes.length === 0) {
			return false;
		}
		const seen = new Set();
		const tiles = [];
		for (const node of rawNodes) {
			const tile = node.closest('div.table[role="button"], div[data-bind*="acctTile"], [data-testid="accountTile"], button[data-test-id]') || node;
			if (!seen.has(tile)) {
				seen.add(tile);
				tiles.push(tile);
			}
		}
		const matches = [];
		for (const el of tiles) {
			if (!isVisible(el)) {
				continue;
			}
			const rawId = (el.getAttribute('id') || '').trim();
			const rawTestId = (el.getAttribute('data-test-id') || '').trim();
			const rawDataBind = (el.getAttribute('data-bind') || '').trim();
			const id = rawId.toLowerCase();
			const testId = rawTestId.toLowerCase();
			const dataBind = rawDataBind.toLowerCase();
			if (rawId === 'otherTile' || rawTestId === 'otherTile' || rawDataBind.includes('otherTile') || id === 'othertile' || testId === 'othertile' || dataBind.includes('othertile')) {
				continue;
			}
			let matched = false;
			if (testId !== '' && testId === target) {
				matched = true;
			}
			if (!matched) {
				const subNodes = [el, ...Array.from(el.querySelectorAll('[data-test-id], .table-cell, small, div, span'))];
				for (const sub of subNodes) {
					if (!isVisible(sub)) {
						continue;
					}
					const subTestId = (sub.getAttribute && sub.getAttribute('data-test-id') || '').trim().toLowerCase();
					if (subTestId === 'othertile') {
						matched = false;
						break;
					}
					if (subTestId !== '' && subTestId === target) {
						matched = true;
						break;
					}
					const rawText = (sub.innerText || sub.textContent || '');
					const lines = rawText.split(/\r?\n/).map(s => s.trim().toLowerCase()).filter(Boolean);
					if (lines.includes(target)) {
						matched = true;
						break;
					}
				}
			}
			if (matched) {
				matches.push(el);
			}
		}
		if (matches.length !== 1) {
			return false;
		}
		const chosen = matches[0];
		const now = Date.now();
		const prev = Number(chosen.dataset.teamsCliAutoSelected || '0');
		if (prev > 0 && now - prev < 3000) {
			return false;
		}
		chosen.dataset.teamsCliAutoSelected = String(now);
		chosen.click();
		return true;
	} catch (_) {
		return false;
	}
})()`, string(encoded))
}
