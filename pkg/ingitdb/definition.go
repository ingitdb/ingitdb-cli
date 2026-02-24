package ingitdb

type Definition struct {
	Collections map[string]*CollectionDef `yaml:"collections,omitempty"`
	Subscribers map[string]*SubscriberDef `yaml:"subscribers,omitempty"`
}
