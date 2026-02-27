package materializer

import (
	"context"
	"os"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestNewFileRecordsReader(t *testing.T) {
	t.Parallel()

	reader := NewFileRecordsReader()
	if reader.readFile == nil {
		t.Error("readFile should not be nil")
	}
	if reader.statFile == nil {
		t.Error("statFile should not be nil")
	}
	if reader.glob == nil {
		t.Error("glob should not be nil")
	}
}

func TestRecordPatternForKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		fileName          string
		dirPath           string
		wantErr           bool
		wantPatternPath   string
		samplePaths       map[string]string // filePath -> expectedKey
	}{
		{
			name:            "simple pattern",
			fileName:        "{key}.json",
			dirPath:         "/data/tags",
			wantErr:         false,
			wantPatternPath: "/data/tags/*.json",
			samplePaths: map[string]string{
				"/data/tags/tag1.json": "tag1",
				"/data/tags/tag2.json": "tag2",
			},
		},
		{
			name:            "pattern with prefix",
			fileName:        "record-{key}.yaml",
			dirPath:         "/data/items",
			wantErr:         false,
			wantPatternPath: "/data/items/record-*.yaml",
			samplePaths: map[string]string{
				"/data/items/record-item1.yaml": "item1",
				"/data/items/record-item2.yaml": "item2",
			},
		},
		{
			name:            "pattern with suffix",
			fileName:        "{key}-data.json",
			dirPath:         "/data/users",
			wantErr:         false,
			wantPatternPath: "/data/users/*-data.json",
			samplePaths: map[string]string{
				"/data/users/user1-data.json": "user1",
				"/data/users/user2-data.json": "user2",
			},
		},
		{
			name:            "sub-directory pattern",
			fileName:        "{key}/{key}.yaml",
			dirPath:         "/data/countries",
			wantErr:         false,
			wantPatternPath: "/data/countries/*/*.yaml",
			samplePaths: map[string]string{
				"/data/countries/us/us.yaml":   "us",
				"/data/countries/uk/uk.yaml":   "uk",
				"/data/countries/fr/fr.yaml":   "fr",
			},
		},
		{
			name:     "no placeholder",
			fileName: "records.json",
			dirPath:  "/data/items",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern, extractKey, err := recordPatternForKey(tt.fileName, tt.dirPath)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if pattern != tt.wantPatternPath {
				t.Errorf("pattern = %q, want %q", pattern, tt.wantPatternPath)
			}
			for filePath, expectedKey := range tt.samplePaths {
				key := extractKey(filePath)
				if key != expectedKey {
					t.Errorf("extractKey(%q) = %q, want %q", filePath, key, expectedKey)
				}
			}
		})
	}
}

func TestFileRecordsReader_ReadRecords_NoRecordFile(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{}
	col := &ingitdb.CollectionDef{
		ID:         "test",
		RecordFile: nil,
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.RecordEntry) error {
		return nil
	})

	if err == nil {
		t.Error("expected error for collection without record file")
	}
}

func TestFileRecordsReader_ReadRecords_UnsupportedRecordType(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{}
	col := &ingitdb.CollectionDef{
		ID: "test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "records.json",
			RecordType: "unsupported",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.RecordEntry) error {
		return nil
	})

	if err == nil {
		t.Error("expected error for unsupported record type")
	}
}

func TestFileRecordsReader_ReadRecords_MapOfIDRecords_FileNotFound(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{
		statFile: func(path string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
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

	var called bool
	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.RecordEntry) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("should not error on non-existent file: %v", err)
	}
	if called {
		t.Error("yield should not be called for non-existent file")
	}
}

func TestFileRecordsReader_ReadRecords_SingleRecord_NoPlaceholder(t *testing.T) {
	t.Parallel()

	reader := FileRecordsReader{}
	col := &ingitdb.CollectionDef{
		ID:      "test",
		DirPath: "/tmp/test",
		RecordFile: &ingitdb.RecordFileDef{
			Name:       "record.json",
			RecordType: ingitdb.SingleRecord,
			Format:     "json",
		},
	}

	err := reader.ReadRecords(context.Background(), "/tmp", col, func(ingitdb.RecordEntry) error {
		return nil
	})

	if err == nil {
		t.Error("expected error for record file name without {key} placeholder")
	}
}
