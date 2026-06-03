package datavalidator

// specscore: feature/cli/validate

import (
	"path/filepath"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// NewChangeSetResolver returns the default ChangeSetResolver, which maps changed
// files to the collection records they belong to. It assumes the database
// directory is the git repository root (changed-file paths are joined onto it).
func NewChangeSetResolver() ChangeSetResolver {
	return changeSetResolver{}
}

type changeSetResolver struct{}

// Resolve maps each changed file to the collection record it affects. Deleted
// files are skipped (there is nothing to open; orphaned-reference checks are
// out of scope). For single-record layouts the affected record key is derived
// from the file name; for map/list layouts the whole shared file is marked
// affected (RecordKey == "").
func (changeSetResolver) Resolve(dbPath string, def *ingitdb.Definition, changedFiles []ingitdb.ChangedFile) ([]AffectedRecord, error) {
	var affected []AffectedRecord
	for _, cf := range changedFiles {
		if cf.Kind == ingitdb.ChangeKindDeleted {
			continue
		}
		absPath := filepath.Clean(filepath.Join(dbPath, cf.Path))
		colID, colDef := CollectionForRecordFile(def, absPath)
		if colDef == nil {
			continue
		}
		switch colDef.RecordFile.RecordType {
		case ingitdb.SingleRecord:
			affected = append(affected, AffectedRecord{
				CollectionID: colID,
				FilePath:     absPath,
				RecordKey:    recordKeyFromFilePath(absPath),
				ChangeKind:   cf.Kind,
			})
		case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
			affected = append(affected, AffectedRecord{
				CollectionID: colID,
				FilePath:     absPath,
				RecordKey:    "",
				ChangeKind:   cf.Kind,
			})
		}
	}
	return affected, nil
}

// CollectionForRecordFile returns the collection (and its ID) that owns absPath
// as a record file, or ("", nil) when no collection's record-file layout
// matches. It mirrors how the full validator enumerates record files, so the
// incremental validator, change-set resolver, and `diff` all agree on which
// files are records.
func CollectionForRecordFile(def *ingitdb.Definition, absPath string) (string, *ingitdb.CollectionDef) {
	for id, colDef := range def.Collections {
		if shouldSkipRecordParsing(colDef) {
			continue
		}
		switch colDef.RecordFile.RecordType {
		case ingitdb.SingleRecord:
			pattern, err := singleRecordGlobPattern(colDef)
			if err != nil {
				continue
			}
			matched, matchErr := filepath.Match(filepath.Clean(pattern), absPath)
			if matchErr == nil && matched && !skipRecordPath(absPath, colDef.RecordFile) {
				return id, colDef
			}
		case ingitdb.MapOfRecords, ingitdb.ListOfRecords:
			if filepath.Clean(collectionRecordFilePath(colDef)) == absPath {
				return id, colDef
			}
		}
	}
	return "", nil
}
