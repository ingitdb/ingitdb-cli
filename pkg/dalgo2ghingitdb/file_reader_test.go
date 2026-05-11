package dalgo2ghingitdb

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-github/v86/github"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		cfg       Config
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid config",
			cfg:       Config{Owner: "test", Repo: "test"},
			wantError: false,
		},
		{
			name:      "missing owner",
			cfg:       Config{Repo: "test"},
			wantError: true,
			errorMsg:  "owner is required",
		},
		{
			name:      "missing repo",
			cfg:       Config{Owner: "test"},
			wantError: true,
			errorMsg:  "repo is required",
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.cfg.validate()
			if tc.wantError {
				if err == nil {
					t.Fatal("validate() expected error, got nil")
				}
				if tc.errorMsg != "" && err.Error() != tc.errorMsg {
					t.Errorf("validate() error = %q, want %q", err.Error(), tc.errorMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNewGitHubFileReader_WithToken(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test", Token: "test-token"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}
	if reader == nil {
		t.Fatal("NewGitHubFileReader returned nil reader")
	}
}

func TestNewGitHubFileReader_WithCustomBaseURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}
	if reader == nil {
		t.Fatal("NewGitHubFileReader returned nil reader")
	}
}

func TestNewGitHubFileReader_InvalidBaseURL(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: "://invalid"}
	_, err := NewGitHubFileReader(cfg)
	if err == nil {
		t.Fatal("NewGitHubFileReader() expected error for invalid URL, got nil")
	}
}

func TestGitHubFileReader_ReadFile_PathIsDirectory(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := []map[string]any{
			{"name": "file1.txt", "type": "file"},
		}
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	_, _, err = reader.ReadFile(ctx, "test/dir")
	if err == nil {
		t.Fatal("ReadFile() expected error for directory path, got nil")
	}
	expectedMsg := "path is not a file: test/dir"
	if err.Error() != expectedMsg {
		t.Errorf("ReadFile() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestGitHubFileReader_ReadFile_WithRef(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test.txt",
		content: "test content",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "develop", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	content, found, err := reader.ReadFile(ctx, "test-ingitdb/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !found {
		t.Fatal("ReadFile() expected found=true")
	}
	if string(content) != "test content" {
		t.Errorf("ReadFile() content = %q, want %q", string(content), "test content")
	}
}

func TestGitHubFileReader_ListDirectory_NotFound(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	entries, err := reader.ListDirectory(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if entries != nil {
		t.Errorf("ListDirectory() entries = %v, want nil", entries)
	}
}

func TestGitHubFileReader_ListDirectory_WithRef(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:     "test-ingitdb/test-dir",
		isDir:    true,
		dirItems: []string{"file1.txt", "file2.txt"},
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "develop", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	entries, err := reader.ListDirectory(ctx, "test-ingitdb/test-dir")
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("ListDirectory() expected 2 entries, got %d", len(entries))
	}
}

func TestWrapGitHubError_RateLimitError(t *testing.T) {
	t.Parallel()
	origErr := &github.RateLimitError{
		Rate: github.Rate{Limit: 60, Remaining: 0},
	}
	wrappedErr := wrapGitHubError("test/path", origErr, nil)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "rate limit exceeded") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'rate limit exceeded'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestWrapGitHubError_AbuseRateLimitError(t *testing.T) {
	t.Parallel()
	origErr := &github.AbuseRateLimitError{
		Message: "abuse detected",
	}
	wrappedErr := wrapGitHubError("test/path", origErr, nil)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "secondary rate limit") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'secondary rate limit'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestWrapGitHubError_ForbiddenError(t *testing.T) {
	t.Parallel()
	origErr := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusForbidden},
		Message:  "forbidden",
	}
	wrappedErr := wrapGitHubError("test/path", origErr, nil)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "forbidden") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'forbidden'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestWrapGitHubError_ErrorResponseWithStatus(t *testing.T) {
	t.Parallel()
	origErr := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusBadRequest},
		Message:  "bad request",
	}
	wrappedErr := wrapGitHubError("test/path", origErr, nil)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "status 400") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'status 400'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestWrapGitHubError_WithResponse(t *testing.T) {
	t.Parallel()
	resp := &github.Response{
		Response: &http.Response{StatusCode: http.StatusInternalServerError},
	}
	origErr := fmt.Errorf("protocol error")
	wrappedErr := wrapGitHubError("test/path", origErr, resp)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "status 500") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'status 500'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestWrapGitHubError_GenericError(t *testing.T) {
	t.Parallel()
	origErr := http.ErrBodyNotAllowed
	wrappedErr := wrapGitHubError("test/path", origErr, nil)
	if wrappedErr == nil {
		t.Fatal("wrapGitHubError() returned nil")
	}
	errMsg := wrappedErr.Error()
	if !strings.Contains(errMsg, "request failed") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'request failed'", errMsg)
	}
	if !strings.Contains(errMsg, "test/path") {
		t.Errorf("wrapGitHubError() error = %q, want to contain 'test/path'", errMsg)
	}
}

