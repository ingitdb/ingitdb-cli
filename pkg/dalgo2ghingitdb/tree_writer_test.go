package dalgo2ghingitdb

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// treeMockServer returns an httptest server that responds to the
// Git Data API endpoints used by TreeWriter. Each handler is selected
// by URL prefix to keep the test wiring explicit.
//
// The endpoints recorded into `seen` so tests can assert that every
// expected request fired in the right order.
type treeMockServer struct {
	t             *testing.T
	defaultBranch string
	headRef       string // commit SHA at heads/<defaultBranch>
	headCommit    *githubCommit
	tree          *githubTree
	// captured request bodies by endpoint name
	createdTrees   []map[string]any
	createdCommits []map[string]any
	updatedRefs    []map[string]any
}

type githubCommit struct {
	SHA  string
	Tree string
}

type githubTreeEntry struct {
	Path string
	Mode string
	Type string
	SHA  string
}

type githubTree struct {
	SHA       string
	Truncated bool
	Entries   []githubTreeEntry
}

func (m *treeMockServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/git/ref/heads/"+m.defaultBranch):
			respond(w, map[string]any{
				"ref":    "refs/heads/" + m.defaultBranch,
				"object": map[string]any{"sha": m.headRef, "type": "commit"},
			})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/git/commits/"+m.headRef):
			respond(w, map[string]any{
				"sha":  m.headCommit.SHA,
				"tree": map[string]any{"sha": m.headCommit.Tree},
			})

		case r.Method == "GET" && strings.Contains(r.URL.Path, "/git/trees/"+m.tree.SHA):
			entries := make([]map[string]any, 0, len(m.tree.Entries))
			for _, e := range m.tree.Entries {
				entries = append(entries, map[string]any{
					"path": e.Path,
					"mode": e.Mode,
					"type": e.Type,
					"sha":  e.SHA,
				})
			}
			respond(w, map[string]any{
				"sha":       m.tree.SHA,
				"tree":      entries,
				"truncated": m.tree.Truncated,
			})

		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/git/trees"):
			body := decodeBody(m.t, r)
			m.createdTrees = append(m.createdTrees, body)
			respond(w, map[string]any{"sha": "new-tree-sha"})

		case r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/git/commits"):
			body := decodeBody(m.t, r)
			m.createdCommits = append(m.createdCommits, body)
			respond(w, map[string]any{"sha": "new-commit-sha"})

		case r.Method == "PATCH" && strings.Contains(r.URL.Path, "/git/refs/heads/"+m.defaultBranch):
			body := decodeBody(m.t, r)
			m.updatedRefs = append(m.updatedRefs, body)
			respond(w, map[string]any{
				"ref":    "refs/heads/" + m.defaultBranch,
				"object": map[string]any{"sha": body["sha"]},
			})

		case r.Method == "GET" && strings.HasSuffix(r.URL.Path, "/repos/owner/repo"):
			// Repositories.Get for default-branch lookup.
			respond(w, map[string]any{
				"name":           "repo",
				"default_branch": m.defaultBranch,
			})

		default:
			http.Error(w, "unexpected request: "+r.Method+" "+r.URL.Path, http.StatusNotImplemented)
		}
	})
}

func respond(w http.ResponseWriter, body any) {
	_ = json.NewEncoder(w).Encode(body)
}

func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("decode request body: %v", err)
	}
	return body
}

func newTestTreeWriter(t *testing.T, srv *httptest.Server, ref string) *TreeWriter {
	t.Helper()
	cfg := Config{
		Owner:      "owner",
		Repo:       "repo",
		Ref:        ref,
		APIBaseURL: srv.URL + "/",
	}
	w, err := NewTreeWriter(cfg)
	if err != nil {
		t.Fatalf("NewTreeWriter: %v", err)
	}
	return w
}

