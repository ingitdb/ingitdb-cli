# ⚙️ Subcollection Definition File

A **subcollection** is a collection nested within another collection's records. Subcollections use the exact same definition format as standard root-level collections (mapping to the [`CollectionDef`](../../pkg/ingitdb/collection_def.go) type), with their placement defining their relationship to parent data.

## 📂 File location

The location of a subcollection's `definition.yaml` depends on which
[layout style](collection.md#-collection-layout-styles) the parent collection uses.

**Dedicated-directory layout (`.collection/`)**

Subcollections live under `.collection/subcollections/` inside the parent collection directory.
Each subcollection has its own named subdirectory containing a `definition.yaml`.

```
<parent-collection-dir>/
  .collection/
    definition.yaml
    subcollections/
      <sub>/
        definition.yaml
```

Nested subcollections mirror the same pattern one level deeper:

```
<parent-collection-dir>/
  .collection/
    subcollections/
      <sub>/
        definition.yaml
        subcollections/       ← sub's own subcollections
          <nested-sub>/
            definition.yaml
```

**Shared-directory layout (`.collections/`)**

Subcollections are **direct non-`$`-prefixed subdirectories** of the parent collection's entry
inside `.collections/`.

```
<base-dir>/
  .collections/
    <parent-name>/
      definition.yaml
      <sub>/                  ← subcollection (no "subcollections/" intermediate dir)
        definition.yaml
        <nested-sub>/         ← nested subcollection
          definition.yaml
```

The subcollection directory name is its ID. Any directory not prefixed with `$` is treated as
a subcollection. `data_dir` is resolved relative to the base directory (parent of
`.collections/`).

The subcollection directory name dictates the identity of the subcollection.

## 📂 Example

Using a company / organisation model is universally understood, cleanly hierarchical, and supports multiple subcollections at the same level without overlap.

### Data Structure

This model works well because of its clear containment rules. For example, `departments` and `offices` are multiple independent subcollections at the same level (under a company). `teams` and `projects` are nested subcollections inside `departments`, and `members` is further nested inside `teams`.

A minimal data sample for the structure above:

```text
companies
  acme-inc
    departments
      engineering
        teams
          backend
            members
              alice
              bob
        projects
          api-v2
      marketing
    offices
      dublin
      london
```

### Schema Structure

**Dedicated-directory layout** — schema definition files under the root `.collection/` folder:

```text
companies/
  .collection/
    definition.yaml                                   <-- "companies" schema
    subcollections/
      departments/
        definition.yaml                               <-- "departments" schema
        subcollections/
          projects/
            definition.yaml                           <-- "projects" schema (subset of departments)
          teams/
            definition.yaml                           <-- "teams" schema
            subcollections/
              members/
                definition.yaml                       <-- "members" schema
      offices/
        definition.yaml                               <-- "offices" schema
```

**Shared-directory layout** — the same hierarchy expressed under `.collections/`:

```text
companies/
  .collections/
    companies/
      definition.yaml                                 <-- "companies" schema
      departments/
        definition.yaml                               <-- "departments" schema
        projects/
          definition.yaml                             <-- "projects" schema
        teams/
          definition.yaml                             <-- "teams" schema
          members/
            definition.yaml                           <-- "members" schema
      offices/
        definition.yaml                               <-- "offices" schema
```

If you manage a `companies` collection and you want to track `departments` (a subcollection)
for each company, the `departments` definition would look exactly like a standard root-level
collection. The comment below shows the dedicated-layout path; adjust to
`.collections/companies/departments/definition.yaml` for the shared layout.

```yaml
# companies/.collection/subcollections/departments/definition.yaml
# (shared layout: companies/.collections/companies/departments/definition.yaml)
titles:
  en: Departments
record_file:
  name: "{key}/{key}.json"
  type: "map[string]any"
  format: json
columns:
  title:
    type: string
    required: true
  manager_id:
    type: string
```

## 📂 Schema Format

The schema format for a subcollection is identical to a standard collection. Please refer to the [Collection Definition File](collection.md) for a comprehensive list of all supported fields, column types, and storage definitions.
