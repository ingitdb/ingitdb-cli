package sqlflags

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestParseMinAffected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "one", input: "1", want: 1},
		{name: "ten", input: "10", want: 10},
		{name: "large", input: "1000000", want: 1000000},
		{name: "zero rejected", input: "0", wantErr: true},
		{name: "negative rejected", input: "-1", wantErr: true},
		{name: "non-numeric rejected", input: "foo", wantErr: true},
		{name: "float rejected", input: "1.5", wantErr: true},
		{name: "empty rejected", input: "", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMinAffected(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("want %d, got %d", tt.want, got)
			}
		})
	}
}

func TestMinAffectedFromCmd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		args         []string
		wantN        int
		wantSupplied bool
		wantErr      bool
	}{
		{name: "not supplied", args: []string{}, wantN: 0, wantSupplied: false},
		{name: "valid 1", args: []string{"--min-affected=1"}, wantN: 1, wantSupplied: true},
		{name: "valid 100", args: []string{"--min-affected=100"}, wantN: 100, wantSupplied: true},
		{name: "zero rejected", args: []string{"--min-affected=0"}, wantErr: true},
		{name: "negative rejected", args: []string{"--min-affected=-1"}, wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd := &cobra.Command{Use: "test"}
			RegisterMinAffectedFlag(cmd)
			if err := cmd.ParseFlags(tt.args); err != nil {
				t.Fatalf("parse flags: %v", err)
			}
			n, supplied, err := MinAffectedFromCmd(cmd)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if n != tt.wantN {
				t.Errorf("n: want %d, got %d", tt.wantN, n)
			}
			if supplied != tt.wantSupplied {
				t.Errorf("supplied: want %v, got %v", tt.wantSupplied, supplied)
			}
		})
	}
}
