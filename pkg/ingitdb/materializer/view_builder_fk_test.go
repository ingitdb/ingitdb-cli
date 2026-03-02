package materializer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeCompaniesCol builds a CollectionDef for companies with a country FK column.
func makeCompaniesCol(t *testing.T, dirPath string) *ingitdb.CollectionDef {
	t.Helper()
	return &ingitdb.CollectionDef{
		ID:           "companies",
		DirPath:      dirPath,
		ColumnsOrder: []string{"id", "name", "country"},
		Columns: map[string]*ingitdb.ColumnDef{
			"name":    {Type: ingitdb.ColumnTypeString},
			"country": {Type: ingitdb.ColumnTypeString, ForeignKey: "countries"},
		},
	}
}

// makeDefaultView builds a simple default ViewDef with the given format.
func makeDefaultView(format string) *ingitdb.ViewDef {
	return &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    format,
	}
}

// fkFilePath builds the expected FK view file path.
func fkFilePath(outputRoot, relColPath, colName, fkCollection, fkValue, ext string) string {
	return filepath.Join(outputRoot, ingitdb.IngitdbDir, relColPath, "$fk_"+colName, fkCollection, fkValue+"."+ext)
}

// TestBuildFKViews_HappyPath_SingleFKColumn verifies that two distinct FK values produce
// two separate files with the correct records each.
func TestBuildFKViews_HappyPath_SingleFKColumn(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
		ingitdb.NewMapRecordEntry("bmo", map[string]any{"id": "bmo", "name": "BMO", "country": "ca"}),
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 2 {
		t.Errorf("expected 2 files created, got %d", created)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if unchanged != 0 {
		t.Errorf("expected 0 unchanged, got %d", unchanged)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "json")
	caPath := fkFilePath(tmpDir, "companies", "country", "countries", "ca", "json")

	gbContent, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.json not found: %v", err)
	}
	if !strings.Contains(string(gbContent), "acme") {
		t.Errorf("gb.json should contain acme, got: %s", string(gbContent))
	}
	if strings.Contains(string(gbContent), "shopify") || strings.Contains(string(gbContent), "bmo") {
		t.Errorf("gb.json should not contain ca records, got: %s", string(gbContent))
	}

	caContent, err := os.ReadFile(caPath)
	if err != nil {
		t.Fatalf("ca.json not found: %v", err)
	}
	if !strings.Contains(string(caContent), "shopify") || !strings.Contains(string(caContent), "bmo") {
		t.Errorf("ca.json should contain shopify and bmo, got: %s", string(caContent))
	}
	if strings.Contains(string(caContent), "acme") {
		t.Errorf("ca.json should not contain acme, got: %s", string(caContent))
	}
}

// TestBuildFKViews_NullAndEmptyFKValuesSkipped verifies that nil and empty-string FK values
// are not written to any FK view file.
func TestBuildFKViews_NullAndEmptyFKValuesSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("r1", map[string]any{"id": "r1", "name": "US Co", "country": "us"}),
		ingitdb.NewMapRecordEntry("r2", map[string]any{"id": "r2", "name": "No Country", "country": ""}),
		ingitdb.NewMapRecordEntry("r3", map[string]any{"id": "r3", "name": "Nil Country", "country": nil}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Errorf("expected 1 file created (only us), got %d", created)
	}

	usPath := fkFilePath(tmpDir, "companies", "country", "countries", "us", "json")
	if _, err := os.Stat(usPath); err != nil {
		t.Errorf("us.json should exist: %v", err)
	}

	fkDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "companies", "$fk_country", "countries")
	entries, err := os.ReadDir(fkDir)
	if err != nil {
		t.Fatalf("readdir fkDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected exactly 1 FK view file, got %d: %v", len(entries), entries)
	}
}

