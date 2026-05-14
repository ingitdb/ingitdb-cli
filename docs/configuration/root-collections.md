# ⚙️ inGitDB Repository Configuration — Root Collections

`.ingitdb/root-collections.yaml` is a flat YAML map. Each entry maps exactly one collection ID
to one collection directory path. No wrapper key is needed.

```yaml
# .ingitdb/root-collections.yaml

companies: data/companies

# Namespace import: imports all root collections from another .ingitdb/root-collections.yaml
# with the key prefix (e.g., todo.* imports todo.statuses, todo.tags, todo.tasks)
todo.*: data/todo
agile.*: data/agile-ledger
```

## Namespace Imports

You can import all collections from another directory's `.ingitdb/root-collections.yaml` file
using the `.*` suffix. The prefix before `.*` is prepended to each imported collection ID.

```yaml
# .ingitdb/root-collections.yaml
agile.*: data/agile-ledger
```

If `data/agile-ledger/.ingitdb/root-collections.yaml` contains:

```yaml
teams: teams
```

Then the result is equivalent to:

```yaml
agile.teams: data/agile-ledger/teams
```

### Path resolution

The path value for a namespace import can be:

- **Relative** — resolved relative to the directory containing the current
  `.ingitdb/root-collections.yaml` file.
- **Absolute** — used as-is.
- **`~` prefixed** — `~` is expanded to the user's home directory
  (e.g. `~/my-data/collections`).

### Error handling

An error is returned when:

- The referenced directory does not exist.
- The referenced directory has no `.ingitdb/root-collections.yaml` file.
- The referenced `.ingitdb/root-collections.yaml` is empty.

## `default_namespace`

The optional `default_namespace` field in `.ingitdb/settings.yaml` specifies a namespace prefix
for collections when the DB is opened directly (not imported via a namespace import).

For example, if `data/todo/.ingitdb/settings.yaml` contains:

```yaml
default_namespace: todo
```

And `data/todo/.ingitdb/root-collections.yaml` contains:

```yaml
statuses: statuses
tags: tags
tasks: tasks
```

Then when this DB is opened directly, the collections are presented as
`todo.statuses`, `todo.tags`, and `todo.tasks`.

When imported via a namespace import (e.g. `todo.*: data/todo`),
the import alias is used instead of `default_namespace`.

# 📘 Examples

Worked examples live in the sibling
[demo-ingitdb](https://github.com/ingitdb/demo-ingitdb) repository, which hosts
the canonical demo databases (commerce, geo, todo, etc.) used in documentation
and end-to-end tests.

---

Collection IDs (the keys in `root-collections.yaml`) must use only alphanumeric characters and
`.`, and must start and end with an alphanumeric character.

Paths must point to a single collection directory and cannot use wildcards such as `*`.
This keeps collection IDs explicit and allows GitHub-backed commands to read only
`.ingitdb/root-collections.yaml` (without extra directory-listing API calls), reducing latency.