func TestTreeWriter_CommitChanges_DeletesAndModifications(t *testing.T) {
	t.Parallel()
	mock := &treeMockServer{
		t:             t,
		defaultBranch: "main",
		headRef:       "head-commit-sha",
		headCommit:    &githubCommit{SHA: "head-commit-sha", Tree: "base-tree-sha"},
		tree: &githubTree{SHA: "base-tree-sha", Entries: []githubTreeEntry{
			{Path: "data/cities/ie.yaml", Mode: "100644", Type: "blob", SHA: "blob1"},
		}},
	}
	srv := httptest.NewServer(mock.handler())
	defer srv.Close()

	w := newTestTreeWriter(t, srv, "main")
	newSHA, err := w.CommitChanges(context.Background(), "drop collection cities", []TreeChange{
		{Path: "data/cities/ie.yaml"}, // delete
		{Path: ".ingitdb/root-collections.yaml", Content: []byte("countries: data/countries\n")}, // modify
	})
	if err != nil {
		t.Fatalf("CommitChanges: %v", err)
	}
	if newSHA != "new-commit-sha" {
		t.Errorf("newSHA = %q, want new-commit-sha", newSHA)
	}
	if len(mock.createdTrees) != 1 {
		t.Fatalf("expected 1 CreateTree call, got %d", len(mock.createdTrees))
	}
	body := mock.createdTrees[0]
	if body["base_tree"] != "base-tree-sha" {
		t.Errorf("base_tree = %v, want base-tree-sha", body["base_tree"])
	}
	entries, ok := body["tree"].([]any)
	if !ok {
		t.Fatalf("tree entries not a list: %T", body["tree"])
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 tree entries, got %d", len(entries))
	}
	// Find the deletion entry: only Path is set, no SHA / mode / type.
	var del, mod map[string]any
	for _, e := range entries {
		m := e.(map[string]any)
		if _, hasMode := m["mode"]; hasMode {
			mod = m
		} else {
			del = m
		}
	}
	if del == nil || del["path"] != "data/cities/ie.yaml" {
		t.Errorf("expected delete entry with path data/cities/ie.yaml, got %+v", del)
	}
	if mod == nil || mod["path"] != ".ingitdb/root-collections.yaml" {
		t.Errorf("expected modify entry with path .ingitdb/root-collections.yaml, got %+v", mod)
	}
	// Modification entry must carry content; deletion must NOT.
	if _, hasContent := mod["content"]; !hasContent {
		t.Errorf("modify entry missing content")
	}
	if _, hasContent := del["content"]; hasContent {
		t.Errorf("delete entry must not carry content, got %+v", del)
	}

	if len(mock.createdCommits) != 1 {
		t.Fatalf("expected 1 CreateCommit call, got %d", len(mock.createdCommits))
	}
	commitBody := mock.createdCommits[0]
	if commitBody["tree"] != "new-tree-sha" {
		t.Errorf("commit tree = %v, want new-tree-sha", commitBody["tree"])
	}
	parents, _ := commitBody["parents"].([]any)
	if len(parents) != 1 || parents[0] != "head-commit-sha" {
		t.Errorf("commit parents = %v, want [head-commit-sha]", parents)
	}

	if len(mock.updatedRefs) != 1 || mock.updatedRefs[0]["sha"] != "new-commit-sha" {
		t.Errorf("UpdateRef payload = %+v, want sha=new-commit-sha", mock.updatedRefs)
	}
}

func TestTreeWriter_CommitChanges_EmptyChangesError(t *testing.T) {
	t.Parallel()
	// No server needed — error fires before any I/O.
	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main"}
	w, err := NewTreeWriter(cfg)
	if err != nil {
		t.Fatalf("NewTreeWriter: %v", err)
	}
	_, err = w.CommitChanges(context.Background(), "msg", nil)
	if err == nil {
		t.Fatal("expected error for empty changes")
	}
	if !strings.Contains(err.Error(), "no changes") {
		t.Errorf("error %q does not mention 'no changes'", err.Error())
	}
}

func TestTreeWriter_CommitChanges_EmptyMessageError(t *testing.T) {
	t.Parallel()
	cfg := Config{Owner: "owner", Repo: "repo", Ref: "main"}
	w, err := NewTreeWriter(cfg)
	if err != nil {
		t.Fatalf("NewTreeWriter: %v", err)
	}
	_, err = w.CommitChanges(context.Background(), "", []TreeChange{{Path: "x"}})
	if err == nil {
		t.Fatal("expected error for empty commit message")
	}
}

