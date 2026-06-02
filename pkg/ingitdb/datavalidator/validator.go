package datavalidator

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

const listRecordsKey = "$records"

// NewValidator creates a data validator that parses records and checks basic schema constraints.
func NewValidator() DataValidator {
	return &simpleValidator{}
}

type simpleValidator struct{}

// Validate performs basic validation of records against their collection schemas.
// Returns a ValidationResult with any errors found.
func (sv *simpleValidator) Validate(_ context.Context, _ string, def *ingitdb.Definition) (*ingitdb.ValidationResult, error) {
	result := &ingitdb.ValidationResult{}

	for collectionKey, colDef := range def.Collections {
		passed, total, errors := validateCollectionRecords(collectionKey, colDef)
		for _, validationErr := range errors {
			result.Append(validationErr)
		}
		result.SetRecordCounts(collectionKey, passed, total)
		result.SetRecordCount(collectionKey, total)
	}

	return result, nil
}

func validateCollectionRecords(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	total, err := countRecords(colDef)
	if err != nil {
		total = 0
	}
	if shouldSkipRecordParsing(colDef) {
		return total, total, nil
	}
	switch colDef.RecordFile.RecordType {
	case ingitdb.SingleRecord:
		return validateSingleRecordFiles(collectionKey, colDef)
	case ingitdb.MapOfRecords:
		return validateMapOfRecordsFile(collectionKey, colDef)
	case ingitdb.ListOfRecords:
		return validateListOfRecordsFile(collectionKey, colDef)
	default:
		validationErr := newValidationError(collectionKey, "", "", "", "unsupported record type", nil)
		return 0, 0, []ingitdb.ValidationError{validationErr}
	}
}

func shouldSkipRecordParsing(colDef *ingitdb.CollectionDef) bool {
	if colDef == nil || colDef.RecordFile == nil {
		return true
	}
	if colDef.RecordFile.Format == "" || colDef.RecordFile.RecordType == "" {
		return true
	}
	return false
}

func validateSingleRecordFiles(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	pattern, err := singleRecordGlobPattern(colDef)
	if err != nil {
		validationErr := newValidationError(collectionKey, "", "", "", "invalid record file pattern", err)
		return 0, 0, []ingitdb.ValidationError{validationErr}
	}
	matches, err := filepath.Glob(pattern)
	if err != nil {
		validationErr := newValidationError(collectionKey, pattern, "", "", "failed to glob record files", err)
		return 0, 0, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	total := 0
	var errors []ingitdb.ValidationError
	for _, filePath := range matches {
		if skipRecordPath(filePath, colDef.RecordFile) {
			continue
		}
		info, statErr := os.Stat(filePath)
		if statErr != nil {
			total++
			validationErr := newValidationError(collectionKey, filePath, "", "", "failed to stat record file", statErr)
			errors = append(errors, validationErr)
			continue
		}
		if info.IsDir() {
			continue
		}
		total++
		content, readErr := os.ReadFile(filePath)
		if readErr != nil {
			validationErr := newValidationError(collectionKey, filePath, "", "", "failed to read record file", readErr)
			errors = append(errors, validationErr)
			continue
		}
		data, parseErr := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
		if parseErr != nil {
			validationErr := newValidationError(collectionKey, filePath, "", "", "failed to parse record file", parseErr)
			errors = append(errors, validationErr)
			continue
		}
		recordKey := recordKeyFromFilePath(filePath)
		recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, data)
		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue
		}
		passed++
	}
	return passed, total, errors
}

func singleRecordGlobPattern(colDef *ingitdb.CollectionDef) (string, error) {
	baseDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	fileName := colDef.RecordFile.Name
	if strings.Contains(fileName, "{key}") {
		globName := strings.ReplaceAll(fileName, "{key}", "*")
		return filepath.Join(baseDir, globName), nil
	}
	return filepath.Join(baseDir, fileName), nil
}

func skipRecordPath(filePath string, rfd *ingitdb.RecordFileDef) bool {
	name := filepath.Base(filePath)
	if strings.HasPrefix(name, ".") {
		return true
	}
	return rfd.IsExcluded(name)
}

func recordKeyFromFilePath(filePath string) string {
	name := filepath.Base(filePath)
	ext := filepath.Ext(name)
	return strings.TrimSuffix(name, ext)
}

