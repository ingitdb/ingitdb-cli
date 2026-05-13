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

			// Remove data directory (which transitively removes any
			// nested $views/ subtree).
			absCol := filepath.Join(dirPath, rel)
			if rmErr := os.RemoveAll(absCol); rmErr != nil {
				return fmt.Errorf("remove collection directory %s: %w", rel, rmErr)
			}

			// Remove entry from root-collections.yaml.
			if writeErr := writeRootCollectionsWithout(dirPath, name); writeErr != nil {
				return fmt.Errorf("update root-collections.yaml after removing %s: %w", name, writeErr)
			}
			return nil
		},
	}
	return cmd
}

// dropView returns the `drop view <name>` subcommand.
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
			name := args[0]

			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}

			// Scan every collection's $views directory for a matching
			// view file.
			entries, err := readRootCollections(dirPath)
			if err != nil {
				return err
			}
			for _, rel := range entries {
				viewPath := filepath.Join(dirPath, rel, "$views", name+".yaml")
				if _, statErr := os.Stat(viewPath); statErr != nil {
					continue
				}
				// Found the view. Read its file_name (if any) to
				// also remove the materialized output.
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
					outputPath := filepath.Join(dirPath, rel, meta.FileName)
					if rmErr := os.Remove(outputPath); rmErr != nil && !os.IsNotExist(rmErr) {
						return fmt.Errorf("remove materialized output %s: %w", outputPath, rmErr)
					}
				}
				return nil
			}

			// View not found in any collection.
			if ifExists {
				return nil
			}
			return fmt.Errorf("view %q not found in any collection", name)
		},
	}
	return cmd
}
