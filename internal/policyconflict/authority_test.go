package policyconflict

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// =============================================================================
// Shared fixture builders (authority.go's own tests only — mechanical_test.go
// and exemption_test.go already contribute typedClaim, discreteClaim,
// testDigest64, phaseScope, refScope, testCatalog, staticFact, readerFunc;
// scope_test.go contributes scopeWith/scopeRefs; operand_test.go contributes
// universalScope. None of those are redeclared here.)
// =============================================================================

const testDigest64B = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// kebab folds an arbitrary test-case name into policyartifact's kebab-case
// id-name grammar (lowercase [a-z0-9], hyphen-separated, no leading/
// trailing/duplicate hyphens) so table-driven subtest names can double as
// exemption/disposition artifact names.
func kebab(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case !prevDash:
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// --- mechanical row fixtures -------------------------------------------------

// authorityTypedClaim builds a TypedClaimRecord with policyID in the SAME
// kinded "policy/<name>" form policyartifact.Exemption's own decoded
// witnesses always carry (DecodeExemption validates Witness.Policy against
// exactly that grammar) — the two identity spaces meet for the first time
// in Task 8, so this fixture deliberately matches production shape rather
// than the bare test-only "policy-a" strings mechanical_test.go/
// exemption_test.go use for THEIR OWN self-contained comparisons.
func authorityTypedClaim(t *testing.T, policyID string, claim policyartifact.Claim) TypedClaimRecord {
	t.Helper()
	digest, err := policyartifact.ClaimDigest(claim)
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}
	return TypedClaimRecord{PolicyID: policyID, PolicyDigest: testDigest64, ClaimDigest: digest, Claim: claim}
}

// authorityRow builds a MechanicalEvaluation satisfying validateRowOperand
// (the only structural precondition ResolveExemptionAuthority enforces on
// its row argument) with an arbitrary but internally consistent Before/
// After/State/Reasons — those fields are never read by authority
// resolution, only Claims and Scope are.
func authorityRow(id string, claims []TypedClaimRecord, scope ScopeProof) MechanicalEvaluation {
	solver := SolverProof{State: SolverUnsatisfiable, Domain: domainDiscreteSet, Values: []string{}, Required: []string{}, Forbidden: []string{}, Witnesses: []string{}}
	return MechanicalEvaluation{
		ID:         id,
		Family:     policyartifact.FamilyConfiguration,
		Subject:    "test-subject",
		Claims:     claims,
		Scope:      scope,
		Domain:     domainDiscreteSet,
		Before:     solver,
		Exemptions: []ExemptionResolution{},
		After:      solver,
		State:      ProofViolatedWithWitness,
		Reasons:    []ReasonCode{ReasonMechanicalConflict},
	}
}

// --- scope-dimension fixtures ------------------------------------------------

func universalDim(name string) DimensionProof {
	return DimensionProof{Dimension: name, State: ScopeOverlap, Left: []string{}, Right: []string{}, Intersection: []string{}, Witnesses: []string{}}
}

func disjointDim(name string) DimensionProof {
	return DimensionProof{Dimension: name, State: ScopeDisjoint, Left: []string{"only-left"}, Right: []string{"only-right"}, Intersection: []string{}, Witnesses: []string{}}
}

func overlapDim(name string, intersection ...string) DimensionProof {
	return DimensionProof{Dimension: name, State: ScopeOverlap, Left: intersection, Right: intersection, Intersection: intersection, Witnesses: intersection}
}

func unknownDim(name string) DimensionProof {
	return DimensionProof{Dimension: name, State: ScopeUnknown, Left: []string{}, Right: []string{}, Intersection: []string{}, Witnesses: []string{}}
}

// fourDims builds a complete phase/environment/path/ref ScopeProof in the
// wire's fixed order.
func fourDims(phase, env, path, ref DimensionProof) ScopeProof {
	return ScopeProof{State: ScopeOverlap, Dimensions: []DimensionProof{phase, env, path, ref}}
}

// --- ref coverage resolver fake ----------------------------------------------

type coverageCall struct{ container, member string }

type fakeCoverageResponse struct {
	state ProofState
	wit   []string
}

// fakeCoverageResolver is a hermetic, deterministic RefCoverageResolver
// fake, mirroring scope_test.go's fakeResolver but DIRECTIONAL (container,
// member are never canonicalized) — the exact shape needed to prove a
// symmetric answer for the reversed pair is never accepted as coverage.
type fakeCoverageResolver struct {
	t         *testing.T
	responses map[coverageCall]fakeCoverageResponse
	calls     []coverageCall
	err       error
	bogus     bool
}

func newFakeCoverageResolver(t *testing.T) *fakeCoverageResolver {
	t.Helper()
	return &fakeCoverageResolver{t: t, responses: make(map[coverageCall]fakeCoverageResponse)}
}

func (f *fakeCoverageResolver) set(container, member string, state ProofState, wit ...string) *fakeCoverageResolver {
	f.responses[coverageCall{container, member}] = fakeCoverageResponse{state: state, wit: append([]string(nil), wit...)}
	return f
}

func (f *fakeCoverageResolver) Covers(_ context.Context, container, member string) (ProofState, []string, error) {
	f.calls = append(f.calls, coverageCall{container, member})
	if f.err != nil {
		return "", nil, f.err
	}
	if f.bogus {
		return ProofState("not-a-real-state"), nil, nil
	}
	r, ok := f.responses[coverageCall{container, member}]
	if !ok {
		f.t.Fatalf("fakeCoverageResolver: no response configured for container=%q member=%q", container, member)
	}
	return r.state, append([]string(nil), r.wit...), nil
}

// noCallCoverageResolver fails the test the instant Covers is invoked —
// proves a code path never reaches the ref port (exact/universal/phase/
// environment/path cases).
type noCallCoverageResolver struct{ t *testing.T }

func (n noCallCoverageResolver) Covers(_ context.Context, container, member string) (ProofState, []string, error) {
	n.t.Fatalf("resolver.Covers called unexpectedly for container=%q member=%q", container, member)
	return "", nil, nil
}

var errCoverageBoom = fmt.Errorf("fake coverage resolver: boom")

// --- exemption artifact fixture ----------------------------------------------

type exemptionFixture struct {
	Name      string
	Scope     policyartifact.Scope
	Witnesses []policyartifact.Witness
	Approvals []policyartifact.Approval
	Expiry    string
	Review    string
}

