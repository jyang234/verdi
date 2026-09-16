package specimport

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/lint"
	"github.com/jyang234/verdi/internal/store"
)

// composeNative handles native format: plan.Native, produced by
// Normalize's own normalizeNative, is the candidate (spec-import-
// contract.md, "Candidate and validation": "plan.Native is the candidate.
// Valid native content and stable IDs remain byte-identical"). Re-running
// normalizeNative here — the identical, already-tested Task 1 structural
// gate, never a second copy of its checks — is Compose's own defense
// against a caller passing a Plan that does not actually correspond to
// request (Normalize's own "no service trusts prior decoding" posture,
// applied one layer up); the shared lint.CheckCandidate seam then applies
// every corpus-aware check (strict decode is already proven by
// normalizeNative; anchors, requiredness, ref resolution, duplicate
// identity, tracker-scheme configuredness) identically to the external
// path — "no grandfathering" for an old native input.
func composeNative(ctx context.Context, root string, request Request, plan Plan) ([]byte, []Finding, error) {
	if len(plan.Native) == 0 {
		return nil, nil, fmt.Errorf("specimport: compose: native format requires a non-empty plan.Native (request/plan mismatch)")
	}
	if _, err := normalizeNative(request, plan.Native); err != nil {
		return nil, []Finding{{Code: FindingInvalidCandidate, Target: "native", Message: err.Error(), Blocking: true}}, nil
	}

	relPath := store.ActiveSpecRelPath(request.Target.Slug)
	lintFindings, err := lint.CheckCandidate(ctx, root, relPath, plan.Native)
	if err != nil {
		return nil, nil, fmt.Errorf("specimport: compose: checking native candidate: %w", err)
	}
	findings := translateLintFindings(lintFindings, relPath)
	if hasBlocking(findings) {
		return nil, findings, nil
	}
	return plan.Native, findings, nil
}
