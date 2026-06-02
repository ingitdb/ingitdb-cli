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

// validateComputedWriteForeignKeys enforces referential integrity for computed
// foreign-key columns (columns with both ForeignKey and Formula set). The
// foreign-key value is never stored, so it is derived by evaluating the column's
// formula from the payload's stored fields and validated against the referenced
// collection exactly as a stored foreign key is. It runs on every write, so an
// update that changes an input field of the formula is re-validated even though
// the computed column itself was not written.
func (r readwriteTx) validateComputedWriteForeignKeys(operation, childCollection string, childDef *ingitdb.CollectionDef, recordKey string, data map[string]any) error {
	// r.def is guaranteed non-nil here: validateWriteForeignKeys runs first on
	// every Set/Insert and returns a configuration error when r.def is nil and
	// any foreign-key column (computed columns included) is present.
	for _, field := range orderedComputedColumns(childDef) {
		column := childDef.Columns[field]
		parentCollection := column.ForeignKey
		if parentCollection == "" {
			continue
		}
		result, err := ingitdb.EvaluateFormula(column.Formula, data)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: %s computed foreign key evaluation failed: collection %q record %q column %q references collection %q: %w", operation, childCollection, recordKey, field, parentCollection, err)
		}
		parentKey := computedForeignKeyString(result)
		parentDef, ok := r.def.Collections[parentCollection]
		if !ok {
			return fmt.Errorf("dalgo2ingitdb: %s configuration error: collection %q record %q column %q references missing foreign_key collection %q", operation, childCollection, recordKey, field, parentCollection)
		}
		exists, err := foreignKeyTargetExists(parentDef, parentKey)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: %s computed foreign key lookup failed: collection %q record %q column %q references collection %q key %q: %w", operation, childCollection, recordKey, field, parentCollection, parentKey, err)
		}
		if !exists {
			return fmt.Errorf("dalgo2ingitdb: %s computed foreign key violation: collection %q record %q column %q references collection %q key %q: parent record not found", operation, childCollection, recordKey, field, parentCollection, parentKey)
		}
	}
	return nil
}

// computedForeignKeyString coerces an evaluated formula result into the string
// key form used to look up the referenced record. EvaluateFormula yields
// string, int64, bool, float64, or nil; foreign keys are strings or integer
// keys, so this matches the stored-FK key handling (fmt's default formatting).
func computedForeignKeyString(result any) string {
	if result == nil {
		return ""
	}
	if s, ok := result.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", result)
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

// validateDeleteComputedForeignKeys mirrors validateDeleteForeignKeys for
// computed foreign keys. Computed FK values are never stored, so each child
// collection that declares a COMPUTED foreign key referencing parentCollection is
// scanned in full (no index): readAllRecordsFromDisk recomputes every child
// record's formula on read, populating the computed column, and if a recomputed
// key equals parentKey the delete (or rename, which manifests as a delete of the
// old key) is blocked with a reference-error-shape error naming the referencing
// collection, the referencing record key, the computed column, and the referenced
// collection.
func (r readwriteTx) validateDeleteComputedForeignKeys(parentCollection, parentKey string) error {
	// r.def is guaranteed non-nil here: Delete runs validateDeleteForeignKeys
	// first, which guards r.def == nil before this function is ever reached.
	childCollections := orderedCollectionIDs(r.def.Collections)
	for _, childCollection := range childCollections {
		childDef := r.def.Collections[childCollection]
		fields := computedForeignKeyFieldsReferencing(childDef, parentCollection)
		if len(fields) == 0 {
			continue
		}

		records, err := readAllRecordsFromDisk(childDef)
		if err != nil {
			return fmt.Errorf("dalgo2ingitdb: Delete computed foreign key scan failed for parent collection %q key %q child collection %q: %w", parentCollection, parentKey, childCollection, err)
		}

		sort.SliceStable(records, func(i, j int) bool {
			left := fmt.Sprintf("%v", records[i].Key().ID)
			right := fmt.Sprintf("%v", records[j].Key().ID)
			return left < right
		})

		for _, record := range records {
			childKey := fmt.Sprintf("%v", record.Key().ID)
			// readAllRecordsFromDisk always builds records via
			// dal.NewRecordWithData with a map[string]any, so Data() is always a map.
			data, _ := record.Data().(map[string]any)

			for _, field := range fields {
				value, ok := data[field]
				if !ok || isEmptyForeignKeyValue(value) {
					continue
				}
				derivedKey := computedForeignKeyString(value)
				if derivedKey == parentKey {
					return fmt.Errorf("dalgo2ingitdb: Delete computed foreign key violation: parent collection %q key %q is referenced by child collection %q record %q column %q", parentCollection, parentKey, childCollection, childKey, field)
				}
			}
		}
	}

	return nil
}

// orderedForeignKeyFields returns the names of stored foreign-key columns
// (ForeignKey set, Formula empty) in deterministic order. Computed foreign keys
// are never stored and are handled separately by the computed-FK validators, so
// they are excluded here.
func orderedForeignKeyFields(colDef *ingitdb.CollectionDef) []string {
	return orderedColumns(colDef, func(c *ingitdb.ColumnDef) bool {
		return c.ForeignKey != "" && c.Formula == ""
	})
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

// computedForeignKeyFieldsReferencing returns the computed foreign-key columns
// (both Formula and ForeignKey set) of colDef that reference parentCollection, in
// the deterministic order produced by orderedComputedColumns.
func computedForeignKeyFieldsReferencing(colDef *ingitdb.CollectionDef, parentCollection string) []string {
	fields := orderedComputedColumns(colDef)
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
