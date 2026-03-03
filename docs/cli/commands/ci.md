# `ci` — run CI checks for the database

[Source Code](../../../cmd/ingitdb/commands/ci.go)

Runs all CI checks for an inGitDB database.  Currently this executes
[`materialize`](materialize.md), rebuilding every materialised view.

Future versions will add CI-specific optimisations — for example, when
running on a pull-request diff, only the collections and views affected by
changed files will be validated and materialised.

## Usage

```
ingitdb ci [--path=PATH] [--views=LIST] [--records-delimiter=N]
```

## Flags

| Flag                    | Description                                                                                                                                              |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--path=PATH`           | Path to the database directory. Defaults to the current working directory.                                                                               |
| `--views=LIST`          | Comma-separated list of view names to materialise. Without this flag, all views are materialised.                                                        |
| `--records-delimiter=N` | Override the `#-` delimiter behaviour for INGR output. `1` = enabled, `-1` = disabled, `0` or omitted = use view/project default (app default is `1`). |

## Examples

```shell
# Run CI checks in the current directory
ingitdb ci

# Run CI checks for a database at a specific path
ingitdb ci --path=/var/db/myapp
```
