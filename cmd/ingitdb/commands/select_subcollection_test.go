package commands

// specscore: feature/subcollection-addressing

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"

	"github.com/ingitdb/ingitdb-cli/internal/testutil"
)

// runNestedSelect runs `select` against the nested lists fixture at dir with
// the real definition reader and local driver.
func runNestedSelect(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
	}
	logf := func(...any) {}
	all := append([]string{"--path=" + dir}, args...)
	return runSelectCmd(t, homeDir, getWd, validator.ReadDefinition, newDB, logf, all...)
}

func decodeJSONRows(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout is not a JSON array: %v\n%s", err, stdout)
	}
	return rows
}

func rowIDs(rows []map[string]any) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		id, _ := r["$id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func TestSelect_Subcollection_EveryFormat(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	cases := []struct {
		name string
		args []string
	}{
		{name: "default-csv"},
		{name: "csv", args: []string{"--format=csv"}},
		{name: "json", args: []string{"--format=json"}},
		{name: "yaml", args: []string{"--format=yaml"}},
		{name: "md", args: []string{"--format=md"}},
		{name: "ingr", args: []string{"--format=ingr"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			args := append([]string{"--from=lists/to-buy/items"}, tc.args...)
			stdout, err := runNestedSelect(t, dir, args...)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			for _, want := range []string{"Milk", "Bananas", "Coffee"} {
				if !strings.Contains(stdout, want) {
					t.Errorf("missing %q in output:\n%s", want, stdout)
				}
			}
			for _, notWant := range []string{"The Matrix", "Interstellar", "To buy"} {
				if strings.Contains(stdout, notWant) {
					t.Errorf("unexpected %q in output:\n%s", notWant, stdout)
				}
			}
		})
	}
}

func TestSelect_Subcollection_DefaultIsCSV(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	stdout, err := runNestedSelect(t, dir, "--from=lists/to-watch/items", "--fields=$id,title")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || lines[0] != "$id,title" {
		t.Fatalf("want CSV header plus 2 rows, got:\n%s", stdout)
	}
}

func TestSelect_Subcollection_OrderByAddedAt(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	stdout, err := runNestedSelect(t, dir, "--from=lists/to-buy/items", "--order-by=added_at", "--format=json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rows := decodeJSONRows(t, stdout)
	var titles []string
	for _, r := range rows {
		title, _ := r["title"].(string)
		titles = append(titles, title)
		if done, _ := r["done"].(bool); done {
			t.Errorf("item %v: done = true, want false", r["$id"])
		}
	}
	if got := strings.Join(titles, ","); got != "Milk,Bananas,Coffee" {
		t.Errorf("titles = %s, want Milk,Bananas,Coffee", got)
	}
	stdout, err = runNestedSelect(t, dir, "--from=lists/to-buy/items", "--order-by=-added_at", "--format=json")
	if err != nil {
		t.Fatalf("run desc: %v", err)
	}
	if got := strings.Join(rowIDs(decodeJSONRows(t, stdout)), ","); got != "coffee,bananas,milk" {
		t.Errorf("descending ids = %s, want coffee,bananas,milk", got)
	}
}

func TestSelect_Subcollection_WhereFieldsLimit(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)

	stdout, err := runNestedSelect(t, dir, "--from=lists/to-buy/items", "--where=title==Bananas", "--format=json")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	if got := strings.Join(rowIDs(decodeJSONRows(t, stdout)), ","); got != "bananas" {
		t.Errorf("--where ids = %s, want bananas", got)
	}

	stdout, err = runNestedSelect(t, dir, "--from=lists/to-buy/items", "--where=$id===coffee", "--fields=title", "--format=json")
	if err != nil {
		t.Fatalf("fields: %v", err)
	}
	rows := decodeJSONRows(t, stdout)
	if len(rows) != 1 || len(rows[0]) != 1 || rows[0]["title"] != "Coffee" {
		t.Errorf("--fields=title rows = %v, want [{title: Coffee}]", rows)
	}

	stdout, err = runNestedSelect(t, dir, "--from=lists/to-buy/items", "--order-by=added_at", "--limit=2", "--format=json")
	if err != nil {
		t.Fatalf("limit: %v", err)
	}
	if got := strings.Join(rowIDs(decodeJSONRows(t, stdout)), ","); got != "milk,bananas" {
		t.Errorf("--limit=2 ids = %s, want milk,bananas", got)
	}

	_, err = runNestedSelect(t, dir, "--from=lists/to-buy/items", "--min-affected=4")
	testutil.MustErrContain(t, err, "matched 3 records, required at least 4")
}

