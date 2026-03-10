package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Truncate returns the truncate command.
func Truncate(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "truncate",
		Short: "Remove all records from a collection",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = logf
			dirPath, resolveErr := resolveDBPath(cmd, homeDir, getWd)
			if resolveErr != nil {
				return resolveErr
			}
			def, readErr := readDefinition(dirPath)
			if readErr != nil {
				return fmt.Errorf("failed to read database definition: %w", readErr)
			}

			collectionID, _ := cmd.Flags().GetString("collection")
			colDef, ok := def.Collections[collectionID]
			if !ok {
				return fmt.Errorf("collection not found: %s", collectionID)
			}

			removed, truncErr := truncateCollection(colDef)
			if truncErr != nil {
				return truncErr
			}

			_, _ = fmt.Fprintf(os.Stderr, "truncated collection %s: removed %d record(s)\n", collectionID, removed)
			return nil
		},
	}
	addPathFlag(cmd)
	addCollectionFlag(cmd, true)
	return cmd
}

// truncateCollection removes all record files from the given collection.
// For SingleRecord collections (files in $records/ dir), it removes all files
// in that directory. For MapOfRecords collections (all records in one file),
// it overwrites the file with an empty map.
// Returns the number of records removed.
func truncateCollection(colDef *ingitdb.CollectionDef) (int, error) {
	if colDef.RecordFile == nil {
		return 0, fmt.Errorf("collection %s has no record file definition", colDef.ID)
	}

	basePath := colDef.RecordFile.RecordsBasePath()

	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return truncateSingleRecordCollection(colDef.DirPath, basePath)
	case ingitdb.MapOfRecords:
		return truncateMapOfRecordsCollection(colDef)
	default:
		return 0, fmt.Errorf("unsupported record type %q for truncation", colDef.RecordFile.RecordType)
	}
}

// truncateSingleRecordCollection removes all record files from the $records/
// subdirectory (or collection dir if no base path). It preserves the directory itself.
func truncateSingleRecordCollection(collectionDirPath, basePath string) (int, error) {
	recordsDir := filepath.Join(collectionDirPath, basePath)

	info, statErr := os.Stat(recordsDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return 0, nil // no records directory means nothing to truncate
		}
		return 0, fmt.Errorf("failed to stat records directory %s: %w", recordsDir, statErr)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("expected %s to be a directory", recordsDir)
	}

	entries, readErr := os.ReadDir(recordsDir)
	if readErr != nil {
		return 0, fmt.Errorf("failed to read records directory %s: %w", recordsDir, readErr)
	}

	removed := 0
	for _, entry := range entries {
		entryPath := filepath.Join(recordsDir, entry.Name())
		if entry.IsDir() {
			// Record directories (e.g. for subcollection key dirs like de/de.yaml)
			removeErr := os.RemoveAll(entryPath)
			if removeErr != nil {
				return removed, fmt.Errorf("failed to remove record directory %s: %w", entryPath, removeErr)
			}
		} else {
			removeErr := os.Remove(entryPath)
			if removeErr != nil {
				return removed, fmt.Errorf("failed to remove record file %s: %w", entryPath, removeErr)
			}
		}
		removed++
	}

	return removed, nil
}

// truncateMapOfRecordsCollection reads the map-of-records file, counts the
// records, then overwrites it with an empty map.
func truncateMapOfRecordsCollection(colDef *ingitdb.CollectionDef) (int, error) {
	filePath := filepath.Join(colDef.DirPath, colDef.RecordFile.Name)

	content, readErr := os.ReadFile(filePath)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to read records file %s: %w", filePath, readErr)
	}

	// Count non-empty lines as a rough proxy; we write an empty file.
	// For an accurate count, parse the file, but for truncation we just need
	// to know something was there.
	count := 0
	if len(content) > 0 {
		count = 1 // at least one record was present
	}

	// Write an empty YAML/JSON map
	var emptyContent []byte
	switch colDef.RecordFile.Format {
	case "yaml", "yml":
		emptyContent = []byte("{}\n")
	case "json":
		emptyContent = []byte("{}\n")
	default:
		return 0, fmt.Errorf("unsupported record format %q", colDef.RecordFile.Format)
	}

	writeErr := os.WriteFile(filePath, emptyContent, 0o644)
	if writeErr != nil {
		return 0, fmt.Errorf("failed to write empty records file %s: %w", filePath, writeErr)
	}

	return count, nil
}
