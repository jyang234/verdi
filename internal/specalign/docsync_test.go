// docsync_test.go proves spec/disposition-verb ac-4 (static evidence,
// obligation/disposition-verb--ac-4--static): verdi/docs/architecture-and-
// journeys.md's closure-ritual narrative names `verdi disposition` as the
// sanctioned way to record a deviation-report finding's disposition.
//
// Controller adjudication ADJ-50 (2026-07-16): the original guard (five
// substring tripwires against hand-edit language) claimed more than it
// checked — two of the five ("edit ... by hand", "editing ... by hand")
// contained a literal " ... " placeholder that can never match real prose,
// so they were dead assertions implying protection the guard never actually
// had (the ADJ-47 honesty-defect class: a check that lies about what it
// covers, no minimum size). Repaired here: the dead patterns are removed,
// the three meaningful phrasings are kept in the named handEditPhrasings
// set, TestFindHandEditPhrasing proves each one actually fires (closing the
// exact defect class — an untested tripwire whose match logic was never
// itself verified), and the doc-facing test is renamed/re-commented to
// claim exactly what it checks. This IS a substring tripwire for the COMMON
// hand-edit phrasings, not a semantic guarantee that no paraphrase could
// ever describe hand-editing — that residual (a differently-worded future
// regression slipping past a substring check) is ADJ-50's accepted
// deviation: string matching cannot parse intent, so the common case is
// tripwired here and the rest stays a review concern, never a silently
// overclaimed guarantee.
package specalign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handEditPhrasings is the closed set of common phrasings that, if present
// in the document, describe or instruct a hand-edit of a deviation report's
// disposition fields (the round-6 D6-25 flow this story retires). Every
// entry here MUST be a substring capable of matching real prose — the
// ADJ-50 defect was two entries containing a literal " ... " placeholder
// that no real sentence would ever contain, a dead assertion implying dead
// protection. TestFindHandEditPhrasing proves each entry fires on a
// realistic sentence using it; a future entry that fails that proof is the
// same defect class recurring.
var handEditPhrasings = []string{"hand-edit", "hand edit", "edited by hand"}

// findHandEditPhrasing returns the first handEditPhrasings entry that
// appears in doc (case-insensitive matching, since normal prose capitalizes
// sentence-initial words), and whether any did.
func findHandEditPhrasing(doc string) (phrase string, found bool) {
	lower := strings.ToLower(doc)
	for _, p := range handEditPhrasings {
		if strings.Contains(lower, p) {
			return p, true
		}
	}
	return "", false
}

// TestFindHandEditPhrasing is the guard-repair proof ADJ-50 demanded: every
// entry in handEditPhrasings actually matches a realistic sentence using
// it (the property the original five-phrase list never itself verified,
// letting two dead, unmatchable patterns hide as if they were protection),
// clean prose matches none of them, and matching is case-insensitive.
func TestFindHandEditPhrasing(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string // "" means "not found"
	}{
		{
			name: "clean prose naming the sanctioned verb",
			doc:  "every finding gets dispositioned by verdi disposition, never a hand-authored edit of the report.",
			want: "",
		},
		{
			name: "hyphenated hand-edit",
			doc:  "round 6 recorded every disposition as a hand-edit of deviation-report.md.",
			want: "hand-edit",
		},
		{
			name: "spaced hand edit",
			doc:  "operators used to hand edit the frontmatter directly.",
			want: "hand edit",
		},
		{
			name: "edited by hand",
			doc:  "the disposition fields were edited by hand before this verb existed.",
			want: "edited by hand",
		},
		{
			name: "case-insensitive match",
			doc:  "Hand-Edit is no longer sanctioned practice.",
			want: "hand-edit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := findHandEditPhrasing(tc.doc)
			wantFound := tc.want != ""
			if found != wantFound {
				t.Fatalf("findHandEditPhrasing(%q) found = %v, want %v", tc.doc, found, wantFound)
			}
			if found && got != tc.want {
				t.Fatalf("findHandEditPhrasing(%q) = %q, want %q", tc.doc, got, tc.want)
			}
		})
	}
}

// TestArchitectureDoc_DispositionMechanismDocumented reads
// verdi/docs/architecture-and-journeys.md (present in every checkout of the
// verdi repo alone, unlike the workspace-sibling docs/design/specs/ fidelity
// gate — no skip path needed here) and asserts exactly two things, each its
// own named subtest so a failure names precisely which claim broke:
//
//  1. names_verdi_disposition_as_the_sanctioned_mechanism: the closure-ritual
//     narrative — "D — The build loop" (which already names "every finding
//     gets dispositioned") through "E — The closure ritual" — contains the
//     literal string "verdi disposition": the positive assertion ac-4
//     requires.
//  2. tripwires_common_hand_edit_phrasing: the document contains none of
//     handEditPhrasings' common hand-edit phrasings — a tripwire for the
//     D6-25 flow this story retires, NOT a semantic proof that no sentence
//     anywhere could describe hand-editing in different words (ADJ-50's
//     disclosed, accepted residual).
func TestArchitectureDoc_DispositionMechanismDocumented(t *testing.T) {
	path := filepath.Join(verdiRepoRoot, "docs", "architecture-and-journeys.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	doc := string(data)

	t.Run("names_verdi_disposition_as_the_sanctioned_mechanism", func(t *testing.T) {
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
	})

	t.Run("tripwires_common_hand_edit_phrasing", func(t *testing.T) {
		if phrase, found := findHandEditPhrasing(doc); found {
			t.Errorf("document contains %q — a deviation report's disposition fields must never be described as hand-edited (spec/disposition-verb ac-4 retires the D6-25 flow)", phrase)
		}
	})
}
