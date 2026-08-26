package countersign

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jyang234/verdi/internal/forge"
	gp "github.com/jyang234/verdi/internal/governanceprincipal"
)

// PrincipalResolver is the consumer-defined governance-kernel port.
type PrincipalResolver interface {
	Resolve(context.Context, gp.Profile, gp.PrincipalClaim) (gp.PrincipalResolution, error)
}

// Request contains only sealed or validated inputs and an explicit evaluation
// stamp; resolution never reads ambient state or the wall clock.
type Request struct {
	Snapshot          forge.ApprovalSnapshot
	LocalCandidateSHA string
	Profile           gp.Profile
	TrustSourceID     string
	Obligation        Obligation
	FreshnessPolicy   FreshnessPolicy
	EvaluatedAt       string
	CandidateAuthor   *gp.PrincipalResolution
	Resolver          PrincipalResolver
}

type evaluatedApproval struct {
	record             ApprovalRecord
	resolution         gp.PrincipalResolution
	sortID             gp.PrincipalID
	eligible           bool
	potential          bool
	staleRelevant      bool
	separationViolated bool
	separationUnproven bool
}

// Resolve reduces every retained provider row into a canonical witness.
func Resolve(ctx context.Context, request Request) (Record, error) {
	profileDigest, evaluated, observationFresh, candidateMatch, err := validateRequest(request)
	if err != nil {
		return Record{}, err
	}

	var author *gp.PrincipalResolution
	var authorFacts []string
	if request.CandidateAuthor != nil {
		if _, err := gp.EvaluateRelation(*request.CandidateAuthor, *request.CandidateAuthor, gp.RelationSamePrincipal); err != nil {
			return Record{}, fmt.Errorf("countersign: candidate author resolution: %w", err)
		}
		copy := *request.CandidateAuthor
		author = &copy
		authorFacts = append(authorFacts, fmt.Sprintf("candidate-author-resolution:claim_trust_source=%q:claim_subject=%q:state=%q:principal_id=%q", author.Claim.TrustSource, author.Claim.Subject, author.State, author.PrincipalID))
		for _, witness := range author.Witnesses {
			authorFacts = append(authorFacts, fmt.Sprintf("candidate-author-witness:code=%q:source_id=%q:evidence_digest=%q:detail=%q", witness.Code, witness.SourceID, witness.EvidenceDigest, witness.Detail))
		}
	}

	evaluations := make([]evaluatedApproval, 0, len(request.Snapshot.Approvals))
	for _, approval := range request.Snapshot.Approvals {
		claim := gp.PrincipalClaim{TrustSource: request.TrustSourceID, Subject: approval.Actor.Subject}
		resolution, err := request.Resolver.Resolve(ctx, request.Profile, claim)
		if err != nil {
			return Record{}, fmt.Errorf("countersign: resolve approval %q principal: %w", approval.ApprovalID, err)
		}
		if resolution.Claim != claim {
			return Record{}, fmt.Errorf("countersign: resolver returned a claim different from approval %q actor", approval.ApprovalID)
		}
		if _, err := gp.EvaluateRelation(resolution, resolution, gp.RelationSamePrincipal); err != nil {
			return Record{}, fmt.Errorf("countersign: approval %q principal resolution: %w", approval.ApprovalID, err)
		}
		sortID, err := gp.CanonicalPrincipalID(claim.TrustSource, claim.Subject)
		if err != nil {
			return Record{}, err
		}
		evaluations = append(evaluations, evaluatedApproval{record: ApprovalRecord{
			ApprovalID: approval.ApprovalID, ApprovalRef: approval.ApprovalRef, State: approval.State,
			ApprovedAt: approval.ApprovedAt, UpdatedAt: approval.UpdatedAt, CandidateSHA: approval.CandidateSHA,
			PrincipalResolution: evidenceResolution(resolution), ProviderWitnesses: append([]forge.ProviderWitness{}, approval.ProviderWitnesses...),
		}, resolution: resolution, sortID: sortID})
	}
	sort.Slice(evaluations, func(i, j int) bool {
		if evaluations[i].sortID != evaluations[j].sortID {
			return evaluations[i].sortID < evaluations[j].sortID
		}
		return evaluations[i].record.ApprovalID < evaluations[j].record.ApprovalID
	})

	witnesses := make([]string, 0, 4+len(authorFacts)+len(evaluations)*6)
	witnesses = append(witnesses, authorFacts...)
	if candidateMatch {
		witnesses = append(witnesses, fmt.Sprintf("candidate-sha-match:snapshot=%q:local=%q", request.Snapshot.CandidateSHA, request.LocalCandidateSHA))
	} else {
		witnesses = append(witnesses, fmt.Sprintf("candidate-sha-mismatch:snapshot=%q:local=%q", request.Snapshot.CandidateSHA, request.LocalCandidateSHA))
	}
	if observationFresh {
		witnesses = append(witnesses, fmt.Sprintf("observation-freshness:proven:observed_at=%q:evaluated_at=%q:maximum_age_seconds=%d", request.Snapshot.ObservedAt, request.EvaluatedAt, request.FreshnessPolicy.MaximumObservationAgeSeconds))
	} else {
		witnesses = append(witnesses, fmt.Sprintf("observation-freshness:violated-with-witness:observed_at=%q:evaluated_at=%q:maximum_age_seconds=%d", request.Snapshot.ObservedAt, request.EvaluatedAt, request.FreshnessPolicy.MaximumObservationAgeSeconds))
	}

	for i := range evaluations {
		e := &evaluations[i]
		row := e.record
		fresh := approvalFresh(evaluated, row.ApprovedAt, row.UpdatedAt, request.FreshnessPolicy.MaximumApprovalAgeSeconds)
		stateOK := row.State == forge.ApprovalActive
		headOK := row.CandidateSHA == request.Snapshot.CandidateSHA && row.CandidateSHA == request.LocalCandidateSHA
		role, err := gp.HoldsRole(request.Profile, row.PrincipalResolution.Claim, request.Obligation.Role)
		if err != nil {
			return Record{}, fmt.Errorf("countersign: approval %q role query: %w", row.ApprovalID, err)
		}
		witnesses = append(witnesses,
			fmt.Sprintf("approval-state:approval_id=%q:state=%q", row.ApprovalID, row.State),
			fmt.Sprintf("approval-candidate:approval_id=%q:match=%t", row.ApprovalID, headOK),
			fmt.Sprintf("approval-freshness:approval_id=%q:approved_at=%q:updated_at=%q:evaluated_at=%q:maximum_age_seconds=%d:verdict=%q", row.ApprovalID, row.ApprovedAt, row.UpdatedAt, request.EvaluatedAt, request.FreshnessPolicy.MaximumApprovalAgeSeconds, boolVerdict(fresh)),
			fmt.Sprintf("principal-resolution:approval_id=%q:state=%q", row.ApprovalID, row.PrincipalResolution.State),
			fmt.Sprintf("role-membership:approval_id=%q:role=%q:verdict=%q", row.ApprovalID, request.Obligation.Role, boolVerdict(role)),
		)

		providerCandidate := candidateMatch && observationFresh && stateOK && headOK && role
		if !providerCandidate {
			continue
		}
		separation := gp.AuthorizationAuthorized
		if request.Obligation.SeparationRule == SeparationDifferentFromAuthor {
			separation, err = gp.EvaluateRelation(e.resolution, *author, gp.RelationDifferentPrincipal)
			if err != nil {
				return Record{}, fmt.Errorf("countersign: approval %q separation: %w", row.ApprovalID, err)
			}
			witnesses = append(witnesses, fmt.Sprintf("approval-separation:approval_id=%q:state=%q", row.ApprovalID, separation))
		}
		if !fresh {
			e.staleRelevant = e.resolution.State != gp.ResolutionViolated && separation != gp.AuthorizationViolated
			continue
		}
		switch {
		case row.PrincipalResolution.State == gp.ResolutionAuthenticated && separation == gp.AuthorizationAuthorized:
			e.eligible = true
		case separation == gp.AuthorizationUnproven || row.PrincipalResolution.State == gp.ResolutionUnproven:
			e.potential = true
			e.separationUnproven = separation == gp.AuthorizationUnproven
		case separation == gp.AuthorizationViolated:
			e.separationViolated = true
		}
	}

	record := Record{
		Schema: SchemaID, Repository: request.Snapshot.Repository, Forge: request.Snapshot.Forge, ChangeID: request.Snapshot.ChangeID, CandidateSHA: request.LocalCandidateSHA,
		Obligation: RecordObligation{
			Transition: request.Obligation.Transition, Scheme: request.Obligation.Scheme, Kind: request.Obligation.Kind,
			Role: request.Obligation.Role, RequiredCount: request.Obligation.RequiredCount,
			GovernanceProfileID: request.Profile.ID, GovernanceProfileDigest: profileDigest, SeparationRule: request.Obligation.SeparationRule,
		},
		Freshness: RecordFreshness{
			PolicyID: request.FreshnessPolicy.ID, PolicyDigest: request.FreshnessPolicy.Digest,
			EvaluatedAt: request.EvaluatedAt, ObservedAt: request.Snapshot.ObservedAt,
			MaximumObservationAgeSeconds: request.FreshnessPolicy.MaximumObservationAgeSeconds,
			MaximumApprovalAgeSeconds:    request.FreshnessPolicy.MaximumApprovalAgeSeconds,
			ProviderSnapshotID:           request.Snapshot.ProviderSnapshotID,
		},
		Approvals: make([]ApprovalRecord, 0, len(evaluations)),
		Reduction: Reduction{EligibleApprovalIDs: []string{}, DistinctPrincipalIDs: []gp.PrincipalID{}, RequiredCount: request.Obligation.RequiredCount},
		Witnesses: []string{},
	}
	eligiblePrincipals := make(map[gp.PrincipalID]bool)
	possiblePrincipals := make(map[gp.PrincipalID]bool)
	staleRelevant, separationViolated, separationUnproven := false, false, false
	for _, e := range evaluations {
		record.Approvals = append(record.Approvals, e.record)
		if e.eligible {
			record.Reduction.EligibleApprovalIDs = append(record.Reduction.EligibleApprovalIDs, e.record.ApprovalID)
			eligiblePrincipals[e.record.PrincipalResolution.PrincipalID] = true
			possiblePrincipals[e.sortID] = true
		}
		if e.potential {
			possiblePrincipals[e.sortID] = true
		}
		staleRelevant = staleRelevant || e.staleRelevant
		separationViolated = separationViolated || e.separationViolated
		separationUnproven = separationUnproven || e.separationUnproven
	}
	for id := range eligiblePrincipals {
		record.Reduction.DistinctPrincipalIDs = append(record.Reduction.DistinctPrincipalIDs, id)
	}
	sort.Slice(record.Reduction.DistinctPrincipalIDs, func(i, j int) bool {
		return record.Reduction.DistinctPrincipalIDs[i] < record.Reduction.DistinctPrincipalIDs[j]
	})
	record.Reduction.EligibleCount = len(record.Reduction.DistinctPrincipalIDs)
	enough := record.Reduction.EligibleCount >= record.Reduction.RequiredCount
	possibleEnough := len(possiblePrincipals) >= record.Reduction.RequiredCount

	record.Reduction.FreshnessVerdict = VerdictProven
	if !observationFresh || (!enough && staleRelevant) {
		record.Reduction.FreshnessVerdict = VerdictViolated
	}
	record.Reduction.SeparationVerdict = VerdictProven
	if request.Obligation.SeparationRule == SeparationDifferentFromAuthor && !enough {
		switch {
		case separationUnproven:
			record.Reduction.SeparationVerdict = VerdictUnproven
		case separationViolated:
			record.Reduction.SeparationVerdict = VerdictViolated
		}
	}
	switch {
	case !candidateMatch || !observationFresh:
		record.Verdict = VerdictViolated
	case enough && record.Reduction.FreshnessVerdict == VerdictProven && record.Reduction.SeparationVerdict == VerdictProven:
		record.Verdict = VerdictProven
	case possibleEnough:
		record.Verdict = VerdictUnproven
	default:
		record.Verdict = VerdictViolated
	}
	witnesses = append(witnesses,
		fmt.Sprintf("distinct-principal-count:eligible=%d:required=%d", record.Reduction.EligibleCount, record.Reduction.RequiredCount),
		fmt.Sprintf("freshness-reduction:verdict=%q", record.Reduction.FreshnessVerdict),
		fmt.Sprintf("separation-reduction:rule=%q:verdict=%q", request.Obligation.SeparationRule, record.Reduction.SeparationVerdict),
		fmt.Sprintf("countersign-verdict:value=%q", record.Verdict),
	)
	sort.Strings(witnesses)
	record.Witnesses = dedupeStrings(witnesses)
	digest, err := recordDigest(record)
	if err != nil {
		return Record{}, err
	}
	record.Digest = digest
	if err := validateRecord(record); err != nil {
		return Record{}, fmt.Errorf("countersign: constructed invalid record: %w", err)
	}
	return record, nil
}

