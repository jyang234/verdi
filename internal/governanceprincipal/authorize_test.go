package governanceprincipal

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

// authzRoleMappings maps author, reviewer, and owner roles so every rule
// family is exercisable; user-123 additionally holds owner for the
// same-principal tests.
const authzRoleMappings = `role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123"]
  - role: reviewer
    trust_source: github
    subjects: ["user-456", "user-789"]
  - role: owner
    trust_source: github
    subjects: ["user-999", "user-123"]
`

// authzProfile is the team fixture with all three roles mapped.
func authzProfile(t *testing.T, replace map[string]string) Profile {
	t.Helper()
	merged := map[string]string{"role_mappings": authzRoleMappings}
	for k, v := range replace {
		merged[k] = v
	}
	return mustDecode(t, profileYAMLWith(merged))
}

func mustPID(t *testing.T, subject string) PrincipalID {
	t.Helper()
	id, err := CanonicalPrincipalID("github", subject)
	if err != nil {
		t.Fatalf("CanonicalPrincipalID: %v", err)
	}
	return id
}

// mintResolution obtains a genuine resolution through Resolver.Resolve —
// the only way a consumable resolution exists.
func mintResolution(t *testing.T, fact TrustFact, subject string, want ResolutionState) PrincipalResolution {
	t.Helper()
	profile := mustDecode(t, profileYAML())
	r := NewResolver(staticFact(fact))
	res, err := r.Resolve(context.Background(), profile, PrincipalClaim{TrustSource: "github", Subject: subject})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if res.State != want {
		t.Fatalf("minted resolution state = %q, want %q", res.State, want)
	}
	return res
}

// authedRes resolves a github subject to a genuine authenticated
// resolution.
func authedRes(t *testing.T, subject string) PrincipalResolution {
	t.Helper()
	return mintResolution(t, githubFact(subject), subject, ResolutionAuthenticated)
}

// failedRes resolves a github subject to a genuine violated or unproven
// resolution.
func failedRes(t *testing.T, subject string, state ResolutionState) PrincipalResolution {
	t.Helper()
	switch state {
	case ResolutionViolated:
		return mintResolution(t, githubFact("someone-else"), subject, state)
	case ResolutionUnproven:
		return mintResolution(t, TrustFact{
			SourceID:   "github",
			SourceKind: TrustSourceForge,
			Available:  false,
			Valid:      false,
			Reason:     "forge api unreachable",
		}, subject, state)
	}
	t.Fatalf("failedRes: unsupported state %q", state)
	return PrincipalResolution{}
}

func findingCodes(d AuthorizationDecision) []string {
	codes := make([]string, 0, len(d.Findings))
	for _, f := range d.Findings {
		codes = append(codes, f.Code)
	}
	return codes
}

func wantDecision(t *testing.T, d AuthorizationDecision, state AuthorizationState, codes ...string) {
	t.Helper()
	if d.State != state {
		t.Errorf("State = %q, want %q (findings: %+v)", d.State, state, d.Findings)
	}
	got := findingCodes(d)
	if len(codes) == 0 {
		if len(got) != 0 {
			t.Errorf("Findings = %v, want none", got)
		}
		return
	}
	for _, c := range codes {
		if !contains(got, c) {
			t.Errorf("finding codes %v missing %q", got, c)
		}
	}
}

func TestAuthorizeTeamHappyPath(t *testing.T) {
	profile := authzProfile(t, nil)
	req := AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
		},
	}
	d, err := Authorize(profile, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if d.Transition != "accept" || d.Posture != PostureAuthoritative {
		t.Errorf("decision = %+v, want accept/authoritative", d)
	}
	wantDecision(t, d, AuthorizationAuthorized)
	if len(d.Disclosures) != 0 {
		t.Errorf("Disclosures = %+v, want none for a team profile", d.Disclosures)
	}
}

func TestAuthorizeTransitionNotApplicable(t *testing.T) {
	profile := authzProfile(t, nil)
	d, err := Authorize(profile, AuthorizationRequest{Transition: "publish", Posture: PostureAuthoritative})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationViolated, ReasonTransitionNotApplicable)
}

