# ⚙️ Subcollection Definition File (`.ingitdb-collection/subcollections/<name>.yaml`)

A **subcollection** is a collection nested within another collection's records. Subcollections use the exact same definition format as standard root-level collections, with their placement defining their relationship to parent data.

## 📂 File location

The definitions for subcollections are located in the `.ingitdb-collection/subcollections` subdirectory of the parent collection's directory. Each file within this subdirectory (e.g., `.ingitdb-collection/subcollections/dates.yaml`) defines a specific subcollection, where the file name dictates the subcollection's identifier.

## 📂 Schema Format

The schema format for a subcollection is identical to a standard collection. Please refer to the [Collection Definition File](collection.md) for a comprehensive list of all supported fields, column types, and storage definitions.

## 📂 Example

Using a company / organisation model is universally understood, cleanly hierarchical, and supports multiple subcollections at the same level without overlap.

```text
companies
  └── {companyId}
        ├── departments
        │     └── {departmentId}
        │           ├── teams
        │           │     └── {teamId}
        │           │           └── members
        │           │                 └── {memberId}
        │           └── projects
        │                 └── {projectId}
        └── offices
              └── {officeId}
```

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

If you manage a `companies` collection and you want to track `departments` (a subcollection) for each company:

```yaml
# companies/.ingitdb-collection/subcollections/departments.yaml
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
