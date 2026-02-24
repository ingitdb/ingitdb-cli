package ingitdb

import "fmt"

type ViewDef struct {
	ID      string            `yaml:"-"`
	Titles  map[string]string `yaml:"titles,omitempty"`
	OrderBy string            `yaml:"order_by,omitempty"`
	Formats []string          `yaml:"formats,omitempty"`
	Columns []string          `yaml:"columns,omitempty"`
	// How many records to include; 0 means all
	Top int `yaml:"top,omitempty"`
	// Template path relative to the collection directory.
	Template string `yaml:"template,omitempty"`
	// Output file name relative to the collection directory.
	FileName string `yaml:"file_name,omitempty"`
	// Template variable name for the records slice.
	RecordsVarName string `yaml:"records_var_name,omitempty"`
}

// Validate checks the view definition for consistency.
func (v *ViewDef) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("missing 'id' in view definition")
	}
	return nil
}