func yamlList(vals []string) string {
	quoted := make([]string, len(vals))
	for i, v := range vals {
		quoted[i] = fmt.Sprintf("%q", v)
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func scopeYAML(s policyartifact.Scope) string {
	return fmt.Sprintf("{phases: %s, environments: %s, paths: %s, refs: %s}",
		yamlList(s.Phases), yamlList(s.Environments), yamlList(s.Paths), yamlList(s.Refs))
}

func defaultApprovals(t *testing.T) []policyartifact.Approval {
	t.Helper()
	return []policyartifact.Approval{{Role: "policy-owner", Principal: authorityPrincipalID(t, "alice")}}
}

// buildExemption renders f as a complete verdi.policy-exemption/v1 document
// and decodes it, so every returned value is genuine, sealed DecodeExemption
// output — the same provenance a real caller's AuthorityInput.Exemptions
// entries always carry (ExemptionResolution.Digest calls e.Digest(), which
// requires an intact seal).
func buildExemption(t *testing.T, f exemptionFixture) policyartifact.Exemption {
	t.Helper()
	if f.Expiry == "" && f.Review == "" {
		f.Expiry = "2026-12-31"
	}
	var witnessLines strings.Builder
	for _, w := range f.Witnesses {
		fmt.Fprintf(&witnessLines, "  - policy: %s\n    claim: %s\n    claim_digest: %q\n", w.Policy, w.Claim, w.ClaimDigest)
	}
	var approvalLines strings.Builder
	for _, a := range f.Approvals {
		fmt.Fprintf(&approvalLines, "  - role: %s\n    principal: %s\n", a.Role, a.Principal)
	}
	var boundLines strings.Builder
	if f.Expiry != "" {
		fmt.Fprintf(&boundLines, "expiry: %q\n", f.Expiry)
	}
	if f.Review != "" {
		fmt.Fprintf(&boundLines, "review_condition: %q\n", f.Review)
	}

	doc := fmt.Sprintf(`---
schema: verdi.policy-exemption/v1
id: policy-exemption/%s
kind: policy-exemption
title: "Test exemption %s"
owners: [test-team]
scope: %s
witnesses:
%scompensating_controls:
  - "Weekly review."
approvals:
%s%stemplate: {identity: "test", digest: %q}
---
Test rationale body.
`, f.Name, f.Name, scopeYAML(f.Scope), witnessLines.String(), approvalLines.String(), boundLines.String(), testDigest64)

	e, err := policyartifact.DecodeExemption([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeExemption(%s): %v\n---\n%s", f.Name, err, doc)
	}
	return *e
}

// --- disposition artifact fixture --------------------------------------------

type dispositionFixture struct {
	Name       string
	Scope      policyartifact.Scope
	InputID    string
	Target     string
	Claims     []policyartifact.SemanticClaimWitness
	Exemptions []policyartifact.SemanticExemptionWitness
	Conclusion policyartifact.DispositionConclusion
	Origin     policyartifact.DispositionOrigin
	Approvals  []policyartifact.Approval
	Expiry     string
	Review     string
}

func witnessClaimsYAML(claims []policyartifact.SemanticClaimWitness) string {
	var b strings.Builder
	for _, c := range claims {
		fmt.Fprintf(&b, "    - id: %s\n      digest: %q\n      category: %s\n      authority_digest: %q\n      scope: %s\n      values: %s\n",
			c.ID, c.Digest, c.Category, c.AuthorityDigest, scopeYAML(c.Scope), yamlList(c.Values))
		if c.Bound != nil {
			fmt.Fprintf(&b, "      bound: %d\n", *c.Bound)
		}
	}
	return b.String()
}

func witnessExemptionsYAML(exs []policyartifact.SemanticExemptionWitness) string {
	if len(exs) == 0 {
		return "  exemptions: []\n"
	}
	var b strings.Builder
	b.WriteString("  exemptions:\n")
	for _, e := range exs {
		fmt.Fprintf(&b, "    - id: %s\n      digest: %q\n", e.ID, e.Digest)
	}
	return b.String()
}

// buildDisposition renders f as a complete verdi.policy-disposition/v1
// document and decodes it — genuine, sealed DecodeDisposition output.
func buildDisposition(t *testing.T, f dispositionFixture) policyartifact.Disposition {
	t.Helper()
	if len(f.Claims) == 0 {
		t.Fatalf("buildDisposition(%s): at least one witness claim is required", f.Name)
	}

	var approvalLines strings.Builder
	for _, a := range f.Approvals {
		fmt.Fprintf(&approvalLines, "  - role: %s\n    principal: %s\n", a.Role, a.Principal)
	}

	var trailer strings.Builder
	switch f.Origin {
	case policyartifact.DispositionJudgeResult:
		fmt.Fprintf(&trailer, "judgment:\n  primary_digest: %q\n", testDigest64)
	case policyartifact.DispositionHumanFallback:
		if f.Expiry != "" {
			fmt.Fprintf(&trailer, "expiry: %q\n", f.Expiry)
		}
		if f.Review != "" {
			fmt.Fprintf(&trailer, "review_condition: %q\n", f.Review)
		}
		trailer.WriteString("compensating_controls:\n  - \"Manual weekly review.\"\n")
	}

	doc := fmt.Sprintf(`---
schema: verdi.policy-disposition/v1
id: policy-disposition/%s
kind: policy-disposition
title: "Test disposition %s"
owners: [test-team]
scope: %s
witness:
  input_id: %q
  target_digest: %q
  claims:
%s%sconclusion: %s
origin: %s
%sapprovals:
%stemplate: {identity: "test", digest: %q}
---
Test rationale body.
`, f.Name, f.Name, scopeYAML(f.Scope), f.InputID, f.Target, witnessClaimsYAML(f.Claims), witnessExemptionsYAML(f.Exemptions),
		f.Conclusion, f.Origin, trailer.String(), approvalLines.String(), testDigest64)

	d, err := policyartifact.DecodeDisposition([]byte(doc))
	if err != nil {
		t.Fatalf("DecodeDisposition(%s): %v\n---\n%s", f.Name, err, doc)
	}
	return *d
}

// --- semantic input fixture ---------------------------------------------------

// authorityProseClaim builds a real, fully valid policy-instruction prose
// claim (the simplest of semantic.go's nine closed categories to satisfy).
func authorityProseClaim(t *testing.T, sourceRef string, n int, text string) contextcompile.ProseClaim {
	t.Helper()
	object := fmt.Sprintf("instruction-%d", n)
	id := sourceRef + "#" + object
	return contextcompile.ProseClaim{
		ID:              id,
		Category:        "policy-instruction",
		Text:            text,
		TextDigest:      rawContentDigest([]byte(text)),
		SourceRef:       sourceRef,
		SourcePath:      "policy/test-policy.md",
		SourceDigest:    testDigest64,
		Scope:           universalScope(),
		AuthorityDigest: testDigest64,
		Object:          object,
		LineIdentity:    id,
	}
}

// authoritySemanticInput builds one valid, current SemanticInput with two
// sorted prose claims and one applicable exemption identity.
func authoritySemanticInput(t *testing.T) SemanticInput {
	t.Helper()
	claims := []contextcompile.ProseClaim{
		authorityProseClaim(t, "policy/test-policy", 1, "First instruction."),
		authorityProseClaim(t, "policy/test-policy", 2, "Second instruction."),
	}
	exemptions := []policyartifact.SemanticExemptionWitness{
		{ID: "policy-exemption/current-ex", Digest: testDigest64},
	}
	return SemanticInput{
		Claims:             claims,
		UnknownMechanicals: []UnknownMechanicalWitness{},
		Exemptions:         exemptions,
		Prompt:             []byte(semanticPrompt),
	}
}

// witnessClaimsFrom mirrors a SemanticInput's prose claims into the
// disposition witness's own wire shape, EXACTLY matching identity (id,
// digest, category) — the fixture used to build a genuinely CURRENT,
// matching disposition.
func witnessClaimsFrom(claims []contextcompile.ProseClaim) []policyartifact.SemanticClaimWitness {
	out := make([]policyartifact.SemanticClaimWitness, len(claims))
	for i, c := range claims {
		out[i] = policyartifact.SemanticClaimWitness{
			ID: c.ID, Digest: c.TextDigest, Category: c.Category,
			AuthorityDigest: c.AuthorityDigest, Scope: c.Scope, Values: []string{},
		}
	}
	return out
}

// --- governance fixtures ------------------------------------------------------

func authorityCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:       []string{"author", "policy-owner"},
		Transitions: []string{"policy-exemption-approval", "policy-disposition-approval"},
	}
}

// authorityProfileYAML builds a profile applicable to BOTH SI-113 fixed
// transitions, with author/policy-owner roles mapped to alice/bob — dave
// deliberately has NO role mapping so a caller can exercise "unknown role".
// governanceprincipal's own class-coverage validation (validate.go)
// requires team/high-assurance profiles to carry a different-principal
// distinctness rule for every applicable transition, and high-assurance
// additionally requires signature/ownership/evidence-source rule coverage;
// solo and experimental profiles carry none of that (solo's permitted
// collapse has no distinctness rule to violate).
func authorityProfileYAML(class string) []byte {
	distinctness := "distinctness_rules: []\n"
	signature := "signature_requirements: []\n"
	ownership := "ownership_sources: []\n"
	evidence := "evidence_source_restrictions: []\n"
	trustSources := "  - { id: github, kind: forge }\n"

	switch class {
	case "team", "high-assurance":
		distinctness = `distinctness_rules:
  - transitions: [policy-exemption-approval, policy-disposition-approval]
    left_role: author
    right_role: policy-owner
    relation: different-principal
`
	}
	if class == "high-assurance" {
		trustSources += "  - { id: git-signature, kind: signed-commit }\n  - { id: codeowners, kind: ownership }\n"
		signature = `signature_requirements:
  - transitions: [policy-exemption-approval, policy-disposition-approval]
    roles: [policy-owner]
    trust_sources: [git-signature]
`
		ownership = `ownership_sources:
  - id: owner-check
    trust_source: codeowners
    transitions: [policy-exemption-approval, policy-disposition-approval]
    roles: [policy-owner]
`
		evidence = `evidence_source_restrictions:
  - transitions: [policy-exemption-approval, policy-disposition-approval]
    allowed_sources: []
`
	}

	return []byte(fmt.Sprintf(`schema: verdi.governance-profile/v1
id: authority-test-%s
class: %s
applicable_transitions: [policy-exemption-approval, policy-disposition-approval]
identity_trust_sources:
%srole_mappings:
  - role: author
    trust_source: github
    subjects: ["alice", "bob"]
  - role: policy-owner
    trust_source: github
    subjects: ["alice", "bob"]
%s%srequired_approvers:
  - transitions: [policy-exemption-approval, policy-disposition-approval]
    roles: [policy-owner]
    minimum: 1
%s%sescalation_thresholds: []
`, class, class, trustSources, ownership, signature, distinctness, evidence))
}

// authorityProfileMissingTransitionYAML builds a solo profile whose
// applicable_transitions omits missing (SI-113/§9: "A missing or mismatched
// transition remains the kernel's explicit non-authorizing finding and
// never becomes a favorable default"). Solo keeps the fixture minimal:
// governanceprincipal's class-coverage validation only applies to
// team/high-assurance, and Authorize short-circuits on transition
// applicability before any approver/distinctness rule is even consulted,
// so no other rule content is load-bearing here.
func authorityProfileMissingTransitionYAML(missing string) []byte {
	all := []string{"policy-exemption-approval", "policy-disposition-approval"}
	kept := make([]string, 0, 1)
	for _, t := range all {
		if t != missing {
			kept = append(kept, t)
		}
	}
	return []byte(fmt.Sprintf(`schema: verdi.governance-profile/v1
id: authority-test-missing-transition
class: solo
applicable_transitions: %s
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["alice", "bob"]
  - role: policy-owner
    trust_source: github
    subjects: ["alice", "bob"]
ownership_sources: []
signature_requirements: []
required_approvers:
  - transitions: [policy-exemption-approval, policy-disposition-approval]
    roles: [policy-owner]
    minimum: 1
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`, yamlList(kept)))
}

func decodeAuthorityProfile(t *testing.T, raw []byte) governanceprincipal.Profile {
	t.Helper()
	p, err := governanceprincipal.DecodeProfile(raw, authorityCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: %v\n%s", err, raw)
	}
	return p
}

func authorityPrincipalID(t *testing.T, subject string) string {
	t.Helper()
	id, err := governanceprincipal.CanonicalPrincipalID("github", subject)
	if err != nil {
		t.Fatalf("CanonicalPrincipalID(%s): %v", subject, err)
	}
	return string(id)
}

func authenticatedFact(subject string) governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
		Available: true, Valid: true, Subjects: []string{subject}, EvidenceDigest: testDigest64,
	}
}

