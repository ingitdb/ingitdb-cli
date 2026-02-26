package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestIsNamespaceImport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key      string
		expected bool
	}{
		{"agile.*", true},
		{"foo.bar.*", true},
		{".*", true},
		{"agile", false},
		{"agile.", false},
		{"", false},
		{"*", false},
		{"agile.teams", false},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			got := IsNamespaceImport(tt.key)
			if got != tt.expected {
				t.Fatalf("IsNamespaceImport(%q) = %v, want %v", tt.key, got, tt.expected)
			}
		})
	}
}

func TestNamespaceImportPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		key      string
		expected string
	}{
		{"agile.*", "agile"},
		{"foo.bar.*", "foo.bar"},
		{".*", ""},
		{"agile", "agile"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Parallel()
			got := namespaceImportPrefix(tt.key)
			if got != tt.expected {
				t.Fatalf("namespaceImportPrefix(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	fakeHome := func() (string, error) {
		return "/home/user", nil
	}
	fakeHomeErr := func() (string, error) {
		return "", errors.New("no home")
	}

	tests := []struct {
		name        string
		baseDirPath string
		path        string
		homeDir     func() (string, error)
		expected    string
		errContains string
	}{
		{
			name:        "relative_path",
			baseDirPath: "/base",
			path:        "sub/dir",
			homeDir:     fakeHome,
			expected:    "/base/sub/dir",
		},
		{
			name:        "absolute_path",
			baseDirPath: "/base",
			path:        "/absolute/path",
			homeDir:     fakeHome,
			expected:    "/absolute/path",
		},
		{
			name:        "tilde_path",
			baseDirPath: "/base",
			path:        "~/projects/data",
			homeDir:     fakeHome,
			expected:    "/home/user/projects/data",
		},
		{
			name:        "tilde_only",
			baseDirPath: "/base",
			path:        "~",
			homeDir:     fakeHome,
			expected:    "/home/user",
		},
		{
			name:        "tilde_home_error",
			baseDirPath: "/base",
			path:        "~/something",
			homeDir:     fakeHomeErr,
			errContains: "failed to resolve home directory",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolvePath(tt.baseDirPath, tt.path, tt.homeDir)
			if tt.errContains != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.expected {
				t.Fatalf("resolvePath(%q, %q) = %q, want %q", tt.baseDirPath, tt.path, got, tt.expected)
			}
		})
	}
}

func TestRootConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		rc   *RootConfig
		err  string
	}{
		{
			name: "nil_receiver",
			rc:   nil,
			err:  "",
		},
		{
			name: "valid_default_namespace",
			rc: &RootConfig{
				Settings: Settings{DefaultNamespace: "todo"},
			},
			err: "",
		},
		{
			name: "valid_default_namespace_dotted",
			rc: &RootConfig{
				Settings: Settings{DefaultNamespace: "app.data"},
			},
			err: "",
		},
		{
			name: "invalid_default_namespace",
			rc: &RootConfig{
				Settings: Settings{DefaultNamespace: ".bad"},
			},
			err: "invalid default_namespace",
		},
		{
			name: "empty_id",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"": "path",
				},
			},
			err: "root collection id cannot be empty",
		},
		{
			name: "empty_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo": "",
				},
			},
			err: "root collection path cannot be empty",
		},
		{
			name: "wildcard_path_not_allowed",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"todo": "todo/*",
				},
			},
			err: "root collection path cannot contain wildcard",
		},
		{
			name: "invalid_collection_id_with_slash",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"todo/tags": "todo/tags",
				},
			},
			err: "invalid root collection id",
		},
		{
			name: "duplicate_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo": "same",
					"bar": "same",
				},
			},
			err: "duplicate path",
		},
		{
			name: "valid_namespace_import",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"agile.*": "demo-dbs/agile-ledger",
				},
			},
			err: "",
		},
		{
			name: "namespace_import_empty_prefix",
			rc: &RootConfig{
				RootCollections: map[string]string{
					".*": "some/path",
				},
			},
			err: "namespace import prefix cannot be empty",
		},
		{
			name: "namespace_import_invalid_prefix",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo/bar.*": "some/path",
				},
			},
			err: "invalid namespace import prefix",
		},
		{
			name: "namespace_import_empty_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"agile.*": "",
				},
			},
			err: "namespace import path cannot be empty",
		},
		{
			name: "namespace_import_duplicate_path",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"foo.*": "same",
					"bar.*": "same",
				},
			},
			err: "duplicate path",
		},
		{
			name: "valid_mixed_namespace_and_regular",
			rc: &RootConfig{
				RootCollections: map[string]string{
					"companies": "demo-dbs/test-db/companies",
					"agile.*":   "demo-dbs/agile-ledger",
				},
			},
			err: "",
		},
		{
			name: "valid_languages",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{Required: "en"},
						{Required: "fr"},
						{Optional: "es"},
					},
				},
			},
			err: "",
		},
		{
			name: "invalid_languages_both_set",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{Required: "en", Optional: "es"},
					},
				},
			},
			err: "cannot have both required and optional fields",
		},
		{
			name: "invalid_languages_neither_set",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{},
					},
				},
			},
			err: "must have either required or optional field",
		},
		{
			name: "invalid_languages_order",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{Optional: "en"},
						{Required: "fr"},
					},
				},
			},
			err: "must be before optional languages",
		},
		{
			name: "invalid_languages_code_short",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{Required: "a"},
					},
				},
			},
			err: "too short",
		},
		{
			name: "invalid_languages_code_chars",
			rc: &RootConfig{
				RootCollections: map[string]string{"foo": "bar"},
				Settings: Settings{
					Languages: []Language{
						{Required: "en$US"},
					},
				},
			},
			err: "contains invalid characters",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.rc.Validate()

			if tt.err == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.err != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.err) {
					t.Fatalf("expected error to contain %q, got %q", tt.err, errMsg)
				}
			}
		})
	}
}

