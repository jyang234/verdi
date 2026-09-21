package journey

import (
	"fmt"
	"sort"
)

// BlockerClass is one of the journey's ergonomics-only classifications of a
// blocker (spec AC-1, DC-4): mechanical work can usually be executed or
// verified; judgmental work requires an authorized conclusion; governance
// work requires identity or approval; external-wait requires a forge, CI,
// provider, or clock-bound condition; unknown is the first-class failure
// to classify. Class controls ergonomics, never truth — a blocker's Class
// is always the fixed class of its ReasonCode (enforced by Record.Validate)
// and may never be reassigned to a different class.
type BlockerClass string

// The closed BlockerClass vocabulary (DC-4's four classes plus explicit
// unknown).
const (
	ClassMechanical   BlockerClass = "mechanical"
	ClassJudgmental   BlockerClass = "judgmental"
	ClassGovernance   BlockerClass = "governance"
	ClassExternalWait BlockerClass = "external-wait"
	ClassUnknown      BlockerClass = "unknown"
)

// validBlockerClasses is the closed set BlockerClass values must belong to.
var validBlockerClasses = map[BlockerClass]bool{
	ClassMechanical:   true,
	ClassJudgmental:   true,
	ClassGovernance:   true,
	ClassExternalWait: true,
	ClassUnknown:      true,
}

// ReasonCode is a journey blocker's stable, closed identifier (AC-1: "each
// with stable reason code"). Human-facing prose may vary; a ReasonCode
// never does. Every ReasonCode has exactly one fixed BlockerClass — see
// Class.
type ReasonCode string

// The closed v1 ReasonCode vocabulary. Each code's fixed class is recorded
// in reasonClasses and returned by Class.
//
// The six eventual-blocker codes (spec/readiness-recovery ac-1, ruling
// R-RR1-1) each name one of the six eventual-closure-blocker sources
// deriveEventual (eventual.go) evaluates: a feature's acceptance-time stub
// is unreconciled; a feature AC's mandatory outcome floor is unsatisfied;
// a declared open question is claimed by an unresolved spike stub; a
// policy-conflict report's mechanical row is not proven; its semantic row
// is not proven; or one of its exemption resolutions is not proven
// effective. R-RR1-1's note: conflict-row-unresolved would carry a
// PER-ROW class (mechanical rows mechanical, semantic rows judgmental),
// which violates reasonClasses' one-fixed-class-per-code invariant, so two
// codes replace it here — conflict-mechanical-unresolved and
// conflict-semantic-unresolved — keeping every code's class fixed.
const (
	ReasonDefaultBranchUnresolved       ReasonCode = "default-branch-unresolved"
	ReasonLifecycleStateUnproven        ReasonCode = "lifecycle-state-unproven"
	ReasonForgeFactsUnavailable         ReasonCode = "forge-facts-unavailable"
	ReasonPrincipalResolutionUnproven   ReasonCode = "principal-resolution-unproven"
	ReasonObligationAuthorVouchUnproven ReasonCode = "obligation-author-vouch-unproven"
	ReasonObligationCountersignUnproven ReasonCode = "obligation-countersign-unproven"
	ReasonObligationDesignUnresolved    ReasonCode = "obligation-design-unresolved"
	ReasonObligationFoldGreenUnproven   ReasonCode = "obligation-fold-green-unproven"
	ReasonObligationUnknownKind         ReasonCode = "obligation-unknown-kind"

	// ReasonStubUnreconciled: a feature's acceptance-time stub is neither
	// realized-by a closed implementing story nor withdrawn with a note
	// (03 §Stub reconciliation).
	ReasonStubUnreconciled ReasonCode = "stub-unreconciled"
	// ReasonOutcomeFloorUnsatisfied: a feature acceptance criterion's
	// mandatory outcome floor (an authored attestation, or a current
	// passing outcome record) is not yet satisfied (03 §The feature fold).
	ReasonOutcomeFloorUnsatisfied ReasonCode = "outcome-floor-unsatisfied"
	// ReasonQuestionClaimedBySpike: a declared open question is named by a
	// spike stub's resolves edge, and that spike has not yet resolved it.
	ReasonQuestionClaimedBySpike ReasonCode = "question-claimed-by-spike"
	// ReasonConflictMechanicalUnresolved: a policy-conflict report's
	// mechanical row is not proven (violated-with-witness or unproven).
	ReasonConflictMechanicalUnresolved ReasonCode = "conflict-mechanical-unresolved"
	// ReasonConflictSemanticUnresolved: a policy-conflict report's
	// semantic row is not proven (violated-with-witness or unproven).
	ReasonConflictSemanticUnresolved ReasonCode = "conflict-semantic-unresolved"
	// ReasonExemptionIneffective: a policy-conflict report's exemption
	// resolution is not proven both bound and fresh.
	ReasonExemptionIneffective ReasonCode = "exemption-ineffective"
)

// reasonClasses is the single source of truth binding each ReasonCode to
// its fixed BlockerClass. A blocker's Class field must equal this mapping
// for its Reason — never forced into a different class (DC-4).
var reasonClasses = map[ReasonCode]BlockerClass{
	ReasonDefaultBranchUnresolved:       ClassUnknown,
	ReasonLifecycleStateUnproven:        ClassUnknown,
	ReasonForgeFactsUnavailable:         ClassExternalWait,
	ReasonPrincipalResolutionUnproven:   ClassGovernance,
	ReasonObligationAuthorVouchUnproven: ClassJudgmental,
	ReasonObligationCountersignUnproven: ClassGovernance,
	ReasonObligationDesignUnresolved:    ClassMechanical,
	ReasonObligationFoldGreenUnproven:   ClassMechanical,
	ReasonObligationUnknownKind:         ClassUnknown,
	ReasonStubUnreconciled:              ClassMechanical,
	ReasonOutcomeFloorUnsatisfied:       ClassJudgmental,
	ReasonQuestionClaimedBySpike:        ClassMechanical,
	ReasonConflictMechanicalUnresolved:  ClassMechanical,
	ReasonConflictSemanticUnresolved:    ClassJudgmental,
	ReasonExemptionIneffective:          ClassGovernance,
}

// Class returns r's fixed blocker class. An unrecognized code is an error
// — reason-code classification fails closed rather than defaulting to any
// class, including unknown.
func (r ReasonCode) Class() (BlockerClass, error) {
	c, ok := reasonClasses[r]
	if !ok {
		return "", fmt.Errorf("journey: unknown reason code %q", string(r))
	}
	return c, nil
}

// ReasonCodes returns every registered v1 reason code, sorted, for display
// and audit enumeration.
func ReasonCodes() []ReasonCode {
	out := make([]ReasonCode, 0, len(reasonClasses))
	for c := range reasonClasses {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
