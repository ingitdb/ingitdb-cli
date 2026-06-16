package commands

// specscore: feature/cli/select

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"gopkg.in/yaml.v3"
)

// peopleComputedDef returns a Definition with a `people` collection whose
// `full_name` column is computed from stored `first_name`/`last_name`.
func peopleComputedDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"first_name": {Type: ingitdb.ColumnTypeString},
					"last_name":  {Type: ingitdb.ColumnTypeString},
					"full_name":  {Type: ingitdb.ColumnTypeString, Formula: `first_name + " " + last_name`},
				},
				ColumnsOrder: []string{"first_name", "last_name", "full_name"},
			},
		},
	}
}

// peopleSelectDeps mirrors selectTestDeps but uses peopleComputedDef so the
// `people` collection with the computed `full_name` column is available.
func peopleSelectDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := peopleComputedDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	return homeDir, getWd, readDef, newDB, logf
}

// seedPerson writes a YAML file for a person record with only the stored
// columns (the computed full_name must never be persisted).
func seedPerson(t *testing.T, dir, key, first, last string) {
	t.Helper()
	colDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out, err := yaml.Marshal(map[string]any{"first_name": first, "last_name": last})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, key+".yaml"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// AC: filter-on-computed-column — --where on the computed full_name returns
// only records whose computed value matches, even though full_name is never
// stored on disk.
func TestSelect_SetMode_WhereOnComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := peopleSelectDeps(t, dir)
	seedPerson(t, dir, "ada", "Ada", "Lovelace")
	seedPerson(t, dir, "alan", "Alan", "Turing")

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people",
		`--where=full_name == "Ada Lovelace"`,
		"--format=yaml",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "Ada Lovelace") {
		t.Errorf("expected the matching computed full_name in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "Alan Turing") {
		t.Errorf("did NOT expect non-matching record in output:\n%s", stdout)
	}
}

// AC: order-by-computed-column — order_by on the computed full_name sorts by
// the computed value, not by any stored column.
func TestSelect_SetMode_OrderByComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := peopleSelectDeps(t, dir)
	// Seeded out of full_name order; record keys chosen so $id order differs
	// from full_name order, proving the sort uses the computed value.
	seedPerson(t, dir, "z", "Grace", "Hopper")
	seedPerson(t, dir, "a", "Ada", "Lovelace")
	seedPerson(t, dir, "m", "Alan", "Turing")

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people",
		"--order-by=full_name",
		"--fields=$id,full_name",
		"--format=csv",
	)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	idxAda := strings.Index(stdout, "Ada Lovelace")
	idxAlan := strings.Index(stdout, "Alan Turing")
	idxGrace := strings.Index(stdout, "Grace Hopper")
	if idxAda < 0 || idxAlan < 0 || idxGrace < 0 {
		t.Fatalf("expected all three computed full_name values in output:\n%s", stdout)
	}
	// Ascending by full_name: "Ada Lovelace" < "Alan Turing" < "Grace Hopper".
	if idxAda >= idxAlan || idxAlan >= idxGrace {
		t.Errorf("rows not ordered by computed full_name asc (Ada<Alan<Grace), got positions Ada@%d Alan@%d Grace@%d:\n%s",
			idxAda, idxAlan, idxGrace, stdout)
	}
}

// peopleHelpersDef returns a people definition exercising the Starlark string
// and numeric helpers through computed columns: full_name (concat), display
// (strip+upper) and rounded (round of a float score, declared int).
func peopleHelpersDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"first_name": {Type: ingitdb.ColumnTypeString},
					"last_name":  {Type: ingitdb.ColumnTypeString},
					"score":      {Type: ingitdb.ColumnTypeFloat},
					"full_name":  {Type: ingitdb.ColumnTypeString, Formula: `first_name + " " + last_name`},
					"display":    {Type: ingitdb.ColumnTypeString, Formula: `first_name.strip().upper()`},
					"rounded":    {Type: ingitdb.ColumnTypeInt, Formula: `round(score)`},
				},
				ColumnsOrder: []string{"first_name", "last_name", "score", "full_name", "display", "rounded"},
			},
		},
	}
}

func helpersSelectDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := peopleHelpersDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	return homeDir, getWd, readDef, newDB, func(...any) {}
}

func seedPersonFields(t *testing.T, dir, key string, fields map[string]any) {
	t.Helper()
	colDir := filepath.Join(dir, "$records")
	if err := os.MkdirAll(colDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	out, err := yaml.Marshal(fields)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(colDir, key+".yaml"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// AC: select-renders-computed — select renders a computed string column.
func TestSelect_RendersComputedColumn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := helpersSelectDeps(t, dir)
	seedPersonFields(t, dir, "ada", map[string]any{"first_name": "Ada", "last_name": "Lovelace"})

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--fields=$id,full_name", "--format=csv")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "Ada Lovelace") {
		t.Errorf("expected computed full_name in output:\n%s", stdout)
	}
}

// AC: string-helper-preserved — Starlark string helpers (strip/upper) work.
func TestSelect_StringHelperPreserved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := helpersSelectDeps(t, dir)
	seedPersonFields(t, dir, "ada", map[string]any{"first_name": " ada "})

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--fields=$id,display", "--format=csv")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(stdout, "ADA") {
		t.Errorf("expected display 'ADA' (strip+upper) in output:\n%s", stdout)
	}
}

