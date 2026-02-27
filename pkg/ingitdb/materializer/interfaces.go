package materializer

import (
	"context"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/datavalidator"
)

// ViewDefReader discovers and parses .ingitdb-view.*.yaml files.
type ViewDefReader interface {
	ReadViewDefs(colDirPath string) (map[string]*ingitdb.ViewDef, error)
}

// ViewRenderer transforms records into rendered bytes per output format.
// Pure function; goroutine-safe.
type ViewRenderer interface {
	RenderView(
		ctx context.Context,
		col *ingitdb.CollectionDef,
		view *ingitdb.ViewDef,
		reader ingitdb.RecordsReader,
		dbPath string,
	) (map[string][]byte, error) // key = format string ("md", "json")
}

// ViewWriter renders a view and writes content to the file system.
// Separate from ViewBuilder so tests can capture output without I/O.
type ViewWriter interface {
	WriteView(
		ctx context.Context,
		col *ingitdb.CollectionDef,
		view *ingitdb.ViewDef,
		records []ingitdb.RecordEntry,
		outPath string,
	) (bool, error)
}

// ViewBuilder orchestrates full materialisation of all views for one collection.
type ViewBuilder interface {
	BuildViews(
		ctx context.Context,
		dbPath string,
		repoRoot string,
		col *ingitdb.CollectionDef,
		def *ingitdb.Definition,
	) (*ingitdb.MaterializeResult, error)
}

// ViewAffectedChecker determines whether a view needs rebuilding given changed records.
type ViewAffectedChecker interface {
	IsAffected(col *ingitdb.CollectionDef, view *ingitdb.ViewDef, changed []datavalidator.AffectedRecord) bool
}

// IncrementalMaterializer rebuilds only views affected by specific changed records.
type IncrementalMaterializer interface {
	UpdateViews(
		ctx context.Context,
		dbPath string,
		def *ingitdb.Definition,
		affected []datavalidator.AffectedRecord,
	) (*ingitdb.MaterializeResult, error)
}
