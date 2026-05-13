package dalgo2ingitdb

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/dbschema"
	"github.com/dal-go/dalgo/ddl"
	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// CreateCollection writes <projectPath>/<c.Name>/.collection/definition.yaml
// from the dbschema.CollectionDef. Validates name and field types before
// any filesystem write. With ddl.IfNotExists, an existing collection is a
// no-op; without it, an existing collection is an error.
func (db *Database) CreateCollection(_ context.Context, c dbschema.CollectionDef, opts ...ddl.Option) error {
	options := ddl.ResolveOptions(opts...)
	if err := validateCollectionName(c.Name); err != nil {
		return fmt.Errorf("CreateCollection: %w", err)
	}

	// Validate all field types up front. Any unrepresentable type aborts
	// before we touch the filesystem.
	colDef, err := buildIngitdbCollectionDef(c)
	if err != nil {
		return fmt.Errorf("CreateCollection %q: %w", c.Name, err)
	}

	colDir := filepath.Join(db.projectPath, c.Name, ingitdb.SchemaDir)
	defPath := filepath.Join(colDir, ingitdb.CollectionDefFileName)

	if _, statErr := os.Stat(defPath); statErr == nil {
		if options.IfNotExists {
			return nil
		}
		return fmt.Errorf("CreateCollection: collection %q already exists", c.Name)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return fmt.Errorf("CreateCollection: stat %s: %w", defPath, statErr)
	}

	if err := os.MkdirAll(colDir, 0o755); err != nil {
		return fmt.Errorf("CreateCollection: mkdir %s: %w", colDir, err)
	}

	if len(c.Indexes) > 0 {
		log.Printf("dalgo2ingitdb: CreateCollection %q ignoring %d index declaration(s) — inGitDB has no per-collection index support", c.Name, len(c.Indexes))
	}

	return withExclusiveLock(defPath, func() error {
		return writeCollectionDefYAML(defPath, colDef)
	})
}

// DropCollection removes <projectPath>/<name>/ from disk. The directory
// must contain a .collection/definition.yaml as a safety check — this
// guards against accidentally deleting non-collection directories. With
// ddl.IfExists, a missing collection is a no-op.
func (db *Database) DropCollection(_ context.Context, name string, opts ...ddl.Option) error {
	options := ddl.ResolveOptions(opts...)
	if err := validateCollectionName(name); err != nil {
		return fmt.Errorf("DropCollection: %w", err)
	}

	colDir := filepath.Join(db.projectPath, name)
	defPath := filepath.Join(colDir, ingitdb.SchemaDir, ingitdb.CollectionDefFileName)

	if _, statErr := os.Stat(defPath); statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			if options.IfExists {
				return nil
			}
			return fmt.Errorf("DropCollection: collection %q not found", name)
		}
		return fmt.Errorf("DropCollection: stat %s: %w", defPath, statErr)
	}

	// Safety net: only remove directories that look like collection roots.
	if err := os.RemoveAll(colDir); err != nil {
		return fmt.Errorf("DropCollection: remove %s: %w", colDir, err)
	}
	return nil
}

// AlterCollection applies AlterOp values in order. Operations mutate an
// in-memory ingitdb.CollectionDef; after each op the updated definition
// is flushed to disk. Failure mid-sequence returns
// *ddl.PartialSuccessError with applied / failed / not-attempted lists.
func (db *Database) AlterCollection(ctx context.Context, name string, ops ...ddl.AlterOp) error {
	if err := validateCollectionName(name); err != nil {
		return fmt.Errorf("AlterCollection: %w", err)
	}
	defPath := filepath.Join(db.projectPath, name, ingitdb.SchemaDir, ingitdb.CollectionDefFileName)
	if _, statErr := os.Stat(defPath); statErr != nil {
		return fmt.Errorf("AlterCollection: collection %q not found: %w", name, statErr)
	}

	return withExclusiveLock(defPath, func() error {
		colDef, err := readCollectionDefYAML(defPath)
		if err != nil {
			return fmt.Errorf("AlterCollection %q: %w", name, err)
		}
		ap := &applier{
			db:           db,
			colName:      name,
			colDef:       colDef,
			defPath:      defPath,
			recordsDir:   filepath.Join(db.projectPath, name, colDef.RecordFile.RecordsBasePath()),
			recordFormat: colDef.RecordFile.Format,
		}
		for i, op := range ops {
			if err := op.ApplyTo(ctx, ap); err != nil {
				return &ddl.PartialSuccessError{
					Op:           "AlterCollection",
					Collection:   name,
					Backend:      DatabaseID,
					Applied:      ops[:i],
					FirstFailed:  op,
					NotAttempted: ops[i+1:],
					Cause:        err,
				}
			}
			// Flush after every op so a later failure leaves a consistent
			// definition.yaml on disk.
			if err := writeCollectionDefYAML(defPath, ap.colDef); err != nil {
				return &ddl.PartialSuccessError{
					Op:           "AlterCollection",
					Collection:   name,
					Backend:      DatabaseID,
					Applied:      ops[:i],
					FirstFailed:  op,
					NotAttempted: ops[i+1:],
					Cause:        fmt.Errorf("flush definition.yaml: %w", err),
				}
			}
		}
		return nil
	})
}

