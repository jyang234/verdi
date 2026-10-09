package workbench

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jyang234/verdi/internal/artifact"
	"github.com/jyang234/verdi/internal/boardlayout"
	"github.com/jyang234/verdi/internal/fixturegit"
	"github.com/jyang234/verdi/internal/index"
	"github.com/jyang234/verdi/internal/refindex"
)

// Tests for the New story dialog's data (spec/new-story-dialog-v2 ac-2,
// dc-2; SI-369 (1), (3), (13), (14)): the coverage chip text the wall and
// the dialog share, and the dialog's criterion coverage.

// TestCoverageChipText: the one chip text is the wall's, word for word —
// "no stub" for a criterion no stub lists, the singular for one stub, and
// the plural for more (index-coverage ac-1 keeps these texts; SI-369 (1)).
func TestCoverageChipText(t *testing.T) {
	for _, tc := range []struct {
		stubs int
		want  string
	}{
		{0, "no stub"},
		{1, "covered by 1 stub"},
		{2, "covered by 2 stubs"},
		{12, "covered by 12 stubs"},
	} {
		if got := coverageChipText(tc.stubs); got != tc.want {
			t.Errorf("coverageChipText(%d) = %q, want %q", tc.stubs, got, tc.want)
		}
	}
}

// TestWriteScopingReceipts_ChipBytesUnchanged: the AC card's chip, now
// worded by coverageChipText, is byte-identical to the literals the
// receipts wrote before the helper existed — the class, the testid, the
// count and the text — for no stub, one stub, and many; an open-question
// card claimed once still wears nothing.
func TestWriteScopingReceipts_ChipBytesUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		kind  boardlayout.ZoneKind
		stubs int
		want  string
	}{
		{"no stub", boardlayout.ZoneAC, 0, `<span class="coverage-chip coverage-chip--none" data-testid="coverage-ac-1" data-coverage="0">no stub</span>`},
		{"one stub", boardlayout.ZoneAC, 1, `<span class="coverage-chip coverage-chip--covered" data-testid="coverage-ac-1" data-coverage="1">covered by 1 stub</span>`},
		{"three stubs", boardlayout.ZoneAC, 3, `<span class="coverage-chip coverage-chip--covered" data-testid="coverage-ac-1" data-coverage="3">covered by 3 stubs</span>`},
		{"an open question claimed once wears nothing", boardlayout.ZoneOpenQuestion, 1, ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &BoardProjection{ACCoverage: map[string]int{"ac-1": tc.stubs}, OQClaims: map[string]int{"ac-1": tc.stubs}}
			var b strings.Builder
			writeScopingReceipts(&b, p, cardView{ID: "ac-1", Kind: string(tc.kind)})
			if got := b.String(); got != tc.want {
				t.Errorf("writeScopingReceipts =\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

// dialogFeature is a decoded feature with criteria ac-1..ac-4: ac-1 listed
// by one stub, ac-4 by another, and a spike stub that covers no criterion.
func dialogFeature() *artifact.SpecFrontmatter {
	return &artifact.SpecFrontmatter{
		AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}, {ID: "ac-2"}, {ID: "ac-3"}, {ID: "ac-4"}},
		Stubs: []artifact.Stub{
			{Slug: "first", AcceptanceCriteria: []string{"ac-1"}},
			{Slug: "fourth", AcceptanceCriteria: []string{"ac-4"}},
			{Slug: "probe", Spike: true, Resolves: []string{"oq-1"}},
		},
	}
}

// TestCreateCoverageOf: the dialog's coverage is featurecoverage.Compute's
// over the wall's stubs and the corpus's implements backlinks (dc-2): each
// criterion carries its stub count, its story refs, any disclosure, and
// Uncovered() — no stub, no story, no disclosure (SI-369 (1)) — and the
// dialog counts the uncovered ones by that rule (SI-369 (3)). A corpus it
// cannot read is a disclosure on every criterion, never a zero: no row is
// uncovered and the view is unproven.
func TestCreateCoverageOf(t *testing.T) {
	stories := fakeBacklinks{
		"spec/f#ac-2": {{From: "spec/second", Type: "implemented-by"}},
		"spec/f#ac-3": {{From: "spec/third-check", Type: "verified-by"}},
		"spec/f#ac-4": {{From: "spec/fourth-b", Type: "implemented-by"}, {From: "spec/fourth-a", Type: "implemented-by"}},
	}
	allClaimed := fakeBacklinks{
		"spec/f#ac-2": {{From: "spec/second", Type: "implemented-by"}},
		"spec/f#ac-3": {{From: "spec/third", Type: "implemented-by"}},
	}
	const notBuilt = "the corpus index could not be built: duplicate ref"
	for _, tc := range []struct {
		name   string
		fm     *artifact.SpecFrontmatter
		corpus corpusRead
		want   createCoverageView
	}{
		{
			name: "stub only, story only, neither (another edge type claims nothing), and both",
			fm:   dialogFeature(), corpus: corpusRead{links: stories},
			want: createCoverageView{
				Criteria: []createCriterionView{
					{ID: "ac-1", Stubs: 1},
					{ID: "ac-2", Stories: []string{"spec/second"}},
					{ID: "ac-3", Uncovered: true},
					{ID: "ac-4", Stubs: 1, Stories: []string{"spec/fourth-a", "spec/fourth-b"}},
				},
				Uncovered: 1,
			},
		},
		{
			name: "every criterion claimed: none uncovered",
			fm:   dialogFeature(), corpus: corpusRead{links: allClaimed},
			want: createCoverageView{
				Criteria: []createCriterionView{
					{ID: "ac-1", Stubs: 1},
					{ID: "ac-2", Stories: []string{"spec/second"}},
					{ID: "ac-3", Stories: []string{"spec/third"}},
					{ID: "ac-4", Stubs: 1},
				},
			},
		},
		{
			name: "an unbuilt corpus discloses every criterion's story half and counts none uncovered",
			fm:   dialogFeature(), corpus: corpusRead{err: errors.New("duplicate ref")},
			want: createCoverageView{
				Criteria: []createCriterionView{
					{ID: "ac-1", Stubs: 1, Disclosed: []string{notBuilt}},
					{ID: "ac-2", Disclosed: []string{notBuilt}},
					{ID: "ac-3", Disclosed: []string{notBuilt}},
					{ID: "ac-4", Stubs: 1, Disclosed: []string{notBuilt}},
				},
				Unproven: true,
			},
		},
		{
			name:   "a nil index is unread, never an empty corpus",
			fm:     &artifact.SpecFrontmatter{AcceptanceCriteria: []artifact.AcceptanceCriterion{{ID: "ac-1"}}},
			corpus: newCorpusRead(nil, nil),
			want: createCoverageView{
				Criteria: []createCriterionView{{ID: "ac-1", Disclosed: []string{"the corpus index could not be built: the corpus index was not built"}}},
				Unproven: true,
			},
		},
		{
			name: "no criteria: no rows, nothing uncovered, nothing unproven",
			fm:   &artifact.SpecFrontmatter{}, corpus: corpusRead{links: stories},
			want: createCoverageView{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := createCoverageOf("spec/f", tc.fm, tc.corpus); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("createCoverageOf =\n%#v\nwant\n%#v", got, tc.want)
			}
		})
	}
}

// dialogCoverageName is the loadBoard fixture's sealed feature.
const dialogCoverageName = "dialog-coverage"

// dialogCoverageSpec is an accepted-pending-build feature wall whose ac-1
// is listed by a stub, ac-2 implemented by a story only, ac-3 implemented
// only by a superseded story, and ac-4 claimed by nothing.
const dialogCoverageSpec = `---
id: spec/dialog-coverage
kind: spec
class: feature
title: "Dialog coverage"
status: accepted-pending-build
owners: [platform-team]
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
acceptance_criteria:
  - { id: ac-1, text: "listed by a stub", evidence: [attestation], anchor: "#ac-1" }
  - { id: ac-2, text: "implemented by a story", evidence: [attestation], anchor: "#ac-2" }
  - { id: ac-3, text: "implemented by a superseded story", evidence: [attestation], anchor: "#ac-3" }
  - { id: ac-4, text: "claimed by nothing", evidence: [attestation], anchor: "#ac-4" }
stubs:
  - { slug: first-stub, acceptance_criteria: [ac-1] }
frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }
---
# Dialog coverage

## Problem

Prose.

## Outcome

Prose.

## ac-1

Prose.

## ac-2

Prose.

## ac-3

Prose.

## ac-4

Prose.
`

// dialogStorySpec is a story implementing dialogCoverageSpec's criterion
// ac, at status st; a superseded one carries the frozen stamp its status
// requires.
func dialogStorySpec(slug, ac, st string) string {
	frozen := ""
	if st == "superseded" {
		frozen = "frozen: { at: 2026-07-12, commit: 6400db382876f416ed943f6b6e22954f9666fde3 }\n"
	}
	return `---
id: spec/` + slug + `
kind: spec
class: story
title: "` + slug + `"
status: ` + st + `
owners: [platform-team]
story: jira:DLG-1
problem: { text: "p", anchor: "#problem" }
outcome: { text: "o", anchor: "#outcome" }
links:
  - { type: implements, ref: "spec/dialog-coverage#` + ac + `" }
acceptance_criteria:
  - { id: ac-1, text: "implements the parent criterion", evidence: [static], anchor: "#ac-1" }
` + frozen + `---
# ` + slug + `

## Problem

## Outcome

## ac-1

Prose.
`
}

// newDialogCoverageFixture is the sealed feature, its two stories, and a
// store manifest, so the wall resolves its creation form.
func newDialogCoverageFixture(t *testing.T) *fixturegit.Repo {
	t.Helper()
	repo := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + dialogCoverageName + "/spec.md": dialogCoverageSpec,
			".verdi/specs/active/second-story/spec.md":               dialogStorySpec("second-story", "ac-2", "draft"),
			".verdi/specs/active/third-story/spec.md":                dialogStorySpec("third-story", "ac-3", "superseded"),
			".verdi/.gitignore":                                      "data/\n",
			".verdi/verdi.yaml":                                      "schema: verdi.layout/v1\n",
		},
		Message: "seed dialog coverage fixture",
	}})
	setDefaultBranchSymref(t, repo.Dir)
	return repo
}

