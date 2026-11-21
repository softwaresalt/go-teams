package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
)

var (
	errQueueEntryNotYetDue         = errors.New("cli: queue entry not yet due")
	errQueueEntryAISummaryDisabled = errors.New("cli: queue AI summary disabled")
	queueFileMu                    sync.Mutex
)

const (
	queueLockTimeout      = 5 * time.Second
	queueLockPollInterval = 10 * time.Millisecond
)

func withQueueFileLock(queuePath string, fn func() error) error {
	queueFileMu.Lock()
	defer queueFileMu.Unlock()

	absPath, err := filepath.Abs(queuePath)
	if err != nil {
		absPath = queuePath
	}
	if dir := filepath.Dir(absPath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0o700)
	}
	lockPath := absPath + ".lock"
	deadline := time.Now().Add(queueLockTimeout)
	for {
		f, openErr := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if openErr == nil {
			_ = f.Close()
			defer func() {
				_ = os.Remove(lockPath)
			}()
			return fn()
		}
		if !errors.Is(openErr, os.ErrExist) {
			return fmt.Errorf("cli: acquire queue lock %s: %w", lockPath, openErr)
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("cli: timed out waiting for queue lock %s", lockPath)
		}
		time.Sleep(queueLockPollInterval)
	}
}

// quoteQueueArg formats s as a single token compatible with splitQueueArgs,
// preserving Windows backslashes (including trailing backslashes) and escaping
// embedded double quotes.
func quoteQueueArg(s string) string {
	escaped := strings.ReplaceAll(s, `"`, `\"`)
	if strings.HasSuffix(escaped, `\`) {
		return `"` + escaped[:len(escaped)-1] + `"` + `'\'`
	}
	return `"` + escaped + `"`
}

func extractQueueFlagValue(args []string, flagName string) string {
	prefix := flagName + "="
	for i := 0; i < len(args); i++ {
		if args[i] == flagName && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if strings.HasPrefix(args[i], prefix) {
			return strings.TrimSpace(strings.TrimPrefix(args[i], prefix))
		}
	}
	return ""
}

func queueLineEnablesAISummary(args []string) bool {
	const prefix = "--no-ai-summary="
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(lower, prefix))
		return value == "false" || value == "0" || value == "off"
	}
	return false
}

func parseQueueContent(data []byte) (lines []string, lineEnding string, hasTrailingNewline bool) {
	content := string(data)
	lineEnding = "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	hasTrailingNewline = strings.HasSuffix(content, "\n")

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if hasTrailingNewline {
		normalized = strings.TrimSuffix(normalized, "\n")
	}
	if normalized != "" {
		lines = strings.Split(normalized, "\n")
	}
	return lines, lineEnding, hasTrailingNewline
}

func appendDeferredAISummaryQueueLine(queuePath, meetingURL, transcriptPath string, notBefore time.Time) error {
	return appendDeferredAISummaryQueueLineWithConfig(queuePath, meetingURL, transcriptPath, notBefore, config.Config{})
}

func appendDeferredAISummaryQueueLineWithConfig(queuePath, meetingURL, transcriptPath string, notBefore time.Time, cfg config.Config) error {
	if strings.TrimSpace(queuePath) == "" {
		return nil
	}
	return withQueueFileLock(queuePath, func() error {
		var (
			lines              []string
			lineEnding         = "\n"
			hasTrailingNewline = true
		)
		if data, err := os.ReadFile(queuePath); err == nil {
			lines, lineEnding, hasTrailingNewline = parseQueueContent(data)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cli: read queue file %s: %w", queuePath, err)
		}

		cleanTarget := filepath.Clean(transcriptPath)
		for _, rawLine := range lines {
			trimmed := strings.TrimSpace(rawLine)
			if trimmed == "" || strings.HasPrefix(trimmed, "::") {
				continue
			}
			if args, err := splitQueueArgs(trimmed); err == nil {
				if got := extractQueueFlagValue(args, "--append-ai-summary"); got != "" && filepath.Clean(got) == cleanTarget {
					return nil
				}
			}
		}

		var b strings.Builder
		fmt.Fprintf(&b, "%s --append-ai-summary %s --not-before %s",
			quoteQueueArg(meetingURL), quoteQueueArg(transcriptPath), notBefore.UTC().Format(time.RFC3339))
		if cPath := strings.TrimSpace(cfg.ConfigPath); cPath != "" {
			fmt.Fprintf(&b, " --config %s", quoteQueueArg(cPath))
		}
		if pDir := strings.TrimSpace(cfg.ProfileDir); pDir != "" {
			fmt.Fprintf(&b, " --profile-dir %s", quoteQueueArg(pDir))
		}
		if bPath := strings.TrimSpace(cfg.BrowserPath); bPath != "" {
			fmt.Fprintf(&b, " --browser-path %s", quoteQueueArg(bPath))
		}
		if acct := strings.TrimSpace(cfg.DefaultAccount); acct != "" {
			fmt.Fprintf(&b, " --account %s", quoteQueueArg(acct))
		}
		if lTimeout := strings.TrimSpace(cfg.LobbyTimeout); lTimeout != "" {
			fmt.Fprintf(&b, " --lobby-timeout %s", quoteQueueArg(lTimeout))
		}
		if cfg.Headless {
			b.WriteString(" --headless")
		}
		lines = append(lines, b.String())
		return writeQueueLines(queuePath, lines, lineEnding, hasTrailingNewline)
	})
}

