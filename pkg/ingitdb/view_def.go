package ingitdb

import (
	"fmt"
	"strings"
)

type ViewDef struct {
	ID      string            `yaml:"-"`
	Titles  map[string]string `yaml:"titles,omitempty"`
	OrderBy string            `yaml:"order_by,omitempty"`

	// Formats TODO: Needs definition
	Formats []string `yaml:"formats,omitempty"`

	Columns []string `yaml:"columns,omitempty"`

	// How many records to include; 0 means all
	Top int `yaml:"top,omitempty"`

	// Where holds filtering condition
	Where string `yaml:"where,omitempty"`

	// Template path relative to the collection directory.
	/*
		Build in templates:
		  - md-table - renders a Markdown table
		  - md-list - renders a Markdown list
		  - JSON - renders JSON
		  - YAML - renders YAML
	*/
	Template string `yaml:"template,omitempty"`

	// Output file name relative to the collection directory.
	FileName string `yaml:"file_name,omitempty"`

	// RecordsVarName provides a custom Template variable name for the records slice. The default is "records".
	RecordsVarName string `yaml:"records_var_name,omitempty"`

	Format       string `yaml:"format,omitempty"`
	MaxBatchSize int    `yaml:"max_batch_size,omitempty"`
	IsDefault    bool   `yaml:"-" json:"-"`

	// IncludeHash controls whether a sha256 hash line is appended to INGR footer.
	IncludeHash bool `yaml:"include_hash,omitempty"`
}

// Validate checks the view definition for consistency.
func (v *ViewDef) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("missing 'id' in view definition")
	}

	if v.Format != "" {
		// Validate format is one of the allowed values (case-insensitive)
		validFormats := map[string]bool{
			"ingr":  true,
			"tsv":   true,
			"csv":   true,
			"json":  true,
			"jsonl": true,
			"yaml":  true,
		}
		formatLower := strings.ToLower(v.Format)
		if !validFormats[formatLower] {
			return fmt.Errorf("invalid 'format' value: %s, must be one of: ingr, tsv, csv, json, jsonl, yaml", v.Format)
		}
	}

	if v.MaxBatchSize < 0 {
		return fmt.Errorf("'max_batch_size' must be >= 0, got %d", v.MaxBatchSize)
	}

	return nil
}
