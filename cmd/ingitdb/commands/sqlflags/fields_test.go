package sqlflags

import (
	"reflect"
	"testing"
)

func TestParseFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string // nil means "all fields"
	}{
		{name: "star", input: "*", want: nil},
		{name: "empty", input: "", want: nil},
		{name: "single id", input: "$id", want: []string{"$id"}},
		{name: "id and name", input: "$id,name", want: []string{"$id", "name"}},
		{name: "with spaces", input: "$id, name , age", want: []string{"$id", "name", "age"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ParseFields(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
}
