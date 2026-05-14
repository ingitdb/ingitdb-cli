# cli/describe — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `ingitdb describe` (alias `desc`) — a read-only CLI command that prints the full definition of a collection (alias `table`) or a view as a `{definition, _meta}` document in YAML (default) or JSON, with a `--format=yaml|json|native|sql|SQL` grammar reusable by the future `datatug describe`.

**Architecture:** Three new files under `cmd/ingitdb/commands/` separate concerns cleanly so the plan executes in vertical slices: `describe_format.go` (pure value resolver, no I/O), `describe_output.go` (pure payload builder, takes Defs, returns `*yaml.Node`), `describe.go` (cobra plumbing — flag parsing, source resolution, kind dispatch). A `MarshalYAML` method on `CollectionDef` in `pkg/ingitdb/collection_marshal.go` pins deterministic column ordering at the type level so every consumer (not just describe) gets stable output. Local-mode only in this plan; `--remote` is accepted (for spec compliance and for the mutual-exclusion AC) but errors with "remote mode not yet implemented" — remote support is a follow-up plan.

**Tech Stack:** Go stdlib (`encoding/json`, `os`, `path/filepath`, `sort`, `strings`), `gopkg.in/yaml.v3` (specifically `yaml.Node` for ordered output), `github.com/spf13/cobra`, the project's `pkg/ingitdb` types and `pkg/ingitdb/validator.ReadDefinition`.

**Spec:** `spec/features/cli/describe/README.md`

---

## Scope check

The spec's 19 ACs all target local-mode behavior. Only AC:source-selection-mutual-exclusion exercises `--remote`, and only to verify the flag-parse-time rejection. Behavioral remote-mode is therefore out of scope for this plan; a stub returning `describe --remote not yet implemented` satisfies the rejection AC while leaving the door open. This decomposition keeps the plan to ~10 tasks instead of doubling it.

---

## Existing draft to discard

