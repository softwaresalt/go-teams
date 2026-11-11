package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testOS   = "linux"
	testArch = "amd64"
)

var testAsset = AssetName(testOS, testArch)

func sumHex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// release describes one fake GitHub release served by the fixture.
type release struct {
	tag      string
	binary   []byte
	sums     string // empty => computed from binary
	noSums   bool
	noBinary bool
}

type fixture struct {
	t        *testing.T
	srv      *httptest.Server
	releases map[string]release // keyed by tag
	latest   string
	mu       sync.Mutex
	paths    []string
	exePath  string
	updater  *Updater
}

func newFixture(t *testing.T, current string, releases ...release) *fixture {
	t.Helper()
	f := &fixture{t: t, releases: map[string]release{}}
	for _, r := range releases {
		f.releases[r.tag] = r
	}
	f.latest = releases[0].tag
	f.srv = httptest.NewServer(http.HandlerFunc(f.handle))
	t.Cleanup(f.srv.Close)

	f.exePath = filepath.Join(t.TempDir(), "teams-cli")
	require.NoError(t, os.WriteFile(f.exePath, []byte("OLD-BINARY"), 0o755))
	f.updater = New(Config{
		HTTPClient:     f.srv.Client(),
		APIBaseURL:     f.srv.URL,
		CurrentVersion: current,
		GOOS:           testOS,
		GOARCH:         testArch,
		ExecutablePath: f.exePath,
	})
	return f
}

func (f *fixture) recorded() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.paths...)
}