func validateMapOfRecordsFile(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, validationErr := readRecordsFile(collectionKey, filePath)
	if !ok {
		if validationErr.Message == "" {
			return 0, 0, nil
		}
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	records, err := dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
	if err != nil {
		validationErr = newValidationError(collectionKey, filePath, "", "", "failed to parse records file", err)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	var errors []ingitdb.ValidationError
	for recordKey, data := range records {
		recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, data)
		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue
		}
		passed++
	}
	total := len(records)
	return passed, total, errors
}

func validateListOfRecordsFile(collectionKey string, colDef *ingitdb.CollectionDef) (int, int, []ingitdb.ValidationError) {
	filePath := collectionRecordFilePath(colDef)
	content, ok, validationErr := readRecordsFile(collectionKey, filePath)
	if !ok {
		if validationErr.Message == "" {
			return 0, 0, nil
		}
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	rows, err := parseListRows(content, colDef)
	if err != nil {
		validationErr = newValidationError(collectionKey, filePath, "", "", "failed to parse records file", err)
		return 0, 1, []ingitdb.ValidationError{validationErr}
	}
	passed := 0
	var errors []ingitdb.ValidationError
	for _, row := range rows {
		recordKey, keyOK := dalgo2ingitdb.ResolveListRecordKey(row, colDef)
		if !keyOK {
			validationErr = newValidationError(collectionKey, filePath, "", "", "list record has no resolvable key", nil)
			errors = append(errors, validationErr)
			continue
		}
		recordErrors := validateRecordData(collectionKey, filePath, recordKey, colDef, row)
		if len(recordErrors) > 0 {
			errors = append(errors, recordErrors...)
			continue
		}
		passed++
	}
	total := len(rows)
	return passed, total, errors
}

func collectionRecordFilePath(colDef *ingitdb.CollectionDef) string {
	baseDir := filepath.Join(colDef.DirPath, colDef.RecordFile.RecordsBasePath())
	return filepath.Join(baseDir, colDef.RecordFile.Name)
}

func readRecordsFile(collectionKey, filePath string) ([]byte, bool, ingitdb.ValidationError) {
	content, err := os.ReadFile(filePath)
	if err == nil {
		return content, true, ingitdb.ValidationError{}
	}
	if os.IsNotExist(err) {
		return nil, false, ingitdb.ValidationError{}
	}
	validationErr := newValidationError(collectionKey, filePath, "", "", "failed to read records file", err)
	return nil, false, validationErr
}

func parseListRows(content []byte, colDef *ingitdb.CollectionDef) ([]map[string]any, error) {
	switch colDef.RecordFile.Format {
	case ingitdb.RecordFormatCSV:
		data, err := dalgo2ingitdb.ParseRecordContentForCollection(content, colDef)
		if err != nil {
			return nil, err
		}
		rawRows, ok := data[listRecordsKey]
		if !ok {
			return nil, fmt.Errorf("csv parser did not return %q rows", listRecordsKey)
		}
		rows, ok := rawRows.([]map[string]any)
		if !ok {
			return nil, fmt.Errorf("csv parser returned %q as %T", listRecordsKey, rawRows)
		}
		return rows, nil
	case ingitdb.RecordFormatINGR:
		records, err := dalgo2ingitdb.ParseMapOfRecordsContent(content, ingitdb.RecordFormatINGR)
		if err != nil {
			return nil, err
		}
		rows := make([]map[string]any, 0, len(records))
		for key, record := range records {
			record["$ID"] = key
			rows = append(rows, record)
		}
		return rows, nil
	default:
		return dalgo2ingitdb.ParseListOfRecordsContent(content, colDef.RecordFile.Format)
	}
}

func validateRecordData(
	collectionKey string,
	filePath string,
	recordKey string,
	colDef *ingitdb.CollectionDef,
	data map[string]any,
) []ingitdb.ValidationError {
	var errors []ingitdb.ValidationError
	for fieldName, columnDef := range colDef.Columns {
		if columnDef.Formula != "" {
			if _, present := data[fieldName]; present {
				message := fmt.Sprintf("computed column %q must not be stored", fieldName)
				validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil)
				errors = append(errors, validationErr)
			}
			continue
		}
		value, ok := data[fieldName]
		if !ok || value == nil {
			if columnDef.Required {
				validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, "missing required field", nil)
				errors = append(errors, validationErr)
			}
			continue
		}
		if valueMatchesColumnType(value, columnDef.Type) {
			continue
		}
		message := fmt.Sprintf("wrong type for field %q: expected %s, got %T", fieldName, columnDef.Type, value)
		validationErr := newValidationError(collectionKey, filePath, recordKey, fieldName, message, nil)
		errors = append(errors, validationErr)
	}
	return errors
}

