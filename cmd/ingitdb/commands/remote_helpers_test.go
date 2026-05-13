package commands

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseRemoteSpec_Valid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPath []string
		wantRef  string
	}{
		// canonical bare form
		{name: "bare", input: "github.com/owner/repo",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: ""},
		{name: "bare with ref", input: "github.com/owner/repo@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},
		{name: "bare with tag ref", input: "github.com/owner/repo@v1.2.0",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "v1.2.0"},

		// bare host aliases
		{name: "alias github", input: "github/owner/repo",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "alias gitlab", input: "gitlab/owner/repo",
			wantHost: "gitlab.com", wantPath: []string{"owner", "repo"}},
		{name: "alias bitbucket", input: "bitbucket/owner/repo",
			wantHost: "bitbucket.org", wantPath: []string{"owner", "repo"}},
		{name: "alias with ref", input: "github/owner/repo@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},

		// HTTPS URL
		{name: "https url", input: "https://github.com/owner/repo",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "https url with ref", input: "https://github.com/owner/repo@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},
		{name: "http url", input: "http://localhost:3000/owner/repo",
			wantHost: "localhost:3000", wantPath: []string{"owner", "repo"}},
		{name: "https url with trailing slash", input: "https://github.com/owner/repo/",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},

		// URL form aliases NOT expanded — "github" stays as a literal single-label host.
		{name: "https url alias not expanded", input: "https://github/owner/repo",
			wantHost: "github", wantPath: []string{"owner", "repo"}},

		// trailing .git suffix
		{name: "bare with .git", input: "github.com/owner/repo.git",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "https url with .git", input: "https://github.com/owner/repo.git",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "https url with .git and ref", input: "https://github.com/owner/repo.git@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},

		// SSH-style
		{name: "ssh", input: "git@github.com:owner/repo",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "ssh with ref", input: "git@github.com:owner/repo@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},
		{name: "ssh with .git", input: "git@github.com:owner/repo.git",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}},
		{name: "ssh with .git and ref", input: "git@github.com:owner/repo.git@main",
			wantHost: "github.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},

		// multi-segment paths (GitLab subgroups)
		{name: "gitlab subgroup", input: "gitlab.com/group/subgroup/project",
			wantHost: "gitlab.com", wantPath: []string{"group", "subgroup", "project"}},
		{name: "gitlab subgroup with ref", input: "gitlab.com/group/subgroup/project@main",
			wantHost: "gitlab.com", wantPath: []string{"group", "subgroup", "project"}, wantRef: "main"},

		// self-hosted (unknown host preserved verbatim — provider dispatch handles it)
		{name: "self-hosted bare", input: "git.corp.example.com/owner/repo",
			wantHost: "git.corp.example.com", wantPath: []string{"owner", "repo"}},
		{name: "self-hosted https", input: "https://git.corp.example.com/owner/repo@main",
			wantHost: "git.corp.example.com", wantPath: []string{"owner", "repo"}, wantRef: "main"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseRemoteSpec(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Host != tc.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tc.wantHost)
			}
			if !reflect.DeepEqual(got.Path, tc.wantPath) {
				t.Errorf("Path = %v, want %v", got.Path, tc.wantPath)
			}
			if got.Ref != tc.wantRef {
				t.Errorf("Ref = %q, want %q", got.Ref, tc.wantRef)
			}
			// Owner / Repo accessors must point at the right segments.
			if got.Owner() != tc.wantPath[0] {
				t.Errorf("Owner() = %q, want %q", got.Owner(), tc.wantPath[0])
			}
			if got.Repo() != tc.wantPath[len(tc.wantPath)-1] {
				t.Errorf("Repo() = %q, want %q", got.Repo(), tc.wantPath[len(tc.wantPath)-1])
			}
		})
	}
}

// TestParseRemoteSpec_GrammarEquivalence locks in spec AC: grammar-equivalence —
// every accepted form for the same repo MUST resolve to the same canonical tuple.
func TestParseRemoteSpec_GrammarEquivalence(t *testing.T) {
	t.Parallel()

	want := remoteSpec{
		Host: "github.com",
		Path: []string{"owner", "repo"},
		Ref:  "main",
	}
	inputs := []string{
		"github.com/owner/repo@main",
		"github/owner/repo@main",
		"https://github.com/owner/repo@main",
		"https://github.com/owner/repo.git@main",
		"git@github.com:owner/repo@main",
		"git@github.com:owner/repo.git@main",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			t.Parallel()

			got, err := parseRemoteSpec(in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Host != want.Host {
				t.Errorf("Host = %q, want %q", got.Host, want.Host)
			}
			if !reflect.DeepEqual(got.Path, want.Path) {
				t.Errorf("Path = %v, want %v", got.Path, want.Path)
			}
			if got.Ref != want.Ref {
				t.Errorf("Ref = %q, want %q", got.Ref, want.Ref)
			}
		})
	}
}

func TestParseRemoteSpec_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		wantSubs string // substring expected in the error message
	}{
		{name: "empty", input: "", wantSubs: "empty"},
		{name: "no slash", input: "github.com", wantSubs: "missing owner/repo"},
		{name: "host only with slash", input: "github.com/", wantSubs: "missing owner/repo"},
		{name: "single owner no repo", input: "github.com/owner", wantSubs: "expected host/owner/repo"},
		{name: "empty ref", input: "github.com/owner/repo@", wantSubs: "empty ref"},
		{name: "empty middle segment", input: "github.com/owner//repo", wantSubs: "empty path segment"},
		{name: "ssh without colon", input: "git@github.com/owner/repo", wantSubs: "ssh-style"},
		{name: "only .git", input: "github.com/owner/.git", wantSubs: "empty repo after stripping .git"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := parseRemoteSpec(tc.input)
			if err == nil {
				t.Fatalf("expected error for input %q", tc.input)
			}
			if !strings.Contains(err.Error(), tc.wantSubs) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantSubs)
			}
		})
	}
}
