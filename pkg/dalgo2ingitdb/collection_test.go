package dalgo2ingitdb

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func collectionForKeyDef() *ingitdb.Definition {
	return &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"countries": {
				ID: "countries",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}/{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
			"todo.tags": {
				ID: "todo.tags",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
			"todo.tasks": {
				ID: "todo.tasks",
				RecordFile: &ingitdb.RecordFileDef{
					Name:       "{key}.yaml",
					Format:     "yaml",
					RecordType: ingitdb.SingleRecord,
				},
			},
		},
	}
}

func TestCollectionForKey_SlashSeparatedID(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	colDef, key, err := CollectionForKey(def, "countries/ie")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "countries" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "countries")
	}
	if key != "ie" {
		t.Errorf("key = %q, want %q", key, "ie")
	}
}

func TestCollectionForKey_DotSeparatedNamespacedID(t *testing.T) {
	t.Parallel()

	// "todo.tags/abc" uses "." as namespace separator in the collection part.
	def := collectionForKeyDef()
	colDef, key, err := CollectionForKey(def, "todo.tags/abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "todo.tags" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "todo.tags")
	}
	if key != "abc" {
		t.Errorf("key = %q, want %q", key, "abc")
	}
}

func TestCollectionForKey_SlashNormalizedNamespacedIDRejected(t *testing.T) {
	t.Parallel()

	// "todo/tags/abc" is invalid because "/" is reserved as the collection/key separator.
	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "todo/tags/abc")
	if err == nil {
		t.Fatal("expected error for slash-normalized namespaced collection ID")
	}
}

func TestCollectionForKey_LongestMatchWins(t *testing.T) {
	t.Parallel()

	// When two collections share a prefix (e.g. "todo.tags" vs "todo.tasks"),
	// the correct one is selected.
	def := collectionForKeyDef()

	colDef, key, err := CollectionForKey(def, "todo.tasks/task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if colDef.ID != "todo.tasks" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "todo.tasks")
	}
	if key != "task-1" {
		t.Errorf("key = %q, want %q", key, "task-1")
	}
}

func TestCollectionForKey_CollectionNotFound(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "no/such/collection/key")
	if err == nil {
		t.Fatal("expected error for unknown collection")
	}
}

func TestCollectionForKey_MissingRecordKey(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	// ID ends right after the collection prefix — no key part.
	_, _, err := CollectionForKey(def, "countries/")
	if err == nil {
		t.Fatal("expected error when record key is empty")
	}
}

func TestCollectionForKey_InvalidCharsetRejected(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "bad!name/ie")
	if err == nil {
		t.Fatal("expected error for invalid collection charset")
	}
	if !strings.Contains(err.Error(), "invalid character") {
		t.Errorf("error %q should diagnose the invalid collection character", err)
	}
}

func TestCollectionForKey_MissingSeparatorRejected(t *testing.T) {
	t.Parallel()

	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "countries")
	if err == nil {
		t.Fatal("expected error when the collection/key separator is missing")
	}
	if !strings.Contains(err.Error(), "<collection>/<key>") {
		t.Errorf("error %q should explain the required <collection>/<key> form", err)
	}
}

func TestCollectionForKey_ValidButUndeclaredStillNotFound(t *testing.T) {
	t.Parallel()

	// A syntactically valid collection segment that is not declared must still
	// report "collection not found", not a charset diagnostic.
	def := collectionForKeyDef()
	_, _, err := CollectionForKey(def, "missing/ie")
	if err == nil {
		t.Fatal("expected error for undeclared collection")
	}
	if !strings.Contains(err.Error(), "collection not found") {
		t.Errorf("error %q should report collection not found", err)
	}
}

func TestCollectionForKey_UnderscoreCollectionAllowed(t *testing.T) {
	t.Parallel()

	def := &ingitdb.Definition{
		Collections: map[string]*ingitdb.CollectionDef{
			"exchange_rates": {ID: "exchange_rates"},
		},
	}
	colDef, key, err := CollectionForKey(def, "exchange_rates/usd")
	if err != nil {
		t.Fatalf("unexpected error for underscore collection: %v", err)
	}
	if colDef.ID != "exchange_rates" {
		t.Errorf("colDef.ID = %q, want %q", colDef.ID, "exchange_rates")
	}
	if key != "usd" {
		t.Errorf("key = %q, want %q", key, "usd")
	}
}
