// scope.go proves the four-dimensional scope conjunction authority design
// §4 fixes: phase, environment, path, and ref, in that fixed order. Both the
// pairwise comparison (CompareScopes) and the N-way group comparison
// (IntersectScopes) share one engine: each dimension computes ONE total
// intersection across every participating scope's dimension set, and the
// same product rule (any proven-disjoint dimension makes the witness
// disjoint; every proven-overlap dimension makes it overlap; otherwise
// unknown) applies to both arities (§4.4; ledger SI-94). CompareScopes is
// implemented as IntersectScopes over the exact two-element slice, so the
// two functions can never observably disagree on the same pair.
//
// Degenerate IntersectScopes inputs (documented per the Task 5 brief, not
// separately named by §4.4):
//   - Zero scopes: every dimension has zero participants, which this
//     package treats identically to "every participant is universal" — the
//     standard vacuous-intersection convention (intersecting over an empty
//     family is the whole universe, since "for every participant, x is a
//     member" is vacuously true for any x). The result is the universal
//     scope: State overlap, every dimension overlap with empty witnesses.
//     This is the smallest reversible choice: it never invents a disjoint
//     or unknown outcome from the absence of any operand.
//   - One scope: intersecting a single set with only itself is always that
//     set (there is no second participant to conflict with), so every
//     dimension is proven overlap and the result State is always overlap.
//     This falls out of the same engine with no special-casing (see the
//     "exactly one non-universal participant" branch below).
package policyconflict

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// RefRelationResolver is scope comparison's one seam onto the accepted/
// candidate artifact graph (authority design §4.3): for two DIFFERENT ref
// strings it proves overlap, disjoint, or unknown, and returns the exact
// graph-edge witnesses supporting that state. Scope comparison never parses
// ref structure or reasons about the graph itself — every non-identical ref
// pair is resolved exclusively through this port.
type RefRelationResolver interface {
	Relate(ctx context.Context, a, b string) (ScopeState, []string, error)
}

// CompareScopes proves the pairwise four-dimensional scope conjunction
// between a and b (authority design §4). It is exactly IntersectScopes over
// the two-element slice {a, b}.
func CompareScopes(ctx context.Context, a, b policyartifact.Scope, resolver RefRelationResolver) (ScopeProof, error) {
	return IntersectScopes(ctx, []policyartifact.Scope{a, b}, resolver)
}

// IntersectScopes proves the N-way four-dimensional scope conjunction
// across scopes (authority design §4.4): each dimension intersects every
// participating scope's set ONCE — never by chaining pairwise proofs — and
// the overall state follows the same product rule CompareScopes uses.
func IntersectScopes(ctx context.Context, scopes []policyartifact.Scope, resolver RefRelationResolver) (ScopeProof, error) {
	phaseSets := make([][]string, len(scopes))
	envSets := make([][]string, len(scopes))
	pathSets := make([][]string, len(scopes))
	refSets := make([][]string, len(scopes))
	for i, s := range scopes {
		phaseSets[i] = sortedUniqueCopy(s.Phases)
		envSets[i] = sortedUniqueCopy(s.Environments)
		pathSets[i] = sortedUniqueCopy(s.Paths)
		refSets[i] = sortedUniqueCopy(s.Refs)
	}

	phaseDim := intersectValueDimension("phase", phaseSets)
	envDim := intersectValueDimension("environment", envSets)
	pathDim := intersectPathDimension(pathSets)
	refDim, err := intersectRefDimension(ctx, refSets, resolver)
	if err != nil {
		return ScopeProof{}, fmt.Errorf("policyconflict: intersect ref dimension: %w", err)
	}

	dims := []DimensionProof{phaseDim, envDim, pathDim, refDim}
	return ScopeProof{State: productState(dims), Dimensions: dims}, nil
}

// productState applies authority design §4.4's product rule to dims, which
// must already be in fixed phase/environment/path/ref order: any proven-
// disjoint dimension makes the whole proof disjoint; if every dimension is
// proven overlap, the whole proof is overlap; otherwise it is unknown.
func productState(dims []DimensionProof) ScopeState {
	anyUnknown := false
	for _, d := range dims {
		switch d.State {
		case ScopeDisjoint:
			return ScopeDisjoint
		case ScopeUnknown:
			anyUnknown = true
		}
	}
	if anyUnknown {
		return ScopeUnknown
	}
	return ScopeOverlap
}

