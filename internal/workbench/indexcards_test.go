package workbench

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/disclosure"
	"github.com/jyang234/verdi/internal/featurecoverage"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/model"
	"github.com/jyang234/verdi/internal/refindex"
)

// cardsNow is the card tests' one fixed clock.
func cardsNow() time.Time { return time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC) }

// before renders the committer date d before cardsNow.
func before(d time.Duration) string {
	return cardsNow().Add(-d).Format("2006-01-02T15:04:05-07:00")
}

const day = 24 * time.Hour

// fakeBacklinks is a corpus backlink lookup keyed by target ref.
type fakeBacklinks map[string][]index.Backlink

func (f fakeBacklinks) Backlinks(ref string) []index.Backlink { return f[ref] }

func TestAgeOf(t *testing.T) {
	unreadable := disclosure.New("refindex:date-unreadable", "spec/lost", "last-change date unreadable: boom")
	desk := refindex.StatusGroupDraftsInProgress
	tests := []struct {
		name string
		e    refindex.Entry
		want ageFact
		// wantUnproven is a substring the unproven reason must carry; ""
		// means the age is proven (want.unproven must then be empty too).
		wantUnproven string
	}{
		{"a change this hour reads today", refindex.Entry{StatusGroup: desk, Date: before(time.Hour)}, ageFact{text: "today", days: 0}, ""},
		{"just under a day still reads today", refindex.Entry{StatusGroup: desk, Date: before(day - time.Second)}, ageFact{text: "today", days: 0}, ""},
		{"one whole day", refindex.Entry{StatusGroup: desk, Date: before(day)}, ageFact{text: "1 d ago", days: 1}, ""},
		{"13 days on the desk: not quiet", refindex.Entry{StatusGroup: desk, Date: before(13 * day)}, ageFact{text: "13 d ago", days: 13}, ""},
		{"exactly 14 days on the desk: not quiet (exclusive)", refindex.Entry{StatusGroup: desk, Date: before(14 * day)}, ageFact{text: "14 d ago", days: 14}, ""},
		{"14 days and a second on the desk: quiet", refindex.Entry{StatusGroup: desk, Date: before(14*day + time.Second)}, ageFact{text: "quiet 14 d", days: 14, quiet: true}, ""},
		{"15 days on the desk: quiet", refindex.Entry{StatusGroup: desk, Date: before(15 * day)}, ageFact{text: "quiet 15 d", days: 15, quiet: true}, ""},
		{"a year in the accepted column: never quiet", refindex.Entry{StatusGroup: refindex.StatusGroupAcceptedPendingBuild, Date: before(365 * day)}, ageFact{text: "365 d ago", days: 365}, ""},
		{"a year among the active components: never quiet", refindex.Entry{StatusGroup: refindex.StatusGroupActiveComponents, Date: before(365 * day)}, ageFact{text: "365 d ago", days: 365}, ""},
		{"a disclosed date: age unproven with the entry's own reason", refindex.Entry{Ref: "spec/lost", StatusGroup: desk, DateDisclosed: &unreadable}, ageFact{text: "age unproven"}, "last-change date unreadable: boom"},
		{"no date at all: age unproven, never today", refindex.Entry{Ref: "spec/undated", StatusGroup: desk}, ageFact{text: "age unproven"}, "no last-change date was computed"},
		{"a date that is not a committer date: age unproven", refindex.Entry{Ref: "spec/garbled", StatusGroup: desk, Date: "yesterday"}, ageFact{text: "age unproven"}, `"yesterday" is not a committer date`},
		{"a change after the clock: age unproven, never a negative age", refindex.Entry{Ref: "spec/ahead", StatusGroup: desk, Date: before(-2 * day)}, ageFact{text: "age unproven"}, "is after this render's clock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ageOf(tt.e, cardsNow())
			if tt.wantUnproven == "" {
				if got != tt.want {
					t.Fatalf("ageOf = %+v, want %+v", got, tt.want)
				}
				return
			}
			if got.text != tt.want.text || got.quiet || !strings.Contains(got.unproven, tt.wantUnproven) {
				t.Fatalf("ageOf = %+v, want text %q, never quiet, unproven carrying %q", got, tt.want.text, tt.wantUnproven)
			}
		})
	}
}