func markDeferredAISummaryQueueLineDone(queuePath, transcriptPath string) error {
	if strings.TrimSpace(queuePath) == "" || strings.TrimSpace(transcriptPath) == "" {
		return nil
	}
	return withQueueFileLock(queuePath, func() error {
		data, err := os.ReadFile(queuePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("cli: read queue file %s: %w", queuePath, err)
		}
		lines, lineEnding, hasTrailingNewline := parseQueueContent(data)
		cleanTarget := filepath.Clean(transcriptPath)
		changed := false
		for i, rawLine := range lines {
			trimmed := strings.TrimSpace(rawLine)
			if trimmed == "" || strings.HasPrefix(trimmed, "::") {
				continue
			}
			args, splitErr := splitQueueArgs(trimmed)
			if splitErr != nil {
				continue
			}
			if got := extractQueueFlagValue(args, "--append-ai-summary"); got != "" && filepath.Clean(got) == cleanTarget {
				lines[i] = "::" + rawLine
				changed = true
			}
		}
		if !changed {
			return nil
		}
		return writeQueueLines(queuePath, lines, lineEnding, hasTrailingNewline)
	})
}

func markQueueLineCompleted(queuePath string, lineIdx int, originalLine string) error {
	return withQueueFileLock(queuePath, func() error {
		data, err := os.ReadFile(queuePath)
		if err != nil {
			return err
		}
		lines, lineEnding, hasTrailingNewline := parseQueueContent(data)
		if lineIdx >= 0 && lineIdx < len(lines) && lines[lineIdx] == originalLine {
			lines[lineIdx] = "::" + originalLine
		} else {
			for i, l := range lines {
				if l == originalLine {
					lines[i] = "::" + originalLine
					break
				}
			}
		}
		return writeQueueLines(queuePath, lines, lineEnding, hasTrailingNewline)
	})
}

// runQueueFile reads a batch queue file line by line, executes each active
// capture command line, and comments out each completed line in-place by
// prepending ":: " so completed meetings are skipped on subsequent runs.
// Blank lines and lines starting with "::" (after leading whitespace) are
// ignored.
func runQueueFile(ctx context.Context, d *deps, queuePath string, inheritedArgs []string) error {
	return runQueueFileFiltered(ctx, d, queuePath, inheritedArgs, nil)
}

