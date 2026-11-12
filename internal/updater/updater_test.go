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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/go-teams/internal/version"
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
	// Fixture binaries are arbitrary bytes rather than executable programs.
	f.updater.installedVersion = func(context.Context, string) (string, error) {
		return current, nil
	}
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

func TestUpdateNonForceSelfUpdateSucceedsWhenRunningImageMatchesTarget(t *testing.T) {
	for _, tt := range []struct {
		name     string
		distinct bool
	}{
		{name: "matching running image"},
		{name: "different running image", distinct: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
			f.updater.cfg.ExecutablePath = ""
			other := f.exePath + ".other"
			require.NoError(t, os.WriteFile(other, []byte("OLD-BINARY"), 0o755))
			calls := 0
			f.updater.osExecutable = func() (string, error) {
				calls++
				if tt.distinct && calls > 2 {
					return other, nil
				}
				return f.exePath, nil
			}
			probed := false
			f.updater.installedVersion = func(context.Context, string) (string, error) {
				probed = true
				return "1.0.0", nil
			}

			res, err := f.updater.Update(context.Background(), "", false)
			if tt.distinct {
				require.ErrorIs(t, err, ErrConcurrentUpdateModified)
				assert.False(t, res.Updated)
				assert.Equal(t, "OLD-BINARY", f.exeContent())
			} else {
				require.NoError(t, err)
				assert.True(t, res.Updated)
				assert.True(t, probed)
				assert.Equal(t, string(newBin), f.exeContent())
			}
			assert.Equal(t, "OLD-BINARY", string(mustReadFile(t, other)))
			assert.NoFileExists(t, f.exePath+".lock")
		})
	}
}

func TestUpdateNonForceSyntheticSelfUpdateWithoutVersionProbe(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	f.updater.cfg.ExecutablePath = ""
	f.updater.osExecutable = func() (string, error) { return f.exePath, nil }
	f.updater.installedVersion = nil

	res, err := f.updater.Update(context.Background(), "", false)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, string(newBin), f.exeContent())
}

func TestUpdateRejectsSameVersionReplacementAfterUpdaterConstruction(t *testing.T) {
	for _, tt := range []struct {
		name    string
		inPlace bool
		content string
	}{
		{name: "different file same bytes", content: "OLD-BINARY"},
		{name: "different file different bytes", content: "SAME-VERSION-B"},
		{name: "same file different bytes", inPlace: true, content: "SAME-VERSION-B"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
			cfg := f.updater.cfg
			cfg.ExecutablePath = ""
			u := newUpdater(cfg, func() (string, error) { return f.exePath, nil })
			u.installedVersion = func(context.Context, string) (string, error) { return "1.0.0", nil }

			if !tt.inPlace {
				require.NoError(t, os.Rename(f.exePath, f.exePath+".previous"))
			}
			require.NoError(t, os.WriteFile(f.exePath, []byte(tt.content), 0o755))
			res, err := u.Update(context.Background(), "", false)
			require.ErrorIs(t, err, ErrConcurrentUpdateModified)
			assert.False(t, res.Updated)
			assert.Equal(t, tt.content, f.exeContent())
			assert.NoFileExists(t, f.exePath+".lock")
		})
	}
}

func TestUpdateSyntheticConstructionSnapshotFailureFailsClosed(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
	cfg := f.updater.cfg
	cfg.ExecutablePath = ""
	calls := 0
	u := newUpdater(cfg, func() (string, error) {
		calls++
		if calls == 1 {
			return "", os.ErrPermission
		}
		return f.exePath, nil
	})
	u.installedVersion = func(context.Context, string) (string, error) { return "1.0.0", nil }

	res, err := u.Update(context.Background(), "", false)
	require.ErrorIs(t, err, os.ErrPermission)
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestUpdateSyntheticConstructionSnapshotMatches(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
	cfg := f.updater.cfg
	cfg.ExecutablePath = ""
	u := newUpdater(cfg, func() (string, error) { return f.exePath, nil })
	u.installedVersion = func(context.Context, string) (string, error) { return "1.0.0", nil }

	res, err := u.Update(context.Background(), "", false)
	require.NoError(t, err)
	assert.True(t, res.Updated)
	assert.Equal(t, string(newBin), f.exeContent())
}

func TestConfiguredExecutableDoesNotBecomeLaunchSnapshot(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
	assert.Nil(t, f.updater.launchSnapshot.info)
	assert.False(t, f.updater.launchSnapshotInjected)
}

func TestNewDefaultUsesProcessInitializationSnapshot(t *testing.T) {
	require.NoError(t, processSnapshot.err)
	u := New(Config{})
	require.NoError(t, u.launchSnapshotErr)
	require.NotNil(t, u.launchSnapshot.info)
	assert.True(t, os.SameFile(processSnapshot.snapshot.info, u.launchSnapshot.info))
	assert.Equal(t, processSnapshot.snapshot.digest, u.launchSnapshot.digest)
	assert.False(t, u.launchSnapshotInjected)
}

func TestUpdateNonForceSelfUpdateRejectsInPlaceRunningImageChange(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	f.updater.cfg.ExecutablePath = ""
	f.updater.installedVersion = nil
	calls := 0
	f.updater.osExecutable = func() (string, error) {
		calls++
		if calls == 3 {
			require.NoError(t, os.WriteFile(f.exePath, []byte("CHANGED"), 0o755))
		}
		return f.exePath, nil
	}

	res, err := f.updater.Update(context.Background(), "", false)
	require.ErrorIs(t, err, ErrConcurrentUpdateModified)
	assert.False(t, res.Updated)
	assert.Equal(t, "CHANGED", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
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

// 018.001-T: a successful acquisition owns a metadata-bearing lock and can
// release it even after the caller has canceled its context.
func TestUpdateReleaseSucceedsWhenCallerContextCanceled(t *testing.T) {
	u := New(Config{})
	exe := filepath.Join(t.TempDir(), "teams-cli")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release, err := u.acquireSwapLock(ctx, exe)
	require.NoError(t, err)
	require.NotNil(t, release)

	var metadata struct {
		PID       int       `json:"pid"`
		Token     string    `json:"token"`
		CreatedAt time.Time `json:"created_at"`
	}
	data, err := os.ReadFile(exe + ".lock")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &metadata))
	assert.Equal(t, os.Getpid(), metadata.PID)
	assert.Len(t, metadata.Token, 32)
	assert.False(t, metadata.CreatedAt.IsZero())
	cancel()
	require.NoError(t, release())
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, exe+".lock.ctl")
}

