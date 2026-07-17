package main

import (
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/model"
)

// TestCanonicalModel_VerbsMatchDispatch is spec/model-schema ac-2's verb
// half: the embedded canonical model's transition verb set (the union
// across every lifecycle it declares) must equal dispatch.go's own
// SpecTransitionVerbs() set exactly, through exported facts on both
// sides — never reflection on dispatch.go's private verbPhase map.
// Drift on either side (a new status-flipping verb added to cmd/verdi
// with no matching model transition, or a model transition naming a
// verb dispatch.go does not recognize) fails this test.
func TestCanonicalModel_VerbsMatchDispatch(t *testing.T) {
	canonical := model.Canonical()
	if len(canonical.Lifecycle) == 0 {
		t.Fatal("model.Canonical().Lifecycle is empty — nothing to compare")
	}

	verbSet := map[string]bool{}
	for _, lc := range canonical.Lifecycle {
		for _, tr := range lc.Transitions {
			verbSet[tr.Verb] = true
		}
	}
	gotVerbs := make([]string, 0, len(verbSet))
	for v := range verbSet {
		gotVerbs = append(gotVerbs, v)
	}
	sort.Strings(gotVerbs)

	wantVerbs := SpecTransitionVerbs()

	if len(gotVerbs) != len(wantVerbs) {
		t.Fatalf("canonical model's transition verbs = %v, dispatch.go's SpecTransitionVerbs() = %v", gotVerbs, wantVerbs)
	}
	for i := range gotVerbs {
		if gotVerbs[i] != wantVerbs[i] {
			t.Fatalf("canonical model's transition verbs = %v, dispatch.go's SpecTransitionVerbs() = %v", gotVerbs, wantVerbs)
		}
	}

	// Every named verb must also be one dispatch.go actually recognizes
	// (verbPhase) — a model transition naming a verb the binary has
	// never heard of would otherwise pass the set-equality check above
	// only by both sides independently drifting the same way.
	for _, v := range gotVerbs {
		if _, known := verbPhase[v]; !known {
			t.Fatalf("canonical model transition verb %q is not a verb dispatch.go recognizes (verbPhase)", v)
		}
	}
}
