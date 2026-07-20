// AC-3 (spec/instruction-conformance): a second, independent check
// tripwires the retired two-phase commit-to-design ritual specifically —
// the case AC-2 structurally cannot see, because `board` remains a real,
// dispatched verb. Follows docsync_test.go's handEditPhrasings idiom
// exactly (DC-3): a closed, named set of phrasings that instruct or
// describe `verdi board commit` / a frozen `board.json` as the CURRENT
// step of finishing a design-branch spec, with NO accompanying
// retirement or grandfathered disclosure ANYWHERE in the same file,
// fails the file. A file that both names the command and discloses its
// retired or grandfathered status in the same breath does not trip it.
//
// Like handEditPhrasings, this scans WHOLE FILE TEXT — never limited to
// backtick-delimited spans the way AC-2's verb extraction is — matching
// the obligation's own instruction to mirror docsync_test.go's idiom
// exactly, and matching the real-world fact that a disclosure sentence
// ("verdi board commit is retired now") is ordinary prose, not
// necessarily backtick-wrapped.
//
// Same disclosed limit as handEditPhrasings/ADJ-50, carried verbatim: a
// lexical/substring tripwire, never a semantic guarantee that no future
// paraphrase of either phrase set could evade or falsely trip this rule
// (CO-2).
package specalign

import (
	"fmt"
	"strings"
	"testing"
)

// ritualCurrentPhrasings is the closed, named set of phrasings that
// instruct or describe `verdi board commit` / a frozen `board.json` as an
// ACTIVE, CURRENT step to run (DC-3's element 1). Every entry here MUST
// be a substring capable of matching real prose —
// TestRitualCurrentPhrasingsMatchRealisticSentence proves each one fires
// on a realistic sentence, the ADJ-47/ADJ-50 defect class (a dead,
// unmatchable pattern implying protection it does not actually have)
// this story must not reintroduce.
var ritualCurrentPhrasings = []string{"verdi board commit", "frozen board.json"}

// ritualDisclosurePhrasings is the closed, named set of retirement/
// grandfathered-disclosure phrasings (DC-3's element 2, its own examples
// verbatim). Presence of ANY of these anywhere in the file is what keeps
// DC-4's rewrite-to-disclose alternative achievable: `verdi board commit`
// and `board.json` are real, legitimately-mentionable strings, and a
// presence-only rule (mirroring handEditPhrasings' own unconditional
// shape) would make an honest disclosure structurally impossible to ever
// pass, since explaining a retirement necessarily names what was
// retired.
var ritualDisclosurePhrasings = []string{"retired", "grandfathered", "superseded"}

// findRitualCurrentPhrasing returns the first ritualCurrentPhrasings
// entry present in doc (case-insensitive matching, since normal prose
// capitalizes sentence-initial words), and whether any was found.
func findRitualCurrentPhrasing(doc string) (phrase string, found bool) {
	lower := strings.ToLower(doc)
	for _, p := range ritualCurrentPhrasings {
		if strings.Contains(lower, strings.ToLower(p)) {
			return p, true
		}
	}
	return "", false
}

