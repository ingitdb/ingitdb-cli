package commands

import (
	"testing"

	"github.com/dal-go/dalgo/dal"
)

func TestParseWhereExpr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantString string // expected Condition.String() output
	}{
		{name: "equal ==", input: "name==Alice", wantString: "name = 'Alice'"},
		{name: "equal =", input: "name=Alice", wantString: "name = 'Alice'"},
		{name: "greater than", input: "population>1000000", wantString: "population > 1000000"},
		{name: "greater or equal", input: "score>=90", wantString: "score >= 90"},
		{name: "less than", input: "age<30", wantString: "age < 30"},
		{name: "less or equal", input: "age<=18", wantString: "age <= 18"},
		{name: "number with comma", input: "population>1,000,000", wantString: "population > 1000000"},
		{name: "string value", input: "status==active", wantString: "status = 'active'"},
		{name: "missing operator", input: "fieldname", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "missing field", input: ">100", wantErr: true},
		{name: "missing value", input: "field>", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cond, err := parseWhereExpr(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cond == nil {
				t.Fatal("expected non-nil condition")
			}
			if tt.wantString != "" && cond.String() != tt.wantString {
				t.Errorf("expected %q, got %q", tt.wantString, cond.String())
			}
		})
	}
}

func TestParseOrderBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		input          string
		wantLen        int
		wantErr        bool
		wantDescending []bool
		wantFields     []string
	}{
		{
			name:           "single ascending",
			input:          "name",
			wantLen:        1,
			wantDescending: []bool{false},
			wantFields:     []string{"name"},
		},
		{
			name:           "single descending",
			input:          "-population",
			wantLen:        1,
			wantDescending: []bool{true},
			wantFields:     []string{"population"},
		},
		{
			name:           "multiple fields",
			input:          "country,-population,name",
			wantLen:        3,
			wantDescending: []bool{false, true, false},
			wantFields:     []string{"country", "population", "name"},
		},
		{name: "empty string", input: "", wantLen: 0},
		{name: "whitespace only", input: "   ", wantLen: 0},
		{name: "dash only", input: "-", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			exprs, err := parseOrderBy(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(exprs) != tt.wantLen {
				t.Fatalf("expected %d expressions, got %d", tt.wantLen, len(exprs))
			}
			for i, expr := range exprs {
				if expr.Descending() != tt.wantDescending[i] {
					t.Errorf("[%d] expected descending=%v, got %v", i, tt.wantDescending[i], expr.Descending())
				}
				fieldRef, ok := expr.Expression().(dal.FieldRef)
				if !ok {
					t.Errorf("[%d] expected FieldRef, got %T", i, expr.Expression())
					continue
				}
				if fieldRef.Name() != tt.wantFields[i] {
					t.Errorf("[%d] expected field %q, got %q", i, tt.wantFields[i], fieldRef.Name())
				}
			}
		})
	}
}

func TestParseFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  []string // nil means "all fields"
	}{
		{name: "star", input: "*", want: nil},
		{name: "empty", input: "", want: nil},
		{name: "single", input: "$id", want: []string{"$id"}},
		{name: "multiple", input: "$id,name,age", want: []string{"$id", "name", "age"}},
		{name: "with spaces", input: "$id, name , age", want: []string{"$id", "name", "age"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := parseFields(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
			for i, f := range tt.want {
				if got[i] != f {
					t.Errorf("[%d] expected %q, got %q", i, f, got[i])
				}
			}
		})
	}
}

func TestProjectRecord(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"name":       "Alice",
		"population": 100,
		"country":    "US",
	}

	tests := []struct {
		name   string
		fields []string
		wantID bool
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
		tt := tt
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