// TestBuildFKViews_MultipleFKColumns verifies that independent $fk_* subtrees are created
// for each FK column.
func TestBuildFKViews_MultipleFKColumns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "employees",
		DirPath:      filepath.Join(tmpDir, "employees"),
		ColumnsOrder: []string{"id", "name", "country", "department"},
		Columns: map[string]*ingitdb.ColumnDef{
			"name":       {Type: ingitdb.ColumnTypeString},
			"country":    {Type: ingitdb.ColumnTypeString, ForeignKey: "countries"},
			"department": {Type: ingitdb.ColumnTypeString, ForeignKey: "departments"},
		},
	}
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("alice", map[string]any{"id": "alice", "name": "Alice", "country": "gb", "department": "eng"}),
		ingitdb.NewMapRecordEntry("bob", map[string]any{"id": "bob", "name": "Bob", "country": "ca", "department": "eng"}),
		ingitdb.NewMapRecordEntry("carol", map[string]any{"id": "carol", "name": "Carol", "country": "gb", "department": "hr"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// country: 2 values (gb, ca), department: 2 values (eng, hr) = 4 files total
	if created != 4 {
		t.Errorf("expected 4 files created (2 country + 2 department), got %d", created)
	}

	// Verify both FK subtrees exist.
	countryDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "employees", "$fk_country")
	deptDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "employees", "$fk_department")

	if _, err := os.Stat(countryDir); err != nil {
		t.Errorf("$fk_country subtree should exist: %v", err)
	}
	if _, err := os.Stat(deptDir); err != nil {
		t.Errorf("$fk_department subtree should exist: %v", err)
	}

	// Verify specific files.
	gbPath := fkFilePath(tmpDir, "employees", "country", "countries", "gb", "json")
	engPath := fkFilePath(tmpDir, "employees", "department", "departments", "eng", "json")
	if _, err := os.Stat(gbPath); err != nil {
		t.Errorf("gb.json should exist: %v", err)
	}
	if _, err := os.Stat(engPath); err != nil {
		t.Errorf("eng.json should exist: %v", err)
	}
}

// TestBuildFKViews_Idempotency verifies that a second run with identical records produces
// unchanged==N, created==0, updated==0.
func TestBuildFKViews_Idempotency(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
	}

	// First run.
	created1, updated1, unchanged1, errs1 := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs1) > 0 {
		t.Fatalf("first run errors: %v", errs1)
	}
	if created1 != 2 {
		t.Fatalf("first run: expected 2 created, got %d", created1)
	}
	if updated1+unchanged1 != 0 {
		t.Fatalf("first run: expected 0 updated/unchanged, got updated=%d unchanged=%d", updated1, unchanged1)
	}

	// Second run — identical records.
	created2, updated2, unchanged2, errs2 := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs2) > 0 {
		t.Fatalf("second run errors: %v", errs2)
	}
	if created2 != 0 {
		t.Errorf("second run: expected 0 created, got %d", created2)
	}
	if updated2 != 0 {
		t.Errorf("second run: expected 0 updated, got %d", updated2)
	}
	if unchanged2 != 2 {
		t.Errorf("second run: expected 2 unchanged, got %d", unchanged2)
	}
}

// TestBuildFKViews_IdempotencyAfterChange verifies that a changed FK record causes
// updated==1 while unchanged records remain unchanged.
func TestBuildFKViews_IdempotencyAfterChange(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records1 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
	}

	// First run.
	_, _, _, errs1 := buildFKViews(tmpDir, "", col, def, view, records1, nil)
	if len(errs1) > 0 {
		t.Fatalf("first run errors: %v", errs1)
	}

	// Second run — gb record changed.
	records2 := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme Corp", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
	}

	created2, updated2, unchanged2, errs2 := buildFKViews(tmpDir, "", col, def, view, records2, nil)
	if len(errs2) > 0 {
		t.Fatalf("second run errors: %v", errs2)
	}
	if created2 != 0 {
		t.Errorf("second run: expected 0 created, got %d", created2)
	}
	if updated2 != 1 {
		t.Errorf("second run: expected 1 updated (gb changed), got %d", updated2)
	}
	if unchanged2 != 1 {
		t.Errorf("second run: expected 1 unchanged (ca), got %d", unchanged2)
	}
}

// TestBuildFKViews_NoFKColumns verifies that buildFKViews returns zeros immediately when
// the collection has no FK columns.
func TestBuildFKViews_NoFKColumns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:           "products",
		DirPath:      filepath.Join(tmpDir, "products"),
		ColumnsOrder: []string{"id", "name"},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
	}
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"id": "1", "name": "Widget"}),
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected zero counters for no-FK collection, got created=%d updated=%d unchanged=%d", created, updated, unchanged)
	}

	// No $fk_* directory should exist.
	ingitdbDir := filepath.Join(tmpDir, ingitdb.IngitdbDir)
	if _, err := os.Stat(ingitdbDir); err == nil {
		entries, readErr := os.ReadDir(ingitdbDir)
		if readErr == nil && len(entries) > 0 {
			t.Errorf("expected no $ingitdb output for no-FK collection, but found: %v", entries)
		}
	}
}

