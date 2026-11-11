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
		{"teams cloud microsoft", "https://teams.cloud.microsoft/l/meetingrecap?threadId=1", false},
		{"teams live", "https://teams.live.com/meet/123?p=synthetic", false},
		{"sharepoint tenant", "https://contoso.sharepoint.com/personal/x", false},
		{"microsoft subdomain", "https://gov.teams.microsoft.com/l/x", false},
		{"office subdomain", "https://www.office.com/x", false},
		{"uppercase host", "https://TEAMS.Microsoft.com/x", false},
		{"loopback http ipv4", "http://127.0.0.1:54321/meet/x", false},
		{"loopback http localhost", "http://localhost:8080/l/meetingrecap", false},
		{"loopback http ipv6", "http://[::1]:9000/meet/x", false},
		{"loopback https", "https://127.0.0.1:9443/meet/x", false},
		{"remote plain http teams", "http://teams.microsoft.com/x", true},
		{"remote plain http example", "http://example.com/x", true},
		{"remote https untrusted", "https://example.com/x", true},
		{"spoofed suffix host", "https://teams.microsoft.com.evil.example/x", true},
		{"non-dot suffix host", "https://evilmicrosoft.com/x", true},
		{"prefix lookalike", "https://evil-teams.microsoft.com.attacker.example/x", true},
		{"userinfo on trusted host", "https://user:pass@teams.microsoft.com/x", true},
		{"userinfo on loopback", "http://user@127.0.0.1:1/x", true},
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

func TestRedactURL(t *testing.T) {
	secrets := []string{"user", "pass", "tok", "frag", "secret", "oauth", "xyz"}
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"valid with userinfo query fragment", "https://user:pass@teams.microsoft.com/meet/123?p=tok#frag", "https://teams.microsoft.com/meet/123"},
		{"passcode query", "https://teams.microsoft.com/meet/123?p=secret-passcode", "https://teams.microsoft.com/meet/123"},
		{"oauth code and token fragment", "https://teams.microsoft.com/v2/?code=oauth#access_token=xyz", "https://teams.microsoft.com/v2/"},
		{"force query", "https://teams.microsoft.com/meet/123?", "https://teams.microsoft.com/meet/123"},
		{"loopback with port", "http://127.0.0.1:8080/meet/x?p=secret", "http://127.0.0.1:8080/meet/x"},
		{"no secrets", "https://teams.microsoft.com/meet/123", "https://teams.microsoft.com/meet/123"},
		{"malformed port with userinfo", "https://user:pass@teams.microsoft.com:badport/meet/123?p=tok", "[invalid-url]"},
		{"malformed escape in host", "https://user:pass@bad%zz/path?p=tok#frag", "[invalid-url]"},
		{"hostless query", "https://?p=secret", "[invalid-url]"},
		{"opaque url", "https:user:pass@teams.microsoft.com/x?p=tok", "[invalid-url]"},
		{"non-http scheme", "msteams://teams.microsoft.com/x?p=secret", "[invalid-url]"},
		{"empty", "", "[invalid-url]"},
		{"control character", "https://teams.microsoft.com/x\x00?p=secret", "[invalid-url]"},
		{"userinfo hiding query char", "https://user:p?ss@bad%zz/x#frag", "[invalid-url]"},
		{"relative path", "/meet/123?p=secret", "[invalid-url]"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactURL(tc.raw)
			assert.Equal(t, tc.want, got)
			for _, s := range secrets {
				assert.NotContains(t, got, s)
			}
			assert.NotContains(t, got, "?")
			assert.NotContains(t, got, "#")
		})
	}
}

func TestRedactURLMalformedStripsQueryAndFragmentBeforeSplittingPath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"slash in query with bad fragment escape", "https://teams.microsoft.com?redirect=/secret-passcode#%zz", "https://teams.microsoft.com"},
		{"slash in fragment with bad escape", "https://teams.microsoft.com/meet/1#/secret-passcode%zz", "https://teams.microsoft.com/meet/1"},
		{"slash in query after path", "https://teams.microsoft.com/meet/1?next=/secret-passcode#%zz", "https://teams.microsoft.com/meet/1"},
		{"hash before slash", "https://teams.microsoft.com#/secret-passcode%zz", "https://teams.microsoft.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactURL(tc.raw)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "secret")
		})
	}
}

func TestRedactArgsUsesEarliestSchemeToken(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			"outer loopback url with nested https url",
			[]string{"-u", "http://localhost/meet/1?p=secret-passcode&next=https://teams.microsoft.com/"},
			"-u http://localhost/meet/1",
		},
		{
			"malformed single-slash delimiter",
			[]string{"-u", "https:/teams.microsoft.com/meet/1?p=secret-passcode"},
			"-u [invalid-url]",
		},
		{
			"flag attached malformed delimiter",
			[]string{"--url=HTTPS:teams.microsoft.com/meet/1?p=secret-passcode"},
			"--url=[invalid-url]",
		},
		{
			"upper-case scheme",
			[]string{"HTTPS://teams.microsoft.com/meet/1?p=secret-passcode"},
			"https://teams.microsoft.com/meet/1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RedactArgs(tc.args)
			assert.Equal(t, tc.want, got)
			assert.NotContains(t, got, "secret")
		})
	}
}

func TestRedactURLMalformedNeverLeaksUserinfo(t *testing.T) {
	inputs := []string{
		"https://user:pass@teams.microsoft.com:badport/meet/123?p=tok",
		"https://user:pass@[::1/x?p=tok#frag",
		"http://user:pass@host with space/x?p=tok",
	}
	for _, raw := range inputs {
		got := RedactURL(raw)
		for _, s := range []string{"user", "pass", "tok", "frag"} {
			assert.NotContains(t, got, s, "input %q produced %q", raw, got)
		}
	}
}

func TestRedactArgs(t *testing.T) {
	got := RedactArgs([]string{
		"join", "--url=https://user:pass@teams.microsoft.com/meet/1?p=tok#frag",
		"-uhttps://teams.microsoft.com/meet/2?p=secret2",
		"https://teams.microsoft.com/meet/3?p=secret3", "-o", `C:\out\x.md`,
	})
	assert.Equal(t,
		`join --url=https://teams.microsoft.com/meet/1 -uhttps://teams.microsoft.com/meet/2 https://teams.microsoft.com/meet/3 -o C:\out\x.md`,
		got)
	for _, s := range []string{"user", "pass", "tok", "frag", "secret"} {
		assert.NotContains(t, got, s)
	}
}

func TestValidateMeetingURLErrorsNeverEchoSecrets(t *testing.T) {
	inputs := []string{
		"https://example.com/x?p=secret-passcode#access_token=xyz",
		"http://teams.microsoft.com/meet/1?p=secret-passcode",
		"https://user:pass@teams.microsoft.com/x?code=oauth",
		"https://user:pass@bad%zz/path?p=secret-passcode",
		"msteams://teams.microsoft.com/x?p=secret-passcode",
	}
	for _, raw := range inputs {
		err := ValidateMeetingURL(raw)
		require.Error(t, err, raw)
		for _, s := range []string{"secret-passcode", "xyz", "pass", "oauth", "user"} {
			assert.NotContains(t, err.Error(), s, "input %q error %q", raw, err.Error())
		}
	}
}
