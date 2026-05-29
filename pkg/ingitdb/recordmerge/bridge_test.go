package recordmerge

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func mapCol() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "data.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.MapOfRecords,
		},
	}
}

func singleCol() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.yaml",
			Format:     ingitdb.RecordFormatYAML,
			RecordType: ingitdb.SingleRecord,
		},
	}
}

func TestMergeFiles_MapOfRecords(t *testing.T) {
	t.Parallel()

	t.Run("disjoint additions unioned", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles(nil, []byte("a:\n  v: 1\n"), []byte("b:\n  v: 2\n"), mapCol(), Options{})
		if got.Escalate {
			t.Fatalf("unexpected escalate: %s", got.Reason)
		}
		if len(got.Merged) != 2 {
			t.Fatalf("merged = %v, want 2 records", got.Merged)
		}
	})

	t.Run("identical addition deduplicated", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles(nil, []byte("a:\n  v: 1\n"), []byte("a:\n  v: 1\n"), mapCol(), Options{})
		if got.Escalate || len(got.Merged) != 1 {
			t.Fatalf("got escalate=%v merged=%v, want single record", got.Escalate, got.Merged)
		}
	})

	t.Run("primary-key collision escalates", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles(nil, []byte("a:\n  v: 1\n"), []byte("a:\n  v: 2\n"), mapCol(), Options{})
		if !got.Escalate {
			t.Fatal("expected escalate on collision")
		}
	})

	t.Run("parse failure escalates", func(t *testing.T) {
		t.Parallel()
		// Top-level scalar value is not a record map -> ParseMapOfRecordsContent errors.
		got := MergeFiles(nil, []byte("a: 1\n"), nil, mapCol(), Options{})
		if !got.Escalate {
			t.Fatal("expected escalate on parse failure")
		}
	})

	t.Run("same-record different fields merged when enabled", func(t *testing.T) {
		t.Parallel()
		base := []byte("a:\n  name: x\n  email: e\n")
		ours := []byte("a:\n  name: y\n  email: e\n")
		their := []byte("a:\n  name: x\n  email: z\n")
		got := MergeFiles(base, ours, their, mapCol(), Options{SameRecord: true})
		if got.Escalate {
			t.Fatalf("unexpected escalate: %s", got.Reason)
		}
		fields, ok := find(got.Merged, "a")
		if !ok || fields["name"] != "y" || fields["email"] != "z" {
			t.Fatalf("merged record = %v, want name=y email=z", fields)
		}
	})

	t.Run("same-record escalates when disabled", func(t *testing.T) {
		t.Parallel()
		base := []byte("a:\n  name: x\n  email: e\n")
		ours := []byte("a:\n  name: y\n  email: e\n")
		their := []byte("a:\n  name: x\n  email: z\n")
		got := MergeFiles(base, ours, their, mapCol(), Options{SameRecord: false})
		if !got.Escalate {
			t.Fatal("expected escalate when same-record disabled")
		}
	})
}

func TestMergeFiles_SingleRecord(t *testing.T) {
	t.Parallel()

	t.Run("different fields merged when enabled", func(t *testing.T) {
		t.Parallel()
		base := []byte("name: x\nemail: e\n")
		ours := []byte("name: y\nemail: e\n")
		their := []byte("name: x\nemail: z\n")
		got := MergeFiles(base, ours, their, singleCol(), Options{SameRecord: true})
		if got.Escalate {
			t.Fatalf("unexpected escalate: %s", got.Reason)
		}
		fields, ok := find(got.Merged, "")
		if !ok || fields["name"] != "y" || fields["email"] != "z" {
			t.Fatalf("merged = %v, want name=y email=z", fields)
		}
	})

	t.Run("contested field escalates", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles([]byte("name: x\n"), []byte("name: y\n"), []byte("name: z\n"), singleCol(), Options{SameRecord: true})
		if !got.Escalate {
			t.Fatal("expected escalate on contested field")
		}
	})

	t.Run("both deleted yields no record and escalates", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles([]byte("name: x\n"), nil, nil, singleCol(), Options{})
		if !got.Escalate {
			t.Fatal("expected escalate when single-record merge yields no record")
		}
	})

	t.Run("parse failure escalates", func(t *testing.T) {
		t.Parallel()
		// A tab in YAML indentation is a parse error.
		got := MergeFiles(nil, []byte("a:\n\tb: c\n"), nil, singleCol(), Options{})
		if !got.Escalate {
			t.Fatal("expected escalate on single-record parse failure")
		}
	})
}

func TestMergeFiles_Unmergeable(t *testing.T) {
	t.Parallel()

	t.Run("nil record-file definition escalates", func(t *testing.T) {
		t.Parallel()
		got := MergeFiles(nil, nil, nil, &ingitdb.CollectionDef{}, Options{})
		if !got.Escalate {
			t.Fatal("expected escalate when record-file is nil")
		}
	})

	t.Run("unsupported layout escalates", func(t *testing.T) {
		t.Parallel()
		col := &ingitdb.CollectionDef{RecordFile: &ingitdb.RecordFileDef{
			Name: "data.csv", Format: ingitdb.RecordFormatCSV, RecordType: ingitdb.ListOfRecords,
		}}
		got := MergeFiles(nil, nil, nil, col, Options{})
		if !got.Escalate {
			t.Fatal("expected escalate for unsupported list layout")
		}
	})
}
