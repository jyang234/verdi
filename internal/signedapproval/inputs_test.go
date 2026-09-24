package signedapproval

import (
	"context"
	"reflect"
	"strings"
	"testing"

	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// documentedPrincipalEvidence is the projection a principal's trust-fact
// evidence digest covers: its authenticated rows' evidence digests, sorted.
type documentedPrincipalEvidence struct {
	RowEvidenceDigests []string `json:"row_evidence_digests"`
}

// ownerThenEscalation writes an owner approval for 1001 and then an
// escalation approval for 1001, each its own commit, and returns the
// authenticated artifact at the head. escalationSigned chooses whether the
// forge verifies the escalation commit.
func ownerThenEscalation(t *testing.T, escalationSigned bool) (Artifact, string) {
	t.Helper()
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	c1 := approveOnce(r, row{ownerRole, p})
	r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}, row{escalationRole, p}))
	c2 := r.commit("approve the escalation")
	facts := map[string]CommitVerification{c1: verifiedBy(c1, "1001"), c2: unverified(c2)}
	if escalationSigned {
		facts[c2] = verifiedBy(c2, "1001")
	}
	a, err := Authenticate(context.Background(), Input{Root: r.dir, Head: c2, Path: artifactPath, Profile: soloProfile(t), Verifier: &fakeVerifier{facts: facts}})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	return a, p
}

func authorize(t *testing.T, profile gp.Profile, in AuthorizationInputs, uses int) gp.AuthorizationDecision {
	t.Helper()
	d, err := gp.Authorize(profile, gp.AuthorizationRequest{
		Transition: exemptionTarget, Posture: gp.PostureAuthoritative,
		Resolutions: in.Resolutions, Approvals: in.Approvals,
		EscalationMetrics: map[string]int{usesMetric: uses},
	})
	if err != nil {
		t.Fatalf("Authorize: %v", err)
	}
	return d
}

// TestAuthorizationInputs_ThroughAuthorize validates the inputs through the
// kernel's own interpretation: two authenticated acts authorize even past
// the escalation threshold, and dropping the unsigned escalation row makes
// the same request unproven there — dropping a row can only make
// authorization harder.
func TestAuthorizationInputs_ThroughAuthorize(t *testing.T) {
	ctx := context.Background()
	profile := soloProfile(t)

	t.Run("both acts authenticated", func(t *testing.T) {
		a, p := ownerThenEscalation(t, true)
		in, err := a.AuthorizationInputs(ctx, profile)
		if err != nil {
			t.Fatalf("AuthorizationInputs: %v", err)
		}
		d := authorize(t, profile, in, 2)
		if d.State != gp.AuthorizationAuthorized {
			t.Fatalf("decision = %+v, want authorized", d)
		}
		if len(d.Disclosures) != 1 || d.Disclosures[0].Code != gp.ReasonSoloRoleCollapse || d.Disclosures[0].PrincipalID != gp.PrincipalID(p) {
			t.Fatalf("disclosures = %+v, want the solo role collapse for %s", d.Disclosures, p)
		}
	})

	t.Run("unsigned escalation dropped", func(t *testing.T) {
		a, p := ownerThenEscalation(t, false)
		in, err := a.AuthorizationInputs(ctx, profile)
		if err != nil {
			t.Fatalf("AuthorizationInputs: %v", err)
		}
		wantApprovals := []gp.ApprovalRecord{{Role: ownerRole, PrincipalID: gp.PrincipalID(p)}}
		if !reflect.DeepEqual(in.Approvals, wantApprovals) {
			t.Fatalf("approvals = %+v, want only the authenticated owner row", in.Approvals)
		}
		if len(in.Resolutions) != 1 || in.Resolutions[0].PrincipalID != gp.PrincipalID(p) {
			t.Fatalf("resolutions = %+v, want one for %s", in.Resolutions, p)
		}
		var escalation Row
		for _, r := range a.Rows {
			if r.Role == escalationRole {
				escalation = r
			}
		}
		want := []string{ReasonSignatureUnverified + ": approval row (" + escalationRole + ", " + p + ") of " + artifactPath + " at " + a.Head + ": " + escalation.Detail}
		if !reflect.DeepEqual(in.Disclosures, want) {
			t.Fatalf("disclosures = %q, want %q", in.Disclosures, want)
		}
		if d := authorize(t, profile, in, 1); d.State != gp.AuthorizationAuthorized {
			t.Fatalf("below the threshold: decision = %+v, want authorized", d)
		}
		d := authorize(t, profile, in, 2)
		if d.State != gp.AuthorizationUnproven || len(d.Findings) != 1 || d.Findings[0].Code != gp.ReasonEscalationRoleMissing {
			t.Fatalf("at the threshold: decision = %+v, want unproven for the missing escalation role", d)
		}
	})
}

