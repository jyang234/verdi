package signedapproval

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/gitx"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// Input names one artifact at one head and the authorities that judge it.
type Input struct {
	// Root is the repository root (a git work tree).
	Root string
	// Head is H, the head being evaluated: a full lowercase object id.
	Head string
	// Path is the repo-relative artifact path, for example
	// .verdi/policy/exemptions/x.md.
	Path string
	// Profile is the governing profile, as produced by
	// governanceprincipal.DecodeProfile.
	Profile gp.Profile
	// Verifier is the forge's commit verification.
	Verifier CommitVerifier
}

// validate refuses malformed input as an operational error.
func (in Input) validate() error {
	if in.Root == "" {
		return fmt.Errorf("signedapproval: input root must name a repository work tree")
	}
	if !objectIDRe.MatchString(in.Head) {
		return fmt.Errorf("signedapproval: input head %q is not a full lowercase object id", in.Head)
	}
	p := in.Path
	if p == "" || p == ".." || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") ||
		strings.ContainsAny(p, "\\\x00") || path.Clean(p) != p {
		return fmt.Errorf("signedapproval: input path %q is not a clean repo-relative path", p)
	}
	if in.Verifier == nil {
		return fmt.Errorf("signedapproval: input has no commit verifier")
	}
	if _, err := in.Profile.Digest(); err != nil {
		return fmt.Errorf("signedapproval: governing profile: %w", err)
	}
	return nil
}

// Authenticate determines every approval row of in.Path at in.Head (SI-256).
// Malformed input, an unreadable or malformed artifact at the head, a git
// failure, and a verifier error or contract violation are operational
// errors. Every other outcome is a Row: authenticated, or unproven with a
// closed reason. A missing or empty approvals sequence yields an Artifact
// with no rows.
func Authenticate(ctx context.Context, in Input) (Artifact, error) {
	if err := in.validate(); err != nil {
		return Artifact{}, err
	}
	headDoc, err := gitx.Show(ctx, in.Root, in.Head, in.Path)
	if err != nil {
		return Artifact{}, fmt.Errorf("signedapproval: reading %s at %s: %w", in.Path, in.Head, err)
	}
	rows, err := parseApprovalRows(headDoc)
	if err != nil {
		return Artifact{}, fmt.Errorf("signedapproval: %s at %s: %w", in.Path, in.Head, err)
	}
	blames := make([][]gitx.BlameLine, len(rows))
	for i, r := range rows {
		lines, err := gitx.Blame(ctx, in.Root, in.Head, in.Path, r.first, r.last)
		if err != nil {
			return Artifact{}, fmt.Errorf("signedapproval: attributing approval row (%s, %s): %w", r.role, r.principal, err)
		}
		blames[i] = lines
	}
	a := authenticator{in: in, headDoc: headDoc, rows: rows, blames: blames}
	out := make([]Row, 0, len(rows))
	for i := range rows {
		row, err := a.determine(ctx, i)
		if err != nil {
			return Artifact{}, err
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Role != out[j].Role {
			return out[i].Role < out[j].Role
		}
		return out[i].Principal < out[j].Principal
	})
	art := Artifact{Path: in.Path, Head: in.Head, Rows: out}
	if err := sealArtifact(&art); err != nil {
		return Artifact{}, err
	}
	return art, nil
}

// authenticator holds one artifact's head state while its rows are
// determined.
type authenticator struct {
	in      Input
	headDoc []byte
	rows    []approvalRow
	blames  [][]gitx.BlameLine

	// shallowKnown and isShallow cache whether the repository is a
	// shallow clone, read at most once.
	shallowKnown, isShallow bool
}

