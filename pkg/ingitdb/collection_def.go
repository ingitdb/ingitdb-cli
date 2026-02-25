package ingitdb

import (
	"errors"
	"fmt"
)

type CollectionDef struct {
	ID           string                `json:"-"` // Taken from dir name
	DirPath      string                `yaml:"-" json:"-"`
	Titles       map[string]string     `yaml:"titles,omitempty"`
	RecordFile   *RecordFileDef        `yaml:"record_file"`
	DataDir      string                `yaml:"data_dir,omitempty"`
	Columns      map[string]*ColumnDef `yaml:"columns"`
	ColumnsOrder []string              `yaml:"columns_order,omitempty"`
	DefaultView  string                `yaml:"default_view,omitempty"`
	// SubCollections are not part of the collection definition file,
	// they are stored in the "subcollections" subdirectory as directories,
	// each containing their own .collection/definition.yaml.
	SubCollections map[string]*CollectionDef `yaml:"-" json:"-"`
	// Views are not part of the collection definition file,
	// they are stored in the "views" subdirectory.
	Views map[string]*ViewDef `yaml:"-" json:"-"`

	Readme *CollectionReadmeDef `yaml:"readme,omitempty" json:"readme,omitempty"`
}

type CollectionReadmeDef struct {
	HideColumns        bool     `yaml:"hide_columns,omitempty" json:"hide_columns,omitempty"`
	HideSubcollections bool     `yaml:"hide_subcollections,omitempty" json:"hide_subcollections,omitempty"`
	HideViews          bool     `yaml:"hide_views,omitempty" json:"hide_views,omitempty"`
	HideTriggers       bool     `yaml:"hide_triggers,omitempty" json:"hide_triggers,omitempty"`
	DataPreview        *ViewDef `yaml:"data_preview,omitempty" json:"data_preview,omitempty"`
}

func (r *CollectionReadmeDef) Validate() error {
	if r.DataPreview != nil {
		if r.DataPreview.Template == "" {
			r.DataPreview.Template = "md-table"
		}
		if err := r.DataPreview.Validate(); err != nil {
			return fmt.Errorf("invalid data_preview: %w", err)
		}
	}
	return nil
}

func (v *CollectionDef) Validate() error {
	if v.ID == "" {
		return fmt.Errorf("missing 'id' in collection definition")
	}
	var allErrors []error
	if len(v.Columns) == 0 {
		return fmt.Errorf("missing 'columns' in collection definition")
	}
	for id, col := range v.Columns {
		if err := col.Validate(); err != nil {
			return fmt.Errorf("invalid column '%s': %w", id, err)
		}
	}
	for i, colName := range v.ColumnsOrder {
		if _, ok := v.Columns[colName]; !ok {
			return fmt.Errorf("columns_order[%d] references unspecified column: %s", i, colName)
		}
		for j, prevCol := range v.ColumnsOrder[:i] {
			if prevCol == colName {
				return fmt.Errorf("duplicate value in columns_order at indexes %d and %d: %s", j, i, colName)
			}
		}
	}
	if v.RecordFile == nil {
		return fmt.Errorf("missing 'record_file' in collection definition")
	}
	if err := v.RecordFile.Validate(); err != nil {
		return fmt.Errorf("invalid record_file definition: %w", err)
	}
	if v.SubCollections != nil {
		for id, subColDef := range v.SubCollections {
			if err := subColDef.Validate(); err != nil {
				allErrors = append(allErrors, fmt.Errorf("invalid subcollection '%s': %w", id, err))
			}
		}
	}
	if v.Views != nil {
		for id, viewDef := range v.Views {
			if err := viewDef.Validate(); err != nil {
				allErrors = append(allErrors, fmt.Errorf("invalid view '%s': %w", id, err))
			}
		}
	}
	if len(allErrors) > 0 {
		return fmt.Errorf("%d errors: %w", len(allErrors), errors.Join(allErrors...))
	}

	if v.Readme != nil {
		if err := v.Readme.Validate(); err != nil {
			return fmt.Errorf("invalid readme: %w", err)
		}
	}

	return nil
}
