### 🔹 `demo install` — create the TODO demo database

[Source Code](../../../cmd/ingitdb/commands/demo_install.go) · [Specification](../../../spec/features/cli/demo/README.md)

```
ingitdb demo install [--path=PATH] [--format=yaml|json]
```

Creates the TODO demo: two lists, **To buy** (Milk, Bananas, Coffee) and **To watch**
(The Matrix, Interstellar), stored as readable YAML files in a new Git repository. It is
the same demo OpenVaultDB's `ovdb demo install` creates: the records come from the shared
Go package `github.com/ingitdb/ingitdb-go/ingitdb/demos/todo`.

| Flag                   | Description                                                                                          |
| ---------------------- | ---------------------------------------------------------------------------------------------------- |
| `--path=PATH`          | Folder to create the demo in. Relative paths resolve against the current directory. Default: `./todo-demo`. |
| `--format=yaml\|json`  | Print one structured document instead of text.                                                        |

`ingitdb demo` without a subcommand prints help.

**What it does**

1. Checks the folder: it must not exist or be empty. A file, a non-empty folder or a folder
   holding another demo is refused, nothing is written, and the error suggests
   `ingitdb demo install --path=<another folder>`.
2. Runs `git init` in the folder (when `git` is on `PATH`) before writing anything, so no
   write lands in an enclosing repository.
3. Writes `.ingitdb/settings.yaml`, `.ingitdb/root-collections.yaml`, the definitions of the
   `lists` collection and its `items` subcollection, the `.ingitdb/demo.yaml` marker and one
   file per record, for example `lists/$records/to-buy.yaml` and
   `lists/to-buy/items/$records/milk.yaml`.
4. Makes one commit, `Install the TODO demo`. When Git has no `user.name` or `user.email`,
   the missing part is taken from `inGitDB <ingitdb@localhost>` for that commit only; Git
   configuration is not changed.

If a step fails, everything written is removed (the folder itself when the command created
it). Without `git`, the files are still installed and the output says to run `git init`.

Running it again on an installed demo writes nothing, keeps your edits and prints
`The TODO demo is already installed in <folder>` with the same next steps. The command never
prompts.

**Output**

```
The TODO demo is ready

Lists:     lists/to-buy, lists/to-watch
Stored in: /home/ada/todo-demo
Git:       a Git repository, commit 34560e6

What next?
  1. Browse the lists in the terminal UI
       ingitdb --path=/home/ada/todo-demo
  2. Query the lists
       ingitdb select --from=lists --path=/home/ada/todo-demo
  3. Query what to buy
       ingitdb select --from=lists/to-buy/items --path=/home/ada/todo-demo
  4. Use the lists in a web TODO app (OpenVaultDB)
       ovdb demo install --yes
       ovdb demo open
       OpenVaultDB keeps its own copy of the same lists. Install ovdb from https://github.com/openvaultdb/ovdb
```

Paths with spaces or shell-special characters are quoted for your shell: single quotes on
Linux and macOS, double quotes on Windows (works in `cmd.exe` and PowerShell).

With `--format=json` (or `yaml`) stdout is a single document with `app`, `installed`,
`already_installed`, `path`, `git` (`repository`, `commit`), `lists` and `next` (a list of
`label` and `command`, plus `note` on the OpenVaultDB step). Errors go to stderr with a
non-zero exit code.

**Examples:**

```shell
# 📘 Create ./todo-demo and browse it
ingitdb demo install
ingitdb --path=todo-demo

# 🔁 Create the demo somewhere else
ingitdb demo install --path=~/lists

# 🤖 Structured result for scripts and agents
ingitdb demo install --format=json
```

---
