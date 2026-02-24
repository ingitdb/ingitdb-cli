package commands

import (
	"errors"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"go.uber.org/mock/gomock"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestDeleteRecord_GitHub_ParseError(t *testing.T) {

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := deleteRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=invalid")
	if err == nil {
		t.Fatal("expected error for invalid GitHub spec")
	}
}

func TestDeleteRecord_GitHub_ReadDefinitionError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := &fakeFileReaderWithError{err: errors.New("network error")}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
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
	cmd := deleteRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when reading remote definition fails")
	}
}

func TestDeleteRecord_GitHub_DBOpenError(t *testing.T) {

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
	cmd := deleteRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}
