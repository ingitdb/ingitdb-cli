package dalgo2ingitdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ingr-io/ingr-go/ingr"
	"github.com/pelletier/go-toml/v2"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/markdown"
	"gopkg.in/yaml.v3"
)

// ParseRecordContent parses record content in YAML or JSON format.
// For markdown-format collections use ParseRecordContentForCollection,
// which also has access to the column schema and content-field name.
func ParseRecordContent(content []byte, format ingitdb.RecordFormat) (map[string]any, error) {
	var data map[string]any
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		err := yaml.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML record: %w", err)
		}
	case ingitdb.RecordFormatJSON:
		err := json.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON record: %w", err)
		}
	case ingitdb.RecordFormatTOML:
		err := toml.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse TOML record: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
	return data, nil
}

// ParseRecordContentForCollection parses record content using the
// collection's declared format. For YAML and JSON this is equivalent to
// ParseRecordContent. For markdown records it parses YAML frontmatter,
// filters to columns declared in the schema, and exposes the body under
// the configured content_field column name.
func ParseRecordContentForCollection(content []byte, colDef *ingitdb.CollectionDef) (map[string]any, error) {
	if colDef == nil || colDef.RecordFile == nil {
		return nil, fmt.Errorf("collection definition missing record_file")
	}
	if colDef.RecordFile.Format != ingitdb.RecordFormatMarkdown {
		return ParseRecordContent(content, colDef.RecordFile.Format)
	}
	frontmatter, body, err := markdown.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown record: %w", err)
	}
	contentField := colDef.RecordFile.ResolvedContentField()
	if _, collision := frontmatter[contentField]; collision {
		return nil, fmt.Errorf("frontmatter key %q collides with the content field name; "+
			"the markdown body is stored under this key — remove it from the frontmatter "+
			"or override content_field in the collection definition", contentField)
	}
	result := make(map[string]any, len(frontmatter)+1)
	for key, value := range frontmatter {
		// $id is a metadata key used for record-key resolution; pass it
		// through so callers (e.g. insert) can extract and strip it.
		if key == "$id" {
			result[key] = value
			continue
		}
		if _, declared := colDef.Columns[key]; !declared {
			continue
		}
		result[key] = value
	}
	result[contentField] = string(body)
	return result, nil
}

// ParseMapOfRecordsContent parses content containing a map of ID-keyed records.
//
// For yaml/json/toml, the file's top-level structure is a mapping from record
// ID to a per-record field map; we re-shape it into map[string]map[string]any.
//
// For INGR, the file is a list of records where the reserved `$ID` column
// holds each record's key; we read the list and re-index by `$ID`. Records
// missing `$ID`, or with duplicate `$ID` values, are rejected as malformed.
func ParseMapOfRecordsContent(content []byte, format ingitdb.RecordFormat) (map[string]map[string]any, error) {
	if format == ingitdb.RecordFormatINGR {
		return parseINGRAsMap(content)
	}
	raw, err := ParseRecordContent(content, format)
	if err != nil {
		return nil, err
	}
	result := make(map[string]map[string]any, len(raw))
	for id, value := range raw {
		recordFields, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("record %q is not a map", id)
		}
		result[id] = recordFields
	}
	return result, nil
}

// EncodeMapOfRecordsContent serializes a map of ID-keyed records into the
// declared format. For yaml/json/toml, the data is written as a top-level
// mapping (record ID -> field map). For INGR, the data is flattened into a
// list of records (each gets `$ID` injected as the first column) and
// written via the ingr-io writer.
//
// recordsetName is used as the INGR header title (typically the collection
// ID). It is ignored for non-INGR formats.
// columnsOrder controls the column order in the INGR header; when empty,
// columns are emitted in alphabetical order with `$ID` first.
func EncodeMapOfRecordsContent(data map[string]map[string]any, format ingitdb.RecordFormat, recordsetName string, columnsOrder []string) ([]byte, error) {
	if format != ingitdb.RecordFormatINGR {
		// Existing formats: write as a top-level map.
		raw := make(map[string]any, len(data))
		for id, fields := range data {
			raw[id] = fields
		}
		return marshalForFormat(raw, format)
	}
	return encodeINGRFromMap(data, recordsetName, columnsOrder)
}

func marshalForFormat(value any, format ingitdb.RecordFormat) ([]byte, error) {
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		out, err := yaml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal YAML: %w", err)
		}
		return out, nil
	case ingitdb.RecordFormatJSON:
		out, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON: %w", err)
		}
		return append(out, '\n'), nil
	case ingitdb.RecordFormatTOML:
		out, err := toml.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal TOML: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
}

// encodeINGRFromMap flattens an ID-keyed record map into a deterministic
// list of records and writes it via the ingr-io RecordsWriter.
func encodeINGRFromMap(data map[string]map[string]any, recordsetName string, columnsOrder []string) ([]byte, error) {
	// Resolve column order. $ID is always first. Then declared columns_order,
	// then any remaining keys alphabetically. Same canonical ordering rule
	// we use for markdown frontmatter.
	colNames := resolveINGRColumns(data, columnsOrder)

	cols := make([]ingr.ColDef, 0, len(colNames))
	for _, name := range colNames {
		cols = append(cols, ingr.ColDef{Name: name})
	}

	// Sort record IDs for deterministic output.
	ids := make([]string, 0, len(data))
	for id := range data {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	records := make([]ingr.Record, 0, len(ids))
	for _, id := range ids {
		row := make(map[string]any, len(data[id])+1)
		for k, v := range data[id] {
			row[k] = v
		}
		row["$ID"] = id
		records = append(records, ingr.NewMapRecordEntry(id, row))
	}

	var buf bytes.Buffer
	w := ingr.NewRecordsWriter(&buf)
	if _, err := w.WriteHeader(recordsetName, cols); err != nil {
		return nil, fmt.Errorf("ingr: write header: %w", err)
	}
	if _, err := w.WriteRecords(0, records...); err != nil {
		return nil, fmt.Errorf("ingr: write records: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("ingr: close writer: %w", err)
	}
	return buf.Bytes(), nil
}

func resolveINGRColumns(data map[string]map[string]any, columnsOrder []string) []string {
	seen := map[string]bool{"$ID": true}
	ordered := []string{"$ID"}
	for _, name := range columnsOrder {
		if name == "$ID" || seen[name] {
			continue
		}
		seen[name] = true
		ordered = append(ordered, name)
	}
	var rest []string
	for _, fields := range data {
		for name := range fields {
			if seen[name] {
				continue
			}
			seen[name] = true
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(ordered, rest...)
}

// parseINGRAsMap decodes an INGR file into map[string]map[string]any keyed
// by the reserved `$ID` column.
func parseINGRAsMap(content []byte) (map[string]map[string]any, error) {
	var rows []map[string]any
	if err := ingr.Unmarshal(content, &rows); err != nil {
		return nil, fmt.Errorf("failed to parse INGR records: %w", err)
	}
	result := make(map[string]map[string]any, len(rows))
	for i, row := range rows {
		raw, ok := row["$ID"]
		if !ok {
			return nil, fmt.Errorf("INGR record at index %d is missing required $ID column", i)
		}
		id, ok := raw.(string)
		if !ok {
			return nil, fmt.Errorf("INGR record at index %d has non-string $ID value (%T)", i, raw)
		}
		if _, dup := result[id]; dup {
			return nil, fmt.Errorf("INGR record has duplicate $ID %q", id)
		}
		fields := make(map[string]any, len(row)-1)
		for k, v := range row {
			if k == "$ID" {
				continue
			}
			fields[k] = v
		}
		result[id] = fields
	}
	return result, nil
}
