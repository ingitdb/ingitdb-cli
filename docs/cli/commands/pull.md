### 🧾 pull` — pull latest changes, resolve conflicts, and rebuild views _(not yet implemented)_

[Source Code](../../../cmd/ingitdb/commands/pull.go)


```
ingitdb pull [--path=PATH] [--strategy=rebase|merge] [--remote=REMOTE] [--branch=BRANCH]
```

| Flag                       | Description                                                                |
| -------------------------- | -------------------------------------------------------------------------- |
| `--path=PATH`              | Path to the database directory. Defaults to the current working directory. |
| `--strategy=rebase\|merge` | Git pull strategy. Default: `rebase`.                                      |
| `--remote=REMOTE`          | Remote to pull from. Default: `origin`.                                    |
| `--branch=BRANCH`          | Branch to pull. Default: the current branch's tracking branch.             |

Performs a complete pull cycle in one command:

1. `git pull --rebase` (or `--merge`) from the specified remote and branch.
2. Auto-resolves any conflicts in generated files (materialized views, `README.md`) by regenerating them.
3. Opens an interactive TUI for any conflicts in source data files that require a human decision.
4. Rebuilds materialized views and `README.md` if new changes require it.
5. Prints a summary of records added, updated, and deleted by the pull.

Exits `0` if all conflicts were resolved and views rebuilt successfully. Exits `1` if unresolved conflicts remain after interactive resolution. Exits `2` on infrastructure errors (git not found, network failure, bad flags).

**Examples:**

```shell
# 📘 Pull from origin using the default rebase strategy
ingitdb pull

# 📘 Pull using merge instead of rebase
ingitdb pull --strategy=merge

# 🔁 Pull from a specific remote and branch
ingitdb pull --remote=upstream --branch=main
```

---

