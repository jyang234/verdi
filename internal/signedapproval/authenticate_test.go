package signedapproval

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/canonjson"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// documentedEvidence is the row evidence projection the contract names:
// path, head, role, principal, commit, signer, trust source id, and
// provider snapshot id.
type documentedEvidence struct {
	Path               string `json:"path"`
	Head               string `json:"head"`
	Role               string `json:"role"`
	Principal          string `json:"principal"`
	Commit             string `json:"commit"`
	SignerAccountID    string `json:"signer_account_id"`
	TrustSourceID      string `json:"trust_source_id"`
	ProviderSnapshotID string `json:"provider_snapshot_id"`
}

func mustDigest(t *testing.T, v any) string {
	t.Helper()
	d, err := canonjson.Digest(v)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestAuthenticate_MergedBranchApproval: the approval row is introduced by
// its own commit on a feature branch and merged with --no-ff; blame passes
// the row through the merge, so the introducing commit is the branch
// commit, never the merge.
func TestAuthenticate_MergedBranchApproval(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	r.write(artifactPath, exemptionDoc("", defaultBody))
	r.write("other.txt", "one\n")
	r.commit("draft the exemption")
	r.git("checkout", "-q", "-b", "approve")
	r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
	c := r.commit("approve the exemption")
	r.git("checkout", "-q", "main")
	r.write("other.txt", "two\n")
	r.commit("unrelated change on main")
	r.git("merge", "-q", "--no-ff", "--no-edit", "-m", "merge the approval", "approve")
	head := r.git("rev-parse", "HEAD")
	if head == c {
		t.Fatal("fixture: the merge must be a distinct commit")
	}

	v := &fakeVerifier{facts: map[string]CommitVerification{c: verifiedBy(c, "1001")}}
	got, err := Authenticate(ctx, Input{Root: r.dir, Head: head, Path: artifactPath, Profile: soloProfile(t), Verifier: v})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	want := Row{
		Role: ownerRole, Principal: p, State: RowAuthenticated, Commit: c,
		SignerAccountID: "1001", TrustSourceID: signedSource,
		EvidenceDigest: mustDigest(t, documentedEvidence{
			Path: artifactPath, Head: head, Role: ownerRole, Principal: p, Commit: c,
			SignerAccountID: "1001", TrustSourceID: signedSource, ProviderSnapshotID: snapshotFor(c),
		}),
	}
	if got.Path != artifactPath || got.Head != head {
		t.Fatalf("artifact identity = %q@%q, want %q@%q", got.Path, got.Head, artifactPath, head)
	}
	if !reflect.DeepEqual(got.Rows, []Row{want}) {
		t.Fatalf("rows =\n%+v\nwant\n%+v", got.Rows, []Row{want})
	}
	if !reflect.DeepEqual(v.calls, []string{c}) {
		t.Fatalf("verifier calls = %v, want exactly the branch commit %s", v.calls, c)
	}
}

// TestAuthenticate_SamePrincipalTwoActs: an exemption approval and an
// escalation approval by one principal, each its own signed commit, are
// both authenticated, and the kernel inputs carry ONE resolution for that
// principal.
func TestAuthenticate_SamePrincipalTwoActs(t *testing.T) {
	ctx := context.Background()
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	r.write(artifactPath, exemptionDoc("", defaultBody))
	r.commit("draft")
	r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
	c1 := r.commit("approve")
	r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}, row{escalationRole, p}))
	c2 := r.commit("approve the escalation")

	v := &fakeVerifier{facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001"), c2: verifiedBy(c2, "1001")}}
	profile := soloProfile(t)
	a, err := Authenticate(ctx, Input{Root: r.dir, Head: c2, Path: artifactPath, Profile: profile, Verifier: v})
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if len(a.Rows) != 2 {
		t.Fatalf("rows = %+v, want 2", a.Rows)
	}
	for i, want := range []struct{ role, commit string }{{ownerRole, c1}, {escalationRole, c2}} {
		got := a.Rows[i]
		if got.Role != want.role || got.State != RowAuthenticated || got.Commit != want.commit || got.Reason != "" {
			t.Fatalf("rows[%d] = %+v, want %s authenticated by %s", i, got, want.role, want.commit)
		}
	}

	in, err := a.AuthorizationInputs(ctx, profile)
	if err != nil {
		t.Fatalf("AuthorizationInputs: %v", err)
	}
	if len(in.Resolutions) != 1 || in.Resolutions[0].PrincipalID != gp.PrincipalID(p) || in.Resolutions[0].State != gp.ResolutionAuthenticated {
		t.Fatalf("resolutions = %+v, want one authenticated resolution for %s", in.Resolutions, p)
	}
	wantApprovals := []gp.ApprovalRecord{{Role: ownerRole, PrincipalID: gp.PrincipalID(p)}, {Role: escalationRole, PrincipalID: gp.PrincipalID(p)}}
	if !reflect.DeepEqual(in.Approvals, wantApprovals) {
		t.Fatalf("approvals = %+v, want %+v", in.Approvals, wantApprovals)
	}
	if len(in.Disclosures) != 0 {
		t.Fatalf("disclosures = %v, want none", in.Disclosures)
	}
}