func violatedFact() governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
		Available: true, Valid: false, EvidenceDigest: testDigest64, Reason: "bad evidence",
	}
}

func unavailableFact() governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
		Available: false, Reason: "no evidence",
	}
}

// referenceProfile is any profile sharing authorityProfileYAML's trust
// sources — resolutions minted against it remain valid resolver output
// (self-contained seal) regardless of which profile variant later consumes
// them in an AuthorizationRequest.
func referenceProfile(t *testing.T) governanceprincipal.Profile {
	t.Helper()
	// solo: default fixtures only ever supply a single "policy-owner"
	// approval, and solo is the one class governanceprincipal's own
	// class-coverage validation never mandates an author-role distinctness
	// rule for — every non-authorization-focused test wants a plain
	// authorized/whatever-matters outcome, not an incidental unproven
	// distinctness finding from an "author" role no fixture here fills.
	return decodeAuthorityProfile(t, authorityProfileYAML("solo"))
}

func authorityResolve(t *testing.T, subject string, fact governanceprincipal.TrustFact) governanceprincipal.PrincipalResolution {
	t.Helper()
	r := governanceprincipal.NewResolver(staticFact(fact))
	res, err := r.Resolve(context.Background(), referenceProfile(t), governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve(%s): %v", subject, err)
	}
	return res
}

// =============================================================================
// Exemption arm (authority design §5.5, ledger SI-115)
// =============================================================================

func TestResolveExemptionAuthorityOmitsUnrelatedExemption(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")))

	unrelated := buildExemption(t, exemptionFixture{
		Name:  "unrelated",
		Scope: universalScope(),
		Witnesses: []policyartifact.Witness{
			{Policy: "policy/policy-b", Claim: "other-claim", ClaimDigest: testDigest64},
		},
		Approvals: defaultApprovals(t),
	})

	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Exemptions:  []policyartifact.Exemption{unrelated},
	}
	got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
	if err != nil {
		t.Fatalf("ResolveExemptionAuthority: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("resolutions = %+v, want none (unrelated exemption must be omitted)", got)
	}
}

