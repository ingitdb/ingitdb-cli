## Contents

| Child | Description |
|---|---|
| [select](select/README.md) | The select verb queries records from a single collection. Two modes: single-record (--id) and set (--from + optional --where/--order-by/--fields/--limit). Output format defaults to yaml in single-record mode and csv in set mode; --format overrides. Replaces read-record and query. |

# CLI Features

This directory contains feature specifications for individual `ingitdb` CLI subcommands.
Each subdirectory corresponds to one command (e.g. `create-record` → `ingitdb create record`).
| [insert](insert/README.md) | The insert verb creates a new record in a collection. Uses --into for the target collection and --key for the record key (or  in the data as fallback). Accepts --data, stdin, --edit, or --empty as the data source. Rejects when the key already exists. Replaces create record. |
| [update](update/README.md) | The update verb applies patch-style changes to records: --set adds/changes fields, --unset removes fields. Two modes inherited from shared-cli-flags: single-record (--id) and set (--from + --where|--all). Shallow patch semantics. Silent on success. --require-match opts into non-zero exit when set mode finds zero records. Renames update record. |
| [delete](delete/README.md) | The delete verb removes records from a collection. Two modes inherited from shared-cli-flags: single-record (--id) and set (--from + --where|--all). --min-affected=N opts into non-zero exit when fewer than N records are deleted. Silent on success. Replaces delete record and delete records. |
| [drop](drop/README.md) | The drop verb removes schema objects from the database. Two kinds today: 'drop collection <name>' and 'drop view <name>'. Removes both the schema entry in .ingitdb.yaml and any associated data directory in a single git commit. --if-exists makes the operation idempotent; --cascade also drops dependents. Replaces delete collection and delete view. |

## Index

| Feature | Status | Command |
|---------|--------|---------|
| [version](version/README.md) | Implementing | `ingitdb version` |
| [validate](validate/README.md) | Implementing | `ingitdb validate` |
| [read-record](read-record/README.md) | Implementing | `ingitdb read record` |
| [create-record](create-record/README.md) | Implementing | `ingitdb create record` |
| [update-record](update-record/README.md) | Implementing | `ingitdb update record` |
| [delete-record](delete-record/README.md) | Implementing | `ingitdb delete record` |
| [list-collections](list-collections/README.md) | Implementing | `ingitdb list collections` |
| [rebase](rebase/README.md) | Implementing | `ingitdb rebase` |
| [find](find/README.md) | Draft | `ingitdb find` |
| [truncate](truncate/README.md) | Draft | `ingitdb truncate` |
| [delete-collection](delete-collection/README.md) | Draft | `ingitdb delete collection` |
| [delete-records](delete-records/README.md) | Draft | `ingitdb delete records` |
| [query](query/README.md) | Draft | `ingitdb query` |
| [materialize](materialize/README.md) | Draft | `ingitdb materialize` |
| [diff](diff/README.md) | Draft | `ingitdb diff` |
| [pull](pull/README.md) | Draft | `ingitdb pull` |
| [watch](watch/README.md) | Draft | `ingitdb watch` |
| [serve](serve/README.md) | Draft | `ingitdb serve` |
| [resolve](resolve/README.md) | Draft | `ingitdb resolve` |
| [setup](setup/README.md) | Draft | `ingitdb setup` |
| [migrate](migrate/README.md) | Draft | `ingitdb migrate` |

## Outstanding Questions

None at this time.

---
*This document follows the https://specscore.md/feature-specification*
