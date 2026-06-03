# ADR 0001: Remove the `serve` command (MCP + HTTP API gateways)

**Status:** Accepted
**Date:** 2026-06-03

## Context

`ingitdb serve` started one or more long-running services in a single process:

- an **HTTP REST API** (`server/api`) over a **GitHub-backed** inGitDB database, with
  GitHub **OAuth** (`server/auth`) — deployed as `api.ingitdb.com`;
- an **MCP-over-HTTP** server (`server/mcp`) exposing inGitDB CRUD tools to AI
  assistants — deployed as `mcp.ingitdb.com`;
- a `--watcher` flag (never implemented).

Both gateways were deployed to **Google Cloud Run** (`server/Dockerfile` +
`deploy-server.yml` / `deploy-server-from-code.yml` + a `deploy-server` job in
`release.yml`). A GitHub Copilot integration (`.github/copilot/mcp.json` +
`copilot-setup-steps.yml`) ran `serve --mcp` to give the coding agent MCP access.

Crucially, the actual GitHub-backed CRUD logic lives in the **`pkg/dalgo2ghingitdb`
library** — the HTTP/MCP handlers are thin wrappers around it. The endpoints were
**undocumented** (no README/website mention) and deployed only via manual
(`workflow_dispatch`) workflows: in practice a **still-born, experimental surface**.

## Decision

**Remove the `serve` command and its server packages entirely**, and mark the
`cli/serve` feature **Deprecated**. Remote/hosted and cross-source serving is the
domain of **[DataTug](https://github.com/datatug/datatug-cli)**, which already
consumes inGitDB as a backend (`datatug-cli` imports `ingitdb-cli`).

## Rationale

- **Dependency direction.** `datatug → ingitdb` is one-way. ingitdb cannot delegate
  `serve` to datatug without a circular dependency. ingitdb stays a self-contained
  CLI + library.
- **No unique value for ingitdb's real consumers.** Go programs (including datatug)
  get GitHub-backed CRUD by importing `pkg/dalgo2ghingitdb` directly — no server
  needed. Coding agents with a shell use the **CLI** directly — MCP adds nothing.
- **The only server-only capability is an OAuth proxy** (holding the GitHub
  `client_secret` to do the browser code→token exchange) for non-shell/remote
  clients. That is a hosted-web-product concern — i.e. datatug's domain, not core
  ingitdb's.
- **Maintenance cost.** ~6.5K LOC (incl. tests) + an MCP SDK dependency, for a
  surface that duplicates datatug.

A platform-conditional or partial (MCP-only) keep was considered and rejected as not
worth the overlap. If a need re-emerges, restore it **with a clear comment explaining
why it is needed** (see Recovery).

## Consequences

Removed: `cmd/ingitdb/commands/serve*.go`, `server/{api,auth,mcp}`, `server/Dockerfile`,
`.github/workflows/deploy-server*.yml`, the `deploy-server` job in `release.yml`, the
`.github/actions/deploy-to-cloud-run` composite action, and the Copilot MCP
integration (`.github/copilot/mcp.json`, `copilot-setup-steps.yml`). `go mod tidy`
drops `github.com/metoro-io/mcp-golang` and `github.com/julienschmidt/httprouter`.

- **GitHub Copilot coding agent** loses its inGitDB MCP server; it should use the
  CLI directly instead.
- **Manual GCP teardown required.** Removing the deploy config only stops
  *redeployment*. The live Cloud Run service must be deleted by hand:
  `gcloud run services delete ingitdb-server --project=ingitdb --region=europe-west3`
  (and revoke/clean the associated GitHub OAuth app + `GCP_OAUTH_CLIENT_SECRET`).
  The `api.ingitdb.com` / `mcp.ingitdb.com` DNS records should be removed once the
  service is gone.
- The `server/` directory now holds only the website (`ingitdb.com/`, `static/`,
  `firebase.json`), deployed via `deploy-website`.

## Recovery

The full implementation is preserved in git history:

- **Last present at commit `184a40e`** (the commit before removal).
- **Removed in commit `REMOVAL_COMMIT_PENDING`.**

Restore a file with, e.g.:

```bash
git show 184a40e:cmd/ingitdb/commands/serve.go
git show 184a40e:server/mcp/handler.go
```
