package commands

// specscore: feature/cli/describe

import (
	"path"
	"sort"

	"gopkg.in/yaml.v3"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// collectionOutputCtx is the per-invocation context the payload builder
// needs that isn't on CollectionDef itself: the collection's location
// inside the database (forward-slash, relative to db root) and the
// names of its views and subcollections (passed in pre-discovered so
// the builder stays pure).
type collectionOutputCtx struct {
	relPath            string
	viewNames          []string
	subcollectionNames []string
}

// viewOutputCtx is the equivalent for a view: the id of the owning
// root collection and the view file's path relative to db root.
type viewOutputCtx struct {
	owningCollection string
	relPath          string
}

// buildCollectionPayload assembles the {definition, _meta} document
// for a collection as a *yaml.Node. The caller chooses the wire
// format (yaml.Marshal or json.Marshal of the same node via a
// node→interface conversion).
func buildCollectionPayload(col *ingitdb.CollectionDef, ctx collectionOutputCtx) (*yaml.Node, error) {
	defNode := &yaml.Node{}
	if err := defNode.Encode(col); err != nil {
		return nil, err
	}
	dataPath := ctx.relPath
	if col.DataDir != "" {
		dataPath = path.Clean(path.Join(ctx.relPath, col.DataDir))
	}
	meta := orderedMap(
		kv("id", col.ID),
		kv("kind", "collection"),
		kv("definition_path", ctx.relPath),
		kv("data_path", dataPath),
		kvList("views", sortedCopy(ctx.viewNames)),
		kvList("subcollections", sortedCopy(ctx.subcollectionNames)),
	)
	return docNode(defNode, meta), nil
}

// buildViewPayload assembles the {definition, _meta} document for a
// view.
func buildViewPayload(view *ingitdb.ViewDef, ctx viewOutputCtx) (*yaml.Node, error) {
	defNode := &yaml.Node{}
	if err := defNode.Encode(view); err != nil {
		return nil, err
	}
	meta := orderedMap(
		kv("id", view.ID),
		kv("kind", "view"),
		kv("collection", ctx.owningCollection),
		kv("definition_path", ctx.relPath),
	)
	return docNode(defNode, meta), nil
}

// docNode wraps two child nodes under top-level keys "definition" and
// "_meta", in that order.
func docNode(def, meta *yaml.Node) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Value: "definition"}, def,
			{Kind: yaml.ScalarNode, Value: "_meta"}, meta,
		},
	}
}

// orderedMap builds a MappingNode preserving caller-given order.
func orderedMap(pairs ...[2]*yaml.Node) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range pairs {
		m.Content = append(m.Content, p[0], p[1])
	}
	return m
}

func kv(key, value string) [2]*yaml.Node {
	return [2]*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key},
		{Kind: yaml.ScalarNode, Value: value},
	}
}

func kvList(key string, values []string) [2]*yaml.Node {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
	}
	return [2]*yaml.Node{
		{Kind: yaml.ScalarNode, Value: key},
		seq,
	}
}

func sortedCopy(in []string) []string {
	if len(in) == 0 {
		return []string{}
	}
	out := make([]string, len(in))
	copy(out, in)
	sort.Strings(out)
	return out
}
