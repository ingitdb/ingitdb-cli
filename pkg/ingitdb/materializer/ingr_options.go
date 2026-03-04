package materializer

import "github.com/ingitdb/ingitdb-cli/pkg/ingitdb"

// Option is a generic functional option that mutates a config struct T.
// Use with ApplyOptions to build a config from a variadic list of options.
//
// Pipeline passing rules:
//   - Exported functions that only forward options downstream: accept ...Option[T] (variadic).
//   - Unexported functions where the options struct has more than one field and every field
//     drives logic: accept T as a plain value. This avoids scattering individual parameters
//     and keeps the signature compact.
type Option[T any] func(*T)

// ApplyOptions applies each option in order to cfg.
func ApplyOptions[T any](cfg *T, opts ...Option[T]) {
	for _, opt := range opts {
		opt(cfg)
	}
}

// ExportOptions holds optional settings that modify INGR serialisation behaviour.
// Non-INGR formats ignore all fields.
type ExportOptions struct {
	// IncludeHash appends a "# sha256:{hex}" line to the INGR footer.
	IncludeHash bool
	// RecordsDelimiter writes a bare "#" line after each record.
	RecordsDelimiter bool
	// ColumnTypes maps column names to their types for inclusion in the INGR header.
	// When set, each header column is written as "name:type" (e.g. "area_km2:int").
	// The "$ID" column key maps to the record key pseudo-column.
	ColumnTypes map[string]ingitdb.ColumnType
}

// ExportOption is a functional option for ExportOptions.
type ExportOption = Option[ExportOptions]

// WithHash enables the sha256 hash footer line in INGR output.
func WithHash() ExportOption {
	return func(o *ExportOptions) {
		o.IncludeHash = true
	}
}

// WithRecordsDelimiter enables a bare "#" delimiter line after each record in INGR output.
func WithRecordsDelimiter() ExportOption {
	return func(o *ExportOptions) {
		o.RecordsDelimiter = true
	}
}

// WithColumnTypes populates ColumnTypes from a CollectionDef so that the INGR header
// includes type annotations for every column (e.g. "area_km2:int", "$ID:string").
func WithColumnTypes(col *ingitdb.CollectionDef) ExportOption {
	return func(o *ExportOptions) {
		o.ColumnTypes = make(map[string]ingitdb.ColumnType, len(col.Columns)+1)
		if def, ok := col.Columns["$ID"]; ok {
			o.ColumnTypes["$ID"] = def.Type
		} else {
			o.ColumnTypes["$ID"] = ingitdb.ColumnTypeString
		}
		for name, def := range col.Columns {
			o.ColumnTypes[name] = def.Type
		}
	}
}
