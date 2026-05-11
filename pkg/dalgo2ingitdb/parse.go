package dalgo2ingitdb

import (
	"encoding/json"
	"fmt"

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
	result := make(map[string]any, len(frontmatter)+1)
	for key, value := range frontmatter {
		if _, declared := colDef.Columns[key]; !declared {
			continue
		}
		result[key] = value
	}
	result[colDef.RecordFile.ResolvedContentField()] = string(body)
	return result, nil
}

// ParseMapOfRecordsContent parses content containing a map of ID-keyed records.
func ParseMapOfRecordsContent(content []byte, format ingitdb.RecordFormat) (map[string]map[string]any, error) {
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
