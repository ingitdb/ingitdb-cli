package ingitdb

import (
	"fmt"
)

// ValidateCollectionID validates a full collection ID.
// Allowed characters are alphanumeric, dot, and underscore; ID must start/end with alphanumeric.
func ValidateCollectionID(id string) error {
	if id == "" {
		return fmt.Errorf("collection id cannot be empty")
	}
	for i, r := range id {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')
		if i == 0 || i == len(id)-1 {
			if !isAlphaNum {
				return fmt.Errorf("collection id %q must start and end with alphanumeric character", id)
			}
			continue
		}
		if isAlphaNum || r == '.' || r == '_' {
			continue
		}
		return fmt.Errorf("collection id %q contains invalid character %q", id, r)
	}
	return nil
}
