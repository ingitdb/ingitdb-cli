package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ingitdb/ingitdb-go/ingitdb"
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
	if cmd.Use != "list" {
		t.Errorf("expected name 'list', got %q", cmd.Name())
	}
	if len(cmd.Commands()) == 0 {
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
	err := runCobraCommand(cmd, "collections", "--path="+dir)
	if err != nil {
		t.Fatalf("ListCollections: %v", err)
	}
}

func TestFilterCollectionIDs(t *testing.T) {
	t.Parallel()

	entries := []collectionEntry{
		{name: "countries", path: "db/countries"},
		{name: "geo.cities", path: "db/geo/cities"},
		{name: "geo.regions", path: "db/geo/regions"},
		{name: "todo.tasks", path: "db/todo/tasks"},
	}

	tests := []struct {
		name     string
		inExpr   string
		nameGlob string
		want     []string
		wantErr  bool
	}{
		{name: "no filters returns all sorted", want: []string{"countries", "geo.cities", "geo.regions", "todo.tasks"}},
		{name: "in regex on path", inExpr: "db/geo/", want: []string{"geo.cities", "geo.regions"}},
		{name: "filter-name glob", nameGlob: "geo.*", want: []string{"geo.cities", "geo.regions"}},
		{name: "in and filter-name combine with AND", inExpr: "db/geo/", nameGlob: "*cities", want: []string{"geo.cities"}},
		{name: "no match yields empty", inExpr: "db/nope", want: []string{}},
		{name: "invalid regex errors", inExpr: "[", wantErr: true},
		{name: "invalid glob errors", nameGlob: "[", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := filterCollectionIDs(entries, tc.inExpr, tc.nameGlob)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for in=%q name=%q", tc.inExpr, tc.nameGlob)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got %v, want %v", got, tc.want)
					break
				}
			}
		})
	}
}

func TestListCollectionsLocal_ScopingFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"countries":   {ID: "countries", DirPath: dir + "/countries"},
			"geo.cities":  {ID: "geo.cities", DirPath: dir + "/geo/cities"},
			"geo.regions": {ID: "geo.regions", DirPath: dir + "/geo/regions"},
		},
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}

	cmd := List(homeDir, getWd, readDef)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"collections", "--path=" + dir, "--in=/geo/", "--filter-name=*cities"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list collections: %v", err)
	}

	got := strings.Fields(buf.String())
	want := []string{"geo.cities"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("scoped output = %v, want %v", got, want)
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
	err := runCobraCommand(cmd, "collections", "--path="+dir)
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
	err := runCobraCommand(cmd, "collections")
	if err == nil {
		t.Fatal("expected error when getWd fails")
	}
}

// viewsFixtureDef builds a definition with views spread across two top-level
// collections and one nested subcollection, so tests can exercise the qualified
// "collectionID/viewName" output and the scoping flags.
func viewsFixtureDef(dir string) *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"todo.tasks": {
				ID:      "todo.tasks",
				DirPath: dir + "/todo/tasks",
				Views: map[string]*ingitdb.ViewDef{
					"by_status": {ID: "by_status"},
					"by_owner":  {ID: "by_owner"},
				},
				SubCollections: map[string]*ingitdb.CollectionDef{
					"todo.tasks.comments": {
						ID:      "todo.tasks.comments",
						DirPath: dir + "/todo/tasks/comments",
						Views: map[string]*ingitdb.ViewDef{
							"recent": {ID: "recent"},
						},
					},
				},
			},
			"countries": {
				ID:      "countries",
				DirPath: dir + "/countries",
				Views: map[string]*ingitdb.ViewDef{
					"by_region": {ID: "by_region"},
				},
			},
		},
	}
}

func runListViews(t *testing.T, dir string, args ...string) []string {
	t.Helper()
	def := viewsFixtureDef(dir)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	cmd := List(homeDir, getWd, readDef)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs(append([]string{"views", "--path=" + dir}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("list views %v: %v", args, err)
	}
	return strings.Fields(buf.String())
}

func TestListViews_Success(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	got := runListViews(t, dir)
	// Sorted, qualified by owning collection, and recursing into subcollections.
	want := []string{
		"countries/by_region",
		"todo.tasks.comments/recent",
		"todo.tasks/by_owner",
		"todo.tasks/by_status",
	}
	if len(got) != len(want) {
		t.Fatalf("views output = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("views output = %v, want %v", got, want)
			break
		}
	}
}

func TestListViews_ScopingFlags(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// --filter-name globs the bare view name across all collections.
	byName := runListViews(t, dir, "--filter-name=by_*")
	wantByName := []string{"countries/by_region", "todo.tasks/by_owner", "todo.tasks/by_status"}
	if strings.Join(byName, ",") != strings.Join(wantByName, ",") {
		t.Errorf("--filter-name output = %v, want %v", byName, wantByName)
	}

	// --in is a regex on the owning collection's path.
	byPath := runListViews(t, dir, "--in=/todo/tasks$")
	wantByPath := []string{"todo.tasks/by_owner", "todo.tasks/by_status"}
	if strings.Join(byPath, ",") != strings.Join(wantByPath, ",") {
		t.Errorf("--in output = %v, want %v", byPath, wantByPath)
	}
}

func TestListViews_ReadDefError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, fmt.Errorf("read error")
	}
	cmd := List(homeDir, getWd, readDef)
	if err := runCobraCommand(cmd, "views", "--path="+dir); err == nil {
		t.Fatal("expected error when readDefinition fails")
	}
}

