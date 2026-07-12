package boardlayout

import (
	"math/rand"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// A drop on open canvas keeps the exact drop position (freedom is not
// taken away when there is nothing to collide with).
func TestResolveDrop_FreePositionVerbatim(t *testing.T) {
	obstacles := []Rect{{X: 40, Y: 40, W: CardWidth, H: CardHeight}}
	p := artifact.Position{X: 600, Y: 300.5}
	got := ResolveDrop(p, CardWidth, CardHeight, obstacles)
	if got != p {
		t.Fatalf("free drop moved: %v -> %v", p, got)
	}
}

// A drop overlapping an obstacle resolves to a nearby position that
// overlaps nothing — and the obstacles are untouched by construction
// (ResolveDrop returns only the dragged card's position).
func TestResolveDrop_CollisionResolvesToFree(t *testing.T) {
	obstacles := []Rect{
		{X: 40, Y: 40, W: CardWidth, H: CardHeight},
		{X: 40, Y: 216, W: CardWidth, H: CardHeight},
	}
	p := artifact.Position{X: 60, Y: 80} // squarely on the first card
	got := ResolveDrop(p, CardWidth, CardHeight, obstacles)
	r := Rect{X: got.X, Y: got.Y, W: CardWidth, H: CardHeight}
	for i, o := range obstacles {
		if r.intersects(o) {
			t.Fatalf("resolved position %v still overlaps obstacle %d (%v)", got, i, o)
		}
	}
	if got.X < 0 || got.Y < 0 {
		t.Fatalf("resolved position off-canvas: %v", got)
	}
}

// Determinism: the same drop against the same obstacles resolves to the
// same position, regardless of obstacle slice order.
func TestResolveDrop_DeterministicAndOrderInvariant(t *testing.T) {
	obstacles := []Rect{
		{X: 40, Y: 40, W: CardWidth, H: CardHeight},
		{X: 268, Y: 40, W: CardWidth, H: CardHeight},
		{X: 40, Y: 216, W: CardWidth, H: CardHeight},
		{X: 496, Y: 40, W: CardWidth, H: RefCardHeight},
	}
	p := artifact.Position{X: 100, Y: 100}
	first := ResolveDrop(p, CardWidth, CardHeight, obstacles)

	rng := rand.New(rand.NewSource(7))
	for trial := 0; trial < 20; trial++ {
		shuffled := append([]Rect(nil), obstacles...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if got := ResolveDrop(p, CardWidth, CardHeight, shuffled); got != first {
			t.Fatalf("trial %d: obstacle order changed the resolution: %v vs %v", trial, got, first)
		}
	}
}

// A negative drop coordinate is clamped onto the canvas before resolving.
func TestResolveDrop_ClampsNegative(t *testing.T) {
	got := ResolveDrop(artifact.Position{X: -30, Y: -10}, CardWidth, CardHeight, nil)
	if got.X != 0 || got.Y != 0 {
		t.Fatalf("negative drop not clamped: %v", got)
	}
}

// Even a crowded neighbourhood resolves (the slot-grid fallback): the
// result overlaps nothing.
func TestResolveDrop_CrowdedFallsBackToGrid(t *testing.T) {
	var obstacles []Rect
	// Wall in the drop point from every side, including the slide-out
	// candidates' landing spots.
	for x := 0.0; x <= 1200; x += 210 {
		for y := 0.0; y <= 900; y += 150 {
			obstacles = append(obstacles, Rect{X: x, Y: y, W: CardWidth, H: CardHeight})
		}
	}
	got := ResolveDrop(artifact.Position{X: 300, Y: 300}, CardWidth, CardHeight, obstacles)
	r := Rect{X: got.X, Y: got.Y, W: CardWidth, H: CardHeight}
	for _, o := range obstacles {
		if r.intersects(o) {
			t.Fatalf("fallback position %v overlaps %v", got, o)
		}
	}
}

// FootprintFor: references are squat cards; everything else is the
// uniform index-card footprint.
func TestFootprintFor(t *testing.T) {
	if w, h := FootprintFor(ZoneReference); w != CardWidth || h != RefCardHeight {
		t.Fatalf("reference footprint = %v x %v", w, h)
	}
	for _, k := range []ZoneKind{ZoneAC, ZoneConstraint, ZoneDecision, ZoneOpenQuestion} {
		if w, h := FootprintFor(k); w != CardWidth || h != CardHeight {
			t.Fatalf("%s footprint = %v x %v", k, w, h)
		}
	}
}
