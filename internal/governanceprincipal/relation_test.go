package governanceprincipal

import (
	"context"
	"testing"
)

func TestPrincipalRelation(t *testing.T) {
	t.Parallel()

	profile := mustDecode(t, profileYAML())
	resolve := func(t *testing.T, subject string, state ResolutionState) PrincipalResolution {
		t.Helper()
		var fact TrustFact
		switch state {
		case ResolutionAuthenticated:
			fact = githubFact(subject)
		case ResolutionViolated:
			fact = githubFact("someone-else")
		case ResolutionUnproven:
			fact = TrustFact{SourceID: "github", SourceKind: TrustSourceForge, Reason: "unavailable"}
		default:
			t.Fatalf("unsupported state %q", state)
		}
		got, err := NewResolver(staticFact(fact)).Resolve(context.Background(), profile, PrincipalClaim{TrustSource: "github", Subject: subject})
		if err != nil {
			t.Fatalf("Resolve(%q): %v", subject, err)
		}
		return got
	}

	alice := resolve(t, "user-123", ResolutionAuthenticated)
	aliceAgain := resolve(t, "user-123", ResolutionAuthenticated)
	bob := resolve(t, "user-456", ResolutionAuthenticated)
	violated := resolve(t, "user-789", ResolutionViolated)
	unproven := resolve(t, "user-999", ResolutionUnproven)

	tests := []struct {
		name     string
		left     PrincipalResolution
		right    PrincipalResolution
		relation DistinctnessRelation
		want     AuthorizationState
	}{
		{"same holds", alice, aliceAgain, RelationSamePrincipal, AuthorizationAuthorized},
		{"same contradicted", alice, bob, RelationSamePrincipal, AuthorizationViolated},
		{"different holds", alice, bob, RelationDifferentPrincipal, AuthorizationAuthorized},
		{"different contradicted", alice, aliceAgain, RelationDifferentPrincipal, AuthorizationViolated},
		{"left violated", violated, alice, RelationDifferentPrincipal, AuthorizationViolated},
		{"right violated outranks unproven", unproven, violated, RelationDifferentPrincipal, AuthorizationViolated},
		{"left unproven", unproven, alice, RelationDifferentPrincipal, AuthorizationUnproven},
		{"right unproven", alice, unproven, RelationDifferentPrincipal, AuthorizationUnproven},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := EvaluateRelation(tc.left, tc.right, tc.relation)
			if err != nil {
				t.Fatalf("EvaluateRelation: %v", err)
			}
			if got != tc.want {
				t.Fatalf("EvaluateRelation = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrincipalRelationRejectsMalformedOperands(t *testing.T) {
	t.Parallel()

	profile := mustDecode(t, profileYAML())
	valid, err := NewResolver(staticFact(githubFact("user-123"))).Resolve(context.Background(), profile, PrincipalClaim{TrustSource: "github", Subject: "user-123"})
	if err != nil {
		t.Fatal(err)
	}

	mutated := valid
	mutated.PrincipalID = mustPID(t, "user-456")
	tests := []struct {
		name     string
		left     PrincipalResolution
		right    PrincipalResolution
		relation DistinctnessRelation
	}{
		{"unknown relation", valid, valid, DistinctnessRelation("nearby")},
		{"forged left", PrincipalResolution{Claim: valid.Claim, State: ResolutionAuthenticated, PrincipalID: valid.PrincipalID, Witnesses: []Witness{}}, valid, RelationSamePrincipal},
		{"mutated right", valid, mutated, RelationSamePrincipal},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := EvaluateRelation(tc.left, tc.right, tc.relation); err == nil {
				t.Fatal("EvaluateRelation unexpectedly accepted malformed operands")
			}
		})
	}
}