func TestUpdatePausedOrStaleHolderNeverEvictedAndClassifiedInError(t *testing.T) {
	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	for _, tt := range []struct {
		name  string
		alive bool
		want  string
	}{
		{"dead", false, "dead"},
		{"live", true, "live"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "teams-cli")
			lock := exe + ".lock"
			content := fmt.Sprintf(`{"pid":1234,"token":"holder","created_at":%q}`, now.Add(-3*time.Minute).Format(time.RFC3339Nano))
			require.NoError(t, os.WriteFile(lock, []byte(content), 0o600))
			u := New(Config{})
			u.lockWaitTimeout = 30 * time.Millisecond
			u.lockPollInterval = time.Millisecond
			u.lockStaleAfter = 2 * time.Minute
			u.now = func() time.Time { return now }
			u.isProcessAlive = func(int) bool { return tt.alive }

			_, err := u.acquireSwapLock(context.Background(), exe)
			require.ErrorIs(t, err, ErrUpdateLocked)
			assert.Contains(t, strings.ToLower(err.Error()), tt.want)
			assert.Contains(t, err.Error(), "1234")
			assert.Contains(t, strings.ToLower(err.Error()), "manual")
			if tt.alive {
				assert.Contains(t, strings.ToLower(err.Error()), "reus")
			}
			got, readErr := os.ReadFile(lock)
			require.NoError(t, readErr)
			assert.Equal(t, content, string(got))
		})
	}
}

func TestSwapLockReleaseInterleavedWithReplacementDoesNotDeleteNewOwnerLock(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	u := New(Config{})
	release, err := u.acquireSwapLock(context.Background(), exe)
	require.NoError(t, err)
	require.NotNil(t, release)
	replacement := []byte(`{"pid":1234,"token":"replacement","created_at":"2026-01-01T00:00:00Z"}`)
	u.beforeReleaseControlLock = func() {
		require.NoError(t, os.WriteFile(exe+".lock", replacement, 0o600))
	}
	require.Error(t, release())
	got, err := os.ReadFile(exe + ".lock")
	require.NoError(t, err)
	assert.Equal(t, replacement, got)
}

func TestUpdateFailsWhenLockHeld(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	lock := f.exePath + ".lock"
	content := []byte(`{"pid":1234,"token":"held","created_at":"2026-01-01T00:00:00Z"}`)
	require.NoError(t, os.WriteFile(lock, content, 0o600))
	f.updater.lockWaitTimeout = 30 * time.Millisecond
	f.updater.lockPollInterval = time.Millisecond

	res, err := f.updater.Update(context.Background(), "", true)
	require.ErrorIs(t, err, ErrUpdateLocked)
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	got, err := os.ReadFile(lock)
	require.NoError(t, err)
	assert.Equal(t, content, got)
	assert.Equal(t, []string{"teams-cli", "teams-cli.lock"}, f.dirEntries())
}

func TestLockPathsFailClosedWhenExclusiveCreateIsDenied(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.lock")
	assert.Equal(t, runtime.GOOS == "windows", lockCreateContended(missing, os.ErrPermission))
	assert.False(t, lockCreateContended(missing, os.ErrInvalid))
	for _, control := range []bool{false, true} {
		t.Run(fmt.Sprintf("control=%t", control), func(t *testing.T) {
			exe := filepath.Join(t.TempDir(), "teams-cli")
			path := exe + ".lock"
			if control {
				path += ".ctl"
			}
			require.NoError(t, os.Mkdir(path, 0o700))
			u := New(Config{})
			u.lockWaitTimeout = 30 * time.Millisecond
			u.ctlLockWaitTimeout = 30 * time.Millisecond
			u.lockPollInterval = time.Millisecond
			u.ctlLockPollInterval = time.Millisecond
			var err error
			if control {
				_, err = u.acquireReleaseControlLock(context.Background(), exe)
			} else {
				_, err = u.acquireSwapLock(context.Background(), exe)
			}
			require.ErrorIs(t, err, ErrUpdateLocked)
			assert.DirExists(t, path, "a contender must never remove an existing lock path")
		})
	}
}

func TestUpdateRejectsLateSnapshotDowngradeAfterConcurrentUpgradeWithoutForce(t *testing.T) {
	f := newFixture(t, "1.0.0",
		release{tag: "v3.0.0", binary: []byte("VERSION-3")},
		release{tag: "v2.0.0", binary: []byte("VERSION-2")})
	first, err := f.updater.Update(context.Background(), "v3.0.0", true)
	require.NoError(t, err, "A must install v3 before B starts")
	require.True(t, first.Updated)
	require.Equal(t, "VERSION-3", f.exeContent())
	b := New(f.updater.cfg)
	checked := false
	b.installedVersion = func(_ context.Context, exe string) (string, error) {
		checked = true
		assert.Equal(t, f.exePath, exe)
		assert.FileExists(t, exe+".lock", "installed version must be checked under lock")
		assert.Equal(t, "VERSION-3", f.exeContent())
		return "3.0.0", nil
	}
	res, err := b.Update(context.Background(), "v2.0.0", false)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrDowngrade) || errors.Is(err, ErrConcurrentUpdateModified))
	assert.True(t, checked)
	assert.False(t, res.Updated)
	assert.Equal(t, "VERSION-3", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestUpdateFailsClosedWhenInstalledVersionCannotBeInspected(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	// The fixture is not executable, so the real version probe must fail.
	f.updater.installedVersion = nil
	res, err := f.updater.Update(context.Background(), "", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "inspect installed version")
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestUpdateConfiguredPathChecksInstalledVersionAgainstCurrent(t *testing.T) {
	tests := []struct {
		name      string
		current   string
		installed string
		wantErr   error
	}{
		{name: "matching release", current: "1.0.0", installed: "v1.0.0"},
		{name: "different release", current: "1.0.0", installed: "1.1.0", wantErr: ErrConcurrentUpdateModified},
		{name: "installed dev", current: "1.0.0", installed: "dev", wantErr: ErrConcurrentUpdateModified},
		{name: "development current", current: "dev", installed: "1.1.0"},
		{name: "installed newer than target", current: "1.0.0", installed: "3.0.0", wantErr: ErrDowngrade},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t, tt.current, release{tag: "v2.0.0", binary: newBin})
			f.updater.installedVersion = func(_ context.Context, exe string) (string, error) {
				assert.FileExists(t, exe+".lock")
				return tt.installed, nil
			}
			res, err := f.updater.Update(context.Background(), "", false)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.False(t, res.Updated)
				assert.Equal(t, "OLD-BINARY", f.exeContent())
			} else {
				require.NoError(t, err)
				assert.True(t, res.Updated)
				assert.Equal(t, string(newBin), f.exeContent())
			}
			assert.NoFileExists(t, f.exePath+".lock")
		})
	}
}

