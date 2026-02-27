package commands

import (
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

func viewBuilderForCollection(colDef *ingitdb.CollectionDef) (materializer.ViewBuilder, error) {
	if colDef == nil {
		return nil, nil
	}
	reader := materializer.FileViewDefReader{}
	views, err := reader.ReadViewDefs(colDef.DirPath)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, nil
	}
	// Use the filesystem reader for template-based views like README builders.
	return materializer.NewViewBuilder(materializer.NewFileRecordsReader(), nil), nil
}
