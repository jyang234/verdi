package policyconflict

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

func allProvenResolution() AuthorityResolution {
	return AuthorityResolution{Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofProven, Authorization: ProofProven}
}

// exemptionFor builds a resolution naming exactly the supplied row claims as
// MechanicalClaimWitnesses (ledger SI-105(c): a removed mechanical claim is
// identified by (policy_id, claim_id, claim_digest), never by the semantic
// prose-witness vocabulary), sorted by the composite key.
func exemptionFor(id string, resolution AuthorityResolution, removed ...TypedClaimRecord) ExemptionResolution {
	witnesses := make([]MechanicalClaimWitness, 0, len(removed))
	for _, c := range removed {
		witnesses = append(witnesses, MechanicalClaimWitness{PolicyID: c.PolicyID, ClaimID: c.Claim.ID, ClaimDigest: c.ClaimDigest})
	}
	sort.Slice(witnesses, func(i, j int) bool {
		if witnesses[i].PolicyID != witnesses[j].PolicyID {
			return witnesses[i].PolicyID < witnesses[j].PolicyID
		}
		return witnesses[i].ClaimID < witnesses[j].ClaimID
	})
	return ExemptionResolution{
		ID: id, Digest: testDigest64, Resolution: resolution, RemovedClaims: witnesses,
	}
}

// rejectedExemption builds a not-all-proven resolution: authority design
// §5.5 requires it name the mandatory-present explicit EMPTY removal set,
// because it removed nothing.
func rejectedExemption(id string, resolution AuthorityResolution) ExemptionResolution {
	return ExemptionResolution{
		ID: id, Digest: testDigest64, Resolution: resolution, RemovedClaims: []MechanicalClaimWitness{},
	}
}

func evaluateOneRow(t *testing.T, in MechanicalInput) MechanicalEvaluation {
	t.Helper()
	result, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	return rows[0]
}

// --- accept only all-proven resolutions -------------------------------------

func TestApplyEffectiveExemptionsNoResolutions(t *testing.T) {
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())),
	}})
	got, err := ApplyEffectiveExemptions(row, nil)
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != row.State {
		t.Fatalf("State = %q, want unchanged %q", got.State, row.State)
	}
	if len(got.Exemptions) != 0 {
		t.Fatalf("Exemptions = %+v, want none", got.Exemptions)
	}
	if !reflect.DeepEqual(got.After, row.Before) {
		t.Fatalf("After = %+v, want equal to Before (no exemptions applied)", got.After)
	}
}

func TestApplyEffectiveExemptionsRejectsEachNotProvenState(t *testing.T) {
	row := violatedRow(t)

	fields := []struct {
		name string
		mk   func(AuthorityResolution) AuthorityResolution
	}{
		{"match", func(r AuthorityResolution) AuthorityResolution { r.Match = ProofUnproven; return r }},
		{"freshness", func(r AuthorityResolution) AuthorityResolution { r.Freshness = ProofViolatedWithWitness; return r }},
		{"scope", func(r AuthorityResolution) AuthorityResolution { r.Scope = ProofUnproven; return r }},
		{"bound", func(r AuthorityResolution) AuthorityResolution { r.Bound = ProofUnproven; return r }},
		{"authorization", func(r AuthorityResolution) AuthorityResolution { r.Authorization = ProofUnproven; return r }},
	}
	for _, f := range fields {
		t.Run(f.name+" not proven is rejected", func(t *testing.T) {
			resolution := rejectedExemption("ex-"+f.name, f.mk(allProvenResolution()))
			got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
			if err != nil {
				t.Fatalf("ApplyEffectiveExemptions: %v", err)
			}
			// Authority design §10 / ledger SI-103: the row carries EVERY
			// applicable resolution with its typed states, so a rejected
			// resolution stays visible and unapplied rather than vanishing.
			if !reflect.DeepEqual(got.Exemptions, []ExemptionResolution{resolution}) {
				t.Fatalf("Exemptions = %+v, want the rejected resolution retained unapplied with its states intact", got.Exemptions)
			}
			if !reflect.DeepEqual(got.After, row.Before) {
				t.Fatalf("After = %+v, want the original proof (nothing was removed)", got.After)
			}
			if got.State != row.State {
				t.Fatalf("State = %q, want unchanged %q (rejected candidate never covers)", got.State, row.State)
			}
			found := false
			for _, r := range got.Reasons {
				if r == ReasonExemptionIneffective {
					found = true
				}
			}
			if !found {
				t.Fatalf("Reasons = %v, want exemption-ineffective disclosed", got.Reasons)
			}
		})
	}
}

