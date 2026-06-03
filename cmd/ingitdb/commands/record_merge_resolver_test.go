package commands

// specscore: feature/cli/resolve/auto-resolve/record-merge

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/recordmerge"
)

// setupDataConflict creates a git repo with a three-way conflict on relPath:
// base committed, then main and feature each commit divergent content, then a
// failing merge leaves the file unmerged with stages :1/:2/:3 populated.
func setupDataConflict(t *testing.T, relPath, base, main, feature string) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")

	full := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	writeFile(t, full, base)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")
	runGit(t, dir, "branch", "-m", "main")
	runGit(t, dir, "branch", "feature")

	writeFile(t, full, main)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "main")

	runGit(t, dir, "checkout", "feature")
	writeFile(t, full, feature)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "feature")
	runGit(t, dir, "checkout", "main")

	mergeCmd := exec.Command("git", "merge", "--no-ff", "feature")
	mergeCmd.Dir = dir
	_ = mergeCmd.Run() // expected to fail with a conflict

	out := runGit(t, dir, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(string(out)) == "" {
		t.Skip("git auto-merged; no conflict produced")
	}
	return dir
}

func mapColDef(dir, relPath string) *ingitdb.Definition {
	colDir := filepath.Dir(filepath.Join(dir, relPath))
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       filepath.Base(relPath),
					Format:     ingitdb.RecordFormatYAML,
					RecordType: ingitdb.MapOfRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{"v": {Type: ingitdb.ColumnTypeString}},
			},
		},
	}
}

func readMap(t *testing.T, dir, relPath string) map[string]map[string]any {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(dir, relPath))
	if err != nil {
		t.Fatalf("read merged file: %v", err)
	}
	m, err := dalgo2ingitdb.ParseMapOfRecordsContent(content, ingitdb.RecordFormatYAML)
	if err != nil {
		t.Fatalf("parse merged file: %v", err)
	}
	return m
}

func TestResolveRecordMergeConflicts_DisjointAdditions(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)

	resolved, unresolved, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(unresolved) != 0 || len(resolved) != 1 {
		t.Fatalf("resolved=%v unresolved=%v, want 1 resolved 0 unresolved", resolved, unresolved)
	}
	m := readMap(t, dir, rel)
	for _, k := range []string{"x", "a", "b"} {
		if _, ok := m[k]; !ok {
			t.Errorf("merged file missing record %q (got %v)", k, m)
		}
	}
	// The merged file must be staged (no longer unmerged).
	out := runGit(t, dir, "diff", "--name-only", "--diff-filter=U")
	if strings.TrimSpace(string(out)) != "" {
		t.Errorf("file still unmerged: %s", out)
	}
}

func TestGitStageContent_MissingStageReturnsNil(t *testing.T) {
	t.Parallel()
	// A normally-committed file (no conflict) has no index stages, so reading
	// stage :1 fails and must yield nil rather than an error.
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	writeFile(t, filepath.Join(dir, "data.yaml"), "x:\n  v: '0'\n")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "base")

	if got := gitStageContent(context.Background(), dir, "data.yaml", 1); got != nil {
		t.Fatalf("expected nil for absent stage, got %q", got)
	}
}

func TestResolveRecordMergeConflicts_CSVListEndToEnd(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.csv")
	dir := setupDataConflict(t, rel,
		"$id,v\nx,0\n",
		"$id,v\nx,0\na,1\n",
		"$id,v\nx,0\nb,2\n",
	)
	colDir := filepath.Dir(filepath.Join(dir, rel))
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:           "c",
				DirPath:      colDir,
				ColumnsOrder: []string{"$id", "v"},
				RecordFile: &ingitdb.RecordFileDef{
					Name: "data.csv", Format: ingitdb.RecordFormatCSV, RecordType: ingitdb.ListOfRecords,
				},
			},
		},
	}

	resolved, unresolved, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resolved) != 1 || len(unresolved) != 0 {
		t.Fatalf("resolved=%v unresolved=%v, want CSV merged", resolved, unresolved)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, rel))
	if readErr != nil {
		t.Fatalf("read merged: %v", readErr)
	}
	for _, want := range []string{"$id,v", "x,0", "a,1", "b,2"} {
		if !strings.Contains(string(content), want) {
			t.Errorf("merged CSV missing %q:\n%s", want, content)
		}
	}
}