func TestUpdateWithRealInstalledVersionProbe(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.go")
	program := `package main
import ("fmt"; "os")
func main() {
	if len(os.Args) != 2 || os.Args[1] != "--version" { os.Exit(1) }
	v := os.Getenv("UPDATER_TEST_VERSION")
	if v == "" { v = "dev" }
	fmt.Println("teams-cli version", v)
}`
	require.NoError(t, os.WriteFile(source, []byte(program), 0o600))
	built := filepath.Join(dir, "fixture.exe")
	output, err := exec.Command("go", "build", "-o", built, source).CombinedOutput()
	require.NoError(t, err, "%s", output)
	binary, err := os.ReadFile(built)
	require.NoError(t, err)

	tests := []struct {
		name       string
		running    string
		reported   string
		selfUpdate bool
		wantErr    error
	}{
		{name: "configured dev", running: "dev"},
		{name: "self update dev", running: "dev", selfUpdate: true},
		{name: "release running installed dev", running: "1.0.0", selfUpdate: true, wantErr: ErrConcurrentUpdateModified},
		{name: "malformed installed version", running: "dev", reported: "dev-invalid", wantErr: version.ErrInvalidVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("UPDATER_TEST_VERSION", tt.reported)
			f := newFixture(t, tt.running, release{tag: "v2.0.0", binary: newBin})
			f.exePath = filepath.Join(filepath.Dir(f.exePath), "fixture.exe")
			require.NoError(t, os.WriteFile(f.exePath, binary, 0o755))
			f.updater.cfg.ExecutablePath = f.exePath
			f.updater.installedVersion = nil
			if tt.selfUpdate {
				f.updater.cfg.ExecutablePath = ""
				f.updater.osExecutable = func() (string, error) { return f.exePath, nil }
				// Exercise the real probe despite the injected running-image path.
				f.updater.installedVersion = readInstalledVersion
			}

			res, err := f.updater.Update(context.Background(), "", false)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.False(t, res.Updated)
				assert.Equal(t, binary, mustReadFile(t, f.exePath))
			} else {
				require.NoError(t, err)
				assert.True(t, res.Updated)
				assert.Equal(t, string(newBin), f.exeContent())
			}
			assert.NoFileExists(t, f.exePath+".lock")
		})
	}
}

func TestUpdateRejectsExecutableChangedDuringVersionProbe(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
	f.updater.installedVersion = func(_ context.Context, exe string) (string, error) {
		assert.FileExists(t, exe+".lock")
		require.NoError(t, os.Rename(exe, exe+".previous"))
		require.NoError(t, os.WriteFile(exe, []byte("OTHER"), 0o755))
		return "1.0.0", nil
	}

	res, err := f.updater.Update(context.Background(), "", false)
	require.ErrorIs(t, err, ErrConcurrentUpdateModified)
	assert.False(t, res.Updated)
	assert.Equal(t, "OTHER", f.exeContent())
	assert.Equal(t, "OLD-BINARY", string(mustReadFile(t, f.exePath+".previous")))
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestUpdateRejectsInPlaceChangeDuringVersionProbe(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: newBin})
	f.updater.cfg.ExecutablePath = ""
	f.updater.osExecutable = func() (string, error) { return f.exePath, nil }
	f.updater.installedVersion = func(_ context.Context, exe string) (string, error) {
		require.NoError(t, os.WriteFile(exe, []byte("CHANGED"), 0o755))
		return "1.0.0", nil
	}

	res, err := f.updater.Update(context.Background(), "", false)
	require.ErrorIs(t, err, ErrConcurrentUpdateModified)
	assert.False(t, res.Updated)
	assert.Equal(t, "CHANGED", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestSwapRunningImageVersionGuardWithoutProc(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux verifies the running image through /proc/self/exe")
	}
	tests := []struct {
		name       string
		running    string
		installed  string
		probeErr   error
		wantErr    error
		wantUpdate bool
	}{
		{name: "matching release", running: "1.0.0", installed: "1.0.0", wantUpdate: true},
		{name: "newer installed release", running: "1.0.0", installed: "3.0.0", wantErr: ErrDowngrade},
		{name: "different installed release", running: "1.0.0", installed: "1.1.0", wantErr: ErrConcurrentUpdateModified},
		{name: "unknown running release", running: "dev", installed: "1.0.0", wantUpdate: true},
		{name: "probe failure", running: "1.0.0", probeErr: os.ErrPermission, wantErr: os.ErrPermission},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			exe := filepath.Join(dir, "teams-cli")
			tmp := filepath.Join(dir, "download")
			require.NoError(t, os.WriteFile(exe, []byte("OLD"), 0o755))
			require.NoError(t, os.WriteFile(tmp, []byte("NEW"), 0o755))
			baseline, err := snapshotExecutable(exe)
			require.NoError(t, err)
			u := New(Config{CurrentVersion: tt.running})
			u.osExecutable = func() (string, error) { return exe, nil }
			probed := false
			u.installedVersion = func(_ context.Context, path string) (string, error) {
				probed = true
				assert.Equal(t, exe, path)
				assert.FileExists(t, exe+".lock")
				return tt.installed, tt.probeErr
			}
			updated, err := u.swap(context.Background(), exe, tmp, baseline, "2.0.0", false)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, "OLD", string(mustReadFile(t, exe)))
			} else {
				require.NoError(t, err)
				assert.Equal(t, "NEW", string(mustReadFile(t, exe)))
			}
			assert.True(t, probed)
			assert.Equal(t, tt.wantUpdate, updated)
			assert.NoFileExists(t, exe+".lock")
		})
	}
}

