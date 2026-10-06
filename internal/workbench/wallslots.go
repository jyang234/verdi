package workbench

import (
	"strings"

	"github.com/jyang234/verdi/internal/boardlayout"
)

// The add-in-place slots (spec/wall-canvas-v2 ac-5; ledger SI-350 (15);
// lane F2b): one per object column, at the column's foot — one row gap
// below the lowest paper whose footprint overlaps the column band, in the
// projection's stored positions, or at the first row when the column is
// empty — and only where the slot fits inside the canvas's working height.
// A pure function of the projection, like canvasMinHeight: the same
// footprints and the same sticky estimate, so a slot can never be placed
// where the canvas does not reach. The toolbar asset (walltoolbar.js)
// opens a slot into its form and declares the object with the server's
// next id (the canvas's data-next-id-* attributes) as one typed
// operation; the markup here carries the operation and prefix it needs.

const (
	// addSlotHeight is the slot's resting footprint (the handoff's dashed
	// 200 × 44 box); its open form grows below the same top.
	addSlotHeight = 44
	// addSlotGap is the gap between the lowest paper's foot and the slot's
	// top: the layout's row rhythm less the card (the handoff's "last card
	// + 176" from a card's top).
	addSlotGap = boardlayout.RowPitch - boardlayout.CardHeight
	// addSlotStickyHeight is canvasMinHeight's working estimate of a
	// sticky's height (stickies grow with their text); the two agree so a
	// slot under a sticky is inside the canvas the sticky sized.
	addSlotStickyHeight = 150
)

// addSlot is one rendered slot: the object kind it declares (the
// data-object-kind vocabulary), its typed operation and id prefix, and
// its canvas position.
type addSlot struct {
	Kind   string
	Op     string
	Prefix string
	X, Y   float64
}

// addSlotOps maps each object zone to its typed add operation and the id
// prefix the server's next id carries — the same pairs boardspec.js's
// ADD_OPS and boardspecasd.js's nextIDFor hold.
var addSlotOps = map[boardlayout.ZoneKind]addSlot{
	boardlayout.ZoneAC:           {Op: "add-ac", Prefix: "ac"},
	boardlayout.ZoneConstraint:   {Op: "add-constraint", Prefix: "co"},
	boardlayout.ZoneDecision:     {Op: "add-decision", Prefix: "dc"},
	boardlayout.ZoneOpenQuestion: {Op: "add-question", Prefix: "oq"},
}

// addSlotsFor computes the slots for a projection, in zone order.
func addSlotsFor(p *BoardProjection) []addSlot {
	type paper struct{ x, y, h float64 }
	var papers []paper
	for _, c := range p.Cards {
		papers = append(papers, paper{c.X, c.Y, cardHeightPx(c)})
	}
	for _, rc := range p.RefCards {
		papers = append(papers, paper{rc.X, rc.Y, refCardHeightPx(rc)})
	}
	for _, sv := range p.StubViews {
		papers = append(papers, paper{sv.X, sv.Y, boardlayout.StubCardHeight})
	}
	for _, s := range p.Stickies {
		papers = append(papers, paper{s.X, s.Y, addSlotStickyHeight})
	}
	limit := canvasMinHeight(p)
	var out []addSlot
	for _, col := range boardlayout.ZoneColumns() {
		ops, ok := addSlotOps[col.Kind]
		if !ok {
			continue
		}
		y := float64(boardlayout.ZoneOriginY)
		for _, pp := range papers {
			if !overlapsColumn(col, pp.x) {
				continue
			}
			if foot := pp.y + pp.h + addSlotGap; foot > y {
				y = foot
			}
		}
		if y+addSlotHeight > limit {
			continue // it does not fit
		}
		out = append(out, addSlot{Kind: string(col.Kind), Op: ops.Op, Prefix: ops.Prefix, X: float64(col.X), Y: y})
	}
	return out
}

// writeAddSlots renders the slots inside the canvas. Called only where
// the domain is live: a slot is a typed write.
func writeAddSlots(b *strings.Builder, p *BoardProjection) {
	for _, s := range addSlotsFor(p) {
		b.WriteString(`<div class="wall-slot" data-testid="slot-` + s.Prefix + `" data-slot-kind="` + s.Kind + `" data-slot-op="` + s.Op + `" data-slot-prefix="` + s.Prefix + `" style="left:` + px(s.X) + `;top:` + px(s.Y) + `">`)
		b.WriteString(`<button type="button" class="wall-slot-open" data-testid="slot-open-` + s.Prefix + `">+ ` + strings.ReplaceAll(s.Kind, "-", " ") + `</button></div>`)
	}
}
