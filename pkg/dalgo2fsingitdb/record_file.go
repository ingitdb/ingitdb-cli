package dalgo2fsingitdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// resolveRecordPath replaces all {key} occurrences in the record file name template and joins with the collection dir.
// When {key} is present, records are stored under a $records/ subdirectory to keep README.md visible on GitHub.com.
func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	name := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	base := colDef.RecordFile.RecordsBasePath()
	return filepath.Join(colDef.DirPath, base, name)
}

// readRecordFromFile reads a YAML or JSON file and returns its content as a map.
// Returns (nil, false, nil) if the file does not exist.
func readRecordFromFile(path string, format ingitdb.RecordFormat) (map[string]any, bool, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	var data map[string]any
	switch format {
	case "yaml", "yml":
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse YAML file %s: %w", path, err)
		}
	case "json":
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse JSON file %s: %w", path, err)
		}
	default:
		return nil, false, fmt.Errorf("unsupported record format %q", format)
	}
	return data, true, nil
}

// writeRecordToFile marshals data to the specified format and writes it to path.
// Intermediate directories are created as needed.
func writeRecordToFile(path string, format ingitdb.RecordFormat, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	var (
		content []byte
		err     error
	)
	switch format {
	case "yaml", "yml":
		content, err = yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data as YAML: %w", err)
		}
	case "json":
		content, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data as JSON: %w", err)
		}
		content = append(content, '\n')
	default:
		return fmt.Errorf("unsupported record format %q", format)
	}
	if err = os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", path, err)
	}
	return nil
}

// deleteRecordFile removes a record file. Returns dal.ErrRecordNotFound if it does not exist.
func deleteRecordFile(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return dal.ErrRecordNotFound
	}
	return err
}

// readMapOfRecordsFile reads a file whose top-level keys are record IDs and whose
// values are field maps (map[$record_id]map[$field_name]any layout).
// Returns (nil, false, nil) if the file does not exist.
func readMapOfRecordsFile(path string, format ingitdb.RecordFormat) (map[string]map[string]any, bool, error) {
	raw, found, err := readRecordFromFile(path, format)
	if err != nil || !found {
		return nil, found, err
	}
	result := make(map[string]map[string]any, len(raw))
	for id, val := range raw {
		fields, ok := val.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("record %q in %s is not a map", id, path)
		}
		result[id] = fields
	}
	return result, true, nil
}

// writeMapOfRecordsFile writes a map[$record_id]map[$field_name]any dataset back to a file.
func writeMapOfRecordsFile(path string, format ingitdb.RecordFormat, data map[string]map[string]any) error {
	raw := make(map[string]any, len(data))
	for id, fields := range data {
		raw[id] = fields
	}
	return writeRecordToFile(path, format, raw)
}
