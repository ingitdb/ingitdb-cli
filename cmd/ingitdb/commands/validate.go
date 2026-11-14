package commands

// specscore: feature/cli/validate

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
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
			safeDiagnostics, _ := cmd.Flags().GetBool("safe-diagnostics")
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
			if safeDiagnostics {
				logf("inGitDB database root: repository-relative path")
			} else {
				logf("inGitDB db path: ", dirPath)
			}

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
					message := formatValidationFailure("incremental validation", result)
					if safeDiagnostics {
						message = formatSafeValidationFailure("incremental validation", dirPath, result)
					}
					findingErr := fmt.Errorf("%s", message)
					return NewValidationFailedError(findingErr)
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
					if safeDiagnostics {
						validationErr := fmt.Errorf("inGitDB database definition is invalid")
						return NewValidationFailedError(validationErr)
					}
					validationErr := fmt.Errorf("inGitDB database validation failed: %w", defErr)
					return NewValidationFailedError(validationErr)
				}
				def = defRes
			} else {
				defRes, defErr := readDefinition(dirPath)
				if defErr != nil {
					if safeDiagnostics {
						validationErr := fmt.Errorf("inGitDB database definition is invalid")
						return NewValidationFailedError(validationErr)
					}
					validationErr := fmt.Errorf("inGitDB database validation failed: %w", defErr)
					return NewValidationFailedError(validationErr)
				}
				def = defRes
			}

			// Validate records if needed
			if shouldValidateRecords && dataVal != nil {
				result, valErr := dataVal.Validate(ctx, dirPath, def)
				if valErr != nil {
					if safeDiagnostics {
						return fmt.Errorf("data validator could not complete")
					}
					return fmt.Errorf("data validation failed: %w", valErr)
				}
				warnings := resultWarnings(result)
				logWarnings(warnings, logf)
				if result.HasErrors() {
					message := formatValidationFailure("data validation", result)
					if safeDiagnostics {
						message = formatSafeValidationFailure("data validation", dirPath, result)
					}
					findingErr := fmt.Errorf("%s", message)
					return NewValidationFailedError(findingErr)
				}
				logValidationSummary(def, result, warnings, logf)
			}
			return nil
		},
	}
	addPathFlag(cmd)
	cmd.Flags().String("from-commit", "", "validate only records changed since this commit")
	cmd.Flags().String("to-commit", "", "validate only records up to this commit")
	cmd.Flags().String("only", "", `validate only "definition" or "records" (default: both)`)
	cmd.Flags().Bool("safe-diagnostics", false, "report finding identities and constraint classes without record values")
	return cmd
}

// logValidationSummary prints one line per validated collection: the root
// collections first, then every subcollection by its full schema path
// (`spaces/ext/days`), whose count adds up the records of every parent
// instance. A final line totals records and collections, so a subcollection
// that held no records is visible instead of absent.
func logValidationSummary(def *ingitdb.Definition, result *ingitdb.ValidationResult, warnings []ingitdb.ValidationError, logf func(...any)) {
	warned := make(map[string]bool, len(warnings))
	for _, w := range warnings {
		warned[w.CollectionID] = true
	}
	rootIDs := slices.Sorted(maps.Keys(def.Collections))
	var subIDs []string
	for _, rootID := range rootIDs {
		subIDs = appendSubCollectionIDs(subIDs, rootID, def.Collections[rootID])
	}
	records := 0
	for _, collectionID := range slices.Concat(rootIDs, subIDs) {
		passed, total := result.GetRecordCounts(collectionID)
		records += total
		if warned[collectionID] {
			// Records exist that validation could not read (see the WARNING
			// lines), so "All N records are valid" would mislead.
			logf(fmt.Sprintf("%d out of %d records read are valid for collection: %s; it has records that were not read (see WARNING above)", passed, total, collectionID))
		} else if passed == total {
			logf(fmt.Sprintf("All %d records are valid for collection: %s", total, collectionID))
		} else {
			logf(fmt.Sprintf("%d out of %d records are valid for collection: %s", passed, total, collectionID))
		}
	}
	logf(fmt.Sprintf("Validated %d records in %d collections (%d root collections, %d subcollections)",
		records, len(rootIDs)+len(subIDs), len(rootIDs), len(subIDs)))
}

// appendSubCollectionIDs appends the full paths of colDef's subcollections,
// depth first and sorted, to ids.
func appendSubCollectionIDs(ids []string, fullID string, colDef *ingitdb.CollectionDef) []string {
	if colDef == nil {
		return ids
	}
	for _, subID := range slices.Sorted(maps.Keys(colDef.SubCollections)) {
		subFullID := fullID + "/" + subID
		ids = append(ids, subFullID)
		ids = appendSubCollectionIDs(ids, subFullID, colDef.SubCollections[subID])
	}
	return ids
}

func formatSafeValidationFailure(prefix, rootPath string, result *ingitdb.ValidationResult) string {
	validationErrors := result.Errors()
	details := formatSafeValidationErrors(rootPath, validationErrors)
	if details == "" {
		return fmt.Sprintf("%s found %d error(s)", prefix, result.ErrorCount())
	}
	return fmt.Sprintf("%s found %d error(s): %s", prefix, result.ErrorCount(), details)
}

