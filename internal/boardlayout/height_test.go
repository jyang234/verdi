package boardlayout

import (
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// Object.Height (SI-278; lane L5 fix pass 2): a card taller than its
// kind's uniform footprint — a closed spec's superseded reference card or
// a decision card carrying its §6 lines — reserves the slots its height
// needs, so the layout stays collision-free by construction and no card
// renders under it.

func TestObjectFootprint(t *testing.T) {
	for _, tc := range []struct {
		name   string
		obj    Object
		wantH  float64
		wantW  float64
		height float64
	}{
		{"zero height is the kind's footprint", Object{Kind: ZoneReference}, RefCardHeight, CardWidth, 0},
		{"a height below the footprint is raised to it", Object{Kind: ZoneReference, Height: 10}, RefCardHeight, CardWidth, 10},
		{"a taller height is kept", Object{Kind: ZoneReference, Height: 240}, 240, CardWidth, 240},
		{"an object card grows likewise", Object{Kind: ZoneDecision, Height: 236}, 236, CardWidth, 236},
		{"a stub keeps its own footprint", Object{Kind: ZoneStub}, StubCardHeight, CardWidth, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, h := tc.obj.footprint()
			if w != tc.wantW || h != tc.wantH {
				t.Fatalf("footprint = %v×%v, want %v×%v", w, h, tc.wantW, tc.wantH)
			}
		})
	}
}

// rectsOf materializes every object's rendered rectangle.
func heightRectsOf(objs []Object, pos map[string]artifact.Position) map[string]Rect {
	out := map[string]Rect{}
	for _, o := range objs {
		w, h := o.footprint()
		p := pos[o.ID]
		out[o.ID] = Rect{X: p.X, Y: p.Y, W: w, H: h}
	}
	return out
}

func assertPairwiseFree(t *testing.T, rects map[string]Rect) {
	t.Helper()
	for a, ra := range rects {
		for b, rb := range rects {
			if a < b && ra.intersects(rb) {
				t.Errorf("%s %+v overlaps %s %+v", a, ra, b, rb)
			}
		}
	}
}