// --- exact scoped removal and rerun ------------------------------------------

// violatedRow builds a genuine exact-scope discrete conflict row: two
// policies both fixing "level" to disagreeing exact values in the same
// scope.
func violatedRow(t *testing.T) MechanicalEvaluation {
	t.Helper()
	scope := phaseScope("build")
	return evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, scope)),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, scope)),
	}})
}

func TestApplyEffectiveExemptionsCoversConflict(t *testing.T) {
	row := violatedRow(t)
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness", row.State)
	}
	// Waive the silver policy's claim: the remainder is one equals(gold)
	// claim alone, trivially satisfiable.
	var silver TypedClaimRecord
	for _, c := range row.Claims {
		if c.PolicyID == "policy-b" {
			silver = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), silver)

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofProven {
		t.Fatalf("State = %q, want proven (post-exemption conjunction satisfiable)", got.State)
	}
	if len(got.Reasons) != 1 || got.Reasons[0] != ReasonExemptionEffective {
		t.Fatalf("Reasons = %v, want [exemption-effective]", got.Reasons)
	}
	if got.After.State != SolverSatisfiable {
		t.Fatalf("After.State = %q, want satisfiable", got.After.State)
	}
	if !reflect.DeepEqual(got.Before, row.Before) {
		t.Fatalf("Before = %+v, want the original mechanical proof retained unchanged", got.Before)
	}
	if len(got.Exemptions) != 1 || len(got.Exemptions[0].RemovedClaims) != 1 {
		t.Fatalf("Exemptions = %+v, want exactly the named removed claim", got.Exemptions)
	}
	wantWitness := MechanicalClaimWitness{PolicyID: silver.PolicyID, ClaimID: silver.Claim.ID, ClaimDigest: silver.ClaimDigest}
	if got.Exemptions[0].RemovedClaims[0] != wantWitness {
		t.Fatalf("RemovedClaims[0] = %+v, want the composite witness %+v", got.Exemptions[0].RemovedClaims[0], wantWitness)
	}
	if err := validateMechanicalEvaluation("row", got); err != nil {
		t.Fatalf("result failed the package's own validation: %v", err)
	}
}

func TestApplyEffectiveExemptionsPartialRemovalStillConflict(t *testing.T) {
	scope := phaseScope("build")
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, scope)),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, scope)),
		typedClaim(t, "policy-c", discreteClaim("c3", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, scope)),
	}})
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness", row.State)
	}
	// Waive only the harmless allowed-values claim: gold vs silver still
	// disagree, so the conflict remains uncovered.
	var harmless TypedClaimRecord
	for _, c := range row.Claims {
		if c.PolicyID == "policy-c" {
			harmless = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), harmless)

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want unchanged violated-with-witness (exemption insufficient)", got.State)
	}
	found := false
	for _, r := range got.Reasons {
		if r == ReasonExemptionIneffective {
			found = true
		}
	}
	if !found {
		t.Fatalf("Reasons = %v, want exemption-ineffective disclosed", got.Reasons)
	}
}

