package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type mockDataValidator struct {
	result *ingitdb.ValidationResult
	err    error
	called bool
}

func (m *mockDataValidator) Validate(_ context.Context, _ string, _ *ingitdb.Definition) (*ingitdb.ValidationResult, error) {
	m.called = true
	return m.result, m.err
}

type mockIncrementalValidator struct {
	result *ingitdb.ValidationResult
	err    error
}

func (m *mockIncrementalValidator) ValidateChanges(_ context.Context, _ string, _ *ingitdb.Definition, _, _ string) (*ingitdb.ValidationResult, error) {
	return m.result, m.err
}

func TestValidate_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	if cmd == nil {
		t.Fatal("Validate() returned nil")
	}
	if cmd.Name != "validate" {
		t.Errorf("expected name 'validate', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestValidate_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	dataVal := &mockDataValidator{
		result: &ingitdb.ValidationResult{},
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_NoDataValidator(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Validate with no dataValidator: %v", err)
	}
}

func TestValidate_DataValidationErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	dataVal := &mockDataValidator{
		result: func() *ingitdb.ValidationResult {
			r := &ingitdb.ValidationResult{}
			r.Append(ingitdb.ValidationError{Message: "error 1"})
			r.Append(ingitdb.ValidationError{Message: "error 2"})
			r.Append(ingitdb.ValidationError{Message: "error 3"})
			return r
		}(),
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when data validation has errors")
	}
}

func TestValidate_DataValidationError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	dataVal := &mockDataValidator{
		err: fmt.Errorf("validation error"),
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when data validation fails")
	}
}

func TestValidate_IncrementalNotImplemented(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--from-commit=abc123")
	if err == nil {
		t.Fatal("expected error when incremental validator is nil")
	}
}

func TestValidate_IncrementalSuccess(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	incVal := &mockIncrementalValidator{
		result: &ingitdb.ValidationResult{},
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, incVal, logf)
	err := runCLICommand(cmd, "--path="+dir, "--from-commit=abc123", "--to-commit=def456")
	if err != nil {
		t.Fatalf("Validate incremental: %v", err)
	}
}

func TestValidate_IncrementalErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	incVal := &mockIncrementalValidator{
		result: func() *ingitdb.ValidationResult {
			r := &ingitdb.ValidationResult{}
			r.Append(ingitdb.ValidationError{Message: "error 1"})
			r.Append(ingitdb.ValidationError{Message: "error 2"})
			return r
		}(),
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, incVal, logf)
	err := runCLICommand(cmd, "--path="+dir, "--from-commit=abc123")
	if err == nil {
		t.Fatal("expected error when incremental validation has errors")
	}
}

func TestValidate_IncrementalValidationError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	incVal := &mockIncrementalValidator{
		err: fmt.Errorf("validation error"),
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, incVal, logf)
	err := runCLICommand(cmd, "--path="+dir, "--from-commit=abc123")
	if err == nil {
		t.Fatal("expected error when incremental validation fails")
	}
}

func TestValidate_IncrementalReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read error")
	}
	incVal := &mockIncrementalValidator{}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, incVal, logf)
	err := runCLICommand(cmd, "--path="+dir, "--from-commit=abc123")
	if err == nil {
		t.Fatal("expected error when readDefinition fails for incremental")
	}
}

func TestValidate_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read error")
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestValidate_GetWdError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

func TestValidate_ExpandHomeError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", fmt.Errorf("no home") }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd, "--path=~")
	if err == nil {
		t.Fatal("expected error when expandHome fails")
	}
}

func TestExpandHome_NoTilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("/tmp/db", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/db" {
		t.Fatalf("expected /tmp/db, got %s", got)
	}
}

func TestExpandHome_Tilde(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("~", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/home" {
		t.Fatalf("expected /tmp/home, got %s", got)
	}
}

func TestExpandHome_TildeWithPath(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }

	got, err := expandHome("~/db", homeDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/tmp/home/db" {
		t.Fatalf("expected /tmp/home/db, got %s", got)
	}
}