func TestAuthorizePrincipalAndRoleFindings(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")

	tests := []struct {
		name      string
		res       []PrincipalResolution
		approvals []ApprovalRecord
		wantState AuthorizationState
		wantCodes []string
	}{
		{
			"approval with no resolution",
			[]PrincipalResolution{authedRes(t, "user-123")},
			[]ApprovalRecord{{Role: "author", PrincipalID: mustPID(t, "user-123")}, {Role: "reviewer", PrincipalID: pRev}},
			AuthorizationUnproven,
			[]string{ReasonPrincipalUnproven, ReasonRequiredApproverMissing},
		},
		{
			"approval with violated resolution",
			[]PrincipalResolution{failedRes(t, "user-456", ResolutionViolated)},
			[]ApprovalRecord{{Role: "reviewer", PrincipalID: pRev}},
			AuthorizationViolated,
			[]string{ReasonPrincipalViolated, ReasonRequiredApproverMissing},
		},
		{
			"approval with unproven resolution",
			[]PrincipalResolution{failedRes(t, "user-456", ResolutionUnproven)},
			[]ApprovalRecord{{Role: "reviewer", PrincipalID: pRev}},
			AuthorizationUnproven,
			[]string{ReasonPrincipalUnproven, ReasonRequiredApproverMissing},
		},
		{
			"authenticated principal without the role",
			[]PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
			[]ApprovalRecord{{Role: "reviewer", PrincipalID: mustPID(t, "user-123")}, {Role: "reviewer", PrincipalID: pRev}},
			AuthorizationViolated,
			[]string{ReasonRoleNotAuthorized},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Authorize(profile, AuthorizationRequest{
				Transition:  "accept",
				Posture:     PostureAuthoritative,
				Resolutions: tt.res,
				Approvals:   tt.approvals,
			})
			if err != nil {
				t.Fatalf("Authorize: %v", err)
			}
			wantDecision(t, d, tt.wantState, tt.wantCodes...)
		})
	}
}

// TestAuthorizeApproverCounting: required approvers count distinct
// authenticated principal IDs; duplicate approvals collapse.
func TestAuthorizeApproverCounting(t *testing.T) {
	profile := authzProfile(t, map[string]string{
		"required_approvers": `required_approvers:
  - transitions: [accept]
    roles: [reviewer]
    minimum: 2
  - transitions: [close, merge-authorize]
    roles: [reviewer]
    minimum: 1
`,
	})

	one := AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")}, // duplicate never double-counts
		},
	}
	d, err := Authorize(profile, one)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationUnproven, ReasonRequiredApproverMissing)

	two := one
	two.Resolutions = []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456"), authedRes(t, "user-789")}
	two.Approvals = []ApprovalRecord{
		{Role: "author", PrincipalID: mustPID(t, "user-123")},
		{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
		{Role: "reviewer", PrincipalID: mustPID(t, "user-789")},
	}
	d, err = Authorize(profile, two)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationAuthorized)
}

func TestAuthorizeDistinctness(t *testing.T) {
	// user-123 fills author and reviewer via an extended reviewer mapping.
	collapseMappings := `role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123"]
  - role: reviewer
    trust_source: github
    subjects: ["user-123", "user-456"]
  - role: owner
    trust_source: github
    subjects: ["user-999", "user-123"]
`
	t.Run("different-principal violated", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"role_mappings": collapseMappings})
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-123")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationViolated, ReasonDistinctnessViolated)
	})

	sameRule := `distinctness_rules:
  - transitions: [close, accept, merge-authorize]
    left_role: author
    right_role: reviewer
    relation: different-principal
  - transitions: [accept]
    left_role: author
    right_role: owner
    relation: same-principal
`
	t.Run("same-principal violated", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"distinctness_rules": sameRule})
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456"), authedRes(t, "user-999")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
				{Role: "owner", PrincipalID: mustPID(t, "user-999")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationViolated, ReasonDistinctnessViolated)
	})
	t.Run("same-principal satisfied", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"distinctness_rules": sameRule})
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
				{Role: "owner", PrincipalID: mustPID(t, "user-123")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationAuthorized)
	})
	t.Run("unfilled side is unproven, never vacuously satisfied", func(t *testing.T) {
		profile := authzProfile(t, nil)
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-456")},
			Approvals:   []ApprovalRecord{{Role: "reviewer", PrincipalID: mustPID(t, "user-456")}},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonDistinctnessUnproven)
	})
}

