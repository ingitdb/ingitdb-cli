package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/datavalidator"
)

// Validate returns the validate command.
func Validate(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	dataVal datavalidator.DataValidator,
	incVal datavalidator.IncrementalValidator,
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate an inGitDB database directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			dirPath, _ := cmd.Flags().GetString("path")
			if dirPath == "" {
				wd, err := getWd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				dirPath = wd
			}
			expanded, err := expandHome(dirPath, homeDir)
			if err != nil {
				return err
			}
			dirPath = expanded
			logf("inGitDB db path: ", dirPath)

			// Validate --only flag
			onlyVal, _ := cmd.Flags().GetString("only")
			if onlyVal != "" && onlyVal != "definition" && onlyVal != "records" {
				return fmt.Errorf("invalid --only value: %q (must be \"definition\", \"records\", or empty)", onlyVal)
			}

			fromCommit, _ := cmd.Flags().GetString("from-commit")
			toCommit, _ := cmd.Flags().GetString("to-commit")

			if fromCommit != "" || toCommit != "" {
				if incVal == nil {
					return fmt.Errorf("incremental validation (--from-commit/--to-commit) is not yet implemented")
				}
				def, defErr := readDefinition(dirPath)
				if defErr != nil {
					return fmt.Errorf("failed to read database definition: %w", defErr)
				}
				result, valErr := incVal.ValidateChanges(ctx, dirPath, def, fromCommit, toCommit)
				if valErr != nil {
					return fmt.Errorf("incremental validation failed: %w", valErr)
				}
				if result.HasErrors() {
					errCount := result.ErrorCount()
					return fmt.Errorf("incremental validation found %d error(s)", errCount)
				}
				return nil
			}

			// Determine which validations to perform
			shouldValidateDef := onlyVal != "records"
			shouldValidateRecords := onlyVal != "definition"

			// Read definition (with validation if needed)
			var def *ingitdb.Definition
			if shouldValidateDef {
				validateOpt := ingitdb.Validate()
				defRes, defErr := readDefinition(dirPath, validateOpt)
				if defErr != nil {
					return fmt.Errorf("inGitDB database validation failed: %w", defErr)
				}
				def = defRes
			} else {
				defRes, defErr := readDefinition(dirPath)
				if defErr != nil {
					return fmt.Errorf("inGitDB database validation failed: %w", defErr)
				}
				def = defRes
			}

			// Validate records if needed
			if shouldValidateRecords && dataVal != nil {
				result, valErr := dataVal.Validate(ctx, dirPath, def)
				if valErr != nil {
					return fmt.Errorf("data validation failed: %w", valErr)
				}
				if result.HasErrors() {
					errCount := result.ErrorCount()
					return fmt.Errorf("data validation found %d error(s)", errCount)
				}
				// Log completion message for each collection
				for collectionKey := range def.Collections {
					passed, total := result.GetRecordCounts(collectionKey)
					if passed == total {
						logf(fmt.Sprintf("All %d records are valid for collection: %s", total, collectionKey))
					} else {
						logf(fmt.Sprintf("%d out of %d records are valid for collection: %s", passed, total, collectionKey))
					}
				}
			}
			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("from-commit", "", "validate only records changed since this commit")
	cmd.Flags().String("to-commit", "", "validate only records up to this commit")
	cmd.Flags().String("only", "", `validate only "definition" or "records" (default: both)`)
	return cmd
}

func expandHome(path string, homeDir func() (string, error)) (string, error) {
	if strings.HasPrefix(path, "~") {
		home, err := homeDir()
		if err != nil {
			return "", fmt.Errorf("failed to expand home directory: %w", err)
		}
		return filepath.Join(home, path[1:]), nil
	}
	return path, nil
}

