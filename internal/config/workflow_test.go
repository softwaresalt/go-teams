package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type workflowNeeds []string

func (n *workflowNeeds) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		if single != "" {
			*n = workflowNeeds{single}
		}
		return nil
	case yaml.SequenceNode:
		var list []string
		if err := value.Decode(&list); err != nil {
			return err
		}
		*n = workflowNeeds(list)
		return nil
	default:
		return fmt.Errorf("unexpected YAML node kind %d for needs", value.Kind)
	}
}

type workflowFile struct {
	Name        string                 `yaml:"name"`
	Permissions map[string]string      `yaml:"permissions"`
	Jobs        map[string]workflowJob `yaml:"jobs"`
}

type workflowJob struct {
	Name            string            `yaml:"name"`
	Needs           workflowNeeds     `yaml:"needs"`
	If              string            `yaml:"if"`
	RunsOn          string            `yaml:"runs-on"`
	Permissions     map[string]string `yaml:"permissions"`
	ContinueOnError bool              `yaml:"continue-on-error"`
	Strategy        workflowStrategy  `yaml:"strategy"`
	Steps           []workflowStep    `yaml:"steps"`
}

type workflowStrategy struct {
	FailFast bool           `yaml:"fail-fast"`
	Matrix   workflowMatrix `yaml:"matrix"`
}

type workflowMatrix struct {
	GOOS    []string               `yaml:"goos"`
	GOArch  []string               `yaml:"goarch"`
	Include []workflowMatrixTarget `yaml:"include"`
}

type workflowMatrixTarget struct {
	GOOS   string `yaml:"goos"`
	GOArch string `yaml:"goarch"`
}

type workflowStep struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	If              string            `yaml:"if"`
	Uses            string            `yaml:"uses"`
	With            map[string]string `yaml:"with"`
	Env             map[string]string `yaml:"env"`
	Shell           string            `yaml:"shell"`
	Run             string            `yaml:"run"`
	ContinueOnError bool              `yaml:"continue-on-error"`
}

func loadWorkflowFile(t *testing.T, relPath string) ([]byte, workflowFile) {
	t.Helper()
	fullPath := filepath.Join(findRepoRoot(t), relPath)
	raw, err := os.ReadFile(fullPath)
	require.NoError(t, err)

	var wf workflowFile
	require.NoError(t, yaml.Unmarshal(raw, &wf))
	return raw, wf
}

func findStepIndexByName(steps []workflowStep, name string) int {
	for i, s := range steps {
		if s.Name == name {
			return i
		}
	}
	return -1
}

func findStepIndexByUsesPrefix(steps []workflowStep, prefix string) int {
	for i, s := range steps {
		if strings.HasPrefix(s.Uses, prefix) {
			return i
		}
	}
	return -1
}

func assertWindowsQualityGateSteps(t *testing.T, steps []workflowStep) {
	t.Helper()

	setupGoIdx := findStepIndexByUsesPrefix(steps, "actions/setup-go@")
	require.NotEqual(t, -1, setupGoIdx, "test-windows must include actions/setup-go")
	assert.Equal(t, "actions/setup-go@0aaccfd150d50ccaeb58ebd88d36e91967a5f35b", steps[setupGoIdx].Uses)
	assert.Equal(t, "go.mod", steps[setupGoIdx].With["go-version-file"])

	lintIdx := findStepIndexByUsesPrefix(steps, "golangci/golangci-lint-action@")
	require.NotEqual(t, -1, lintIdx, "test-windows must include golangci/golangci-lint-action")
	assert.Less(t, setupGoIdx, lintIdx, "actions/setup-go must run strictly before golangci-lint-action")
	assert.Equal(t, "golangci/golangci-lint-action@2226d7cb06a077cd73e56eedd38eecad18e5d837", steps[lintIdx].Uses)
	assert.Equal(t, "v1.64.8", steps[lintIdx].With["version"])
	assert.Equal(t, "goinstall", steps[lintIdx].With["install-mode"])
	assert.Equal(t, "--timeout=5m", steps[lintIdx].With["args"])

	var hasBuildWin, hasVet, hasVetIntegration, hasVetDarwin, hasBrowserE2E, hasRaceTest bool
	assertUnconditional := func(step workflowStep) {
		assert.Empty(t, step.If, "quality-gate step %q must not set if:", step.Name)
		assert.False(t, step.ContinueOnError, "quality-gate step %q must not set continue-on-error", step.Name)
	}
	assertUnconditional(steps[setupGoIdx])
	assertUnconditional(steps[lintIdx])

	for _, step := range steps {
		if strings.Contains(step.Run, "go build -o bin/go-teams.exe ./cmd/go-teams") {
			assertUnconditional(step)
			hasBuildWin = true
		}
		if step.Name == "Vet" && strings.Contains(step.Run, "go vet ./...") {
			assertUnconditional(step)
			hasVet = true
		}
		if strings.Contains(step.Run, "go vet -tags=integration ./...") {
			assertUnconditional(step)
			hasVetIntegration = true
		}
		if step.Env["GOOS"] == "darwin" && step.Env["GOARCH"] == "amd64" && strings.Contains(step.Run, "go vet ./...") {
			assertUnconditional(step)
			hasVetDarwin = true
		}
		if strings.Contains(step.Run, "TestChromeMeetingEndToEnd") &&
			strings.Contains(step.Run, "--- PASS: TestChromeMeetingEndToEnd") &&
			strings.Contains(step.Run, "--- SKIP:") &&
			strings.Contains(step.Run, "$LASTEXITCODE") &&
			strings.Contains(step.Run, "exit 1") {
			assertUnconditional(step)
			hasBrowserE2E = true
		}
		if strings.Contains(step.Run, `go test -race "-coverprofile=coverage.out" ./...`) {
			assertUnconditional(step)
			hasRaceTest = true
		}
	}

	assert.True(t, hasBuildWin, "test-windows must build bin/go-teams.exe")
	assert.True(t, hasVet, "test-windows must run go vet ./...")
	assert.True(t, hasVetIntegration, "test-windows must run go vet -tags=integration ./...")
	assert.True(t, hasVetDarwin, "test-windows must vet GOOS=darwin GOARCH=amd64 ./...")
	assert.True(t, hasBrowserE2E, "test-windows must run non-skipped TestChromeMeetingEndToEnd verification")
	assert.True(t, hasRaceTest, "test-windows must run go test -race -coverprofile=coverage.out ./...")
}