// TestAuthorizeDistinctnessRolesCarryExactPair pins ledger SI-106: the
// kernel exposes each distinctness finding's EXACT sorted role pair, so a
// consumer never has to infer rule identity from human detail text and can
// tell a second rule's finding apart from its own. Findings from every other
// rule family carry no pair at all.
func TestAuthorizeDistinctnessRolesCarryExactPair(t *testing.T) {
	twoRules := `distinctness_rules:
  - transitions: [close, accept, merge-authorize]
    left_role: author
    right_role: reviewer
    relation: different-principal
  - transitions: [accept]
    left_role: owner
    right_role: author
    relation: different-principal
`
	findingFor := func(t *testing.T, d AuthorizationDecision, code string, roles ...string) Finding {
		t.Helper()
		for _, f := range d.Findings {
			if f.Code == code && reflect.DeepEqual(f.Roles, roles) {
				return f
			}
		}
		t.Fatalf("no %q finding carrying role pair %v in %+v", code, roles, d.Findings)
		return Finding{}
	}

	t.Run("violated finding carries its own sorted pair", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"distinctness_rules": twoRules})
		// user-123 fills author AND owner, so only the (author, owner) rule
		// is violated; (author, reviewer) is satisfied by user-456.
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
				{Role: "owner", PrincipalID: mustPID(t, "user-123")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		got := findingFor(t, d, ReasonDistinctnessViolated, "author", "owner")
		if got.State != AuthorizationViolated {
			t.Fatalf("finding state = %q, want violated-with-witness", got.State)
		}
		for _, f := range d.Findings {
			if f.Code == ReasonDistinctnessViolated && reflect.DeepEqual(f.Roles, []string{"author", "reviewer"}) {
				t.Fatalf("the satisfied (author, reviewer) rule must produce no finding: %+v", f)
			}
		}
	})

	t.Run("unproven finding carries its own sorted pair", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"distinctness_rules": twoRules})
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-456")},
			Approvals:   []ApprovalRecord{{Role: "reviewer", PrincipalID: mustPID(t, "user-456")}},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		byPair := findingFor(t, d, ReasonDistinctnessUnproven, "author", "reviewer")
		if byPair.Role != "author" {
			t.Fatalf("finding role = %q, want the unfilled role author", byPair.Role)
		}
		// The second rule's own unproven findings carry ITS pair, reversed
		// spelling normalized lexically.
		findingFor(t, d, ReasonDistinctnessUnproven, "author", "owner")
	})

	t.Run("other rule families carry no pair", func(t *testing.T) {
		profile := authzProfile(t, map[string]string{"distinctness_rules": twoRules})
		// The unresolved owner approval adds a principal-unproven finding:
		// a different rule family, which must carry no pair.
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-456")},
			Approvals: []ApprovalRecord{
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
				{Role: "owner", PrincipalID: mustPID(t, "user-999")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		saw := false
		for _, f := range d.Findings {
			switch f.Code {
			case ReasonDistinctnessViolated, ReasonDistinctnessUnproven:
				if len(f.Roles) != 2 {
					t.Fatalf("distinctness finding %+v must carry exactly two roles", f)
				}
			default:
				saw = true
				if len(f.Roles) != 0 {
					t.Fatalf("finding %+v is not a distinctness finding and must carry no role pair", f)
				}
			}
		}
		if !saw {
			t.Fatalf("expected at least one non-distinctness finding in %+v", d.Findings)
		}
	})
}

// TestHoldsRoleIsTheOneValidatedRoleQuery pins ledger SI-106's single
// exported role-membership seam: consumers must not duplicate role-mapping
// semantics, so the exported query is exactly the kernel's own private
// predicate behind validated operands.
func TestHoldsRoleIsTheOneValidatedRoleQuery(t *testing.T) {
	profile := authzProfile(t, nil)

	t.Run("parity with the kernel's own predicate", func(t *testing.T) {
		claims := []PrincipalClaim{
			{TrustSource: "github", Subject: "user-123"},
			{TrustSource: "github", Subject: "user-456"},
			{TrustSource: "github", Subject: "user-999"},
			{TrustSource: "github", Subject: "nobody"},
		}
		for _, claim := range claims {
			for _, role := range []string{"author", "reviewer", "owner"} {
				got, err := HoldsRole(profile, claim, role)
				if err != nil {
					t.Fatalf("HoldsRole(%q, %q): %v", claim.Subject, role, err)
				}
				if want := holdsRole(profile, claim, role); got != want {
					t.Fatalf("HoldsRole(%q, %q) = %v, want kernel parity %v", claim.Subject, role, got, want)
				}
			}
		}
	})

	t.Run("known membership", func(t *testing.T) {
		got, err := HoldsRole(profile, PrincipalClaim{TrustSource: "github", Subject: "user-123"}, "author")
		if err != nil || !got {
			t.Fatalf("HoldsRole = %v, %v; want true, nil", got, err)
		}
	})

	t.Run("unmapped role for this principal", func(t *testing.T) {
		got, err := HoldsRole(profile, PrincipalClaim{TrustSource: "github", Subject: "user-123"}, "reviewer")
		if err != nil || got {
			t.Fatalf("HoldsRole = %v, %v; want false, nil", got, err)
		}
	})

	negatives := []struct {
		name  string
		claim PrincipalClaim
		role  string
	}{
		{"malformed role id", PrincipalClaim{TrustSource: "github", Subject: "user-123"}, "Author"},
		{"blank role id", PrincipalClaim{TrustSource: "github", Subject: "user-123"}, ""},
		{"malformed trust source", PrincipalClaim{TrustSource: "GitHub", Subject: "user-123"}, "author"},
		{"blank subject", PrincipalClaim{TrustSource: "github", Subject: ""}, "author"},
	}
	for _, tc := range negatives {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := HoldsRole(profile, tc.claim, tc.role); err == nil {
				t.Fatalf("want operational error for %s, got nil", tc.name)
			}
		})
	}

	t.Run("unsealed profile is refused", func(t *testing.T) {
		if _, err := HoldsRole(Profile{}, PrincipalClaim{TrustSource: "github", Subject: "user-123"}, "author"); err == nil {
			t.Fatal("want operational error for a profile that did not come from DecodeProfile, got nil")
		}
	})
}

