# `ingitdb rebase`

Rebase the current branch on top of a base reference, automatically handling common inGitDB-specific conflicts.

The following types of files can have their conflicts automatically resolved by `ingitdb`:

- `README.md` files (for collections, views, triggers, etc.)
- Materialized views
- Data indexes

## ⚙️ Usage

```shell
ingitdb rebase [--base_ref=REF] [--resolve=FILES]
```

## 🚩 Flags

| Flag         | Description                                                                                                                                    |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| `--base_ref` | Git branch or commit to rebase onto. If omitted, falls back to `BASE_REF` or `GITHUB_BASE_REF` environment variables.                          |
| `--resolve`  | Comma-separated list of file names to auto-resolve during merge conflicts. Defaults to none. In internal tooling, setting to `readme` is used. |

## 📖 Description

The `rebase` command runs a `git rebase <base_ref>` operation under the hood. If a git conflict is encountered during the rebase procedure, `ingitdb` performs an automatic check to determine if the conflict can be safely resolved without human intervention.

### Automatic Conflict Detection & Resolution

When `git rebase` stops due to a conflict, `ingitdb` evaluates the unmerged files:

1. It executes `git diff --name-only --diff-filter=U`.
2. It verifies if the conflicting files are safe to overwrite. Specifically, if the conflict is **solely** within collection `README.md` files (and the `--resolve=readme` flag is provided), the CLI automatically resolves the issue by internally invoking the same logic used by the `docs update` command.
3. The shared internal function detects the specific collections the `README.md` files belong to, and **knows to update only the problematic ones** instead of regenerating the entire database documentation.
   - 🔗 **See Implementation:** The logic for matching conflicted files to their respective collections is implemented in [`docsbuilder.FindCollectionsForConflictingFiles`](../../pkg/ingitdb/docsbuilder/update.go).
4. `ingitdb` stages the auto-resolved files (`git add`) and resumes the operation with `git rebase --continue`.

If any conflicts exist outside of the explicitly resolvable files (e.g., in a `collection.yaml` or a Golang source file), the `rebase` command will immediately abort and list the unresolved files, requiring manual resolution by the user.
