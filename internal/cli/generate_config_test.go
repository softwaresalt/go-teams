package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/config"
)

// generateConfigDeps returns deps that fail the test if generate-config ever
// tries to load a configuration file.
func generateConfigDeps(t *testing.T) *deps {
	t.Helper()
	return &deps{
		loadConfig: func(path string) (config.Config, error) {
			t.Errorf("generate-config must not load configuration, got %q", path)
			return config.Config{}, nil
		},
	}
}

// runGenerate executes the root command with args and returns captured stdout.
func runGenerate(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCommand(generateConfigDeps(t))
	root.SetArgs(args)
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	err := root.Execute()
	return out.String(), err
}

func TestRootRegistersGenerateConfig(t *testing.T) {
	root := newRootCommand(defaultDeps())
	var found *cobra.Command
	for _, c := range root.Commands() {
		if c.Name() == "generate-config" {
			found = c
		}
	}
	require.NotNil(t, found, "generate-config must be registered on the root command")
	assert.ElementsMatch(t, []string{"init-config", "gen-config"}, found.Aliases)
	for flag, short := range map[string]string{"output-dir": "d", "name": "n", "force": ""} {
		f := found.Flags().Lookup(flag)
		require.NotNilf(t, f, "missing flag --%s", flag)
		assert.Equal(t, short, f.Shorthand)
	}
}

func TestGenerateConfigCommandFlagsAndPositionalArgs(t *testing.T) {
	base := t.TempDir()
	t.Setenv("GEN_CFG_BASE", base)

	// An existing directory whose name looks like a YAML file.
	yamlNamedDir := filepath.Join(base, "dirs", "team.yaml")
	require.NoError(t, os.MkdirAll(yamlNamedDir, 0o700))
	envYamlNamedDir := filepath.Join(base, "dirs", "env.yml")
	require.NoError(t, os.MkdirAll(envYamlNamedDir, 0o700))

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"default name with -d", []string{"generate-config", "-d", filepath.Join(base, "a")}, filepath.Join(base, "a", "config.yaml")},
		{"long output-dir flag", []string{"generate-config", "--output-dir", filepath.Join(base, "b")}, filepath.Join(base, "b", "config.yaml")},
		{"name flag extensionless", []string{"generate-config", "-d", filepath.Join(base, "c"), "--name", "my-team"}, filepath.Join(base, "c", "my-team.yaml")},
		{"short name flag", []string{"generate-config", "-d", filepath.Join(base, "c2"), "-n", "short.yml"}, filepath.Join(base, "c2", "short.yml")},
		{"uppercase YAML name", []string{"generate-config", "-d", filepath.Join(base, "d"), "-n", "TEAM.YAML"}, filepath.Join(base, "d", "TEAM.YAML")},
		{"uppercase YML name", []string{"generate-config", "-d", filepath.Join(base, "e"), "-n", "TEAM.YML"}, filepath.Join(base, "e", "TEAM.YML")},
		{"positional directory", []string{"generate-config", filepath.Join(base, "f")}, filepath.Join(base, "f", "config.yaml")},
		{"positional directory with name flag", []string{"generate-config", filepath.Join(base, "g"), "-n", "named"}, filepath.Join(base, "g", "named.yaml")},
		{"positional yaml file", []string{"generate-config", filepath.Join(base, "h", "custom.yaml")}, filepath.Join(base, "h", "custom.yaml")},
		{"positional yml file uppercase", []string{"generate-config", filepath.Join(base, "i", "CUSTOM.YML")}, filepath.Join(base, "i", "CUSTOM.YML")},
		{"positional yaml file via env var", []string{"generate-config", "$GEN_CFG_BASE/j/custom.yaml"}, filepath.Join(base, "j", "custom.yaml")},
		{"positional existing yaml-named dir", []string{"generate-config", yamlNamedDir}, filepath.Join(yamlNamedDir, "config.yaml")},
		{"positional existing yaml-named dir via env var", []string{"generate-config", "$GEN_CFG_BASE/dirs/env.yml"}, filepath.Join(envYamlNamedDir, "config.yaml")},
		{"explicit output-dir beats positional directory", []string{"generate-config", filepath.Join(base, "ignored"), "-d", filepath.Join(base, "k")}, filepath.Join(base, "k", "config.yaml")},
		{"explicit output-dir beats positional yaml file", []string{"generate-config", filepath.Join(base, "ignored2", "x.yaml"), "-d", filepath.Join(base, "l")}, filepath.Join(base, "l", "config.yaml")},
		{"alias init-config", []string{"init-config", "-d", filepath.Join(base, "m")}, filepath.Join(base, "m", "config.yaml")},
		{"alias gen-config", []string{"gen-config", "-d", filepath.Join(base, "n")}, filepath.Join(base, "n", "config.yaml")},
		{"root --config is ignored", []string{"--config", filepath.Join(base, "missing.yaml"), "generate-config", "-d", filepath.Join(base, "o")}, filepath.Join(base, "o", "config.yaml")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runGenerate(t, tc.args...)
			require.NoError(t, err)
			assert.Equal(t, tc.want+"\n", out)
			_, err = config.Load(tc.want)
			require.NoError(t, err, "generated file must load")
			data, err := os.ReadFile(tc.want)
			require.NoError(t, err)
			assert.Equal(t, config.TemplateYAML(), string(data))
		})
	}

	t.Run("root --config path is never created", func(t *testing.T) {
		_, statErr := os.Stat(filepath.Join(base, "missing.yaml"))
		assert.ErrorIs(t, statErr, os.ErrNotExist)
	})

	t.Run("defaults to config.yaml in the working directory", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		out, err := runGenerate(t, "generate-config")
		require.NoError(t, err)
		assert.Equal(t, "config.yaml\n", out)
		_, err = os.Stat(filepath.Join(dir, "config.yaml"))
		require.NoError(t, err)
	})

	t.Run("rejects more than one positional argument", func(t *testing.T) {
		_, err := runGenerate(t, "generate-config", "one", "two")
		require.Error(t, err)
	})
}

func TestGenerateConfigCommandCollisionAndForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte("original"), 0o600))

	out, err := runGenerate(t, "generate-config", "-d", dir)
	require.ErrorIs(t, err, config.ErrConfigExists)
	assert.Empty(t, out, "no path is printed when nothing was written")
	data, readErr := os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, "original", string(data))

	out, err = runGenerate(t, "generate-config", "-d", dir, "--force")
	require.NoError(t, err)
	assert.Equal(t, path+"\n", out)
	data, readErr = os.ReadFile(path)
	require.NoError(t, readErr)
	assert.Equal(t, config.TemplateYAML(), string(data))
}

func TestGenerateConfigCommandRejectsInvalidName(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "out")

	for _, name := range []string{"../escape.yaml", "   ", `sub\x.yaml`, "config.yaml:extra", ".yaml"} {
		t.Run(name, func(t *testing.T) {
			out, err := runGenerate(t, "generate-config", "-d", dir, "--name", name)
			require.ErrorIs(t, err, config.ErrInvalidConfigName)
			assert.Empty(t, out)
		})
	}

	_, statErr := os.Stat(dir)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "no directory is created for a rejected name")
	_, statErr = os.Stat(filepath.Join(parent, "escape.yaml"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestGenerateConfigPositionalEnvVarExpandedBeforeExtensionCheck(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "envfile", "team.yaml")
	t.Setenv("GEN_CFG_TARGET", target)

	// The raw argument "$GEN_CFG_TARGET" has no YAML extension; only the
	// expanded value does, so it must be treated as the full file path.
	out, err := runGenerate(t, "generate-config", "$GEN_CFG_TARGET")
	require.NoError(t, err)
	assert.Equal(t, target+"\n", out)
	data, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	assert.Equal(t, config.TemplateYAML(), string(data))
}

func TestGenerateConfigPositionalDoesNotDoubleExpand(t *testing.T) {
	base := t.TempDir()
	// A directory whose real name contains a literal "$" and looks like a
	// variable reference; it must not be expanded a second time.
	literalDir := filepath.Join(base, "$GEN_CFG_LITERAL")
	t.Setenv("GEN_CFG_LITERAL", "wrong")
	t.Setenv("GEN_CFG_PARENT", literalDir)

	out, err := runGenerate(t, "generate-config", "$GEN_CFG_PARENT/team.yaml")
	require.NoError(t, err)
	want := filepath.Join(literalDir, "team.yaml")
	assert.Equal(t, want+"\n", out)
	_, statErr := os.Stat(want)
	require.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(base, "wrong"))
	assert.ErrorIs(t, statErr, os.ErrNotExist, "expanded directory must not be expanded again")

	// Same guarantee for a positional directory argument.
	out, err = runGenerate(t, "generate-config", "$GEN_CFG_PARENT")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(literalDir, "config.yaml")+"\n", out)
}

func TestGenerateConfigExistsErrorIsActionable(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("original"), 0o600))

	_, err := runGenerate(t, "generate-config", "-d", dir)
	require.ErrorIs(t, err, config.ErrConfigExists)
	assert.Contains(t, err.Error(), "--force")
}

func TestGenerateConfigUsesInjectedWriter(t *testing.T) {
	var gotPath string
	var gotForce bool
	d := generateConfigDeps(t)
	d.writeTemplate = func(path string, force bool) error {
		gotPath, gotForce = path, force
		return nil
	}
	dir := t.TempDir()
	var out bytes.Buffer
	root := newRootCommand(d)
	root.SetArgs([]string{"generate-config", "-d", dir, "--force"})
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	require.NoError(t, root.Execute())
	assert.Equal(t, filepath.Join(dir, "config.yaml"), gotPath)
	assert.True(t, gotForce)
	_, statErr := os.Stat(gotPath)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "injected writer replaces the real write")
}

func TestGenerateConfigRejectsReservedDeviceNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"NUL", "con.yaml", "COM1.yml"} {
		out, err := runGenerate(t, "generate-config", "-d", dir, "--name", name)
		require.ErrorIsf(t, err, config.ErrInvalidConfigName, "name %q", name)
		assert.Empty(t, out)
	}
}
