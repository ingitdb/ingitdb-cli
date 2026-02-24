package config

import (
	"fmt"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

const SubscribersConfigFileName = ".ingitdb/subscribers.yaml"

type SubscribersConfig struct {
	Subscribers map[string]*ingitdb.SubscriberDef `yaml:"subscribers,omitempty"`
}

func (sc *SubscribersConfig) Validate() error {
	if sc == nil {
		return nil
	}
	for id, s := range sc.Subscribers {
		if err := s.Validate(); err != nil {
			return fmt.Errorf("subscriber '%s' is invalid: %w", id, err)
		}
	}
	return nil
}