func TestResolveExemptionAuthorityApplicabilityAndFreshness(t *testing.T) {
	claimDigest, err := policyartifact.ClaimDigest(discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}

	tests := []struct {
		name          string
		witnessDigest string
		wantFreshness ProofState
	}{
		{"current digest is fresh", claimDigest, ProofProven},
		{"stale digest is violated", testDigest64B, ProofViolatedWithWitness},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
			row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")))

			ex := buildExemption(t, exemptionFixture{
				Name:  kebab("ex-" + tc.name),
				Scope: universalScope(),
				Witnesses: []policyartifact.Witness{
					{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: tc.witnessDigest},
				},
				Approvals: defaultApprovals(t),
			})

			in := AuthorityInput{
				EvaluatedOn: "2026-01-01",
				Profile:     referenceProfile(t),
				Actors:      []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
				Exemptions:  []policyartifact.Exemption{ex},
			}
			got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
			if err != nil {
				t.Fatalf("ResolveExemptionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one applicable", got)
			}
			if got[0].Resolution.Match != ProofProven {
				t.Fatalf("Match = %q, want proven (applicability is structural)", got[0].Resolution.Match)
			}
			if got[0].Resolution.Freshness != tc.wantFreshness {
				t.Fatalf("Freshness = %q, want %q", got[0].Resolution.Freshness, tc.wantFreshness)
			}
			if tc.wantFreshness == ProofProven {
				if !allProven(got[0].Resolution) {
					t.Fatalf("resolution = %+v, want all-proven (fully current, in-scope, bounded, authorized fixture)", got[0].Resolution)
				}
				want := []MechanicalClaimWitness{{PolicyID: "policy/policy-a", ClaimID: "c1", ClaimDigest: claimDigest}}
				if !reflect.DeepEqual(got[0].RemovedClaims, want) {
					t.Fatalf("RemovedClaims = %+v, want %+v", got[0].RemovedClaims, want)
				}
			} else {
				if got[0].RemovedClaims == nil || len(got[0].RemovedClaims) != 0 {
					t.Fatalf("RemovedClaims = %+v, want explicit empty (rejected resolution removed nothing)", got[0].RemovedClaims)
				}
			}
		})
	}
}

func TestResolveExemptionAuthorityRowLocalFreshness(t *testing.T) {
	claimX := discreteClaim("claim-x", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	claimY := discreteClaim("claim-y", "level", policyartifact.OpAllowedValues, []string{"silver"}, universalScope())
	digestX, err := policyartifact.ClaimDigest(claimX)
	if err != nil {
		t.Fatalf("ClaimDigest(x): %v", err)
	}
	// claim-y's committed witness digest is deliberately testDigest64B, not
	// its own real digest — it is stale by construction (see the exemption
	// fixture below), so its real digest is never needed here.

	// The exemption spans two witnesses; witness Y's committed digest is
	// already stale against the current claim (SI-115: "an exemption
	// spanning multiple rows can resolve each row without requiring
	// unrelated witnesses to occur there").
	ex := buildExemption(t, exemptionFixture{
		Name:  "spanning",
		Scope: universalScope(),
		Witnesses: []policyartifact.Witness{
			{Policy: "policy/policy-a", Claim: "claim-x", ClaimDigest: digestX},
			{Policy: "policy/policy-a", Claim: "claim-y", ClaimDigest: testDigest64B},
		},
		Approvals: defaultApprovals(t),
	})

	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))
	rowX := authorityRow("row-x", []TypedClaimRecord{authorityTypedClaim(t, "policy/policy-a", claimX)}, dims)
	rowY := authorityRow("row-y", []TypedClaimRecord{authorityTypedClaim(t, "policy/policy-a", claimY)}, dims)

	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Actors:      []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
		Exemptions:  []policyartifact.Exemption{ex},
	}

	gotX, err := ResolveExemptionAuthority(context.Background(), in, rowX, noCallCoverageResolver{t: t})
	if err != nil {
		t.Fatalf("ResolveExemptionAuthority(rowX): %v", err)
	}
	if len(gotX) != 1 || gotX[0].Resolution.Freshness != ProofProven {
		t.Fatalf("rowX resolutions = %+v, want one proven-fresh resolution (its own witness is current)", gotX)
	}

	gotY, err := ResolveExemptionAuthority(context.Background(), in, rowY, noCallCoverageResolver{t: t})
	if err != nil {
		t.Fatalf("ResolveExemptionAuthority(rowY): %v", err)
	}
	if len(gotY) != 1 || gotY[0].Resolution.Freshness != ProofViolatedWithWitness {
		t.Fatalf("rowY resolutions = %+v, want one violated-fresh resolution (its own witness is stale)", gotY)
	}
	if len(gotY[0].RemovedClaims) != 0 {
		t.Fatalf("rowY RemovedClaims = %+v, want explicit empty (rejected)", gotY[0].RemovedClaims)
	}
}

func TestResolveExemptionAuthorityScopeCoverage(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))

	tests := []struct {
		name     string
		dims     ScopeProof
		exScope  policyartifact.Scope
		resolver RefCoverageResolver
		want     ProofState
	}{
		{
			name:     "universal exemption scope covers everything",
			dims:     fourDims(overlapDim("phase", "design"), overlapDim("environment", "prod"), overlapDim("path", "cmd/x.go"), overlapDim("ref", "spec/feature-a")),
			exScope:  universalScope(),
			resolver: noCallCoverageResolver{t: t},
			want:     ProofProven,
		},
		{
			name:     "disjoint dimension trivially covered regardless of exemption content",
			dims:     fourDims(disjointDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{"build"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofProven,
		},
		{
			name:     "universal row dimension needs a universal exemption dimension",
			dims:     fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{"design"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofViolatedWithWitness,
		},
		{
			name:     "narrow phase/environment covering",
			dims:     fourDims(overlapDim("phase", "design"), overlapDim("environment", "prod"), universalDim("path"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{"design", "build"}, Environments: []string{"prod", "staging"}, Paths: []string{}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofProven,
		},
		{
			name:     "narrow phase not covering",
			dims:     fourDims(overlapDim("phase", "design"), universalDim("environment"), universalDim("path"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{"build"}, Environments: []string{}, Paths: []string{}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofViolatedWithWitness,
		},
		{
			name:     "unknown row dimension is unproven",
			dims:     fourDims(unknownDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")),
			exScope:  universalScope(),
			resolver: noCallCoverageResolver{t: t},
			want:     ProofUnproven,
		},
		{
			name:     "segment-aware path containment covers",
			dims:     fourDims(universalDim("phase"), universalDim("environment"), overlapDim("path", "cmd/verdi/main.go"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofProven,
		},
		{
			name:     "segment-aware path containment does not cross a segment boundary",
			dims:     fourDims(universalDim("phase"), universalDim("environment"), overlapDim("path", "cmdline/x"), universalDim("ref")),
			exScope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{"cmd/"}, Refs: []string{}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofViolatedWithWitness,
		},
		{
			name:     "exact ref equality covers without invoking the resolver",
			dims:     fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "spec/feature-a")),
			exScope:  policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
			resolver: noCallCoverageResolver{t: t},
			want:     ProofProven,
		},
		{
			name:    "directionally covered different ref",
			dims:    fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "spec/story-impl")),
			exScope: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
			resolver: newFakeCoverageResolver(t).
				set("spec/feature-a", "spec/story-impl", ProofProven),
			want: ProofProven,
		},
		{
			name:    "directionally uncovered different ref is violated",
			dims:    fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "spec/story-unrelated")),
			exScope: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
			resolver: newFakeCoverageResolver(t).
				set("spec/feature-a", "spec/story-unrelated", ProofViolatedWithWitness),
			want: ProofViolatedWithWitness,
		},
		{
			name:    "unknown ref coverage is unproven",
			dims:    fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "spec/story-maybe")),
			exScope: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
			resolver: newFakeCoverageResolver(t).
				set("spec/feature-a", "spec/story-maybe", ProofUnproven),
			want: ProofUnproven,
		},
		{
			name:    "symmetric overlap is never accepted as directional coverage",
			dims:    fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "spec/story-reverse-only")),
			exScope: policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
			resolver: newFakeCoverageResolver(t).
				// The FORWARD (container=exemption ref, member=row ref)
				// relation this package must call is proven false.
				set("spec/feature-a", "spec/story-reverse-only", ProofViolatedWithWitness).
				// The REVERSE relation is proven true — a resolver that
				// answered symmetrically would wrongly cover this ref.
				set("spec/story-reverse-only", "spec/feature-a", ProofProven),
			want: ProofViolatedWithWitness,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := authorityRow("row-1", []TypedClaimRecord{claim}, tc.dims)
			ex := buildExemption(t, exemptionFixture{
				Name:      kebab("scope-" + tc.name),
				Scope:     tc.exScope,
				Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
				Approvals: defaultApprovals(t),
			})
			in := AuthorityInput{
				EvaluatedOn: "2026-01-01",
				Profile:     referenceProfile(t),
				Actors:      []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
				Exemptions:  []policyartifact.Exemption{ex},
			}
			got, err := ResolveExemptionAuthority(context.Background(), in, row, tc.resolver)
			if err != nil {
				t.Fatalf("ResolveExemptionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			if got[0].Resolution.Scope != tc.want {
				t.Fatalf("Scope = %q, want %q", got[0].Resolution.Scope, tc.want)
			}
		})
	}
}

