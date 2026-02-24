package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

const RootConfigFileName = ".ingitdb.yaml"

// NamespaceImportSuffix is the suffix used to identify namespace import keys.
const NamespaceImportSuffix = ".*"

type Language struct {
	Required string `yaml:"required,omitempty"`
	Optional string `yaml:"optional,omitempty"`
}

type RootConfig struct {
	RootCollections map[string]string `yaml:"rootCollections,omitempty"`
	Languages       []Language        `yaml:"languages,omitempty"`
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
// in rootCollections. For each such key, it reads the .ingitdb.yaml file from
// the referenced directory and imports all its rootCollections with the
// namespace prefix prepended.
//
// baseDirPath is the directory containing the current .ingitdb.yaml file.
//
// Returns an error if:
//   - The referenced directory does not exist
//   - The referenced directory has no .ingitdb.yaml file
//   - The referenced .ingitdb.yaml has no or empty rootCollections
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

		// Read .ingitdb.yaml from the referenced directory
		configFilePath := filepath.Join(resolvedPath, RootConfigFileName)
		data, err := readFile(configFilePath)
		if err != nil {
			return fmt.Errorf("failed to read %s for namespace import key %q, path=%q: %w", RootConfigFileName, key, path, err)
		}

		var importedConfig RootConfig
		if err = yaml.Unmarshal(data, &importedConfig); err != nil {
			return fmt.Errorf("failed to parse %s for namespace import key %q, path=%q: %w", RootConfigFileName, key, path, err)
		}

		if len(importedConfig.RootCollections) == 0 {
			return fmt.Errorf("namespace import has no rootCollections for key %q, path=%q", key, path)
		}

		// Remove the namespace import key
		delete(rc.RootCollections, key)

		// Import collections with prefix
		for importedID, importedPath := range importedConfig.RootCollections {
			newID := prefix + "." + importedID
			// Make imported paths relative to the current config's base dir
			newPath := filepath.Join(path, importedPath)
			rc.RootCollections[newID] = newPath
		}
	}

	return nil
}

func ReadRootConfigFromFile(dirPath string, o ingitdb.ReadOptions) (rootConfig RootConfig, err error) {
	return readRootConfigFromFile(dirPath, o, os.OpenFile)
}

func readRootConfigFromFile(dirPath string, o ingitdb.ReadOptions, openFile func(string, int, os.FileMode) (*os.File, error)) (rootConfig RootConfig, err error) {
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, RootConfigFileName)

	var file *os.File
	if file, err = openFile(filePath, os.O_RDONLY, 0666); err != nil {
		err = fmt.Errorf("failed to open root config file: %w", err)
		return
	}

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)

	if err = decoder.Decode(&rootConfig); err != nil {
		err = fmt.Errorf("failed to parse root config file: %w\nNote: Expected keys in .ingitdb.yaml include 'rootCollections'", err)
		return
	}

	if o.IsValidationRequired() {
		if err = rootConfig.Validate(); err != nil {
			return rootConfig, fmt.Errorf("content of root config is not valid: %w", err)
		}
		log.Println(".ingitdb.yaml is valid")
	}

	// Resolve namespace imports after validation
	if err = rootConfig.ResolveNamespaceImports(dirPath); err != nil {
		return rootConfig, fmt.Errorf("failed to resolve namespace imports: %w", err)
	}

	return
}
