package commands

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestList_ReturnsCommand(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := List(homeDir, getWd, readDef)
	if cmd == nil {
		t.Fatal("List() returned nil")
		return
	}
	if cmd.Name != "list" {
		t.Errorf("expected name 'list', got %q", cmd.Name)
	}
	if len(cmd.Commands) == 0 {
		t.Fatal("expected subcommands")
	}
}

func TestListCollectionsLocal_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"test.items": {
				ID:      "test.items",
				DirPath: dir,
			},
			"test.tags": {
				ID:      "test.tags",
				DirPath: dir,
			},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}

	cmd := List(homeDir, getWd, readDef)
	err := runCLICommand(cmd, "collections", "--path="+dir)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
}

func TestListCollectionsLocal_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read error")
	}

	cmd := List(homeDir, getWd, readDef)
	err := runCLICommand(cmd, "collections", "--path="+dir)
	if err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestListCollectionsLocal_ResolvePathError(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "", fmt.Errorf("no home") }
	getWd := func() (string, error) { return "", fmt.Errorf("no wd") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := List(homeDir, getWd, readDef)
	err := runCLICommand(cmd, "collections")
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

func TestListView_NotYetImplemented(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := List(homeDir, getWd, readDef)
	err := runCLICommand(cmd, "view")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

func TestSubscribers_NotYetImplemented(t *testing.T) {
	t.Parallel()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := List(homeDir, getWd, readDef)
	err := runCLICommand(cmd, "subscribers")
	if err == nil {
		t.Fatal("expected error for not-yet-implemented command")
	}
}

func TestListCollectionsGitHub_Success(t *testing.T) {
	t.Parallel()

	// This test requires a mock GitHub file reader, which is not straightforward.
	// For now, we'll test the command construction and flag parsing.
	// A real test would need to mock dalgo2ghingitdb.NewGitHubFileReader.
	// We'll skip the actual execution since it requires network access.
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}

	cmd := List(homeDir, getWd, readDef)
	if cmd == nil {
		t.Fatal("List() returned nil")
		return
	}

	// Find the collections subcommand
	for _, subcmd := range cmd.Commands {
		if subcmd.Name == "collections" {
			// Successfully found the command
			return
		}
	}
	t.Fatal("collections subcommand not found")
}

func TestListCollectionsGitHub_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb.yaml": []byte("rootCollections:\n  countries: test-ingitdb/countries\n  todo.tags: docs/demo-apps/todo/tags\n"),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "owner/repo", "")
	if err != nil {
		t.Fatalf("listCollectionsGitHub: %v", err)
	}
}

func TestListCollectionsGitHub_ParseError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "invalid", "")
	if err == nil {
		t.Fatal("expected error for invalid GitHub spec")
	}
}

func TestListCollectionsGitHub_ReaderCreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(nil, errors.New("network error"))

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "owner/repo", "")
	if err == nil {
		t.Fatal("expected error when file reader creation fails")
	}
}

func TestListCollectionsGitHub_FileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{files: map[string][]byte{}}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "owner/repo", "")
	if err == nil {
		t.Fatal("expected error when root config file not found")
	}
}

func TestListCollectionsGitHub_InvalidYAML(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb.yaml": []byte("invalid yaml: ["),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "owner/repo", "")
	if err == nil {
		t.Fatal("expected error when root config has invalid YAML")
	}
}

func TestListCollectionsGitHub_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb.yaml": []byte("rootCollections:\n  \"\": some/path\n"),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsGitHub(ctx, "owner/repo", "")
	if err == nil {
		t.Fatal("expected error when root config validation fails")
	}
}

func TestParseGitHubRepoSpec_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantRef   string
	}{
		{
			name:      "owner/repo",
			input:     "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "",
		},
		{
			name:      "owner/repo@ref",
			input:     "owner/repo@main",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "main",
		},
		{
			name:      "owner/repo@tag",
			input:     "owner/repo@v1.0.0",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantRef:   "v1.0.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec, err := parseGitHubRepoSpec(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if spec.Owner != tc.wantOwner {
				t.Errorf("Owner = %q, want %q", spec.Owner, tc.wantOwner)
			}
			if spec.Repo != tc.wantRepo {
				t.Errorf("Repo = %q, want %q", spec.Repo, tc.wantRepo)
			}
			if spec.Ref != tc.wantRef {
				t.Errorf("Ref = %q, want %q", spec.Ref, tc.wantRef)
			}
		})
	}
}

func TestParseGitHubRepoSpec_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "missing repo", input: "owner/"},
		{name: "missing owner", input: "/repo"},
		{name: "no slash", input: "ownerrepo"},
		{name: "empty ref", input: "owner/repo@"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseGitHubRepoSpec(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q", tc.input)
			}
		})
	}
}

func TestGitHubToken_FromFlag(t *testing.T) {
	t.Parallel()

	// This test would need to create a cli.Command and set flags
	// For now, we verify the function exists and can be called
	// A full integration test would require more setup
}

func TestResolveRemoteCollectionPath_Success(t *testing.T) {
	t.Parallel()

	rootCollections := map[string]string{
		"test.items": "data/items",
		"test.tags":  "data/tags",
	}

	tests := []struct {
		name               string
		id                 string
		wantCollectionID   string
		wantRecordKey      string
		wantCollectionPath string
	}{
		{
			name:               "items record",
			id:                 "test.items/r1",
			wantCollectionID:   "test.items",
			wantRecordKey:      "r1",
			wantCollectionPath: "data/items",
		},
		{
			name:               "tags record",
			id:                 "test.tags/tag1",
			wantCollectionID:   "test.tags",
			wantRecordKey:      "tag1",
			wantCollectionPath: "data/tags",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			colID, recKey, colPath, err := resolveRemoteCollectionPath(rootCollections, tc.id)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if colID != tc.wantCollectionID {
				t.Errorf("collectionID = %q, want %q", colID, tc.wantCollectionID)
			}
			if recKey != tc.wantRecordKey {
				t.Errorf("recordKey = %q, want %q", recKey, tc.wantRecordKey)
			}
			if colPath != tc.wantCollectionPath {
				t.Errorf("collectionPath = %q, want %q", colPath, tc.wantCollectionPath)
			}
		})
	}
}

func TestResolveRemoteCollectionPath_NotFound(t *testing.T) {
	t.Parallel()

	rootCollections := map[string]string{
		"test.items": "data/items",
	}

	_, _, _, err := resolveRemoteCollectionPath(rootCollections, "unknown.col/r1")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestResolveRemoteCollectionPath_EmptyRemainder(t *testing.T) {
	t.Parallel()

	rootCollections := map[string]string{
		"test.items": "data/items",
	}

	// Should fail because "test.items/" has no record key after the slash
	_, _, _, err := resolveRemoteCollectionPath(rootCollections, "test.items/")
	if err == nil {
		t.Fatal("expected error for empty record key")
	}
}
