package journey

import (
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/specstate"
)

func testOwner() Owner {
	return Owner{Declared: "platform-team", Attribution: governanceprincipal.NewUnauthenticatedAttribution()}
}

// --- candidateTransitions -------------------------------------------------

func TestCandidateTransitions(t *testing.T) {
	mdl := model.Canonical()
	tests := []struct {
		name      string
		state     specstate.State
		wantVerbs []string
	}{
		{"proposed joins draft, candidate is the merge-signaled transition", specstate.Proposed, []string{"merge"}},
		{"accepted-pending-build joins, candidate is close", specstate.AcceptedPendingBuild, []string{"close"}},
		{"closed is terminal: no candidates", specstate.Closed, nil},
		{"superseded is terminal: no candidates", specstate.Superseded, nil},
		{"unproven: no candidates regardless of ArtifactStatus", specstate.Unproven, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, classDeclared := candidateTransitions(mdl, "feature", specstate.Result{State: tt.state})
			if !classDeclared {
				t.Fatalf("classDeclared = false, want true (canonical model declares feature)")
			}
			var verbs []string
			for _, tr := range got {
				verbs = append(verbs, tr.Verb)
			}
			if len(verbs) != len(tt.wantVerbs) {
				t.Fatalf("verbs = %v, want %v", verbs, tt.wantVerbs)
			}
			for i := range verbs {
				if verbs[i] != tt.wantVerbs[i] {
					t.Fatalf("verbs = %v, want %v", verbs, tt.wantVerbs)
				}
			}
		})
	}
}

func TestCandidateTransitions_UnknownClass(t *testing.T) {
	mdl := model.Canonical()
	got, classDeclared := candidateTransitions(mdl, "component", specstate.Result{State: specstate.Proposed})
	if got != nil {
		t.Fatalf("candidateTransitions(component) = %v, want nil (no lifecycle declared)", got)
	}
	if classDeclared {
		t.Fatal("classDeclared = true, want false (component has no declared lifecycle)")
	}
}

// TestCandidateTransitions_NilLifecycleMap proves F4's nil-map safety: a
// *model.Model whose Lifecycle map is nil (never even initialized) reads
// as "declared for nothing" via a plain map read, never a panic.
func TestCandidateTransitions_NilLifecycleMap(t *testing.T) {
	mdl := &model.Model{}
	got, classDeclared := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.Proposed})
	if got != nil || classDeclared {
		t.Fatalf("candidateTransitions(nil Lifecycle) = (%v, %v), want (nil, false)", got, classDeclared)
	}
}

// TestCandidateTransitions_ClassDeclaredIndependentOfState proves
// classDeclared is checked BEFORE state: an undeclared class reports false
// even when state is unproven, never masked by the unproven check.
func TestCandidateTransitions_ClassDeclaredIndependentOfState(t *testing.T) {
	mdl := model.Canonical()
	got, classDeclared := candidateTransitions(mdl, "component", specstate.Result{State: specstate.Unproven})
	if got != nil || classDeclared {
		t.Fatalf("candidateTransitions(component, unproven) = (%v, %v), want (nil, false)", got, classDeclared)
	}
}

func TestCandidateTransitions_SortedByVerb(t *testing.T) {
	mdl := &model.Model{
		Lifecycle: map[string]model.Lifecycle{
			"feature": {
				States: []string{"draft"},
				Transitions: []model.Transition{
					{Verb: "zzz", From: "draft", To: "x"},
					{Verb: "aaa", From: "draft", To: "y"},
				},
			},
		},
	}
	got, classDeclared := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.Proposed})
	if !classDeclared {
		t.Fatal("classDeclared = false, want true")
	}
	if len(got) != 2 || got[0].Verb != "aaa" || got[1].Verb != "zzz" {
		t.Fatalf("got = %+v, want sorted [aaa zzz]", got)
	}
}

// --- obligationReason / obligationBlockerID / transitionHasCountersign ---

