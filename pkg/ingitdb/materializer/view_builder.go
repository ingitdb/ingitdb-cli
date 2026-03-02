package materializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// SimpleViewBuilder materializes view outputs using injected dependencies.
type SimpleViewBuilder struct {
	DefReader     ViewDefReader
	RecordsReader ingitdb.RecordsReader
	Writer        ViewWriter
	Logf          func(format string, args ...any)
}

func (b SimpleViewBuilder) BuildViews(
	ctx context.Context,
	dbPath string,
	repoRoot string,
	col *ingitdb.CollectionDef,
	def *ingitdb.Definition,
) (*ingitdb.MaterializeResult, error) {
	if b.DefReader == nil {
		return nil, fmt.Errorf("view definition reader is required")
	}
	if b.RecordsReader == nil {
		return nil, fmt.Errorf("records reader is required")
	}
	if b.Writer == nil {
		return nil, fmt.Errorf("view writer is required")
	}
	views, err := b.DefReader.ReadViewDefs(col.DirPath)
	if err != nil {
		return nil, err
	}
	// Inject the inline default_view from the collection definition.
	if col.DefaultView != nil {
		dv := *col.DefaultView
		dv.ID = ingitdb.DefaultViewID
		dv.IsDefault = true
		views[ingitdb.DefaultViewID] = &dv
	}
	result := &ingitdb.MaterializeResult{}
	for _, view := range views {
		records, err := readAllRecords(ctx, b.RecordsReader, dbPath, col)
		if err != nil {
			return nil, err
		}

		if view.IsDefault {
			// Handle default view export
			created, updated, unchanged, errs := buildDefaultView(dbPath, repoRoot, col, def, view, records, b.Logf)
			result.FilesCreated += created
			result.FilesUpdated += updated
			result.FilesUnchanged += unchanged
			result.Errors = append(result.Errors, errs...)
			continue
		}

		records = filterColumns(records, view.Columns)
		sortRecordsByOrderBy(records, view.OrderBy)
		if view.Top > 0 && len(records) > view.Top {
			records = records[:view.Top]
		}
		outPath := resolveViewOutputPath(col, view, dbPath, repoRoot)
		outcome, err := b.Writer.WriteView(ctx, col, view, records, outPath)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		switch outcome {
		case WriteOutcomeCreated:
			result.FilesCreated++
		case WriteOutcomeUpdated:
			result.FilesUpdated++
		default:
			result.FilesUnchanged++
		}
		if b.Logf != nil {
			b.Logf("Materializing view %s/%s... %d records saved to %s",
				col.ID, view.ID, len(records), displayRelPath(repoRoot, outPath))
		}
	}
	return result, nil
}

func (b SimpleViewBuilder) BuildView(
	ctx context.Context,
	dbPath string,
	repoRoot string,
	col *ingitdb.CollectionDef,
	def *ingitdb.Definition,
	view *ingitdb.ViewDef,
) (*ingitdb.MaterializeResult, error) {
	_ = def
	if b.RecordsReader == nil {
		return nil, fmt.Errorf("records reader is required")
	}
	if b.Writer == nil {
		return nil, fmt.Errorf("view writer is required")
	}

	result := &ingitdb.MaterializeResult{}

	records, err := readAllRecords(ctx, b.RecordsReader, dbPath, col)
	if err != nil {
		return nil, err
	}

	if view.IsDefault {
		// Handle default view export
		created, updated, unchanged, errs := buildDefaultView(dbPath, repoRoot, col, def, view, records, b.Logf)
		result.FilesCreated += created
		result.FilesUpdated += updated
		result.FilesUnchanged += unchanged
		result.Errors = append(result.Errors, errs...)
		return result, nil
	}

	records = filterColumns(records, view.Columns)
	sortRecordsByOrderBy(records, view.OrderBy)
	if view.Top > 0 && len(records) > view.Top {
		records = records[:view.Top]
	}
	outPath := resolveViewOutputPath(col, view, dbPath, repoRoot)
	outcome, err := b.Writer.WriteView(ctx, col, view, records, outPath)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, nil
	}
	switch outcome {
	case WriteOutcomeCreated:
		result.FilesCreated++
	case WriteOutcomeUpdated:
		result.FilesUpdated++
	default:
		result.FilesUnchanged++
	}
	if b.Logf != nil {
		b.Logf("Materializing view %s/%s... %d records saved to %s",
			col.ID, view.ID, len(records), displayRelPath(repoRoot, outPath))
	}

	return result, nil
}

