package ingitdb

import (
	"strings"
	"testing"
)

func TestValidateColumnType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ct      ColumnType
		wantErr string
	}{
		{
			name:    "missing",
			ct:      "",
			wantErr: errMissingRequiredField.Error(),
		},
		{
			name: "known_string",
			ct:   ColumnTypeString,
		},
		{
			name: "known_l10n",
			ct:   ColumnTypeL10N,
		},
		{
			name:    "map_missing_key_type",
			ct:      "map[]string",
			wantErr: "missing key type",
		},
		{
			name:    "map_unsupported_key_type",
			ct:      "map[uuid]string",
			wantErr: "unsupported key type",
		},
		{
			name: "map_supported_key_type",
			ct:   "map[string]string",
		},
		{
			name: "custom_non_map",
			ct:   "custom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateColumnType(tt.ct)
			if tt.wantErr == "" && err != nil {
				t.Fatalf("expected no error, got %s", err)
			}
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error to contain %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}
