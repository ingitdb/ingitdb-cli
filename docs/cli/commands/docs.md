# 📝 docs

Manage documentation.

## 🔸 docs update

```shell
ingitdb docs update [--path=PATH] [--collection=ID]
```

Updates the `README.md` documentation file based on metadata for a given collection, including its subcollections if specified using a glob pattern.

If the generated content has not changed compared to the existing `README.md` file, the file is not modified.

| Flag              | Description                                                                  |
| ----------------- | ---------------------------------------------------------------------------- |
| `--collection=ID` | Collection ID or glob pattern to use. E.g. `teams`, `agile.teams/*`, or `**` |
| `--path=PATH`     | Database directory path. Defaults to the current directory if omitted.       |

### Examples

#### Update specific collection only

```shell
ingitdb docs update --collection agile.teams
```

#### Update specific collection and its direct subcollections

```shell
ingitdb docs update --collection "agile.teams/*"
```

#### Update all collections starting from a specific collection (recursive)

```shell
ingitdb docs update --collection "agile.teams/**"
```

#### Update all collections in the entire database

```shell
ingitdb docs update --collection "**"
```
