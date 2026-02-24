package commands

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/urfave/cli/v3"
)

type fakeFileReader struct {
	files       map[string][]byte
	directories map[string][]string
}

func (f fakeFileReader) ReadFile(_ context.Context, filePath string) ([]byte, bool, error) {
	content, ok := f.files[filePath]
	if !ok {
		return nil, false, nil
	}
	return content, true, nil
}

func (f fakeFileReader) ListDirectory(_ context.Context, dirPath string) ([]string, error) {
	entries, ok := f.directories[dirPath]
	if !ok {
		return []string{}, nil
	}
	return entries, nil
}

func TestParseGitHubRepoSpec(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantRef   string
		wantErr   bool
	}{
		{name: "owner repo only", input: "ingitdb/ingitdb-cli", wantOwner: "ingitdb", wantRepo: "ingitdb-cli"},
		{name: "branch", input: "ingitdb/ingitdb-cli@main", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "main"},
		{name: "tag", input: "ingitdb/ingitdb-cli@v1.2.3", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "v1.2.3"},
		{name: "commit", input: "ingitdb/ingitdb-cli@a1b2c3d", wantOwner: "ingitdb", wantRepo: "ingitdb-cli", wantRef: "a1b2c3d"},
		{name: "invalid", input: "ingitdb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, err := parseGitHubRepoSpec(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseGitHubRepoSpec: %v", err)
			}
			if spec.Owner != tt.wantOwner || spec.Repo != tt.wantRepo || spec.Ref != tt.wantRef {
				t.Fatalf("unexpected spec: %+v", spec)
			}
		})
	}
}

func TestResolveRemoteCollectionPath(t *testing.T) {
	rootCollections := map[string]string{
		"countries": "test-ingitdb/countries",
		"todo.tags": "docs/demo-apps/todo/tags",
	}
	collectionID, recordKey, collectionPath, err := resolveRemoteCollectionPath(rootCollections, "todo.tags/active")
	if err != nil {
		t.Fatalf("resolveRemoteCollectionPath: %v", err)
	}
	if collectionID != "todo.tags" {
		t.Fatalf("expected collectionID todo.tags, got %s", collectionID)
	}
	if recordKey != "active" {
		t.Fatalf("expected recordKey active, got %s", recordKey)
	}
	if collectionPath != "docs/demo-apps/todo/tags" {
		t.Fatalf("expected collectionPath docs/demo-apps/todo/tags, got %s", collectionPath)
	}
}

func TestReadRemoteDefinitionForIDWithReader(t *testing.T) {
	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: docs/demo-apps/todo/tags\n"),
		"docs/demo-apps/todo/tags/.collection/todo.tags.yaml": []byte("record_file:\n  name: tags.json\n  type: map[string]map[string]any\n  format: json\ncolumns:\n  title:\n    type: string\n"),
	}}
	def, collectionID, recordKey, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err != nil {
		t.Fatalf("readRemoteDefinitionForIDWithReader: %v", err)
	}
	if collectionID != "todo.tags" {
		t.Fatalf("expected collectionID todo.tags, got %s", collectionID)
	}
	if recordKey != "active" {
		t.Fatalf("expected recordKey active, got %s", recordKey)
	}
	colDef := def.Collections[collectionID]
	if colDef == nil {
		t.Fatal("expected collection in definition")
		return
	}
	if colDef.DirPath != "docs/demo-apps/todo/tags" {
		t.Fatalf("unexpected DirPath: %s", colDef.DirPath)
	}
}

func TestReadRecord_GitHubWithPathUnsupported(t *testing.T) {
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := readRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=todo.tags/active", "--github=ingitdb/ingitdb-cli", "--path=/tmp/cache")
	if err == nil {
		t.Fatal("expected error for --github with --path")
	}
}

