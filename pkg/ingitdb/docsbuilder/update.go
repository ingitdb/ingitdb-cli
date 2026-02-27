package docsbuilder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/gitrepo"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/materializer"
)

type ViewRenderer func(ctx context.Context, col *ingitdb.CollectionDef, view *ingitdb.ViewDef) (string, error)

// UpdateDocs resolves collections by dot-separated glob pattern and updates their README files
func UpdateDocs(ctx context.Context, def *ingitdb.Definition, collectionGlob string, dbPath string, recordsReader ingitdb.RecordsReader) (*ingitdb.MaterializeResult, error) {
	result := &ingitdb.MaterializeResult{}

	targets := ResolveCollections(def.Collections, collectionGlob)
	if len(targets) == 0 {
		return result, nil
	}

	for _, col := range targets {
		changed, err := ProcessCollection(ctx, def, col, dbPath, recordsReader)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Errorf("collection %s: %w", col.ID, err))
			continue
		}
		if changed {
			result.FilesWritten++
		} else {
			result.FilesUnchanged++
		}
	}

	return result, nil
}

func ProcessCollection(ctx context.Context, def *ingitdb.Definition, col *ingitdb.CollectionDef, dbPath string, recordsReader ingitdb.RecordsReader) (bool, error) {
	repoRoot, err := gitrepo.FindRepoRoot(dbPath)
	if err != nil {
		repoRoot = ""
	}

	renderer := func(ctx context.Context, col *ingitdb.CollectionDef, view *ingitdb.ViewDef) (string, error) {
		var buf strings.Builder
		writer := materializer.NewFuncViewWriter(func(content []byte) error {
			buf.Write(content)
			return nil
		})
		builder := materializer.SimpleViewBuilder{
			DefReader:     nil, // Not needed as we pass the view down
			RecordsReader: recordsReader,
			Writer:        writer,
		}
		res, err := builder.BuildView(ctx, dbPath, repoRoot, col, def, view)
		if err != nil {
			return "", err
		}
		if len(res.Errors) > 0 {
			return "", res.Errors[0]
		}
		return buf.String(), nil
	}

	content, err := BuildCollectionReadme(ctx, col, def, renderer)
	if err != nil {
		return false, err
	}

	readmePath := filepath.Join(col.DirPath, "README.md")
	existing, err := os.ReadFile(readmePath)
	if err == nil && string(existing) == content {
		return false, nil // no change
	}

	if err := os.WriteFile(readmePath, []byte(content), 0o644); err != nil {
		return false, err
	}

	return true, nil
}

// ResolveCollections returns a list of collection definitions that match the dot-separated path or glob pattern.
func ResolveCollections(collections map[string]*ingitdb.CollectionDef, pattern string) []*ingitdb.CollectionDef {
	if pattern == "" {
		return nil
	}

	matchAll := pattern == "**"
	if matchAll {
		var all []*ingitdb.CollectionDef
		for _, col := range collections {
			all = append(all, col)
			all = append(all, collectSub(col, true)...)
		}
		return all
	}

	parts := strings.Split(pattern, "/*")
	targetPath := parts[0]
	matchDirectSub := len(parts) > 1 && parts[1] == ""
	matchRecursiveSub := len(parts) > 1 && parts[1] == "*" // "/**" split by "/*" -> ["", "*"]

	pathSegments := strings.Split(targetPath, ".")

	var findCol func(curr map[string]*ingitdb.CollectionDef, parts []string) *ingitdb.CollectionDef
	findCol = func(curr map[string]*ingitdb.CollectionDef, parts []string) *ingitdb.CollectionDef {
		col, ok := curr[parts[0]]
		if !ok {
			return nil
		}
		if len(parts) == 1 {
			return col
		}
		return findCol(col.SubCollections, parts[1:])
	}

	targetCol := findCol(collections, pathSegments)
	if targetCol == nil {
		return nil
	}

	var results []*ingitdb.CollectionDef
	if matchRecursiveSub {
		results = append(results, targetCol)
		results = append(results, collectSub(targetCol, true)...)
	} else if matchDirectSub {
		results = append(results, targetCol)
		results = append(results, collectSub(targetCol, false)...)
	} else {
		results = append(results, targetCol)
	}

	return results
}

func collectSub(col *ingitdb.CollectionDef, recursive bool) []*ingitdb.CollectionDef {
	var subs []*ingitdb.CollectionDef
	for _, sub := range col.SubCollections {
		subs = append(subs, sub)
		if recursive {
			subs = append(subs, collectSub(sub, true)...)
		}
	}
	return subs
}

// FindCollectionByDir recursively searches for a collection by its directory path
func FindCollectionByDir(collections map[string]*ingitdb.CollectionDef, dir string) *ingitdb.CollectionDef {
	for _, col := range collections {
		if col.DirPath == dir {
			return col
		}
		if found := FindCollectionByDir(col.SubCollections, dir); found != nil {
			return found
		}
	}
	return nil
}

// FindCollectionsForConflictingFiles takes a list of conflicted files and a map of allowed resolutions.
// It returns a list of matched collections and a list of unresolved file paths.
func FindCollectionsForConflictingFiles(def *ingitdb.Definition, wd string, conflictedFiles []string, resolveItems map[string]bool) (collectionsToUpdate []*ingitdb.CollectionDef, readmesToUpdate []string, unresolved []string) {
	for _, f := range conflictedFiles {
		if f == "" {
			continue
		}
		base := strings.ToLower(filepath.Base(f))
		if base == "readme.md" && resolveItems["readme"] {
			readmesToUpdate = append(readmesToUpdate, f)
		} else {
			unresolved = append(unresolved, f)
		}
	}

	for _, readmePath := range readmesToUpdate {
		absPath := filepath.Join(wd, readmePath)
		dir := filepath.Dir(absPath)
		col := FindCollectionByDir(def.Collections, dir)
		if col != nil {
			collectionsToUpdate = append(collectionsToUpdate, col)
		}
	}

	return collectionsToUpdate, readmesToUpdate, unresolved
}
