package boardlayout

import (
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/OWNER/verdi/internal/artifact"
)

func sampleObjects() []Object {
	return []Object{
		{Kind: ZoneAC, ID: "ac-1", DocOrder: 0},
		{Kind: ZoneAC, ID: "ac-2", DocOrder: 1},
		{Kind: ZoneAC, ID: "ac-3", DocOrder: 2},
		{Kind: ZoneConstraint, ID: "co-1", DocOrder: 0},
		{Kind: ZoneDecision, ID: "dc-1", DocOrder: 0},
		{Kind: ZoneDecision, ID: "dc-2", DocOrder: 1},
		{Kind: ZoneReference, ID: "adr/0001-outbox-events", DocOrder: 0},
	}
}

func sampleStored() map[string]artifact.Position {
	return map[string]artifact.Position{
		"ac-1": {X: 40, Y: 60},
		"dc-1": {X: 613, Y: 217.5},
	}
}

// Property 1 + 4 (S8): same inputs → identical output, run twice, and
// invariant to input construction order.
func TestGenerate_DeterministicAndOrderInvariant(t *testing.T) {
	first, err := Generate(sampleObjects(), sampleStored())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	second, err := Generate(sampleObjects(), sampleStored())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("run-twice mismatch:\n%v\n%v", first, second)
	}

	rng := rand.New(rand.NewSource(1)) // fixed seed: the TEST is deterministic; the shuffles exercise input order
	for trial := 0; trial < 20; trial++ {
		objs := sampleObjects()
		rng.Shuffle(len(objs), func(i, j int) { objs[i], objs[j] = objs[j], objs[i] })
		got, err := Generate(objs, sampleStored())
		if err != nil {
			t.Fatalf("Generate (trial %d): %v", trial, err)
		}
		if !reflect.DeepEqual(first, got) {
			t.Fatalf("trial %d: shuffled input changed the layout", trial)
		}
	}
}

// Property 2 (S8): stored positions pass through verbatim — including two
// stored positions that collide.
func TestGenerate_StoredVerbatimEvenColliding(t *testing.T) {
	stored := map[string]artifact.Position{
		"ac-1": {X: 100, Y: 100},
		"ac-2": {X: 100, Y: 100}, // collision: kept as-is, never "fixed"
	}
	got, err := Generate(sampleObjects(), stored)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for id, want := range stored {
		if got[id] != want {
			t.Errorf("stored %s = %v, want verbatim %v", id, got[id], want)
		}
	}
}

// Property 3 (S8): adding a new object — the board always appends in
// document order — never moves any previously placed object.
func TestGenerate_AddOneNeverMovesOthers(t *testing.T) {
	before, err := Generate(sampleObjects(), sampleStored())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	added := append(sampleObjects(), Object{Kind: ZoneConstraint, ID: "co-2", DocOrder: 1})
	after, err := Generate(added, sampleStored())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for id, p := range before {
		if after[id] != p {
			t.Errorf("adding co-2 moved %s: %v -> %v", id, p, after[id])
		}
	}
	if _, ok := after["co-2"]; !ok {
		t.Fatal("co-2 was not placed")
	}
	// The new object landed on a free pixel.
	seen := map[artifact.Position][]string{}
	for id, p := range after {
		seen[p] = append(seen[p], id)
	}
	for p, ids := range seen {
		if len(ids) > 1 {
			t.Errorf("pixel %v shared by %v (no stored collision was seeded here)", p, ids)
		}
	}
}

// The adjudicated policy: orphaned stored entries are ignored by
// generation (slot counter from currently-occupied slots), so a freed
// slot IS deterministically reused.
func TestGenerate_OrphanIgnored_FreedSlotReused(t *testing.T) {
	objs := []Object{
		{Kind: ZoneAC, ID: "ac-2", DocOrder: 1},
	}
	stored := map[string]artifact.Position{
		// ac-1 was deleted from the spec: its entry is an orphan.
		"ac-1": {X: 40, Y: 40},
	}
	got, err := Generate(objs, stored)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, ok := got["ac-1"]; ok {
		t.Error("orphan ac-1 was emitted")
	}
	// ac-2 reuses the freed first slot: the orphan neither occupies its
	// pixel nor raises the counter.
	if want := (artifact.Position{X: 40, Y: 40}); got["ac-2"] != want {
		t.Errorf("ac-2 = %v, want freed slot %v", got["ac-2"], want)
	}
}

