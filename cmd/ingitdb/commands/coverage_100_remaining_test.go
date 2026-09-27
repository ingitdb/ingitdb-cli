package commands

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dal-go/dalgo/dal"
	dalgo2ghingitdb "github.com/ingitdb/dalgo2ingitdb4github"
	dalgo2fsingitdb "github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/datavalidator"
	"github.com/ingitdb/ingitdb-go/ingitdb/validator"
	"github.com/spf13/cobra"
	"github.com/strongo/cli-helpers/skillsync"
	"go.uber.org/mock/gomock"
)

func TestCI_RecordsDelimiterFlag(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items", DirPath: dir},
		},
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	viewBuilder := &mockViewBuilder{
		result: &ingitdb.MaterializeResult{
			FilesCreated: 1,
		},
	}
	logf := func(...any) {}

	cmd := CI(homeDir, getWd, readDef, viewBuilder, logf)
	err := runCobraCommand(cmd, "--path="+dir, "--records-delimiter=2")
	if err != nil {
		t.Fatalf("CI with records-delimiter: %v", err)
	}
	if def.RuntimeOverrides.RecordsDelimiter == nil || *def.RuntimeOverrides.RecordsDelimiter != 2 {
		t.Fatalf("expected RecordsDelimiter=2, got %v", def.RuntimeOverrides.RecordsDelimiter)
	}
}

func TestCI_Branches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items", DirPath: dir},
		},
	}

	// 1. dirPath == "" uses getWd
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	viewBuilder := &mockViewBuilder{result: &ingitdb.MaterializeResult{}}
	cmdDefaultPath := CI(homeDir, getWd, readDef, viewBuilder, func(...any) {})
	if err := runCobraCommand(cmdDefaultPath); err != nil {
		t.Fatalf("expected success with default path, got %v", err)
	}

	// 2. expandHome error with ~
	homeDirErr := func() (string, error) { return "", errors.New("home error") }
	cmdHomeErr := CI(homeDirErr, getWd, readDef, viewBuilder, func(...any) {})
	if err := runCobraCommand(cmdHomeErr, "--path=~/db"); err == nil {
		t.Fatal("expected expandHome error")
	}

	// 3. readDefinition error
	readDefErr := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read def error")
	}
	cmdReadErr := CI(homeDir, getWd, readDefErr, viewBuilder, func(...any) {})
	if err := runCobraCommand(cmdReadErr, "--path="+dir); err == nil {
		t.Fatal("expected readDefinition error")
	}
}

func TestRequireRemoteWriteToken_SingleLabelHost(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().String("remote", "https://localhost/owner/repo", "")
	cmd.Flags().String("provider", "github", "")
	cmd.Flags().String("token", "", "")

	oldVal := os.Getenv("LOCALHOST_TOKEN")
	_ = os.Unsetenv("LOCALHOST_TOKEN")
	defer func() {
		if oldVal != "" {
			_ = os.Setenv("LOCALHOST_TOKEN", oldVal)
		}
	}()

	err := requireRemoteWriteToken(cmd)
	if err == nil {
		t.Fatal("expected error for missing token on remote write")
	}
	if !strings.Contains(err.Error(), "LOCALHOST_TOKEN") {
		t.Errorf("expected error to mention LOCALHOST_TOKEN, got %v", err)
	}
}

func TestMaybeWrapWithBatching_Invalid(t *testing.T) {
	badCmd := &cobra.Command{}
	badCmd.Flags().String("remote", "::invalid::", "")
	_, err := maybeWrapWithBatching(badCmd, nil, nil, "msg")
	if err == nil {
		t.Fatal("expected error on invalid remote")
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("remote", "https://github.com/owner/repo", "")
	cmd.Flags().String("provider", "github", "")
	cmd.Flags().String("token", "tok", "")
	_, err = maybeWrapWithBatching(cmd, nil, nil, "msg")
	if err == nil {
		t.Fatal("expected error from NewBatchingGitHubDB when def is nil")
	}
}

func TestRunDeleteByID_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cmd := &cobra.Command{}
	cmd.Flags().String("path", "/nonexistent", "")
	cmd.Flags().String("remote", "", "")

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/nonexistent", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read def error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, errors.New("new db error")
	}

	err := runDeleteByID(ctx, cmd, "col/key", homeDir, getWd, readDef, newDB)
	if err == nil {
		t.Fatal("expected error when resolveRecordContext fails")
	}
}

type failWriter struct{}

func (f failWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failure")
}

