package signedapproval

import (
	"os/exec"
	"strings"
	"testing"
)

// staleRestore builds the review's F2 history: C (signed by 1001) adds the
// owner row, a branch "stale" is left at C, and W (signed by 1001)
// withdraws the row on main. restore then brings the row back without a
// new signature, and the returned head carries it. It returns C, W, and
// the head.
func staleRestore(t *testing.T, r *testRepo, p string, restore func(w string) string) (c, w, head string) {
	t.Helper()
	c = approveOnce(r, row{ownerRole, p})
	r.git("branch", "stale")
	r.write(artifactPath, exemptionDoc("", defaultBody))
	w = r.commit("withdraw the approval (signed by 1001)")
	head = restore(w)
	if r.git("merge-base", "--is-ancestor", w, head) != "" {
		t.Fatal("fixture: the head must descend from the withdrawal")
	}
	return c, w, head
}

// TestAuthenticate_WithdrawalScenarios pins review F2 (SI-256): the row
// must be present, unchanged, in every commit that changes the artifact on
// the full-history ancestry path from C to the head. Blame passes a row a
// merge restores from a stale branch back to C, so without that check a
// withdrawn approval came back without a new signature.
func TestAuthenticate_WithdrawalScenarios(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, r *testRepo) scenario
	}{
		{name: "withdrawn, then restored by an ours-strategy merge on the stale branch", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c, w, m := staleRestore(t, r, p, func(string) string {
				r.git("checkout", "-q", "stale")
				r.git("merge", "-q", "-s", "ours", "--no-edit", "-m", "unsigned merge keeping the stale row", "main")
				return r.git("rev-parse", "HEAD")
			})
			return scenario{head: m, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowWithdrawnAfterApproval, commit: c, detailSubstr: w}}}
		}},
		{name: "withdrawn, then restored by a merge that checks the artifact out from the stale branch", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c, w, m := staleRestore(t, r, p, func(string) string {
				r.git("checkout", "-q", "stale")
				r.write("other.txt", "stale side change\n")
				r.commit("unrelated change on stale")
				r.git("checkout", "-q", "main")
				r.git("merge", "-q", "--no-commit", "--no-ff", "stale")
				r.git("checkout", "stale", "--", artifactPath)
				r.git("commit", "-q", "--no-verify", "-m", "unsigned merge that keeps the row")
				return r.git("rev-parse", "HEAD")
			})
			return scenario{head: m, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowWithdrawnAfterApproval, commit: c, detailSubstr: w}}}
		}},
		{name: "artifact deleted, then restored by a merge from the stale branch", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			r.git("branch", "stale")
			r.git("rm", "-q", artifactPath)
			w := r.commit("delete the artifact")
			r.git("checkout", "-q", "stale")
			r.write("other.txt", "stale side change\n")
			r.commit("unrelated change on stale")
			r.git("checkout", "-q", "main")
			r.git("merge", "-q", "--no-commit", "--no-ff", "stale")
			r.git("checkout", "stale", "--", artifactPath)
			r.git("commit", "-q", "--no-verify", "-m", "unsigned merge that restores the artifact")
			m := r.git("rev-parse", "HEAD")
			return scenario{head: m, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowWithdrawnAfterApproval, commit: c, detailSubstr: w}}}
		}},
		{name: "another row added and removed after the approval", build: func(t *testing.T, r *testRepo) scenario {
			// Every later change to the artifact keeps the signed row, so
			// nothing withdrew it.
			p1, p2 := principalFor(t, signedSource, "1001"), principalFor(t, signedSource, "2002")
			c := approveOnce(r, row{ownerRole, p1})
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p1}, row{escalationRole, p2}))
			r.commit("unsigned: add a row")
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p1}))
			h := r.commit("unsigned: remove it again")
			return scenario{head: h, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p1, commit: c, signer: "1001"}}}
		}},
		{name: "a later copy that does not parse", build: func(t *testing.T, r *testRepo) scenario {
			// A copy the row cannot be read from does not show the row
			// present, even when a later commit restores the content.
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			r.write(artifactPath, strings.Replace(exemptionDoc("", defaultBody, row{ownerRole, p}), "expiry: \"2026-12-31\"", "expiry: [unclosed", 1))
			x := r.commit("unsigned: break the expiry")
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
			h := r.commit("unsigned: restore the expiry")
			return scenario{head: h, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowWithdrawnAfterApproval, commit: c, detailSubstr: x}}}
		}},
		{name: "a later copy with a nonstandard line break", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			r.write(artifactPath, strings.Replace(exemptionDoc("", defaultBody, row{ownerRole, p}), "expiry: \"2026-12-31\"\n", "expiry: \"2026-12-31\"\rnote: x\n", 1))
			x := r.commit("unsigned: add a key behind a lone CR")
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
			h := r.commit("unsigned: remove it")
			return scenario{head: h, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonNonstandardLineBreak, commit: c, detailSubstr: x}}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			checkScenario(t, r, tc.build(t, r))
		})
	}
}

// hiddenWithdrawal builds a history whose withdrawal sits on a side
// branch that is longer than the path to the approval: C (signed) adds the
// row; the side branch withdraws it (W) and adds three unrelated commits;
// main adds one unrelated commit; an unsigned ours-strategy merge on main
// keeps the row. A clone of depth 4 holds C and C's parent but not W, so
// blame still reaches C and not a boundary.
func hiddenWithdrawal(t *testing.T, r *testRepo, p string) (c, w, head string) {
	t.Helper()
	c = approveOnce(r, row{ownerRole, p})
	r.git("checkout", "-q", "-b", "side")
	r.write(artifactPath, exemptionDoc("", defaultBody))
	w = r.commit("withdraw the approval (signed by 1001)")
	for i := 1; i <= 3; i++ {
		r.write("side.txt", strings.Repeat("s", i)+"\n")
		r.commit("side change")
	}
	r.git("checkout", "-q", "main")
	r.write("other.txt", "main change\n")
	r.commit("main change")
	r.git("merge", "-q", "-s", "ours", "--no-edit", "-m", "unsigned merge that discards the withdrawal", "side")
	return c, w, r.git("rev-parse", "HEAD")
}

// TestAuthenticate_WithdrawalBeyondShallowBoundary: a shallow clone can
// hide a withdrawal on a side branch whose depth runs past the clone's
// boundary while C itself stays inside it. The ancestry path from C to the
// head cannot be shown complete there, so the row is unproven with
// history-boundary; the full clone finds the withdrawal.
func TestAuthenticate_WithdrawalBeyondShallowBoundary(t *testing.T) {
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	c, w, head := hiddenWithdrawal(t, r, p)
	shallow := r.shallowClone(4)

	// Fixture: the clone lacks W but blame at the head still reaches C,
	// which is not a boundary there.
	if err := exec.Command("git", "-C", shallow, "cat-file", "-e", w+"^{commit}").Run(); err == nil {
		t.Fatalf("fixture: the depth-4 clone must not hold the withdrawal %s", w)
	}
	facts := map[string]CommitVerification{c: verifiedBy(c, "1001")}
	t.Run("shallow clone", func(t *testing.T) {
		checkScenario(t, r, scenario{root: shallow, head: head, facts: facts,
			want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonHistoryBoundary, commit: c, detailSubstr: "shallow"}}})
	})
	t.Run("full clone", func(t *testing.T) {
		checkScenario(t, r, scenario{head: head, facts: facts,
			want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowWithdrawnAfterApproval, commit: c, detailSubstr: w}}})
	})
}
