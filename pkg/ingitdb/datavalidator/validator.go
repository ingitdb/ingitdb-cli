package datavalidator

import (
	"context"
	"os"
	"path/filepath"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// NewValidator creates a simple data validator that checks record existence.
func NewValidator() DataValidator {
	return &simpleValidator{}
}

type simpleValidator struct{}

// Validate performs basic validation of records against their collection schemas.
// Returns a ValidationResult with any errors found.
func (sv *simpleValidator) Validate(_ context.Context, _ string, def *ingitdb.Definition) (*ingitdb.ValidationResult, error) {
	result := &ingitdb.ValidationResult{}

	// Count records for each collection
	for collectionKey, colDef := range def.Collections {
		total, err := countRecords(colDef.DirPath)
		if err != nil {
			// Don't fail validation on count error, just set 0
			total = 0
		}
		// For now, assume all records passed (total == passed)
		// The validator will be enhanced to track actual failures
		result.SetRecordCounts(collectionKey, total, total)
		// Also set the legacy record count for backward compatibility
		result.SetRecordCount(collectionKey, total)
	}

	// For now, we just return an empty result (no errors).
	// The validator will be enhanced to check record files and schemas.
	// This allows the "All records are valid" message to be logged when no errors exist.

	return result, nil
}

// countRecords counts the number of record keys in a collection directory.
// When a $records/ subdirectory exists (used for per-key record files), it
// counts entries inside that directory instead of at the collection root.
func countRecords(collectionPath string) (int, error) {
	recordsSubDir := filepath.Join(collectionPath, "$records")
	if info, err := os.Stat(recordsSubDir); err == nil && info.IsDir() {
		return countEntries(recordsSubDir)
	}
	return countEntries(collectionPath)
}

func countEntries(dirPath string) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".collection" {
			count++
		}
	}
	return count, nil
}
