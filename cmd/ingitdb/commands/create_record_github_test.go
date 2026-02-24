package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"go.uber.org/mock/gomock"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestCreateRecord_GitHub_ParseError(t *testing.T) {

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := createRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--data={name: X}", "--github=invalid")
	if err == nil {
		t.Fatal("expected error for invalid GitHub spec")
	}
}

func TestCreateRecord_GitHub_ReadDefinitionError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := &fakeFileReaderWithError{err: errors.New("network error")}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	// Expect two calls - one for read definition, one potentially later
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(mockReader, nil).AnyTimes()

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := createRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--data={name: X}", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when reading remote definition fails")
	}
}

func TestCreateRecord_GitHub_DBOpenError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := &fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  test.items: test-ingitdb/items\n"),
		"test-ingitdb/items/.collection/definition.yaml": []byte("record_file:\n  name: items.json\n  type: map[string]map[string]any\n  format: json\ncolumns:\n  name:\n    type: string\n"),
	}}
	mockReaderFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockReaderFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(mockReader, nil).AnyTimes()

	mockDBFactory := NewMockGitHubDBFactory(ctrl)
	mockDBFactory.EXPECT().NewGitHubDBWithDef(gomock.Any(), gomock.Any()).Return(nil, errors.New("db open error")).AnyTimes()

	originalReaderFactory := gitHubFileReaderFactory
	originalDBFactory := gitHubDBFactory
	gitHubFileReaderFactory = mockReaderFactory
	gitHubDBFactory = mockDBFactory
	defer func() {
		gitHubFileReaderFactory = originalReaderFactory
		gitHubDBFactory = originalDBFactory
	}()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := createRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--data={name: X}", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}

func TestCreateRecord_InvalidDataYAML(t *testing.T) {

	dir := t.TempDir()
	def := testDef(dir)

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused - should not be called")
	}

	cmd := createRecord(homeDir, getWd, readDef, newDB, func(...any) {})
	err := runCLICommand(cmd, "--path="+dir, "--id=test.items/x", "--data=: invalid yaml :")
	if err == nil {
		t.Fatal("expected error for invalid YAML data")
	}
}

type fakeFileReaderWithError struct {
	err error
}

func (f *fakeFileReaderWithError) ReadFile(_ context.Context, _ string) ([]byte, bool, error) {
	return nil, false, f.err
}

func (f *fakeFileReaderWithError) ListDirectory(_ context.Context, _ string) ([]string, error) {
	return nil, f.err
}

var _ dalgo2ghingitdb.FileReader = (*fakeFileReaderWithError)(nil)
