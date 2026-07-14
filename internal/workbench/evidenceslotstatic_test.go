package workbench

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEvidenceSlot_StaticOneReader is evidence-slot ac-1's STATIC
// obligation, workbench half (co-3: "a wall-private record scan or a
// lookalike per-kind reduction is a defect"): the slot surface's own
// files — the badge attachment tier, the pure projector, and the board
// renderer — contain NO record loading, no Current reduction, and no
// attestation probing of their own. Slot state reaches the wall ONLY as
// wallbadge.EmptySlotBadges' output (which internal/wallbadge's own
// TestEmptySlotStaticCallSites pins to the fold's exact seams). The
// same deliberately-minimal source-text witness badgesstatic_test.go
// already established for this package. matrix.go (the advisory preview
// page) legitimately calls the fold's loader — it IS a fold consumer,
// not the wall's card surface — so it is deliberately out of this
// check's scope.
func TestEvidenceSlot_StaticOneReader(t *testing.T) {
	for _, name := range []string{"badges.go", "boardspecrender.go", "projection.go", "boardspec.go"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		src := string(data)
		for _, bad := range []string{
			"LoadRecords",           // the fold's loader — only wallbadge's compute may call it for the wall
			"evidence.Current(",     // the fold's reduction — a wall-local call would be the lookalike co-3 names
			"AttestationExists",     // the fold's attestation probe
			"evidence.RecordsForAC", // the fold's per-AC filter
			"verdicts.json",         // a hardcoded derived-tree layout would be a private scan
			"data/derived",          // likewise the derived-tree path itself
		} {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q — slot state must arrive from wallbadge.EmptySlotBadges, never a wall-private read (evidence-slot co-3)", name, bad)
			}
		}
	}
}

// TestEvidenceSlot_StaticOneRowRenderer is evidence-slot ac-3's STATIC
// obligation: the record-state chip is emitted INSIDE the card's
// existing per-kind obligation row renderer (writeObligations,
// boardspecrender.go) — one renderer producing one row per declared kind
// carrying both obligation content and record state — and no second
// per-kind list renderer exists. The page and the fragment share this
// single code path by construction (renderBoardRegion is the one board
// region renderer; boardspec.go's page and fragment handlers both call
// it — already pinned by badgesstatic_test.go's loadBoard witnesses).
func TestEvidenceSlot_StaticOneRowRenderer(t *testing.T) {
	data, err := os.ReadFile("boardspecrender.go")
	if err != nil {
		t.Fatalf("reading boardspecrender.go: %v", err)
	}
	src := string(data)

	// Exactly ONE per-kind list container in the whole renderer: the
	// obligation column the obligation-wall story shipped, extended in
	// place (dc-2), never duplicated beside itself.
	if got := strings.Count(src, `class="card-obligations"`); got != 1 {
		t.Errorf("boardspecrender.go emits %d card-obligations containers, want exactly 1 (the one per-kind list, ac-3)", got)
	}

	// The slot chip is written from within writeObligations' own row loop
	// (both row forms call writeSlotChip), and nowhere else: no second
	// call site could grow a second per-kind surface.
	obligStart := strings.Index(src, "func writeObligations(")
	obligEnd := strings.Index(src[obligStart:], "\n}\n")
	if obligStart < 0 || obligEnd < 0 {
		t.Fatal("could not locate writeObligations in boardspecrender.go")
	}
	body := src[obligStart : obligStart+obligEnd]
	if got := strings.Count(body, "writeSlotChip(b, c.ID, o)"); got != 2 {
		t.Errorf("writeObligations calls writeSlotChip %d times, want exactly 2 (once per row form: obligated and un-obligated)", got)
	}
	if got := strings.Count(src, "writeSlotChip(b, c.ID, o)"); got != 2 {
		t.Errorf("boardspecrender.go calls writeSlotChip %d times total, want exactly 2 — both inside writeObligations (one row surface, ac-3)", got)
	}
	// The chip markup itself exists only inside writeSlotChip.
	if got := strings.Count(src, `slot-chip slot-chip--`); got != 1 {
		t.Errorf("slot-chip markup emitted from %d sites, want exactly 1 (writeSlotChip)", got)
	}
}

// TestEvidenceSlot_StaticDisclosureNeverConsumed is evidence-slot ac-2's
// STATIC obligation's second half (co-2: "no write path, gate, or lint
// verdict consumes slot state"): the board's entire write-handler
// surface (boardspecapi.go) never reads slot or badge state, and
// internal/lint (every gate-feeding rule) imports neither the badge
// compute package nor the projection that carries slot state. The badge
// is disclosure, never a consumed input.
func TestEvidenceSlot_StaticDisclosureNeverConsumed(t *testing.T) {
	api, err := os.ReadFile("boardspecapi.go")
	if err != nil {
		t.Fatalf("reading boardspecapi.go: %v", err)
	}
	for _, bad := range []string{"Slot", "EvidenceSlots", ".Badges", "slot-chip", "wallbadge"} {
		if strings.Contains(string(api), bad) {
			t.Errorf("boardspecapi.go references %q — a write handler reading slot/badge state (evidence-slot co-2)", bad)
		}
	}

	lintFiles, err := filepath.Glob(filepath.Join("..", "lint", "*.go"))
	if err != nil || len(lintFiles) == 0 {
		t.Fatalf("globbing ../lint: %v (%d files)", err, len(lintFiles))
	}
	for _, f := range lintFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("reading %s: %v", f, err)
		}
		// Quoted import paths only: prose comments legitimately NAME these
		// packages when documenting boundaries.
		for _, bad := range []string{
			`"github.com/jyang234/verdi/internal/wallbadge"`,
			`"github.com/jyang234/verdi/internal/workbench"`,
		} {
			if strings.Contains(string(data), bad) {
				t.Errorf("%s imports %s — a lint rule must never consume slot or badge state (evidence-slot co-2)", f, bad)
			}
		}
	}
}
