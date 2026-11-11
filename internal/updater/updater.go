// Package updater implements the teams-cli self-update flow: GitHub Releases
// lookup, SHA-256 verification against SHA256SUMS, and rollback-safe
// executable replacement.
//
// Release assets are the raw platform executables published by
// .github/workflows/release.yml (teams-cli-<goos>-<goarch>[.exe]) plus a
// SHA256SUMS file. The updater never replaces the installed executable with a
// download that is not verified against SHA256SUMS.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/softwaresalt/go-teams/internal/version"
)

// Defaults for Config fields left empty.
const (
	// DefaultAPIBaseURL is the GitHub REST API root.
	DefaultAPIBaseURL = "https://api.github.com"
	// DefaultRepo is the owner/name repository that publishes teams-cli releases.
	DefaultRepo = "softwaresalt/go-teams"
	// ChecksumAsset is the release asset listing SHA-256 digests of binaries.
	ChecksumAsset = "SHA256SUMS"
)

const (
	defaultTimeout          = 10 * time.Minute
	defaultMaxMetadataBytes = 4 << 20   // release JSON
	defaultMaxChecksumBytes = 1 << 20   // SHA256SUMS
	defaultMaxBinaryBytes   = 256 << 20 // platform executable
	maxRedirects            = 10
	tempPattern             = ".teams-cli-update-*"
	oldSuffix               = ".old"
)

// Sentinel errors returned by the updater.
var (
	// ErrChecksumMismatch reports a downloaded binary whose digest differs from SHA256SUMS.
	ErrChecksumMismatch = errors.New("updater: checksum mismatch")
	// ErrChecksumMissing reports a SHA256SUMS file with no entry for the platform binary.
	ErrChecksumMissing = errors.New("updater: no checksum entry for asset")
	// ErrAssetNotFound reports a release lacking the platform binary or SHA256SUMS asset.
	ErrAssetNotFound = errors.New("updater: release asset not found")
	// ErrInsecureURL reports a non-HTTPS URL for a non-loopback host.
	ErrInsecureURL = errors.New("updater: insecure URL")
	// ErrDowngrade reports an attempt to install an older release without force.
	ErrDowngrade = errors.New("updater: target version is older than current version")
)

// Config configures an Updater. Zero values select production defaults.
type Config struct {
	// HTTPClient performs all requests; defaults to a client with a generous timeout.
	HTTPClient *http.Client
	// APIBaseURL is the GitHub API root; defaults to DefaultAPIBaseURL.
	APIBaseURL string
	// Repo is the owner/name repository; defaults to DefaultRepo.
	Repo string
	// CurrentVersion is the running version ("dev" for local builds).
	CurrentVersion string
	// GOOS selects the release asset OS; defaults to runtime.GOOS.
	GOOS string
	// GOARCH selects the release asset architecture; defaults to runtime.GOARCH.
	GOARCH string
	// ExecutablePath is the binary to replace; defaults to the running executable.
	ExecutablePath string
}

// Result describes the outcome of Check or Update.
type Result struct {
	// CurrentVersion is the normalized running version, or "dev".
	CurrentVersion string
	// LatestVersion is the normalized version of the inspected release.
	LatestVersion string
	// TagName is the release tag as published.
	TagName string
	// AssetName is the platform binary asset name.
	AssetName string
	// UpdateAvailable reports whether the inspected release is newer than the running version.
	UpdateAvailable bool
	// Downgrade reports whether the inspected release is older than the running version.
	Downgrade bool
	// Updated reports whether the executable was replaced.
	Updated bool
}

// Updater checks for and installs teams-cli releases.
type Updater struct {
	cfg Config
	// rename is a test seam over os.Rename for rollback testing.
	rename func(oldPath, newPath string) error
	// Download size caps, overridable by tests.
	maxMetadataBytes int64
	maxChecksumBytes int64
	maxBinaryBytes   int64
}

