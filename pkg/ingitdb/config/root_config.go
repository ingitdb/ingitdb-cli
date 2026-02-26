package config

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// IngitDBDirName is the directory that holds all inGitDB configuration files.
const IngitDBDirName = ".ingitdb"

// RootCollectionsFileName is the file that maps collection IDs to local paths.
// It is a flat YAML map with no wrapper key.
const RootCollectionsFileName = "root-collections.yaml"

// SettingsFileName is the file that holds per-database settings.
const SettingsFileName = "settings.yaml"

// NamespaceImportSuffix is the suffix used to identify namespace import keys.
const NamespaceImportSuffix = ".*"

// Language defines a supported language for this database.
type Language struct {
	Required string `yaml:"required,omitempty"`
	Optional string `yaml:"optional,omitempty"`
}

// Settings holds per-database settings stored in .ingitdb/settings.yaml.
type Settings struct {
	// DefaultNamespace is used as the collection ID prefix when this DB is
	// opened directly (not imported via a namespace import). For example,
	// if DefaultNamespace is "todo" and the DB has collection "tasks",
	// it becomes "todo.tasks" when opened directly.
	DefaultNamespace string     `yaml:"default_namespace,omitempty"`
	Languages        []Language `yaml:"languages,omitempty"`
}

// RootConfig holds the full configuration for an inGitDB database.
// Settings is loaded from .ingitdb/settings.yaml.
// RootCollections is loaded from .ingitdb/root-collections.yaml (flat YAML map).
type RootConfig struct {
	Settings
	RootCollections map[string]string // loaded from root-collections.yaml; no yaml tag (loaded separately)
}

// IsNamespaceImport returns true if the key ends with ".*" suffix,
// indicating it is a namespace import that references another directory's
// .ingitdb.yaml file.
func IsNamespaceImport(key string) bool {
	return strings.HasSuffix(key, NamespaceImportSuffix)
}

// namespaceImportPrefix returns the prefix part of a namespace import key.
// For example, "agile.*" returns "agile".
func namespaceImportPrefix(key string) string {
	return strings.TrimSuffix(key, NamespaceImportSuffix)
}

func (rc *RootConfig) Validate() error {
	if rc == nil {
		return nil
	}
	if rc.DefaultNamespace != "" {
		if err := ingitdb.ValidateCollectionID(rc.DefaultNamespace); err != nil {
			return fmt.Errorf("invalid default_namespace %q: %w", rc.DefaultNamespace, err)
		}
	}
	var paths []string
	for id, path := range rc.RootCollections {
		if id == "" {
			return errors.New("root collection id cannot be empty")
		}
		if IsNamespaceImport(id) {
			// Validate the prefix before ".*"
			prefix := namespaceImportPrefix(id)
			if prefix == "" {
				return fmt.Errorf("namespace import prefix cannot be empty for key %q", id)
			}
			if err := ingitdb.ValidateCollectionID(prefix); err != nil {
				return fmt.Errorf("invalid namespace import prefix %q: %w", id, err)
			}
			if path == "" {
				return fmt.Errorf("namespace import path cannot be empty, key=%s", id)
			}
		} else {
			if err := ingitdb.ValidateCollectionID(id); err != nil {
				return fmt.Errorf("invalid root collection id %q: %w", id, err)
			}
			if path == "" {
				return fmt.Errorf("root collection path cannot be empty, ID=%s", id)
			}
			if path != "" {
				for _, r := range path {
					if r == '*' {
						return fmt.Errorf("root collection path cannot contain wildcard '*', ID=%s, path=%s", id, path)
					}
				}
			}
		}
		for _, p := range paths {
			if p == path {
				return fmt.Errorf("duplicate path for ID=%s: %s", id, p)
			}
		}
		paths = append(paths, path)
	}

	foundOptional := false
	for i, l := range rc.Languages {
		if l.Required != "" && l.Optional != "" {
			return fmt.Errorf("language entry at index %d cannot have both required and optional fields", i)
		}
		if l.Required == "" && l.Optional == "" {
			return fmt.Errorf("language entry at index %d must have either required or optional field", i)
		}

		langCode := l.Required
		if langCode == "" {
			langCode = l.Optional
		}

		// Basic validation for language code format (e.g., "en", "en-US", "zh-Hant-TW")
		// This regex matches simple ISO 639-1 codes and BCP 47 tags with subtags.
		// It is not exhaustive but catches obviously bad formats.
		// Regex explanation:
		// ^[a-zA-Z]{2,3} : Starts with 2 or 3 letters (primary language)
		// (-[a-zA-Z0-9]+)*$ : Optional subtags separated by hyphen
		// We can implement a simple check without pulling in a large regex library if prefered,
		// but simple string checks are efficient.
		// For simplicity/robustness without heavy deps, we'll check length and authorized chars.
		if len(langCode) < 2 {
			return fmt.Errorf("language code '%s' at index %d is too short", langCode, i)
		}
		for _, r := range langCode {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("language code '%s' at index %d contains invalid characters", langCode, i)
			}
		}

		if l.Required != "" {
			if foundOptional {
				return fmt.Errorf("required language '%s' at index %d must be before optional languages", l.Required, i)
			}
		} else {
			foundOptional = true
		}
	}
	return nil
}