// TestBuildFKViews_NoDefaultView_BuildViewsIntegration verifies that if a collection has
// FK columns but no default_view, BuildViews does not call buildFKViews.
func TestBuildFKViews_NoDefaultView_BuildViewsIntegration(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	// No DefaultView set.
	col.DefaultView = nil

	// Use a non-default view.
	nonDefaultView := &ingitdb.ViewDef{
		ID:       "custom",
		FileName: "custom.md",
		Template: "md-table",
	}

	writer := &capturingWriter{}
	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{"custom": nonDefaultView}},
		RecordsReader: fakeRecordsReader{records: []ingitdb.IRecordEntry{
			ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		}},
		Writer: writer,
	}

	result, err := builder.BuildViews(context.TODO(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}

	// No FK view files should exist since no default_view.
	fkDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "companies", "$fk_country")
	if _, statErr := os.Stat(fkDir); statErr == nil {
		t.Errorf("$fk_country directory should not exist when there is no default_view")
	}
	_ = result
}

// TestBuildFKViews_INGRHeaderHasColumnTypeAnnotations verifies that the INGR header
// includes column-type annotations when using ingr format.
func TestBuildFKViews_INGRHeaderHasColumnTypeAnnotations(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("ingr")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 file created, got %d", created)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}

	// The INGR header should include type annotations like "name:string".
	if !strings.Contains(string(content), ":string") {
		t.Errorf("expected INGR header to contain type annotations (:string), got:\n%s", string(content))
	}
}

// TestBuildFKViews_ViewNameInINGRHeader verifies that the viewName embedded in the INGR
// header follows the pattern col.ID + "/$fk_" + colName + "/" + fkCollection + "/" + fkValue.
func TestBuildFKViews_ViewNameInINGRHeader(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("ingr")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	_, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}

	// INGR header should contain the view name pattern.
	expectedViewName := "companies/$fk_country/countries/gb"
	if !strings.Contains(string(content), expectedViewName) {
		t.Errorf("expected INGR header to contain %q, got:\n%s", expectedViewName, string(content))
	}
}

// TestBuildFKViews_ErrorAccumulation verifies that a write failure for one FK value does not
// abort processing of other FK values; other files are still written and the error is returned.
func TestBuildFKViews_ErrorAccumulation(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
	}

	// Pre-create the gb path as a directory so WriteFile on it will fail.
	gbDir := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "json")
	if err := os.MkdirAll(gbDir, 0o755); err != nil {
		t.Fatalf("setup: mkdir gb as dir: %v", err)
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	// At least one error should be collected.
	if len(errs) == 0 {
		t.Fatal("expected at least one error due to write failure for gb")
	}

	// The ca file should still be written despite the gb failure.
	caPath := fkFilePath(tmpDir, "companies", "country", "countries", "ca", "json")
	if _, err := os.Stat(caPath); err != nil {
		t.Errorf("ca.json should still have been written despite gb failure: %v", err)
	}

	// Total files processed = 1 (ca written successfully), 1 error.
	total := created + updated + unchanged
	if total != 1 {
		t.Errorf("expected 1 file processed successfully, got created=%d updated=%d unchanged=%d", created, updated, unchanged)
	}
}

// TestBuildFKViews_LogfCalled verifies that the logf function is called when provided.
func TestBuildFKViews_LogfCalled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	var logMessages []string
	logf := func(format string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, logf)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}
	if len(logMessages) == 0 {
		t.Error("expected logf to be called at least once")
	}
}

// TestBuildFKViews_LogfCalledOnUnchanged verifies that logf is called even when the content is unchanged.
func TestBuildFKViews_LogfCalledOnUnchanged(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	// First run to create.
	_, _, _, errs1 := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs1) > 0 {
		t.Fatalf("first run errors: %v", errs1)
	}

	// Second run with logf.
	var logMessages []string
	logf := func(format string, args ...any) {
		logMessages = append(logMessages, fmt.Sprintf(format, args...))
	}
	_, _, unchanged, errs2 := buildFKViews(tmpDir, "", col, def, view, records, logf)
	if len(errs2) > 0 {
		t.Fatalf("second run errors: %v", errs2)
	}
	if unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", unchanged)
	}
	if len(logMessages) == 0 {
		t.Error("expected logf to be called on unchanged file too")
	}
}

