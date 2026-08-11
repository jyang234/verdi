package draftmutation

import (
	"fmt"
	"sort"
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
	initial, err := snapshot(current)
	if err != nil {
		return Applied{}, err
	}
	resultBytes := append([]byte(nil), current...)
	firstTouches := make(map[string]int)
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
		if primaryTarget == "" {
			return Applied{}, fmt.Errorf("draftmutation: operation[%d] has no semantic target", index)
		}
		if _, seen := firstTouches[primaryTarget]; !seen {
			firstTouches[primaryTarget] = index
		}
		for _, target := range changedSnapshotTargets(before, after) {
			if _, seen := firstTouches[target]; !seen {
				firstTouches[target] = index
			}
		}
		resultBytes = next
	}
	final, err := snapshot(resultBytes)
	if err != nil {
		return Applied{}, err
	}
	targets := changedSnapshotTargets(initial, final)
	sort.Slice(targets, func(i, j int) bool {
		left, leftOK := firstTouches[targets[i]]
		right, rightOK := firstTouches[targets[j]]
		if leftOK != rightOK {
			return leftOK
		}
		if left != right {
			return left < right
		}
		return targets[i] < targets[j]
	})
	changes := make([]Change, 0, len(targets))
	warnings := make([]Warning, 0, len(targets))
	for _, target := range targets {
		if _, ok := firstTouches[target]; !ok {
			return Applied{}, fmt.Errorf("draftmutation: final target %q has no originating operation", target)
		}
		change, err := changeForFinalState(target, initial[target], final[target])
		if err != nil {
			return Applied{}, err
		}
		changes = append(changes, change)
		warnings = append(warnings, warningsForFinalChange(change, final[target])...)
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

func changeForFinalState(target string, before, after semanticValue) (Change, error) {
	change := Change{Target: target}
	beforePresent := before.ObjectDigest != ""
	afterPresent := after.ObjectDigest != ""
	relationship := strings.HasPrefix(target, "link/") || strings.HasPrefix(target, "context/")
	switch {
	case relationship && !beforePresent:
		change.Change, change.AfterDigest = ChangeRelationshipAdded, after.ObjectDigest
	case relationship && !afterPresent:
		change.Change, change.BeforeDigest = ChangeRelationshipRemoved, before.ObjectDigest
	case !beforePresent:
		change.Change, change.AfterDigest = ChangeAdded, after.ObjectDigest
	case !afterPresent:
		change.Change, change.BeforeDigest = ChangeRemoved, before.ObjectDigest
	case before.ObjectDigest == after.ObjectDigest && before.Ordered && after.Ordered && before.Position != after.Position:
		change.Change, change.BeforeDigest, change.AfterDigest = ChangeReordered, before.ObjectDigest, after.ObjectDigest
	default:
		change.Change, change.BeforeDigest, change.AfterDigest = ChangeReplaced, before.ObjectDigest, after.ObjectDigest
	}
	if err := change.Validate(); err != nil {
		return Change{}, err
	}
	return change, nil
}

func warningsForFinalChange(change Change, after semanticValue) []Warning {
	warnings := []Warning{}
	switch change.Change {
	case ChangeRemoved:
		warnings = append(warnings, Warning{Code: WarningDestructiveRemoval, Target: change.Target})
	case ChangeReordered:
		warnings = append(warnings, Warning{Code: WarningSemanticReorder, Target: change.Target})
	}
	if strings.HasPrefix(change.Target, "link/") || strings.HasPrefix(change.Target, "context/") {
		warnings = append(warnings, Warning{Code: WarningRelationshipChange, Target: change.Target})
	}
	if change.Change == ChangeReplaced && utf8.RuneCountInString(after.Text) > 1000 {
		warnings = append(warnings, Warning{Code: WarningLargeReplacement, Target: change.Target})
	}
	return warnings
}