func TestAuthorizeSoloCollapse(t *testing.T) {
	solo := mustDecode(t, []byte(soloYAML))
	req := AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-123")},
		},
	}
	d, err := Authorize(solo, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationAuthorized)
	if len(d.Disclosures) != 1 {
		t.Fatalf("Disclosures = %+v, want exactly one solo-role-collapse", d.Disclosures)
	}
	disc := d.Disclosures[0]
	if disc.Code != ReasonSoloRoleCollapse || disc.PrincipalID != mustPID(t, "user-123") {
		t.Errorf("disclosure = %+v, want %s for user-123", disc, ReasonSoloRoleCollapse)
	}
	if want := []string{"author", "reviewer"}; !reflect.DeepEqual(disc.Roles, want) {
		t.Errorf("disclosure roles = %v, want %v", disc.Roles, want)
	}

	// A different-principal rule forbids the collapse even in solo.
	forbidden := mustDecode(t, []byte(strings.Replace(soloYAML,
		"distinctness_rules: []",
		"distinctness_rules:\n  - transitions: [accept]\n    left_role: author\n    right_role: reviewer\n    relation: different-principal", 1)))
	d, err = Authorize(forbidden, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationViolated, ReasonDistinctnessViolated)
}

func signatureFact(state RuleFactState, principal PrincipalID) RuleFact {
	f := RuleFact{
		Kind:        RuleFactSignature,
		SourceID:    "git-signature",
		PrincipalID: principal,
		State:       state,
		Reason:      "signature verification result",
	}
	if state == RuleFactProven || state == RuleFactViolated {
		f.EvidenceDigest = testDigest
	}
	return f
}