func validateRequest(request Request) (string, time.Time, bool, bool, error) {
	if err := request.Snapshot.Validate(); err != nil {
		return "", time.Time{}, false, false, fmt.Errorf("countersign: approval snapshot: %w", err)
	}
	if err := validateSHA("local candidate SHA", request.LocalCandidateSHA); err != nil {
		return "", time.Time{}, false, false, err
	}
	profileDigest, err := request.Profile.Digest()
	if err != nil {
		return "", time.Time{}, false, false, fmt.Errorf("countersign: governance profile: %w", err)
	}
	forgeSource, err := gp.HasTrustSourceKind(request.Profile, request.TrustSourceID, gp.TrustSourceForge)
	if err != nil {
		return "", time.Time{}, false, false, fmt.Errorf("countersign: forge trust source: %w", err)
	}
	if !forgeSource {
		return "", time.Time{}, false, false, fmt.Errorf("countersign: trust source %q is not a forge-kind source in profile %q", request.TrustSourceID, request.Profile.ID)
	}
	if err := request.Obligation.validate(); err != nil {
		return "", time.Time{}, false, false, err
	}
	if err := request.FreshnessPolicy.Validate(); err != nil {
		return "", time.Time{}, false, false, err
	}
	evaluated, err := normalizedTime("evaluated_at", request.EvaluatedAt)
	if err != nil {
		return "", time.Time{}, false, false, err
	}
	if request.Resolver == nil {
		return "", time.Time{}, false, false, fmt.Errorf("countersign: resolver is required")
	}
	switch request.Obligation.SeparationRule {
	case SeparationNone:
		if request.CandidateAuthor != nil {
			return "", time.Time{}, false, false, fmt.Errorf("countersign: candidate author must be absent when separation is none")
		}
	case SeparationDifferentFromAuthor:
		if request.CandidateAuthor == nil {
			return "", time.Time{}, false, false, fmt.Errorf("countersign: candidate author is required for different-from-author separation")
		}
	}
	observationFresh := withinAge(evaluated, request.Snapshot.ObservedAt, request.FreshnessPolicy.MaximumObservationAgeSeconds)
	return profileDigest, evaluated, observationFresh, request.Snapshot.CandidateSHA == request.LocalCandidateSHA, nil
}

