// The size-smell observation (spec/case-file-flags ac-2, dc-1, dc-2): an
// acceptance-criteria column whose ESTIMATED rendered height exceeds the
// declared reference viewport raises a case-file badge — an observation,
// never a rule (co-2: nothing blocks, gates, or refuses on it; its entire
// effect is to be seen).
//
// The estimate is dc-1's deterministic proxy, never a client measurement:
// AC-zone top offset + declared AC count × the board layout's declared
// row pitch, measured against ReferenceViewportHeight. It reads the spec
// frontmatter's AC count and internal/boardlayout's declared geometry
// constants ONLY — never stored or dragged card positions (dragging paper
// around must not create or destroy an observation about the spec's
// size), and never a measured client viewport (a badge that appears on a
// small laptop and vanishes on a tall monitor would not be a pure
// function of pinned inputs, wall-receipts co-1).
package wallbadge

import (
	"fmt"

	"github.com/jyang234/verdi/internal/boardlayout"
)

// ReferenceViewportHeight is dc-1's declared reference-viewport-height
// constant: 900 CSS px, a laptop-class viewport. A DECLARED CONSTANT,
// deliberately not configuration (dc-2: a config knob would invite tuning
// the smell away instead of reading it) — tunable only by amending
// spec/case-file-flags dc-1/dc-2.
const ReferenceViewportHeight = 900

// SizeSmellBadge computes the size-smell observation for one spec wall —
// ANY spec wall that declares acceptance criteria, feature and story
// alike (spec/case-file-flags dc-3). A pure function of its pinned
// inputs: acCount is the spec frontmatter's declared AC count
// (len(fm.AcceptanceCriteria), counted by the caller that already decoded
// the document); specRelPath/specRevision identify that one input for the
// derivation record (co-1: every drawer citation is an input revision).
//
// Returns nil when nothing is observed — no ACs declared, or the dc-1
// estimate at or under ReferenceViewportHeight — and a case-file
// DerivationRecord (Source "observe:size-smell", dc-2's record
// vocabulary) when the estimate exceeds it. The records disclose every
// operand by name and value — the constants, the count, the computed
// estimate — so the proxy is legible in the drawer, not hidden behind
// the badge (dc-1). The copy stays in the wall's observation register
// (dc-2: the multi-claim observation's voice — "worth a look", never an
// error).
func SizeSmellBadge(specRelPath, specRevision string, acCount int) *DerivationRecord {
	if acCount <= 0 {
		return nil // no AC column at all: nothing to observe
	}
	estimate := boardlayout.ZoneOriginY + acCount*boardlayout.RowPitch
	if estimate <= ReferenceViewportHeight {
		return nil // proven-unraised: the column fits the reference viewport
	}
	return &DerivationRecord{
		Source: "observe:size-smell",
		Label:  "size-smell",
		Inputs: []InputRecord{{Name: "spec", Path: specRelPath, Revision: specRevision}},
		Records: []string{
			fmt.Sprintf("boardlayout.ZoneOriginY = %d (the AC zone's top offset, declared)", boardlayout.ZoneOriginY),
			fmt.Sprintf("boardlayout.RowPitch = %d (card height %d + gap %d, declared)", boardlayout.RowPitch, boardlayout.CardHeight, boardlayout.RowPitch-boardlayout.CardHeight),
			fmt.Sprintf("wallbadge.ReferenceViewportHeight = %d (declared reference constant, not a measurement)", ReferenceViewportHeight),
			fmt.Sprintf("declared acceptance criteria: %d", acCount),
			fmt.Sprintf("estimated AC-column height %d + %d × %d = %d exceeds the reference viewport %d", boardlayout.ZoneOriginY, acCount, boardlayout.RowPitch, estimate, ReferenceViewportHeight),
			"the AC column has outgrown one screen; outcome-shaped acceptance criteria this numerous are worth a scoping look",
		},
	}
}
