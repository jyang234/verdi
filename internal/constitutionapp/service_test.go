package constitutionapp

import (
	"context"
	"os"
	"strings"
	"testing"
)

// TestProposeToImpactReviewFlow is the Wave 6 Task 3 semantic RED this
// package existed to make pass: propose a Git-backed constitution change on
// an explicit branch, then run an impact review over it, proving the whole
// propose -> impact-review application flow — previously entirely
// unimplemented (design §2.2: "no application operation owns propose,
// validate, impact-review, and submit preparation") — now composes end to
// end without merging, approving, or inventing conflict semantics.
func TestProposeToImpactReviewFlow(t *testing.T) {
	root := buildFixtureRepo(t)
	svc := testService()
	ctx := context.Background()

	proposed := strings.Replace(readFixtureFile(t, root, "policies/go-toolchain.md"),
		`instructions:
  - "Run make verify before claiming completion."`,
		`instructions:
  - "Run make verify before claiming completion."
  - "Announce toolchain changes in the platform channel."`,
		1)

	propose, typed := svc.Propose(ctx, root, ProposeRequest{
		Branch:   "policy/widen-go-versions",
		Kind:     KindPolicy,
		Name:     "go-toolchain",
		Content:  []byte(proposed),
		Expected: Expected{Branch: "policy/widen-go-versions"},
	})
	if typed != nil {
		t.Fatalf("Propose: %v", typed)
	}
	if propose.ZeroEffect {
		t.Fatal("Propose: expected a real effect, got zero-effect")
	}
	if propose.Commit == "" {
		t.Fatal("Propose: expected a commit SHA")
	}

	review, typed := svc.ImpactReview(ctx, root, ImpactReviewRequest{})
	if typed != nil {
		t.Fatalf("ImpactReview: %v", typed)
	}
	if !review.Accepted.Adopted || !review.Proposed.Adopted {
		t.Fatalf("ImpactReview: expected both accepted and proposed to be adopted, got accepted=%v proposed=%v", review.Accepted.Adopted, review.Proposed.Adopted)
	}
	if review.Accepted.ConstitutionDigest == "" || review.Proposed.ConstitutionDigest == "" {
		t.Fatal("ImpactReview: expected non-empty constitution digests on both sides")
	}
	found := false
	for _, l := range review.Layers {
		if l.Kind == KindPolicy && l.ID == "policy/go-toolchain" && l.Change == "changed" {
			found = true
			if l.AcceptedDigest == "" || l.ProposedDigest == "" || l.AcceptedDigest == l.ProposedDigest {
				t.Fatalf("ImpactReview: expected distinct accepted/proposed digests, got %+v", l)
			}
		}
	}
	if !found {
		t.Fatalf("ImpactReview: expected a changed policy/go-toolchain layer, got %+v", review.Layers)
	}

	prep, typed := svc.SubmitPreparation(ctx, root, SubmitPreparationRequest{})
	if typed != nil {
		t.Fatalf("SubmitPreparation: %v", typed)
	}
	if !prep.ReadyForSubmission {
		t.Fatalf("SubmitPreparation: expected ready for submission, got blocking reasons %v", prep.BlockingReasons)
	}
}

func readFixtureFile(t testing.TB, root, rel string) string {
	t.Helper()
	data, err := os.ReadFile(root + "/.verdi/policy/" + rel)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
