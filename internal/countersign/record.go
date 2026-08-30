// Package countersign resolves candidate-bound forge approval snapshots into
// canonical countersign evidence without performing I/O or authorization
// outside the narrow governance-principal kernel operations it consumes.
package countersign

import (
	"encoding/hex"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/forge"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// SchemaID is the sole countersign witness schema.
const SchemaID = "verdi.countersign-witness/v1"

// Verdict is the closed countersign, freshness, and separation vocabulary.
type Verdict string

const (
	VerdictProven   Verdict = "proven"
	VerdictViolated Verdict = "violated-with-witness"
	VerdictUnproven Verdict = "unproven"
)

func (v Verdict) validate() error {
	switch v {
	case VerdictProven, VerdictViolated, VerdictUnproven:
		return nil
	default:
		return fmt.Errorf("countersign: unknown verdict %q", v)
	}
}

// SeparationRule is the closed obligation separation vocabulary.
type SeparationRule string

const (
	SeparationNone                SeparationRule = "none"
	SeparationDifferentFromAuthor SeparationRule = "different-from-author"
)

func (r SeparationRule) validate() error {
	switch r {
	case SeparationNone, SeparationDifferentFromAuthor:
		return nil
	default:
		return fmt.Errorf("countersign: unknown separation rule %q", r)
	}
}

const (
	// SchemeAttestation is the sole accepted obligation scheme.
	SchemeAttestation = "attestation"
	// KindCountersign is the sole accepted obligation kind.
	KindCountersign = "countersign"
)

// Obligation is the profile-local countersign question supplied by a caller.
type Obligation struct {
	Transition     string
	Scheme         string
	Kind           string
	Role           string
	RequiredCount  int
	SeparationRule SeparationRule
}

func (o Obligation) validate() error {
	if err := gp.ValidateID(o.Transition); err != nil {
		return fmt.Errorf("countersign: obligation transition: %w", err)
	}
	if o.Scheme != SchemeAttestation {
		return fmt.Errorf("countersign: obligation scheme must be %q, got %q", SchemeAttestation, o.Scheme)
	}
	if o.Kind != KindCountersign {
		return fmt.Errorf("countersign: obligation kind must be %q, got %q", KindCountersign, o.Kind)
	}
	if err := gp.ValidateID(o.Role); err != nil {
		return fmt.Errorf("countersign: obligation role: %w", err)
	}
	if o.RequiredCount <= 0 {
		return fmt.Errorf("countersign: obligation required_count must be positive")
	}
	return o.SeparationRule.validate()
}

// FreshnessPolicy is a self-bound policy. Construct it with
// NewFreshnessPolicy; Validate rejects mutation or cross-policy pairing.
type FreshnessPolicy struct {
	ID                           string
	Digest                       string
	MaximumObservationAgeSeconds int64
	MaximumApprovalAgeSeconds    int64
}

// NewFreshnessPolicy validates identity and ceilings and computes the policy
// digest over exactly those identity fields.
func NewFreshnessPolicy(id string, maximumObservationAgeSeconds, maximumApprovalAgeSeconds int64) (FreshnessPolicy, error) {
	p := FreshnessPolicy{ID: id, MaximumObservationAgeSeconds: maximumObservationAgeSeconds, MaximumApprovalAgeSeconds: maximumApprovalAgeSeconds}
	if err := p.validateValues(); err != nil {
		return FreshnessPolicy{}, err
	}
	digest, err := p.contentDigest()
	if err != nil {
		return FreshnessPolicy{}, err
	}
	p.Digest = digest
	return p, nil
}

// Validate proves that the policy remains paired with its own digest.
func (p FreshnessPolicy) Validate() error {
	if err := p.validateValues(); err != nil {
		return err
	}
	want, err := p.contentDigest()
	if err != nil {
		return err
	}
	if p.Digest != want {
		return fmt.Errorf("countersign: freshness policy digest does not match policy identity fields")
	}
	return nil
}

func (p FreshnessPolicy) validateValues() error {
	if err := gp.ValidateID(p.ID); err != nil {
		return fmt.Errorf("countersign: freshness policy id: %w", err)
	}
	const maxSeconds = math.MaxInt64 / int64(time.Second)
	if p.MaximumObservationAgeSeconds <= 0 || p.MaximumObservationAgeSeconds > maxSeconds {
		return fmt.Errorf("countersign: maximum_observation_age_seconds must be positive and representable as a duration")
	}
	if p.MaximumApprovalAgeSeconds < 0 || p.MaximumApprovalAgeSeconds > maxSeconds {
		return fmt.Errorf("countersign: maximum_approval_age_seconds must be nonnegative and representable as a duration")
	}
	return nil
}

func (p FreshnessPolicy) contentDigest() (string, error) {
	identity := struct {
		PolicyID                     string `json:"policy_id"`
		MaximumObservationAgeSeconds int64  `json:"maximum_observation_age_seconds"`
		MaximumApprovalAgeSeconds    int64  `json:"maximum_approval_age_seconds"`
	}{p.ID, p.MaximumObservationAgeSeconds, p.MaximumApprovalAgeSeconds}
	return canonjson.Digest(identity)
}

// Record is the exact canonical countersign witness wire tree.
type Record struct {
	Schema       string           `json:"schema"`
	Repository   string           `json:"repository"`
	Forge        string           `json:"forge"`
	ChangeID     string           `json:"change_id"`
	CandidateSHA string           `json:"candidate_sha"`
	Obligation   RecordObligation `json:"obligation"`
	Freshness    RecordFreshness  `json:"freshness"`
	Approvals    []ApprovalRecord `json:"approvals"`
	Reduction    Reduction        `json:"reduction"`
	Verdict      Verdict          `json:"verdict"`
	Witnesses    []string         `json:"witnesses"`
	Digest       string           `json:"digest"`
}

// RecordObligation is the wire-level obligation plus its sealed profile binding.
type RecordObligation struct {
	Transition              string         `json:"transition"`
	Scheme                  string         `json:"scheme"`
	Kind                    string         `json:"kind"`
	Role                    string         `json:"role"`
	RequiredCount           int            `json:"required_count"`
	GovernanceProfileID     string         `json:"governance_profile_id"`
	GovernanceProfileDigest string         `json:"governance_profile_digest"`
	SeparationRule          SeparationRule `json:"separation_rule"`
}

// RecordFreshness is the wire-level policy, evaluation, and snapshot binding.
type RecordFreshness struct {
	PolicyID                     string `json:"policy_id"`
	PolicyDigest                 string `json:"policy_digest"`
	EvaluatedAt                  string `json:"evaluated_at"`
	ObservedAt                   string `json:"observed_at"`
	MaximumObservationAgeSeconds int64  `json:"maximum_observation_age_seconds"`
	MaximumApprovalAgeSeconds    int64  `json:"maximum_approval_age_seconds"`
	ProviderSnapshotID           string `json:"provider_snapshot_id"`
}

// ApprovalRecord retains one provider row and its kernel resolution evidence.
type ApprovalRecord struct {
	ApprovalID          string                  `json:"approval_id"`
	ApprovalRef         string                  `json:"approval_ref"`
	State               forge.ApprovalState     `json:"state"`
	ApprovedAt          string                  `json:"approved_at"`
	UpdatedAt           string                  `json:"updated_at"`
	CandidateSHA        string                  `json:"candidate_sha"`
	PrincipalResolution gp.PrincipalResolution  `json:"principal_resolution"`
	ProviderWitnesses   []forge.ProviderWitness `json:"provider_witnesses"`
}

// Reduction is the deterministic distinct-principal countersign reduction.
type Reduction struct {
	EligibleApprovalIDs  []string         `json:"eligible_approval_ids"`
	DistinctPrincipalIDs []gp.PrincipalID `json:"distinct_principal_ids"`
	EligibleCount        int              `json:"eligible_count"`
	RequiredCount        int              `json:"required_count"`
	FreshnessVerdict     Verdict          `json:"freshness_verdict"`
	SeparationVerdict    Verdict          `json:"separation_verdict"`
}

func validateRecord(record Record) error {
	if record.Schema != SchemaID {
		return fmt.Errorf("countersign: schema must be %q", SchemaID)
	}
	for field, value := range map[string]string{"repository": record.Repository, "forge": record.Forge, "change_id": record.ChangeID} {
		if err := requireText(field, value); err != nil {
			return err
		}
	}
	if err := validateSHA("candidate_sha", record.CandidateSHA); err != nil {
		return err
	}
	if err := validateRecordObligation(record.Obligation); err != nil {
		return err
	}
	if err := validateRecordFreshness(record.Freshness); err != nil {
		return err
	}
	if record.Approvals == nil {
		return fmt.Errorf("countersign: approvals must be non-null")
	}
	if record.Reduction.EligibleApprovalIDs == nil {
		return fmt.Errorf("countersign: eligible_approval_ids must be non-null")
	}
	if record.Reduction.DistinctPrincipalIDs == nil {
		return fmt.Errorf("countersign: distinct_principal_ids must be non-null")
	}
	if record.Witnesses == nil {
		return fmt.Errorf("countersign: witnesses must be non-null")
	}
	if err := validateApprovals(record); err != nil {
		return err
	}
	if err := validateReduction(record); err != nil {
		return err
	}
	if err := record.Verdict.validate(); err != nil {
		return err
	}
	if err := validateStringSet("witnesses", record.Witnesses, true); err != nil {
		return err
	}
	provenReduction := record.Reduction.EligibleCount >= record.Reduction.RequiredCount && record.Reduction.FreshnessVerdict == VerdictProven && record.Reduction.SeparationVerdict == VerdictProven
	if (record.Verdict == VerdictProven) != provenReduction {
		return fmt.Errorf("countersign: verdict contradicts reduction")
	}
	if err := validateDigest("digest", record.Digest); err != nil {
		return err
	}
	want, err := recordDigest(record)
	if err != nil {
		return err
	}
	if record.Digest != want {
		return fmt.Errorf("countersign: digest does not match canonical record")
	}
	return nil
}

func validateRecordObligation(o RecordObligation) error {
	input := Obligation{
		Transition: o.Transition, Scheme: o.Scheme, Kind: o.Kind, Role: o.Role,
		RequiredCount: o.RequiredCount, SeparationRule: o.SeparationRule,
	}
	if err := input.validate(); err != nil {
		return err
	}
	if err := gp.ValidateID(o.GovernanceProfileID); err != nil {
		return fmt.Errorf("countersign: governance profile id: %w", err)
	}
	return validateDigest("governance_profile_digest", o.GovernanceProfileDigest)
}

func validateRecordFreshness(f RecordFreshness) error {
	p := FreshnessPolicy{ID: f.PolicyID, Digest: f.PolicyDigest, MaximumObservationAgeSeconds: f.MaximumObservationAgeSeconds, MaximumApprovalAgeSeconds: f.MaximumApprovalAgeSeconds}
	if err := p.Validate(); err != nil {
		return err
	}
	if _, err := normalizedTime("evaluated_at", f.EvaluatedAt); err != nil {
		return err
	}
	if _, err := normalizedTime("observed_at", f.ObservedAt); err != nil {
		return err
	}
	return validateDigest("provider_snapshot_id", f.ProviderSnapshotID)
}

func validateApprovals(record Record) error {
	seen := make(map[string]bool, len(record.Approvals))
	previousKey := ""
	for i, row := range record.Approvals {
		prefix := fmt.Sprintf("approvals[%d]", i)
		if err := requireText(prefix+".approval_id", row.ApprovalID); err != nil {
			return err
		}
		if seen[row.ApprovalID] {
			return fmt.Errorf("countersign: duplicate approval_id %q", row.ApprovalID)
		}
		seen[row.ApprovalID] = true
		if err := requireText(prefix+".approval_ref", row.ApprovalRef); err != nil {
			return err
		}
		switch row.State {
		case forge.ApprovalActive, forge.ApprovalDismissed, forge.ApprovalRevoked:
		default:
			return fmt.Errorf("countersign: %s: unknown approval state %q", prefix, row.State)
		}
		approved, err := normalizedTime(prefix+".approved_at", row.ApprovedAt)
		if err != nil {
			return err
		}
		updated, err := normalizedTime(prefix+".updated_at", row.UpdatedAt)
		if err != nil {
			return err
		}
		if updated.Before(approved) {
			return fmt.Errorf("countersign: %s.updated_at precedes approved_at", prefix)
		}
		if err := validateSHA(prefix+".candidate_sha", row.CandidateSHA); err != nil {
			return err
		}
		if err := validateResolutionEvidence(prefix+".principal_resolution", row.PrincipalResolution); err != nil {
			return err
		}
		if row.ProviderWitnesses == nil {
			return fmt.Errorf("countersign: %s.provider_witnesses must be non-null", prefix)
		}
		for j, witness := range row.ProviderWitnesses {
			if err := requireText(fmt.Sprintf("%s.provider_witnesses[%d].name", prefix, j), witness.Name); err != nil {
				return err
			}
			if err := requireText(fmt.Sprintf("%s.provider_witnesses[%d].value", prefix, j), witness.Value); err != nil {
				return err
			}
			if j > 0 {
				prev := row.ProviderWitnesses[j-1]
				if witness.Name < prev.Name || (witness.Name == prev.Name && witness.Value < prev.Value) {
					return fmt.Errorf("countersign: %s.provider_witnesses are not canonically ordered", prefix)
				}
			}
		}
		principalKey, err := gp.CanonicalPrincipalID(row.PrincipalResolution.Claim.TrustSource, row.PrincipalResolution.Claim.Subject)
		if err != nil {
			return err
		}
		key := string(principalKey) + "\x00" + row.ApprovalID
		if i > 0 && key <= previousKey {
			return fmt.Errorf("countersign: approvals are not canonically ordered")
		}
		previousKey = key
	}
	return nil
}

func validateResolutionEvidence(field string, resolution gp.PrincipalResolution) error {
	if err := resolution.State.Validate(); err != nil {
		return fmt.Errorf("countersign: %s: %w", field, err)
	}
	if err := resolution.Claim.Validate(); err != nil {
		return fmt.Errorf("countersign: %s: %w", field, err)
	}
	derived, err := gp.CanonicalPrincipalID(resolution.Claim.TrustSource, resolution.Claim.Subject)
	if err != nil {
		return err
	}
	if resolution.State == gp.ResolutionAuthenticated {
		if resolution.PrincipalID != derived {
			return fmt.Errorf("countersign: %s authenticated principal id does not match claim", field)
		}
	} else if resolution.PrincipalID != "" {
		return fmt.Errorf("countersign: %s non-authenticated resolution carries principal id", field)
	}
	if resolution.Witnesses == nil {
		return fmt.Errorf("countersign: %s.witnesses must be non-null", field)
	}
	if len(resolution.Witnesses) == 0 {
		return fmt.Errorf("countersign: %s.witnesses must preserve kernel evidence", field)
	}
	for i, witness := range resolution.Witnesses {
		if err := requireText(fmt.Sprintf("%s.witnesses[%d].code", field, i), witness.Code); err != nil {
			return err
		}
		if err := requireText(fmt.Sprintf("%s.witnesses[%d].source_id", field, i), witness.SourceID); err != nil {
			return err
		}
		if i > 0 && !kernelWitnessLess(resolution.Witnesses[i-1], witness) {
			return fmt.Errorf("countersign: %s.witnesses are not strictly canonically ordered", field)
		}
	}
	return nil
}

func kernelWitnessLess(a, b gp.Witness) bool {
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	if a.SourceID != b.SourceID {
		return a.SourceID < b.SourceID
	}
	if a.EvidenceDigest != b.EvidenceDigest {
		return a.EvidenceDigest < b.EvidenceDigest
	}
	return a.Detail < b.Detail
}

func validateReduction(record Record) error {
	r := record.Reduction
	evaluated, err := normalizedTime("freshness.evaluated_at", record.Freshness.EvaluatedAt)
	if err != nil {
		return err
	}
	observationFresh := withinAge(evaluated, record.Freshness.ObservedAt, record.Freshness.MaximumObservationAgeSeconds)
	if !observationFresh && r.FreshnessVerdict != VerdictViolated {
		return fmt.Errorf("countersign: freshness verdict does not preserve stale or future observation")
	}
	if r.RequiredCount != record.Obligation.RequiredCount {
		return fmt.Errorf("countersign: reduction required_count does not match obligation")
	}
	if r.EligibleCount != len(r.DistinctPrincipalIDs) {
		return fmt.Errorf("countersign: eligible_count does not equal distinct principal count")
	}
	if err := r.FreshnessVerdict.validate(); err != nil {
		return fmt.Errorf("countersign: freshness verdict: %w", err)
	}
	if err := r.SeparationVerdict.validate(); err != nil {
		return fmt.Errorf("countersign: separation verdict: %w", err)
	}
	if record.Obligation.SeparationRule == SeparationNone && r.SeparationVerdict != VerdictProven {
		return fmt.Errorf("countersign: none separation must be proven")
	}
	if err := validateUniqueStrings("eligible_approval_ids", r.EligibleApprovalIDs); err != nil {
		return err
	}
	for i, id := range r.DistinctPrincipalIDs {
		if err := id.Validate(); err != nil {
			return fmt.Errorf("countersign: distinct_principal_ids[%d]: %w", i, err)
		}
		if i > 0 && id <= r.DistinctPrincipalIDs[i-1] {
			return fmt.Errorf("countersign: distinct_principal_ids are not sorted and unique")
		}
	}
	rows := make(map[string]ApprovalRecord, len(record.Approvals))
	position := make(map[string]int, len(record.Approvals))
	for i, row := range record.Approvals {
		rows[row.ApprovalID], position[row.ApprovalID] = row, i
	}
	principalSet := make(map[gp.PrincipalID]bool)
	previousPosition := -1
	for _, id := range r.EligibleApprovalIDs {
		row, ok := rows[id]
		if !ok {
			return fmt.Errorf("countersign: eligible approval id %q has no approval row", id)
		}
		if position[id] <= previousPosition {
			return fmt.Errorf("countersign: eligible_approval_ids do not retain canonical approval order")
		}
		previousPosition = position[id]
		if row.State != forge.ApprovalActive || row.CandidateSHA != record.CandidateSHA || row.PrincipalResolution.State != gp.ResolutionAuthenticated {
			return fmt.Errorf("countersign: eligible approval %q contradicts retained operands", id)
		}
		if !observationFresh || !approvalFresh(evaluated, row.ApprovedAt, row.UpdatedAt, record.Freshness.MaximumApprovalAgeSeconds) {
			return fmt.Errorf("countersign: eligible approval %q contradicts recorded freshness", id)
		}
		if err := validateEligibleApprovalWitnesses(record, row); err != nil {
			return err
		}
		principalSet[row.PrincipalResolution.PrincipalID] = true
	}
	wantPrincipals := make([]gp.PrincipalID, 0, len(principalSet))
	for id := range principalSet {
		wantPrincipals = append(wantPrincipals, id)
	}
	sort.Slice(wantPrincipals, func(i, j int) bool { return wantPrincipals[i] < wantPrincipals[j] })
	if len(wantPrincipals) != len(r.DistinctPrincipalIDs) {
		return fmt.Errorf("countersign: distinct principals do not match eligible approvals")
	}
	for i := range wantPrincipals {
		if wantPrincipals[i] != r.DistinctPrincipalIDs[i] {
			return fmt.Errorf("countersign: distinct principals do not match eligible approvals")
		}
	}
	return nil
}

func validateEligibleApprovalWitnesses(record Record, row ApprovalRecord) error {
	role := roleMembershipWitness(row.ApprovalID, record.Obligation.Role, VerdictProven)
	rolePrefix := fmt.Sprintf("role-membership:approval_id=%q:", row.ApprovalID)
	if err := requireExactApprovalWitness(record.Witnesses, rolePrefix, role); err != nil {
		return fmt.Errorf("countersign: eligible approval %q role membership: %w", row.ApprovalID, err)
	}
	if record.Obligation.SeparationRule == SeparationDifferentFromAuthor {
		separation := approvalSeparationWitness(row.ApprovalID, string(gp.AuthorizationAuthorized))
		separationPrefix := fmt.Sprintf("approval-separation:approval_id=%q:", row.ApprovalID)
		if err := requireExactApprovalWitness(record.Witnesses, separationPrefix, separation); err != nil {
			return fmt.Errorf("countersign: eligible approval %q separation: %w", row.ApprovalID, err)
		}
	}
	return nil
}

func requireExactApprovalWitness(witnesses []string, prefix, want string) error {
	found := false
	for _, witness := range witnesses {
		if !strings.HasPrefix(witness, prefix) {
			continue
		}
		if witness != want {
			return fmt.Errorf("retained witness %q contradicts required witness %q", witness, want)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("required retained witness %q is absent", want)
	}
	return nil
}

func roleMembershipWitness(approvalID, role string, verdict Verdict) string {
	return fmt.Sprintf("role-membership:approval_id=%q:role=%q:verdict=%q", approvalID, role, verdict)
}

func approvalSeparationWitness(approvalID, state string) string {
	return fmt.Sprintf("approval-separation:approval_id=%q:state=%q", approvalID, state)
}

func validateUniqueStrings(field string, values []string) error {
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		if err := requireText(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
		if seen[value] {
			return fmt.Errorf("countersign: %s must be deduplicated", field)
		}
		seen[value] = true
	}
	return nil
}

func validateStringSet(field string, values []string, requireCode bool) error {
	for i, value := range values {
		if err := requireText(fmt.Sprintf("%s[%d]", field, i), value); err != nil {
			return err
		}
		if requireCode && !witnessCodeRe.MatchString(value) {
			return fmt.Errorf("countersign: %s[%d] lacks a stable code prefix", field, i)
		}
		if i > 0 && value <= values[i-1] {
			return fmt.Errorf("countersign: %s must be sorted and deduplicated", field)
		}
	}
	return nil
}

var witnessCodeRe = regexp.MustCompile(`^[a-z][a-z0-9-]*:`)

func requireText(field, value string) error {
	if value == "" || strings.TrimSpace(value) != value {
		return fmt.Errorf("countersign: %s must be nonempty without surrounding whitespace", field)
	}
	return nil
}

func validateSHA(field, value string) error {
	if len(value) != 40 && len(value) != 64 {
		return fmt.Errorf("countersign: %s must be a full 40- or 64-character commit SHA", field)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf("countersign: %s must be lowercase hexadecimal", field)
	}
	if _, err := hex.DecodeString(value); err != nil {
		return fmt.Errorf("countersign: %s must be hexadecimal: %w", field, err)
	}
	return nil
}

var digestRe = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func validateDigest(field, value string) error {
	if !digestRe.MatchString(value) {
		return fmt.Errorf("countersign: %s must be a canonical sha256 digest", field)
	}
	return nil
}

func normalizedTime(field, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("countersign: %s must be RFC3339Nano: %w", field, err)
	}
	if parsed.UTC().Format(time.RFC3339Nano) != value {
		return time.Time{}, fmt.Errorf("countersign: %s must be normalized UTC RFC3339Nano", field)
	}
	return parsed, nil
}