func TestSelect_Subcollection_UnknownParentIsEmpty(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	stdout, err := runNestedSelect(t, dir, "--from=lists/nothing/items", "--format=json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("want [], got:\n%s", stdout)
	}

	// csv (explicit and default): same empty-result output as an empty root
	// collection result. The missing empty-csv header is a pre-existing gap
	// (https://github.com/ingitdb/ingitdb-cli/issues/155); this asserts parity only.
	rootEmpty, err := runNestedSelect(t, dir, "--from=lists", "--where=title==zzz")
	if err != nil {
		t.Fatalf("root empty: %v", err)
	}
	for _, args := range [][]string{{"--from=lists/nothing/items"}, {"--from=lists/nothing/items", "--format=csv"}} {
		got, runErr := runNestedSelect(t, dir, args...)
		if runErr != nil {
			t.Fatalf("%v: %v", args, runErr)
		}
		if got != rootEmpty {
			t.Errorf("%v: output %q, want the root empty-result output %q", args, got, rootEmpty)
		}
	}
}

func TestSelect_Subcollection_UndeclaredSegmentNotFound(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	for _, from := range []string{
		"lists/to-buy/notes",
		"lists/to-buy",
		"nope/to-buy/items",
		"lists//items",
		"lists/to-buy/items/milk",
		"lists/to-buy/items/milk/tags",
		"/lists/to-buy/items",
		"lists/../items",
		"lists/./items",
		"lists/to-buy/../to-buy/items",
		`lists/to\buy/items`,
		`lists/..\to-buy/items`,
		"lists/to-buy/items/",
	} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()
			stdout, err := runNestedSelect(t, dir, "--from="+from)
			testutil.MustErrContain(t, err, fmt.Sprintf("collection %q not found in definition", from))
			if stdout != "" {
				t.Errorf("stdout should be empty, got:\n%s", stdout)
			}
		})
	}
}

func TestSelect_Subcollection_RootFromUnchanged(t *testing.T) {
	t.Parallel()
	dir := testutil.WriteNestedListsDB(t)
	stdout, err := runNestedSelect(t, dir, "--from=lists", "--format=json")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	rows := decodeJSONRows(t, stdout)
	got := map[string]any{}
	for _, r := range rows {
		id, _ := r["$id"].(string)
		got[id] = r["title"]
	}
	if len(rows) != 2 || got["to-buy"] != "To buy" || got["to-watch"] != "To watch" {
		t.Errorf("root lists = %v, want to-buy/To buy and to-watch/To watch", rows)
	}

	_, err = runNestedSelect(t, dir, "--from=missing")
	testutil.MustErrContain(t, err, `collection "missing" not found in definition`)
}

// Swaps the package-level gitHubFileReaderFactory seam, so not parallel.
func TestSelect_Subcollection_RemoteRejected(t *testing.T) {
	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb/root-collections.yaml": []byte("lists: lists\n"),
	}}
	orig := gitHubFileReaderFactory
	gitHubFileReaderFactory = &constantFileReaderFactory{reader: reader}
	defer func() { gitHubFileReaderFactory = orig }()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		t.Fatal("local definition must not be read for --remote")
		return nil, nil
	}
	newDB := func(string, *ingitdb.Definition) (dal.DB, error) {
		t.Fatal("local database must not be opened for --remote")
		return nil, nil
	}
	stdout, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
		"--remote=github.com/owner/repo", "--from=lists/to-buy/items")
	testutil.MustErrContain(t, err, `failed to read remote definition: collection "lists/to-buy/items" not found in root config`)
	if stdout != "" {
		t.Errorf("stdout should be empty, got:\n%s", stdout)
	}
}
