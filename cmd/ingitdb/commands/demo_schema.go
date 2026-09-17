package commands

// specscore: feature/cli/demo

import (
	"github.com/ingitdb/ingitdb-go/ingitdb"
	"github.com/ingitdb/ingitdb-go/ingitdb/config"
)

// The TODO demo's inGitDB-specific files. The records and list paths come
// from the shared package github.com/ingitdb/ingitdb-go/ingitdb/demos/todo;
// the definitions and the marker have one consumer, this command, so they
// live here (cli/demo#REQ:shared-demo-data).
const (
	demoListsCollection = "lists"
	demoItemsCollection = "items"

	// demoMarkerFileName is the marker a later install reads to recognise the
	// folder as the demo (cli/demo#REQ:demo-marker).
	demoMarkerFileName = "demo.yaml"
	// demoMarkerVersion is the marker's format version.
	demoMarkerVersion = 1

	demoCommitMessage = "Install the TODO demo"

	// demoLockFileName is the file an install creates exclusively in the demo
	// folder to claim it, and removes once the demo is complete. A marker
	// without this lock means the demo is complete.
	demoLockFileName = ".ingitdb-demo-install.lock"
)

// demoMarker is the content of .ingitdb/demo.yaml.
type demoMarker struct {
	App     string `yaml:"app"`
	Version int    `yaml:"version"`
}

// demoSingleRecordFile is the record file shape both demo collections use:
// one YAML file per record, named after the record key.
func demoSingleRecordFile() *ingitdb.RecordFileDef {
	return &ingitdb.RecordFileDef{
		Name:       "{key}.yaml",
		Format:     ingitdb.RecordFormatYAML,
		RecordType: ingitdb.SingleRecord,
	}
}

// demoListsDef is the definition of the root collection `lists`.
func demoListsDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		RecordFile: demoSingleRecordFile(),
		Columns: map[string]*ingitdb.ColumnDef{
			"title": {Type: ingitdb.ColumnTypeString, Required: true},
		},
		ColumnsOrder: []string{"title"},
	}
}

// demoItemsDef is the definition of the subcollection `items` of `lists`.
func demoItemsDef() *ingitdb.CollectionDef {
	return &ingitdb.CollectionDef{
		RecordFile: demoSingleRecordFile(),
		Columns: map[string]*ingitdb.ColumnDef{
			"title":    {Type: ingitdb.ColumnTypeString, Required: true},
			"done":     {Type: ingitdb.ColumnTypeBool},
			"added_at": {Type: ingitdb.ColumnTypeDateTime},
		},
		ColumnsOrder: []string{"title", "done", "added_at"},
	}
}

// demoFile is one configuration file the install writes, relative to the
// demo folder with forward slashes.
type demoFile struct {
	path  string
	value any
}

// demoConfigFiles lists the configuration files of the TODO demo, except the
// marker, in the order they are written.
func demoConfigFiles() []demoFile {
	return []demoFile{
		{path: config.IngitDBDirName + "/" + config.SettingsFileName, value: config.Settings{}},
		{path: config.IngitDBDirName + "/" + config.RootCollectionsFileName, value: map[string]string{demoListsCollection: demoListsCollection}},
		{path: demoListsCollection + "/.collection/definition.yaml", value: demoListsDef()},
		{path: demoListsCollection + "/.collection/subcollections/" + demoItemsCollection + "/definition.yaml", value: demoItemsDef()},
	}
}

// demoMarkerFile is the .ingitdb/demo.yaml marker, written after every other
// file of the demo.
func demoMarkerFile() demoFile {
	return demoFile{path: config.IngitDBDirName + "/" + demoMarkerFileName, value: demoMarker{App: demoApp, Version: demoMarkerVersion}}
}
