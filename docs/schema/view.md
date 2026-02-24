# ⚙️ View Definition File (`.ingitdb-collection/view_<name>.yaml`)

A materialized view definition specifies how records from a collection should be queried, mapped, sorted, and output into distinct formats or files.

## 📂 File location

Each view is defined in a file starting with `view_` (such as `view_{status}.yaml`) inside the collection's `.ingitdb-collection` directory.

The string `<name>` following `view_` becomes the name of the output directory under the main view directory. Views support string substitution in names using `{field}` placeholders.

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

## 📂 Field references in view partitions

When defining names using `{field}` blocks (for example, `.ingitdb-collection/view_status_{status}.yaml`), the output engine will output a separate, distinct view file layout for every identified value matching that partition field, simplifying data segmentation in your system.

## 📂 Further Reading

- [Views Builder Component Document](../components/views-builder.md)