func TestCIWorkflowRunsWindowsQualityGatesOnly(t *testing.T) {
	raw, wf := loadWorkflowFile(t, filepath.Join(".github", "workflows", "ci.yml"))

	assert.NotContains(t, wf.Jobs, "test", "ci.yml must not define a duplicate Linux test job")
	assert.NotContains(t, wf.Jobs, "docs-lint", "ci.yml must not define docs-lint")
	assert.NotRegexp(t, regexp.MustCompile(`(?m)^\s*#?\s*docs-lint:`), string(raw))

	changesJob, ok := wf.Jobs["changes"]
	require.True(t, ok, "ci.yml must define changes job")
	assert.Equal(t, "ubuntu-latest", changesJob.RunsOn)
	assert.Empty(t, changesJob.If)
	assert.False(t, changesJob.ContinueOnError)

	secretIdx := findStepIndexByName(changesJob.Steps, "Secret hygiene check (tracked files)")
	require.NotEqual(t, -1, secretIdx, "changes job must include Secret hygiene check (tracked files)")
	secretStep := changesJob.Steps[secretIdx]
	assert.Empty(t, secretStep.If)
	assert.False(t, secretStep.ContinueOnError)
	assert.Contains(t, secretStep.Run, "git ls-files")

	winJob, ok := wf.Jobs["test-windows"]
	require.True(t, ok, "ci.yml must define test-windows job")
	assert.Equal(t, "Build, vet, lint & test (Windows)", winJob.Name)
	assert.Equal(t, "windows-latest", winJob.RunsOn)
	assert.Equal(t, workflowNeeds{"changes"}, winJob.Needs)
	assert.Equal(t, "needs.changes.outputs.code == 'true'", winJob.If)

	assertWindowsQualityGateSteps(t, winJob.Steps)

	var hasCoverageReport bool
	for _, step := range winJob.Steps {
		if strings.Contains(step.Run, `go tool cover "-func=coverage.out"`) {
			assert.Empty(t, step.If)
			assert.False(t, step.ContinueOnError)
			hasCoverageReport = true
		}
	}
	assert.True(t, hasCoverageReport, `test-windows in ci.yml must report coverage via go tool cover "-func=coverage.out"`)
}

