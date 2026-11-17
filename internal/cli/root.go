// Package cli wires the go-teams Cobra command tree.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/softwaresalt/go-teams/internal/browser"
	"github.com/softwaresalt/go-teams/internal/capture"
	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
	"github.com/softwaresalt/go-teams/internal/updater"
	"github.com/softwaresalt/go-teams/internal/version"
)

// deps holds injectable dependencies so commands can be unit-tested with fakes
// in place of real config loading and browser automation.
type deps struct {
	loadConfig func(path string) (config.Config, error)
	newRunner  func(config.Config) browser.Runner
	// newMeeting builds a browser.Meeting bound to a fresh Chromium context
	// derived from ctx, plus a cleanup func that tears the browser down. The
	// returned context is the one that drives the meeting's methods.
	newMeeting func(ctx context.Context, cfg config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error)
	// newSession builds the capture session that records a joined meeting.
	newSession func(cfg config.Config, meetingURL string) meetingSession
	// runUpdate checks for or installs a go-teams release.
	runUpdate func(ctx context.Context, opts updateOptions) (updater.Result, error)
	// writeTemplate writes the commented configuration template to path,
	// replacing an existing file only when force is true.
	writeTemplate func(path string, force bool) error
	// promptCtx, promptIn, promptOut, promptTimeout, and promptMu preserve the
	// outer command's interactive I/O streams safely across --queue and
	// schedule execution.
	promptCtx     context.Context
	promptIn      io.Reader
	promptOut     io.Writer
	promptTimeout time.Duration
	promptMu      sync.Mutex
}

func bindPromptStreams(cmd *cobra.Command, d *deps) {
	if cmd == nil || d == nil || isInsideQueueExecution(cmd.Context()) {
		return
	}
	d.promptCtx = cmd.Context()
	d.promptIn = cmd.InOrStdin()
	d.promptOut = cmd.ErrOrStderr()
}

func promptOverwriteApproval(ctx context.Context, in io.Reader, out io.Writer, paths []string) (bool, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return false, err
		}
	}
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stderr
	}
	if _, err := fmt.Fprintf(out, "File already exists: %s\nOverwrite? (Y)es | (N)o: ", strings.Join(paths, ", ")); err != nil {
		return false, err
	}
	type readResult struct {
		line string
		err  error
	}
	ch := make(chan readResult, 1)
	go func() {
		line, err := readPromptLine(in)
		ch <- readResult{line: line, err: err}
	}()
	var res readResult
	if ctx != nil && ctx.Done() != nil {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case res = <-ch:
		}
	} else {
		res = <-ch
	}
	if res.err != nil && !errors.Is(res.err, io.EOF) {
		return false, res.err
	}
	switch strings.ToLower(strings.TrimSpace(res.line)) {
	case "y", "yes", "(y)es":
		return true, nil
	default:
		return false, nil
	}
}

// readPromptLine reads up to one newline-terminated line byte-by-byte so
// buffered lookahead never consumes answers intended for subsequent --queue
// prompts. Bytes beyond maxPromptLineBytes are drained through the end of the
// line so trailing characters never leak into the next prompt.
func readPromptLine(in io.Reader) (string, error) {
	const maxPromptLineBytes = 4096
	var (
		buf [1]byte
		out strings.Builder
	)
	for {
		n, err := in.Read(buf[:])
		if n > 0 {
			if buf[0] == '\n' {
				return out.String(), nil
			}
			if out.Len() < maxPromptLineBytes {
				out.WriteByte(buf[0])
			}
		}
		if err != nil {
			return out.String(), err
		}
	}
}