// determine runs the SI-256 checks for row i in order. The first failing
// check names the row's reason.
func (a *authenticator) determine(ctx context.Context, i int) (Row, error) {
	r, lines := a.rows[i], a.blames[i]
	out := Row{Role: r.role, Principal: r.principal, State: RowUnproven}
	unproven := func(reason, detail string) (Row, error) {
		out.Reason, out.Detail = reason, detail
		return out, nil
	}

	commits := distinctCommits(lines)
	boundary := ""
	for _, l := range lines {
		if l.Boundary {
			boundary = l.Commit
			break
		}
	}
	if len(commits) == 1 && boundary == "" {
		out.Commit = commits[0]
	}

	if j, ok := a.sharedLineWith(i); ok {
		return unproven(ReasonRowLinesShared, fmt.Sprintf("a line of this row is also a line of the row (%s, %s)", a.rows[j].role, a.rows[j].principal))
	}
	if boundary != "" {
		return unproven(ReasonHistoryBoundary, fmt.Sprintf("blame reached the history boundary %s, so the commit that introduced this row is not determined", boundary))
	}
	if len(commits) != 1 {
		return unproven(ReasonRowLinesSplit, fmt.Sprintf("the lines of this row blame to %d commits: %s", len(commits), strings.Join(commits, ", ")))
	}
	c := commits[0]
	if j, ok := a.commitSharedWith(i, c); ok {
		return unproven(ReasonApprovalCommitShared, fmt.Sprintf("commit %s also contributed a line to the row (%s, %s)", c, a.rows[j].role, a.rows[j].principal))
	}
	for _, l := range lines {
		if l.Filename != a.in.Path {
			return unproven(ReasonArtifactPathChanged, fmt.Sprintf("at %s the artifact's path was %q, not %q", c, l.Filename, a.in.Path))
		}
	}

	atCommit, err := gitx.Show(ctx, a.in.Root, c, a.in.Path)
	if err != nil {
		return Row{}, fmt.Errorf("signedapproval: reading %s at %s: %w", a.in.Path, c, err)
	}
	commitRows, perr := parseApprovalRows(atCommit)
	if perr != nil {
		return unproven(unreadableReason(perr, ReasonRowAbsentAtApprovalCommit), fmt.Sprintf("the artifact at %s has no readable approval rows: %v", c, perr))
	}
	if !carriesRow(commitRows, r, lines) {
		return unproven(ReasonRowAbsentAtApprovalCommit, fmt.Sprintf("the artifact at %s does not carry this row as one approval row on the lines blame maps it from", c))
	}
	if !bytes.Equal(withoutRowLines(atCommit, commitRows), withoutRowLines(a.headDoc, a.rows)) {
		return unproven(ReasonArtifactChangedAfterApproval, fmt.Sprintf("outside its approval rows the artifact at %s differs from the artifact at %s", c, a.in.Head))
	}
	reason, detail, err := a.withdrawal(ctx, r, c)
	if err != nil {
		return Row{}, err
	}
	if reason != "" {
		return unproven(reason, detail)
	}

	v, err := a.in.Verifier.VerifyCommit(ctx, c)
	if err != nil {
		return Row{}, fmt.Errorf("signedapproval: verifying commit %s: %w", c, err)
	}
	if err := v.Validate(); err != nil {
		return Row{}, err
	}
	if v.Commit != c {
		return Row{}, fmt.Errorf("signedapproval: asked to verify %s, the verifier answered about another commit %s", c, v.Commit)
	}
	if !v.Available {
		return unproven(ReasonVerificationUnavailable, v.UnavailableReason)
	}
	if !v.Verified {
		return unproven(ReasonSignatureUnverified, fmt.Sprintf("the forge does not report a verified signature on %s", c))
	}
	out.SignerAccountID = v.SignerAccountID
	source, ok, err := signedCommitSource(a.in.Profile, v.SignerAccountID, r.principal)
	if err != nil {
		return Row{}, err
	}
	if !ok {
		return unproven(ReasonSignerNotPrincipal, fmt.Sprintf("no signed-commit trust source of profile %q maps the verified signer, account %s, to this principal", a.in.Profile.ID, v.SignerAccountID))
	}
	digest, err := canonjson.Digest(rowEvidence{
		Path: a.in.Path, Head: a.in.Head, Role: r.role, Principal: r.principal, Commit: c,
		SignerAccountID: v.SignerAccountID, TrustSourceID: source, ProviderSnapshotID: v.ProviderSnapshotID,
	})
	if err != nil {
		return Row{}, fmt.Errorf("signedapproval: row evidence digest: %w", err)
	}
	out.State, out.TrustSourceID, out.EvidenceDigest = RowAuthenticated, source, digest
	return out, nil
}

// sharedLineWith reports another row sharing a document line with row i.
func (a *authenticator) sharedLineWith(i int) (int, bool) {
	r := a.rows[i]
	for j, o := range a.rows {
		if j != i && r.first <= o.last && o.first <= r.last {
			return j, true
		}
	}
	return 0, false
}

// commitSharedWith reports another row with any line blamed to c: one
// commit is one act, so it may introduce lines of one row only.
func (a *authenticator) commitSharedWith(i int, c string) (int, bool) {
	for j, lines := range a.blames {
		if j == i {
			continue
		}
		for _, l := range lines {
			if l.Commit == c {
				return j, true
			}
		}
	}
	return 0, false
}

// distinctCommits lists the commits lines blame to, in first-seen order.
func distinctCommits(lines []gitx.BlameLine) []string {
	var commits []string
	seen := make(map[string]bool, len(lines))
	for _, l := range lines {
		if !seen[l.Commit] {
			seen[l.Commit] = true
			commits = append(commits, l.Commit)
		}
	}
	return commits
}

// carriesRow reports whether commitRows holds a row with r's role and
// principal occupying exactly the contiguous lines blame maps r's lines
// from.
func carriesRow(commitRows []approvalRow, r approvalRow, lines []gitx.BlameLine) bool {
	first := lines[0].OrigLine
	for k, l := range lines {
		if l.OrigLine != first+k {
			return false
		}
	}
	last := first + len(lines) - 1
	for _, cr := range commitRows {
		if cr.first == first && cr.last == last {
			return cr.role == r.role && cr.principal == r.principal
		}
	}
	return false
}

// signedCommitSource finds the signed-commit trust source of profile under
// which the signer's canonical principal is principal. Source ids are
// unique within a decoded profile and CanonicalPrincipalID is injective, so
// at most one source can match.
func signedCommitSource(profile gp.Profile, signer, principal string) (string, bool, error) {
	for _, s := range profile.IdentityTrustSources {
		if s.Kind != gp.TrustSourceSignedCommit {
			continue
		}
		id, err := gp.CanonicalPrincipalID(s.ID, signer)
		if err != nil {
			return "", false, fmt.Errorf("signedapproval: principal for signer %s under %s: %w", signer, s.ID, err)
		}
		if string(id) == principal {
			return s.ID, true, nil
		}
	}
	return "", false, nil
}