func TestDiff_Branches(t *testing.T) {
	ctx := context.Background()

	// 1. orWorkingTree
	if got := orWorkingTree(""); got != "(working tree)" {
		t.Errorf("expected '(working tree)', got %q", got)
	}
	if got := orWorkingTree("main"); got != "main" {
		t.Errorf("expected 'main', got %q", got)
	}

	// 2. parseKeyedRecords - SingleRecord corrupted
	colSingle := &ingitdb.CollectionDef{
		ID: "items",
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: ingitdb.SingleRecord,
			Format:     "yaml",
		},
	}
	badRecs := parseKeyedRecords([]byte("invalid: [yaml: broken"), colSingle, "items/item1.yaml")
	if len(badRecs) != 0 {
		t.Errorf("expected empty map on corrupted record, got %v", badRecs)
	}

	// 3. parseKeyedRecords - MapOfRecords
	colMap := &ingitdb.CollectionDef{
		ID: "items",
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: ingitdb.MapOfRecords,
			Format:     "yaml",
		},
	}
	mapContent := []byte("key1:\n  title: item 1\nkey2:\n  title: item 2\n")
	mapRecs := parseKeyedRecords(mapContent, colMap, "items.yaml")
	if len(mapRecs) != 2 {
		t.Errorf("expected 2 map records, got %d", len(mapRecs))
	}
	badMapRecs := parseKeyedRecords([]byte("not a map"), colMap, "items.yaml")
	if len(badMapRecs) != 0 {
		t.Errorf("expected empty map on invalid map content, got %v", badMapRecs)
	}

	// 4. parseKeyedRecords - ListOfRecords
	colList := &ingitdb.CollectionDef{
		ID: "items",
		RecordFile: &ingitdb.RecordFileDef{
			RecordType: ingitdb.ListOfRecords,
			Format:     "yaml",
		},
		Columns: map[string]*ingitdb.ColumnDef{
			"id": {Type: ingitdb.ColumnTypeString, Required: true},
		},
	}
	listContent := []byte("- id: k1\n  title: i1\n- id: k2\n  title: i2\n")
	listRecs := parseKeyedRecords(listContent, colList, "items.yaml")
	if len(listRecs) != 2 {
		t.Errorf("expected 2 list records, got %d", len(listRecs))
	}
	badListRecs := parseKeyedRecords([]byte("not a list: {"), colList, "items.yaml")
	if len(badListRecs) != 0 {
		t.Errorf("expected empty list on invalid list content, got %v", badListRecs)
	}

	// 5. gitShow with ref == ""
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	_ = os.WriteFile(filePath, []byte("hello"), 0o644)
	if got := gitShow(ctx, tmpDir, "", "test.txt"); string(got) != "hello" {
		t.Errorf("expected 'hello', got %q", string(got))
	}
	if got := gitShow(ctx, tmpDir, "", "nonexistent.txt"); got != nil {
		t.Errorf("expected nil for nonexistent file, got %v", got)
	}

	// 6. renderDiff formats and writer errors
	report := &diffReport{
		From: "main",
		To:   "HEAD",
		Summary: []collectionCount{
			{Collection: "items", Added: 1, Updated: 1, Deleted: 1},
		},
		Records: []recordChange{
			{
				Collection: "items",
				Key:        "k1",
				Kind:       diffUpdated,
				Fields:     []fieldChange{{Field: "title", Before: "old", After: "new"}},
			},
		},
	}
	var buf strings.Builder
	if err := renderDiff(&buf, report, "summary", "yaml"); err != nil {
		t.Errorf("renderDiff yaml: %v", err)
	}
	buf.Reset()
	if err := renderDiff(&buf, report, "fields", "toml"); err != nil {
		t.Errorf("renderDiff toml: %v", err)
	}
	buf.Reset()
	if err := renderDiff(&buf, report, "full", "json"); err != nil {
		t.Errorf("renderDiff json: %v", err)
	}

	// Writer failure on yaml and toml
	if err := renderDiff(failWriter{}, report, "summary", "yaml"); err == nil {
		t.Error("expected yaml writer error")
	}
	if err := renderDiff(failWriter{}, report, "summary", "toml"); err == nil {
		t.Error("expected toml writer error")
	}

	// 7. diffView depth
	sumView := diffView(report, "summary")
	if len(sumView.Records) != 0 {
		t.Errorf("expected 0 records in summary view, got %d", len(sumView.Records))
	}
	fieldsView := diffView(report, "fields")
	if len(fieldsView.Records) != 1 || fieldsView.Records[0].Fields[0].Before != nil {
		t.Errorf("expected field change without before/after, got %v", fieldsView.Records[0].Fields[0])
	}
	fullView := diffView(report, "full")
	if fullView.Records[0].Fields[0].Before != "old" {
		t.Errorf("expected full fields, got %v", fullView.Records[0].Fields[0])
	}

	// 8. Diff command mutual exclusion
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return tmpDir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	diffCmd := Diff(homeDir, getWd, readDef, func(...any) {}, func(int) {})
	err := runCobraCommand(diffCmd, "--collection=col", "--view=v")
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}

	// 9. Diff command error branches
	errGetWd := func() (string, error) { return "", errors.New("wd error") }
	cmdWdErr := Diff(homeDir, errGetWd, readDef, func(...any) {}, func(int) {})
	if err := runCobraCommand(cmdWdErr); err == nil {
		t.Fatal("expected getWd error")
	}

	errReadDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read definition error")
	}
	cmdReadErr := Diff(homeDir, getWd, errReadDef, func(...any) {}, func(int) {})
	if err := runCobraCommand(cmdReadErr, "--path="+tmpDir); err == nil {
		t.Fatal("expected readDefinition error")
	}

	cmdBadDiff := Diff(homeDir, getWd, readDef, func(...any) {}, func(int) {})
	if err := runCobraCommand(cmdBadDiff, "--path="+tmpDir, "main..HEAD"); err == nil {
		t.Fatal("expected computeDiff error on non-git dir")
	}
}

