---
type: sidekick-seed
captured_by: claude
status: done
---
# Implement --in and --filter-name scoped filtering for `list collections` (or remove the dead flags from the command and spec)

Gap found while reconciling cli/list-collections (1/2 ACs). The `--in` and `--filter-name` flags are registered in `collections()` (`cmd/ingitdb/commands/list.go`) but their values are never read via `GetString` and no glob/regex filtering is applied in `listCollectionsLocal` or `listCollectionsRemoteWithSpec` — they are dead no-op flags, leaving `AC:scoped-listing` (REQ:in-flag, REQ:filter-name-flag) unsatisfied and untested. Sibling commands `describe.go` and `drop.go` already read `--in` via `GetString("in")`, so the pattern exists. Either wire the flags up (and add a filtered-output test) or remove them from both the command and the spec.
