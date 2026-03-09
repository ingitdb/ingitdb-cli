### 🔀 diff` — show record-level changes between two git refs _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/diff.go)

```
ingitdb diff [<ref> | <ref>..<ref> | <commit>]
             [--path=PATH]
             [--collection=KEY | --view=VIEW_KEY [--view-mode=output|source]]
             [--path-filter=PATTERN]
             [--depth=summary|record|fields|full]
             [--format=text|json|yaml|toml]
```

Compares two git reference points and reports inGitDB record-level changes: which records were added, updated, or deleted, grouped by collection. Per-record output is annotated with the number of commits and short commit hashes that touched each record between the two refs.

| Flag | Required | Description |
|------|----------|-------------|
| `<ref>` | no | Compare `<ref>` against `HEAD` — primary use case (e.g. `ingitdb diff main`). |
| `<ref>..<ref>` | no | Compare two explicit refs (branches, tags, commits). |
| `--path=PATH` | no | Path to the database directory. Defaults to the current working directory. |
| `--collection=KEY` | no | Limit to one collection (dot-notation). Mutually exclusive with `--view`. |
| `--view=VIEW_KEY` | no | Limit to a named materialized view. Mutually exclusive with `--collection`. |
| `--view-mode=output\|source` | no | With `--view`: diff the generated output file (`output`, default) or the source records feeding it (`source`). |
| `--path-filter=PATTERN` | no | Limit to records whose path matches the prefix or glob (e.g. `countries/ie/*`). |
| `--depth=summary\|record\|fields\|full` | no | Detail level (see below). Default: `summary`. |
| `--format=text\|json\|yaml\|toml` | no | Output format. Default: `text`. |

**Depth levels:**

| `--depth` | Output |
|-----------|--------|
| `summary` _(default)_ | One line per collection/view with added/updated/deleted counts. |
| `record` | One line per record with change type, commit count, and short commit hashes. |
| `fields` | Per record + list of field names that changed. |
| `full` | Per record + before and after value for each changed field. |

Exits `0` when no changes are found, `1` when changes are found (suitable for CI guards), `2` on error.

**Examples:**

```shell
# 🔀 Compare current HEAD against main (primary use case)
ingitdb diff main

# 🔀 Compare against a remote tracking branch
ingitdb diff origin/main

# 🔀 Diff working tree against HEAD (uncommitted changes)
ingitdb diff

# 🔀 Compare two explicit branches
ingitdb diff main..feature/add-regions

# 🔀 See what changed in the last 5 commits
ingitdb diff HEAD~5

# 📦 Limit to a single collection
ingitdb diff main --collection=countries.cities

# 🗂️ Limit by path prefix
ingitdb diff main --path-filter=countries/ie/*

# 📋 Per-record list with commit annotations
ingitdb diff main --depth=record

# 🔍 Show which fields changed (no values)
ingitdb diff main --depth=fields

# 🔬 Show full before/after field values
ingitdb diff main --depth=full

# 📄 Diff a view's generated output
ingitdb diff main --view=top-cities-by-population

# 📄 Diff the source records feeding a view
ingitdb diff main --view=top-cities-by-population --view-mode=source

# 🤖 JSON output for scripting
ingitdb diff main --format=json

# 🤖 JSON with full field values
ingitdb diff main --depth=full --format=json

# ✅ Use in CI — fail if records in a collection changed
ingitdb diff origin/main --collection=countries && echo "No changes"
```

---