`cmd/ingitdb/commands/describe.go` and `describe_test.go` exist in the working tree from a pre-spec scaffold (see the feature spec's Implementation section). Task 0 deletes both. The `commands.Describe(...)` registration in `cmd/ingitdb/main.go` stays — Task 8 re-validates it.

---

## File Map

| Action | File | Change |
|--------|------|--------|
| Create | `pkg/ingitdb/collection_marshal.go` | `CollectionDef.MarshalYAML` honoring `columns_order`, alphabetical fallback |
| Create | `pkg/ingitdb/collection_marshal_test.go` | Unit tests for the marshaller |
| Create | `cmd/ingitdb/commands/describe_format.go` | Pure `resolveFormat(raw, engine)` returning canonical format or typed error |
| Create | `cmd/ingitdb/commands/describe_format_test.go` | Tests for every input value, both engines (ingitdb + future-sql) |
| Create | `cmd/ingitdb/commands/describe_output.go` | Pure `buildCollectionPayload` + `buildViewPayload`, returning `*yaml.Node` |
| Create | `cmd/ingitdb/commands/describe_output_test.go` | Tests covering `_meta` shape, both kinds, data_dir divergence |
| Delete | `cmd/ingitdb/commands/describe.go` (existing draft) | Replaced by Task 4 rewrite |
| Delete | `cmd/ingitdb/commands/describe_test.go` (existing draft) | Replaced |
| Create | `cmd/ingitdb/commands/describe.go` | Cobra parent + `collection` subcommand (with `table` alias) + `view` subcommand (with `--in`) + bare-name resolver |
| Create | `cmd/ingitdb/commands/describe_test.go` | Subcommand-level tests against `runCobraCommand` helper |
| Verify | `cmd/ingitdb/main.go` | `commands.Describe(...)` already registered; just confirm |

---

## Task 0: Discard the pre-spec draft

**Files:**
- Delete: `cmd/ingitdb/commands/describe.go`
- Delete: `cmd/ingitdb/commands/describe_test.go`

- [ ] **Step 1: Delete both files**

```bash
rm cmd/ingitdb/commands/describe.go cmd/ingitdb/commands/describe_test.go
```

- [ ] **Step 2: Verify build now fails on missing `commands.Describe`**

Run: `go build ./cmd/ingitdb/...`
Expected: error `undefined: commands.Describe` from `main.go`. This confirms the registration line is still there and that the next task must restore the symbol.

- [ ] **Step 3: Commit**

```bash
git add -u cmd/ingitdb/commands/
git commit -m "chore(cli): drop pre-spec describe scaffold

Removes the scaffold that predated spec/features/cli/describe/.
Rebuild driven by spec/plans/2026-05-14-cli-describe.md."
```

---

## Task 1: `CollectionDef.MarshalYAML` for deterministic column order

**Why first:** Tasks 3 and 5 depend on `yaml.Marshal(colDef)` being deterministic. Pinning this at the type level means every consumer (drop, list, validate, future commands) gets stable output for free.

**Files:**
- Create: `pkg/ingitdb/collection_marshal.go`
- Create: `pkg/ingitdb/collection_marshal_test.go`

- [ ] **Step 1: Write the failing test**

Create `pkg/ingitdb/collection_marshal_test.go`:

```go
package ingitdb

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCollectionDef_MarshalYAML_HonorsColumnsOrder(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		RecordFile: &RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: SingleRecord},
		Columns: map[string]*ColumnDef{
			"id":    {Type: ColumnTypeString},
			"email": {Type: ColumnTypeString},
			"name":  {Type: ColumnTypeString},
		},
		ColumnsOrder: []string{"email", "id", "name"},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	emailIdx := strings.Index(got, "email:")
	idIdx := strings.Index(got, "id:")
	nameIdx := strings.Index(got, "name:")
	if !(emailIdx < idIdx && idIdx < nameIdx) {
		t.Errorf("expected columns_order [email, id, name]; got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_AlphabeticalFallback(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		RecordFile: &RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: SingleRecord},
		Columns: map[string]*ColumnDef{
			"name":  {Type: ColumnTypeString},
			"email": {Type: ColumnTypeString},
			"id":    {Type: ColumnTypeString},
		},
	}
	out, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(out)
	emailIdx := strings.Index(got, "email:")
	idIdx := strings.Index(got, "id:")
	nameIdx := strings.Index(got, "name:")
	if !(emailIdx < idIdx && idIdx < nameIdx) {
		t.Errorf("expected alphabetical fallback (email, id, name); got:\n%s", got)
	}
}

func TestCollectionDef_MarshalYAML_DeterministicAcrossRuns(t *testing.T) {
	t.Parallel()
	def := &CollectionDef{
		RecordFile: &RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: SingleRecord},
		Columns: map[string]*ColumnDef{
			"a": {Type: ColumnTypeString},
			"b": {Type: ColumnTypeString},
			"c": {Type: ColumnTypeString},
		},
	}
	first, err := yaml.Marshal(def)
	if err != nil {
		t.Fatalf("marshal 1: %v", err)
	}
	for i := 0; i < 50; i++ {
		next, err := yaml.Marshal(def)
		if err != nil {
			t.Fatalf("marshal iter %d: %v", i, err)
		}
		if string(next) != string(first) {
			t.Fatalf("non-deterministic output at iter %d", i)
		}
	}
}
```

- [ ] **Step 2: Run tests; verify they fail**

Run: `go test -timeout=10s ./pkg/ingitdb/ -run TestCollectionDef_MarshalYAML -v`
Expected: FAIL — either compile error (`undefined: TestCollectionDef_MarshalYAML…` won't fire, but order assertions WILL fail because Go map order is random).

- [ ] **Step 3: Write `CollectionDef.MarshalYAML`**

Create `pkg/ingitdb/collection_marshal.go`:

```go
package ingitdb

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// MarshalYAML emits a CollectionDef as a YAML mapping with deterministic
// column ordering: columns named in ColumnsOrder appear in that order;
// any remaining columns follow in alphabetical order. The other fields
// keep their struct-declaration order.
//
// This makes any yaml.Marshal of a CollectionDef diff-stable across
// runs and across machines.
func (c *CollectionDef) MarshalYAML() (interface{}, error) {
	if c == nil {
		return nil, nil
	}
	root := &yaml.Node{Kind: yaml.MappingNode}

	addScalar := func(key, value string) {
		if value == "" {
			return
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}
	addNode := func(key string, node *yaml.Node) {
		if node == nil {
			return
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			node,
		)
	}

	if len(c.Titles) > 0 {
		titlesNode := &yaml.Node{}
		_ = titlesNode.Encode(c.Titles)
		addNode("titles", titlesNode)
	}
	if c.RecordFile != nil {
		rfNode := &yaml.Node{}
		if err := rfNode.Encode(c.RecordFile); err != nil {
			return nil, err
		}
		addNode("record_file", rfNode)
	}
	addScalar("data_dir", c.DataDir)

	columnsNode := orderedColumnsNode(c.Columns, c.ColumnsOrder)
	if columnsNode != nil {
		addNode("columns", columnsNode)
	}

	if len(c.ColumnsOrder) > 0 {
		coNode := &yaml.Node{}
		_ = coNode.Encode(c.ColumnsOrder)
		addNode("columns_order", coNode)
	}
	if len(c.PrimaryKey) > 0 {
		pkNode := &yaml.Node{}
		_ = pkNode.Encode(c.PrimaryKey)
		addNode("primary_key", pkNode)
	}
	if c.DefaultView != nil {
		dvNode := &yaml.Node{}
		if err := dvNode.Encode(c.DefaultView); err != nil {
			return nil, err
		}
		addNode("default_view", dvNode)
	}
	if c.Readme != nil {
		rNode := &yaml.Node{}
		if err := rNode.Encode(c.Readme); err != nil {
			return nil, err
		}
		addNode("readme", rNode)
	}
	return root, nil
}

// orderedColumnsNode returns a MappingNode containing every entry from
// columns, with keys ordered by columnsOrder followed by alphabetical
// for anything not listed. Returns nil if columns is empty.
func orderedColumnsNode(columns map[string]*ColumnDef, columnsOrder []string) *yaml.Node {
	if len(columns) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(columns))
	keys := make([]string, 0, len(columns))
	for _, k := range columnsOrder {
		if _, ok := columns[k]; ok && !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	tail := make([]string, 0, len(columns))
	for k := range columns {
		if !seen[k] {
			tail = append(tail, k)
		}
	}
	sort.Strings(tail)
	keys = append(keys, tail...)

	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		colNode := &yaml.Node{}
		_ = colNode.Encode(columns[k])
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			colNode,
		)
	}
	return node
}
```

- [ ] **Step 4: Run tests; verify they pass**

Run: `go test -timeout=10s ./pkg/ingitdb/ -run TestCollectionDef_MarshalYAML -v`
Expected: all three tests PASS.

- [ ] **Step 5: Run the full package's tests to catch regressions**

Run: `go test -timeout=10s ./pkg/ingitdb/...`
Expected: all PASS. If any test that compares marshalled output fails, fix this task before committing — likely the regression is a test that hand-rolled an expected YAML string with a specific column order; reorder the expected literal to match the new deterministic order.

- [ ] **Step 6: Commit**

```bash
git add pkg/ingitdb/collection_marshal.go pkg/ingitdb/collection_marshal_test.go
git commit -m "feat(ingitdb): deterministic CollectionDef YAML output

Adds MarshalYAML on CollectionDef that orders columns by columns_order,
then alphabetically. Makes yaml.Marshal output diff-stable across runs.

specscore: feature/cli/describe"
```

---

## Task 2: `resolveFormat` — pure format-flag resolver

**Files:**
- Create: `cmd/ingitdb/commands/describe_format.go`
- Create: `cmd/ingitdb/commands/describe_format_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/ingitdb/commands/describe_format_test.go`:

```go
package commands

import (
	"strings"
	"testing"
)

func TestResolveFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		raw       string
		engine    string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{name: "default_yaml_for_empty", raw: "", engine: "ingitdb", want: "yaml"},
		{name: "yaml_explicit", raw: "yaml", engine: "ingitdb", want: "yaml"},
		{name: "json", raw: "json", engine: "ingitdb", want: "json"},
		{name: "native_on_ingitdb_is_yaml", raw: "native", engine: "ingitdb", want: "yaml"},
		{name: "native_on_sql_engine_is_sql", raw: "native", engine: "sqlite", want: "sql"},
		{name: "sql_on_ingitdb_errors", raw: "sql", engine: "ingitdb", wantErr: true,
			errSubstr: `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`},
		{name: "SQL_case_insensitive_on_ingitdb_errors", raw: "SQL", engine: "ingitdb", wantErr: true,
			errSubstr: `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`},
		{name: "sql_on_sql_engine_passes", raw: "sql", engine: "sqlite", want: "sql"},
		{name: "unknown_value_lists_options", raw: "xml", engine: "ingitdb", wantErr: true,
			errSubstr: "valid values: yaml, json, native, sql"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveFormat(tc.raw, tc.engine)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error containing %q, got nil; canonical=%q", tc.errSubstr, got)
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q missing substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test; verify it fails**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestResolveFormat -v`
Expected: compile error — `undefined: resolveFormat`.

- [ ] **Step 3: Implement `resolveFormat`**

Create `cmd/ingitdb/commands/describe_format.go`:

```go
package commands

// specscore: feature/cli/describe

import (
	"fmt"
	"strings"
)

const (
	// engineIngitDB is the engine identifier the CLI passes when the
	// describe command runs against an ingitdb project. resolveFormat
	// uses it to determine that "sql" is not a supported native output.
	engineIngitDB = "ingitdb"
)

// resolveFormat normalises the user-supplied --format value into one of
// the canonical output formats {yaml, json, sql}.
//
//   - empty   → "yaml" (the documented default)
//   - "yaml"  → "yaml"
//   - "json"  → "json"
//   - "native"→ the engine's canonical format ("yaml" for ingitdb;
//     "sql" for any non-ingitdb engine)
//   - "sql"/"SQL" → routes to native; for the ingitdb engine this is
//     reported as an error so the caller surfaces it to the user.
//
// Any other value produces an error listing the accepted values.
func resolveFormat(raw, engine string) (string, error) {
	switch strings.ToLower(raw) {
	case "":
		return "yaml", nil
	case "yaml":
		return "yaml", nil
	case "json":
		return "json", nil
	case "native":
		if engine == engineIngitDB {
			return "yaml", nil
		}
		return "sql", nil
	case "sql":
		if engine == engineIngitDB {
			return "", fmt.Errorf(`engine %q native format is "yaml"; use --format=yaml or --format=native`, engine)
		}
		return "sql", nil
	default:
		return "", fmt.Errorf("invalid --format value %q (valid values: yaml, json, native, sql)", raw)
	}
}
```

- [ ] **Step 4: Run test; verify it passes**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestResolveFormat -v`
Expected: all subtests PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/describe_format.go cmd/ingitdb/commands/describe_format_test.go
git commit -m "feat(cli): add resolveFormat for describe --format flag

Pure resolver mapping user input to canonical {yaml, json, sql}.
'native' resolves per-engine; on ingitdb that is yaml. 'sql'/'SQL'
routes to native and errors on ingitdb with a message that points the
user at --format=yaml.

The grammar is shared with the planned datatug describe command.

specscore: feature/cli/describe"
```

---

## Task 3: `describe_output` — pure payload builders

**Files:**
- Create: `cmd/ingitdb/commands/describe_output.go`
- Create: `cmd/ingitdb/commands/describe_output_test.go`

- [ ] **Step 1: Write the failing tests**

Create `cmd/ingitdb/commands/describe_output_test.go`:

```go
package commands

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestBuildCollectionPayload_BasicShape(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		ID:         "users",
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
		Columns: map[string]*ingitdb.ColumnDef{
			"id":    {Type: ingitdb.ColumnTypeString},
			"email": {Type: ingitdb.ColumnTypeString},
		},
	}
	ctx := collectionOutputCtx{relPath: "users", viewNames: nil, subcollectionNames: nil}
	node, err := buildCollectionPayload(col, ctx)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out, err := yaml.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, want := range []string{
		"definition:", "_meta:",
		"id: users", "kind: collection",
		"definition_path: users", "data_path: users",
		"views: []", "subcollections: []",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in output:\n%s", want, s)
		}
	}
}

func TestBuildCollectionPayload_DataDirDivergence(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		ID:         "events",
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
		Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		DataDir:    "../events-archive",
	}
	ctx := collectionOutputCtx{relPath: "events"}
	node, err := buildCollectionPayload(col, ctx)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out, _ := yaml.Marshal(node)
	s := string(out)
	if !strings.Contains(s, "definition_path: events") {
		t.Errorf("missing definition_path: events; got:\n%s", s)
	}
	if !strings.Contains(s, "data_path: events-archive") {
		t.Errorf("missing data_path: events-archive; got:\n%s", s)
	}
}

func TestBuildCollectionPayload_SortedViewsAndSubcollections(t *testing.T) {
	t.Parallel()
	col := &ingitdb.CollectionDef{
		ID:         "users",
		RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
		Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
	}
	ctx := collectionOutputCtx{
		relPath:            "users",
		viewNames:          []string{"top_buyers", "active_users"},
		subcollectionNames: []string{"sessions", "orders"},
	}
	node, _ := buildCollectionPayload(col, ctx)
	out, _ := yaml.Marshal(node)
	s := string(out)
	activeIdx := strings.Index(s, "active_users")
	topIdx := strings.Index(s, "top_buyers")
	if !(activeIdx > 0 && topIdx > activeIdx) {
		t.Errorf("expected views sorted [active_users, top_buyers]; got:\n%s", s)
	}
	ordersIdx := strings.Index(s, "orders")
	sessionsIdx := strings.Index(s, "sessions")
	if !(ordersIdx > 0 && sessionsIdx > ordersIdx) {
		t.Errorf("expected subcollections sorted [orders, sessions]; got:\n%s", s)
	}
}

func TestBuildViewPayload_BasicShape(t *testing.T) {
	t.Parallel()
	view := &ingitdb.ViewDef{
		ID:       "top_buyers",
		OrderBy:  "total_spend DESC",
		Top:      100,
		Template: "md-table",
		FileName: "top-buyers.md",
	}
	node, err := buildViewPayload(view, viewOutputCtx{
		owningCollection: "users",
		relPath:          "users/$views/top_buyers.yaml",
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	out, _ := yaml.Marshal(node)
	s := string(out)
	for _, want := range []string{
		"definition:", "_meta:",
		"id: top_buyers", "kind: view",
		"collection: users",
		"definition_path: users/$views/top_buyers.yaml",
		"order_by: total_spend DESC",
		"top: 100",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in output:\n%s", want, s)
		}
	}
}
```

- [ ] **Step 2: Run test; verify it fails**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run "TestBuildCollectionPayload|TestBuildViewPayload" -v`
Expected: compile error — undefined `buildCollectionPayload`, `buildViewPayload`, `collectionOutputCtx`, `viewOutputCtx`.

- [ ] **Step 3: Implement the builders**

Create `cmd/ingitdb/commands/describe_output.go`:

```go
package commands

// specscore: feature/cli/describe

import (
	"path"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// collectionOutputCtx is the per-invocation context the payload builder
// needs that isn't on CollectionDef itself: the collection's location
// inside the database (forward-slash, relative to db root) and the
// names of its views and subcollections (passed in pre-discovered so
// the builder stays pure).
type collectionOutputCtx struct {
	relPath            string
	viewNames          []string
	subcollectionNames []string
}

// viewOutputCtx is the equivalent for a view: the id of the owning
// root collection and the view file's path relative to db root.
type viewOutputCtx struct {
	owningCollection string
	relPath          string
}

// buildCollectionPayload assembles the {definition, _meta} document
// for a collection as a *yaml.Node. The caller chooses the wire
// format (yaml.Marshal or json.Marshal of the same node via a
// node→interface conversion).
func buildCollectionPayload(col *ingitdb.CollectionDef, ctx collectionOutputCtx) (*yaml.Node, error) {
	defNode := &yaml.Node{}
	if err := defNode.Encode(col); err != nil {
		return nil, err
	}
	dataPath := ctx.relPath
	if col.DataDir != "" {
		dataPath = path.Clean(path.Join(ctx.relPath, col.DataDir))
	}
	meta := orderedMap(
		kv("id", col.ID),
		kv("kind", "collection"),
		kv("definition_path", ctx.relPath),
		kv("data_path", dataPath),
		kvList("views", sortedCopy(ctx.viewNames)),
		kvList("subcollections", sortedCopy(ctx.subcollectionNames)),
	)
	return docNode(defNode, meta), nil
}

// buildViewPayload assembles the {definition, _meta} document for a
// view.
func buildViewPayload(view *ingitdb.ViewDef, ctx viewOutputCtx) (*yaml.Node, error) {
	defNode := &yaml.Node{}
	if err := defNode.Encode(view); err != nil {
		return nil, err
	}
	meta := orderedMap(
		kv("id", view.ID),
		kv("kind", "view"),
		kv("collection", ctx.owningCollection),
		kv("definition_path", ctx.relPath),
	)
	return docNode(defNode, meta), nil
}

// docNode wraps two child nodes under top-level keys "definition" and
// "_meta", in that order.
func docNode(def, meta *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "definition"}, def,
			{Kind: yaml.ScalarNode, Value: "_meta"}, meta,
		},
	}
}

