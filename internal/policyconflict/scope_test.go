package policyconflict

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/policyartifact"
)

// --- test fixtures / fakes ------------------------------------------------

func scopeWith(phases, environments, paths, refs []string) policyartifact.Scope {
	return policyartifact.Scope{Phases: phases, Environments: environments, Paths: paths, Refs: refs}
}

// universalScope is declared once for the package in operand_test.go.

// scopeRefs returns a scope whose only non-universal dimension is Refs —
// convenient for isolating the ref dimension in a pairwise/N-way call.
func scopeRefs(refs ...string) policyartifact.Scope {
	return scopeWith(nil, nil, nil, refs)
}

// fakeResolver is a hermetic, deterministic RefRelationResolver fake:
// configured responses keyed by the canonical unordered pair, every call
// recorded for assertions, and an optional forced error/invalid-state mode.
type fakeResolver struct {
	t           *testing.T
	responses   map[refPairKey]fakeResolverResponse
	calls       []refPairKey
	err         error
	returnBogus bool
}

type fakeResolverResponse struct {
	state ScopeState
	wit   []string
}

func newFakeResolver(t *testing.T) *fakeResolver {
	t.Helper()
	return &fakeResolver{t: t, responses: make(map[refPairKey]fakeResolverResponse)}
}

func (f *fakeResolver) set(a, b string, state ScopeState, wit ...string) *fakeResolver {
	f.responses[canonicalRefPair(a, b)] = fakeResolverResponse{state: state, wit: append([]string(nil), wit...)}
	return f
}

func (f *fakeResolver) Relate(_ context.Context, a, b string) (ScopeState, []string, error) {
	f.calls = append(f.calls, refPairKey{a, b})
	if f.err != nil {
		return "", nil, f.err
	}
	if f.returnBogus {
		return ScopeState("not-a-real-state"), nil, nil
	}
	r, ok := f.responses[canonicalRefPair(a, b)]
	if !ok {
		f.t.Fatalf("fakeResolver: no response configured for %q vs %q", a, b)
	}
	return r.state, append([]string(nil), r.wit...), nil
}

// noCallResolver fails the test the instant Relate is invoked — used to
// prove a code path never reaches the resolver (exact-equal refs, a
// universal ref dimension).
type noCallResolver struct{ t *testing.T }

func (n noCallResolver) Relate(_ context.Context, a, b string) (ScopeState, []string, error) {
	n.t.Fatalf("resolver.Relate called unexpectedly for %q vs %q", a, b)
	return "", nil, nil
}

var errResolverBoom = errors.New("boom")

// dimOf finds proof's dimension row named name or fails the test.
func dimOf(t *testing.T, proof ScopeProof, name string) DimensionProof {
	t.Helper()
	for _, d := range proof.Dimensions {
		if d.Dimension == name {
			return d
		}
	}
	t.Fatalf("dimension %q not present in proof %+v", name, proof)
	return DimensionProof{}
}

func mustValidScopeProof(t *testing.T, proof ScopeProof) {
	t.Helper()
	if err := validateScopeProof("proof", proof); err != nil {
		t.Fatalf("produced ScopeProof fails the package's own validation: %v\nproof: %+v", err, proof)
	}
}

func assertDimOrder(t *testing.T, proof ScopeProof) {
	t.Helper()
	want := []string{"phase", "environment", "path", "ref"}
	if len(proof.Dimensions) != len(want) {
		t.Fatalf("dimension count = %d, want %d (%+v)", len(proof.Dimensions), len(want), proof.Dimensions)
	}
	for i, d := range proof.Dimensions {
		if d.Dimension != want[i] {
			t.Fatalf("dimensions[%d] = %q, want %q (fixed phase/environment/path/ref order)", i, d.Dimension, want[i])
		}
	}
}

func strEq(t *testing.T, field string, got, want []string) {
	t.Helper()
	if got == nil {
		got = []string{}
	}
	if want == nil {
		want = []string{}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", field, got, want)
	}
}

// --- CompareScopes: universal dimensions ----------------------------------