// --- shared set helpers ------------------------------------------------

// sortedUniqueCopy returns a fresh canonical-lexical-order, duplicate-free,
// non-nil copy of s. Every DimensionProof set this package emits goes
// through this helper (or a helper built on it) so the package's own
// validate.go ordering rules are met by construction.
func sortedUniqueCopy(s []string) []string {
	cp := append([]string(nil), s...)
	sort.Strings(cp)
	out := make([]string, 0, len(cp))
	for i, v := range cp {
		if i == 0 || v != cp[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// nonUniversalSets returns only sets' non-empty (non-universal) members,
// preserving their original relative order — the fixed, deterministic
// iteration order every dimension function below relies on.
func nonUniversalSets(sets [][]string) [][]string {
	out := make([][]string, 0, len(sets))
	for _, s := range sets {
		if len(s) > 0 {
			out = append(out, s)
		}
	}
	return out
}

// unionSortedUnique returns the canonical-lexical-order, duplicate-free
// union of every set in sets.
func unionSortedUnique(sets [][]string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0)
	for _, s := range sets {
		for _, v := range s {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	sort.Strings(out)
	return out
}

// intersectSortedStrings returns the sorted intersection of a and b, which
// must already be canonical-lexical-order and duplicate-free.
func intersectSortedStrings(a, b []string) []string {
	out := make([]string, 0)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			out = append(out, a[i])
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return out
}

// dimensionSides returns a DimensionProof's Left/Right pair for sets. The
// wire schema (schema.go's DimensionProof) has exactly two named sides,
// which is the natural, information-preserving shape for a pairwise
// comparison (len(sets)==2): Left and Right are then literally the two
// operands' own dimension values. For any other arity — the N-way case, and
// the two degenerate single/zero-scope cases — there is no natural two-
// sided split, so both sides carry the identical sorted-unique union of
// every participating set's values: the full input surface remains visible
// on the wire, and the dimension's own Intersection/Witnesses fields carry
// the actual computed result.
func dimensionSides(sets [][]string) (left, right []string) {
	if len(sets) == 2 {
		return sets[0], sets[1]
	}
	return unionSortedUnique(sets), unionSortedUnique(sets)
}

// --- phase / environment: exact-value-set dimensions --------------------

// intersectValueDimension proves dim (phase or environment) via literal
// set-fold intersection across every participating set (authority design
// §4.1: exact set intersection, complete because the vocabularies are
// closed). Universal (empty) sets never narrow the fold (§4.1: "A universal
// side overlaps the other side").
func intersectValueDimension(dim string, sets [][]string) DimensionProof {
	left, right := dimensionSides(sets)
	nonUniversal := nonUniversalSets(sets)
	switch len(nonUniversal) {
	case 0:
		return DimensionProof{Dimension: dim, State: ScopeOverlap, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}
	case 1:
		vals := sortedUniqueCopy(nonUniversal[0])
		return DimensionProof{Dimension: dim, State: ScopeOverlap, Left: left, Right: right, Intersection: vals, Witnesses: vals}
	default:
		inter := nonUniversal[0]
		for _, s := range nonUniversal[1:] {
			inter = intersectSortedStrings(inter, s)
			if len(inter) == 0 {
				break
			}
		}
		if len(inter) == 0 {
			return DimensionProof{Dimension: dim, State: ScopeDisjoint, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}
		}
		return DimensionProof{Dimension: dim, State: ScopeOverlap, Left: left, Right: right, Intersection: inter, Witnesses: inter}
	}
}

// --- path: segment-aware hierarchical dimension --------------------------

// pathIsSubtree reports whether p names a directory subtree selector
// (authority design §4.2: a trailing slash is a directory subtree; no
// trailing slash is one exact entry).
func pathIsSubtree(p string) bool { return strings.HasSuffix(p, "/") }

// pathsOverlap decides one path VALUE pair per authority design §4.2:
// exact-equal, an exact entry inside a directory subtree, or one subtree
// containing another, all segment-aware since every subtree selector
// already carries its own trailing slash (so a plain string-prefix test
// cannot cross a segment boundary: "cmd/" is not a prefix of "cmdline/x").
func pathsOverlap(a, b string) bool {
	if a == b {
		return true
	}
	aDir, bDir := pathIsSubtree(a), pathIsSubtree(b)
	switch {
	case aDir && bDir:
		return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
	case aDir && !bDir:
		return strings.HasPrefix(b, a)
	case !aDir && bDir:
		return strings.HasPrefix(a, b)
	default:
		return false
	}
}

// intersectPathDimension proves the path dimension's total N-way
// intersection (authority design §4.2/§4.4). A candidate path value c —
// drawn only from the union of every participating set's own values, since
// the deepest/narrowest selector any group of nested or sibling subtree
// selectors can agree on is always one of those given values — is a proven
// total-intersection witness when it overlaps (pathsOverlap) at least one
// value in EVERY non-universal participating set. This is a single direct
// per-candidate computation, never a chain through a further unrelated
// candidate, so it cannot manufacture agreement between two sets that do
// not themselves share a region.
func intersectPathDimension(sets [][]string) DimensionProof {
	left, right := dimensionSides(sets)
	nonUniversal := nonUniversalSets(sets)
	switch len(nonUniversal) {
	case 0:
		return DimensionProof{Dimension: "path", State: ScopeOverlap, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}
	case 1:
		vals := sortedUniqueCopy(nonUniversal[0])
		return DimensionProof{Dimension: "path", State: ScopeOverlap, Left: left, Right: right, Intersection: vals, Witnesses: vals}
	default:
		candidates := unionSortedUnique(nonUniversal)
		covered := make([]string, 0)
		for _, c := range candidates {
			coveredByAll := true
			for _, s := range nonUniversal {
				coveredByS := false
				for _, t := range s {
					if pathsOverlap(c, t) {
						coveredByS = true
						break
					}
				}
				if !coveredByS {
					coveredByAll = false
					break
				}
			}
			if coveredByAll {
				covered = append(covered, c)
			}
		}
		witnesses := minimalPathWitnesses(covered)
		if len(witnesses) == 0 {
			return DimensionProof{Dimension: "path", State: ScopeDisjoint, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}
		}
		return DimensionProof{Dimension: "path", State: ScopeOverlap, Left: left, Right: right, Intersection: witnesses, Witnesses: witnesses}
	}
}

// minimalPathWitnesses drops every covered candidate that is itself a
// directory subtree properly containing another covered candidate: the
// broader selector is real evidence that coverage held, but the true
// overlap REGION is the narrower one it contains (authority design §4.2's
// containment semantics), so reporting both would misrepresent a single
// exact-entry overlap as if it also meant "the whole containing subtree is
// shared." Candidates that do not contain one another (separate covered
// regions) are all kept. covered must already be canonical-lexical-order;
// the result preserves that order.
func minimalPathWitnesses(covered []string) []string {
	out := make([]string, 0, len(covered))
	for _, c := range covered {
		tooBroad := false
		if pathIsSubtree(c) {
			for _, d := range covered {
				if d != c && strings.HasPrefix(d, c) {
					tooBroad = true
					break
				}
			}
		}
		if !tooBroad {
			out = append(out, c)
		}
	}
	return out
}

// --- ref: resolver-mediated dimension ------------------------------------

// refPairKey canonicalizes an unordered ref pair so the resolver is always
// queried in one fixed lexical order and, within one CompareScopes/
// IntersectScopes call, at most once per distinct pair.
type refPairKey struct{ lo, hi string }

func canonicalRefPair(a, b string) refPairKey {
	if a <= b {
		return refPairKey{a, b}
	}
	return refPairKey{b, a}
}

// refPairResolution is one memoized resolver outcome.
type refPairResolution struct {
	state ScopeState
	wit   []string
}

// resolveRefPair returns a's relation to b: exact-equal is overlap with no
// resolver call (authority design §4.3: "Exact-equal refs overlap"); every
// other pair goes exclusively through resolver, canonicalized and cached so
// repeated candidate/witness combinations invoke it at most once and in a
// deterministic order. A resolver error is operational and aborts the
// whole dimension computation (never guessed at); an out-of-vocabulary
// ScopeState is treated the same way, since an unknown enum value fails
// closed here just as it does at every decode boundary.
func resolveRefPair(ctx context.Context, cache map[refPairKey]refPairResolution, resolver RefRelationResolver, a, b string) (ScopeState, []string, error) {
	if a == b {
		return ScopeOverlap, nil, nil
	}
	key := canonicalRefPair(a, b)
	if r, ok := cache[key]; ok {
		return r.state, r.wit, nil
	}
	state, wit, err := resolver.Relate(ctx, key.lo, key.hi)
	if err != nil {
		return "", nil, fmt.Errorf("policyconflict: ref relation resolver: %q vs %q: %w", key.lo, key.hi, err)
	}
	if verr := state.Validate(); verr != nil {
		return "", nil, fmt.Errorf("policyconflict: ref relation resolver returned an invalid scope state for %q vs %q: %w", key.lo, key.hi, verr)
	}
	cache[key] = refPairResolution{state: state, wit: wit}
	return state, wit, nil
}

// intersectRefDimension proves the ref dimension's total N-way
// intersection (authority design §4.3/§4.4). It mirrors
// intersectPathDimension's candidate/coverage shape, but "overlap" between
// a candidate and a set member is proven exclusively by exact-equality or
// resolveRefPair — never inferred from any other pair. A candidate that is
// directly proven to overlap at least one member of EVERY non-universal
// participating set is a proven total-intersection witness; this can never
// manufacture agreement between two sets from an unrelated pair's overlap,
// because every check is between the SAME fixed candidate and a member of
// the set actually being tested — it never substitutes a third ref's
// relation for the two sets' own relation to each other.
func intersectRefDimension(ctx context.Context, sets [][]string, resolver RefRelationResolver) (DimensionProof, error) {
	left, right := dimensionSides(sets)
	nonUniversal := nonUniversalSets(sets)
	switch len(nonUniversal) {
	case 0:
		return DimensionProof{Dimension: "ref", State: ScopeOverlap, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}, nil
	case 1:
		vals := sortedUniqueCopy(nonUniversal[0])
		return DimensionProof{Dimension: "ref", State: ScopeOverlap, Left: left, Right: right, Intersection: vals, Witnesses: vals}, nil
	}

	cache := make(map[refPairKey]refPairResolution)
	candidates := unionSortedUnique(nonUniversal)

	proven := make([]string, 0)
	ambiguous := make([]string, 0)
	overlapEvidence := make(map[string]bool)
	unknownEvidence := make(map[string]bool)

	for _, c := range candidates {
		excluded := false
		candidateAmbiguous := false
		var candidateEvidence []string
		for _, s := range nonUniversal {
			setOverlap := false
			setAmbiguous := false
			for _, t := range s {
				state, wit, err := resolveRefPair(ctx, cache, resolver, c, t)
				if err != nil {
					return DimensionProof{}, err
				}
				switch state {
				case ScopeOverlap:
					setOverlap = true
					candidateEvidence = append(candidateEvidence, wit...)
				case ScopeUnknown:
					setAmbiguous = true
					candidateEvidence = append(candidateEvidence, wit...)
				}
			}
			if setOverlap {
				continue
			}
			if setAmbiguous {
				candidateAmbiguous = true
				continue
			}
			excluded = true
			break
		}
		if excluded {
			continue
		}
		if candidateAmbiguous {
			ambiguous = append(ambiguous, c)
			for _, w := range candidateEvidence {
				unknownEvidence[w] = true
			}
			continue
		}
		proven = append(proven, c)
		for _, w := range candidateEvidence {
			overlapEvidence[w] = true
		}
	}

	if len(proven) > 0 {
		return DimensionProof{
			Dimension:    "ref",
			State:        ScopeOverlap,
			Left:         left,
			Right:        right,
			Intersection: sortedUniqueCopy(proven),
			Witnesses:    sortedUniqueSetKeys(overlapEvidence),
		}, nil
	}
	if len(ambiguous) > 0 {
		return DimensionProof{
			Dimension:    "ref",
			State:        ScopeUnknown,
			Left:         left,
			Right:        right,
			Intersection: []string{},
			Witnesses:    sortedUniqueSetKeys(unknownEvidence),
		}, nil
	}
	return DimensionProof{Dimension: "ref", State: ScopeDisjoint, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}, nil
}

// sortedUniqueSetKeys returns m's keys in canonical lexical order.
func sortedUniqueSetKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