// AC: math-helper-preserved — round() yields an int, rendered as 5 not 4.6/5.0.
func TestSelect_MathHelperPreserved(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := helpersSelectDeps(t, dir)
	seedPersonFields(t, dir, "ada", map[string]any{"score": 4.6})

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--fields=$id,rounded", "--format=csv")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("want header + 1 row, got:\n%s", stdout)
	}
	if !strings.Contains(lines[1], "5") || strings.Contains(lines[1], "4.6") || strings.Contains(lines[1], "5.0") {
		t.Errorf("expected rounded integer 5, got row: %q", lines[1])
	}
}

// peopleRatioDef returns a people definition with a stored qty and a computed
// int ratio = qty / 0 that raises at runtime when evaluated.
func peopleRatioDef(dirPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"qty":   {Type: ingitdb.ColumnTypeInt},
					"ratio": {Type: ingitdb.ColumnTypeInt, Formula: "qty / 0"},
				},
				ColumnsOrder: []string{"qty", "ratio"},
			},
		},
	}
}

func ratioSelectDeps(t *testing.T, dir string) (
	func() (string, error),
	func() (string, error),
	func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	func(string, *ingitdb.Definition) (dal.DB, error),
	func(...any),
) {
	t.Helper()
	def := peopleRatioDef(dir)
	return func() (string, error) { return "/tmp/home", nil },
		func() (string, error) { return dir, nil },
		func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil },
		func(root string, d *ingitdb.Definition) (dal.DB, error) {
			return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
		},
		func(...any) {}
}

// AC: unreferenced-erroring-column-not-evaluated — projecting only the stored
// column succeeds even though the computed ratio would raise, because ratio is
// never referenced and therefore never evaluated (lazy).
func TestSelect_UnreferencedErroringColumnNotEvaluated(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := ratioSelectDeps(t, dir)
	seedPersonFields(t, dir, "a", map[string]any{"qty": 3})

	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--fields=$id,qty", "--format=csv")
	if err != nil {
		t.Fatalf("projecting only qty must not evaluate ratio: %v", err)
	}
	if !strings.Contains(stdout, "3") {
		t.Errorf("expected qty 3 in output:\n%s", stdout)
	}
}

// Referencing the erroring computed column (projection) aborts the read.
func TestSelect_ReferencedErroringColumnFailsLoud(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := ratioSelectDeps(t, dir)
	seedPersonFields(t, dir, "a", map[string]any{"qty": 3})

	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf,
		"--path="+dir, "--from=people", "--fields=$id,ratio", "--format=csv")
	if err == nil {
		t.Fatal("projecting the erroring ratio column must fail loud")
	}
	if !strings.Contains(err.Error(), "ratio") {
		t.Errorf("error should name the ratio column, got: %v", err)
	}
}

// peopleBadFormulaDef returns a people definition whose computed column's
// formula references a missing field, so formula evaluation fails on read.
func peopleBadFormulaDef(dirPath string, recordType ingitdb.RecordType, name string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: dirPath,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       name,
					Format:     "yaml",
					RecordType: recordType,
				},
				Columns: map[string]*ingitdb.ColumnDef{
					"first_name": {Type: ingitdb.ColumnTypeString},
					"full_name":  {Type: ingitdb.ColumnTypeString, Formula: `missing_field + "x"`},
				},
				ColumnsOrder: []string{"first_name", "full_name"},
			},
		},
	}
}

// A formula that references a non-existent field surfaces as a query error
// from the single-records read path (covers the formula error branch).
func TestSelect_SetMode_ComputedFormulaError_SingleRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := peopleBadFormulaDef(dir, ingitdb.SingleRecord, "{key}.yaml")
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	seedPerson(t, dir, "ada", "Ada", "Lovelace")

	_, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=people")
	if err == nil {
		t.Fatal("expected error from failing computed-column formula")
	}
	if !strings.Contains(err.Error(), "full_name") {
		t.Errorf("error should name the failing computed column, got: %v", err)
	}
}

// Same error branch on the map-of-records read path.
func TestSelect_SetMode_ComputedFormulaError_MapOfRecords(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := peopleBadFormulaDef(dir, ingitdb.MapOfRecords, "all.yaml")
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	out, err := yaml.Marshal(map[string]any{"ada": map[string]any{"first_name": "Ada"}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "all.yaml"), out, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err = runSelectCmd(t, homeDir, getWd, readDef, newDB, logf, "--path="+dir, "--from=people")
	if err == nil {
		t.Fatal("expected error from failing computed-column formula")
	}
	if !strings.Contains(err.Error(), "full_name") {
		t.Errorf("error should name the failing computed column, got: %v", err)
	}
}
