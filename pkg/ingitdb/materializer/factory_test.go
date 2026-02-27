package materializer

import (
	"context"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

type noopRecordsReader struct{}

func (noopRecordsReader) ReadRecords(
	ctx context.Context,
	dbPath string,
	col *ingitdb.CollectionDef,
	yield func(ingitdb.RecordEntry) error,
) error {
	return nil
}

func TestNewViewBuilder_WiresDefaults(t *testing.T) {
	t.Parallel()

	builder := NewViewBuilder(noopRecordsReader{}, nil)
	if builder.DefReader == nil {
		t.Fatalf("expected default view def reader")
	}
	if builder.RecordsReader == nil {
		t.Fatalf("expected records reader to be set")
	}
	if builder.Writer == nil {
		t.Fatalf("expected default view writer")
	}
	if _, ok := builder.Writer.(FileViewWriter); !ok {
		t.Fatalf("expected FileViewWriter, got %T", builder.Writer)
	}
}
