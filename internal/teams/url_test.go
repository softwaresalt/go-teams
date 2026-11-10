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

func TestMutateJoinURLMeetPathPreservesPasscodeAndStripsLegacyType(t *testing.T) {
	tests := []struct {
		name         string
		raw          string
		wantPath     string
		wantPasscode string
		wantFoo      string
		wantFragment string
	}{
		{
			name:         "meet url with passcode",
			raw:          "https://teams.microsoft.com/meet/example-id?p=synthetic-token",
			wantPath:     "/meet/example-id",
			wantPasscode: "synthetic-token",
		},
		{
			name:     "meet url without passcode",
			raw:      "https://teams.microsoft.com/meet/example-id",
			wantPath: "/meet/example-id",
		},
		{
			name:         "meet url strips conflicting legacy type param",
			raw:          "https://teams.microsoft.com/meet/example-id?p=synthetic-token&type=meetup-join",
			wantPath:     "/meet/example-id",
			wantPasscode: "synthetic-token",
		},
		{
			name:         "meet url preserves unrelated query keys and fragment",
			raw:          "https://teams.microsoft.com/meet/example-id?p=synthetic-token&foo=bar#frag",
			wantPath:     "/meet/example-id",
			wantPasscode: "synthetic-token",
			wantFoo:      "bar",
			wantFragment: "frag",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := MutateJoinURL(tc.raw)
			require.NoError(t, err)

			u, err := url.Parse(got)
			require.NoError(t, err)
			assert.Equal(t, tc.wantPath, u.Path)
			assert.Equal(t, tc.wantPasscode, u.Query().Get("p"))
			assert.False(t, u.Query().Has("type"), "legacy type param must be absent for /meet/ URLs")
			assert.Equal(t, tc.wantFoo, u.Query().Get("foo"))
			assert.Equal(t, tc.wantFragment, u.Fragment)
		})
	}
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

func TestValidateMeetingURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid https", "https://teams.microsoft.com/l/meetup-join/x", false},
		{"valid http", "http://example.com/x", false},
		{"empty", "", true},
		{"non-http scheme", "ftp://example.com/x", true},
		{"app scheme", "msteams://teams.microsoft.com/x", true},
		{"no host", "/l/meetup-join/x", true},
		{"scheme only, no host", "https://", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateMeetingURL(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