func TestCompareScopesUniversal(t *testing.T) {
	ctx := context.Background()

	t.Run("both sides universal on every dimension overlaps", func(t *testing.T) {
		proof, err := CompareScopes(ctx, universalScope(), universalScope(), noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeOverlap {
			t.Fatalf("state = %q, want overlap", proof.State)
		}
		for _, name := range []string{"phase", "environment", "path", "ref"} {
			d := dimOf(t, proof, name)
			if d.State != ScopeOverlap {
				t.Fatalf("%s state = %q, want overlap", name, d.State)
			}
			strEq(t, name+".left", d.Left, []string{})
			strEq(t, name+".right", d.Right, []string{})
			strEq(t, name+".intersection", d.Intersection, []string{})
			strEq(t, name+".witnesses", d.Witnesses, []string{})
		}
		assertDimOrder(t, proof)
		mustValidScopeProof(t, proof)
	})

	t.Run("universal side overlaps a concrete side on every dimension", func(t *testing.T) {
		concrete := scopeWith([]string{"build", "design"}, []string{"prod"}, []string{"cmd/"}, []string{"adr/alpha"})
		proof, err := CompareScopes(ctx, universalScope(), concrete, newFakeResolver(t))
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeOverlap {
			t.Fatalf("state = %q, want overlap", proof.State)
		}
		phase := dimOf(t, proof, "phase")
		strEq(t, "phase.left", phase.Left, []string{})
		strEq(t, "phase.right", phase.Right, []string{"build", "design"})
		strEq(t, "phase.intersection", phase.Intersection, []string{"build", "design"})
		ref := dimOf(t, proof, "ref")
		strEq(t, "ref.intersection", ref.Intersection, []string{"adr/alpha"})
		mustValidScopeProof(t, proof)
	})
}

// --- CompareScopes: phase / environment truth table -----------------------

func TestCompareScopesPhaseEnvironment(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		left   []string
		right  []string
		want   ScopeState
		wantIn []string
	}{
		{"equal sets overlap", []string{"build", "design"}, []string{"build", "design"}, ScopeOverlap, []string{"build", "design"}},
		{"intersecting sets overlap", []string{"build", "design"}, []string{"build", "review"}, ScopeOverlap, []string{"build"}},
		{"disjoint sets disjoint", []string{"design"}, []string{"review"}, ScopeDisjoint, nil},
	}
	for _, dimName := range []string{"phase", "environment"} {
		for _, c := range cases {
			t.Run(dimName+"/"+c.name, func(t *testing.T) {
				var a, b policyartifact.Scope
				if dimName == "phase" {
					a, b = scopeWith(c.left, nil, nil, nil), scopeWith(c.right, nil, nil, nil)
				} else {
					a, b = scopeWith(nil, c.left, nil, nil), scopeWith(nil, c.right, nil, nil)
				}
				proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
				if err != nil {
					t.Fatalf("CompareScopes: %v", err)
				}
				d := dimOf(t, proof, dimName)
				if d.State != c.want {
					t.Fatalf("%s state = %q, want %q", dimName, d.State, c.want)
				}
				strEq(t, dimName+".intersection", d.Intersection, c.wantIn)
				if proof.State != c.want {
					t.Fatalf("overall state = %q, want %q (only this dimension is non-universal)", proof.State, c.want)
				}
				mustValidScopeProof(t, proof)
			})
		}
	}
}

// --- CompareScopes: path truth table ---------------------------------------

