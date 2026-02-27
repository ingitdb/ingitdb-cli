# 📦 Default Collection View

The **default view** feature generates flat export files for every collection that declares a `default_view` block in its `definition.yaml`. These files are designed for web applications and other tools that need the full (or top-N) collection dataset without issuing one HTTP request per record.

## 📂 Output location

All export files are written to a dedicated `$ingitdb/` directory at the **repository root**, mirroring the collection directory tree:

```
{repo_root}/
  $ingitdb/
    {collection_path}/
      {collection_id}.tsv            ← single file (all records fit in one batch)
      {collection_id}-000001.tsv     ← paginated (batch number injected when > 1 batch needed)
      {collection_id}-000002.tsv
    {collection_path}/{sub_collection}/
      {sub_collection_id}.tsv
```

Example for a repo with a `todos` top-level collection and a `tags` sub-collection:

```
$ingitdb/
  todos/
    todos.tsv
  todos/tags/
    tags.tsv
```

The `$ingitdb/` directory **is committed** to the repository so that web apps can load data directly from raw file URLs (e.g., GitHub raw content, Gitea, self-hosted Git servers).

### Why a separate root directory?

Keeping generated artefacts in `$ingitdb/` (rather than a `$views/` subfolder inside each collection) provides two key benefits:

1. **Clean git history** — source data commits and automated materialisation commits are clearly separated. Readers can instantly see who changed source records versus what was auto-generated.
2. **Single deploy target** — a CI/CD pipeline can treat `$ingitdb/` as a static-site build output and serve or deploy it independently.

---

## 📂 How the default view is processed

The `default_view` is not a special case in the materialiser. After the collection definition is loaded, the inline `default_view` block is **injected into the collection's views map** under the reserved ID `default_view` with its `IsDefault` flag set to `true`. From that point it is validated and executed exactly like any other view defined in `.collection/views/`.

The only special behaviour is **output routing**: because `IsDefault` is `true`, the materialiser writes the output files to `{repo_root}/$ingitdb/{collection_path}/` instead of the usual `{collection_path}/$views/`.

At most one view per collection may have `IsDefault = true`. The validator enforces this constraint.

---

## 📂 Configuring the default view

The `default_view` field in `.collection/definition.yaml` accepts an inline [ViewDef](../schema/view.md) that controls what gets exported and in what format.

```yaml
default_view:
  top: 0                    # 0 = all records (default)
  order_by: id asc          # sort order (default: record id ascending)
  format: tsv               # tsv (default), csv, json, jsonl, yaml
  max_batch_size: 0         # 0 = single file; N > 0 = max N records per file
  file_name: "${collection_id}"   # base file name without extension (default: collection ID)
  columns:                  # optional: omit to include all columns
    - id
    - title
    - status
```

All fields are optional. A minimal configuration that exports all records as TSV:

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
| `max_batch_size: 0` or all records ≤ `max_batch_size` | `todos.tsv` |
| Records span multiple batches | `todos-000001.tsv`, `todos-000002.tsv`, … |

---

## 📂 File formats

| Format  | Extension | Notes |
| ------- | --------- | ----- |
| `tsv`   | `.tsv`    | **Default.** Tab-separated values. Row 1 = column headers. Minimal overhead — one record per line gives the smallest possible git diffs. |
| `csv`   | `.csv`    | Comma-separated values (RFC 4180). Row 1 = column headers. Values containing commas or double-quotes are quoted. |
| `json`  | `.json`   | JSON array of objects `[{…}, …]`. |
| `jsonl` | `.jsonl`  | Newline-delimited JSON — one JSON object per line. |
| `yaml`  | `.yaml`   | YAML sequence of mappings. |

**TSV is the recommended default** because:
- Column names appear only once (header row), unlike JSONL where every row repeats all keys.
- Tab characters in real-world text values are extremely rare.
- Each record maps to exactly one line — a single-row data change produces a single-line git diff even across millions of rows.

### TSV escaping

| Character | Escaped as |
| --------- | ---------- |
| Tab (`\t`)       | `\t` (literal backslash + `t`) |
| Newline (`\n`)   | `\n` (literal backslash + `n`) |
| Backslash (`\`)  | `\\` |

---

## 📂 Column ordering

When no explicit `columns` list is given, the export uses `columns_order` from `definition.yaml`. The record `id` is always the first column.

---

## 📂 README links

When a collection has `default_view` configured, the auto-generated `README.md` for that collection includes a link to the corresponding `$ingitdb/` export path so that repository browsers can navigate to the data.

---

## 📂 See also

- [`default_view` field reference → collection.md](../schema/collection.md#-default_view)
- [`format` and `max_batch_size` → view.md](../schema/view.md)
- [Collection definition reference](../schema/collection.md)
