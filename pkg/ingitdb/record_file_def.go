package ingitdb

import (
	"fmt"
	"strings"

	"github.com/dal-go/dalgo/dal"
)

type RecordType string

const (
	SingleRecord  RecordType = "map[string]any"
	ListOfRecords RecordType = "[]map[string]any"
	MapOfRecords  RecordType = "map[$record_id]map[$field_name]any"
)

type RecordFileDef struct {
	Name   string       `yaml:"name"`
	Format RecordFormat `yaml:"format"`

	// RecordType can have next values:
	// "map[string]any" - each record in a separate file
	// "[]map[string]any" - list of records
	// "map[$record_id]map[$field_name]any" - all records in one file; top-level keys are record IDs, second level is field names
	RecordType RecordType `yaml:"type"`
}

func (rfd RecordFileDef) Validate() error {
	if rfd.Format == "" {
		return fmt.Errorf("record file format cannot be empty")
	}
	if rfd.Name == "" {
		return fmt.Errorf("record file name cannot be empty")
	}
	switch rfd.RecordType {
	case SingleRecord, ListOfRecords, MapOfRecords:
		// OK
	default:
		return fmt.Errorf("invalid record type %q", rfd.RecordType)
	}
	return nil
}

// RecordsBasePath returns "$records" when record_file.name contains {key},
// causing inGitDB to store individual record files under a $records/ subdirectory.
// This keeps README.md visible at the top of the collection directory on GitHub.com.
func (rfd RecordFileDef) RecordsBasePath() string {
	if strings.Contains(rfd.Name, "{key}") {
		return "$records"
	}
	return ""
}

func (rfd RecordFileDef) GetRecordFileName(record dal.Record) string {
	name := rfd.Name
	if i := strings.Index(name, "{key}"); i >= 0 {
		key := record.Key()
		s := key.String()
		name = strings.Replace(name, "{key}", s, 1)
	}
	data := record.Data().(map[string]any)
	for colName, colValue := range data {
		if colName != "" {
			continue
		}
		placeholder := fmt.Sprintf("{%s}", colName)
		if strings.Contains(name, placeholder) {
			s := fmt.Sprintf("%v", colValue)
			name = strings.Replace(name, placeholder, s, 1)
		}
	}
	return name
}