func TestResolveExemptionAuthorityScopeResolverFailure(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "story/x")))
	ex := buildExemption(t, exemptionFixture{
		Name:      "resolver-boom",
		Scope:     policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
		Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
		Approvals: defaultApprovals(t),
	})
	resolver := newFakeCoverageResolver(t)
	resolver.err = errCoverageBoom

	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Exemptions:  []policyartifact.Exemption{ex},
	}
	_, err := ResolveExemptionAuthority(context.Background(), in, row, resolver)
	if err == nil {
		t.Fatal("ResolveExemptionAuthority: want operational error on resolver failure, got nil")
	}
}

func TestResolveExemptionAuthorityMissingResolverIsOperational(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), overlapDim("ref", "story/x")))
	ex := buildExemption(t, exemptionFixture{
		Name:      "no-resolver",
		Scope:     policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{"spec/feature-a"}},
		Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
		Approvals: defaultApprovals(t),
	})
	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Exemptions:  []policyartifact.Exemption{ex},
	}
	_, err := ResolveExemptionAuthority(context.Background(), in, row, nil)
	if err == nil {
		t.Fatal("ResolveExemptionAuthority: want operational error with a nil resolver and a nonuniversal different-ref pair, got nil")
	}
}

func TestResolveExemptionAuthorityBound(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))

	tests := []struct {
		name        string
		evaluatedOn string
		expiry      string
		review      string
		want        ProofState
	}{
		{"expiry after evaluated_on is proven", "2026-01-01", "2026-12-31", "", ProofProven},
		{"expiry equal to evaluated_on is proven", "2026-01-01", "2026-01-01", "", ProofProven},
		{"expiry before evaluated_on is violated", "2026-06-01", "2026-01-01", "", ProofViolatedWithWitness},
		{"review-condition-only is unproven", "2026-01-01", "", "quarterly-review", ProofUnproven},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := authorityRow("row-1", []TypedClaimRecord{claim}, dims)
			ex := buildExemption(t, exemptionFixture{
				Name:      kebab("bound-" + tc.name),
				Scope:     universalScope(),
				Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
				Approvals: defaultApprovals(t),
				Expiry:    tc.expiry,
				Review:    tc.review,
			})
			in := AuthorityInput{
				EvaluatedOn: tc.evaluatedOn,
				Profile:     referenceProfile(t),
				Exemptions:  []policyartifact.Exemption{ex},
			}
			got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
			if err != nil {
				t.Fatalf("ResolveExemptionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			if got[0].Resolution.Bound != tc.want {
				t.Fatalf("Bound = %q, want %q", got[0].Resolution.Bound, tc.want)
			}
		})
	}
}

func TestResolveExemptionAuthorityMalformedEvaluatedOn(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")))

	for _, evaluatedOn := range []string{"", "not-a-date", "2026-13-40"} {
		t.Run(fmt.Sprintf("evaluated_on=%q", evaluatedOn), func(t *testing.T) {
			in := AuthorityInput{EvaluatedOn: evaluatedOn, Profile: referenceProfile(t)}
			_, err := ResolveExemptionAuthority(context.Background(), in, row, nil)
			if err == nil {
				t.Fatalf("ResolveExemptionAuthority: want operational error for evaluated_on %q, got nil", evaluatedOn)
			}
		})
	}
}

func TestResolveExemptionAuthorityMalformedRow(t *testing.T) {
	claim := discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	bad := TypedClaimRecord{PolicyID: "policy/policy-a", PolicyDigest: testDigest64, ClaimDigest: testDigest64B /* wrong */, Claim: claim}
	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))

	t.Run("stale carried claim digest", func(t *testing.T) {
		row := authorityRow("row-1", []TypedClaimRecord{bad}, dims)
		in := AuthorityInput{EvaluatedOn: "2026-01-01", Profile: referenceProfile(t)}
		_, err := ResolveExemptionAuthority(context.Background(), in, row, nil)
		if err == nil {
			t.Fatal("ResolveExemptionAuthority: want operational error for a hand-built row with a non-recomputing claim digest, got nil")
		}
	})

	t.Run("scope proof missing a dimension", func(t *testing.T) {
		row := authorityRow("row-1", []TypedClaimRecord{authorityTypedClaim(t, "policy/policy-a", claim)}, ScopeProof{
			State:      ScopeOverlap,
			Dimensions: []DimensionProof{universalDim("phase"), universalDim("environment"), universalDim("path")}, // no ref
		})
		in := AuthorityInput{EvaluatedOn: "2026-01-01", Profile: referenceProfile(t)}
		_, err := ResolveExemptionAuthority(context.Background(), in, row, nil)
		if err != nil {
			t.Fatalf("ResolveExemptionAuthority: unexpected error for a wire-legal subsequence scope proof with no applicable exemption: %v", err)
		}
	})
}

func TestResolveExemptionAuthorityDuplicateExemptionID(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")))

	// Same declared id "dup", two DIFFERENT (by content) exemption artifacts.
	e1 := buildExemption(t, exemptionFixture{Name: "dup", Scope: universalScope(), Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}}, Approvals: defaultApprovals(t), Expiry: "2026-06-01"})
	e2 := buildExemption(t, exemptionFixture{Name: "dup", Scope: universalScope(), Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}}, Approvals: defaultApprovals(t), Expiry: "2026-12-31"})

	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Exemptions:  []policyartifact.Exemption{e1, e2},
	}
	_, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
	if err == nil {
		t.Fatal("ResolveExemptionAuthority: want operational error for two different exemption artifacts sharing one id, got nil")
	}
}

func TestResolveExemptionAuthoritySortedByID(t *testing.T) {
	claimA := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	claimB := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c2", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))
	row := authorityRow("row-1", []TypedClaimRecord{claimA, claimB}, dims)

	exZ := buildExemption(t, exemptionFixture{Name: "zzz", Scope: universalScope(), Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claimA.ClaimDigest}}, Approvals: defaultApprovals(t)})
	exA := buildExemption(t, exemptionFixture{Name: "aaa", Scope: universalScope(), Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c2", ClaimDigest: claimB.ClaimDigest}}, Approvals: defaultApprovals(t)})

	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Exemptions:  []policyartifact.Exemption{exZ, exA},
	}
	got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
	if err != nil {
		t.Fatalf("ResolveExemptionAuthority: %v", err)
	}
	if len(got) != 2 || got[0].ID >= got[1].ID {
		t.Fatalf("resolutions = %+v, want two entries sorted ascending by id", got)
	}
}