func TestSwapRejectsSameVersionReplacementWithoutRunningImageProof(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("Linux verifies the running image through /proc/self/exe")
	}
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	tmp := filepath.Join(dir, "download")
	// This replacement was installed before the updater took its baseline.
	require.NoError(t, os.WriteFile(exe, []byte("REPLACEMENT-SAME-VERSION"), 0o755))
	require.NoError(t, os.WriteFile(tmp, []byte("NEW"), 0o755))
	baseline, err := snapshotExecutable(exe)
	require.NoError(t, err)
	u := New(Config{CurrentVersion: "1.0.0"})
	u.installedVersion = func(context.Context, string) (string, error) { return "1.0.0", nil }
	updated, err := u.swap(context.Background(), exe, tmp, baseline, "2.0.0", false)
	require.ErrorIs(t, err, ErrConcurrentUpdateModified)
	assert.Contains(t, err.Error(), "running image identity")
	assert.False(t, updated)
	assert.Equal(t, "REPLACEMENT-SAME-VERSION", string(mustReadFile(t, exe)))
	assert.NoFileExists(t, exe+".lock")
}

func TestReleaseControlLockPreservesReplacement(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	u := New(Config{})
	release, err := u.acquireReleaseControlLock(context.Background(), exe)
	require.NoError(t, err)
	path := exe + ".lock.ctl"
	require.NoError(t, os.Rename(path, path+".previous"))
	require.NoError(t, os.WriteFile(path, []byte("other owner"), 0o600))
	err = release()
	require.ErrorIs(t, err, ErrUpdateLocked)
	assert.Equal(t, "other owner", string(mustReadFile(t, path)))
}