func (f *fixture) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.paths = append(f.paths, r.URL.Path)
	f.mu.Unlock()
	const prefix = "/repos/" + DefaultRepo + "/releases/"
	switch {
	case r.URL.Path == prefix+"latest":
		f.writeRelease(w, f.releases[f.latest])
	case strings.HasPrefix(r.URL.Path, prefix+"tags/"):
		rel, ok := f.releases[strings.TrimPrefix(r.URL.Path, prefix+"tags/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		f.writeRelease(w, rel)
	case strings.HasPrefix(r.URL.Path, "/dl/"):
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/dl/"), "/")
		rel := f.releases[parts[0]]
		if parts[1] == ChecksumAsset {
			sums := rel.sums
			if sums == "" {
				sums = fmt.Sprintf("%s  ./%s\n%s  ./other-file\n", sumHex(rel.binary), testAsset, sumHex([]byte("x")))
			}
			_, _ = w.Write([]byte(sums))
			return
		}
		_, _ = w.Write(rel.binary)
	default:
		http.NotFound(w, r)
	}
}

func (f *fixture) writeRelease(w http.ResponseWriter, rel release) {
	type asset struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	}
	var assets []asset
	if !rel.noBinary {
		assets = append(assets, asset{testAsset, f.srv.URL + "/dl/" + rel.tag + "/" + testAsset})
	}
	if !rel.noSums {
		assets = append(assets, asset{ChecksumAsset, f.srv.URL + "/dl/" + rel.tag + "/" + ChecksumAsset})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": rel.tag, "assets": assets})
}

func (f *fixture) exeContent() string {
	f.t.Helper()
	b, err := os.ReadFile(f.exePath)
	require.NoError(f.t, err)
	return string(b)
}

func (f *fixture) dirEntries() []string {
	f.t.Helper()
	entries, err := os.ReadDir(filepath.Dir(f.exePath))
	require.NoError(f.t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

var newBin = []byte("NEW-BINARY-V1.2.0")

func TestAssetName(t *testing.T) {
	assert.Equal(t, "teams-cli-windows-amd64.exe", AssetName("windows", "amd64"))
	assert.Equal(t, "teams-cli-windows-arm64.exe", AssetName("windows", "arm64"))
	assert.Equal(t, "teams-cli-linux-amd64", AssetName("linux", "amd64"))
}

func TestParseChecksums(t *testing.T) {
	data := []byte("" +
		strings.Repeat("A", 64) + "  ./teams-cli-windows-amd64.exe\n" +
		strings.Repeat("b", 64) + " *teams-cli-windows-arm64.exe\r\n" +
		strings.Repeat("c", 64) + "  plain-name\n" +
		"short  bad-digest\n" +
		"not-hex-" + strings.Repeat("z", 56) + "  bad-hex\n" +
		"\n" +
		"justonefield\n")
	got := parseChecksums(data)
	assert.Equal(t, strings.Repeat("a", 64), got["teams-cli-windows-amd64.exe"])
	assert.Equal(t, strings.Repeat("b", 64), got["teams-cli-windows-arm64.exe"])
	assert.Equal(t, strings.Repeat("c", 64), got["plain-name"])
	assert.NotContains(t, got, "bad-digest")
	assert.NotContains(t, got, "bad-hex")
	assert.Len(t, got, 3)
}

func TestCheckReportsUpdateAvailable(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	res, err := f.updater.Check(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, res.UpdateAvailable)
	assert.False(t, res.Updated)
	assert.False(t, res.Downgrade)
	assert.Equal(t, "1.0.0", res.CurrentVersion)
	assert.Equal(t, "1.2.0", res.LatestVersion)
	assert.Equal(t, "v1.2.0", res.TagName)
	assert.Equal(t, testAsset, res.AssetName)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
}

func TestCheckUpToDate(t *testing.T) {
	f := newFixture(t, "v1.2.0", release{tag: "v1.2.0", binary: newBin})
	res, err := f.updater.Check(context.Background(), "")
	require.NoError(t, err)
	assert.False(t, res.UpdateAvailable)
	assert.False(t, res.Downgrade)
}

func TestCheckDevBuildIsUpgradable(t *testing.T) {
	f := newFixture(t, "dev", release{tag: "v0.0.1", binary: newBin})
	res, err := f.updater.Check(context.Background(), "")
	require.NoError(t, err)
	assert.True(t, res.UpdateAvailable)
	assert.False(t, res.Downgrade)
}

func TestCheckTargetVersionUsesTagsEndpoint(t *testing.T) {
	for _, target := range []string{"1.1.0", "v1.1.0"} {
		t.Run(target, func(t *testing.T) {
			f := newFixture(t, "1.0.0",
				release{tag: "v1.2.0", binary: newBin},
				release{tag: "v1.1.0", binary: []byte("v110")})
			res, err := f.updater.Check(context.Background(), target)
			require.NoError(t, err)
			assert.Equal(t, "1.1.0", res.LatestVersion)
			assert.Equal(t, []string{"/repos/" + DefaultRepo + "/releases/tags/v1.1.0"}, f.recorded())
		})
	}
}

func TestCheckTargetVersionPreservesBuildMetadata(t *testing.T) {
	f := newFixture(t, "1.0.0",
		release{tag: "v1.2.3", binary: []byte("plain")},
		release{tag: "v1.2.3+build.2", binary: newBin})
	res, err := f.updater.Check(context.Background(), "v1.2.3+build.2")
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3+build.2", res.TagName, "the exact requested tag must be looked up")
	assert.Equal(t, "1.2.3", res.LatestVersion)
	assert.Equal(t, []string{"/repos/" + DefaultRepo + "/releases/tags/v1.2.3+build.2"}, f.recorded())
}

func TestCheckFailsWhenUpdateIsNotInstallable(t *testing.T) {
	tests := []struct {
		name string
		rel  release
	}{
		{"no platform asset", release{tag: "v1.2.0", binary: newBin, noBinary: true}},
		{"no SHA256SUMS asset", release{tag: "v1.2.0", binary: newBin, noSums: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "1.0.0", tt.rel)
			res, err := f.updater.Check(context.Background(), "")
			require.ErrorIs(t, err, ErrAssetNotFound)
			assert.False(t, res.Updated)
		})
		t.Run(tt.name+" but already current", func(t *testing.T) {
			f := newFixture(t, "1.2.0", tt.rel)
			res, err := f.updater.Check(context.Background(), "")
			require.NoError(t, err, "a current install needs no assets")
			assert.False(t, res.UpdateAvailable)
		})
	}
}

func TestDownloadErrorsRedactSignedURLQuery(t *testing.T) {
	const secret = "SIGNED-TOKEN-VALUE"
	// Port 1 on loopback refuses connections, so the download fails after the
	// release metadata (which carries signed-style URLs) has been fetched.
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name": "v1.2.0",
			"assets": []map[string]string{
				{"name": testAsset, "browser_download_url": "http://127.0.0.1:1/bin?token=" + secret},
				{"name": ChecksumAsset, "browser_download_url": "http://127.0.0.1:1/sums?token=" + secret},
			},
		})
	}))
	t.Cleanup(api.Close)
	exe := filepath.Join(t.TempDir(), "teams-cli")
	require.NoError(t, os.WriteFile(exe, []byte("OLD-BINARY"), 0o755))
	u := New(Config{HTTPClient: api.Client(), APIBaseURL: api.URL, CurrentVersion: "1.0.0",
		GOOS: testOS, GOARCH: testArch, ExecutablePath: exe})

	_, err := u.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret, "signed-URL query parameters must not reach CLI output")
	var ue *url.Error
	require.ErrorAs(t, err, &ue, "the underlying *url.Error must stay inspectable")
	assert.Equal(t, "http://127.0.0.1:1/sums", ue.URL)
}