func runQueueFileFiltered(
	ctx context.Context,
	d *deps,
	queuePath string,
	inheritedArgs []string,
	skipLine func([]string) bool,
) error {
	data, err := os.ReadFile(queuePath)
	if err != nil {
		return fmt.Errorf("cli: read queue file %s: %w", queuePath, err)
	}

	lines, _, _ := parseQueueContent(data)

	var errs []error
	for i, rawLine := range lines {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "::") {
			continue
		}

		lineArgs, err := splitQueueArgs(trimmed)
		if err != nil {
			errs = append(errs, fmt.Errorf("cli: queue line %d: %w", i+1, err))
			continue
		}
		if len(lineArgs) == 0 {
			continue
		}
		// Strip an optional leading "go-teams" / "go-teams.exe" (or legacy
		// "teams-cli" / "teams-cli.exe") token in case the operator pasted a
		// full command line into the queue file.
		firstBase := strings.ToLower(filepath.Base(strings.ReplaceAll(lineArgs[0], `\`, "/")))
		if firstBase == "go-teams" || firstBase == "go-teams.exe" || firstBase == "teams-cli" || firstBase == "teams-cli.exe" {
			lineArgs = lineArgs[1:]
		}
		if len(lineArgs) == 0 {
			continue
		}
		if skipLine != nil && skipLine(lineArgs) {
			continue
		}
		if containsQueueFlag(lineArgs) {
			errs = append(errs, fmt.Errorf("cli: queue line %d: nested --queue flag is not allowed", i+1))
			continue
		}
		if notBeforeRaw := extractQueueFlagValue(lineArgs, "--not-before"); notBeforeRaw != "" {
			nb, parseErr := time.Parse(time.RFC3339, notBeforeRaw)
			if parseErr != nil {
				errs = append(errs, fmt.Errorf("cli: queue line %d: invalid --not-before timestamp %q: %w", i+1, notBeforeRaw, parseErr))
				continue
			}
			if time.Now().Before(nb) {
				slog.Info("cli: skipping deferred queue entry before not-before timestamp", "line", i+1)
				continue
			}
		}

		effectiveArgs := append([]string(nil), inheritedArgs...)
		effectiveArgs = append(effectiveArgs, lineArgs...)

		slog.Info("cli: processing queue entry", "line", i+1, "args", sanitizeQueueArgsForLog(lineArgs))
		if err := executeQueueLine(ctx, d, effectiveArgs); err != nil {
			if errors.Is(err, errQueueEntryNotYetDue) ||
				errors.Is(err, errQueueEntryAISummaryDisabled) {
				slog.Info("cli: skipping deferred queue entry before not-before timestamp", "line", i+1)
				continue
			}
			slog.Error("cli: queue entry failed", "line", i+1, "err", err)
			errs = append(errs, fmt.Errorf("cli: queue line %d: %w", i+1, err))
			continue
		}
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}

		if writeErr := markQueueLineCompleted(queuePath, i, rawLine); writeErr != nil {
			return fmt.Errorf("cli: update queue file %s after line %d: %w", queuePath, i+1, writeErr)
		}
		slog.Info("cli: completed queue entry", "line", i+1)
	}

	return errors.Join(errs...)
}

type queueExecKey struct{}

// sanitizeQueueArgsForLog renders queue line arguments for logging with every
// URL reduced to scheme://host/path and any --account identifier redacted so
// passcodes, userinfo, fragments, and raw account identifiers are never written
// to logs.
func sanitizeQueueArgsForLog(args []string) string {
	masked := make([]string, len(args))
	copy(masked, args)
	for i := 0; i < len(masked); i++ {
		arg := masked[i]
		if arg == "--account" && i+1 < len(masked) {
			masked[i+1] = browser.RedactAccountIdentifier(masked[i+1])
			i++
			continue
		}
		if strings.HasPrefix(arg, "--account=") {
			val := strings.TrimPrefix(arg, "--account=")
			masked[i] = "--account=" + browser.RedactAccountIdentifier(val)
		}
	}
	return teams.RedactArgs(masked)
}

func executeQueueLine(ctx context.Context, d *deps, args []string) error {
	root := newRootCommand(d)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.ExecuteContext(context.WithValue(ctx, queueExecKey{}, true))
}

func isInsideQueueExecution(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(queueExecKey{}).(bool)
	return v
}

func containsQueueFlag(args []string) bool {
	for _, a := range args {
		if a == "--queue" || a == "-q" || strings.HasPrefix(a, "--queue=") || strings.HasPrefix(a, "-q") {
			return true
		}
		if len(a) > 1 && a[0] == '-' && a[1] != '-' {
			// Check short flag cluster up to any '=' for '-q'.
			cluster := a[1:]
			if eq := strings.IndexByte(cluster, '='); eq >= 0 {
				cluster = cluster[:eq]
			}
			// Only check when the leading short flags are boolean flags (-h)
			// before -q; flags taking a value (-u, -o, -c, -f) consume the rest
			// of the token as their argument.
			for _, ch := range cluster {
				if ch == 'q' {
					return true
				}
				if ch == 'u' || ch == 'o' || ch == 'c' || ch == 'f' {
					break
				}
			}
		}
	}
	return false
}

// splitQueueArgs splits a queue file line into CLI tokens, respecting single
// and double quotes while preserving Windows backslash file paths.
func splitQueueArgs(line string) ([]string, error) {
	var (
		args    []string
		cur     strings.Builder
		inToken bool
		quote   rune
	)
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		if quote != 0 {
			if ch == '\\' && quote == '"' && i+1 < len(runes) && runes[i+1] == '"' {
				cur.WriteRune('"')
				i++
				continue
			}
			if ch == quote {
				quote = 0
				continue
			}
			cur.WriteRune(ch)
			continue
		}

		switch {
		case ch == '"' || ch == '\'':
			quote = ch
			inToken = true
		case ch == ' ' || ch == '\t':
			if inToken {
				args = append(args, cur.String())
				cur.Reset()
				inToken = false
			}
		default:
			cur.WriteRune(ch)
			inToken = true
		}
	}
	if quote != 0 {
		// The line is deliberately omitted: it may contain a passcode-bearing URL.
		return nil, fmt.Errorf("unterminated quote in queue line")
	}
	if inToken {
		args = append(args, cur.String())
	}
	return args, nil
}

func writeQueueLines(path string, lines []string, lineEnding string, trailingNewline bool) error {
	body := strings.Join(lines, lineEnding)
	if trailingNewline || len(lines) > 0 {
		body += lineEnding
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".queue-*.tmp")
	if err != nil {
		return os.WriteFile(path, []byte(body), 0o600)
	}
	tmpName := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return os.WriteFile(path, []byte(body), 0o600)
	}
	return nil
}
