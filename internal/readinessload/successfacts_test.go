package readinessload

import (
	"reflect"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
)

// TestSuccessFactsOf is the loader's success facts over decoded
// frontmatter: sorted criteria, and on a feature the criteria no non-spike
// stub lists (featurecoverage.StubDecls — the wall's rule). A spike stub
// that lists a criterion — refused on decode, so only an in-memory literal
// carries one — covers nothing; a story carries no uncovered criterion.
func TestSuccessFactsOf(t *testing.T) {
	t.Parallel()

	criteria := []artifact.AcceptanceCriterion{{ID: "ac-2"}, {ID: "ac-1"}, {ID: "ac-3"}}
	tests := []struct {
		name string
		fm   *artifact.SpecFrontmatter
		want []string
	}{
		{
			name: "feature: spike stubs never cover",
			fm: &artifact.SpecFrontmatter{Class: artifact.ClassFeature, AcceptanceCriteria: criteria, Stubs: []artifact.Stub{
				{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}, AcceptanceCriteria: []string{"ac-1", "ac-2"}},
				{Slug: "plain", AcceptanceCriteria: []string{"ac-2"}},
			}},
			want: []string{"ac-1", "ac-3"},
		},
		{
			name: "feature without stubs",
			fm:   &artifact.SpecFrontmatter{Class: artifact.ClassFeature, AcceptanceCriteria: criteria},
			want: []string{"ac-1", "ac-2", "ac-3"},
		},
		{
			name: "story",
			fm:   &artifact.SpecFrontmatter{Class: artifact.ClassStory, AcceptanceCriteria: criteria},
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := successFactsOf(tt.fm)
			if !reflect.DeepEqual(got.CriterionIDs, []string{"ac-1", "ac-2", "ac-3"}) {
				t.Fatalf("CriterionIDs = %q, want sorted declared criteria", got.CriterionIDs)
			}
			if !reflect.DeepEqual(got.UncoveredCriteria, tt.want) {
				t.Fatalf("UncoveredCriteria = %q, want %q", got.UncoveredCriteria, tt.want)
			}
		})
	}
	if got := successFactsOf(&artifact.SpecFrontmatter{Class: artifact.ClassStory}); got.CriterionIDs == nil || got.UncoveredCriteria == nil {
		t.Fatalf("successFactsOf(no criteria) = %+v, want non-nil empty lists", got)
	}
}