func TestAuthorizeSignatureRules(t *testing.T) {
	profile := authzProfile(t, nil)
	pAuthor := mustPID(t, "user-123")
	base := AuthorizationRequest{
		Transition:  "merge-authorize",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: pAuthor},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
		},
	}

	tests := []struct {
		name      string
		facts     []RuleFact
		wantState AuthorizationState
		wantCodes []string
	}{
		{"proven", []RuleFact{signatureFact(RuleFactProven, pAuthor)}, AuthorizationAuthorized, nil},
		{"violated", []RuleFact{signatureFact(RuleFactViolated, pAuthor)}, AuthorizationViolated, []string{ReasonSignatureViolated}},
		{"unproven fact", []RuleFact{signatureFact(RuleFactUnproven, pAuthor)}, AuthorizationUnproven, []string{ReasonSignatureUnproven}},
		{"missing fact", nil, AuthorizationUnproven, []string{ReasonSignatureUnproven}},
		{"fact for another principal only", []RuleFact{signatureFact(RuleFactProven, mustPID(t, "user-456"))}, AuthorizationUnproven, []string{ReasonSignatureUnproven}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := base
			req.RuleFacts = tt.facts
			d, err := Authorize(profile, req)
			if err != nil {
				t.Fatalf("Authorize: %v", err)
			}
			wantDecision(t, d, tt.wantState, tt.wantCodes...)
		})
	}
}

func ownershipFact(state RuleFactState, principal PrincipalID) RuleFact {
	f := RuleFact{
		Kind:        RuleFactOwnership,
		SourceID:    "repository-owners",
		PrincipalID: principal,
		State:       state,
		Reason:      "ownership evaluation result",
	}
	if state == RuleFactProven || state == RuleFactViolated {
		f.EvidenceDigest = testDigest
	}
	return f
}

// closeRequest satisfies close's evidence restriction, escalation metric,
// and author/reviewer distinctness so ownership behavior is isolated.
func closeRequest(t *testing.T, facts []RuleFact) AuthorizationRequest {
	t.Helper()
	return AuthorizationRequest{
		Transition:  "close",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
		Approvals: []ApprovalRecord{
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
		},
		RuleFacts:         facts,
		EvidenceSources:   []string{"merge-gate"},
		EscalationMetrics: map[string]int{"unresolved-exceptions": 0},
	}
}

func TestAuthorizeOwnershipRules(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")
	tests := []struct {
		name      string
		facts     []RuleFact
		wantState AuthorizationState
		wantCodes []string
	}{
		{"proven", []RuleFact{ownershipFact(RuleFactProven, pRev)}, AuthorizationAuthorized, nil},
		{"violated", []RuleFact{ownershipFact(RuleFactViolated, pRev)}, AuthorizationViolated, []string{ReasonOwnershipViolated}},
		{"unproven fact", []RuleFact{ownershipFact(RuleFactUnproven, pRev)}, AuthorizationUnproven, []string{ReasonOwnershipUnproven}},
		{"missing fact", nil, AuthorizationUnproven, []string{ReasonOwnershipUnproven}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := Authorize(profile, closeRequest(t, tt.facts))
			if err != nil {
				t.Fatalf("Authorize: %v", err)
			}
			wantDecision(t, d, tt.wantState, tt.wantCodes...)
		})
	}
}

func TestAuthorizeEvidenceRestrictions(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")

	t.Run("forbidden source", func(t *testing.T) {
		req := closeRequest(t, []RuleFact{ownershipFact(RuleFactProven, pRev)})
		req.EvidenceSources = []string{"ci-verify"}
		d, err := Authorize(profile, req)
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationViolated, ReasonEvidenceSourceForbidden)
	})
	t.Run("restriction with no presented source is unproven", func(t *testing.T) {
		req := closeRequest(t, []RuleFact{ownershipFact(RuleFactProven, pRev)})
		req.EvidenceSources = nil
		d, err := Authorize(profile, req)
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonEvidenceSourceUnproven)
	})
	t.Run("unrestricted transition accepts any presented source", func(t *testing.T) {
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: pRev},
			},
			EvidenceSources: []string{"ci-verify"},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationAuthorized)
	})
}