func TestResolveRecordMergeConflicts_CollisionEscalates(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\na:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)

	resolved, unresolved, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resolved) != 0 || len(unresolved) != 1 {
		t.Fatalf("resolved=%v unresolved=%v, want collision to escalate", resolved, unresolved)
	}
}

func TestResolveRecordMergeConflicts_NoCollection(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := &ingitdb.Definition{} // no collections

	resolved, unresolved, _ := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if len(resolved) != 0 || len(unresolved) != 1 {
		t.Fatalf("resolved=%v unresolved=%v, want unresolved when no collection", resolved, unresolved)
	}
}

func TestResolveRecordMergeConflicts_DisabledByConfig(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)
	disabled := false
	def.Collections["c"].ConflictResolution = &ingitdb.ConflictResolutionConfig{
		RecordMerge: &ingitdb.RecordMergeConfig{Enabled: &disabled},
	}

	resolved, unresolved, _ := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if len(resolved) != 0 || len(unresolved) != 1 {
		t.Fatalf("resolved=%v unresolved=%v, want unresolved when disabled", resolved, unresolved)
	}
}

func TestResolveRecordMergeConflicts_SameRecordGated(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	base := "r:\n  name: x\n  email: e\n"
	main := "r:\n  name: y\n  email: e\n"
	feature := "r:\n  name: x\n  email: z\n"

	// Disabled: escalates.
	dir := setupDataConflict(t, rel, base, main, feature)
	def := mapColDef(dir, rel)
	_, unresolved, _ := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if len(unresolved) != 1 {
		t.Fatalf("want escalate when same-record disabled, got unresolved=%v", unresolved)
	}

	// Enabled: merges both field edits.
	dir2 := setupDataConflict(t, rel, base, main, feature)
	def2 := mapColDef(dir2, rel)
	enabled := true
	def2.Collections["c"].ConflictResolution = &ingitdb.ConflictResolutionConfig{
		RecordMerge: &ingitdb.RecordMergeConfig{SameRecord: &enabled},
	}
	resolved, unresolved2, err := resolveRecordMergeConflicts(context.Background(), dir2, def2, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resolved) != 1 || len(unresolved2) != 0 {
		t.Fatalf("resolved=%v unresolved=%v, want merged when same-record enabled", resolved, unresolved2)
	}
	m := readMap(t, dir2, rel)
	if m["r"]["name"] != "y" || m["r"]["email"] != "z" {
		t.Errorf("merged record = %v, want name=y email=z", m["r"])
	}
}

func TestResolveRecordMergeConflicts_WriteError(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)

	// Replace the working-tree file with a directory so WriteFile fails.
	full := filepath.Join(dir, rel)
	if err := os.Remove(full); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, _, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err == nil || !strings.Contains(err.Error(), "write merged") {
		t.Fatalf("expected write error, got %v", err)
	}
}

func TestResolveRecordMergeConflicts_GitAddError(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"x:\n  v: '0'\n",
		"x:\n  v: '0'\na:\n  v: '1'\n",
		"x:\n  v: '0'\nb:\n  v: '2'\n",
	)
	def := mapColDef(dir, rel)
	lockGitIndex(t, dir)

	_, _, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err == nil || !strings.Contains(err.Error(), "stage merged") {
		t.Fatalf("expected git add error, got %v", err)
	}
}

