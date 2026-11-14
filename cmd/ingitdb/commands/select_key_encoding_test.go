package commands

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
