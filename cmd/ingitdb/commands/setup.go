package commands

// specscore: feature/cli/setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// Setup returns the setup command.
func Setup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up a new inGitDB database",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("path")
			if path == "" {
				path = "."
			}
			defaultFormat, _ := cmd.Flags().GetString("default-format")
			return runSetup(path, defaultFormat)
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("default-format", "",
		"project-level default record format (one of: yaml, yml, json, markdown, toml, ingr, csv)")
	return cmd
}

// runSetup creates .ingitdb/ and writes settings.yaml at the given path.
// When defaultFormatFlag is non-empty, the value is validated against
// the seven supported record formats and written to
// settings.yaml#default_record_format. When empty, the file is created
// with no default_record_format key.
//
// This is the minimum write path the --default-format flag needs. The
// broader setup behavior (idempotency on already-initialised directory,
// default-namespace prompts, root-collections seed) is governed by the
// existing setup Feature spec and is intentionally out of scope here.
func runSetup(path, defaultFormatFlag string) error {
	settings := config.Settings{}
	if defaultFormatFlag != "" {
		f, err := parseDefaultFormat(defaultFormatFlag)
		if err != nil {
			return err
		}
		settings.DefaultRecordFormat = f
	}
	if validateErr := settings.Validate(); validateErr != nil {
		return validateErr
	}
	configDir := filepath.Join(path, config.IngitDBDirName)
	if mkErr := os.MkdirAll(configDir, 0o755); mkErr != nil {
		return fmt.Errorf("failed to create %s directory: %w", configDir, mkErr)
	}
	out, marshalErr := yaml.Marshal(settings)
	if marshalErr != nil {
		return fmt.Errorf("failed to marshal settings: %w", marshalErr)
	}
	settingsPath := filepath.Join(configDir, config.SettingsFileName)
	if writeErr := os.WriteFile(settingsPath, out, 0o644); writeErr != nil {
		return fmt.Errorf("failed to write %s: %w", settingsPath, writeErr)
	}
	return nil
}

// parseDefaultFormat validates the --default-format flag value against
// the seven supported record formats. Comparison is case-sensitive (the
// canonical forms are lowercase); the error message names the offending
// value and lists all seven valid options.
func parseDefaultFormat(raw string) (ingitdb.RecordFormat, error) {
	valid := []ingitdb.RecordFormat{
		ingitdb.RecordFormatYAML,
		ingitdb.RecordFormatYML,
		ingitdb.RecordFormatJSON,
		ingitdb.RecordFormatMarkdown,
		ingitdb.RecordFormatTOML,
		ingitdb.RecordFormatINGR,
		ingitdb.RecordFormatCSV,
	}
	for _, f := range valid {
		if string(f) == raw {
			return f, nil
		}
	}
	names := make([]string, len(valid))
	for i, f := range valid {
		names[i] = string(f)
	}
	return "", fmt.Errorf("unsupported --default-format=%q; valid options are: %s",
		raw, joinComma(names))
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}
