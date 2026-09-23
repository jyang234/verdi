package forge

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ChangeRequestState is the closed provider-neutral state of one change
// request (a GitHub pull request or a GitLab merge request) that the forge
// associates with a commit (SI-249; plan R-PB-2). Adapters map each
// provider's own vocabulary onto it and fail closed on any value they do not
// know; unknown values also fail closed here, at decode and in Validate.
type ChangeRequestState string

const (
	// ChangeRequestOpen is a change that has not been merged or closed. A
	// merge in progress is still open: only a completed merge is a merge.
	ChangeRequestOpen ChangeRequestState = "open"
	// ChangeRequestClosed is a change closed without merging.
	ChangeRequestClosed ChangeRequestState = "closed"
	// ChangeRequestMerged is a change the forge merged.
	ChangeRequestMerged ChangeRequestState = "merged"
)

// UnmarshalJSON rejects states outside the closed set, mirroring
// EnvironmentReviewState's decode contract.
func (s *ChangeRequestState) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("forge: decode change request state: %w", err)
	}
	state := ChangeRequestState(value)
	if !state.valid() {
		return fmt.Errorf("forge: unknown change request state %q", value)
	}
	*s = state
	return nil
}

func (s ChangeRequestState) valid() bool {
	switch s {
	case ChangeRequestOpen, ChangeRequestClosed, ChangeRequestMerged:
		return true
	default:
		return false
	}
}

// ChangeRequestMerge is one change request the forge associates with the
// observed commit, exactly as the forge reports it. ChangeID is the
// change's canonical decimal number within the repository (GitHub pull
// request number, GitLab merge request iid). MergeCommitSHA is "" unless
// State is merged and the forge reports a merge commit; an open change's
// provider test merge is never carried. MergedAt is the forge's merge time,
// normalized UTC RFC3339Nano, present exactly when State is merged.
type ChangeRequestMerge struct {
	ChangeID       string             `json:"change_id"`
	State          ChangeRequestState `json:"state"`
	TargetBranch   string             `json:"target_branch"`
	MergeCommitSHA string             `json:"merge_commit_sha"`
	MergedAt       string             `json:"merged_at"`
}

// MergeRecordFacts is one forge observation of the change requests
// associated with one commit, with the forge's own default branch for the
// repository (SI-249; plan R-PB-2): provider facts, never yet a judgment.
// ProveMergedIntoDefault is the pure predicate over them.
//
// Supported is false for a forge adapter that cannot read merge records:
// UnsupportedReason then names why, the facts carry only Repository and
// Commit, DefaultBranch is empty, and Changes is empty, and the predicate
// reads them as unproven, never as an error.
//
// Changes is never nil and is sorted by ChangeID numerically. ObservedAt is
// the normalized UTC observation time; ProviderSnapshotID is the digest of
// every other field, so two observations of the same provider facts share
// it (mirroring EnvironmentReviewFacts).
type MergeRecordFacts struct {
	Supported          bool                 `json:"supported"`
	UnsupportedReason  string               `json:"unsupported_reason"`
	Repository         string               `json:"repository"`
	Commit             string               `json:"commit"`
	DefaultBranch      string               `json:"default_branch"`
	Changes            []ChangeRequestMerge `json:"changes"`
	ObservedAt         string               `json:"observed_at"`
	ProviderSnapshotID string               `json:"provider_snapshot_id"`
}

// ValidateMergeRecordCommit checks the commit a merge-record read names: a
// full lowercase 40- or 64-character hexadecimal SHA. Adapters call it
// before any request, so an abbreviation, a branch name, or any other ref is
// an operational error that never reaches the forge.
func ValidateMergeRecordCommit(commit string) error {
	return validateCandidateSHA("merge_records.commit", commit)
}

// NewMergeRecordFacts normalizes ordering and observation time, derives the
// provider snapshot digest (which excludes observation time), and validates
// the result — mirroring NewEnvironmentReviewFacts' contract. The draft is
// never modified.
func NewMergeRecordFacts(draft MergeRecordFacts, observedAt time.Time) (MergeRecordFacts, error) {
	stamp, err := NormalizeTimestamp(observedAt.Format(time.RFC3339Nano))
	if err != nil {
		return MergeRecordFacts{}, fmt.Errorf("forge: merge records observed_at: %w", err)
	}
	facts := draft
	facts.ObservedAt = stamp

	changes := append([]ChangeRequestMerge(nil), draft.Changes...)
	if changes == nil {
		changes = []ChangeRequestMerge{}
	}
	sort.SliceStable(changes, func(i, j int) bool { return changeIDLess(changes[i].ChangeID, changes[j].ChangeID) })
	facts.Changes = changes

	facts.ProviderSnapshotID, err = facts.providerFactsDigest()
	if err != nil {
		return MergeRecordFacts{}, fmt.Errorf("forge: merge records provider snapshot identity: %w", err)
	}
	if err := facts.Validate(); err != nil {
		return MergeRecordFacts{}, err
	}
	return facts, nil
}

