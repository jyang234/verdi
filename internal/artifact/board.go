package artifact

import "fmt"

const boardSchema = "verdi.board/v1"

// Pin is a board card: a pinned context-manifest entry placed at (X, Y)
// (05 §Workbench board model table).
type Pin struct {
	Ref string  `json:"ref"`
	X   float64 `json:"x"`
	Y   float64 `json:"y"`
}

// Sticky is a board-anchored annotation's position (the annotation record
// itself lives in the mutable annotations JSONL; the board only stores its
// id and coordinates).
type Sticky struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

// Yarn is a proto-link between two board elements, promoted to a typed
// link or prose by the commit-to-design skill (05 §Workbench).
type Yarn struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
}

// Board is schema verdi.board/v1 (02 §Record schemas): live at
// data/mutable/boards/<story>.json (no Frozen/Provenance), or committed
// alongside a spec at commit-to-design (Frozen + Provenance present —
// "one frame, not a drag history", 05 §Workbench).
type Board struct {
	Schema     string      `json:"schema"`
	Pins       []Pin       `json:"pins"`
	Stickies   []Sticky    `json:"stickies"`
	Yarn       []Yarn      `json:"yarn"`
	Frozen     *Frozen     `json:"frozen,omitempty"`
	Provenance *Provenance `json:"provenance,omitempty"`
}

// DecodeBoard strict-decodes and validates a board.json document.
func DecodeBoard(data []byte) (*Board, error) {
	var b Board
	if err := DecodeStrictJSON(data, &b); err != nil {
		return nil, err
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	return &b, nil
}

// Validate checks the schema literal, that every pin ref is a pinned
// artifact ref, every sticky id looks like an annotation id, yarn entries
// have both endpoints, and Frozen/Provenance are both present or both
// absent (a frozen board snapshot always carries design provenance).
func (b Board) Validate() error {
	if b.Schema != boardSchema {
		return fmt.Errorf("artifact: board schema %q, want %q", b.Schema, boardSchema)
	}
	for i, p := range b.Pins {
		if _, err := ParsePinnedRef(p.Ref); err != nil {
			return fmt.Errorf("artifact: board pins[%d]: %w", i, err)
		}
	}
	for i, s := range b.Stickies {
		if !annotationIDRe.MatchString(s.ID) {
			return fmt.Errorf("artifact: board stickies[%d]: id %q is not a valid annotation id (I-11)", i, s.ID)
		}
	}
	for i, y := range b.Yarn {
		if y.From == "" || y.To == "" {
			return fmt.Errorf("artifact: board yarn[%d]: from and to are both required", i)
		}
	}
	if (b.Frozen == nil) != (b.Provenance == nil) {
		return fmt.Errorf("artifact: board frozen and provenance must be both present or both absent")
	}
	if b.Frozen != nil {
		if err := b.Frozen.Validate(); err != nil {
			return fmt.Errorf("artifact: board frozen: %w", err)
		}
	}
	if b.Provenance != nil {
		if err := b.Provenance.Validate(); err != nil {
			return fmt.Errorf("artifact: board provenance: %w", err)
		}
	}
	return nil
}
