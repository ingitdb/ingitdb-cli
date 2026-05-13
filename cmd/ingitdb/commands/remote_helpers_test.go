package commands

import (
	"reflect"
	"strings"
	"testing"
)

func TestResolveRemoteProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		host       string
		override   string
		wantProv   remoteProvider
		wantErrSub string // empty = no error expected
	}{
		// Built-in inference, implemented provider.
		{name: "github.com infers github", host: "github.com", wantProv: providerGitHub},
		{name: "github.com with matching override", host: "github.com", override: "github", wantProv: providerGitHub},

		// Registered but not implemented.
		{name: "gitlab.com not yet supported", host: "gitlab.com", wantErrSub: "not yet supported"},
		{name: "bitbucket.org not yet supported", host: "bitbucket.org", wantErrSub: "not yet supported"},

		// Unknown host paths.
		{name: "unknown host no override", host: "git.corp.example.com", wantErrSub: "unknown remote host"},
		{name: "unknown host github override", host: "git.corp.example.com", override: "github", wantProv: providerGitHub},
		{name: "unknown host gitlab override not impl", host: "git.corp.example.com", override: "gitlab", wantErrSub: "not yet supported"},

		// Unknown --provider value.
		{name: "unknown provider override", host: "github.com", override: "foobar", wantErrSub: "unknown --provider"},

		// Single-label host (URL-form alias not expanded).
		{name: "single-label host", host: "github", wantErrSub: "unknown remote host"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			spec := remoteSpec{Host: tc.host, Path: []string{"owner", "repo"}}
			got, err := resolveRemoteProvider(spec, tc.override)
			if tc.wantErrSub != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got provider %q", tc.wantErrSub, got)
				}
				if !strings.Contains(err.Error(), tc.wantErrSub) {
					t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrSub)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantProv {
				t.Errorf("provider = %q, want %q", got, tc.wantProv)
			}
		})
	}
}

// TestResolveRemoteProvider_ErrorListsKnown verifies the error messages
// include the supported provider list, so users learn what's available.
func TestResolveRemoteProvider_ErrorListsKnown(t *testing.T) {
	t.Parallel()

	_, err := resolveRemoteProvider(remoteSpec{Host: "unknown.example.com"}, "")
	if err == nil {
		t.Fatal("expected error for unknown host")
	}
	// "known: github, gitlab, bitbucket" (sorted alphabetically)
	for _, want := range []string{"github", "gitlab", "bitbucket"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing provider id %q", err.Error(), want)
		}
	}
}

func TestHostTokenEnvName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		host    string
		dropTLD bool
		want    string
	}{
		{name: "github.com no TLD", host: "github.com", dropTLD: true, want: "GITHUB_TOKEN"},
		{name: "github.com full", host: "github.com", dropTLD: false, want: "GITHUB_COM_TOKEN"},
		{name: "gitlab.com no TLD", host: "gitlab.com", dropTLD: true, want: "GITLAB_TOKEN"},
		{name: "bitbucket.org no TLD", host: "bitbucket.org", dropTLD: true, want: "BITBUCKET_TOKEN"},
		{name: "bitbucket.org full", host: "bitbucket.org", dropTLD: false, want: "BITBUCKET_ORG_TOKEN"},
		{name: "self-hosted no TLD", host: "git.corp.example.com", dropTLD: true, want: "GIT_CORP_EXAMPLE_TOKEN"},
		{name: "self-hosted full", host: "git.corp.example.com", dropTLD: false, want: "GIT_CORP_EXAMPLE_COM_TOKEN"},
		{name: "test.example.com no TLD", host: "test.example.com", dropTLD: true, want: "TEST_EXAMPLE_TOKEN"},
		{name: "test.example.com full", host: "test.example.com", dropTLD: false, want: "TEST_EXAMPLE_COM_TOKEN"},
		{name: "single-label drop TLD empty", host: "github", dropTLD: true, want: ""},
		{name: "single-label full", host: "github", dropTLD: false, want: "GITHUB_TOKEN"},
		{name: "empty host", host: "", dropTLD: true, want: ""},
		{name: "empty host full", host: "", dropTLD: false, want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := hostTokenEnvName(tc.host, tc.dropTLD)
			if got != tc.want {
				t.Errorf("hostTokenEnvName(%q, %v) = %q, want %q", tc.host, tc.dropTLD, got, tc.want)
			}
		})
	}
}

func TestResolveRemoteToken(t *testing.T) {
	t.Parallel()

	// mkEnv returns a getEnv stub backed by a map.
	mkEnv := func(env map[string]string) func(string) string {
		return func(k string) string { return env[k] }
	}

	tests := []struct {
		name      string
		host      string
		flagValue string
		env       map[string]string
		want      string
	}{
		// --token flag wins over everything.
		{
			name: "flag wins over env",
			host: "github.com", flagValue: "from-flag",
			env:  map[string]string{"GITHUB_TOKEN": "from-short", "GITHUB_COM_TOKEN": "from-full"},
			want: "from-flag",
		},
		// Short form wins when both env vars set (AC: token-env-fallback-order).
		{
			name: "short env beats full env",
			host: "github.com",
			env:  map[string]string{"GITHUB_TOKEN": "from-short", "GITHUB_COM_TOKEN": "from-full"},
			want: "from-short",
		},
		// Only the full form set.
		{
			name: "only full env set",
			host: "github.com",
			env:  map[string]string{"GITHUB_COM_TOKEN": "from-full"},
			want: "from-full",
		},
		// Only the short form set.
		{
			name: "only short env set",
			host: "github.com",
			env:  map[string]string{"GITHUB_TOKEN": "from-short"},
			want: "from-short",
		},
		// No source supplies a value → empty.
		{
			name: "nothing set",
			host: "github.com",
			env:  map[string]string{},
			want: "",
		},
		// Self-hosted host uses the mechanical rule.
		{
			name: "self-hosted short env",
			host: "git.corp.example.com",
			env:  map[string]string{"GIT_CORP_EXAMPLE_TOKEN": "from-short"},
			want: "from-short",
		},
		{
			name: "self-hosted full env",
			host: "git.corp.example.com",
			env:  map[string]string{"GIT_CORP_EXAMPLE_COM_TOKEN": "from-full"},
			want: "from-full",
		},
		// Single-label host falls back to full form only (short is empty).
		{
			name: "single-label full env",
			host: "github",
			env:  map[string]string{"GITHUB_TOKEN": "from-full"},
			want: "from-full",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := resolveRemoteToken(tc.host, tc.flagValue, mkEnv(tc.env))
			if got != tc.want {
				t.Errorf("token = %q, want %q", got, tc.want)
			}
		})
	}
}

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
