package ingitdb

import (
	"strings"
	"testing"
)

func TestViewDefValidate_MissingID(t *testing.T) {
	t.Parallel()

	v := &ViewDef{}
	err := v.Validate()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "missing 'id' in view definition") {
		t.Fatalf("unexpected error: %s", errMsg)
	}
}

func TestViewDefValidate_Success(t *testing.T) {
	t.Parallel()

	v := &ViewDef{
		ID:      "readme",
		OrderBy: "title",
	}
	if err := v.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestViewDefValidate_Format(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		format  string
		wantErr bool
	}{
		{
			name:    "valid_ingr",
			format:  "ingr",
			wantErr: false,
		},
		{
			name:    "valid_tsv",
			format:  "tsv",
			wantErr: false,
		},
		{
			name:    "valid_csv",
			format:  "csv",
			wantErr: false,
		},
		{
			name:    "valid_json",
			format:  "json",
			wantErr: false,
		},
		{
			name:    "valid_jsonl",
			format:  "jsonl",
			wantErr: false,
		},
		{
			name:    "valid_yaml",
			format:  "yaml",
			wantErr: false,
		},
		{
			name:    "valid_uppercase",
			format:  "CSV",
			wantErr: false,
		},
		{
			name:    "invalid_format",
			format:  "xml",
			wantErr: true,
		},
		{
			name:    "empty_format_valid",
			format:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := &ViewDef{
				ID:     "test",
				Format: tt.format,
			}
			err := v.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "invalid 'format' value") {
				t.Fatalf("expected error to contain 'invalid 'format' value', got %s", err.Error())
			}
		})
	}
}

func TestViewDefValidate_MaxBatchSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		maxBatchSize int
		wantErr      bool
	}{
		{
			name:         "zero_valid",
			maxBatchSize: 0,
			wantErr:      false,
		},
		{
			name:         "positive_valid",
			maxBatchSize: 100,
			wantErr:      false,
		},
		{
			name:         "negative_invalid",
			maxBatchSize: -1,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			v := &ViewDef{
				ID:           "test",
				MaxBatchSize: tt.maxBatchSize,
			}
			err := v.Validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("got error %v, want error %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "'max_batch_size' must be >= 0") {
				t.Fatalf("expected error to contain ''max_batch_size' must be >= 0', got %s", err.Error())
			}
		})
	}
}