func TestGenerate_UnknownKindFailsClosed(t *testing.T) {
	_, err := Generate([]Object{{Kind: "story", ID: "st-1"}}, nil)
	if err == nil {
		t.Fatal("Generate accepted an unknown kind")
	}
}

// Occupied slots are skipped: a stored position sitting exactly on a grid
// slot routes an unstored object around it.
func TestGenerate_OccupiedSlotSkipped(t *testing.T) {
	objs := []Object{
		{Kind: ZoneAC, ID: "ac-1", DocOrder: 0},
		{Kind: ZoneAC, ID: "ac-2", DocOrder: 1},
	}
	stored := map[string]artifact.Position{
		"ac-1": {X: 40, Y: 40}, // exactly slot 0 of the AC zone
	}
	got, err := Generate(objs, stored)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got["ac-2"] == got["ac-1"] {
		t.Fatalf("ac-2 landed on ac-1's occupied pixel %v", got["ac-1"])
	}
}

func TestPrune(t *testing.T) {
	stored := map[string]artifact.Position{
		"ac-1": {X: 1, Y: 2},
		"zz-9": {X: 3, Y: 4},
	}
	got := Prune(stored, map[string]bool{"ac-1": true})
	if len(got) != 1 {
		t.Fatalf("Prune kept %d entries, want 1", len(got))
	}
	if _, ok := got["zz-9"]; ok {
		t.Error("Prune kept the orphan")
	}
}

func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Missing file: empty map, no error.
	got, err := ReadFile(dir)
	if err != nil {
		t.Fatalf("ReadFile (missing): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReadFile (missing) = %v, want empty", got)
	}

	positions := map[string]artifact.Position{
		"ac-1": {X: 40, Y: 60},
		"zz-9": {X: 1, Y: 1}, // orphan: must be pruned on write
	}
	live := map[string]bool{"ac-1": true}
	if err := WriteFile(dir, positions, live); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Canonical bytes: sorted keys, trailing newline, and write-twice
	// byte-identity.
	data1, err := os.ReadFile(FilePath(dir))
	if err != nil {
		t.Fatalf("reading layout.json: %v", err)
	}
	want := `{"positions":{"ac-1":{"x":40,"y":60}},"schema":"verdi.boardlayout/v1"}` + "\n"
	if string(data1) != want {
		t.Fatalf("layout.json = %q, want %q", data1, want)
	}
	if err := WriteFile(dir, positions, live); err != nil {
		t.Fatalf("WriteFile (second): %v", err)
	}
	data2, _ := os.ReadFile(FilePath(dir))
	if string(data1) != string(data2) {
		t.Fatal("write-twice bytes differ")
	}

	back, err := ReadFile(dir)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !reflect.DeepEqual(back, map[string]artifact.Position{"ac-1": {X: 40, Y: 60}}) {
		t.Fatalf("round-trip = %v", back)
	}

	// No temp litter.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "layout.json" {
			t.Errorf("unexpected file %s left behind", e.Name())
		}
	}
}

