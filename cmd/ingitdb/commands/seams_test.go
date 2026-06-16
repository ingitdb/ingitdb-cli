package commands

import (
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ingitdb/dalgo2ingitdb4github"
	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// TestGitHubDBFactory_WithMock demonstrates how to use mocks with package-level variables.
// This test DOES NOT call t.Parallel() because it modifies package-level variables.
func TestGitHubDBFactory_WithMock(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore it after test
	original := gitHubDBFactory
	defer func() {
		gitHubDBFactory = original
	}()

	// Create mock
	mockFactory := NewMockGitHubDBFactory(ctrl)
	gitHubDBFactory = mockFactory

	// Set expectations
	cfg := dalgo2ghingitdb.Config{Owner: "test", Repo: "repo"}
	def := &ingitdb.Definition{}
	expectedErr := errors.New("test error")

	mockFactory.EXPECT().
		NewGitHubDBWithDef(cfg, def).
		Return(nil, expectedErr)

	// Call the function that uses the package-level variable
	_, err := gitHubDBFactory.NewGitHubDBWithDef(cfg, def)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestGitHubFileReaderFactory_WithMock demonstrates how to use mocks with package-level variables.
// This test DOES NOT call t.Parallel() because it modifies package-level variables.
func TestGitHubFileReaderFactory_WithMock(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore it after test
	original := gitHubFileReaderFactory
	defer func() {
		gitHubFileReaderFactory = original
	}()

	// Create mock
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	gitHubFileReaderFactory = mockFactory

	// Set expectations
	cfg := dalgo2ghingitdb.Config{Owner: "test", Repo: "repo"}
	expectedErr := errors.New("test error")

	mockFactory.EXPECT().
		NewGitHubFileReader(cfg).
		Return(nil, expectedErr)

	// Call the function that uses the package-level variable
	_, err := gitHubFileReaderFactory.NewGitHubFileReader(cfg)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestViewBuilderFactory_WithMock demonstrates how to use mocks with package-level variables.
// This test DOES NOT call t.Parallel() because it modifies package-level variables.
func TestViewBuilderFactory_WithMock(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore it after test
	original := viewBuilderFactory
	defer func() {
		viewBuilderFactory = original
	}()

	// Create mock
	mockFactory := NewMockViewBuilderFactory(ctrl)
	viewBuilderFactory = mockFactory

	// Set expectations
	colDef := &ingitdb.CollectionDef{ID: "test.collection"}
	expectedErr := errors.New("test error")

	mockFactory.EXPECT().
		ViewBuilderForCollection(colDef).
		Return(nil, expectedErr)

	// Call the function that uses the package-level variable
	_, err := viewBuilderFactory.ViewBuilderForCollection(colDef)
	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

// TestDefaultFactories_RealImplementations tests that default factories work correctly.
// This test CAN run in parallel because it doesn't modify package-level variables.
func TestDefaultFactories_RealImplementations(t *testing.T) {
	t.Parallel()

	// Test that default factories are not nil
	if gitHubDBFactory == nil {
		t.Error("gitHubDBFactory is nil")
	}
	if gitHubFileReaderFactory == nil {
		t.Error("gitHubFileReaderFactory is nil")
	}
	if viewBuilderFactory == nil {
		t.Error("viewBuilderFactory is nil")
	}

	// Test that we can call the default implementations
	// (though they will fail without proper setup)
	_, _ = gitHubDBFactory.NewGitHubDBWithDef(dalgo2ghingitdb.Config{}, nil)
	_, _ = gitHubFileReaderFactory.NewGitHubFileReader(dalgo2ghingitdb.Config{})

	// ViewBuilder can be tested with nil
	builder, err := viewBuilderFactory.ViewBuilderForCollection(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if builder != nil {
		t.Error("expected nil builder for nil collection")
	}
}

// Compile-time interface checks
var (
	_ GitHubDBFactory         = (*defaultGitHubDBFactory)(nil)
	_ GitHubFileReaderFactory = (*defaultGitHubFileReaderFactory)(nil)
	_ ViewBuilderFactory      = (*defaultViewBuilderFactory)(nil)
)