func TestUpdateLockedOldAbortsBeforeMovingExecutable(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	require.NoError(t, os.WriteFile(f.exePath+".old", []byte("LOCKED"), 0o600))
	f.updater.removeOld = func(string) error { return os.ErrPermission }
	res, err := f.updater.Update(context.Background(), "", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remove stale backup")
	assert.False(t, res.Updated)
	assert.Equal(t, "OLD-BINARY", f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
}

func TestUpdateReleaseFailureSurfacedWhenControlLockHeld(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	ctl := f.exePath + ".lock.ctl"
	require.NoError(t, os.WriteFile(ctl, []byte("other owner"), 0o600))
	f.updater.ctlLockWaitTimeout = 30 * time.Millisecond
	f.updater.ctlLockPollInterval = time.Millisecond

	res, err := f.updater.Update(context.Background(), "", true)
	require.ErrorIs(t, err, ErrUpdateLocked, "release failure must preserve its identity")
	assert.Contains(t, err.Error(), "binary updated to 1.2.0, but release swap lock failed:")
	assert.True(t, res.Updated, "the install succeeded before the release failed")
	assert.Equal(t, string(newBin), f.exeContent())
	assert.FileExists(t, f.exePath+".lock")
	got, readErr := os.ReadFile(ctl)
	require.NoError(t, readErr)
	assert.Equal(t, "other owner", string(got))
}

func TestUpdatePreSwapStatWaitsWhileConcurrentSwapMovesExecutableAside(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	paused := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeFirst := func() { resumeOnce.Do(func() { close(resume) }) }
	defer resumeFirst()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	f.updater.rename = func(oldPath, newPath string) error {
		err := os.Rename(oldPath, newPath)
		if err == nil && oldPath == f.exePath && newPath == f.exePath+".old" {
			close(paused)
			select {
			case <-resume:
			case <-ctx.Done():
			}
		}
		return err
	}
	type outcome struct {
		result Result
		err    error
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)
	go func() {
		res, err := f.updater.Update(ctx, "", true)
		first <- outcome{res, err}
	}()
	select {
	case <-paused:
	case got := <-first:
		t.Fatalf("first update failed before rename gap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("first update did not reach the rename gap")
	}
	assert.FileExists(t, f.exePath+".lock", "swap must hold its lock during the rename gap")
	u2 := New(f.updater.cfg)
	go func() {
		res, err := u2.Update(ctx, "", true)
		second <- outcome{res, err}
	}()
	var early *outcome
	select {
	case got := <-second:
		early = &got
	case <-time.After(300 * time.Millisecond):
	}
	resumeFirst()
	var a, b outcome
	select {
	case a = <-first:
	case <-ctx.Done():
		t.Fatal("first update did not finish")
	}
	if early != nil {
		b = *early
	} else {
		select {
		case b = <-second:
		case <-ctx.Done():
			t.Fatal("second update did not finish")
		}
	}
	assert.Nil(t, early, "second update must wait for the executable to reappear")
	assert.NoError(t, a.err)
	assert.NoError(t, b.err)
	assert.True(t, a.result.Updated)
	assert.True(t, b.result.Updated)
	assert.Equal(t, string(newBin), f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
}

func TestUpdateReleaseSucceedsWhenCallerContextCanceledDuringSwap(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reachedSwap := false
	f.updater.rename = func(from, to string) error {
		if from == f.exePath {
			reachedSwap = true
			cancel()
		}
		return os.Rename(from, to)
	}
	res, err := f.updater.Update(ctx, "", false)
	require.NoError(t, err)
	assert.True(t, reachedSwap)
	assert.True(t, res.Updated)
	assert.Equal(t, string(newBin), f.exeContent())
	assert.NoFileExists(t, f.exePath+".lock")
	assert.NoFileExists(t, f.exePath+".lock.ctl")
}

func TestUpdateTwoConcurrentContendersSerializeWithoutEvictionRace(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	holder := New(Config{})
	release, err := holder.acquireSwapLock(context.Background(), exe)
	require.NoError(t, err)
	require.NotNil(t, release)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type acquisition struct {
		release func() error
		err     error
	}
	start := make(chan struct{})
	done := make(chan acquisition, 2)
	for range 2 {
		go func() {
			select {
			case <-start:
			case <-ctx.Done():
				done <- acquisition{err: ctx.Err()}
				return
			}
			u := New(Config{})
			u.lockWaitTimeout = 4 * time.Second
			u.lockPollInterval = time.Millisecond
			unlock, lockErr := u.acquireSwapLock(ctx, exe)
			done <- acquisition{unlock, lockErr}
		}()
	}
	close(start)
	select {
	case got := <-done:
		if got.release != nil {
			assert.NoError(t, got.release())
		}
		t.Errorf("contender returned while holder owns lock: %v", got.err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, release())
	var first acquisition
	select {
	case first = <-done:
	case <-ctx.Done():
		t.Fatal("first contender did not acquire")
	}
	require.NoError(t, first.err)
	require.NotNil(t, first.release)
	select {
	case got := <-done:
		if got.release != nil {
			assert.NoError(t, got.release())
		}
		t.Errorf("second contender returned while first owns lock: %v", got.err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, first.release())
	select {
	case last := <-done:
		require.NoError(t, last.err)
		require.NotNil(t, last.release)
		require.NoError(t, last.release())
	case <-ctx.Done():
		t.Fatal("second contender did not acquire")
	}
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, exe+".lock.ctl")
}

func TestUpdateOverlappingBaseAndOldInstallationsSerialize(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	old := exe + ".old"
	baseTmp := filepath.Join(dir, "base-download")
	oldTmp := filepath.Join(dir, "old-download")
	require.NoError(t, os.WriteFile(exe, []byte("base"), 0o755))
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(baseTmp, []byte("new base"), 0o755))
	require.NoError(t, os.WriteFile(oldTmp, []byte("new old"), 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	resumeSwap := func() { once.Do(func() { close(resume) }) }
	defer resumeSwap()
	u := New(Config{TestHooks: &TestHooks{AfterMoveAside: func(ctx context.Context) {
		close(paused)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	}}})
	type outcome struct {
		updated bool
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		updated, err := u.swap(ctx, old, oldTmp, executableSnapshot{}, "2.0.0", true)
		done <- outcome{updated, err}
	}()
	select {
	case <-paused:
	case got := <-done:
		t.Fatalf("old installation did not reach swap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("old installation did not reach swap")
	}

	assert.FileExists(t, exe+".lock")
	assert.FileExists(t, old+".lock")
	contender := New(Config{})
	contender.lockPollInterval = time.Millisecond
	waitCtx, stopWaiting := context.WithTimeout(ctx, 200*time.Millisecond)
	updated, err := contender.swap(waitCtx, exe, baseTmp, executableSnapshot{}, "2.0.0", true)
	stopWaiting()
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, updated)
	assert.Equal(t, "base", string(mustReadFile(t, exe)))
	assert.Equal(t, "old", string(mustReadFile(t, old+".old")))

	resumeSwap()
	select {
	case got := <-done:
		require.NoError(t, got.err)
		assert.True(t, got.updated)
	case <-ctx.Done():
		t.Fatal("old installation did not finish")
	}
	updated, err = contender.swap(ctx, exe, baseTmp, executableSnapshot{}, "2.0.0", true)
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, "new base", string(mustReadFile(t, exe)))
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, old+".lock")
}

func TestUpdateOverlappingBaseHeldBlocksOldInstallation(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	old := exe + ".old"
	baseTmp := filepath.Join(dir, "base-download")
	oldTmp := filepath.Join(dir, "old-download")
	require.NoError(t, os.WriteFile(exe, []byte("base"), 0o755))
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(baseTmp, []byte("new base"), 0o755))
	require.NoError(t, os.WriteFile(oldTmp, []byte("new old"), 0o755))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{})
	var once sync.Once
	resumeSwap := func() { once.Do(func() { close(resume) }) }
	defer resumeSwap()
	u := New(Config{TestHooks: &TestHooks{AfterMoveAside: func(ctx context.Context) {
		close(paused)
		select {
		case <-resume:
		case <-ctx.Done():
		}
	}}})
	type outcome struct {
		updated bool
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		updated, err := u.swap(ctx, exe, baseTmp, executableSnapshot{}, "2.0.0", true)
		done <- outcome{updated, err}
	}()
	select {
	case <-paused:
	case got := <-done:
		t.Fatalf("base installation did not reach swap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("base installation did not reach swap")
	}

	assert.FileExists(t, exe+".lock")
	contender := New(Config{})
	contender.lockPollInterval = time.Millisecond
	waitCtx, stopWaiting := context.WithTimeout(ctx, 200*time.Millisecond)
	updated, err := contender.swap(waitCtx, old, oldTmp, executableSnapshot{}, "2.0.0", true)
	stopWaiting()
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, updated)
	// The paused base swap owns the backup path; the blocked contender must not touch it.
	assert.Equal(t, "base", string(mustReadFile(t, old)))
	assert.NoFileExists(t, old+".old")
	assert.NoFileExists(t, old+".lock")

	resumeSwap()
	select {
	case got := <-done:
		require.NoError(t, got.err)
		assert.True(t, got.updated)
	case <-ctx.Done():
		t.Fatal("base installation did not finish")
	}
	assert.Equal(t, "new base", string(mustReadFile(t, exe)))
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o755))
	updated, err = contender.swap(ctx, old, oldTmp, executableSnapshot{}, "2.0.0", true)
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, "new old", string(mustReadFile(t, old)))
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, old+".lock")
}