// TestApplyEffectiveExemptionsRetainsRejectedBesideAccepted covers the
// mixed case: one all-proven resolution covers the conflict while a second,
// not-all-proven resolution is rejected. Both stay on the row (authority
// design §10: the row carries every applicable resolution), and the
// rejected one's ineffectiveness stays visible in the reason set.
func TestApplyEffectiveExemptionsRetainsRejectedBesideAccepted(t *testing.T) {
	row := violatedRow(t)
	var silver TypedClaimRecord
	for _, c := range row.Claims {
		if c.PolicyID == "policy-b" {
			silver = c
		}
	}
	accepted := exemptionFor("ex-a-accepted", allProvenResolution(), silver)
	rejected := rejectedExemption("ex-b-rejected", AuthorityResolution{
		Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofUnproven, Authorization: ProofProven,
	})

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{rejected, accepted})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofProven {
		t.Fatalf("State = %q, want proven (the accepted resolution covers the conflict)", got.State)
	}
	if len(got.Exemptions) != 2 || got.Exemptions[0].ID != "ex-a-accepted" || got.Exemptions[1].ID != "ex-b-rejected" {
		t.Fatalf("Exemptions = %+v, want both resolutions retained in ascending ID order", got.Exemptions)
	}
	if got.Exemptions[1].Resolution.Bound != ProofUnproven {
		t.Fatalf("retained rejected resolution's states = %+v, want them intact", got.Exemptions[1].Resolution)
	}
	// The rejected resolution removed nothing: only the accepted one's claim
	// left the conjunction.
	if len(got.After.Values) != 1 || got.After.Values[0] != "gold" {
		t.Fatalf("After = %+v, want only the accepted resolution's removal applied", got.After)
	}
	wantReasons := []ReasonCode{ReasonExemptionEffective, ReasonExemptionIneffective}
	if !reflect.DeepEqual(got.Reasons, wantReasons) {
		t.Fatalf("Reasons = %v, want %v (coverage proven, one applicable resolution ineffective)", got.Reasons, wantReasons)
	}
	if err := validateMechanicalEvaluation("row", got); err != nil {
		t.Fatalf("result failed the package's own validation: %v", err)
	}
}

// TestApplyEffectiveExemptionsDedupesIdenticalResolutionIDs pins the wire's
// exemption identity rule (validate.go's requireSortedUnique over
// ExemptionResolution.ID): the same resolution presented twice is one
// applicable resolution, while two DIFFERENT resolutions sharing one id are
// a caller defect rather than a silent pick.
func TestApplyEffectiveExemptionsDedupesIdenticalResolutionIDs(t *testing.T) {
	row := violatedRow(t)
	var silver, gold TypedClaimRecord
	for _, c := range row.Claims {
		switch c.PolicyID {
		case "policy-a":
			gold = c
		case "policy-b":
			silver = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), silver)

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution, resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if len(got.Exemptions) != 1 {
		t.Fatalf("Exemptions = %+v, want the identical duplicate collapsed", got.Exemptions)
	}
	if err := validateMechanicalEvaluation("row", got); err != nil {
		t.Fatalf("result failed the package's own validation: %v", err)
	}

	conflicting := exemptionFor("ex-1", allProvenResolution(), gold)
	if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution, conflicting}); err == nil {
		t.Fatal("want operational error for two different resolutions sharing one id, got nil")
	}
}

