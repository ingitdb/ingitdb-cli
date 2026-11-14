package commands

// specscore: feature/subcollection-addressing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/internal/testutil"
)

// Verifies subcollection-addressing#ac:select-collection-group.
func TestSelect_Group_ReadsEveryInstance(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteDeepNestedListsDB(t)
	// An instance below a parent with no record file is read too.
	ghost := filepath.Join(dir, "lists", "ghost", "items", "$records")
	if err := os.MkdirAll(ghost, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ghost, "g1.yaml"), []byte("title: Ghost item\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, err := runNestedSelect(t, dir, "--from=lists/items", "--group", "--format=json", "--order-by=$id", "--fields=$id,title")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := strings.Join(rowIDs(decodeJSONRows(t, stdout)), " ")
	want := "lists/ghost/items/g1 lists/items/items/self lists/to-buy/items/bananas lists/to-buy/items/coffee lists/to-buy/items/milk lists/to-watch/items/interstellar lists/to-watch/items/the-matrix"
	if got != want {
		t.Fatalf("$id values:\n got %s\nwant %s", got, want)
	}
}

func TestSelect_Group_DeepAndRootAndSetFlags(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteDeepNestedListsDB(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"deep", []string{"--from=lists/items/tags", "--order-by=$id"}, "lists/to-buy/items/milk/tags/dairy lists/to-buy/items/milk/tags/fresh"},
		{"root", []string{"--from=lists", "--order-by=$id"}, "lists/items lists/to-buy lists/to-watch"},
		{"where on field", []string{"--from=lists/items", "--where=title==Milk"}, "lists/to-buy/items/milk"},
		{"where on full $id", []string{"--from=lists/items", "--where=$id==lists/to-watch/items/interstellar"}, "lists/to-watch/items/interstellar"},
		{"order and limit", []string{"--from=lists/items", "--order-by=-title", "--limit=2"}, "lists/to-watch/items/the-matrix lists/items/items/self"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--group", "--format=json"}, tc.args...)
			stdout, err := runNestedSelect(t, dir, args...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			if got := strings.Join(rowIDs(decodeJSONRows(t, stdout)), " "); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestSelect_Group_EmptyAndErrors(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteDeepNestedListsDB(t)
	if err := os.RemoveAll(filepath.Join(dir, "lists", "to-buy", "items", "milk")); err != nil {
		t.Fatal(err)
	}
	stdout, err := runNestedSelect(t, dir, "--from=lists/items/tags", "--group", "--format=json")
	if err != nil {
		t.Fatalf("no instances must be an empty result, got %v", err)
	}
	if rows := decodeJSONRows(t, stdout); len(rows) != 0 {
		t.Fatalf("expected no rows, got %v", rows)
	}
	errorCases := []struct {
		args []string
		want string
	}{
		{[]string{"--from=lists/nope", "--group"}, `collection "lists/nope" not found in definition`},
		{[]string{"--from=lists/../items", "--group"}, `collection "lists/../items" not found in definition`},
		{[]string{"--from=lists/to-buy/items", "--group"}, `collection "lists/to-buy/items" not found in definition`},
		{[]string{"--id=lists/to-buy", "--group"}, "--group requires --from"},
	}
	for _, tc := range errorCases {
		stdout, err := runNestedSelect(t, dir, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) || stdout != "" {
			t.Errorf("%v: expected error %q and no output, got %v, %q", tc.args, tc.want, err, stdout)
		}
	}
}

func TestSelect_Group_RemoteRejected(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp", nil }
	_, err := runSelectCmd(t, homeDir, getWd, nil, nil, func(...any) {}, "--remote=github.com/example/db", "--from=lists/items", "--group")
	if err == nil || !strings.Contains(err.Error(), "--group is supported for local databases only") {
		t.Fatalf("expected local-only error, got %v", err)
	}
}

// Without --group, a schema path is still not a --from value.
func TestSelect_SchemaPathWithoutGroupUnchanged(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteDeepNestedListsDB(t)
	_, err := runNestedSelect(t, dir, "--from=lists/items/tags")
	if err == nil || !strings.Contains(err.Error(), `collection "lists/items/tags" not found in definition`) {
		t.Fatalf("expected not found, got %v", err)
	}
}
