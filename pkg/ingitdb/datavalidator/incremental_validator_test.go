package datavalidator

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitdiff"
)

type fakeDiffer struct {
	files []ingitdb.ChangedFile
	err   error
}

func (f fakeDiffer) DiffFiles(_ context.Context, _, _, _ string) ([]ingitdb.ChangedFile, error) {
	return f.files, f.err
}

type fakeFullValidator struct {
	called bool
	result *ingitdb.ValidationResult
}

func (f *fakeFullValidator) Validate(_ context.Context, _ string, _ *ingitdb.Definition) (*ingitdb.ValidationResult, error) {
	f.called = true
	return f.result, nil
}

func TestChangedDefinitionFile(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"root config", ".ingitdb.yaml", true},
		{"collection definition", "people/.collection/definition.yaml", true},
		{"root-collections", ".ingitdb/root-collections.yaml", true},
		{"plain record", "people/alice.yaml", false},
		{"readme", "people/README.md", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := changedDefinitionFile([]ingitdb.ChangedFile{{Kind: ingitdb.ChangeKindModified, Path: tc.path}})
			if got != tc.want {
				t.Errorf("changedDefinitionFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestIncrementalValidator_FallsBackOnDefinitionChange(t *testing.T) {
	t.Parallel()

	sentinel := &ingitdb.ValidationResult{}
	full := &fakeFullValidator{result: sentinel}
	differ := fakeDiffer{files: []ingitdb.ChangedFile{{Kind: ingitdb.ChangeKindModified, Path: ".ingitdb.yaml"}}}
	iv := NewIncrementalValidator(differ, NewChangeSetResolver(), full)

	got, err := iv.ValidateChanges(context.Background(), "/db", &ingitdb.Definition{}, "A", "B")
	if err != nil {
		t.Fatalf("ValidateChanges: %v", err)
	}
	if !full.called {
		t.Error("expected fall-back to full validation when a definition file changed")
	}
	if got != sentinel {
		t.Error("expected the full validator's result to be returned")
	}
}

// TestIncrementalValidator_ScopedToChangedRecords is the AC:scoped-validation
// end-to-end: only records whose files changed in the range are validated;
// records outside the range are not opened.
func TestIncrementalValidator_ScopedToChangedRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	git("init")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "T")
	// Single-record collection with a templated {key} name stores records under
	// "$records". Base: an already-invalid record (missing required name) that we
	// will NOT touch, plus a valid record.
	write("people/$records/keep_bad.yaml", "{}\n")
	write("people/$records/alice.yaml", "name: Alice\n")
	git("add", ".")
	git("commit", "-m", "base")
	base := strings.TrimSpace(gitOut(t, dir, "rev-parse", "HEAD"))

	// Second commit: modify alice (still valid) and add an invalid charlie.
	write("people/$records/alice.yaml", "name: Alice2\n")
	write("people/$records/charlie.yaml", "{}\n")
	git("add", ".")
	git("commit", "-m", "second")

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: filepath.Join(dir, "people"),
				RecordFile: &ingitdb.RecordFileDef{
					Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString, Required: true}},
			},
		},
	}

	iv := NewIncrementalValidator(gitdiff.NewGitDiffer(), NewChangeSetResolver(), NewValidator())
	result, err := iv.ValidateChanges(context.Background(), dir, def, base, "HEAD")
	if err != nil {
		t.Fatalf("ValidateChanges: %v", err)
	}
	if !result.HasErrors() {
		t.Fatal("expected the changed invalid record (charlie) to fail validation")
	}
	joined := strings.Join(errorStrings(result), " | ")
	if !strings.Contains(joined, "charlie") {
		t.Errorf("expected error to reference changed record charlie, got: %s", joined)
	}
	if strings.Contains(joined, "keep_bad") {
		t.Errorf("unchanged invalid record keep_bad must not be opened/validated, got: %s", joined)
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func errorStrings(result *ingitdb.ValidationResult) []string {
	var out []string
	for _, e := range result.Errors() {
		out = append(out, e.FilePath+":"+e.Error())
	}
	return out
}
