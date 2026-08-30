package forge

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
)

// ApprovalState is the closed provider-neutral state of a forge approval.
type ApprovalState string

const (
	// ApprovalActive means the provider currently recognizes the approval.
	ApprovalActive ApprovalState = "active"
	// ApprovalDismissed means the provider retained the review but dismissed it.
	ApprovalDismissed ApprovalState = "dismissed"
	// ApprovalRevoked means the provider retained an explicit revoked fact.
	ApprovalRevoked ApprovalState = "revoked"
)

// UnmarshalJSON rejects states outside the provider-neutral closed set.
func (s *ApprovalState) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf("forge: decode approval state: %w", err)
	}
	state := ApprovalState(value)
	if !state.valid() {
		return fmt.Errorf("forge: unknown approval state %q", value)
	}
	*s = state
	return nil
}

func (s ApprovalState) valid() bool {
	switch s {
	case ApprovalActive, ApprovalDismissed, ApprovalRevoked:
		return true
	default:
		return false
	}
}

// ProviderActor is provider-stable subject evidence, never a display name.
type ProviderActor struct {
	Scheme  string `json:"scheme"`
	Subject string `json:"subject"`
}

// ProviderWitness preserves one normalized provider operand behind a fact.
type ProviderWitness struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Approval is one immutable provider approval fact bound to a candidate.
type Approval struct {
	ApprovalID        string            `json:"approval_id"`
	ApprovalRef       string            `json:"approval_ref"`
	State             ApprovalState     `json:"state"`
	ApprovedAt        string            `json:"approved_at"`
	UpdatedAt         string            `json:"updated_at"`
	CandidateSHA      string            `json:"candidate_sha"`
	Actor             ProviderActor     `json:"actor"`
	ProviderWitnesses []ProviderWitness `json:"provider_witnesses"`
}

// ApprovalSnapshot is one forge observation, including an empty approval set.
type ApprovalSnapshot struct {
	Forge              string        `json:"forge"`
	Repository         string        `json:"repository"`
	ChangeID           string        `json:"change_id"`
	CandidateSHA       string        `json:"candidate_sha"`
	CandidateAuthor    ProviderActor `json:"candidate_author"`
	ObservedAt         string        `json:"observed_at"`
	ProviderSnapshotID string        `json:"provider_snapshot_id"`
	Approvals          []Approval    `json:"approvals"`
}

// NewApprovalSnapshot normalizes ordering and observation time, validates all
// facts, and derives an identity that deliberately excludes observation time.
func NewApprovalSnapshot(forgeName, repository, changeID, candidateSHA string, candidateAuthor ProviderActor, observedAt time.Time, approvals []Approval) (ApprovalSnapshot, error) {
	stamp, err := NormalizeTimestamp(observedAt.Format(time.RFC3339Nano))
	if err != nil {
		return ApprovalSnapshot{}, fmt.Errorf("forge: observed_at: %w", err)
	}

	rows := cloneApprovals(approvals)
	if rows == nil {
		rows = []Approval{}
	}
	for i := range rows {
		sort.Slice(rows[i].ProviderWitnesses, func(a, b int) bool {
			left, right := rows[i].ProviderWitnesses[a], rows[i].ProviderWitnesses[b]
			if left.Name != right.Name {
				return left.Name < right.Name
			}
			return left.Value < right.Value
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ApprovalID < rows[j].ApprovalID })

	snapshot := ApprovalSnapshot{
		Forge: forgeName, Repository: repository, ChangeID: changeID,
		CandidateSHA: candidateSHA, CandidateAuthor: candidateAuthor,
		ObservedAt: stamp, Approvals: rows,
	}
	snapshot.ProviderSnapshotID, err = snapshot.providerFactsDigest()
	if err != nil {
		return ApprovalSnapshot{}, fmt.Errorf("forge: provider snapshot identity: %w", err)
	}
	if err := snapshot.Validate(); err != nil {
		return ApprovalSnapshot{}, err
	}
	return snapshot, nil
}

// Validate checks the closed provider-neutral snapshot contract.
func (s ApprovalSnapshot) Validate() error {
	if err := requireValue("forge", s.Forge); err != nil {
		return err
	}
	if err := requireValue("repository", s.Repository); err != nil {
		return err
	}
	if err := requireValue("change_id", s.ChangeID); err != nil {
		return err
	}
	if err := validateCandidateSHA("candidate_sha", s.CandidateSHA); err != nil {
		return err
	}
	if s.CandidateAuthor == (ProviderActor{}) {
		return fmt.Errorf("forge: candidate_author is required")
	}
	if err := validateProviderActor("forge: candidate_author", s.Forge, s.CandidateAuthor); err != nil {
		return err
	}
	if _, err := normalizedTime("forge: observed_at", s.ObservedAt); err != nil {
		return err
	}
	if err := validateDigest(s.ProviderSnapshotID); err != nil {
		return err
	}
	if s.Approvals == nil {
		return fmt.Errorf("forge: approvals must be non-null")
	}

	seen := make(map[string]struct{}, len(s.Approvals))
	previous := ""
	for i, approval := range s.Approvals {
		if err := approval.validate(s.Forge, i); err != nil {
			return err
		}
		if _, exists := seen[approval.ApprovalID]; exists {
			return fmt.Errorf("forge: duplicate approval_id %q", approval.ApprovalID)
		}
		seen[approval.ApprovalID] = struct{}{}
		if i > 0 && approval.ApprovalID < previous {
			return fmt.Errorf("forge: approvals are not sorted by approval_id")
		}
		previous = approval.ApprovalID
	}
	wantDigest, err := s.providerFactsDigest()
	if err != nil {
		return fmt.Errorf("forge: recompute provider snapshot identity: %w", err)
	}
	if s.ProviderSnapshotID != wantDigest {
		return fmt.Errorf("forge: provider_snapshot_id does not match normalized provider facts")
	}
	return nil
}

func (s ApprovalSnapshot) providerFactsDigest() (string, error) {
	identity := struct {
		Forge           string        `json:"forge"`
		Repository      string        `json:"repository"`
		ChangeID        string        `json:"change_id"`
		CandidateSHA    string        `json:"candidate_sha"`
		CandidateAuthor ProviderActor `json:"candidate_author"`
		Approvals       []Approval    `json:"approvals"`
	}{s.Forge, s.Repository, s.ChangeID, s.CandidateSHA, s.CandidateAuthor, s.Approvals}
	return canonjson.Digest(identity)
}

// DecodeApprovalJSON strict-decodes one closed provider approval response.
func DecodeApprovalJSON(reader io.Reader, out any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return fmt.Errorf("forge: decode approval response: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("forge: trailing data after approval response")
		}
		return fmt.Errorf("forge: trailing data after approval response: %w", err)
	}
	return nil
}

