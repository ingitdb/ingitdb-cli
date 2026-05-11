package dalgo2fsingitdb

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// executeQueryToRecordsReader reads all records for the collection referenced by
// query.From(), applies WHERE / ORDER BY / LIMIT in-memory, and returns a
// slice-backed RecordsReader.
func executeQueryToRecordsReader(ctx context.Context, r readonlyTx, query dal.Query) (dal.RecordsReader, error) {
	_ = ctx
	if r.db.def == nil {
		return nil, fmt.Errorf("definition is required: use NewLocalDBWithDef")
	}

	sq, ok := query.(dal.StructuredQuery)
	if !ok {
		return nil, fmt.Errorf("only StructuredQuery is supported")
	}

	// Extract collection ID from the From clause.
	fromSrc := sq.From()
	if fromSrc == nil {
		return nil, fmt.Errorf("query has no FROM clause")
	}
	base := fromSrc.Base()
	colRef, ok := base.(dal.CollectionRef)
	if !ok {
		return nil, fmt.Errorf("FROM source must be a CollectionRef, got %T", base)
	}
	collectionID := colRef.Name()

	colDef, exists := r.db.def.Collections[collectionID]
	if !exists {
		return nil, fmt.Errorf("collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, fmt.Errorf("collection %q has no record_file definition", collectionID)
	}

	// Read all records from disk.
	allRecords, err := readAllRecordsFromDisk(colDef)
	if err != nil {
		return nil, err
	}

	// Apply WHERE filter.
	condition := sq.Where()
	if condition != nil {
		filtered := allRecords[:0]
		for _, rec := range allRecords {
			data := rec.Data().(map[string]any)
			recKey := fmt.Sprintf("%v", rec.Key().ID)
			match, evalErr := evaluateCondition(condition, data, recKey)
			if evalErr != nil {
				return nil, evalErr
			}
			if match {
				filtered = append(filtered, rec)
			}
		}
		allRecords = filtered
	}

	// Apply ORDER BY.
	orderBy := sq.OrderBy()
	if len(orderBy) > 0 {
		sort.SliceStable(allRecords, func(i, j int) bool {
			dataI := allRecords[i].Data().(map[string]any)
			dataJ := allRecords[j].Data().(map[string]any)
			keyI := fmt.Sprintf("%v", allRecords[i].Key().ID)
			keyJ := fmt.Sprintf("%v", allRecords[j].Key().ID)
			for _, expr := range orderBy {
				fieldRef, isRef := expr.Expression().(dal.FieldRef)
				if !isRef {
					continue
				}
				fieldName := fieldRef.Name()
				var vI, vJ any
				if fieldName == "$id" {
					vI, vJ = keyI, keyJ
				} else {
					vI = dataI[fieldName]
					vJ = dataJ[fieldName]
				}
				cmp := compareValues(vI, vJ)
				if cmp == 0 {
					continue
				}
				if expr.Descending() {
					return cmp > 0
				}
				return cmp < 0
			}
			return false
		})
	}

	// Apply LIMIT.
	limit := sq.Limit()
	if limit > 0 && len(allRecords) > limit {
		allRecords = allRecords[:limit]
	}

	return newSliceRecordsReader(allRecords), nil
}

// readAllRecordsFromDisk loads every record in colDef from the filesystem.
func readAllRecordsFromDisk(colDef *ingitdb.CollectionDef) ([]dal.Record, error) {
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return readAllSingleRecords(colDef)
	case ingitdb.MapOfRecords:
		return readAllMapOfRecords(colDef)
	default:
		return nil, fmt.Errorf("unsupported record type %q for query", colDef.RecordFile.RecordType)
	}
}

// readAllSingleRecords globs for all record files and reads each one.
func readAllSingleRecords(colDef *ingitdb.CollectionDef) ([]dal.Record, error) {
	nameTemplate := colDef.RecordFile.Name
	globPattern := strings.ReplaceAll(nameTemplate, "{key}", "*")
	basePath := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	matches, err := filepath.Glob(filepath.Join(basePath, globPattern))
	if err != nil {
		return nil, fmt.Errorf("failed to glob records in %s: %w", basePath, err)
	}

	keyExtractor, err := buildKeyExtractor(nameTemplate)
	if err != nil {
		return nil, err
	}

	records := make([]dal.Record, 0, len(matches))
	for _, match := range matches {
		if colDef.RecordFile.IsExcluded(filepath.Base(match)) {
			continue
		}
		relPath, relErr := filepath.Rel(basePath, match)
		if relErr != nil {
			return nil, relErr
		}
		recordKey := keyExtractor(relPath)
		if recordKey == "" {
			continue
		}
		var (
			data    map[string]any
			found   bool
			readErr error
		)
		if colDef.RecordFile.Format == ingitdb.RecordFormatMarkdown {
			data, found, readErr = readMarkdownRecord(match, colDef)
		} else {
			data, found, readErr = readRecordFromFile(match, colDef.RecordFile.Format)
		}
		if readErr != nil {
			return nil, readErr
		}
		if !found {
			continue
		}
		key := dal.NewKeyWithID(colDef.ID, recordKey)
		rec := dal.NewRecordWithData(key, data)
		rec.SetError(nil)
		records = append(records, rec)
	}
	return records, nil
}