func TestGenerate_TallObjectReservesItsHeight(t *testing.T) {
	objs := []Object{
		{Kind: ZoneReference, ID: "spec/closed-feature#dc-1", DocOrder: 0, Height: 240},
		{Kind: ZoneReference, ID: "spec/closed-story#ac-1", DocOrder: 1},
		{Kind: ZoneDecision, ID: "dc-1", DocOrder: 0, Height: 236},
		{Kind: ZoneDecision, ID: "dc-2", DocOrder: 1},
	}
	pos, err := Generate(objs, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The tall reference card takes slot 0 and spans past slot 1's origin
	// (ZoneOriginY + RowPitch = 216 < 40 + 240), so the next reference
	// card lands at slot 2; the same for the decisions.
	slot := func(n int) float64 { return float64(ZoneOriginY + n*RowPitch) }
	if pos["spec/closed-feature#dc-1"].Y != slot(0) || pos["spec/closed-story#ac-1"].Y != slot(2) {
		t.Errorf("reference slots = %v / %v, want %v / %v", pos["spec/closed-feature#dc-1"].Y, pos["spec/closed-story#ac-1"].Y, slot(0), slot(2))
	}
	if pos["dc-1"].Y != slot(0) || pos["dc-2"].Y != slot(2) {
		t.Errorf("decision slots = %v / %v, want %v / %v", pos["dc-1"].Y, pos["dc-2"].Y, slot(0), slot(2))
	}
	assertPairwiseFree(t, heightRectsOf(objs, pos))

	// A height inside one pitch reserves nothing extra.
	short := []Object{
		{Kind: ZoneReference, ID: "spec/a#dc-1", DocOrder: 0, Height: 150},
		{Kind: ZoneReference, ID: "spec/b#ac-1", DocOrder: 1},
	}
	pos, err = Generate(short, nil)
	if err != nil {
		t.Fatal(err)
	}
	if pos["spec/b#ac-1"].Y != slot(1) {
		t.Errorf("a 150 px card must leave slot 1 free: got %v", pos["spec/b#ac-1"].Y)
	}
	assertPairwiseFree(t, heightRectsOf(short, pos))
}

// TestGenerate_TallStoredCardIsAnObstacle: a stored position's claim is
// its full rendered height, so an unstored card slots below it.
func TestGenerate_TallStoredCardIsAnObstacle(t *testing.T) {
	x := float64(ZoneColumns()[zoneIndex[ZoneReference]].X)
	objs := []Object{
		{Kind: ZoneReference, ID: "spec/tall#dc-1", DocOrder: 0, Height: 300},
		{Kind: ZoneReference, ID: "spec/next#ac-1", DocOrder: 1},
	}
	stored := map[string]artifact.Position{"spec/tall#dc-1": {X: x, Y: float64(ZoneOriginY)}}
	pos, err := Generate(objs, stored)
	if err != nil {
		t.Fatal(err)
	}
	if pos["spec/tall#dc-1"] != stored["spec/tall#dc-1"] {
		t.Errorf("stored position not verbatim: %+v", pos["spec/tall#dc-1"])
	}
	if pos["spec/next#ac-1"].Y < float64(ZoneOriginY)+300 {
		t.Errorf("the unstored card slotted under the tall stored card: y=%v", pos["spec/next#ac-1"].Y)
	}
	assertPairwiseFree(t, heightRectsOf(objs, pos))
}

// TestResolveDisplayOverlaps_TallStoredCardNudgesLater: two stored
// positions that collide only because the first card is tall resolve at
// display time — the later claimant moves, the first stays verbatim.
func TestResolveDisplayOverlaps_TallStoredCardNudgesLater(t *testing.T) {
	x := float64(ZoneColumns()[zoneIndex[ZoneReference]].X)
	objs := []Object{
		{Kind: ZoneReference, ID: "spec/tall#dc-1", DocOrder: 0, Height: 240},
		{Kind: ZoneReference, ID: "spec/next#ac-1", DocOrder: 1},
	}
	stored := map[string]artifact.Position{
		"spec/tall#dc-1": {X: x, Y: float64(ZoneOriginY)},
		"spec/next#ac-1": {X: x, Y: float64(ZoneOriginY + RowPitch)}, // inside the tall card's 240 px
	}
	pos, err := ResolveDisplayOverlaps(objs, stored)
	if err != nil {
		t.Fatal(err)
	}
	if pos["spec/tall#dc-1"] != stored["spec/tall#dc-1"] {
		t.Errorf("first claimant moved: %+v", pos["spec/tall#dc-1"])
	}
	if pos["spec/next#ac-1"] == stored["spec/next#ac-1"] {
		t.Errorf("the later colliding card was not nudged")
	}
	assertPairwiseFree(t, heightRectsOf(objs, pos))
	// Without the height the same stored positions do not collide.
	objs[0].Height = 0
	pos, err = ResolveDisplayOverlaps(objs, stored)
	if err != nil {
		t.Fatal(err)
	}
	if pos["spec/next#ac-1"] != stored["spec/next#ac-1"] {
		t.Errorf("a footprint-height card must leave the next stored position verbatim: %+v", pos["spec/next#ac-1"])
	}
}

func TestGenerate_HeightIsDeterministicAndOrderInvariant(t *testing.T) {
	objs := []Object{
		{Kind: ZoneReference, ID: "spec/a#dc-1", DocOrder: 0, Height: 240},
		{Kind: ZoneReference, ID: "spec/b#ac-1", DocOrder: 1, Height: 200},
		{Kind: ZoneReference, ID: "spec/c#ac-1", DocOrder: 2},
	}
	first, err := Generate(objs, nil)
	if err != nil {
		t.Fatal(err)
	}
	reversed := []Object{objs[2], objs[1], objs[0]}
	second, err := Generate(reversed, nil)
	if err != nil {
		t.Fatal(err)
	}
	for id := range first {
		if first[id] != second[id] {
			t.Errorf("%s: %+v vs %+v under a different input order", id, first[id], second[id])
		}
	}
	assertPairwiseFree(t, heightRectsOf(objs, first))
}