func defaultDeps() *deps {
	d := &deps{
		loadConfig:    config.Load,
		runUpdate:     defaultRunUpdate,
		writeTemplate: config.WriteTemplate,
		newRunner: func(c config.Config) browser.Runner {
			return browser.NewChromeRunner(browserOptions(c))
		},
		newMeeting: defaultNewMeeting,
	}
	d.newSession = func(c config.Config, meetingURL string) meetingSession {
		dur, _ := parseOptionalDuration(c.Duration)
		idle, _ := parseOptionalDuration(c.IdleTimeout)
		var (
			confirmOverwrite    func([]string) (bool, error)
			confirmOverwriteCtx func(context.Context, []string) (bool, error)
		)
		if !c.Overwrite {
			confirmOverwriteCtx = func(sessCtx context.Context, paths []string) (bool, error) {
				if !d.promptMu.TryLock() {
					return false, errors.New("cli: prompt stream busy after cancelled prompt")
				}
				defer d.promptMu.Unlock()
				effectiveCtx := sessCtx
				if effectiveCtx == nil {
					effectiveCtx = d.promptCtx
				}
				if effectiveCtx == nil {
					effectiveCtx = context.Background()
				}
				if d.promptTimeout > 0 {
					var cancel context.CancelFunc
					effectiveCtx, cancel = context.WithTimeout(effectiveCtx, d.promptTimeout)
					defer cancel()
				}
				return promptOverwriteApproval(effectiveCtx, d.promptIn, d.promptOut, paths)
			}
			confirmOverwrite = func(paths []string) (bool, error) {
				return confirmOverwriteCtx(d.promptCtx, paths)
			}
		}
		return capture.NewSession(capture.SessionConfig{
			OutputDir:                   c.OutputDir,
			OutputFile:                  c.OutputFile,
			FilenameTemplate:            c.FilenameTemplate,
			MeetingURL:                  meetingURL,
			MeetingTitle:                c.MeetingTitle,
			Category:                    c.Category,
			BotName:                     c.BotName,
			CaptionsOnly:                !c.CaptureAudio,
			Overwrite:                   c.Overwrite,
			ConfirmOverwrite:            confirmOverwrite,
			ConfirmOverwriteWithContext: confirmOverwriteCtx,
			Mode:                        c.CaptureMode,
			Duration:                    dur,
			IdleTimeout:                 idle,
			DisableAISummary:            isAISummaryDisabled(c),
		})
	}
	return d
}

// defaultRunUpdate runs the production updater against GitHub Releases.
func defaultRunUpdate(ctx context.Context, opts updateOptions) (updater.Result, error) {
	u := updater.New(updater.Config{CurrentVersion: version.Version})
	if opts.CheckOnly {
		return u.Check(ctx, opts.TargetVersion)
	}
	return u.Update(ctx, opts.TargetVersion, opts.Force)
}

// browserOptions maps the effective config to the browser allocator options,
// so every command launches the same configured Chromium executable and profile.
func browserOptions(c config.Config) browser.Options {
	return browser.Options{
		UserDataDir:    c.ProfileDir,
		Headless:       c.Headless,
		ExecPath:       c.BrowserPath,
		DefaultAccount: c.DefaultAccount,
	}
}

// browserFlags holds the browser selection flags shared by commands that do
// not bind the full capture flag set (auth and schedule).
type browserFlags struct {
	browserPath string
	profileDir  string
	account     string
}

// bindBrowserFlags registers --browser-path, --profile-dir, and --account on cmd.
func bindBrowserFlags(cmd *cobra.Command, f *browserFlags) {
	cmd.Flags().StringVar(&f.browserPath, "browser-path", "", "path to Chrome/Edge/Chromium executable")
	cmd.Flags().StringVar(&f.profileDir, "profile-dir", "", "browser user-data directory for session persistence")
	cmd.Flags().StringVar(&f.account, "account", "", "default Microsoft Entra account (UPN/email or display name) to auto-select")
}

