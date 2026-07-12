package boardlayout

// Display-time collision resolution (owner directive, round 6: cards
// must never RENDER stacked, in any mode). Stored positions still pass
// through Generate verbatim and layout.json is never rewritten by
// rendering — this is a separate projection step applied to what gets
// DRAWN: the first claimant of a contested footprint keeps its stored
// position; a later stored object whose footprint intersects an
// already-claimed rect is nudged to the nearest free position via
// ResolveDrop (the same machinery that already guards the write path);
// unstored objects then slot zoned and overlap-aware around the RESOLVED
// rects. The accepted consequence: when the card that kept a contested
// spot is dragged away, the nudged card snaps back to its stored
// position on the next render — deterministic and honest, because the
// stored record was never rewritten.

import (
	"fmt"
	"sort"

	"github.com/OWNER/verdi/internal/artifact"
)

// ResolveDisplayOverlaps computes the RENDERED position of every object:
// Generate's layout with stored-position collisions resolved for
// display. Stored-position objects are processed in Generate's canonical
// order (zone order, then DocOrder, then ID); the first claimant of a
// contested footprint keeps its stored position verbatim, later
// intersecting ones resolve to the nearest free position. Unstored
// objects slot afterwards — Generate itself, fed the display-resolved
// map, so there is exactly one slotting algorithm — which is what keeps
// add-never-reflows true at the rendered layer. Pure function; the
// result is pairwise collision-free by construction; nothing is ever
// written back to layout.json.
func ResolveDisplayOverlaps(objects []Object, stored map[string]artifact.Position) (map[string]artifact.Position, error) {
	for _, o := range objects {
		if _, ok := zoneIndex[o.Kind]; !ok {
			return nil, fmt.Errorf("boardlayout: unknown object kind %q for id %q (fail closed)", o.Kind, o.ID)
		}
	}

	withStored := make([]Object, 0, len(stored))
	for _, o := range objects {
		if _, ok := stored[o.ID]; ok {
			withStored = append(withStored, o)
		}
	}
	sort.Slice(withStored, func(i, j int) bool { return canonicalLess(withStored[i], withStored[j]) })

	resolved := make(map[string]artifact.Position, len(withStored))
	claimed := make([]Rect, 0, len(withStored))
	for _, o := range withStored {
		p := stored[o.ID]
		w, h := FootprintFor(o.Kind)
		for _, c := range claimed {
			if (Rect{X: p.X, Y: p.Y, W: w, H: h}).intersects(c) {
				p = ResolveDrop(p, w, h, claimed)
				break
			}
		}
		resolved[o.ID] = p
		claimed = append(claimed, Rect{X: p.X, Y: p.Y, W: w, H: h})
	}

	// Orphaned stored entries never made it into resolved (only live
	// objects were collected), matching Generate's adjudicated policy.
	return Generate(objects, resolved)
}

// canonicalLess orders objects the way the board lays them out: zone
// order, then DocOrder, then ID (byte-wise ordinal, locale-independent).
// Generate's per-zone bucket sort and display resolution's stored pass
// share this one definition of canonical order.
func canonicalLess(a, b Object) bool {
	if zoneIndex[a.Kind] != zoneIndex[b.Kind] {
		return zoneIndex[a.Kind] < zoneIndex[b.Kind]
	}
	if a.DocOrder != b.DocOrder {
		return a.DocOrder < b.DocOrder
	}
	return a.ID < b.ID
}
