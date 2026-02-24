package materializer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

// FileViewDefReader reads view definitions from .collection/views/*.yaml files.
type FileViewDefReader struct{}

func (FileViewDefReader) ReadViewDefs(colDirPath string) (map[string]*ingitdb.ViewDef, error) {
	pattern := filepath.Join(colDirPath, ".collection", "views", "*.yaml")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob view defs: %w", err)
	}
	defs := make(map[string]*ingitdb.ViewDef, len(matches))
	for _, path := range matches {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read view def %s: %w", path, err)
		}
		var view ingitdb.ViewDef
		if err := yaml.Unmarshal(content, &view); err != nil {
			return nil, fmt.Errorf("failed to parse view def %s: %w", path, err)
		}
		name, err := viewNameFromPath(path)
		if err != nil {
			return nil, err
		}
		view.ID = name
		defs[name] = &view
	}
	return defs, nil
}

func viewNameFromPath(path string) (string, error) {
	base := filepath.Base(path)
	const suffix = ".yaml"
	if !strings.HasSuffix(base, suffix) {
		return "", fmt.Errorf("invalid view def file name: %s", base)
	}
	name := strings.TrimSuffix(base, suffix)
	if name == "" {
		return "", fmt.Errorf("missing view name in file: %s", base)
	}
	return name, nil
}
