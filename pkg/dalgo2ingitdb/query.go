package dalgo2ingitdb

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// executeQueryToRecordsReader reads every record in the collection
// referenced by query.From(), applies WHERE / ORDER BY / LIMIT in memory,
// and returns a slice-backed RecordsReader. Mirrors the algorithm in
// dalgo2fsingitdb but uses this package's locked record I/O helpers.
func executeQueryToRecordsReader(_ context.Context, r readonlyTx, query dal.Query) (dal.RecordsReader, error) {
	if r.def == nil {
		return nil, fmt.Errorf("dalgo2ingitdb: transaction has no loaded definition")
	}
	sq, ok := query.(dal.StructuredQuery)
	if !ok {
		return nil, fmt.Errorf("dalgo2ingitdb: only StructuredQuery is supported, got %T", query)
	}
	fromSrc := sq.From()
	if fromSrc == nil {
		return nil, fmt.Errorf("dalgo2ingitdb: query has no FROM clause")
	}
	base := fromSrc.Base()
	colRef, ok := base.(dal.CollectionRef)
	if !ok {
		return nil, fmt.Errorf("dalgo2ingitdb: FROM source must be a CollectionRef, got %T", base)
	}
	collectionID := colRef.Name()
	colDef, exists := r.def.Collections[collectionID]
	if !exists {
		return nil, fmt.Errorf("dalgo2ingitdb: collection %q not found in definition", collectionID)
	}
	if colDef.RecordFile == nil {
		return nil, fmt.Errorf("dalgo2ingitdb: collection %q has no record_file definition", collectionID)
	}

	records, err := readAllRecordsFromDisk(colDef)
	if err != nil {
		return nil, err
	}

	if cond := sq.Where(); cond != nil {
		records, err = applyWhere(records, cond)
		if err != nil {
			return nil, err
		}
	}
	if orderBy := sq.OrderBy(); len(orderBy) > 0 {
		applyOrderBy(records, orderBy)
	}
	if limit := sq.Limit(); limit > 0 && len(records) > limit {
		records = records[:limit]
	}
	return newSliceRecordsReader(records), nil
}

func readAllRecordsFromDisk(colDef *ingitdb.CollectionDef) ([]dal.Record, error) {
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return readAllSingleRecords(colDef)
	case ingitdb.MapOfRecords:
		return readAllMapOfRecords(colDef)
	default:
		return nil, fmt.Errorf("dalgo2ingitdb: query unsupported for record type %q", colDef.RecordFile.RecordType)
	}
}

func readAllSingleRecords(colDef *ingitdb.CollectionDef) ([]dal.Record, error) {
	nameTemplate := colDef.RecordFile.Name
	globPattern := strings.ReplaceAll(nameTemplate, "{key}", "*")
	basePath := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	matches, err := filepath.Glob(filepath.Join(basePath, globPattern))
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", basePath, err)
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
			return nil, fmt.Errorf("rel %s: %w", match, relErr)
		}
		recordKey := keyExtractor(relPath)
		if recordKey == "" {
			continue
		}
		data, found, readErr := readSingleRecordFile(match, colDef)
		if readErr != nil {
			return nil, readErr
		}
		if !found {
			continue
		}
		key := dal.NewKeyWithID(colDef.ID, recordKey)
		rec := dal.NewRecordWithData(key, ApplyLocaleToRead(data, colDef.Columns))
		rec.SetError(nil)
		records = append(records, rec)
	}
	return records, nil
}

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
		normalized := ApplyLocaleToRead(fields, colDef.Columns)
		key := dal.NewKeyWithID(colDef.ID, id)
		rec := dal.NewRecordWithData(key, normalized)
		rec.SetError(nil)
		records = append(records, rec)
	}
	return records, nil
}

// buildKeyExtractor returns a function that recovers the record key from
// a path relative to the records base directory.
func buildKeyExtractor(nameTemplate string) (func(relPath string) string, error) {
	idx := strings.Index(nameTemplate, "{key}")
	if idx < 0 {
		return func(relPath string) string {
			return strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
		}, nil
	}
	prefix := regexp.QuoteMeta(nameTemplate[:idx])
	remainder := nameTemplate[idx+len("{key}"):]
	suffix := regexp.QuoteMeta(strings.ReplaceAll(remainder, "{key}", "\x00"))
	suffix = strings.ReplaceAll(suffix, "\x00", ".*")
	re, err := regexp.Compile("^" + prefix + "(.*?)" + suffix + "$")
	if err != nil {
		return nil, fmt.Errorf("build key extractor for %q: %w", nameTemplate, err)
	}
	return func(relPath string) string {
		normalised := filepath.ToSlash(relPath)
		m := re.FindStringSubmatch(normalised)
		if m == nil {
			return ""
		}
		return m[1]
	}, nil
}

func applyWhere(records []dal.Record, cond dal.Condition) ([]dal.Record, error) {
	filtered := records[:0]
	for _, rec := range records {
		data := rec.Data().(map[string]any)
		recKey := fmt.Sprintf("%v", rec.Key().ID)
		match, err := evaluateCondition(cond, data, recKey)
		if err != nil {
			return nil, err
		}
		if match {
			filtered = append(filtered, rec)
		}
	}
	return filtered, nil
}

func applyOrderBy(records []dal.Record, orderBy []dal.OrderExpression) {
	sort.SliceStable(records, func(i, j int) bool {
		dataI := records[i].Data().(map[string]any)
		dataJ := records[j].Data().(map[string]any)
		keyI := fmt.Sprintf("%v", records[i].Key().ID)
		keyJ := fmt.Sprintf("%v", records[j].Key().ID)
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

func evaluateCondition(cond dal.Condition, data map[string]any, recordKey string) (bool, error) {
	switch c := cond.(type) {
	case dal.Comparison:
		return evaluateComparison(c, data, recordKey)
	case dal.GroupCondition:
		return evaluateGroupCondition(c, data, recordKey)
	default:
		return false, fmt.Errorf("dalgo2ingitdb: unsupported condition type %T", cond)
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
		return false, fmt.Errorf("dalgo2ingitdb: unsupported group operator %q", gc.Operator())
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
		return false, fmt.Errorf("dalgo2ingitdb: unsupported operator %q", c.Operator)
	}
}

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
		return nil, fmt.Errorf("dalgo2ingitdb: unsupported expression type %T", expr)
	}
}

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