func TestObligationReason(t *testing.T) {
	tests := []struct {
		kind       string
		wantReason ReasonCode
	}{
		{"author-vouch", ReasonObligationAuthorVouchUnproven},
		{"countersign", ReasonObligationCountersignUnproven},
		{"fold-green", ReasonObligationFoldGreenUnproven},
		{"hook", ReasonObligationUnknownKind},
		{"", ReasonObligationUnknownKind},
	}
	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			reason, idPrefix := obligationReason(tt.kind)
			if reason != tt.wantReason {
				t.Fatalf("reason = %q, want %q", reason, tt.wantReason)
			}
			if idPrefix == "" {
				t.Fatal("idPrefix must be non-empty")
			}
			if _, err := reason.Class(); err != nil {
				t.Fatalf("reason.Class(): %v", err)
			}
		})
	}
}

func TestObligationBlockerID(t *testing.T) {
	got := obligationBlockerID("obligation-countersign-unproven", "close", "attestation", "countersign")
	want := "obligation-countersign-unproven/close/attestation/countersign"
	if got != want {
		t.Fatalf("obligationBlockerID = %q, want %q", got, want)
	}
	if !blockerIDRe.MatchString(got) {
		t.Fatalf("obligationBlockerID output %q does not match blockerIDRe", got)
	}
}

func TestTransitionHasCountersign(t *testing.T) {
	tests := []struct {
		name string
		tr   model.Transition
		want bool
	}{
		{"has countersign", model.Transition{Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign"}}}, true},
		{"only author-vouch", model.Transition{Obligations: []model.Obligation{{Scheme: "attestation", Kind: "author-vouch"}}}, false},
		{"behavioral countersign name is not attestation-scoped", model.Transition{Obligations: []model.Obligation{{Scheme: "behavioral", Kind: "countersign"}}}, false},
		{"no obligations", model.Transition{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transitionHasCountersign(tt.tr); got != tt.want {
				t.Fatalf("transitionHasCountersign(%+v) = %v, want %v", tt.tr, got, tt.want)
			}
		})
	}
}

// --- deriveBlockers ---------------------------------------------------

func TestDeriveBlockers_DefaultBranchUnresolved(t *testing.T) {
	blockers := deriveBlockers(false, specstate.Result{State: specstate.AcceptedPendingBuild}, nil, testOwner())
	found := findBlocker(blockers, "default-branch-unresolved/unknown")
	if found == nil {
		t.Fatalf("blockers = %+v, want default-branch-unresolved/unknown", blockers)
	}
	if found.Reason != ReasonDefaultBranchUnresolved || found.Class != ClassUnknown || found.Transition != "unknown" {
		t.Fatalf("blocker = %+v", found)
	}
	// F2: the witness names only what was observed — no per-input claims
	// about which of the three resolution steps individually failed.
	wantWitness := "no default branch could be resolved by the resolution chain (CI_DEFAULT_BRANCH, origin/HEAD symbolic ref, lone conventional remote-tracking ref)"
	if len(found.Witnesses) != 1 || found.Witnesses[0] != wantWitness {
		t.Fatalf("Witnesses = %v, want [%q]", found.Witnesses, wantWitness)
	}
	if err := found.validate("blocker"); err != nil {
		t.Fatalf("blocker fails Blocker.validate: %v", err)
	}
}

func TestDeriveBlockers_LifecycleStateUnproven(t *testing.T) {
	t.Run("with disclosures", func(t *testing.T) {
		result := specstate.Result{State: specstate.Unproven, Disclosures: []string{"b", "a", "a"}}
		blockers := deriveBlockers(true, result, nil, testOwner())
		found := findBlocker(blockers, "lifecycle-state-unproven/unknown")
		if found == nil {
			t.Fatal("want lifecycle-state-unproven/unknown blocker")
		}
		if len(found.Witnesses) != 2 || found.Witnesses[0] != "a" || found.Witnesses[1] != "b" {
			t.Fatalf("Witnesses = %v, want sorted/deduped [a b]", found.Witnesses)
		}
	})
	t.Run("without disclosures uses a fixed witness", func(t *testing.T) {
		result := specstate.Result{State: specstate.Unproven}
		blockers := deriveBlockers(true, result, nil, testOwner())
		found := findBlocker(blockers, "lifecycle-state-unproven/unknown")
		if found == nil || len(found.Witnesses) != 1 || found.Witnesses[0] == "" {
			t.Fatalf("blocker = %+v, want exactly one non-empty fixed witness", found)
		}
	})
}

func TestDeriveBlockers_ObligationsAndPrincipalResolution(t *testing.T) {
	mdl := model.Canonical()
	// accepted-pending-build joins to "close", which carries countersign
	// AND fold-green obligations in the canonical model.
	candidates, classDeclared := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.AcceptedPendingBuild})
	if !classDeclared {
		t.Fatal("classDeclared = false, want true")
	}
	blockers := deriveBlockers(true, specstate.Result{State: specstate.AcceptedPendingBuild}, candidates, testOwner())

	wantIDs := []string{
		"obligation-countersign-unproven/close/attestation/countersign",
		"obligation-fold-green-unproven/close/behavioral/fold-green",
		"principal-resolution-unproven/close",
	}
	for _, id := range wantIDs {
		if findBlocker(blockers, id) == nil {
			t.Errorf("blockers missing %q; got %v", id, blockerIDs(blockers))
		}
	}
	if b := findBlocker(blockers, "obligation-countersign-unproven/close/attestation/countersign"); b.Class != ClassGovernance {
		t.Errorf("countersign blocker class = %v, want governance", b.Class)
	}
	if b := findBlocker(blockers, "obligation-fold-green-unproven/close/behavioral/fold-green"); b.Class != ClassMechanical {
		t.Errorf("fold-green blocker class = %v, want mechanical", b.Class)
	}
	if b := findBlocker(blockers, "principal-resolution-unproven/close"); b.Class != ClassGovernance {
		t.Errorf("principal-resolution blocker class = %v, want governance", b.Class)
	}
	// draft-side obligations (author-vouch) must NOT appear: only "close"
	// is a candidate here, not the draft-exit "merge" transition.
	if findBlocker(blockers, "obligation-author-vouch-unproven/merge/attestation/author-vouch") != nil {
		t.Errorf("blockers unexpectedly include the draft-exit obligation blocker")
	}
}

