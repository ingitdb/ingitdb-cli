package dalgo2ingitdb

import (
	"encoding/json"
	"fmt"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// ParseRecordContent parses record content in YAML or JSON format.
func ParseRecordContent(content []byte, format ingitdb.RecordFormat) (map[string]any, error) {
	var data map[string]any
	switch format {
	case "yaml", "yml":
		err := yaml.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML record: %w", err)
		}
	case "json":
		err := json.Unmarshal(content, &data)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON record: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
	return data, nil
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
