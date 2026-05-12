package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseUnset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{name: "single", input: "active", want: []string{"active"}},
		{name: "multiple", input: "active,note", want: []string{"active", "note"}},
		{name: "with spaces", input: "active, note", want: []string{"active", "note"}},
		{name: "empty input", input: "", wantErr: true},
		{name: "trailing comma", input: "active,", wantErr: true},
		{name: "leading comma", input: ",active", wantErr: true},
		{name: "double comma", input: "a,,b", wantErr: true},
		{name: "name with =", input: "active=true", wantErr: true},
		{name: "name with space inside", input: "active flag", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseUnset(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
