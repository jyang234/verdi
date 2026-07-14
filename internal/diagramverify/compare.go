package diagramverify

import (
	"context"
	"fmt"

	"github.com/jyang234/verdi/internal/gitx"
)

// Classification is the three-way structural comparison's CLOSED result
// set (spec/verification-extractor ac-3): exactly Exists, ProposedNew, and
// KeptButGone — deliberately no fourth "renamed" value anywhere (parent
// spec/diagram-proposals dc-5: a rename is two independent facts, never
// one inferred fact).
type Classification string

const (
	// Exists: the element is present in regenerated truth, whether or not
	// it was inherited from a base (spec/verification-extractor ac-3:
	// "proposal element present in truth").
	Exists Classification = "exists"
	// ProposedNew: the element is absent from truth and was not
	// inherited from a base — design intent, honestly unverifiable,
	// never "impossible" (parent ac-1).
	ProposedNew Classification = "proposed-new"
	// KeptButGone: the element was part of the proposal's base and truth
	// no longer has it — contradicted, the oversight-catcher (parent
	// ac-1/dc-5).
	KeptButGone Classification = "kept-but-gone"
)

// Result is one element's three-way comparison outcome. Witness is
// present ONLY when Classification == KeptButGone and DC-4's `git log -S`
// pickaxe search resolved a hit; nil otherwise (including every
// Exists/ProposedNew result, and an unresolved KeptButGone search) — never
// a fabricated placeholder commit. A non-nil Witness names a CANDIDATE
// witness only: a fixed-string pickaxe hit proves the identity string's
// occurrence count changed in that commit somewhere under the searched
// directory, never that the commit specifically removed THIS element
// (dc-4's corrected candor) — callers must render it as "candidate
// witness", never as a verified cause.
type Result struct {
	Identity       string
	Classification Classification
	Witness        *string
}

// Compare runs the three-way structural comparison (spec/
// verification-extractor ac-3) between a proposal's extracted identity set
// (proposal), its base's extracted identity set (base — nil for a
// from-scratch proposal), and truth's identity set (truth, a set
// membership map: TruthShortNames or TruthEdgeIdentities' shape).
//
// Two passes, deterministic (proposal order, then base order):
//
//  1. Every identity the proposal currently draws: Exists if truth has it;
//     else KeptButGone if it was inherited from base unedited (the base
//     had it, truth has since dropped it, and the proposal is still
//     drawing an assumption that is no longer true); else ProposedNew (the
//     proposal's own delta, absent from both base and truth).
//  2. Every identity in base the CURRENT proposal no longer draws at all
//     (edited away — a rename's old half, or a plain deletion) that truth
//     ALSO no longer has: KeptButGone. This is what makes a rename render
//     as two independent facts (dc-5) rather than one inferred "renamed"
//     fact — the dropped-away old identity still gets its own
//     contradiction disclosed, computed independently of whatever the
//     proposal replaced it with. A base identity the proposal dropped but
//     truth STILL has is not disclosed at all: the proposal simply chose
//     not to depict something real, which is not a contradiction.
func Compare(proposal, base []string, truth map[string]bool) []Result {
	baseSet := toSet(base)
	seen := make(map[string]bool, len(proposal))
	var out []Result

	for _, id := range dedupOrdered(proposal) {
		seen[id] = true
		switch {
		case truth[id]:
			out = append(out, Result{Identity: id, Classification: Exists})
		case baseSet[id]:
			out = append(out, Result{Identity: id, Classification: KeptButGone})
		default:
			out = append(out, Result{Identity: id, Classification: ProposedNew})
		}
	}

	for _, id := range dedupOrdered(base) {
		if seen[id] || truth[id] {
			continue
		}
		out = append(out, Result{Identity: id, Classification: KeptButGone})
	}

	return out
}

// ResolveWitness runs DC-4's candidate-witness discovery
// (gitx.PickaxeCommit — the only commit-discovery mechanism this story
// implements) for identity under paths in the git repository at dir,
// returning a pointer to the commit sha when a hit resolved, or nil (with
// a nil error) when the search found no hit at all: the caller discloses
// that as witness-unresolved, never a fabricated placeholder commit.
func ResolveWitness(ctx context.Context, dir, identity string, paths ...string) (*string, error) {
	sha, ok, err := gitx.PickaxeCommit(ctx, dir, identity, paths...)
	if err != nil {
		return nil, fmt.Errorf("diagramverify: resolving witness for %q: %w", identity, err)
	}
	if !ok {
		return nil, nil
	}
	return &sha, nil
}

// CompareWithWitness runs Compare and attaches a CANDIDATE witness commit
// (ResolveWitness) to each KeptButGone result — the shape AC-3's
// obligation calls for: exists/proposed-new results never carry a
// witness, a resolved kept-but-gone carries the candidate sha, an
// unresolved one carries nil.
func CompareWithWitness(ctx context.Context, dir string, proposal, base []string, truth map[string]bool, paths ...string) ([]Result, error) {
	results := Compare(proposal, base, truth)
	for i := range results {
		if results[i].Classification != KeptButGone {
			continue
		}
		w, err := ResolveWitness(ctx, dir, results[i].Identity, paths...)
		if err != nil {
			return nil, err
		}
		results[i].Witness = w
	}
	return results, nil
}

func toSet(xs []string) map[string]bool {
	m := make(map[string]bool, len(xs))
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func dedupOrdered(xs []string) []string {
	seen := make(map[string]bool, len(xs))
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if seen[x] {
			continue
		}
		seen[x] = true
		out = append(out, x)
	}
	return out
}
