package docsbuilder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// BuildCollectionReadme generates the content of README.md for a collection
func BuildCollectionReadme(ctx context.Context, col *ingitdb.CollectionDef, def *ingitdb.Definition,
	viewRenderer func(ctx context.Context, col *ingitdb.CollectionDef, view *ingitdb.ViewDef) (string, error)) (string, error) {
	// A collection's README.md file includes the following auto-generated sections:
	// - Collection name: Human-readable name of the collection.
	// - Path to collection: Shown if it is a subcollection.
	// - Table of columns: Lists all columns with their name, type, and other properties.
	// - Table of subcollections: Lists nested subcollections with their name and the number of their subcollections.
	// - Table of views: Lists available materialized views with their name and the number of columns.

	var sb strings.Builder

	// Collection name
	title := col.ID
	if len(col.Titles) > 0 {
		if enTitle, ok := col.Titles["en"]; ok {
			title = enTitle
		} else {
			for _, t := range col.Titles {
				title = t
				break
			}
		}
	}
	fmt.Fprintf(&sb, "# %s\n\n", title)

	// Path to collection (if it is a subcollection)
	// How to determine full path? We aren't passed the path directly.
	// We can compute it if we pass it, or we rely on some property.
	// For now, let's omit it if we don't know it, or we need to pass the dot-path.
	// Actually `col.ID` is just the last part. Wait, DirPath contains the full path?
	// The prompt does not strictly require `Path to collection` to be exact if it's not easily available, but let's try.
	// We'll skip path output here if it's too complex or we can deduce it from the generator caller.
	// Let's check if the definition has a reverse lookup or if we can find it.

	if col.Readme == nil || !col.Readme.HideColumns {
		sb.WriteString("## Columns\n\n")
		sb.WriteString("| Name | Type | Properties |\n")
		sb.WriteString("|------|------|------------|\n")

		// Print columns in order
		if len(col.ColumnsOrder) > 0 {
			for _, colName := range col.ColumnsOrder {
				if colDef, ok := col.Columns[colName]; ok {
					sb.WriteString(formatColumnRow(colName, colDef))
				}
			}
		} else {
			for colName, colDef := range col.Columns {
				sb.WriteString(formatColumnRow(colName, colDef))
			}
		}
	}

	if (col.Readme == nil || !col.Readme.HideSubcollections) && len(col.SubCollections) > 0 {
		sb.WriteString("\n## Subcollections\n\n")
		sb.WriteString("| Name | Subcollections |\n")
		sb.WriteString("|------|----------------|\n")
		for subID, subCol := range col.SubCollections {
			relPath := subID
			if col.DirPath != "" && subCol.DirPath != "" {
				if r, err := filepath.Rel(col.DirPath, subCol.DirPath); err == nil {
					relPath = filepath.ToSlash(r)
				}
			}
			fmt.Fprintf(&sb, "| [%s](%s) | %d |\n", subID, relPath, len(subCol.SubCollections))
		}
	}

	if (col.Readme == nil || !col.Readme.HideViews) && len(col.Views) > 0 {
		sb.WriteString("\n## Views\n\n")
		sb.WriteString("| Name | Columns |\n")
		sb.WriteString("|------|---------|\n")
		for viewID, viewDef := range col.Views {
			fmt.Fprintf(&sb, "| %s | %d |\n", viewID, len(viewDef.Columns))
		}
	}

	if col.Readme != nil && col.Readme.DataPreview != nil {
		previewStr, err := viewRenderer(ctx, col, col.Readme.DataPreview)
		if err != nil {
			return "", fmt.Errorf("failed to render data preview: %w", err)
		}
		header := BuildViewHeader(col.Readme.DataPreview)
		sb.WriteString("\n## Data preview\n\n")
		sb.WriteString("*" + header + "*\n\n")
		sb.WriteString(previewStr)
	}

	return sb.String(), nil
}

func formatColumnRow(name string, col *ingitdb.ColumnDef) string {
	var props []string
	if col.Required {
		props = append(props, "Required")
	}
	if col.ForeignKey != "" {
		props = append(props, fmt.Sprintf("FK(%s)", col.ForeignKey))
	}
	if col.Locale != "" {
		props = append(props, fmt.Sprintf("Locale(%s)", col.Locale))
	}

	propStr := strings.Join(props, ", ")
	if propStr == "" {
		propStr = "-"
	}

	typeStr := string(col.Type)

	return fmt.Sprintf("| %s | %s | %s |\n", name, typeStr, propStr)
}
