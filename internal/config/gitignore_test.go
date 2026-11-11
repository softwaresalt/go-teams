package config

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// maxScanBytes bounds the size of a tracked file the secret scanner reads.
const maxScanBytes = 8 << 20

// findRepoRoot walks up from the test's working directory to the directory
// containing go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, dir, parent, "go.mod not found above the test working directory")
		dir = parent
	}
}

func TestGitignoreCoversLocalEnvAndSecretPatterns(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(findRepoRoot(t), ".gitignore"))
	require.NoError(t, err)

	lines := map[string]int{}
	for i, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, seen := lines[line]; !seen {
			lines[line] = i
		}
	}
	for _, want := range []string{".env", ".env.*", "*.env", "!.env.example", ".mcp.json", "bin/", "logs/", "transcripts/"} {
		_, ok := lines[want]
		assert.Truef(t, ok, ".gitignore must contain %q", want)
	}
	// A negation only re-includes a path when it follows the pattern it
	// carves an exception out of.
	for _, ignore := range []string{".env.*", "*.env"} {
		if ignoreLine, ok := lines[ignore]; ok {
			assert.Greaterf(t, lines["!.env.example"], ignoreLine,
				"!.env.example must come after %q so the example file stays tracked", ignore)
		}
	}
}

// secretFinding describes a credential-shaped match without echoing the secret.
type secretFinding struct {
	Rule string
	Path string
}

var (
	// livePasscodeRe matches a Teams meeting URL carrying a "p" passcode.
	livePasscodeRe = regexp.MustCompile(
		`(?i)https?://[^\s"'<>]*?(?:teams\.microsoft\.com|teams\.live\.com|teams\.cloud\.microsoft|/meet/)[^\s"'<>]*?[?&]p=([A-Za-z0-9_-]{8,})`)
	// skypeTokenRe matches a SKYPE-TOKEN key assigned an actual JWT value, not
	// a bare storage-key name.
	skypeTokenRe = regexp.MustCompile(`SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}`)
	// bearerRe matches a raw bearer JWT credential.
	bearerRe = regexp.MustCompile(`Bearer\s+eyJ[A-Za-z0-9_-]{10,}`)
)

// syntheticPasscodeMarkers flag a "p" value as an obvious documentation or test
// placeholder rather than a live passcode.
var syntheticPasscodeMarkers = []string{
	"synthetic", "secret", "example", "fake", "dummy", "placeholder",
	"redacted", "sample", "test", "token", "passcode", "notreal", "changeme", "xxxx",
}

func isSyntheticPasscode(value string) bool {
	lower := strings.ToLower(value)
	for _, m := range syntheticPasscodeMarkers {
		if strings.Contains(lower, m) {
			return true
		}
	}
	return false
}

// isLocalEnvPath reports whether a tracked path is a local environment file
// other than the committed .env.example template.
func isLocalEnvPath(p string) bool {
	base := path.Base(filepath.ToSlash(p))
	if base == ".env.example" {
		return false
	}
	return base == ".env" || strings.HasPrefix(base, ".env.") || strings.HasSuffix(base, ".env")
}

// scanTrackedContent reports credential-shaped content in data. Findings carry
// only the rule name and path so the secret is never echoed.
func scanTrackedContent(p string, data []byte) []secretFinding {
	var out []secretFinding
	for _, m := range livePasscodeRe.FindAllSubmatch(data, -1) {
		if !isSyntheticPasscode(string(m[1])) {
			out = append(out, secretFinding{Rule: "live Teams meeting passcode URL", Path: p})
		}
	}
	if skypeTokenRe.Match(data) {
		out = append(out, secretFinding{Rule: "SKYPE-TOKEN credential value", Path: p})
	}
	if bearerRe.Match(data) {
		out = append(out, secretFinding{Rule: "bearer JWT credential value", Path: p})
	}
	return out
}

// findGit locates a git executable on PATH or in standard Windows locations.
func findGit() (string, error) {
	if p, err := exec.LookPath("git"); err == nil {
		return p, nil
	}
	for _, candidate := range []string{
		`C:\Program Files\Git\cmd\git.exe`,
		`C:\Program Files (x86)\Git\cmd\git.exe`,
		`C:\Program Files\Git\bin\git.exe`,
	} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("git executable not found")
}

