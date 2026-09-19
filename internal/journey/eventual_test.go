package journey

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/evidence"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/policyconflict"
	"github.com/jyang234/verdi/internal/specstate"
)

// indexBlockers maps blockers by ID for by-ID assertions in the tests
// below — the eventual-blocker sibling of derive_test.go's blockerIDs.
func indexBlockers(blockers []Blocker) map[string]Blocker {
	out := make(map[string]Blocker, len(blockers))
	for _, b := range blockers {
		out[b.ID] = b
	}
	return out
}

// statusOf joins a specstate.State to the lifecycle state id the record's
// derivation reads, through specstate's OWN mapping (DC-15) — exactly as
// project.go does, so these tests can never drift from the real join.
func statusOf(s specstate.State) string {
	return string(specstate.Result{State: s}.ArtifactStatus())
}

// canonicalCandidates returns class's real candidate transitions at state
// under the embedded canonical model — the same candidateTransitions call
// ProjectWith makes, so a test's "later" expectation is measured against
// the real immediate set rather than a hand-typed one.
func canonicalCandidates(t *testing.T, class string, s specstate.State) []model.Transition {
	t.Helper()
	candidates, _ := candidateTransitions(model.Canonical(), class, specstate.Result{State: s})
	return candidates
}

func transitionVerbs(trs []model.Transition) []string {
	out := make([]string, 0, len(trs))
	for _, tr := range trs {
		out = append(out, tr.Verb)
	}
	return out
}

// TestLaterTransitions pins R-RR1-11 (ledger SI-207) directly: "later"
// transitions are the ones FORWARD-REACHABLE from the target's current
// lifecycle state, minus the immediate candidates — never a transition
// already behind the state. The superseded resolution (a) (every declared
// transition minus the candidates) would return [merge] for an accepted
// spec; this table is what refuses it.
func TestLaterTransitions(t *testing.T) {
	mdl := model.Canonical()
	tests := []struct {
		name  string
		class string
		state specstate.State
		want  []string
	}{
		{"proposed feature: close is still ahead", "feature", specstate.Proposed, []string{"close"}},
		{"accepted feature: nothing is ahead but the candidate close", "feature", specstate.AcceptedPendingBuild, nil},
		{"closed feature: the whole lifecycle is behind it", "feature", specstate.Closed, nil},
		{"superseded feature: a terminal state reaches nothing", "feature", specstate.Superseded, nil},
		{"proposed story: the story lifecycle behaves identically", "story", specstate.Proposed, []string{"close"}},
		{"accepted story", "story", specstate.AcceptedPendingBuild, nil},
		{"unproven state: no from-state, so nothing is reachable", "feature", specstate.Unproven, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := canonicalCandidates(t, tt.class, tt.state)
			got := transitionVerbs(laterTransitions(mdl, tt.class, statusOf(tt.state), candidates))
			want := tt.want
			if want == nil {
				want = []string{}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("laterTransitions(%s @ %s) = %v, want %v", tt.class, statusOf(tt.state), got, want)
			}
			// No later transition may repeat a candidate verb: a current
			// blocker and an eventual one would otherwise collide on the
			// obligation id's own verb segment.
			for _, tr := range laterTransitions(mdl, tt.class, statusOf(tt.state), candidates) {
				for _, c := range candidates {
					if tr.Verb == c.Verb {
						t.Fatalf("later transition %q is also a candidate", tr.Verb)
					}
				}
			}
		})
	}
}

// TestLaterTransitions_UndeclaredClass proves a class the operating model
// declares no lifecycle for yields no later transitions at all — never a
// panic and never an invented verb.
func TestLaterTransitions_UndeclaredClass(t *testing.T) {
	if got := laterTransitions(model.Canonical(), "widget", "draft", nil); got != nil {
		t.Fatalf("laterTransitions(widget) = %v, want nil", got)
	}
	if got := laterTransitions(nil, "feature", "draft", nil); got != nil {
		t.Fatalf("laterTransitions(nil model) = %v, want nil", got)
	}
}

