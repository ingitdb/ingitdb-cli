package dalgo2ingitdb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/ddl"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/validator"
)

func newReader() ingitdb.CollectionsReader { return validator.NewCollectionsReader() }

func TestNewDatabase_OpensExistingPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	db, err := NewDatabase(dir, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if db == nil {
		t.Fatal("NewDatabase returned nil db")
	}
	if _, ok := db.(dal.ConcurrencyAware); !ok {
		t.Error("db should satisfy dal.ConcurrencyAware")
	}
	if _, ok := db.(ddl.TransactionalDDL); !ok {
		t.Error("db should satisfy ddl.TransactionalDDL")
	}
}

func TestNewDatabase_RejectsEmptyPath(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase("", newReader())
	if err == nil {
		t.Fatal("NewDatabase(\"\"): want error, got nil")
	}
	if db != nil {
		t.Error("NewDatabase(\"\"): want nil db on error")
	}
}

func TestNewDatabase_RejectsMissingPath(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	db, err := NewDatabase(missing, newReader())
	if err == nil {
		t.Fatal("NewDatabase(missing): want error, got nil")
	}
	if db != nil {
		t.Error("NewDatabase(missing): want nil db on error")
	}
}

func TestNewDatabase_RejectsNonDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	file := filepath.Join(dir, "a-file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	db, err := NewDatabase(file, newReader())
	if err == nil {
		t.Fatal("NewDatabase(file): want error, got nil")
	}
	if db != nil {
		t.Error("NewDatabase(file): want nil db on error")
	}
}

func TestDatabase_SupportsConcurrentConnections(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if !db.SupportsConcurrentConnections() {
		t.Error("SupportsConcurrentConnections: want true (file locking is implemented)")
	}
}

func TestDatabase_SupportsTransactionalDDL(t *testing.T) {
	t.Parallel()
	db, err := NewDatabase(t.TempDir(), newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	if ddl.SupportsTransactionalDDL(db) {
		t.Error("SupportsTransactionalDDL: want false (MVP is non-transactional)")
	}
}
