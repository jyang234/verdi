package diagramverify

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/canonjson"
	"github.com/jyang234/verdi/internal/upstream"
)

// StaleBase recomputes a derived proposal's base digest at current HEAD —
// the SAME flowmap invocation ac-2's RegenerateTruth already performs, at
// the proposal's declared scope (spec/verification-extractor ac-4/dc-5) —
// canonicalizing it through the shared internal/canonjson.Digest formula
// this codebase already standardizes on for every other computed digest,
// and compares the result against baseDigest (derived_from.digest) by
// plain string equality. It runs independently of Compare's own
// three-way result: a proposal can be stale-base and still have every
// element Exists, or vice versa — callers run both checks, never
// conflating one into the other.
func StaleBase(ctx context.Context, runner upstream.Runner, dir, stamp, scope, baseDigest string) (stale bool, currentDigest string, err error) {
	g, err := RegenerateTruth(ctx, runner, dir, stamp, scope)
	if err != nil {
		return false, "", fmt.Errorf("diagramverify: recomputing base digest: %w", err)
	}
	currentDigest, err = canonjson.Digest(g)
	if err != nil {
		return false, "", fmt.Errorf("diagramverify: digesting regenerated graph: %w", err)
	}
	return currentDigest != baseDigest, currentDigest, nil
}
