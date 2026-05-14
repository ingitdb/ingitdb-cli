// Package sqlflags implements the shared CLI flag grammar for the
// SQL-verb commands (select, insert, update, delete, drop).
//
// Each parser is a pure function. Each flag-registration helper takes
// a *cobra.Command and adds the flag with the documented metadata.
// The package is the single source of truth for:
//
//   - --where  comparison operators (==, ===, !=, !==, >=, <=, >, <)
//   - --set    YAML-inferred assignments
//   - --unset  comma-separated field removal list
//   - --id     collection/key targeting (single-record mode)
//   - --from   collection targeting (set mode)
//   - --into   collection targeting (insert only)
//   - --all    full-collection scope guard
//   - --min-affected   positive-integer count threshold
//   - --order-by       comma-separated, '-' prefix for descending
//   - --fields         '*', '$id', or comma-separated projection
//
// Mode resolution (single-record vs set) is handled by ResolveMode.
// Applicability checks (which verb accepts which flag) are handled by
// the Reject* helpers. Authoritative spec:
// spec/features/shared-cli-flags/README.md
package sqlflags

// specscore: feature/shared-cli-flags