// changeIDLess orders canonical decimal change ids numerically. Ids that do
// not parse sort after those that do, by text, only so that ordering is
// total; Validate refuses them.
func changeIDLess(a, b string) bool {
	left, errLeft := strconv.ParseInt(a, 10, 64)
	right, errRight := strconv.ParseInt(b, 10, 64)
	switch {
	case errLeft == nil && errRight == nil && left != right:
		return left < right
	case (errLeft == nil) != (errRight == nil):
		return errLeft == nil
	default:
		return a < b
	}
}

// Validate checks the closed provider-neutral merge-record contract.
func (f MergeRecordFacts) Validate() error {
	if err := requireValue("merge_records.repository", f.Repository); err != nil {
		return err
	}
	if err := validateCandidateSHA("merge_records.commit", f.Commit); err != nil {
		return err
	}
	if _, err := normalizedTime("forge: merge_records.observed_at", f.ObservedAt); err != nil {
		return err
	}
	if err := validateDigest(f.ProviderSnapshotID); err != nil {
		return err
	}
	if f.Changes == nil {
		return fmt.Errorf("forge: merge_records.changes must be non-null")
	}
	if !f.Supported {
		if err := f.validateUnsupported(); err != nil {
			return err
		}
	} else if err := f.validateSupported(); err != nil {
		return err
	}
	wantDigest, err := f.providerFactsDigest()
	if err != nil {
		return fmt.Errorf("forge: recompute merge records provider snapshot identity: %w", err)
	}
	if f.ProviderSnapshotID != wantDigest {
		return fmt.Errorf("forge: merge_records.provider_snapshot_id does not match normalized facts")
	}
	return nil
}

func (f MergeRecordFacts) validateUnsupported() error {
	if err := requireValue("merge_records.unsupported_reason", f.UnsupportedReason); err != nil {
		return err
	}
	if f.DefaultBranch != "" || len(f.Changes) != 0 {
		return fmt.Errorf("forge: unsupported merge records must carry no default branch and no changes")
	}
	return nil
}

func (f MergeRecordFacts) validateSupported() error {
	if f.UnsupportedReason != "" {
		return fmt.Errorf("forge: supported merge records must carry no unsupported_reason")
	}
	if err := requireValue("merge_records.default_branch", f.DefaultBranch); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(f.Changes))
	previous := int64(0)
	for i, change := range f.Changes {
		prefix := fmt.Sprintf("merge_records.changes[%d]", i)
		if err := validateCanonicalID(prefix+".change_id", change.ChangeID); err != nil {
			return err
		}
		if _, exists := seen[change.ChangeID]; exists {
			return fmt.Errorf("forge: %s: duplicate change_id %q", prefix, change.ChangeID)
		}
		seen[change.ChangeID] = struct{}{}
		id, _ := strconv.ParseInt(change.ChangeID, 10, 64)
		if id < previous {
			return fmt.Errorf("forge: merge_records.changes are not sorted by change_id")
		}
		previous = id
		if err := change.validate(prefix); err != nil {
			return err
		}
	}
	return nil
}

func (c ChangeRequestMerge) validate(prefix string) error {
	if !c.State.valid() {
		return fmt.Errorf("forge: %s.state: unknown change request state %q", prefix, c.State)
	}
	if err := requireValue(prefix+".target_branch", c.TargetBranch); err != nil {
		return err
	}
	if c.State != ChangeRequestMerged {
		if c.MergeCommitSHA != "" || c.MergedAt != "" {
			return fmt.Errorf("forge: %s: a change in state %q carries no merge commit or merge time", prefix, c.State)
		}
		return nil
	}
	if c.MergeCommitSHA != "" {
		if err := validateCandidateSHA(prefix+".merge_commit_sha", c.MergeCommitSHA); err != nil {
			return err
		}
	}
	if _, err := normalizedTime("forge: "+prefix+".merged_at", c.MergedAt); err != nil {
		return err
	}
	return nil
}

// providerFactsDigest is the digest of every provider fact, deliberately
// excluding the observation stamp and the digest itself.
func (f MergeRecordFacts) providerFactsDigest() (string, error) {
	identity := f
	identity.ObservedAt = ""
	identity.ProviderSnapshotID = ""
	return canonjson.Digest(identity)
}

// MergeProofState is the closed outcome of ProveMergedIntoDefault. There is
// no violated state: the forge's answer either proves the merge or leaves it
// unproven, and the consumer blocks on unproven.
type MergeProofState string

const (
	// MergeProofProven means the forge reports a change merged into its
	// default branch whose merge commit is exactly the commit.
	MergeProofProven MergeProofState = "proven"
	// MergeProofUnproven means it does not; Reason says why.
	MergeProofUnproven MergeProofState = "unproven"
)

// The closed reason codes of an unproven MergeProof.
const (
	// MergeProofReasonUnsupported means the adapter cannot read merge
	// records (facts.Supported is false).
	MergeProofReasonUnsupported = "merge-records-unsupported"
	// MergeProofReasonNoMergeIntoDefault means no associated change is
	// merged into the forge's default branch with the commit as its merge
	// commit.
	MergeProofReasonNoMergeIntoDefault = "no-merge-into-default-branch"
)

