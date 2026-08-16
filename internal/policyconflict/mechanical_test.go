package policyconflict

import (
	"context"
	"fmt"
	"reflect"
	"sort"
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

// mustPrincipalID derives the canonical kernel principal id for a github
// subject — the exact left component of an SI-108 witness token.
func mustPrincipalID(t *testing.T, subject string) string {
	t.Helper()
	id, err := governanceprincipal.CanonicalPrincipalID("github", subject)
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	return string(id)
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
	got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, nil)
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

	got1, _, err := solvePrincipalRelation("release", "author", "reviewer", forward, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation forward: %v", err)
	}
	got2, _, err := solvePrincipalRelation("release", "author", "reviewer", reversed, profile, actors)
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
		got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
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
		got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, collapsed, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverUnsatisfiable {
			t.Fatalf("State = %q, want unsatisfiable (kernel violated)", got.State)
		}
	})

	t.Run("unproven: no authenticated fillers", func(t *testing.T) {
		actors := []governanceprincipal.PrincipalResolution{unprovenActor(t, profile, "user-a")}
		got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
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

// TestSolvePrincipalRelationProfileExperimentalUnproven pins authority
// design §5.3's advisory-posture rule (ledger SI-106): "Advisory/experimental
// kernel posture is unproven for the authoritative consumer, not evidence
// that the requested relation is violated". Two distinct authenticated
// actors satisfy the distinctness rule itself, but an experimental profile
// can never produce an authoritative authorization — so the relation is
// neither proven nor violated, it is UNPROVEN.
func TestSolvePrincipalRelationProfileExperimentalUnproven(t *testing.T) {
	profile := mustDecodeProfile(t, experimentalPolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}

	got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverUnproven {
		t.Fatalf("State = %q, want unproven: advisory posture is never relation-violation evidence (proof: %+v)", got.State, got)
	}
	if !stringsContain(got.Witnesses, governanceprincipal.ReasonExperimentalAuthorityForbidden) {
		t.Fatalf("Witnesses = %v, want the kernel's own experimental finding code carried as a witness", got.Witnesses)
	}
	if err := validateSolverProof("proof", got); err != nil {
		t.Fatalf("proof failed the package's own wire validation: %v", err)
	}
}

// TestSolvePrincipalRelationProfileExperimentalNeverPasses is the other half
// of the same rule: advisory posture must not authorize an authoritative
// pass either, even when the separation rule itself is satisfied.
func TestSolvePrincipalRelationProfileExperimentalNeverPasses(t *testing.T) {
	profile := mustDecodeProfile(t, experimentalPolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}
	got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State == SolverSatisfiable {
		t.Fatalf("State = %q, want anything but satisfiable: an experimental profile can never authorize an authoritative pass", got.State)
	}
}

func TestEvaluateMechanicalProfileExperimentalRowUnproven(t *testing.T) {
	profile := mustDecodeProfile(t, experimentalPolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	if rows[0].State != ProofUnproven {
		t.Fatalf("State = %q, want unproven (advisory posture is never a mechanical violation)", rows[0].State)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonProfileExperimental {
		t.Fatalf("Reasons = %v, want [profile-experimental]", rows[0].Reasons)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed validation: %v", err)
	}
}

// approverCatalog extends testCatalog with the third role the
// required-approver fixtures below name.
func approverCatalog() governanceprincipal.Catalog {
	return governanceprincipal.Catalog{
		Roles:       []string{"approver", "author", "reviewer"},
		Transitions: []string{"release", "publish"},
	}
}

func mustDecodeProfileWith(t *testing.T, raw string, catalog governanceprincipal.Catalog) governanceprincipal.Profile {
	t.Helper()
	p, err := governanceprincipal.DecodeProfile([]byte(raw), catalog)
	if err != nil {
		t.Fatalf("DecodeProfile: %v", err)
	}
	return p
}

// approverProfileYAML is a team-class profile whose required-approvers rule
// names approverRole. Every other rule is identical to rolePolicyYAML's.
func approverProfileYAML(approverRole string) string {
	return `schema: verdi.governance-profile/v1
id: team-default
class: team
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
  - role: approver
    trust_source: github
    subjects: ["user-d"]
ownership_sources: []
signature_requirements: []
required_approvers:
  - transitions: [release]
    roles: [` + approverRole + `]
    minimum: 1
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`
}

// TestSolvePrincipalRelationIgnoresUnrelatedKernelFindings pins authority
// design §5.3's operand set: the relation question is asked over "the exact
// authenticated resolutions, transition, canonical role pair, profile, and
// separation mode". A required-approver count for a role this claim never
// names is not part of that operand set, so a kernel decision degraded to
// unproven by it alone leaves the relation itself proven — while the same
// decision shape with the approval rule naming a role the claim DOES name
// stays proven for the ordinary reason, and a whole-request authority
// violation still blocks.
func TestSolvePrincipalRelationIgnoresUnrelatedKernelFindings(t *testing.T) {
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}

	t.Run("unrelated required-approver shortfall does not unprove the relation", func(t *testing.T) {
		profile := mustDecodeProfileWith(t, approverProfileYAML("approver"), approverCatalog())
		actors := []governanceprincipal.PrincipalResolution{
			authenticatedActor(t, profile, "user-a"),
			authenticatedActor(t, profile, "user-b"),
		}
		got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverSatisfiable {
			t.Fatalf("State = %q, want satisfiable: the kernel ran the distinctness rule and reported no relation-bearing shortfall (proof: %+v)", got.State, got)
		}
	})

	t.Run("approval rule naming a claim role stays proven", func(t *testing.T) {
		profile := mustDecodeProfileWith(t, approverProfileYAML("reviewer"), approverCatalog())
		actors := []governanceprincipal.PrincipalResolution{
			authenticatedActor(t, profile, "user-a"),
			authenticatedActor(t, profile, "user-b"),
		}
		got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
		if err != nil {
			t.Fatalf("solvePrincipalRelation: %v", err)
		}
		if got.State != SolverSatisfiable {
			t.Fatalf("State = %q, want satisfiable (kernel authorized)", got.State)
		}
	})
}

func TestEvaluateMechanicalPrincipalRelationUnrelatedApproverShortfallProven(t *testing.T) {
	profile := mustDecodeProfileWith(t, approverProfileYAML("approver"), approverCatalog())
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	if rows[0].State != ProofProven {
		t.Fatalf("State = %q, want proven (an approver count is not relation evidence)", rows[0].State)
	}
	if len(rows[0].Reasons) != 1 || rows[0].Reasons[0] != ReasonMechanicalSatisfiable {
		t.Fatalf("Reasons = %v, want [mechanical-satisfiable]", rows[0].Reasons)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed validation: %v", err)
	}
}

// TestSolvePrincipalRelationRelationBearingFindingsStillBlock keeps the
// other half of the classification honest: a finding that names one of the
// claim's own roles is relation-bearing evidence and still blocks.
func TestSolvePrincipalRelationRelationBearingFindingsStillBlock(t *testing.T) {
	profile := mustDecodeProfileWith(t, `schema: verdi.governance-profile/v1
id: team-default
class: team
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
  - { id: git-signature, kind: signed-commit }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a"]
  - role: reviewer
    trust_source: github
    subjects: ["user-b"]
ownership_sources: []
signature_requirements:
  - transitions: [release]
    roles: [reviewer]
    trust_sources: [git-signature]
required_approvers:
  - transitions: [release]
    roles: [author]
    minimum: 1
distinctness_rules:
  - transitions: [release]
    left_role: author
    right_role: reviewer
    relation: different-principal
evidence_source_restrictions: []
escalation_thresholds: []
`, approverCatalog())
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())}
	got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
	if err != nil {
		t.Fatalf("solvePrincipalRelation: %v", err)
	}
	if got.State != SolverUnproven {
		t.Fatalf("State = %q, want unproven: an unproven signature finding naming role %q is evidence about a principal filling this claim's own role", got.State, "reviewer")
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
	got, _, err := solvePrincipalRelation("release", "author", "reviewer", claims, profile, actors)
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
	if _, _, err := solvePrincipalRelation("unregistered-transition", "author", "reviewer", claims, profile, nil); err == nil {
		t.Fatal("want operational error for unregistered transition, got nil")
	}
}

func TestSolvePrincipalRelationUnregisteredRoleRejected(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	claims := []policyartifact.Claim{principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "ghost-role", universalScope())}
	if _, _, err := solvePrincipalRelation("release", "author", "ghost-role", claims, profile, nil); err == nil {
		t.Fatal("want operational error for unregistered role, got nil")
	}
}

func TestSolvePrincipalRelationNoRelationTriviallySatisfiable(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	got, _, err := solvePrincipalRelation("release", "author", "reviewer", nil, profile, nil)
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 0 {
		t.Fatalf("rows = %+v, want none", rows)
	}
}

func TestEvaluateMechanicalSatisfiableRow(t *testing.T) {
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold", "silver"}, universalScope())),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpRequiredValues, []string{"gold"}, universalScope())),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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

// TestMechanicalClaimIdentityPreservesTwoPolicies pins ledger SI-105(c):
// a row claim is keyed by the composite (policy_id, claim_id), so equal
// claim BYTES declared by two different policies remain two records. Policy
// identity is part of an exemption witness and can never be discarded
// merely because two policies declare identical bytes.
func TestMechanicalClaimIdentityPreservesTwoPolicies(t *testing.T) {
	shared := discreteClaim("shared-claim", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-b", shared),
		typedClaim(t, "policy-a", shared),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 1 {
		t.Fatalf("rows = %+v, want exactly one", rows)
	}
	if err := validateMechanicalEvaluation("row", rows[0]); err != nil {
		t.Fatalf("row failed the package's own wire validation: %v", err)
	}
	if len(rows[0].Claims) != 2 {
		t.Fatalf("Claims = %+v, want one record per contributing policy identity", rows[0].Claims)
	}
	if rows[0].Claims[0].PolicyID != "policy-a" || rows[0].Claims[1].PolicyID != "policy-b" {
		t.Fatalf("Claims policy ids = %q,%q, want composite-key ascending order policy-a,policy-b",
			rows[0].Claims[0].PolicyID, rows[0].Claims[1].PolicyID)
	}
	if rows[0].Claims[0].ClaimDigest != rows[0].Claims[1].ClaimDigest {
		t.Fatalf("Claims digests = %q,%q, want the identical claim bytes retained on both records",
			rows[0].Claims[0].ClaimDigest, rows[0].Claims[1].ClaimDigest)
	}
}

// TestMechanicalClaimIdentityDuplicateCompositeCollapses covers the other
// half of composite identity: the SAME (policy_id, claim_id) supplied twice
// is one record, while two records disagreeing about that identity's
// content is contradictory authority and fails operationally.
func TestMechanicalClaimIdentityDuplicateCompositeCollapses(t *testing.T) {
	claim := discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope())
	dup := []contextcompile.TypedClaim{typedClaim(t, "policy-a", claim), typedClaim(t, "policy-a", claim)}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: dup})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(result.Evaluations) != 1 || len(result.Evaluations[0].Claims) != 1 {
		t.Fatalf("Evaluations = %+v, want one row carrying one record for the repeated identity", result.Evaluations)
	}

	other := discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"silver"}, universalScope())
	conflicting := []contextcompile.TypedClaim{typedClaim(t, "policy-a", claim), typedClaim(t, "policy-a", other)}
	if _, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: conflicting}); err == nil {
		t.Fatal("want operational error for two different claims sharing one (policy_id, claim_id), got nil")
	}
}

