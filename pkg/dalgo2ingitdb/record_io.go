package dalgo2ingitdb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// resolveRecordPath builds the on-disk path for a record by expanding the
// `{key}` placeholder in record_file.name and joining with the collection
// directory (plus the $records/ subdirectory when the name contains
// `{key}`). Mirrors dalgo2fsingitdb.resolveRecordPath.
func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	name := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	base := colDef.RecordFile.RecordsBasePath()
	return filepath.Join(colDef.DirPath, base, name)
}

// readSingleRecordFile reads one record file under a shared lock and
// returns the decoded record. Markdown and CSV formats are decoded via
// ParseRecordContentForCollection so column filtering and content_field
// handling apply. Returns (nil, false, nil) when the file does not exist.
func readSingleRecordFile(path string, colDef *ingitdb.CollectionDef) (map[string]any, bool, error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat %s: %w", path, statErr)
	}
	var (
		data    map[string]any
		readErr error
	)
	if err := withSharedLock(path, func() error {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		data, readErr = ParseRecordContentForCollection(content, colDef)
		if readErr != nil {
			return fmt.Errorf("parse %s: %w", path, readErr)
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	return data, true, nil
}

// writeSingleRecordFile encodes data using the collection's format and
// writes it to path under an exclusive lock. Intermediate directories are
// created as needed.
func writeSingleRecordFile(path string, colDef *ingitdb.CollectionDef, data map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	content, err := EncodeRecordContentForCollection(data, colDef)
	if err != nil {
		return fmt.Errorf("encode record for %s: %w", path, err)
	}
	return withExclusiveLock(path, func() error {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	})
}

// readMapOfRecordsFile reads a map-of-records file under a shared lock.
// Returns (nil, false, nil) when the file does not exist.
func readMapOfRecordsFile(path string, format ingitdb.RecordFormat) (map[string]map[string]any, bool, error) {
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("stat %s: %w", path, statErr)
	}
	var result map[string]map[string]any
	if err := withSharedLock(path, func() error {
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		result, err = ParseMapOfRecordsContent(content, format)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		return nil
	}); err != nil {
		return nil, false, err
	}
	return result, true, nil
}

// writeMapOfRecordsFile encodes a full map-of-records dataset and writes
// it to path under an exclusive lock.
func writeMapOfRecordsFile(path string, colDef *ingitdb.CollectionDef, data map[string]map[string]any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}
	content, err := EncodeMapOfRecordsContent(data, colDef.RecordFile.Format, colDef.ID, colDef.ColumnsOrder)
	if err != nil {
		return fmt.Errorf("encode records for %s: %w", path, err)
	}
	return withExclusiveLock(path, func() error {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
		return nil
	})
}

// deleteSingleRecordFile removes a record file under an exclusive lock.
// Returns dal.ErrRecordNotFound if the file does not exist.
//
// The pre-lock stat is required because gofrs/flock opens the lock target
// with O_CREATE; without this check we'd recreate the file just to delete
// it again and never report the miss.
func deleteSingleRecordFile(path string) error {
	if _, statErr := os.Stat(path); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return dal.ErrRecordNotFound
		}
		return fmt.Errorf("stat %s: %w", path, statErr)
	}
	return withExclusiveLock(path, func() error {
		err := os.Remove(path)
		if errors.Is(err, fs.ErrNotExist) {
			return dal.ErrRecordNotFound
		}
		if err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
		return nil
	})
}
