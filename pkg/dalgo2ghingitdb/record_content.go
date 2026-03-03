package dalgo2ghingitdb

import (
	"path"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func resolveRecordPath(colDef *ingitdb.CollectionDef, recordKey string) string {
	recordName := strings.ReplaceAll(colDef.RecordFile.Name, "{key}", recordKey)
	base := colDef.RecordFile.RecordsBasePath()
	recordPath := path.Join(colDef.DirPath, base, recordName)
	return path.Clean(recordPath)
}