func TestGithubToken_FromFlag(t *testing.T) {
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "test-token" {
				t.Fatalf("expected test-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app", "--token=test-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGithubToken_FromEnv(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "env-token" {
				t.Fatalf("expected env-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGithubToken_FlagTakesPrecedence(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "env-token")
	app := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			token := githubToken(cmd)
			if token != "flag-token" {
				t.Fatalf("expected flag-token, got %s", token)
			}
			return nil
		},
	}
	err := app.Run(context.Background(), []string{"app", "--token=flag-token"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewGitHubConfig(t *testing.T) {
	spec := githubRepoSpec{
		Owner: "testowner",
		Repo:  "testrepo",
		Ref:   "main",
	}
	cfg := newGitHubConfig(spec, "test-token")
	if cfg.Owner != "testowner" {
		t.Fatalf("expected owner testowner, got %s", cfg.Owner)
	}
	if cfg.Repo != "testrepo" {
		t.Fatalf("expected repo testrepo, got %s", cfg.Repo)
	}
	if cfg.Ref != "main" {
		t.Fatalf("expected ref main, got %s", cfg.Ref)
	}
	if cfg.Token != "test-token" {
		t.Fatalf("expected token test-token, got %s", cfg.Token)
	}
}

func TestListCollections_GitHub(t *testing.T) {
	reader := fakeFileReader{
		files: map[string][]byte{
			".ingitdb.yaml": []byte("rootCollections:\n  countries: test-ingitdb/countries\n  todo.tags: docs/demo-apps/todo/tags\n"),
		},
	}
	collections, err := listCollectionsFromFileReader(&reader)
	if err != nil {
		t.Fatalf("listCollectionsFromFileReader: %v", err)
	}
	expectedCollections := []string{"countries", "todo.tags"}
	if len(collections) != len(expectedCollections) {
		t.Fatalf("expected %d collections, got %d", len(expectedCollections), len(collections))
	}
	for i, expected := range expectedCollections {
		if collections[i] != expected {
			t.Fatalf("expected collection %q at index %d, got %q", expected, i, collections[i])
		}
	}
}

func TestReadRemoteDefinitionForIDWithReader_RootConfigNotFound(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{}}
	_, _, _, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err == nil {
		t.Fatal("expected error when root config not found")
	}
}

func TestReadRemoteDefinitionForIDWithReader_InvalidRootConfig(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("invalid yaml: ["),
	}}
	_, _, _, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err == nil {
		t.Fatal("expected error when root config is invalid")
	}
}

func TestReadRemoteDefinitionForIDWithReader_CollectionDefNotFound(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: docs/demo-apps/todo/tags\n"),
	}}
	_, _, _, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err == nil {
		t.Fatal("expected error when collection def not found")
	}
}

func TestReadRemoteDefinitionForIDWithReader_InvalidCollectionDef(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: docs/demo-apps/todo/tags\n"),
		"docs/demo-apps/todo/tags/.collection/todo.tags.yaml": []byte("invalid yaml: ["),
	}}
	_, _, _, err := readRemoteDefinitionForIDWithReader(context.Background(), "todo.tags/active", reader)
	if err == nil {
		t.Fatal("expected error when collection def is invalid")
	}
}

func TestReadRemoteDefinitionForIDWithReader_UnresolvedID(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: docs/demo-apps/todo/tags\n"),
	}}
	_, _, _, err := readRemoteDefinitionForIDWithReader(context.Background(), "unknown.collection/active", reader)
	if err == nil {
		t.Fatal("expected error when ID cannot be resolved")
	}
}

func TestListCollectionsFromFileReader_NotFound(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{}}
	_, err := listCollectionsFromFileReader(&reader)
	if err == nil {
		t.Fatal("expected error when root config not found")
	}
}

func TestListCollectionsFromFileReader_InvalidYAML(t *testing.T) {

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("invalid yaml: ["),
	}}
	_, err := listCollectionsFromFileReader(&reader)
	if err == nil {
		t.Fatal("expected error when root config is invalid YAML")
	}
}

func TestListCollectionsFromFileReader_InvalidConfig(t *testing.T) {

	// Empty collection ID is invalid
	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  \"\": some/path\n"),
	}}
	_, err := listCollectionsFromFileReader(&reader)
	if err == nil {
		t.Fatal("expected error when root config validation fails")
	}
}

