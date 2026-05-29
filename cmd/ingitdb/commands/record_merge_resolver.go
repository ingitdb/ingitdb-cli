package commands

// specscore: feature/cli/resolve/auto-resolve/record-merge

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/docsbuilder"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/recordmerge"
)

// resolveRecordMergeConflicts attempts a record-aware three-way merge of each
// conflicted source-data file. Files it merges are written and staged and
// returned in resolved; files it cannot safely auto-merge (no collection,
// record-merge disabled, or an escalating conflict) are returned in unresolved
// for the caller to hand to manual resolution. A non-nil error indicates an
// infrastructure failure (write or `git add`).
func resolveRecordMergeConflicts(
	ctx context.Context,
	dirPath string,
	def *ingitdb.Definition,
	files []string,
) (resolved []string, unresolved []string, err error) {
	for _, f := range files {
		col := findCollectionForRecordFile(def, dirPath, f)
		if col == nil {
			unresolved = append(unresolved, f)
			continue
		}
		eff := ingitdb.ResolveRecordMerge(def, col)
		if !eff.Enabled {
			unresolved = append(unresolved, f)
			continue
		}

		base := gitStageContent(ctx, dirPath, f, 1)
		ours := gitStageContent(ctx, dirPath, f, 2)
		theirs := gitStageContent(ctx, dirPath, f, 3)

		merged, ok := mergeAndSerialize(base, ours, theirs, col, recordmerge.Options{SameRecord: eff.SameRecord})
		if !ok {
			unresolved = append(unresolved, f)
			continue
		}

		absPath := filepath.Join(dirPath, f)
		if writeErr := os.WriteFile(absPath, merged, 0o600); writeErr != nil {
			return resolved, unresolved, fmt.Errorf("write merged %s: %w", f, writeErr)
		}
		addCmd := exec.CommandContext(ctx, "git", "add", f)
		addCmd.Dir = dirPath
		if addErr := addCmd.Run(); addErr != nil {
			return resolved, unresolved, fmt.Errorf("stage merged %s: %w", f, addErr)
		}
		resolved = append(resolved, f)
	}
	return resolved, unresolved, nil
}

// mergeAndSerialize runs the three-way merge of a conflicted file's stages and
// serializes the result. ok is false — meaning the file must escalate to manual
// resolution — when the merge escalates or the merged records cannot be
// serialized for the collection's format.
func mergeAndSerialize(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts recordmerge.Options) ([]byte, bool) {
	outcome := recordmerge.MergeFiles(base, ours, theirs, col, opts)
	if outcome.Escalate {
		return nil, false
	}
	merged, err := serializeMergedRecords(outcome.Merged, col)
	if err != nil {
		return nil, false
	}
	return merged, true
}

// findCollectionForRecordFile maps a conflicted file (relative to dirPath) to
// the collection whose directory contains it, or nil when none matches.
func findCollectionForRecordFile(def *ingitdb.Definition, dirPath, file string) *ingitdb.CollectionDef {
	absPath := filepath.Join(dirPath, file)
	dir := filepath.Dir(absPath)
	return docsbuilder.FindCollectionByDir(def.Collections, dir)
}

// gitStageContent reads conflict stage n (1=base, 2=ours, 3=theirs) of file.
// A missing stage — e.g. a record file added on only one side — yields nil,
// which the merge engine treats as an absent record set.
func gitStageContent(ctx context.Context, dirPath, file string, stage int) []byte {
	spec := fmt.Sprintf(":%d:%s", stage, file)
	cmd := exec.CommandContext(ctx, "git", "show", spec)
	cmd.Dir = dirPath
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return out
}

// serializeMergedRecords renders merged records back to file bytes for the
// collection's record layout.
func serializeMergedRecords(records []recordmerge.Record, col *ingitdb.CollectionDef) ([]byte, error) {
	switch col.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		data := make(map[string]map[string]any, len(records))
		for _, r := range records {
			data[r.Key] = r.Fields
		}
		return dalgo2ingitdb.EncodeMapOfRecordsContent(data, col.RecordFile.Format, col.RecordFile.Name, col.ColumnsOrder)
	case ingitdb.SingleRecord:
		return dalgo2ingitdb.EncodeRecordContentForCollection(records[0].Fields, col)
	case ingitdb.ListOfRecords:
		return serializeListRecords(records, col)
	default:
		return nil, fmt.Errorf("unsupported record layout %q", col.RecordFile.RecordType)
	}
}

// serializeListRecords renders merged list records: CSV rows are written in
// merge order; INGR reuses the keyed map-of-records encoder.
func serializeListRecords(records []recordmerge.Record, col *ingitdb.CollectionDef) ([]byte, error) {
	switch col.RecordFile.Format {
	case ingitdb.RecordFormatCSV:
		rows := make([]map[string]any, len(records))
		for i, r := range records {
			rows[i] = r.Fields
		}
		return dalgo2ingitdb.EncodeRecordContentForCollection(rows, col)
	case ingitdb.RecordFormatINGR:
		data := make(map[string]map[string]any, len(records))
		for _, r := range records {
			data[r.Key] = r.Fields
		}
		return dalgo2ingitdb.EncodeMapOfRecordsContent(data, col.RecordFile.Format, col.RecordFile.Name, col.ColumnsOrder)
	default:
		return nil, fmt.Errorf("unsupported list format %q", col.RecordFile.Format)
	}
}
