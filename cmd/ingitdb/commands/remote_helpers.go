package commands

// specscore: feature/remote-repo-access

import (
	"fmt"
	"net/url"
	"slices"
	"sort"
	"strings"
)

// remoteSpec is the canonical representation of a `--remote` flag value.
// It is host- and provider-agnostic at this layer; provider resolution
// happens in a separate step (see resolveProvider).
//
// Path contains the path segments after the host. For most providers
// len(Path) == 2 (owner, repo), but GitLab subgroups can produce
// len(Path) > 2 (e.g. ["group", "subgroup", "project"]).
type remoteSpec struct {
	Host string
	Path []string
	Ref  string
}

// Owner returns the first path segment.
func (r remoteSpec) Owner() string { return r.Path[0] }

// Repo returns the last path segment (with any trailing .git already stripped).
func (r remoteSpec) Repo() string { return r.Path[len(r.Path)-1] }

// bareHostAliases maps short host aliases (bare form only) to canonical hosts.
// See spec/features/remote-repo-access REQ:remote-host-alias.
var bareHostAliases = map[string]string{
	"github":    "github.com",
	"gitlab":    "gitlab.com",
	"bitbucket": "bitbucket.org",
}

// parseRemoteSpec parses a --remote flag value into a canonical remoteSpec.
//
// Accepted forms (all normalize to the same canonical tuple):
//   - Bare:                host/owner/repo[@ref]                (e.g. github.com/owner/repo@main)
//   - Bare alias:          alias/owner/repo[@ref]               (alias ∈ {github, gitlab, bitbucket})
//   - HTTPS URL:           https://host/owner/repo[@ref]
//   - HTTP URL:            http://host/owner/repo[@ref]
//   - URL with .git:       https://host/owner/repo.git[@ref]
//   - SSH-style:           git@host:owner/repo[@ref]
//   - SSH-style with .git: git@host:owner/repo.git[@ref]
//
// Multi-segment paths are supported (GitLab subgroups):
// host/group/subgroup/project[@ref].
//
// A trailing `.git` is stripped from the last path segment.
//
// The @ref separator is the LAST `@` in the path portion of the value
// (after the host has been isolated). URL userinfo (user@host) is handled
// separately by the URL parser and cannot collide with @ref.
func parseRemoteSpec(value string) (remoteSpec, error) {
	if value == "" {
		return remoteSpec{}, fmt.Errorf("--remote cannot be empty")
	}

	var host, pathPart string
	var err error
	switch {
	case strings.HasPrefix(value, "git@"):
		host, pathPart, err = splitRemoteSSHForm(value)
	case strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://"):
		host, pathPart, err = splitRemoteURLForm(value)
	default:
		host, pathPart = splitRemoteBareForm(value)
	}
	if err != nil {
		return remoteSpec{}, fmt.Errorf("invalid --remote value %q: %w", value, err)
	}
	if host == "" {
		return remoteSpec{}, fmt.Errorf("invalid --remote value %q: empty host", value)
	}
	if pathPart == "" {
		return remoteSpec{}, fmt.Errorf("invalid --remote value %q: missing owner/repo", value)
	}

	// Extract optional @ref from the path portion. The ref separator is the LAST `@`.
	var ref string
	if at := strings.LastIndex(pathPart, "@"); at >= 0 {
		ref = pathPart[at+1:]
		pathPart = pathPart[:at]
		if ref == "" {
			return remoteSpec{}, fmt.Errorf("invalid --remote value %q: empty ref", value)
		}
	}

	// Trim leading/trailing slashes, then split into segments.
	pathPart = strings.Trim(pathPart, "/")
	segments := strings.Split(pathPart, "/")
	if len(segments) < 2 {
		return remoteSpec{}, fmt.Errorf("invalid --remote value %q: expected host/owner/repo[@ref]", value)
	}
	if slices.Contains(segments, "") {
		return remoteSpec{}, fmt.Errorf("invalid --remote value %q: empty path segment", value)
	}

	// Strip trailing .git from the last segment.
	last := segments[len(segments)-1]
	if trimmed, ok := strings.CutSuffix(last, ".git"); ok {
		if trimmed == "" {
			return remoteSpec{}, fmt.Errorf("invalid --remote value %q: empty repo after stripping .git", value)
		}
		segments[len(segments)-1] = trimmed
	}

	return remoteSpec{Host: host, Path: segments, Ref: ref}, nil
}

// splitRemoteSSHForm parses "git@host:path" into (host, pathPart).
func splitRemoteSSHForm(value string) (host, pathPart string, err error) {
	rest := strings.TrimPrefix(value, "git@")
	h, p, ok := strings.Cut(rest, ":")
	if !ok {
		return "", "", fmt.Errorf("ssh-style value must contain ':' separating host from path")
	}
	return h, p, nil
}