// TestResolveEventualScope pins R-RR1-12's verb table (ledger SI-207):
// the CLOSURE verb is resolved from the lifecycle as the transition whose
// target is the closed state, and the POLICY verb is the acceptance
// transition (resolved as the transition whose target is the accepted
// state, never the hard-coded literal "merge") while acceptance is still
// forward-reachable, the closure verb once it is behind. The literal
// "unknown" is never a verb.
func TestResolveEventualScope(t *testing.T) {
	tests := []struct {
		name           string
		mdl            *model.Model
		class          string
		state          specstate.State
		wantClosure    string
		wantPolicy     string
		wantResolved   bool
		wantDisclosure string
	}{
		{
			name: "proposed feature: acceptance is still ahead", mdl: model.Canonical(), class: "feature", state: specstate.Proposed,
			wantClosure: "close", wantPolicy: "merge", wantResolved: true,
		},
		{
			name: "accepted feature: acceptance is behind, so policy is consumed at close", mdl: model.Canonical(), class: "feature", state: specstate.AcceptedPendingBuild,
			wantClosure: "close", wantPolicy: "close", wantResolved: true,
		},
		{
			name: "closed feature", mdl: model.Canonical(), class: "feature", state: specstate.Closed,
			wantClosure: "close", wantPolicy: "close", wantResolved: true,
		},
		{
			name: "proposed story", mdl: model.Canonical(), class: "story", state: specstate.Proposed,
			wantClosure: "close", wantPolicy: "merge", wantResolved: true,
		},
		{
			name: "unproven state: the verbs still resolve, but the narrowing is disclosed", mdl: model.Canonical(), class: "feature", state: specstate.Unproven,
			wantClosure: "close", wantPolicy: "close", wantResolved: true,
			wantDisclosure: "is not a declared state",
		},
		{
			name: "undeclared class: nothing resolves and the gap is disclosed", mdl: model.Canonical(), class: "widget", state: specstate.Proposed,
			wantResolved: false, wantDisclosure: "declares no closure transition",
		},
		{
			name: "nil model", mdl: nil, class: "feature", state: specstate.Proposed,
			wantResolved: false, wantDisclosure: "declares no closure transition",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEventualScope(tt.mdl, tt.class, statusOf(tt.state))
			if got.resolved != tt.wantResolved {
				t.Fatalf("resolved = %v, want %v (scope = %+v)", got.resolved, tt.wantResolved, got)
			}
			if got.closureVerb != tt.wantClosure || got.policyVerb != tt.wantPolicy {
				t.Fatalf("closure/policy = %q/%q, want %q/%q", got.closureVerb, got.policyVerb, tt.wantClosure, tt.wantPolicy)
			}
			if tt.wantDisclosure == "" {
				if got.disclosure != "" {
					t.Fatalf("disclosure = %q, want none", got.disclosure)
				}
				return
			}
			if !strings.Contains(got.disclosure, tt.wantDisclosure) {
				t.Fatalf("disclosure = %q, want it to contain %q", got.disclosure, tt.wantDisclosure)
			}
		})
	}
}

