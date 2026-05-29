package ingitdb

// Settings holds database-level defaults that apply to all collections.
type Settings struct {
	// RecordsDelimiter controls whether a "#-" line is written after each record in INGR output.
	// 0 = use project or app default (app default is 1 = enabled). 1 = enabled. -1 = disabled.
	RecordsDelimiter int `yaml:"records_delimiter,omitempty"`
	// ConflictResolution holds the database-level conflict-resolution
	// defaults (e.g. record-merge), overridable per collection.
	ConflictResolution *ConflictResolutionConfig `yaml:"conflict_resolution,omitempty"`
}

// RuntimeOverrides holds values set at runtime (e.g. CLI flags) that take
// highest priority over schema settings. Not persisted to YAML.
type RuntimeOverrides struct {
	RecordsDelimiter *int `yaml:"-"`
}

type Definition struct {
	Settings         Settings                  `yaml:"settings,omitempty"`
	RuntimeOverrides RuntimeOverrides          `yaml:"-"`
	Collections      map[string]*CollectionDef `yaml:"collections,omitempty"`
	Subscribers      map[string]*SubscriberDef `yaml:"subscribers,omitempty"`
}
