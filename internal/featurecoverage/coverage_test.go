package featurecoverage

import (
	"reflect"
	"testing"
)

// TestCoverage_PerCriterion proves Compute's happy path (spec/index-
// coverage ac-1--static): for every fixture feature (a table of declared
// criteria plus stub and story inputs), the returned Coverage per
// criterion is exactly what the stubs and links give — stub-covered,
// story-covered through a stub, story-covered on a criterion no stub
// lists (dc-1's SI-283 reading), both together, several of each per
// criterion, an uncovered criterion, and an input naming a criterion this
// feature does not declare (ignored, never fabricating a new key).
func TestCoverage_PerCriterion(t *testing.T) {
	tests := []struct {
		name  string
		ids   []string
		stubs []StubDecl
		links []StoryLink
		want  map[string]Coverage
	}{
		{
			name: "uncovered: no stub, no story",
			ids:  []string{"ac-1"},
			want: map[string]Coverage{"ac-1": {}},
		},
		{
			name:  "stub-covered",
			ids:   []string{"ac-1"},
			stubs: []StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}}},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}},
			},
		},
		{
			name:  "story-covered through a declaring stub",
			ids:   []string{"ac-1"},
			stubs: []StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}}},
			links: []StoryLink{{CriterionID: "ac-1", StoryRef: "spec/story-a"}},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}, Stories: []string{"spec/story-a"}},
			},
		},
		{
			// dc-1 / SI-283: a story implementing a criterion NO stub
			// lists still counts as covering it — the story half is read
			// from the index's backlinks directly, never through a
			// stub's own criterion list.
			name:  "story-covered on a criterion no stub lists",
			ids:   []string{"ac-1"},
			links: []StoryLink{{CriterionID: "ac-1", StoryRef: "spec/story-a"}},
			want: map[string]Coverage{
				"ac-1": {Stories: []string{"spec/story-a"}},
			},
		},
		{
			name: "both halves on separate criteria in one feature",
			ids:  []string{"ac-1", "ac-2"},
			stubs: []StubDecl{
				{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}},
			},
			links: []StoryLink{
				{CriterionID: "ac-2", StoryRef: "spec/story-b"},
			},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}},
				"ac-2": {Stories: []string{"spec/story-b"}},
			},
		},
		{
			name: "several stubs and stories on one criterion",
			ids:  []string{"ac-1"},
			stubs: []StubDecl{
				{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}},
				{Slug: "stub-b", AcceptanceCriteria: []string{"ac-1"}},
			},
			links: []StoryLink{
				{CriterionID: "ac-1", StoryRef: "spec/story-a"},
				{CriterionID: "ac-1", StoryRef: "spec/story-b"},
			},
			want: map[string]Coverage{
				"ac-1": {
					Stubs:   []string{"stub-a", "stub-b"},
					Stories: []string{"spec/story-a", "spec/story-b"},
				},
			},
		},
		{
			// A repeated id WITHIN one stub's own declared list still
			// counts as one covering stub (defense-in-depth mirroring the
			// wall projection's pre-extraction dedup; artifact.Stub.
			// Validate refuses this shape on decode, but Compute must
			// stay honest for a value that reached it another way).
			name:  "one stub repeating an id counts once",
			ids:   []string{"ac-1"},
			stubs: []StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1", "ac-1"}}},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}},
			},
		},
		{
			// An input naming a criterion this feature does not declare
			// is ignored — it is not this feature's own criterion, and
			// Compute never fabricates a new key for it.
			name: "input naming an undeclared criterion is ignored",
			ids:  []string{"ac-1"},
			stubs: []StubDecl{
				{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1", "ac-99"}},
			},
			links: []StoryLink{
				{CriterionID: "ac-99", StoryRef: "spec/story-a"},
			},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.ids, tt.stubs, tt.links)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Compute(%v, %+v, %+v) = %+v, want %+v", tt.ids, tt.stubs, tt.links, got, tt.want)
			}
			for id, cov := range tt.want {
				if got := got[id].Uncovered(); got != cov.Uncovered() {
					t.Errorf("Coverage(%s).Uncovered() = %v, want %v", id, got, cov.Uncovered())
				}
			}
		})
	}
}