// releaseInfo is the subset of the GitHub release payload the updater reads.
type releaseInfo struct {
	TagName string         `json:"tag_name"`
	Assets  []releaseAsset `json:"assets"`
}

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// New returns an Updater using cfg with defaults applied.
func New(cfg Config) *Updater {
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = DefaultAPIBaseURL
	}
	if cfg.Repo == "" {
		cfg.Repo = DefaultRepo
	}
	if cfg.GOOS == "" {
		cfg.GOOS = runtime.GOOS
	}
	if cfg.GOARCH == "" {
		cfg.GOARCH = runtime.GOARCH
	}
	return &Updater{
		cfg:              cfg,
		rename:           os.Rename,
		maxMetadataBytes: defaultMaxMetadataBytes,
		maxChecksumBytes: defaultMaxChecksumBytes,
		maxBinaryBytes:   defaultMaxBinaryBytes,
	}
}

// AssetName returns the release binary name for goos/goarch.
func AssetName(goos, goarch string) string {
	name := "teams-cli-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// Check inspects the latest release, or targetVersion when non-empty, without
// modifying the executable.
func (u *Updater) Check(ctx context.Context, targetVersion string) (Result, error) {
	res, rel, err := u.lookup(ctx, targetVersion)
	if err != nil {
		return res, err
	}
	// Do not advertise an update that Update could not install: the release
	// must carry the platform binary and SHA256SUMS (neither is downloaded here).
	if res.UpdateAvailable {
		if _, _, err := findAssets(rel, res.AssetName); err != nil {
			return res, err
		}
	}
	return res, nil
}

// Update installs the latest release, or targetVersion when non-empty. It
// refuses unverified downloads and downgrades unless force is set; force also
// reinstalls the current version.
func (u *Updater) Update(ctx context.Context, targetVersion string, force bool) (Result, error) {
	res, rel, err := u.lookup(ctx, targetVersion)
	if err != nil {
		return res, err
	}
	if res.Downgrade && !force {
		return res, fmt.Errorf("%w: running %s, requested %s (use --force to downgrade)",
			ErrDowngrade, res.CurrentVersion, res.LatestVersion)
	}
	if !res.UpdateAvailable && !force {
		return res, nil
	}

	exe, err := u.executablePath()
	if err != nil {
		return res, err
	}
	binaryURL, sumsURL, err := findAssets(rel, res.AssetName)
	if err != nil {
		return res, err
	}

	sums, err := u.fetch(ctx, sumsURL, u.maxChecksumBytes)
	if err != nil {
		return res, fmt.Errorf("download %s: %w", ChecksumAsset, err)
	}
	want, ok := parseChecksums(sums)[res.AssetName]
	if !ok {
		return res, fmt.Errorf("%w: %s", ErrChecksumMissing, res.AssetName)
	}

	tmp, err := u.downloadVerified(ctx, binaryURL, exe, want)
	if err != nil {
		return res, err
	}
	// Best-effort cleanup: after a successful swap tmp no longer exists, and a
	// leftover temp file after a failure is harmless and re-cleaned by the OS.
	defer func() { _ = os.Remove(tmp) }()

	if err := u.swap(exe, tmp); err != nil {
		return res, err
	}
	res.Updated = true
	return res, nil
}

// lookup fetches release metadata and computes the version comparison.
func (u *Updater) lookup(ctx context.Context, targetVersion string) (Result, releaseInfo, error) {
	var res Result
	res.AssetName = AssetName(u.cfg.GOOS, u.cfg.GOARCH)

	current := strings.TrimSpace(u.cfg.CurrentVersion)
	isDev := current == "dev"
	if isDev {
		res.CurrentVersion = current
	} else {
		norm, err := version.Normalize(current)
		if err != nil {
			return res, releaseInfo{}, fmt.Errorf("current version: %w", err)
		}
		res.CurrentVersion = norm
	}

	endpoint, err := u.releaseURL(targetVersion)
	if err != nil {
		return res, releaseInfo{}, err
	}
	body, err := u.fetch(ctx, endpoint, u.maxMetadataBytes)
	if err != nil {
		return res, releaseInfo{}, fmt.Errorf("query release: %w", err)
	}
	var rel releaseInfo
	if err := json.Unmarshal(body, &rel); err != nil {
		return res, releaseInfo{}, fmt.Errorf("decode release metadata: %w", err)
	}
	latest, err := version.Normalize(rel.TagName)
	if err != nil {
		return res, releaseInfo{}, fmt.Errorf("release tag %q: %w", rel.TagName, err)
	}
	res.TagName = rel.TagName
	res.LatestVersion = latest

	if res.UpdateAvailable, err = version.IsNewer(current, latest); err != nil {
		return res, releaseInfo{}, err
	}
	if !isDev {
		cmp, err := version.Compare(latest, res.CurrentVersion)
		if err != nil {
			return res, releaseInfo{}, err
		}
		res.Downgrade = cmp < 0
	}
	return res, rel, nil
}

// releaseURL builds the latest-release or tagged-release API endpoint.
func (u *Updater) releaseURL(targetVersion string) (string, error) {
	base := strings.TrimRight(u.cfg.APIBaseURL, "/")
	if err := validateURL(base); err != nil {
		return "", err
	}
	if !validRepo(u.cfg.Repo) {
		return "", fmt.Errorf("updater: invalid repository %q (want owner/name)", u.cfg.Repo)
	}
	root := base + "/repos/" + u.cfg.Repo + "/releases/"
	if strings.TrimSpace(targetVersion) == "" {
		return root + "latest", nil
	}
	if _, err := version.Normalize(targetVersion); err != nil {
		return "", fmt.Errorf("target version: %w", err)
	}
	// Look up the exact requested tag: normalization drops build metadata
	// ("+build.2"), which is significant for tag names though not for precedence.
	tag := "v" + strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(targetVersion), "v"), "V")
	return root + "tags/" + url.PathEscape(tag), nil
}

