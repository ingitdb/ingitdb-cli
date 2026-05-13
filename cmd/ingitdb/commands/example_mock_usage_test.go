package commands

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ghingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// TestCreateRecord_GitHubDBError demonstrates testing error handling when GitHub DB factory fails.
// This test does NOT call t.Parallel() because it modifies package-level variables.
func TestCreateRecord_GitHubDBError(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore after test
	originalGitHubDB := gitHubDBFactory
	defer func() {
		gitHubDBFactory = originalGitHubDB
	}()

	// Install mock
	mockGitHubDB := NewMockGitHubDBFactory(ctrl)
	gitHubDBFactory = mockGitHubDB

	// Set up expectation: GitHub DB factory fails
	expectedErr := errors.New("github connection failed")
	mockGitHubDB.EXPECT().
		NewGitHubDBWithDef(gomock.Any(), gomock.Any()).
		Return(nil, expectedErr)

	// Directly test that the mock works as expected
	_, err := gitHubDBFactory.NewGitHubDBWithDef(
		// Pass dummy values - in real code these come from command flags
		dalgo2ghingitdb.Config{Owner: "test", Repo: "repo"},
		nil,
	)

	if err != expectedErr {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}

	// In a real integration test, you would:
	// 1. Create a complete CLI command with proper flags
	// 2. Run it with the --remote flag to trigger the remote code path
	// 3. Assert that the error returned matches expectedErr
}

// TestListCollections_GitHubFileReaderError demonstrates testing error handling when file reader fails.
// This test does NOT call t.Parallel() because it modifies package-level variables.
func TestListCollections_GitHubFileReaderError(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore after test
	original := gitHubFileReaderFactory
	defer func() {
		gitHubFileReaderFactory = original
	}()

	// Install mock
	mockFactory := NewMockGitHubFileReaderFactory(ctrl)
	gitHubFileReaderFactory = mockFactory

	// Set up expectation: file reader creation fails
	expectedErr := errors.New("github api error")
	mockFactory.EXPECT().
		NewGitHubFileReader(gomock.Any()).
		Return(nil, expectedErr)

	// Test the function that uses gitHubFileReaderFactory
	ctx := context.Background()
	spec := remoteSpec{Host: "github.com", Path: []string{"owner", "repo"}}
	err := listCollectionsRemoteWithSpec(ctx, spec, "fake-token")

	if err == nil {
		t.Fatal("expected error when file reader creation fails")
	}
	// The error should be wrapped, but should contain our original error
	if !errors.Is(err, expectedErr) {
		// Check if error message contains our error
		if err.Error() != "failed to create github file reader: github api error" {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

// TestViewBuilder_MockedForLocalOperations demonstrates mocking view builder.
// This test does NOT call t.Parallel() because it modifies package-level variables.
func TestViewBuilder_MockedForLocalOperations(t *testing.T) {
	// NOTE: No t.Parallel() call here because we're modifying package-level variables

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Save original value and restore after test
	original := viewBuilderFactory
	defer func() {
		viewBuilderFactory = original
	}()

	// Install mock
	mockFactory := NewMockViewBuilderFactory(ctrl)
	viewBuilderFactory = mockFactory

	// Set up expectation: view builder returns nil (no views)
	mockFactory.EXPECT().
		ViewBuilderForCollection(gomock.Any()).
		Return(nil, nil)

	// Call the function
	builder, err := viewBuilderFactory.ViewBuilderForCollection(&ingitdb.CollectionDef{
		ID: "test.collection",
	})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if builder != nil {
		t.Error("expected nil builder")
	}
}