// TestMechanicalClaimDigestMutationRefused pins ledger SI-105(c)'s digest
// recomputation: a carried claim_digest that is not the canonical digest of
// the carried base claim is hand-built or mutated operand drift, refused
// operationally rather than trusted.
func TestMechanicalClaimDigestMutationRefused(t *testing.T) {
	good := typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"gold"}, universalScope()))

	t.Run("recomputed digest accepted", func(t *testing.T) {
		if _, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: []contextcompile.TypedClaim{good}}); err != nil {
			t.Fatalf("EvaluateMechanical(honest digest): %v", err)
		}
	})

	t.Run("mutated digest refused", func(t *testing.T) {
		mutated := good
		mutated.ClaimDigest = testDigest64
		if _, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: []contextcompile.TypedClaim{mutated}}); err == nil {
			t.Fatal("want operational error for a claim digest that does not recompute, got nil")
		}
	})

	t.Run("mutated claim body refused", func(t *testing.T) {
		swapped := good
		swapped.Claim = discreteClaim("c1", "level", policyartifact.OpAllowedValues, []string{"silver"}, universalScope())
		if _, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: []contextcompile.TypedClaim{swapped}}); err == nil {
			t.Fatal("want operational error for a claim body that no longer digests to its carried digest, got nil")
		}
	})
}