func TestExpandHome_Error(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", errors.New("no home") }

	got, err := expandHome("~", homeDir)
	if err == nil {
		t.Fatal("expected error")
	}
	if got != "" {
		t.Fatalf("expected empty result, got %s", got)
	}
}

func TestValidate_OnlyInvalid(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, nil, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--only=invalid")
	if err == nil {
		t.Fatal("expected error for invalid --only value")
	}
	if !strings.Contains(err.Error(), "invalid --only value") {
		t.Errorf("expected 'invalid --only value' in error, got: %v", err)
	}
}

func TestValidate_OnlyDefinition(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	dataVal := &mockDataValidator{
		result: &ingitdb.ValidationResult{},
	}

	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--only=definition")
	if err != nil {
		t.Fatalf("Validate with --only=definition: %v", err)
	}
	if dataVal.called {
		t.Fatal("expected DataValidator.Validate not to be called with --only=definition")
	}
}

func TestValidate_OnlyRecords(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	readDefCalls := 0
	readDef := func(_ string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readDefCalls++
		// Check if validation option was passed - should not be when --only=records
		if len(opts) > 0 {
			t.Errorf("expected no validation option passed, got %d options", len(opts))
		}
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"users": {},
			},
		}, nil
	}
	dataVal := &mockDataValidator{
		result: &ingitdb.ValidationResult{},
	}

	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--only=records")
	if err != nil {
		t.Fatalf("Validate with --only=records: %v", err)
	}
	if readDefCalls != 1 {
		t.Errorf("expected readDefinition to be called once, got %d calls", readDefCalls)
	}
}

func TestValidate_CompletionMessage(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"users":     {},
				"products":  {},
				"customers": {},
			},
		}, nil
	}
	dataVal := &mockDataValidator{
		result: func() *ingitdb.ValidationResult {
			r := &ingitdb.ValidationResult{}
			r.SetRecordCount("users", 5)
			r.SetRecordCount("products", 10)
			r.SetRecordCount("customers", 3)
			return r
		}(),
	}

	logMessages := []string{}
	logf := func(args ...any) {
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				logMessages = append(logMessages, str)
			}
		}
	}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}

	// Check that completion messages appear for all collections with counts
	expectedMessages := map[string]bool{
		"5 records are valid for collection: users":     false,
		"10 records are valid for collection: products":  false,
		"3 records are valid for collection: customers": false,
	}

	for _, msg := range logMessages {
		if _, exists := expectedMessages[msg]; exists {
			expectedMessages[msg] = true
		}
	}

	for msg, found := range expectedMessages {
		if !found {
			t.Errorf("expected log message: %q", msg)
		}
	}
}

func TestValidate_OnlyRecordsSkipsDefinitionValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	// Track if validation option was passed
	validateOptionPassed := false
	readDef := func(_ string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		// When --only=records, no validation option should be passed
		if len(opts) > 0 {
			validateOptionPassed = true
		}
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"test": {},
			},
		}, nil
	}
	dataVal := &mockDataValidator{
		result: &ingitdb.ValidationResult{},
	}
	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--only=records")
	if err != nil {
		t.Fatalf("Validate with --only=records: %v", err)
	}
	if validateOptionPassed {
		t.Fatal("expected validation option not to be passed with --only=records")
	}
}

func TestValidate_OnlyDefinitionSkipsRecordValidation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"test": {},
			},
		}, nil
	}

	dataVal := &mockDataValidator{
		result: &ingitdb.ValidationResult{},
	}

	logf := func(...any) {}

	cmd := Validate(homeDir, getWd, readDef, dataVal, nil, logf)
	err := runCLICommand(cmd, "--path="+dir, "--only=definition")
	if err != nil {
		t.Fatalf("Validate with --only=definition: %v", err)
	}
	if dataVal.called {
		t.Fatal("expected DataValidator.Validate not to be called with --only=definition")
	}
}