func TestReviewOf(t *testing.T) {
	answered := reviewConsultation{configured: true, inReview: map[string]bool{"design/in-review": true}}
	failed := reviewConsultation{configured: true, failed: true}
	tests := []struct {
		name   string
		source refindex.Source
		ref    string
		rc     reviewConsultation
		want   reviewState
	}{
		{"a local draft with an open MR", refindex.SourceLocal, "spec/in-review", answered, reviewOpen},
		{"a remote-only draft with an open MR", refindex.SourceRemote, "spec/in-review", answered, reviewOpen},
		{"a local + remote draft with none", refindex.SourceBoth, "spec/quiet-one", answered, reviewNotOpen},
		{"a draft when the forge failed: unavailable, never not open", refindex.SourceLocal, "spec/in-review", failed, reviewUnavailable},
		{"a draft with no forge configured", refindex.SourceLocal, "spec/in-review", reviewConsultation{}, reviewUnconfigured},
		{"a default-branch spec has no branch in review", refindex.SourceDefault, "spec/in-review", answered, reviewNotOpen},
		{"a default-branch spec when the forge failed: still not in review", refindex.SourceDefault, "spec/in-review", failed, reviewNotOpen},
		{"an unknown source fails closed to unavailable", refindex.Source("sideways"), "spec/in-review", answered, reviewUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewOf(refindex.Entry{Ref: tt.ref, Source: tt.source}, tt.rc); got != tt.want {
				t.Fatalf("reviewOf = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMoveOf(t *testing.T) {
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/x", "no spec.md yet")
	successor := fakeBacklinks{"spec/old": {{From: "spec/new", Type: "superseded-by"}}}
	renamed := classWords{m: &model.Model{Vocabulary: model.Vocabulary{Verbs: map[string]string{"merge": "land"}}}}
	active, archive := refindex.ZoneActive, refindex.ZoneArchive
	entry := func(g refindex.StatusGroup, status string, z refindex.Zone) refindex.Entry {
		return refindex.Entry{Ref: "spec/old", StatusGroup: g, SpecStatus: status, Zone: z}
	}
	tests := []struct {
		name   string
		e      refindex.Entry
		review reviewState
		corpus corpusRead
		words  classWords
		want   nextMove
	}{
		{"an ordinary desk draft", entry(refindex.StatusGroupDraftsInProgress, "draft", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{kind: moveOpenWall, text: "open the wall"}},
		{"a desk draft with review unavailable is still ordinary", entry(refindex.StatusGroupDraftsInProgress, "draft", active), reviewUnavailable, corpusRead{}, classWords{}, nextMove{kind: moveOpenWall, text: "open the wall"}},
		{"a desk draft in review", entry(refindex.StatusGroupDraftsInProgress, "draft", active), reviewOpen, corpusRead{}, classWords{}, nextMove{kind: moveAwaitingMerge, text: "awaiting merge"}},
		{"a desk draft in review speaks the model's merge word", entry(refindex.StatusGroupDraftsInProgress, "draft", active), reviewOpen, corpusRead{}, renamed, nextMove{kind: moveAwaitingMerge, text: "awaiting land"}},
		{"a branch with no draft spec", refindex.Entry{Ref: "spec/x", StatusGroup: refindex.StatusGroupDraftsInProgress, Disclosed: &noDraft, Zone: active}, reviewOpen, corpusRead{}, classWords{}, nextMove{kind: moveInspectBranch, text: "inspect the branch"}},
		{"an unproven entry on the desk", entry(refindex.StatusGroupDraftsInProgress, "unproven", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{kind: moveInspectBranch, text: "inspect the branch"}},
		{"an accepted spec", entry(refindex.StatusGroupAcceptedPendingBuild, "accepted-pending-build", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{kind: moveSealedWall, text: "sealed wall"}},
		{"an active component", entry(refindex.StatusGroupActiveComponents, "active", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{kind: moveObligations, text: "obligations"}},
		{"a superseded spec with a successor backlink links it", entry(refindex.StatusGroupTerminal, "superseded", active), reviewNotOpen, corpusRead{links: successor}, classWords{}, nextMove{kind: moveSeeSuccessor, text: "see successor", href: "/a/spec/new"}},
		{"a superseded spec with no backlink is not linked", entry(refindex.StatusGroupTerminal, "superseded", active), reviewNotOpen, corpusRead{links: fakeBacklinks{}}, classWords{}, nextMove{kind: moveSeeSuccessor, text: "see successor"}},
		{"a superseded spec with an unbuilt corpus is not linked", entry(refindex.StatusGroupTerminal, "superseded", active), reviewNotOpen, corpusRead{err: errors.New("boom")}, classWords{}, nextMove{kind: moveSeeSuccessor, text: "see successor"}},
		{"a closed spec still in the active zone", entry(refindex.StatusGroupTerminal, "closed", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{kind: moveArchive, text: "archive"}},
		{"an archived spec has none", entry(refindex.StatusGroupTerminal, "closed", archive), reviewNotOpen, corpusRead{}, classWords{}, nextMove{}},
		{"an archived superseded spec has none", entry(refindex.StatusGroupTerminal, "superseded", archive), reviewNotOpen, corpusRead{links: successor}, classWords{}, nextMove{}},
		{"an unknown zone fails closed", entry(refindex.StatusGroupDraftsInProgress, "draft", ""), reviewNotOpen, corpusRead{}, classWords{}, nextMove{}},
		{"an unknown group fails closed", entry(refindex.StatusGroup("sideways"), "draft", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{}},
		{"an unknown terminal status fails closed", entry(refindex.StatusGroupTerminal, "retired", active), reviewNotOpen, corpusRead{}, classWords{}, nextMove{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := moveOf(tt.e, tt.review, tt.corpus, tt.words); got != tt.want {
				t.Fatalf("moveOf = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// acceptedFeature is an accepted default-branch entry named spec/f.
func acceptedFeature() refindex.Entry {
	return refindex.Entry{Ref: "spec/f", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive}
}

// featureTree is a decoded, servable feature with criteria ac-1..ac-3,
// one stub listing ac-1, and one spike stub.
func featureTree() specTreeMeta {
	return specTreeMeta{
		title: "F", class: artifact.ClassFeature, boardServable: true,
		criteria: []string{"ac-1", "ac-2", "ac-3"},
		stubs: []artifact.Stub{
			{Slug: "first", AcceptanceCriteria: []string{"ac-1"}},
			{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}},
		},
	}
}

func TestCoverageOf(t *testing.T) {
	stories := fakeBacklinks{
		"spec/f#ac-2": {{From: "spec/story-two", Type: "implemented-by"}},
		"spec/f#ac-3": {{From: "spec/not-a-story-edge", Type: "verified-by"}},
	}
	story := featureTree()
	story.class = artifact.ClassStory
	archiveOnly := featureTree()
	archiveOnly.boardServable = false
	desk := acceptedFeature()
	desk.StatusGroup = refindex.StatusGroupDraftsInProgress
	draft := acceptedFeature()
	draft.Source = refindex.SourceLocal
	tests := []struct {
		name   string
		e      refindex.Entry
		tree   specTreeMeta
		corpus corpusRead
		// want is compared whole, except coverage, which wantCoverage
		// checks when non-nil.
		want         coverageRead
		wantCoverage map[string]featurecoverage.Coverage
	}{
		{
			name: "an accepted feature: stubs from the working tree, stories from implemented-by backlinks only",
			e:    acceptedFeature(), tree: featureTree(), corpus: corpusRead{links: stories},
			want: coverageRead{applies: true, criteria: []string{"ac-1", "ac-2", "ac-3"}},
			wantCoverage: map[string]featurecoverage.Coverage{
				"ac-1": {Stubs: []string{"first"}},
				"ac-2": {Stories: []string{"spec/story-two"}},
				"ac-3": {},
			},
		},
		{name: "an accepted story: no call to action", e: acceptedFeature(), tree: story, corpus: corpusRead{links: stories}, want: coverageRead{}},
		{name: "a desk entry: no call to action", e: desk, tree: featureTree(), corpus: corpusRead{links: stories}, want: coverageRead{}},
		{name: "a design-branch entry: no call to action", e: draft, tree: featureTree(), corpus: corpusRead{links: stories}, want: coverageRead{}},
		{name: "an unreadable working tree: unproven", e: acceptedFeature(), tree: specTreeMeta{unreadable: "spec/f's working-tree file does not decode: x"}, corpus: corpusRead{links: stories}, want: coverageRead{applies: true, unproven: "spec/f's working-tree file does not decode: x"}},
		{name: "no active-zone file: unproven", e: acceptedFeature(), tree: archiveOnly, corpus: corpusRead{links: stories}, want: coverageRead{applies: true, unproven: "no active-zone working-tree file exists to serve its wall"}},
		{name: "an unbuilt corpus: unproven", e: acceptedFeature(), tree: featureTree(), corpus: corpusRead{err: errors.New("duplicate ref")}, want: coverageRead{applies: true, unproven: "the corpus index could not be built: duplicate ref"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coverageOf(tt.e, tt.tree, tt.corpus)
			gotCoverage := got.coverage
			got.coverage = nil
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("coverageOf = %+v, want %+v", got, tt.want)
			}
			if tt.wantCoverage == nil {
				if gotCoverage != nil {
					t.Fatalf("coverage = %+v, want none computed", gotCoverage)
				}
				return
			}
			if !reflect.DeepEqual(gotCoverage, tt.wantCoverage) {
				t.Fatalf("coverage = %+v, want %+v", gotCoverage, tt.wantCoverage)
			}
		})
	}
}

func TestCtaFrom(t *testing.T) {
	cov := func(m map[string]featurecoverage.Coverage) coverageRead {
		return coverageRead{applies: true, criteria: []string{"ac-1", "ac-2", "ac-3"}, coverage: m}
	}
	renamed := classWords{m: &model.Model{Vocabulary: model.Vocabulary{Classes: map[string]string{"story": "ticket"}}}}
	tests := []struct {
		name  string
		cov   coverageRead
		words classWords
		want  *callToAction
	}{
		{
			name:  "two uncovered criteria: counted, the first in declared order named and linked",
			cov:   cov(map[string]featurecoverage.Coverage{"ac-1": {Stubs: []string{"s"}}, "ac-2": {}, "ac-3": {}}),
			words: classWords{},
			want:  &callToAction{unclaimed: 2, criterion: "ac-2", href: "/board/spec/f?new-story=ac-2", text: "2 AC unclaimed · ac-2 · New story"},
		},
		{
			name:  "the story word comes from the model",
			cov:   cov(map[string]featurecoverage.Coverage{"ac-1": {}, "ac-2": {Stories: []string{"spec/s"}}, "ac-3": {Stubs: []string{"s"}}}),
			words: renamed,
			want:  &callToAction{unclaimed: 1, criterion: "ac-1", href: "/board/spec/f?new-story=ac-1", text: "1 AC unclaimed · ac-1 · New ticket"},
		},
		{
			name: "every criterion covered: no call to action",
			cov:  cov(map[string]featurecoverage.Coverage{"ac-1": {Stubs: []string{"s"}}, "ac-2": {Stories: []string{"spec/s"}}, "ac-3": {Stubs: []string{"t"}}}),
			want: nil,
		},
		{
			name: "a disclosed criterion: coverage unproven, never counted as uncovered",
			cov:  cov(map[string]featurecoverage.Coverage{"ac-1": {Disclosed: []string{"stub x unreadable"}}, "ac-2": {Stubs: []string{"s"}}, "ac-3": {Stubs: []string{"t"}}}),
			want: &callToAction{text: "coverage unproven", unproven: "ac-1: stub x unreadable"},
		},
		{
			name: "a disclosed criterion beside an uncovered one: still unproven, never a partial count",
			cov:  cov(map[string]featurecoverage.Coverage{"ac-1": {}, "ac-2": {Disclosed: []string{"link y unreadable"}}, "ac-3": {}}),
			want: &callToAction{text: "coverage unproven", unproven: "ac-2: link y unreadable"},
		},
		{
			name: "an unreadable input: coverage unproven, never zero",
			cov:  coverageRead{applies: true, unproven: "the corpus index could not be built: boom"},
			want: &callToAction{text: "coverage unproven", unproven: "the corpus index could not be built: boom"},
		},
		{name: "not an accepted feature: none", cov: coverageRead{}, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ctaFrom("f", tt.cov, tt.words); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ctaFrom = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestProjectCard(t *testing.T) {
	noDraft := disclosure.New("refindex:no-draft-spec", "spec/empty", "no spec.md yet")
	unproven := disclosure.New("refindex:unproven-spec-state", "spec/unproven", "scan incomplete")
	dateLost := disclosure.New("refindex:date-unreadable", "spec/lost", "boom")
	cc := cardContext{
		review: reviewConsultation{configured: true, inReview: map[string]bool{"design/drafted": true}},
		corpus: corpusRead{links: fakeBacklinks{}},
		now:    cardsNow(),
	}
	tests := []struct {
		name string
		e    refindex.Entry
		tree specTreeMeta
		want cardFacts
	}{
		{
			name: "a design draft: its decoded title, its review state, its move",
			e:    refindex.Entry{Ref: "spec/drafted", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Date: before(3 * day), Title: "A drafted thing"},
			want: cardFacts{name: "drafted", title: "A drafted thing", age: ageFact{text: "3 d ago", days: 3}, review: reviewOpen, move: nextMove{kind: moveAwaitingMerge, text: "awaiting merge"}},
		},
		{
			name: "a design draft with no decoded title: empty and disclosed, never its ref",
			e:    refindex.Entry{Ref: "spec/untitled", Source: refindex.SourceRemote, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Date: before(day)},
			want: cardFacts{name: "untitled", titleUnproven: "disclosed-unproven [workbench:title-unproven] spec/untitled: no title was decoded from this design branch's spec", age: ageFact{text: "1 d ago", days: 1}, review: reviewNotOpen, move: nextMove{kind: moveOpenWall, text: "open the wall"}, disclosed: true},
		},
		{
			name: "a branch with no draft spec: no title, its own disclosure, disclosed",
			e:    refindex.Entry{Ref: "spec/empty", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, Disclosed: &noDraft, Zone: refindex.ZoneActive, Date: before(20 * day)},
			want: cardFacts{name: "empty", age: ageFact{text: "quiet 20 d", days: 20, quiet: true}, review: reviewNotOpen, move: nextMove{kind: moveInspectBranch, text: "inspect the branch"}, disclosed: true},
		},
		{
			name: "a default entry: the working-tree title and trim",
			e:    refindex.Entry{Ref: "spec/comp", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupActiveComponents, SpecStatus: "active", Zone: refindex.ZoneActive, Date: before(40 * day)},
			tree: specTreeMeta{title: "The Component", class: artifact.ClassComponent, story: "jira:X-1", boardServable: true},
			want: cardFacts{name: "comp", title: "The Component", class: artifact.ClassComponent, story: "jira:X-1", boardServable: true, age: ageFact{text: "40 d ago", days: 40}, review: reviewNotOpen, move: nextMove{kind: moveObligations, text: "obligations"}},
		},
		{
			name: "a default entry with no working-tree title keeps the directory's ref rule",
			e:    refindex.Entry{Ref: "spec/bare", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupTerminal, SpecStatus: "closed", Zone: refindex.ZoneArchive, Date: before(400 * day)},
			tree: specTreeMeta{unreadable: "spec/bare has no readable working-tree file in the active or archive zone"},
			want: cardFacts{name: "bare", title: "spec/bare", age: ageFact{text: "400 d ago", days: 400}, review: reviewNotOpen},
		},
		{
			name: "an unproven default entry with an unproven date: disclosed",
			e:    refindex.Entry{Ref: "spec/unproven", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "unproven", Disclosed: &unproven, DateDisclosed: &dateLost, Zone: refindex.ZoneActive},
			tree: specTreeMeta{title: "Unproven", class: artifact.ClassStory, boardServable: true},
			want: cardFacts{name: "unproven", title: "Unproven", class: artifact.ClassStory, boardServable: true, age: ageFact{text: "age unproven", unproven: disclosure.Render(dateLost)}, review: reviewNotOpen, move: nextMove{kind: moveInspectBranch, text: "inspect the branch"}, disclosed: true},
		},
		{
			name: "a date disclosure alone marks the card disclosed",
			e:    refindex.Entry{Ref: "spec/lost", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", DateDisclosed: &dateLost, Zone: refindex.ZoneActive, Title: "Lost"},
			want: cardFacts{name: "lost", title: "Lost", age: ageFact{text: "age unproven", unproven: disclosure.Render(dateLost)}, review: reviewNotOpen, move: nextMove{kind: moveOpenWall, text: "open the wall"}, disclosed: true},
		},
		{
			name: "an age unproven by the clock alone (no DateDisclosed) marks the card disclosed",
			e:    refindex.Entry{Ref: "spec/ahead", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Date: before(-2 * day), Title: "Ahead"},
			want: cardFacts{name: "ahead", title: "Ahead", age: ageFact{text: "age unproven", unproven: "disclosed-unproven [workbench:age-unproven] spec/ahead: last-change date " + before(-2*day) + " is after this render's clock 2024-06-15T12:00:00Z"}, review: reviewNotOpen, move: nextMove{kind: moveOpenWall, text: "open the wall"}, disclosed: true},
		},
		{
			name: "an accepted feature with an unproven age: its call to action rides on the card, and the age marks it disclosed",
			e:    acceptedFeature(),
			tree: featureTree(),
			want: cardFacts{name: "f", title: "F", class: artifact.ClassFeature, boardServable: true, age: ageFact{text: "age unproven", unproven: "disclosed-unproven [workbench:date-unproven] spec/f: no last-change date was computed for this entry"}, review: reviewNotOpen, move: nextMove{kind: moveSealedWall, text: "sealed wall"}, cta: &callToAction{unclaimed: 2, criterion: "ac-2", href: "/board/spec/f?new-story=ac-2", text: "2 AC unclaimed · ac-2 · New story"}, disclosed: true},
		},
		{
			name: "a coverage disclosure alone never marks the card disclosed",
			e:    func() refindex.Entry { e := acceptedFeature(); e.Date = before(5 * day); return e }(),
			tree: func() specTreeMeta { t := featureTree(); t.boardServable = false; return t }(),
			want: cardFacts{name: "f", title: "F", class: artifact.ClassFeature, age: ageFact{text: "5 d ago", days: 5}, review: reviewNotOpen, move: nextMove{kind: moveSealedWall, text: "sealed wall"}, cta: &callToAction{text: "coverage unproven", unproven: "no active-zone working-tree file exists to serve its wall"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.want.entry = tt.e
			if got := projectCard(tt.e, tt.tree, cc); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("projectCard =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestNewCorpusRead(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".verdi"), 0o755); err != nil {
		t.Fatal(err)
	}
	ix, err := index.Build(root)
	if err != nil {
		t.Fatalf("index.Build over an empty root: %v", err)
	}
	if got := newCorpusRead(ix, nil); got.err != nil || got.links == nil {
		t.Fatalf("a built index: %+v, want its backlinks and no error", got)
	}
	boom := errors.New("boom")
	if got := newCorpusRead(nil, boom); got.links != nil || !errors.Is(got.err, boom) {
		t.Fatalf("a build error: %+v, want the error and no backlinks", got)
	}
	if got := newCorpusRead(nil, nil); got.links != nil || got.err == nil {
		t.Fatalf("no index and no error: %+v, want an error and no backlinks, never a nil index behind the interface", got)
	}
}

// cardsFeatureSpecMD is a statusless feature spec with three criteria, a
// stub listing ac-1, and a spike stub.
const cardsFeatureSpecMD = `---
id: spec/cards-feature
kind: spec
class: feature
title: "Cards Feature"
owners: [platform-team]
story: jira:CARDS-1
acceptance_criteria:
  - { id: ac-1, text: "one", evidence: [static] }
  - { id: ac-2, text: "two", evidence: [static] }
  - { id: ac-3, text: "three", evidence: [static] }
open_questions:
  - { id: oq-1, text: "why?", anchor: "#oq-1" }
stubs:
  - { slug: first-story, acceptance_criteria: [ac-1] }
  - { slug: probe, spike: true, resolves: [oq-1] }
---
# Cards Feature
`

// writeCardsSpec writes content as name's spec.md in zone under root.
func writeCardsSpec(t *testing.T, root, zone, name, content string) {
	t.Helper()
	dir := filepath.Join(root, ".verdi", "specs", zone, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "spec.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadSpecTreeMeta(t *testing.T) {
	root := t.TempDir()
	writeCardsSpec(t, root, "active", "cards-feature", cardsFeatureSpecMD)
	writeCardsSpec(t, root, "archive", "archived-feature", strings.ReplaceAll(cardsFeatureSpecMD, "cards-feature", "archived-feature"))
	writeCardsSpec(t, root, "active", "undecodable", strings.Replace(cardsFeatureSpecMD, "title:", "bogus_field: x\ntitle:", 1))
	writeCardsSpec(t, root, "active", "unsplittable", "no frontmatter here\n")
	tests := []struct {
		name           string
		spec           string
		want           specTreeMeta
		wantUnreadable string
	}{
		{
			name: "an active-zone feature: trim, criteria in declared order, stubs, servable",
			spec: "cards-feature",
			want: specTreeMeta{
				title: "Cards Feature", class: artifact.ClassFeature, story: "jira:CARDS-1", boardServable: true,
				criteria: []string{"ac-1", "ac-2", "ac-3"},
				stubs:    []artifact.Stub{{Slug: "first-story", AcceptanceCriteria: []string{"ac-1"}}, {Slug: "probe", Spike: true, Resolves: []string{"oq-1"}}},
			},
		},
		{
			name: "an archive-only feature decodes but is not servable",
			spec: "archived-feature",
			want: specTreeMeta{
				title: "Cards Feature", class: artifact.ClassFeature, story: "jira:CARDS-1",
				criteria: []string{"ac-1", "ac-2", "ac-3"},
				stubs:    []artifact.Stub{{Slug: "first-story", AcceptanceCriteria: []string{"ac-1"}}, {Slug: "probe", Spike: true, Resolves: []string{"oq-1"}}},
			},
		},
		{name: "a missing file names itself", spec: "nowhere", wantUnreadable: "spec/nowhere has no readable working-tree file in the active or archive zone"},
		{name: "an undecodable active file stays servable and names itself", spec: "undecodable", want: specTreeMeta{boardServable: true}, wantUnreadable: "spec/undecodable's working-tree file does not decode"},
		{name: "an unsplittable active file stays servable and names itself", spec: "unsplittable", want: specTreeMeta{boardServable: true}, wantUnreadable: "spec/unsplittable's working-tree file does not split"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readSpecTreeMeta(root, tt.spec)
			if !strings.HasPrefix(got.unreadable, tt.wantUnreadable) || (tt.wantUnreadable == "") != (got.unreadable == "") {
				t.Fatalf("unreadable = %q, want it to start with %q", got.unreadable, tt.wantUnreadable)
			}
			got.unreadable = ""
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("readSpecTreeMeta = %+v, want %+v", got, tt.want)
			}
			title, class, story, servable := specWorkingTreeMeta(root, tt.spec)
			if title != tt.want.title || class != tt.want.class || story != tt.want.story || servable != tt.want.boardServable {
				t.Fatalf("specWorkingTreeMeta = (%q, %q, %q, %v), want readSpecTreeMeta's trim", title, class, story, servable)
			}
		})
	}
}

// TestHomeCards: each card is index-aligned with its entry; a default
// entry's card carries its working-tree read, and a design entry's reads
// nothing from the working tree (a same-named working-tree file never
// leaks into a draft's card).
func TestHomeCards(t *testing.T) {
	root := t.TempDir()
	writeCardsSpec(t, root, "active", "cards-feature", cardsFeatureSpecMD)
	entries := []refindex.Entry{
		{Ref: "spec/cards-feature", Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild, SpecStatus: "accepted-pending-build", Zone: refindex.ZoneActive},
		{Ref: "spec/cards-feature", Source: refindex.SourceLocal, StatusGroup: refindex.StatusGroupDraftsInProgress, SpecStatus: "draft", Zone: refindex.ZoneActive, Title: "Its revision"},
	}
	cards := homeCards(root, entries, cardContext{corpus: corpusRead{links: fakeBacklinks{}}, now: cardsNow()})
	if len(cards) != len(entries) {
		t.Fatalf("homeCards = %d cards, want %d", len(cards), len(entries))
	}
	for i := range entries {
		if !reflect.DeepEqual(cards[i].entry, entries[i]) {
			t.Fatalf("card %d carries entry %+v, want %+v", i, cards[i].entry, entries[i])
		}
	}
	if got := cards[0]; got.title != "Cards Feature" || got.class != artifact.ClassFeature || !got.boardServable || got.cta == nil || got.cta.criterion != "ac-2" {
		t.Fatalf("default card = %+v, want the working-tree trim and a call to action naming ac-2", got)
	}
	if got := cards[1]; got.title != "Its revision" || got.class != "" || got.boardServable || got.cta != nil {
		t.Fatalf("design card = %+v, want its own title and no working-tree trim", got)
	}
}
