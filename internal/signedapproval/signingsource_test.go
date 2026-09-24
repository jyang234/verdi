package signedapproval

import (
	"context"
	"strings"
	"testing"
)

// twoSignedProfileYAML adds a second signed-commit trust source to the solo
// profile. The forge's verification carries only an account id, never the
// source it belongs to, so account 1001 would satisfy a row under either.
var twoSignedProfileYAML = strings.NewReplacer(
	"  - {id: github-signed, kind: signed-commit}\n",
	"  - {id: github-signed, kind: signed-commit}\n  - {id: gitlab-signed, kind: signed-commit}\n",
	`  - {role: unsealed-exemption-escalation, trust_source: github-signed, subjects: ["1001"]}`+"\n",
	`  - {role: unsealed-exemption-escalation, trust_source: github-signed, subjects: ["1001"]}`+"\n"+
		`  - {role: unsealed-exemption-escalation, trust_source: gitlab-signed, subjects: ["1001"]}`+"\n",
).Replace(soloProfileYAML)

// noSignedProfileYAML is the solo profile without its signed-commit source:
// only the forge source remains.
var noSignedProfileYAML = strings.NewReplacer(
	"  - {id: github-signed, kind: signed-commit}\n", "",
	`  - {role: policy-owner, trust_source: github-signed, subjects: ["1001", "2002"]}`+"\n", "",
	`  - {role: unsealed-exemption-escalation, trust_source: github-signed, subjects: ["1001"]}`+"\n", "",
).Replace(soloProfileYAML)

// twoActs writes an owner row for owner and then an escalation row for
// escalation, each its own commit verified as account 1001, and returns
// the head and the verifier.
func twoActs(t *testing.T, r *testRepo, owner, escalation string) (string, *fakeVerifier) {
	t.Helper()
	c1 := approveOnce(r, row{ownerRole, owner})
	r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, owner}, row{escalationRole, escalation}))
	c2 := r.commit("approve the escalation")
	return c2, &fakeVerifier{facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001"), c2: verifiedBy(c2, "1001")}}
}

// TestAuthenticate_SignedCommitSources pins review F3 (SI-256): a profile
// that declares more than one signed-commit trust source leaves every row
// unproven, because a forge account id cannot name which source it belongs
// to; with none, a verified signer maps to no row's principal.
func TestAuthenticate_SignedCommitSources(t *testing.T) {
	t.Run("two signed-commit sources", func(t *testing.T) {
		// Account 1001 signs both acts. The escalation row names 1001
		// under the second source, which may be another person's account.
		r := newTestRepo(t)
		github, gitlab := principalFor(t, signedSource, "1001"), principalFor(t, "gitlab-signed", "1001")
		head, v := twoActs(t, r, github, gitlab)
		a, err := Authenticate(context.Background(), Input{Root: r.dir, Head: head, Path: artifactPath, Profile: decodeProfile(t, twoSignedProfileYAML), Verifier: v})
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if len(a.Rows) != 2 {
			t.Fatalf("rows = %+v, want 2", a.Rows)
		}
		for _, got := range a.Rows {
			if got.State != RowUnproven || got.Reason != ReasonSignedCommitSourceAmbiguous ||
				!strings.Contains(got.Detail, signedSource) || !strings.Contains(got.Detail, "gitlab-signed") {
				t.Fatalf("rows %s: row %+v, want unproven %s naming both sources", rowStates(a), got, ReasonSignedCommitSourceAmbiguous)
			}
		}
		if len(v.calls) != 0 {
			t.Fatalf("verifier calls = %v, want none: no answer could authenticate a row", v.calls)
		}
	})

	t.Run("no signed-commit source", func(t *testing.T) {
		r := newTestRepo(t)
		forge, signed := principalFor(t, forgeSource, "1001"), principalFor(t, signedSource, "1001")
		head, v := twoActs(t, r, forge, signed)
		a, err := Authenticate(context.Background(), Input{Root: r.dir, Head: head, Path: artifactPath, Profile: decodeProfile(t, noSignedProfileYAML), Verifier: v})
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if len(a.Rows) != 2 {
			t.Fatalf("rows = %+v, want 2", a.Rows)
		}
		for _, got := range a.Rows {
			if got.State != RowUnproven || got.Reason != ReasonSignerNotPrincipal || got.SignerAccountID != "1001" {
				t.Fatalf("row %+v, want unproven %s for the verified signer 1001", got, ReasonSignerNotPrincipal)
			}
		}
	})
}

// TestAuthorizationInputs_SignedCommitSourceAmbiguous: the same rule holds
// for the profile AuthorizationInputs is given, so rows authenticated under
// a one-source profile are dropped, with a disclosure, under a two-source
// one.
func TestAuthorizationInputs_SignedCommitSourceAmbiguous(t *testing.T) {
	a, p := ownerThenEscalation(t, true)
	in, err := a.AuthorizationInputs(context.Background(), decodeProfile(t, twoSignedProfileYAML))
	if err != nil {
		t.Fatalf("AuthorizationInputs: %v", err)
	}
	if len(in.Approvals) != 0 || len(in.Resolutions) != 0 || len(in.Disclosures) != 2 {
		t.Fatalf("inputs = %+v, want both rows dropped with a disclosure each", in)
	}
	for _, d := range in.Disclosures {
		if !strings.HasPrefix(d, ReasonSignedCommitSourceAmbiguous+": approval row (") || !strings.Contains(d, p) || !strings.Contains(d, "gitlab-signed") {
			t.Fatalf("disclosure %q: want %s for %s naming both sources", d, ReasonSignedCommitSourceAmbiguous, p)
		}
	}
}
