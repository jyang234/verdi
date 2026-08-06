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