func TestTreeWriter_ListFilesUnder_FiltersByPrefix(t *testing.T) {
	t.Parallel()
	mock := &treeMockServer{
		t:             t,
		defaultBranch: "main",
		headRef:       "head-commit-sha",
		headCommit:    &githubCommit{SHA: "head-commit-sha", Tree: "base-tree-sha"},
		tree: &githubTree{SHA: "base-tree-sha", Entries: []githubTreeEntry{
			{Path: "data/cities/ie.yaml", Mode: "100644", Type: "blob", SHA: "b1"},
			{Path: "data/cities/gb.yaml", Mode: "100644", Type: "blob", SHA: "b2"},
			{Path: "data/cities", Mode: "040000", Type: "tree", SHA: "t1"}, // directory entry — skipped
			{Path: "data/countries/fr.yaml", Mode: "100644", Type: "blob", SHA: "b3"},
			{Path: ".ingitdb/root-collections.yaml", Mode: "100644", Type: "blob", SHA: "b4"},
		}},
	}
	srv := httptest.NewServer(mock.handler())
	defer srv.Close()

	w := newTestTreeWriter(t, srv, "main")
	paths, err := w.ListFilesUnder(context.Background(), "data/cities")
	if err != nil {
		t.Fatalf("ListFilesUnder: %v", err)
	}
	want := map[string]bool{"data/cities/ie.yaml": true, "data/cities/gb.yaml": true}
	if len(paths) != len(want) {
		t.Fatalf("got %d paths, want %d: %v", len(paths), len(want), paths)
	}
	for _, p := range paths {
		if !want[p] {
			t.Errorf("unexpected path %q in result", p)
		}
	}
}

func TestTreeWriter_ListFilesUnder_TruncatedError(t *testing.T) {
	t.Parallel()
	mock := &treeMockServer{
		t:             t,
		defaultBranch: "main",
		headRef:       "head-commit-sha",
		headCommit:    &githubCommit{SHA: "head-commit-sha", Tree: "base-tree-sha"},
		tree:          &githubTree{SHA: "base-tree-sha", Truncated: true, Entries: nil},
	}
	srv := httptest.NewServer(mock.handler())
	defer srv.Close()

	w := newTestTreeWriter(t, srv, "main")
	_, err := w.ListFilesUnder(context.Background(), "data/cities")
	if err == nil {
		t.Fatal("expected error for truncated tree")
	}
	if !strings.Contains(err.Error(), "truncated") {
		t.Errorf("error %q does not mention 'truncated'", err.Error())
	}
}

func TestTreeWriter_ResolveBranch_DefaultsWhenRefEmpty(t *testing.T) {
	t.Parallel()
	mock := &treeMockServer{
		t:             t,
		defaultBranch: "trunk",
		headRef:       "head",
		headCommit:    &githubCommit{SHA: "head", Tree: "tree"},
		tree:          &githubTree{SHA: "tree"},
	}
	srv := httptest.NewServer(mock.handler())
	defer srv.Close()

	// Ref intentionally left empty — TreeWriter must call Repositories.Get
	// to resolve "trunk" as the default branch.
	w := newTestTreeWriter(t, srv, "")
	branch, err := w.resolveBranch(context.Background())
	if err != nil {
		t.Fatalf("resolveBranch: %v", err)
	}
	if branch != "trunk" {
		t.Errorf("branch = %q, want trunk", branch)
	}
}

func TestTreeWriter_NewTreeWriter_RequiresOwnerRepo(t *testing.T) {
	t.Parallel()
	cases := []Config{
		{Repo: "r"},
		{Owner: "o"},
	}
	for _, cfg := range cases {
		_, err := NewTreeWriter(cfg)
		if err == nil {
			t.Errorf("expected error for cfg %+v", cfg)
		}
	}
}
