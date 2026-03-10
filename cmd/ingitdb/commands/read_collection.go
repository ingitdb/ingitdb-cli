package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func readCollection(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collection",
		Short: "Output the definition YAML of a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dirPath, err := resolveDBPath(cmd, homeDir, getWd)
			if err != nil {
				return err
			}
			_ = logf

			def, err := readDefinition(dirPath)
			if err != nil {
				return fmt.Errorf("failed to read database definition: %w", err)
			}

			colID, _ := cmd.Flags().GetString("collection")
			colDef := def.Collections[colID]
			if colDef == nil {
				return fmt.Errorf("collection %q not found", colID)
			}

			defPath := filepath.Join(colDef.DirPath, ingitdb.SchemaDir, colDef.ID+".yaml")
			content, readErr := os.ReadFile(defPath)
			if readErr != nil {
				return fmt.Errorf("failed to read collection definition file: %w", readErr)
			}
			_, _ = os.Stdout.Write(content)
			return nil
		},
	}
	addPathFlag(cmd)
	addCollectionFlag(cmd, true)
	return cmd
}

