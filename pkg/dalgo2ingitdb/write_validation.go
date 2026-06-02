package dalgo2ingitdb

import "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"

// ValidateWrite enforces every write-time rule before a record is inserted or
// set, against the on-disk state described by def:
//
//   - a computed column's value must not be supplied (it is derived, not stored),
//   - every stored foreign key must resolve to an existing parent record, and
//   - every computed foreign key, evaluated from the record's stored fields, must
//     resolve to an existing parent record.
//
// operation labels the caller ("Insert" or "Set") for error messages.
// collectionID and colDef identify the collection being written; recordKey and
// data are the record's key and field values.
//
// This is the shared entry point for any DALgo driver's read-write transaction,
// so enforcement is identical whether a record is written through the
// concurrent-safe driver or the filesystem driver.
func ValidateWrite(def *ingitdb.Definition, operation, collectionID string, colDef *ingitdb.CollectionDef, recordKey string, data map[string]any) error {
	r := readwriteTx{readonlyTx: readonlyTx{def: def}}
	if err := r.validateNoStoredComputedValues(collectionID, colDef, recordKey, data); err != nil {
		return err
	}
	if err := r.validateWriteForeignKeys(operation, collectionID, colDef, data); err != nil {
		return err
	}
	return r.validateComputedWriteForeignKeys(operation, collectionID, colDef, recordKey, data)
}

// ValidateDelete enforces parent-side referential integrity before a record is
// removed (or its key renamed, which manifests as removal of the old key): no
// stored or computed foreign key in any collection may still reference it.
//
// This is the shared entry point for any DALgo driver's delete path, so
// enforcement is identical across drivers.
func ValidateDelete(def *ingitdb.Definition, parentCollection, parentKey string) error {
	r := readwriteTx{readonlyTx: readonlyTx{def: def}}
	if err := r.validateDeleteForeignKeys(parentCollection, parentKey); err != nil {
		return err
	}
	return r.validateDeleteComputedForeignKeys(parentCollection, parentKey)
}
