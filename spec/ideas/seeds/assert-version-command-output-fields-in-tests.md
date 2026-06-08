---
type: sidekick-seed
captured_by: claude
status: done
---
# Add a test that captures `ingitdb version` stdout and asserts all three fields (version, commit, date) are present

Gap found while reconciling cli/version (1/2 ACs). The implementation in `cmd/ingitdb/commands/version.go` prints all three fields via `fmt.Printf("ingitdb %s (%s) @ %s", ...)` to stdout, but the tests (`TestVersion_PrintsVersionInfo`, `TestVersion_EmptyValues`) only assert that RunE returns no error — they never capture stdout. So `AC:version-prints-three-fields` ("a reader can identify each of the three fields") is implemented but unverified. The test helper `runCobraCommand` does not capture output. Closing this gap (capture stdout, assert version/commit/date appear) is the only thing keeping cli/version out of Stable.