// writeIngitDBFile writes content to a file within the .ingitdb sub-directory
// of dir, creating .ingitdb/ if needed.
func writeIngitDBFile(t *testing.T, dir, filename string, content []byte) {
	t.Helper()
	ingitDir := filepath.Join(dir, IngitDBDirName)
	if err := os.MkdirAll(ingitDir, 0755); err != nil {
		t.Fatalf("create %s dir: %v", IngitDBDirName, err)
	}
	if err := os.WriteFile(filepath.Join(ingitDir, filename), content, 0644); err != nil {
		t.Fatalf("write %s: %v", filename, err)
	}
}

func TestResolveNamespaceImports(t *testing.T) {
	t.Parallel()

	t.Run("nil_receiver", func(t *testing.T) {
		t.Parallel()
		var rc *RootConfig
		err := rc.resolveNamespaceImports(".", nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error for nil receiver, got %v", err)
		}
	})

	t.Run("empty_root_collections", func(t *testing.T) {
		t.Parallel()
		rc := &RootConfig{RootCollections: map[string]string{}}
		err := rc.resolveNamespaceImports(".", nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error for empty collections, got %v", err)
		}
	})

	t.Run("no_namespace_imports", func(t *testing.T) {
		t.Parallel()
		rc := &RootConfig{RootCollections: map[string]string{
			"foo": "path/to/foo",
		}}
		err := rc.resolveNamespaceImports(".", nil, nil, nil)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(rc.RootCollections) != 1 {
			t.Fatalf("expected 1 collection, got %d", len(rc.RootCollections))
		}
	})

	t.Run("successful_import", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write flat root-collections.yaml in sub/.ingitdb/
		writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("teams: teams\nsprints: sprints\n"))

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "sub",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, ok := rc.RootCollections["agile.*"]; ok {
			t.Fatal("namespace import key should be removed")
		}

		if got, ok := rc.RootCollections["agile.teams"]; !ok {
			t.Fatal("expected agile.teams to be imported")
		} else if got != filepath.Join("sub", "teams") {
			t.Fatalf("expected path %q, got %q", filepath.Join("sub", "teams"), got)
		}

		if got, ok := rc.RootCollections["agile.sprints"]; !ok {
			t.Fatal("expected agile.sprints to be imported")
		} else if got != filepath.Join("sub", "sprints") {
			t.Fatalf("expected path %q, got %q", filepath.Join("sub", "sprints"), got)
		}
	})

	t.Run("directory_not_found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "nonexistent",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "namespace import directory not found") {
			t.Fatalf("expected 'namespace import directory not found', got %q", err.Error())
		}
	})

	t.Run("path_is_not_a_directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		// Create a file instead of directory
		filePath := filepath.Join(dir, "notadir")
		if err := os.WriteFile(filePath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "notadir",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "namespace import path is not a directory") {
			t.Fatalf("expected 'not a directory' error, got %q", err.Error())
		}
	})

	t.Run("missing_config_file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		// No .ingitdb/root-collections.yaml in subDir

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "sub",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to read") {
			t.Fatalf("expected 'failed to read' error, got %q", err.Error())
		}
	})

	t.Run("empty_root_collections_in_imported", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Write an empty flat map
		writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("{}\n"))

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "sub",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "namespace import has no rootCollections") {
			t.Fatalf("expected 'no rootCollections' error, got %q", err.Error())
		}
	})

	t.Run("invalid_yaml_in_imported", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("{{invalid yaml"))

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "sub",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse") {
			t.Fatalf("expected 'failed to parse' error, got %q", err.Error())
		}
	})

	t.Run("resolve_path_error_home_dir", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		rc := &RootConfig{RootCollections: map[string]string{
			"agile.*": "~/some/path",
		}}

		fakeHomeErr := func() (string, error) {
			return "", errors.New("no home")
		}

		err := rc.resolveNamespaceImports(dir, fakeHomeErr, os.ReadFile, os.Stat)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to resolve namespace import path") {
			t.Fatalf("expected 'failed to resolve namespace import path' error, got %q", err.Error())
		}
	})

	t.Run("absolute_path_import", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		absSubDir := filepath.Join(dir, "absolute-sub")
		if err := os.MkdirAll(absSubDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeIngitDBFile(t, absSubDir, RootCollectionsFileName, []byte("items: items\n"))

		rc := &RootConfig{RootCollections: map[string]string{
			"ns.*": absSubDir,
		}}

		err := rc.resolveNamespaceImports("/other/dir", os.UserHomeDir, os.ReadFile, os.Stat)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, ok := rc.RootCollections["ns.items"]; !ok {
			t.Fatal("expected ns.items to be imported")
		} else if got != filepath.Join(absSubDir, "items") {
			t.Fatalf("expected path %q, got %q", filepath.Join(absSubDir, "items"), got)
		}
	})

	t.Run("home_dir_path_import", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		homeSubDir := filepath.Join(dir, "home-data")
		if err := os.MkdirAll(homeSubDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeIngitDBFile(t, homeSubDir, RootCollectionsFileName, []byte("records: records\n"))

		fakeHome := func() (string, error) {
			return dir, nil
		}

		rc := &RootConfig{RootCollections: map[string]string{
			"data.*": "~/home-data",
		}}

		err := rc.resolveNamespaceImports("/other/dir", fakeHome, os.ReadFile, os.Stat)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got, ok := rc.RootCollections["data.records"]; !ok {
			t.Fatal("expected data.records to be imported")
		} else if got != filepath.Join("~/home-data", "records") {
			t.Fatalf("expected path %q, got %q", filepath.Join("~/home-data", "records"), got)
		}
	})

	t.Run("mixed_namespace_and_regular", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		subDir := filepath.Join(dir, "sub")
		if err := os.MkdirAll(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("teams: teams\n"))

		rc := &RootConfig{RootCollections: map[string]string{
			"companies": "companies-path",
			"agile.*":   "sub",
		}}

		err := rc.resolveNamespaceImports(dir, os.UserHomeDir, os.ReadFile, os.Stat)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Regular collection should be preserved
		if got, ok := rc.RootCollections["companies"]; !ok {
			t.Fatal("expected companies to be present")
		} else if got != "companies-path" {
			t.Fatalf("expected path %q, got %q", "companies-path", got)
		}

		// Namespace import should be resolved
		if _, ok := rc.RootCollections["agile.*"]; ok {
			t.Fatal("namespace import key should be removed")
		}
		if got, ok := rc.RootCollections["agile.teams"]; !ok {
			t.Fatal("expected agile.teams to be imported")
		} else if got != filepath.Join("sub", "teams") {
			t.Fatalf("expected path %q, got %q", filepath.Join("sub", "teams"), got)
		}
	})
}

