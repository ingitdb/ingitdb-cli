### 🧾 materialize` — regenerate derived files from records

[Source Code](../../../cmd/ingitdb/commands/materialize.go)

A single flat command that regenerates inGitDB's derived artifacts: per-collection
`README.md` files and materialized view files under each view's configured `$views/`
directory. Run with no flags it regenerates **everything**; the `--collections` and
`--views` flags narrow what is regenerated. Files are written only when their content
differs from what is already on disk, so repeated runs are idempotent.

```
ingitdb materialize [--collections[=GLOB[,GLOB...]]] [--views[=GLOB[,GLOB...]]] [--records-delimiter=N] [--path=PATH]
```

#### Selection flags

`--collections` and `--views` are each **tri-state**:

- **absent** — that artifact type is not touched.
- **bare flag** (`--collections`) — every artifact of that type.
- **with a value** (`--collections=GLOB[,GLOB...]`) — only the matching ones.

Running `ingitdb materialize` with **neither** flag regenerates all collection READMEs
**and** all views.

> ⚠️ Because the bare flag means "all", a value MUST be attached with `=` — use
> `--views=v1,v2`, **not** `--views v1,v2` (the space-separated form is parsed as a
> positional argument).

| Flag                    | Description                                                                                                                                                |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `--collections[=GLOB]`  | Regenerate collection `README.md` files. Bare = all collections; `=GLOB[,GLOB]` = matching collection IDs. Patterns are separated by `,` (canonical) or `;`. |
| `--views[=GLOB]`        | Regenerate materialized views. Bare = all views; `=GLOB[,GLOB]` = matching view names. Same separators as `--collections`.                                  |
| `--records-delimiter=N` | Override the `#-` delimiter behaviour for INGR **view** output. `1` = enabled, `-1` = disabled, `0` or omitted = use view/project default (app default `1`). No effect when only collections are regenerated. |
| `--path=PATH`           | Path to the database directory. Defaults to the current working directory.                                                                                |

Glob semantics match those used by collection targeting elsewhere: `**` (all), `path/*`
(direct subcollections), `path/**` (recursive), or an exact ID. View output is written
into the `$views/` directory defined in each view's definition. A summary of
created/updated/deleted/unchanged files is printed to stderr; stdout stays silent for
scriptability.

**Examples:**

```shell
# 🧾 Regenerate everything (all collection READMEs + all views)
ingitdb materialize

# 📄 All collection READMEs only
ingitdb materialize --collections

# 🔁 All views only
ingitdb materialize --views

# 🔁 Specific views (note the '=')
ingitdb materialize --views=by_status,by_assignee

# 📄 Specific collections, including nested ones, via globs
ingitdb materialize --collections='agile.teams/**,countries'

# 🎯 Mix: one view plus two collection READMEs in one run
ingitdb materialize --views=by_status --collections=countries,teams

# 🔁 Target a database at a specific path
ingitdb materialize --path=/var/db/myapp
```

> ℹ️ Collection-README regeneration was previously a separate command,
> [`docs update`](docs.md), which is now **deprecated** in favour of
> `materialize --collections`.

---
