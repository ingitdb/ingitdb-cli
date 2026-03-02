package materializer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestFileRecordsReader_ReadRecords_MapOfIDRecords_StatError(t *testing.T) {
	t.Parallel()

	statErr := errors.New("permission denied")
	reader := FileRecordsReader{
		statFile: func(path string) (os.FileInfo, error) {
			return nil, statErr
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: ingitdb.MapOfIDRecords,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for stat failure")
	}
	if !errors.Is(err, statErr) {
		t.Errorf("expected error to wrap stat error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_MapOfIDRecords_ReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	reader := FileRecordsReader{
		statFile: func(path string) (os.FileInfo, error) {
			return nil, nil
		},
		readFile: func(path string) ([]byte, error) {
			return nil, readErr
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: ingitdb.MapOfIDRecords,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for read failure")
	}
	if !errors.Is(err, readErr) {
		t.Errorf("expected error to wrap read error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_MapOfIDRecords_ParseError(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{
		statFile: func(path string) (os.FileInfo, error) {
			return nil, nil
		},
		readFile: func(path string) ([]byte, error) {
			return []byte("invalid json"), nil
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: ingitdb.MapOfIDRecords,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for parse failure")
	}
}

func TestFileRecordsReader_ReadRecords_MapOfIDRecords_YieldError(t *testing.T) {
	t.Parallel()

	yieldErr := errors.New("yield error")
	reader := FileRecordsReader{
		statFile: func(path string) (os.FileInfo, error) {
			return nil, nil
		},
		readFile: func(path string) ([]byte, error) {
			return []byte(`{"key1": {"title": "Test"}}`), nil
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: ingitdb.MapOfIDRecords,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return yieldErr
	})

	if err == nil {
		t.Fatal("expected error from yield function")
	}
	if !errors.Is(err, yieldErr) {
		t.Errorf("expected error to be yield error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_GlobError(t *testing.T) {
	t.Parallel()

	globErr := errors.New("glob failed")
	reader := FileRecordsReader{
		glob: func(pattern string) ([]string, error) {
			return nil, globErr
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for glob failure")
	}
	if !errors.Is(err, globErr) {
		t.Errorf("expected error to wrap glob error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_ReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	reader := FileRecordsReader{
		glob: func(pattern string) ([]string, error) {
			return []string{"/tmp/test/tag1.json"}, nil
		},
		readFile: func(path string) ([]byte, error) {
			return nil, readErr
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for read failure")
	}
	if !errors.Is(err, readErr) {
		t.Errorf("expected error to wrap read error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_ParseError(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{
		glob: func(pattern string) ([]string, error) {
			return []string{"/tmp/test/tag1.json"}, nil
		},
		readFile: func(path string) ([]byte, error) {
			return []byte("invalid json"), nil
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error for parse failure")
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_YieldError(t *testing.T) {
	t.Parallel()

	yieldErr := errors.New("yield error")
	reader := FileRecordsReader{
		glob: func(pattern string) ([]string, error) {
			return []string{"/tmp/test/tag1.json"}, nil
		},
		readFile: func(path string) ([]byte, error) {
			return []byte(`{"title": "Test"}`), nil
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "{key}.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.IRecordEntry) error {
		return yieldErr
	})

	if err == nil {
		t.Fatal("expected error from yield function")
	}
	if !errors.Is(err, yieldErr) {
		t.Errorf("expected error to be yield error, got: %v", err)
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_Success(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{
		glob: func(pattern string) ([]string, error) {
			return []string{
				"/tmp/test/prefix-tag1-suffix.json",
				"/tmp/test/prefix-tag2-suffix.json",
			}, nil
		},
		readFile: func(path string) ([]byte, error) {
			if filepath.Base(path) == "prefix-tag1-suffix.json" {
				return []byte(`{"title": "Tag 1"}`), nil
			}
			return []byte(`{"title": "Tag 2"}`), nil
		},
	}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "prefix-{key}-suffix.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	var entries []ingitdb.IRecordEntry
	err := reader.ReadRecords(context.Background(), "/tmp", col, func(entry ingitdb.IRecordEntry) error {
		entries = append(entries, entry)
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].GetID() != "tag1" {
		t.Errorf("expected key tag1, got %q", entries[0].GetID())
	}
	if entries[1].GetID() != "tag2" {
		t.Errorf("expected key tag2, got %q", entries[1].GetID())
	}
}
