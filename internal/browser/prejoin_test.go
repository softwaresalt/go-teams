package browser

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyAdmission(t *testing.T) {
	tests := []struct {
		name           string
		toolbar, lobby bool
		want           AdmissionState
	}{
		{"admitted", true, false, AdmissionAdmitted},
		{"lobby", false, true, AdmissionInLobby},
		{"unknown", false, false, AdmissionUnknown},
		{"toolbar wins over lobby", true, true, AdmissionAdmitted},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, classifyAdmission(tc.toolbar, tc.lobby))
		})
	}
}

func TestMuteRequired(t *testing.T) {
	assert.True(t, muteRequired("true"))
	assert.False(t, muteRequired("false"))
	assert.False(t, muteRequired(""))
}

func TestMicStateRequiresMute(t *testing.T) {
	assert.True(t, micStateRequiresMute(true, ""))
	assert.True(t, micStateRequiresMute(false, "true"))
	assert.False(t, micStateRequiresMute(false, "false"))
	assert.False(t, micStateRequiresMute(false, ""))
}

func TestClassifyPreJoinState(t *testing.T) {
	tests := []struct {
		name                              string
		toolbar, lobby, prejoin, launcher bool
		want                              PreJoinState
	}{
		{"admitted takes highest precedence", true, true, true, true, PreJoinAdmitted},
		{"lobby takes precedence over prejoin", false, true, true, false, PreJoinInLobby},
		{"prejoin ready", false, false, true, true, PreJoinReady},
		{"launcher interstitial", false, false, false, true, PreJoinLauncher},
		{"waiting for user auth", false, false, false, false, PreJoinWaitingForUser},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyPreJoinState(tc.toolbar, tc.lobby, tc.prejoin, tc.launcher)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExistsJS(t *testing.T) {
	js := existsJS(`[data-tid="x"]`)
	assert.Contains(t, js, "querySelector")
	assert.Contains(t, js, "data-tid")
	assert.Contains(t, js, "!== null")
}

func TestContainsTextJS(t *testing.T) {
	js := containsTextJS("Someone will let you in shortly")
	assert.Contains(t, js, "innerText.includes")
	assert.Contains(t, js, "document.body")
	assert.Contains(t, js, "Someone will let you in shortly")
}

func TestRedactAccountIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		account string
		want    string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \t  ", ""},
		{"standard upn", "alice@contoso.com", "a***@contoso.com"},
		{"trimmed upn", "  Bob.Smith@contoso.com  ", "B***@contoso.com"},
		{"single char local part", "a@contoso.com", "a***@contoso.com"},
		{"display name", "Alice Smith", "A***"},
		{"single char display name", "X", "X***"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := redactAccountIdentifier(tc.account)
			assert.Equal(t, tc.want, got)
			if tc.want != "" && len(tc.account) > 3 {
				assert.NotContains(t, got, "alice")
				assert.NotContains(t, got, "Smith")
			}
		})
	}
}

func TestSelectEntraAccountJS(t *testing.T) {
	assert.Equal(t, "false", selectEntraAccountJS(""))
	assert.Equal(t, "false", selectEntraAccountJS("   \t "))

	js := selectEntraAccountJS(`Alice.O'Connor"test@\contoso.com`)
	assert.Contains(t, js, `#tilesHolder`)
	assert.Contains(t, js, `otherTile`)
	assert.Contains(t, js, `teamsCliAutoSelected`)
	assert.Contains(t, js, `3000`)
	assert.Contains(t, js, `alice.o'connor\"test@\\contoso.com`)
}