// wantRow is one expected row, keyed by role in a scenario.
type wantRow struct {
	principal    string
	reason       string // "" means authenticated
	commit       string
	signer       string
	detailSubstr string
}

// scenario is one built history and its expected rows.
type scenario struct {
	root, head, path string
	facts            map[string]CommitVerification
	want             map[string]wantRow
}

func TestAuthenticate_Scenarios(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, r *testRepo) scenario
	}{
		{name: "unsigned approval commit", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			return scenario{head: c, facts: map[string]CommitVerification{c: unverified(c)},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonSignatureUnverified, commit: c}}}
		}},
		{name: "verification unavailable", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			return scenario{head: c, facts: map[string]CommitVerification{c: unavailable(c, "forge unreachable")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonVerificationUnavailable, commit: c, detailSubstr: "forge unreachable"}}}
		}},
		{name: "signer is another account", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			return scenario{head: c, facts: map[string]CommitVerification{c: verifiedBy(c, "2002")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonSignerNotPrincipal, commit: c, signer: "2002"}}}
		}},
		{name: "principal under a forge source", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, forgeSource, "1001")
			c := approveOnce(r, row{ownerRole, p})
			return scenario{head: c, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonSignerNotPrincipal, commit: c, signer: "1001"}}}
		}},
		{name: "two rows added in one commit", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c := approveOnce(r, row{ownerRole, p}, row{escalationRole, p})
			return scenario{head: c, facts: map[string]CommitVerification{},
				want: map[string]wantRow{
					ownerRole:      {principal: p, reason: ReasonApprovalCommitShared, commit: c},
					escalationRole: {principal: p, reason: ReasonApprovalCommitShared, commit: c},
				}}
		}},
		{name: "approval commit also edits another row", build: func(t *testing.T, r *testRepo) scenario {
			p1, p2 := principalFor(t, signedSource, "1001"), principalFor(t, signedSource, "2002")
			approveOnce(r, row{ownerRole, p2})
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p1}, row{escalationRole, p1}))
			c2 := r.commit("approve the escalation and rewrite the owner row")
			return scenario{head: c2, facts: map[string]CommitVerification{},
				want: map[string]wantRow{
					ownerRole:      {principal: p1, reason: ReasonRowLinesSplit},
					escalationRole: {principal: p1, reason: ReasonApprovalCommitShared, commit: c2},
				}}
		}},
		{name: "flow-style rows on one line", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			r.write(artifactPath, exemptionDoc("", defaultBody))
			r.commit("draft")
			doc := strings.Replace(exemptionDoc("", defaultBody), "approvals:\n",
				"approvals: [{role: "+ownerRole+", principal: "+p+"}, {role: "+escalationRole+", principal: "+p+"}]\n", 1)
			r.write(artifactPath, doc)
			c := r.commit("approve in flow style")
			return scenario{head: c, facts: map[string]CommitVerification{},
				want: map[string]wantRow{
					ownerRole:      {principal: p, reason: ReasonRowLinesShared, commit: c},
					escalationRole: {principal: p, reason: ReasonRowLinesShared, commit: c},
				}}
		}},
		{name: "role and principal lines added by different commits", build: func(t *testing.T, r *testRepo) scenario {
			p1, p2 := principalFor(t, signedSource, "1001"), principalFor(t, signedSource, "2002")
			c1 := approveOnce(r, row{ownerRole, p2})
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p1}))
			c2 := r.commit("repoint the principal")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "2002"), c2: verifiedBy(c2, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p1, reason: ReasonRowLinesSplit}}}
		}},
		{name: "body changed after the approval", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c1 := approveOnce(r, row{ownerRole, p})
			r.write(artifactPath, exemptionDoc("", "A different body.\n", row{ownerRole, p}))
			c2 := r.commit("edit the body")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonArtifactChangedAfterApproval, commit: c1}}}
		}},
		{name: "frontmatter changed after the approval", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			r.write(artifactPath, exemptionDoc("title: first\n", defaultBody))
			r.commit("draft")
			r.write(artifactPath, exemptionDoc("title: first\n", defaultBody, row{ownerRole, p}))
			c1 := r.commit("approve")
			r.write(artifactPath, exemptionDoc("title: second\n", defaultBody, row{ownerRole, p}))
			c2 := r.commit("retitle")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonArtifactChangedAfterApproval, commit: c1}}}
		}},
		{name: "withdrawn by a signed commit then re-added unsigned", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c1 := approveOnce(r, row{ownerRole, p})
			r.write(artifactPath, exemptionDoc("", defaultBody))
			c2 := r.commit("withdraw the approval")
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
			c3 := r.commit("re-add the approval")
			return scenario{head: c3, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001"), c2: verifiedBy(c2, "1001"), c3: unverified(c3)},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonSignatureUnverified, commit: c3}}}
		}},
		{name: "full clone control for the shallow case", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c1 := approveOnce(r, row{ownerRole, p})
			r.write("other.txt", "later\n")
			c2 := r.commit("unrelated")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, commit: c1, signer: "1001"}}}
		}},
		{name: "shallow clone whose boundary owns the row", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			c1 := approveOnce(r, row{ownerRole, p})
			r.write("other.txt", "later\n")
			c2 := r.commit("unrelated")
			return scenario{root: r.shallowClone(2), head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonHistoryBoundary, detailSubstr: c1}}}
		}},
		{name: "row introduced in the root commit", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			r.write(artifactPath, exemptionDoc("", defaultBody, row{ownerRole, p}))
			c := r.commit("draft with the approval")
			return scenario{head: c, facts: map[string]CommitVerification{c: verifiedBy(c, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonHistoryBoundary}}}
		}},
		{name: "artifact renamed after the approval", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			old := ".verdi/policy/exemptions/old.md"
			r.write(old, exemptionDoc("", defaultBody))
			r.write("other.txt", "base\n")
			r.commit("draft")
			r.write(old, exemptionDoc("", defaultBody, row{ownerRole, p}))
			c1 := r.commit("approve")
			r.git("mv", old, artifactPath)
			c2 := r.commit("rename")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonArtifactPathChanged, commit: c1, detailSubstr: old}}}
		}},
		{name: "body text spliced into the approvals", build: func(t *testing.T, r *testRepo) scenario {
			// The signed commit carries no approval, only body text that
			// reads like one; an unsigned commit then moves the delimiter so
			// those lines land inside approvals, and re-adds the body text.
			// Blame and the content binding alone would authenticate it.
			p := principalFor(t, signedSource, "1001")
			rowText := "  - role: " + ownerRole + "\n    principal: " + p + "\n"
			r.write("other.txt", "base\n")
			r.commit("base")
			r.write(artifactPath, "---\nschema: test/v1\napprovals:\n---\nExample:\n"+rowText)
			c1 := r.commit("draft with an example")
			r.write(artifactPath, "---\nschema: test/v1\napprovals:\n"+rowText+"---\n")
			r.commit("move the delimiter")
			r.write(artifactPath, "---\nschema: test/v1\napprovals:\n"+rowText+"---\nExample:\n"+rowText)
			c3 := r.commit("restore the example")
			return scenario{head: c3, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowAbsentAtApprovalCommit, commit: c1}}}
		}},
		{name: "rows spliced across two approvals", build: func(t *testing.T, r *testRepo) scenario {
			// One commit signed by 1001 approves the escalation for 1001
			// and writes an owner row for 2002; an unsigned commit deletes
			// the middle two lines, leaving an owner row for 1001 whose
			// lines all blame to that one signed commit.
			p1, p2 := principalFor(t, signedSource, "1001"), principalFor(t, signedSource, "2002")
			c1 := approveOnce(r, row{ownerRole, p2}, row{escalationRole, p1})
			spliced := strings.Replace(exemptionDoc("", defaultBody, row{ownerRole, p2}, row{escalationRole, p1}),
				"    principal: "+p2+"\n  - role: "+escalationRole+"\n", "", 1)
			r.write(artifactPath, spliced)
			c2 := r.commit("delete two lines")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p1, reason: ReasonRowAbsentAtApprovalCommit, commit: c1}}}
		}},
		{name: "approval commit copy of the artifact does not parse", build: func(t *testing.T, r *testRepo) scenario {
			p := principalFor(t, signedSource, "1001")
			r.write(artifactPath, exemptionDoc("title: [unclosed\n", defaultBody))
			r.commit("draft")
			r.write(artifactPath, exemptionDoc("title: [unclosed\n", defaultBody, row{ownerRole, p}))
			c1 := r.commit("approve")
			r.write(artifactPath, exemptionDoc("title: fixed\n", defaultBody, row{ownerRole, p}))
			c2 := r.commit("fix the title")
			return scenario{head: c2, facts: map[string]CommitVerification{c1: verifiedBy(c1, "1001")},
				want: map[string]wantRow{ownerRole: {principal: p, reason: ReasonRowAbsentAtApprovalCommit, commit: c1, detailSubstr: "yaml"}}}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			sc := tc.build(t, r)
			root, path := sc.root, sc.path
			if root == "" {
				root = r.dir
			}
			if path == "" {
				path = artifactPath
			}
			v := &fakeVerifier{facts: sc.facts}
			a, err := Authenticate(context.Background(), Input{Root: root, Head: sc.head, Path: path, Profile: soloProfile(t), Verifier: v})
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if len(a.Rows) != len(sc.want) {
				t.Fatalf("rows = %+v, want %d rows", a.Rows, len(sc.want))
			}
			for _, got := range a.Rows {
				want, ok := sc.want[got.Role]
				if !ok {
					t.Fatalf("unexpected row %+v", got)
				}
				assertRow(t, got, want)
			}
		})
	}
}

