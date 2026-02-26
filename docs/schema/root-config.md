# ⚙️ Root Configuration Schema (`.ingitdb/`)

> **Frontend client builders and AI agents: start here.**
> This document is the authoritative schema reference for the inGitDB repository-level
> configuration directory.

The `.ingitdb/` directory lives at the root of every inGitDB-enabled repository. All files
inside it are **optional** — an absent directory (or an empty one) is a valid, empty inGitDB
instance.

```
<repo-root>/
└── .ingitdb/
    ├── root-collections.yaml   # collection ID → path map  (optional)
    ├── settings.yaml           # default_namespace, languages  (optional)
    └── README.md               # human-readable overview & stats  (doc only)
```

---

## 📄 `root-collections.yaml`

### Purpose

Maps collection IDs to their directory paths. Also supports **namespace imports** — pulling
the entire collection map from a sub-directory's own `.ingitdb/root-collections.yaml`.

### Format

A **flat YAML map** — no wrapper key, no nesting.

```yaml
# .ingitdb/root-collections.yaml

# Plain collection entry
companies: demo-dbs/test-db/companies

# Namespace import (.*  suffix) — imports all collections from the sub-directory,
# prefixing each imported ID with the key before ".*"
todo.*: demo-dbs/todo
agile.*: demo-dbs/agile-ledger
```

### Collection ID rules

| Rule                        | Detail                                                  |
| --------------------------- | ------------------------------------------------------- |
| Allowed characters          | Alphanumeric (`a-z`, `A-Z`, `0-9`) and `.`             |
| Must start and end with     | An alphanumeric character                               |
| Namespace import suffix     | `.*` — e.g. `todo.*`                                   |
| Case sensitivity            | IDs are case-sensitive                                  |

### Path values

| Format       | Behaviour                                                          |
| ------------ | ------------------------------------------------------------------ |
| Relative     | Resolved relative to the directory containing `root-collections.yaml` |
| Absolute     | Used as-is                                                         |
| `~/…`        | `~` is expanded to the user's home directory                       |

Paths must point to a **single collection directory**. Glob wildcards (`*`) are not allowed
in path values (only in collection ID keys, as the `.*` namespace-import syntax).

### Namespace imports

A key ending in `.*` triggers a namespace import. inGitDB reads
`.ingitdb/root-collections.yaml` inside the referenced directory and re-exports every
collection it finds, prepending the key prefix (minus `.*`).

**Example:**

```yaml
# This repo's .ingitdb/root-collections.yaml
todo.*: demo-dbs/todo
```

```yaml
# demo-dbs/todo/.ingitdb/root-collections.yaml
statuses: statuses
tags: tags
tasks: tasks
```

**Effective result:**

```yaml
todo.statuses: demo-dbs/todo/statuses
todo.tags:     demo-dbs/todo/tags
todo.tasks:    demo-dbs/todo/tasks
```

#### Error conditions

| Condition                                                              | Behaviour    |
| ---------------------------------------------------------------------- | ------------ |
| Referenced directory does not exist                                    | Returns error |
| Referenced directory has no `.ingitdb/root-collections.yaml`          | Returns error |
| Referenced `.ingitdb/root-collections.yaml` is empty                  | Returns error |

### Design rationale

Keeping collection IDs explicit (no server-side directory listing) means a GitHub-backed
client can resolve the entire collection map by reading a single file per level — no extra
API calls, lower latency.

---

## 📄 `settings.yaml`

### Purpose

Repository-level settings: default namespace prefix and supported languages.

### Schema

```yaml
# .ingitdb/settings.yaml

# Optional. Namespace prefix applied when this DB is opened directly
# (not via a namespace import from a parent repo).
default_namespace: todo

# Optional. Ordered list of supported languages.
languages:
  - required: en      # at least one required language is recommended
  - optional: fr
  - optional: es
  - optional: ru
```

### Fields

#### `default_namespace`

| Property | Detail                          |
| -------- | ------------------------------- |
| Type     | `string`                        |
| Required | No                              |
| Format   | Valid collection ID prefix (alphanumeric + `.`) |

When set, every collection in this DB is exposed with this prefix when the DB is opened
directly. When the DB is **imported** via a namespace import in a parent repo, the import
alias overrides `default_namespace`.

**Example:** `default_namespace: todo` makes `statuses` appear as `todo.statuses`.

#### `languages`

| Property | Detail                                          |
| -------- | ----------------------------------------------- |
| Type     | `array` of language entries                     |
| Required | No                                              |

Each entry is a single-key map with either `required` or `optional` as the key and a
language code as the value.

```yaml
languages:
  - required: en        # required language — must always be present in records
  - required: es-MX     # required regional variant
  - optional: fr        # optional — may be absent in a record without error
  - optional: ru
```

##### Language code format

Prefer **ISO 639-1** two-letter codes (`en`, `fr`, `es`).
**IETF BCP 47** (RFC 5646 / RFC 4647) regional variants are also accepted (`es-MX`, `es-ES`,
`zh-Hant`).

##### Ordering rules

1. All `required` entries must appear **before** any `optional` entries.
2. The language selector in UIs shows languages in the order they appear in this list.

---

## 📄 `.ingitdb/README.md`

This file is **documentation only** — inGitDB tooling does not parse it.
It is intended for human readers browsing the repository and for auto-generated summaries.

Recommended sections:

| Section       | Content                                                               |
| ------------- | --------------------------------------------------------------------- |
| **Overview**  | Short description of what this inGitDB instance stores               |
| **Stats**     | Collection count, record count — to be auto-generated by tooling     |

---

## 📘 Live examples

| File                                                                                                       | Notes                                        |
| ---------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| [`.ingitdb/root-collections.yaml`](../../.ingitdb/root-collections.yaml)                                   | This repo — namespace imports in action      |
| [`.ingitdb/settings.yaml`](../../.ingitdb/settings.yaml)                                                   | This repo — multi-language setup             |
| [`demo-dbs/todo/.ingitdb/root-collections.yaml`](../../demo-dbs/todo/.ingitdb/root-collections.yaml) | Simple flat map, three collections     |
| [`demo-dbs/todo/.ingitdb/settings.yaml`](../../demo-dbs/todo/.ingitdb/settings.yaml)           | `default_namespace: todo`                    |
| [`demo-dbs/agile-ledger/.ingitdb/root-collections.yaml`](../../demo-dbs/agile-ledger/.ingitdb/root-collections.yaml) | Single collection |
| [`demo-dbs/agile-ledger/.ingitdb/settings.yaml`](../../demo-dbs/agile-ledger/.ingitdb/settings.yaml) | `default_namespace` + one required language |

---

## 🔗 Related docs

- [Configuration overview](../../docs/configuration/README.md)
- [Root collections reference](../../docs/configuration/root-collections.md)
- [Languages reference](../../docs/configuration/languages.md)
- [Collection-level schema](collection.md) — the `.collection/definition.yaml` file inside
  each collection directory
- [Schema definitions index](README.md)
