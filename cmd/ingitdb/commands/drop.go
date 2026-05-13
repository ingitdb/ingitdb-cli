package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dal-go/dalgo/dal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Drop returns the `ingitdb drop` command. Two kinds are supported:
// `drop collection <name>` and `drop view <name>`. The flags
// --if-exists (idempotent on missing target) and --cascade (no-op in
// the current data model; reserved for future cross-object dependency
// graphs) are inherited by both subcommands.
func Drop(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drop <kind> <name>",
		Short: "Drop a schema object (collection or view)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("drop requires a kind: collection or view")
		},
	}
	cmd.PersistentFlags().String("path", "", "path to the database directory (default: current directory)")
	cmd.PersistentFlags().String("remote", "",
		"remote repository, e.g. github.com/owner/repo[@branch|tag|commit] "+
			"(mutually exclusive with --path)")
	cmd.PersistentFlags().String("token", "",
		"personal access token; falls back to host-derived env vars "+
			"(e.g. GITHUB_TOKEN for github.com)")
	cmd.PersistentFlags().String("provider", "",
		"explicit provider id (github, gitlab, bitbucket) — required for unknown hosts")
	cmd.PersistentFlags().Bool("if-exists", false, "do not fail when the target does not exist")
	cmd.PersistentFlags().Bool("cascade", false, "drop dependents along with the target (no-op in the current data model)")

	cmd.AddCommand(
		dropCollection(homeDir, getWd, readDefinition, newDB, logf),
		dropView(homeDir, getWd, readDefinition, newDB, logf),
	)
	return cmd
}

// dropCollection returns the `drop collection <name>` subcommand.
func dropCollection(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection <name>",
		Short: "Drop a collection (removes schema entry + data directory)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _ = readDefinition, newDB, logf
			ifExists, _ := cmd.Flags().GetBool("if-exists")
			_, _ = cmd.Flags().GetBool("cascade") // accepted, no-op
			name := args[0]

			remoteVal, _ := cmd.Flags().GetString("remote")
			pathVal, _ := cmd.Flags().GetString("path")
			if remoteVal != "" && pathVal != "" {
				return fmt.Errorf("--path and --remote are mutually exclusive")
			}
			if remoteVal != "" {
				return dropCollectionRemote(cmd.Context(), cmd, name, ifExists)
			}

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			entries, err := readRootCollections(dirPath)
			if err != nil {
				return err
			}
			rel, ok := entries[name]
			if !ok {
				if ifExists {
					return nil
				}
				return fmt.Errorf("collection %q not found", name)
			}

			absCol := filepath.Join(dirPath, rel)
			if rmErr := os.RemoveAll(absCol); rmErr != nil {
				return fmt.Errorf("remove collection directory %s: %w", rel, rmErr)
			}
			if writeErr := writeRootCollectionsWithout(dirPath, name); writeErr != nil {
				return fmt.Errorf("update root-collections.yaml after removing %s: %w", name, writeErr)
			}
			return nil
		},
	}
	return cmd
}

// dropView returns the `drop view <name>` subcommand. Supports
// --in=<collection> to disambiguate when two collections shadow the
// same view name.
func dropView(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Drop a view (removes view definition + materialized output)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, _ = readDefinition, newDB, logf
			ifExists, _ := cmd.Flags().GetBool("if-exists")
			_, _ = cmd.Flags().GetBool("cascade") // accepted, no-op
			scopeCol, _ := cmd.Flags().GetString("in")
			name := args[0]

			remoteVal, _ := cmd.Flags().GetString("remote")
			pathVal, _ := cmd.Flags().GetString("path")
			if remoteVal != "" && pathVal != "" {
				return fmt.Errorf("--path and --remote are mutually exclusive")
			}
			if remoteVal != "" {
				return dropViewRemote(cmd.Context(), cmd, name, scopeCol, ifExists)
			}

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			entries, err := readRootCollections(dirPath)
			if err != nil {
				return err
			}
			if scopeCol != "" {
				rel, ok := entries[scopeCol]
				if !ok {
					return fmt.Errorf("collection %q (from --in) not found", scopeCol)
				}
				entries = map[string]string{scopeCol: rel}
			}

			type viewMatch struct {
				collection string
				viewPath   string
				colDir     string
			}
			var matches []viewMatch
			for colID, rel := range entries {
				viewPath := filepath.Join(dirPath, rel, "$views", name+".yaml")
				if _, statErr := os.Stat(viewPath); statErr == nil {
					matches = append(matches, viewMatch{
						collection: colID,
						viewPath:   viewPath,
						colDir:     filepath.Join(dirPath, rel),
					})
				}
			}

			switch len(matches) {
			case 0:
				if ifExists {
					return nil
				}
				if scopeCol != "" {
					return fmt.Errorf("view %q not found in collection %q", name, scopeCol)
				}
				return fmt.Errorf("view %q not found in any collection", name)
			case 1:
				return removeViewFiles(matches[0].viewPath, matches[0].colDir)
			default:
				cols := make([]string, 0, len(matches))
				for _, m := range matches {
					cols = append(cols, m.collection)
				}
				return fmt.Errorf("view %q is ambiguous — exists in multiple collections: %v; use --in=<collection> to disambiguate", name, cols)
			}
		},
	}
	cmd.Flags().String("in", "", "limit the search to a specific collection (disambiguates duplicate view names)")
	return cmd
}

// removeViewFiles removes the view-definition file and, if the view
// declares a materialized output via `file_name`, removes that output
// too. Missing output files are tolerated; the goal is to leave no
// trace of the view after the call.
func removeViewFiles(viewPath, colDir string) error {
	rawView, readErr := os.ReadFile(viewPath)
	if readErr != nil {
		return fmt.Errorf("read view file %s: %w", viewPath, readErr)
	}
	var meta struct {
		FileName string `yaml:"file_name"`
	}
	_ = yaml.Unmarshal(rawView, &meta)

	if rmErr := os.Remove(viewPath); rmErr != nil {
		return fmt.Errorf("remove view file %s: %w", viewPath, rmErr)
	}
	if meta.FileName != "" {
		outputPath := filepath.Join(colDir, meta.FileName)
		if rmErr := os.Remove(outputPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("remove materialized output %s: %w", outputPath, rmErr)
		}
	}
	return nil
}