// orderedMap builds a MappingNode preserving caller-given order.
func orderedMap(pairs ...[2]*yaml.Node) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range pairs {
		m.Content = append(m.Content, p[0], p[1])
	}
	return m
}

func kv(key, value string) [2]*yaml.Node {
	return [2]*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key},
		{Kind: yaml.ScalarNode, Value: value},
	}
}

func kvList(key string, values []string) [2]*yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
	}
	return [2]*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key},
		seq,
	}
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
```

- [ ] **Step 4: Run test; verify it passes**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run "TestBuildCollectionPayload|TestBuildViewPayload" -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/describe_output.go cmd/ingitdb/commands/describe_output_test.go
git commit -m "feat(cli): add describe payload builders

buildCollectionPayload and buildViewPayload return *yaml.Node
documents shaped as {definition, _meta}. Pure functions: take the
def + minimal context, no I/O. The CLI command in a later task wraps
these with source resolution and format selection.

specscore: feature/cli/describe"
```

---

## Task 4: `describe.go` cobra skeleton

Wire up the cobra plumbing (parent command, persistent flags, kind dispatch, alias) without implementing the kind handlers. Each kind handler returns `errNotYetImplemented` so we can land an end-to-end build and table-test the wiring before the meat goes in.

**Files:**
- Create: `cmd/ingitdb/commands/describe.go`
- Create: `cmd/ingitdb/commands/describe_test.go`