func TestDeriveEventual_StubsAndFloor(t *testing.T) {
	owner := Owner{Declared: "platform-team", Attribution: governanceprincipal.NewUnauthenticatedAttribution()}
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: owner,
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}, {Slug: "core", Bucket: evidence.StubRealized, RealizedBy: []string{"spec/core"}}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false, DeclaresAttestation: true, Attestation: evidence.AttestationAbsent}}, {ID: "ac-2", Floor: evidence.FloorResult{Satisfied: true}}}},
	}
	eb := deriveEventual(in)
	if !eb.Derived || len(eb.Disclosures) != 1 || !strings.Contains(eb.Disclosures[0], "no policy-conflict report") {
		t.Fatalf("eventual = %+v", eb)
	}
	ids := blockerIDs(eb.Items)
	want := []string{"outcome-floor/ac-1", "stub-unreconciled/board-tab"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	byID := indexBlockers(eb.Items)
	if b := byID["stub-unreconciled/board-tab"]; b.Reason != ReasonStubUnreconciled || b.Class != ClassMechanical || b.Transition != "close" || !strings.Contains(b.ClearingCondition, "board-tab") || len(b.Witnesses) == 0 {
		t.Fatalf("stub blocker = %+v", b)
	}
	if b := byID["outcome-floor/ac-1"]; b.Reason != ReasonOutcomeFloorUnsatisfied || b.Class != ClassJudgmental || !strings.Contains(b.ClearingCondition, "attestations/") {
		t.Fatalf("floor blocker = %+v", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_DigitLeadingStubSlug is C1's regression: a stub slug
// may legally begin with a DIGIT (internal/artifact's simpleNameRe,
// ^[a-z0-9]+(?:-[a-z0-9]+)*$), while a journey blocker id segment must
// begin with a LETTER (record.go's blockerIDRe). An unnormalized slug
// therefore produced a record that failed its own validation, and
// `verdi journey` exited 2 on a legal store.
func TestDeriveEventual_DigitLeadingStubSlug(t *testing.T) {
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: testOwner(),
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{
			{Slug: "2fa-login", Bucket: evidence.StubUnreconciled},
			{Slug: "3ds-checkout", Bucket: evidence.StubUnreconciled},
		}},
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)
	want := []string{"stub-unreconciled/s-2fa-login", "stub-unreconciled/s-3ds-checkout"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for _, b := range eb.Items {
		if !blockerIDRe.MatchString(b.ID) {
			t.Fatalf("blocker id %q does not match the journey grammar", b.ID)
		}
	}
	// The un-normalized slug must still be NAMED — the id is normalized
	// for the grammar, never for the operator.
	byID := indexBlockers(eb.Items)
	if b := byID["stub-unreconciled/s-2fa-login"]; !strings.Contains(b.ClearingCondition, "2fa-login") || !strings.Contains(b.Witnesses[0], "2fa-login") {
		t.Fatalf("blocker = %+v, want the raw slug named in its witness and clearing condition", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_DuplicateBlockerIDDisclosed proves the one thing
// CO-1 forbids cannot happen: when two derived debts resolve to the SAME
// blocker id (here a stub slug "2fa" normalizing onto the legal slug
// "s-2fa"), the second is never silently dropped — its loss is disclosed.
func TestDeriveEventual_DuplicateBlockerIDDisclosed(t *testing.T) {
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: testOwner(),
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{
			{Slug: "2fa", Bucket: evidence.StubUnreconciled},
			{Slug: "s-2fa", Bucket: evidence.StubUnreconciled},
		}},
	}
	eb := deriveEventual(in)
	if got := blockerIDs(eb.Items); !reflect.DeepEqual(got, []string{"stub-unreconciled/s-2fa"}) {
		t.Fatalf("ids = %v, want exactly one item", got)
	}
	found := false
	for _, d := range eb.Disclosures {
		if strings.Contains(d, "stub-unreconciled/s-2fa") {
			found = true
		}
	}
	if !found {
		t.Fatalf("disclosures = %v, want one naming the dropped duplicate id", eb.Disclosures)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_ClaimedQuestions proves the question-claimed-by-spike
// source (R-RR1-1): a declared open question named by a spike stub's
// resolves edge is a debt (the spike's own resolution is what will clear
// it); an open question no spike stub claims is NOT a journey blocker at
// all — it stays the readiness-shape concern it already is, so it is
// never counted twice (the plan's own R-RR1-1 note).
func TestDeriveEventual_ClaimedQuestions(t *testing.T) {
	owner := testOwner()
	spec := &artifact.SpecFrontmatter{
		OpenQuestions: []artifact.OpenQuestion{
			{ID: "oq-1", Text: "which route?", Anchor: "#oq-1"},
			{ID: "oq-2", Text: "unclaimed question", Anchor: "#oq-2"},
		},
		Stubs: []artifact.Stub{
			{Slug: "s", Spike: true, Resolves: []string{"oq-1"}},
		},
	}
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: owner,
		Spec: spec,
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)
	want := []string{"question-claimed/oq-1"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v (an unclaimed open question must produce NO item)", ids, want)
	}
	byID := indexBlockers(eb.Items)
	b := byID["question-claimed/oq-1"]
	if b.Reason != ReasonQuestionClaimedBySpike || b.Class != ClassMechanical || b.Transition != "close" {
		t.Fatalf("claimed-question blocker = %+v", b)
	}
	found := false
	for _, w := range b.Witnesses {
		if strings.Contains(w, "spike stub s claims oq-1") {
			found = true
		}
	}
	if !found {
		t.Fatalf("witnesses = %v, want one reading %q", b.Witnesses, "spike stub s claims oq-1")
	}
	if !strings.Contains(b.ClearingCondition, "before close") {
		t.Fatalf("clearing condition = %q, want it to name the closure verb", b.ClearingCondition)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_LaterTransitionObligations proves the obligation and
// principal sources bound to a NON-candidate later transition, over the
// REAL canonical lifecycle: a proposed feature's candidate is merge and
// its one later transition is close, so the obligations that gate close
// (countersign, fold-green) plus its principal resolution are eventual
// debts, while merge's own author-vouch obligation stays a CURRENT
// concern and is never duplicated here.
func TestDeriveEventual_LaterTransitionObligations(t *testing.T) {
	owner := testOwner()
	mdl := model.Canonical()
	candidates := canonicalCandidates(t, "feature", specstate.Proposed)
	later := laterTransitions(mdl, "feature", statusOf(specstate.Proposed), candidates)
	if len(later) != 1 || later[0].Verb != "close" {
		t.Fatalf("later = %v, want exactly [close]", transitionVerbs(later))
	}
	in := eventualInput{
		Class:      "feature",
		Model:      mdl,
		State:      statusOf(specstate.Proposed),
		Owner:      owner,
		Candidates: candidates,
		// The candidate is included AGAIN here (defensively) to prove a
		// later transition that duplicates a candidate is skipped, never
		// double-derived.
		LaterTransitions: append(append([]model.Transition(nil), candidates...), later...),
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)

	wantIDs := []string{
		"obligation-countersign-unproven/close/attestation/countersign",
		"obligation-fold-green-unproven/close/behavioral/fold-green",
		"principal-resolution-unproven/close",
	}
	for _, id := range wantIDs {
		found := false
		for _, got := range ids {
			if got == id {
				found = true
			}
		}
		if !found {
			t.Errorf("items = %v, want to contain %q", ids, id)
		}
	}
	unwantedID := "obligation-author-vouch-unproven/merge/attestation/author-vouch"
	for _, got := range ids {
		if got == unwantedID {
			t.Errorf("items = %v, must NOT duplicate the candidate transition's own obligation as an eventual item", ids)
		}
	}
	byID := indexBlockers(eb.Items)
	if b := byID["obligation-countersign-unproven/close/attestation/countersign"]; b.Class != ClassGovernance || b.Transition != "close" {
		t.Fatalf("later countersign obligation blocker = %+v", b)
	}
	if b := byID["obligation-fold-green-unproven/close/behavioral/fold-green"]; b.Class != ClassMechanical || b.Transition != "close" {
		t.Fatalf("later fold-green obligation blocker = %+v", b)
	}
	if b := byID["principal-resolution-unproven/close"]; b.Class != ClassGovernance || b.Transition != "close" {
		t.Fatalf("later principal-resolution blocker = %+v", b)
	}
	if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
		t.Fatal(err)
	}
}

// TestDeriveEventual_AcceptedFeatureCarriesNoObligationDebt is R-RR1-11's
// refusal, stated as an outcome: an ACCEPTED feature's only forward
// transition is its own immediate candidate (close), so it carries no
// later-transition obligation at all — and above all never merge's, the
// transition it has already made.
func TestDeriveEventual_AcceptedFeatureCarriesNoObligationDebt(t *testing.T) {
	mdl := model.Canonical()
	state := statusOf(specstate.AcceptedPendingBuild)
	candidates := canonicalCandidates(t, "feature", specstate.AcceptedPendingBuild)
	in := eventualInput{
		Class: "feature", Model: mdl, State: state, Owner: testOwner(),
		Candidates:       candidates,
		LaterTransitions: laterTransitions(mdl, "feature", state, candidates),
		Fold:             &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
	}
	eb := deriveEventual(in)
	if got := blockerIDs(eb.Items); !reflect.DeepEqual(got, []string{"outcome-floor/ac-1"}) {
		t.Fatalf("items = %v, want only the outcome-floor debt: an accepted feature has no later transition", got)
	}
	if b := indexBlockers(eb.Items)["outcome-floor/ac-1"]; b.Transition != "close" {
		t.Fatalf("outcome-floor transition = %q, want close (the gate that consumes it)", b.Transition)
	}
}

// TestDeriveEventual_ConflictReport proves the three conflict-report
// sources (mechanical, semantic, exemption): each fires only when the
// report is supplied (R-RR1-2), only for a non-proven row/resolution, and
// a nil report yields none of them plus the fixed disclosure. The verb
// each names is R-RR1-12's policy verb: the acceptance transition while
// acceptance is still ahead, the closure verb once it is behind.
func TestDeriveEventual_ConflictReport(t *testing.T) {
	owner := testOwner()
	report := &policyconflict.Report{
		Mechanical: []policyconflict.MechanicalEvaluation{
			{
				ID:      "mech-1",
				State:   policyconflict.ProofViolatedWithWitness,
				Reasons: []policyconflict.ReasonCode{policyconflict.ReasonMechanicalConflict},
				Exemptions: []policyconflict.ExemptionResolution{
					{
						ID: "ex-1",
						Resolution: policyconflict.AuthorityResolution{
							Match: policyconflict.ProofProven, Freshness: policyconflict.ProofUnproven,
							Scope: policyconflict.ProofProven, Bound: policyconflict.ProofUnproven, Authorization: policyconflict.ProofProven,
						},
					},
				},
			},
		},
		Semantic: []policyconflict.SemanticEvaluation{
			{ID: "sem-1", State: policyconflict.ProofUnproven, Reasons: []policyconflict.ReasonCode{policyconflict.ReasonJudgeUnavailable}},
		},
	}
	wantIDs := []string{"conflict-mechanical/mech-1", "conflict-semantic/sem-1", "exemption-ineffective/ex-1"}

	t.Run("report supplied before acceptance names the acceptance verb", func(t *testing.T) {
		in := eventualInput{Class: "feature", Model: model.Canonical(), State: statusOf(specstate.Proposed), Owner: owner, Conflict: report}
		eb := deriveEventual(in)
		if !eb.Derived {
			t.Fatal("Derived must be true")
		}
		ids := blockerIDs(eb.Items)
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("ids = %v, want %v", ids, wantIDs)
		}
		byID := indexBlockers(eb.Items)
		if b := byID["conflict-mechanical/mech-1"]; b.Reason != ReasonConflictMechanicalUnresolved || b.Class != ClassMechanical || b.Transition != "merge" {
			t.Fatalf("mechanical blocker = %+v", b)
		}
		if b := byID["conflict-semantic/sem-1"]; b.Reason != ReasonConflictSemanticUnresolved || b.Class != ClassJudgmental || b.Transition != "merge" {
			t.Fatalf("semantic blocker = %+v", b)
		}
		if b := byID["exemption-ineffective/ex-1"]; b.Reason != ReasonExemptionIneffective || b.Class != ClassGovernance || b.Transition != "merge" {
			t.Fatalf("exemption blocker = %+v", b)
		}
		for _, d := range eb.Disclosures {
			if strings.Contains(d, "no policy-conflict report") {
				t.Fatalf("disclosures = %v, must not disclose a missing report when one was supplied", eb.Disclosures)
			}
		}
		if err := (Blockers{Current: []Blocker{}, Eventual: eb}).validate(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("report supplied after acceptance names the closure verb", func(t *testing.T) {
		in := eventualInput{Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: owner, Conflict: report}
		eb := deriveEventual(in)
		if got := blockerIDs(eb.Items); !reflect.DeepEqual(got, wantIDs) {
			t.Fatalf("ids = %v, want %v", got, wantIDs)
		}
		for _, b := range eb.Items {
			if b.Transition != "close" {
				t.Fatalf("blocker %s names transition %q, want close once acceptance is behind", b.ID, b.Transition)
			}
		}
	})

	t.Run("nil report", func(t *testing.T) {
		in := eventualInput{Class: "feature", Model: model.Canonical(), State: statusOf(specstate.Proposed), Owner: owner, Conflict: nil}
		eb := deriveEventual(in)
		if !eb.Derived {
			t.Fatal("Derived must be true even with no conflict report")
		}
		if len(eb.Items) != 0 {
			t.Fatalf("items = %v, want none of the conflict-derived items with a nil report", blockerIDs(eb.Items))
		}
		if len(eb.Disclosures) != 1 || !strings.Contains(eb.Disclosures[0], "no policy-conflict report") {
			t.Fatalf("disclosures = %v, want exactly the no-report disclosure", eb.Disclosures)
		}
	})
}

// TestDeriveEventual_StoryClassSkipsFeatureSources proves R-RR1-3's class
// gate: the three feature-only sources (stub reconciliation, the outcome
// floor, claimed questions) never fire for class story, even when their
// facts are (incorrectly) supplied.
func TestDeriveEventual_StoryClassSkipsFeatureSources(t *testing.T) {
	owner := testOwner()
	in := eventualInput{
		Class: "story", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: owner,
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
		Spec: &artifact.SpecFrontmatter{
			OpenQuestions: []artifact.OpenQuestion{{ID: "oq-1", Text: "x", Anchor: "#oq-1"}},
			Stubs:         []artifact.Stub{{Slug: "s", Spike: true, Resolves: []string{"oq-1"}}},
		},
	}
	eb := deriveEventual(in)
	if len(eb.Items) != 0 {
		t.Fatalf("items = %v, want none: class story must skip every feature-only source", blockerIDs(eb.Items))
	}
	if !eb.Derived {
		t.Fatal("Derived must still be true for class story")
	}
}

// TestDeriveEventual_UndeclaredClassDerivesNothingAndDiscloses is
// R-RR1-12's last clause: a class the operating model declares no
// lifecycle for emits no feature-source and no conflict-source item —
// there is no transition to name, and the literal "unknown" is never
// emitted — and says so.
func TestDeriveEventual_UndeclaredClassDerivesNothingAndDiscloses(t *testing.T) {
	in := eventualInput{
		Class: "widget", Model: model.Canonical(), State: "draft", Owner: testOwner(),
		Stubs:    &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}}},
		Fold:     &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
		Conflict: &policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{{ID: "sem-1", State: policyconflict.ProofUnproven}}},
	}
	eb := deriveEventual(in)
	if len(eb.Items) != 0 {
		t.Fatalf("items = %v, want none for a class with no declared lifecycle", blockerIDs(eb.Items))
	}
	if !eb.Derived {
		t.Fatal("Derived must still be true: a partial derivation discloses, it does not go underived")
	}
	found := false
	for _, d := range eb.Disclosures {
		if strings.Contains(d, "declares no closure transition") {
			found = true
		}
	}
	if !found {
		t.Fatalf("disclosures = %v, want one explaining the undeclared lifecycle", eb.Disclosures)
	}
}

// TestDeriveEventual_NoUnknownTransition sweeps every source at once: no
// derived item may ever carry the literal "unknown" as its transition
// (SI-207), even when the lifecycle state itself is unproven.
func TestDeriveEventual_NoUnknownTransition(t *testing.T) {
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.Unproven), Owner: testOwner(),
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{{Slug: "board-tab", Bucket: evidence.StubUnreconciled}}},
		Fold:  &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
		Spec: &artifact.SpecFrontmatter{
			OpenQuestions: []artifact.OpenQuestion{{ID: "oq-1", Text: "x", Anchor: "#oq-1"}},
			Stubs:         []artifact.Stub{{Slug: "s", Spike: true, Resolves: []string{"oq-1"}}},
		},
		Conflict: &policyconflict.Report{Semantic: []policyconflict.SemanticEvaluation{{ID: "sem-1", State: policyconflict.ProofUnproven}}},
	}
	eb := deriveEventual(in)
	if len(eb.Items) == 0 {
		t.Fatal("want items to sweep")
	}
	for _, b := range eb.Items {
		if b.Transition == "unknown" {
			t.Fatalf("blocker %s names the literal unknown transition", b.ID)
		}
	}
}