func TestDiff_ComputeDiffFilteringAndSorting(t *testing.T) {
	dir, base, readDef := diffTestRepo(t)
	def, err := readDef(dir)
	if err != nil {
		t.Fatalf("readDef: %v", err)
	}

	// Add second collection "cities" to test sorting across collections
	def.Collections["cities"] = &ingitdb.CollectionDef{
		ID:      "cities",
		DirPath: filepath.Join(dir, "cities"),
		RecordFile: &ingitdb.RecordFileDef{
			Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
		},
	}
	recDir := filepath.Join(dir, "cities", "$records")
	_ = os.MkdirAll(recDir, 0o755)
	_ = os.WriteFile(filepath.Join(recDir, "nyc.yaml"), []byte("name: NYC\n"), 0o644)

	// Add an unrelated non-record file to git history so colDef == nil branch is hit
	unrelatedFile := filepath.Join(dir, "README.md")
	_ = os.WriteFile(unrelatedFile, []byte("updated readme"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "add cities and update readme")

	// 1. Path filter matching & non-matching
	rep, err := computeDiff(context.Background(), dir, def, base, "HEAD", "", "people/")
	if err != nil {
		t.Fatalf("computeDiff with path filter: %v", err)
	}
	if len(rep.Records) == 0 {
		t.Errorf("expected records matching people/")
	}

	repGlob, err := computeDiff(context.Background(), dir, def, base, "HEAD", "", "people/$records/*.yaml")
	if err != nil {
		t.Fatalf("computeDiff with glob: %v", err)
	}
	if len(repGlob.Records) == 0 {
		t.Errorf("expected records matching glob")
	}

	repNoMatch, err := computeDiff(context.Background(), dir, def, base, "HEAD", "", "nonexistent/*")
	if err != nil {
		t.Fatalf("computeDiff with non-matching path filter: %v", err)
	}
	if len(repNoMatch.Records) != 0 {
		t.Errorf("expected 0 records, got %d", len(repNoMatch.Records))
	}

	// 2. Collection filter matching and non-matching
	repCollMismatch, err := computeDiff(context.Background(), dir, def, base, "HEAD", "other_collection", "")
	if err != nil {
		t.Fatalf("computeDiff with mismatching collection: %v", err)
	}
	if len(repCollMismatch.Records) != 0 {
		t.Errorf("expected 0 records for mismatched collection, got %d", len(repCollMismatch.Records))
	}

	// 3. Multi-collection diff to test sorting
	repAll, err := computeDiff(context.Background(), dir, def, base, "HEAD", "", "")
	if err != nil {
		t.Fatalf("computeDiff all: %v", err)
	}
	if len(repAll.Records) < 2 {
		t.Errorf("expected multiple records across collections, got %d", len(repAll.Records))
	}
}

func TestDiff_RenamedRecord(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	disableGitBackgroundMaintenance(t, dir)
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")

	recDir := filepath.Join(dir, "people", "$records")
	_ = os.MkdirAll(recDir, 0o755)
	_ = os.WriteFile(filepath.Join(recDir, "old.yaml"), []byte("name: Old\n"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")
	base := strings.TrimSpace(string(runGit(t, dir, "rev-parse", "HEAD")))

	runGit(t, dir, "mv", "people/$records/old.yaml", "people/$records/new.yaml")
	_ = os.WriteFile(filepath.Join(recDir, "new.yaml"), []byte("name: New\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "rename")

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"people": {
				ID:      "people",
				DirPath: filepath.Join(dir, "people"),
				RecordFile: &ingitdb.RecordFileDef{
					Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{"name": {Type: ingitdb.ColumnTypeString}},
			},
		},
	}

	rep, err := computeDiff(context.Background(), dir, def, base, "HEAD", "", "")
	if err != nil {
		t.Fatalf("computeDiff with rename: %v", err)
	}
	if len(rep.Records) == 0 {
		t.Errorf("expected records from rename diff")
	}
}

func TestDiff_RenderDiffError(t *testing.T) {
	dir, base, readDef := diffTestRepo(t)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	cmd := Diff(homeDir, getWd, readDef, func(...any) {}, func(int) {})
	cmd.SetOut(failWriter{})
	cmd.SetArgs([]string{base + "..HEAD", "--path=" + dir, "--format=yaml"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected renderDiff error when writer fails")
	}
}

func TestDropRemote_InvalidRemote(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cmd := &cobra.Command{}
	cmd.Flags().String("remote", "::bad::", "")
	cmd.Flags().String("provider", "", "")
	cmd.Flags().String("token", "", "")

	if err := dropCollectionRemote(ctx, cmd, "col", false); err == nil {
		t.Fatal("expected error from dropCollectionRemote on invalid remote")
	}
	if err := dropViewRemote(ctx, cmd, "v", "", false); err == nil {
		t.Fatal("expected error from dropViewRemote on invalid remote")
	}
}

func TestInsert_Errors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. resolveInsertContext with both --path and --remote
	cmdPathRemote := &cobra.Command{}
	cmdPathRemote.Flags().String("path", "/some/path", "")
	cmdPathRemote.Flags().String("remote", "https://github.com/owner/repo", "")
	_, err := resolveInsertContext(ctx, cmdPathRemote, "col", nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "--path with --remote is not supported") {
		t.Fatalf("expected --path with --remote error, got %v", err)
	}

	// 2. Insert command when resolveInsertContext fails
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read def error")
	}
	newDB := func(_ string, _ *ingitdb.Definition) (dal.DB, error) {
		return nil, nil
	}
	cmdInsert := Insert(homeDir, getWd, readDef, newDB, func(...any) {}, nil, nil, nil)
	err = runCobraCommand(cmdInsert, "--into=items", "--path=/tmp/db", "--empty", "--key=k1")
	if err == nil {
		t.Fatal("expected error from Insert when resolveInsertContext fails")
	}
}

type mockFileReaderReturningRootCollections struct{}

func (m *mockFileReaderReturningRootCollections) ReadFile(_ context.Context, p string) ([]byte, bool, error) {
	if p == ".ingitdb/root-collections.yaml" {
		return []byte("items: items\n"), true, nil
	}
	return nil, false, nil
}

func (m *mockFileReaderReturningRootCollections) ListDirectory(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

type mockFileReaderFactoryRootCollections struct{}

func (m *mockFileReaderFactoryRootCollections) NewGitHubFileReader(_ dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
	return &mockFileReaderReturningRootCollections{}, nil
}

func TestList_FilterRegexError(t *testing.T) {
	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items", DirPath: dir, Views: map[string]*ingitdb.ViewDef{"v1": {ID: "v1"}}},
		},
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}

	// 1. listCollectionsLocal with invalid regex
	cmdCol := collections(homeDir, getWd, readDef)
	err := runCobraCommand(cmdCol, "--path="+dir, "--in=[")
	if err == nil {
		t.Fatal("expected regex error from list collections")
	}

	// 2. listCollectionsRemoteWithSpec with invalid regex when file exists
	origReader := gitHubFileReaderFactory
	gitHubFileReaderFactory = &mockFileReaderFactoryRootCollections{}
	defer func() { gitHubFileReaderFactory = origReader }()

	err = listCollectionsRemoteWithSpec(context.Background(), remoteSpec{Host: "github.com", Path: []string{"o", "r"}}, "tok", "[", "")
	if err == nil {
		t.Fatal("expected regex filter error from listCollectionsRemoteWithSpec")
	}

	// 3. listViews with resolveDBPath error
	errGetWd := func() (string, error) { return "", errors.New("wd error") }
	cmdViewsWd := listViews(homeDir, errGetWd, readDef)
	if err := runCobraCommand(cmdViewsWd); err == nil {
		t.Fatal("expected getWd error in listViews")
	}

	// 4. listViews with invalid regex
	cmdViews := listViews(homeDir, getWd, readDef)
	err = runCobraCommand(cmdViews, "--path="+dir, "--in=[")
	if err == nil {
		t.Fatal("expected regex error from list views")
	}
}

type mockNonSimpleViewBuilder struct{}

func (m *mockNonSimpleViewBuilder) BuildViews(_ context.Context, _, _ string, _ *ingitdb.CollectionDef, _ *ingitdb.Definition) (*ingitdb.MaterializeResult, error) {
	return &ingitdb.MaterializeResult{}, nil
}

func TestMaterialize_Branches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. mergeMaterializeResult(dst, nil)
	dst := &ingitdb.MaterializeResult{FilesCreated: 1}
	mergeMaterializeResult(dst, nil)
	if dst.FilesCreated != 1 {
		t.Errorf("expected FilesCreated=1, got %d", dst.FilesCreated)
	}

	// 2. materializeViews with non-SimpleViewBuilder when filtering by name
	colDir := filepath.Join(t.TempDir(), "items")
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items", DirPath: colDir, Views: map[string]*ingitdb.ViewDef{"v1": {ID: "v1"}}},
		},
	}
	sel := selection{kind: selectionList, globs: []string{"v1"}}
	_, err := materializeViews(ctx, &mockNonSimpleViewBuilder{}, def, "/tmp", "", sel)
	if err == nil || !strings.Contains(err.Error(), "requires the standard view builder") {
		t.Fatalf("expected non-simple view builder error, got %v", err)
	}

	// 3. materializeViews when BuildView fails
	mockBuilder := &mockViewBuilder{err: errors.New("build view failed")}
	_, err = materializeViews(ctx, mockBuilder, def, "/tmp", "", sel)
	if err == nil {
		t.Fatal("expected BuildView error")
	}

	// 4. materializeCollections when docsbuilder fails
	_, _ = materializeCollections(ctx, def, "/nonexistent/invalid/dir", selection{kind: selectionAll})

	// 5. materializeCommandRunE with collection error
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/nonexistent/invalid", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	cmdMat := Materialize(homeDir, getWd, readDef, mockBuilder, func(...any) {})
	_ = runCobraCommand(cmdMat, "--path=/nonexistent/invalid", "--views=v1")
}

