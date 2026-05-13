package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2fsingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// insertTestDeps returns a minimal DI set for the Insert command.
// stdin/isStdinTTY/openEditor default to inert values; tests that
// exercise those paths override them via runInsertCmd's variants.
func insertTestDeps(t *testing.T, dir string) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	def := testDef(dir)
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return dir, nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB = func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf = func(...any) {}
	return
}

// runInsertCmd invokes the Insert command with stdin set to the given
// reader and stdin-TTY simulation flag, captures stdout, and returns
// the captured output + any error.
func runInsertCmd(
	t *testing.T,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
	stdin io.Reader,
	stdinIsTTY bool,
	openEditor func(string) error,
	args ...string,
) (string, error) {
	t.Helper()
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, stdin, func() bool { return stdinIsTTY }, openEditor)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestInsert_RegistersAllFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	cmd := Insert(homeDir, getWd, readDef, newDB, logf, strings.NewReader(""), func() bool { return true }, nil)
	for _, name := range []string{"into", "key", "data", "edit", "empty", "path", "remote"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("flag --%s not registered", name)
		}
	}
}

func TestInsert_RequiresInto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--key=x", "--data={}",
	)
	if err == nil {
		t.Fatal("expected error when --into is missing")
	}
	if !strings.Contains(err.Error(), "into") {
		t.Errorf("error should mention --into, got: %v", err)
	}
}

func TestInsert_RejectsForbiddenSharedFlags(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
		want string // substring expected in error
	}{
		{name: "from rejected", args: []string{"--into=test.items", "--from=other", "--key=x", "--data={}"}, want: "from"},
		{name: "id rejected", args: []string{"--into=test.items", "--id=test.items/x", "--data={}"}, want: "id"},
		{name: "where rejected", args: []string{"--into=test.items", "--key=x", "--data={}", "--where=a==1"}, want: "where"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Errorf("expected error to mention %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestInsert_DataSource_DataFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=alpha", "--data={title: Alpha}",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Stdin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader("title: FromStdin"), false /* not a TTY */, nil,
		"--path="+dir, "--into=test.items", "--key=beta",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=gamma", "--empty",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_Edit(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Editor stub: replace template with real content.
	openEditor := func(tmpPath string) error {
		return os.WriteFile(tmpPath, []byte("title: FromEditor\n"), 0o644)
	}
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, openEditor,
		"--path="+dir, "--into=test.items", "--key=delta", "--edit",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestInsert_DataSource_EditUnchanged(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Editor stub: leave the file unmodified (no write).
	openEditor := func(tmpPath string) error { return nil }
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, openEditor,
		"--path="+dir, "--into=test.items", "--key=epsilon", "--edit",
	)
	if err == nil {
		t.Fatal("expected error when editor exits without modifying template")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "not edited") &&
		!strings.Contains(strings.ToLower(err.Error()), "no changes") &&
		!strings.Contains(strings.ToLower(err.Error()), "unchanged") {
		t.Errorf("error should mention the template was unchanged, got: %v", err)
	}
}

func TestInsert_DataSource_None(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true /* TTY stdin */, nil,
		"--path="+dir, "--into=test.items", "--key=zeta",
	)
	if err == nil {
		t.Fatal("expected error when no data source supplied (TTY stdin, no --data/--edit/--empty)")
	}
}

func TestInsert_DataSource_MutualExclusion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	cases := []struct {
		name string
		args []string
	}{
		{name: "data + empty", args: []string{"--into=test.items", "--key=x", "--data={a: 1}", "--empty"}},
		{name: "data + edit", args: []string{"--into=test.items", "--key=x", "--data={a: 1}", "--edit"}},
		{name: "edit + empty", args: []string{"--into=test.items", "--key=x", "--edit", "--empty"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				append([]string{"--path=" + dir}, tc.args...)...,
			)
			if err == nil {
				t.Fatalf("expected error for %v", tc.args)
			}
		})
	}
}

func TestInsert_Key_FromDataIDField(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// $id provides the key; no --key flag.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--data={$id: from-data, title: Eta}",
	)
	if err != nil {
		t.Fatalf("expected success when $id provides the key, got: %v", err)
	}

	// Verify the record was written at the key from $id and $id is NOT
	// stored as a data field. Read the file directly to confirm.
	// testDef uses RecordFile.Name="{key}.yaml", so RecordsBasePath()="$records".
	got, readErr := os.ReadFile(filepath.Join(dir, "$records", "from-data.yaml"))
	if readErr != nil {
		t.Fatalf("read inserted record: %v", readErr)
	}
	if strings.Contains(string(got), "$id:") {
		t.Errorf("$id MUST NOT appear in the stored record file:\n%s", string(got))
	}
	if !strings.Contains(string(got), "title: Eta") {
		t.Errorf("expected title: Eta in the stored record, got:\n%s", string(got))
	}
}

func TestInsert_Key_FlagAndDataIDConsistent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Both supplied AND equal: proceed.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=theta", "--data={$id: theta, title: Theta}",
	)
	if err != nil {
		t.Fatalf("expected success when --key and $id match, got: %v", err)
	}
}

func TestInsert_Key_FlagAndDataIDMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// Both supplied AND differ: reject, name both values.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=iota", "--data={$id: kappa, title: Iota}",
	)
	if err == nil {
		t.Fatal("expected error when --key and $id differ")
	}
	if !strings.Contains(err.Error(), "iota") || !strings.Contains(err.Error(), "kappa") {
		t.Errorf("error should name both keys (iota, kappa), got: %v", err)
	}
}

