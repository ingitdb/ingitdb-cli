package ingitdb

import "fmt"

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
}

// Validate checks the view definition for consistency.
func (v *ViewDef) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("missing 'id' in view definition")
	}
	return nil
}