// validRepo accepts only owner/name with conservative path-safe characters.
func validRepo(repo string) bool {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return false
	}
	for _, part := range []string{owner, name} {
		if part == "." || part == ".." {
			return false
		}
		for _, r := range part {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-' || r == '_' || r == '.') {
				return false
			}
		}
	}
	return true
}

// findAssets locates the platform binary and SHA256SUMS download URLs and
// validates both before any download starts.
func findAssets(rel releaseInfo, binaryName string) (binaryURL, sumsURL string, err error) {
	for _, a := range rel.Assets {
		switch a.Name {
		case binaryName:
			binaryURL = a.URL
		case ChecksumAsset:
			sumsURL = a.URL
		}
	}
	if binaryURL == "" {
		return "", "", fmt.Errorf("%w: %s in release %s", ErrAssetNotFound, binaryName, rel.TagName)
	}
	if sumsURL == "" {
		return "", "", fmt.Errorf("%w: %s in release %s (refusing unverified update)", ErrAssetNotFound, ChecksumAsset, rel.TagName)
	}
	for _, raw := range []string{binaryURL, sumsURL} {
		if err := validateURL(raw); err != nil {
			return "", "", err
		}
	}
	return binaryURL, sumsURL, nil
}

// executablePath resolves the binary to replace.
func (u *Updater) executablePath() (string, error) {
	if u.cfg.ExecutablePath != "" {
		return u.cfg.ExecutablePath, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", fmt.Errorf("resolve executable symlinks: %w", err)
	}
	return resolved, nil
}

// client returns an HTTP client that re-validates every redirect target.
func (u *Updater) client() *http.Client {
	base := u.cfg.HTTPClient
	if base == nil {
		base = &http.Client{Timeout: defaultTimeout}
	}
	c := *base
	prev := c.CheckRedirect
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if err := validateURL(req.URL.String()); err != nil {
			return err
		}
		if prev != nil {
			return prev(req, via)
		}
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		return nil
	}
	return &c
}

// open issues a validated GET and returns the response on HTTP 200.
func (u *Updater) open(ctx context.Context, rawURL string) (*http.Response, error) {
	if err := validateURL(rawURL); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json, application/octet-stream")
	req.Header.Set("User-Agent", "teams-cli-updater")
	resp, err := u.client().Do(req)
	if err != nil {
		// Signed asset URLs can carry tokens in the query string; keep them out
		// of error text while leaving the wrapped error chain intact.
		var ue *url.Error
		if errors.As(err, &ue) {
			ue.URL = redact(ue.URL)
		}
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close() // status already decides the error; close error adds nothing
		return nil, fmt.Errorf("GET %s: unexpected HTTP status %d", redact(rawURL), resp.StatusCode)
	}
	return resp, nil
}

