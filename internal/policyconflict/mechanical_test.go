package policyconflict

import (
	"context"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/contextcompile"
	"github.com/jyang234/verdi/internal/governanceprincipal"
	"github.com/jyang234/verdi/internal/policyartifact"
)

// --- shared test fixtures (used by exemption_test.go too) ------------------

func phaseScope(phase string) policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{phase}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
}

func refScope(ref string) policyartifact.Scope {
	return policyartifact.Scope{Phases: []string{}, Environments: []string{}, Paths: []string{}, Refs: []string{ref}}
}

const testDigest64 = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// typedClaim builds a sealed-shape contextcompile.TypedClaim: the real
// canonical claim digest so distinct claims never collide, and a fixed
// well-formed policy digest.
func typedClaim(t *testing.T, policyID string, claim policyartifact.Claim) contextcompile.TypedClaim {
	t.Helper()
	digest, err := policyartifact.ClaimDigest(claim)
	if err != nil {
		t.Fatalf("ClaimDigest: %v", err)
	}
	return contextcompile.TypedClaim{PolicyID: policyID, PolicyDigest: testDigest64, ClaimDigest: digest, Claim: claim}
}

func discreteClaim(id, subject string, op policyartifact.Operator, values []string, scope policyartifact.Scope) policyartifact.Claim {
	return policyartifact.Claim{
		ID: id, Family: policyartifact.FamilyConfiguration, Operator: op, Subject: subject,
		Values: values, Scope: scope, Overridable: true,
	}
}

func intervalClaim(id, subject string, op policyartifact.Operator, bound int, scope policyartifact.Scope) policyartifact.Claim {
	b := bound
	return policyartifact.Claim{
		ID: id, Family: policyartifact.FamilyResource, Operator: op, Subject: subject,
		Values: []string{}, Bound: &b, Scope: scope, Overridable: true,
	}
}

func pathClaim(id, subject string, op policyartifact.Operator, values []string, scope policyartifact.Scope) policyartifact.Claim {
	return policyartifact.Claim{
		ID: id, Family: policyartifact.FamilyResource, Operator: op, Subject: subject,
		Values: values, Scope: scope, Overridable: true,
	}
}

func principalClaim(id, transition string, op policyartifact.Operator, roleA, roleB string, scope policyartifact.Scope) policyartifact.Claim {
	return policyartifact.Claim{
		ID: id, Family: policyartifact.FamilyIdentity, Operator: op, Subject: transition,
		Values: []string{roleA, roleB}, Scope: scope, Overridable: true,
	}
}

// --- governanceprincipal fixtures -------------------------------------------

func testCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:       []string{"author", "reviewer"},
		Transitions: []string{"release", "publish"},
	}
}

const rolePolicyYAML = `schema: verdi.governance-profile/v1
id: team-default
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-b", "user-c"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`

func mustDecodeProfile(t *testing.T, raw string) governanceprincipal.Profile {
	t.Helper()
	p, err := governanceprincipal.DecodeProfile([]byte(raw), testCatalog())
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return p
}

type readerFunc func(ctx context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error)

func (f readerFunc) ReadTrustFact(ctx context.Context, source governanceprincipal.TrustSource, claim governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
	return f(ctx, source, claim)
}

func staticFact(fact governanceprincipal.TrustFact) governanceprincipal.TrustFactReader {
	return readerFunc(func(context.Context, governanceprincipal.TrustSource, governanceprincipal.PrincipalClaim) (governanceprincipal.TrustFact, error) {
		return fact, nil
	})
}

func githubFact(subjects ...string) governanceprincipal.TrustFact {
	return governanceprincipal.TrustFact{
		SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
		Subjects: subjects, EvidenceDigest: testDigest64, Available: true, Valid: true,
	}
}