- [ ] **Step 1: Write the failing test**

Create `cmd/ingitdb/commands/describe_test.go`:

```go
package commands

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestDescribe_ReturnsCommand(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	if cmd == nil {
		t.Fatal("Describe() returned nil")
	}
	if cmd.Use != "describe <kind> <name>" {
		t.Errorf("unexpected Use: %q", cmd.Use)
	}
	gotAliases := cmd.Aliases
	if len(gotAliases) != 1 || gotAliases[0] != "desc" {
		t.Errorf("expected desc alias; got %v", gotAliases)
	}
	if len(cmd.Commands()) != 2 {
		t.Fatalf("expected 2 subcommands (collection, view); got %d", len(cmd.Commands()))
	}
}

func TestDescribe_RootRequiresKind(t *testing.T) {
	t.Parallel()
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return "/tmp/db", nil }
	readDef := func(_ string, _ ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
		return &ingitdb.Definition{}, nil
	}
	cmd := Describe(homeDir, getWd, readDef)
	err := runCobraCommand(cmd)
	if err == nil {
		t.Fatal("expected error when invoked without a kind")
	}
}
```

- [ ] **Step 2: Run test; verify it fails**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribe_ -v`
Expected: compile error — `undefined: Describe`.

- [ ] **Step 3: Implement the skeleton**

Create `cmd/ingitdb/commands/describe.go`:

```go
package commands

// specscore: feature/cli/describe

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// Describe returns the `ingitdb describe` command. Two kinds are
// supported: `describe collection <name>` (alias `table`) and
// `describe view <name>` (with `--in=<collection>` to disambiguate).
// `desc` is registered as a top-level alias for `describe`.
func Describe(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "describe <kind> <name>",
		Aliases: []string{"desc"},
		Short:   "Describe a schema object (collection or view)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return bareNameDescribe(cmd, args[0], homeDir, getWd, readDefinition)
			}
			return fmt.Errorf("describe requires a kind: collection or view")
		},
	}
	cmd.PersistentFlags().String("path", "", "path to the database directory (default: current directory)")
	cmd.PersistentFlags().String("remote", "",
		"remote repository, e.g. github.com/owner/repo[@branch|tag|commit] "+
			"(mutually exclusive with --path)")
	cmd.PersistentFlags().String("token", "",
		"personal access token; falls back to host-derived env vars "+
			"(e.g. GITHUB_TOKEN for github.com)")
	cmd.PersistentFlags().String("provider", "",
		"explicit provider id (github, gitlab, bitbucket)")
	cmd.PersistentFlags().String("format", "",
		"output format: yaml (default), json, native, sql")

	cmd.AddCommand(
		describeCollectionCmd(homeDir, getWd, readDefinition),
		describeViewCmd(homeDir, getWd, readDefinition),
	)
	return cmd
}

// bareNameDescribe is invoked when the user runs `describe <name>`
// without a kind. The full implementation lives in Task 7; this stub
// keeps the wiring honest.
func bareNameDescribe(
	_ *cobra.Command,
	name string,
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	_ = name
	return fmt.Errorf("describe: bare-name resolution not yet implemented")
}

func describeCollectionCmd(
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection <name>",
		Aliases: []string{"table"},
		Short:   "Describe a collection (schema, columns, primary key, views)",
		Args:    cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("describe collection: not yet implemented")
		},
	}
	return cmd
}

func describeViewCmd(
	_ func() (string, error),
	_ func() (string, error),
	_ func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Describe a view (definition, source, template)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, _ []string) error {
			return fmt.Errorf("describe view: not yet implemented")
		},
	}
	cmd.Flags().String("in", "", "limit the search to a specific collection (disambiguates duplicate view names)")
	return cmd
}
```

- [ ] **Step 4: Run test; verify it passes**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribe_ -v`
Expected: PASS.

- [ ] **Step 5: Confirm full build is green**

Run: `go build ./...`
Expected: no errors. `commands.Describe` is now resolved in `main.go`.

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/describe.go cmd/ingitdb/commands/describe_test.go
git commit -m "feat(cli): scaffold describe command with desc alias

Adds parent describe command (alias desc) plus stub collection and
view subcommands (collection has 'table' alias). Each subcommand
returns 'not yet implemented' pending Tasks 5-7.

