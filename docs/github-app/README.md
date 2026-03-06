# inGitDB GitHub App

## Purpose

The inGitDB GitHub App provides automation for inGitDB repositories hosted on GitHub, with
behaviour governed by the inGitDB schema and permission model rather than GitHub branch
protection rules or per-repository workflow YAML.

### Why not GitHub Actions workflows?

GitHub Actions workflows are sufficient when PRs come from branches within the same repository.
However, in the AgileLedger model — and other privacy-first workflows — users submit data by
forking the team repository into their own private account and opening a PR from the fork.
GitHub's security model intentionally prevents fork-originated workflows from having write
access to the upstream repository, so auto-merging is impossible.

The GitHub App is installed on the upstream (team) repository and always has the permissions it
was granted, regardless of whether the PR originates from a branch or a fork.

---

## Features

### [PR Auto-Merge _(priority)_](./pr-auto-merge-feature.md)

When a user submits data via a PR, the App validates the changed records, evaluates the
repository's permission model, and merges automatically if all rules are satisfied. PRs that
cannot be merged automatically receive a structured comment and trigger review requests to the
configured reviewers.

- [Feature spec](./pr-auto-merge-feature.md)
- [Dev plan](./pr-auto-merge-dev-plan.md)

### [Background Dependency Validation](./dependency-validation-feature.md)

On a configurable schedule, the App scans inGitDB records for broken cross-collection references
and external source references. When violations are found it opens or updates a GitHub Issue with
a structured report; when all violations are resolved it closes the issue automatically.

- [Feature spec](./dependency-validation-feature.md)
- [Dev plan](./dependency-validation-dev-plan.md)

---

## GitHub App Permissions

| Permission      | Level | Required for                                    |
|-----------------|-------|-------------------------------------------------|
| `contents`      | read  | Reading repo files via GitHub API               |
| `contents`      | write | Merging PRs                                     |
| `pull_requests` | write | Posting review comments, merging PRs            |
| `checks`        | write | Creating and updating Check Runs                |
| `issues`        | write | Opening validation report issues (dep. validation) |
| `metadata`      | read  | Required by GitHub for all Apps                 |

**Webhook subscriptions:** `pull_request` — opened, synchronize, reopened, closed

---

## Authentication: PAT vs GitHub App

The CLI and HTTP server support two authentication modes selected by which credentials are
configured.

| Mode       | How it works                                                    | Best for                             |
|------------|-----------------------------------------------------------------|--------------------------------------|
| PAT        | `GITHUB_TOKEN` env var or `--token` flag; belongs to a user    | Local development, single-user repos |
| GitHub App | App ID + Installation ID + private key; token is auto-rotating | Team repos, hosted service, fork PRs |

When running as a GitHub App, the server exchanges the App's private key for a short-lived
installation access token before each GitHub API call. No long-lived user token is stored.

### Configuration

```yaml
# .ingitdb.yaml
github_app:
  auth:
    app_id: 123456
    installation_id: 78901234
    private_key_path: /run/secrets/ingitdb-app.pem
    # Or base64-encoded for container deployments:
    # private_key_base64: <base64>
```

Environment variable equivalents:
- `INGITDB_GITHUB_APP_ID`
- `INGITDB_GITHUB_APP_INSTALLATION_ID`
- `INGITDB_GITHUB_APP_PRIVATE_KEY_PATH`

---

## Deployment

The GitHub App server is the existing `ingitdb serve` binary extended with a webhook endpoint.

```
ingitdb serve \
  --port 8080 \
  --github-app
```

The server must be reachable from GitHub's webhook delivery IPs. The webhook secret is verified
via `X-Hub-Signature-256` on every incoming request.

```
INGITDB_GITHUB_WEBHOOK_SECRET=<secret>
```