func TestPull_Branches(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	// 1. gitHeadRef error in non-git directory
	_, err := gitHeadRef(ctx, dir)
	if err == nil {
		t.Fatal("expected gitHeadRef error on non-git dir")
	}

	// 2. rebuildViews with nil viewBuilder
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{"items": {ID: "items"}},
	}
	if err := rebuildViews(ctx, dir, def, nil, func(...any) {}); err != nil {
		t.Fatalf("expected nil error when viewBuilder is nil, got %v", err)
	}

	// 3. rebuildViews with error
	viewBuilderErr := &mockViewBuilder{err: errors.New("rebuild error")}
	if err := rebuildViews(ctx, dir, def, viewBuilderErr, func(...any) {}); err == nil {
		t.Fatal("expected rebuildViews error")
	}

	// 4. summarizeRecordChanges error on non-git dir
	if _, err := summarizeRecordChanges(ctx, dir, def, "HEAD"); err == nil {
		t.Fatal("expected summarizeRecordChanges error on non-git dir")
	}

	// 5. Pull command with getWd error
	homeDir := func() (string, error) { return "/tmp/home", nil }
	errGetWd := func() (string, error) { return "", errors.New("wd error") }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return def, nil
	}
	cmdPull := Pull(homeDir, errGetWd, readDef, nil, func(...any) {}, func() bool { return false }, nil)
	if err := runCobraCommand(cmdPull); err == nil {
		t.Fatal("expected error when getWd fails in Pull")
	}

	// 6. Pull command on non-git dir (fails at gitHeadRef)
	getWd := func() (string, error) { return dir, nil }
	cmdPullDir := Pull(homeDir, getWd, readDef, nil, func(...any) {}, func() bool { return false }, nil)
	if err := runCobraCommand(cmdPullDir, "--path="+dir); err == nil {
		t.Fatal("expected error on non-git dir in Pull")
	}

	// 7. runGitPull error logging
	_ = runGitPull(ctx, dir, "rebase", "origin", "main", func(...any) {})
}