// TestMechanicalClaimIdentityPairRowsStayDistinct pins ledger SI-109: a
// step-2 pair-row component is the canonical digest of that claim's
// composite (policy_id, claim_id) identity, never the claim digest alone.
// Two policies declaring BYTE-IDENTICAL contradictory claims each pair with
// the same third claim, so claim-digest components would mint two rows
// carrying one ID — a duplicate the report's own row-ID uniqueness rule
// refuses (a legitimate gate run failing operationally) and an ordering
// sort.Slice cannot make deterministic.
func TestMechanicalClaimIdentityPairRowsStayDistinct(t *testing.T) {
	shared := discreteClaim("shared-claim", "level", policyartifact.OpEquals, []string{"gold"}, universalScope())
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", shared),
		typedClaim(t, "policy-b", shared),
		typedClaim(t, "policy-c", discreteClaim("other-claim", "level", policyartifact.OpEquals, []string{"silver"}, phaseScope("build"))),
	}
	in := MechanicalInput{Claims: claims}

	result, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
	if len(rows) != 2 {
		t.Fatalf("rows = %+v, want exactly two differently-scoped pair witnesses (one per contradicting policy identity)", rows)
	}
	for i, row := range rows {
		if row.State != ProofViolatedWithWitness {
			t.Fatalf("rows[%d].State = %q, want violated-with-witness", i, row.State)
		}
		if err := validateMechanicalEvaluation(fmt.Sprintf("row[%d]", i), row); err != nil {
			t.Fatalf("rows[%d] failed the package's own wire validation: %v", i, err)
		}
	}
	if rows[0].ID == rows[1].ID {
		t.Fatalf("pair rows share one ID %q: byte-identical claims from two policies collided", rows[0].ID)
	}
	// The exact report-level rule a colliding ID breaks.
	if err := requireSortedUnique("report.mechanical", rows, func(m MechanicalEvaluation) string { return m.ID }); err != nil {
		t.Fatalf("pair rows fail the report's own row-ID uniqueness rule: %v", err)
	}

	again, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical(second run): %v", err)
	}
	if !reflect.DeepEqual(result, again) {
		t.Fatalf("EvaluateMechanical is not deterministic:\n run1: %+v\n run2: %+v", result, again)
	}
}