func TestCallEndedProbeJS(t *testing.T) {
	js := callEndedProbeJS()
	assert.Contains(t, js, "hangup-button")
	assert.Contains(t, js, "call-ended-screen")
	assert.Contains(t, js, "You left the meeting")
	assert.Contains(t, js, "Call ended")

	missing := 0
	assert.False(t, shouldNotifyCallEnded(`{"toolbarPresent":true,"callEndedScreen":false}`, &missing))
	assert.Equal(t, 0, missing)
	assert.False(t, shouldNotifyCallEnded(`{"toolbarPresent":false,"callEndedScreen":false}`, &missing))
	assert.Equal(t, 1, missing)
	assert.False(t, shouldNotifyCallEnded(`{"toolbarPresent":false,"callEndedScreen":false}`, &missing))
	assert.Equal(t, 2, missing)
	assert.True(t, shouldNotifyCallEnded(`{"toolbarPresent":false,"callEndedScreen":false}`, &missing))
	assert.Equal(t, 3, missing)

	missing = 0
	assert.True(t, shouldNotifyCallEnded(`{"toolbarPresent":false,"callEndedScreen":true}`, &missing))
	assert.False(t, shouldNotifyCallEnded(`{"toolbarPresent":false,"callEndedScreen":true,"recapView":true}`, &missing))
}

func TestChromeMeetingActivityMonitorAndLobbyTimeout(t *testing.T) {
	cm := newChromeMeeting(MeetingOptions{})
	var capCalled, audCalled, endCalled bool
	cm.SetActivityCallbacks(
		func() { capCalled = true },
		func() { audCalled = true },
		func() { endCalled = true },
	)
	cm.notifyCaptionActivity()
	cm.notifyAudioActivity()
	cm.notifyCallEnded()
	assert.True(t, capCalled)
	assert.True(t, audCalled)
	assert.True(t, endCalled)

	// Verify Join returns ErrLobbyTimeout when LobbyTimeout expires in lobby,
	// while keeping the caller's session-lifetime ctx uncancelled.
	sessionCtx, cancelSession := context.WithCancel(context.Background())
	defer cancelSession()
	lobbyMeeting := newChromeMeeting(MeetingOptions{
		MeetingURL:   "https://teams.microsoft.com/meet/123",
		LobbyTimeout: 25 * time.Millisecond,
	})
	lobbyMeeting.pollInterval = 5 * time.Millisecond
	lobbyMeeting.probePreJoinState = func(context.Context) (bool, bool, bool, bool, error) {
		return false, true, false, false, nil // PreJoinInLobby
	}
	lobbyMeeting.probeAdmissionState = func(context.Context) (bool, bool, error) {
		return false, true, nil // AdmissionInLobby
	}
	err := lobbyMeeting.Join(sessionCtx)
	require.ErrorIs(t, err, ErrLobbyTimeout)
	assert.NoError(t, sessionCtx.Err(), "session-lifetime context must remain uncancelled after lobby timeout")

	// Verify ForceRecap bypasses live pre-join click flow even when prejoin/toolbar is present.
	recapMeeting := newChromeMeeting(MeetingOptions{
		MeetingURL:   "https://teams.microsoft.com/meet/123",
		ForceRecap:   true,
		LobbyTimeout: 20 * time.Millisecond,
	})
	recapMeeting.pollInterval = 5 * time.Millisecond
	recapMeeting.probePreJoinState = func(context.Context) (bool, bool, bool, bool, error) {
		return true, false, true, false, nil // would be PreJoinAdmitted/PreJoinReady in live mode
	}
	err = recapMeeting.Join(sessionCtx)
	require.ErrorIs(t, err, ErrLobbyTimeout, "ForceRecap must not admit to a live call toolbar")

	// Verify attachedCtx derived from sessionCtx remains alive for targetContext
	// after Join returns.
	attachedCtx, cancelAttached := context.WithCancel(sessionCtx)
	defer cancelAttached()
	admittedMeeting := newChromeMeeting(MeetingOptions{
		MeetingURL:   "https://teams.microsoft.com/meet/123",
		LobbyTimeout: 100 * time.Millisecond,
	})
	admittedMeeting.attachedCtx = attachedCtx
	admittedMeeting.pollInterval = 5 * time.Millisecond
	admittedMeeting.probePreJoinState = func(context.Context) (bool, bool, bool, bool, error) {
		return true, false, false, false, nil // PreJoinAdmitted
	}
	require.NoError(t, admittedMeeting.Join(sessionCtx))
	tCtx, cancelTarget := admittedMeeting.targetContext(sessionCtx)
	defer cancelTarget()
	assert.NoError(t, tCtx.Err(), "attachedCtx and targetContext must remain uncancelled after Join returns")
}
