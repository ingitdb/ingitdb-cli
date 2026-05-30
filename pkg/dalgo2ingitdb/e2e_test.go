package dalgo2ingitdb

import (
	"errors"
	"testing"

	e2e "github.com/dal-go/dalgo-end2end-tests"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// TestDalgoEndToEnd runs the shared dalgo-end2end-tests suite against this
// driver. It sets up the two collections the suite uses (DalgoE2E_E2ETest1/2)
// as single-record JSON collections, then delegates to end2end.TestDalgoDB.
//
// Not parallel: the suite manages its own subtests and shared temp state.
func TestDalgoEndToEnd(t *testing.T) {
	root := t.TempDir()
	const def = `record_file:
  name: "{key}.json"
  format: json
  type: "map[string]any"
`
	collections := map[string]string{}
	for _, name := range []string{e2e.E2ETestKind1, e2e.E2ETestKind2} {
		writeCollectionDef(t, root, name, def)
		collections[name] = name
	}
	if err := config.WriteRootCollectionsToFile(root, collections); err != nil {
		t.Fatalf("WriteRootCollectionsToFile: %v", err)
	}

	db, err := NewDatabase(root, newReader())
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}

	// The query suite is skipped for now: it uses a DalgoTest_Cities collection
	// and incomplete-key (auto-generated ID) inserts that this driver does not
	// yet support. Single/multi CRUD run fully. eventuallyConsistent=false because
	// the filesystem is immediately consistent.
	errQuerySupport := errors.New("dalgo2ingitdb: query e2e pending (cities collection + auto-ID inserts)")
	e2e.TestDalgoDB(t, db, errQuerySupport, false)
}