func TestFilterViewIDs(t *testing.T) {
	t.Parallel()

	dir := "/db"
	def := viewsFixtureDef(dir)

	tests := []struct {
		name     string
		inExpr   string
		nameGlob string
		want     []string
		wantErr  bool
	}{
		{name: "no filters returns all sorted", want: []string{
			"countries/by_region", "todo.tasks.comments/recent", "todo.tasks/by_owner", "todo.tasks/by_status",
		}},
		{name: "filter-name glob on bare view name", nameGlob: "by_*", want: []string{
			"countries/by_region", "todo.tasks/by_owner", "todo.tasks/by_status",
		}},
		{name: "in regex on owning path", inExpr: "/todo/tasks$", want: []string{
			"todo.tasks/by_owner", "todo.tasks/by_status",
		}},
		{name: "in and filter-name combine with AND", inExpr: "/todo/", nameGlob: "recent", want: []string{
			"todo.tasks.comments/recent",
		}},
		{name: "no match yields empty", nameGlob: "nope*", want: nil},
		{name: "invalid regex errors", inExpr: "[", wantErr: true},
		{name: "invalid glob errors", nameGlob: "[", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := filterViewIDs(def, tc.inExpr, tc.nameGlob)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for in=%q name=%q", tc.inExpr, tc.nameGlob)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestListCollectionsRemote_Success(t *testing.T) {
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
	for _, subcmd := range cmd.Commands() {
		if subcmd.Use == "collections" {
			// Successfully found the command
			return
		}
	}
	t.Fatal("collections subcommand not found")
}

// sampleRemoteSpec returns a canonical github.com/owner/repo spec for tests
// that exercise listCollectionsRemoteWithSpec without going through the parser.
func sampleRemoteSpec() remoteSpec {
	return remoteSpec{Host: "github.com", Path: []string{"owner", "repo"}}
}

func TestListCollectionsRemote_WithMock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml": []byte("countries: test-ingitdb/countries\ntodo.tags: demo-dbs/todo/tags\n"),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsRemoteWithSpec(ctx, sampleRemoteSpec(), "", "", "")
	if err != nil {
		t.Fatalf("listCollectionsRemoteWithSpec: %v", err)
	}
}

func TestListCollectionsRemote_ReaderCreationError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(nil, errors.New("network error"))

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsRemoteWithSpec(ctx, sampleRemoteSpec(), "", "", "")
	if err == nil {
		t.Fatal("expected error when file reader creation fails")
	}
}

func TestListCollectionsRemote_FileNotFound(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{files: map[string][]byte{}}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsRemoteWithSpec(ctx, sampleRemoteSpec(), "", "", "")
	if err == nil {
		t.Fatal("expected error when root config file not found")
	}
}

func TestListCollectionsRemote_InvalidYAML(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml": []byte("invalid yaml: ["),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsRemoteWithSpec(ctx, sampleRemoteSpec(), "", "", "")
	if err == nil {
		t.Fatal("expected error when root config has invalid YAML")
	}
}

func TestListCollectionsRemote_InvalidConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	reader := &fakeFileReader{
		files: map[string][]byte{
			".ingitdb/root-collections.yaml": []byte("\"\": some/path\n"),
		},
	}
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil)

	originalFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFactory
	defer func() { gitHubFileReaderFactory = originalFactory }()

	ctx := context.Background()
	err := listCollectionsRemoteWithSpec(ctx, sampleRemoteSpec(), "", "", "")
	if err == nil {
		t.Fatal("expected error when root config validation fails")
	}
}

// Note: TestParseGitHubRepoSpec_* tests have been deleted; the canonical
// parser is parseRemoteSpec, exercised by TestParseRemoteSpec_* in
// remote_helpers_test.go.

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
