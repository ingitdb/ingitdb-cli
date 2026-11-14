package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

const tildeDefinitionYAML = `record_file:
    name: '{key}.yaml'
    format: yaml
    type: map[string]any
    key_encoding: tilde-v1
columns:
    title:
        type: string
columns_order:
    - title
`

// select shows the decoded keys of a collection that declares
// record_file.key_encoding: tilde-v1, also in a collection-group read
// (ingitdb-go spec record-key-encoding#ac:readers-use-decoded-keys).
func TestSelect_KeyEncodingTildeV1ShowsDecodedKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for name, content := range map[string]string{
		".ingitdb/root-collections.yaml":                          "spaces: spaces\n",
		"spaces/.collection/definition.yaml":                      tildeDefinitionYAML,
		"spaces/.collection/subcollections/notes/definition.yaml": tildeDefinitionYAML,
		"spaces/$records/a~2Fb.yaml":                              "title: A\n",
		"spaces/$records/~43ON.yaml":                              "title: C\n",
		"spaces/$records/a~2Fb/notes/$records/~2E~2E.yaml":        "title: N\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	stdout, err := runNestedSelect(t, dir, "--from=spaces", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	ids := rowIDs(decodeJSONRows(t, stdout))
	slices.Sort(ids)
	if strings.Join(ids, " ") != "CON a/b" {
		t.Errorf("ids = %q", ids)
	}
	stdout, err = runNestedSelect(t, dir, "--from=spaces/notes", "--group", "--format=json")
	if err != nil {
		t.Fatal(err)
	}
	// The group read's $id is the record's path; with a "/" in a parent id the
	// path is ambiguous to a reader (reported, not changed here).
	if ids := rowIDs(decodeJSONRows(t, stdout)); strings.Join(ids, " ") != "spaces/a/b/notes/.." {
		t.Errorf("group ids = %q\n%s", ids, stdout)
	}
}

// diff shows decoded keys, and a batch insert rolls back the files it wrote
// under their encoded names.
func TestKeyEncodingHelpers_DiffAndBatchRollbackPaths(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{ID: "spaces", DirPath: "/db/spaces",
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord, KeyEncoding: ingitdb.KeyEncodingTildeV1},
		Columns:    map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}}}
	got := parseKeyedRecords([]byte("title: x\n"), col, "spaces/$records/a~2Fb.yaml")
	if _, ok := got["a/b"]; !ok || len(got) != 1 {
		t.Errorf("diff key: %v", got)
	}
	got = parseKeyedRecords([]byte("title: x\n"), col, "spaces/$records/a b.yaml")
	if _, ok := got["a b"]; !ok {
		t.Errorf("a name that does not decode is shown as it is: %v", got)
	}
	if p := resolveBatchRecordPath(col, "../.."); p != filepath.Join("/db/spaces", "$records", "~2E.~2F.~2E.yaml") {
		t.Errorf("rollback path: %s", p)
	}
	none := *col
	rf := *col.RecordFile
	rf.KeyEncoding = ""
	none.RecordFile = &rf
	if p := resolveBatchRecordPath(&none, "a b"); p != filepath.Join("/db/spaces", "$records", "a b.yaml") {
		t.Errorf("no encoding: %s", p)
	}
}