specscore: feature/cli/describe"
```

---

## Task 5: `describe collection <name>` — local mode

**Files:**
- Modify: `cmd/ingitdb/commands/describe.go` (replace `describeCollectionCmd` body)
- Modify: `cmd/ingitdb/commands/describe_test.go` (add ACs 1, 2, 3, 10, 11, 12, 13, 14, 16, 17, 18)

- [ ] **Step 1: Write the failing tests**

Add to `cmd/ingitdb/commands/describe_test.go`:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	// keep existing imports + add:
	"path/filepath"
	"gopkg.in/yaml.v3"
)

// describeFixtureDB builds an on-disk database with one or more
// collections; each collection has an empty $views/ dir unless views
// is non-empty. Returns the absolute root dir.
func describeFixtureDB(t *testing.T, collections map[string]*ingitdb.CollectionDef, views map[string]map[string]*ingitdb.ViewDef) string {
	t.Helper()
	root := t.TempDir()
	// Write .ingitdb/root-collections.yaml
	rootColls := make(map[string]string)
	for id := range collections {
		rootColls[id] = id
	}
	if err := os.MkdirAll(filepath.Join(root, ".ingitdb"), 0o755); err != nil {
		t.Fatal(err)
	}
	rcBytes, _ := yaml.Marshal(rootColls)
	if err := os.WriteFile(filepath.Join(root, ".ingitdb", "root-collections.yaml"), rcBytes, 0o644); err != nil {
		t.Fatal(err)
	}
	// Write each collection's .collection/definition.yaml + view files
	for id, def := range collections {
		colDir := filepath.Join(root, id)
		if err := os.MkdirAll(filepath.Join(colDir, ".collection"), 0o755); err != nil {
			t.Fatal(err)
		}
		raw, _ := yaml.Marshal(def)
		if err := os.WriteFile(filepath.Join(colDir, ".collection", "definition.yaml"), raw, 0o644); err != nil {
			t.Fatal(err)
		}
		for vName, vDef := range views[id] {
			viewsDir := filepath.Join(colDir, "$views")
			if err := os.MkdirAll(viewsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			vRaw, _ := yaml.Marshal(vDef)
			if err := os.WriteFile(filepath.Join(viewsDir, vName+".yaml"), vRaw, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root
}

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	<-done
	return buf.String()
}

func TestDescribeCollection_LocalYAML_Shape(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
			},
		},
	}, nil)
	homeDir := func() (string, error) { return "/tmp/home", nil }
	getWd := func() (string, error) { return dir, nil }
	cmd := Describe(homeDir, getWd, ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		if err := runCobraCommand(cmd, "collection", "users", "--path="+dir); err != nil {
			t.Fatalf("collection users: %v", err)
		}
	})
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse yaml: %v\nout:\n%s", err, out)
	}
	if _, ok := parsed["definition"]; !ok {
		t.Errorf("missing definition key")
	}
	meta, ok := parsed["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing _meta map; got %T", parsed["_meta"])
	}
	checks := map[string]any{
		"id":              "users",
		"kind":            "collection",
		"definition_path": "users",
		"data_path":       "users",
	}
	for k, want := range checks {
		if meta[k] != want {
			t.Errorf("_meta.%s = %v; want %v", k, meta[k], want)
		}
	}
}

func TestDescribeCollection_TableAliasEquivalent(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	collOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	tableOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "table", "users", "--path="+dir)
	})
	if collOut != tableOut {
		t.Errorf("table alias produced different output:\n--collection--\n%s\n--table--\n%s", collOut, tableOut)
	}
}

func TestDescribeCollection_NotFound(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "widgets", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `collection "widgets" not found`) {
		t.Fatalf("want not-found error; got: %v", err)
	}
}

func TestDescribeCollection_JSONFormat(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=json")
	})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse json: %v\nout:\n%s", err, out)
	}
	if _, ok := parsed["definition"]; !ok {
		t.Errorf("json missing definition key")
	}
	if _, ok := parsed["_meta"]; !ok {
		t.Errorf("json missing _meta key")
	}
}

func TestDescribeCollection_SQLFormatErrors(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=sql")
	if err == nil || !strings.Contains(err.Error(),
		`engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`) {
		t.Fatalf("want SQL-on-ingitdb error; got: %v", err)
	}
}

func TestDescribeCollection_NativeResolvesToYAML(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	yamlOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=yaml")
	})
	nativeOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=native")
	})
	if yamlOut != nativeOut {
		t.Errorf("--format=native ≠ --format=yaml on ingitdb")
	}
}
```

At the top of `describe_test.go` also add this helper (used by every test from now on):

```go
var ingitdbValidatorReadDef = func(p string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return validator.ReadDefinition(p, opts...)
}
```

…and add the import `"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"`.

- [ ] **Step 2: Run tests; verify they fail**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeCollection -v`
Expected: every new test fails with "not yet implemented".

- [ ] **Step 3: Implement `describeCollectionCmd`**

In `cmd/ingitdb/commands/describe.go`, replace the entire `describeCollectionCmd` function with:

```go
func describeCollectionCmd(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "collection <name>",
		Aliases: []string{"table"},
		Short:   "Describe a collection (schema, columns, primary key, views)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return runDescribeCollection(cmd, name, homeDir, getWd, readDefinition)
		},
	}
	return cmd
}

// runDescribeCollection is split out so the bare-name resolver in
// Task 7 can call it with the same dependencies.
func runDescribeCollection(
	cmd *cobra.Command,
	name string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	pathVal, _ := cmd.Flags().GetString("path")
	remoteVal, _ := cmd.Flags().GetString("remote")
	if pathVal != "" && remoteVal != "" {
		return fmt.Errorf("--path and --remote are mutually exclusive")
	}
	if remoteVal != "" {
		return fmt.Errorf("describe --remote not yet implemented")
	}

	rawFormat, _ := cmd.Flags().GetString("format")
	format, err := resolveFormat(rawFormat, engineIngitDB)
	if err != nil {
		return err
	}

	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return err
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return fmt.Errorf("failed to read database definition: %w", readErr)
	}
	col, ok := def.Collections[name]
	if !ok {
		return fmt.Errorf("collection %q not found in database at %s", name, dirPath)
	}

	views, subcols, err := discoverCollectionChildren(dirPath, name)
	if err != nil {
		return err
	}

	node, err := buildCollectionPayload(col, collectionOutputCtx{
		relPath:            name,
		viewNames:          views,
		subcollectionNames: subcols,
	})
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}
	return emitNode(node, format)
}

// discoverCollectionChildren walks the on-disk collection directory
// to find views (under $views/) and subcollections (directories that
// are neither $views nor .collection and contain a .collection/
// subdirectory). Returns names sorted by buildCollectionPayload.
func discoverCollectionChildren(dbDir, colName string) (views, subcols []string, err error) {
	colDir := filepath.Join(dbDir, colName)
	viewsDir := filepath.Join(colDir, "$views")
	if entries, statErr := os.ReadDir(viewsDir); statErr == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if strings.HasSuffix(e.Name(), ".yaml") {
				views = append(views, strings.TrimSuffix(e.Name(), ".yaml"))
			}
		}
	}
	entries, _ := os.ReadDir(colDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "$views" || e.Name() == ".collection" {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(colDir, e.Name(), ".collection")); statErr == nil {
			subcols = append(subcols, e.Name())
		}
	}
	return
}

// emitNode writes a yaml.Node to stdout in the chosen format.
func emitNode(node *yaml.Node, format string) error {
	switch format {
	case "yaml":
		out, err := yaml.Marshal(node)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, _ = fmt.Fprint(os.Stdout, string(out))
		return nil
	case "json":
		var v any
		if err := node.Decode(&v); err != nil {
			return fmt.Errorf("convert node: %w", err)
		}
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal json: %w", err)
		}
		_, _ = fmt.Fprintln(os.Stdout, string(raw))
		return nil
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
}
```

At the top of `describe.go`, expand the imports to:

```go
import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)
```

- [ ] **Step 4: Run tests; verify they pass**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeCollection -v`
Expected: every test PASS.

