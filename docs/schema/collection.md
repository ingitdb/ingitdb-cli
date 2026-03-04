# ⚙️ Collection Definition File (`definition.yaml`)

Each collection is described by a `definition.yaml` file that controls how records are stored
and what columns (fields) they contain. The file structure maps to the
[`CollectionDef`](../../pkg/ingitdb/collection_def.go) type.

See the [Collection README Builder](../components/readme-builders/collection.md) for details on
how a collection's `README.md` is automatically populated and updated.

---

## 📂 Collection layout styles

inGitDB supports **two directory layouts** for `definition.yaml`. Choose the one that fits
each directory's content.

### 1 · Dedicated-directory layout (`.collection/`)

One collection per directory. The collection's ID is the name of the directory that contains
`.collection/`. This is the default choice for most standalone collections.

```
<collection-dir>/
  .collection/
    definition.yaml         ← collection schema
    views/
      <view-name>.yaml      ← named views
    subcollections/
      <sub>/
        definition.yaml     ← subcollection schema
  <data files …>
```

**Use when** a directory is devoted to a single collection.

```
todo/tags/
  .collection/
    definition.yaml   ← "tags" collection
  tags.json
```

### 2 · Shared-directory layout (`.collections/`)

Multiple collections share one directory. Each collection lives in its own named subdirectory
under `.collections/`. The collection's ID is the subdirectory name.

```
<base-dir>/
  .collections/
    <collection-name>/
      definition.yaml         ← collection schema
      $views/
        <view-name>.yaml      ← named views (reserved folder)
      <subcollection-name>/   ← subcollection schema (any non-$-prefixed subdir)
        definition.yaml
  <data files …>              ← resolved via data_dir in each definition.yaml
```

**Use when** two or more related data files (e.g. `recipes.yaml` and `ingredients.csv`) must
live in the same directory.

```
cooking/
  .collections/
    recipes/
      definition.yaml         ← "recipes" collection
      $views/
        by_cuisine.yaml
      ingredients_of_recipe/
        definition.yaml       ← subcollection
    ingredients/
      definition.yaml         ← "ingredients" collection
  recipes/
    chicken-soup.yaml
    pasta.yaml
  ingredients.csv
```

| Term | Meaning |
| ---- | ------- |
| **base dir** | Parent of `.collections/`; anchor for all `data_dir` path resolution |
| **collection name** | Subdirectory name inside `.collections/`; becomes the collection's ID |
| **`$views/`** | Reserved folder for named view definitions; `$`-prefix marks inGitDB-managed dirs |

### Choosing a layout

| Situation | Recommended layout |
| --------- | ------------------ |
| One collection per directory (most cases) | Dedicated (`.collection/`) |
| Two or more collections share the same data directory | Shared (`.collections/`) |

### Referencing layouts from `root-collections.yaml`