// TestDeriveBlockers_ObligationIDCollisionRegression is F3's collision
// regression test: two DISTINCT obligations sharing the same transition
// verb AND the same kind, but different schemes, must produce TWO
// blockers (both witnessed), never collide into one id and silently lose
// a witness to deriveBlockers' own seen-map dedup.
func TestDeriveBlockers_ObligationIDCollisionRegression(t *testing.T) {
	candidates := []model.Transition{
		{
			Verb: "close", From: "accepted-pending-build", To: "closed",
			Obligations: []model.Obligation{
				{Scheme: "attestation", Kind: "fold-green"},
				{Scheme: "behavioral", Kind: "fold-green"},
			},
		},
	}
	blockers := deriveBlockers(true, specstate.Result{State: specstate.AcceptedPendingBuild}, candidates, testOwner())

	idA := "obligation-fold-green-unproven/close/attestation/fold-green"
	idB := "obligation-fold-green-unproven/close/behavioral/fold-green"
	bA, bB := findBlocker(blockers, idA), findBlocker(blockers, idB)
	if bA == nil || bB == nil {
		t.Fatalf("blockers = %v, want both %q and %q", blockerIDs(blockers), idA, idB)
	}
	if len(bA.Witnesses) == 0 || len(bB.Witnesses) == 0 {
		t.Fatalf("both blockers must carry a witness: %+v / %+v", bA, bB)
	}
	if bA.Witnesses[0] == bB.Witnesses[0] {
		t.Fatalf("witnesses should differ (they name different schemes): %q", bA.Witnesses[0])
	}
}

// TestDeriveBlockers_ObligationIDByteIdenticalDuplicateMerges proves the
// OTHER half of F3: a byte-identical duplicate obligation (same scheme AND
// kind, same transition) still produces exactly ONE blocker — correct
// dedup via the seen-map, not a collision.
func TestDeriveBlockers_ObligationIDByteIdenticalDuplicateMerges(t *testing.T) {
	candidates := []model.Transition{
		{
			Verb: "close", From: "accepted-pending-build", To: "closed",
			Obligations: []model.Obligation{
				{Scheme: "attestation", Kind: "countersign", Count: 1},
				{Scheme: "attestation", Kind: "countersign", Count: 1},
			},
		},
	}
	blockers := deriveBlockers(true, specstate.Result{State: specstate.AcceptedPendingBuild}, candidates, testOwner())
	count := 0
	for _, b := range blockers {
		if b.ID == "obligation-countersign-unproven/close/attestation/countersign" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d blockers for the byte-identical duplicate obligation, want exactly 1", count)
	}
}

