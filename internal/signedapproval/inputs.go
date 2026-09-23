package signedapproval

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/canonjson"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// DisclosurePrincipalUnauthenticated names a row that Authenticate found
// authenticated but whose signer claim the kernel did not authenticate as
// the row's principal under the profile AuthorizationInputs was given.
// It is a disclosure reason, not a Row reason.
const DisclosurePrincipalUnauthenticated = "principal-unauthenticated"

// AuthorizationInputs is one artifact's contribution to a kernel
// authorization request.
type AuthorizationInputs struct {
	// Approvals holds authenticated rows only.
	Approvals []gp.ApprovalRecord
	// Resolutions holds exactly one resolution per principal with an
	// authenticated row.
	Resolutions []gp.PrincipalResolution
	// Disclosures holds one entry per dropped row, naming its reason.
	Disclosures []string
}

// AuthorizationInputs builds the kernel inputs for this artifact alone
// (SI-256), never store-wide. For each principal with authenticated rows,
// a trust-fact reader scoped to exactly those rows answers for the rows'
// signed-commit source with the verified signer as the one subject, and
// the kernel resolves the signer claim. Only a resolution that comes back
// authenticated as the row's principal keeps the principal's rows; every
// other row is dropped with a disclosure. Dropping a row can only make
// authorization harder, never easier.
//
// An Artifact not produced by Authenticate, or modified since, and a
// profile not produced by governanceprincipal.DecodeProfile are
// operational errors.
func (a Artifact) AuthorizationInputs(ctx context.Context, profile gp.Profile) (AuthorizationInputs, error) {
	if err := a.checkSeal(); err != nil {
		return AuthorizationInputs{}, err
	}
	if _, err := profile.Digest(); err != nil {
		return AuthorizationInputs{}, fmt.Errorf("signedapproval: governing profile: %w", err)
	}
	out := AuthorizationInputs{Approvals: []gp.ApprovalRecord{}, Resolutions: []gp.PrincipalResolution{}, Disclosures: []string{}}
	byPrincipal := make(map[string][]Row)
	for _, r := range a.Rows {
		if r.State == RowAuthenticated {
			byPrincipal[r.Principal] = append(byPrincipal[r.Principal], r)
			continue
		}
		out.Disclosures = append(out.Disclosures, a.disclosure(r, r.Reason, r.Detail))
	}
	principals := make([]string, 0, len(byPrincipal))
	for p := range byPrincipal {
		principals = append(principals, p)
	}
	sort.Strings(principals)

	for _, p := range principals {
		rows := byPrincipal[p]
		// Every authenticated row of one principal carries the same source
		// and signer: Principal = CanonicalPrincipalID(source, signer),
		// which is injective.
		source, signer := rows[0].TrustSourceID, rows[0].SignerAccountID
		digests := make([]string, 0, len(rows))
		for _, r := range rows {
			digests = append(digests, r.EvidenceDigest)
		}
		sort.Strings(digests)
		evidence, err := canonjson.Digest(principalEvidence{RowEvidenceDigests: digests})
		if err != nil {
			return AuthorizationInputs{}, fmt.Errorf("signedapproval: principal evidence digest: %w", err)
		}
		reader := principalFacts{sourceID: source, signer: signer, evidence: evidence}
		res, err := gp.NewResolver(reader).Resolve(ctx, profile, gp.PrincipalClaim{TrustSource: source, Subject: signer})
		if err != nil {
			return AuthorizationInputs{}, fmt.Errorf("signedapproval: resolving principal %s: %w", p, err)
		}
		if res.State != gp.ResolutionAuthenticated || res.PrincipalID != gp.PrincipalID(p) {
			detail := fmt.Sprintf("the kernel resolved the signer claim %s/%s as %s (%s)", source, signer, res.State, witnessCodes(res.Witnesses))
			for _, r := range rows {
				out.Disclosures = append(out.Disclosures, a.disclosure(r, DisclosurePrincipalUnauthenticated, detail))
			}
			continue
		}
		out.Resolutions = append(out.Resolutions, res)
		for _, r := range rows {
			out.Approvals = append(out.Approvals, gp.ApprovalRecord{Role: r.Role, PrincipalID: gp.PrincipalID(p)})
		}
	}
	sort.Slice(out.Approvals, func(i, j int) bool {
		if out.Approvals[i].Role != out.Approvals[j].Role {
			return out.Approvals[i].Role < out.Approvals[j].Role
		}
		return out.Approvals[i].PrincipalID < out.Approvals[j].PrincipalID
	})
	sort.Strings(out.Disclosures)
	return out, nil
}

// disclosure renders one dropped row: its reason, the row, the artifact,
// and the detail.
func (a Artifact) disclosure(r Row, reason, detail string) string {
	return fmt.Sprintf("%s: approval row (%s, %s) of %s at %s: %s", reason, r.Role, r.Principal, a.Path, a.Head, detail)
}

// witnessCodes lists a resolution's witness codes for a disclosure.
func witnessCodes(ws []gp.Witness) string {
	codes := make([]string, 0, len(ws))
	for _, w := range ws {
		codes = append(codes, w.Code)
	}
	return strings.Join(codes, ", ")
}

// principalEvidence is the projection a principal's trust-fact evidence
// digest covers: the evidence digests of that principal's authenticated
// rows in this artifact, sorted.
type principalEvidence struct {
	RowEvidenceDigests []string `json:"row_evidence_digests"`
}

// principalFacts is the trust-fact reader for one principal of one
// artifact. It answers its own signed-commit source with the verified
// signer as the one subject and every other source as unavailable.
type principalFacts struct {
	sourceID, signer, evidence string
}

func (f principalFacts) ReadTrustFact(_ context.Context, source gp.TrustSource, _ gp.PrincipalClaim) (gp.TrustFact, error) {
	if source.ID != f.sourceID || source.Kind != gp.TrustSourceSignedCommit {
		return gp.TrustFact{
			SourceID: source.ID, SourceKind: source.Kind,
			Reason: fmt.Sprintf("this artifact carries signed-commit approval evidence only from source %q", f.sourceID),
		}, nil
	}
	return gp.TrustFact{
		SourceID: source.ID, SourceKind: source.Kind,
		Subjects: []string{f.signer}, EvidenceDigest: f.evidence,
		Available: true, Valid: true,
	}, nil
}