func TestAuthorizeEscalationThresholds(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")
	base := func(metric map[string]int) AuthorizationRequest {
		req := closeRequest(t, []RuleFact{ownershipFact(RuleFactProven, pRev)})
		req.EscalationMetrics = metric
		return req
	}

	t.Run("below threshold", func(t *testing.T) {
		d, err := Authorize(profile, base(map[string]int{"unresolved-exceptions": 0}))
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationAuthorized)
	})
	t.Run("at threshold requires escalation role", func(t *testing.T) {
		d, err := Authorize(profile, base(map[string]int{"unresolved-exceptions": 1}))
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonEscalationRoleMissing)
	})
	t.Run("above threshold requires escalation role", func(t *testing.T) {
		d, err := Authorize(profile, base(map[string]int{"unresolved-exceptions": 3}))
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonEscalationRoleMissing)
	})
	t.Run("at threshold with escalation approver", func(t *testing.T) {
		req := base(map[string]int{"unresolved-exceptions": 1})
		req.Resolutions = append(req.Resolutions, authedRes(t, "user-999"))
		req.Approvals = append(req.Approvals, ApprovalRecord{Role: "owner", PrincipalID: mustPID(t, "user-999")})
		d, err := Authorize(profile, req)
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationAuthorized)
	})
	t.Run("missing metric value is unproven", func(t *testing.T) {
		d, err := Authorize(profile, base(nil))
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonEscalationMetricUnavailable)
	})
}

