package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestParseDiffRefs(t *testing.T) {
	t.Parallel()
	tests := []struct{ arg, from, to string }{
		{"", "HEAD", ""},
		{"v1", "v1", "HEAD"},
		{"a..b", "a", "b"},
		{"main..feature", "main", "feature"},
	}
	for _, tc := range tests {
		from, to := parseDiffRefs(tc.arg)
		if from != tc.from || to != tc.to {
			t.Errorf("parseDiffRefs(%q) = (%q,%q), want (%q,%q)", tc.arg, from, to, tc.from, tc.to)
		}
	}
}

func TestDiffFields(t *testing.T) {
	t.Parallel()
	before := map[string]any{"name": "Alice", "age": 30, "city": "NYC"}
	after := map[string]any{"name": "Alice", "age": 31, "zip": "10001"}
	got := diffFields(before, after)
	// changed: age (30->31), city (removed), zip (added) — name unchanged.
	gotNames := make([]string, len(got))
	for i, f := range got {
		gotNames[i] = f.Field
	}
	want := []string{"age", "city", "zip"} // sorted
	if strings.Join(gotNames, ",") != strings.Join(want, ",") {
		t.Errorf("changed fields = %v, want %v", gotNames, want)
	}
}

func TestDiffRecordSets(t *testing.T) {
	t.Parallel()
	before := map[string]map[string]any{
		"alice": {"age": 30},
		"carol": {"age": 40},
	}
	after := map[string]map[string]any{
		"alice": {"age": 31}, // updated
		"bob":   {"age": 20}, // added
		// carol deleted
	}
	got := diffRecordSets("people", before, after)
	kinds := map[string]diffKind{}
	for _, r := range got {
		kinds[r.Key] = r.Kind
	}
	if kinds["alice"] != diffUpdated || kinds["bob"] != diffAdded || kinds["carol"] != diffDeleted {
		t.Errorf("unexpected kinds: %+v", kinds)
	}
}

// diffTestRepo builds a git repo with a single-record "people" collection,
// two commits, and returns (dir, baseSha, readDef).
func diffTestRepo(t *testing.T) (string, string, func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error)) {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	disableGitBackgroundMaintenance(t, dir)
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")

	write := func(key, content string) {
		recDir := filepath.Join(dir, "people", "$records")
		if err := os.MkdirAll(recDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(recDir, key+".yaml"), []byte(content), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	write("alice", "name: Alice\nage: 30\n")
	write("carol", "name: Carol\nage: 40\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	base := strings.TrimSpace(string(runGit(t, dir, "rev-parse", "HEAD")))

	write("alice", "name: Alice\nage: 31\n") // updated
	write("bob", "name: Bob\nage: 20\n")     // added
	if err := os.Remove(filepath.Join(dir, "people", "$records", "carol.yaml")); err != nil {
		t.Fatalf("remove carol: %v", err)
	}
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "second")

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"people": {
					ID:      "people",
					DirPath: filepath.Join(dir, "people"),
					RecordFile: &ingitdb.RecordFileDef{
						Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
					},
					Columns: map[string]*ingitdb.ColumnDef{
						"name": {Type: ingitdb.ColumnTypeString},
						"age":  {Type: ingitdb.ColumnTypeInt},
					},
				},
			},
		}, nil
	}
	return dir, base, readDef
}

func runDiff(t *testing.T, dir, readDefBase string, readDef func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error), args ...string) (string, int) {
	t.Helper()
	exitCode := 0
	cmd := Diff(
		func() (string, error) { return "/tmp/home", nil },
		func() (string, error) { return dir, nil },
		readDef, func(...any) {},
		func(c int) { exitCode = c },
	)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(append([]string{readDefBase + "..HEAD", "--path=" + dir}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("diff %v: %v", args, err)
	}
	return buf.String(), exitCode
}

func TestDiff_EndToEnd(t *testing.T) {
	dir, base, readDef := diffTestRepo(t)

	// summary
	out, code := runDiff(t, dir, base, readDef) // default depth=summary
	if code != 1 {
		t.Errorf("expected exit 1 (changes), got %d", code)
	}
	if !strings.Contains(out, "people: +1 ~1 -1") {
		t.Errorf("summary mismatch: %s", out)
	}

	// record
	out, _ = runDiff(t, dir, base, readDef, "--depth=record")
	for _, want := range []string{"added   people/bob", "updated people/alice", "deleted people/carol"} {
		if !strings.Contains(out, want) {
			t.Errorf("record output missing %q:\n%s", want, out)
		}
	}

	// fields
	out, _ = runDiff(t, dir, base, readDef, "--depth=fields")
	if !strings.Contains(out, "fields: age") {
		t.Errorf("fields output should list changed field 'age':\n%s", out)
	}

	// full
	out, _ = runDiff(t, dir, base, readDef, "--depth=full")
	if !strings.Contains(out, "age: 30 -> 31") {
		t.Errorf("full output should show age before/after:\n%s", out)
	}

	// json
	out, _ = runDiff(t, dir, base, readDef, "--depth=record", "--format=json")
	var report diffReport
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("json output not valid: %v\n%s", err, out)
	}
	if len(report.Records) != 3 {
		t.Errorf("expected 3 record changes in json, got %d", len(report.Records))
	}
}

func TestDiff_NoChanges_Exit0(t *testing.T) {
	dir, _, readDef := diffTestRepo(t)
	// HEAD..HEAD → no changes.
	out, code := runDiff(t, dir, "HEAD", readDef)
	if code != 0 {
		t.Errorf("expected exit 0 (no changes), got %d", code)
	}
	if !strings.Contains(out, "no record changes") {
		t.Errorf("expected 'no record changes', got: %s", out)
	}
}

func TestDiff_InvalidFlags(t *testing.T) {
	dir, base, readDef := diffTestRepo(t)
	for _, args := range [][]string{{"--depth=bogus"}, {"--format=xml"}, {"--view=v1"}} {
		cmd := Diff(
			func() (string, error) { return "/tmp/home", nil },
			func() (string, error) { return dir, nil },
			readDef, func(...any) {}, func(int) {},
		)
		cmd.SetArgs(append([]string{base + "..HEAD", "--path=" + dir}, args...))
		if err := cmd.Execute(); err == nil {
			t.Errorf("expected error for args %v", args)
		}
	}
}
