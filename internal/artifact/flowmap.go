package artifact

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FlowmapSummary is the deliberately partial view of an upstream-owned
// .flowmap.yaml this package reads (PLAN.md Phase 3, service discovery
// row): `service` (the service name) and the names of every entry in
// `obligations:`. Every other upstream field — classify, obligations'
// acquire/release/require/before/fromCallers, whatever upstream adds next
// — is read by upstream's own tooling, never by verdi.
type FlowmapSummary struct {
	// Service is the `service:` key's value, or "" if the key is absent —
	// callers substitute the directory name in that case, per upstream's
	// own default-naming behavior (this package does not know the
	// directory name, so it cannot default here).
	Service string
	// Obligations lists `obligations[].name`, in file order.
	Obligations []string
}

// DecodeFlowmapLoose is the documented exception to this package's
// strict-decode discipline (CLAUDE.md: "Strict decode everywhere"; 02
// §Common frontmatter's dialect rule). `.flowmap.yaml` is an
// **upstream-owned** file — verdi does not control its schema, and
// verdi-go strict-decodes it against a schema verdi has no visibility
// into (PLAN.md §3 "AC bindings" row: "upstream has no such field and
// strict-decodes .flowmap.yaml"). Strict-decoding it here against a
// verdi-authored struct would make every future upstream field addition a
// verdi breakage, which is backwards: upstream owns this file, verdi is a
// guest reader.
//
// So this function does the opposite of DecodeStrict: it walks the parsed
// yaml.Node tree by hand, reads only the two top-level keys verdi actually
// consumes (`service`, `obligations[].name`), and silently ignores every
// other key, known or unknown. The one thing it still enforces is the
// restricted YAML dialect (no anchors, aliases, or custom tags) — the file
// must still be *legible* even though verdi doesn't own its schema; dialect
// enforcement is a property of the parser, not the schema.
func DecodeFlowmapLoose(data []byte) (*FlowmapSummary, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("artifact: .flowmap.yaml: yaml parse: %w", err)
	}
	if err := checkDialect(&root); err != nil {
		return nil, err
	}

	if len(root.Content) == 0 {
		return &FlowmapSummary{}, nil
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("artifact: .flowmap.yaml: document root is not a mapping")
	}

	var summary FlowmapSummary
	for i := 0; i+1 < len(doc.Content); i += 2 {
		key, val := doc.Content[i], doc.Content[i+1]
		switch key.Value {
		case "service":
			if val.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("artifact: .flowmap.yaml: service (line %d) must be a scalar", val.Line)
			}
			summary.Service = val.Value

		case "obligations":
			if val.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("artifact: .flowmap.yaml: obligations (line %d) must be a sequence", val.Line)
			}
			for _, item := range val.Content {
				if item.Kind != yaml.MappingNode {
					return nil, fmt.Errorf("artifact: .flowmap.yaml: obligations entry (line %d) must be a mapping", item.Line)
				}
				name, err := obligationName(item)
				if err != nil {
					return nil, err
				}
				summary.Obligations = append(summary.Obligations, name)
			}
		}
		// Every other key — "version", "classify", or anything upstream
		// adds next — is intentionally ignored: the documented exception.
	}
	return &summary, nil
}

// obligationName extracts the "name" field from one obligations[] mapping
// node, failing loudly if the entry has no name (an upstream obligation
// without a name would be a silent hole in verdi's discovery, which
// contradicts "silence is never a pass").
func obligationName(item *yaml.Node) (string, error) {
	for i := 0; i+1 < len(item.Content); i += 2 {
		if item.Content[i].Value == "name" {
			if item.Content[i+1].Kind != yaml.ScalarNode || item.Content[i+1].Value == "" {
				return "", fmt.Errorf("artifact: .flowmap.yaml: obligations entry (line %d) has a non-scalar or empty name", item.Line)
			}
			return item.Content[i+1].Value, nil
		}
	}
	return "", fmt.Errorf("artifact: .flowmap.yaml: obligations entry (line %d) has no name", item.Line)
}