func readAllRecords(
	ctx context.Context,
	reader ingitdb.RecordsReader,
	dbPath string,
	col *ingitdb.CollectionDef,
) ([]ingitdb.IRecordEntry, error) {
	var records []ingitdb.IRecordEntry
	err := reader.ReadRecords(ctx, dbPath, col, func(entry ingitdb.IRecordEntry) error {
		records = append(records, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func filterColumns(records []ingitdb.IRecordEntry, cols []string) []ingitdb.IRecordEntry {
	if len(cols) == 0 {
		return records
	}
	allowed := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		allowed[col] = struct{}{}
	}
	filtered := make([]ingitdb.IRecordEntry, 0, len(records))
	for _, record := range records {
		d := record.GetData()
		if d == nil {
			filtered = append(filtered, record)
			continue
		}
		data := make(map[string]any, len(cols))
		for key := range allowed {
			if value, ok := d[key]; ok {
				data[key] = value
			}
		}
		filtered = append(filtered, ingitdb.NewMapRecordEntry(record.GetID(), data))
	}
	return filtered
}

func buildDefaultView(dbPath string, repoRoot string, col *ingitdb.CollectionDef, def *ingitdb.Definition, view *ingitdb.ViewDef, records []ingitdb.IRecordEntry, logf func(string, ...any)) (created, updated, unchanged int, errs []error) {
	columns := determineColumns(col, view)
	format := strings.ToLower(view.Format)
	ext := defaultViewFormatExtension(format)
	base := view.FileName
	if base == "" {
		base = col.ID
	}

	outputRoot := repoRoot
	if outputRoot == "" {
		outputRoot = dbPath
	}

	// Determine batches
	totalBatches := 1
	batchSize := view.MaxBatchSize
	if batchSize > 0 && len(records) > batchSize {
		totalBatches = (len(records) + batchSize - 1) / batchSize
	}

	for batchNum := 1; batchNum <= totalBatches; batchNum++ {
		var batchRecords []ingitdb.IRecordEntry
		if totalBatches == 1 {
			batchRecords = records
		} else {
			start := (batchNum - 1) * batchSize
			end := start + batchSize
			if end > len(records) {
				end = len(records)
			}
			batchRecords = records[start:end]
		}

		var exportOpts []ExportOption
		if view.IncludeHash {
			exportOpts = append(exportOpts, WithHash())
		}
		recordsDelimiter := view.RecordsDelimiter || def.Settings.RecordsDelimiter
		if def.RuntimeOverrides.RecordsDelimiter != nil {
			recordsDelimiter = *def.RuntimeOverrides.RecordsDelimiter
		}
		if recordsDelimiter {
			exportOpts = append(exportOpts, WithRecordsDelimiter())
		}
		content, err := formatExportBatch(format, col.ID+"/"+view.ID, columns, batchRecords, exportOpts...)
		if err != nil {
			errs = append(errs, fmt.Errorf("batch %d: %w", batchNum, err))
			continue
		}

		fileName := formatBatchFileName(base, ext, batchNum, totalBatches)
		relColPath, _ := filepath.Rel(outputRoot, col.DirPath)
		outPath := filepath.Join(outputRoot, ingitdb.IngitdbDir, relColPath, fileName)

		if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
			errs = append(errs, fmt.Errorf("mkdir for %s: %w", outPath, err))
			continue
		}

		existing, readErr := os.ReadFile(outPath)
		if readErr == nil && bytes.Equal(existing, content) {
			unchanged++
			if logf != nil {
				logf("Materializing view %s/%s... %d records saved to %s",
					col.ID, view.ID, len(batchRecords), displayRelPath(repoRoot, outPath))
			}
			continue
		}

		if err := os.WriteFile(outPath, content, 0o644); err != nil {
			errs = append(errs, fmt.Errorf("write %s: %w", outPath, err))
			continue
		}
		if readErr == nil {
			updated++
		} else {
			created++
		}
		if logf != nil {
			logf("Materializing view %s/%s... %d records saved to %s",
				col.ID, view.ID, len(batchRecords), displayRelPath(repoRoot, outPath))
		}
	}
	return
}

func resolveViewOutputPath(col *ingitdb.CollectionDef, view *ingitdb.ViewDef, dbPath, repoRoot string) string {
	relPath, _ := filepath.Rel(dbPath, col.DirPath)
	if view.IsDefault {
		base := view.FileName
		if base == "" {
			base = col.ID
		}
		ext := defaultViewFormatExtension(strings.ToLower(view.Format))
		return filepath.Join(repoRoot, ingitdb.IngitdbDir, relPath, base+"."+ext)
	}
	// Template-rendered views (e.g. README.md) live in the collection directory itself.
	if view.Template != "" {
		if view.FileName != "" {
			return filepath.Join(col.DirPath, view.FileName)
		}
		name := view.ID
		if name == "" {
			name = "view"
		}
		return filepath.Join(col.DirPath, name+".md")
	}
	// Data-export views go to $ingitdb/
	if view.FileName != "" {
		return filepath.Join(repoRoot, ingitdb.IngitdbDir, relPath, view.FileName)
	}
	name := view.ID
	if name == "" {
		name = "view"
	}
	ext := defaultViewFormatExtension(strings.ToLower(view.Format))
	return filepath.Join(repoRoot, ingitdb.IngitdbDir, relPath, name+"."+ext)
}

// displayRelPath returns outPath relative to repoRoot for display, or outPath if unavailable.
func displayRelPath(repoRoot, outPath string) string {
	if repoRoot != "" {
		if rel, err := filepath.Rel(repoRoot, outPath); err == nil {
			return rel
		}
	}
	return outPath
}

// sortRecordsByOrderBy sorts records in-place according to the orderBy expression.
// Format: "<field> [asc|desc]" (case-insensitive; default is ascending).
// No-op when orderBy is empty.
func sortRecordsByOrderBy(records []ingitdb.IRecordEntry, orderBy string) {
	orderBy = strings.TrimSpace(orderBy)
	if orderBy == "" {
		return
	}
	parts := strings.Fields(orderBy)
	field := parts[0]
	desc := len(parts) >= 2 && strings.EqualFold(parts[1], "desc")

	sort.SliceStable(records, func(i, j int) bool {
		vi := recordFieldValue(records[i], field)
		vj := recordFieldValue(records[j], field)
		cmp := compareAny(vi, vj)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
}

// recordFieldValue returns the value of a field from a record's Data map, or nil if absent.
func recordFieldValue(r ingitdb.IRecordEntry, field string) any {
	d := r.GetData()
	if d == nil {
		return nil
	}
	return d[field]
}

// compareAny compares two values of arbitrary type.
// Returns negative, zero, or positive for less-than, equal, greater-than.
func compareAny(a, b any) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}
	switch av := a.(type) {
	case int:
		if bv, ok := toComparableInt(b); ok {
			return cmpInts(int64(av), bv)
		}
	case int64:
		if bv, ok := toComparableInt(b); ok {
			return cmpInts(av, bv)
		}
	case float64:
		if bv, ok := toComparableFloat(b); ok {
			return cmpFloats(av, bv)
		}
	case string:
		if bv, ok := b.(string); ok {
			if av < bv {
				return -1
			}
			if av > bv {
				return 1
			}
			return 0
		}
	}
	// fallback: compare as formatted strings
	as := fmt.Sprintf("%v", a)
	bs := fmt.Sprintf("%v", b)
	if as < bs {
		return -1
	}
	if as > bs {
		return 1
	}
	return 0
}

func toComparableInt(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int64:
		return n, true
	case float64:
		return int64(n), true
	}
	return 0, false
}

func toComparableFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func cmpInts(a, b int64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func cmpFloats(a, b float64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
