package lintratchet

import (
	"slices"
	"testing"
)

// TestCompare covers the two comparisons between the current run and the
// committed baseline (spec/strict-lint-gate ac-2): a key the run reports
// more times than the baseline allows is a new finding, and a key the
// baseline allows more times than the run reports is a stale allowance.
// Counts, never presence, decide both.
func TestCompare(t *testing.T) {
	a := Key{Linter: "errorlint", Package: "p", Message: "m", Source: "s"}
	b := Key{Linter: "noctx", Package: "p", Message: "m", Source: "s"}
	c := Key{Linter: "noctx", Package: "q", Message: "m", Source: "s"}
	cases := []struct {
		name              string
		current, baseline Counts
		want              []Violation
	}{
		{name: "both empty", current: Counts{}, baseline: Counts{}},
		{name: "equal", current: Counts{a: 2, b: 1}, baseline: Counts{a: 2, b: 1}},
		{name: "a new key", current: Counts{a: 1, b: 1}, baseline: Counts{a: 1}, want: []Violation{{Kind: KindNew, Key: b, Have: 1, Allowed: 0}}},
		{name: "a key reported once more than allowed", current: Counts{a: 3}, baseline: Counts{a: 2}, want: []Violation{{Kind: KindNew, Key: a, Have: 3, Allowed: 2}}},
		{name: "a fixed key still allowed", current: Counts{}, baseline: Counts{a: 1}, want: []Violation{{Kind: KindStale, Key: a, Have: 0, Allowed: 1}}},
		{name: "a key allowed once more than reported", current: Counts{a: 1}, baseline: Counts{a: 2}, want: []Violation{{Kind: KindStale, Key: a, Have: 1, Allowed: 2}}},
		{
			name:     "both kinds, new before stale, each sorted by key",
			current:  Counts{c: 1, a: 1},
			baseline: Counts{b: 1, a: 1},
			want:     []Violation{{Kind: KindNew, Key: c, Have: 1, Allowed: 0}, {Kind: KindStale, Key: b, Have: 0, Allowed: 1}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Compare(tc.current, tc.baseline); !slices.Equal(got, tc.want) {
				t.Fatalf("Compare = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestCompareGrowth covers the growth comparison: the committed baseline
// may allow no key more times than the baseline at the merge base (or, on
// the default branch, at HEAD's first parent) allowed it.
func TestCompareGrowth(t *testing.T) {
	a := Key{Linter: "errorlint", Package: "p", Message: "m", Source: "s"}
	b := Key{Linter: "noctx", Package: "p", Message: "m", Source: "s"}
	cases := []struct {
		name              string
		baseline, earlier Counts
		want              []Violation
	}{
		{name: "unchanged", baseline: Counts{a: 1}, earlier: Counts{a: 1}},
		{name: "shrunk", baseline: Counts{a: 1}, earlier: Counts{a: 2, b: 1}},
		{name: "emptied", baseline: Counts{}, earlier: Counts{a: 2}},
		{name: "a key added", baseline: Counts{a: 1, b: 1}, earlier: Counts{a: 1}, want: []Violation{{Kind: KindGrowth, Key: b, Have: 1, Allowed: 0}}},
		{name: "a key's count raised", baseline: Counts{a: 2}, earlier: Counts{a: 1}, want: []Violation{{Kind: KindGrowth, Key: a, Have: 2, Allowed: 1}}},
		{name: "one grown, one shrunk", baseline: Counts{a: 3, b: 0}, earlier: Counts{a: 1, b: 4}, want: []Violation{{Kind: KindGrowth, Key: a, Have: 3, Allowed: 1}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareGrowth(tc.baseline, tc.earlier); !slices.Equal(got, tc.want) {
				t.Fatalf("CompareGrowth = %+v, want %+v", got, tc.want)
			}
		})
	}
}

// TestKindString covers each violation kind's name and the fail-closed
// rendering of a value outside the closed set.
func TestKindString(t *testing.T) {
	cases := []struct {
		kind Kind
		want string
	}{
		{KindNew, "new finding"},
		{KindStale, "stale allowance"},
		{KindGrowth, "baseline grew"},
		{Kind(0), "unknown violation kind 0"},
		{Kind(9), "unknown violation kind 9"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(tc.kind), got, tc.want)
		}
	}
}
