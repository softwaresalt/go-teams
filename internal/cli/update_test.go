package cli

import (
	"bufio"
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
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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
	return runCapturingContext(context.Background(), d, args...)
}

func runCapturingContext(ctx context.Context, d *deps, args ...string) (string, error) {
	root := newRootCommand(d)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
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
		updater.ErrUpdateLocked,
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
	return e2eDepsWithHooks(srv, exe, current, nil)
}

func e2eDepsWithHooks(srv *httptest.Server, exe, current string, hooks *updater.TestHooks) *deps {
	return &deps{runUpdate: func(ctx context.Context, opts updateOptions) (updater.Result, error) {
		u := updater.New(updater.Config{
			HTTPClient: srv.Client(), APIBaseURL: srv.URL, CurrentVersion: current,
			GOOS: "linux", GOARCH: "amd64", ExecutablePath: exe, TestHooks: hooks,
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

// newVersionedExe builds a real executable so the updater can inspect it.
func newVersionedExe(t *testing.T, installed string) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "fixture.go")
	exe := filepath.Join(dir, "teams-cli")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	code := fmt.Sprintf("package main\nimport (\"fmt\"; \"os\")\nfunc main() { if len(os.Args) != 2 || os.Args[1] != \"--version\" { os.Exit(1) }; fmt.Println(%q) }\n", installed)
	require.NoError(t, os.WriteFile(src, []byte(code), 0o600))
	cmd := exec.Command("go", "build", "-o", exe, src)
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "build versioned fixture: %s", output)
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
	exe := newVersionedExe(t, "1.0.0")
	before := readFile(t, exe)

	out, err := runCapturing(e2eDeps(srv, exe, "1.0.0"), "update", "--check")
	require.NoError(t, err)
	assert.Contains(t, out, "Update available: 1.0.0 -> 1.2.0")
	assert.Equal(t, before, readFile(t, exe), "--check must not modify the binary")

	out, err = runCapturing(e2eDeps(srv, exe, "1.0.0"), "update")
	require.NoError(t, err)
	assert.Contains(t, out, "Updated teams-cli from 1.0.0 to 1.2.0")
	assert.Equal(t, "NEW", readFile(t, exe))
}

func TestUpdateEndToEndRejectsNewerInstalledBinary(t *testing.T) {
	srv := releaseServer(t, "v2.0.0", []byte("NEW"), "")
	exe := newVersionedExe(t, "3.0.0")
	before := readFile(t, exe)

	_, err := runCapturing(e2eDeps(srv, exe, "1.0.0"), "update")
	require.ErrorIs(t, err, updater.ErrDowngrade)
	assert.Equal(t, before, readFile(t, exe))
	assert.NoFileExists(t, exe+".lock")
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

// 018.002-T: exercise the rename gap through two actual CLI update invocations.
func TestUpdateEndToEndConcurrentUpdatesSerialize(t *testing.T) {
	srv := releaseServer(t, "v1.2.0", []byte("NEW"), "")
	exe := newExe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{}, 1)
	defer close(resume)
	type outcome struct {
		out string
		err error
	}
	run := func(d *deps, done chan<- outcome) {
		root := newRootCommand(d)
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"update", "--force"})
		err := root.ExecuteContext(ctx)
		done <- outcome{out.String(), err}
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)
	hooks := &updater.TestHooks{AfterMoveAside: func(ctx context.Context) {
		close(paused)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	}}
	go run(e2eDepsWithHooks(srv, exe, "1.0.0", hooks), first)
	select {
	case <-paused:
	case result := <-first:
		t.Fatalf("first updater did not reach rename gap: %v", result.err)
	case <-ctx.Done():
		t.Fatal("first updater did not reach rename gap")
	}
	assert.FileExists(t, exe+".lock")
	assert.FileExists(t, exe+".old")
	assert.NoFileExists(t, exe)
	probed := make(chan error, 1)
	secondHooks := &updater.TestHooks{AfterInitialStat: func(_ string, err error) {
		probed <- err
	}}
	go run(e2eDepsWithHooks(srv, exe, "1.0.0", secondHooks), second)
	select {
	case err := <-probed:
		assert.ErrorIs(t, err, os.ErrNotExist)
	case result := <-second:
		t.Fatalf("second updater did not probe the rename gap: %v", result.err)
	case <-ctx.Done():
		t.Fatal("second updater did not probe the rename gap")
	}
	select {
	case result := <-second:
		t.Errorf("second update returned during rename gap: %v", result.err)
		return
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal("timed out waiting for contender")
	}
	resume <- struct{}{}
	for _, done := range []<-chan outcome{first, second} {
		var result outcome
		select {
		case result = <-done:
		case <-ctx.Done():
			t.Fatal("updates did not finish after the holder resumed")
		}
		assert.NoError(t, result.err)
		assert.Contains(t, result.out, "Updated teams-cli")
	}
	assert.Equal(t, "NEW", readFile(t, exe))
	entries, err := os.ReadDir(filepath.Dir(exe))
	require.NoError(t, err)
	assert.Len(t, entries, 1, "lock, control lock, backup, and temporary files must be cleaned up")
}