// TestBuildFKViews_RepoRootOverridesOutputRoot verifies that when repoRoot is set, it is
// used as the outputRoot instead of dbPath.
func TestBuildFKViews_RepoRootOverridesOutputRoot(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	dbPath := filepath.Join(repoRoot, "databases", "mydb")

	if err := os.MkdirAll(dbPath, 0o755); err != nil {
		t.Fatalf("setup mkdir: %v", err)
	}

	col := &ingitdb.CollectionDef{
		ID:           "companies",
		DirPath:      filepath.Join(dbPath, "companies"),
		ColumnsOrder: []string{"id", "name", "country"},
		Columns: map[string]*ingitdb.ColumnDef{
			"name":    {Type: ingitdb.ColumnTypeString},
			"country": {Type: ingitdb.ColumnTypeString, ForeignKey: "countries"},
		},
	}
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(dbPath, repoRoot, col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	// File should be under repoRoot/$ingitdb/..., not dbPath/$ingitdb/...
	relColPath, _ := filepath.Rel(repoRoot, col.DirPath)
	expectedPath := filepath.Join(repoRoot, ingitdb.IngitdbDir, relColPath, "$fk_country", "countries", "gb.json")
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected file at %q (relative to repoRoot) to exist: %v", expectedPath, err)
	}

	wrongPath := filepath.Join(dbPath, ingitdb.IngitdbDir, "companies", "$fk_country", "countries", "gb.json")
	if _, err := os.Stat(wrongPath); err == nil {
		t.Errorf("file should not be under dbPath/$ingitdb when repoRoot is set")
	}
}

// TestBuildFKViews_EmptyColumns verifies that buildFKViews returns zeros immediately when
// col.Columns is nil/empty.
func TestBuildFKViews_EmptyColumns(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := &ingitdb.CollectionDef{
		ID:      "empty",
		DirPath: filepath.Join(tmpDir, "empty"),
		Columns: nil,
	}
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("1", map[string]any{"id": "1"}),
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
	if created+updated+unchanged != 0 {
		t.Errorf("expected zero counters for empty columns, got %d+%d+%d", created, updated, unchanged)
	}
}

// TestBuildFKViews_IncludeHash verifies that when view.IncludeHash is true the exported
// INGR file contains a "# sha256:" line.
func TestBuildFKViews_IncludeHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := &ingitdb.ViewDef{
		ID:          ingitdb.DefaultViewID,
		IsDefault:   true,
		Format:      "ingr",
		IncludeHash: true,
	}
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}
	if !strings.Contains(string(content), "# sha256:") {
		t.Errorf("expected '# sha256:' in output when IncludeHash=true, got:\n%s", string(content))
	}
}

// TestBuildFKViews_RecordsDelimiterFromSettings verifies that when
// def.Settings.RecordsDelimiter is set to 1, the INGR FK view file contains a "#-" line.
func TestBuildFKViews_RecordsDelimiterFromSettings(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := &ingitdb.ViewDef{
		ID:        ingitdb.DefaultViewID,
		IsDefault: true,
		Format:    "ingr",
	}
	def := &ingitdb.Definition{
		Settings: ingitdb.Settings{RecordsDelimiter: 1},
	}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}
	if !strings.Contains(string(content), "\n#-\n") {
		t.Errorf("expected '#-' delimiter line in output when Settings.RecordsDelimiter=1, got:\n%s", string(content))
	}
}

// TestBuildFKViews_RecordsDelimiterFromView verifies that when view.RecordsDelimiter is -1
// the resolved value becomes negative and WithRecordsDelimiter is NOT applied.
func TestBuildFKViews_RecordsDelimiterFromView(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := &ingitdb.ViewDef{
		ID:               ingitdb.DefaultViewID,
		IsDefault:        true,
		Format:           "ingr",
		RecordsDelimiter: -1,
	}
	def := &ingitdb.Definition{}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}
	if strings.Contains(string(content), "#-") {
		t.Errorf("expected no '#-' delimiter when view.RecordsDelimiter=-1, got:\n%s", string(content))
	}
}

// TestBuildFKViews_RuntimeOverrideDisablesDelimiter verifies that when
// RuntimeOverrides.RecordsDelimiter is -1 it overrides view.RecordsDelimiter=1
// and no "#-" line appears in the FK view output.
func TestBuildFKViews_RuntimeOverrideDisablesDelimiter(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := &ingitdb.ViewDef{
		ID:               ingitdb.DefaultViewID,
		IsDefault:        true,
		Format:           "ingr",
		RecordsDelimiter: 1,
	}
	minusOne := -1
	def := &ingitdb.Definition{
		RuntimeOverrides: ingitdb.RuntimeOverrides{RecordsDelimiter: &minusOne},
	}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if created != 1 {
		t.Fatalf("expected 1 created, got %d", created)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "ingr")
	content, err := os.ReadFile(gbPath)
	if err != nil {
		t.Fatalf("gb.ingr not found: %v", err)
	}
	if strings.Contains(string(content), "#-") {
		t.Errorf("expected no '#-' delimiter when RuntimeOverrides.RecordsDelimiter=-1, got:\n%s", string(content))
	}
}

