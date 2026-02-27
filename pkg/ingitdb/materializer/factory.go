package materializer

import "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"

// NewViewBuilder wires default view definition reader and file writer.
func NewViewBuilder(recordsReader ingitdb.RecordsReader, logf func(string, ...any)) SimpleViewBuilder {
	return SimpleViewBuilder{
		DefReader:     FileViewDefReader{},
		RecordsReader: recordsReader,
		Writer:        NewFileViewWriter(),
		Logf:          logf,
	}
}
