package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubscribersConfig_ValidateNil(t *testing.T) {
	var config *SubscribersConfig
	assert.NoError(t, config.Validate())
}