// readAllMapOfRecords reads a single map-of-records file and returns all entries.
func readAllMapOfRecords(colDef *ingitdb.CollectionDef) ([]dal.Record, error) {
	path := resolveRecordPath(colDef, "")
	allData, found, err := readMapOfRecordsFile(path, colDef.RecordFile.Format)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	records := make([]dal.Record, 0, len(allData))
	for id, fields := range allData {
		normalized := dalgo2ingitdb.ApplyLocaleToRead(fields, colDef.Columns)
		key := dal.NewKeyWithID(colDef.ID, id)
		rec := dal.NewRecordWithData(key, normalized)
		rec.SetError(nil)
		records = append(records, rec)
	}
	return records, nil
}

// buildKeyExtractor creates a function that extracts the record key from a path
// relative to the records base directory, using the name template.
func buildKeyExtractor(nameTemplate string) (func(relPath string) string, error) {
	idx := strings.Index(nameTemplate, "{key}")
	if idx < 0 {
		// Fixed filename — derive key from the base of the relative path
		return func(relPath string) string {
			return strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
		}, nil
	}

	// Build a regex: escape the template, replace first {key} with a capture group,
	// replace remaining {key} occurrences with .*.
	prefix := regexp.QuoteMeta(nameTemplate[:idx])
	remainder := nameTemplate[idx+len("{key}"):]
	suffix := regexp.QuoteMeta(strings.ReplaceAll(remainder, "{key}", "\x00"))
	suffix = strings.ReplaceAll(suffix, "\x00", ".*")
	// On Windows, filepath.Glob uses OS separator; keep posix separators in regex.
	re, err := regexp.Compile("^" + prefix + "(.*?)" + suffix + "$")
	if err != nil {
		return nil, fmt.Errorf("failed to build key extractor regex from template %q: %w", nameTemplate, err)
	}
	return func(relPath string) string {
		// Normalize to forward slashes for consistent regex matching.
		normalised := filepath.ToSlash(relPath)
		m := re.FindStringSubmatch(normalised)
		if m == nil {
			return ""
		}
		return m[1]
	}, nil
}

// evaluateCondition evaluates a dal.Condition against a record's field map.
func evaluateCondition(cond dal.Condition, data map[string]any, recordKey string) (bool, error) {
	switch c := cond.(type) {
	case dal.Comparison:
		return evaluateComparison(c, data, recordKey)
	case dal.GroupCondition:
		return evaluateGroupCondition(c, data, recordKey)
	default:
		return false, fmt.Errorf("unsupported condition type %T", cond)
	}
}

func evaluateGroupCondition(gc dal.GroupCondition, data map[string]any, recordKey string) (bool, error) {
	switch gc.Operator() {
	case dal.And:
		for _, cond := range gc.Conditions() {
			match, err := evaluateCondition(cond, data, recordKey)
			if err != nil {
				return false, err
			}
			if !match {
				return false, nil
			}
		}
		return true, nil
	case dal.Or:
		for _, cond := range gc.Conditions() {
			match, err := evaluateCondition(cond, data, recordKey)
			if err != nil {
				return false, err
			}
			if match {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported group operator %q", gc.Operator())
	}
}

func evaluateComparison(c dal.Comparison, data map[string]any, recordKey string) (bool, error) {
	leftVal, err := resolveExpression(c.Left, data, recordKey)
	if err != nil {
		return false, err
	}
	rightVal, err := resolveExpression(c.Right, data, recordKey)
	if err != nil {
		return false, err
	}

	cmp := compareValues(leftVal, rightVal)
	switch c.Operator {
	case dal.Equal:
		return cmp == 0, nil
	case dal.GreaterThen:
		return cmp > 0, nil
	case dal.GreaterOrEqual:
		return cmp >= 0, nil
	case dal.LessThen:
		return cmp < 0, nil
	case dal.LessOrEqual:
		return cmp <= 0, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", c.Operator)
	}
}

// resolveExpression extracts the value for a FieldRef or Constant expression.
func resolveExpression(expr dal.Expression, data map[string]any, recordKey string) (any, error) {
	switch e := expr.(type) {
	case dal.FieldRef:
		if e.Name() == "$id" {
			return recordKey, nil
		}
		return data[e.Name()], nil
	case dal.Constant:
		return e.Value, nil
	default:
		return nil, fmt.Errorf("unsupported expression type %T", expr)
	}
}

// compareValues compares two values returning -1, 0, or 1.
// Numbers are compared as float64; strings lexicographically; mixed types fall back to string comparison.
func compareValues(a, b any) int {
	aFloat, aIsNum := toFloat64(a)
	bFloat, bIsNum := toFloat64(b)
	if aIsNum && bIsNum {
		switch {
		case aFloat < bFloat:
			return -1
		case aFloat > bFloat:
			return 1
		default:
			return 0
		}
	}
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Compare(aStr, bStr)
}

// toFloat64 converts numeric types to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
