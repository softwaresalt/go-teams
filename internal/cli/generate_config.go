package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/softwaresalt/go-teams/internal/config"
)

// newGenerateConfigCommand builds the generate-config subcommand, which writes
// a fully commented YAML configuration template. It deliberately ignores the
// root --config flag and never loads a configuration file.
func newGenerateConfigCommand(d *deps) *cobra.Command {
	writeTemplate := config.WriteTemplate
	if d != nil && d.writeTemplate != nil {
		writeTemplate = d.writeTemplate
	}
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
			dest := destination{dir: outputDir, name: name}
			if len(args) == 1 {
				dest = applyPositionalDestination(cmd, strings.TrimSpace(args[0]), outputDir, name)
			}
			destPath, err := dest.resolve()
			if err != nil {
				return err
			}
			if err := writeTemplate(destPath, force); err != nil {
				if errors.Is(err, config.ErrConfigExists) {
					return fmt.Errorf("%w (use --force to overwrite)", err)
				}
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

// destination is the directory and filename a generated configuration file is
// written to. When dirExpanded is true, dir has already been expanded and must
// not be expanded again (a literal "$" in an expanded path is not a variable).
type destination struct {
	dir         string
	name        string
	dirExpanded bool
}

// resolve returns the validated destination path. An already-expanded
// directory is joined directly so it is never expanded a second time.
func (d destination) resolve() (string, error) {
	if !d.dirExpanded {
		return config.ResolveTemplatePath(d.dir, d.name)
	}
	// Validate the filename only; "." is a no-op expansion target.
	validated, err := config.ResolveTemplatePath(".", d.name)
	if err != nil {
		return "", err
	}
	return filepath.Join(d.dir, filepath.Base(validated)), nil
}

// hasYAMLExtension reports whether path ends in .yaml or .yml (any case).
func hasYAMLExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

// applyPositionalDestination folds the optional positional argument into the
// destination. An explicit --output-dir always wins. When neither --output-dir
// nor --name was given, an argument that (after variable expansion) ends in
// .yaml or .yml and is not an existing directory names the file itself.
func applyPositionalDestination(cmd *cobra.Command, rawArg, outputDir, name string) destination {
	if rawArg == "" || cmd.Flags().Changed("output-dir") {
		return destination{dir: outputDir, name: name}
	}
	if cmd.Flags().Changed("name") {
		return destination{dir: rawArg, name: name}
	}
	expanded := config.ExpandPath(rawArg)
	if !hasYAMLExtension(expanded) {
		return destination{dir: expanded, name: name, dirExpanded: true}
	}
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		return destination{dir: expanded, name: name, dirExpanded: true}
	}
	return destination{dir: filepath.Dir(expanded), name: filepath.Base(expanded), dirExpanded: true}
}
