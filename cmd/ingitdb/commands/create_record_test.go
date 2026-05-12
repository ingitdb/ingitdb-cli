package commands

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestCreate_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/hello", "--data={name: Hello}")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(dir, "$records", "hello.yaml")); statErr != nil {
		t.Fatalf("expected file hello.yaml to be created: %v", statErr)
	}
}

func TestCreate_MissingID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--data={name: Hello}")
	if err == nil {
		t.Fatal("expected error for missing --id flag")
	}
}

func TestCreate_InvalidYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--data=: invalid: yaml: :")
	if err == nil {
		t.Fatal("expected error for invalid YAML in --data")
	}
}

func TestCreate_CollectionNotFound(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=no/such/thing", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestCreate_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("boom")
	}
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf, nil, nil, nil)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--data={name: X}")
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestCreate_NoInputProvided(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(""),
		func() bool { return true },
		nil,
	)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x")
	if err == nil {
		t.Fatal("expected error when stdin is a TTY and no --data or --edit")
	}
	if !strings.Contains(err.Error(), "stdin") && !strings.Contains(err.Error(), "--edit") {
		t.Fatalf("error should mention stdin or --edit, got: %v", err)
	}
}

func TestCreate_StdinInputSmoke(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("name: Piped\n"),
		func() bool { return false }, // not a TTY
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/piped"); err != nil {
		t.Fatalf("stdin smoke: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "$records", "piped.yaml")); err != nil {
		t.Fatalf("expected record file to be created: %v", err)
	}
}

func TestCreate_StdinYAML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("name: Ireland\n"),
		func() bool { return false },
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ie"); err != nil {
		t.Fatalf("Create via stdin YAML: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.yaml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile %s: %v", path, readErr)
	}
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}

func TestCreate_StdinMarkdown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testMarkdownDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	mdContent := "---\ntitle: Product 1\ncategory: software\n---\nBody here.\n"
	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(mdContent),
		func() bool { return false },
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.notes/p1"); err != nil {
		t.Fatalf("Create via stdin Markdown: %v", err)
	}

	path := filepath.Join(dir, "$records", "p1.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	fileBytes, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile %s: %v", path, readErr)
	}
	fileStr := string(fileBytes)
	if !strings.Contains(fileStr, "title: Product 1") {
		t.Fatalf("expected frontmatter title, got: %s", fileStr)
	}
	if !strings.Contains(fileStr, "category: software") {
		t.Fatalf("expected frontmatter category, got: %s", fileStr)
	}
	if !strings.Contains(fileStr, "Body here.") {
		t.Fatalf("expected body in record, got: %s", fileStr)
	}
}

func TestCreate_StdinTOML(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testTOMLDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("name = \"Ireland\"\n"),
		func() bool { return false },
		nil,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.things/ie"); err != nil {
		t.Fatalf("Create via stdin TOML: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.toml")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file %s to be created: %v", path, err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile %s: %v", path, readErr)
	}
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}

func TestBuildRecordTemplate_Markdown(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatMarkdown,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"title":    {Type: ingitdb.ColumnTypeString},
			"category": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"title", "category"},
	}

	got := string(buildRecordTemplate(colDef))
	want := "---\ntitle: \ncategory: \n---\n"
	if got != want {
		t.Fatalf("markdown template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildRecordTemplate_YAML(t *testing.T) {
	t.Parallel()

	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString},
		},
		ColumnsOrder: []string{"name"},
	}

	got := string(buildRecordTemplate(colDef))
	want := "name: \n"
	if got != want {
		t.Fatalf("yaml template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestBuildRecordTemplate_AlphaFallback(t *testing.T) {
	t.Parallel()

	// No ColumnsOrder: columns appear alphabetically.
	colDef := &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"zebra": {Type: ingitdb.ColumnTypeString},
			"alpha": {Type: ingitdb.ColumnTypeString},
		},
	}

	got := string(buildRecordTemplate(colDef))
	want := "alpha: \nzebra: \n"
	if got != want {
		t.Fatalf("alphabetical template:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCreate_EditNoChanges(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}

	var logMessages []string
	logf := func(args ...any) {
		logMessages = append(logMessages, fmt.Sprint(args...))
	}

	// A no-op editor that does not modify the file.
	noOpEditor := func(_ string) error { return nil }

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		nil,
		func() bool { return true }, // TTY — would error without --edit
		noOpEditor,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/x", "--edit"); err != nil {
		t.Fatalf("expected no error for no-op edit, got: %v", err)
	}

	// Record file must NOT be written.
	if _, statErr := os.Stat(filepath.Join(dir, "$records", "x.yaml")); statErr == nil {
		t.Fatal("record file should not be created for a no-op edit")
	}

	// "no changes" must appear in a log message.
	found := false
	for _, msg := range logMessages {
		if strings.Contains(msg, "no changes") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected 'no changes' in log output, got: %v", logMessages)
	}
}

func TestCreate_EditInserts(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testDef(dir) // yaml collection: test.items, column: name

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	// An editor that overwrites the temp file with known YAML.
	writingEditor := func(tmpPath string) error {
		return os.WriteFile(tmpPath, []byte("name: Ireland\n"), 0o644)
	}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		nil,
		func() bool { return true },
		writingEditor,
	)
	if err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.items/ie", "--edit"); err != nil {
		t.Fatalf("Create via --edit: %v", err)
	}

	path := filepath.Join(dir, "$records", "ie.yaml")
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected file %s to be created: %v", path, statErr)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("ReadFile %s: %v", path, readErr)
	}
	if !strings.Contains(string(content), "Ireland") {
		t.Fatalf("expected record to contain Ireland, got: %s", content)
	}
}

func TestCreate_StdinINGRUnsupported(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := testINGRDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}

	cmd := Create(homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("name: x\n"),
		func() bool { return false }, // not a TTY
		nil,
	)
	err := runCobraCommand(cmd, "record", "--path="+dir, "--id=test.grid/x")
	if err == nil {
		t.Fatal("expected error for INGR format stdin (unsupported for SingleRecord)")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected 'unsupported' in error, got: %v", err)
	}
}