- [ ] **Step 5: Verify the full commands package still builds and other tests still pass**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/...`
Expected: PASS (including the existing list, drop, etc. tests).

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/describe.go cmd/ingitdb/commands/describe_test.go
git commit -m "feat(cli): implement describe collection (local mode)

Implements describe collection <name> with the table alias, the
--format flag (yaml/json/native/sql/SQL), and the {definition, _meta}
output shape. Walks \$views/ and subcollection directories to populate
_meta.views and _meta.subcollections. Resolves data_path from the
collection's data_dir when set.

Remote mode is accepted at the flag layer and errors with
'describe --remote not yet implemented'; behavioral remote support
is a follow-up plan.

specscore: feature/cli/describe"
```

---

## Task 6: `describe view <name>` — local mode with `--in`

**Files:**
- Modify: `cmd/ingitdb/commands/describe.go` (replace `describeViewCmd` body, add helpers)
- Modify: `cmd/ingitdb/commands/describe_test.go` (add ACs 4, 5, 6, 7, 15)

- [ ] **Step 1: Write the failing tests**

Append to `cmd/ingitdb/commands/describe_test.go`:

```go
func TestDescribeView_BasicShape(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users": {
				"top_buyers": {OrderBy: "total_spend DESC", Top: 100, Template: "md-table"},
			},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		if err := runCobraCommand(cmd, "view", "top_buyers", "--path="+dir); err != nil {
			t.Fatalf("view top_buyers: %v", err)
		}
	})
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	meta := parsed["_meta"].(map[string]any)
	if meta["kind"] != "view" || meta["collection"] != "users" {
		t.Errorf("unexpected meta: %v", meta)
	}
	if meta["definition_path"] != "users/$views/top_buyers.yaml" {
		t.Errorf("unexpected definition_path: %v", meta["definition_path"])
	}
}

func TestDescribeView_AmbiguousRequiresIn(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users":  {"recent": {Top: 10}},
			"orders": {"recent": {Top: 10}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "recent", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(),
		`view "recent" is ambiguous — exists in collections: [orders, users]; use --in=<collection>`) {
		t.Fatalf("want ambiguous error; got: %v", err)
	}
}

func TestDescribeView_ResolvedByIn(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"orders": {
				ID:         "orders",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users":  {"recent": {Top: 10}},
			"orders": {"recent": {Top: 20}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	out := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "view", "recent", "--in=orders", "--path="+dir)
	})
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	meta := parsed["_meta"].(map[string]any)
	if meta["collection"] != "orders" {
		t.Errorf("want collection=orders; got %v", meta["collection"])
	}
}

func TestDescribeView_InCollectionMissing(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		nil,
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "anything", "--in=ghosts", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `collection "ghosts" (from --in) not found`) {
		t.Fatalf("want missing --in error; got: %v", err)
	}
}

func TestDescribeView_NotFoundAnywhere(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		nil,
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "view", "ghost", "--path="+dir)
	if err == nil || !strings.Contains(err.Error(), `view "ghost" not found in any collection`) {
		t.Fatalf("want view-not-found error; got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests; verify they fail**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeView -v`
Expected: every new test fails with `view: not yet implemented`.

- [ ] **Step 3: Implement `describeViewCmd`**

Replace the entire `describeViewCmd` function in `cmd/ingitdb/commands/describe.go` with:

```go
func describeViewCmd(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "view <name>",
		Short: "Describe a view (definition, source, template)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			scopeCol, _ := cmd.Flags().GetString("in")
			return runDescribeView(cmd, name, scopeCol, homeDir, getWd, readDefinition)
		},
	}
	cmd.Flags().String("in", "", "limit the search to a specific collection (disambiguates duplicate view names)")
	return cmd
}

func runDescribeView(
	cmd *cobra.Command,
	name, scopeCol string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	pathVal, _ := cmd.Flags().GetString("path")
	remoteVal, _ := cmd.Flags().GetString("remote")
	if pathVal != "" && remoteVal != "" {
		return fmt.Errorf("--path and --remote are mutually exclusive")
	}
	if remoteVal != "" {
		return fmt.Errorf("describe --remote not yet implemented")
	}

	rawFormat, _ := cmd.Flags().GetString("format")
	format, err := resolveFormat(rawFormat, engineIngitDB)
	if err != nil {
		return err
	}

	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return err
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return fmt.Errorf("failed to read database definition: %w", readErr)
	}

	if scopeCol != "" {
		if _, ok := def.Collections[scopeCol]; !ok {
			return fmt.Errorf("collection %q (from --in) not found", scopeCol)
		}
	}

	matches := findViewMatches(dirPath, def, name, scopeCol)
	switch len(matches) {
	case 0:
		if scopeCol != "" {
			return fmt.Errorf("view %q not found in collection %q", name, scopeCol)
		}
		return fmt.Errorf("view %q not found in any collection", name)
	case 1:
		// fall through
	default:
		cols := make([]string, 0, len(matches))
		for _, m := range matches {
			cols = append(cols, m.collection)
		}
		sort.Strings(cols)
		return fmt.Errorf(
			"view %q is ambiguous — exists in collections: [%s]; use --in=<collection>",
			name, strings.Join(cols, ", "),
		)
	}

	m := matches[0]
	raw, readErr := os.ReadFile(m.absPath)
	if readErr != nil {
		return fmt.Errorf("read view file %s: %w", m.relPath, readErr)
	}
	viewDef := &ingitdb.ViewDef{}
	if uErr := yaml.Unmarshal(raw, viewDef); uErr != nil {
		return fmt.Errorf("parse view file %s: %w", m.relPath, uErr)
	}
	viewDef.ID = name

	node, err := buildViewPayload(viewDef, viewOutputCtx{
		owningCollection: m.collection,
		relPath:          m.relPath,
	})
	if err != nil {
		return fmt.Errorf("build payload: %w", err)
	}
	return emitNode(node, format)
}

type viewMatch struct {
	collection string
	absPath    string
	relPath    string
}

// findViewMatches walks each collection's $views/ directory looking
// for <name>.yaml. When scopeCol is non-empty, restricts the search
// to that collection.
func findViewMatches(dbDir string, def *ingitdb.Definition, name, scopeCol string) []viewMatch {
	var out []viewMatch
	for colID := range def.Collections {
		if scopeCol != "" && colID != scopeCol {
			continue
		}
		rel := colID + "/$views/" + name + ".yaml"
		abs := filepath.Join(dbDir, colID, "$views", name+".yaml")
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		out = append(out, viewMatch{collection: colID, absPath: abs, relPath: rel})
	}
	return out
}
```

Add `"sort"` to the imports if not already present.

- [ ] **Step 4: Run tests; verify they pass**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeView -v`
Expected: every new test PASS.

- [ ] **Step 5: Run the full package test suite**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/...`
Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd/ingitdb/commands/describe.go cmd/ingitdb/commands/describe_test.go
git commit -m "feat(cli): implement describe view (local mode)

describe view <name> walks each root collection's \$views/ dir, errors
on ambiguity with a sorted collections list, supports --in to scope.
Reports a clear error when --in names a missing collection.

