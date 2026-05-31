package dalgo2ingitdb

import (
	"fmt"
	"sort"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func (r readwriteTx) validateInsertForeignKeys(childCollection string, childDef *ingitdb.CollectionDef, data map[string]any) error {
	fields := orderedForeignKeyFields(childDef)
	if len(fields) == 0 {
		return nil
	}
	if r.def == nil {
		return fmt.Errorf("dalgo2ingitdb: Insert configuration error: transaction has no loaded definition")
	}
	for _, field := range fields {
		column := childDef.Columns[field]
		parentCollection := column.ForeignKey
		if parentCollection == "" {
			continue
		}
		value, ok := data[field]
		if !ok {
			continue
		}
		if isEmptyForeignKeyValue(value) {
			continue
		}
		parentKey := fmt.Sprintf("%v", value)
		parentDef, ok := r.def.Collections[parentCollection]
		if !ok {
			return fmt.Errorf("dalgo2ingitdb: Insert configuration error: child collection %q field %q references missing foreign_key collection %q", childCollection, field, parentCollection)
		}
		exists, err := foreignKeyTargetExists(parentDef, parentKey)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: Insert foreign key lookup failed: child collection %q field %q parent collection %q key %q: %w", childCollection, field, parentCollection, parentKey, err)
		}
		if !exists {
			return fmt.Errorf("dalgo2ingitdb: Insert foreign key violation: child collection %q field %q references parent collection %q key %q: parent record not found", childCollection, field, parentCollection, parentKey)
		}
	}
	return nil
}

func orderedForeignKeyFields(colDef *ingitdb.CollectionDef) []string {
	if colDef == nil || len(colDef.Columns) == 0 {
		return nil
	}
	fields := make([]string, 0, len(colDef.Columns))
	seen := make(map[string]bool, len(colDef.Columns))
	for _, field := range colDef.ColumnsOrder {
		column, ok := colDef.Columns[field]
		if !ok || column == nil || column.ForeignKey == "" {
			continue
		}
		fields = append(fields, field)
		seen[field] = true
	}
	var remaining []string
	for field, column := range colDef.Columns {
		if seen[field] || column == nil || column.ForeignKey == "" {
			continue
		}
		remaining = append(remaining, field)
	}
	sort.Strings(remaining)
	fields = append(fields, remaining...)
	return fields
}

func isEmptyForeignKeyValue(value any) bool {
	if value == nil {
		return true
	}
	text, ok := value.(string)
	if !ok {
		return false
	}
	return text == ""
}

func foreignKeyTargetExists(parentDef *ingitdb.CollectionDef, parentKey string) (bool, error) {
	if parentDef == nil {
		return false, fmt.Errorf("configuration error: parent collection definition is nil")
	}
	if parentDef.RecordFile == nil {
		return false, fmt.Errorf("configuration error: parent collection %q has no record_file definition", parentDef.ID)
	}
	path := resolveRecordPath(parentDef, parentKey)
	switch parentDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		data, found, err := readSingleRecordFile(path, parentDef)
		_ = data
		if err != nil {
			return false, err
		}
		return found, nil
	case ingitdb.MapOfRecords:
		allRecords, err := readMapOfRecordsFile(path, parentDef.RecordFile.Format)
		if err != nil {
			return false, err
		}
		_, exists := allRecords[parentKey]
		return exists, nil
	default:
		return false, fmt.Errorf("configuration error: parent collection %q has unsupported record type %q", parentDef.ID, parentDef.RecordFile.RecordType)
	}
}
