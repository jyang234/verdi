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
//   - Zero scopes: an operational error, never a proof. The vacuous
//     convention (intersecting over an empty family is the whole universe)
//     is mathematically defensible but wrong here: §5's consumers read a
//     returned proof as authority over the claims that produced it, so
//     answering "universal overlap" for a call that supplied no operand at
//     all would score an unsatisfiable witness against a scope nobody ever
//     stated. A zero-scope call is always a caller defect, so this package
//     fails closed instead of proving something about the universe.
//   - One scope: intersecting a single set with only itself is always that
//     set (there is no second participant to conflict with), so every
//     dimension is proven overlap and the result State is always overlap.
//     This falls out of the same engine with no special-casing (see the
//     "exactly one non-universal participant" branch below).
package policyconflict

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// The two operational (never verdict) failures this file can report: each one
// is a defect in how the caller invoked scope comparison, not a fact about
// any scope, so neither may ever be answered with a ScopeState.
var (
	errNoScopes       = errors.New("policyconflict: intersect scopes: at least one scope is required")
	errNilRefResolver = errors.New("policyconflict: a ref relation resolver is required to relate two different refs")
)

// RefRelationResolver is scope comparison's one seam onto the accepted/
// candidate artifact graph (authority design §4.3): for two DIFFERENT ref
// strings it proves overlap, disjoint, or unknown, and returns the exact
// graph-edge witnesses supporting that state. Scope comparison never parses
// ref structure or reasons about the graph itself — every non-identical ref
// pair is resolved exclusively through this port.
//
// Two guarantees implementations may rely on and must be prepared for:
//   - Arguments always arrive in canonical lexical order (a <= b), so an
//     implementation sees each unordered pair under exactly one argument
//     order and may key its own caches on that order.
//   - The pair is not always drawn from two different scopes: proving one
//     scope's own ref against another member of that SAME scope's ref set is
//     a normal query, so an implementation must answer intra-set pairs
//     rather than assuming the two refs come from opposing operands.
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
	if len(scopes) == 0 {
		return ScopeProof{}, errNoScopes
	}
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

// pathContains reports whether every entry the selector p names is also named
// by the selector container. Authority design §4.2's three overlap cases are
// each a containment: exact-equal values contain each other, a directory
// subtree contains an exact entry inside it, and a directory subtree contains
// another subtree beneath it — so two path values overlap exactly when one of
// them contains the other. The test is segment-aware for free, because a
// subtree selector always carries its own trailing slash and a plain prefix
// test therefore cannot cross a segment boundary: "cmd/" contains
// "cmd/verdi/main.go" but not "cmdline/x", and the exact entry "a" neither
// contains nor is contained by the directory "a/".
func pathContains(container, p string) bool {
	if container == p {
		return true
	}
	return pathIsSubtree(container) && strings.HasPrefix(p, container)
}

// intersectPathDimension proves the path dimension's total N-way
// intersection (authority design §4.2/§4.4). Two path selectors either
// contain one another or name disjoint regions, so the intersection of two
// selectors is always the narrower one — which means every maximal region of
// the total intersection is itself one of the given values, and the union of
// every participating set's values is a complete candidate list.
//
// A candidate c belongs to the total intersection exactly when EVERY
// non-universal participating set contains it: some member t of that set must
// satisfy pathContains(t, c). Coverage is containment, not mere pair overlap
// — a candidate that merely overlaps some member of every set can still name
// a region two of those sets do not share (internal/ overlaps both
// internal/policy/ and internal/context/, yet those two share nothing), and
// admitting it would manufacture an intersection out of non-transitive pair
// overlap (§5). Each test is between the fixed candidate and a member of the
// set actually being tested, never a chain through a third value.
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
					if pathContains(t, c) {
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
		witnesses := broadestPathWitnesses(covered)
		if len(witnesses) == 0 {
			return DimensionProof{Dimension: "path", State: ScopeDisjoint, Left: left, Right: right, Intersection: []string{}, Witnesses: []string{}}
		}
		return DimensionProof{Dimension: "path", State: ScopeOverlap, Left: left, Right: right, Intersection: witnesses, Witnesses: witnesses}
	}
}

