package draftmutation

import (
	"fmt"
	"unicode/utf8"

	"github.com/jyang234/verdi/internal/artifact/splice"
	"github.com/jyang234/verdi/internal/designprovenance"
)

// Applied is the pure mutation output: exact resulting bytes, the public
// result, and excerpts ready for the provenance entry.
type Applied struct {
	Spec               []byte
	Result             Result
	ProvenanceExcerpts []designprovenance.Excerpt
}

// Apply executes an ordered request without filesystem access.
func Apply(current []byte, request Request, identity Identity) (Applied, error) {
	if err := identity.Validate(); err != nil {
		return Applied{}, err
	}
	resultBytes := append([]byte(nil), current...)
	changes := make([]Change, 0, len(request.Operations))
	warnings := make([]Warning, 0, len(request.Operations))
	for index, operation := range request.Operations {
		before, err := snapshot(resultBytes)
		if err != nil {
			return Applied{}, err
		}
		next, err := splice.ApplyDraftMutations(resultBytes, []Operation{operation})
		if err != nil {
			return Applied{}, fmt.Errorf("draftmutation: operation[%d]: %w", index, err)
		}
		after, err := snapshot(next)
		if err != nil {
			return Applied{}, err
		}
		target := operationTarget(operation)
		change, err := changeForOperation(operation, target, before[target], after[target])
		if err != nil {
			return Applied{}, fmt.Errorf("draftmutation: operation[%d]: %w", index, err)
		}
		changes = append(changes, change)
		warnings = append(warnings, warningsForOperation(operation, target)...)
		resultBytes = next
	}
	final, err := snapshot(resultBytes)
	if err != nil {
		return Applied{}, err
	}
	excerpts := make([]designprovenance.Excerpt, len(request.Excerpts))
	for i, excerpt := range request.Excerpts {
		value, ok := final[excerpt.Target]
		if !ok || excerpt.Target == "" || len(excerpt.Target) >= len("stub/") && excerpt.Target[:len("stub/")] == "stub/" || len(excerpt.Target) >= len("link/") && excerpt.Target[:len("link/")] == "link/" || len(excerpt.Target) >= len("context/") && excerpt.Target[:len("context/")] == "context/" {
			return Applied{}, fmt.Errorf("draftmutation: excerpt target %q does not name a resulting problem, outcome, or object", excerpt.Target)
		}
		excerpts[i] = designprovenance.Excerpt{
			Target: excerpt.Target, TargetDigest: value.ObjectDigest,
			Classification: excerpt.Classification, Representation: excerpt.Representation, Text: excerpt.Text,
		}
	}
	result := Result{
		Schema: ResultSchema, Identity: identity,
		PreviousDigest: DigestBytes(current), ResultDigest: DigestBytes(resultBytes),
		Changes: changes, Warnings: warnings,
		Disclosures: []Disclosure{{Code: DisclosureContextUnavailable, Reason: designprovenance.ContextUnavailableReason}},
	}
	if err := result.Validate(); err != nil {
		return Applied{}, err
	}
	return Applied{Spec: resultBytes, Result: result, ProvenanceExcerpts: excerpts}, nil
}

func changeForOperation(operation Operation, target string, before, after semanticValue) (Change, error) {
	change := Change{Target: target}
	switch operation.Op {
	case OpAddLink, OpAddContextRef:
		change.Change, change.AfterDigest = ChangeRelationshipAdded, after.Digest
	case OpRemoveLink, OpRemoveContextRef:
		change.Change, change.BeforeDigest = ChangeRelationshipRemoved, before.Digest
	case OpRemoveAC, OpRemoveConstraint, OpRemoveDecision, OpRemoveQuestion, OpRemoveStub:
		change.Change, change.BeforeDigest = ChangeRemoved, before.Digest
	case OpReorderAC, OpReorderStub:
		change.Change, change.BeforeDigest, change.AfterDigest = ChangeReordered, before.Digest, after.Digest
	default:
		if before.Digest == "" {
			change.Change, change.AfterDigest = ChangeAdded, after.Digest
		} else {
			change.Change, change.BeforeDigest, change.AfterDigest = ChangeReplaced, before.Digest, after.Digest
		}
	}
	if err := change.Validate(); err != nil {
		return Change{}, err
	}
	return change, nil
}

func warningsForOperation(operation Operation, target string) []Warning {
	warnings := []Warning{}
	switch operation.Op {
	case OpRemoveAC, OpRemoveConstraint, OpRemoveDecision, OpRemoveQuestion, OpRemoveStub:
		warnings = append(warnings, Warning{Code: WarningDestructiveRemoval, Target: target})
	case OpReorderAC, OpReorderStub:
		warnings = append(warnings, Warning{Code: WarningSemanticReorder, Target: target})
	case OpAddLink, OpRemoveLink, OpAddContextRef, OpRemoveContextRef:
		warnings = append(warnings, Warning{Code: WarningRelationshipChange, Target: target})
	}
	if (operation.Op == OpSetProblem || operation.Op == OpSetOutcome || operation.Op == OpEditAC || operation.Op == OpEditConstraint || operation.Op == OpEditDecision || operation.Op == OpEditQuestion) && utf8.RuneCountInString(operation.Text) > 1000 {
		warnings = append(warnings, Warning{Code: WarningLargeReplacement, Target: target})
	}
	return warnings
}