func TestIsGitHubNotFound_ErrorResponse(t *testing.T) {
	t.Parallel()
	err := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}
	if !isGitHubNotFound(err, nil) {
		t.Error("isGitHubNotFound() = false, want true for 404 ErrorResponse")
	}
}

func TestIsGitHubNotFound_ResponseOnly(t *testing.T) {
	t.Parallel()
	resp := &github.Response{
		Response: &http.Response{StatusCode: http.StatusNotFound},
	}
	if !isGitHubNotFound(http.ErrBodyNotAllowed, resp) {
		t.Error("isGitHubNotFound() = false, want true for 404 Response")
	}
}

func TestIsGitHubNotFound_NotFoundStatus(t *testing.T) {
	t.Parallel()
	err := &github.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusOK},
	}
	if isGitHubNotFound(err, nil) {
		t.Error("isGitHubNotFound() = true, want false for non-404 status")
	}
}

func TestGitHubFileReader_ReadFileWithSHA_PathIsDirectory(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := []map[string]any{
			{"name": "file1.txt", "type": "file"},
		}
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	_, _, _, err = concrete.readFileWithSHA(ctx, "test/dir")
	if err == nil {
		t.Fatal("readFileWithSHA() expected error for directory path, got nil")
	}
	expectedMsg := "path is not a file: test/dir"
	if err.Error() != expectedMsg {
		t.Errorf("readFileWithSHA() error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestGitHubFileReader_ReadFileWithSHA_DecodeError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  "!!invalid-base64!!",
			"sha":      "abc123",
		}
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	_, _, _, err = concrete.readFileWithSHA(ctx, "test/file.txt")
	if err == nil {
		t.Fatal("readFileWithSHA() expected error for invalid base64, got nil")
	}
}

func TestGitHubFileReader_WriteFile_CreateFile(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.writeFile(ctx, "test-ingitdb/test/new.txt", "create file", []byte("content"), "")
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func TestGitHubFileReader_WriteFile_UpdateFile(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/existing.txt",
		content: "original content",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.writeFile(ctx, "test-ingitdb/test/existing.txt", "update file", []byte("new content"), "abc123def456")
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func TestGitHubFileReader_DeleteFile(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/delete.txt",
		content: "to be deleted",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.deleteFile(ctx, "test-ingitdb/test/delete.txt", "delete file", "abc123def456")
	if err != nil {
		t.Fatalf("deleteFile: %v", err)
	}
}

func TestGitHubFileReader_ReadFile_DecodeContentError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  "!!invalid-base64!!",
			"sha":      "abc123",
		}
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	_, _, err = reader.ReadFile(ctx, "test/file.txt")
	if err == nil {
		t.Fatal("ReadFile() expected error for invalid base64, got nil")
	}
}

func TestGitHubFileReader_ReadFileWithSHA_WithRef(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test.txt",
		content: "test content",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "develop", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	content, sha, found, err := concrete.readFileWithSHA(ctx, "test-ingitdb/test.txt")
	if err != nil {
		t.Fatalf("readFileWithSHA: %v", err)
	}
	if !found {
		t.Fatal("readFileWithSHA() expected found=true")
	}
	if string(content) != "test content" {
		t.Errorf("readFileWithSHA() content = %q, want %q", string(content), "test content")
	}
	if sha != "abc123def456" {
		t.Errorf("readFileWithSHA() sha = %q, want %q", sha, "abc123def456")
	}
}

func TestGitHubFileReader_WriteFile_WithRef(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "develop", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.writeFile(ctx, "test-ingitdb/test/new.txt", "create file", []byte("content"), "")
	if err != nil {
		t.Fatalf("writeFile: %v", err)
	}
}

func TestGitHubFileReader_DeleteFile_WithRef(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test/delete.txt",
		content: "to be deleted",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", Ref: "develop", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.deleteFile(ctx, "test-ingitdb/test/delete.txt", "delete file", "abc123def456")
	if err != nil {
		t.Fatalf("deleteFile: %v", err)
	}
}

func TestGitHubFileReader_WriteFile_APIError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.writeFile(ctx, "test/new.txt", "create file", []byte("content"), "")
	if err == nil {
		t.Fatal("writeFile() expected error for API error, got nil")
	}
}

