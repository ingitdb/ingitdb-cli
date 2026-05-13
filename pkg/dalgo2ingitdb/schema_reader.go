package dalgo2ingitdb

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// pkFieldName is the synthesized primary-key field name used to describe
// the inGitDB record-key convention. inGitDB does not declare PKs in
// definition.yaml; each record's filesystem key is the de-facto PK.
const pkFieldName dal.FieldName = "$key"

// reservedSubDirs are directory names that MUST NOT be traversed as
// candidate collection roots during ListCollections — they hold
// auxiliary state (schema, records, subcollections, views) and never
// contain a sibling .collection/definition.yaml of their own.
var reservedSubDirs = map[string]bool{
	ingitdb.SchemaDir:      true, // .collection
	"$records":             true,
	"subcollections":       true,
	"views":                true,
	ingitdb.SharedViewsDir: true, // $views
}

// ListCollections walks the project directory looking for directories
// that contain a .collection/definition.yaml file. The parent argument
// is ignored (inGitDB has no catalog hierarchy). Results are sorted
// alphabetically by name; names use "/" as the separator for nested
// collection paths relative to projectPath.
func (db *Database) ListCollections(_ context.Context, _ *dal.Key) ([]dal.CollectionRef, error) {
	var refs []dal.CollectionRef
	walkErr := filepath.WalkDir(db.projectPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() {
			return nil
		}
		// Skip reserved sub-directory names anywhere in the tree.
		base := d.Name()
		if path != db.projectPath && reservedSubDirs[base] {
			return fs.SkipDir
		}
		// Check whether this directory is a collection root.
		defPath := filepath.Join(path, ingitdb.SchemaDir, ingitdb.CollectionDefFileName)
		info, statErr := os.Stat(defPath)
		if statErr != nil || info.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(db.projectPath, path)
		if relErr != nil {
			return fmt.Errorf("relative path for %s: %w", path, relErr)
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		refs = append(refs, dal.NewRootCollectionRef(name, ""))
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("walk %s: %w", db.projectPath, walkErr)
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Name() < refs[j].Name() })
	return refs, nil
}

// DescribeCollection reads and parses the collection's definition.yaml
// under a shared lock, then maps the ingitdb columns to dbschema fields
// via type_mapping. PrimaryKey is synthesized as [pkFieldName] because
// inGitDB uses the record's filesystem key as the de-facto PK.
func (db *Database) DescribeCollection(_ context.Context, ref *dal.CollectionRef) (*dbschema.CollectionDef, error) {
	if ref == nil {
		return nil, fmt.Errorf("dalgo2ingitdb: DescribeCollection: ref is nil")
	}
	name := ref.Name()
	defPath := filepath.Join(db.projectPath, name, ingitdb.SchemaDir, ingitdb.CollectionDefFileName)

	if _, err := os.Stat(defPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("dalgo2ingitdb: collection %q not found: %w", name, err)
		}
		return nil, fmt.Errorf("dalgo2ingitdb: stat definition.yaml for %q: %w", name, err)
	}

	var colDef ingitdb.CollectionDef
	readErr := withSharedLock(defPath, func() error {
		content, err := os.ReadFile(defPath)
		if err != nil {
			return fmt.Errorf("read definition.yaml: %w", err)
		}
		if err := yaml.Unmarshal(content, &colDef); err != nil {
			return fmt.Errorf("parse definition.yaml: %w", err)
		}
		return nil
	})
	if readErr != nil {
		return nil, fmt.Errorf("dalgo2ingitdb: describe %q: %w", name, readErr)
	}

	fieldOrder := colDef.ColumnsOrder
	if len(fieldOrder) == 0 {
		fieldOrder = make([]string, 0, len(colDef.Columns))
		for colName := range colDef.Columns {
			fieldOrder = append(fieldOrder, colName)
		}
		sort.Strings(fieldOrder)
	}

	fields := make([]dbschema.FieldDef, 0, len(fieldOrder))
	for _, colName := range fieldOrder {
		if colName == string(pkFieldName) {
			// Never expose the synthesized PK as a regular field.
			continue
		}
		col, ok := colDef.Columns[colName]
		if !ok {
			return nil, fmt.Errorf("dalgo2ingitdb: describe %q: columns_order references unknown column %q", name, colName)
		}
		t, err := ingitdbTypeToDBSchema(col.Type)
		if err != nil {
			return nil, fmt.Errorf("dalgo2ingitdb: describe %q field %q: %w", name, colName, err)
		}
		fields = append(fields, dbschema.FieldDef{
			Name:     dal.FieldName(colName),
			Type:     t,
			Nullable: !col.Required,
		})
	}

	return &dbschema.CollectionDef{
		Name:       name,
		Fields:     fields,
		PrimaryKey: []dal.FieldName{pkFieldName},
		Indexes:    []dbschema.IndexDef{},
	}, nil
}

// ListIndexes returns a non-nil empty slice and nil error. inGitDB has
// no per-collection index declarations today.
func (db *Database) ListIndexes(_ context.Context, _ *dal.CollectionRef) ([]dbschema.IndexDef, error) {
	return []dbschema.IndexDef{}, nil
}

// ListConstraints returns a synthesized single-element slice describing
// the record-key PK as the only structural constraint. ingitdb does not
// store other constraint kinds in definition.yaml.
func (db *Database) ListConstraints(_ context.Context, _ *dal.CollectionRef) ([]dbschema.ConstraintDef, error) {
	return []dbschema.ConstraintDef{{Name: "$key-pk", Type: "primary-key"}}, nil
}

// ListReferrers returns *dbschema.NotSupportedError — inGitDB has no
// structural foreign-key declarations; ColumnDef.ForeignKey is a
// free-text hint, not a navigable reference.
func (db *Database) ListReferrers(_ context.Context, _ *dal.CollectionRef) ([]dbschema.Referrer, error) {
	return nil, &dbschema.NotSupportedError{
		Op:      "ListReferrers",
		Backend: DatabaseID,
		Reason:  "inGitDB has no native foreign-key declarations",
	}
}

// Compile-time check: *Database satisfies dbschema.SchemaReader.
var _ dbschema.SchemaReader = (*Database)(nil)
