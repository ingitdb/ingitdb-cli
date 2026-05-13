package commands

import (
	"fmt"
	"net/url"
	"slices"
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