// TestCoverage_UnreadableInputDisclosed proves the negative path (spec/
// index-coverage ac-2--static): an undecodable stub declaration or story
// backlink is disclosed on every criterion it could have affected, never
// silently counted as no coverage — on both a criterion that otherwise
// has no other coverage and one that does.
func TestCoverage_UnreadableInputDisclosed(t *testing.T) {
	tests := []struct {
		name  string
		ids   []string
		stubs []StubDecl
		links []StoryLink
		want  map[string]Coverage
	}{
		{
			// An unreadable stub declaration could have named ANY of the
			// feature's criteria, so it discloses on every one of them —
			// never a bare "no stub" on any.
			name: "unreadable stub discloses on every declared criterion",
			ids:  []string{"ac-1", "ac-2"},
			stubs: []StubDecl{
				{Unreadable: "stub declaration failed to decode"},
			},
			want: map[string]Coverage{
				"ac-1": {Disclosed: []string{"stub declaration failed to decode"}},
				"ac-2": {Disclosed: []string{"stub declaration failed to decode"}},
			},
		},
		{
			// An unreadable story backlink already names exactly which
			// criterion it targets, so it discloses only there —
			// alongside a criterion that IS otherwise stub-covered, the
			// disclosure rides beside the stub coverage rather than
			// replacing it.
			name:  "unreadable story link discloses on its one named criterion, beside stub coverage",
			ids:   []string{"ac-1"},
			stubs: []StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}}},
			links: []StoryLink{{CriterionID: "ac-1", Unreadable: "story could not be resolved"}},
			want: map[string]Coverage{
				"ac-1": {Stubs: []string{"stub-a"}, Disclosed: []string{"story could not be resolved"}},
			},
		},
		{
			// An unreadable story link on an otherwise-uncovered criterion
			// is disclosed, never reported as "no coverage".
			name:  "unreadable story link on an otherwise-uncovered criterion",
			ids:   []string{"ac-1"},
			links: []StoryLink{{CriterionID: "ac-1", Unreadable: "story could not be resolved"}},
			want: map[string]Coverage{
				"ac-1": {Disclosed: []string{"story could not be resolved"}},
			},
		},
		{
			// Both an unreadable stub AND an unreadable story land on the
			// same criterion: both disclosures are kept, in input order.
			name: "unreadable stub and unreadable story both disclosed",
			ids:  []string{"ac-1"},
			stubs: []StubDecl{
				{Unreadable: "stub declaration failed to decode"},
			},
			links: []StoryLink{
				{CriterionID: "ac-1", Unreadable: "story could not be resolved"},
			},
			want: map[string]Coverage{
				"ac-1": {Disclosed: []string{"stub declaration failed to decode", "story could not be resolved"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.ids, tt.stubs, tt.links)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Compute(%v, %+v, %+v) = %+v, want %+v", tt.ids, tt.stubs, tt.links, got, tt.want)
			}
			for id, cov := range tt.want {
				if len(cov.Disclosed) == 0 {
					continue
				}
				if got[id].Uncovered() {
					t.Errorf("Coverage(%s) with a disclosure reports Uncovered() = true, want false: an unreadable input is never counted as no coverage", id)
				}
			}
		})
	}
}

// TestCoverage_ReadsOnlyItsArguments proves Compute is a pure function of
// exactly its three arguments (ac-1: "it reads nothing else") — calling
// it twice with equal arguments (freshly built, not shared slices/maps)
// yields deeply equal results, and it never mutates its inputs.
func TestCoverage_ReadsOnlyItsArguments(t *testing.T) {
	newArgs := func() ([]string, []StubDecl, []StoryLink) {
		return []string{"ac-1", "ac-2"},
			[]StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}}},
			[]StoryLink{{CriterionID: "ac-2", StoryRef: "spec/story-a"}}
	}

	ids1, stubs1, links1 := newArgs()
	got1 := Compute(ids1, stubs1, links1)

	ids2, stubs2, links2 := newArgs()
	got2 := Compute(ids2, stubs2, links2)

	if !reflect.DeepEqual(got1, got2) {
		t.Fatalf("two calls with equal fresh arguments diverged: %+v vs %+v", got1, got2)
	}

	wantStubs := []StubDecl{{Slug: "stub-a", AcceptanceCriteria: []string{"ac-1"}}}
	if !reflect.DeepEqual(stubs1, wantStubs) {
		t.Errorf("Compute mutated its stubs argument: got %+v, want %+v", stubs1, wantStubs)
	}
	wantLinks := []StoryLink{{CriterionID: "ac-2", StoryRef: "spec/story-a"}}
	if !reflect.DeepEqual(links1, wantLinks) {
		t.Errorf("Compute mutated its links argument: got %+v, want %+v", links1, wantLinks)
	}
}
