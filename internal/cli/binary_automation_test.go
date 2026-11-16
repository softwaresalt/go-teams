package cli

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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
)

const maxCLITestEnvBytes = 64 << 10

var loadedCLITestEnvTB sync.Map

func loadCLITestEnv(t testing.TB) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory for .env.local.test: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			loadCLITestEnvFromFile(t, filepath.Join(dir, ".env.local.test"), dir)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func loadCLITestEnvFromFile(t testing.TB, envPath, repoRoot string) {
	t.Helper()
	if _, loaded := loadedCLITestEnvTB.LoadOrStore(t, struct{}{}); loaded {
		return
	}
	t.Cleanup(func() { loadedCLITestEnvTB.Delete(t) })

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

		if err := verifyCLIEnvFileUntracked(realRoot, filepath.ToSlash(rel)); err != nil {
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
		if finfo.Size() > maxCLITestEnvBytes {
			t.Fatalf("test env file %s exceeds %d byte limit", filepath.Base(envPath), maxCLITestEnvBytes)
		}
		data, err := io.ReadAll(io.LimitReader(f, maxCLITestEnvBytes+1))
		if err != nil {
			t.Fatalf("read test env file %s: %v", filepath.Base(envPath), err)
		}
		if len(data) > maxCLITestEnvBytes {
			t.Fatalf("test env file %s exceeds %d byte limit", filepath.Base(envPath), maxCLITestEnvBytes)
		}

		parsed, err := parseCLIDotEnv(data)
		if err != nil {
			t.Fatalf("parse %s: %v", filepath.Base(envPath), err)
		}

		for k, v := range parsed {
			if strings.TrimSpace(v) == "" {
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

func parseCLIDotEnv(data []byte) (map[string]string, error) {
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

func verifyCLIEnvFileUntracked(realRoot, relSlash string) error {
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

type cliFatalSpyTB struct {
	testing.TB
	mu    sync.Mutex
	fatal string
}

func (s *cliFatalSpyTB) Helper()            {}
func (s *cliFatalSpyTB) Cleanup(fn func())  { s.TB.Cleanup(fn) }
func (s *cliFatalSpyTB) Setenv(k, v string) { s.TB.Setenv(k, v) }
func (s *cliFatalSpyTB) Fatalf(f string, a ...any) {
	s.mu.Lock()
	s.fatal = fmt.Sprintf(f, a...)
	s.mu.Unlock()
	runtime.Goexit()
}

func TestLoadCLITestEnvPrecedenceAndPaths(t *testing.T) {
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

	t.Run("loads GO_TEAMS_BINARY and browser path with alias resolution", func(t *testing.T) {
		clearEnv(t)
		root := t.TempDir()
		envPath := filepath.Join(root, ".env.local.test")
		content := strings.Join([]string{
			`GO_TEAMS_BINARY=".\bin\go-teams.exe"`,
			`TEAMS_BROWSER_PATH="C:\Program Files\teams\msedge.exe"`,
			"TEAMS_TEST_PROFILE_DIR=./bin/profile",
		}, "\n")
		require.NoError(t, os.WriteFile(envPath, []byte(content), 0o600))

		loadCLITestEnvFromFile(t, envPath, root)
		assert.Equal(t, filepath.Join(root, "bin", "go-teams.exe"), os.Getenv("GO_TEAMS_BINARY"))
		assert.Equal(t, `C:\Program Files\teams\msedge.exe`, os.Getenv("TEAMS_BROWSER_PATH"))
		assert.Equal(t, filepath.Join(root, "bin", "profile"), os.Getenv("TEAMS_PROFILE_DIR"))
	})

	t.Run("missing file still syncs process env aliases", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("TEAMS_TEST_BROWSER_PATH", "msedge")
		missingRoot := t.TempDir()
		loadCLITestEnvFromFile(t, filepath.Join(missingRoot, ".env.local.test"), missingRoot)
		assert.Equal(t, "msedge", os.Getenv("TEAMS_BROWSER_PATH"))
	})

	t.Run("process env wins over file", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("GO_TEAMS_BINARY", `C:\custom\go-teams.exe`)
		t.Setenv("TEAMS_TEST_BROWSER_PATH", "msedge")
		root := t.TempDir()
		envPath := filepath.Join(root, ".env.local.test")
		require.NoError(t, os.WriteFile(envPath, []byte("GO_TEAMS_BINARY=./bin/other.exe\nTEAMS_BROWSER_PATH=./bin/other-browser\nGO_TEAMS_INTEGRATION= # inline comment\n"), 0o600))

		loadCLITestEnvFromFile(t, envPath, root)
		assert.Equal(t, `C:\custom\go-teams.exe`, os.Getenv("GO_TEAMS_BINARY"))
		assert.Equal(t, "msedge", os.Getenv("TEAMS_BROWSER_PATH"))
		assert.Empty(t, os.Getenv("GO_TEAMS_INTEGRATION"))
	})
}

func TestLoadCLITestEnvSecurityAndTemplateGuardrails(t *testing.T) {
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
		spy := &cliFatalSpyTB{TB: t}
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

	t.Run("real .env.example leaves allowlisted keys unset", func(t *testing.T) {
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

		loadCLITestEnvFromFile(t, exampleCopy, filepath.Dir(exampleCopy))
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

	t.Run("rejects symlink oversized file traversal drive-relative malformed line and git-tracked env", func(t *testing.T) {
		clearEnv(t)
		root := t.TempDir()

		bigPath := filepath.Join(root, ".env.big")
		require.NoError(t, os.WriteFile(bigPath, bytes.Repeat([]byte("#\n"), (maxCLITestEnvBytes/2)+10), 0o600))
		msg := runExpectFatal(t, func(tb testing.TB) {
			loadCLITestEnvFromFile(tb, bigPath, root)
		})
		assert.Contains(t, msg, "limit")

		travPath := filepath.Join(root, ".env.trav")
		require.NoError(t, os.WriteFile(travPath, []byte("GO_TEAMS_BINARY=../outside-cli-secret\n"), 0o600))
		msg = runExpectFatal(t, func(tb testing.TB) {
			loadCLITestEnvFromFile(tb, travPath, root)
		})
		assert.Contains(t, msg, "escaping repository root")
		assert.NotContains(t, msg, "outside-cli-secret")

		driveRelPath := filepath.Join(root, ".env.driverel")
		require.NoError(t, os.WriteFile(driveRelPath, []byte(`GO_TEAMS_BINARY=C:..\outside-cli-drive.exe`+"\n"), 0o600))
		msg = runExpectFatal(t, func(tb testing.TB) {
			loadCLITestEnvFromFile(tb, driveRelPath, root)
		})
		assert.Contains(t, msg, "escaping repository root")
		assert.NotContains(t, msg, "outside-cli-drive")

		badPath := filepath.Join(root, ".env.bad")
		require.NoError(t, os.WriteFile(badPath, []byte("SECRET_WITHOUT_EQUALS_VALUE\n"), 0o600))
		msg = runExpectFatal(t, func(tb testing.TB) {
			loadCLITestEnvFromFile(tb, badPath, root)
		})
		assert.Contains(t, msg, "line 1:")
		assert.NotContains(t, msg, "SECRET_WITHOUT_EQUALS_VALUE")

		target := filepath.Join(root, "real.env")
		require.NoError(t, os.WriteFile(target, []byte("GO_TEAMS_BINARY=./bin/go-teams.exe\n"), 0o600))
		link := filepath.Join(root, ".env.link")
		if err := os.Symlink(target, link); err == nil {
			msg = runExpectFatal(t, func(tb testing.TB) {
				loadCLITestEnvFromFile(tb, link, root)
			})
			assert.Contains(t, msg, "symlink")
		}

		if _, err := exec.LookPath("git"); err == nil {
			gitRoot := t.TempDir()
			initCmd := exec.Command("git", "init")
			initCmd.Dir = gitRoot
			require.NoError(t, initCmd.Run())

			trackedEnv := filepath.Join(gitRoot, ".env.local.test")
			require.NoError(t, os.WriteFile(trackedEnv, []byte("GO_TEAMS_BINARY=./bin/go-teams.exe\n"), 0o600))
			addCmd := exec.Command("git", "add", "-f", ".env.local.test")
			addCmd.Dir = gitRoot
			require.NoError(t, addCmd.Run())

			msg = runExpectFatal(t, func(tb testing.TB) {
				loadCLITestEnvFromFile(tb, trackedEnv, gitRoot)
			})
			assert.Contains(t, msg, "tracked in git")
		}
	})
}

func TestCompiledBinaryCapturesRecapToExplicitOutput(t *testing.T) {
	loadCLITestEnv(t)
	binary := os.Getenv("GO_TEAMS_BINARY")
	if binary == "" {
		t.Skip("set GO_TEAMS_BINARY to a freshly built go-teams executable")
	}
	binary, err := filepath.Abs(binary)
	require.NoError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write([]byte(`<!doctype html><html><body>
<div class="ms-List"><div class="ms-List-cell" data-transcript-entry-id="synthetic">
<span data-transcript-copy-field="speaker">Presenter</span>
<span data-transcript-copy-field="timestamp">PT3S</span>
<span class="entryText-synthetic">Synthetic compiled binary transcript.</span>
</div></div>
<button aria-label="Copy all"
 onclick="navigator.clipboard.writeText('Synthetic compiled binary AI summary.')">Copy all</button>
</body></html>`))
		if err != nil {
			t.Errorf("write fixture: %v", err)
		}
	}))
	defer srv.Close()

	for _, form := range []string{"root positional", "captions positional", "short flags", "join long flags", "output file", "output directory"} {
		t.Run(form, func(t *testing.T) {
			dir := t.TempDir()
			output := filepath.Join(dir, "nested", "result.md")
			configPath := filepath.Join(dir, "config.yaml")
			require.NoError(t, os.WriteFile(configPath,
				[]byte("output_dir: unused\nprofile_dir: unused\nbot_name: Fixture Bot\n"), 0o600))
			url := srv.URL + "/l/meetingrecap"
			var args []string
			switch form {
			case "root positional":
				args = []string{url, output}
			case "captions positional":
				args = []string{"captions", url, output}
			case "short flags":
				args = []string{"-u", url, "-o", output}
			case "join long flags":
				args = []string{"join", "--url", url, "--output", output}
			case "output file":
				args = []string{"captions", "--url", url, "--output-file", output}
			case "output directory":
				output = filepath.Join(dir, "generated.md")
				args = []string{"captions", "--url", url, "--output-dir", output}
			}
			args = append(args, "--headless", "--profile-dir", filepath.Join(dir, "profile"),
				"--config", configPath)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, args...)
			cmd.Dir = dir
			log, err := cmd.CombinedOutput()
			require.NoError(t, err, "compiled capture failed: %s", log)
			if form == "output directory" {
				files, err := filepath.Glob(filepath.Join(output, "*.md"))
				require.NoError(t, err)
				require.Len(t, files, 1)
				output = files[0]
			}
			content, err := os.ReadFile(output)
			require.NoError(t, err)
			require.Contains(t, string(content), "**Presenter** (00:00:03):\n> Synthetic compiled binary transcript.")
			require.Contains(t, string(content), "\n---\n")
			require.Contains(t, string(content), "Synthetic compiled binary AI summary.")
		})
	}
}
