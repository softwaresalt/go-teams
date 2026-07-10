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
