package config

import (
	"strings"
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestSubscribersConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		config      *SubscribersConfig
		errContains string
	}{
		{
			name:   "nil_receiver",
			config: nil,
		},
		{
			name: "empty_subscribers_map",
			config: &SubscribersConfig{
				Subscribers: map[string]*ingitdb.SubscriberDef{},
			},
		},
		{
			name: "valid_subscriber",
			config: &SubscribersConfig{
				Subscribers: map[string]*ingitdb.SubscriberDef{
					"on-change": {
						For: &ingitdb.SubscriberFor{
							Paths:  []string{"*"},
							Events: []ingitdb.TriggerEventType{"created"},
						},
						Webhooks: []ingitdb.WebhookDef{
							{URL: "https://example.com/webhook"},
						},
					},
				},
			},
		},
		{
			name: "invalid_subscriber",
			config: &SubscribersConfig{
				Subscribers: map[string]*ingitdb.SubscriberDef{
					"bad": {
						// Missing 'for' selector — will fail validation
					},
				},
			},
			errContains: "subscriber 'bad' is invalid",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.errContains == "" {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
			}
		})
	}
}
