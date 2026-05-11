package dalgo2fsingitdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/pelletier/go-toml/v2"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/markdown"
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
// For markdown-format collections use readMarkdownRecord instead.
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
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse YAML file %s: %w", path, err)
		}
	case ingitdb.RecordFormatJSON:
		if err = yaml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse JSON file %s: %w", path, err)
		}
	case ingitdb.RecordFormatTOML:
		if err = toml.Unmarshal(fileContent, &data); err != nil {
			return nil, false, fmt.Errorf("failed to parse TOML file %s: %w", path, err)
		}
	default:
		return nil, false, fmt.Errorf("unsupported record format %q", format)
	}
	return data, true, nil
}

// readMarkdownRecord reads a Markdown record file: parses YAML frontmatter,
// filters frontmatter keys to columns declared in colDef, and exposes the
// document body under the configured content_field column name.
// Returns (nil, false, nil) if the file does not exist.
func readMarkdownRecord(path string, colDef *ingitdb.CollectionDef) (map[string]any, bool, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	data, parseErr := dalgo2ingitdb.ParseRecordContentForCollection(fileContent, colDef)
	if parseErr != nil {
		return nil, false, fmt.Errorf("failed to parse markdown file %s: %w", path, parseErr)
	}
	return data, true, nil
}

// writeMarkdownRecord writes a Markdown record file. The content_field
// column value is written to the body byte-for-byte; all other columns
// declared in the schema are written to YAML frontmatter in the order
// defined by colDef.ColumnsOrder, with alphabetical fallback for columns
// not in ColumnsOrder. Undeclared keys in data are passed through
// (appended after declared columns, alphabetically).
func writeMarkdownRecord(path string, colDef *ingitdb.CollectionDef, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	contentField := colDef.RecordFile.ResolvedContentField()
	body := extractBody(data, contentField)
	frontmatter := make(map[string]any, len(data))
	for key, value := range data {
		if key == contentField {
			continue
		}
		frontmatter[key] = value
	}
	content, err := markdown.Serialize(frontmatter, colDef.ColumnsOrder, body)
	if err != nil {
		return fmt.Errorf("failed to serialize markdown record: %w", err)
	}
	if writeErr := os.WriteFile(path, content, 0o644); writeErr != nil {
		return fmt.Errorf("failed to write file %s: %w", path, writeErr)
	}
	return nil
}

// extractBody pulls the content field's value out of data and converts it
// to body bytes. A missing or nil value yields nil (empty body). A string
// or []byte value is used verbatim. Any other type is rendered as text.
func extractBody(data map[string]any, contentField string) []byte {
	raw, ok := data[contentField]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		return fmt.Appendf(nil, "%v", v)
	}
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
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		content, err = yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data as YAML: %w", err)
		}
	case ingitdb.RecordFormatJSON:
		content, err = json.MarshalIndent(data, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal data as JSON: %w", err)
		}
		content = append(content, '\n')
	case ingitdb.RecordFormatTOML:
		content, err = toml.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data as TOML: %w", err)
		}
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
//
// Dispatches to dalgo2ingitdb.ParseMapOfRecordsContent which understands
// every supported format including INGR (where records are read from a
// list and re-indexed by `$ID`).
func readMapOfRecordsFile(path string, format ingitdb.RecordFormat) (map[string]map[string]any, bool, error) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	result, parseErr := dalgo2ingitdb.ParseMapOfRecordsContent(fileContent, format)
	if parseErr != nil {
		return nil, false, fmt.Errorf("failed to parse records file %s: %w", path, parseErr)
	}
	return result, true, nil
}

// writeMapOfRecordsFile writes a map[$record_id]map[$field_name]any dataset back to a file.
//
// For yaml/json/toml the data is written as a top-level ID-keyed mapping
// using the existing single-record writer. For INGR the data is flattened
// into a record list with `$ID` injected, written through the ingr-io
// RecordsWriter; the recordsetName is taken from the collection ID and
// column order from the collection schema.
func writeMapOfRecordsFile(path string, colDef *ingitdb.CollectionDef, data map[string]map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	content, err := dalgo2ingitdb.EncodeMapOfRecordsContent(
		data, colDef.RecordFile.Format, colDef.ID, colDef.ColumnsOrder)
	if err != nil {
		return fmt.Errorf("failed to encode records for %s: %w", path, err)
	}
	if writeErr := os.WriteFile(path, content, 0o644); writeErr != nil {
		return fmt.Errorf("failed to write file %s: %w", path, writeErr)
	}
	return nil
}
