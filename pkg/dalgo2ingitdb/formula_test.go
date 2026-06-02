package dalgo2ingitdb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// makeFormulaMapTx builds a readonlyTx for a MapOfRecords "scores" collection
// whose schema declares a computed int column "doubled" = score * 2 and a
// computed int "ratio" = score / divisor. It mirrors makeMapOfRecordsTx but
// attaches formula columns so the MapOfRecords read path exercises formula
// computation and its error branch.
func makeFormulaMapTx(t *testing.T, root string) (readonlyTx, *ingitdb.CollectionDef) {
	t.Helper()
	colDef := &ingitdb.CollectionDef{
		ID:      "scores",
		DirPath: filepath.Join(root, "scores"),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "scores.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"score":   {Type: ingitdb.ColumnTypeInt},
			"divisor": {Type: ingitdb.ColumnTypeInt},
			"doubled": {Type: ingitdb.ColumnTypeInt, Formula: "score * 2"},
			"ratio":   {Type: ingitdb.ColumnTypeInt, Formula: "score / divisor"},
		},
	}
	tx := readonlyTx{
		db: &Database{projectPath: root},
		def: &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{
			"scores": colDef,
		}},
	}
	return tx, colDef
}

func writeScoresFile(t *testing.T, colDef *ingitdb.CollectionDef, content string) {
	t.Helper()
	if err := os.MkdirAll(colDef.DirPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	p := filepath.Join(colDef.DirPath, "scores.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestReadonlyTx_Get_MapOfRecords_ComputesFormula(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeFormulaMapTx(t, root)
	writeScoresFile(t, colDef, "alice:\n  score: 5\n  divisor: 5\n")

	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	if err := tx.Get(context.Background(), rec); err != nil {
		t.Fatalf("Get: %v", err)
	}
	data := rec.Data().(map[string]any)
	if data["doubled"] != int64(10) {
		t.Errorf("doubled: got %v, want int64(10)", data["doubled"])
	}
}

func TestReadonlyTx_Get_MapOfRecords_FormulaError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	tx, colDef := makeFormulaMapTx(t, root)
	writeScoresFile(t, colDef, "alice:\n  score: 5\n  divisor: 0\n")

	rec := dal.NewRecordWithData(dal.NewKeyWithID("scores", "alice"), map[string]any{})
	err := tx.Get(context.Background(), rec)
	if err == nil {
		t.Fatal("expected formula runtime error (division by zero)")
	}
	for _, want := range []string{"scores", "alice", "ratio"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestReadAllMapOfRecords_FormulaError(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, colDef := makeFormulaMapTx(t, root)
	writeScoresFile(t, colDef, "alice:\n  score: 5\n  divisor: 0\n")

	_, err := readAllMapOfRecords(colDef)
	if err == nil {
		t.Fatal("expected formula runtime error from readAllMapOfRecords")
	}
	if !strings.Contains(err.Error(), "ratio") {
		t.Errorf("error %q missing 'ratio'", err.Error())
	}
}

func TestReadAllMapOfRecords_ComputesFormula(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, colDef := makeFormulaMapTx(t, root)
	writeScoresFile(t, colDef, "alice:\n  score: 5\n  divisor: 5\n")

	records, err := readAllMapOfRecords(colDef)
	if err != nil {
		t.Fatalf("readAllMapOfRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	data := records[0].Data().(map[string]any)
	if data["doubled"] != int64(10) {
		t.Errorf("doubled: got %v, want int64(10)", data["doubled"])
	}
}

func TestApplyFormulasToRead_FullNameString(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"full_name": {Type: ingitdb.ColumnTypeString, Formula: `first_name + " " + last_name`},
	}
	data := map[string]any{"first_name": "Ada", "last_name": "Lovelace"}

	result, err := ApplyFormulasToRead(data, cols, "people", "ada")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["full_name"] != "Ada Lovelace" {
		t.Fatalf("expected full_name='Ada Lovelace', got %v", result["full_name"])
	}
	// The input map must not be mutated.
	if _, has := data["full_name"]; has {
		t.Fatal("expected input data not to be mutated")
	}
}

func TestApplyFormulasToRead_IntTotal(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"total": {Type: ingitdb.ColumnTypeInt, Formula: "qty * price"},
	}
	data := map[string]any{"qty": 3, "price": 4}

	result, err := ApplyFormulasToRead(data, cols, "orders", "o1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := result["total"].(int64)
	if !ok {
		t.Fatalf("expected total int64, got %T", result["total"])
	}
	if got != 12 {
		t.Fatalf("expected total=12, got %d", got)
	}
}

func TestApplyFormulasToRead_IntFromIntegralFloat(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"half": {Type: ingitdb.ColumnTypeInt, Formula: "a / b"},
	}
	data := map[string]any{"a": 10, "b": 2}

	result, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["half"] != int64(5) {
		t.Fatalf("expected half=5, got %v (%T)", result["half"], result["half"])
	}
}

func TestApplyFormulasToRead_IntFromNonIntegralFloatFails(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"x": {Type: ingitdb.ColumnTypeInt, Formula: "a / b"},
	}
	data := map[string]any{"a": 7, "b": 2}

	_, err := ApplyFormulasToRead(data, cols, "coll", "rec")
	if err == nil {
		t.Fatal("expected error for non-integral float into int")
	}
	for _, want := range []string{"coll", "rec", "x"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestApplyFormulasToRead_FloatFromFloat(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"ratio": {Type: ingitdb.ColumnTypeFloat, Formula: "a / b"},
	}
	data := map[string]any{"a": 7, "b": 2}

	result, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["ratio"] != 3.5 {
		t.Fatalf("expected ratio=3.5, got %v", result["ratio"])
	}
}

func TestApplyFormulasToRead_FloatFromInt(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"f": {Type: ingitdb.ColumnTypeFloat, Formula: "a + b"},
	}
	data := map[string]any{"a": 2, "b": 3}

	result, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["f"] != float64(5) {
		t.Fatalf("expected f=5.0, got %v (%T)", result["f"], result["f"])
	}
}

func TestApplyFormulasToRead_Bool(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"adult": {Type: ingitdb.ColumnTypeBool, Formula: "age >= 18"},
	}
	data := map[string]any{"age": 21}

	result, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["adult"] != true {
		t.Fatalf("expected adult=true, got %v", result["adult"])
	}
}

func TestApplyFormulasToRead_Any(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"v": {Type: ingitdb.ColumnTypeAny, Formula: `"hi"`},
	}
	data := map[string]any{}

	result, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["v"] != "hi" {
		t.Fatalf("expected v='hi', got %v", result["v"])
	}
}

