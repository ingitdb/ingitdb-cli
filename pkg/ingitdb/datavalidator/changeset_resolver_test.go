package datavalidator

import (
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func resolverTestDef(dbPath string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: filepath.Join(dbPath, "people"),
				RecordFile: &ingitdb.RecordFileDef{
					Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
			},
			"tags": {
				ID:      "tags",
				DirPath: filepath.Join(dbPath, "tags"),
				RecordFile: &ingitdb.RecordFileDef{
					Name: "tags.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.MapOfRecords,
				},
				Columns: map[string]*ingitdb.ColumnDef{"label": {Type: ingitdb.ColumnTypeString}},
			},
		},
	}
}

func TestChangeSetResolver_Resolve(t *testing.T) {
	t.Parallel()

	dbPath := "/db"
	def := resolverTestDef(dbPath)
	// Single-record collections with a templated {key} name store records under
	// a "$records" subdirectory (see RecordFileDef.RecordsBasePath).
	changed := []ingitdb.ChangedFile{
		{Kind: ingitdb.ChangeKindModified, Path: "people/$records/alice.yaml"},
		{Kind: ingitdb.ChangeKindAdded, Path: "people/$records/bob.yaml"},
		{Kind: ingitdb.ChangeKindModified, Path: "tags/tags.yaml"},
		{Kind: ingitdb.ChangeKindDeleted, Path: "people/$records/carol.yaml"}, // skipped: deletion
		{Kind: ingitdb.ChangeKindModified, Path: "README.md"},                 // skipped: not a record file
	}

	got, err := NewChangeSetResolver().Resolve(dbPath, def, changed)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	// Index by file path for assertions.
	byPath := map[string]AffectedRecord{}
	for _, ar := range got {
		byPath[ar.FilePath] = ar
	}
	if len(byPath) != 3 {
		t.Fatalf("expected 3 affected records, got %d: %+v", len(byPath), got)
	}

	alice := byPath[filepath.Join(dbPath, "people/$records/alice.yaml")]
	if alice.CollectionID != "people" || alice.RecordKey != "alice" {
		t.Errorf("alice affected = %+v, want collection people key alice", alice)
	}
	tags := byPath[filepath.Join(dbPath, "tags/tags.yaml")]
	if tags.CollectionID != "tags" || tags.RecordKey != "" {
		t.Errorf("tags affected = %+v, want collection tags whole-file key \"\"", tags)
	}
	if _, ok := byPath[filepath.Join(dbPath, "people/$records/carol.yaml")]; ok {
		t.Error("deleted file must not be reported as affected")
	}
	if _, ok := byPath[filepath.Join(dbPath, "README.md")]; ok {
		t.Error("non-record file must not be reported as affected")
	}
}
