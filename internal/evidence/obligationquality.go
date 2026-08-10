package evidence

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jyang234/verdi/internal/artifact"
)

// ObligationQualityAdoptionCommit is the exact PR #290 owner-merge that made
// the obligation-quality plan and SI-71..SI-74 reachable. Legacy absence is
// interpreted prospectively relative to this first-parent authority cutoff.
const ObligationQualityAdoptionCommit = "c433f8d1a2ee12cc613b7a9e187c8e0947562a5f"

// ObligationStructuralState is the closed structural assessment for one
// declared (AC, evidence-kind) pair.
type ObligationStructuralState string

const (
	ObligationElaborated           ObligationStructuralState = "elaborated"
	ObligationUnresolvedDesignDebt ObligationStructuralState = "unresolved-design-debt"
	ObligationLegacyUnelaborated   ObligationStructuralState = "legacy-unelaborated"
	ObligationMissing              ObligationStructuralState = "missing"
)

// ObligationMatchState is one candidate evidence record's quality result.
type ObligationMatchState string

const (
	ObligationMatched             ObligationMatchState = "matched"
	ObligationViolatedWithWitness ObligationMatchState = "violated-with-witness"
	ObligationUnproven            ObligationMatchState = "unproven"
)

// ObligationMatchReason is the closed, precedence-ordered reason why a
// positive candidate could not match an elaborated obligation.
type ObligationMatchReason string

const (
	ObligationReasonProducerMissing   ObligationMatchReason = "producer-missing"
	ObligationReasonProducerMismatch  ObligationMatchReason = "producer-mismatch"
	ObligationReasonSourceMismatch    ObligationMatchReason = "source-mismatch"
	ObligationReasonSourceRefMissing  ObligationMatchReason = "source-ref-missing"
	ObligationReasonSourceRefMismatch ObligationMatchReason = "source-ref-mismatch"
	ObligationReasonFreshnessStale    ObligationMatchReason = "freshness-stale"
	ObligationReasonFreshnessUnproven ObligationMatchReason = "freshness-unproven"
)

// ObligationEvaluationClass says whether an evaluation is covered by the
// compatibility side of the prospective adoption cutoff.
type ObligationEvaluationClass string

const (
	ObligationEvaluationHistorical   ObligationEvaluationClass = "historical"
	ObligationEvaluationPostAdoption ObligationEvaluationClass = "post-adoption"
)

// ObligationAncestryReader is defined at the consumer. It is the sole Git
// capability quality evaluation needs for adoption and spec freshness.
type ObligationAncestryReader interface {
	IsAncestor(ctx context.Context, dir, ancestor, descendant string) (bool, error)
}

// ClassifyObligationEvaluation applies the adoption cutoff. Equal and
// ancestors are historical; descendants are post-adoption. Divergence and
// unavailable ancestry are operational because guessing would silently pick
// an authority regime.
func ClassifyObligationEvaluation(ctx context.Context, git ObligationAncestryReader, root, evaluationCommit string) (ObligationEvaluationClass, error) {
	if evaluationCommit == "" {
		return "", fmt.Errorf("evidence: obligation quality evaluation commit is required")
	}
	if evaluationCommit == ObligationQualityAdoptionCommit {
		return ObligationEvaluationHistorical, nil
	}
	if git == nil {
		return "", fmt.Errorf("evidence: obligation quality ancestry reader is required")
	}
	before, err := git.IsAncestor(ctx, root, evaluationCommit, ObligationQualityAdoptionCommit)
	if err != nil {
		return "", fmt.Errorf("evidence: proving obligation quality evaluation %s is at or before adoption %s: %w", evaluationCommit, ObligationQualityAdoptionCommit, err)
	}
	if before {
		return ObligationEvaluationHistorical, nil
	}
	after, err := git.IsAncestor(ctx, root, ObligationQualityAdoptionCommit, evaluationCommit)
	if err != nil {
		return "", fmt.Errorf("evidence: proving obligation quality adoption %s is before evaluation %s: %w", ObligationQualityAdoptionCommit, evaluationCommit, err)
	}
	if after {
		return ObligationEvaluationPostAdoption, nil
	}
	return "", fmt.Errorf("evidence: obligation quality evaluation %s diverges from adoption %s", evaluationCommit, ObligationQualityAdoptionCommit)
}

// ObligationAssessmentInput identifies one declared pair and, optionally, one
// candidate evidence record to match against its declaration.
type ObligationAssessmentInput struct {
	StoreRoot         string
	SpecName          string
	ACID              string
	Kind              artifact.EvidenceKind
	Record            *artifact.Evidence
	EvaluationCommit  string
	SpecLandingCommit string
	Git               ObligationAncestryReader
}

// ObligationAssessment is the one structural-and-record result shared by the
// fold, build gate, journey projection, and backend consumers.
type ObligationAssessment struct {
	StructuralState ObligationStructuralState
	MatchState      ObligationMatchState
	Reason          ObligationMatchReason
	WitnessPath     string
	Quality         *artifact.ObligationQuality
	Violating       *artifact.Evidence
}

