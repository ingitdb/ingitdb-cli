package ingitdb

// RecordEntry is one parsed record.
type RecordEntry struct {
	Key  string // may be empty for list-type files
	Data map[string]any
}