// TestApplyEffectiveExemptionsMalformedRowIsOperationalError mirrors
// EvaluateMechanical's own entry validation: a hand-built row carrying a
// claim that never passed Claim.Validate is a caller defect reported as an
// operational error, never a panic inside a solver.
func TestApplyEffectiveExemptionsMalformedRowIsOperationalError(t *testing.T) {
	malformed := policyartifact.Claim{
		ID: "c1", Family: policyartifact.FamilyConfiguration, Operator: policyartifact.OpEquals,
		Subject: "level", Values: []string{}, Scope: universalScope(), Overridable: true,
	}
	malformedDigest, err := policyartifact.ClaimDigest(malformed)
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}
	healthy := discreteClaim("c2", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	healthyDigest, err := policyartifact.ClaimDigest(healthy)
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}
	records := []TypedClaimRecord{
		{PolicyID: "policy-a", PolicyDigest: testDigest64, ClaimDigest: malformedDigest, Claim: malformed},
		{PolicyID: "policy-b", PolicyDigest: testDigest64, ClaimDigest: healthyDigest, Claim: healthy},
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ClaimDigest < records[j].ClaimDigest })
	proof := SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}
	row := MechanicalEvaluation{
		ID: "configuration:level#complete", Family: policyartifact.FamilyConfiguration, Subject: "level",
		Claims: records, Scope: ScopeProof{State: ScopeOverlap, Dimensions: []DimensionProof{}},
		Domain: domainDiscreteSet, Before: proof, Exemptions: []ExemptionResolution{}, After: proof,
		State: ProofViolatedWithWitness, Reasons: []ReasonCode{ReasonMechanicalConflict},
	}

	// Removing only the healthy claim leaves the malformed one in the
	// remainder the solver reruns over.
	var healthyRecord TypedClaimRecord
	for _, c := range records {
		if c.ClaimDigest == healthyDigest {
			healthyRecord = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), healthyRecord)
	if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution}); err == nil {
		t.Fatal("want operational error for a malformed row claim, got nil")
	}
}

// TestApplyEffectiveExemptionsProvenDisjointRowIsNotCredited covers a row
// that is already proven by proven-disjoint scope: it was never a mechanical
// conflict, so no exemption can be credited with covering it (§5.5's
// coverage rule applies to a conflict).
func TestApplyEffectiveExemptionsProvenDisjointRowIsNotCredited(t *testing.T) {
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, phaseScope("design"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("build"))),
	}})
	if row.State != ProofProven || len(row.Reasons) != 1 || row.Reasons[0] != ReasonScopeDisjoint {
		t.Fatalf("precondition: row = %+v, want a proven scope-disjoint row", row)
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), row.Claims[0])

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofProven {
		t.Fatalf("State = %q, want proven (unchanged)", got.State)
	}
	if !reflect.DeepEqual(got.Reasons, row.Reasons) {
		t.Fatalf("Reasons = %v, want the row's own %v: a disjoint row needs no exemption credit", got.Reasons, row.Reasons)
	}
	if len(got.Exemptions) != 1 {
		t.Fatalf("Exemptions = %+v, want the applicable resolution recorded", got.Exemptions)
	}
}

// TestExemptionResolutionSharedClaimBytesDepartOnePolicyIdentity pins ledger
// SI-105(c): two policies declaring one byte-identical claim contribute TWO
// composite identities, so an exemption naming one policy's identity departs
// from that policy's claim ONLY — the other policy's identical requirement
// survives the departure.
func TestExemptionResolutionSharedClaimBytesDepartOnePolicyIdentity(t *testing.T) {
	shared := discreteClaim("shared-claim", "level", policyartifact.OpEquals, []string{"gold"}, phaseScope("build"))
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", shared),
		typedClaim(t, "policy-b", shared),
		typedClaim(t, "policy-c", discreteClaim("c3", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("build"))),
	}})
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness", row.State)
	}
	if len(row.Claims) != 3 {
		t.Fatalf("row Claims = %+v, want one record per (policy_id, claim_id) identity", row.Claims)
	}
	var policyAShared TypedClaimRecord
	for _, c := range row.Claims {
		if c.Claim.ID == "shared-claim" && c.PolicyID == "policy-a" {
			policyAShared = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), policyAShared)

	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want the conflict uncovered: policy-b still declares the identical gold requirement", got.State)
	}
	if err := validateMechanicalEvaluation("row", got); err != nil {
		t.Fatalf("result failed the package's own validation: %v", err)
	}

	// Departing from BOTH policy identities does cover it.
	var policyBShared TypedClaimRecord
	for _, c := range row.Claims {
		if c.Claim.ID == "shared-claim" && c.PolicyID == "policy-b" {
			policyBShared = c
		}
	}
	both := exemptionFor("ex-2", allProvenResolution(), policyAShared, policyBShared)
	covered, err := ApplyEffectiveExemptions(row, []ExemptionResolution{both})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if covered.State != ProofProven {
		t.Fatalf("State = %q, want proven once every declaring policy identity is departed from", covered.State)
	}
}

