# ⚙️ Collection README Builder

The `README.md` for a collection is a built-in specialized implementation of [materialized views](../../features/materialized-views.md).

When running `ingitdb docs update --collection <path>`, the CLI renders the `README.md` file for the collection. If the generated content differs from the existing file, it is automatically updated.

A collection's `README.md` file includes the following auto-generated sections:

- **Collection name**: Human-readable name of the collection.
- **Path to collection**: Shown if it is a subcollection.
- **Table of columns**: Lists all columns with their name, type, and other properties.
- **Table of subcollections**: Lists nested subcollections with their name and the number of their subcollections.
- **Table of views**: Lists available materialized views with their name and the number of columns.
- **Data preview**: A preview table rendered from collection records, based on the optional `data_preview` [view configuration](../../pkg/ingitdb/view_def.go).

## Configuration

You can customize the generated README content using the `readme` section in the [collection definition](../../schema/collection.md).

```yaml
readme:
  hide_columns: false
  hide_subcollections: false
  hide_views: false
  hide_triggers: false
  data_preview:
    top: 10
    order_by: "created_at DESC"
    # template defaults to "md-table"
```