func TestResolveRecordMergeConflicts_MarkdownDifferentFieldsMerge(t *testing.T) {
	t.Parallel()
	// A markdown single-record conflict whose two sides edit different
	// frontmatter fields: the engine merges them and markdown is re-serialized.
	rel := filepath.Join("c", "note.md")
	dir := setupDataConflict(t, rel,
		"---\ntitle: x\nstatus: a\n---\nbody\n",
		"---\ntitle: y\nstatus: a\n---\nbody\n",
		"---\ntitle: x\nstatus: b\n---\nbody\n",
	)

	colDir := filepath.Dir(filepath.Join(dir, rel))
	enabled := true
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:      "c",
				DirPath: colDir,
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "note.md",
					Format:     ingitdb.RecordFormatMarkdown,
					RecordType: ingitdb.SingleRecord,
				},
				ColumnsOrder: []string{"title", "status"},
				Columns: map[string]*ingitdb.ColumnDef{
					"title":  {Type: ingitdb.ColumnTypeString},
					"status": {Type: ingitdb.ColumnTypeString},
				},
				ConflictResolution: &ingitdb.ConflictResolutionConfig{
					RecordMerge: &ingitdb.RecordMergeConfig{SameRecord: &enabled},
				},
			},
		},
	}

	resolved, unresolved, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resolved) != 1 || len(unresolved) != 0 {
		t.Fatalf("resolved=%v unresolved=%v, want markdown merged", resolved, unresolved)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, rel))
	if readErr != nil {
		t.Fatalf("read merged: %v", readErr)
	}
	merged, parseErr := dalgo2ingitdb.ParseRecordContentForCollection(content, def.Collections["c"])
	if parseErr != nil {
		t.Fatalf("parse merged markdown: %v\n%s", parseErr, content)
	}
	if merged["title"] != "y" || merged["status"] != "b" {
		t.Errorf("merged frontmatter = title=%v status=%v, want y/b", merged["title"], merged["status"])
	}
}

func TestMergeAndSerialize_SerializeFailureNotOK(t *testing.T) {
	t.Parallel()
	// A single-record collection declared with the CSV format parses (header
	// matches) but cannot be serialized as a single record map, so the merge
	// is reported as not-ok (escalate) rather than producing a broken file.
	col := &ingitdb.CollectionDef{
		ColumnsOrder: []string{"$id", "v"},
		RecordFile: &ingitdb.RecordFileDef{
			Name: "x.csv", Format: ingitdb.RecordFormatCSV, RecordType: ingitdb.SingleRecord,
		},
	}
	csv := []byte("$id,v\nx,1\n")
	if _, ok := mergeAndSerialize(csv, csv, csv, col, recordmerge.Options{}); ok {
		t.Fatal("expected mergeAndSerialize to report not-ok on serialize failure")
	}
}

func TestMergeAndSerialize_InvalidSchemaEscalates(t *testing.T) {
	t.Parallel()
	// A map-of-records collection with a required "name" column. Each side adds
	// a disjoint record (DM-1), so the merge does not textually escalate — but
	// THEIRS' record omits the required "name", so the merged result fails
	// schema validation and MUST escalate (AC:invalid-merge-escalates).
	col := &ingitdb.CollectionDef{
		ID:           "c",
		ColumnsOrder: []string{"name", "note"},
		RecordFile: &ingitdb.RecordFileDef{
			Name: "data.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords,
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"name": {Type: ingitdb.ColumnTypeString, Required: true},
			"note": {Type: ingitdb.ColumnTypeString},
		},
	}
	base := []byte("{}\n")
	ours := []byte("a:\n  name: Alice\n")
	theirsInvalid := []byte("b:\n  note: world\n") // missing required "name"
	if _, ok := mergeAndSerialize(base, ours, theirsInvalid, col, recordmerge.Options{}); ok {
		t.Fatal("expected escalation when the merged result fails schema validation")
	}

	// Control: a disjoint addition that satisfies the schema must still merge.
	theirsValid := []byte("b:\n  name: Bob\n")
	if _, ok := mergeAndSerialize(base, ours, theirsValid, col, recordmerge.Options{}); !ok {
		t.Fatal("expected a schema-valid disjoint merge to succeed")
	}
}

