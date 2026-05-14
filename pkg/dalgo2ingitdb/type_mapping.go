package dalgo2ingitdb

// specscore: feature/dalgo2ingitdb-dbschema-ddl-coverage

import (
	"fmt"
	"strings"

	"github.com/dal-go/dalgo/dbschema"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// ingitdbTypeToDBSchema converts an ingitdb column type to its portable
// dbschema.Type equivalent. Returns an error if the type cannot be
// represented.
//
// The mapping is many-to-one for time types (date, time, datetime all map
// to dbschema.Time) and for non-scalar types (any, map[...]string all map
// to dbschema.String). Round-tripping through dbschemaTypeToIngitdb is
// therefore lossy: a date column read back as dbschema.Time then written
// back to ingitdb becomes datetime.
func ingitdbTypeToDBSchema(t ingitdb.ColumnType) (dbschema.Type, error) {
	switch t {
	case ingitdb.ColumnTypeString:
		return dbschema.String, nil
	case ingitdb.ColumnTypeInt:
		return dbschema.Int, nil
	case ingitdb.ColumnTypeFloat:
		return dbschema.Float, nil
	case ingitdb.ColumnTypeBool:
		return dbschema.Bool, nil
	case ingitdb.ColumnTypeDate, ingitdb.ColumnTypeTime, ingitdb.ColumnTypeDateTime:
		return dbschema.Time, nil
	case ingitdb.ColumnTypeAny, ingitdb.ColumnTypeL10N:
		return dbschema.String, nil
	case "":
		return dbschema.Null, fmt.Errorf("empty ingitdb column type")
	}
	if strings.HasPrefix(string(t), "map[") {
		return dbschema.String, nil
	}
	return dbschema.Null, fmt.Errorf("unsupported ingitdb column type: %q", t)
}

// dbschemaTypeToIngitdb converts a portable dbschema.Type to its ingitdb
// column-type equivalent. Returns an error for types ingitdb cannot
// represent (dbschema.Null, dbschema.Bytes, dbschema.Decimal).
//
// dbschema.Time always maps to ingitdb.ColumnTypeDateTime (the most
// general time type). Callers that need date- or time-of-day-only
// granularity must override the type manually after a round-trip.
func dbschemaTypeToIngitdb(t dbschema.Type) (ingitdb.ColumnType, error) {
	switch t {
	case dbschema.Bool:
		return ingitdb.ColumnTypeBool, nil
	case dbschema.Int:
		return ingitdb.ColumnTypeInt, nil
	case dbschema.Float:
		return ingitdb.ColumnTypeFloat, nil
	case dbschema.String:
		return ingitdb.ColumnTypeString, nil
	case dbschema.Time:
		return ingitdb.ColumnTypeDateTime, nil
	case dbschema.Null:
		return "", fmt.Errorf("dbschema.Null cannot be mapped to an ingitdb column type")
	}
	return "", fmt.Errorf("unsupported dbschema.Type: %s", t)
}
