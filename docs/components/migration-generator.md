# 🔁 Migration Generator

The Migration Generator diffs two versions of an inGitDB database and produces a forward migration script and a rollback script for a target database.

## 📂 How it works

1. **Resolve versions** — use `git show <sha>:<path>` to read collection schemas and record files at both `--from` and `--to` SHAs without checking out the commits.
2. **Diff records** — compare records present in `--from` vs `--to` per collection:
   - Added records → `INSERT` (migration) / `DELETE` (rollback)
   - Removed records → `DELETE` (migration) / `INSERT` (rollback)
   - Changed records → `UPDATE` both ways (field-level diff to produce minimal statements)
3. **Diff schema** — compare `.definition.yaml` at both versions:
   - Added columns → `ALTER TABLE ADD COLUMN` (migration) / `ALTER TABLE DROP COLUMN` (rollback)
   - Removed columns → `ALTER TABLE DROP COLUMN` (migration) / `ALTER TABLE ADD COLUMN` (rollback)
   - Type changes → handled per target database dialect
4. **Write output** — render statements into the chosen format and write both files.

## ⚙️ Collection filtering

When `--collections` is provided, only the listed collection IDs are included in the diff. Schema changes in excluded collections are ignored.

## 📂 Target database connection

The `--target` connection string is used to:
- Introspect the current state of the target schema (to validate the migration is applicable)
- Infer the SQL dialect for statement generation

Connection string format follows standard DSN conventions (e.g. `postgres://user:pass@host/db`).

## 📂 Supported output formats

| Format | Status |
|---|---|
| SQL | Phase 9 |

Additional formats (e.g. Prisma migrations, Liquibase changesets) may be added in later phases.

## 📂 Package location

Implement in `pkg/ingitdb/migration/`. The package must have no dependency on the CLI layer — the generator accepts plain Go structs (two `Definition` snapshots and their record sets) and returns the script content as strings.