// TestExemptionResolutionRemovedClaimsCardinality pins authority design
// §5.5's mandatory-present removal set: an all-five-proven resolution names
// at least one witness, and every rejected resolution names the explicit
// empty set because it removed nothing.
func TestExemptionResolutionRemovedClaimsCardinality(t *testing.T) {
	row := violatedRow(t)
	notProven := AuthorityResolution{
		Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofUnproven, Authorization: ProofProven,
	}

	t.Run("rejected resolution with an explicit empty set is accepted", func(t *testing.T) {
		got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{rejectedExemption("ex-1", notProven)})
		if err != nil {
			t.Fatalf("ApplyEffectiveExemptions: %v", err)
		}
		if len(got.Exemptions) != 1 || len(got.Exemptions[0].RemovedClaims) != 0 || got.Exemptions[0].RemovedClaims == nil {
			t.Fatalf("Exemptions = %+v, want the rejected resolution retained with a mandatory-present empty removal set", got.Exemptions)
		}
		if err := validateMechanicalEvaluation("row", got); err != nil {
			t.Fatalf("result failed the package's own validation: %v", err)
		}
	})

	t.Run("proven resolution removing nothing is refused", func(t *testing.T) {
		empty := ExemptionResolution{ID: "ex-1", Digest: testDigest64, Resolution: allProvenResolution(), RemovedClaims: []MechanicalClaimWitness{}}
		if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{empty}); err == nil {
			t.Fatal("want operational error for an all-proven resolution naming no removed claim, got nil")
		}
	})

	t.Run("rejected resolution claiming a removal is refused", func(t *testing.T) {
		lying := exemptionFor("ex-1", notProven, row.Claims[0])
		if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{lying}); err == nil {
			t.Fatal("want operational error for a rejected resolution claiming a removal it never made, got nil")
		}
	})

	t.Run("absent removal set is refused", func(t *testing.T) {
		absent := ExemptionResolution{ID: "ex-1", Digest: testDigest64, Resolution: notProven}
		if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{absent}); err == nil {
			t.Fatal("want operational error for an absent (nil) removal set, got nil")
		}
	})
}

// TestExemptionResolutionWitnessMustNameCurrentRowClaim pins §5.5's
// exact-current-row membership rule: an applied witness must belong to the
// row's own typed-claim set by BOTH composite key and digest.
func TestExemptionResolutionWitnessMustNameCurrentRowClaim(t *testing.T) {
	row := violatedRow(t)
	present := row.Claims[0]

	negatives := []struct {
		name    string
		witness MechanicalClaimWitness
	}{
		{"policy id absent from the row", MechanicalClaimWitness{PolicyID: "policy-ghost", ClaimID: present.Claim.ID, ClaimDigest: present.ClaimDigest}},
		{"claim id absent from the row", MechanicalClaimWitness{PolicyID: present.PolicyID, ClaimID: "ghost-claim", ClaimDigest: present.ClaimDigest}},
		{"stale digest for a present identity", MechanicalClaimWitness{PolicyID: present.PolicyID, ClaimID: present.Claim.ID, ClaimDigest: testDigest64}},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			resolution := ExemptionResolution{
				ID: "ex-1", Digest: testDigest64, Resolution: allProvenResolution(),
				RemovedClaims: []MechanicalClaimWitness{tc.witness},
			}
			if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution}); err == nil {
				t.Fatalf("want operational error for %s, got nil", tc.name)
			}
		})
	}

	t.Run("exact current witness is applied", func(t *testing.T) {
		resolution := exemptionFor("ex-1", allProvenResolution(), present)
		if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution}); err != nil {
			t.Fatalf("ApplyEffectiveExemptions(exact witness): %v", err)
		}
	})
}