// --- SI-106: exact distinctness role-pair attribution ------------------------

// TestRelationBearingFindingDistinctnessRolesAttribution pins authority
// design §5.3's operand set (ledger SI-106): the evaluator consumes only
// whole-request authority findings plus findings whose exact role or sorted
// role pair belongs to the requested relation. A SECOND distinctness rule's
// finding, carrying its own role pair, is not evidence about this claim's
// relation and must not flip it.
func TestRelationBearingFindingDistinctnessRolesAttribution(t *testing.T) {
	cases := []struct {
		name    string
		finding governanceprincipal.Finding
		want    bool
	}{
		{
			"this claim's own distinctness pair bears",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonDistinctnessViolated,
				State: governanceprincipal.AuthorizationViolated,
				Roles: []string{"author", "reviewer"},
			},
			true,
		},
		{
			"a second distinctness rule's pair does not bear",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonDistinctnessViolated,
				State: governanceprincipal.AuthorizationViolated,
				Roles: []string{"author", "owner"},
			},
			false,
		},
		{
			"a second rule's unproven pair does not bear even when it names one shared role",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonDistinctnessUnproven,
				State: governanceprincipal.AuthorizationUnproven,
				Role:  "author",
				Roles: []string{"author", "owner"},
			},
			false,
		},
		{
			"whole-request experimental authority always bears",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonExperimentalAuthorityForbidden,
				State: governanceprincipal.AuthorizationViolated,
			},
			true,
		},
		{
			"whole-request inapplicable transition always bears",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonTransitionNotApplicable,
				State: governanceprincipal.AuthorizationViolated,
			},
			true,
		},
		{
			"a signature shortfall naming one of this claim's roles bears",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonSignatureUnproven,
				State: governanceprincipal.AuthorizationUnproven,
				Role:  "reviewer",
			},
			true,
		},
		{
			"an approver shortfall naming no role of this claim does not bear",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonRequiredApproverMissing,
				State: governanceprincipal.AuthorizationUnproven,
			},
			false,
		},
		{
			"an ownership shortfall about an unrelated role does not bear",
			governanceprincipal.Finding{
				Code:  governanceprincipal.ReasonOwnershipViolated,
				State: governanceprincipal.AuthorizationViolated,
				Role:  "owner",
			},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relationBearingFinding(tc.finding, "author", "reviewer"); got != tc.want {
				t.Fatalf("relationBearingFinding(%+v) = %v, want %v", tc.finding, got, tc.want)
			}
			// The canonical pair normalizes lexically, so a reversed
			// spelling of the same requested relation classifies identically.
			if got := relationBearingFinding(tc.finding, "reviewer", "author"); got != tc.want {
				t.Fatalf("relationBearingFinding(%+v) with reversed roles = %v, want %v", tc.finding, got, tc.want)
			}
		})
	}
}