// TestDeriveEventual_OrderingAndUniqueness proves items are returned
// strictly ascending by ID, and that Blockers.validate refuses an eventual
// item whose ID collides with a current blocker's ID (the cross-section
// uniqueness rule record.go already enforces; this proves deriveEventual's
// own output composes with it correctly).
func TestDeriveEventual_OrderingAndUniqueness(t *testing.T) {
	owner := testOwner()
	in := eventualInput{
		Class: "feature", Model: model.Canonical(), State: statusOf(specstate.AcceptedPendingBuild), Owner: owner,
		Stubs: &evidence.StubReconciliation{Stubs: []evidence.StubResult{
			{Slug: "zzz-last", Bucket: evidence.StubUnreconciled},
			{Slug: "aaa-first", Bucket: evidence.StubUnreconciled},
		}},
		Fold: &evidence.FeatureResult{ACs: []evidence.FeatureACResult{{ID: "ac-1", Floor: evidence.FloorResult{Satisfied: false}}}},
	}
	eb := deriveEventual(in)
	ids := blockerIDs(eb.Items)
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("items not ascending: %v", ids)
	}
	if len(ids) < 2 {
		t.Fatalf("want multiple items to prove ordering, got %v", ids)
	}

	current := []Blocker{eb.Items[0]}
	if err := (Blockers{Current: current, Eventual: eb}).validate(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("validate() = %v, want a duplicate-id refusal when an eventual item's ID equals a current blocker's ID", err)
	}
}