func TestDeriveBlockers_ProposedForgeFacts(t *testing.T) {
	mdl := model.Canonical()
	candidates, _ := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.Proposed})
	blockers := deriveBlockers(true, specstate.Result{State: specstate.Proposed}, candidates, testOwner())

	// The draft-exit transition is merge-signaled (docs/superpowers/specs/
	// 2026-08-01-merge-signals-spec-acceptance-design.md), so the
	// forge-facts gap is attributed to "merge" — the forge transition
	// that actually advances the spec — never to the retired `accept`.
	found := findBlocker(blockers, "forge-facts-unavailable/merge")
	if found == nil {
		t.Fatalf("blockers = %v, want forge-facts-unavailable/merge", blockerIDs(blockers))
	}
	if found.Reason != ReasonForgeFactsUnavailable || found.Class != ClassExternalWait {
		t.Fatalf("blocker = %+v", found)
	}
}

func TestDeriveBlockers_ProposedForgeFacts_NoCandidatesFallsBackToUnknown(t *testing.T) {
	// A model with no draft-exit transition at all: the
	// draft-exit verb cannot be named, so the blocker's transition is the
	// literal "unknown" rather than a guess.
	blockers := deriveBlockers(true, specstate.Result{State: specstate.Proposed}, nil, testOwner())
	found := findBlocker(blockers, "forge-facts-unavailable/unknown")
	if found == nil {
		t.Fatalf("blockers = %v, want forge-facts-unavailable/unknown", blockerIDs(blockers))
	}
}

func TestDeriveBlockers_TerminalState_Empty(t *testing.T) {
	mdl := model.Canonical()
	candidates, _ := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.Closed})
	blockers := deriveBlockers(true, specstate.Result{State: specstate.Closed}, candidates, testOwner())
	if len(blockers) != 0 {
		t.Fatalf("terminal-state blockers = %v, want none", blockerIDs(blockers))
	}
}

func TestDeriveBlockers_SortedAscendingByID(t *testing.T) {
	mdl := model.Canonical()
	candidates, _ := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.AcceptedPendingBuild})
	blockers := deriveBlockers(false, specstate.Result{State: specstate.AcceptedPendingBuild}, candidates, testOwner())
	if len(blockers) < 2 {
		t.Fatalf("want multiple blockers to prove ordering, got %v", blockerIDs(blockers))
	}
	ids := blockerIDs(blockers)
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("blocker ids not ascending: %v", ids)
	}
}

func findBlocker(blockers []Blocker, id string) *Blocker {
	for i := range blockers {
		if blockers[i].ID == id {
			return &blockers[i]
		}
	}
	return nil
}

func blockerIDs(blockers []Blocker) []string {
	ids := make([]string, len(blockers))
	for i, b := range blockers {
		ids[i] = b.ID
	}
	return ids
}

// --- deriveEventual -----------------------------------------------------

func TestDeriveEventual(t *testing.T) {
	eb := deriveEventual()
	if eb.Derived {
		t.Fatal("Derived must be false")
	}
	if len(eb.Items) != 0 {
		t.Fatalf("Items = %v, want empty", eb.Items)
	}
	if len(eb.Disclosures) == 0 {
		t.Fatal("Disclosures must be non-empty for an underived section")
	}
	if err := eb.validate(); err != nil {
		t.Fatalf("EventualBlockers.validate: %v", err)
	}
}

// --- derivePrincipals ---------------------------------------------------