// applyBrowserFlags overrides cfg with any browser flag the operator supplied.
func applyBrowserFlags(cmd *cobra.Command, cfg *config.Config, f browserFlags) {
	if f.browserPath != "" {
		cfg.BrowserPath = f.browserPath
	}
	if f.profileDir != "" {
		cfg.ProfileDir = f.profileDir
	}
	if (cmd != nil && cmd.Flags().Changed("account")) || strings.TrimSpace(f.account) != "" {
		cfg.DefaultAccount = strings.TrimSpace(f.account)
	}
}

func buildMeetingOptions(c config.Config, meetingURL string) browser.MeetingOptions {
	lobbyTimeout, _ := parseOptionalDuration(c.LobbyTimeout) // Validated beforehand by validateCaptureLifecycles; invalid values fall back to 0.
	aiSummaryOnly := strings.TrimSpace(c.AppendAISummary) != ""
	return browser.MeetingOptions{
		MeetingURL:       meetingURL,
		BotName:          c.BotName,
		DefaultAccount:   c.DefaultAccount,
		LobbyTimeout:     lobbyTimeout,
		ForceRecap:       strings.EqualFold(strings.TrimSpace(c.CaptureMode), config.CaptureModePastRecap) || aiSummaryOnly,
		AISummaryOnly:    aiSummaryOnly,
		DisableAISummary: isAISummaryDisabled(c),
	}
}

// defaultNewMeeting allocates a Chromium context for meeting capture and
// returns a chromedp-backed Meeting bound to it. The cleanup func cancels the
// browser and allocator contexts, terminating the spawned Chromium process.
func defaultNewMeeting(ctx context.Context, c config.Config, meetingURL string) (browser.Meeting, context.Context, func(), error) {
	return browser.NewLiveMeeting(ctx,
		browserOptions(c),
		buildMeetingOptions(c, meetingURL))
}

// NewRootCommand builds the root command with all subcommands attached, using
// the production dependency set.
func NewRootCommand() *cobra.Command {
	return newRootCommand(defaultDeps())
}

func newRootCommand(d *deps) *cobra.Command {
	var configPath string
	var flags captureFlags
	root := &cobra.Command{
		Use:   "go-teams [meeting-url] [output-markdown-path]",
		Short: "Autonomous Microsoft Teams live-transcription and meeting recap capture",
		Long: "go-teams captures live captions and Meeting Recap transcripts (including " +
			"the AI Summary) from Microsoft Teams into local Markdown files.\n\n" +
			"With a meeting URL (and optional output path) it captures directly; use a subcommand for\n" +
			"other tasks. Replace [command] with a real subcommand and run, for example,\n" +
			"go-teams join --help, go-teams captions --help, go-teams schedule --help,\n" +
			"go-teams auth --help, go-teams generate-config --help, or go-teams update --help.\n\n" +
			"Subcommands: join, captions, schedule, auth, update, generate-config.\n\n" +
			"Configuration file: pass --config (-f) or let go-teams use the first file that exists:\n" +
			"  1. ./config.yaml in the working directory\n" +
			"  2. config.yaml next to the executable\n" +
			"  3. the per-user config directory (%APPDATA%\\go-teams\\config.yaml on Windows,\n" +
			"     ~/.config/go-teams/config.yaml on Linux)\n" +
			"When none exists, built-in defaults apply and a visible browser window opens.\n\n" +
			captureHelpBody(),
		Example: "  go-teams \"https://teams.microsoft.com/meet/synthetic\" \"C:\\out\\transcript.md\" --headless\n" +
			"  go-teams \"https://teams.cloud.microsoft/l/meetingrecap?threadId=synthetic\" -o \"C:\\out\\recap.md\" --headless\n" +
			"  go-teams --queue transcript-queue.txt\n" +
			"  go-teams captions \"https://teams.microsoft.com/meet/synthetic\" -o \"C:\\out\\transcript.md\" --headless\n" +
			"  go-teams join --help",
		SilenceUsage: true,
		Version:      version.Version,
		Args:         cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			bindPromptStreams(cmd, d)
			if flags.queueFile != "" {
				if isInsideQueueExecution(cmd.Context()) {
					return errors.New("cli: nested --queue flag is not allowed")
				}
				return runQueueFile(cmd.Context(), d, flags.queueFile, inheritedQueueFlags(cmd, configPath, flags))
			}
			cfg, err := loadConfigFrom(d, configPath)
			if err != nil {
				return err
			}
			applyCaptureFlags(cmd, &cfg, flags)
			if len(args) >= 2 {
				applyOutputTarget(&cfg, args[1])
			}
			rawURL := cfg.MeetingURL
			if flags.meetingURL != "" {
				rawURL = flags.meetingURL
			}
			if len(args) >= 1 {
				rawURL = args[0]
			}
			if rawURL == "" {
				return cmd.Help()
			}
			mutated, err := teams.MutateJoinURL(rawURL)
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "f", "",
		"path to the YAML config file (default: ./config.yaml or user config directory)")
	bindCaptureFlags(root, &flags)
	root.AddCommand(
		newAuthCommand(d, &configPath),
		newJoinCommand(d, &configPath),
		newCaptionsCommand(d, &configPath),
		newScheduleCommand(d, &configPath),
		newUpdateCommand(d),
		newGenerateConfigCommand(d),
	)
	installRootHelpHandler(root)
	return root
}

