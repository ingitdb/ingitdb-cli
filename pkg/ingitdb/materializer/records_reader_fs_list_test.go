package materializer

// specscore: feature/record-format/list-of-records

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func listCol() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		ID:      "notes",
		DirPath: "/tmp/notes",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: ingitdb.ListOfRecords,
			Format:     ingitdb.RecordFormatJSON,
		},
	}
}

func collectIDs(reader FileRecordsReader, col *ingitdb.CollectionDef) ([]string, error) {
	var ids []string
	err := reader.ReadRecords(context.Background(), "/tmp", col, func(e ingitdb.IRecordEntry) error {
		ids = append(ids, e.GetID())
		return nil
	})
	return ids, err
}

func TestReadRecords_List_SuccessInOrder(t *testing.T) {
	t.Parallel()
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, nil },
		readFile: func(string) ([]byte, error) {
			return []byte(`[{"$id":"a","v":1},{"$id":"b","v":2},{"$id":"c"}]`), nil
		},
	}
	ids, err := collectIDs(reader, listCol())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	want := []string{"a", "b", "c"}
	if len(ids) != 3 || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] {
		t.Fatalf("ids = %v, want %v (order preserved)", ids, want)
	}
}

func TestReadRecords_List_KeylessFails(t *testing.T) {
	t.Parallel()
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, nil },
		readFile: func(string) ([]byte, error) { return []byte(`[{"name":"Alex"}]`), nil },
	}
	_, err := collectIDs(reader, listCol())
	if err == nil || !strings.Contains(err.Error(), "no resolvable key") {
		t.Fatalf("expected keyless error, got %v", err)
	}
}

func TestReadRecords_List_NotExistYieldsNothing(t *testing.T) {
	t.Parallel()
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) {
			return nil, &os.PathError{Op: "stat", Path: "x", Err: os.ErrNotExist}
		},
	}
	ids, err := collectIDs(reader, listCol())
	if err != nil || len(ids) != 0 {
		t.Fatalf("ids=%v err=%v, want no records, no error", ids, err)
	}
}

func TestReadRecords_List_StatError(t *testing.T) {
	t.Parallel()
	statErr := errors.New("permission denied")
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, statErr },
	}
	_, err := collectIDs(reader, listCol())
	if !errors.Is(err, statErr) {
		t.Fatalf("expected stat error, got %v", err)
	}
}

func TestReadRecords_List_ReadError(t *testing.T) {
	t.Parallel()
	readErr := errors.New("read failed")
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, nil },
		readFile: func(string) ([]byte, error) { return nil, readErr },
	}
	_, err := collectIDs(reader, listCol())
	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestReadRecords_List_ParseError(t *testing.T) {
	t.Parallel()
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, nil },
		readFile: func(string) ([]byte, error) { return []byte("[not json"), nil },
	}
	_, err := collectIDs(reader, listCol())
	if err == nil || !strings.Contains(err.Error(), "failed to parse") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestReadRecords_List_YieldError(t *testing.T) {
	t.Parallel()
	yieldErr := errors.New("yield boom")
	reader := FileRecordsReader{
		statFile: func(string) (os.FileInfo, error) { return nil, nil },
		readFile: func(string) ([]byte, error) { return []byte(`[{"$id":"a"}]`), nil },
	}
	err := reader.ReadRecords(context.Background(), "/tmp", listCol(), func(ingitdb.IRecordEntry) error {
		return yieldErr
	})
	if !errors.Is(err, yieldErr) {
		t.Fatalf("expected yield error, got %v", err)
	}
}