// approveOnce writes a draft without approvals (with a sibling file, so
// the root commit never owns an approval line), then adds rows in one
// commit, returning that commit.
func approveOnce(r *testRepo, rows ...row) string {
	r.write(artifactPath, exemptionDoc("", defaultBody))
	r.write("other.txt", "base\n")
	r.commit("draft")
	r.write(artifactPath, exemptionDoc("", defaultBody, rows...))
	return r.commit("approve")
}

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func assertRow(t *testing.T, got Row, want wantRow) {
	t.Helper()
	if got.Principal != want.principal {
		t.Fatalf("row %s principal = %q, want %q", got.Role, got.Principal, want.principal)
	}
	if got.Commit != want.commit {
		t.Fatalf("row %s commit = %q, want %q (row %+v)", got.Role, got.Commit, want.commit, got)
	}
	if got.SignerAccountID != want.signer {
		t.Fatalf("row %s signer = %q, want %q (row %+v)", got.Role, got.SignerAccountID, want.signer, got)
	}
	if want.reason == "" {
		if got.State != RowAuthenticated || got.Reason != "" || got.Detail != "" || got.TrustSourceID != signedSource || !digestRe.MatchString(got.EvidenceDigest) {
			t.Fatalf("row %s = %+v, want authenticated under %s with an evidence digest", got.Role, got, signedSource)
		}
		return
	}
	if got.State != RowUnproven || got.Reason != want.reason {
		t.Fatalf("row %s = %+v, want unproven %s", got.Role, got, want.reason)
	}
	if got.Detail == "" || got.TrustSourceID != "" || got.EvidenceDigest != "" {
		t.Fatalf("unproven row %s = %+v: want a detail and no trust source or evidence digest", got.Role, got)
	}
	if want.detailSubstr != "" && !strings.Contains(got.Detail, want.detailSubstr) {
		t.Fatalf("row %s detail %q does not mention %q", got.Role, got.Detail, want.detailSubstr)
	}
}

