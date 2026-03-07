# CLI Architecture

For the **storage format and data model** specification, see [STORAGE_FORMAT.md](https://github.com/ingitdb/ingitdb-specs/blob/main/docs/STORAGE_FORMAT.md) in [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs).

## Component Architecture

```
CLI (cmd/ingitdb)
    │
    ├── cmd/ingitdb/commands  ← one file per top-level command
    │       │
    │       ├── validate [--path] [--from-commit] [--to-commit]
    │       │       └── validator.ReadDefinition()
    │       │               ├── config.ReadRootConfigFromFile()     reads .ingitdb/root-collections.yaml
    │       │               ├── readCollectionDef() × N             reads .definition.yaml per collection
    │       │               └── colDef.Validate()                   validates schema structure
    │       │               └── [TODO] DataValidator                walks $records/, validates records against schema
    │       │
    │       ├── query --collection [--path] [--format]
    │       │       └── [TODO] Query engine                         reads and filters records, formats output
    │       │
    │       ├── materialize [--path] [--views]
    │       │       └── [TODO] Views Builder                        reads view defs, generates $views/ output
    │       │
    │       ├── list (collections|view|subscribers) [--in] [--filter-name]
    │       ├── find [--substr] [--re] [--exact] [--in] [--fields] [--limit]
    │       ├── delete (collection|view|records) [--collection|--view]
    │       ├── truncate --collection
    │       │
    │       └── [TODO] Subscribers/Triggers
    │               └── dispatches events (webhook, email, shell) on data changes
    │
    └── cmd/ingitdb/main.go   ← wiring: assembles commands, injects dependencies, handles exit
```

The **Scanner** (see `docs/components/scanner.md` in [ingitdb-specs](https://github.com/ingitdb/ingitdb-specs)) orchestrates the full pipeline: it walks the filesystem and invokes the Validator and Views Builder in sequence.

## Package Map

| Package                 | Responsibility                                                                                                                                                                    |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/ingitdb`           | Domain types only: `Definition`, `CollectionDef`, `ColumnDef`, `ViewDef`, etc. No I/O.                                                                                            |
| `pkg/ingitdb/config`    | Reads `.ingitdb/root-collections.yaml` and `.ingitdb/settings.yaml` (root config) and `~/.ingitdb/.ingitdb-user.yaml` (user config).                                              |
| `pkg/ingitdb/validator` | Reads and validates collection schemas. Entry point: `ReadDefinition()`.                                                                                                          |
| `pkg/dalgo2ingitdb`     | DALgo integration: implements `dal.DB`, read-only and read-write transactions.                                                                                                    |
| `cmd/ingitdb/commands`  | One file per CLI command. Each exports a single `*cli.Command` constructor. Subcommands are unexported functions named after the subcommand (parent-prefixed on name collisions). |
| `cmd/ingitdb`           | CLI entry point only: assembles the `commands` slice, injects dependencies, and handles process exit.                                                                             |

## Key Design Decisions

**Commands package.** Each top-level CLI command lives in its own file under `cmd/ingitdb/commands/` and exposes a single exported constructor (e.g. `Validate(...)`, `List()`, `Find()`). Subcommands are unexported functions whose names match the subcommand name; when the same subcommand name appears under multiple parents (e.g. `view` in both `list` and `delete`), the function is prefixed with the parent name (`listView`, `deleteView`). `cmd/ingitdb/main.go` is reduced to wiring and process-level concerns.

**Subcommand-based CLI.** Commands are implemented using `github.com/urfave/cli/v3`. `--path` is a per-subcommand flag defaulting to the current working directory. See `docs/cli/README.md` for the full interface spec.

**Stdout reserved for data output.** All diagnostic output (logs, errors) goes to `os.Stderr`. Stdout carries only structured data (query results) or TUI output. This allows piping without mixing logs into results.

**Dependency injection in `run()`.** `homeDir`, `readDefinition`, `fatal`, and `logf` are injected as function parameters, making the CLI fully unit-testable without real I/O.

**DALgo abstraction.** `pkg/dalgo2ingitdb` implements the DALgo `dal.DB` interface so consumers can work with inGitDB through a standard Go database abstraction, decoupled from the file-based storage format.

**Two validation modes.** Full validation scans the entire DB. Change validation validates only the files changed between two git commits — essential for keeping CI fast on large databases.

**No package-level variables.** All dependencies are passed via struct fields or function parameters to keep code testable and avoid global state.