func TestPull_ConflictAndErrorBranches(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	disableGitBackgroundMaintenance(t, dir)
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")

	rootCfg := filepath.Join(dir, ".ingitdb", "root-collections.yaml")
	_ = os.MkdirAll(filepath.Dir(rootCfg), 0o755)
	_ = os.WriteFile(rootCfg, []byte("people: people\n"), 0o644)

	colDefPath := filepath.Join(dir, "people", ".collection", "definition.yaml")
	_ = os.MkdirAll(filepath.Dir(colDefPath), 0o755)
	_ = os.WriteFile(colDefPath, []byte("record_file:\n  name: \"{key}.yaml\"\n  format: yaml\n  type: \"map[string]any\"\ncolumns:\n  name:\n    type: string\n"), 0o644)

	recDir := filepath.Join(dir, "people", "$records")
	_ = os.MkdirAll(recDir, 0o755)
	_ = os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("name: A\n"), 0o644)
	_ = os.WriteFile(filepath.Join(recDir, "b.yaml"), []byte("name: B\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "base")
	base := strings.TrimSpace(string(runGit(t, dir, "rev-parse", "HEAD")))

	// Delete b and modify a
	_ = os.Remove(filepath.Join(recDir, "b.yaml"))
	_ = os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("name: A modified\n"), 0o644)
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "delete and modify")

	def, err := validator.ReadDefinition(dir)
	if err != nil {
		t.Fatalf("ReadDefinition: %v", err)
	}

	// Test summarizeRecordChanges exercising deleted and modified
	summary, err := summarizeRecordChanges(context.Background(), dir, def, base)
	if err != nil {
		t.Fatalf("summarizeRecordChanges: %v", err)
	}
	if !strings.Contains(summary, "pulled:") {
		t.Errorf("expected 'pulled:' in summary, got: %s", summary)
	}

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }

	// 1. Pull with bad remote causing pullErr != nil
	readDefOk := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	cmdPullBadRemote := Pull(homeDir, getWd, readDefOk, nil, func(...any) {}, func() bool { return false }, nil)
	err = runCobraCommand(cmdPullBadRemote, "--path="+dir, "--remote=nonexistent_remote_xyz")
	if err == nil || !strings.Contains(err.Error(), "git pull failed") {
		t.Fatalf("expected 'git pull failed' error, got: %v", err)
	}

	// 2. resolveWorkingTreeConflicts error return
	runConflictsTUIErr := func(context.Context, []string) error {
		return errors.New("tui conflict error")
	}
	err = resolveWorkingTreeConflicts(context.Background(), dir, def, []string{"people/$records/a.yaml"}, func() bool { return true }, runConflictsTUIErr, func(...any) {})
	if err == nil {
		t.Fatal("expected resolveWorkingTreeConflicts error")
	}

	// 3. Pull command when view definition read fails
	readCallCount := 0
	readDefViewErr := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		readCallCount++
		if readCallCount > 0 {
			return nil, errors.New("view def read error")
		}
		return def, nil
	}
	cmdPullViewErr := Pull(homeDir, getWd, readDefViewErr, nil, func(...any) {}, func() bool { return false }, nil)
	_ = runCobraCommand(cmdPullViewErr, "--path="+dir)
}

func TestDeleteAndUpdate_RemoteSetModeNoViews(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dir := t.TempDir()
	colDefYAML := "id: items\ncolumns:\n  title:\n    type: string\n"
	reader := &fakeFileReader{files: map[string][]byte{
		".ingitdb/root-collections.yaml":     []byte("items: data/items\n"),
		"data/items/.schema/items.yaml":     []byte(colDefYAML),
		"data/items/.collection/items.yaml": []byte(colDefYAML),
	}}
	mockFileReaderFactory := NewMockGitHubFileReaderFactory(ctrl)
	mockFileReaderFactory.EXPECT().NewGitHubFileReader(gomock.Any()).Return(reader, nil).AnyTimes()

	localDef := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {
				ID:      "items",
				DirPath: dir,
				RecordFile: &ingitdb.RecordFileDef{
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
				Columns: map[string]*ingitdb.ColumnDef{"title": {Type: ingitdb.ColumnTypeString}},
			},
		},
	}
	localDB, dbErr := dalgo2fsingitdb.NewLocalDBWithDef(dir, localDef)
	if dbErr != nil {
		t.Fatalf("NewLocalDBWithDef: %v", dbErr)
	}
	mockDBFactory := NewMockGitHubDBFactory(ctrl)
	mockDBFactory.EXPECT().NewGitHubDBWithDef(gomock.Any(), gomock.Any()).Return(localDB, nil).AnyTimes()

	origFileFactory := gitHubFileReaderFactory
	gitHubFileReaderFactory = mockFileReaderFactory
	defer func() { gitHubFileReaderFactory = origFileFactory }()

	origDBFactory := gitHubDBFactory
	gitHubDBFactory = mockDBFactory
	defer func() { gitHubDBFactory = origDBFactory }()

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return localDef, nil }
	newDB := func(_ string, d *ingitdb.Definition) (dal.DB, error) { return localDB, nil }
	logf := func(...any) {}

	// Delete from remote
	cmdDel := Delete(homeDir, getWd, readDef, newDB, logf)
	_ = runCobraCommand(cmdDel, "--from=items", "--all", "--remote=github.com/owner/repo", "--token=test-token")

	// Update on remote
	cmdUpd := Update(homeDir, getWd, readDef, newDB, logf)
	_ = runCobraCommand(cmdUpd, "--from=items", "--all", "--set=title=new", "--remote=github.com/owner/repo", "--token=test-token")
}