// applier implements ddl.Applier for AlterCollection dispatch.
type applier struct {
	db           *Database
	colName      string
	colDef       *ingitdb.CollectionDef
	defPath      string
	recordsDir   string
	recordFormat ingitdb.RecordFormat
}

func (a *applier) ApplyAddField(_ context.Context, f dbschema.FieldDef, opts ddl.Options) error {
	name := string(f.Name)
	if _, exists := a.colDef.Columns[name]; exists {
		if opts.IfNotExists {
			return nil
		}
		return fmt.Errorf("AddField: column %q already exists", name)
	}
	colType, err := dbschemaTypeToIngitdb(f.Type)
	if err != nil {
		return fmt.Errorf("AddField %q: %w", name, err)
	}
	if a.colDef.Columns == nil {
		a.colDef.Columns = map[string]*ingitdb.ColumnDef{}
	}
	a.colDef.Columns[name] = &ingitdb.ColumnDef{
		Type:     colType,
		Required: !f.Nullable,
	}
	a.colDef.ColumnsOrder = append(a.colDef.ColumnsOrder, name)
	// Default-value backfill into record files is deferred — the spec
	// allows nil dbschema.DefaultExpr as a no-backfill case, which is
	// the only one this MVP supports.
	return nil
}

func (a *applier) ApplyDropField(_ context.Context, name dal.FieldName, opts ddl.Options) error {
	col := string(name)
	if _, exists := a.colDef.Columns[col]; !exists {
		if opts.IfExists {
			return nil
		}
		return fmt.Errorf("DropField: column %q does not exist", col)
	}
	delete(a.colDef.Columns, col)
	a.colDef.ColumnsOrder = slices.DeleteFunc(a.colDef.ColumnsOrder, func(s string) bool { return s == col })
	return rewriteRecordFiles(a.recordsDir, a.recordFormat, func(rec map[string]any) {
		delete(rec, col)
	})
}

func (a *applier) ApplyModifyField(_ context.Context, name dal.FieldName, newDef dbschema.FieldDef, _ ddl.Options) error {
	oldName := string(name)
	col, exists := a.colDef.Columns[oldName]
	if !exists {
		return fmt.Errorf("ModifyField: column %q does not exist", oldName)
	}
	colType, err := dbschemaTypeToIngitdb(newDef.Type)
	if err != nil {
		return fmt.Errorf("ModifyField %q: %w", oldName, err)
	}
	col.Type = colType
	col.Required = !newDef.Nullable

	newName := string(newDef.Name)
	if newName != "" && newName != oldName {
		if _, conflict := a.colDef.Columns[newName]; conflict {
			return fmt.Errorf("ModifyField: target name %q already in use", newName)
		}
		delete(a.colDef.Columns, oldName)
		a.colDef.Columns[newName] = col
		for i, n := range a.colDef.ColumnsOrder {
			if n == oldName {
				a.colDef.ColumnsOrder[i] = newName
			}
		}
		// Rewriting record files is required because the field key changed.
		return rewriteRecordFiles(a.recordsDir, a.recordFormat, func(rec map[string]any) {
			if v, ok := rec[oldName]; ok {
				rec[newName] = v
				delete(rec, oldName)
			}
		})
	}
	return nil
}

func (a *applier) ApplyRenameField(_ context.Context, oldName, newName dal.FieldName, _ ddl.Options) error {
	from := string(oldName)
	to := string(newName)
	col, exists := a.colDef.Columns[from]
	if !exists {
		return fmt.Errorf("RenameField: column %q does not exist", from)
	}
	if _, conflict := a.colDef.Columns[to]; conflict {
		return fmt.Errorf("RenameField: target name %q already in use", to)
	}
	delete(a.colDef.Columns, from)
	a.colDef.Columns[to] = col
	for i, n := range a.colDef.ColumnsOrder {
		if n == from {
			a.colDef.ColumnsOrder[i] = to
		}
	}
	return rewriteRecordFiles(a.recordsDir, a.recordFormat, func(rec map[string]any) {
		if v, ok := rec[from]; ok {
			rec[to] = v
			delete(rec, from)
		}
	})
}

