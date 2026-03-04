package validator

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"gopkg.in/yaml.v3"
)

// definitionReader wraps ReadDefinition to satisfy ingitdb.CollectionsReader.
type definitionReader struct{}

// NewCollectionsReader returns an ingitdb.CollectionsReader backed by ReadDefinition.
func NewCollectionsReader() ingitdb.CollectionsReader { return definitionReader{} }

func (definitionReader) ReadDefinition(dbPath string, opts ...ingitdb.ReadOption) (*ingitdb.Definition, error) {
	return ReadDefinition(dbPath, opts...)
}

// defLoader holds the I/O primitives used when reading collection definitions.
// Struct fields allow test code to inject fakes without changing production behaviour.
type defLoader struct {
	readFile func(string) ([]byte, error)
	readDir  func(string) ([]os.DirEntry, error)
}

// newDefLoader returns a defLoader that delegates directly to the OS.
func newDefLoader() defLoader {
	return defLoader{readFile: os.ReadFile, readDir: os.ReadDir}
}

func ReadDefinition(rootPath string, o ...ingitdb.ReadOption) (def *ingitdb.Definition, err error) {
	opts := ingitdb.NewReadOptions(o...)
	var rootConfig config.RootConfig
	rootConfig, err = config.ReadRootConfigFromFile(rootPath, opts)
	if err != nil {
		err = fmt.Errorf("failed to read root config from %s: %v", config.IngitDBDirName, err)
		return
	}
	dl := newDefLoader()
	def, err = dl.readRootCollections(rootPath, rootConfig, opts)
	if err != nil {
		return nil, err
	}
	def.Subscribers, err = ReadSubscribers(rootPath, opts)
	if err != nil {
		return nil, err
	}
	return def, nil
}

func (dl defLoader) readRootCollections(rootPath string, rootConfig config.RootConfig, o ingitdb.ReadOptions) (def *ingitdb.Definition, err error) {
	def = new(ingitdb.Definition)
	def.Collections = make(map[string]*ingitdb.CollectionDef)
	for id, colPath := range rootConfig.RootCollections {
		if strings.Contains(colPath, "*") {
			err = fmt.Errorf("wildcard root collection paths are not supported, ID=%s, path=%s", id, colPath)
			return
		}
		var colDef *ingitdb.CollectionDef
		if colDef, err = dl.readCollectionDef(rootPath, colPath, "", id, nil, o); err != nil {
			err = fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			return
		}
		def.Collections[id] = colDef
	}
	return
}

func (dl defLoader) readCollectionDef(rootPath, relPath, parentPath, id string, subPath []string, o ingitdb.ReadOptions) (colDef *ingitdb.CollectionDef, err error) {
	colDir := filepath.Join(rootPath, relPath)
	schemaDir := filepath.Join(colDir, ingitdb.SchemaDir)

	var fileContent []byte
	isNewLayout := false

	if len(subPath) > 0 {
		// Old-layout subcollection: navigate via subPath.
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
		colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
		fileContent, err = dl.readFile(colDefFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
		}
	} else {
		// Detect layout by trying both definition file locations.
		oldDefPath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
		newDefPath := filepath.Join(colDir, ingitdb.CollectionDefFileName)
		oldContent, oldErr := dl.readFile(oldDefPath)
		newContent, newErr := dl.readFile(newDefPath)
		oldExists := oldErr == nil
		newExists := newErr == nil
		switch {
		case oldExists && newExists:
			return nil, fmt.Errorf("collection %q: both %s and %s exist; use only one layout", id, oldDefPath, newDefPath)
		case oldExists:
			fileContent = oldContent
		case newExists:
			fileContent = newContent
			schemaDir = colDir
			isNewLayout = true
		default:
			if !os.IsNotExist(oldErr) {
				return nil, fmt.Errorf("failed to read file %s: %w", oldDefPath, oldErr)
			}
			return nil, fmt.Errorf("failed to read file %s: %w", newDefPath, newErr)
		}
	}

	colDef = new(ingitdb.CollectionDef)
	colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	if err = yaml.Unmarshal(fileContent, colDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id

	var dataBase string
	if isNewLayout {
		dataBase = colDir
		if filepath.Base(filepath.Dir(colDir)) == ingitdb.CollectionsDir {
			dataBase = filepath.Dir(filepath.Dir(colDir))
		}
		if colDef.DataDir != "" {
			colDef.DirPath = filepath.Join(dataBase, colDef.DataDir)
		} else {
			colDef.DirPath = dataBase
		}
	} else if len(subPath) == 0 {
		colDef.DirPath = colDir
	} else {
		colDef.DirPath = schemaDir
	}

	fullPath := id
	if parentPath != "" {
		fullPath = parentPath + "/" + id
	}

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			if len(subPath) > 0 {
				return nil, fmt.Errorf("not valid definition of subcollection '%s': %w", fullPath, err)
			}
			return nil, fmt.Errorf("not valid definition of collection '%s': %w", fullPath, err)
		}
		if len(subPath) > 0 {
			log.Printf("Definition of subcollection '%s' is valid", fullPath)
		} else {
			log.Printf("Definition of collection '%s' is valid", fullPath)
		}
	}

	if isNewLayout {
		if colDef.SubCollections, err = dl.loadSubCollectionsShared(schemaDir, dataBase, fullPath, o); err != nil {
			return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		}
		viewsDir := filepath.Join(schemaDir, ingitdb.SharedViewsDir)
		if colDef.Views, err = dl.loadViews(viewsDir, o); err != nil {
			return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
		}
	} else {
		if colDef.SubCollections, err = dl.loadSubCollections(rootPath, relPath, subPath, fullPath, o); err != nil {
			return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		}
		viewsDir := filepath.Join(schemaDir, "views")
		if colDef.Views, err = dl.loadViews(viewsDir, o); err != nil {
			return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
		}
	}

	if colDef.DefaultView != nil {
		colDef.DefaultView.ID = ingitdb.DefaultViewID
		colDef.DefaultView.IsDefault = true
		if colDef.Views == nil {
			colDef.Views = make(map[string]*ingitdb.ViewDef)
		}
		colDef.Views[ingitdb.DefaultViewID] = colDef.DefaultView
	}

	return
}

