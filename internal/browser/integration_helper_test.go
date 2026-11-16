package browser

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const maxTestEnvBytes = 64 << 10

var loadedTestEnvTB sync.Map

// LoadTestEnv loads test-suite configuration variables from the gitignored
// .env.local.test file at the repository root when those variables are not
// already set in the process environment.
func LoadTestEnv(t testing.TB) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory for .env.local.test: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			loadTestEnvFromFile(t, filepath.Join(dir, ".env.local.test"), dir)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func loadTestEnvFromFile(t testing.TB, envPath, repoRoot string) {
	t.Helper()
	if _, loaded := loadedTestEnvTB.LoadOrStore(t, struct{}{}); loaded {
		return
	}
	t.Cleanup(func() { loadedTestEnvTB.Delete(t) })

	filtered := map[string]string{}
	info, err := os.Lstat(envPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("stat test env file %s: %v", filepath.Base(envPath), err)
		}
	} else {
		if info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("test env file %s must not be a symlink", filepath.Base(envPath))
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("test env file %s must be a regular file", filepath.Base(envPath))
		}

		realRoot, err := filepath.EvalSymlinks(repoRoot)
		if err != nil {
			t.Fatalf("resolve repository root for %s: %v", filepath.Base(envPath), err)
		}
		realEnv, err := filepath.EvalSymlinks(envPath)
		if err != nil {
			t.Fatalf("resolve test env path %s: %v", filepath.Base(envPath), err)
		}
		rel, err := filepath.Rel(realRoot, realEnv)
		if err != nil || !filepath.IsLocal(rel) {
			t.Fatalf("test env file %s resolves outside repository root", filepath.Base(envPath))
		}

		if err := verifyEnvFileUntracked(realRoot, filepath.ToSlash(rel)); err != nil {
			t.Fatalf("%v", err)
		}

		f, err := os.Open(realEnv)
		if err != nil {
			t.Fatalf("open test env file %s: %v", filepath.Base(envPath), err)
		}
		defer func() {
			_ = f.Close() // read-only file handle; read errors already checked
		}()

		finfo, err := f.Stat()
		if err != nil {
			t.Fatalf("stat open test env file %s: %v", filepath.Base(envPath), err)
		}
		if !finfo.Mode().IsRegular() {
			t.Fatalf("test env file %s must be a regular file", filepath.Base(envPath))
		}
		if finfo.Size() > maxTestEnvBytes {
			t.Fatalf("test env file %s exceeds %d byte limit", filepath.Base(envPath), maxTestEnvBytes)
		}
		data, err := io.ReadAll(io.LimitReader(f, maxTestEnvBytes+1))
		if err != nil {
			t.Fatalf("read test env file %s: %v", filepath.Base(envPath), err)
		}
		if len(data) > maxTestEnvBytes {
			t.Fatalf("test env file %s exceeds %d byte limit", filepath.Base(envPath), maxTestEnvBytes)
		}

		parsed, err := parseDotEnv(data)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(envPath), err)
		}

		for k, v := range parsed {
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				continue
			}
			if strings.Contains(strings.ToLower(v), "synthetic-placeholder") || strings.EqualFold(v, "synthetic-user@example.com") {
				continue
			}
			filtered[k] = v
		}
	}

	resolveRelative := func(keyName, val string, isBrowserPath bool) string {
		hasDrive := len(val) >= 2 && ((val[0] >= 'A' && val[0] <= 'Z') || (val[0] >= 'a' && val[0] <= 'z')) && val[1] == ':'
		if hasDrive && !filepath.IsAbs(val) {
			t.Fatalf("test env key %s has invalid relative path escaping repository root", keyName)
		}
		if filepath.IsAbs(val) || strings.HasPrefix(val, "/") || strings.HasPrefix(val, `\`) {
			return val
		}
		if isBrowserPath && !strings.HasPrefix(val, ".") && !strings.ContainsAny(val, `/\`) {
			return val
		}
		norm := strings.NewReplacer("/", string(filepath.Separator), `\`, string(filepath.Separator)).Replace(val)
		cleaned := filepath.Clean(norm)
		if cleaned == "." || !filepath.IsLocal(cleaned) {
			t.Fatalf("test env key %s has invalid relative path escaping repository root", keyName)
		}
		return filepath.Join(repoRoot, cleaned)
	}

	type aliasPair struct {
		primary       string
		alias         string
		isPath        bool
		isBrowserPath bool
	}
	pairs := []aliasPair{
		{primary: "TEAMS_ACCOUNT", alias: "TEAMS_TEST_ACCOUNT"},
		{primary: "TEAMS_MEETING_URL", alias: "TEAMS_TEST_MEETING_URL"},
		{primary: "TEAMS_RECAP_URL", alias: "TEAMS_TEST_RECAP_URL"},
		{primary: "TEAMS_PROFILE_DIR", alias: "TEAMS_TEST_PROFILE_DIR", isPath: true},
		{primary: "TEAMS_BROWSER_PATH", alias: "TEAMS_TEST_BROWSER_PATH", isPath: true, isBrowserPath: true},
	}
	for _, pair := range pairs {
		procPrimary := strings.TrimSpace(os.Getenv(pair.primary))
		procAlias := strings.TrimSpace(os.Getenv(pair.alias))
		if procPrimary != "" || procAlias != "" {
			if procPrimary != "" && procAlias == "" {
				t.Setenv(pair.alias, os.Getenv(pair.primary))
			} else if procPrimary == "" && procAlias != "" {
				t.Setenv(pair.primary, os.Getenv(pair.alias))
			}
			continue
		}
		selectedKey := pair.primary
		val := filtered[pair.primary]
		if val == "" {
			selectedKey = pair.alias
			val = filtered[pair.alias]
		}
		if val == "" {
			continue
		}
		if pair.isPath {
			val = resolveRelative(selectedKey, val, pair.isBrowserPath)
		}
		t.Setenv(pair.primary, val)
		t.Setenv(pair.alias, val)
	}

	for _, key := range []string{"GO_TEAMS_BINARY", "GO_TEAMS_INTEGRATION", "TEAMS_DISABLE_SANDBOX"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			continue
		}
		val := filtered[key]
		if val == "" {
			continue
		}
		if key == "GO_TEAMS_BINARY" {
			val = resolveRelative(key, val, false)
		}
		t.Setenv(key, val)
	}
}

func parseDotEnv(data []byte) (map[string]string, error) {
	text := strings.TrimPrefix(string(data), "\xef\xbb\xbf")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	out := make(map[string]string)

	for i, rawLine := range lines {
		lineNum := i + 1
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if rest, ok := strings.CutPrefix(line, "export "); ok {
			line = strings.TrimSpace(rest)
		} else if rest, ok := strings.CutPrefix(line, "export\t"); ok {
			line = strings.TrimSpace(rest)
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("line %d: expected KEY=VALUE assignment", lineNum)
		}
		key := strings.TrimSpace(line[:eq])
		validKey := len(key) > 0 && ((key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= 'a' && key[0] <= 'z') || key[0] == '_')
		for j := 1; validKey && j < len(key); j++ {
			c := key[j]
			validKey = (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_'
		}
		if !validKey {
			return nil, fmt.Errorf("line %d: invalid variable name", lineNum)
		}

		rawAfterEq := line[eq+1:]
		rawVal := strings.TrimSpace(rawAfterEq)
		var val string
		switch {
		case rawVal == "":
			val = ""
		case strings.HasPrefix(rawVal, "#") && len(rawAfterEq) > 0 && (rawAfterEq[0] == ' ' || rawAfterEq[0] == '\t'):
			val = ""
		case rawVal[0] == '\'':
			end := strings.IndexByte(rawVal[1:], '\'')
			if end < 0 {
				return nil, fmt.Errorf("line %d: unterminated single-quoted value", lineNum)
			}
			val = rawVal[1 : 1+end]
			trail := strings.TrimSpace(rawVal[1+end+1:])
			if trail != "" && !strings.HasPrefix(trail, "#") {
				return nil, fmt.Errorf("line %d: unexpected trailing characters after quoted value", lineNum)
			}
		case rawVal[0] == '"':
			var b strings.Builder
			closed := false
			pos := 1
			for pos < len(rawVal) {
				ch := rawVal[pos]
				if ch == '"' {
					closed = true
					pos++
					break
				}
				if ch == '\\' && pos+1 < len(rawVal) {
					next := rawVal[pos+1]
					if next == '"' || next == '\\' {
						b.WriteByte(next)
						pos += 2
						continue
					}
				}
				b.WriteByte(ch)
				pos++
			}
			if !closed {
				return nil, fmt.Errorf("line %d: unterminated double-quoted value", lineNum)
			}
			trail := strings.TrimSpace(rawVal[pos:])
			if trail != "" && !strings.HasPrefix(trail, "#") {
				return nil, fmt.Errorf("line %d: unexpected trailing characters after quoted value", lineNum)
			}
			val = b.String()
		default:
			cut := len(rawVal)
			for idx := 1; idx < len(rawVal); idx++ {
				if rawVal[idx] == '#' && (rawVal[idx-1] == ' ' || rawVal[idx-1] == '\t') {
					cut = idx
					break
				}
			}
			val = strings.TrimSpace(rawVal[:cut])
		}
		out[key] = val
	}
	return out, nil
}

func verifyEnvFileUntracked(realRoot, relSlash string) error {
	dotGit := filepath.Join(realRoot, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("verify untracked %s: stat .git: %w", relSlash, err)
	}

	gitPath, err := exec.LookPath("git")
	if err != nil {
		for _, candidate := range []string{
			`C:\Program Files\Git\cmd\git.exe`,
			`C:\Program Files (x86)\Git\cmd\git.exe`,
			`C:\Program Files\Git\bin\git.exe`,
		} {
			if cinfo, cerr := os.Stat(candidate); cerr == nil && !cinfo.IsDir() {
				gitPath = candidate
				err = nil
				break
			}
		}
	}
	matchPath := func(entry string) bool {
		if runtime.GOOS == "windows" {
			return strings.EqualFold(entry, relSlash)
		}
		return entry == relSlash
	}
	if err == nil {
		cmd := exec.Command(gitPath, "ls-files", "-z")
		cmd.Dir = realRoot
		out, cmdErr := cmd.Output()
		if cmdErr != nil {
			return fmt.Errorf("verify untracked %s: git ls-files failed: %w", relSlash, cmdErr)
		}
		for _, entry := range bytes.Split(out, []byte{0}) {
			if len(entry) > 0 && matchPath(string(entry)) {
				return fmt.Errorf("test env file %s must not be tracked in git", relSlash)
			}
		}
		return nil
	}

	gitDirPath := dotGit
	if !info.IsDir() {
		rawPtr, ptrErr := os.ReadFile(dotGit)
		if ptrErr != nil {
			return fmt.Errorf("verify untracked %s: read .git pointer: %w", relSlash, ptrErr)
		}
		target, ok := strings.CutPrefix(strings.TrimSpace(string(rawPtr)), "gitdir:")
		if !ok {
			return fmt.Errorf("verify untracked %s: invalid .git pointer", relSlash)
		}
		target = strings.TrimSpace(target)
		if !filepath.IsAbs(target) {
			target = filepath.Join(realRoot, target)
		}
		gitDirPath = target
	}

	idxData, idxErr := os.ReadFile(filepath.Join(gitDirPath, "index"))
	if idxErr != nil {
		return fmt.Errorf("verify untracked %s: read git index: %w", relSlash, idxErr)
	}
	if len(idxData) < 32 || string(idxData[:4]) != "DIRC" {
		return fmt.Errorf("verify untracked %s: invalid git index header", relSlash)
	}
	version := binary.BigEndian.Uint32(idxData[4:8])
	if version < 2 || version > 4 {
		return fmt.Errorf("verify untracked %s: unsupported git index version %d", relSlash, version)
	}
	count := int(binary.BigEndian.Uint32(idxData[8:12]))
	extEnd := len(idxData) - 20
	pos := 12
	var prev []byte
	for i := 0; i < count; i++ {
		if pos+64 > extEnd {
			return fmt.Errorf("verify untracked %s: truncated git index", relSlash)
		}
		start := pos
		flags := binary.BigEndian.Uint16(idxData[pos+60 : pos+62])
		pos += 62
		if version >= 3 && flags&0x4000 != 0 {
			pos += 2
		}
		var name []byte
		if version == 4 {
			if pos >= extEnd {
				return fmt.Errorf("verify untracked %s: truncated v4 index entry", relSlash)
			}
			c := idxData[pos]
			strip := uint64(c & 0x7f)
			n := 1
			for c&0x80 != 0 {
				if pos+n >= extEnd || n > 8 {
					return fmt.Errorf("verify untracked %s: invalid v4 index varint", relSlash)
				}
				c = idxData[pos+n]
				n++
				strip = ((strip + 1) << 7) | uint64(c&0x7f)
			}
			if strip > uint64(len(prev)) {
				return fmt.Errorf("verify untracked %s: invalid v4 prefix length", relSlash)
			}
			pos += n
			end := bytes.IndexByte(idxData[pos:extEnd], 0)
			if end < 0 {
				return fmt.Errorf("verify untracked %s: unterminated v4 index path", relSlash)
			}
			name = append(append([]byte{}, prev[:len(prev)-int(strip)]...), idxData[pos:pos+end]...)
			pos += end + 1
			prev = name
		} else {
			end := bytes.IndexByte(idxData[pos:extEnd], 0)
			if end < 0 {
				return fmt.Errorf("verify untracked %s: unterminated index path", relSlash)
			}
			name = idxData[pos : pos+end]
			entryLen := (pos - start) + end
			pos = start + (entryLen+8)/8*8
		}
		if matchPath(string(name)) {
			return fmt.Errorf("test env file %s must not be tracked in git", relSlash)
		}
	}
	for pos+8 <= extEnd {
		sig := idxData[pos : pos+4]
		size := int(binary.BigEndian.Uint32(idxData[pos+4 : pos+8]))
		if sig[0] < 'A' || sig[0] > 'Z' {
			return fmt.Errorf("verify untracked %s: unsupported mandatory git index extension", relSlash)
		}
		if size < 0 || size > extEnd-pos-8 {
			return fmt.Errorf("verify untracked %s: truncated git index extension", relSlash)
		}
		pos += 8 + size
	}
	if pos != extEnd {
		return fmt.Errorf("verify untracked %s: trailing or truncated git index data", relSlash)
	}
	return nil
}

func TestParseDotEnvSyntaxAndRedaction(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      map[string]string
		wantErr   bool
		redactStr string
	}{
		{
			name:  "unquoted single quoted double quoted and comments",
			input: "# header comment\nTEAMS_ACCOUNT=user@example.com\nTEAMS_MEETING_URL='https://teams.microsoft.com/meet/123?p=abc#frag' # inline\nTEAMS_RECAP_URL=\"https://teams.microsoft.com/l/meetingrecap?a=1&b=2#top\"\n",
			want: map[string]string{
				"TEAMS_ACCOUNT":     "user@example.com",
				"TEAMS_MEETING_URL": "https://teams.microsoft.com/meet/123?p=abc#frag",
				"TEAMS_RECAP_URL":   "https://teams.microsoft.com/l/meetingrecap?a=1&b=2#top",
			},
		},
		{
			name:  "windows backslash preservation in double quotes",
			input: "TEAMS_BROWSER_PATH=\"C:\\Program Files\\teams\\msedge.exe\"\nGO_TEAMS_BINARY=\".\\bin\\go-teams.exe\"\n",
			want: map[string]string{
				"TEAMS_BROWSER_PATH": `C:\Program Files\teams\msedge.exe`,
				"GO_TEAMS_BINARY":    `.\bin\go-teams.exe`,
			},
		},
		{
			name:  "empty assignment with inline comment stays empty",
			input: "GO_TEAMS_INTEGRATION= # set to 1\nTEAMS_BROWSER_PATH=\t# override\n",
			want: map[string]string{
				"GO_TEAMS_INTEGRATION": "",
				"TEAMS_BROWSER_PATH":   "",
			},
		},
		{
			name:  "utf8 bom crlf and export prefix",
			input: "\xef\xbb\xbfexport TEAMS_ACCOUNT=\"alice@contoso.com\"\r\nexport GO_TEAMS_INTEGRATION=1\r\n",
			want: map[string]string{
				"TEAMS_ACCOUNT":        "alice@contoso.com",
				"GO_TEAMS_INTEGRATION": "1",
			},
		},
		{
			name:      "missing equals never echoes secret",
			input:     "SUPER_SECRET_LINE_TOKEN_VALUE\n",
			wantErr:   true,
			redactStr: "SUPER_SECRET_LINE_TOKEN_VALUE",
		},
		{
			name:      "invalid key never echoes key or secret",
			input:     "123BAD_KEY=secret-value-never-echo\n",
			wantErr:   true,
			redactStr: "secret-value-never-echo",
		},
		{
			name:      "unclosed quote never echoes secret",
			input:     "TEAMS_MEETING_URL=\"https://teams.microsoft.com/meet/secret-passcode\n",
			wantErr:   true,
			redactStr: "secret-passcode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDotEnv([]byte(tc.input))
			if tc.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "line 1:")
				assert.NotContains(t, err.Error(), tc.redactStr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLoadTestEnvPrecedenceAndPaths(t *testing.T) {
	clearEnv := func(t *testing.T) {
		for _, k := range []string{
			"GO_TEAMS_INTEGRATION",
			"TEAMS_ACCOUNT",
			"TEAMS_TEST_ACCOUNT",
			"TEAMS_MEETING_URL",
			"TEAMS_TEST_MEETING_URL",
			"TEAMS_RECAP_URL",
			"TEAMS_TEST_RECAP_URL",
			"TEAMS_PROFILE_DIR",
			"TEAMS_TEST_PROFILE_DIR",
			"TEAMS_BROWSER_PATH",
			"TEAMS_TEST_BROWSER_PATH",
			"GO_TEAMS_BINARY",
			"TEAMS_DISABLE_SANDBOX",
		} {
			t.Setenv(k, "")
		}
	}

	t.Run("missing file is a no-op except process alias sync", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("TEAMS_TEST_MEETING_URL", "https://teams.microsoft.com/meet/proc-only")
		root := t.TempDir()
		loadTestEnvFromFile(t, filepath.Join(root, ".env.local.test"), root)
		assert.Empty(t, os.Getenv("TEAMS_ACCOUNT"))
		assert.Equal(t, "https://teams.microsoft.com/meet/proc-only", os.Getenv("TEAMS_MEETING_URL"))
	})

	t.Run("loads all allowlisted keys and resolves relative paths", func(t *testing.T) {
		clearEnv(t)
		root := t.TempDir()
		envPath := filepath.Join(root, ".env.local.test")
		content := strings.Join([]string{
			"GO_TEAMS_INTEGRATION=1",
			"TEAMS_ACCOUNT=alice@contoso.com",
			"TEAMS_MEETING_URL=https://teams.microsoft.com/meet/111?p=samplepass",
			"TEAMS_RECAP_URL=https://teams.microsoft.com/l/meetingrecap?id=222",
			"TEAMS_PROFILE_DIR=./bin/teams-profile",
			`TEAMS_BROWSER_PATH=".\bin\msedge.exe"`,
			"GO_TEAMS_BINARY=./bin/go-teams.exe",
			"TEAMS_DISABLE_SANDBOX=1",
			"PATH=/tmp/evil-override",
		}, "\n")
		require.NoError(t, os.WriteFile(envPath, []byte(content), 0o600))

		loadTestEnvFromFile(t, envPath, root)

		assert.Equal(t, "1", os.Getenv("GO_TEAMS_INTEGRATION"))
		assert.Equal(t, "alice@contoso.com", os.Getenv("TEAMS_ACCOUNT"))
		assert.Equal(t, "alice@contoso.com", os.Getenv("TEAMS_TEST_ACCOUNT"))
		assert.Equal(t, "https://teams.microsoft.com/meet/111?p=samplepass", os.Getenv("TEAMS_MEETING_URL"))
		assert.Equal(t, "https://teams.microsoft.com/meet/111?p=samplepass", os.Getenv("TEAMS_TEST_MEETING_URL"))
		assert.Equal(t, "https://teams.microsoft.com/l/meetingrecap?id=222", os.Getenv("TEAMS_RECAP_URL"))
		assert.Equal(t, "https://teams.microsoft.com/l/meetingrecap?id=222", os.Getenv("TEAMS_TEST_RECAP_URL"))
		assert.Equal(t, filepath.Join(root, "bin", "teams-profile"), os.Getenv("TEAMS_PROFILE_DIR"))
		assert.Equal(t, filepath.Join(root, "bin", "teams-profile"), os.Getenv("TEAMS_TEST_PROFILE_DIR"))
		assert.Equal(t, filepath.Join(root, "bin", "msedge.exe"), os.Getenv("TEAMS_BROWSER_PATH"))
		assert.Equal(t, filepath.Join(root, "bin", "msedge.exe"), os.Getenv("TEAMS_TEST_BROWSER_PATH"))
		assert.Equal(t, filepath.Join(root, "bin", "go-teams.exe"), os.Getenv("GO_TEAMS_BINARY"))
		assert.Equal(t, "1", os.Getenv("TEAMS_DISABLE_SANDBOX"))
		assert.NotEqual(t, "/tmp/evil-override", os.Getenv("PATH"))
	})

	t.Run("process env precedence across alias pairs and primary-over-alias", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("TEAMS_TEST_ACCOUNT", "proc-alias@contoso.com")
		t.Setenv("TEAMS_MEETING_URL", "https://teams.microsoft.com/meet/proc-primary")

		root := t.TempDir()
		envPath := filepath.Join(root, ".env.local.test")
		content := strings.Join([]string{
			"TEAMS_ACCOUNT=file-primary@contoso.com",
			"TEAMS_TEST_ACCOUNT=file-alias@contoso.com",
			"TEAMS_MEETING_URL=https://teams.microsoft.com/meet/file-primary",
			"TEAMS_TEST_MEETING_URL=https://teams.microsoft.com/meet/file-alias",
			"TEAMS_RECAP_URL=https://teams.microsoft.com/l/meetingrecap?primary=1",
			"TEAMS_TEST_RECAP_URL=https://teams.microsoft.com/l/meetingrecap?alias=1",
			"TEAMS_TEST_PROFILE_DIR=./bin/alias-profile",
			"TEAMS_BROWSER_PATH=msedge",
		}, "\n")
		require.NoError(t, os.WriteFile(envPath, []byte(content), 0o600))

		loadTestEnvFromFile(t, envPath, root)

		assert.Equal(t, "proc-alias@contoso.com", os.Getenv("TEAMS_ACCOUNT"))
		assert.Equal(t, "proc-alias@contoso.com", os.Getenv("TEAMS_TEST_ACCOUNT"))
		assert.Equal(t, "https://teams.microsoft.com/meet/proc-primary", os.Getenv("TEAMS_MEETING_URL"))
		assert.Equal(t, "https://teams.microsoft.com/meet/proc-primary", os.Getenv("TEAMS_TEST_MEETING_URL"))
		assert.Equal(t, "https://teams.microsoft.com/l/meetingrecap?primary=1", os.Getenv("TEAMS_RECAP_URL"))
		assert.Equal(t, filepath.Join(root, "bin", "alias-profile"), os.Getenv("TEAMS_PROFILE_DIR"))
		assert.Equal(t, "msedge", os.Getenv("TEAMS_BROWSER_PATH"))

		require.NoError(t, os.WriteFile(envPath, []byte("TEAMS_RECAP_URL=https://teams.microsoft.com/l/meetingrecap?changed=1\n"), 0o600))
		loadTestEnvFromFile(t, envPath, root)
		assert.Equal(t, "https://teams.microsoft.com/l/meetingrecap?primary=1", os.Getenv("TEAMS_RECAP_URL"))
	})
}

type fatalSpyTB struct {
	testing.TB
	mu    sync.Mutex
	fatal string
}

func (s *fatalSpyTB) Helper()            {}
func (s *fatalSpyTB) Cleanup(fn func())  { s.TB.Cleanup(fn) }
func (s *fatalSpyTB) Setenv(k, v string) { s.TB.Setenv(k, v) }
func (s *fatalSpyTB) Fatalf(f string, a ...any) {
	s.mu.Lock()
	s.fatal = fmt.Sprintf(f, a...)
	s.mu.Unlock()
	runtime.Goexit()
}

func TestLoadTestEnvSecurityAndTemplateGuardrails(t *testing.T) {
	clearEnv := func(t *testing.T) {
		for _, k := range []string{
			"GO_TEAMS_INTEGRATION",
			"TEAMS_ACCOUNT",
			"TEAMS_TEST_ACCOUNT",
			"TEAMS_MEETING_URL",
			"TEAMS_TEST_MEETING_URL",
			"TEAMS_RECAP_URL",
			"TEAMS_TEST_RECAP_URL",
			"TEAMS_PROFILE_DIR",
			"TEAMS_TEST_PROFILE_DIR",
			"TEAMS_BROWSER_PATH",
			"TEAMS_TEST_BROWSER_PATH",
			"GO_TEAMS_BINARY",
			"TEAMS_DISABLE_SANDBOX",
		} {
			t.Setenv(k, "")
		}
	}

	runExpectFatal := func(t *testing.T, fn func(tb testing.TB)) string {
		spy := &fatalSpyTB{TB: t}
		done := make(chan struct{})
		go func() {
			defer close(done)
			fn(spy)
		}()
		<-done
		spy.mu.Lock()
		defer spy.mu.Unlock()
		return spy.fatal
	}

	t.Run("real .env.example leaves all allowlisted keys unset", func(t *testing.T) {
		clearEnv(t)
		dir, err := os.Getwd()
		require.NoError(t, err)
		for {
			if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
				break
			}
			parent := filepath.Dir(dir)
			require.NotEqual(t, dir, parent)
			dir = parent
		}
		exampleCopy := filepath.Join(t.TempDir(), ".env.local.test")
		data, err := os.ReadFile(filepath.Join(dir, ".env.example"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(exampleCopy, data, 0o600))

		loadTestEnvFromFile(t, exampleCopy, filepath.Dir(exampleCopy))
		for _, k := range []string{
			"GO_TEAMS_INTEGRATION",
			"TEAMS_ACCOUNT",
			"TEAMS_TEST_ACCOUNT",
			"TEAMS_MEETING_URL",
			"TEAMS_TEST_MEETING_URL",
			"TEAMS_RECAP_URL",
			"TEAMS_TEST_RECAP_URL",
			"TEAMS_PROFILE_DIR",
			"TEAMS_TEST_PROFILE_DIR",
			"TEAMS_BROWSER_PATH",
			"TEAMS_TEST_BROWSER_PATH",
			"GO_TEAMS_BINARY",
			"TEAMS_DISABLE_SANDBOX",
		} {
			assert.Emptyf(t, os.Getenv(k), "key %s from .env.example must remain unset", k)
		}
	})

	t.Run("rejects symlink oversized file traversal drive-relative and git-tracked env", func(t *testing.T) {
		clearEnv(t)
		root := t.TempDir()

		bigPath := filepath.Join(root, ".env.big")
		require.NoError(t, os.WriteFile(bigPath, bytes.Repeat([]byte("#\n"), (maxTestEnvBytes/2)+10), 0o600))
		msg := runExpectFatal(t, func(tb testing.TB) {
			loadTestEnvFromFile(tb, bigPath, root)
		})
		assert.Contains(t, msg, "limit")

		travPath := filepath.Join(root, ".env.trav")
		require.NoError(t, os.WriteFile(travPath, []byte("GO_TEAMS_BINARY=../outside-secret-bin\n"), 0o600))
		msg = runExpectFatal(t, func(tb testing.TB) {
			loadTestEnvFromFile(tb, travPath, root)
		})
		assert.Contains(t, msg, "escaping repository root")
		assert.NotContains(t, msg, "outside-secret-bin")

		driveRelPath := filepath.Join(root, ".env.driverel")
		require.NoError(t, os.WriteFile(driveRelPath, []byte(`GO_TEAMS_BINARY=C:..\outside-drive-secret.exe`+"\n"), 0o600))
		msg = runExpectFatal(t, func(tb testing.TB) {
			loadTestEnvFromFile(tb, driveRelPath, root)
		})
		assert.Contains(t, msg, "escaping repository root")
		assert.NotContains(t, msg, "outside-drive-secret")

		// Direct truncated git index check (no-git fallback invariant) scoped in a
		// nested subtest so PATH is restored before the git-tracked check below.
		t.Run("truncated git index fails closed", func(t *testing.T) {
			truncRoot := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(truncRoot, ".git"), 0o750))
			require.NoError(t, os.WriteFile(filepath.Join(truncRoot, ".git", "index"), []byte("DIRC\x00\x00\x00\x02\x00\x00\x00\x00"), 0o600))
			t.Setenv("PATH", "")
			require.Error(t, verifyEnvFileUntracked(truncRoot, ".env.local.test"))
		})

		target := filepath.Join(root, "real.env")
		require.NoError(t, os.WriteFile(target, []byte("TEAMS_ACCOUNT=a@b.com\n"), 0o600))
		link := filepath.Join(root, ".env.link")
		if err := os.Symlink(target, link); err == nil {
			msg = runExpectFatal(t, func(tb testing.TB) {
				loadTestEnvFromFile(tb, link, root)
			})
			assert.Contains(t, msg, "symlink")
		}

		if _, err := exec.LookPath("git"); err == nil {
			gitRoot := t.TempDir()
			initCmd := exec.Command("git", "init")
			initCmd.Dir = gitRoot
			require.NoError(t, initCmd.Run())

			trackedEnv := filepath.Join(gitRoot, ".env.local.test")
			require.NoError(t, os.WriteFile(trackedEnv, []byte("TEAMS_ACCOUNT=a@b.com\n"), 0o600))
			addCmd := exec.Command("git", "add", "-f", ".env.local.test")
			addCmd.Dir = gitRoot
			require.NoError(t, addCmd.Run())

			msg = runExpectFatal(t, func(tb testing.TB) {
				loadTestEnvFromFile(tb, trackedEnv, gitRoot)
			})
			assert.Contains(t, msg, "tracked in git")
		}
	})
}