// gitLsFiles returns the tracked paths from `git ls-files -z`.
func gitLsFiles(root string) ([]string, error) {
	gitPath, err := findGit()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(gitPath, "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, f := range bytes.Split(out, []byte{0}) {
		if len(f) > 0 {
			files = append(files, string(f))
		}
	}
	return files, nil
}

// gitDir resolves the repository's git directory, following a ".git" file
// pointer used by linked worktrees and submodules.
func gitDir(root string) (string, error) {
	dotGit := filepath.Join(root, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return dotGit, nil
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", err
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir:")
	if !ok {
		return "", errors.New(".git file has no gitdir pointer")
	}
	target = strings.TrimSpace(target)
	if !filepath.IsAbs(target) {
		target = filepath.Join(root, target)
	}
	return target, nil
}

// parseGitIndexPaths extracts entry paths from a git index (DIRC v2, v3, v4).
// Unmerged entries (stage != 0) are reported once.
func parseGitIndexPaths(data []byte) ([]string, error) {
	if len(data) < 12 || string(data[:4]) != "DIRC" {
		return nil, errors.New("not a git index: missing DIRC signature")
	}
	version := binary.BigEndian.Uint32(data[4:8])
	if version < 2 || version > 4 {
		return nil, fmt.Errorf("unsupported git index version %d", version)
	}
	count := int(binary.BigEndian.Uint32(data[8:12]))
	const fixed = 62
	pos := 12
	var prev []byte
	seen := map[string]bool{}
	var paths []string
	for i := 0; i < count; i++ {
		if pos+fixed+2 > len(data) {
			return nil, fmt.Errorf("git index truncated at entry %d", i)
		}
		start := pos
		flags := binary.BigEndian.Uint16(data[pos+60 : pos+62])
		pos += fixed
		if version >= 3 && flags&0x4000 != 0 {
			pos += 2
		}
		var name []byte
		if version == 4 {
			strip, n := readIndexVarint(data[pos:])
			if n == 0 || int(strip) > len(prev) {
				return nil, fmt.Errorf("git index v4 entry %d has an invalid prefix length", i)
			}
			pos += n
			end := bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, fmt.Errorf("git index v4 entry %d path is unterminated", i)
			}
			name = append(append([]byte{}, prev[:len(prev)-int(strip)]...), data[pos:pos+end]...)
			pos += end + 1
			prev = name
		} else {
			end := bytes.IndexByte(data[pos:], 0)
			if end < 0 {
				return nil, fmt.Errorf("git index entry %d path is unterminated", i)
			}
			name = data[pos : pos+end]
			entryLen := (pos - start) + end
			pos = start + (entryLen+8)/8*8
		}
		if pos > len(data) {
			return nil, fmt.Errorf("git index truncated after entry %d", i)
		}
		if p := string(name); !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	if err := rejectMandatoryIndexExtensions(data, pos); err != nil {
		return nil, err
	}
	return paths, nil
}

// errUnsupportedIndexExtension reports an index whose entries are incomplete
// without an extension this parser does not implement.
var errUnsupportedIndexExtension = errors.New("git index uses an unsupported mandatory extension")

// rejectMandatoryIndexExtensions walks the extension blocks that follow the
// entries (up to the 20-byte checksum). Git treats an extension as optional
// only when its signature starts with an uppercase ASCII letter; anything else,
// such as the split-index "link" extension that stores only delta entries,
// is mandatory and changes which paths are tracked. Because this parser cannot
// resolve those, it fails closed instead of returning a partial inventory.
func rejectMandatoryIndexExtensions(data []byte, pos int) error {
	end := len(data) - 20
	for pos+8 <= end {
		sig := data[pos : pos+4]
		size := int(binary.BigEndian.Uint32(data[pos+4 : pos+8]))
		if sig[0] < 'A' || sig[0] > 'Z' {
			return fmt.Errorf("%w: %q", errUnsupportedIndexExtension, sig)
		}
		if size < 0 || size > end-pos-8 {
			return errors.New("git index extension is truncated")
		}
		pos += 8 + size
	}
	return nil
}

// readIndexVarint decodes git's offset-encoded varint used by index v4 and
// returns the value and the number of bytes consumed (0 on malformed input).
func readIndexVarint(b []byte) (uint64, int) {
	if len(b) == 0 {
		return 0, 0
	}
	c := b[0]
	value := uint64(c & 0x7f)
	n := 1
	for c&0x80 != 0 {
		if n >= len(b) || n > 9 {
			return 0, 0
		}
		c = b[n]
		n++
		value = ((value + 1) << 7) | uint64(c&0x7f)
	}
	return value, n
}

// trackedFiles enumerates tracked paths strictly from the Git index: first
// `git ls-files -z`, then a pure-Go parse of .git/index. It never walks the
// working tree, so ignored local files such as .env are never reported.
func trackedFiles(root string) ([]string, error) {
	return enumerateTrackedFiles(root, gitLsFiles)
}

// enumerateTrackedFiles is trackedFiles with an injectable `git ls-files`
// lister so the fail-closed path can be exercised without hiding git.
func enumerateTrackedFiles(root string, lsFiles func(root string) ([]string, error)) ([]string, error) {
	files, gitErr := lsFiles(root)
	if gitErr == nil {
		return files, nil
	}
	dir, dirErr := gitDir(root)
	if dirErr != nil {
		return nil, fmt.Errorf("git ls-files unavailable (%v) and .git is unreadable: %w", gitErr, dirErr)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, "index"))
	if readErr != nil {
		return nil, fmt.Errorf("git ls-files unavailable (%v) and .git/index is unreadable: %w", gitErr, readErr)
	}
	files, parseErr := parseGitIndexPaths(data)
	if parseErr != nil {
		return nil, fmt.Errorf("git ls-files unavailable (%v) and .git/index is unparseable: %w", gitErr, parseErr)
	}
	return files, nil
}

