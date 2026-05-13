# Feature: Remote Repository Access

**Status:** Implementing

## Summary

Several commands accept `--remote=<value>` as a substitute for `--path` to read or write records directly against a remote Git hosting service (GitHub, GitLab, Bitbucket, and self-hosted instances) over the service's REST API, without a local clone. Authentication is provided via `--token` or host-derived environment variables. Each successful write operation MUST result in exactly one commit in the remote repository.

This feature supersedes the earlier `--github`-only flag and its `github-direct-access` feature, generalizing the design to multiple providers without per-provider flag inflation.

## Problem

Cloning a repository solely to read or update one record is wasteful when the data already lives on a remote Git host. Direct remote access lets ingitdb-cli participate in CI workflows, scripts, and AI agents that operate on remote repositories without ever touching local disk.

Per-provider flags (`--github`, `--gitlab`, `--bitbucket`) scale poorly: they require N(N−1)/2 mutual-exclusion checks per verb, cannot express "I have a URL on a host I don't recognize" for self-hosted instances (e.g. GitHub Enterprise, self-hosted GitLab), and force every new verb to remember every provider flag. A single `--remote=<value>` flag with host-based provider inference solves both problems and keeps the verb surface stable as new backends land.

## Behavior

### Flag

#### REQ: remote-flag-canonical

The `--remote=<value>` flag MUST resolve `<value>` to a canonical `(host, owner, repo, ref)` tuple, where `ref` is optional. The flag MUST be mutually exclusive with `--path`; supplying both MUST result in an error before any I/O.

#### REQ: remote-flag-grammar

The `--remote` value MUST accept every form in the table below, normalizing each to the same canonical tuple:

| Form | Example |
|---|---|
| Bare | `github.com/owner/repo` |
| Bare with ref | `github.com/owner/repo@main` |
| Bare alias (see REQ: remote-host-alias) | `github/owner/repo` |
| HTTPS URL | `https://github.com/owner/repo` |
| HTTPS URL with ref | `https://github.com/owner/repo@main` |
| HTTP URL | `http://localhost:3000/owner/repo` |
| URL with `.git` suffix | `https://github.com/owner/repo.git` |
| SSH-style | `git@github.com:owner/repo` |
| SSH-style with `.git` | `git@github.com:owner/repo.git` |

A trailing `.git` on the final path segment MUST be stripped before canonicalization. The SSH-style form is accepted as syntactic sugar only: authentication still uses tokens, never SSH keys.

The `ref` separator is the **last** `@` after host and path components are isolated. For URL forms, the scheme (`https://` / `http://`) is stripped first; for SSH-style forms, the `git@host:` prefix is rewritten to `host/`. There is no collision with URL userinfo (`user@host`), because userinfo precedes the host while `@ref` follows the path.

Multi-segment paths (e.g. GitLab subgroups) MUST be supported: `gitlab.com/group/subgroup/project`. From canonicalization's perspective, every path segment after the host belongs to the opaque repo identifier; the provider adapter interprets the structure.

#### REQ: remote-host-alias

In the **bare** form only, the following short host aliases MUST be expanded before canonicalization:

| Alias | Expands to |
|---|---|
| `github` | `github.com` |
| `gitlab` | `gitlab.com` |
| `bitbucket` | `bitbucket.org` |

Aliases MUST NOT be expanded in URL or SSH-style forms. In those forms the user is presumed to have a real hostname (typically copy-pasted from the provider UI), so `https://github/...` and `git@github:...` MUST be treated as literal, non-aliased hostnames and rejected as unknown hosts per REQ: provider-override.

### Provider dispatch

#### REQ: provider-inference

The canonical host MUST be matched against a built-in provider table:

| Host | Provider |
|---|---|
| `github.com` | `github` |
| `gitlab.com` | `gitlab` |
| `bitbucket.org` | `bitbucket` |

When the host matches a built-in entry, no `--provider` flag is required.

#### REQ: provider-override

For any host not in the built-in table, `--provider=<id>` MUST be supplied. The id MUST identify a registered provider adapter. Absence of `--provider` for an unknown host MUST result in an error listing the supported provider ids, before any I/O.

#### REQ: provider-not-implemented

If the resolved provider does not yet have an adapter compiled in, the command MUST fail with a clear error such as `provider "gitlab" is not yet supported`, with a non-zero exit code, before any I/O. Adding a new provider MUST NOT require any user-facing flag change.

### Authentication

#### REQ: token-resolution

A token for the resolved host MUST be obtained from the first non-empty source in the following order:

1. `--token=<value>` (CLI flag).
2. `<HOST_NO_TLD>_TOKEN` environment variable.
3. `<HOST_FULL>_TOKEN` environment variable.

`<HOST_FULL>` is derived from the canonical host by uppercasing it and replacing every `.` with `_`. `<HOST_NO_TLD>` is derived the same way after dropping the rightmost host label.

| Canonical host | `<HOST_NO_TLD>_TOKEN` | `<HOST_FULL>_TOKEN` |
|---|---|---|
| `github.com` | `GITHUB_TOKEN` | `GITHUB_COM_TOKEN` |
| `gitlab.com` | `GITLAB_TOKEN` | `GITLAB_COM_TOKEN` |
| `bitbucket.org` | `BITBUCKET_TOKEN` | `BITBUCKET_ORG_TOKEN` |
| `git.corp.example.com` | `GIT_CORP_EXAMPLE_TOKEN` | `GIT_CORP_EXAMPLE_COM_TOKEN` |
| `test.example.com` | `TEST_EXAMPLE_TOKEN` | `TEST_EXAMPLE_COM_TOKEN` |