func TestSwapOldChainReleasesAcquiredLocksOnCancellation(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	target := exe + ".old.old"
	require.NoError(t, os.WriteFile(target+".lock", []byte("holder"), 0o600))
	u := New(Config{})
	u.lockPollInterval = time.Millisecond
	var releaseStates [][2]bool
	u.beforeReleaseControlLock = func() {
		baseLocked, baseErr := markerExists(exe + ".lock")
		oldLocked, oldErr := markerExists(exe + ".old.lock")
		assert.NoError(t, baseErr)
		assert.NoError(t, oldErr)
		releaseStates = append(releaseStates, [2]bool{baseLocked, oldLocked})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Cancel only after both prerequisite locks are held so the outcome does
	// not depend on wall-clock timing under load.
	watcherDone := make(chan struct{})
	go func() {
		defer close(watcherDone)
		deadline := time.NewTimer(10 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			baseLocked, baseErr := markerExists(exe + ".lock")
			oldLocked, oldErr := markerExists(exe + ".old.lock")
			if baseErr == nil && oldErr == nil && baseLocked && oldLocked {
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-deadline.C:
				cancel()
				return
			case <-ticker.C:
			}
		}
	}()
	updated, err := u.swap(ctx, target, "", executableSnapshot{}, "2.0.0", true)
	cancel()
	<-watcherDone
	assert.ErrorIs(t, err, context.Canceled)
	assert.False(t, updated)
	assert.Equal(t, [][2]bool{{true, true}, {true, false}}, releaseStates)
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, exe+".old.lock")
	assert.FileExists(t, target+".lock")
}

func TestSwapOldChainAcquiresAndReleasesInOrder(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	target := exe + ".old.old"
	tmp := filepath.Join(dir, "download")
	require.NoError(t, os.WriteFile(target, []byte("old"), 0o755))
	require.NoError(t, os.WriteFile(tmp, []byte("new"), 0o755))
	u := New(Config{})
	var releaseStates [][3]bool
	u.beforeReleaseControlLock = func() {
		var state [3]bool
		for i, path := range []string{exe, exe + ".old", target} {
			locked, err := markerExists(path + ".lock")
			assert.NoError(t, err)
			state[i] = locked
		}
		releaseStates = append(releaseStates, state)
	}
	updated, err := u.swap(context.Background(), target, tmp, executableSnapshot{}, "2.0.0", true)
	require.NoError(t, err)
	assert.True(t, updated)
	assert.Equal(t, [][3]bool{{true, true, true}, {true, true, false}, {true, false, false}}, releaseStates)
	assert.Equal(t, "new", string(mustReadFile(t, target)))
}

func TestSwapOldChainReleasesAcquiredLocksOnLockTimeout(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	target := exe + ".old"
	require.NoError(t, os.WriteFile(target+".lock", []byte("holder"), 0o600))
	u := New(Config{})
	u.lockPollInterval = time.Millisecond
	u.lockWaitTimeout = 50 * time.Millisecond
	updated, err := u.swap(context.Background(), target, "", executableSnapshot{}, "2.0.0", true)
	assert.ErrorIs(t, err, ErrUpdateLocked)
	assert.False(t, updated)
	assert.NoFileExists(t, exe+".lock")
	assert.FileExists(t, target+".lock")
}

func TestUpdateTwoConcurrentControlLockContendersNeverStealLiveControlLock(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	control := []byte("live control owner")
	require.NoError(t, os.WriteFile(exe+".lock.ctl", control, 0o600))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type acquisition struct {
		release func() error
		err     error
	}
	done := make(chan acquisition, 2)
	for range 2 {
		go func() {
			u := New(Config{})
			u.ctlLockWaitTimeout = 100 * time.Millisecond
			u.ctlLockPollInterval = time.Millisecond
			unlock, lockErr := u.acquireReleaseControlLock(ctx, exe)
			done <- acquisition{unlock, lockErr}
		}()
	}
	for range 2 {
		select {
		case got := <-done:
			assert.ErrorIs(t, got.err, ErrUpdateLocked, "live control lock must not be stolen")
			if got.release != nil {
				assert.NoError(t, got.release())
			}
		case <-ctx.Done():
			t.Error("control-lock contender did not finish")
		}
	}
	got, readErr := os.ReadFile(exe + ".lock.ctl")
	assert.NoError(t, readErr)
	assert.Equal(t, control, got)
}

