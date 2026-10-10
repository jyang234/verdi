package readinesspilot

// guidance.go carries every unresolved concern's source-derived corrective
// guidance (SI-338 (1); Wave 6 §3.1: "source-derived corrective guidance
// for every violated or unproven row"). Guidance holds the sentences the
// wall shell and this derivation shared — moved here from the wall's own
// derivation (internal/workbench's deriveASDShell, since retired: the
// wall's Readiness tab renders this derivation's facts), so no sentence
// exists twice. guidanceFor attaches one sentence to every
// unresolved row of the closed vocabulary: the shared sentence where the
// wall had one, the journey blocker's own clearing condition for blocker
// rows, and one template per remaining family that names the corrective
// action and the row's destination (the wall, `verdi journey`, or the
// context-conflict verb). A proven row carries none.

import (
	"fmt"
	"strings"
)

// GuidanceFamily names a concern family whose corrective guidance sentence
// the wall shell and this derivation both render.
type GuidanceFamily int

const (
	// GuidanceProblem is shape/problem: no problem statement is declared.
	GuidanceProblem GuidanceFamily = iota + 1
	// GuidanceOutcome is shape/outcome: no outcome statement is declared.
	GuidanceOutcome
	// GuidanceQuestion is shape/question/<oq>: a declared open question is
	// unresolved, claimed by spike stubs or not.
	GuidanceQuestion
	// GuidanceScratch is an open scratch sticky on the wall: the wall's
	// shape/board count row and this derivation's per-item
	// shape/board/<kind>/<id> rows.
	GuidanceScratch
	// GuidanceCriteria is success/criteria: no acceptance criteria are
	// declared.
	GuidanceCriteria
	// GuidanceCoverage is success/coverage/<ac>: no stub covers a feature's
	// acceptance criterion.
	GuidanceCoverage
)

// GuidanceFacts carries the source facts a shared sentence names beyond
// its family.
type GuidanceFacts struct {
	// Object is the declared object the row names: the open question
	// (GuidanceQuestion) or the acceptance criterion (GuidanceCoverage).
	Object string
	// SpikeWord is the spike pseudo-class's resolved display word
	// (spec/vocabulary-surfaces), for a claimed GuidanceQuestion.
	SpikeWord string
	// ClaimingStubs counts the spike stubs whose resolves claims the open
	// question (GuidanceQuestion); zero means unclaimed.
	ClaimingStubs int
}

// Guidance returns family's corrective guidance sentence over facts, or ""
// for a family outside the shared set.
func Guidance(family GuidanceFamily, facts GuidanceFacts) string {
	switch family {
	case GuidanceProblem:
		return "State the problem (typed operation set-problem) — the case file opens with it."
	case GuidanceOutcome:
		return "State the intended outcome (typed operation set-outcome)."
	case GuidanceQuestion:
		if facts.ClaimingStubs > 0 {
			// A spike stub's `resolves` claims this question (PLAN.md §7
			// I-128 option (a); spec/uat-round-1 ac-10): acceptance does not
			// need a wall edit — the spike answers it after acceptance. Two
			// or more stubs may claim one question, so the head noun and its
			// verb agree with the count; the renameable class word stays the
			// attributive singular every sibling surface speaks.
			stubNoun, answerVerb := "stub", "answers"
			if facts.ClaimingStubs > 1 {
				stubNoun, answerVerb = "stubs", "answer"
			}
			return "No wall edit is required to accept: the claiming " + facts.SpikeWord + " " + stubNoun + " " + answerVerb + " it after acceptance."
		}
		return "Resolve it on the wall: edit or remove " + facts.Object + ", or graduate a decision that answers it."
	case GuidanceScratch:
		return "Graduate each sticky into the spec, or delete it — scratch never enters the record by itself."
	case GuidanceCriteria:
		return "Declare what must be true when this lands (typed operation add-ac)."
	case GuidanceCoverage:
		// vocab:identity — "story sticky" names the annotation TYPE id (02 §Record schemas' proto-sticky enum), not display class prose
		return "Plan the delivery: graduate a story sticky into a stub claiming " + facts.Object + "."
	default:
		return ""
	}
}

// guidanceFor returns c's corrective guidance within the derivation over
// in: "" when c is proven or its id is outside the closed vocabulary.
func guidanceFor(c Concern, in Input) string {
	if c.State == StateProven {
		return ""
	}
	parts := strings.Split(c.ID, "/")
	switch classifyConcern(c.ID) {
	case familyShapeProblem:
		return Guidance(GuidanceProblem, GuidanceFacts{})
	case familyShapeOutcome:
		return Guidance(GuidanceOutcome, GuidanceFacts{})
	case familyShapeQuestion:
		question := strings.Join(parts[2:], "/")
		return Guidance(GuidanceQuestion, GuidanceFacts{
			Object: question, SpikeWord: in.SpikeWord,
			ClaimingStubs: len(claimedQuestionSlugs(in.Shape.ClaimedQuestions)[question]),
		})
	case familyShapeBoardItem:
		return Guidance(GuidanceScratch, GuidanceFacts{})
	case familySuccessCriteria:
		return Guidance(GuidanceCriteria, GuidanceFacts{})
	case familySuccessCoverage:
		return Guidance(GuidanceCoverage, GuidanceFacts{Object: parts[2]})
	case familySuccessBlocker, familyReviewBlocker:
		// Journey blocker rows reuse their clearing condition (SI-338 (1)),
		// which blockerConcern already carries as the row's Summary.
		return c.Summary
	case familyShapeProvenance:
		return "Make further spec edits through the wall's typed operations, so the design-provenance chain records each one; a gap or an unavailable context it already records stays disclosed."
	case familyShapeMutation:
		return "Make the next typed edit on the wall: it first completes or rolls back the interrupted mutation that left this residue, or refuses and names why."
	case familyShapeBoard:
		return "Repair the scratch board's annotation records the witness names, so the wall can enumerate this spec's open stickies again."
	case familySuccessContributor:
		return fmt.Sprintf("Produce the %s evidence the acceptance criteria on the wall declare; this row stays unproven while no evidence source for it is wired to readiness.", strings.Join(parts[2:], "/"))
	case familyContextVerdict:
		return "Run the context-conflict verb with this spec's context request, then resolve what it reports."
	case familyContextMechanical:
		return "Resolve this mechanical policy-conflict row's reasons, then re-run the context-conflict verb with this spec's context request."
	case familyContextSemantic:
		return "Resolve this semantic policy-conflict row's reasons or record a disposition, then re-run the context-conflict verb with this spec's context request."
	case familyContextDisclosure:
		return "Address the condition this policy-conflict disclosure names, then re-run the context-conflict verb with this spec's context request."
	case familyReviewRole:
		return fmt.Sprintf("Before %s, a principal entitled to give %s must provide it; verdi journey shows the requirement.", parts[2], strings.Join(parts[3:], "/"))
	case familyReviewAction:
		return "Establish the facts the witnesses name, so verdi journey can offer a safe review action."
	case familyReviewEventualDerivation:
		return "Restore the sources the witnesses name, so verdi journey derives every eventual closure blocker."
	default:
		return ""
	}
}