func TestInsert_Key_Missing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// No --key, no $id in data: reject.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--data={title: NoKey}",
	)
	if err == nil {
		t.Fatal("expected error when no key supplied")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "key") {
		t.Errorf("error should mention key, got: %v", err)
	}
}

func TestInsert_RejectsExistingKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	// First insert succeeds.
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=collision", "--data={title: Original}",
	)
	if err != nil {
		t.Fatalf("first insert should succeed: %v", err)
	}

	// Second insert with same key must fail.
	_, err = runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=collision", "--data={title: Replacement}",
	)
	if err == nil {
		t.Fatal("expected error on duplicate key")
	}
	if !strings.Contains(err.Error(), "collision") {
		t.Errorf("error should name the conflicting key, got: %v", err)
	}

	// Confirm the original record was NOT overwritten.
	// testDef stores records under $records/ inside dir (RecordsBasePath of "{key}.yaml").
	got, readErr := os.ReadFile(filepath.Join(dir, "$records", "collision.yaml"))
	if readErr != nil {
		t.Fatalf("read original record: %v", readErr)
	}
	if !strings.Contains(string(got), "Original") {
		t.Errorf("original record was modified, got:\n%s", string(got))
	}
	if strings.Contains(string(got), "Replacement") {
		t.Errorf("original record was overwritten, got:\n%s", string(got))
	}
}

// insertMarkdownTestDeps is like insertTestDeps but uses testMarkdownDef
// so the test can target the test.notes markdown collection.
func insertMarkdownTestDeps(t *testing.T, dir string) (
	homeDir func() (string, error),
	getWd func() (string, error),
	readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	newDB func(string, *ingitdb.Definition) (dal.DB, error),
	logf func(...any),
) {
	t.Helper()
	def := testMarkdownDef(dir)
	homeDir = func() (string, error) { return "/tmp/home", nil }
	getWd = func() (string, error) { return dir, nil }
	readDef = func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB = func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf = func(...any) {}
	return
}

func TestInsert_MarkdownFromStdin(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertMarkdownTestDeps(t, dir)

	mdContent := "---\ntitle: Hello World\ntags: [intro, demo]\n---\n\nThis is the body of the post.\n"
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(mdContent), false /* not TTY */, nil,
		"--path="+dir, "--into=test.notes", "--key=hello",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Read the stored file and verify it contains BOTH frontmatter
	// (title) and the body (under the $content convention).
	colDef := testMarkdownDef(dir).Collections["test.notes"]
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.RecordFile.RecordsBasePath(), "hello.md"))
	if readErr != nil {
		t.Fatalf("read stored file: %v", readErr)
	}
	got := string(stored)
	if !strings.Contains(got, "title: Hello World") {
		t.Errorf("stored markdown should contain title from frontmatter:\n%s", got)
	}
	if !strings.Contains(got, "This is the body") {
		t.Errorf("stored markdown should contain body:\n%s", got)
	}
}

func TestInsert_MarkdownDollarIDFromFrontmatter(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertMarkdownTestDeps(t, dir)

	// $id in markdown frontmatter provides the key; no --key flag.
	mdContent := "---\n$id: from-frontmatter\ntitle: Auto-Keyed\n---\n\nBody here.\n"
	_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf,
		strings.NewReader(mdContent), false, nil,
		"--path="+dir, "--into=test.notes",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	colDef := testMarkdownDef(dir).Collections["test.notes"]
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.RecordFile.RecordsBasePath(), "from-frontmatter.md"))
	if readErr != nil {
		t.Fatalf("read stored file: %v", readErr)
	}
	// $id must NOT appear in the stored frontmatter (it's metadata).
	if strings.Contains(string(stored), "$id:") {
		t.Errorf("$id must be stripped from stored frontmatter, got:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "title: Auto-Keyed") {
		t.Errorf("stored frontmatter should contain title:\n%s", string(stored))
	}
}

func TestInsert_EndToEnd_RealisticInvocation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)

	stdout, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
		"--path="+dir, "--into=test.items", "--key=e2e",
		"--data={title: End-to-End, priority: 3, active: true}",
	)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if stdout != "" {
		t.Errorf("success path MUST be silent on stdout, got: %q", stdout)
	}

	// Verify the record landed.
	colDef := testDef(dir).Collections["test.items"]
	stored, readErr := os.ReadFile(filepath.Join(dir, colDef.RecordFile.RecordsBasePath(), "e2e.yaml"))
	if readErr != nil {
		t.Fatalf("read inserted record: %v", readErr)
	}
	if !strings.Contains(string(stored), "title: End-to-End") {
		t.Errorf("expected title in stored record:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "priority: 3") {
		t.Errorf("expected priority in stored record:\n%s", string(stored))
	}
	if !strings.Contains(string(stored), "active: true") {
		t.Errorf("expected active flag in stored record:\n%s", string(stored))
	}
}

func TestInsert_RejectsInvalidFormatValue(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		format string
	}{
		{"xml", "xml"},
		{"markdown", "markdown"},
		{"empty-but-set", ""},
		{"yaml-stream typo", "yaml-stream"},
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			homeDir, getWd, readDef, newDB, logf := insertTestDeps(t, dir)
			_, err := runInsertCmd(t, homeDir, getWd, readDef, newDB, logf, nil, true, nil,
				"--path="+dir,
				"--into=test.items",
				"--format="+tt.format,
			)
			if err == nil {
				t.Fatalf("expected error for --format=%q, got nil", tt.format)
			}
			msg := err.Error()
			for _, want := range []string{"jsonl", "yaml", "ingr", "csv"} {
				if !strings.Contains(msg, want) {
					t.Errorf("error message %q should list %q as a valid format", msg, want)
				}
			}
		})
	}
}
