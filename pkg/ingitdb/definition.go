package ingitdb

// Settings holds database-level defaults that apply to all collections.
type Settings struct {
	RecordsDelimiter bool `yaml:"records_delimiter,omitempty"`
}

// RuntimeOverrides holds values set at runtime (e.g. CLI flags) that take
// highest priority over schema settings. Not persisted to YAML.
type RuntimeOverrides struct {
	RecordsDelimiter *bool `yaml:"-"`
}

type Definition struct {
	Settings         Settings                  `yaml:"settings,omitempty"`
	RuntimeOverrides RuntimeOverrides          `yaml:"-"`
	Collections      map[string]*CollectionDef `yaml:"collections,omitempty"`
	Subscribers      map[string]*SubscriberDef `yaml:"subscribers,omitempty"`
}
