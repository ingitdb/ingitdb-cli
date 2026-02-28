package ingitdb

// RecordEntry is one parsed record.
type RecordEntry struct {
	ID   string // can be empty for list-type files
	Data map[string]any
}
