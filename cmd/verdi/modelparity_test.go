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

	// Every named verb that is a VERDI verb must also be one dispatch.go
	// actually recognizes (verbPhase) — a model transition naming a
	// command the binary has never heard of would otherwise pass the
	// set-equality check above only by both sides independently drifting
	// the same way. A FORGE transition (SpecTransitionForgeVerbs — today
	// `merge`, the merge-signaled acceptance ritual) is deliberately
	// exempt: no Verdi command performs it, and GLG v3 dc-3 admits forge
	// transitions alongside Verdi verbs.
	forge := map[string]bool{}
	for _, v := range SpecTransitionForgeVerbs() {
		forge[v] = true
	}
	for _, v := range gotVerbs {
		if forge[v] {
			if _, known := verbPhase[v]; known {
				t.Fatalf("catalog verb %q is declared a FORGE transition but dispatch.go also recognizes it as a CLI verb — one of the two inventories is wrong", v)
			}
			continue
		}
		if _, known := verbPhase[v]; !known {
			t.Fatalf("canonical model transition verb %q is not a verb dispatch.go recognizes (verbPhase)", v)
		}
	}
}

// TestSpecTransitionForgeVerbs_SubsetOfTransitionVerbs pins the two
// inventories' relationship: every declared forge transition must itself
// be one of the catalog's transition verbs, so the exemption above can
// never quietly excuse a verb the catalog does not even declare.
func TestSpecTransitionForgeVerbs_SubsetOfTransitionVerbs(t *testing.T) {
	all := map[string]bool{}
	for _, v := range SpecTransitionVerbs() {
		all[v] = true
	}
	for _, v := range SpecTransitionForgeVerbs() {
		if !all[v] {
			t.Errorf("forge transition %q is not one of SpecTransitionVerbs() = %v", v, SpecTransitionVerbs())
		}
	}
}

// TestAcceptRemainsARecognizedCLIVerb is the negative twin of the catalog
// change: retiring `accept` from the CATALOG (acceptance is merge-signaled)
// must not remove it from the CLI-verb inventory — accept.go still prints
// its compatibility notice, and specalign/showcasealign still count it.
func TestAcceptRemainsARecognizedCLIVerb(t *testing.T) {
	if _, known := verbPhase["accept"]; !known {
		t.Fatal("dispatch.go no longer recognizes `accept`; the CLI-verb inventory must be unchanged by the catalog's merge-signaled transition")
	}
	for _, v := range SpecTransitionVerbs() {
		if v == "accept" {
			t.Fatal("`accept` is back in the catalog's transition verbs; acceptance is merge-signaled and accept.go flips nothing")
		}
	}
}