func TestAuthorizeExperimentalPosture(t *testing.T) {
	exp := mustDecode(t, []byte(strings.Replace(strings.Replace(soloYAML, "class: solo", "class: experimental", 1), "id: solo-default", "id: exp-default", 1)))
	req := AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAdvisory,
		Resolutions: []PrincipalResolution{authedRes(t, "user-123")},
		Approvals:   []ApprovalRecord{{Role: "author", PrincipalID: mustPID(t, "user-123")}},
	}
	d, err := Authorize(exp, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if d.Posture != PostureAdvisory {
		t.Errorf("Posture = %q, want advisory", d.Posture)
	}
	wantDecision(t, d, AuthorizationAuthorized)

	req.Posture = PostureAuthoritative
	d, err = Authorize(exp, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if d.Posture != PostureAdvisory {
		t.Errorf("Posture = %q, want advisory even when authoritative was requested", d.Posture)
	}
	wantDecision(t, d, AuthorizationViolated, ReasonExperimentalAuthorityForbidden)
}

// TestAuthorizeViolatedOutranksUnproven: explicit contradiction wins over
// unproven, and unproven wins over authorized.
func TestAuthorizeViolatedOutranksUnproven(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")
	req := closeRequest(t, []RuleFact{ownershipFact(RuleFactViolated, pRev)})
	req.EvidenceSources = nil // adds an unproven evidence finding
	d, err := Authorize(profile, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationViolated, ReasonOwnershipViolated, ReasonEvidenceSourceUnproven)
}

// TestAuthorizeDeterministicOrdering: permuted input slices produce the
// identical decision with sorted findings.
func TestAuthorizeDeterministicOrdering(t *testing.T) {
	profile := authzProfile(t, nil)
	req := closeRequest(t, nil) // ownership unproven
	req.EvidenceSources = []string{"ci-verify"}
	req.Approvals = append(req.Approvals, ApprovalRecord{Role: "reviewer", PrincipalID: mustPID(t, "user-789")})
	req.Resolutions = append(req.Resolutions, authedRes(t, "user-789"))

	permuted := req
	permuted.Approvals = make([]ApprovalRecord, 0, len(req.Approvals))
	for i := len(req.Approvals) - 1; i >= 0; i-- {
		permuted.Approvals = append(permuted.Approvals, req.Approvals[i])
	}
	permuted.Resolutions = make([]PrincipalResolution, 0, len(req.Resolutions))
	for i := len(req.Resolutions) - 1; i >= 0; i-- {
		permuted.Resolutions = append(permuted.Resolutions, req.Resolutions[i])
	}

	d1, err := Authorize(profile, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	d2, err := Authorize(profile, permuted)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	if !reflect.DeepEqual(d1, d2) {
		t.Errorf("decisions differ across input permutations:\n%+v\n%+v", d1, d2)
	}
	for i := 1; i < len(d1.Findings); i++ {
		if findingLess(d1.Findings[i], d1.Findings[i-1]) {
			t.Errorf("findings not sorted at %d: %+v", i, d1.Findings)
		}
	}
	if len(d1.Findings) < 3 {
		t.Fatalf("expected multiple findings, got %+v", d1.Findings)
	}
}

func TestAuthorizeMalformedRequests(t *testing.T) {
	profile := authzProfile(t, nil)
	pRev := mustPID(t, "user-456")
	good := func() AuthorizationRequest {
		return AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-456")},
			Approvals:   []ApprovalRecord{{Role: "reviewer", PrincipalID: pRev}},
		}
	}

	tests := []struct {
		name    string
		mutate  func(*AuthorizationRequest)
		wantSub string
	}{
		{"invalid transition id", func(r *AuthorizationRequest) { r.Transition = "Accept" }, "invalid id"},
		{"unknown posture", func(r *AuthorizationRequest) { r.Posture = "definitive" }, "posture"},
		{"unknown resolution state", func(r *AuthorizationRequest) { r.Resolutions[0].State = "certified" }, "resolution state"},
		{"authenticated resolution without principal id", func(r *AuthorizationRequest) { r.Resolutions[0].PrincipalID = "" }, "principal"},
		{"authenticated resolution with mismatched principal id", func(r *AuthorizationRequest) { r.Resolutions[0].PrincipalID = mustPID(t, "user-999") }, "match"},
		{"violated resolution with principal id", func(r *AuthorizationRequest) {
			r.Resolutions = append(r.Resolutions, PrincipalResolution{
				Claim: PrincipalClaim{TrustSource: "github", Subject: "user-123"}, State: ResolutionViolated, PrincipalID: mustPID(t, "user-123"),
			})
		}, "principal"},
		{"malformed resolution claim", func(r *AuthorizationRequest) { r.Resolutions[0].Claim.Subject = "" }, "subject"},
		{"conflicting duplicate resolutions", func(r *AuthorizationRequest) {
			r.Resolutions = append(r.Resolutions, failedRes(t, "user-456", ResolutionUnproven))
		}, "conflicting"},
		{"invalid approval role", func(r *AuthorizationRequest) { r.Approvals[0].Role = "Reviewer" }, "invalid id"},
		{"invalid approval principal", func(r *AuthorizationRequest) { r.Approvals[0].PrincipalID = "bare-name" }, "principal"},
		{"unknown rule-fact kind", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: "vibes", SourceID: "git-signature", PrincipalID: pRev, State: RuleFactProven, EvidenceDigest: testDigest}}
		}, "kind"},
		{"unknown rule-fact state", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: "sworn", EvidenceDigest: testDigest}}
		}, "state"},
		{"proven rule fact without digest", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: RuleFactProven}}
		}, "digest"},
		{"malformed rule-fact digest", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: RuleFactProven, EvidenceDigest: "sha1:abc"}}
		}, "digest"},
		{"violated rule fact without reason", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: RuleFactViolated, EvidenceDigest: testDigest}}
		}, "reason"},
		{"rule fact invalid source id", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "Git", PrincipalID: pRev, State: RuleFactUnproven, Reason: "x"}}
		}, "invalid id"},
		{"rule fact invalid principal id", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: "bare", State: RuleFactUnproven, Reason: "x"}}
		}, "principal"},
		{"conflicting rule facts", func(r *AuthorizationRequest) {
			r.RuleFacts = []RuleFact{
				{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: RuleFactProven, EvidenceDigest: testDigest},
				{Kind: RuleFactSignature, SourceID: "git-signature", PrincipalID: pRev, State: RuleFactViolated, EvidenceDigest: testDigest, Reason: "bad"},
			}
		}, "conflicting"},
		{"invalid evidence source id", func(r *AuthorizationRequest) { r.EvidenceSources = []string{"Merge-Gate"} }, "invalid id"},
		{"invalid escalation metric key", func(r *AuthorizationRequest) { r.EscalationMetrics = map[string]int{"Bad": 1} }, "invalid id"},
		{"negative escalation metric", func(r *AuthorizationRequest) { r.EscalationMetrics = map[string]int{"unresolved-exceptions": -1} }, "nonnegative"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := good()
			tt.mutate(&req)
			if d, err := Authorize(profile, req); err == nil {
				t.Fatalf("Authorize: expected operational error containing %q, got decision %+v", tt.wantSub, d)
			} else if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

// TestAuthorizeIgnoresAttribution: the request type carries no
// attribution field — attribution can never satisfy Authorize.
func TestAuthorizeIgnoresAttribution(t *testing.T) {
	typ := reflect.TypeOf(AuthorizationRequest{})
	attr := reflect.TypeOf(Attribution{})
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		if f.Type == attr || (f.Type.Kind() == reflect.Slice && f.Type.Elem() == attr) {
			t.Errorf("AuthorizationRequest field %s carries Attribution; attribution must never satisfy Authorize", f.Name)
		}
	}
}