func authenticatedActor(t *testing.T, profile governanceprincipal.Profile, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	r := governanceprincipal.NewResolver(staticFact(githubFact(subject)))
	res, err := r.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.State != governanceprincipal.ResolutionAuthenticated {
		t.Fatalf("resolution state = %q, want authenticated", res.State)
	}
	return res
}

func unprovenActor(t *testing.T, profile governanceprincipal.Profile, subject string) governanceprincipal.PrincipalResolution {
	t.Helper()
	r := governanceprincipal.NewResolver(staticFact(governanceprincipal.TrustFact{
		SourceID: "github", SourceKind: governanceprincipal.TrustSourceForge,
		Available: false, Valid: false, Reason: "forge api unreachable",
	}))
	res, err := r.Resolve(context.Background(), profile, governanceprincipal.PrincipalClaim{TrustSource: "github", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.State != governanceprincipal.ResolutionUnproven {
		t.Fatalf("resolution state = %q, want unproven", res.State)
	}
	return res
}

// --- solveDiscrete -----------------------------------------------------------

func TestSolveDiscrete(t *testing.T) {
	s := universalScope()
	tests := []struct {
		name    string
		claims  []policyartifact.Claim
		want    SolverState
		openDom bool
	}{
		{
			name: "equals agreement satisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpEquals, []string{"gold"}, s),
			},
			want: SolverSatisfiable,
		},
		{
			name: "equals disagreement unsatisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "allowed-set intersection satisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, s),
				discreteClaim("c2", "level", policyartifact.OpAllowedValues, []string{"silver", "bronze"}, s),
			},
			want: SolverSatisfiable,
		},
		{
			name: "allowed-set intersection empty unsatisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpAllowedValues, []string{"silver"}, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "required/forbidden union unsatisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpRequiredValues, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpForbiddenValues, []string{"gold"}, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "required/forbidden union satisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpRequiredValues, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpForbiddenValues, []string{"bronze"}, s),
			},
			want: SolverSatisfiable,
		},
		{
			name: "not-equals membership exclusion unsatisfiable via equals",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpNotEquals, []string{"gold"}, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "not-equals excludes required unsatisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpRequiredValues, []string{"gold"}, s),
				discreteClaim("c2", "level", policyartifact.OpNotEquals, []string{"gold"}, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "finite allowed witness satisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, s),
			},
			want: SolverSatisfiable,
		},
		{
			name: "open-domain witness satisfiable",
			claims: []policyartifact.Claim{
				discreteClaim("c1", "level", policyartifact.OpForbiddenValues, []string{"bronze"}, s),
			},
			want:    SolverSatisfiable,
			openDom: true,
		},
		{
			name:   "no claims trivially satisfiable open-domain",
			claims: []policyartifact.Claim{},
			want:   SolverSatisfiable, openDom: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := solveDiscrete(tt.claims)
			if got.State != tt.want {
				t.Fatalf("State = %q, want %q (proof: %+v)", got.State, tt.want, got)
			}
			if got.Domain != domainDiscreteSet {
				t.Fatalf("Domain = %q, want %q", got.Domain, domainDiscreteSet)
			}
			if got.OpenDomain != tt.openDom {
				t.Fatalf("OpenDomain = %v, want %v", got.OpenDomain, tt.openDom)
			}
		})
	}
}

// --- solveInterval -----------------------------------------------------------

