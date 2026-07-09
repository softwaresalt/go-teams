package teams

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMutateJoinURLAddsAllBypassParams(t *testing.T) {
	got, err := MutateJoinURL("https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc")
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "false", q.Get("msLaunch"))
	assert.Equal(t, "meetup-join", q.Get("type"))
	assert.Equal(t, "true", q.Get("directDl"))
	assert.Equal(t, "true", q.Get("enableMobilePage"))
	assert.Equal(t, "true", q.Get("suppressPrompt"))
	assert.Equal(t, "/l/meetup-join/19:meeting_abc", u.Path)
}

func TestMutateJoinURLPreservesExistingParams(t *testing.T) {
	got, err := MutateJoinURL("https://teams.microsoft.com/l/meetup-join/x?tenantId=abc&foo=bar")
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "abc", q.Get("tenantId"))
	assert.Equal(t, "bar", q.Get("foo"))
	assert.Equal(t, "false", q.Get("msLaunch"))
}

func TestMutateJoinURLOverridesConflictingBypassParam(t *testing.T) {
	got, err := MutateJoinURL("https://teams.microsoft.com/l/meetup-join/x?suppressPrompt=false")
	require.NoError(t, err)

	u, err := url.Parse(got)
	require.NoError(t, err)
	assert.Equal(t, "true", u.Query().Get("suppressPrompt"))
}

func TestMutateJoinURLErrors(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"non-http scheme", "ftp://teams.microsoft.com/x"},
		{"no host", "/l/meetup-join/x"},
		{"app scheme", "msteams://teams.microsoft.com/x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := MutateJoinURL(tc.raw)
			assert.Error(t, err)
		})
	}
}
