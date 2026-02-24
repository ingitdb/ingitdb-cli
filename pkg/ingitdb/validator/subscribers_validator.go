package validator

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
	"gopkg.in/yaml.v3"
)

// ReadSubscribers reads subscribers from .ingitdb/subscribers.yaml
func ReadSubscribers(dirPath string, o ingitdb.ReadOptions) (map[string]*ingitdb.SubscriberDef, error) {
	return readSubscribers(dirPath, o, os.ReadFile)
}

func readSubscribers(dirPath string, o ingitdb.ReadOptions, readFile func(string) ([]byte, error)) (subs map[string]*ingitdb.SubscriberDef, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	if dirPath == "" {
		dirPath = "."
	}
	filePath := filepath.Join(dirPath, config.SubscribersConfigFileName)

	fileContent, err := readFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// subscribers config is optional
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read subscribers config file: %w", err)
	}

	var cfg config.SubscribersConfig
	decoder := yaml.NewDecoder(bytes.NewReader(fileContent))
	decoder.KnownFields(true)

	if err = decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse subscribers config file: %w", err)
	}

	if o.IsValidationRequired() {
		if err = cfg.Validate(); err != nil {
			return nil, fmt.Errorf("content of subscribers config is not valid: %w", err)
		}
		log.Println(".ingitdb/subscribers.yaml is valid")
	}
	return cfg.Subscribers, nil
}