// fetch downloads a small document, failing if it exceeds limit bytes.
func (u *Updater) fetch(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	resp, err := u.open(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }() // read-only body; close error is not actionable
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return data, nil
}

// downloadVerified streams the binary into a temp file beside exe, verifying
// its SHA-256 digest against wantHex. On any failure the temp file is removed.
func (u *Updater) downloadVerified(ctx context.Context, rawURL, exe, wantHex string) (string, error) {
	info, err := os.Stat(exe)
	if err != nil {
		return "", fmt.Errorf("stat executable: %w", err)
	}
	resp, err := u.open(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // read-only body; close error is not actionable

	f, err := os.CreateTemp(filepath.Dir(exe), tempPattern)
	if err != nil {
		return "", fmt.Errorf("create temp file next to executable: %w", err)
	}
	tmpPath := f.Name()
	if err := u.streamVerified(f, resp.Body, wantHex, info.Mode().Perm()); err != nil {
		_ = os.Remove(tmpPath) // best-effort cleanup; the original error is returned
		return "", err
	}
	return tmpPath, nil
}

// streamVerified copies body into f (closing it), verifies the SHA-256 digest
// against wantHex, and applies perm to the file.
func (u *Updater) streamVerified(f *os.File, body io.Reader, wantHex string, perm fs.FileMode) error {
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(body, u.maxBinaryBytes+1))
	if err != nil {
		_ = f.Close() // already failing; the copy error is the one to report
		return fmt.Errorf("download binary: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if n > u.maxBinaryBytes {
		return fmt.Errorf("download binary: response exceeds %d bytes", u.maxBinaryBytes)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != strings.ToLower(wantHex) {
		return fmt.Errorf("%w: got %s, want %s", ErrChecksumMismatch, got, wantHex)
	}
	if err := os.Chmod(f.Name(), perm); err != nil {
		return fmt.Errorf("set permissions on temp file: %w", err)
	}
	return nil
}

// swap performs the Windows-safe .old rename dance with rollback: a running
// executable can be renamed but not overwritten, so the current binary is
// moved aside first and restored if installing the new one fails.
func (u *Updater) swap(exe, tmp string) error {
	rename := u.rename
	if rename == nil {
		rename = os.Rename
	}
	old := exe + oldSuffix
	if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove stale backup %s: %w", old, err)
	}
	if err := rename(exe, old); err != nil {
		return fmt.Errorf("move current executable aside: %w", err)
	}
	if err := rename(tmp, exe); err != nil {
		if rbErr := rename(old, exe); rbErr != nil {
			return fmt.Errorf("install new executable: %w; rollback failed, previous binary remains at %s: %w", err, old, rbErr)
		}
		return fmt.Errorf("install new executable (previous binary restored): %w", err)
	}
	// Best-effort: on Windows the running image keeps the .old file locked; the
	// next update removes it at the start of swap.
	_ = os.Remove(old)
	return nil
}

// validateURL requires HTTPS for every host except loopback addresses, which
// exist so unit tests can use httptest servers.
func validateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: %q", ErrInsecureURL, redact(raw))
	}
	host := u.Hostname()
	switch {
	case u.Scheme == "https" && host != "":
		return nil
	case u.Scheme == "http" && isLoopback(host):
		return nil
	}
	return fmt.Errorf("%w: %q (HTTPS required for non-loopback hosts)", ErrInsecureURL, redact(raw))
}

func isLoopback(host string) bool {
	switch host {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// redact drops the query string, which may carry signed-URL tokens.
func redact(raw string) string {
	if i := strings.IndexByte(raw, '?'); i >= 0 {
		return raw[:i]
	}
	return raw
}

// parseChecksums extracts lowercase SHA-256 hex digests keyed by file name from
// sha256sum-formatted content. Malformed lines are ignored.
func parseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || len(fields[0]) != sha256.Size*2 {
			continue
		}
		if _, err := hex.DecodeString(fields[0]); err != nil {
			continue
		}
		name := strings.TrimPrefix(strings.TrimPrefix(fields[1], "*"), "./")
		out[name] = strings.ToLower(fields[0])
	}
	return out
}