// TestEventualFeatureName covers resolution (f)'s name lookup and M3's
// fallback: a spec id wins, a fold ref is the fallback, and a ref that
// parses as neither still NAMES the raw text rather than composing an
// empty path segment into the clearing condition.
func TestEventualFeatureName(t *testing.T) {
	tests := []struct {
		name string
		in   eventualInput
		want string
	}{
		{"spec id wins", eventualInput{Spec: &artifact.SpecFrontmatter{Base: artifact.Base{ID: "spec/checkout"}}, Fold: &evidence.FeatureResult{SpecRef: "spec/other"}}, "checkout"},
		{"fold ref is the fallback", eventualInput{Fold: &evidence.FeatureResult{SpecRef: "spec/other"}}, "other"},
		{"unparseable spec id falls back to the fold ref", eventualInput{Spec: &artifact.SpecFrontmatter{Base: artifact.Base{ID: "nonsense"}}, Fold: &evidence.FeatureResult{SpecRef: "spec/other"}}, "other"},
		{"neither parses: the raw ref is named", eventualInput{Spec: &artifact.SpecFrontmatter{Base: artifact.Base{ID: "nonsense"}}}, "nonsense"},
		{"nothing at all", eventualInput{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventualFeatureName(tt.in); got != tt.want {
				t.Fatalf("eventualFeatureName = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- sanitizeConflictID / sanitizeStubSlug / dedupeConflictIDs -----------

func TestSanitizeConflictID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"already valid kebab-case", "mech-1", "mech-1"},
		{"uppercase lowercased", "MECH-1", "mech-1"},
		{"internal punctuation collapses to one dash", "Mech Row #1", "mech-row-1"},
		{"run of punctuation collapses to one dash", "a!!!b", "a-b"},
		{"leading/trailing punctuation trimmed", "!!!abc!!!", "abc"},
		{"leading digit gets a letter prefix", "3xyz", "row-3xyz"},
		{"empty string falls back to row", "", "row"},
		{"all punctuation falls back to row", "###", "row"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeConflictID(tt.raw)
			if got != tt.want {
				t.Fatalf("sanitizeConflictID(%q) = %q, want %q", tt.raw, got, tt.want)
			}
			if !blockerIDRe.MatchString("conflict-mechanical/" + got) {
				t.Fatalf("sanitizeConflictID(%q) = %q does not compose into a valid blocker id", tt.raw, got)
			}
		})
	}
}

// TestSanitizeStubSlug is C1's unit half: every slug internal/artifact's
// own simpleNameRe admits must compose into a valid journey blocker id
// segment, and a slug that already starts with a letter must pass through
// byte-identical (the id stays the slug an operator can search for).
func TestSanitizeStubSlug(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"letter-led slug is unchanged", "board-tab", "board-tab"},
		{"single letter-led word", "core", "core"},
		{"digit-led slug gets a letter prefix", "2fa-login", "s-2fa-login"},
		{"digit-led slug, second case", "3ds-checkout", "s-3ds-checkout"},
		{"all digits", "2024", "s-2024"},
		{"empty falls back to the prefix alone", "", "s"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeStubSlug(tt.raw)
			if got != tt.want {
				t.Fatalf("sanitizeStubSlug(%q) = %q, want %q", tt.raw, got, tt.want)
			}
			if !blockerIDRe.MatchString("stub-unreconciled/" + got) {
				t.Fatalf("sanitizeStubSlug(%q) = %q does not compose into a valid blocker id", tt.raw, got)
			}
		})
	}
}

