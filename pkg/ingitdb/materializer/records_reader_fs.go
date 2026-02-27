package materializer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// FileRecordsReader loads records from collection files on disk.
type FileRecordsReader struct {
	readFile func(string) ([]byte, error)
	statFile func(string) (os.FileInfo, error)
	glob     func(string) ([]string, error)
}

func NewFileRecordsReader() FileRecordsReader {
	return FileRecordsReader{
		readFile: os.ReadFile,
		statFile: os.Stat,
		glob:     filepath.Glob,
	}
}

func (r FileRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.RecordEntry) error,
) error {
	_ = ctx
	_ = dbPath
	if col.RecordFile == nil {
		return fmt.Errorf("collection %q has no record file definition", col.ID)
	}
	fileName := col.RecordFile.Name
	path := filepath.Join(col.DirPath, fileName)
	switch col.RecordFile.RecordType {
	case ingitdb.MapOfIDRecords:
		if _, err := r.statFile(path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("failed to stat %s: %w", path, err)
		}
		content, err := r.readFile(path)
		if err != nil {
			return fmt.Errorf("failed to read records file %s: %w", path, err)
		}
		records, err := dalgo2ingitdb.ParseMapOfIDRecordsContent(content, col.RecordFile.Format)
		if err != nil {
			return fmt.Errorf("failed to parse records file %s: %w", path, err)
		}
		for key, data := range records {
			d := dalgo2ingitdb.ApplyLocaleToRead(data, col.Columns)
			d["id"] = key
			entry := ingitdb.RecordEntry{
				Key:      key,
				FilePath: path,
				Data:     d,
			}
			if err := yield(entry); err != nil {
				return err
			}
		}
		return nil
	case ingitdb.SingleRecord:
		patternPath, extractKey, err := recordPatternForKey(fileName, col.DirPath)
		if err != nil {
			return err
		}
		matches, err := r.glob(patternPath)
		if err != nil {
			return fmt.Errorf("failed to glob records: %w", err)
		}
		for _, filePath := range matches {
			content, err := r.readFile(filePath)
			if err != nil {
				return fmt.Errorf("failed to read record %s: %w", filePath, err)
			}
			data, err := dalgo2ingitdb.ParseRecordContent(content, col.RecordFile.Format)
			if err != nil {
				return fmt.Errorf("failed to parse record %s: %w", filePath, err)
			}
			key := extractKey(filePath)
			if strings.HasPrefix(key, ".") {
				continue // skip hidden directories like .collection
			}
			d := dalgo2ingitdb.ApplyLocaleToRead(data, col.Columns)
			d["id"] = key
			entry := ingitdb.RecordEntry{
				Key:      key,
				FilePath: filePath,
				Data:     d,
			}
			if err := yield(entry); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("record type %q is not supported", col.RecordFile.RecordType)
	}
}

func recordPatternForKey(name, dirPath string) (patternPath string, extractKey func(string) string, err error) {
	const placeholder = "{key}"
	if !strings.Contains(name, placeholder) {
		return "", nil, fmt.Errorf("record file name %q must include {key}", name)
	}
	// Replace ALL {key} placeholders with * for globbing.
	globName := strings.ReplaceAll(name, placeholder, "*")
	patternPath = filepath.Join(dirPath, globName)

	// Build the key extractor based on the position of the first {key}.
	idx := strings.Index(name, placeholder)
	prefix := filepath.ToSlash(name[:idx])
	rest := name[idx+len(placeholder):]
	// Key segment ends at the first "/" in rest (or at the end if no slash).
	endIdx := strings.IndexByte(rest, '/')
	var keySuffix string
	if endIdx < 0 {
		keySuffix = rest
	} else {
		keySuffix = rest[:endIdx]
	}

	extractKey = func(filePath string) string {
		rel, relErr := filepath.Rel(dirPath, filePath)
		if relErr != nil {
			return filepath.Base(filePath)
		}
		rel = filepath.ToSlash(rel)
		s := strings.TrimPrefix(rel, prefix)
		if slashIdx := strings.IndexByte(s, '/'); slashIdx >= 0 {
			return strings.TrimSuffix(s[:slashIdx], keySuffix)
		}
		return strings.TrimSuffix(s, keySuffix)
	}
	return patternPath, extractKey, nil
}
