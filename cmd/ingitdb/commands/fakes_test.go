package commands

import (
	"context"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
)

// fakeFileReader is an in-memory FileReader stub used by tests that
// want to exercise the GitHub-source code path without making a real
// network request. files maps file paths to their bytes; directories
// maps directory paths to their entries.
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

var _ dalgo2ghingitdb.FileReader = fakeFileReader{}

// fakeFileReaderWithError is a FileReader stub that returns a fixed
// error from every read/list call.
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