func TestSolveInterval(t *testing.T) {
	s := universalScope()
	tests := []struct {
		name   string
		claims []policyartifact.Claim
		want   SolverState
	}{
		{
			name: "multiple minimums reduce to max satisfiable",
			claims: []policyartifact.Claim{
				intervalClaim("c1", "replicas", policyartifact.OpMinimum, 2, s),
				intervalClaim("c2", "replicas", policyartifact.OpMinimum, 5, s),
				intervalClaim("c3", "replicas", policyartifact.OpMaximum, 10, s),
			},
			want: SolverSatisfiable,
		},
		{
			name: "multiple maximums reduce to min unsatisfiable",
			claims: []policyartifact.Claim{
				intervalClaim("c1", "replicas", policyartifact.OpMinimum, 8, s),
				intervalClaim("c2", "replicas", policyartifact.OpMaximum, 6, s),
				intervalClaim("c3", "replicas", policyartifact.OpMaximum, 10, s),
			},
			want: SolverUnsatisfiable,
		},
		{
			name: "equality satisfiable",
			claims: []policyartifact.Claim{
				intervalClaim("c1", "replicas", policyartifact.OpMinimum, 5, s),
				intervalClaim("c2", "replicas", policyartifact.OpMaximum, 5, s),
			},
			want: SolverSatisfiable,
		},
		{
			name:   "no claims trivially satisfiable",
			claims: []policyartifact.Claim{},
			want:   SolverSatisfiable,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := solveInterval(tt.claims)
			if err != nil {
				t.Fatalf("solveInterval: %v", err)
			}
			if got.State != tt.want {
				t.Fatalf("State = %q, want %q (proof: %+v)", got.State, tt.want, got)
			}
			if got.Domain != domainIntegerInterval {
				t.Fatalf("Domain = %q, want %q", got.Domain, domainIntegerInterval)
			}
		})
	}
}

// TestSolveIntervalUnsatWitnessesAreSortedUnique pins the wire's own
// ordering rule (validate.go's requireSortedUniqueStrings over a solver
// proof's witness set) for the one interval case whose numeric order
// disagrees with its lexical order: minimum 8 with maximum 6.
func TestSolveIntervalUnsatWitnessesAreSortedUnique(t *testing.T) {
	s := universalScope()
	got, err := solveInterval([]policyartifact.Claim{
		intervalClaim("c1", "replicas", policyartifact.OpMinimum, 8, s),
		intervalClaim("c2", "replicas", policyartifact.OpMaximum, 6, s),
	})
	if err != nil {
		t.Fatalf("solveInterval: %v", err)
	}
	if got.State != SolverUnsatisfiable {
		t.Fatalf("State = %q, want unsatisfiable", got.State)
	}
	if err := validateSolverProof("proof", got); err != nil {
		t.Fatalf("unsatisfiable interval proof failed the package's own wire validation: %v (proof: %+v)", err, got)
	}
}

func TestSolveIntervalMalformedBound(t *testing.T) {
	claim := intervalClaim("c1", "replicas", policyartifact.OpMinimum, 1, universalScope())
	claim.Bound = nil
	if _, err := solveInterval([]policyartifact.Claim{claim}); err == nil {
		t.Fatal("solveInterval: want error for malformed bound, got nil")
	}
}

// --- solvePathCapability ------------------------------------------------------

func TestSolvePathCapability(t *testing.T) {
	s := universalScope()
	claims := []policyartifact.Claim{
		pathClaim("c1", "workspace", policyartifact.OpPathRead, []string{"internal/"}, s),
		pathClaim("c2", "workspace", policyartifact.OpPathWrite, []string{"internal/"}, s),
	}
	got := solvePathCapability(claims)
	if got.State != SolverSatisfiable {
		t.Fatalf("read/write coexist: State = %q, want satisfiable (DC-5: absent execution grant is not a conflict)", got.State)
	}
	if got.Domain != domainPathCapability {
		t.Fatalf("Domain = %q, want %q", got.Domain, domainPathCapability)
	}
	if len(got.Values) != 1 || got.Values[0] != "internal/" {
		t.Fatalf("Values = %v, want canonical union [internal/]", got.Values)
	}
}

