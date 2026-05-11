// Package markdown parses and serializes inGitDB Markdown record files —
// YAML frontmatter delimited by "---" lines followed by a body. The body is
// preserved byte-for-byte across round-trips; the writer canonicalizes
// frontmatter key order to columns_order with alphabetical fallback for
// columns absent from columns_order, as required by the markdown-records
// feature spec.
package markdown

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// delimiter is the line that opens and closes the frontmatter block.
const delimiter = "---"

// Parse splits a Markdown record into frontmatter and body.
//
// When the file's first line is exactly "---" and a matching closing "---"
// appears on its own line later, the text between is parsed as a YAML
// mapping and returned as frontmatter; everything after the closing
// delimiter (including the newline that terminates the closing line) is
// returned as body, byte-for-byte.
//
// When the file does not begin with "---", frontmatter is nil and the entire
// content is returned as body.
//
// A leading "---" with no matching closing delimiter is a malformed
// frontmatter block and returns an error.
func Parse(content []byte) (frontmatter map[string]any, body []byte, err error) {
	open, openLen, hasOpen := findDelimiter(content, 0)
	if !hasOpen || open != 0 {
		return nil, content, nil
	}
	afterOpen := openLen
	close, closeLen, hasClose := findDelimiter(content, afterOpen)
	if !hasClose {
		return nil, nil, fmt.Errorf("markdown: opening %q delimiter has no matching closing %q delimiter", delimiter, delimiter)
	}
	fmBytes := content[afterOpen:close]
	if len(fmBytes) > 0 {
		err = yaml.Unmarshal(fmBytes, &frontmatter)
		if err != nil {
			return nil, nil, fmt.Errorf("markdown: invalid YAML frontmatter: %w", err)
		}
	}
	if frontmatter == nil {
		// Document with `---\n---\n` (no keys) is valid; expose empty map.
		frontmatter = map[string]any{}
	}
	body = content[close+closeLen:]
	return frontmatter, body, nil
}

// Serialize emits a Markdown record: an opening "---" line, the frontmatter
// keys in canonical order (columns_order first, then alphabetical for any
// keys not in columns_order), a closing "---" line, and the body bytes
// verbatim.
//
// When frontmatter is empty (or nil), the output still includes the two
// "---" lines so the file remains a valid frontmatter document.
func Serialize(frontmatter map[string]any, columnsOrder []string, body []byte) ([]byte, error) {
	ordered := orderKeys(frontmatter, columnsOrder)
	var buf bytes.Buffer
	buf.WriteString(delimiter)
	buf.WriteByte('\n')
	if len(ordered) > 0 {
		node, err := buildMappingNode(frontmatter, ordered)
		if err != nil {
			return nil, fmt.Errorf("markdown: build frontmatter node: %w", err)
		}
		fmBytes, marshalErr := yaml.Marshal(node)
		if marshalErr != nil {
			return nil, fmt.Errorf("markdown: marshal frontmatter: %w", marshalErr)
		}
		buf.Write(fmBytes)
	}
	buf.WriteString(delimiter)
	buf.WriteByte('\n')
	buf.Write(body)
	return buf.Bytes(), nil
}

// findDelimiter returns the byte offset of the next "---" line starting at or
// after `from`, the length of that line (including its trailing newline), and
// whether one was found. A delimiter line is a line whose only content
// before the newline (or EOF) is exactly "---".
func findDelimiter(content []byte, from int) (offset, lineLen int, found bool) {
	for i := from; i < len(content); {
		end := i
		for end < len(content) && content[end] != '\n' {
			end++
		}
		line := content[i:end]
		if isDelimiterLine(line) {
			lineEnd := end
			if lineEnd < len(content) && content[lineEnd] == '\n' {
				lineEnd++
			}
			return i, lineEnd - i, true
		}
		if end == len(content) {
			return 0, 0, false
		}
		i = end + 1
	}
	return 0, 0, false
}

// isDelimiterLine reports whether the (newline-stripped) line bytes match
// the frontmatter delimiter exactly. Trailing carriage return is tolerated
// for CRLF files.
func isDelimiterLine(line []byte) bool {
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	return bytes.Equal(line, []byte(delimiter))
}

// orderKeys returns the keys of `frontmatter` in canonical order:
//  1. Keys named in `columnsOrder`, in that order, if present in frontmatter.
//  2. Remaining keys in alphabetical order.
//
// Keys named in columnsOrder but absent from frontmatter are skipped (we
// don't synthesize null entries).
func orderKeys(frontmatter map[string]any, columnsOrder []string) []string {
	if len(frontmatter) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(frontmatter))
	ordered := make([]string, 0, len(frontmatter))
	for _, k := range columnsOrder {
		if _, ok := frontmatter[k]; !ok {
			continue
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		ordered = append(ordered, k)
	}
	var rest []string
	for k := range frontmatter {
		if seen[k] {
			continue
		}
		rest = append(rest, k)
	}
	sort.Strings(rest)
	ordered = append(ordered, rest...)
	return ordered
}

// buildMappingNode constructs a yaml.Node representing a mapping whose keys
// appear in the order given by `keys`. This is the only reliable way to
// force key order with gopkg.in/yaml.v3: marshaling a map[string]any does
// not preserve order.
func buildMappingNode(frontmatter map[string]any, keys []string) (*yaml.Node, error) {
	root := &yaml.Node{Kind: yaml.MappingNode}
	for _, k := range keys {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k}
		valNode := &yaml.Node{}
		err := valNode.Encode(frontmatter[k])
		if err != nil {
			return nil, fmt.Errorf("encode value for key %q: %w", k, err)
		}
		root.Content = append(root.Content, keyNode, valNode)
	}
	return root, nil
}