// scanTrackedFiles scans every tracked file under root and returns findings.
func scanTrackedFiles(root string, files []string) ([]secretFinding, error) {
	var findings []secretFinding
	for _, rel := range files {
		if isLocalEnvPath(rel) {
			findings = append(findings, secretFinding{Rule: "tracked local environment file", Path: rel})
			continue
		}
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(full)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // tracked but absent from the working tree
			}
			return nil, err
		}
		if !info.Mode().IsRegular() || info.Size() > maxScanBytes {
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			return nil, err
		}
		if bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			continue // binary
		}
		findings = append(findings, scanTrackedContent(rel, data)...)
	}
	return findings, nil
}

func TestTrackedFilesContainNoLivePasscodeOrTokenLeaks(t *testing.T) {
	root := findRepoRoot(t)
	files, err := trackedFiles(root)
	if err != nil {
		t.Fatalf("secret-hygiene prerequisite failed: cannot enumerate tracked files from the Git index: %v", err)
	}
	require.NotEmpty(t, files, "the Git index lists no tracked files")

	findings, err := scanTrackedFiles(root, files)
	require.NoError(t, err)
	for _, f := range findings {
		t.Errorf("tracked file %s contains a %s", f.Path, f.Rule)
	}
}

func TestScanTrackedContentIgnoresMarkerOnlyReferences(t *testing.T) {
	clean := map[string]string{
		"go source key lookup":   `if bytes.Contains(data, []byte("SKYPE-TOKEN")) { return true }`,
		"storage key constant":   `const storageKey = "SKYPE-TOKEN"`,
		"prose mention":          "Teams stores the SKYPE-TOKEN key in local storage.",
		"token marker with text": "SKYPE-TOKEN-value is a marker",
		"bearer word only":       "send a Bearer token in the Authorization header",
		"regex source":           `SKYPE-TOKEN["'\s:=]+eyJ[A-Za-z0-9_-]{10,}`,
		"synthetic passcode":     "https://teams.microsoft.com/meet/123456789?p=synthetic-token",
		"secret placeholder":     "https://teams.microsoft.com/meet/234567890?p=SecretPasscode123",
		"short passcode":         "https://teams.microsoft.com/meet/123?p=abc",
		"non-teams p param":      "https://example.com/page?p=AbCdEfGh12345678",
	}
	for name, content := range clean {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, scanTrackedContent("doc.md", []byte(content)))
		})
	}
}