// TestExemptionResolutionRemovedClaimsCompositeOrdering pins the composite
// sort/dedupe rule the wire requires: identical witnesses collapse, and two
// witnesses disagreeing about one identity's digest are contradictory.
func TestExemptionResolutionRemovedClaimsCompositeOrdering(t *testing.T) {
	row := violatedRow(t)
	var gold, silver TypedClaimRecord
	for _, c := range row.Claims {
		switch c.PolicyID {
		case "policy-a":
			gold = c
		case "policy-b":
			silver = c
		}
	}

	t.Run("unsorted duplicates normalize", func(t *testing.T) {
		unsorted := ExemptionResolution{
			ID: "ex-1", Digest: testDigest64, Resolution: allProvenResolution(),
			RemovedClaims: []MechanicalClaimWitness{
				{PolicyID: silver.PolicyID, ClaimID: silver.Claim.ID, ClaimDigest: silver.ClaimDigest},
				{PolicyID: gold.PolicyID, ClaimID: gold.Claim.ID, ClaimDigest: gold.ClaimDigest},
				{PolicyID: silver.PolicyID, ClaimID: silver.Claim.ID, ClaimDigest: silver.ClaimDigest},
			},
		}
		got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{unsorted})
		if err != nil {
			t.Fatalf("ApplyEffectiveExemptions: %v", err)
		}
		removed := got.Exemptions[0].RemovedClaims
		if len(removed) != 2 || removed[0].PolicyID != "policy-a" || removed[1].PolicyID != "policy-b" {
			t.Fatalf("RemovedClaims = %+v, want composite-sorted and deduplicated", removed)
		}
		if err := validateMechanicalEvaluation("row", got); err != nil {
			t.Fatalf("result failed the package's own validation: %v", err)
		}
	})

	t.Run("one identity with two digests is refused", func(t *testing.T) {
		contradictory := ExemptionResolution{
			ID: "ex-1", Digest: testDigest64, Resolution: allProvenResolution(),
			RemovedClaims: []MechanicalClaimWitness{
				{PolicyID: gold.PolicyID, ClaimID: gold.Claim.ID, ClaimDigest: gold.ClaimDigest},
				{PolicyID: gold.PolicyID, ClaimID: gold.Claim.ID, ClaimDigest: testDigest64},
			},
		}
		if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{contradictory}); err == nil {
			t.Fatal("want operational error for one identity carrying two digests, got nil")
		}
	})
}

func TestApplyEffectiveExemptionsNeverMutatesInputRow(t *testing.T) {
	row := violatedRow(t)
	before := row.Before
	beforeReasons := append([]ReasonCode{}, row.Reasons...)

	var silver TypedClaimRecord
	for _, c := range row.Claims {
		if c.PolicyID == "policy-b" {
			silver = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), silver)
	if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution}); err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}

	if !reflect.DeepEqual(row.Before, before) {
		t.Fatalf("input row.Before mutated: got %+v, want %+v", row.Before, before)
	}
	if !reflect.DeepEqual(row.Reasons, beforeReasons) {
		t.Fatalf("input row.Reasons mutated: got %v, want %v", row.Reasons, beforeReasons)
	}
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("input row.State mutated: got %q", row.State)
	}
}

func TestApplyEffectiveExemptionsUnknownClaimIdentityIsOperationalError(t *testing.T) {
	row := violatedRow(t)
	ghost := TypedClaimRecord{
		PolicyID: "policy-ghost", PolicyDigest: testDigest64, ClaimDigest: testDigest64,
		Claim: discreteClaim("ghost-claim", "level", policyartifact.OpEquals, []string{"bronze"}, phaseScope("build")),
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), ghost)
	if _, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution}); err == nil {
		t.Fatal("want operational error for a named claim absent from the row, got nil")
	}
}