func formatSafeValidationErrors(rootPath string, validationErrors []ingitdb.ValidationError) string {
	if len(validationErrors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(validationErrors))
	for _, validationErr := range validationErrors {
		part := formatSafeValidationError(rootPath, validationErr)
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func formatSafeValidationError(rootPath string, validationErr ingitdb.ValidationError) string {
	parts := make([]string, 0, 5)
	if validationErr.CollectionID != "" {
		collectionPart := fmt.Sprintf("collection %q", validationErr.CollectionID)
		parts = append(parts, collectionPart)
	}
	if validationErr.FilePath != "" {
		filePath := safeRepositoryPath(rootPath, validationErr.FilePath)
		filePart := fmt.Sprintf("file %q", filePath)
		parts = append(parts, filePart)
	}
	if validationErr.RecordKey != "" {
		recordPart := fmt.Sprintf("record %q", validationErr.RecordKey)
		parts = append(parts, recordPart)
	}
	if validationErr.FieldName != "" {
		fieldPart := fmt.Sprintf("field %q", validationErr.FieldName)
		parts = append(parts, fieldPart)
	}
	parts = append(parts, safeConstraintClass(validationErr.Message))
	return strings.Join(parts, ": ")
}

func safeRepositoryPath(rootPath, filePath string) string {
	cleanRoot, _ := filepath.Abs(rootPath)
	cleanFile := filePath
	if !filepath.IsAbs(cleanFile) {
		cleanFile = filepath.Join(cleanRoot, cleanFile)
	}
	cleanFile, _ = filepath.Abs(cleanFile)
	relPath, relErr := filepath.Rel(cleanRoot, cleanFile)
	separator := string(filepath.Separator)
	parentPrefix := ".." + separator
	if relErr != nil || relPath == ".." || strings.HasPrefix(relPath, parentPrefix) {
		return filepath.Base(filePath)
	}
	return filepath.ToSlash(relPath)
}

func safeConstraintClass(message string) string {
	normalized := strings.ToLower(message)
	switch {
	case strings.Contains(normalized, "foreign key"):
		return "foreign-key constraint failed"
	case strings.Contains(normalized, "missing required field"):
		return "required-field constraint failed"
	case strings.Contains(normalized, "wrong type"):
		return "type constraint failed"
	case strings.Contains(normalized, "not one of the permitted values"):
		return "enum constraint failed"
	case strings.Contains(normalized, "min_value"), strings.Contains(normalized, "max_value"):
		return "range constraint failed"
	case strings.Contains(normalized, "min_length"), strings.Contains(normalized, "max_length"), strings.Contains(normalized, "required length"):
		return "length constraint failed"
	case strings.Contains(normalized, "undeclared field"):
		return "declared-field constraint failed"
	case strings.Contains(normalized, "computed column"):
		return "computed-field constraint failed"
	case strings.Contains(normalized, "record(s)"):
		return "record-count constraint failed"
	case strings.Contains(normalized, "parse"):
		return "record-format constraint failed"
	default:
		return "validation constraint failed"
	}
}

func formatValidationFailure(prefix string, result *ingitdb.ValidationResult) string {
	details := formatValidationErrors(result.Errors())
	if details == "" {
		return fmt.Sprintf("%s found %d error(s)", prefix, result.ErrorCount())
	}
	return fmt.Sprintf("%s found %d error(s): %s", prefix, result.ErrorCount(), details)
}

func formatValidationErrors(errors []ingitdb.ValidationError) string {
	if len(errors) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errors))
	for _, validationErr := range errors {
		part := formatValidationError(validationErr)
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func formatValidationError(validationErr ingitdb.ValidationError) string {
	parts := make([]string, 0, 4)
	if validationErr.FilePath != "" {
		parts = append(parts, validationErr.FilePath)
	}
	if validationErr.RecordKey != "" {
		recordPart := fmt.Sprintf("record %q", validationErr.RecordKey)
		parts = append(parts, recordPart)
	}
	if validationErr.FieldName != "" {
		fieldPart := fmt.Sprintf("field %q", validationErr.FieldName)
		parts = append(parts, fieldPart)
	}
	message := validationErr.Error()
	parts = append(parts, message)
	return strings.Join(parts, ": ")
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

// resultWarnings returns the warnings of a validation result when the linked
// ingitdb-go provides them (ValidationResult.Warnings: records that
// validation could not read, such as subcollection data in the storage
// drivers' layout or behind a symbolic link that leads outside the database).
// Older ingitdb-go versions have no warnings, and none are returned.
func resultWarnings(result any) []ingitdb.ValidationError {
	if w, ok := result.(interface {
		Warnings() []ingitdb.ValidationError
	}); ok {
		return w.Warnings()
	}
	return nil
}

// logWarnings prints one "WARNING: " line per warning. Warnings never change
// the exit code.
func logWarnings(warnings []ingitdb.ValidationError, logf func(...any)) {
	for _, w := range warnings {
		logf("WARNING: " + w.Message)
	}
}
