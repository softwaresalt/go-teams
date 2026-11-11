package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/updater"
	"github.com/softwaresalt/go-teams/internal/version"
)

// updateRecorder is a fake runUpdate seam that records its invocation.
type updateRecorder struct {
	called bool
	opts   updateOptions
	res    updater.Result
	err    error
}

func (r *updateRecorder) run(_ context.Context, opts updateOptions) (updater.Result, error) {
	r.called = true
	r.opts = opts
	return r.res, r.err
}

// updateDeps returns deps with only runUpdate wired: the update command must
// not need config loading or a browser.
func updateDeps(r *updateRecorder) *deps { return &deps{runUpdate: r.run} }

// runCapturing executes args on a fresh root and returns combined output.
func runCapturing(d *deps, args ...string) (string, error) {
	root := newRootCommand(d)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return out.String(), err
}

func TestUpdateCommandAppliesUpdate(t *testing.T) {
	rec := &updateRecorder{res: updater.Result{
		CurrentVersion: "1.0.0", LatestVersion: "1.2.0", TagName: "v1.2.0",
		UpdateAvailable: true, Updated: true,
	}}
	out, err := runCapturing(updateDeps(rec), "update")
	require.NoError(t, err)
	assert.True(t, rec.called)
	assert.Equal(t, updateOptions{}, rec.opts)
	assert.Contains(t, out, "Updated teams-cli from 1.0.0 to 1.2.0")
}

func TestUpdateCommandAlreadyUpToDate(t *testing.T) {
	rec := &updateRecorder{res: updater.Result{CurrentVersion: "1.2.0", LatestVersion: "1.2.0"}}
	out, err := runCapturing(updateDeps(rec), "update")
	require.NoError(t, err)
	assert.Contains(t, out, "already up to date (1.2.0)")
	assert.Contains(t, out, "--force")
}

func TestUpdateCommandCheckHintKeepsRequestedVersion(t *testing.T) {
	rec := &updateRecorder{res: updater.Result{
		CurrentVersion: "1.0.0", LatestVersion: "2.0.0-rc.1", TagName: "v2.0.0-rc.1", UpdateAvailable: true,
	}}
	out, err := runCapturing(updateDeps(rec), "update", "--check", "--version", "v2.0.0-rc.1")
	require.NoError(t, err)
	assert.Equal(t, updateOptions{CheckOnly: true, TargetVersion: "v2.0.0-rc.1"}, rec.opts)
	assert.Contains(t, out, "`teams-cli update --version v2.0.0-rc.1`",
		"the install hint must target the inspected release, not the latest stable one")

	rec = &updateRecorder{res: updater.Result{CurrentVersion: "1.0.0", LatestVersion: "1.2.0", TagName: "v1.2.0", UpdateAvailable: true}}
	out, err = runCapturing(updateDeps(rec), "update", "--check")
	require.NoError(t, err)
	assert.Contains(t, out, "`teams-cli update`")
	assert.NotContains(t, out, "--version")
}

func TestUpdateCommandCheckReportsAvailability(t *testing.T) {
	tests := []struct {
		name string
		res  updater.Result
		want string
	}{
		{"available", updater.Result{CurrentVersion: "1.0.0", LatestVersion: "1.2.0", UpdateAvailable: true},
			"Update available: 1.0.0 -> 1.2.0"},
		{"current", updater.Result{CurrentVersion: "1.2.0", LatestVersion: "1.2.0"},
			"up to date (1.2.0)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &updateRecorder{res: tt.res}
			out, err := runCapturing(updateDeps(rec), "update", "--check")
			require.NoError(t, err)
			assert.True(t, rec.opts.CheckOnly)
			assert.False(t, rec.opts.Force)
			assert.Contains(t, out, tt.want)
			assert.NotContains(t, out, "Updated teams-cli", "--check must never claim an install")
		})
	}
}

func TestUpdateCommandForwardsVersionAndForce(t *testing.T) {
	rec := &updateRecorder{res: updater.Result{
		CurrentVersion: "1.3.0", LatestVersion: "1.2.0", Updated: true, Downgrade: true,
	}}
	out, err := runCapturing(updateDeps(rec), "update", "--version", "v1.2.0", "--force")
	require.NoError(t, err)
	assert.Equal(t, updateOptions{TargetVersion: "v1.2.0", Force: true}, rec.opts)
	assert.Contains(t, out, "Updated teams-cli from 1.3.0 to 1.2.0")
}

func TestUpdateCommandPropagatesErrors(t *testing.T) {
	for _, wantErr := range []error{
		errors.New("network down"),
		updater.ErrChecksumMismatch,
		updater.ErrDowngrade,
	} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			rec := &updateRecorder{err: wantErr}
			_, err := runCapturing(updateDeps(rec), "update")
			require.ErrorIs(t, err, wantErr)
		})
	}
}

func TestUpdateCommandRejectsPositionalArgs(t *testing.T) {
	rec := &updateRecorder{}
	_, err := runCapturing(updateDeps(rec), "update", "extra")
	require.Error(t, err)
	assert.False(t, rec.called)
}

