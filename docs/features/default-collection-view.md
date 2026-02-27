# 📦 Default Collection View

The **default view** feature generates flat export files for every collection that declares a `default_view` block in its `definition.yaml`. These files are designed for web applications and other tools that need the full (or top-N) collection dataset without issuing one HTTP request per record.

## 📂 Output location

The output destination depends on the type of view being materialised:

| View type | Output path |
| --------- | ----------- |
| **Default view** (`default_view` block) | `{repo_root}/$ingitdb/{collection_path}/{collection_id}.ingr` |
| **Template-rendered view** (e.g. `README.md`) | Written directly into the collection directory alongside source records |
| **Other named export view** | `{repo_root}/$ingitdb/{collection_path}/{view_file}` |

Default and named export views are written to a dedicated `$ingitdb/` directory at the **repository
root**, mirroring the collection directory tree:

```
{repo_root}/
  $ingitdb/
    {collection_path}/
      {collection_id}.ingr           ← single file (all records fit in one batch)
      {collection_id}-000001.ingr    ← paginated (batch number injected when > 1 batch needed)
      {collection_id}-000002.ingr
    {collection_path}/{sub_collection}/
      {sub_collection_id}.ingr
```

Example for a repo with a `todos` top-level collection and a `tags` sub-collection:

```
$ingitdb/
  todos/
    todos.ingr
  todos/tags/
    tags.ingr
```

The `$ingitdb/` directory **is committed** to the repository so that web apps can load data directly
from raw file URLs (e.g., GitHub raw content, Gitea, self-hosted Git servers).

### Why a separate root directory?

Keeping generated artefacts in `$ingitdb/` (rather than a `$views/` subfolder inside each collection) provides two key benefits:

1. **Clean git history** — source data commits and automated materialisation commits are clearly separated. Readers can instantly see who changed source records versus what was auto-generated.
2. **Single deploy target** — a CI/CD pipeline can treat `$ingitdb/` as a static-site build output and serve or deploy it independently.

---

## 📂 How the default view is processed

The `default_view` is not a special case in the materialiser. After the collection definition is loaded, the inline `default_view` block is **injected into the collection's views map** under the reserved ID `default_view` with its `IsDefault` flag set to `true`. From that point it is validated and executed exactly like any other view defined in `.collection/views/`.

Output paths follow the routing rules described in [Output location](#-output-location): default and
named export views land in `{repo_root}/$ingitdb/{collection_path}/`, while template-rendered views
(e.g. `README.md`) are written directly into the collection directory.

At most one view per collection may have `IsDefault = true`. The validator enforces this constraint.

---

## 📂 Configuring the default view

The `default_view` field in `.collection/definition.yaml` accepts an inline [ViewDef](../schema/view.md) that controls what gets exported and in what format.

```yaml
default_view:
  top: 0                    # 0 = all records (default)
  order_by: id asc          # sort order (default: record id ascending)
  format: ingr              # ingr (default), tsv, csv, json, jsonl, yaml
  max_batch_size: 0         # 0 = single file; N > 0 = max N records per file
  file_name: "${collection_id}"   # base file name without extension (default: collection ID)
  columns:                  # optional: omit to include all columns
    - id
    - title
    - status
```

All fields are optional. A minimal configuration that exports all records in the default INGR format:

```yaml
default_view: {}
```

---

## 📂 File naming

The full output file name is composed as:

```
{file_name}.{format_extension}
```

When the total record count exceeds `max_batch_size` (and `max_batch_size > 0`), a zero-padded six-digit batch number is injected before the extension:

```
{file_name}-{NNNNNN}.{format_extension}
```

The batch number is **only injected when more than one batch is required**. If all records fit in a single batch, the file name has no batch number suffix:

| Scenario | Example output |
| --- | --- |
| `max_batch_size: 0` or all records ≤ `max_batch_size` | `todos.ingr` |
| Records span multiple batches | `todos-000001.ingr`, `todos-000002.ingr`, … |

---

## 📂 File formats

| Format  | Extension | Notes |
| ------- | --------- | ----- |
| `ingr`  | `.ingr`   | **Default.** Compact fixed-line record format — one field per line, no header, no delimiters. Optimised for Git diffs. See [INGR spec](../file-formats/ingr.md). |
| `tsv`   | `.tsv`    | Tab-separated values. Row 1 = column headers. One record per line. |
| `csv`   | `.csv`    | Comma-separated values (RFC 4180). Row 1 = column headers. Values containing commas or double-quotes are quoted. |
| `json`  | `.json`   | JSON array of objects `[{…}, …]`. |
| `jsonl` | `.jsonl`  | Newline-delimited JSON — one JSON object per line. |
| `yaml`  | `.yaml`   | YAML sequence of mappings. |

**INGR is the default format** because it produces the smallest, most readable Git diffs: each field
occupies exactly one line, so a change to a single field appears as a single-line diff regardless of
record size. TSV remains available when a header row or spreadsheet compatibility is needed.

### TSV escaping

| Character | Escaped as |
| --------- | ---------- |
| Tab (`\t`)       | `\t` (literal backslash + `t`) |
| Newline (`\n`)   | `\n` (literal backslash + `n`) |
| Backslash (`\`)  | `\\` |

---

## 📂 Column ordering

Column order is resolved in priority order:

1. **`columns`** in the view definition — used as-is, in the order specified.
2. **`columns_order`** in `definition.yaml` — used when `columns` is absent.
3. **All columns from `definition.yaml`** sorted alphabetically — used when neither `columns` nor `columns_order` is defined.

The record `id` is always the first column.

---

## 📂 README links

When a collection has `default_view` configured, the auto-generated `README.md` for that collection includes a link to the corresponding `$ingitdb/` export path so that repository browsers can navigate to the data.

---

## 📂 Examples

- **Countries** — a minimal default view using all defaults. See [definition](../../demo-dbs/test-db/countries/.collection/definition.yaml) and expected output: `$ingitdb/countries/countries.ingr`

---

## 📂 CLI output

Running `ingitdb materialize` prints one progress line **for every view processed** (whether created,
updated, or unchanged) followed by a single summary line.

### Per-view progress line

```
Materializing view {collection}/{view}... N records saved to {path-relative-to-repo-root}
```

Examples:

```
Materializing view todos/default_view... 42 records saved to $ingitdb/todos/todos.ingr
Materializing view todos/README.md... 42 records saved to todos/.collection/README.md
Materializing view countries/default_view... 5 records saved to $ingitdb/countries/countries.ingr
```

### Summary line

After all views are processed, a single summary is printed:

```
materialized views: N created, N updated, N deleted, N unchanged
```

Example:

```shell
$ ingitdb materialize
Materializing view todos/default_view... 42 records saved to $ingitdb/todos/todos.ingr
Materializing view countries/default_view... 5 records saved to $ingitdb/countries/countries.ingr
materialized views: 0 created, 1 updated, 0 deleted, 1 unchanged
```

### Validation output

Schema validation runs automatically before materialisation, but **validation messages are
suppressed** during `materialize` — you will not see "Definition of collection '…' is valid"
lines in normal operation. Validation errors that block materialisation are still reported.

---

## 📂 See also

- [`default_view` field reference → collection.md](../schema/collection.md#-default_view)
- [`format` and `max_batch_size` → view.md](../schema/view.md)
- [Collection definition reference](../schema/collection.md)
