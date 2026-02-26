package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestReadDefinition(t *testing.T) {
	for _, tt := range []struct {
		name            string
		dir             string
		err             string
		wantCollections int
	}{
		{
			name:            "missing_root_config_file",
			dir:             ".",
			err:             "",
			wantCollections: 0,
		},
		{
			name:            "repo_root",
			dir:             "../../../",
			err:             "",
			wantCollections: -1, // any positive count is fine; only check no error
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			currentDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get current dir: %s", err)
			}
			dbDirPath := filepath.Join(currentDir, tt.dir)
			def, err := ReadDefinition(dbDirPath, ingitdb.Validate())
			if err == nil && tt.err != "" {
				t.Fatal("got no error, expected: " + tt.err)
			}
			if tt.err == "" && err != nil {
				t.Fatal("expected no error, got: " + err.Error())
			}
			if tt.err != "" && err != nil && !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("expected error to contain '%s', got '%s'", tt.err, err)
			}
			if tt.err == "" && def == nil {
				t.Fatalf("expected definition to be non-nil")
			}
			if tt.err == "" && len(def.Collections) != tt.wantCollections && tt.wantCollections >= 0 {
				t.Fatalf("expected %d collections, got %d", tt.wantCollections, len(def.Collections))
			}
		})
	}
}
