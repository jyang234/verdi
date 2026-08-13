// Sealed conflict-operand acquisition (docs/superpowers/specs/2026-08-12-
// policy-conflict-gate-authority-design.md §§2-3; ledger SI-93). This file
// owns the one dispatch seam between a validated Request's exact target
// union arm and internal/contextcompile's sealed ConflictOperands: it
// never reloads policy itself and never reimplements accepted- or
// candidate-target resolution.
package policyconflict

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/contextcompile"
)

// ResolveOperands validates req and resolves its exact target arm into
// sealed contextcompile.ConflictOperands over ONE authority resolution
// (authority design §§2-3): the accepted-context arm dispatches to the
// injected compiler's own CompileConflict (reusing the identical single
// compile/policy-resolution pass a hermetic caller controls), while the
// acceptance-candidate arm maps AcceptanceCandidate's fields exactly onto
// contextcompile.CandidateRequest and dispatches to the package-level
// contextcompile.ResolveConflictCandidate, which constructs its own
// production compiler internally. compiler is therefore consumed only by
// the accepted-context arm; the acceptance-candidate arm never reloads
// policy through it.
func ResolveOperands(ctx context.Context, compiler contextcompile.Compiler, root string, req Request, facts contextcompile.ConflictFacts) (*contextcompile.ConflictOperands, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("policyconflict: resolve operands: %w", err)
	}

	switch req.Target.Kind {
	case TargetAcceptedContext:
		operands, err := compiler.CompileConflict(ctx, root, *req.Target.AcceptedContext, facts)
		if err != nil {
			return nil, fmt.Errorf("policyconflict: resolve accepted-context operands: %w", err)
		}
		return operands, nil

	case TargetAcceptanceCandidate:
		candidate := req.Target.AcceptanceCandidate
		candidateRequest := contextcompile.CandidateRequest{
			Adapter:  candidate.Adapter,
			Expected: candidate.Expected,
			Grants:   candidate.Grants,
			Scope:    candidate.Scope,
			Spec:     candidate.Spec,
		}
		operands, err := contextcompile.ResolveConflictCandidate(ctx, root, candidateRequest, facts)
		if err != nil {
			return nil, fmt.Errorf("policyconflict: resolve acceptance-candidate operands: %w", err)
		}
		return operands, nil

	default:
		return nil, fmt.Errorf("policyconflict: resolve operands: unknown target kind %q", req.Target.Kind)
	}
}