// resolvePath resolves a path that can be relative to baseDirPath, absolute,
// or prefixed with ~ for the user's home directory.
func resolvePath(baseDirPath, path string, userHomeDir func() (string, error)) (string, error) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		home, err := userHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to resolve home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(baseDirPath, path), nil
}

// ResolveNamespaceImports resolves all namespace import keys (ending with ".*")
// in rootCollections. For each such key, it reads the
// .ingitdb/root-collections.yaml file from the referenced directory and imports
// all its entries with the namespace prefix prepended.
//
// baseDirPath is the directory containing the current .ingitdb/ directory.
//
// Returns an error if:
//   - The referenced directory does not exist
//   - The referenced directory has no .ingitdb/root-collections.yaml file
//   - The referenced root-collections.yaml has no entries
func (rc *RootConfig) ResolveNamespaceImports(baseDirPath string) error {
	return rc.resolveNamespaceImports(baseDirPath, os.UserHomeDir, os.ReadFile, osStat)
}

// osStat is a variable for testing
var osStat = os.Stat

func (rc *RootConfig) resolveNamespaceImports(
	baseDirPath string,
	userHomeDir func() (string, error),
	readFile func(string) ([]byte, error),
	statFn func(string) (os.FileInfo, error),
) error {
	if rc == nil {
		return nil
	}
	if len(rc.RootCollections) == 0 {
		return nil
	}

	// Collect namespace import keys separately to avoid modifying map during iteration
	var nsKeys []string
	for k := range rc.RootCollections {
		if IsNamespaceImport(k) {
			nsKeys = append(nsKeys, k)
		}
	}

	for _, key := range nsKeys {
		path := rc.RootCollections[key]
		prefix := namespaceImportPrefix(key)

		// Resolve the path
		resolvedPath, err := resolvePath(baseDirPath, path, userHomeDir)
		if err != nil {
			return fmt.Errorf("failed to resolve namespace import path for key %q: %w", key, err)
		}

		// Check if directory exists
		info, err := statFn(resolvedPath)
		if err != nil {
			return fmt.Errorf("namespace import directory not found for key %q, path=%q: %w", key, path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("namespace import path is not a directory for key %q, path=%q", key, path)
		}

		// Read .ingitdb/root-collections.yaml from the referenced directory
		collectionsFilePath := filepath.Join(resolvedPath, IngitDBDirName, RootCollectionsFileName)
		data, err := readFile(collectionsFilePath)
		if err != nil {
			return fmt.Errorf("failed to read %s for namespace import key %q, path=%q: %w", RootCollectionsFileName, key, path, err)
		}

		var importedCollections map[string]string
		if err = yaml.Unmarshal(data, &importedCollections); err != nil {
			return fmt.Errorf("failed to parse %s for namespace import key %q, path=%q: %w", RootCollectionsFileName, key, path, err)
		}

		if len(importedCollections) == 0 {
			return fmt.Errorf("namespace import has no rootCollections for key %q, path=%q", key, path)
		}

		// Remove the namespace import key
		delete(rc.RootCollections, key)

		// Import collections with prefix
		for importedID, importedPath := range importedCollections {
			newID := prefix + "." + importedID
			// Make imported paths relative to the current config's base dir
			newPath := filepath.Join(path, importedPath)
			rc.RootCollections[newID] = newPath
		}
	}

	return nil
}

// ReadSettingsFromFile reads .ingitdb/settings.yaml from dirPath.
// If the file does not exist, returns zero Settings with no error.
func ReadSettingsFromFile(dirPath string, o ingitdb.ReadOptions) (Settings, error) {
	return readSettingsFromFile(dirPath, o, os.ReadFile)
}

func readSettingsFromFile(dirPath string, _ ingitdb.ReadOptions, readFile func(string) ([]byte, error)) (Settings, error) {
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, IngitDBDirName, SettingsFileName)
	data, err := readFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, fmt.Errorf("failed to read settings file: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var s Settings
	if decErr := dec.Decode(&s); decErr != nil {
		return Settings{}, fmt.Errorf("failed to parse settings file: %w", decErr)
	}
	return s, nil
}

// ReadRootCollectionsFromFile reads .ingitdb/root-collections.yaml from dirPath.
// If the file does not exist, returns nil map with no error.
func ReadRootCollectionsFromFile(dirPath string, o ingitdb.ReadOptions) (map[string]string, error) {
	return readRootCollectionsFromFile(dirPath, o, os.ReadFile)
}

func readRootCollectionsFromFile(dirPath string, _ ingitdb.ReadOptions, readFile func(string) ([]byte, error)) (map[string]string, error) {
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, IngitDBDirName, RootCollectionsFileName)
	data, err := readFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read root collections file: %w", err)
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var m map[string]string
	if decErr := dec.Decode(&m); decErr != nil {
		return nil, fmt.Errorf("failed to parse root collections file: %w", decErr)
	}
	return m, nil
}