func TestGitHubFileReader_DeleteFile_APIError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		http.NotFound(w, r)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	err = concrete.deleteFile(ctx, "test/delete.txt", "delete file", "abc123")
	if err == nil {
		t.Fatal("deleteFile() expected error for API error, got nil")
	}
}

func TestGitHubFileReader_ReadFile_APIError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	_, _, err = reader.ReadFile(ctx, "test/file.txt")
	if err == nil {
		t.Fatal("ReadFile() expected error for API error, got nil")
	}
}

func TestGitHubFileReader_ListDirectory_APIError(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	_, err = reader.ListDirectory(ctx, "test/dir")
	if err == nil {
		t.Fatal("ListDirectory() expected error for API error, got nil")
	}
}

func TestGitHubFileReader_ReadFileWithSHA_NotFound(t *testing.T) {
	t.Parallel()
	server := newGitHubContentsServer(t, nil)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	concrete, ok := reader.(*githubFileReader)
	if !ok {
		t.Fatal("reader is not *githubFileReader")
	}

	ctx := context.Background()
	content, sha, found, err := concrete.readFileWithSHA(ctx, "nonexistent.txt")
	if err != nil {
		t.Fatalf("readFileWithSHA: %v", err)
	}
	if found {
		t.Fatal("readFileWithSHA() expected found=false")
	}
	if content != nil {
		t.Errorf("readFileWithSHA() content = %v, want nil", content)
	}
	if sha != "" {
		t.Errorf("readFileWithSHA() sha = %q, want empty", sha)
	}
}

func TestGitHubFileReader_ReadFile_LeadingSlash(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:    "test-ingitdb/test.txt",
		content: "test content",
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	content, found, err := reader.ReadFile(ctx, "/test-ingitdb/test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !found {
		t.Fatal("ReadFile() expected found=true")
	}
	if string(content) != "test content" {
		t.Errorf("ReadFile() content = %q, want %q", string(content), "test content")
	}
}

func TestGitHubFileReader_ListDirectory_LeadingSlash(t *testing.T) {
	t.Parallel()
	fixtures := []githubFileFixture{{
		path:     "test-ingitdb/test-dir",
		isDir:    true,
		dirItems: []string{"file1.txt"},
	}}
	server := newGitHubContentsServer(t, fixtures)
	defer server.Close()

	cfg := Config{Owner: "ingitdb", Repo: "ingitdb-cli", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	entries, err := reader.ListDirectory(ctx, "/test-ingitdb/test-dir")
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ListDirectory() expected 1 entry, got %d", len(entries))
	}
}

func TestGitHubServer_ReadFile_ValidBase64(t *testing.T) {
	t.Parallel()
	content := "test content with special chars: 你好"
	encoded := base64.StdEncoding.EncodeToString([]byte(content))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		response := map[string]any{
			"type":     "file",
			"encoding": "base64",
			"content":  encoded,
			"sha":      "abc123",
		}
		encodeErr := json.NewEncoder(w).Encode(response)
		if encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	cfg := Config{Owner: "test", Repo: "test", APIBaseURL: server.URL + "/"}
	reader, err := NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader: %v", err)
	}

	ctx := context.Background()
	result, found, err := reader.ReadFile(ctx, "test.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !found {
		t.Fatal("ReadFile() expected found=true")
	}
	if string(result) != content {
		t.Errorf("ReadFile() content = %q, want %q", string(result), content)
	}
}
