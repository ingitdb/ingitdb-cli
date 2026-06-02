---
type: sidekick-seed
slug: wire-write-time-validations-foreign-keys-and-reject-stored
captured_at: 2026-06-02T08:46:12Z
captured_by: specstudio:implement
captured_during: null
trigger: explicit
status: queued
synchestra_task: null
---
# Wire write-time validations (foreign keys and reject-stored-computed-values) into the CLI dalgo2fsingitdb read-write path so inserts and updates enforce them, not just the dalgo2ingitdb DALgo layer

The CLI builds its DB via dalgo2fsingitdb.NewLocalDBWithDef (cmd/ingitdb/main.go), whose readwriteTx.Set/Insert/Update/Delete perform no validation. So write-time foreign-key enforcement and computed-value rejection currently run only at the dalgo2ingitdb layer (and the validate command via datavalidator), not on CLI inserts/updates. This is a pre-existing limitation that also affects the shipped dalgo2ingitdb-referential-integrity feature. Consider extracting the write validations into shared functions callable from both dalgo2ingitdb and dalgo2fsingitdb (and dalgo2ghingitdb), or have the fs/gh write txns delegate.