func TestScanTrackedContentDetectsSyntheticCredentials(t *testing.T) {
	// Assembled at runtime so this file never contains a literal credential.
	skype := "SKYPE-TOKEN=" + "\"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.syntheticPayload\""
	bearer := "Authorization: Bearer " + "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.syntheticPayload"
	passcode := "https://teams.microsoft.com/meet/123456789" + "?p=AbCdEfGh12345678"
	passcodeAmp := "https://teams.cloud.microsoft/l/meetingrecap?threadId=1" + "&p=Zx9Yw8Vu7Ts6"

	tests := []struct {
		name    string
		content string
		rule    string
	}{
		{"skype token assignment", skype, "SKYPE-TOKEN credential value"},
		{"bearer jwt", bearer, "bearer JWT credential value"},
		{"live passcode url", passcode, "live Teams meeting passcode URL"},
		{"live passcode after ampersand", passcodeAmp, "live Teams meeting passcode URL"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := scanTrackedContent("leak.txt", []byte(tc.content))
			require.Len(t, got, 1)
			assert.Equal(t, tc.rule, got[0].Rule)
			assert.Equal(t, "leak.txt", got[0].Path)
		})
	}
}

func TestIsLocalEnvPath(t *testing.T) {
	assert.True(t, isLocalEnvPath(".env"))
	assert.True(t, isLocalEnvPath(".env.local"))
	assert.True(t, isLocalEnvPath("local.env"))
	assert.True(t, isLocalEnvPath("config/prod.env"))
	assert.False(t, isLocalEnvPath(".env.example"))
	assert.False(t, isLocalEnvPath("config/.env.example"))
	assert.False(t, isLocalEnvPath("environment.go"))
}

func TestScanTrackedFilesFlagsTrackedEnvAndSkipsUntracked(t *testing.T) {
	root := t.TempDir()
	// An untracked, ignored local file with a credential must not be reported
	// because the scan only visits paths enumerated from the Git index.
	live := "SKYPE-TOKEN=" + "\"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.syntheticPayload\""
	require.NoError(t, os.WriteFile(filepath.Join(root, "local.env"), []byte(live), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.md"), []byte("SKYPE-TOKEN storage key"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "leak.txt"), []byte(live), 0o600))

	findings, err := scanTrackedFiles(root, []string{"notes.md", "leak.txt", "missing.txt"})
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "leak.txt", findings[0].Path)

	findings, err = scanTrackedFiles(root, []string{".env"})
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "tracked local environment file", findings[0].Rule)
}

// buildIndexV2 encodes a DIRC v2/v3 index with the given paths.
func buildIndexV2(version uint32, paths []string, extended bool) []byte {
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	_ = binary.Write(&buf, binary.BigEndian, version)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(paths)))
	for _, p := range paths {
		entry := make([]byte, 62)
		flags := uint16(len(p))
		if extended {
			flags |= 0x4000
		}
		binary.BigEndian.PutUint16(entry[60:62], flags)
		if extended {
			entry = append(entry, 0, 0)
		}
		entry = append(entry, p...)
		entry = append(entry, 0)
		for len(entry)%8 != 0 {
			entry = append(entry, 0)
		}
		buf.Write(entry)
	}
	buf.WriteString("TREE\x00\x00\x00\x00") // optional trailing extension, ignored
	buf.Write(make([]byte, 20))             // checksum trailer
	return buf.Bytes()
}

// buildIndexV4 encodes a DIRC v4 index using prefix compression.
func buildIndexV4(paths []string) []byte {
	var buf bytes.Buffer
	buf.WriteString("DIRC")
	_ = binary.Write(&buf, binary.BigEndian, uint32(4))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(paths)))
	prev := ""
	for _, p := range paths {
		common := 0
		for common < len(prev) && common < len(p) && prev[common] == p[common] {
			common++
		}
		strip := len(prev) - common
		entry := make([]byte, 62)
		binary.BigEndian.PutUint16(entry[60:62], uint16(len(p)))
		buf.Write(entry)
		buf.WriteByte(byte(strip)) // strip < 128 in these fixtures
		buf.WriteString(p[common:])
		buf.WriteByte(0)
		prev = p
	}
	buf.Write(make([]byte, 20)) // checksum trailer
	return buf.Bytes()
}

