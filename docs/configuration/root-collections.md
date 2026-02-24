# ⚙️ inGitDB Repository Configuration - Root Collections

```yaml
# ⚙️ Each entry maps exactly one collection ID to one collection directory.
rootCollections:
  countries: geo/countries
  todo.tags: todo/tags
  todo.tasks: todo/tasks
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

# 📘 Examples:

- [/.ingitdb.yaml](../../.ingitdb.yaml) - config for inGitDB in this repository.

Collection IDs (the keys in `rootCollections`) must use only alphanumeric characters and `.`,
and must start and end with an alphanumeric character.

Paths in `rootCollections` must point to a single collection directory and cannot use wildcards
such as `*`. This keeps collection IDs explicit and allows GitHub-backed commands to read only
`.ingitdb.yaml` (without extra directory listing API calls), reducing latency.
