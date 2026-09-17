package commands

// specscore: feature/subcollection-addressing

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/record"

	"github.com/ingitdb/ingitdb-go/ingitdb"
)

// resolveFromCollection resolves a `--from` value to the collection reference
// a query reads and the definition of that collection.
//
// A declared root collection ID resolves exactly as before. Any other value
// must be a subcollection path `<collection>/<record-key>/<subcollection>`
// (repeatable to deeper levels) in which every subcollection segment is
// declared by the definition before it and no segment is empty, `.`, `..` or
// contains a path separator; the returned reference carries the
// parent record key so the storage driver scopes the read to that parent's
// records on its own on-disk layout. Every other value fails with the
// existing "not found in definition" error naming the whole value.
func resolveFromCollection(def *ingitdb.Definition, from string) (dal.CollectionRef, *ingitdb.CollectionDef, error) {
	if colDef, ok := def.Collections[from]; ok {
		return dal.NewRootCollectionRef(from, ""), colDef, nil
	}
	notFound := fmt.Errorf("collection %q not found in definition", from)
	segments := strings.Split(from, "/")
	if len(segments) < 3 || len(segments)%2 == 0 {
		return dal.CollectionRef{}, nil, notFound
	}
	for _, s := range segments {
		if !isPlainPathSegment(s) {
			return dal.CollectionRef{}, nil, notFound
		}
	}
	colDef, ok := def.Collections[segments[0]]
	if !ok {
		return dal.CollectionRef{}, nil, notFound
	}
	var parent *record.Key
	collection := segments[0]
	for i := 1; i < len(segments); i += 2 {
		parent = record.NewKeyWithParentAndID(parent, collection, segments[i])
		collection = segments[i+1]
		sub, declared := colDef.SubCollections[collection]
		if !declared {
			return dal.CollectionRef{}, nil, notFound
		}
		colDef = sub
	}
	return dal.NewCollectionRef(collection, "", parent), colDef, nil
}

// isPlainPathSegment reports whether s can name a collection or a record in a
// subcollection path: non-empty, not `.` or `..`, and free of path separators
// (`/`, `\` and the OS separator), so a parent record key can never move the
// driver's scoped directory outside its parent record.
func isPlainPathSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	return !strings.ContainsAny(s, `/\`+string(filepath.Separator))
}
