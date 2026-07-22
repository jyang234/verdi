package initwizard

import (
	"reflect"
	"testing"
)

// TestRenameableIDs_MatchesCanonical proves the interview's own menu of
// renameable class/state/verb ids is derived from model.Canonical() —
// never a hand-hardcoded literal list that could silently drift from the
// canonical model — and lands sorted (CLAUDE.md: deterministic outputs).
// "spike" is included among the renameable classes even though it is not
// a Model.Classes key: it is the one pseudo-class Vocabulary.Classes
// legally renames (internal/model/validate.go's vocabularySpikePseudoClass,
// the L-M13-ratified carve).
func TestRenameableIDs_MatchesCanonical(t *testing.T) {
	ids := RenameableIDs()

	wantClasses := []string{"feature", "spike", "story"}
	if !reflect.DeepEqual(ids.Classes, wantClasses) {
		t.Fatalf("RenameableIDs().Classes = %v, want %v", ids.Classes, wantClasses)
	}

	wantStates := []string{"accepted-pending-build", "closed", "draft", "superseded"}
	if !reflect.DeepEqual(ids.States, wantStates) {
		t.Fatalf("RenameableIDs().States = %v, want %v", ids.States, wantStates)
	}

	wantVerbs := []string{"accept", "close"}
	if !reflect.DeepEqual(ids.Verbs, wantVerbs) {
		t.Fatalf("RenameableIDs().Verbs = %v, want %v", ids.Verbs, wantVerbs)
	}
}

// TestRenameableIDs_Deterministic proves two calls agree byte-for-byte —
// map iteration order must never leak into the returned slices.
func TestRenameableIDs_Deterministic(t *testing.T) {
	a := RenameableIDs()
	b := RenameableIDs()
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("RenameableIDs() is non-deterministic across calls: %+v vs %+v", a, b)
	}
}
