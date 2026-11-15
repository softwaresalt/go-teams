package browser

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChromeRunnerJoinRejectsDisallowedURLBeforeBrowserAllocation(t *testing.T) {
	runner := NewChromeRunner(Options{ExecPath: "/nonexistent/chrome-binary"})

	err := runner.Join(context.Background(), "https://evil.example.com/meet/123?p=secret")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret", "validation error must not leak the passcode")
	assert.NotContains(t, err.Error(), "exec", "validation must fail before any browser allocation/launch is attempted")
	assert.NotContains(t, err.Error(), "chrome-binary", "validation must fail before any browser allocation/launch is attempted")
}

func TestNavigateErrorRedactsTargetSecrets(t *testing.T) {
	cause := errors.New("net::ERR_CONNECTION_REFUSED")

	err := navigateError("https://user:pass@127.0.0.1:1/meet/1?p=secret-passcode#access_token=xyz", cause)

	require.Error(t, err)
	assert.ErrorIs(t, err, cause)
	for _, s := range []string{"secret-passcode", "pass", "user", "xyz"} {
		assert.NotContains(t, err.Error(), s)
	}
	assert.Contains(t, err.Error(), "http")
}

func TestChromeRunnerJoinRejectsPlainHTTPRemoteURL(t *testing.T) {
	runner := NewChromeRunner(Options{ExecPath: "/nonexistent/chrome-binary"})

	err := runner.Join(context.Background(), "http://teams.microsoft.com/meet/123?p=secret")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
	assert.NotContains(t, err.Error(), "chrome-binary")
}

func TestChromeRunnerAuthenticateRequiresDefaultAccount(t *testing.T) {
	runner := NewChromeRunner(Options{
		ExecPath:       "/nonexistent/chrome-binary",
		DefaultAccount: "   ",
	})

	err := runner.Authenticate(context.Background())

	require.ErrorIs(t, err, ErrAccountRequired)
	assert.NotContains(t, err.Error(), "chrome-binary", "Authenticate must fail fast with ErrAccountRequired before launching browser")
}
