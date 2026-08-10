package draftmutation

import (
	"fmt"
	"strings"
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
		primaryTarget := operationTarget(operation)
		for _, target := range changedSnapshotTargets(before, after) {
			var change Change
			if target == primaryTarget {
				change, err = changeForOperation(operation, target, before[target], after[target])
			} else {
				change, err = changeForSecondaryTarget(target, before[target], after[target])
			}
			if err != nil {
				return Applied{}, fmt.Errorf("draftmutation: operation[%d]: %w", index, err)
			}
			changes = append(changes, change)
			warnings = append(warnings, warningsForChange(operation, primaryTarget, change)...)
		}
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

func changeForSecondaryTarget(target string, before, after semanticValue) (Change, error) {
	change := Change{Target: target}
	switch {
	case strings.HasPrefix(target, "link/") || strings.HasPrefix(target, "context/"):
		if before.Digest == "" {
			change.Change, change.AfterDigest = ChangeRelationshipAdded, after.Digest
		} else if after.Digest == "" {
			change.Change, change.BeforeDigest = ChangeRelationshipRemoved, before.Digest
		} else {
			return Change{}, fmt.Errorf("relationship target %q changed without being the operation target", target)
		}
	case before.Digest != "" && after.Digest != "" && before.ObjectDigest == after.ObjectDigest:
		change.Change, change.BeforeDigest, change.AfterDigest = ChangeReordered, before.Digest, after.Digest
	case before.Digest == "":
		change.Change, change.AfterDigest = ChangeAdded, after.Digest
	case after.Digest == "":
		change.Change, change.BeforeDigest = ChangeRemoved, before.Digest
	default:
		change.Change, change.BeforeDigest, change.AfterDigest = ChangeReplaced, before.Digest, after.Digest
	}
	if err := change.Validate(); err != nil {
		return Change{}, err
	}
	return change, nil
}

func warningsForChange(operation Operation, primaryTarget string, change Change) []Warning {
	warnings := []Warning{}
	switch change.Change {
	case ChangeRemoved:
		warnings = append(warnings, Warning{Code: WarningDestructiveRemoval, Target: change.Target})
	case ChangeReordered:
		warnings = append(warnings, Warning{Code: WarningSemanticReorder, Target: change.Target})
	case ChangeRelationshipAdded, ChangeRelationshipRemoved:
		warnings = append(warnings, Warning{Code: WarningRelationshipChange, Target: change.Target})
	}
	if change.Target == primaryTarget && (operation.Op == OpSetProblem || operation.Op == OpSetOutcome || operation.Op == OpEditAC || operation.Op == OpEditConstraint || operation.Op == OpEditDecision || operation.Op == OpEditQuestion) && utf8.RuneCountInString(operation.Text) > 1000 {
		warnings = append(warnings, Warning{Code: WarningLargeReplacement, Target: change.Target})
	}
	return warnings
}