func TestExecutablePathPreservesLegitimateOldFilenameAndFailsClosedOnRenamedRunningImage(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "teams-cli")
	old := exe + ".old"
	require.NoError(t, os.WriteFile(exe, []byte("current"), 0o755))
	require.NoError(t, os.WriteFile(old, []byte("old"), 0o755))

	t.Run("configured old is legitimate", func(t *testing.T) {
		u := New(Config{ExecutablePath: old})
		got, err := u.executablePath()
		require.NoError(t, err)
		assert.Equal(t, old, got)
	})
	t.Run("unconfigured old with verified provenance", func(t *testing.T) {
		u := New(Config{})
		u.osExecutable = func() (string, error) { return old, nil }
		u.args0 = func() string { return old }
		u.evalSymlinks = func(s string) (string, error) { return s, nil }
		u.lookPath = func(s string) (string, error) { return s, nil }
		got, err := u.executablePath()
		require.NoError(t, err)
		assert.Equal(t, old, got)
	})
	t.Run("renamed image with sibling lock fails closed", func(t *testing.T) {
		require.NoError(t, os.WriteFile(exe+".lock", []byte("holder"), 0o600))
		defer func() { assert.NoError(t, os.Remove(exe+".lock")) }()
		u := New(Config{})
		u.osExecutable = func() (string, error) { return old, nil }
		u.args0 = func() string { return old }
		u.evalSymlinks = func(s string) (string, error) { return s, nil }
		u.lookPath = func(s string) (string, error) { return s, nil }
		got, err := u.executablePath()
		require.ErrorIs(t, err, ErrConcurrentUpdateModified)
		assert.Empty(t, got)
	})
	t.Run("mismatched launch without sibling lock fails closed", func(t *testing.T) {
		assert.NoFileExists(t, exe+".lock")
		u := New(Config{})
		u.osExecutable = func() (string, error) { return old, nil }
		u.args0 = func() string { return exe }
		u.evalSymlinks = func(s string) (string, error) { return s, nil }
		u.lookPath = func(s string) (string, error) { return s, nil }
		got, err := u.executablePath()
		require.ErrorIs(t, err, ErrConcurrentUpdateModified)
		assert.Empty(t, got)
	})
	t.Run("deleted running image fails closed", func(t *testing.T) {
		u := New(Config{})
		u.osExecutable = func() (string, error) { return exe + " (deleted)", nil }
		u.args0 = func() string { return exe }
		u.evalSymlinks = func(s string) (string, error) { return s, nil }
		u.lookPath = func(s string) (string, error) { return s, nil }
		got, err := u.executablePath()
		require.ErrorIs(t, err, ErrConcurrentUpdateModified)
		assert.Empty(t, got)
	})
	t.Run("old suffix on alias is not a renamed image", func(t *testing.T) {
		alias := filepath.Join(dir, "launcher.old")
		u := New(Config{})
		u.osExecutable = func() (string, error) { return exe, nil }
		u.args0 = func() string { return alias }
		u.evalSymlinks = func(s string) (string, error) {
			if s == alias {
				return exe, nil
			}
			return s, nil
		}
		u.lookPath = func(s string) (string, error) { return s, nil }
		got, err := u.executablePath()
		require.NoError(t, err)
		assert.Equal(t, exe, got)
	})
	t.Run("old alias targeting current image cannot prove renamed image provenance", func(t *testing.T) {
		alias := filepath.Join(dir, "launcher.old")
		if err := os.Symlink(exe, alias); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		u := New(Config{})
		u.osExecutable = func() (string, error) { return old, nil }
		u.args0 = func() string { return alias }
		got, err := u.executablePath()
		require.ErrorIs(t, err, ErrConcurrentUpdateModified)
		assert.Empty(t, got)
		assert.NoFileExists(t, old+".lock")
		assert.Equal(t, "old", string(mustReadFile(t, old)))
	})
	t.Run("unresolvable launch path fails closed", func(t *testing.T) {
		u := New(Config{})
		u.osExecutable = func() (string, error) { return old, nil }
		u.args0 = func() string { return filepath.Join(dir, "missing-launcher") }
		got, err := u.executablePath()
		require.ErrorIs(t, err, ErrConcurrentUpdateModified)
		assert.Empty(t, got)
	})
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

type testRoundTripper func(*http.Request) (*http.Response, error)

func (f testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// 018.001-T: a running image replaced after the initial snapshot must not be
// overwritten by a non-forced update, even if its version is not known.
func TestUpdateRejectsReplacedRunningImageIdentity(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v2.0.0", binary: []byte("VERSION-2")})
	f.updater.cfg.ExecutablePath = ""
	f.updater.osExecutable = func() (string, error) { return f.exePath, nil }
	f.updater.args0 = func() string { return f.exePath }
	snapshot := false
	f.updater.cfg.TestHooks = &TestHooks{AfterInitialStat: func(path string, err error) {
		snapshot = path == f.exePath && err == nil
	}}
	base := f.srv.Client().Transport
	if base == nil {
		base = http.DefaultTransport
	}
	var once sync.Once
	var replaceErr error
	client := *f.srv.Client()
	client.Transport = testRoundTripper(func(req *http.Request) (*http.Response, error) {
		once.Do(func() {
			replaceErr = os.Rename(f.exePath, f.exePath+".launched")
			if replaceErr == nil {
				replaceErr = os.WriteFile(f.exePath, []byte("OTHER-IMAGE"), 0o755)
			}
		})
		return base.RoundTrip(req)
	})
	f.updater.cfg.HTTPClient = &client
	res, err := f.updater.Update(context.Background(), "", false)
	require.NoError(t, replaceErr)
	require.True(t, snapshot, "the running-image baseline must precede lookup")
	require.ErrorIs(t, err, ErrConcurrentUpdateModified)
	assert.False(t, res.Updated)
	assert.Equal(t, "OTHER-IMAGE", f.exeContent())
	assert.Equal(t, "OLD-BINARY", string(mustReadFile(t, f.exePath+".launched")))
	assert.NoFileExists(t, f.exePath+".lock")
}

// 018.001-T: the initial missing snapshot must not mask a completed swap.
func TestUpdateReprobesImmediatelyAfterTransientMissingExecutable(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	require.NoError(t, os.Remove(f.exePath))
	probed := false
	var restoreErr error
	f.updater.afterMissingMarkerCheck = func() {
		probed = true
		assert.NoFileExists(t, f.exePath+".lock")
		assert.NoFileExists(t, f.exePath+".old")
		restoreErr = os.WriteFile(f.exePath, []byte("RESTORED"), 0o755)
	}
	info, err := f.updater.statExecutable(context.Background(), f.exePath)
	require.NoError(t, restoreErr)
	require.True(t, probed, "marker check must observe absence before the restore")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, int64(len("RESTORED")), info.Size())
}

