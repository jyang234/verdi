package policyconflict

import (
	"context"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// exemptionClaimCategory is the closed policyartifact witness category
// this package uses when a ClaimWitness names a typed mechanical claim
// (the nine-value vocabulary policyartifact.ValidateWitnessCategory
// enforces is authored for §6 prose claims; "constraint" is the closest
// semantic fit for a typed policy constraint claim — see the task report's
// disclosed judgment call).
const exemptionClaimCategory = "constraint"

func allProvenResolution() AuthorityResolution {
	return AuthorityResolution{Match: ProofProven, Freshness: ProofProven, Scope: ProofProven, Bound: ProofProven, Authorization: ProofProven}
}

func exemptionFor(id string, resolution AuthorityResolution, removed ...TypedClaimRecord) ExemptionResolution {
	witnesses := make([]ClaimWitness, len(removed))
	for i, c := range removed {
		witnesses[i] = ClaimWitness{ID: c.PolicyID + "/" + c.ClaimDigest, Digest: c.ClaimDigest, Category: exemptionClaimCategory}
	}
	return ExemptionResolution{
		ID: id, Digest: testDigest64, Resolution: resolution, RemovedClaims: witnesses,
	}
}

func evaluateOneRow(t *testing.T, in MechanicalInput) MechanicalEvaluation {
	t.Helper()
	rows, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
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
	removed := row.Claims[0]

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
			resolution := exemptionFor("ex-"+f.name, f.mk(allProvenResolution()), removed)
			got, err := ApplyEffectiveExemptions(row, []ExemptionResolution{resolution})
			if err != nil {
				t.Fatalf("ApplyEffectiveExemptions: %v", err)
			}
			if len(got.Exemptions) != 0 {
				t.Fatalf("Exemptions = %+v, want none applied (not all-proven)", got.Exemptions)
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
	if len(got.Exemptions) != 1 || len(got.Exemptions[0].RemovedClaims) != 1 || got.Exemptions[0].RemovedClaims[0].Digest != silver.ClaimDigest {
		t.Fatalf("Exemptions = %+v, want exactly the named removed claim", got.Exemptions)
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

func TestApplyEffectiveExemptionsUnknownClaimDigestIsOperationalError(t *testing.T) {
	row := violatedRow(t)
	ghost := TypedClaimRecord{PolicyID: "policy-ghost", PolicyDigest: testDigest64, ClaimDigest: testDigest64}
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
	// remains. ApplyEffectiveExemptions carries no kernel context (its
	// signature has no ctx/profile/actors), so it cannot re-ask the kernel
	// and must conservatively retain the original proof rather than invent
	// a fresh one.
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
	if got.State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want unchanged violated-with-witness (kernel unavailable, conservative)", got.State)
	}
	if !reflect.DeepEqual(got.After, row.Before) {
		t.Fatalf("After = %+v, want equal to the original Before (conservative retention)", got.After)
	}
}
