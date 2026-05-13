package dalgo2ghingitdb

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-github/v86/github"
)

// TreeChange describes a single path modification within a tree. Content nil
// means "delete the file at Path"; non-nil Content means "set the file at Path
// to this content" (creating it if absent).
type TreeChange struct {
	Path    string
	Content []byte
}

// TreeWriter performs atomic multi-file commits on a remote GitHub repository
// via the Git Data API (blobs / trees / commits / refs). Unlike the per-file
// Contents API used by FileReader.writeFile / deleteFile (which each produce
// their own commit), TreeWriter bundles arbitrary file modifications into a
// single commit — satisfying spec REQ:one-commit-per-write for multi-file
// operations such as `drop collection`.
type TreeWriter struct {
	cfg    Config
	client *github.Client
}

// NewTreeWriter builds a TreeWriter for the given Config. It does not perform
// any network I/O; the first request fires on the first method call.
func NewTreeWriter(cfg Config) (*TreeWriter, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	client := github.NewClient(httpClient)
	if cfg.Token != "" {
		client = client.WithAuthToken(cfg.Token)
	}
	if cfg.APIBaseURL != "" {
		baseURL := cfg.APIBaseURL
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		parsedURL, parseErr := url.Parse(baseURL)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid api base url: %w", parseErr)
		}
		client.BaseURL = parsedURL
		client.UploadURL = parsedURL
	}
	return &TreeWriter{cfg: cfg, client: client}, nil
}

// ListFilesUnder returns the paths of every blob in the current tree of the
// target branch whose path equals dir or has dir+"/" as a prefix. Empty dir
// returns every blob in the tree. Returned paths are relative to the repo
// root. Used to enumerate the files to delete when dropping a collection.
//
// Errors if the upstream tree is truncated (too large to fetch in one call);
// drop semantics require an atomic view of the directory.
func (w *TreeWriter) ListFilesUnder(ctx context.Context, dir string) ([]string, error) {
	treeSHA, _, err := w.headTree(ctx)
	if err != nil {
		return nil, err
	}
	tree, _, treeErr := w.client.Git.GetTree(ctx, w.cfg.Owner, w.cfg.Repo, treeSHA, true)
	if treeErr != nil {
		return nil, wrapGitHubError(treeSHA, treeErr, nil)
	}
	if tree.GetTruncated() {
		return nil, fmt.Errorf("repository tree at %s is truncated; ingitdb does not support atomic operations on trees larger than the github api limit", treeSHA)
	}
	cleanDir := strings.Trim(dir, "/")
	prefix := cleanDir + "/"
	paths := make([]string, 0)
	for _, entry := range tree.Entries {
		if entry.GetType() != "blob" {
			continue
		}
		path := entry.GetPath()
		if cleanDir == "" || path == cleanDir || strings.HasPrefix(path, prefix) {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

// CommitChanges atomically applies the given changes in a single commit on the
// target branch and returns the new commit SHA. The repository ends in
// exactly one of two states: either every change is applied as part of the
// new commit, or no change is applied at all (the previous head is unchanged).
//
// Changes with nil Content delete the path; non-nil Content creates or
// updates a blob at the path with that content. Mode is set to "100644"
// (non-executable file) for all non-delete entries.
func (w *TreeWriter) CommitChanges(ctx context.Context, message string, changes []TreeChange) (string, error) {
	if len(changes) == 0 {
		return "", errors.New("no changes to commit")
	}
	if message == "" {
		return "", errors.New("commit message is required")
	}
	branch, err := w.resolveBranch(ctx)
	if err != nil {
		return "", err
	}
	baseTreeSHA, headCommitSHA, err := w.headTreeForBranch(ctx, branch)
	if err != nil {
		return "", err
	}

	entries := make([]*github.TreeEntry, 0, len(changes))
	for _, ch := range changes {
		path := ch.Path
		entry := &github.TreeEntry{Path: &path}
		if ch.Content != nil {
			mode := "100644"
			typeBlob := "blob"
			content := string(ch.Content)
			entry.Mode = &mode
			entry.Type = &typeBlob
			entry.Content = &content
		}
		// For deletions only Path is set; GitHub interprets the missing
		// sha + mode + type as "remove this path from the base tree".
		entries = append(entries, entry)
	}

	newTree, _, treeErr := w.client.Git.CreateTree(ctx, w.cfg.Owner, w.cfg.Repo, baseTreeSHA, entries)
	if treeErr != nil {
		return "", wrapGitHubError("CreateTree", treeErr, nil)
	}

	parents := []*github.Commit{{SHA: &headCommitSHA}}
	commitInput := github.Commit{
		Message: &message,
		Tree:    &github.Tree{SHA: newTree.SHA},
		Parents: parents,
	}
	newCommit, _, commitErr := w.client.Git.CreateCommit(ctx, w.cfg.Owner, w.cfg.Repo, commitInput, nil)
	if commitErr != nil {
		return "", wrapGitHubError("CreateCommit", commitErr, nil)
	}

	newSHA := newCommit.GetSHA()
	refPath := "heads/" + branch
	if _, _, refErr := w.client.Git.UpdateRef(ctx, w.cfg.Owner, w.cfg.Repo, refPath, github.UpdateRef{SHA: newSHA}); refErr != nil {
		return "", wrapGitHubError("UpdateRef "+refPath, refErr, nil)
	}
	return newSHA, nil
}

// resolveBranch returns cfg.Ref if non-empty (treated as a branch name; tags
// and commit SHAs are out of scope for write operations) or the repository's
// default branch otherwise.
func (w *TreeWriter) resolveBranch(ctx context.Context) (string, error) {
	if w.cfg.Ref != "" {
		return w.cfg.Ref, nil
	}
	repo, _, err := w.client.Repositories.Get(ctx, w.cfg.Owner, w.cfg.Repo)
	if err != nil {
		return "", wrapGitHubError("Repositories.Get", err, nil)
	}
	branch := repo.GetDefaultBranch()
	if branch == "" {
		return "", fmt.Errorf("repository %s/%s has no default branch", w.cfg.Owner, w.cfg.Repo)
	}
	return branch, nil
}

// headTree returns the tree SHA + commit SHA at the head of the target branch.
func (w *TreeWriter) headTree(ctx context.Context) (treeSHA, commitSHA string, err error) {
	branch, err := w.resolveBranch(ctx)
	if err != nil {
		return "", "", err
	}
	return w.headTreeForBranch(ctx, branch)
}

func (w *TreeWriter) headTreeForBranch(ctx context.Context, branch string) (treeSHA, commitSHA string, err error) {
	ref, _, refErr := w.client.Git.GetRef(ctx, w.cfg.Owner, w.cfg.Repo, "heads/"+branch)
	if refErr != nil {
		return "", "", wrapGitHubError("GetRef heads/"+branch, refErr, nil)
	}
	commitSHA = ref.GetObject().GetSHA()
	commit, _, commitErr := w.client.Git.GetCommit(ctx, w.cfg.Owner, w.cfg.Repo, commitSHA)
	if commitErr != nil {
		return "", "", wrapGitHubError("GetCommit "+commitSHA, commitErr, nil)
	}
	return commit.GetTree().GetSHA(), commitSHA, nil
}
