package dalgo2ingitdb

// specscore: feature/id-flag-format

import (
	"fmt"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// CollectionForKey finds the collection and record key for a given ID string.
//
// The id format is "{collectionID}/{recordKey}" where collection IDs use "." for namespaces.
// "/" is reserved for separating collection ID from record key path segments.
// The longest matching collection prefix wins.
func CollectionForKey(def *ingitdb.Definition, id string) (*ingitdb.CollectionDef, string, error) {
	// The collection ID is everything before the first "/", which separates it
	// from the record key. Validate that segment up front so a malformed ID
	// (missing separator or an invalid collection charset) yields a clear
	// diagnostic instead of a generic "collection not found".
	collectionSegment, _, found := strings.Cut(id, "/")
	if !found {
		return nil, "", fmt.Errorf("invalid ID %q: must be in the form <collection>/<key>", id)
	}
	if err := ingitdb.ValidateCollectionID(collectionSegment); err != nil {
		return nil, "", fmt.Errorf("invalid collection in ID %q: %w", id, err)
	}

	var bestColDef *ingitdb.CollectionDef
	var bestKey string
	var bestLen int

	for colID, colDef := range def.Collections {
		prefix := colID + "/"
		if len(prefix) <= bestLen+1 {
			continue
		}
		if !strings.HasPrefix(id, prefix) {
			continue
		}
		bestLen = len(prefix) - 1
		bestColDef = colDef
		bestKey = id[len(prefix):]
	}

	if bestColDef == nil {
		return nil, "", fmt.Errorf("collection not found for ID %q", id)
	}
	if bestKey == "" {
		return nil, "", fmt.Errorf("no record key in ID %q", id)
	}
	return bestColDef, bestKey, nil
}
