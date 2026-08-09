package model

import (
	"reflect"
	"testing"
)

// TestCanonicalSpecLifecycle_AdvancementIsMergeSignaled pins the catalog's
// draft -> accepted-pending-build transition to the MERGE-signaled ritual
// (docs/superpowers/specs/2026-08-01-merge-signals-spec-acceptance-design.md:
// "the successful merge is the state transition"). `verdi accept` is
// retired — cmd/verdi/accept.go prints a notice and mutates nothing — so a
// catalog that still declared `accept` as the advancement verb would make
// every consumer projecting the catalog (notably internal/journey's safe
// actions and blockers) name a retired command as the way forward, which
// GLG v3 dc-3 forbids ("never presents a currently-illegal transition as
// advice"). The obligation is unchanged: dropping a governance requirement
// is not what retiring a command does.
func TestCanonicalSpecLifecycle_AdvancementIsMergeSignaled(t *testing.T) {
	lc := canonicalSpecLifecycle()

	var advancement *Transition
	for i, tr := range lc.Transitions {
		if tr.Verb == "accept" {
			t.Fatalf("canonicalSpecLifecycle still declares the retired verb %q as a transition", tr.Verb)
		}
		if tr.To == "accepted-pending-build" {
			advancement = &lc.Transitions[i]
		}
	}
	if advancement == nil {
		t.Fatal("canonicalSpecLifecycle declares no transition into accepted-pending-build")
	}
	if advancement.Verb != "merge" {
		t.Errorf("advancement transition verb = %q, want %q (a forge transition; GLG v3 dc-3 admits forge transitions)", advancement.Verb, "merge")
	}
	if advancement.From != "draft" {
		t.Errorf("advancement transition from = %q, want %q", advancement.From, "draft")
	}
	wantObligations := []Obligation{{Scheme: "attestation", Kind: "author-vouch"}}
	if !reflect.DeepEqual(advancement.Obligations, wantObligations) {
		t.Errorf("advancement transition obligations = %+v, want %+v (retiring a command never drops a governance requirement)", advancement.Obligations, wantObligations)
	}
}

// TestCanonicalSpecLifecycle_CloseIsUnchanged is the negative twin: `verdi
// close` is a LIVE verb (close.go's closeAcceptedStatusLineRe flip), so the
// merge-signal retirement must not have touched it.
func TestCanonicalSpecLifecycle_CloseIsUnchanged(t *testing.T) {
	lc := canonicalSpecLifecycle()

	var found *Transition
	for i, tr := range lc.Transitions {
		if tr.Verb == "close" {
			found = &lc.Transitions[i]
		}
	}
	if found == nil {
		t.Fatal("canonicalSpecLifecycle declares no close transition")
	}
	if found.From != "accepted-pending-build" || found.To != "closed" {
		t.Errorf("close transition = %s -> %s, want accepted-pending-build -> closed", found.From, found.To)
	}
	wantObligations := []Obligation{
		{Scheme: "attestation", Kind: "countersign", Count: 1},
		{Scheme: "behavioral", Kind: "fold-green"},
	}
	if !reflect.DeepEqual(found.Obligations, wantObligations) {
		t.Errorf("close transition obligations = %+v, want %+v", found.Obligations, wantObligations)
	}
}