// TestBuildFKViews_NilDataRecordSkipped verifies that a record whose GetData() returns nil
// is silently skipped and does not cause a panic or error.
func TestBuildFKViews_NilDataRecordSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	nilDataRecord := ingitdb.NewMapRecordEntry("ghost", nil)
	records := []ingitdb.IRecordEntry{
		nilDataRecord,
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	created, updated, unchanged, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	// Only the acme record (gb) should produce a file; the nil-data record is skipped.
	if created != 1 {
		t.Errorf("expected 1 created (nil-data record skipped), got %d", created)
	}
	if updated != 0 {
		t.Errorf("expected 0 updated, got %d", updated)
	}
	if unchanged != 0 {
		t.Errorf("expected 0 unchanged, got %d", unchanged)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "json")
	if _, err := os.Stat(gbPath); err != nil {
		t.Errorf("gb.json should exist for the non-nil record: %v", err)
	}
}

// TestBuildViews_FKViewsIntegration_DefaultView verifies that BuildViews generates FK view
// files when a collection has a default_view with FK columns.
func TestBuildViews_FKViewsIntegration_DefaultView(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	col.DefaultView = makeDefaultView("json")

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
		ingitdb.NewMapRecordEntry("shopify", map[string]any{"id": "shopify", "name": "Shopify", "country": "ca"}),
	}

	builder := SimpleViewBuilder{
		DefReader:     fakeViewDefReader{views: map[string]*ingitdb.ViewDef{}},
		RecordsReader: fakeRecordsReader{records: records},
		Writer:        &capturingWriter{},
	}

	result, err := builder.BuildViews(context.TODO(), tmpDir, "", col, &ingitdb.Definition{})
	if err != nil {
		t.Fatalf("BuildViews: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected result errors: %v", result.Errors)
	}

	// The default view file itself (created by buildDefaultView) + 2 FK view files.
	// capturingWriter handles the non-default view; buildDefaultView/buildFKViews write directly.
	// So FilesCreated should include at least the 2 FK files.
	totalFiles := result.FilesCreated + result.FilesUpdated + result.FilesUnchanged
	if totalFiles < 2 {
		t.Errorf("expected at least 2 files (FK views), got total=%d (created=%d updated=%d unchanged=%d)",
			totalFiles, result.FilesCreated, result.FilesUpdated, result.FilesUnchanged)
	}

	gbPath := fkFilePath(tmpDir, "companies", "country", "countries", "gb", "json")
	caPath := fkFilePath(tmpDir, "companies", "country", "countries", "ca", "json")

	if _, err := os.Stat(gbPath); err != nil {
		t.Errorf("gb.json should exist after BuildViews: %v", err)
	}
	if _, err := os.Stat(caPath); err != nil {
		t.Errorf("ca.json should exist after BuildViews: %v", err)
	}
}

// TestBuildFKViews_MkdirAllError verifies that when MkdirAll fails (a regular file blocks the
// path), the error is collected and processing continues for other FK values.
func TestBuildFKViews_MkdirAllError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	col := makeCompaniesCol(t, filepath.Join(tmpDir, "companies"))
	view := makeDefaultView("json")
	def := &ingitdb.Definition{}

	// Block MkdirAll for both FK values by placing a regular FILE at the path that
	// MkdirAll must treat as a directory (the "countries" subdirectory).
	// outPath for any country value: tmpDir/$ingitdb/companies/$fk_country/countries/<val>.json
	// filepath.Dir(outPath)         = tmpDir/$ingitdb/companies/$fk_country/countries
	// Creating a FILE at that path makes MkdirAll fail with "not a directory".
	countriesDir := filepath.Join(tmpDir, ingitdb.IngitdbDir, "companies", "$fk_country", "countries")
	countriesParent := filepath.Dir(countriesDir)
	if err := os.MkdirAll(countriesParent, 0o755); err != nil {
		t.Fatalf("setup: mkdir parent: %v", err)
	}
	if err := os.WriteFile(countriesDir, []byte("blocker"), 0o644); err != nil {
		t.Fatalf("setup: write blocker file: %v", err)
	}

	records := []ingitdb.IRecordEntry{
		ingitdb.NewMapRecordEntry("acme", map[string]any{"id": "acme", "name": "Acme", "country": "gb"}),
	}

	_, _, _, errs := buildFKViews(tmpDir, "", col, def, view, records, nil)

	if len(errs) == 0 {
		t.Fatal("expected at least one error due to MkdirAll failure")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "mkdir") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected a 'mkdir' error in errs, got: %v", errs)
	}
}
