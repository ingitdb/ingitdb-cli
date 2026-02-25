# 🖥️ ingitdb Command Line Interface

`--path` defaults to the current working directory when omitted. `~` in paths is expanded to the user's home directory.

## 📂 Global flags

| Flag           | Description                       |
| -------------- | --------------------------------- |
| `--help`, `-h` | Print usage information and exit. |

## 🖥️ Commands Index

- [version](commands/version.md) — print build information
- [validate](commands/validate.md) — validate database schema and data
- [read](commands/read.md) — read a single record
- [create](commands/create.md) — create a new record
- [update](commands/update.md) — update fields of an existing record
- [query](commands/query.md) — query records from a collection _(not yet implemented)_
- [materialize](commands/materialize.md) — build generated files from records _(not yet implemented)_
- [pull](commands/pull.md) — pull latest changes, resolve conflicts, and rebuild views _(not yet implemented)_
- [setup](commands/setup.md) — initialise a new database directory _(not yet implemented)_
- [resolve](commands/resolve.md) — resolve merge conflicts in database files _(not yet implemented)_
- [watch](commands/watch.md) — watch database for changes _(not yet implemented)_
- [serve](commands/serve.md) — start one or more servers _(not yet implemented)_
- [list](commands/list.md) — list database objects
- [find](commands/find.md) — search records by value _(not yet implemented)_
- [delete](commands/delete.md) — delete database objects
- [truncate](commands/truncate.md) — remove all records from a collection _(not yet implemented)_
- [migrate](commands/migrate.md) — migrate data between schema versions _(not yet implemented)_
- [rebase](commands/rebase.md) — rebase on top of a base ref and auto-resolve specific documentation conflicts