// --- SI-106/SI-108: kernel disclosure translation ---------------------------

// soloCollapsePolicyYAML is a solo profile in which BOTH mapped subjects
// hold both roles, so the kernel discloses two distinct solo collapses.
const soloCollapsePolicyYAML = `schema: verdi.governance-profile/v1
id: team-default
class: solo
applicable_transitions: [release]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-a", "user-b"]
  - role: reviewer
    trust_source: github
    subjects: ["user-a", "user-b"]
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

// TestMechanicalDisclosureSoloCollapseTranslation pins ledger SI-108: each
// kernel solo-collapse principal/role membership becomes exactly one report
// witness token `<principal_id>:<role_id>` under report code
// solo-principal-collapse, sorted and deduplicated, with duplicate
// translations from repeated kernel calls collapsing into one disclosure.
func TestMechanicalDisclosureSoloCollapseTranslation(t *testing.T) {
	profile := mustDecodeProfile(t, soloCollapsePolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	// Two differently-scoped claims in one group force the complete solve
	// AND the scope-witness solves to call the kernel, so the identical
	// disclosure is translated several times.
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", phaseScope("build"))),
		typedClaim(t, "policy-b", principalClaim("c2", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", phaseScope("review"))),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if len(result.Disclosures) != 1 {
		t.Fatalf("Disclosures = %+v, want exactly one collapsed disclosure", result.Disclosures)
	}
	got := result.Disclosures[0]
	if got.Code != DisclosureSoloPrincipalCollapse {
		t.Fatalf("Code = %q, want %q", got.Code, DisclosureSoloPrincipalCollapse)
	}
	pa := mustPrincipalID(t, "user-a")
	pb := mustPrincipalID(t, "user-b")
	want := []string{pa + ":author", pa + ":reviewer", pb + ":author", pb + ":reviewer"}
	sort.Strings(want)
	if !reflect.DeepEqual(got.Witnesses, want) {
		t.Fatalf("Witnesses = %v, want the sorted deduplicated membership tokens %v", got.Witnesses, want)
	}
	if err := validateDisclosure("disclosure", got); err != nil {
		t.Fatalf("translated disclosure failed the package's own wire validation: %v", err)
	}
}

// TestMechanicalDisclosureNoneWithoutKernelEvidence keeps the empty case
// honest: a profile that discloses nothing yields an explicitly empty set,
// never a nil or a manufactured disclosure.
func TestMechanicalDisclosureNoneWithoutKernelEvidence(t *testing.T) {
	profile := mustDecodeProfile(t, rolePolicyYAML)
	actors := []governanceprincipal.PrincipalResolution{
		authenticatedActor(t, profile, "user-a"),
		authenticatedActor(t, profile, "user-b"),
	}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", principalClaim("c1", "release", policyartifact.OpDifferentPrincipal, "author", "reviewer", universalScope())),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	if result.Disclosures == nil || len(result.Disclosures) != 0 {
		t.Fatalf("Disclosures = %+v, want an explicitly empty set", result.Disclosures)
	}
}

// TestMechanicalDisclosureTranslationRefusals pins the closed-vocabulary and
// SI-108 grammar rules the translation fails closed on. These operands
// cannot be produced by the landed kernel, so they are supplied directly:
// the point is that policyconflict never invents a report label or a lossy
// token when a future kernel does produce them.
func TestMechanicalDisclosureTranslationRefusals(t *testing.T) {
	principal := mustPrincipalID(t, "user-a")

	t.Run("known code translates", func(t *testing.T) {
		got, err := translateKernelDisclosures([]governanceprincipal.Disclosure{{
			Code:        governanceprincipal.ReasonSoloRoleCollapse,
			PrincipalID: governanceprincipal.PrincipalID(principal),
			Roles:       []string{"reviewer", "author"},
		}})
		if err != nil {
			t.Fatalf("translateKernelDisclosures: %v", err)
		}
		want := []Disclosure{{Code: DisclosureSoloPrincipalCollapse, Witnesses: []string{principal + ":author", principal + ":reviewer"}}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	negatives := []struct {
		name string
		in   governanceprincipal.Disclosure
	}{
		{"unknown kernel disclosure code", governanceprincipal.Disclosure{
			Code: "some-future-disclosure", PrincipalID: governanceprincipal.PrincipalID(principal), Roles: []string{"author"},
		}},
		{"principal component outside its grammar", governanceprincipal.Disclosure{
			Code: governanceprincipal.ReasonSoloRoleCollapse, PrincipalID: "principal/github", Roles: []string{"author"},
		}},
		{"principal component containing a colon", governanceprincipal.Disclosure{
			Code: governanceprincipal.ReasonSoloRoleCollapse, PrincipalID: governanceprincipal.PrincipalID(principal + ":extra"), Roles: []string{"author"},
		}},
		{"role component containing a colon", governanceprincipal.Disclosure{
			Code: governanceprincipal.ReasonSoloRoleCollapse, PrincipalID: governanceprincipal.PrincipalID(principal), Roles: []string{"author:reviewer"},
		}},
		{"role component outside its grammar", governanceprincipal.Disclosure{
			Code: governanceprincipal.ReasonSoloRoleCollapse, PrincipalID: governanceprincipal.PrincipalID(principal), Roles: []string{"Author"},
		}},
		{"membership naming no role at all", governanceprincipal.Disclosure{
			Code: governanceprincipal.ReasonSoloRoleCollapse, PrincipalID: governanceprincipal.PrincipalID(principal), Roles: nil,
		}},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := translateKernelDisclosures([]governanceprincipal.Disclosure{tc.in}); err == nil {
				t.Fatalf("want operational error for %s, got nil", tc.name)
			}
		})
	}
}

func TestEvaluateMechanicalUnknownRefHigherOrderUnproven(t *testing.T) {
	resolver := &mapRefResolver{states: map[[2]string]ScopeState{{"spec/a", "spec/b"}: ScopeUnknown}}
	claims := []contextcompile.TypedClaim{
		typedClaim(t, "policy-a", discreteClaim("c1", "level", policyartifact.OpEquals, []string{"gold"}, refScope("spec/a"))),
		typedClaim(t, "policy-b", discreteClaim("c2", "level", policyartifact.OpEquals, []string{"silver"}, refScope("spec/b"))),
	}
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Refs: resolver})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result1, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows1 := result1.Evaluations
	result2, err := EvaluateMechanical(context.Background(), in)
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows2 := result2.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile, Actors: actors})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
	result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile})
	if err != nil {
		t.Fatalf("EvaluateMechanical: %v", err)
	}
	rows := result.Evaluations
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
		result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: collapsed, Actors: actors})
		if err != nil {
			t.Fatalf("EvaluateMechanical: %v", err)
		}
		rows := result.Evaluations
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
		result, err := EvaluateMechanical(context.Background(), MechanicalInput{Claims: claims, Profile: profile})
		if err != nil {
			t.Fatalf("EvaluateMechanical: %v", err)
		}
		rows := result.Evaluations
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