func TestRebase_AbortFailsWithUnresolved(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()

	err := gitRebaseAbort(ctx, dir)
	if err == nil {
		t.Fatal("expected gitRebaseAbort error on non-git directory")
	}
}

func TestSkills_ConfigAndErrors(t *testing.T) {
	// 1. NewSkillsConfig with invalid version string
	cfg, err := NewSkillsConfig("invalid-version")
	if err != nil {
		t.Fatalf("unexpected error from NewSkillsConfig: %v", err)
	}
	if cfg.CurrentVersion != "invalid-version" {
		t.Errorf("expected 'invalid-version', got %q", cfg.CurrentVersion)
	}

	// 2. skillsSyncErrors.Conflict
	errs := skillsSyncErrors{}
	rep := skillsync.Report{Dir: "/test/skills"}
	conflictErr := errs.Conflict(rep)
	if conflictErr == nil || !strings.Contains(conflictErr.Error(), "skills sync:") {
		t.Fatalf("expected conflict message, got %v", conflictErr)
	}

	// 3. NewSkillsConfig with broken skillsFS
	origFS := skillsFS
	defer func() { skillsFS = origFS }()
	skillsFS = fstest.MapFS{} // missing skills/

	_, err = NewSkillsConfig("1.0.0")
	if err == nil {
		t.Fatal("expected error on missing skills directory")
	}

	// 4. Skills command with broken skillsFS
	badCmd := Skills("1.0.0")
	if err := runCobraCommand(badCmd); err == nil {
		t.Fatal("expected error when Skills command runs with broken skillsFS")
	}
}

func TestValidate_SafeDiagnosticsBranches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// 1. safeRepositoryPath test
	absDir, _ := filepath.Abs(dir)
	if got := safeRepositoryPath(absDir, filepath.Join(absDir, "sub", "file.yaml")); got != "sub/file.yaml" {
		t.Errorf("expected 'sub/file.yaml', got %q", got)
	}
	if got := safeRepositoryPath(absDir, "/outside/file.yaml"); got != "file.yaml" {
		t.Errorf("expected fallback 'file.yaml', got %q", got)
	}

	// 2. safeConstraintClass test all branches
	tests := []struct {
		msg  string
		want string
	}{
		{"foreign key violation", "foreign-key constraint failed"},
		{"missing required field name", "required-field constraint failed"},
		{"wrong type for int", "type constraint failed"},
		{"not one of the permitted values", "enum constraint failed"},
		{"value exceeds min_value", "range constraint failed"},
		{"length below min_length", "length constraint failed"},
		{"undeclared field extra", "declared-field constraint failed"},
		{"cannot set computed column", "computed-field constraint failed"},
		{"expected at least 1 record(s)", "record-count constraint failed"},
		{"failed to parse record", "record-format constraint failed"},
		{"other random problem", "validation constraint failed"},
	}
	for _, tc := range tests {
		if got := safeConstraintClass(tc.msg); got != tc.want {
			t.Errorf("safeConstraintClass(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}

	// 3. formatSafeValidationFailure / formatSafeValidationErrors / formatValidationFailure / formatValidationErrors with 0 errors
	resEmpty := &ingitdb.ValidationResult{}
	if got := formatSafeValidationFailure("prefix", dir, resEmpty); !strings.Contains(got, "0 error(s)") {
		t.Errorf("expected '0 error(s)', got %q", got)
	}
	if got := formatValidationFailure("prefix", resEmpty); !strings.Contains(got, "0 error(s)") {
		t.Errorf("expected '0 error(s)', got %q", got)
	}
	if got := formatSafeValidationErrors(dir, nil); got != "" {
		t.Errorf("expected empty string for nil errors, got %q", got)
	}
	if got := formatValidationErrors(nil); got != "" {
		t.Errorf("expected empty string for nil errors, got %q", got)
	}

	// 4. Validate command with safeDiagnostics
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDefErr := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read error")
	}
	cmdValDefErr := Validate(homeDir, getWd, readDefErr, nil, nil, func(...any) {})
	err := runCobraCommand(cmdValDefErr, "--path="+dir, "--only=records", "--safe-diagnostics")
	if err == nil || !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed with safe-diagnostics, got %v", err)
	}

	// 5. Data validator failure with safeDiagnostics
	mockDataValErr := &mockDataValidator{err: errors.New("data val failed")}
	readDefOk := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"c": {ID: "c"}}}, nil
	}
	cmdValDataErr := Validate(homeDir, getWd, readDefOk, mockDataValErr, nil, func(...any) {})
	err = runCobraCommand(cmdValDataErr, "--path="+dir, "--safe-diagnostics")
	if err == nil || !strings.Contains(err.Error(), "data validator could not complete") {
		t.Fatalf("expected 'data validator could not complete', got %v", err)
	}

	// 6. Incremental validator with safeDiagnostics
	mockIncValResult := &mockIncrementalValidator{
		result: func() *ingitdb.ValidationResult {
			r := &ingitdb.ValidationResult{}
			r.Append(ingitdb.ValidationError{Message: "missing required field", CollectionID: "c", RecordKey: "k", FieldName: "f", FilePath: "items/k.yaml"})
			return r
		}(),
	}
	cmdValInc := Validate(homeDir, getWd, readDefOk, nil, mockIncValResult, func(...any) {})
	err = runCobraCommand(cmdValInc, "--path="+dir, "--from-commit=HEAD~1", "--safe-diagnostics")
	if err == nil || !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed for incremental safe-diagnostics, got %v", err)
	}
}

