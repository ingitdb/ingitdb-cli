package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type mockViewBuilder struct {
	result   *ingitdb.MaterializeResult
	err      error
	lastCols []*ingitdb.CollectionDef
	lastDefs []*ingitdb.Definition
}

func (m *mockViewBuilder) BuildViews(_ context.Context, _ string, _ string, col *ingitdb.CollectionDef, def *ingitdb.Definition) (*ingitdb.MaterializeResult, error) {
	colCopy := *col
	m.lastCols = append(m.lastCols, &colCopy)
	defCopy := *def
	m.lastDefs = append(m.lastDefs, &defCopy)
	return m.result, m.err
}

func TestMaterialize_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, nil, logf)
	if cmd == nil {
		t.Fatal("Materialize() returned nil")
		return
	}
	if cmd.Name != "materialize" {
		t.Errorf("expected name 'materialize', got %q", cmd.Name)
	}
	if cmd.Action == nil {
		t.Fatal("expected Action to be set")
	}
}

func TestMaterialize_NotYetImplemented(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, nil, logf)
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error when viewBuilder is nil")
	}
}

func TestMaterialize_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		result: &ingitdb.MaterializeResult{
			FilesCreated:   1,
			FilesUpdated:   1,
			FilesUnchanged: 1,
		},
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
}

func TestMaterialize_BuildViewsError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		err: fmt.Errorf("build error"),
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when BuildViews fails")
	}
}

func TestMaterialize_ReadDefinitionError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read error")
	}
	viewBuilder := &mockViewBuilder{}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestMaterialize_GetWdError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	viewBuilder := &mockViewBuilder{}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd)
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

func TestMaterialize_ExpandHomeError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", fmt.Errorf("no home") }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	viewBuilder := &mockViewBuilder{}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path=~")
	if err == nil {
		t.Fatal("expected error when expandHome fails")
	}
}

func TestMaterialize_RecordsDelimiterPassedToBuilder(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	defaultView := &ingitdb.ViewDef{Format: "ingr"}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:          "test.items",
				DirPath:     dir,
				DefaultView: defaultView,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		result: &ingitdb.MaterializeResult{},
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir, "--records-delimiter")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	if len(viewBuilder.lastCols) == 0 {
		t.Fatal("expected BuildViews to be called at least once")
	}
	d := viewBuilder.lastDefs[0]
	if d.RuntimeOverrides.RecordsDelimiter == nil {
		t.Fatal("expected def.RuntimeOverrides.RecordsDelimiter to be set when --records-delimiter flag is passed")
	}
	if !*d.RuntimeOverrides.RecordsDelimiter {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be true when --records-delimiter flag is passed")
	}
}

func TestMaterialize_RecordsDelimiterPreservedFromViewDef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	defaultView := &ingitdb.ViewDef{Format: "ingr", RecordsDelimiter: true}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:          "test.items",
				DirPath:     dir,
				DefaultView: defaultView,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{result: &ingitdb.MaterializeResult{}}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	d := viewBuilder.lastDefs[0]
	if d.RuntimeOverrides.RecordsDelimiter != nil {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be nil when flag is not passed")
	}
	col := viewBuilder.lastCols[0]
	if !col.DefaultView.RecordsDelimiter {
		t.Error("expected ViewDef.RecordsDelimiter=true to be preserved when flag is not passed")
	}
}

func TestMaterialize_RecordsDelimiterFlagOverridesViewDef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	defaultView := &ingitdb.ViewDef{Format: "ingr", RecordsDelimiter: true}
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:          "test.items",
				DirPath:     dir,
				DefaultView: defaultView,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{result: &ingitdb.MaterializeResult{}}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCLICommand(cmd, "--path="+dir, "--records-delimiter=false")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	d := viewBuilder.lastDefs[0]
	if d.RuntimeOverrides.RecordsDelimiter == nil {
		t.Fatal("expected def.RuntimeOverrides.RecordsDelimiter to be set when flag is explicitly passed")
	}
	if *d.RuntimeOverrides.RecordsDelimiter {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be false when --records-delimiter=false")
	}
}
