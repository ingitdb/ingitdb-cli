package commands

// specscore: feature/subcollection-addressing

import (
	"strings"

	"github.com/dal-go/dalgo/dal"
	dalgo2fsingitdb "github.com/ingitdb/dalgo2ingitdb4local"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// resolveCollectionGroup resolves a `select --group --from` value, a schema
// path of collection ids only (`spaces/ext/days`, the full path `validate`
// prints), to one source per instance of that collection, and the
// collection's definition. Instances are found by the storage driver
// (dalgo2fsingitdb.SubCollectionParents), which owns the on-disk layout, so
// this obeys REQ:driver-owned-layout. A value that names no declared
// collection fails with the existing "not found in definition" error naming
// the whole value.
func resolveCollectionGroup(def *ingitdb.Definition, from string) ([]selectSource, *ingitdb.CollectionDef, error) {
	// The driver rejects an undeclared, empty, `.`/`..` or separator-bearing
	// segment with `collection "<from>" not found in definition`.
	parents, err := dalgo2fsingitdb.SubCollectionParents(def, from)
	if err != nil {
		return nil, nil, err
	}
	segments := strings.Split(from, "/")
	colDef := def.Collections[segments[0]]
	for _, subID := range segments[1:] {
		colDef = colDef.SubCollections[subID]
	}
	leaf := segments[len(segments)-1]
	sources := make([]selectSource, 0, len(parents))
	for _, parent := range parents {
		if parent == nil { // a root collection is its only instance
			sources = append(sources, selectSource{ref: dal.NewRootCollectionRef(leaf, ""), idPrefix: leaf + "/"})
			continue
		}
		sources = append(sources, selectSource{
			ref:      dal.NewCollectionRef(leaf, "", parent),
			idPrefix: parent.String() + "/" + leaf + "/",
		})
	}
	return sources, colDef, nil
}
