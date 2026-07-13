package boardlayout

// Drop resolution: a dragged card comes to rest at the nearest
// non-overlapping position (owner directive: the board is collision-free
// by construction). Resolution produces ONLY the dragged card's position
// — no other stored position is ever touched, so the ratified layout
// properties (stored-verbatim rendering, add-never-reflows, determinism)
// are untouched: this guards the WRITE path against creating new
// collisions, while colliding positions already stored keep rendering
// verbatim.

import (
	"sort"

	"github.com/jyang234/verdi/internal/artifact"
)

// Rect is an axis-aligned card footprint at a board position (px).
type Rect struct {
	X, Y, W, H float64
}

func (r Rect) intersects(o Rect) bool {
	return r.X < o.X+o.W && o.X < r.X+r.W && r.Y < o.Y+o.H && o.Y < r.Y+r.H
}

// FootprintFor is the rendered footprint (px) of a card of the given
// kind. The values mirror style.css's fixed card dimensions (.objcard,
// .refcard, .stubcard) — the uniform footprint is what makes slot
// placement and drop resolution collision-free by construction.
func FootprintFor(kind ZoneKind) (w, h float64) {
	switch kind {
	case ZoneReference:
		return CardWidth, RefCardHeight
	case ZoneStub:
		return CardWidth, StubCardHeight
	}
	return CardWidth, CardHeight
}

// dropClearance is the breathing room a resolved drop keeps from the
// obstacle it slid off.
const dropClearance = 12

// ResolveDrop returns where a dragged card of footprint w×h dropped at p
// comes to rest: p itself (clamped onto the canvas) when it overlaps no
// obstacle, otherwise the nearest non-overlapping position. Deterministic
// pure function: candidates are ranked by squared distance to the drop
// point, ties broken by smaller Y then smaller X, so obstacle order never
// matters; a crowded neighbourhood falls back to the first free zone-grid
// slot in row-major order.
func ResolveDrop(p artifact.Position, w, h float64, obstacles []Rect) artifact.Position {
	if p.X < 0 {
		p.X = 0
	}
	if p.Y < 0 {
		p.Y = 0
	}
	free := func(x, y float64) bool {
		r := Rect{X: x, Y: y, W: w, H: h}
		for _, o := range obstacles {
			if r.intersects(o) {
				return false
			}
		}
		return true
	}
	if free(p.X, p.Y) {
		return p
	}

	// Slide-out candidates: for every obstacle, the four positions flush
	// past its edges (keeping the drop's other coordinate), clamped onto
	// the canvas.
	var cands []artifact.Position
	add := func(x, y float64) {
		if x < 0 {
			x = 0
		}
		if y < 0 {
			y = 0
		}
		if free(x, y) {
			cands = append(cands, artifact.Position{X: x, Y: y})
		}
	}
	for _, o := range obstacles {
		add(o.X-w-dropClearance, p.Y)
		add(o.X+o.W+dropClearance, p.Y)
		add(p.X, o.Y-h-dropClearance)
		add(p.X, o.Y+o.H+dropClearance)
	}
	if len(cands) > 0 {
		sort.Slice(cands, func(i, j int) bool {
			di := sq(cands[i].X-p.X) + sq(cands[i].Y-p.Y)
			dj := sq(cands[j].X-p.X) + sq(cands[j].Y-p.Y)
			if di != dj {
				return di < dj
			}
			if cands[i].Y != cands[j].Y {
				return cands[i].Y < cands[j].Y
			}
			return cands[i].X < cands[j].X
		})
		return cands[0]
	}

	// Fallback: the first free zone-grid slot, row-major. Obstacles are
	// finite, so a free row always exists.
	for row := 0; ; row++ {
		for col := 0; col < len(zoneOrder); col++ {
			x := float64(zoneMarginX + col*zonePitchX)
			y := float64(zoneOriginY + row*rowPitch)
			if free(x, y) {
				return artifact.Position{X: x, Y: y}
			}
		}
	}
}

func sq(v float64) float64 { return v * v }