func valueMatchesColumnType(value any, columnType ingitdb.ColumnType) bool {
	switch columnType {
	case ingitdb.ColumnTypeAny:
		return true
	case ingitdb.ColumnTypeString:
		_, ok := value.(string)
		return ok
	case ingitdb.ColumnTypeInt:
		return isIntegerValue(value)
	case ingitdb.ColumnTypeFloat:
		return isNumberValue(value)
	case ingitdb.ColumnTypeBool:
		_, ok := value.(bool)
		return ok
	case ingitdb.ColumnTypeDate, ingitdb.ColumnTypeTime, ingitdb.ColumnTypeDateTime:
		return isTemporalValue(value)
	case ingitdb.ColumnTypeL10N:
		return isStringMap(value)
	default:
		if strings.HasPrefix(string(columnType), "map[") {
			return isMapValue(value)
		}
		return true
	}
}

func isIntegerValue(value any) bool {
	switch v := value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32:
		floatValue := float64(v)
		return math.Trunc(floatValue) == floatValue
	case float64:
		return math.Trunc(v) == v
	default:
		return false
	}
}

func isNumberValue(value any) bool {
	switch value.(type) {
	case int, int8, int16, int32, int64:
		return true
	case uint, uint8, uint16, uint32, uint64:
		return true
	case float32, float64:
		return true
	default:
		return false
	}
}

func isTemporalValue(value any) bool {
	switch value.(type) {
	case string, time.Time:
		return true
	default:
		return false
	}
}

func isStringMap(value any) bool {
	switch typed := value.(type) {
	case map[string]string:
		return true
	case map[string]any:
		for _, item := range typed {
			if _, ok := item.(string); !ok {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func isMapValue(value any) bool {
	switch value.(type) {
	case map[string]any, map[string]string:
		return true
	default:
		return false
	}
}

func newValidationError(collectionKey, filePath, recordKey, fieldName, message string, err error) ingitdb.ValidationError {
	return ingitdb.ValidationError{
		Severity:     ingitdb.SeverityError,
		CollectionID: collectionKey,
		FilePath:     filePath,
		RecordKey:    recordKey,
		FieldName:    fieldName,
		Message:      message,
		Err:          err,
	}
}

// countRecords counts the number of record keys in a collection directory.
// When a $records/ subdirectory exists (used for per-key record files), it
// counts entries inside that directory instead of at the collection root.
func countRecords(colDef *ingitdb.CollectionDef) (int, error) {
	collectionPath := colDef.DirPath
	exts := expectedRecordExtensions(colDef)
	recordsSubDir := filepath.Join(collectionPath, "$records")
	if info, err := os.Stat(recordsSubDir); err == nil && info.IsDir() {
		return countEntries(recordsSubDir, exts, colDef.RecordFile)
	}
	return countEntries(collectionPath, exts, colDef.RecordFile)
}

// expectedRecordExtensions returns the file extensions that count as record
// files for this collection.
//
// The authoritative source is the collection's `record_file.name` template
// (e.g. `{key}.md`) — whatever extension it ends with is the single
// extension that records use. This naturally extends to any future format
// without needing changes here.
//
// When no `RecordFile` is declared (older test fixtures), the legacy
// permissive set (`.yaml`, `.yml`, `.json`) is returned so existing
// behavior is preserved.
func expectedRecordExtensions(colDef *ingitdb.CollectionDef) map[string]struct{} {
	if colDef.RecordFile != nil && colDef.RecordFile.Name != "" {
		ext := strings.ToLower(filepath.Ext(colDef.RecordFile.Name))
		if ext != "" {
			return map[string]struct{}{ext: {}}
		}
	}
	return map[string]struct{}{".yaml": {}, ".yml": {}, ".json": {}}
}

func countEntries(dirPath string, exts map[string]struct{}, rfd *ingitdb.RecordFileDef) (int, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return 0, err
	}
	// Count unique record keys. A key may appear as a plain file (e.g. USD.yaml)
	// or as a subdirectory (e.g. ord001/ holding subcollection data), or both.
	// We deduplicate by stripping the file extension so that a record with both
	// an ord001.yaml and an ord001/ directory is counted only once.
	seen := make(map[string]struct{})
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") || name == "$records" {
			continue
		}
		if rfd != nil && rfd.IsExcluded(name) {
			continue
		}
		if entry.IsDir() {
			seen[name] = struct{}{}
			continue
		}
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := exts[ext]; ok {
			seen[strings.TrimSuffix(name, filepath.Ext(name))] = struct{}{}
		}
	}
	return len(seen), nil
}
