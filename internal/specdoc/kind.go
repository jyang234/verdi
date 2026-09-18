package specdoc

import "fmt"

// Kind selects which sections a document carries (spec/spec-documents dc-3).
type Kind string

const (
	KindSpec  Kind = "spec"
	KindPlan  Kind = "plan"
	KindTasks Kind = "tasks"
)

// SectionID names one document section in its fixed order.
type SectionID string

const (
	SectionIdentity    SectionID = "identity"
	SectionProblem     SectionID = "problem"
	SectionOutcome     SectionID = "outcome"
	SectionDecisions   SectionID = "decisions"
	SectionConstraints SectionID = "constraints"
	SectionCriteria    SectionID = "criteria"
	SectionQuestions   SectionID = "questions"
	SectionPlan        SectionID = "plan"
	SectionEvidence    SectionID = "evidence"
	SectionReadiness   SectionID = "readiness"
)

// ParseKind accepts exactly the three lowercase kind names.
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindSpec, KindPlan, KindTasks:
		return Kind(s), nil
	}
	return "", fmt.Errorf("specdoc: unknown document kind %q (want spec, plan, or tasks)", s)
}

// Sections returns the ordered section set for the kind. The order is
// part of the engine digest (stamp.go): changing it changes every stamp.
func (k Kind) Sections() []SectionID {
	switch k {
	case KindPlan:
		return []SectionID{SectionIdentity, SectionDecisions, SectionConstraints, SectionPlan}
	case KindTasks:
		return []SectionID{SectionIdentity, SectionPlan, SectionEvidence, SectionReadiness}
	default:
		return []SectionID{SectionIdentity, SectionProblem, SectionOutcome, SectionDecisions, SectionConstraints, SectionCriteria, SectionQuestions, SectionPlan, SectionEvidence, SectionReadiness}
	}
}
