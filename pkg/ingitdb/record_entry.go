package ingitdb

import "fmt"

type IRecordEntry interface {
	GetID() string
	GetData() map[string]any
}

type mapRecordEntry[TKey comparable] struct {
	id   TKey
	data map[string]any
}

func (r mapRecordEntry[TKey]) GetID() string {
	return fmt.Sprintf("%v", r.id)
}

func (r mapRecordEntry[TKey]) GetData() map[string]any {
	return r.data
}

func NewMapRecordEntry[TKey comparable](id TKey, data map[string]any) IRecordEntry {
	return mapRecordEntry[TKey]{
		id:   id,
		data: data,
	}
}

var _ IRecordEntry = (*RecordEntry)(nil)

// RecordEntry is one parsed record.
type RecordEntry struct {
	ID   string // can be empty for list-type files
	Data map[string]any
}

func (r RecordEntry) GetID() string {
	return r.ID
}

func (r RecordEntry) GetData() map[string]any {
	return r.Data
}