// TestLoadBoard_CreateCoverage: on a sealed accepted-pending-build feature
// wall whose creation form resolved, loadBoard attaches the dialog's
// coverage from the render's one corpus index (dc-2): the stub half is the
// wall chip's count and words the wall chip's text, the stories are the
// index's, a superseded story still claims its criterion (SI-369 (14)),
// and the whole equals what the index's call to action computes for the
// same feature (coverageOf), so the dialog, the wall and the index agree.
func TestLoadBoard_CreateCoverage(t *testing.T) {
	repo := newDialogCoverageFixture(t)
	proj, _, _, _, err := (&boardSpecServer{root: repo.Dir}).loadBoard(context.Background(), dialogCoverageName)
	if err != nil {
		t.Fatalf("loadBoard: %v", err)
	}
	if len(proj.CreateFields) == 0 {
		t.Fatalf("the sealed wall resolved no creation form (notices %v): the fixture proves nothing", proj.Notices)
	}
	want := createCoverageView{
		Criteria: []createCriterionView{
			{ID: "ac-1", Stubs: 1},
			{ID: "ac-2", Stories: []string{"spec/second-story"}},
			{ID: "ac-3", Stories: []string{"spec/third-story"}},
			{ID: "ac-4", Uncovered: true},
		},
		Uncovered: 1,
	}
	if !reflect.DeepEqual(proj.CreateCoverage, want) {
		t.Fatalf("CreateCoverage =\n%#v\nwant\n%#v", proj.CreateCoverage, want)
	}

	t.Run("the stub half is the wall chip's count and text", func(t *testing.T) {
		body := getBoard(t, NewHandler(repo.Dir), dialogCoverageName).Body.String()
		for _, row := range proj.CreateCoverage.Criteria {
			if n := proj.ACCoverage[row.ID]; n != row.Stubs {
				t.Errorf("%s: dialog stub count %d, wall chip count %d", row.ID, row.Stubs, n)
			}
			chip := regexp.MustCompile(`data-testid="coverage-` + regexp.QuoteMeta(row.ID) + `" data-coverage="[0-9]+">([^<]*)</span>`).FindStringSubmatch(body)
			if chip == nil {
				t.Fatalf("%s: the served wall has no coverage chip", row.ID)
			}
			if got := coverageChipText(row.Stubs); got != chip[1] {
				t.Errorf("%s: the dialog words its stub count %q, the wall chip reads %q", row.ID, got, chip[1])
			}
		}
	})

	t.Run("the index computes the same coverage", func(t *testing.T) {
		ix, err := index.Build(repo.Dir)
		if err != nil {
			t.Fatal(err)
		}
		entry := refindex.Entry{Ref: "spec/" + dialogCoverageName, Source: refindex.SourceDefault, StatusGroup: refindex.StatusGroupAcceptedPendingBuild}
		cov := coverageOf(entry, readSpecTreeMeta(repo.Dir, dialogCoverageName), newCorpusRead(ix, nil))
		if cov.unproven != "" || len(cov.criteria) != len(proj.CreateCoverage.Criteria) {
			t.Fatalf("the index read %+v", cov)
		}
		for i, row := range proj.CreateCoverage.Criteria {
			c := cov.coverage[cov.criteria[i]]
			if cov.criteria[i] != row.ID || len(c.Stubs) != row.Stubs || !reflect.DeepEqual(c.Stories, row.Stories) || c.Uncovered() != row.Uncovered {
				t.Errorf("%s: the index reads %+v, the dialog %+v", row.ID, c, row)
			}
		}
	})
}