// hasRitualDisclosure reports whether doc contains ANY
// ritualDisclosurePhrasings entry (case-insensitive), anywhere in the
// file.
func hasRitualDisclosure(doc string) bool {
	lower := strings.ToLower(doc)
	for _, p := range ritualDisclosurePhrasings {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// checkRetiredRitualTripwire is AC-3's own combinator — DC-3's
// conjunction: a file gets a finding only when a ritualCurrentPhrasings
// entry IS present AND no ritualDisclosurePhrasings entry is present
// anywhere in that same file. Returns nil (no finding) otherwise —
// including when a disclosure phrasing appears with NO current-ritual
// phrasing at all, which is vacuously clean: nothing is being taught as
// current, so there is nothing to disclose.
func checkRetiredRitualTripwire(file, content string) *instructionFinding {
	phrase, found := findRitualCurrentPhrasing(content)
	if !found {
		return nil
	}
	if hasRitualDisclosure(content) {
		return nil
	}
	return &instructionFinding{
		File:   file,
		Detail: fmt.Sprintf("teaches %q as a current step with no retirement or grandfathered disclosure anywhere in the file (spec/instruction-conformance ac-3)", phrase),
	}
}

// TestRitualCurrentPhrasingsMatchRealisticSentence and
// TestRitualDisclosurePhrasingsMatchRealisticSentence are the ADJ-47/
// ADJ-50 guard-repair proof this story's own outcome text demands: every
// entry in both closed sets actually fires on a realistic sentence using
// it — never a dead, unmatchable pattern that implies protection it does
// not have. Each test also fails loudly if a set gains an entry with no
// matching proof registered, so the property holds for future entries
// too.
func TestRitualCurrentPhrasingsMatchRealisticSentence(t *testing.T) {
	realisticSentences := map[string]string{
		"verdi board commit": "finish the ritual by running `verdi board commit <board-key> --name <spec-name>`.",
		"frozen board.json":  "a frozen board.json sits beside it once the board is committed.",
	}
	if len(realisticSentences) != len(ritualCurrentPhrasings) {
		t.Fatalf("test table has %d realistic sentence(s), ritualCurrentPhrasings has %d entries — every entry needs its own realistic-sentence proof", len(realisticSentences), len(ritualCurrentPhrasings))
	}
	for _, p := range ritualCurrentPhrasings {
		t.Run(p, func(t *testing.T) {
			sentence, ok := realisticSentences[p]
			if !ok {
				t.Fatalf("no realistic-sentence proof registered for ritualCurrentPhrasings entry %q", p)
			}
			got, found := findRitualCurrentPhrasing(sentence)
			if !found || got != p {
				t.Errorf("findRitualCurrentPhrasing(%q) = (%q, %v), want (%q, true) — this phrasing does not actually match its own realistic-sentence proof", sentence, got, found, p)
			}
		})
	}
}

func TestRitualDisclosurePhrasingsMatchRealisticSentence(t *testing.T) {
	realisticSentences := map[string]string{
		"retired":       "verdi board commit is retired now.",
		"grandfathered": "VL-014 is retained but scoped to grandfathered specs only.",
		"superseded":    "the commit-to-design ritual is superseded by board-as-projection.",
	}
	if len(realisticSentences) != len(ritualDisclosurePhrasings) {
		t.Fatalf("test table has %d realistic sentence(s), ritualDisclosurePhrasings has %d entries — every entry needs its own realistic-sentence proof", len(realisticSentences), len(ritualDisclosurePhrasings))
	}
	for _, p := range ritualDisclosurePhrasings {
		t.Run(p, func(t *testing.T) {
			sentence, ok := realisticSentences[p]
			if !ok {
				t.Fatalf("no realistic-sentence proof registered for ritualDisclosurePhrasings entry %q", p)
			}
			if !hasRitualDisclosure(sentence) {
				t.Errorf("hasRitualDisclosure(%q) = false, want true — %q does not actually match its own realistic-sentence proof", sentence, p)
			}
		})
	}
}

func TestFindRitualCurrentPhrasing(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want string // "" means "not found"
	}{
		{"clean prose, no ritual mention at all", "the workbench renders the corpus and the board.", ""},
		{"case-insensitive match", "Verdi Board Commit is how you used to finish a spec.", "verdi board commit"},
		{"board mentioned without commit is not a match", "the board page shows every sticky.", ""},
		{"board.json mentioned without frozen is not a match", "board.json carries pins, stickies, and yarn.", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := findRitualCurrentPhrasing(tc.doc)
			wantFound := tc.want != ""
			if found != wantFound {
				t.Fatalf("findRitualCurrentPhrasing(%q) found = %v, want %v", tc.doc, found, wantFound)
			}
			if found && got != tc.want {
				t.Fatalf("findRitualCurrentPhrasing(%q) = %q, want %q", tc.doc, got, tc.want)
			}
		})
	}
}

func TestHasRitualDisclosure(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{"no disclosure words", "run verdi board commit to finish.", false},
		{"retired present", "this command is retired.", true},
		{"grandfathered present", "VL-014 is scoped to grandfathered specs.", true},
		{"superseded present", "the ritual is superseded by board-as-projection.", true},
		{"case-insensitive", "This Ritual Is Superseded.", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasRitualDisclosure(tc.doc); got != tc.want {
				t.Errorf("hasRitualDisclosure(%q) = %v, want %v", tc.doc, got, tc.want)
			}
		})
	}
}

// TestCheckRetiredRitualTripwire is AC-3's combinator-level proof of
// DC-3's conjunction, including the "presence-only would wrongly trip"
// negative case the decision's own rationale names: a disclosed mention
// must NOT fire.
func TestCheckRetiredRitualTripwire(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantFinding bool
	}{
		{
			name:        "undisclosed current-ritual phrase fires",
			content:     "finish by running `verdi board commit <board-key> --name <spec-name>`.",
			wantFinding: true,
		},
		{
			name:        "disclosed current-ritual phrase in the same file does not fire",
			content:     "historically you ran `verdi board commit`, but that ritual is retired now.",
			wantFinding: false,
		},
		{
			name:        "no current-ritual phrase at all does not fire, even with disclosure words present",
			content:     "several older CLI verbs were retired or superseded over time.",
			wantFinding: false,
		},
		{
			name:        "disclosure elsewhere in a long file still clears the finding",
			content:     "Step 1: read the board.\n\nStep 2: run `verdi board commit`.\n\n...\n\nNote: this whole ritual is grandfathered — see the migration guide.",
			wantFinding: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkRetiredRitualTripwire("fixture.md", tc.content)
			if (got != nil) != tc.wantFinding {
				t.Errorf("checkRetiredRitualTripwire(%q) = %v, want finding = %v", tc.content, got, tc.wantFinding)
			}
			if got != nil && got.File != "fixture.md" {
				t.Errorf("checkRetiredRitualTripwire(...).File = %q, want %q", got.File, "fixture.md")
			}
		})
	}
}
