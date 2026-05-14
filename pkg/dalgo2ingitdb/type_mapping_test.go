package dalgo2ingitdb

import (
	"testing"

	"github.com/dal-go/dalgo/dbschema"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestIngitdbTypeToDBSchema(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      ingitdb.ColumnType
		want    dbschema.Type
		wantErr bool
	}{
		{ingitdb.ColumnTypeString, dbschema.String, false},
		{ingitdb.ColumnTypeInt, dbschema.Int, false},
		{ingitdb.ColumnTypeFloat, dbschema.Float, false},
		{ingitdb.ColumnTypeBool, dbschema.Bool, false},
		{ingitdb.ColumnTypeDate, dbschema.Time, false},
		{ingitdb.ColumnTypeTime, dbschema.Time, false},
		{ingitdb.ColumnTypeDateTime, dbschema.Time, false},
		{ingitdb.ColumnTypeAny, dbschema.String, false},
		{ingitdb.ColumnTypeL10N, dbschema.String, false},
		{ingitdb.ColumnType("map[string]string"), dbschema.String, false},
		{ingitdb.ColumnType("map[int]string"), dbschema.String, false},
		{"", dbschema.Null, true},
		{ingitdb.ColumnType("unknown"), dbschema.Null, true},
	}
	for _, tc := range cases {
		got, err := ingitdbTypeToDBSchema(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ingitdbTypeToDBSchema(%q): want error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ingitdbTypeToDBSchema(%q): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ingitdbTypeToDBSchema(%q): got %s, want %s", tc.in, got, tc.want)
		}
	}
}

func TestDBSchemaTypeToIngitdb(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in      dbschema.Type
		want    ingitdb.ColumnType
		wantErr bool
	}{
		{dbschema.Bool, ingitdb.ColumnTypeBool, false},
		{dbschema.Int, ingitdb.ColumnTypeInt, false},
		{dbschema.Float, ingitdb.ColumnTypeFloat, false},
		{dbschema.String, ingitdb.ColumnTypeString, false},
		{dbschema.Time, ingitdb.ColumnTypeDateTime, false},
		{dbschema.Null, "", true},
		// Decimal and Bytes now map lossily to existing column types so
		// cross-engine copies (e.g. SQLite NUMERIC(p,s) → inGitDB) don't
		// fail at schema-creation time. See doc comment on
		// dbschemaTypeToIngitdb for the precision-loss caveat.
		{dbschema.Bytes, ingitdb.ColumnTypeString, false},
		{dbschema.Decimal, ingitdb.ColumnTypeFloat, false},
	}
	for _, tc := range cases {
		got, err := dbschemaTypeToIngitdb(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("dbschemaTypeToIngitdb(%s): want error, got nil", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("dbschemaTypeToIngitdb(%s): unexpected error: %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("dbschemaTypeToIngitdb(%s): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTypeMappingRoundTrip(t *testing.T) {
	t.Parallel()
	roundTrippable := []ingitdb.ColumnType{
		ingitdb.ColumnTypeString,
		ingitdb.ColumnTypeInt,
		ingitdb.ColumnTypeFloat,
		ingitdb.ColumnTypeBool,
		ingitdb.ColumnTypeDateTime,
	}
	for _, in := range roundTrippable {
		mid, err := ingitdbTypeToDBSchema(in)
		if err != nil {
			t.Errorf("forward %q: unexpected error: %v", in, err)
			continue
		}
		back, err := dbschemaTypeToIngitdb(mid)
		if err != nil {
			t.Errorf("backward %s: unexpected error: %v", mid, err)
			continue
		}
		if back != in {
			t.Errorf("round-trip %q: got %q (via %s)", in, back, mid)
		}
	}

	lossyToDateTime := []ingitdb.ColumnType{ingitdb.ColumnTypeDate, ingitdb.ColumnTypeTime}
	for _, in := range lossyToDateTime {
		mid, _ := ingitdbTypeToDBSchema(in)
		back, _ := dbschemaTypeToIngitdb(mid)
		if back != ingitdb.ColumnTypeDateTime {
			t.Errorf("lossy round-trip %q: got %q, want %q", in, back, ingitdb.ColumnTypeDateTime)
		}
	}
}