specscore: feature/cli/describe"
```

---

## Task 7: Bare-name resolution

**Files:**
- Modify: `cmd/ingitdb/commands/describe.go` (replace `bareNameDescribe` body)
- Modify: `cmd/ingitdb/commands/describe_test.go` (add ACs 8, 9)

- [ ] **Step 1: Write the failing tests**

Append to `describe_test.go`:

```go
func TestDescribeBareName_ResolvesToCollection(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	bareOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "users", "--path="+dir)
	})
	collOut := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	if bareOut != collOut {
		t.Errorf("bare-name output differs from explicit collection")
	}
}

func TestDescribeBareName_AmbiguousErrors(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t,
		map[string]*ingitdb.CollectionDef{
			"archive": {
				ID:         "archive",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
			"users": {
				ID:         "users",
				RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
				Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
			},
		},
		map[string]map[string]*ingitdb.ViewDef{
			"users": {"archive": {Top: 10}},
		},
	)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "archive", "--path="+dir)
	if err == nil ||
		!strings.Contains(err.Error(), `name "archive" is ambiguous`) ||
		!strings.Contains(err.Error(), `'describe collection archive'`) ||
		!strings.Contains(err.Error(), `'describe view archive'`) {
		t.Fatalf("want ambiguous-name error; got: %v", err)
	}
}
```

- [ ] **Step 2: Run tests; verify they fail**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeBareName -v`
Expected: both fail with `bare-name resolution not yet implemented`.

- [ ] **Step 3: Implement `bareNameDescribe`**

Replace `bareNameDescribe` in `describe.go` with:

```go
func bareNameDescribe(
	cmd *cobra.Command,
	name string,
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
) error {
	pathVal, _ := cmd.Flags().GetString("path")
	remoteVal, _ := cmd.Flags().GetString("remote")
	if pathVal != "" && remoteVal != "" {
		return fmt.Errorf("--path and --remote are mutually exclusive")
	}
	if remoteVal != "" {
		return fmt.Errorf("describe --remote not yet implemented")
	}

	dirPath, err := resolveDBPath(cmd, homeDir, getWd)
	if err != nil {
		return err
	}
	def, readErr := readDefinition(dirPath)
	if readErr != nil {
		return fmt.Errorf("failed to read database definition: %w", readErr)
	}

	_, collectionMatch := def.Collections[name]
	viewMatches := findViewMatches(dirPath, def, name, "")
	hasView := len(viewMatches) > 0

	switch {
	case collectionMatch && hasView:
		return fmt.Errorf(
			"name %q is ambiguous — exists as both collection and view; use 'describe collection %s' or 'describe view %s'",
			name, name, name,
		)
	case collectionMatch:
		return runDescribeCollection(cmd, name, homeDir, getWd, readDefinition)
	case hasView:
		return runDescribeView(cmd, name, "", homeDir, getWd, readDefinition)
	default:
		return fmt.Errorf("no collection or view named %q in database at %s", name, dirPath)
	}
}
```

- [ ] **Step 4: Run tests; verify they pass**

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run TestDescribeBareName -v`
Expected: both PASS.

- [ ] **Step 5: Commit**

```bash
git add cmd/ingitdb/commands/describe.go cmd/ingitdb/commands/describe_test.go
git commit -m "feat(cli): describe bare-name shortcut

\`ingitdb describe <name>\` resolves collections first, views second,
errors loudly on collection/view name collision pointing the user at
the explicit-kind form.

specscore: feature/cli/describe"
```

---

## Task 8: Mutual-exclusion AC + columns-order ACs

