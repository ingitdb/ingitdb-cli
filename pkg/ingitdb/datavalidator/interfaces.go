package datavalidator

import (
	"context"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ForeignKeyIndex is a pre-built read-only lookup for FK validation.
// Built once before validation begins; shared read-only across goroutines.
type ForeignKeyIndex interface {
	Contains(collectionID, key string) bool
}

// RecordValidator validates one record against its collection schema.
// Goroutine-safe. Appends findings to result; returns count of errors appended.
type RecordValidator interface {
	ValidateRecord(
		col *ingitdb.CollectionDef,
		entry ingitdb.IRecordEntry,
		result *ingitdb.ValidationResult,
	) int
}

// DataValidator runs a full validation pass over all records.
type DataValidator interface {
	Validate(ctx context.Context, dbPath string, def *ingitdb.Definition) (*ingitdb.ValidationResult, error)
}

// AffectedRecord identifies which record was touched by a file change.
type AffectedRecord struct {
	CollectionID string
	FilePath     string
	RecordKey    string // empty if entire file changed (list/map format)
	ChangeKind   ingitdb.ChangeKind
}

// ChangeSetResolver maps changed files → (collectionID, recordKey) pairs.
// Handles all three RecordType layouts.
type ChangeSetResolver interface {
	Resolve(dbPath string, def *ingitdb.Definition, changedFiles []ingitdb.ChangedFile) ([]AffectedRecord, error)
}

// IncrementalValidator validates only records changed between two git refs.
type IncrementalValidator interface {
	ValidateChanges(
		ctx context.Context,
		dbPath string,
		def *ingitdb.Definition,
		fromCommit, toCommit string,
	) (*ingitdb.ValidationResult, error)
}