// MergeProof is ProveMergedIntoDefault's answer for one commit, bound to the
// provider snapshot it was derived from. ChangeID, TargetBranch, and
// MergedAt are set only when State is proven; Reason is set only when it is
// unproven, and Detail then names what the forge reported.
type MergeProof struct {
	State              MergeProofState `json:"state"`
	Commit             string          `json:"commit"`
	ChangeID           string          `json:"change_id,omitempty"`
	TargetBranch       string          `json:"target_branch,omitempty"`
	MergedAt           string          `json:"merged_at,omitempty"`
	ProviderSnapshotID string          `json:"provider_snapshot_id"`
	Reason             string          `json:"reason,omitempty"`
	Detail             string          `json:"detail,omitempty"`
}

// ProveMergedIntoDefault decides, from forge-authenticated merge records
// alone, whether the forge merged a change request into its default branch
// with facts.Commit as that change's merge commit (SI-249; design §5 Scope,
// the landing snapshot D and the issuance merge M_e). It is proven exactly
// when some change has State merged, TargetBranch equal to the forge's
// DefaultBranch, and MergeCommitSHA equal to Commit; when several qualify,
// the lowest ChangeID is reported. Everything else is unproven with a closed
// Reason, including an unsupported adapter.
//
// This predicate never claims a two-parent merge commit: a squash or rebase
// merge can report a single-parent commit as its merge commit, and whether
// Commit has exactly two parents, and every ancestry relation the exemption
// needs, are the consumer's checks against the local object graph.
//
// It validates facts first; facts that break the contract (a hand-built or
// tampered value) are an error, never a proof. For valid facts it is pure and
// deterministic.
func ProveMergedIntoDefault(facts MergeRecordFacts) (MergeProof, error) {
	if err := facts.Validate(); err != nil {
		return MergeProof{}, fmt.Errorf("forge: prove merged into default branch: %w", err)
	}
	proof := MergeProof{Commit: facts.Commit, ProviderSnapshotID: facts.ProviderSnapshotID}
	if !facts.Supported {
		proof.State = MergeProofUnproven
		proof.Reason = MergeProofReasonUnsupported
		proof.Detail = facts.UnsupportedReason
		return proof, nil
	}

	var qualifying []ChangeRequestMerge
	for _, change := range facts.Changes {
		if change.State == ChangeRequestMerged && change.TargetBranch == facts.DefaultBranch && change.MergeCommitSHA == facts.Commit {
			qualifying = append(qualifying, change)
		}
	}
	if len(qualifying) == 0 {
		proof.State = MergeProofUnproven
		proof.Reason = MergeProofReasonNoMergeIntoDefault
		proof.Detail = noMergeIntoDefaultDetail(facts)
		return proof, nil
	}

	// Validate proved Changes sorted by ChangeID, so the first qualifying
	// change carries the lowest id.
	reported := qualifying[0]
	proof.State = MergeProofProven
	proof.ChangeID = reported.ChangeID
	proof.TargetBranch = reported.TargetBranch
	proof.MergedAt = reported.MergedAt
	if len(qualifying) > 1 {
		ids := make([]string, 0, len(qualifying))
		for _, change := range qualifying {
			ids = append(ids, change.ChangeID)
		}
		proof.Detail = fmt.Sprintf("changes %s each report merge commit %s into the default branch %s; the lowest change id is reported",
			strings.Join(ids, ", "), facts.Commit, facts.DefaultBranch)
	}
	return proof, nil
}

// noMergeIntoDefaultDetail names, change by change, what the forge reported
// instead of a merge into its default branch with the commit as merge commit.
func noMergeIntoDefaultDetail(facts MergeRecordFacts) string {
	if len(facts.Changes) == 0 {
		return fmt.Sprintf("default_branch=%s: the forge associates no change request with commit %s", facts.DefaultBranch, facts.Commit)
	}
	parts := make([]string, 0, len(facts.Changes))
	for _, change := range facts.Changes {
		parts = append(parts, fmt.Sprintf("change %s (state=%s target_branch=%s): %s",
			change.ChangeID, change.State, change.TargetBranch, changeNotQualifying(facts, change)))
	}
	return fmt.Sprintf("default_branch=%s commit=%s: %s", facts.DefaultBranch, facts.Commit, strings.Join(parts, "; "))
}

// changeNotQualifying says why one change is not a merge of the commit into
// the default branch.
func changeNotQualifying(facts MergeRecordFacts, change ChangeRequestMerge) string {
	switch {
	case change.State == ChangeRequestOpen:
		return "open, not merged; an open change's provider test merge is never a merge"
	case change.State == ChangeRequestClosed:
		return "closed without merging"
	case change.TargetBranch != facts.DefaultBranch:
		return fmt.Sprintf("merged into %s, not the default branch %s (merge commit %s)",
			change.TargetBranch, facts.DefaultBranch, orNone(change.MergeCommitSHA))
	case change.MergeCommitSHA == "":
		return "merged into the default branch with no forge-reported merge commit (a fast-forward merge leaves none)"
	default:
		return fmt.Sprintf("merged into the default branch with merge commit %s, not %s", change.MergeCommitSHA, facts.Commit)
	}
}
