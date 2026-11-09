package capture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExplicitOutputRejectsTranscriptAudioPathCollision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.mp3")
	require.NoError(t, os.WriteFile(path, []byte("existing recording"), 0o600))

	out, err := setupOutput(OutputConfig{FilePath: path, MeetingURL: "https://example.invalid/meet/synthetic"})
	if out != nil {
		require.NoError(t, out.Close())
	}
	require.Error(t, err, "transcript and audio must not open the same path")
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "existing recording", string(content))
}