func TestAuthenticate_NoApprovals(t *testing.T) {
	tests := []struct {
		name, doc string
	}{
		{name: "absent key", doc: "---\nschema: test/v1\n---\nbody\n"},
		{name: "null sequence", doc: exemptionDoc("", defaultBody)},
		{name: "empty sequence", doc: "---\nschema: test/v1\napprovals: []\n---\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRepo(t)
			r.write(artifactPath, tc.doc)
			head := r.commit("draft")
			v := &fakeVerifier{}
			a, err := Authenticate(context.Background(), Input{Root: r.dir, Head: head, Path: artifactPath, Profile: soloProfile(t), Verifier: v})
			if err != nil {
				t.Fatalf("Authenticate: %v", err)
			}
			if a.Path != artifactPath || a.Head != head || len(a.Rows) != 0 || len(v.calls) != 0 {
				t.Fatalf("artifact = %+v, calls %v: want no rows and no verifier calls", a, v.calls)
			}
			in, err := a.AuthorizationInputs(context.Background(), soloProfile(t))
			if err != nil {
				t.Fatalf("AuthorizationInputs: %v", err)
			}
			if len(in.Approvals)+len(in.Resolutions)+len(in.Disclosures) != 0 {
				t.Fatalf("inputs = %+v, want empty", in)
			}
		})
	}
}

