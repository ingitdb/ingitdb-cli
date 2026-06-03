---
type: sidekick-seed
slug: wire-collection-id-charset-validation-into-id-flag-parsing
captured_at: 2026-06-03T14:25:20Z
captured_by: claude
captured_during: null
trigger: explicit
status: done
synchestra_task: null
---
# Wire ValidateCollectionID into the --id resolution path so invalid collection segments yield a syntax diagnostic, and reconcile the underscore charset discrepancy

Gap found while reconciling id-flag-format (1/2 ACs). `ValidateCollectionID` (`pkg/ingitdb/collection_id.go`) enforces the collection-segment charset and start/end-alphanumeric rules, but it is only called for declared collection IDs (`pkg/ingitdb/config/root_config.go`) — never on the `--id` parse path. `CollectionForKey` (`pkg/dalgo2ingitdb/collection.go`) does longest-prefix resolution without charset checks, so an `--id` with an invalid collection segment is rejected only by a generic "collection not found for ID" error, not the clear syntax diagnostic `AC:id-syntax-rejects-invalid` requires. Add a test feeding a charset-invalid / no-slash collection segment through `--id`. Also reconcile a discrepancy: `ValidateCollectionID` permits `_` (and tests assert it), but REQ:collection-id-charset lists only alphanumeric and `.` — decide whether spec or code is authoritative.
