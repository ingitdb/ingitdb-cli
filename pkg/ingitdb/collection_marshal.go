package ingitdb

import (
	"sort"

	"gopkg.in/yaml.v3"
)

// MarshalYAML emits a CollectionDef as a YAML mapping with deterministic
// column ordering: columns named in ColumnsOrder appear in that order;
// any remaining columns follow in alphabetical order. The other fields
// keep their struct-declaration order.
//
// This makes any yaml.Marshal of a CollectionDef diff-stable across
// runs and across machines.
func (c *CollectionDef) MarshalYAML() (interface{}, error) {
	if c == nil {
		return nil, nil
	}
	root := &yaml.Node{Kind: yaml.MappingNode}

	addScalar := func(key, value string) {
		if value == "" {
			return
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Value: value},
		)
	}
	addNode := func(key string, node *yaml.Node) {
		if node == nil {
			return
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: key},
			node,
		)
	}

	if len(c.Titles) > 0 {
		titlesNode := &yaml.Node{}
		_ = titlesNode.Encode(c.Titles)
		addNode("titles", titlesNode)
	}
	if c.RecordFile != nil {
		rfNode := &yaml.Node{}
		if err := rfNode.Encode(c.RecordFile); err != nil {
			return nil, err
		}
		addNode("record_file", rfNode)
	}
	addScalar("data_dir", c.DataDir)

	columnsNode := orderedColumnsNode(c.Columns, c.ColumnsOrder)
	if columnsNode != nil {
		addNode("columns", columnsNode)
	}

	if len(c.ColumnsOrder) > 0 {
		coNode := &yaml.Node{}
		_ = coNode.Encode(c.ColumnsOrder)
		addNode("columns_order", coNode)
	}
	if len(c.PrimaryKey) > 0 {
		pkNode := &yaml.Node{}
		_ = pkNode.Encode(c.PrimaryKey)
		addNode("primary_key", pkNode)
	}
	if c.DefaultView != nil {
		dvNode := &yaml.Node{}
		if err := dvNode.Encode(c.DefaultView); err != nil {
			return nil, err
		}
		addNode("default_view", dvNode)
	}
	if c.Readme != nil {
		rNode := &yaml.Node{}
		if err := rNode.Encode(c.Readme); err != nil {
			return nil, err
		}
		addNode("readme", rNode)
	}
	return root, nil
}

// orderedColumnsNode returns a MappingNode containing every entry from
// columns, with keys ordered by columnsOrder followed by alphabetical
// for anything not listed. Returns nil if columns is empty.
func orderedColumnsNode(columns map[string]*ColumnDef, columnsOrder []string) *yaml.Node {
	if len(columns) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(columns))
	keys := make([]string, 0, len(columns))
	for _, k := range columnsOrder {
		if _, ok := columns[k]; ok && !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	tail := make([]string, 0, len(columns))
	for k := range columns {
		if !seen[k] {
			tail = append(tail, k)
		}
	}
	sort.Strings(tail)
	keys = append(keys, tail...)

	node := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		colNode := &yaml.Node{}
		_ = colNode.Encode(columns[k])
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k},
			colNode,
		)
	}
	return node
}
