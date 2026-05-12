package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []string // nil means "all fields"
		wantErr bool
	}{
		{name: "star", input: "*", want: nil},
		{name: "empty", input: "", want: nil},
		{name: "single id", input: "$id", want: []string{"$id"}},
		{name: "id and name", input: "$id,name", want: []string{"$id", "name"}},
		{name: "with spaces", input: "$id, name , age", want: []string{"$id", "name", "age"}},
		{name: "empty between commas", input: "name,,age", wantErr: true},
		{name: "trailing comma", input: "name,", wantErr: true},
		{name: "leading comma", input: ",name", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseFields(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.input)
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
