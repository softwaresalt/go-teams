package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
)

// probeFileName is a throwaway filename used only to reuse
// config.ResolveTemplatePath's path expansion for a positional argument.
const probeFileName = "probe.yaml"

// newGenerateConfigCommand builds the generate-config subcommand, which writes
// a fully commented YAML configuration template. It deliberately ignores the
// root --config flag and never loads a configuration file.
func newGenerateConfigCommand(_ *deps) *cobra.Command {
	var (
		outputDir string
		name      string
		force     bool
	)
	cmd := &cobra.Command{
		Use:     "generate-config [output-dir]",
		Aliases: []string{"init-config", "gen-config"},
		Short:   "Generate a commented YAML configuration file template",
		Long: "generate-config writes a fully commented YAML configuration file in which every\n" +
			"option is commented out and explained in plain language. Remove the leading \"# \"\n" +
			"from a setting to turn it on. The file defaults to ./config.yaml.\n\n" +
			"The destination is controlled only by --output-dir (-d), --name (-n), and the\n" +
			"optional [output-dir] argument; the root --config / -f flag is not read. An\n" +
			"[output-dir] argument ending in .yaml or .yml that is not an existing directory\n" +
			"is treated as the full file path. An existing file is never overwritten unless\n" +
			"--force is set.\n\n" +
			"Examples:\n" +
			"  teams-cli generate-config\n" +
			"  teams-cli generate-config C:\\teams\\team.yaml\n" +
			"  teams-cli generate-config -d \"%LOCALAPPDATA%\\teams-cli\" --force",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, fileName := outputDir, name
			if len(args) == 1 {
				dir, fileName = applyPositionalDestination(cmd, strings.TrimSpace(args[0]), outputDir, name)
			}
			destPath, err := config.ResolveTemplatePath(dir, fileName)
			if err != nil {
				return err
			}
			if err := config.WriteTemplate(destPath, force); err != nil {
				return err
			}
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), destPath) // informational stdout output after template is persisted
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputDir, "output-dir", "d", ".", "directory for the generated configuration file")
	cmd.Flags().StringVarP(&name, "name", "n", "config.yaml", "configuration filename to generate inside the target directory")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing configuration file at the destination")
	return cmd
}

// applyPositionalDestination folds the optional positional argument into the
// directory and filename. An explicit --output-dir always wins. When neither
// --output-dir nor --name was given, an argument ending in .yaml or .yml that
// is not an existing directory names the file itself.
func applyPositionalDestination(cmd *cobra.Command, arg, outputDir, name string) (string, string) {
	if arg == "" || cmd.Flags().Changed("output-dir") {
		return outputDir, name
	}
	ext := strings.ToLower(filepath.Ext(arg))
	if (ext == ".yaml" || ext == ".yml") && !cmd.Flags().Changed("name") {
		expanded := expandPositional(arg)
		if info, err := os.Stat(expanded); err == nil && info.IsDir() {
			return arg, name
		}
		return filepath.Dir(expanded), filepath.Base(expanded)
	}
	return arg, name
}

// expandPositional expands user and environment path prefixes in arg using the
// same rules as configuration paths.
func expandPositional(arg string) string {
	probe, err := config.ResolveTemplatePath(arg, probeFileName)
	if err != nil {
		return arg
	}
	return filepath.Dir(probe)
}
