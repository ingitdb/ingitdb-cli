# 🖥️ inGitDB CLI Roadmap

## 🎯 Vision

A CLI tool that turns a Git repository into a fully-featured, AI-friendly database: schema-validated, queryable, with precomputed views, event-driven integrations, and native MCP support for AI agents.

## 🧭 Phases

| Phase | Feature | Status |
|---|---|---|
| 1 | Validator + Materialized Views | WIP |
| 2 | Query | Pending |
| 3 | Git Merge Conflict Resolution | Pending |
| 4 | Watcher | Pending |
| 5 | Subscribers | Pending |
| 6 | MCP Server | Pending |
| 7 | HTTP API Server | Pending |
| 8 | GraphQL | Pending |
| 9 | Migration Script Generator | Pending |

---

## 🧩 Phase 1: Validator + Materialized Views

**Goal:** Reliable, fast schema and data validation with materialized views rebuilt as part of the same pass.

**Why together:** The validator already walks every collection and record. Materialized views are computed from that same data, so rebuilding them during validation avoids a second full scan.

**Deliverables:**
- `ingitdb validate [--path=PATH] [--from-commit=SHA] [--to-commit=SHA]` validates schema and all records, then rebuilds materialized views for affected collections
- Clear, actionable error messages: collection, file path, field, and violation description
- Change validation mode: validate and rematerialize only files changed between two commits
- CLI subcommand interface per `docs/CLI.md`

---

## 🔎 Phase 2: Query

**Goal:** Read and filter records from collections via the CLI.

**Deliverables:**
- `ingitdb query --collection=<key> [--path=PATH] [--format=CSV|JSON|YAML]` returns records from a collection
- Default format is JSON

---

## ⚔️ Phase 3: Git Merge Conflict Resolution

**Goal:** Resolve git merge and rebase conflicts inside an inGitDB database without manual editing of conflict markers.

**Why here:** Depends on Phase 1 (views builder, to regenerate conflicted generated files) and Phase 2 (data reading, used by the diff TUI).

**Deliverables:**
- `ingitdb setup [--path=PATH]` — registers `ingitdb` as a git merge driver in `.gitattributes` and local git config
- `ingitdb resolve [--path=PATH]` — scans for conflicted files and resolves them
- `ingitdb pull [--path=PATH] [--strategy=rebase|merge] [--remote=REMOTE] [--branch=BRANCH]` — pulls from origin, auto-resolves generated file conflicts, opens TUI for data conflicts, rebuilds views, and prints a change summary
- Generated files (`$views/**`, `README.md`) resolved automatically by regenerating from current source data
- Source data files (`$records/*`) resolved interactively via a TUI showing conflicting fields side-by-side in a data table format
- See [Merge Conflict Resolver component doc](components/merge-conflict-resolver.md) for implementation details

---

## 🧩 Phase 4: Watcher

**Goal:** Real-time visibility into record changes as they happen on the file system.

**Deliverables:**
- `ingitdb watch [--path=PATH] [--format=text|json]` — foreground command that streams record events to stdout until interrupted
- `ingitdb serve --watcher` — same engine running as part of a multi-service process
- Events: `added`, `updated` (with changed field names and values), `deleted`
- Text and JSON output formats; one event per line for easy piping
- See [Watcher component doc](components/watcher.md) and [feature doc](features/watcher.md)

---

## 🔔 Phase 5: Subscribers

**Goal:** Event-driven notifications when data changes, usable in CI or as standalone hooks.

**Deliverables:**
- Subscriber configuration at collection or DB level in YAML
- Three subscriber types: webhook (HTTP POST), email, and shell command
- Triggered after successful validation and materialization

---

## 🤖 Phase 6: MCP Server

**Goal:** Expose inGitDB to AI agents via the Model Context Protocol (MCP).

**Deliverables:**
- `ingitdb serve --mcp [--path=PATH]`
- Tools: list known DBs, list collections, get collection metadata, CRUD on records
- Transaction support: begin / commit / rollback

---

## 🌐 Phase 7: HTTP API Server

**Goal:** REST access to inGitDB data for external tooling and integrations.

**Deliverables:**
- OpenAPI-compatible HTTP server
- CRUD endpoints per collection
- Schema-driven request and response validation

---

## 🧾 Phase 8: GraphQL

**Goal:** GraphQL interface auto-generated from collection schemas.

**Implementation:** Uses [graphql-go/graphql](https://github.com/graphql-go/graphql) library for schema and query execution.

**Deliverables:**
- Schema derived from `.definition.yaml` definitions
- Query and mutation support
- Auto-generated GraphQL schema from collection definitions

---

## 🧭 Phase 9: Migration Script Generator

**Goal:** Generate forward and rollback migration scripts to sync a target database with a desired inGitDB version.

**Deliverables:**
- `ingitdb migrate --from=<sha> --to=<sha> --target=<connection-string> [--collections=...] [--output-dir=...]`
- Diffs records and schemas between two git SHAs; produces INSERT/UPDATE/DELETE and ALTER TABLE statements
- Rollback script generated alongside the forward migration
- Initial format: SQL; target introspected via connection string to validate applicability and infer dialect
- See [Migration Generator component doc](components/migration-generator.md) for implementation details
