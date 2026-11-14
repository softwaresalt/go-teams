package browser

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
