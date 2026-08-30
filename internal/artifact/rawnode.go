package artifact

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// RawNode is the seam's typed conduit for a deferred sub-document: a
// frontmatter field whose keys are open (a registry dispatches them) but
// whose VALUES must still round-trip through this package's strict
// decode. It is a type alias, not a wrapper, so sibling packages can
// declare `map[string]artifact.RawNode` decode targets without importing
// gopkg.in/yaml.v3 themselves — TestYAMLImportSeam keeps the module's
// yaml handling inside this one subtree, and the constitution store's
// typed-payload dispatch (internal/policyartifact) is the first
// consumer.
//
// A RawNode captured from a document that already passed DecodeStrict
// has passed the dialect wall (no anchors, aliases, or custom tags
// anywhere in the document); EncodeRawNode then re-encodes it for a
// second DecodeStrict against the registered concrete type, which
// re-enforces KnownFields and duplicate-key rejection on the sub-tree.
type RawNode = yaml.Node

// EncodeRawNode re-encodes a captured sub-document node so a registry
// can strict-decode it into its concrete registered type.
func EncodeRawNode(n *RawNode) ([]byte, error) {
	data, err := yaml.Marshal(n)
	if err != nil {
		return nil, fmt.Errorf("artifact: re-encoding sub-document node: %w", err)
	}
	return data, nil
}

// RawNodeStringScalar reports the node's value when it is exactly a plain
// string scalar — the seam-owned shape test sibling packages use inside
// custom unmarshalers without importing gopkg.in/yaml.v3 or comparing raw
// node-kind constants themselves.
func RawNodeStringScalar(n *RawNode) (string, bool) {
	if n == nil || n.Kind != yaml.ScalarNode || n.Tag != "!!str" {
		return "", false
	}
	return n.Value, true
}

// RawNodeStringMapping projects a mapping node whose keys and values are
// all plain string scalars. ok=false reports a node that is not a mapping
// at all (the caller decides what other shapes mean); a mapping that
// breaks the string-only or unique-key contract is an error naming the
// offending key. Closed key sets and required keys remain the caller's
// grammar — this owns only the strict shape.
func RawNodeStringMapping(n *RawNode) (map[string]string, bool, error) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, false, nil
	}
	if len(n.Content)%2 != 0 {
		return nil, true, fmt.Errorf("artifact: malformed mapping node")
	}
	fields := make(map[string]string, len(n.Content)/2)
	for i := 0; i < len(n.Content); i += 2 {
		key, ok := RawNodeStringScalar(n.Content[i])
		if !ok {
			return nil, true, fmt.Errorf("artifact: mapping key is not a plain string")
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, true, fmt.Errorf("artifact: mapping key %q is duplicated", key)
		}
		value, ok := RawNodeStringScalar(n.Content[i+1])
		if !ok {
			return nil, true, fmt.Errorf("artifact: mapping value for %q is not a plain string", key)
		}
		fields[key] = value
	}
	return fields, true, nil
}
