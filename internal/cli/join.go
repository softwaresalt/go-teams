package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
	"github.com/softwaresalt/go-teams/internal/teams"
)

const defaultTeamsCaptureURL = "https://teams.microsoft.com/v2/"

type captureFlags struct {
	meetingURL       string
	queueFile        string
	output           string
	outputFile       string
	outputDir        string
	outputFolder     string
	filenameTemplate string
	nameTemplate     string
	profileDir       string
	botName          string
	category         string
	browserPath      string
	account          string
	headless         bool
	headed           bool
	audio            bool
	mode             string
	duration         time.Duration
	idleTimeout      time.Duration
	aiSummaryDelay   time.Duration
	noAISummary      bool
	lobbyTimeout     time.Duration
	appendAISummary  string
	notBefore        string
}

func bindCaptureFlags(cmd *cobra.Command, f *captureFlags) {
	cmd.Flags().StringVarP(&f.meetingURL, "url", "u", "", "Teams meeting or recap URL to capture")
	cmd.Flags().StringVarP(&f.queueFile, "queue", "q", "", "path to a batch queue file of Teams capture arguments")
	cmd.Flags().StringVarP(&f.output, "output", "o", "", "output Markdown file path (.md) or directory")
	cmd.Flags().StringVar(&f.outputFile, "output-file", "", "explicit output Markdown file path (.md)")
	cmd.Flags().StringVar(&f.outputFolder, "output-folder", "", "default folder where Markdown transcripts are saved")
	cmd.Flags().StringVar(&f.outputDir, "output-dir", "", "directory where the Markdown transcript is saved")
	cmd.Flags().StringVar(&f.filenameTemplate, "filename-template", "", "filename template when writing to a directory (default: <category>-<concat-meeting-name>-transcript-<YYYYMMDD>.md)")
	cmd.Flags().StringVar(&f.nameTemplate, "name-template", "", "alias for --filename-template")
	cmd.Flags().StringVar(&f.profileDir, "profile-dir", "", "browser user-data directory for session persistence")
	cmd.Flags().StringVar(&f.botName, "bot-name", "", "display name injected on guest pre-join")
	cmd.Flags().StringVarP(&f.category, "category", "c", "", "optional meeting category for the transcript header and filename")
	cmd.Flags().StringVar(&f.browserPath, "browser-path", "", "path to Chrome/Edge/Chromium executable")
	cmd.Flags().StringVar(&f.account, "account", "", "default Microsoft Entra account (UPN/email or display name) to auto-select")
	cmd.Flags().BoolVar(&f.headless, "headless", false, "run browser in headless mode")
	cmd.Flags().BoolVar(&f.headed, "headed", false, "open a visible browser window for interactive authentication")
	cmd.Flags().BoolVar(&f.audio, "audio", false, "also capture meeting or recap audio to an MP3 file")
	// --recap-audio is an alias that shares state with --audio so the argument
	// that appears last wins, including for per-entry queue overrides.
	cmd.Flags().BoolVar(&f.audio, "recap-audio", false, "also capture Meeting Recap audio to an MP3 file")
	cmd.Flags().StringVar(&f.mode, "mode", "", "meeting capture protocol: auto, ephemeral-live, live-with-summary, or past-recap")
	cmd.Flags().DurationVar(&f.duration, "duration", 0, "nominal live capture duration (e.g. 30m, 1h) before post-duration idle monitoring")
	cmd.Flags().DurationVar(&f.idleTimeout, "idle-timeout", 0, "inactivity grace period after duration or call-end before stopping capture (default 3m)")
	cmd.Flags().DurationVar(&f.aiSummaryDelay, "ai-summary-delay", 0, "delay after a live call ends before fetching and appending the AI Summary (default 30m)")
	cmd.Flags().BoolVar(&f.noAISummary, "no-ai-summary", false, "disable delayed post-call AI Summary capture")
	cmd.Flags().DurationVar(&f.lobbyTimeout, "lobby-timeout", 0, "maximum time to wait in pre-join or lobby before aborting")
	cmd.Flags().StringVar(&f.appendAISummary, "append-ai-summary", "", "existing Markdown transcript file to append the Meeting Recap AI Summary into")
	cmd.Flags().StringVar(&f.notBefore, "not-before", "", "RFC3339 timestamp before which a queued --append-ai-summary entry is skipped")
}

