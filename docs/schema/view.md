# ⚙️ View Definition File (`.collection/views/<name>.yaml`)

A materialized view definition specifies how records from a collection should be queried, mapped, sorted, and output into distinct formats or files. It maps to the [`ViewDef`](../../pkg/ingitdb/view_def.go) type.

## 📂 File location

Each view is defined in a YAML file inside the `.collection/views/` subdirectory of the collection's directory. For example, `.collection/views/readme.yaml` defines a view named `readme`. The file name (without the `.yaml` extension) becomes the view's identifier.

Views support string substitution in names using `{field}` placeholders (e.g., `status_{status}.yaml`).

## 📂 Top-level fields

| Field              | Type                | Description                                                                                       |
| ------------------ | ------------------- | ------------------------------------------------------------------------------------------------- |
| `titles`           | `map[locale]string` | i18n display names for the view. `{field}` placeholder substitution is supported.                 |
| `order_by`         | `string`            | Field name to sort by, optionally followed by `asc` or `desc`. Defaults to `$last_modified desc`. |
| `formats`          | `[]string`          | An array of output formats to generate (e.g., `md`, `csv`, `yaml`).                               |
| `columns`          | `[]string`          | An ordered list of column IDs from the collection to include in the output.                       |
| `top`              | `int`               | Limits total output to top `N` records after sorting. Defaults to `0` representing all.           |
| `template`         | `string`            | Path to a custom view template, relative to the collection directory.                             |
| `file_name`        | `string`            | The desired file name for the view output, relative to the collection directory.                  |
| `records_var_name` | `string`            | Template variable name acting as the handler for the target slice sequence.                       |
| `format`           | `string`            | Export file format. Supported values: `ingr` (default), `tsv`, `csv`, `json`, `jsonl`, `yaml`. Default: `ingr`. |
| `max_batch_size`   | `int`               | Max records per output file. `0` = all records in single file (default).                          |

## 📂 Field references in view partitions

When defining names using `{field}` blocks (for example, `.collection/views/status_{status}.yaml`), the output engine will output a separate, distinct view file layout for every identified value matching that partition field, simplifying data segmentation in your system.

## 📂 Further Reading

- [Views Builder Component Document](../components/views-builder.md)

**Note:** The `IsDefault` flag is runtime-only (not serialized). It is set when a collection has an inline `default_view` block.