func TestValidationFailure_Nil(t *testing.T) {
	t.Parallel()
	err := NewValidationFailedError(nil)
	if !errors.Is(err, ErrValidationFailed) {
		t.Fatalf("expected ErrValidationFailed, got %v", err)
	}
}

func TestDiff_RenamedFile(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	disableGitBackgroundMaintenance(t, dir)
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")

	recDir := filepath.Join(dir, "items", "$records")
	_ = os.MkdirAll(recDir, 0o755)
	_ = os.WriteFile(filepath.Join(recDir, "a.yaml"), []byte("name: alpha\n"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	// Rename file
	_ = os.Rename(filepath.Join(recDir, "a.yaml"), filepath.Join(recDir, "b.yaml"))
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-m", "rename")

	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{
				"items": {
					ID:      "items",
					DirPath: filepath.Join(dir, "items"),
					RecordFile: &ingitdb.RecordFileDef{
						Name: "{key}.yaml", Format: ingitdb.RecordFormatYAML, RecordType: ingitdb.SingleRecord,
					},
				},
			},
		}, nil
	}

	cmd := Diff(homeDir, getWd, readDef, func(...any) {}, func(int) {})
	if err := runCobraCommand(cmd, "--path="+dir, "HEAD~1..HEAD"); err != nil {
		t.Fatalf("diff renamed: %v", err)
	}
}

type brokenSubFS struct{}

func (brokenSubFS) Open(string) (fs.File, error) { return nil, errors.New("broken fs") }
func (brokenSubFS) Sub(string) (fs.FS, error)     { return nil, errors.New("sub error") }

func TestSkills_ConfigErrors(t *testing.T) {
	// 1. fs.Sub error
	oldFS := skillsFS
	skillsFS = brokenSubFS{}
	defer func() { skillsFS = oldFS }()

	_, err := NewSkillsConfig("1.0.0")
	if err == nil {
		t.Fatal("expected error with brokenSubFS")
	}

	// 2. embeddedBundleFn error
	skillsFS = oldFS
	oldBundleFn := embeddedBundleFn
	embeddedBundleFn = func(d skillsync.BundleDescriptor, content fs.FS) (skillsync.Bundle, error) {
		return skillsync.Bundle{}, errors.New("bundle error")
	}
	defer func() { embeddedBundleFn = oldBundleFn }()

	_, err = NewSkillsConfig("1.0.0")
	if err == nil {
		t.Fatal("expected error with failing embeddedBundleFn")
	}
}

func TestRebase_AbortFails(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init")
	disableGitBackgroundMaintenance(t, dir)
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")
	_ = os.WriteFile(filepath.Join(dir, "other.txt"), []byte("c1\n"), 0o644)
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "initial")

	oldAbort := gitRebaseAbort
	gitRebaseAbort = func(ctx context.Context, wd string) error {
		return errors.New("simulated abort error")
	}
	defer func() { gitRebaseAbort = oldAbort }()

	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{
			Collections: map[string]*ingitdb.CollectionDef{},
		}, nil
	}

	oldDiff := gitConflictedFiles
	gitConflictedFiles = func(ctx context.Context, dirPath string) ([]string, error) {
		return []string{"other.txt"}, nil
	}
	defer func() { gitConflictedFiles = oldDiff }()

	cmd := Rebase(getWd, readDef, func(...any) {})
	err := runCobraCommand(cmd, "--base_ref=nonexistent_branch")
	if err == nil || !strings.Contains(err.Error(), "and 'git rebase --abort' also failed") {
		t.Fatalf("expected abort failure error, got %v", err)
	}
}

type mockSimpleViewBuilderWithErr struct {
	mockViewBuilder
	buildViewErr error
}

func (m *mockSimpleViewBuilderWithErr) BuildView(ctx context.Context, dirPath, repoRoot string, col *ingitdb.CollectionDef, def *ingitdb.Definition, view *ingitdb.ViewDef) (*ingitdb.MaterializeResult, error) {
	if m.buildViewErr != nil {
		return nil, m.buildViewErr
	}
	return m.result, nil
}

func TestMaterialize_Errors(t *testing.T) {
	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {
				ID:      "items",
				DirPath: filepath.Join(dir, "items"),
				Views: map[string]*ingitdb.ViewDef{
					"v1": {ID: "v1"},
				},
			},
		},
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }

	// 1. docsbuilderUpdateDocsFn fails
	oldUpdateDocs := docsbuilderUpdateDocsFn
	docsbuilderUpdateDocsFn = func(ctx context.Context, def *ingitdb.Definition, collectionGlob string, dbPath string, recordsReader ingitdb.RecordsReader) (*ingitdb.MaterializeResult, error) {
		return nil, errors.New("docs error")
	}
	defer func() { docsbuilderUpdateDocsFn = oldUpdateDocs }()

	cmdDocsErr := Materialize(homeDir, getWd, readDef, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, func(...any) {})
	err := runCobraCommand(cmdDocsErr, "--path="+dir, "--collections")
	if err == nil || !strings.Contains(err.Error(), "failed to regenerate collection READMEs") {
		t.Fatalf("expected docs error, got %v", err)
	}

	// 2. BuildView error on subset
	docsbuilderUpdateDocsFn = oldUpdateDocs
	simpleBuilder := &mockSimpleViewBuilderWithErr{
		mockViewBuilder: mockViewBuilder{result: &ingitdb.MaterializeResult{}},
		buildViewErr:    errors.New("build view error"),
	}
	cmdViewErr := Materialize(homeDir, getWd, readDef, simpleBuilder, func(...any) {})
	err = runCobraCommand(cmdViewErr, "--path="+dir, "--views=v1")
	if err == nil || !strings.Contains(err.Error(), "failed to materialize view") {
		t.Fatalf("expected materialize view error, got %v", err)
	}
}

