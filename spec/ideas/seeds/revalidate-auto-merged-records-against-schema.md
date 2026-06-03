---
type: sidekick-seed
slug: revalidate-auto-merged-records-against-schema
captured_at: 2026-06-03T14:25:20Z
captured_by: claude
captured_during: null
trigger: explicit
status: done
synchestra_task: null
---
# Re-validate auto-merged records against the collection schema and escalate invalid merges instead of staging them

Gap found while reconciling cli/resolve/auto-resolve/record-merge (5/7 ACs). The Concept and REQ:three-way-record-merge require the merge to "re-parse and re-validate against the collection schema," and `AC:invalid-merge-escalates` requires escalation when a merge would produce a schema-invalid file. But `mergeAndSerialize` (`cmd/ingitdb/commands/record_merge_resolver.go`) only re-parses during serialization — it never validates against the schema (grep for validate/validator across the record-merge path returns nothing). A serializable-but-schema-invalid merge (required-field/type/enum violation) is staged rather than escalated. Wire schema validation into the merge result and escalate on failure; add a test for a schema-invalid merge. The README's "Open Questions: None outstanding" is stale relative to this. (Related: the queued `wire-write-time-validations...` seed — both want shared write-time validation hooks.)