// TestSolvePathCapabilityRecordsPerKindRequirements pins authority design
// §5.4's "canonical requirement set" per capability KIND: read and write
// requirements union within their own kind, never into one anonymous set
// that loses which access each path value was required for.
func TestSolvePathCapabilityRecordsPerKindRequirements(t *testing.T) {
	s := universalScope()
	got := solvePathCapability([]policyartifact.Claim{
		pathClaim("c1", "workspace", policyartifact.OpPathRead, []string{"internal/"}, s),
		pathClaim("c2", "workspace", policyartifact.OpPathWrite, []string{"cmd/"}, s),
	})
	if got.State != SolverSatisfiable {
		t.Fatalf("State = %q, want satisfiable", got.State)
	}
	wantValues := []string{"cmd/", "internal/"}
	if !reflect.DeepEqual(got.Values, wantValues) {
		t.Fatalf("Values = %v, want the canonical union %v", got.Values, wantValues)
	}
	wantWitnesses := []string{"path-read:internal/", "path-write:cmd/"}
	if !reflect.DeepEqual(got.Witnesses, wantWitnesses) {
		t.Fatalf("Witnesses = %v, want per-kind canonical requirement sets %v", got.Witnesses, wantWitnesses)
	}
	if err := validateSolverProof("proof", got); err != nil {
		t.Fatalf("path-capability proof failed the package's own wire validation: %v", err)
	}
}

// --- solvePrincipalRelation ---------------------------------------------------

func TestSolvePrincipalRelationSameDifferentContradiction(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []policyartifact.Claim{
		principalClaim("c1", "release", policyartifact.OpSamePrincipal, "author", "reviewer", universalScope()),
		principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope()),
	}
	got, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, nil)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverUnsatisfiable {
		t.Fatalf("same+different for one transition/role pair: State = %q, want unsatisfiable", got.State)
	}
}

func TestSolvePrincipalRelationCanonicalReversedRolePair(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	author := authenticatedActor(t, profile, "user-a")
	reviewer := authenticatedActor(t, profile, "user-b")
	actors := []governanceprincipal.PrincipalResolution{author, reviewer}

	forward := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}
	reversed := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope())}

	got1, err := solvePrincipalRelation("release", "author", "reviewer", forward, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation forward: %v", err)
	}
	got2, err := solvePrincipalRelation("release", "author", "reviewer", reversed, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation reversed: %v", err)
	}
	if got1.State != got2.State {
		t.Fatalf("reversed role spelling must resolve identically: forward=%q reversed=%q", got1.State, got2.State)
	}
	if got1.State != SolverSatisfiable {
		t.Fatalf("distinct authenticated author/reviewer: State = %q, want satisfiable", got1.State)
	}
}

func TestSolvePrincipalRelationKernelProvenViolatedUnproven(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}

	t.Run("proven: distinct principals", func(t *testing.T) {
		actors := []governanceprincipal.PrincipalResolution{authenticatedActor(t, profile, "user-a"), authenticatedActor(t, profile, "user-b")}
		got, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverSatisfiable {
			t.Fatalf("State = %q, want satisfiable", got.State)
		}
	})

	t.Run("violated: same principal fills both roles", func(t *testing.T) {
		// user-a authenticates as author only; extend the profile so the
		// same subject also holds reviewer, producing one shared filler.
		collapsed := mustDecodeProfile(t, `schema: verdi.governance-profile/v1
id: team-default
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-a", "user-c"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`)
		actors := []governanceprincipal.PrincipalResolution{authenticatedActor(t, collapsed, "user-a")}
		got, err := solvePrincipalRelation("release", "author", "reviewer", claims, collapsed, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverUnsatisfiable {
			t.Fatalf("State = %q, want unsatisfiable (kernel violated)", got.State)
		}
	})

	t.Run("unproven: no authenticated fillers", func(t *testing.T) {
		actors := []governanceprincipal.PrincipalResolution{unprovenActor(t, profile, "user-a")}
		got, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverUnproven {
			t.Fatalf("State = %q, want unproven", got.State)
		}
	})
}

// experimentalPolicyYAML is rolePolicyYAML's identical role/distinctness
// content under the experimental class: the kernel forces the effective
// posture to advisory and returns a violated experimental-authority-
// forbidden finding for the authoritative request this package makes.
const experimentalPolicyYAML = `schema: verdi.governance-profile/v1
id: team-default
class: experimental
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-b", "user-c"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`

