package ingitdb

import (
	"fmt"
	"regexp"
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

	// ContentField is the name of the column that maps to the Markdown body
	// in a `format: markdown` collection. When empty, the default
	// (`DefaultMarkdownContentField`, `$content`) is used.
	// It MUST only be set when Format is `markdown`.
	ContentField string `yaml:"content_field,omitempty"`

	// ExcludeRegex is an optional regular expression applied to the
	// basename of each candidate record file. Files whose basename matches
	// MUST be excluded from reads, validation, and record counts.
	//
	// Typical use: a record directory that legitimately contains a
	// README.md or .gitkeep alongside `{key}.md` records can set
	// `exclude_regex: '^README\.md$'` to keep those auxiliary files from
	// being treated as records.
	ExcludeRegex string `yaml:"exclude_regex,omitempty"`
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
	if rfd.Format == RecordFormatMarkdown {
		if rfd.RecordType != SingleRecord {
			return fmt.Errorf("format %q requires record type %q, got %q",
				RecordFormatMarkdown, SingleRecord, rfd.RecordType)
		}
	} else if rfd.ContentField != "" {
		return fmt.Errorf("content_field is only valid for format %q, got %q",
			RecordFormatMarkdown, rfd.Format)
	}
	if rfd.ExcludeRegex != "" {
		if _, err := regexp.Compile(rfd.ExcludeRegex); err != nil {
			return fmt.Errorf("invalid exclude_regex %q: %w", rfd.ExcludeRegex, err)
		}
	}
	return nil
}

// IsExcluded reports whether a filename's basename matches the configured
// `exclude_regex`. Files for which IsExcluded returns true MUST be omitted
// from reads, validation, and record counts.
//
// When ExcludeRegex is empty, IsExcluded always returns false. When the
// regex fails to compile (caller skipped Validate()), IsExcluded returns
// false rather than panicking — Validate() is the right place to surface
// compile errors.
func (rfd RecordFileDef) IsExcluded(filename string) bool {
	if rfd.ExcludeRegex == "" {
		return false
	}
	re, err := regexp.Compile(rfd.ExcludeRegex)
	if err != nil {
		return false
	}
	return re.MatchString(filename)
}

// ResolvedContentField returns the configured content_field name when set,
// otherwise the default (`$content`). Only meaningful when Format is
// `markdown`.
func (rfd RecordFileDef) ResolvedContentField() string {
	if rfd.ContentField != "" {
		return rfd.ContentField
	}
	return DefaultMarkdownContentField
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
