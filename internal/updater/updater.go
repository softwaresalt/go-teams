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
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
	defaultTimeout             = 10 * time.Minute
	defaultMaxMetadataBytes    = 4 << 20   // release JSON
	defaultMaxChecksumBytes    = 1 << 20   // SHA256SUMS
	defaultMaxBinaryBytes      = 256 << 20 // platform executable
	maxRedirects               = 10
	tempPattern                = ".teams-cli-update-*"
	oldSuffix                  = ".old"
	lockSuffix                 = ".lock"
	lockCtlSuffix              = ".ctl"
	defaultLockWaitTimeout     = 15 * time.Second
	defaultLockPollInterval    = 50 * time.Millisecond
	defaultLockStaleAfter      = 2 * time.Minute
	defaultCtlLockWaitTimeout  = 2 * time.Second
	defaultCtlLockPollInterval = 10 * time.Millisecond
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
	// ErrUpdateLocked reports an update blocked by another update.
	ErrUpdateLocked = errors.New("updater: another update is in progress")
	// ErrConcurrentUpdateModified reports a concurrent change to the executable.
	ErrConcurrentUpdateModified = fmt.Errorf("updater: executable changed during update: %w", ErrUpdateLocked)
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
	// TestHooks supplies deterministic synchronization points for update tests.
	TestHooks *TestHooks
}

// TestHooks defines optional synchronization points for exercising swaps.
type TestHooks struct {
	// AfterMoveAside runs after the executable is moved to its backup.
	AfterMoveAside func(context.Context)
	// AfterInitialStat reports the initial executable snapshot attempt.
	AfterInitialStat func(string, error)
	// RemoveOld overrides stale-backup removal.
	RemoveOld func(string) error
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
	cfg                      Config
	launchSnapshot           executableSnapshot
	launchSnapshotErr        error
	launchSnapshotInjected   bool
	lockWaitTimeout          time.Duration
	lockPollInterval         time.Duration
	lockStaleAfter           time.Duration
	ctlLockWaitTimeout       time.Duration
	ctlLockPollInterval      time.Duration
	now                      func() time.Time
	isProcessAlive           func(int) bool
	installedVersion         func(context.Context, string) (string, error)
	removeOld                func(string) error
	beforeReleaseControlLock func()
	afterMissingMarkerCheck  func()
	osExecutable             func() (string, error)
	evalSymlinks             func(string) (string, error)
	args0                    func() string
	lookPath                 func(string) (string, error)
	// rename is a test seam over os.Rename for rollback testing.
	rename func(oldPath, newPath string) error
	// Download size caps, overridable by tests.
	maxMetadataBytes int64
	maxChecksumBytes int64
	maxBinaryBytes   int64
}

