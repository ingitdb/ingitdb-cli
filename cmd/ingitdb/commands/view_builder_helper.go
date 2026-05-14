package commands

// specscore: feature/cli/materialize

import (
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

func viewBuilderForCollection(colDef *ingitdb.CollectionDef) (materializer.ViewBuilder, error) {
	if colDef == nil {
		return nil, nil
	}
	if len(colDef.Views) == 0 {
		// Fall back to disk reading for callers that haven't gone through
		// ReadDefinition (e.g. GitHub path).
		reader := materializer.FileViewDefReader{}
		views, err := reader.ReadViewDefs(colDef.DirPath)
		if err != nil {
			return nil, err
		}
		if len(views) == 0 {
			return nil, nil
		}
	}
	// Use the filesystem reader for template-based views like README builders.
	return materializer.NewViewBuilder(materializer.NewFileRecordsReader(), nil), nil
}
