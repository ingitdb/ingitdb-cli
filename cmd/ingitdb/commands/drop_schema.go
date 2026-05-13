package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// rootCollectionsRelPath is the on-disk location of the root
// collections registry, relative to the database root.
const rootCollectionsRelPath = ".ingitdb/root-collections.yaml"

// readRootCollections reads the database's root-collections.yaml file
// and returns it as a map of collection-name → relative directory
// path.
func readRootCollections(dbDir string) (map[string]string, error) {
	path := filepath.Join(dbDir, rootCollectionsRelPath)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", rootCollectionsRelPath, err)
	}
	entries := make(map[string]string)
	if unmarshalErr := yaml.Unmarshal(raw, &entries); unmarshalErr != nil {
		return nil, fmt.Errorf("parse %s: %w", rootCollectionsRelPath, unmarshalErr)
	}
	return entries, nil
}

// writeRootCollectionsWithout reads the root-collections.yaml file,
// removes the entry for the named collection (no-op if absent), and
// writes the file back. Other entries are preserved with their
// original values.
func writeRootCollectionsWithout(dbDir, dropName string) error {
	entries, err := readRootCollections(dbDir)
	if err != nil {
		return err
	}
	delete(entries, dropName)
	out, err := yaml.Marshal(entries)
	if err != nil {
		return fmt.Errorf("marshal root-collections: %w", err)
	}
	path := filepath.Join(dbDir, rootCollectionsRelPath)
	if writeErr := os.WriteFile(path, out, 0o644); writeErr != nil {
		return fmt.Errorf("write %s: %w", rootCollectionsRelPath, writeErr)
	}
	return nil
}