// TestSolvePrincipalRelationExperimentalProfileIsNeverProven pins authority
// design §5.3's literal kernel mapping: "violated and unproven kernel
// results remain violated-with-witness or unproven respectively". Two
// distinct authenticated actors satisfy the distinctness rule itself, but
// the kernel decision as a whole is violated (an experimental profile can
// never produce an authoritative authorization), so the relation is NOT
// proven.
func TestSolvePrincipalRelationExperimentalProfileIsNeverProven(t *testing.T) {
	profile := mustDecodeProfile(t, experimentalPolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}

	got, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverUnsatisfiable {
		t.Fatalf("State = %q, want unsatisfiable: the kernel decision is violated-with-witness, never a proof (proof: %+v)", got.State, got)
	}
	if !stringsContain(got.Witnesses, governanceprincipal.ReasonExperimentalAuthorityForbidden) {
		t.Fatalf("Witnesses = %v, want the kernel's own violated finding code carried as a witness", got.Witnesses)
	}
	if err := validateSolverProof("proof", got); err != nil {
		t.Fatalf("proof failed the package's own wire validation: %v", err)
	}
}

func TestEvaluateMechanicalPrincipalRelationExperimentalRowViolated(t *testing.T) {
	profile := mustDecodeProfile(t, experimentalPolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	if rows[0].State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want violated-with-witness (kernel result is violated)", rows[0].State)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed validation: %v", err)
	}
}

func TestSolvePrincipalRelationNoMatchingDistinctnessRuleUnproven(t *testing.T) {
	profile := mustDecodeProfile(t, `schema: verdi.governance-profile/v1
id: team-default
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-b"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`)
	actors := []governanceprincipal.PrincipalResolution{authenticatedActor(t, profile, "user-a"), authenticatedActor(t, profile, "user-b")}
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}
	got, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverUnproven {
		t.Fatalf("no kernel-registered separation rule: State = %q, want unproven (never a false pass)", got.State)
	}
}

func TestSolvePrincipalRelationUnregisteredTransitionRejected(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []policyartifact.Claim{principalClaim("c1", "unregistered-transition", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}
	if _, err := solvePrincipalRelation("unregistered-transition", "author", "reviewer", claims, profile, nil); err == nil {
		t.Fatal("want operational error for unregistered transition, got nil")
	}
}

func TestSolvePrincipalRelationUnregisteredRoleRejected(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "ghost-role", universalScope())}
	if _, err := solvePrincipalRelation("release", "author", "ghost-role", claims, profile, nil); err == nil {
		t.Fatal("want operational error for unregistered role, got nil")
	}
}

func TestSolvePrincipalRelationNoRelationTriviallySatisfiable(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	got, err := solvePrincipalRelation("release", "author", "reviewer", nil, profile, nil)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverSatisfiable {
		t.Fatalf("State = %q, want satisfiable", got.State)
	}
}

// --- EvaluateMechanical: grouping, mixed-domain rejection ---------------------

func TestEvaluateMechanicalMixedDomainRejected(t *testing.T) {
	// Both claims share (family=resource, subject=workspace) but mix the
	// discrete-set and path-capability domains: invalid authority.
	discreteInResource := policyartifact.Claim{
		ID: "c1", Family: policyartifact.FamilyResource, Operator: policyartifact.OpAllowedValues,
		Subject: "workspace", Values: []string{"internal/"}, Scope: universalScope(), Overridable: true,
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteInResource),
		typedClaim(t, "policy-a", pathClaim("c2", "workspace", policyartifact.OpPathRead, []string{"internal/"}, universalScope())),
	}
	in := MechanicalInput{Claims: claims}
	if _, err := EvaluateMechanical(context.Background(), in); err == nil {
		t.Fatal("want operational error for mixed-domain group, got nil")
	}
}

