---
type: sidekick-seed
slug: reconcile-dbschema-concurrency-contract
captured_at: 2026-06-03T16:54:53Z
captured_by: claude
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Reconcile the dbschema-ddl concurrency contract: spec says single-writer (false), code returns true via flock

Gap found verifying dalgo2ingitdb-dbschema-ddl-coverage (25/27 ACs — otherwise complete and well-tested). REQ:concurrency-aware-false / AC:supports-concurrent-connections-false mandate `SupportsConcurrentConnections() == false` (single-writer git working tree), but `database.go` embeds `dal.ConcurrencyAvailable` and returns `true`, and the tests (`TestDatabase_SupportsConcurrentConnections`, `TestIntegration_ConcurrentReads`) assert `true`, with code comments justifying it via gofrs/flock file locking. This is a deliberate design divergence, not a missing implementation.

DECISION NEEDED: is flock-based concurrent access the intended contract? If yes, update the spec (Summary bullet, Out-of-Scope "Concurrent writes" line, REQ:concurrency-aware-false, AC) to reflect `true`. If single-writer is still required, change the code to return `false`. Once reconciled (plus the minor AC:list-constraints-returns-pk `Fields` wording, which contradicts its own REQ), the feature can move to Stable.
