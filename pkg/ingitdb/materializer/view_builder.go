package materializer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
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
) ([]ingitdb.RecordEntry, error) {
	var records []ingitdb.RecordEntry
	err := reader.ReadRecords(ctx, dbPath, col, func(entry ingitdb.RecordEntry) error {
		records = append(records, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return records, nil
}

func filterColumns(records []ingitdb.RecordEntry, cols []string) []ingitdb.RecordEntry {
	if len(cols) == 0 {
		return records
	}
	allowed := make(map[string]struct{}, len(cols))
	for _, col := range cols {
		allowed[col] = struct{}{}
	}
	filtered := make([]ingitdb.RecordEntry, 0, len(records))
	for _, record := range records {
		if record.Data == nil {
			filtered = append(filtered, record)
			continue
		}
		data := make(map[string]any, len(cols))
		for key := range allowed {
			if value, ok := record.Data[key]; ok {
				data[key] = value
			}
		}
		record.Data = data
		filtered = append(filtered, record)
	}
	return filtered
}

func buildDefaultView(dbPath string, repoRoot string, col *ingitdb.CollectionDef, def *ingitdb.Definition, view *ingitdb.ViewDef, records []ingitdb.RecordEntry, logf func(string, ...any)) (created, updated, unchanged int, errs []error) {
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
		var batchRecords []ingitdb.RecordEntry
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