func TestApplyEffectiveExemptionsDeterministic(t *testing.T) {
	row := violatedRow(t)
	var silver TypedClaimRecord
	for _, c := range row.Claims {
		if c.PolicyID == "policy-b" {
			silver = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), silver)

	got1, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	got2, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("ApplyEffectiveExemptions is non-deterministic: %+v vs %+v", got1, got2)
	}
}

// --- principal-relation domain: no kernel context available -----------------

func TestApplyEffectiveExemptionsPrincipalRelationEmptiedRemainderSatisfiable(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpSamePrincipal, "author", "reviewer", universalScope())),
		typedClaim(t, "policy-b", principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope())),
	}, Profile: profile})
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness (same+different contradiction)", row.State)
	}

	resolution := exemptionFor("ex-1", allProvenResolution(), row.Claims...)
	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.State != ProofProven {
		t.Fatalf("State = %q, want proven (every relation claim removed)", got.State)
	}
	if got.After.State != SolverSatisfiable {
		t.Fatalf("After.State = %q, want satisfiable", got.After.State)
	}
}

func TestApplyEffectiveExemptionsPrincipalRelationPartialRemovalConservative(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpSamePrincipal, "author", "reviewer", universalScope())),
		typedClaim(t, "policy-b", principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope())),
	}, Profile: profile})
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness", row.State)
	}

	// Remove only the same-principal claim: one different-principal claim
	// remains. Authority design §5.5 requires "the same solver runs again",
	// and the surviving half is exactly the kernel-free part of §5.3 — the
	// same+different textual contradiction is gone, so the original violated
	// proof must NOT be repeated. What remains (a single relation claim)
	// cannot be re-verified without the kernel, which this fixed signature
	// carries no ctx/profile/actors for, so the rerun is unproven.
	var samePrincipalClaim TypedClaimRecord
	for _, c := range row.Claims {
		if c.Claim.Operator == policyartifact.OpSamePrincipal {
			samePrincipalClaim = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), samePrincipalClaim)
	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.After.State != SolverUnproven {
		t.Fatalf("After.State = %q, want unproven (one surviving relation claim, no kernel available)", got.After.State)
	}
	if got.State != ProofUnproven {
		t.Fatalf("State = %q, want unproven (the original contradiction no longer holds and nothing re-proves it)", got.State)
	}
	wantReasons := []ReasonCode{ReasonExemptionIneffective, ReasonPrincipalRelationUnproven}
	if !reflect.DeepEqual(got.Reasons, wantReasons) {
		t.Fatalf("Reasons = %v, want %v", got.Reasons, wantReasons)
	}
	if err := validateMechanicalEvaluation("row", got); err != nil {
		t.Fatalf("result failed the package's own validation: %v", err)
	}
}

// TestApplyEffectiveExemptionsPrincipalRelationBothOperatorsSurvive covers
// the kernel-free half §5.3 fixes: when both relation operators remain in
// the remainder, the rerun proves the textual contradiction again without
// any kernel call.
func TestApplyEffectiveExemptionsPrincipalRelationBothOperatorsSurvive(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	row := evaluateOneRow(t, MechanicalInput{Claims: []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpSamePrincipal, "author", "reviewer", universalScope())),
		typedClaim(t, "policy-b", principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope())),
		typedClaim(t, "policy-c", principalClaim("c3", "release", policyartifact.OpSamePrincipal, "reviewer", "author", universalScope())),
	}, Profile: profile})
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("precondition: row.State = %q, want violated-with-witness", row.State)
	}
	var oneSame TypedClaimRecord
	for _, c := range row.Claims {
		if c.Claim.ID == "c3" {
			oneSame = c
		}
	}
	resolution := exemptionFor("ex-1", allProvenResolution(), oneSame)
	got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
	if err != nil {
		t.Fatalf("ApplyEffectiveExemptions: %v", err)
	}
	if got.After.State != SolverUnsatisfiable {
		t.Fatalf("After.State = %q, want unsatisfiable (same+different both survive)", got.After.State)
	}
	if got.State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want unchanged violated-with-witness", got.State)
	}
}