// 018.001-T: markers belong to a dangling symlink's target, not its alias.
func TestUpdateWaitsForDanglingSymlinkTargetMarkers(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	target := f.exePath
	alias := filepath.Join(filepath.Dir(target), "alias")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	require.NoError(t, os.Rename(target, target+".old"))
	require.NoError(t, os.WriteFile(target+".lock", []byte("holder"), 0o600))
	probed := make(chan error, 1)
	f.updater.cfg.ExecutablePath = alias
	f.updater.cfg.TestHooks = &TestHooks{AfterInitialStat: func(_ string, err error) {
		probed <- err
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	type outcome struct {
		result Result
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		res, err := f.updater.Update(ctx, "", true)
		done <- outcome{res, err}
	}()
	select {
	case err := <-probed:
		assert.ErrorIs(t, err, os.ErrNotExist)
	case got := <-done:
		t.Fatalf("update returned before probing dangling alias: %v", got.err)
	case <-ctx.Done():
		t.Fatal("update did not probe dangling alias")
	}
	select {
	case got := <-done:
		t.Fatalf("update did not wait for target markers: %v", got.err)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, os.Rename(target+".old", target))
	require.NoError(t, os.Remove(target+".lock"))
	select {
	case got := <-done:
		require.NoError(t, got.err)
		assert.True(t, got.result.Updated)
	case <-ctx.Done():
		t.Fatal("update did not resume")
	}
	assert.Equal(t, string(newBin), string(mustReadFile(t, target)))
	assert.NoFileExists(t, target+".lock")
}

// 018.001-T: a non-forced contender that starts during the rename gap must
// not overwrite the newer image after the holder completes.
func TestUpdateNonForceContenderRejectsConcurrentUpgrade(t *testing.T) {
	f := newFixture(t, "1.0.0",
		release{tag: "v3.0.0", binary: []byte("VERSION-3")},
		release{tag: "v2.0.0", binary: []byte("VERSION-2")})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeFirst := func() { resumeOnce.Do(func() { close(resume) }) }
	defer resumeFirst()
	f.updater.rename = func(from, to string) error {
		err := os.Rename(from, to)
		if err == nil && from == f.exePath && to == f.exePath+".old" {
			close(paused)
			select {
			case <-resume:
			case <-ctx.Done():
			}
		}
		return err
	}
	type outcome struct {
		result Result
		err    error
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)
	go func() {
		res, err := f.updater.Update(ctx, "v3.0.0", true)
		first <- outcome{res, err}
	}()
	select {
	case <-paused:
	case got := <-first:
		t.Fatalf("first update did not reach rename gap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("first update did not reach rename gap")
	}
	probed := make(chan error, 1)
	b := New(f.updater.cfg)
	b.cfg.TestHooks = &TestHooks{AfterInitialStat: func(_ string, err error) { probed <- err }}
	go func() {
		res, err := b.Update(ctx, "v2.0.0", false)
		second <- outcome{res, err}
	}()
	select {
	case err := <-probed:
		require.ErrorIs(t, err, os.ErrNotExist)
	case got := <-second:
		t.Fatalf("contender did not observe the rename gap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("contender did not start")
	}
	resumeFirst()
	select {
	case got := <-first:
		require.NoError(t, got.err)
		require.True(t, got.result.Updated)
	case <-ctx.Done():
		t.Fatal("first update did not finish")
	}
	select {
	case got := <-second:
		assert.ErrorIs(t, got.err, ErrConcurrentUpdateModified)
		assert.False(t, got.result.Updated)
	case <-ctx.Done():
		t.Fatal("contender did not finish")
	}
	assert.Equal(t, "VERSION-3", f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
}

// 018.001-T: a contender must not remove the first updater's rollback copy.
func TestUpdateConcurrentSwapsDoNotDeleteRollbackCopy(t *testing.T) {
	f := newFixture(t, "1.0.0", release{tag: "v1.2.0", binary: newBin})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	paused := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	resumeFirst := func() { resumeOnce.Do(func() { close(resume) }) }
	defer resumeFirst()
	f.updater.rename = func(from, to string) error {
		if from == f.exePath && to == f.exePath+".old" {
			if err := os.Rename(from, to); err != nil {
				return err
			}
			close(paused)
			select {
			case <-resume:
			case <-ctx.Done():
			}
			return nil
		}
		if to == f.exePath && from != f.exePath+".old" {
			return os.ErrPermission
		}
		return os.Rename(from, to)
	}
	type outcome struct {
		result Result
		err    error
	}
	first := make(chan outcome, 1)
	second := make(chan outcome, 1)
	go func() {
		res, err := f.updater.Update(ctx, "", true)
		first <- outcome{res, err}
	}()
	select {
	case <-paused:
	case got := <-first:
		t.Fatalf("first update did not reach rename gap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("first update did not reach rename gap")
	}
	probed := make(chan error, 1)
	b := New(f.updater.cfg)
	b.cfg.TestHooks = &TestHooks{AfterInitialStat: func(_ string, err error) { probed <- err }}
	go func() {
		res, err := b.Update(ctx, "", true)
		second <- outcome{res, err}
	}()
	select {
	case err := <-probed:
		require.ErrorIs(t, err, os.ErrNotExist)
	case got := <-second:
		t.Fatalf("contender did not probe the gap: %v", got.err)
	case <-ctx.Done():
		t.Fatal("contender did not probe the gap")
	}
	select {
	case got := <-second:
		t.Fatalf("contender returned before rollback: %v", got.err)
	case <-time.After(150 * time.Millisecond):
	}
	assert.Equal(t, "OLD-BINARY", string(mustReadFile(t, f.exePath+".old")))
	resumeFirst()
	select {
	case got := <-first:
		require.ErrorIs(t, got.err, os.ErrPermission)
		assert.NotContains(t, got.err.Error(), "rollback failed")
		assert.False(t, got.result.Updated)
	case <-ctx.Done():
		t.Fatal("first update did not roll back")
	}
	select {
	case got := <-second:
		require.NoError(t, got.err)
		assert.True(t, got.result.Updated)
	case <-ctx.Done():
		t.Fatal("contender did not finish")
	}
	assert.Equal(t, string(newBin), f.exeContent())
	assert.Equal(t, []string{"teams-cli"}, f.dirEntries())
}

// 018.001-T: canceled acquisition context must not cancel release's control wait.
func TestReleaseWaitsForControlLockAfterCallerCancellation(t *testing.T) {
	exe := filepath.Join(t.TempDir(), "teams-cli")
	u := New(Config{})
	u.ctlLockWaitTimeout = time.Second
	u.ctlLockPollInterval = time.Millisecond
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	release, err := u.acquireSwapLock(ctx, exe)
	require.NoError(t, err)
	require.NotNil(t, release)
	require.NoError(t, os.WriteFile(exe+".lock.ctl", []byte("other"), 0o600))
	cancel()
	done := make(chan error, 1)
	go func() { done <- release() }()
	select {
	case err := <-done:
		t.Fatalf("release returned before control lock was freed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	require.NoError(t, os.Remove(exe+".lock.ctl"))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("release did not finish")
	}
	assert.NoFileExists(t, exe+".lock")
	assert.NoFileExists(t, exe+".lock.ctl")
}