// TestAuthorizeSoloCollapseOrdering: multiple collapse disclosures come
// back sorted by principal.
func TestAuthorizeSoloCollapseOrdering(t *testing.T) {
	yaml := `schema: verdi.governance-profile/v1
id: solo-pair
class: solo
applicable_transitions: [accept]
identity_trust_sources:
  - { id: github, kind: forge }
role_mappings:
  - role: author
    trust_source: github
    subjects: ["user-123", "user-456"]
  - role: reviewer
    trust_source: github
    subjects: ["user-123", "user-456"]
ownership_sources: []
signature_requirements: []
required_approvers: []
distinctness_rules: []
evidence_source_restrictions: []
escalation_thresholds: []
`
	solo := mustDecode(t, []byte(yaml))
	req := AuthorizationRequest{
		Transition:  "accept",
		Posture:     PostureAuthoritative,
		Resolutions: []PrincipalResolution{authedRes(t, "user-456"), authedRes(t, "user-123")},
		Approvals: []ApprovalRecord{
			{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
			{Role: "author", PrincipalID: mustPID(t, "user-456")},
			{Role: "reviewer", PrincipalID: mustPID(t, "user-123")},
			{Role: "author", PrincipalID: mustPID(t, "user-123")},
		},
	}
	d, err := Authorize(solo, req)
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	wantDecision(t, d, AuthorizationAuthorized)
	if len(d.Disclosures) != 2 {
		t.Fatalf("Disclosures = %+v, want two", d.Disclosures)
	}
	for i := 1; i < len(d.Disclosures); i++ {
		if d.Disclosures[i].PrincipalID < d.Disclosures[i-1].PrincipalID {
			t.Errorf("disclosures not sorted by principal: %+v", d.Disclosures)
		}
	}
}

// TestAuthorizeNonVacuousRules: an applicable actor rule never passes
// merely because its target role has no authenticated filler.
func TestAuthorizeNonVacuousRules(t *testing.T) {
	t.Run("team reviewers without an author cannot authorize", func(t *testing.T) {
		profile := authzProfile(t, nil)
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-456"), authedRes(t, "user-789")},
			Approvals: []ApprovalRecord{
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-789")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonDistinctnessUnproven)
	})

	t.Run("same-principal with one side unfilled is unproven", func(t *testing.T) {
		sameRule := `distinctness_rules:
  - transitions: [close, accept, merge-authorize]
    left_role: author
    right_role: reviewer
    relation: different-principal
  - transitions: [accept]
    left_role: author
    right_role: owner
    relation: same-principal
`
		profile := authzProfile(t, map[string]string{"distinctness_rules": sameRule})
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:  "accept",
			Posture:     PostureAuthoritative,
			Resolutions: []PrincipalResolution{authedRes(t, "user-123"), authedRes(t, "user-456")},
			Approvals: []ApprovalRecord{
				{Role: "author", PrincipalID: mustPID(t, "user-123")},
				{Role: "reviewer", PrincipalID: mustPID(t, "user-456")},
			},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonDistinctnessUnproven)
	})

	t.Run("both sides unfilled is unproven", func(t *testing.T) {
		profile := authzProfile(t, nil)
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:        "close",
			Posture:           PostureAuthoritative,
			EvidenceSources:   []string{"merge-gate"},
			EscalationMetrics: map[string]int{"unresolved-exceptions": 0},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven, ReasonDistinctnessUnproven, ReasonRequiredApproverMissing)
	})

	t.Run("high-assurance signature and ownership never pass with empty roles", func(t *testing.T) {
		profile := mustDecode(t, []byte(highAssuranceYAML))
		d, err := Authorize(profile, AuthorizationRequest{
			Transition:        "close",
			Posture:           PostureAuthoritative,
			EvidenceSources:   []string{"merge-gate"},
			EscalationMetrics: map[string]int{"unresolved-exceptions": 0},
		})
		if err != nil {
			t.Fatalf("Authorize: %v", err)
		}
		wantDecision(t, d, AuthorizationUnproven,
			ReasonSignatureUnproven, ReasonOwnershipUnproven,
			ReasonDistinctnessUnproven, ReasonRequiredApproverMissing)
	})
}
