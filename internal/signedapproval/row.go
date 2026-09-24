package signedapproval

import (
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
)

// RowState is the closed two-valued state of one approval row. A row is
// authenticated or unproven; this package never reports a row violated,
// because a missing proof of an approval is not a contradiction of it.
type RowState string

// The two row states.
const (
	RowAuthenticated RowState = "authenticated"
	RowUnproven      RowState = "unproven"
)

// The closed reasons an approval row is unproven.
const (
	// ReasonRowLinesShared: a line of the row is shared with another row,
	// as in flow style.
	ReasonRowLinesShared = "row-lines-shared"
	// ReasonRowLinesSplit: the row's lines blame to different commits.
	ReasonRowLinesSplit = "row-lines-split"
	// ReasonHistoryBoundary: the available history ends before the
	// determination can finish. Blame reached a history boundary (a root
	// commit, or a shallow or grafted boundary), so the row's introducing
	// commit is not determined; or the repository is a shallow clone, so
	// the ancestry path from the introducing commit to the head cannot be
	// shown free of a withdrawal.
	ReasonHistoryBoundary = "history-boundary"
	// ReasonApprovalCommitShared: the commit that introduced the row also
	// contributed a line to another row of the same artifact.
	ReasonApprovalCommitShared = "approval-commit-shared"
	// ReasonArtifactPathChanged: at the introducing commit the artifact
	// had a different path.
	ReasonArtifactPathChanged = "artifact-path-changed"
	// ReasonRowAbsentAtApprovalCommit: the introducing commit's own copy
	// of the artifact does not carry exactly this row, as one approval
	// row, on exactly the lines blame maps it from. Without this check a
	// later commit could splice lines one signed commit introduced (from
	// the body, or from two different rows) into a row that commit never
	// approved.
	ReasonRowAbsentAtApprovalCommit = "row-absent-at-approval-commit"
	// ReasonRowWithdrawnAfterApproval: a commit that changes the artifact
	// on the full-history ancestry path from the introducing commit to the
	// head does not carry this row, so the approval was withdrawn after it
	// was given; a merge that brings the row back is not a new signature.
	ReasonRowWithdrawnAfterApproval = "row-withdrawn-after-approval"
	// ReasonNonstandardLineBreak: a historical copy of the artifact that
	// the determination reads (the introducing commit's, or a later one on
	// the path to the head) has a frontmatter line break other than LF or
	// CRLF: a lone CR, NEL, LS, or PS, which YAML counts as a line break
	// and Git does not. At the head the same frontmatter is an operational
	// error.
	ReasonNonstandardLineBreak = "nonstandard-line-break"
	// ReasonArtifactChangedAfterApproval: the artifact outside its
	// approval rows differs between the introducing commit and the head.
	ReasonArtifactChangedAfterApproval = "artifact-changed-after-approval"
	// ReasonVerificationUnavailable: the forge could not answer for the
	// introducing commit.
	ReasonVerificationUnavailable = "verification-unavailable"
	// ReasonSignatureUnverified: the forge does not report a verified
	// signature on the introducing commit.
	ReasonSignatureUnverified = "signature-unverified"
	// ReasonSignerNotPrincipal: no signed-commit trust source of the
	// profile maps the verified signer to the row's principal. This covers
	// an approval attributed to another account and a row whose principal
	// is not a signed-commit principal.
	ReasonSignerNotPrincipal = "signer-not-principal"
)

// Row is the determination for one approval row at the head.
type Row struct {
	Role, Principal string
	State           RowState
	// Commit is the introducing commit C, when determined.
	Commit string
	// SignerAccountID is set when the forge verified C.
	SignerAccountID string
	// TrustSourceID is the signed-commit source whose canonical principal
	// for the signer equals Principal; authenticated rows only.
	TrustSourceID string
	// EvidenceDigest is the canonical digest over path, head, role,
	// principal, commit, signer, trust source id, and provider snapshot
	// id; authenticated rows only.
	EvidenceDigest string
	// Reason is a closed code and Detail a human explanation; unproven
	// rows only.
	Reason, Detail string
}

// Artifact is one artifact's approval rows at one head: one Row per
// approval row, sorted by (role, principal).
//
// Only Authenticate produces a consumable Artifact. It carries an
// unexported seal over its exported content, and AuthorizationInputs
// refuses an Artifact that was built by hand or modified afterwards, so no
// caller can turn a claimed row into kernel approval inputs.
type Artifact struct {
	Path, Head string
	Rows       []Row

	seal string
}

// sealArtifact mints the integrity seal on a freshly determined artifact.
func sealArtifact(a *Artifact) error {
	d, err := canonjson.Digest(*a)
	if err != nil {
		return fmt.Errorf("signedapproval: sealing artifact: %w", err)
	}
	a.seal = d
	return nil
}

// checkSeal proves a is unmodified Authenticate output.
func (a Artifact) checkSeal() error {
	if a.seal == "" {
		return fmt.Errorf("signedapproval: artifact %s at %s was not produced by Authenticate", a.Path, a.Head)
	}
	d, err := canonjson.Digest(a)
	if err != nil {
		return fmt.Errorf("signedapproval: checking artifact seal: %w", err)
	}
	if d != a.seal {
		return fmt.Errorf("signedapproval: artifact %s at %s was modified after Authenticate", a.Path, a.Head)
	}
	return nil
}

// rowEvidence is the projection a row's EvidenceDigest covers.
type rowEvidence struct {
	Path               string `json:"path"`
	Head               string `json:"head"`
	Role               string `json:"role"`
	Principal          string `json:"principal"`
	Commit             string `json:"commit"`
	SignerAccountID    string `json:"signer_account_id"`
	TrustSourceID      string `json:"trust_source_id"`
	ProviderSnapshotID string `json:"provider_snapshot_id"`
}
