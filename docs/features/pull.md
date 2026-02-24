# 📘 inGitDB Pull

`ingitdb pull` is a single command that performs a complete pull cycle for an inGitDB database.

## 📂 Status

Pending (Phase 3).

## 📂 What it does

1. **Pulls latest changes** from the remote via `git pull --rebase` (or `--merge`).
2. **Auto-resolves generated file conflicts** — materialized views and `README.md` files are regenerated rather than manually merged.
3. **Resolves data file conflicts interactively** — opens the field-by-field TUI for any source record files that have conflicts requiring a human decision.
4. **Rebuilds materialized views and README.md** if the incoming changes affect them.
5. **Prints a change summary** listing every record added, updated, or deleted by the pull.

## 📂 Why

Running `git pull` + `ingitdb resolve` + `ingitdb materialize` manually is repetitive and error-prone. `ingitdb pull` automates the full cycle in one command.

## 🖥️ CLI

```
ingitdb pull [--path=PATH] [--strategy=rebase|merge] [--remote=REMOTE] [--branch=BRANCH]
```

| Flag | Description |
|------|-------------|
| `--path=PATH` | Path to the database directory. Defaults to the current working directory. |
| `--strategy=rebase\|merge` | Git pull strategy. Default: `rebase`. |
| `--remote=REMOTE` | Remote to pull from. Default: `origin`. |
| `--branch=BRANCH` | Branch to pull. Default: the current branch's tracking branch. |

See [CLI reference](../cli/README.md#pull--pull-latest-changes-resolve-conflicts-and-rebuild-views-not-yet-implemented).

## 📂 Change summary format

```
Pulled 3 commits from origin/main (rebase)

  Records added:   2
    + /countries/de/cities/berlin
    + /countries/fr/cities/paris

  Records updated: 1
    ~ /countries/gb/cities/london  (2 fields: population, area)

  Records deleted: 0
```

## 📂 Exit codes

| Code | Meaning |
|------|---------|
| `0` | Pull completed; all conflicts resolved; views rebuilt. |
| `1` | Unresolved conflicts remain after interactive resolution. |
| `2` | Infrastructure error (git not found, network failure, bad flags). |

## 📂 Related

- [Merge Conflict Resolution](merge-conflict-resolution.md) — the resolver used by `pull` for both generated and data files.
- [Watcher](watcher.md) — for real-time change events without a git pull.