func TestReadSettingsFromFile(t *testing.T) {
	t.Parallel()

	t.Run("missing_file_returns_zero_value", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		s, err := ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if s.DefaultNamespace != "" {
			t.Fatalf("expected empty DefaultNamespace, got %q", s.DefaultNamespace)
		}
		if len(s.Languages) != 0 {
			t.Fatalf("expected no languages, got %d", len(s.Languages))
		}
	})

	t.Run("valid_settings", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeIngitDBFile(t, dir, SettingsFileName, []byte("default_namespace: myapp\nlanguages:\n  - required: en\n  - optional: fr\n"))
		s, err := ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if s.DefaultNamespace != "myapp" {
			t.Fatalf("expected default_namespace 'myapp', got %q", s.DefaultNamespace)
		}
		if len(s.Languages) != 2 {
			t.Fatalf("expected 2 languages, got %d", len(s.Languages))
		}
	})

	t.Run("unknown_field_returns_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeIngitDBFile(t, dir, SettingsFileName, []byte("unknown_field: value\n"))
		_, err := ReadSettingsFromFile(dir, ingitdb.NewReadOptions())
		if err == nil {
			t.Fatal("expected error for unknown field, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse settings file") {
			t.Fatalf("expected 'failed to parse settings file' error, got %q", err.Error())
		}
	})

	t.Run("empty_dir_path_treated_as_dot", func(t *testing.T) {
		t.Parallel()
		// With "" dirPath, should not error (no .ingitdb/settings.yaml at ".")
		s, err := ReadSettingsFromFile("", ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("expected no error for empty dirPath, got %v", err)
		}
		// zero-value expected
		if s.DefaultNamespace != "" {
			t.Fatalf("expected empty DefaultNamespace, got %q", s.DefaultNamespace)
		}
	})
}