func TestApplyFormulasToRead_CoercionMismatch(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"name": {Type: ingitdb.ColumnTypeString, Formula: "1 + 2"},
	}
	data := map[string]any{}

	_, err := ApplyFormulasToRead(data, cols, "coll", "rec")
	if err == nil {
		t.Fatal("expected coercion mismatch error")
	}
	for _, want := range []string{"coll", "rec", "name"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestApplyFormulasToRead_RuntimeError(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"q": {Type: ingitdb.ColumnTypeInt, Formula: "a / b"},
	}
	data := map[string]any{"a": 1, "b": 0}

	_, err := ApplyFormulasToRead(data, cols, "nums", "row7")
	if err == nil {
		t.Fatal("expected runtime error for division by zero")
	}
	for _, want := range []string{"nums", "row7", "q"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q missing %q", err.Error(), want)
		}
	}
}

func TestApplyFormulasToRead_NoComputedColumnsUnchanged(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"first_name": {Type: ingitdb.ColumnTypeString},
	}
	data := map[string]any{"first_name": "Ada"}

	result, err := ApplyFormulasToRead(data, cols, "people", "ada")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["first_name"] != "Ada" {
		t.Fatalf("expected first_name unchanged, got %v", result["first_name"])
	}
	if len(result) != 1 {
		t.Fatalf("expected exactly one field, got %d", len(result))
	}
}

func TestApplyFormulasToRead_EmptyCols(t *testing.T) {
	t.Parallel()

	data := map[string]any{"x": 1}
	result, err := ApplyFormulasToRead(data, nil, "c", "k")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["x"] != 1 {
		t.Fatalf("expected x=1, got %v", result["x"])
	}
}

func TestApplyFormulasToRead_FloatCoercionMismatch(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"f": {Type: ingitdb.ColumnTypeFloat, Formula: `"str"`},
	}
	data := map[string]any{}

	_, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err == nil {
		t.Fatal("expected float coercion mismatch error")
	}
}

func TestApplyFormulasToRead_BoolCoercionMismatch(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"b": {Type: ingitdb.ColumnTypeBool, Formula: "1"},
	}
	data := map[string]any{}

	_, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err == nil {
		t.Fatal("expected bool coercion mismatch error")
	}
}

func TestApplyFormulasToRead_UnsupportedColumnType(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"d": {Type: ingitdb.ColumnTypeDate, Formula: `"x"`},
	}
	data := map[string]any{}

	_, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err == nil {
		t.Fatal("expected error for unsupported computed-column type")
	}
	if !strings.Contains(err.Error(), "do not support type") {
		t.Fatalf("error %q missing expected cause", err.Error())
	}
}

func TestApplyFormulasToRead_IntCoercionMismatch(t *testing.T) {
	t.Parallel()

	cols := map[string]*ingitdb.ColumnDef{
		"i": {Type: ingitdb.ColumnTypeInt, Formula: `"str"`},
	}
	data := map[string]any{}

	_, err := ApplyFormulasToRead(data, cols, "c", "k")
	if err == nil {
		t.Fatal("expected int coercion mismatch error")
	}
}