func TestReadFile_Negative(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "layout.json"), []byte(`{"schema":"wrong/v9","positions":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(dir); err == nil {
		t.Fatal("ReadFile accepted a wrong schema")
	}
}

// ZoneColumns is the board's zone-label geometry: fixed order, one band
// per zone, bands aligned with where Generate actually slots unstored
// cards (the label must sit over its own column).
func TestZoneColumns(t *testing.T) {
	cols := ZoneColumns()
	// The stubs band sits between open questions and references
	// (spec/scoping-canvas dc-6: "a dedicated kind-locked stubs zone ...
	// between open questions and references"). AMENDED with the scoping
	// canvas: the previous order had no ZoneStub; references stay
	// outermost (dc-6's non-binding house style, kept deliberately).
	wantOrder := []ZoneKind{ZoneAC, ZoneConstraint, ZoneDecision, ZoneOpenQuestion, ZoneStub, ZoneReference}
	if len(cols) != len(wantOrder) {
		t.Fatalf("ZoneColumns() has %d entries, want %d", len(cols), len(wantOrder))
	}
	for i, c := range cols {
		if c.Kind != wantOrder[i] {
			t.Errorf("cols[%d].Kind = %s, want %s", i, c.Kind, wantOrder[i])
		}
		if c.Width != CardWidth {
			t.Errorf("cols[%d].Width = %d, want CardWidth", i, c.Width)
		}
	}
	// The band origin agrees with Generate's slotting: an unstored card
	// of each kind lands at its column's X.
	for i, c := range cols {
		objs := []Object{{Kind: c.Kind, ID: "z-1", DocOrder: 0}}
		got, err := Generate(objs, nil)
		if err != nil {
			t.Fatalf("Generate(%s): %v", c.Kind, err)
		}
		if got["z-1"].X != float64(c.X) {
			t.Errorf("zone %d (%s): label X %d but cards land at %v", i, c.Kind, c.X, got["z-1"].X)
		}
	}
	// Pure function of constants: two calls agree.
	again := ZoneColumns()
	for i := range cols {
		if cols[i] != again[i] {
			t.Errorf("ZoneColumns() not stable at %d: %+v vs %+v", i, cols[i], again[i])
		}
	}
}

// The scratch lane sits one band past the object zones and stays
// OUTSIDE the object layout: Generate must keep failing closed on
// ZoneScratch (no spec object ever files there).
func TestScratchColumn(t *testing.T) {
	cols := ZoneColumns()
	sc := ScratchColumn()
	if sc.Kind != ZoneScratch {
		t.Errorf("ScratchColumn().Kind = %s, want %s", sc.Kind, ZoneScratch)
	}
	last := cols[len(cols)-1]
	if sc.X <= last.X {
		t.Errorf("scratch lane X %d not past the last object zone (%d)", sc.X, last.X)
	}
	for _, c := range cols {
		if c.Kind == ZoneScratch {
			t.Error("ZoneScratch leaked into ZoneColumns — objects could be slotted there")
		}
	}
	if _, err := Generate([]Object{{Kind: ZoneScratch, ID: "x-1"}}, nil); err == nil {
		t.Error("Generate accepted a ZoneScratch object; must fail closed")
	}
}

// The stubs zone (spec/scoping-canvas dc-6): a real layout zone —
// Generate slots stub cards there like any kind — with its own squat
// footprint (stub cards are typeset claims, not full object cards), and
// the S8 properties hold across it: determinism and add-never-reflows.
func TestStubZone(t *testing.T) {
	w, h := FootprintFor(ZoneStub)
	if w != CardWidth {
		t.Errorf("stub footprint width = %v, want CardWidth", w)
	}
	if h != StubCardHeight {
		t.Errorf("stub footprint height = %v, want StubCardHeight", h)
	}
	if h >= CardHeight {
		t.Errorf("StubCardHeight %v not squatter than an object card (%v) — the stub card must read as its own paper", h, CardHeight)
	}

	// Two stubs slot deterministically down their own column; adding a
	// third never moves the first two (S8 property 3).
	objs := []Object{
		{Kind: ZoneStub, ID: "stub:alpha", DocOrder: 0},
		{Kind: ZoneStub, ID: "stub:beta", DocOrder: 1},
	}
	first, err := Generate(objs, nil)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	again, err := Generate(objs, nil)
	if err != nil {
		t.Fatalf("Generate (again): %v", err)
	}
	for id := range first {
		if first[id] != again[id] {
			t.Errorf("stub layout not deterministic for %s: %v vs %v", id, first[id], again[id])
		}
	}
	grown, err := Generate(append(objs, Object{Kind: ZoneStub, ID: "stub:gamma", DocOrder: 2}), nil)
	if err != nil {
		t.Fatalf("Generate (grown): %v", err)
	}
	for _, id := range []string{"stub:alpha", "stub:beta"} {
		if grown[id] != first[id] {
			t.Errorf("adding a stub moved %s: %v -> %v", id, first[id], grown[id])
		}
	}
	if first["stub:alpha"] == first["stub:beta"] {
		t.Error("two stubs share one slot")
	}
}