func TestCheckRejectsMalformedTargetVersion(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	_, err := f.updater.Check(context.Background(), "../../etc")
	require.Error(t, err)
	assert.Empty(t, f.recorded(), "no request may be sent for a malformed target version")
}

func TestCheckHTTPErrorStatus(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	_, err := f.updater.Check(context.Background(), "9.9.9")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestCheckMetadataSizeCap(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	f.updater.maxMetadataBytes = 16
	_, err := f.updater.Check(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestUpdateReplacesExecutable(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	res, err := f.updater.Update(context.Background(), "", false)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.True(t, res.UpdateAvailable)
	assert.Equal(t, "1.2.0", res.LatestVersion)
	assert.Equal(t, string(newBin), f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries(), "no temp or .old files may remain")
	if runtime.GOOS != "windows" {
		info, err := os.Stat(f.exePath)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode().Perm()&0o100, "replacement must stay executable")
	}
}

func TestUpdateAlreadyUpToDateIsNoOp(t *testing.T) {
	f := newFixture(t, "1.2.0", release{tag: "v1.2.0", binary: newBin})
	res, err := f.updater.Update(context.Background(), "", false)
	require.NoError(t, err)
	assert.False(t, res.Updated)
	assert.False(t, res.UpdateAvailable)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	for _, p := range f.recorded() {
		assert.NotContains(t, p, "/dl/", "no download when already current")
	}
}

func TestUpdateForceReinstallsSameVersion(t *testing.T) {
	f := newFixture(t, "1.2.0", release{tag: "v1.2.0", binary: newBin})
	res, err := f.updater.Update(context.Background(), "", true)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, string(newBin), f.exeContent())
}

func TestUpdateRejectsDowngradeWithoutForce(t *testing.T) {
	f := newFixture(t, "1.2.0", release{tag: "v1.1.0", binary: []byte("v110")})
	res, err := f.updater.Update(context.Background(), "1.1.0", false)
	require.ErrorIs(t, err, ErrDowngrade)
	assert.True(t, res.Downgrade)
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
}

func TestUpdateAllowsDowngradeWithForce(t *testing.T) {
	f := newFixture(t, "1.2.0", release{tag: "v1.1.0", binary: []byte("v110")})
	res, err := f.updater.Update(context.Background(), "1.1.0", true)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, "v110", f.exeContent())
}

func TestUpdateRejectsChecksumMismatch(t *testing.T) {
	bad := fmt.Sprintf("%s  ./%s\n", sumHex([]byte("different")), testAsset)
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin, sums: bad})
	res, err := f.updater.Update(context.Background(), "", false)
	require.ErrorIs(t, err, ErrChecksumMismatch)
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries(), "temp file must be removed on mismatch")
}

func TestUpdateFailsClosedWithoutVerifiableChecksum(t *testing.T) {
	tests := []struct {
		name    string
		rel     release
		wantErr error
	}{
		{"no SHA256SUMS asset", release{tag: "v1.2.0", binary: newBin, noSums: true}, ErrAssetNotFound},
		{"no platform asset", release{tag: "v1.2.0", binary: newBin, noBinary: true}, ErrAssetNotFound},
		{"no entry for platform", release{tag: "v1.2.0", binary: newBin,
			sums: sumHex(newBin) + "  ./some-other-binary\n"}, ErrChecksumMissing},
		{"empty SHA256SUMS", release{tag: "v1.2.0", binary: newBin, sums: "\n"}, ErrChecksumMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "1.0.0", tt.rel)
			res, err := f.updater.Update(context.Background(), "", false)
			require.ErrorIs(t, err, tt.wantErr)
			assert.False(t, res.Updated)
			assert.Equal(t, "OLD-BINARY", f.exeContent())
			assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
		})
	}
}

