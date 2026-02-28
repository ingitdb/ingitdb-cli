# AI Skills

Reusable patterns and conventions discovered or established during development.
AI agents must apply these skills when the context matches.

---

## Functional Options Pattern

Use Go's functional options pattern for optional configuration that flows through a call pipeline.

### Generic building blocks

```go
// Option is a generic functional option that mutates a config struct T.
type Option[T any] func(*T)

// ApplyOptions applies each option in order to cfg.
func ApplyOptions[T any](cfg *T, opts ...Option[T]) {
    for _, opt := range opts {
        opt(cfg)
    }
}
```

Define a concrete options struct and typed alias per feature:

```go
type FooOptions struct {
    FeatureA bool
    FeatureB bool
}

type FooOption = Option[FooOptions]

func WithFeatureA() FooOption { return func(o *FooOptions) { o.FeatureA = true } }
func WithFeatureB() FooOption { return func(o *FooOptions) { o.FeatureB = true } }
```

### Rules for passing options through a pipeline

| Situation | How to accept options |
|---|---|
| Function **forwards** options downstream without reading them | `opts ...FooOption` (variadic) |
| **Exported** function that is the pipeline entry point | `opts ...FooOption` (variadic) |
| **Unexported** function where the options struct has **more than one field** and **every field drives logic** | `opts FooOptions` (plain value) |
| Only **one** option field is relevant to the function | extract and pass as a direct `bool` / scalar argument |

The plain-value form for unexported functions avoids scattering individual boolean parameters
across the signature while still being explicit about what each option does.

### Example — two-level pipeline

```go
// Exported: just forwards opts, does not read them.
func FormatBatch(format string, headers []string, records []Record, opts ...FooOption) ([]byte, error) {
    switch format {
    case "special":
        var cfg FooOptions
        ApplyOptions(&cfg, opts...)
        return formatSpecial(headers, records, cfg) // unexported: all fields used
    default:
        return formatDefault(headers, records)
    }
}

// Unexported: all FooOptions fields affect logic → plain value, compact signature.
func formatSpecial(headers []string, records []Record, opts FooOptions) ([]byte, error) {
    if opts.FeatureA { /* ... */ }
    if opts.FeatureB { /* ... */ }
    // ...
}
```

### Real usage in this project

`pkg/ingitdb/materializer/options.go` — `Option[T]`, `ApplyOptions`, `ExportOptions`, `WithHash()`, `WithRecordsDelimiter()`

- `formatExportBatch` (exported-ish, routes by format) — accepts `...ExportOption`; only the INGR branch reads them.
- `formatINGR` (unexported, both `IncludeHash` and `RecordsDelimiter` drive logic) — accepts `ExportOptions` as a plain value.