// 018.002-T: a non-forced CLI contender must reject an in-flight upgrade.
func TestUpdateEndToEndNonForceContenderRejectsConcurrentUpgrade(t *testing.T) {
	upgrade := releaseServer(t, "v3.0.0", []byte("VERSION-3"), "")
	downgrade := releaseServer(t, "v2.0.0", []byte("VERSION-2"), "")
	exe := newExe(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{}, 1)
	defer close(resume)
	first := make(chan error, 1)
	second := make(chan error, 1)
	hooks := &updater.TestHooks{AfterMoveAside: func(ctx context.Context) {
		close(paused)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	}}
	go func() {
		_, err := runCapturingContext(ctx, e2eDepsWithHooks(upgrade, exe, "1.0.0", hooks), "update", "--force")
		first <- err
	}()
	select {
	case <-paused:
	case err := <-first:
		t.Fatalf("first CLI update did not reach rename gap: %v", err)
	case <-ctx.Done():
		t.Fatal("first CLI update did not reach rename gap")
	}
	probed := make(chan error, 1)
	secondHooks := &updater.TestHooks{AfterInitialStat: func(_ string, err error) { probed <- err }}
	go func() {
		_, err := runCapturingContext(ctx, e2eDepsWithHooks(downgrade, exe, "1.0.0", secondHooks), "update")
		second <- err
	}()
	select {
	case err := <-probed:
		require.ErrorIs(t, err, os.ErrNotExist)
	case err := <-second:
		t.Fatalf("second CLI update did not probe the rename gap: %v", err)
	case <-ctx.Done():
		t.Fatal("second CLI update did not probe the rename gap")
	}
	resume <- struct{}{}
	select {
	case err := <-first:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("first CLI update did not finish")
	}
	select {
	case err := <-second:
		require.ErrorIs(t, err, updater.ErrConcurrentUpdateModified)
	case <-ctx.Done():
		t.Fatal("second CLI update did not finish")
	}
	assert.Equal(t, "VERSION-3", readFile(t, exe))
	entries, err := os.ReadDir(filepath.Dir(exe))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestUpdateEndToEndCannotRemoveStaleBackup(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows exclusive handles are required")
	}
	srv := releaseServer(t, "v1.2.0", []byte("NEW"), "")
	exe := newExe(t)
	_, err := runCapturing(e2eDeps(srv, exe, "1.0.0"), "update", "--force")
	require.NoError(t, err, "first updater must install before contention")
	require.NoError(t, os.WriteFile(exe+".old", []byte("MAPPED-OLD"), 0o600))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		`$f = [IO.File]::Open($env:TEAMS_TEST_BACKUP, [IO.FileMode]::Open, [IO.FileAccess]::Read, [IO.FileShare]::None); [Console]::WriteLine('LOCKED'); [Console]::ReadLine() | Out-Null; $f.Dispose()`)
	cmd.Env = append(os.Environ(), "TEAMS_TEST_BACKUP="+exe+".old")
	stdin, err := cmd.StdinPipe()
	require.NoError(t, err)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())
	waited := false
	t.Cleanup(func() {
		if !waited {
			cancel()
			if err := cmd.Wait(); err != nil && ctx.Err() == nil {
				t.Errorf("wait for handle holder: %v: %s", err, stderr.String())
			}
		}
	})
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "LOCKED\r\n", line)
	_, err = runCapturing(e2eDeps(srv, exe, "1.0.0"), "update", "--force")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove stale backup")
	assert.Equal(t, "NEW", readFile(t, exe))
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, exe+".lock.ctl")
	require.NoError(t, stdin.Close())
	err = cmd.Wait()
	waited = true
	require.NoError(t, err, stderr.String())
	assert.Equal(t, "MAPPED-OLD", readFile(t, exe+".old"))
}
