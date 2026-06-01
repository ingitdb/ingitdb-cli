package dalgo2ingitdb

import (
	"fmt"
	"sort"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func (r readwriteTx) validateWriteForeignKeys(operation, childCollection string, childDef *ingitdb.CollectionDef, data map[string]any) error {
	fields := orderedForeignKeyFields(childDef)
	if len(fields) == 0 {
		return nil
	}
	if r.def == nil {
		return fmt.Errorf("dalgo2ingitdb: %s configuration error: transaction has no loaded definition", operation)
	}
	for _, field := range fields {
		column := childDef.Columns[field]
		parentCollection := column.ForeignKey
		if parentCollection == "" {
			continue
		}
		value, ok := data[field]
		empty := !ok
		if ok {
			empty = isEmptyForeignKeyValue(value)
		}
		if empty {
			if column.Required {
				return fmt.Errorf("dalgo2ingitdb: %s required foreign key field missing or empty: child collection %q field %q references parent collection %q", operation, childCollection, field, parentCollection)
			}
			continue
		}
		parentKey := fmt.Sprintf("%v", value)
		parentDef, ok := r.def.Collections[parentCollection]
		if !ok {
			return fmt.Errorf("dalgo2ingitdb: %s configuration error: child collection %q field %q references missing foreign_key collection %q", operation, childCollection, field, parentCollection)
		}
		exists, err := foreignKeyTargetExists(parentDef, parentKey)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: %s foreign key lookup failed: child collection %q field %q parent collection %q key %q: %w", operation, childCollection, field, parentCollection, parentKey, err)
		}
		if !exists {
			return fmt.Errorf("dalgo2ingitdb: %s foreign key violation: child collection %q field %q references parent collection %q key %q: parent record not found", operation, childCollection, field, parentCollection, parentKey)
		}
	}
	return nil
}

func (r readwriteTx) validateDeleteForeignKeys(parentCollection, parentKey string) error {
	if r.def == nil {
		return fmt.Errorf("dalgo2ingitdb: Delete configuration error: transaction has no loaded definition")
	}

	childCollections := orderedCollectionIDs(r.def.Collections)
	for _, childCollection := range childCollections {
		childDef := r.def.Collections[childCollection]
		fields := foreignKeyFieldsReferencing(childDef, parentCollection)
		if len(fields) == 0 {
			continue
		}

		records, err := readAllRecordsFromDisk(childDef)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: Delete foreign key scan failed for parent collection %q key %q child collection %q: %w", parentCollection, parentKey, childCollection, err)
		}

		sort.SliceStable(records, func(i, j int) bool {
			left := fmt.Sprintf("%v", records[i].Key().ID)
			right := fmt.Sprintf("%v", records[j].Key().ID)
			return left < right
		})

		for _, record := range records {
			childKey := fmt.Sprintf("%v", record.Key().ID)
			recordData := record.Data()
			data, ok := recordData.(map[string]any)
			if !ok {
				return fmt.Errorf("dalgo2ingitdb: Delete foreign key scan failed for child collection %q record %q: data has type %T", childCollection, childKey, recordData)
			}

			for _, field := range fields {
				value, ok := data[field]
				if !ok || isEmptyForeignKeyValue(value) {
					continue
				}

				valueText := fmt.Sprintf("%v", value)
				if valueText == parentKey {
					return fmt.Errorf("dalgo2ingitdb: Delete foreign key violation: parent collection %q key %q is referenced by child collection %q record %q field %q", parentCollection, parentKey, childCollection, childKey, field)
				}
			}
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

func orderedCollectionIDs(collections map[string]*ingitdb.CollectionDef) []string {
	if len(collections) == 0 {
		return nil
	}

	ids := make([]string, 0, len(collections))
	for id := range collections {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func foreignKeyFieldsReferencing(colDef *ingitdb.CollectionDef, parentCollection string) []string {
	fields := orderedForeignKeyFields(colDef)
	matchingFields := make([]string, 0, len(fields))
	for _, field := range fields {
		column := colDef.Columns[field]
		if column.ForeignKey == parentCollection {
			matchingFields = append(matchingFields, field)
		}
	}
	return matchingFields
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