func TestReadRootCollectionsFromFile(t *testing.T) {
	t.Parallel()

	t.Run("missing_file_returns_nil_map", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		m, err := ReadRootCollectionsFromFile(dir, ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if m != nil {
			t.Fatalf("expected nil map, got %v", m)
		}
	})

	t.Run("valid_flat_map", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("companies: demo-dbs/test-db/companies\ntodo: docs/todo\n"))
		m, err := ReadRootCollectionsFromFile(dir, ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(m) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(m))
		}
		if m["companies"] != "demo-dbs/test-db/companies" {
			t.Errorf("unexpected companies path: %q", m["companies"])
		}
		if m["todo"] != "docs/todo" {
			t.Errorf("unexpected todo path: %q", m["todo"])
		}
	})

	t.Run("invalid_yaml_returns_error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("{{invalid"))
		_, err := ReadRootCollectionsFromFile(dir, ingitdb.NewReadOptions())
		if err == nil {
			t.Fatal("expected error for invalid YAML, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse root collections file") {
			t.Fatalf("expected 'failed to parse root collections file' error, got %q", err.Error())
		}
	})

	t.Run("empty_dir_path_treated_as_dot", func(t *testing.T) {
		t.Parallel()
		m, err := ReadRootCollectionsFromFile("", ingitdb.NewReadOptions())
		if err != nil {
			t.Fatalf("expected no error for empty dirPath, got %v", err)
		}
		// Likely nil since no .ingitdb/root-collections.yaml at "."
		_ = m
	})
}

func TestReadRootConfigFromFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		setup         func(dir string) error
		options       ingitdb.ReadOptions
		dirPath       string
		useDirPath    bool
		expectedError string
		verify        func(t *testing.T, rc RootConfig)
	}{
		{
			name:          "missing_files_returns_zero_value",
			options:       ingitdb.NewReadOptions(),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if rc.DefaultNamespace != "" {
					t.Errorf("expected empty DefaultNamespace, got %q", rc.DefaultNamespace)
				}
				if rc.RootCollections != nil {
					t.Errorf("expected nil RootCollections, got %v", rc.RootCollections)
				}
			},
		},
		{
			name:          "empty_dir_path_returns_zero_value",
			options:       ingitdb.NewReadOptions(),
			dirPath:       "",
			useDirPath:    true,
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if rc.DefaultNamespace != "" {
					t.Errorf("expected empty DefaultNamespace, got %q", rc.DefaultNamespace)
				}
			},
		},
		{
			name: "unknown_field_in_settings",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, SettingsFileName, []byte("unknown: value\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(),
			expectedError: "failed to parse settings file",
		},
		{
			name: "invalid_content_with_validation",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("\"\": \"path\"\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "content of root config is not valid",
		},
		{
			name: "valid_content_with_validation",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("countries: \"geo/countries\"\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
		},
		{
			name: "default_namespace_parsed",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, SettingsFileName, []byte("default_namespace: myapp\n"))
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("users: users\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if rc.DefaultNamespace != "myapp" {
					t.Fatalf("expected default_namespace 'myapp', got %q", rc.DefaultNamespace)
				}
			},
		},
		{
			name: "valid_languages_yaml",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, SettingsFileName, []byte("languages:\n  - required: en\n  - optional: fr\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if len(rc.Languages) != 2 {
					t.Fatalf("expected 2 languages, got %d", len(rc.Languages))
				}
				if rc.Languages[0].Required != "en" {
					t.Errorf("expected first language required=en, got %s", rc.Languages[0].Required)
				}
				if rc.Languages[1].Optional != "fr" {
					t.Errorf("expected second language optional=fr, got %s", rc.Languages[1].Optional)
				}
			},
		},
		{
			name: "namespace_import_integration",
			setup: func(dir string) error {
				// Main DB: root-collections.yaml references sub dir via namespace import
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("agile.*: sub\n"))

				// Sub directory with its own .ingitdb/root-collections.yaml
				subDir := filepath.Join(dir, "sub")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("teams: teams\nsprints: sprints\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if _, ok := rc.RootCollections["agile.*"]; ok {
					t.Fatal("namespace import key should be removed")
				}
				if got, ok := rc.RootCollections["agile.teams"]; !ok {
					t.Fatal("expected agile.teams")
				} else if got != filepath.Join("sub", "teams") {
					t.Fatalf("expected path %q, got %q", filepath.Join("sub", "teams"), got)
				}
				if got, ok := rc.RootCollections["agile.sprints"]; !ok {
					t.Fatal("expected agile.sprints")
				} else if got != filepath.Join("sub", "sprints") {
					t.Fatalf("expected path %q, got %q", filepath.Join("sub", "sprints"), got)
				}
			},
		},
		{
			name: "namespace_import_dir_not_found",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("agile.*: nonexistent\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(),
			expectedError: "failed to resolve namespace imports",
		},
		{
			name: "namespace_import_with_validation",
			setup: func(dir string) error {
				writeIngitDBFile(t, dir, RootCollectionsFileName, []byte("agile.*: sub\n"))
				subDir := filepath.Join(dir, "sub")
				if err := os.MkdirAll(subDir, 0755); err != nil {
					return err
				}
				writeIngitDBFile(t, subDir, RootCollectionsFileName, []byte("teams: teams\n"))
				return nil
			},
			options:       ingitdb.NewReadOptions(ingitdb.Validate()),
			expectedError: "",
			verify: func(t *testing.T, rc RootConfig) {
				if got, ok := rc.RootCollections["agile.teams"]; !ok {
					t.Fatal("expected agile.teams")
				} else if got != filepath.Join("sub", "teams") {
					t.Fatalf("expected path %q, got %q", filepath.Join("sub", "teams"), got)
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if tt.setup != nil {
				err := tt.setup(dir)
				if err != nil {
					errMsg := err.Error()
					t.Fatalf("failed to setup test data: %s", errMsg)
				}
			}

			dirPath := dir
			if tt.useDirPath {
				dirPath = tt.dirPath
			}

			rc, err := ReadRootConfigFromFile(dirPath, tt.options)
			if tt.expectedError == "" && err != nil {
				errMsg := err.Error()
				t.Fatalf("expected no error, got %s", errMsg)
			}
			if tt.expectedError != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.expectedError) {
					t.Fatalf("expected error to contain %q, got %q", tt.expectedError, errMsg)
				}
			}
			if tt.verify != nil {
				tt.verify(t, rc)
			}
		})
	}
}

func TestReadRootConfigFromFile_PanicRecovery(t *testing.T) {
	t.Parallel()

	readFile := func(string) ([]byte, error) {
		panic("boom")
	}

	_, err := readRootConfigFromFile("irrelevant", ingitdb.NewReadOptions(), readFile)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "panic: boom") {
		t.Fatalf("expected panic error, got %s", errMsg)
	}
}
