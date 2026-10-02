package featurecoverage

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestStubDecls is the shared stub-to-declaration step (SI-338 (4)): a
// decoded feature's non-spike stubs, in declared order, each with the
// criteria it lists — the wall's own Compute input — so the readiness
// loader reads coverage through the same rule without a second copy of it.
func TestStubDecls(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		stubs []artifact.Stub
		want  []StubDecl
	}{
		{name: "nil stubs", stubs: nil, want: []StubDecl{}},
		{name: "no stubs", stubs: []artifact.Stub{}, want: []StubDecl{}},
		{
			name: "plain stubs in declared order",
			stubs: []artifact.Stub{
				{Slug: "zeta", AcceptanceCriteria: []string{"ac-2"}},
				{Slug: "alpha", AcceptanceCriteria: []string{"ac-1", "ac-3"}},
			},
			want: []StubDecl{
				{Slug: "zeta", AcceptanceCriteria: []string{"ac-2"}},
				{Slug: "alpha", AcceptanceCriteria: []string{"ac-1", "ac-3"}},
			},
		},
		{
			name: "a spike stub never covers a criterion",
			stubs: []artifact.Stub{
				{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}},
				{Slug: "plain", AcceptanceCriteria: []string{"ac-1"}},
			},
			want: []StubDecl{{Slug: "plain", AcceptanceCriteria: []string{"ac-1"}}},
		},
		{
			name:  "only spike stubs",
			stubs: []artifact.Stub{{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}}},
			want:  []StubDecl{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StubDecls(tt.stubs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("StubDecls() = %+v, want %+v", got, tt.want)
			}
		})
	}

	// The returned declarations never alias the decoded stubs' own lists.
	stubs := []artifact.Stub{{Slug: "plain", AcceptanceCriteria: []string{"ac-1"}}}
	got := StubDecls(stubs)
	got[0].AcceptanceCriteria[0] = "ac-9"
	if stubs[0].AcceptanceCriteria[0] != "ac-1" {
		t.Fatalf("StubDecls aliased the stub's criterion list")
	}
}

// TestStubDecls_UncoveredIsTheWallsRule proves Compute over StubDecls
// reads "no stub" exactly as the wall does: a criterion only a spike stub
// touches, or no stub at all, has an empty stub half.
func TestStubDecls_UncoveredIsTheWallsRule(t *testing.T) {
	t.Parallel()

	stubs := []artifact.Stub{
		{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}},
		{Slug: "plain", AcceptanceCriteria: []string{"ac-1"}},
	}
	got := Compute([]string{"ac-1", "ac-2"}, StubDecls(stubs), nil)
	if len(got["ac-1"].Stubs) != 1 || len(got["ac-2"].Stubs) != 0 {
		t.Fatalf("Compute(StubDecls) = %+v, want ac-1 stub-covered and ac-2 with no stub", got)
	}
}