func approvalFresh(evaluated time.Time, approvedAt, updatedAt string, maximumSeconds int64) bool {
	if maximumSeconds == 0 {
		approved, approvedErr := time.Parse(time.RFC3339Nano, approvedAt)
		updated, updatedErr := time.Parse(time.RFC3339Nano, updatedAt)
		return approvedErr == nil && updatedErr == nil && !approved.After(evaluated) && !updated.After(evaluated)
	}
	return withinAge(evaluated, approvedAt, maximumSeconds) && withinAge(evaluated, updatedAt, maximumSeconds)
}

func withinAge(evaluated time.Time, stamp string, maximumSeconds int64) bool {
	parsed, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil || parsed.After(evaluated) {
		return false
	}
	return evaluated.Sub(parsed) <= time.Duration(maximumSeconds)*time.Second
}

func evidenceResolution(resolution gp.PrincipalResolution) gp.PrincipalResolution {
	return gp.PrincipalResolution{Claim: resolution.Claim, PrincipalID: resolution.PrincipalID, State: resolution.State, Witnesses: append([]gp.Witness{}, resolution.Witnesses...)}
}

func boolVerdict(value bool) Verdict {
	if value {
		return VerdictProven
	}
	return VerdictViolated
}

func dedupeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for i, value := range values {
		if i == 0 || value != values[i-1] {
			out = append(out, value)
		}
	}
	return out
}