// =============================================================================
// Disposition arm (authority design §8/§9, ledger SI-114/SI-115)
// =============================================================================

func matchingDispositionFixture(t *testing.T, name string, origin policyartifact.DispositionOrigin, si SemanticInput, inputID, target string, approvals []policyartifact.Approval) policyartifact.Disposition {
	t.Helper()
	f := dispositionFixture{
		Name:       name,
		Scope:      universalScope(),
		InputID:    inputID,
		Target:     target,
		Claims:     witnessClaimsFrom(si.Claims),
		Exemptions: si.Exemptions,
		Conclusion: policyartifact.DispositionNoConflict,
		Origin:     origin,
		Approvals:  approvals,
	}
	if origin == policyartifact.DispositionHumanFallback {
		f.Expiry = "2026-12-31"
	}
	return buildDisposition(t, f)
}

func TestResolveDispositionAuthorityExactMatchJudgeResult(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "exact-match", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))

	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Actors:       []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
		Dispositions: []policyartifact.Disposition{d},
	}
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("resolutions = %+v, want exactly one", got)
	}
	r := got[0].Resolution
	if r.Match != ProofProven || r.Freshness != ProofProven || r.Scope != ProofProven || r.Bound != ProofProven {
		t.Fatalf("resolution = %+v, want match/freshness/scope/bound all proven", r)
	}
	if got[0].Conclusion != policyartifact.DispositionNoConflict {
		t.Fatalf("Conclusion = %q, want no-conflict (verbatim passthrough)", got[0].Conclusion)
	}
}

func TestResolveDispositionAuthorityMismatch(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(f dispositionFixture) dispositionFixture
	}{
		{
			name: "stale input id",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.InputID = testDigest64B
				return f
			},
		},
		{
			name: "mismatched target digest",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Target = testDigest64
				return f
			},
		},
		{
			name: "mismatched claim digest",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Claims[0].Digest = testDigest64B
				return f
			},
		},
		{
			name: "mismatched claim category",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Claims[0].Category = "spec-problem"
				return f
			},
		},
		{
			name: "mismatched exemption identity",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Exemptions = []policyartifact.SemanticExemptionWitness{{ID: "policy-exemption/current-ex", Digest: testDigest64B}}
				return f
			},
		},
		{
			name: "extra exemption identity",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Exemptions = append(append([]policyartifact.SemanticExemptionWitness{}, f.Exemptions...), policyartifact.SemanticExemptionWitness{ID: "policy-exemption/second", Digest: testDigest64})
				return f
			},
		},
		{
			// SI-114/SI-115: no partial equality is favorable — a witness
			// claim set with one MORE identity than the current input's
			// normalized set is a definite mismatch, not a superset pass.
			name: "witness claim set has one extra claim",
			mutate: func(f dispositionFixture) dispositionFixture {
				extra := policyartifact.SemanticClaimWitness{
					ID:              "policy/test-policy#instruction-3",
					Digest:          testDigest64,
					Category:        "policy-instruction",
					AuthorityDigest: testDigest64,
					Scope:           universalScope(),
					Values:          []string{},
				}
				f.Claims = append(append([]policyartifact.SemanticClaimWitness{}, f.Claims...), extra)
				return f
			},
		},
		{
			// The reverse cardinality mismatch: one FEWER witness claim
			// than the current input's normalized set is equally a
			// definite mismatch, never a favorable subset pass.
			name: "witness claim set is missing a claim",
			mutate: func(f dispositionFixture) dispositionFixture {
				f.Claims = append([]policyartifact.SemanticClaimWitness{}, f.Claims[:1]...)
				return f
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := dispositionFixture{
				Name:       kebab("mismatch-" + tc.name),
				Scope:      universalScope(),
				InputID:    digest,
				Target:     testDigest64B,
				Claims:     witnessClaimsFrom(si.Claims),
				Exemptions: si.Exemptions,
				Conclusion: policyartifact.DispositionConflict,
				Origin:     policyartifact.DispositionJudgeResult,
				Approvals:  defaultApprovals(t),
			}
			d := buildDisposition(t, tc.mutate(base))

			in := AuthorityInput{
				EvaluatedOn:  "2026-01-01",
				TargetDigest: testDigest64B,
				Profile:      referenceProfile(t),
				Actors:       []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
				Dispositions: []policyartifact.Disposition{d},
			}
			got, err := ResolveDispositionAuthority(in, si, nil, nil)
			if err != nil {
				t.Fatalf("ResolveDispositionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			r := got[0].Resolution
			if r.Match != ProofViolatedWithWitness || r.Freshness != ProofViolatedWithWitness || r.Scope != ProofViolatedWithWitness || r.Bound != ProofViolatedWithWitness {
				t.Fatalf("resolution = %+v, want match/freshness/scope/bound all violated-with-witness", r)
			}
			if got[0].Conclusion != policyartifact.DispositionConflict {
				t.Fatalf("Conclusion = %q, want conflict (verbatim passthrough even when stale)", got[0].Conclusion)
			}
		})
	}
}

func TestResolveDispositionAuthorityHumanFallbackBound(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}

	tests := []struct {
		name        string
		evaluatedOn string
		expiry      string
		review      string
		want        ProofState
	}{
		{"expiry after evaluated_on is proven", "2026-01-01", "2026-12-31", "", ProofProven},
		{"expiry equal to evaluated_on is proven", "2026-01-01", "2026-01-01", "", ProofProven},
		{"expiry before evaluated_on is violated", "2026-06-01", "2026-01-01", "", ProofViolatedWithWitness},
		{"review-condition-only is unproven", "2026-01-01", "", "quarterly-review", ProofUnproven},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := dispositionFixture{
				Name:       kebab("fallback-" + tc.name),
				Scope:      universalScope(),
				InputID:    digest,
				Target:     testDigest64B,
				Claims:     witnessClaimsFrom(si.Claims),
				Exemptions: si.Exemptions,
				Conclusion: policyartifact.DispositionNoConflict,
				Origin:     policyartifact.DispositionHumanFallback,
				Approvals:  defaultApprovals(t),
				Expiry:     tc.expiry,
				Review:     tc.review,
			}
			d := buildDisposition(t, f)
			in := AuthorityInput{
				EvaluatedOn:  tc.evaluatedOn,
				TargetDigest: testDigest64B,
				Profile:      referenceProfile(t),
				Dispositions: []policyartifact.Disposition{d},
			}
			got, err := ResolveDispositionAuthority(in, si, nil, nil)
			if err != nil {
				t.Fatalf("ResolveDispositionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			if got[0].Resolution.Bound != tc.want {
				t.Fatalf("Bound = %q, want %q", got[0].Resolution.Bound, tc.want)
			}
			// Human-fallback Match/Freshness/Scope are unaffected by Bound —
			// this fixture is otherwise exact-current.
			if got[0].Resolution.Match != ProofProven {
				t.Fatalf("Match = %q, want proven", got[0].Resolution.Match)
			}
		})
	}
}