func TestUpdateBinarySizeCap(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	f.updater.maxBinaryBytes = 4
	_, err := f.updater.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
}

func TestUpdateChecksumSizeCap(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	f.updater.maxChecksumBytes = 8
	_, err := f.updater.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
}

func TestRejectsInsecureNonLoopbackURLs(t *testing.T) {
	t.Run("api base url", func(t *testing.T) {
		u := New(Config{APIBaseURL: "http://example.com", CurrentVersion: "1.0.0", GOOS: testOS, GOARCH: testArch})
		_, err := u.Check(context.Background(), "")
		require.ErrorIs(t, err, ErrInsecureURL)
	})
	t.Run("asset download url", func(t *testing.T) {
		f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
		// Serve a release whose asset URLs are plain-http, non-loopback.
		insecure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": "v1.2.0",
				"assets": []map[string]string{
					{"name": testAsset, "browser_download_url": "http://example.com/" + testAsset},
					{"name": ChecksumAsset, "browser_download_url": "http://example.com/" + ChecksumAsset},
				},
			})
		}))
		t.Cleanup(insecure.Close)
		f.updater.cfg.APIBaseURL = insecure.URL
		f.updater.cfg.HTTPClient = insecure.Client()
		res, err := f.updater.Update(context.Background(), "", false)
		require.ErrorIs(t, err, ErrInsecureURL)
		assert.False(t, res.Updated)
		assert.Equal(t, "OLD-BINARY", f.exeContent())
		assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
	})
	t.Run("redirect to insecure host", func(t *testing.T) {
		redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "http://example.com/evil", http.StatusFound)
		}))
		t.Cleanup(redirector.Close)
		u := New(Config{APIBaseURL: redirector.URL, HTTPClient: redirector.Client(), CurrentVersion: "1.0.0", GOOS: testOS, GOARCH: testArch})
		_, err := u.Check(context.Background(), "")
		require.ErrorIs(t, err, ErrInsecureURL)
	})
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://api.github.com/x", true},
		{"https://example.com", true},
		{"http://127.0.0.1:8080/x", true},
		{"http://localhost:8080/x", true},
		{"http://[::1]:8080/x", true},
		{"http://example.com/x", false},
		{"http://127.0.0.1.evil.com/x", false},
		{"ftp://127.0.0.1/x", false},
		{"file:///etc/passwd", false},
		{"//example.com/x", false},
		{"not a url", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			err := validateURL(tt.url)
			if tt.ok {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, ErrInsecureURL)
		})
	}
}

func TestUpdateRollsBackWhenFinalRenameFails(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	calls := 0
	f.updater.rename = func(oldPath, newPath string) error {
		calls++
		if calls == 2 { // tmp -> exe
			return errors.New("simulated rename failure")
		}
		return os.Rename(oldPath, newPath)
	}
	res, err := f.updater.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "simulated rename failure")
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent(), "previous binary must be restored from .old")
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries(), "temp and .old must be cleaned up")
	assert.Equal(t, 3, calls, "exe->old, tmp->exe (fails), old->exe (rollback)")
}

func TestUpdateReportsFailedRollback(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	calls := 0
	f.updater.rename = func(oldPath, newPath string) error {
		calls++
		if calls >= 2 {
			return errors.New("simulated failure")
		}
		return os.Rename(oldPath, newPath)
	}
	_, err := f.updater.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), f.exePath+".old", "error must tell the operator where the backup lives")
}

func TestUpdateReplacesStaleOldFile(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	require.NoError(t, os.WriteFile(f.exePath+".old", []byte("STALE"), 0o644))
	_, err := f.updater.Update(context.Background(), "", false)
	require.NoError(t, err)
	assert.Equal(t, string(newBin), f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
}

func TestUpdateHonorsContextCancellation(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.updater.Update(ctx, "", false)
	require.Error(t, err)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
}