func TestDerivePrincipals_CanonicalModel(t *testing.T) {
	mdl := model.Canonical()

	t.Run("merge candidate: author-vouch only", func(t *testing.T) {
		candidates, _ := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.Proposed})
		pf := derivePrincipals(candidates)
		if len(pf.Required) != 1 {
			t.Fatalf("Required = %+v, want exactly 1", pf.Required)
		}
		rr := pf.Required[0]
		if rr.Transition != "merge" || rr.Obligation != "attestation/author-vouch" || rr.Count != 1 || rr.Resolution != "unproven" {
			t.Fatalf("RequiredRole = %+v", rr)
		}
	})

	t.Run("close candidate: countersign only (fold-green is behavioral, excluded)", func(t *testing.T) {
		candidates, _ := candidateTransitions(mdl, "feature", specstate.Result{State: specstate.AcceptedPendingBuild})
		pf := derivePrincipals(candidates)
		if len(pf.Required) != 1 {
			t.Fatalf("Required = %+v, want exactly 1 (fold-green must be excluded)", pf.Required)
		}
		rr := pf.Required[0]
		if rr.Transition != "close" || rr.Obligation != "attestation/countersign" || rr.Count != 1 {
			t.Fatalf("RequiredRole = %+v", rr)
		}
		// The canonical model's own countersign obligation carries an
		// explicit Count: 1 (canonical.go), so no F5 assumption-disclosure
		// should fire here.
		if containsString(pf.Disclosures, "countersign count is unstated in the operating model for transition close; a minimum of 1 is assumed") {
			t.Fatalf("unexpected F5 disclosure for a model that DOES state its count: %v", pf.Disclosures)
		}
	})

	t.Run("no candidates: empty, non-nil, disclosed", func(t *testing.T) {
		pf := derivePrincipals(nil)
		if pf.Required == nil || len(pf.Required) != 0 {
			t.Fatalf("Required = %v, want empty non-nil", pf.Required)
		}
		if pf.ProfileAdopted {
			t.Fatal("ProfileAdopted must be false")
		}
		if len(pf.Disclosures) == 0 {
			t.Fatal("Disclosures must be non-empty")
		}
		if err := pf.validate(); err != nil {
			t.Fatalf("PrincipalFacts.validate: %v", err)
		}
	})
}

func TestDerivePrincipals_SortedByTransitionThenObligation(t *testing.T) {
	candidates := []model.Transition{
		{Verb: "zzz", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "author-vouch"}}},
		{Verb: "aaa", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 2}}},
	}
	pf := derivePrincipals(candidates)
	if len(pf.Required) != 2 || pf.Required[0].Transition != "aaa" || pf.Required[1].Transition != "zzz" {
		t.Fatalf("Required = %+v, want sorted by transition", pf.Required)
	}
	if pf.Required[0].Count != 2 {
		t.Fatalf("countersign Count = %d, want the obligation's own Count (2)", pf.Required[0].Count)
	}
}

// --- F5: unstated countersign count -------------------------------------

func TestDerivePrincipals_CountersignCountUnstated(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"zero", 0},
		{"negative", -3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := []model.Transition{
				{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: tt.count}}},
			}
			pf := derivePrincipals(candidates)
			if len(pf.Required) != 1 || pf.Required[0].Count != 1 {
				t.Fatalf("Required = %+v, want exactly one entry with Count 1", pf.Required)
			}
			want := "countersign count is unstated in the operating model for transition close; a minimum of 1 is assumed"
			if !containsString(pf.Disclosures, want) {
				t.Fatalf("Disclosures = %v, want to contain %q", pf.Disclosures, want)
			}
		})
	}
}

func TestDerivePrincipals_CountersignCountStated_NoDisclosure(t *testing.T) {
	candidates := []model.Transition{
		{Verb: "close", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "countersign", Count: 2}}},
	}
	pf := derivePrincipals(candidates)
	if len(pf.Required) != 1 || pf.Required[0].Count != 2 {
		t.Fatalf("Required = %+v, want exactly one entry with Count 2", pf.Required)
	}
	for _, d := range pf.Disclosures {
		if d != "no governance profile is adopted at the evaluated revision; role and approver requirements beyond the operating-model obligations are unknown" {
			t.Fatalf("unexpected disclosure for a model that DOES state its count: %v", pf.Disclosures)
		}
	}
}

// --- F6: dedup by (transition, obligation), keeping the max count -------

