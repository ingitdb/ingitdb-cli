package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseOrderBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    []OrderTerm
		wantErr bool
	}{
		{name: "empty", input: "", want: nil},
		{name: "single ascending", input: "name", want: []OrderTerm{{Field: "name", Descending: false}}},
		{name: "single descending", input: "-population", want: []OrderTerm{{Field: "population", Descending: true}}},
		{name: "mixed", input: "country,-population,name", want: []OrderTerm{
			{Field: "country", Descending: false},
			{Field: "population", Descending: true},
			{Field: "name", Descending: false},
		}},
		{name: "with spaces", input: "country, -population , name", want: []OrderTerm{
			{Field: "country", Descending: false},
			{Field: "population", Descending: true},
			{Field: "name", Descending: false},
		}},
		{name: "dash only", input: "-", wantErr: true},
		{name: "empty between commas", input: "name,,country", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOrderBy(tt.input)
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