func TestEvaluateMechanicalEmptyClaims(t *testing.T) {
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want none", rows)
	}
}

func TestEvaluateMechanicalSatisfiableRow(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, universalScope())),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpRequiredValues, []string{"gold"}, universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	row := rows[0]
	if err := validateMechanicalEvaluation("row", row); err != nil {
		t.Fatalf("row failed the package's own validation: %v", err)
	}
	if row.State != ProofProven {
		t.Fatalf("State = %q, want proven", row.State)
	}
	if len(row.Reasons) != 1 || row.Reasons[0] != ReasonMechanicalSatisfiable {
		t.Fatalf("Reasons = %v, want [mechanical-satisfiable]", row.Reasons)
	}
}

func TestEvaluateMechanicalExactScopeConflict(t *testing.T) {
	scope := phaseScope("build")
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, scope)),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, scope)),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one exact-scope witness", rows)
	}
	row := rows[0]
	if row.State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want violated-with-witness", row.State)
	}
	if len(row.Reasons) != 1 || row.Reasons[0] != ReasonMechanicalConflict {
		t.Fatalf("Reasons = %v, want [mechanical-conflict]", row.Reasons)
	}
	if row.Scope.State != ScopeOverlap {
		t.Fatalf("Scope.State = %q, want overlap (identical scope proves itself)", row.Scope.State)
	}
	if len(row.Claims) != 2 {
		t.Fatalf("Claims = %+v, want both conflicting claims", row.Claims)
	}
}

func TestEvaluateMechanicalDifferentlyScopedPairConflict(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, universalScope())),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("build"))),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one pair witness", rows)
	}
	if rows[0].State != ProofViolatedWithWitness {
		t.Fatalf("State = %q, want violated-with-witness (universal scope overlaps build)", rows[0].State)
	}
}

func TestEvaluateMechanicalDisjointScopedPairNoConflictRow(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, phaseScope("design"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("build"))),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	// Steps 1-2 produce no overlap witness at all: each singleton exact-
	// scope subgroup is trivially satisfiable and the one differently-scoped
	// pair is PROVEN disjoint. Authority design §5: "Proven-disjoint
	// witnesses do not conflict" — no co-applicable subset can exist, so
	// nothing remains for the higher-order case and the row is proven with
	// scope-disjoint.
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one row", rows)
	}
	if rows[0].State != ProofProven {
		t.Fatalf("State = %q, want proven (proven-disjoint witnesses do not conflict)", rows[0].State)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonScopeDisjoint {
		t.Fatalf("Reasons = %v, want [scope-disjoint]", rows[0].Reasons)
	}
	if len(rows[0].Claims) != 2 {
		t.Fatalf("row Claims = %+v, want the complete group", rows[0].Claims)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed validation: %v", err)
	}
}

// TestEvaluateMechanicalPairwiseOverlappingTripleStaysUnproven is the
// counterweight to the disjoint case above: three claims whose scopes
// overlap PAIRWISE in every pair, each pair mechanically satisfiable, but
// whose three-way conjunction is unsatisfiable. Steps 1-2 therefore produce
// no witness at all, and the complete group's N-way scope intersection is
// empty — yet a co-applicable unsatisfiable subset cannot be ruled out from
// pairwise-disjointness evidence, because there is none. This must stay the
// blocked-unproven higher-order residual and must never become a
// scope-disjoint pass.
func TestEvaluateMechanicalPairwiseOverlappingTripleStaysUnproven(t *testing.T) {
	twoPhases := func(a, b string) policyartifact.Scope {
		return policyartifact.Scope{Phases: []string{a, b}, Environments: []string{}, Paths: []string{}, Refs: []string{}}
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, twoPhases("build", "design"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpAllowedValues, []string{"bronze", "silver"}, twoPhases("build", "review"))),
		typedClaim(t, "policy-c", discreteClaim("c3", "level", policyartifact.OpAllowedValues, []string{"bronze", "gold"}, twoPhases("design", "review"))),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one fallback row", rows)
	}
	if rows[0].State != ProofUnproven {
		t.Fatalf("State = %q, want unproven (no pairwise disjointness evidence rules the triple out)", rows[0].State)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonHigherOrderScopeUnproven {
		t.Fatalf("Reasons = %v, want [higher-order-scope-unproven]", rows[0].Reasons)
	}
	if len(rows[0].Claims) != 3 {
		t.Fatalf("fallback row Claims = %+v, want the complete group", rows[0].Claims)
	}
}