func TestSerializeMergedRecords(t *testing.T) {
	t.Parallel()

	t.Run("single record", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{RecordFile: &ingitdb.RecordFileDef{
			Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
		}}
		got, err := serializeMergedRecords([]recordmerge.Record{{Fields: map[string]any{"name": "y"}}}, col)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !strings.Contains(string(got), "name:") || !strings.Contains(string(got), "y") {
			t.Errorf("serialized = %q, want a name field with value y", got)
		}
	})

	t.Run("unknown layout errors", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{RecordFile: &ingitdb.RecordFileDef{
			Name: "data.x", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.RecordType("bogus"),
		}}
		if _, err := serializeMergedRecords(nil, col); err == nil {
			t.Fatal("expected error for unknown layout")
		}
	})

	t.Run("list csv", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{
			ColumnsOrder: []string{"$id", "v"},
			RecordFile: &ingitdb.RecordFileDef{
				Name: "data.csv", Format: ingitdb.RecordFormatCSV, RecordType: ingitdb.ListOfRecords,
			},
		}
		records := []recordmerge.Record{
			{Key: "a", Fields: map[string]any{"$id": "a", "v": "1"}},
			{Key: "b", Fields: map[string]any{"$id": "b", "v": "2"}},
		}
		got, err := serializeMergedRecords(records, col)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if !strings.Contains(string(got), "$id,v") || !strings.Contains(string(got), "a,1") {
			t.Errorf("csv output = %q", got)
		}
	})

	t.Run("list ingr", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{
			ColumnsOrder: []string{"v"},
			RecordFile: &ingitdb.RecordFileDef{
				Name: "rs", Format: ingitdb.RecordFormatINGR, RecordType: ingitdb.ListOfRecords,
			},
		}
		records := []recordmerge.Record{{Key: "a", Fields: map[string]any{"$ID": "a", "v": "1"}}}
		got, err := serializeMergedRecords(records, col)
		if err != nil {
			t.Fatalf("err: %v", err)
		}
		if len(got) == 0 {
			t.Error("expected INGR output")
		}
	})

	t.Run("list yaml/json/jsonl", func(t *testing.T) {
		t.Parallel()
		records := []recordmerge.Record{
			{Key: "a", Fields: map[string]any{"$id": "a", "v": "1"}},
			{Key: "b", Fields: map[string]any{"$id": "b", "v": "2"}},
		}
		for _, format := range []ingitdb.RecordFormat{
			ingitdb.RecordFormatYAML, ingitdb.RecordFormatJSON, ingitdb.RecordFormatJSONL,
		} {
			col := &ingitdb.CollectionDef{
				ColumnsOrder: []string{"$id", "v"},
				RecordFile:   &ingitdb.RecordFileDef{Name: "data", Format: format, RecordType: ingitdb.ListOfRecords},
			}
			out, err := serializeMergedRecords(records, col)
			if err != nil {
				t.Fatalf("%s: %v", format, err)
			}
			if !strings.Contains(string(out), "a") || !strings.Contains(string(out), "b") {
				t.Errorf("%s: output missing records: %q", format, out)
			}
		}
	})

	t.Run("unsupported list format errors", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{
			ColumnsOrder: []string{"v"},
			RecordFile: &ingitdb.RecordFileDef{
				Name: "data.toml", Format: ingitdb.RecordFormatTOML, RecordType: ingitdb.ListOfRecords,
			},
		}
		if _, err := serializeMergedRecords(nil, col); err == nil {
			t.Fatal("expected error for unsupported list format")
		}
	})
}

func TestResolveRecordMergeConflicts_YAMLListEndToEnd(t *testing.T) {
	t.Parallel()
	rel := filepath.Join("c", "data.yaml")
	dir := setupDataConflict(t, rel,
		"- $id: x\n  v: '0'\n",
		"- $id: x\n  v: '0'\n- $id: a\n  v: '1'\n",
		"- $id: x\n  v: '0'\n- $id: b\n  v: '2'\n",
	)
	colDir := filepath.Dir(filepath.Join(dir, rel))
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"c": {
				ID:           "c",
				DirPath:      colDir,
				ColumnsOrder: []string{"$id", "v"},
				RecordFile: &ingitdb.RecordFileDef{
					Name: "data.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.ListOfRecords,
				},
			},
		},
	}

	resolved, unresolved, err := resolveRecordMergeConflicts(context.Background(), dir, def, []string{rel})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(resolved) != 1 || len(unresolved) != 0 {
		t.Fatalf("resolved=%v unresolved=%v, want YAML list merged", resolved, unresolved)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, rel))
	if readErr != nil {
		t.Fatalf("read merged: %v", readErr)
	}
	rows, parseErr := dalgo2ingitdb.ParseListOfRecordsContent(content, ingitdb.RecordFormatYAML)
	if parseErr != nil {
		t.Fatalf("parse merged: %v\n%s", parseErr, content)
	}
	if len(rows) != 3 {
		t.Errorf("merged rows = %d, want 3 (x,a,b):\n%s", len(rows), content)
	}
}
