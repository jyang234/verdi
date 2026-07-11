package artifact

import "fmt"

const boardLayoutSchema = "verdi.boardlayout/v1"

// Position is one object's board coordinates in a layout.json sidecar
// (02 §Record schemas: "Board layout").
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BoardLayout is schema verdi.boardlayout/v1 (R4-I-5, 02 §Record schemas):
// `.verdi/specs/<status-dir>/<name>/layout.json`, a sibling of the spec.
// Positions only, keyed by object id — never content. It inherits its
// spec's temporal class (01 §Temporal classes) rather than carrying its
// own Frozen/Provenance. Whether every key resolves to a real object id
// declared in the sibling spec's frontmatter is VL-018's job (V1-P2), not
// checked here.
type BoardLayout struct {
	Schema    string              `json:"schema"`
	Positions map[string]Position `json:"positions"`
}

// DecodeBoardLayout strict-decodes and validates a layout.json document.
func DecodeBoardLayout(data []byte) (*BoardLayout, error) {
	var bl BoardLayout
	if err := DecodeStrictJSON(data, &bl); err != nil {
		return nil, err
	}
	if err := bl.Validate(); err != nil {
		return nil, err
	}
	return &bl, nil
}

// Validate checks the schema literal and that every positions key looks
// like a real object id.
func (bl BoardLayout) Validate() error {
	if bl.Schema != boardLayoutSchema {
		return fmt.Errorf("artifact: boardlayout schema %q, want %q", bl.Schema, boardLayoutSchema)
	}
	for k := range bl.Positions {
		if !objectIDRe.MatchString(k) {
			return fmt.Errorf("artifact: boardlayout positions key %q is not a valid object id", k)
		}
	}
	return nil
}
