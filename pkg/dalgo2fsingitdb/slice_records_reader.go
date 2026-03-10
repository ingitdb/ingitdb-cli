package dalgo2fsingitdb

import "github.com/dal-go/dalgo/dal"

// sliceRecordsReader implements dal.RecordsReader over a pre-loaded slice of records.
type sliceRecordsReader struct {
	records []dal.Record
	index   int
}

func newSliceRecordsReader(records []dal.Record) dal.RecordsReader {
	return &sliceRecordsReader{records: records}
}

func (r *sliceRecordsReader) Next() (dal.Record, error) {
	if r.index >= len(r.records) {
		return nil, dal.ErrNoMoreRecords
	}
	rec := r.records[r.index]
	r.index++
	return rec, nil
}

func (r *sliceRecordsReader) Cursor() (string, error) {
	return "", nil
}

func (r *sliceRecordsReader) Close() error {
	return nil
}