func TestDerivePrincipals_DedupesDuplicateObligationsKeepingMaxCount(t *testing.T) {
	candidates := []model.Transition{
		{
			Verb: "close",
			Obligations: []model.Obligation{
				{Scheme: "attestation", Kind: "countersign", Count: 1},
				{Scheme: "attestation", Kind: "countersign", Count: 3},
			},
		},
	}
	pf := derivePrincipals(candidates)
	if len(pf.Required) != 1 {
		t.Fatalf("Required = %+v, want exactly 1 (deduped by (transition, obligation))", pf.Required)
	}
	if pf.Required[0].Count != 3 {
		t.Fatalf("Count = %d, want 3 (the max of the two duplicate obligations)", pf.Required[0].Count)
	}
	if err := pf.validate(); err != nil {
		t.Fatalf("PrincipalFacts.validate: %v (a Validate-clean duplicate-obligation model must never abort the projection)", err)
	}
}

func TestDerivePrincipals_DedupedAuthorVouchAlsoMerges(t *testing.T) {
	candidates := []model.Transition{
		{
			Verb: "merge",
			Obligations: []model.Obligation{
				{Scheme: "attestation", Kind: "author-vouch"},
				{Scheme: "attestation", Kind: "author-vouch"},
			},
		},
	}
	pf := derivePrincipals(candidates)
	if len(pf.Required) != 1 {
		t.Fatalf("Required = %+v, want exactly 1", pf.Required)
	}
}

// --- deriveActions ------------------------------------------------------

func TestDeriveActions_Unproven(t *testing.T) {
	actions := deriveActions("feature", specstate.Result{State: specstate.Unproven}, nil, true, "spec/x")
	if len(actions.Safe) != 0 {
		t.Fatalf("Safe = %v, want none", actions.Safe)
	}
	if !containsString(actions.NeededFacts, "lifecycle state is unproven; no transition's from-state can be established") {
		t.Fatalf("NeededFacts = %v", actions.NeededFacts)
	}
}

// TestDeriveActions_ClassNotDeclared is F4's test: a class absent from
// Model.Lifecycle yields no Safe actions and a NeededFacts entry naming
// the gap — never silently falling through to the unproven-state message.
func TestDeriveActions_ClassNotDeclared(t *testing.T) {
	actions := deriveActions("component", specstate.Result{State: specstate.Proposed}, nil, false, "spec/x")
	if len(actions.Safe) != 0 {
		t.Fatalf("Safe = %v, want none", actions.Safe)
	}
	want := "the operating model declares no lifecycle for class component; its transitions are unknown"
	if !containsString(actions.NeededFacts, want) {
		t.Fatalf("NeededFacts = %v, want to contain %q", actions.NeededFacts, want)
	}
	if containsString(actions.NeededFacts, "lifecycle state is unproven; no transition's from-state can be established") {
		t.Fatalf("NeededFacts unexpectedly also carries the unproven-state message: %v", actions.NeededFacts)
	}
}

func TestDeriveActions_CanonicalModel_AllHaveObligations_SafeEmpty(t *testing.T) {
	mdl := model.Canonical()
	for _, state := range []specstate.State{specstate.Proposed, specstate.AcceptedPendingBuild} {
		candidates, classDeclared := candidateTransitions(mdl, "feature", specstate.Result{State: state})
		actions := deriveActions("feature", specstate.Result{State: state}, candidates, classDeclared, "spec/x")
		if len(actions.Safe) != 0 {
			t.Fatalf("state %v: Safe = %v, want empty (every canonical transition carries an obligation)", state, actions.Safe)
		}
		if len(actions.NeededFacts) == 0 {
			t.Fatalf("state %v: NeededFacts = %v, want at least one obligation named", state, actions.NeededFacts)
		}
	}
}

