package commands

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteSingleRecord(t *testing.T) {
	t.Parallel()

	record := map[string]any{"$id": "ie", "name": "Ireland", "population": float64(5000000)}

	tests := []struct {
		name    string
		format  string
		columns []string
		want    []string // substrings that must appear
		wantNot []string
	}{
		{name: "yaml default", format: "", want: []string{"name: Ireland", "population: 5"}, wantNot: []string{"["}},
		{name: "yaml explicit", format: "yaml", want: []string{"name: Ireland"}, wantNot: []string{"["}},
		{name: "json bare object", format: "json", want: []string{`"name": "Ireland"`}, wantNot: []string{"["}},
		{name: "ingr single row", format: "ingr", columns: []string{"$id", "name", "population"}, want: []string{"# INGR.io | select", "Ireland", "# 1 record"}, wantNot: []string{"["}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := writeSingleRecord(&buf, record, tt.format, tt.columns); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := buf.String()
			for _, s := range tt.want {
				if !strings.Contains(got, s) {
					t.Errorf("expected %q in output:\n%s", s, got)
				}
			}
			for _, s := range tt.wantNot {
				if strings.Contains(got, s) {
					t.Errorf("did not expect %q in output:\n%s", s, got)
				}
			}
		})
	}
}