The remaining ACs are unknown-format-rejected (AC #13 — already exercised via TestResolveFormat in Task 2), source-selection-mutual-exclusion (AC #19), and the two columns-order ACs (#17, #18 — already exercised in Task 1's marshal tests, but we add CLI-level tests pinning the spec wording).

**Files:**
- Modify: `cmd/ingitdb/commands/describe_test.go` (final ACs)

- [ ] **Step 1: Write the failing tests**

Append to `describe_test.go`:

```go
func TestDescribeCollection_MutualExclusion(t *testing.T) {
	t.Parallel()
	cmd := Describe(
		func() (string, error) { return "/tmp", nil },
		func() (string, error) { return "/tmp", nil },
		ingitdbValidatorReadDef,
	)
	err := runCobraCommand(cmd, "collection", "users", "--path=/tmp", "--remote=github.com/owner/repo")
	if err == nil || !strings.Contains(err.Error(), "--path and --remote are mutually exclusive") {
		t.Fatalf("want mutual-exclusion error; got: %v", err)
	}
}

func TestDescribeCollection_UnknownFormatRejected(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns:    map[string]*ingitdb.ColumnDef{"id": {Type: ingitdb.ColumnTypeString}},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	err := runCobraCommand(cmd, "collection", "users", "--path="+dir, "--format=xml")
	if err == nil ||
		!strings.Contains(err.Error(), `invalid --format value "xml"`) ||
		!strings.Contains(err.Error(), "valid values: yaml, json, native, sql") {
		t.Fatalf("want unknown-format error; got: %v", err)
	}
}

func TestDescribeCollection_ColumnsOrderRespected_CLI(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
				"name":  {Type: ingitdb.ColumnTypeString},
			},
			ColumnsOrder: []string{"email", "id", "name"},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	first := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	second := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	if first != second {
		t.Errorf("output is not byte-identical across runs")
	}
	emailIdx := strings.Index(first, "email:")
	idIdx := strings.Index(first, "id:")
	nameIdx := strings.Index(first, "name:")
	if !(emailIdx > 0 && idIdx > emailIdx && nameIdx > idIdx) {
		t.Errorf("expected columns ordered email, id, name; got:\n%s", first)
	}
}

func TestDescribeCollection_ColumnsOrderAlphaFallback_CLI(t *testing.T) {
	t.Parallel()
	dir := describeFixtureDB(t, map[string]*ingitdb.CollectionDef{
		"users": {
			ID:         "users",
			RecordFile: &ingitdb.RecordFileDef{Name: "{key}.yaml", Format: "yaml", RecordType: ingitdb.SingleRecord},
			Columns: map[string]*ingitdb.ColumnDef{
				"id":    {Type: ingitdb.ColumnTypeString},
				"email": {Type: ingitdb.ColumnTypeString},
				"name":  {Type: ingitdb.ColumnTypeString},
			},
		},
	}, nil)
	cmd := Describe(func() (string, error) { return "/tmp", nil },
		func() (string, error) { return dir, nil },
		ingitdbValidatorReadDef)
	first := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	second := captureStdout(t, func() {
		_ = runCobraCommand(cmd, "collection", "users", "--path="+dir)
	})
	if first != second {
		t.Errorf("output is not byte-identical across runs")
	}
	emailIdx := strings.Index(first, "email:")
	idIdx := strings.Index(first, "id:")
	nameIdx := strings.Index(first, "name:")
	if !(emailIdx > 0 && idIdx > emailIdx && nameIdx > idIdx) {
		t.Errorf("expected alphabetical email, id, name; got:\n%s", first)
	}
}
```

- [ ] **Step 2: Run; verify they pass on first try (no implementation needed)**

These ACs are exercised by code already in place after Tasks 1-7:
- mutual-exclusion is checked in each `run*` entry point.
- unknown-format is rejected by `resolveFormat`.
- columns-order determinism comes from `CollectionDef.MarshalYAML` from Task 1.

Run: `go test -timeout=10s ./cmd/ingitdb/commands/ -run "TestDescribeCollection_MutualExclusion|TestDescribeCollection_UnknownFormatRejected|TestDescribeCollection_ColumnsOrder" -v`
Expected: all PASS.

If one fails, that's information — investigate before patching. Likely candidates: the mutual-exclusion check needs to fire *before* `cobra.ExactArgs` (it does, because the args are valid in this test). The unknown-format error wording must match the spec exactly.

- [ ] **Step 3: Commit**

```bash
git add cmd/ingitdb/commands/describe_test.go
git commit -m "test(cli): final describe AC coverage

CLI-level tests for: --path/--remote mutual exclusion, unknown
--format rejection, columns_order determinism, alphabetical fallback
determinism. These exercise behavior already implemented in Tasks 1-7.

specscore: feature/cli/describe"
```

---

## Task 9: Integration smoke test + lint + spec verification

**Files:**
- Modify: `cmd/ingitdb/main.go` (verify registration; no expected change)

- [ ] **Step 1: Confirm `main.go` still wires `Describe`**

Run: `grep -n "commands.Describe" cmd/ingitdb/main.go`
Expected: one line, the one added by the earlier scaffold:
```
		commands.Describe(homeDir, getWd, readDefinition),
```

If it's missing, restore it inside the `rootCmd.AddCommand(...)` block, between `commands.List(...)` and `commands.Select(...)`.

- [ ] **Step 2: Run the full test suite**

Run: `go test -timeout=10s ./...`
Expected: all PASS.

- [ ] **Step 3: Build the binary**

Run: `go build -o /tmp/ingitdb-describe-smoke ./cmd/ingitdb`
Expected: no errors, binary produced.

- [ ] **Step 4: End-to-end smoke test against a freshly built database**

```bash
# Create a tiny fixture
mkdir -p /tmp/sm/users/.collection /tmp/sm/.ingitdb
cat >/tmp/sm/.ingitdb/root-collections.yaml <<'EOF'
users: users
EOF
cat >/tmp/sm/users/.collection/definition.yaml <<'EOF'
record_file:
  name: "{key}.yaml"
  format: yaml
  type: SingleRecord
columns:
  id:    {type: string}
  email: {type: string}
columns_order: [id, email]
primary_key: [id]
EOF

/tmp/ingitdb-describe-smoke describe collection users --path=/tmp/sm
```

Expected stdout (first run; byte-identical on every subsequent run):

```yaml
definition:
  record_file:
    name: '{key}.yaml'
    format: yaml
    type: SingleRecord
  columns:
    id:
      type: string
    email:
      type: string
  columns_order:
    - id
    - email
  primary_key:
    - id
_meta:
  id: users
  kind: collection
  definition_path: users
  data_path: users
  views: []
  subcollections: []
```

Run the command again — output MUST be byte-identical (`diff <(./ingitdb describe ...) <(./ingitdb describe ...)` returns empty).

Also smoke `--format=json`, `--format=sql` (expect error), `table` alias, `desc` alias, bare name.

- [ ] **Step 5: Run lint**

Run: `golangci-lint run ./...`
Expected: no errors. If any new lint findings touch only the new files, fix in-place and re-run. Do not touch unrelated lint findings.

- [ ] **Step 6: Verify the specscore feature lint stays clean**

Run: `$(go env GOPATH)/bin/specscore spec lint 2>&1 | grep "cli/describe"`
Expected: no output (lint clean on the new feature).

- [ ] **Step 7: Clean up the smoke fixture**

```bash
rm -rf /tmp/sm /tmp/ingitdb-describe-smoke
```

- [ ] **Step 8: Final commit (only if any files changed in this task; expected: none)**

If `main.go` needed restoration:

```bash
git add cmd/ingitdb/main.go
git commit -m "fix(cli): restore Describe registration in main.go

specscore: feature/cli/describe"
```

Otherwise skip.

---

## Spec coverage check

| AC | Pinned by |
|----|-----------|
| describes-collection-yaml | TestDescribeCollection_LocalYAML_Shape (Task 5) |
| table-alias-equivalent | TestDescribeCollection_TableAliasEquivalent (Task 5) |
| desc-alias-equivalent | Smoke test in Task 9; surface assertion in TestDescribe_ReturnsCommand (Task 4) |
| describes-view-with-meta | TestDescribeView_BasicShape (Task 6) |
| ambiguous-view-requires-in | TestDescribeView_AmbiguousRequiresIn (Task 6) |
| in-flag-collection-missing | TestDescribeView_InCollectionMissing (Task 6) |
| view-resolved-by-in-flag | TestDescribeView_ResolvedByIn (Task 6) |
| bare-name-collection | TestDescribeBareName_ResolvesToCollection (Task 7) |
| bare-name-ambiguous | TestDescribeBareName_AmbiguousErrors (Task 7) |
| format-yaml-and-json-equivalent-shape | TestDescribeCollection_JSONFormat (Task 5) |
| format-native-resolves-to-yaml-on-ingitdb | TestDescribeCollection_NativeResolvesToYAML (Task 5) |
| format-sql-errors-on-ingitdb | TestDescribeCollection_SQLFormatErrors (Task 5) |
| unknown-format-rejected | TestResolveFormat unknown_value subtest (Task 2) + TestDescribeCollection_UnknownFormatRejected (Task 8) |
| collection-not-found-error | TestDescribeCollection_NotFound (Task 5) |
| view-not-found-error | TestDescribeView_NotFoundAnywhere (Task 6) |
| data-dir-divergence-surfaces-both-paths | TestBuildCollectionPayload_DataDirDivergence (Task 3) |
| columns-order-respected | TestCollectionDef_MarshalYAML_HonorsColumnsOrder (Task 1) + TestDescribeCollection_ColumnsOrderRespected_CLI (Task 8) |
| columns-order-fallback-alphabetical | TestCollectionDef_MarshalYAML_AlphabeticalFallback (Task 1) + TestDescribeCollection_ColumnsOrderAlphaFallback_CLI (Task 8) |
| source-selection-mutual-exclusion | TestDescribeCollection_MutualExclusion (Task 8) |

Every AC is pinned. No gaps.

---

## Out-of-plan follow-ups

1. **Remote-mode describe.** Mirror `dropCollectionRemote` / `listCollectionsRemoteWithSpec` for both `collection` and `view`. Separate plan because it pulls in the GitHub file-reader + directory listing for the view walk.
2. **Subcollection describe.** Resolve the addressing convention (`users/orders` vs `users.orders`) and wire it through. Affects `drop` and `list` too; coordinate.
3. **MarshalYAML on `ViewDef`?** Not needed for v1 — `ViewDef` has only scalar/list fields, so its current `yaml.Marshal` is already deterministic. Revisit if any `views`-level map fields are added.
