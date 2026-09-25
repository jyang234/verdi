package objsupersede

import (
	"fmt"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
)

// Outcome classifies one Result.
type Outcome string

const (
	// ResolvedNew: a new replacement whose records match (design §3); it
	// takes effect when its spec is accepted.
	ResolvedNew Outcome = "resolved-new"
	// ResolvedCarried: a revision carries a replacement an accepted
	// successor established (SI-265, SI-273).
	ResolvedCarried Outcome = "resolved-carried"
	// Unresolved: not established; Reason names the first failing
	// condition.
	Unresolved Outcome = "unresolved"
)

// Reason is the closed vocabulary of why a supersession is not established:
// design §5's ordered conditions, SI-271's pin, SI-274's additions, and
// §5's completeness finding. Every consumer renders it through Result.Text;
// an unknown value fails closed.
type Reason string

const (
	ReasonPinned                 Reason = "pinned"                           // SI-271, checked first
	ReasonRecordsUndecodable     Reason = "records-undecodable"              // SI-274(6)
	ReasonTargetMissing          Reason = "target-missing"                   // §5 condition 1
	ReasonTargetNotClosed        Reason = "target-not-closed"                // condition 2
	ReasonObjectNotDeclared      Reason = "object-not-declared"              // condition 3
	ReasonObjectNotTarget        Reason = "object-not-criterion-or-decision" // condition 4
	ReasonAlreadySuperseded      Reason = "already-superseded"               // condition 5, SI-272
	ReasonNoConflict             Reason = "no-conflict"                      // condition 6
	ReasonConflictNotSuperseded  Reason = "conflict-not-superseded"          // condition 7
	ReasonResolvedByOther        Reason = "resolved-by-other"                // condition 8
	ReasonMultipleConflicts      Reason = "multiple-conflicts"               // SI-274(1)
	ReasonConflictSpansSpecs     Reason = "conflict-spans-specs"             // SI-274(2)
	ReasonCarryMismatch          Reason = "carry-mismatch"                   // condition 9, SI-273
	ReasonEstablisherNotAccepted Reason = "establisher-not-accepted"         // SI-274(3)
	ReasonEstablisherNotInForce  Reason = "establisher-not-in-force"         // SI-274(3)
	ReasonAcceptanceUnproven     Reason = "acceptance-unproven"              // SI-274(3)
	ReasonUnmatchedChallenge     Reason = "unmatched-challenge"              // §5 completeness
)

// reasons is the closed vocabulary, in check order.
var reasons = []Reason{
	ReasonPinned, ReasonRecordsUndecodable, ReasonTargetMissing, ReasonTargetNotClosed,
	ReasonObjectNotDeclared, ReasonObjectNotTarget, ReasonAlreadySuperseded, ReasonNoConflict,
	ReasonConflictNotSuperseded, ReasonResolvedByOther, ReasonMultipleConflicts,
	ReasonConflictSpansSpecs, ReasonCarryMismatch, ReasonEstablisherNotAccepted,
	ReasonEstablisherNotInForce, ReasonAcceptanceUnproven, ReasonUnmatchedChallenge,
}

// Valid reports whether r is in the closed vocabulary.
func (r Reason) Valid() bool {
	for _, known := range reasons {
		if r == known {
			return true
		}
	}
	return false
}

// ParseReason returns s as a Reason, refusing any value outside the closed
// vocabulary.
func ParseReason(s string) (Reason, error) {
	if r := Reason(s); r.Valid() {
		return r, nil
	}
	return "", fmt.Errorf("objsupersede: unknown supersession reason %q", s)
}

// Text renders r in its one canonical English form: design §5's texts for
// the two resolved outcomes, or the reason for an unresolved one (align's
// finding text; the surfaces print it after "supersession not established:
// "). It is deterministic. An unknown outcome or reason, a resolved result
// carrying a reason, and a result missing a name, witness, or YYYY-MM-DD
// date its text needs are errors, never renderings.
func (r Result) Text() (string, error) {
	var text string
	var needs []string
	switch r.Outcome {
	case ResolvedNew, ResolvedCarried:
		if r.Reason != "" {
			return "", fmt.Errorf("objsupersede: a %s result carries reason %q", r.Outcome, r.Reason)
		}
		if r.Outcome == ResolvedNew {
			text, needs = fmt.Sprintf("records match; takes effect when spec/%s is accepted", r.Spec), []string{r.Spec}
			break
		}
		if !isDay(r.Since) {
			return "", fmt.Errorf("objsupersede: a carried result's date %q is not YYYY-MM-DD", r.Since)
		}
		text = fmt.Sprintf("carries the replacement established by spec/%s (conflict/%s, since %s)", r.Other, r.Conflict, r.Since)
		needs = []string{r.Other, r.Conflict}
	case Unresolved:
		var err error
		if text, needs, err = r.reasonText(); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("objsupersede: unknown supersession outcome %q", r.Outcome)
	}
	for _, v := range needs {
		if v == "" {
			return "", fmt.Errorf("objsupersede: a %s %s result lacks a name or witness its text needs", r.Outcome, r.Reason)
		}
	}
	return text, nil
}

