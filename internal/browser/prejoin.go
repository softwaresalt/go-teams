package browser

import "fmt"

// Teams pre-join and in-meeting DOM selectors.
//
// VERIFY LIVE: Microsoft Teams changes its DOM frequently. These selectors are
// derived from the architecture spec and public references and MUST be
// confirmed and adjusted via the integration harness (TestMeetingJoin) against
// a live meeting before the FSM can be considered working.
const (
	preJoinContainerSel = `[data-tid="prejoin-join-button"]`
	micToggleSel        = `[data-tid="toggle-mute"]`
	displayNameInputSel = `input[data-tid="prejoin-display-name-input"]`
	joinNowSel          = `[data-tid="prejoin-join-button"]`
	meetingToolbarSel   = `[data-tid="hangup-button"]`
	lobbyText           = "Someone will let you in shortly"
)

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

// existsJS returns a JavaScript expression that evaluates to true when selector
// matches an element in the document.
func existsJS(selector string) string {
	return fmt.Sprintf(`document.querySelector(%q) !== null`, selector)
}

// containsTextJS returns a JavaScript expression that evaluates to true when the
// document body text contains substr.
func containsTextJS(substr string) string {
	return fmt.Sprintf(`document.body !== null && document.body.innerText.includes(%q)`, substr)
}
