package artifact

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v3"
)

// allowedYAMLTags is the restricted frontmatter dialect's tag whitelist
// (PLAN.md I-1, 02 §Common frontmatter): the standard scalar/collection
// tags YAML resolves implicitly. Anything else — an explicit non-standard
// tag like "!mytag", or "!!binary"/"!!merge" — is a custom tag and is
// rejected. Custom tags are rejected regardless of blank Tag on document
// nodes; see checkDialect.
var allowedYAMLTags = map[string]bool{
	"!!str":       true,
	"!!int":       true,
	"!!bool":      true,
	"!!float":     true,
	"!!null":      true,
	"!!seq":       true,
	"!!map":       true,
	"!!timestamp": true,
}

// SplitFrontmatter extracts the YAML frontmatter block and the remaining
// body from a markdown document's raw bytes. Frontmatter is the text
// between the first two lines that are exactly "---" (02 §Common
// frontmatter examples; the convention this package's callers rely on).
func SplitFrontmatter(doc []byte) (frontmatter, body []byte, err error) {
	const delim = "---"

	lines := bytes.Split(doc, []byte("\n"))
	if len(lines) == 0 || string(bytes.TrimRight(lines[0], "\r")) != delim {
		return nil, nil, fmt.Errorf("artifact: document does not start with a %q frontmatter delimiter", delim)
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if string(bytes.TrimRight(lines[i], "\r")) == delim {
			end = i
			break
		}
	}
	if end == -1 {
		return nil, nil, fmt.Errorf("artifact: no closing %q frontmatter delimiter found", delim)
	}

	fm := bytes.Join(lines[1:end], []byte("\n"))
	rest := bytes.Join(lines[end+1:], []byte("\n"))
	return fm, rest, nil
}

// DecodeStrict decodes YAML frontmatter bytes into out, enforcing:
//
//   - KnownFields(true): any key in the document that out's type does not
//     declare is a hard decode error (VL-001's "decodes strictly").
//   - the restricted dialect (PLAN.md I-1): anchors, aliases, and custom
//     tags are rejected outright, each naming the offense and its
//     line/column.
//
// This is the package's — and the module's — single YAML decode seam.
func DecodeStrict(data []byte, out interface{}) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("artifact: yaml parse: %w", err)
	}
	if err := checkDialect(&root); err != nil {
		return err
	}

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("artifact: strict decode: %w", err)
	}
	return nil
}

// checkDialect walks n and every descendant, failing on the first anchor,
// alias, or non-standard tag it finds.
func checkDialect(n *yaml.Node) error {
	if n == nil {
		return nil
	}

	if n.Anchor != "" {
		return fmt.Errorf("artifact: dialect violation: anchor %q not allowed (line %d, column %d) — PLAN.md I-1 rejects YAML anchors in frontmatter", "&"+n.Anchor, n.Line, n.Column)
	}
	if n.Kind == yaml.AliasNode {
		return fmt.Errorf("artifact: dialect violation: alias %q not allowed (line %d, column %d) — PLAN.md I-1 rejects YAML aliases in frontmatter", "*"+n.Value, n.Line, n.Column)
	}
	if n.Kind != yaml.DocumentNode && n.Tag != "" && !allowedYAMLTags[n.Tag] {
		return fmt.Errorf("artifact: dialect violation: custom tag %q not allowed (line %d, column %d) — PLAN.md I-1 rejects custom YAML tags in frontmatter", n.Tag, n.Line, n.Column)
	}

	for _, c := range n.Content {
		if err := checkDialect(c); err != nil {
			return err
		}
	}
	return nil
}

// DecodeYAMLLoose decodes arbitrary, foreign-schema YAML (data) into a
// generic Go value (map[string]interface{} / []interface{} / scalars) —
// the same "verdi doesn't own this schema, read it as a guest" posture
// DecodeFlowmapLoose established for .flowmap.yaml, generalized for any
// upstream-owned YAML document a caller needs to transcode rather than
// strictly validate (dex build's OpenAPI-doc-to-JSON transcoding: 05
// §Verdi-dex mechanics discovers `<service-root>/api/openapi.{yaml,yml,json}`
// by convention, and the committed file — not a verdi schema — is the
// source of truth). The restricted dialect (no anchors, aliases, or custom
// tags) is still enforced, since dialect is a property of the parser, not
// the schema, exactly as DecodeFlowmapLoose reasons.
func DecodeYAMLLoose(data []byte) (interface{}, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("artifact: yaml parse: %w", err)
	}
	if err := checkDialect(&root); err != nil {
		return nil, err
	}
	var generic interface{}
	if err := root.Decode(&generic); err != nil {
		return nil, fmt.Errorf("artifact: decoding generic yaml: %w", err)
	}
	return generic, nil
}

// DecodeStrictJSON decodes JSON bytes into out with DisallowUnknownFields
// and trailing-data rejection (CLAUDE.md: "JSON via DisallowUnknownFields +
// trailing-data rejection"). It is used for the record schemas that live in
// plain JSON files (board.json, verdicts.json entries, rollup.json) rather
// than markdown frontmatter.
func DecodeStrictJSON(data []byte, out interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("artifact: strict json decode: %w", err)
	}
	if dec.More() {
		return fmt.Errorf("artifact: trailing data after top-level JSON value")
	}
	return nil
}