func (a Approval) validate(forgeName string, index int) error {
	prefix := fmt.Sprintf("forge: approvals[%d]", index)
	if err := requireValue(prefix+".approval_id", a.ApprovalID); err != nil {
		return err
	}
	if err := requireValue(prefix+".approval_ref", a.ApprovalRef); err != nil {
		return err
	}
	if !a.State.valid() {
		return fmt.Errorf("%s.state: unknown approval state %q", prefix, a.State)
	}
	approvedAt, err := normalizedTime(prefix+".approved_at", a.ApprovedAt)
	if err != nil {
		return err
	}
	updatedAt, err := normalizedTime(prefix+".updated_at", a.UpdatedAt)
	if err != nil {
		return err
	}
	if updatedAt.Before(approvedAt) {
		return fmt.Errorf("%s.updated_at precedes approved_at", prefix)
	}
	if err := validateCandidateSHA(prefix+".candidate_sha", a.CandidateSHA); err != nil {
		return err
	}
	if err := validateProviderActor(prefix, forgeName, a.Actor); err != nil {
		return err
	}
	if a.ProviderWitnesses == nil {
		return fmt.Errorf("%s.provider_witnesses must be non-null", prefix)
	}
	for witnessIndex, witness := range a.ProviderWitnesses {
		if err := requireValue(fmt.Sprintf("%s.provider_witnesses[%d].name", prefix, witnessIndex), witness.Name); err != nil {
			return err
		}
		if err := requireValue(fmt.Sprintf("%s.provider_witnesses[%d].value", prefix, witnessIndex), witness.Value); err != nil {
			return err
		}
	}
	return nil
}

func validateProviderActor(prefix, forgeName string, actor ProviderActor) error {
	wantScheme := ""
	switch forgeName {
	case "github":
		wantScheme = "github-user-id"
	case "gitlab":
		wantScheme = "gitlab-user-id"
	default:
		return fmt.Errorf("%s.actor.scheme: forge %q has no legal provider actor scheme", prefix, forgeName)
	}
	if actor.Scheme != wantScheme {
		return fmt.Errorf("%s.actor.scheme must be %q for forge %q, got %q", prefix, wantScheme, forgeName, actor.Scheme)
	}
	if err := requireValue(prefix+".actor.subject", actor.Subject); err != nil {
		return err
	}
	numericID, err := strconv.ParseInt(actor.Subject, 10, 64)
	if err != nil || numericID <= 0 || strconv.FormatInt(numericID, 10) != actor.Subject {
		return fmt.Errorf("%s.actor.subject must be a canonical positive base-10 provider numeric ID", prefix)
	}
	return nil
}

// NormalizeTimestamp returns a provider timestamp in UTC RFC3339Nano form.
func NormalizeTimestamp(value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("missing timestamp")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return "", fmt.Errorf("invalid RFC3339Nano timestamp %q: %w", value, err)
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func normalizedTime(field, value string) (time.Time, error) {
	normalized, err := NormalizeTimestamp(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %w", field, err)
	}
	if normalized != value {
		return time.Time{}, fmt.Errorf("%s must be normalized UTC RFC3339Nano, got %q", field, value)
	}
	parsed, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: parse normalized timestamp: %w", field, err)
	}
	return parsed, nil
}

func requireValue(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("forge: %s must be nonempty without surrounding whitespace", field)
	}
	return nil
}

func validateCandidateSHA(field, value string) error {
	if len(value) != 40 && len(value) != 64 {
		return fmt.Errorf("forge: %s must be a full 40- or 64-character commit SHA", field)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf("forge: %s must be lowercase hexadecimal", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("forge: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

func validateDigest(value string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+64 {
		return fmt.Errorf("forge: provider_snapshot_id must be a sha256 digest")
	}
	hexPart := strings.TrimPrefix(value, prefix)
	if hexPart != strings.ToLower(hexPart) {
		return fmt.Errorf("forge: provider_snapshot_id must use lowercase hexadecimal")
	}
	if _, err := hex.DecodeString(hexPart); err != nil {
		return fmt.Errorf("forge: provider_snapshot_id must use hexadecimal: %w", err)
	}
	return nil
}

func cloneApprovals(in []Approval) []Approval {
	if in == nil {
		return nil
	}
	out := append([]Approval(nil), in...)
	for i := range out {
		if in[i].ProviderWitnesses != nil {
			out[i].ProviderWitnesses = append([]ProviderWitness(nil), in[i].ProviderWitnesses...)
		}
	}
	return out
}
