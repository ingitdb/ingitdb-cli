package docsbuilder

import (
	"testing"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

func TestBuildViewHeader(t *testing.T) {
	tests := []struct {
		name     string
		view     *ingitdb.ViewDef
		expected string
	}{
		{
			name:     "all records",
			view:     &ingitdb.ViewDef{},
			expected: "All records",
		},
		{
			name:     "all records ordered by",
			view:     &ingitdb.ViewDef{OrderBy: "{order_by}"},
			expected: "All records ordered by {order_by}",
		},
		{
			name:     "top n records",
			view:     &ingitdb.ViewDef{Top: 10},
			expected: "Top 10 records",
		},
		{
			name:     "top n records ordered by",
			view:     &ingitdb.ViewDef{Top: 10, OrderBy: "{order_by}"},
			expected: "Top 10 records ordered by {order_by}",
		},
		{
			name:     "top n records where",
			view:     &ingitdb.ViewDef{Top: 10, Where: "{condition}"},
			expected: "Top 10 records where {condition}",
		},
		{
			name:     "top n records where ordered by",
			view:     &ingitdb.ViewDef{Top: 10, Where: "{condition}", OrderBy: "{order_by}"},
			expected: "Top 10 records where {condition} ordered by {order_by}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := BuildViewHeader(tt.view)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}
