# 📂 inGitDB Documentation

inGitDB turns a Git repository into a schema-validated, queryable database where every record is
a plain YAML or JSON file. There is no server to run and no binary format to decode — the data
lives in files you can read, edit, and diff with any text editor or git client. Git itself
provides the history, branching, and merge story for free. If that sounds like the right
trade-off for your project, read on — and consider contributing.

## 📂 Index

### 📂 User docs

- [CLI](cli/README.md) — command reference: flags, subcommands, exit codes
- [Configuration](configuration/README.md) — `.ingitdb/` directory and collection config files
- [Features](features/README.md) — what inGitDB can do (and what is coming)

### 📂 Developer docs

- [Architecture](ARCHITECTURE.md) — CLI component architecture and package map
- [Components (readme-builders)](components/readme-builders/) — README generation component (CLI-specific)
- [Contributing](CONTRIBUTING.md) — how to open issues and submit pull requests
- [Coding Standards](CODING_STANDARDS.md) — conventions every contributor should follow
- [CI](CI.md) — continuous integration setup and checks

### 📂 Specification (ingitdb-specs)

Storage format, schema definitions, roadmap, and system-level feature specs live in the
[ingitdb-specs](https://github.com/ingitdb/ingitdb-specs) repository:

- [Storage Format](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/STORAGE_FORMAT.md) — directory layout, collection schema, column types, record files
- [Schema Reference](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/schema/) — `.definition.yaml`, views, triggers, subcollections, subscribers
- [Features](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/features/) — feature specifications with status
- [Components](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/components/) — Validator, Scanner, Views Builder, Watcher, Triggers, MCP Server, and more
- [Roadmap](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/ROADMAP.md) — nine delivery phases from Validator to GraphQL
- [Backlog](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/BACKLOG.md) — concrete tasks with acceptance criteria
- [Competitors](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/COMPETITORS.md) — honest feature comparison with related projects
- [Guidelines](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/GUIDELINES.md) — design and process guidelines

> When working locally, ingitdb-specs is at `../../ingitdb-specs/`

---

## 📂 Documentation Map

A one-line description of each document, grouped by purpose.

### 📂 User docs

| Document                                                 | What it covers                                                          |
| -------------------------------------------------------- | ----------------------------------------------------------------------- |
| [CLI](cli/README.md)                                            | Every subcommand, flag, and exit code for the `ingitdb` binary          |
| [Configuration](configuration/README.md)                 | Root DB config (`.ingitdb/`) and per-collection schema files            |
| [Features](features/README.md)                           | Full feature list with status (WIP / pending) and links to detail pages |
| [Remote Repository Access](features/remote-repo-access.md) | Read and write records in remote Git repositories (GitHub, GitLab, Bitbucket) without cloning |
| [Default Collection View](features/default-collection-view.md) | Export collection records to `$ingitdb/` flat files for web-app consumption |

### 📂 Developer docs

| Document                                | What it covers                                                                             |
| --------------------------------------- | ------------------------------------------------------------------------------------------ |
| [Architecture](ARCHITECTURE.md)         | CLI component architecture, package map, and key design decisions                          |
| [Components (readme-builders)](components/readme-builders/) | README generation component (CLI-specific output)                   |
| [Contributing](CONTRIBUTING.md)         | Bug reports, feature suggestions, pull request workflow                                    |
| [Coding Standards](CODING_STANDARDS.md) | Go conventions enforced in this codebase                                                   |
| [CI](CI.md)                             | What the CI pipeline runs and how to satisfy it locally                                    |

### 📂 Specification (ingitdb-specs)

Spec-level docs live in [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/):

| Document | What it covers |
| -------- | -------------- |
| [Storage Format](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/STORAGE_FORMAT.md) | Directory layout, collection schema, column types, record files |
| [Schema Reference](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/schema/) | `.definition.yaml`, views, triggers, subcollections, subscribers |
| [Features](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/features/) | Feature specifications with status |
| [Components](https://github.com/ingitdb/ingitdb-specs/tree/main/docs/components/) | Validator, Scanner, Views Builder, Watcher, MCP Server, and more |
| [Roadmap](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/ROADMAP.md) | Nine delivery phases from schema validation to GraphQL |
| [Backlog](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/BACKLOG.md) | Actionable tasks with acceptance criteria and implementation notes |
| [Competitors](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/COMPETITORS.md) | Feature matrix comparing inGitDB to related tools |
| [Guidelines](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/GUIDELINES.md) | Design and process guidelines that apply project-wide |

---

## 🖥️ CLI Commands at a Glance

| Command       | Subcommands                            | Status                                    | Description                                                                                  |
| ------------- | -------------------------------------- | ----------------------------------------- | -------------------------------------------------------------------------------------------- |
| `version`     | —                                      | implemented                               | Print build version, commit hash, and date                                                   |
| `validate`    | —                                      | implemented                               | Check schema and data against `.ingitdb/root-collections.yaml`                               |
| `read`        | `record`                               | implemented                               | Read a single record by ID (local or remote)                                                 |
| `create`      | `record`                               | implemented                               | Create a new record (local or remote)                                                        |
| `update`      | `record`                               | implemented                               | Update fields of an existing record (local or remote)                                        |
| `delete`      | `record` `collection` `view` `records` | `record` implemented; others planned      | Remove a record, collection, view definition, or individual records                          |
| `list`        | `collections` `view` `subscribers`     | `collections` implemented; others planned | List schema objects, optionally scoped with `--in` and `--filter-name`                       |
| `find`        | —                                      | planned                                   | Search records by `--substr`, `--re`, or `--exact`; scope with `--in`, `--fields`, `--limit` |
| `truncate`    | —                                      | planned                                   | Remove all records from a collection while keeping its schema                                |
| `query`       | —                                      | planned                                   | Query and format records from a collection                                                   |
| `materialize` | —                                      | planned                                   | Build materialized views into `$views/` and default collection views into `$ingitdb/`        |
| `pull`        | —                                      | planned                                   | Pull remote changes, auto-resolve generated-file conflicts, rebuild views                    |
| `watch`       | —                                      | planned                                   | Stream change events to stdout as records are added, updated, or deleted                     |
| `serve`       | —                                      | planned                                   | Start MCP server, HTTP API server, or file watcher (flags: `--mcp`, `--http`, `--watcher`)   |
| `resolve`     | —                                      | planned                                   | Interactive TUI for resolving merge conflicts in data files                                  |
| `setup`       | —                                      | planned                                   | Initialise a new database directory with a starter `.ingitdb/` config                        |
| `migrate`     | —                                      | planned                                   | Migrate records between schema versions                                                      |

See [cli/README.md](cli/README.md) for the full flag reference.

---

## 🏗️ Architecture at a Glance

The diagram below shows the write path: a CLI command loads the database definition, validates
records, rebuilds materialized views, and leaves every change as a normal git commit.

```mermaid
flowchart LR
    A([ingitdb CLI]) --> B[validator.ReadDefinition]
    B --> C{Schema valid?}
    C -- yes --> D[DataValidator\nwalks $records/]
    C -- no --> E([Exit 1 — schema error])
    D --> F{Data valid?}
    F -- no --> G([Exit 1 — data errors])
    F -- yes --> H[ViewsBuilder\ngenerates $views/]
    H --> I[(Git repository\non disk)]
```

The same Git repository is also exposed to AI agents via an MCP server (Phase 6), and to any
Go program through the [DALgo](https://github.com/dal-go/dalgo) abstraction layer implemented
in `pkg/dalgo2ingitdb`.

---

## 🙋 Get Involved

The best way to understand inGitDB is to point it at a directory of YAML files and run
`ingitdb validate`. From there, check the [Roadmap](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/ROADMAP.md) to see what is being built and
the [Backlog](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/BACKLOG.md) for tasks that are ready to be picked up.

To contribute:

1. Read [CONTRIBUTING.md](CONTRIBUTING.md) for the pull-request workflow.
2. Read [CODING_STANDARDS.md](CODING_STANDARDS.md) before writing any Go.
3. Open or comment on an [issue](https://github.com/ingitdb/ingitdb-cli/issues) to discuss your
   idea before investing time in a large change.

Bug reports, documentation improvements, and questions are all welcome. The project is small
enough that every contribution makes a visible difference.