// readCollectionDefShared reads a single collection from the new shared-directory
// layout. schemaDir is the absolute path to the .collections/{name}/ directory.
// dataBaseDir is the parent of .collections/ (the shared data root).
func (dl defLoader) readCollectionDefShared(schemaDir, dataBaseDir, parentPath, id string, o ingitdb.ReadOptions) (*ingitdb.CollectionDef, error) {
	colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	fileContent, err := dl.readFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}

	colDef := new(ingitdb.CollectionDef)
	if err = yaml.Unmarshal(fileContent, colDef); err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id

	if colDef.DataDir != "" {
		colDef.DirPath = filepath.Join(dataBaseDir, colDef.DataDir)
	} else {
		colDef.DirPath = dataBaseDir
	}

	fullPath := id
	if parentPath != "" {
		fullPath = parentPath + "/" + id
	}

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			return nil, fmt.Errorf("not valid definition of collection '%s': %w", fullPath, err)
		}
		log.Printf("Definition of collection '%s' is valid", fullPath)
	}

	colDef.SubCollections, err = dl.loadSubCollectionsShared(schemaDir, dataBaseDir, fullPath, o)
	if err != nil {
		return nil, fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
	}

	viewsDir := filepath.Join(schemaDir, ingitdb.SharedViewsDir)
	colDef.Views, err = dl.loadViews(viewsDir, o)
	if err != nil {
		return nil, fmt.Errorf("failed to load views for '%s': %w", id, err)
	}

	if colDef.DefaultView != nil {
		colDef.DefaultView.ID = ingitdb.DefaultViewID
		colDef.DefaultView.IsDefault = true
		if colDef.Views == nil {
			colDef.Views = make(map[string]*ingitdb.ViewDef)
		}
		colDef.Views[ingitdb.DefaultViewID] = colDef.DefaultView
	}

	return colDef, nil
}

// loadSubCollectionsShared discovers subcollections in the new shared-directory
// layout. Each non-$-prefixed sub-directory of schemaDir that contains a
// definition.yaml is treated as a subcollection.
func (dl defLoader) loadSubCollectionsShared(schemaDir, dataBaseDir, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	entries, err := dl.readDir(schemaDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory: %w", err)
	}

	var subCollections map[string]*ingitdb.CollectionDef
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "$") {
			continue
		}
		subID := entry.Name()
		subSchemaDir := filepath.Join(schemaDir, subID)

		colDef, subErr := dl.readCollectionDefShared(subSchemaDir, dataBaseDir, parentPath, subID, o)
		if subErr != nil {
			if errors.Is(subErr, os.ErrNotExist) {
				continue // no definition.yaml — not a collection, skip
			}
			return nil, subErr
		}

		if subCollections == nil {
			subCollections = make(map[string]*ingitdb.CollectionDef)
		}
		subCollections[subID] = colDef
	}
	return subCollections, nil
}

func (dl defLoader) loadSubCollections(rootPath, relPath string, subPath []string, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	schemaDir := filepath.Join(rootPath, relPath, ingitdb.SchemaDir)
	if len(subPath) > 0 {
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
	}
	subCollectionsPath := filepath.Join(schemaDir, "subcollections")

	entries, err := dl.readDir(subCollectionsPath)
	if os.IsNotExist(err) {
		return nil, nil // No subcollections
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read subcollections directory: %w", err)
	}

	var subCollections map[string]*ingitdb.CollectionDef

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		id := entry.Name()
		childSubPath := append(append([]string(nil), subPath...), id)

		colDef, err := dl.readCollectionDef(rootPath, relPath, parentPath, id, childSubPath, o)
		if err != nil {
			return nil, err
		}

		if subCollections == nil {
			subCollections = make(map[string]*ingitdb.CollectionDef)
		}
		subCollections[id] = colDef
	}
	return subCollections, nil
}

func (dl defLoader) loadViews(viewsDir string, o ingitdb.ReadOptions) (map[string]*ingitdb.ViewDef, error) {
	entries, err := dl.readDir(viewsDir)
	if os.IsNotExist(err) {
		return nil, nil // No views
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read views directory: %w", err)
	}

	var views map[string]*ingitdb.ViewDef

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".yaml")
		viewDefFilePath := filepath.Join(viewsDir, entry.Name())

		fileContent, err := dl.readFile(viewDefFilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", viewDefFilePath, err)
		}

		viewDef := new(ingitdb.ViewDef)
		if err = yaml.Unmarshal(fileContent, viewDef); err != nil {
			return nil, fmt.Errorf("failed to parse YAML file %s: %w", viewDefFilePath, err)
		}
		viewDef.ID = id

		if o.IsValidationRequired() {
			if err = viewDef.Validate(); err != nil {
				return nil, fmt.Errorf("not valid definition of view '%s': %w", id, err)
			}
			log.Printf("Definition of view '%s' is valid", id)
		}

		if views == nil {
			views = make(map[string]*ingitdb.ViewDef)
		}
		views[id] = viewDef
	}
	return views, nil
}
