package browser

import "fmt"

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
	lobbyText           = "Someone will let you in shortly"
)

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
