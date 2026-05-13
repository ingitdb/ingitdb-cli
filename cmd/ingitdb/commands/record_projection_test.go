package commands

import "testing"

func TestProjectRecord(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"name":       "Alice",
		"population": 100,
		"country":    "US",
	}

	tests := []struct {
		name     string
		fields   []string
		wantID   bool
		wantKeys []string
	}{
		{
			name:     "all fields (nil)",
			fields:   nil,
			wantID:   true,
			wantKeys: []string{"name", "population", "country"},
		},
		{
			name:     "$id only",
			fields:   []string{"$id"},
			wantID:   true,
			wantKeys: nil,
		},
		{
			name:     "specific fields",
			fields:   []string{"$id", "name"},
			wantID:   true,
			wantKeys: []string{"name"},
		},
		{
			name:     "missing field omitted gracefully",
			fields:   []string{"nonexistent"},
			wantID:   false,
			wantKeys: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := projectRecord(data, "rec1", tt.fields)
			if tt.wantID {
				if _, ok := result["$id"]; !ok {
					t.Error("expected $id in result")
				} else if result["$id"] != "rec1" {
					t.Errorf("expected $id=rec1, got %v", result["$id"])
				}
			} else {
				if _, ok := result["$id"]; ok {
					t.Error("expected $id NOT in result")
				}
			}
			for _, k := range tt.wantKeys {
				if _, ok := result[k]; !ok {
					t.Errorf("expected key %q in result", k)
				}
			}
		})
	}
}