func applyOutputTarget(cfg *config.Config, target string) {
	if target == "" {
		return
	}
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		cfg.OutputDir = target
		cfg.OutputFile = ""
		return
	}
	ext := strings.ToLower(filepath.Ext(target))
	if ext == ".md" || ext == ".markdown" {
		cfg.OutputFile = target
		return
	}
	cfg.OutputDir = target
	cfg.OutputFile = ""
}

func applyCaptureFlags(cmd *cobra.Command, cfg *config.Config, f captureFlags) {
	if f.outputFolder != "" {
		cfg.OutputDir = f.outputFolder
		cfg.OutputFile = ""
	}
	if f.outputDir != "" {
		cfg.OutputDir = f.outputDir
		cfg.OutputFile = ""
	}
	if f.output != "" {
		applyOutputTarget(cfg, f.output)
	}
	if f.outputFile != "" {
		cfg.OutputFile = f.outputFile
	}
	if f.nameTemplate != "" {
		cfg.FilenameTemplate = f.nameTemplate
	}
	if f.filenameTemplate != "" {
		cfg.FilenameTemplate = f.filenameTemplate
	}
	if f.profileDir != "" {
		cfg.ProfileDir = f.profileDir
	}
	if f.botName != "" {
		cfg.BotName = f.botName
	}
	if f.category != "" {
		cfg.Category = f.category
	}
	if f.browserPath != "" {
		cfg.BrowserPath = f.browserPath
	}
	if cmd.Flags().Changed("account") || strings.TrimSpace(f.account) != "" {
		cfg.DefaultAccount = strings.TrimSpace(f.account)
	}
	if cmd.Flags().Changed("headless") {
		cfg.Headless = f.headless
	}
	if f.headed {
		cfg.Headless = false
	}
	audioFlagChanged := cmd.Flags().Changed("audio") || cmd.Flags().Changed("recap-audio")
	if audioFlagChanged {
		cfg.CaptureAudio = f.audio
	}
	if cmd.Flags().Changed("mode") || strings.TrimSpace(f.mode) != "" {
		cfg.CaptureMode = strings.ToLower(strings.TrimSpace(f.mode))
	}
	if cmd.Flags().Changed("duration") || f.duration > 0 {
		cfg.Duration = f.duration.String()
	}
	if cmd.Flags().Changed("idle-timeout") || f.idleTimeout > 0 {
		cfg.IdleTimeout = f.idleTimeout.String()
	}
	if cmd.Flags().Changed("ai-summary-delay") {
		cfg.AISummaryDelay = f.aiSummaryDelay.String()
	}
	if f.noAISummary {
		cfg.AISummaryDelay = "0s"
	}
	if cmd.Flags().Changed("lobby-timeout") || f.lobbyTimeout > 0 {
		cfg.LobbyTimeout = f.lobbyTimeout.String()
	}
	if cmd.Flags().Changed("append-ai-summary") || strings.TrimSpace(f.appendAISummary) != "" {
		cfg.AppendAISummary = strings.TrimSpace(f.appendAISummary)
	}
	if cmd.Flags().Changed("not-before") || strings.TrimSpace(f.notBefore) != "" {
		cfg.NotBefore = strings.TrimSpace(f.notBefore)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.CaptureMode), config.CaptureModeEphemeralLive) {
		cfg.AISummaryDelay = "0s"
		if !audioFlagChanged && (cmd.Flags().Changed("mode") || strings.TrimSpace(f.mode) != "") {
			cfg.CaptureAudio = true
		}
	}
}

