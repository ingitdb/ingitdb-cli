package dalgo2ingitdb

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dbschema"
	"github.com/ingr-io/ingr-go/ingr"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// errSeam is a sentinel error injected by seam-swapping tests in this file.
var errSeam = errors.New("seam failure")

// The tests below swap package-level seams from seams.go to reach error
// branches that are unreachable in production (marshal/compile/path failures on
// already-validated, in-memory, or glob-derived inputs). Each is intentionally
// NOT parallel because it mutates package-level state.

// TestMarshalForFormat_YAMLError covers the yaml.Marshal error branch in
// marshalForFormat (parse.go line 142-144) via the yamlMarshal seam.
func TestMarshalForFormat_YAMLError(t *testing.T) {
	orig := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errSeam }
	defer func() { yamlMarshal = orig }()

	_, err := marshalForFormat(map[string]any{"k": "v"}, ingitdb.RecordFormatYAML)
	if err == nil {
		t.Fatal("marshalForFormat: want error when yamlMarshal fails")
	}
	if !strings.Contains(err.Error(), "marshal YAML") {
		t.Errorf("error = %v, want it to wrap the YAML marshal failure", err)
	}
}

// TestMarshalForFormat_TOMLError covers the toml.Marshal error branch in
// marshalForFormat (parse.go line 154-156) via the tomlMarshal seam.
func TestMarshalForFormat_TOMLError(t *testing.T) {
	orig := tomlMarshal
	tomlMarshal = func(any) ([]byte, error) { return nil, errSeam }
	defer func() { tomlMarshal = orig }()

	_, err := marshalForFormat(map[string]any{"k": "v"}, ingitdb.RecordFormatTOML)
	if err == nil {
		t.Fatal("marshalForFormat: want error when tomlMarshal fails")
	}
	if !strings.Contains(err.Error(), "marshal TOML") {
		t.Errorf("error = %v, want it to wrap the TOML marshal failure", err)
	}
}

// TestWriteCollectionDefYAML_MarshalError covers the yaml.Marshal error branch
// in writeCollectionDefYAML (schema_modifier.go line 365-367) via yamlMarshal.
func TestWriteCollectionDefYAML_MarshalError(t *testing.T) {
	orig := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errSeam }
	defer func() { yamlMarshal = orig }()

	p := filepath.Join(t.TempDir(), "definition.yaml")
	err := writeCollectionDefYAML(p, &ingitdb.CollectionDef{ID: "c"})
	if err == nil {
		t.Fatal("writeCollectionDefYAML: want error when yamlMarshal fails")
	}
	if !strings.Contains(err.Error(), "marshal definition.yaml") {
		t.Errorf("error = %v, want it to wrap the marshal failure", err)
	}
}

// TestRewriteRecordFiles_MarshalError covers the yaml.Marshal error branch
// inside the walk in rewriteRecordFiles (schema_modifier.go line 422-424) via
// the yamlMarshal seam. A real record file is present so the walk reaches the
// marshal step after a successful read+unmarshal.
func TestRewriteRecordFiles_MarshalError(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "rec.yaml"), []byte("a: 1\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	orig := yamlMarshal
	yamlMarshal = func(any) ([]byte, error) { return nil, errSeam }
	defer func() { yamlMarshal = orig }()

	err := rewriteRecordFiles(root, ingitdb.RecordFormatYAML, func(_ map[string]any) {})
	if err == nil {
		t.Fatal("rewriteRecordFiles: want error when yamlMarshal fails")
	}
	if !strings.Contains(err.Error(), "marshal ") {
		t.Errorf("error = %v, want it to wrap the marshal failure", err)
	}
}

// TestValidateCollectionName_IsAbs covers the filepath.IsAbs branch in
// validateCollectionName (schema_modifier.go line 316-318) via the filepathIsAbs
// seam. In production the earlier path-segment check rejects names that would be
// absolute, so this branch cannot be reached with a real name.
func TestValidateCollectionName_IsAbs(t *testing.T) {
	orig := filepathIsAbs
	filepathIsAbs = func(string) bool { return true }
	defer func() { filepathIsAbs = orig }()

	err := validateCollectionName("valid-name")
	if err == nil {
		t.Fatal("validateCollectionName: want error when filepath.IsAbs reports absolute")
	}
	if !strings.Contains(err.Error(), "must be relative") {
		t.Errorf("error = %v, want the must-be-relative error", err)
	}
}

// TestListCollections_RelError covers the filepath.Rel error branch in
// ListCollections (schema_reader.go line 63-65) via the filepathRel seam. In
// production the walked path is always under projectPath, so filepath.Rel never
// fails.
func TestListCollections_RelError(t *testing.T) {
	root := t.TempDir()
	writeCollectionDef(t, root, "tags", countriesDef)

	orig := filepathRel
	filepathRel = func(string, string) (string, error) { return "", errSeam }
	defer func() { filepathRel = orig }()

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	reader := db.(dbschema.SchemaReader)
	_, err = reader.ListCollections(context.Background(), nil)
	if err == nil {
		t.Fatal("ListCollections: want error when filepath.Rel fails")
	}
}