func TestCompareScopesPaths(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name   string
		left   string
		right  string
		want   ScopeState
		wantIn []string
	}{
		{"exact match overlaps", "cmd/verdi/main.go", "cmd/verdi/main.go", ScopeOverlap, []string{"cmd/verdi/main.go"}},
		{"entry inside subtree overlaps", "cmd/verdi/main.go", "cmd/", ScopeOverlap, []string{"cmd/verdi/main.go"}},
		{"subtree inside subtree overlaps", "cmd/", "cmd/verdi/", ScopeOverlap, []string{"cmd/verdi/"}},
		{"subtree inside subtree overlaps reversed", "cmd/verdi/", "cmd/", ScopeOverlap, []string{"cmd/verdi/"}},
		{"sibling subtrees disjoint", "cmd/", "internal/", ScopeDisjoint, nil},
		{"segment-boundary prefix collision disjoint", "cmd/", "cmdline/x", ScopeDisjoint, nil},
		{"distinct exact entries disjoint", "a/b.go", "a/c.go", ScopeDisjoint, nil},
		{"directory marker distinct from same-named exact entry", "a/", "a", ScopeDisjoint, nil},
		{"same-named exact entry distinct from directory marker reversed", "a", "a/", ScopeDisjoint, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := scopeWith(nil, nil, []string{c.left}, nil)
			b := scopeWith(nil, nil, []string{c.right}, nil)
			proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
			if err != nil {
				t.Fatalf("CompareScopes: %v", err)
			}
			d := dimOf(t, proof, "path")
			if d.State != c.want {
				t.Fatalf("path state = %q, want %q", d.State, c.want)
			}
			strEq(t, "path.intersection", d.Intersection, c.wantIn)
			if proof.State != c.want {
				t.Fatalf("overall state = %q, want %q", proof.State, c.want)
			}
			mustValidScopeProof(t, proof)
		})
	}
}

// --- CompareScopes: ref dimension -------------------------------------------

