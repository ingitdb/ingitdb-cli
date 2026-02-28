package materializer

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
