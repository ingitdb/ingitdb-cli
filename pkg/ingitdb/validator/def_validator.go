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
		err = fmt.Errorf("failed to read root config file %s: %v", config.RootConfigFileName, err)
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
		if colDef, err = readCollectionDef(rootPath, colPath, id, o); err != nil {
			err = fmt.Errorf("failed to validate root collection def ID=%s: %w", id, err)
			return
		}
		def.Collections[id] = colDef
	}
	return
}

func readCollectionDef(rootPath, relPath, id string, o ingitdb.ReadOptions) (colDef *ingitdb.CollectionDef, err error) {
	colDefFilePath := filepath.Join(rootPath, relPath, ingitdb.SchemaDir, id+".yaml")
	var fileContent []byte
	fileContent, err = os.ReadFile(colDefFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
	}
	//log.Println(string(fileContent))
	colDef = new(ingitdb.CollectionDef)

	err = yaml.Unmarshal(fileContent, colDef)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
	}
	colDef.ID = id
	colDef.DirPath = filepath.Join(rootPath, relPath)

	if o.IsValidationRequired() {
		if err = colDef.Validate(); err != nil {
			err = fmt.Errorf("not valid definition of collection '%s': %w", id, err)
			return
		}
		log.Printf("Definition of collection '%s' is valid", colDef.ID)
	}

	if colDef.SubCollections, err = loadSubCollections(rootPath, relPath, "", id, o); err != nil {
		err = fmt.Errorf("failed to load subcollections for '%s': %w", id, err)
		return
	}

	return
}

func loadSubCollections(rootPath, relPath, parentSubDir, parentPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.CollectionDef, error) {
	subCollectionsPath := filepath.Join(rootPath, relPath, ingitdb.SchemaDir, "subcollections", parentSubDir)
	entries, err := os.ReadDir(subCollectionsPath)
	if os.IsNotExist(err) {
		return nil, nil // No subcollections
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read subcollections directory: %w", err)
	}

	var subCollections map[string]*ingitdb.CollectionDef

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			id := strings.TrimSuffix(entry.Name(), ".yaml")
			colDefFilePath := filepath.Join(subCollectionsPath, entry.Name())

			fileContent, err := os.ReadFile(colDefFilePath)
			if err != nil {
				return nil, fmt.Errorf("failed to read file %s: %w", colDefFilePath, err)
			}

			colDef := new(ingitdb.CollectionDef)
			if err = yaml.Unmarshal(fileContent, colDef); err != nil {
				return nil, fmt.Errorf("failed to parse YAML file %s: %w", colDefFilePath, err)
			}
			colDef.ID = id
			colDef.DirPath = filepath.Join(rootPath, relPath)

			fullPath := parentPath + "/" + id

			if o.IsValidationRequired() {
				if err = colDef.Validate(); err != nil {
					return nil, fmt.Errorf("not valid definition of subcollection '%s': %w", fullPath, err)
				}
				log.Printf("Definition of subcollection '%s' is valid", fullPath)
			}

			subSubDir := filepath.Join(parentSubDir, id)
			subCols, err := loadSubCollections(rootPath, relPath, subSubDir, fullPath, o)
			if err != nil {
				return nil, err
			}
			if len(subCols) > 0 {
				colDef.SubCollections = subCols
			}

			if subCollections == nil {
				subCollections = make(map[string]*ingitdb.CollectionDef)
			}
			subCollections[id] = colDef
		}
	}
	return subCollections, nil
}