// TestEvaluateMechanicalIdenticalClaimsFromTwoPoliciesCollapse pins the
// frozen wire's row-claim identity rule (validate.go's
// requireSortedUnique over ClaimDigest): a claim two policies declare
// byte-identically has ONE row identity, so the row carries one record for
// it, deterministically attributed.
func TestEvaluateMechanicalIdenticalClaimsFromTwoPoliciesCollapse(t *testing.T) {
	shared := discreteClaim("shared-claim", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-b", shared),
		typedClaim(t, "policy-a", shared),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed the package's own wire validation: %v", err)
	}
	if len(rows[0].Claims) != 1 {
		t.Fatalf("Claims = %+v, want one record for the single shared claim identity", rows[0].Claims)
	}
	if rows[0].Claims[0].PolicyID != "policy-a" {
		t.Fatalf("Claims[0].PolicyID = %q, want the deterministic lowest contributing policy id", rows[0].Claims[0].PolicyID)
	}
}

func TestEvaluateMechanicalUnknownRefHigherOrderUnproven(t *testing.T) {
	resolver := &mapRefResolver{states: map[[2]string]ScopeState{{"spec/a", "spec/b"}: ScopeUnknown}}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, refScope("spec/a"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, refScope("spec/b"))),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Refs: resolver})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofUnproven {
		t.Fatalf("rows = %+v, want exactly one unproven fallback row", rows)
	}
}

func TestEvaluateMechanicalDeterministicOrdering(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-b", discreteClaim("c2", "zeta", policyartifact.OpAllowedValues, []string{"x"}, universalScope())),
		typedClaim(t, "policy-a", discreteClaim("c1", "alpha", policyartifact.OpAllowedValues, []string{"x"}, universalScope())),
	}
	in := MechanicalInput{Claims: claims}
	rows1, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows2, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows1) != 2 {
		t.Fatalf("rows1 = %+v, want two groups", rows1)
	}
	for i := range rows1 {
		if rows1[i].ID != rows2[i].ID {
			t.Fatalf("non-deterministic row order: run1[%d]=%q run2[%d]=%q", i, rows1[i].ID, i, rows2[i].ID)
		}
	}
	if rows1[0].ID >= rows1[1].ID {
		t.Fatalf("rows not sorted ascending by ID: %q then %q", rows1[0].ID, rows1[1].ID)
	}
}

