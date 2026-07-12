// Package boardlayout is the zoned, incremental, position-stable board
// layout algorithm (05 §Workbench "Layout": objects without a stored
// coordinate are grouped by kind, ordered by document/ID order, and
// slotted into their zone's next free position; stored coordinates are
// never moved by generation). The algorithm is spike S8's, under the
// review adjudication recorded in PLAN-V1 §5 V1-P6: the layout.json
// WRITER prunes orphaned position entries (a dangling key is a VL-018
// lint error), the slot counter derives from currently-occupied slots,
// and deterministic freed-slot reuse is permitted.
//
// Only the S8 properties bind: same inputs → same layout (no wall clock,
// no randomness, no map-iteration-order dependence); stored positions
// pass through GENERATE verbatim — never rewritten, even when they
// collide; adding a new object (which the board always appends in
// document order) never moves any previously placed object. Zone
// geometry is explicitly NOT binding (S8 findings §"Binding constraints"
// item 5) — this package lays each kind out as its own vertical column,
// a kind-per-column wall that keeps the projection compact and legible.
//
// Rendering is a separate, documented step: the owner directive
// (R4-I-35) is that cards never RENDER stacked in any mode, so the projection
// applies ResolveDisplayOverlaps (display.go) to Generate's output —
// display-time nudging of colliding stored positions that never touches
// layout.json (only a real drag writes, and it writes only the dragged
// card).
package boardlayout

import (
	"fmt"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
)

// ZoneKind is the closed enum of board zones. Unknown kinds fail closed
// (CLAUDE.md; S8's adjudicated edge case).
type ZoneKind string

const (
	ZoneAC           ZoneKind = "acceptance-criterion"
	ZoneConstraint   ZoneKind = "constraint"
	ZoneDecision     ZoneKind = "decision"
	ZoneOpenQuestion ZoneKind = "open-question"
	// ZoneStub holds a feature wall's declared-stub cards — the scoping
	// canvas's kind-locked band between open questions and references
	// (spec/scoping-canvas dc-6). Stub positions are always computed,
	// never stored, exactly like reference cards: layout.json keys must
	// resolve to declared object ids (VL-018), and a stub is not an
	// object; stored stub positions are deferred until dragging stubs
	// proves needed (dc-6, the smallest reversible option).
	ZoneStub ZoneKind = "stub"
	// ZoneReference holds reference cards — external edge targets
	// (05 §Workbench: an edge target outside the spec still renders).
	// Their positions are always computed, never stored: layout.json keys
	// must resolve to declared object ids (VL-018), which a ref is not.
	ZoneReference ZoneKind = "reference"
	// ZoneScratch is the scratch lane — the landing band for
	// comment/agent-task stickies (annotation-layer paper with no
	// graduation column of its own). Deliberately NOT in zoneOrder:
	// no spec OBJECT files there, so Generate keeps failing closed on
	// it; only the sticky landing policy and the zone labels use it.
	ZoneScratch ZoneKind = "scratch"
)

// zoneOrder is the fixed left-to-right column order. The stubs band
// sits between open questions and references (spec/scoping-canvas dc-6:
// spike stubs beside the questions they resolve; references stay
// outermost, non-binding house style).
var zoneOrder = []ZoneKind{ZoneAC, ZoneConstraint, ZoneDecision, ZoneOpenQuestion, ZoneStub, ZoneReference}

var zoneIndex = func() map[ZoneKind]int {
	m := make(map[ZoneKind]int, len(zoneOrder))
	for i, k := range zoneOrder {
		m[k] = i
	}
	return m
}()

// Object is one board-placeable element: a spec object or a reference
// card. DocOrder is its position in document/parse order within its own
// kind (05 §Workbench: "ordered by document/ID order").
type Object struct {
	Kind     ZoneKind
	ID       string
	DocOrder int
}

// Grid geometry (non-binding, see package comment): one column per
// zone, compact enough that every zone — including the reference column
// — sits inside one viewport, so drawing yarn from a decision to a
// reference card never needs a scroll mid-gesture.
//
// The card footprint is UNIFORM (owner directive: fixed index-card
// dimensions, clamped text) and mirrored by style.css's .objcard/.refcard
// rules; the pitches derive from it (footprint + gutter), which is what
// makes default placement collision-free by construction.
const (
	// CardWidth and CardHeight are the object card's fixed rendered
	// footprint in px (style.css .objcard: 12.5rem × 8.75rem).
	CardWidth  = 200
	CardHeight = 140
	// RefCardHeight is the squat reference-card footprint
	// (style.css .refcard: 12.5rem × 4.5rem).
	RefCardHeight = 72
	// StubCardHeight is the stub card's footprint (style.css .stubcard:
	// 12.5rem × 7.5rem) — squatter than an object card (a stub is a
	// typeset claim about a future story, not this spec's own object),
	// taller than a reference (it carries its attribution chips).
	StubCardHeight = 120

	// ZoneOriginY is the first row's y origin — exported alongside the
	// column bands so the sticky landing policy can start an empty lane
	// at the same first slot the zoned algorithm uses.
	ZoneOriginY = 40

	zoneOriginY = ZoneOriginY
	zoneMarginX = 40
	zonePitchX  = CardWidth + 28  // column rhythm: footprint + gutter
	rowPitch    = CardHeight + 36 // row rhythm: footprint + gap
)

