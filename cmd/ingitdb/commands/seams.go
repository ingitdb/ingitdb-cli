package commands

import (
	"context"

	"github.com/dal-go/dalgo/dal"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

// GitHubDBFactory creates a GitHub-backed DAL database.
type GitHubDBFactory interface {
	NewGitHubDBWithDef(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error)
}

// GitHubFileReaderFactory creates a GitHub file reader.
type GitHubFileReaderFactory interface {
	NewGitHubFileReader(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error)
}

// ViewBuilderFactory creates a view builder for a collection.
type ViewBuilderFactory interface {
	ViewBuilderForCollection(colDef *ingitdb.CollectionDef) (materializer.ViewBuilder, error)
}

// treeWriter is the subset of dalgo2ghingitdb.TreeWriter that the remote
// drop path depends on. Defined here so tests can substitute fake
// implementations without spinning up a full GitHub mock server.
type treeWriter interface {
	ListFilesUnder(ctx context.Context, dir string) ([]string, error)
	CommitChanges(ctx context.Context, message string, changes []dalgo2ghingitdb.TreeChange) (string, error)
}

// TreeWriterFactory creates a treeWriter for atomic multi-file commits.
type TreeWriterFactory interface {
	NewTreeWriter(cfg dalgo2ghingitdb.Config) (treeWriter, error)
}

// Package-level variables for testing seams.
// These can be replaced with mocks in tests.
// Tests that replace these variables MUST NOT run in parallel.
var (
	// gitHubDBFactory is the factory for creating GitHub-backed databases.
	gitHubDBFactory GitHubDBFactory = &defaultGitHubDBFactory{}

	// gitHubFileReaderFactory is the factory for creating GitHub file readers.
	gitHubFileReaderFactory GitHubFileReaderFactory = &defaultGitHubFileReaderFactory{}

	// viewBuilderFactory is the factory for creating view builders.
	viewBuilderFactory ViewBuilderFactory = &defaultViewBuilderFactory{}

	// treeWriterFactory is the factory for creating atomic multi-file writers.
	treeWriterFactory TreeWriterFactory = &defaultTreeWriterFactory{}
)

// defaultGitHubDBFactory is the default implementation of GitHubDBFactory.
type defaultGitHubDBFactory struct{}

func (f *defaultGitHubDBFactory) NewGitHubDBWithDef(cfg dalgo2ghingitdb.Config, def *ingitdb.Definition) (dal.DB, error) {
	return dalgo2ghingitdb.NewGitHubDBWithDef(cfg, def)
}

// defaultGitHubFileReaderFactory is the default implementation of GitHubFileReaderFactory.
type defaultGitHubFileReaderFactory struct{}

func (f *defaultGitHubFileReaderFactory) NewGitHubFileReader(cfg dalgo2ghingitdb.Config) (dalgo2ghingitdb.FileReader, error) {
	return dalgo2ghingitdb.NewGitHubFileReader(cfg)
}

// defaultViewBuilderFactory is the default implementation of ViewBuilderFactory.
type defaultViewBuilderFactory struct{}

func (f *defaultViewBuilderFactory) ViewBuilderForCollection(colDef *ingitdb.CollectionDef) (materializer.ViewBuilder, error) {
	return viewBuilderForCollection(colDef)
}

// defaultTreeWriterFactory is the default implementation of TreeWriterFactory.
type defaultTreeWriterFactory struct{}

func (f *defaultTreeWriterFactory) NewTreeWriter(cfg dalgo2ghingitdb.Config) (treeWriter, error) {
	return dalgo2ghingitdb.NewTreeWriter(cfg)
}