type lockOwner struct {
	PID       int       `json:"pid"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

func waitPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (u *Updater) lockDiagnostic(path string) error {
	recovery := "manual recovery: confirm all teams-cli update processes have exited; if the executable is missing and its .old backup exists, restore .old to the executable; then remove the .lock and any .lock.ctl"
	data, err := os.ReadFile(path)
	var owner lockOwner
	if err != nil || json.Unmarshal(data, &owner) != nil || owner.PID <= 0 || owner.Token == "" || owner.CreatedAt.IsZero() {
		return fmt.Errorf("%w: active contention or unreadable metadata at %s; %s", ErrUpdateLocked, path, recovery)
	}
	age := u.now().Sub(owner.CreatedAt)
	if age > u.lockStaleAfter {
		if !u.isProcessAlive(owner.PID) {
			return fmt.Errorf("%w: %s appears orphaned by dead PID %d (age %s); %s", ErrUpdateLocked, path, owner.PID, age, recovery)
		}
		return fmt.Errorf("%w: %s held by live PID %d past lockStaleAfter (age %s); if no teams-cli update process is running the OS may have reused PID %d; %s", ErrUpdateLocked, path, owner.PID, age, owner.PID, recovery)
	}
	return fmt.Errorf("%w: active contention at %s by PID %d (age %s); %s", ErrUpdateLocked, path, owner.PID, age, recovery)
}

// Windows can report access denied while an existing lock is being deleted.
// Other create errors are contention only if the lock path actually exists.
func lockCreateContended(path string, err error) bool {
	if errors.Is(err, fs.ErrExist) || (runtime.GOOS == "windows" && errors.Is(err, fs.ErrPermission)) {
		return true
	}
	exists, statErr := markerExists(path)
	return statErr == nil && exists
}

// acquireSwapLock creates an exclusive lock which is never evicted by a contender.
func (u *Updater) acquireSwapLock(ctx context.Context, exe string) (func() error, error) {
	path := exe + lockSuffix
	waitCtx, cancel := context.WithTimeout(ctx, u.lockWaitTimeout)
	defer cancel()
	var lastErr error
	for {
		if err := waitCtx.Err(); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(lastErr, fs.ErrPermission) {
				exists, statErr := markerExists(path)
				if statErr != nil || !exists {
					return nil, fmt.Errorf("create lock %s: %w", path, lastErr)
				}
			}
			return nil, u.lockDiagnostic(path)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			var token [16]byte
			_, randErr := rand.Read(token[:])
			owner := lockOwner{PID: os.Getpid(), Token: hex.EncodeToString(token[:]), CreatedAt: u.now()}
			var writeErr error
			if randErr == nil {
				writeErr = json.NewEncoder(f).Encode(owner)
				if writeErr == nil {
					writeErr = f.Sync()
				}
			}
			closeErr := f.Close()
			if err := errors.Join(randErr, writeErr, closeErr); err != nil {
				// Leave an incomplete lock in place: cleanup without a token is unsafe.
				return nil, fmt.Errorf("initialize lock %s (manual cleanup required): %w", path, err)
			}
			return func() error { return u.releaseSwapLock(ctx, exe, owner.Token) }, nil
		}
		if !lockCreateContended(path, err) {
			return nil, fmt.Errorf("create lock %s: %w", path, err)
		}
		lastErr = err
		if err := waitPoll(waitCtx, u.lockPollInterval); err != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

// acquireSwapLockChain protects both the target and any executable whose backup
// could overlap it. All contenders acquire the shortest prefix first.
func (u *Updater) acquireSwapLockChain(ctx context.Context, exe string) (func() error, error) {
	paths := []string{exe}
	for path := exe; ; {
		name := filepath.Base(path)
		if len(name) <= len(oldSuffix) {
			break
		}
		suffix := name[len(name)-len(oldSuffix):]
		if suffix != oldSuffix && !(runtime.GOOS == "windows" && strings.EqualFold(suffix, oldSuffix)) {
			break
		}
		path = path[:len(path)-len(oldSuffix)]
		paths = append(paths, path)
	}

	releases := make([]func() error, 0, len(paths))
	release := func() error {
		var err error
		for i := len(releases) - 1; i >= 0; i-- {
			err = errors.Join(err, releases[i]())
		}
		return err
	}
	for i := len(paths) - 1; i >= 0; i-- {
		unlock, err := u.acquireSwapLock(ctx, paths[i])
		if err != nil {
			return nil, errors.Join(fmt.Errorf("acquire lock for %s: %w", paths[i], err), release())
		}
		releases = append(releases, unlock)
	}
	return release, nil
}

func (u *Updater) releaseSwapLock(ctx context.Context, exe, token string) error {
	path := exe + lockSuffix
	// This preliminary read is advisory; only the read under the control lock
	// authorizes removal.
	_, initialErr := os.ReadFile(path)
	if initialErr != nil {
		return fmt.Errorf("inspect lock %s: %w", path, initialErr)
	}
	if u.beforeReleaseControlLock != nil {
		u.beforeReleaseControlLock()
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), u.ctlLockWaitTimeout)
	defer cancel()
	releaseControl, err := u.acquireReleaseControlLock(cleanupCtx, exe)
	if err != nil {
		return err
	}
	data, readErr := os.ReadFile(path)
	var owner lockOwner
	if readErr == nil {
		readErr = json.Unmarshal(data, &owner)
	}
	if readErr == nil && owner.Token != token {
		readErr = fmt.Errorf("%w: lock owner changed at %s", ErrUpdateLocked, path)
	}
	if readErr == nil {
		readErr = os.Remove(path)
	}
	if readErr != nil {
		readErr = fmt.Errorf("release lock %s: %w", path, readErr)
	}
	return errors.Join(readErr, releaseControl())
}

// acquireReleaseControlLock fails closed on an existing control lock.
func (u *Updater) acquireReleaseControlLock(ctx context.Context, exe string) (func() error, error) {
	path := exe + lockSuffix + lockCtlSuffix
	waitCtx, cancel := context.WithTimeout(ctx, u.ctlLockWaitTimeout)
	defer cancel()
	var lastErr error
	for {
		if err := waitCtx.Err(); err != nil {
			if errors.Is(lastErr, fs.ErrPermission) {
				exists, statErr := markerExists(path)
				if statErr != nil || !exists {
					return nil, fmt.Errorf("create control lock %s: %w", path, lastErr)
				}
			}
			return nil, fmt.Errorf("%w: control lock %s remains; manual cleanup required after all update processes exit: %v", ErrUpdateLocked, path, err)
		}
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			var tokenBytes [16]byte
			_, randErr := rand.Read(tokenBytes[:])
			token := hex.EncodeToString(tokenBytes[:])
			var writeErr error
			if randErr == nil {
				_, writeErr = f.WriteString(token)
				if writeErr == nil {
					writeErr = f.Sync()
				}
			}
			closeErr := f.Close()
			if err := errors.Join(randErr, writeErr, closeErr); err != nil {
				return nil, fmt.Errorf("initialize control lock %s (manual cleanup required): %w", path, err)
			}
			return func() error {
				data, err := os.ReadFile(path)
				if err != nil {
					return fmt.Errorf("inspect control lock %s: %w", path, err)
				}
				if string(data) != token {
					return fmt.Errorf("%w: control lock owner changed at %s; manual cleanup required", ErrUpdateLocked, path)
				}
				if err := os.Remove(path); err != nil {
					return fmt.Errorf("remove control lock %s: %w", path, err)
				}
				return nil
			}, nil
		}
		if !lockCreateContended(path, err) {
			return nil, fmt.Errorf("create control lock %s: %w", path, err)
		}
		lastErr = err
		// The deadline is classified at the top of the loop.
		_ = waitPoll(waitCtx, u.ctlLockPollInterval)
	}
}

// targetPath resolves dangling final symlinks as well as existing paths.
func targetPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	for range 40 {
		vol := filepath.VolumeName(path)
		root := vol + string(filepath.Separator)
		parts := strings.Split(strings.TrimPrefix(path, root), string(filepath.Separator))
		current := root
		found := false
		for i, part := range parts {
			current = filepath.Join(current, part)
			info, err := os.Lstat(current)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					return path, nil
				}
				return "", err
			}
			if info.Mode()&os.ModeSymlink == 0 {
				continue
			}
			link, err := os.Readlink(current)
			if err != nil {
				return "", err
			}
			if !filepath.IsAbs(link) {
				link = filepath.Join(filepath.Dir(current), link)
			}
			path = filepath.Join(append([]string{link}, parts[i+1:]...)...)
			found = true
			break
		}
		if !found {
			return path, nil
		}
	}
	return "", fmt.Errorf("too many symlinks resolving %s", path)
}

func markerExists(path string) (bool, error) {
	_, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// statExecutable waits for an in-flight swap to restore a missing executable.
func (u *Updater) statExecutable(ctx context.Context, exe string) (os.FileInfo, error) {
	waitCtx, cancel := context.WithTimeout(ctx, u.lockWaitTimeout)
	defer cancel()
	for {
		info, err := os.Stat(exe)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return info, err
		}
		target, err := targetPath(exe)
		if err != nil {
			return nil, err
		}
		locked, err := markerExists(target + lockSuffix)
		if err != nil {
			return nil, err
		}
		old, err := markerExists(target + oldSuffix)
		if err != nil {
			return nil, err
		}
		if !locked && !old {
			if u.afterMissingMarkerCheck != nil {
				u.afterMissingMarkerCheck()
			}
			return os.Stat(exe)
		}
		if err := waitPoll(waitCtx, u.lockPollInterval); err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			locked, markerErr := markerExists(target + lockSuffix)
			if markerErr != nil {
				return nil, markerErr
			}
			if locked {
				return nil, u.lockDiagnostic(target + lockSuffix)
			}
			return nil, fmt.Errorf("executable missing while backup exists: %w", fs.ErrNotExist)
		}
	}
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
	return newUpdater(cfg, nil)
}

// newUpdater permits tests to capture a synthetic executable at construction.
// A synthetic snapshot is not evidence of the actual process image.
func newUpdater(cfg Config, executable func() (string, error)) *Updater {
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
	u := &Updater{
		cfg:                 cfg,
		osExecutable:        executable,
		rename:              os.Rename,
		lockWaitTimeout:     defaultLockWaitTimeout,
		lockPollInterval:    defaultLockPollInterval,
		lockStaleAfter:      defaultLockStaleAfter,
		ctlLockWaitTimeout:  defaultCtlLockWaitTimeout,
		ctlLockPollInterval: defaultCtlLockPollInterval,
		now:                 time.Now,
		isProcessAlive:      processAlive,
		maxMetadataBytes:    defaultMaxMetadataBytes,
		maxChecksumBytes:    defaultMaxChecksumBytes,
		maxBinaryBytes:      defaultMaxBinaryBytes,
	}
	if cfg.ExecutablePath == "" {
		if executable == nil {
			u.launchSnapshot = processSnapshot.snapshot
			u.launchSnapshotErr = processSnapshot.err
		} else {
			u.launchSnapshotInjected = true
			path, err := executable()
			if err == nil {
				u.launchSnapshot, err = snapshotExecutable(path)
			}
			u.launchSnapshotErr = err
		}
	}
	return u
}

func processAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(87)) {
			return false // Windows reports ERROR_INVALID_PARAMETER for an absent PID.
		}
		return true // Unknown is not evidence that the owner is dead.
	}
	defer func() {
		if err := p.Release(); err != nil {
			slog.Warn("updater: failed to release process handle", "pid", pid, "error", err)
		}
	}()
	err = p.Signal(syscall.Signal(0))
	return !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH)
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
	raw, pathErr := u.rawExecutablePath()
	var baseline executableSnapshot
	var snapshotErr error
	if pathErr == nil {
		baseline, snapshotErr = snapshotExecutable(raw)
		if u.cfg.TestHooks != nil && u.cfg.TestHooks.AfterInitialStat != nil {
			u.cfg.TestHooks.AfterInitialStat(raw, snapshotErr)
		}
		target, err := targetPath(raw)
		if err != nil {
			pathErr = err
		} else {
			locked, err := markerExists(target + lockSuffix)
			if err != nil {
				pathErr = err
			}
			baseline.concurrent = errors.Is(snapshotErr, fs.ErrNotExist) || locked
		}
	}
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

	if pathErr != nil {
		return res, pathErr
	}
	if snapshotErr != nil && !errors.Is(snapshotErr, fs.ErrNotExist) {
		return res, fmt.Errorf("snapshot executable: %w", snapshotErr)
	}
	exe, err := u.executablePathContext(ctx)
	if err != nil {
		return res, err
	}
	if _, err := u.statExecutable(ctx, exe); err != nil {
		return res, fmt.Errorf("stat executable: %w", err)
	}
	exe, err = targetPath(exe)
	if err != nil {
		return res, fmt.Errorf("resolve executable target: %w", err)
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
	defer func() {
		if err := os.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("updater: failed to remove temporary download", "path", tmp, "error", err)
		}
	}()

	updated, err := u.swap(ctx, exe, tmp, baseline, res.LatestVersion, force)
	res.Updated = updated
	if err != nil {
		return res, err
	}
	return res, nil
}

type executableSnapshot struct {
	info       os.FileInfo
	digest     [sha256.Size]byte
	concurrent bool
}

// On Linux procfs refers to the executable's backing file even if its pathname
// changes. Elsewhere this is only an early pathname snapshot, not proof that
// the file still represented the launch image when initialization began.
// Neither approach proves that file contents already modified before this
// snapshot match the bytes mapped when the process was started.
var processSnapshot = func() struct {
	snapshot executableSnapshot
	err      error
} {
	path := "/proc/self/exe"
	var err error
	if runtime.GOOS != "linux" {
		path, err = os.Executable()
	}
	var snapshot executableSnapshot
	if err == nil {
		snapshot, err = snapshotExecutable(path)
	}
	return struct {
		snapshot executableSnapshot
		err      error
	}{snapshot, err}
}()

func snapshotExecutable(path string) (executableSnapshot, error) {
	var snapshot executableSnapshot
	f, err := os.Open(path)
	if err != nil {
		return snapshot, err
	}
	info, statErr := f.Stat()
	h := sha256.New()
	var readErr error
	if statErr == nil {
		_, readErr = io.Copy(h, f)
	}
	closeErr := f.Close()
	if err := errors.Join(statErr, readErr, closeErr); err != nil {
		return snapshot, err
	}
	snapshot.info = info
	copy(snapshot.digest[:], h.Sum(nil))
	return snapshot, nil
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
	return u.executablePathContext(context.Background())
}

func (u *Updater) rawExecutablePath() (string, error) {
	if u.cfg.ExecutablePath != "" {
		return u.cfg.ExecutablePath, nil
	}
	executable := u.osExecutable
	if executable == nil {
		executable = os.Executable
	}
	exe, err := executable()
	if err != nil {
		return "", fmt.Errorf("resolve executable path: %w", err)
	}
	if strings.HasSuffix(exe, " (deleted)") {
		return "", fmt.Errorf("%w: running executable was deleted", ErrConcurrentUpdateModified)
	}
	return exe, nil
}

func (u *Updater) executablePathContext(ctx context.Context) (string, error) {
	exe, err := u.rawExecutablePath()
	if err != nil {
		return "", err
	}
	if u.cfg.ExecutablePath != "" {
		// Preserve the explicitly configured name for direct calls; Update
		// resolves aliases after waiting for their target to reappear.
		if _, err := u.statExecutable(ctx, exe); err != nil {
			return "", fmt.Errorf("stat executable: %w", err)
		}
		return exe, nil
	}
	eval := u.evalSymlinks
	if eval == nil {
		eval = filepath.EvalSymlinks
	}
	resolved, err := eval(exe)
	if err != nil {
		if _, statErr := u.statExecutable(ctx, exe); statErr != nil {
			return "", fmt.Errorf("resolve executable symlinks: %w", statErr)
		}
		resolved, err = eval(exe)
		if err != nil {
			return "", fmt.Errorf("resolve executable symlinks: %w", err)
		}
	}
	if strings.HasSuffix(resolved, " (deleted)") {
		return "", fmt.Errorf("%w: running executable was deleted", ErrConcurrentUpdateModified)
	}
	if strings.HasSuffix(resolved, oldSuffix) {
		arg := os.Args[0]
		if u.args0 != nil {
			arg = u.args0()
		}
		look := u.lookPath
		if look == nil {
			look = exec.LookPath
		}
		launch, err := look(arg)
		if err != nil {
			return "", fmt.Errorf("%w: cannot resolve launch path: %v", ErrConcurrentUpdateModified, err)
		}
		launch, err = filepath.Abs(launch)
		if err == nil {
			launch, err = eval(launch)
		}
		if err != nil || filepath.Clean(launch) != filepath.Clean(resolved) {
			return "", fmt.Errorf("%w: launch path does not prove executable provenance", ErrConcurrentUpdateModified)
		}
		launchStat, launchErr := os.Stat(launch)
		resolvedStat, resolvedErr := os.Stat(resolved)
		locked, lockErr := markerExists(strings.TrimSuffix(resolved, oldSuffix) + lockSuffix)
		if launchErr != nil || resolvedErr != nil || lockErr != nil || locked || !os.SameFile(launchStat, resolvedStat) {
			return "", fmt.Errorf("%w: cannot verify old executable provenance", ErrConcurrentUpdateModified)
		}
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
	info, err := u.statExecutable(ctx, exe)
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
func (u *Updater) swap(ctx context.Context, exe, tmp string, baseline executableSnapshot, latest string, force bool) (updated bool, err error) {
	release, err := u.acquireSwapLockChain(ctx, exe)
	if err != nil {
		return false, fmt.Errorf("acquire swap lock: %w", err)
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil {
			if updated && err == nil {
				err = fmt.Errorf("binary updated to %s, but release swap lock failed: %w", latest, releaseErr)
			} else {
				err = errors.Join(err, fmt.Errorf("release swap lock: %w", releaseErr))
			}
		}
	}()
	if !force {
		if baseline.concurrent || baseline.info == nil {
			return false, ErrConcurrentUpdateModified
		}
		current, err := snapshotExecutable(exe)
		if err != nil {
			return false, fmt.Errorf("snapshot installed executable: %w", err)
		}
		if !os.SameFile(baseline.info, current.info) || baseline.digest != current.digest {
			return false, ErrConcurrentUpdateModified
		}
		selfUpdate := u.cfg.ExecutablePath == ""
		if selfUpdate {
			// A seam installed after construction cannot provide a construction
			// snapshot; existing synthetic tests still use the baseline guard.
			if u.osExecutable == nil || u.launchSnapshotInjected {
				if u.launchSnapshotErr != nil {
					return false, fmt.Errorf("snapshot process executable at initialization: %w", u.launchSnapshotErr)
				}
				if u.launchSnapshot.info == nil || !os.SameFile(u.launchSnapshot.info, current.info) || u.launchSnapshot.digest != current.digest {
					return false, fmt.Errorf("%w: running image identity differs from initialization snapshot", ErrConcurrentUpdateModified)
				}
			}
			raw, err := u.rawExecutablePath()
			if err != nil {
				return false, fmt.Errorf("inspect running image identity: %w", err)
			}
			running, err := snapshotExecutable(raw)
			if err != nil {
				return false, fmt.Errorf("inspect running image identity: %w", err)
			}
			if !os.SameFile(running.info, current.info) || running.digest != current.digest {
				return false, fmt.Errorf("%w: running image identity differs from target", ErrConcurrentUpdateModified)
			}
		}
		if selfUpdate && u.osExecutable == nil && runtime.GOOS == "linux" {
			running, err := snapshotExecutable("/proc/self/exe")
			if err != nil {
				return false, fmt.Errorf("inspect running image: %w", err)
			}
			if !os.SameFile(running.info, current.info) || running.digest != current.digest {
				return false, ErrConcurrentUpdateModified
			}
		}
		// An injected executable path may point to a synthetic test fixture.
		if u.installedVersion != nil || !selfUpdate || u.osExecutable == nil {
			probe := u.installedVersion
			if probe == nil {
				probe = readInstalledVersion
			}
			installed, err := probe(ctx, exe)
			if err != nil {
				return false, fmt.Errorf("inspect installed version: %w", err)
			}
			if installed == "dev" {
				if _, err := version.IsNewer(installed, latest); err != nil {
					return false, fmt.Errorf("compare installed version: %w", err)
				}
			} else {
				cmp, err := version.Compare(installed, latest)
				if err != nil {
					return false, fmt.Errorf("compare installed version: %w", err)
				}
				if cmp > 0 {
					return false, fmt.Errorf("%w: installed %s, requested %s", ErrDowngrade, installed, latest)
				}
			}
			if strings.TrimSpace(u.cfg.CurrentVersion) != "dev" {
				if installed == "dev" {
					return false, fmt.Errorf("%w: installed version %s differs from running version %s", ErrConcurrentUpdateModified, installed, u.cfg.CurrentVersion)
				}
				cmp, err := version.Compare(installed, u.cfg.CurrentVersion)
				if err != nil {
					return false, fmt.Errorf("%w: compare running version: %v", ErrConcurrentUpdateModified, err)
				}
				if cmp != 0 {
					return false, fmt.Errorf("%w: installed version %s differs from running version %s", ErrConcurrentUpdateModified, installed, u.cfg.CurrentVersion)
				}
			}
			afterProbe, err := snapshotExecutable(exe)
			if err != nil {
				return false, fmt.Errorf("snapshot executable after version inspection: %w", err)
			}
			if !os.SameFile(current.info, afterProbe.info) || current.digest != afterProbe.digest {
				return false, ErrConcurrentUpdateModified
			}
		}
	}
	rename := u.rename
	if rename == nil {
		rename = os.Rename
	}
	old := exe + oldSuffix
	remove := u.removeOld
	if remove == nil && u.cfg.TestHooks != nil {
		remove = u.cfg.TestHooks.RemoveOld
	}
	if remove == nil {
		remove = os.Remove
	}
	if err := remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("remove stale backup %s: %w", old, err)
	}
	if err := rename(exe, old); err != nil {
		return false, fmt.Errorf("move current executable aside: %w", err)
	}
	if u.cfg.TestHooks != nil && u.cfg.TestHooks.AfterMoveAside != nil {
		u.cfg.TestHooks.AfterMoveAside(ctx)
	}
	if err := rename(tmp, exe); err != nil {
		if rbErr := rename(old, exe); rbErr != nil {
			return false, fmt.Errorf("install new executable: %w; rollback failed, previous binary remains at %s: %w", err, old, rbErr)
		}
		return false, fmt.Errorf("install new executable (previous binary restored): %w", err)
	}
	// Best-effort: on Windows the running image keeps the .old file locked; the
	// next update removes it at the start of swap.
	if err := os.Remove(old); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("updater: backup retained for later cleanup", "path", old, "error", err)
	}
	return true, nil
}

func readInstalledVersion(ctx context.Context, exe string) (string, error) {
	output, err := exec.CommandContext(ctx, exe, "--version").Output()
	if err != nil {
		return "", fmt.Errorf("run installed executable --version: %w", err)
	}
	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return "", errors.New("installed executable returned no version")
	}
	installed := fields[len(fields)-1]
	if installed == "dev" {
		return installed, nil
	}
	normalized, err := version.Normalize(installed)
	if err != nil {
		return "", fmt.Errorf("parse installed version: %w", err)
	}
	return normalized, nil
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