func TestPull_BranchCoverage(t *testing.T) {
	dir := t.TempDir()
	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"items": {ID: "items"},
		},
	}
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	logf := func(...any) {}

	// Seam backups
	oldHeadRef := gitHeadRef
	oldGitPull := runGitPull
	oldConflicted := gitConflictedFiles
	oldDiffFiles := diffFilesFn
	oldResolver := changeSetResolverResolveFn
	oldResolveWorkingTree := resolveWorkingTreeConflictsFn
	defer func() {
		gitHeadRef = oldHeadRef
		runGitPull = oldGitPull
		gitConflictedFiles = oldConflicted
		diffFilesFn = oldDiffFiles
		changeSetResolverResolveFn = oldResolver
		resolveWorkingTreeConflictsFn = oldResolveWorkingTree
	}()

	gitHeadRef = func(ctx context.Context, dirPath string) (string, error) { return "head123", nil }
	runGitPull = func(ctx context.Context, dirPath, strategy, remote, branch string, logf func(...any)) error { return nil }

	// 1. gitConflictedFiles error
	gitConflictedFiles = func(ctx context.Context, dirPath string) ([]string, error) {
		return nil, errors.New("diff conflicted error")
	}
	cmd1 := Pull(homeDir, getWd, func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, logf, func() bool { return false }, nil)
	err := runCobraCommand(cmd1, "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), "failed to check conflicts") {
		t.Fatalf("expected failed to check conflicts, got %v", err)
	}

	// 2. len(conflicted) > 0 with readDefinition error
	gitConflictedFiles = func(ctx context.Context, dirPath string) ([]string, error) {
		return []string{"README.md"}, nil
	}
	readDefErr := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("read def error")
	}
	cmd2 := Pull(homeDir, getWd, readDefErr, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, logf, func() bool { return false }, nil)
	err = runCobraCommand(cmd2, "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), "failed to read database definition") {
		t.Fatalf("expected failed to read database definition, got %v", err)
	}

	// 3. len(conflicted) > 0 with resolveWorkingTreeConflicts error
	readDefOk := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
	resolveWorkingTreeConflictsFn = func(ctx context.Context, dirPath string, def *ingitdb.Definition, conflictedFiles []string, isTerminal func() bool, runConflictsTUI func(context.Context, []string) error, logf func(...any)) error {
		return errors.New("resolve conflict error")
	}
	cmd3 := Pull(homeDir, getWd, readDefOk, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, logf, func() bool { return false }, nil)
	err = runCobraCommand(cmd3, "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), "resolve conflict error") {
		t.Fatalf("expected resolve conflict error, got %v", err)
	}
	resolveWorkingTreeConflictsFn = resolveWorkingTreeConflicts

	// 4. readDefinition error after pull (no conflicts)
	gitConflictedFiles = func(ctx context.Context, dirPath string) ([]string, error) { return nil, nil }
	readDefSeq := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return nil, errors.New("def err after pull")
	}
	cmd4 := Pull(homeDir, getWd, readDefSeq, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, logf, func() bool { return false }, nil)
	err = runCobraCommand(cmd4, "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), "failed to read database definition") {
		t.Fatalf("expected def err after pull, got %v", err)
	}

	// 5. rebuildViews error
	builderErr := &mockViewBuilder{err: errors.New("rebuild views err")}
	cmd5 := Pull(homeDir, getWd, readDefOk, builderErr, logf, func() bool { return false }, nil)
	err = runCobraCommand(cmd5, "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), "failed to rebuild views") {
		t.Fatalf("expected rebuild views error, got %v", err)
	}

	// 6. summarizeRecordChanges error (logged and returns nil)
	diffFilesFn = func(ctx context.Context, dirPath, from, to string) ([]ingitdb.ChangedFile, error) {
		return nil, errors.New("diff error")
	}
	cmd6 := Pull(homeDir, getWd, readDefOk, &mockViewBuilder{result: &ingitdb.MaterializeResult{}}, logf, func() bool { return false }, nil)
	err = runCobraCommand(cmd6, "--path="+dir)
	if err != nil {
		t.Fatalf("expected success with logged error, got %v", err)
	}

	// 7. summarizeRecordChanges with resolver error
	diffFilesFn = func(ctx context.Context, dirPath, from, to string) ([]ingitdb.ChangedFile, error) {
		return []ingitdb.ChangedFile{{Path: "items/a.yaml"}}, nil
	}
	changeSetResolverResolveFn = func(dirPath string, def *ingitdb.Definition, changed []ingitdb.ChangedFile) ([]datavalidator.AffectedRecord, error) {
		return nil, errors.New("resolver error")
	}
	_, err = summarizeRecordChanges(context.Background(), dir, def, "head123")
	if err == nil || !strings.Contains(err.Error(), "resolver error") {
		t.Fatalf("expected resolver error, got %v", err)
	}

	// 8. summarizeRecordChanges with deleted records
	changeSetResolverResolveFn = func(dirPath string, def *ingitdb.Definition, changed []ingitdb.ChangedFile) ([]datavalidator.AffectedRecord, error) {
		return []datavalidator.AffectedRecord{
			{ChangeKind: ingitdb.ChangeKindDeleted},
		}, nil
	}
	summary, err := summarizeRecordChanges(context.Background(), dir, def, "head123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(summary, "1 deleted") {
		t.Fatalf("expected '1 deleted', got %q", summary)
	}
}