// installRootHelpHandler makes the root help explain a mistyped or placeholder
// argument. When a positional argument that is neither a subcommand nor a valid
// Teams meeting URL reaches the root help (for example "go-teams [command]
// --help" or "go-teams help [command]"), a redacted one-line note precedes the
// normal root help. It also forwards the topic given to "help" so the root help
// function can see it; Cobra's default help command otherwise drops it. It must
// be called after all subcommands are attached.
func installRootHelpHandler(root *cobra.Command) {
	root.InitDefaultHelpCmd()
	for _, c := range root.Commands() {
		if c.Name() != "help" {
			continue
		}
		defaultRun := c.Run
		c.Run = func(hc *cobra.Command, args []string) {
			target, rest, err := root.Find(args)
			if err == nil && target == root && len(rest) > 0 {
				root.InitDefaultHelpFlag()
				root.InitDefaultVersionFlag()
				root.HelpFunc()(root, rest)
				return
			}
			defaultRun(hc, args)
		}
		break
	}

	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			posArgs := helpPositionalArgs(cmd, args)
			if len(posArgs) > 0 {
				if sub := findDirectSubcommand(root, posArgs[0]); sub != nil {
					// "go-teams --help join" renders the subcommand's own help.
					sub.InitDefaultHelpFlag()
					sub.HelpFunc()(sub, posArgs[1:])
					return
				}
			}
			if len(posArgs) > 0 && teams.ValidateMeetingURL(posArgs[0]) != nil {
				redacted := redactHelpArg(posArgs[0])
				// Informational help output only; a failed write to stdout is not actionable (Principle I).
				_, _ = fmt.Fprintf(cmd.OutOrStdout(),
					"Note: %q is not a subcommand or a valid Teams meeting URL. "+
						"Replace [command] with one of the subcommands below "+
						"(for example: go-teams join --help, go-teams captions --help, "+
						"go-teams schedule --help, go-teams auth --help, "+
						"go-teams generate-config --help, or go-teams update --help).\n\n",
					redacted)
			}
		}
		defaultHelp(cmd, args)
	})
}

// helpPositionalArgs returns the positional arguments seen by the root command.
// After flag parsing it uses the parsed positionals (so flag values such as the
// path after --config are not mistaken for arguments); otherwise it falls back
// to the non-flag tokens of args.
func helpPositionalArgs(cmd *cobra.Command, args []string) []string {
	if cmd.Flags().Parsed() {
		return cmd.Flags().Args()
	}
	var out []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			out = append(out, a)
			continue
		}
		if flagTakesSeparateValue(cmd, a) {
			i++ // skip the flag's value so it is not mistaken for a positional argument
		}
	}
	return out
}

