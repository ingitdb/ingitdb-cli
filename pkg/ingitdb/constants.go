package ingitdb

type RecordFormat string

// Recognized record file formats. Tooling MUST treat unrecognized values as
// unsupported. The values are the strings that appear in
// `record_file.format` in `.collection/definition.yaml`.
const (
	RecordFormatYAML     RecordFormat = "yaml"
	RecordFormatYML      RecordFormat = "yml"
	RecordFormatJSON     RecordFormat = "json"
	RecordFormatMarkdown RecordFormat = "markdown"
)

// DefaultMarkdownContentField is the default name of the column that holds
// the Markdown body when a collection uses `format: markdown` and does not
// override `record_file.content_field`.
const DefaultMarkdownContentField = "$content"

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
