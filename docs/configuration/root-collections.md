# ⚙️ inGitDB Repository Configuration - Root Collections

```yaml
# ⚙️ Each entry maps exactly one collection ID to one collection directory.
rootCollections:
  countries: geo/countries
  todo.*: todo
  agile.*: docs/demo-apps/agile-ledger
```

## Namespace Imports

You can import all collections from another directory's `.ingitdb.yaml` file using the `.*` suffix.
The prefix before `.*` is prepended to each imported collection ID.

```yaml
rootCollections:
  agile.*: docs/demo-apps/agile-ledger
```

If `docs/demo-apps/agile-ledger/.ingitdb.yaml` contains:

```yaml
rootCollections:
  teams: teams
```

Then the result is equivalent to:

```yaml
rootCollections:
  agile.teams: docs/demo-apps/agile-ledger/teams
```

### Path resolution

The path value for a namespace import can be:

- **Relative** — resolved relative to the directory containing the current `.ingitdb.yaml` file.
- **Absolute** — used as-is.
- **`~` prefixed** — `~` is expanded to the user's home directory (e.g. `~/my-data/collections`).

### Error handling

An error is returned when:

- The referenced directory does not exist.
- The referenced directory has no `.ingitdb.yaml` file.
- The referenced `.ingitdb.yaml` has no or empty `rootCollections`.

## `default_namespace`

The optional `default_namespace` field specifies a namespace prefix for collections
when the DB is opened directly (not imported via a namespace import).

For example, if `docs/demo-apps/todo/.ingitdb.yaml` contains:

```yaml
default_namespace: todo

rootCollections:
  statuses: statuses
  tags: tags
  tasks: tasks
```

Then when this DB is opened directly, the collections are presented as
`todo.statuses`, `todo.tags`, and `todo.tasks`.

When imported via a namespace import (e.g. `todo.*: docs/demo-apps/todo`),
the import alias is used instead of `default_namespace`.

# 📘 Examples:

- [/.ingitdb.yaml](../../.ingitdb.yaml) - config for inGitDB in this repository.
- [/docs/demo-apps/todo/.ingitdb.yaml](../../docs/demo-apps/todo/.ingitdb.yaml) - todo collections with `default_namespace`.
- [/docs/demo-apps/agile-ledger/.ingitdb.yaml](../../docs/demo-apps/agile-ledger/.ingitdb.yaml) - agile-ledger with `default_namespace`.

Collection IDs (the keys in `rootCollections`) must use only alphanumeric characters and `.`,
and must start and end with an alphanumeric character.

Paths in `rootCollections` must point to a single collection directory and cannot use wildcards
such as `*`. This keeps collection IDs explicit and allows GitHub-backed commands to read only
`.ingitdb.yaml` (without extra directory listing API calls), reducing latency.