// TestAuthorizationInputs_KernelRefusesPrincipal: when the kernel does not
// authenticate the signer claim under the profile it is given, that
// principal's rows are dropped with a disclosure, never passed through.
func TestAuthorizationInputs_KernelRefusesPrincipal(t *testing.T) {
	ctx := context.Background()
	base := strings.Replace(soloProfileYAML, `  - {role: unsealed-exemption-escalation, trust_source: github-signed, subjects: ["1001"]}
`, "", 1)
	tests := []struct {
		name, profile, wantState string
	}{
		{
			name:      "source is not a signed-commit source",
			profile:   strings.Replace(base, "{id: github-signed, kind: signed-commit}", "{id: github-signed, kind: identity-provider}", 1),
			wantState: string(gp.ResolutionUnproven),
		},
		{
			name: "source is absent from the profile",
			profile: strings.NewReplacer(
				"  - {id: github-signed, kind: signed-commit}\n", "",
				`  - {role: policy-owner, trust_source: github-signed, subjects: ["1001", "2002"]}`+"\n", "",
			).Replace(base),
			wantState: string(gp.ResolutionViolated),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			a, err := Authenticate(ctx, Input{Root: r.dir, Head: c, Path: artifactPath, Profile: soloProfile(t),
				Verifier: &fakeVerifier{facts: map[string]CommitVerification{c: verifiedBy(c, "1001")}}})
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if a.Rows[0].State != RowAuthenticated {
				t.Fatalf("fixture: row %+v, want authenticated", a.Rows[0])
			}
			in, err := a.AuthorizationInputs(ctx, decodeProfile(t, tc.profile))
			if err != nil {
				t.Fatalf("AuthorizationInputs: %v", err)
			}
			if len(in.Approvals) != 0 || len(in.Resolutions) != 0 || len(in.Disclosures) != 1 {
				t.Fatalf("inputs = %+v, want the row dropped with one disclosure", in)
			}
			d := in.Disclosures[0]
			if !strings.HasPrefix(d, DisclosurePrincipalUnauthenticated+": approval row ("+ownerRole+", "+p+")") || !strings.Contains(d, tc.wantState) {
				t.Fatalf("disclosure %q: want %s naming the %s resolution", d, DisclosurePrincipalUnauthenticated, tc.wantState)
			}
		})
	}
}