func TestEvaluateMechanicalIntervalConflict(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", intervalClaim("c1", "replicas", policyartifact.OpMinimum, 10, universalScope())),
		typedClaim(t, "policy-b", intervalClaim("c2", "replicas", policyartifact.OpMaximum, 2, universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofViolatedWithWitness {
		t.Fatalf("rows = %+v, want one violated row", rows)
	}
}

// TestEvaluateMechanicalIntervalConflictRowValidates covers the interval
// witness pair whose lexical order disagrees with its numeric order
// (minimum 8, maximum 6): the emitted row must satisfy the frozen wire.
func TestEvaluateMechanicalIntervalConflictRowValidates(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", intervalClaim("c1", "replicas", policyartifact.OpMinimum, 8, universalScope())),
		typedClaim(t, "policy-b", intervalClaim("c2", "replicas", policyartifact.OpMaximum, 6, universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofViolatedWithWitness {
		t.Fatalf("rows = %+v, want one violated row", rows)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed the package's own wire validation: %v", err)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonMechanicalConflict {
		t.Fatalf("Reasons = %v, want [mechanical-conflict]", rows[0].Reasons)
	}
}

func TestEvaluateMechanicalPathCapabilityNeverConflicts(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", pathClaim("c1", "workspace", policyartifact.OpPathRead, []string{"internal/"}, universalScope())),
		typedClaim(t, "policy-b", pathClaim("c2", "workspace", policyartifact.OpPathWrite, []string{"internal/"}, universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofProven {
		t.Fatalf("rows = %+v, want one proven row (path domain never manufactures conflict)", rows)
	}
}

func TestEvaluateMechanicalPrincipalRelationRow(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{authenticatedActor(t, profile, "user-a"), authenticatedActor(t, profile, "user-b")}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofProven {
		t.Fatalf("rows = %+v, want one proven row", rows)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed validation: %v", err)
	}
}

func TestEvaluateMechanicalPrincipalRelationUnprovenRow(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(rows) != 1 || rows[0].State != ProofUnproven {
		t.Fatalf("rows = %+v, want one unproven row (no actors)", rows)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonPrincipalRelationUnproven {
		t.Fatalf("Reasons = %v, want [principal-relation-unproven]", rows[0].Reasons)
	}
}

// TestEvaluateMechanicalPrincipalRelationReasonsAreDomainDerived pins
// ledger SI-103's "one stable consumer label per outcome" across the
// identity domain's two distinct violated outcomes: the kernel-derived
// relation violation carries principal-relation-violated, while the
// kernel-free same+different textual contradiction is exactly what
// authority design §5.3 calls "a mechanical conflict".
func TestEvaluateMechanicalPrincipalRelationReasonsAreDomainDerived(t *testing.T) {
	t.Run("kernel-derived violation", func(t *testing.T) {
		collapsed := mustDecodeProfile(t, `schema: verdi.governance-profile/v1
id: team-default
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-a", "user-c"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`)
		actors := []governanceprincipal.PrincipalResolution{authenticatedActor(t, collapsed, "user-a")}
		claims := []contextcompile.TypedClaim{
			typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
		}
		rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: collapsed, Actors: actors})
		if err != nil {
			t.Fatalf("EvaluateMechanical: %v", err)
		}
		if len(rows) != 1 || rows[0].State != ProofViolatedWithWitness {
			t.Fatalf("rows = %+v, want one violated row", rows)
		}
		if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonPrincipalRelationViolated {
			t.Fatalf("Reasons = %v, want [principal-relation-violated]", rows[0].Reasons)
		}
		if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
			t.Fatalf("row failed validation: %v", err)
		}
	})

	t.Run("same+different textual contradiction", func(t *testing.T) {
		profile := mustDecodeProfile(t, rolePolicyYAML)
		claims := []contextcompile.TypedClaim{
			typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpSamePrincipal, "author", "reviewer", universalScope())),
			typedClaim(t, "policy-b", principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "reviewer", "author", universalScope())),
		}
		rows, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile})
		if err != nil {
			t.Fatalf("EvaluateMechanical: %v", err)
		}
		if len(rows) != 1 || rows[0].State != ProofViolatedWithWitness {
			t.Fatalf("rows = %+v, want one violated row", rows)
		}
		if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonMechanicalConflict {
			t.Fatalf("Reasons = %v, want [mechanical-conflict]", rows[0].Reasons)
		}
	})
}

// mapRefResolver is the hermetic fake RefRelationResolver used across
// mechanical/exemption tests.
type mapRefResolver struct {
	states map[[2]string]ScopeState
	calls  int
}

func (m *mapRefResolver) Relate(ctx context.Context, a, b string) (ScopeState, []string, error) {
	m.calls++
	if s, ok := m.states[[2]string{a, b}]; ok {
		return s, []string{a + "->" + b}, nil
	}
	return ScopeDisjoint, nil, nil
}
