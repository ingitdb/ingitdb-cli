# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Build
go build -o ingitdb ./cmd/ingitdb

# Run all tests
go test -timeout=10s ./...

# Run a single test
go test -timeout=10s -run TestName ./path/to/package

# Test coverage
go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out

# Lint (must report no errors before committing)
golangci-lint run
```

## Architecture

**inGitDB** stores database records as YAML/JSON files in a Git repository. Collections, schemas, views, and
materialized views are defined declaratively in `.ingitdb.yaml` configuration.

The codebase has two main packages:

- **`pkg/ingitdb/`** — Core schema definitions (`Definition`, `CollectionDef`, `ColumnDef`, views) and the `validator/`
  sub-package that reads and validates a database directory against its schema.
- **`pkg/dalgo2ingitdb/`** — DALgo (Database Abstraction Layer) integration, implementing `dal.DB`, read-only and
  read-write transactions for CRUD access.
- **`cmd/ingitdb/`** — CLI entry point using `github.com/spf13/cobra` for subcommand and flag parsing. The `run()`
  function is dependency-injected for testability (accepts `homeDir`, `readDefinition`, `fatal`, `logf` as parameters).
- **`cmd/watcher/`** — Obsolete file watcher, to be folded into `ingitdb watch`.
- **`cmd/ingitdb/commands/sqlflags/`** — Shared CLI flag grammar for the
  SQL-verb redesign (select, insert, update, delete, drop). Parsers,
  mode resolution, applicability checks, and cobra registration helpers.
  Each verb command imports from here. Old verbs (`read record`,
  `create record`, `query`, etc.) do not — they keep using
  `cmd/ingitdb/commands/query_parser.go` until the final cleanup plan.

Test data lives in `test-ingitdb/` and `.ingitdb.yaml` at the repo root points to it.

## Code Conventions

- **No nested calls**: never write `f2(f1())`; assign the intermediate result first.
- **Errors**: always check or explicitly ignore returned errors. Avoid `panic` in production code.
- **Output**: use `fmt.Fprintf(os.Stderr, ...)` — never `fmt.Println`/`fmt.Printf` — to avoid interfering with TUI
  stdout.
- **Unused params**: mark intentionally unused function parameters with `_, _ = a1, a2`.
- **No package-level variables**: pass dependencies via struct fields or function parameters.
- **Tests**: call `t.Parallel()` as the first statement in every top-level test.
- **Build validation**: if any Go code or `go.mod` is modified, run `go build ./...` and `go test ./...` before
  reporting the task as done to ensure the code compiles and tests are passing.

## Commit Messages

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/):

```
<type>(<scope>): <short summary>

<optional body>

<optional footer>
```

**Type:** `feat`, `fix`, `docs`, `refactor`, `test`, `chore`, `ci`, `perf`

**Guidelines:**

- Summary must be lowercase, imperative, and not end with a period
- Use `!` after type/scope for breaking changes: `feat!:` or `feat(scope)!:`
- Body is optional but recommended for non-trivial changes
- Include `Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>` footer when appropriate

**Examples:**

```
feat(cli): add --output flag for JSON export

Allows users to export database records as JSON format.
Implements RFC-42.

Co-Authored-By: Claude Haiku 4.5 <noreply@anthropic.com>
```

```
fix: handle empty collections gracefully

Previously panicked when encountering empty collection directories.
Now gracefully handles and logs the situation.
```

```
docs: update installation instructions
```

See [Conventional Commits specification](https://www.conventionalcommits.org/en/v1.0.0/) for full details.

## Spec Workflow (SpecScore)

This project uses [SpecScore](https://specscore.md) to track feature specifications.
The `specscore` CLI is installed at `$(go env GOPATH)/bin/specscore`.

**Directory layout:**

```
spec/
  ideas/     → raw ideas before promotion (spec/ideas/<slug>.md)
  features/  → feature specs (spec/features/<group>/<slug>/README.md)
  plans/     → implementation plans
```

**Rules for AI agents:**

1. **New ideas** → scaffold with `specscore idea new <slug>`, then fill in sections.
   Never save ideas to `docs/`.
2. **Feature specs** → live under `spec/features/`. Revise in place when enhancing an
   existing feature; create a new slug only when scope is entirely new.
3. **Lint before finishing** → run `specscore spec lint` and fix all errors before
   marking any spec work as done. Use `specscore spec lint --fix` for auto-fixable issues.
4. **Ideas index** → `spec/ideas/README.md` is managed by specscore lint `--fix`; do not
   hand-edit the index table rows.

**Common commands:**

```bash
specscore idea new <slug> --title "..." --hmw "How Might We..."
specscore spec lint
specscore spec lint --fix
specscore feature list
```
