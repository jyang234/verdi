package readinessload

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/readinesspilot"
)

// TestClaimedQuestionsOf_NonSpikeStubNeverClaims pins the loader's spike
// filter (PLAN.md §7 I-128 option (a); spec/uat-round-1 ac-10), the wall's
// readiness's only claim source since the wall shell's own derivation
// retired (spec/wall-strip-and-drawer-v2 dc-4; SI-368 (32) T5): only a
// spike stub's `resolves` claims an open question. A plain stub cannot
// legally carry `resolves` — the decode seam refuses it (02 §Kind
// registry, DC-4) — so this is the defense behind a refused state: one
// reaching the loader anyway claims nothing. Claims are sorted by question
// and by slug, and the result is never nil.
func TestClaimedQuestionsOf_NonSpikeStubNeverClaims(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stubs []artifact.Stub
		want  []readinesspilot.ClaimedQuestion
	}{
		{
			name:  "no stubs",
			stubs: nil,
			want:  []readinesspilot.ClaimedQuestion{},
		},
		{
			name: "a plain stub naming a question claims nothing",
			stubs: []artifact.Stub{
				{Slug: "smuggled-plain-stub", AcceptanceCriteria: []string{"ac-1"}, Resolves: []string{"oq-1"}},
			},
			want: []readinesspilot.ClaimedQuestion{},
		},
		{
			name: "spike stubs claim, sorted, and a plain stub beside them adds nothing",
			stubs: []artifact.Stub{
				{Slug: "zeta-spike", Spike: true, Resolves: []string{"oq-2"}},
				{Slug: "smuggled-plain-stub", AcceptanceCriteria: []string{"ac-1"}, Resolves: []string{"oq-1", "oq-2"}},
				{Slug: "alpha-spike", Spike: true, Resolves: []string{"oq-2", "oq-1"}},
			},
			want: []readinesspilot.ClaimedQuestion{
				{QuestionID: "oq-1", StubSlugs: []string{"alpha-spike"}},
				{QuestionID: "oq-2", StubSlugs: []string{"alpha-spike", "zeta-spike"}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := claimedQuestionsOf(tc.stubs)
			if got == nil || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("claimedQuestionsOf = %#v, want %#v", got, tc.want)
			}
		})
	}
}