func TestParseGitIndexPaths(t *testing.T) {
	paths := []string{".gitignore", "go.mod", "internal/config/config.go", "internal/config/gitignore_test.go", "a"}
	tests := []struct {
		name string
		data []byte
	}{
		{"v2", buildIndexV2(2, paths, false)},
		{"v3 extended flags", buildIndexV2(3, paths, true)},
		{"v4 prefix compressed", buildIndexV4(paths)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGitIndexPaths(tc.data)
			require.NoError(t, err)
			assert.Equal(t, paths, got)
		})
	}
}

func TestParseGitIndexPathsRejectsMalformedInput(t *testing.T) {
	good := buildIndexV2(2, []string{"a.txt"}, false)
	tests := map[string][]byte{
		"empty":       nil,
		"bad magic":   append([]byte("XXXX"), good[4:]...),
		"bad version": append(append(append([]byte{}, good[:4]...), 0, 0, 0, 9), good[8:]...),
		"truncated":   good[:20],
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := parseGitIndexPaths(data)
			assert.Error(t, err)
		})
	}
}

func TestParseGitIndexPathsRejectsMandatoryExtensions(t *testing.T) {
	// A split index stores only delta entries and points at the rest through
	// the lowercase (mandatory) "link" extension.
	good := buildIndexV2(2, []string{"a.txt"}, false)
	body := good[:len(good)-20]
	split := append(append([]byte{}, body...), []byte("link\x00\x00\x00\x00")...)
	split = append(split, make([]byte, 20)...)

	_, err := parseGitIndexPaths(split)
	require.ErrorIs(t, err, errUnsupportedIndexExtension)

	// The same fixture without the mandatory extension still parses.
	got, err := parseGitIndexPaths(good)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt"}, got)

	// A mandatory extension must also fail closed through trackedFiles when git
	// is unavailable.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "index"), split, 0o600))
	_, err = enumerateTrackedFiles(root, func(string) ([]string, error) { return nil, errors.New("git not available") })
	require.ErrorIs(t, err, errUnsupportedIndexExtension)
}

func TestTrackedFilesFailsClosedWithoutGitOrIndex(t *testing.T) {
	failingGit := func(string) ([]string, error) { return nil, errors.New("git not available") }

	_, err := enumerateTrackedFiles(t.TempDir(), failingGit)
	require.Error(t, err, "a missing git binary and missing .git/index must be a hard failure, never a workspace walk")

	// A readable but corrupt index must also fail closed.
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".git"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "index"), []byte("not an index"), 0o600))
	_, err = enumerateTrackedFiles(root, failingGit)
	require.Error(t, err)

	// A readable index is used when git is unavailable.
	require.NoError(t, os.WriteFile(filepath.Join(root, ".git", "index"), buildIndexV2(2, []string{"a.txt", ".env"}, false), 0o600))
	got, err := enumerateTrackedFiles(root, failingGit)
	require.NoError(t, err)
	assert.Equal(t, []string{"a.txt", ".env"}, got)
}

func TestTrackedFilesIndexParserMatchesGitLsFiles(t *testing.T) {
	root := findRepoRoot(t)
	want, err := gitLsFiles(root)
	if err != nil {
		t.Skipf("git ls-files unavailable: %v", err)
	}
	dir, err := gitDir(root)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "index"))
	require.NoError(t, err)
	got, err := parseGitIndexPaths(data)
	if errors.Is(err, errUnsupportedIndexExtension) {
		t.Skipf("index uses a mandatory extension the pure-Go parser rejects by design: %v", err)
	}
	require.NoError(t, err)

	sort.Strings(want)
	sort.Strings(got)
	assert.Equal(t, want, got, "the pure-Go index parser must agree with git ls-files")
}