// splitRemoteURLForm parses "scheme://host/path" using net/url. URL userinfo
// (user@host) is recognized by the URL parser and does not collide with @ref;
// any @ in the path portion is preserved as part of u.Path.
func splitRemoteURLForm(value string) (host, pathPart string, err error) {
	u, parseErr := url.Parse(value)
	if parseErr != nil {
		return "", "", fmt.Errorf("invalid url: %w", parseErr)
	}
	return u.Host, strings.TrimPrefix(u.Path, "/"), nil
}

// splitRemoteBareForm parses "host/path" with optional alias expansion. Aliases
// expand only in this (bare) form — URL and SSH forms preserve literal hosts.
func splitRemoteBareForm(value string) (host, pathPart string) {
	host, pathPart, _ = strings.Cut(value, "/")
	if expanded, ok := bareHostAliases[host]; ok {
		host = expanded
	}
	return host, pathPart
}

// remoteProvider identifies a remote Git hosting backend.
type remoteProvider string

const (
	providerGitHub    remoteProvider = "github"
	providerGitLab    remoteProvider = "gitlab"
	providerBitbucket remoteProvider = "bitbucket"
)

// builtInProviderHosts maps canonical hosts to provider IDs. See spec
// REQ:provider-inference.
var builtInProviderHosts = map[string]remoteProvider{
	"github.com":    providerGitHub,
	"gitlab.com":    providerGitLab,
	"bitbucket.org": providerBitbucket,
}

// registeredProviders is the set of provider IDs the CLI recognizes, even
// when no adapter is yet compiled in. An unknown --provider value is rejected
// before any I/O.
var registeredProviders = map[remoteProvider]bool{
	providerGitHub:    true,
	providerGitLab:    true,
	providerBitbucket: true,
}

// implementedProviders is the subset of registeredProviders that has a
// working adapter today. Adding a new provider means adding its key here
// (and wiring its adapter at the dispatch site).
var implementedProviders = map[remoteProvider]bool{
	providerGitHub: true,
}

// resolveRemoteProvider decides which provider adapter handles spec, honoring
// an optional explicit override.
//
// Errors (always before any I/O):
//   - host not in builtInProviderHosts AND override is empty → "unknown remote host"
//   - override is set but not in registeredProviders → "unknown --provider"
//   - resolved provider is registered but not implemented → "provider X is not yet supported"
func resolveRemoteProvider(spec remoteSpec, override string) (remoteProvider, error) {
	var p remoteProvider
	if override != "" {
		p = remoteProvider(override)
		if !registeredProviders[p] {
			return "", fmt.Errorf("unknown --provider %q (known: %s)",
				override, registeredProviderList())
		}
	} else {
		var ok bool
		p, ok = builtInProviderHosts[spec.Host]
		if !ok {
			return "", fmt.Errorf("unknown remote host %q: pass --provider=<id> to choose an adapter (known: %s)",
				spec.Host, registeredProviderList())
		}
	}
	if !implementedProviders[p] {
		return "", fmt.Errorf("provider %q is not yet supported (implemented: %s)",
			p, implementedProviderList())
	}
	return p, nil
}

// registeredProviderList returns a stable, comma-separated list of registered
// provider IDs, for use in error messages.
func registeredProviderList() string {
	return sortedProviderKeys(registeredProviders)
}

// implementedProviderList returns a stable, comma-separated list of
// implemented provider IDs, for use in error messages.
func implementedProviderList() string {
	return sortedProviderKeys(implementedProviders)
}

func sortedProviderKeys(m map[remoteProvider]bool) string {
	ids := make([]string, 0, len(m))
	for p, ok := range m {
		if ok {
			ids = append(ids, string(p))
		}
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

// resolveRemoteToken finds the auth token for host, honoring the resolution
// order from spec REQ:token-resolution:
//  1. flagValue (the --token CLI flag, "" if unset)
//  2. <HOST_NO_TLD>_TOKEN env var (rightmost host label dropped)
//  3. <HOST_FULL>_TOKEN env var (full host, dots → underscores)
//
// Returns "" if no source supplies a value; callers decide whether that is
// an error for the operation at hand (writes always require a token; reads
// of public repos do not).
//
// getEnv is passed in for testability — production callers pass os.Getenv.
func resolveRemoteToken(host, flagValue string, getEnv func(string) string) string {
	if flagValue != "" {
		return flagValue
	}
	if shortName := hostTokenEnvName(host, true); shortName != "" {
		if v := getEnv(shortName); v != "" {
			return v
		}
	}
	if fullName := hostTokenEnvName(host, false); fullName != "" {
		if v := getEnv(fullName); v != "" {
			return v
		}
	}
	return ""
}

// hostTokenEnvName builds the env var name for host per the mechanical rule:
// uppercase, replace `.` with `_`, append `_TOKEN`. When dropTLD is true the
// rightmost label is removed first; if that leaves nothing (single-label
// host), returns "" — only the full form is meaningful then.
func hostTokenEnvName(host string, dropTLD bool) string {
	if host == "" {
		return ""
	}
	if dropTLD {
		idx := strings.LastIndex(host, ".")
		if idx < 0 {
			return ""
		}
		host = host[:idx]
	}
	return strings.ToUpper(strings.ReplaceAll(host, ".", "_")) + "_TOKEN"
}
