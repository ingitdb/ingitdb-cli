package commands

import (
	"strings"
	"testing"
)

func TestResolveFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		raw       string
		engine    string
		want      string
		wantErr   bool
		errSubstr string
	}{
		{name: "default_yaml_for_empty", raw: "", engine: "ingitdb", want: "yaml"},
		{name: "yaml_explicit", raw: "yaml", engine: "ingitdb", want: "yaml"},
		{name: "json", raw: "json", engine: "ingitdb", want: "json"},
		{name: "native_on_ingitdb_is_yaml", raw: "native", engine: "ingitdb", want: "yaml"},
		{name: "native_on_sql_engine_is_sql", raw: "native", engine: "sqlite", want: "sql"},
		{name: "sql_on_ingitdb_errors", raw: "sql", engine: "ingitdb", wantErr: true,
			errSubstr: `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`},
		{name: "SQL_case_insensitive_on_ingitdb_errors", raw: "SQL", engine: "ingitdb", wantErr: true,
			errSubstr: `engine "ingitdb" native format is "yaml"; use --format=yaml or --format=native`},
		{name: "sql_on_sql_engine_passes", raw: "sql", engine: "sqlite", want: "sql"},
		{name: "unknown_value_lists_options", raw: "xml", engine: "ingitdb", wantErr: true,
			errSubstr: "valid values: yaml, json, native, sql"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveFormat(tc.raw, tc.engine)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error containing %q, got nil; canonical=%q", tc.errSubstr, got)
				}
				if !strings.Contains(err.Error(), tc.errSubstr) {
					t.Errorf("error %q missing substring %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q; want %q", got, tc.want)
			}
		})
	}
}