// TestDedupeConflictIDs proves resolution (d)'s collision rule: two DISTINCT
// raw ids that normalize to the same sanitized form are disambiguated
// deterministically in report order (-2, -3, ...), each collision disclosed;
// a raw id that normalizes uniquely keeps its bare sanitized form; and the
// disambiguated form is itself checked for use, so the suffix can never
// land on an id the report already carries.
func TestDedupeConflictIDs(t *testing.T) {
	tests := []struct {
		name            string
		raw             []string
		want            []string
		wantDisclosures int
	}{
		{
			name:            "case and punctuation collisions disambiguate in report order",
			raw:             []string{"Mech-1", "mech-1", "sem/1", "mech_1", "sem-1"},
			want:            []string{"mech-1", "mech-1-2", "sem-1", "mech-1-3", "sem-1-2"},
			wantDisclosures: 3,
		},
		{
			name:            "a raw id equal to an earlier disambiguated form is itself disambiguated",
			raw:             []string{"Mech-1", "mech-1", "mech-1-2"},
			want:            []string{"mech-1", "mech-1-2", "mech-1-2-2"},
			wantDisclosures: 2,
		},
		{
			name: "no collisions",
			raw:  []string{"a", "b", "c"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "empty input",
			raw:  nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, disclosures := dedupeConflictIDs(tt.raw)
			if len(ids) == 0 {
				ids = []string{}
			}
			if !reflect.DeepEqual(ids, tt.want) {
				t.Fatalf("ids = %v, want %v", ids, tt.want)
			}
			if len(disclosures) != tt.wantDisclosures {
				t.Fatalf("disclosures = %v, want exactly %d", disclosures, tt.wantDisclosures)
			}
			seen := map[string]bool{}
			for i, id := range ids {
				if !blockerIDRe.MatchString("conflict-mechanical/" + id) {
					t.Fatalf("ids[%d] = %q does not compose into a valid blocker id", i, id)
				}
				if seen[id] {
					t.Fatalf("ids = %v, contains a duplicate after dedup: %q", ids, id)
				}
				seen[id] = true
			}
		})
	}
}

// TestConflictBlockers_ProvenRowsAreNotNumbered is M4: a proven row never
// produces a blocker, so it must not consume an id either — two proven
// rows colliding would otherwise emit a disclosure with no corresponding
// item, and a proven row could push an unproven row's id to "-2".
func TestConflictBlockers_ProvenRowsAreNotNumbered(t *testing.T) {
	report := &policyconflict.Report{
		Mechanical: []policyconflict.MechanicalEvaluation{
			{ID: "Row-1", State: policyconflict.ProofProven},
			{ID: "row_1", State: policyconflict.ProofProven},
			{ID: "row/1", State: policyconflict.ProofUnproven},
		},
	}
	blockers, disclosures := conflictMechanicalBlockers(report, "close", testOwner())
	if len(blockers) != 1 || blockers[0].ID != "conflict-mechanical/row-1" {
		t.Fatalf("blockers = %v, want exactly conflict-mechanical/row-1", blockerIDs(blockers))
	}
	if len(disclosures) != 0 {
		t.Fatalf("disclosures = %v, want none: proven rows are skipped before numbering", disclosures)
	}
}