func TestReleaseWorkflowBuildsThreeSupportedTargetsOnly(t *testing.T) {
	raw, wf := loadWorkflowFile(t, filepath.Join(".github", "workflows", "release.yml"))

	assert.NotContains(t, wf.Jobs, "test", "release.yml must not define a duplicate Linux test job")
	assert.NotContains(t, wf.Jobs, "npm-package", "release.yml must not define npm-package")
	assert.NotContains(t, wf.Jobs, "npm-publish", "release.yml must not define npm-publish")
	assert.NotRegexp(t, regexp.MustCompile(`(?m)^\s*#?\s*npm-(?:package|publish):`), string(raw))

	winJob, ok := wf.Jobs["test-windows"]
	require.True(t, ok, "release.yml must define test-windows job")
	assert.Equal(t, "windows-latest", winJob.RunsOn)
	assertWindowsQualityGateSteps(t, winJob.Steps)

	_, ok = wf.Jobs["changelog"]
	require.True(t, ok, "release.yml must define changelog job")

	buildJob, ok := wf.Jobs["build"]
	require.True(t, ok, "release.yml must define build job")
	assert.Equal(t, workflowNeeds{"test-windows"}, buildJob.Needs)
	assert.Empty(t, buildJob.Strategy.Matrix.GOOS)
	assert.Empty(t, buildJob.Strategy.Matrix.GOArch)
	expectedTargets := []workflowMatrixTarget{
		{GOOS: "windows", GOArch: "amd64"},
		{GOOS: "windows", GOArch: "arm64"},
		{GOOS: "darwin", GOArch: "amd64"},
	}
	assert.Equal(t, expectedTargets, buildJob.Strategy.Matrix.Include)
	for _, target := range buildJob.Strategy.Matrix.Include {
		assert.NotEqual(t, "linux", target.GOOS)
		assert.False(t, target.GOOS == "darwin" && target.GOArch == "arm64")
	}

	releaseJob, ok := wf.Jobs["release"]
	require.True(t, ok, "release.yml must define release job")
	assert.Equal(t, workflowNeeds{"changelog", "build"}, releaseJob.Needs)

	shaIdx := findStepIndexByName(releaseJob.Steps, "Generate SHA256SUMS")
	publishIdx := findStepIndexByName(releaseJob.Steps, "Publish release")
	require.NotEqual(t, -1, shaIdx, "release job must define Generate SHA256SUMS")
	require.NotEqual(t, -1, publishIdx, "release job must define Publish release")
	assert.Less(t, shaIdx, publishIdx, "Generate SHA256SUMS must precede Publish release")

	shaStep := releaseJob.Steps[shaIdx]
	publishStep := releaseJob.Steps[publishIdx]
	assert.Empty(t, shaStep.If)
	assert.False(t, shaStep.ContinueOnError)
	assert.Empty(t, publishStep.If)
	assert.False(t, publishStep.ContinueOnError)

	assert.Contains(t, shaStep.Run, `test "$(wc -l < SHA256SUMS)" -eq 3`)
	assert.Contains(t, shaStep.Run, "exit 1")
	assert.NotContains(t, shaStep.Run, "|| true")
	assert.NotContains(t, shaStep.Run, "-eq 6")
	assert.NotContains(t, shaStep.Run, "go-teams-linux-amd64")
	assert.NotContains(t, shaStep.Run, "go-teams-linux-arm64")
	assert.NotContains(t, shaStep.Run, "go-teams-darwin-arm64")

	for _, target := range buildJob.Strategy.Matrix.Include {
		asset := fmt.Sprintf("go-teams-%s-%s", target.GOOS, target.GOArch)
		if target.GOOS == "windows" {
			asset += ".exe"
		}
		assert.Contains(t, shaStep.Run, asset)
	}
}

func TestWorkflowsPinThirdPartyActionsAndDisableCredentials(t *testing.T) {
	root := findRepoRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "*.yml"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(matches), 3, "expected at least ci.yml, release.yml, and cli-reference-drift.yml")

	shaPinnedActionRe := regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+(/[A-Za-z0-9_.-]+)*@[0-9a-f]{40}$`)

	for _, wfPath := range matches {
		rel, err := filepath.Rel(root, wfPath)
		require.NoError(t, err)
		_, wf := loadWorkflowFile(t, rel)
		base := filepath.Base(wfPath)

		assert.Equal(t, map[string]string{"contents": "read"}, wf.Permissions,
			"%s must declare top-level permissions: contents: read", base)

		for jobID, job := range wf.Jobs {
			for permKey, permVal := range job.Permissions {
				if permVal != "read" {
					assert.Truef(t, base == "release.yml" && jobID == "release" && permKey == "contents" && permVal == "write",
						"%s job %s grants elevated permission %s: %s", base, jobID, permKey, permVal)
				}
			}
			for _, step := range job.Steps {
				if step.Uses == "" {
					continue
				}
				if !strings.HasPrefix(step.Uses, "./") {
					assert.Regexpf(t, shaPinnedActionRe, step.Uses,
						"%s job %s step %q uses non-SHA-pinned action %q", base, jobID, step.Name, step.Uses)
				}
				if strings.HasPrefix(step.Uses, "actions/checkout@") {
					assert.Equalf(t, "false", step.With["persist-credentials"],
						"%s job %s step %q must set persist-credentials: false", base, jobID, step.Name)
				}
			}
		}
	}
}
