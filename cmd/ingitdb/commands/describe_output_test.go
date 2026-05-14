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