// TestDeriveActions_ZeroObligationTransition_EmitsSafeAction proves the
// positive emission path with an in-memory model.Model fixture whose
// transition carries Obligations: []Obligation{} — constructed as a Go
// value (DecodeModel's frontier check would reject a non-canonical model
// like this one at decode time, but derivation consumes the already-
// decoded value, never re-validating it against the frontier).
func TestDeriveActions_ZeroObligationTransition_EmitsSafeAction(t *testing.T) {
	candidates := []model.Transition{
		{Verb: "advance", From: "draft", To: "accepted-pending-build", Obligations: []model.Obligation{}},
	}
	actions := deriveActions("feature", specstate.Result{State: specstate.Proposed}, candidates, true, "spec/x")

	if len(actions.Safe) != 1 {
		t.Fatalf("Safe = %+v, want exactly one action", actions.Safe)
	}
	a := actions.Safe[0]
	if a.ID != "advance" || a.Verb != "advance" || a.FromState != "draft" || a.ToState != "accepted-pending-build" {
		t.Fatalf("action = %+v", a)
	}
	if len(a.Arguments) != 1 || a.Arguments[0] != "spec/x" {
		t.Fatalf("Arguments = %v", a.Arguments)
	}
	if a.Confirmation != "none" {
		t.Fatalf("Confirmation = %q, want none", a.Confirmation)
	}
	if len(a.Authority) != 0 {
		t.Fatalf("Authority = %v, want empty (zero obligations)", a.Authority)
	}
	wantPre := []string{"lifecycle-state-matches", "transition-registered"}
	if len(a.Preconditions) != 2 || a.Preconditions[0].ID != wantPre[0] || a.Preconditions[1].ID != wantPre[1] {
		t.Fatalf("Preconditions = %+v", a.Preconditions)
	}
	for _, p := range a.Preconditions {
		if p.Witness == "" {
			t.Fatalf("precondition %q has an empty witness", p.ID)
		}
	}
	if err := a.validate("action"); err != nil {
		t.Fatalf("Action.validate: %v", err)
	}
	if len(actions.NeededFacts) != 0 {
		t.Fatalf("NeededFacts = %v, want none (the only candidate was safely emitted)", actions.NeededFacts)
	}
}

func TestDeriveActions_MixedObligationAndObligationFree(t *testing.T) {
	candidates := []model.Transition{
		{Verb: "free", From: "draft", To: "mid", Obligations: []model.Obligation{}},
		{Verb: "gated", From: "draft", To: "other", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "author-vouch"}}},
	}
	actions := deriveActions("feature", specstate.Result{State: specstate.Proposed}, candidates, true, "spec/x")
	if len(actions.Safe) != 1 || actions.Safe[0].Verb != "free" {
		t.Fatalf("Safe = %+v, want exactly [free]", actions.Safe)
	}
	if !containsString(actions.NeededFacts, "obligation attestation/author-vouch for transition gated is unproven") {
		t.Fatalf("NeededFacts = %v", actions.NeededFacts)
	}
}

// TestDeriveActions_EveryEmittedVerbInCatalog is the property test: every
// action Verb this package can emit, across a spread of synthetic models
// and states, actually names a transition present in the supplied catalog.
func TestDeriveActions_EveryEmittedVerbInCatalog(t *testing.T) {
	mdl := &model.Model{
		Lifecycle: map[string]model.Lifecycle{
			"feature": {
				States: []string{"draft", "mid", "done"},
				Transitions: []model.Transition{
					{Verb: "advance", From: "draft", To: "mid", Obligations: []model.Obligation{}},
					{Verb: "finish", From: "mid", To: "done", Obligations: []model.Obligation{{Scheme: "attestation", Kind: "author-vouch"}}},
				},
			},
		},
	}
	catalogVerbs := map[string]bool{}
	for _, tr := range mdl.Lifecycle["feature"].Transitions {
		catalogVerbs[tr.Verb] = true
	}

	for _, state := range []specstate.State{specstate.Proposed, specstate.AcceptedPendingBuild, specstate.Closed} {
		result := specstate.Result{State: state}
		var effective string
		switch state {
		case specstate.Proposed:
			effective = "draft"
		case specstate.AcceptedPendingBuild:
			effective = "mid"
		}
		candidates := filterTransitionsByFrom(mdl.Lifecycle["feature"].Transitions, effective)
		actions := deriveActions("feature", result, candidates, true, "spec/x")
		for _, a := range actions.Safe {
			if !catalogVerbs[a.Verb] {
				t.Fatalf("emitted action verb %q is not in the supplied catalog %v", a.Verb, catalogVerbs)
			}
		}
	}
}

func filterTransitionsByFrom(trs []model.Transition, from string) []model.Transition {
	if from == "" {
		return nil
	}
	var out []model.Transition
	for _, tr := range trs {
		if tr.From == from {
			out = append(out, tr)
		}
	}
	return out
}