// ReadRootConfigFromFile reads both .ingitdb/settings.yaml and
// .ingitdb/root-collections.yaml from dirPath, merges them into a RootConfig,
// optionally validates, and resolves namespace imports.
// Missing files are not errors; zero-values are used instead.
func ReadRootConfigFromFile(dirPath string, o ingitdb.ReadOptions) (RootConfig, error) {
	return readRootConfigFromFile(dirPath, o, os.ReadFile)
}

func readRootConfigFromFile(dirPath string, o ingitdb.ReadOptions, readFile func(string) ([]byte, error)) (rootConfig RootConfig, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	var settings Settings
	settings, err = readSettingsFromFile(dirPath, o, readFile)
	if err != nil {
		return
	}

	var rootCollections map[string]string
	rootCollections, err = readRootCollectionsFromFile(dirPath, o, readFile)
	if err != nil {
		return
	}

	rootConfig = RootConfig{
		Settings:        settings,
		RootCollections: rootCollections,
	}

	if o.IsValidationRequired() {
		if err = rootConfig.Validate(); err != nil {
			return rootConfig, fmt.Errorf("content of root config is not valid: %w", err)
		}
		log.Printf("%s/%s and %s/%s are valid", IngitDBDirName, SettingsFileName, IngitDBDirName, RootCollectionsFileName)
	}

	// Resolve namespace imports after validation
	if err = rootConfig.resolveNamespaceImports(dirPath, os.UserHomeDir, readFile, osStat); err != nil {
		return rootConfig, fmt.Errorf("failed to resolve namespace imports: %w", err)
	}

	return
}
