package boardlayout

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

// The regression fixture's geometry (owner UAT, R4-I-35):
// testdata/corpus/.verdi/specs/active/accepted-pending-build/layout.json
// stores ac-1 at (40,20) and ac-2 at (220,20) — a 20px footprint overlap
// under the uniform CardWidth=200 — plus dc-1 at (40,180), clear of both.
// Positions saved before the footprint enlargement can collide like this
// in any store; the spec is accepted (read-only board), so a drag can
// never repair it.
func collidingFixtureObjects() []Object {
	return []Object{
		{Kind: ZoneAC, ID: "ac-1", DocOrder: 0},
		{Kind: ZoneAC, ID: "ac-2", DocOrder: 1},
		{Kind: ZoneAC, ID: "ac-3", DocOrder: 2},
		{Kind: ZoneDecision, ID: "dc-1", DocOrder: 0},
		{Kind: ZoneReference, ID: "adr/0001-outbox-events", DocOrder: 0},
	}
}

func collidingFixtureStored() map[string]artifact.Position {
	return map[string]artifact.Position{
		"ac-1": {X: 40, Y: 20},
		"ac-2": {X: 220, Y: 20},
		"dc-1": {X: 40, Y: 180},
	}
}

// rectsOf materializes every object's rendered footprint at its resolved
// position, for pairwise overlap checks.
func rectsOf(t *testing.T, objects []Object, positions map[string]artifact.Position) map[string]Rect {
	t.Helper()
	out := make(map[string]Rect, len(objects))
	for _, o := range objects {
		p, ok := positions[o.ID]
		if !ok {
			t.Fatalf("no position for %s", o.ID)
		}
		w, h := FootprintFor(o.Kind)
		out[o.ID] = Rect{X: p.X, Y: p.Y, W: w, H: h}
	}
	return out
}

func assertPairwiseDisjoint(t *testing.T, rects map[string]Rect) {
	t.Helper()
	ids := make([]string, 0, len(rects))
	for id := range rects {
		ids = append(ids, id)
	}
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if rects[ids[i]].intersects(rects[ids[j]]) {
				t.Errorf("rendered cards %s and %s overlap: %+v vs %+v",
					ids[i], ids[j], rects[ids[i]], rects[ids[j]])
			}
		}
	}
}

// The owner directive: cards must never render stacked, in any mode. The
// first claimant (canonical order: zone, DocOrder, ID) keeps its stored
// position verbatim; the later collider is nudged; the clear stored card
// stays verbatim; unstored objects slot around all of it.
func TestResolveDisplayOverlaps_CollidingFixture(t *testing.T) {
	objects := collidingFixtureObjects()
	stored := collidingFixtureStored()
	got, err := ResolveDisplayOverlaps(objects, stored)
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}

	// First claimant keeps its stored position verbatim.
	if want := (artifact.Position{X: 40, Y: 20}); got["ac-1"] != want {
		t.Errorf("ac-1 = %v, want stored verbatim %v", got["ac-1"], want)
	}
	// The later collider was nudged off the contested footprint.
	if got["ac-2"] == (artifact.Position{X: 220, Y: 20}) {
		t.Error("ac-2 still renders at its stored colliding position")
	}
	// A stored position that collides with nothing stays verbatim.
	if want := (artifact.Position{X: 40, Y: 180}); got["dc-1"] != want {
		t.Errorf("dc-1 = %v, want stored verbatim %v", got["dc-1"], want)
	}
	// And nothing on the board overlaps anything else.
	assertPairwiseDisjoint(t, rectsOf(t, objects, got))
}

// Pure function: same inputs → same board, and input construction order
// never leaks into the result (the projection is reload-deterministic).
func TestResolveDisplayOverlaps_DeterministicAndOrderInvariant(t *testing.T) {
	first, err := ResolveDisplayOverlaps(collidingFixtureObjects(), collidingFixtureStored())
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}
	rng := rand.New(rand.NewSource(2)) // fixed seed: the TEST is deterministic; shuffles exercise input order
	for trial := 0; trial < 20; trial++ {
		objs := collidingFixtureObjects()
		rng.Shuffle(len(objs), func(i, j int) { objs[i], objs[j] = objs[j], objs[i] })
		got, err := ResolveDisplayOverlaps(objs, collidingFixtureStored())
		if err != nil {
			t.Fatalf("trial %d: %v", trial, err)
		}
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("trial %d: shuffled input changed the display layout", trial)
		}
	}
}

// Collision-free inputs pass through untouched: display resolution is
// exactly Generate's board whenever the stored positions already honor
// the footprint.
func TestResolveDisplayOverlaps_NoCollisionMatchesGenerate(t *testing.T) {
	objects := collidingFixtureObjects()
	stored := map[string]artifact.Position{
		"ac-1": {X: 40, Y: 20},
		"ac-2": {X: 260, Y: 20}, // clear of ac-1's 200px span
		"dc-1": {X: 40, Y: 180},
	}
	want, err := Generate(objects, stored)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	got, err := ResolveDisplayOverlaps(objects, stored)
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("collision-free display layout diverged from Generate:\n%v\n%v", want, got)
	}
}

