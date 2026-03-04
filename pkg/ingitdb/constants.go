package ingitdb

type RecordFormat string

const SchemaDir = ".collection"

// CollectionsDir is the shared-directory layout folder name. When a directory
// contains a CollectionsDir sub-folder, each non-$-prefixed sub-directory
// inside it is treated as a separate collection (ID = sub-directory name).
const CollectionsDir = ".collections"

// SharedViewsDir is the reserved sub-folder name for named views inside a
// CollectionsDir/{name}/ directory (new layout).
const SharedViewsDir = "$views"

// CollectionDefFileName is the fixed file name for collection definitions
// inside the SchemaDir directory.
const CollectionDefFileName = "definition.yaml"

const IngitdbDir = "$ingitdb"
const DefaultViewID = "$default_view"
