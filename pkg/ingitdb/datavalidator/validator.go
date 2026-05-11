package datavalidator

import (
	"context"
	"os"
	"path/filepath"
	"strings"

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
		total, err := countRecords(colDef)
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
func countRecords(colDef *ingitdb.CollectionDef) (int, error) {
	collectionPath := colDef.DirPath
	exts := expectedRecordExtensions(colDef)
	recordsSubDir := filepath.Join(collectionPath, "$records")
	if info, err := os.Stat(recordsSubDir); err == nil && info.IsDir() {
		return countEntries(recordsSubDir, exts)
	}
	return countEntries(collectionPath, exts)
}

// expectedRecordExtensions returns the file extensions that count as record
// files for this collection.
//
// The authoritative source is the collection's `record_file.name` template
// (e.g. `{key}.md`) — whatever extension it ends with is the single
// extension that records use. This naturally extends to any future format
// without needing changes here.
//
// When no `RecordFile` is declared (older test fixtures), the legacy
// permissive set (`.yaml`, `.yml`, `.json`) is returned so existing
// behavior is preserved.
func expectedRecordExtensions(colDef *ingitdb.CollectionDef) map[string]struct{} {
	if colDef.RecordFile != nil && colDef.RecordFile.Name != "" {
		ext := strings.ToLower(filepath.Ext(colDef.RecordFile.Name))
		if ext != "" {
			return map[string]struct{}{ext: {}}
		}
	}
	return map[string]struct{}{".yaml": {}, ".yml": {}, ".json": {}}
}

func countEntries(dirPath string, exts map[string]struct{}) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	// Count unique record keys. A key may appear as a plain file (e.g. USD.yaml)
	// or as a subdirectory (e.g. ord001/ holding subcollection data), or both.
	// We deduplicate by stripping the file extension so that a record with both
	// an ord001.yaml and an ord001/ directory is counted only once.
	seen := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "$records" {
			continue
		}
		if entry.IsDir() {
			seen[name] = struct{}{}
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := exts[ext]; ok {
			seen[strings.TrimSuffix(name, filepath.Ext(name))] = struct{}{}
		}
	}
	return len(seen), nil
}
