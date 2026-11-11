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

	"github.com/softwaresalt/go-teams/internal/teams"
)

// runQueueFile reads a batch queue file line by line, executes each active
// capture command line, and comments out each completed line in-place by
// prepending ":: " so completed meetings are skipped on subsequent runs.
// Blank lines and lines starting with "::" (after leading whitespace) are
// ignored.
func runQueueFile(ctx context.Context, d *deps, queuePath string, inheritedArgs []string) error {
	data, err := os.ReadFile(queuePath)
	if err != nil {
		return fmt.Errorf("cli: read queue file %s: %w", queuePath, err)
	}

	content := string(data)
	lineEnding := "\n"
	if strings.Contains(content, "\r\n") {
		lineEnding = "\r\n"
	}
	hasTrailingNewline := strings.HasSuffix(content, "\n")

	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if hasTrailingNewline {
		normalized = strings.TrimSuffix(normalized, "\n")
	}
	var lines []string
	if normalized != "" {
		lines = strings.Split(normalized, "\n")
	}

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
		// Strip an optional leading "teams-cli" or "teams-cli.exe" token in
		// case the operator pasted a full command line into the queue file.
		firstBase := strings.ToLower(filepath.Base(lineArgs[0]))
		if firstBase == "teams-cli" || firstBase == "teams-cli.exe" {
			lineArgs = lineArgs[1:]
		}
		if containsQueueFlag(lineArgs) {
			errs = append(errs, fmt.Errorf("cli: queue line %d: nested --queue flag is not allowed", i+1))
			continue
		}

		effectiveArgs := append([]string(nil), inheritedArgs...)
		effectiveArgs = append(effectiveArgs, lineArgs...)

		slog.Info("cli: processing queue entry", "line", i+1, "args", sanitizeQueueArgsForLog(lineArgs))
		if err := executeQueueLine(ctx, d, effectiveArgs); err != nil {
			slog.Error("cli: queue entry failed", "line", i+1, "err", err)
			errs = append(errs, fmt.Errorf("cli: queue line %d: %w", i+1, err))
			continue
		}
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}

		lines[i] = "::" + rawLine
		if writeErr := writeQueueLines(queuePath, lines, lineEnding, hasTrailingNewline); writeErr != nil {
			return fmt.Errorf("cli: update queue file %s after line %d: %w", queuePath, i+1, writeErr)
		}
		slog.Info("cli: completed queue entry", "line", i+1)
	}

	return errors.Join(errs...)
}

type queueExecKey struct{}

// sanitizeQueueArgsForLog renders queue line arguments for logging with every
// URL reduced to scheme://host/path so passcodes, userinfo, and fragments are
// never written to logs.
func sanitizeQueueArgsForLog(args []string) string {
	return teams.RedactArgs(args)
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