// broadestPathWitnesses reduces the covered candidates to the exact
// intersection region (authority design §4.4's "exact ... path ...
// witness"). Every covered candidate is genuinely inside the intersection,
// but one contained in another covered candidate adds no region the broader
// one does not already carry, so only the maximal covered selectors are
// reported: if {a/, a/b/} is intersected with itself, both sets denote
// exactly the a/ subtree and the shared region is a/, so reporting a/b/
// would understate what the sets share. Covered candidates that do not
// contain one another are separate shared regions and are all kept. covered
// must already be canonical-lexical-order; the result preserves that order.
func broadestPathWitnesses(covered []string) []string {
	out := make([]string, 0, len(covered))
	for _, c := range covered {
		insideAnother := false
		for _, d := range covered {
			if d != c && pathContains(d, c) {
				insideAnother = true
				break
			}
		}
		if !insideAnother {
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
// closed here just as it does at every decode boundary. A missing resolver is
// the same class of failure one step earlier: with no port there is no
// evidence at all, so it is reported operationally rather than crashing or
// standing in for a relation nobody proved.
func resolveRefPair(ctx context.Context, cache map[refPairKey]refPairResolution, resolver RefRelationResolver, a, b string) (ScopeState, []string, error) {
	if a == b {
		return ScopeOverlap, nil, nil
	}
	key := canonicalRefPair(a, b)
	if r, ok := cache[key]; ok {
		return r.state, r.wit, nil
	}
	if resolver == nil {
		return "", nil, fmt.Errorf("%w: %q vs %q", errNilRefResolver, key.lo, key.hi)
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

// intersectRefDimension proves the ref dimension's total N-way intersection
// (authority design §4.3/§4.4). It mirrors intersectPathDimension's
// candidate/coverage shape, but a relation between a candidate and a set
// member is proven exclusively by exact-equality or resolveRefPair — never
// inferred from any other pair. Refs have no containment grammar to reduce
// with, so what a candidate must satisfy depends on the arity:
//
//   - Exactly two participating sets. A resolver overlap between the
//     candidate and a member of the other set IS direct evidence about the
//     only relation the pair asserts, so a candidate that overlaps at least
//     one member of both sets is proven.
//   - More than two participating sets. Separate pairwise proofs against
//     different sets never compose: overlap "is not assumed transitive and
//     is never used as an equivalence relation" (§4.4), so "b overlaps a"
//     plus "b overlaps c" says nothing about whether a's set and c's set
//     share anything, and admitting b would manufacture a conflict from
//     non-transitive pair overlap (§5). The only ref such a group is proven
//     to share is one every participating set exactly contains, which is
//     direct evidence from every set at once.
//
// Exclusion is arity-independent and unchanged: a candidate is soundly
// excluded only when some set's every member answers disjoint. A candidate
// that is neither proven nor excluded leaves the intersection unresolved, so
// the dimension is unknown — withholding a mechanical conclusion rather than
// inventing either verdict (§5).
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
	pairwise := len(nonUniversal) == 2

	proven := make([]string, 0)
	ambiguous := make([]string, 0)
	overlapEvidence := make(map[string]bool)
	unknownEvidence := make(map[string]bool)

	for _, c := range candidates {
		excluded := false
		overlapEverywhere := true
		// Overlap and unknown witnesses stay separated: an unknown result
		// proves nothing, so it may never be reported as the evidence
		// supporting a proven overlap (§4.4's "exact ... graph-edge
		// witness" is a witness FOR the recorded state).
		var overlapWit, unknownWit []string
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
					overlapWit = append(overlapWit, wit...)
				case ScopeUnknown:
					setAmbiguous = true
					unknownWit = append(unknownWit, wit...)
				}
			}
			if setOverlap {
				continue
			}
			if setAmbiguous {
				overlapEverywhere = false
				continue
			}
			// Every member of this set answered disjoint, so no part of
			// this candidate can be in the total intersection.
			excluded = true
			break
		}
		if excluded {
			continue
		}
		candidateProven := overlapEverywhere
		if !pairwise {
			candidateProven = presentInEverySet(nonUniversal, c)
		}
		if candidateProven {
			proven = append(proven, c)
			for _, w := range overlapWit {
				overlapEvidence[w] = true
			}
			continue
		}
		// Not proven and not soundly excluded: this candidate's own
		// membership in the total intersection is unresolved. Every witness
		// consulted along the way is recorded as evidence for that unknown.
		ambiguous = append(ambiguous, c)
		for _, w := range overlapWit {
			unknownEvidence[w] = true
		}
		for _, w := range unknownWit {
			unknownEvidence[w] = true
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

// presentInEverySet reports whether v appears verbatim in every set in sets,
// which must already be canonical-lexical-order and duplicate-free. This is
// the only ref evidence that holds for three or more participants at once:
// exact equality is proven directly against each set, with no pairwise proof
// composed into another (§4.4).
func presentInEverySet(sets [][]string, v string) bool {
	for _, s := range sets {
		i := sort.SearchStrings(s, v)
		if i >= len(s) || s[i] != v {
			return false
		}
	}
	return true
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
