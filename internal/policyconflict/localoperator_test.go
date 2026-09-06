package policyconflict

import (
	"context"
	"testing"

	"github.com/jyang234/verdi/internal/governanceprincipal"
)

// localOperatorPolicyYAML declares one local-operator trust source ("local")
// under the existing shared testCatalog() (mechanical_test.go): role
// "author" is bound to subject "bound@example.com" only, so any other
// subject resolves violated-with-witness rather than authenticated.
const localOperatorPolicyYAML = `schema: verdi.governance-profile/v1
id: solo-local
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: local, kind: local-operator }
role_mappings:
  - role: author
    trust_source: local
    subjects: ["bound@example.com"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`

func localOperatorFact(subjects ...string) governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID: "local", SourceKind: governanceprincipal.TrustSourceLocalOperator,
		Subjects: subjects, EvidenceDigest: testDigest64, Available: true, Valid: true,
	}
}

func resolveLocalOperatorActor(t *testing.T, profile governanceprincipal.Profile, subject string, fact governanceprincipal.TrustFact) governanceprincipal.PrincipalResolution {
	t.Helper()
	r := governanceprincipal.NewResolver(staticFact(fact))
	res, err := r.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "local", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return res
}

func authenticatedLocalActor(t *testing.T, profile governanceprincipal.Profile, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	res := resolveLocalOperatorActor(t, profile, subject, localOperatorFact(subject))
	if res.State != governanceprincipal.ResolutionAuthenticated {
		t.Fatalf("resolution state = %q, want authenticated", res.State)
	}
	return res
}

func violatedLocalActor(t *testing.T, profile governanceprincipal.Profile, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	res := resolveLocalOperatorActor(t, profile, subject, localOperatorFact(subject))
	if res.State != governanceprincipal.ResolutionViolated {
		t.Fatalf("resolution state = %q, want violated-with-witness", res.State)
	}
	return res
}

func unprovenLocalActor(t *testing.T, profile governanceprincipal.Profile, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	res := resolveLocalOperatorActor(t, profile, subject, governanceprincipal.TrustFact{
		SourceID: "local", SourceKind: governanceprincipal.TrustSourceLocalOperator,
		Available: false, Reason: "no git identity configured",
	})
	if res.State != governanceprincipal.ResolutionUnproven {
		t.Fatalf("resolution state = %q, want unproven", res.State)
	}
	return res
}

// TestLocalOperatorDisclosures_NoSourceDeclared proves a profile that
// declares no local-operator trust source at all never carries the
// disclosure, even when non-local-operator actors are present — the
// byte-identity guarantee design §2.2 requires for every profile that does
// not opt in.
func TestLocalOperatorDisclosures_NoSourceDeclared(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	actor := authenticatedActor(t, profile, "user-a")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{actor})
	if len(got) != 0 {
		t.Fatalf("disclosures = %#v, want none", got)
	}
}

// TestLocalOperatorDisclosures_SourceDeclaredNoActors proves declaring the
// source alone (no actor resolved against it — e.g. the request needed no
// approval at all) never manufactures the disclosure.
func TestLocalOperatorDisclosures_SourceDeclaredNoActors(t *testing.T) {
	profile := mustDecodeProfile(t, localOperatorPolicyYAML)
	got := localOperatorDisclosures(profile, nil)
	if len(got) != 0 {
		t.Fatalf("disclosures = %#v, want none", got)
	}
}

// TestLocalOperatorDisclosures_SourceDeclaredUnrelatedActor proves an actor
// resolved against a DIFFERENT (non-local-operator) trust source never
// triggers the disclosure merely because the profile also happens to
// declare a local-operator source.
func TestLocalOperatorDisclosures_SourceDeclaredUnrelatedActor(t *testing.T) {
	mixed := `schema: verdi.governance-profile/v1
id: solo-mixed
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: local, kind: local-operator }
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: local
    subjects: ["bound@example.com"]
  - role: reviewer
    trust_source: github
    subjects: ["user-b"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`
	profile := mustDecodeProfile(t, mixed)
	actor := authenticatedActor(t, profile, "user-b")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{actor})
	if len(got) != 0 {
		t.Fatalf("disclosures = %#v, want none for a non-local-operator actor", got)
	}
}

// TestLocalOperatorDisclosures_Authenticated proves a genuine
// local-operator resolution carries exactly the one disclosure, coded and
// witnessed by the trust source id.
func TestLocalOperatorDisclosures_Authenticated(t *testing.T) {
	profile := mustDecodeProfile(t, localOperatorPolicyYAML)
	actor := authenticatedLocalActor(t, profile, "bound@example.com")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{actor})
	want := []Disclosure{{Code: DisclosureLocalOperatorAsserted, Witnesses: []string{"local"}}}
	if len(got) != 1 || got[0].Code != want[0].Code || len(got[0].Witnesses) != 1 || got[0].Witnesses[0] != "local" {
		t.Fatalf("disclosures = %#v, want %#v", got, want)
	}
}

// TestLocalOperatorDisclosures_ViolatedStillDiscloses proves the disclosure
// fires even when the local-operator resolution itself is
// violated-with-witness: an attempted self-asserted resolution that FAILED
// still means the evaluation touched self-asserted identity, and that must
// never be hidden by the outcome (design §2.2's "no consumer can read such
// a pass as an authenticated-identity pass" cuts both ways: absence of the
// disclosure must never be misread as "no local-operator path was tried").
func TestLocalOperatorDisclosures_ViolatedStillDiscloses(t *testing.T) {
	profile := mustDecodeProfile(t, localOperatorPolicyYAML)
	actor := violatedLocalActor(t, profile, "unbound@example.com")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{actor})
	if len(got) != 1 || got[0].Code != DisclosureLocalOperatorAsserted {
		t.Fatalf("disclosures = %#v, want exactly one local-operator-asserted", got)
	}
}

// TestLocalOperatorDisclosures_UnprovenStillDiscloses mirrors the violated
// case for an unavailable (unproven) resolution.
func TestLocalOperatorDisclosures_UnprovenStillDiscloses(t *testing.T) {
	profile := mustDecodeProfile(t, localOperatorPolicyYAML)
	actor := unprovenLocalActor(t, profile, "placeholder")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{actor})
	if len(got) != 1 || got[0].Code != DisclosureLocalOperatorAsserted {
		t.Fatalf("disclosures = %#v, want exactly one local-operator-asserted", got)
	}
}

// TestLocalOperatorDisclosures_ExactlyOnce proves several local-operator
// actor resolutions in the same report still collapse to exactly one
// disclosure entry (never a duplicate code, matching every other
// report-level disclosure's set semantics).
func TestLocalOperatorDisclosures_ExactlyOnce(t *testing.T) {
	profile := mustDecodeProfile(t, localOperatorPolicyYAML)
	a := authenticatedLocalActor(t, profile, "bound@example.com")
	b := violatedLocalActor(t, profile, "someone-else@example.com")
	got := localOperatorDisclosures(profile, []governanceprincipal.PrincipalResolution{a, b})
	if len(got) != 1 {
		t.Fatalf("disclosures = %#v, want exactly one entry", got)
	}
	if err := validateDisclosure("test", got[0]); err != nil {
		t.Fatalf("validateDisclosure: %v", err)
	}
}
