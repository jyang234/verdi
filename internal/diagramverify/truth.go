package diagramverify

import (
	"context"

	"github.com/jyang234/verdi/internal/upstream"
)

// RegenerateTruth regenerates truth for a proposal by execing the pinned
// flowmap CLI's `graph` subcommand at scope (flowmap's `-entry` selector,
// or "" for unscoped) through the existing internal/upstream seam's
// RunGraph/DecodeGraph strict-JSON path (spec/verification-extractor
// ac-2/dc-3) — no second, parallel exec path. stamp is the identity stamp
// (typically the commit SHA under verification) flowmap records into the
// graph.
func RegenerateTruth(ctx context.Context, runner upstream.Runner, dir, stamp, scope string) (*upstream.Graph, error) {
	return upstream.RunGraph(ctx, runner, dir, stamp, scope)
}

// TruthFQNs returns every first-party node FQN in g, in g's own order —
// the identity-normalization input Parse (grammar.go) needs.
func TruthFQNs(g *upstream.Graph) []string {
	fqns := make([]string, len(g.Nodes))
	for i, n := range g.Nodes {
		fqns[i] = n.FQN
	}
	return fqns
}

// TruthShortNames returns the set of UNAMBIGUOUS ShortNames in g — the
// identity space Compare (compare.go) checks proposal/base node elements
// against. A ShortName colliding across more than one FQN (dc-2) is
// deliberately EXCLUDED: Parse already downgrades any proposal node using
// that name to CoveragePartial and marks it Ambiguous, so Compare must
// never silently treat an ambiguous name as resolved to one specific FQN
// it cannot actually distinguish.
func TruthShortNames(g *upstream.Graph) map[string]bool {
	idx := shortNameIndex(TruthFQNs(g))
	out := make(map[string]bool, len(idx))
	for name, fqns := range idx {
		if len(fqns) == 1 {
			out[name] = true
		}
	}
	return out
}

// EdgeIdentity renders an edge's ordered (from, to) pair as the single
// string identity dc-1 defines edges by — shared by truth-side and
// proposal-side edge identities so both sides of Compare speak the same
// string space.
func EdgeIdentity(from, to string) string {
	return from + "->" + to
}

// TruthEdgeIdentities returns g's edges' identities (EdgeIdentity over
// each endpoint's ShortName) as the comparison set Compare checks proposal
// edges against. A boundary-effect target (flowmap's "boundary:..."
// pseudo-node) never collides with a real mermaid node id (dc-1's
// nodeIDPattern excludes ':'/space), so it is included unfiltered here
// without risk of a false match.
func TruthEdgeIdentities(g *upstream.Graph) map[string]bool {
	out := make(map[string]bool, len(g.Edges))
	for _, e := range g.Edges {
		out[EdgeIdentity(ShortName(e.From), ShortName(e.To))] = true
	}
	return out
}
