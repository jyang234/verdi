// docsync_test.go proves spec/disposition-verb ac-4 (static evidence,
// obligation/disposition-verb--ac-4--static): verdi/docs/architecture-and-
// journeys.md's closure-ritual narrative names `verdi disposition` as the
// sanctioned way to record a deviation-report finding's disposition, and no
// sentence in the document describes or instructs a hand-edit of a report's
// disposition fields as an accepted practice — the round-6 D6-25 flow this
// story retires.
package specalign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestArchitectureDoc_NamesDispositionVerb reads
// verdi/docs/architecture-and-journeys.md (present in every checkout of the
// verdi repo alone, unlike the workspace-sibling docs/design/specs/ fidelity
// gate — no skip path needed here) and asserts:
//
//  1. The closure-ritual narrative — "D — The build loop" (which already
//     names "every finding gets dispositioned") through "E — The closure
//     ritual" — contains the literal string "verdi disposition".
//  2. No sentence anywhere in the document describes or instructs a
//     hand-edit of a deviation report's disposition fields as a sanctioned
//     step (the D6-25 flow this story's whole point is to retire).
func TestArchitectureDoc_NamesDispositionVerb(t *testing.T) {
	path := filepath.Join(verdiRepoRoot, "docs", "architecture-and-journeys.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	doc := string(data)

	const startMarker = "### D — The build loop"
	const endMarker = "### F — The agent journey"
	start := strings.Index(doc, startMarker)
	end := strings.Index(doc, endMarker)
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("could not locate the closure-ritual narrative (between %q and %q) in %s", startMarker, endMarker, path)
	}
	narrative := doc[start:end]

	if !strings.Contains(narrative, "verdi disposition") {
		t.Errorf("the %q..%q narrative does not name `verdi disposition` as the mechanism that records a finding's disposition (spec/disposition-verb ac-4):\n%s", startMarker, endMarker, narrative)
	}

	lower := strings.ToLower(doc)
	for _, phrase := range []string{"hand-edit", "hand edit", "edit ... by hand", "edited by hand", "editing ... by hand"} {
		if strings.Contains(lower, phrase) {
			t.Errorf("document contains %q — a deviation report's disposition fields must never be described as hand-edited (spec/disposition-verb ac-4 retires the D6-25 flow)", phrase)
		}
	}
}
