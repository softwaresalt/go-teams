package browser

import (
	"context"
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

func TestChromeRunnerJoinRejectsPlainHTTPRemoteURL(t *testing.T) {
	runner := NewChromeRunner(Options{ExecPath: "/nonexistent/chrome-binary"})

	err := runner.Join(context.Background(), "http://teams.microsoft.com/meet/123?p=secret")

	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
	assert.NotContains(t, err.Error(), "chrome-binary")
}