// Generate computes every object's position: stored positions verbatim,
// everything else at its zone's next free slot. It is a pure function of
// its inputs. Stored entries whose key names no current object (orphans)
// are ignored entirely — the adjudicated "slot counter from
// currently-occupied slots" policy — so generation agrees with a writer
// that prunes them (WriteFile).
func Generate(objects []Object, stored map[string]artifact.Position) (map[string]artifact.Position, error) {
	for _, o := range objects {
		if _, ok := zoneIndex[o.Kind]; !ok {
			return nil, fmt.Errorf("boardlayout: unknown object kind %q for id %q (fail closed)", o.Kind, o.ID)
		}
	}

	live := make(map[string]bool, len(objects))
	for _, o := range objects {
		live[o.ID] = true
	}

	buckets := make([][]Object, len(zoneOrder))
	for _, o := range objects {
		zi := zoneIndex[o.Kind]
		buckets[zi] = append(buckets[zi], o)
	}
	for zi := range buckets {
		b := buckets[zi]
		sort.Slice(b, func(i, j int) bool { return canonicalLess(b[i], b[j]) })
	}

	out := make(map[string]artifact.Position, len(objects))

	// occupied tracks every FOOTPRINT spoken for by a LIVE stored
	// position (orphans ignored per the adjudication) or an already-
	// placed object. Overlap is tested rect-against-rect — with the
	// uniform footprint, a fresh slot can never land under a stored
	// card, even one dragged off the grid. Membership testing is
	// order-independent, so map iteration order cannot leak into the
	// layout (S8 property 1).
	kindOf := make(map[string]ZoneKind, len(objects))
	for _, o := range objects {
		kindOf[o.ID] = o.Kind
	}
	occupied := make([]Rect, 0, len(objects))
	claim := func(id string, p artifact.Position) {
		w, h := FootprintFor(kindOf[id])
		occupied = append(occupied, Rect{X: p.X, Y: p.Y, W: w, H: h})
	}
	overlaps := func(r Rect) bool {
		for _, o := range occupied {
			if r.intersects(o) {
				return true
			}
		}
		return false
	}
	for id, p := range stored {
		if live[id] {
			claim(id, p)
		}
	}

	for zi, bucket := range buckets {
		zoneX := zoneMarginX + zi*zonePitchX
		next := 0
		for _, o := range bucket {
			if p, ok := stored[o.ID]; ok {
				out[o.ID] = p // verbatim, never inspected or "fixed"
				continue
			}
			w, h := FootprintFor(o.Kind)
			for {
				p := positionForSlot(next, zoneX)
				if !overlaps(Rect{X: p.X, Y: p.Y, W: w, H: h}) {
					out[o.ID] = p
					claim(o.ID, p)
					next++
					break
				}
				next++
			}
		}
	}

	return out, nil
}

func positionForSlot(slot, zoneX int) artifact.Position {
	return artifact.Position{X: float64(zoneX), Y: float64(zoneOriginY + slot*rowPitch)}
}

// ZoneColumn is one zone's rendered column band: the kind, its column's
// x origin, and the band width (one card footprint). Exposed so the
// board can LABEL the regions the zoned algorithm already files cards
// into (05 §Workbench "grouped by object kind" — the wall's filing
// scheme, made legible). Geometry stays non-binding (package comment);
// the labels describe where unstored cards land, never where a dragged
// card must stay.
type ZoneColumn struct {
	Kind  ZoneKind
	X     int
	Width int
}

// ZoneColumns returns every zone's column band in the fixed
// left-to-right zone order — a pure function of the package constants
// (no inputs, same output every call).
func ZoneColumns() []ZoneColumn {
	cols := make([]ZoneColumn, len(zoneOrder))
	for i, k := range zoneOrder {
		cols[i] = ZoneColumn{Kind: k, X: zoneMarginX + i*zonePitchX, Width: CardWidth}
	}
	return cols
}

// ScratchColumn is the scratch lane's band: one column past the last
// object zone. Not part of ZoneColumns — no spec object is ever slotted
// there (ZoneScratch stays outside zoneOrder, so Generate fails closed
// on it); it exists for the sticky landing policy and its zone label.
func ScratchColumn() ZoneColumn {
	return ZoneColumn{Kind: ZoneScratch, X: zoneMarginX + len(zoneOrder)*zonePitchX, Width: CardWidth}
}

// Prune returns stored restricted to keys naming a live object id — the
// adjudicated writer policy (a dangling layout.json key is a VL-018 lint
// error, so the writer never persists one).
func Prune(stored map[string]artifact.Position, live map[string]bool) map[string]artifact.Position {
	out := make(map[string]artifact.Position, len(stored))
	for id, p := range stored {
		if live[id] {
			out[id] = p
		}
	}
	return out
}