The two layouts differ only in where `definition.yaml` lives. From the perspective of
[`root-collections.yaml`](root-config.md#-root-collectionsyaml), each style is referenced
symmetrically — the path simply points to the directory that **contains the definition**:

| Layout | Path in `root-collections.yaml` | Resolves to |
| ------ | ------------------------------- | ----------- |
| Dedicated (`.collection/`) | The collection directory itself | `definition.yaml` is at `<path>/.collection/definition.yaml` |
| Shared (`.collections/`) | The named subdir inside `.collections/` | `definition.yaml` is at `<path>/definition.yaml` |

```yaml
# .ingitdb/root-collections.yaml

# Dedicated: .collection/definition.yaml lives inside data/countries/
countries: data/countries

# Shared: point directly at the named subdir inside .collections/
recipes: cooking/.collections/recipes
ingredients: cooking/.collections/ingredients
```

Both entries are valid plain collection entries; `root-collections.yaml` treats them
identically once the path resolves to a directory that contains the expected
`definition.yaml`.

### Layout conflict

If both `.collection/` and `.collections/` exist in the same directory the validator returns
an **error**. Only one layout may be active per directory.

---

## 📂 Top-level fields

| Field           | Type                                                         | Description                                     |
| --------------- | ------------------------------------------------------------ | ----------------------------------------------- |
| `titles`        | `map[locale]string`                                          | Human-readable collection name, keyed by locale |
| `record_file`   | [`RecordFileDef`](../../pkg/ingitdb/record_file_def.go)      | How records are stored on disk (required)       |
| `data_dir`      | `string`                                                     | Custom data directory (optional; see [layout notes](#-shared-directory-layout-details)) |
| `readme`        | [`CollectionReadmeDef`](../../pkg/ingitdb/collection_def.go) | README.md generation configuration (optional)   |
| `columns`       | `map[string]`[`ColumnDef`](../../pkg/ingitdb/column_def.go)  | Column (field) definitions                      |
| `columns_order` | `[]string`                                                   | Display order for columns                       |
| `default_view`  | [`*ViewDef` (inline)](../features/default-collection-view.md) | Inline default web export view — exported to `$ingitdb/` for web-app consumption (optional) |

---

## 📂 `record_file`

Controls how records are physically stored.

| Field    | Type                                                 | Description                                                               |
| -------- | ---------------------------------------------------- | ------------------------------------------------------------------------- |
| `name`   | `string`                                             | File name or pattern. May contain `{key}` and `{fieldName}` placeholders. |
| `format` | `string`                                             | File format: `json`, `yaml`, or `yml`                                     |
| `type`   | [`RecordType`](../../pkg/ingitdb/record_file_def.go) | Layout of records within the file (see below)                             |

### 🔹 `record_file.type` values

#### 🔸 `map[string]any` — one record per file

Each record is a separate file. The `name` pattern must contain `{key}`.

```yaml
record_file:
  name: "{key}.json"
  type: "map[string]any"
  format: json
```

> **`$records/` subdirectory** — When `{key}` is present in `name`, inGitDB automatically
> stores all record files under a `$records/` subdirectory inside the collection directory,
> rather than directly in the collection directory. This keeps the collection's `README.md`
> visible at the top of the directory listing on GitHub.com — if records were stored at the
> collection root, a large number of them would push `README.md` below the initial render
> viewport. The `$` prefix signals to viewers that this is an inGitDB-managed directory,
> consistent with the `/$ingitdb` convention.

File `$records/ireland.json`:

```json
{
  "title": "Ireland",
  "population": 5000000
}
```

#### 🔸 `map[string]any` — list of records in one file

All records are stored as an array (or YAML list) in a single file.

```yaml
record_file:
  name: "statuses.yaml"
  type: "[]map[string]any"
  format: yaml
```

File `statuses.yaml`:

```yaml
- id: new
  title: New
- id: in_progress
  title: In Progress
```

#### 🔸 `map[string]map[string]any` — dictionary of records

All records are stored in one file as a dictionary where the top-level key is
the record ID.

```yaml
record_file:
  name: "statuses.yaml"
  type: "map[string]map[string]any"
  format: yaml
```

File `statuses.yaml`:

```yaml
new:
  titles: { en: New, ru: Новая }
in_progress:
  titles: { en: In Progress, ru: В работе }
```

#### 🔸 `map[$record_id]map[$field_name]any` — all records in one file keyed by ID

All records are stored in a single file. Top-level keys are record IDs, second-level keys are
field names. `$record_id` and `$field_name` are aliases for `string` used for readability —
they make it clear which dimension of the map represents record identifiers versus field names.

This is the preferred type when you want a single, human-readable file for a
small reference collection (e.g. tags, categories).

```yaml
record_file:
  name: "tags.json"
  type: "map[$record_id]map[$field_name]any"
  format: json
```

File `tags.json`:

```json
{
  "work": {
    "titles": { "en": "Work", "ru": "Работа", "es": "Trabajo" }
  },
  "home": {
    "titles": { "en": "Home", "ru": "Дом", "es": "Casa" }
  }
}
```

---

## 📂 `columns`

Each entry under `columns` is a **ColumnDef** keyed by the field name.

| Field         | Type                                             | Description                       |
| ------------- | ------------------------------------------------ | --------------------------------- |
| `type`        | [`ColumnType`](../../pkg/ingitdb/column_type.go) | Field type (required)             |
| `required`    | `bool`                                           | Whether the field must be present |
| `title`       | `string`                                         | Display label                     |
| `titles`      | `map[locale]string`                              | Localized display labels          |
| `length`      | `int`                                            | Exact length constraint           |
| `min_length`  | `int`                                            | Minimum length                    |
| `max_length`  | `int`                                            | Maximum length                    |
| `foreign_key` | `string`                                         | Reference to another collection   |
| `locale`      | `string`                                         | Locale shortcut (see below)       |

### 🔹 Column types

| Type                | Description                                     |
| ------------------- | ----------------------------------------------- |
| `string`            | Plain text                                      |
| `int`               | Integer number                                  |
| `float`             | Floating-point number                           |
| `bool`              | Boolean flag                                    |
| `date`              | Date (ISO 8601)                                 |
| `time`              | Time                                            |
| `datetime`          | Date and time                                   |
| `any`               | Untyped value                                   |
| `map[locale]string` | Localized string map (see Locale columns below) |

---

## 📂 Locale columns and the `locale` field

A column with `locale` set is a **locale shortcut** — a convenient single-value
alias for one locale within a paired `map[locale]string` column.

The pairing rule is: the pair column name is `<shortcut_column_name> + "s"`.
For example, column `title` (with `locale: en`) is paired with column `titles`
(of type `map[locale]string`).

```yaml
columns:
  title:
    type: string
    locale: en # shortcut for titles["en"]
  titles:
    type: "map[locale]string"
    required: true
```

### 🔹 Read behaviour

When a record is read from the file, the locale value is **extracted** from the
pair column and exposed via the shortcut column. The locale key is **removed**
from the pair column to avoid duplication. Example:

File data:

```json
{ "titles": { "en": "Work", "ru": "Работа" } }
```

Data presented to the application:

```json
{ "title": "Work", "titles": { "ru": "Работа" } }
```

The `en` entry is not stored in `titles` when `title` already carries it.

### 🔹 Write behaviour

When a record is written, the shortcut column value is **merged** into the pair
column under the configured locale key, and the shortcut column itself is
**not stored** in the file. Example:

Data from the application:

```json
{ "title": "Work", "titles": { "ru": "Работа" } }
```

Data written to the file:

```json
{ "titles": { "en": "Work", "ru": "Работа" } }
```

This keeps the on-disk representation clean and avoids redundant storage of the
same value in two places.

---

## 📂 `default_view`

The `default_view` field is an **inline [`ViewDef`](view.md)** that configures how the collection is exported as a flat file into the repository's `$ingitdb/` directory. It is designed for web applications that need to load the full (or top-N) dataset in a single request rather than fetching one record at a time.

The inline definition is the key distinction from named views stored in `.collection/views/`: it lives directly in `definition.yaml` so that a web app only needs to know the collection's path to find its data — no additional discovery step required.

At load time the CLI injects the `default_view` into the collection's views map under the reserved ID `default_view` and processes it alongside all other views during `ingitdb materialize`.

```yaml
default_view:
  top: 0                       # 0 = all records (default)
  order_by: id asc             # sort order; default: id ascending
  format: tsv                  # tsv (default), csv, json, jsonl, yaml
  max_batch_size: 0            # 0 = single file; N splits into N-record files
  file_name: "${collection_id}" # base name without extension; default: collection ID
  columns:                     # optional: omit to export all columns
    - id
    - title
```

Output is written to `{repo_root}/$ingitdb/{collection_path}/{file_name}.{ext}`.
See [Default Collection View](../features/default-collection-view.md) for the full specification including file naming, pagination, and format details.

---

## 📂 Shared-directory layout details

This section applies only when using the `.collections/` layout.

### `data_dir` resolution

`data_dir` is always resolved **relative to the base directory** (the parent of `.collections/`),
not relative to the collection's own schema subdirectory.

| `data_dir` value | Effective data path (base = `/cooking/`) |
| ---------------- | ---------------------------------------- |
| `recipes`        | `/cooking/recipes/`                      |
| `.`              | `/cooking/`                              |
| *(omitted)*      | `/cooking/`                              |

```yaml
# /cooking/.collections/recipes/definition.yaml
data_dir: recipes
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
```

Record path: `/cooking/recipes/chicken-soup.yaml`

```yaml
# /cooking/.collections/ingredients/definition.yaml
# data_dir omitted — data lives directly in /cooking/
record_file:
  name: "ingredients.csv"
  type: "[]map[string]any"
  format: csv
```

Record path: `/cooking/ingredients.csv`

> **Note:** In the dedicated layout (`.collection/`), `data_dir` is resolved relative to the
> collection directory itself (the parent of `.collection/`). The resolution anchor differs
> between the two layouts.

### Named views (`$views/`)

Named views are stored in the reserved `$views/` subdirectory inside the collection's entry
under `.collections/`. The file name without `.yaml` becomes the view's identifier.

```
/cooking/.collections/recipes/$views/by_cuisine.yaml
```

The `$` prefix signals that this is an inGitDB-managed directory and not a subcollection.
See [View Definition File](view.md) for the full view schema.

### Subcollections

A subcollection's schema is a **direct non-`$`-prefixed subdirectory** of the parent
collection's `.collections/` entry.

```
/cooking/.collections/recipes/ingredients_of_recipe/definition.yaml
```

Subcollections can nest arbitrarily:

```
/cooking/.collections/recipes/ingredients_of_recipe/nutrient/definition.yaml
```

A subcollection's own `data_dir` follows the same resolution rule — relative to the **base
directory** (parent of `.collections/`), not to the subcollection's schema directory.

See [Subcollection Definition File](subcollection.md) for the full hierarchical data model.

### Discovery rules

The scanner walks the repository looking for `.collections/` directories. For each one found:

1. The **base directory** is set to the parent of `.collections/`.
2. Every direct subdirectory of `.collections/` is examined:
   - Subdirectories whose name starts with `$` are **skipped** (managed directories).
   - All others must contain a `definition.yaml` and are treated as collection definitions.
3. Within each collection's subdirectory, subcollections are discovered recursively using the
   same rule (skip `$`-prefixed, recurse into the rest).

### Validator rules

| Rule | Severity | Description |
| ---- | -------- | ----------- |
| Duplicate `record_file.name` within the same base directory | **error** | Two collections resolving to the same file path would corrupt each other's data. |
| Duplicate collection name within the same `.collections/` directory | **error** | Each named subdirectory must be unique. |
| Both `.collection/` and `.collections/` present in the same directory | **error** | Only one layout may be active per directory. |
| `$views/` entry that is not a directory | **warning** | A plain file named `$views` is unexpected and ignored. |
| Subdirectory with no `definition.yaml` | **warning** | Reported but not fatal; may be a work-in-progress collection. |

---

## 📂 Full examples

### Dedicated-directory layout

`todo/tags/.collection/definition.yaml`:

```yaml
titles:
  en: Tags
  ru: Теги
record_file:
  name: "tags.json"
  type: "map[$record_id]map[$field_name]any"
  format: json
columns:
  title:
    type: string
    locale: en
  titles:
    type: "map[locale]string"
    required: true
```

`todo/tags/tags.json`:

```json
{
  "work": { "titles": { "en": "Work", "ru": "Работа" } },
  "home": { "titles": { "en": "Home", "ru": "Дом" } },
  "personal": { "titles": { "en": "Personal", "ru": "Личное" } }
}
```

### Shared-directory layout

`/cooking/.collections/recipes/definition.yaml`:

```yaml
titles:
  en: Recipes
data_dir: recipes
record_file:
  name: "{key}.yaml"
  type: "map[string]any"
  format: yaml
columns:
  title:
    type: string
    required: true
  cuisine:
    type: string
  prep_time_minutes:
    type: int
columns_order:
  - title
  - cuisine
  - prep_time_minutes
default_view:
  format: tsv
  columns: [id, title, cuisine]
```

`/cooking/.collections/ingredients/definition.yaml`:

```yaml
titles:
  en: Ingredients
# data_dir omitted — data lives directly in /cooking/
record_file:
  name: "ingredients.csv"
  type: "[]map[string]any"
  format: csv
columns:
  name:
    type: string
    required: true
  unit:
    type: string
```