// flagTakesSeparateValue reports whether token is a non-boolean flag written
// without an inline value ("-f", "--config"), meaning the next token is its value.
func flagTakesSeparateValue(cmd *cobra.Command, token string) bool {
	if token == "--" || strings.Contains(token, "=") {
		return false
	}
	var name string
	var lookup func(string) *pflag.Flag
	switch {
	case strings.HasPrefix(token, "--"):
		name = token[2:]
		lookup = func(n string) *pflag.Flag {
			if f := cmd.Flags().Lookup(n); f != nil {
				return f
			}
			return cmd.PersistentFlags().Lookup(n)
		}
	case len(token) == 2:
		name = token[1:]
		lookup = func(n string) *pflag.Flag {
			if f := cmd.Flags().ShorthandLookup(n); f != nil {
				return f
			}
			return cmd.PersistentFlags().ShorthandLookup(n)
		}
	default:
		return false
	}
	f := lookup(name)
	return f != nil && f.NoOptDefVal == ""
}

// findDirectSubcommand returns the direct subcommand of root named name (or
// carrying it as an alias), or nil when there is none.
func findDirectSubcommand(root *cobra.Command, name string) *cobra.Command {
	for _, sub := range root.Commands() {
		if sub.Name() == name || sub.HasAlias(name) {
			return sub
		}
	}
	return nil
}

// redactedHelpArg is echoed in place of a rejected argument that does not
// start with an http:// or https:// prefix and contains a credential-bearing
// delimiter.
const redactedHelpArg = "[redacted]"

// redactHelpArg redacts a rejected help argument for echo in the Note banner.
// The original argument is inspected first. Inputs with an http:// or
// https:// prefix (after trimming whitespace, matched case-insensitively) are
// delegated to teams.RedactArgs; this is prefix-based dispatch, not URL
// validation, so malformed http(s) inputs still reach RedactArgs and any
// retained host/path text is what RedactURL keeps. Any other input containing
// a credential-bearing delimiter (userinfo "@", query "?", fragment "#", a
// scheme separator "://", or an embedded http:/https: token) is replaced with
// redactedHelpArg entirely, because a passcode or password may itself contain
// those delimiters and partial truncation could still echo a secret prefix.
func redactHelpArg(raw string) string {
	lower := strings.ToLower(strings.TrimSpace(raw))
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return teams.RedactArgs([]string{raw})
	}
	if strings.ContainsAny(raw, "@?#") || strings.Contains(lower, "://") ||
		strings.Contains(lower, "http:") || strings.Contains(lower, "https:") {
		return redactedHelpArg
	}
	return raw
}

// loadConfigFrom resolves the effective config path (falling back to the
// default location) and loads it via the injected loader. When no --config path
// was explicitly specified and the default config file does not exist on disk,
// it falls back to DefaultConfig with Headless=false so the CLI opens a visible
// browser window for interactive user authentication out of the box.
func loadConfigFrom(d *deps, configPath string) (config.Config, error) {
	path := configPath
	if path == "" {
		p, err := resolveDefaultConfigPath()
		if err != nil {
			return config.Config{}, err
		}
		path = p
	}
	cfg, err := d.loadConfig(path)
	if err != nil {
		if configPath == "" && errors.Is(err, os.ErrNotExist) {
			def, defErr := config.DefaultConfig()
			if defErr != nil {
				return config.Config{}, defErr
			}
			def.Headless = false
			def.OutputDir = "transcripts"
			return def, nil
		}
		return config.Config{}, err
	}
	if configPath != "" {
		cfg.ConfigPath = configPath
	}
	return cfg, nil
}

func resolveDefaultConfigPath() (string, error) {
	candidates := []string{"config.yaml"}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(exeDir, "config.yaml"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c, nil
		}
	}
	return config.DefaultConfigPath()
}