func TestResolveRemoteCollectionPath_LongestPrefix(t *testing.T) {

	rootCollections := map[string]string{
		"countries":            "test-ingitdb/countries",
		"countries.cities":     "test-ingitdb/countries/cities",
		"countries.cities.zip": "test-ingitdb/countries/cities/zip",
	}

	// Should resolve to the longest matching prefix
	collectionID, recordKey, collectionPath, err := resolveRemoteCollectionPath(rootCollections, "countries.cities.zip/12345")
	if err != nil {
		t.Fatalf("resolveRemoteCollectionPath: %v", err)
	}
	if collectionID != "countries.cities.zip" {
		t.Fatalf("expected collectionID countries.cities.zip, got %s", collectionID)
	}
	if recordKey != "12345" {
		t.Fatalf("expected recordKey 12345, got %s", recordKey)
	}
	if collectionPath != "test-ingitdb/countries/cities/zip" {
		t.Fatalf("expected collectionPath test-ingitdb/countries/cities/zip, got %s", collectionPath)
	}
}

func TestResolveRemoteCollectionPath_NoMatch(t *testing.T) {

	rootCollections := map[string]string{
		"countries": "test-ingitdb/countries",
	}

	_, _, _, err := resolveRemoteCollectionPath(rootCollections, "todo.tags/active")
	if err == nil {
		t.Fatal("expected error when no collection matches")
	}
}

func TestResolveRemoteCollectionPath_EmptyRecordKey(t *testing.T) {

	rootCollections := map[string]string{
		"countries": "test-ingitdb/countries",
	}

	// Should fail because there's no record key after the slash
	_, _, _, err := resolveRemoteCollectionPath(rootCollections, "countries/")
	if err == nil {
		t.Fatal("expected error when record key is empty")
	}
}

var _ dalgo2ghingitdb.FileReader = (*fakeFileReader)(nil)

func TestReadRecord_GitHub_ParseError(t *testing.T) {

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/wd", nil }
	readDefinition := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("unused")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("unused")
	}
	cmd := readRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=invalid")
	if err == nil {
		t.Fatal("expected error for invalid GitHub spec")
	}
}

func TestReadRecord_GitHub_ReadDefinitionError(t *testing.T) {

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
	cmd := readRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when reading remote definition fails")
	}
}

func TestReadRecord_GitHub_DBOpenError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockReader := &fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  test.items: test-ingitdb/items\n"),
		"test-ingitdb/items/.collection/test.items.yaml": []byte("record_file:\n  name: items.json\n  type: map[string]map[string]any\n  format: json\ncolumns:\n  name:\n    type: string\n"),
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
	cmd := readRecord(homeDir, getWd, readDefinition, newDB, func(...any) {})
	err := runCLICommand(cmd, "--id=test.items/x", "--github=owner/repo")
	if err == nil {
		t.Fatal("expected error when DB open fails")
	}
}

func TestReadRemoteDefinitionForID_WithMock(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := fakeFileReader{files: map[string][]byte{
		".ingitdb.yaml": []byte("rootCollections:\n  todo.tags: docs/demo-apps/todo/tags\n"),
		"docs/demo-apps/todo/tags/.collection/todo.tags.yaml": []byte("record_file:\n  name: tags.json\n  type: map[string]map[string]any\n  format: json\ncolumns:\n  title:\n    type: string\n"),
	}}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	spec := githubRepoSpec{Owner: "owner", Repo: "repo"}
	def, collectionID, recordKey, err := readRemoteDefinitionForID(context.Background(), spec, "todo.tags/active")
	if err != nil {
		t.Fatalf("readRemoteDefinitionForID: %v", err)
	}
	if collectionID != "todo.tags" {
		t.Fatalf("expected collectionID todo.tags, got %s", collectionID)
	}
	if recordKey != "active" {
		t.Fatalf("expected recordKey active, got %s", recordKey)
	}
	if def == nil {
		t.Fatal("expected non-nil definition")
	}
}

func TestReadRemoteDefinitionForID_FactoryError(t *testing.T) {

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(nil, errors.New("factory error"))

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	spec := githubRepoSpec{Owner: "owner", Repo: "repo"}
	_, _, _, err := readRemoteDefinitionForID(context.Background(), spec, "todo.tags/active")
	if err == nil {
		t.Fatal("expected error when factory fails")
	}
}