// TestLoadBoard_CreateCoverageOnlyWithTheForm: a wall that renders no
// creation form carries no dialog coverage — a draft feature wall, and a
// sealed one whose form could not resolve (no store manifest), which
// discloses why instead.
func TestLoadBoard_CreateCoverageOnlyWithTheForm(t *testing.T) {
	noManifest := fixturegit.Build(t, []fixturegit.Layer{{
		Files: map[string]string{
			".verdi/specs/active/" + dialogCoverageName + "/spec.md": dialogCoverageSpec,
			".verdi/.gitignore": "data/\n",
		},
		Message: "seed a sealed feature without a store manifest",
	}})
	setDefaultBranchSymref(t, noManifest.Dir)
	for _, tc := range []struct {
		name, root, spec string
	}{
		{"a draft feature wall", newScopingWallFixture(t), scopingWallName},
		{"a sealed wall whose form did not resolve", noManifest.Dir, dialogCoverageName},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proj, _, _, _, err := (&boardSpecServer{root: tc.root}).loadBoard(context.Background(), tc.spec)
			if err != nil {
				t.Fatalf("loadBoard: %v", err)
			}
			if len(proj.CreateFields) != 0 {
				t.Fatalf("the wall resolved a creation form: the case proves nothing")
			}
			if !reflect.DeepEqual(proj.CreateCoverage, createCoverageView{}) {
				t.Errorf("CreateCoverage = %#v, want none without the form", proj.CreateCoverage)
			}
		})
	}
}

// TestCorpusRead_UnreadReason: the one disclosure the index and the dialog
// give for a corpus they could not read — empty for a built index, the
// build's own error otherwise, and a missing index named as such.
func TestCorpusRead_UnreadReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		corpus corpusRead
		want   string
	}{
		{"a built index reads", corpusRead{links: fakeBacklinks{}}, ""},
		{"a build error is disclosed", newCorpusRead(nil, errors.New("duplicate ref")), "the corpus index could not be built: duplicate ref"},
		{"a missing index is disclosed", newCorpusRead(nil, nil), "the corpus index could not be built: the corpus index was not built"},
	} {
		if got := tc.corpus.unreadReason(); got != tc.want {
			t.Errorf("%s: unreadReason = %q, want %q", tc.name, got, tc.want)
		}
	}
}