// reasonText renders an unresolved result's reason, with the fields its
// text needs.
func (r Result) reasonText() (string, []string, error) {
	target, object := r.Edge, r.Edge
	if ref, err := artifact.ParseRef(r.Edge); err == nil {
		target = artifact.Ref{Kind: ref.Kind, Name: ref.Name}.String()
		object = artifact.Ref{Kind: ref.Kind, Name: ref.Name, Object: ref.Object}.String()
	}
	switch r.Reason {
	case ReasonPinned:
		// vocab:identity — design §5 / SI-272 / SI-274 reason wording: lifecycle and record-status ids, not display prose
		return fmt.Sprintf("the edge %s is pinned; a closed spec's object is superseded only by an unpinned ref", r.Edge), []string{r.Edge}, nil
	case ReasonRecordsUndecodable:
		return "records do not decode: " + r.Detail, []string{r.Detail}, nil
	case ReasonTargetMissing:
		return fmt.Sprintf("the target spec %s is missing", target), []string{r.Edge}, nil
	case ReasonTargetNotClosed:
		// vocab:identity — design §5 / SI-272 / SI-274 reason wording: lifecycle and record-status ids, not display prose
		return fmt.Sprintf("the target spec %s is not closed", target), []string{r.Edge}, nil
	case ReasonObjectNotDeclared:
		return fmt.Sprintf("the object %s is not declared", object), []string{r.Edge}, nil
	case ReasonObjectNotTarget:
		return fmt.Sprintf("the object %s is not an acceptance criterion or a decision", object), []string{r.Edge}, nil
	case ReasonAlreadySuperseded:
		// vocab:identity — design §5 / SI-272 / SI-274 reason wording: lifecycle and record-status ids, not display prose
		return fmt.Sprintf("the object %s is already superseded by spec/%s (conflict/%s)", object, r.Other, r.Conflict), []string{r.Edge, r.Other, r.Conflict}, nil
	case ReasonNoConflict:
		return fmt.Sprintf("no conflict challenges %s", object), []string{r.Edge}, nil
	case ReasonConflictNotSuperseded:
		// vocab:identity — design §5 / SI-272 / SI-274 reason wording: lifecycle and record-status ids, not display prose
		return fmt.Sprintf("the conflict conflict/%s is not superseded", r.Conflict), []string{r.Conflict}, nil
	case ReasonResolvedByOther:
		if r.Other == "" {
			return fmt.Sprintf("the conflict conflict/%s's resolved_by names no spec", r.Conflict), []string{r.Conflict}, nil
		}
		return fmt.Sprintf("the conflict conflict/%s's resolved_by names another spec, spec/%s", r.Conflict, r.Other), []string{r.Conflict}, nil
	case ReasonMultipleConflicts:
		// vocab:identity — design §5 / SI-272 / SI-274 reason wording: lifecycle and record-status ids, not display prose
		return fmt.Sprintf("more than one superseded conflict names spec/%s for %s", r.Other, target), []string{r.Other, r.Edge}, nil
	case ReasonConflictSpansSpecs:
		return "the conflict challenges objects of more than one spec", nil, nil
	case ReasonCarryMismatch:
		return fmt.Sprintf("a carried decision's classification or edge does not match its predecessor (spec/%s from spec/%s)", r.Other, r.Predecessor), []string{r.Other, r.Predecessor}, nil
	case ReasonEstablisherNotAccepted:
		return fmt.Sprintf("spec/%s is not accepted", r.Other), []string{r.Other}, nil
	case ReasonEstablisherNotInForce:
		return fmt.Sprintf("spec/%s's supersession was not in force at its acceptance: %s", r.Other, r.Detail), []string{r.Other, r.Detail}, nil
	case ReasonAcceptanceUnproven:
		return "acceptance unproven: " + r.Detail, []string{r.Detail}, nil
	case ReasonUnmatchedChallenge:
		return fmt.Sprintf("conflict/%s challenges %s, but spec/%s carries no matching edge", r.Conflict, r.Edge, r.Spec), []string{r.Conflict, r.Edge, r.Spec}, nil
	default:
		return "", nil, fmt.Errorf("objsupersede: unknown supersession reason %q", r.Reason)
	}
}

// isDay reports whether s is a YYYY-MM-DD date.
func isDay(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}