// TestAuthorizationInputs_ScopedToOneArtifact: one principal's signed
// approval of artifact A never fills the same principal's unsigned row in
// artifact B, and A's trust fact covers A's rows only.
func TestAuthorizationInputs_ScopedToOneArtifact(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	pathA, pathB := ".verdi/policy/exemptions/a.md", ".verdi/policy/exemptions/b.md"
	r.write(pathA, exemptionDoc("", defaultBody))
	r.write(pathB, exemptionDoc("", defaultBody))
	r.commit("draft both")
	r.write(pathA, exemptionDoc("", defaultBody, row{ownerRole, p}))
	c1 := r.commit("approve A")
	r.write(pathB, exemptionDoc("", defaultBody, row{ownerRole, p}))
	c2 := r.commit("approve B")
	v := &fakeVerifier{facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001"), c2: unverified(c2)}}
	profile := soloProfile(t)

	artifactA, err := Authenticate(ctx, Input{Root: r.dir, Head: c2, Path: pathA, Profile: profile, Verifier: v})
	if err != nil {
		t.Fatalf("Authenticate A: %v", err)
	}
	artifactB, err := Authenticate(ctx, Input{Root: r.dir, Head: c2, Path: pathB, Profile: profile, Verifier: v})
	if err != nil {
		t.Fatalf("Authenticate B: %v", err)
	}
	inA, err := artifactA.AuthorizationInputs(ctx, profile)
	if err != nil {
		t.Fatalf("AuthorizationInputs A: %v", err)
	}
	inB, err := artifactB.AuthorizationInputs(ctx, profile)
	if err != nil {
		t.Fatalf("AuthorizationInputs B: %v", err)
	}

	if len(inA.Approvals) != 1 || len(inA.Resolutions) != 1 || len(inA.Disclosures) != 0 {
		t.Fatalf("A inputs = %+v, want its one authenticated approval", inA)
	}
	if len(inB.Approvals) != 0 || len(inB.Resolutions) != 0 || len(inB.Disclosures) != 1 || !strings.HasPrefix(inB.Disclosures[0], ReasonSignatureUnverified+":") {
		t.Fatalf("B inputs = %+v, want no approval and one signature-unverified disclosure", inB)
	}
	if d := authorize(t, profile, inB, 0); d.State != gp.AuthorizationUnproven {
		t.Fatalf("B decision = %+v, want unproven: A's approval must not fill B", d)
	}
	wantEvidence := mustDigest(t, documentedPrincipalEvidence{RowEvidenceDigests: []string{artifactA.Rows[0].EvidenceDigest}})
	w := inA.Resolutions[0].Witnesses
	if len(w) != 1 || w[0].EvidenceDigest != wantEvidence || w[0].SourceID != signedSource {
		t.Fatalf("A resolution witnesses = %+v, want evidence %s from %s", w, wantEvidence, signedSource)
	}
}

// TestAuthorizationInputs_RefusesForgedArtifacts: only unmodified
// Authenticate output becomes kernel input.
func TestAuthorizationInputs_RefusesForgedArtifacts(t *testing.T) {
	ctx := context.Background()
	profile := soloProfile(t)
	a, p := ownerThenEscalation(t, false)

	copyOf := func(a Artifact) Artifact {
		b := a
		b.Rows = append([]Row(nil), a.Rows...)
		return b
	}
	if _, err := copyOf(a).AuthorizationInputs(ctx, profile); err != nil {
		t.Fatalf("a faithful copy: AuthorizationInputs: %v", err)
	}

	promoted := copyOf(a)
	for i := range promoted.Rows {
		promoted.Rows[i].State, promoted.Rows[i].Reason, promoted.Rows[i].Detail = RowAuthenticated, "", ""
	}
	retargeted := copyOf(a)
	retargeted.Path = ".verdi/policy/exemptions/other.md"

	tests := []struct {
		name, wantSubstr string
		a                Artifact
		profile          gp.Profile
	}{
		{name: "built by hand", wantSubstr: "not produced by Authenticate", profile: profile, a: Artifact{Path: artifactPath, Head: a.Head, Rows: []Row{{
			Role: ownerRole, Principal: p, State: RowAuthenticated, Commit: a.Head, SignerAccountID: "1001", TrustSourceID: signedSource,
			EvidenceDigest: "sha256:" + strings.Repeat("0", 64),
		}}}},
		{name: "unproven row promoted", wantSubstr: "modified after Authenticate", profile: profile, a: promoted},
		{name: "path retargeted", wantSubstr: "modified after Authenticate", profile: profile, a: retargeted},
		{name: "profile not from DecodeProfile", wantSubstr: "DecodeProfile", profile: gp.Profile{ID: "x"}, a: a},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in, err := tc.a.AuthorizationInputs(ctx, tc.profile)
			if err == nil {
				t.Fatalf("AuthorizationInputs: want error, got %+v", in)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}