// TestReadAllSingleRecords_CompileError covers the regexp.Compile error branch
// in buildKeyExtractor (query.go line 148-150) and its propagation in
// readAllSingleRecords (query.go line 86-88) via the regexpCompile seam. The
// record-file name contains {key}, so buildKeyExtractor takes the regexp path.
func TestReadAllSingleRecords_CompileError(t *testing.T) {
	orig := regexpCompile
	regexpCompile = func(string) (*regexp.Regexp, error) { return nil, errSeam }
	defer func() { regexpCompile = orig }()

	colDef := &ingitdb.CollectionDef{
		ID:      "things",
		DirPath: t.TempDir(),
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
	_, err := readAllSingleRecords(colDef)
	if err == nil {
		t.Fatal("readAllSingleRecords: want error when regexp.Compile fails")
	}
	if !strings.Contains(err.Error(), "build key extractor") {
		t.Errorf("error = %v, want it to wrap the compile failure", err)
	}
}

// fakeCSVWriter is a csvWriter whose Write fails on a chosen call index and
// whose Error() returns a configurable result.
type fakeCSVWriter struct {
	failWriteOn int // 1-based Write call index that should fail; 0 = never
	writes      int
	errResult   error
}

func (f *fakeCSVWriter) Write([]string) error {
	f.writes++
	if f.failWriteOn != 0 && f.writes == f.failWriteOn {
		return errSeam
	}
	return nil
}
func (f *fakeCSVWriter) Flush()       {}
func (f *fakeCSVWriter) Error() error { return f.errResult }

// TestEncodeCSVForCollection_WriterErrors covers the three csv.Writer error
// branches in encodeCSVForCollection (csv.go lines 113-115 header, 130-132 row,
// 135-137 flush/Error) via the newCSVWriter seam. In production the writer
// targets a bytes.Buffer and never errors. Each subtest mutates a seam, so the
// parent is intentionally NOT parallel.
func TestEncodeCSVForCollection_WriterErrors(t *testing.T) {
	colDef := &ingitdb.CollectionDef{ID: "c", ColumnsOrder: []string{"a"}}
	rows := []map[string]any{{"a": "x"}}

	cases := []struct {
		name    string
		fake    *fakeCSVWriter
		value   any
		wantSub string
	}{
		{"header write", &fakeCSVWriter{failWriteOn: 1}, rows, "failed to write csv header"},
		{"row write", &fakeCSVWriter{failWriteOn: 2}, rows, "failed to write csv row"},
		{"flush error", &fakeCSVWriter{errResult: errSeam}, []map[string]any{}, "csv writer error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := newCSVWriter
			newCSVWriter = func(io.Writer) csvWriter { return tc.fake }
			defer func() { newCSVWriter = orig }()

			_, err := encodeCSVForCollection(tc.value, colDef)
			if err == nil {
				t.Fatal("encodeCSVForCollection: want error from the writer")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}

// fakeRecordsWriter is an ingr.RecordsWriter whose methods return configurable
// errors.
type fakeRecordsWriter struct {
	headerErr, recordsErr, closeErr error
}

func (f *fakeRecordsWriter) WriteHeader(string, []ingr.ColDef) (int, error) { return 0, f.headerErr }
func (f *fakeRecordsWriter) WriteRecords(int, ...ingr.Record) (int, error)  { return 0, f.recordsErr }
func (f *fakeRecordsWriter) Close() error                                   { return f.closeErr }

// TestEncodeINGRFromMap_WriterErrors covers the three ingr writer error branches
// in encodeINGRFromMap (parse.go lines 193-195 header, 196-198 records, 199-201
// close) via the newRecordsWriter seam. In production the writer targets a
// bytes.Buffer and never errors. Each subtest mutates a seam, so the parent is
// intentionally NOT parallel.
func TestEncodeINGRFromMap_WriterErrors(t *testing.T) {
	data := map[string]map[string]any{"id1": {"a": "x"}}

	cases := []struct {
		name    string
		fake    *fakeRecordsWriter
		wantSub string
	}{
		{"header", &fakeRecordsWriter{headerErr: errSeam}, "ingr: write header"},
		{"records", &fakeRecordsWriter{recordsErr: errSeam}, "ingr: write records"},
		{"close", &fakeRecordsWriter{closeErr: errSeam}, "ingr: close writer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orig := newRecordsWriter
			newRecordsWriter = func(io.Writer) ingr.RecordsWriter { return tc.fake }
			defer func() { newRecordsWriter = orig }()

			_, err := encodeINGRFromMap(data, "test", nil)
			if err == nil {
				t.Fatal("encodeINGRFromMap: want error from the writer")
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %v, want substring %q", err, tc.wantSub)
			}
		})
	}
}