func TestUpdateCommandWithoutRunnerFailsCleanly(t *testing.T) {
	_, err := runCapturing(&deps{}, "update")
	require.Error(t, err)
}

func TestDefaultDepsWiresUpdater(t *testing.T) {
	assert.NotNil(t, defaultDeps().runUpdate)
}

func TestRootVersionFlagReportsBuildVersion(t *testing.T) {
	out, err := runCapturing(&deps{}, "--version")
	require.NoError(t, err)
	assert.Contains(t, out, version.Version)
}

// releaseServer serves one fake release whose platform binary is newBinary and
// whose SHA256SUMS lists sums (computed from newBinary when empty).
func releaseServer(t *testing.T, tag string, newBinary []byte, sums string) *httptest.Server {
	t.Helper()
	asset := updater.AssetName("linux", "amd64")
	if sums == "" {
		d := sha256.Sum256(newBinary)
		sums = fmt.Sprintf("%s  ./%s\n", hex.EncodeToString(d[:]), asset)
	}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/" + updater.DefaultRepo + "/releases/latest",
			"/repos/" + updater.DefaultRepo + "/releases/tags/" + tag:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag,
				"assets": []map[string]string{
					{"name": asset, "browser_download_url": srv.URL + "/dl/bin"},
					{"name": updater.ChecksumAsset, "browser_download_url": srv.URL + "/dl/sums"},
				},
			})
		case "/dl/bin":
			_, _ = w.Write(newBinary)
		case "/dl/sums":
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// e2eDeps wires runUpdate to a real updater pointed at srv and exe.
func e2eDeps(srv *httptest.Server, exe, current string) *deps {
	return &deps{runUpdate: func(ctx context.Context, opts updateOptions) (updater.Result, error) {
		u := updater.New(updater.Config{
			HTTPClient: srv.Client(), APIBaseURL: srv.URL, CurrentVersion: current,
			GOOS: "linux", GOARCH: "amd64", ExecutablePath: exe,
		})
		if opts.CheckOnly {
			return u.Check(ctx, opts.TargetVersion)
		}
		return u.Update(ctx, opts.TargetVersion, opts.Force)
	}}
}

func newExe(t *testing.T) string {
	t.Helper()
	exe := filepath.Join(t.TempDir(), "teams-cli")
	require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
	return exe
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestUpdateEndToEndReplacesBinary(t *testing.T) {
	srv := releaseServer(t, "v1.2.0", []byte("NEW"), "")
	exe := newExe(t)

	out, err := runCapturing(e2eDeps(srv, exe, "1.0.0"), "update", "--check")
	require.NoError(t, err)
	assert.Contains(t, out, "Update available: 1.0.0 -> 1.2.0")
	assert.Equal(t, "OLD", readFile(t, exe), "--check must not modify the binary")

	out, err = runCapturing(e2eDeps(srv, exe, "1.0.0"), "update")
	require.NoError(t, err)
	assert.Contains(t, out, "Updated teams-cli from 1.0.0 to 1.2.0")
	assert.Equal(t, "NEW", readFile(t, exe))
}

func TestUpdateEndToEndRejectsChecksumMismatch(t *testing.T) {
	bad := fmt.Sprintf("%s  ./%s\n", hex.EncodeToString(make([]byte, 32)), updater.AssetName("linux", "amd64"))
	srv := releaseServer(t, "v1.2.0", []byte("NEW"), bad)
	exe := newExe(t)

	_, err := runCapturing(e2eDeps(srv, exe, "1.0.0"), "update")
	require.ErrorIs(t, err, updater.ErrChecksumMismatch)
	assert.Equal(t, "OLD", readFile(t, exe))
	entries, readErr := os.ReadDir(filepath.Dir(exe))
	require.NoError(t, readErr)
	assert.Len(t, entries, 1, "no temp or .old files may remain")
}

func TestUpdateEndToEndDowngradeRequiresForce(t *testing.T) {
	srv := releaseServer(t, "v1.1.0", []byte("OLDER"), "")
	exe := newExe(t)

	_, err := runCapturing(e2eDeps(srv, exe, "1.2.0"), "update", "--version", "v1.1.0")
	require.ErrorIs(t, err, updater.ErrDowngrade)
	assert.Equal(t, "OLD", readFile(t, exe))

	out, err := runCapturing(e2eDeps(srv, exe, "1.2.0"), "update", "--version", "v1.1.0", "--force")
	require.NoError(t, err)
	assert.Contains(t, out, "Updated teams-cli from 1.2.0 to 1.1.0")
	assert.Equal(t, "OLDER", readFile(t, exe))
}

func TestUpdateEndToEndForceReinstallsCurrent(t *testing.T) {
	srv := releaseServer(t, "v1.2.0", []byte("NEW"), "")
	exe := newExe(t)

	out, err := runCapturing(e2eDeps(srv, exe, "1.2.0"), "update")
	require.NoError(t, err)
	assert.Contains(t, out, "already up to date")
	assert.Equal(t, "OLD", readFile(t, exe))

	_, err = runCapturing(e2eDeps(srv, exe, "1.2.0"), "update", "--force")
	require.NoError(t, err)
	assert.Equal(t, "NEW", readFile(t, exe))
}