// errVerifier returns one fixed verification for every commit.
type fixedVerifier struct{ v CommitVerification }

func (f fixedVerifier) VerifyCommit(context.Context, string) (CommitVerification, error) {
	return f.v, nil
}

func TestAuthenticate_Operational(t *testing.T) {
	r := newTestRepo(t)
	p := principalFor(t, signedSource, "1001")
	c := approveOnce(r, row{ownerRole, p})
	profile := soloProfile(t)
	good := func() Input {
		return Input{Root: r.dir, Head: c, Path: artifactPath, Profile: profile,
			Verifier: &fakeVerifier{facts: map[string]CommitVerification{c: verifiedBy(c, "1001")}}}
	}
	if _, err := Authenticate(context.Background(), good()); err != nil {
		t.Fatalf("control: Authenticate: %v", err)
	}

	commitDoc := func(doc string) string {
		r.write(".verdi/policy/exemptions/bad.md", doc)
		return r.commit("write " + doc[:min(len(doc), 20)])
	}
	badPath := ".verdi/policy/exemptions/bad.md"
	withDoc := func(doc string) Input {
		in := good()
		in.Head = commitDoc(doc)
		in.Path = badPath
		return in
	}
	rowsDoc := func(rows string) string {
		return "---\nschema: test/v1\napprovals:\n" + rows + "---\n"
	}

	tests := []struct {
		name, wantSubstr string
		in               func() Input
	}{
		{name: "empty root", wantSubstr: "root", in: func() Input { in := good(); in.Root = ""; return in }},
		{name: "short head", wantSubstr: "head", in: func() Input { in := good(); in.Head = c[:12]; return in }},
		{name: "uppercase head", wantSubstr: "head", in: func() Input { in := good(); in.Head = strings.ToUpper(c); return in }},
		{name: "absolute path", wantSubstr: "path", in: func() Input { in := good(); in.Path = "/" + artifactPath; return in }},
		{name: "dot-dot path", wantSubstr: "path", in: func() Input { in := good(); in.Path = "../x.md"; return in }},
		{name: "unclean path", wantSubstr: "path", in: func() Input { in := good(); in.Path = ".verdi//x.md"; return in }},
		{name: "empty path", wantSubstr: "path", in: func() Input { in := good(); in.Path = ""; return in }},
		{name: "backslash path", wantSubstr: "path", in: func() Input { in := good(); in.Path = `a\b.md`; return in }},
		{name: "nil verifier", wantSubstr: "verifier", in: func() Input { in := good(); in.Verifier = nil; return in }},
		{name: "profile not from DecodeProfile", wantSubstr: "DecodeProfile", in: func() Input { in := good(); in.Profile = gp.Profile{ID: "x"}; return in }},
		{name: "path missing at head", wantSubstr: "missing.md", in: func() Input { in := good(); in.Path = ".verdi/policy/exemptions/missing.md"; return in }},
		{name: "no frontmatter", wantSubstr: "frontmatter", in: func() Input { return withDoc("just text\n") }},
		{name: "malformed yaml", wantSubstr: "yaml", in: func() Input { return withDoc("---\napprovals: [\n---\n") }},
		{name: "row without principal", wantSubstr: "role and principal", in: func() Input { return withDoc(rowsDoc("  - role: policy-owner\n")) }},
		{name: "row with an extra key", wantSubstr: "role and principal", in: func() Input {
			return withDoc(rowsDoc("  - role: policy-owner\n    principal: " + p + "\n    note: x\n"))
		}},
		{name: "row with an invalid role", wantSubstr: "role", in: func() Input {
			return withDoc(rowsDoc("  - role: Policy-Owner\n    principal: " + p + "\n"))
		}},
		{name: "row with an invalid principal", wantSubstr: "principal", in: func() Input {
			return withDoc(rowsDoc("  - role: policy-owner\n    principal: alice\n"))
		}},
		{name: "duplicate row", wantSubstr: "duplicate", in: func() Input {
			return withDoc(rowsDoc("  - role: policy-owner\n    principal: " + p + "\n  - role: policy-owner\n    principal: " + p + "\n"))
		}},
		{name: "verifier error", wantSubstr: "forge down", in: func() Input {
			in := good()
			in.Verifier = &fakeVerifier{err: errors.New("forge down")}
			return in
		}},
		{name: "verifier breaks the port contract", wantSubstr: "snapshot", in: func() Input {
			in := good()
			bad := verifiedBy(c, "1001")
			bad.ProviderSnapshotID = ""
			in.Verifier = fixedVerifier{v: bad}
			return in
		}},
		{name: "verifier answers about another commit", wantSubstr: "another commit", in: func() Input {
			in := good()
			in.Verifier = fixedVerifier{v: verifiedBy(strings.Repeat("e", 40), "1001")}
			return in
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, err := Authenticate(context.Background(), tc.in())
			if err == nil {
				t.Fatalf("Authenticate: want error, got %+v", a)
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("error %q does not mention %q", err, tc.wantSubstr)
			}
		})
	}
}
