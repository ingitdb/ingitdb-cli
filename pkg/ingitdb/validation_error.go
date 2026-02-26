package ingitdb

import (
	"fmt"
	"sync"
)

// Severity is the level of a validation finding.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// ValidationError is one finding from data validation.
// All location fields (CollectionID…FieldName) are optional.
// Implements error.
type ValidationError struct {
	Severity     Severity
	CollectionID string // e.g. "todo.tasks"
	FilePath     string // absolute path to offending file
	RecordKey    string // empty for list/map files where key is unknown
	FieldName    string // set for field-level errors
	Message      string
	Err          error // wrapped cause
}

// Error implements the error interface.
func (v ValidationError) Error() string {
	if v.Err != nil {
		return fmt.Sprintf("%s: %s: %v", v.Severity, v.Message, v.Err)
	}
	return fmt.Sprintf("%s: %s", v.Severity, v.Message)
}

// ValidationResult aggregates findings; mutex-protected for goroutine safety.
type ValidationResult struct {
	mu            sync.Mutex
	errors        []ValidationError
	recordCounts  map[string]int // collection -> record count
}

// Append adds a finding to the result.
func (r *ValidationResult) Append(e ValidationError) {
	r.mu.Lock()
	r.errors = append(r.errors, e)
	r.mu.Unlock()
}

// Errors returns a snapshot copy of all findings.
func (r *ValidationResult) Errors() []ValidationError {
	r.mu.Lock()
	out := make([]ValidationError, len(r.errors))
	copy(out, r.errors)
	r.mu.Unlock()
	return out
}

// HasErrors reports whether any findings have been recorded.
func (r *ValidationResult) HasErrors() bool {
	r.mu.Lock()
	has := len(r.errors) > 0
	r.mu.Unlock()
	return has
}

// ErrorCount returns the total number of recorded findings.
func (r *ValidationResult) ErrorCount() int {
	r.mu.Lock()
	n := len(r.errors)
	r.mu.Unlock()
	return n
}

// Filter returns findings that match the given predicate.
func (r *ValidationResult) Filter(fn func(ValidationError) bool) []ValidationError {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ValidationError
	for _, e := range r.errors {
		if fn(e) {
			out = append(out, e)
		}
	}
	return out
}

// SetRecordCount sets the number of records validated for a collection.
func (r *ValidationResult) SetRecordCount(collectionID string, count int) {
	r.mu.Lock()
	if r.recordCounts == nil {
		r.recordCounts = make(map[string]int)
	}
	r.recordCounts[collectionID] = count
	r.mu.Unlock()
}

// GetRecordCount returns the number of records validated for a collection.
func (r *ValidationResult) GetRecordCount(collectionID string) int {
	r.mu.Lock()
	count := r.recordCounts[collectionID]
	r.mu.Unlock()
	return count
}
