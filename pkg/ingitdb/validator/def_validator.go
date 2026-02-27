package validator

import (
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

func ReadDefinition(rootPath string, o ...ingitdb.ReadOption) (def *ingitdb.Definition, err error) {
	opts := ingitdb.NewReadOptions(o...)
	var rootConfig config.RootConfig
	rootConfig, err = config.ReadRootConfigFromFile(rootPath, opts)
	if err != nil {
		err = fmt.Errorf("failed to read root config from %s: %v", config.IngitDBDirName, err)
		return
	}
	def, err = readRootCollections(rootPath, rootConfig, opts)
	if err != nil {
		return nil, err
	}
	def.Subscribers, err = ReadSubscribers(rootPath, opts)
	if err != nil {
		return nil, err
	}
	return def, nil
}

func readRootCollections(rootPath string, rootConfig config.RootConfig, o ingitdb.ReadOptions) (def *ingitdb.Definition, err error) {
	def = new(ingitdb.Definition)
	def.Collections = make(map[string]*ingitdb.CollectionDef)
	for id, colPath := range rootConfig.RootCollections {
		if strings.Contains(colPath, "*") {
			err = fmt.Errorf("wildcard root collection paths are not supported, ID=%s, path=%s", id, colPath)
			return
		}
		var colDef *ingitdb.CollectionDef
		if colDef, err = readCollectionDef(rootPath, colPath, "", id, nil, o); err != nil {
			err = fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			return
		}
		def.Collections[id] = colDef
	}
	return
}

func readCollectionDef(rootPath, relPath, parentPath, id string, subPath []string, o ingitdb.ReadOptions) (colDef *ingitdb.CollectionDef, err error) {
	// For root collections, the definition file is in relPath/.collection/definition.yaml
	// For subcollections, it is located recursively inside relPath/.collection/subcollections/...
	colDir := filepath.Join(rootPath, relPath)
	schemaDir := filepath.Join(colDir, ingitdb.SchemaDir)

	if len(subPath) > 0 {
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
	}

	colDefFilePath := filepath.Join(schemaDir, ingitdb.CollectionDefFileName)
	var fileContent []byte
	fileContent, err = os.ReadFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}
	colDef = new(ingitdb.CollectionDef)

	err = yaml.Unmarshal(fileContent, colDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id
	if len(subPath) == 0 {
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

	if colDef.SubCollections, err = loadSubCollections(rootPath, relPath, subPath, fullPath, o); err != nil {
		err = fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		return
	}

	if colDef.Views, err = loadViews(schemaDir, o); err != nil {
		err = fmt.Errorf("failed to load views for '%s': %w", id, err)
		return
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

func loadSubCollections(rootPath, relPath string, subPath []string, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	schemaDir := filepath.Join(rootPath, relPath, ingitdb.SchemaDir)
	if len(subPath) > 0 {
		for _, p := range subPath {
			schemaDir = filepath.Join(schemaDir, "subcollections", p)
		}
	}
	subCollectionsPath := filepath.Join(schemaDir, "subcollections")

	entries, err := os.ReadDir(subCollectionsPath)
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

		colDef, err := readCollectionDef(rootPath, relPath, parentPath, id, childSubPath, o)
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

func loadViews(schemaDir string, o ingitdb.ReadOptions) (map[string]*ingitdb.ViewDef, error) {
	viewsPath := filepath.Join(schemaDir, "views")
	entries, err := os.ReadDir(viewsPath)
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
		viewDefFilePath := filepath.Join(viewsPath, entry.Name())

		fileContent, err := os.ReadFile(viewDefFilePath)
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