// AssessObligation strict-loads the exact convention path for a declared pair
// and, when supplied a record, applies exact producer/source/freshness matching.
// A failing record is preserved as a witnessed violation regardless of design
// debt; debt may block positive satisfaction but never hide negative evidence.
func AssessObligation(ctx context.Context, in ObligationAssessmentInput) (ObligationAssessment, error) {
	rel := filepath.ToSlash(filepath.Join(".verdi", "obligations", in.SpecName, in.ACID+"--"+string(in.Kind)+".md"))
	result := ObligationAssessment{WitnessPath: rel, MatchState: ObligationUnproven}
	path := filepath.Join(in.StoreRoot, filepath.FromSlash(rel))
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.StructuralState = ObligationMissing
			return preserveViolation(result, in.Record), nil
		}
		return ObligationAssessment{}, fmt.Errorf("evidence: reading obligation quality %s: %w", path, err)
	}
	fm, body, err := artifact.SplitFrontmatter(raw)
	if err != nil {
		return ObligationAssessment{}, fmt.Errorf("evidence: obligation quality %s: %w", path, err)
	}
	obligation, err := artifact.DecodeObligation(fm)
	if err != nil {
		return ObligationAssessment{}, fmt.Errorf("evidence: obligation quality %s: %w", path, err)
	}
	if obligation.ForKind != in.Kind {
		return ObligationAssessment{}, fmt.Errorf("evidence: obligation quality %s declares for_kind %q, want %q", path, obligation.ForKind, in.Kind)
	}
	result.Quality = obligation.Quality
	switch {
	case strings.Contains(string(body), UnauthoredObligationMarker):
		result.StructuralState = ObligationUnresolvedDesignDebt
	case obligation.Quality == nil:
		result.StructuralState = ObligationLegacyUnelaborated
	case obligation.Quality.State == artifact.ObligationQualityUnresolved:
		result.StructuralState = ObligationUnresolvedDesignDebt
	case obligation.Quality.State == artifact.ObligationQualityElaborated:
		result.StructuralState = ObligationElaborated
	default:
		return ObligationAssessment{}, fmt.Errorf("evidence: obligation quality %s decoded unknown state %q", path, obligation.Quality.State)
	}

	return MatchObligation(ctx, result, in)
}

// MatchObligation applies one candidate record to an already-loaded pair
// assessment. Fold uses it to load the obligation exactly once, then evaluate
// every current record without a second filesystem read.
func MatchObligation(ctx context.Context, result ObligationAssessment, in ObligationAssessmentInput) (ObligationAssessment, error) {
	result.MatchState = ObligationUnproven
	result.Reason = ""
	result.Violating = nil
	if preserved := preserveViolation(result, in.Record); preserved.MatchState == ObligationViolatedWithWitness {
		return preserved, nil
	}
	if result.StructuralState != ObligationElaborated {
		return result, nil
	}
	return matchElaborated(ctx, in, result)
}

func preserveViolation(result ObligationAssessment, record *artifact.Evidence) ObligationAssessment {
	if record != nil && record.Verdict == artifact.VerdictFail {
		copy := *record
		result.MatchState = ObligationViolatedWithWitness
		result.Violating = &copy
	}
	return result
}

func matchElaborated(ctx context.Context, in ObligationAssessmentInput, result ObligationAssessment) (ObligationAssessment, error) {
	q := result.Quality
	if in.Kind == artifact.EvidenceAttestation {
		// verdi.evidence/v1 has neither an authenticated principal nor a
		// governed-attestation identity. Never parse free-text witness fields as
		// either identity.
		result.Reason = ObligationReasonSourceRefMissing
		return result, nil
	}
	if in.Record == nil || in.Record.Producer == "" {
		result.Reason = ObligationReasonProducerMissing
		return result, nil
	}
	if in.Record.Producer != q.Producer.Ref {
		result.Reason = ObligationReasonProducerMismatch
		return result, nil
	}
	if in.Record.Provenance.Source != artifact.SourceCI {
		result.Reason = ObligationReasonSourceMismatch
		return result, nil
	}
	if in.Record.Provenance.Job == "" {
		result.Reason = ObligationReasonSourceRefMissing
		return result, nil
	}
	if in.Record.Provenance.Job != q.AuthoritativeSource.Ref {
		result.Reason = ObligationReasonSourceRefMismatch
		return result, nil
	}

	for _, invalidator := range q.Freshness.InvalidatedBy {
		switch invalidator {
		case artifact.ObligationInvalidatorCode:
			if in.EvaluationCommit == "" {
				return ObligationAssessment{}, fmt.Errorf("evidence: code freshness requires an evaluation commit for %s/%s", in.ACID, in.Kind)
			}
			if in.Record.Provenance.Commit != in.EvaluationCommit {
				result.Reason = ObligationReasonFreshnessStale
				return result, nil
			}
		case artifact.ObligationInvalidatorSpec:
			if in.SpecLandingCommit == "" || in.Record.Provenance.Commit == "" {
				return ObligationAssessment{}, fmt.Errorf("evidence: spec freshness requires obligation source landing and evidence commit for %s/%s", in.ACID, in.Kind)
			}
			if in.Record.Provenance.Commit != in.SpecLandingCommit {
				if in.Git == nil {
					return ObligationAssessment{}, fmt.Errorf("evidence: spec freshness ancestry reader is required for %s/%s", in.ACID, in.Kind)
				}
				fresh, err := in.Git.IsAncestor(ctx, in.StoreRoot, in.SpecLandingCommit, in.Record.Provenance.Commit)
				if err != nil {
					return ObligationAssessment{}, fmt.Errorf("evidence: proving spec freshness for %s/%s: %w", in.ACID, in.Kind, err)
				}
				if !fresh {
					result.Reason = ObligationReasonFreshnessStale
					return result, nil
				}
			}
		case artifact.ObligationInvalidatorDependency, artifact.ObligationInvalidatorEnvironment, artifact.ObligationInvalidatorPolicy:
			result.Reason = ObligationReasonFreshnessUnproven
			return result, nil
		}
	}
	result.MatchState = ObligationMatched
	return result, nil
}