The "drop rightmost label" rule does NOT use the Public Suffix List. For two-label public suffixes (e.g. `.co.uk`), the `<HOST_NO_TLD>` form will look unusual (`EXAMPLE_CO_TOKEN`), but the `<HOST_FULL>` form (`EXAMPLE_CO_UK_TOKEN`) MUST always work. Users on such domains are expected to set the full form.

When the canonical host is a single label (degenerate case), `<HOST_NO_TLD>` collapses to the empty string and that lookup MUST be skipped; only `<HOST_FULL>_TOKEN` applies.

#### REQ: token-required-for-writes

Read operations against publicly accessible repositories MUST work without a token. All write operations (`insert`, `update`, `delete`, `drop`) MUST require a token, even when the target repository is public.

### Write semantics

#### REQ: one-commit-per-write

Each successful write operation MUST produce exactly one commit in the remote repository, containing the change for that single operation. The command MUST NOT batch multiple unrelated writes into a single commit.

### No local clone

#### REQ: no-local-clone

The command MUST NOT clone the remote repository to local disk in order to satisfy a `--remote` operation. All reads and writes are performed via the resolved provider's REST API.

## Acceptance Criteria

### AC: read-public-without-token

**Requirements:** remote-repo-access#req:remote-flag-canonical, remote-repo-access#req:token-resolution, remote-repo-access#req:provider-inference

`ingitdb select --remote=github.com/owner/public-repo --id=collection/key` succeeds without `--token` or any `*_TOKEN` environment variable set, when the repository is public.

### AC: write-creates-one-commit

**Requirements:** remote-repo-access#req:token-required-for-writes, remote-repo-access#req:one-commit-per-write, remote-repo-access#req:no-local-clone

With a valid token, every `--remote` write produces exactly one commit in the target repository whose diff is limited to that operation. No `.git` directory is created on the caller's filesystem.

### AC: path-remote-mutex

**Requirements:** remote-repo-access#req:remote-flag-canonical

`ingitdb select --path=. --remote=github.com/owner/repo --id=x/y` exits non-zero with an error mentioning both `--path` and `--remote`, before any I/O.

### AC: grammar-equivalence

**Requirements:** remote-repo-access#req:remote-flag-grammar, remote-repo-access#req:remote-host-alias

All of the following invocations MUST resolve to the same canonical `(github.com, owner, repo, main)` tuple:

- `--remote=github.com/owner/repo@main`
- `--remote=github/owner/repo@main`
- `--remote=https://github.com/owner/repo@main`
- `--remote=https://github.com/owner/repo.git@main`
- `--remote=git@github.com:owner/repo@main`
- `--remote=git@github.com:owner/repo.git@main`

### AC: url-alias-not-expanded

**Requirements:** remote-repo-access#req:remote-host-alias, remote-repo-access#req:provider-override

`ingitdb select --remote=https://github/owner/repo --id=x/y` MUST be treated as the literal host `github` (a real, single-label hostname). With no `--provider`, it MUST fail with the unknown-host error from REQ: provider-override. Aliases apply only to the bare form.

### AC: unknown-host-requires-provider

**Requirements:** remote-repo-access#req:provider-override

`ingitdb select --remote=git.corp.example.com/owner/repo --id=x/y`, without `--provider`, MUST fail with an error listing the supported provider ids, before any I/O. The same invocation with `--provider=<id>` MUST proceed (subject to REQ: provider-not-implemented).

### AC: token-env-fallback-order

**Requirements:** remote-repo-access#req:token-resolution

With only `GITHUB_TOKEN` set, `ingitdb update --remote=github.com/owner/repo --id=x/y --set=...` MUST use that token. With only `GITHUB_COM_TOKEN` set, it MUST use that token. With both set, `GITHUB_TOKEN` MUST win because it appears earlier in the resolution order. `--token=<value>` on the CLI MUST override both.

### AC: ssh-form-uses-rest

**Requirements:** remote-repo-access#req:remote-flag-grammar, remote-repo-access#req:no-local-clone

`ingitdb update --remote=git@github.com:owner/repo --token=ghp_... --id=x/y --set=...` MUST succeed via the GitHub REST API and produce a single commit. The user's SSH keys, agent, or `~/.ssh/` config MUST NOT be consulted.

### AC: self-hosted-host-token

**Requirements:** remote-repo-access#req:token-resolution, remote-repo-access#req:provider-override

With `GIT_CORP_EXAMPLE_TOKEN` set and `--provider=github-enterprise`, `ingitdb select --remote=git.corp.example.com/owner/repo --id=x/y` MUST authenticate using that token without requiring `--token` on the command line.

## Outstanding Questions

- How should the command behave when the provider rate-limits the caller mid-operation? (carried over from `github-direct-access`)
- Should the commit message format be configurable per command or fixed by convention? (carried over)
- Is `--provider` a global flag or per-command? Proposed: global, registered alongside `--remote` and `--token` by `addRemoteFlags`.
- Should `--provider` accept a versioned id (e.g. `github-enterprise@v3`) for hosts that expose multiple incompatible API generations, or is one id per provider sufficient?

---
*This document follows the https://specscore.md/feature-specification*