func TestCompareScopesRefs(t *testing.T) {
	ctx := context.Background()

	t.Run("equal whole ref overlaps without resolver call", func(t *testing.T) {
		a := scopeRefs("adr/alpha")
		b := scopeRefs("adr/alpha")
		proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		d := dimOf(t, proof, "ref")
		if d.State != ScopeOverlap {
			t.Fatalf("ref state = %q, want overlap", d.State)
		}
		strEq(t, "ref.intersection", d.Intersection, []string{"adr/alpha"})
		mustValidScopeProof(t, proof)
	})

	t.Run("equal fragment ref overlaps without resolver call", func(t *testing.T) {
		a := scopeRefs("adr/alpha#decision")
		b := scopeRefs("adr/alpha#decision")
		proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if dimOf(t, proof, "ref").State != ScopeOverlap {
			t.Fatalf("ref state, want overlap")
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("pinned ref exact match overlaps without resolver call", func(t *testing.T) {
		pinned := "adr/alpha@1111111111111111111111111111111111111111#decision"
		a := scopeRefs(pinned)
		b := scopeRefs(pinned)
		proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if dimOf(t, proof, "ref").State != ScopeOverlap {
			t.Fatalf("ref state, want overlap")
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("universal ref side overlaps without resolver call", func(t *testing.T) {
		a := scopeRefs()
		b := scopeRefs("adr/alpha", "adr/beta")
		proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		d := dimOf(t, proof, "ref")
		if d.State != ScopeOverlap {
			t.Fatalf("ref state = %q, want overlap", d.State)
		}
		strEq(t, "ref.intersection", d.Intersection, []string{"adr/alpha", "adr/beta"})
		mustValidScopeProof(t, proof)
	})

	t.Run("governing parent ref overlaps its resolving child via resolver", func(t *testing.T) {
		resolver := newFakeResolver(t).set("feature/checkout", "story/checkout-api", ScopeOverlap, "governance-edge:feature/checkout->story/checkout-api")
		a := scopeRefs("feature/checkout")
		b := scopeRefs("story/checkout-api")
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		d := dimOf(t, proof, "ref")
		if d.State != ScopeOverlap {
			t.Fatalf("ref state = %q, want overlap", d.State)
		}
		// Both ref values participated directly in the proven overlap (each
		// is trivially in its own set and resolver-overlaps the other's
		// member); unlike paths, refs have no hierarchical "narrower
		// representative" to collapse to, so both are legitimate witnesses.
		strEq(t, "ref.intersection", d.Intersection, []string{"feature/checkout", "story/checkout-api"})
		strEq(t, "ref.witnesses", d.Witnesses, []string{"governance-edge:feature/checkout->story/checkout-api"})
		if len(resolver.calls) != 1 {
			t.Fatalf("resolver calls = %d, want 1", len(resolver.calls))
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("separate accepted roots disjoint via resolver", func(t *testing.T) {
		resolver := newFakeResolver(t).set("adr/alpha", "adr/omega", ScopeDisjoint)
		a := scopeRefs("adr/alpha")
		b := scopeRefs("adr/omega")
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		d := dimOf(t, proof, "ref")
		if d.State != ScopeDisjoint {
			t.Fatalf("ref state = %q, want disjoint", d.State)
		}
		if proof.State != ScopeDisjoint {
			t.Fatalf("overall state = %q, want disjoint", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("missing ref evidence unknown via resolver, never guessed disjoint", func(t *testing.T) {
		resolver := newFakeResolver(t).set("adr/alpha", "adr/unclear", ScopeUnknown, "no-governing-edge-found")
		a := scopeRefs("adr/alpha")
		b := scopeRefs("adr/unclear")
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		d := dimOf(t, proof, "ref")
		if d.State != ScopeUnknown {
			t.Fatalf("ref state = %q, want unknown", d.State)
		}
		strEq(t, "ref.intersection", d.Intersection, []string{})
		strEq(t, "ref.witnesses", d.Witnesses, []string{"no-governing-edge-found"})
		if proof.State != ScopeUnknown {
			t.Fatalf("overall state = %q, want unknown", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("resolver error is operational and fails the call", func(t *testing.T) {
		resolver := newFakeResolver(t)
		resolver.err = errResolverBoom
		a := scopeRefs("adr/alpha")
		b := scopeRefs("adr/beta")
		_, err := CompareScopes(ctx, a, b, resolver)
		if err == nil {
			t.Fatalf("CompareScopes: want error, got nil")
		}
		if !errors.Is(err, errResolverBoom) {
			t.Fatalf("CompareScopes error = %v, want it to wrap %v", err, errResolverBoom)
		}
	})

	t.Run("resolver returning an out-of-vocabulary state fails closed", func(t *testing.T) {
		resolver := newFakeResolver(t)
		resolver.returnBogus = true
		a := scopeRefs("adr/alpha")
		b := scopeRefs("adr/beta")
		_, err := CompareScopes(ctx, a, b, resolver)
		if err == nil {
			t.Fatalf("CompareScopes: want error for an unknown resolver state, got nil")
		}
	})
}

// --- CompareScopes: product rule --------------------------------------------

func TestCompareScopesProductRule(t *testing.T) {
	ctx := context.Background()

	t.Run("one disjoint dimension makes the whole pair disjoint", func(t *testing.T) {
		a := scopeWith([]string{"build"}, nil, []string{"cmd/"}, nil)
		b := scopeWith([]string{"build"}, nil, []string{"internal/"}, nil)
		proof, err := CompareScopes(ctx, a, b, noCallResolver{t})
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeDisjoint {
			t.Fatalf("state = %q, want disjoint", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("all four dimensions proven overlap makes the whole pair overlap", func(t *testing.T) {
		resolver := newFakeResolver(t)
		a := scopeWith([]string{"build"}, []string{"prod"}, []string{"cmd/"}, []string{"adr/alpha"})
		b := scopeWith([]string{"build"}, []string{"prod"}, []string{"cmd/verdi/main.go"}, []string{"adr/alpha"})
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeOverlap {
			t.Fatalf("state = %q, want overlap", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("an unresolved ref dimension with everything else overlapping is unknown, never disjoint", func(t *testing.T) {
		resolver := newFakeResolver(t).set("adr/alpha", "adr/unclear", ScopeUnknown)
		a := scopeWith([]string{"build"}, nil, nil, []string{"adr/alpha"})
		b := scopeWith([]string{"build"}, nil, nil, []string{"adr/unclear"})
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeUnknown {
			t.Fatalf("state = %q, want unknown", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("a disjoint dimension outranks an unknown dimension", func(t *testing.T) {
		resolver := newFakeResolver(t).set("adr/alpha", "adr/unclear", ScopeUnknown)
		a := scopeWith([]string{"build"}, nil, nil, []string{"adr/alpha"})
		b := scopeWith([]string{"review"}, nil, nil, []string{"adr/unclear"})
		proof, err := CompareScopes(ctx, a, b, resolver)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		if proof.State != ScopeDisjoint {
			t.Fatalf("state = %q, want disjoint (proven-disjoint always wins over unknown)", proof.State)
		}
		mustValidScopeProof(t, proof)
	})
}

// --- non-transitive witness --------------------------------------------------

func TestCompareScopesNonTransitiveWitness(t *testing.T) {
	ctx := context.Background()
	resolver := newFakeResolver(t).
		set("a", "b", ScopeOverlap, "edge:a-b").
		set("b", "c", ScopeOverlap, "edge:b-c").
		set("a", "c", ScopeDisjoint)

	ab, err := CompareScopes(ctx, scopeRefs("a"), scopeRefs("b"), resolver)
	if err != nil {
		t.Fatalf("CompareScopes(a,b): %v", err)
	}
	if got := dimOf(t, ab, "ref").State; got != ScopeOverlap {
		t.Fatalf("a vs b ref state = %q, want overlap", got)
	}

	bc, err := CompareScopes(ctx, scopeRefs("b"), scopeRefs("c"), resolver)
	if err != nil {
		t.Fatalf("CompareScopes(b,c): %v", err)
	}
	if got := dimOf(t, bc, "ref").State; got != ScopeOverlap {
		t.Fatalf("b vs c ref state = %q, want overlap", got)
	}

	ac, err := CompareScopes(ctx, scopeRefs("a"), scopeRefs("c"), resolver)
	if err != nil {
		t.Fatalf("CompareScopes(a,c): %v", err)
	}
	if got := dimOf(t, ac, "ref").State; got != ScopeDisjoint {
		t.Fatalf("a vs c ref state = %q, want disjoint — overlap must never be assumed transitive", got)
	}
	mustValidScopeProof(t, ab)
	mustValidScopeProof(t, bc)
	mustValidScopeProof(t, ac)
}

// --- IntersectScopes: N-way ---------------------------------------------------

func TestIntersectScopesNWay(t *testing.T) {
	ctx := context.Background()

	t.Run("degenerate empty slice is the universal scope", func(t *testing.T) {
		proof, err := IntersectScopes(ctx, nil, noCallResolver{t})
		if err != nil {
			t.Fatalf("IntersectScopes(nil): %v", err)
		}
		if proof.State != ScopeOverlap {
			t.Fatalf("state = %q, want overlap (vacuous intersection convention)", proof.State)
		}
		for _, name := range []string{"phase", "environment", "path", "ref"} {
			d := dimOf(t, proof, name)
			if d.State != ScopeOverlap {
				t.Fatalf("%s state = %q, want overlap", name, d.State)
			}
			strEq(t, name+".intersection", d.Intersection, []string{})
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("degenerate single scope always proves overlap", func(t *testing.T) {
		one := scopeWith([]string{"build"}, nil, []string{"cmd/"}, []string{"adr/alpha"})
		proof, err := IntersectScopes(ctx, []policyartifact.Scope{one}, noCallResolver{t})
		if err != nil {
			t.Fatalf("IntersectScopes([one]): %v", err)
		}
		if proof.State != ScopeOverlap {
			t.Fatalf("state = %q, want overlap", proof.State)
		}
		phase := dimOf(t, proof, "phase")
		strEq(t, "phase.intersection", phase.Intersection, []string{"build"})
		env := dimOf(t, proof, "environment")
		strEq(t, "environment.intersection", env.Intersection, []string{})
		mustValidScopeProof(t, proof)
	})

	t.Run("IntersectScopes over exactly two scopes matches CompareScopes", func(t *testing.T) {
		resolver := newFakeResolver(t).set("adr/alpha", "adr/beta", ScopeOverlap, "edge")
		a := scopeWith([]string{"build", "design"}, []string{"prod"}, []string{"cmd/"}, []string{"adr/alpha"})
		b := scopeWith([]string{"build"}, []string{"prod", "staging"}, []string{"cmd/verdi/main.go"}, []string{"adr/beta"})

		via := newFakeResolver(t).set("adr/alpha", "adr/beta", ScopeOverlap, "edge")
		compared, err := CompareScopes(ctx, a, b, via)
		if err != nil {
			t.Fatalf("CompareScopes: %v", err)
		}
		intersected, err := IntersectScopes(ctx, []policyartifact.Scope{a, b}, resolver)
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		if !reflect.DeepEqual(compared, intersected) {
			t.Fatalf("CompareScopes(a,b) != IntersectScopes([a,b])\nCompareScopes:   %+v\nIntersectScopes: %+v", compared, intersected)
		}
		mustValidScopeProof(t, intersected)
	})

	t.Run("pairwise-overlapping environment sets with empty total intersection are disjoint", func(t *testing.T) {
		s1 := scopeWith(nil, []string{"build", "design"}, nil, nil)
		s2 := scopeWith(nil, []string{"build", "review"}, nil, nil)
		s3 := scopeWith(nil, []string{"design", "review"}, nil, nil)

		// Sanity: every pair overlaps.
		for _, pair := range [][2]policyartifact.Scope{{s1, s2}, {s1, s3}, {s2, s3}} {
			p, err := CompareScopes(ctx, pair[0], pair[1], noCallResolver{t})
			if err != nil {
				t.Fatalf("pairwise CompareScopes: %v", err)
			}
			if dimOf(t, p, "environment").State != ScopeOverlap {
				t.Fatalf("expected this pair to overlap on environment as test setup")
			}
		}

		proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, noCallResolver{t})
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		env := dimOf(t, proof, "environment")
		if env.State != ScopeDisjoint {
			t.Fatalf("environment state = %q, want disjoint (no single value is common to all three sets)", env.State)
		}
		if proof.State != ScopeDisjoint {
			t.Fatalf("overall state = %q, want disjoint", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("pairwise-overlapping path sets with empty total intersection are disjoint", func(t *testing.T) {
		s1 := scopeWith(nil, nil, []string{"a/", "b/"}, nil)
		s2 := scopeWith(nil, nil, []string{"a/x", "c/"}, nil)
		s3 := scopeWith(nil, nil, []string{"b/y", "c/z"}, nil)

		for _, pair := range [][2]policyartifact.Scope{{s1, s2}, {s1, s3}, {s2, s3}} {
			p, err := CompareScopes(ctx, pair[0], pair[1], noCallResolver{t})
			if err != nil {
				t.Fatalf("pairwise CompareScopes: %v", err)
			}
			if dimOf(t, p, "path").State != ScopeOverlap {
				t.Fatalf("expected this pair to overlap on path as test setup")
			}
		}

		proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, noCallResolver{t})
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		path := dimOf(t, proof, "path")
		if path.State != ScopeDisjoint {
			t.Fatalf("path state = %q, want disjoint", path.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("pairwise-overlapping ref sets with unknown total intersection are unknown", func(t *testing.T) {
		resolver := newFakeResolver(t).
			set("a", "b", ScopeOverlap, "edge:a-b").
			set("b", "c", ScopeUnknown, "no-governing-edge-found:b-c").
			set("a", "c", ScopeUnknown, "no-governing-edge-found:a-c")
		s1, s2, s3 := scopeRefs("a"), scopeRefs("b"), scopeRefs("c")

		ab, err := CompareScopes(ctx, s1, s2, resolver)
		if err != nil {
			t.Fatalf("CompareScopes(a,b): %v", err)
		}
		if dimOf(t, ab, "ref").State != ScopeOverlap {
			t.Fatalf("expected a vs b to overlap as test setup")
		}

		proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, resolver)
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		ref := dimOf(t, proof, "ref")
		if ref.State != ScopeUnknown {
			t.Fatalf("ref state = %q, want unknown", ref.State)
		}
		strEq(t, "ref.intersection", ref.Intersection, []string{})
		if proof.State != ScopeUnknown {
			t.Fatalf("overall state = %q, want unknown", proof.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("pairwise-overlapping ref sets with proven-empty total intersection are disjoint", func(t *testing.T) {
		resolver := newFakeResolver(t).
			set("a", "b", ScopeOverlap, "edge:a-b").
			set("c", "d", ScopeOverlap, "edge:c-d").
			set("a", "c", ScopeDisjoint).
			set("a", "d", ScopeDisjoint).
			set("b", "c", ScopeDisjoint).
			set("b", "d", ScopeDisjoint)
		s1, s2 := scopeRefs("a", "b"), scopeRefs("c", "d")

		proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2}, resolver)
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		ref := dimOf(t, proof, "ref")
		if ref.State != ScopeDisjoint {
			t.Fatalf("ref state = %q, want disjoint", ref.State)
		}
		mustValidScopeProof(t, proof)
	})

	t.Run("resolver error inside an N-way call is operational and fails the whole call", func(t *testing.T) {
		resolver := newFakeResolver(t)
		resolver.err = errResolverBoom
		s1, s2, s3 := scopeRefs("a"), scopeRefs("b"), scopeRefs("c")
		_, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, resolver)
		if !errors.Is(err, errResolverBoom) {
			t.Fatalf("IntersectScopes error = %v, want it to wrap %v", err, errResolverBoom)
		}
	})

	t.Run("nil resolver is never dereferenced when every ref relation is exact-equal or universal", func(t *testing.T) {
		s1 := scopeRefs("adr/alpha")
		s2 := scopeRefs("adr/alpha")
		s3 := scopeRefs()
		proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, nil)
		if err != nil {
			t.Fatalf("IntersectScopes: %v", err)
		}
		if dimOf(t, proof, "ref").State != ScopeOverlap {
			t.Fatalf("ref state, want overlap")
		}
		mustValidScopeProof(t, proof)
	})
}

// --- ref pair resolver call caching ------------------------------------------

func TestIntersectScopesRefResolverCaching(t *testing.T) {
	ctx := context.Background()
	resolver := newFakeResolver(t).set("a", "b", ScopeOverlap, "edge:a-b")
	s1, s2 := scopeRefs("a"), scopeRefs("b")

	proof, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2}, resolver)
	if err != nil {
		t.Fatalf("IntersectScopes: %v", err)
	}
	if dimOf(t, proof, "ref").State != ScopeOverlap {
		t.Fatalf("ref state, want overlap")
	}
	if len(resolver.calls) != 1 {
		t.Fatalf("resolver calls = %d, want exactly 1 (the (a,b) pair reached from both candidate directions must be cached, not re-queried)", len(resolver.calls))
	}
}

// --- fixed dimension/witness ordering ----------------------------------------

func TestFixedDimensionAndWitnessOrdering(t *testing.T) {
	ctx := context.Background()
	resolver := newFakeResolver(t).set("adr/zeta", "adr/alpha", ScopeOverlap, "z-edge", "a-edge")
	a := scopeWith([]string{"review", "build"}, []string{"prod"}, []string{"cmd/verdi/main.go", "cmd/"}, []string{"adr/zeta"})
	b := scopeWith([]string{"build", "review"}, []string{"prod", "prod"}, []string{"cmd/"}, []string{"adr/alpha"})
	proof, err := CompareScopes(ctx, a, b, resolver)
	if err != nil {
		t.Fatalf("CompareScopes: %v", err)
	}
	assertDimOrder(t, proof)
	for _, d := range proof.Dimensions {
		for i := 1; i < len(d.Left); i++ {
			if d.Left[i-1] >= d.Left[i] {
				t.Fatalf("%s.left not canonical-lexical-order/unique: %v", d.Dimension, d.Left)
			}
		}
		for i := 1; i < len(d.Right); i++ {
			if d.Right[i-1] >= d.Right[i] {
				t.Fatalf("%s.right not canonical-lexical-order/unique: %v", d.Dimension, d.Right)
			}
		}
		for i := 1; i < len(d.Intersection); i++ {
			if d.Intersection[i-1] >= d.Intersection[i] {
				t.Fatalf("%s.intersection not canonical-lexical-order/unique: %v", d.Dimension, d.Intersection)
			}
		}
		for i := 1; i < len(d.Witnesses); i++ {
			if d.Witnesses[i-1] >= d.Witnesses[i] {
				t.Fatalf("%s.witnesses not canonical-lexical-order/unique: %v", d.Dimension, d.Witnesses)
			}
		}
	}
	mustValidScopeProof(t, proof)
}

// --- determinism ---------------------------------------------------------

func TestCompareScopesDeterminism(t *testing.T) {
	ctx := context.Background()
	newInputs := func() (policyartifact.Scope, policyartifact.Scope, *fakeResolver) {
		a := scopeWith([]string{"build", "design"}, []string{"prod"}, []string{"cmd/", "internal/"}, []string{"adr/alpha", "feature/checkout"})
		b := scopeWith([]string{"build"}, []string{"prod", "staging"}, []string{"cmd/verdi/main.go"}, []string{"adr/alpha", "story/checkout-api"})
		resolver := newFakeResolver(t).
			set("feature/checkout", "story/checkout-api", ScopeOverlap, "edge:1").
			set("adr/alpha", "story/checkout-api", ScopeUnknown, "no-governing-edge-found").
			set("feature/checkout", "adr/alpha", ScopeDisjoint)
		return a, b, resolver
	}

	a1, b1, r1 := newInputs()
	proof1, err := CompareScopes(ctx, a1, b1, r1)
	if err != nil {
		t.Fatalf("CompareScopes (first run): %v", err)
	}
	a2, b2, r2 := newInputs()
	proof2, err := CompareScopes(ctx, a2, b2, r2)
	if err != nil {
		t.Fatalf("CompareScopes (second run): %v", err)
	}
	if !reflect.DeepEqual(proof1, proof2) {
		t.Fatalf("CompareScopes is not deterministic across identical inputs:\nrun 1: %+v\nrun 2: %+v", proof1, proof2)
	}
	mustValidScopeProof(t, proof1)
	mustValidScopeProof(t, proof2)
}

func TestIntersectScopesDeterminism(t *testing.T) {
	ctx := context.Background()
	newInputs := func() ([]policyartifact.Scope, *fakeResolver) {
		s1 := scopeWith([]string{"build", "design"}, nil, []string{"cmd/"}, []string{"a"})
		s2 := scopeWith([]string{"build"}, nil, []string{"cmd/verdi/"}, []string{"b"})
		s3 := scopeWith([]string{"build", "review"}, nil, []string{"cmd/verdi/main.go"}, []string{"c"})
		resolver := newFakeResolver(t).
			set("a", "b", ScopeOverlap, "edge:a-b").
			set("b", "c", ScopeOverlap, "edge:b-c").
			set("a", "c", ScopeUnknown, "no-governing-edge-found")
		return []policyartifact.Scope{s1, s2, s3}, resolver
	}

	scopes1, r1 := newInputs()
	proof1, err := IntersectScopes(ctx, scopes1, r1)
	if err != nil {
		t.Fatalf("IntersectScopes (first run): %v", err)
	}
	scopes2, r2 := newInputs()
	proof2, err := IntersectScopes(ctx, scopes2, r2)
	if err != nil {
		t.Fatalf("IntersectScopes (second run): %v", err)
	}
	if !reflect.DeepEqual(proof1, proof2) {
		t.Fatalf("IntersectScopes is not deterministic across identical inputs:\nrun 1: %+v\nrun 2: %+v", proof1, proof2)
	}
	mustValidScopeProof(t, proof1)
	mustValidScopeProof(t, proof2)
}

// --- negative paths beyond resolver errors -----------------------------------

func TestCompareScopesNegativePaths(t *testing.T) {
	ctx := context.Background()

	t.Run("resolver error surfaces even when only one of many dimensions needs it", func(t *testing.T) {
		resolver := newFakeResolver(t)
		resolver.err = fmt.Errorf("network unreachable: %w", errResolverBoom)
		a := scopeWith([]string{"build"}, []string{"prod"}, []string{"cmd/"}, []string{"adr/alpha"})
		b := scopeWith([]string{"build"}, []string{"prod"}, []string{"cmd/"}, []string{"adr/omega"})
		_, err := CompareScopes(ctx, a, b, resolver)
		if !errors.Is(err, errResolverBoom) {
			t.Fatalf("CompareScopes error = %v, want it to wrap %v", err, errResolverBoom)
		}
	})
}

func TestIntersectScopesNegativePaths(t *testing.T) {
	ctx := context.Background()

	t.Run("out-of-vocabulary resolver state inside an N-way call fails closed", func(t *testing.T) {
		resolver := newFakeResolver(t)
		resolver.returnBogus = true
		s1, s2, s3 := scopeRefs("a"), scopeRefs("b"), scopeRefs("c")
		_, err := IntersectScopes(ctx, []policyartifact.Scope{s1, s2, s3}, resolver)
		if err == nil {
			t.Fatalf("IntersectScopes: want error for an unknown resolver state, got nil")
		}
	})
}
