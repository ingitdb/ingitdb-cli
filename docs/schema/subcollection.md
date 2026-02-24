# ⚙️ Subcollection Definition File (`.collection/subcollections/<name>/definition.yaml`)

A **subcollection** is a collection nested within another collection's records. Subcollections use the exact same definition format as standard root-level collections (mapping to the [`CollectionDef`](../../pkg/ingitdb/collection_def.go) type), with their placement defining their relationship to parent data.

## 📂 File location

The definitions for subcollections are located in the `.collection/subcollections` subdirectory of the parent collection's directory.
Each subcollection is a dedicated directory (e.g., `.collection/subcollections/departments/`) that contains its own `.collection/definition.yaml` file, mirroring the structure of root collections. The subcollection directory name dictates the identity of the subcollection.

If a subcollection has its own subcollections (i.e., nested subcollections), their definitions are placed within the `.collection/subcollections` directory of that subcollection.
For example, subcollections of `departments` are defined inside `.collection/subcollections/departments/.collection/subcollections/`.

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

To support the above data structure, the schema definition files are organized into a matching metadata hierarchy under the root `.collection` folder:

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

If you manage a `companies` collection and you want to track `departments` (a subcollection) for each company, the `departments` definition would look exactly like a standard root-level collection:

```yaml
# companies/.collection/subcollections/departments/definition.yaml
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
