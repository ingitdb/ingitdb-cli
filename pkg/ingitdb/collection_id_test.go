package ingitdb

import "testing"

func TestValidateCollectionID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   string
		ok   bool
	}{
		{name: "single_alnum", id: "a", ok: true},
		{name: "namespaced", id: "todo.tags", ok: true},
		{name: "underscore", id: "exchange_rates", ok: true},
		{name: "namespace_underscore", id: "demo_commerce", ok: true},
		{name: "contains_slash", id: "todo/tags", ok: false},
		{name: "contains_dash", id: "todo-tags", ok: false},
		{name: "starts_with_dot", id: ".todo", ok: false},
		{name: "ends_with_dot", id: "todo.", ok: false},
		{name: "empty", id: "", ok: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateCollectionID(tt.id)
			if tt.ok && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatalf("expected error for id %q", tt.id)
			}
		})
	}
}