func TestResolveDispositionAuthorityJudgeResultBoundIgnoresJudgeAvailability(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "no-live-judge", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))

	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{d},
	}
	// primary and challenger are both absent (nil) — §8: currency depends
	// only on the complete semantic-input witness, never on a live judge
	// being available this run.
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 1 || got[0].Resolution.Bound != ProofProven {
		t.Fatalf("resolutions = %+v, want one proven-bound resolution despite no live judge exchange", got)
	}
}

func TestResolveDispositionAuthorityJudgmentProvenanceNeverConsulted(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	// The fixture's Judgment.primary_digest (set by buildDisposition to
	// testDigest64) corresponds to no live exchange or cache entry at all —
	// proving §8's "cache presence is never required to load or validate a
	// disposition" and that this package never reads the citation.
	d := matchingDispositionFixture(t, "cache-absent", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))

	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{d},
	}
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 1 || got[0].Resolution.Match != ProofProven {
		t.Fatalf("resolutions = %+v, want one proven-match resolution unaffected by an uncorrelated judgment citation", got)
	}
}

func TestResolveDispositionAuthorityCrossSnapshotExchangeMismatch(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "cross-snapshot", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))
	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{d},
	}

	mismatched := &ValidatedExchange{RecordDigest: testDigest64B}

	t.Run("primary", func(t *testing.T) {
		_, err := ResolveDispositionAuthority(in, si, mismatched, nil)
		if err == nil {
			t.Fatal("ResolveDispositionAuthority: want operational error when primary was validated against a different semantic input, got nil")
		}
	})
	t.Run("challenger", func(t *testing.T) {
		_, err := ResolveDispositionAuthority(in, si, nil, mismatched)
		if err == nil {
			t.Fatal("ResolveDispositionAuthority: want operational error when challenger was validated against a different semantic input, got nil")
		}
	})
}

func TestResolveDispositionAuthorityMalformedEvaluatedOn(t *testing.T) {
	si := authoritySemanticInput(t)
	for _, evaluatedOn := range []string{"", "not-a-date"} {
		t.Run(fmt.Sprintf("evaluated_on=%q", evaluatedOn), func(t *testing.T) {
			in := AuthorityInput{EvaluatedOn: evaluatedOn, Profile: referenceProfile(t)}
			_, err := ResolveDispositionAuthority(in, si, nil, nil)
			if err == nil {
				t.Fatalf("ResolveDispositionAuthority: want operational error for evaluated_on %q, got nil", evaluatedOn)
			}
		})
	}
}

// TestResolveDispositionAuthorityTargetDigestShape proves
// AuthorityInput.TargetDigest is validated fail-closed, matching this
// package's malformed-injected-input convention (validateEvaluatedOn): a
// missing or non-"sha256:<64 hex>" TargetDigest is an operational error —
// caught before any resolution work, never silently compared as a bare
// string and never treated as a favorable or unfavorable Match outcome.
func TestResolveDispositionAuthorityTargetDigestShape(t *testing.T) {
	si := authoritySemanticInput(t)

	for _, tc := range []string{"", "not-a-digest", "sha256:abcd", "sha256:" + strings.Repeat("g", 64)} {
		t.Run(fmt.Sprintf("target_digest=%q", tc), func(t *testing.T) {
			in := AuthorityInput{EvaluatedOn: "2026-01-01", TargetDigest: tc, Profile: referenceProfile(t)}
			_, err := ResolveDispositionAuthority(in, si, nil, nil)
			if err == nil {
				t.Fatalf("ResolveDispositionAuthority: want operational error for target_digest %q, got nil", tc)
			}
		})
	}

	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "target-digest-well-formed", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))
	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{d},
	}
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 1 || got[0].Resolution.Match != ProofProven {
		t.Fatalf("resolutions = %+v, want one proven-match resolution for a well-formed target digest", got)
	}
}

func TestResolveDispositionAuthorityMalformedSemanticInput(t *testing.T) {
	in := AuthorityInput{EvaluatedOn: "2026-01-01", Profile: referenceProfile(t)}
	_, err := ResolveDispositionAuthority(in, SemanticInput{}, nil, nil)
	if err == nil {
		t.Fatal("ResolveDispositionAuthority: want operational error for a hand-built zero-value semantic input, got nil")
	}
}

func TestResolveDispositionAuthorityDuplicateDispositionID(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	f := dispositionFixture{
		Name: "dup", Scope: universalScope(), InputID: digest, Target: testDigest64B,
		Claims: witnessClaimsFrom(si.Claims), Exemptions: si.Exemptions,
		Conclusion: policyartifact.DispositionNoConflict, Origin: policyartifact.DispositionJudgeResult, Approvals: defaultApprovals(t),
	}
	d1 := buildDisposition(t, f)
	f.Conclusion = policyartifact.DispositionConflict
	d2 := buildDisposition(t, f)

	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{d1, d2},
	}
	_, err = ResolveDispositionAuthority(in, si, nil, nil)
	if err == nil {
		t.Fatal("ResolveDispositionAuthority: want operational error for two different disposition artifacts sharing one id, got nil")
	}
}

func TestResolveDispositionAuthoritySortedByID(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	dZ := matchingDispositionFixture(t, "zzz", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))
	dA := matchingDispositionFixture(t, "aaa", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))

	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Dispositions: []policyartifact.Disposition{dZ, dA},
	}
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 2 || got[0].ID >= got[1].ID {
		t.Fatalf("resolutions = %+v, want two entries sorted ascending by id", got)
	}
}

// =============================================================================
// Shared kernel-authorization table (design §9; ledger SI-113) — exercised
// through BOTH public entry points, since Authorization is derived
// identically in each arm.
// =============================================================================

type authorizationCase struct {
	name      string
	profile   []byte
	approvals []policyartifact.Approval
	actors    []governanceprincipal.PrincipalResolution
	want      ProofState
}

func authorizationCases(t *testing.T) []authorizationCase {
	t.Helper()
	alice := authorityPrincipalID(t, "alice")
	bob := authorityPrincipalID(t, "bob")
	dave := authorityPrincipalID(t, "dave")
	erin := authorityPrincipalID(t, "erin")

	return []authorizationCase{
		{
			name:    "authorized with distinct approvers",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "author", Principal: alice}, {Role: "policy-owner", Principal: bob},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", authenticatedFact("alice")),
				authorityResolve(t, "bob", authenticatedFact("bob")),
			},
			want: ProofProven,
		},
		{
			name:    "solo permitted collapse is authorized",
			profile: authorityProfileYAML("solo"),
			approvals: []policyartifact.Approval{
				{Role: "author", Principal: alice}, {Role: "policy-owner", Principal: alice},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", authenticatedFact("alice")),
			},
			want: ProofProven,
		},
		{
			name:    "team distinctness violated by collapse",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "author", Principal: alice}, {Role: "policy-owner", Principal: alice},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", authenticatedFact("alice")),
			},
			want: ProofViolatedWithWitness,
		},
		{
			name:    "high-assurance distinctness violated by collapse",
			profile: authorityProfileYAML("high-assurance"),
			approvals: []policyartifact.Approval{
				{Role: "author", Principal: alice}, {Role: "policy-owner", Principal: alice},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", authenticatedFact("alice")),
			},
			want: ProofViolatedWithWitness,
		},
		{
			name:    "violated actor",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "policy-owner", Principal: alice},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", violatedFact()),
			},
			want: ProofViolatedWithWitness,
		},
		{
			name:    "unproven actor",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "policy-owner", Principal: alice},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", unavailableFact()),
			},
			want: ProofUnproven,
		},
		{
			name:    "unknown role",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "policy-owner", Principal: dave},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "dave", authenticatedFact("dave")),
			},
			want: ProofViolatedWithWitness,
		},
		{
			name:    "unknown principal (no resolution supplied)",
			profile: authorityProfileYAML("team"),
			approvals: []policyartifact.Approval{
				{Role: "policy-owner", Principal: erin},
			},
			actors: nil,
			want:   ProofUnproven,
		},
		{
			name:    "experimental profile always unproven",
			profile: authorityProfileYAML("experimental"),
			approvals: []policyartifact.Approval{
				{Role: "author", Principal: alice}, {Role: "policy-owner", Principal: bob},
			},
			actors: []governanceprincipal.PrincipalResolution{
				authorityResolve(t, "alice", authenticatedFact("alice")),
				authorityResolve(t, "bob", authenticatedFact("bob")),
			},
			want: ProofUnproven,
		},
	}
}