func inheritedQueueFlags(cmd *cobra.Command, configPath string, f captureFlags) []string {
	var out []string
	if configPath != "" {
		out = append(out, "--config", configPath)
	}
	if f.outputFolder != "" {
		out = append(out, "--output-folder", f.outputFolder)
	}
	if f.outputDir != "" {
		out = append(out, "--output-dir", f.outputDir)
	}
	if f.output != "" {
		out = append(out, "--output", f.output)
	}
	if f.outputFile != "" {
		out = append(out, "--output-file", f.outputFile)
	}
	if f.filenameTemplate != "" {
		out = append(out, "--filename-template", f.filenameTemplate)
	} else if f.nameTemplate != "" {
		out = append(out, "--name-template", f.nameTemplate)
	}
	if f.profileDir != "" {
		out = append(out, "--profile-dir", f.profileDir)
	}
	if f.botName != "" {
		out = append(out, "--bot-name", f.botName)
	}
	if f.category != "" {
		out = append(out, "--category", f.category)
	}
	if f.browserPath != "" {
		out = append(out, "--browser-path", f.browserPath)
	}
	if cmd.Flags().Changed("account") {
		if trimmed := strings.TrimSpace(f.account); trimmed != "" {
			out = append(out, "--account", trimmed)
		} else {
			out = append(out, "--account=")
		}
	} else if strings.TrimSpace(f.account) != "" {
		out = append(out, "--account", strings.TrimSpace(f.account))
	}
	if cmd.Flags().Changed("headless") {
		if f.headless {
			out = append(out, "--headless")
		} else {
			out = append(out, "--headless=false")
		}
	}
	if f.headed {
		out = append(out, "--headed")
	}
	// Both flags share f.audio, so forward the resolved value under each flag
	// that was set; the values agree, so their order cannot matter downstream.
	for _, name := range []string{"audio", "recap-audio"} {
		if !cmd.Flags().Changed(name) {
			continue
		}
		if f.audio {
			out = append(out, "--"+name)
		} else {
			out = append(out, "--"+name+"=false")
		}
	}
	if cmd.Flags().Changed("mode") || strings.TrimSpace(f.mode) != "" {
		out = append(out, "--mode", strings.ToLower(strings.TrimSpace(f.mode)))
	}
	if cmd.Flags().Changed("duration") || f.duration > 0 {
		out = append(out, "--duration", f.duration.String())
	}
	if cmd.Flags().Changed("idle-timeout") || f.idleTimeout > 0 {
		out = append(out, "--idle-timeout", f.idleTimeout.String())
	}
	if cmd.Flags().Changed("ai-summary-delay") {
		out = append(out, "--ai-summary-delay", f.aiSummaryDelay.String())
	}
	if cmd.Flags().Changed("no-ai-summary") && f.noAISummary {
		out = append(out, "--no-ai-summary")
	}
	if cmd.Flags().Changed("lobby-timeout") || f.lobbyTimeout > 0 {
		out = append(out, "--lobby-timeout", f.lobbyTimeout.String())
	}
	return out
}

func newJoinCommand(d *deps, configPath *string) *cobra.Command {
	var flags captureFlags
	cmd := &cobra.Command{
		Use:   "join <meeting-url> [output-markdown-path]",
		Short: "Join a Teams meeting and capture its live captions to a Markdown file",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.queueFile != "" {
				if isInsideQueueExecution(cmd.Context()) {
					return fmt.Errorf("cli: nested --queue flag is not allowed")
				}
				return runQueueFile(cmd.Context(), d, flags.queueFile, inheritedQueueFlags(cmd, *configPath, flags))
			}
			cfg, err := loadConfigFrom(d, *configPath)
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
				return fmt.Errorf("cli: join requires a <meeting-url> argument, --url flag, or meeting_url in config")
			}
			mutated, err := teams.MutateJoinURL(rawURL)
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	bindCaptureFlags(cmd, &flags)
	return cmd
}

func newCaptionsCommand(d *deps, configPath *string) *cobra.Command {
	var flags captureFlags
	cmd := &cobra.Command{
		Use:   "captions [meeting-url] [output-markdown-path]",
		Short: "Capture Teams live captions or Meeting Recap transcript + AI Summary to Markdown",
		Long: "captions opens a browser (headed by default, or headless with --headless), " +
			"navigates to the provided Teams meeting or Meeting Recap URL, and writes " +
			"the captured transcript and AI Summary to the specified Markdown file or output directory.",
		Args: cobra.MaximumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.queueFile != "" {
				if isInsideQueueExecution(cmd.Context()) {
					return fmt.Errorf("cli: nested --queue flag is not allowed")
				}
				return runQueueFile(cmd.Context(), d, flags.queueFile, inheritedQueueFlags(cmd, *configPath, flags))
			}
			cfg, err := loadConfigFrom(d, *configPath)
			if err != nil {
				return err
			}
			applyCaptureFlags(cmd, &cfg, flags)
			rawURL := defaultTeamsCaptureURL
			if cfg.MeetingURL != "" {
				rawURL = cfg.MeetingURL
			}
			if flags.meetingURL != "" {
				rawURL = flags.meetingURL
			}
			if len(args) >= 1 {
				rawURL = args[0]
			}
			if len(args) >= 2 {
				applyOutputTarget(&cfg, args[1])
			}
			mutated, err := teams.MutateJoinURL(rawURL)
			if err != nil {
				return err
			}
			return runCapture(cmd.Context(), d, cfg, mutated)
		},
	}
	bindCaptureFlags(cmd, &flags)
	return cmd
}
