// specscore: feature/cli/resolve/auto-resolve/record-merge
package recordmerge

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// MergeFiles parses the BASE/OURS/THEIRS bytes of a conflicted record file into
// typed records and runs the three-way merge. A nil or empty stage is treated
// as an absent record set (added on one side, or deleted).
//
// It returns an Outcome whose Merged holds the union of non-conflicting changes
// on success, or Escalate=true (with a reason) when the conflict is not
// auto-resolvable — including unsupported record layouts and parse failures.
// Serialization of the merged records back to file bytes is the caller's
// responsibility.
func MergeFiles(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	if col == nil || col.RecordFile == nil {
		return escalate("collection has no record-file definition")
	}

	switch col.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		return mergeMapOfRecords(base, ours, theirs, col, opts)
	case ingitdb.SingleRecord:
		return mergeSingleRecord(base, ours, theirs, col, opts)
	case ingitdb.ListOfRecords:
		return mergeListOfRecords(base, ours, theirs, col, opts)
	default:
		return escalate("record layout %q is not auto-mergeable yet", col.RecordFile.RecordType)
	}
}

// mergeListOfRecords handles the list layout. CSV rows are keyed by the
// collection's key column(s); INGR records are keyed by their `$ID` and reuse
// the map-of-records path.
func mergeListOfRecords(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	switch col.RecordFile.Format {
	case ingitdb.RecordFormatCSV:
		return mergeListCSV(base, ours, theirs, col, opts)
	case ingitdb.RecordFormatINGR:
		return mergeMapOfRecords(base, ours, theirs, col, opts)
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML,
		ingitdb.RecordFormatJSON, ingitdb.RecordFormatJSONL:
		return mergeListSequence(base, ours, theirs, col, opts)
	default:
		return escalate("list layout with format %q is not auto-mergeable yet", col.RecordFile.Format)
	}
}

// parseSequenceRecords parses one stage of a YAML/JSON/JSONL list into keyed
// records using the shared key-resolution rule. Empty content yields no
// records; a row with no resolvable key is an error (the conflict escalates).
func parseSequenceRecords(content []byte, col *ingitdb.CollectionDef) ([]Record, error) {
	if len(content) == 0 {
		return nil, nil
	}
	rows, err := dalgo2ingitdb.ParseListOfRecordsContent(content, col.RecordFile.Format)
	if err != nil {
		return nil, err
	}
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		key, ok := dalgo2ingitdb.ResolveListRecordKey(row, col)
		if !ok {
			return nil, fmt.Errorf("list record has no resolvable key")
		}
		records = append(records, Record{Key: key, Fields: row})
	}
	return records, nil
}

func mergeListSequence(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	b, err1 := parseSequenceRecords(base, col)
	o, err2 := parseSequenceRecords(ours, col)
	t, err3 := parseSequenceRecords(theirs, col)
	if err := firstErr(err1, err2, err3); err != nil {
		return escalate("failed to parse a conflict side: %v", err)
	}
	return Merge(b, o, t, opts)
}

// csvKeyColumns returns the column name(s) that identify a CSV row: the
// declared primary key, else a `$id` or `id` column, else nil when none is
// available (in which case the conflict escalates).
func csvKeyColumns(col *ingitdb.CollectionDef) []string {
	if len(col.PrimaryKey) > 0 {
		return col.PrimaryKey
	}
	for _, candidate := range []string{"$id", "id"} {
		if slices.Contains(col.ColumnsOrder, candidate) {
			return []string{candidate}
		}
	}
	return nil
}

// parseCSVRecords parses one stage of a CSV list into keyed records, joining
// the key-column values into each record's Key. Empty content yields no
// records.
func parseCSVRecords(content []byte, col *ingitdb.CollectionDef, keyCols []string) ([]Record, error) {
	if len(content) == 0 {
		return nil, nil
	}
	parsed, err := dalgo2ingitdb.ParseRecordContentForCollection(content, col)
	if err != nil {
		return nil, err
	}
	rows, _ := parsed["$records"].([]map[string]any)
	records := make([]Record, 0, len(rows))
	for _, row := range rows {
		parts := make([]string, len(keyCols))
		for i, kc := range keyCols {
			parts[i] = fmt.Sprintf("%v", row[kc])
		}
		records = append(records, Record{Key: strings.Join(parts, "\x1f"), Fields: row})
	}
	return records, nil
}

func mergeListCSV(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	keyCols := csvKeyColumns(col)
	if keyCols == nil {
		return escalate("csv list has no usable key column (set primary_key or a $id/id column)")
	}
	b, err1 := parseCSVRecords(base, col, keyCols)
	o, err2 := parseCSVRecords(ours, col, keyCols)
	t, err3 := parseCSVRecords(theirs, col, keyCols)
	if err := firstErr(err1, err2, err3); err != nil {
		return escalate("failed to parse a conflict side: %v", err)
	}
	return Merge(b, o, t, opts)
}

// parseMap parses one stage of a map-of-records file into ordered records
// (sorted by key for determinism). Empty content yields no records.
func parseMap(content []byte, format ingitdb.RecordFormat) ([]Record, error) {
	if len(content) == 0 {
		return nil, nil
	}
	m, err := dalgo2ingitdb.ParseMapOfRecordsContent(content, format)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	records := make([]Record, 0, len(m))
	for _, k := range keys {
		records = append(records, Record{Key: k, Fields: m[k]})
	}
	return records, nil
}

func mergeMapOfRecords(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	format := col.RecordFile.Format
	b, err1 := parseMap(base, format)
	o, err2 := parseMap(ours, format)
	t, err3 := parseMap(theirs, format)
	if err := firstErr(err1, err2, err3); err != nil {
		return escalate("failed to parse a conflict side: %v", err)
	}
	return Merge(b, o, t, opts)
}

// parseSingle parses one stage of a single-record file into at most one record
// (keyed by the empty string). Empty content yields no record.
func parseSingle(content []byte, col *ingitdb.CollectionDef) ([]Record, error) {
	if len(content) == 0 {
		return nil, nil
	}
	m, err := dalgo2ingitdb.ParseRecordContentForCollection(content, col)
	if err != nil {
		return nil, err
	}
	return []Record{{Key: "", Fields: m}}, nil
}

func mergeSingleRecord(base, ours, theirs []byte, col *ingitdb.CollectionDef, opts Options) Outcome {
	b, err1 := parseSingle(base, col)
	o, err2 := parseSingle(ours, col)
	t, err3 := parseSingle(theirs, col)
	if err := firstErr(err1, err2, err3); err != nil {
		return escalate("failed to parse a conflict side: %v", err)
	}

	outcome := Merge(b, o, t, opts)
	if outcome.Escalate {
		return outcome
	}
	if len(outcome.Merged) != 1 {
		return escalate("single-record merge did not yield exactly one record")
	}
	return outcome
}

func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