func TestResolveExemptionAuthorityAuthorization(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))

	for _, tc := range authorizationCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			row := authorityRow("row-1", []TypedClaimRecord{claim}, dims)
			ex := buildExemption(t, exemptionFixture{
				Name:      kebab("auth-" + tc.name),
				Scope:     universalScope(),
				Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
				Approvals: tc.approvals,
			})
			in := AuthorityInput{
				EvaluatedOn: "2026-01-01",
				Profile:     decodeAuthorityProfile(t, tc.profile),
				Actors:      tc.actors,
				Exemptions:  []policyartifact.Exemption{ex},
			}
			got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
			if err != nil {
				t.Fatalf("ResolveExemptionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			if got[0].Resolution.Authorization != tc.want {
				t.Fatalf("Authorization = %q, want %q", got[0].Resolution.Authorization, tc.want)
			}
		})
	}
}

func TestResolveDispositionAuthorityAuthorization(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}

	for _, tc := range authorizationCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			d := matchingDispositionFixture(t, kebab("auth-"+tc.name), policyartifact.DispositionJudgeResult, si, digest, testDigest64B, tc.approvals)
			in := AuthorityInput{
				EvaluatedOn:  "2026-01-01",
				TargetDigest: testDigest64B,
				Profile:      decodeAuthorityProfile(t, tc.profile),
				Actors:       tc.actors,
				Dispositions: []policyartifact.Disposition{d},
			}
			got, err := ResolveDispositionAuthority(in, si, nil, nil)
			if err != nil {
				t.Fatalf("ResolveDispositionAuthority: %v", err)
			}
			if len(got) != 1 {
				t.Fatalf("resolutions = %+v, want exactly one", got)
			}
			if got[0].Resolution.Authorization != tc.want {
				t.Fatalf("Authorization = %q, want %q", got[0].Resolution.Authorization, tc.want)
			}
		})
	}
}

// TestResolveExemptionAuthorityUnlistedTransition proves SI-113/§9's
// "explicit non-authorizing finding, never a favorable default": a profile
// that does not list "policy-exemption-approval" among its applicable
// transitions must map to Authorization = violated-with-witness, not
// unproven and never proven.
func TestResolveExemptionAuthorityUnlistedTransition(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	dims := fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref"))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, dims)
	ex := buildExemption(t, exemptionFixture{
		Name:      "unlisted-transition",
		Scope:     universalScope(),
		Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
		Approvals: defaultApprovals(t),
	})
	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     decodeAuthorityProfile(t, authorityProfileMissingTransitionYAML(transitionExemptionApproval)),
		Actors:      []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
		Exemptions:  []policyartifact.Exemption{ex},
	}
	got, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
	if err != nil {
		t.Fatalf("ResolveExemptionAuthority: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("resolutions = %+v, want exactly one", got)
	}
	if got[0].Resolution.Authorization != ProofViolatedWithWitness {
		t.Fatalf("Authorization = %q, want violated-with-witness (unlisted transition is the kernel's explicit non-authorizing finding)", got[0].Resolution.Authorization)
	}
}

// TestResolveDispositionAuthorityUnlistedTransition is the disposition
// arm's mirror: a profile that does not list "policy-disposition-approval"
// must map to Authorization = violated-with-witness.
func TestResolveDispositionAuthorityUnlistedTransition(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "unlisted-transition", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))
	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      decodeAuthorityProfile(t, authorityProfileMissingTransitionYAML(transitionDispositionApproval)),
		Actors:       []governanceprincipal.PrincipalResolution{authorityResolve(t, "alice", authenticatedFact("alice"))},
		Dispositions: []policyartifact.Disposition{d},
	}
	got, err := ResolveDispositionAuthority(in, si, nil, nil)
	if err != nil {
		t.Fatalf("ResolveDispositionAuthority: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("resolutions = %+v, want exactly one", got)
	}
	if got[0].Resolution.Authorization != ProofViolatedWithWitness {
		t.Fatalf("Authorization = %q, want violated-with-witness (unlisted transition is the kernel's explicit non-authorizing finding)", got[0].Resolution.Authorization)
	}
}

func TestResolveExemptionAuthorityZeroValuePrincipalResolutionRejected(t *testing.T) {
	claim := authorityTypedClaim(t, "policy/policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))
	row := authorityRow("row-1", []TypedClaimRecord{claim}, fourDims(universalDim("phase"), universalDim("environment"), universalDim("path"), universalDim("ref")))
	ex := buildExemption(t, exemptionFixture{
		Name:      "zero-value-actor",
		Scope:     universalScope(),
		Witnesses: []policyartifact.Witness{{Policy: "policy/policy-a", Claim: "c1", ClaimDigest: claim.ClaimDigest}},
		Approvals: defaultApprovals(t),
	})
	in := AuthorityInput{
		EvaluatedOn: "2026-01-01",
		Profile:     referenceProfile(t),
		Actors:      []governanceprincipal.PrincipalResolution{{}}, // zero value: never produced by Resolver.Resolve
		Exemptions:  []policyartifact.Exemption{ex},
	}
	_, err := ResolveExemptionAuthority(context.Background(), in, row, noCallCoverageResolver{t: t})
	if err == nil {
		t.Fatal("ResolveExemptionAuthority: want operational error for a zero-value (unsealed) PrincipalResolution, got nil")
	}
}

func TestResolveDispositionAuthorityZeroValuePrincipalResolutionRejected(t *testing.T) {
	si := authoritySemanticInput(t)
	digest, err := semanticInputDigest(si)
	if err != nil {
		t.Fatalf("semanticInputDigest: %v", err)
	}
	d := matchingDispositionFixture(t, "zero-value-actor", policyartifact.DispositionJudgeResult, si, digest, testDigest64B, defaultApprovals(t))
	in := AuthorityInput{
		EvaluatedOn:  "2026-01-01",
		TargetDigest: testDigest64B,
		Profile:      referenceProfile(t),
		Actors:       []governanceprincipal.PrincipalResolution{{}},
		Dispositions: []policyartifact.Disposition{d},
	}
	_, err = ResolveDispositionAuthority(in, si, nil, nil)
	if err == nil {
		t.Fatal("ResolveDispositionAuthority: want operational error for a zero-value (unsealed) PrincipalResolution, got nil")
	}
}
