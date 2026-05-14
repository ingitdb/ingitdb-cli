package commands

// specscore: feature/output-formats

// projectRecord returns a map containing only the requested fields from data.
// If fields is nil or empty, all fields are returned with $id injected.
// The special field "$id" resolves to the record's key string.
//
// Used by cli/select for both single-record and set-mode output projection.
func projectRecord(data map[string]any, id string, fields []string) map[string]any {
	if len(fields) == 0 {
		result := make(map[string]any, len(data)+1)
		result["$id"] = id
		for k, v := range data {
			result[k] = v
		}
		return result
	}
	result := make(map[string]any, len(fields))
	for _, f := range fields {
		if f == "$id" {
			result["$id"] = id
		} else if v, ok := data[f]; ok {
			result[f] = v
		}
	}
	return result
}