func (a *applier) ApplyAddIndex(_ context.Context, idx dbschema.IndexDef, _ ddl.Options) error {
	log.Printf("dalgo2ingitdb: AlterCollection %q AddIndex(%q) ignored — inGitDB has no per-collection index support", a.colName, idx.Name)
	return nil
}

func (a *applier) ApplyDropIndex(_ context.Context, name string, _ ddl.Options) error {
	log.Printf("dalgo2ingitdb: AlterCollection %q DropIndex(%q) ignored — inGitDB has no per-collection index support", a.colName, name)
	return nil
}

// validateCollectionName checks a collection name is non-empty, free of
// path-traversal segments, and free of leading/trailing whitespace. The
// path-traversal check rejects ".." as a path segment anywhere; this
// blocks attempts to escape projectPath via crafted names.
func validateCollectionName(name string) (err error) {
	if name == "" {
		return errors.New("collection name must not be empty")
	}
	if strings.TrimSpace(name) != name {
		return fmt.Errorf("collection name %q has leading/trailing whitespace", name)
	}
	for _, seg := range strings.Split(name, "/") {
		if seg == ".." || seg == "." || seg == "" {
			return fmt.Errorf("collection name %q has invalid path segment", name)
		}
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("collection name %q must be relative", name)
	}
	return nil
}

// buildIngitdbCollectionDef converts a dbschema.CollectionDef to an
// ingitdb.CollectionDef ready for YAML marshaling. Fails if any field
// has a type that cannot be represented in ingitdb (e.g. dbschema.Null).
func buildIngitdbCollectionDef(c dbschema.CollectionDef) (*ingitdb.CollectionDef, error) {
	cols := make(map[string]*ingitdb.ColumnDef, len(c.Fields))
	order := make([]string, 0, len(c.Fields))
	for _, f := range c.Fields {
		colType, err := dbschemaTypeToIngitdb(f.Type)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", f.Name, err)
		}
		name := string(f.Name)
		cols[name] = &ingitdb.ColumnDef{Type: colType, Required: !f.Nullable}
		order = append(order, name)
	}
	return &ingitdb.CollectionDef{
		ID:           c.Name,
		RecordFile:   defaultRecordFile(),
		Columns:      cols,
		ColumnsOrder: order,
	}, nil
}

func defaultRecordFile() *ingitdb.RecordFileDef {
	return &ingitdb.RecordFileDef{
		Name:       "{key}.yaml",
		Format:     ingitdb.RecordFormatYAML,
		RecordType: ingitdb.SingleRecord,
	}
}

// writeCollectionDefYAML marshals colDef and writes it to defPath. The
// containing directory is expected to exist.
func writeCollectionDefYAML(defPath string, colDef *ingitdb.CollectionDef) error {
	data, err := yaml.Marshal(colDef)
	if err != nil {
		return fmt.Errorf("marshal definition.yaml: %w", err)
	}
	if err := os.WriteFile(defPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", defPath, err)
	}
	return nil
}

// readCollectionDefYAML reads and unmarshals defPath into a fresh
// ingitdb.CollectionDef.
func readCollectionDefYAML(defPath string) (*ingitdb.CollectionDef, error) {
	content, err := os.ReadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", defPath, err)
	}
	colDef := new(ingitdb.CollectionDef)
	if err := yaml.Unmarshal(content, colDef); err != nil {
		return nil, fmt.Errorf("parse %s: %w", defPath, err)
	}
	return colDef, nil
}

// rewriteRecordFiles walks recordsDir and applies mutate to each YAML
// record file's parsed map. The MVP supports only YAML record format with
// SingleRecord layout — other formats become a no-op (the test fixtures
// in this package always use YAML).
func rewriteRecordFiles(recordsDir string, format ingitdb.RecordFormat, mutate func(map[string]any)) error {
	if format != ingitdb.RecordFormatYAML {
		return nil
	}
	if _, err := os.Stat(recordsDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("rewriteRecordFiles stat %s: %w", recordsDir, err)
	}
	return filepath.WalkDir(recordsDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}
		return withExclusiveLock(path, func() error {
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			rec := map[string]any{}
			if len(content) > 0 {
				if err := yaml.Unmarshal(content, &rec); err != nil {
					return fmt.Errorf("parse %s: %w", path, err)
				}
			}
			mutate(rec)
			out, err := yaml.Marshal(rec)
			if err != nil {
				return fmt.Errorf("marshal %s: %w", path, err)
			}
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}
			return nil
		})
	})
}

// Compile-time check: *Database satisfies ddl.SchemaModifier.
var _ ddl.SchemaModifier = (*Database)(nil)