// Two stored positions on the identical pixel: the canonical order's
// first claimant (lower DocOrder) keeps it; ties beyond that fall to ID.
func TestResolveDisplayOverlaps_FirstClaimantByCanonicalOrder(t *testing.T) {
	objects := []Object{
		{Kind: ZoneAC, ID: "ac-1", DocOrder: 0},
		{Kind: ZoneAC, ID: "ac-2", DocOrder: 1},
	}
	stored := map[string]artifact.Position{
		"ac-1": {X: 100, Y: 100},
		"ac-2": {X: 100, Y: 100},
	}
	got, err := ResolveDisplayOverlaps(objects, stored)
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}
	if want := (artifact.Position{X: 100, Y: 100}); got["ac-1"] != want {
		t.Errorf("first claimant ac-1 = %v, want %v", got["ac-1"], want)
	}
	if got["ac-2"] == got["ac-1"] {
		t.Error("ac-2 still stacked on ac-1")
	}
	assertPairwiseDisjoint(t, rectsOf(t, objects, got))
}

// Add-never-reflows survives display resolution: appending a new object
// (always unstored — only a drag writes positions) changes no existing
// object's RENDERED position, even with a collision being resolved.
func TestResolveDisplayOverlaps_AddOneNeverMovesOthers(t *testing.T) {
	before, err := ResolveDisplayOverlaps(collidingFixtureObjects(), collidingFixtureStored())
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}
	added := append(collidingFixtureObjects(), Object{Kind: ZoneOpenQuestion, ID: "oq-1", DocOrder: 0})
	after, err := ResolveDisplayOverlaps(added, collidingFixtureStored())
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps (added): %v", err)
	}
	for id, p := range before {
		if after[id] != p {
			t.Errorf("adding oq-1 moved %s's rendered position: %v -> %v", id, p, after[id])
		}
	}
	if _, ok := after["oq-1"]; !ok {
		t.Fatal("oq-1 was not placed")
	}
	assertPairwiseDisjoint(t, rectsOf(t, added, after))
}

// TestResolveDisplayOverlaps_StubParticipates proves round 5.5's dc-6
// amendment holds at display time too (owner directive R4-I-35): a stored
// stub position colliding with a stored object's footprint is resolved
// with the SAME machinery and canonical order as an object-object
// collision — the stub zone sorts after open-question in zoneIndex, so
// when a stub's stored spot collides with an earlier-zone object, the
// object (the earlier claimant) keeps its stored position and the stub is
// the one nudged.
func TestResolveDisplayOverlaps_StubParticipates(t *testing.T) {
	objects := []Object{
		{Kind: ZoneAC, ID: "ac-1", DocOrder: 0},
		{Kind: ZoneStub, ID: "stub:alpha", DocOrder: 0},
	}
	stored := map[string]artifact.Position{
		"ac-1":       {X: 100, Y: 100},
		"stub:alpha": {X: 100, Y: 100}, // squarely on ac-1's footprint
	}
	got, err := ResolveDisplayOverlaps(objects, stored)
	if err != nil {
		t.Fatalf("ResolveDisplayOverlaps: %v", err)
	}
	if want := (artifact.Position{X: 100, Y: 100}); got["ac-1"] != want {
		t.Errorf("ac-1 (earlier zone, first claimant) = %v, want stored verbatim %v", got["ac-1"], want)
	}
	if got["stub:alpha"] == (artifact.Position{X: 100, Y: 100}) {
		t.Error("stub:alpha still renders stacked on ac-1's stored position")
	}
	assertPairwiseDisjoint(t, rectsOf(t, objects, got))
}

// Negative paths: unknown kinds fail closed (CLAUDE.md), and orphaned
// stored entries (keys naming no live object) neither render nor claim
// a footprint — the same adjudicated policy Generate follows.
func TestResolveDisplayOverlaps_Negative(t *testing.T) {
	t.Run("unknown kind fails closed", func(t *testing.T) {
		if _, err := ResolveDisplayOverlaps([]Object{{Kind: "story", ID: "st-1"}}, nil); err == nil {
			t.Fatal("ResolveDisplayOverlaps accepted an unknown kind")
		}
	})
	t.Run("orphan stored entry ignored", func(t *testing.T) {
		objects := []Object{{Kind: ZoneAC, ID: "ac-2", DocOrder: 1}}
		stored := map[string]artifact.Position{
			"ac-1": {X: 40, Y: 40}, // deleted object's orphan entry
		}
		got, err := ResolveDisplayOverlaps(objects, stored)
		if err != nil {
			t.Fatalf("ResolveDisplayOverlaps: %v", err)
		}
		if _, ok := got["ac-1"]; ok {
			t.Error("orphan ac-1 was emitted")
		}
		if want := (artifact.Position{X: 40, Y: 40}); got["ac-2"] != want {
			t.Errorf("ac-2 = %v, want the freed slot %v (orphan claims nothing)", got["ac-2"], want)
		}
	})
}
