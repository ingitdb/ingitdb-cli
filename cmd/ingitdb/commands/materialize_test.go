package commands

import (
	"context"
	"fmt"
	"testing"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestMaterialize_Flags(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, nil, logf)

	for _, name := range []string{"collections", "views", "path", "records-delimiter"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("expected flag %q to be registered", name)
		}
	}

	for _, name := range []string{"collections", "views"} {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			continue
		}
		if f.NoOptDefVal != materializeAllSentinel {
			t.Errorf("flag %q: expected NoOptDefVal %q, got %q", name, materializeAllSentinel, f.NoOptDefVal)
		}
	}
}

func TestMaterialize_FlagTriState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		args        []string
		wantValue   string
		wantChanged bool
		wantPosArgs []string
	}{
		{
			name:        "equals list",
			args:        []string{"--views=a,b"},
			wantValue:   "a,b",
			wantChanged: true,
		},
		{
			name:        "bare flag is sentinel",
			args:        []string{"--views"},
			wantValue:   materializeAllSentinel,
			wantChanged: true,
		},
		{
			name:        "space form leaves positional",
			args:        []string{"--views", "a,b"},
			wantValue:   materializeAllSentinel,
			wantChanged: true,
			wantPosArgs: []string{"a,b"},
		},
		{
			name:        "absent",
			args:        nil,
			wantValue:   "",
			wantChanged: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{Use: "materialize", RunE: func(*cobra.Command, []string) error { return nil }}
			addMaterializeCommandFlags(cmd)
			cmd.SetArgs(tc.args)
			var gotPosArgs []string
			cmd.RunE = func(c *cobra.Command, posArgs []string) error {
				gotPosArgs = posArgs
				return nil
			}
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			got, _ := cmd.Flags().GetString("views")
			if got != tc.wantValue {
				t.Errorf("views value: got %q, want %q", got, tc.wantValue)
			}
			if changed := cmd.Flags().Changed("views"); changed != tc.wantChanged {
				t.Errorf("views changed: got %v, want %v", changed, tc.wantChanged)
			}
			if len(tc.wantPosArgs) > 0 {
				if fmt.Sprintf("%v", gotPosArgs) != fmt.Sprintf("%v", tc.wantPosArgs) {
					t.Errorf("positional args: got %v, want %v", gotPosArgs, tc.wantPosArgs)
				}
			}
		})
	}
}

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
	if cmd.Use != "materialize" {
		t.Errorf("expected name 'materialize', got %q", cmd.Name())
	}
	if cmd.RunE == nil {
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
	err := runCobraCommand(cmd)
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
	err := runCobraCommand(cmd, "--path="+dir)
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
	err := runCobraCommand(cmd, "--path="+dir)
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
	err := runCobraCommand(cmd, "--path="+dir)
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
	err := runCobraCommand(cmd)
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
	err := runCobraCommand(cmd, "--path=~")
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
	err := runCobraCommand(cmd, "--path="+dir, "--records-delimiter=1")
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
	if *d.RuntimeOverrides.RecordsDelimiter != 1 {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be 1 when --records-delimiter=1 flag is passed")
	}
}

func TestMaterialize_RecordsDelimiterPreservedFromViewDef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	defaultView := &ingitdb.ViewDef{Format: "ingr", RecordsDelimiter: 1}
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
	err := runCobraCommand(cmd, "--path="+dir)
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	d := viewBuilder.lastDefs[0]
	if d.RuntimeOverrides.RecordsDelimiter != nil {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be nil when flag is not passed")
	}
	col := viewBuilder.lastCols[0]
	if col.DefaultView.RecordsDelimiter != 1 {
		t.Error("expected ViewDef.RecordsDelimiter=1 to be preserved when flag is not passed")
	}
}

func TestMaterialize_RecordsDelimiterFlagOverridesViewDef(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	defaultView := &ingitdb.ViewDef{Format: "ingr", RecordsDelimiter: 1}
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
	err := runCobraCommand(cmd, "--path="+dir, "--records-delimiter=-1")
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	d := viewBuilder.lastDefs[0]
	if d.RuntimeOverrides.RecordsDelimiter == nil {
		t.Fatal("expected def.RuntimeOverrides.RecordsDelimiter to be set when flag is explicitly passed")
	}
	if *d.RuntimeOverrides.RecordsDelimiter != -1 {
		t.Error("expected def.RuntimeOverrides.RecordsDelimiter to be -1 when --records-delimiter=-1")
	}
}

func TestMaterialize_ViewErrorsReported(t *testing.T) {
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
			Errors: []error{fmt.Errorf("view failed")},
		},
	}
	logf := func(...any) {}

	cmd := Materialize(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCobraCommand(cmd, "--path="+dir)
	if err == nil {
		t.Fatal("expected non-nil error when MaterializeResult.Errors is non-empty")
	}
}
