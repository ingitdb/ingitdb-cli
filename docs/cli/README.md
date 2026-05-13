# 🖥️ ingitdb Command Line Interface

`--path` defaults to the current working directory when omitted. `~` in paths is expanded to the user's home directory.

## 📂 Global flags

| Flag           | Description                       |
| -------------- | --------------------------------- |
| `--help`, `-h` | Print usage information and exit. |

## 🖥️ Commands Index

- [version](commands/version.md) — print build information
- [validate](commands/validate.md) — validate database schema and data
- [select](commands/select.md) — read a single record (`--id`) or query a set of records (`--from`/`--where`)
- [insert](commands/insert.md) — create a new record
- [update](commands/update.md) — patch fields of one or more existing records
- [delete](commands/delete.md) — delete one or more records
- [drop](commands/drop.md) — drop a collection or view
- [truncate](commands/truncate.md) — remove all records from a collection
- [materialize](commands/materialize.md) — build generated files from records
- [ci](commands/ci.md) — run CI checks for the database (currently: materialize views)
- [pull](commands/pull.md) — pull latest changes, resolve conflicts, and rebuild views
- [setup](commands/setup.md) — initialise a new database directory
- [resolve](commands/resolve.md) — resolve merge conflicts in database files
- [watch](commands/watch.md) — watch database for changes
- [serve](commands/serve.md) — start one or more servers
- [list](commands/list.md) — list database objects
- [find](commands/find.md) — search records by value
- [migrate](commands/migrate.md) — migrate data between schema versions
- [rebase](commands/rebase.md) — rebase on top of a base ref and auto-resolve specific documentation conflicts
