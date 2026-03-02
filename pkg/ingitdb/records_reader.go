package ingitdb

import "context"

// RecordsReader streams records from one collection.
// Uses a yield callback to avoid goroutine leaks and keep allocation low.
type RecordsReader interface {
	ReadRecords(
		ctx context.Context,
		dbPath string,
		col *CollectionDef,
		yield func(IRecordEntry) error,
	) error
}
